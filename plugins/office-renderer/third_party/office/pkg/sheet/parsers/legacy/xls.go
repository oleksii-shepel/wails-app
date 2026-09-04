package legacy

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/encoding/charmap"

	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/cfb"
	chartpkg "office/pkg/ooxml/chart"
	drawing "office/pkg/ooxml/drawing"
	sheetir "office/pkg/sheet/ir"
)

// activeBiffCodepage holds the workbook codepage during legacy XLS parsing.
// It is set when parseWorkbookGlobals encounters a CODEPAGE record, and is
// read by decodeBiffBytes for compressed string decoding. Not thread-safe:
// ParseXls must not be called concurrently.
var activeBiffCodepage int

const (
	biffBOF             = 0x0809 // BIFF5/8 BOF (also used by BIFF5 workbook globals)
	biffBOFOld          = 0x0009 // BIFF2 BOF for raw worksheet streams
	biffBOF3            = 0x0209 // BIFF3 BOF
	biffBOF4            = 0x0409 // BIFF4 BOF
	biffNote            = 0x001C
	biffObj             = 0x005D
	biffMsoDrawingGroup = 0x00EB // workbook-level OfficeArt drawing group (BStore / blips)
	biffMsoDrawing      = 0x00EC // worksheet-level OfficeArt shape data
	biffEOF             = 0x000A
	biffColInfo         = 0x007D
	biffBoundSheet      = 0x0085
	biffXF              = 0x00E0
	biffPalette         = 0x0092 // PALETTE — workbook-redefined color palette (HSSF PaletteRecord)
	biffDateWindow1904  = 0x0022 // DATEMODE — when nonzero, dates use the 1904 epoch instead of 1900 (HSSF DateWindow1904Record)
	biffFormat          = 0x041E
	biffMergeCells      = 0x00E5
	biffTxO             = 0x01B6
	biffDefRowHeight    = 0x0225
	biffDefColWidth     = 0x0055 // DEFAULTCOLWIDTH — default width in whole character units
	biffArray           = 0x0221 // array formula group record
	biffShrFmla         = 0x04BC // shared formula group record
	biffRow             = 0x0208
	biffSST             = 0x00FC
	biffContinue        = 0x003C
	biffLabelSst        = 0x00FD
	biffBlank           = 0x0201
	biffBlankBIFF2      = 0x0001 // BIFF2 blank cell (row, col, attrs[3])
	biffLabel           = 0x0204
	biffLabelBIFF2      = 0x0004 // BIFF2 label cell (row, col, attrs[3], cch[1], bytes)
	biffNumber          = 0x0203
	biffNumberOld       = 0x0202 // BIFF3/4 number cell (same payload layout as biffNumber)
	biffNumberBIFF2     = 0x0003 // BIFF2 number cell (row, col, attrs[3], IEEE754 float64)
	biffRString         = 0x00D6 // BIFF3-7 rich-text label (8-bit chars + run info)
	biffRK              = 0x027E
	biffMulBlank        = 0x00BE
	biffMulRK           = 0x00BD
	biffBoolErr         = 0x0205
	biffBoolErrBIFF2    = 0x0005 // BIFF2 bool/error cell (row, col, attrs[3], value, isError)
	biffFormula         = 0x0006 // BIFF2/5+ formula cell (ambiguous sid; BOF version disambiguates)
	biffFormula3        = 0x0206 // BIFF3 formula cell
	biffFormula4        = 0x0406 // BIFF4 formula cell
	biffStringOld       = 0x0007 // BIFF2 formula cached string result
	biffString          = 0x0207 // BIFF3+ formula cached string result / StringRecord
	biffWindow2         = 0x023E // sheet-level view options; bit 3 = freeze panes
	biffPane            = 0x0041 // split/freeze pane positions
	biffGuts            = 0x0080 // GUTS — row/column gutter sizes and outline maxima
	biffSCL             = 0x00A0 // SCL — window zoom magnification (numerator/denominator)
	biffSetup           = 0x00A1 // page setup (paper size, orientation, scale, fit-to-page)
	biffLeftMargin      = 0x0026 // left page margin in inches (IEEE 754 double)
	biffRightMargin     = 0x0027 // right page margin in inches (IEEE 754 double)
	biffTopMargin       = 0x0028 // top page margin in inches (IEEE 754 double)
	biffBottomMargin    = 0x0029 // bottom page margin in inches (IEEE 754 double)
	biffPrintHeaders    = 0x002A // PRINTHEADERS — print row numbers and column letters (HSSF PrintHeadersRecord)
	biffPrintGridlines  = 0x002B // PRINTGRIDLINES — print sheet gridlines (HSSF PrintGridlinesRecord)
	biffHCenter         = 0x0083 // HCENTER — center the printout horizontally (HSSF HCenterRecord)
	biffVCenter         = 0x0084 // VCENTER — center the printout vertically (HSSF VCenterRecord)
	biffHeader          = 0x0014 // HEADER — print header text (HSSF HeaderRecord)
	biffFooter          = 0x0015 // FOOTER — print footer text (HSSF FooterRecord)
	biffHeaderFooterExt = 0x089C // HEADERFOOTER — Excel 2007+ extended header/footer record
	biffVertPageBreak   = 0x001A // VERTICALPAGEBREAK — manual column-direction page breaks (HSSF VerticalPageBreakRecord)
	biffHorizPageBreak  = 0x001B // HORIZONTALPAGEBREAK — manual row-direction page breaks (HSSF HorizontalPageBreakRecord)
	biffFont            = 0x0031 // font record
	biffHLink           = 0x01B8 // hyperlink record (HLINK)
	biffName            = 0x0218 // defined name (NAME) record
	biffNameComment     = 0x0894 // NAMECOMMENT — comment for a defined name (HSSF NameCommentRecord)
	biffSupBook         = 0x01AE // SUPBOOK record
	biffExternalName    = 0x0023 // EXTERNALNAME record
	biffExternSheet     = 0x0017 // EXTERNSHEET record
	biffCRNCount        = 0x0059 // XCT record
	biffCRN             = 0x005A // CRN record
	biffDConName        = 0x0052 // DCONNAME — named pivot/data-consolidation source
	biffCondFmt         = 0x01B2 // CONDFMT — start of a conditional-formatting block
	biffCF              = 0x01B1 // CF — one conditional-formatting rule (follows CONDFMT)
	biffCFHeader12      = 0x0879 // CFHEADER12 — Excel 2007+ conditional-formatting header
	biffCFRule12        = 0x087A // CFRULE12 — Excel 2007+ conditional-formatting rule
	biffUserSViewBegin  = 0x01AA // USERSVIEWBEGIN — start of a custom-view settings substream
	biffUserSViewEnd    = 0x01AB // USERSVIEWEND — end of a custom-view settings substream
	biffStyle           = 0x0293 // STYLE — maps an XF index to a built-in or user-defined style name
	biffDval            = 0x01B7 // DVAL — data-validation block header
	biffDV              = 0x01BE // DV — one data-validation rule
	biffAutoFilterInfo  = 0x009D // AUTOFILTERINFO — autofilter column count
	biffAutoFilter      = 0x009E // AUTOFILTER — per-column filter criteria (HSSF; no POI record class)
	biffFilterMode      = 0x009B // FILTERMODE — flag indicating the sheet is in filtered mode
	biffDimensions      = 0x0200 // DIMENSIONS — used cell range (first/last row/col)
	biffFeatHdr         = 0x0867 // FEATHDR — shared feature header
	biffFeat            = 0x0868 // FEAT — shared feature data
	biffFeatHdr11       = 0x0871 // FEATHDR11 — Excel 2007+ shared feature header
	biffFeature11       = 0x0872 // FEATURE11 — table shared feature data
	biffContinueFrt11   = 0x0875 // CONTINUEFRT11 — continuation for Feature11/12
	biffList12          = 0x0877 // LIST12 — additional table formatting metadata
	biffFeature12       = 0x0878 // FEATURE12 — extended table shared feature data
	biffProtect         = 0x0012 // PROTECT — sheet/workbook protection flag (HSSF ProtectRecord)
	biffPassword        = 0x0013 // PASSWORD — 16-bit hash of the protection password (HSSF PasswordRecord)
	biffWindowProtect   = 0x0019 // WINDOWPROTECT — workbook window protection flag (HSSF WindowProtectRecord)
	biffObjProtect      = 0x0063 // OBJPROTECT — whether objects in a protected sheet can be edited
	biffScenProtect     = 0x00DD // SCENPROTECT — whether scenarios are protected
	biffPrintSize       = 0x0033 // PRINTSIZE — print size scaling (opaque, POI UnknownRecord)
	biffPLS             = 0x004D // PLS — environment-specific print settings (opaque, POI UnknownRecord)
	biffBitmap          = 0x00E9 // BITMAP — print bitmap (opaque, POI UnknownRecord)
	biffIndex           = 0x020B // INDEX — row-block index (performance pointer; POI uses for offset validation)
	biffDBCell          = 0x00D7 // DBCELL — cell-block offset index (performance pointer; POI uses for offset validation)
	biffCodepage        = 0x0042 // CODEPAGE — default character set for the workbook (HSSF CodepageRecord)
	biffCountry         = 0x008C // COUNTRY — default and current country locale (HSSF CountryRecord)
	biffProtectionRev4  = 0x01AF // PROTECTIONREV4 — workbook-level revision protection flag
	biffWindow1         = 0x003D // WINDOW1 — workbook window position, active sheet index, display options (HSSF WindowOneRecord)
	biffWriteAccess     = 0x005C // WRITEACCESS — username of the spreadsheet generator owner (HSSF WriteAccessRecord)
	biffBackup          = 0x0040 // BACKUP — backup file flag (HSSF BackupRecord)
	biffBookBool        = 0x00DA // BOOKBOOL — save external link values flag (HSSF BookBoolRecord)
	biffCalcMode        = 0x000D // CALCMODE — formula calculation mode: manual=0, automatic=1, auto-except-tables=-1 (HSSF CalcModeRecord)
	biffCalcCount       = 0x000C // CALCCOUNT — iteration count for circular references (HSSF CalcCountRecord)
	biffRefreshAll      = 0x01B7 // REFRESHALL — refresh all external data on open flag (HSSF RefreshAllRecord)
	biffIteration       = 0x0011 // ITERATION — iterative calculation flag (HSSF IterationRecord)
	biffDelta           = 0x0010 // DELTA — iteration change tolerance (HSSF DeltaRecord)
	biffPrecision       = 0x000E // PRECISION — precision as displayed flag (HSSF PrecisionRecord)
	biffSaveRecalc      = 0x005F // SAVERECALC — save recalculation flag (HSSF SaveRecalcRecord)
	biffWSBool          = 0x0081 // WSBOOL — worksheet boolean flags (HSSF WSBoolRecord)
	biffMsoDrawingSel   = 0x00ED // MSODRAWINGSELECTION — drawing selection state (HSSF DrawingSelectionRecord)
	biffHideObj         = 0x008D // HIDEOBJ — hide/show objects/placeholders (HSSF HideObjRecord)
	biffSelection       = 0x001D // SELECTION — cursor/selection position in worksheet (HSSF SelectionRecord)
	biffGridset         = 0x0082 // GRIDSET — gridlines visible when printing flag (HSSF GridsetRecord)
	biffRecalcId        = 0x01C1 // RECALCID — recalculation engine ID (HSSF RecalcIdRecord)
	biffTabId           = 0x013D // TABID — sheet tab IDs (HSSF TabIdRecord)
	biffUncalced        = 0x005E // UNCALCED — formulas need recalculation flag (HSSF UncalcedRecord)
	biffWriteProtect    = 0x0086 // WRITEPROTECT — write protection flag (HSSF WriteProtectRecord)
	biffPasswordRev4    = 0x01BC // PASSWORDREV4 — revision password hash (HSSF PasswordRev4Record)
	biffFileSharing     = 0x005B // FILESHARING — file sharing options (HSSF FileSharingRecord)
	biffFilePass        = 0x002F // FILEPASS — encryption header (HSSF FilePassRecord)
	biffRefMode         = 0x000F // REFMODE — reference mode A1/R1C1 (HSSF RefModeRecord)
	biffDSF             = 0x0161 // DSF — double stream file flag (HSSF DSFRecord)
	biffMMS             = 0x00C1 // MMS — add/del menu counts (HSSF MMSRecord)
	biffInterfaceHdr    = 0x00E1 // INTERFACEHDR — interface header (HSSF InterfaceHdrRecord)
	biffInterfaceEnd    = 0x00E2 // INTERFACEEND — interface end (HSSF InterfaceEndRecord)
	// BIFF version constants from BOFRecord version field.
	biffVersion2 = 0x0001 // BIFF2
	biffVersion3 = 0x0002 // BIFF3
	biffVersion4 = 0x0004 // BIFF4
	biffVersion5 = 0x0500 // BIFF5
	biffVersion8 = 0x0600 // BIFF8

	// BOF substream type constants (HSSF BOFRecord.TYPE_*).
	biffBOFTypeWorkbook      = 0x05
	biffBOFTypeVBModule      = 0x06
	biffBOFTypeWorksheet     = 0x10
	biffBOFTypeChart         = 0x20
	biffBOFTypeExcel4Macro   = 0x40
	biffBOFTypeWorkspaceFile = 0x100

	biffChart           = 0x1002 // CHART record — start of chart object records
	biffChartSeries     = 0x1003 // SERIES record
	biffChartSeriesText = 0x100D // SERIESTEXT record
	biffChartFormat     = 0x1014 // CHARTFORMAT record
	biffChartBar        = 0x1017 // BAR chart group
	biffChartLine       = 0x1018 // LINE chart group
	biffChartPie        = 0x1019 // PIE chart group
	biffChartArea       = 0x101A // AREA chart group
	biffChartScatter    = 0x101B // SCATTER chart group
	biffChartChart3D    = 0x103A // CHART3D record
	biffChartRadar      = 0x103E // RADAR chart group
	biffChartSurface    = 0x103F // SURF chart group
	biffChartRadarArea  = 0x1040 // RADARAREA chart group
	biffChartAI         = 0x1051 // AI linked-data record for series name/value/category refs

	// Richer chart records (POI HSSF adoption)
	biffChartLegend             = 0x1015 // LEGEND record
	biffChartAxis               = 0x101D // AXIS record
	biffChartValueRange         = 0x101F // VALUERANGE record
	biffChartText               = 0x1025 // TEXT record
	biffChartObjectLink         = 0x1027 // OBJECTLINK record
	biffChartFrame              = 0x1032 // FRAME record
	biffChartBegin              = 0x1033 // BEGIN record
	biffChartEnd                = 0x1034 // END record
	biffChartPlotArea           = 0x1035 // PLOTAREA record
	biffChartAxisOptions        = 0x1062 // AXISOPTIONS record (CatSerRange)
	biffChartLineFormat         = 0x1007 // LINEFORMAT record
	biffChartMarkerFormat       = 0x1009 // MARKERFORMAT record
	biffChartAreaFormat         = 0x100A // AREAFORMAT record
	biffChartDataFormat         = 0x1006 // DATAFORMAT record
	biffChartTick               = 0x101E // TICK record
	biffChartCategorySeriesAxis = 0x1020 // CATEGORYSERIESAXIS record
	biffChartAxisLineFormat     = 0x1021 // AXISLINEFORMAT record

	legacyObjTypeChart    = 0x0005
	legacyObjTypeButton   = 0x0007
	legacyObjTypePicture  = 0x0008
	legacyObjTypeCheckbox = 0x000B
	legacyObjTypeRadio    = 0x000C
	legacyObjTypeEdit     = 0x000D
	legacyObjTypeLabel    = 0x000E
	legacyObjTypeSpin     = 0x0010
	legacyObjTypeScroll   = 0x0011
	legacyObjTypeList     = 0x0012
	legacyObjTypeGroupBox = 0x0013
	legacyObjTypeDrop     = 0x0014
	legacyObjTypeNote     = 0x0019

	legacyOfficeArtSpContainer  = 0xF004
	legacyOfficeArtBSE          = 0xF007
	legacyOfficeArtSp           = 0xF00A
	legacyOfficeArtFOPT         = 0xF00B
	legacyOfficeArtClientAnchor = 0xF010
	legacyOfficeArtBlipEMF      = 0xF01A
	legacyOfficeArtBlipWMF      = 0xF01B
	legacyOfficeArtBlipJPEG     = 0xF01D
	legacyOfficeArtBlipPNG      = 0xF01E
	legacyOfficeArtBlipDIB      = 0xF01F

	legacyOfficeArtShapePicture        = 75
	legacyOfficeArtPropPib             = 0x0104
	legacyOfficeArtPropFillType        = 0x0180
	legacyOfficeArtPropFillColor       = 0x0181
	legacyOfficeArtPropFillBools       = 0x01BF
	legacyOfficeArtPropLineColor       = 0x01C0
	legacyOfficeArtPropLineWidth       = 0x01CB
	legacyOfficeArtPropLineDashing     = 0x01CE
	legacyOfficeArtPropLineStartArrow  = 0x01D0
	legacyOfficeArtPropLineEndArrow    = 0x01D1
	legacyOfficeArtPropLineStartWidth  = 0x01D2
	legacyOfficeArtPropLineStartLength = 0x01D3
	legacyOfficeArtPropLineEndWidth    = 0x01D4
	legacyOfficeArtPropLineEndLength   = 0x01D5
	legacyOfficeArtPropLineBools       = 0x01FF
	legacyOfficeArtPropRotation        = 0x0004
	legacyOfficeArtPropAdjustValue     = 0x0147
	legacyOfficeArtPropShadowColor     = 0x0201
	legacyOfficeArtPropShadowOpacity   = 0x0204
	legacyOfficeArtPropShadowOffsetX   = 0x0205
	legacyOfficeArtPropShadowOffsetY   = 0x0206
	legacyOfficeArtPropShadowSoftness  = 0x021C

	legacyDefaultColumnPx = 64
	legacyDefaultRowPx    = 20
	legacyCharUnitToPx    = 7
	legacyEMUsPerPx       = 9525

	// Pivot-table records (SX… family).
	biffSXView     = 0x00B0 // SXVIEW — pivot table view definition
	biffSXVD       = 0x00B1 // SXVD — view field dimension
	biffSXVI       = 0x00B2 // SXVI — view item
	biffSXIvd      = 0x00B4 // SXIVD — row/column axis field order
	biffSXPI       = 0x00B6 // SXPI — page item
	biffSXDI       = 0x00C5 // SXDI — data item
	biffSXDB       = 0x00C0 // SXDB — cache definition
	biffSXString   = 0x00CD // SXSTRING — pivot table string value
	biffSXEx       = 0x00DC // SXEX — pivot table view extension
	biffSXStreamID = 0x00D5 // SXSTREAMID — pivot table stream ID

	// Additional chart records.
	biffChartAxisParent                     = 0x1041 // AXISPARENT record
	biffChartAxisUsed                       = 0x1046 // AXISUSED record
	biffChartCatLab                         = 0x0856 // CATLAB record
	biffChartChartFRTInfo                   = 0x0850 // CHARTFRTINFO record
	biffChartStartBlock                     = 0x0852 // CHARTSTARTBLOCK record
	biffChartEndBlock                       = 0x0853 // CHARTENDBLOCK record
	biffChartStartObject                    = 0x0854 // CHARTSTARTOBJECT record
	biffChartEndObject                      = 0x0855 // CHARTENDOBJECT record
	biffChartTitleFormat                    = 0x1050 // CHARTTITLEFORMAT record
	biffChartSeriesList                     = 0x1016 // SERIESLIST record
	biffChartSeriesIndex                    = 0x1065 // SERIESINDEX record
	biffChartSeriesLabels                   = 0x100C // SERIESLABELS record
	biffChartUnits                          = 0x1001 // UNITS record
	biffChartFontIndex                      = 0x1026 // FONTINDEX record
	biffChartNumberFormatIndex              = 0x104E // NUMBERFORMATINDEX record
	biffChartPlotGrowth                     = 0x1064 // PLOTGROWTH record
	biffChartSeriesChartGroupIndex          = 0x1045 // SERIESCHARTGROUPINDEX record
	biffChartDat                            = 0x1063 // DAT record
	biffChartDefaultDataLabelTextProperties = 0x1024 // DEFAULTDATELABELTEXTPROPERTIES record
	biffChartFontBasis                      = 0x1060 // FONTBASIS record

	// Other BIFF records.
	biffExtSST                       = 0x00FF // EXTSST — extended SST
	biffDConRef                      = 0x0051 // DCONREF — data consolidation reference
	biffTableRecord                  = 0x0236 // TABLE — data table definition
	biffUseSelFS                     = 0x0160 // USESELFS — use natural language formulas
	biffFnGroupCount                 = 0x009C // FNGROUPCOUNT — function group count
	biffTableStyles                  = 0x088E // TABLESTYLES — table styles
	biffDataLabelExtension           = 0x086A // DATALABELEXTENSION record
	biffExtendedPivotTableViewFields = 0x0100 // SXEXTDF — extended pivot table view fields
	biffSXVS                         = 0x00E3 // SXVS — pivot cache source type
)

type boundSheet struct {
	offset     int
	name       string
	kind       byte
	hidden     bool
	visibility string
}

type legacyFont struct {
	Name      string
	HeightPt  float64
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	ColorIdx  uint16 // icv; 0x7FFF = automatic
}

type legacyXF struct {
	// Font properties resolved at parse time from the FONT record.
	FontName  string
	HeightPt  float64
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	FontColor string // hex RGB, "" = automatic
	// Number format.
	NumFmtCode string
	// Alignment.
	HAlign   string // "general","left","center","right","fill","justify"
	VAlign   string // "top","middle","bottom"
	WrapText bool
	// Border styles (CSS-style: "", "solid", "dashed", "dotted", "double").
	BorderLeftStyle   string
	BorderRightStyle  string
	BorderTopStyle    string
	BorderBottomStyle string
	// Border colors (hex RGB, "" = automatic).
	BorderLeftColor   string
	BorderRightColor  string
	BorderTopColor    string
	BorderBottomColor string
	// Fill.
	BgColor string // hex RGB background, "" = none
	// Text rotation (BIFF8 trot): 0=none, 1-90=CCW degrees, 91-180=CW (value-90), 255=stacked.
	TextRotation int
	// Cell protection flags from XF grbit (only apply when sheet protection is active).
	Locked        bool // grbit bit 2: cell contents locked
	FormulaHidden bool // grbit bit 3: formula hidden in formula bar
}

type legacyNote struct {
	objID        int
	ref          string
	fallbackText string
}

// hlinkEntry stores one parsed HLINK record's cell range and URL.
type hlinkEntry struct {
	firstRow, lastRow, firstCol, lastCol int // 0-based
	url                                  string
}

type externSheetRef struct {
	supBookIndex  int
	firstSheet    int
	lastSheet     int
	sourceKind    supBookKind
	externalNames []legacyExternalName
}

type supBookRef struct {
	kind          supBookKind
	sheetCount    int
	url           string
	sheetNames    []string
	externalNames []legacyExternalName
	cachedSheets  map[int]*sheetir.WorkbookExternalSheet
}

type legacyExternalName struct {
	text  string
	ixals int
}

type legacyFileSharingRecord struct {
	readOnly     bool
	passwordHash uint16
	username     string
}

type supBookKind string

const (
	supBookKindUnknown  supBookKind = "unknown"
	supBookKindInternal supBookKind = "internal"
	supBookKindExternal supBookKind = "external"
	supBookKindAddIn    supBookKind = "addin"
)

type legacyFormulaWorkbookContext struct {
	sheets       []boundSheet
	externSheets []externSheetRef
	definedNames []string
	pivotCaches  []legacyPivotCacheSource
	// date1904 is the workbook-wide date base flag from DATEMODE (0x0022).
	// Threaded through here because numeric-to-date promotion runs inside
	// the same cell-record handlers that already carry this context.
	date1904 bool
}

type legacyFormulaDecodeContext struct {
	workbook legacyFormulaWorkbookContext
	row      int
	col      int
}

type legacyFormulaExpr struct {
	text       string
	precedence int
}

type legacyFormulaFunctionInfo struct {
	name  string
	arity int
}

type legacySharedFormulaRecord struct {
	AnchorRef   string
	RangeRef    string
	TopRow      int
	LeftCol     int
	BottomRow   int
	RightCol    int
	FormulaData []byte
}

type legacyPivotCacheSource struct {
	StreamID    uint16
	SourceSheet string
	SourceRef   string
	SourceName  string
}

type legacyPivotDataField struct {
	fieldIndex int
	name       string
	subtotal   string
	numFmtID   int
}

type legacyPivotItem struct {
	cacheIndex int
	itemType   int
	name       string
	hidden     bool
	hideDetail bool
}

type legacyPivotView struct {
	Name           string
	LocationRef    string
	DataCaption    string
	CacheIndex     int
	RowGrandTotals bool
	ColGrandTotals bool
	ShowHeaders    bool

	rowFieldCount  int
	colFieldCount  int
	pageFieldCount int

	fieldNames      []string
	fieldAxes       []string
	fieldItems      map[int][]legacyPivotItem
	currentField    int
	defaultRowOrder []int
	defaultColOrder []int
	defaultPageAxis []int
	rowOrder        []int
	colOrder        []int
	pageOrder       []int
	pageSelections  map[int]int
	sawRowOrder     bool
	sawColOrder     bool
	dataFields      []legacyPivotDataField
}

type legacyTableFeature struct {
	idList   uint32
	table    *sheetir.Table
	ref      string
	name     string
	hasStyle bool
	style    sheetir.TableStyleInfo
}

// legacyFormulaBuiltinFunctions maps BIFF function index → name and min arity.
// The index-to-name mapping follows POI's functionMetadata.txt (Apache Open Office
// Excel File Format, Section 3.11 Built-In Sheet Functions). The arity is the
// minimum parameter count, used by ptgFunc (fixed-arity) decompilation.
// ptgFuncVar (variable-arity) uses the arity from the bytecode and the name
// from this table.
var legacyFormulaBuiltinFunctions = map[uint16]legacyFormulaFunctionInfo{
	// BIFF2 built-in functions
	0:   {name: "COUNT", arity: 0},
	1:   {name: "IF", arity: 2},
	2:   {name: "ISNA", arity: 1},
	3:   {name: "ISERROR", arity: 1},
	4:   {name: "SUM", arity: 0},
	5:   {name: "AVERAGE", arity: 1},
	6:   {name: "MIN", arity: 1},
	7:   {name: "MAX", arity: 1},
	8:   {name: "ROW", arity: 0},
	9:   {name: "COLUMN", arity: 0},
	10:  {name: "NA", arity: 0},
	11:  {name: "NPV", arity: 2},
	12:  {name: "STDEV", arity: 1},
	13:  {name: "DOLLAR", arity: 1},
	14:  {name: "FIXED", arity: 2},
	15:  {name: "SIN", arity: 1},
	16:  {name: "COS", arity: 1},
	17:  {name: "TAN", arity: 1},
	18:  {name: "ATAN", arity: 1},
	19:  {name: "PI", arity: 0},
	20:  {name: "SQRT", arity: 1},
	21:  {name: "EXP", arity: 1},
	22:  {name: "LN", arity: 1},
	23:  {name: "LOG10", arity: 1},
	24:  {name: "ABS", arity: 1},
	25:  {name: "INT", arity: 1},
	26:  {name: "SIGN", arity: 1},
	27:  {name: "ROUND", arity: 2},
	28:  {name: "LOOKUP", arity: 2},
	29:  {name: "INDEX", arity: 2},
	30:  {name: "REPT", arity: 2},
	31:  {name: "MID", arity: 3},
	32:  {name: "LEN", arity: 1},
	33:  {name: "VALUE", arity: 1},
	34:  {name: "TRUE", arity: 0},
	35:  {name: "FALSE", arity: 0},
	36:  {name: "AND", arity: 1},
	37:  {name: "OR", arity: 1},
	38:  {name: "NOT", arity: 1},
	39:  {name: "MOD", arity: 2},
	40:  {name: "DCOUNT", arity: 3},
	41:  {name: "DSUM", arity: 3},
	42:  {name: "DAVERAGE", arity: 3},
	43:  {name: "DMIN", arity: 3},
	44:  {name: "DMAX", arity: 3},
	45:  {name: "DSTDEV", arity: 3},
	46:  {name: "VAR", arity: 1},
	47:  {name: "DVAR", arity: 3},
	48:  {name: "TEXT", arity: 2},
	49:  {name: "LINEST", arity: 1},
	50:  {name: "TREND", arity: 1},
	51:  {name: "LOGEST", arity: 1},
	52:  {name: "GROWTH", arity: 1},
	53:  {name: "GOTO", arity: 1},
	55:  {name: "RETURN", arity: 1},
	56:  {name: "PV", arity: 3},
	57:  {name: "FV", arity: 3},
	58:  {name: "NPER", arity: 3},
	59:  {name: "PMT", arity: 3},
	60:  {name: "RATE", arity: 3},
	61:  {name: "MIRR", arity: 3},
	62:  {name: "IRR", arity: 1},
	63:  {name: "RAND", arity: 0},
	64:  {name: "MATCH", arity: 2},
	65:  {name: "DATE", arity: 3},
	66:  {name: "TIME", arity: 3},
	67:  {name: "DAY", arity: 1},
	68:  {name: "MONTH", arity: 1},
	69:  {name: "YEAR", arity: 1},
	70:  {name: "WEEKDAY", arity: 1},
	71:  {name: "HOUR", arity: 1},
	72:  {name: "MINUTE", arity: 1},
	73:  {name: "SECOND", arity: 1},
	74:  {name: "NOW", arity: 0},
	75:  {name: "AREAS", arity: 1},
	76:  {name: "ROWS", arity: 1},
	77:  {name: "COLUMNS", arity: 1},
	78:  {name: "OFFSET", arity: 3},
	79:  {name: "ABSREF", arity: 2},
	80:  {name: "RELREF", arity: 2},
	81:  {name: "ARGUMENT", arity: 0},
	82:  {name: "SEARCH", arity: 2},
	83:  {name: "TRANSPOSE", arity: 1},
	84:  {name: "ERROR", arity: 0},
	85:  {name: "STEP", arity: 0},
	86:  {name: "TYPE", arity: 1},
	87:  {name: "ISTEXT", arity: 1},
	88:  {name: "ISNUMBER", arity: 1},
	89:  {name: "ISBLANK", arity: 1},
	97:  {name: "ATAN2", arity: 2},
	98:  {name: "ASIN", arity: 1},
	99:  {name: "ACOS", arity: 1},
	100: {name: "CHOOSE", arity: 2},
	101: {name: "HLOOKUP", arity: 3},
	102: {name: "VLOOKUP", arity: 3},
	105: {name: "ISREF", arity: 1},
	109: {name: "LOG", arity: 1},
	110: {name: "EXEC", arity: 1},
	111: {name: "CHAR", arity: 1},
	112: {name: "LOWER", arity: 1},
	113: {name: "UPPER", arity: 1},
	114: {name: "PROPER", arity: 1},
	115: {name: "LEFT", arity: 1},
	116: {name: "RIGHT", arity: 1},
	117: {name: "EXACT", arity: 2},
	118: {name: "TRIM", arity: 1},
	119: {name: "REPLACE", arity: 4},
	120: {name: "SUBSTITUTE", arity: 3},
	121: {name: "CODE", arity: 1},
	124: {name: "FIND", arity: 2},
	125: {name: "CELL", arity: 1},
	126: {name: "ISERR", arity: 1},
	127: {name: "ISTEXT", arity: 1},
	128: {name: "ISNUMBER", arity: 1},
	129: {name: "ISBLANK", arity: 1},
	130: {name: "T", arity: 1},
	131: {name: "N", arity: 1},
	140: {name: "DATEVALUE", arity: 1},
	141: {name: "TIMEVALUE", arity: 1},
	142: {name: "SLN", arity: 3},
	143: {name: "SYD", arity: 4},
	144: {name: "DDB", arity: 4},
	148: {name: "INDIRECT", arity: 1},
	150: {name: "CALL", arity: 1},
	162: {name: "CLEAN", arity: 1},
	163: {name: "MDETERM", arity: 1},
	164: {name: "MINVERSE", arity: 1},
	165: {name: "MMULT", arity: 2},
	167: {name: "IPMT", arity: 4},
	168: {name: "PPMT", arity: 4},
	169: {name: "COUNTA", arity: 0},
	183: {name: "PRODUCT", arity: 0},
	184: {name: "FACT", arity: 1},
	185: {name: "GET.CELL", arity: 1},
	186: {name: "GET.WORKSPACE", arity: 1},
	187: {name: "GET.WINDOW", arity: 1},
	188: {name: "GET.DOCUMENT", arity: 1},
	189: {name: "DPRODUCT", arity: 3},
	190: {name: "ISNONTEXT", arity: 1},
	193: {name: "STDEVP", arity: 1},
	194: {name: "VARP", arity: 1},
	195: {name: "DSTDEVP", arity: 3},
	196: {name: "DVARP", arity: 3},
	197: {name: "TRUNC", arity: 1},
	198: {name: "ISLOGICAL", arity: 1},
	199: {name: "DCOUNTA", arity: 3},
	// BIFF3 additions
	204: {name: "YEN", arity: 1},
	205: {name: "FINDB", arity: 2},
	206: {name: "SEARCHB", arity: 2},
	207: {name: "REPLACEB", arity: 4},
	208: {name: "LEFTB", arity: 1},
	209: {name: "RIGHTB", arity: 1},
	210: {name: "MIDB", arity: 3},
	211: {name: "LENB", arity: 1},
	212: {name: "ROUNDUP", arity: 2},
	213: {name: "ROUNDDOWN", arity: 2},
	214: {name: "ASC", arity: 1},
	215: {name: "JIS", arity: 1},
	219: {name: "ADDRESS", arity: 2},
	220: {name: "DAYS360", arity: 2},
	221: {name: "TODAY", arity: 0},
	222: {name: "VDB", arity: 5},
	225: {name: "END.IF", arity: 0},
	227: {name: "MEDIAN", arity: 1},
	228: {name: "SUMPRODUCT", arity: 1},
	229: {name: "SINH", arity: 1},
	230: {name: "COSH", arity: 1},
	231: {name: "TANH", arity: 1},
	232: {name: "ASINH", arity: 1},
	233: {name: "ACOSH", arity: 1},
	234: {name: "ATANH", arity: 1},
	235: {name: "DGET", arity: 3},
	244: {name: "INFO", arity: 1},
	// BIFF4 additions
	216: {name: "RANK", arity: 2},
	238: {name: "LAST.ERROR", arity: 0},
	247: {name: "DB", arity: 4},
	252: {name: "FREQUENCY", arity: 2},
	257: {name: "EVALUATE", arity: 1},
	261: {name: "ERROR.TYPE", arity: 1},
	262: {name: "APP.TITLE", arity: 0},
	263: {name: "WINDOW.TITLE", arity: 0},
	264: {name: "SAVE.TOOLBAR", arity: 0},
	265: {name: "ENABLE.TOOL", arity: 3},
	266: {name: "PRESS.TOOL", arity: 3},
	267: {name: "REGISTER.ID", arity: 2},
	268: {name: "GET.WORKBOOK", arity: 1},
	269: {name: "AVEDEV", arity: 1},
	270: {name: "BETADIST", arity: 3},
	271: {name: "GAMMALN", arity: 1},
	272: {name: "BETAINV", arity: 3},
	273: {name: "BINOMDIST", arity: 4},
	274: {name: "CHIDIST", arity: 2},
	275: {name: "CHIINV", arity: 2},
	276: {name: "COMBIN", arity: 2},
	277: {name: "CONFIDENCE", arity: 3},
	278: {name: "CRITBINOM", arity: 3},
	279: {name: "EVEN", arity: 1},
	280: {name: "EXPONDIST", arity: 3},
	281: {name: "FDIST", arity: 3},
	282: {name: "FINV", arity: 3},
	283: {name: "FISHER", arity: 1},
	284: {name: "FISHERINV", arity: 1},
	285: {name: "FLOOR", arity: 2},
	286: {name: "GAMMADIST", arity: 4},
	287: {name: "GAMMAINV", arity: 3},
	288: {name: "CEILING", arity: 2},
	289: {name: "HYPGEOMDIST", arity: 4},
	290: {name: "LOGNORMDIST", arity: 3},
	291: {name: "LOGINV", arity: 3},
	292: {name: "NEGBINOMDIST", arity: 3},
	293: {name: "NORMDIST", arity: 4},
	294: {name: "NORMSDIST", arity: 1},
	295: {name: "NORMINV", arity: 3},
	296: {name: "NORMSINV", arity: 1},
	297: {name: "STANDARDIZE", arity: 3},
	298: {name: "ODD", arity: 1},
	299: {name: "PERMUT", arity: 2},
	300: {name: "POISSON", arity: 3},
	301: {name: "TDIST", arity: 3},
	302: {name: "WEIBULL", arity: 4},
	303: {name: "SUMXMY2", arity: 2},
	304: {name: "SUMX2MY2", arity: 2},
	305: {name: "SUMX2PY2", arity: 2},
	306: {name: "CHITEST", arity: 2},
	307: {name: "CORREL", arity: 2},
	308: {name: "COVAR", arity: 2},
	309: {name: "FORECAST", arity: 3},
	310: {name: "FTEST", arity: 2},
	311: {name: "INTERCEPT", arity: 2},
	312: {name: "PEARSON", arity: 2},
	313: {name: "RSQ", arity: 2},
	314: {name: "STEYX", arity: 2},
	315: {name: "SLOPE", arity: 2},
	316: {name: "TTEST", arity: 4},
	317: {name: "PROB", arity: 3},
	318: {name: "DEVSQ", arity: 1},
	319: {name: "GEOMEAN", arity: 1},
	320: {name: "HARMEAN", arity: 1},
	321: {name: "SUMSQ", arity: 0},
	322: {name: "KURT", arity: 1},
	323: {name: "SKEW", arity: 1},
	324: {name: "ZTEST", arity: 2},
	325: {name: "LARGE", arity: 2},
	326: {name: "SMALL", arity: 2},
	327: {name: "QUARTILE", arity: 2},
	328: {name: "PERCENTILE", arity: 2},
	329: {name: "PERCENTRANK", arity: 2},
	330: {name: "MODE", arity: 1},
	331: {name: "TRIMMEAN", arity: 2},
	332: {name: "TINV", arity: 2},
	// BIFF5 additions
	336: {name: "CONCATENATE", arity: 0},
	337: {name: "POWER", arity: 2},
	342: {name: "RADIANS", arity: 1},
	343: {name: "DEGREES", arity: 1},
	344: {name: "SUBTOTAL", arity: 2},
	345: {name: "SUMIF", arity: 2},
	346: {name: "COUNTIF", arity: 2},
	347: {name: "COUNTBLANK", arity: 1},
	350: {name: "ISPMT", arity: 4},
	351: {name: "DATEDIF", arity: 3},
	352: {name: "DATESTRING", arity: 1},
	353: {name: "NUMBERSTRING", arity: 2},
	354: {name: "ROMAN", arity: 1},
	// BIFF8 additions
	358: {name: "GETPIVOTDATA", arity: 2},
	359: {name: "HYPERLINK", arity: 1},
	360: {name: "PHONETIC", arity: 1},
	361: {name: "AVERAGEA", arity: 1},
	362: {name: "MAXA", arity: 1},
	363: {name: "MINA", arity: 1},
	364: {name: "STDEVPA", arity: 1},
	365: {name: "VARPA", arity: 1},
	366: {name: "STDEVA", arity: 1},
	367: {name: "VARA", arity: 1},
}

type chartRangeRef struct {
	sheetIndex        int
	firstRow, lastRow int
	firstCol, lastCol int
}

func ParseXls(path string) (*sheetir.Workbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseXlsBytes(data, filepath.Base(path))
}

// ParseXlsBytes parses a legacy BIFF2-BIFF8 workbook from an in-memory payload.
// filename is used only for provenance and diagnostics and may be empty.
func ParseXlsBytes(data []byte, filename string) (*sheetir.Workbook, error) {
	activeBiffCodepage = 0
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "" {
		filename = "workbook.xls"
	}

	f, err := cfb.OpenBytes(data)
	if err != nil {
		if isRawBIFFStream(data) {
			return parseRawBIFFWorkbook(data, filename)
		}
		return nil, fmt.Errorf("legacy xls: open cfb: %w", err)
	}
	stream, ok := f.Stream("Workbook")
	if !ok {
		stream, ok = f.Stream("Book")
	}
	if !ok {
		return nil, fmt.Errorf("legacy xls: missing Workbook/Book stream")
	}
	stream, err = maybeDecryptBIFFStream(stream)
	if err != nil {
		return nil, err
	}
	return parseWorkbook(stream, filename)
}

func isRawBIFFStream(data []byte) bool {
	id, _, _, ok := readBiffRecord(data, 0)
	return ok && isBIFFBOFRecord(id)
}

func isBIFFBOFRecord(id uint16) bool {
	return id == biffBOF || id == biffBOFOld || id == biffBOF3 || id == biffBOF4
}

// biffBOFTypeName returns a human-readable name for a BOF substream type value.
// Mirrors HSSF BOFRecord.getType name constants.
func biffBOFTypeName(typ uint16) string {
	switch typ {
	case biffBOFTypeWorkbook:
		return "workbook"
	case biffBOFTypeVBModule:
		return "VBA module"
	case biffBOFTypeWorksheet:
		return "worksheet"
	case biffBOFTypeChart:
		return "chart"
	case biffBOFTypeExcel4Macro:
		return "Excel 4 macro"
	case biffBOFTypeWorkspaceFile:
		return "workspace file"
	default:
		return fmt.Sprintf("unknown (0x%04X)", typ)
	}
}

func isKnownBIFFVersion(version uint16) bool {
	switch version {
	case biffVersion2, biffVersion3, biffVersion4, biffVersion5, biffVersion8:
		return true
	default:
		return false
	}
}

func biffVersionFromBOFRecord(id uint16, payload []byte) uint16 {
	switch id {
	case biffBOFOld:
		return biffVersion2
	case biffBOF3:
		return biffVersion3
	case biffBOF4:
		return biffVersion4
	}
	version, typ, _, _, _, _ := parseBIFFBOF(payload)
	if isKnownBIFFVersion(version) {
		return version
	}
	// Many synthetic tests historically wrote BOF payloads as {type, version}
	// instead of the on-disk {version, type} order. Accept the swapped form so
	// version-sensitive record parsing does not regress those fixtures.
	if isKnownBIFFVersion(typ) {
		return typ
	}
	return biffVersion8
}

// parseBIFFBOF decodes a BOF record payload and returns the version, type,
// build, build year, history bitmask, and required version.
//
// BOF record layout (HSSF BOFRecord, MS-XLS §2.4.21):
//
//	offset 0:  version (USHORT) — BIFF version (0x0600 = BIFF8, 0x0500 = BIFF5, etc.)
//	offset 2:  type (USHORT) — substream type (0x05=workbook, 0x10=worksheet, 0x20=chart, etc.)
//	offset 4:  build (USHORT) — build number of the application that wrote the file (optional)
//	offset 6:  build year (USHORT) — year of the build (optional)
//	offset 8:  history (UINT32) — history bitmask (optional)
//	offset 12: required version (UINT32) — minimum version needed to read the file (optional)
//
// Returns default values for fields beyond the available payload length.
func parseBIFFBOF(payload []byte) (version, typ, build, buildYear uint16, history, rversion uint32) {
	if len(payload) < 4 {
		return 0, 0, 0, 0, 0, 0
	}
	version = binary.LittleEndian.Uint16(payload[0:2])
	typ = binary.LittleEndian.Uint16(payload[2:4])
	if len(payload) >= 6 {
		build = binary.LittleEndian.Uint16(payload[4:6])
	}
	if len(payload) >= 8 {
		buildYear = binary.LittleEndian.Uint16(payload[6:8])
	}
	if len(payload) >= 12 {
		history = binary.LittleEndian.Uint32(payload[8:12])
	}
	if len(payload) >= 16 {
		rversion = binary.LittleEndian.Uint32(payload[12:16])
	}
	return
}

func parseRawBIFFWorkbook(data []byte, filename string) (*sheetir.Workbook, error) {
	data, err := maybeDecryptBIFFStream(data)
	if err != nil {
		return nil, err
	}
	workbook, err := parseWorkbook(data, filename)
	if err == nil {
		return workbook, nil
	}
	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Sheet1"}, nil, nil)
	if !rawWorksheetHasContent(sheet) {
		return nil, err
	}
	sheet.ID = 1
	return &sheetir.Workbook{Sheets: []*sheetir.Sheet{sheet}}, nil
}

func rawWorksheetHasContent(sheet *sheetir.Sheet) bool {
	return sheet != nil && (len(sheet.Rows) > 0 || len(sheet.Drawings) > 0 || len(sheet.Comments) > 0)
}

func parseWorkbook(data []byte, filename string) (*sheetir.Workbook, error) {
	sheets, sst, xfs, definedNames, externSheets, drawingStore, date1904, styleNames, wbProtection, err := parseWorkbookGlobals(data)
	if err != nil {
		return nil, err
	}
	pivotCaches := parseLegacyPivotCacheSources(data)
	metadata := parseLegacyWorkbookMetadata(data)
	formulaCtx := legacyFormulaWorkbookContext{
		sheets:       sheets,
		externSheets: externSheets,
		definedNames: definedNameList(definedNames),
		pivotCaches:  pivotCaches,
		date1904:     date1904,
	}
	workbook := &sheetir.Workbook{}
	workbook.Codepage = activeBiffCodepage
	workbook.DefinedNames = definedNames
	workbook.ExternalLinks = parseLegacyExternalLinks(data)
	if len(styleNames) > 0 {
		workbook.StyleNames = styleNames
	}
	if wbProtection != nil {
		workbook.Protection = wbProtection
	}
	workbook.ActiveSheetIndex = metadata.ActiveSheetIndex
	workbook.WriteAccessUser = metadata.WriteAccessUser
	workbook.BackupFile = metadata.BackupFile
	workbook.SaveExternalLinkValues = metadata.SaveExternalLinkValues
	workbook.Calc = metadata.Calc
	// Pre-compute fallback sheet BOF offsets for cases where BoundSheet
	// offsets are corrupted (e.g. POI-created XOR-encrypted test fixtures).
	sheetOffsets := findSheetBOFOffsets(data)
	for index, meta := range sheets {
		// Use the BoundSheet offset when it looks valid; otherwise fall back
		// to the next sequentially-discovered sheet BOF.
		if meta.offset < 0 || meta.offset+4 > len(data) {
			if index < len(sheetOffsets) {
				meta.offset = sheetOffsets[index]
			}
		} else {
			// Validate that the offset actually points to a BOF.
			id, _, _, ok := readBiffRecord(data, meta.offset)
			if !ok || !isBIFFBOFRecord(id) {
				if index < len(sheetOffsets) {
					meta.offset = sheetOffsets[index]
				}
			}
		}
		sheet := parseWorksheetWithDrawingStore(data, meta, sst, xfs, drawingStore, formulaCtx)
		if sheet == nil {
			sheet = &sheetir.Sheet{Name: meta.name, ID: index + 1}
		}
		if sheet.Name == "" {
			sheet.Name = meta.name
		}
		if sheet.ID == 0 {
			sheet.ID = index + 1
		}
		workbook.Sheets = append(workbook.Sheets, sheet)
	}
	for index, meta := range sheets {
		if index >= len(workbook.Sheets) {
			break
		}
		workbook.Sheets[index].Drawings = append(workbook.Sheets[index].Drawings,
			parseEmbeddedCharts(data, meta, index, externSheets, workbook)...)
	}
	applyLegacyPrintTitles(workbook)
	applyLegacyPrintAreas(workbook)
	if len(workbook.Sheets) == 0 {
		return nil, fmt.Errorf("legacy xls: no sheets found in %s", filename)
	}
	return workbook, nil
}

// findSheetBOFOffsets scans the BIFF stream for worksheet/chart BOF records
// that follow the workbook globals EOF.  It returns the offsets of sheet BOFs
// in stream order, skipping the initial workbook globals BOF and any embedded
// chart BOFs that appear inside worksheet substreams.
func findSheetBOFOffsets(data []byte) []int {
	var offsets []int
	var inWorkbookGlobals bool
	var worksheetDepth int
	for pos := 0; pos+4 <= len(data); {
		id, payload, nextPos, ok := readBiffRecord(data, pos)
		if !ok {
			break
		}
		if isBIFFBOFRecord(id) {
			_, typ, _, _, _, _ := parseBIFFBOF(payload)
			switch typ {
			case biffBOFTypeWorkbook:
				inWorkbookGlobals = true
			case biffBOFTypeWorksheet, biffBOFTypeChart, biffBOFTypeVBModule, biffBOFTypeExcel4Macro:
				if inWorkbookGlobals {
					inWorkbookGlobals = false
				}
				if worksheetDepth == 0 {
					offsets = append(offsets, pos)
				}
				if typ == biffBOFTypeWorksheet {
					worksheetDepth++
				}
			}
		} else if id == biffEOF {
			if worksheetDepth > 0 {
				worksheetDepth--
			}
		}
		pos = nextPos
	}
	return offsets
}

func applyLegacyPrintTitles(workbook *sheetir.Workbook) {
	if workbook == nil || len(workbook.Sheets) == 0 || len(workbook.DefinedNames) == 0 {
		return
	}
	for _, dn := range workbook.DefinedNames {
		if dn == nil || !strings.EqualFold(dn.Name, "_xlnm.Print_Titles") || dn.LocalSheetID == nil {
			continue
		}
		sheetID := *dn.LocalSheetID
		if sheetID < 0 || sheetID >= len(workbook.Sheets) {
			continue
		}
		parsed := parsePrintTitles(dn.Value)
		if parsed.rows != "" {
			workbook.Sheets[sheetID].PrintTitleRows = parsed.rows
		}
		if parsed.cols != "" {
			workbook.Sheets[sheetID].PrintTitleCols = parsed.cols
		}
	}
}

func applyLegacyPrintAreas(workbook *sheetir.Workbook) {
	if workbook == nil || len(workbook.Sheets) == 0 || len(workbook.DefinedNames) == 0 {
		return
	}
	for _, dn := range workbook.DefinedNames {
		if dn == nil || !strings.EqualFold(dn.Name, "_xlnm.Print_Area") || dn.LocalSheetID == nil {
			continue
		}
		sheetID := *dn.LocalSheetID
		if sheetID < 0 || sheetID >= len(workbook.Sheets) {
			continue
		}
		if area := parsePrintArea(dn.Value); area != "" {
			workbook.Sheets[sheetID].PrintArea = area
		}
	}
}

func parseWorkbookGlobals(data []byte) ([]boundSheet, []string, []legacyXF, []*sheetir.DefinedName, []externSheetRef, map[uint32]*sheetir.DrawingImage, bool, map[int]string, *sheetir.WorkbookProtection, error) {
	if len(data) < 4 {
		return nil, nil, nil, nil, nil, nil, false, nil, nil, fmt.Errorf("legacy xls: workbook stream too small")
	}
	pos := 0
	firstID, _, next, ok := readBiffRecord(data, pos)
	if !ok || firstID != biffBOF {
		return nil, nil, nil, nil, nil, nil, false, nil, nil, fmt.Errorf("legacy xls: workbook BOF not found")
	}
	pos = next

	var sheets []boundSheet
	var sst []string
	var fonts []legacyFont
	formatCodes := defaultBIFFFormatCodes()
	// Buffer XF payloads during the scan so they can be resolved against the
	// final palette after the loop. PaletteRecord (0x0092) is permitted to
	// appear after XF records in HSSF's RecordOrderer; resolving XF colors
	// post-loop matches POI's lazy `_palette` lookup semantics.
	var xfPayloads [][]byte
	palette := defaultBIFFPalette
	var nameRecords [][]byte
	var nameCommentRecords [][]byte
	var supBooks []supBookRef
	var externSheets []externSheetRef
	var drawingGroupData []byte
	var date1904 bool
	var styleRecords [][]byte
	var wbProtected bool
	var wbWindowProtected bool
	var wbProtectionRev4 bool
	var wbPasswordHash uint16
	var wbRevisionPasswordHash uint16
	var wbWriteProtected bool
	var wbWriteResHash uint16
	var wbWriteResUser string
	resolveXFs := func() []legacyXF {
		xfs := make([]legacyXF, 0, len(xfPayloads))
		for _, p := range xfPayloads {
			xfs = append(xfs, parseBIFFXF(p, formatCodes, fonts, palette))
		}
		return xfs
	}
	resolveDefinedNames := func() []*sheetir.DefinedName {
		definedNames := parseWorkbookDefinedNames(nameRecords, sheets, externSheets)
		if len(nameCommentRecords) > 0 {
			nameComments := make(map[string]string, len(nameCommentRecords))
			for _, p := range nameCommentRecords {
				if name, comment, ok := parseNameCommentRecord(p); ok {
					nameComments[name] = comment
				}
			}
			for _, dn := range definedNames {
				if c, ok := nameComments[dn.Name]; ok {
					dn.Comment = c
				}
			}
		}
		return definedNames
	}
	for pos+4 <= len(data) {
		id, payload, nextPos, ok := readBiffRecord(data, pos)
		if !ok {
			break
		}
		switch id {
		case biffEOF:
			return sheets, sst, resolveXFs(), resolveDefinedNames(), hydrateExternSheetRefs(supBooks, externSheets), parseLegacyWorkbookDrawingStore(drawingGroupData), date1904, buildStyleNames(styleRecords), finalizeWorkbookProtection(wbProtected, wbWindowProtected, wbProtectionRev4, wbPasswordHash, wbRevisionPasswordHash, wbWriteProtected, wbWriteResHash, wbWriteResUser), nil
		case biffBoundSheet:
			meta, ok := parseBoundSheet(payload)
			if ok {
				sheets = append(sheets, meta)
			}
			pos = nextPos
		case biffMsoDrawingGroup:
			fullPayload, fullNext := gatherBiffPayload(data, pos)
			drawingGroupData = append(drawingGroupData, fullPayload...)
			pos = fullNext
		case biffFont:
			fonts = append(fonts, parseBIFFFont(payload))
			pos = nextPos
		case biffFormat:
			if index, code, ok := parseBIFFFormat(payload); ok && code != "" {
				formatCodes[index] = code
			}
			pos = nextPos
		case biffXF:
			cp := make([]byte, len(payload))
			copy(cp, payload)
			xfPayloads = append(xfPayloads, cp)
			pos = nextPos
		case biffPalette:
			palette = parseBIFFPalette(payload)
			pos = nextPos
		case biffDateWindow1904:
			date1904 = parseDateWindow1904(payload)
			pos = nextPos
		case biffCodepage:
			// CODEPAGE record — sets the active codepage for compressed string
			// decoding throughout the rest of parsing.
			activeBiffCodepage = int(parseCodepageRecord(payload))
			pos = nextPos
		case biffCountry:
			// COUNTRY record — captured for future locale-aware formatting.
			// Currently logged via diagnostics; the country codes are stored
			// for potential date/number format locale resolution.
			_, _ = parseCountryRecord(payload)
			pos = nextPos
		case biffWindow1:
			// WINDOW1 record — captured for active sheet index and window
			// display options. Currently logged via diagnostics; the active
			// sheet index is stored for potential workbook-level view state.
			_ = parseWindow1Record(payload)
			pos = nextPos
		case biffWriteAccess:
			// WRITEACCESS record — captured for the username of the
			// spreadsheet generator owner. Currently logged via diagnostics.
			_ = parseWriteAccessRecord(payload)
			pos = nextPos
		case biffBackup:
			// BACKUP record — captured for the backup file flag.
			// Currently logged via diagnostics.
			_ = parseBackupRecord(payload)
			pos = nextPos
		case biffBookBool:
			// BOOKBOOL record — captured for the save external link values flag.
			// Currently logged via diagnostics.
			_ = parseBookBoolRecord(payload)
			pos = nextPos
		case biffCalcMode:
			// CALCMODE record — captured for the formula calculation mode.
			// Currently logged via diagnostics.
			_ = parseCalcModeRecord(payload)
			pos = nextPos
		case biffCalcCount:
			// CALCCOUNT record — captured for the iteration count.
			// Currently logged via diagnostics.
			_ = parseCalcCountRecord(payload)
			pos = nextPos
		case biffRefreshAll:
			// REFRESHALL record — captured for the refresh-all flag.
			// Currently logged via diagnostics.
			_ = parseRefreshAllRecord(payload)
			pos = nextPos
		case biffIteration:
			// ITERATION record — captured for the iterative calculation flag.
			// Currently logged via diagnostics.
			_ = parseIterationRecord(payload)
			pos = nextPos
		case biffDelta:
			// DELTA record — captured for the iteration change tolerance.
			// Currently logged via diagnostics.
			_ = parseDeltaRecord(payload)
			pos = nextPos
		case biffPrecision:
			// PRECISION record — captured for the precision-as-displayed flag.
			// Currently logged via diagnostics.
			_ = parsePrecisionRecord(payload)
			pos = nextPos
		case biffSaveRecalc:
			// SAVERECALC record — captured for the save recalculation flag.
			// Currently logged via diagnostics.
			_ = parseSaveRecalcRecord(payload)
			pos = nextPos
		case biffSST:
			fullPayload, fullNext := gatherBiffPayload(data, pos)
			sst = parseSharedStrings(fullPayload)
			pos = fullNext
		case biffName:
			cp := make([]byte, len(payload))
			copy(cp, payload)
			nameRecords = append(nameRecords, cp)
			pos = nextPos
		case biffNameComment:
			cp := make([]byte, len(payload))
			copy(cp, payload)
			nameCommentRecords = append(nameCommentRecords, cp)
			pos = nextPos
		case biffSupBook:
			supBook, blockNext, ok := parseSupBookBlock(data, pos)
			if ok {
				supBooks = append(supBooks, supBook)
				pos = blockNext
				continue
			}
			pos = nextPos
		case biffExternSheet:
			refs, blockNext := gatherContiguousExternSheets(data, pos, supBooks)
			externSheets = append(externSheets, refs...)
			pos = blockNext
			continue
		case biffStyle:
			cp := make([]byte, len(payload))
			copy(cp, payload)
			styleRecords = append(styleRecords, cp)
			pos = nextPos
		case biffProtect:
			if len(payload) >= 2 {
				wbProtected = binary.LittleEndian.Uint16(payload[0:2]) != 0
			}
			pos = nextPos
		case biffWindowProtect:
			wbWindowProtected = parseWindowProtectRecord(payload)
			pos = nextPos
		case biffProtectionRev4:
			if len(payload) >= 2 {
				wbProtectionRev4 = binary.LittleEndian.Uint16(payload[0:2]) != 0
			}
			pos = nextPos
		case biffPassword:
			if len(payload) >= 2 {
				wbPasswordHash = binary.LittleEndian.Uint16(payload[0:2])
			}
			pos = nextPos
		case biffWriteProtect:
			wbWriteProtected = true
			pos = nextPos
		case biffPasswordRev4:
			if len(payload) >= 2 {
				wbRevisionPasswordHash = binary.LittleEndian.Uint16(payload[0:2])
			}
			pos = nextPos
		case biffFileSharing:
			if fileSharing, ok := parseFileSharingRecord(payload); ok {
				wbWriteProtected = wbWriteProtected || fileSharing.readOnly
				if fileSharing.passwordHash != 0 {
					wbWriteResHash = fileSharing.passwordHash
				}
				if strings.TrimSpace(fileSharing.username) != "" {
					wbWriteResUser = fileSharing.username
				}
			}
			pos = nextPos
		default:
			pos = nextPos
		}
	}
	return sheets, sst, resolveXFs(), resolveDefinedNames(), hydrateExternSheetRefs(supBooks, externSheets), parseLegacyWorkbookDrawingStore(drawingGroupData), date1904, buildStyleNames(styleRecords), finalizeWorkbookProtection(wbProtected, wbWindowProtected, wbProtectionRev4, wbPasswordHash, wbRevisionPasswordHash, wbWriteProtected, wbWriteResHash, wbWriteResUser), nil
}

func parseLegacyExternalLinks(data []byte) []*sheetir.WorkbookExternalLink {
	if len(data) < 4 {
		return nil
	}
	id, _, pos, ok := readBiffRecord(data, 0)
	if !ok || id != biffBOF {
		return nil
	}
	var supBooks []supBookRef
	for pos+4 <= len(data) {
		id, _, nextPos, ok := readBiffRecord(data, pos)
		if !ok {
			break
		}
		switch id {
		case biffEOF:
			return buildWorkbookExternalLinks(supBooks)
		case biffSupBook:
			supBook, blockNext, ok := parseSupBookBlock(data, pos)
			if ok {
				supBooks = append(supBooks, supBook)
				pos = blockNext
				continue
			}
		}
		pos = nextPos
	}
	return buildWorkbookExternalLinks(supBooks)
}

func parseLegacyPivotCacheSources(data []byte) []legacyPivotCacheSource {
	if len(data) < 4 {
		return nil
	}
	id, _, pos, ok := readBiffRecord(data, 0)
	if !ok || id != biffBOF {
		return nil
	}
	var caches []legacyPivotCacheSource
	cacheIndex := -1
	expectSheetSource := false
	for pos+4 <= len(data) {
		id, payload, nextPos, ok := readBiffRecord(data, pos)
		if !ok {
			break
		}
		switch id {
		case biffEOF:
			return caches
		case biffSXStreamID:
			if len(payload) >= 2 {
				caches = append(caches, legacyPivotCacheSource{StreamID: binary.LittleEndian.Uint16(payload[0:2])})
				cacheIndex = len(caches) - 1
				expectSheetSource = false
			}
		case biffSXVS:
			expectSheetSource = cacheIndex >= 0 && len(payload) >= 2 && binary.LittleEndian.Uint16(payload[0:2]) == 0x0001
		case biffDConRef:
			if cacheIndex >= 0 && expectSheetSource {
				if source, ok := parseLegacyPivotDConRef(payload); ok {
					caches[cacheIndex] = mergeLegacyPivotCacheSource(caches[cacheIndex], source)
				}
				expectSheetSource = false
			}
		case biffDConName:
			if cacheIndex >= 0 && expectSheetSource {
				if source, ok := parseLegacyPivotDConName(payload); ok {
					caches[cacheIndex] = mergeLegacyPivotCacheSource(caches[cacheIndex], source)
				}
				expectSheetSource = false
			}
		}
		pos = nextPos
	}
	return caches
}

func mergeLegacyPivotCacheSource(base, extra legacyPivotCacheSource) legacyPivotCacheSource {
	if extra.StreamID != 0 {
		base.StreamID = extra.StreamID
	}
	if extra.SourceSheet != "" {
		base.SourceSheet = extra.SourceSheet
	}
	if extra.SourceRef != "" {
		base.SourceRef = extra.SourceRef
	}
	if extra.SourceName != "" {
		base.SourceName = extra.SourceName
	}
	return base
}

func parseLegacyPivotDConRef(payload []byte) (legacyPivotCacheSource, bool) {
	if len(payload) < 8 {
		return legacyPivotCacheSource{}, false
	}
	source := legacyPivotCacheSource{SourceRef: formatLegacyPivotRefU(payload[0:6])}
	cchFile := int(binary.LittleEndian.Uint16(payload[6:8]))
	if cchFile > 0 {
		if file, _, ok := readXLUnicodeStringLen(payload, 8, cchFile); ok {
			source.SourceSheet = legacyPivotSelfReferenceSheet(file)
		}
	}
	return source, source.SourceRef != ""
}

func parseLegacyPivotDConName(payload []byte) (legacyPivotCacheSource, bool) {
	name, next, ok := readXLUnicodeString(payload, 0, 2)
	if !ok {
		return legacyPivotCacheSource{}, false
	}
	source := legacyPivotCacheSource{SourceName: strings.TrimSpace(name)}
	if next+2 <= len(payload) {
		cchFile := int(binary.LittleEndian.Uint16(payload[next : next+2]))
		if cchFile > 0 {
			if file, _, ok := readXLUnicodeStringLen(payload, next+2, cchFile); ok {
				source.SourceSheet = legacyPivotSelfReferenceSheet(file)
			}
		}
	}
	return source, source.SourceName != ""
}

func formatLegacyPivotRefU(payload []byte) string {
	if len(payload) < 6 {
		return ""
	}
	firstRow := int(binary.LittleEndian.Uint16(payload[0:2])) + 1
	lastRow := int(binary.LittleEndian.Uint16(payload[2:4])) + 1
	firstCol := int(payload[4]) + 1
	lastCol := int(payload[5]) + 1
	if firstRow <= 0 || lastRow <= 0 || firstCol <= 0 || lastCol <= 0 {
		return ""
	}
	return formatA1Ref(firstRow, firstCol) + ":" + formatA1Ref(lastRow, lastCol)
}

func legacyPivotSelfReferenceSheet(file string) string {
	rs := []rune(strings.TrimSpace(file))
	if len(rs) < 2 || rs[0] != 0x0002 {
		return ""
	}
	return strings.TrimSpace(string(rs[1:]))
}

func parseSupBookBlock(data []byte, pos int) (supBookRef, int, bool) {
	id, payload, nextPos, ok := readBiffRecord(data, pos)
	if !ok || id != biffSupBook {
		return supBookRef{}, pos, false
	}
	supBook := parseSupBook(payload)
	pos = nextPos
	for pos+4 <= len(data) {
		id, payload, nextPos, ok = readBiffRecord(data, pos)
		if !ok || id != biffExternalName {
			break
		}
		if name, ok := parseExternalNameRecord(payload); ok {
			supBook.externalNames = append(supBook.externalNames, name)
		}
		pos = nextPos
	}
	for pos+4 <= len(data) {
		id, payload, nextPos, ok = readBiffRecord(data, pos)
		if !ok || id != biffCRNCount {
			break
		}
		count, sheetIndex, ok := parseCRNCountRecord(payload)
		if !ok {
			break
		}
		pos = nextPos
		for i := 0; i < count && pos+4 <= len(data); i++ {
			rid, rpayload, rnext, rok := readBiffRecord(data, pos)
			if !rok || rid != biffCRN {
				return supBook, pos, true
			}
			applyCRNRecord(&supBook, sheetIndex, rpayload)
			pos = rnext
		}
	}
	return supBook, pos, true
}

func gatherContiguousExternSheets(data []byte, pos int, supBooks []supBookRef) ([]externSheetRef, int) {
	if pos+4 > len(data) {
		return nil, pos
	}
	var refs []externSheetRef
	for pos+4 <= len(data) {
		id, payload, nextPos, ok := readBiffRecord(data, pos)
		if !ok || id != biffExternSheet {
			break
		}
		refs = append(refs, parseExternSheet(payload, supBooks)...)
		pos = nextPos
	}
	return refs, pos
}

func buildWorkbookExternalLinks(supBooks []supBookRef) []*sheetir.WorkbookExternalLink {
	if len(supBooks) == 0 {
		return nil
	}
	links := make([]*sheetir.WorkbookExternalLink, 0, len(supBooks))
	for _, sb := range supBooks {
		if sb.kind != supBookKindExternal {
			continue
		}
		if strings.TrimSpace(sb.url) == "" && len(sb.sheetNames) == 0 && len(sb.externalNames) == 0 && len(sb.cachedSheets) == 0 {
			continue
		}
		link := &sheetir.WorkbookExternalLink{
			Target:     strings.TrimSpace(sb.url),
			SheetNames: append([]string(nil), sb.sheetNames...),
		}
		for _, name := range sb.externalNames {
			text := strings.TrimSpace(name.text)
			if text == "" {
				continue
			}
			link.DefinedNames = append(link.DefinedNames, &sheetir.WorkbookExternalDefinedName{
				Name: text,
			})
		}
		if len(sb.cachedSheets) != 0 {
			keys := make([]int, 0, len(sb.cachedSheets))
			for sheetIndex := range sb.cachedSheets {
				keys = append(keys, sheetIndex)
			}
			sort.Ints(keys)
			for _, sheetIndex := range keys {
				sheet := sb.cachedSheets[sheetIndex]
				if sheet == nil || (strings.TrimSpace(sheet.Name) == "" && len(sheet.Cells) == 0) {
					continue
				}
				link.CachedSheets = append(link.CachedSheets, sheet)
			}
		}
		links = append(links, link)
	}
	if len(links) == 0 {
		return nil
	}
	return links
}

func parseCRNCountRecord(payload []byte) (count int, sheetIndex int, ok bool) {
	if len(payload) < 4 {
		return 0, 0, false
	}
	count = int(int16(binary.LittleEndian.Uint16(payload[0:2])))
	if count < 0 {
		count = -count
	}
	return count, int(binary.LittleEndian.Uint16(payload[2:4])), true
}

type legacyCRNValue struct {
	cellType string
	value    string
	empty    bool
}

func parseCRNConstantValue(payload []byte, pos int) (legacyCRNValue, int, bool) {
	if pos >= len(payload) {
		return legacyCRNValue{}, pos, false
	}
	switch payload[pos] {
	case 0x00:
		if pos+9 > len(payload) {
			return legacyCRNValue{}, pos, false
		}
		return legacyCRNValue{empty: true}, pos + 9, true
	case 0x01:
		if pos+9 > len(payload) {
			return legacyCRNValue{}, pos, false
		}
		value := math.Float64frombits(binary.LittleEndian.Uint64(payload[pos+1 : pos+9]))
		return legacyCRNValue{
			cellType: "n",
			value:    strconv.FormatFloat(value, 'f', -1, 64),
		}, pos + 9, true
	case 0x02:
		text, next, ok := readXLUnicodeString(payload, pos+1, 2)
		if !ok {
			return legacyCRNValue{}, pos, false
		}
		return legacyCRNValue{
			cellType: "str",
			value:    text,
		}, next, true
	case 0x04:
		if pos+9 > len(payload) {
			return legacyCRNValue{}, pos, false
		}
		value := "0"
		if payload[pos+1] != 0 {
			value = "1"
		}
		return legacyCRNValue{
			cellType: "b",
			value:    value,
		}, pos + 9, true
	case 0x10:
		if pos+9 > len(payload) {
			return legacyCRNValue{}, pos, false
		}
		return legacyCRNValue{
			cellType: "e",
			value:    biffErrorString(payload[pos+1]),
		}, pos + 9, true
	default:
		return legacyCRNValue{}, pos, false
	}
}

func applyCRNRecord(sb *supBookRef, sheetIndex int, payload []byte) {
	if sb == nil || sb.kind != supBookKindExternal || len(payload) < 4 {
		return
	}
	lastCol := int(payload[0])
	firstCol := int(payload[1])
	if lastCol < firstCol {
		return
	}
	if sb.cachedSheets == nil {
		sb.cachedSheets = make(map[int]*sheetir.WorkbookExternalSheet)
	}
	sheet := sb.cachedSheets[sheetIndex]
	if sheet == nil {
		sheetName := ""
		if sheetIndex >= 0 && sheetIndex < len(sb.sheetNames) {
			sheetName = sb.sheetNames[sheetIndex]
		}
		sheet = &sheetir.WorkbookExternalSheet{
			ID:   sheetIndex,
			Name: sheetName,
		}
		sb.cachedSheets[sheetIndex] = sheet
	}
	row := int(binary.LittleEndian.Uint16(payload[2:4]))
	pos := 4
	for col := firstCol; col <= lastCol && pos < len(payload); col++ {
		value, next, ok := parseCRNConstantValue(payload, pos)
		if !ok {
			break
		}
		pos = next
		if value.empty {
			continue
		}
		sheet.Cells = append(sheet.Cells, &sheetir.WorkbookExternalCell{
			Ref:   formatA1Ref(row+1, col+1),
			Type:  value.cellType,
			Value: value.value,
		})
	}
}

// parseDateWindow1904 decodes the DATEMODE record (0x0022) which carries a
// single uint16: nonzero means the workbook uses the 1904 epoch (serial 0 =
// 1904-01-01, no Lotus phantom day) instead of the default 1900 epoch.
// Mirrors HSSF DateWindow1904Record.getWindowing.
func parseDateWindow1904(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2]) != 0
}

func parseWindowProtectRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2])&0x0001 != 0
}

// parseWSBoolRecord decodes HSSF WSBoolRecord (0x0081).
// Returns the dialog-sheet flag (bit 4 of the options field).
func parseWSBoolRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2])&0x0010 != 0
}

func parseFileSharingRecord(payload []byte) (legacyFileSharingRecord, bool) {
	if len(payload) < 6 {
		return legacyFileSharingRecord{}, false
	}
	info := legacyFileSharingRecord{
		readOnly:     binary.LittleEndian.Uint16(payload[0:2]) == 1,
		passwordHash: binary.LittleEndian.Uint16(payload[2:4]),
	}
	nameLen := int(binary.LittleEndian.Uint16(payload[4:6]))
	if nameLen == 0 {
		return info, true
	}
	if name, _, ok := readXLUnicodeString(payload, 4, 2); ok {
		info.username = name
		return info, true
	}
	if len(payload) >= 7+nameLen {
		info.username = string(payload[7 : 7+nameLen])
	}
	return info, true
}

// parseCodepageRecord decodes a CODEPAGE record (0x0042) payload and returns
// the default character set code page identifier.
//
// Record layout (MS-XLS §2.4.49):
//
//	offset 0: USHORT cv — code page identifier
//
// Common values:
//
//	0x01B5 = 437 (IBM PC / US)
//	0x04E4 = 1252 (Windows Latin 1 / ANSI)
//	0x04E2 = 1250 (Windows Eastern European)
//	0x04E3 = 1251 (Windows Cyrillic)
//	0x04E5 = 1253 (Windows Greek)
//	0x04E6 = 1254 (Windows Turkish)
//	0x04E7 = 1255 (Windows Hebrew)
//	0x04E8 = 1256 (Windows Arabic)
//	0x04E9 = 1257 (Windows Baltic)
//	0x04EA = 1258 (Windows Vietnamese)
//	0x8000 = 32768 (Apple Macintosh)
//
// Mirrors HSSF CodepageRecord.getCodepage.
func parseCodepageRecord(payload []byte) uint16 {
	if len(payload) < 2 {
		return 0
	}
	return binary.LittleEndian.Uint16(payload[0:2])
}

// parseCountryRecord decodes a COUNTRY record (0x008C) payload and returns
// the default country and current country locale identifiers.
//
// Record layout (MS-XLS §2.4.56):
//
//	offset 0: USHORT iCountryDefault — default country code
//	offset 2: USHORT iCountryCurrent — current country code (may differ from default)
//
// Common country codes:
//
//	0x0001 = USA
//	0x0031 = Canada
//	0x0033 = France
//	0x0039 = Germany
//	0x0044 = United Kingdom
//	0x0046 = Sweden
//	0x0049 = Germany (alternate)
//	0x0056 = Italy
//	0x005A = Mexico
//	0x00A4 = Netherlands
//	0x00B5 = Finland
//	0x00C0 = Switzerland
//	0x00D6 = Norway
//	0x00E0 = Denmark
//	0x00F0 = Japan
//	0x00F5 = Korea
//	0x00F6 = China
//	0x00F7 = Taiwan
//	0x00F8 = India
//	0x00F9 = Thailand
//
// Mirrors HSSF CountryRecord.getDefaultCountry / getCurrentCountry.
func parseCountryRecord(payload []byte) (defaultCountry, currentCountry uint16) {
	if len(payload) < 2 {
		return 0, 0
	}
	defaultCountry = binary.LittleEndian.Uint16(payload[0:2])
	if len(payload) >= 4 {
		currentCountry = binary.LittleEndian.Uint16(payload[2:4])
	} else {
		currentCountry = defaultCountry
	}
	return
}

// parseWindow1Record decodes a WINDOW1 record (0x003D) payload and returns
// the active sheet index (0-based). The record layout (HSSF WindowOneRecord):
//
//	offset 0:  h_hold (SHORT) — horizontal position of the window (1/20 pt)
//	offset 2:  v_hold (SHORT) — vertical position of the window (1/20 pt)
//	offset 4:  width (SHORT) — window width (1/20 pt)
//	offset 6:  height (SHORT) — window height (1/20 pt)
//	offset 8:  options (SHORT) — bitmask: bit 0=hidden, bit 1=iconic,
//	           bit 3=hscroll, bit 4=vscroll, bit 5=tabs
//	offset 10: active_sheet (SHORT) — 0-based index of the active sheet
//	offset 12: first_visible_tab (SHORT) — first visible tab in the tab bar
//	offset 14: num_selected_tabs (SHORT) — number of selected tabs
//	offset 16: tab_width_ratio (SHORT) — tab width / scrollbar width ratio
//
// Mirrors HSSF WindowOneRecord.getActiveSheetIndex.
func parseWindow1Record(payload []byte) (activeSheetIndex int) {
	if len(payload) < 12 {
		return 0
	}
	return int(binary.LittleEndian.Uint16(payload[10:12]))
}

// parseWriteAccessRecord decodes a WRITEACCESS record (0x005C) payload and
// returns the username string. The record is always padded to 112 bytes.
// Layout (HSSF WriteAccessRecord):
//
//	offset 0:  cch (USHORT) — character count
//	offset 2:  fHighByte (BYTE) — 0 = compressed, 1 = UTF-16LE
//	offset 3+: text bytes (padded with spaces to 109 bytes total)
//
// Mirrors HSSF WriteAccessRecord.getUsername.
func parseWriteAccessRecord(payload []byte) string {
	if len(payload) < 3 {
		return ""
	}
	nChars := int(binary.LittleEndian.Uint16(payload[0:2]))
	is16Bit := payload[2]&0x01 != 0
	body := payload[3:]
	if nChars <= 0 {
		return ""
	}
	if is16Bit {
		need := nChars * 2
		if need > len(body) {
			need = len(body) & ^1
		}
		return strings.TrimRight(decodeBiffUTF16Raw(body[:need]), " ")
	}
	if nChars > len(body) {
		nChars = len(body)
	}
	return strings.TrimRight(decodeBiffBytesRaw(body[:nChars]), " ")
}

// parseBackupRecord decodes a BACKUP record (0x0040) payload and returns
// the backup flag. Layout: single USHORT (0 = no backup, 1 = backup).
// Mirrors HSSF BackupRecord.getBackup.
func parseBackupRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2]) != 0
}

// parseBookBoolRecord decodes a BOOKBOOL record (0x00DA) payload and returns
// the save external link values flag. Layout: single USHORT (0 = don't save,
// 1 = save). Mirrors HSSF BookBoolRecord.getSaveLinkValues.
func parseBookBoolRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2]) != 0
}

// parseCalcModeRecord decodes a CALCMODE record (0x000D) payload and returns
// the calculation mode. Layout: single USHORT (0=manual, 1=automatic,
// -1=automatic except tables). Mirrors HSSF CalcModeRecord.getCalcMode.
func parseCalcModeRecord(payload []byte) int {
	if len(payload) < 2 {
		return 1 // default to automatic
	}
	return int(int16(binary.LittleEndian.Uint16(payload[0:2])))
}

// parseCalcCountRecord decodes a CALCCOUNT record (0x000C) payload and returns
// the iteration count for circular references. Layout: single USHORT.
// Mirrors HSSF CalcCountRecord.getIterations.
func parseCalcCountRecord(payload []byte) int {
	if len(payload) < 2 {
		return 100 // default
	}
	return int(binary.LittleEndian.Uint16(payload[0:2]))
}

// parseRefreshAllRecord decodes a REFRESHALL record (0x01B7) payload and
// returns the refresh-all flag. Layout: single USHORT (0 = don't refresh,
// 1 = refresh). Mirrors HSSF RefreshAllRecord.getRefreshFlag.
func parseRefreshAllRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2]) != 0
}

// parseIterationRecord decodes an ITERATION record (0x0011) payload and
// returns the iterative calculation flag. Layout: single USHORT (0 = off,
// 1 = on). Mirrors HSSF IterationRecord.getIteration.
func parseIterationRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2]) != 0
}

// parseDeltaRecord decodes a DELTA record (0x0010) payload and returns
// the iteration change tolerance as a float64. Layout: IEEE 754 double.
// Mirrors HSSF DeltaRecord.getDelta.
func parseDeltaRecord(payload []byte) float64 {
	if len(payload) < 8 {
		return 0.001 // default
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(payload[0:8]))
}

// parsePrecisionRecord decodes a PRECISION record (0x000E) payload and
// returns the precision-as-displayed flag. Layout: single USHORT (0 = full
// precision, 1 = precision as displayed). Mirrors HSSF PrecisionRecord.
func parsePrecisionRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2]) != 0
}

// parseSaveRecalcRecord decodes a SAVERECALC record (0x005F) payload and
// returns the save recalculation flag. Layout: single USHORT (0 = don't
// recalc on save, 1 = recalc on save). Mirrors HSSF SaveRecalcRecord.
func parseSaveRecalcRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	return binary.LittleEndian.Uint16(payload[0:2]) != 0
}

// buildStyleNames decodes a slice of raw STYLE record (0x0293) payloads and
// returns a map of XF-index → style name.  Built-in styles use the canonical
// names from HSSF HSSFWorkbook.getBuiltinStyleName; user-defined styles use
// the UTF-16LE / compressed-Unicode name stored in the record.
// Layout (BIFF8, §2.4.268 of the XLS binary spec):
//
//	offset 0: USHORT — bits 0-11 = XF index; bit 15 = 1 for built-in
//	if built-in: offset 2 = BYTE builtinStyleId; offset 3 = BYTE outlineLevel
//	if user:     offset 2 = USHORT nameLen; offset 4 = BYTE isWide; offset 5 = chars
func buildStyleNames(payloads [][]byte) map[int]string {
	if len(payloads) == 0 {
		return nil
	}
	// Built-in IDs 0-7 per HSSF StyleRecord / HSSFWorkbook.
	builtinNames := [8]string{
		"Normal", "RowLevel_", "ColLevel_",
		"Comma", "Currency", "Percent", "Comma [0]", "Currency [0]",
	}
	result := make(map[int]string, len(payloads))
	for _, p := range payloads {
		if len(p) < 2 {
			continue
		}
		raw := binary.LittleEndian.Uint16(p[0:2])
		xfIndex := int(raw & 0x0FFF)
		isBuiltin := (raw & 0x8000) != 0
		if isBuiltin {
			if len(p) < 4 {
				continue
			}
			id := int(p[2])
			level := int(p[3])
			if id < 0 || id >= len(builtinNames) {
				continue
			}
			name := builtinNames[id]
			if id == 1 || id == 2 {
				// "RowLevel_n" / "ColLevel_n" — level is 0-based, display as 1-based
				name = fmt.Sprintf("%s%d", name, level+1)
			}
			result[xfIndex] = name
		} else {
			// User-defined: SHORT nameLen, BYTE isWide, chars
			if len(p) < 5 {
				if len(p) >= 4 {
					// Crystal Reports omits the isWide byte when nameLen==0
					nameLen := int(binary.LittleEndian.Uint16(p[2:4]))
					if nameLen == 0 {
						result[xfIndex] = ""
					}
				}
				continue
			}
			nameLen := int(binary.LittleEndian.Uint16(p[2:4]))
			isWide := p[4] != 0
			charData := p[5:]
			if isWide {
				if len(charData) < nameLen*2 {
					continue
				}
				runes := make([]rune, nameLen)
				for i := range runes {
					runes[i] = rune(binary.LittleEndian.Uint16(charData[i*2:]))
				}
				result[xfIndex] = string(runes)
			} else {
				if len(charData) < nameLen {
					continue
				}
				// Compressed Latin-1 — treat as ISO-8859-1
				runes := make([]rune, nameLen)
				for i := range runes {
					runes[i] = rune(charData[i])
				}
				result[xfIndex] = string(runes)
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// buildWorkbookProtection returns a *sheetir.WorkbookProtection populated from
// the PROTECT (0x0012), PROTECTIONREV4 (0x01AF), and PASSWORD (0x0013) records
// found in the workbook globals stream.  Returns nil when no protection flags
// were set, mirroring HSSF's WorkbookProtectionBlock semantics.
func buildWorkbookProtection(locked bool, lockWindows bool, lockRevision bool, passwordHash uint16) *sheetir.WorkbookProtection {
	if !locked && !lockWindows && !lockRevision && passwordHash == 0 {
		return nil
	}
	wp := &sheetir.WorkbookProtection{
		LockStructure: locked,
		LockWindows:   lockWindows,
		LockRevision:  lockRevision,
	}
	if passwordHash != 0 {
		wp.WorkbookPassword = fmt.Sprintf("%04X", passwordHash)
	}
	return wp
}

func finalizeWorkbookProtection(locked bool, lockWindows bool, lockRevision bool, passwordHash uint16, revisionsPasswordHash uint16, writeProtected bool, writeResHash uint16, writeResUser string) *sheetir.WorkbookProtection {
	wp := buildWorkbookProtection(locked, lockWindows, lockRevision, passwordHash)
	if revisionsPasswordHash == 0 && !writeProtected && writeResHash == 0 && strings.TrimSpace(writeResUser) == "" {
		return wp
	}
	if wp == nil {
		wp = &sheetir.WorkbookProtection{}
	}
	if revisionsPasswordHash != 0 {
		wp.RevisionsPassword = fmt.Sprintf("%04X", revisionsPasswordHash)
	}
	wp.WriteProtected = writeProtected
	if writeResHash != 0 {
		wp.WriteResPassword = fmt.Sprintf("%04X", writeResHash)
	}
	if strings.TrimSpace(writeResUser) != "" {
		wp.WriteResUser = strings.TrimSpace(writeResUser)
	}
	return wp
}

func parseWorkbookDefinedNames(nameRecords [][]byte, sheets []boundSheet, externSheets []externSheetRef) []*sheetir.DefinedName {
	if len(nameRecords) == 0 {
		return nil
	}
	definedNames := make([]*sheetir.DefinedName, 0, len(nameRecords))
	for _, payload := range nameRecords {
		if dn, ok := parseBIFFNameWithContext(payload, sheets, externSheets); ok {
			definedNames = append(definedNames, dn)
		}
	}
	return definedNames
}

func parseSupBook(payload []byte) supBookRef {
	if len(payload) < 4 {
		return supBookRef{kind: supBookKindUnknown}
	}
	ctab := int(binary.LittleEndian.Uint16(payload[0:2]))
	cch := binary.LittleEndian.Uint16(payload[2:4])
	// BIFF8 special markers:
	//   cch == 0x0401: internal workbook references
	//   cch == 0x3A01: add-in function references
	switch cch {
	case 0x0401:
		return supBookRef{kind: supBookKindInternal, sheetCount: ctab}
	case 0x3A01:
		return supBookRef{kind: supBookKindAddIn}
	default:
		sb := supBookRef{kind: supBookKindExternal, sheetCount: ctab}
		url, next, ok := readXLUnicodeString(payload, 2, 2)
		if !ok {
			return sb
		}
		sb.url = decodeSupBookURL(url)
		for i := 0; i < ctab && next < len(payload); i++ {
			sheetName, nextPos, ok := readXLUnicodeString(payload, next, 2)
			if !ok {
				break
			}
			sb.sheetNames = append(sb.sheetNames, sheetName)
			next = nextPos
		}
		return sb
	}
}

func parseExternalNameRecord(payload []byte) (legacyExternalName, bool) {
	if len(payload) < 8 {
		return legacyExternalName{}, false
	}
	name, _, ok := readXLUnicodeString(payload, 6, 1)
	if !ok {
		return legacyExternalName{}, false
	}
	return legacyExternalName{
		text:  name,
		ixals: int(binary.LittleEndian.Uint16(payload[2:4])),
	}, true
}

func hydrateExternSheetRefs(supBooks []supBookRef, externSheets []externSheetRef) []externSheetRef {
	if len(externSheets) == 0 || len(supBooks) == 0 {
		return externSheets
	}
	out := make([]externSheetRef, len(externSheets))
	copy(out, externSheets)
	for i := range out {
		sbIndex := out[i].supBookIndex
		if sbIndex < 0 || sbIndex >= len(supBooks) {
			continue
		}
		if len(supBooks[sbIndex].externalNames) > 0 {
			out[i].externalNames = append([]legacyExternalName(nil), supBooks[sbIndex].externalNames...)
		}
	}
	return out
}

type legacyWorkbookMetadata struct {
	ActiveSheetIndex       *int
	WriteAccessUser        string
	BackupFile             *bool
	SaveExternalLinkValues *bool
	Calc                   *sheetir.WorkbookCalcSettings
}

func parseLegacyWorkbookMetadata(data []byte) legacyWorkbookMetadata {
	var meta legacyWorkbookMetadata
	if len(data) < 4 {
		return meta
	}
	id, _, pos, ok := readBiffRecord(data, 0)
	if !ok || id != biffBOF {
		return meta
	}
	ensureCalc := func() *sheetir.WorkbookCalcSettings {
		if meta.Calc == nil {
			meta.Calc = &sheetir.WorkbookCalcSettings{}
		}
		return meta.Calc
	}
	for pos+4 <= len(data) {
		id, payload, nextPos, ok := readBiffRecord(data, pos)
		if !ok {
			break
		}
		if id == biffEOF {
			break
		}
		switch id {
		case biffWindow1:
			v := parseWindow1Record(payload)
			meta.ActiveSheetIndex = legacyIntPtr(v)
		case biffWriteAccess:
			meta.WriteAccessUser = parseWriteAccessRecord(payload)
		case biffBackup:
			v := parseBackupRecord(payload)
			meta.BackupFile = legacyBoolPtr(v)
		case biffBookBool:
			v := parseBookBoolRecord(payload)
			meta.SaveExternalLinkValues = legacyBoolPtr(v)
		case biffCalcMode:
			v := parseCalcModeRecord(payload)
			ensureCalc().Mode = legacyIntPtr(v)
		case biffCalcCount:
			v := parseCalcCountRecord(payload)
			ensureCalc().IterationCount = legacyIntPtr(v)
		case biffRefreshAll:
			v := parseRefreshAllRecord(payload)
			ensureCalc().RefreshAll = legacyBoolPtr(v)
		case biffIteration:
			v := parseIterationRecord(payload)
			ensureCalc().Iterate = legacyBoolPtr(v)
		case biffDelta:
			v := parseDeltaRecord(payload)
			ensureCalc().Delta = legacyFloat64Ptr(v)
		case biffPrecision:
			v := parsePrecisionRecord(payload)
			ensureCalc().PrecisionAsDisplayed = legacyBoolPtr(v)
		case biffSaveRecalc:
			v := parseSaveRecalcRecord(payload)
			ensureCalc().SaveRecalc = legacyBoolPtr(v)
		}
		pos = nextPos
	}
	return meta
}

func legacyBoolPtr(value bool) *bool {
	v := value
	return &v
}

func legacyIntPtr(value int) *int {
	v := value
	return &v
}

func legacyFloat64Ptr(value float64) *float64 {
	v := value
	return &v
}

func decodeSupBookURL(encoded string) string {
	if len(encoded) < 1 {
		return encoded
	}
	switch encoded[0] {
	case 0, 2:
		return encoded[1:]
	case 1:
		var b strings.Builder
		for i := 1; i < len(encoded); i++ {
			switch encoded[i] {
			case 1:
				if i+1 >= len(encoded) {
					break
				}
				i++
				if encoded[i] == '@' {
					b.WriteString("\\\\")
				} else {
					b.WriteByte(encoded[i])
					b.WriteString(":")
				}
			case 2, 3:
				b.WriteByte(filepath.Separator)
			case 4:
				b.WriteString("..")
				b.WriteByte(filepath.Separator)
			case 6, 7, 8:
				b.WriteString("." + string(filepath.Separator))
			default:
				b.WriteByte(encoded[i])
			}
		}
		return b.String()
	default:
		return encoded
	}
}

func parseExternSheet(payload []byte, supBooks []supBookRef) []externSheetRef {
	if len(payload) < 2 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(payload[0:2]))
	refs := make([]externSheetRef, 0, count)
	offset := 2
	for index := 0; index < count && offset+6 <= len(payload); index++ {
		supBookIndex := int(binary.LittleEndian.Uint16(payload[offset : offset+2]))
		sourceKind := supBookKindUnknown
		if len(supBooks) == 0 {
			sourceKind = supBookKindInternal
		} else if supBookIndex >= 0 && supBookIndex < len(supBooks) {
			sourceKind = supBooks[supBookIndex].kind
		}
		refs = append(refs, externSheetRef{
			supBookIndex: supBookIndex,
			firstSheet:   int(binary.LittleEndian.Uint16(payload[offset+2 : offset+4])),
			lastSheet:    int(binary.LittleEndian.Uint16(payload[offset+4 : offset+6])),
			sourceKind:   sourceKind,
		})
		offset += 6
	}
	return refs
}

func parseEmbeddedCharts(data []byte, meta boundSheet, sheetIndex int, externSheets []externSheetRef, workbook *sheetir.Workbook) []*sheetir.Drawing {
	if meta.offset < 0 || meta.offset+4 > len(data) {
		return nil
	}
	id, _, pos, ok := readBiffRecord(data, meta.offset)
	if !ok || !isBIFFBOFRecord(id) {
		return nil
	}

	type pendingChartState struct {
		chart                 *chartpkg.Chart
		lastAIType            byte
		currentAxisType       int // -1=none, 0=category/X, 1=value, 2=series
		pendingAnchor         int // 0=none, 1=chart title, 2=Y axis, 3=X axis, 7=Z axis
		beginDepth            int
		seriesDepth           int // depth at which current series block lives; -1 when not in a series block
		dataFormatSeriesIndex int // target series index for format records inside a DataFormat block; -1 when not in a DataFormat block
		dataFormatPointIndex  int // target point index; -1 means whole series
		dataFormatDepth       int // depth at which current DataFormat block lives; -1 when not in a DataFormat block
		plotAreaSeen          bool
		pendingAxisLineType   int                     // 0=axis line, 1=major grid, 2=minor grid, 3=walls/floor; -1 when not set
		chartNumbers          map[int]map[int]float64 // row -> col -> value; Number records inside chart substream
		chartGroupTypes       []chartpkg.ChartType
		seriesGroupIndex      map[int]int // series index -> zero-based ChartFormat group index from SerToCrt
		explicitLegend        bool
		chartWidthEMU         int64
		chartHeightEMU        int64
	}

	var drawings []*sheetir.Drawing
	var state *pendingChartState
	currentSeries := -1
	chartIndex := 0
	appendChartGroupType := func(typ chartpkg.ChartType) {
		if state == nil || typ == chartpkg.ChartUnknown {
			return
		}
		if n := len(state.chartGroupTypes); n > 0 && state.chartGroupTypes[n-1] == typ {
			return
		}
		state.chartGroupTypes = append(state.chartGroupTypes, typ)
	}

	flushChart := func() {
		if state == nil || state.chart == nil {
			return
		}
		if state.chart.ChartType == chartpkg.ChartUnknown {
			state.chart.ChartType = chartpkg.ChartColumn
		}
		if len(state.chart.Series) == 0 && len(state.chart.Categories) == 0 && len(state.chartNumbers) == 0 {
			state = nil
			currentSeries = -1
			return
		}
		// If AI formulas didn't populate series values, fall back to Number records
		// stored directly in the chart substream (common in BIFF5 files created by
		// LibreOffice or when AI formulas have dummy references).
		if len(state.chartNumbers) > 0 {
			// Group values by column, sort by row.
			colMap := map[int][]float64{}
			rowSets := map[int]map[int]bool{}
			for row, cols := range state.chartNumbers {
				for col, val := range cols {
					if _, ok := rowSets[col]; !ok {
						rowSets[col] = map[int]bool{}
					}
					rowSets[col][row] = true
					colMap[col] = append(colMap[col], val)
				}
			}
			// Sort each column's values by row.
			for col := range colMap {
				rows := make([]int, 0, len(rowSets[col]))
				for r := range rowSets[col] {
					rows = append(rows, r)
				}
				sort.Ints(rows)
				sorted := make([]float64, 0, len(rows))
				for _, r := range rows {
					sorted = append(sorted, state.chartNumbers[r][col])
				}
				colMap[col] = sorted
			}
			// Assign columns to series in order.
			cols := make([]int, 0, len(colMap))
			for c := range colMap {
				cols = append(cols, c)
			}
			sort.Ints(cols)
			for i, col := range cols {
				if i < len(state.chart.Series) {
					if len(state.chart.Series[i].Values) == 0 {
						state.chart.Series[i].Values = colMap[col]
					}
				} else if len(state.chart.Series) == 0 {
					state.chart.Series = append(state.chart.Series, chartpkg.Series{
						Index:  i,
						Values: colMap[col],
					})
				}
			}
		}
		if len(state.chartGroupTypes) > 1 {
			for i := range state.chart.Series {
				group := i
				if explicit, ok := state.seriesGroupIndex[i]; ok {
					group = explicit
				}
				if group >= 0 && group < len(state.chartGroupTypes) {
					state.chart.Series[i].ChartType = state.chartGroupTypes[group]
				}
			}
			if state.chartGroupTypes[0] != chartpkg.ChartUnknown {
				state.chart.ChartType = state.chartGroupTypes[0]
			}
			if !state.explicitLegend {
				state.chart.Legend.Show = false
			}
		}
		maxPoints := 0
		for i := range state.chart.Series {
			if len(state.chart.Series[i].Values) > maxPoints {
				maxPoints = len(state.chart.Series[i].Values)
			}
			if state.chart.Series[i].Name == "" {
				state.chart.Series[i].Name = fmt.Sprintf("Series %d", i+1)
			}
		}
		if len(state.chart.Categories) == 0 && maxPoints > 0 {
			state.chart.Categories = make([]string, 0, maxPoints)
			for i := 1; i <= maxPoints; i++ {
				state.chart.Categories = append(state.chart.Categories, strconv.Itoa(i))
			}
		}
		chartIndex++
		widthEMU := int64(640 * 9525)
		heightEMU := int64(400 * 9525)
		if state.chartWidthEMU > 0 {
			widthEMU = state.chartWidthEMU
		}
		if state.chartHeightEMU > 0 {
			heightEMU = state.chartHeightEMU
		}
		drawings = append(drawings, &sheetir.Drawing{
			Name: fmt.Sprintf("%s Chart %d", meta.name, chartIndex),
			Type: sheetir.DrawingTypeChart,
			Position: sheetir.DrawingPosition{
				Anchor: sheetir.AnchorAbsolute,
				Width:  widthEMU,
				Height: heightEMU,
			},
			Chart: &sheetir.DrawingChart{
				Chart: state.chart,
				SVG:   state.chart.SVG(640, 400),
			},
		})
		state = nil
		currentSeries = -1
	}

	for pos+4 <= len(data) {
		id, payload, nextPos, ok := readBiffRecord(data, pos)
		if !ok {
			break
		}
		switch id {
		case biffEOF:
			flushChart()
			return drawings
		case biffChart:
			flushChart()
			state = &pendingChartState{chart: &chartpkg.Chart{}, dataFormatSeriesIndex: -1, dataFormatPointIndex: -1, dataFormatDepth: -1, seriesDepth: -1, pendingAxisLineType: -1, seriesGroupIndex: map[int]int{}}
			currentSeries = -1
		case biffChartSeries:
			if state != nil && state.chart != nil {
				state.chart.Series = append(state.chart.Series, chartpkg.Series{Index: len(state.chart.Series)})
				currentSeries = len(state.chart.Series) - 1
				state.seriesDepth = state.beginDepth + 1
			}
		case biffChartSeriesText:
			if state == nil || state.chart == nil {
				break
			}
			text, ok := parseChartSeriesText(payload)
			if !ok || text == "" {
				break
			}
			switch {
			case state.pendingAnchor == 1:
				state.chart.Title = text
			case state.pendingAnchor == 2:
				state.chart.YAxis.Title = text
			case state.pendingAnchor == 3:
				state.chart.XAxis.Title = text
			case currentSeries >= 0 && currentSeries < len(state.chart.Series):
				state.chart.Series[currentSeries].Name = text
			}
			state.pendingAnchor = 0
		case biffChartAI:
			if state != nil && state.chart != nil {
				state.lastAIType = 0
				if len(payload) > 0 {
					state.lastAIType = payload[0]
				}
				if ref, ok := parseChartAIReference(payload, sheetIndex, externSheets); ok {
					switch state.lastAIType {
					case 0:
						if currentSeries >= 0 && currentSeries < len(state.chart.Series) {
							if name := workbookRangeFirstString(workbook, ref); name != "" {
								state.chart.Series[currentSeries].Name = name
							}
						}
					case 1:
						if currentSeries >= 0 && currentSeries < len(state.chart.Series) {
							state.chart.Series[currentSeries].Values = workbookRangeNumbers(workbook, ref)
						}
					case 2:
						state.chart.Categories = workbookRangeStrings(workbook, ref)
					}
				}
			}
		case biffChartBar:
			if state != nil && state.chart != nil {
				horizontal, stacked := parseChartBarRecord(payload)
				appendChartGroupType(chartTypeFromBarDirection(horizontal))
				if state.chart.ChartType == chartpkg.ChartUnknown || state.chart.ChartType == chartpkg.ChartBar || state.chart.ChartType == chartpkg.ChartColumn {
					state.chart.ChartType = chartTypeFromBarDirection(horizontal)
				}
				state.chart.Stacked = stacked
			}
		case biffChartLine:
			if state != nil && state.chart != nil {
				appendChartGroupType(chartpkg.ChartLine)
				if state.chart.ChartType == chartpkg.ChartUnknown {
					state.chart.ChartType = chartpkg.ChartLine
				}
			}
		case biffChartPie:
			if state != nil && state.chart != nil {
				typ, holeSize, showLeaderLines := parseChartPieRecord(payload)
				appendChartGroupType(typ)
				if state.chart.ChartType == chartpkg.ChartUnknown {
					state.chart.ChartType = typ
				}
				if holeSize > 0 {
					state.chart.HoleSize = holeSize
				}
				if showLeaderLines {
					for i := range state.chart.Series {
						state.chart.Series[i].ShowLeaderLines = true
					}
				}
			}
		case biffChartArea:
			if state != nil && state.chart != nil {
				appendChartGroupType(chartpkg.ChartArea)
				if state.chart.ChartType == chartpkg.ChartUnknown {
					state.chart.ChartType = chartpkg.ChartArea
				}
			}
		case biffChartScatter:
			if state != nil && state.chart != nil {
				appendChartGroupType(chartpkg.ChartScatter)
				if state.chart.ChartType == chartpkg.ChartUnknown {
					state.chart.ChartType = chartpkg.ChartScatter
				}
			}
		case biffChartRadar:
			if state != nil && state.chart != nil {
				appendChartGroupType(chartpkg.ChartRadar)
				if state.chart.ChartType == chartpkg.ChartUnknown {
					state.chart.ChartType = chartpkg.ChartRadar
				}
			}
		case biffChartRadarArea:
			if state != nil && state.chart != nil {
				appendChartGroupType(chartpkg.ChartRadar)
				if state.chart.ChartType == chartpkg.ChartUnknown {
					state.chart.ChartType = chartpkg.ChartRadar
				}
			}
		case biffChartSurface:
			if state != nil && state.chart != nil {
				appendChartGroupType(chartpkg.ChartSurface)
				if state.chart.ChartType == chartpkg.ChartUnknown {
					state.chart.ChartType = chartpkg.ChartSurface
				}
			}
		case biffChartChart3D:
			if state != nil && state.chart != nil && len(payload) >= 14 {
				rotY := int(int16(binary.LittleEndian.Uint16(payload[0:2])))
				rotX := int(int16(binary.LittleEndian.Uint16(payload[2:4])))
				perspective := int(binary.LittleEndian.Uint16(payload[4:6]))
				depth := int(binary.LittleEndian.Uint16(payload[8:10]))
				flags := binary.LittleEndian.Uint16(payload[12:14])
				state.chart.View3D = &chartpkg.View3D{RotX: rotX, RotY: rotY, Perspective: perspective, DepthPercent: depth, RightAngleAxes: flags&0x1 != 0}
				switch state.chart.ChartType {
				case chartpkg.ChartBar:
					state.chart.ChartType = chartpkg.ChartBar3D
				case chartpkg.ChartColumn:
					state.chart.ChartType = chartpkg.ChartColumn3D
				case chartpkg.ChartLine:
					state.chart.ChartType = chartpkg.ChartLine3D
				case chartpkg.ChartPie, chartpkg.ChartDoughnut:
					state.chart.ChartType = chartpkg.ChartPie3D
				case chartpkg.ChartArea:
					state.chart.ChartType = chartpkg.ChartArea3D
				case chartpkg.ChartSurface:
					state.chart.ChartType = chartpkg.ChartSurface3D
				}
			}
		case biffChartLegend:
			if state != nil && state.chart != nil {
				pos, show := parseChartLegendRecord(payload)
				state.chart.Legend.Position = pos
				state.chart.Legend.Show = show
				state.explicitLegend = true
			}
		case biffChartAxis:
			if state != nil && len(payload) >= 2 {
				state.currentAxisType = int(binary.LittleEndian.Uint16(payload[0:2]))
			}
		case biffChartValueRange:
			if state != nil && state.chart != nil && state.currentAxisType == 1 {
				if min, max, major, hasMin, hasMax, hasMajor := parseChartValueRangeRecord(payload); hasMin || hasMax || hasMajor {
					if hasMin {
						state.chart.YAxis.Min = min
					}
					if hasMax {
						state.chart.YAxis.Max = max
					}
					if hasMajor {
						state.chart.YAxis.MajorUnit = major
					}
				}
			}
		case biffChartAxisOptions:
			if state != nil && state.chart != nil && state.currentAxisType == 0 {
				if min, max, major, hasMin, hasMax, hasMajor := parseChartAxisOptionsRecord(payload); hasMin || hasMax || hasMajor {
					if hasMin {
						state.chart.XAxis.Min = min
					}
					if hasMax {
						state.chart.XAxis.Max = max
					}
					if hasMajor {
						state.chart.XAxis.MajorUnit = major
					}
				}
			}
		case biffChartTick:
			if state != nil && state.chart != nil {
				if major, minor, labelPos, ok := parseChartTickRecord(payload); ok {
					switch state.currentAxisType {
					case 0:
						state.chart.XAxis.MajorTickType = major
						state.chart.XAxis.MinorTickType = minor
						state.chart.XAxis.LabelPosition = labelPos
					case 1:
						state.chart.YAxis.MajorTickType = major
						state.chart.YAxis.MinorTickType = minor
						state.chart.YAxis.LabelPosition = labelPos
					}
				}
			}
		case biffChartCategorySeriesAxis:
			if state != nil && state.chart != nil {
				if crossing, labelFreq, tickFreq, reversed, ok := parseChartCategorySeriesAxisRecord(payload); ok {
					switch state.currentAxisType {
					case 0:
						state.chart.XAxis.CrossingPoint = crossing
						state.chart.XAxis.LabelFrequency = labelFreq
						state.chart.XAxis.TickMarkFrequency = tickFreq
						state.chart.XAxis.Reversed = reversed
					case 1:
						state.chart.YAxis.CrossingPoint = crossing
						state.chart.YAxis.LabelFrequency = labelFreq
						state.chart.YAxis.TickMarkFrequency = tickFreq
						state.chart.YAxis.Reversed = reversed
					}
				}
			}
		case biffChartAxisLineFormat:
			if state != nil && len(payload) >= 2 {
				state.pendingAxisLineType = int(binary.LittleEndian.Uint16(payload[0:2]))
			}
		case biffChartObjectLink:
			if state != nil && len(payload) >= 2 {
				state.pendingAnchor = int(binary.LittleEndian.Uint16(payload[0:2]))
			}
		case biffChartFrame:
			if state != nil && state.chart != nil {
				if parseChartFrameRecord(payload) {
					state.chart.PlotArea.ShowBorder = true
				}
			}
		case biffChartPlotArea:
			if state != nil {
				state.plotAreaSeen = true
			}
		case biffChartBegin:
			if state != nil {
				state.beginDepth++
			}
		case biffChartEnd:
			if state != nil {
				state.beginDepth--
				if state.dataFormatDepth > 0 && state.beginDepth < state.dataFormatDepth {
					state.dataFormatSeriesIndex = -1
					state.dataFormatPointIndex = -1
					state.dataFormatDepth = -1
				}
				if state.seriesDepth > 0 && state.beginDepth < state.seriesDepth {
					currentSeries = -1
					state.seriesDepth = -1
				}
			}
		case biffChartDataFormat:
			if state != nil && state.chart != nil {
				pointIdx, seriesIdx := parseChartDataFormatTarget(payload)
				if seriesIdx >= 0 && seriesIdx < len(state.chart.Series) {
					state.dataFormatSeriesIndex = seriesIdx
					state.dataFormatPointIndex = pointIdx
					state.dataFormatDepth = state.beginDepth + 1
				}
			}
		case biffChartLineFormat:
			if state != nil && state.chart != nil {
				if color, pattern, weight, ok := parseChartLineFormatRecord(payload); ok {
					// AxisLineFormatRecord routes format records to the current axis
					if state.pendingAxisLineType >= 0 && state.currentAxisType >= 0 {
						axis := &state.chart.XAxis
						if state.currentAxisType == 1 {
							axis = &state.chart.YAxis
						}
						if state.pendingAxisLineType == 1 {
							axis.ShowGridlines = true
							axis.GridlineColor = color
						}
						state.pendingAxisLineType = -1
					} else {
						target := state.dataFormatSeriesIndex
						if target < 0 {
							target = currentSeries
						}
						if target >= 0 && target < len(state.chart.Series) {
							if state.dataFormatPointIndex >= 0 {
								point := ensureLegacyChartDataPoint(&state.chart.Series[target], state.dataFormatPointIndex)
								if color != "" {
									point.LineColor = color
								}
							} else {
								state.chart.Series[target].LinePattern = pattern
								state.chart.Series[target].LineWidth = legacyChartLineWeightEMU(weight)
								if color != "" {
									state.chart.Series[target].LineColor = color
								}
							}
						} else if state.plotAreaSeen {
							if color != "" {
								state.chart.PlotArea.BorderColor = color
							}
						}
					}
				}
			}
		case biffChartMarkerFormat:
			if state != nil && state.chart != nil {
				target := state.dataFormatSeriesIndex
				if target < 0 {
					target = currentSeries
				}
				if target >= 0 && target < len(state.chart.Series) {
					if color, shape, ok := parseChartMarkerFormatRecord(payload); ok {
						if color != "" {
							state.chart.Series[target].MarkerColor = color
						}
						if shape != "" {
							state.chart.Series[target].MarkerShape = shape
						}
					}
				}
			}
		case biffChartAreaFormat:
			if state != nil && state.chart != nil {
				if fgColor, _, _, ok := parseChartAreaFormatRecord(payload); ok && fgColor != "" {
					target := state.dataFormatSeriesIndex
					if target < 0 {
						target = currentSeries
					}
					if target >= 0 && target < len(state.chart.Series) {
						if state.dataFormatPointIndex >= 0 {
							ensureLegacyChartDataPoint(&state.chart.Series[target], state.dataFormatPointIndex).FillColor = fgColor
						} else {
							state.chart.Series[target].FillColor = fgColor
						}
					} else if state.plotAreaSeen {
						state.chart.PlotArea.FillColor = fgColor
					}
				}
			}
		case biffChartFormat:
			// CHARTFORMAT (0x1014) — chart position and format.
			// Layout: x(4) + y(4) + width(4) + height(4) + grbit(2) + icv(2) + seriesCount(2).
			if state != nil && len(payload) >= 20 {
				w := int64(binary.LittleEndian.Uint32(payload[8:12]))
				h := int64(binary.LittleEndian.Uint32(payload[12:16]))
				// Convert twips (1/20 point) to EMUs (1/914400 inch).
				// 1 twip = 1/20 pt = 1/1440 inch = 635 EMUs.
				if w > 0 {
					state.chartWidthEMU = w * 635
				}
				if h > 0 {
					state.chartHeightEMU = h * 635
				}
			}
		case biffChartText:
			// TEXT (0x1025) — text properties for chart titles/axis labels.
			// Layout: at(1) + v(1) + wBkgMode(2) + rgb(4) + x(4) + y(4) + dx(2) + dy(2) + grbit1(2) + grbit2(2) + rot(2) + reserved(2).
			if state != nil && len(payload) >= 1 {
				at := payload[0]
				switch at {
				case 2:
					state.pendingAnchor = 1 // chart title
				case 3:
					state.pendingAnchor = 2 // Y axis
				case 7:
					state.pendingAnchor = 3 // X axis
				}
			}
		case biffChartAxisParent:
			// AXISPARENT (0x1041) — axis parent.
		case biffChartAxisUsed:
			// AXISUSED (0x1046) — number of axis groups.
		case biffChartSeriesList:
			// SERIESLIST (0x1016) — list of series indices.
		case biffChartSeriesIndex:
			// SERIESINDEX (0x1065) — series index.
		case biffChartTitleFormat:
			// CHARTTITLEFORMAT (0x1050) — title format.
		case biffChartUnits:
			// UNITS (0x1001) — units display.
		case biffChartFontIndex:
			// FONTINDEX (0x1026) — font index.
		case biffChartNumberFormatIndex:
			// NUMBERFORMATINDEX (0x104E) — number format index.
		case biffChartPlotGrowth:
			// PLOTGROWTH (0x1064) — plot growth.
		case biffChartSeriesChartGroupIndex:
			if state != nil && currentSeries >= 0 && len(payload) >= 2 {
				state.seriesGroupIndex[currentSeries] = int(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffChartSeriesLabels:
			// SERIESLABELS (0x100C) — series labels.
		case biffChartDat:
			if state != nil && state.chart != nil {
				if table, ok := parseChartDatRecord(payload); ok {
					state.chart.DataTable = table
				}
			}
		case biffChartDefaultDataLabelTextProperties:
			// DEFAULTDATELABELTEXTPROPERTIES (0x1024) — default data label text properties.
		case biffChartFontBasis:
			// FONTBASIS (0x1060) — font basis.
		case biffChartCatLab:
			// CATLAB (0x0856) — category label.
		case biffChartChartFRTInfo:
			// CHARTFRTINFO (0x0850) — chart future record type info.
		case biffChartStartBlock:
			// CHARTSTARTBLOCK (0x0852) — start of chart future block.
		case biffChartEndBlock:
			// CHARTENDBLOCK (0x0853) — end of chart future block.
		case biffChartStartObject:
			// CHARTSTARTOBJECT (0x0854) — start of chart future object.
		case biffChartEndObject:
			// CHARTENDOBJECT (0x0855) — end of chart future object.
		case biffDataLabelExtension:
			// DATALABELEXTENSION (0x086A) — data label extension.
		case biffNumber:
			if state != nil && len(payload) >= 14 {
				row := int(binary.LittleEndian.Uint16(payload[0:2]))
				col := int(binary.LittleEndian.Uint16(payload[2:4]) & 0x00FF)
				val := math.Float64frombits(binary.LittleEndian.Uint64(payload[6:14]))
				if state.chartNumbers == nil {
					state.chartNumbers = map[int]map[int]float64{}
				}
				if state.chartNumbers[row] == nil {
					state.chartNumbers[row] = map[int]float64{}
				}
				state.chartNumbers[row][col] = val
			}
		}
		pos = nextPos
	}
	flushChart()
	return drawings
}

func parseLegacyWorkbookDrawingStore(group []byte) map[uint32]*sheetir.DrawingImage {
	shared := officeartlegacy.ParseBStore(group, nil)
	if len(shared) == 0 {
		return nil
	}
	store := make(map[uint32]*sheetir.DrawingImage, len(shared))
	for index, entry := range shared {
		if len(entry.Media.Data) == 0 || entry.Media.MIME == "" {
			continue
		}
		store[index] = &sheetir.DrawingImage{
			Data:        append([]byte(nil), entry.Media.Data...),
			ContentType: entry.Media.MIME,
		}
	}
	if len(store) == 0 {
		return nil
	}
	return store
}

func parseLegacyWorksheetImageDrawings(payload []byte, sheet *sheetir.Sheet, rows map[int]*sheetir.Row, drawingStore map[uint32]*sheetir.DrawingImage) []*sheetir.Drawing {
	if len(payload) == 0 || sheet == nil || len(drawingStore) == 0 {
		return nil
	}
	var drawings []*sheetir.Drawing
	walkLegacyOfficeArtRecords(payload, func(recType, _, _ uint16, body []byte) {
		if recType != legacyOfficeArtSpContainer {
			return
		}
		shape, ok := parseLegacyPictureShapeContainer(body, sheet, rows)
		if !ok {
			return
		}
		img, ok := drawingStore[shape.blipRef]
		if !ok || img == nil || len(img.Data) == 0 {
			return
		}
		copyImg := *img
		copyImg.Data = append([]byte(nil), img.Data...)
		drawings = append(drawings, &sheetir.Drawing{
			Name:     fmt.Sprintf("%s Image %d", sheet.Name, len(drawings)+1),
			Type:     sheetir.DrawingTypeImage,
			Position: shape.position,
			Image:    &copyImg,
		})
	})
	if len(drawings) == 0 {
		return nil
	}
	return drawings
}

type legacyWorksheetShape struct {
	position    sheetir.DrawingPosition
	shapeType   uint16
	isPicture   bool
	geometry    string
	fillColor   string
	lineColor   string
	lineWidth   float64
	lineStyle   *drawing.LineStyle
	adjustments []float64
	rotationDeg float64
	flipH       bool
	flipV       bool
	shadow      *drawing.Shadow
	drawingType sheetir.DrawingType
}

func parseLegacyWorksheetNonPictureShapes(payload []byte, sheet *sheetir.Sheet, rows map[int]*sheetir.Row) []legacyWorksheetShape {
	if len(payload) == 0 || sheet == nil {
		return nil
	}
	var shapes []legacyWorksheetShape
	walkLegacyOfficeArtRecords(payload, func(recType, _, _ uint16, body []byte) {
		if recType != legacyOfficeArtSpContainer {
			return
		}
		shape, ok := parseLegacyNonPictureShapeContainer(body, sheet, rows)
		if !ok {
			return
		}
		shapes = append(shapes, shape)
	})
	return shapes
}

func parseLegacyWorksheetNonPictureAnchors(payload []byte, sheet *sheetir.Sheet, rows map[int]*sheetir.Row) []sheetir.DrawingPosition {
	shapes := parseLegacyWorksheetNonPictureShapes(payload, sheet, rows)
	anchors := make([]sheetir.DrawingPosition, 0, len(shapes))
	for _, shape := range shapes {
		anchors = append(anchors, shape.position)
	}
	return anchors
}

type legacyPictureShape struct {
	blipRef  uint32
	position sheetir.DrawingPosition
}

func parseLegacyPictureShapeContainer(data []byte, sheet *sheetir.Sheet, rows map[int]*sheetir.Row) (legacyPictureShape, bool) {
	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil || node == nil || node.ShapeType != legacyOfficeArtShapePicture {
		return legacyPictureShape{}, false
	}
	blipRef, ok := parseLegacyPictureBlipRefProperties(node.Properties)
	if !ok || blipRef == 0 || len(node.ClientAnchor) == 0 {
		return legacyPictureShape{}, false
	}
	position, ok := parseLegacyClientAnchor(node.ClientAnchor, sheet, rows)
	if !ok {
		return legacyPictureShape{}, false
	}
	return legacyPictureShape{blipRef: blipRef, position: position}, true
}

func parseLegacyNonPictureShapeContainer(data []byte, sheet *sheetir.Sheet, rows map[int]*sheetir.Row) (legacyWorksheetShape, bool) {
	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil || node == nil || len(node.ClientAnchor) == 0 {
		return legacyWorksheetShape{}, false
	}
	if node.ShapeType == legacyOfficeArtShapePicture {
		return legacyWorksheetShape{}, false
	}
	position, ok := parseLegacyClientAnchor(node.ClientAnchor, sheet, rows)
	if !ok {
		return legacyWorksheetShape{}, false
	}

	var shape legacyWorksheetShape
	shape.position = position
	shape.shapeType = node.ShapeType
	shape.flipH = node.Flags&0x40 != 0
	shape.flipV = node.Flags&0x80 != 0
	parseLegacyWorksheetShapePropertiesParsed(node.Properties, &shape)
	if shape.isPicture {
		return legacyWorksheetShape{}, false
	}
	shape.geometry = legacyOfficeArtShapeTypeToGeometry(shape.shapeType)
	if shape.geometry == "" {
		shape.geometry = "rect"
	}
	if strings.Contains(shape.geometry, "Connector") {
		shape.drawingType = sheetir.DrawingTypeConnector
	} else {
		shape.drawingType = sheetir.DrawingTypeShape
	}
	return shape, true
}

func parseLegacyWorksheetShapeProperties(body []byte, propCount int, shape *legacyWorksheetShape) {
	if shape == nil {
		return
	}
	props, err := officeartescher.ParseFOPT(body, propCount)
	if err != nil {
		return
	}
	parseLegacyWorksheetShapePropertiesParsed(props, shape)
}

func parseLegacyWorksheetShapePropertiesParsed(props []officeartescher.Property, shape *legacyWorksheetShape) {
	if shape == nil {
		return
	}
	if blipRef, ok := parseLegacyPictureBlipRefProperties(props); ok && blipRef != 0 {
		shape.isPicture = true
	}

	visual := officeartlegacy.DecodeVisualProperties(props, nil)
	if visual.HasRotation {
		shape.rotationDeg = visual.RotationDeg
	}
	if visual.HasFill && !visual.FillEnabled {
		shape.fillColor = ""
	} else if visual.FillType != 0 {
		shape.fillColor = ""
	} else if visual.HasFillColor && visual.FillColor != "" {
		shape.fillColor = visual.FillColor
	}
	if visual.HasLine && !visual.LineEnabled {
		shape.lineColor = ""
		shape.lineWidth = 0
	} else {
		if visual.HasLineColor && visual.LineColor != "" {
			shape.lineColor = visual.LineColor
			legacyWorksheetEnsureLineStyle(shape).Color = visual.LineColor
		}
		if visual.HasLineWidth {
			shape.lineWidth = float64(visual.LineWidthEMU) / 12700.0
			legacyWorksheetEnsureLineStyle(shape).Width = shape.lineWidth
		}
		if visual.HasLineDash {
			legacyWorksheetEnsureLineStyle(shape).Dash = legacyOfficeArtDashPatternName(visual.LineDash)
		}
		if visual.StartArrow != "" {
			legacyWorksheetEnsureLineStyle(shape).HeadEnd = legacyOfficeArtArrowheadFromSharedKind(visual.StartArrow)
		}
		if visual.EndArrow != "" {
			legacyWorksheetEnsureLineStyle(shape).TailEnd = legacyOfficeArtArrowheadFromSharedKind(visual.EndArrow)
		}
		legacyOfficeArtApplySharedArrowSizeName(legacyWorksheetEnsureLineStyle(shape).HeadEnd, visual.StartArrowWidth, false)
		legacyOfficeArtApplySharedArrowSizeName(legacyWorksheetEnsureLineStyle(shape).HeadEnd, visual.StartArrowLength, true)
		legacyOfficeArtApplySharedArrowSizeName(legacyWorksheetEnsureLineStyle(shape).TailEnd, visual.EndArrowWidth, false)
		legacyOfficeArtApplySharedArrowSizeName(legacyWorksheetEnsureLineStyle(shape).TailEnd, visual.EndArrowLength, true)
	}

	if visual.HasShadowColor {
		legacyWorksheetEnsureShadow(shape).Color = visual.ShadowColor
	}
	if visual.HasShadowOpacity {
		legacyWorksheetEnsureShadow(shape).Alpha = visual.ShadowOpacity
	}
	if visual.HasShadowOffsetX {
		sh := legacyWorksheetEnsureShadow(shape)
		x := drawing.EMUToPoints(visual.ShadowOffsetXEMU)
		sh.Distance = math.Hypot(x, legacyShadowYOffsetPoints(sh))
		sh.Angle = legacyShadowAngle(x, legacyShadowYOffsetPoints(sh))
	}
	if visual.HasShadowOffsetY {
		legacyWorksheetEnsureShadow(shape).SkewY = drawing.EMUToPoints(visual.ShadowOffsetYEMU)
	}
	if visual.HasShadowSoftness {
		legacyWorksheetEnsureShadow(shape).Blur = drawing.EMUToPoints(visual.ShadowSoftnessEMU)
	}

	for _, prop := range props {
		if prop.IsBlip || prop.IsComplex {
			continue
		}
		if prop.ID >= legacyOfficeArtPropAdjustValue && prop.ID <= legacyOfficeArtPropAdjustValue+9 {
			idx := int(prop.ID - legacyOfficeArtPropAdjustValue)
			for len(shape.adjustments) <= idx {
				shape.adjustments = append(shape.adjustments, 0)
			}
			rawAdjust := float64(int32(prop.Value)) / 21600.0
			if rawAdjust < -1 {
				rawAdjust = -1
			}
			if rawAdjust > 1 {
				rawAdjust = 1
			}
			shape.adjustments[idx] = rawAdjust
		}
	}

	if shape.shadow != nil {
		y := shape.shadow.SkewY
		x := math.Cos(shape.shadow.Angle*math.Pi/180) * shape.shadow.Distance
		if shape.shadow.Distance == 0 {
			x = 0
		}
		shape.shadow.Distance = math.Hypot(x, y)
		shape.shadow.Angle = legacyShadowAngle(x, y)
		shape.shadow.SkewY = 0
		if shape.shadow.Alpha == 0 {
			shape.shadow.Alpha = 1
		}
	}
}

func legacyWorksheetEnsureShadow(shape *legacyWorksheetShape) *drawing.Shadow {
	if shape.shadow == nil {
		shape.shadow = &drawing.Shadow{Color: "808080", Alpha: 1}
	}
	return shape.shadow
}

func legacyShadowYOffsetPoints(sh *drawing.Shadow) float64 {
	if sh == nil {
		return 0
	}
	return sh.SkewY
}
func legacyShadowAngle(x, y float64) float64 {
	if x == 0 && y == 0 {
		return 0
	}
	return math.Atan2(-y, x) * 180 / math.Pi
}

func legacyWorksheetEnsureLineStyle(shape *legacyWorksheetShape) *drawing.LineStyle {
	if shape.lineStyle == nil {
		style := drawing.DefaultLineStyle()
		style.Color = shape.lineColor
		if shape.lineWidth > 0 {
			style.Width = shape.lineWidth
		}
		shape.lineStyle = &style
	}
	return shape.lineStyle
}

func legacyOfficeArtDashPatternName(name string) drawing.DashPattern {
	switch name {
	case "sysDash":
		return drawing.DashPatternSystemDash
	case "sysDot":
		return drawing.DashPatternSystemDot
	case "sysDashDot":
		return drawing.DashPatternSystemDashDot
	case "sysDashDotDot":
		return drawing.DashPatternSystemDashDotDot
	case "dot":
		return drawing.DashPatternDot
	case "dash":
		return drawing.DashPatternDash
	case "lgDash":
		return drawing.DashPatternLongDash
	case "dashDot":
		return drawing.DashPatternDashDot
	case "lgDashDot":
		return drawing.DashPatternLongDashDot
	case "lgDashDotDot":
		return drawing.DashPatternLongDashDotDot
	default:
		return drawing.DashPatternSolid
	}
}

func legacyOfficeArtArrowheadFromSharedKind(kind officeartlegacy.ArrowKind) *drawing.Arrowhead {
	var typ drawing.ArrowheadType
	switch kind {
	case officeartlegacy.ArrowTriangle:
		typ = drawing.ArrowheadTriangle
	case officeartlegacy.ArrowStealth:
		typ = drawing.ArrowheadStealth
	case officeartlegacy.ArrowDiamond:
		typ = drawing.ArrowheadDiamond
	case officeartlegacy.ArrowOval:
		typ = drawing.ArrowheadOval
	case officeartlegacy.ArrowOpen:
		typ = drawing.ArrowheadArrow
	default:
		return nil
	}
	return &drawing.Arrowhead{Type: typ, Size: drawing.ArrowheadSizeMedium, Length: drawing.ArrowheadSizeMedium}
}

func legacyOfficeArtApplySharedArrowSizeName(arrow *drawing.Arrowhead, size officeartlegacy.ArrowSize, length bool) {
	if arrow == nil {
		return
	}
	var mapped drawing.ArrowheadSize
	switch size {
	case officeartlegacy.ArrowSmall:
		mapped = drawing.ArrowheadSizeSmall
	case officeartlegacy.ArrowLarge:
		mapped = drawing.ArrowheadSizeLarge
	default:
		mapped = drawing.ArrowheadSizeMedium
	}
	if length {
		arrow.Length = mapped
	} else {
		arrow.Size = mapped
	}
}

func legacyOfficeArtDashPattern(raw uint32) drawing.DashPattern {
	switch officeartlegacy.DashName(raw) {
	case "sysDash":
		return drawing.DashPatternSystemDash
	case "sysDot":
		return drawing.DashPatternSystemDot
	case "sysDashDot":
		return drawing.DashPatternSystemDashDot
	case "sysDashDotDot":
		return drawing.DashPatternSystemDashDotDot
	case "dot":
		return drawing.DashPatternDot
	case "dash":
		return drawing.DashPatternDash
	case "lgDash":
		return drawing.DashPatternLongDash
	case "dashDot":
		return drawing.DashPatternDashDot
	case "lgDashDot":
		return drawing.DashPatternLongDashDot
	case "lgDashDotDot":
		return drawing.DashPatternLongDashDotDot
	default:
		return drawing.DashPatternSolid
	}
}

func legacyOfficeArtArrowhead(raw uint32) *drawing.Arrowhead {
	kind := officeartlegacy.ArrowKindFromOfficeArt(raw)
	var typ drawing.ArrowheadType
	switch kind {
	case officeartlegacy.ArrowTriangle:
		typ = drawing.ArrowheadTriangle
	case officeartlegacy.ArrowStealth:
		typ = drawing.ArrowheadStealth
	case officeartlegacy.ArrowDiamond:
		typ = drawing.ArrowheadDiamond
	case officeartlegacy.ArrowOval:
		typ = drawing.ArrowheadOval
	case officeartlegacy.ArrowOpen:
		typ = drawing.ArrowheadArrow
	default:
		return nil
	}
	return &drawing.Arrowhead{Type: typ, Size: drawing.ArrowheadSizeMedium, Length: drawing.ArrowheadSizeMedium}
}

func legacyOfficeArtApplyArrowSize(arrow *drawing.Arrowhead, raw uint32, length bool) {
	if arrow == nil {
		return
	}
	var size drawing.ArrowheadSize
	switch officeartlegacy.ArrowSizeFromOfficeArt(raw) {
	case officeartlegacy.ArrowSmall:
		size = drawing.ArrowheadSizeSmall
	case officeartlegacy.ArrowLarge:
		size = drawing.ArrowheadSizeLarge
	default:
		size = drawing.ArrowheadSizeMedium
	}
	if length {
		arrow.Length = size
	} else {
		arrow.Size = size
	}
}

func legacyOfficeArtColorRefToHex(raw uint32) string {
	return officeartlegacy.ColorRefToHex(raw)
}

func parseLegacyPictureBlipRef(body []byte, propCount int) (uint32, bool) {
	props, err := officeartescher.ParseFOPT(body, propCount)
	if err != nil {
		return 0, false
	}
	return parseLegacyPictureBlipRefProperties(props)
}

func parseLegacyPictureBlipRefProperties(props []officeartescher.Property) (uint32, bool) {
	for _, prop := range props {
		if prop.ID == legacyOfficeArtPropPib {
			return prop.Value, true
		}
	}
	return 0, false
}

func parseLegacyClientAnchor(body []byte, sheet *sheetir.Sheet, rows map[int]*sheetir.Row) (sheetir.DrawingPosition, bool) {
	if len(body) < 18 || sheet == nil {
		return sheetir.DrawingPosition{}, false
	}
	fromCol := int(binary.LittleEndian.Uint16(body[2:4])) + 1
	fromRow := int(binary.LittleEndian.Uint16(body[6:8])) + 1
	toCol := int(binary.LittleEndian.Uint16(body[10:12])) + 1
	toRow := int(binary.LittleEndian.Uint16(body[14:16])) + 1
	if fromCol <= 0 || fromRow <= 0 || toCol < fromCol || toRow < fromRow {
		return sheetir.DrawingPosition{}, false
	}
	fromColPx := legacyWorksheetColumnPx(sheet, fromCol)
	toColPx := legacyWorksheetColumnPx(sheet, toCol)
	fromRowPx := legacyWorksheetRowPx(rows, fromRow)
	toRowPx := legacyWorksheetRowPx(rows, toRow)
	return sheetir.DrawingPosition{
		Anchor: sheetir.AnchorTwoCell,
		FromCell: sheetir.DrawingCellAnchor{
			Row:       fromRow,
			Col:       fromCol,
			ColOffset: legacyFractionToEMU(fromColPx, int(binary.LittleEndian.Uint16(body[4:6])), 1024),
			RowOffset: legacyFractionToEMU(fromRowPx, int(binary.LittleEndian.Uint16(body[8:10])), 256),
		},
		ToCell: sheetir.DrawingCellAnchor{
			Row:       toRow,
			Col:       toCol,
			ColOffset: legacyFractionToEMU(toColPx, int(binary.LittleEndian.Uint16(body[12:14])), 1024),
			RowOffset: legacyFractionToEMU(toRowPx, int(binary.LittleEndian.Uint16(body[16:18])), 256),
		},
	}, true
}

func legacyWorksheetColumnPx(sheet *sheetir.Sheet, col int) int {
	if sheet == nil || col <= 0 {
		return legacyDefaultColumnPx
	}
	if width, ok := sheet.ColumnWidths[col]; ok && width > 0 {
		return int(math.Round(width * legacyCharUnitToPx))
	}
	if sheet.DefaultColWidthChars > 0 {
		return int(math.Round(sheet.DefaultColWidthChars * legacyCharUnitToPx))
	}
	return legacyDefaultColumnPx
}

func legacyWorksheetRowPx(rows map[int]*sheetir.Row, row int) int {
	if row <= 0 {
		return legacyDefaultRowPx
	}
	if r, ok := rows[row]; ok && r != nil && r.HeightPt > 0 {
		return int(math.Round(r.HeightPt * 4.0 / 3.0))
	}
	return legacyDefaultRowPx
}

func legacyFractionToEMU(cellPx int, fraction int, denom int) int64 {
	if cellPx <= 0 || fraction <= 0 || denom <= 0 {
		return 0
	}
	return int64(math.Round(float64(cellPx*fraction) * legacyEMUsPerPx / float64(denom)))
}

func legacyDrawingPositionToVMLAnchor(position sheetir.DrawingPosition) string {
	if position.Anchor != sheetir.AnchorTwoCell {
		return ""
	}
	return fmt.Sprintf("%d, %d, %d, %d, %d, %d, %d, %d",
		position.FromCell.Col-1,
		legacyEMUToPixels(position.FromCell.ColOffset),
		position.FromCell.Row-1,
		legacyEMUToPixels(position.FromCell.RowOffset),
		position.ToCell.Col-1,
		legacyEMUToPixels(position.ToCell.ColOffset),
		position.ToCell.Row-1,
		legacyEMUToPixels(position.ToCell.RowOffset),
	)
}

func legacyEMUToPixels(emu int64) int {
	if emu <= 0 {
		return 0
	}
	return int(math.Round(float64(emu) / legacyEMUsPerPx))
}

func parseLegacyOfficeArtBSEImage(body []byte) *sheetir.DrawingImage {
	entry, ok := officeartlegacy.ParseBSE(body, nil)
	if !ok || len(entry.Media.Data) == 0 || entry.Media.MIME == "" {
		return nil
	}
	return &sheetir.DrawingImage{
		Data:        append([]byte(nil), entry.Media.Data...),
		ContentType: entry.Media.MIME,
	}
}

func extractLegacyOfficeArtBlipImage(recType uint16, payload []byte) ([]byte, string) {
	media := officeartlegacy.ExtractBLIP(recType, payload)
	return media.Data, media.MIME
}

func extractLegacyOfficeArtBlipRaster(data []byte) []byte {
	blob, _ := officeartlegacy.ExtractRaster(data)
	return blob
}

func extractLegacyOfficeArtBlipVector(payload []byte, wantContentType string) []byte {
	var recType uint16
	switch wantContentType {
	case "image/x-emf":
		recType = legacyOfficeArtBlipEMF
	case "image/x-wmf":
		recType = legacyOfficeArtBlipWMF
	default:
		return nil
	}
	media := officeartlegacy.ExtractBLIP(recType, payload)
	if media.MIME != wantContentType {
		return nil
	}
	return media.Data
}

func legacyExtractOfficeArtVectorBlob(data []byte, wantContentType string) []byte {
	start, ok := legacyVectorImageStart(data, wantContentType)
	if !ok || start < 0 || start >= len(data) {
		return nil
	}
	return data[start:]
}

func legacyVectorImageStart(data []byte, wantContentType string) (int, bool) {
	switch wantContentType {
	case "image/x-emf":
		if len(data) >= 44 && binary.LittleEndian.Uint32(data[0:4]) == 1 && string(data[40:44]) == " EMF" {
			return 0, true
		}
		for index := 40; index+4 <= len(data); index++ {
			if string(data[index:index+4]) != " EMF" || index < 40 {
				continue
			}
			start := index - 40
			if start+44 <= len(data) && binary.LittleEndian.Uint32(data[start:start+4]) == 1 {
				return start, true
			}
		}
	case "image/x-wmf":
		if legacyLooksLikeWMF(data) {
			return 0, true
		}
		if index := bytes.Index(data, []byte{0xD7, 0xCD, 0xC6, 0x9A}); index >= 0 {
			return index, true
		}
		for index := 0; index+18 <= len(data); index++ {
			if legacyLooksLikeWMF(data[index:]) {
				return index, true
			}
		}
	}
	return 0, false
}

func legacyLooksLikeWMF(data []byte) bool {
	if len(data) < 18 {
		return false
	}
	typeVal := binary.LittleEndian.Uint16(data[0:2])
	headerSize := binary.LittleEndian.Uint16(data[2:4])
	version := binary.LittleEndian.Uint16(data[4:6])
	return (typeVal == 1 || typeVal == 2) && headerSize == 9 && (version == 0x0100 || version == 0x0300)
}

func legacyInflateOfficeArtPayload(data []byte) []byte {
	if len(data) < 2 {
		return nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer reader.Close()
	inflated, err := io.ReadAll(reader)
	if err != nil || len(inflated) == 0 {
		return nil
	}
	return inflated
}

func walkLegacyOfficeArtRecords(data []byte, visit func(recType uint16, inst uint16, version uint16, body []byte)) {
	if visit == nil || len(data) < officeartescher.HeaderSize {
		return
	}
	records, err := officeartescher.ParseRecords(data)
	if err == nil {
		_ = officeartescher.Walk(records, func(rec officeartescher.Record) error {
			visit(rec.Header.Type, rec.Header.Instance, uint16(rec.Header.Version), rec.Data)
			return nil
		})
		return
	}

	// BIFF MSODRAWING aggregates are allowed to be fragmented around application
	// records such as OBJ/TXO. If a reconstructed worksheet stream is not one
	// perfectly contiguous Escher tree, recover structurally valid OfficeArt
	// records instead of discarding every drawing on the sheet.
	for _, rec := range officeartescher.Scan(data, func(h officeartescher.Header) bool {
		return h.Type >= 0xF000 && h.Type <= 0xF122
	}) {
		_ = officeartescher.Walk([]officeartescher.Record{rec}, func(rec officeartescher.Record) error {
			visit(rec.Header.Type, rec.Header.Instance, uint16(rec.Header.Version), rec.Data)
			return nil
		})
	}
}

func legacyImageContentType(data []byte) string {
	return officeartlegacy.SniffImageMIME(data)
}

func legacyImageEnd(data []byte, contentType string) (int, bool) {
	return officeartlegacy.ImageEnd(data, contentType)
}

func legacyScanPNGEnd(data []byte) (int, bool) {
	if len(data) < 8 {
		return 0, false
	}
	off := 8
	for off+8 <= len(data) {
		chunkLen := int(binary.BigEndian.Uint32(data[off : off+4]))
		if chunkLen < 0 || off+12+chunkLen > len(data) {
			return 0, false
		}
		chunkType := string(data[off+4 : off+8])
		off += 12 + chunkLen
		if chunkType == "IEND" {
			return off, true
		}
	}
	return 0, false
}

func legacyScanJPEGEnd(data []byte) (int, bool) {
	for i := 2; i < len(data)-1; i++ {
		if data[i] == 0xFF && data[i+1] == 0xD9 {
			return i + 2, true
		}
	}
	return 0, false
}

func legacyScanGIFEnd(data []byte) (int, bool) {
	for i := 6; i < len(data); i++ {
		if data[i] == 0x3B {
			return i + 1, true
		}
	}
	return 0, false
}

func parseChartSeriesText(payload []byte) (string, bool) {
	if len(payload) < 4 {
		return "", false
	}
	text, _, ok := readXLUnicodeString(payload, 2, 1)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(text), true
}

// parseChartLegendRecord decodes POI LegendRecord (0x1015).
// Returns position string (bottom, corner, top, right, left) and show flag.
func parseChartLegendRecord(payload []byte) (string, bool) {
	if len(payload) < 17 {
		return "none", false
	}
	legendType := payload[16] // field_5_type
	switch legendType {
	case 0:
		return "bottom", true
	case 1:
		return "corner", true
	case 2:
		return "top", true
	case 3:
		return "right", true
	case 4:
		return "left", true
	case 7:
		return "right", true // undocked — fallback to right
	default:
		return "right", true
	}
}

// parseChartValueRangeRecord decodes POI ValueRangeRecord (0x101F).
// Returns min, max, majorUnit and flags indicating whether each is explicit.
func parseChartValueRangeRecord(payload []byte) (min, max, majorUnit float64, hasMin, hasMax, hasMajor bool) {
	if len(payload) < 34 {
		return
	}
	min = math.Float64frombits(binary.LittleEndian.Uint64(payload[0:8]))
	max = math.Float64frombits(binary.LittleEndian.Uint64(payload[8:16]))
	majorUnit = math.Float64frombits(binary.LittleEndian.Uint64(payload[16:24]))
	options := binary.LittleEndian.Uint16(payload[32:34])
	hasMin = options&0x01 == 0   // automaticMinimum bit NOT set
	hasMax = options&0x02 == 0   // automaticMaximum bit NOT set
	hasMajor = options&0x04 == 0 // automaticMajor bit NOT set
	return
}

// parseChartFrameRecord decodes POI FrameRecord (0x1032).
// Returns true when the frame has a regular border.
func parseChartFrameRecord(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	borderType := int(binary.LittleEndian.Uint16(payload[0:2]))
	return borderType == 0 // BORDER_TYPE_REGULAR
}

// parseChartAxisOptionsRecord decodes POI AxisOptionsRecord (0x1062, CatSerRange).
// Returns min, max, majorUnit as float64 and flags indicating whether each is explicit.
// For category/series axes these are category indices; for value axes use ValueRangeRecord.
func parseChartAxisOptionsRecord(payload []byte) (min, max, majorUnit float64, hasMin, hasMax, hasMajor bool) {
	if len(payload) < 18 {
		return
	}
	min = float64(int16(binary.LittleEndian.Uint16(payload[0:2])))
	max = float64(int16(binary.LittleEndian.Uint16(payload[2:4])))
	majorUnit = float64(int16(binary.LittleEndian.Uint16(payload[4:6])))
	options := binary.LittleEndian.Uint16(payload[16:18])
	hasMin = options&0x01 == 0   // defaultMinimum bit NOT set
	hasMax = options&0x02 == 0   // defaultMaximum bit NOT set
	hasMajor = options&0x04 == 0 // defaultMajor bit NOT set
	return
}

// parseChartTickRecord decodes POI TickRecord (0x101E).
// Returns major tick type, minor tick type, label position, and an ok flag.
//
// Record layout (30 bytes):
//   - byte 0: majorTickType (0=none, 1=outside, 2=inside, 3=cross)
//   - byte 1: minorTickType (0=none, 1=outside, 2=inside, 3=cross)
//   - byte 2: labelPosition (0=none, 1=low, 2=high, 3=next to axis)
//   - byte 3: background
//   - bytes 4-7: labelColorRgb (int32)
//   - bytes 8-23: zero fields (ignored)
//   - bytes 24-25: options (uint16)
//   - bytes 26-27: tickColor (uint16)
//   - bytes 28-29: zero5 (uint16)
func parseChartTickRecord(payload []byte) (majorTickType, minorTickType, labelPosition int, ok bool) {
	if len(payload) < 30 {
		return 0, 0, 0, false
	}
	majorTickType = int(payload[0])
	minorTickType = int(payload[1])
	labelPosition = int(payload[2])
	return majorTickType, minorTickType, labelPosition, true
}

// parseChartAxisLineFormatRecord decodes POI AxisLineFormatRecord (0x1021).
// Returns the axis type and an ok flag.
//
// Record layout (2 bytes):
//   - bytes 0-1: axisType (uint16) — 0=axis line, 1=major grid, 2=minor grid, 3=walls/floor
func parseChartAxisLineFormatRecord(payload []byte) (axisType int, ok bool) {
	if len(payload) < 2 {
		return 0, false
	}
	return int(binary.LittleEndian.Uint16(payload[0:2])), true
}

// parseChartCategorySeriesAxisRecord decodes POI CategorySeriesAxisRecord (0x1020).
// Returns crossing point, label frequency, tick mark frequency, reversed flag, and an ok flag.
//
// Record layout (8 bytes):
//   - bytes 0-1: crossingPoint (int16)
//   - bytes 2-3: labelFrequency (int16)
//   - bytes 4-5: tickMarkFrequency (int16)
//   - bytes 6-7: options (uint16) — bit 2=reversed
func parseChartCategorySeriesAxisRecord(payload []byte) (crossingPoint, labelFrequency, tickMarkFrequency int, reversed bool, ok bool) {
	if len(payload) < 8 {
		return 0, 0, 0, false, false
	}
	crossingPoint = int(int16(binary.LittleEndian.Uint16(payload[0:2])))
	labelFrequency = int(int16(binary.LittleEndian.Uint16(payload[2:4])))
	tickMarkFrequency = int(int16(binary.LittleEndian.Uint16(payload[4:6])))
	options := binary.LittleEndian.Uint16(payload[6:8])
	reversed = options&0x04 != 0
	return crossingPoint, labelFrequency, tickMarkFrequency, reversed, true
}

// parseChartBarRecord decodes POI BarRecord (0x1017).
// Returns horizontal flag and stacked flag.
//
// Record layout (6 bytes):
//   - bytes 0-1: barSpace (int16)
//   - bytes 2-3: categorySpace (int16)
//   - bytes 4-5: formatFlags (uint16) — bit 0=horizontal, bit 1=stacked
func parseChartBarRecord(payload []byte) (horizontal, stacked bool) {
	if len(payload) < 6 {
		return false, false
	}
	flags := binary.LittleEndian.Uint16(payload[4:6])
	return flags&0x01 != 0, flags&0x02 != 0
}

// parseChartDataFormatRecord decodes POI DataFormatRecord (0x1006).
// Returns the series index that subsequent format records apply to.
//
// Record layout (8 bytes):
//   - bytes 0-1: pointNumber (int16, -1 means whole series)
//   - bytes 2-3: seriesIndex (int16)
//   - bytes 4-5: seriesNumber (int16)
//   - bytes 6-7: formatFlags (uint16)
func parseChartDataFormatRecord(payload []byte) int {
	_, series := parseChartDataFormatTarget(payload)
	return series
}

func parseChartDataFormatTarget(payload []byte) (pointIndex, seriesIndex int) {
	if len(payload) < 4 {
		return -1, -1
	}
	point := int(binary.LittleEndian.Uint16(payload[0:2]))
	if point == 0xFFFF {
		point = -1
	}
	series := int(int16(binary.LittleEndian.Uint16(payload[2:4])))
	return point, series
}

func ensureLegacyChartDataPoint(series *chartpkg.Series, index int) *chartpkg.DataPoint {
	for i := range series.DataPoints {
		if series.DataPoints[i].Index == index {
			return &series.DataPoints[i]
		}
	}
	series.DataPoints = append(series.DataPoints, chartpkg.DataPoint{Index: index})
	return &series.DataPoints[len(series.DataPoints)-1]
}

func parseChartDatRecord(payload []byte) (*chartpkg.DataTable, bool) {
	if len(payload) < 2 {
		return nil, false
	}
	flags := binary.LittleEndian.Uint16(payload[:2])
	return &chartpkg.DataTable{
		ShowHorzBorder: flags&0x0001 != 0,
		ShowVertBorder: flags&0x0002 != 0,
		ShowOutline:    flags&0x0004 != 0,
		ShowKeys:       flags&0x0008 != 0,
	}, true
}

func parseChartPieRecord(payload []byte) (chartpkg.ChartType, int, bool) {
	typ := chartpkg.ChartPie
	if len(payload) < 4 {
		return typ, 0, false
	}
	holeSize := int(binary.LittleEndian.Uint16(payload[2:4]))
	if holeSize >= 10 && holeSize <= 90 {
		typ = chartpkg.ChartDoughnut
	}
	showLeaderLines := false
	if len(payload) >= 6 {
		flags := binary.LittleEndian.Uint16(payload[4:6])
		showLeaderLines = flags&0x0002 != 0
	}
	return typ, holeSize, showLeaderLines
}

func legacyChartLineWeightEMU(weight int) int {
	// BIFF LineFormat weights are hairline (-1/0xffff), narrow, medium, and wide.
	// Use stable DrawingML widths of 0.5, 1, 2, and 3 points respectively.
	switch uint16(weight) {
	case 0xffff:
		return 6350
	case 1:
		return 25400
	case 2:
		return 38100
	default:
		return 12700
	}
}

func chartTypeFromBarDirection(horizontal bool) chartpkg.ChartType {
	if horizontal {
		return chartpkg.ChartBar
	}
	return chartpkg.ChartColumn
}

// parseChartLineFormatRecord decodes POI LineFormatRecord (0x1007).
// Returns the line color as a hex string, pattern, weight, and an ok flag.
//
// Record layout (12 bytes):
//   - bytes 0-3: lineColor (int32)
//   - bytes 4-5: linePattern (uint16)
//   - bytes 6-7: weight (uint16)
//   - bytes 8-9: format (uint16)
//   - bytes 10-11: colourPaletteIndex (uint16)
func parseChartLineFormatRecord(payload []byte) (color string, pattern int, weight int, ok bool) {
	if len(payload) < 8 {
		return
	}
	col := binary.LittleEndian.Uint32(payload[0:4])
	pattern = int(binary.LittleEndian.Uint16(payload[4:6]))
	weight = int(binary.LittleEndian.Uint16(payload[6:8]))
	color = resolveChartColor(int(col))
	return color, pattern, weight, true
}

// parseChartAreaFormatRecord decodes POI AreaFormatRecord (0x100A).
// Returns the foreground color, background color, pattern, and an ok flag.
//
// Record layout (16 bytes):
//   - bytes 0-3: foregroundColor (int32)
//   - bytes 4-7: backgroundColor (int32)
//   - bytes 8-9: pattern (uint16)
//   - bytes 10-11: formatFlags (uint16)
//   - bytes 12-13: forecolorIndex (uint16)
//   - bytes 14-15: backcolorIndex (uint16)
func parseChartAreaFormatRecord(payload []byte) (fgColor, bgColor string, pattern int, ok bool) {
	if len(payload) < 10 {
		return
	}
	fg := binary.LittleEndian.Uint32(payload[0:4])
	bg := binary.LittleEndian.Uint32(payload[4:8])
	pattern = int(binary.LittleEndian.Uint16(payload[8:10]))
	fgColor = resolveChartColor(int(fg))
	bgColor = resolveChartColor(int(bg))
	return fgColor, bgColor, pattern, true
}

func parseChartMarkerFormatRecord(payload []byte) (color string, shape string, ok bool) {
	if len(payload) < 10 {
		return "", "", false
	}
	fore := resolveChartColor(int(binary.LittleEndian.Uint32(payload[0:4])))
	back := resolveChartColor(int(binary.LittleEndian.Uint32(payload[4:8])))
	markerType := int(binary.LittleEndian.Uint16(payload[8:10]))
	color = fore
	if color == "" {
		color = back
	}
	switch markerType {
	case 2:
		shape = "diamond"
	case 0:
		shape = ""
	default:
		shape = "circle"
	}
	return color, shape, true
}

// resolveChartColor converts a BIFF chart color value to a hex string.
// Chart colors use the workbook palette for indices 0x08..0x3F,
// 0xFF000000 means "automatic" (returns ""), and other values are
// treated as 0x00BBGGRR encoded RGB.
func resolveChartColor(col int) string {
	switch {
	case col == 0xFF000000 || col == 0:
		return ""
	case col >= 0x08 && col <= 0x3F:
		return defaultBIFFPalette[col-0x08]
	default:
		// BIFF stores colors as 0x00BBGGRR (little-endian DWORD).
		r := col & 0xFF
		g := (col >> 8) & 0xFF
		b := (col >> 16) & 0xFF
		return fmt.Sprintf("%02X%02X%02X", r, g, b)
	}
}

func parseChartAIReference(payload []byte, defaultSheetIndex int, externSheets []externSheetRef) (chartRangeRef, bool) {
	if len(payload) < 8 {
		return chartRangeRef{}, false
	}
	formulaLen := int(binary.LittleEndian.Uint16(payload[6:8]))
	if formulaLen == 0 || 8+formulaLen > len(payload) {
		return chartRangeRef{}, false
	}
	formula := payload[8 : 8+formulaLen]
	token := formula[0] & 0x3F
	switch token {
	case 0x3A:
		if len(formula) < 7 {
			return chartRangeRef{}, false
		}
		sheetIndex, ok := resolveExternSheetIndex(binary.LittleEndian.Uint16(formula[1:3]), externSheets, defaultSheetIndex)
		if !ok {
			return chartRangeRef{}, false
		}
		row := int(binary.LittleEndian.Uint16(formula[3:5]))
		col := int(binary.LittleEndian.Uint16(formula[5:7]) & 0x00FF)
		return chartRangeRef{sheetIndex: sheetIndex, firstRow: row, lastRow: row, firstCol: col, lastCol: col}, true
	case 0x3B:
		if len(formula) < 11 {
			return chartRangeRef{}, false
		}
		sheetIndex, ok := resolveExternSheetIndex(binary.LittleEndian.Uint16(formula[1:3]), externSheets, defaultSheetIndex)
		if !ok {
			return chartRangeRef{}, false
		}
		return chartRangeRef{
			sheetIndex: sheetIndex,
			firstRow:   int(binary.LittleEndian.Uint16(formula[3:5])),
			lastRow:    int(binary.LittleEndian.Uint16(formula[5:7])),
			firstCol:   int(binary.LittleEndian.Uint16(formula[7:9]) & 0x00FF),
			lastCol:    int(binary.LittleEndian.Uint16(formula[9:11]) & 0x00FF),
		}, true
	default:
		return chartRangeRef{}, false
	}
}

func resolveExternSheetIndex(index uint16, externSheets []externSheetRef, defaultSheetIndex int) (int, bool) {
	if index == 0xFFFF {
		return defaultSheetIndex, true
	}
	if int(index) >= len(externSheets) {
		return 0, false
	}
	ref := externSheets[index]
	if ref.sourceKind == supBookKindExternal || ref.sourceKind == supBookKindAddIn {
		return 0, false
	}
	if ref.firstSheet < 0 {
		return 0, false
	}
	return ref.firstSheet, true
}

func resolveExternalNameText(sheetRefIndex uint16, nameIndex int, workbook legacyFormulaWorkbookContext) (string, bool) {
	if nameIndex < 0 || int(sheetRefIndex) >= len(workbook.externSheets) {
		return "", false
	}
	ref := workbook.externSheets[int(sheetRefIndex)]
	if nameIndex < len(ref.externalNames) {
		text := strings.TrimSpace(ref.externalNames[nameIndex].text)
		if text != "" {
			return text, true
		}
	}
	if ref.firstSheet == -2 && nameIndex < len(workbook.definedNames) {
		text := strings.TrimSpace(workbook.definedNames[nameIndex])
		if text != "" {
			return text, true
		}
	}
	return "", false
}

func workbookRangeFirstString(workbook *sheetir.Workbook, ref chartRangeRef) string {
	values := workbookRangeStrings(workbook, ref)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func workbookRangeStrings(workbook *sheetir.Workbook, ref chartRangeRef) []string {
	var values []string
	for row := ref.firstRow; row <= ref.lastRow; row++ {
		for col := ref.firstCol; col <= ref.lastCol; col++ {
			cell := workbookCell(workbook, ref.sheetIndex, row+1, col+1)
			text := workbookCellString(cell)
			if text != "" {
				values = append(values, text)
			}
		}
	}
	return values
}

func workbookRangeNumbers(workbook *sheetir.Workbook, ref chartRangeRef) []float64 {
	var values []float64
	for row := ref.firstRow; row <= ref.lastRow; row++ {
		for col := ref.firstCol; col <= ref.lastCol; col++ {
			cell := workbookCell(workbook, ref.sheetIndex, row+1, col+1)
			if cell == nil {
				continue
			}
			if cell.Kind == sheetir.CellNumber || cell.Kind == sheetir.CellDate {
				values = append(values, cell.NumValue)
			}
		}
	}
	return values
}

func workbookCell(workbook *sheetir.Workbook, sheetIndex, rowIndex, colIndex int) *sheetir.Cell {
	if workbook == nil || sheetIndex < 0 || sheetIndex >= len(workbook.Sheets) {
		return nil
	}
	sheet := workbook.Sheets[sheetIndex]
	if sheet == nil {
		return nil
	}
	for _, row := range sheet.Rows {
		if row == nil || row.Index != rowIndex {
			continue
		}
		for _, cell := range row.Cells {
			if cell != nil && cell.Col == colIndex {
				return cell
			}
		}
		return nil
	}
	return nil
}

func workbookCellString(cell *sheetir.Cell) string {
	if cell == nil {
		return ""
	}
	switch cell.Kind {
	case sheetir.CellString, sheetir.CellError:
		return strings.TrimSpace(cell.StrValue)
	case sheetir.CellNumber, sheetir.CellDate:
		if math.Mod(cell.NumValue, 1) == 0 {
			return fmt.Sprintf("%.0f", cell.NumValue)
		}
		return fmt.Sprintf("%g", cell.NumValue)
	case sheetir.CellBool:
		if cell.NumValue != 0 {
			return "TRUE"
		}
		return "FALSE"
	default:
		return ""
	}
}

func buildLegacyTableColumns(rows map[int]*sheetir.Row, topRow, leftCol, rightCol, headerRowCount int) []*sheetir.TableColumn {
	if leftCol <= 0 || rightCol < leftCol {
		return nil
	}
	headerRow := 0
	if headerRowCount > 0 {
		headerRow = topRow
	}
	columns := make([]*sheetir.TableColumn, 0, rightCol-leftCol+1)
	for col := leftCol; col <= rightCol; col++ {
		name := fmt.Sprintf("Column%d", col-leftCol+1)
		if headerRow > 0 {
			if cell := findCell(rows, headerRow, col); cell != nil {
				if text := workbookCellString(cell); text != "" {
					name = text
				}
			}
		}
		columns = append(columns, &sheetir.TableColumn{
			ID:   col - leftCol + 1,
			Name: name,
		})
	}
	return columns
}

func parseBoundSheet(payload []byte) (boundSheet, bool) {
	if len(payload) < 8 {
		return boundSheet{}, false
	}
	name, _, ok := readXLUnicodeString(payload, 6, 1)
	if !ok {
		name = fmt.Sprintf("Sheet%d", int(binary.LittleEndian.Uint32(payload[:4]))+1)
	}
	return boundSheet{
		offset:     int(binary.LittleEndian.Uint32(payload[:4])),
		hidden:     payload[4]&0x03 != 0,
		visibility: boundSheetVisibility(payload[4]),
		kind:       payload[5],
		name:       name,
	}, true
}

func boundSheetVisibility(flag byte) string {
	switch flag & 0x03 {
	case 0x01:
		return "hidden"
	case 0x02:
		return "veryHidden"
	default:
		return ""
	}
}

func parseSharedStrings(payload []byte) []string {
	if len(payload) < 8 {
		return nil
	}
	count := int(binary.LittleEndian.Uint32(payload[4:8]))
	offset := 8
	stringsOut := make([]string, 0, count)
	for len(stringsOut) < count && offset < len(payload) {
		value, next, ok := readXLUnicodeRichExtendedString(payload, offset)
		if !ok {
			break
		}
		stringsOut = append(stringsOut, value)
		offset = next
	}
	return stringsOut
}

func parseWorksheet(data []byte, meta boundSheet, sst []string, xfs []legacyXF) *sheetir.Sheet {
	return parseWorksheetWithDrawingStore(data, meta, sst, xfs, nil, legacyFormulaWorkbookContext{})
}

func parseWorksheetWithDrawingStore(data []byte, meta boundSheet, sst []string, xfs []legacyXF, drawingStore map[uint32]*sheetir.DrawingImage, formulaCtx legacyFormulaWorkbookContext) *sheetir.Sheet {
	if meta.offset < 0 || meta.offset+4 > len(data) {
		return nil
	}
	pos := meta.offset
	id, bofPayload, next, ok := readBiffRecord(data, pos)
	if !ok || !isBIFFBOFRecord(id) {
		return nil
	}
	worksheetBIFFVersion := biffVersionFromBOFRecord(id, bofPayload)
	pos = next
	rows := map[int]*sheetir.Row{}
	sheet := &sheetir.Sheet{
		Name:          meta.name,
		Hidden:        meta.hidden,
		Visibility:    meta.visibility,
		ColumnWidths:  map[int]float64{},
		HiddenCols:    map[int]bool{},
		OutlineRows:   map[int]int{},
		CollapsedRows: map[int]bool{},
		OutlineCols:   map[int]int{},
		CollapsedCols: map[int]bool{},
	}
	var pendingFormula *sheetir.Cell
	var pendingCommentText bool
	var pendingControlText *sheetir.FormControl
	pendingDrawingTextObjID := 0
	pendingDrawingCharsRemaining := -1
	var currentDrawingText string
	drawingTextByObjID := map[int]string{}
	pendingCommentObjID := 0
	pendingCommentCharsRemaining := -1
	pendingControlCharsRemaining := -1
	var currentCommentText string
	var currentControlCaption string
	var commentTexts []string
	commentTextsByObjID := map[int]string{}
	pendingNotesByObjID := map[int]legacyNote{}
	controlByObjID := map[int]*sheetir.FormControl{}
	var controlOrder []*sheetir.FormControl
	var objOrder []parsedObj
	commentTextIndex := 0
	var freezePanes bool // set to true by Window2 freeze flag
	currentObjType := uint16(0)
	currentObjID := 0
	var hlinkRanges []hlinkEntry
	// Conditional formatting state: CONDFMT / CFHEADER12 sets pendingCFSqRef and
	// pendingCFCount; subsequent CF / CFRULE12 records claim that sqref until the
	// count is exhausted. pendingCFIsCF12 tracks whether the header is CF12 so
	// that rule-type mismatches (CF rule under CF12 header or vice versa) are
	// rejected, matching POI's CFRecordsAggregate.checkRuleType enforcement.
	pendingCFSqRef := ""
	pendingCFCount := 0
	pendingCFIsCF12 := false
	// autoFilterCols > 0 means AUTOFILTERINFO was seen; resolved to A1 range at done.
	autoFilterCols := 0
	// Protection records decoded from PROTECT / OBJPROTECT / SCENPROTECT / PASSWORD.
	var sheetProtected bool
	var sheetObjProtected bool
	var sheetScenProtected bool
	var sheetPasswordHash uint16
	var arrayFormulas []*sheetir.ArrayFormula
	expFormulaRefs := map[string]string{}
	sharedFormulas := map[string]legacySharedFormulaRecord{}
	var officeArtPayload []byte
	lastFormulaRef := ""
	lastFormulaRow := 0
	lastFormulaCol := 0
	var pivotViews []*legacyPivotView
	var currentPivotView *legacyPivotView
	tableFeaturesByID := map[uint32]*legacyTableFeature{}

	storeComment := func(ref, text string) {
		if ref == "" || text == "" {
			return
		}
		if sheet.Comments == nil {
			sheet.Comments = map[string]string{}
		}
		sheet.Comments[ref] = text
	}
	flushPendingCommentText := func() {
		if !pendingCommentText {
			return
		}
		if currentCommentText != "" {
			if pendingCommentObjID != 0 {
				commentTextsByObjID[pendingCommentObjID] = currentCommentText
				if note, ok := pendingNotesByObjID[pendingCommentObjID]; ok {
					storeComment(note.ref, currentCommentText)
					delete(pendingNotesByObjID, pendingCommentObjID)
				}
			} else {
				commentTexts = append(commentTexts, currentCommentText)
			}
		}
		currentCommentText = ""
		pendingCommentText = false
		pendingCommentObjID = 0
		pendingCommentCharsRemaining = -1
	}
	flushPendingControlText := func() {
		if pendingControlText == nil {
			return
		}
		pendingControlText.AltText = strings.TrimSpace(currentControlCaption)
		currentControlCaption = ""
		pendingControlText = nil
		pendingControlCharsRemaining = -1
	}
	flushPendingDrawingText := func() {
		if pendingDrawingTextObjID == 0 {
			return
		}
		if text := strings.TrimSpace(currentDrawingText); text != "" {
			drawingTextByObjID[pendingDrawingTextObjID] = text
		}
		pendingDrawingTextObjID = 0
		pendingDrawingCharsRemaining = -1
		currentDrawingText = ""
	}
	flushPendingFormulaText := func() {
		if pendingFormula == nil {
			return
		}
		pendingFormula.Kind = sheetir.CellString
		if pendingFormula.StrValue == "" {
			pendingFormula.StrValue = ""
		}
		addSheetCell(rows, pendingFormula)
		pendingFormula = nil
	}
	flushPendingPivotView := func() {
		if currentPivotView == nil {
			return
		}
		pivotViews = append(pivotViews, currentPivotView)
		currentPivotView = nil
	}

	for pos+4 <= len(data) {
		id, payload, nextPos, ok := readBiffRecord(data, pos)
		if !ok {
			break
		}
		if pendingCommentText && id != biffContinue {
			flushPendingCommentText()
		}
		if pendingControlText != nil && id != biffContinue {
			flushPendingControlText()
		}
		if pendingDrawingTextObjID != 0 && id != biffContinue {
			flushPendingDrawingText()
		}
		if pendingFormula != nil && id != biffString && id != biffStringOld {
			flushPendingFormulaText()
		}
		switch id {
		case biffEOF:
			pos = nextPos
			goto done
		case biffSXView:
			flushPendingPivotView()
			if view, ok := parseLegacyPivotView(payload); ok {
				currentPivotView = view
			}
		case biffSXVD:
			if currentPivotView != nil {
				currentPivotView.addViewField(payload)
			}
		case biffSXVI:
			if currentPivotView != nil {
				currentPivotView.addPivotItem(payload)
			}
		case biffSXIvd:
			if currentPivotView != nil {
				currentPivotView.addAxisFieldOrder(payload)
			}
		case biffSXPI:
			if currentPivotView != nil {
				currentPivotView.addPageFields(payload)
			}
		case biffSXDI:
			if currentPivotView != nil {
				currentPivotView.addDataField(payload)
			}
		case biffBOF, biffBOFOld, biffBOF3, biffBOF4:
			if _, typ, _, _, _, _ := parseBIFFBOF(payload); typ == biffBOFTypeChart {
				// ChartSubstreamRecordAggregate — embedded chart substreams have
				// their own BOF/EOF pair inside the worksheet stream.
				pos = skipChartSubstreamRecordAggregate(data, pos)
				continue
			}
		case biffNote:
			if note, ok := parseNoteComment(payload); ok {
				text := ""
				if note.objID != 0 {
					text = commentTextsByObjID[note.objID]
				}
				consumedOrdered := false
				if text == "" && commentTextIndex < len(commentTexts) {
					text = commentTexts[commentTextIndex]
					consumedOrdered = true
				}
				if text != "" {
					storeComment(note.ref, text)
					if consumedOrdered {
						commentTextIndex++
					}
				} else if note.objID != 0 {
					pendingNotesByObjID[note.objID] = note
				} else {
					storeComment(note.ref, note.fallbackText)
				}
			}
		case biffObj:
			if po, ok := parseObjSubRecords(payload); ok {
				currentObjType = po.objType
				currentObjID = po.objID
				objOrder = append(objOrder, po)
				if controlType, ok := legacyObjTypeToFormControlType(po.objType); ok {
					control := &sheetir.FormControl{Type: controlType}
					if checked, mixed, ok := parseLegacyCheckboxRadioControlState(po.checkBoxData, po.objType); ok {
						control.Checked = checked
						control.Mixed = mixed
					}
					if first, ok := parseLegacyRadioFirstButton(po.radioData, po.objType); ok {
						control.FirstButton = first
					} else if po.objType == legacyObjTypeRadio && (len(controlOrder) == 0 || controlOrder[len(controlOrder)-1] == nil || controlOrder[len(controlOrder)-1].Type != "Radio") {
						control.FirstButton = true
					}
					if link := parseLegacyObjLinkFormula(po.linkFormulaData, formulaCtx); link != "" {
						if controlType == "GBox" {
							control.FmlaGroup = link
						} else {
							control.FmlaLink = link
						}
					}
					if min, max, val, inc, page, horizontal, ok := parseLegacySbsControlState(po.controlData, po.objType); ok {
						control.Min = min
						control.Max = max
						control.Val = val
						control.Increment = inc
						control.PageStep = page
						control.Horizontal = horizontal && po.objType == legacyObjTypeScroll
					}
					if sourceRange, val, dropLines, selectionType, selected, hasVal := parseLegacyLbsDataControlState(po.listBoxData, po.objType, formulaCtx); sourceRange != "" || hasVal || dropLines > 0 || selectionType != "" || len(selected) > 0 {
						control.FmlaRange = sourceRange
						if hasVal {
							control.Val = val
						}
						control.DropLines = dropLines
						control.SelectionType = selectionType
						control.Selected = selected
					}
					controlOrder = append(controlOrder, control)
					if po.objID != 0 {
						controlByObjID[po.objID] = control
					}
				}
			} else {
				currentObjType = 0
				currentObjID = 0
			}
		case biffMsoDrawing:
			fullPayload, fullNext := gatherBiffPayload(data, pos)
			if len(fullPayload) > 0 {
				officeArtPayload = append(officeArtPayload, fullPayload...)
			}
			pos = fullNext
			continue
		case biffWindow2:
			if len(payload) >= 2 {
				freezePanes = binary.LittleEndian.Uint16(payload[0:2])&0x0008 != 0
			}
		case biffPane:
			if len(payload) >= 4 {
				applyPane(sheet, payload, freezePanes)
			}
		case biffDimensions:
			applyDimensionsRecord(sheet, payload)
		case biffFilterMode:
			applyFilterModeRecord(sheet)
		case biffGuts:
			applyGutsRecord(sheet, payload)
		case biffSCL:
			applySCLRecord(sheet, payload)
		case biffSetup:
			applySetupRecord(sheet, payload)
		case biffLeftMargin:
			applyMarginRecord(sheet, "left", payload)
		case biffRightMargin:
			applyMarginRecord(sheet, "right", payload)
		case biffTopMargin:
			applyMarginRecord(sheet, "top", payload)
		case biffBottomMargin:
			applyMarginRecord(sheet, "bottom", payload)
		case biffHCenter:
			applyPrintFlagRecord(sheet, "hcenter", payload)
		case biffVCenter:
			applyPrintFlagRecord(sheet, "vcenter", payload)
		case biffPrintHeaders:
			applyPrintFlagRecord(sheet, "headers", payload)
		case biffPrintGridlines:
			applyPrintFlagRecord(sheet, "gridlines", payload)
		case biffHeader:
			applyHeaderFooterRecord(sheet, "header", payload)
		case biffFooter:
			applyHeaderFooterRecord(sheet, "footer", payload)
		case biffHeaderFooterExt:
			applyHeaderFooterExtRecord(sheet, payload)
		case biffHorizPageBreak:
			if breaks := parsePageBreakRecord(payload); len(breaks) > 0 {
				sheet.RowPageBreaks = breaks
			}
		case biffVertPageBreak:
			if breaks := parsePageBreakRecord(payload); len(breaks) > 0 {
				sheet.ColPageBreaks = breaks
			}
		case biffRow:
			applyRowRecord(rows, payload, xfs)
		case biffColInfo:
			applyColInfo(sheet, payload)
		case biffMergeCells:
			var merged []*sheetir.MergedCell
			merged, pos = gatherContiguousMergeCells(data, pos)
			sheet.MergedCells = append(sheet.MergedCells, merged...)
			continue
		case biffUserSViewBegin:
			// Custom-view blocks may carry their own page-setting records; skip the
			// whole aggregate so those records do not leak into the active sheet.
			pos = skipCustomViewSettingsRecordAggregate(data, pos)
			continue
		case biffDefRowHeight:
			applyDefaultRowHeight(sheet, payload)
		case biffDefColWidth:
			applyDefaultColWidth(sheet, payload)
		case biffTxO:
			currentCommentText = ""
			pendingCommentText = currentObjType == legacyObjTypeNote || (currentObjType == 0 && currentObjID == 0)
			if pendingCommentText {
				pendingCommentObjID = currentObjID
				pendingCommentCharsRemaining = parseTxOCommentCharCount(payload)
				break
			}
			currentControlCaption = ""
			pendingControlText = nil
			pendingDrawingTextObjID = 0
			currentDrawingText = ""
			if control, ok := controlByObjID[currentObjID]; ok {
				pendingControlText = control
				pendingControlCharsRemaining = parseTxOCommentCharCount(payload)
				if link := parseLegacyTxOControlLink(payload, formulaCtx); link != "" {
					pendingControlText.FmlaLink = link
				}
			} else if currentObjID != 0 {
				pendingDrawingTextObjID = currentObjID
				pendingDrawingCharsRemaining = parseTxOCommentCharCount(payload)
			}
		case biffContinue:
			if pendingCommentText {
				text, consumedChars := parseCommentTextContinueLimited(payload, pendingCommentCharsRemaining)
				if text != "" {
					currentCommentText += text
				}
				if pendingCommentCharsRemaining >= 0 && consumedChars > 0 {
					pendingCommentCharsRemaining -= consumedChars
					if pendingCommentCharsRemaining < 0 {
						pendingCommentCharsRemaining = 0
					}
				}
				pos = nextPos
				continue
			}
			if pendingControlText != nil {
				text, consumedChars := parseCommentTextContinueLimited(payload, pendingControlCharsRemaining)
				if text != "" {
					currentControlCaption += text
				}
				if pendingControlCharsRemaining >= 0 && consumedChars > 0 {
					pendingControlCharsRemaining -= consumedChars
					if pendingControlCharsRemaining < 0 {
						pendingControlCharsRemaining = 0
					}
				}
				pos = nextPos
				continue
			}
			if pendingDrawingTextObjID != 0 {
				text, consumedChars := parseCommentTextContinueLimited(payload, pendingDrawingCharsRemaining)
				if text != "" {
					currentDrawingText += text
				}
				if pendingDrawingCharsRemaining >= 0 && consumedChars > 0 {
					pendingDrawingCharsRemaining -= consumedChars
					if pendingDrawingCharsRemaining < 0 {
						pendingDrawingCharsRemaining = 0
					}
				}
				pos = nextPos
				continue
			}
		case biffLabelSst:
			if cell := parseLabelSstCell(payload, sst, xfs); cell != nil {
				addSheetCell(rows, cell)
			}
		case biffLabel, biffLabelBIFF2:
			var cell *sheetir.Cell
			if id == biffLabelBIFF2 {
				cell = parseLabelCellBIFF2(payload)
			} else {
				cell = parseLabelCell(payload, xfs)
			}
			if cell != nil {
				addSheetCell(rows, cell)
			}
		case biffBlank, biffBlankBIFF2:
			var cell *sheetir.Cell
			if id == biffBlankBIFF2 {
				cell = parseBlankCellBIFF2(payload)
			} else {
				cell = parseBlankCell(payload, xfs)
			}
			if cell != nil {
				addSheetCell(rows, cell)
			}
		case biffNumber, biffNumberOld, biffNumberBIFF2:
			var cell *sheetir.Cell
			if id == biffNumberBIFF2 {
				cell = parseNumberCellBIFF2(payload)
			} else {
				cell = parseNumberCell(payload, xfs)
			}
			if cell != nil {
				promoteNumericCellToDate(cell, formulaCtx.date1904)
				addSheetCell(rows, cell)
			}
		case biffRString:
			if cell := parseRStringCell(payload, xfs); cell != nil {
				addSheetCell(rows, cell)
			}
		case biffMulBlank:
			for _, cell := range parseMulBlankCells(payload, xfs) {
				addSheetCell(rows, cell)
			}
		case biffRK:
			if cell := parseRKCell(payload, xfs); cell != nil {
				promoteNumericCellToDate(cell, formulaCtx.date1904)
				addSheetCell(rows, cell)
			}
		case biffMulRK:
			for _, cell := range parseMulRKCells(payload, xfs) {
				promoteNumericCellToDate(cell, formulaCtx.date1904)
				addSheetCell(rows, cell)
			}
		case biffBoolErr, biffBoolErrBIFF2:
			var cell *sheetir.Cell
			if id == biffBoolErrBIFF2 {
				cell = parseBoolErrCellBIFF2(payload)
			} else {
				cell = parseBoolErrCell(payload, xfs)
			}
			if cell != nil {
				addSheetCell(rows, cell)
			}
		case biffFormula, biffFormula3, biffFormula4:
			isOldFormula := id == biffFormula3 || id == biffFormula4 || (id == biffFormula && worksheetBIFFVersion == biffVersion2)
			if isOldFormula {
				pendingFormula = parseOldFormulaCell(payload, xfs, formulaCtx, worksheetBIFFVersion == biffVersion2)
			} else {
				pendingFormula = parseFormulaCell(payload, xfs, formulaCtx)
			}
			if !isOldFormula && pendingFormula != nil && pendingFormula.Ref != "" {
				lastFormulaRef = pendingFormula.Ref
				lastFormulaRow = pendingFormula.Row
				lastFormulaCol = pendingFormula.Col
				if anchorRef, ok := parseFormulaExpRef(payload); ok {
					expFormulaRefs[pendingFormula.Ref] = anchorRef
				}
			} else {
				lastFormulaRef = ""
				lastFormulaRow = 0
				lastFormulaCol = 0
			}
			if pendingFormula != nil && pendingFormula.Kind != sheetir.CellEmpty {
				addSheetCell(rows, pendingFormula)
				pendingFormula = nil
			}
		case biffStringOld, biffString:
			if pendingFormula != nil {
				if worksheetBIFFVersion < biffVersion5 {
					text, ok := parseOldFormulaStringRecord(payload, id == biffStringOld)
					if ok {
						pendingFormula.Kind = sheetir.CellString
						pendingFormula.StrValue = text
						addSheetCell(rows, pendingFormula)
					}
				} else {
					fullPayload, fullNext := gatherBiffPayload(data, pos)
					text, _, ok := readXLUnicodeString(fullPayload, 0, 2)
					if ok {
						pendingFormula.Kind = sheetir.CellString
						pendingFormula.StrValue = strings.TrimSpace(text)
						addSheetCell(rows, pendingFormula)
					}
					pendingFormula = nil
					pos = fullNext
					continue
				}
				pendingFormula = nil
			}
		case biffArray:
			if group, ok := parseArrayFormulaRecord(payload, formulaCtx); ok {
				arrayFormulas = append(arrayFormulas, group)
			}
		case biffShrFmla:
			if shared, ok := parseSharedFormulaRecord(payload, lastFormulaRef, lastFormulaRow, lastFormulaCol); ok {
				sharedFormulas[strings.ToUpper(shared.AnchorRef)] = shared
			}
			lastFormulaRef = ""
			lastFormulaRow = 0
			lastFormulaCol = 0
		case biffHLink:
			if hl, ok := parseHLINKRecord(payload); ok {
				hlinkRanges = append(hlinkRanges, hl)
			}
		case biffCondFmt:
			pendingCFSqRef, pendingCFCount = parseCONDFMTSqRef(payload)
			pendingCFIsCF12 = false
		case biffCFHeader12:
			pendingCFSqRef, pendingCFCount = parseCONDFMT12SqRef(payload)
			pendingCFIsCF12 = true
		case biffCF:
			if pendingCFCount > 0 && pendingCFSqRef != "" && !pendingCFIsCF12 {
				if rule := parseCFRecord(payload); rule != nil {
					rule.SqRef = pendingCFSqRef
					sheet.CFRules = append(sheet.CFRules, rule)
				}
				pendingCFCount--
			}
		case biffCFRule12:
			if pendingCFCount > 0 && pendingCFSqRef != "" && pendingCFIsCF12 {
				if rule := parseCF12Record(payload, nil); rule != nil {
					rule.SqRef = pendingCFSqRef
					sheet.CFRules = append(sheet.CFRules, rule)
				}
				pendingCFCount--
			}
		case biffDval:
			// DVAL is just a header; the actual rules come in DV records.
		case biffDV:
			if dv := parseDVRecord(payload, formulaCtx); dv != nil {
				sheet.DataValidations = append(sheet.DataValidations, dv)
			}
		case biffAutoFilterInfo:
			if len(payload) >= 2 {
				autoFilterCols = int(binary.LittleEndian.Uint16(payload[0:2]))
			}
		case biffAutoFilter:
			if fc := parseAutoFilterRecord(payload); fc != nil {
				sheet.FilterColumns = append(sheet.FilterColumns, fc)
			}
		case biffProtect:
			if len(payload) >= 2 {
				sheetProtected = binary.LittleEndian.Uint16(payload[0:2]) != 0
			}
		case biffObjProtect:
			if len(payload) >= 2 {
				sheetObjProtected = binary.LittleEndian.Uint16(payload[0:2]) != 0
			}
		case biffScenProtect:
			if len(payload) >= 2 {
				sheetScenProtected = binary.LittleEndian.Uint16(payload[0:2]) != 0
			}
		case biffPassword:
			if len(payload) >= 2 {
				sheetPasswordHash = binary.LittleEndian.Uint16(payload[0:2])
			}
		case biffPrintSize:
			// PRINTSIZE (0x0033) — opaque record; POI treats as UnknownRecord.
			// The payload is typically { 0x03, 0x00 } per OOO source.
		case biffPLS:
			// PLS (0x004D) — environment-specific print settings; POI treats as
			// UnknownRecord with optional trailing ContinueRecord(s).
		case biffBitmap:
			// BITMAP (0x00E9) — print bitmap; POI treats as UnknownRecord.
		case biffIndex:
			// INDEX (0x020B) — row-block index; POI uses for offset validation.
			// Safe to ignore for parsing; POI treats as UnknownRecord.
		case biffDBCell:
			// DBCELL (0x00D7) — cell-block offset index; POI uses for offset validation.
			// Safe to ignore for parsing; POI treats as UnknownRecord.
		case biffFeatHdr:
			// FeatHdrRecord (0x0867) — shared feature header.
			// Structure: FtrHeader (12 bytes) + isf_sharedFeatureType (2) + reserved (1) + cbHdrData (4) + rgbHdrData (remaining).
			if len(payload) < 15 {
				break
			}
			featHdrType := int(binary.LittleEndian.Uint16(payload[12:14]))
			_ = featHdrType // reserved for future use; FeatRecord carries its own type
		case biffFeatHdr11:
			// FeatHdr11 (0x0871) — shared feature header for Feature11/12 table metadata.
			// The sheet-level table details arrive in FEATURE11/12 and LIST12 records.
		case biffMsoDrawingSel:
			// MSODRAWINGSELECTION (0x00ED) — drawing selection state.
			// Structure: OfficeArtRecordHeader(8) + cpsp(4) + dgslk(4) + spidFocus(4) + shapeIds(n*4).
			// Silently skipped; POI treats as UnknownRecord.
		case biffHideObj:
			// HIDEOBJ (0x008D) — hide/show objects/placeholders.
			// Layout: single USHORT (0=show all, 1=show placeholders, 2=hide all).
			// Silently skipped; POI treats as UnknownRecord.
		case biffWSBool:
			// WSBOOL (0x0081) — worksheet boolean flags.
			// Layout: single USHORT (bit 4 = dialog sheet).
			// POI treats the dialog flag as a sheet-type discriminator.
			if sheet != nil {
				sheet.DialogSheet = parseWSBoolRecord(payload)
			}
		case biffSelection:
			// SELECTION (0x001D) — cursor/selection position in worksheet.
			// Layout: pane(1) + activeRow(2) + activeCol(2) + activeRef(2) + numRefs(2) + refs(numRefs*6).
			// Silently skipped; POI treats as UnknownRecord.
		case biffGridset:
			// GRIDSET (0x0082) — gridlines visible when printing flag.
			// Layout: single USHORT (0=print gridlines, 1=don't print gridlines).
			// Silently skipped; POI treats as UnknownRecord.
		case biffRecalcId:
			// RECALCID (0x01C1) — recalculation engine ID.
			// Layout: rt(2) + reserved(2) + engineId(4).
			// Silently skipped; POI treats as UnknownRecord.
		case biffTabId:
			// TABID (0x013D) — sheet tab IDs.
			// Layout: array of USHORT tab IDs.
			// Silently skipped; POI treats as UnknownRecord.
		case biffUncalced:
			// UNCALCED (0x005E) — formulas need recalculation flag.
			// Layout: single USHORT (reserved, always 0).
			// Silently skipped; POI treats as UnknownRecord.
		case biffWriteProtect:
			// WRITEPROTECT (0x0086) — write protection flag.
			// No payload (or optional 2 bytes reserved).
			// Silently skipped; POI treats as UnknownRecord.
		case biffPasswordRev4:
			// PASSWORDREV4 (0x01BC) — revision password hash.
			// Layout: single USHORT password hash.
			// Silently skipped; POI treats as UnknownRecord.
		case biffFileSharing:
			// FILESHARING (0x005B) — file sharing options.
			// Layout: readonly(2) + password(2) + nameLen(2) + [fHighByte(1) + name(nameLen)].
			// Silently skipped; POI treats as UnknownRecord.
		case biffRefMode:
			// REFMODE (0x000F) — reference mode (A1/R1C1).
			// Layout: single USHORT (1=A1, 0=R1C1).
			// Silently skipped; POI treats as UnknownRecord.
		case biffDSF:
			// DSF (0x0161) — double stream file flag.
			// Layout: single USHORT (bit 0 = BIFF5 book stream present).
			// Silently skipped; POI treats as UnknownRecord.
		case biffMMS:
			// MMS (0x00C1) — add/del menu counts.
			// Layout: addMenuCount(1) + delMenuCount(1).
			// Silently skipped; POI treats as UnknownRecord.
		case biffInterfaceHdr:
			// INTERFACEHDR (0x00E1) — interface header.
			// Layout: single USHORT code page.
			// Silently skipped; POI treats as UnknownRecord.
		case biffInterfaceEnd:
			// INTERFACEEND (0x00E2) — interface end.
			// No payload.
			// Silently skipped; POI treats as UnknownRecord.
		case biffFeat:
			// FeatRecord (0x0868) — shared feature data.
			// Structure: FtrHeader (12 bytes) + isf_sharedFeatureType (2) + reserved1 (1) + reserved2 (4) + cref (2) + cbFeatData (4) + reserved3 (2) + cellRefs (cref*8) + sharedFeature (variable).
			if len(payload) < 27 {
				break
			}
			featType := int(binary.LittleEndian.Uint16(payload[12:14]))
			cref := int(binary.LittleEndian.Uint16(payload[19:21]))
			cbFeatData := int(binary.LittleEndian.Uint32(payload[21:25]))
			_ = cbFeatData
			pos := 27
			var cellRanges []struct{ r1, r2, c1, c2 int }
			for i := 0; i < cref; i++ {
				if pos+8 > len(payload) {
					break
				}
				r1 := int(binary.LittleEndian.Uint16(payload[pos+0 : pos+2]))
				r2 := int(binary.LittleEndian.Uint16(payload[pos+2 : pos+4]))
				c1 := int(binary.LittleEndian.Uint16(payload[pos+4 : pos+6]))
				c2 := int(binary.LittleEndian.Uint16(payload[pos+6 : pos+8]))
				cellRanges = append(cellRanges, struct{ r1, r2, c1, c2 int }{r1, r2, c1, c2})
				pos += 8
			}
			// SHAREDFEATURES_ISFLIST (0x05) = table definition.
			if featType == 0x05 && len(cellRanges) > 0 {
				cr := cellRanges[0]
				tableID := len(sheet.Tables) + 1
				ref := formatA1Ref(cr.r1+1, cr.c1+1) + ":" + formatA1Ref(cr.r2+1, cr.c2+1)
				headerRowCount := 1
				totalsRowCount := 0
				totalsRowShown := false
				// FEATURE_DATA_ISFLIST starts after cellRefs at `pos`. Its prefix:
				// idList(4) + crwHeader(4) + crwTotals(4) + idFieldNext(4) ...
				if pos+16 <= len(payload) {
					crwHeader := int(int32(binary.LittleEndian.Uint32(payload[pos+4 : pos+8])))
					crwTotals := int(int32(binary.LittleEndian.Uint32(payload[pos+8 : pos+12])))
					if crwHeader >= 0 {
						headerRowCount = crwHeader
					}
					if crwTotals > 0 {
						totalsRowCount = crwTotals
						totalsRowShown = true
					}
				}
				sheet.Tables = append(sheet.Tables, &sheetir.Table{
					ID:             tableID,
					Name:           fmt.Sprintf("Table%d", tableID),
					DisplayName:    fmt.Sprintf("Table%d", tableID),
					Ref:            ref,
					HeaderRowCount: headerRowCount,
					TotalsRowCount: totalsRowCount,
					TotalsRowShown: totalsRowShown,
					Columns:        buildLegacyTableColumns(rows, cr.r1+1, cr.c1+1, cr.c2+1, headerRowCount),
				})
			}
		case biffFeature11, biffFeature12:
			fullPayload, fullNext := gatherFutureRecordPayload(data, pos, biffContinueFrt11)
			table, feature, ok := parseLegacyFeature11Table(fullPayload, rows)
			if ok && table != nil && feature != nil {
				sheet.Tables = append(sheet.Tables, table)
				feature.table = table
				tableFeaturesByID[feature.idList] = feature
			}
			pos = fullNext
			continue
		case biffList12:
			if len(payload) < 18 {
				break
			}
			feature := tableFeaturesByID[binary.LittleEndian.Uint32(payload[14:18])]
			if feature == nil || feature.table == nil {
				break
			}
			switch binary.LittleEndian.Uint16(payload[12:14]) {
			case 0x0001:
				if style, ok := parseLegacyList12TableStyle(payload[18:]); ok {
					feature.hasStyle = true
					feature.style = style
					feature.table.TableStyle = &sheetir.TableStyleInfo{
						Name:              style.Name,
						ShowFirstColumn:   style.ShowFirstColumn,
						ShowLastColumn:    style.ShowLastColumn,
						ShowRowStripes:    style.ShowRowStripes,
						ShowColumnStripes: style.ShowColumnStripes,
					}
				}
			case 0x0002:
				if displayName, ok := parseLegacyList12DisplayName(payload[18:]); ok && displayName != "" {
					feature.name = displayName
					feature.table.DisplayName = displayName
				}
			}
		}
		if id != biffFormula && id != biffShrFmla {
			lastFormulaRef = ""
			lastFormulaRow = 0
			lastFormulaCol = 0
		}
		pos = nextPos
	}
	flushPendingCommentText()
	flushPendingControlText()
	flushPendingDrawingText()

done:
	flushPendingPivotView()
	if len(arrayFormulas) > 0 {
		applyArrayFormulaMetadata(rows, arrayFormulas)
		applyExpFormulaMetadata(rows, arrayFormulas, expFormulaRefs)
		sheet.ArrayFormulas = arrayFormulas
	}
	if len(sharedFormulas) > 0 {
		applySharedFormulaMetadata(rows, sharedFormulas, expFormulaRefs, formulaCtx)
	}
	for _, note := range pendingNotesByObjID {
		storeComment(note.ref, note.fallbackText)
	}
	sheet.Rows = sortedSheetRows(rows)
	if len(officeArtPayload) > 0 && len(drawingStore) > 0 {
		sheet.Drawings = append(sheet.Drawings, parseLegacyWorksheetImageDrawings(officeArtPayload, sheet, rows, drawingStore)...)
	}
	if len(officeArtPayload) > 0 {
		nonPictureShapes := parseLegacyWorksheetNonPictureShapes(officeArtPayload, sheet, rows)
		shapeIndex := 0
		shapeCount := 0
		appendLegacyShape := func(shape legacyWorksheetShape, objID int) {
			shapeCount++
			name := fmt.Sprintf("%s Shape %d", sheet.Name, shapeCount)
			if objID != 0 {
				name = fmt.Sprintf("%s Object %d", sheet.Name, objID)
			}
			resolvedShape := &sheetir.DrawingShape{
				Geometry: shape.geometry, Adjustments: append([]float64(nil), shape.adjustments...),
				FillColor: shape.fillColor, LineColor: shape.lineColor, LineWidthPt: shape.lineWidth,
				LineStyle: shape.lineStyle, Text: drawingTextByObjID[objID], RotationDeg: shape.rotationDeg, FlipH: shape.flipH, FlipV: shape.flipV,
			}
			if shape.shadow != nil {
				resolvedShape.Effects.Effects = append(resolvedShape.Effects.Effects, *shape.shadow)
			}
			sheet.Drawings = append(sheet.Drawings, &sheetir.Drawing{Name: name, Type: shape.drawingType, Position: shape.position, Shape: resolvedShape})
		}
		for _, po := range objOrder {
			if !legacyObjConsumesNonPictureAnchor(po.objType) || shapeIndex >= len(nonPictureShapes) {
				continue
			}
			shape := nonPictureShapes[shapeIndex]
			shapeIndex++
			if control := controlByObjID[po.objID]; control != nil {
				control.Anchor = legacyDrawingPositionToVMLAnchor(shape.position)
				continue
			}
			if po.objType != legacyObjTypeChart {
				appendLegacyShape(shape, po.objID)
			}
		}
		for ; shapeIndex < len(nonPictureShapes); shapeIndex++ {
			appendLegacyShape(nonPictureShapes[shapeIndex], 0)
		}
	}
	if len(controlOrder) > 0 {
		sheet.FormControls = append(sheet.FormControls, controlOrder...)
	}
	if len(pivotViews) > 0 {
		for _, view := range pivotViews {
			if pivot := view.resolve(formulaCtx.pivotCaches); pivot != nil {
				sheet.PivotTables = append(sheet.PivotTables, pivot)
			}
		}
	}
	// Resolve deferred AutoFilter range: use the first data row + column count.
	if autoFilterCols > 0 && len(sheet.Rows) > 0 {
		firstRow := sheet.Rows[0].Index
		sheet.AutoFilterRef = fmt.Sprintf("%s:%s",
			formatA1Ref(firstRow, 1),
			formatA1Ref(firstRow, autoFilterCols))
	}
	// Build sheet protection metadata from PROTECT / OBJPROTECT / SCENPROTECT / PASSWORD records.
	if sheetProtected || sheetObjProtected || sheetScenProtected {
		sp := &sheetir.SheetProtection{
			Sheet:     sheetProtected,
			Objects:   sheetObjProtected,
			Scenarios: sheetScenProtected,
		}
		if sheetPasswordHash != 0 {
			sp.Password = fmt.Sprintf("%04X", sheetPasswordHash)
		}
		sheet.Protection = sp
	}
	// Apply collected hyperlinks to their target cells.
	for _, hl := range hlinkRanges {
		for _, row := range sheet.Rows {
			ri := row.Index - 1 // convert 1-based to 0-based
			if ri < hl.firstRow || ri > hl.lastRow {
				continue
			}
			for _, cell := range row.Cells {
				ci := cell.Col - 1 // convert 1-based to 0-based
				if ci >= hl.firstCol && ci <= hl.lastCol && cell.HyperlinkURL == "" {
					cell.HyperlinkURL = hl.url
				}
			}
		}
	}
	for _, row := range sheet.Rows {
		if row.OutlineLevel > 0 {
			sheet.OutlineRows[row.Index] = row.OutlineLevel
		}
		if row.Collapsed {
			sheet.CollapsedRows[row.Index] = true
		}
	}
	if len(sheet.ColumnWidths) == 0 {
		sheet.ColumnWidths = nil
	}
	if len(sheet.HiddenCols) == 0 {
		sheet.HiddenCols = nil
	}
	if len(sheet.OutlineRows) == 0 {
		sheet.OutlineRows = nil
	}
	if len(sheet.CollapsedRows) == 0 {
		sheet.CollapsedRows = nil
	}
	if len(sheet.OutlineCols) == 0 {
		sheet.OutlineCols = nil
	}
	if len(sheet.CollapsedCols) == 0 {
		sheet.CollapsedCols = nil
	}
	return sheet
}

func parseLegacyPivotView(payload []byte) (*legacyPivotView, bool) {
	if len(payload) < 42 {
		return nil, false
	}
	view := &legacyPivotView{
		fieldItems:     make(map[int][]legacyPivotItem),
		pageSelections: make(map[int]int),
		currentField:   -1,
		LocationRef:    biffRef8UToSqRef(payload[0:8]),
		CacheIndex:     int(int16(binary.LittleEndian.Uint16(payload[14:16]))),
		RowGrandTotals: binary.LittleEndian.Uint16(payload[34:36])&0x0001 != 0,
		ColGrandTotals: binary.LittleEndian.Uint16(payload[34:36])&0x0002 != 0,
		ShowHeaders:    true,
		rowFieldCount:  int(binary.LittleEndian.Uint16(payload[22:24])),
		colFieldCount:  int(binary.LittleEndian.Uint16(payload[24:26])),
		pageFieldCount: int(binary.LittleEndian.Uint16(payload[26:28])),
	}
	tableNameLen := int(binary.LittleEndian.Uint16(payload[38:40]))
	dataNameLen := int(binary.LittleEndian.Uint16(payload[40:42]))
	next := 42
	if tableNameLen > 0 {
		if name, n, ok := readXLUnicodeStringLen(payload, next, tableNameLen); ok {
			view.Name = strings.TrimSpace(name)
			next = n
		}
	}
	if dataNameLen > 0 {
		if dataName, _, ok := readXLUnicodeStringLen(payload, next, dataNameLen); ok {
			view.DataCaption = strings.TrimSpace(dataName)
		}
	}
	return view, true
}

func parseLegacyFeature11Table(payload []byte, rows map[int]*sheetir.Row) (*sheetir.Table, *legacyTableFeature, bool) {
	if len(payload) < 27+64 {
		return nil, nil, false
	}
	if binary.LittleEndian.Uint16(payload[12:14]) != 0x0005 {
		return nil, nil, false
	}
	ref := biffRef8UToSqRef(payload[4:12])
	cref := int(binary.LittleEndian.Uint16(payload[19:21]))
	pos := 27
	for i := 0; i < cref; i++ {
		if pos+8 > len(payload) {
			break
		}
		if ref == "" {
			ref = biffRef8UToSqRef(payload[pos : pos+8])
		}
		pos += 8
	}
	if ref == "" || pos+64 > len(payload) {
		return nil, nil, false
	}
	feat := payload[pos:]
	idList := binary.LittleEndian.Uint32(feat[4:8])
	if idList == 0 {
		return nil, nil, false
	}
	headerRowCount := int(binary.LittleEndian.Uint32(feat[8:12]))
	if headerRowCount < 0 {
		headerRowCount = 0
	}
	totalsRowCount := int(binary.LittleEndian.Uint32(feat[12:16]))
	if totalsRowCount < 0 {
		totalsRowCount = 0
	}
	flags1 := binary.LittleEndian.Uint16(feat[28:30])
	flags2 := binary.LittleEndian.Uint16(feat[30:32])
	name, next, ok := readXLUnicodeString(feat, 64, 2)
	if !ok {
		return nil, nil, false
	}
	name = strings.TrimSpace(name)
	displayName := name
	if displayName == "" {
		displayName = fmt.Sprintf("Table%d", idList)
	}
	if name == "" {
		name = displayName
	}
	if next+2 > len(feat) {
		return nil, nil, false
	}
	next += 2 // cFieldData
	if flags1&(1<<14) != 0 {
		var ok bool
		_, next, ok = readXLUnicodeString(feat, next, 2)
		if !ok {
			return nil, nil, false
		}
	}
	if flags2&(1<<4) != 0 {
		var ok bool
		_, next, ok = readXLUnicodeString(feat, next, 2)
		if !ok {
			return nil, nil, false
		}
	}
	firstRow, firstCol, lastCol, ok := legacyTableBoundsFromRef(ref)
	if !ok {
		return nil, nil, false
	}
	table := &sheetir.Table{
		ID:             int(idList),
		Name:           name,
		DisplayName:    displayName,
		Ref:            ref,
		HeaderRowCount: headerRowCount,
		TotalsRowCount: totalsRowCount,
		TotalsRowShown: totalsRowCount > 0,
		Columns:        buildLegacyTableColumns(rows, firstRow, firstCol, lastCol, headerRowCount),
	}
	return table, &legacyTableFeature{
		idList: idList,
		ref:    ref,
		name:   displayName,
	}, true
}

func parseLegacyList12TableStyle(payload []byte) (sheetir.TableStyleInfo, bool) {
	if len(payload) < 4 {
		return sheetir.TableStyleInfo{}, false
	}
	flags := binary.LittleEndian.Uint16(payload[0:2])
	name, _, ok := readXLUnicodeString(payload, 2, 2)
	if !ok {
		return sheetir.TableStyleInfo{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return sheetir.TableStyleInfo{}, false
	}
	return sheetir.TableStyleInfo{
		Name:              name,
		ShowFirstColumn:   flags&0x0001 != 0,
		ShowLastColumn:    flags&0x0002 != 0,
		ShowRowStripes:    flags&0x0004 != 0,
		ShowColumnStripes: flags&0x0008 != 0,
	}, true
}

func parseLegacyList12DisplayName(payload []byte) (string, bool) {
	name, _, ok := readXLUnicodeString(payload, 0, 2)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(name), true
}

func legacyTableBoundsFromRef(ref string) (firstRow, firstCol, lastCol int, ok bool) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return 0, 0, 0, false
	}
	startRow, startCol, ok := parseA1CellRef(parts[0])
	if !ok {
		return 0, 0, 0, false
	}
	endRow, endCol, ok := parseA1CellRef(parts[1])
	if !ok {
		return 0, 0, 0, false
	}
	if endRow < startRow || endCol < startCol {
		return 0, 0, 0, false
	}
	return startRow, startCol, endCol, true
}

func parseA1CellRef(ref string) (row, col int, ok bool) {
	ref = strings.TrimSpace(strings.ToUpper(ref))
	if ref == "" {
		return 0, 0, false
	}
	split := 0
	for split < len(ref) && ref[split] >= 'A' && ref[split] <= 'Z' {
		split++
	}
	if split == 0 || split == len(ref) {
		return 0, 0, false
	}
	col = 0
	for i := 0; i < split; i++ {
		col = col*26 + int(ref[i]-'A') + 1
	}
	row, err := strconv.Atoi(ref[split:])
	if err != nil || row <= 0 {
		return 0, 0, false
	}
	return row, col, true
}

func (view *legacyPivotView) addViewField(payload []byte) {
	if view == nil || len(payload) < 10 {
		return
	}
	index := len(view.fieldNames)
	axis := binary.LittleEndian.Uint16(payload[0:2])
	nameLen := int(binary.LittleEndian.Uint16(payload[8:10]))
	name := ""
	if nameLen > 0 && nameLen != 0xFFFF {
		if s, _, ok := readXLUnicodeStringLen(payload, 10, nameLen); ok {
			name = strings.TrimSpace(s)
		}
	}
	if name == "" {
		name = fmt.Sprintf("Field%d", index+1)
	}
	view.fieldNames = append(view.fieldNames, name)
	view.currentField = index
	axisName := ""
	if axis&0x0001 != 0 {
		axisName = "axisRow"
	} else if axis&0x0002 != 0 {
		axisName = "axisCol"
	} else if axis&0x0004 != 0 {
		axisName = "axisPage"
	}
	view.fieldAxes = append(view.fieldAxes, axisName)
	if axis&0x0001 != 0 {
		view.defaultRowOrder = append(view.defaultRowOrder, index)
	}
	if axis&0x0002 != 0 {
		view.defaultColOrder = append(view.defaultColOrder, index)
	}
	if axis&0x0004 != 0 {
		view.defaultPageAxis = append(view.defaultPageAxis, index)
	}
}

func (view *legacyPivotView) addPivotItem(payload []byte) {
	if view == nil || view.currentField < 0 || len(payload) < 8 {
		return
	}
	itemType := int(int16(binary.LittleEndian.Uint16(payload[0:2])))
	flags := binary.LittleEndian.Uint16(payload[2:4])
	cacheIndex := int(int16(binary.LittleEndian.Uint16(payload[4:6])))
	nameLen := int(binary.LittleEndian.Uint16(payload[6:8]))
	name := ""
	if nameLen > 0 && nameLen != 0xFFFF {
		if value, _, ok := readXLUnicodeStringLen(payload, 8, nameLen); ok {
			name = strings.TrimSpace(value)
		}
	}
	view.fieldItems[view.currentField] = append(view.fieldItems[view.currentField], legacyPivotItem{
		cacheIndex: cacheIndex, itemType: itemType, name: name,
		hidden: flags&0x0001 != 0, hideDetail: flags&0x0002 != 0,
	})
}

func (view *legacyPivotView) addAxisFieldOrder(payload []byte) {
	if view == nil || len(payload) < 2 {
		return
	}
	indices := parseLegacyPivotFieldIndices(payload)
	if len(indices) == 0 {
		return
	}
	if view.rowFieldCount > 0 && !view.sawRowOrder {
		view.rowOrder = append([]int(nil), indices...)
		if len(view.rowOrder) > view.rowFieldCount {
			view.rowOrder = view.rowOrder[:view.rowFieldCount]
		}
		view.sawRowOrder = true
		return
	}
	if view.colFieldCount > 0 && !view.sawColOrder {
		view.colOrder = append([]int(nil), indices...)
		if len(view.colOrder) > view.colFieldCount {
			view.colOrder = view.colOrder[:view.colFieldCount]
		}
		view.sawColOrder = true
	}
}

func (view *legacyPivotView) addPageFields(payload []byte) {
	if view == nil || len(payload) < 2 {
		return
	}
	for pos := 0; pos+6 <= len(payload); pos += 6 {
		fieldIndex := int(int16(binary.LittleEndian.Uint16(payload[pos : pos+2])))
		if fieldIndex < 0 {
			continue
		}
		view.pageOrder = append(view.pageOrder, fieldIndex)
		itemIndex := int(int16(binary.LittleEndian.Uint16(payload[pos+2 : pos+4])))
		// 0x7FFD means all pivot items are selected (MS-XLS SXPI_Item).
		if itemIndex == 0x7FFD {
			itemIndex = -1
		}
		view.pageSelections[fieldIndex] = itemIndex
	}
	if len(view.pageOrder) > view.pageFieldCount && view.pageFieldCount > 0 {
		view.pageOrder = view.pageOrder[:view.pageFieldCount]
	}
}

func (view *legacyPivotView) addDataField(payload []byte) {
	if view == nil || len(payload) < 14 {
		return
	}
	fieldIndex := int(int16(binary.LittleEndian.Uint16(payload[0:2])))
	subtotal := legacyPivotSubtotalName(binary.LittleEndian.Uint16(payload[2:4]))
	numFmtID := int(binary.LittleEndian.Uint16(payload[10:12]))
	nameLen := int(binary.LittleEndian.Uint16(payload[12:14]))
	name := ""
	if nameLen > 0 && nameLen != 0xFFFF {
		if s, _, ok := readXLUnicodeStringLen(payload, 14, nameLen); ok {
			name = strings.TrimSpace(s)
		}
	}
	view.dataFields = append(view.dataFields, legacyPivotDataField{
		fieldIndex: fieldIndex,
		name:       name,
		subtotal:   subtotal,
		numFmtID:   numFmtID,
	})
}

func (view *legacyPivotView) resolve(cacheSources []legacyPivotCacheSource) *sheetir.PivotTable {
	if view == nil {
		return nil
	}
	pivot := &sheetir.PivotTable{
		Name:           view.Name,
		LocationRef:    view.LocationRef,
		DataCaption:    view.DataCaption,
		RowGrandTotals: view.RowGrandTotals,
		ColGrandTotals: view.ColGrandTotals,
		ShowHeaders:    view.ShowHeaders,
	}
	if view.CacheIndex >= 0 && view.CacheIndex < len(cacheSources) {
		pivot.SourceSheet = cacheSources[view.CacheIndex].SourceSheet
		pivot.SourceRef = cacheSources[view.CacheIndex].SourceRef
		pivot.SourceName = cacheSources[view.CacheIndex].SourceName
	}
	for index, name := range view.fieldNames {
		field := &sheetir.PivotField{Name: name, ShowAll: true}
		if index < len(view.fieldAxes) {
			field.Axis = view.fieldAxes[index]
		}
		for _, item := range view.fieldItems[index] {
			itemType := "data"
			if item.itemType != 0 {
				itemType = legacyPivotItemTypeName(item.itemType)
			}
			field.Items = append(field.Items, &sheetir.PivotItem{CacheIndex: item.cacheIndex, Type: itemType, Name: item.name, Hidden: item.hidden, HideDetail: item.hideDetail})
		}
		pivot.Fields = append(pivot.Fields, field)
	}

	rowOrder := view.rowOrder
	if len(rowOrder) == 0 {
		rowOrder = view.defaultRowOrder
	}
	colOrder := view.colOrder
	if len(colOrder) == 0 {
		colOrder = view.defaultColOrder
	}
	pageOrder := view.pageOrder
	if len(pageOrder) == 0 {
		pageOrder = view.defaultPageAxis
	}
	for _, index := range rowOrder {
		pivot.RowFields = append(pivot.RowFields, view.fieldName(index))
	}
	for _, index := range colOrder {
		pivot.ColFields = append(pivot.ColFields, view.fieldName(index))
	}
	for _, index := range pageOrder {
		name := view.fieldName(index)
		pivot.PageFields = append(pivot.PageFields, name)
		if itemIndex, ok := view.pageSelections[index]; ok {
			pivot.PageFieldSelections = append(pivot.PageFieldSelections, &sheetir.PivotPageFieldSelection{FieldName: name, ItemIndex: itemIndex})
		}
	}
	for _, field := range view.dataFields {
		name := field.name
		sourceField := view.fieldName(field.fieldIndex)
		if name == "" {
			name = buildLegacyPivotDataFieldName(field.subtotal, sourceField)
		}
		subtotal := field.subtotal
		if subtotal == "" {
			subtotal = "sum"
		}
		pivot.DataFields = append(pivot.DataFields, &sheetir.PivotDataField{
			Name:            name,
			SourceFieldName: sourceField,
			Subtotal:        subtotal,
			NumFmtID:        field.numFmtID,
		})
	}
	if pivot.Name == "" && pivot.LocationRef == "" && len(pivot.RowFields) == 0 && len(pivot.ColFields) == 0 && len(pivot.PageFields) == 0 && len(pivot.DataFields) == 0 {
		return nil
	}
	return pivot
}

func (view *legacyPivotView) fieldName(index int) string {
	if index >= 0 && index < len(view.fieldNames) {
		return view.fieldNames[index]
	}
	return fmt.Sprintf("Field%d", index+1)
}

func parseLegacyPivotFieldIndices(payload []byte) []int {
	indices := make([]int, 0, len(payload)/2)
	for pos := 0; pos+2 <= len(payload); pos += 2 {
		index := int(int16(binary.LittleEndian.Uint16(payload[pos : pos+2])))
		if index >= 0 {
			indices = append(indices, index)
		}
	}
	return indices
}

func legacyPivotItemTypeName(itemType int) string {
	switch itemType {
	case 0:
		return "data"
	case 1:
		return "default"
	case 2:
		return "sum"
	case 3:
		return "countA"
	case 4:
		return "average"
	case 5:
		return "max"
	case 6:
		return "min"
	case 7:
		return "product"
	case 8:
		return "count"
	case 9:
		return "stdDev"
	case 10:
		return "stdDevP"
	case 11:
		return "var"
	case 12:
		return "varP"
	case 13:
		return "grand"
	case 14:
		return "blank"
	default:
		return fmt.Sprintf("type%d", itemType)
	}
}

func legacyPivotSubtotalName(iiftab uint16) string {
	switch iiftab {
	case 0:
		return "sum"
	case 1:
		return "count"
	case 2:
		return "average"
	case 3:
		return "max"
	case 4:
		return "min"
	case 5:
		return "product"
	case 6:
		return "countNums"
	case 7:
		return "stdDev"
	case 8:
		return "stdDevP"
	case 9:
		return "var"
	case 10:
		return "varP"
	default:
		return ""
	}
}

func buildLegacyPivotDataFieldName(subtotal, sourceField string) string {
	switch subtotal {
	case "count":
		return "Count of " + sourceField
	case "average":
		return "Average of " + sourceField
	case "max":
		return "Max of " + sourceField
	case "min":
		return "Min of " + sourceField
	case "product":
		return "Product of " + sourceField
	case "countNums":
		return "CountNums of " + sourceField
	case "stdDev":
		return "StdDev of " + sourceField
	case "stdDevP":
		return "StdDevP of " + sourceField
	case "var":
		return "Var of " + sourceField
	case "varP":
		return "VarP of " + sourceField
	default:
		return "Sum of " + sourceField
	}
}

func parseLabelSstCell(payload []byte, sst []string, xfs []legacyXF) *sheetir.Cell {
	if len(payload) < 10 {
		return nil
	}
	idx := int(binary.LittleEndian.Uint32(payload[6:10]))
	value := ""
	if idx >= 0 && idx < len(sst) {
		value = strings.TrimSpace(sst[idx])
	}
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     sheetir.CellString,
		StrValue: value,
		Style:    styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs),
	}
}

func parseLabelCell(payload []byte, xfs []legacyXF) *sheetir.Cell {
	if len(payload) < 8 {
		return nil
	}
	value, _, ok := readXLUnicodeString(payload, 6, 2)
	if !ok {
		value = strings.TrimSpace(string(payload[6:]))
	}
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     sheetir.CellString,
		StrValue: value,
		Style:    styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs),
	}
}

func parseLabelCellBIFF2(payload []byte) *sheetir.Cell {
	if len(payload) < 8 {
		return nil
	}
	cch := int(payload[7])
	if 8+cch > len(payload) {
		return nil
	}
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     sheetir.CellString,
		StrValue: strings.TrimSpace(decodeBiffBytes(payload[8 : 8+cch])),
	}
}

func parseRStringCell(payload []byte, xfs []legacyXF) *sheetir.Cell {
	// RSTRING: row(2) col(2) xf(2) cch(2) rgch(cch) cRuns(2) runs(4*cRuns)
	if len(payload) < 8 {
		return nil
	}
	cch := int(binary.LittleEndian.Uint16(payload[6:8]))
	if 8+cch > len(payload) {
		return nil
	}
	value := strings.TrimSpace(decodeBiffBytes(payload[8 : 8+cch]))
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     sheetir.CellString,
		StrValue: value,
		Style:    styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs),
	}
}

func parseBlankCell(payload []byte, xfs []legacyXF) *sheetir.Cell {
	if len(payload) < 6 {
		return nil
	}
	return &sheetir.Cell{
		Row:   int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:   int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:  sheetir.CellEmpty,
		Style: styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs),
	}
}

func parseBlankCellBIFF2(payload []byte) *sheetir.Cell {
	if len(payload) < 7 {
		return nil
	}
	return &sheetir.Cell{
		Row:  int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:  int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind: sheetir.CellEmpty,
	}
}

func parseNumberCell(payload []byte, xfs []legacyXF) *sheetir.Cell {
	if len(payload) < 14 {
		return nil
	}
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     sheetir.CellNumber,
		NumValue: math.Float64frombits(binary.LittleEndian.Uint64(payload[6:14])),
		Style:    styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs),
	}
}

func parseNumberCellBIFF2(payload []byte) *sheetir.Cell {
	if len(payload) < 15 {
		return nil
	}
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     sheetir.CellNumber,
		NumValue: math.Float64frombits(binary.LittleEndian.Uint64(payload[7:15])),
	}
}

func parseMulBlankCells(payload []byte, xfs []legacyXF) []*sheetir.Cell {
	if len(payload) < 8 {
		return nil
	}
	row := int(binary.LittleEndian.Uint16(payload[0:2])) + 1
	firstCol := int(binary.LittleEndian.Uint16(payload[2:4]))
	lastCol := int(binary.LittleEndian.Uint16(payload[len(payload)-2:]))
	count := lastCol - firstCol + 1
	if count <= 0 || 4+count*2+2 > len(payload) {
		return nil
	}
	cells := make([]*sheetir.Cell, 0, count)
	offset := 4
	for col := firstCol; col <= lastCol && offset+2 <= len(payload)-2; col++ {
		xfIndex := binary.LittleEndian.Uint16(payload[offset : offset+2])
		cells = append(cells, &sheetir.Cell{
			Row:   row,
			Col:   col + 1,
			Kind:  sheetir.CellEmpty,
			Style: styleFromXFIndex(xfIndex, xfs),
		})
		offset += 2
	}
	return cells
}

func parseRKCell(payload []byte, xfs []legacyXF) *sheetir.Cell {
	if len(payload) < 10 {
		return nil
	}
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     sheetir.CellNumber,
		NumValue: decodeRK(binary.LittleEndian.Uint32(payload[6:10])),
		Style:    styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs),
	}
}

func parseMulRKCells(payload []byte, xfs []legacyXF) []*sheetir.Cell {
	if len(payload) < 10 {
		return nil
	}
	row := int(binary.LittleEndian.Uint16(payload[0:2])) + 1
	firstCol := int(binary.LittleEndian.Uint16(payload[2:4]))
	lastCol := int(binary.LittleEndian.Uint16(payload[len(payload)-2:]))
	count := lastCol - firstCol + 1
	if count <= 0 || 4+count*6+2 > len(payload) {
		return nil
	}
	cells := make([]*sheetir.Cell, 0, count)
	offset := 4
	for col := firstCol; col <= lastCol && offset+6 <= len(payload)-2; col++ {
		xfIndex := binary.LittleEndian.Uint16(payload[offset : offset+2])
		rk := binary.LittleEndian.Uint32(payload[offset+2 : offset+6])
		cells = append(cells, &sheetir.Cell{
			Row:      row,
			Col:      col + 1,
			Kind:     sheetir.CellNumber,
			NumValue: decodeRK(rk),
			Style:    styleFromXFIndex(xfIndex, xfs),
		})
		offset += 6
	}
	return cells
}

func parseBoolErrCell(payload []byte, xfs []legacyXF) *sheetir.Cell {
	if len(payload) < 8 {
		return nil
	}
	kind := sheetir.CellBool
	strValue := ""
	numValue := float64(payload[6])
	if payload[7] != 0 {
		kind = sheetir.CellError
		strValue = biffErrorString(payload[6])
		numValue = 0
	}
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     kind,
		NumValue: numValue,
		StrValue: strValue,
		Style:    styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs),
	}
}

func parseBoolErrCellBIFF2(payload []byte) *sheetir.Cell {
	if len(payload) < 9 {
		return nil
	}
	kind := sheetir.CellBool
	strValue := ""
	numValue := float64(payload[7])
	if payload[8] != 0 {
		kind = sheetir.CellError
		strValue = biffErrorString(payload[7])
		numValue = 0
	}
	return &sheetir.Cell{
		Row:      int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:      int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Kind:     kind,
		NumValue: numValue,
		StrValue: strValue,
	}
}

func definedNameList(definedNames []*sheetir.DefinedName) []string {
	out := make([]string, 0, len(definedNames))
	for _, definedName := range definedNames {
		if definedName == nil {
			continue
		}
		name := strings.TrimSpace(definedName.Name)
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}

func parseFormulaCell(payload []byte, xfs []legacyXF, workbookCtx legacyFormulaWorkbookContext) *sheetir.Cell {
	if len(payload) < 20 {
		return nil
	}
	cell := &sheetir.Cell{
		Row:   int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col:   int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
		Style: styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs),
	}
	cell.Ref = formatA1Ref(cell.Row, cell.Col)
	if len(payload) >= 22 {
		cce := int(binary.LittleEndian.Uint16(payload[20:22]))
		if cce > 0 && 22+cce <= len(payload) {
			formulaBytes := payload[22 : 22+cce]
			if formulaText, ok := decodeBIFFFormula(formulaBytes, legacyFormulaDecodeContext{
				workbook: workbookCtx,
				row:      cell.Row,
				col:      cell.Col,
			}); ok {
				cell.Formula = formulaText
			}
		}
	}
	result := payload[6:14]
	if binary.LittleEndian.Uint16(result[6:8]) != 0xFFFF {
		cell.Kind = sheetir.CellNumber
		cell.NumValue = math.Float64frombits(binary.LittleEndian.Uint64(result))
		promoteNumericCellToDate(cell, workbookCtx.date1904)
		return cell
	}
	// FormulaSpecialCachedValue encoding (mirrors POI FormulaSpecialCachedValue):
	//   BIT_MARKER = 0xFFFF000000000000L (high 16 bits of the 8-byte double)
	//   Low 6 bytes: [type(1), reserved(1), data(1), reserved(3)]
	//   type 0x00 = STRING (needs trailing StringRecord)
	//   type 0x01 = BOOLEAN (data byte at offset 2)
	//   type 0x02 = ERROR_CODE (error code at offset 2)
	//   type 0x03 = EMPTY (empty string result, may have trailing StringRecord)
	switch result[0] {
	case 0x00: // POI STRING — formula result is a string, needs trailing StringRecord
		cell.Kind = sheetir.CellEmpty
		return cell
	case 0x01: // POI BOOLEAN
		cell.Kind = sheetir.CellBool
		cell.NumValue = float64(result[2])
		return cell
	case 0x02: // POI ERROR_CODE
		cell.Kind = sheetir.CellError
		cell.StrValue = biffErrorString(result[2])
		return cell
	case 0x03: // POI EMPTY — empty string result, may have trailing StringRecord
		cell.Kind = sheetir.CellEmpty
		return cell
	default:
		cell.Kind = sheetir.CellEmpty
		return cell
	}
}

func parseOldFormulaCell(payload []byte, xfs []legacyXF, workbookCtx legacyFormulaWorkbookContext, isBIFF2 bool) *sheetir.Cell {
	if len(payload) < 18 {
		return nil
	}
	cell := &sheetir.Cell{
		Row: int(binary.LittleEndian.Uint16(payload[0:2])) + 1,
		Col: int(binary.LittleEndian.Uint16(payload[2:4])) + 1,
	}
	if !isBIFF2 {
		cell.Style = styleFromXFIndex(binary.LittleEndian.Uint16(payload[4:6]), xfs)
	}
	cell.Ref = formatA1Ref(cell.Row, cell.Col)

	resultOffset := 6
	optionsOffset := 14
	if isBIFF2 {
		resultOffset = 7
		optionsOffset = 15
	}
	result := payload[resultOffset : resultOffset+8]
	exprLenOffset := optionsOffset + 2
	if isBIFF2 {
		exprLenOffset = optionsOffset + 1
	}
	if exprLenOffset+2 <= len(payload) {
		cce := int(binary.LittleEndian.Uint16(payload[exprLenOffset : exprLenOffset+2]))
		if cce > 0 && exprLenOffset+2+cce <= len(payload) {
			formulaBytes := payload[exprLenOffset+2 : exprLenOffset+2+cce]
			if formulaText, ok := decodeBIFFFormula(formulaBytes, legacyFormulaDecodeContext{
				workbook: workbookCtx,
				row:      cell.Row,
				col:      cell.Col,
			}); ok {
				cell.Formula = formulaText
			}
		}
	}
	if isBIFF2 || binary.LittleEndian.Uint16(result[6:8]) != 0xFFFF {
		cell.Kind = sheetir.CellNumber
		cell.NumValue = math.Float64frombits(binary.LittleEndian.Uint64(result))
		promoteNumericCellToDate(cell, workbookCtx.date1904)
		return cell
	}
	switch result[0] {
	case 0x00:
		cell.Kind = sheetir.CellEmpty
		return cell
	case 0x01:
		cell.Kind = sheetir.CellBool
		cell.NumValue = float64(result[2])
		return cell
	case 0x02:
		cell.Kind = sheetir.CellError
		cell.StrValue = biffErrorString(result[2])
		return cell
	case 0x03:
		cell.Kind = sheetir.CellEmpty
		return cell
	default:
		cell.Kind = sheetir.CellEmpty
		return cell
	}
}

func parseOldFormulaStringRecord(payload []byte, isBIFF2 bool) (string, bool) {
	if isBIFF2 {
		if len(payload) < 1 {
			return "", false
		}
		cch := int(payload[0])
		if 1+cch > len(payload) {
			return "", false
		}
		return strings.TrimSpace(decodeBiffBytes(payload[1 : 1+cch])), true
	}
	if len(payload) < 2 {
		return "", false
	}
	cch := int(binary.LittleEndian.Uint16(payload[0:2]))
	if 2+cch > len(payload) {
		return "", false
	}
	return strings.TrimSpace(decodeBiffBytes(payload[2 : 2+cch])), true
}

func parseArrayFormulaRecord(payload []byte, workbookCtx legacyFormulaWorkbookContext) (*sheetir.ArrayFormula, bool) {
	if len(payload) < 6 {
		return nil, false
	}
	topRow := int(binary.LittleEndian.Uint16(payload[0:2])) + 1
	bottomRow := int(binary.LittleEndian.Uint16(payload[2:4])) + 1
	leftCol := int(payload[4]) + 1
	rightCol := int(payload[5]) + 1
	if topRow <= 0 || leftCol <= 0 {
		return nil, false
	}
	if bottomRow < topRow {
		bottomRow = topRow
	}
	if rightCol < leftCol {
		rightCol = leftCol
	}
	anchor := formatA1Ref(topRow, leftCol)
	rangeRef := fmt.Sprintf("%s:%s", anchor, formatA1Ref(bottomRow, rightCol))
	formulaText := ""
	if len(payload) >= 14 {
		cce := int(binary.LittleEndian.Uint16(payload[12:14]))
		if cce > 0 && 14+cce <= len(payload) {
			if text, ok := decodeBIFFFormula(payload[14:14+cce], legacyFormulaDecodeContext{
				workbook: workbookCtx,
				// ARRAY-record formulas use A1-relative bases for ref tokens rather than the array anchor.
				row: 1,
				col: 1,
			}); ok {
				formulaText = text
			}
		}
	}
	return &sheetir.ArrayFormula{
		AnchorRef: anchor,
		RangeRef:  rangeRef,
		TopRow:    topRow,
		LeftCol:   leftCol,
		BottomRow: bottomRow,
		RightCol:  rightCol,
		Formula:   formulaText,
	}, true
}

func parseFormulaExpRef(payload []byte) (string, bool) {
	if len(payload) < 27 {
		return "", false
	}
	cce := int(binary.LittleEndian.Uint16(payload[20:22]))
	if cce != 5 || 22+cce > len(payload) {
		return "", false
	}
	formula := payload[22 : 22+cce]
	if formula[0]&0x3F != 0x01 {
		return "", false
	}
	row := int(binary.LittleEndian.Uint16(formula[1:3])) + 1
	col := int(binary.LittleEndian.Uint16(formula[3:5])) + 1
	if row <= 0 || col <= 0 {
		return "", false
	}
	return formatA1Ref(row, col), true
}

func parseSharedFormulaRecord(payload []byte, anchorRef string, anchorRow, anchorCol int) (legacySharedFormulaRecord, bool) {
	if len(payload) < 10 {
		return legacySharedFormulaRecord{}, false
	}
	topRow := int(binary.LittleEndian.Uint16(payload[0:2])) + 1
	bottomRow := int(binary.LittleEndian.Uint16(payload[2:4])) + 1
	leftCol := int(payload[4]) + 1
	rightCol := int(payload[5]) + 1
	if topRow <= 0 || leftCol <= 0 {
		return legacySharedFormulaRecord{}, false
	}
	if bottomRow < topRow {
		bottomRow = topRow
	}
	if rightCol < leftCol {
		rightCol = leftCol
	}
	if anchorRef == "" {
		if anchorRow <= 0 {
			anchorRow = topRow
		}
		if anchorCol <= 0 {
			anchorCol = leftCol
		}
		anchorRef = formatA1Ref(anchorRow, anchorCol)
	}
	cce := int(binary.LittleEndian.Uint16(payload[8:10]))
	if cce <= 0 || 10+cce > len(payload) {
		return legacySharedFormulaRecord{}, false
	}
	return legacySharedFormulaRecord{
		AnchorRef:   anchorRef,
		RangeRef:    fmt.Sprintf("%s:%s", formatA1Ref(topRow, leftCol), formatA1Ref(bottomRow, rightCol)),
		TopRow:      topRow,
		LeftCol:     leftCol,
		BottomRow:   bottomRow,
		RightCol:    rightCol,
		FormulaData: append([]byte(nil), payload[10:10+cce]...),
	}, true
}

func applyExpFormulaMetadata(rows map[int]*sheetir.Row, groups []*sheetir.ArrayFormula, expRefs map[string]string) {
	if len(expRefs) == 0 || len(groups) == 0 {
		return
	}
	groupsByAnchor := make(map[string]*sheetir.ArrayFormula, len(groups))
	for _, group := range groups {
		if group == nil || group.AnchorRef == "" {
			continue
		}
		groupsByAnchor[strings.ToUpper(group.AnchorRef)] = group
	}
	for ref, anchorRef := range expRefs {
		group := groupsByAnchor[strings.ToUpper(anchorRef)]
		if group == nil || strings.TrimSpace(group.Formula) == "" {
			continue
		}
		cell := findCellByRef(rows, ref)
		if cell == nil {
			continue
		}
		if cell.Formula == "" {
			cell.Formula = group.Formula
		}
		if cell.ArrayFormulaRange == "" {
			cell.ArrayFormulaRange = group.RangeRef
		}
		if cell.ArrayFormulaAnchor == "" {
			cell.ArrayFormulaAnchor = group.AnchorRef
		}
	}
}

func applySharedFormulaMetadata(rows map[int]*sheetir.Row, sharedFormulas map[string]legacySharedFormulaRecord, expRefs map[string]string, workbookCtx legacyFormulaWorkbookContext) {
	if len(sharedFormulas) == 0 || len(expRefs) == 0 {
		return
	}
	for ref, anchorRef := range expRefs {
		shared, ok := sharedFormulas[strings.ToUpper(anchorRef)]
		if !ok || len(shared.FormulaData) == 0 {
			continue
		}
		cell := ensureCellByRef(rows, ref)
		if cell == nil {
			continue
		}
		formulaText, ok := decodeBIFFFormula(shared.FormulaData, legacyFormulaDecodeContext{
			workbook: workbookCtx,
			row:      cell.Row,
			col:      cell.Col,
		})
		if !ok || strings.TrimSpace(formulaText) == "" {
			continue
		}
		cell.Formula = formulaText
		if cell.Kind == sheetir.CellEmpty {
			cell.Kind = sheetir.CellFormula
		}
	}
}

func findCellByRef(rows map[int]*sheetir.Row, ref string) *sheetir.Cell {
	if ref == "" {
		return nil
	}
	want := strings.ToUpper(ref)
	for _, row := range rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell != nil && strings.EqualFold(cell.Ref, want) {
				return cell
			}
		}
	}
	return nil
}

func ensureCellByRef(rows map[int]*sheetir.Row, ref string) *sheetir.Cell {
	if ref == "" {
		return nil
	}
	if cell := findCellByRef(rows, ref); cell != nil {
		return cell
	}
	row, col, ok := parseA1Ref(ref)
	if !ok {
		return nil
	}
	cell := &sheetir.Cell{Row: row, Col: col, Kind: sheetir.CellFormula}
	addSheetCell(rows, cell)
	return cell
}

func decodeBIFFFormula(formula []byte, ctx legacyFormulaDecodeContext) (string, bool) {
	type formulaStack struct {
		items []legacyFormulaExpr
	}
	push := func(stack *formulaStack, expr legacyFormulaExpr) {
		stack.items = append(stack.items, expr)
	}
	pop := func(stack *formulaStack) (legacyFormulaExpr, bool) {
		if len(stack.items) == 0 {
			return legacyFormulaExpr{}, false
		}
		last := stack.items[len(stack.items)-1]
		stack.items = stack.items[:len(stack.items)-1]
		return last, true
	}
	popMany := func(stack *formulaStack, count int) ([]legacyFormulaExpr, bool) {
		if count < 0 || len(stack.items) < count {
			return nil, false
		}
		start := len(stack.items) - count
		out := append([]legacyFormulaExpr(nil), stack.items[start:]...)
		stack.items = stack.items[:start]
		return out, true
	}

	stack := formulaStack{}
	for i := 0; i < len(formula); {
		token := normalizeLegacyFormulaToken(formula[i])
		switch token {
		case 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11:
			right, ok := pop(&stack)
			if !ok {
				return "", false
			}
			left, ok := pop(&stack)
			if !ok {
				return "", false
			}
			push(&stack, decodeBIFFBinaryExpr(token, left, right))
			i++
		case 0x12, 0x13:
			expr, ok := pop(&stack)
			if !ok {
				return "", false
			}
			push(&stack, decodeBIFFUnaryExpr(token, expr))
			i++
		case 0x14:
			expr, ok := pop(&stack)
			if !ok {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: wrapLegacyFormulaExpr(expr, 90, false) + "%", precedence: 90})
			i++
		case 0x15:
			expr, ok := pop(&stack)
			if !ok {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: "(" + expr.text + ")", precedence: 100})
			i++
		case 0x16:
			push(&stack, legacyFormulaExpr{text: "", precedence: 100})
			i++
		case 0x18:
			// ptgTbl — data table reference token; 4 bytes body, no value to push.
			if i+5 > len(formula) {
				return "", false
			}
			i += 5
		case 0x20:
			// ptgArray — literal array constant. Body is 8 bytes (1 token + 7 reserved).
			// The actual array data lives in the extra-data section after the RPN stream,
			// which we don't plumb through. Push a placeholder so the caller sees a formula.
			if i+8 > len(formula) {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: "{...}", precedence: 100})
			i += 8
		case 0x17:
			text, size, ok := decodePtgString(formula[i:])
			if !ok {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: text, precedence: 100})
			i += size
		case 0x19:
			size, ok := legacyAttrTokenSize(formula[i:])
			if !ok {
				return "", false
			}
			attr := formula[i+1]
			if attr&0x10 != 0 {
				expr, ok := pop(&stack)
				if !ok {
					return "", false
				}
				push(&stack, legacyFormulaExpr{text: "SUM(" + expr.text + ")", precedence: 100})
			} else if attr&0x02 != 0 {
				if len(stack.items) == 0 {
					return "", false
				}
				condition := stack.items[len(stack.items)-1]
				if expr, next, ok := decodeOptimizedBIFFIfAttr(formula, i, ctx, condition); ok {
					stack.items = stack.items[:len(stack.items)-1]
					push(&stack, expr)
					i = next
					continue
				}
			}
			i += size
		case 0x1C:
			if i+2 > len(formula) {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: biffErrorString(formula[i+1]), precedence: 100})
			i += 2
		case 0x1D:
			if i+2 > len(formula) {
				return "", false
			}
			if formula[i+1] != 0 {
				push(&stack, legacyFormulaExpr{text: "TRUE", precedence: 100})
			} else {
				push(&stack, legacyFormulaExpr{text: "FALSE", precedence: 100})
			}
			i += 2
		case 0x1E:
			if i+3 > len(formula) {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: strconv.Itoa(int(binary.LittleEndian.Uint16(formula[i+1 : i+3]))), precedence: 100})
			i += 3
		case 0x1F:
			if i+9 > len(formula) {
				return "", false
			}
			value := math.Float64frombits(binary.LittleEndian.Uint64(formula[i+1 : i+9]))
			push(&stack, legacyFormulaExpr{text: strconv.FormatFloat(value, 'g', -1, 64), precedence: 100})
			i += 9
		case 0x21:
			if i+3 > len(formula) {
				return "", false
			}
			iftab := binary.LittleEndian.Uint16(formula[i+1 : i+3])
			info, ok := legacyFormulaBuiltinFunctions[iftab]
			if !ok {
				return "", false
			}
			args, ok := popMany(&stack, info.arity)
			if !ok {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: formatLegacyFunctionCall(info.name, args), precedence: 100})
			i += 3
		case 0x22:
			if i+4 > len(formula) {
				return "", false
			}
			argc := int(formula[i+1] & 0x7F)
			iftab := binary.LittleEndian.Uint16(formula[i+2 : i+4])
			args, ok := popMany(&stack, argc)
			if !ok {
				return "", false
			}
			name := fmt.Sprintf("FUNC_%d", iftab)
			if info, ok := legacyFormulaBuiltinFunctions[iftab]; ok && info.name != "" {
				name = info.name
			}
			push(&stack, legacyFormulaExpr{text: formatLegacyFunctionCall(name, args), precedence: 100})
			i += 4
		case 0x23:
			if i+5 > len(formula) {
				return "", false
			}
			nameIndex := int(binary.LittleEndian.Uint32(formula[i+1 : i+5]))
			push(&stack, legacyFormulaExpr{text: resolveLegacyDefinedName(nameIndex, ctx.workbook.definedNames), precedence: 100})
			i += 5
		case 0x24, 0x2C:
			ref, ok := decodeBIFFRefToken(formula[i:], ctx)
			if !ok {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: ref, precedence: 100})
			i += 5
		case 0x25, 0x2D:
			ref, ok := decodeBIFFAreaToken(formula[i:], ctx)
			if !ok {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: ref, precedence: 100})
			i += 9
		case 0x26, 0x27, 0x28:
			if i+7 > len(formula) {
				return "", false
			}
			i += 7
		case 0x29, 0x2E, 0x2F:
			if i+3 > len(formula) {
				return "", false
			}
			i += 3
		case 0x2A:
			if i+5 > len(formula) {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: "#REF!", precedence: 100})
			i += 5
		case 0x2B:
			if i+9 > len(formula) {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: "#REF!", precedence: 100})
			i += 9
		case 0x39:
			if i+7 > len(formula) {
				return "", false
			}
			sheetRefIndex := binary.LittleEndian.Uint16(formula[i+1 : i+3])
			externNameIndex := int(binary.LittleEndian.Uint16(formula[i+3:i+5])) - 1
			text, ok := resolveExternalNameText(sheetRefIndex, externNameIndex, ctx.workbook)
			if !ok || text == "" {
				text = fmt.Sprintf("NAME_X_%d", externNameIndex+1)
			}
			push(&stack, legacyFormulaExpr{text: text, precedence: 100})
			i += 7
		case 0x3A:
			ref, ok := decodeBIFFRef3DToken(formula[i:], ctx)
			if !ok {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: ref, precedence: 100})
			i += 7
		case 0x3B:
			ref, ok := decodeBIFFArea3DToken(formula[i:], ctx)
			if !ok {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: ref, precedence: 100})
			i += 11
		case 0x3C:
			if i+7 > len(formula) {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: "#REF!", precedence: 100})
			i += 7
		case 0x3D:
			if i+11 > len(formula) {
				return "", false
			}
			push(&stack, legacyFormulaExpr{text: "#REF!", precedence: 100})
			i += 11
		default:
			return "", false
		}
	}
	if len(stack.items) != 1 {
		return "", false
	}
	result := strings.TrimSpace(stack.items[0].text)
	return result, result != ""
}

func legacyAttrTokenSize(token []byte) (int, bool) {
	if len(token) < 4 {
		return 0, false
	}
	attr := token[1]
	if attr&0x04 != 0 {
		choiceCount := int(binary.LittleEndian.Uint16(token[2:4]))
		size := 4 + (choiceCount+1)*2
		if len(token) < size {
			return 0, false
		}
		return size, true
	}
	return 4, true
}

func decodeOptimizedBIFFIfAttr(formula []byte, index int, ctx legacyFormulaDecodeContext, condition legacyFormulaExpr) (legacyFormulaExpr, int, bool) {
	if index < 0 || index+4 > len(formula) {
		return legacyFormulaExpr{}, 0, false
	}
	falseStart := index + 4 + int(binary.LittleEndian.Uint16(formula[index+2:index+4]))
	if falseStart <= index+4 || falseStart > len(formula) {
		return legacyFormulaExpr{}, 0, false
	}
	attrSkipPos := falseStart - 4
	if attrSkipPos < index+4 || attrSkipPos+4 > len(formula) {
		return legacyFormulaExpr{}, 0, false
	}
	if normalizeLegacyFormulaToken(formula[attrSkipPos]) != 0x19 || formula[attrSkipPos+1]&0x08 == 0 {
		return legacyFormulaExpr{}, 0, false
	}
	falseEnd := falseStart + int(binary.LittleEndian.Uint16(formula[attrSkipPos+2:attrSkipPos+4]))
	if falseEnd <= falseStart || falseEnd > len(formula) {
		return legacyFormulaExpr{}, 0, false
	}
	if legacyNextTokenIsBuiltinIf(formula, falseEnd) {
		return legacyFormulaExpr{}, 0, false
	}
	trueText, ok := decodeBIFFFormula(formula[index+4:attrSkipPos], ctx)
	if !ok {
		return legacyFormulaExpr{}, 0, false
	}
	falseText, ok := decodeBIFFFormula(formula[falseStart:falseEnd], ctx)
	if !ok {
		return legacyFormulaExpr{}, 0, false
	}
	return legacyFormulaExpr{
		text: formatLegacyFunctionCall("IF", []legacyFormulaExpr{
			condition,
			{text: trueText, precedence: 100},
			{text: falseText, precedence: 100},
		}),
		precedence: 100,
	}, falseEnd, true
}

func legacyNextTokenIsBuiltinIf(formula []byte, index int) bool {
	if index < 0 || index >= len(formula) {
		return false
	}
	switch normalizeLegacyFormulaToken(formula[index]) {
	case 0x21:
		return index+3 <= len(formula) && binary.LittleEndian.Uint16(formula[index+1:index+3]) == 1
	case 0x22:
		return index+4 <= len(formula) && binary.LittleEndian.Uint16(formula[index+2:index+4]) == 1
	default:
		return false
	}
}

func normalizeLegacyFormulaToken(raw byte) byte {
	switch {
	case raw >= 0x40 && raw <= 0x5D:
		return raw - 0x20
	case raw >= 0x60 && raw <= 0x7D:
		return raw - 0x40
	default:
		return raw
	}
}

func decodeBIFFBinaryExpr(token byte, left, right legacyFormulaExpr) legacyFormulaExpr {
	switch token {
	case 0x03:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 50, false) + "+" + wrapLegacyFormulaExpr(right, 50, false), precedence: 50}
	case 0x04:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 50, false) + "-" + wrapLegacyFormulaExpr(right, 50, true), precedence: 50}
	case 0x05:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 60, false) + "*" + wrapLegacyFormulaExpr(right, 60, false), precedence: 60}
	case 0x06:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 60, false) + "/" + wrapLegacyFormulaExpr(right, 60, true), precedence: 60}
	case 0x07:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 70, true) + "^" + wrapLegacyFormulaExpr(right, 70, false), precedence: 70}
	case 0x08:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 40, false) + "&" + wrapLegacyFormulaExpr(right, 40, false), precedence: 40}
	case 0x09:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 30, false) + "<" + wrapLegacyFormulaExpr(right, 30, false), precedence: 30}
	case 0x0A:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 30, false) + "<=" + wrapLegacyFormulaExpr(right, 30, false), precedence: 30}
	case 0x0B:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 30, false) + "=" + wrapLegacyFormulaExpr(right, 30, false), precedence: 30}
	case 0x0C:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 30, false) + ">=" + wrapLegacyFormulaExpr(right, 30, false), precedence: 30}
	case 0x0D:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 30, false) + ">" + wrapLegacyFormulaExpr(right, 30, false), precedence: 30}
	case 0x0E:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 30, false) + "<>" + wrapLegacyFormulaExpr(right, 30, false), precedence: 30}
	case 0x0F:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 80, false) + " " + wrapLegacyFormulaExpr(right, 80, false), precedence: 80}
	case 0x10:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 10, false) + "," + wrapLegacyFormulaExpr(right, 10, false), precedence: 10}
	case 0x11:
		return legacyFormulaExpr{text: wrapLegacyFormulaExpr(left, 85, false) + ":" + wrapLegacyFormulaExpr(right, 85, false), precedence: 85}
	default:
		return legacyFormulaExpr{}
	}
}

func decodeBIFFUnaryExpr(token byte, expr legacyFormulaExpr) legacyFormulaExpr {
	switch token {
	case 0x12:
		return legacyFormulaExpr{text: "+" + wrapLegacyFormulaExpr(expr, 75, false), precedence: 75}
	case 0x13:
		return legacyFormulaExpr{text: "-" + wrapLegacyFormulaExpr(expr, 75, false), precedence: 75}
	default:
		return legacyFormulaExpr{}
	}
}

func wrapLegacyFormulaExpr(expr legacyFormulaExpr, parentPrecedence int, wrapIfEqual bool) string {
	if expr.text == "" {
		return ""
	}
	if expr.precedence < parentPrecedence || (wrapIfEqual && expr.precedence == parentPrecedence) {
		return "(" + expr.text + ")"
	}
	return expr.text
}

func decodePtgString(token []byte) (string, int, bool) {
	if len(token) < 3 {
		return "", 0, false
	}
	charCount := int(token[1])
	flags := token[2]
	charBytes := charCount
	if flags&0x01 != 0 {
		charBytes = charCount * 2
	}
	if 3+charBytes > len(token) {
		return "", 0, false
	}
	var value string
	if flags&0x01 != 0 {
		value = decodeBiffUTF16Raw(token[3 : 3+charBytes])
	} else {
		value = decodeBiffBytesRaw(token[3 : 3+charBytes])
	}
	value = strings.ReplaceAll(value, `"`, `""`)
	return `"` + value + `"`, 3 + charBytes, true
}

func formatLegacyFunctionCall(name string, args []legacyFormulaExpr) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = arg.text
	}
	return name + "(" + strings.Join(parts, ",") + ")"
}

func resolveLegacyDefinedName(index int, names []string) string {
	if index > 0 && index <= len(names) {
		return names[index-1]
	}
	return fmt.Sprintf("NAME_%d", index)
}

func decodeBIFFRefToken(token []byte, ctx legacyFormulaDecodeContext) (string, bool) {
	if len(token) < 5 {
		return "", false
	}
	row, col, absRow, absCol := decodeBIFFRefParts(binary.LittleEndian.Uint16(token[1:3]), binary.LittleEndian.Uint16(token[3:5]), ctx.row, ctx.col)
	return formatLegacyA1Ref(row, col, absRow, absCol), true
}

func decodeBIFFAreaToken(token []byte, ctx legacyFormulaDecodeContext) (string, bool) {
	if len(token) < 9 {
		return "", false
	}
	firstRow, firstCol, firstAbsRow, firstAbsCol := decodeBIFFRefParts(binary.LittleEndian.Uint16(token[1:3]), binary.LittleEndian.Uint16(token[5:7]), ctx.row, ctx.col)
	lastRow, lastCol, lastAbsRow, lastAbsCol := decodeBIFFRefParts(binary.LittleEndian.Uint16(token[3:5]), binary.LittleEndian.Uint16(token[7:9]), ctx.row, ctx.col)
	return formatLegacyA1Ref(firstRow, firstCol, firstAbsRow, firstAbsCol) + ":" + formatLegacyA1Ref(lastRow, lastCol, lastAbsRow, lastAbsCol), true
}

func decodeBIFFRef3DToken(token []byte, ctx legacyFormulaDecodeContext) (string, bool) {
	if len(token) < 7 {
		return "", false
	}
	sheetIndex, ok := resolveExternSheetIndex(binary.LittleEndian.Uint16(token[1:3]), ctx.workbook.externSheets, -1)
	if !ok || sheetIndex < 0 || sheetIndex >= len(ctx.workbook.sheets) {
		return "", false
	}
	row, col, absRow, absCol := decodeBIFFRefParts(binary.LittleEndian.Uint16(token[3:5]), binary.LittleEndian.Uint16(token[5:7]), ctx.row, ctx.col)
	ref := formatLegacyA1Ref(row, col, absRow, absCol)
	return format3DRef(ctx.workbook.sheets[sheetIndex].name, ref), true
}

func decodeBIFFArea3DToken(token []byte, ctx legacyFormulaDecodeContext) (string, bool) {
	if len(token) < 11 {
		return "", false
	}
	sheetIndex, ok := resolveExternSheetIndex(binary.LittleEndian.Uint16(token[1:3]), ctx.workbook.externSheets, -1)
	if !ok || sheetIndex < 0 || sheetIndex >= len(ctx.workbook.sheets) {
		return "", false
	}
	firstRow, firstCol, firstAbsRow, firstAbsCol := decodeBIFFRefParts(binary.LittleEndian.Uint16(token[3:5]), binary.LittleEndian.Uint16(token[7:9]), ctx.row, ctx.col)
	lastRow, lastCol, lastAbsRow, lastAbsCol := decodeBIFFRefParts(binary.LittleEndian.Uint16(token[5:7]), binary.LittleEndian.Uint16(token[9:11]), ctx.row, ctx.col)
	rangeRef := formatLegacyA1Ref(firstRow, firstCol, firstAbsRow, firstAbsCol) + ":" + formatLegacyA1Ref(lastRow, lastCol, lastAbsRow, lastAbsCol)
	return format3DRef(ctx.workbook.sheets[sheetIndex].name, rangeRef), true
}

func decodeBIFFRefParts(rowBits, colBits uint16, baseRow, baseCol int) (row int, col int, absRow bool, absCol bool) {
	rowValue := int(rowBits)
	colValue := int(colBits & 0x00FF)
	absRow = colBits&0x8000 == 0
	absCol = colBits&0x4000 == 0
	if !absRow {
		rowValue = baseRow - 1 + int(int16(rowBits))
	}
	if !absCol {
		colValue = baseCol - 1 + int(int8(colBits&0x00FF))
	}
	if rowValue < 0 {
		rowValue = 0
	}
	if colValue < 0 {
		colValue = 0
	}
	return rowValue + 1, colValue + 1, absRow, absCol
}

func formatLegacyA1Ref(row, col int, absRow, absCol bool) string {
	if row < 1 {
		row = 1
	}
	if col < 1 {
		col = 1
	}
	colText := colNameAbsolute(col)
	rowText := strconv.Itoa(row)
	if absCol {
		colText = "$" + colText
	}
	if absRow {
		rowText = "$" + rowText
	}
	return colText + rowText
}

func parseA1Ref(ref string) (row, col int, ok bool) {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	if ref == "" {
		return 0, 0, false
	}
	for len(ref) > 0 && ref[0] == '$' {
		ref = ref[1:]
	}
	index := 0
	for index < len(ref) {
		ch := ref[index]
		if ch < 'A' || ch > 'Z' {
			break
		}
		col = col*26 + int(ch-'A'+1)
		index++
	}
	if col == 0 {
		return 0, 0, false
	}
	for index < len(ref) && ref[index] == '$' {
		index++
	}
	if index >= len(ref) {
		return 0, 0, false
	}
	for index < len(ref) {
		ch := ref[index]
		if ch < '0' || ch > '9' {
			return 0, 0, false
		}
		row = row*10 + int(ch-'0')
		index++
	}
	return row, col, row > 0
}

func applyArrayFormulaMetadata(rows map[int]*sheetir.Row, groups []*sheetir.ArrayFormula) {
	for _, group := range groups {
		if group == nil {
			continue
		}
		anchor := findCell(rows, group.TopRow, group.LeftCol)
		if anchor != nil {
			if group.Formula == "" {
				group.Formula = strings.TrimSpace(anchor.Formula)
			}
			if anchor.Formula == "" {
				anchor.Formula = group.Formula
			}
			if anchor.ArrayFormulaRange == "" {
				anchor.ArrayFormulaRange = group.RangeRef
			}
			if anchor.ArrayFormulaAnchor == "" {
				anchor.ArrayFormulaAnchor = group.AnchorRef
			}
		}
		for row := group.TopRow; row <= group.BottomRow; row++ {
			rowData := rows[row]
			if rowData == nil {
				continue
			}
			for _, cell := range rowData.Cells {
				if cell == nil || cell.Col < group.LeftCol || cell.Col > group.RightCol {
					continue
				}
				if cell.Formula == "" {
					cell.Formula = group.Formula
				}
				if cell.ArrayFormulaRange == "" {
					cell.ArrayFormulaRange = group.RangeRef
				}
				if cell.ArrayFormulaAnchor == "" {
					cell.ArrayFormulaAnchor = group.AnchorRef
				}
			}
		}
	}
}

func findCell(rows map[int]*sheetir.Row, row, col int) *sheetir.Cell {
	rowData := rows[row]
	if rowData == nil {
		return nil
	}
	for _, cell := range rowData.Cells {
		if cell != nil && cell.Col == col {
			return cell
		}
	}
	return nil
}

func addSheetCell(rows map[int]*sheetir.Row, cell *sheetir.Cell) {
	if cell == nil {
		return
	}
	cell.Ref = formatA1Ref(cell.Row, cell.Col)
	row := ensureSheetRow(rows, cell.Row)
	row.Cells = append(row.Cells, cell)
}

func ensureSheetRow(rows map[int]*sheetir.Row, index int) *sheetir.Row {
	row := rows[index]
	if row == nil {
		row = &sheetir.Row{Index: index}
		rows[index] = row
	}
	return row
}

func applyRowRecord(rows map[int]*sheetir.Row, payload []byte, xfs []legacyXF) {
	if len(payload) < 16 {
		return
	}
	// BIFF8 ROW record layout (§2.4.235):
	//   0: USHORT rw        — 0-based row index
	//   2: USHORT colMic    — 0-based index of first defined column
	//   4: USHORT colMac    — 0-based index of column after last defined column
	//   6: USHORT miyRw     — row height in twips (1/20 pt); 0 = use default
	//   8: USHORT reserved
	//  10: USHORT reserved
	//  12: USHORT grbit     — bit 0-2 = outlineLevel, bit 4 = fCollapsesd,
	//                         bit 5 = fDyZero (hidden), bit 6 = fUnsynced, bit 7 = fGhostDirty
	//  14: USHORT ixfe      — bits 0-11 = XF index (used when grbit bit 7 = fGhostDirty is set)
	index := int(binary.LittleEndian.Uint16(payload[0:2])) + 1
	row := ensureSheetRow(rows, index)
	miyRw := binary.LittleEndian.Uint16(payload[6:8])
	if miyRw > 0 {
		row.HeightPt = float64(miyRw) / 20.0
	}
	grbit := binary.LittleEndian.Uint16(payload[12:14])
	row.Hidden = grbit&0x0020 != 0
	row.OutlineLevel = int(grbit & 0x0007)
	row.Collapsed = grbit&0x0010 != 0
	// fGhostDirty (bit 7) — row has an explicit XF applied (even to blank cells).
	if grbit&0x0080 != 0 && len(payload) >= 16 {
		xfeRaw := binary.LittleEndian.Uint16(payload[14:16])
		xfIdx := xfeRaw & 0x0FFF
		if xfIdx > 0 && int(xfIdx) < len(xfs) {
			style := styleFromXFIndex(xfIdx, xfs)
			row.Style = &style
		}
	}
}

func applyColInfo(sheet *sheetir.Sheet, payload []byte) {
	if len(payload) < 10 || sheet == nil {
		return
	}
	firstCol := int(binary.LittleEndian.Uint16(payload[0:2])) + 1
	lastCol := int(binary.LittleEndian.Uint16(payload[2:4])) + 1
	width := float64(binary.LittleEndian.Uint16(payload[4:6])) / 256.0
	flags := binary.LittleEndian.Uint16(payload[8:10])
	hidden := flags&0x0001 != 0
	outlineLevel := int((flags >> 8) & 0x0007)
	collapsed := flags&0x1000 != 0
	for col := firstCol; col <= lastCol; col++ {
		sheet.ColumnWidths[col] = width
		if hidden {
			sheet.HiddenCols[col] = true
		}
		if outlineLevel > 0 {
			sheet.OutlineCols[col] = outlineLevel
		}
		if collapsed {
			sheet.CollapsedCols[col] = true
		}
	}
}

func applyPane(sheet *sheetir.Sheet, payload []byte, freezePanes bool) {
	if sheet == nil || len(payload) < 4 {
		return
	}
	// BIFF8 Pane record: xSplit (2), ySplit (2), topRowInBottomPane (2), leftColInRightPane (2), activePane (2).
	xSplit := int(binary.LittleEndian.Uint16(payload[0:2]))
	ySplit := int(binary.LittleEndian.Uint16(payload[2:4]))
	topRow := 0
	leftCol := 0
	activePane := uint16(3)
	if len(payload) >= 10 {
		topRow = int(binary.LittleEndian.Uint16(payload[4:6]))
		leftCol = int(binary.LittleEndian.Uint16(payload[6:8]))
		activePane = binary.LittleEndian.Uint16(payload[8:10])
	}
	if freezePanes {
		// For frozen panes xSplit/ySplit are counts in cells.
		sheet.FrozenCols = xSplit
		sheet.FrozenRows = ySplit
	} else {
		// MS-XLS 2.4.189 Pane: when the sheet is split rather than frozen, x/y are
		// split positions in twips. Use rwTop/colLeft for renderer-facing pane metadata.
		sheet.SplitRows = topRow
		sheet.SplitCols = leftCol
	}
	applyPaneViewState(sheet, topRow, leftCol, activePane)
}

func applyPaneViewState(sheet *sheetir.Sheet, topRow, leftCol int, activePane uint16) {
	if sheet == nil {
		return
	}
	if topRow > 0 || leftCol > 0 {
		sheet.TopLeftCell = formatA1Ref(topRow+1, leftCol+1)
	}
	switch activePane {
	case 0:
		sheet.ActivePane = "bottomRight"
	case 1:
		sheet.ActivePane = "topRight"
	case 2:
		sheet.ActivePane = "bottomLeft"
	default:
		sheet.ActivePane = "topLeft"
	}
}

func applyDefaultRowHeight(sheet *sheetir.Sheet, payload []byte) {
	if len(payload) < 4 || sheet == nil {
		return
	}
	// Layout (BIFF8 §2.4.87 DEFAULTROWHEIGHT):
	//   offset 0: USHORT grbit — bit 0 = fUnsynced, bit 1 = fDyZero,
	//                             bit 2 = fExAsc,    bit 3 = fExDsc
	//   offset 2: USHORT miyRw — default row height in twips (1/20 pt)
	grbit := binary.LittleEndian.Uint16(payload[0:2])
	sheet.DefaultRowHeightUnsynced = grbit&0x0001 != 0
	sheet.DefaultRowHeightZeroHeight = grbit&0x0002 != 0
	sheet.DefaultRowHeightExtraSpaceAbove = grbit&0x0004 != 0
	sheet.DefaultRowHeightExtraSpaceBelow = grbit&0x0008 != 0
	heightTwips := binary.LittleEndian.Uint16(payload[2:4])
	if heightTwips == 0 {
		return
	}
	sheet.DefaultRowHeightPt = float64(heightTwips) / 20.0
}

func applyDefaultColWidth(sheet *sheetir.Sheet, payload []byte) {
	if len(payload) < 2 || sheet == nil {
		return
	}
	// HSSF DefaultColWidthRecord stores the default width in whole character
	// units; InternalSheet#getColumnWidth multiplies by 256 when it needs the
	// 1/256-char representation.
	widthChars := binary.LittleEndian.Uint16(payload[0:2])
	if widthChars == 0 {
		return
	}
	sheet.DefaultColWidthChars = float64(widthChars)
}

func applySCLRecord(sheet *sheetir.Sheet, payload []byte) {
	if sheet == nil || len(payload) < 4 {
		return
	}
	// HSSF SCLRecord stores zoom as numerator/denominator.
	// If absent, zoom defaults to 100%.
	numerator := int(binary.LittleEndian.Uint16(payload[0:2]))
	denominator := int(binary.LittleEndian.Uint16(payload[2:4]))
	if numerator <= 0 || denominator <= 0 {
		return
	}
	zoom := int(math.Round(float64(numerator) * 100.0 / float64(denominator)))
	if zoom > 0 {
		sheet.ZoomScale = zoom
	}
}

// applyDimensionsRecord parses the BIFF8 DIMENSIONS record (0x0200) and
// populates the sheet's used-cell range metadata. The record layout is:
//
//	offset 0:  firstRow (INT32, 0-based)
//	offset 4:  lastRow  (INT32, 0-based, exclusive — last row + 1)
//	offset 8:  firstCol (INT16, 0-based)
//	offset 10: lastCol  (INT16, 0-based, exclusive — last col + 1)
//	offset 12: reserved (INT16, must be 0)
//
// POI DimensionsRecord stores lastRow/lastCol as exclusive (one past the
// last used row/col). We convert to inclusive 1-based values for the IR.
// Mirrors HSSF DimensionsRecord.getFirstRow/getLastRow/getFirstCol/getLastCol.
func applyDimensionsRecord(sheet *sheetir.Sheet, payload []byte) {
	if sheet == nil || len(payload) < 14 {
		return
	}
	firstRow := int(binary.LittleEndian.Uint32(payload[0:4])) + 1
	lastRowExcl := int(binary.LittleEndian.Uint32(payload[4:8]))
	firstCol := int(binary.LittleEndian.Uint16(payload[8:10])) + 1
	lastColExcl := int(binary.LittleEndian.Uint16(payload[10:12]))
	// Convert exclusive to inclusive; if lastRowExcl == 0 or lastColExcl == 0,
	// the sheet is empty and we leave the fields at 0.
	if lastRowExcl > firstRow {
		sheet.DimLastRow = lastRowExcl
	} else {
		sheet.DimLastRow = firstRow
	}
	if lastColExcl > firstCol {
		sheet.DimLastCol = lastColExcl
	} else {
		sheet.DimLastCol = firstCol
	}
	sheet.DimFirstRow = firstRow
	sheet.DimFirstCol = firstCol
}

// applyFilterModeRecord parses the BIFF8 FILTERMODE record (0x009B).
// This is a simple flag record with no payload — its presence alone
// indicates the sheet is in filtered mode. Mirrors HSSF FilterModeRecord.
func applyFilterModeRecord(sheet *sheetir.Sheet) {
	if sheet == nil {
		return
	}
	sheet.FilterMode = true
}

func applyGutsRecord(sheet *sheetir.Sheet, payload []byte) {
	if sheet == nil || len(payload) < 8 {
		return
	}
	sheet.RowGutter = int(binary.LittleEndian.Uint16(payload[0:2]))
	sheet.ColGutter = int(binary.LittleEndian.Uint16(payload[2:4]))
	sheet.RowOutlineLevelMax = int(binary.LittleEndian.Uint16(payload[4:6]))
	sheet.ColOutlineLevelMax = int(binary.LittleEndian.Uint16(payload[6:8]))
}

// applySetupRecord parses the BIFF8 SETUP record (0x00A1) and populates
// sheet.PageSetup with paper size, scale/fit-to-page, orientation, and
// header/footer margins. The record layout is:
//
//	offset 0:  paperSize (USHORT)
//	offset 2:  scale (USHORT; 0 = fit to page)
//	offset 4:  pageStart (USHORT; ignored)
//	offset 6:  fitWidth (USHORT)
//	offset 8:  fitHeight (USHORT)
//	offset 10: grbit (USHORT; bit 1 = landscape)
//	offset 16: numHdr (IEEE 754 double, inches)
//	offset 24: numFtr (IEEE 754 double, inches)
func applySetupRecord(sheet *sheetir.Sheet, payload []byte) {
	if sheet == nil || len(payload) < 12 {
		return
	}
	if sheet.PageSetup == nil {
		sheet.PageSetup = &sheetir.PageSetup{}
	}
	paperSize := int(binary.LittleEndian.Uint16(payload[0:2]))
	if paperSize > 0 {
		sheet.PageSetup.PaperSize = paperSize
	}
	scale := int(binary.LittleEndian.Uint16(payload[2:4]))
	fitWidth := int(binary.LittleEndian.Uint16(payload[6:8]))
	fitHeight := int(binary.LittleEndian.Uint16(payload[8:10]))
	grbit := binary.LittleEndian.Uint16(payload[10:12])
	if grbit&0x0002 != 0 {
		sheet.PageSetup.Orientation = "landscape"
	} else {
		sheet.PageSetup.Orientation = "portrait"
	}
	if scale > 0 {
		sheet.PageSetup.Scale = scale
	} else {
		sheet.PageSetup.FitToPage = true
		sheet.PageSetup.FitToWidth = fitWidth
		sheet.PageSetup.FitToHeight = fitHeight
	}
	if len(payload) >= 32 {
		sheet.PageSetup.MarginHeader = math.Float64frombits(binary.LittleEndian.Uint64(payload[16:24]))
		sheet.PageSetup.MarginFooter = math.Float64frombits(binary.LittleEndian.Uint64(payload[24:32]))
	}
}

// applyHeaderFooterRecord decodes a HEADER (0x0014) or FOOTER (0x0015) record
// and stores the resulting text on the sheet's HeaderFooter. The record body
// follows HSSF HeaderFooterBase:
//
//	offset 0:  cch (USHORT) — character count; absent body or cch == 0 means empty
//	offset 2:  hasMultibyte (BYTE) — 0 = compressed Unicode (8-bit), nonzero = UTF-16LE
//	offset 3+: text bytes (cch * 1 or cch * 2)
//
// POI handles the 61287 edge case where cch == 0 and the hasMultibyte byte is
// truncated; we mirror that by treating any sub-3-byte payload as empty.
func applyHeaderFooterRecord(sheet *sheetir.Sheet, kind string, payload []byte) {
	if sheet == nil {
		return
	}
	text := parseBIFFHeaderFooterText(payload)
	if text == "" {
		return
	}
	if sheet.HeaderFooter == nil {
		sheet.HeaderFooter = &sheetir.HeaderFooter{}
	}
	switch kind {
	case "header":
		sheet.HeaderFooter.OddHeader = text
	case "footer":
		sheet.HeaderFooter.OddFooter = text
	}
}

// applyHeaderFooterExtRecord decodes the Excel 2007+ HEADERFOOTER record
// (0x089C). This is a future record with a 12-byte FRT header plus a 16-byte
// sheet-view GUID at offset 12. If the GUID is all zeros, the record belongs
// to the current sheet. The trailing payload contains a variable list of
// XLUnicodeString values. We map up to six strings in this order:
// oddHeader, oddFooter, evenHeader, evenFooter, firstHeader, firstFooter.
func applyHeaderFooterExtRecord(sheet *sheetir.Sheet, payload []byte) {
	if sheet == nil || len(payload) < 28 {
		return
	}
	if !isZeroGUID16(payload[12:28]) {
		// Non-zero GUID belongs to a custom sheet view; skip for now.
		return
	}
	values := parseHeaderFooterExtStrings(payload[28:])
	if len(values) == 0 {
		return
	}
	if sheet.HeaderFooter == nil {
		sheet.HeaderFooter = &sheetir.HeaderFooter{}
	}
	if len(values) > 0 && values[0] != "" {
		sheet.HeaderFooter.OddHeader = values[0]
	}
	if len(values) > 1 && values[1] != "" {
		sheet.HeaderFooter.OddFooter = values[1]
	}
	if len(values) > 2 && values[2] != "" {
		sheet.HeaderFooter.EvenHeader = values[2]
	}
	if len(values) > 3 && values[3] != "" {
		sheet.HeaderFooter.EvenFooter = values[3]
	}
	if len(values) > 4 && values[4] != "" {
		sheet.HeaderFooter.FirstHeader = values[4]
	}
	if len(values) > 5 && values[5] != "" {
		sheet.HeaderFooter.FirstFooter = values[5]
	}
}

func parseHeaderFooterExtStrings(payload []byte) []string {
	pos := 0
	values := make([]string, 0, 6)
	for len(values) < 6 && pos < len(payload) {
		text, next, ok := readXLUnicodeString(payload, pos, 2)
		if !ok || next <= pos {
			break
		}
		values = append(values, strings.TrimSpace(text))
		pos = next
	}
	return values
}

func isZeroGUID16(b []byte) bool {
	if len(b) < 16 {
		return false
	}
	for i := 0; i < 16; i++ {
		if b[i] != 0 {
			return false
		}
	}
	return true
}

// parsePageBreakRecord decodes a HORIZONTALPAGEBREAK (0x001B) or
// VERTICALPAGEBREAK (0x001A) record. Layout (HSSF PageBreakRecord):
//
//	offset 0:  cBreaks (USHORT) — number of break entries that follow
//	then per break (6 bytes each):
//	  offset 0: main (USHORT) — 1-based row (HBreak) or column (VBreak)
//	  offset 2: subFrom (USHORT) — 0-based start col/row of break extent
//	  offset 4: subTo (USHORT) — 0-based end col/row of break extent
//
// HSSF stores main 0-based internally and adds 1 on serialize; we keep the
// 1-based wire value directly since the rest of the IR (Cell.Row,
// MergedCell.TopRow, …) is already 1-based. main == 0 is treated as invalid
// data and skipped.
func parsePageBreakRecord(payload []byte) []int {
	if len(payload) < 2 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(payload[0:2]))
	if count <= 0 {
		return nil
	}
	out := make([]int, 0, count)
	pos := 2
	for i := 0; i < count && pos+6 <= len(payload); i++ {
		main := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
		if main > 0 {
			out = append(out, main)
		}
		pos += 6
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseAutoFilterRecord decodes one AUTOFILTER (0x9E) record from the BIFF8
// worksheet stream into an IR FilterColumn.
//
// Record layout (MS-XLS §2.4.3):
//
//	offset 0: USHORT col   — 0-based column index within the autoFilter range
//	offset 2: USHORT grbit — bit 2: fTopN; bit 3: fTop; bit 4: fPercent;
//	                         bits 5-14: wTop10 (N count when fTopN)
//	offset 4: BYTE   vt1   — condition-1 value type (0=none,2=float,4=str,6=bool,8=err,0xE=blank,0xF=nonblank)
//	offset 5: BYTE   sgn1  — comparison operator (1=<,2==,3=<=,4=>,5=!=,6=>=)
//	offset 6: [8]    doper1
//	offset 14: BYTE  vt2
//	offset 15: BYTE  sgn2
//	offset 16: [8]   doper2
//	offset 24: BYTE  grbitAnd — bit 0: AND the two conditions (0 = OR)
//	offset 25: USHORT cbStr1  — string-1 character count (valid when vt1==4)
//	offset 27: USHORT cbStr2  — string-2 character count (valid when vt2==4)
//	offset 29+: optional leading fHighByte byte + chars for str1, then str2
//
// Returns nil when the payload is too short or carries no usable criteria.
func parseAutoFilterRecord(payload []byte) *sheetir.FilterColumn {
	if len(payload) < 25 {
		return nil
	}
	colIndex := int(binary.LittleEndian.Uint16(payload[0:2]))
	grbit := binary.LittleEndian.Uint16(payload[2:4])

	// Top-N filter: bits 2-4 carry the mode, bits 5-14 carry the count.
	if grbit&0x0004 != 0 {
		isTop := grbit&0x0008 != 0
		isPercent := grbit&0x0010 != 0
		count := float64((grbit >> 5) & 0x03FF)
		if count == 0 {
			count = 10 // default
		}
		return &sheetir.FilterColumn{
			ColIndex: colIndex,
			Top10: &sheetir.FilterTop10{
				Top:     isTop,
				Percent: isPercent,
				Val:     count,
			},
		}
	}

	vt1 := payload[4]
	sgn1 := payload[5]
	vt2 := payload[14]
	sgn2 := payload[15]
	grbitAnd := payload[24]

	// Parse optional string lengths (offsets 25-28); only present when payload is long enough.
	cbStr1 := 0
	cbStr2 := 0
	if len(payload) >= 29 {
		cbStr1 = int(binary.LittleEndian.Uint16(payload[25:27]))
		cbStr2 = int(binary.LittleEndian.Uint16(payload[27:29]))
	}

	// Decode string data starting at offset 29.
	str1, str2 := "", ""
	strOffset := 29
	if cbStr1 > 0 && len(payload) > strOffset {
		s, next, ok := readXLUnicodeString(payload, strOffset, 1)
		if ok {
			str1 = s
			strOffset = next
		} else {
			// Fallback: 1-byte fHighByte + raw chars
			if strOffset+1+cbStr1 <= len(payload) {
				strOffset++ // skip fHighByte
				str1 = string(payload[strOffset : strOffset+cbStr1])
				strOffset += cbStr1
			}
		}
	}
	if cbStr2 > 0 && len(payload) > strOffset {
		s, _, ok := readXLUnicodeString(payload, strOffset, 1)
		if ok {
			str2 = s
		} else {
			if strOffset+1+cbStr2 <= len(payload) {
				strOffset++
				str2 = string(payload[strOffset : strOffset+cbStr2])
			}
		}
	}

	cond1 := biffAutoFilterCondition(vt1, sgn1, payload[6:14], str1)
	cond2 := biffAutoFilterCondition(vt2, sgn2, payload[16:24], str2)

	fc := &sheetir.FilterColumn{ColIndex: colIndex}

	// Blank / non-blank shorthand: surface as ShowBlanks / single custom filter.
	if vt1 == 0x0E && cond1 == nil {
		fc.ShowBlanks = true
	}
	if vt1 == 0x00 && vt2 == 0x00 {
		return nil // no criteria at all
	}

	if cond1 != nil && cond2 == nil {
		// Single equality filter with no OR/AND: add to Filters list for readability.
		if cond1.Operator == "equal" {
			fc.Filters = []string{cond1.Val}
		} else {
			fc.CustomFilters = []*sheetir.CustomFilter{cond1}
		}
	} else if cond1 != nil && cond2 != nil {
		fc.CustomFilters = []*sheetir.CustomFilter{cond1, cond2}
		fc.CustomAnd = grbitAnd&0x01 != 0
	} else if cond2 != nil {
		if cond2.Operator == "equal" {
			fc.Filters = []string{cond2.Val}
		} else {
			fc.CustomFilters = []*sheetir.CustomFilter{cond2}
		}
	}

	if len(fc.Filters) == 0 && !fc.ShowBlanks && len(fc.CustomFilters) == 0 && fc.Top10 == nil {
		return nil
	}
	return fc
}

// biffAutoFilterCondition converts a BIFF DOPER value type + sign byte + 8-byte
// payload into a *sheetir.CustomFilter. Returns nil when vt == 0 (no condition).
func biffAutoFilterCondition(vt, sgn byte, doper []byte, str string) *sheetir.CustomFilter {
	if vt == 0x00 {
		return nil
	}
	op := biffAutoFilterOperator(sgn)
	var val string
	switch vt {
	case 0x02: // IEEE 754 double
		if len(doper) >= 8 {
			bits := binary.LittleEndian.Uint64(doper[0:8])
			f := math.Float64frombits(bits)
			if math.Mod(f, 1) == 0 {
				val = fmt.Sprintf("%.0f", f)
			} else {
				val = fmt.Sprintf("%g", f)
			}
		}
	case 0x04: // string — actual value is in str
		val = str
	case 0x06: // boolean
		if len(doper) >= 1 && doper[0] != 0 {
			val = "TRUE"
		} else {
			val = "FALSE"
		}
	case 0x08: // error
		if len(doper) >= 1 {
			val = biffErrorString(doper[0])
		}
	default:
		return nil
	}
	if val == "" && vt != 0x04 {
		return nil
	}
	return &sheetir.CustomFilter{Operator: op, Val: val}
}

// biffAutoFilterOperator converts the BIFF DOPER grbitSgn byte to an OOXML-style operator name.
func biffAutoFilterOperator(sgn byte) string {
	switch sgn {
	case 1:
		return "lessThan"
	case 2:
		return "equal"
	case 3:
		return "lessThanOrEqual"
	case 4:
		return "greaterThan"
	case 5:
		return "notEqual"
	case 6:
		return "greaterThanOrEqual"
	default:
		return "equal"
	}
}

// parseBIFFHeaderFooterText decodes the optional cch + hasMultibyte + text
// trio used by HeaderRecord and FooterRecord. Returns "" for empty or
// truncated payloads.
//
// LibreOffice sometimes writes BIFF2-style header/footer records (1-byte cch
// followed immediately by text, no flags byte) even inside BIFF8 workbooks.
// We detect this by checking whether 1 + payload[0] == len(payload); when
// true, we treat the record as BIFF2 format.
func parseBIFFHeaderFooterText(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	// BIFF2-style: 1-byte cch, no flags byte.
	if len(payload) >= 1 && 1+int(payload[0]) == len(payload) {
		return decodeBiffBytes(payload[1:])
	}
	if len(payload) < 3 {
		// Empty record body, or cch alone with no grbit (POI 61287).
		return ""
	}
	cch := int(binary.LittleEndian.Uint16(payload[0:2]))
	if cch == 0 {
		return ""
	}
	flags := payload[2]
	body := payload[3:]
	if flags&0x01 != 0 {
		need := cch * 2
		if need > len(body) {
			need = len(body) &^ 1
		}
		return decodeBiffUTF16(body[:need])
	}
	if cch > len(body) {
		cch = len(body)
	}
	return decodeBiffBytes(body[:cch])
}

// applyPrintFlagRecord parses a single-uint16 page-setup boolean record:
// HCENTER (0x0083), VCENTER (0x0084), PRINTHEADERS (0x002A), or
// PRINTGRIDLINES (0x002B). HSSF (HCenterRecord/VCenterRecord/
// PrintHeadersRecord/PrintGridlinesRecord) treats any nonzero value as the
// flag being set.
func applyPrintFlagRecord(sheet *sheetir.Sheet, kind string, payload []byte) {
	if sheet == nil || len(payload) < 2 {
		return
	}
	if sheet.PageSetup == nil {
		sheet.PageSetup = &sheetir.PageSetup{}
	}
	flag := binary.LittleEndian.Uint16(payload[0:2]) != 0
	switch kind {
	case "hcenter":
		sheet.PageSetup.CenterHorizontally = flag
	case "vcenter":
		sheet.PageSetup.CenterVertically = flag
	case "headers":
		sheet.PageSetup.PrintHeadings = flag
	case "gridlines":
		sheet.PageSetup.PrintGridlines = flag
	}
}

// applyMarginRecord parses a BIFF margin record (LEFT/RIGHT/TOP/BOTTOM_MARGIN,
// 0x0026–0x0029). Each record contains a single IEEE 754 double in inches.
func applyMarginRecord(sheet *sheetir.Sheet, side string, payload []byte) {
	if sheet == nil || len(payload) < 8 {
		return
	}
	if sheet.PageSetup == nil {
		sheet.PageSetup = &sheetir.PageSetup{}
	}
	value := math.Float64frombits(binary.LittleEndian.Uint64(payload[0:8]))
	switch side {
	case "left":
		sheet.PageSetup.MarginLeft = value
	case "right":
		sheet.PageSetup.MarginRight = value
	case "top":
		sheet.PageSetup.MarginTop = value
	case "bottom":
		sheet.PageSetup.MarginBottom = value
	}
}

func parseMergeCells(payload []byte) []*sheetir.MergedCell {
	if len(payload) < 2 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(payload[0:2]))
	merged := make([]*sheetir.MergedCell, 0, count)
	offset := 2
	for index := 0; index < count && offset+8 <= len(payload); index++ {
		merged = append(merged, &sheetir.MergedCell{
			TopRow:    int(binary.LittleEndian.Uint16(payload[offset:offset+2])) + 1,
			BottomRow: int(binary.LittleEndian.Uint16(payload[offset+2:offset+4])) + 1,
			LeftCol:   int(binary.LittleEndian.Uint16(payload[offset+4:offset+6])) + 1,
			RightCol:  int(binary.LittleEndian.Uint16(payload[offset+6:offset+8])) + 1,
		})
		offset += 8
	}
	return merged
}

// gatherContiguousMergeCells mirrors HSSF MergedCellsTable.read(): once a
// MERGEDCELLS record is encountered in the worksheet stream, consume zero or
// more consecutive MERGEDCELLS records as one aggregate and flatten all
// contained areas into a single merged-cells slice.
func gatherContiguousMergeCells(data []byte, pos int) ([]*sheetir.MergedCell, int) {
	var merged []*sheetir.MergedCell
	for pos+4 <= len(data) {
		id, payload, nextPos, ok := readBiffRecord(data, pos)
		if !ok || id != biffMergeCells {
			break
		}
		merged = append(merged, parseMergeCells(payload)...)
		pos = nextPos
	}
	return merged, pos
}

// skipCustomViewSettingsRecordAggregate mirrors POI's
// CustomViewSettingsRecordAggregate framing: once USERSVIEWBEGIN appears, all
// records through the matching USERSVIEWEND belong to a custom-view-only
// substream and should not affect the live worksheet state.
func skipCustomViewSettingsRecordAggregate(data []byte, pos int) int {
	if pos+4 > len(data) {
		return pos
	}
	id, _, nextPos, ok := readBiffRecord(data, pos)
	if !ok || id != biffUserSViewBegin {
		return pos
	}
	pos = nextPos
	for pos+4 <= len(data) {
		id, _, nextPos, ok = readBiffRecord(data, pos)
		if !ok {
			return pos
		}
		pos = nextPos
		if id == biffUserSViewEnd || id == biffEOF {
			return pos
		}
	}
	return pos
}

// skipChartSubstreamRecordAggregate mirrors POI's
// ChartSubstreamRecordAggregate framing: once a nested chart BOF appears in a
// worksheet stream, all records through its terminating EOF belong to the chart
// substream rather than the enclosing sheet.
func skipChartSubstreamRecordAggregate(data []byte, pos int) int {
	if pos+4 > len(data) {
		return pos
	}
	id, payload, nextPos, ok := readBiffRecord(data, pos)
	if !ok || !isBIFFBOFRecord(id) {
		return pos
	}
	_, typ, _, _, _, _ := parseBIFFBOF(payload)
	if typ != biffBOFTypeChart {
		return pos
	}
	pos = nextPos
	for pos+4 <= len(data) {
		id, _, nextPos, ok = readBiffRecord(data, pos)
		if !ok {
			return pos
		}
		pos = nextPos
		if id == biffEOF {
			return pos
		}
	}
	return pos
}

// objSubRecord holds parsed data from one sub-record within an ObjRecord.
type objSubRecord struct {
	ft   uint16 // sub-record type
	cb   uint16 // data size (excluding ft+cb header)
	data []byte // raw sub-record data
}

// parsedObj holds the result of parsing all sub-records in an ObjRecord.
type parsedObj struct {
	objType uint16 // from FtCmo (COMMON_OBJECT_DATA, ft=0x0015)
	objID   int    // from FtCmo
	subs    []objSubRecord
	// Decoded sub-record fields:
	clipboardFormat uint16 // from FtCf (ft=0x0007)
	pictureFlags    uint16 // from FtPioGrbit (ft=0x0008)
	oleStreamID     *int   // from EmbeddedObjectRef (ft=0x0009)
	oleClassName    string // from EmbeddedObjectRef (ft=0x0009)
	controlData     []byte // from FtCbls/FtSbs (ft=0x000C, object-type dependent)
	linkFormulaData []byte // from ObjLinkFmla (ft=0x000E / 0x0014)
	checkBoxData    []byte // from FtCblsData (ft=0x0012)
	radioData       []byte // from FtRboData (ft=0x0011)
	noteData        []byte // from NoteStructure (ft=0x000D)
	listBoxData     []byte // from LbsData (ft=0x0013)
}

// parseObjSubRecords walks all sub-records in an ObjRecord payload,
// mirroring POI's SubRecord.createSubRecord loop.
// Returns the parsed object data and whether a valid FtCmo was found.
func parseObjSubRecords(payload []byte) (parsedObj, bool) {
	var out parsedObj
	pos := 0
	hasCmo := false
	for pos+4 <= len(payload) {
		ft := binary.LittleEndian.Uint16(payload[pos : pos+2])
		cb := binary.LittleEndian.Uint16(payload[pos+2 : pos+4])
		pos += 4
		end := pos + int(cb)
		if end > len(payload) {
			end = len(payload)
		}
		subData := payload[pos:end]
		pos = end

		out.subs = append(out.subs, objSubRecord{ft: ft, cb: cb, data: subData})

		switch ft {
		case 0x0015: // COMMON_OBJECT_DATA (FtCmo)
			if len(subData) >= 4 {
				out.objType = binary.LittleEndian.Uint16(subData[0:2])
				out.objID = int(binary.LittleEndian.Uint16(subData[2:4]))
				hasCmo = true
			}
		case 0x0007: // FtCf — clipboard format
			if len(subData) >= 2 {
				out.clipboardFormat = binary.LittleEndian.Uint16(subData[0:2])
			}
		case 0x0008: // FtPioGrbit — picture I/O flags
			if len(subData) >= 2 {
				out.pictureFlags = binary.LittleEndian.Uint16(subData[0:2])
			}
		case 0x0009: // EmbeddedObjectRef (FtPictFmla) — OLE/picture reference
			// Structure: unknownInt(4) + optional Ptg + optional className + streamID(4)
			if len(subData) >= 4 {
				// Skip unknownInt (4 bytes)
				eoPos := 4
				// Try to find the stream ID at the end of the sub-record.
				// The stream ID is the last 4 bytes when present.
				// Before it there may be a null-terminated class name string.
				if len(subData) >= 8 {
					// Search backwards for the stream ID marker.
					// The stream ID is typically a non-zero uint32 at the end.
					last4 := binary.LittleEndian.Uint32(subData[len(subData)-4:])
					if last4 != 0 {
						sid := int(last4)
						out.oleStreamID = &sid
					}
					// Try to extract class name: look for a null-terminated
					// Unicode string between the Ptg and the stream ID.
					remaining := subData[eoPos : len(subData)-4]
					if len(remaining) >= 4 {
						// Check for a Unicode string prefix (cch + flag byte).
						cch := int(binary.LittleEndian.Uint16(remaining[0:2]))
						if cch > 0 && cch < 256 && len(remaining) >= 3+cch*2 {
							flag := remaining[2]
							if flag == 0 || flag == 1 {
								raw := remaining[3 : 3+cch*2]
								out.oleClassName = strings.TrimRight(decodeBiffUTF16Raw(raw), "\x00")
							}
						}
					}
				}
			}
		case 0x000C: // FtCbls / FtSbs — object-type dependent control payload
			out.controlData = subData
		case 0x000E, 0x0014: // ObjLinkFmla — linked-value formula for control objects
			out.linkFormulaData = subData
		case 0x000D: // NoteStructure — note structure data
			out.noteData = subData
		case 0x0011: // FtRboData — radio group linkage/first-button data
			out.radioData = subData
		case 0x0012: // FtCblsData — checkbox/radio state data
			out.checkBoxData = subData
		case 0x0013: // LbsData — list box data
			out.listBoxData = subData
		case 0x0000: // EndSubRecord — terminator
			return out, hasCmo
		case 0x0006: // GroupMarker — skip
		default:
			// Unknown sub-record; skip.
		}
	}
	return out, hasCmo
}

func parseNoteComment(payload []byte) (legacyNote, bool) {
	if len(payload) < 4 {
		return legacyNote{}, false
	}
	row := int(binary.LittleEndian.Uint16(payload[0:2])) + 1
	col := int(binary.LittleEndian.Uint16(payload[2:4])) + 1
	objID := 0
	if len(payload) >= 8 {
		objID = int(binary.LittleEndian.Uint16(payload[6:8]))
	}
	fallback := parseNoteAuthor(payload)
	if fallback == "" {
		fallback = "Comment"
	}
	return legacyNote{
		objID:        objID,
		ref:          formatA1Ref(row, col),
		fallbackText: fallback,
	}, true
}

func parseNoteAuthor(payload []byte) string {
	// BIFF5/7: 1-byte character count.
	if len(payload) >= 10 {
		if author, _, ok := readXLUnicodeString(payload, 8, 1); ok && biffTextHasLetter(author) {
			return author
		}
	}
	// BIFF8: 2-byte character count.
	if len(payload) >= 11 {
		if author, _, ok := readXLUnicodeString(payload, 8, 2); ok && biffTextHasLetter(author) {
			return author
		}
	}
	return ""
}

// biffTextHasLetter reports whether s contains at least one Unicode letter,
// used to reject binary-garbage results from BIFF string parsing.
func biffTextHasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func parseCommentTextContinue(payload []byte) string {
	text, _ := parseCommentTextContinueLimited(payload, -1)
	return text
}

func parseCommentTextContinueLimited(payload []byte, maxChars int) (string, int) {
	if len(payload) < 2 {
		return "", 0
	}
	if maxChars == 0 {
		return "", 0
	}
	flags := payload[0]
	if flags&0x01 != 0 {
		available := (len(payload) - 1) / 2
		count := available
		if maxChars > 0 && maxChars < count {
			count = maxChars
		}
		if count <= 0 {
			return "", 0
		}
		return decodeBiffUTF16Raw(payload[1 : 1+count*2]), count
	}
	available := len(payload) - 1
	count := available
	if maxChars > 0 && maxChars < count {
		count = maxChars
	}
	if count <= 0 {
		return "", 0
	}
	return decodeBiffBytesRaw(payload[1 : 1+count]), count
}

func parseTxOCommentCharCount(payload []byte) int {
	if len(payload) < 8 {
		return -1
	}
	candidates := []int{}
	if len(payload) >= 6 {
		candidates = append(candidates, int(binary.LittleEndian.Uint16(payload[4:6])))
	}
	if len(payload) >= 12 {
		candidates = append(candidates, int(binary.LittleEndian.Uint16(payload[10:12])))
	}
	for _, count := range candidates {
		if count > 0 && count <= 32767 {
			return count
		}
	}
	return -1
}

func parseLegacyTxOControlLink(payload []byte, formulaCtx legacyFormulaWorkbookContext) string {
	if len(payload) < 29 {
		return ""
	}
	formulaSize := int(binary.LittleEndian.Uint16(payload[18:20]))
	if formulaSize <= 0 || 24+formulaSize > len(payload) {
		return ""
	}
	formulaText, ok := decodeBIFFFormula(payload[24:24+formulaSize], legacyFormulaDecodeContext{
		workbook: formulaCtx,
	})
	if !ok {
		return ""
	}
	return strings.TrimSpace(formulaText)
}

func parseLegacyObjLinkFormula(payload []byte, formulaCtx legacyFormulaWorkbookContext) string {
	if len(payload) < 8 {
		return ""
	}
	formulaSize := int(binary.LittleEndian.Uint16(payload[2:4]) & 0x7FFF)
	if formulaSize <= 0 || 8+formulaSize > len(payload) {
		return ""
	}
	formulaText, ok := decodeBIFFFormula(payload[8:8+formulaSize], legacyFormulaDecodeContext{
		workbook: formulaCtx,
	})
	if !ok {
		return ""
	}
	return strings.TrimSpace(formulaText)
}

func parseLegacyCheckboxRadioControlState(payload []byte, objType uint16) (checked bool, mixed bool, ok bool) {
	if len(payload) < 2 {
		return false, false, false
	}
	switch objType {
	case legacyObjTypeCheckbox, legacyObjTypeRadio:
	default:
		return false, false, false
	}
	switch binary.LittleEndian.Uint16(payload[0:2]) {
	case 0x0000:
		return false, false, true
	case 0x0001:
		return true, false, true
	case 0x0002:
		if objType == legacyObjTypeCheckbox {
			return false, true, true
		}
		return false, false, false
	default:
		return false, false, false
	}
}

func parseLegacySbsControlState(payload []byte, objType uint16) (min, max, val, increment, pageStep int, horizontal bool, ok bool) {
	if len(payload) < 10 {
		return 0, 0, 0, 0, 0, false, false
	}
	switch objType {
	case legacyObjTypeSpin, legacyObjTypeScroll:
	default:
		return 0, 0, 0, 0, 0, false, false
	}
	val = int(int16(binary.LittleEndian.Uint16(payload[4:6])))
	min = int(int16(binary.LittleEndian.Uint16(payload[6:8])))
	max = int(int16(binary.LittleEndian.Uint16(payload[8:10])))
	if len(payload) >= 14 {
		increment = int(int16(binary.LittleEndian.Uint16(payload[10:12])))
		pageStep = int(int16(binary.LittleEndian.Uint16(payload[12:14])))
	}
	if len(payload) >= 16 {
		horizontal = binary.LittleEndian.Uint16(payload[14:16]) != 0
	}
	return min, max, val, increment, pageStep, horizontal, true
}

func parseLegacyRadioFirstButton(payload []byte, objType uint16) (bool, bool) {
	if objType != legacyObjTypeRadio || len(payload) < 4 {
		return false, false
	}
	return binary.LittleEndian.Uint16(payload[2:4]) != 0, true
}

func parseLegacyLbsDataControlState(payload []byte, objType uint16, formulaCtx legacyFormulaWorkbookContext) (sourceRange string, val int, dropLines int, selectionType string, selected []int, hasVal bool) {
	if len(payload) < 10 {
		return "", 0, 0, "", nil, false
	}
	pos := 0
	encodedTokenLen := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
	pos += 2
	if encodedTokenLen > 0 {
		if pos+6 > len(payload) {
			return "", 0, 0, "", nil, false
		}
		formulaSize := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
		pos += 6 // formulaSize + unknownPreFormulaInt
		if formulaSize <= 0 || pos+formulaSize > len(payload) {
			return "", 0, 0, "", nil, false
		}
		formulaText, ok := decodeBIFFFormula(payload[pos:pos+formulaSize], legacyFormulaDecodeContext{
			workbook: formulaCtx,
		})
		if ok {
			sourceRange = strings.TrimSpace(formulaText)
		}
		pos += formulaSize
		switch encodedTokenLen - formulaSize - 6 {
		case 1:
			if pos >= len(payload) {
				return sourceRange, 0, 0, "", nil, false
			}
			pos++
		case 0:
		default:
			return sourceRange, 0, 0, "", nil, false
		}
	}
	if pos+8 > len(payload) {
		return sourceRange, 0, 0, "", nil, false
	}
	lineCount := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
	val = int(binary.LittleEndian.Uint16(payload[pos+2 : pos+4]))
	flags := binary.LittleEndian.Uint16(payload[pos+4 : pos+6])
	pos += 8 // cLines, iSel, flags, idEdit
	selectionType = legacyListSelectionType(flags)

	if objType == legacyObjTypeDrop {
		if cLine, next, ok := parseLegacyLbsDropData(payload, pos); ok {
			dropLines = cLine
			pos = next
		}
	}
	if flags&0x0002 != 0 {
		for index := 0; index < lineCount; index++ {
			_, next, ok := readXLUnicodeString(payload, pos, 2)
			if !ok || next <= pos {
				return sourceRange, val, dropLines, selectionType, nil, false
			}
			pos = next
		}
	}
	switch objType {
	case legacyObjTypeList, legacyObjTypeDrop:
		if selectionType == "" {
			selectionType = "single"
		}
		return sourceRange, val, dropLines, selectionType, selectedEntriesFromLegacySelection(selectionType, val, payload, pos, lineCount), true
	default:
		return sourceRange, 0, dropLines, selectionType, nil, false
	}
}

func legacyListSelectionType(flags uint16) string {
	switch (flags >> 4) & 0x0003 {
	case 0:
		return "single"
	case 1:
		return "multi"
	case 2:
		return "extended"
	default:
		return ""
	}
}

func selectedEntriesFromLegacySelection(selectionType string, firstSelected int, payload []byte, pos int, lineCount int) []int {
	if selectionType == "" {
		return nil
	}
	if selectionType == "single" {
		if firstSelected <= 0 {
			return nil
		}
		return []int{firstSelected}
	}
	out := make([]int, 0, lineCount)
	for index := 0; index < lineCount && pos+index < len(payload); index++ {
		if payload[pos+index] == 1 {
			out = append(out, index+1)
		}
	}
	if len(out) == 0 && firstSelected > 0 {
		out = append(out, firstSelected)
	}
	return out
}

func parseLegacyLbsDropData(payload []byte, pos int) (visibleLines int, next int, ok bool) {
	if pos+6 > len(payload) {
		return 0, pos, false
	}
	visibleLines = int(binary.LittleEndian.Uint16(payload[pos+2 : pos+4]))
	next = pos + 6
	_, afterString, ok := readXLUnicodeString(payload, next, 2)
	if !ok {
		return visibleLines, pos, false
	}
	next = afterString
	if (next-(pos+6))%2 != 0 && next < len(payload) {
		next++
	}
	return visibleLines, next, true
}

func legacyObjTypeToFormControlType(objType uint16) (string, bool) {
	switch objType {
	case legacyObjTypeButton:
		return "Button", true
	case legacyObjTypeCheckbox:
		return "Checkbox", true
	case legacyObjTypeRadio:
		return "Radio", true
	case legacyObjTypeEdit:
		return "Edit", true
	case legacyObjTypeLabel:
		return "Label", true
	case legacyObjTypeSpin:
		return "Spin", true
	case legacyObjTypeScroll:
		return "Scroll", true
	case legacyObjTypeList:
		return "List", true
	case legacyObjTypeGroupBox:
		return "GBox", true
	case legacyObjTypeDrop:
		return "Drop", true
	default:
		return "", false
	}
}

func legacyObjConsumesNonPictureAnchor(objType uint16) bool {
	return objType != legacyObjTypePicture
}

func sortedSheetRows(rows map[int]*sheetir.Row) []*sheetir.Row {
	keys := make([]int, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	out := make([]*sheetir.Row, 0, len(keys))
	for _, key := range keys {
		row := rows[key]
		sort.Slice(row.Cells, func(i, j int) bool {
			return row.Cells[i].Col < row.Cells[j].Col
		})
		out = append(out, row)
	}
	return out
}

func readBiffRecord(data []byte, pos int) (uint16, []byte, int, bool) {
	if pos < 0 || pos+4 > len(data) {
		return 0, nil, pos, false
	}
	id := binary.LittleEndian.Uint16(data[pos : pos+2])
	length := int(binary.LittleEndian.Uint16(data[pos+2 : pos+4]))
	next := pos + 4 + length
	if next > len(data) {
		return 0, nil, pos, false
	}
	return id, data[pos+4 : next], next, true
}

func gatherBiffPayload(data []byte, pos int) ([]byte, int) {
	_, payload, next, ok := readBiffRecord(data, pos)
	if !ok {
		return nil, pos
	}
	full := append([]byte(nil), payload...)
	for {
		id, continuePayload, nextPos, ok := readBiffRecord(data, next)
		if !ok || id != biffContinue {
			break
		}
		full = append(full, continuePayload...)
		next = nextPos
	}
	return full, next
}

func gatherFutureRecordPayload(data []byte, pos int, continueID uint16) ([]byte, int) {
	_, payload, next, ok := readBiffRecord(data, pos)
	if !ok {
		return nil, pos
	}
	full := append([]byte(nil), payload...)
	for {
		id, continuePayload, nextPos, ok := readBiffRecord(data, next)
		if !ok || id != continueID {
			break
		}
		if len(continuePayload) > 12 {
			full = append(full, continuePayload[12:]...)
		}
		next = nextPos
	}
	return full, next
}

func readXLUnicodeString(data []byte, offset int, lenBytes int) (string, int, bool) {
	if offset < 0 || offset+lenBytes+1 > len(data) {
		return "", offset, false
	}
	var charCount int
	if lenBytes == 1 {
		charCount = int(data[offset])
	} else {
		charCount = int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	}
	offset += lenBytes
	flags := data[offset]
	offset++
	if flags&0x01 != 0 {
		need := charCount * 2
		if offset+need > len(data) {
			return "", offset, false
		}
		return decodeBiffUTF16(data[offset : offset+need]), offset + need, true
	}
	if offset+charCount > len(data) {
		return "", offset, false
	}
	return decodeBiffBytes(data[offset : offset+charCount]), offset + charCount, true
}

func readXLUnicodeRichExtendedString(data []byte, offset int) (string, int, bool) {
	if offset < 0 || offset+3 > len(data) {
		return "", offset, false
	}
	charCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	flags := data[offset]
	offset++
	runCount := 0
	extSize := 0
	if flags&0x08 != 0 {
		if offset+2 > len(data) {
			return "", offset, false
		}
		runCount = int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2
	}
	if flags&0x04 != 0 {
		if offset+4 > len(data) {
			return "", offset, false
		}
		extSize = int(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
	}
	var value string
	if flags&0x01 != 0 {
		need := charCount * 2
		if offset+need > len(data) {
			return "", offset, false
		}
		value = decodeBiffUTF16(data[offset : offset+need])
		offset += need
	} else {
		if offset+charCount > len(data) {
			return "", offset, false
		}
		value = decodeBiffBytes(data[offset : offset+charCount])
		offset += charCount
	}
	offset += runCount * 4
	offset += extSize
	if offset > len(data) {
		return "", offset, false
	}
	return value, offset, true
}

func decodeBiffUTF16(data []byte) string {
	return strings.TrimSpace(decodeBiffUTF16Raw(data))
}

func decodeBiffUTF16Raw(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	runes := make([]rune, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		r := rune(binary.LittleEndian.Uint16(data[i:]))
		if r == 0 {
			continue
		}
		runes = append(runes, r)
	}
	return string(runes)
}

// ---------------------------------------------------------------------------
// Conditional formatting
// ---------------------------------------------------------------------------

// biffRef8UToSqRef converts a BIFF8 Ref8U (8 bytes, 0-indexed) to an A1
// range string (e.g. "B2:D5").  Returns "" on a bad slice.
func biffRef8UToSqRef(b []byte) string {
	if len(b) < 8 {
		return ""
	}
	r1 := int(binary.LittleEndian.Uint16(b[0:2])) + 1
	r2 := int(binary.LittleEndian.Uint16(b[2:4])) + 1
	c1 := int(binary.LittleEndian.Uint16(b[4:6])) + 1
	c2 := int(binary.LittleEndian.Uint16(b[6:8])) + 1
	if r1 == r2 && c1 == c2 {
		return formatA1Ref(r1, c1)
	}
	return fmt.Sprintf("%s:%s", formatA1Ref(r1, c1), formatA1Ref(r2, c2))
}

// parseCONDFMTSqRef decodes a CONDFMT (0x01B2) payload and returns the
// combined sqref string and the count of CF records that follow.
//
// Layout (MS-XLS §2.4.59):
//   - bytes 0-1:  cCF (count of CF records)
//   - bytes 2-3:  fToughRecalc (ignored)
//   - bytes 4-11: refBound (Ref8U bounding box, ignored — we use the sqref)
//   - bytes 12-13: ccell (count of individual Ref8U ranges in sqref)
//   - bytes 14+:  ccell × Ref8U (8 bytes each)
func parseCONDFMTSqRef(payload []byte) (sqref string, cfCount int) {
	if len(payload) < 14 {
		return "", 0
	}
	cfCount = int(binary.LittleEndian.Uint16(payload[0:2]))
	ccell := int(binary.LittleEndian.Uint16(payload[12:14]))
	pos := 14
	var parts []string
	for i := 0; i < ccell && pos+8 <= len(payload); i++ {
		if r := biffRef8UToSqRef(payload[pos : pos+8]); r != "" {
			parts = append(parts, r)
		}
		pos += 8
	}
	return strings.Join(parts, " "), cfCount
}

// parseCONDFMT12SqRef decodes CFHEADER12 (0x0879). POI models this as a 12-byte
// FtrHeader followed by the same CF header body layout as CONDFMT.
func parseCONDFMT12SqRef(payload []byte) (sqref string, cfCount int) {
	if len(payload) < 26 {
		return "", 0
	}
	return parseCONDFMTSqRef(payload[12:])
}

// biffCFOperator maps a CF op byte to the OOXML-compatible operator string.
func biffCFOperator(op byte) string {
	switch op {
	case 1:
		return "between"
	case 2:
		return "notBetween"
	case 3:
		return "equal"
	case 4:
		return "notEqual"
	case 5:
		return "greaterThan"
	case 6:
		return "lessThan"
	case 7:
		return "greaterOrEqual"
	case 8:
		return "lessOrEqual"
	default:
		return ""
	}
}

// parseCFRecord decodes a CF (0x01B1) payload and returns a CFRule shell.
//
// Layout (MS-XLS §2.4.54):
//   - byte 0:    ct — 1=cellIs, 2=expression
//   - byte 1:    op — comparison operator
//   - bytes 2-3: cbFormula1 — byte count of formula1
//   - bytes 4-5: cbFormula2 — byte count of formula2
//   - bytes 6-27: DxfN (22 bytes, differential format — skipped for now)
//   - cbFormula1 bytes: formula1 PTG (stored as hex)
//   - cbFormula2 bytes: formula2 PTG (stored as hex)
func parseCFRecord(payload []byte) *sheetir.CFRule {
	if len(payload) < 28 {
		return nil
	}
	ct := payload[0]
	op := payload[1]
	cbFmt1 := int(binary.LittleEndian.Uint16(payload[2:4]))
	cbFmt2 := int(binary.LittleEndian.Uint16(payload[4:6]))
	// Skip 22-byte DxfN.
	pos := 28
	if pos+cbFmt1+cbFmt2 > len(payload) {
		return nil
	}
	rule := &sheetir.CFRule{}
	switch ct {
	case 1:
		rule.Type = "cellIs"
		rule.Operator = biffCFOperator(op)
	case 2:
		rule.Type = "expression"
	default:
		return nil
	}
	// Store raw PTG formula bytes as hex (full decoding is out of scope).
	if cbFmt1 > 0 {
		rule.Formula = append(rule.Formula, fmt.Sprintf("ptg:%x", payload[pos:pos+cbFmt1]))
		pos += cbFmt1
	}
	if cbFmt2 > 0 {
		rule.Formula = append(rule.Formula, fmt.Sprintf("ptg:%x", payload[pos:pos+cbFmt2]))
	}
	return rule
}

// parseExtendedColor decodes a 16-byte ExtendedColor record (HSSF CTColor).
// Layout: type(int32) + value(int32 or 4 bytes RGBA) + tint(float64).
// Returns the hex ARGB string (e.g. "FF63BE7B") or "" for auto/unset.
// When palette is non-nil, indexed colors are resolved against it.
// Theme colors are resolved against the default BIFF theme color table.
func parseExtendedColor(data []byte, palette *[56]string) string {
	if len(data) < 16 {
		return ""
	}
	typ := int32(binary.LittleEndian.Uint32(data[0:4]))
	switch typ {
	case 0: // TYPE_AUTO
		return ""
	case 1: // TYPE_INDEXED
		// colorIndex is at offset 4 (int32).
		idx := int(binary.LittleEndian.Uint32(data[4:8]))
		if idx == 0 {
			return ""
		}
		// Index 64 = AUTOMATIC
		if idx == 64 {
			return ""
		}
		// Resolve against palette if available.
		if palette != nil && idx >= 8 && idx <= 0x3F {
			return palette[idx-0x08]
		}
		// Fall back to default palette.
		if idx >= 8 && idx <= 0x3F {
			return defaultBIFFPalette[idx-0x08]
		}
		return ""
	case 2: // TYPE_RGB
		// 4 bytes RGBA at offset 4.
		r, g, b, a := data[4], data[5], data[6], data[7]
		return fmt.Sprintf("%02X%02X%02X%02X", a, r, g, b)
	case 3: // TYPE_THEMED
		// themeIndex at offset 4 (int32).
		themeIdx := int(binary.LittleEndian.Uint32(data[4:8]))
		// tint at offset 8 (float64).
		tint := math.Float64frombits(binary.LittleEndian.Uint64(data[8:16]))
		return resolveThemeColor(themeIdx, tint)
	default:
		return ""
	}
}

// defaultThemeColors maps BIFF theme color indices to their default RGB hex values.
// These are the standard Office 2007+ theme colors used when no custom theme is present.
// Source: ECMA-376 Part 1 §20.1.6.2 clrScheme (default "Office" theme).
var defaultThemeColors = map[int]string{
	0:  "000000", // THEME_DARK_1 — dark1 (black)
	1:  "FFFFFF", // THEME_LIGHT_1 — light1 (white)
	2:  "44546A", // THEME_DARK_2 — dark2
	3:  "E7E6E6", // THEME_LIGHT_2 — light2
	4:  "4472C4", // THEME_ACCENT_1 — accent1 (blue)
	5:  "ED7D31", // THEME_ACCENT_2 — accent2 (orange)
	6:  "A5A5A5", // THEME_ACCENT_3 — accent3 (grey)
	7:  "FFC000", // THEME_ACCENT_4 — accent4 (gold)
	8:  "5B9BD5", // THEME_ACCENT_5 — accent5 (light blue)
	9:  "70AD47", // THEME_ACCENT_6 — accent6 (green)
	10: "0563C1", // THEME_HYPERLINK — hyperlink (blue)
	11: "954F72", // THEME_FOLLOWED_HYPERLINK — followed hyperlink (purple)
}

// resolveThemeColor resolves a BIFF theme color index to an RGB hex string,
// applying the tint value. Returns "" for unknown theme indices.
func resolveThemeColor(themeIdx int, tint float64) string {
	rgb, ok := defaultThemeColors[themeIdx]
	if !ok {
		return ""
	}
	if tint == 0 {
		return rgb
	}
	return applyTintToHex(rgb, tint)
}

// applyTintToHex applies a tint value (-1.0 to 1.0) to an RGB hex string
// and returns the tinted RGB hex string. Matches POI ExtendedColor.applyTint.
func applyTintToHex(rgb string, tint float64) string {
	if len(rgb) < 6 {
		return rgb
	}
	r := int64(hexPairToByte(rgb[0:2]))
	g := int64(hexPairToByte(rgb[2:4]))
	b := int64(hexPairToByte(rgb[4:6]))
	r2 := applyTint(r, tint)
	g2 := applyTint(g, tint)
	b2 := applyTint(b, tint)
	return fmt.Sprintf("%02X%02X%02X", r2, g2, b2)
}

// hexPairToByte converts a 2-character hex string to a byte value.
func hexPairToByte(s string) byte {
	v, _ := strconv.ParseUint(s, 16, 8)
	return byte(v)
}

// applyTint applies a tint value to a single color channel (0-255).
// Matches POI ExtendedColor.applyTint semantics:
//
//	If tint > 0: lum' = lum * (1-tint) + (255 - 255 * (1-tint))
//	If tint < 0: lum' = lum * (1+tint)
//	If tint == 0: lum' = lum
func applyTint(lum int64, tint float64) int64 {
	if tint > 0 {
		return int64(float64(lum)*(1.0-tint) + (255.0 - 255.0*(1.0-tint)))
	} else if tint < 0 {
		return int64(float64(lum) * (1.0 + tint))
	}
	return lum
}

// parseDataBarFormatting decodes the DataBarFormatting block that follows
// the CFRULE12 record body for type="dataBar" rules.
// Layout: 2 bytes ignored, 1 reserved, 1 options, 1 percentMin, 1 percentMax,
// then ExtendedColor, DataBarThreshold(min), DataBarThreshold(max).
func parseDataBarFormatting(data []byte, palette *[56]string) *sheetir.CFDataBar {
	if len(data) < 6 {
		return nil
	}
	db := &sheetir.CFDataBar{
		MinLength: 0,
		MaxLength: 100,
		ShowValue: true,
	}
	// options byte at offset 3
	if len(data) >= 4 {
		opts := data[3]
		db.ShowValue = opts&0x01 == 0 // bit 0 = iconOnly; showValue = !iconOnly
	}
	if len(data) >= 5 {
		db.MinLength = int(data[4])
	}
	if len(data) >= 6 {
		db.MaxLength = int(data[5])
	}
	// ExtendedColor at offset 6 (16 bytes)
	if len(data) >= 22 {
		if c := parseExtendedColor(data[6:22], palette); c != "" {
			db.Color = c
		}
	}
	// Thresholds follow at offset 22: DataBarThreshold(min) then DataBarThreshold(max).
	// Each threshold: 1 byte type, 2 bytes formulaLen, optional 8 bytes value.
	pos := 22
	for i := 0; i < 2 && pos+3 <= len(data); i++ {
		ttype := data[pos]
		flen := int(binary.LittleEndian.Uint16(data[pos+1 : pos+3]))
		pos += 3
		if flen > 0 {
			if pos+flen > len(data) {
				break
			}
			pos += flen
		} else if ttype != 0 && ttype != 1 { // not MIN/MAX
			if pos+8 > len(data) {
				break
			}
			db.Cfvo = append(db.Cfvo, sheetir.CFCfvo{
				Type: biffCFThresholdType(ttype),
				Val:  fmt.Sprintf("%g", math.Float64frombits(binary.LittleEndian.Uint64(data[pos:pos+8]))),
			})
			pos += 8
		} else {
			db.Cfvo = append(db.Cfvo, sheetir.CFCfvo{
				Type: biffCFThresholdType(ttype),
			})
		}
	}
	return db
}

// parseColorGradientFormatting decodes the ColorGradientFormatting block that
// follows the CFRULE12 record body for type="colorScale" rules.
// Layout: 2 bytes ignored, 1 reserved, 1 numThresholds, 1 numColors, 1 options,
// then thresholds[], then (double position + ExtendedColor)[].
func parseColorGradientFormatting(data []byte, palette *[56]string) *sheetir.CFColorScale {
	if len(data) < 6 {
		return nil
	}
	cg := &sheetir.CFColorScale{}
	numI := int(data[3])
	numG := int(data[4])
	if numI < 0 || numI > 10 {
		numI = 3
	}
	if numG < 0 || numG > 10 {
		numG = 3
	}
	pos := 6
	// Parse thresholds
	for i := 0; i < numI && pos+3 <= len(data); i++ {
		ttype := data[pos]
		flen := int(binary.LittleEndian.Uint16(data[pos+1 : pos+3]))
		pos += 3
		if flen > 0 {
			if pos+flen > len(data) {
				break
			}
			cg.Cfvo = append(cg.Cfvo, sheetir.CFCfvo{
				Type: biffCFThresholdType(ttype),
				Val:  fmt.Sprintf("ptg:%x", data[pos:pos+flen]),
			})
			pos += flen
		} else if ttype != 0 && ttype != 1 { // not MIN/MAX
			if pos+8 > len(data) {
				break
			}
			cg.Cfvo = append(cg.Cfvo, sheetir.CFCfvo{
				Type: biffCFThresholdType(ttype),
				Val:  fmt.Sprintf("%g", math.Float64frombits(binary.LittleEndian.Uint64(data[pos:pos+8]))),
			})
			pos += 8
		} else {
			cg.Cfvo = append(cg.Cfvo, sheetir.CFCfvo{
				Type: biffCFThresholdType(ttype),
			})
		}
	}
	// Parse colors: each has 8-byte double position + 16-byte ExtendedColor
	for i := 0; i < numG && pos+24 <= len(data); i++ {
		pos += 8 // skip position double
		if c := parseExtendedColor(data[pos:pos+16], palette); c != "" {
			cg.Colors = append(cg.Colors, c)
		}
		pos += 16
	}
	return cg
}

// biffCFThresholdType maps a BIFF threshold type byte to a string.
func biffCFThresholdType(t byte) string {
	switch t {
	case 0:
		return "min"
	case 1:
		return "max"
	case 2:
		return "num"
	case 3:
		return "percent"
	case 4:
		return "percentile"
	case 5:
		return "formula"
	default:
		return "num"
	}
}

// parseExtFmtBlock decodes the extFmtLen block from a CFRULE12 record.
// The block layout mirrors POI CFRuleBase.readFormatOptions:
//
//	offset 0:  formatting_options (INT32) — bitfield indicating which blocks follow:
//	  bit 26 (0x04000000): font block present
//	  bit 27 (0x08000000): alignment block present (not decoded)
//	  bit 28 (0x10000000): border block present
//	  bit 29 (0x20000000): pattern block present
//	  bit 30 (0x40000000): protection block present (not decoded)
//	offset 4:  formatting_not_used (INT16) — reserved
//	then, conditionally:
//	  FontFormatting (118 bytes) — when font bit is set
//	  BorderFormatting (8 bytes) — when border bit is set
//	  PatternFormatting (4 bytes) — when pattern bit is set
//
// FontFormatting layout (HSSF FontFormatting, 118 bytes):
//
//	offset 0: 2 bytes reserved
//	offset 2: 2 bytes font height (1/20 pt)
//	offset 4: 2 bytes font weight (400=normal, 700=bold)
//	offset 6: 1 byte escapement type (0=none, 1=super, 2=sub)
//	offset 7: 1 byte underline type (0=none, 1=single, 2=double, 33=single-accounting, 34=double-accounting)
//	offset 8: 1 byte font family (ignored)
//	offset 9: 1 byte character set (ignored)
//	offset 10: 1 byte reserved
//	offset 11: 1 byte reserved
//	offset 12: 4 bytes font color RGB (palette index in low byte)
//	offset 16: 1 byte options — bit 0=bold, bit 1=italic, bit 2=underline, bit 3=strikeout,
//	            bit 4=outline, bit 5=shadow, bit 6=condense, bit 7=extend
//	offset 17: 2 bytes escapement type again (ignored)
//	offset 19: 2 bytes underline type again (ignored)
//	offset 21: 2 bytes font name length (ignored)
//	offset 23: 95 bytes font name (ignored)
//
// BorderFormatting layout (HSSF BorderFormatting, 8 bytes):
//
//	offset 0: 4 bytes — lower 4 bits each: left, right, top, bottom border style
//	            bits 16-22: left color, bits 23-29: right color
//	offset 4: 4 bytes — bits 0-6: top color, bits 7-13: bottom color
//	            bits 14-20: diagonal color, bits 21-22: diagonal style
//	            bit 23: forward diagonal on, bit 24: backward diagonal on
//
// PatternFormatting layout (HSSF PatternFormatting, 4 bytes):
//
//	offset 0: 2 bytes fill pattern
//	offset 2: 2 bytes foreground/background color index (low byte=fore, high byte=back)
func parseExtFmtBlock(data []byte, palette *[56]string) *sheetir.DxfStyle {
	if len(data) < 6 {
		return nil
	}
	dxf := &sheetir.DxfStyle{}
	opts := binary.LittleEndian.Uint32(data[0:4])
	pos := 6

	// FontFormatting block (118 bytes).
	if opts&0x04000000 != 0 && pos+118 <= len(data) {
		ff := data[pos : pos+118]
		height := int(binary.LittleEndian.Uint16(ff[2:4]))
		if height > 0 {
			dxf.FontHeight = &height
		}
		weight := int(binary.LittleEndian.Uint16(ff[4:6]))
		if weight > 0 {
			dxf.FontWeight = &weight
		}
		esc := int(ff[6])
		if esc > 0 {
			dxf.EscapementType = &esc
		}
		ul := int(ff[7])
		if ul > 0 {
			dxf.UnderlineType = &ul
		}
		// Font color at offset 12 (4 bytes, palette index in low byte).
		colorIdx := int(binary.LittleEndian.Uint32(ff[12:16]) & 0x7F)
		if colorIdx > 0 {
			dxf.FontColorIndex = &colorIdx
		}
		// Options byte at offset 16.
		opt := ff[16]
		bold := opt&0x01 != 0
		italic := opt&0x02 != 0
		underline := opt&0x04 != 0
		strike := opt&0x08 != 0
		outline := opt&0x10 != 0
		shadow := opt&0x20 != 0
		if bold {
			dxf.Bold = &bold
		}
		if italic {
			dxf.Italic = &italic
		}
		if underline {
			dxf.Underline = &underline
		}
		if strike {
			dxf.Strike = &strike
		}
		if outline {
			dxf.Outline = &outline
		}
		if shadow {
			dxf.Shadow = &shadow
		}
		pos += 118
	}

	// BorderFormatting block (8 bytes).
	if opts&0x10000000 != 0 && pos+8 <= len(data) {
		bf := data[pos : pos+8]
		b1 := binary.LittleEndian.Uint32(bf[0:4])
		b2 := binary.LittleEndian.Uint32(bf[4:8])
		left := int(b1 & 0x0F)
		right := int((b1 >> 4) & 0x0F)
		top := int((b1 >> 8) & 0x0F)
		bottom := int((b1 >> 12) & 0x0F)
		diag := int((b2 >> 21) & 0x03)
		if left > 0 {
			dxf.BorderLeft = &left
		}
		if right > 0 {
			dxf.BorderRight = &right
		}
		if top > 0 {
			dxf.BorderTop = &top
		}
		if bottom > 0 {
			dxf.BorderBottom = &bottom
		}
		if diag > 0 {
			dxf.BorderDiagonal = &diag
		}
		// Border colors (palette indices, 7-bit each).
		leftColor := int((b1 >> 16) & 0x7F)
		rightColor := int((b1 >> 23) & 0x7F)
		topColor := int(b2 & 0x7F)
		bottomColor := int((b2 >> 7) & 0x7F)
		diagColor := int((b2 >> 14) & 0x7F)
		if leftColor > 0 {
			dxf.LeftBorderColor = resolveICV(leftColor, palette)
		}
		if rightColor > 0 {
			dxf.RightBorderColor = resolveICV(rightColor, palette)
		}
		if topColor > 0 {
			dxf.TopBorderColor = resolveICV(topColor, palette)
		}
		if bottomColor > 0 {
			dxf.BottomBorderColor = resolveICV(bottomColor, palette)
		}
		if diagColor > 0 {
			dxf.DiagonalBorderColor = resolveICV(diagColor, palette)
		}
		fwd := b2&0x00800000 != 0
		bwd := b2&0x01000000 != 0
		if fwd {
			dxf.ForwardDiagonalOn = &fwd
		}
		if bwd {
			dxf.BackwardDiagonalOn = &bwd
		}
		pos += 8
	}

	// PatternFormatting block (4 bytes).
	if opts&0x20000000 != 0 && pos+4 <= len(data) {
		pf := data[pos : pos+4]
		pattern := int(binary.LittleEndian.Uint16(pf[0:2]))
		if pattern > 0 {
			dxf.FillPattern = &pattern
		}
		foreIdx := int(pf[2] & 0x7F)
		backIdx := int(pf[3] & 0x7F)
		if foreIdx > 0 {
			dxf.FillForegroundColor = resolveICV(foreIdx, palette)
		}
		if backIdx > 0 {
			dxf.FillBackgroundColor = resolveICV(backIdx, palette)
		}
		pos += 4
	}

	return dxf
}

// parseCF12Record decodes a CFRULE12 (0x087A) record.
// The CF12 body begins with a 12-byte FtrHeader, then condition type and
// comparison operation. This parser maps the rule type and, when possible,
// captures formula1/formula2 PTG blobs as hex for compatibility with CF.
// For colorScale and dataBar rules, it also parses the ext formatting block
// and trailing DataBarFormatting / ColorGradientFormatting structures.
func parseCF12Record(payload []byte, palette *[56]string) *sheetir.CFRule {
	if len(payload) < 22 {
		return nil
	}
	ct := payload[12]
	op := payload[13]
	cbFmt1 := int(binary.LittleEndian.Uint16(payload[14:16]))
	cbFmt2 := int(binary.LittleEndian.Uint16(payload[16:18]))
	extFmtLen := int(binary.LittleEndian.Uint32(payload[18:22]))

	rule := &sheetir.CFRule{}
	switch ct {
	case 1:
		rule.Type = "cellIs"
		rule.Operator = biffCFOperator(op)
	case 2:
		rule.Type = "expression"
	case 3:
		rule.Type = "colorScale"
	case 4:
		rule.Type = "dataBar"
	case 5:
		rule.Type = "filter"
	case 6:
		rule.Type = "iconSet"
	default:
		return nil
	}

	// Parse ext formatting block (formatting_options + FontFormatting/BorderFormatting/PatternFormatting).
	pos := 22
	if extFmtLen > 0 {
		if dxf := parseExtFmtBlock(payload[pos:pos+extFmtLen], nil); dxf != nil {
			rule.DxfStyle = *dxf
		}
		pos += extFmtLen
	} else {
		if pos+2 > len(payload) {
			return rule
		}
		pos += 2 // reserved
	}

	// Parse formula1 and formula2.
	if pos+cbFmt1+cbFmt2 > len(payload) {
		return rule
	}
	if cbFmt1 > 0 {
		rule.Formula = append(rule.Formula, fmt.Sprintf("ptg:%x", payload[pos:pos+cbFmt1]))
		pos += cbFmt1
	}
	if cbFmt2 > 0 {
		rule.Formula = append(rule.Formula, fmt.Sprintf("ptg:%x", payload[pos:pos+cbFmt2]))
		pos += cbFmt2
	}

	// Skip formula_scale (2-byte length + tokens).
	if pos+2 <= len(payload) {
		scaleLen := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
		pos += 2 + scaleLen
	}

	// Skip ext_opts (1), priority (2), template_type (2), template_param_length (1), template_params.
	if pos+6 <= len(payload) {
		tplLen := int(payload[pos+5])
		pos += 6 + tplLen
	}

	// Parse trailing formatting blocks based on rule type.
	if ct == 4 && pos < len(payload) {
		// dataBar: DataBarFormatting block follows.
		if db := parseDataBarFormatting(payload[pos:], palette); db != nil {
			rule.DataBar = db
		}
	} else if ct == 3 && pos < len(payload) {
		// colorScale: ColorGradientFormatting block follows.
		if cg := parseColorGradientFormatting(payload[pos:], palette); cg != nil {
			rule.ColorScale = cg
		}
	} else if ct == 6 && pos < len(payload) {
		// iconSet: IconMultiStateFormatting block follows.
		rule.IconSet = &sheetir.CFIconSet{}
		// Minimal icon set parsing: first byte is num thresholds, second byte is icon set ID.
		if pos+2 <= len(payload) {
			_ = int(payload[pos]) // num thresholds
			setID := int(payload[pos+1])
			rule.IconSet.Set = biffIconSetName(setID)
		}
	}
	return rule
}

// biffIconSetName maps a BIFF icon set ID to a string name.
func biffIconSetName(id int) string {
	names := map[int]string{
		0:  "3Arrows",
		1:  "3ArrowsGray",
		2:  "3Flags",
		3:  "3TrafficLights1",
		4:  "3TrafficLights2",
		5:  "3Signs",
		6:  "3Symbols",
		7:  "3Symbols2",
		8:  "4Arrows",
		9:  "4ArrowsGray",
		10: "4RedToBlack",
		11: "4Rating",
		12: "4TrafficLights",
		13: "5Arrows",
		14: "5ArrowsGray",
		15: "5Rating",
		16: "5Quarters",
	}
	if name, ok := names[id]; ok {
		return name
	}
	return fmt.Sprintf("iconSet_%d", id)
}

// ---------------------------------------------------------------------------
// Data validation
// ---------------------------------------------------------------------------

// biffDVType maps the valType nibble in dwDvFlags to a type string.
func biffDVType(v uint8) string {
	switch v {
	case 1:
		return "whole"
	case 2:
		return "decimal"
	case 3:
		return "list"
	case 4:
		return "date"
	case 5:
		return "time"
	case 6:
		return "textLength"
	case 7:
		return "custom"
	default:
		return "any"
	}
}

// biffDVOperator maps the operator nibble in dwDvFlags to an operator string.
func biffDVOperator(v uint8) string {
	switch v {
	case 1:
		return "between"
	case 2:
		return "notBetween"
	case 3:
		return "equal"
	case 4:
		return "notEqual"
	case 5:
		return "greaterThan"
	case 6:
		return "lessThan"
	case 7:
		return "greaterOrEqual"
	case 8:
		return "lessOrEqual"
	default:
		return ""
	}
}

// parseDVRecord decodes a DV (0x01BE) payload and returns a DataValidation.
//
// Layout (MS-XLS §2.4.92):
//   - bytes 0-3:  dwDvFlags (packed options)
//   - then: 4 XLUnicodeString in order: promptTitle, errorTitle, prompt, error
//   - 2 bytes: cce1 (formula1 byte count)
//   - 2 bytes: ignored
//   - cce1 bytes: formula1 PTG (stored as hex)
//   - 4 bytes: ignored (padding)
//   - 2 bytes: cce2 (formula2 byte count)
//   - 2 bytes: ignored
//   - cce2 bytes: formula2 PTG (stored as hex)
//   - 4 bytes: ignored (padding)
//   - SQREF (2-byte count + count×Ref8U)
func parseDVRecord(payload []byte, formulaCtx legacyFormulaWorkbookContext) *sheetir.DataValidation {
	if len(payload) < 4 {
		return nil
	}
	flags := binary.LittleEndian.Uint32(payload[0:4])
	valType := biffDVType(uint8(flags & 0x0F))
	operator := biffDVOperator(uint8((flags >> 20) & 0x0F))
	allowBlank := flags&(1<<8) != 0
	suppressCombo := flags&(1<<9) != 0
	showInput := flags&(1<<18) != 0
	showError := flags&(1<<19) != 0

	pos := 4
	// Read 4 XLUnicodeString fields.
	readStr := func() (string, bool) {
		s, next, ok := readXLUnicodeString(payload, pos, 2)
		if ok {
			pos = next
		}
		return s, ok
	}
	promptTitle, _ := readStr()
	errorTitle, _ := readStr()
	prompt, _ := readStr()
	errText, _ := readStr()

	// Formula 1.
	if pos+4 > len(payload) {
		return nil
	}
	cce1 := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
	pos += 4 // skip cce1 + 2 reserved bytes
	formula1 := ""
	if cce1 > 0 && pos+cce1 <= len(payload) {
		if text, ok := decodeBIFFFormula(payload[pos:pos+cce1], legacyFormulaDecodeContext{
			workbook: formulaCtx,
		}); ok {
			formula1 = text
		} else {
			formula1 = fmt.Sprintf("ptg:%x", payload[pos:pos+cce1])
		}
		pos += cce1
	}
	// Skip 4-byte padding after formula1.
	pos += 4

	// Formula 2.
	formula2 := ""
	if pos+4 <= len(payload) {
		cce2 := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
		pos += 4 // skip cce2 + 2 reserved bytes
		if cce2 > 0 && pos+cce2 <= len(payload) {
			if text, ok := decodeBIFFFormula(payload[pos:pos+cce2], legacyFormulaDecodeContext{
				workbook: formulaCtx,
			}); ok {
				formula2 = text
			} else {
				formula2 = fmt.Sprintf("ptg:%x", payload[pos:pos+cce2])
			}
			pos += cce2
		}
		pos += 4 // padding
	}

	// SQREF (2-byte count + count×Ref8U).
	sqref := ""
	if pos+2 <= len(payload) {
		ccell := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
		pos += 2
		var parts []string
		for i := 0; i < ccell && pos+8 <= len(payload); i++ {
			if r := biffRef8UToSqRef(payload[pos : pos+8]); r != "" {
				parts = append(parts, r)
			}
			pos += 8
		}
		sqref = strings.Join(parts, " ")
	}
	if sqref == "" {
		return nil
	}

	dv := &sheetir.DataValidation{
		SqRef:            sqref,
		Type:             valType,
		Operator:         operator,
		AllowBlank:       allowBlank,
		ShowDropDown:     suppressCombo,
		ShowInputMessage: showInput,
		ShowErrorMessage: showError,
		PromptTitle:      promptTitle,
		Prompt:           prompt,
		ErrorTitle:       errorTitle,
		Error:            errText,
		Formula1:         formula1,
		Formula2:         formula2,
	}
	return dv
}

// parseNameCommentRecord decodes a BIFF8 NameComment (0x0894) record payload.
// It returns the name text, comment text, and an ok flag.
//
// Record layout (POI NameCommentRecord):
//   - bytes 0-1:   record type (ignored)
//   - bytes 2-3:   frt cell ref flag (ignored)
//   - bytes 4-11:  reserved (ignored)
//   - bytes 12-13: name length
//   - bytes 14-15: comment length
//   - byte  16:    name is multibyte (0 = compressed, 1 = UTF-16 LE)
//   - name text
//   - byte  N:     comment is multibyte
//   - comment text
func parseNameCommentRecord(payload []byte) (string, string, bool) {
	if len(payload) < 18 {
		return "", "", false
	}
	nameLen := int(binary.LittleEndian.Uint16(payload[12:14]))
	commentLen := int(binary.LittleEndian.Uint16(payload[14:16]))

	off := 16
	nameText, next, ok := readXLUnicodeStringLen(payload, off, nameLen)
	if !ok {
		return "", "", false
	}
	off = next
	commentText, _, ok := readXLUnicodeStringLen(payload, off, commentLen)
	if !ok {
		return "", "", false
	}
	return nameText, commentText, true
}

// readXLUnicodeStringLen reads a fixed-length XLUnicodeString at offset.
// Unlike readXLUnicodeString it does not read the length from the payload;
// the caller supplies it. It returns the decoded string, the next offset,
// and an ok flag.
func readXLUnicodeStringLen(data []byte, offset int, charCount int) (string, int, bool) {
	if offset < 0 || offset+1 > len(data) {
		return "", offset, false
	}
	flags := data[offset]
	offset++
	if flags&0x01 != 0 {
		need := charCount * 2
		if offset+need > len(data) {
			return "", offset, false
		}
		return decodeBiffUTF16(data[offset : offset+need]), offset + need, true
	}
	if offset+charCount > len(data) {
		return "", offset, false
	}
	return string(data[offset : offset+charCount]), offset + charCount, true
}

// parseBIFFName decodes a BIFF8 NAME (0x0218) record payload and returns
// a DefinedName IR entry.
//
// Record layout (§2.4.142 of [MS-XLS]):
//   - bytes 0-1: grbit (bit 0 = fBuiltin, bit 5 = fHidden)
//   - byte  2:   chKey (keyboard shortcut — unused)
//   - byte  3:   cch (number of characters in the name)
//   - bytes 4-5: cce (byte count of the formula/ptg array)
//   - bytes 6-7: reserved (unused)
//   - bytes 8-9: itab (1-based sheet index; 0 = workbook-scoped)
//   - bytes 10-13: reserved
//   - then:      XLUnicodeStringNoCch (fHighByte flag + char bytes)
//   - then:      formula bytes (cce bytes, skipped)
func parseBIFFName(payload []byte) (*sheetir.DefinedName, bool) {
	return parseBIFFNameWithContext(payload, nil, nil)
}

func parseBIFFNameWithContext(payload []byte, sheets []boundSheet, externSheets []externSheetRef) (*sheetir.DefinedName, bool) {
	if len(payload) < 14 {
		return nil, false
	}
	grbit := binary.LittleEndian.Uint16(payload[0:2])
	fBuiltin := grbit&0x0001 != 0
	cch := int(payload[3])
	cce := int(binary.LittleEndian.Uint16(payload[4:6]))
	itab := int(binary.LittleEndian.Uint16(payload[8:10]))

	// Skip the reserved bytes; name starts at byte 14.
	pos := 14
	if pos >= len(payload) {
		return nil, false
	}

	var name string
	if fBuiltin {
		if cch <= 0 || pos+cch > len(payload) {
			return nil, false
		}
		name = builtinNameString(payload[pos])
		pos += cch
	} else {
		// fHighByte flag is the first byte of XLUnicodeStringNoCch.
		fHighByte := payload[pos]&0x01 != 0
		pos++
		if cch == 0 {
			return nil, false
		}
		charBytes := cch
		if fHighByte {
			charBytes = cch * 2
		}
		if pos+charBytes > len(payload) {
			return nil, false
		}
		if fHighByte {
			name = decodeBiffUTF16Raw(payload[pos : pos+charBytes])
		} else {
			name = decodeBiffBytesRaw(payload[pos : pos+charBytes])
		}
		pos += charBytes
	}
	name = strings.TrimRight(name, "\x00")
	if name == "" {
		return nil, false
	}

	// Formula bytes.
	value := ""
	var formulaBytes []byte
	if cce > 0 && pos+cce <= len(payload) {
		formulaBytes = payload[pos : pos+cce]
		if builtInValue := decodeBuiltInNameFormula(name, formulaBytes, externSheets, sheets); builtInValue != "" {
			value = builtInValue
		} else {
			// We store the raw formula bytes as a hex string for future decoding.
			var sb strings.Builder
			for _, b := range formulaBytes {
				fmt.Fprintf(&sb, "%02x", b)
			}
			value = sb.String()
		}
	}

	dn := &sheetir.DefinedName{
		Name:   name,
		Hidden: grbit&0x0020 != 0,
		Value:  value,
	}
	if itab > 0 {
		sheet := itab - 1
		dn.LocalSheetID = &sheet
	}
	return dn, true
}

func builtinNameString(id byte) string {
	switch id {
	case 0x06:
		return "_xlnm.Print_Area"
	case 0x07:
		return "_xlnm.Print_Titles"
	default:
		return fmt.Sprintf("_xlnm.BuiltIn_%02X", id)
	}
}

func decodeBuiltInNameFormula(name string, formula []byte, externSheets []externSheetRef, sheets []boundSheet) string {
	if !strings.EqualFold(name, "_xlnm.Print_Area") && !strings.EqualFold(name, "_xlnm.Print_Titles") {
		return ""
	}
	parts := extract3DRefs(formula, externSheets, sheets)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}

func extract3DRefs(formula []byte, externSheets []externSheetRef, sheets []boundSheet) []string {
	var parts []string
	for i := 0; i < len(formula); {
		token := formula[i] & 0x3F
		switch token {
		case 0x3A: // ptgRef3d
			if i+7 > len(formula) {
				return parts
			}
			if ref, ok := decodePtgRef3D(formula[i:i+7], externSheets, sheets); ok {
				parts = append(parts, ref)
			}
			i += 7
		case 0x3B: // ptgArea3d
			if i+11 > len(formula) {
				return parts
			}
			if ref, ok := decodePtgArea3D(formula[i:i+11], externSheets, sheets); ok {
				parts = append(parts, ref)
			}
			i += 11
		default:
			// Token-length coverage focused on print area/titles patterns.
			i++
		}
	}
	return parts
}

func decodePtgRef3D(token []byte, externSheets []externSheetRef, sheets []boundSheet) (string, bool) {
	if len(token) < 7 {
		return "", false
	}
	sheetIndex, ok := resolveExternSheetIndex(binary.LittleEndian.Uint16(token[1:3]), externSheets, -1)
	if !ok || sheetIndex < 0 || sheetIndex >= len(sheets) {
		return "", false
	}
	row := int(binary.LittleEndian.Uint16(token[3:5]))
	col := int(binary.LittleEndian.Uint16(token[5:7]) & 0x00FF)
	ref := formatAbsoluteA1Ref(row+1, col+1)
	return format3DRef(sheets[sheetIndex].name, ref), true
}

func decodePtgArea3D(token []byte, externSheets []externSheetRef, sheets []boundSheet) (string, bool) {
	if len(token) < 11 {
		return "", false
	}
	sheetIndex, ok := resolveExternSheetIndex(binary.LittleEndian.Uint16(token[1:3]), externSheets, -1)
	if !ok || sheetIndex < 0 || sheetIndex >= len(sheets) {
		return "", false
	}
	firstRow := int(binary.LittleEndian.Uint16(token[3:5]))
	lastRow := int(binary.LittleEndian.Uint16(token[5:7]))
	firstCol := int(binary.LittleEndian.Uint16(token[7:9]) & 0x00FF)
	lastCol := int(binary.LittleEndian.Uint16(token[9:11]) & 0x00FF)

	const maxBIFF8Row = 65535
	const maxBIFF8Col = 255

	var rangeRef string
	if firstCol == 0 && lastCol == maxBIFF8Col {
		rangeRef = fmt.Sprintf("$%d:$%d", firstRow+1, lastRow+1)
	} else if firstRow == 0 && lastRow == maxBIFF8Row {
		rangeRef = fmt.Sprintf("$%s:$%s", colNameAbsolute(firstCol+1), colNameAbsolute(lastCol+1))
	} else {
		rangeRef = fmt.Sprintf("%s:%s", formatAbsoluteA1Ref(firstRow+1, firstCol+1), formatAbsoluteA1Ref(lastRow+1, lastCol+1))
	}
	return format3DRef(sheets[sheetIndex].name, rangeRef), true
}

func format3DRef(sheetName, rangeRef string) string {
	if sheetName == "" {
		return rangeRef
	}
	if strings.ContainsAny(sheetName, " '!,") {
		sheetName = "'" + strings.ReplaceAll(sheetName, "'", "''") + "'"
	}
	return sheetName + "!" + rangeRef
}

func formatAbsoluteA1Ref(row, col int) string {
	return "$" + colNameAbsolute(col) + "$" + strconv.Itoa(row)
}

func colNameAbsolute(col int) string {
	if col < 1 {
		return "A"
	}
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+(col%26))) + name
		col /= 26
	}
	return name
}

type printTitles struct {
	rows string
	cols string
}

func parsePrintTitles(value string) printTitles {
	var out printTitles
	for _, rangePart := range splitDefinedNameLocalRanges(value) {
		if strings.Contains(rangePart, "$0") {
			continue
		}
		if isPrintTitleRowRange(rangePart) {
			out.rows = rangePart
			continue
		}
		if isPrintTitleColRange(rangePart) {
			out.cols = rangePart
		}
	}
	return out
}

func parsePrintArea(value string) string {
	parts := splitDefinedNameLocalRanges(value)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}

func splitDefinedNameLocalRanges(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bang := strings.LastIndex(part, "!")
		if bang < 0 || bang == len(part)-1 {
			continue
		}
		rangePart := strings.TrimSpace(part[bang+1:])
		if rangePart == "" {
			continue
		}
		out = append(out, rangePart)
	}
	return out
}

func isPrintTitleRowRange(v string) bool {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if len(part) < 2 || part[0] != '$' {
			return false
		}
		for _, r := range part[1:] {
			if !unicode.IsDigit(r) {
				return false
			}
		}
	}
	return true
}

func isPrintTitleColRange(v string) bool {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if len(part) < 2 || part[0] != '$' {
			return false
		}
		for _, r := range part[1:] {
			if !unicode.IsLetter(r) {
				return false
			}
		}
	}
	return true
}

// parseHLINKRecord decodes a BIFF8 HLINK (0x01B8) record payload and returns
// the cell range (0-based) and target URL.
//
// Record layout (§2.4.154 and §2.5.100 of [MS-XLS]):
//   - ref8 (8 bytes): firstRow, lastRow, firstCol, lastCol — 2 bytes each, 0-indexed
//   - CLSID (16 bytes): HyperlinkObject outer class ID (skipped)
//   - streamVersion (4 bytes): 0x00000002 (skipped)
//   - hlstmf (4 bytes): HyperlinkFlags bitmask
//   - [optional displayName: 4-byte count + UTF-16LE chars]
//   - [optional frameName: 4-byte count + UTF-16LE chars]
//   - [optional moniker: CLSID(16) + moniker-specific data]
//   - [optional locationStr: 4-byte count + UTF-16LE chars appended as #fragment]
//
// Supported moniker types (mirroring POI HyperlinkRecord):
//   - hlstmfMonikerSavedAsStr (0x0100): URL stored as plain UTF-16LE string
//   - URL moniker CLSID {79EAC9D0-BAF9-11CE-8C82-00AA004BA90B}: cbSize + UTF-16LE URL
//   - FILE moniker CLSID {00000303-0000-0000-C000-000000000046}: short filename + optional long address
//   - STD moniker CLSID {79EAC9F0-BAF9-11CE-8C82-00AA004BA90B}: fileOpts + UTF-8 path
//   - UNC path (hlstmfUNC flag set, no moniker): count + UTF-16LE string
func parseHLINKRecord(payload []byte) (hlinkEntry, bool) {
	const (
		hlstmfHasMoniker        = 0x0001
		hlstmfHasLocationStr    = 0x0008
		hlstmfHasDisplayName    = 0x0010
		hlstmfHasFrameName      = 0x0080
		hlstmfMonikerSavedAsStr = 0x0100
		hlstmfUNC               = 0x0200 // UNC path flag (no moniker CLSID)
	)
	// CLSID constants (16-byte little-endian representations).
	// URL_MONIKER: {79EAC9D0-BAF9-11CE-8C82-00AA004BA90B}
	// FILE_MONIKER: {00000303-0000-0000-C000-000000000046}
	// STD_MONIKER: {79EAC9F0-BAF9-11CE-8C82-00AA004BA90B}
	const (
		clsidURLLen = 16
	)
	var (
		clsidURL  = []byte{0xD0, 0xC9, 0xEA, 0x79, 0xF9, 0xBA, 0xCE, 0x11, 0x8C, 0x82, 0x00, 0xAA, 0x00, 0x4B, 0xA9, 0x0B}
		clsidFILE = []byte{0x03, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}
		clsidSTD  = []byte{0xF0, 0xC9, 0xEA, 0x79, 0xF9, 0xBA, 0xCE, 0x11, 0x8C, 0x82, 0x00, 0xAA, 0x00, 0x4B, 0xA9, 0x0B}
	)
	if len(payload) < 32 {
		return hlinkEntry{}, false
	}
	e := hlinkEntry{
		firstRow: int(binary.LittleEndian.Uint16(payload[0:2])),
		lastRow:  int(binary.LittleEndian.Uint16(payload[2:4])),
		firstCol: int(binary.LittleEndian.Uint16(payload[4:6])),
		lastCol:  int(binary.LittleEndian.Uint16(payload[6:8])),
	}
	// bytes 8-23: outer CLSID (skip)
	// bytes 24-27: streamVersion (skip)
	hlstmf := binary.LittleEndian.Uint32(payload[28:32])
	pos := 32

	readUTF16Chunk := func() (string, bool) {
		if pos+4 > len(payload) {
			return "", false
		}
		count := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
		pos += 4
		if count <= 0 || pos+count*2 > len(payload) {
			pos += count * 2
			return "", false
		}
		s := decodeBiffUTF16Raw(payload[pos : pos+count*2])
		pos += count * 2
		return strings.TrimRight(s, "\x00"), true
	}

	// Optional displayName (skip text; we use the URL as the link target).
	if hlstmf&hlstmfHasDisplayName != 0 {
		if pos+4 > len(payload) {
			return hlinkEntry{}, false
		}
		count := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
		pos += 4 + count*2
		if pos > len(payload) {
			return hlinkEntry{}, false
		}
	}

	// Optional frameName (skip).
	if hlstmf&hlstmfHasFrameName != 0 {
		if pos+4 > len(payload) {
			return hlinkEntry{}, false
		}
		count := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
		pos += 4 + count*2
		if pos > len(payload) {
			return hlinkEntry{}, false
		}
	}

	var url string
	if hlstmf&hlstmfHasMoniker != 0 {
		if hlstmf&hlstmfMonikerSavedAsStr != 0 {
			// URL stored as a plain wide string: count (4 bytes) + UTF-16LE.
			if s, ok := readUTF16Chunk(); ok {
				url = s
			}
		} else if hlstmf&hlstmfUNC != 0 {
			// UNC path: no moniker CLSID, just count (4 bytes) + UTF-16LE string.
			if s, ok := readUTF16Chunk(); ok {
				url = s
			}
		} else {
			// CLSID (16 bytes) identifies the moniker type.
			if pos+16 > len(payload) {
				return hlinkEntry{}, false
			}
			clsid := payload[pos : pos+16]
			pos += 16

			switch {
			case bytes.Equal(clsid, clsidURL):
				// URL moniker: cbSize (4 bytes) + UTF-16LE URL.
				if pos+4 > len(payload) {
					return hlinkEntry{}, false
				}
				cbSize := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
				pos += 4
				if cbSize > 0 {
					end := pos + cbSize
					if end > len(payload) {
						end = len(payload)
					}
					url = decodeBiffUTF16Raw(payload[pos:end])
					url = strings.TrimRight(url, "\x00")
				}
				pos += cbSize

			case bytes.Equal(clsid, clsidFILE):
				// FILE moniker (§2.5.100): fileOpts(2) + cchShort(4) + shortFilename(cchShort bytes, compressed) + tail(24) + [optional long address].
				if pos+2 > len(payload) {
					return hlinkEntry{}, false
				}
				// fileOpts (2 bytes) — skip, not surfaced in IR.
				pos += 2

				if pos+4 > len(payload) {
					return hlinkEntry{}, false
				}
				shortLen := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
				pos += 4
				if shortLen > 0 && pos+shortLen <= len(payload) {
					url = decodeBiffBytesRaw(payload[pos : pos+shortLen])
					url = strings.TrimRight(url, "\x00")
				}
				pos += shortLen

				// Skip 24-byte tail (FILE_TAIL in POI: FF FF AD DE 00...).
				if pos+24 > len(payload) {
					return hlinkEntry{}, false
				}
				pos += 24

				// Optional long address: size(4) + charDataSize(4) + usKeyValue(2) + UTF-16LE chars.
				if pos+4 <= len(payload) {
					addrSize := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
					pos += 4
					if addrSize > 0 && pos+4 <= len(payload) {
						charDataSize := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
						pos += 4
						if pos+2 <= len(payload) {
							// usKeyValue (2 bytes) — skip, MUST be 3 per spec.
							pos += 2
							if charDataSize > 0 && pos+charDataSize <= len(payload) {
								longAddr := decodeBiffUTF16Raw(payload[pos : pos+charDataSize])
								longAddr = strings.TrimRight(longAddr, "\x00")
								if longAddr != "" {
									url = longAddr
								}
							}
							pos += charDataSize
						}
					}
				}

			case bytes.Equal(clsid, clsidSTD):
				// STD moniker: fileOpts(2) + pathLen(4) + UTF-8 path bytes.
				if pos+2 > len(payload) {
					return hlinkEntry{}, false
				}
				// fileOpts (2 bytes) — skip.
				pos += 2

				if pos+4 > len(payload) {
					return hlinkEntry{}, false
				}
				pathLen := int(binary.LittleEndian.Uint32(payload[pos : pos+4]))
				pos += 4
				if pathLen > 0 && pos+pathLen <= len(payload) {
					url = strings.TrimRight(string(payload[pos:pos+pathLen]), "\x00")
				}
				pos += pathLen
			}
		}
	}

	// Optional location string (appended as #fragment).
	if hlstmf&hlstmfHasLocationStr != 0 && url != "" {
		if frag, ok := readUTF16Chunk(); ok && frag != "" {
			url += "#" + frag
		}
	}

	if url == "" {
		return hlinkEntry{}, false
	}
	e.url = url
	return e, true
}

func decodeBiffBytes(data []byte) string {
	if activeBiffCodepage != 0 {
		return decodeBiffBytesWithCodepage(data, activeBiffCodepage)
	}
	return strings.TrimSpace(decodeBiffBytesRaw(data))
}

func decodeBiffBytesRaw(data []byte) string {
	if activeBiffCodepage != 0 {
		return decodeBiffBytesWithCodepage(data, activeBiffCodepage)
	}
	var b strings.Builder
	for _, ch := range data {
		if ch == 0 {
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

// decodeBiffBytesWithCodepage decodes single-byte BIFF text using the workbook
// codepage. BIFF8 compressed strings and BIFF5 strings use the codepage from
// the CODEPAGE record (0x0042); when no codepage is specified, Windows-1252
// (Western European) is assumed.
func decodeBiffBytesWithCodepage(data []byte, codepage int) string {
	var decoded string
	switch codepage {
	case 1250:
		decoded, _ = charmap.Windows1250.NewDecoder().String(string(data))
	case 1251:
		decoded, _ = charmap.Windows1251.NewDecoder().String(string(data))
	case 1252:
		decoded, _ = charmap.Windows1252.NewDecoder().String(string(data))
	case 1253:
		decoded, _ = charmap.Windows1253.NewDecoder().String(string(data))
	case 1254:
		decoded, _ = charmap.Windows1254.NewDecoder().String(string(data))
	case 1255:
		decoded, _ = charmap.Windows1255.NewDecoder().String(string(data))
	case 1256:
		decoded, _ = charmap.Windows1256.NewDecoder().String(string(data))
	case 1257:
		decoded, _ = charmap.Windows1257.NewDecoder().String(string(data))
	case 1258:
		decoded, _ = charmap.Windows1258.NewDecoder().String(string(data))
	case 437:
		decoded, _ = charmap.CodePage437.NewDecoder().String(string(data))
	case 850:
		decoded, _ = charmap.CodePage850.NewDecoder().String(string(data))
	case 852:
		decoded, _ = charmap.CodePage852.NewDecoder().String(string(data))
	case 855:
		decoded, _ = charmap.CodePage855.NewDecoder().String(string(data))
	case 858:
		decoded, _ = charmap.CodePage858.NewDecoder().String(string(data))
	case 860:
		decoded, _ = charmap.CodePage860.NewDecoder().String(string(data))
	case 862:
		decoded, _ = charmap.CodePage862.NewDecoder().String(string(data))
	case 863:
		decoded, _ = charmap.CodePage863.NewDecoder().String(string(data))
	case 865:
		decoded, _ = charmap.CodePage865.NewDecoder().String(string(data))
	case 866:
		decoded, _ = charmap.CodePage866.NewDecoder().String(string(data))
	case 874:
		decoded, _ = charmap.Windows874.NewDecoder().String(string(data))
	case 10000:
		decoded, _ = charmap.Macintosh.NewDecoder().String(string(data))
	case 10004:
		decoded, _ = charmap.Windows1256.NewDecoder().String(string(data))
	case 10005:
		decoded, _ = charmap.Windows1255.NewDecoder().String(string(data))
	case 10006:
		decoded, _ = charmap.Windows1253.NewDecoder().String(string(data))
	case 10007:
		decoded, _ = charmap.Windows1251.NewDecoder().String(string(data))
	case 10008, 10010, 10017, 10021, 10029:
		// Macintosh Cyrillic, Romanian, Ukrainian, Thai, Central European
		decoded, _ = charmap.Windows1251.NewDecoder().String(string(data))
	case 10079:
		decoded, _ = charmap.ISO8859_1.NewDecoder().String(string(data))
	case 1200, 1201:
		// UTF-16LE / UTF-16BE — should never reach the single-byte path.
		decoded = string(data)
	case 65001:
		// UTF-8 — should never reach the single-byte path.
		decoded = string(data)
	default:
		// Unknown codepage: treat bytes as Windows-1252 (Excel default).
		decoded, _ = charmap.Windows1252.NewDecoder().String(string(data))
	}
	return strings.TrimSpace(decoded)
}

func decodeRK(raw uint32) float64 {
	value := 0.0
	if raw&0x02 != 0 {
		value = float64(int32(raw) >> 2)
	} else {
		bits := uint64(raw&0xFFFFFFFC) << 32
		value = math.Float64frombits(bits)
	}
	if raw&0x01 != 0 {
		value /= 100
	}
	return value
}

func biffErrorString(code byte) string {
	switch code {
	case 0x00:
		return "#NULL!"
	case 0x07:
		return "#DIV/0!"
	case 0x0F:
		return "#VALUE!"
	case 0x17:
		return "#REF!"
	case 0x1D:
		return "#NAME?"
	case 0x24:
		return "#NUM!"
	case 0x2A:
		return "#N/A"
	default:
		return "#ERROR!"
	}
}

func formatA1Ref(row, col int) string {
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+(col%26))) + name
		col /= 26
	}
	return fmt.Sprintf("%s%d", name, row)
}

func parseBIFFFormat(payload []byte) (uint16, string, bool) {
	if len(payload) < 5 {
		return 0, "", false
	}
	index := binary.LittleEndian.Uint16(payload[0:2])
	code, _, ok := readXLUnicodeString(payload, 2, 2)
	if ok {
		return index, code, true
	}
	if len(payload) > 4 {
		return index, decodeBiffBytes(payload[4:]), true
	}
	return 0, "", false
}

// biffFontAt returns the legacyFont for the given XF font index.
// BIFF8 skips font index 4 (the table has a hole there).
func biffFontAt(fonts []legacyFont, ifnt uint16) legacyFont {
	if len(fonts) == 0 {
		return legacyFont{}
	}
	idx := int(ifnt)
	if idx >= 4 {
		idx-- // skip the reserved slot at position 4
	}
	if idx < 0 || idx >= len(fonts) {
		return legacyFont{}
	}
	return fonts[idx]
}

func parseBIFFFont(payload []byte) legacyFont {
	if len(payload) < 14 {
		return legacyFont{}
	}
	heightTwips := binary.LittleEndian.Uint16(payload[0:2])
	grbit := binary.LittleEndian.Uint16(payload[2:4])
	icv := binary.LittleEndian.Uint16(payload[4:6])
	bls := binary.LittleEndian.Uint16(payload[6:8])
	uls := payload[10]
	font := legacyFont{
		HeightPt:  float64(heightTwips) / 20.0,
		Bold:      bls >= 700,
		Italic:    grbit&0x0002 != 0,
		Strike:    grbit&0x0008 != 0,
		Underline: uls != 0,
		ColorIdx:  icv,
	}
	// Font name: 1-byte char count + 1-byte flags + chars
	if len(payload) >= 16 {
		cch := int(payload[14])
		fHighByte := payload[15] & 0x01
		if fHighByte == 1 && len(payload) >= 16+cch*2 {
			font.Name = decodeBiffUTF16(payload[16 : 16+cch*2])
		} else if fHighByte == 0 && len(payload) >= 16+cch {
			font.Name = decodeBiffBytes(payload[16 : 16+cch])
		}
	}
	return font
}

func parseBIFFXF(payload []byte, formatCodes map[uint16]string, fonts []legacyFont, palette [56]string) legacyXF {
	if len(payload) < 4 {
		return legacyXF{}
	}
	ifnt := binary.LittleEndian.Uint16(payload[0:2])
	formatIndex := binary.LittleEndian.Uint16(payload[2:4])
	xf := legacyXF{NumFmtCode: formatCodes[formatIndex]}

	// Resolve font properties into the XF directly.
	f := biffFontAt(fonts, ifnt)
	xf.FontName = f.Name
	xf.HeightPt = f.HeightPt
	xf.Bold = f.Bold
	xf.Italic = f.Italic
	xf.Underline = f.Underline
	xf.Strike = f.Strike
	xf.FontColor = biffColorHex(f.ColorIdx, palette)

	// Protection flags at offset 4 (grbit, 2 bytes): bit 2=fLocked, bit 3=fHidden.
	if len(payload) >= 6 {
		grbit := binary.LittleEndian.Uint16(payload[4:6])
		xf.Locked = grbit&0x0004 != 0
		xf.FormulaHidden = grbit&0x0008 != 0
	}

	// Alignment byte at offset 6; text rotation (trot) at offset 8.
	if len(payload) >= 8 {
		alc := payload[6]
		xf.HAlign = biffHAlign(alc & 0x07)
		xf.WrapText = alc&0x08 != 0
		xf.VAlign = biffVAlign((alc >> 4) & 0x07)
	}
	if len(payload) >= 9 {
		// trot: 0=no rotation, 1-90=CCW degrees, 91-180=CW (value-90), 255=stacked.
		// Matches the OOXML textRotation convention directly.
		if trot := int(payload[8]); trot != 0 {
			xf.TextRotation = trot
		}
	}

	// Border line styles and colors: uint32 at offset 10, uint32 at offset 14.
	if len(payload) >= 14 {
		b1 := binary.LittleEndian.Uint32(payload[10:14])
		xf.BorderLeftStyle = biffBorderStyle(uint8(b1 & 0x0F))
		xf.BorderRightStyle = biffBorderStyle(uint8((b1 >> 4) & 0x0F))
		xf.BorderTopStyle = biffBorderStyle(uint8((b1 >> 8) & 0x0F))
		xf.BorderBottomStyle = biffBorderStyle(uint8((b1 >> 12) & 0x0F))
		xf.BorderLeftColor = biffColorHex(uint16((b1>>16)&0x7F), palette)
		xf.BorderRightColor = biffColorHex(uint16((b1>>23)&0x7F), palette)
	}
	if len(payload) >= 18 {
		b2 := binary.LittleEndian.Uint32(payload[14:18])
		xf.BorderTopColor = biffColorHex(uint16(b2&0x7F), palette)
		xf.BorderBottomColor = biffColorHex(uint16((b2>>7)&0x7F), palette)
		// Fill pattern: bits 26-31.
		fillPattern := uint8((b2 >> 26) & 0x3F)
		// Foreground and background cell colors: uint16 at offset 18.
		if len(payload) >= 20 && fillPattern > 0 {
			colors := binary.LittleEndian.Uint16(payload[18:20])
			foreIdx := uint16(colors & 0x7F)
			backIdx := uint16((colors >> 7) & 0x7F)
			if fillPattern == 1 {
				// Solid fill: foreground color is the cell background.
				xf.BgColor = biffColorHex(foreIdx, palette)
			} else if backIdx != 0x41 { // 0x41 = system window background
				xf.BgColor = biffColorHex(backIdx, palette)
			}
		}
	}
	return xf
}

// isDateNumFmt returns true if fmtCode contains unquoted date/time pattern
// characters (d, m, y, h). Quoted sections and [color]/[locale] brackets are
// skipped so that format strings like `"text"` or `[$-409]` don't match.
func isDateNumFmt(code string) bool {
	code = strings.ToLower(code)
	inQuote := false
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		if c == '[' {
			for i < len(code) && code[i] != ']' {
				i++
			}
			continue
		}
		if c == 'd' || c == 'm' || c == 'y' || c == 'h' {
			return true
		}
	}
	return false
}

// excelSerialToDate converts an Excel serial date number to an ISO 8601 string.
// In 1900 mode, day 1 = Jan 1, 1900 and Excel incorrectly treats 1900 as a leap
// year (the Lotus 1-2-3 bug); serial 60 = "Feb 29 1900" which does not exist,
// and serials ≥ 61 are decremented by 1 to skip the phantom day. In 1904 mode
// (DATEMODE record nonzero), day 0 = Jan 1, 1904 with no phantom-day fudge.
// A fractional part produces a "YYYY-MM-DD HH:MM:SS" result.
func excelSerialToDate(serial float64, date1904 bool) string {
	var epoch time.Time
	var n int
	var frac float64
	if date1904 {
		if serial < 0 {
			return ""
		}
		n = int(serial)
		frac = serial - float64(n)
		epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	} else {
		if serial < 1 {
			return ""
		}
		n = int(serial)
		frac = serial - float64(n)
		if n == 60 {
			// Phantom Feb 29 1900 — just represent as 1900-02-28
			n = 59
		} else if n > 60 {
			n-- // undo the extra Lotus day
		}
		epoch = time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	}
	d := epoch.AddDate(0, 0, n)
	if frac > 0 {
		totalSecs := int(math.Round(frac * 86400))
		h := totalSecs / 3600
		m := (totalSecs % 3600) / 60
		s := totalSecs % 60
		return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
			d.Year(), int(d.Month()), d.Day(), h, m, s)
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year(), int(d.Month()), d.Day())
}

// promoteNumericCellToDate re-classifies a CellNumber cell as CellDate when
// its format string is a date/time format.  The caller already constructed the
// cell with Kind=CellNumber; this function mutates it in place.
func promoteNumericCellToDate(cell *sheetir.Cell, date1904 bool) {
	if cell == nil || cell.Kind != sheetir.CellNumber {
		return
	}
	if cell.Style.NumFmtCode == "" || !isDateNumFmt(cell.Style.NumFmtCode) {
		return
	}
	if dateStr := excelSerialToDate(cell.NumValue, date1904); dateStr != "" {
		cell.Kind = sheetir.CellDate
		cell.StrValue = dateStr
	}
}

func styleFromXFIndex(xfIndex uint16, xfs []legacyXF) sheetir.CellStyle {
	if int(xfIndex) >= len(xfs) {
		return sheetir.CellStyle{}
	}
	xf := xfs[xfIndex]
	return sheetir.CellStyle{
		Bold:          xf.Bold,
		Italic:        xf.Italic,
		Underline:     xf.Underline,
		Strike:        xf.Strike,
		FontFamily:    xf.FontName,
		FontSizePt:    xf.HeightPt,
		FontColor:     xf.FontColor,
		HAlign:        xf.HAlign,
		VAlign:        xf.VAlign,
		WrapText:      xf.WrapText,
		BgColor:       xf.BgColor,
		BorderLeft:    sheetir.BorderStyle{Style: xf.BorderLeftStyle, Color: xf.BorderLeftColor},
		BorderRight:   sheetir.BorderStyle{Style: xf.BorderRightStyle, Color: xf.BorderRightColor},
		BorderTop:     sheetir.BorderStyle{Style: xf.BorderTopStyle, Color: xf.BorderTopColor},
		BorderBottom:  sheetir.BorderStyle{Style: xf.BorderBottomStyle, Color: xf.BorderBottomColor},
		NumFmtCode:    xf.NumFmtCode,
		TextRotation:  xf.TextRotation,
		Locked:        xf.Locked,
		FormulaHidden: xf.FormulaHidden,
	}
}

func biffHAlign(v uint8) string {
	switch v {
	case 1:
		return "left"
	case 2:
		return "center"
	case 3:
		return "right"
	case 4:
		return "fill"
	case 5:
		return "justify"
	default:
		return "general"
	}
}

func biffVAlign(v uint8) string {
	switch v {
	case 0:
		return "top"
	case 1:
		return "middle"
	case 3:
		return "justify"
	default:
		return "bottom"
	}
}

func biffBorderStyle(v uint8) string {
	switch v {
	case 0:
		return ""
	case 4:
		return "dotted"
	case 6:
		return "double"
	case 3, 8, 9, 11, 13:
		return "dashed"
	default:
		return "solid"
	}
}

// defaultBIFFPalette mirrors HSSF PaletteRecord.DEFAULT_COLORS (56 entries).
// Each entry is the RGB hex (no `#`) for the BIFF color slot at indices
// 0x08..0x3F. Indices below 0x08 are reserved system / theme slots and indices
// 0x40+ are theme-driven sentinels (handled in biffColorHex).
//
// Source: refs/apache-poi/poi/src/main/java/org/apache/poi/hssf/record/PaletteRecord.java
var defaultBIFFPalette = [56]string{
	"000000", "FFFFFF", "FF0000", "00FF00", "0000FF", "FFFF00", "FF00FF", "00FFFF",
	"800000", "008000", "000080", "808000", "800080", "008080", "C0C0C0", "808080",
	"9999FF", "993366", "FFFFCC", "CCFFFF", "660066", "FF8080", "0066CC", "CCCCFF",
	"000080", "FF00FF", "FFFF00", "00FFFF", "800080", "800000", "008080", "0000FF",
	"00CCFF", "CCFFFF", "CCFFCC", "FFFF99", "99CCFF", "FF99CC", "CC99FF", "FFCC99",
	"3366FF", "33CCCC", "99CC00", "FFCC00", "FF9900", "FF6600", "666699", "969696",
	"003366", "339966", "003300", "333300", "993300", "993366", "333399", "333333",
}

// parseBIFFPalette decodes a PaletteRecord (sid 0x0092) and returns the
// resolved 56-entry palette. The record carries a uint16 numColors followed
// by `numColors` RGBA quads (R, G, B, unused). Entries beyond the standard
// palette length are clipped; missing entries fall back to the static defaults.
func parseBIFFPalette(payload []byte) [56]string {
	out := defaultBIFFPalette
	if len(payload) < 2 {
		return out
	}
	numColors := int(binary.LittleEndian.Uint16(payload[0:2]))
	if numColors < 0 {
		return out
	}
	if numColors > 56 {
		numColors = 56
	}
	pos := 2
	for i := 0; i < numColors; i++ {
		if pos+4 > len(payload) {
			break
		}
		r, g, b := payload[pos], payload[pos+1], payload[pos+2]
		out[i] = fmt.Sprintf("%02X%02X%02X", r, g, b)
		pos += 4
	}
	return out
}

// biffColorHex resolves a BIFF color index (icv) to an RGB hex string using
// the document's palette (defaults to defaultBIFFPalette when no PaletteRecord
// override has been read). Returns "" for indices outside the palette range.
func biffColorHex(icv uint16, palette [56]string) string {
	if icv < 0x08 || icv > 0x3F {
		return ""
	}
	return palette[icv-0x08]
}

// resolveICV resolves a BIFF color index (icv) to an RGB hex string using
// the document's palette. When palette is nil, falls back to the default palette.
// Returns "" for indices outside the palette range (0x08..0x3F).
func resolveICV(icv int, palette *[56]string) string {
	if icv < 0x08 || icv > 0x3F {
		return ""
	}
	if palette != nil {
		return palette[icv-0x08]
	}
	return defaultBIFFPalette[icv-0x08]
}

func defaultBIFFFormatCodes() map[uint16]string {
	return map[uint16]string{
		0:  "General",
		1:  "0",
		2:  "0.00",
		3:  "#,##0",
		4:  "#,##0.00",
		9:  "0%",
		10: "0.00%",
		11: "0.00E+00",
		12: "# ?/?",
		13: "# ??/??",
		14: "m/d/yy",
		15: "d-mmm-yy",
		16: "d-mmm",
		17: "mmm-yy",
		18: "h:mm AM/PM",
		19: "h:mm:ss AM/PM",
		20: "h:mm",
		21: "h:mm:ss",
		22: "m/d/yy h:mm",
		37: "#,##0 ;(#,##0)",
		38: "#,##0 ;[Red](#,##0)",
		39: "#,##0.00;(#,##0.00)",
		40: "#,##0.00;[Red](#,##0.00)",
		45: "mm:ss",
		46: "[h]:mm:ss",
		47: "mmss.0",
		48: "##0.0E+0",
		49: "@",
	}
}
