package batch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"

	internalarchive "wails-app/internal/archive"
	contracts "wails-app/internal/contracts"
)

type SelectiveExtractCallbacks struct {
	IsCancelled          func() bool
	CheckConflict        func(srcLabel, dst string, blanket **contracts.ConflictResolution) (string, bool, error)
	ExtractNestedArchive func(archivePath string, nestedChain []string) ([]byte, error)
	Normalize            func(string) string
	ArchiveEntryPath     func(archiver.File) string
}

type SelectiveExtractor struct {
	callbacks     SelectiveExtractCallbacks
	progress      *ProgressTracker
	addTotalBytes func(int64)
}

type archiveEntry struct {
	innerPath string
	data      []byte
	size      int64
	mtime     int64
}

func NewSelectiveExtractor(callbacks SelectiveExtractCallbacks, progress *ProgressTracker, addTotalBytes func(int64)) *SelectiveExtractor {
	return &SelectiveExtractor{callbacks: callbacks, progress: progress, addTotalBytes: addTotalBytes}
}

func (e *SelectiveExtractor) ExtractItem(item contracts.SelectiveExtractItem, defaultDest string, blanket *contracts.ConflictResolution) (int64, int64, *contracts.ConflictResolution, error) {
	if e.callbacks.IsCancelled != nil && e.callbacks.IsCancelled() {
		return 0, 0, blanket, fmt.Errorf("operation cancelled")
	}

	destRoot := item.DestinationDir
	if destRoot == "" {
		destRoot = defaultDest
	}

	var collected []archiveEntry
	var err error
	if len(item.NestedChain) == 0 {
		collected, err = e.collectFromDisk(item.ArchivePath, item.Entries)
	} else {
		collected, err = e.collectFromNested(item.ArchivePath, item.NestedChain, item.Entries)
	}
	if err != nil {
		return 0, 0, blanket, err
	}
	if len(collected) == 0 {
		return 0, 0, blanket, nil
	}

	var totalSize int64
	for _, entry := range collected {
		totalSize += entry.size
	}
	if e.addTotalBytes != nil {
		e.addTotalBytes(totalSize)
	}

	files, bytes, err := e.writeEntries(collected, destRoot, item.StripPrefix, item.FlattenPaths, &blanket)
	return files, bytes, blanket, err
}

func (e *SelectiveExtractor) collectFromDisk(archivePath string, entries []string) ([]archiveEntry, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return e.collectZipFile(archivePath, entries)
	}
	return e.collectTarFile(archivePath, entries)
}

func (e *SelectiveExtractor) collectZipFile(archivePath string, entries []string) ([]archiveEntry, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var out []archiveEntry
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ep := e.callbacks.Normalize(f.Name)
		if !wantEntry(ep, entries, e.callbacks.Normalize) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", f.Name, err)
		}
		out = append(out, archiveEntry{ep, data, int64(f.UncompressedSize64), f.Modified.Unix()})
	}
	return out, nil
}

func (e *SelectiveExtractor) collectTarFile(archivePath string, entries []string) ([]archiveEntry, error) {
	var out []archiveEntry
	err := archiver.Walk(archivePath, func(f archiver.File) error {
		if f.IsDir() {
			return nil
		}
		ep := e.callbacks.Normalize(e.callbacks.ArchiveEntryPath(f))
		if !wantEntry(ep, entries, e.callbacks.Normalize) {
			return nil
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("read %q: %w", ep, err)
		}
		var mtime int64
		if hdr, ok := f.Header.(*tar.Header); ok {
			mtime = hdr.ModTime.Unix()
		}
		out = append(out, archiveEntry{ep, data, f.Size(), mtime})
		return nil
	})
	return out, err
}

func (e *SelectiveExtractor) collectFromNested(archivePath string, nestedChain []string, entries []string) ([]archiveEntry, error) {
	data, err := e.callbacks.ExtractNestedArchive(archivePath, nestedChain)
	if err != nil {
		return nil, fmt.Errorf("resolve nested chain: %w", err)
	}
	return e.collectFromBytes(data, nestedChain[len(nestedChain)-1], entries)
}

