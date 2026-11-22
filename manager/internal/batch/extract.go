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
	tartar "wails-app/vfs/archive/tar"
	contracts "wails-app/internal/contracts"
)

type ExtractCallbacks struct {
	IsCancelled      func() bool
	CheckConflict    func(srcLabel, dst string, blanket **contracts.ConflictResolution) (string, bool, error)
	StreamToFile     func(r io.Reader, dst string, size int64, label string) (int64, error)
	ArchiveEntryPath func(archiver.File) string
	Normalize        func(string) string
	IsArchive        func(string) bool
}

type ArchiveExtractor struct {
	callbacks ExtractCallbacks
}

func NewArchiveExtractor(callbacks ExtractCallbacks) *ArchiveExtractor {
	return &ArchiveExtractor{callbacks: callbacks}
}

func (e *ArchiveExtractor) ExtractOnDisk(archivePath string, req *contracts.BatchExtractRequest, destRoot string, blanket *contracts.ConflictResolution) (int64, int64, *contracts.ConflictResolution, error) {
	ctx := extractCtx{callbacks: e.callbacks, req: req, blanket: blanket, destRoot: destRoot, depth: 0}
	files, bytes, err := ctx.extractOnDisk(archivePath)
	return files, bytes, ctx.blanket, err
}

type extractCtx struct {
	callbacks ExtractCallbacks
	req       *contracts.BatchExtractRequest
	blanket   *contracts.ConflictResolution
	destRoot  string
	depth     int
}

func (ec *extractCtx) cancelled() bool {
	return ec.callbacks.IsCancelled != nil && ec.callbacks.IsCancelled()
}

func (ec *extractCtx) tooDeep() bool {
	if !ec.req.ExpandNestedArchives {
		return true
	}
	if ec.req.MaxNestDepth < 0 {
		return false
	}
	return ec.depth >= ec.req.MaxNestDepth
}

func (ec *extractCtx) extractOnDisk(archivePath string) (int64, int64, error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return ec.extractZipOnDisk(archivePath)
	}
	return ec.extractTarOnDisk(archivePath)
}

func (ec *extractCtx) extractZipOnDisk(archivePath string) (int64, int64, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, 0, err
	}
	defer r.Close()

	rootStrip := zipRootPrefix(r.File, ec.req.FlattenSingleRoot, ec.callbacks.Normalize)
	var totalFiles, totalBytes int64

	for _, f := range r.File {
		if ec.cancelled() {
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}

		ep := ec.callbacks.Normalize(f.Name)
		if rootStrip != "" {
			if ep = strings.TrimPrefix(ep, rootStrip); ep == "" {
				continue
			}
		}
		if !internalarchive.SafeExtractEntryName(ep) {
			continue
		}
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))

		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}

		if ec.callbacks.IsArchive(f.Name) && !ec.tooDeep() {
			rc, err := f.Open()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			child := ec.child(filepath.Join(ec.destRoot, ArchiveBaseName(f.Name)))
			nf, nb, nerr := child.extractNestedFromBytes(data, f.Name)
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			if nerr != nil {
				return totalFiles, totalBytes, nerr
			}
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return totalFiles, totalBytes, err
		}
		finalDst, skip, cerr := ec.callbacks.CheckConflict(f.Name, destPath, &ec.blanket)
		if cerr != nil {
			rc.Close()
			return totalFiles, totalBytes, cerr
		}
		if skip {
			rc.Close()
			continue
		}
		if err := os.MkdirAll(filepath.Dir(finalDst), 0755); err != nil {
			rc.Close()
			return totalFiles, totalBytes, err
		}
		written, werr := ec.callbacks.StreamToFile(rc, finalDst, int64(f.UncompressedSize64), f.Name)
		rc.Close()
		if werr != nil {
			return totalFiles, totalBytes, werr
		}
		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

