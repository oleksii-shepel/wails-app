package sax

import (
	"io"
	"strings"
)

// ExtractTextPath extracts plain cell text from an XLSX file on disk.
func ExtractTextPath(path string, opts Options) (string, error) {
	var extractor textExtractor
	if err := WalkPath(path, &extractor, opts); err != nil {
		return "", err
	}
	return extractor.String(), nil
}

// ExtractTextBytes extracts plain cell text from an in-memory XLSX byte slice.
func ExtractTextBytes(data []byte, opts Options) (string, error) {
	var extractor textExtractor
	if err := WalkBytes(data, &extractor, opts); err != nil {
		return "", err
	}
	return extractor.String(), nil
}

// ExtractTextStream extracts plain cell text from a non-seekable XLSX stream.
func ExtractTextStream(r io.Reader, opts Options) (string, error) {
	var extractor textExtractor
	if err := WalkStream(r, &extractor, opts); err != nil {
		return "", err
	}
	return extractor.String(), nil
}

// ExtractTextReaderAt extracts plain cell text from a seekable XLSX reader.
func ExtractTextReaderAt(r io.ReaderAt, size int64, opts Options) (string, error) {
	var extractor textExtractor
	if err := WalkReaderAt(r, size, &extractor, opts); err != nil {
		return "", err
	}
	return extractor.String(), nil
}

type textExtractor struct {
	builder strings.Builder
	cellIdx int
	rowOpen bool
}

// HandleEvent implements Handler.
func (e *textExtractor) HandleEvent(event Event) error {
	switch event.Type {
	case EventStartRow:
		e.cellIdx = 0
		e.rowOpen = true
	case EventCell:
		if event.Text == "" {
			return nil
		}
		if e.rowOpen {
			if e.cellIdx > 0 {
				e.builder.WriteByte('\t')
			}
			e.cellIdx++
		}
		e.builder.WriteString(event.Text)
	case EventEndRow:
		if e.rowOpen && e.builder.Len() > 0 {
			e.builder.WriteByte('\n')
		}
		e.rowOpen = false
	}
	return nil
}

func (e *textExtractor) String() string {
	return strings.TrimSpace(e.builder.String())
}