func (e *SelectiveExtractor) collectFromBytes(data []byte, archiveName string, entries []string) ([]archiveEntry, error) {
	if internalarchive.IsZipMagic(data) {
		return e.collectZipBytes(data, entries)
	}
	if internalarchive.IsRarArchiveName(archiveName) {
		return e.collectRarBytes(data, archiveName, entries)
	}
	return e.collectTarBytes(data, entries)
}

func (e *SelectiveExtractor) collectZipBytes(data []byte, entries []string) ([]archiveEntry, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var out []archiveEntry
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ep := e.callbacks.Normalize(f.Name)
		if !wantEntry(ep, entries, e.callbacks.Normalize) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", f.Name, err)
		}
		buf, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", f.Name, err)
		}
		out = append(out, archiveEntry{ep, buf, int64(f.UncompressedSize64), f.Modified.Unix()})
	}
	return out, nil
}

func (e *SelectiveExtractor) collectTarBytes(data []byte, entries []string) ([]archiveEntry, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	var out []archiveEntry
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.FileInfo().IsDir() {
			io.Copy(io.Discard, tr)
			continue
		}
		ep := e.callbacks.Normalize(h.Name)
		if !wantEntry(ep, entries, e.callbacks.Normalize) {
			io.Copy(io.Discard, tr)
			continue
		}
		buf, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", h.Name, err)
		}
		out = append(out, archiveEntry{ep, buf, h.Size, h.ModTime.Unix()})
	}
	return out, nil
}

func (e *SelectiveExtractor) collectRarBytes(data []byte, archiveName string, entries []string) ([]archiveEntry, error) {
	reader, err := internalarchive.OpenArchiveReader(archiveName, data)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var out []archiveEntry
	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ep := e.callbacks.Normalize(e.callbacks.ArchiveEntryPath(f))
		if f.IsDir() || !wantEntry(ep, entries, e.callbacks.Normalize) {
			f.Close()
			continue
		}
		buf, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", ep, err)
		}
		out = append(out, archiveEntry{ep, buf, f.Size(), f.ModTime().Unix()})
	}
	return out, nil
}

func (e *SelectiveExtractor) writeEntries(collected []archiveEntry, destRoot string, stripPrefix string, flatten bool, blanket **contracts.ConflictResolution) (int64, int64, error) {
	var totalFiles, totalBytes int64
	rebasePrefix := e.callbacks.Normalize(stripPrefix)

	for _, entry := range collected {
		if e.callbacks.IsCancelled != nil && e.callbacks.IsCancelled() {
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}

		relPath := e.callbacks.Normalize(entry.innerPath)
		if rebasePrefix != "" {
			if relPath == rebasePrefix {
				continue
			}
			prefixWithSep := rebasePrefix + "/"
			relPath = strings.TrimPrefix(relPath, prefixWithSep)
		}
		if relPath == "" {
			continue
		}

		var destPath string
		if flatten {
			destPath = filepath.Join(destRoot, filepath.Base(filepath.FromSlash(relPath)))
		} else {
			destPath = filepath.Join(destRoot, filepath.FromSlash(relPath))
		}

		finalDst, skip, err := e.callbacks.CheckConflict(entry.innerPath, destPath, blanket)
		if err != nil {
			return totalFiles, totalBytes, err
		}
		if skip {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(finalDst), 0755); err != nil {
			return totalFiles, totalBytes, err
		}

		baseItems, baseBytes := e.progress.Snapshot()
		e.progress.Set(baseItems, baseBytes, filepath.Base(entry.innerPath), entry.size, false)

		if err := os.WriteFile(finalDst, entry.data, 0644); err != nil {
			return totalFiles, totalBytes, fmt.Errorf("write %q: %w", finalDst, err)
		}

		written := int64(len(entry.data))
		e.progress.Set(baseItems+1, baseBytes+written, "", 0, false)

		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

func wantEntry(innerPath string, entries []string, normalize func(string) string) bool {
	if len(entries) == 0 {
		return true
	}
	for _, entry := range entries {
		entry = normalize(entry)
		if entry == "" || entry == "." {
			return true
		}
		if innerPath == entry || strings.HasPrefix(innerPath, entry+"/") {
			return true
		}
	}
	return false
}