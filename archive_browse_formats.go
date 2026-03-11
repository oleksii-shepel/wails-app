package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/mholt/archiver/v3"
)

func (a *App) walkFileZip(fsPath, innerPath string, nestedChain []string) ([]FileInfo, error) {
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

func (a *App) walkFileViaArchiver(fsPath, innerPath string, nestedChain []string) ([]FileInfo, error) {
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
