package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func rewriteArchiveByFormat(fsPath string, entries []string, op *ActiveOperation) (removed, kept int, err error) {
	lower := strings.ToLower(fsPath)
	switch {
	case isZipArchiveName(fsPath):
		return rewriteZip(fsPath, entries, op)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return rewriteTar(fsPath, entries, "gz", op)
	case strings.HasSuffix(lower, ".tar.bz2"):
		return rewriteTar(fsPath, entries, "bz2", op)
	case strings.HasSuffix(lower, ".tar.xz"):
		return rewriteTar(fsPath, entries, "xz", op)
	case strings.HasSuffix(lower, ".tar"):
		return rewriteTar(fsPath, entries, "", op)
	case needsArchiveReader(fsPath):
		return 0, 0, unsupportedRewriteFormatError(fsPath)
	default:
		return 0, 0, fmt.Errorf("unsupported archive format: %q", fsPath)
	}
}

func unsupportedRewriteFormatError(fsPath string) error {
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(strings.ToLower(fsPath)), "."))
	if ext == "" {
		ext = "UNKNOWN"
	}
	return fmt.Errorf("deleting entries from %s archives is not supported (format is read-only)", ext)
}
