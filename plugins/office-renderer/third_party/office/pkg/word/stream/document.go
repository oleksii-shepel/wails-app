package stream

import (
	"io"

	wordir "office/pkg/word/ir"
)

// NewDocumentSession creates a streaming semantic document sink without
// incremental pagination. This is intended for full-document renderers that
// still consume word/ir.Document but should not materialize the format-specific
// parser model first.
func NewDocumentSession() *LayoutSession {
	return &LayoutSession{}
}

// ReadDOCXDocument streams a DOCX into the shared semantic Word IR without
// using the materialized OOXML parts/resolver pipeline. The returned document
// is suitable for the existing renderer/edit-neutral consumers.
func ReadDOCXDocument(r io.ReaderAt, size int64, opts DOCXOptions) (*wordir.Document, error) {
	session := NewDocumentSession()
	source := NewDOCXSource(r, size, opts)
	if err := source.Walk(session); err != nil {
		return nil, err
	}
	return session.Document(), nil
}

// ReadDOCDocument streams an in-memory legacy binary DOC through the shared
// semantic event sink.
func ReadDOCDocument(data []byte) (*wordir.Document, error) {
	session := NewDocumentSession()
	source := NewDOCSource(data)
	if err := source.Walk(session); err != nil {
		return nil, err
	}
	return session.Document(), nil
}

// ReadDOCFileDocument streams a legacy DOC directly from a file-backed CFB
// container, avoiding whole-file materialization before semantic parsing.
func ReadDOCFileDocument(path string) (*wordir.Document, error) {
	session := NewDocumentSession()
	source := NewDOCFileSource(path)
	if err := source.Walk(session); err != nil {
		return nil, err
	}
	return session.Document(), nil
}
