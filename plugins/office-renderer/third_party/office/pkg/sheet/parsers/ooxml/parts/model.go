package parts

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/media"
)

// XDRScene3D stores a subset of DrawingML scene3d camera and lighting metadata.
type XDRScene3D struct {
	Camera   *XDRSceneCamera   `xml:"camera"`
	LightRig *XDRSceneLightRig `xml:"lightRig"`
}

// XDRSceneCamera stores the scene camera preset and rotation.
type XDRSceneCamera struct {
	Prst string            `xml:"prst,attr,omitempty"`
	Rot  *XDRSceneRotation `xml:"rot"`
}

// XDRSceneLightRig stores the scene light rig preset and direction.
type XDRSceneLightRig struct {
	Rig string `xml:"rig,attr,omitempty"`
	Dir string `xml:"dir,attr,omitempty"`
}

// XDRSceneRotation stores camera rotation values in OOXML angle units.
type XDRSceneRotation struct {
	Lat int64 `xml:"lat,attr,omitempty"`
	Lon int64 `xml:"lon,attr,omitempty"`
	Rev int64 `xml:"rev,attr,omitempty"`
}

// XDRShape3D stores a subset of DrawingML sp3d geometry metadata.
type XDRShape3D struct {
	Z             int64                 `xml:"z,attr,omitempty"`
	ExtrusionH    int64                 `xml:"extrusionH,attr,omitempty"`
	ContourW      int64                 `xml:"contourW,attr,omitempty"`
	PrstMaterial  string                `xml:"prstMaterial,attr,omitempty"`
	BevelT        *XDRShape3DBevel      `xml:"bevelT"`
	BevelB        *XDRShape3DBevel      `xml:"bevelB"`
	ContourClr    *XDRColorRef          `xml:"contourClr>srgbClr"`
	ContourScheme *drawingxml.SchemeClr `xml:"contourClr>schemeClr"`
	ExtrusionClr  *XDRColorRef          `xml:"extrusionClr>srgbClr"`
	ExtrudeScheme *drawingxml.SchemeClr `xml:"extrusionClr>schemeClr"`
}

// XDRShape3DBevel stores the bevel size for the top or bottom face.
type XDRShape3DBevel struct {
	W    int64  `xml:"w,attr,omitempty"`
	H    int64  `xml:"h,attr,omitempty"`
	Prst string `xml:"prst,attr,omitempty"`
}

// XDRColorRef stores a direct sRGB color value.
type XDRColorRef struct {
	Val string `xml:"val,attr"`
}

const (
	relTypeOfficeDocument    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	relTypeWorksheet         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"
	relTypeSharedStrings     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings"
	relTypeStyles            = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	relTypeTheme             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"
	relTypeImage             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	relTypeDrawing           = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing"
	relTypeChart             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart"
	relTypeFont              = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/font"
	relTypeHyperlink         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	relTypeComments          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	relTypeVMLDrawing        = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/vmlDrawing"
	relTypePivotTable        = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotTable"
	relTypePivotCache        = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotCacheDefinition"
	relTypePivotCacheRecords = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotCacheRecords"
	relTypeTable             = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/table"
	relTypeConnections       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/connections"
	relTypeExternalLink      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/externalLink"
	relTypeExternalLinkPath  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/externalLinkPath"
	relTypeOLEObject         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject"
	relTypePackage           = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/package"
	relTypeDiagramData       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData"
	relTypeDiagramLayout     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout"
	relTypeDiagramQuickStyle = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle"
	relTypeDiagramColors     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors"
	relTypeDiagramDrawing    = "http://schemas.microsoft.com/office/2007/relationships/diagramDrawing"

	drawingGraphicDataURIChartTransitional   = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	drawingGraphicDataURIChartStrict         = "http://purl.oclc.org/ooxml/drawingml/chart"
	drawingGraphicDataURIDiagramTransitional = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
	drawingGraphicDataURIDiagramStrict       = "http://purl.oclc.org/ooxml/drawingml/diagram"
	drawingGraphicDataURIOLE                 = "urn:ooxml:ole"
)

// Package contains the typed SpreadsheetML parts resolved from an OOXML package.
type Package struct {
	MainPartName  string
	Workbook      *Workbook
	SharedStrings *SharedStrings
	Styles        *StyleSheet
	Theme         *Theme
	FontTable     *FontTable
	Connections   *Connections
	ExternalLinks []*ExternalLinkEntry
	EmbeddedFonts []*EmbeddedFontData
	// Worksheets is ordered by the sheet sequence in Workbook.Sheets.
	Worksheets []*WorksheetEntry
	// ChartParts maps relationship IDs to chart XML data.
	ChartParts  map[string][]byte
	MediaParts  map[string]media.Part
	Diagnostics diag.List
}

// WorksheetEntry pairs a workbook sheet descriptor with its parsed worksheet.
type WorksheetEntry struct {
	// SheetID is the numeric sheet identifier from workbook.xml (sheetId attribute).
	SheetID int
	// RelID is the relationship ID (r:id) used to locate the worksheet part.
	RelID string
	// Name is the user-visible sheet tab name.
	Name string
	// PartName is the resolved OPC part name for this worksheet.
	PartName string
	// Sheet is the parsed worksheet content.
	Sheet *Worksheet
	// Drawing is the parsed spreadsheet drawing for this worksheet, if any.
	Drawing *SpreadsheetDrawing
	// DrawingMedia maps drawing-level relationship IDs to image media parts.
	DrawingMedia map[string]media.Part
	// DrawingChartParts maps drawing-level relationship IDs to chart XML data.
	DrawingChartParts map[string][]byte
	// DrawingDiagramParts preserves SmartArt diagram parts keyed by drawing relationship ID.
	DrawingDiagramParts map[string]PreservedPart
	// OLEParts preserves worksheet OLE/package payloads keyed by worksheet relationship ID.
	OLEParts map[string]PreservedPart
	// HyperlinkURLs maps worksheet hyperlink relationship IDs to their external URLs.
	HyperlinkURLs map[string]string
	// Comments is the parsed comments part for this worksheet, if any.
	Comments *Comments
	// LegacyNotes stores VML note metadata keyed by cell ref when a legacy note drawing is present.
	LegacyNotes map[string]LegacyNote
	// FormControls holds parsed VML-based form controls for this worksheet.
	FormControls []FormControl
	// VMLObjects holds generic worksheet VML shapes that are neither form controls nor comment notes.
	VMLObjects []VMLObject
	// HFImages holds header/footer VML picture shapes in the order they appear in the VML file.
	// Each element corresponds to one &G placeholder in the header/footer text strings.
	HFImages []HFImage
	// LegacyDrawingVML holds the raw bytes of the worksheet-level VML drawing part
	// referenced by legacyDrawing (form controls and comment indicators).
	LegacyDrawingVML []byte
	// LegacyDrawingHFVML holds the raw bytes of the VML drawing part referenced by
	// legacyDrawingHF (header/footer images).
	LegacyDrawingHFVML []byte
	// Tables holds all parsed table definitions for this worksheet.
	Tables []*Table
	// PivotTables holds all parsed pivot table definitions for this worksheet.
	PivotTables []*PivotTableWithCache
}

// WorkbookPr captures workbook-level properties.
type WorkbookPr struct {
	Date1904               bool  `xml:"date1904,attr,omitempty"`
	DefaultThemeVersion    int   `xml:"defaultThemeVersion,attr,omitempty"`
	BackupFile             *bool `xml:"backupFile,attr,omitempty"`
	SaveExternalLinkValues *bool `xml:"saveExternalLinkValues,attr,omitempty"`
}

// WorkbookView is one entry in xl/workbook.xml > bookViews. ECMA-376 §18.2.30.
type WorkbookView struct {
	ActiveTab *int `xml:"activeTab,attr,omitempty"`
}

// FileSharing represents xl/workbook.xml > fileSharing. ECMA-376 §18.2.6.
type FileSharing struct {
	UserName string `xml:"userName,attr,omitempty"`
}

// CalcPr represents xl/workbook.xml > calcPr. ECMA-376 §18.2.2.
type CalcPr struct {
	CalcMode       string   `xml:"calcMode,attr,omitempty"` // "auto"|"manual"|"autoNoTable"
	RefMode        string   `xml:"refMode,attr,omitempty"`  // "A1"|"R1C1"
	CalcID         *int     `xml:"calcId,attr,omitempty"`
	Iterate        *bool    `xml:"iterate,attr,omitempty"`
	IterateCount   *int     `xml:"iterateCount,attr,omitempty"`
	IterateDelta   *float64 `xml:"iterateDelta,attr,omitempty"`
	CalcOnSave     *bool    `xml:"calcOnSave,attr,omitempty"`
	FullPrecision  *bool    `xml:"fullPrecision,attr,omitempty"`
	FullCalcOnLoad *bool    `xml:"fullCalcOnLoad,attr,omitempty"`
	ForceFullCalc  *bool    `xml:"forceFullCalc,attr,omitempty"`
}

// Workbook represents xl/workbook.xml.
// ECMA-376 Part 1 §18.2.
type Workbook struct {
	XMLName            xml.Name            `xml:"workbook"`
	FileSharing        *FileSharing        `xml:"fileSharing"`
	WorkbookPr         *WorkbookPr         `xml:"workbookPr"`
	BookViews          []WorkbookView      `xml:"bookViews>workbookView"`
	Sheets             []SheetRef          `xml:"sheets>sheet"`
	ExternalReferences []ExternalReference `xml:"externalReferences>externalReference"`
	DefinedNames       []DefinedName       `xml:"definedNames>definedName"`
	PivotCaches        []PivotCache        `xml:"pivotCaches>pivotCache"`
	CalcPr             *CalcPr             `xml:"calcPr"`
	Protection         *WorkbookProtection `xml:"workbookProtection"`
}

// ExternalReference is a workbook-level reference to an external link part.
// ECMA-376 Part 1 §18.2.11.
type ExternalReference struct {
	RelID string `xml:"id,attr"`
}

// PivotCache is a workbook-level reference to a pivot cache definition part.
// ECMA-376 Part 1 §18.2.13.
type PivotCache struct {
	CacheID int    `xml:"cacheId,attr"`
	RelID   string `xml:"id,attr"`
}

// Connections represents xl/connections.xml.
type Connections struct {
	XMLName     xml.Name     `xml:"connections"`
	Connections []Connection `xml:"connection"`
}

// Connection is one workbook external data connection definition.
type Connection struct {
	ID            int               `xml:"id,attr"`
	Name          string            `xml:"name,attr,omitempty"`
	Type          string            `xml:"type,attr,omitempty"`
	Description   string            `xml:"description,attr,omitempty"`
	RefreshOnLoad bool              `xml:"refreshOnLoad,attr,omitempty"`
	Background    bool              `xml:"background,attr,omitempty"`
	SaveData      bool              `xml:"saveData,attr,omitempty"`
	SourceFile    string            `xml:"sourceFile,attr,omitempty"`
	ODCFile       string            `xml:"odcFile,attr,omitempty"`
	DBPr          *ConnectionDBPr   `xml:"dbPr"`
	OLAPPr        *ConnectionOLAPPr `xml:"olapPr"`
	TextPr        *ConnectionTextPr `xml:"textPr"`
	WebPr         *ConnectionWebPr  `xml:"webPr"`
}

// ConnectionDBPr contains database connection properties.
type ConnectionDBPr struct {
	Connection  string `xml:"connection,attr,omitempty"`
	Command     string `xml:"command,attr,omitempty"`
	CommandType string `xml:"commandType,attr,omitempty"`
}

// ConnectionOLAPPr contains OLAP connection properties.
type ConnectionOLAPPr struct {
	LocalConnection string `xml:"localConnection,attr,omitempty"`
	RowDrillCount   int    `xml:"rowDrillCount,attr,omitempty"`
}

