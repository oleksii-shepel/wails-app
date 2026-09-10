package ir

import (
	"office/pkg/diag"
	officeartir "office/pkg/officeart/ir"
	"office/pkg/ooxml/chart"
	dml "office/pkg/ooxml/drawing"
)

// Document is a minimal semantic IR derived from WordprocessingML parts.
type Document struct {
	Paragraphs        []Paragraph
	Tables            []Table
	Blocks            []Block
	Fields            []StoryField // block-spanning complex fields in body
	Sections          []Section
	Title             string
	Subject           string
	Author            string
	UserName          string // user name for USERNAME field; falls back to Author
	Description       string
	Keywords          string
	LastModifiedBy    string
	Company           string
	Manager           string
	Application       string
	AppVersion        string
	RevNum            string
	Template          string
	TemplatePath      string // attached template path from w:settings/w:attachedTemplate
	TotalEditMinutes  int    // docProps/app.xml TotalTime in minutes when available
	CustomProperties  map[string]string
	DocumentVariables map[string]string
	CustomXMLParts    map[string][]byte
	// HasSignatures reports whether the source package contained an OPC digital
	// signature origin part.
	HasSignatures bool
	// DigitalSignatures exposes parsed XML-DSig metadata together with OPC
	// verification results discovered in the source package.
	DigitalSignatures          []DigitalSignature
	Filename                   string // base filename, e.g. "report.docx"
	FootnoteNumberFormat       string // numbering format from settings (e.g. "decimal", "lowerRoman")
	FootnoteNumberStart        int    // display start number from settings; 0 = default
	FootnoteRestart            string // "continuous" | "eachSect" | "eachPage" | ""
	FootnoteSeparator          Story
	FootnoteContinuationSep    Story
	FootnoteContinuationNotice Story
	EndnotePosition            string // "docEnd" | "sectEnd" | "" (defaults to document-end behavior)
	EndnoteNumberFormat        string // numbering format from settings (e.g. "decimal", "lowerRoman")
	EndnoteNumberStart         int    // display start number from settings; 0 = default
	EndnoteRestart             string // "continuous" | "eachSect" | "eachPage" | ""
	EndnoteSeparator           Story
	EndnoteContinuationSep     Story
	EndnoteContinuationNotice  Story
	Headers                    map[string]Story
	Footers                    map[string]Story
	DefaultParagraphStyleID    string
	DefaultCharacterStyleID    string
	DefaultParagraphFormat     ParagraphFormat
	DefaultRunFormat           RunFormat
	Styles                     map[string]Style
	CharacterStyles            map[string]CharacterStyle
	// FontFamilies stores word/fontTable.xml metadata keyed by normalized font family name.
	FontFamilies  map[string]FontFamilyInfo
	EmbeddedFonts []*EmbeddedFont
	// LatentStyles stores the word/styles.xml latentStyles configuration.
	LatentStyles *LatentStyles
	Footnotes    map[int]Story // note ID → resolved story
	Endnotes     map[int]Story // note ID → resolved story
	Comments     map[int]Story // comment ID → resolved story
	// DefaultTabStop is the document-wide default tab interval in twips
	// (legacy DOC: DOP.dxaTab; OOXML: w:settings > w:defaultTabStop). 0 = unset;
	// renderers should fall back to 720 twips (Word's hardcoded default).
	DefaultTabStop int
	// BookmarkEnds maps bookmark name to its exclusive end character position
	// in the source document. Populated from PlcfBkl (FIB index 20) for legacy
	// .doc files. Used by renderers to compute bookmark spans.
	BookmarkEnds map[string]int
	Diagnostics  diag.List
	// Source carries the raw binary provenance for the whole document. It is
	// populated by lossless parsers and used by projection layers.
	Source *DocumentSource
	// Compatibility stores document-wide compatibility flags that affect layout
	// and rendering behavior (legacy DOP or w:settings/w:compat).
	Compatibility *DocCompatibility
	// CompatibilityMode stores the Word compatibility mode version
	// (e.g., "15" for Word 2013, from w:settings/w:compatSetting).
	CompatibilityMode string
	// ZoomPercent stores the document's zoom percentage (legacy DOP wScaleSaved).
	// 0 means unset / 100% default.
	ZoomPercent int
	// ViewMode stores the document's saved view mode (legacy DOP wvkSaved).
	// Values: "print", "outline", "masterDoc", "normal", "online", "web", or "".
	ViewMode string
	// ProtectionEnabled indicates whether document protection is active
	// (legacy DOP fProtEnabled).
	ProtectionEnabled bool
	// CreatedAt is the document creation timestamp from DOP.dttmCreated,
	// formatted as ISO-8601 (YYYY-MM-DDTHH:MM:SS). Empty when unset.
	CreatedAt string
	// ModifiedAt is the last revision timestamp from DOP.dttmRevised.
	ModifiedAt string
	// LastPrintedAt is the last print timestamp from DOP.dttmLastPrint.
	LastPrintedAt string
	// WordCount stores the document's word count from DOP.cWords.
	WordCount int
	// CharCount stores the document's character count from DOP.cCh.
	CharCount int
	// PageCount stores the document's page count from DOP.cPg.
	PageCount int
	// ParagraphCount stores the document's paragraph count from DOP.cParas.
	ParagraphCount int
	// NumberingDefinitions holds editor-created numbering definitions that
	// are serialized to numbering.xml when converting to OOXML.
	NumberingDefinitions []NumberingDefinition
	// MirrorMargins indicates whether left/right margins are mirrored on
	// facing pages (legacy DOP fMirrorMargins).
	MirrorMargins bool
	// TrackChangesEnabled indicates whether track-changes mode was active
	// when the document was saved (legacy DOP fRevMarking).
	TrackChangesEnabled bool
	// DocumentProtection stores editing restrictions when present.
	DocumentProtection *DocumentProtection
	// CharacterSpacingControl is "compressPunctuation", "compressPunctuationAndJapaneseKana", or "".
	CharacterSpacingControl string
	// AutoHyphenation is true when automatic hyphenation is enabled.
	AutoHyphenation bool
	// ConsecutiveHyphenLimit is the maximum number of consecutive lines that may end with a hyphen.
	ConsecutiveHyphenLimit int
	// HyphenationZone is the hyphenation zone width in twips.
	HyphenationZone int
	// DoNotHyphenateCaps suppresses hyphenation of words in ALL CAPS.
	DoNotHyphenateCaps bool
	// EvenAndOddHeaders is true when even/odd header/footer pairs differ.
	EvenAndOddHeaders bool
	// UpdateFields is true when fields should be auto-updated on open.
	UpdateFields bool
	// ThemeFontLang stores the theme font language settings.
	ThemeFontLang *ThemeFontLang
	// ProofState stores spelling/grammar proofing state.
	ProofState *ProofState
	// MailMerge stores mail-merge settings when present.
	MailMerge *MailMergeSettings
	// DocumentType is "notSpecified", "letter", "email", or "".
	DocumentType string
	// DefaultTableStyle is the document-wide default table style name
	// (w:settings/w:defaultTableStyle). When a table has no explicit style,
	// this style is applied. Empty means no default is set.
	DefaultTableStyle string
	// BackgroundColor is the document page background color as a hex RGB string
	// (e.g., "E6E6FA"), from w:background@w:color. Empty means no background
	// color was specified.
	BackgroundColor string
	// BackgroundImage is the optional page background image resolved from
	// w:background picture relationships.
	BackgroundImage *Image
	// DisplayBackgroundShape is true when the document background shape
	// (page background color/image) should be displayed (w:settings/w:displayBackgroundShape).
	// nil means unset; renderers should show the background when a color/image is present.
	// Explicit false suppresses the background.
	DisplayBackgroundShape *bool
	// DoNotDisplayPageBoundaries suppresses display of page boundaries in page layout view.
	DoNotDisplayPageBoundaries bool
	// BordersDoNotSurroundFooter prevents page borders from surrounding the footer.
	BordersDoNotSurroundFooter bool
	// BordersDoNotSurroundHeader prevents page borders from surrounding the header.
	BordersDoNotSurroundHeader bool
	// DoNotTrackFormatting suppresses tracking of formatting changes.
	DoNotTrackFormatting bool
	// DoNotTrackMoves suppresses tracking of move changes.
	DoNotTrackMoves bool
	// ActiveWritingStyles holds the active writing style configurations from
	// w:settings/w:activeWritingStyle. Each entry specifies a grammar checker
	// language and vendor configuration.
	ActiveWritingStyles []ActiveWritingStyle
	// WriteProtection holds write-protection settings from
	// w:settings/w:writeProtection. Nil means no write protection is set.
	WriteProtection *WriteProtection
	// RevisionSaveIDs holds the list of revision save IDs from
	// w:settings/w:rsids. These are used to track document revision history.
	RevisionSaveIDs []string
	// MiscSettings holds long-tail settings parsed from w:settings that do not
	// have dedicated top-level fields on Document.
	MiscSettings *MiscSettings
	// Theme holds the resolved DrawingML theme data from word/theme/theme1.xml.
	// nil when no theme part is present (e.g. legacy .doc source).
	Theme *Theme
}

