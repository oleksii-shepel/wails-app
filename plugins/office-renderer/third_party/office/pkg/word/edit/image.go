package edit

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	wordir "office/pkg/word/ir"
)

// AddImage attaches an inline image to this run. The caller must supply the
// raw image data, MIME type, and dimensions in EMU (914400 EMU = 1 inch).
func (r *Run) AddImage(data []byte, mimeType string, widthEMU, heightEMU int64) {
	if ir := r.ir(); ir != nil {
		ir.Kind = "image"
		ir.Image = &wordir.Image{
			MIMEType: mimeType,
			Data:     data,
			Width:    widthEMU,
			Height:   heightEMU,
		}
	}
}

// AddImageFromFile reads an image file and attaches it to this run.
// Dimensions are converted to EMU assuming 96 DPI.
func (r *Run) AddImageFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("edit: read image file: %w", err)
	}

	mimeType := mimeTypeFromPath(path)
	if mimeType == "" {
		mimeType = "image/png"
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("edit: decode image config: %w", err)
	}

	// Convert pixels at 96 DPI to EMU.
	const emuPerInch int64 = 914400
	widthEMU := int64(cfg.Width) * emuPerInch / 96
	heightEMU := int64(cfg.Height) * emuPerInch / 96

	r.AddImage(data, mimeType, widthEMU, heightEMU)
	return nil
}

func mimeTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".wmf":
		return "image/x-wmf"
	case ".emf":
		return "image/x-emf"
	default:
		return ""
	}
}