package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/mholt/archiver/v3"
)

func (s *Service) ExtractNestedArchive(fsPath string, nestedChain []string) ([]byte, error) {
	if len(nestedChain) == 0 {
		return nil, fmt.Errorf("empty nested chain")
	}

	current, err := s.extractFromFile(fsPath, nestedChain[0])
	if err != nil {
		return nil, fmt.Errorf("extracting %q from %q: %w", nestedChain[0], fsPath, err)
	}

	for index := 1; index < len(nestedChain); index++ {
		next, err := s.extractFromBytes(current, nestedChain[index], nestedChain[index])
		if err != nil {
			return nil, fmt.Errorf("extracting %q (chain depth %d): %w", nestedChain[index], index, err)
		}
		current = next
	}
	return current, nil
}

func (s *Service) extractFromFile(fsPath, entryPath string) ([]byte, error) {
	want := normalizePath(entryPath)

	if isZipArchiveName(fsPath) {
		return s.extractFromFileZip(fsPath, want)
	}
	if needsArchiveReader(fsPath) {
		return s.extractFromFileArchiveReader(fsPath, want)
	}
	return s.extractFromFileViaArchiver(fsPath, want)
}

func (s *Service) extractFromFileZip(fsPath, want string) ([]byte, error) {
	r, err := zip.OpenReader(fsPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		if normalizePath(f.Name) == want {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("entry %q not found in %q", want, fsPath)
}

func (s *Service) extractFromFileViaArchiver(fsPath, want string) ([]byte, error) {
	var buf []byte
	err := archiver.Walk(fsPath, func(f archiver.File) error {
		if normalizePath(archiveEntryPath(f)) == want {
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
		return nil, fmt.Errorf("entry %q not found in %q", want, fsPath)
	}
	return buf, nil
}

func (s *Service) extractFromFileArchiveReader(fsPath, want string) ([]byte, error) {
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
		if normalizePath(archiveEntryPath(entry)) == want {
			buf, readErr := io.ReadAll(entry)
			entry.Close()
			if readErr != nil {
				return nil, readErr
			}
			return buf, nil
		}
		entry.Close()
	}
	return nil, fmt.Errorf("entry %q not found in %q", want, fsPath)
}

func (s *Service) extractFromBytes(data []byte, entryPath string, archiveName string) ([]byte, error) {
	want := normalizePath(entryPath)

	if isZipMagic(data) {
		return s.extractFromZipBytes(data, want)
	}
	if archiveName != "" && needsArchiveReader(archiveName) {
		return s.extractFromArchiveReader(data, archiveName, want)
	}
	return s.extractFromTarBytes(data, want)
}

func (s *Service) extractFromZipBytes(data []byte, want string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if normalizePath(f.Name) == want {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("entry %q not found in ZIP", want)
}

func (s *Service) extractFromTarBytes(data []byte, want string) ([]byte, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalizePath(h.Name) == want {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("entry %q not found in archive", want)
}

func (s *Service) extractFromArchiveReader(data []byte, archiveName, want string) ([]byte, error) {
	reader, err := openArchiveReader(archiveName, data)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	for {
		f, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if normalizePath(archiveEntryPath(f)) == want {
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

	return nil, fmt.Errorf("entry %q not found in %s", want, archiveName)
}