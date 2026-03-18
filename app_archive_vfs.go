package main

import (
	"context"
	"fmt"

	vfslib "wails-app/vfs"
)

// vfsBrowseArchive delegates archive listing to whichever VFS handler is
// registered for the given path (zip, tar, rar, 7z, ...). Archives are opened
// once and kept alive in archCache; subsequent calls for any innerPath within
// the same (fsPath, nestedChain) identity reuse the cached archive.
func (a *App) vfsBrowseArchive(fsPath string, nestedChain []string, innerPath string) (ArchiveListing, error) {
	ctx := context.Background()

	arch, err := a.archCache.Get(ctx, fsPath, nestedChain, "")
	if err != nil {
		return ArchiveListing{}, fmt.Errorf("vfs: open %q: %w", fsPath, err)
	}

	vfsEntries, err := arch.ReadDir(ctx, normalize(innerPath))
	if err != nil {
		return ArchiveListing{}, fmt.Errorf("vfs: readdir %q in %q: %w", innerPath, fsPath, err)
	}

	entries := make([]FileInfo, 0, len(vfsEntries))
	for _, entry := range vfsEntries {
		entries = append(entries, vfsEntryToFileInfo(entry, nestedChain))
	}

	return ArchiveListing{
		ArchivePath: fsPath,
		InnerPath:   innerPath,
		NestedChain: nestedChain,
		Entries:     entries,
	}, nil
}

// vfsEntryToFileInfo converts a vfs.FileInfo into the app wire FileInfo while
// preserving the nested archive chain the frontend needs to reopen child archives.
func vfsEntryToFileInfo(entry vfslib.FileInfo, parentChain []string) FileInfo {
	entryType := FileTypeFile
	switch {
	case entry.IsDir():
		entryType = FileTypeDirectory
	case entry.IsArchive():
		entryType = FileTypeArchive
	case entry.IsSymlink():
		entryType = FileTypeSymlink
	}

	var nestedChain []string
	if entryType == FileTypeArchive {
		nestedChain = append(append([]string{}, parentChain...), entry.Path())
	}

	return FileInfo{
		Name:        entry.Name(),
		Path:        entry.Path(),
		Type:        entryType,
		Size:        entry.Size(),
		Modified:    entry.ModTime().UnixMilli(),
		IsHidden:    entry.IsHidden(),
		IsSymlink:   entry.IsSymlink(),
		IsNested:    entryType == FileTypeArchive,
		NestedChain: nestedChain,
	}
}