func (ec *extractCtx) extractTarOnDisk(archivePath string) (int64, int64, error) {
	var rootStrip string
	if ec.req.FlattenSingleRoot {
		var tops []string
		_ = archiver.Walk(archivePath, func(f archiver.File) error {
			parts := strings.SplitN(ec.callbacks.Normalize(ec.callbacks.ArchiveEntryPath(f)), "/", 2)
			tops = appendUnique(tops, parts[0])
			return nil
		})
		if len(tops) == 1 {
			rootStrip = tops[0] + "/"
		}
	}

	var totalFiles, totalBytes int64
	err := archiver.Walk(archivePath, func(f archiver.File) error {
		if ec.cancelled() {
			return fmt.Errorf("operation cancelled")
		}
		ep := ec.callbacks.Normalize(ec.callbacks.ArchiveEntryPath(f))
		if rootStrip != "" {
			if ep = strings.TrimPrefix(ep, rootStrip); ep == "" {
				return nil
			}
		}
		if !internalarchive.SafeExtractEntryName(ep) {
			return nil
		}
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))

		if f.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		if ec.callbacks.IsArchive(f.Name()) && !ec.tooDeep() {
			data, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			child := ec.child(filepath.Join(ec.destRoot, ArchiveBaseName(f.Name())))
			nf, nb, nerr := child.extractNestedFromBytes(data, f.Name())
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			return nerr
		}

		finalDst, skip, err := ec.callbacks.CheckConflict(f.Name(), destPath, &ec.blanket)
		if err != nil {
			return err
		}
		if skip {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(finalDst), 0755); err != nil {
			return err
		}
		written, err := ec.callbacks.StreamToFile(f, finalDst, f.Size(), f.Name())
		if err != nil {
			return err
		}
		totalFiles++
		totalBytes += written
		return nil
	})
	return totalFiles, totalBytes, err
}

func (ec *extractCtx) extractNestedFromBytes(data []byte, archiveName string) (int64, int64, error) {
	if internalarchive.IsZipMagic(data) {
		return ec.extractZipBytes(data)
	}
	if internalarchive.IsRarArchiveName(archiveName) {
		return ec.extractRarBytes(data, archiveName)
	}
	if isCompressedTarName(archiveName) {
		// Decompress with the archive package's codec, then run the plain
		// tar path: the archiver reader mangles entry paths for compressed
		// tars (directory prefixes are lost).
		dc, err := tartar.DecompressReader(bytes.NewReader(data), archiveName)
		if err != nil {
			return 0, 0, err
		}
		defer dc.Close()
		decompressed, err := io.ReadAll(dc)
		if err != nil {
			return 0, 0, err
		}
		return ec.extractTarBytes(decompressed)
	}
	return ec.extractTarBytes(data)
}

func isCompressedTarName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2") ||
		strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz")
}

func (ec *extractCtx) extractZipBytes(data []byte) (int64, int64, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, 0, err
	}
	var totalFiles, totalBytes int64
	for _, f := range r.File {
		if ec.cancelled() {
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}
		ep := ec.callbacks.Normalize(f.Name)
		if !internalarchive.SafeExtractEntryName(ep) {
			continue
		}
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))
		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}
		if ec.callbacks.IsArchive(f.Name) && !ec.tooDeep() {
			rc, err := f.Open()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			nested, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			child := ec.child(filepath.Join(ec.destRoot, ArchiveBaseName(f.Name)))
			nf, nb, nerr := child.extractNestedFromBytes(nested, f.Name)
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			if nerr != nil {
				return totalFiles, totalBytes, nerr
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return totalFiles, totalBytes, err
		}
		finalDst, skip, cerr := ec.callbacks.CheckConflict(f.Name, destPath, &ec.blanket)
		if cerr != nil {
			rc.Close()
			return totalFiles, totalBytes, cerr
		}
		if skip {
			rc.Close()
			continue
		}
		if err := os.MkdirAll(filepath.Dir(finalDst), 0755); err != nil {
			rc.Close()
			return totalFiles, totalBytes, err
		}
		written, werr := ec.callbacks.StreamToFile(rc, finalDst, int64(f.UncompressedSize64), f.Name)
		rc.Close()
		if werr != nil {
			return totalFiles, totalBytes, werr
		}
		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

func (ec *extractCtx) extractTarBytes(data []byte) (int64, int64, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	var totalFiles, totalBytes int64
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return totalFiles, totalBytes, err
		}
		if ec.cancelled() {
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}
		ep := ec.callbacks.Normalize(h.Name)
		if !internalarchive.SafeExtractEntryName(ep) {
			continue
		}
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))
		if h.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}
		if ec.callbacks.IsArchive(h.Name) && !ec.tooDeep() {
			nested, err := io.ReadAll(tr)
			if err != nil {
				return totalFiles, totalBytes, err
			}
			child := ec.child(filepath.Join(ec.destRoot, ArchiveBaseName(h.Name)))
			nf, nb, nerr := child.extractNestedFromBytes(nested, h.Name)
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			if nerr != nil {
				return totalFiles, totalBytes, nerr
			}
			continue
		}
		finalDst, skip, cerr := ec.callbacks.CheckConflict(h.Name, destPath, &ec.blanket)
		if cerr != nil {
			return totalFiles, totalBytes, cerr
		}
		if skip {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(finalDst), 0755); err != nil {
			return totalFiles, totalBytes, err
		}
		written, werr := ec.callbacks.StreamToFile(tr, finalDst, h.Size, h.Name)
		if werr != nil {
			return totalFiles, totalBytes, werr
		}
		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