// ConnectionTextPr contains text import properties.
type ConnectionTextPr struct {
	SourceFile string `xml:"sourceFile,attr,omitempty"`
	Delimiter  string `xml:"delimiter,attr,omitempty"`
	Qualifier  string `xml:"qualifier,attr,omitempty"`
}

// ConnectionWebPr contains web query properties.
type ConnectionWebPr struct {
	URL         string `xml:"url,attr,omitempty"`
	Post        string `xml:"post,attr,omitempty"`
	HtmlTables  bool   `xml:"htmlTables,attr,omitempty"`
	SourceData  bool   `xml:"sourceData,attr,omitempty"`
	ParsePre    bool   `xml:"parsePre,attr,omitempty"`
	Consecutive bool   `xml:"consecutive,attr,omitempty"`
	FirstRow    bool   `xml:"firstRow,attr,omitempty"`
	Xl97        bool   `xml:"xl97,attr,omitempty"`
	TextDates   bool   `xml:"textDates,attr,omitempty"`
	XL2000      bool   `xml:"xl2000,attr,omitempty"`
}

// ExternalLinkEntry pairs an externalLink part with its resolved target path metadata.
type ExternalLinkEntry struct {
	RelID    string
	PartName string
	Target   string
	Link     *ExternalLink
}

// ExternalLink represents xl/externalLinks/externalLinkN.xml.
type ExternalLink struct {
	XMLName      xml.Name      `xml:"externalLink"`
	ExternalBook *ExternalBook `xml:"externalBook"`
}

