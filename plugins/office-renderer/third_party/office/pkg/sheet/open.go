package sheet

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"office/pkg/ooxml/opc"
	sheetedit "office/pkg/sheet/edit"
	sheetlegacy "office/pkg/sheet/parsers/legacy"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
	sheetresolve "office/pkg/sheet/parsers/ooxml/resolve"
	sheetrender "office/pkg/sheet/render"
	sheetstream "office/pkg/sheet/stream"
)

// Format identifies the spreadsheet container family detected by the high-level API.
type Format string

const (
	FormatUnknown Format = ""
	FormatXLSX    Format = "xlsx"
	FormatXLS     Format = "xls"
)

// Workbook is the resolved, format-neutral spreadsheet model returned by Open and OpenBytes.
type Workbook = sheetresolve.Workbook

// HTMLOptions configures high-level HTML rendering.
type HTMLOptions = sheetrender.HTMLOptions

// HTMLAsset is a sidecar asset produced by HTMLWithOptions.
type HTMLAsset = sheetrender.HTMLAsset

// StreamOptions configures bounded-memory XLSX writing.
type StreamOptions = sheetstream.Options

// DetectFormat identifies XLSX-family ZIP packages and legacy XLS/BIFF payloads.
// Unknown non-ZIP payloads are classified as XLS only after the legacy parser is attempted by OpenBytes.
func DetectFormat(data []byte) Format {
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'P', 'K', 0x03, 0x04}) {
		return FormatXLSX
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		return FormatXLS
	}
	return FormatUnknown
}

// IsSpreadsheetExtension reports whether name has a supported spreadsheet extension.
func IsSpreadsheetExtension(name string) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(name))) {
	case ".xlsx", ".xlsm", ".xltx", ".xltm", ".xls":
		return true
	default:
		return false
	}
}

// Open loads an XLSX-family or legacy XLS workbook from path into the resolved IR.
func Open(path string) (*Workbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return OpenBytes(data, filepath.Base(path))
}

// OpenBytes loads an XLSX-family or legacy XLS workbook from memory.
// filename is optional and is used for legacy provenance and diagnostics.
func OpenBytes(data []byte, filename ...string) (*Workbook, error) {
	name := "workbook"
	if len(filename) > 0 && strings.TrimSpace(filename[0]) != "" {
		name = filename[0]
	}

	if DetectFormat(data) == FormatXLSX {
		reader := bytes.NewReader(data)
		pkg, err := opc.OpenReaderAt(reader, int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("sheet: open XLSX package: %w", err)
		}
		parsed, err := sheetparts.LoadPackage(pkg)
		if err != nil {
			return nil, fmt.Errorf("sheet: load XLSX package: %w", err)
		}
		wb, err := sheetresolve.ResolvePackage(parsed)
		if err != nil {
			return nil, fmt.Errorf("sheet: resolve XLSX workbook: %w", err)
		}
		return wb, nil
	}

	wb, err := sheetlegacy.ParseXlsBytes(data, name)
	if err != nil {
		return nil, fmt.Errorf("sheet: unsupported or invalid spreadsheet payload: %w", err)
	}
	return wb, nil
}

// New returns an empty editable workbook.
func New() *sheetedit.Workbook {
	return sheetedit.NewWorkbook()
}

// Edit wraps a resolved workbook in the in-memory editing API.
func Edit(wb *Workbook) (*sheetedit.Workbook, error) {
	return sheetedit.OpenIR(wb)
}

// HTML renders a workbook as a complete HTML document.
func HTML(wb *Workbook) (string, error) {
	return sheetrender.HTML(wb)
}

// HTMLWithOptions renders a workbook as HTML and returns any sidecar assets.
func HTMLWithOptions(wb *Workbook, opts HTMLOptions) (string, []HTMLAsset, error) {
	return sheetrender.HTMLWithOptions(wb, opts)
}

// NewStreamWriter creates the bounded-memory XLSX writer.
// Call Close on the returned writer to finalize the package.
func NewStreamWriter(out io.Writer, opts StreamOptions) (*sheetstream.Workbook, error) {
	return sheetstream.New(out, opts)
}
