package stream

import (
	"archive/zip"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type docxMediaLoader struct {
	files map[string]*zip.File
	cache map[string][]byte
}

func newDOCXMediaLoader(r io.ReaderAt, size int64) (*docxMediaLoader, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, file := range zr.File {
		files[normalizeDOCXPartName(file.Name)] = file
	}
	return &docxMediaLoader{files: files, cache: make(map[string][]byte)}, nil
}

func (l *docxMediaLoader) Load(target string) ([]byte, string, error) {
	if l == nil || target == "" {
		return nil, "", nil
	}
	name := normalizeDOCXPartName(target)
	if data, ok := l.cache[name]; ok {
		return data, mimeTypeFromDOCXPart(name), nil
	}
	file := l.files[name]
	if file == nil {
		return nil, "", fmt.Errorf("word stream: drawing media part %q not found", target)
	}
	rc, err := file.Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", err
	}
	l.cache[name] = data
	return data, mimeTypeFromDOCXPart(name), nil
}

func mimeTypeFromDOCXPart(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".tif", ".tiff":
		return "image/tiff"
	case ".svg":
		return "image/svg+xml"
	case ".wmf":
		return "image/x-wmf"
	case ".emf":
		return "image/x-emf"
	default:
		return "application/octet-stream"
	}
}