// ExternalBook captures workbook-level external reference metadata.
type ExternalBook struct {
	RelID        string                `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	SheetNames   []ExternalSheetName   `xml:"sheetNames>sheetName"`
	DefinedNames []ExternalDefinedName `xml:"definedNames>definedName"`
	SheetDataSet *ExternalSheetDataSet `xml:"sheetDataSet"`
}

// ExternalSheetName is one sheet name in an external workbook reference.
type ExternalSheetName struct {
	Val string `xml:"val,attr"`
}

// ExternalDefinedName is one externally defined name.
type ExternalDefinedName struct {
	Name     string `xml:"name,attr"`
	RefersTo string `xml:"refersTo,attr,omitempty"`
	SheetID  *int   `xml:"sheetId,attr"`
}

// ExternalSheetDataSet contains cached external sheet cell data.
type ExternalSheetDataSet struct {
	Sheets []ExternalSheetData `xml:"sheetData"`
}

// ExternalSheetData contains the cached rows for one external sheet.
type ExternalSheetData struct {
	SheetID      int                `xml:"sheetId,attr"`
	RefreshError bool               `xml:"refreshError,attr,omitempty"`
	Rows         []ExternalSheetRow `xml:"row"`
}

// ExternalSheetRow contains cached external cells for one row.
type ExternalSheetRow struct {
	Index int                 `xml:"r,attr"`
	Cells []ExternalSheetCell `xml:"cell"`
}

// ExternalSheetCell is one cached external cell value.
type ExternalSheetCell struct {
	Ref   string `xml:"r,attr"`
	Type  string `xml:"t,attr,omitempty"`
	Value string `xml:"v"`
}

// SheetRef is a reference to a worksheet within the workbook.
type SheetRef struct {
	Name    string `xml:"name,attr"`
	SheetID int    `xml:"sheetId,attr"`
	RelID   string `xml:"id,attr"`
	State   string `xml:"state,attr,omitempty"` // "hidden", "veryHidden", or absent
}

// DefinedName is a named formula or range.
type DefinedName struct {
	Name         string `xml:"name,attr"`
	LocalSheetID *int   `xml:"localSheetId,attr"`
	Hidden       bool   `xml:"hidden,attr,omitempty"`
	Value        string `xml:",chardata"`
}

// Worksheet represents xl/worksheets/sheetN.xml.
// ECMA-376 Part 1 §18.3.
type Worksheet struct {
	XMLName         xml.Name         `xml:"worksheet"`
	Dimension       *SheetDimension  `xml:"dimension"`
	SheetViews      []SheetView      `xml:"sheetViews>sheetView"`
	SheetFormatPr   *SheetFormatPr   `xml:"sheetFormatPr"`
	Cols            []ColRange       `xml:"cols>col"`
	Rows            []Row            `xml:"sheetData>row"`
	MergeCells      []MergeCell      `xml:"mergeCells>mergeCell"`
	Hyperlinks      []Hyperlink      `xml:"hyperlinks>hyperlink"`
	ConditionalFmts []ConditionalFmt `xml:"conditionalFormatting"`
	DataValidations []DataValidation `xml:"dataValidations>dataValidation"`
	AutoFilter      *AutoFilter      `xml:"autoFilter"`
	PrintOptions    *PrintOptions    `xml:"printOptions"`
	RowBreaks       *PageBreaks      `xml:"rowBreaks"`
	ColBreaks       *PageBreaks      `xml:"colBreaks"`
	PageMargins     *PageMargins     `xml:"pageMargins"`
	PageSetup       *PageSetup       `xml:"pageSetup"`
	SheetProtection *SheetProtection `xml:"sheetProtection"`
	DrawingRef      *DrawingRef      `xml:"drawing"`
	LegacyDrawing   *DrawingRef      `xml:"legacyDrawing"`
	LegacyDrawingHF *DrawingRef      `xml:"legacyDrawingHF"`
	OLEObjects      *OLEObjects      `xml:"oleObjects"`
	HeaderFooter    *HeaderFooter    `xml:"headerFooter"`
	TableParts      *TableParts      `xml:"tableParts"`
	PivotTables     *PivotTableRefs  `xml:"pivotTables"`
}

// OLEObjects holds worksheet embedded/linked OLE object declarations.
type OLEObjects struct {
	Objects []OLEObject `xml:"oleObject"`
}

// OLEObject is the worksheet-level semantic metadata for an embedded or linked OLE object.
type OLEObject struct {
	ProgID    string `xml:"progId,attr,omitempty"`
	DVAspect  string `xml:"dvAspect,attr,omitempty"`
	Link      string `xml:"link,attr,omitempty"`
	OLEUpdate string `xml:"oleUpdate,attr,omitempty"`
	AutoLoad  bool   `xml:"autoLoad,attr,omitempty"`
	ShapeID   int    `xml:"shapeId,attr,omitempty"`
	RelID     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// PreservedPart stores an OOXML part that is semantically associated with a worksheet object.
type PreservedPart struct {
	RelID         string
	RelType       string
	PartName      string
	ContentType   string
	Data          []byte
	External      bool
	Target        string
	Relationships []PreservedRelationship
	Children      map[string]PreservedPart
}

// PreservedRelationship retains an object-owned OPC relationship.
type PreservedRelationship struct {
	ID, Type, Target, TargetMode string
}

// TableParts holds references to table definitions via relationship IDs.
type TableParts struct {
	Count  int         `xml:"count,attr,omitempty"`
	Tables []TablePart `xml:"tablePart"`
}

// TablePart references a single table definition part via its relationship ID.
type TablePart struct {
	RelID string `xml:"id,attr"`
}

// PivotTableRefs holds references to pivot table definitions via relationship IDs.
type PivotTableRefs struct {
	Count  int              `xml:"count,attr,omitempty"`
	Tables []PivotTablePart `xml:"pivotTable"`
}

// PivotTablePart references a single pivot table definition part.
type PivotTablePart struct {
	RelID string `xml:"id,attr"`
	Name  string `xml:"name,attr,omitempty"`
}

// WorkbookProtection captures workbook-level protection metadata.
type WorkbookProtection struct {
	WorkbookPassword  string `xml:"workbookPassword,attr,omitempty"`
	RevisionsPassword string `xml:"revisionsPassword,attr,omitempty"`
	LockStructure     bool   `xml:"lockStructure,attr,omitempty"`
	LockWindows       bool   `xml:"lockWindows,attr,omitempty"`
	LockRevision      bool   `xml:"lockRevision,attr,omitempty"`
	AlgorithmName     string `xml:"algorithmName,attr,omitempty"`
	HashValue         string `xml:"hashValue,attr,omitempty"`
	SaltValue         string `xml:"saltValue,attr,omitempty"`
	SpinCount         int    `xml:"spinCount,attr,omitempty"`
}

// SheetProtection captures worksheet-level protection metadata.
type SheetProtection struct {
	Password            string `xml:"password,attr,omitempty"`
	Sheet               bool   `xml:"sheet,attr,omitempty"`
	Objects             bool   `xml:"objects,attr,omitempty"`
	Scenarios           bool   `xml:"scenarios,attr,omitempty"`
	FormatCells         bool   `xml:"formatCells,attr,omitempty"`
	FormatColumns       bool   `xml:"formatColumns,attr,omitempty"`
	FormatRows          bool   `xml:"formatRows,attr,omitempty"`
	InsertColumns       bool   `xml:"insertColumns,attr,omitempty"`
	InsertRows          bool   `xml:"insertRows,attr,omitempty"`
	InsertHyperlinks    bool   `xml:"insertHyperlinks,attr,omitempty"`
	DeleteColumns       bool   `xml:"deleteColumns,attr,omitempty"`
	DeleteRows          bool   `xml:"deleteRows,attr,omitempty"`
	SelectLockedCells   bool   `xml:"selectLockedCells,attr,omitempty"`
	Sort                bool   `xml:"sort,attr,omitempty"`
	AutoFilter          bool   `xml:"autoFilter,attr,omitempty"`
	PivotTables         bool   `xml:"pivotTables,attr,omitempty"`
	SelectUnlockedCells bool   `xml:"selectUnlockedCells,attr,omitempty"`
	AlgorithmName       string `xml:"algorithmName,attr,omitempty"`
	HashValue           string `xml:"hashValue,attr,omitempty"`
	SaltValue           string `xml:"saltValue,attr,omitempty"`
	SpinCount           int    `xml:"spinCount,attr,omitempty"`
}

// Hyperlink is a hyperlink element within a worksheet.
// ECMA-376 Part 1 §18.3.1.13.
// The hyperlink target is resolved through a worksheet relationship (r:id) for
// external URLs, or through the Location attribute for internal references.
type Hyperlink struct {
	// Ref is the cell or range address this hyperlink is attached to (e.g. "A1" or "A1:B3").
	Ref string `xml:"ref,attr"`
	// RelID is the worksheet-level relationship ID for external targets.
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	// Location is the internal navigation target (e.g. "Sheet2!A1"), used when RelID is absent.
	Location string `xml:"location,attr,omitempty"`
	// Display is the optional display text override.
	Display string `xml:"display,attr,omitempty"`
	// Tooltip is the optional tooltip text.
	Tooltip string `xml:"tooltip,attr,omitempty"`
}

// DrawingRef is the <drawing r:id="..."/> element referencing a drawing part.
type DrawingRef struct {
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// SheetDimension declares the used cell range.
type SheetDimension struct {
	Ref string `xml:"ref,attr"`
}

// SheetView describes a view configuration for this worksheet.
type SheetView struct {
	TabSelected    bool        `xml:"tabSelected,attr,omitempty"`
	WorkbookViewID int         `xml:"workbookViewId,attr"`
	ZoomScale      int         `xml:"zoomScale,attr,omitempty"`
	ShowGridLines  *bool       `xml:"showGridLines,attr"`
	Pane           *Pane       `xml:"pane"`
	Selections     []Selection `xml:"selection"`
}

// Selection describes one active selection within a worksheet pane.
type Selection struct {
	Pane         string `xml:"pane,attr,omitempty"`
	ActiveCell   string `xml:"activeCell,attr,omitempty"`
	ActiveCellID int    `xml:"activeCellId,attr,omitempty"`
	SqRef        string `xml:"sqref,attr,omitempty"`
}

// Pane describes a frozen or split pane configuration.
type Pane struct {
	XSplit      float64 `xml:"xSplit,attr,omitempty"`
	YSplit      float64 `xml:"ySplit,attr,omitempty"`
	TopLeftCell string  `xml:"topLeftCell,attr,omitempty"`
	ActivePane  string  `xml:"activePane,attr,omitempty"`
	State       string  `xml:"state,attr,omitempty"` // "frozen", "split"
}

// SheetFormatPr holds default row/column sizing.
// ECMA-376 §18.3.1.81.
type SheetFormatPr struct {
	DefaultColWidth  float64 `xml:"defaultColWidth,attr,omitempty"`
	DefaultRowHeight float64 `xml:"defaultRowHeight,attr,omitempty"`
	// CustomHeight signals that DefaultRowHeight was set explicitly rather than
	// derived from the active font; legacy XLS surfaces the same flag as
	// DefaultRowHeightUnsynced.
	CustomHeight bool `xml:"customHeight,attr,omitempty"`
	// ZeroHeight signals that rows of default height should be hidden.
	ZeroHeight bool `xml:"zeroHeight,attr,omitempty"`
	// OutlineLevelRow is the maximum outline level used by any row in the sheet.
	OutlineLevelRow int `xml:"outlineLevelRow,attr,omitempty"`
	// OutlineLevelCol is the maximum outline level used by any column in the sheet.
	OutlineLevelCol int `xml:"outlineLevelCol,attr,omitempty"`
}

// ColRange describes a contiguous run of columns with shared formatting.
type ColRange struct {
	Min          int     `xml:"min,attr"`
	Max          int     `xml:"max,attr"`
	Width        float64 `xml:"width,attr,omitempty"`
	CustomWidth  bool    `xml:"customWidth,attr,omitempty"`
	Hidden       bool    `xml:"hidden,attr,omitempty"`
	BestFit      bool    `xml:"bestFit,attr,omitempty"`
	StyleIndex   int     `xml:"style,attr,omitempty"`
	OutlineLevel int     `xml:"outlineLevel,attr,omitempty"`
	Collapsed    bool    `xml:"collapsed,attr,omitempty"`
}

// Row is a worksheet row.
type Row struct {
	Index        int     `xml:"r,attr"`
	Cells        []Cell  `xml:"c"`
	Height       float64 `xml:"ht,attr,omitempty"`
	CustomHt     bool    `xml:"customHeight,attr,omitempty"`
	Hidden       bool    `xml:"hidden,attr,omitempty"`
	StyleIndex   int     `xml:"s,attr,omitempty"`
	OutlineLevel int     `xml:"outlineLevel,attr,omitempty"`
	Collapsed    bool    `xml:"collapsed,attr,omitempty"`
}

// Cell is a single worksheet cell.
// ECMA-376 Part 1 §18.3.1.4.
type Cell struct {
	// Ref is the A1-style cell address, e.g. "A1".
	Ref string `xml:"r,attr"`
	// StyleIndex indexes into styleSheet/cellXfs.
	StyleIndex int `xml:"s,attr,omitempty"`
	// Type is the cell value type: "s" (shared string), "b" (boolean),
	// "n" (number, default), "str" (formula string), "inlineStr", "e" (error), "shared" (shared formula).
	Type string `xml:"t,attr,omitempty"`
	// Value holds the raw cell value (shared-string index for t="s", number otherwise).
	Value string `xml:"v"`
	// Formula holds the cell formula, if present.
	Formula *Formula `xml:"f"`
	// InlineStr holds the inline string value when t="inlineStr".
	InlineStr *RichText `xml:"is"`
}

// Formula is a worksheet cell formula (<f> element).
// ECMA-376 Part 1 §18.3.1.40.
type Formula struct {
	Type        string `xml:"t,attr,omitempty"`
	Ref         string `xml:"ref,attr,omitempty"`
	SharedIndex *int   `xml:"si,attr"`
	Text        string `xml:",chardata"`
}

// MergeCell records a merged cell range.
type MergeCell struct {
	Ref string `xml:"ref,attr"`
}

// ConditionalFmt records a conditional formatting region with all its rules.
// ECMA-376 Part 1 §18.3.1.18.
type ConditionalFmt struct {
	SqRef string   `xml:"sqref,attr"`
	Rules []CFRule `xml:"cfRule"`
}

// CFRule is one conditional formatting rule.
// ECMA-376 Part 1 §18.3.1.10.
type CFRule struct {
	Type       string   `xml:"type,attr,omitempty"`
	Operator   string   `xml:"operator,attr,omitempty"`
	DxfID      int      `xml:"dxfId,attr"`
	Priority   int      `xml:"priority,attr"`
	Percent    bool     `xml:"percent,attr,omitempty"`
	Bottom     bool     `xml:"bottom,attr,omitempty"`
	Rank       int      `xml:"rank,attr,omitempty"`
	Text       string   `xml:"text,attr,omitempty"`
	TimePeriod string   `xml:"timePeriod,attr,omitempty"`
	Formula    []string `xml:"formula"`
	// DataBar for data bar rules (type="dataBar")
	DataBar *DataBar `xml:"dataBar"`
	// IconSet for icon set rules (type="iconSet")
	IconSet *IconSet `xml:"iconSet"`
	// ColorScale for color scale rules (type="colorScale")
	ColorScale *ColorScale `xml:"colorScale"`
}

// Cfvo represents a conditional format value object (threshold).
// ECMA-376 Part 1 §18.3.1.11 (dataBar), §18.3.1.12 (iconSet), §18.3.1.13 (colorScale).
type Cfvo struct {
	Type string `xml:"type,attr"`
	Val  string `xml:"val,attr,omitempty"`
}

// DataBar represents a data bar conditional format.
// ECMA-376 Part 1 §18.3.1.11.
type DataBar struct {
	MinLength int       `xml:"minLength,attr,omitempty"`
	MaxLength int       `xml:"maxLength,attr,omitempty"`
	ShowValue bool      `xml:"showValue,attr,omitempty"`
	Color     *ColorVal `xml:"color"`
	Cfvo      []Cfvo    `xml:"cfvo"`
}

// IconSet represents an icon set conditional format.
// ECMA-376 Part 1 §18.3.1.12.
type IconSet struct {
	// ECMA-376 Part 1 §18.3.1.11: the attribute is named like its element,
	// <iconSet iconSet="3Arrows">; reading "set" left every rule on the default.
	Set       string `xml:"iconSet,attr,omitempty"`
	ShowValue bool   `xml:"showValue,attr,omitempty"`
	Reverse   bool   `xml:"reverse,attr,omitempty"`
	Cfvo      []Cfvo `xml:"cfvo"`
}

// ColorScale represents a color scale (heat map) conditional format.
// ECMA-376 Part 1 §18.3.1.13.
type ColorScale struct {
	Cfvo   []Cfvo            `xml:"cfvo"`
	Colors []ColorScaleColor `xml:"color"`
}

// ColorScaleColor represents a color stop in a color scale.
type ColorScaleColor struct {
	Theme   *int    `xml:"theme,attr"`
	RGB     string  `xml:"rgb,attr,omitempty"`
	Indexed *int    `xml:"indexed,attr"`
	Tint    float64 `xml:"tint,attr,omitempty"`
}

// ColorVal converts a color-scale stop into the shared SpreadsheetML color shape.
func (c ColorScaleColor) ColorVal() *ColorVal {
	return &ColorVal{
		Theme:   c.Theme,
		RGB:     c.RGB,
		Indexed: c.Indexed,
		Tint:    c.Tint,
	}
}

// DataValidation records one <dataValidation> element from a worksheet.
// ECMA-376 Part 1 §18.3.1.32.
type DataValidation struct {
	SqRef            string `xml:"sqref,attr"`
	Type             string `xml:"type,attr,omitempty"`
	Operator         string `xml:"operator,attr,omitempty"`
	ErrorStyle       string `xml:"errorStyle,attr,omitempty"`
	IMEMode          string `xml:"imeMode,attr,omitempty"`
	AllowBlank       bool   `xml:"allowBlank,attr,omitempty"`
	ShowDropDown     bool   `xml:"showDropDown,attr,omitempty"`
	ShowInputMessage bool   `xml:"showInputMessage,attr,omitempty"`
	ShowErrorMessage bool   `xml:"showErrorMessage,attr,omitempty"`
	PromptTitle      string `xml:"promptTitle,attr,omitempty"`
	Prompt           string `xml:"prompt,attr,omitempty"`
	ErrorTitle       string `xml:"errorTitle,attr,omitempty"`
	Error            string `xml:"error,attr,omitempty"`
	Formula1         string `xml:"formula1"`
	Formula2         string `xml:"formula2,omitempty"`
}

// AutoFilter records the <autoFilter ref="…"/> element in a worksheet.
// ECMA-376 Part 1 §18.3.1.2.
type AutoFilter struct {
	Ref           string         `xml:"ref,attr"`
	FilterColumns []FilterColumn `xml:"filterColumn"`
	SortState     *SortState     `xml:"sortState"`
}

// FilterColumn is one <filterColumn colId="…"> within an <autoFilter>.
// ECMA-376 Part 1 §18.3.2.7.
type FilterColumn struct {
	ColID         int            `xml:"colId,attr"`
	Filters       *FilterList    `xml:"filters"`
	CustomFilters *CustomFilters `xml:"customFilters"`
	Top10         *FilterTop10   `xml:"top10"`
	DynamicFilter *DynamicFilter `xml:"dynamicFilter"`
}

// FilterList holds <filters blank="…"> with discrete equality-match entries.
// ECMA-376 Part 1 §18.3.2.9.
type FilterList struct {
	Blank   bool     `xml:"blank,attr,omitempty"`
	Filters []Filter `xml:"filter"`
}

// Filter is one <filter val="…"/> entry within <filters>.
// ECMA-376 Part 1 §18.3.2.6.
type Filter struct {
	Val string `xml:"val,attr"`
}

// CustomFilters holds <customFilters and="…"> with up to two conditions.
// ECMA-376 Part 1 §18.3.2.4.
type CustomFilters struct {
	And    bool           `xml:"and,attr,omitempty"`
	Filter []CustomFilter `xml:"customFilter"`
}

// CustomFilter is one <customFilter operator="…" val="…"/> entry.
// ECMA-376 Part 1 §18.3.2.3.
type CustomFilter struct {
	Operator string `xml:"operator,attr"`
	Val      string `xml:"val,attr"`
}

// FilterTop10 holds the <top10 top="…" percent="…" val="…"/> element.
// ECMA-376 Part 1 §18.3.2.10.
type FilterTop10 struct {
	Top     bool    `xml:"top,attr,omitempty"`
	Percent bool    `xml:"percent,attr,omitempty"`
	Val     float64 `xml:"val,attr"`
}

// DynamicFilter holds the <dynamicFilter> element inside one autoFilter column.
// ECMA-376 Part 1 §18.3.2.5.
type DynamicFilter struct {
	Type      string   `xml:"type,attr,omitempty"`
	Val       *float64 `xml:"val,attr,omitempty"`
	MaxVal    *float64 `xml:"maxVal,attr,omitempty"`
	ValISO    string   `xml:"valIso,attr,omitempty"`
	MaxValISO string   `xml:"maxValIso,attr,omitempty"`
}

// SortState records the <sortState> element nested under an autoFilter.
// ECMA-376 Part 1 §18.3.1.92.
type SortState struct {
	Ref           string          `xml:"ref,attr,omitempty"`
	CaseSensitive bool            `xml:"caseSensitive,attr,omitempty"`
	Conditions    []SortCondition `xml:"sortCondition"`
}

// SortCondition is one <sortCondition> inside a sortState.
// ECMA-376 Part 1 §18.3.1.91.
type SortCondition struct {
	Ref        string `xml:"ref,attr,omitempty"`
	Descending bool   `xml:"descending,attr,omitempty"`
	SortBy     string `xml:"sortBy,attr,omitempty"`
	CustomList string `xml:"customList,attr,omitempty"`
	DxfID      *int   `xml:"dxfId,attr,omitempty"`
	IconSet    string `xml:"iconSet,attr,omitempty"`
	IconID     *int   `xml:"iconId,attr,omitempty"`
}

// PrintOptions records the <printOptions> element in a worksheet.
// ECMA-376 Part 1 §18.3.1.66.
type PrintOptions struct {
	HorizontalCentered bool `xml:"horizontalCentered,attr,omitempty"`
	VerticalCentered   bool `xml:"verticalCentered,attr,omitempty"`
	Headings           bool `xml:"headings,attr,omitempty"`
	GridLines          bool `xml:"gridLines,attr,omitempty"`
}

// PageBreaks records the <rowBreaks> or <colBreaks> element in a worksheet.
// ECMA-376 Part 1 §18.3.1.71 / §18.3.1.13.
type PageBreaks struct {
	Count            int         `xml:"count,attr,omitempty"`
	ManualBreakCount int         `xml:"manualBreakCount,attr,omitempty"`
	Breaks           []PageBreak `xml:"brk"`
}

// PageBreak is one <brk> entry inside rowBreaks/colBreaks.
type PageBreak struct {
	ID     int  `xml:"id,attr"`
	Min    int  `xml:"min,attr,omitempty"`
	Max    int  `xml:"max,attr,omitempty"`
	Manual bool `xml:"man,attr,omitempty"`
}

// PageMargins records the <pageMargins> element in a worksheet.
// ECMA-376 Part 1 §18.3.1.62.
type PageMargins struct {
	Left   float64 `xml:"left,attr,omitempty"`
	Right  float64 `xml:"right,attr,omitempty"`
	Top    float64 `xml:"top,attr,omitempty"`
	Bottom float64 `xml:"bottom,attr,omitempty"`
	Header float64 `xml:"header,attr,omitempty"`
	Footer float64 `xml:"footer,attr,omitempty"`
}

// HeaderFooter records the <headerFooter> element in a worksheet.
// ECMA-376 Part 1 §18.3.1.46.
type HeaderFooter struct {
	DifferentOddEven bool   `xml:"differentOddEven,attr,omitempty"`
	DifferentFirst   bool   `xml:"differentFirst,attr,omitempty"`
	ScaleWithDoc     bool   `xml:"scaleWithDoc,attr,omitempty"`
	AlignWithMargins bool   `xml:"alignWithMargins,attr,omitempty"`
	OddHeader        string `xml:"oddHeader"`
	OddFooter        string `xml:"oddFooter"`
	EvenHeader       string `xml:"evenHeader"`
	EvenFooter       string `xml:"evenFooter"`
	FirstHeader      string `xml:"firstHeader"`
	FirstFooter      string `xml:"firstFooter"`
}

// PageSetup records the <pageSetup> element in a worksheet.
// ECMA-376 Part 1 §18.3.1.63.
type PageSetup struct {
	PaperSize          int    `xml:"paperSize,attr,omitempty"`
	Orientation        string `xml:"orientation,attr,omitempty"` // "portrait" | "landscape"
	FitToPage          bool   `xml:"fitToPage,attr,omitempty"`
	FitToWidth         int    `xml:"fitToWidth,attr,omitempty"`
	FitToHeight        int    `xml:"fitToHeight,attr,omitempty"`
	Scale              int    `xml:"scale,attr,omitempty"`
	FirstPageNumber    int    `xml:"firstPageNumber,attr,omitempty"`
	UseFirstPageNumber bool   `xml:"useFirstPageNumber,attr,omitempty"`
	PageOrder          string `xml:"pageOrder,attr,omitempty"`
	BlackAndWhite      bool   `xml:"blackAndWhite,attr,omitempty"`
	Draft              bool   `xml:"draft,attr,omitempty"`
	HorizontalDPI      int    `xml:"horizontalDpi,attr,omitempty"`
	VerticalDPI        int    `xml:"verticalDpi,attr,omitempty"`
	Copies             int    `xml:"copies,attr,omitempty"`
}

// Comments represents xl/comments.xml.
// ECMA-376 Part 1 §18.7.
type Comments struct {
	XMLName     xml.Name       `xml:"comments"`
	Authors     []string       `xml:"authors>author"`
	CommentList []CommentEntry `xml:"commentList>comment"`
}

// CommentEntry is one cell comment (note).
// ECMA-376 Part 1 §18.7.3.
type CommentEntry struct {
	// Ref is the A1-style cell address.
	Ref      string   `xml:"ref,attr"`
	AuthorID int      `xml:"authorId,attr,omitempty"`
	Text     RichText `xml:"text"`
}

// LegacyNote is note-shape metadata extracted from a worksheet VML drawing.
type LegacyNote struct {
	Ref    string
	Row    int
	Col    int
	Anchor string
}

type vmlDrawing struct {
	Shapes []vmlShape `xml:"urn:schemas-microsoft-com:vml shape"`
}

type vmlShape struct {
	AltText    string         `xml:"alt,attr,omitempty"`
	ImageData  *vmlImageData  `xml:"urn:schemas-microsoft-com:vml imagedata"`
	ClientData *vmlClientData `xml:"urn:schemas-microsoft-com:office:excel ClientData"`
}

// vmlImageData holds the relationship id for a VML image shape.
type vmlImageData struct {
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// HFImage holds image media for a header/footer VML picture shape.
type HFImage struct {
	// ContentType is the MIME type of the image (e.g. "image/png").
	ContentType string
	// Data is the raw image bytes.
	Data []byte
}

type vmlClientData struct {
	ObjectType  string `xml:"ObjectType,attr"`
	Row         int    `xml:"urn:schemas-microsoft-com:office:excel Row"`
	Column      int    `xml:"urn:schemas-microsoft-com:office:excel Column"`
	Anchor      string `xml:"urn:schemas-microsoft-com:office:excel Anchor"`
	FmlaLink    string `xml:"urn:schemas-microsoft-com:office:excel FmlaLink"`
	FmlaGroup   string `xml:"urn:schemas-microsoft-com:office:excel FmlaGroup"`
	FmlaRange   string `xml:"urn:schemas-microsoft-com:office:excel FmlaRange"`
	FirstButton string `xml:"urn:schemas-microsoft-com:office:excel FirstButton"`
	SelType     string `xml:"urn:schemas-microsoft-com:office:excel SelType"`
	Sel         []int  `xml:"urn:schemas-microsoft-com:office:excel Sel"`
	FmlaMacro   string `xml:"urn:schemas-microsoft-com:office:excel FmlaMacro"`
	Checked     string `xml:"urn:schemas-microsoft-com:office:excel Checked"`
	Min         int    `xml:"urn:schemas-microsoft-com:office:excel Min"`
	Max         int    `xml:"urn:schemas-microsoft-com:office:excel Max"`
	Inc         int    `xml:"urn:schemas-microsoft-com:office:excel Inc"`
	Page        int    `xml:"urn:schemas-microsoft-com:office:excel Page"`
	Horiz       string `xml:"urn:schemas-microsoft-com:office:excel Horiz"`
	Val         int    `xml:"urn:schemas-microsoft-com:office:excel Val"`
	DropLines   int    `xml:"urn:schemas-microsoft-com:office:excel DropLines"`
}

// VMLObject is a generic editable worksheet VML shape. Unknown VML children are
// preserved in the raw drawing when an existing object is updated.
type VMLObject struct {
	ID          string
	Type        string
	Style       string
	FillColor   string
	StrokeColor string
	Text        string
	Anchor      string
}

// FormControl holds metadata for a VML-based Excel form control (button, checkbox, etc.).
type FormControl struct {
	// Type is the control type: "Button", "Checkbox", "List", "Drop", "Spin", "Scroll", "GBox", "Label", "Edit", "Radio".
	Type string
	// AltText is the shape's alt attribute (often the control's caption).
	AltText string
	// Anchor is the raw VML anchor: "col1, colOff, row1, rowOff, col2, colOff2, row2, rowOff2" (0-based col/row, pixel offsets).
	Anchor string
	// Checked is true for Checkbox/Radio controls that are initially checked.
	Checked bool
	// Mixed is true for tri-state Checkbox controls that are initially indeterminate.
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
	Min, Max, Val       int
	Increment, PageStep int
	Horizontal          bool
	// DropLines is the number of visible rows for List/Drop controls when specified.
	DropLines int
}

// SharedStrings represents xl/sharedStrings.xml.
// ECMA-376 Part 1 §18.4.
type SharedStrings struct {
	XMLName     xml.Name   `xml:"sst"`
	Count       int        `xml:"count,attr,omitempty"`
	UniqueCount int        `xml:"uniqueCount,attr,omitempty"`
	Items       []RichText `xml:"si"`
}

// RichText is a shared-string item. It may contain either a plain <t> or
// one or more rich-text runs <r>.
type RichText struct {
	// PlainText is the content of the <t> child when no <r> runs are present.
	PlainText string  `xml:"t"`
	Runs      []RTRun `xml:"r"`
}

// PlainValue returns the plain string value, collapsing rich-text runs if present.
func (rt *RichText) PlainValue() string {
	if len(rt.Runs) == 0 {
		return rt.PlainText
	}
	var b []byte
	for _, r := range rt.Runs {
		b = append(b, r.Text...)
	}
	return string(b)
}

// RTRun is a rich-text run within a shared-string item.
type RTRun struct {
	Text       string   `xml:"t"`
	Properties *RTProps `xml:"rPr"`
}

// RTProps holds run-level formatting for a rich-text run.
type RTProps struct {
	Bold      *struct{}  `xml:"b"`
	Italic    *struct{}  `xml:"i"`
	Underline *StringVal `xml:"u"`
	Strike    *struct{}  `xml:"strike"`
	VertAlign *StringVal `xml:"vertAlign"`
	FontSize  *FloatVal  `xml:"sz"`
	Color     *ColorVal  `xml:"color"`
	FontName  *StringVal `xml:"name"`
}

// StyleSheet represents xl/styles.xml.
// ECMA-376 Part 1 §18.8.
type StyleSheet struct {
	XMLName      xml.Name     `xml:"styleSheet"`
	NumFmts      []NumFmt     `xml:"numFmts>numFmt"`
	Fonts        []Font       `xml:"fonts>font"`
	Fills        []Fill       `xml:"fills>fill"`
	Borders      []Border     `xml:"borders>border"`
	Colors       *StyleColors `xml:"colors"`
	CellStyleXfs []Xf         `xml:"cellStyleXfs>xf"`
	CellXfs      []Xf         `xml:"cellXfs>xf"`
	CellStyles   []CellStyle  `xml:"cellStyles>cellStyle"`
	Dxfs         []Dxf        `xml:"dxfs>dxf"`
}

// StyleColors holds optional stylesheet-scoped color indexes.
type StyleColors struct {
	IndexedColors *IndexedColors `xml:"indexedColors"`
}

// IndexedColors stores SpreadsheetML indexed color palette entries.
type IndexedColors struct {
	RGBColors []IndexedRGBColor `xml:"rgbColor"`
}

// IndexedRGBColor is one indexed palette entry.
type IndexedRGBColor struct {
	RGB string `xml:"rgb,attr"`
}

// Dxf is a differential format entry (used by conditional formatting).
// ECMA-376 Part 1 §18.8.14.
type Dxf struct {
	Font   *DxfFont   `xml:"font"`
	NumFmt *NumFmt    `xml:"numFmt"`
	Fill   *DxfFill   `xml:"fill"`
	Border *DxfBorder `xml:"border"`
}

// DxfFont holds font overrides inside a dxf.
type DxfFont struct {
	Bold      *struct{} `xml:"b"`
	Italic    *struct{} `xml:"i"`
	Underline *struct{} `xml:"u"`
	Strike    *struct{} `xml:"strike"`
	Color     *ColorVal `xml:"color"`
}

// DxfFill holds fill overrides inside a dxf.
type DxfFill struct {
	PatternFill *PatternFill `xml:"patternFill"`
}

// DxfBorder holds border overrides inside a differential format.
type DxfBorder struct {
	Left   *BorderSide `xml:"left"`
	Right  *BorderSide `xml:"right"`
	Top    *BorderSide `xml:"top"`
	Bottom *BorderSide `xml:"bottom"`
}

// FontTable represents an embedded font table if present.
type FontTable struct {
	XMLName     xml.Name    `xml:"fonts"`
	Fonts       []*FontDecl `xml:"font"`
	Diagnostics diag.List   `xml:"-"`
}

// FontDecl describes a font table entry.
type FontDecl struct {
	Name            string        `xml:"name,attr"`
	EmbedRegular    *FontEmbedRef `xml:"embedRegular"`
	EmbedBold       *FontEmbedRef `xml:"embedBold"`
	EmbedItalic     *FontEmbedRef `xml:"embedItalic"`
	EmbedBoldItalic *FontEmbedRef `xml:"embedBoldItalic"`
}

// FontEmbedRef points to an embedded font part via a relationship ID.
type FontEmbedRef struct {
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	// FontKey is the OOXML fontKey GUID needed to undo ODTTF obfuscation.
	FontKey string `xml:"fontKey,attr,omitempty"`
}

type EmbeddedFontData = fontdata.EmbeddedFontData
type FontPart = fontdata.FontPart

// Theme represents the DrawingML workbook theme part.
type Theme struct {
	XMLName  xml.Name      `xml:"theme"`
	Name     string        `xml:"name,attr,omitempty"`
	Elements ThemeElements `xml:"themeElements"`
}

// ThemeElements holds the theme color scheme.
type ThemeElements struct {
	ClrScheme ColorScheme `xml:"clrScheme"`
}

// ColorScheme holds the named theme colors.
type ColorScheme struct {
	Name     string      `xml:"name,attr,omitempty"`
	DK1      *ThemeColor `xml:"dk1"`
	LT1      *ThemeColor `xml:"lt1"`
	DK2      *ThemeColor `xml:"dk2"`
	LT2      *ThemeColor `xml:"lt2"`
	Accent1  *ThemeColor `xml:"accent1"`
	Accent2  *ThemeColor `xml:"accent2"`
	Accent3  *ThemeColor `xml:"accent3"`
	Accent4  *ThemeColor `xml:"accent4"`
	Accent5  *ThemeColor `xml:"accent5"`
	Accent6  *ThemeColor `xml:"accent6"`
	HLink    *ThemeColor `xml:"hlink"`
	FolHLink *ThemeColor `xml:"folHlink"`
}

// ThemeColor stores either an sRGB value or a system-color fallback.
type ThemeColor struct {
	SrgbClr *SrgbColor `xml:"srgbClr"`
	SysClr  *SysColor  `xml:"sysClr"`
}

// ThemeColorTransforms preserves DrawingML child transforms on workbook theme
// colors so the resolver can apply them when building style/chart color maps.
type ThemeColorTransforms struct {
	Alpha    *drawingxml.ColorTransform `xml:"alpha"`
	AlphaMod *drawingxml.ColorTransform `xml:"alphaMod"`
	AlphaOff *drawingxml.ColorTransform `xml:"alphaOff"`
	Shade    *drawingxml.ColorTransform `xml:"shade"`
	Tint     *drawingxml.ColorTransform `xml:"tint"`
	Red      *drawingxml.ColorTransform `xml:"red"`
	RedMod   *drawingxml.ColorTransform `xml:"redMod"`
	RedOff   *drawingxml.ColorTransform `xml:"redOff"`
	Green    *drawingxml.ColorTransform `xml:"green"`
	GreenMod *drawingxml.ColorTransform `xml:"greenMod"`
	GreenOff *drawingxml.ColorTransform `xml:"greenOff"`
	Blue     *drawingxml.ColorTransform `xml:"blue"`
	BlueMod  *drawingxml.ColorTransform `xml:"blueMod"`
	BlueOff  *drawingxml.ColorTransform `xml:"blueOff"`
	Hue      *drawingxml.ColorTransform `xml:"hue"`
	Sat      *drawingxml.ColorTransform `xml:"sat"`
	Lum      *drawingxml.ColorTransform `xml:"lum"`
	LumMod   *drawingxml.ColorTransform `xml:"lumMod"`
	LumOff   *drawingxml.ColorTransform `xml:"lumOff"`
	SatMod   *drawingxml.ColorTransform `xml:"satMod"`
	SatOff   *drawingxml.ColorTransform `xml:"satOff"`
	HueMod   *drawingxml.ColorTransform `xml:"hueMod"`
	HueOff   *drawingxml.ColorTransform `xml:"hueOff"`
	Gray     *struct{}                  `xml:"gray"`
	Comp     *struct{}                  `xml:"comp"`
	Inv      *struct{}                  `xml:"inv"`
	Gamma    *struct{}                  `xml:"gamma"`
	InvGamma *struct{}                  `xml:"invGamma"`
}

type SrgbColor struct {
	Val string `xml:"val,attr"`
	ThemeColorTransforms
}

type SysColor struct {
	Val     string `xml:"val,attr"`
	LastClr string `xml:"lastClr,attr,omitempty"`
	ThemeColorTransforms
}

// NumFmt is a number format definition.
type NumFmt struct {
	ID         int    `xml:"numFmtId,attr"`
	FormatCode string `xml:"formatCode,attr"`
}

// Font describes a font entry in the stylesheet.
type Font struct {
	Bold      *struct{}  `xml:"b"`
	Italic    *struct{}  `xml:"i"`
	Underline *StringVal `xml:"u"`
	Strike    *struct{}  `xml:"strike"`
	VertAlign *StringVal `xml:"vertAlign"`
	Size      *FloatVal  `xml:"sz"`
	Color     *ColorVal  `xml:"color"`
	Name      *StringVal `xml:"name"`
	Family    *IntVal    `xml:"family"`
	Scheme    *StringVal `xml:"scheme"`
}

// Fill describes a fill entry.
type Fill struct {
	PatternFill *PatternFill `xml:"patternFill"`
}

// PatternFill describes a pattern fill.
type PatternFill struct {
	PatternType string    `xml:"patternType,attr,omitempty"`
	FgColor     *ColorVal `xml:"fgColor"`
	BgColor     *ColorVal `xml:"bgColor"`
}

// Border describes a border entry.
type Border struct {
	Left         *BorderSide `xml:"left"`
	Right        *BorderSide `xml:"right"`
	Top          *BorderSide `xml:"top"`
	Bottom       *BorderSide `xml:"bottom"`
	Diagonal     *BorderSide `xml:"diagonal"`
	Vertical     *BorderSide `xml:"vertical"`
	Horizontal   *BorderSide `xml:"horizontal"`
	DiagonalUp   bool        `xml:"diagonalUp,attr,omitempty"`
	DiagonalDown bool        `xml:"diagonalDown,attr,omitempty"`
}

// BorderSide is one side of a border.
type BorderSide struct {
	Style string    `xml:"style,attr,omitempty"`
	Color *ColorVal `xml:"color"`
}

// Xf is a cell format (xf) record, indexing font/fill/border/numFmt.
type Xf struct {
	NumFmtID          int             `xml:"numFmtId,attr"`
	FontID            int             `xml:"fontId,attr"`
	FillID            int             `xml:"fillId,attr"`
	BorderID          int             `xml:"borderId,attr"`
	XfID              int             `xml:"xfId,attr,omitempty"`
	ApplyNumberFormat bool            `xml:"applyNumberFormat,attr,omitempty"`
	ApplyFont         bool            `xml:"applyFont,attr,omitempty"`
	ApplyFill         bool            `xml:"applyFill,attr,omitempty"`
	ApplyBorder       bool            `xml:"applyBorder,attr,omitempty"`
	ApplyAlignment    bool            `xml:"applyAlignment,attr,omitempty"`
	ApplyProtection   bool            `xml:"applyProtection,attr,omitempty"`
	Alignment         *Alignment      `xml:"alignment"`
	Protection        *CellProtection `xml:"protection"`
}

// Alignment describes cell alignment.
type Alignment struct {
	Horizontal   string `xml:"horizontal,attr,omitempty"`
	Vertical     string `xml:"vertical,attr,omitempty"`
	WrapText     bool   `xml:"wrapText,attr,omitempty"`
	ShrinkToFit  bool   `xml:"shrinkToFit,attr,omitempty"`
	Indent       int    `xml:"indent,attr,omitempty"`
	TextRotation int    `xml:"textRotation,attr,omitempty"`
}

// CellProtection captures xf-level cell locking and formula-hiding flags.
type CellProtection struct {
	Locked        bool `xml:"locked,attr,omitempty"`
	FormulaHidden bool `xml:"hidden,attr,omitempty"`
}

// CellStyle is a named cell style linking a name to a cellStyleXfs index.
type CellStyle struct {
	Name      string `xml:"name,attr"`
	XfID      int    `xml:"xfId,attr"`
	BuiltinID int    `xml:"builtinId,attr,omitempty"`
}

// FloatVal wraps an XML attribute-based float value.
type FloatVal struct {
	Val float64 `xml:"val,attr"`
}

// IntVal wraps an XML attribute-based int value.
type IntVal struct {
	Val int `xml:"val,attr"`
}

// StringVal wraps an XML attribute-based string value.
type StringVal struct {
	Val string `xml:"val,attr"`
}

// ColorVal represents an OOXML color with optional theme, rgb, or indexed reference.
type ColorVal struct {
	Theme   *int    `xml:"theme,attr"`
	RGB     string  `xml:"rgb,attr,omitempty"`
	Indexed *int    `xml:"indexed,attr"`
	Tint    float64 `xml:"tint,attr,omitempty"`
}

// ── Spreadsheet Drawing (xl/drawings/drawingN.xml) ───────────────────────────
// ECMA-376 Part 1 §20.5 (SpreadsheetDrawing).

const (
	nsXDR      = "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"
	nsDMLMain  = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsRelships = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsDMLChart = "http://schemas.openxmlformats.org/drawingml/2006/chart"
)

// SpreadsheetDrawing is the root element of xl/drawings/drawingN.xml.
type SpreadsheetDrawing struct {
	XMLName         xml.Name            `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing wsDr"`
	OneCellAnchors  []XDROneCellAnchor  `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing oneCellAnchor"`
	TwoCellAnchors  []XDRTwoCellAnchor  `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing twoCellAnchor"`
	AbsoluteAnchors []XDRAbsoluteAnchor `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing absoluteAnchor"`
}

