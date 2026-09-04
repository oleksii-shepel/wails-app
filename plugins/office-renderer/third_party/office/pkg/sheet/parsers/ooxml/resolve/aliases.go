package resolve

import ir "office/pkg/sheet/ir"

// The following type aliases re-export all IR types from office/pkg/sheet/ir.
// Existing callers can continue to use resolve.Workbook, resolve.Sheet, etc.
// without importing sheet/ir directly. New format parsers (legacy, ODF) should
// import sheet/ir directly and produce ir.Workbook values.
type (
	Workbook                    = ir.Workbook
	WorkbookConnection          = ir.WorkbookConnection
	WorkbookExternalLink        = ir.WorkbookExternalLink
	WorkbookExternalDefinedName = ir.WorkbookExternalDefinedName
	WorkbookExternalSheet       = ir.WorkbookExternalSheet
	WorkbookExternalCell        = ir.WorkbookExternalCell
	WorkbookCalcSettings        = ir.WorkbookCalcSettings
	DefinedName                 = ir.DefinedName
	Sheet                       = ir.Sheet
	ArrayFormula                = ir.ArrayFormula
	SharedFormula               = ir.SharedFormula
	MergedCell                  = ir.MergedCell
	Drawing                     = ir.Drawing
	DrawingType                 = ir.DrawingType
	DrawingPosition             = ir.DrawingPosition
	DrawingAnchor               = ir.DrawingAnchor
	DrawingCellAnchor           = ir.DrawingCellAnchor
	DrawingImage                = ir.DrawingImage
	DrawingShape                = ir.DrawingShape
	DrawingChart                = ir.DrawingChart
	DrawingGroup                = ir.DrawingGroup
	DrawingSmartArt             = ir.DrawingSmartArt
	SmartArtNode                = ir.SmartArtNode
	SmartArtConnection          = ir.SmartArtConnection
	DrawingOLE                  = ir.DrawingOLE
	DrawingObjectPart           = ir.DrawingObjectPart
	DrawingObjectRelationship   = ir.DrawingObjectRelationship
	Row                         = ir.Row
	Cell                        = ir.Cell
	RichTextRun                 = ir.RichTextRun
	CellStyle                   = ir.CellStyle
	NamedCellStyle              = ir.NamedCellStyle
	BorderStyle                 = ir.BorderStyle
	EmbeddedFont                = ir.EmbeddedFont
	EmbeddedFontFace            = ir.EmbeddedFontFace
	CellKind                    = ir.CellKind
	DataValidation              = ir.DataValidation
	CFRule                      = ir.CFRule
	CFCfvo                      = ir.CFCfvo
	CFDataBar                   = ir.CFDataBar
	CFIconSet                   = ir.CFIconSet
	CFColorScale                = ir.CFColorScale
	DxfStyle                    = ir.DxfStyle
	HeaderFooter                = ir.HeaderFooter
	HFImage                     = ir.HFImage
	WorkbookProtection          = ir.WorkbookProtection
	SheetProtection             = ir.SheetProtection
	PageSetup                   = ir.PageSetup
	PivotTable                  = ir.PivotTable
	PivotField                  = ir.PivotField
	PivotItem                   = ir.PivotItem
	PivotPageFieldSelection     = ir.PivotPageFieldSelection
	PivotDataField              = ir.PivotDataField
	Table                       = ir.Table
	TableColumn                 = ir.TableColumn
	TableStyleInfo              = ir.TableStyleInfo
	FormControl                 = ir.FormControl
	VMLObject                   = ir.VMLObject
	FilterColumn                = ir.FilterColumn
	CustomFilter                = ir.CustomFilter
	FilterTop10                 = ir.FilterTop10
	DynamicFilter               = ir.DynamicFilter
	SortState                   = ir.SortState
	SortCondition               = ir.SortCondition
)

const (
	DrawingTypeImage     = ir.DrawingTypeImage
	DrawingTypeShape     = ir.DrawingTypeShape
	DrawingTypeChart     = ir.DrawingTypeChart
	DrawingTypeConnector = ir.DrawingTypeConnector
	DrawingTypeGroup     = ir.DrawingTypeGroup
	DrawingTypeSmartArt  = ir.DrawingTypeSmartArt
	DrawingTypeOLE       = ir.DrawingTypeOLE

	AnchorAbsolute = ir.AnchorAbsolute
	AnchorOneCell  = ir.AnchorOneCell
	AnchorTwoCell  = ir.AnchorTwoCell

	CellEmpty   = ir.CellEmpty
	CellString  = ir.CellString
	CellNumber  = ir.CellNumber
	CellBool    = ir.CellBool
	CellError   = ir.CellError
	CellFormula = ir.CellFormula
)