// MiscSettings groups miscellaneous document settings from w:settings that are
// parsed but rarely affect rendering.  These are authoring-time or print-time
// behaviours rather than layout-critical properties.
type MiscSettings struct {
	// Drawing grid.
	DisplayHorizontalDrawingGrid        int
	DisplayVerticalDrawingGrid          int
	DoNotUseMarginsForDrawingGridOrigin bool
	DrawingGridHorizontalSpacing        int
	DrawingGridVerticalSpacing          int
	DrawingGridHorizontalOrigin         int
	DrawingGridVerticalOrigin           int

	// Book fold / printing.
	BookFoldRevPrinting           bool
	BookFoldPrintingSheets        int
	BookFoldPrinting              bool
	PrintTwoOnOne                 bool
	PrintFormsData                bool
	PrintPostScriptOverText       bool
	PrintFractionalCharacterWidth bool

	// Symbols / separators.
	DecimalSymbol string
	ListSeparator string

	// Font embedding.
	EmbedTrueTypeFonts bool
	EmbedSystemFonts   bool
	SaveSubsetFonts    bool
	SaveFormsData      bool

	// Display / UI.
	ShowXMLTags                bool
	DoNotDisplayPageBoundaries bool
	DisplayBackgroundShape     *bool
	HideSpellingErrors         bool
	HideGrammaticalErrors      bool
	FormsDesign                bool
	LinkStyles                 bool
	RevisionView               bool
	StyleLockTheme             bool
	StyleLockQFSet             bool
	ShowEnvelope               bool
	StrictFirstAndLastChars    bool
	DoNotShadeFormData         bool
	NoPunctuationKerning       bool
	AlignBordersAndEdges       bool
	GutterAtTop                bool

	// Validation / saving.
	DoNotValidateAgainstSchema bool
	SaveInvalidXml             bool
	IgnoreMixedContent         bool
	AlwaysShowPlaceholderText  bool
	DoNotDemarcateInvalidXml   bool
	SaveXmlDataOnly            bool
	UseXSLTWhenSaving          bool
	AlwaysMergeEmptyNamespace  bool
	DoNotIncludeSubdocsInStats bool
	DoNotAutoCompressPictures  bool
	DoNotEmbedSmartTags        bool
	ForceUpgrade               bool

	// Paragraph / layout behaviour.
	Active                            bool
	BodyDiv                           bool
	CachedColBalance                  bool
	DoNotSnapToGridInCell             bool
	DoNotSuppressIndentation          bool
	DoNotSuppressParagraphBorders     bool
	DoNotUseEastAsianBreakRules       bool
	DoNotUseHTMLParagraphAutoSpacing  bool
	DoNotWrapTextWithPunct            bool
	GrowAutofit                       bool
	LvlRestart                        int
	NumIDMacAtCleanup                 int
	SaveSmartTagsAsXml                bool
	SelectFldWithFirstOrLastChar      bool
	SplitPgBreakAndParaMark           bool
	StyleLink                         string
	SuppressBottomSpacing             bool
	SuppressOverlap                   bool
	SuppressSpacingAtTopOfPage        bool
	SuppressTopSpacing                bool
	SuppressTopSpacingWP              bool
	SwapBordersFacingPages            bool
	TextboxTightWrap                  bool
	UnderlineTabInNumList             bool
	UseAnsiKerningPairs               bool
	UsePrinterMetrics                 bool
	UseSingleBorderforContiguousCells bool

	// Style pane / captions / read mode.
	StylePaneFormatFilter string
	StylePaneSortMethod   string
	Captions              *Captions
	ReadModeInkLockDown   *ReadModeInkLockDown

	// Smart tags / schemas.
	SmartTagTypes   []SmartTagType
	AttachedSchemas []string
}

// ReadModeInkLockDown holds read-mode ink lock-down settings.
type ReadModeInkLockDown struct {
	ActualPage bool
	Width      int
	Height     int
	FontSize   int
}

// Caption holds a single caption configuration.
type Caption struct {
	Name      string
	Position  string
	ChapNum   bool
	Heading   int
	NoLabel   bool
	NumFormat string
	Separator string
}

// AutoCaption holds a single auto-caption configuration.
type AutoCaption struct {
	Name    string
	Caption string
}

// Captions holds caption and auto-caption settings.
type Captions struct {
	Captions     []Caption
	AutoCaptions []AutoCaption
}

// SmartTagType holds a single smart-tag type entry.
type SmartTagType struct {
	NamespaceURI string
	Name         string
	URL          string
}

// DocumentProtection captures document editing restrictions.
type DocumentProtection struct {
	Edit           string
	Enforcement    bool
	FormatSHA1     string
	HashValue      string
	SaltValue      string
	SpinCount      int
	AlgorithmSID   int
	CryptProvType  string
	CryptAlgorithm string
}

// ThemeFontLang stores theme font language settings.
type ThemeFontLang struct {
	Val      string
	EastAsia string
	Bidi     string
}

// Theme holds the resolved DrawingML theme data for a document.
type Theme struct {
	Name         string
	ColorScheme  map[string]string
	FontScheme   ThemeFontScheme
	FormatScheme ThemeFormatScheme
}

// DigitalSignature is parsed XML-DSig metadata attached to a DOCX package.
type DigitalSignature struct {
	PartName               string
	ContentType            string
	ID                     string
	SignatureMethod        string
	CanonicalizationMethod string
	References             []DigitalSignatureReference
	Signer                 *DigitalSignatureCertificate
	Certificates           []DigitalSignatureCertificate
	TrustChain             []DigitalSignatureCertificate
	Valid                  bool
	SignatureValid         bool
	ReferencesValid        bool
	TrustValid             bool
	RevocationChecked      bool
	Revoked                bool
	SigningTime            string
	SigningTimeFormat      string
	SigningTimeValid       bool
	TrustedTimestamp       bool
	VerificationErrors     []string
}

// DigitalSignatureReference identifies one signed resource reference.
type DigitalSignatureReference struct {
	URI          string
	Type         string
	DigestMethod string
}

// DigitalSignatureCertificate is a simplified X.509 certificate summary.
type DigitalSignatureCertificate struct {
	Subject      string
	Issuer       string
	SerialNumber string
	NotBefore    string
	NotAfter     string
}

// ThemeFontScheme holds the major and minor font sets.
type ThemeFontScheme struct {
	MajorFont ThemeFontSet
	MinorFont ThemeFontSet
}

// ThemeFontSet holds typefaces for one font slot.
type ThemeFontSet struct {
	Latin         string
	EastAsia      string
	ComplexScript string
}

// ThemeFormatScheme holds resolved fill, line, effect, and background fill styles.
type ThemeFormatScheme struct {
	FillStyles   []dml.Fill
	LineStyles   []*dml.LineStyle
	EffectStyles []dml.EffectList
	BgFillStyles []dml.Fill
}

// ProofState stores spelling/grammar proofing state.
type ProofState struct {
	Spelling string
	Grammar  string
}

// MailMergeSettings stores mail-merge settings.
type MailMergeSettings struct {
	MainDocumentType        string
	LinkToQuery             bool
	DataType                string
	ConnectString           string
	Query                   string
	DataSource              string
	HeaderSource            string
	Destination             string
	ActiveRecord            string
	AddressFieldName        string
	CheckErrors             string
	DoNotSuppressBlankLines bool
	DynamicAddress          bool
	EntryMacro              string
	ExitMacro               string
	DataSourceHasHeaders    bool
	LinkedToFile            bool
	MailAsAttachment        bool
	MailSubject             string
	MappedName              string
	SourceFileName          string
	StoreMappedDataAs       string
	SubDoc                  string
	UDL                     string
	ViewMergedData          bool
	Personal                bool
	PersonalCompose         bool
	PersonalReply           bool
	// Types preserves w:mailMerge/w:types/w:type values.
	Types []string
	// Recipients preserves top-level w:mailMerge/w:recipients entries.
	Recipients []RecipientData
	// ODSO holds nested Office Data Source Object metadata when present.
	ODSO *ODSOSettings
}

// ODSOSettings captures nested mail-merge data-source metadata.
type ODSOSettings struct {
	ColDelim       int
	DynamicAddress bool
	FHdr           bool
	SourceFileName string
	UDL            string
	FieldMapData   []FieldMapData
	RecipientData  []RecipientData
}

// FieldMapData captures one field mapping from w:odso/w:fieldMapData.
type FieldMapData struct {
	MappedName string
	Name       string
	Lang       string
	Val        string
	Column     int
}

// RecipientData captures one recipient entry from w:mailMerge/w:recipients or
// w:mailMerge/w:odso.
type RecipientData struct {
	Active bool
}

// ActiveWritingStyle holds a single w:activeWritingStyle entry from w:settings.
// It specifies the grammar checker configuration for a given language.
type ActiveWritingStyle struct {
	// Lang is the BCP 47 language tag (w:lang attribute).
	Lang string
	// VendorID identifies the grammar checker vendor (w:vendorID attribute).
	VendorID string
	// DLLVersion is the grammar checker DLL version (w:dllVersion attribute).
	DLLVersion int
	// NLCheck enables natural language checking (w:nlCheck attribute).
	NLCheck bool
	// CheckStyle enables style checking (w:checkStyle attribute).
	CheckStyle bool
	// AppName is the application name for the grammar checker (w:appName attribute).
	AppName string
}

// WriteProtection holds write-protection settings from w:settings/w:writeProtection.
type WriteProtection struct {
	// Recommended indicates that write protection is recommended but not enforced.
	Recommended bool
	// HashValue is the base64-encoded hash of the protection password.
	HashValue string
	// SaltValue is the base64-encoded salt used in the password hash.
	SaltValue string
	// SpinCount is the number of hash iterations.
	SpinCount int
	// AlgorithmName is the hash algorithm name (e.g., "SHA-512").
	AlgorithmName string
}

// FontFamilyInfo captures resolved fontTable metadata used for fallback selection.
type FontFamilyInfo struct {
	Family  string // OOXML family hint such as roman, swiss, modern, script, decorative
	Pitch   string // OOXML pitch hint such as fixed or variable
	Charset string // OOXML charset hint, preserved for diagnostics/future matching
	Panose1 string // OOXML PANOSE hex string, preserved for future matching
}

// LatentStyle represents a single latent style exception (w:lsdException).
type LatentStyle struct {
	Name           string
	Locked         bool
	UIPriority     int
	SemiHidden     bool
	UnhideWhenUsed bool
	QFormat        bool
}

// LatentStyles holds the document-level latent styles configuration.
type LatentStyles struct {
	Count      int // w:latentStyles@count (def = 276 in OOXML spec)
	Exceptions []LatentStyle
}

// StoryField represents a complex field whose cached result spans multiple
// blocks (paragraphs, tables) within a story. It preserves the authored
// result structure separately from any renderer-computed semantic value.
type StoryField struct {
	Kind      string
	Instr     string
	Arg       string
	Anchor    string
	Target    string
	InstrRuns []Run
	Blocks    []Block // cached result blocks
}

// Story is a resolved header/footer story.
type Story struct {
	Paragraphs []Paragraph
	Tables     []Table
	Blocks     []Block
	Fields     []StoryField // block-spanning complex fields
	Text       string
	Author     string // comment author or initials (populated for comment stories)
	Date       string // comment date in YYYY-MM-DD format (populated for comment stories)
}

