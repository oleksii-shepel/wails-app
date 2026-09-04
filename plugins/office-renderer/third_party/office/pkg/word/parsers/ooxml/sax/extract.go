package sax

import (
	"io"
	"strings"
)

// ExtractTextPath extracts plain text from a DOCX file on disk using the SAX walker.
func ExtractTextPath(path string, opts Options) (string, error) {
	var extractor textExtractor
	if err := WalkPath(path, &extractor, opts); err != nil {
		return "", err
	}
	return extractor.String(), nil
}

// ExtractTextBytes extracts plain text from an in-memory DOCX byte slice using the SAX walker.
func ExtractTextBytes(data []byte, opts Options) (string, error) {
	var extractor textExtractor
	if err := WalkBytes(data, &extractor, opts); err != nil {
		return "", err
	}
	return extractor.String(), nil
}

// ExtractTextStream extracts plain text from a non-seekable DOCX stream using the SAX walker.
func ExtractTextStream(r io.Reader, opts Options) (string, error) {
	var extractor textExtractor
	if err := WalkStream(r, &extractor, opts); err != nil {
		return "", err
	}
	return extractor.String(), nil
}

// ExtractTextReaderAt extracts plain text from a DOCX reader/size pair using the SAX walker.
func ExtractTextReaderAt(r io.ReaderAt, size int64, opts Options) (string, error) {
	var extractor textExtractor
	if err := WalkReaderAt(r, size, &extractor, opts); err != nil {
		return "", err
	}
	return extractor.String(), nil
}

type textExtractor struct {
	builder strings.Builder
	pending string
	rowOpen bool
	cellIdx int
}

func (e *textExtractor) HandleEvent(event Event) error {
	switch event.Type {
	case EventStartParagraph:
		e.flushPending()
	case EventStartTableRow:
		e.rowOpen = true
		e.cellIdx = 0
	case EventStartTableCell:
		if e.rowOpen && e.cellIdx > 0 {
			e.pending = "\t"
		}
		e.cellIdx++
	case EventText:
		e.flushPending()
		e.builder.WriteString(event.Text)
	case EventSymbol, EventSoftHyphen, EventNoBreakHyphen:
		e.flushPending()
		e.builder.WriteString(event.Text)
	case EventTab:
		e.flushPending()
		e.builder.WriteByte('\t')
	case EventBreak:
		e.flushPending()
		e.builder.WriteByte('\n')
	case EventEndParagraph:
		e.queueSeparator("\n")
	case EventEndTableRow:
		e.rowOpen = false
		e.queueSeparator("\n")
	case EventEndStory:
		e.queueSeparator("\n")
	}
	return nil
}

func (e *textExtractor) String() string {
	return e.builder.String()
}

func (e *textExtractor) flushPending() {
	if e.pending == "" {
		return
	}
	if e.builder.Len() > 0 {
		e.builder.WriteString(e.pending)
	}
	e.pending = ""
}

func (e *textExtractor) queueSeparator(sep string) {
	switch {
	case sep == "\n":
		e.pending = "\n"
	case e.pending == "":
		e.pending = sep
	}
}