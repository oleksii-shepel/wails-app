package word

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"office/pkg/ooxml/opc"
	"office/pkg/word/edit"
	"office/pkg/word/ir"
	"office/pkg/word/parsers/legacy"
	"office/pkg/word/parsers/ooxml/parts"
	"office/pkg/word/parsers/ooxml/resolve"
	"office/pkg/word/render"
)

// Format identifies the Word container family used by an input document.
type Format string

const (
	// FormatUnknown means the input cannot be classified as a supported Word
	// document from its name/container signature.
	FormatUnknown Format = ""
	// FormatDOCX represents OPC/WordprocessingML documents, including .docx,
	// .docm, .dotx, and .dotm files.
	FormatDOCX Format = "docx"
	// FormatDOC represents legacy binary Word .doc files.
	FormatDOC Format = "doc"
)

// ErrUnsupportedFormat is returned when an input cannot be classified as a
// supported Word document container.
var ErrUnsupportedFormat = errors.New("word: unsupported document format")

// Document is the stable format-neutral Word intermediate representation.
type Document = ir.Document

// Editor is the format-neutral mutation API for a Word document.
type Editor = edit.Document

// HTMLOptions controls HTML rendering.
type HTMLOptions = render.HTMLOptions

// HTMLAsset is a sidecar asset produced by HTMLWithOptions.
type HTMLAsset = render.HTMLAsset

// HTMLImageMode controls whether rendered images are inline or external.
type HTMLImageMode = render.HTMLImageMode

// HTMLFontMode controls whether rendered fonts are inline or external.
type HTMLFontMode = render.HTMLFontMode

// MathMLOptions controls MathML rendering.
type MathMLOptions = render.MathMLOptions

const (
	HTMLImageModeInline   = render.HTMLImageModeInline
	HTMLImageModeExternal = render.HTMLImageModeExternal
	HTMLFontModeInline    = render.HTMLFontModeInline
	HTMLFontModeExternal  = render.HTMLFontModeExternal
)

// DetectFormat classifies a Word document using its filename and, when
// necessary, its container signature. Filename extensions take precedence so
// encrypted OOXML files (which may use a compound-file envelope) are not
// mistaken for legacy .doc files.
func DetectFormat(name string, data []byte) Format {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".docx", ".docm", ".dotx", ".dotm":
		return FormatDOCX
	case ".doc":
		return FormatDOC
	}
	if looksLikeZIP(data) {
		return FormatDOCX
	}
	if looksLikeCFB(data) {
		return FormatDOC
	}
	return FormatUnknown
}

// Open reads and resolves a Word document from path into the shared semantic
// IR. Legacy DOC files remain on their native lossless IR path so binary
// provenance needed by layout/rendering is preserved.
func Open(path string) (*Document, error) {
	format := DetectFormat(path, nil)
	switch format {
	case FormatDOCX:
		pkg, err := opc.Open(path)
		if err != nil {
			return nil, fmt.Errorf("word: open OOXML %q: %w", path, err)
		}
		return resolveOPCPackage(pkg, path)
	case FormatDOC:
		doc, err := legacy.ParseDocLossless(path)
		if err != nil {
			return nil, fmt.Errorf("word: open DOC %q: %w", path, err)
		}
		return doc, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("word: open %q: %w", path, err)
	}
	return OpenBytes(filepath.Base(path), data)
}

// OpenBytes resolves an in-memory Word document. name is used for format
// selection and diagnostics and should normally include the original file
// extension.
func OpenBytes(name string, data []byte) (*Document, error) {
	switch DetectFormat(name, data) {
	case FormatDOCX:
		pkg, err := opc.OpenBytes(data)
		if err != nil {
			return nil, fmt.Errorf("word: open OOXML %q: %w", displayName(name), err)
		}
		return resolveOPCPackage(pkg, "")
	case FormatDOC:
		doc, err := legacy.ParseDocBytesLossless(data, name)
		if err != nil {
			return nil, fmt.Errorf("word: open DOC %q: %w", displayName(name), err)
		}
		return doc, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedFormat, displayName(name))
	}
}

// ConvertToDOCX opens a Word document from path and writes a DOCX package to w.
// Legacy DOC input is parsed into the native lossless IR first; conversion to
// OOXML happens only at this explicit export boundary.
func ConvertToDOCX(path string, w io.Writer) error {
	doc, err := Open(path)
	if err != nil {
		return err
	}
	if err := Edit(doc).SaveAsDOCX(w); err != nil {
		return fmt.Errorf("word: convert %q to DOCX: %w", path, err)
	}
	return nil
}

// ConvertBytesToDOCX converts an in-memory Word document to DOCX. name should
// normally contain the original extension so the input format can be selected.
func ConvertBytesToDOCX(name string, data []byte, w io.Writer) error {
	doc, err := OpenBytes(name, data)
	if err != nil {
		return err
	}
	if err := Edit(doc).SaveAsDOCX(w); err != nil {
		return fmt.Errorf("word: convert %q to DOCX: %w", displayName(name), err)
	}
	return nil
}

// New creates an empty editable Word document.
func New() *Editor { return edit.NewDocument() }

// Edit wraps a semantic Word document in the mutation API. A nil document
// produces a new empty editor.
func Edit(doc *Document) *Editor { return edit.OpenIR(doc) }

// HTML renders a Word document to self-contained HTML.
func HTML(doc *Document) (string, error) { return render.HTML(doc) }

// HTMLWithOptions renders a Word document and returns optional sidecar assets.
func HTMLWithOptions(doc *Document, opts HTMLOptions) (string, []HTMLAsset, error) {
	return render.HTMLWithOptions(doc, opts)
}

// MathML extracts and renders MathML content from the document and its stories.
func MathML(doc *Document, opts MathMLOptions) (string, error) {
	return render.MathML(doc, opts)
}

func resolveOPCPackage(pkg *opc.Package, sourcePath string) (*Document, error) {
	typed, err := parts.LoadPackage(pkg)
	if err != nil {
		return nil, fmt.Errorf("word: load WordprocessingML parts: %w", err)
	}
	typed.SourcePath = sourcePath
	doc, err := resolve.ResolvePackage(typed)
	if err != nil {
		return nil, fmt.Errorf("word: resolve WordprocessingML: %w", err)
	}
	return doc, nil
}

func looksLikeZIP(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return string(data[:4]) == "PK\x03\x04" || string(data[:4]) == "PK\x05\x06" || string(data[:4]) == "PK\x07\x08"
}

func looksLikeCFB(data []byte) bool {
	cfbSignature := [...]byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}
	if len(data) < len(cfbSignature) {
		return false
	}
	for i, b := range cfbSignature {
		if data[i] != b {
			return false
		}
	}
	return true
}

func displayName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "document"
	}
	return name
}
