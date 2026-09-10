package ir

import (
	"office/pkg/diag"
	officeartir "office/pkg/officeart/ir"
	drawing "office/pkg/ooxml/drawing"
)

// Workbook is the resolved spreadsheet IR produced from a parts.Package.
type Workbook struct {
	Sheets        []*Sheet
	DefinedNames  []*DefinedName
	EmbeddedFonts []*EmbeddedFont
	Connections   []*WorkbookConnection
	ExternalLinks []*WorkbookExternalLink
	Protection    *WorkbookProtection
	Calc          *WorkbookCalcSettings
	Diagnostics   diag.List
	// NamedCellStyleXFs holds the OOXML cellStyleXfs table resolved into IR cell
	// style values. Indexes match NamedCellStyle.StyleXFIndex.
	NamedCellStyleXFs []CellStyle
	// NamedCellStyles holds OOXML named cell-style metadata from styles.xml.
	NamedCellStyles []*NamedCellStyle
	// Codepage is the default character set for compressed strings in legacy
	// XLS workbooks (e.g. 1252 for Windows Latin-1, 1251 for Windows Cyrillic).
	// Populated only for legacy XLS; OOXML uses UTF-8 natively.
	Codepage int
	// ActiveSheetIndex is the workbook's active tab index from WINDOW1.
	// Populated only for legacy XLS workbooks when the record is present.
	ActiveSheetIndex *int
	// WriteAccessUser is the WRITEACCESS workbook owner string from legacy XLS.
	WriteAccessUser string
	// BackupFile records the BACKUP workbook flag from legacy XLS.
	BackupFile *bool
	// SaveExternalLinkValues records the BOOKBOOL workbook flag from legacy XLS.
	SaveExternalLinkValues *bool
}

// WorkbookCalcSettings captures workbook-level recalculation settings.
type WorkbookCalcSettings struct {
	// Mode is the legacy XLS CALCMODE value: 0=manual, 1=automatic, -1=automatic except tables.
	Mode *int
	// IterationCount is the CALCCOUNT limit for circular-reference iterations.
	IterationCount *int
	// Iterate records whether iterative calculation is enabled.
	Iterate *bool
	// Delta is the DELTA tolerance used for iterative recalculation.
	Delta *float64
	// PrecisionAsDisplayed records the PRECISION workbook flag.
	PrecisionAsDisplayed *bool
	// RefreshAll records the REFRESHALL workbook flag.
	RefreshAll *bool
	// SaveRecalc records whether recalculation results should be saved.
	SaveRecalc *bool
	// ReferenceMode is the authored formula reference mode: "A1" or "R1C1".
	ReferenceMode string
	// RecalcID identifies the calculation engine version that last recalculated the workbook.
	RecalcID *int
	// FullCalcOnLoad requests a full workbook recalculation when the file is opened.
	FullCalcOnLoad *bool
	// ForceFullCalc requests full calculation rather than dependency-chain calculation.
	ForceFullCalc *bool
}

// DefinedName is a workbook-level or sheet-local named formula or range.
type DefinedName struct {
	Name         string
	LocalSheetID *int
	Hidden       bool
	Value        string
	Comment      string
}

// WorkbookConnection is one workbook-level external data connection definition.
type WorkbookConnection struct {
	ID             int
	Name           string
	Kind           string
	Description    string
	RefreshOnLoad  bool
	Background     bool
	SaveData       bool
	SourceFile     string
	ODCFile        string
	Connection     string
	Command        string
	CommandType    string
	URL            string
	WebPost        string
	WebHTMLTables  bool
	WebSourceData  bool
	WebParsePre    bool
	WebConsecutive bool
	WebFirstRow    bool
	WebTextDates   bool
	TextFile       string
	TextDelimiter  string
	LocalCube      string
}

// WorkbookExternalLink captures workbook-level external workbook reference metadata.
type WorkbookExternalLink struct {
	Target       string
	SheetNames   []string
	DefinedNames []*WorkbookExternalDefinedName
	CachedSheets []*WorkbookExternalSheet
}

// WorkbookExternalDefinedName is one externally defined name exposed by an external workbook link.
type WorkbookExternalDefinedName struct {
	Name     string
	RefersTo string
	SheetID  *int
}

// WorkbookExternalSheet holds cached cell data for one external workbook sheet.
type WorkbookExternalSheet struct {
	ID           int
	Name         string
	RefreshError bool
	Cells        []*WorkbookExternalCell
}

// WorkbookExternalCell is one cached external workbook cell value.
type WorkbookExternalCell struct {
	Ref   string
	Type  string
	Value string
}

