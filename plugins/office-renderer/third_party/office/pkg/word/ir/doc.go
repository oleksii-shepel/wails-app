// Package ir defines the stable public intermediate representation (IR) for
// Word documents across OOXML (.docx/.docm/.dotx/.dotm) and legacy binary .doc.
//
// # Stability guarantee
//
// All exported types and fields in this package are considered stable public
// API. Additions are non-breaking. Fields will not be removed or renamed
// without a major version bump. Internal OOXML parsing details (parts,
// resolve) are NOT part of this contract.
//
// # Overview
//
// The IR is produced by [office/pkg/word/parsers/ooxml/resolve.ResolvePackage]
// and consumed by the layout and render layers. It can also be consumed
// directly by callers who need structured access to document content without
// going through the rendering pipeline.
//
// # Type hierarchy
//
//	Document
//	├── Blocks []Block          — document-order mix of paragraphs, tables, sections
//	├── Paragraphs []Paragraph  — flat paragraph list (same content, different view)
//	├── Tables []Table          — flat table list
//	├── Sections []Section      — page-layout section breaks
//	├── Headers / Footers       — header/footer stories keyed by relationship type
//	├── Footnotes / Endnotes    — note stories keyed by note ID
//	├── Comments                — comment stories keyed by comment ID
//	├── Styles                  — resolved paragraph style definitions
//	└── CharacterStyles         — resolved character style definitions
//
// # Formatting model
//
// All formatting values are fully resolved: style inheritance, theme font/color
// substitution, and default overrides have been applied. Pointer fields (e.g.
// *bool, *int) use nil to signal "unset at this level", which lets callers
// implement their own inheritance if they keep the raw style chain. For most
// use cases, the effective value from the resolved Document is sufficient.
//
// # Units
//
// Lengths default to twips (1 twip = 1/1440 inch) unless the field name
// carries an explicit suffix (e.g. *EMU, *Pt, *Lines). Use
// [office/pkg/ooxml/units] for conversion helpers.
//
// # DrawingML objects
//
// Floating images, text boxes, and shapes in [Run] fields reference types from
// [office/pkg/ooxml/drawing], which is a stable shared sub-package. Its types
// are part of this package's public contract as well.
//
// # Legacy-only provenance fields
//
// Several fields are populated exclusively by the legacy .doc parser so that
// projection layers can reconstruct binary formatting or round-trip to the
// original file. They are intentionally ignored by the OOXML parser and by
// renderers unless explicitly noted.
//
//   - ParagraphFormat.BinaryListLevel, BinaryListOverride, BinaryListFormat
//     Carry raw PAPx list pointers (ilvl, ilfo, nfcSeqNumb). Used by the
//     legacy list resolver to synthesize ListMarker/Numbering values.
//   - ParagraphFormat.BinaryStyleIndex
//     Transitional staging field for PAPx→style mapping. Cleared after use
//     by the final paragraph builder so it does not leak into resolved output.
//   - ParagraphFormat.InTable, IsTableCellTrailingParagraph, IsTableTrailingParagraph
//     Raw PAPx table flags. Heuristic table detection uses them; OOXML uses
//     explicit w:tbl/w:tr/w:tc nesting instead.
//   - ParagraphFormat.BinaryFrame
//     Raw PAPx frame SPRMs. The legacy parser promotes this to Paragraph.Frame
//     after style/direct-format merging; OOXML uses w:framePr directly.
//   - Paragraph.Source, Run.Source
//     Preserve raw binary provenance (PAPx/CHPx bytes, FIB version, piece
//     table offsets) for round-trip or diagnostic use.
//   - Document.Source
//     Preserves DOP, Clx, font table, and other document-level raw bytes.
package ir
