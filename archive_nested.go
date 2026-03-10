package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mholt/archiver/v3"
)

// ArchiveReadFile extracts a single entry from an archive into memory.
func (a *App) archiveReadFile(fsPath string, nestedChain []string, entryPath string, encoding string) (*FileContent, error) {
	entryPath = normalize(entryPath)

	var data []byte
	var err error

	if len(nestedChain) == 0 {
		data, err = a.extractFromFile(fsPath, entryPath)
	} else {
		archiveData, nestedErr := a.extractNestedArchive(fsPath, nestedChain)
		if nestedErr != nil {
			return nil, nestedErr
		}
		data, err = a.extractFromBytes(archiveData, entryPath, nestedChain[len(nestedChain)-1])
	}
	if err != nil {
		return nil, err
	}

	if encoding == "" {
		encoding = "utf8"
	}
	var content string
	if encoding == "base64" {
		content = base64.StdEncoding.EncodeToString(data)
	} else {
		content = string(data)
		encoding = "utf8"
	}

	return &FileContent{
		Path:     entryPath,
		Content:  content,
		Encoding: encoding,
		Size:     int64(len(data)),
	}, nil
}

func (a *App) extractNestedArchive(fsPath string, nestedChain []string) ([]byte, error) {
	if len(nestedChain) == 0 {
		return nil, fmt.Errorf("empty nested chain")
	}

	current, err := a.extractFromFile(fsPath, nestedChain[0])
	if err != nil {
		return nil, fmt.Errorf("extracting %q from %q: %w", nestedChain[0], fsPath, err)
	}

	for index := 1; index < len(nestedChain); index += 1 {
		next, err := a.extractFromBytes(current, nestedChain[index], nestedChain[index])
		if err != nil {
			return nil, fmt.Errorf("extracting %q (chain depth %d): %w", nestedChain[index], index, err)
		}
		current = next
	}
	return current, nil
}

func (a *App) extractFromFile(fsPath, entryPath string) ([]byte, error) {
	want := normalize(entryPath)
	lower := strings.ToLower(fsPath)

	if strings.HasSuffix(lower, ".zip") {
		r, err := zip.OpenReader(fsPath)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		for _, f := range r.File {
			if normalize(f.Name) == want {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("entry %q not found in %q", entryPath, fsPath)
	}

	if needsArchiveReader(fsPath) {
		return a.extractFromFileArchiveReader(fsPath, entryPath)
	}

	var buf []byte
	err := archiver.Walk(fsPath, func(f archiver.File) error {
		if normalize(archiveEntryPath(f)) == want {
			var readErr error
			buf, readErr = io.ReadAll(f)
			if readErr != nil {
				return readErr
			}
			return archiver.ErrStopWalk
		}
		return nil
	})
	if err != nil && err != archiver.ErrStopWalk {
		return nil, err
	}
	if buf == nil {
		return nil, fmt.Errorf("entry %q not found in %q", entryPath, fsPath)
	}
	return buf, nil
}

func (a *App) extractFromFileArchiveReader(fsPath, entryPath string) ([]byte, error) {
	want := normalize(entryPath)

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
		return nil, fmt.Errorf("extractFromFileArchiveReader: %w", err)
	}
	reader, ok := aIface.(archiver.Reader)
	if !ok {
		return nil, fmt.Errorf("extractFromFileArchiveReader: format %q is not readable", fsPath)
	}
	if err := reader.Open(f, fi.Size()); err != nil {
		return nil, err
	}
	defer reader.Close()

	for {
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalize(archiveEntryPath(entry)) == want {
			buf, readErr := io.ReadAll(entry)
			entry.Close()
			if readErr != nil {
				return nil, readErr
			}
			return buf, nil
		}
		entry.Close()
	}
	return nil, fmt.Errorf("entry %q not found in %q", entryPath, fsPath)
}

func (a *App) extractFromBytes(data []byte, entryPath string, archiveName string) ([]byte, error) {
	want := normalize(entryPath)

	if isZipMagic(data) {
		r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err == nil {
			for _, f := range r.File {
				if normalize(f.Name) == want {
					rc, err := f.Open()
					if err != nil {
						return nil, err
					}
					defer rc.Close()
					return io.ReadAll(rc)
				}
			}
			return nil, fmt.Errorf("entry %q not found in ZIP", entryPath)
		}
	}

	if archiveName != "" && needsArchiveReader(archiveName) {
		return a.extractFromArchiveReader(data, archiveName, entryPath)
	}

	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalize(h.Name) == want {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("entry %q not found in archive", entryPath)
}

func (a *App) extractFromArchiveReader(data []byte, archiveName, entryPath string) ([]byte, error) {
	reader, err := openArchiveReader(archiveName, data)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	want := normalize(entryPath)

	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalize(archiveEntryPath(f)) == want {
			buf, readErr := io.ReadAll(f)
			f.Close()
			if readErr != nil {
				return nil, readErr
			}
			return buf, nil
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("entry %q not found in %s", entryPath, archiveName)
}