// Sheet is one worksheet tab in the workbook.
type Sheet struct {
	// Name is the user-visible tab name.
	Name string
	// ID is the numeric sheet identifier from the workbook (sheetId attribute).
	ID int
	// Hidden reports whether the sheet tab is hidden or very hidden.
	Hidden bool
	// Visibility is the exact sheet tab visibility state when known:
	// "", "hidden", or "veryHidden".
	Visibility string
	// DialogSheet is true when the sheet is a dialog sheet (WSBOOL bit 4).
	DialogSheet bool
	// Rows holds the non-empty rows in order. Sparse: rows with no cells are omitted.
	Rows []*Row
	// ArrayFormulas records array-formula groups declared on this sheet.
	ArrayFormulas []*ArrayFormula
	// SharedFormulas records shared-formula groups declared or created on this sheet.
	SharedFormulas []*SharedFormula
	// ColumnWidths maps 1-based column index to character-unit width.
	// Only columns with an explicit width are present.
	ColumnWidths map[int]float64
	// HiddenCols contains the 1-based column indices that are hidden.
	// Nil when no columns are hidden.
	HiddenCols map[int]bool
	// DefaultColWidthChars is the worksheet default column width in character
	// units for columns with no explicit ColumnInfo width (0 = unset).
	DefaultColWidthChars float64
	// DefaultRowHeightPt is the default row height in points (0 = unset).
	DefaultRowHeightPt float64
	// DefaultRowHeightUnsynced reports that the default row height was set automatically,
	// not by the user (DEFROWHT grbit bit 0 = fUnsynced).
	DefaultRowHeightUnsynced bool
	// DefaultRowHeightZeroHeight reports that rows using the default height should be
	// displayed with zero height, i.e. hidden (DEFROWHT grbit bit 1 = fDyZero).
	DefaultRowHeightZeroHeight bool
	// DefaultRowHeightExtraSpaceAbove reports that extra space above the default-height rows
	// is enabled (DEFROWHT grbit bit 2 = fExAsc).
	DefaultRowHeightExtraSpaceAbove bool
	// DefaultRowHeightExtraSpaceBelow reports that extra space below the default-height rows
	// is enabled (DEFROWHT grbit bit 3 = fExDsc).
	DefaultRowHeightExtraSpaceBelow bool
	// FrozenRows is the number of rows frozen in the default sheet view (0 = none).
	FrozenRows int
	// FrozenCols is the number of columns frozen in the default sheet view (0 = none).
	FrozenCols int
	// SplitRows/SplitCols are the legacy integer compatibility view of a split pane.
	SplitRows int
	SplitCols int
	// PaneState preserves the authored OOXML pane state (frozen, frozenSplit, split).
	PaneState string
	// SplitX/SplitY preserve exact OOXML xSplit/ySplit values. For frozen panes
	// these are integral row/column counts; for split panes they may be fractional.
	SplitX float64
	SplitY float64
	// PaneSelections preserves per-pane active-cell/selection metadata.
	PaneSelections []PaneSelection
	// TopLeftCell is the cell reference shown in the top-left of the viewport
	// when panes are frozen or split (e.g., "B5").
	TopLeftCell string
	// ActivePane indicates which pane is active: "topLeft", "topRight", "bottomLeft", "bottomRight".
	ActivePane string
	// ShowGridLines controls worksheet gridline visibility in the default sheet view.
	// Nil means the file did not specify an explicit setting.
	ShowGridLines *bool
	// ZoomScale is the default worksheet view zoom percentage (10..400 typical;
	// 0 means unset/default 100%).
	ZoomScale int
	// MergedCells contains merged ranges keyed by their top-left anchor.
	MergedCells []*MergedCell
	// Drawings contains shapes, images, and other drawing elements on this sheet.
	Drawings []*Drawing
	// DimFirstRow is the first used row (1-based) from the DIMENSIONS record.
	// 0 when the DIMENSIONS record was not present or the sheet is empty.
	DimFirstRow int
	// DimLastRow is the last used row (1-based, inclusive) from the DIMENSIONS record.
	// 0 when the DIMENSIONS record was not present or the sheet is empty.
	DimLastRow int
	// DimFirstCol is the first used column (1-based) from the DIMENSIONS record.
	// 0 when the DIMENSIONS record was not present or the sheet is empty.
	DimFirstCol int
	// DimLastCol is the last used column (1-based, inclusive) from the DIMENSIONS record.
	// 0 when the DIMENSIONS record was not present or the sheet is empty.
	DimLastCol int
	// FilterMode indicates the sheet is in filtered mode (FILTERMODE record present).
	// When true, the sheet has an active autoFilter that may be collapsed/hidden.
	FilterMode bool
	// AutoFilterRef is the A1-notation range of the autoFilter, e.g. "A1:E10".
	// Empty when no autoFilter is set.
	AutoFilterRef string
	// FilterColumns holds per-column active filter criteria for this sheet.
	// Populated from BIFF AUTOFILTER (0x9E) records (XLS) or <filterColumn> (XLSX).
	// ColIndex in each entry is 0-based within the AutoFilter range.
	FilterColumns []*FilterColumn
	// SortState holds the active sort metadata associated with the worksheet
	// autoFilter, when present.
	SortState *SortState
	// Tables lists all defined structured tables on this sheet.
	Tables []*Table
	// DataValidations lists all data-validation rules on this sheet.
	DataValidations []*DataValidation
	// CFRules holds all parsed conditional formatting rules for this sheet.
	CFRules []*CFRule
	// Comments maps cell refs (e.g. "B2") to their comment text.
	// Kept as the plain-text compatibility view of Notes.
	Comments map[string]string
	// Notes preserves authored worksheet-note metadata and rich text.
	Notes map[string]*CommentNote
	// Hyperlinks preserves worksheet hyperlink records, including range refs,
	// display text, tooltips, and internal/external target semantics. Cells also
	// retain HyperlinkURL as the convenient per-cell resolved view.
	Hyperlinks []*Hyperlink
	// PageSetup holds the resolved print/page configuration. Nil when not specified.
	PageSetup *PageSetup
	// Protection holds worksheet protection metadata. Nil when the sheet is unprotected.
	Protection *SheetProtection
	// PrintArea is the worksheet print area range or comma-separated ranges,
	// e.g. "$A$1:$D$20" or "$A$1:$D$20,$F$1:$F$10".
	PrintArea string
	// PrintTitleRows is the workbook-defined row repeat range for printing, e.g. "$1:$2".
	PrintTitleRows string
	// PrintTitleCols is the workbook-defined column repeat range for printing, e.g. "$A:$B".
	PrintTitleCols string
	// RowPageBreaks lists 1-based row indices where a manual horizontal page
	// break occurs (a break before that row). HSSF HorizontalPageBreakRecord
	// (0x001B) / OOXML <rowBreaks>.
	RowPageBreaks []int
	// ColPageBreaks lists 1-based column indices where a manual vertical page
	// break occurs (a break before that column). HSSF VerticalPageBreakRecord
	// (0x001A) / OOXML <colBreaks>.
	ColPageBreaks []int
	// OutlineRows holds row outline levels. Key is 1-based row index, value is the level (1–7).
	OutlineRows map[int]int
	// CollapsedRows marks rows that carry the worksheet collapsed-outline flag.
	CollapsedRows map[int]bool
	// OutlineCols holds column outline levels. Key is 1-based column index, value is the level (1–7).
	OutlineCols map[int]int
	// CollapsedCols marks columns that carry the worksheet collapsed-outline flag.
	CollapsedCols map[int]bool
	// RowGutter is the row gutter size from BIFF GUTS record (screen units).
	RowGutter int
	// ColGutter is the column gutter size from BIFF GUTS record (screen units).
	ColGutter int
	// RowOutlineLevelMax is the maximum row outline level from BIFF GUTS.
	RowOutlineLevelMax int
	// ColOutlineLevelMax is the maximum column outline level from BIFF GUTS.
	ColOutlineLevelMax int
	// HeaderFooter holds the header and footer text for this worksheet. Nil when not specified.
	HeaderFooter *HeaderFooter
	// PivotTables holds all pivot tables anchored to this sheet.
	PivotTables []*PivotTable
	// FormControls holds VML-based form controls (buttons, checkboxes, etc.) anchored to this sheet.
	FormControls []*FormControl
	// VMLObjects holds generic editable VML-backed worksheet objects.
	VMLObjects []*VMLObject
	// LegacyDrawingVML preserves the original worksheet VML part so unsupported VML
	// children and formatting survive canonical round-trips. Structured FormControls
	// and VMLObjects remain authoritative when they are edited.
	LegacyDrawingVML []byte
}

