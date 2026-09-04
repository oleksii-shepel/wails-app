package presentation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"office/pkg/presentation/edit"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/legacy"
	"office/pkg/presentation/render"
)

// Format identifies a supported PowerPoint container family.
type Format string

const (
	FormatPPTX Format = "pptx"
	FormatPPT  Format = "ppt"
)

// DetectFormat detects PPTX/OPC and legacy PPT/CFB containers from their signatures.
func DetectFormat(data []byte) (Format, error) {
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'P', 'K', 0x03, 0x04}) {
		return FormatPPTX, nil
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return FormatPPT, nil
	}
	return "", fmt.Errorf("presentation: unsupported or unrecognized file format")
}

// Open reads a .pptx/.ppt presentation and returns the common semantic IR.
func Open(path string) (*ir.Presentation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("presentation: open %q: %w", path, err)
	}
	return OpenBytesNamed(data, filepath.Base(path))
}

// OpenBytes reads a PPTX or legacy PPT presentation from memory.
func OpenBytes(data []byte) (*ir.Presentation, error) { return OpenBytesNamed(data, "") }

// OpenBytesNamed is OpenBytes with an optional source filename for metadata.
func OpenBytesNamed(data []byte, filename string) (*ir.Presentation, error) {
	format, err := DetectFormat(data)
	if err != nil {
		return nil, err
	}
	switch format {
	case FormatPPTX:
		doc, err := edit.LoadFromPPTXBytes(data)
		if err != nil {
			return nil, fmt.Errorf("presentation: parse pptx: %w", err)
		}
		return doc.IR, nil
	case FormatPPT:
		doc, err := legacy.ParsePptBytes(data, filename)
		if err != nil {
			return nil, fmt.Errorf("presentation: parse ppt: %w", err)
		}
		return doc, nil
	default:
		return nil, fmt.Errorf("presentation: unsupported format %q", format)
	}
}

// New creates a blank editable presentation.
func New() *edit.Presentation { return edit.NewPresentation() }

// Edit wraps a semantic presentation IR in the editing API.
func Edit(doc *ir.Presentation) *edit.Presentation { return edit.OpenIR(doc) }

// HTML renders a semantic presentation as self-contained HTML.
func HTML(doc *ir.Presentation) (string, error) { return render.HTML(doc) }

// IsPowerPointExtension reports whether name has a supported PowerPoint extension.
func IsPowerPointExtension(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".ppt", ".pptx", ".pptm", ".potx", ".potm":
		return true
	default:
		return false
	}
}