// Table is a normalized table node.
type Table struct {
	Index            int
	PropertyRevision *RevisionMark // OOXML tblPrChange when known
	Format           TableFormat
	Rows             []TableRow
	Text             string
	StyleID          string // table style ID from w:tblStyle
}

// TableFormat captures resolved table-level geometry.
type TableFormat struct {
	Width         *int   // twips; nil = auto
	WidthType     string // "dxa" | "pct" | "auto" | "nil"
	Layout        string // "fixed" | "autofit" | ""
	Alignment     string // "left" | "center" | "right"
	Shading       string // background fill from w:tblPr > w:shd
	Borders       *TableBorders
	ColumnWidths  []int            // twips, from w:tblGrid; empty = equal distribution
	Indent        int              // twips, from w:tblInd; 0 = no indent
	CellSpacing   int              // twips, inter-cell gap (legacy sprmTDxaGapHalf * 2)
	CellPadTop    int              // twips, default top cell padding for all cells
	CellPadBottom int              // twips, default bottom cell padding for all cells
	CellPadLeft   int              // twips, default left cell padding for all cells
	CellPadRight  int              // twips, default right cell padding for all cells
	FloatProps    *TableFloatProps // set when w:tblpPr is present; table is out-of-flow
	Overlap       string           // "never" | "overlap" | ""
	Caption       string           // table caption text
	Description   string           // table description / alt text
	BidiVisual    bool             // true when bidiVisual is enabled
	// Look holds conditional table-style formatting flags (w:tblLook).
	Look *TableLook
	// StyleRowBandSize is the number of rows in each row band for conditional
	// table style formatting (w:tblStyleRowBandSize). 0 means unset (default 1).
	StyleRowBandSize int
	// StyleColBandSize is the number of columns in each column band for
	// conditional table style formatting (w:tblStyleColBandSize). 0 means unset (default 1).
	StyleColBandSize int
}

// TableLook holds conditional table-style formatting flags.
type TableLook struct {
	Val         string
	FirstRow    bool
	LastRow     bool
	FirstColumn bool
	LastColumn  bool
	NoHBand     bool
	NoVBand     bool
}

// TableFloatProps holds resolved floating-table positioning from w:tblpPr.
type TableFloatProps struct {
	HorzAnchor     string // "text"|"margin"|"page"
	VertAnchor     string // "text"|"margin"|"page"
	X              int    // horizontal offset from anchor (twips)
	Y              int    // vertical offset from anchor (twips)
	XAlign         string // "left"|"right"|"center"|"inside"|"outside"
	YAlign         string // "top"|"bottom"|"center"|"inline"
	LeftFromText   int    // wrap gap to left (twips)
	RightFromText  int    // wrap gap to right (twips)
	TopFromText    int    // wrap gap above (twips)
	BottomFromText int    // wrap gap below (twips)
}

// TableBorders holds resolved border sides for a table (ECMA-376 Part 1 §17.4.39).
type TableBorders struct {
	Top     *BorderSide
	Left    *BorderSide
	Start   *BorderSide
	Bottom  *BorderSide
	Right   *BorderSide
	End     *BorderSide
	InsideH *BorderSide
	InsideV *BorderSide
}

// BorderSide is a single resolved border edge.
type BorderSide struct {
	Val     string  // "single", "double", "dotted", "dashed", "none", …
	SizePt  float64 // border thickness in points (w:sz is eighths-of-a-point)
	SpacePt float64 // gap between text and border in points (w:space)
	Color   string  // hex RGB or ""
}

// TableRow is a normalized table row.
type TableRow struct {
	PropertyRevision *RevisionMark // OOXML trPrChange when known
	Format           TableRowFormat
	// Override carries table property exceptions from w:tblPrEx that apply to
	// this row only, overriding the table-level properties.
	Override *TableRowOverride
	Cells    []TableCell
}

// TableRowOverride carries resolved w:tblPrEx properties for a single row.
type TableRowOverride struct {
	Shading       string        // background fill from w:tblPrEx/w:shd
	Borders       *TableBorders // border override from w:tblPrEx/w:tblBorders
	CellPadTop    int           // twips, from w:tblPrEx/w:tblCellMar/w:top
	CellPadBottom int           // twips, from w:tblPrEx/w:tblCellMar/w:bottom
	CellPadLeft   int           // twips, from w:tblPrEx/w:tblCellMar/w:left
	CellPadRight  int           // twips, from w:tblPrEx/w:tblCellMar/w:right
	Alignment     string        // "left" | "center" | "right"
	Indent        int           // twips, from w:tblPrEx/w:tblInd
	BidiVisual    bool          // true when bidiVisual is enabled
}

// TableRowFormat captures resolved row-level geometry.
type TableRowFormat struct {
	Height          *int   // twips; nil = auto
	HeightRule      string // "exact" | "atLeast"
	IsHeader        bool   // w:tblHeader / sprmTFTableHeader: repeat as header on each page
	CantSplit       bool   // w:cantSplit / sprmTFCantSplit: keep row together across pages
	Hidden          bool   // w:hidden / sprmTFHiddenRow: hide row
	GridBefore      int
	GridAfter       int
	WidthBefore     *int
	WidthBeforeType string
	WidthAfter      *int
	WidthAfterType  string
	Justification   string // w:jc: "left" | "center" | "right"
	Conditional     string // w:cnfStyle
	DivID           int    // w:divId
	CellSpacing     int    // twips, from w:trPr/w:tblCellSpacing; 0 = no override
}

// TableCell is a normalized table cell.
type TableCell struct {
	PropertyRevision *RevisionMark // OOXML tcPrChange when known
	CellIns          *RevisionMark // OOXML w:cellIns — tracked cell insertion
	CellDel          *RevisionMark // OOXML w:cellDel — tracked cell deletion
	CellMerge        *RevisionMark // OOXML w:cellMerge — tracked cell merge
	Format           TableCellFormat
	Paragraphs       []Paragraph
	Tables           []Table
	Blocks           []Block
	Text             string
	ContentControl   *ContentControl // non-nil when this cell is wrapped in a w:sdt
}

// TableCellFormat captures resolved cell-level geometry and appearance.
type TableCellFormat struct {
	Width         *int   // twips; nil = auto
	WidthType     string // "dxa" | "pct" | "auto" | "nil"
	GridSpan      int    // column span; 0 and 1 both mean 1
	VMerge        string // "restart" | "continue" | ""
	HMerge        bool   // true when this cell is horizontally merged (legacy DOC)
	VertAlign     string // "top" | "center" | "bottom"
	TextDirection string // e.g. "tbRl" | "btLr"
	Shading       string // fill hex color; "" = none
	Borders       *CellBorders
	NoWrap        bool // w:noWrap — prevent text wrapping in cell
	HideMark      bool // w:hideMark — hide end-of-cell paragraph mark
	CellPadTop    int  // twips, from w:tcMar/w:top
	CellPadBottom int  // twips, from w:tcMar/w:bottom
	CellPadLeft   int  // twips, from w:tcMar/w:left
	CellPadRight  int  // twips, from w:tcMar/w:right
	// Cell3D holds approximate DrawingML 3D bevel/extrusion properties for
	// cells in DrawingML tables (a:tc). nil when absent.
	Cell3D *Cell3D
}

// Cell3D captures simplified 3D formatting for a table cell.
// Values are in EMUs (English Metric Units: 1 emu = 1/360000 cm).
type Cell3D struct {
	BevelTopWidth     int64
	BevelTopHeight    int64
	BevelBottomWidth  int64
	BevelBottomHeight int64
	ExtrusionColor    string // hex RGB, e.g. "808080"
	ContourColor      string // hex RGB
	Material          string // e.g. "legacyMatte", "clear"
}

// CellBorders holds the resolved border sides for a table cell (ECMA-376 Part 1 §17.4.6),
// including the two diagonal directions and logical start/end alternatives.
type CellBorders struct {
	Top    *BorderSide
	Left   *BorderSide
	Start  *BorderSide
	Bottom *BorderSide
	Right  *BorderSide
	End    *BorderSide
	TL2BR  *BorderSide // top-left to bottom-right diagonal
	TR2BL  *BorderSide // top-right to bottom-left diagonal
}