// FilterColumn holds the active filter criteria for one column within an autoFilter range.
// ColIndex is 0-based within the AutoFilter range (0 = first column of the range).
// Matches HSSF AutoFilter record (0x9E) and OOXML <filterColumn>.
type FilterColumn struct {
	// ColIndex is the 0-based column position within the autoFilter range.
	ColIndex int
	// Filters lists the discrete equality-match values (OOXML <filters><filter val="..."/>
	// or HSSF string DOPER with operator "equal").
	Filters []string
	// ShowBlanks is true when blank cells pass this filter (OOXML blank attr).
	ShowBlanks bool
	// CustomFilters holds up to two comparison conditions.
	CustomFilters []*CustomFilter
	// CustomAnd is true when two CustomFilters are AND-combined (false = OR).
	CustomAnd bool
	// Top10, when non-nil, is a top/bottom-N filter instead of value filtering.
	Top10 *FilterTop10
	// Dynamic, when non-nil, is an OOXML dynamic filter such as "today",
	// "lastMonth", or "aboveAverage".
	Dynamic *DynamicFilter
}

// CustomFilter is one comparison condition in a custom autoFilter.
// Mirrors OOXML <customFilter operator="..." val="..."/>.
type CustomFilter struct {
	// Operator is one of: "lessThan", "lessThanOrEqual", "equal",
	// "notEqual", "greaterThanOrEqual", "greaterThan".
	Operator string
	// Val is the comparison value as a string.
	Val string
}

// Table describes a contiguous structured table range (OOXML Table / HSSF FEAT List).
type Table struct {
	// ID is the unique table identifier.
	ID int
	// Name is the defined table name (e.g. "Table1").
	Name string
	// DisplayName is the display name (often matches Name).
	DisplayName string
	// Ref is the total A1-notation range bounding the table, including headers and totals.
	Ref string
	// HeaderRowCount is the number of header rows at the top of the table.
	HeaderRowCount int
	// TotalsRowCount is the number of totals rows at the bottom of the table.
	TotalsRowCount int
	// TotalsRowShown is true when the totals row is displayed (OOXML totalsRowShown attribute).
	TotalsRowShown bool
	// AutoFilterRef is the A1-notation range of the table's embedded autoFilter, if any.
	AutoFilterRef string
	// TableStyle holds the table style info (name, showFirstColumn, showLastColumn, showRowStripes, showColumnStripes).
	TableStyle *TableStyleInfo
	// Columns describes the columns within the table.
	Columns []*TableColumn
}

// TableStyleInfo carries the visual style metadata from a table's tableStyleInfo element.
type TableStyleInfo struct {
	// Name is the table style name (e.g. "TableStyleLight1").
	Name string
	// ShowFirstColumn is true when the first column receives special formatting.
	ShowFirstColumn bool
	// ShowLastColumn is true when the last column receives special formatting.
	ShowLastColumn bool
	// ShowRowStripes is true when banded row formatting is applied.
	ShowRowStripes bool
	// ShowColumnStripes is true when banded column formatting is applied.
	ShowColumnStripes bool
}

// TableColumn describes a single column inside a Table.
type TableColumn struct {
	// ID is the column identifier.
	ID int
	// Name is the column header text.
	Name string
	// CalculatedColumnFormula is the formula string for a calculated column, if any.
	CalculatedColumnFormula string
	// TotalsRowFormula is the explicit formula string for the totals row, if any.
	TotalsRowFormula string
	// TotalsRowFunction is the built-in aggregation function for the totals row (e.g. "sum", "average").
	TotalsRowFunction string
	// TotalsRowLabel is the static label text for the totals row cell (e.g. "Total").
	TotalsRowLabel string
}

// FilterTop10 holds a top-N or bottom-N autoFilter condition.
// Mirrors OOXML <top10 top="1" percent="0" val="5"/>.
type FilterTop10 struct {
	// Top is true for top-N, false for bottom-N.
	Top bool
	// Percent is true when Val is a percentage rather than a count.
	Percent bool
	// Val is the N value (count or percent threshold).
	Val float64
}

// DynamicFilter holds an OOXML dynamic autoFilter condition.
// Mirrors <dynamicFilter type="..." .../>.
type DynamicFilter struct {
	// Type is the OOXML dynamic filter type, for example "today",
	// "lastMonth", "aboveAverage", or "M1".
	Type string
	// Val is an optional numeric operand used by some dynamic filter types.
	Val *float64
	// MaxVal is an optional secondary numeric operand used by some dynamic filter types.
	MaxVal *float64
	// ValISO is an optional ISO-8601 date/time operand used by some dynamic filter types.
	ValISO string
	// MaxValISO is an optional secondary ISO-8601 date/time operand.
	MaxValISO string
}

// SortState holds worksheet autoFilter sort metadata.
// Mirrors <sortState> with one or more <sortCondition> children.
type SortState struct {
	// Ref is the A1-notation range the sort applies to.
	Ref string
	// CaseSensitive reports whether the sort is case-sensitive.
	CaseSensitive bool
	// Conditions are the ordered sort keys.
	Conditions []*SortCondition
}