// UnmarshalXML preserves the interleaved document order of DrawingML anchors.
// Excel uses anchor element order as drawing stacking order; decoding each anchor
// kind into a separate slice otherwise loses that information.
func (d *SpreadsheetDrawing) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	d.XMLName = start.Name
	order := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space != nsXDR {
				if err := dec.Skip(); err != nil {
					return err
				}
				continue
			}
			switch t.Name.Local {
			case "oneCellAnchor":
				var a XDROneCellAnchor
				if err := dec.DecodeElement(&a, &t); err != nil {
					return err
				}
				a.DocumentOrder = order
				order++
				d.OneCellAnchors = append(d.OneCellAnchors, a)
			case "twoCellAnchor":
				var a XDRTwoCellAnchor
				if err := dec.DecodeElement(&a, &t); err != nil {
					return err
				}
				a.DocumentOrder = order
				order++
				d.TwoCellAnchors = append(d.TwoCellAnchors, a)
			case "absoluteAnchor":
				var a XDRAbsoluteAnchor
				if err := dec.DecodeElement(&a, &t); err != nil {
					return err
				}
				a.DocumentOrder = order
				order++
				d.AbsoluteAnchors = append(d.AbsoluteAnchors, a)
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// XDRMarker is xdr:from / xdr:to — a cell-based anchor point.
type XDRMarker struct {
	Col    int   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing col"`
	ColOff int64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing colOff"`
	Row    int   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing row"`
	RowOff int64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing rowOff"`
}

// XDRExtent is xdr:ext — size of a drawing object in EMUs.
type XDRExtent struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

// XDRPos is xdr:pos — absolute position in EMUs.
type XDRPos struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// XDROneCellAnchor is xdr:oneCellAnchor.
type XDRClientData struct {
	LocksWithSheet  *bool `xml:"fLocksWithSheet,attr"`
	PrintsWithSheet *bool `xml:"fPrintsWithSheet,attr"`
}

type XDROneCellAnchor struct {
	DocumentOrder int              `xml:"-"`
	From          XDRMarker        `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing from"`
	Ext           XDRExtent        `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing ext"`
	Pic           *XDRPic          `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing pic"`
	GraphicFrame  *XDRGraphicFrame `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing graphicFrame"`
	Shape         *XDRShape        `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing sp"`
	GroupShape    *XDRGroupShape   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing grpSp"`
	ClientData    *XDRClientData   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing clientData"`
}

// XDRTwoCellAnchor is xdr:twoCellAnchor.
type XDRTwoCellAnchor struct {
	DocumentOrder int              `xml:"-"`
	EditAs        string           `xml:"editAs,attr,omitempty"`
	From          XDRMarker        `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing from"`
	To            XDRMarker        `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing to"`
	Pic           *XDRPic          `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing pic"`
	GraphicFrame  *XDRGraphicFrame `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing graphicFrame"`
	Shape         *XDRShape        `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing sp"`
	GroupShape    *XDRGroupShape   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing grpSp"`
	ClientData    *XDRClientData   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing clientData"`
}

// XDRAbsoluteAnchor is xdr:absoluteAnchor.
type XDRAbsoluteAnchor struct {
	DocumentOrder int              `xml:"-"`
	Pos           XDRPos           `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing pos"`
	Ext           XDRExtent        `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing ext"`
	Pic           *XDRPic          `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing pic"`
	GraphicFrame  *XDRGraphicFrame `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing graphicFrame"`
	Shape         *XDRShape        `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing sp"`
	GroupShape    *XDRGroupShape   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing grpSp"`
	ClientData    *XDRClientData   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing clientData"`
}

// XDRCNvPr is xdr:cNvPr — common non-visual drawing properties.
type XDRCNvPr struct {
	ID    int    `xml:"id,attr"`
	Name  string `xml:"name,attr"`
	Title string `xml:"title,attr,omitempty"`
	Descr string `xml:"descr,attr,omitempty"`
}

// XDRPic is xdr:pic — a raster/vector picture element.
type XDRPic struct {
	NvPicPr  XDRNvPicPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing nvPicPr"`
	BlipFill XDRBlipFill `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing blipFill"`
	SpPr     XDRSpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing spPr"`
}

// XDRNvPicPr is xdr:nvPicPr.
type XDRNvPicPr struct {
	CNvPr XDRCNvPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvPr"`
}

// XDRBlipFill is xdr:blipFill.
type XDRBlipFill struct {
	Blip    XDRBlip     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip"`
	SrcRect *XDRSrcRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srcRect"`
}

// XDRSrcRect is a:srcRect. Values are thousandths of a percent (0..100000)
// cropped from each source-image edge.
type XDRSrcRect struct {
	Left   int `xml:"l,attr,omitempty"`
	Top    int `xml:"t,attr,omitempty"`
	Right  int `xml:"r,attr,omitempty"`
	Bottom int `xml:"b,attr,omitempty"`
}

// XDRBlip is a:blip — references an image via relationship ID.
type XDRBlip struct {
	Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
}

// XDRGraphicFrame is xdr:graphicFrame — wraps charts and other graphic objects.
type XDRGraphicFrame struct {
	NvGraphicFramePr XDRNvGraphicFramePr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing nvGraphicFramePr"`
	Xfrm             *XDRTransform2D     `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing xfrm"`
	Graphic          XDRGraphic          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphic"`
}

// XDRNvGraphicFramePr is xdr:nvGraphicFramePr.
type XDRNvGraphicFramePr struct {
	CNvPr XDRCNvPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvPr"`
}

// XDRGraphic is a:graphic.
type XDRGraphic struct {
	GraphicData XDRGraphicData `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphicData"`
}

// XDRGraphicData is a:graphicData.
type XDRGraphicData struct {
	URI    string            `xml:"uri,attr"`
	Chart  *XDRChart         `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart chart"`
	RelIDs *XDRDiagramRelIDs `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram relIds"`
}

// XDRDiagramRelIDs references the four core SmartArt definition parts.
type XDRDiagramRelIDs struct {
	Data       string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships dm,attr,omitempty"`
	Layout     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships lo,attr,omitempty"`
	QuickStyle string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships qs,attr,omitempty"`
	Colors     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships cs,attr,omitempty"`
}

// XDRChart is c:chart — references a chart part via relationship ID.
type XDRChart struct {
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// XDRShape is xdr:sp — a shape element.
type XDRShape struct {
	NvSpPr XDRNvSpPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing nvSpPr"`
	SpPr   XDRSpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing spPr"`
	TxBody *XDRTxBody `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing txBody"`
}

// XDRTransform2D stores DrawingML xfrm metadata used by grouped worksheet drawings.
type XDRTransform2D struct {
	Rot   int64      `xml:"rot,attr,omitempty"`
	FlipH bool       `xml:"flipH,attr,omitempty"`
	FlipV bool       `xml:"flipV,attr,omitempty"`
	Off   *XDROffset `xml:"off"`
	Ext   *XDRExtent `xml:"ext"`
	ChOff *XDROffset `xml:"chOff"`
	ChExt *XDRExtent `xml:"chExt"`
}

// XDROffset is a:off within a DrawingML transform.
type XDROffset struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// XDRGroupShape is xdr:grpSp, a grouped worksheet drawing container.
type XDRGroupShape struct {
	NvGrpSpPr   *XDRNvGrpSpPr     `xml:"nvGrpSpPr"`
	GrpSpPr     *XDRGrpSpPr       `xml:"grpSpPr"`
	Shapes      []XDRShape        `xml:"-"`
	Pictures    []XDRPic          `xml:"-"`
	GraphicFrms []XDRGraphicFrame `xml:"-"`
	GroupShapes []XDRGroupShape   `xml:"-"`
	Items       []XDRGroupItem    `xml:"-"`
}

// XDRNvGrpSpPr contains non-visual group metadata.
type XDRNvGrpSpPr struct {
	CNvPr XDRCNvPr `xml:"cNvPr"`
}

// XDRGrpSpPr contains the group transform.
type XDRGrpSpPr struct {
	Xfrm *XDRTransform2D `xml:"xfrm"`
}

// XDRGroupItemKind classifies grouped child elements.
type XDRGroupItemKind string

const (
	xdrGroupItemShape        XDRGroupItemKind = "sp"
	xdrGroupItemPicture      XDRGroupItemKind = "pic"
	xdrGroupItemGraphicFrame XDRGroupItemKind = "graphicFrame"
	xdrGroupItemGroup        XDRGroupItemKind = "grpSp"
)

// XDRGroupItem preserves grouped child order.
type XDRGroupItem struct {
	Kind  XDRGroupItemKind
	Index int
}

// XDRNvSpPr is xdr:nvSpPr.
type XDRNvSpPr struct {
	CNvPr XDRCNvPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvPr"`
}

// XDRSpBlipFill is a:blipFill inside xdr:spPr — a picture fill on a shape.
type XDRSpBlipFill struct {
	Blip    *XDRBlip `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip"`
	SrcRect *struct {
		L int `xml:"l,attr,omitempty"`
		T int `xml:"t,attr,omitempty"`
		R int `xml:"r,attr,omitempty"`
		B int `xml:"b,attr,omitempty"`
	} `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srcRect"`
	Tile    *drawingxml.Tile    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tile"`
	Stretch *drawingxml.Stretch `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stretch"`
}

// XDRSpPr is xdr:spPr — shape visual properties (geometry, fill, line).
type XDRSpPr struct {
	Xfrm      *XDRTransform2D            `xml:"xfrm"`
	Scene3D   *XDRScene3D                `xml:"scene3d"`
	Shape3D   *XDRShape3D                `xml:"sp3d"`
	PrstGeom  *drawingxml.PresetGeometry `xml:"prstGeom"`
	SolidFill *drawingxml.SolidFill      `xml:"solidFill"`
	BlipFill  *XDRSpBlipFill             `xml:"blipFill"`
	Ln        *drawingxml.Line           `xml:"ln"`
	EffectLst *XDREffectList             `xml:"effectLst"`
}

// UnmarshalXML preserves grouped child order.
func (g *XDRGroupShape) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*g = XDRGroupShape{}
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "nvGrpSpPr":
				var nv XDRNvGrpSpPr
				if err := d.DecodeElement(&nv, &se); err != nil {
					return err
				}
				g.NvGrpSpPr = &nv
			case "grpSpPr":
				var props XDRGrpSpPr
				if err := d.DecodeElement(&props, &se); err != nil {
					return err
				}
				g.GrpSpPr = &props
			case "sp":
				idx := len(g.Shapes)
				var shape XDRShape
				if err := d.DecodeElement(&shape, &se); err != nil {
					return err
				}
				g.Shapes = append(g.Shapes, shape)
				g.Items = append(g.Items, XDRGroupItem{Kind: xdrGroupItemShape, Index: idx})
			case "pic":
				idx := len(g.Pictures)
				var pic XDRPic
				if err := d.DecodeElement(&pic, &se); err != nil {
					return err
				}
				g.Pictures = append(g.Pictures, pic)
				g.Items = append(g.Items, XDRGroupItem{Kind: xdrGroupItemPicture, Index: idx})
			case "graphicFrame":
				idx := len(g.GraphicFrms)
				var frame XDRGraphicFrame
				if err := d.DecodeElement(&frame, &se); err != nil {
					return err
				}
				g.GraphicFrms = append(g.GraphicFrms, frame)
				g.Items = append(g.Items, XDRGroupItem{Kind: xdrGroupItemGraphicFrame, Index: idx})
			case "grpSp":
				idx := len(g.GroupShapes)
				var grp XDRGroupShape
				if err := d.DecodeElement(&grp, &se); err != nil {
					return err
				}
				g.GroupShapes = append(g.GroupShapes, grp)
				g.Items = append(g.Items, XDRGroupItem{Kind: xdrGroupItemGroup, Index: idx})
			default:
				var discard any
				if err := d.DecodeElement(&discard, &se); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}

// MarshalXML emits grouped child elements in stored order.
func (g XDRGroupShape) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Space: nsXDR, Local: "grpSp"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if g.NvGrpSpPr != nil {
		if err := e.EncodeElement(g.NvGrpSpPr, xml.StartElement{Name: xml.Name{Space: nsXDR, Local: "nvGrpSpPr"}}); err != nil {
			return err
		}
	}
	if g.GrpSpPr != nil {
		if err := e.EncodeElement(g.GrpSpPr, xml.StartElement{Name: xml.Name{Space: nsXDR, Local: "grpSpPr"}}); err != nil {
			return err
		}
	}
	items := g.Items
	if len(items) == 0 {
		for i := range g.Shapes {
			items = append(items, XDRGroupItem{Kind: xdrGroupItemShape, Index: i})
		}
		for i := range g.Pictures {
			items = append(items, XDRGroupItem{Kind: xdrGroupItemPicture, Index: i})
		}
		for i := range g.GraphicFrms {
			items = append(items, XDRGroupItem{Kind: xdrGroupItemGraphicFrame, Index: i})
		}
		for i := range g.GroupShapes {
			items = append(items, XDRGroupItem{Kind: xdrGroupItemGroup, Index: i})
		}
	}
	for _, item := range items {
		switch item.Kind {
		case xdrGroupItemShape:
			if item.Index >= 0 && item.Index < len(g.Shapes) {
				if err := e.EncodeElement(g.Shapes[item.Index], xml.StartElement{Name: xml.Name{Space: nsXDR, Local: "sp"}}); err != nil {
					return err
				}
			}
		case xdrGroupItemPicture:
			if item.Index >= 0 && item.Index < len(g.Pictures) {
				if err := e.EncodeElement(g.Pictures[item.Index], xml.StartElement{Name: xml.Name{Space: nsXDR, Local: "pic"}}); err != nil {
					return err
				}
			}
		case xdrGroupItemGraphicFrame:
			if item.Index >= 0 && item.Index < len(g.GraphicFrms) {
				if err := e.EncodeElement(g.GraphicFrms[item.Index], xml.StartElement{Name: xml.Name{Space: nsXDR, Local: "graphicFrame"}}); err != nil {
					return err
				}
			}
		case xdrGroupItemGroup:
			if item.Index >= 0 && item.Index < len(g.GroupShapes) {
				if err := e.EncodeElement(g.GroupShapes[item.Index], xml.StartElement{Name: xml.Name{Space: nsXDR, Local: "grpSp"}}); err != nil {
					return err
				}
			}
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// XDREffectList stores the supported DrawingML shape effects for worksheet shapes.
type XDREffectList struct {
	OuterShadow *XDROuterShadowEffect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main outerShdw"`
	InnerShadow *XDRInnerShadowEffect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main innerShdw"`
	Glow        *XDRGlowEffect        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main glow"`
	SoftEdge    *XDRSoftEdgeEffect    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main softEdge"`
	Reflection  *XDRReflectionEffect  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main reflection"`
	Blur        *XDRBlurEffect        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blur"`
}

// XDREffectAlpha stores an alpha percentage in thousandths of a percent.
type XDREffectAlpha struct {
	Val int `xml:"val,attr,omitempty"`
}

// XDREffectColorRef holds an effect color with optional alpha.
type XDREffectColorRef struct {
	Val   string          `xml:"val,attr"`
	Alpha *XDREffectAlpha `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha"`
}

// XDREffectSchemeColor references a theme color with optional alpha.
type XDREffectSchemeColor struct {
	Val   string          `xml:"val,attr"`
	Alpha *XDREffectAlpha `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha"`
}

// XDRInnerShadowEffect stores DrawingML inner shadow attributes (a:innerShdw).
type XDRInnerShadowEffect struct {
	BlurRad   int64                 `xml:"blurRad,attr,omitempty"`
	Dist      int64                 `xml:"dist,attr,omitempty"`
	Dir       int                   `xml:"dir,attr,omitempty"`
	SrgbClr   *XDREffectColorRef    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr"`
	SchemeClr *XDREffectSchemeColor `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr"`
}

// XDRBlurEffect stores DrawingML blur radius (a:blur).
type XDRBlurEffect struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

// XDROuterShadowEffect stores DrawingML outer-shadow attributes.
type XDROuterShadowEffect struct {
	BlurRad   int64                 `xml:"blurRad,attr,omitempty"`
	Dist      int64                 `xml:"dist,attr,omitempty"`
	Dir       int                   `xml:"dir,attr,omitempty"`
	SrgbClr   *XDREffectColorRef    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr"`
	SchemeClr *XDREffectSchemeColor `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr"`
	PrstClr   *XDREffectColorRef    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr"`
}

// XDRGlowEffect stores DrawingML glow attributes.
type XDRGlowEffect struct {
	Rad       int64                 `xml:"rad,attr,omitempty"`
	SrgbClr   *XDREffectColorRef    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr"`
	SchemeClr *XDREffectSchemeColor `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr"`
}

// XDRSoftEdgeEffect stores DrawingML soft-edge radius.
type XDRSoftEdgeEffect struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

// XDRReflectionEffect stores a subset of DrawingML reflection attributes.
type XDRReflectionEffect struct {
	BlurRad int64 `xml:"blurRad,attr,omitempty"`
	Dist    int64 `xml:"dist,attr,omitempty"`
	Dir     int   `xml:"dir,attr,omitempty"`
	StA     int   `xml:"stA,attr,omitempty"`
}

// DrawingEffectList converts worksheet DrawingML effects to shared DrawingML effects.
func (sp *XDRSpPr) DrawingEffectList(themeColors map[string]string) drawing.EffectList {
	if sp == nil || sp.EffectLst == nil {
		if threeD := sp.DrawingThreeDEffect(themeColors); threeD != nil {
			return drawing.EffectList{Effects: []drawing.Effect{*threeD}}
		}
		return drawing.EffectList{}
	}
	var effects []drawing.Effect
	if threeD := sp.DrawingThreeDEffect(themeColors); threeD != nil {
		effects = append(effects, *threeD)
	}
	if sp.EffectLst.OuterShadow != nil {
		color, alpha := resolveXDREffectColor(sp.EffectLst.OuterShadow.SrgbClr, sp.EffectLst.OuterShadow.SchemeClr, themeColors)
		if preset := sp.EffectLst.OuterShadow.PrstClr; preset != nil {
			color, alpha = resolveXDRPresetEffectColor(preset)
		}
		effects = append(effects, drawing.Shadow{
			Color:    color,
			Alpha:    alpha,
			Blur:     drawing.EMUToPoints(sp.EffectLst.OuterShadow.BlurRad),
			Distance: drawing.EMUToPoints(sp.EffectLst.OuterShadow.Dist),
			Angle:    float64(sp.EffectLst.OuterShadow.Dir) / 60000.0,
			ScaleX:   1.0,
			ScaleY:   1.0,
		})
	}
	if sp.EffectLst.InnerShadow != nil {
		color, alpha := resolveXDREffectColor(sp.EffectLst.InnerShadow.SrgbClr, sp.EffectLst.InnerShadow.SchemeClr, themeColors)
		effects = append(effects, drawing.InnerShadow{
			Color:    color,
			Alpha:    alpha,
			Blur:     drawing.EMUToPoints(sp.EffectLst.InnerShadow.BlurRad),
			Distance: drawing.EMUToPoints(sp.EffectLst.InnerShadow.Dist),
			Angle:    float64(sp.EffectLst.InnerShadow.Dir) / 60000.0,
		})
	}
	if sp.EffectLst.Glow != nil {
		color, alpha := resolveXDREffectColor(sp.EffectLst.Glow.SrgbClr, sp.EffectLst.Glow.SchemeClr, themeColors)
		effects = append(effects, drawing.Glow{
			Color:  color,
			Alpha:  alpha,
			Radius: drawing.EMUToPoints(sp.EffectLst.Glow.Rad),
		})
	}
	if sp.EffectLst.SoftEdge != nil {
		effects = append(effects, drawing.SoftEdge{Radius: drawing.EMUToPoints(sp.EffectLst.SoftEdge.Rad)})
	}
	if sp.EffectLst.Reflection != nil {
		alpha := 0.3
		if sp.EffectLst.Reflection.StA > 0 {
			alpha = float64(sp.EffectLst.Reflection.StA) / 100000.0
		}
		effects = append(effects, drawing.Reflection{
			Alpha:     alpha,
			Blur:      drawing.EMUToPoints(sp.EffectLst.Reflection.BlurRad),
			Distance:  drawing.EMUToPoints(sp.EffectLst.Reflection.Dist),
			Direction: float64(sp.EffectLst.Reflection.Dir) / 60000.0,
			Size:      1.0,
			Fade:      true,
		})
	}
	if sp.EffectLst.Blur != nil {
		effects = append(effects, drawing.Blur{
			Radius: drawing.EMUToPoints(sp.EffectLst.Blur.Rad),
		})
	}
	return drawing.EffectList{Effects: effects}
}

func resolveXDRPresetEffectColor(color *XDREffectColorRef) (string, float64) {
	if color == nil {
		return "000000", 1.0
	}
	switch strings.ToLower(strings.TrimSpace(color.Val)) {
	case "black":
		return "000000", xdrEffectAlphaValue(color.Alpha)
	case "white":
		return "FFFFFF", xdrEffectAlphaValue(color.Alpha)
	default:
		return "000000", xdrEffectAlphaValue(color.Alpha)
	}
}

// DrawingThreeDEffect converts scene3d/sp3d metadata into a shared 3D approximation.
func (sp *XDRSpPr) DrawingThreeDEffect(themeColors map[string]string) *drawing.ThreeD {
	if sp == nil || (sp.Scene3D == nil && sp.Shape3D == nil) {
		return nil
	}
	effect := &drawing.ThreeD{}
	if sp.Scene3D != nil {
		if sp.Scene3D.Camera != nil {
			effect.CameraPreset = sp.Scene3D.Camera.Prst
			if sp.Scene3D.Camera.Rot != nil {
				effect.RotationX = float64(sp.Scene3D.Camera.Rot.Lat) / 60000.0
				effect.RotationY = float64(sp.Scene3D.Camera.Rot.Lon) / 60000.0
				effect.RotationZ = float64(sp.Scene3D.Camera.Rot.Rev) / 60000.0
			}
		}
		if sp.Scene3D.LightRig != nil {
			effect.LightRig = sp.Scene3D.LightRig.Rig
			effect.LightDirection = sp.Scene3D.LightRig.Dir
		}
	}
	if sp.Shape3D != nil {
		effect.Material = sp.Shape3D.PrstMaterial
		effect.ExtrusionDepth = drawing.EMUToPoints(maxXDRInt64(sp.Shape3D.ExtrusionH, sp.Shape3D.Z))
		effect.ContourWidth = drawing.EMUToPoints(sp.Shape3D.ContourW)
		if sp.Shape3D.BevelT != nil {
			effect.BevelTopWidth = drawing.EMUToPoints(sp.Shape3D.BevelT.W)
			effect.BevelTopHeight = drawing.EMUToPoints(sp.Shape3D.BevelT.H)
			effect.BevelTopPreset = sp.Shape3D.BevelT.Prst
		}
		if sp.Shape3D.BevelB != nil {
			effect.BevelBottomWidth = drawing.EMUToPoints(sp.Shape3D.BevelB.W)
			effect.BevelBottomHeight = drawing.EMUToPoints(sp.Shape3D.BevelB.H)
			effect.BevelBottomPreset = sp.Shape3D.BevelB.Prst
		}
		effect.ContourColor = resolveXDR3DColor(sp.Shape3D.ContourClr, sp.Shape3D.ContourScheme, themeColors)
		effect.ExtrusionColor = resolveXDR3DColor(sp.Shape3D.ExtrusionClr, sp.Shape3D.ExtrudeScheme, themeColors)
	}
	if effect.CameraPreset == "" && effect.LightRig == "" && effect.LightDirection == "" && effect.Material == "" && effect.RotationX == 0 && effect.RotationY == 0 && effect.RotationZ == 0 && effect.ExtrusionDepth == 0 && effect.ContourWidth == 0 && effect.BevelTopHeight == 0 && effect.BevelBottomHeight == 0 {
		return nil
	}
	return effect
}

func resolveXDREffectColor(srgb *XDREffectColorRef, scheme *XDREffectSchemeColor, themeColors map[string]string) (string, float64) {
	if srgb != nil {
		return srgb.Val, xdrEffectAlphaValue(srgb.Alpha)
	}
	if scheme != nil {
		color := "000000"
		if themeColors != nil {
			if resolved := themeColors[scheme.Val]; resolved != "" {
				color = resolved
			}
		}
		return color, xdrEffectAlphaValue(scheme.Alpha)
	}
	return "000000", 1.0
}

func xdrEffectAlphaValue(alpha *XDREffectAlpha) float64 {
	if alpha == nil {
		return 1.0
	}
	value := float64(alpha.Val) / 100000.0
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func resolveXDR3DColor(srgb *XDRColorRef, scheme *drawingxml.SchemeClr, themeColors map[string]string) string {
	if srgb != nil && srgb.Val != "" {
		return srgb.Val
	}
	if scheme != nil && themeColors != nil {
		if color := themeColors[scheme.Val]; color != "" {
			return color
		}
	}
	return ""
}

func maxXDRInt64(values ...int64) int64 {
	var max int64
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

// XDRTxBody is xdr:txBody — text body inside a shape.
type XDRTxBody struct {
	Paragraphs []XDRTxParagraph `xml:"http://schemas.openxmlformats.org/drawingml/2006/main p"`
}

// XDRTxParagraph is a:p — a text paragraph.
type XDRTxParagraph struct {
	Runs []XDRTxRun `xml:"http://schemas.openxmlformats.org/drawingml/2006/main r"`
}

// XDRTxRun is a:r — a text run.
type XDRTxRun struct {
	Properties *XDRRunProperties `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rPr"`
	T          string            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main t"`
}

// XDRRunProperties is a:rPr. DrawingML stores size and emphasis as attributes;
// the embedded RTProps keeps the SpreadsheetML-style children older writers emitted.
type XDRRunProperties struct {
	RTProps
	SizeHundredths int                   `xml:"sz,attr,omitempty"`
	B              string                `xml:"b,attr,omitempty"`
	I              string                `xml:"i,attr,omitempty"`
	U              string                `xml:"u,attr,omitempty"`
	StrikeType     string                `xml:"strike,attr,omitempty"`
	Baseline       int                   `xml:"baseline,attr,omitempty"`
	SolidFill      *drawingxml.SolidFill `xml:"solidFill"`
	Latin          *XDRTextFont          `xml:"latin"`
}

// XDRTextFont is a:latin and its siblings.
type XDRTextFont struct {
	Typeface string `xml:"typeface,attr"`
}

// Table represents a table definition (xl/tables/tableN.xml).
// ECMA-376 Part 1 §18.5.
type Table struct {
	XMLName        xml.Name           `xml:"table"`
	ID             int                `xml:"id,attr,omitempty"`
	Name           string             `xml:"name,attr,omitempty"`
	DisplayName    string             `xml:"displayName,attr,omitempty"`
	Ref            string             `xml:"ref,attr,omitempty"`
	TotalsRowShown bool               `xml:"totalsRowShown,attr,omitempty"`
	HeaderRowCount int                `xml:"headerRowCount,attr,omitempty"`
	TotalsRowCount int                `xml:"totalsRowCount,attr,omitempty"`
	AutoFilter     *TableAutoFilter   `xml:"autoFilter"`
	TableStyleInfo *TableStyleInfoXML `xml:"tableStyleInfo"`
	Columns        []TableColumn      `xml:"tableColumns>tableColumn"`
}

// TableAutoFilter mirrors the autoFilter child inside a table definition.
type TableAutoFilter struct {
	Ref string `xml:"ref,attr,omitempty"`
}

// TableStyleInfoXML mirrors the tableStyleInfo child inside a table definition.
type TableStyleInfoXML struct {
	Name              string `xml:"name,attr,omitempty"`
	ShowFirstColumn   bool   `xml:"showFirstColumn,attr,omitempty"`
	ShowLastColumn    bool   `xml:"showLastColumn,attr,omitempty"`
	ShowRowStripes    bool   `xml:"showRowStripes,attr,omitempty"`
	ShowColumnStripes bool   `xml:"showColumnStripes,attr,omitempty"`
}

// TableColumn describes a single column inside a Table.
type TableColumn struct {
	XMLName                 xml.Name `xml:"tableColumn"`
	ID                      int      `xml:"id,attr,omitempty"`
	Name                    string   `xml:"name,attr,omitempty"`
	TotalsRowFunction       string   `xml:"totalsRowFunction,attr,omitempty"`
	TotalsRowLabel          string   `xml:"totalsRowLabel,attr,omitempty"`
	CalculatedColumnFormula string   `xml:"calculatedColumnFormula"`
	TotalsRowFormula        string   `xml:"totalsRowFormula"`
}

// PivotTableWithCache pairs a pivot table definition with its cache definition.
type PivotTableWithCache struct {
	Table   *PivotTableDefinition
	Cache   *PivotCacheDefinition // may be nil when the cache part is absent
	Records *PivotCacheRecords    // may be nil when the records part is absent
}

// PivotTableDefinition represents a xl/pivotTables/pivotTableN.xml part.
// ECMA-376 Part 1 §18.10.
type PivotTableDefinition struct {
	XMLName        xml.Name          `xml:"pivotTableDefinition"`
	Name           string            `xml:"name,attr"`
	CacheID        int               `xml:"cacheId,attr,omitempty"`
	DataCaption    string            `xml:"dataCaption,attr,omitempty"`
	RowGrandTotals *bool             `xml:"rowGrandTotals,attr,omitempty"`
	ColGrandTotals *bool             `xml:"colGrandTotals,attr,omitempty"`
	ShowHeaders    *bool             `xml:"showHeaders,attr,omitempty"`
	Location       PivotLocation     `xml:"location"`
	PivotFields    []PivotField      `xml:"pivotFields>pivotField"`
	RowFields      []PivotFieldIndex `xml:"rowFields>field"`
	ColFields      []PivotFieldIndex `xml:"colFields>field"`
	PageFields     []PageField       `xml:"pageFields>pageField"`
	DataFields     []DataField       `xml:"dataFields>dataField"`
	Raw            []byte            `xml:"-"`
}

// PivotLocation describes where the pivot table is placed on the worksheet.
type PivotLocation struct {
	Ref            string `xml:"ref,attr"`
	FirstHeaderRow int    `xml:"firstHeaderRow,attr,omitempty"`
	FirstDataRow   int    `xml:"firstDataRow,attr,omitempty"`
	FirstDataCol   int    `xml:"firstDataCol,attr,omitempty"`
}

// PivotField is metadata for one field in the pivot table.
type PivotField struct {
	Name        string      `xml:"name,attr,omitempty"`
	Axis        string      `xml:"axis,attr,omitempty"` // "axisRow", "axisCol", "axisPage"
	DataField   bool        `xml:"dataField,attr,omitempty"`
	Compact     bool        `xml:"compact,attr,omitempty"`
	Outline     bool        `xml:"outline,attr,omitempty"`
	SubtotalTop bool        `xml:"subtotalTop,attr,omitempty"`
	ShowAll     bool        `xml:"showAll,attr,omitempty"`
	Items       *PivotItems `xml:"items"`
}

// PivotItems contains the ordered display items for one pivot field.
type PivotItems struct {
	Count int         `xml:"count,attr,omitempty"`
	Items []PivotItem `xml:"item"`
}

// PivotItem represents one OOXML pivot item. X is the associated cache-item index.
type PivotItem struct {
	X          int    `xml:"x,attr,omitempty"`
	Type       string `xml:"t,attr,omitempty"`
	Hidden     bool   `xml:"h,attr,omitempty"`
	HideDetail *bool  `xml:"sd,attr,omitempty"`
	Name       string `xml:"n,attr,omitempty"`
}

// PivotFieldIndex is a <field index="N"/> reference.
type PivotFieldIndex struct {
	Index int `xml:"x,attr,omitempty"`
}

// PageField references a pivot field used as a report filter.
type PageField struct {
	Field int    `xml:"fld,attr"`
	Item  *int   `xml:"item,attr,omitempty"`
	Name  string `xml:"name,attr,omitempty"`
}

// DataField describes one value field in the pivot table.
type DataField struct {
	Name     string `xml:"name,attr,omitempty"`
	Field    int    `xml:"fld,attr"`
	Subtotal string `xml:"subtotal,attr,omitempty"` // "sum", "count", "average", …
	NumFmtID int    `xml:"numFmtId,attr,omitempty"`
}

// PivotCacheDefinition represents a xl/pivotCache/pivotCacheDefinitionN.xml part.
// ECMA-376 Part 1 §18.10.7.
type PivotCacheDefinition struct {
	XMLName               xml.Name         `xml:"pivotCacheDefinition"`
	RefreshedBy           string           `xml:"refreshedBy,attr,omitempty"`
	RefreshedDate         string           `xml:"refreshedDate,attr,omitempty"`
	CreatedVersion        int              `xml:"createdVersion,attr,omitempty"`
	RefreshedVersion      int              `xml:"refreshedVersion,attr,omitempty"`
	MinRefreshableVersion int              `xml:"minRefreshableVersion,attr,omitempty"`
	RecordCount           int              `xml:"recordCount,attr,omitempty"`
	WorksheetSource       *WorksheetSource `xml:"cacheSource>worksheetSource"`
	CacheFields           *CacheFields     `xml:"cacheFields"`
	Raw                   []byte           `xml:"-"`
}

// CacheFields is the list of cache fields in a pivot cache definition.
type CacheFields struct {
	Count  int          `xml:"count,attr,omitempty"`
	Fields []CacheField `xml:"cacheField"`
}

// CacheField describes one field in the pivot cache.
type CacheField struct {
	Name        string       `xml:"name,attr"`
	NumFmtID    int          `xml:"numFmtId,attr,omitempty"`
	SharedItems *SharedItems `xml:"sharedItems"`
}

// SharedItems holds the ordered set of distinct cache values for a pivot field.
// Items can be strings, numbers, booleans, errors, dates, or missing values.
type SharedItems struct {
	Count int          `xml:"count,attr,omitempty"`
	Items []SharedItem `xml:"-"`
}

// SharedItem is one cached value in a cache field's shared-items list. Type is
// the OOXML child element name (s, n, b, e, d, or m).
type SharedItem struct {
	Type  string
	Value string
}

func (s *SharedItems) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "count" {
			fmt.Sscanf(attr.Value, "%d", &s.Count)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			item := SharedItem{Type: t.Name.Local}
			for _, attr := range t.Attr {
				if attr.Name.Local == "v" {
					item.Value = attr.Value
				}
			}
			if err := d.Skip(); err != nil {
				return err
			}
			s.Items = append(s.Items, item)
		case xml.EndElement:
			if t.Name == start.Name {
				if s.Count == 0 {
					s.Count = len(s.Items)
				}
				return nil
			}
		}
	}
}

func (s SharedItems) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name.Local = "sharedItems"
	count := s.Count
	if count == 0 {
		count = len(s.Items)
	}
	if count > 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "count"}, Value: fmt.Sprintf("%d", count)})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, item := range s.Items {
		name := item.Type
		if name == "" {
			name = "s"
		}
		st := xml.StartElement{Name: xml.Name{Local: name}}
		if name != "m" {
			st.Attr = []xml.Attr{{Name: xml.Name{Local: "v"}, Value: item.Value}}
		}
		if err := e.EncodeToken(st); err != nil {
			return err
		}
		if err := e.EncodeToken(st.End()); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// PivotCacheRecords represents xl/pivotCache/pivotCacheRecordsN.xml.
// For raw passthrough the original bytes are kept unchanged; otherwise it can be
// populated programmatically and marshaled.
type PivotCacheRecords struct {
	XMLName xml.Name        `xml:"pivotCacheRecords"`
	Count   int             `xml:"count,attr,omitempty"`
	Rows    []PivotCacheRow `xml:"r"`
	Raw     []byte          `xml:"-"`
}

// PivotCacheRow is one record in the pivot cache records part.
type PivotCacheRow struct {
	Cells []PivotCacheCell `xml:"c"`
}

// PivotCacheCell is one field value within a pivot cache record.
type PivotCacheCell struct {
	Type  string `xml:"t,attr,omitempty"` // "s" shared index, "n" number, "b" bool, "e" error
	Value string `xml:"v,attr,omitempty"`
}

// WorksheetSource identifies the worksheet range that feeds the pivot cache.
type WorksheetSource struct {
	Ref   string `xml:"ref,attr,omitempty"`
	Sheet string `xml:"sheet,attr,omitempty"`
	Name  string `xml:"name,attr,omitempty"` // defined name alternative to Ref+Sheet
}