// Paragraph is a normalized paragraph-level semantic node.
type Paragraph struct {
	Index     int
	StyleID   string
	StyleName string
	StylePath []string
	Numbering *Numbering
	List      *ListMarker
	// PropertyRevision carries paragraph-format change metadata from OOXML
	// pPrChange / legacy DOC sprmPPropRMark. Nil means no tracked property
	// revision is known for this paragraph.
	PropertyRevision *RevisionMark
	Format           ParagraphFormat
	// DefaultRunFormat carries the effective run formatting for this paragraph's
	// paragraph mark (the invisible ¶ character). It is the merged result of the
	// paragraph style's run format, conditional table style overrides, and the
	// direct w:rPr inside w:pPr (ECMA-376 Part 1 §17.3.1.27). Renderers may use
	// it to style the visible paragraph mark when formatting marks are shown, or
	// to compute empty-paragraph min-height based on the paragraph's font size.
	DefaultRunFormat   *RunFormat
	Runs               []Run
	Text               string
	Bookmarks          []string // anchor names from w:bookmarkStart
	CommentRangeStarts []int    // comment IDs from w:commentRangeStart
	CommentRangeEnds   []int    // comment IDs from w:commentRangeEnd
	// MoveFromRangeStarts holds the tracked-change move-from range starts that
	// appear in this paragraph (w:moveFromRangeStart). The resolver does not
	// correlate these with inline positions, so renderers should treat them as
	// paragraph-level metadata.
	MoveFromRangeStarts []MoveRange
	// MoveFromRangeEnds holds the tracked-change move-from range ends that
	// appear in this paragraph (w:moveFromRangeEnd).
	MoveFromRangeEnds []int // move IDs from w:moveFromRangeEnd
	// MoveToRangeStarts holds the tracked-change move-to range starts that
	// appear in this paragraph (w:moveToRangeStart).
	MoveToRangeStarts []MoveRange
	// MoveToRangeEnds holds the tracked-change move-to range ends that appear
	// in this paragraph (w:moveToRangeEnd).
	MoveToRangeEnds []int // move IDs from w:moveToRangeEnd
	// PermRangeStarts holds permission range starts that appear in this paragraph
	// (w:permStart). The resolver does not correlate these with inline positions.
	PermRangeStarts []PermRange
	// PermRangeEnds holds permission range ends that appear in this paragraph
	// (w:permEnd).
	PermRangeEnds []int // permission IDs from w:permEnd
	// CustomXmlInsRangeStarts holds custom-xml insertion range starts (w:customXmlInsRangeStart).
	CustomXmlInsRangeStarts []MoveRange
	// CustomXmlInsRangeEnds holds custom-xml insertion range ends (w:customXmlInsRangeEnd).
	CustomXmlInsRangeEnds []int
	// CustomXmlDelRangeStarts holds custom-xml deletion range starts (w:customXmlDelRangeStart).
	CustomXmlDelRangeStarts []MoveRange
	// CustomXmlDelRangeEnds holds custom-xml deletion range ends (w:customXmlDelRangeEnd).
	CustomXmlDelRangeEnds []int
	// CustomXmlMoveFromRangeStarts holds custom-xml move-from range starts (w:customXmlMoveFromRangeStart).
	CustomXmlMoveFromRangeStarts []MoveRange
	// CustomXmlMoveFromRangeEnds holds custom-xml move-from range ends (w:customXmlMoveFromRangeEnd).
	CustomXmlMoveFromRangeEnds []int
	// CustomXmlMoveToRangeStarts holds custom-xml move-to range starts (w:customXmlMoveToRangeStart).
	CustomXmlMoveToRangeStarts []MoveRange
	// CustomXmlMoveToRangeEnds holds custom-xml move-to range ends (w:customXmlMoveToRangeEnd).
	CustomXmlMoveToRangeEnds []int
	// InlineMoveRanges holds inline move-range metadata with position correlation.
	// This is populated when move-range boundaries appear within a paragraph's runs.
	InlineMoveRanges []InlineMoveRange
	Frame            *FrameProps // set when w:framePr is present; paragraph is out-of-flow
	// Source carries the raw binary provenance for this paragraph. It is
	// populated by lossless parsers (e.g. legacy .doc) and used by projection
	// layers to (re)compute effective formatting or round-trip to binary.
	Source *ParagraphSource
}

// LegacyStyleIndex returns the raw legacy paragraph style index when the
// paragraph originated from a binary .doc source. Provenance in Source takes
// precedence over the older staging mirror on Format.
func (p Paragraph) LegacyStyleIndex() *int {
	if p.Source != nil && p.Source.StyleIndex != nil {
		return p.Source.StyleIndex
	}
	return p.Format.BinaryStyleIndex
}

// FrameProps holds resolved w:framePr positioning for a legacy frame paragraph.
type FrameProps struct {
	W             int    // width in twips (0 = auto)
	H             int    // height in twips (0 = auto)
	X             int    // horizontal offset in twips
	Y             int    // vertical offset in twips
	XAlign        string // "left"|"right"|"center"|"inside"|"outside"|""
	YAlign        string // "top"|"bottom"|"center"|"inline"|""
	HAnchor       string // "text"|"margin"|"page"
	VAnchor       string // "text"|"margin"|"page"
	WrapType      string // "none"|"around"|"notBeside"
	HSpace        int    // horizontal space in twips
	VSpace        int    // vertical space in twips
	DropCap       string // "drop"|"margin"|"none"|"" — non-empty/non-"none" means drop cap
	Lines         int    // number of body lines the drop cap spans (w:lines)
	TextDirection string // "tbRl" | "btLr" | "tbRlV" | "" — from frame text-flow SPRM
	// AnchorLock prevents the frame anchor from moving when the paragraph moves
	// (w:anchorLock / sprmPFLocked). false = anchor can move (default).
	AnchorLock bool
}

// Style is a normalized paragraph style definition.
type Style struct {
	ID             string
	Name           string
	BasedOn        string
	Default        bool
	LinkedCharID   string // ID of the linked character style (w:link)
	NextStyleID    string
	UIPriority     int
	RSID           string
	UnhideWhenUsed bool
	QFormat        bool
	SemiHidden     bool
	AutoRedefine   bool
	StylePath      []string
	RawFormat      ParagraphFormat
	Format         ParagraphFormat
	RawRunFormat   RunFormat               // run properties defined directly on this style
	RunFormat      RunFormat               // effective run format through the basedOn chain
	Numbering      *NumberingReference     // numPr from the style's own pPr, if any
	TableFormat    *TableFormat            // base tblPr for table styles; nil for non-table styles
	TableStyles    []ConditionalTableStyle // tblStylePr conditional formatting; nil for non-table styles
	// Source carries the raw binary provenance for this style.
	Source *StyleSource
}

// NumberingReference is a raw numId+ilvl pointer from a pPr element.
type NumberingReference struct {
	NumID int
	Level int
}

// CharacterStyle is a normalized character style definition.
type CharacterStyle struct {
	ID             string
	Name           string
	BasedOn        string
	NextStyleID    string
	UIPriority     int
	RSID           string
	UnhideWhenUsed bool
	QFormat        bool
	SemiHidden     bool
	AutoRedefine   bool
	StylePath      []string
	RawRunFormat   RunFormat
	Format         RunFormat // effective run format through the basedOn chain
	// Source carries the raw binary provenance for this character style.
	Source *StyleSource
}

// Numbering captures normalized numbering references.
type Numbering struct {
	NumID           int
	Level           int
	AbstractNumID   int
	Start           int
	Format          string
	Text            string
	IndentLeft      int  // twips, from the w:lvl/w:pPr/w:ind definition
	IndentHanging   int  // twips, numbering-level hanging indent
	IndentFirstLine int  // twips, numbering-level first-line indent
	FromStyle       bool // true when numPr was inherited from a paragraph style, not the paragraph itself
	// RunFormat carries the w:lvl/w:rPr formatting for this level's list marker.
	RunFormat *RunFormat
	// LegacyOverride preserves per-list-instance LFOLVL provenance from binary
	// .doc files. Nil means the numbering reference has no explicit instance
	// override at this level.
	LegacyOverride *LegacyNumberingOverride
}

// LegacyNumberingOverride preserves list-instance overrides that cannot be
// inferred safely from the normalized level definition alone.
type LegacyNumberingOverride struct {
	Start          *int
	Format         string
	Restart        *bool
	RestartOnLevel *int
}

// NumberingLevel defines a single level within an abstract numbering definition.
type NumberingLevel struct {
	Level      int
	Start      int
	Format     string // e.g. "decimal", "upperRoman", "lowerRoman", "upperLetter", "lowerLetter", "bullet"
	Text       string // e.g. "%1.", "•"
	StyleID    string // paragraph style linked to this level (w:lvl/w:pStyle)
	IndentLeft int    // twips
	RunFormat  *RunFormat
}

// NumberingDefinition defines a numbering instance (w:num) and its abstract
// levels (w:abstractNum). This is used by the editor to create valid numbering
// parts for documents that did not originate from an existing OOXML package.
type NumberingDefinition struct {
	NumID         int
	AbstractNumID int
	Levels        []NumberingLevel
}

// ListMarker is a higher-level semantic list marker derived from numbering.
type ListMarker struct {
	Kind     string
	Level    int
	Start    int
	Format   string
	Template string
}

// RevisionMark carries tracked property-change attribution for non-run
// surfaces such as paragraph/table/section property revisions.
type RevisionMark struct {
	Author string // resolved author name when known
	Date   string // ISO-8601 date when known
	ID     int    // revision identifier when known (e.g. move ID, cell change ID)
}

// MoveRange carries the metadata for a tracked-change move-range start
// (w:moveFromRangeStart or w:moveToRangeStart).
type MoveRange struct {
	ID     int    // w:id attribute
	Author string // w:author attribute
	Date   string // w:date attribute
	Name   string // w:name attribute (move-from only)
}

// InlineMoveRange carries inline move-range metadata with position correlation.
// This is used when move-range boundaries appear within a paragraph's runs.
type InlineMoveRange struct {
	MoveRange
	// StartRunIndex is the index of the run where this move range starts.
	StartRunIndex int
	// EndRunIndex is the index of the run where this move range ends.
	// If -1, the range extends to the end of the paragraph.
	EndRunIndex int
	// StartCharOffset is the character offset within the start run where the
	// move range begins. -1 means the entire run.
	StartCharOffset int
	// EndCharOffset is the character offset within the end run where the move
	// range ends. -1 means the entire run.
	EndCharOffset int
}

// PermRange carries the metadata for a permission range start (w:permStart).
type PermRange struct {
	ID       int    // w:id attribute
	EdGrp    string // w:edGrp attribute (editor group)
	Ed       string // w:ed attribute (single editor)
	ColFirst int    // w:colFirst attribute
	ColLast  int    // w:colLast attribute
}

// Field carries a nested field structure inside a Run.  Fields are populated
// by parsers when a field's instruction or result contains further fields.
type Field struct {
	Kind      string // "IF", "REF", "PAGE", etc.
	Instr     string // raw field instruction text
	Arg       string // field argument
	Anchor    string // bookmark anchor
	Target    string // hyperlink target
	Children  []Run  // display result runs (may contain further Fields)
	InstrRuns []Run  // runs inside the field instruction (between fldBegin and fldSep)
}