// SortCondition is one sort key within a SortState.
type SortCondition struct {
	// Ref is the A1-notation range sorted by this condition.
	Ref string
	// Descending reports descending sort order.
	Descending bool
	// SortBy is the OOXML sortBy discriminator, for example "value", "cellColor",
	// "fontColor", or "icon".
	SortBy string
	// CustomList is the optional custom-list name/value.
	CustomList string
	// DxfID is the differential-format index for color-based sorts when present.
	DxfID *int
	// IconSet is the icon set name for icon-based sorts when present.
	IconSet string
	// IconID is the icon index for icon-based sorts when present.
	IconID *int
}

// VMLObject represents a generic VML-backed worksheet object.
type VMLObject struct {
	ID          string
	Type        string
	Style       string
	FillColor   string
	StrokeColor string
	Text        string
	Anchor      string
}

// FormControl represents a VML-based Excel form control (button, checkbox, dropdown, etc.).
type FormControl struct {
	// Type is the control type: "Button", "Checkbox", "Radio", "List", "Drop", "Spin", "Scroll", "GBox", "Label", "Edit".
	Type string
	// AltText is the caption or alt text for the control.
	AltText string
	// Anchor is the raw VML anchor string: "col1, colOff, row1, rowOff, col2, colOff2, row2, rowOff2" (0-based, pixel offsets).
	Anchor string
	// Checked is true for checked Checkbox/Radio controls.
	Checked bool
	// Mixed is true for tri-state Checkbox controls in the indeterminate state.
	Mixed bool
	// FirstButton is true for the first radio button in an option-button group.
	FirstButton bool
	// FmlaLink is the linked cell reference (e.g. "$B$1").
	FmlaLink string
	// FmlaGroup is the group-linked cell reference for Group Box controls (e.g. "$H$1").
	FmlaGroup string
	// FmlaRange is the source range for List/Drop controls (e.g. "$D$1:$D$3").
	FmlaRange string
	// SelectionType is the list-box selection mode: "single", "multi", or "extended".
	SelectionType string
	// Selected holds one-based indexes of selected items for multi-select list controls.
	Selected []int
	// Min, Max, Val are used for Spin and Scroll controls.
	Min, Max, Val int
	// Increment and PageStep preserve the BIFF/VML minor and page-step values.
	Increment, PageStep int
	// Horizontal is true for horizontal scroll bars.
	Horizontal bool
	// DropLines is the number of visible rows for List/Drop controls when specified.
	DropLines int
}

// MergedCell describes a rectangular merged range in worksheet coordinates.
type MergedCell struct {
	TopRow    int
	LeftCol   int
	BottomRow int
	RightCol  int
}

// RowSpan returns the merged range height in rows.
func (m *MergedCell) RowSpan() int {
	if m == nil || m.BottomRow < m.TopRow {
		return 1
	}
	return m.BottomRow - m.TopRow + 1
}

// ColSpan returns the merged range width in columns.
func (m *MergedCell) ColSpan() int {
	if m == nil || m.RightCol < m.LeftCol {
		return 1
	}
	return m.RightCol - m.LeftCol + 1
}

// Drawing represents a drawing element (shape, image, chart, etc.) on a worksheet.
type Drawing struct {
	// OfficeArt is the canonical cross-format drawing representation.
	OfficeArt *officeartir.Shape
	// Name is the drawing element name.
	Name string
	// Type indicates the kind of drawing element.
	Type DrawingType
	// Position defines where the drawing is placed on the sheet.
	Position DrawingPosition
	// For images: image data and metadata.
	Image *DrawingImage
	// For shapes: shape properties.
	Shape *DrawingShape
	// For charts: chart data (future extension).
	Chart *DrawingChart
	// For groups: nested drawings positioned relative to the group bounds.
	Group *DrawingGroup
	// SmartArt preserves a worksheet diagram and its raw OOXML definition parts.
	SmartArt *DrawingSmartArt
	// OLE preserves embedded or linked object metadata/payload.
	OLE *DrawingOLE
}

// DrawingType classifies drawing elements.
type DrawingType string

const (
	DrawingTypeImage DrawingType = "image"
	DrawingTypeShape DrawingType = "shape"
	DrawingTypeChart DrawingType = "chart"
	// DrawingTypeConnector is a connector/line drawing.
	DrawingTypeConnector DrawingType = "connector"
	// DrawingTypeGroup is a grouped drawing container with nested child drawings.
	DrawingTypeGroup    DrawingType = "group"
	DrawingTypeSmartArt DrawingType = "smartArt"
	DrawingTypeOLE      DrawingType = "ole"
)

// DrawingPosition defines where a drawing is placed on a worksheet.
type DrawingPosition struct {
	// Anchor defines the geometric anchor representation.
	Anchor DrawingAnchor
	// Behavior preserves how a two-cell DrawingML anchor responds when rows or
	// columns move/resize. Empty means the producer default.
	Behavior DrawingAnchorBehavior
	// LocksWithSheet and PrintsWithSheet preserve xdr:clientData object behavior.
	LocksWithSheet  *bool
	PrintsWithSheet *bool
	// FromCell and ToCell define the cell range the drawing spans.
	FromCell DrawingCellAnchor
	ToCell   DrawingCellAnchor
	// Absolute coordinates in EMUs (for absolute positioning).
	X, Y, Width, Height int64
}

// DrawingAnchor defines how a drawing is anchored to worksheet cells.
type DrawingAnchor string

const (
	// AnchorAbsolute means the drawing is positioned with absolute coordinates.
	AnchorAbsolute DrawingAnchor = "absolute"
	// AnchorOneCell means the drawing is anchored to a single cell.
	AnchorOneCell DrawingAnchor = "oneCell"
	// AnchorTwoCell means the drawing is anchored between two cells.
	AnchorTwoCell DrawingAnchor = "twoCell"
)

// DrawingAnchorBehavior describes DrawingML two-cell anchor edit behavior.
type DrawingAnchorBehavior string

const (
	AnchorBehaviorMoveAndSize DrawingAnchorBehavior = "twoCell"
	AnchorBehaviorMoveOnly    DrawingAnchorBehavior = "oneCell"
	AnchorBehaviorFixed       DrawingAnchorBehavior = "absolute"
)

