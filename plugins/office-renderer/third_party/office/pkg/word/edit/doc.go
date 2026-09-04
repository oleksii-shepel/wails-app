// Package edit provides a format-neutral mutation layer over word/ir.
//
// It keeps canonical block and flat paragraph/table views synchronized, assigns
// stable indices for newly authored content, exposes ergonomic wrappers for
// formatting and structure changes, and writes the resulting semantic model to
// either DOCX or legacy DOC.
//
// Use office/pkg/word.New and office/pkg/word.Edit for the common entry points.
// Applications that need opaque/vendor DOCX package data preserved in addition
// to semantic edits should work deliberately at the parsers/ooxml/parts layer;
// the IR is a semantic document model rather than an OPC archive.
package edit
