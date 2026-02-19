package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/mholt/archiver/v3"
)

// ArchiveList lists a single level of a filesystem archive
func (a *App) ArchiveList(fsPath string, innerPath string) (ArchiveListing, error) {
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

// ArchiveListNested lists a single level of a nested archive without temp files
func (a *App) ArchiveListNested(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	// Extract the nested archive data into memory
	archiveData, err := a.extractNestedArchive(fsPath, nestedChain)
	if err != nil {
		return ArchiveListing{}, err
	}

	// Determine format (defaulting to zip for this example logic)
	// In production, you'd use detectExt(nestedChain[len(nestedChain)-1])
	entries, err := a.walkBytes(archiveData, innerPath, nestedChain)
	if err != nil {
		return ArchiveListing{}, err
	}

	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		NestedChain: nestedChain,
		Entries:     entries,
	}, nil
}

// ─── Internal helpers ─────────────────────────────────────────────

func (a *App) walkFile(fsPath, innerPath string, nestedChain []string) ([]FileInfo, error) {
	seen := make(map[string]FileInfo)

	err := archiver.Walk(fsPath, func(f archiver.File) error {
		return a.processEntry(f, innerPath, nestedChain, seen)
	})
	if err != nil {
		return nil, err
	}

	return a.sortFileInfo(seen), nil
}

func (a *App) walkBytes(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	// Detect if it's a ZIP or TAR
	// This is a simplified check; archiver.ByHeader is better for production
	if len(data) > 2 && data[0] == 'P' && data[1] == 'K' {
		return a.walkZip(data, innerPath, nestedChain)
	}
	return a.walkTar(data, innerPath, nestedChain)
}

func (a *App) walkZip(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	reader := bytes.NewReader(data)
	r, err := zip.NewReader(reader, int64(len(data)))
	if err != nil {
		return nil, err
	}

	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for _, f := range r.File {
		ep := normalize(f.Name)
		rel := getRelativePath(ep, innerPath)
		if rel == "" {
			continue
		}

		parts := strings.SplitN(rel, "/", 2)
		childName := parts[0]
		isDir := f.FileInfo().IsDir() || len(parts) > 1

		if _, exists := seen[childName]; !exists {
			fi := FileInfo{
				Name:     childName,
				Path:     joinPath(innerPath, childName),
				Type:     "FILE",
				Size:     int64(f.UncompressedSize64),
				Modified: f.Modified.UnixMilli(),
			}
			if isDir {
				fi.Type = "DIRECTORY"
			} else if isArchiveFile(childName) {
				fi.Type = "ARCHIVE"
				fi.IsNested = true
				fi.NestedChain = append(append([]string{}, nestedChain...), childName)
			}
			seen[childName] = fi
		}
	}
	return a.sortFileInfo(seen), nil
}

func (a *App) walkTar(data []byte, innerPath string, nestedChain []string) ([]FileInfo, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	seen := make(map[string]FileInfo)
	innerPath = normalize(innerPath)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		ep := normalize(header.Name)
		rel := getRelativePath(ep, innerPath)
		if rel == "" {
			continue
		}

		parts := strings.SplitN(rel, "/", 2)
		childName := parts[0]
		isDir := header.FileInfo().IsDir() || len(parts) > 1

		if _, exists := seen[childName]; !exists {
			fi := FileInfo{
				Name:     childName,
				Path:     joinPath(innerPath, childName),
				Type:     "FILE",
				Size:     header.Size,
				Modified: header.ModTime.UnixMilli(),
			}
			if isDir {
				fi.Type = "DIRECTORY"
			} else if isArchiveFile(childName) {
				fi.Type = "ARCHIVE"
				fi.IsNested = true
				fi.NestedChain = append(append([]string{}, nestedChain...), childName)
			}
			seen[childName] = fi
		}
	}
	return a.sortFileInfo(seen), nil
}

func (a *App) extractNestedArchive(fsPath string, nestedChain []string) ([]byte, error) {
	if len(nestedChain) == 0 {
		return nil, fmt.Errorf("empty chain")
	}

	// 1. Extract first layer from physical file
	currentData, err := a.extractFromFile(fsPath, nestedChain[0])
	if err != nil {
		return nil, err
	}

	// 2. Extract subsequent layers from memory
	for i := 1; i < len(nestedChain); i++ {
		nextData, err := a.extractFromBytes(currentData, nestedChain[i])
		if err != nil {
			return nil, err
		}
		currentData = nextData
	}
	return currentData, nil
}

func (a *App) extractFromFile(fsPath, entryPath string) ([]byte, error) {
	entryPath = normalize(entryPath)
	var buf []byte
	err := archiver.Walk(fsPath, func(f archiver.File) error {
		if normalize(f.Name()) == entryPath {
			buf, _ = io.ReadAll(f)
			return archiver.ErrStopWalk
		}
		return nil
	})
	if buf == nil && err == nil {
		return nil, fmt.Errorf("not found: %s", entryPath)
	}
	return buf, err
}

func (a *App) extractFromBytes(data []byte, entryPath string) ([]byte, error) {
	entryPath = normalize(entryPath)

	// Try ZIP
	if r, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
		for _, f := range r.File {
			if normalize(f.Name) == entryPath {
				rc, _ := f.Open()
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
	}

	// Try TAR
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if normalize(h.Name) == entryPath {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("entry not found in memory")
}

// ─── Shared Processing & Sorting ─────────────────────────────────

func (a *App) processEntry(f archiver.File, innerPath string, nestedChain []string, seen map[string]FileInfo) error {
	ep := normalize(f.Name())
	rel := getRelativePath(ep, innerPath)
	if rel == "" {
		return nil
	}

	parts := strings.SplitN(rel, "/", 2)
	childName := parts[0]
	isDir := f.IsDir() || len(parts) > 1
	isArchive := !isDir && isArchiveFile(childName)

	if _, ok := seen[childName]; ok {
		return nil
	}

	fi := FileInfo{
		Name:     childName,
		Path:     joinPath(innerPath, childName),
		Type:     "FILE",
		Size:     f.Size(),
		Modified: f.ModTime().UnixMilli(),
	}

	if isDir {
		fi.Type = "DIRECTORY"
	} else if isArchive {
		fi.Type = "ARCHIVE"
		fi.IsNested = true
		fi.NestedChain = append(append([]string{}, nestedChain...), childName)
	}

	seen[childName] = fi
	return nil
}

// archive.go

func (a *App) sortFileInfo(seen map[string]FileInfo) []FileInfo {
	entries := make([]FileInfo, 0, len(seen))
	for _, e := range seen {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		// Priority: Directory > Archive > File
		order := map[FileType]int{FileTypeDirectory: 0, FileTypeArchive: 1, FileTypeFile: 2}
		if entries[i].Type != entries[j].Type {
			return order[entries[i].Type] < order[entries[j].Type]
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}