// DrawingCellAnchor describes a cell anchor point.
type DrawingCellAnchor struct {
	// Row and Col are 1-based cell coordinates.
	Row, Col int
	// RowOffset and ColOffset are offsets in EMUs from the cell corner.
	RowOffset, ColOffset int64
}

// DrawingImage contains image-specific data.
type DrawingImage struct {
	// Relationship ID to the image part.
	RelID string
	// Image data (loaded separately).
	Data []byte
	// MIME type of the image.
	ContentType string
	// Alternative text.
	AltText string
	// Crop values are thousandths of a percent (0..100000) removed from
	// the corresponding source-image edge.
	CropLeft, CropTop, CropRight, CropBottom int
}

// DrawingShape contains shape-specific data.
type DrawingShape struct {
	// Geometry type (rectangle, ellipse, etc.).
	Geometry string
	// Adjustments contains adjustment guide values (0.0–1.0) that customise the shape.
	Adjustments []float64
	// Fill is the resolved DrawingML fill (gradient, pattern, picture, etc.).
	// When non-nil it takes precedence over FillColor for SVG rendering.
	Fill drawing.Fill
	// Effects contains resolved DrawingML shape effects.
	Effects drawing.EffectList
	// Fill color in hex format (RRGGBB).
	FillColor string
	// Line/border color in hex format.
	LineColor string
	// Line width in points.
	LineWidthPt float64
	// LineStyle preserves shared DrawingML stroke semantics such as dash
	// patterns, joins, caps, and arrowheads when available.
	LineStyle *drawing.LineStyle
	// ConnectorType is the DrawingML preset geometry for connector shapes.
	// Examples: "straightConnector1", "bentConnector3", "curvedConnector5".
	ConnectorType string
	// Text content for text boxes.
	Text string
	// RichTextRuns holds formatted text runs for text boxes.
	RichTextRuns []*RichTextRun
	// RotationDeg is the clockwise shape rotation in degrees.
	RotationDeg float64
	// FlipH and FlipV preserve horizontal/vertical OfficeArt/DrawingML flips.
	FlipH bool
	FlipV bool
}

// DrawingChart contains chart-specific data.
type DrawingChart struct {
	// RelID is the drawing relationship that owns the chart part.
	RelID string
	// AltTitle and AltText preserve non-visual chart accessibility metadata.
	AltTitle string
	AltText  string
	// Chart is the parsed chart data. Parsed semantics are preferred for rendering
	// and editing, while RawXML remains the lossless fallback for unsupported charts.
	Chart interface{}
	// RawXML preserves the original chart part even when the chart parser cannot
	// model it yet. Writers use it when no parsed chart payload is available.
	RawXML []byte
	// SVG is the rendered SVG string.
	SVG string
}

// DrawingSmartArt preserves the SmartArt relationship set, semantic data model,
// and raw OOXML definition parts. The raw parts remain authoritative for
// unmodeled extension data, while Nodes/Connections provide bounded semantic
// editing of the diagram data model.
type DrawingSmartArt struct {
	DataRelID, LayoutRelID, QuickStyleRelID, ColorsRelID string
	Parts                                                map[string]DrawingObjectPart
	// DiagramType is the resolved layout category (for example hierarchy, process,
	// cycle, radial, or matrix).
	DiagramType string
	// Nodes are SmartArt data points in declaration order.
	Nodes []*SmartArtNode
	// Connections are logical parent/child edges between node IDs.
	Connections []SmartArtConnection
	// SemanticDirty is set by the edit layer when the data model changes. Writers
	// use it to regenerate dgm:dataModel and drop stale generated drawing geometry.
	SemanticDirty bool
}

// SmartArtNode is an editable data point inside a worksheet SmartArt diagram.
type SmartArtNode struct {
	ID        string
	Type      string
	Text      string
	FillColor string
	TextColor string
	Children  []string
}

// SmartArtConnection is a logical edge between two SmartArt data-model nodes.
type SmartArtConnection struct {
	SrcID  string
	DestID string
}

// DrawingOLE preserves embedded or linked OLE object semantics.
type DrawingOLE struct {
	ProgID      string
	Link        string
	RelID       string
	ShapeID     int
	Data        []byte
	ContentType string
	External    bool
}

// DrawingObjectPart is a raw object-owned OOXML part used for lossless round-trip.
type DrawingObjectPart struct {
	RelID, RelType, PartName, ContentType, Target string
	Data                                          []byte
	External                                      bool
	Relationships                                 []DrawingObjectRelationship
	Children                                      map[string]DrawingObjectPart
}

// DrawingObjectRelationship preserves an OOXML relationship owned by an object part.
type DrawingObjectRelationship struct{ ID, Type, Target, TargetMode string }

// DrawingGroup contains drawings nested inside a grouped worksheet drawing.
// Child positions are always relative to the group's bounds in EMU.
type DrawingGroup struct {
	Children []*Drawing
}

// Row is a resolved worksheet row.
type Row struct {
	// Index is the 1-based row number.
	Index int
	// HeightPt is the row height in points (0 = default).
	HeightPt float64
	// Hidden reports whether the row is hidden.
	Hidden bool
	// OutlineLevel is the grouping depth (0 = not grouped, 1–7 = grouped).
	OutlineLevel int
	// Collapsed reports whether this row is marked as a collapsed outline boundary.
	Collapsed bool
	// Style holds row-level formatting (background, font, etc.) applied when the
	// BIFF RowRecord "formatted" flag is set. Nil when no row-level style is present.
	Style *CellStyle
	// Cells holds the non-empty cells in column order.
	Cells []*Cell
}

// ArrayFormula describes one array-formula group on a worksheet.
type ArrayFormula struct {
	// AnchorRef is the top-left cell that owns the formula text.
	AnchorRef string
	// RangeRef is the declared A1-style array range (for example, "A1:C3").
	RangeRef string
	// Formula is the formula text stored on the anchor cell.
	Formula string
	// TopRow, LeftCol, BottomRow, RightCol bound the array range.
	TopRow    int
	LeftCol   int
	BottomRow int
	RightCol  int
}

