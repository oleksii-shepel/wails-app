// Package ir defines the stable public intermediate representation (IR) for
// spreadsheet workbooks (.xlsx / SpreadsheetML).
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
// The IR is produced by [office/pkg/sheet/parsers/ooxml/resolve.ResolvePackage]
// and consumed by the layout and render layers. It can also be consumed
// directly by callers who need structured access to workbook content without
// going through the rendering pipeline.
//
// # Type hierarchy
//
//	Workbook
//	├── Sheets []*Sheet          — worksheet tabs in workbook order
//	│   ├── Rows []*Row          — non-empty rows (sparse; rows with no cells are omitted)
//	│   │   └── Cells []*Cell    — non-empty cells in column order
//	│   ├── MergedCells          — merged cell ranges
//	│   ├── Drawings             — shapes, images, charts, and groups anchored to this sheet
//	│   ├── DataValidations      — data-validation rules
//	│   ├── CFRules              — conditional formatting rules
//	│   ├── Comments             — cell comments keyed by A1 ref
//	│   └── PageSetup / HeaderFooter
//	├── DefinedNames             — workbook-level named ranges and formulas
//	└── EmbeddedFonts            — font faces available for CSS @font-face emission
//
// # Cell values
//
// [Cell.Kind] classifies the resolved value type:
//   - [CellEmpty]   — no value
//   - [CellString]  — string (shared or inline); text in [Cell.StrValue]
//   - [CellNumber]  — numeric; value in [Cell.NumValue]; formatted via [CellStyle.NumFmtCode]
//   - [CellBool]    — boolean stored as 0/1; value in [Cell.NumValue]
//   - [CellError]   — error string such as "#DIV/0!"; text in [Cell.StrValue]
//   - [CellFormula] — formula result string; text in [Cell.StrValue]
//
// # Coordinates
//
// Row and column indices are always 1-based. [Cell.Ref] holds the A1-notation
// address. [MergedCell] uses 1-based TopRow/LeftCol/BottomRow/RightCol.
//
// # Drawing positions
//
// [DrawingPosition.Anchor] classifies how a drawing is attached to the grid:
//   - [AnchorAbsolute] — absolute EMU coordinates
//   - [AnchorOneCell]  — top-left pinned to a cell, free-floating bottom-right
//   - [AnchorTwoCell]  — both corners pinned to cells (scales with row/col resize)
//
// # Units
//
// Cell dimensions are expressed in character units (column widths) or points
// (row heights). Drawing coordinates use EMU (914400 = 1 inch). Use
// [office/pkg/ooxml/units] for conversion helpers.
package ir
