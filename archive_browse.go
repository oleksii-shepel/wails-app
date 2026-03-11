package main

import (
	"strings"
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
		return a.walkFileZip(fsPath, innerPath, nestedChain)
	}

	if needsArchiveReader(fsPath) {
		return a.walkFileArchiveReader(fsPath, innerPath, nestedChain)
	}
	return a.walkFileViaArchiver(fsPath, innerPath, nestedChain)
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