// SharedFormula describes one OOXML shared-formula group on a worksheet.
type SharedFormula struct {
	// Index is the shared formula group index (OOXML f@si).
	Index int
	// AnchorRef is the top-left/master cell that owns the formula text.
	AnchorRef string
	// RangeRef is the declared A1-style shared formula range.
	RangeRef string
	// Formula is the formula text stored on the anchor cell.
	Formula string
	// TopRow, LeftCol, BottomRow, RightCol bound the shared formula range.
	TopRow    int
	LeftCol   int
	BottomRow int
	RightCol  int
}

// Cell is a resolved worksheet cell.
type Cell struct {
	// Row and Col are 1-based coordinates.
	Row int
	Col int
	// Ref is the A1-style address, e.g. "B3".
	Ref string
	// Kind classifies the resolved value type.
	Kind CellKind
	// StrValue holds the string value for String and Error cells.
	StrValue string
	// NumValue holds the numeric value for Number and Boolean cells.
	NumValue float64
	// Formula holds the cell formula text, if present.
	Formula string
	// ArrayFormulaRange holds the array-formula range this cell belongs to.
	ArrayFormulaRange string
	// ArrayFormulaAnchor holds the anchor cell ref for the array-formula group.
	ArrayFormulaAnchor string
	// Style holds resolved formatting for this cell.
	Style CellStyle
	// HyperlinkURL is the hyperlink target for this cell, or "" if none.
	// External URLs are absolute; internal references are prefixed with "#".
	HyperlinkURL string
	// RichTextRuns holds the formatted runs for a rich-text cell.
	// Non-nil only when the cell is a shared string with multiple RTRun entries.
	RichTextRuns []*RichTextRun
}

// RichTextRun is a single styled run within a rich-text cell value.
type RichTextRun struct {
	Text          string
	Bold          bool
	Italic        bool
	Underline     bool
	UnderlineType string
	Strike        bool
	VertAlign     string
	FontSizePt    float64
	FontColor     string
	FontFamily    string
}

// CellStyle holds the resolved display properties of a cell.
type CellStyle struct {
	Bold      bool
	Italic    bool
	Underline bool
	// UnderlineType preserves OOXML underline variants such as double and accounting.
	UnderlineType string
	Strike        bool
	// FontVertAlign is "superscript", "subscript", or empty.
	FontVertAlign string
	// FontFamily is the resolved cell font family, if the style specifies one.
	FontFamily string
	// HAlign is the horizontal alignment: "left", "center", "right", "general", or "".
	HAlign string
	// VAlign is the vertical alignment: "top", "middle", "bottom", or "".
	VAlign      string
	WrapText    bool
	ShrinkToFit bool
	Indent      int
	// TextRotation is the Excel text rotation in degrees-like OOXML units.
	TextRotation int
	// FontSizePt is the font size in points (0 = default).
	FontSizePt float64
	// FontColor is a hex sRGB color string (e.g. "FF0000"), or "" for default.
	FontColor string
	// BgColor is the resolved foreground/solid fill color, or "" for none.
	BgColor string
	// FillPattern preserves the OOXML patternFill patternType. Empty and "none" mean no fill.
	FillPattern string
	// FillPatternBgColor is the resolved pattern background color. It is used
	// together with BgColor for non-solid patterned fills.
	FillPatternBgColor string
	BorderTop          BorderStyle
	BorderRight        BorderStyle
	BorderBottom       BorderStyle
	BorderLeft         BorderStyle
	// BorderDiagonal preserves Excel diagonal border styling. The direction
	// flags distinguish rising (bottom-left to top-right) and falling diagonals.
	BorderDiagonal     BorderStyle
	BorderDiagonalUp   bool
	BorderDiagonalDown bool
	// BorderVertical/BorderHorizontal preserve OOXML inside-border semantics.
	// They are canonical metadata even when a single HTML cell has no direct
	// equivalent for the inside edge.
	BorderVertical   BorderStyle
	BorderHorizontal BorderStyle
	// NumFmtCode is the number format string for numeric cells, or "".
	NumFmtCode string
	// Locked is true when the cell is locked for editing (XF grbit bit 2; only
	// takes effect when sheet protection is enabled).
	Locked bool
	// FormulaHidden is true when the cell formula is hidden from the formula bar
	// (XF grbit bit 3; only takes effect when sheet protection is enabled).
	FormulaHidden bool
}

// NamedCellStyle describes one OOXML named style entry from styles.xml.
type NamedCellStyle struct {
	Name string
	// StyleXFIndex points into Workbook.NamedCellStyleXFs.
	StyleXFIndex int
	// BuiltinID is the optional built-in style identifier (for example, 0 for
	// Normal). Nil means the style is user-defined.
	BuiltinID *int
	// Style is the resolved style-xf value referenced by StyleXFIndex.
	Style CellStyle
}

// BorderStyle is a resolved border side.
type BorderStyle struct {
	Style string
	Color string
}

// EmbeddedFont is a resolved font family with one or more usable faces.
type EmbeddedFont struct {
	Family string
	Faces  []*EmbeddedFontFace
}

// EmbeddedFontFace is a concrete font asset bound to a CSS weight/style.
type EmbeddedFontFace struct {
	Family      string
	Weight      int
	Style       string
	ContentType string
	Data        []byte
}

// CellKind classifies the resolved type of a cell value.
type CellKind int

const (
	CellEmpty   CellKind = iota
	CellString           // shared string or inline string
	CellNumber           // numeric (integer or floating-point)
	CellBool             // boolean (stored as 0/1 with t="b")
	CellError            // error value (t="e"), e.g. "#DIV/0!"
	CellFormula          // formula string result (t="str")
	CellDate             // numeric cell whose format string encodes a date; StrValue holds ISO 8601 date (YYYY-MM-DD or YYYY-MM-DD HH:MM:SS)
)

// PaneSelection preserves one worksheet sheetView selection record.
type PaneSelection struct {
	Pane         string
	ActiveCell   string
	ActiveCellID int
	SqRef        string
}

// Hyperlink preserves one authored worksheet hyperlink record.
type Hyperlink struct {
	Ref     string
	URL     string
	Display string
	Tooltip string
}

// CommentNote preserves one legacy Excel cell note, including author and rich text.
type CommentNote struct {
	Ref      string
	Author   string
	Text     string
	RichText []*RichTextRun
}