// Run is a normalized inline run.
type Run struct {
	Text           string
	Kind           string
	StyleID        string // character style ID (w:rStyle / legacy sprmCIstd)
	Revision       string // "inserted" | "deleted" | "movedFrom" | "movedTo" | "" — from w:ins / w:del / w:moveFrom / w:moveTo tracked changes
	RevisionAuthor string // resolved author name for tracked changes (legacy: SttbfRMark[ibstRMark])
	RevisionDate   string // ISO-8601 date for tracked changes when known
	MoveID         int    // w:id for moveFrom/moveTo revision marks
	// PropertyRevision carries run-format change metadata from OOXML rPrChange /
	// legacy DOC sprmCPropRMark. Nil means no tracked property revision is known
	// for this run.
	PropertyRevision *RevisionMark
	// DisplayFieldRevision carries display-field revision metadata from legacy
	// DOC sprmCDispFldRMark. Nil means no display-field revision is known.
	DisplayFieldRevision *RevisionMark
	FieldKind            string // "PAGE" | "NUMPAGES" | "DATE" | "PAGEREF" | "" — field semantic type
	FieldInstr           string // raw field instruction text (e.g. `HYPERLINK "url"` or `REF bookmark \h`)
	FieldArg             string // field argument for metadata-backed fields like DOCPROPERTY
	// FieldInstrRuns preserves the structured instruction-run sequence for
	// complex fields so nested field operands can be re-evaluated later.
	FieldInstrRuns []Run
	Target         string
	Anchor         string
	RelID          string // relationship ID for hyperlinks / images (preserved for round-trip)
	BreakType      string
	BreakClear     string
	NoteID         int // footnoteRef or endnoteRef ID (Kind == "footnoteRef" or "endnoteRef")
	CommentID      int // commentRef ID (Kind == "commentRef")
	Image          *Image
	TextBox        *TextBox
	Shape          *Shape
	SignatureLine  *SignatureLine  // non-nil when this run carries an Office signature line
	Chart          *chart.Chart    // non-nil when Kind == "chart"
	MathML         string          // non-empty when Kind == "math"
	FormField      *FormField      // non-nil when Kind == "formField"
	Ruby           *RubyContent    // non-nil when Kind == "ruby"
	ContentControl *ContentControl // non-nil when this run originates from a w:sdt element
	CustomXml      *CustomXmlInfo  // non-nil when this run originates from a w:customXml element
	SmartTag       *SmartTagInfo   // non-nil when this run originates from a w:smartTag element
	OLERelID       string          // r:id of the embedded/linked OLE object (when Kind == "olePlaceholder")
	OLEProgID      string          // authored OLE ProgID
	OLEData        []byte          // embedded OLE/package payload when available
	OLEMIME        string          // embedded payload content type
	OLELinkURL     string          // external linked-object target when present
	OLEPreview     *Image          // preview image carried by w:object/v:imagedata
	OLEType        string          // authored OLE Type (Embed/Link)
	OLEDrawAspect  string
	OLEObjectID    string
	OLEUpdateMode  string
	Format         RunFormat
	// Fields carries nested field structures when this run is part of a field
	// result that contains further fields. Empty for non-nested or simple fields.
	Fields []Field
	// Source carries the raw binary provenance for this run. It is populated by
	// lossless parsers and used by projection layers.
	Source *RunSource
}

// SignatureLine represents an Office signature line (VML + o:signatureline).
type SignatureLine struct {
	// SetupID is the signature setup GUID (o:id).
	SetupID string
	// SuggestedSigner is the first line of signer info (o:suggestedsigner).
	SuggestedSigner string
	// SuggestedSigner2 is the second line (o:suggestedsigner2).
	SuggestedSigner2 string
	// SuggestedSignerEmail is the signer email (o:suggestedsigneremail).
	SuggestedSignerEmail string
	// SigningInstructions is the helper text (o:signinginstructions).
	SigningInstructions string
	// AllowComments indicates whether comments are allowed (o:allowcomments).
	AllowComments bool
	// ShowSignDate indicates whether to show the signing date (o:showsigndate).
	ShowSignDate bool
	// IsSignatureLine mirrors o:issignatureline (should be true).
	IsSignatureLine bool
	// ProviderID is the signature provider GUID (o:provid).
	ProviderID string
	// SigningInstructionsSet mirrors o:signinginstructionsset.
	SigningInstructionsSet bool
	// AdditionalXML is raw o:addlxml.
	AdditionalXML string
	// ProviderURL is the provider URL (o:sigprovurl).
	ProviderURL string
	// Image carries the rendered signature line image (v:imagedata rel).
	Image *Image
}

// FormField carries resolved form field metadata for legacy w:fldChar form fields.
// Kind=="formField"; Type is "text", "checkbox", or "dropdown".
type FormField struct {
	Type             string   // "text" | "checkbox" | "dropdown"
	Name             string   // field name from w:ffData/w:name
	Enabled          bool     // form field enabled state; true if absent
	CalcOnExit       bool     // calculate on exit
	DefaultText      string   // for "text": default/current value
	MaxLength        int      // for "text": maximum allowed character count (0 = unlimited)
	TextType         string   // for "text": input type (regular, number, date, etc.)
	TextFormat       string   // for "text": format string
	Checked          bool     // for "checkbox": checked state
	CheckboxSize     int      // for "checkbox": size in half-points
	CheckboxSizeAuto bool     // for "checkbox": auto-size
	Items            []string // for "dropdown": list option strings
	DefaultIdx       int      // for "dropdown": index of the selected item
}

// RubyContent carries the base text and phonetic annotation for a w:ruby element.
// Kind=="ruby".
type RubyContent struct {
	Base         string  // base text (w:rubyBase)
	Text         string  // phonetic annotation (w:rt)
	Align        string  // rubyAlign: center, distributeLetter, distributeSpace, left, right, rightVertical
	FontSize     float64 // phonetic guide text font size in points (hps / 2)
	BaseFontSize float64 // base text font size in points (hpsBaseText / 2)
	Raise        float64 // phonetic guide raise in points (hpsRaise / 2)
	Lang         string  // language ID (lid)
}

// TextBox holds resolved block content from a wps:txbx WordprocessingShape text box.
type TextBox struct {
	// Legacy OfficeArt property-presence flags distinguish an authored
	// zero/false value from a missing FOPT property during textbox attachment.
	HasPaddingLeft   bool
	HasPaddingTop    bool
	HasPaddingRight  bool
	HasPaddingBottom bool
	HasWrapSetting   bool
	HasAnchorSetting bool
	HasVertSetting   bool

	// LegacyShapeID binds a DOC textbox PLC story (FTXBXS.lid) to its OfficeArt FSP.spid.
	LegacyShapeID  uint32
	Blocks         []Block
	Floating       bool
	NoWrap         bool
	Vert           string
	Anchor         string  // vertical text anchor: "top", "center", "bottom"
	AnchorCenter   bool    // horizontal center anchor
	TextRotation   float64 // degrees; from wps:bodyPr rot
	Upright        bool    // text stays upright regardless of shape rotation
	X              int64
	Y              int64
	Width          int64
	Height         int64
	XRelative      string
	YRelative      string
	XAlign         string       // wp:align for positionH: "left","right","center","inside","outside"
	YAlign         string       // wp:align for positionV: "top","bottom","center","inside","outside"
	ZIndex         int64        // wp:anchor relativeHeight; 0 = use CSS default
	BehindDoc      bool         // wp:anchor behindDoc=1 — object is behind text
	WrapType       string       // "none"|"square"|"tight"|"through"|"topAndBottom"; "none" = no wrapping
	WrapSide       string       // "bothSides"|"left"|"right"|"largest"; "" = both sides
	WrapPolygon    [][2]float64 // contour polygon in object-relative points; non-nil only for tight/through
	DistTop        float64      // pt; from wp:anchor distT
	DistBottom     float64      // pt; from wp:anchor distB
	DistLeft       float64      // pt; from wp:anchor distL
	DistRight      float64      // pt; from wp:anchor distR
	SimplePosAttr  bool         // true when wp:simplePos attr is true
	SimplePosX     int64        // EMU; from wp:simplePos x when simplePos attr is true
	SimplePosY     int64        // EMU; from wp:simplePos y when simplePos attr is true
	EffectExtentL  int64        // EMU; from wp:effectExtent l
	EffectExtentT  int64        // EMU; from wp:effectExtent t
	EffectExtentR  int64        // EMU; from wp:effectExtent r
	EffectExtentB  int64        // EMU; from wp:effectExtent b
	Locked         bool         // wp:anchor locked
	LayoutInCell   bool         // wp:anchor layoutInCell
	AllowOverlap   bool         // wp:anchor allowOverlap
	SizeRelHPct    int64        // thousandths of a percent; from wp:sizeRelH pct
	SizeRelHFrom   string       // from wp:sizeRelH relativeFrom
	SizeRelVPct    int64        // thousandths of a percent; from wp:sizeRelV pct
	SizeRelVFrom   string       // from wp:sizeRelV relativeFrom
	FillColor      string       // hex RGB background fill, empty = transparent
	BorderColor    string       // hex RGB border from wps:spPr/a:ln; empty = no border
	BorderWidth    float64      // pt; from wps:spPr/a:ln width
	Effects        dml.EffectList
	PaddingLeft    float64 // pt; from wps:bodyPr lIns (OOXML default 7.2pt)
	PaddingTop     float64 // pt; from wps:bodyPr tIns (OOXML default 3.6pt)
	PaddingRight   float64 // pt; from wps:bodyPr rIns (OOXML default 7.2pt)
	PaddingBottom  float64 // pt; from wps:bodyPr bIns (OOXML default 3.6pt)
	RotationDeg    float64 // clockwise shape rotation from wps:spPr/a:xfrm
	FlipH          bool    // horizontal shape flip from wps:spPr/a:xfrm
	FlipV          bool    // vertical shape flip from wps:spPr/a:xfrm
	Autofit        string  // "none", "shape", "norm" from wps:bodyPr children
	FontScale      int     // thousandths of a percent for normAutofit
	LnSpcReduction int     // thousandths of a percent for normAutofit
	PrstTxWarp     string  // preset text warp from wps:bodyPr
	FlatTx         bool    // flat text from wps:bodyPr
	NumCol         int     // number of text columns (0 = single column)
	SpcCol         float64 // pt; space between columns
	RtlCol         bool    // true when columns flow right-to-left
}

