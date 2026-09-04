// Package render renders the shared OfficeArt IR.
//
// This package is intentionally format-neutral. DOC/DOCX, XLS/XLSX, and
// PPT/PPTX renderers should delegate drawing nodes here rather than carrying
// independent shape/text CSS implementations.
package render