func (ec *extractCtx) extractRarBytes(data []byte, archiveName string) (int64, int64, error) {
	reader, err := internalarchive.OpenArchiveReader(archiveName, data)
	if err != nil {
		return 0, 0, err
	}
	defer reader.Close()

	var totalFiles, totalBytes int64
	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return totalFiles, totalBytes, err
		}
		if ec.cancelled() {
			f.Close()
			return totalFiles, totalBytes, fmt.Errorf("operation cancelled")
		}
		ep := ec.callbacks.Normalize(ec.callbacks.ArchiveEntryPath(f))
		if !internalarchive.SafeExtractEntryName(ep) {
			continue
		}
		destPath := filepath.Join(ec.destRoot, filepath.FromSlash(ep))
		if f.IsDir() {
			f.Close()
			os.MkdirAll(destPath, 0755)
			continue
		}
		if ec.callbacks.IsArchive(f.Name()) && !ec.tooDeep() {
			nested, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				return totalFiles, totalBytes, err
			}
			child := ec.child(filepath.Join(ec.destRoot, ArchiveBaseName(f.Name())))
			nf, nb, nerr := child.extractNestedFromBytes(nested, f.Name())
			ec.blanket = child.blanket
			totalFiles += nf
			totalBytes += nb
			if nerr != nil {
				return totalFiles, totalBytes, nerr
			}
			continue
		}
		finalDst, skip, cerr := ec.callbacks.CheckConflict(f.Name(), destPath, &ec.blanket)
		if cerr != nil {
			f.Close()
			return totalFiles, totalBytes, cerr
		}
		if skip {
			f.Close()
			continue
		}
		if err := os.MkdirAll(filepath.Dir(finalDst), 0755); err != nil {
			f.Close()
			return totalFiles, totalBytes, err
		}
		written, werr := ec.callbacks.StreamToFile(f, finalDst, f.Size(), f.Name())
		f.Close()
		if werr != nil {
			return totalFiles, totalBytes, werr
		}
		totalFiles++
		totalBytes += written
	}
	return totalFiles, totalBytes, nil
}

func (ec *extractCtx) child(destRoot string) *extractCtx {
	return &extractCtx{
		callbacks: ec.callbacks,
		req:       ec.req,
		blanket:   ec.blanket,
		destRoot:  destRoot,
		depth:     ec.depth + 1,
	}
}

func ArchiveBaseName(name string) string {
	base := filepath.Base(name)
	for _, suffix := range []string{".tar.gz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(strings.ToLower(base), suffix) {
			return base[:len(base)-len(suffix)]
		}
	}
	if ext := filepath.Ext(base); ext != "" {
		return base[:len(base)-len(ext)]
	}
	return base
}

func zipRootPrefix(files []*zip.File, flatten bool, normalize func(string) string) string {
	if !flatten {
		return ""
	}
	var tops []string
	for _, f := range files {
		parts := strings.SplitN(normalize(f.Name), "/", 2)
		tops = appendUnique(tops, parts[0])
	}
	if len(tops) == 1 {
		return tops[0] + "/"
	}
	return ""
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}