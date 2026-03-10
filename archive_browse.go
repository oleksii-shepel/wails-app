package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mholt/archiver/v3"
)

// ArchiveList lists one level of a top-level archive on disk.
func (a *App) archiveList(fsPath string, innerPath string) (ArchiveListing, error) {
	entries, err := a.walkFile(fsPath, innerPath, nil)
	if err != nil {
		return ArchiveListing{}, err
	}
	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		Entries:     entries,
	}, nil
}

// ArchiveListNested lists one level inside a nested archive chain without
// writing any temporary files.
func (a *App) archiveListNested(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	return a.vfsBrowseArchive(fsPath, nestedChain, innerPath)
}

func (a *App) walkFile(fsPath, innerPath string, nestedChain []string) ([]FileInfo, error) {
	lower := strings.ToLower(fsPath)

	if strings.HasSuffix(lower, ".zip") {
		r, err := zip.OpenReader(fsPath)
		if err != nil {
			return nil, err
		}
		defer r.Close()

		seen := make(map[string]FileInfo)
		innerPath = normalize(innerPath)
		for _, f := range r.File {
			ep := normalize(f.Name)
			isDir := f.FileInfo().IsDir()
			if err := a.processEntryInto(ep, isDir, int64(f.UncompressedSize64), f.Modified.UnixMilli(), innerPath, nestedChain, seen); err != nil {
				return nil, err
			}
		}
		return a.sortFileInfo(seen), nil
	}

	if needsArchiveReader(fsPath) {
		return a.walkFileArchiveReader(fsPath, innerPath, nestedChain)
	}

	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)
	err := archiver.Walk(fsPath, func(f archiver.File) error {
		ep := normalize(archiveEntryPath(f))
		return a.processEntryInto(ep, f.IsDir(), f.Size(), f.ModTime().UnixMilli(), innerPath, nestedChain, seen)
	})
	if err != nil {
		return nil, err
	}
	return a.sortFileInfo(seen), nil
}

func (a *App) walkFileArchiveReader(fsPath, innerPath string, nestedChain []string) ([]FileInfo, error) {
	f, err := os.Open(fsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	aIface, err := archiver.ByExtension(fsPath)
	if err != nil {
		return nil, fmt.Errorf("walkFileArchiveReader: unrecognised format %q: %w", fsPath, err)
	}
	reader, ok := aIface.(archiver.Reader)
	if !ok {
		return nil, fmt.Errorf("walkFileArchiveReader: format %q is not readable", fsPath)
	}
	if err := reader.Open(f, fi.Size()); err != nil {
		return nil, err
	}
	defer reader.Close()

	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for {
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ep := normalize(archiveEntryPath(entry))
		procErr := a.processEntryInto(ep, entry.IsDir(), entry.Size(), entry.ModTime().UnixMilli(), innerPath, nestedChain, seen)
		entry.Close()
		if procErr != nil {
			return nil, procErr
		}
	}
	return a.sortFileInfo(seen), nil
}

func (a *App) walkBytes(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	if isZipMagic(data) {
		return a.walkZip(data, innerPath, nestedChain)
	}
	archiveName := ""
	if len(nestedChain) > 0 {
		archiveName = nestedChain[len(nestedChain)-1]
	}
	if archiveName != "" && needsArchiveReader(archiveName) {
		return a.walkArchiveReader(data, innerPath, nestedChain, archiveName)
	}
	return a.walkTar(data, innerPath, nestedChain)
}

func (a *App) walkZip(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for _, f := range r.File {
		ep := normalize(f.Name)
		isDir := f.FileInfo().IsDir()
		if err := a.processEntryInto(ep, isDir, int64(f.UncompressedSize64), f.Modified.UnixMilli(), innerPath, nestedChain, seen); err != nil {
			return nil, err
		}
	}
	return a.sortFileInfo(seen), nil
}

func (a *App) walkTar(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ep := normalize(h.Name)
		isDir := h.FileInfo().IsDir()
		if err := a.processEntryInto(ep, isDir, h.Size, h.ModTime.UnixMilli(), innerPath, nestedChain, seen); err != nil {
			return nil, err
		}
	}
	return a.sortFileInfo(seen), nil
}

func (a *App) walkArchiveReader(data []byte, innerPath string, nestedChain []string, archiveName string) ([]FileInfo, error) {
	reader, err := openArchiveReader(archiveName, data)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ep := normalize(archiveEntryPath(f))
		isDir := f.IsDir()
		if err := a.processEntryInto(ep, isDir, f.Size(), f.ModTime().UnixMilli(), innerPath, nestedChain, seen); err != nil {
			f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}

	return a.sortFileInfo(seen), nil
}

func (a *App) processEntryInto(
	entryPath string,
	isDir bool,
	size int64,
	modifiedMs int64,
	innerPath string,
	nestedChain []string,
	seen map[string]FileInfo,
) error {
	rel := getRelativePath(entryPath, innerPath)
	if rel == "" {
		return nil
	}

	parts := strings.SplitN(rel, "/", 2)
	childName := parts[0]
	implicitDir := len(parts) > 1
	childIsDir := isDir || implicitDir
	childIsArchive := !childIsDir && isArchive(childName)

	if _, exists := seen[childName]; exists {
		return nil
	}

	fi := FileInfo{
		Name:     childName,
		Path:     joinPath(innerPath, childName),
		Type:     FileTypeFile,
		Size:     size,
		Modified: modifiedMs,
	}
	switch {
	case childIsDir:
		fi.Type = FileTypeDirectory
		fi.Size = 0
	case childIsArchive:
		fi.Type = FileTypeArchive
		fi.IsNested = true
		fi.NestedChain = append(append([]string{}, nestedChain...), fi.Path)
	}

	seen[childName] = fi
	return nil
}

func (a *App) sortFileInfo(seen map[string]FileInfo) []FileInfo {
	entries := make([]FileInfo, 0, len(seen))
	for _, entry := range seen {
		entries = append(entries, entry)
	}
	order := map[FileType]int{FileTypeDirectory: 0, FileTypeArchive: 1, FileTypeFile: 2, FileTypeSymlink: 3}
	sort.Slice(entries, func(i, j int) bool {
		oi, oj := order[entries[i].Type], order[entries[j].Type]
		if oi != oj {
			return oi < oj
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}