// Image carries the resolved image data for a drawing run (Kind == "image").
type Image struct {
	MIMEType      string // e.g. "image/png"
	Data          []byte // raw image bytes
	Width         int64  // EMU (914400 = 1 inch)
	Height        int64  // EMU
	Alt           string // from docPr name/desc/title
	Title         string // from docPr title
	Hidden        bool   // from docPr hidden
	Floating      bool   // true = wp:anchor, false = wp:inline
	X             int64
	Y             int64
	XRelative     string
	YRelative     string
	XAlign        string       // wp:align for positionH: "left","right","center","inside","outside"
	YAlign        string       // wp:align for positionV: "top","bottom","center","inside","outside"
	ZIndex        int64        // wp:anchor relativeHeight; 0 = use CSS default
	BehindDoc     bool         // wp:anchor behindDoc=1 — object is behind text
	WrapType      string       // "none"|"square"|"tight"|"through"|"topAndBottom"; "none" = no wrapping
	WrapSide      string       // "bothSides"|"left"|"right"|"largest"; "" = both sides
	WrapPolygon   [][2]float64 // contour polygon in object-relative points; non-nil only for tight/through
	DistTop       float64      // pt; from wp:anchor distT
	DistBottom    float64      // pt; from wp:anchor distB
	DistLeft      float64      // pt; from wp:anchor distL
	DistRight     float64      // pt; from wp:anchor distR
	SimplePosAttr bool         // true when wp:simplePos attr is true
	SimplePosX    int64        // EMU; from wp:simplePos x when simplePos attr is true
	SimplePosY    int64        // EMU; from wp:simplePos y when simplePos attr is true
	EffectExtentL int64        // EMU; from wp:effectExtent l
	EffectExtentT int64        // EMU; from wp:effectExtent t
	EffectExtentR int64        // EMU; from wp:effectExtent r
	EffectExtentB int64        // EMU; from wp:effectExtent b
	Locked        bool         // wp:anchor locked
	LayoutInCell  bool         // wp:anchor layoutInCell
	AllowOverlap  bool         // wp:anchor allowOverlap
	SizeRelHPct   int64        // thousandths of a percent; from wp:sizeRelH pct
	SizeRelHFrom  string       // from wp:sizeRelH relativeFrom
	SizeRelVPct   int64        // thousandths of a percent; from wp:sizeRelV pct
	SizeRelVFrom  string       // from wp:sizeRelV relativeFrom
	CropLeft      float64      // visible source inset from the left edge [0,1]
	CropTop       float64      // visible source inset from the top edge [0,1]
	CropRight     float64      // visible source inset from the right edge [0,1]
	CropBottom    float64      // visible source inset from the bottom edge [0,1]
	FillColor     string       // hex RGB background fill from pic:spPr; empty = transparent
	BorderColor   string       // hex RGB border from pic:spPr/a:ln; empty = no border
	BorderWidth   float64      // pt; from pic:spPr/a:ln width
	FrameGeometry string       // pic:spPr/a:prstGeom preset; geometry of the picture frame, not the bitmap
	RotationDeg   float64      // clockwise picture rotation from pic:spPr/a:xfrm
	FlipH         bool         // horizontal picture flip from pic:spPr/a:xfrm
	FlipV         bool         // vertical picture flip from pic:spPr/a:xfrm
	// Picture adjustments from a:blip child effects (DrawingML picture adjustments).
	Brightness       float64        // CSS brightness multiplier; 1 = unchanged, 0 = unset
	Contrast         float64        // CSS contrast multiplier; 1 = unchanged, 0 = unset
	Alpha            float64        // 0..1 (1 = fully opaque), from a:alphaMod@val
	Grayscale        bool           // from a:grayscale
	BiLevel          bool           // from a:biLevel
	BiLevelThreshold int            // thousandths of a percent; from a:biLevel@thresh
	Tile             bool           // true if pic:blipFill uses a:tile
	Stretch          bool           // true if pic:blipFill uses a:stretch (default when neither tile nor stretch)
	TileOffsetX      int64          // EMU; horizontal offset of the first tile
	TileOffsetY      int64          // EMU; vertical offset of the first tile
	TileScaleX       int            // thousandths of a percent (100000 = 100%)
	TileScaleY       int            // thousandths of a percent (100000 = 100%)
	TileFlip         string         // "x", "y", "xy", or empty
	TileAlign        string         // e.g. "tl", "ctr", etc.
	Effects          dml.EffectList // from pic:spPr/a:effectLst or a:effectDag
}

// Shape carries a minimal resolved floating DrawingML shape.
type Shape struct {
	// LegacyShapeID preserves the binary OfficeArt FSP.spid used for DOC ownership.
	LegacyShapeID uint32
	// LegacyFlowAnchor marks a Word binary SHAPE field whose FSPA coordinates
	// are local to the anchor-line packing flow rather than page-absolute.
	LegacyFlowAnchor bool
	// LegacyFlowOrder is the authoritative FSPA CP. It must not be replaced by
	// OfficeArt drawing/z-order when packing consecutive SHAPE fields.
	LegacyFlowOrder int
	// OfficeArt is the canonical cross-format drawing representation.
	OfficeArt     *officeartir.Shape
	Kind          string
	Floating      bool
	X             int64
	Y             int64
	Width         int64
	Height        int64
	XRelative     string
	YRelative     string
	XAlign        string       // wp:align for positionH: "left","right","center","inside","outside"
	YAlign        string       // wp:align for positionV: "top","bottom","center","inside","outside"
	ZIndex        int64        // wp:anchor relativeHeight; 0 = use CSS default
	BehindDoc     bool         // wp:anchor behindDoc=1 — object is behind text
	WrapType      string       // "none"|"square"|"tight"|"through"|"topAndBottom"; "" = treated as square
	WrapSide      string       // "bothSides"|"left"|"right"|"largest"; "" = both sides
	WrapPolygon   [][2]float64 // contour polygon in object-relative points; non-nil only for tight/through
	DistTop       float64      // pt; from wp:anchor distT
	DistBottom    float64      // pt; from wp:anchor distB
	DistLeft      float64      // pt; from wp:anchor distL
	DistRight     float64      // pt; from wp:anchor distR
	SimplePosAttr bool         // true when wp:simplePos attr is true
	SimplePosX    int64        // EMU; from wp:simplePos x when simplePos attr is true
	SimplePosY    int64        // EMU; from wp:simplePos y when simplePos attr is true
	EffectExtentL int64        // EMU; from wp:effectExtent l
	EffectExtentT int64        // EMU; from wp:effectExtent t
	EffectExtentR int64        // EMU; from wp:effectExtent r
	EffectExtentB int64        // EMU; from wp:effectExtent b
	Locked        bool         // wp:anchor locked
	LayoutInCell  bool         // wp:anchor layoutInCell
	AllowOverlap  bool         // wp:anchor allowOverlap
	SizeRelHPct   int64        // thousandths of a percent; from wp:sizeRelH pct
	SizeRelHFrom  string       // from wp:sizeRelH relativeFrom
	SizeRelVPct   int64        // thousandths of a percent; from wp:sizeRelV pct
	SizeRelVFrom  string       // from wp:sizeRelV relativeFrom
	StrokeColor   string       // hex RGB, empty = no stroke
	StrokeWidth   int64        // EMU; 0 = default 1pt
	FillColor     string       // hex RGB, empty = no fill (transparent)
	GeometryData  *dml.Geometry
	Geometry      *dml.PresetGeometry
	Fill          dml.Fill
	LineStyle     *dml.LineStyle
	Effects       dml.EffectList
	RotationDeg   float64  // clockwise shape rotation from wps:spPr/a:xfrm
	FlipH         bool     // horizontal shape flip from wps:spPr/a:xfrm
	FlipV         bool     // vertical shape flip from wps:spPr/a:xfrm
	Children      []Shape  // nested shapes for group containers (a:grpSp)
	TextBox       *TextBox // text content for shapes with a:txBody
	// TextPath is set when a VML shape has a v:textpath child.
	TextPath *TextPath
}

// TextPath carries VML textpath data for rendering text along a shape's path.
type TextPath struct {
	On     string  // "t" / "true" / "1" or empty
	String string  // the text to render
	Font   string  // geometry-text font family when authored
	SizePt float64 // geometry-text size in points when authored

	// Legacy OfficeArt GeoText/WordArt semantics. These are also harmless for
	// ordinary VML text paths, where they remain at their zero values.
	Spacing float64 // 16.16 gtextSpacing normalized to a multiplier
	Align   uint32  // OfficeArt geometry-text alignment enum
	Warp    string  // normalized OfficeArt text-effect shape name

	ReverseRows   bool
	Vertical      bool
	Kerning       bool
	Tight         bool
	Stretch       bool
	ShrinkFit     bool
	BestFit       bool
	Normalize     bool
	DxMeasure     bool
	Bold          bool
	Italic        bool
	Underline     bool
	Shadow        bool
	SmallCaps     bool
	Strikethrough bool
}

