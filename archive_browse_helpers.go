package main

import (
	"sort"
	"strings"
)

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