// DataValidation is a resolved data-validation rule from a worksheet.
type DataValidation struct {
	// SqRef is the cell range(s) the rule applies to, e.g. "B2:B10".
	SqRef string
	// Type is the validation type: "list", "whole", "decimal", "date", "time", "textLength", "custom".
	Type string
	// Operator is the comparison operator (for non-list types).
	Operator string
	// ErrorStyle preserves the authored validation alert style (stop, warning, information).
	ErrorStyle string
	// IMEMode preserves the authored input-method mode.
	IMEMode string
	// AllowBlank reports whether blank cells are allowed.
	AllowBlank bool
	// ShowDropDown reports whether a drop-down list is hidden (true = hidden).
	ShowDropDown bool
	// ShowInputMessage reports whether to show an input prompt.
	ShowInputMessage bool
	// ShowErrorMessage reports whether to show an error alert.
	ShowErrorMessage bool
	// PromptTitle is the title of the input prompt.
	PromptTitle string
	// Prompt is the body text of the input prompt.
	Prompt string
	// ErrorTitle is the title of the error alert.
	ErrorTitle string
	// Error is the body text of the error alert.
	Error string
	// Formula1 is the primary formula or value.
	Formula1 string
	// Formula2 is the secondary formula or value (used by between/notBetween).
	Formula2 string
	// ListValues contains resolved list entries for list validations when they can
	// be determined from inline values, named ranges, or simple worksheet ranges.
	ListValues []string
}

// CFRule is a resolved conditional formatting rule.
type CFRule struct {
	// SqRef is the cell range the rule applies to.
	SqRef string
	// Type identifies the rule type: "cellIs", "expression", "colorScale", etc.
	Type string
	// Operator is the comparison operator for cellIs rules.
	Operator string
	// Priority is the rule priority (lower = higher precedence).
	Priority int
	// Percent indicates that Rank is a percentage instead of an item count.
	Percent bool
	// Bottom indicates a bottom-N rule when Type is "top10".
	Bottom bool
	// Rank is the top/bottom item count or percentage for top10 rules.
	Rank int
	// Formula holds the formula operands (1 or 2 entries for most rules).
	Formula []string
	// Text is the text value for containsText/notContainsText rules.
	Text string
	// TimePeriod is the OOXML timePeriod value for date/time conditional rules.
	TimePeriod string
	// DxfStyle holds the resolved style delta to apply when the rule matches.
	DxfStyle DxfStyle
	// DataBar holds data bar visual formatting (type="dataBar").
	DataBar *CFDataBar
	// IconSet holds icon set visual formatting (type="iconSet").
	IconSet *CFIconSet
	// ColorScale holds color scale visual formatting (type="colorScale").
	ColorScale *CFColorScale
}

// CFCfvo represents a conditional format value object (threshold).
type CFCfvo struct {
	Type string // "min", "max", "num", "percent", "percentile", "formula"
	Val  string // The threshold value (may be a number, percent, or formula)
}

// CFDataBar represents a data bar visual format.
type CFDataBar struct {
	MinLength int
	MaxLength int
	ShowValue bool
	Color     string
	Cfvo      []CFCfvo
}

// CFIconSet represents an icon set visual format.
type CFIconSet struct {
	Set       string
	ShowValue bool
	Reverse   bool
	Cfvo      []CFCfvo
}

// CFColorScale represents a color scale (heat map) visual format.
type CFColorScale struct {
	Cfvo   []CFCfvo // Thresholds (min/max/percentile points)
	Colors []string // RGB color stops
}

// DxfStyle is the resolved differential format (style override) for a CF rule.
type DxfStyle struct {
	Bold      *bool
	Italic    *bool
	Underline *bool
	Strike    *bool
	FontColor string
	BgColor   string

	// BorderFormatting holds the border style overrides (mirrors POI BorderFormatting).
	BorderLeft          *int // 0=none, 1=thin, 2=medium, 3=dashed, 4=hair, 5=thick, 6=double, 7=dotted, ...
	BorderRight         *int
	BorderTop           *int
	BorderBottom        *int
	BorderDiagonal      *int
	LeftBorderColor     string
	RightBorderColor    string
	TopBorderColor      string
	BottomBorderColor   string
	DiagonalBorderColor string
	ForwardDiagonalOn   *bool
	BackwardDiagonalOn  *bool

	// FontFormatting holds the font override details (mirrors POI FontFormatting).
	FontHeight     *int // in 1/20th point units; -1 means preserve
	FontWeight     *int // 400=normal, 700=bold
	EscapementType *int // 0=none, 1=superscript, 2=subscript
	UnderlineType  *int // 0=none, 1=single, 2=double, 33=single-accounting, 34=double-accounting
	FontColorIndex *int // palette index
	Outline        *bool
	Shadow         *bool

	// PatternFormatting holds the fill pattern overrides (mirrors POI PatternFormatting).
	FillPattern         *int // 0=no fill, 1=solid, 2=fine dots, ...
	FillForegroundColor string
	FillBackgroundColor string
}

// HeaderFooter holds the header and footer text for a worksheet.
// ECMA-376 Part 1 §18.3.1.46.
type HeaderFooter struct {
	// OddHeader is the header text for odd-numbered pages (and all pages when DifferentOddEven is false).
	OddHeader string
	// OddFooter is the footer text for odd-numbered pages (and all pages when DifferentOddEven is false).
	OddFooter string
	// EvenHeader is the header text for even-numbered pages (only used when DifferentOddEven is true).
	EvenHeader string
	// EvenFooter is the footer text for even-numbered pages (only used when DifferentOddEven is true).
	EvenFooter string
	// FirstHeader is the header text for the first page (only used when DifferentFirst is true).
	FirstHeader string
	// FirstFooter is the footer text for the first page (only used when DifferentFirst is true).
	FirstFooter string
	// DifferentOddEven reports whether odd and even pages use different headers/footers.
	DifferentOddEven bool
	// DifferentFirst reports whether the first page uses a different header/footer.
	DifferentFirst bool
	// ScaleWithDoc scales header/footer text and graphics with the document.
	ScaleWithDoc bool
	// AlignWithMargins aligns header/footer sections to the page margins.
	AlignWithMargins bool
	// Images holds the header/footer VML picture images in the order they correspond to
	// &G placeholders in the header/footer text strings.
	Images []HFImage
}