// ParagraphFormat is the effective semantic paragraph formatting model.
type ParagraphFormat struct {
	Alignment          string
	Shading            string // background fill from w:shd ("auto" is treated as empty)
	SpacingBefore      *int
	SpacingAfter       *int
	SpacingBeforeLines *int // hundredths of a line (100 = 1 line); takes precedence over SpacingBefore when set
	SpacingAfterLines  *int // hundredths of a line; takes precedence over SpacingAfter when set
	LineSpacing        *int
	LineRule           string
	IndentLeft         *int
	IndentRight        *int
	IndentFirstLine    *int
	IndentHanging      *int
	ContextualSpacing  bool              // w:contextualSpacing — suppress space to/from same-style adjacent paragraphs
	KeepNext           bool              // w:keepNext — keep paragraph on same page as next
	KeepLines          bool              // w:keepLines — keep all lines of paragraph together
	WidowControl       bool              // w:widowControl — keep at least two lines on each side of a page break
	PageBreakBefore    bool              // w:pageBreakBefore — force page break before paragraph
	SpacingBeforeAuto  bool              // w:beforeAutospacing — treat spacing-before as 0 regardless of w:before
	SpacingAfterAuto   bool              // w:afterAutospacing — treat spacing-after as 0 regardless of w:after
	Borders            *ParagraphBorders // w:pBdr
	SnapToGrid         bool
	// Bidi marks the paragraph text direction as right-to-left (w:bidi). ECMA-376 Part 1 §17.3.1.6.
	Bidi bool
	// Kinsoku controls whether Japanese/Chinese line-breaking rules (kinsoku shori) are applied.
	// Corresponds to w:kinsoku / sprmPFKinsoku.
	Kinsoku bool
	// WordWrap controls whether Latin text wraps at mid-word at the text boundary
	// (w:wordWrap / sprmPFWordWrap). nil = not set (Word default is true);
	// *true = wrap words; *false = wrap characters.
	WordWrap *bool
	// OverflowPunct allows punctuation to extend beyond the text margin
	// (w:overflowPunct / sprmPFOverflowPunct).
	OverflowPunct bool
	// TopLinePunct compresses punctuation at the start of lines (w:topLinePunct / sprmPFTopLinePunct).
	TopLinePunct bool
	// AutoSpaceDE enables automatic spacing between East Asian and Latin text
	// (w:autoSpaceDE / sprmPFAutoSpaceDE).
	AutoSpaceDE bool
	// AutoSpaceDN enables automatic spacing between East Asian text and numbers
	// (w:autoSpaceDN / sprmPFAutoSpaceDN).
	AutoSpaceDN bool
	// UsePageSetup enables page setup settings for line grid when true
	// (sprmPFUsePgsuSettings). Legacy binary-only property.
	UsePageSetup bool
	// FontAlign is the vertical font alignment within the line
	// (w:textAlignment / sprmPWAlignFont): "auto"|"top"|"center"|"baseline"|"bottom".
	FontAlign string
	// TextDirection is the paragraph text direction: "lrTb", "tbRl", "btLr", "tbRlV".
	// Empty string means default left-to-right, top-to-bottom.
	TextDirection string
	// TabStops holds custom tab stops defined for this paragraph.
	TabStops []TabStop
	// OutlineLevel is the paragraph's heading level for TOC generation
	// (w:outlineLvl / sprmPOutLvl). 0..8 = heading 1..9; 9 = body text. nil = unset.
	OutlineLevel *int
	// Binary-format table markers (MS-DOC PAPx; never set by the DOCX parser).
	InTable                      bool // sprmPFInTable (0x2416): paragraph belongs to a table cell
	IsTableCellTrailingParagraph bool // sprmPFInnerTableCell (0x244B): paragraph mark acts as a nested-table cell end
	IsTableTrailingParagraph     bool // sprmPFTtp (0x2417): paragraph is the end-of-row mark
	// NoLineNumber suppresses line numbering for this paragraph (w:suppressLineNumbers / sprmPFNoLineNumb).
	NoLineNumber bool
	// NoAutoHyphenation suppresses auto-hyphenation for this paragraph when
	// hyphenation is enabled for the document (w:suppressAutoHyphens / sprmPFNoAutoHyph).
	NoAutoHyphenation bool
	// MirrorIndents swaps left/right indents to inside/outside on facing pages
	// (w:mirrorIndents / sprmPFMirrorIndents).
	MirrorIndents bool
	// NoAllowOverlap prevents frame overlap (w:noAllowOverlap / sprmPFNoAllowOverlap).
	NoAllowOverlap bool
	// AdjustRightInd automatically adjusts the right indent when the document
	// is in right-to-left mode (w:adjustRightInd / sprmPFAdjustRight).
	AdjustRightInd bool
	// BlockQuote indicates this paragraph is a block quotation (w:blockQuote).
	BlockQuote bool
	// Binary-format list pointers (MS-DOC PAPx; never set by the DOCX parser).
	BinaryListLevel    *int // PAP.ilvl from sprmPIlvl (0x260A)
	BinaryListOverride *int // PAP.ilfo from sprmPIlfo (0x460B)
	BinaryListFormat   *int // PAP.nfcSeqNumb (old-format, 1-byte opcode 12); nil = not set
	// BinaryStyleIndex is a transitional legacy parser staging field. Final
	// paragraphs should prefer Paragraph.Source.StyleIndex via
	// Paragraph.LegacyStyleIndex().
	BinaryStyleIndex *int // PAP.istd from PapxInFkp
	BinaryFrame      *FrameProps
	// BinaryFrame is legacy DOC staging metadata for PAPX frame SPRMs. The
	// legacy parser promotes it to Paragraph.Frame after style/direct-format
	// merging; OOXML never sets this field.
	// Source carries the raw binary provenance for this paragraph. It is
	// populated by lossless parsers (e.g. legacy .doc) and used by projection
	// layers to (re)compute effective formatting or round-trip to binary.
	Source *ParagraphSource
}

// ParagraphBorders holds resolved border sides for a paragraph (w:pBdr).
type ParagraphBorders struct {
	Top     *BorderSide
	Left    *BorderSide
	Start   *BorderSide
	Bottom  *BorderSide
	Right   *BorderSide
	End     *BorderSide
	Between *BorderSide
	Bar     *BorderSide
}

// RunBorders holds resolved border sides for a run of text (w:bdr).
// Character borders are specified by sprmCBrc (0xCA72) in the MS-DOC binary format.
type RunBorders struct {
	Top    *BorderSide
	Left   *BorderSide
	Bottom *BorderSide
	Right  *BorderSide
}

// FELayout holds East Asian text layout settings from sprmCFELayout (0xCA78).
// These control Warichu, combining characters, and vertical text layout.
type FELayout struct {
	// CombineBrackets specifies whether to use brackets around combined characters.
	// true = use brackets, false = no brackets.
	CombineBrackets bool
	// CombineCharacters enables combining of consecutive characters (w:eastAsianLayout@w:combine).
	CombineCharacters bool
	// Vertical enables vertical text layout (w:eastAsianLayout@w:vert).
	Vertical bool
	// VerticalCompressed enables compressed vertical layout (w:eastAsianLayout@w:vertCompress).
	VerticalCompressed bool
	// CombineCharCount is the number of characters to combine (w:eastAsianLayout@w:combineCharCount).
	// 0 means use default (usually 2).
	CombineCharCount int
}

// CustomXmlAttr carries a single attribute from a w:customXmlPr/w:attr element.
type CustomXmlAttr struct {
	URI   string
	Name  string
	Value string
}

// CustomXmlInfo carries the metadata from a w:customXml element.
// It is attached to every Block or Run that originates from inside a w:customXml element.
type CustomXmlInfo struct {
	URI        string
	Element    string
	Attributes []CustomXmlAttr
}

// SmartTagInfo carries the metadata from a w:smartTag element.
// It is attached to every Run that originates from inside a w:smartTag element.
type SmartTagInfo struct {
	URI     string // w:uri attribute
	Element string // w:element attribute
}

// SdtListItem represents a single item in a comboBox or dropDownList content control.
type SdtListItem struct {
	DisplayText string
	Value       string
}

// ContentControl carries the metadata from a w:sdt structured document tag.
// It is attached to every Block or Run that originates from inside a w:sdt element.
type ContentControl struct {
	ID    int    // w:id/@w:val; 0 = not set
	Tag   string // w:tag/@w:val — programmatic identifier
	Title string // w:alias/@w:val — display title
	// Type is one of "plainText", "richText", "checkBox", "comboBox", "dropDownList",
	// "date", "picture", "docPartObj", "docPartList", "citation", "equation", "group",
	// "repeatingSection", "repeatingSectionItem", "contentPart", or "" when implicit.
	Type                      string
	Checked                   bool          // checkBox: whether the box is checked
	DateFormat                string        // date: w:dateFormat/@w:val
	DateFullDate              string        // date: w:fullDate/@w:val
	DateLanguage              string        // date: w:lid/@w:val
	DateCalendar              string        // date: w:calendar/@w:val
	DocPartGallery            string        // docPartObj: w:docPartGallery/@w:val
	DocPartCategory           string        // docPartObj: w:docPartCategory/@w:val
	DocPartUnique             bool          // docPartObj: w:docPartUnique present
	ListItems                 []SdtListItem // comboBox/dropDownList: list items with display text and value
	ShowingPlaceholder        bool          // w:showingPlcHdr — placeholder text is currently shown
	PlaceholderDocPart        string        // w:placeholder/w:docPart/@w:val
	PlaceholderDocPartGallery string        // w:placeholder/w:docPartGallery/@w:val
	DataXPath                 string        // w:dataBinding/@w:xpath
	DataStoreID               string        // w:dataBinding/@w:storeItemID
	DataValue                 string        // resolved custom XML value for supported data-bound controls
	Lock                      string        // w:lock/@w:val: unlocked|sdtLocked|contentLocked|sdtContentLocked
	Temporary                 bool          // w:temporary: remove the SDT wrapper when edited in Word
	Appearance                string        // w15:appearance/@w15:val: boundingBox|tags|hidden
	Color                     string        // w15:color/@w15:val content-control UI color
	PlainTextMultiLine        bool          // w:text/@w:multiLine for plain-text controls
}

// Block preserves document-order semantic content from the body.
type Block struct {
	Index          int
	Kind           string
	Paragraph      *Paragraph
	Table          *Table
	Section        *Section
	ContentControl *ContentControl // non-nil when this block originates from a w:sdt element
	CustomXml      *CustomXmlInfo  // non-nil when this block originates from a w:customXml element
	AltChunk       *AltChunk       // non-nil when Kind == "altChunk"
	FieldKind      string          // non-empty when this block is part of a StoryField result
	FieldIndex     int             // index into the parent story's Fields slice; -1 when not part of a field
	NoteKind       string          // "footnote" or "endnote" for materialized note blocks
	NoteID         int             // source note identifier when NoteKind is set
}

// AltChunk carries raw imported external content from w:altChunk.
type AltChunk struct {
	Data        []byte // raw part bytes
	ContentType string // e.g. "text/html", "application/xhtml+xml", "text/plain"
}

