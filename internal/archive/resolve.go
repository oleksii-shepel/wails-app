package archive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	vfsarchive "wails-app/vfs/archive"
)

// ResolveChain opens a possibly nested archive chain and returns the innermost
// archive ready for browsing.
func ResolveChain(ctx context.Context, fsPath string, nestedChain []string, password string) (vfsarchive.Archive, error) {
	handler, ok := vfsarchive.ForPath(fsPath)
	if !ok {
		return nil, fmt.Errorf("no archive handler for %q", fsPath)
	}

	if len(nestedChain) == 0 {
		return handler.Open(ctx, fsPath, vfsarchive.OpenOptions{ReadOnly: true, Password: password})
	}

	outer, err := handler.Open(ctx, fsPath, vfsarchive.OpenOptions{ReadOnly: true, Password: password})
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", fsPath, err)
	}

	data, err := readArchiveEntryBytes(ctx, outer, nestedChain[0])
	outer.Close()
	if err != nil {
		return nil, fmt.Errorf("extract %q from %q: %w", nestedChain[0], fsPath, err)
	}

	for index := 1; index < len(nestedChain); index++ {
		currentName := nestedChain[index-1]
		mid, err := openArchiveFromMemory(ctx, currentName, data, password)
		if err != nil {
			return nil, fmt.Errorf("open level %d %q from bytes: %w", index, currentName, err)
		}
		next, err := readArchiveEntryBytes(ctx, mid, nestedChain[index])
		mid.Close()
		if err != nil {
			return nil, fmt.Errorf("extract %q from level %d (%q): %w", nestedChain[index], index, currentName, err)
		}
		data = next
	}

	return openArchiveFromMemory(ctx, nestedChain[len(nestedChain)-1], data, password)
}

func openArchiveFromMemory(ctx context.Context, name string, data []byte, password string) (vfsarchive.Archive, error) {
	handler, ok := vfsarchive.ForPath(name)
	if !ok {
		return nil, fmt.Errorf("no archive handler for %q", name)
	}

	tempFile, err := os.CreateTemp("", "vfs-nested-*"+detectArchiveExt(name))
	if err != nil {
		return nil, fmt.Errorf("temp file for %q: %w", name, err)
	}
	tempPath := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return nil, fmt.Errorf("write temp %q: %w", name, err)
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("close temp %q: %w", name, err)
	}

	arch, err := handler.Open(ctx, tempPath, vfsarchive.OpenOptions{ReadOnly: true, Password: password})
	if err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("open %q from temp: %w", name, err)
	}
	return &tempArchive{Archive: arch, tmp: tempPath}, nil
}

func readArchiveEntryBytes(ctx context.Context, arch vfsarchive.Archive, entryPath string) ([]byte, error) {
	file, err := arch.Open(ctx, normalizePath(entryPath))
	if err != nil {
		return nil, fmt.Errorf("open entry %q: %w", entryPath, err)
	}
	defer file.Close()
	return io.ReadAll(file)
}

type tempArchive struct {
	vfsarchive.Archive
	tmp string
}

func (t *tempArchive) Close() error {
	err := t.Archive.Close()
	os.Remove(t.tmp)
	return err
}

func detectArchiveExt(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return ".tar.gz"
	case strings.HasSuffix(lower, ".tar.bz2"):
		return ".tar.bz2"
	case strings.HasSuffix(lower, ".tar.xz"):
		return ".tar.xz"
	case strings.HasSuffix(lower, ".tgz"):
		return ".tgz"
	case strings.HasSuffix(lower, ".tbz2"):
		return ".tbz2"
	default:
		return filepath.Ext(name)
	}
}
