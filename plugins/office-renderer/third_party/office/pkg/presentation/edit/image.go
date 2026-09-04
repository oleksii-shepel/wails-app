package edit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	presir "office/pkg/presentation/ir"
)

// readImageFile reads image file bytes and returns a MIME type based on extension.
func readImageFile(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("edit: read image %q: %w", path, err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	var mime string
	switch ext {
	case ".png":
		mime = "image/png"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	case ".bmp":
		mime = "image/bmp"
	case ".webp":
		mime = "image/webp"
	default:
		mime = "image/png"
	}
	return data, mime, nil
}

// SetImageData replaces the bitmap payload of an existing image shape.
// It is a no-op for non-image shapes.
func (sh *Shape) SetImageData(data []byte, mimeType string) {
	if sh == nil || sh.ir == nil || sh.ir.Kind != presir.ShapeImage {
		return
	}
	sh.ir.ImageData = append([]byte(nil), data...)
	sh.ir.ImageMIME = mimeType
}

// SetImageFromFile replaces the bitmap payload of an existing image shape
// with the contents of path. It returns an error for unreadable files.
func (sh *Shape) SetImageFromFile(path string) error {
	if sh == nil || sh.ir == nil || sh.ir.Kind != presir.ShapeImage {
		return fmt.Errorf("edit: shape is not an image")
	}
	data, mimeType, err := readImageFile(path)
	if err != nil {
		return err
	}
	sh.SetImageData(data, mimeType)
	return nil
}