// RunFormat is the effective semantic run formatting model.
// Pointer fields use nil to mean "not set at this level" so inheritance
// layers can be merged without false overrides.
type RunFormat struct {
	Bold      *bool
	Italic    *bool
	Caps      *bool
	SmallCaps *bool
	Strike    *bool
	// DoubleStrike is true when the run has double strikethrough formatting
	// (w:dstrike / sprmCFDStrike). nil = unset.
	DoubleStrike   *bool
	Underline      string
	UnderlineColor string
	Color          string
	Background     string // w:shd fill on run
	FontSize       *int   // half-points (w:sz)
	FontSizeCS     *int   // half-points (w:szCs) — complex-script font size
	BoldCS         *bool  // w:bCs — complex-script bold; nil = unset
	ItalicCS       *bool  // w:iCs — complex-script italic; nil = unset
	FontASCII      string // w:rFonts ascii — Latin font name
	FontHAnsi      string // w:rFonts hAnsi — high-ANSI font name
	FontEastAsia   string // w:rFonts eastAsia — East Asian font name
	FontCS         string // w:rFonts cs — complex-script font name
	TextFill       dml.Fill
	TextOutline    *dml.LineStyle
	TextGlow       *dml.Glow
	TextShadow     *dml.Shadow
	TextSoftEdge   *dml.SoftEdge
	TextReflection *dml.Reflection
	Ligatures      string // w14:ligatures val
	NumberSpacing  string // w14:numSpacing val
	StylisticSets  []int  // enabled w14:styleSet ids
	Highlight      string
	VertAlignVal   string // "superscript" | "subscript" | ""
	Vanish         *bool  // w:vanish — hidden text; nil = unset
	NoProof        *bool  // w:noProof / sprmCFNoProof — exclude from proofing; nil = unset
	// WebHidden specifies that text is hidden in Web Layout view (w:webHidden / sprmCFWebHidden).
	WebHidden *bool
	// ComplexScript forces complex-script formatting rules regardless of character content
	// (w:cs / sprmCFComplexScripts). nil = unset.
	ComplexScript    *bool
	Lang             string // w:lang val — primary language tag (BCP 47)
	CharacterSpacing *int   // w:spacing in twentieths of a point; nil = unset
	// FitTextWidth specifies the width (in twips) that the run text should be fit into.
	// When set, text is scaled horizontally to fit within this width (w:fitText / sprmCFitText).
	// nil = unset (no fit-text scaling).
	FitTextWidth *int
	// Kerning is the minimum font size (in half-points) above which kerning is applied (w:kern / sprmCHpsKern).
	// 0 or nil means kerning is disabled.
	Kerning *int
	// CharScale is the horizontal scaling of characters as a percentage of their normal width
	// (w:w / sprmCCharScale). 100 = normal; nil = unset.
	CharScale *int
	// UsePageSetup enables page setup settings for line grid when true
	// (sprmCFUsePgsuSettings). Legacy binary-only property.
	UsePageSetup *bool
	// RTL marks the run text direction as right-to-left complex-script (w:rtl). ECMA-376 Part 1 §17.3.2.30.
	RTL *bool
	// Dir is the inline text direction from a w:dir container ("ltr" or "rtl").
	Dir string
	// Bdo is the bidirectional override from a w:bdo container ("ltr" or "rtl").
	Bdo string
	// Text-effect toggles (w:outline / w:shadow / w:emboss / w:imprint).
	// Each is nil when unset so style inheritance can layer correctly.
	Outline *bool
	Shadow  *bool
	Emboss  *bool
	Imprint *bool
	// EmphasisMark is w:em@val: "none" | "dot" | "comma" | "circle" | "underDot" | "".
	EmphasisMark string
	// FELayout specifies East Asian text layout settings (w:textLayout / sprmCFELayout).
	// nil = unset.
	FELayout *FELayout
	// Borders specifies character-level borders for this run (w:bdr / sprmCBrc).
	// nil means no character borders are set.
	Borders *RunBorders
	// BinaryStyleIndex is a transitional legacy parser staging field. It carries
	// the raw istd value from a sprmCIstd CHP SPRM (legacy .doc only) so the
	// parser can resolve the character style before the run is emitted. Final
	// runs should rely on the resolved Format fields; this index is cleared
	// after use and should not be consulted by renderers.
	// nil = unset.
	BinaryStyleIndex *int
	// PicLocation is the fcObj value from a sprmCPicLocation CHP SPRM (legacy .doc
	// only). It stores the file offset of an embedded picture or OLE object when
	// the run is marked as a picture placeholder. 0 = unset.
	PicLocation int
	// SnapToGrid snaps the run to the document grid (w:snapToGrid). nil = unset.
	SnapToGrid *bool
	// SpecVanish makes the run invisible even when formatting marks are shown
	// (w:specVanish). Used for paragraph marks in list items. nil = unset.
	SpecVanish *bool
	// OMath marks the run as part of an Office Math region (w:oMath). nil = unset.
	OMath *bool
	// TextAnimation specifies a text animation effect (w:effect@w:val).
	// Values: "blinkBackground", "lights", "antsBlack", "antsRed",
	// "shimmer", "sparkle", "none". Empty string means unset.
	TextAnimation string
	// Position specifies the vertical position of the run relative to the
	// baseline in half-points (w:position@w:val). Positive = raised, negative = lowered.
	// nil = unset.
	Position *int
}

// ConditionalTableStyle is a pre-resolved tblStylePr entry.
// It holds the formatting overrides applied to a table cell when the cell
// satisfies the named condition (e.g. "firstRow", "lastCol", "band1H").
// The overlays are fully converted to IR-level types so that consumers of
// the IR package never need to touch the OOXML parts layer.
type ConditionalTableStyle struct {
	// Type is the OOXML condition selector, one of:
	// "firstRow", "lastRow", "firstCol", "lastCol",
	// "band1H", "band2H", "band1V", "band2V",
	// "neCell", "nwCell", "seCell", "swCell".
	Type       string
	CellFormat TableCellFormat
	RunFormat  RunFormat
	ParaFormat ParagraphFormat
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

// Section captures resolved section-level references.
type Section struct {
	Type             string
	Headers          []Reference
	Footers          []Reference
	PropertyRevision *RevisionMark // legacy/OOXML section property-change metadata when known
	PageSize         *PageSize
	PageMargin       *PageMargin
	Columns          *Columns
	DocGrid          *DocGrid
	LineNumbering    *LineNumbering
	PageBorders      *PageBorders
	// TextDirection is the section-level text direction: "tbRl", "btLr", or "" (lrTb/default).
	TextDirection string
	// PageNumberFormat is the page-number display format for this section
	// (w:pgNumType@w:fmt / sprmSNfcPgn), e.g. "decimal" or "upperRoman".
	PageNumberFormat string
	// PageNumberStart is the explicit starting page number (w:pgNumType@w:start /
	// sprmSPgnStart / sprmSPgnStart97). nil = continue numbering from the previous section.
	PageNumberStart *int
	// PageNumberRestart is true when the section restarts its page number sequence
	// (w:pgNumType present / sprmSFPgnRestart = 1).
	PageNumberRestart bool
	// TitlePage is true when the section uses a different first-page header/footer
	// (w:titlePg / sprmSFTitlePage).
	TitlePage bool
	// BreakKind is the section-break type: "continuous", "nextPage", "oddPage",
	// "evenPage", "nextColumn", or "" (default = nextPage). w:type / sprmSBkc.
	BreakKind string
	// VerticalAlign is the page vertical justification: "top", "center", "both",
	// "bottom", or "" (default = top). w:vAlign / sprmSVjc.
	VerticalAlign string
	// Bidi is true when the section uses right-to-left column layout (sprmSFBiDi / w:bidi).
	Bidi bool
	// FootnotePos is the footnote placement: "pageBottom" (default) | "belowText" | "sectEnd".
	FootnotePos string
	// FootnoteNumFmt is the numbering format for footnotes (MSONFC → list format string).
	FootnoteNumFmt string
	// FootnoteNumStart is the offset added to footnote numbers in this section (sprmSNFtn).
	FootnoteNumStart int
	// FootnoteRestart is the footnote restart mode: "continuous" | "eachSect" | "eachPage".
	FootnoteRestart string
	// EndnoteNumFmt is the numbering format for endnotes (MSONFC → list format string).
	EndnoteNumFmt string
	// EndnoteNumStart is the offset added to endnote numbers in this section (sprmSNEdn).
	EndnoteNumStart int
	// EndnoteRestart is the endnote restart mode: "continuous" | "eachSect".
	EndnoteRestart string
	// EndnoteAtSectionEnd is true when endnotes are shown at the end of this section
	// rather than deferred to the next one (sprmSFEndnote).
	EndnoteAtSectionEnd bool
	// PaperSource stores the resolved paper source trays when present.
	PaperSource *PaperSource
	// FormProt is true when the section is protected for forms (w:formProt).
	FormProt bool
	// Source carries the raw binary provenance for this section.
	Source *SectionSource
	// Diagnostics accumulates parser-level issues for this section.
	Diagnostics diag.List
}

// PageBorders holds resolved page border sides for a section (w:pgBorders / sprmSBrc*).
type PageBorders struct {
	Top    *BorderSide
	Left   *BorderSide
	Start  *BorderSide
	Bottom *BorderSide
	Right  *BorderSide
	End    *BorderSide
	// Display controls which pages show borders: "allPages" | "firstPage" | "notFirstPage".
	Display string
	// OffsetFrom controls whether distance is measured from "text" or "edge".
	OffsetFrom string
	// ZOrder controls rendering layer: "front" | "back".
	ZOrder string
}

// LineNumbering holds resolved line numbering settings from w:lnNumType.
type LineNumbering struct {
	// CountBy is the interval between displayed line numbers (e.g. 5 = every 5th line).
	CountBy int
	// Start is the starting line number (1 by default).
	Start int
	// Restart is "newPage", "newSection", or "continuous".
	Restart string
	// Distance is the gap between the line numbers and the text margin in twips
	// (w:lnNumType@w:distance / sprmSDxaLnn). 0 = use Word default.
	Distance int
}

// Reference points to a resolved story part.
type Reference struct {
	Type     string
	PartName string
}

// PageSize is a normalized section page size.
type PageSize struct {
	Width  int
	Height int
	Orient string
}

// PageMargin is a normalized section margin set.
type PageMargin struct {
	Top       int
	Right     int
	Bottom    int
	Left      int
	Header    int
	Footer    int
	Gutter    int
	RtlGutter bool // true when w:rtlGutter is present in sectPr
}

// PaperSource holds resolved paper source trays (w:paperSrc).
type PaperSource struct {
	First int
	Other int
}

// Columns is a normalized section column configuration.
type Columns struct {
	Num          int
	Space        int
	Sep          bool
	EvenlySpaced *bool // nil = unspecified; false = explicit per-column geometry
	Widths       []int // per-column widths in twips
	Spaces       []int // spacing after each column in twips (normally Num-1 entries)
}

// TabStop is a custom tab stop within a paragraph.
type TabStop struct {
	Pos    int    // position in twips
	Algn   string // "left", "center", "right", "decimal", "bar"
	Leader string // "none", "dot", "hyphen", "underscore", "middleDot"
}

// DocGrid is a normalized section line/grid configuration.
type DocGrid struct {
	LinePitch int
	CharSpace int
	Type      string
}
