package sax

import (
	"io"
	"strings"
)

// ExtractTextPath returns the plain text of a PPTX file on disk.
func ExtractTextPath(path string, opts Options) (string, error) {
	var ex textExtractor
	if err := WalkPath(path, ex.handler(), opts); err != nil {
		return "", err
	}
	return ex.String(), nil
}

// ExtractTextBytes returns the plain text of a PPTX file from an in-memory byte slice.
func ExtractTextBytes(data []byte, opts Options) (string, error) {
	var ex textExtractor
	if err := WalkBytes(data, ex.handler(), opts); err != nil {
		return "", err
	}
	return ex.String(), nil
}

// ExtractTextStream returns the plain text of a PPTX file from a non-seekable stream.
func ExtractTextStream(r io.Reader, opts Options) (string, error) {
	var ex textExtractor
	if err := WalkStream(r, ex.handler(), opts); err != nil {
		return "", err
	}
	return ex.String(), nil
}

// ExtractTextReaderAt returns the plain text of a PPTX file from a seekable reader.
func ExtractTextReaderAt(r io.ReaderAt, size int64, opts Options) (string, error) {
	var ex textExtractor
	if err := WalkReaderAt(r, size, ex.handler(), opts); err != nil {
		return "", err
	}
	return ex.String(), nil
}

type textExtractor struct {
	b              strings.Builder
	pendingTab     bool
	pendingNewline bool
}

func (ex *textExtractor) handler() HandlerFunc {
	return func(ev Event) error {
		switch ev.Type {
		case EventText:
			ex.flushPending()
			ex.b.WriteString(ev.Text)
		case EventBreak:
			ex.pendingNewline = true
		case EventTab:
			ex.pendingTab = true
		case EventEndParagraph:
			ex.pendingNewline = true
		case EventStartComment:
			// Comment text is carried on the start event in this walker.
			ex.flushPending()
			ex.b.WriteString(ev.Text)
		}
		return nil
	}
}

func (ex *textExtractor) flushPending() {
	if ex.pendingTab {
		ex.b.WriteByte('\t')
		ex.pendingTab = false
	}
	if ex.pendingNewline {
		ex.b.WriteByte('\n')
		ex.pendingNewline = false
	}
}

func (ex *textExtractor) String() string {
	ex.flushPending()
	return strings.TrimSpace(ex.b.String())
}
