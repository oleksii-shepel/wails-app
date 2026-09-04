// Package ir defines the format-neutral OfficeArt intermediate representation.
//
// Legacy Office binary Escher records and OOXML DrawingML/VML are intentionally
// parsed by different frontends. Both frontends converge on these types before
// rendering so DOC/DOCX, XLS/XLSX, and PPT/PPTX share one visual model.
package ir
