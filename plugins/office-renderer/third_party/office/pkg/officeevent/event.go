package officeevent

import (
	"errors"
	"io"
	"path/filepath"
	"strings"

	presax "office/pkg/presentation/parsers/ooxml/sax"
	"office/pkg/render/textsafe"
	sheetsax "office/pkg/sheet/parsers/ooxml/sax"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

// Format identifies the Office package family being streamed.
type Format string

const (
	FormatUnknown      Format = ""
	FormatWord         Format = "word"
	FormatPresentation Format = "presentation"
	FormatSpreadsheet  Format = "spreadsheet"
)

// ErrUnsupportedFormat reports that the input container is not currently
// supported by the shared Office event dispatcher.
var ErrUnsupportedFormat = errors.New("officeevent: unsupported streaming format")

// WordOptions controls format-specific event emission for Word documents.
type WordOptions = wordsax.Options

// PresentationOptions controls format-specific event emission for presentations.
type PresentationOptions = presax.Options

// SheetOptions controls format-specific event emission for spreadsheets.
type SheetOptions = sheetsax.Options

// Options controls the shared Office event dispatcher and embeds format-
// specific native options.
type Options struct {
	Word         WordOptions
	Presentation PresentationOptions
	Sheet        SheetOptions
}

// Event is a shared Office event envelope carrying the native format event
// payload unchanged alongside a common format/type header.
type Event struct {
	Format       Format
	Type         string
	PartName     string
	Text         string
	Story        string
	SheetIndex   int
	SheetID      int
	SheetName    string
	RowIndex     int
	ColIndex     int
	CellRef      string
	SlideIndex   int
	ShapeID      int
	ShapeName    string
	Word         *wordsax.Event
	Spreadsheet  *sheetsax.Event
	Presentation *presax.Event
}

// Handler consumes shared Office event envelopes.
type Handler interface {
	HandleEvent(Event) error
}

// HandlerFunc adapts a function to the shared Office handler interface.
type HandlerFunc func(Event) error

// HandleEvent implements Handler.
func (f HandlerFunc) HandleEvent(event Event) error {
	return f(event)
}

// WalkPath streams Office events from a file on disk, dispatching by extension.
func WalkPath(path string, handler Handler, opts Options) error {
	switch detectFormat(path) {
	case FormatWord:
		return wordsax.WalkPath(path, wordHandler{handler: handler}, opts.Word)
	case FormatSpreadsheet:
		return sheetsax.WalkPath(path, spreadsheetHandler{handler: handler}, opts.Sheet)
	case FormatPresentation:
		return presax.WalkPath(path, presentationHandler{handler: handler}, opts.Presentation)
	default:
		return ErrUnsupportedFormat
	}
}

// WalkBytes streams Office events from an in-memory OOXML payload. name should
// include the original file extension so the dispatcher can choose a format.
func WalkBytes(name string, data []byte, handler Handler, opts Options) error {
	switch detectFormat(name) {
	case FormatWord:
		return wordsax.WalkBytes(data, wordHandler{handler: handler}, opts.Word)
	case FormatSpreadsheet:
		return sheetsax.WalkBytes(data, spreadsheetHandler{handler: handler}, opts.Sheet)
	case FormatPresentation:
		return presax.WalkBytes(data, presentationHandler{handler: handler}, opts.Presentation)
	default:
		return ErrUnsupportedFormat
	}
}

// WalkStream streams Office events from a non-seekable OOXML stream. name
// should include the original file extension so the dispatcher can choose a format.
func WalkStream(name string, r io.Reader, handler Handler, opts Options) error {
	switch detectFormat(name) {
	case FormatWord:
		return wordsax.WalkStream(r, wordHandler{handler: handler}, opts.Word)
	case FormatSpreadsheet:
		return sheetsax.WalkStream(r, spreadsheetHandler{handler: handler}, opts.Sheet)
	case FormatPresentation:
		return presax.WalkStream(r, presentationHandler{handler: handler}, opts.Presentation)
	default:
		return ErrUnsupportedFormat
	}
}

// WalkReaderAt streams Office events from a seekable OOXML reader. name should
// include the original file extension so the dispatcher can choose a format.
func WalkReaderAt(name string, r io.ReaderAt, size int64, handler Handler, opts Options) error {
	switch detectFormat(name) {
	case FormatWord:
		return wordsax.WalkReaderAt(r, size, wordHandler{handler: handler}, opts.Word)
	case FormatSpreadsheet:
		return sheetsax.WalkReaderAt(r, size, spreadsheetHandler{handler: handler}, opts.Sheet)
	case FormatPresentation:
		return presax.WalkReaderAt(r, size, presentationHandler{handler: handler}, opts.Presentation)
	default:
		return ErrUnsupportedFormat
	}
}

type wordHandler struct {
	handler Handler
}

func (h wordHandler) HandleEvent(ev wordsax.Event) error {
	if h.handler == nil {
		return nil
	}
	copyEv := ev
	return h.handler.HandleEvent(Event{
		Format:   FormatWord,
		Type:     string(ev.Type),
		PartName: ev.PartName,
		Text:     textsafe.Visible(ev.Text),
		Story:    string(ev.Story),
		Word:     &copyEv,
	})
}

type spreadsheetHandler struct {
	handler Handler
}

func (h spreadsheetHandler) HandleEvent(ev sheetsax.Event) error {
	if h.handler == nil {
		return nil
	}
	copyEv := ev
	return h.handler.HandleEvent(Event{
		Format:      FormatSpreadsheet,
		Type:        string(ev.Type),
		PartName:    ev.PartName,
		Text:        textsafe.Visible(ev.Text),
		SheetIndex:  ev.SheetIndex,
		SheetID:     ev.SheetID,
		SheetName:   ev.SheetName,
		RowIndex:    ev.RowIndex,
		ColIndex:    ev.ColIndex,
		CellRef:     ev.CellRef,
		Spreadsheet: &copyEv,
	})
}

type presentationHandler struct {
	handler Handler
}

func (h presentationHandler) HandleEvent(ev presax.Event) error {
	if h.handler == nil {
		return nil
	}
	copyEv := ev
	return h.handler.HandleEvent(Event{
		Format:       FormatPresentation,
		Type:         string(ev.Type),
		PartName:     ev.PartName,
		Text:         textsafe.Visible(ev.Text),
		SlideIndex:   ev.SlideIndex,
		ShapeID:      ev.ShapeID,
		ShapeName:    ev.ShapeName,
		Presentation: &copyEv,
	})
}

func detectFormat(name string) Format {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".docx", ".docm", ".dotx", ".dotm":
		return FormatWord
	case ".xlsx", ".xlsm", ".xltx", ".xltm", ".xlam":
		return FormatSpreadsheet
	case ".pptx", ".pptm", ".ppsx", ".ppsm", ".potx", ".potm":
		return FormatPresentation
	default:
		return FormatUnknown
	}
}