// HFImage holds image data for a header/footer VML picture.
type HFImage struct {
	// ContentType is the MIME content-type of the image (e.g. "image/png").
	ContentType string
	// Data is the raw image bytes.
	Data []byte
}

// WorkbookProtection holds workbook-level protection metadata.
type WorkbookProtection struct {
	WorkbookPassword  string
	RevisionsPassword string
	LockStructure     bool
	LockWindows       bool
	LockRevision      bool
	WriteProtected    bool
	WriteResPassword  string
	WriteResUser      string
	AlgorithmName     string
	HashValue         string
	SaltValue         string
	SpinCount         int
}

// SheetProtection holds worksheet-level protection metadata.
type SheetProtection struct {
	Password            string
	Sheet               bool
	Objects             bool
	Scenarios           bool
	FormatCells         bool
	FormatColumns       bool
	FormatRows          bool
	InsertColumns       bool
	InsertRows          bool
	InsertHyperlinks    bool
	DeleteColumns       bool
	DeleteRows          bool
	SelectLockedCells   bool
	Sort                bool
	AutoFilter          bool
	PivotTables         bool
	SelectUnlockedCells bool
	AlgorithmName       string
	HashValue           string
	SaltValue           string
	SpinCount           int
}

// PageSetup holds resolved print/page configuration for a worksheet.
type PageSetup struct {
	// PaperSize is the paper size code (1=letter, 9=A4, etc.).
	PaperSize int
	// Orientation is "portrait", "landscape", or "" (default).
	Orientation string
	// Scale is the print scale percentage (100 = 100%).
	Scale int
	// FitToPage reports whether fit-to-page scaling is active.
	FitToPage bool
	// FitToWidth is the number of pages wide (0 = auto).
	FitToWidth int
	// FitToHeight is the number of pages tall (0 = auto).
	FitToHeight int
	// FirstPageNumber is the authored starting page number.
	FirstPageNumber int
	// UseFirstPageNumber reports whether FirstPageNumber is active.
	UseFirstPageNumber bool
	// PageOrder is "downThenOver" or "overThenDown".
	PageOrder string
	// BlackAndWhite requests monochrome printing.
	BlackAndWhite bool
	// Draft requests draft-quality printing.
	Draft bool
	// HorizontalDPI and VerticalDPI are printer-resolution hints.
	HorizontalDPI int
	VerticalDPI   int
	// Copies is the requested print copy count.
	Copies int
	// MarginLeft/Right/Top/Bottom are the page margins in inches.
	MarginLeft   float64
	MarginRight  float64
	MarginTop    float64
	MarginBottom float64
	MarginHeader float64
	MarginFooter float64
	// CenterHorizontally centers the printout horizontally on the page
	// (HSSF HCenterRecord, OOXML printOptions/@horizontalCentered).
	CenterHorizontally bool
	// CenterVertically centers the printout vertically on the page
	// (HSSF VCenterRecord, OOXML printOptions/@verticalCentered).
	CenterVertically bool
	// PrintHeadings prints row numbers and column letters on the printout
	// (HSSF PrintHeadersRecord, OOXML printOptions/@headings).
	PrintHeadings bool
	// PrintGridlines prints sheet gridlines on the printout
	// (HSSF PrintGridlinesRecord, OOXML printOptions/@gridLines).
	PrintGridlines bool
}

// PivotTable is a resolved pivot table anchored to a worksheet.
type PivotTable struct {
	// Name is the user-visible pivot table name.
	Name string
	// LocationRef is the A1-notation range occupied by the pivot table, e.g. "A3:D10".
	LocationRef string
	// DataCaption is the caption shown for the Values area.
	DataCaption string
	// SourceSheet is the sheet containing the source data range (from the cache definition).
	// Empty when the source is a named range or external.
	SourceSheet string
	// SourceRef is the A1-notation range of the source data, e.g. "A1:D100".
	// Empty when the source is a named range or external.
	SourceRef string
	// SourceName is the defined-name source when SourceRef is empty.
	SourceName string
	// RowGrandTotals indicates whether row grand totals are shown.
	RowGrandTotals bool
	// ColGrandTotals indicates whether column grand totals are shown.
	ColGrandTotals bool
	// ShowHeaders indicates whether pivot field headers are shown.
	ShowHeaders bool
	// RowFields lists the names of fields placed on the row axis.
	RowFields []string
	// ColFields lists the names of fields placed on the column axis.
	ColFields []string
	// PageFields lists the names of fields used as report filters.
	PageFields []string
	// DataFields lists the value field descriptors.
	DataFields []*PivotDataField
	// Fields carries semantic pivot-field metadata, including item visibility.
	Fields []*PivotField
	// PageFieldSelections records the selected pivot-item index for report filters.
	// A negative item index means that all items are selected.
	PageFieldSelections []*PivotPageFieldSelection
}

// PivotField describes one semantic pivot field and its visible/cache items.
type PivotField struct {
	Name        string
	Axis        string
	DataField   bool
	Compact     bool
	Outline     bool
	SubtotalTop bool
	ShowAll     bool
	Items       []*PivotItem
}

// PivotItem describes one PivotTable item. CacheIndex references the corresponding
// cache shared-item entry when Type is data; -1 means there is no cache item.
type PivotItem struct {
	CacheIndex int
	Type       string
	Name       string
	Hidden     bool
	HideDetail bool
}

// PivotPageFieldSelection preserves the selected item for a report-filter field.
type PivotPageFieldSelection struct {
	FieldName string
	ItemIndex int
}

// PivotDataField describes one value field in a pivot table.
type PivotDataField struct {
	// Name is the column header shown in the pivot table (e.g. "Sum of Sales").
	Name string
	// SourceFieldName is the name of the underlying pivot field.
	SourceFieldName string
	// Subtotal is the aggregation function: "sum", "count", "average", etc.
	Subtotal string
	// NumFmtID is the workbook number-format ID applied to the value field.
	NumFmtID int
}
