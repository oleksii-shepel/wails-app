package legacy

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"office/pkg/mathtype"
	"office/pkg/msgraph"
	officeartir "office/pkg/officeart/ir"
	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/imageutil"
	presentationir "office/pkg/presentation/ir"
	presentationevent "office/pkg/presentation/parsers/event"
	"office/pkg/render/metafile"
)

const (
	pptRTDocument               = 0x03E8
	pptRTDocumentAtom           = 0x03E9
	pptRTNotes                  = 0x03F0
	pptRTDocumentTextInfo       = 0x03F2
	pptRTSlideListWithText      = 0x0FF0
	pptRTSlidePersistAtom       = 0x03F3
	pptRTUserEditAtom           = 0x0FF5
	pptRTCurrentUserAtom        = 0x0FF6
	pptRTPersistDirectory       = 0x1772
	pptRTSlide                  = 0x03EE
	pptRTSlideAtom              = 0x03EF
	pptRTSlideShowSlideInfoAtom = 0x03F9
	pptRTMainMaster             = 0x07D0 // MainMasterContainer: Document-embedded stub
	pptRTMainMasterContent      = 0x03F8 // MainMaster persist record: full master content at persist-dir offset
	pptRTExObjList              = 0x0409
	pptRTPPDrawingGroup         = 0x040B
	pptRTPPDrawing              = 0x040C
	pptRTSoundCollection        = 0x07E4
	pptRTSound                  = 0x07E6
	pptRTSoundDataBlob          = 0x07E7
	pptRTColorSchemeAtom        = 0x07F0
	pptRTRecolorInfoAtom        = 0x0FE7
	pptRTFontEntityAtom         = 0x0FB7
	pptRTTextHeaderAtom         = 0x0F9F
	pptRTTextCharsAtom          = 0x0FA0
	pptRTTextBytesAtom          = 0x0FA8
	pptRTStyleTextPropAtom      = 0x0FA1
	pptRTTextMasterStyle        = 0x0FA3
	pptRTTextMasterStyle9       = 0x0FAD
	pptRTStyleTextProp9Atom     = 0x0FAC // StyleTextProp9Atom: auto-numbering and PP9 paragraph/character exceptions
	pptRTTextDefaults9          = 0x0FB0
	pptRTTextMasterStyle10      = 0x0FB2
	pptRTTextDefaults10         = 0x0FB4
	pptRTFontCollection10       = 0x07D6
	pptRTCString                = 0x0FBA
	pptRTExObjRefAtom           = 0x0BC1
	pptRTHeadersFooters         = 0x0FD9
	pptRTHeadersFootersAtom     = 0x0FDA
	pptRTProgTags               = 0x1388
	pptRTProgBinaryTag          = 0x138A
	pptRTBinaryTagDataBlob      = 0x138B
	pptRTTextRulerAtom          = 0x0FA6 // TextRulerAtom: per-level indent/margin ruler
	pptRTExOleObjAtom           = 0x0FC3
	pptRTExOleEmbed             = 0x0FCC
	pptRTExHyperlink            = 0x0FD7
	pptRTExHyperlinkAtom        = 0x0FD3
	pptRTSlideNumberMCAtom      = 0x0FD8 // SlideNumberMCAtom: position of a slide-number metacharacter in text
	pptRTDateTimeMCAtom         = 0x0FF7 // DateTimeMCAtom
	pptRTGenericDateMCAtom      = 0x0FF8 // GenericDateMCAtom
	pptRTHeaderMCAtom           = 0x0FF9 // HeaderMCAtom
	pptRTFooterMCAtom           = 0x0FFA // FooterMCAtom
	pptRTRTFDateTimeMCAtom      = 0x1015 // RTFDateTimeMCAtom
	pptRTInteractiveInfo        = 0x0FF2
	pptRTInteractiveInfoAtom    = 0x0FF3
	pptRTAnimationInfo          = 0x1014
	pptRTAnimationInfoAtom      = 0x0FF1
	pptRTExMediaAtom            = 0x1004
	pptRTExWAVAudioEmbedded     = 0x100F
	pptRTExOleObjStg            = 0x1011
	pptRTExWAVAudioEmbeddedAtom = 0x1013
	defaultSlideWidthEMU        = 9144000
	defaultSlideHeightEMU       = 6858000

	// TextTypeEnum values from TextHeaderAtom.
	pptTextTypeTitle       = 0
	pptTextTypeBody        = 1
	pptTextTypeNotes       = 2
	pptTextTypeOther       = 4
	pptTextTypeCenterBody  = 5
	pptTextTypeCenterTitle = 6
	pptTextTypeUnknown     = -1

	legacyOfficeArtSpContainer   = 0xF004
	legacyOfficeArtSpgrContainer = 0xF003
	legacyOfficeArtBSE           = 0xF007
	legacyOfficeArtSpgr          = 0xF009
	legacyOfficeArtSp            = 0xF00A
	legacyOfficeArtFOPT          = 0xF00B
	legacyOfficeArtClientTextbox = 0xF00D
	legacyOfficeArtChildAnchor   = 0xF00F
	legacyOfficeArtClientAnchor  = 0xF010
	legacyOfficeArtClientData    = 0xF011
	legacyOfficeArtBlipEMF       = 0xF01A
	legacyOfficeArtBlipWMF       = 0xF01B
	legacyOfficeArtBlipPICT      = 0xF01C
	legacyOfficeArtBlipJPEG      = 0xF01D
	legacyOfficeArtBlipPNG       = 0xF01E
	legacyOfficeArtBlipDIB       = 0xF01F
	pptRTOutlineTextRefAtom      = 0x0F9E
	pptRTPlaceholderAtom         = 0x0BC3

	legacyOfficeArtShapePicture           = 75
	legacyOfficeArtPropPib                = 0x0104
	legacyOfficeArtPropPictureTransparent = 0x0107

	// EscherSpRecord flags (MS-ODRAW §2.2.38).
	legacyOfficeArtSpFlagChild = 0x02 // shape is a child of a group shape
	legacyOfficeArtSpFlagFlipH = 0x40 // bit 6 = fFlipH
	legacyOfficeArtSpFlagFlipV = 0x80 // bit 7 = fFlipV

	// OfficeArt fill property IDs (MS-ODRAW §2.3.7).
	legacyOfficeArtPropFillType        = 0x0180 // fillType: 0=solid
	legacyOfficeArtPropFillColor       = 0x0181
	legacyOfficeArtPropPictureRecolor  = 0x011A // pictureRecolor: OfficeArtCOLORREF // fillColor: COLORREF (R,G,B,extra)
	legacyOfficeArtPropFillBools       = 0x01BF // fillStyleBooleanProperties
	legacyOfficeArtPropTableProperties = 0x039F // groupshape.tableProperties

	// OfficeArt transform / line property IDs.
	legacyOfficeArtPropRotation             = 0x0004 // rotation, signed FixedPoint /65536 = degrees
	legacyOfficeArtPropAdjustValue          = 0x0147 // geometry.adjustValue; adjust2..adjust10 follow sequentially
	legacyOfficeArtPropLineColor            = 0x01C0 // lineColor: COLORREF
	legacyOfficeArtPropLineWidth            = 0x01CB // lineWidth: EMU (12700 = 1pt)
	legacyOfficeArtPropLineDashing          = 0x01CE // lineDashing: enum 0..n
	legacyOfficeArtPropLineStartArrowhead   = 0x01D0 // lineStartArrowhead
	legacyOfficeArtPropLineEndArrowhead     = 0x01D1 // lineEndArrowhead
	legacyOfficeArtPropLineStartArrowWidth  = 0x01D2 // inferred from adjacent MS-ODRAW arrow opids
	legacyOfficeArtPropLineStartArrowLength = 0x01D3 // lineStartArrowLength
	legacyOfficeArtPropLineEndArrowWidth    = 0x01D4 // inferred from adjacent MS-ODRAW arrow opids
	legacyOfficeArtPropLineEndArrowLength   = 0x01D5 // inferred from adjacent MS-ODRAW arrow opids
	legacyOfficeArtPropLineBools            = 0x01FF // lineStyleBooleanProperties (bit 3 = fLine, bit 19 = fLineUse)
	legacyOfficeArtFConnectorRule           = 0xF012 // OfficeArtFConnectorRule

	pptRTBackground     = 0x0410 // BackgroundContainer
	pptRTBackgroundAtom = 0x0411 // BackgroundAtom

	legacyPptMasterWidth  = 5760
	legacyPptMasterHeight = 4320

	legacyPptHFFlagDateVisible = 0x0001
	legacyPptHFFlagTodayDate   = 0x0002
	legacyPptHFFlagUserDate    = 0x0004
	legacyPptHFFlagSlideNumber = 0x0008
	legacyPptHFFlagHeader      = 0x0010
	legacyPptHFFlagFooter      = 0x0020
)

type pptRecord struct {
	Version  uint16
	Instance uint16
	Type     uint16
	Length   int
}

type legacyPptTextMetaField struct {
	Position int
	Kind     string
}

type legacySlideFragment struct {
	Text         string                      // normalized (for rendering fallback)
	RawText      string                      // decoded text preserving \r (for STPA parsing)
	TextType     int                         // pptTextType* constant; pptTextTypeUnknown when no TextHeaderAtom precedes
	ShapeID      uint32                      // explicit ESCHER shape ID when available; 0 when unknown
	OutlineRef   int                         // per-slide OutlineTextRefAtom value; -1 when unknown
	Paragraphs   []*presentationir.Paragraph // non-nil when StyleTextPropAtom was parsed
	metaFields   []legacyPptTextMetaField    // legacy text metacharacters (slide number/date/header/footer)
	pendingRuler *legacyPptTextRuler         // ruler received before STPA; applied once STPA creates paragraphs
	pendingSTP9  interface{}                 // auto-numbering from STPA9; map[int]int encoded or marker for later application
}

type legacySlideText struct {
	PersistID              int
	MasterID               int // SlideIdentifier from SlideAtom; -2147483648 (0x80000000) means use MainMaster
	MasterPersistID        int // resolved persist-directory RefID for MasterID
	LayoutType             string
	NotesPersistID         int
	HasSlideFlags          bool
	FollowMasterObjects    bool
	FollowMasterScheme     bool
	FollowMasterBackground bool
	Fragments              []legacySlideFragment
	NotesHeaderFooter      legacyPptHeaderFooterInfo
	Transition             *presentationir.SlideTransition
	BgColor                string
	BgFill                 drawing.Fill
}

type pptUserEdit struct {
	OffsetLastEdit         int
	OffsetPersistDirectory int
	DocPersistIDRef        int
}

// legacyPptMasterEntry holds the binding between a master's persist-directory
// key (RefID) and the SlideIdentifier used by slides to reference it (via
// SlideAtom.masterID).  Entries come from the master SlideListWithText
// (instance 1) SlidePersistAtom records.
type legacyPptMasterEntry struct {
	RefID           int
	SlideIdentifier int
}

type pptPersistContext struct {
	documentOffset    int
	persistDirectory  map[int]int
	slidePersistIDs   []int
	notesListInstance uint16
	notesPersistIDs   []int
	masterEntries     []legacyPptMasterEntry
}

type legacyPptPicture struct {
	Data []byte
	MIME string
}

type legacyPptSound struct {
	ID   uint32
	Name string
	Data []byte
	MIME string
}

type legacyPptExternalObject struct {
	Kind       presentationir.ShapeKind
	Name       string
	ProgID     string
	LinkURL    string
	Data       []byte
	MIME       string
	MediaKind  string
	PosterName string
}

type legacyPptPictureShape struct {
	shapeID          uint32
	officeArt        *officeartir.Shape
	blipRef          uint32
	bounds           presentationir.Rect
	transform        legacyPptShapeTransform
	animation        legacyPptAnimationInfo
	exObjID          uint32
	imageEffects     []drawing.Effect
	mathColor        string
	metafileRecolors []metafile.ColorReplacement
}

type legacyPptConnectorShape struct {
	shapeID     uint32
	officeArt   *officeartir.Shape
	shapeType   uint16
	bounds      presentationir.Rect
	adjustments []int
	transform   legacyPptShapeTransform
	animation   legacyPptAnimationInfo
}

type legacyPptConnectorRule struct {
	startShapeID uint32
	endShapeID   uint32
	connectorID  uint32
	startSite    uint32
	endSite      uint32
}

type legacyPptTableCellShape struct {
	bounds     presentationir.Rect
	paragraphs []*presentationir.Paragraph
	fillColor  string
	outlineRef int
}

type legacyPptHeaderFooterInfo struct {
	hasAtom    bool
	formatID   uint16
	flags      uint16
	dateText   string
	headerText string
	footerText string
}

type legacyPptAnimationInfo struct {
	Step    int
	Trigger string
	Effect  string
	DelayMs int
}

func (info legacyPptHeaderFooterInfo) mergedWith(override legacyPptHeaderFooterInfo) legacyPptHeaderFooterInfo {
	out := info
	if override.hasAtom || override.formatID != 0 || override.flags != 0 {
		out.hasAtom = out.hasAtom || override.hasAtom
		out.formatID = override.formatID
		out.flags = override.flags
		if override.hasAtom {
			if out.flags&legacyPptHFFlagHeader == 0 {
				out.headerText = ""
			}
			if out.flags&legacyPptHFFlagDateVisible == 0 {
				out.dateText = ""
			}
			if out.flags&legacyPptHFFlagFooter == 0 {
				out.footerText = ""
			}
		}
	}
	if override.dateText != "" {
		out.dateText = override.dateText
	}
	if override.headerText != "" {
		out.headerText = override.headerText
	}
	if override.footerText != "" {
		out.footerText = override.footerText
	}
	return out
}

func (info legacyPptHeaderFooterInfo) hasRenderableContent() bool {
	return info.headerEnabled() || info.dateEnabled() || info.footerEnabled() || info.slideNumberEnabled()
}

func (info legacyPptHeaderFooterInfo) headerEnabled() bool {
	return info.headerText != "" || info.flags&legacyPptHFFlagHeader != 0
}

func (info legacyPptHeaderFooterInfo) dateEnabled() bool {
	return info.flags&legacyPptHFFlagDateVisible != 0
}

func (info legacyPptHeaderFooterInfo) footerEnabled() bool {
	return info.footerText != "" || info.flags&legacyPptHFFlagFooter != 0
}

func (info legacyPptHeaderFooterInfo) slideNumberEnabled() bool {
	return info.flags&legacyPptHFFlagSlideNumber != 0
}

type legacyPptGroupFrame struct {
	// domain is the Escher group coordinate system used by child anchors.
	domain presentationir.Rect
	// localTarget is the group anchor in its immediate parent coordinate system.
	// target is the same anchor materialized through ancestor frames and is retained
	// only for legacy discovery paths that still need slide-space lookup geometry.
	localTarget presentationir.Rect
	target      presentationir.Rect
	valid       bool
}

func legacyPptRectToOfficeArt(rect presentationir.Rect) officeartir.Rect {
	return officeartir.Rect{X: rect.X, Y: rect.Y, W: rect.W, H: rect.H}
}

func legacyPptRectFromOfficeArt(rect officeartir.Rect) presentationir.Rect {
	return presentationir.Rect{X: rect.X, Y: rect.Y, W: rect.W, H: rect.H}
}

func legacyPptGroupFrameToOfficeArt(frame legacyPptGroupFrame) officeartlegacy.GroupFrame {
	return officeartlegacy.GroupFrame{
		Domain: legacyPptRectToOfficeArt(frame.domain),
		Target: legacyPptRectToOfficeArt(frame.target),
		Valid:  frame.valid,
	}
}

type legacyPptGroupShapeMeta struct {
	shapeID      uint32
	bounds       presentationir.Rect
	transform    legacyPptShapeTransform
	hyperlinkURL string
	actionLink   string
	animation    legacyPptAnimationInfo
}

type legacyPptTextShape struct {
	bounds         presentationir.Rect
	shapeID        uint32
	outlineRef     int
	placeholder    presentationir.PlaceholderKind
	placeholderIdx int
	textType       int
	paragraphs     []*presentationir.Paragraph
	inlineFallback []*presentationir.Paragraph
	fillColor      string             // sRGB hex from OfficeArt FOPT solid fill; "" when absent
	officeArt      *officeartir.Shape // complete authored OfficeArt visual semantics
	transform      legacyPptShapeTransform
	hyperlinkURL   string // shape-level click hyperlink URL (ACTION_HYPERLINK)
	actionLink     string // shape-level action target ("firstSlide", "nextSlide", etc.)
	animation      legacyPptAnimationInfo
}

// legacyPptShapeTransform carries the FSP/FOPT properties not encoded in the
// shape's bounds: rotation/flip from spFlags+rotation FOPT, and stroke (line)
// fields from the line FOPT properties.
type legacyPptShapeTransform struct {
	HasRotation     bool
	Rotation        float64 // degrees, clockwise
	FlipH           bool
	FlipV           bool
	LineColor       string              // sRGB hex; "" = unset
	LineWidthEMU    int64               // 12700 EMU = 1pt; 0 = unset
	LineDash        drawing.DashPattern // resolved dash pattern; "" = unset
	HasLineEnable   bool
	LineEnabled     bool
	ArrowHead       string // connector end decoration
	ArrowHeadWidth  string
	ArrowHeadLength string
	ArrowTail       string // connector start decoration
	ArrowTailWidth  string
	ArrowTailLength string

	// Text-body properties decoded from the same FOPT property tables.
	HasTextAnchor      bool
	TextAnchor         string // "t"|"ctr"|"b"
	HasAnchorCenter    bool
	AnchorCenter       bool
	HasTextWrap        bool
	TextWrap           string // "square"|"none"
	HasTextVertical    bool
	TextVertical       string // ""|"vert"|"vert270"
	HasTextInsets      bool
	HasTextInsetLeft   bool
	HasTextInsetTop    bool
	HasTextInsetRight  bool
	HasTextInsetBottom bool
	TextInsetLeft      int64
	TextInsetTop       int64
	TextInsetRight     int64
	TextInsetBottom    int64
	HasAllowOverflow   bool
	AllowOverflow      bool
	HasAutoGrow        bool
	AutoGrow           bool

	// GeometryText carries shared OfficeArt GeoText / WordArt semantics.
	HasGeometryText bool
	GeometryText    officeartlegacy.GeometryText

	// Picture crop fractions (0..1 per edge) from FOPT 0x0102–0x0105.
	HasImageCrop bool
	CropTop      float64
	CropBottom   float64
	CropLeft     float64
	CropRight    float64
}

type legacyPptMasterTextStyles map[int]map[int]legacyPptMasterTextLevel

type legacyPptMasterTextLevel struct {
	paragraph stpaPFEntry
	run       stpaCFRun
}

type legacyPptMasterTextBundle struct {
	base       legacyPptMasterTextStyles
	ext9       legacyPptMasterTextStyles
	ext10      legacyPptMasterTextStyles
	defaults9  *legacyPptMasterTextLevel
	defaults10 *legacyPptMasterTextLevel
}

type legacyPptTextRuler struct {
	defaultTabSize int
	hasDefaultTab  bool
	tabs           []presentationir.TabStop
	hasTabs        bool
	leftMargins    [5]int
	hasLeft        [5]bool
	indents        [5]int
	hasIndent      [5]bool
}

// WalkPpt emits format-neutral presentation events for a legacy binary PPT.
// This is the compatibility bridge for the event-driven migration: callers no
// longer need to depend on legacy parser internals while record-level emission
// is moved progressively into the binary decoder.
func WalkPpt(path string, consumer presentationevent.Consumer) error {
	p, err := ParsePpt(path)
	if err != nil {
		return err
	}
	return presentationevent.EmitPresentation("ppt", p, consumer)
}

// WalkPptBytes is the in-memory equivalent of WalkPpt.
func WalkPptBytes(data []byte, filename string, consumer presentationevent.Consumer) error {
	p, err := ParsePptBytes(data, filename)
	if err != nil {
		return err
	}
	return presentationevent.EmitPresentation("ppt", p, consumer)
}

func ParsePpt(path string) (*presentationir.Presentation, error) {
	f, err := cfb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("legacy ppt: open cfb: %w", err)
	}
	return parsePptFile(f, filepath.Base(path))
}

// ParsePptBytes parses a legacy binary PowerPoint presentation from memory.
// filename is used only for presentation metadata and may be empty.
func ParsePptBytes(data []byte, filename string) (*presentationir.Presentation, error) {
	f, err := cfb.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("legacy ppt: open cfb bytes: %w", err)
	}
	return parsePptFile(f, filepath.Base(filename))
}

func parsePptFile(f *cfb.File, filename string) (*presentationir.Presentation, error) {
	if pres, ok, err := parsePre97Ppt(f, filename); ok || err != nil {
		return pres, err
	}
	stream, ok := f.Stream("PowerPoint Document")
	if !ok {
		return nil, fmt.Errorf("legacy ppt: missing PowerPoint Document stream")
	}
	// Preserve the physical stream separately from the normal decrypted view.
	// Compatibility recovery must inspect the original PPT record tree.
	rawStream := stream
	decryptedStream, decryptedPics, decryptErr := maybeDecryptPptStream(f, stream)
	if decryptErr != nil {
		return nil, decryptErr
	}
	stream = decryptedStream
	var colorScheme map[uint8]string
	var fontTable map[uint16]string
	var slides []legacySlideText
	var slideShapes map[int][]*presentationir.Shape
	var slideTextRoles map[int]map[uint32]int
	var theme *presentationir.Theme
	var masterTextStyles legacyPptMasterTextStyles
	var masterStylesByID map[int]legacyPptMasterTextStyles
	var documentBgFill string
	var masterBgFillByPersistID map[int]string
	var masterPictureBgFillByPersistID map[int]drawing.Fill
	var masterColorSchemes map[int]map[uint8]string
	var slideColorSchemes map[int]map[uint8]string
	var masterComposition legacyPptMasterComposition
	var exHyperlinks map[uint32]string
	var slideHeaderFooterDefaults legacyPptHeaderFooterInfo
	var slideHeaderFooters map[int]legacyPptHeaderFooterInfo
	var notesHeaderFooterDefaults legacyPptHeaderFooterInfo
	var notesHeaderFooters map[int]legacyPptHeaderFooterInfo
	if ctx, ok := extractPptPersistContext(f, stream); ok {
		// BIG22: materialize the persist lifecycle once. Slide visual consumers now
		// receive the already-opened Slide payload from this index; masters and notes
		// are indexed at the same boundary for the next semantic ownership transfers.
		persistEvents := collectLegacyPptPersistEventIndex(stream, ctx)
		// BIG27: document-owned resources are projected from the already-opened
		// Document persist event. The valid persist path must not reopen or walk the
		// physical PowerPoint Document stream for fonts, theme presence, hyperlinks,
		// the drawing-group picture store, or the external-object list.
		fontTable = parseLegacyPptFontTable(persistEvents.document.Payload)
		colorScheme = presentationColorSchemeFromPersistEvents(persistEvents, ctx.masterEntries)
		exHyperlinks = parseLegacyPptExHyperlinkURLsFromPayload(persistEvents.document.Payload)
		theme = parseLegacyPptThemeFromPayload(persistEvents.document.Payload, colorScheme, fontTable)
		masterColorSchemes = masterColorSchemesFromPersistEvents(persistEvents, ctx.masterEntries, colorScheme)
		slideColorSchemes = slideColorSchemesFromPersistEvents(persistEvents)
		masterBgFillByPersistID = masterBackgroundsFromPersistEvents(persistEvents, ctx.masterEntries, masterColorSchemes, colorScheme)
		// Preserve non-solid master backgrounds (gradient/pattern/noFill) from the
		// same persist-owned BackgroundContainer used for the legacy color path.
		masterTextStyles = masterTextFallbackFromPersistEvents(persistEvents, ctx.masterEntries, fontTable, colorScheme)
		masterStylesByID = parseMasterStylesFromPersistEvents(persistEvents, ctx.masterEntries, parseLegacyPptDocumentTextBundle(persistEvents.document.Payload, fontTable, colorScheme), fontTable, colorScheme)
		// BIG29: materialize each master's OfficeArt stream once and share that semantic
		// scene between placeholder inheritance and inherited artwork projection.
		masterScenes := collectLegacyPptMasterVisualScenes(persistEvents, ctx.masterEntries)
		masterPictureBgFillByPersistID = make(map[int]drawing.Fill)
		for _, entry := range ctx.masterEntries {
			event, ok := persistEvents.masters[entry.RefID]
			if !ok {
				continue
			}
			scheme := masterColorSchemes[entry.RefID]
			if len(scheme) == 0 {
				scheme = colorScheme
			}
			if fill := parseLegacyPptBgDrawingFillFromContainer(event.Payload, scheme); fill != nil {
				masterPictureBgFillByPersistID[entry.RefID] = fill
			}
		}
		slideHeaderFooterDefaults = parseLegacyPptDocumentHeaderFooterInfoFromPayload(persistEvents.document.Payload, 3)
		notesHeaderFooterDefaults = parseLegacyPptDocumentHeaderFooterInfoFromPayload(persistEvents.document.Payload, 4)
		slideHeaderFooters = parsePersistHeaderFooterInfos(persistEvents.slides, ctx.slidePersistIDs)
		notesHeaderFooters = parsePersistHeaderFooterInfos(persistEvents.notes, ctx.notesPersistIDs)
		slides = extractSlideTextsFromPersistEvents(persistEvents, ctx.notesListInstance, fontTable, colorScheme, exHyperlinks)
		slides = bindLegacyPptMasterOwnership(slides, ctx.masterEntries)
		externalObjects := parseLegacyPptExternalObjectsFromEvents(persistEvents)
		var pictureStore map[uint32]*legacyPptPicture
		picturesStream := decryptedPics
		if picturesStream == nil {
			if ps, ok := f.Stream("Pictures"); ok {
				picturesStream = ps
			}
		}
		if picturesStream != nil {
			pictureStore = parseLegacyPptPictureStoreFromPayload(persistEvents.document.Payload, picturesStream)
		}

		// Background picture shapes are master-owned paint, not ordinary artwork.
		// Resolve them from the same already-decoded master scenes used by BIG37 so
		// fBackground + fillBlip survives without inventing a full-slide shape.
		for _, entry := range ctx.masterEntries {
			if scene, ok := masterScenes[entry.RefID]; ok {
				scheme := masterColorSchemes[entry.RefID]
				if len(scheme) == 0 {
					scheme = colorScheme
				}
				if fill := legacyPptBackgroundFill(scene, scheme); fill != nil {
					masterPictureBgFillByPersistID[entry.RefID] = fill
				}
				// A BLIP background needs the presentation Pictures store to materialize
				// bytes, so it overrides the metadata-only shared OfficeArt picture fill.
				if fill := legacyPptPictureBackgroundFill(scene, pictureStore); fill != nil {
					masterPictureBgFillByPersistID[entry.RefID] = fill
				}
			}
		}
		if len(masterPictureBgFillByPersistID) == 0 {
			masterPictureBgFillByPersistID = nil
		}

		// Legacy PPT can encode a slide background as an OfficeArt fBackground
		// shape whose fill is a BLIP. Such shapes deliberately have no client
		// anchor and therefore must become slide background paint rather than a
		// normal ShapeImage. Resolve them while the Pictures store is available.
		for i := range slides {
			if event, ok := persistEvents.slides[slides[i].PersistID]; ok {
				slideScheme := effectiveLegacyPptSlideColorScheme(slides[i], slides[i].PersistID, colorScheme, masterColorSchemes, slideColorSchemes)
				slides[i].BgColor = parseLegacyPptBgFillFromContainer(event.Payload, slideScheme)
				scene := collectLegacyPptVisualScene(event.Payload)
				slides[i].BgFill = parseLegacyPptBgDrawingFillFromContainer(event.Payload, slideScheme)
				// A slide whose SlideAtom follows the master can still carry an OfficeArt
				// fBackground shape containing the already-resolved fallback color. That
				// shape is not authored local paint and must not mask the master's richer
				// gradient/pattern surface. Only promote background shapes when this slide
				// explicitly opts out of master-background inheritance.
				ownsBackgroundShape := slides[i].HasSlideFlags && !slides[i].FollowMasterBackground
				if slides[i].BgFill == nil && ownsBackgroundShape {
					slides[i].BgFill = legacyPptBackgroundFill(scene, slideScheme)
				}
				if ownsBackgroundShape {
					if fill := legacyPptPictureBackgroundFill(scene, pictureStore); fill != nil {
						slides[i].BgFill = fill
					}
				}
			}
		}

		// BIG37: master artwork is a complete semantic scene, not a picture-side
		// projection. Generic shapes, authored text, groups, connectors and pictures
		// are all materialized from the same scene even when the file has no Pictures
		// stream at all. Placeholder shapes remain templates and are not painted as
		// duplicate master content.
		masterComposition = composeLegacyPptMasterSemanticScenes(
			masterScenes, ctx.masterEntries, fontTable, masterColorSchemes, colorScheme, exHyperlinks, pictureStore, externalObjects,
		)

		// BIG10: resolve each slide record exactly once at the persist boundary, then
		// project all core visual families from that event-local payload.  The old
		// pipeline independently re-opened the same Slide record for text, tables,
		// images, connectors, and groups.  Keeping one slide payload/context is the
		// first production ownership transfer to the binary event parser.
		slideShapes, slideTextRoles = parseLegacyPptSlideVisualsEventDriven(
			persistEvents, ctx.slidePersistIDs, slides, fontTable, colorScheme,
			masterColorSchemes, slideColorSchemes, masterComposition.PlaceholderTemplates, exHyperlinks, pictureStore, externalObjects,
		)
		// Authored slide membership is resolved before master artwork is composed.
		// Header/footer-only persists must be classified from their local scene;
		// inherited master artwork is not evidence that a persist owns a slide.
		slides = projectLegacySlidesInAuthoredOrder(ctx.slidePersistIDs, slides)
		slides, slideHeaderFooters = normalizeLegacyPptHeaderFooterOnlySlides(slides, slideShapes, slideHeaderFooters)

		// Master artwork is intentionally not spliced into the slide-shape map here.
		// Pass 7 moves that inheritance decision into the effective-slide composition
		// boundary together with background, text-style and header/footer inheritance.
	} else {
		// Pre-event compatibility path for damaged files without a usable persist chain.
		fontTable = parseLegacyPptFontTable(stream)
		colorScheme = parseLegacyPptColorScheme(stream)
		slides = extractSlideTextsFallback(stream, fontTable, colorScheme, exHyperlinks)
	}
	slides = attachNotesHeaderFooterInfoByPersistID(slides, notesHeaderFooters, notesHeaderFooterDefaults)
	if len(slides) == 0 {
		// Some valid presentations originally created by PowerPoint 4.0 were later
		// saved with a PPT97-style Current User/PowerPoint Document pair. Their
		// edit/persist chain can be incomplete even though the canonical Document
		// container at stream offset zero is intact. PowerPoint accepts these files.
		// Recover from that canonical container before falling back to text scraping.
		// Recover from the physical stream. Older valid files can have Current User
		// metadata that makes encryption probing ambiguous while the root
		// Document -> SlideListWithText tree is already plaintext.
		slides = extractLegacyPptSlidesFromRootDocument(rawStream, fontTable, colorScheme, exHyperlinks)
		if len(slides) == 0 && !bytes.Equal(rawStream, stream) {
			slides = extractLegacyPptSlidesFromRootDocument(stream, fontTable, colorScheme, exHyperlinks)
		}
	}
	if len(slides) == 0 {
		text := fallbackPptText(stream)
		if text == "" {
			return nil, fmt.Errorf("legacy ppt: no readable text found")
		}
		slides = buildLegacyPptTextOnlySlides(text)
	}
	// BIG37: effective master background/style selection is owned by presentation
	// composition itself. There is no post-build inheritance mutation.
	pres := buildPresentationWithEffectiveMasterComposition(slides, slideShapes, slideTextRoles, slideHeaderFooters, slideHeaderFooterDefaults, filename, theme, masterStylesByID, masterTextStyles, documentBgFill, masterBgFillByPersistID, masterPictureBgFillByPersistID, masterComposition.Visuals, masterComposition.PlaceholderTemplates)
	applyLegacyPptThemeFontFallback(pres)
	normalizeLegacyPptConnectorEndpointOwnership(pres)
	normalizeLegacyPptEffectiveParagraphDefaults(pres)
	normalizeLegacyPptEffectiveTextAutofitDefaults(pres)
	normalizeLegacyPptEmptyBulletMarkers(pres)
	normalizeLegacyPptSymbolRunStorageCodes(pres)
	normalizeLegacyPptSymbolBulletStorageCodes(pres)
	presentationir.NormalizeLegacyListMarkerGlyphs(pres)
	normalizeLegacyPptDuplicateBulletShapes(pres)
	return pres, nil
}

// normalizeLegacyPptEmptyBulletMarkers runs after every legacy text/style merge.
// Some PPT streams carry an enabled bullet on an otherwise empty paragraph used
// purely as spacing between list entries. PowerPoint does not paint that marker
// in slide-show/print output. Clearing only the marker at the final canonical
// boundary prevents master/style inheritance and grouped-shape projection from
// reintroducing an editing-only dot while preserving the blank line itself.
func normalizeLegacyPptEmptyBulletMarkers(pres *presentationir.Presentation) {
	if pres == nil {
		return
	}
	var visit func([]*presentationir.Shape)
	visit = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			suppressEmptyLegacyBulletMarkers(shape.Paragraphs)
			if len(shape.Children) > 0 {
				visit(shape.Children)
			}
		}
	}
	for _, slide := range pres.Slides {
		if slide != nil {
			visit(slide.Shapes)
		}
	}
}

// normalizeLegacyPptSymbolRunStorageCodes removes the U+F000 legacy Symbol
// storage encoding from ordinary text runs before canonical layout/rendering.
// Binary PPT commonly stores an original 8-bit Symbol character c as U+F000+c.
// These are not Unicode glyphs; allowing them to reach HTML produces black
// .notdef rectangles on systems without the exact legacy Symbol cmap.
func normalizeLegacyPptSymbolRunStorageCodes(pres *presentationir.Presentation) {
	if pres == nil {
		return
	}
	var visit func([]*presentationir.Shape)
	visit = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			for _, paragraph := range shape.Paragraphs {
				if paragraph == nil {
					continue
				}
				for _, run := range paragraph.Runs {
					if run == nil || run.Text == "" || !strings.EqualFold(strings.TrimSpace(run.FontFamily), "Symbol") {
						continue
					}
					run.Text = strings.Map(func(r rune) rune {
						if r < 0xF000 || r > 0xF0FF {
							return r
						}
						if decoded, ok := legacyPptSymbolStorageRune(byte(r - 0xF000)); ok {
							return decoded
						}
						return r
					}, run.Text)
				}
			}
			if len(shape.Children) > 0 {
				visit(shape.Children)
			}
		}
	}
	for _, slide := range pres.Slides {
		if slide != nil {
			visit(slide.Shapes)
		}
	}
}

func legacyPptSymbolStorageRune(code byte) (rune, bool) {
	if code >= 0x20 && code <= 0x7e {
		switch code {
		case 0x44:
			return '\u0394', true
		case 0x53:
			return '\u03a3', true
		case 0x61:
			return '\u03b1', true
		case 0x64:
			return '\u03b4', true
		case 0x70:
			return '\u03c0', true
		case 0x72:
			return '\u03c1', true
		case 0x74:
			return '\u03c4', true
		case 0x2d:
			return '\u2212', true
		default:
			return rune(code), true
		}
	}
	switch code {
	case 0xb1:
		return '\u00b1', true
	case 0xb7:
		return '\u2022', true
	case 0xbc:
		return '\u2026', true
	case 0xf2:
		return '\u222b', true
	default:
		return 0, false
	}
}

// normalizeLegacyPptSymbolBulletStorageCodes decodes binary-PPT bullet glyph
// storage before the format-independent IR normalizer runs. Some older PPT
// masters persist Wingdings bullets as the ASCII storage codes "w" (diamond)
// and "n" (square), while the effective bullet-font provenance has already
// collapsed to follow-text by the time the final paragraph is assembled. Those
// are not authored Latin bullet characters: they are legacy symbol-font codes.
// Decode them here, while the source format is still known, so canonical IR and
// every downstream layout/render path see stable Unicode marker semantics.
func normalizeLegacyPptSymbolBulletStorageCodes(pres *presentationir.Presentation) {
	if pres == nil {
		return
	}
	var visit func([]*presentationir.Shape)
	visit = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			for _, paragraph := range shape.Paragraphs {
				if paragraph == nil || paragraph.BulletNone || paragraph.AutoNum || paragraph.BulletChar == "" {
					continue
				}
				switch paragraph.BulletChar {
				case "w":
					paragraph.BulletChar = "◆"
				case "n":
					paragraph.BulletChar = "▪"
				default:
					continue
				}
				paragraph.BulletFontFamily = ""
				paragraph.BulletFontTx = true
			}
			if len(shape.Children) > 0 {
				visit(shape.Children)
			}
		}
	}
	for _, slide := range pres.Slides {
		if slide != nil {
			visit(slide.Shapes)
		}
	}
}

// normalizeLegacyPptDuplicateBulletShapes removes OfficeArt ellipse primitives
// that duplicate a text paragraph's canonical list marker. Some binary PPT
// animation/emphasis records materialize the marker as a tiny filled ellipse in
// the same coordinate space as the text box while the text stream still carries
// its ordinary bullet. Painting both representations produces the double-dot
// artifact seen in static HTML. The paragraph marker is the semantic owner; the
// redundant ellipse is discarded only when its size, gutter position, and color
// all match an authored bullet in a sibling text shape.
func normalizeLegacyPptDuplicateBulletShapes(pres *presentationir.Presentation) {
	presentationir.NormalizeDuplicateBulletGlyphShapes(pres)
}

// normalizeLegacyPptEffectiveTextAutofitDefaults materializes OfficeArt's
// default fitShapeToText=off behavior at the canonical slide boundary. A PPT
// text shape with no authored/inherited fit flag is a fixed-size noAutofit
// frame: glyphs may paint beyond the authored rectangle, but the shape itself
// does not grow. Keeping the default implicit made the shared renderer clip
// ordinary text boxes even though PowerPoint displays their overflow.
func normalizeLegacyPptEffectiveTextAutofitDefaults(pres *presentationir.Presentation) {
	if pres == nil {
		return
	}
	var visit func([]*presentationir.Shape)
	visit = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			if len(shape.Paragraphs) > 0 && shape.TextBodyProps.AutofitType == "" && !shape.TextAutoGrow && !shape.TextAllowOverflow {
				shape.TextBodyProps.AutofitType = "noAutofit"
				shape.TextAllowOverflow = true
			}
			visit(shape.Children)
		}
	}
	for _, slide := range pres.Slides {
		if slide != nil {
			visit(slide.Shapes)
		}
	}
}

// normalizeLegacyPptEffectiveParagraphDefaults resolves presentation defaults that
// the binary format leaves implicit. Canonical slide IR describes effective text
// semantics, so an absent alignment at the final slide boundary is left-aligned
// rather than an unresolved inheritance sentinel. Master/style records themselves
// remain sparse; this normalization only touches the materialized slide scene.
func normalizeLegacyPptEffectiveParagraphDefaults(pres *presentationir.Presentation) {
	if pres == nil {
		return
	}
	var visit func([]*presentationir.Shape)
	visit = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			for _, paragraph := range shape.Paragraphs {
				if paragraph != nil && paragraph.Align == "" {
					paragraph.Align = "l"
				}
			}
			visit(shape.Children)
		}
	}
	for _, slide := range pres.Slides {
		if slide != nil {
			visit(slide.Shapes)
		}
	}
}

// applyLegacyPptThemeFontFallback closes the final legacy text-cascade slot.
// Binary PPT runs may legitimately omit a typeface and inherit the document's
// FontEntity[0] default; unlike OOXML, those runs do not pass through the
// DrawingML text-default cascade later. Keep explicit run/master fonts intact.
func applyLegacyPptThemeFontFallback(pres *presentationir.Presentation) {
	if pres == nil || pres.Theme == nil {
		return
	}
	body := pres.Theme.MinorFont
	heading := pres.Theme.MajorFont
	if body == "" {
		body = heading
	}
	if heading == "" {
		heading = body
	}
	var visit func(*presentationir.Shape)
	visit = func(shape *presentationir.Shape) {
		if shape == nil {
			return
		}
		for _, child := range shape.Children {
			visit(child)
		}
		family := body
		if shape.Placeholder == presentationir.PlaceholderTitle {
			family = heading
		}
		if family == "" {
			return
		}
		for _, p := range shape.Paragraphs {
			if p == nil {
				continue
			}
			for _, r := range p.Runs {
				if r != nil && r.FontFamily == "" {
					r.FontFamily = family
				}
			}
		}
	}
	for _, slide := range pres.Slides {
		if slide == nil {
			continue
		}
		for _, shape := range slide.Shapes {
			visit(shape)
		}
	}
}

func parsePre97Ppt(f *cfb.File, filename string) (*presentationir.Presentation, bool, error) {
	if f == nil {
		return nil, false, nil
	}
	if stream, ok := f.Stream("PP40"); ok && len(stream) > 0 {
		stream, err := maybeDecryptPre97PptStream(stream)
		if err != nil {
			return nil, true, err
		}
		text := extractPP40Text(stream)
		if text == "" {
			return nil, true, fmt.Errorf("legacy ppt: no readable PP40 text found")
		}
		return buildPresentation(buildLegacyPptTextOnlySlides(text), filename), true, nil
	}
	textContent, ok := f.Stream("Text_Content")
	if !ok || len(textContent) == 0 {
		return nil, false, nil
	}
	if currentUser, ok := f.Stream("Current User"); ok && isPptCurrentUserAtom(currentUser) {
		return nil, false, nil
	}
	textContent, err := maybeDecryptPre97PptStream(textContent)
	if err != nil {
		return nil, true, err
	}
	text := normalizePptText(decodePptBytes(textContent))
	if text == "" {
		return nil, true, fmt.Errorf("legacy ppt: no readable Text_Content text found")
	}
	return buildPresentation(buildLegacyPptTextOnlySlides(text), filename), true, nil
}

func buildLegacyPptTextOnlySlides(text string) []legacySlideText {
	return []legacySlideText{{Fragments: []legacySlideFragment{{Text: text, TextType: pptTextTypeUnknown}}}}
}

func isPptCurrentUserAtom(currentUser []byte) bool {
	record, payload, _, ok := readPptRecord(currentUser, 0)
	return ok && record.Type == pptRTCurrentUserAtom && len(payload) >= 12
}

func extractPP40Text(stream []byte) string {
	fragments := make([]string, 0, 32)
	for pos := 0; pos+6 <= len(stream); {
		if binary.LittleEndian.Uint16(stream[pos:pos+2]) != 1 {
			pos++
			continue
		}
		payloadLen := int(binary.LittleEndian.Uint32(stream[pos+2 : pos+6]))
		if payloadLen <= 0 || payloadLen > 4096 || pos+6+payloadLen > len(stream) {
			pos++
			continue
		}
		payload := stream[pos+6 : pos+6+payloadLen]
		text, ok := decodePP40TextPayload(payload)
		if !ok {
			pos++
			continue
		}
		if len(fragments) == 0 || fragments[len(fragments)-1] != text {
			fragments = append(fragments, text)
		}
		pos += 6 + payloadLen
	}
	return normalizePptText(strings.Join(fragments, "\n"))
}

func decodePP40TextPayload(payload []byte) (string, bool) {
	printable := 0
	alphaNum := 0
	for _, ch := range payload {
		switch {
		case ch == '\r' || ch == '\n' || ch == '\t':
			printable++
		case ch >= 32 && ch < 127:
			printable++
			if unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) {
				alphaNum++
			}
		default:
			return "", false
		}
	}
	text := normalizePptText(decodePptBytes(payload))
	if text == "" || printable != len(payload) || alphaNum == 0 {
		return "", false
	}
	return text, true
}

func parseLegacyPptThemeFromPayload(payload []byte, colorScheme map[uint8]string, fontTable map[uint16]string) *presentationir.Theme {
	if len(colorScheme) == 0 || len(payload) == 0 {
		return nil
	}
	hasMaster := false
	for pos := 0; pos+8 <= len(payload); {
		child, _, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTMainMaster {
			hasMaster = true
			break
		}
		pos = next
	}
	if !hasMaster {
		return nil
	}
	theme := &presentationir.Theme{ColorScheme: legacyPptThemeColorScheme(colorScheme)}
	// Legacy PPT does not split fonts into major/minor like the OOXML font scheme;
	// use the first FontEntityAtom name as the default for both. Callers that need
	// finer attribution can override per-shape via Run.FontFamily.
	if first := legacyPptFirstFontName(fontTable); first != "" {
		theme.MajorFont = first
		theme.MinorFont = first
	}
	return theme
}

// legacyPptFirstFontName returns the FontEntityAtom name at index 0, which the
// 97-2003 binary format uses as the document's default text font; falls back
// to the lowest-numbered entry when index 0 is absent.
func legacyPptFirstFontName(fontTable map[uint16]string) string {
	if len(fontTable) == 0 {
		return ""
	}
	if name, ok := fontTable[0]; ok && name != "" {
		return name
	}
	bestKey := uint16(0xFFFF)
	bestName := ""
	for k, name := range fontTable {
		if name == "" {
			continue
		}
		if bestName == "" || k < bestKey {
			bestKey = k
			bestName = name
		}
	}
	return bestName
}

func parseLegacyPptExHyperlinkURLsFromPayload(payload []byte) map[uint32]string {
	urls := map[uint32]string{}
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTExObjList {
			for id, url := range parseLegacyPptExHyperlinksFromList(childPayload) {
				urls[id] = url
			}
		}
		pos = next
	}
	if len(urls) == 0 {
		return nil
	}
	return urls
}

func parseLegacyPptExHyperlinksFromList(payload []byte) map[uint32]string {
	urls := map[uint32]string{}
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTExHyperlink {
			if id, url, ok := parseLegacyPptExHyperlinkRecord(childPayload); ok {
				urls[id] = url
			}
		}
		pos = next
	}
	if len(urls) == 0 {
		return nil
	}
	return urls
}

func parseLegacyPptExHyperlinkRecord(payload []byte) (uint32, string, bool) {
	var exHyperlinkID uint32
	hasID := false
	url := ""
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTExHyperlinkAtom:
			if len(childPayload) >= 4 {
				exHyperlinkID = binary.LittleEndian.Uint32(childPayload[:4])
				hasID = true
			}
		case pptRTCString:
			if candidate, ok := normalizeLegacyPptHyperlinkURL(decodePptUTF16(childPayload)); ok && url == "" {
				url = candidate
			}
		}
		pos = next
	}
	if !hasID || url == "" {
		return 0, "", false
	}
	return exHyperlinkID, url, true
}

// parseLegacyPptExternalObjectsFromEvents resolves document-owned external-object
// metadata and persisted OLE storage entirely from the persist event index. The
// valid PPT97 path therefore never seeks back into the PowerPoint Document stream.
func parseLegacyPptExternalObjectsFromEvents(events legacyPptPersistEventIndex) map[uint32]legacyPptExternalObject {
	payload := events.document.Payload
	sounds := parseLegacyPptSoundCollection(payload)
	out := map[uint32]legacyPptExternalObject{}
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTExObjList {
			for id, ext := range parseLegacyPptExternalObjectsFromEventList(childPayload, events.resources, sounds) {
				out[id] = ext
			}
		}
		pos = next
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptExternalObjectsFromEventList(payload []byte, resources map[int]PersistEvent, sounds map[uint32]legacyPptSound) map[uint32]legacyPptExternalObject {
	out := map[uint32]legacyPptExternalObject{}
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTExOleEmbed:
			if obj, ok := parseLegacyPptOleEmbedContainerFromEvents(childPayload, resources); ok {
				out[obj.exObjID] = obj.external
			}
		case pptRTExWAVAudioEmbedded:
			if exObjID, ext, ok := parseLegacyPptEmbeddedWAVContainer(childPayload, sounds); ok {
				out[exObjID] = ext
			}
		default:
			if record.Version == 0x0F {
				if obj, ok := parseLegacyPptOleEmbedContainerFromEvents(childPayload, resources); ok {
					out[obj.exObjID] = obj.external
				} else if exObjID, ext, ok := parseLegacyPptGenericMediaContainer(childPayload); ok {
					out[exObjID] = ext
				}
			}
		}
		pos = next
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptOleEmbedContainerFromEvents(payload []byte, resources map[int]PersistEvent) (legacyPptExternalObjectParse, bool) {
	var out legacyPptExternalObjectParse
	var persistID uint32
	var objectType uint32
	var menuName, progID, clipboardName string
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTExOleObjAtom:
			if len(childPayload) >= 20 {
				objectType = binary.LittleEndian.Uint32(childPayload[4:8])
				out.exObjID = binary.LittleEndian.Uint32(childPayload[8:12])
				persistID = binary.LittleEndian.Uint32(childPayload[16:20])
			}
		case pptRTCString:
			text := normalizePptText(decodePptUTF16(childPayload))
			switch record.Instance {
			case 1:
				menuName = text
			case 2:
				progID = text
			case 3:
				clipboardName = text
			}
		}
		pos = next
	}
	if out.exObjID == 0 {
		return legacyPptExternalObjectParse{}, false
	}
	data := parseLegacyPptOleStorageFromEvents(resources, int(persistID))
	out.external = legacyPptExternalObject{Kind: presentationir.ShapeOleObject, Name: firstLegacyPptNonEmpty(clipboardName, menuName, progID, fmt.Sprintf("Embedded Object %d", out.exObjID)), ProgID: progID, Data: data, MIME: "application/x-ole-storage"}
	if objectType == 1 {
		out.external.LinkURL = menuName
		out.external.Data = nil
	}
	return out, true
}

// maxLegacyPptDecompressedBytes bounds the payload an OLE storage record may
// inflate to. A malformed or hostile container can declare a tiny compressed
// payload that expands without limit; the parser must stop rather than grow the
// buffer until the process stalls.
const maxLegacyPptDecompressedBytes = 256 << 20

// inflateLegacyPptPayload decompresses an OLE storage payload.
func inflateLegacyPptPayload(compressed []byte) []byte {
	return inflateLegacyPptPayloadLimit(compressed, maxLegacyPptDecompressedBytes)
}

// inflateLegacyPptPayloadLimit decompresses an OLE storage payload, stopping
// when the stream ends, when the output reaches limit bytes, or when the reader
// stops making progress. A truncated or hostile container therefore cannot spin
// the parser forever or grow the buffer without bound.
func inflateLegacyPptPayloadLimit(compressed []byte, limit int) []byte {
	if len(compressed) == 0 {
		return nil
	}
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil
	}
	defer r.Close()
	var out bytes.Buffer
	buf := make([]byte, 4096)
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			if remaining := limit - out.Len(); n > remaining {
				if remaining > 0 {
					out.Write(buf[:remaining])
				}
				break
			}
			out.Write(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if out.Len() > 0 {
				break
			}
			return nil
		}
		if n == 0 {
			// No progress: a decompressor that returns (0, nil) forever would
			// otherwise keep this loop running indefinitely.
			break
		}
	}
	return out.Bytes()
}

func parseLegacyPptOleStorageFromEvents(resources map[int]PersistEvent, persistID int) []byte {
	e, ok := resources[persistID]
	if !ok || e.Record.Type != pptRTExOleObjStg {
		return nil
	}
	payload := e.Payload
	if e.Record.Instance == 1 {
		if len(payload) <= 4 {
			return nil
		}
		return inflateLegacyPptPayload(payload[4:])
	}
	return append([]byte(nil), payload...)
}

func parseLegacyPptExternalObjectsFromPayload(payload []byte, stream []byte, persistDirectory map[int]int) map[uint32]legacyPptExternalObject {
	sounds := parseLegacyPptSoundCollection(payload)
	out := map[uint32]legacyPptExternalObject{}
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTExObjList {
			for id, ext := range parseLegacyPptExternalObjectsFromList(childPayload, stream, persistDirectory, sounds) {
				out[id] = ext
			}
		}
		pos = next
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptSoundCollection(documentPayload []byte) map[uint32]legacyPptSound {
	sounds := map[uint32]legacyPptSound{}
	for pos := 0; pos+8 <= len(documentPayload); {
		record, payload, next, ok := readPptRecord(documentPayload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTSoundCollection {
			for sID, sound := range parseLegacyPptSoundsFromCollection(payload) {
				sounds[sID] = sound
			}
		}
		pos = next
	}
	if len(sounds) == 0 {
		return nil
	}
	return sounds
}

func parseLegacyPptSoundsFromCollection(payload []byte) map[uint32]legacyPptSound {
	out := map[uint32]legacyPptSound{}
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTSound {
			if sound, ok := parseLegacyPptSoundContainer(childPayload); ok {
				out[sound.ID] = sound
			}
		}
		pos = next
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptSoundContainer(payload []byte) (legacyPptSound, bool) {
	var sound legacyPptSound
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTCString:
			text := normalizePptText(decodePptUTF16(childPayload))
			switch record.Instance {
			case 0:
				if sound.Name == "" {
					sound.Name = text
				}
			case 2:
				if id, ok := parseLegacyPptDecimalID(text); ok {
					sound.ID = id
				}
			}
		case pptRTSoundDataBlob:
			sound.Data = append([]byte(nil), childPayload...)
		}
		pos = next
	}
	if sound.ID == 0 || len(sound.Data) == 0 {
		return legacyPptSound{}, false
	}
	sound.MIME = legacyPptMediaMIME(sound.Data, sound.Name)
	return sound, true
}

func parseLegacyPptDecimalID(text string) (uint32, bool) {
	if text == "" {
		return 0, false
	}
	var id uint32
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, false
		}
		id = id*10 + uint32(r-'0')
	}
	return id, id != 0
}

func parseLegacyPptExternalObjectsFromList(payload []byte, stream []byte, persistDirectory map[int]int, sounds map[uint32]legacyPptSound) map[uint32]legacyPptExternalObject {
	out := map[uint32]legacyPptExternalObject{}
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTExOleEmbed:
			if obj, ok := parseLegacyPptOleEmbedContainer(childPayload, stream, persistDirectory); ok {
				out[obj.exObjID] = obj.external
			}
		case pptRTExWAVAudioEmbedded:
			if exObjID, ext, ok := parseLegacyPptEmbeddedWAVContainer(childPayload, sounds); ok {
				out[exObjID] = ext
			}
		default:
			// A few producer/version-specific ExObjList containers wrap the same
			// ExOleObjAtom contract under a different container type. Prefer that
			// structural contract before trying generic media recognition.
			if record.Version == 0x0F {
				if obj, ok := parseLegacyPptOleEmbedContainer(childPayload, stream, persistDirectory); ok {
					out[obj.exObjID] = obj.external
				} else if exObjID, ext, ok := parseLegacyPptGenericMediaContainer(childPayload); ok {
					out[exObjID] = ext
				}
			}
		}
		pos = next
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type legacyPptExternalObjectParse struct {
	exObjID  uint32
	external legacyPptExternalObject
}

func parseLegacyPptOleEmbedContainer(payload []byte, stream []byte, persistDirectory map[int]int) (legacyPptExternalObjectParse, bool) {
	var out legacyPptExternalObjectParse
	var persistID uint32
	var objectType uint32
	var menuName, progID, clipboardName string
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTExOleObjAtom:
			if len(childPayload) >= 20 {
				objectType = binary.LittleEndian.Uint32(childPayload[4:8])
				out.exObjID = binary.LittleEndian.Uint32(childPayload[8:12])
				persistID = binary.LittleEndian.Uint32(childPayload[16:20])
			}
		case pptRTCString:
			text := normalizePptText(decodePptUTF16(childPayload))
			switch record.Instance {
			case 1:
				menuName = text
			case 2:
				progID = text
			case 3:
				clipboardName = text
			}
		}
		pos = next
	}
	if out.exObjID == 0 {
		return legacyPptExternalObjectParse{}, false
	}
	data := parseLegacyPptOleStorage(stream, persistDirectory, int(persistID))
	out.external = legacyPptExternalObject{
		Kind:   presentationir.ShapeOleObject,
		Name:   firstLegacyPptNonEmpty(clipboardName, menuName, progID, fmt.Sprintf("Embedded Object %d", out.exObjID)),
		ProgID: progID,
		Data:   data,
		MIME:   "application/x-ole-storage",
	}
	if objectType == 1 {
		out.external.LinkURL = menuName
		out.external.Data = nil
	}
	return out, true
}

func parseLegacyPptOleStorage(stream []byte, persistDirectory map[int]int, persistID int) []byte {
	if persistID == 0 {
		return nil
	}
	offset, ok := persistDirectory[persistID]
	if !ok {
		return nil
	}
	record, payload, _, ok := readPptRecord(stream, offset)
	if !ok || record.Type != pptRTExOleObjStg {
		return nil
	}
	if record.Instance == 1 {
		if len(payload) <= 4 {
			return nil
		}
		return inflateLegacyPptPayload(payload[4:])
	}
	return append([]byte(nil), payload...)
}

func parseLegacyPptEmbeddedWAVContainer(payload []byte, sounds map[uint32]legacyPptSound) (uint32, legacyPptExternalObject, bool) {
	var exObjID uint32
	var soundID uint32
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTExMediaAtom:
			if len(childPayload) >= 4 {
				exObjID = binary.LittleEndian.Uint32(childPayload[:4])
			}
		case pptRTExWAVAudioEmbeddedAtom:
			if len(childPayload) >= 4 {
				soundID = binary.LittleEndian.Uint32(childPayload[:4])
			}
		}
		pos = next
	}
	sound, ok := sounds[soundID]
	if exObjID == 0 || !ok || len(sound.Data) == 0 {
		return 0, legacyPptExternalObject{}, false
	}
	return exObjID, legacyPptExternalObject{
		Kind:      presentationir.ShapeMedia,
		Name:      firstLegacyPptNonEmpty(sound.Name, fmt.Sprintf("Embedded Audio %d", exObjID)),
		Data:      append([]byte(nil), sound.Data...),
		MIME:      sound.MIME,
		MediaKind: "audio",
	}, true
}

func parseLegacyPptGenericMediaContainer(payload []byte) (uint32, legacyPptExternalObject, bool) {
	var exObjID uint32
	var stringsSeen []string
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		switch record.Type {
		case pptRTExMediaAtom:
			if exObjID == 0 && len(childPayload) >= 4 {
				exObjID = binary.LittleEndian.Uint32(childPayload[:4])
			}
		case pptRTCString:
			if text := strings.TrimSpace(normalizePptText(decodePptUTF16(childPayload))); text != "" {
				stringsSeen = append(stringsSeen, text)
			}
		}
	})
	if exObjID == 0 {
		return 0, legacyPptExternalObject{}, false
	}

	name := ""
	link := ""
	for _, text := range stringsSeen {
		if name == "" {
			name = text
		}
		if link == "" && legacyPptLooksLikeMediaTarget(text) {
			link = text
		}
	}
	kind, mime := legacyPptMediaKindAndMIME(nil, firstLegacyPptNonEmpty(link, name))
	if kind == "" {
		return 0, legacyPptExternalObject{}, false
	}
	return exObjID, legacyPptExternalObject{
		Kind:      presentationir.ShapeMedia,
		Name:      firstLegacyPptNonEmpty(name, filepath.Base(link), fmt.Sprintf("Linked Media %d", exObjID)),
		LinkURL:   link,
		MIME:      mime,
		MediaKind: kind,
	}, true
}

func legacyPptLooksLikeMediaTarget(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "://") || strings.HasPrefix(lower, "file:") || strings.ContainsAny(trimmed, "\\/") {
		kind, _ := legacyPptMediaKindAndMIME(nil, trimmed)
		return kind != ""
	}
	kind, _ := legacyPptMediaKindAndMIME(nil, trimmed)
	return kind != ""
}

func legacyPptMediaKindAndMIME(data []byte, name string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(name))
	ext := strings.ToLower(filepath.Ext(strings.SplitN(lower, "?", 2)[0]))
	switch ext {
	case ".avi":
		return "video", "video/x-msvideo"
	case ".mpg", ".mpeg", ".mpe":
		return "video", "video/mpeg"
	case ".wmv":
		return "video", "video/x-ms-wmv"
	case ".mp4", ".m4v":
		return "video", "video/mp4"
	case ".mov", ".qt":
		return "video", "video/quicktime"
	case ".wav":
		return "audio", "audio/wav"
	case ".mp3":
		return "audio", "audio/mpeg"
	case ".mid", ".midi":
		return "audio", "audio/midi"
	case ".wma":
		return "audio", "audio/x-ms-wma"
	case ".aif", ".aiff":
		return "audio", "audio/aiff"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" {
		if string(data[8:12]) == "WAVE" {
			return "audio", "audio/wav"
		}
		if string(data[8:12]) == "AVI " {
			return "video", "video/x-msvideo"
		}
	}
	return "", "application/octet-stream"
}

func firstLegacyPptNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func legacyPptMediaMIME(data []byte, name string) string {
	_, mime := legacyPptMediaKindAndMIME(data, name)
	return mime
}

func normalizeLegacyPptHyperlinkURL(raw string) (string, bool) {
	url := strings.TrimSpace(raw)
	if url == "" {
		return "", false
	}
	lower := strings.ToLower(url)
	if strings.Contains(lower, "schemas.openxmlformats.org") || strings.Contains(lower, "www.apple.com/dtds/propertylist") {
		return "", false
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
		return url, true
	}
	return "", false
}

func legacyPptThemeColorScheme(colorScheme map[uint8]string) map[string]string {
	if len(colorScheme) == 0 {
		return nil
	}
	mapped := map[string]string{}
	if value, ok := colorScheme[0]; ok && value != "" {
		mapped["lt1"] = value
	}
	if value, ok := colorScheme[1]; ok && value != "" {
		mapped["dk1"] = value
	}
	if value, ok := colorScheme[2]; ok && value != "" {
		mapped["dk2"] = value
	}
	if value, ok := colorScheme[3]; ok && value != "" {
		mapped["lt2"] = value
	}
	if value, ok := colorScheme[4]; ok && value != "" {
		mapped["accent1"] = value
	}
	if value, ok := colorScheme[5]; ok && value != "" {
		mapped["accent2"] = value
	}
	if value, ok := colorScheme[6]; ok && value != "" {
		mapped["accent3"] = value
	}
	if value, ok := colorScheme[7]; ok && value != "" {
		mapped["accent4"] = value
	}
	if len(mapped) == 0 {
		return nil
	}
	return mapped
}

func parseLegacyPptMasterTextStyles(stream []byte, documentOffset int, fontTable map[uint16]string, colorScheme map[uint8]string) legacyPptMasterTextStyles {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return nil
	}
	var docBundle legacyPptMasterTextBundle
	var mainMasterPayload []byte
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch child.Type {
		case pptRTDocumentTextInfo:
			docBundle.base = mergeLegacyPptMasterStyleMaps(docBundle.base, parseLegacyPptBaseMasterTextStyles(childPayload, fontTable, colorScheme))
		case pptRTMainMaster:
			if len(mainMasterPayload) == 0 {
				mainMasterPayload = append([]byte(nil), childPayload...)
			}
		default:
			docBundle.merge(parseLegacyPptProgTagsBundle(childPayload, colorScheme))
		}
		pos = next
	}
	styles := legacyPptMasterTextStyles{}
	if len(mainMasterPayload) > 0 {
		mainBundle := parseLegacyPptMainMasterTextBundle(mainMasterPayload, fontTable, colorScheme)
		styles = composeLegacyPptMasterTextStyles(docBundle, mainBundle)
	}
	walkPptRecords(stream, func(record pptRecord, payload []byte) {
		if record.Type != pptRTTextMasterStyle {
			return
		}
		textType, levels, ok := parseLegacyPptTextMasterStyle(record, payload, fontTable, colorScheme)
		if !ok || len(levels) == 0 {
			return
		}
		if existing, ok := styles[textType]; ok && len(existing) >= len(levels) {
			return
		}
		styles[textType] = levels
	})
	if len(mainMasterPayload) > 0 {
		for textType, levels := range parseLegacyPptDirectMasterTextStyles(mainMasterPayload, fontTable, colorScheme) {
			if len(levels) == 0 {
				continue
			}
			if styles == nil {
				styles = legacyPptMasterTextStyles{}
			}
			existing := styles[textType]
			if len(existing) == 0 {
				existing = map[int]legacyPptMasterTextLevel{}
				styles[textType] = existing
			}
			for level, entry := range levels {
				if _, ok := existing[level]; ok {
					continue
				}
				if docBundle.defaults10 != nil {
					entry = mergeLegacyPptMasterTextLevel(entry, *docBundle.defaults10)
				}
				if docBundle.defaults9 != nil {
					entry = mergeLegacyPptMasterTextLevel(entry, *docBundle.defaults9)
				}
				if legacyPptMasterTextLevelEmpty(entry) {
					continue
				}
				existing[level] = entry
			}
		}
	}
	if len(styles) > 0 {
		return styles
	}
	if len(mainMasterPayload) > 0 {
		return parseLegacyPptDirectMasterTextStyles(mainMasterPayload, fontTable, colorScheme)
	}
	return nil
}

// parseLegacyPptMasterStylesByID returns per-master text styles keyed by
// SlideIdentifier (the value that SlideAtom.masterID carries).  For each
// legacyPptMasterEntry the function resolves the master's stream offset via
// persistDirectory and parses the 0x03F8 MainMaster content record there,
// composing styles against the Document-level defaults (docBundle).
// This is the POI HSLFSlide.getMasterSheet() / HSLFSlideMaster analogue: the
// caller can look up slide.MasterID to get the exact master's text styles
// rather than the first-found master from a stream walk.
func parseLegacyPptDocumentTextBundle(payload []byte, fontTable map[uint16]string, colorScheme map[uint8]string) legacyPptMasterTextBundle {
	var docBundle legacyPptMasterTextBundle
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch child.Type {
		case pptRTDocumentTextInfo:
			docBundle.base = mergeLegacyPptMasterStyleMaps(docBundle.base, parseLegacyPptBaseMasterTextStyles(childPayload, fontTable, colorScheme))
		default:
			docBundle.merge(parseLegacyPptProgTagsBundle(childPayload, colorScheme))
		}
		pos = next
	}
	return docBundle
}

func parseLegacyPptMainMasterTextBundle(payload []byte, fontTable map[uint16]string, colorScheme map[uint8]string) legacyPptMasterTextBundle {
	var bundle legacyPptMasterTextBundle
	bundle.base = parseLegacyPptBaseMasterTextStyles(payload, fontTable, colorScheme)
	bundle.merge(parseLegacyPptProgTagsBundle(payload, colorScheme))
	return bundle
}

func parseLegacyPptBaseMasterTextStyles(payload []byte, fontTable map[uint16]string, colorScheme map[uint8]string) legacyPptMasterTextStyles {
	styles := legacyPptMasterTextStyles{}
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		if record.Type != pptRTTextMasterStyle {
			return
		}
		textType, levels, ok := parseLegacyPptTextMasterStyle(record, childPayload, fontTable, colorScheme)
		if !ok || len(levels) == 0 {
			return
		}
		styles = mergeLegacyPptMasterStyleMaps(styles, legacyPptMasterTextStyles{textType: levels})
	})
	if len(styles) == 0 {
		return nil
	}
	return styles
}

func parseLegacyPptTextMasterStyle9s(payload []byte) legacyPptMasterTextStyles {
	styles := legacyPptMasterTextStyles{}
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		if record.Type != pptRTTextMasterStyle9 {
			return
		}
		textType, levels, ok := parseLegacyPptTextMasterStyle9(record, childPayload)
		if !ok || len(levels) == 0 {
			return
		}
		styles = mergeLegacyPptMasterStyleMaps(styles, legacyPptMasterTextStyles{textType: levels})
	})
	if len(styles) == 0 {
		return nil
	}
	return styles
}

func parseLegacyPptProgTagsBundle(payload []byte, colorScheme map[uint8]string) legacyPptMasterTextBundle {
	var bundle legacyPptMasterTextBundle
	parseProgBinaryTags := func(data []byte) {
		for tagPos := 0; tagPos+8 <= len(data); {
			tagRecord, tagPayload, tagNext, ok := readPptRecord(data, tagPos)
			if !ok {
				break
			}
			if tagRecord.Type == pptRTProgBinaryTag {
				name, blob, ok := parseLegacyPptProgBinaryTag(tagPayload)
				if ok {
					switch name {
					case "___PPT9":
						bundle.merge(parseLegacyPptPP9TextBundle(blob, colorScheme))
					case "___PPT10":
						bundle.merge(parseLegacyPptPP10TextBundle(blob))
					}
				}
			}
			tagPos = tagNext
		}
	}
	foundProgTags := false
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTProgTags {
			foundProgTags = true
			parseProgBinaryTags(childPayload)
		}
		pos = next
	}
	if !foundProgTags {
		parseProgBinaryTags(payload)
	}
	return bundle
}

func parseLegacyPptProgBinaryTag(payload []byte) (string, []byte, bool) {
	var tagName string
	var blob []byte
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTCString:
			if tagName == "" {
				tagName = decodePptUTF16(childPayload)
			}
		case pptRTBinaryTagDataBlob:
			if len(blob) == 0 {
				blob = append([]byte(nil), childPayload...)
			}
		}
		pos = next
	}
	if tagName == "" || len(blob) == 0 {
		return "", nil, false
	}
	return tagName, blob, true
}

func parseLegacyPptPP9TextBundle(payload []byte, colorScheme map[uint8]string) legacyPptMasterTextBundle {
	bundle := legacyPptMasterTextBundle{ext9: parseLegacyPptTextMasterStyle9s(payload)}
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTTextDefaults9 {
			if level, ok := parseLegacyPptTextDefaults9(childPayload); ok {
				bundle.defaults9 = &level
			}
		}
		pos = next
	}
	return bundle
}

func parseLegacyPptPP10TextBundle(payload []byte) legacyPptMasterTextBundle {
	fontTable := map[uint16]string(nil)
	var bundle legacyPptMasterTextBundle
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		if record.Type == pptRTFontCollection10 {
			fontTable = parseLegacyPptFontCollection(childPayload)
		}
	})
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		switch record.Type {
		case pptRTTextMasterStyle10:
			textType, levels, ok := parseLegacyPptTextMasterStyle10(record, childPayload, fontTable)
			if ok && len(levels) > 0 {
				bundle.ext10 = mergeLegacyPptMasterStyleMaps(bundle.ext10, legacyPptMasterTextStyles{textType: levels})
			}
		case pptRTTextDefaults10:
			if level, ok := parseLegacyPptTextDefaults10(childPayload, fontTable); ok {
				bundle.defaults10 = &level
			}
		}
	})
	return bundle
}

func parseLegacyPptDirectMasterTextStyles(payload []byte, fontTable map[uint16]string, colorScheme map[uint8]string) legacyPptMasterTextStyles {
	frags, _ := parseLegacyPptTextBoxFragmentsRecursive(payload, fontTable, colorScheme, nil)
	if len(frags) == 0 {
		return nil
	}
	styles := legacyPptMasterTextStyles{}
	for _, frag := range frags {
		if !legacyPptDirectMasterFragmentUsable(frag) {
			continue
		}
		paragraphs := fragmentParagraphs(frag)
		if len(paragraphs) == 0 {
			continue
		}
		textType := frag.TextType
		if textType == pptTextTypeUnknown && frag.OutlineRef >= 0 {
			textType = frag.OutlineRef
		}
		if textType == pptTextTypeUnknown {
			continue
		}
		levels := styles[textType]
		if levels == nil {
			levels = map[int]legacyPptMasterTextLevel{}
			styles[textType] = levels
		}
		for _, paragraph := range paragraphs {
			if paragraph == nil {
				continue
			}
			level := legacyPptParagraphLevel(paragraph)
			if _, exists := levels[level]; exists {
				continue
			}
			levels[level] = legacyPptMasterTextLevel{
				paragraph: legacyPptMasterParagraphFromIR(paragraph),
				run:       legacyPptMasterRunFromIR(paragraph),
			}
		}
	}
	if len(styles) == 0 {
		return nil
	}
	return styles
}

func legacyPptDirectMasterFragmentUsable(frag legacySlideFragment) bool {
	text := strings.TrimSpace(frag.Text)
	if text == "" || text == "*" {
		return false
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func legacyPptMasterParagraphFromIR(paragraph *presentationir.Paragraph) stpaPFEntry {
	entry := stpaPFEntry{
		Align:          paragraph.Align,
		TextDirection:  paragraph.TextDirection,
		AutoNum:        paragraph.AutoNum,
		AutoNumType:    paragraph.AutoNumType,
		AutoNumStart:   paragraph.AutoNumStart,
		BulletColor:    paragraph.BulletColor,
		HasBulletColor: paragraph.BulletColorExplicit,
		BulletSize:     paragraph.BulletSize,
		SpaceBeforePct: paragraph.SpaceBeforePct,
		SpaceAfterPct:  paragraph.SpaceAfterPct,
		LineSpacingPct: paragraph.LineSpacingPct,
		MarginLeftEMU:  paragraph.MarginLeftEMU,
		HasMarginLeft:  paragraph.HasMarginLeft,
		TextIndentEMU:  paragraph.TextIndentEMU,
		HasTextIndent:  paragraph.HasTextIndent,
	}
	if paragraph.BulletChar != "" {
		entry.HasBullet = true
		entry.BulletChar = []rune(paragraph.BulletChar)[0]
	}
	if paragraph.HasSpaceBefore {
		entry.SpaceBefore = paragraph.SpaceBefore
		entry.HasSpaceBefore = true
	}
	if paragraph.HasSpaceAfter {
		entry.SpaceAfter = paragraph.SpaceAfter
		entry.HasSpaceAfter = true
	}
	if paragraph.HasLineSpacing {
		entry.HasLineSpacing = true
	}
	return entry
}

func legacyPptMasterRunFromIR(paragraph *presentationir.Paragraph) stpaCFRun {
	run := stpaCFRun{}
	for _, candidate := range paragraph.Runs {
		if candidate == nil {
			continue
		}
		if candidate.Bold {
			value := true
			run.Bold = &value
		}
		if candidate.Italic {
			value := true
			run.Italic = &value
		}
		if candidate.Underline {
			value := true
			run.Underline = &value
		}
		if candidate.Strike {
			value := true
			run.Strike = &value
		}
		if run.FontSize == 0 {
			run.FontSize = candidate.FontSize
		}
		if run.FontFamily == "" {
			run.FontFamily = candidate.FontFamily
		}
		if run.Color == "" {
			run.Color = candidate.Color
		}
		if run.Bold != nil || run.Italic != nil || run.Underline != nil || run.Strike != nil || run.FontSize != 0 || run.FontFamily != "" || run.Color != "" {
			break
		}
	}
	return run
}

func parseLegacyPptTextMasterStyle(record pptRecord, payload []byte, fontTable map[uint16]string, colorScheme map[uint8]string) (int, map[int]legacyPptMasterTextLevel, bool) {
	if len(payload) < 2 {
		return 0, nil, false
	}
	textType := int(record.Instance)
	cLevels := int(binary.LittleEndian.Uint16(payload[:2]))
	if cLevels <= 0 {
		return textType, nil, false
	}
	if cLevels > 5 {
		cLevels = 5
	}
	levels := map[int]legacyPptMasterTextLevel{}
	pos := 2
	for levelIndex := 0; levelIndex < cLevels && pos < len(payload); levelIndex++ {
		level := levelIndex
		if textType >= pptTextTypeCenterBody {
			if pos+2 > len(payload) {
				break
			}
			level = int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
			pos += 2
		}
		pf, newPos, ok := parsePFExceptionSpecWithTheme(payload, pos, colorScheme, fontTable)
		if !ok || newPos > len(payload) {
			break
		}
		cf, newPos, ok := parseCFExceptionSpecWithTheme(payload, newPos, fontTable, colorScheme)
		if !ok || newPos > len(payload) {
			break
		}
		levels[level] = legacyPptMasterTextLevel{paragraph: pf, run: cf}
		pos = newPos
	}
	if len(levels) == 0 {
		return textType, nil, false
	}
	return textType, levels, true
}

func parseLegacyPptTextMasterStyle9(record pptRecord, payload []byte) (int, map[int]legacyPptMasterTextLevel, bool) {
	if len(payload) < 2 {
		return 0, nil, false
	}
	textType := int(record.Instance)
	cLevels := int(binary.LittleEndian.Uint16(payload[:2]))
	if cLevels <= 0 {
		return textType, nil, false
	}
	if cLevels > 5 {
		cLevels = 5
	}
	levels := map[int]legacyPptMasterTextLevel{}
	pos := 2
	for levelIndex := 0; levelIndex < cLevels && pos < len(payload); levelIndex++ {
		pf, newPos, ok := parseLegacyPptPFException9(payload, pos)
		if !ok || newPos > len(payload) {
			break
		}
		cf, newPos, ok := parseLegacyPptCFException9(payload, newPos)
		if !ok || newPos > len(payload) {
			break
		}
		levels[levelIndex] = legacyPptMasterTextLevel{paragraph: pf, run: cf}
		pos = newPos
	}
	if len(levels) == 0 {
		return textType, nil, false
	}
	return textType, levels, true
}

func parseLegacyPptTextMasterStyle10(record pptRecord, payload []byte, fontTable map[uint16]string) (int, map[int]legacyPptMasterTextLevel, bool) {
	if len(payload) < 2 {
		return 0, nil, false
	}
	textType := int(record.Instance)
	cLevels := int(binary.LittleEndian.Uint16(payload[:2]))
	if cLevels <= 0 {
		return textType, nil, false
	}
	if cLevels > 5 {
		cLevels = 5
	}
	levels := map[int]legacyPptMasterTextLevel{}
	pos := 2
	for levelIndex := 0; levelIndex < cLevels && pos < len(payload); levelIndex++ {
		level := levelIndex
		if textType >= pptTextTypeCenterBody {
			if pos+2 > len(payload) {
				break
			}
			level = int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
			pos += 2
		}
		cf, newPos, ok := parseLegacyPptCFException10(payload, pos, fontTable)
		if !ok || newPos > len(payload) {
			break
		}
		levels[level] = legacyPptMasterTextLevel{run: cf}
		pos = newPos
	}
	if len(levels) == 0 {
		return textType, nil, false
	}
	return textType, levels, true
}

func parseLegacyPptTextDefaults9(payload []byte) (legacyPptMasterTextLevel, bool) {
	cf, pos, ok := parseLegacyPptCFException9(payload, 0)
	if !ok {
		return legacyPptMasterTextLevel{}, false
	}
	pf, pos, ok := parseLegacyPptPFException9(payload, pos)
	if !ok || pos > len(payload) {
		return legacyPptMasterTextLevel{}, false
	}
	return legacyPptMasterTextLevel{paragraph: pf, run: cf}, true
}

func parseLegacyPptTextDefaults10(payload []byte, fontTable map[uint16]string) (legacyPptMasterTextLevel, bool) {
	cf, pos, ok := parseLegacyPptCFException10(payload, 0, fontTable)
	if !ok || pos > len(payload) {
		return legacyPptMasterTextLevel{}, false
	}
	return legacyPptMasterTextLevel{run: cf}, true
}

func parseLegacyPptPFException9(data []byte, pos int) (stpaPFEntry, int, bool) {
	if pos+4 > len(data) {
		return stpaPFEntry{}, pos, false
	}
	masks := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	var entry stpaPFEntry
	if masks&(1<<20) != 0 {
		if pos+2 > len(data) {
			return entry, pos, false
		}
		pos += 2
	}
	autoNumEnabled := false
	if masks&(1<<22) != 0 {
		if pos+2 > len(data) {
			return entry, pos, false
		}
		autoNumEnabled = int16(binary.LittleEndian.Uint16(data[pos:pos+2])) != 0
		pos += 2
	}
	if masks&(1<<21) != 0 {
		if pos+2 > len(data) {
			return entry, pos, false
		}
		scheme := binary.LittleEndian.Uint16(data[pos : pos+2])
		pos += 2
		if autoNumEnabled {
			entry.AutoNum = true
			entry.AutoNumType = legacyPptAutoNumType(scheme)
			entry.AutoNumStart = 1
		}
	}
	return entry, pos, true
}

func parseLegacyPptCFException9(data []byte, pos int) (stpaCFRun, int, bool) {
	if pos+4 > len(data) {
		return stpaCFRun{}, pos, false
	}
	masks := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	if masks&(1<<13) != 0 {
		if pos+4 > len(data) {
			return stpaCFRun{}, pos, false
		}
		pos += 4
	}
	return stpaCFRun{}, pos, true
}

func parseLegacyPptCFException10(data []byte, pos int, fontTable map[uint16]string) (stpaCFRun, int, bool) {
	return parseCFExceptionSpecWithTheme(data, pos, fontTable, nil)
}

func parseLegacyPptFontCollection(payload []byte) map[uint16]string {
	fontTable := map[uint16]string{}
	index := uint16(0)
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		if record.Type != pptRTFontEntityAtom {
			return
		}
		name := parseLegacyPptFontEntityName(childPayload)
		if name == "" {
			return
		}
		fontTable[index] = name
		index++
	})
	if len(fontTable) == 0 {
		return nil
	}
	return fontTable
}

func composeLegacyPptMasterTextStyles(docBundle, mainBundle legacyPptMasterTextBundle) legacyPptMasterTextStyles {
	textTypes := map[int]struct{}{}
	collectLegacyPptMasterTextTypes(textTypes, docBundle.base)
	collectLegacyPptMasterTextTypes(textTypes, docBundle.ext9)
	collectLegacyPptMasterTextTypes(textTypes, docBundle.ext10)
	collectLegacyPptMasterTextTypes(textTypes, mainBundle.base)
	collectLegacyPptMasterTextTypes(textTypes, mainBundle.ext9)
	collectLegacyPptMasterTextTypes(textTypes, mainBundle.ext10)
	out := legacyPptMasterTextStyles{}
	for textType := range textTypes {
		levels := collectLegacyPptMasterTextLevels(textType, docBundle, mainBundle)
		if len(levels) == 0 {
			continue
		}
		mergedLevels := map[int]legacyPptMasterTextLevel{}
		for level := range levels {
			merged := legacyPptMasterTextLevel{}
			merged = mergeLegacyPptMasterTextLevel(merged, levelFromLegacyPptMasterTextStyles(mainBundle.ext10, textType, level))
			merged = mergeLegacyPptMasterTextLevel(merged, levelFromLegacyPptMasterTextStyles(docBundle.ext10, textType, level))
			merged = mergeLegacyPptMasterTextLevel(merged, levelFromLegacyPptMasterTextStyles(mainBundle.ext9, textType, level))
			merged = mergeLegacyPptMasterTextLevel(merged, levelFromLegacyPptMasterTextStyles(docBundle.ext9, textType, level))
			merged = mergeLegacyPptMasterTextLevel(merged, levelFromLegacyPptMasterTextStyles(mainBundle.base, textType, level))
			merged = mergeLegacyPptMasterTextLevel(merged, levelFromLegacyPptMasterTextStyles(docBundle.base, textType, level))
			if mainBundle.defaults10 != nil {
				merged = mergeLegacyPptMasterTextLevel(merged, *mainBundle.defaults10)
			}
			if docBundle.defaults10 != nil {
				merged = mergeLegacyPptMasterTextLevel(merged, *docBundle.defaults10)
			}
			if mainBundle.defaults9 != nil {
				merged = mergeLegacyPptMasterTextLevel(merged, *mainBundle.defaults9)
			}
			if docBundle.defaults9 != nil {
				merged = mergeLegacyPptMasterTextLevel(merged, *docBundle.defaults9)
			}
			if !legacyPptMasterTextLevelEmpty(merged) {
				mergedLevels[level] = merged
			}
		}
		if len(mergedLevels) > 0 {
			out[textType] = mergedLevels
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectLegacyPptMasterTextTypes(dst map[int]struct{}, styles legacyPptMasterTextStyles) {
	for textType := range styles {
		dst[textType] = struct{}{}
	}
}

func collectLegacyPptMasterTextLevels(textType int, docBundle, mainBundle legacyPptMasterTextBundle) map[int]struct{} {
	levels := map[int]struct{}{}
	for _, styles := range []legacyPptMasterTextStyles{docBundle.base, docBundle.ext9, docBundle.ext10, mainBundle.base, mainBundle.ext9, mainBundle.ext10} {
		for level := range styles[textType] {
			levels[level] = struct{}{}
		}
	}
	return levels
}

func levelFromLegacyPptMasterTextStyles(styles legacyPptMasterTextStyles, textType, level int) legacyPptMasterTextLevel {
	if len(styles) == 0 {
		return legacyPptMasterTextLevel{}
	}
	if levels, ok := styles[textType]; ok {
		if entry, ok := levels[level]; ok {
			return entry
		}
		if level != 0 {
			if entry, ok := levels[0]; ok {
				return entry
			}
		}
	}
	return legacyPptMasterTextLevel{}
}

func cloneLegacyPptMasterTextStyles(styles legacyPptMasterTextStyles) legacyPptMasterTextStyles {
	if len(styles) == 0 {
		return nil
	}
	cloned := make(legacyPptMasterTextStyles, len(styles))
	for textType, levels := range styles {
		if len(levels) == 0 {
			continue
		}
		clonedLevels := make(map[int]legacyPptMasterTextLevel, len(levels))
		for level, entry := range levels {
			clonedLevels[level] = entry
		}
		cloned[textType] = clonedLevels
	}
	return cloned
}

func mergeLegacyPptMasterStyleMaps(primary, fallback legacyPptMasterTextStyles) legacyPptMasterTextStyles {
	if len(fallback) == 0 {
		return primary
	}
	if primary == nil {
		primary = legacyPptMasterTextStyles{}
	}
	for textType, levels := range fallback {
		if len(levels) == 0 {
			continue
		}
		if primary[textType] == nil {
			primary[textType] = map[int]legacyPptMasterTextLevel{}
		}
		for level, entry := range levels {
			primary[textType][level] = mergeLegacyPptMasterTextLevel(primary[textType][level], entry)
		}
	}
	if len(primary) == 0 {
		return nil
	}
	return primary
}

func mergeLegacyPptMasterTextLevel(primary, fallback legacyPptMasterTextLevel) legacyPptMasterTextLevel {
	primary.paragraph = mergeLegacyPptMasterParagraphStyle(primary.paragraph, fallback.paragraph)
	primary.run = mergeLegacyPptMasterRunStyle(primary.run, fallback.run)
	return primary
}

func legacyPptMasterParagraphHasImplicitBulletGlyph(paragraph stpaPFEntry) bool {
	return !paragraph.HasBulletSpecified && !paragraph.HasBullet && paragraph.BulletChar != 0 &&
		(paragraph.HasMarginLeft || paragraph.HasTextIndent)
}

func mergeLegacyPptMasterParagraphStyle(primary, fallback stpaPFEntry) stpaPFEntry {
	if primary.Align == "" {
		primary.Align = fallback.Align
	}
	if primary.TextDirection == "" {
		primary.TextDirection = fallback.TextDirection
	}
	if !primary.HasBulletSpecified && !primary.HasBullet && primary.BulletChar == 0 && !primary.AutoNum {
		// A master level can carry a cached bulletChar even when bullets are not
		// enabled. Only inherit the character when the fallback actually enables
		// bullets; otherwise the cached glyph must not manufacture a bullet.
		if fallback.HasBulletSpecified {
			primary.HasBulletSpecified = true
			primary.HasBullet = fallback.HasBullet
		}
		if fallback.HasBullet {
			primary.HasBullet = true
			primary.BulletChar = fallback.BulletChar
		} else if legacyPptMasterParagraphHasImplicitBulletGlyph(fallback) {
			primary.BulletChar = fallback.BulletChar
		}
		if fallback.AutoNum {
			primary.AutoNum = true
			primary.AutoNumType = fallback.AutoNumType
			primary.AutoNumStart = fallback.AutoNumStart
		}
	}
	if primary.BulletColor == "" {
		primary.BulletColor = fallback.BulletColor
	}
	if primary.BulletSize == 0 {
		primary.BulletSize = fallback.BulletSize
	}
	if !primary.HasSpaceBefore {
		primary.SpaceBefore = fallback.SpaceBefore
		primary.HasSpaceBefore = fallback.HasSpaceBefore
		if primary.SpaceBeforePct == 0 {
			primary.SpaceBeforePct = fallback.SpaceBeforePct
		}
	}
	if !primary.HasSpaceAfter {
		primary.SpaceAfter = fallback.SpaceAfter
		primary.HasSpaceAfter = fallback.HasSpaceAfter
		if primary.SpaceAfterPct == 0 {
			primary.SpaceAfterPct = fallback.SpaceAfterPct
		}
	}
	if !primary.HasLineSpacing {
		primary.LineSpacingPct = fallback.LineSpacingPct
		primary.HasLineSpacing = fallback.HasLineSpacing
	}
	if !primary.HasMarginLeft {
		primary.MarginLeftEMU = fallback.MarginLeftEMU
		primary.HasMarginLeft = fallback.HasMarginLeft
	}
	if !primary.HasTextIndent {
		primary.TextIndentEMU = fallback.TextIndentEMU
		primary.HasTextIndent = fallback.HasTextIndent
	}
	if !primary.HasBulletSpecified && !primary.AutoNum && primary.BulletChar == 0 && fallback.AutoNum {
		primary.AutoNum = true
		primary.AutoNumType = fallback.AutoNumType
		primary.AutoNumStart = fallback.AutoNumStart
	}
	return primary
}

func mergeLegacyPptMasterRunStyle(primary, fallback stpaCFRun) stpaCFRun {
	if primary.Bold == nil {
		primary.Bold = fallback.Bold
	}
	if primary.Italic == nil {
		primary.Italic = fallback.Italic
	}
	if primary.Underline == nil {
		primary.Underline = fallback.Underline
	}
	if primary.Strike == nil {
		primary.Strike = fallback.Strike
	}
	if primary.FontSize == 0 {
		primary.FontSize = fallback.FontSize
	}
	if primary.FontFamily == "" {
		primary.FontFamily = fallback.FontFamily
	}
	if primary.Color == "" {
		primary.Color = fallback.Color
	}
	return primary
}

// legacyPptMasterTextLevelSemanticallyValid is a format-disambiguation gate,
// not a renderer clamp.  A master level decoded as a control-character bullet,
// a >400pt font, or the characteristic all-decoration collision cannot be a
// valid PowerPoint text master.  Treating such a level as inheritance poisons
// every otherwise-correct slide paragraph (the 720pt-per-character failure is
// the canonical symptom), so it must not participate in the cascade.
func legacyPptMasterTextLevelSemanticallyValid(level legacyPptMasterTextLevel) bool {
	if level.paragraph.BulletChar != 0 && level.paragraph.BulletChar < 0x20 && level.paragraph.BulletChar != '\t' {
		return false
	}
	if level.run.FontSize > 40000 {
		return false
	}
	if level.run.Bold != nil && *level.run.Bold &&
		level.run.Italic != nil && *level.run.Italic &&
		level.run.Underline != nil && *level.run.Underline &&
		level.run.Strike != nil && *level.run.Strike {
		return false
	}
	return true
}

func legacyPptMasterTextLevelEmpty(level legacyPptMasterTextLevel) bool {
	paragraph := level.paragraph
	run := level.run
	return paragraph.Align == "" && paragraph.TextDirection == "" && !paragraph.HasBulletSpecified && !paragraph.HasBullet && paragraph.BulletChar == 0 && !paragraph.AutoNum && paragraph.BulletColor == "" && paragraph.BulletSize == 0 && !paragraph.HasSpaceBefore && paragraph.SpaceBeforePct == 0 && !paragraph.HasSpaceAfter && paragraph.SpaceAfterPct == 0 && !paragraph.HasLineSpacing && !paragraph.HasMarginLeft && !paragraph.HasTextIndent && run.Bold == nil && run.Italic == nil && run.Underline == nil && run.Strike == nil && run.FontSize == 0 && run.FontFamily == "" && run.Color == ""
}

func (bundle *legacyPptMasterTextBundle) merge(other legacyPptMasterTextBundle) {
	bundle.base = mergeLegacyPptMasterStyleMaps(bundle.base, other.base)
	bundle.ext9 = mergeLegacyPptMasterStyleMaps(bundle.ext9, other.ext9)
	bundle.ext10 = mergeLegacyPptMasterStyleMaps(bundle.ext10, other.ext10)
	if other.defaults9 != nil {
		if bundle.defaults9 == nil {
			level := *other.defaults9
			bundle.defaults9 = &level
		} else {
			merged := mergeLegacyPptMasterTextLevel(*bundle.defaults9, *other.defaults9)
			bundle.defaults9 = &merged
		}
	}
	if other.defaults10 != nil {
		if bundle.defaults10 == nil {
			level := *other.defaults10
			bundle.defaults10 = &level
		} else {
			merged := mergeLegacyPptMasterTextLevel(*bundle.defaults10, *other.defaults10)
			bundle.defaults10 = &merged
		}
	}
}

func legacyPptAutoNumType(scheme uint16) string {
	switch scheme {
	case 0x0000:
		return "alphaLcPeriod"
	case 0x0001:
		return "alphaUcPeriod"
	case 0x0002:
		return "arabicParenR"
	case 0x0003:
		return "arabicPeriod"
	case 0x0006:
		return "romanLcPeriod"
	case 0x0007:
		return "romanUcPeriod"
	default:
		return ""
	}
}

func parseLegacyPptColorScheme(stream []byte) map[uint8]string {
	type paletteEntry struct {
		key     string
		palette map[uint8]string
		count   int
	}

	palettes := map[string]*paletteEntry{}
	walkPptRecords(stream, func(record pptRecord, payload []byte) {
		if record.Type != pptRTColorSchemeAtom {
			return
		}
		palette, key := parseLegacyPptColorSchemePayload(payload)
		if len(palette) == 0 || key == "" {
			return
		}
		entry, ok := palettes[key]
		if !ok {
			palettes[key] = &paletteEntry{key: key, palette: palette, count: 1}
			return
		}
		entry.count++
	})
	if len(palettes) == 0 {
		return nil
	}
	var best *paletteEntry
	for _, entry := range palettes {
		if best == nil || entry.count > best.count {
			best = entry
		}
	}
	if best == nil {
		return nil
	}
	out := make(map[uint8]string, len(best.palette))
	for index, color := range best.palette {
		out[index] = color
	}
	return out
}

func parseLegacyPptColorSchemePayload(payload []byte) (map[uint8]string, string) {
	if len(payload) < 32 {
		return nil, ""
	}
	palette := make(map[uint8]string, 8)
	var key strings.Builder
	for index := 0; index < 8; index++ {
		offset := index * 4
		color := fmt.Sprintf("%02X%02X%02X", payload[offset], payload[offset+1], payload[offset+2])
		palette[uint8(index)] = color
		key.WriteString(color)
		key.WriteByte('|')
	}
	return palette, key.String()
}

func parseLegacyPptFontTable(stream []byte) map[uint16]string {
	type fontEntry struct {
		instance uint16
		name     string
	}

	var currentRun []fontEntry
	var bestRun []fontEntry
	finalizeRun := func() {
		if len(currentRun) > len(bestRun) {
			bestRun = append([]fontEntry(nil), currentRun...)
		}
		currentRun = currentRun[:0]
	}

	walkPptRecords(stream, func(record pptRecord, payload []byte) {
		if record.Type != pptRTFontEntityAtom {
			if len(currentRun) > 0 {
				finalizeRun()
			}
			return
		}
		name := parseLegacyPptFontEntityName(payload)
		if name == "" {
			if len(currentRun) > 0 {
				finalizeRun()
			}
			return
		}
		currentRun = append(currentRun, fontEntry{instance: uint16(record.Instance), name: name})
	})
	if len(currentRun) > 0 {
		finalizeRun()
	}
	if len(bestRun) == 0 {
		return nil
	}

	fontTable := make(map[uint16]string, len(bestRun))
	seenInstances := make(map[uint16]struct{}, len(bestRun))
	useInstances := true
	for _, entry := range bestRun {
		if _, exists := seenInstances[entry.instance]; exists {
			useInstances = false
			break
		}
		seenInstances[entry.instance] = struct{}{}
	}
	if useInstances {
		for _, entry := range bestRun {
			fontTable[entry.instance] = entry.name
		}
		return fontTable
	}
	for index, entry := range bestRun {
		fontTable[uint16(index)] = entry.name
	}
	return fontTable
}

func parseLegacyPptFontEntityName(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	limit := len(payload)
	if limit > 64 {
		limit = 64
	}
	for index := 0; index+1 < limit; index += 2 {
		if binary.LittleEndian.Uint16(payload[index:index+2]) != 0 {
			continue
		}
		return decodePptUTF16(payload[:index])
	}
	return decodePptUTF16(payload[:limit])
}

func extractPptPersistContext(f *cfb.File, stream []byte) (pptPersistContext, bool) {
	if f == nil {
		return pptPersistContext{}, false
	}
	currentUser, ok := f.Stream("Current User")
	if !ok {
		return pptPersistContext{}, false
	}
	currentEditOffset, ok := parseCurrentUserOffset(currentUser)
	if !ok {
		return pptPersistContext{}, false
	}
	userEdits, ok := collectUserEdits(stream, currentEditOffset)
	if !ok || len(userEdits) == 0 {
		return pptPersistContext{}, false
	}
	persistDirectory := buildPersistDirectory(stream, userEdits)
	if len(persistDirectory) == 0 {
		return pptPersistContext{}, false
	}
	documentOffset, ok := persistDirectory[userEdits[0].DocPersistIDRef]
	if !ok {
		return pptPersistContext{}, false
	}
	notesListInstance := selectLegacyPptNotesSlideListInstance(stream, documentOffset, persistDirectory)
	return pptPersistContext{
		documentOffset:    documentOffset,
		persistDirectory:  persistDirectory,
		slidePersistIDs:   collectPersistIDsFromDocument(stream, documentOffset, 0),
		notesListInstance: notesListInstance,
		notesPersistIDs:   collectPersistIDsFromDocument(stream, documentOffset, notesListInstance),
		masterEntries:     collectMasterEntriesFromDocument(stream, documentOffset),
	}, true
}

func extractSlideTextsFallback(stream []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	var combined []legacySlideText
	var notesList []legacySlideText
	walkPptRecords(stream, func(record pptRecord, payload []byte) {
		if record.Type != pptRTSlideListWithText {
			return
		}
		switch record.Instance {
		case 0: // Regular slides
			candidate := parseSlideListWithText(payload, fontTable, colorScheme, exHyperlinks)
			if len(candidate) == 0 {
				return
			}
			combined = mergeLegacySlides(combined, candidate)
		case 1: // Master slides - explicitly skip; never treat as notes or regular slides
			// Instance 1 is for master slides only; do not include in fallback slide list
		case 2: // Notes
			notesList = append(notesList, parseSlideListWithText(payload, fontTable, colorScheme, exHyperlinks)...)
		}
	})
	if len(notesList) > 0 {
		combined = attachNotesByIndex(combined, notesList)
	}
	return combined
}

func parseCurrentUserOffset(currentUser []byte) (int, bool) {
	record, payload, _, ok := readPptRecord(currentUser, 0)
	if !ok || record.Type != pptRTCurrentUserAtom || len(payload) < 12 {
		return 0, false
	}
	return int(binary.LittleEndian.Uint32(payload[8:12])), true
}

func collectUserEdits(stream []byte, startOffset int) ([]pptUserEdit, bool) {
	seen := map[int]struct{}{}
	minOffset := startOffset
	var edits []pptUserEdit
	for offset := startOffset; offset != 0; {
		if _, ok := seen[offset]; ok {
			// Cycle detected in the UserEditAtom back-link chain.  Attempt the
			// same heuristic repair that HSLFSlideShowImpl.initRecordOffsets
			// applies: the original (oldest) UserEditAtom typically sits 36 bytes
			// before the smallest known UserEditAtom offset (one full atom length
			// for the common 28-byte-payload variant).
			candidate := minOffset - 36
			if candidate >= 0 {
				if _, alreadySeen := seen[candidate]; !alreadySeen {
					r, p, _, ok2 := readPptRecord(stream, candidate)
					if ok2 && r.Version == 0 && r.Instance == 0 &&
						r.Type == pptRTUserEditAtom &&
						(r.Length == 0x1C || r.Length == 0x20) &&
						len(p) >= 20 {
						edits = append(edits, pptUserEdit{
							OffsetLastEdit:         int(binary.LittleEndian.Uint32(p[8:12])),
							OffsetPersistDirectory: int(binary.LittleEndian.Uint32(p[12:16])),
							DocPersistIDRef:        int(binary.LittleEndian.Uint32(p[16:20])),
						})
					}
				}
			}
			break
		}
		seen[offset] = struct{}{}
		if offset < minOffset {
			minOffset = offset
		}
		record, payload, _, ok := readPptRecord(stream, offset)
		if !ok || record.Type != pptRTUserEditAtom || len(payload) < 20 {
			// Broken link: the target is not a valid UserEditAtom.
			// If we have already collected at least one valid edit (the chain
			// head was sound), stop walking and return what we have — matching
			// the robustness goal of HSLFSlideShowImpl.initRecordOffsets.
			// If nothing was collected yet, the start offset itself was bad.
			if len(edits) > 0 {
				break
			}
			return nil, false
		}
		edit := pptUserEdit{
			OffsetLastEdit:         int(binary.LittleEndian.Uint32(payload[8:12])),
			OffsetPersistDirectory: int(binary.LittleEndian.Uint32(payload[12:16])),
			DocPersistIDRef:        int(binary.LittleEndian.Uint32(payload[16:20])),
		}
		edits = append(edits, edit)
		offset = edit.OffsetLastEdit
	}
	return edits, len(edits) > 0
}

func buildPersistDirectory(stream []byte, edits []pptUserEdit) map[int]int {
	directory := map[int]int{}
	for index := len(edits) - 1; index >= 0; index-- {
		record, payload, _, ok := readPptRecord(stream, edits[index].OffsetPersistDirectory)
		if !ok || record.Type != pptRTPersistDirectory {
			continue
		}
		applyPersistDirectoryEntries(directory, payload)
	}
	return directory
}

func applyPersistDirectoryEntries(directory map[int]int, payload []byte) {
	for offset := 0; offset+4 <= len(payload); {
		header := binary.LittleEndian.Uint32(payload[offset : offset+4])
		offset += 4
		persistID := int(header & 0x000FFFFF)
		count := int(header >> 20)
		if count <= 0 || offset+count*4 > len(payload) {
			return
		}
		for index := 0; index < count; index++ {
			directory[persistID+index] = int(binary.LittleEndian.Uint32(payload[offset : offset+4]))
			offset += 4
		}
	}
}

func extractLegacyPptSlidesFromRootDocument(stream []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	record, _, _, ok := readPptRecord(stream, 0)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return nil
	}
	return extractSlideTextsFromDocument(stream, 0, 2, nil, fontTable, colorScheme, exHyperlinks)
}

func extractSlideTextsFromDocument(stream []byte, documentOffset int, notesListInstance uint16, persistDirectory map[int]int, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return nil
	}
	var combined []legacySlideText
	var notesList []legacySlideText
	// Ensure notesListInstance is not instance 0 (regular slides)
	if notesListInstance == 0 {
		notesListInstance = 2 // Default to instance 2 (notes), not instance 1 (master)
	}
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTSlideListWithText {
			switch child.Instance {
			case 0: // Regular slides
				combined = mergeLegacySlides(combined, parseSlideListWithText(childPayload, fontTable, colorScheme, exHyperlinks))
			case 1: // Master slides - explicitly skip; never treat as notes or regular slides
				// Instance 1 is for master slides only; POI handles these separately via getMasterSlideListWithText()
			case notesListInstance: // Notes or fallback
				notesList = mergeLegacySlides(notesList, parseSlideListWithText(childPayload, fontTable, colorScheme, exHyperlinks))
			}
		}
		pos = next
	}
	notesList = hydrateLegacyPptNotesFromPersistShapes(notesList, stream, persistDirectory, fontTable, colorScheme, exHyperlinks)
	if len(notesList) > 0 {
		combined = attachNotesByIndex(combined, notesList)
	}
	return combined
}

func selectLegacyPptNotesSlideListInstance(stream []byte, documentOffset int, persistDirectory map[int]int) uint16 {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return 2 // Default to instance 2 (notes)
	}
	// First pass: look for instance 2 (NOTES) explicitly
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTSlideListWithText && child.Instance == 2 {
			persistIDs := collectSlidePersistIDs(childPayload)
			if len(persistIDs) > 0 && legacyPptPersistIDsContainRecordType(stream, persistDirectory, persistIDs, pptRTNotes) {
				return 2 // Found notes
			}
		}
		pos = next
	}
	// If no instance 2 notes found, look for alternative but skip instance 1 (master slides)
	// Instance 1 should NEVER be used as notes
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type != pptRTSlideListWithText || child.Instance == 0 || child.Instance == 1 {
			pos = next
			continue
		}
		persistIDs := collectSlidePersistIDs(childPayload)
		if len(persistIDs) > 0 {
			if legacyPptPersistIDsContainRecordType(stream, persistDirectory, persistIDs, pptRTNotes) {
				return child.Instance
			}
		}
		pos = next
	}
	return 2 // Default to 2 if nothing found
}

func legacyPptPersistIDsContainRecordType(stream []byte, persistDirectory map[int]int, persistIDs []int, recordType uint16) bool {
	if len(persistDirectory) == 0 {
		return false
	}
	for _, persistID := range persistIDs {
		offset, ok := persistDirectory[persistID]
		if !ok {
			continue
		}
		record, _, _, ok := readPptRecord(stream, offset)
		if ok && record.Type == recordType {
			return true
		}
	}
	return false
}

func hydrateLegacyPptNotesFromPersistShapes(notes []legacySlideText, stream []byte, persistDirectory map[int]int, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	if len(notes) == 0 || len(persistDirectory) == 0 {
		return notes
	}
	for index := range notes {
		if notes[index].PersistID == 0 {
			continue
		}
		offset, ok := persistDirectory[notes[index].PersistID]
		if !ok {
			continue
		}
		record, payload, _, ok := readPptRecord(stream, offset)
		if !ok || record.Type != pptRTNotes {
			continue
		}
		scene := collectLegacyPptVisualScene(payload)
		shapes := buildLegacyPptTextShapesFromMetas(scene.textMetas(fontTable, colorScheme, exHyperlinks), notes[index])
		fragments := legacyPptNotesFragmentsFromShapes(shapes)
		if len(fragments) > 0 {
			notes[index].Fragments = mergeLegacyPptNotesFragments(notes[index].Fragments, fragments)
		}
	}
	return notes
}

func mergeLegacyPptNotesFragments(existing []legacySlideFragment, hydrated []legacySlideFragment) []legacySlideFragment {
	if len(hydrated) == 0 {
		return existing
	}
	if !legacyPptHasRenderableNotesFragments(existing) {
		out := make([]legacySlideFragment, 0, len(existing)+len(hydrated))
		for _, frag := range existing {
			if legacyPptFragmentIsNotesCandidate(frag) {
				continue
			}
			out = append(out, frag)
		}
		out = append(out, hydrated...)
		return out
	}
	out := append([]legacySlideFragment(nil), existing...)
	for _, frag := range hydrated {
		duplicate := false
		for _, have := range out {
			if !legacyPptFragmentIsNotesCandidate(have) {
				continue
			}
			if strings.TrimSpace(have.Text) == strings.TrimSpace(frag.Text) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, frag)
		}
	}
	return out
}

func legacyPptHasRenderableNotesFragments(fragments []legacySlideFragment) bool {
	for _, frag := range fragments {
		if !legacyPptFragmentIsNotesCandidate(frag) {
			continue
		}
		text := strings.TrimSpace(frag.Text)
		if text == "" {
			continue
		}
		if legacyPptLooksLikePlaceholderSampleText(text) {
			continue
		}
		return true
	}
	return false
}

func legacyPptFragmentIsNotesCandidate(frag legacySlideFragment) bool {
	return frag.TextType == pptTextTypeNotes || frag.TextType == pptTextTypeUnknown
}

func legacyPptNotesFragmentsFromShapes(shapes []*presentationir.Shape) []legacySlideFragment {
	fragments := make([]legacySlideFragment, 0, len(shapes))
	for _, shape := range shapes {
		if shape == nil || len(shape.Paragraphs) == 0 {
			continue
		}
		text := legacyPptParagraphsText(shape.Paragraphs)
		if strings.TrimSpace(text) == "" {
			continue
		}
		fragments = append(fragments, legacySlideFragment{
			Text:       text,
			TextType:   pptTextTypeNotes,
			Paragraphs: cloneLegacyParagraphs(shape.Paragraphs),
		})
	}
	return fragments
}

func legacyPptParagraphsText(paragraphs []*presentationir.Paragraph) string {
	parts := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if paragraph == nil {
			continue
		}
		var builder strings.Builder
		for _, run := range paragraph.Runs {
			if run == nil {
				continue
			}
			builder.WriteString(run.Text)
		}
		if text := strings.TrimSpace(builder.String()); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func collectPersistIDsFromDocument(stream []byte, documentOffset int, slideListInstance uint16) []int {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return nil
	}
	var persistIDs []int
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTSlideListWithText && child.Instance == slideListInstance {
			persistIDs = append(persistIDs, collectSlidePersistIDs(childPayload)...)
		}
		pos = next
	}
	return persistIDs
}

func collectSlidePersistIDs(data []byte) []int {
	var persistIDs []int
	for pos := 0; pos+8 <= len(data); {
		record, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			break
		}
		if record.Type == pptRTSlidePersistAtom && len(payload) >= 4 {
			persistIDs = append(persistIDs, int(binary.LittleEndian.Uint32(payload[:4])))
		}
		pos = next
	}
	return persistIDs
}

// collectMasterEntriesFromDocument walks the master SlideListWithText
// (instance 1) in the Document container and returns one legacyPptMasterEntry
// per SlidePersistAtom, pairing the persist-directory key (RefID, payload[0:4])
// with the slide-identifier used by slides to reference this master
// (SlideIdentifier, payload[12:16] — same layout as SlidePersistAtom.slideIdentifier
// in the POI spec).
func collectMasterEntriesFromDocument(stream []byte, documentOffset int) []legacyPptMasterEntry {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return nil
	}
	var entries []legacyPptMasterEntry
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTSlideListWithText && child.Instance == 1 {
			for spos := 0; spos+8 <= len(childPayload); {
				sRecord, sPayload, sNext, sOk := readPptRecord(childPayload, spos)
				if !sOk {
					break
				}
				if sRecord.Type == pptRTSlidePersistAtom && len(sPayload) >= 16 {
					refID := int(binary.LittleEndian.Uint32(sPayload[:4]))
					slideID := int(int32(binary.LittleEndian.Uint32(sPayload[12:16])))
					entries = append(entries, legacyPptMasterEntry{RefID: refID, SlideIdentifier: slideID})
				}
				spos = sNext
			}
		}
		pos = next
	}
	return entries
}

// parseLegacyPptSlideShowInfo decodes SSlideInfoAtom according to MS-PPT
// §2.5.27. The flags are not OOXML transition flags: fManualAdvance is bit 0,
// fHidden is bit 2 and fAutoAdvance is bit 10. The trailing speed byte is
// present in the 16-byte atom payload (12 bytes of fields plus speed+unused).
func parseLegacyPptSlideShowInfo(payload []byte) (*presentationir.SlideTransition, bool) {
	if len(payload) < 12 {
		return nil, false
	}
	slideTime := int32(binary.LittleEndian.Uint32(payload[0:4]))
	effectType := payload[9]
	flags := binary.LittleEndian.Uint16(payload[10:12])

	transition := &presentationir.SlideTransition{
		Type:     legacyPptTransitionType(effectType),
		AdvClick: flags&(1<<0) != 0, // fManualAdvance
	}
	if flags&(1<<10) != 0 && slideTime > 0 { // fAutoAdvance
		transition.AdvTmMs = int(slideTime)
	}
	if len(payload) >= 13 {
		switch payload[12] {
		case 0x00:
			transition.Spd = "slow" // 0.75 s
		case 0x01:
			transition.Spd = "med" // 0.5 s
		case 0x02:
			transition.Spd = "fast" // 0.25 s
		}
	}
	return transition, true
}

func legacyPptTextMetaFieldFromRecord(recordType uint16, payload []byte) (legacyPptTextMetaField, bool) {
	if len(payload) < 4 {
		return legacyPptTextMetaField{}, false
	}
	kind := ""
	switch recordType {
	case pptRTSlideNumberMCAtom:
		kind = "slidenum"
	case pptRTDateTimeMCAtom, pptRTGenericDateMCAtom, pptRTRTFDateTimeMCAtom:
		kind = "datetime"
	case pptRTHeaderMCAtom:
		kind = "header"
	case pptRTFooterMCAtom:
		kind = "footer"
	default:
		return legacyPptTextMetaField{}, false
	}
	return legacyPptTextMetaField{Position: int(binary.LittleEndian.Uint32(payload[:4])), Kind: kind}, true
}

func buildLegacyPptFieldParagraphs(text string) []*presentationir.Paragraph {
	text = strings.ReplaceAll(text, "\r\n", "\r")
	text = strings.ReplaceAll(text, "\n", "\r")
	text = normalizePptSoftLineBreaks(text)
	parts := strings.Split(text, "\r")
	out := make([]*presentationir.Paragraph, 0, len(parts))
	for _, part := range parts {
		// Do not apply placeholder-sample filtering here. A metacharacter is
		// commonly serialized as "*" and becomes semantic only after the
		// corresponding *MCAtom is applied below.
		out = append(out, &presentationir.Paragraph{Runs: []*presentationir.Run{{Text: part}}})
	}
	return out
}

func legacyPptUTF16Len(text string) int {
	length := 0
	for _, r := range text {
		if r > 0xFFFF {
			length += 2
		} else {
			length++
		}
	}
	return length
}

func applyLegacyPptTextMetaFields(paragraphs []*presentationir.Paragraph, fields []legacyPptTextMetaField) []*presentationir.Paragraph {
	if len(paragraphs) == 0 || len(fields) == 0 {
		return paragraphs
	}
	fields = append([]legacyPptTextMetaField(nil), fields...)
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].Position < fields[j].Position })

	out := make([]*presentationir.Paragraph, 0, len(paragraphs))
	absolute := 0
	fieldIndex := 0
	for paragraphIndex, paragraph := range paragraphs {
		clone := cloneLegacyParagraph(paragraph)
		if clone == nil {
			continue
		}
		var runs []*presentationir.Run
		for _, run := range clone.Runs {
			if run == nil {
				continue
			}
			runes := []rune(run.Text)
			pieceStart := 0
			localUTF16 := 0
			for runeIndex, r := range runes {
				runeUnits := 1
				if r > 0xFFFF {
					runeUnits = 2
				}
				for fieldIndex < len(fields) && fields[fieldIndex].Position < absolute+localUTF16 {
					fieldIndex++
				}
				if fieldIndex < len(fields) && fields[fieldIndex].Position == absolute+localUTF16 {
					if runeIndex > pieceStart {
						textRun := *run
						textRun.Text = string(runes[pieceStart:runeIndex])
						textRun.FieldKind = ""
						runs = append(runs, &textRun)
					}
					fieldRun := *run
					fieldRun.Text = ""
					fieldRun.FieldKind = fields[fieldIndex].Kind
					runs = append(runs, &fieldRun)
					pieceStart = runeIndex + 1 // the source marker occupies this character
					fieldIndex++
				}
				localUTF16 += runeUnits
			}
			if pieceStart < len(runes) {
				textRun := *run
				textRun.Text = string(runes[pieceStart:])
				textRun.FieldKind = ""
				runs = append(runs, &textRun)
			} else if len(runes) == 0 && run.FieldKind != "" {
				runs = append(runs, run)
			}
			absolute += legacyPptUTF16Len(run.Text)
		}
		clone.Runs = runs
		out = append(out, clone)
		// PowerPoint text positions count the paragraph terminator between
		// paragraphs. There is no terminator after the final paragraph.
		if paragraphIndex+1 < len(paragraphs) {
			absolute++
		}
	}
	return out
}

func finalizeLegacyPptFragmentMetaFields(frag *legacySlideFragment) {
	if frag == nil || len(frag.metaFields) == 0 {
		return
	}
	paragraphs := frag.Paragraphs
	if paragraphs == nil {
		paragraphs = buildLegacyPptFieldParagraphs(frag.RawText)
		if frag.RawText == "" {
			paragraphs = buildLegacyPptFieldParagraphs(frag.Text)
		}
	}
	frag.Paragraphs = applyLegacyPptTextMetaFields(paragraphs, frag.metaFields)
}

func parseSlideListWithText(data []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	var slides []legacySlideText
	current := -1
	currentTextType := pptTextTypeUnknown
	currentOutlineRef := -1
	nextOutlineRef := 0
	for pos := 0; pos+8 <= len(data); {
		record, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTSlidePersistAtom:
			persistID := 0
			if len(payload) >= 4 {
				persistID = int(binary.LittleEndian.Uint32(payload[:4]))
			}
			slides = append(slides, legacySlideText{PersistID: persistID, MasterID: -1})
			current = len(slides) - 1
			currentTextType = pptTextTypeUnknown
			currentOutlineRef = -1
			nextOutlineRef = 0
		case pptRTSlideAtom:
			// Extract MasterID from SlideAtom (if we have a current slide).
			// SlideAtom structure: 8-byte header + 12-byte SSlideLayoutAtom + 4-byte masterID + ...
			// In the payload (without header), masterID is at offset 12-16.
			if current >= 0 && len(payload) >= 4 {
				slides[current].LayoutType = legacyPptSlideLayoutType(payload)
			}
			if current >= 0 && len(payload) >= 16 {
				masterID := int(int32(binary.LittleEndian.Uint32(payload[12:16])))
				slides[current].MasterID = masterID
			}
		case pptRTSlideShowSlideInfoAtom:
			// SSlideInfoAtom (MS-PPT §2.5.27) owns legacy slide-show transition
			// semantics. Keep its bit layout at the legacy boundary rather than
			// translating it through OOXML defaults.
			if current >= 0 {
				if transition, ok := parseLegacyPptSlideShowInfo(payload); ok {
					slides[current].Transition = transition
				}
			}
		case pptRTTextHeaderAtom:
			if len(payload) >= 4 {
				currentTextType = int(binary.LittleEndian.Uint32(payload[:4]))
			}
		case pptRTTextCharsAtom:
			if current >= 0 {
				raw := decodePptUTF16(payload)
				text := normalizePptText(raw)
				if text != "" {
					outlineRef := currentOutlineRef
					if outlineRef < 0 {
						outlineRef = nextOutlineRef
					}
					slides[current].Fragments = append(slides[current].Fragments, legacySlideFragment{Text: text, RawText: raw, TextType: currentTextType, OutlineRef: outlineRef})
					nextOutlineRef = outlineRef + 1
				}
				currentTextType = pptTextTypeUnknown
				currentOutlineRef = -1
			}
		case pptRTTextBytesAtom:
			if current >= 0 {
				raw := decodePptBytes(payload)
				text := normalizePptText(raw)
				if text != "" {
					outlineRef := currentOutlineRef
					if outlineRef < 0 {
						outlineRef = nextOutlineRef
					}
					slides[current].Fragments = append(slides[current].Fragments, legacySlideFragment{Text: text, RawText: raw, TextType: currentTextType, OutlineRef: outlineRef})
					nextOutlineRef = outlineRef + 1
				}
				currentTextType = pptTextTypeUnknown
				currentOutlineRef = -1
			}
		case pptRTStyleTextPropAtom:
			if current >= 0 {
				frags := slides[current].Fragments
				if n := len(frags); n > 0 {
					if paras := parseStyleTextPropAtomWithTheme(frags[n-1].RawText, payload, fontTable, colorScheme); paras != nil {
						slides[current].Fragments[n-1].Paragraphs = paras
						if slides[current].Fragments[n-1].pendingRuler != nil {
							applyRulerLevelsToParas(paras, *slides[current].Fragments[n-1].pendingRuler)
							slides[current].Fragments[n-1].pendingRuler = nil
						}
					}
				}
			}
		case pptRTStyleTextProp9Atom:
			if current >= 0 {
				frags := slides[current].Fragments
				if n := len(frags); n > 0 {
					if stpa9Data := parseStyleTextProp9Atom(payload); stpa9Data != nil {
						// Apply auto-numbering to paragraphs if they exist
						if paras := slides[current].Fragments[n-1].Paragraphs; paras != nil {
							applyStyleTextProp9ToParas(paras, stpa9Data)
						} else {
							// Store for later application if paragraphs haven't been created yet
							slides[current].Fragments[n-1].pendingSTP9 = stpa9Data
						}
					}
				}
			}
		case pptRTTextRulerAtom:
			if current >= 0 {
				frags := slides[current].Fragments
				if n := len(frags); n > 0 {
					if ruler, ok := parseTextRulerAtom(payload); ok {
						if slides[current].Fragments[n-1].Paragraphs != nil {
							applyRulerLevelsToParas(slides[current].Fragments[n-1].Paragraphs, ruler)
						} else {
							slides[current].Fragments[n-1].pendingRuler = &ruler
						}
					}
				}
			}
		case pptRTSlideNumberMCAtom, pptRTDateTimeMCAtom, pptRTGenericDateMCAtom, pptRTHeaderMCAtom, pptRTFooterMCAtom, pptRTRTFDateTimeMCAtom:
			if current >= 0 {
				frags := slides[current].Fragments
				if n := len(frags); n > 0 {
					if field, ok := legacyPptTextMetaFieldFromRecord(record.Type, payload); ok {
						slides[current].Fragments[n-1].metaFields = append(slides[current].Fragments[n-1].metaFields, field)
					}
				}
			}
		case pptRTInteractiveInfoAtom:
			if current >= 0 {
				frags := slides[current].Fragments
				if n := len(frags); n > 0 {
					applyLegacyHyperlinkToFragment(&slides[current].Fragments[n-1], parseLegacyPptInteractiveInfoURL(payload, exHyperlinks))
				}
			}
		case pptRTInteractiveInfo:
			if current >= 0 {
				frags := slides[current].Fragments
				if n := len(frags); n > 0 {
					url := parseLegacyPptInteractiveInfoURLFromContainer(payload, exHyperlinks)
					applyLegacyHyperlinkToFragment(&slides[current].Fragments[n-1], url)
				}
			}
		}
		pos = next
	}

	for i := range slides {
		for j := range slides[i].Fragments {
			finalizeLegacyPptFragmentMetaFields(&slides[i].Fragments[j])
		}
	}
	return slides
}

func parseLegacyPptInteractiveInfoURLFromContainer(payload []byte, exHyperlinks map[uint32]string) string {
	if len(payload) == 0 || len(exHyperlinks) == 0 {
		return ""
	}
	url := ""
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		if url != "" {
			return
		}
		if record.Type == pptRTInteractiveInfoAtom {
			url = parseLegacyPptInteractiveInfoURL(childPayload, exHyperlinks)
		}
	})
	return url
}

// parseLegacyPptShapeInteractiveInfo recursively walks ClientData payloads
// looking for InteractiveInfoAtom records (RT 0x0FD1) so that shape-level
// click actions encoded under MouseClickInteractiveInfoContainers surface as
// Shape.HyperlinkURL / Shape.ActionLink. It returns the first non-empty URL
// (ACTION_HYPERLINK) and the first action vocabulary string (ACTION_JUMP).
func parseLegacyPptShapeInteractiveInfo(data []byte, exHyperlinks map[uint32]string) (string, string) {
	url, action := "", ""
	walkPptRecords(data, func(record pptRecord, payload []byte) {
		if url != "" && action != "" {
			return
		}
		if record.Type != pptRTInteractiveInfoAtom {
			return
		}
		if u, a := parseLegacyPptInteractiveInfoURLAndAction(payload, exHyperlinks); u != "" || a != "" {
			if url == "" {
				url = u
			}
			if action == "" {
				action = a
			}
		}
	})
	return url, action
}

func parseLegacyPptShapeAnimationInfo(data []byte) (legacyPptAnimationInfo, bool) {
	var info legacyPptAnimationInfo
	found := false
	walkPptRecords(data, func(record pptRecord, payload []byte) {
		if record.Type != pptRTAnimationInfoAtom {
			return
		}
		parsed, ok := parseLegacyPptAnimationInfoAtom(payload)
		if !ok {
			return
		}
		// Some producers emit more than one AnimationInfoAtom for a shape.
		// Merge the strongest available semantics instead of stopping at the
		// first atom, while keeping the earliest positive build order.
		if !found || (parsed.Step > 0 && (info.Step == 0 || parsed.Step < info.Step)) {
			if parsed.Step > 0 {
				info.Step = parsed.Step
			}
		}
		if parsed.Trigger != "" {
			info.Trigger = parsed.Trigger
		}
		if parsed.Effect != "" {
			info.Effect = parsed.Effect
		}
		if parsed.DelayMs > info.DelayMs {
			info.DelayMs = parsed.DelayMs
		}
		found = true
	})
	return info, found
}

func parseLegacyPptAnimationInfoAtom(payload []byte) (legacyPptAnimationInfo, bool) {
	if len(payload) < 28 {
		return legacyPptAnimationInfo{}, false
	}
	flags := binary.LittleEndian.Uint16(payload[4:6])
	automatic := ((flags >> 2) & 0x3) == 1
	delay := int(int32(binary.LittleEndian.Uint32(payload[12:16])))
	step := int(int16(binary.LittleEndian.Uint16(payload[16:18])))
	buildType := payload[20]
	effect := legacyPptAnimationEffectName(payload[21])
	trigger := "onClick"
	if automatic {
		trigger = "afterPrevious"
	}
	if buildType == 0 {
		step = 0
	}
	if step < 0 {
		step = 0
	}
	if delay < 0 {
		delay = 0
	}
	if trigger == "" && effect == "" && step == 0 && delay == 0 {
		return legacyPptAnimationInfo{}, false
	}
	return legacyPptAnimationInfo{
		Step:    step,
		Trigger: trigger,
		Effect:  effect,
		DelayMs: delay,
	}, true
}

// parseLegacyPptInteractiveInfoURLAndAction extends the URL-only decoder to
// also produce an action-link string for ACTION_JUMP. The byte-9 jumpType
// enum maps to the OOXML ppaction vocabulary (firstSlide/lastSlide/...).
func parseLegacyPptInteractiveInfoURLAndAction(payload []byte, exHyperlinks map[uint32]string) (string, string) {
	if len(payload) < 9 {
		return "", ""
	}
	switch payload[8] {
	case 4: // ACTION_HYPERLINK
		if len(exHyperlinks) == 0 {
			return "", ""
		}
		id := binary.LittleEndian.Uint32(payload[4:8])
		if u, ok := exHyperlinks[id]; ok {
			if trimmed := strings.TrimSpace(u); trimmed != "" {
				return trimmed, ""
			}
		}
	case 3: // ACTION_JUMP — jumpType is at byte 9.
		switch payload[9] {
		case 1:
			return "", "nextSlide"
		case 2:
			return "", "prevSlide"
		case 3:
			return "", "firstSlide"
		case 4:
			return "", "lastSlide"
		case 5:
			return "", "lastSlideViewed"
		case 6:
			return "", "endShow"
		}
	}
	return "", ""
}

func parseLegacyPptInteractiveInfoURL(payload []byte, exHyperlinks map[uint32]string) string {
	// InteractiveInfoAtom layout (after the 8-byte PPT record header):
	//   bytes 0-3:  soundRef (uint32)
	//   bytes 4-7:  hyperlinkID (uint32) — index into ExHyperlink list
	//   byte  8:    action (0=none,1=macro,2=runProgram,3=jump,4=hyperlink,5=ole,6=media,7=customShow)
	//   byte  9:    jump type
	//   byte  10:   flags (animated, etc.)
	//   byte  11:   stop-sound flag
	//   byte  12:   hyperlinkType (8=URL, 0xFF=null, etc.)
	if len(payload) < 9 || len(exHyperlinks) == 0 {
		return ""
	}
	// Only ACTION_HYPERLINK (4) points to an external URL via ExHyperlink.
	// ACTION_JUMP (3) targets a slide number and has no ExHyperlink entry.
	if payload[8] != 4 {
		return ""
	}
	id := binary.LittleEndian.Uint32(payload[4:8])
	if url, ok := exHyperlinks[id]; ok {
		if trimmed := strings.TrimSpace(url); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func applyLegacyHyperlinkToFragment(frag *legacySlideFragment, url string) {
	if frag == nil || strings.TrimSpace(url) == "" {
		return
	}
	paras := fragmentParagraphs(*frag)
	if len(paras) == 0 {
		return
	}
	for _, para := range paras {
		if para == nil {
			continue
		}
		if len(para.Runs) == 0 {
			para.Runs = []*presentationir.Run{{Text: frag.Text, HyperlinkURL: url}}
			continue
		}
		for _, run := range para.Runs {
			if run == nil {
				continue
			}
			if strings.TrimSpace(run.HyperlinkURL) == "" {
				run.HyperlinkURL = url
			}
		}
	}
	frag.Paragraphs = paras
}

func fallbackPptText(stream []byte) string {
	fragments := make([]string, 0, 16)
	walkPptRecords(stream, func(record pptRecord, payload []byte) {
		switch record.Type {
		case pptRTTextCharsAtom:
			text := normalizePptText(decodePptUTF16(payload))
			if text != "" {
				fragments = append(fragments, text)
			}
		case pptRTTextBytesAtom:
			text := normalizePptText(decodePptBytes(payload))
			if text != "" {
				fragments = append(fragments, text)
			}
		}
	})
	return normalizePptText(strings.Join(fragments, "\n"))
}

func walkPptRecords(data []byte, visit func(record pptRecord, payload []byte)) {
	for pos := 0; pos+8 <= len(data); {
		record, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			// Third-party generators occasionally leave damaged optional records
			// between otherwise valid siblings. Recover within a bounded window
			// instead of dropping the rest of the containing record family.
			recovered := false
			limit := pos + 4096
			if limit > len(data)-8 {
				limit = len(data) - 8
			}
			for candidate := pos + 1; candidate <= limit; candidate++ {
				if _, _, _, candidateOK := readPptRecord(data, candidate); candidateOK {
					pos = candidate
					recovered = true
					break
				}
			}
			if recovered {
				continue
			}
			break
		}
		visit(record, payload)
		if record.Version == 0x0F && len(payload) > 0 {
			walkPptRecords(payload, visit)
		}
		pos = next
	}
}

func readPptRecord(data []byte, pos int) (pptRecord, []byte, int, bool) {
	if pos < 0 || pos+8 > len(data) {
		return pptRecord{}, nil, pos, false
	}
	opts := binary.LittleEndian.Uint16(data[pos : pos+2])
	record := pptRecord{
		Version:  opts & 0x000F,
		Instance: opts >> 4,
		Type:     binary.LittleEndian.Uint16(data[pos+2 : pos+4]),
		Length:   int(binary.LittleEndian.Uint32(data[pos+4 : pos+8])),
	}
	next := pos + 8 + record.Length
	if record.Length < 0 || next > len(data) {
		return pptRecord{}, nil, pos, false
	}
	return record, data[pos+8 : next], next, true
}

func decodePptUTF16(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	var b strings.Builder
	for i := 0; i+1 < len(data); {
		r := rune(binary.LittleEndian.Uint16(data[i:]))
		if r == 0 {
			i += 2
			continue
		}
		// UTF-16 surrogate pair decode: high surrogate (0xD800-0xDBFF)
		// followed by low surrogate (0xDC00-0xDFFF).
		if r >= 0xD800 && r < 0xDC00 && i+3 < len(data) {
			low := rune(binary.LittleEndian.Uint16(data[i+2:]))
			if low >= 0xDC00 && low < 0xE000 {
				r = 0x10000 + (r-0xD800)*0x400 + (low - 0xDC00)
				i += 4
				b.WriteRune(r)
				continue
			}
		}
		b.WriteRune(r)
		i += 2
	}
	return b.String()
}

func decodePptBytes(data []byte) string {
	var b strings.Builder
	for _, ch := range data {
		if ch == 0 {
			continue
		}
		b.WriteRune(rune(ch))
	}
	return b.String()
}

func normalizePptText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\u000B", "\n")
	text = strings.ReplaceAll(text, "\u000C", "\n")

	var b strings.Builder
	prevNewline := true
	for _, r := range text {
		switch {
		case r == '\n':
			if !prevNewline {
				b.WriteRune('\n')
			}
			prevNewline = true
		case r == '\t':
			b.WriteRune('\t')
			prevNewline = false
		case unicode.IsPrint(r) || legacyPptPrivateUseRune(r):
			// Symbol/Wingdings text in binary PowerPoint is frequently stored as
			// Unicode private-use code points. unicode.IsPrint intentionally excludes
			// category Co, but these are authored glyph codes whose meaning is supplied
			// by the accompanying symbol font. Dropping them here loses the text before
			// TextCFException can attach the correct typeface.
			b.WriteRune(r)
			prevNewline = false
		}
	}

	lines := strings.Split(b.String(), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func legacyPptPrivateUseRune(r rune) bool {
	return (r >= 0xE000 && r <= 0xF8FF) ||
		(r >= 0xF0000 && r <= 0xFFFFD) ||
		(r >= 0x100000 && r <= 0x10FFFD)
}

func buildPresentation(slides []legacySlideText, filename string) *presentationir.Presentation {
	return buildPresentationWithMasterComposition(slides, nil, nil, legacyPptHeaderFooterInfo{}, filename, nil, nil, nil, "", nil)
}

func legacyPptSlideLayoutType(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	switch binary.LittleEndian.Uint32(payload[:4]) {
	case 0x0000:
		return "title"
	case 0x0001:
		return "titleBody"
	case 0x0002:
		return "masterTitle"
	case 0x0003:
		return "masterSlide"
	case 0x0004:
		return "masterNotes"
	case 0x0005:
		return "notesTitleBody"
	case 0x0006:
		return "handout"
	case 0x0007:
		return "titleOnly"
	case 0x0008:
		return "twoColumns"
	case 0x0009:
		return "twoRows"
	case 0x000A:
		return "columnTwoRows"
	case 0x000B:
		return "twoRowsColumn"
	case 0x000C:
		return "title2RowBottom2ColumnBody"
	case 0x000D:
		return "twoColumnsRow"
	case 0x000E:
		return "fourObjects"
	case 0x000F:
		return "bigObject"
	case 0x0010:
		return "blank"
	case 0x0011:
		return "verticalTitleBody"
	case 0x0012:
		return "verticalTwoRows"
	default:
		return ""
	}
}

// buildPresentationWithMasterComposition is the BIG37 inheritance boundary.
// Master-specific styles and backgrounds are selected while each effective slide
// is constructed; callers must not mutate the canonical presentation afterward.
func buildPresentationWithMasterComposition(slides []legacySlideText, extraShapes map[int][]*presentationir.Shape, slideHeaderFooters map[int]legacyPptHeaderFooterInfo, slideHeaderFooterDefaults legacyPptHeaderFooterInfo, filename string, theme *presentationir.Theme, masterStylesByID map[int]legacyPptMasterTextStyles, masterTextStyles legacyPptMasterTextStyles, documentBgFill string, masterBgFillByPersistID map[int]string) *presentationir.Presentation {
	return buildPresentationWithMasterBackgroundPaint(slides, extraShapes, slideHeaderFooters, slideHeaderFooterDefaults, filename, theme, masterStylesByID, masterTextStyles, documentBgFill, masterBgFillByPersistID, nil)
}

func buildPresentationWithMasterBackgroundPaint(slides []legacySlideText, extraShapes map[int][]*presentationir.Shape, slideHeaderFooters map[int]legacyPptHeaderFooterInfo, slideHeaderFooterDefaults legacyPptHeaderFooterInfo, filename string, theme *presentationir.Theme, masterStylesByID map[int]legacyPptMasterTextStyles, masterTextStyles legacyPptMasterTextStyles, documentBgFill string, masterBgFillByPersistID map[int]string, masterPictureBgFillByPersistID map[int]drawing.Fill) *presentationir.Presentation {
	return buildPresentationWithEffectiveMasterComposition(slides, extraShapes, nil, slideHeaderFooters, slideHeaderFooterDefaults, filename, theme, masterStylesByID, masterTextStyles, documentBgFill, masterBgFillByPersistID, masterPictureBgFillByPersistID, nil, nil)
}

// buildPresentationWithEffectiveMasterComposition is the Pass 7 inheritance boundary.
// It is the only production location that combines persisted slide content with
// inherited master paint. Master artwork is no longer pre-spliced into the local
// shape map before background/style/header-footer inheritance is resolved.
func buildPresentationWithEffectiveMasterComposition(slides []legacySlideText, extraShapes map[int][]*presentationir.Shape, slideTextRoles map[int]map[uint32]int, slideHeaderFooters map[int]legacyPptHeaderFooterInfo, slideHeaderFooterDefaults legacyPptHeaderFooterInfo, filename string, theme *presentationir.Theme, masterStylesByID map[int]legacyPptMasterTextStyles, masterTextStyles legacyPptMasterTextStyles, documentBgFill string, masterBgFillByPersistID map[int]string, masterPictureBgFillByPersistID map[int]drawing.Fill, masterVisualShapesByPersistID map[int][]*presentationir.Shape, masterPlaceholderTemplates map[int]map[presentationir.PlaceholderKind]legacyPptTextShape) *presentationir.Presentation {
	slides, slideHeaderFooters = normalizeLegacyPptHeaderFooterOnlySlides(slides, extraShapes, slideHeaderFooters)
	presentation := &presentationir.Presentation{
		SlideWidthEMU:  defaultSlideWidthEMU,
		SlideHeightEMU: defaultSlideHeightEMU,
		Slides:         make([]*presentationir.Slide, 0, len(slides)),
		Theme:          theme,
	}
	// Canonical IR owns the source template graph for both parser families. Legacy
	// PPT has a master plus an SSlideLayoutAtom on each slide rather than OOXML's
	// explicit slideLayout part, so expose that relationship as a master + logical
	// layout graph here. The effective slide scene below remains independently
	// materialized; these shapes are immutable inheritance templates.
	legacyMasters := make(map[int]*presentationir.SlideMaster)
	legacyLayouts := make(map[string]*presentationir.SlideLayout)
	masterForSlide := func(slide legacySlideText) (*presentationir.SlideMaster, *presentationir.SlideLayout) {
		if slide.MasterPersistID == 0 {
			return nil, nil
		}
		master := legacyMasters[slide.MasterPersistID]
		if master == nil {
			master = &presentationir.SlideMaster{}
			for _, shape := range masterVisualShapesByPersistID[slide.MasterPersistID] {
				if clone := cloneLegacyShapeRecursive(shape, nil); clone != nil {
					master.Shapes = append(master.Shapes, clone)
				}
			}
			legacyMasters[slide.MasterPersistID] = master
			presentation.SlideMasters = append(presentation.SlideMasters, master)
		}
		key := fmt.Sprintf("%d/%s", slide.MasterPersistID, slide.LayoutType)
		layout := legacyLayouts[key]
		if layout == nil {
			layout = &presentationir.SlideLayout{Type: slide.LayoutType, Master: master}
			legacyLayouts[key] = layout
			master.Layouts = append(master.Layouts, layout)
		}
		return master, layout
	}
	var masterBackgroundColor string
	if documentBgFill != "" {
		masterBackgroundColor = documentBgFill
	} else {
		masterBackgroundColor = legacyPptMasterBackgroundColor(theme)
	}
	for index, slideText := range slides {
		// Select the master text styles for this slide: use the master referenced
		// by SlideAtom.masterID when available (POI HSLFSlide.getMasterSheet()
		// parity); fall back to the Document-level stream-walk styles otherwise.
		slideStyles := masterTextStyles
		if slideText.MasterID != -1 {
			if masterSpecific, ok := masterStylesByID[slideText.MasterID]; ok {
				// A persisted master commonly stores only the properties that differ
				// from DocumentTextInfo / environment defaults.  Treating that sparse
				// map as a replacement drops the inherited character style entirely.
				// Keep the exact master authoritative, but fill its missing properties
				// from the document-level cascade (the same ownership model as POI's
				// master-sheet + environment lookup).
				slideStyles = cloneLegacyPptMasterTextStyles(masterSpecific)
				slideStyles = mergeLegacyPptMasterStyleMaps(slideStyles, masterTextStyles)
			}
		}
		effectiveBackground := masterBackgroundColor
		if slideText.BgColor != "" {
			effectiveBackground = slideText.BgColor
		} else if (!slideText.HasSlideFlags || slideText.FollowMasterBackground) && slideText.MasterPersistID != 0 {
			if masterBackground := masterBgFillByPersistID[slideText.MasterPersistID]; masterBackground != "" {
				effectiveBackground = masterBackground
			}
		}
		var effectiveBackgroundFill drawing.Fill
		// A slide-local background wins. Otherwise inherit the master's complete
		// background paint only when SlideAtom.fMasterBackground permits it.
		if slideText.BgFill != nil {
			effectiveBackgroundFill = slideText.BgFill
		} else if (!slideText.HasSlideFlags || slideText.FollowMasterBackground) && slideText.MasterPersistID != 0 {
			effectiveBackgroundFill = masterPictureBgFillByPersistID[slideText.MasterPersistID]
		}
		headerFooterInfo := effectiveLegacyPptHeaderFooterInfo(slideText, slideHeaderFooterDefaults, slideHeaderFooters[slideText.PersistID])
		notesParagraphs := extractLegacyNotesParagraphs(slideText)
		hasNotesContent := len(notesParagraphs) > 0 || slideText.NotesHeaderFooter.hasRenderableContent()
		baseShapes := make([]*presentationir.Shape, 0, 6)
		// Master artwork and local artwork are materialized in authored paint order at
		// the same boundary that resolves every other inherited slide property.
		// fMasterObjects suppresses only painted master objects; master text styles,
		// placeholder templates and background inheritance remain independent switches.
		baseShapes = append(baseShapes, inheritLegacyPptMasterVisualsForSlide(slideText, masterVisualShapesByPersistID)...)
		localShapes := extraShapes[slideText.PersistID]
		anchoredTextShapes, nonTextShapes := splitLegacyExtraShapes(localShapes)
		if len(anchoredTextShapes) > 0 {
			// The event-driven semantic scene has already reconciled every shape family
			// back into authored OfficeArt source order. Never split that complete scene
			// into text/non-text families again here: doing so destroys z-order and can
			// expose objects that are intentionally covered by a later authored shape.
			baseShapes = append(baseShapes, cloneLegacyExtraShapes(index, len(baseShapes)+1, localShapes)...)
		} else {
			// Damaged/text-only compatibility slides can lack an anchored OfficeArt text
			// projection. In that case synthesize text from SlideListWithText and retain
			// the surviving non-text scene after it, matching the historical fallback.
			if len(slideText.Fragments) > 0 {
				baseShapes = append(baseShapes, buildLegacySlideShapes(index, slideText, filename)...)
			}
			if extras := cloneLegacyExtraShapes(index, len(baseShapes)+1, nonTextShapes); len(extras) > 0 {
				baseShapes = append(baseShapes, extras...)
			}
		}
		if len(baseShapes) == 0 {
			// Persist records that contain only PowerPoint's placeholder/sample prompts
			// are structural scaffolding (most commonly header/footer or notes metadata),
			// not authored empty slides.  Preserve a genuinely empty persisted slide, but
			// never let prompt-only scaffolding materialize a second slide.
			if !headerFooterInfo.hasRenderableContent() && !hasNotesContent {
				if legacyPptSlideHasOnlyPlaceholderPrompts(slideText) {
					continue
				}
				master, layout := masterForSlide(slideText)
				presentation.Slides = append(presentation.Slides, &presentationir.Slide{
					Index:           index,
					MasterID:        slideText.MasterID,
					MasterPersistID: slideText.MasterPersistID,
					LayoutType:      slideText.LayoutType,
					BgColor:         effectiveBackground,
					BgFill:          effectiveBackgroundFill,
					Transition:      slideText.Transition,
					Master:          master,
					Layout:          layout,
				})
				continue
			}
		}
		header, shapes, footer, dateTime, slideNumber := splitLegacySlideShapes(baseShapes)
		header, footer, dateTime, slideNumber = synthesizeLegacyHeaderFooterShapes(index, headerFooterInfo, masterPlaceholderTemplates[slideText.MasterID], header, footer, dateTime, slideNumber)
		var notes *presentationir.NotesSlide
		if noteShapes := buildLegacyNotesShapes(index, notesParagraphs, slideText.NotesHeaderFooter); len(noteShapes) > 0 {
			for _, shape := range noteShapes {
				textType := pptTextTypeOther
				if shape.Placeholder == presentationir.PlaceholderBody {
					textType = pptTextTypeNotes
				}
				applyLegacyPptMasterTextStylesToShape(shape, slideStyles, textType)
			}
			notes = &presentationir.NotesSlide{Shapes: noteShapes}
		}
		roles := slideTextRoles[slideText.PersistID]
		for _, shape := range shapes {
			applyLegacyPptMasterTextStylesToShapeWithRoles(shape, slideStyles, legacyPptMasterTextTypeForShapeWithRoles(shape, roles), roles)
		}
		normalizeLegacyPptPlaceholderGeometry(shapes)
		normalizeLegacyPptAnimatedEntrancePaint(shapes)
		markLegacyPptAnimationStateOverlays(shapes)
		if header != nil {
			applyLegacyPptMasterTextStylesToShape(header, slideStyles, pptTextTypeOther)
		}
		if footer != nil {
			applyLegacyPptMasterTextStylesToShape(footer, slideStyles, pptTextTypeOther)
		}
		if dateTime != nil {
			applyLegacyPptMasterTextStylesToShape(dateTime, slideStyles, pptTextTypeOther)
		}
		if slideNumber != nil {
			applyLegacyPptMasterTextStylesToShape(slideNumber, slideStyles, pptTextTypeOther)
		}
		master, layout := masterForSlide(slideText)
		presentation.Slides = append(presentation.Slides, &presentationir.Slide{
			Index:             index,
			MasterID:          slideText.MasterID,
			MasterPersistID:   slideText.MasterPersistID,
			LayoutType:        slideText.LayoutType,
			BgColor:           effectiveBackground,
			BgFill:            effectiveBackgroundFill,
			Shapes:            shapes,
			Header:            header,
			Footer:            footer,
			DateTime:          dateTime,
			SlideNumber:       slideNumber,
			Notes:             notes,
			Transition:        slideText.Transition,
			AnimationTimeline: buildLegacySlideAnimationTimeline(header, shapes, footer, dateTime, slideNumber),
			Master:            master,
			Layout:            layout,
		})
	}
	return presentation
}

func effectiveLegacyPptHeaderFooterInfo(slide legacySlideText, defaults, override legacyPptHeaderFooterInfo) legacyPptHeaderFooterInfo {
	// Header/footer placeholders are master-owned objects in legacy PPT. When
	// SlideAtom.fMasterObjects is clear, document/master defaults must not be
	// synthesized onto the slide. A genuine slide-local HeadersFooters override
	// remains authoritative and may explicitly enable its own footer/date.
	if slide.HasSlideFlags && !slide.FollowMasterObjects {
		return legacyPptHeaderFooterInfo{}.mergedWith(override)
	}
	return defaults.mergedWith(override)
}

func normalizeLegacyPptHeaderFooterOnlySlides(slides []legacySlideText, extraShapes map[int][]*presentationir.Shape, slideHeaderFooters map[int]legacyPptHeaderFooterInfo) ([]legacySlideText, map[int]legacyPptHeaderFooterInfo) {
	if len(slides) == 0 || len(slideHeaderFooters) == 0 {
		return slides, slideHeaderFooters
	}
	normalized := make(map[int]legacyPptHeaderFooterInfo, len(slideHeaderFooters))
	for persistID, info := range slideHeaderFooters {
		normalized[persistID] = info
	}
	out := make([]legacySlideText, 0, len(slides))
	for _, slide := range slides {
		persistID := slide.PersistID
		// Header/footer persist records can surface through SlideListWithText as a
		// synthetic slide containing only master prompt text.  Prompt-only text is
		// template metadata, not authored slide content, and must not create another
		// presentation slide.  Likewise ignore prompt-only placeholder projections
		// when deciding whether this persist owns a real slide.
		hasTextContent := len(slide.Fragments) > 0 && !legacyPptSlideHasOnlyPlaceholderPrompts(slide)
		hasShapeContent := legacyPptShapesHaveAuthoredSlideContent(extraShapes[persistID])
		hasContent := hasTextContent || hasShapeContent
		info := normalized[persistID]
		promptOnlyPersist := legacyPptSlideHasOnlyPlaceholderPrompts(slide) && len(slide.Fragments) > 0
		if !hasContent && len(out) > 0 && (info.hasAtom || info.hasRenderableContent() || promptOnlyPersist) {
			lastPersistID := out[len(out)-1].PersistID
			// A zero-flag HeadersFootersAtom is still authored state: it disables
			// the document/master header-footer defaults for the preceding real
			// slide. Header/footer metadata can arrive in its own prompt-only
			// persist after that slide, so move the override even when it has no
			// independently renderable content.
			if info.hasAtom || info.hasRenderableContent() {
				normalized[lastPersistID] = normalized[lastPersistID].mergedWith(info)
			}
			delete(normalized, persistID)
			continue
		}
		out = append(out, slide)
	}
	return out, normalized
}

func legacyPptShapesHaveAuthoredSlideContent(shapes []*presentationir.Shape) bool {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		// Placeholder prompts belong to the template/header-footer ownership layer.
		// A persist containing only those shapes is not an authored slide.
		if len(shape.Paragraphs) > 0 {
			onlyPrompts := true
			for _, paragraph := range shape.Paragraphs {
				if !legacyPptLooksLikePlaceholderSampleText(paragraphText(paragraph)) {
					onlyPrompts = false
					break
				}
			}
			if onlyPrompts {
				continue
			}
		}
		return true
	}
	return false
}

func legacyPptMasterTextTypeForShapeWithRoles(shape *presentationir.Shape, roles map[uint32]int) int {
	if shape != nil && shape.OriginalShapeID != 0 && len(roles) != 0 {
		if textType, ok := roles[shape.OriginalShapeID]; ok && textType != pptTextTypeUnknown {
			// TextHeaderAtom is the authoritative legacy text-style role even for
			// non-placeholder AutoShapes. Use it only to resolve the effective
			// style; canonical IR remains free of PPT-specific role metadata.
			return textType
		}
	}
	return legacyPptMasterTextTypeForShape(shape)
}

func legacyPptMasterTextTypeForShape(shape *presentationir.Shape) int {
	if shape == nil {
		return pptTextTypeUnknown
	}
	switch shape.Placeholder {
	case presentationir.PlaceholderTitle:
		return pptTextTypeTitle
	case presentationir.PlaceholderSubtitle:
		return pptTextTypeCenterBody
	case presentationir.PlaceholderBody:
		return pptTextTypeBody
	case presentationir.PlaceholderOther, presentationir.PlaceholderFooter, presentationir.PlaceholderDateTime, presentationir.PlaceholderSlideNumber:
		return pptTextTypeOther
	default:
		return pptTextTypeOther
	}
}

func parseLegacyPptDocumentHeaderFooterInfoFromPayload(payload []byte, instance uint16) legacyPptHeaderFooterInfo {
	var info legacyPptHeaderFooterInfo
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTHeadersFooters && child.Instance == instance {
			info = info.mergedWith(parseLegacyPptHeaderFooterInfo(childPayload))
		}
		pos = next
	}
	return info
}

func mapLegacyPptPersistHeaderFooterInfos(persistIDs []int, infos []legacyPptHeaderFooterInfo) map[int]legacyPptHeaderFooterInfo {
	if len(persistIDs) == 0 || len(infos) == 0 {
		return nil
	}
	out := make(map[int]legacyPptHeaderFooterInfo, len(infos))
	for index, persistID := range persistIDs {
		if persistID == 0 || index >= len(infos) {
			continue
		}
		if info := infos[index]; info.hasAtom || info.hasRenderableContent() {
			out[persistID] = info
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptHeaderFooterInfoFromPersist(payload []byte) legacyPptHeaderFooterInfo {
	var info legacyPptHeaderFooterInfo
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		if record.Type == pptRTHeadersFooters {
			info = info.mergedWith(parseLegacyPptHeaderFooterInfo(childPayload))
		}
	})
	return info
}

func parseLegacyPptHeaderFooterInfo(payload []byte) legacyPptHeaderFooterInfo {
	var info legacyPptHeaderFooterInfo
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch child.Type {
		case pptRTHeadersFootersAtom:
			if len(childPayload) >= 4 {
				info.hasAtom = true
				info.formatID = binary.LittleEndian.Uint16(childPayload[:2])
				info.flags = binary.LittleEndian.Uint16(childPayload[2:4])
			}
		case pptRTCString:
			text := strings.TrimSpace(normalizePptText(decodePptUTF16(childPayload)))
			switch child.Instance {
			case 0:
				info.dateText = text
			case 1:
				info.headerText = text
			case 2:
				info.footerText = text
			}
		}
		pos = next
	}
	return info
}

func synthesizeLegacyHeaderFooterShapes(slideIndex int, info legacyPptHeaderFooterInfo, templates map[presentationir.PlaceholderKind]legacyPptTextShape, header, footer, dateTime, slideNumber *presentationir.Shape) (*presentationir.Shape, *presentationir.Shape, *presentationir.Shape, *presentationir.Shape) {
	nextShapeID := 89
	if header == nil && info.headerEnabled() && info.headerText != "" {
		header = newLegacySlideShape(slideIndex, nextShapeID, "hdr", presentationir.PlaceholderOther, legacyHeaderBounds(), buildTextParagraphs(info.headerText))
		nextShapeID++
	}
	if footer == nil && info.footerEnabled() && info.footerText != "" {
		footer = newLegacyHeaderFooterShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Footer %d", slideIndex+1), presentationir.PlaceholderFooter, templates[presentationir.PlaceholderFooter], info.footerText, "")
		nextShapeID++
	}
	if dateTime == nil && info.dateEnabled() {
		if info.dateText != "" {
			dateTime = newLegacyHeaderFooterShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Date %d", slideIndex+1), presentationir.PlaceholderDateTime, templates[presentationir.PlaceholderDateTime], info.dateText, "")
		} else if info.flags&legacyPptHFFlagTodayDate != 0 {
			dateTime = newLegacyHeaderFooterShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Date %d", slideIndex+1), presentationir.PlaceholderDateTime, templates[presentationir.PlaceholderDateTime], "", "date")
		}
		if dateTime != nil {
			nextShapeID++
		}
	}
	if slideNumber == nil && info.slideNumberEnabled() {
		slideNumber = newLegacyHeaderFooterShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Slide Number %d", slideIndex+1), presentationir.PlaceholderSlideNumber, templates[presentationir.PlaceholderSlideNumber], "", "slidenum")
	}
	return header, footer, dateTime, slideNumber
}

func newLegacyHeaderFooterShape(slideIndex, shapeID int, name string, placeholder presentationir.PlaceholderKind, template legacyPptTextShape, text, fieldKind string) *presentationir.Shape {
	bounds := template.bounds
	if bounds.W <= 0 || bounds.H <= 0 {
		bounds = legacyFooterBoundsForPlaceholder(placeholder)
	}
	paragraphs := legacyHeaderFooterParagraphsFromTemplate(template, text, fieldKind)
	shape := newLegacySlideShape(slideIndex, shapeID, name, placeholder, bounds, paragraphs)
	if template.shapeID != 0 {
		shape.OriginalShapeID = template.shapeID
	}
	shape.PlaceholderIdx = template.placeholderIdx
	shape.FillColor = template.fillColor
	shape.OfficeArt = template.officeArt
	applyLegacyPptOfficeArtSurface(shape, template.officeArt, template.transform)
	if template.fillColor != "" && shape.Fill == nil {
		shape.Fill = &drawing.SolidFill{Color: template.fillColor, Alpha: 1.0}
	}
	return shape
}

func legacyHeaderFooterParagraphsFromTemplate(template legacyPptTextShape, text, fieldKind string) []*presentationir.Paragraph {
	var paragraph *presentationir.Paragraph
	if cloned := cloneLegacyParagraphs(template.paragraphs); len(cloned) > 0 {
		paragraph = cloned[0]
	} else if cloned := cloneLegacyParagraphs(template.inlineFallback); len(cloned) > 0 {
		paragraph = cloned[0]
	}
	if paragraph == nil {
		paragraph = &presentationir.Paragraph{}
	}
	var run presentationir.Run
	if len(paragraph.Runs) > 0 && paragraph.Runs[0] != nil {
		run = *paragraph.Runs[0]
	}
	run.Text = text
	run.FieldKind = fieldKind
	paragraph.Runs = []*presentationir.Run{&run}
	return []*presentationir.Paragraph{paragraph}
}

func applyLegacyPptMasterTextStylesToShape(shape *presentationir.Shape, styles legacyPptMasterTextStyles, textType int) {
	applyLegacyPptMasterTextStylesToShapeWithRoles(shape, styles, textType, nil)
}

func applyLegacyPptMasterTextStylesToShapeWithRoles(shape *presentationir.Shape, styles legacyPptMasterTextStyles, textType int, roles map[uint32]int) {
	if shape == nil {
		return
	}
	if shape.Kind == presentationir.ShapeGroup {
		for _, child := range shape.Children {
			applyLegacyPptMasterTextStylesToShapeWithRoles(child, styles, legacyPptMasterTextTypeForShapeWithRoles(child, roles), roles)
		}
		return
	}
	if shape.Kind != presentationir.ShapeText || len(shape.Paragraphs) == 0 || len(styles) == 0 {
		return
	}
	levels := legacyPptMasterTextLevelsForType(styles, textType)
	if len(levels) == 0 {
		return
	}
	for index, paragraph := range shape.Paragraphs {
		if paragraph == nil {
			continue
		}
		reqLevel := legacyPptParagraphLevel(paragraph)
		masterLevel, ok := levels[reqLevel]
		if !ok {
			// Clamp to highest defined level ≤ requested, matching POI
			// HSLFSlideMaster.getPropHelper: minLevel = min(level, styles.size()-1).
			best := -1
			for l := range levels {
				if l <= reqLevel && l > best {
					best = l
				}
			}
			if best >= 0 {
				masterLevel, ok = levels[best], true
			} else {
				masterLevel, ok = levels[0]
			}
		}
		if !ok {
			continue
		}
		if !legacyPptMasterTextLevelSemanticallyValid(masterLevel) {
			fallback := legacyPptMasterTextLevelsWithBuiltInFallback(textType, nil)
			masterLevel, ok = fallback[reqLevel]
			if !ok {
				masterLevel, ok = fallback[0]
			}
			if !ok {
				continue
			}
		}
		inheritedOnly := paragraphLooksUnstyled(paragraph)
		// PT_MasterSubTitle / PT_SubTitle is a centered placeholder role. Local
		// alignment remains authoritative; only an omitted alignment inherits it.
		if shape.Placeholder == presentationir.PlaceholderSubtitle && paragraph.Align == "" && textType == pptTextTypeCenterBody {
			masterLevel.paragraph.Align = "ctr"
		}
		shape.Paragraphs[index] = mergeLegacyPptMasterParagraph(paragraph, masterLevel, inheritedOnly)
	}
}

func legacyPptMasterTextLevelsForType(styles legacyPptMasterTextStyles, textType int) map[int]legacyPptMasterTextLevel {
	if len(styles) == 0 {
		return legacyPptMasterTextLevelsWithBuiltInFallback(textType, nil)
	}
	var primary map[int]legacyPptMasterTextLevel
	if levels, ok := styles[textType]; ok && legacyPptMasterTextLevelsUsable(levels) {
		primary = levels
		if textType != pptTextTypeTitle || legacyPptMasterTextLevelsHaveRunStyling(levels) {
			resolved := legacyPptMasterTextLevelsWithRoleRunFallback(textType, legacyPptMasterTextLevelsWithBuiltInFallback(textType, levels))
			if textType == pptTextTypeBody {
				resolved = legacyPptMasterTextLevelsWithBodyCharacterFallback(resolved)
			}
			return resolved
		}
	}
	switch textType {
	case pptTextTypeTitle:
		if levels, ok := styles[pptTextTypeCenterTitle]; ok && legacyPptMasterTextLevelsUsable(levels) {
			if legacyPptMasterTextLevelsHaveRunStyling(levels) {
				return legacyPptMasterTextLevelsWithRoleRunFallback(textType, legacyPptMasterTextLevelsWithBuiltInFallback(textType, levels))
			}
			primary = levels
		}
		// Some binary masters contain only title paragraph properties and keep
		// the character defaults in the shared BODY/environment collection.
		// Preserve the title paragraph role but inherit only missing run fields.
		if body, ok := styles[pptTextTypeBody]; ok && legacyPptMasterTextLevelsHaveRunStyling(body) {
			return mergeLegacyPptTitleLevelsWithRunFallback(primary, body)
		}
	case pptTextTypeCenterBody:
		if levels, ok := styles[pptTextTypeBody]; ok && legacyPptMasterTextLevelsUsable(levels) {
			return legacyPptMasterTextLevelsWithRoleRunFallback(textType, levels)
		}
	case pptTextTypeCenterTitle:
		if levels, ok := styles[pptTextTypeTitle]; ok && legacyPptMasterTextLevelsUsable(levels) {
			return legacyPptMasterTextLevelsWithRoleRunFallback(textType, legacyPptMasterTextLevelsWithBuiltInFallback(textType, levels))
		}
	case pptTextTypeUnknown:
		// POI HSLFSlideMaster.getPropCollection maps null txtype → BODY fallback.
		// Shapes without an explicit TextHeaderAtom (textType = -1) should inherit
		// body master defaults rather than receiving no styling at all.
		if levels, ok := styles[pptTextTypeBody]; ok && legacyPptMasterTextLevelsUsable(levels) {
			return legacyPptMasterTextLevelsWithRoleRunFallback(textType, levels)
		}
	}
	return legacyPptMasterTextLevelsWithRoleRunFallback(textType, legacyPptMasterTextLevelsWithBuiltInFallback(textType, primary))
}

func legacyPptMasterTextLevelsWithBodyCharacterFallback(levels map[int]legacyPptMasterTextLevel) map[int]legacyPptMasterTextLevel {
	if len(levels) == 0 {
		return levels
	}
	base, ok := levels[0]
	if !ok {
		return levels
	}
	// In binary PPT a TextMasterStyleAtom can encode the BODY level-zero
	// character face/color once and leave deeper levels with only their own
	// metrics (most commonly font size). PowerPoint carries those invariant
	// character defaults into every level when saving the same master as OOXML.
	// Resolve that effective inheritance here so canonical IR never depends on
	// the renderer knowing the sparse legacy style grammar. Deliberately do not
	// copy FontSize or boolean flags: those remain level-specific properties.
	out := make(map[int]legacyPptMasterTextLevel, len(levels))
	for level, entry := range levels {
		if level > 0 {
			if entry.run.FontFamily == "" {
				entry.run.FontFamily = base.run.FontFamily
			}
			if entry.run.Color == "" {
				entry.run.Color = base.run.Color
			}
		}
		out[level] = entry
	}
	return out
}

func mergeLegacyPptTitleLevelsWithRunFallback(title, fallback map[int]legacyPptMasterTextLevel) map[int]legacyPptMasterTextLevel {
	if len(title) == 0 {
		title = map[int]legacyPptMasterTextLevel{0: {}}
	}
	out := make(map[int]legacyPptMasterTextLevel, len(title))
	for level, entry := range title {
		fallbackEntry, ok := fallback[level]
		if !ok {
			fallbackEntry = fallback[0]
		}
		entry.run = mergeLegacyPptMasterRunStyle(entry.run, fallbackEntry.run)
		out[level] = entry
	}
	return legacyPptMasterTextLevelsWithRoleRunFallback(pptTextTypeTitle, legacyPptMasterTextLevelsWithBuiltInFallback(pptTextTypeTitle, out))
}

func legacyPptMasterTextLevelsUsable(levels map[int]legacyPptMasterTextLevel) bool {
	if len(levels) == 0 {
		return false
	}
	for _, level := range levels {
		if !legacyPptMasterTextLevelSemanticallyValid(level) {
			continue
		}
		if !legacyPptMasterTextLevelEmpty(level) {
			return true
		}
	}
	return false
}

func legacyPptMasterTextLevelsHaveRunStyling(levels map[int]legacyPptMasterTextLevel) bool {
	for _, level := range levels {
		run := level.run
		// Explicit false boolean flags are useful cascade information, but they do
		// not constitute a usable visual character default by themselves.  A master
		// that says only "not underlined / not struck" still needs the built-in
		// PowerPoint role metrics when no decoded font metrics are available.
		if run.FontSize > 0 || run.FontFamily != "" || run.Color != "" ||
			(run.Bold != nil && *run.Bold) || (run.Italic != nil && *run.Italic) ||
			(run.Underline != nil && *run.Underline) || (run.Strike != nil && *run.Strike) {
			return true
		}
	}
	return false
}

func legacyPptMasterTextLevelsWithBuiltInFallback(textType int, levels map[int]legacyPptMasterTextLevel) map[int]legacyPptMasterTextLevel {
	builtIn, ok := legacyPptBuiltInMasterTextLevel(textType)
	if !ok {
		return levels
	}
	if legacyPptMasterTextLevelsHaveRunStyling(levels) {
		return levels
	}
	if len(levels) == 0 {
		return map[int]legacyPptMasterTextLevel{0: builtIn}
	}
	merged := make(map[int]legacyPptMasterTextLevel, len(levels))
	for level, entry := range levels {
		merged[level] = mergeLegacyPptMasterTextLevel(entry, builtIn)
	}
	return merged
}

func legacyPptMasterTextLevelsWithRoleRunFallback(textType int, levels map[int]legacyPptMasterTextLevel) map[int]legacyPptMasterTextLevel {
	if len(levels) == 0 {
		return levels
	}
	// Title roles have semantic PowerPoint metrics even when the decoded master
	// level is sparse (for example, it specifies only a typeface). Fill only
	// missing title run fields. BODY levels intentionally remain sparse: their
	// missing metrics are resolved by the document/master cascade, not invented
	// per master.
	if textType != pptTextTypeTitle && textType != pptTextTypeCenterTitle && legacyPptMasterTextLevelsHaveRunStyling(levels) {
		return levels
	}
	fallback, ok := legacyPptRoleRunFallback(textType)
	if !ok {
		return levels
	}
	merged := make(map[int]legacyPptMasterTextLevel, len(levels))
	for level, entry := range levels {
		entry.run = mergeLegacyPptMasterRunStyle(entry.run, fallback)
		merged[level] = entry
	}
	return merged
}

func legacyPptRoleRunFallback(textType int) (stpaCFRun, bool) {
	switch textType {
	case pptTextTypeTitle, pptTextTypeCenterTitle:
		return stpaCFRun{FontSize: 4400, FontFamily: "Arial", Color: "000000"}, true
	case pptTextTypeBody, pptTextTypeCenterBody, pptTextTypeUnknown:
		return stpaCFRun{FontSize: 3200, FontFamily: "Arial", Color: "000000"}, true
	case pptTextTypeOther:
		// Ordinary AutoShape/graphic text inherits PowerPoint's body-like 18pt
		// default when neither local nor master character properties author a
		// size. Keep the family unset so the document FontEntity/theme remains
		// authoritative.
		return stpaCFRun{FontSize: 1800, Color: "000000"}, true
	default:
		return stpaCFRun{}, false
	}
}

func legacyPptBuiltInMasterTextLevel(textType int) (legacyPptMasterTextLevel, bool) {
	// These are PowerPoint's role defaults, expressed in the legacy parser's
	// 1/100-point units. They are a last-resort semantic fallback only: decoded
	// document/master/local character properties remain authoritative.
	switch textType {
	case pptTextTypeTitle:
		return legacyPptMasterTextLevel{paragraph: stpaPFEntry{Align: "ctr"}}, true
	case pptTextTypeCenterTitle:
		return legacyPptMasterTextLevel{paragraph: stpaPFEntry{Align: "ctr"}}, true
	case pptTextTypeCenterBody:
		return legacyPptMasterTextLevel{paragraph: stpaPFEntry{Align: "ctr"}}, true
	case pptTextTypeBody:
		return legacyPptMasterTextLevel{}, true
	case pptTextTypeOther:
		return legacyPptMasterTextLevel{}, true
	default:
		return legacyPptMasterTextLevel{}, false
	}
}

func legacyPptParagraphLevel(paragraph *presentationir.Paragraph) int {
	if paragraph == nil || paragraph.Level < 0 {
		return 0
	}
	if paragraph.Level > 4 {
		return 4
	}
	return paragraph.Level
}

func paragraphLooksUnstyled(paragraph *presentationir.Paragraph) bool {
	if paragraph == nil {
		return false
	}
	if paragraph.Align != "" || paragraph.TextDirection != "" || paragraph.BulletSpecified || paragraph.BulletChar != "" || paragraph.AutoNum || paragraph.BulletColor != "" || paragraph.BulletSize > 0 || paragraph.HasSpaceBefore || paragraph.HasSpaceAfter || paragraph.HasLineSpacing || paragraph.HasMarginLeft || paragraph.HasTextIndent {
		return false
	}
	for _, run := range paragraph.Runs {
		if run == nil {
			continue
		}
		if run.BoldSet || run.ItalicSet || run.UnderlineSet || run.StrikeSet || run.FontSize > 0 || run.FontFamily != "" || run.Color != "" {
			return false
		}
	}
	return true
}

func mergeLegacyPptMasterParagraph(paragraph *presentationir.Paragraph, master legacyPptMasterTextLevel, inheritRunBooleans bool) *presentationir.Paragraph {
	if paragraph == nil {
		return nil
	}
	merged := cloneLegacyParagraph(paragraph)
	if merged.Align == "" {
		merged.Align = master.paragraph.Align
	}
	if merged.TextDirection == "" {
		merged.TextDirection = master.paragraph.TextDirection
	}
	if merged.BulletSpecified {
		// Binary PPT separates bullet enablement from the marker glyph. A local
		// fHasBullet=true can intentionally omit bulletChar, in which case the
		// marker inherits from the master even when that master has bullets
		// disabled at this level. If neither level authors a glyph, PowerPoint's
		// canonical fallback is U+2022.
		if !merged.BulletNone && !merged.AutoNum && merged.BulletChar == "" {
			merged.BulletChar = bulletCharString(master.paragraph.BulletChar)
			if merged.BulletChar == "" {
				merged.BulletChar = "•"
			}
		}
	} else if !merged.BulletNone && merged.BulletChar == "" {
		if master.paragraph.HasBullet {
			merged.BulletChar = bulletCharString(master.paragraph.BulletChar)
			if merged.BulletChar == "" {
				merged.BulletChar = "•"
			}
		} else if master.paragraph.HasBulletSpecified {
			// Explicit fHasBullet=false is semantic inheritance, not an absent
			// bullet character. Preserve it so a cached master bulletChar cannot
			// leak into the resolved paragraph.
			merged.BulletNone = true
		} else if legacyPptMasterParagraphHasImplicitBulletGlyph(master.paragraph) {
			// TextMasterStyleAtom may author the marker glyph and list geometry for
			// a deeper level without repeating fHasBullet. That is an inherited list
			// marker, not a stale cached glyph. Require list geometry as provenance
			// so isolated cached bulletChar values still cannot manufacture bullets.
			merged.BulletChar = bulletCharString(master.paragraph.BulletChar)
		}
	}
	if !merged.BulletNone && !merged.AutoNum && merged.BulletChar == "" && master.paragraph.AutoNum {
		merged.AutoNum = true
		merged.AutoNumType = master.paragraph.AutoNumType
		merged.AutoNumStart = master.paragraph.AutoNumStart
	}
	if merged.BulletFontFamily == "" && !merged.BulletFontTx && master.paragraph.BulletFont != "" {
		merged.BulletFontFamily = master.paragraph.BulletFont
	}
	if merged.BulletColor == "" && master.paragraph.HasBulletColor {
		merged.BulletColor = master.paragraph.BulletColor
		merged.BulletColorExplicit = true
	}
	if merged.BulletSize == 0 && master.paragraph.BulletSize > 0 {
		merged.BulletSize = master.paragraph.BulletSize
	}
	if !merged.HasSpaceBefore {
		if master.paragraph.HasSpaceBefore {
			merged.SpaceBefore = master.paragraph.SpaceBefore
			merged.HasSpaceBefore = true
		} else if master.paragraph.SpaceBeforePct > 0 {
			merged.SpaceBeforePct = master.paragraph.SpaceBeforePct
			merged.HasSpaceBefore = true
		}
	}
	if !merged.HasSpaceAfter {
		if master.paragraph.HasSpaceAfter {
			merged.SpaceAfter = master.paragraph.SpaceAfter
			merged.HasSpaceAfter = true
		} else if master.paragraph.SpaceAfterPct > 0 {
			merged.SpaceAfterPct = master.paragraph.SpaceAfterPct
			merged.HasSpaceAfter = true
		}
	}
	if !merged.HasLineSpacing && master.paragraph.HasLineSpacing {
		merged.LineSpacingPct = master.paragraph.LineSpacingPct
		merged.LineSpacingPt = master.paragraph.LineSpacingPt
		merged.HasLineSpacing = true
	}
	if !merged.HasMarginRight && master.paragraph.HasMarginRight {
		merged.MarginRightEMU = master.paragraph.MarginRightEMU
		merged.HasMarginRight = true
	}
	if merged.DefaultTabSizeEMU == 0 && master.paragraph.HasDefaultTabSize {
		merged.DefaultTabSizeEMU = master.paragraph.DefaultTabSizeEMU
	}
	if len(merged.TabStops) == 0 && len(master.paragraph.TabStops) > 0 {
		merged.TabStops = append([]presentationir.TabStop(nil), master.paragraph.TabStops...)
	}
	if merged.FontAlign == "" && master.paragraph.FontAlign != "" {
		merged.FontAlign = master.paragraph.FontAlign
	}
	if !merged.HasMarginLeft && master.paragraph.HasMarginLeft {
		merged.MarginLeftEMU = master.paragraph.MarginLeftEMU
		merged.HasMarginLeft = true
	}
	if !merged.HasTextIndent && master.paragraph.HasTextIndent {
		merged.TextIndentEMU = master.paragraph.TextIndentEMU
		merged.HasTextIndent = true
	}
	for index, run := range merged.Runs {
		merged.Runs[index] = mergeLegacyPptMasterRun(run, master.run, inheritRunBooleans)
	}
	// PPT stores text.offset (left margin) and bullet.offset as absolute positions
	// from the text box edge. Canonical IR follows DrawingML, where indent is
	// relative to marL. Resolve that representation only after local/master
	// paragraph properties have been merged so either side may supply a value.
	if merged.HasMarginLeft && merged.HasTextIndent {
		merged.TextIndentEMU -= merged.MarginLeftEMU
	}
	// When neither the paragraph nor its master authored a bullet color, binary
	// PowerPoint uses the effective text color for the marker. Materialize that
	// resolved color here so renderers do not fall back to black.
	if !merged.BulletNone && (merged.BulletChar != "" || merged.AutoNum) && !merged.BulletColorExplicit && merged.BulletColor == "" {
		for _, run := range merged.Runs {
			if run != nil && run.Color != "" {
				merged.BulletColor = run.Color
				break
			}
		}
	}
	return merged
}

func mergeLegacyPptMasterRun(run *presentationir.Run, master stpaCFRun, inheritBooleans bool) *presentationir.Run {
	if run == nil {
		return nil
	}
	merged := *run
	// Inherit booleans from master when the run did not have an explicit STPA mask bit
	// for that attribute (*Set == false), regardless of whether inheritBooleans is set.
	// inheritBooleans=true means the run has no styling at all (full unstyled paragraph);
	// even with inheritBooleans=false we still fill unset boolean slots.
	if master.Bold != nil && (!merged.BoldSet || inheritBooleans) {
		merged.Bold = *master.Bold
		merged.BoldSet = true
	}
	if master.Italic != nil && (!merged.ItalicSet || inheritBooleans) {
		merged.Italic = *master.Italic
		merged.ItalicSet = true
	}
	if master.Underline != nil && (!merged.UnderlineSet || inheritBooleans) {
		merged.Underline = *master.Underline
		merged.UnderlineSet = true
	}
	if master.Strike != nil && (!merged.StrikeSet || inheritBooleans) {
		merged.Strike = *master.Strike
		merged.StrikeSet = true
	}
	_ = inheritBooleans // consumed above
	if merged.FontSize == 0 && master.FontSize > 0 {
		merged.FontSize = master.FontSize
	}
	if merged.FontFamily == "" {
		merged.FontFamily = master.FontFamily
	}
	if merged.Color == "" {
		merged.Color = master.Color
	}
	return &merged
}

func cloneLegacyParagraph(paragraph *presentationir.Paragraph) *presentationir.Paragraph {
	if paragraph == nil {
		return nil
	}
	clone := *paragraph
	if len(paragraph.Runs) > 0 {
		clone.Runs = make([]*presentationir.Run, 0, len(paragraph.Runs))
		for _, run := range paragraph.Runs {
			if run == nil {
				clone.Runs = append(clone.Runs, nil)
				continue
			}
			runClone := *run
			clone.Runs = append(clone.Runs, &runClone)
		}
	}
	return &clone
}

func legacyPptMasterBackgroundColor(theme *presentationir.Theme) string {
	if theme == nil || len(theme.ColorScheme) == 0 {
		return ""
	}
	if color := theme.ColorScheme["lt1"]; color != "" {
		return color
	}
	return ""
}

func splitLegacyExtraShapes(shapes []*presentationir.Shape) (textShapes, nonTextShapes []*presentationir.Shape) {
	if len(shapes) == 0 {
		return nil, nil
	}
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if shape.Kind == presentationir.ShapeText {
			textShapes = append(textShapes, shape)
			continue
		}
		nonTextShapes = append(nonTextShapes, shape)
	}
	return textShapes, nonTextShapes
}

func cloneLegacyExtraShapes(slideIndex, startShapeID int, shapes []*presentationir.Shape) []*presentationir.Shape {
	if len(shapes) == 0 {
		return nil
	}
	out := make([]*presentationir.Shape, 0, len(shapes))
	nextID := slideIndex*10 + startShapeID
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if clone := cloneLegacyShapeRecursive(shape, &nextID); clone != nil {
			out = append(out, clone)
		}
	}
	return out
}

func cloneLegacyShapeRecursive(shape *presentationir.Shape, nextID *int) *presentationir.Shape {
	if shape == nil {
		return nil
	}
	clone := *shape
	// Master and extra shapes are semantic templates. Deep-clone the shared
	// OfficeArt graph as well as presentation-owned mutable slices so layout,
	// composition, or later fidelity passes cannot mutate a sibling slide through
	// an apparently independent presentation Shape.
	clone.OfficeArt = cloneLegacyOfficeArtShape(shape.OfficeArt)
	// BIG39 final closure: every mutable presentation-side value reachable from a
	// master template is slide-local, just like the shared OfficeArt graph.
	if shape.PictureTile != nil {
		tile := *shape.PictureTile
		clone.PictureTile = &tile
	}
	if shape.EffectOverlay != nil {
		overlay := *shape.EffectOverlay
		clone.EffectOverlay = &overlay
	}
	if shape.GeometryData != nil {
		geometry := *shape.GeometryData
		geometry.Guides = append([]drawing.Guide(nil), shape.GeometryData.Guides...)
		geometry.Handles = append([]drawing.AdjustHandle(nil), shape.GeometryData.Handles...)
		if len(shape.GeometryData.Paths) != 0 {
			geometry.Paths = make([]drawing.Path, len(shape.GeometryData.Paths))
			for i, sourcePath := range shape.GeometryData.Paths {
				geometry.Paths[i] = sourcePath
				if len(sourcePath.Commands) != 0 {
					geometry.Paths[i].Commands = make([]drawing.PathCommand, len(sourcePath.Commands))
					for j, sourceCommand := range sourcePath.Commands {
						geometry.Paths[i].Commands[j] = sourceCommand
						geometry.Paths[i].Commands[j].Points = append([]drawing.Point(nil), sourceCommand.Points...)
					}
				}
			}
		}
		clone.GeometryData = &geometry
	}
	if shape.GroupTransform != nil {
		transform := *shape.GroupTransform
		clone.GroupTransform = &transform
	}
	if shape.LineStyle != nil {
		line := *shape.LineStyle
		line.CompoundWidths = append([]float64(nil), shape.LineStyle.CompoundWidths...)
		if shape.LineStyle.HeadEnd != nil {
			head := *shape.LineStyle.HeadEnd
			line.HeadEnd = &head
		}
		if shape.LineStyle.TailEnd != nil {
			tail := *shape.LineStyle.TailEnd
			line.TailEnd = &tail
		}
		clone.LineStyle = &line
	}
	clone.ChartData = append([]byte(nil), shape.ChartData...)
	clone.ImageEffects = append([]drawing.Effect(nil), shape.ImageEffects...)
	clone.GeometryAdjustments = append([]float64(nil), shape.GeometryAdjustments...)
	clone.ConnectorAdjustments = append([]int(nil), shape.ConnectorAdjustments...)
	clone.ConnectorPath = append([]presentationir.Point(nil), shape.ConnectorPath...)
	clone.AnimationTriggers = append([]string(nil), shape.AnimationTriggers...)
	clone.EmphasisEffects = append([]string(nil), shape.EmphasisEffects...)
	if shape.ConnectorStart != nil {
		point := *shape.ConnectorStart
		clone.ConnectorStart = &point
	}
	if shape.ConnectorEnd != nil {
		point := *shape.ConnectorEnd
		clone.ConnectorEnd = &point
	}
	if nextID != nil {
		clone.ID = *nextID
		*nextID++
	}
	if len(shape.ImageData) > 0 {
		clone.ImageData = append([]byte(nil), shape.ImageData...)
	}
	if len(shape.MediaData) > 0 {
		clone.MediaData = append([]byte(nil), shape.MediaData...)
	}
	if len(shape.MediaPosterData) > 0 {
		clone.MediaPosterData = append([]byte(nil), shape.MediaPosterData...)
	}
	if len(shape.OLEObjectData) > 0 {
		clone.OLEObjectData = append([]byte(nil), shape.OLEObjectData...)
	}
	if len(shape.OLEIconData) > 0 {
		clone.OLEIconData = append([]byte(nil), shape.OLEIconData...)
	}
	if len(shape.Paragraphs) > 0 {
		clone.Paragraphs = make([]*presentationir.Paragraph, 0, len(shape.Paragraphs))
		for _, paragraph := range shape.Paragraphs {
			if paragraph == nil {
				clone.Paragraphs = append(clone.Paragraphs, nil)
				continue
			}
			paragraphClone := *paragraph
			paragraphClone.TabStops = append([]presentationir.TabStop(nil), paragraph.TabStops...)
			if len(paragraph.Runs) != 0 {
				paragraphClone.Runs = make([]*presentationir.Run, 0, len(paragraph.Runs))
				for _, run := range paragraph.Runs {
					if run == nil {
						paragraphClone.Runs = append(paragraphClone.Runs, nil)
						continue
					}
					runClone := *run
					paragraphClone.Runs = append(paragraphClone.Runs, &runClone)
				}
			}
			clone.Paragraphs = append(clone.Paragraphs, &paragraphClone)
		}
	}
	if len(shape.Children) > 0 {
		clone.Children = make([]*presentationir.Shape, 0, len(shape.Children))
		for _, child := range shape.Children {
			if childClone := cloneLegacyShapeRecursive(child, nextID); childClone != nil {
				clone.Children = append(clone.Children, childClone)
			}
		}
	}
	return &clone
}

func cloneLegacyOfficeArtShape(shape *officeartir.Shape) *officeartir.Shape {
	if shape == nil {
		return nil
	}
	clone := *shape
	clone.Geometry.Adjustments = append([]float64(nil), shape.Geometry.Adjustments...)
	if len(shape.Geometry.Paths) > 0 {
		clone.Geometry.Paths = make([]officeartir.Path, len(shape.Geometry.Paths))
		for i, path := range shape.Geometry.Paths {
			clone.Geometry.Paths[i] = path
			if len(path.Commands) > 0 {
				clone.Geometry.Paths[i].Commands = make([]officeartir.PathCommand, len(path.Commands))
				for j, command := range path.Commands {
					clone.Geometry.Paths[i].Commands[j] = command
					clone.Geometry.Paths[i].Commands[j].Points = append([]officeartir.PointF(nil), command.Points...)
				}
			}
		}
	}
	if shape.Fill.Gradient != nil {
		gradient := *shape.Fill.Gradient
		gradient.Stops = append([]officeartir.GradientStop(nil), shape.Fill.Gradient.Stops...)
		clone.Fill.Gradient = &gradient
	}
	if shape.Fill.Pattern != nil {
		pattern := *shape.Fill.Pattern
		clone.Fill.Pattern = &pattern
	}
	if shape.Fill.Picture != nil {
		picture := *shape.Fill.Picture
		picture.Image.Data = append([]byte(nil), shape.Fill.Picture.Image.Data...)
		if shape.Fill.Picture.Tile != nil {
			tile := *shape.Fill.Picture.Tile
			picture.Tile = &tile
		}
		clone.Fill.Picture = &picture
	}
	if shape.Effects.Shadow != nil {
		shadow := *shape.Effects.Shadow
		clone.Effects.Shadow = &shadow
	}
	if shape.Effects.Glow != nil {
		glow := *shape.Effects.Glow
		clone.Effects.Glow = &glow
	}
	if shape.Effects.Blur != nil {
		blur := *shape.Effects.Blur
		clone.Effects.Blur = &blur
	}
	if shape.Text != nil {
		text := *shape.Text
		if len(shape.Text.Paragraphs) > 0 {
			text.Paragraphs = make([]officeartir.Paragraph, len(shape.Text.Paragraphs))
			for i, paragraph := range shape.Text.Paragraphs {
				text.Paragraphs[i] = paragraph
				text.Paragraphs[i].Runs = append([]officeartir.TextRun(nil), paragraph.Runs...)
			}
		}
		clone.Text = &text
	}
	if shape.GeometryText != nil {
		geometryText := *shape.GeometryText
		clone.GeometryText = &geometryText
	}
	if shape.Image != nil {
		image := *shape.Image
		image.Data = append([]byte(nil), shape.Image.Data...)
		clone.Image = &image
	}
	if shape.Connector != nil {
		connector := *shape.Connector
		if shape.Connector.Start != nil {
			point := *shape.Connector.Start
			connector.Start = &point
		}
		if shape.Connector.End != nil {
			point := *shape.Connector.End
			connector.End = &point
		}
		connector.Path = append([]officeartir.Point(nil), shape.Connector.Path...)
		connector.Adjustments = append([]float64(nil), shape.Connector.Adjustments...)
		clone.Connector = &connector
	}
	if len(shape.Children) > 0 {
		clone.Children = make([]*officeartir.Shape, 0, len(shape.Children))
		for _, child := range shape.Children {
			clone.Children = append(clone.Children, cloneLegacyOfficeArtShape(child))
		}
	}
	return &clone
}

// normalizeLegacyPptAnimatedEntrancePaint removes a legacy animation-state leak
// from effective slide paint. Binary PPT can persist lineOpacity=0 on descendants
// of a group whose click animation is an entrance/cut. That zero is the pre-entry
// presentation state, not the authored final stroke paint. AnimationTimeline owns
// visibility in canonical IR, so carrying the zero into LineStyle makes the child
// remain invisible even after its group has entered. PowerPoint's PPTX save drops
// these zero-opacity caches and its static/PDF rendering shows the strokes opaque.
//
// Keep explicitly transparent strokes everywhere else. The normalization is scoped
// to descendants of an entrance/cut animation target, where the timeline already
// owns the hidden/visible state.
func normalizeLegacyPptAnimatedEntrancePaint(shapes []*presentationir.Shape) {
	var walk func(*presentationir.Shape, bool)
	walk = func(shape *presentationir.Shape, insideEntrance bool) {
		if shape == nil {
			return
		}
		entrance := insideEntrance || legacyPptShapeHasCutEntrance(shape)
		// Old PPT also leaves this pre-entry alpha cache on some standalone
		// freeform arrows without attaching the animation record to that leaf.
		// A visible OfficeArt line with an authored line-end marker is final paint,
		// not a permanently invisible connector. Keep ordinary transparent lines
		// untouched; the marker is the extra semantic signal that this is the same
		// stale presentation-state cache seen under cut-entrance groups.
		lineEndCache := legacyPptShapeHasVisibleLineEndCache(shape)
		if (entrance || lineEndCache) && shape.LineStyle != nil && shape.LineStyle.Alpha == 0 && shape.LineColor != "" {
			shape.LineStyle.Alpha = 1
			if shape.OfficeArt != nil && shape.OfficeArt.Line.Visible && shape.OfficeArt.Line.Alpha == 0 {
				shape.OfficeArt.Line.Alpha = 1
			}
		}
		for _, child := range shape.Children {
			walk(child, entrance)
		}
	}
	for _, shape := range shapes {
		walk(shape, false)
	}
}

func legacyPptShapeHasVisibleLineEndCache(shape *presentationir.Shape) bool {
	if shape == nil || shape.OfficeArt == nil || !shape.OfficeArt.Line.Visible || shape.OfficeArt.Line.Alpha != 0 {
		return false
	}
	hasMarker := func(v string) bool {
		v = strings.TrimSpace(strings.ToLower(v))
		return v != "" && v != "none"
	}
	return hasMarker(shape.ArrowHead) || hasMarker(shape.ArrowTail)
}

func legacyPptShapeHasCutEntrance(shape *presentationir.Shape) bool {
	if shape == nil || shape.AnimStep <= 0 {
		return false
	}
	for _, effect := range shape.EmphasisEffects {
		if strings.EqualFold(strings.TrimSpace(effect), "cut") {
			return true
		}
	}
	return false
}

// markLegacyPptAnimationStateOverlays identifies binary-PPT animation paint-state
// duplicates. Some old decks persist an on-click cut group containing a second copy
// of already-visible text plus highlight geometry. That subtree is playback state,
// not static slide content; painting it in document HTML produces duplicated text,
// selection rectangles and highlight dots (for example legacy emphasis builds).
func markLegacyPptAnimationStateOverlays(shapes []*presentationir.Shape) {
	for _, candidate := range shapes {
		if candidate == nil || !legacyPptShapeHasCutEntrance(candidate) {
			continue
		}
		texts := legacyPptNonEmptyTextDescendants(candidate)
		if len(texts) == 0 {
			continue
		}
		matched := false
		for _, animated := range texts {
			animatedText := legacyPptNormalizedShapeText(animated)
			if animatedText == "" {
				continue
			}
			for _, peer := range shapes {
				if peer == nil || peer == candidate || peer.AnimStep > 0 || peer.Kind != presentationir.ShapeText {
					continue
				}
				if legacyPptNormalizedShapeText(peer) != animatedText {
					continue
				}
				if legacyPptAnimationOverlayBoundsMatch(candidate.Bounds, peer.Bounds) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			continue
		}
		if !legacyPptAnimationOverlayHasUniqueStaticPaint(candidate) {
			legacyPptMarkAnimationOnly(candidate)
			continue
		}
		// A persisted cut state can contain a full duplicate of an already-visible
		// text box plus unique static paint (for example a yellow selection frame).
		// Static PowerPoint output shows the *paint-state text* once, not the base
		// text and state text simultaneously. Fold the authored state paragraphs
		// into the matching base text shape and remove only the duplicate text
		// descendant from the state group; unique frame/dot/callout geometry stays.
		legacyPptFoldStaticCutTextState(candidate, shapes)
	}
}

// legacyPptAnimationOverlayHasUniqueStaticPaint distinguishes a pure duplicate
// animation-state cache from an authored highlighted/static state. Some legacy
// cut groups contain only a second copy of already-visible text and can safely be
// timeline-only. Others add visible geometry (selection/highlight frames, dots,
// callouts, etc.). PowerPoint paints that geometry in its static/PDF output, so
// suppressing the entire group loses authored slide appearance.

func legacyPptFoldStaticCutTextState(candidate *presentationir.Shape, siblings []*presentationir.Shape) {
	if candidate == nil {
		return
	}
	for _, animated := range legacyPptNonEmptyTextDescendants(candidate) {
		animatedText := legacyPptNormalizedShapeText(animated)
		if animatedText == "" {
			continue
		}
		for _, peer := range siblings {
			if peer == nil || peer == candidate || peer.AnimStep > 0 || peer.Kind != presentationir.ShapeText {
				continue
			}
			if legacyPptNormalizedShapeText(peer) != animatedText {
				continue
			}
			if !legacyPptAnimationOverlayBoundsMatch(candidate.Bounds, peer.Bounds) {
				continue
			}
			// Deduplicate any persisted ellipse marker while the animated text still
			// shares the candidate's local coordinate space. Once the text state is
			// folded into the top-level peer, the ellipse's local bounds can no longer
			// be compared reliably with that peer.
			candidate.Children = presentationir.NormalizeDuplicateBulletGlyphShapesInList(candidate.Children)
			peer.Paragraphs = cloneLegacyParagraphsDeep(animated.Paragraphs)
			// Preserve the authored animation-state text node in canonical IR. Static
			// rendering suppresses AnimationOnly descendants, while playback can still
			// materialize the complete persisted state. Removing the node here made PPT
			// structurally diverge from the equivalent PPTX and discarded animation
			// semantics that belong in IR rather than parser-side paint flattening.
			legacyPptMarkAnimationOnly(animated)
			return
		}
	}
}

func cloneLegacyParagraphsDeep(paragraphs []*presentationir.Paragraph) []*presentationir.Paragraph {
	if len(paragraphs) == 0 {
		return nil
	}
	out := make([]*presentationir.Paragraph, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		out = append(out, cloneLegacyParagraph(paragraph))
	}
	return out
}

func legacyPptRemoveShapeDescendant(root, target *presentationir.Shape) bool {
	if root == nil || target == nil {
		return false
	}
	for i, child := range root.Children {
		if child == target {
			root.Children = append(root.Children[:i], root.Children[i+1:]...)
			return true
		}
		if legacyPptRemoveShapeDescendant(child, target) {
			return true
		}
	}
	return false
}

func legacyPptAnimationOverlayHasUniqueStaticPaint(shape *presentationir.Shape) bool {
	var visible bool
	var walk func(*presentationir.Shape, bool)
	walk = func(current *presentationir.Shape, root bool) {
		if current == nil || visible {
			return
		}
		// The root group itself is only a composition node. Descendant geometry
		// with visible fill/stroke is the semantic signal that this state adds
		// authored static paint beyond duplicated text.
		if !root {
			if current.Kind != presentationir.ShapeText && current.Kind != presentationir.ShapeGroup {
				if current.FillColor != "" || current.LineColor != "" {
					visible = true
					return
				}
			}
			if current.OfficeArt != nil {
				fillVisible := current.OfficeArt.Fill.Kind != officeartir.FillUnset &&
					current.OfficeArt.Fill.Kind != officeartir.FillNone && current.OfficeArt.Fill.Alpha != 0
				lineVisible := current.OfficeArt.Line.Visible && current.OfficeArt.Line.Alpha != 0
				if fillVisible || lineVisible {
					visible = true
					return
				}
			}
		}
		for _, child := range current.Children {
			walk(child, false)
		}
	}
	walk(shape, true)
	return visible
}

func legacyPptNonEmptyTextDescendants(shape *presentationir.Shape) []*presentationir.Shape {
	var out []*presentationir.Shape
	var walk func(*presentationir.Shape)
	walk = func(current *presentationir.Shape) {
		if current == nil {
			return
		}
		if current.Kind == presentationir.ShapeText && legacyPptNormalizedShapeText(current) != "" {
			out = append(out, current)
		}
		for _, child := range current.Children {
			walk(child)
		}
	}
	walk(shape)
	return out
}

func legacyPptNormalizedShapeText(shape *presentationir.Shape) string {
	if shape == nil {
		return ""
	}
	var b strings.Builder
	for _, paragraph := range shape.Paragraphs {
		if paragraph == nil {
			continue
		}
		for _, run := range paragraph.Runs {
			if run != nil {
				b.WriteString(run.Text)
			}
		}
		b.WriteByte('\n')
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func legacyPptAnimationOverlayBoundsMatch(group, peer presentationir.Rect) bool {
	if group.W <= 0 || group.H <= 0 || peer.W <= 0 || peer.H <= 0 {
		return false
	}
	abs64 := func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	}
	// Animation-state groups often add a narrow marker gutter, so compare with a
	// modest tolerance rather than requiring byte-identical anchors.
	return abs64(group.Y-peer.Y) <= maxInt64(group.H, peer.H)/50 &&
		abs64(group.H-peer.H) <= maxInt64(group.H, peer.H)/50 &&
		abs64(group.X-peer.X) <= maxInt64(group.W, peer.W)/20 &&
		abs64(group.W-peer.W) <= maxInt64(group.W, peer.W)/20
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func legacyPptMarkAnimationOnly(shape *presentationir.Shape) {
	if shape == nil {
		return
	}
	shape.AnimationOnly = true
	for _, child := range shape.Children {
		legacyPptMarkAnimationOnly(child)
	}
}

func buildLegacySlideAnimationTimeline(header *presentationir.Shape, shapes []*presentationir.Shape, footer *presentationir.Shape, dateTime *presentationir.Shape, slideNumber *presentationir.Shape) []presentationir.AnimationEvent {
	var events []presentationir.AnimationEvent
	var walk func(*presentationir.Shape)
	walk = func(shape *presentationir.Shape) {
		if shape == nil {
			return
		}
		trigger := ""
		if len(shape.AnimationTriggers) > 0 {
			trigger = shape.AnimationTriggers[0]
		}
		effect := ""
		if len(shape.EmphasisEffects) > 0 {
			effect = shape.EmphasisEffects[0]
		}
		if shape.AnimStep > 0 || trigger != "" || effect != "" {
			events = append(events, presentationir.AnimationEvent{
				ShapeID:  shape.ID,
				Step:     shape.AnimStep,
				NodeType: "shape",
				Behavior: effect,
				Effect:   effect,
				Trigger:  trigger,
				DelayMs:  shape.AnimationDelayMs,
			})
		}
		for _, child := range shape.Children {
			walk(child)
		}
	}
	walk(header)
	for _, shape := range shapes {
		walk(shape)
	}
	walk(footer)
	walk(dateTime)
	walk(slideNumber)
	if len(events) == 0 {
		return nil
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Step == events[j].Step {
			return events[i].ShapeID < events[j].ShapeID
		}
		if events[i].Step == 0 {
			return false
		}
		if events[j].Step == 0 {
			return true
		}
		return events[i].Step < events[j].Step
	})
	return events
}

// extractLegacyNotesParagraphs collects paragraphs from all pptTextTypeNotes
// fragments in a slide text record, filtering out placeholder prompt text
// (e.g. "Click to add notes") that PowerPoint stores as default content.
func extractLegacyNotesParagraphs(slideText legacySlideText) []*presentationir.Paragraph {
	var paras []*presentationir.Paragraph
	for _, frag := range slideText.Fragments {
		if frag.TextType != pptTextTypeNotes {
			continue
		}
		if legacyPptLooksLikePlaceholderSampleText(frag.Text) {
			continue
		}
		if len(frag.Paragraphs) > 0 {
			paras = append(paras, frag.Paragraphs...)
		} else if frag.Text != "" {
			paras = append(paras, fragmentParagraphs(frag)...)
		}
	}
	return paras
}

func attachNotesHeaderFooterInfoByPersistID(slides []legacySlideText, infos map[int]legacyPptHeaderFooterInfo, defaults legacyPptHeaderFooterInfo) []legacySlideText {
	for index := range slides {
		info := defaults
		if slideInfo, ok := infos[slides[index].NotesPersistID]; ok {
			info = info.mergedWith(slideInfo)
		}
		slides[index].NotesHeaderFooter = info
	}
	return slides
}

func buildLegacyNotesShapes(slideIndex int, paragraphs []*presentationir.Paragraph, info legacyPptHeaderFooterInfo) []*presentationir.Shape {
	shapes := make([]*presentationir.Shape, 0, 5)
	nextShapeID := 1
	if info.headerEnabled() && info.headerText != "" {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextShapeID, "hdr", presentationir.PlaceholderOther, legacyHeaderBounds(), buildTextParagraphs(info.headerText)))
		nextShapeID++
	}
	if len(paragraphs) > 0 {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Notes %d", slideIndex+1), presentationir.PlaceholderBody, legacyBodyBounds(false), paragraphs))
		nextShapeID++
	}
	if info.footerEnabled() && info.footerText != "" {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Notes Footer %d", slideIndex+1), presentationir.PlaceholderFooter, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderFooter), buildTextParagraphs(info.footerText)))
		nextShapeID++
	}
	if info.dateEnabled() {
		if info.dateText != "" {
			shapes = append(shapes, newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Notes Date %d", slideIndex+1), presentationir.PlaceholderDateTime, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime), buildTextParagraphs(info.dateText)))
			nextShapeID++
		} else if info.flags&legacyPptHFFlagTodayDate != 0 {
			shapes = append(shapes, newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Notes Date %d", slideIndex+1), presentationir.PlaceholderDateTime, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime), []*presentationir.Paragraph{{Runs: []*presentationir.Run{{FieldKind: "date"}}}}))
			nextShapeID++
		}
	}
	if info.slideNumberEnabled() {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Notes Slide Number %d", slideIndex+1), presentationir.PlaceholderSlideNumber, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderSlideNumber), []*presentationir.Paragraph{{Runs: []*presentationir.Run{{FieldKind: "slidenum"}}}}))
	}
	return shapes
}

func splitLegacySlideShapes(shapes []*presentationir.Shape) (header *presentationir.Shape, content []*presentationir.Shape, footer, dateTime, slideNumber *presentationir.Shape) {
	content = make([]*presentationir.Shape, 0, len(shapes))
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		if shape.Placeholder == presentationir.PlaceholderOther && shape.Name == "hdr" {
			if header == nil {
				header = shape
				continue
			}
		}
		switch shape.Placeholder {
		case presentationir.PlaceholderFooter:
			if footer == nil {
				footer = shape
				continue
			}
		case presentationir.PlaceholderDateTime:
			if dateTime == nil {
				dateTime = shape
				continue
			}
		case presentationir.PlaceholderSlideNumber:
			if slideNumber == nil {
				slideNumber = shape
				continue
			}
		}
		content = append(content, shape)
	}
	return header, content, footer, dateTime, slideNumber
}

func buildLegacySlideShapes(slideIndex int, slideText legacySlideText, filename string) []*presentationir.Shape {
	// When TextHeaderAtom type info is available, group fragments by role directly.
	// Fall back to heuristic partitioning when no type info is present.
	if hasTypedFragments(slideText.Fragments) {
		return buildLegacySlideShapesTyped(slideIndex, slideText, filename)
	}
	paragraphs := buildTextParagraphs(joinSlideFragments(slideText.Fragments))
	if len(paragraphs) == 0 {
		paragraphs = []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: filename}}}}
	}
	title, subtitle, body, footer := partitionLegacySlideParagraphs(paragraphs, filename)
	if len(title) == 0 && len(subtitle) == 0 {
		shapes := []*presentationir.Shape{newLegacySlideShape(slideIndex, 1, fmt.Sprintf("Legacy Text %d", slideIndex+1), presentationir.PlaceholderBody, legacyBodyBounds(false), body)}
		if footerShapes := buildLegacyFooterShapes(slideIndex, 2, footer); len(footerShapes) > 0 {
			shapes = append(shapes, footerShapes...)
		}
		return shapes
	}
	if legacyPptLooksLikePlaceholderPromptParagraphs(title) {
		title = nil
	}
	if legacyPptLooksLikePlaceholderPromptParagraphs(subtitle) {
		subtitle = nil
	}
	if legacyPptLooksLikePlaceholderPromptParagraphs(body) {
		body = nil
	}
	if legacyPptLooksLikePlaceholderPromptParagraphs(footer) {
		footer = nil
	}
	shapes := make([]*presentationir.Shape, 0, 2)
	nextID := 1
	if len(title) > 0 {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextID, fmt.Sprintf("Legacy Title %d", slideIndex+1), presentationir.PlaceholderTitle, legacyTitleBounds(), title))
		nextID++
	}
	if len(subtitle) > 0 {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextID, fmt.Sprintf("Legacy Subtitle %d", slideIndex+1), presentationir.PlaceholderSubtitle, legacySubtitleBounds(), subtitle))
		nextID++
	}
	if len(body) > 0 {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextID, fmt.Sprintf("Legacy Body %d", slideIndex+1), presentationir.PlaceholderBody, legacyBodyBounds(len(title) > 0), body))
		nextID++
	}
	if footerShapes := buildLegacyFooterShapes(slideIndex, nextID, footer); len(footerShapes) > 0 {
		shapes = append(shapes, footerShapes...)
	}
	return shapes
}

func newLegacySlideShape(slideIndex, shapeID int, name string, placeholder presentationir.PlaceholderKind, bounds presentationir.Rect, paragraphs []*presentationir.Paragraph) *presentationir.Shape {
	return &presentationir.Shape{
		ID:          slideIndex*10 + shapeID,
		Name:        name,
		Kind:        presentationir.ShapeText,
		Placeholder: placeholder,
		Bounds:      bounds,
		Paragraphs:  paragraphs,
	}
}

func partitionLegacySlideParagraphs(paragraphs []*presentationir.Paragraph, fallbackName string) (title, subtitle, body, footer []*presentationir.Paragraph) {
	if len(paragraphs) == 0 {
		return nil, nil, nil, nil
	}
	paragraphs, footer = splitLegacyFooterParagraphs(paragraphs)
	if len(paragraphs) == 0 {
		return nil, nil, nil, footer
	}
	if len(paragraphs) < 2 {
		if isLikelyLegacySlideTitle(paragraphText(paragraphs[0]), fallbackName) {
			return paragraphs, nil, nil, footer
		}
		return nil, nil, paragraphs, footer
	}
	firstText := paragraphText(paragraphs[0])
	if !isLikelyLegacySlideTitle(firstText, fallbackName) {
		return nil, nil, paragraphs, footer
	}
	title = []*presentationir.Paragraph{paragraphs[0]}
	remaining := paragraphs[1:]
	if len(remaining) == 1 && isLikelyLegacySubtitle(paragraphText(remaining[0])) {
		return title, remaining, nil, footer
	}
	if len(remaining) > 1 && isLikelyLegacySubtitle(paragraphText(remaining[0])) && looksLikeLegacyBulletParagraphs(remaining[1:]) {
		return title, remaining[:1], remaining[1:], footer
	}
	return title, nil, remaining, footer
}

func splitLegacyFooterParagraphs(paragraphs []*presentationir.Paragraph) ([]*presentationir.Paragraph, []*presentationir.Paragraph) {
	if len(paragraphs) < 2 {
		return paragraphs, nil
	}
	originalLen := len(paragraphs)
	cut := len(paragraphs)
	sawStrongFooter := false
	for cut > 0 {
		tailText := paragraphText(paragraphs[cut-1])
		if !isLikelyLegacyFooterLine(tailText) {
			break
		}
		if isStrongLegacyFooterLine(tailText) {
			sawStrongFooter = true
		}
		cut--
	}
	if cut == len(paragraphs) {
		return paragraphs, nil
	}
	if !sawStrongFooter && originalLen == 2 && cut == 1 {
		return paragraphs, nil
	}
	if cut == 0 {
		if !sawStrongFooter {
			return paragraphs, nil
		}
		cut = 1
	}
	return paragraphs[:cut], paragraphs[cut:]
}

func promoteLegacyFooterOnlyParagraphs(paragraphs []*presentationir.Paragraph, allowAll bool) ([]*presentationir.Paragraph, []*presentationir.Paragraph) {
	body, footer := splitLegacyFooterParagraphs(paragraphs)
	if len(footer) > 0 || !allowAll || len(body) == 0 {
		return body, footer
	}
	for _, paragraph := range body {
		if !isLikelyLegacyFooterLine(paragraphText(paragraph)) {
			return body, nil
		}
	}
	return nil, body
}

func looksLikeLegacyBulletParagraphs(paragraphs []*presentationir.Paragraph) bool {
	if len(paragraphs) == 0 {
		return false
	}
	bulletCount := 0
	for _, paragraph := range paragraphs {
		if looksLikeLegacyBullet(paragraphText(paragraph)) {
			bulletCount++
		}
	}
	return bulletCount*2 >= len(paragraphs)
}

func isLikelyLegacyFooterLine(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || looksLikeLegacyBullet(text) {
		return false
	}
	if isStrongLegacyFooterLine(text) {
		return true
	}
	return len(strings.Fields(text)) <= 8 && len(text) <= 80
}

func isStrongLegacyFooterLine(text string) bool {
	return isLikelyLegacySlideNumber(text) || isLikelyLegacyDateTime(text)
}

func isLikelyLegacySlideNumber(text string) bool {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return false
	}
	if strings.HasPrefix(text, "slide ") || strings.HasPrefix(text, "page ") {
		return true
	}
	if strings.Contains(text, "/") || strings.Contains(text, ":") || strings.Contains(text, "-") {
		return false
	}
	for _, r := range text {
		if !unicode.IsDigit(r) && r != '/' && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func isLikelyLegacyDateTime(text string) bool {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return false
	}
	hasMonthToken := false
	for _, token := range []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"} {
		if strings.Contains(text, token) {
			hasMonthToken = true
			break
		}
	}
	hasMeridiemToken := hasLegacyMeridiemToken(text)
	if strings.Contains(text, "/") || strings.Contains(text, ":") || strings.Contains(text, "-") {
		for _, token := range []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec", "am", "pm"} {
			if strings.Contains(text, token) {
				return true
			}
		}
		digits := 0
		for _, r := range text {
			if unicode.IsDigit(r) {
				digits++
			}
		}
		return digits >= 4
	}
	if hasMonthToken {
		digits := 0
		for _, r := range text {
			if unicode.IsDigit(r) {
				digits++
			}
		}
		return digits >= 4
	}
	if hasMeridiemToken {
		digits := 0
		for _, r := range text {
			if unicode.IsDigit(r) {
				digits++
			}
		}
		return digits >= 1
	}
	return false
}

func hasLegacyMeridiemToken(text string) bool {
	for _, field := range strings.Fields(strings.ToLower(text)) {
		token := strings.Trim(field, ".,;:!?()[]{}")
		if token == "am" || token == "pm" {
			return true
		}
	}
	return false
}

func buildLegacyFooterShapes(slideIndex, startShapeID int, paragraphs []*presentationir.Paragraph) []*presentationir.Shape {
	if len(paragraphs) == 0 {
		return nil
	}
	shapes := make([]*presentationir.Shape, 0, len(paragraphs))
	for index, paragraph := range paragraphs {
		if paragraph == nil {
			continue
		}
		text := paragraphText(paragraph)
		placeholder := presentationir.PlaceholderFooter
		name := fmt.Sprintf("Legacy Footer %d", slideIndex+1)
		switch {
		case isLikelyLegacySlideNumber(text):
			placeholder = presentationir.PlaceholderSlideNumber
			name = fmt.Sprintf("Legacy Slide Number %d", slideIndex+1)
		case isLikelyLegacyDateTime(text):
			placeholder = presentationir.PlaceholderDateTime
			name = fmt.Sprintf("Legacy Date %d", slideIndex+1)
		}
		shapeParagraphs := annotateLegacyPlaceholderFields([]*presentationir.Paragraph{paragraph}, placeholder)
		shapes = append(shapes, newLegacySlideShape(slideIndex, startShapeID+index, name, placeholder, legacyFooterBoundsForPlaceholder(placeholder), shapeParagraphs))
	}
	return shapes
}

func annotateLegacyPlaceholderFields(paragraphs []*presentationir.Paragraph, placeholder presentationir.PlaceholderKind) []*presentationir.Paragraph {
	if len(paragraphs) == 0 {
		return paragraphs
	}
	fieldKind := ""
	switch placeholder {
	case presentationir.PlaceholderSlideNumber:
		fieldKind = "slidenum"
	default:
		return paragraphs
	}
	for _, paragraph := range paragraphs {
		if paragraph == nil || len(paragraph.Runs) == 0 {
			continue
		}
		paragraph.Runs[0].FieldKind = fieldKind
	}
	return paragraphs
}

func legacyFooterBounds() presentationir.Rect {
	return presentationir.Rect{X: 685800, Y: defaultSlideHeightEMU - 685800, W: defaultSlideWidthEMU - 1371600, H: 304800}
}

func legacyHeaderBounds() presentationir.Rect {
	return presentationir.Rect{X: 685800, Y: 381000, W: defaultSlideWidthEMU - 1371600, H: 304800}
}

func legacyFooterBoundsForPlaceholder(placeholder presentationir.PlaceholderKind) presentationir.Rect {
	switch placeholder {
	case presentationir.PlaceholderDateTime:
		return presentationir.Rect{X: 685800, Y: defaultSlideHeightEMU - 685800, W: 2286000, H: 304800}
	case presentationir.PlaceholderSlideNumber:
		return presentationir.Rect{X: defaultSlideWidthEMU - 2286000 - 685800, Y: defaultSlideHeightEMU - 685800, W: 2286000, H: 304800}
	case presentationir.PlaceholderFooter:
		// Keep a synthetic footer out of the date and slide-number columns.
		// Old/damaged PPTs can omit the authored footer anchor; using the generic
		// full-width fallback makes date and footer paint on top of each other.
		return presentationir.Rect{X: defaultSlideWidthEMU * 325 / 1000, Y: defaultSlideHeightEMU - 685800, W: defaultSlideWidthEMU * 350 / 1000, H: 304800}
	default:
		return legacyFooterBounds()
	}
}

func paragraphText(paragraph *presentationir.Paragraph) string {
	if paragraph == nil {
		return ""
	}
	var b strings.Builder
	for _, run := range paragraph.Runs {
		b.WriteString(run.Text)
	}
	return strings.TrimSpace(b.String())
}

func isLikelyLegacySlideTitle(text, fallbackName string) bool {
	text = strings.TrimSpace(text)
	if text == "" || strings.EqualFold(text, fallbackName) {
		return false
	}
	if looksLikeLegacyBullet(text) {
		return false
	}
	if len(strings.Fields(text)) > 4 && (strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?")) {
		return false
	}
	return len(strings.Fields(text)) <= 10 || len(text) <= 60
}

func isLikelyLegacySubtitle(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || looksLikeLegacyBullet(text) {
		return false
	}
	if len(strings.Fields(text)) > 4 && (strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?")) {
		return false
	}
	return len(strings.Fields(text)) <= 16 && len(text) <= 120
}

func looksLikeLegacyBullet(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, prefix := range []string{"* ", "- ", "• ", "o ", "▪ ", "‣ ", "› "} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	if strings.HasPrefix(trimmed, "(") {
		if inner, _, ok2 := strings.Cut(trimmed[1:], ")"); ok2 && inner != "" {
			allDigit := true
			for _, r := range inner {
				if !unicode.IsDigit(r) {
					allDigit = false
					break
				}
			}
			if allDigit || isLegacyBulletAlphaMarker(inner) || isLegacyBulletRomanMarker(inner) {
				return true
			}
		}
	}
	marker, _, ok := strings.Cut(trimmed, ". ")
	if !ok {
		marker, _, ok = strings.Cut(trimmed, ") ")
	}
	if !ok || marker == "" {
		return false
	}
	for _, r := range marker {
		if !unicode.IsDigit(r) {
			goto notNumeric
		}
	}
	return true

notNumeric:
	return isLegacyBulletAlphaMarker(marker) || isLegacyBulletRomanMarker(marker)
}

func isLegacyBulletAlphaMarker(marker string) bool {
	if len(marker) != 1 {
		return false
	}
	r := rune(marker[0])
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isLegacyBulletRomanMarker(marker string) bool {
	if marker == "" {
		return false
	}
	for _, r := range marker {
		switch unicode.ToUpper(r) {
		case 'I', 'V', 'X', 'L', 'C', 'D', 'M':
		default:
			return false
		}
	}
	return true
}

// normalizeLegacyPptPlaceholderGeometry repairs a legacy placeholder whose local
// Escher rectangle aliases the title rectangle instead of contributing its own
// layout geometry. The fallback is semantic (placeholder role), not fixture-specific.
// Authored non-overlapping geometry is preserved unchanged.
func normalizeLegacyPptPlaceholderGeometry(shapes []*presentationir.Shape) {
	var title *presentationir.Shape
	for _, shape := range shapes {
		if shape != nil && shape.Placeholder == presentationir.PlaceholderTitle {
			title = shape
			break
		}
	}
	if title == nil {
		return
	}
	titleBottom := title.Bounds.Y + title.Bounds.H
	for _, shape := range shapes {
		if shape == nil || shape.Placeholder != presentationir.PlaceholderSubtitle {
			continue
		}
		if shape.Bounds.Y < titleBottom && legacyRectsOverlap(title.Bounds, shape.Bounds) {
			shape.Bounds = legacySubtitleBounds()
		}
	}
}

func legacyRectsOverlap(a, b presentationir.Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func legacyTitleBounds() presentationir.Rect {
	return presentationir.Rect{X: 685800, Y: 457200, W: defaultSlideWidthEMU - 1371600, H: 914400}
}

func legacySubtitleBounds() presentationir.Rect {
	return presentationir.Rect{X: 914400, Y: 1600200, W: defaultSlideWidthEMU - 1828800, H: 762000}
}

func legacyBodyBounds(hasTitle bool) presentationir.Rect {
	y := int64(1371600)
	h := int64(defaultSlideHeightEMU - 1828800)
	if hasTitle {
		y = 1828800
		h = int64(defaultSlideHeightEMU - 2286000)
	}
	return presentationir.Rect{X: 914400, Y: y, W: defaultSlideWidthEMU - 1828800, H: h}
}

func buildTextParagraphs(text string) []*presentationir.Paragraph {
	// CR/LF are paragraph marks in the plain-text fallback. VT/FF are soft
	// line breaks and must remain inside the same IR paragraph so paragraph
	// spacing/list semantics are not applied to a visual line break.
	text = strings.ReplaceAll(text, "\r\n", "\r")
	text = strings.ReplaceAll(text, "\n", "\r")
	text = normalizePptSoftLineBreaks(text)
	rawParagraphs := strings.Split(text, "\r")
	paragraphs := make([]*presentationir.Paragraph, 0, len(rawParagraphs))
	for _, raw := range rawParagraphs {
		line := strings.TrimSpace(raw)
		if line == "" || legacyPptLooksLikePlaceholderSampleText(line) {
			continue
		}
		paragraphs = append(paragraphs, &presentationir.Paragraph{
			Runs: []*presentationir.Run{{Text: line}},
		})
	}
	return paragraphs
}

// fragmentParagraphs returns frag.Paragraphs when the StyleTextPropAtom has
// been parsed, otherwise falls back to building plain paragraphs from the
// normalized text.
func fragmentParagraphs(frag legacySlideFragment) []*presentationir.Paragraph {
	if frag.Paragraphs != nil {
		return filterLegacyPptPlaceholderSampleParagraphs(frag.Paragraphs)
	}
	return buildTextParagraphs(frag.Text)
}

func filterLegacyPptPlaceholderSampleParagraphs(paragraphs []*presentationir.Paragraph) []*presentationir.Paragraph {
	if len(paragraphs) == 0 {
		return nil
	}
	filtered := make([]*presentationir.Paragraph, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if legacyPptLooksLikePlaceholderSampleText(paragraphText(paragraph)) {
			continue
		}
		filtered = append(filtered, paragraph)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// joinSlideFragments concatenates fragment texts with newline separators.
func joinSlideFragments(fragments []legacySlideFragment) string {
	parts := make([]string, 0, len(fragments))
	for _, f := range fragments {
		if f.Text != "" {
			parts = append(parts, f.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// hasTypedFragments reports whether any fragment carries an explicit TextHeaderAtom type.
func hasTypedFragments(fragments []legacySlideFragment) bool {
	for _, f := range fragments {
		if f.TextType != pptTextTypeUnknown {
			return true
		}
	}
	return false
}

// buildLegacySlideShapesTyped groups fragments by TextHeaderAtom type into
// title and body shapes, falling back to heuristic footer detection.
func buildLegacySlideShapesTyped(slideIndex int, slideText legacySlideText, filename string) []*presentationir.Shape {
	var titleParas, centerBodyParas, bodyParas []*presentationir.Paragraph
	for _, frag := range slideText.Fragments {
		paras := fragmentParagraphs(frag)
		switch frag.TextType {
		case pptTextTypeTitle, pptTextTypeCenterTitle:
			titleParas = append(titleParas, paras...)
		case pptTextTypeBody:
			bodyParas = append(bodyParas, paras...)
		case pptTextTypeCenterBody:
			centerBodyParas = append(centerBodyParas, paras...)
		case pptTextTypeNotes:
			// Notes are not rendered as slide shapes.
		default:
			bodyParas = append(bodyParas, paras...)
		}
	}
	subtitleParas := centerBodyParas
	if len(titleParas) == 0 && len(subtitleParas) > 0 {
		// CenterBody is the subtitle region on title slides, but on body-only
		// slides it is closer to generic slide text than subtitle geometry.
		bodyParas = append(bodyParas, subtitleParas...)
		subtitleParas = nil
	}
	if len(titleParas) == 0 && len(subtitleParas) == 0 && len(bodyParas) == 0 {
		bodyParas = []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: filename}}}}
	}
	// Apply footer splitting to body content. When typed title/subtitle fragments
	// are present, a footer-only companion fragment should still promote out of
	// the body path even if it is the only non-title paragraph.
	bodyParas, footer := promoteLegacyFooterOnlyParagraphs(bodyParas, len(titleParas) > 0 || len(subtitleParas) > 0)
	if legacyPptLooksLikePlaceholderPromptParagraphs(titleParas) {
		titleParas = nil
	}
	if legacyPptLooksLikePlaceholderPromptParagraphs(subtitleParas) {
		subtitleParas = nil
	}
	if legacyPptLooksLikePlaceholderPromptParagraphs(bodyParas) {
		bodyParas = nil
	}
	if legacyPptLooksLikePlaceholderPromptParagraphs(footer) {
		footer = nil
	}
	shapes := make([]*presentationir.Shape, 0, 4)
	nextID := 1
	if len(titleParas) > 0 {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextID, fmt.Sprintf("Legacy Title %d", slideIndex+1), presentationir.PlaceholderTitle, legacyTitleBounds(), titleParas))
		nextID++
	}
	if len(subtitleParas) > 0 {
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextID, fmt.Sprintf("Legacy Subtitle %d", slideIndex+1), presentationir.PlaceholderSubtitle, legacySubtitleBounds(), subtitleParas))
		nextID++
	}
	if len(bodyParas) > 0 {
		bounds := legacyBodyBounds(len(titleParas) > 0)
		shapes = append(shapes, newLegacySlideShape(slideIndex, nextID, fmt.Sprintf("Legacy Body %d", slideIndex+1), presentationir.PlaceholderBody, bounds, bodyParas))
		nextID++
	}
	if footerShapes := buildLegacyFooterShapes(slideIndex, nextID, footer); len(footerShapes) > 0 {
		shapes = append(shapes, footerShapes...)
	}
	return shapes
}

// attachNotesByIndex merges notes slide text into corresponding presentation
// slides by position. Notes from the first notes slide go into slides[0], etc.
// Fragments without an explicit text type are promoted to pptTextTypeNotes.
func attachNotesByIndex(slides []legacySlideText, notes []legacySlideText) []legacySlideText {
	for noteIndex, notesSlide := range notes {
		if noteIndex >= len(slides) {
			break
		}
		slides[noteIndex].NotesPersistID = notesSlide.PersistID
		for _, frag := range notesSlide.Fragments {
			if frag.TextType == pptTextTypeUnknown {
				frag.TextType = pptTextTypeNotes
			}
			if frag.TextType != pptTextTypeNotes {
				continue
			}
			if frag.Text == "" {
				continue
			}
			alreadyPresent := false
			for _, existing := range slides[noteIndex].Fragments {
				if existing.Text == frag.Text && existing.TextType == pptTextTypeNotes {
					alreadyPresent = true
					break
				}
			}
			if !alreadyPresent {
				slides[noteIndex].Fragments = append(slides[noteIndex].Fragments, frag)
			}
		}
	}
	return slides
}

func mergeLegacySlides(dst, src []legacySlideText) []legacySlideText {
	if len(src) == 0 {
		return dst
	}
	if len(dst) == 0 {
		out := make([]legacySlideText, len(src))
		copy(out, src)
		return out
	}
	for _, slide := range src {
		if mergeLegacySlideByPersistID(dst, slide) {
			continue
		}
		dst = append(dst, slide)
	}
	return dst
}

func projectLegacySlidesInAuthoredOrder(order []int, slides []legacySlideText) []legacySlideText {
	if len(order) == 0 {
		return slides
	}
	byPersistID := make(map[int]legacySlideText, len(slides))
	for _, slide := range slides {
		if slide.PersistID == 0 {
			continue
		}
		// The authored SlideListWithText order owns membership and ordering. If
		// duplicate projections exist, keep the first materialized projection;
		// later parser passes may enrich it through mergeLegacySlides before this
		// structural projection runs.
		if _, exists := byPersistID[slide.PersistID]; !exists {
			byPersistID[slide.PersistID] = slide
		}
	}
	out := make([]legacySlideText, 0, len(order))
	seen := make(map[int]struct{}, len(order))
	for _, persistID := range order {
		if persistID == 0 {
			continue
		}
		if _, duplicate := seen[persistID]; duplicate {
			continue
		}
		seen[persistID] = struct{}{}
		slide, ok := byPersistID[persistID]
		if !ok {
			// Authored membership is authoritative even for drawing-only, table-only,
			// or otherwise empty slides that produced no text projection.
			slide = legacySlideText{PersistID: persistID}
		}
		out = append(out, slide)
	}
	return out
}

func filterLegacyDuplicateProjectedTables(flatShapes, tableShapes map[int][]*presentationir.Shape) map[int][]*presentationir.Shape {
	if len(flatShapes) == 0 || len(tableShapes) == 0 {
		return tableShapes
	}
	for persistID, tables := range tableShapes {
		siblings := flatShapes[persistID]
		if len(siblings) == 0 {
			continue
		}
		filtered := tables[:0]
		for _, tableShape := range tables {
			if legacyTableDuplicatesSiblingText(tableShape, siblings) {
				continue
			}
			filtered = append(filtered, tableShape)
		}
		if len(filtered) == 0 {
			delete(tableShapes, persistID)
		} else {
			tableShapes[persistID] = filtered
		}
	}
	return tableShapes
}

func legacyTableDuplicatesSiblingText(tableShape *presentationir.Shape, siblings []*presentationir.Shape) bool {
	if tableShape == nil || tableShape.Table == nil {
		return false
	}

	// Compare paragraph-sized projection atoms rather than whole-cell strings.
	// Legacy diagrams commonly project one logical label per ordinary text shape,
	// while the compatibility table scanner combines several of those labels into
	// one inferred cell. Whole-cell comparison therefore misses the duplicate even
	// though both projections came from the same Escher group.
	cellTexts := make([]string, 0)
	for _, row := range tableShape.Table.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellTexts = append(cellTexts, legacyPptParagraphProjectionKeys(cell.Paragraphs)...)
		}
	}
	if len(cellTexts) < 2 {
		return false
	}

	siblingTexts := make(map[string]int)
	var collect func([]*presentationir.Shape)
	collect = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil || shape == tableShape {
				continue
			}
			for _, text := range legacyPptParagraphProjectionKeys(shape.Paragraphs) {
				siblingTexts[text]++
			}
			if len(shape.Children) > 0 {
				collect(shape.Children)
			}
		}
	}
	collect(siblings)

	matched := 0
	for _, text := range cellTexts {
		if siblingTexts[text] > 0 {
			matched++
			siblingTexts[text]--
		}
	}
	// Require multiple independently projected labels and majority coverage. This
	// rejects diagram-derived pseudo tables without letting one coincidental label
	// suppress a legitimate compatibility table such as bug57820.
	return matched >= 2 && matched*2 >= len(cellTexts)
}

func legacyPptParagraphProjectionKeys(paragraphs []*presentationir.Paragraph) []string {
	keys := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if paragraph == nil {
			continue
		}
		var b strings.Builder
		for _, run := range paragraph.Runs {
			if run != nil {
				b.WriteString(run.Text)
			}
		}
		// Projection paths disagree about tabs/newlines and repeated spaces. Those
		// are formatting details, not provenance, so compare canonical word spacing.
		key := strings.Join(strings.Fields(b.String()), " ")
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func filterLegacyGroupedShapeDuplicates(flatShapes, groupShapes map[int][]*presentationir.Shape) map[int][]*presentationir.Shape {
	if len(flatShapes) == 0 || len(groupShapes) == 0 {
		return flatShapes
	}
	for persistID, shapes := range flatShapes {
		groupedIDs := collectLegacyGroupedShapeIDs(groupShapes[persistID])
		if len(groupedIDs) == 0 {
			continue
		}
		filtered := shapes[:0]
		for _, shape := range shapes {
			if shape == nil || shape.OriginalShapeID == 0 {
				filtered = append(filtered, shape)
				continue
			}
			if _, ok := groupedIDs[shape.OriginalShapeID]; ok {
				continue
			}
			filtered = append(filtered, shape)
		}
		if len(filtered) == 0 {
			delete(flatShapes, persistID)
			continue
		}
		flatShapes[persistID] = filtered
	}
	return flatShapes
}

func collectLegacyGroupedShapeIDs(groups []*presentationir.Shape) map[uint32]struct{} {
	if len(groups) == 0 {
		return nil
	}
	ids := map[uint32]struct{}{}
	var walk func(*presentationir.Shape)
	walk = func(shape *presentationir.Shape) {
		if shape == nil {
			return
		}
		if shape.Kind != presentationir.ShapeGroup && shape.OriginalShapeID != 0 {
			ids[shape.OriginalShapeID] = struct{}{}
		}
		for _, child := range shape.Children {
			walk(child)
		}
	}
	for _, group := range groups {
		walk(group)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func legacyPptShapeIndexByOriginalID(shapes []*presentationir.Shape) map[uint32]*presentationir.Shape {
	if len(shapes) == 0 {
		return nil
	}
	out := map[uint32]*presentationir.Shape{}
	var walk func(*presentationir.Shape)
	walk = func(shape *presentationir.Shape) {
		if shape == nil {
			return
		}
		if shape.Kind != presentationir.ShapeGroup && shape.OriginalShapeID != 0 {
			if _, exists := out[shape.OriginalShapeID]; !exists {
				out[shape.OriginalShapeID] = shape
			}
		}
		for _, child := range shape.Children {
			walk(child)
		}
	}
	for _, shape := range shapes {
		walk(shape)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// legacyPptGroupChildSource supplies semantic payload for a shape while group
// construction owns its authored local geometry. This deliberately prevents the
// group parser from treating the slide-space discovery index as canonical visual
// composition. The source is transitional: subsequent passes can populate it
// directly from Escher shape containers without changing group construction.
type legacyPptGroupChildSource interface {
	CloneSemanticPayload(shapeID uint32) *presentationir.Shape
}

type legacyPptDiscoveredShapeSource map[uint32]*presentationir.Shape

func (s legacyPptDiscoveredShapeSource) CloneSemanticPayload(shapeID uint32) *presentationir.Shape {
	shape := s[shapeID]
	if shape == nil {
		return nil
	}
	clone := cloneLegacyShapeRecursive(shape, nil)
	if clone == nil {
		return nil
	}
	// Discovery is allowed to supply semantic payload only. Canonical geometry and
	// transform state are reconstructed from the Escher SpContainer below.
	clone.Bounds = presentationir.Rect{}
	clone.RotationDeg = 0
	clone.FlipH = false
	clone.FlipV = false
	clone.ConnectorStart = nil
	clone.ConnectorEnd = nil
	clone.ConnectorPath = nil

	// Discovery contributes payload only. Every visual fact below belongs to the
	// authoritative group-child SpContainer and must be reconstructed there;
	// retaining discovery-time values can leak stale transforms, arrow mappings,
	// inherited paint, or effects into the canonical hierarchy.
	clone.OfficeArt = nil
	clone.Geometry = ""
	clone.GeometryData = nil
	clone.GeometryAdjustments = nil
	clone.Fill = nil
	clone.FillColor = ""
	clone.LineStyle = nil
	clone.LineColor = ""
	clone.LineWidthPt = 0
	clone.ArrowHead = ""
	clone.ArrowHeadWidth = ""
	clone.ArrowHeadLength = ""
	clone.ArrowTail = ""
	clone.ArrowTailWidth = ""
	clone.ArrowTailLength = ""
	clone.Effects = drawing.EffectList{}
	return clone
}

// canonicalizeLegacyPptGroupChild reconstructs every visual property whose meaning
// depends on the Escher shape container. The discovery pipeline contributes only
// payload that cannot be recovered locally (resolved text, image bytes, etc.).
// This keeps the canonical group tree independent from slide-space discovery.
func canonicalizeLegacyPptGroupChild(child *presentationir.Shape, data []byte, colorScheme map[uint8]string) bool {
	if child == nil {
		return false
	}
	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil || node == nil || len(node.ChildAnchor) == 0 {
		return false
	}
	// Group construction owns authored local geometry. Line/connector children
	// commonly encode a zero-width or zero-height ChildAnchor. Preserve those
	// degenerate connector frames so the child stays in its group and inherits
	// the parent transform; keep the historical strictness for other group-child
	// families until their zero-axis semantics are promoted deliberately.
	allowDegenerate := child.Kind == presentationir.ShapeConnector || child.Kind == presentationir.ShapeSmartArtConnector
	bounds, ok := parseLegacyPptChildPlacementAnchor(node.ChildAnchor, allowDegenerate)
	if !ok {
		return false
	}
	child.OriginalShapeID = node.ShapeID
	child.Bounds = bounds
	child.OfficeArt = officeartlegacy.ShapeFromEscherWithColorResolver(node, func(raw uint32) string {
		return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
	})
	var transform legacyPptShapeTransform
	transform.FlipH = node.Flags&legacyOfficeArtSpFlagFlipH != 0
	transform.FlipV = node.Flags&legacyOfficeArtSpFlagFlipV != 0
	if len(node.Properties) > 0 {
		parseLegacyPptFOPTPropertiesTransform(node.Properties, colorScheme, &transform)
	}
	applyLegacyPptOfficeArtSurface(child, child.OfficeArt, transform)

	if child.Kind == presentationir.ShapeConnector {
		child.ConnectorFlipH = transform.FlipH
		child.ConnectorFlipV = transform.FlipV
		child.ConnectorStart, child.ConnectorEnd = legacyPptConnectorEndpoints(bounds, transform.FlipH, transform.FlipV)
	}
	return true
}

func parseLegacyPptGroupShapeMetaFromSpgrContainer(data []byte, parent legacyPptGroupFrame, exHyperlinks map[uint32]string) (legacyPptGroupShapeMeta, legacyPptGroupFrame, bool) {
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		if recType == legacyOfficeArtSpContainer && legacyPptSpContainerHasGroupRect(body) {
			meta, frame, ok := parseLegacyPptGroupShapeMetaFromSpContainer(body, parent, exHyperlinks)
			if ok {
				return meta, frame, true
			}
		}
		off += 8 + recLen
	}
	return legacyPptGroupShapeMeta{}, legacyPptGroupFrame{}, false
}

func parseLegacyPptGroupShapeMetaFromSpContainer(data []byte, parent legacyPptGroupFrame, exHyperlinks map[uint32]string) (legacyPptGroupShapeMeta, legacyPptGroupFrame, bool) {
	frame, ok := parseLegacyPptGroupFrameFromSpContainer(data, parent)
	if !ok {
		return legacyPptGroupShapeMeta{}, legacyPptGroupFrame{}, false
	}
	meta := legacyPptGroupShapeMeta{bounds: frame.localTarget}
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		switch recType {
		case legacyOfficeArtSp:
			if len(body) >= 4 {
				meta.shapeID = binary.LittleEndian.Uint32(body[:4])
			}
			if len(body) >= 8 {
				spFlags := binary.LittleEndian.Uint32(body[4:8])
				meta.transform.FlipH = spFlags&legacyOfficeArtSpFlagFlipH != 0
				meta.transform.FlipV = spFlags&legacyOfficeArtSpFlagFlipV != 0
			}
			_ = verInst
		case officeartescher.RecordFOPT, officeartescher.RecordSecondaryFOPT, officeartescher.RecordTertiaryFOPT:
			// OfficeArt properties are a three-tier cascade. Later secondary/tertiary
			// tables override earlier authored values and must participate in the same
			// transform decode as the primary FOPT.
			parseLegacyPptFOPTTransform(body, int(verInst>>4), nil, &meta.transform)
		case legacyOfficeArtClientData:
			if meta.hyperlinkURL == "" && meta.actionLink == "" {
				meta.hyperlinkURL, meta.actionLink = parseLegacyPptShapeInteractiveInfo(body, exHyperlinks)
			}
			if anim, ok := parseLegacyPptShapeAnimationInfo(body); ok {
				meta.animation = anim
			}
		}
		off += 8 + recLen
	}
	return meta, frame, true
}

// parseLegacyPptSlideVisualsEventDriven is the production bridge from the
// forward binary record boundary to canonical visual IR.  Each persisted Slide
// record is opened once.  All core visual projections share that same payload
// and are reconciled before the parser advances to the next slide.
//
// The family decoders are intentionally reused in this takeover pass: they
// already encode years of OfficeArt compatibility behavior.  What changes here
// is ownership and traversal.  Subsequent legacy passes can replace individual
// projections with direct RecordEvent consumers without changing the slide
// lifecycle or downstream IR contract again.

type legacyPptSlideVisualDispatch struct {
	containers legacyPptShapeContainerScene
	rules      map[uint32]legacyPptConnectorRule
}

func collectLegacyPptSlideVisualDispatch(payload []byte) legacyPptSlideVisualDispatch {
	var dispatch legacyPptSlideVisualDispatch
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTPPDrawing {
			events, _ := CollectOfficeArtEvents(childPayload)
			dispatch.containers = mergeLegacyPptShapeContainerScenes(dispatch.containers, decodeLegacyPptShapeContainerScene(events))
			for id, rule := range parseLegacyPptConnectorRules(childPayload) {
				if dispatch.rules == nil {
					dispatch.rules = map[uint32]legacyPptConnectorRule{}
				}
				dispatch.rules[id] = rule
			}
		}
		pos = next
	}
	return dispatch
}

type legacyPptEventGroupState struct {
	shape       *presentationir.Shape
	frame       legacyPptGroupFrame
	skippedRoot bool
	skip        bool
}

// buildLegacyPptGroupShapesFromSceneWithOwnership reports table group paths that
// materialize while the group projector consumes the semantic scene. The dedicated
// table projector is not the only table materialization path: nested/compatibility
// tables can be recognized while ordinary groups are assembled. Ownership must be
// emitted at that same boundary so the completeness ledger cannot lose their grid
// topology.
func buildLegacyPptGroupShapesFromSceneWithOwnership(scene legacyPptShapeContainerScene, childSource legacyPptGroupChildSource, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, projectedTablePaths *[][]int) []*presentationir.Shape {
	if len(scene.officeArtEvents()) == 0 || childSource == nil {
		return nil
	}
	stack := []legacyPptEventGroupState{{frame: legacyPptGroupFrame{}}}
	var groups []*presentationir.Shape
	ordinal := 1
	groupIndex := 0
	for _, event := range scene.officeArtEvents() {
		switch {
		case event.GroupStart:
			parent := stack[len(stack)-1]
			state := legacyPptEventGroupState{frame: parent.frame, skip: parent.skip}
			var semanticGroup legacyPptShapeGroup
			if groupIndex < len(scene.groups) {
				semanticGroup = scene.groups[groupIndex]
			}
			groupIndex++
			table, tableOwned := parseLegacyPptTableShapeFromSceneGroup(scene, semanticGroup, fontTable, colorScheme, exHyperlinks, 1)
			if !state.skip && tableOwned {
				state.skip = true
				if projectedTablePaths != nil && len(semanticGroup.Path) != 0 {
					*projectedTablePaths = appendLegacyPptOwnedGroupPath(*projectedTablePaths, semanticGroup.Path)
				}
				if len(stack) > 1 && parent.shape != nil {
					parent.shape.Children = append(parent.shape.Children, table)
					stack[len(stack)-1] = parent
				}
			} else if !state.skip {
				meta, frame, ok := parseLegacyPptGroupShapeMetaFromSpgrContainer(event.Payload, parent.frame, exHyperlinks)
				if ok {
					state.frame = frame
					groupTransform := legacyPptCanonicalGroupTransform(frame, meta.transform)
					state.shape = &presentationir.Shape{Kind: presentationir.ShapeGroup, Name: fmt.Sprintf("Legacy Group %d", ordinal), OriginalShapeID: meta.shapeID, Bounds: meta.bounds, GroupTransform: &groupTransform, HyperlinkURL: meta.hyperlinkURL, ActionLink: meta.actionLink}
					applyLegacyPptShapeTransform(state.shape, meta.transform)
					applyLegacyPptAnimationMetadata(state.shape, meta.animation)
					ordinal++
				}
			}
			stack = append(stack, state)
		case event.ShapeStart:
			if len(stack) <= 1 {
				continue
			}
			state := &stack[len(stack)-1]
			if state.skip || state.shape == nil {
				continue
			}
			if !state.skippedRoot && event.ShapeNode != nil && event.ShapeNode.GroupRect != nil {
				state.skippedRoot = true
				continue
			}
			shapeID := uint32(0)
			if event.ShapeNode != nil {
				shapeID = event.ShapeNode.ShapeID
			}
			if shapeID == 0 {
				continue
			}
			child := childSource.CloneSemanticPayload(shapeID)
			if child != nil && canonicalizeLegacyPptGroupChild(child, event.Payload, colorScheme) {
				state.shape.Children = append(state.shape.Children, child)
			}
		case event.GroupEnd:
			if len(stack) <= 1 {
				continue
			}
			state := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if state.skip || state.shape == nil || len(state.shape.Children) == 0 {
				continue
			}

			// The outer SpgrContainer of a slide/master is a structural drawing
			// root. It frequently has no canonical ShapeGroup of its own. Do not
			// let that transparent structural parent discard authored nested
			// groups when they close. Attach to the nearest materialized ancestor;
			// if none exists, the group is a top-level canonical visual.
			attached := false
			for i := len(stack) - 1; i >= 1; i-- {
				if stack[i].skip {
					break
				}
				if stack[i].shape != nil {
					stack[i].shape.Children = append(stack[i].shape.Children, state.shape)
					attached = true
					break
				}
			}
			if !attached {
				groups = append(groups, state.shape)
			}
		}
	}
	if len(groups) == 0 {
		return nil
	}
	return groups
}

func parseLegacyPptSlideVisualsEventDriven(
	persistEvents legacyPptPersistEventIndex,
	slidePersistIDs []int,
	slides []legacySlideText,
	fontTable map[uint16]string,
	colorScheme map[uint8]string,
	masterColorSchemes map[int]map[uint8]string,
	slideColorSchemes map[int]map[uint8]string,
	masterPlaceholderTemplates map[int]map[presentationir.PlaceholderKind]legacyPptTextShape,
	exHyperlinks map[uint32]string,
	pictureStore map[uint32]*legacyPptPicture,
	externalObjects map[uint32]legacyPptExternalObject,
) (map[int][]*presentationir.Shape, map[int]map[uint32]int) {
	if len(slidePersistIDs) == 0 || len(persistEvents.slides) == 0 {
		return nil, nil
	}
	textByPersistID := make(map[int]legacySlideText, len(slides))
	for _, slide := range slides {
		if slide.PersistID != 0 {
			textByPersistID[slide.PersistID] = slide
		}
	}
	out := make(map[int][]*presentationir.Shape, len(slidePersistIDs))
	roles := make(map[int]map[uint32]int, len(slidePersistIDs))
	for _, persistID := range slidePersistIDs {
		event, ok := persistEvents.slides[persistID]
		if !ok || event.Record.Type != pptRTSlide {
			continue
		}
		payload := event.Payload

		// BIG13: one physical OfficeArt traversal feeds every core visual family.
		// Family decoders consume the transient event buffer rather than reopening or
		// recursively walking SpgrContainer/SpContainer trees independently.
		scene := collectLegacyPptVisualScene(payload)
		slideScheme := effectiveLegacyPptSlideColorScheme(textByPersistID[persistID], persistID, colorScheme, masterColorSchemes, slideColorSchemes)
		projection := scene.completeProjection(
			textByPersistID[persistID], fontTable, slideScheme,
			masterPlaceholderTemplates[textByPersistID[persistID].MasterID],
			exHyperlinks, pictureStore, externalObjects,
		)
		if len(projection.Shapes) != 0 {
			out[persistID] = projection.Shapes
		}
		if len(projection.TextRoles) != 0 {
			roles[persistID] = projection.TextRoles
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	if len(roles) == 0 {
		roles = nil
	}
	return out, roles
}

func buildLegacyPptTextShapesFromMetas(metas []legacyPptTextShape, slideText legacySlideText) []*presentationir.Shape {
	if len(metas) == 0 {
		return nil
	}
	hasTitleContext := false
	for _, meta := range metas {
		if meta.placeholder == presentationir.PlaceholderTitle {
			hasTitleContext = true
			break
		}
		if meta.textType == pptTextTypeTitle || meta.textType == pptTextTypeCenterTitle {
			hasTitleContext = true
		}
	}
	if !hasTitleContext {
		for _, frag := range slideText.Fragments {
			if frag.TextType == pptTextTypeTitle || frag.TextType == pptTextTypeCenterTitle {
				hasTitleContext = true
				break
			}
		}
	}
	usedFragments := make([]bool, len(slideText.Fragments))
	shapes := make([]*presentationir.Shape, 0, len(metas)+1)
	for index, meta := range metas {
		// OEPlaceholderAtom owns placeholder identity for slide-local shapes.
		// TextHeaderAtom's text type describes the text stream and is not, by
		// itself, evidence that an arbitrary textbox is a placeholder. Master
		// structural inference is handled separately by legacyPptMasterPlaceholderKind.
		placeholder := meta.placeholder
		paragraphs := cloneLegacyParagraphs(meta.paragraphs)
		if len(paragraphs) > 0 {
			matched := markLegacySlideFragmentByShapeID(slideText, usedFragments, meta.shapeID) || markLegacySlideFragmentByOutlineRef(slideText, usedFragments, meta.outlineRef)
			if !matched {
				matched = markLegacySlideFragmentsByParagraphs(slideText, usedFragments, paragraphs)
			}
			if !matched && legacyPptLooksLikePlaceholderPromptParagraphs(paragraphs) {
				paragraphs = nil
			}
		} else {
			shapeIDParagraphs := previewLegacyPptShapeIDParagraphs(slideText, usedFragments, meta.shapeID)
			outlineParagraphs := previewLegacyPptOutlineRefParagraphs(slideText, usedFragments, meta.outlineRef)
			if len(shapeIDParagraphs) > 0 {
				if legacyPptLooksLikePlaceholderPromptParagraphs(shapeIDParagraphs) && len(outlineParagraphs) > 0 && !legacyPptLooksLikePlaceholderPromptParagraphs(outlineParagraphs) {
					paragraphs = consumeLegacyPptOutlineRefParagraphs(slideText, usedFragments, meta.outlineRef)
				} else {
					paragraphs = consumeLegacyPptShapeIDParagraphs(slideText, usedFragments, meta.shapeID)
				}
			} else if len(outlineParagraphs) > 0 {
				paragraphs = consumeLegacyPptOutlineRefParagraphs(slideText, usedFragments, meta.outlineRef)
			}
			if len(paragraphs) == 0 && placeholder != presentationir.PlaceholderNone {
				paragraphs = consumeLegacyPptPlaceholderParagraphs(slideText, usedFragments, placeholder, hasTitleContext)
			}
			if len(paragraphs) == 0 && placeholder != presentationir.PlaceholderNone && meta.textType != pptTextTypeUnknown {
				paragraphs = consumeLegacyPptTextTypeParagraphs(slideText, usedFragments, []int{meta.textType})
			}
			if len(paragraphs) == 0 && len(meta.inlineFallback) > 0 {
				paragraphs = cloneLegacyParagraphs(meta.inlineFallback)
			}
		}
		if len(paragraphs) == 0 {
			continue
		}
		if legacyPptLooksLikePlaceholderPromptParagraphs(paragraphs) {
			continue
		}
		paragraphs = annotateLegacyPlaceholderFields(paragraphs, placeholder)
		// Final text-stream matching and master/style inheritance can reintroduce
		// an inherited marker after the ClientTextbox pass. Apply the static PPT
		// empty-paragraph rule at the canonical shape boundary, after all merges.
		suppressEmptyLegacyBulletMarkers(paragraphs)
		shape := &presentationir.Shape{
			Kind:            presentationir.ShapeText,
			OriginalShapeID: meta.shapeID,
			Name:            legacyPptTextShapeName(index+1, placeholder),
			Placeholder:     placeholder,
			PlaceholderIdx:  meta.placeholderIdx,
			Bounds:          meta.bounds,
			FillColor:       meta.fillColor,
			Paragraphs:      paragraphs,
			HyperlinkURL:    meta.hyperlinkURL,
			ActionLink:      meta.actionLink,
			OfficeArt:       meta.officeArt,
		}
		// Text is payload of an OfficeArt shape, not a replacement for it.
		applyLegacyPptOfficeArtSurface(shape, meta.officeArt, meta.transform)
		if meta.fillColor != "" && shape.Fill == nil {
			shape.Fill = &drawing.SolidFill{Color: meta.fillColor, Alpha: 1.0}
		}
		applyLegacyPptAnimationMetadata(shape, meta.animation)
		shapes = append(shapes, shape)
	}
	if leftovers := collectUnusedLegacySlideFragments(slideText, usedFragments); len(leftovers.Fragments) > 0 {
		for _, shape := range buildLegacySlideShapesTyped(0, leftovers, "") {
			if shape == nil {
				continue
			}
			clone := *shape
			clone.ID = 0
			shapes = append(shapes, &clone)
		}
	}
	if len(shapes) == 0 {
		return nil
	}
	return shapes
}

func collectLegacyPptTextShapesFromEvents(events []OfficeArtEvent, frame legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, shapes *[]legacyPptTextShape) {
	frames := []legacyPptGroupFrame{frame}
	_ = walkCollectedOfficeArtEvents(events, OfficeArtConsumerFunc(func(event OfficeArtEvent) error {
		switch {
		case event.GroupStart:
			parent := frames[len(frames)-1]
			next := parent
			if groupFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(event.Payload, parent); ok {
				next = groupFrame
			}
			frames = append(frames, next)
		case event.GroupEnd:
			if len(frames) > 1 {
				frames = frames[:len(frames)-1]
			}
		case event.ShapeStart:
			// Text extraction must not speculate about table ownership. The table
			// projector runs at the semantic-scene boundary and reports the exact
			// group paths it successfully materialized. Until that outcome exists,
			// every authored ClientTextbox remains recoverable text. This prevents
			// a permissive compatibility-table guess from deleting diagram labels
			// when the stricter table projector later rejects the candidate.
			shape, ok := parseLegacyPptTextShapeNode(event.ShapeNode, frames[len(frames)-1], fontTable, colorScheme, exHyperlinks)
			if ok {
				*shapes = append(*shapes, shape)
			}
		}
		return nil
	}))
}

func parseLegacyPptTextShapeContainer(data []byte, frame legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) (legacyPptTextShape, bool) {
	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil {
		return legacyPptTextShape{}, false
	}
	return parseLegacyPptTextShapeNode(node, frame, fontTable, colorScheme, exHyperlinks)
}

func parseLegacyPptTextShapeNode(node *officeartescher.ShapeNode, frame legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) (legacyPptTextShape, bool) {
	var shape legacyPptTextShape
	shape.outlineRef = -1
	shape.textType = pptTextTypeUnknown
	if node == nil {
		return legacyPptTextShape{}, false
	}
	// Connector SpContainers can carry an empty ClientTextbox in legacy PPT.
	// They are owned by the connector projection; treating that textbox as a
	// text shape paints the connector's bounding rectangle a second time.
	if legacyPptConnectorType(node.ShapeType) != "" {
		return legacyPptTextShape{}, false
	}
	shape.shapeID = node.ShapeID
	shape.officeArt = officeartlegacy.ShapeFromEscherWithColorResolver(node, func(raw uint32) string {
		return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
	})
	shape.transform.FlipH = node.Flags&legacyOfficeArtSpFlagFlipH != 0
	shape.transform.FlipV = node.Flags&legacyOfficeArtSpFlagFlipV != 0

	placement := resolveLegacyPptShapePlacement(node, frame)

	if len(node.ClientData) > 0 {
		if placeholder, placeholderIdx, ok := parseLegacyPptPlaceholderClientData(node.ClientData); ok {
			shape.placeholder = placeholder
			shape.placeholderIdx = placeholderIdx
		}
		if url, action := parseLegacyPptShapeInteractiveInfo(node.ClientData, exHyperlinks); url != "" || action != "" {
			shape.hyperlinkURL = url
			shape.actionLink = action
		}
		if anim, ok := parseLegacyPptShapeAnimationInfo(node.ClientData); ok {
			shape.animation = anim
		}
	}

	if len(node.Properties) > 0 {
		// Prefer the shared OfficeArt surface: it resolves contextual COLORREFs
		// before PPT text/table projection. The local helper is intentionally only
		// a fallback because it cannot resolve property-relative fSysIndex colors.
		if shape.officeArt != nil && shape.officeArt.Fill.Kind == officeartir.FillSolid && shape.officeArt.Fill.Color != "" {
			shape.fillColor = shape.officeArt.Fill.Color
		} else if color, ok := parseLegacyPptFOPTPropertiesTextFillColor(node.Properties, colorScheme); ok {
			shape.fillColor = color
		}
		parseLegacyPptFOPTPropertiesTransform(node.Properties, colorScheme, &shape.transform)
	}

	sawTextbox := node.HasClientTextbox
	if len(node.ClientTextbox) > 0 {
		frags, outlineRef := parseLegacyPptTextBoxFragments(node.ClientTextbox, fontTable, colorScheme, exHyperlinks)
		if outlineRef >= 0 {
			shape.outlineRef = outlineRef
		}
		if len(frags) > 0 {
			shape.textType = dominantLegacyPptTextType(frags)
			shape.paragraphs = legacyPptFragmentsParagraphs(frags)
			suppressEmptyLegacyBulletMarkers(shape.paragraphs)
		}
		if inlineFrags, _ := parseLegacyPptTextBoxFragmentsRecursive(node.ClientTextbox, fontTable, colorScheme, exHyperlinks); len(inlineFrags) > 0 {
			if shape.textType == pptTextTypeUnknown {
				shape.textType = dominantLegacyPptTextType(inlineFrags)
			}
			shape.inlineFallback = legacyPptFragmentsParagraphs(inlineFrags)
			suppressEmptyLegacyBulletMarkers(shape.inlineFallback)
		}
	}

	haveAnchor := placement.valid()
	if haveAnchor {
		shape.bounds = placement.Bounds
	}
	if !haveAnchor || (!sawTextbox && len(shape.paragraphs) == 0 && len(shape.inlineFallback) == 0) {
		return legacyPptTextShape{}, false
	}

	return shape, true
}

// suppressEmptyLegacyBulletMarkers preserves blank legacy paragraphs as spacing
// boundaries without painting an editing-only list marker. Binary PPT can carry
// fHasBullet/bulletChar on an empty TextPFRun paragraph between visible list
// entries. PowerPoint does not paint that marker in slide show/print output.
// Keep all paragraph metrics and provenance; only the marker identity is cleared.
func suppressEmptyLegacyBulletMarkers(paragraphs []*presentationir.Paragraph) {
	for _, paragraph := range paragraphs {
		if paragraph == nil || (paragraph.BulletChar == "" && !paragraph.AutoNum) {
			continue
		}
		visible := false
		for _, run := range paragraph.Runs {
			if run != nil && strings.TrimSpace(run.Text) != "" {
				visible = true
				break
			}
		}
		if visible {
			continue
		}
		paragraph.BulletChar = ""
		paragraph.AutoNum = false
		paragraph.AutoNumType = ""
		paragraph.AutoNumStart = 0
		// Empty paragraphs are still paragraph nodes and therefore still pass
		// through level/master list-style resolution later. Merely clearing the
		// local glyph is not enough: the layout cascade can inherit the level's
		// bullet back onto the blank line. Preserve the blank line for spacing,
		// but make its no-marker state authoritative at the canonical boundary.
		paragraph.BulletSpecified = true
		paragraph.BulletNone = true
	}
}

func parseLegacyPptPlaceholderClientData(data []byte) (presentationir.PlaceholderKind, int, bool) {
	for pos := 0; pos+8 <= len(data); {
		record, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			break
		}
		if record.Type == pptRTPlaceholderAtom {
			if placeholder, placeholderIdx, ok := parseLegacyPptPlaceholderAtom(payload); ok {
				return placeholder, placeholderIdx, true
			}
		}
		pos = next
	}
	return presentationir.PlaceholderNone, 0, false
}

func parseLegacyPptPlaceholderAtom(payload []byte) (presentationir.PlaceholderKind, int, bool) {
	if len(payload) < 4 {
		return presentationir.PlaceholderNone, 0, false
	}
	// placementId (bytes 0–3) is a per-slide shape-sequence index.
	// 0xFFFFFFFF is the sentinel meaning "not a placeholder shape".
	placementID := binary.LittleEndian.Uint32(payload[:4])
	if placementID == 0xFFFFFFFF {
		return presentationir.PlaceholderNone, 0, false
	}
	if len(payload) < 5 {
		return presentationir.PlaceholderNone, 0, false
	}
	placeholderIdx := int(placementID)
	// placeholderId (byte 4) encodes the placeholder type per MS-PPT
	// OEPlaceholderAtom PlaceholderEnum (spec section 2.9.29).
	switch payload[4] {
	case 0x01: // PT_MasterTitle
		return presentationir.PlaceholderTitle, placeholderIdx, true
	case 0x02: // PT_MasterBody
		return presentationir.PlaceholderBody, placeholderIdx, true
	case 0x03: // PT_MasterCenterTitle
		return presentationir.PlaceholderTitle, placeholderIdx, true
	case 0x04: // PT_MasterSubTitle
		return presentationir.PlaceholderSubtitle, placeholderIdx, true
	case 0x0D: // PT_Title
		return presentationir.PlaceholderTitle, placeholderIdx, true
	case 0x0E: // PT_Body
		return presentationir.PlaceholderBody, placeholderIdx, true
	case 0x0F: // PT_CenterTitle
		return presentationir.PlaceholderTitle, placeholderIdx, true
	case 0x10: // PT_SubTitle
		return presentationir.PlaceholderSubtitle, placeholderIdx, true
	case 0x11: // PT_VerticalTitle
		return presentationir.PlaceholderTitle, placeholderIdx, true
	case 0x12: // PT_VerticalBody
		return presentationir.PlaceholderBody, placeholderIdx, true
	case 0x07: // PT_MasterDate
		return presentationir.PlaceholderDateTime, placeholderIdx, true
	case 0x08: // PT_MasterSlideNumber
		return presentationir.PlaceholderSlideNumber, placeholderIdx, true
	case 0x09: // PT_MasterFooter
		return presentationir.PlaceholderFooter, placeholderIdx, true
	case 0x0C: // PT_NotesBody
		return presentationir.PlaceholderBody, placeholderIdx, true
	default:
		return presentationir.PlaceholderNone, 0, false
	}
}

func parseLegacyPptTextBoxFragments(data []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) ([]legacySlideFragment, int) {
	var frags []legacySlideFragment
	currentTextType := pptTextTypeUnknown
	currentOutlineRef := -1
	firstOutlineRef := -1
	for pos := 0; pos+8 <= len(data); {
		record, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			break
		}
		switch record.Type {
		case pptRTTextHeaderAtom:
			if len(payload) >= 4 {
				currentTextType = int(binary.LittleEndian.Uint32(payload[:4]))
			}
		case pptRTTextCharsAtom:
			raw := decodePptUTF16(payload)
			text := normalizePptText(raw)
			if text != "" {
				frags = append(frags, legacySlideFragment{Text: text, RawText: raw, TextType: currentTextType, OutlineRef: currentOutlineRef})
			}
			currentTextType = pptTextTypeUnknown
		case pptRTTextBytesAtom:
			raw := decodePptBytes(payload)
			text := normalizePptText(raw)
			if text != "" {
				frags = append(frags, legacySlideFragment{Text: text, RawText: raw, TextType: currentTextType, OutlineRef: currentOutlineRef})
			}
			currentTextType = pptTextTypeUnknown
		case pptRTStyleTextPropAtom:
			if n := len(frags); n > 0 {
				if paras := parseStyleTextPropAtomWithTheme(frags[n-1].RawText, payload, fontTable, colorScheme); paras != nil {
					frags[n-1].Paragraphs = paras
					if frags[n-1].pendingRuler != nil {
						applyRulerLevelsToParas(paras, *frags[n-1].pendingRuler)
						frags[n-1].pendingRuler = nil
					}
					// Apply any pending STPA9 auto-numbering
					if frags[n-1].pendingSTP9 != nil {
						if stpa9Data, ok := frags[n-1].pendingSTP9.(map[int]textPFException9); ok {
							applyStyleTextProp9ToParas(paras, stpa9Data)
							frags[n-1].pendingSTP9 = nil
						}
					}
				}
			}
		case pptRTStyleTextProp9Atom:
			if n := len(frags); n > 0 {
				if stpa9Data := parseStyleTextProp9Atom(payload); stpa9Data != nil {
					// Apply auto-numbering to paragraphs if they exist
					if paras := frags[n-1].Paragraphs; paras != nil {
						applyStyleTextProp9ToParas(paras, stpa9Data)
					} else {
						// Store for later application if paragraphs haven't been created yet
						frags[n-1].pendingSTP9 = stpa9Data
					}
				}
			}
		case pptRTTextRulerAtom:
			if n := len(frags); n > 0 {
				if ruler, ok := parseTextRulerAtom(payload); ok {
					if frags[n-1].Paragraphs != nil {
						applyRulerLevelsToParas(frags[n-1].Paragraphs, ruler)
					} else {
						frags[n-1].pendingRuler = &ruler
					}
				}
			}
		case pptRTOutlineTextRefAtom:
			if len(payload) >= 4 {
				currentOutlineRef = int(binary.LittleEndian.Uint32(payload[:4]))
				if firstOutlineRef < 0 {
					firstOutlineRef = currentOutlineRef
				}
				if currentTextType == pptTextTypeUnknown {
					currentTextType = currentOutlineRef
				}
			}
		case pptRTSlideNumberMCAtom, pptRTDateTimeMCAtom, pptRTGenericDateMCAtom, pptRTHeaderMCAtom, pptRTFooterMCAtom, pptRTRTFDateTimeMCAtom:
			if n := len(frags); n > 0 {
				if field, ok := legacyPptTextMetaFieldFromRecord(record.Type, payload); ok {
					frags[n-1].metaFields = append(frags[n-1].metaFields, field)
				}
			}
		case pptRTInteractiveInfoAtom:
			if n := len(frags); n > 0 {
				applyLegacyHyperlinkToFragment(&frags[n-1], parseLegacyPptInteractiveInfoURL(payload, exHyperlinks))
			}
		case pptRTInteractiveInfo:
			if n := len(frags); n > 0 {
				applyLegacyHyperlinkToFragment(&frags[n-1], parseLegacyPptInteractiveInfoURLFromContainer(payload, exHyperlinks))
			}
		}
		pos = next
	}
	for i := range frags {
		finalizeLegacyPptFragmentMetaFields(&frags[i])
	}
	// POI HSLFTextParagraph.findTextParagraphs: when an OutlineTextRefAtom is
	// present the inline ClientTextbox text is ignored entirely — the outline
	// ref index alone resolves the text from SlideListWithText. Discard any
	// inline frags so the caller uses only the outline-ref path.
	if firstOutlineRef >= 0 {
		return nil, firstOutlineRef
	}
	return frags, firstOutlineRef
}

func parseLegacyPptTextBoxFragmentsRecursive(data []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) ([]legacySlideFragment, int) {
	var frags []legacySlideFragment
	currentTextType := pptTextTypeUnknown
	currentOutlineRef := -1
	firstOutlineRef := -1
	walkPptRecords(data, func(record pptRecord, payload []byte) {
		switch record.Type {
		case pptRTTextHeaderAtom:
			if len(payload) >= 4 {
				currentTextType = int(binary.LittleEndian.Uint32(payload[:4]))
			}
		case pptRTTextCharsAtom:
			raw := decodePptUTF16(payload)
			text := normalizePptText(raw)
			if text != "" {
				frags = append(frags, legacySlideFragment{Text: text, RawText: raw, TextType: currentTextType, OutlineRef: currentOutlineRef})
			}
			currentTextType = pptTextTypeUnknown
		case pptRTTextBytesAtom:
			raw := decodePptBytes(payload)
			text := normalizePptText(raw)
			if text != "" {
				frags = append(frags, legacySlideFragment{Text: text, RawText: raw, TextType: currentTextType, OutlineRef: currentOutlineRef})
			}
			currentTextType = pptTextTypeUnknown
		case pptRTStyleTextPropAtom:
			if n := len(frags); n > 0 {
				if paras := parseStyleTextPropAtomWithTheme(frags[n-1].RawText, payload, fontTable, colorScheme); paras != nil {
					frags[n-1].Paragraphs = paras
					if frags[n-1].pendingRuler != nil {
						applyRulerLevelsToParas(paras, *frags[n-1].pendingRuler)
						frags[n-1].pendingRuler = nil
					}
					// Apply any pending STPA9 auto-numbering
					if frags[n-1].pendingSTP9 != nil {
						if stpa9Data, ok := frags[n-1].pendingSTP9.(map[int]textPFException9); ok {
							applyStyleTextProp9ToParas(paras, stpa9Data)
							frags[n-1].pendingSTP9 = nil
						}
					}
				}
			}
		case pptRTStyleTextProp9Atom:
			if n := len(frags); n > 0 {
				if stpa9Data := parseStyleTextProp9Atom(payload); stpa9Data != nil {
					// Apply auto-numbering to paragraphs if they exist
					if paras := frags[n-1].Paragraphs; paras != nil {
						applyStyleTextProp9ToParas(paras, stpa9Data)
					} else {
						// Store for later application if paragraphs haven't been created yet
						frags[n-1].pendingSTP9 = stpa9Data
					}
				}
			}
		case pptRTTextRulerAtom:
			if n := len(frags); n > 0 {
				if ruler, ok := parseTextRulerAtom(payload); ok {
					if frags[n-1].Paragraphs != nil {
						applyRulerLevelsToParas(frags[n-1].Paragraphs, ruler)
					} else {
						frags[n-1].pendingRuler = &ruler
					}
				}
			}
		case pptRTOutlineTextRefAtom:
			if len(payload) >= 4 {
				currentOutlineRef = int(binary.LittleEndian.Uint32(payload[:4]))
				if firstOutlineRef < 0 {
					firstOutlineRef = currentOutlineRef
				}
				if currentTextType == pptTextTypeUnknown {
					currentTextType = currentOutlineRef
				}
			}
		case pptRTSlideNumberMCAtom, pptRTDateTimeMCAtom, pptRTGenericDateMCAtom, pptRTHeaderMCAtom, pptRTFooterMCAtom, pptRTRTFDateTimeMCAtom:
			if n := len(frags); n > 0 {
				if field, ok := legacyPptTextMetaFieldFromRecord(record.Type, payload); ok {
					frags[n-1].metaFields = append(frags[n-1].metaFields, field)
				}
			}
		case pptRTInteractiveInfoAtom:
			if n := len(frags); n > 0 {
				applyLegacyHyperlinkToFragment(&frags[n-1], parseLegacyPptInteractiveInfoURL(payload, exHyperlinks))
			}
		case pptRTInteractiveInfo:
			if n := len(frags); n > 0 {
				applyLegacyHyperlinkToFragment(&frags[n-1], parseLegacyPptInteractiveInfoURLFromContainer(payload, exHyperlinks))
			}
		}
	})
	for i := range frags {
		finalizeLegacyPptFragmentMetaFields(&frags[i])
	}
	return frags, firstOutlineRef
}

func dominantLegacyPptTextType(frags []legacySlideFragment) int {
	for _, frag := range frags {
		if frag.TextType != pptTextTypeUnknown {
			return frag.TextType
		}
	}
	return pptTextTypeUnknown
}

func legacyPptFragmentsParagraphs(frags []legacySlideFragment) []*presentationir.Paragraph {
	var paragraphs []*presentationir.Paragraph
	for _, frag := range frags {
		paragraphs = append(paragraphs, fragmentParagraphs(frag)...)
	}
	return paragraphs
}

func legacyPptPlaceholderFromTextType(textType int, hasTitleContext bool) presentationir.PlaceholderKind {
	switch textType {
	case pptTextTypeTitle, pptTextTypeCenterTitle:
		return presentationir.PlaceholderTitle
	case pptTextTypeCenterBody:
		if hasTitleContext {
			return presentationir.PlaceholderSubtitle
		}
		return presentationir.PlaceholderBody
	case pptTextTypeBody, pptTextTypeUnknown:
		return presentationir.PlaceholderBody
	case pptTextTypeOther:
		// TextHeaderAtom OTHER_TYPE classifies ordinary shape text; it is not
		// evidence that the containing shape is a placeholder. Placeholder
		// ownership comes from OEPlaceholderAtom (or the narrow structural
		// master inference above), so keep slide-local "other" text ordinary.
		return presentationir.PlaceholderNone
	default:
		return presentationir.PlaceholderNone
	}
}

func consumeLegacyPptPlaceholderParagraphs(slideText legacySlideText, used []bool, placeholder presentationir.PlaceholderKind, hasTitleContext bool) []*presentationir.Paragraph {
	switch placeholder {
	case presentationir.PlaceholderTitle:
		return consumeLegacyPptTextTypeParagraphs(slideText, used, []int{pptTextTypeTitle, pptTextTypeCenterTitle})
	case presentationir.PlaceholderSubtitle:
		return consumeLegacyPptTextTypeParagraphs(slideText, used, []int{pptTextTypeCenterBody})
	case presentationir.PlaceholderBody:
		types := []int{pptTextTypeBody, pptTextTypeOther}
		if !hasTitleContext {
			types = append(types, pptTextTypeCenterBody)
		}
		return consumeLegacyPptTextTypeParagraphs(slideText, used, types)
	case presentationir.PlaceholderDateTime, presentationir.PlaceholderSlideNumber, presentationir.PlaceholderFooter:
		return consumeLegacyFooterParagraphs(slideText, used)
	default:
		return nil
	}
}

func consumeLegacyPptTextTypeParagraphs(slideText legacySlideText, used []bool, types []int) []*presentationir.Paragraph {
	var paragraphs []*presentationir.Paragraph
	for index, frag := range slideText.Fragments {
		if used[index] || frag.TextType == pptTextTypeNotes || frag.Text == "" {
			continue
		}
		if !legacyPptTextTypeMatches(frag.TextType, types) {
			continue
		}
		used[index] = true
		paragraphs = append(paragraphs, fragmentParagraphs(frag)...)
	}
	return paragraphs
}

func consumeLegacyPptOutlineRefParagraphs(slideText legacySlideText, used []bool, outlineRef int) []*presentationir.Paragraph {
	if outlineRef < 0 {
		return nil
	}
	for index, frag := range slideText.Fragments {
		if used[index] || frag.TextType == pptTextTypeNotes || frag.Text == "" {
			continue
		}
		if frag.OutlineRef != outlineRef {
			continue
		}
		used[index] = true
		return fragmentParagraphs(frag)
	}
	return nil
}

func consumeLegacyPptShapeIDParagraphs(slideText legacySlideText, used []bool, shapeID uint32) []*presentationir.Paragraph {
	if shapeID == 0 {
		return nil
	}
	for index, frag := range slideText.Fragments {
		if used[index] || frag.TextType == pptTextTypeNotes || frag.Text == "" {
			continue
		}
		if frag.ShapeID != shapeID {
			continue
		}
		used[index] = true
		return fragmentParagraphs(frag)
	}
	return nil
}

func previewLegacyPptOutlineRefParagraphs(slideText legacySlideText, used []bool, outlineRef int) []*presentationir.Paragraph {
	if outlineRef < 0 {
		return nil
	}
	for index, frag := range slideText.Fragments {
		if used[index] || frag.TextType == pptTextTypeNotes || frag.Text == "" {
			continue
		}
		if frag.OutlineRef != outlineRef {
			continue
		}
		return fragmentParagraphs(frag)
	}
	return nil
}

func previewLegacyPptShapeIDParagraphs(slideText legacySlideText, used []bool, shapeID uint32) []*presentationir.Paragraph {
	if shapeID == 0 {
		return nil
	}
	for index, frag := range slideText.Fragments {
		if used[index] || frag.TextType == pptTextTypeNotes || frag.Text == "" {
			continue
		}
		if frag.ShapeID != shapeID {
			continue
		}
		return fragmentParagraphs(frag)
	}
	return nil
}

func consumeLegacyFooterParagraphs(slideText legacySlideText, used []bool) []*presentationir.Paragraph {
	var paragraphs []*presentationir.Paragraph
	for index, frag := range slideText.Fragments {
		if used[index] || frag.TextType == pptTextTypeNotes || frag.Text == "" {
			continue
		}
		if frag.TextType != pptTextTypeOther && frag.TextType != pptTextTypeUnknown {
			continue
		}
		if !isLikelyLegacyFooterLine(frag.Text) {
			continue
		}
		used[index] = true
		paragraphs = append(paragraphs, fragmentParagraphs(frag)...)
	}
	return paragraphs
}

func legacyPptTextTypeMatches(textType int, want []int) bool {
	for _, candidate := range want {
		if textType == candidate {
			return true
		}
	}
	return false
}

func markLegacySlideFragmentsByParagraphs(slideText legacySlideText, used []bool, paragraphs []*presentationir.Paragraph) bool {
	matched := false
	for _, paragraph := range paragraphs {
		text := paragraphText(paragraph)
		if text == "" {
			continue
		}
		for index, frag := range slideText.Fragments {
			if used[index] || frag.Text == "" {
				continue
			}
			if normalizePptText(frag.Text) == text {
				used[index] = true
				matched = true
				break
			}
		}
	}
	return matched
}

func legacyPptLooksLikePlaceholderPromptParagraphs(paragraphs []*presentationir.Paragraph) bool {
	if len(paragraphs) == 0 {
		return false
	}
	for _, paragraph := range paragraphs {
		if !legacyPptLooksLikePlaceholderPromptText(paragraphText(paragraph)) {
			return false
		}
	}
	return true
}

func legacyPptLooksLikePlaceholderPromptText(text string) bool {
	text = strings.TrimSpace(strings.ToLower(text))
	return text != "" && (strings.HasPrefix(text, "click to edit ") || strings.HasPrefix(text, "click to add "))
}

func legacyPptLooksLikePlaceholderSampleText(text string) bool {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return false
	}
	if legacyPptLooksLikePlaceholderPromptText(text) {
		return true
	}
	switch text {
	case "second level", "third level", "fourth level", "fifth level", "*":
		return true
	default:
		return false
	}
}

func legacyPptSlideHasOnlyPlaceholderPrompts(slideText legacySlideText) bool {
	if len(slideText.Fragments) == 0 {
		return false
	}
	for _, frag := range slideText.Fragments {
		if frag.Text == "" {
			continue
		}
		if !legacyPptLooksLikePlaceholderSampleText(frag.Text) {
			return false
		}
	}
	return true
}

func markLegacySlideFragmentByOutlineRef(slideText legacySlideText, used []bool, outlineRef int) bool {
	if outlineRef < 0 {
		return false
	}
	for index, frag := range slideText.Fragments {
		if used[index] || frag.Text == "" {
			continue
		}
		if frag.OutlineRef != outlineRef {
			continue
		}
		used[index] = true
		return true
	}
	return false
}

func markLegacySlideFragmentByShapeID(slideText legacySlideText, used []bool, shapeID uint32) bool {
	if shapeID == 0 {
		return false
	}
	for index, frag := range slideText.Fragments {
		if used[index] || frag.Text == "" {
			continue
		}
		if frag.ShapeID != shapeID {
			continue
		}
		used[index] = true
		return true
	}
	return false
}

func collectUnusedLegacySlideFragments(slideText legacySlideText, used []bool) legacySlideText {
	out := legacySlideText{PersistID: slideText.PersistID}
	for index, frag := range slideText.Fragments {
		if used[index] || frag.TextType == pptTextTypeNotes || frag.Text == "" || legacyPptLooksLikePlaceholderSampleText(frag.Text) {
			continue
		}
		out.Fragments = append(out.Fragments, frag)
	}
	return out
}

func cloneLegacyParagraphs(paragraphs []*presentationir.Paragraph) []*presentationir.Paragraph {
	if len(paragraphs) == 0 {
		return nil
	}
	out := make([]*presentationir.Paragraph, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if paragraph == nil {
			continue
		}
		clone := *paragraph
		if len(paragraph.Runs) > 0 {
			clone.Runs = append([]*presentationir.Run(nil), paragraph.Runs...)
		}
		out = append(out, &clone)
	}
	return out
}

func legacyPptTextShapeName(index int, placeholder presentationir.PlaceholderKind) string {
	switch placeholder {
	case presentationir.PlaceholderTitle:
		return fmt.Sprintf("Legacy Title Shape %d", index)
	case presentationir.PlaceholderSubtitle:
		return fmt.Sprintf("Legacy Subtitle Shape %d", index)
	case presentationir.PlaceholderDateTime:
		return fmt.Sprintf("Legacy Date Shape %d", index)
	case presentationir.PlaceholderSlideNumber:
		return fmt.Sprintf("Legacy Slide Number Shape %d", index)
	case presentationir.PlaceholderFooter:
		return fmt.Sprintf("Legacy Footer Shape %d", index)
	default:
		return fmt.Sprintf("Legacy Text Shape %d", index)
	}
}

func parseLegacyPptPictureStoreFromPayload(payload []byte, picturesStream []byte) map[uint32]*legacyPptPicture {
	if len(picturesStream) == 0 || len(payload) == 0 {
		return nil
	}

	resolver := func(offset, size uint32) []byte {
		start := uint64(offset)
		end := start + uint64(size)
		if end > uint64(len(picturesStream)) || start >= end {
			return nil
		}
		return picturesStream[int(start):int(end)]
	}

	var store map[uint32]*legacyPptPicture
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTPPDrawingGroup {
			shared := officeartlegacy.ParseBStore(childPayload, resolver)
			if len(shared) > 0 {
				store = make(map[uint32]*legacyPptPicture, len(shared))
				for index, entry := range shared {
					if len(entry.Media.Data) == 0 || entry.Media.MIME == "" {
						continue
					}
					store[index] = &legacyPptPicture{
						Data: append([]byte(nil), entry.Media.Data...),
						MIME: entry.Media.MIME,
					}
				}
			}
		}
		pos = next
	}
	if len(store) == 0 {
		return nil
	}
	return store
}

func extractLegacyPptOfficeArtBlipImage(recType uint16, payload []byte) ([]byte, string) {
	media := officeartlegacy.ExtractBLIP(recType, payload)
	return media.Data, media.MIME
}

// extractLegacyPptOfficeArtBlipPICT decompresses a Macintosh PICT BLIP payload.
// Layout per MS-ODRAW / HSLF PICT.java:
//
//	bytes 0..15  — UID checksum (16 bytes for single-UID instance; some files
//	               use 32 bytes for dual-UID instance, which we also try)
//	bytes N..N+33 — Metafile.Header (34 bytes: wmfSize[4] + bounds[16] + size[8] +
//	                zipSize[4] + compression[1] + filter[1])
//	rest         — zlib-compressed PICT data (no 512-byte Mac header)
//
// We inflate the compressed stream and prepend the 512-byte QuickDraw header
// (all zero) that POI injects in PICT.getData(), so consumers see a complete
// PICT file.
func extractLegacyPptOfficeArtBlipPICT(payload []byte) []byte {
	media := officeartlegacy.ExtractBLIP(legacyOfficeArtBlipPICT, payload)
	return media.Data
}

// bindLegacyPptMasterOwnership resolves the two master identities carried by PPT:
// SlideAtom references a SlideIdentifier, while persisted master payloads are opened by
// RefID. Keep both identities on the slide so later semantic stages never have to infer
// persist ownership from ordering or from whichever master happened to be decoded first.
func bindLegacyPptMasterOwnership(slides []legacySlideText, entries []legacyPptMasterEntry) []legacySlideText {
	if len(slides) == 0 || len(entries) == 0 {
		return slides
	}
	byIdentifier := make(map[int]int, len(entries))
	for _, entry := range entries {
		if entry.RefID == 0 {
			continue
		}
		// SlideListWithText is authored order. Preserve the first binding for a
		// duplicated identifier instead of letting a later malformed entry silently
		// retarget already-authored slides.
		if _, exists := byIdentifier[entry.SlideIdentifier]; !exists {
			byIdentifier[entry.SlideIdentifier] = entry.RefID
		}
	}
	for i := range slides {
		if refID, ok := byIdentifier[slides[i].MasterID]; ok {
			slides[i].MasterPersistID = refID
		}
	}
	return slides
}

// collectLegacyPptMasterVisualDispatches is the master-side counterpart to the
// slide visual boundary. Each persisted master drawing is traversed exactly once;
// every downstream master semantic projection consumes this shared event scene.
func collectLegacyPptMasterVisualScenes(events legacyPptPersistEventIndex, entries []legacyPptMasterEntry) map[int]legacyPptVisualScene {
	if len(events.masters) == 0 || len(entries) == 0 {
		return nil
	}
	out := make(map[int]legacyPptVisualScene, len(entries))
	for _, entry := range entries {
		e, ok := events.masters[entry.RefID]
		if !ok || len(e.Payload) == 0 {
			continue
		}
		scene := collectLegacyPptVisualScene(e.Payload)
		if !scene.empty() {
			out[entry.RefID] = scene
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseLegacyPptMasterPlaceholderTemplatesFromEvents captures the authored placeholder
// geometry/paint owned by each master from the same persist + OfficeArt event boundary
// used by slide visuals. Placeholder prompt text is never materialized as slide content;
// the template only supplies inherited visual properties to a slide-local placeholder.
// legacyPptMasterComposition is the single BIG37 master semantic product.
// Placeholder templates and painted artwork are two projections of the same
// already-decoded master scenes; production must not rediscover either family
// from the PowerPoint stream or independently collect OfficeArt.
type legacyPptMasterComposition struct {
	Visuals              map[int][]*presentationir.Shape // keyed by master persist-directory RefID
	PlaceholderTemplates map[int]map[presentationir.PlaceholderKind]legacyPptTextShape
}

// legacyPptMasterPlaceholderKind classifies structural master text before the
// master scene is split into template semantics and painted artwork. Some PPTs
// carry an unknown/unsupported OEPlaceholderAtom but retain an authoritative
// TextHeaderAtom role; prompt-bearing master body text is likewise structural.
// Keeping this decision here prevents a placeholder SpContainer from re-entering
// the effective slide as an ordinary generic/text master visual.
func legacyPptMasterPlaceholderKind(meta legacyPptTextShape, hasTitleContext bool) presentationir.PlaceholderKind {
	if meta.placeholder != presentationir.PlaceholderNone {
		return meta.placeholder
	}
	switch meta.textType {
	case pptTextTypeTitle, pptTextTypeCenterTitle:
		return presentationir.PlaceholderTitle
	case pptTextTypeCenterBody:
		if hasTitleContext || legacyPptLooksLikePlaceholderPromptParagraphs(meta.paragraphs) || legacyPptLooksLikePlaceholderPromptParagraphs(meta.inlineFallback) {
			return legacyPptPlaceholderFromTextType(meta.textType, hasTitleContext)
		}
	case pptTextTypeBody:
		if legacyPptLooksLikePlaceholderPromptParagraphs(meta.paragraphs) || legacyPptLooksLikePlaceholderPromptParagraphs(meta.inlineFallback) {
			return presentationir.PlaceholderBody
		}
	}
	return presentationir.PlaceholderNone
}

func composeLegacyPptMasterSemanticScenes(
	scenes map[int]legacyPptVisualScene,
	entries []legacyPptMasterEntry,
	fontTable map[uint16]string,
	masterSchemes map[int]map[uint8]string,
	fallbackScheme map[uint8]string,
	exHyperlinks map[uint32]string,
	pictureStore map[uint32]*legacyPptPicture,
	externalObjects map[uint32]legacyPptExternalObject,
) legacyPptMasterComposition {
	if len(scenes) == 0 || len(entries) == 0 {
		return legacyPptMasterComposition{}
	}
	composition := legacyPptMasterComposition{
		Visuals:              make(map[int][]*presentationir.Shape),
		PlaceholderTemplates: make(map[int]map[presentationir.PlaceholderKind]legacyPptTextShape),
	}
	for _, entry := range entries {
		scene, ok := scenes[entry.RefID]
		if !ok || scene.empty() {
			continue
		}
		scheme := masterSchemes[entry.SlideIdentifier]
		if len(scheme) == 0 {
			scheme = fallbackScheme
		}

		placeholderShapeIDs := make(map[uint32]struct{})
		masterTextMetas := scene.textMetas(fontTable, scheme, exHyperlinks)
		hasTitleContext := false
		for _, meta := range masterTextMetas {
			if meta.placeholder == presentationir.PlaceholderTitle || meta.textType == pptTextTypeTitle || meta.textType == pptTextTypeCenterTitle {
				hasTitleContext = true
				break
			}
		}
		for _, meta := range masterTextMetas {
			kind := legacyPptMasterPlaceholderKind(meta, hasTitleContext)
			if kind == presentationir.PlaceholderNone {
				continue
			}
			if meta.shapeID != 0 {
				placeholderShapeIDs[meta.shapeID] = struct{}{}
			}
			byKind := composition.PlaceholderTemplates[entry.SlideIdentifier]
			if byKind == nil {
				byKind = make(map[presentationir.PlaceholderKind]legacyPptTextShape)
				composition.PlaceholderTemplates[entry.SlideIdentifier] = byKind
			}
			if _, exists := byKind[kind]; !exists {
				meta.placeholder = kind
				// Keep the authored placeholder paragraph formatting in the template.
				// The template is never painted as slide content; synthesized header/footer
				// shapes reuse its geometry, alignment, font metrics and text-body anchor.
				// Prompt/sample text remains template-owned and is replaced at synthesis.
				byKind[kind] = meta
			}
		}

		projection := scene.completeProjection(legacySlideText{}, fontTable, scheme, nil, exHyperlinks, pictureStore, externalObjects)
		projected := projection.Shapes
		visuals := projected[:0]
		for _, shape := range projected {
			if shape == nil || shape.Placeholder != presentationir.PlaceholderNone {
				continue
			}
			// Placeholder OfficeArt can survive the text projection as a generic
			// visual when its sample text is intentionally suppressed. It is still
			// a master template, not painted artwork. Classify by authored ShapeID,
			// not by the fallback projection's Placeholder field.
			if _, isPlaceholderTemplate := placeholderShapeIDs[shape.OriginalShapeID]; isPlaceholderTemplate {
				continue
			}
			// A master drawing can contain ClientTextbox containers that are semantic
			// scaffolding only: no text, no fill and no stroke.  They are useful while
			// resolving placeholder/text inheritance, but they are not painted master
			// artwork.  Materializing them on every slide is especially harmful when
			// an old SmallRectStruct carries a very large (but syntactically valid)
			// anchor: the invisible box expands the HTML/layout scene by several slide
			// widths.  Keep the source anchor intact and make the ownership decision at
			// the master semantic boundary instead of clipping geometry downstream.
			if !legacyPptShapePaintsMasterArtwork(shape) {
				continue
			}
			visuals = append(visuals, shape)
		}
		if len(visuals) != 0 {
			composition.Visuals[entry.RefID] = visuals
		}
	}
	if len(composition.Visuals) == 0 {
		composition.Visuals = nil
	}
	if len(composition.PlaceholderTemplates) == 0 {
		composition.PlaceholderTemplates = nil
	}
	return composition
}

func inheritLegacyPptMasterPlaceholderTemplates(metas []legacyPptTextShape, templates map[presentationir.PlaceholderKind]legacyPptTextShape) []legacyPptTextShape {
	if len(metas) == 0 || len(templates) == 0 {
		return metas
	}
	for i := range metas {
		kind := metas[i].placeholder
		if kind == presentationir.PlaceholderNone {
			continue
		}
		template, ok := templates[kind]
		if !ok {
			continue
		}
		// A slide anchor is authoritative. Missing/degenerate local geometry inherits
		// the master anchor instead of falling back to renderer-era role heuristics.
		if metas[i].bounds.W <= 0 || metas[i].bounds.H <= 0 {
			metas[i].bounds = template.bounds
		}
		if metas[i].fillColor == "" {
			metas[i].fillColor = template.fillColor
		}
		metas[i].transform = mergeLegacyPptPlaceholderTransform(metas[i].transform, template.transform)
	}
	return metas
}

func normalizeLegacyTextInsetPresence(t *legacyPptShapeTransform) {
	if t == nil || !t.HasTextInsets {
		return
	}
	if t.HasTextInsetLeft || t.HasTextInsetTop || t.HasTextInsetRight || t.HasTextInsetBottom {
		return
	}
	t.HasTextInsetLeft, t.HasTextInsetTop, t.HasTextInsetRight, t.HasTextInsetBottom = true, true, true, true
}

func mergeLegacyPptPlaceholderTransform(local, inherited legacyPptShapeTransform) legacyPptShapeTransform {
	// Compatibility for transforms constructed before per-edge provenance was
	// introduced: HasTextInsets historically meant all four values were owned.
	normalizeLegacyTextInsetPresence(&local)
	normalizeLegacyTextInsetPresence(&inherited)
	if !local.HasRotation && inherited.HasRotation {
		local.HasRotation, local.Rotation = true, inherited.Rotation
	}
	if !local.FlipH {
		local.FlipH = inherited.FlipH
	}
	if !local.FlipV {
		local.FlipV = inherited.FlipV
	}
	if local.LineColor == "" {
		local.LineColor = inherited.LineColor
	}
	if local.LineWidthEMU == 0 {
		local.LineWidthEMU = inherited.LineWidthEMU
	}
	if local.LineDash == "" {
		local.LineDash = inherited.LineDash
	}
	if !local.HasLineEnable && inherited.HasLineEnable {
		local.HasLineEnable, local.LineEnabled = true, inherited.LineEnabled
	}
	// Placeholder text-body semantics are part of the master's authored visual
	// contract just like its anchor and stroke. A slide placeholder only
	// overrides properties it explicitly owns; otherwise keep the master's
	// vertical anchor, margins and overflow/autofit behavior.
	if !local.HasTextAnchor && inherited.HasTextAnchor {
		local.HasTextAnchor, local.TextAnchor = true, inherited.TextAnchor
	}
	if !local.HasAnchorCenter && inherited.HasAnchorCenter {
		local.HasAnchorCenter, local.AnchorCenter = true, inherited.AnchorCenter
	}
	if !local.HasTextWrap && inherited.HasTextWrap {
		local.HasTextWrap, local.TextWrap = true, inherited.TextWrap
	}
	if !local.HasTextVertical && inherited.HasTextVertical {
		local.HasTextVertical, local.TextVertical = true, inherited.TextVertical
	}
	// Text insets are independently authored OfficeArt properties.  Merge each
	// edge separately: a slide placeholder may override only its left inset and
	// still inherit the other three edges from the master.
	if !local.HasTextInsetLeft && inherited.HasTextInsetLeft {
		local.HasTextInsetLeft, local.TextInsetLeft = true, inherited.TextInsetLeft
	}
	if !local.HasTextInsetTop && inherited.HasTextInsetTop {
		local.HasTextInsetTop, local.TextInsetTop = true, inherited.TextInsetTop
	}
	if !local.HasTextInsetRight && inherited.HasTextInsetRight {
		local.HasTextInsetRight, local.TextInsetRight = true, inherited.TextInsetRight
	}
	if !local.HasTextInsetBottom && inherited.HasTextInsetBottom {
		local.HasTextInsetBottom, local.TextInsetBottom = true, inherited.TextInsetBottom
	}
	local.HasTextInsets = local.HasTextInsetLeft || local.HasTextInsetTop || local.HasTextInsetRight || local.HasTextInsetBottom
	if !local.HasAllowOverflow && inherited.HasAllowOverflow {
		local.HasAllowOverflow, local.AllowOverflow = true, inherited.AllowOverflow
	}
	if !local.HasAutoGrow && inherited.HasAutoGrow {
		local.HasAutoGrow, local.AutoGrow = true, inherited.AutoGrow
	}
	if !local.HasGeometryText && inherited.HasGeometryText {
		local.HasGeometryText, local.GeometryText = true, inherited.GeometryText
	}
	if !local.HasImageCrop && inherited.HasImageCrop {
		local.HasImageCrop = true
		local.CropTop, local.CropBottom = inherited.CropTop, inherited.CropBottom
		local.CropLeft, local.CropRight = inherited.CropLeft, inherited.CropRight
	}
	if local.ArrowHead == "" {
		local.ArrowHead = inherited.ArrowHead
	}
	if local.ArrowHeadWidth == "" {
		local.ArrowHeadWidth = inherited.ArrowHeadWidth
	}
	if local.ArrowHeadLength == "" {
		local.ArrowHeadLength = inherited.ArrowHeadLength
	}
	if local.ArrowTail == "" {
		local.ArrowTail = inherited.ArrowTail
	}
	if local.ArrowTailWidth == "" {
		local.ArrowTailWidth = inherited.ArrowTailWidth
	}
	if local.ArrowTailLength == "" {
		local.ArrowTailLength = inherited.ArrowTailLength
	}
	return local
}

func legacyPptShapeContainsOnlyPlaceholderPromptText(shape *presentationir.Shape) bool {
	if shape == nil || len(shape.Paragraphs) == 0 {
		return false
	}
	sawText := false
	for _, paragraph := range shape.Paragraphs {
		if paragraph == nil {
			continue
		}
		text := strings.TrimSpace(paragraphText(paragraph))
		if text == "" {
			continue
		}
		sawText = true
		if !legacyPptLooksLikePlaceholderSampleText(text) {
			return false
		}
	}
	return sawText
}

func legacyPptShapePaintsMasterArtwork(shape *presentationir.Shape) bool {
	if shape == nil {
		return false
	}
	// Master sample/prompt text is inheritance scaffolding, not painted artwork.
	// Some legacy producers omit placeholder metadata on these ClientTextbox shapes,
	// so classify the text itself at the semantic master boundary rather than letting
	// it materialize as an extra slide-local object.
	if legacyPptShapeContainsOnlyPlaceholderPromptText(shape) {
		return false
	}
	switch shape.Kind {
	case presentationir.ShapeImage, presentationir.ShapeChart, presentationir.ShapeTable, presentationir.ShapeConnector:
		return true
	case presentationir.ShapeGroup:
		for _, child := range shape.Children {
			if legacyPptShapePaintsMasterArtwork(child) {
				return true
			}
		}
		return false
	}
	for _, paragraph := range shape.Paragraphs {
		if paragraph == nil {
			continue
		}
		for _, run := range paragraph.Runs {
			if run != nil && (run.Text != "" || run.MathML != "" || run.FieldKind != "") {
				return true
			}
		}
	}
	fillPaints := shape.Fill != nil
	switch shape.Fill.(type) {
	case drawing.NoFill, *drawing.NoFill:
		fillPaints = false
	}
	if fillPaints || shape.FillColor != "" || shape.LineStyle != nil || shape.LineColor != "" || shape.LineWidthPt > 0 {
		return true
	}
	return false
}

// inheritLegacyPptMasterVisualsForSlide materializes the painted master scene for
// one effective slide. The master scene remains immutable/template-owned; every
// referencing slide receives independent canonical shape instances.
func inheritLegacyPptMasterVisualsForSlide(slide legacySlideText, masterVisualShapesByPersistID map[int][]*presentationir.Shape) []*presentationir.Shape {
	if len(masterVisualShapesByPersistID) == 0 || slide.MasterPersistID == 0 {
		return nil
	}
	if slide.HasSlideFlags && !slide.FollowMasterObjects {
		return nil
	}
	inherited := masterVisualShapesByPersistID[slide.MasterPersistID]
	if len(inherited) == 0 {
		return nil
	}
	out := make([]*presentationir.Shape, 0, len(inherited))
	for _, masterShape := range inherited {
		if clone := cloneLegacyShapeRecursive(masterShape, nil); clone != nil {
			out = append(out, clone)
		}
	}
	return out
}

func composeLegacyPptMasterVisuals(slides []legacySlideText, slideShapes map[int][]*presentationir.Shape, masterVisualShapesByPersistID map[int][]*presentationir.Shape) map[int][]*presentationir.Shape {
	if len(slides) == 0 || len(masterVisualShapesByPersistID) == 0 {
		return slideShapes
	}
	if slideShapes == nil {
		slideShapes = make(map[int][]*presentationir.Shape)
	}
	for _, slide := range slides {
		inherited := inheritLegacyPptMasterVisualsForSlide(slide, masterVisualShapesByPersistID)
		if len(inherited) == 0 {
			continue
		}
		local := slideShapes[slide.PersistID]
		composed := make([]*presentationir.Shape, 0, len(inherited)+len(local))
		composed = append(composed, inherited...)
		composed = append(composed, local...)
		slideShapes[slide.PersistID] = composed
	}
	return slideShapes
}

// collectLegacyPptTableShapesFromSceneForSlideWithOwnership reports the exact
// scene group paths that successfully materialized as tables. The path is the
// authoritative semantic owner for descendant grid-line SpContainers; recovered
// tables cannot always provide a stable group-root ShapeID.
func collectLegacyPptTableShapesFromSceneForSlideWithOwnership(scene legacyPptShapeContainerScene, frame legacyPptGroupFrame, slideText legacySlideText, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, shapes *[]*presentationir.Shape, projectedTablePaths *[][]int) {
	var ownedTablePaths [][]int
	for _, group := range scene.shapeGroups() {
		if legacyPptSkipTableGroupProjection(group, ownedTablePaths) {
			continue
		}
		tableShape, ok := parseLegacyPptTableShapeFromSceneGroupForSlide(scene, group, slideText, fontTable, colorScheme, exHyperlinks, len(*shapes)+1)
		if !ok && legacyPptSceneGroupMayNeedTableRecovery(scene, group) {
			// The scene projection is authoritative when the ShapeNode topology is
			// complete. Damaged PPTs can, however, preserve the raw SpgrContainer's
			// table marker / ClientTextbox records while the decoded scene loses a
			// usable child anchor. Recovery is permitted only when this group itself
			// carries table evidence; never flatten nested diagram groups to invent it.
			tableShape, ok = recoverLegacyPptTableShapeFromGroupContainer(
				group.Event.Payload, group.ParentFrame, fontTable, colorScheme, exHyperlinks, len(*shapes)+1,
			)
			if ok {
				hydrateLegacyPptRecoveredTableText(tableShape, slideText)
			}
		}
		if ok {
			*shapes = append(*shapes, tableShape)
			ownedPath := append([]int(nil), group.Path...)
			ownedTablePaths = append(ownedTablePaths, ownedPath)
			if projectedTablePaths != nil {
				*projectedTablePaths = append(*projectedTablePaths, append([]int(nil), ownedPath...))
			}
		}
	}
}

func legacyPptSceneGroupMayNeedTableRecovery(scene legacyPptShapeContainerScene, group legacyPptShapeGroup) bool {
	if legacyPptGroupContainerIsTable(group.Event.Payload) {
		return true
	}
	if !legacyPptGroupMayOwnImplicitTable(group) {
		return false
	}
	// Recovery for an unmarked table must be justified by semantics authored
	// directly in this group. Never recursively flatten nested ordinary groups: a
	// pair of independent box stacks connected by arrows is a diagram, not a table.
	var cells []legacyPptTableCellShape
	var gridLines []presentationir.Rect
	for _, container := range scene.containersDirectlyUnder(group.Path) {
		if container.Node != nil && container.Node.ShapeType == 20 {
			if line, ok := parseLegacyPptConnectorShapeContainer(container.Event.Payload, group.Frame, nil); ok {
				gridLines = append(gridLines, line.bounds)
			}
			continue
		}
		shape, ok := parseLegacyPptTextShapeContainer(container.Event.Payload, group.Frame, nil, nil, nil)
		if !ok {
			continue
		}
		paragraphs := shape.paragraphs
		if len(paragraphs) == 0 {
			paragraphs = shape.inlineFallback
		}
		cells = append(cells, legacyPptTableCellShape{bounds: shape.bounds, paragraphs: paragraphs, outlineRef: shape.outlineRef})
	}
	return legacyPptSceneGroupOwnsTable(false, cells, gridLines)
}

func legacyPptGroupMayOwnImplicitTable(group legacyPptShapeGroup) bool {
	// Canonical ShapeTable requires authored table semantics. A geometric lattice
	// alone is not a semantic table: legacy diagrams, requirement matrices and
	// chart-like groups routinely contain aligned text boxes and grid lines.
	// Without groupshape.tableProperties, preserve the authored group/shape tree.
	// This also keeps PPT and PPTX conversion paths semantically aligned instead
	// of changing shape kind from Group to Table based on visual coincidence.
	_ = group
	return false
}

func legacyPptSkipTableGroupProjection(group legacyPptShapeGroup, ownedTablePaths [][]int) bool {
	if !legacyPptGroupPathOwned(group.Path, ownedTablePaths) {
		return false
	}
	// An already-materialized ancestor may suppress ordinary nested groups, but
	// an explicitly marked nested table is an independent semantic owner. The
	// ancestor table parser already excludes nested explicit-table descendants,
	// so skipping the nested group here would lose a real authored table while
	// making the ownership ledger report a false materialization gap.
	return !legacyPptGroupContainerIsTable(group.Event.Payload)
}

func hydrateLegacyPptRecoveredTableText(tableShape *presentationir.Shape, slideText legacySlideText) {
	if tableShape == nil || tableShape.Table == nil || len(slideText.Fragments) == 0 {
		return
	}
	// Compatibility recovery can reconstruct geometry from a damaged group while
	// the cell text survives only in SlideListWithText. Fill empty cells in authored
	// fragment order; never overwrite text already decoded from ClientTextbox.
	fragment := 0
	for _, row := range tableShape.Table.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || len(legacyPptParagraphProjectionKeys(cell.Paragraphs)) != 0 {
				continue
			}
			for fragment < len(slideText.Fragments) {
				f := slideText.Fragments[fragment]
				fragment++
				if len(f.Paragraphs) == 0 || legacyPptLooksLikePlaceholderPromptParagraphs(f.Paragraphs) {
					continue
				}
				cell.Paragraphs = cloneLegacyParagraphs(f.Paragraphs)
				break
			}
		}
	}
}

// parseLegacyPptTableShapeFromSceneGroup classifies and materializes a table from
// the scene-owned topology. It deliberately does not recursively walk the group's
// raw SpgrContainer payload; descendants and coordinate frames were established
// once while the shape-container scene was decoded.
func parseLegacyPptTableShapeFromSceneGroup(scene legacyPptShapeContainerScene, group legacyPptShapeGroup, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, ordinal int) (*presentationir.Shape, bool) {
	return parseLegacyPptTableShapeFromSceneGroupForSlide(scene, group, legacySlideText{}, fontTable, colorScheme, exHyperlinks, ordinal)
}

func parseLegacyPptTableShapeFromSceneGroupForSlide(scene legacyPptShapeContainerScene, group legacyPptShapeGroup, slideText legacySlideText, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, ordinal int) (*presentationir.Shape, bool) {
	if legacyPptOfficeArtContainsExternalObject(group.Event.Payload) {
		return nil, false
	}
	var cells []legacyPptTableCellShape
	var gridLines []presentationir.Rect
	explicitTable := legacyPptGroupContainerIsTable(group.Event.Payload)
	if !explicitTable && !legacyPptGroupMayOwnImplicitTable(group) {
		return nil, false
	}
	containers := scene.containersUnder(group.Path)
	if !explicitTable {
		// Implicit table inference is local to one authored SpgrContainer. Flattening
		// nested ordinary groups can merge independent box stacks and the connectors
		// between them into a fictitious table (42485.ppt). Explicit table groups may
		// still consume their authored descendant topology.
		containers = scene.containersDirectlyUnder(group.Path)
	}
	for _, container := range containers {
		// BIG36 ownership rule: table topology consumes scene-owned descendants,
		// but a nested group that authors its own tableProperties marker owns its
		// subtree independently. Do not let that nested table promote or feed an
		// ancestor composition table. Structural membership still comes entirely
		// from the decoded scene; this is semantic ownership partitioning, not a
		// recursive raw-OfficeArt walk.
		if legacyPptSceneContainerOwnedByNestedExplicitTable(scene, group.Path, container.GroupPath) {
			continue
		}
		frame := group.Frame
		for _, nested := range scene.shapeGroups() {
			if legacyPptGroupPathEqual(nested.Path, container.GroupPath) {
				frame = nested.Frame
				break
			}
		}
		body := container.Event.Payload
		if container.Node != nil && container.Node.ShapeType == 20 {
			if line, ok := parseLegacyPptConnectorShapeContainer(body, frame, colorScheme); ok {
				gridLines = append(gridLines, line.bounds)
			} else {
				gridLines = append(gridLines, presentationir.Rect{})
			}
			continue
		}
		shape, ok := parseLegacyPptTextShapeContainer(body, frame, fontTable, colorScheme, exHyperlinks)
		if !ok {
			continue
		}
		paragraphs := shape.paragraphs
		if len(paragraphs) == 0 {
			paragraphs = shape.inlineFallback
		}
		cells = append(cells, legacyPptTableCellShape{bounds: shape.bounds, paragraphs: cloneLegacyParagraphs(paragraphs), fillColor: shape.fillColor, outlineRef: shape.outlineRef})
	}
	if len(cells) == 0 {
		return nil, false
	}
	// In binary PPT the cell's ClientTextbox can carry only OutlineTextRefAtom;
	// the authored text itself lives in SlideListWithText. Hydrate those cells at
	// the semantic scene boundary before table ownership/building, rather than
	// letting the generic text projector own the same content separately.
	if len(slideText.Fragments) != 0 {
		used := make([]bool, len(slideText.Fragments))
		for i := range cells {
			if len(cells[i].paragraphs) != 0 || cells[i].outlineRef < 0 {
				continue
			}
			if paragraphs := previewLegacyPptOutlineRefParagraphs(slideText, used, cells[i].outlineRef); len(paragraphs) != 0 {
				cells[i].paragraphs = cloneLegacyParagraphs(paragraphs)
				markLegacySlideFragmentByOutlineRef(slideText, used, cells[i].outlineRef)
			}
		}
	}
	if !legacyPptSceneGroupOwnsTable(explicitTable, cells, gridLines) {
		return nil, false
	}
	table, bounds, ok := buildLegacyPptTableFromCells(cells, group.Frame.target)
	if !ok || table == nil || len(table.Rows) == 0 {
		return nil, false
	}
	groupShapeID, _ := legacyPptGroupRootShapeID(group.Event.Payload)
	return &presentationir.Shape{Kind: presentationir.ShapeTable, Name: fmt.Sprintf("Legacy Table %d", ordinal), OriginalShapeID: groupShapeID, Bounds: bounds, Table: table}, true
}

func legacyPptSceneContainerOwnedByNestedExplicitTable(scene legacyPptShapeContainerScene, ownerPath, containerPath []int) bool {
	for _, nested := range scene.shapeGroups() {
		if len(nested.Path) <= len(ownerPath) || !legacyPptGroupPathHasPrefix(nested.Path, ownerPath) {
			continue
		}
		if !legacyPptGroupPathHasPrefix(containerPath, nested.Path) {
			continue
		}
		if legacyPptGroupContainerIsTable(nested.Event.Payload) {
			return true
		}
	}
	return false
}

// legacyPptSceneGroupOwnsTable is the production table-ownership gate for the
// event-decoded OfficeArt scene. A scene group must carry authored table semantics
// or prove a coherent cell lattice. Merely containing connector/line shapes is not
// table evidence: diagrams and charts routinely contain both text boxes and lines.
// The older permissive line-presence fallback remains confined to compatibility
// recovery and must not steal ordinary group ownership in the semantic scene.
func legacyPptSceneGroupOwnsTable(explicit bool, cells []legacyPptTableCellShape, gridLines []presentationir.Rect) bool {
	if explicit {
		return true
	}
	// OutlineTextRefAtom identifies authored text storage, but it is also used by
	// ordinary one-axis grouped diagrams. It becomes table ownership evidence only
	// when the cells establish both row and column axes. This preserves sparse/merged
	// tables without allowing a row of labelled boxes to become a semantic table.
	if legacyPptCellsCarryTableSemantics(cells, len(gridLines)) && legacyPptOutlineRefsSupportTableOwnership(cells) {
		return true
	}
	// Geometry alone is not table semantics. Dense arrangements of ordinary
	// rectangles occur in diagrams, legends and master artwork (42485.ppt).
	// Without tableProperties or OutlineTextRef identity, require authored grid
	// lines that actually support the cell lattice.
	return legacyPptGridLinesSupportCellLattice(cells, gridLines)
}

func legacyPptGroupPathEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func legacyPptGroupPathKey(path []int) string {
	if len(path) == 0 {
		return ""
	}
	var b strings.Builder
	for _, v := range path {
		fmt.Fprintf(&b, "/%d", v)
	}
	return b.String()
}

func legacyPptGroupPathOwned(path []int, owners [][]int) bool {
	for _, owner := range owners {
		if len(owner) > len(path) {
			continue
		}
		match := true
		for i := range owner {
			if owner[i] != path[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func legacyPptOfficeArtContainsExternalObject(data []byte) bool {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			return false
		}
		version := verInst & 0x000F
		body := data[off+8 : off+8+recLen]

		if recType == legacyOfficeArtSpContainer {
			node, err := officeartescher.ParseShapeContainerPayload(body)
			if err == nil && node != nil && len(node.ClientData) > 0 {
				if _, ok := parseLegacyPptExObjRef(node.ClientData); ok {
					return true
				}
			}
		}
		if version == 0x000F && legacyPptOfficeArtContainsExternalObject(body) {
			return true
		}
		off += 8 + recLen
	}
	return false
}

func recoverLegacyPptTableShapeFromGroupContainer(data []byte, parent legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, ordinal int) (*presentationir.Shape, bool) {
	// An embedded/linked OLE object may contain a surprisingly rich OfficeArt
	// subtree (chart labels, grid lines, drawing primitives, and text boxes).
	// Those children describe the object's preview; they are not a PowerPoint
	// table and must never be promoted into slide-level table geometry. The OLE
	// shape parser owns the outer Escher anchor and materializes the preview
	// strictly inside that authored rectangle.
	if legacyPptOfficeArtContainsExternalObject(data) {
		return nil, false
	}

	tableFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(data, parent)
	if !ok {
		return nil, false
	}
	groupShapeID, _ := legacyPptGroupRootShapeID(data)
	var cells []legacyPptTableCellShape
	var gridLines []presentationir.Rect
	recoverLegacyPptTableCellsFromGroup(data, tableFrame, fontTable, colorScheme, exHyperlinks, &cells, &gridLines)
	if len(cells) == 0 {
		return nil, false
	}
	// Prefer the authored OfficeArt tableProperties marker.  Older producers
	// (and a few POI corpus files) omit that property but still encode table
	// cells as a group of text boxes carrying OutlineTextRefAtom identities plus
	// authored grid-line shapes.  That is semantic record evidence, not merely a
	// geometric lattice, and is safe as the compatibility fallback.
	explicitTable := legacyPptGroupContainerIsTable(data)
	// Some older producers omit groupshape.tableProperties. Preserve that legacy
	// compatibility when the group carries semantic cell identities. OutlineTextRefAtom
	// identities are authored PowerPoint table evidence and must not be subjected to
	// geometric density heuristics: valid merged/sparse legacy tables such as bug57820
	// can occupy only a small fraction of their inferred lattice. For producers that
	// lack those semantic identities, retain the geometric compatibility path only
	// behind the density gate so sparse labelled diagrams remain ordinary drawings.
	fallbackTable := (legacyPptCellsCarryTableSemantics(cells, len(gridLines)) && legacyPptOutlineRefsSupportTableOwnership(cells)) || legacyPptLooksLikeTableGroup(cells, gridLines)
	if !explicitTable && !fallbackTable {
		return nil, false
	}
	table, bounds, ok := buildLegacyPptTableFromCells(cells, tableFrame.target)
	if !ok || table == nil || len(table.Rows) == 0 {
		return nil, false
	}
	// Unmarked legacy tables are recognized from the authored cell lattice itself.
	// Do not compare transformed child bounds with the group frame here: several valid
	// legacy producers use child/group coordinate spaces that are not directly comparable.
	// The density gate above rejects sparse diagram compositions without depending on
	// those coordinate-space assumptions.
	return &presentationir.Shape{
		Kind:            presentationir.ShapeTable,
		Name:            fmt.Sprintf("Legacy Table %d", ordinal),
		OriginalShapeID: groupShapeID,
		Bounds:          bounds,
		Table:           table,
	}, true
}

func legacyPptCellsCarryTableSemantics(cells []legacyPptTableCellShape, lineCount int) bool {
	if len(cells) < 2 {
		return false
	}
	withOutlineRef := 0
	for _, cell := range cells {
		if cell.outlineRef >= 0 {
			withOutlineRef++
		}
	}
	// Require multiple independently referenced text cells.  This deliberately
	// avoids classifying ordinary grouped drawings from alignment alone.
	return withOutlineRef >= 2
}

func legacyPptOutlineRefsSupportTableOwnership(cells []legacyPptTableCellShape) bool {
	haveUsableBounds := false
	for _, cell := range cells {
		if cell.bounds.W > 0 && cell.bounds.H > 0 {
			haveUsableBounds = true
			break
		}
	}
	// Damaged compatibility records can preserve OutlineTextRef identities after
	// losing usable anchors. In that case the authored references are the only
	// remaining table evidence. Once geometry exists, require a genuine 2-D cell
	// arrangement so a one-row labelled diagram cannot steal table ownership.
	return !haveUsableBounds || legacyPptTableCellGridHasTwoAxesRelaxed(cells)
}

func legacyPptTableCellGridHasTwoAxesRelaxed(cells []legacyPptTableCellShape) bool {
	if len(cells) < 2 {
		return false
	}
	return len(legacyPptTableAxisPositions(cells, true)) >= 2 && len(legacyPptTableAxisPositions(cells, false)) >= 2
}

func legacyPptTableCellGridHasTwoAxes(cells []legacyPptTableCellShape) bool {
	if len(cells) < 4 {
		return false
	}
	cols := legacyPptTableAxisPositions(cells, true)
	rows := legacyPptTableAxisPositions(cells, false)
	return len(cols) >= 2 && len(rows) >= 2
}

func legacyPptTableCellGridIsDense(cells []legacyPptTableCellShape) bool {
	if len(cells) < 2 {
		return false
	}
	cols := legacyPptTableAxisPositions(cells, true)
	rows := legacyPptTableAxisPositions(cells, false)
	if len(cols) == 0 || len(rows) == 0 {
		return false
	}
	slots := len(cols) * len(rows)
	// Legacy merged cells can make a real table less than 100% occupied, but a table
	// still populates a substantial part of its inferred row/column lattice. Requiring
	// half the slots rejects sparse labelled diagrams such as the GPS composition while
	// preserving ordinary and merged legacy tables.
	return len(cells)*2 >= slots
}

func legacyPptRectContainedByFrame(rect, frame presentationir.Rect) bool {
	if rect.W <= 0 || rect.H <= 0 || frame.W <= 0 || frame.H <= 0 {
		return false
	}
	// Small rounding differences are normal when child anchors are transformed
	// through nested group coordinate systems.  Two percent of the frame is
	// enough tolerance for those differences without accepting escaped geometry.
	tolX := frame.W / 50
	tolY := frame.H / 50
	return rect.X >= frame.X-tolX && rect.Y >= frame.Y-tolY &&
		rect.X+rect.W <= frame.X+frame.W+tolX &&
		rect.Y+rect.H <= frame.Y+frame.H+tolY
}

func recoverLegacyPptGroupIsTableLike(data []byte, parent legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) bool {
	// Text-family routing must use the same ownership gate as the semantic scene.
	// OutlineTextRefAtom is common in ordinary grouped diagrams and is not, by
	// itself, table evidence. A weaker compatibility-table guess here used to
	// suppress every ClientTextbox in such groups before semantic reconciliation.
	if legacyPptOfficeArtContainsExternalObject(data) {
		return false
	}
	tableFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(data, parent)
	if !ok {
		return false
	}
	var cells []legacyPptTableCellShape
	var gridLines []presentationir.Rect
	recoverLegacyPptTableCellsFromGroup(data, tableFrame, fontTable, colorScheme, exHyperlinks, &cells, &gridLines)
	return legacyPptSceneGroupOwnsTable(legacyPptGroupContainerIsTable(data), cells, gridLines)
}

func legacyPptGroupContainerIsTable(data []byte) bool {
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		// Only the group's own root SpContainer can author tableProperties for
		// this group. Nested SpgrContainers own their markers independently.
		// Recursing here promotes an ordinary composition group when one of its
		// descendant groups happens to be a table.
		if recType == legacyOfficeArtSpContainer && legacyPptSpContainerHasGroupRect(body) && legacyPptSpContainerIsTable(body) {
			return true
		}
		off += 8 + recLen
	}
	return false
}

func legacyPptSpContainerIsTable(data []byte) bool {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		switch recType {
		case officeartescher.RecordFOPT, officeartescher.RecordSecondaryFOPT, officeartescher.RecordTertiaryFOPT:
			// groupshape.tableProperties is itself the semantic marker.  Escher
			// property ordering is not constrained to follow the Spgr atom, so do
			// not make recognition depend on having already visited that sibling.
			if legacyPptFOPTMarksTable(body, int(verInst>>4)) {
				return true
			}
		}
		off += 8 + recLen
	}
	return false
}

func legacyPptFOPTMarksTable(body []byte, propCount int) bool {
	for index := 0; index < propCount; index++ {
		offset := index * 6
		if offset+6 > len(body) {
			break
		}
		opid := binary.LittleEndian.Uint16(body[offset : offset+2])
		if opid&0x3FFF != legacyOfficeArtPropTableProperties {
			continue
		}
		// POI/HSLF treats the presence of groupshape.tableProperties as the
		// semantic table marker.  The property value is a flag bundle and bit 0
		// is not an "is table" discriminator.
		return true
	}
	return false
}

func recoverLegacyPptTableCellsFromGroup(data []byte, frame legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, cells *[]legacyPptTableCellShape, gridLines *[]presentationir.Rect) {
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		switch recType {
		case legacyOfficeArtSpgrContainer:
			nextFrame := frame
			if groupFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(body, frame); ok {
				nextFrame = groupFrame
			}
			recoverLegacyPptTableCellsFromGroup(body, nextFrame, fontTable, colorScheme, exHyperlinks, cells, gridLines)
		case legacyOfficeArtSpContainer:
			if legacyPptSpContainerHasGroupRect(body) {
				off += 8 + recLen
				continue
			}
			if shapeType, ok := legacyPptSpContainerShapeType(body); ok && shapeType == 20 {
				// Candidate discovery must remain permissive for old PPT producers: the
				// mere presence of a line shape was historically part of compatibility
				// table inference. Preserve that fact even when the connector parser cannot
				// recover a usable rectangle. Ownership is decided later by
				// recoverLegacyPptGroupIsTableLike/provenance, so a diagram is free to produce a
				// candidate without surrendering its group projection.
				if line, ok := parseLegacyPptConnectorShapeContainer(body, frame, colorScheme); ok {
					*gridLines = append(*gridLines, line.bounds)
				} else {
					*gridLines = append(*gridLines, presentationir.Rect{})
				}
				off += 8 + recLen
				continue
			}
			shape, ok := parseLegacyPptTextShapeContainer(body, frame, fontTable, colorScheme, exHyperlinks)
			if !ok {
				off += 8 + recLen
				continue
			}
			paragraphs := shape.paragraphs
			if len(paragraphs) == 0 {
				paragraphs = shape.inlineFallback
			}
			*cells = append(*cells, legacyPptTableCellShape{
				bounds:     shape.bounds,
				paragraphs: cloneLegacyParagraphs(paragraphs),
				fillColor:  shape.fillColor,
				outlineRef: shape.outlineRef,
			})
		}
		off += 8 + recLen
	}
}

func legacyPptSpContainerShapeType(data []byte) (uint16, bool) {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		if recType == legacyOfficeArtSp {
			return verInst >> 4, true
		}
		off += 8 + recLen
	}
	return 0, false
}

func legacyPptSpContainerShapeID(data []byte) (uint32, bool) {
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		if recType == legacyOfficeArtSp {
			body := data[off+8 : off+8+recLen]
			if len(body) >= 4 {
				return binary.LittleEndian.Uint32(body[:4]), true
			}
			break
		}
		off += 8 + recLen
	}
	return 0, false
}

func legacyPptGroupRootShapeID(data []byte) (uint32, bool) {
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		if recType == legacyOfficeArtSpContainer && legacyPptSpContainerHasGroupRect(body) {
			return legacyPptSpContainerShapeID(body)
		}
		off += 8 + recLen
	}
	return 0, false
}

func legacyPptLooksLikeTableGroup(cells []legacyPptTableCellShape, gridLines []presentationir.Rect) bool {
	if len(cells) < 2 {
		return false
	}
	colPos := legacyPptTableAxisPositions(cells, true)
	rowPos := legacyPptTableAxisPositions(cells, false)
	if len(colPos) < 2 && len(rowPos) < 2 {
		return false
	}

	// A sufficiently populated cell lattice is useful evidence even when an old
	// producer omitted tableProperties and did not emit explicit grid lines. Two
	// arbitrary text boxes can trivially form a 2x2 half-filled lattice, so density
	// only becomes table evidence once there are enough independently anchored cells.
	if len(cells) >= 4 && legacyPptTableCellGridIsDense(cells) {
		return true
	}

	// Sparse/merged legacy tables exist (bug57820 is the compatibility fixture).
	// Candidate discovery intentionally retains the historical line-presence fallback:
	// some producers do not give us enough geometry to prove a coherent lattice. A
	// candidate is not ownership. recoverLegacyPptGroupIsTableLike deliberately does *not*
	// call this helper, so weak candidates keep their ordinary group projection and
	// can be rejected by the later sibling-provenance pass.
	if legacyPptGridLinesSupportCellLattice(cells, gridLines) {
		return true
	}
	return len(gridLines) > 0
}

func legacyPptGridLinesSupportCellLattice(cells []legacyPptTableCellShape, lines []presentationir.Rect) bool {
	// A one-cell table is legal and occurs in damaged legacy files where the table
	// flag has been lost but the enclosing border/grid survives.  Requiring two
	// text cells made those tables impossible to recover.  The two-axis line test
	// below is the ownership proof and still rejects ordinary one-row diagrams.
	if len(cells) < 1 || len(lines) < 2 {
		return false
	}
	bounds := legacyPptTableBounds(cells)
	if bounds.W <= 0 || bounds.H <= 0 {
		return false
	}
	// Child anchors are integer OfficeArt coordinates and may differ slightly after
	// nested-group projection. Use a small scale-relative tolerance, never a fixed
	// fixture-specific distance.
	tolX := bounds.W / 100
	tolY := bounds.H / 100
	if tolX < 1 {
		tolX = 1
	}
	if tolY < 1 {
		tolY = 1
	}

	xEdges := make([]int64, 0, len(cells)*2)
	yEdges := make([]int64, 0, len(cells)*2)
	for _, cell := range cells {
		xEdges = append(xEdges, cell.bounds.X, cell.bounds.X+cell.bounds.W)
		yEdges = append(yEdges, cell.bounds.Y, cell.bounds.Y+cell.bounds.H)
	}
	near := func(value int64, edges []int64, tol int64) bool {
		for _, edge := range edges {
			d := value - edge
			if d < 0 {
				d = -d
			}
			if d <= tol {
				return true
			}
		}
		return false
	}

	vertical, horizontal := 0, 0
	for _, line := range lines {
		// Zero-width/height anchors are common for OfficeArt lines. A slightly
		// non-zero minor axis is also accepted when it is tiny relative to the span.
		if line.H > 0 && (line.W == 0 || line.W*20 <= line.H) && near(line.X, xEdges, tolX) {
			vertical++
		}
		if line.W > 0 && (line.H == 0 || line.H*20 <= line.W) && near(line.Y, yEdges, tolY) {
			horizontal++
		}
	}

	// A real grid must constrain both axes. Requiring two aligned rules overall
	// still permits sparse/merged tables while rejecting directional diagram arrows.
	return vertical > 0 && horizontal > 0 && vertical+horizontal >= 2
}

func legacyPptSpContainerHasGroupRect(data []byte) bool {
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		if recType == legacyOfficeArtSpgr {
			return true
		}
		off += 8 + recLen
	}
	return false
}

func buildLegacyPptTableFromCells(cells []legacyPptTableCellShape, fallbackBounds presentationir.Rect) (*presentationir.Table, presentationir.Rect, bool) {
	if len(cells) == 0 {
		return nil, presentationir.Rect{}, false
	}
	colPos := legacyPptTableAxisPositions(cells, true)
	rowPos := legacyPptTableAxisPositions(cells, false)
	if len(colPos) == 0 || len(rowPos) == 0 {
		return nil, presentationir.Rect{}, false
	}
	grid := make([][]*presentationir.TableCell, len(rowPos))
	for r := range grid {
		grid[r] = make([]*presentationir.TableCell, len(colPos))
	}
	rowHeights := make([]int64, len(rowPos))
	colWidths := make([]int64, len(colPos))
	for _, cell := range cells {
		r, okRow := legacyPptNearestTablePositionIndex(rowPos, cell.bounds.Y, legacyPptTableAxisTolerance(cells, false))
		c, okCol := legacyPptNearestTablePositionIndex(colPos, cell.bounds.X, legacyPptTableAxisTolerance(cells, true))
		if !okRow || !okCol {
			continue
		}
		gridSpan := legacyPptTableSpan(colPos, c, cell.bounds.X+cell.bounds.W)
		rowSpan := legacyPptTableSpan(rowPos, r, cell.bounds.Y+cell.bounds.H)
		if gridSpan <= 0 {
			gridSpan = 1
		}
		if rowSpan <= 0 {
			rowSpan = 1
		}
		if rowHeights[r] < cell.bounds.H/int64(rowSpan) {
			rowHeights[r] = cell.bounds.H / int64(rowSpan)
		}
		if colWidths[c] < cell.bounds.W/int64(gridSpan) {
			colWidths[c] = cell.bounds.W / int64(gridSpan)
		}
		tableCell := &presentationir.TableCell{
			GridSpan:   gridSpan,
			RowSpan:    rowSpan,
			Paragraphs: cloneLegacyParagraphs(cell.paragraphs),
			FillColor:  cell.fillColor,
		}
		if cell.fillColor != "" {
			tableCell.Fill = &drawing.SolidFill{Color: cell.fillColor, Alpha: 1.0}
		}
		grid[r][c] = tableCell
		for rr := r; rr < r+rowSpan && rr < len(grid); rr++ {
			for cc := c; cc < c+gridSpan && cc < len(grid[rr]); cc++ {
				if rr == r && cc == c {
					continue
				}
				mergeCell := &presentationir.TableCell{}
				if rr > r {
					mergeCell.VMerge = true
				}
				if cc > c {
					mergeCell.HMerge = true
				}
				grid[rr][cc] = mergeCell
			}
		}
	}
	table := &presentationir.Table{ColumnWidthsEMU: make([]int64, len(colPos))}
	for c := range colWidths {
		if colWidths[c] <= 0 {
			colWidths[c] = legacyPptTableFallbackColumnWidth(colPos, c)
		}
		table.ColumnWidthsEMU[c] = colWidths[c]
	}
	for r := range grid {
		if rowHeights[r] <= 0 {
			rowHeights[r] = legacyPptTableFallbackRowHeight(rowPos, r)
		}
		row := &presentationir.TableRow{HeightEMU: rowHeights[r], Cells: make([]*presentationir.TableCell, len(grid[r]))}
		for c := range grid[r] {
			if grid[r][c] == nil {
				grid[r][c] = &presentationir.TableCell{}
			}
			row.Cells[c] = grid[r][c]
		}
		table.Rows = append(table.Rows, row)
	}
	bounds := fallbackBounds
	if bounds.W == 0 || bounds.H == 0 {
		bounds = legacyPptTableBounds(cells)
	}
	if bounds.W == 0 || bounds.H == 0 {
		return nil, presentationir.Rect{}, false
	}
	return table, bounds, true
}

func legacyPptTableAxisTolerance(cells []legacyPptTableCellShape, horizontal bool) int64 {
	sizes := make([]int64, 0, len(cells))
	for _, cell := range cells {
		size := cell.bounds.H
		if horizontal {
			size = cell.bounds.W
		}
		if size > 0 {
			sizes = append(sizes, size)
		}
	}
	if len(sizes) == 0 {
		return 1
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i] < sizes[j] })
	tol := sizes[len(sizes)/2] / 50
	if tol < 1 {
		tol = 1
	}
	return tol
}

func legacyPptTableAxisPositions(cells []legacyPptTableCellShape, horizontal bool) []int64 {
	values := make([]int64, 0, len(cells))
	for _, cell := range cells {
		if horizontal {
			values = append(values, cell.bounds.X)
		} else {
			values = append(values, cell.bounds.Y)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	tol := legacyPptTableAxisTolerance(cells, horizontal)
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if len(out) == 0 || value-out[len(out)-1] > tol {
			out = append(out, value)
		}
	}
	return out
}

func legacyPptNearestTablePositionIndex(positions []int64, value, tolerance int64) (int, bool) {
	best, bestDistance := -1, int64(0)
	for i, position := range positions {
		d := value - position
		if d < 0 {
			d = -d
		}
		if best < 0 || d < bestDistance {
			best, bestDistance = i, d
		}
	}
	return best, best >= 0 && bestDistance <= tolerance
}

func legacyPptTableSpan(positions []int64, start int, end int64) int {
	if start < 0 || start >= len(positions) {
		return 1
	}
	span := 1
	base := positions[start]
	for i := start + 1; i < len(positions) && positions[i] < end; i++ {
		if positions[i] > base {
			span++
		}
	}
	return span
}

func legacyPptTableFallbackColumnWidth(positions []int64, index int) int64 {
	if index >= 0 && index+1 < len(positions) {
		if width := positions[index+1] - positions[index]; width > 0 {
			return width
		}
	}
	return 914400
}

func legacyPptTableFallbackRowHeight(positions []int64, index int) int64 {
	if index >= 0 && index+1 < len(positions) {
		if height := positions[index+1] - positions[index]; height > 0 {
			return height
		}
	}
	return 457200
}

func legacyPptTableBounds(cells []legacyPptTableCellShape) presentationir.Rect {
	if len(cells) == 0 {
		return presentationir.Rect{}
	}
	left := cells[0].bounds.X
	top := cells[0].bounds.Y
	right := cells[0].bounds.X + cells[0].bounds.W
	bottom := cells[0].bounds.Y + cells[0].bounds.H
	for _, cell := range cells[1:] {
		if cell.bounds.X < left {
			left = cell.bounds.X
		}
		if cell.bounds.Y < top {
			top = cell.bounds.Y
		}
		if x2 := cell.bounds.X + cell.bounds.W; x2 > right {
			right = x2
		}
		if y2 := cell.bounds.Y + cell.bounds.H; y2 > bottom {
			bottom = y2
		}
	}
	if right <= left || bottom <= top {
		return presentationir.Rect{}
	}
	return presentationir.Rect{X: left, Y: top, W: right - left, H: bottom - top}
}

func collectLegacyPptConnectorShapesFromEvents(events []OfficeArtEvent, frame legacyPptGroupFrame, colorScheme map[uint8]string, rules map[uint32]legacyPptConnectorRule, shapeIndex map[uint32]*presentationir.Shape, shapes *[]*presentationir.Shape) {
	frames := []legacyPptGroupFrame{frame}
	skipTableGroups := 0
	_ = walkCollectedOfficeArtEvents(events, OfficeArtConsumerFunc(func(event OfficeArtEvent) error {
		switch {
		case event.GroupStart:
			parent := frames[len(frames)-1]
			next := parent
			if groupFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(event.Payload, parent); ok {
				next = groupFrame
			}
			frames = append(frames, next)
			if skipTableGroups > 0 || legacyPptGroupContainerIsTable(event.Payload) {
				skipTableGroups++
			}
		case event.GroupEnd:
			if skipTableGroups > 0 {
				skipTableGroups--
			}
			if len(frames) > 1 {
				frames = frames[:len(frames)-1]
			}
		case event.ShapeStart:
			if skipTableGroups > 0 {
				return nil
			}
			connector, ok := parseLegacyPptConnectorShapeNode(event.ShapeNode, frames[len(frames)-1], colorScheme)
			if !ok {
				return nil
			}
			start, end := legacyPptConnectorEndpoints(connector.bounds, connector.transform.FlipH, connector.transform.FlipV)
			if rule, ok := rules[connector.shapeID]; ok {
				if target := shapeIndex[rule.startShapeID]; target != nil {
					start = legacyPptConnectorRuleSitePointForShape(target, rule.startSite)
				}
				if target := shapeIndex[rule.endShapeID]; target != nil {
					end = legacyPptConnectorRuleSitePointForShape(target, rule.endSite)
				}
			}
			connectorIR := &presentationir.Shape{Kind: presentationir.ShapeConnector, Name: fmt.Sprintf("Legacy Connector %d", len(*shapes)+1), OriginalShapeID: connector.shapeID, Bounds: connector.bounds, ConnectorType: legacyPptConnectorType(connector.shapeType), ConnectorFlipH: connector.transform.FlipH, ConnectorFlipV: connector.transform.FlipV, ConnectorAdjustments: append([]int(nil), connector.adjustments...), ConnectorStart: start, ConnectorEnd: end, OfficeArt: connector.officeArt}
			// One surface bridge owns geometry, paint, effects and PPT-specific
			// compatibility. Do not stage stroke/arrow state twice around the shared
			// OfficeArt visual bridge.
			applyLegacyPptOfficeArtSurface(connectorIR, connector.officeArt, connector.transform)
			// Generic OfficeArt geometry adjustments contain connector solver/cache
			// state as well as authored preset guides. The canonical connector path
			// above has already reduced that source syntax to the semantic routing
			// slots owned by ConnectorAdjustments.
			connectorIR.GeometryAdjustments = nil
			applyLegacyPptAnimationMetadata(connectorIR, connector.animation)
			*shapes = append(*shapes, connectorIR)
		}
		return nil
	}))
}
func parseLegacyPptConnectorShapeContainer(data []byte, frame legacyPptGroupFrame, colorScheme map[uint8]string) (legacyPptConnectorShape, bool) {
	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil {
		return legacyPptConnectorShape{}, false
	}
	return parseLegacyPptConnectorShapeNode(node, frame, colorScheme)
}

func parseLegacyPptConnectorShapeNode(node *officeartescher.ShapeNode, frame legacyPptGroupFrame, colorScheme map[uint8]string) (legacyPptConnectorShape, bool) {
	var shape legacyPptConnectorShape
	if node == nil {
		return legacyPptConnectorShape{}, false
	}
	shape.shapeID = node.ShapeID
	shape.officeArt = officeartlegacy.ShapeFromEscherWithColorResolver(node, func(raw uint32) string {
		return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
	})
	shape.shapeType = node.ShapeType
	shape.transform.FlipH = node.Flags&legacyOfficeArtSpFlagFlipH != 0
	shape.transform.FlipV = node.Flags&legacyOfficeArtSpFlagFlipV != 0

	if len(node.Properties) > 0 {
		parseLegacyPptFOPTPropertiesTransform(node.Properties, colorScheme, &shape.transform)
	}
	shape.adjustments = canonicalLegacyPptConnectorAdjustments(shape.shapeType, shape.officeArt)
	if len(node.ClientData) > 0 {
		if anim, ok := parseLegacyPptShapeAnimationInfo(node.ClientData); ok {
			shape.animation = anim
		}
	}

	placement := resolveLegacyPptShapePlacement(node, frame)

	haveAnchor := placement.valid()
	if haveAnchor {
		shape.bounds = placement.Bounds
	}
	if !haveAnchor || legacyPptConnectorType(shape.shapeType) == "" {
		return legacyPptConnectorShape{}, false
	}
	return shape, true
}

func legacyPptConnectorType(shapeType uint16) string {
	switch shapeType {
	case 20, 0x20:
		return "straightConnector1"
	case 0x21:
		return "bentConnector2"
	case 0x22:
		return "bentConnector3"
	case 0x23:
		return "bentConnector4"
	case 0x24:
		return "bentConnector5"
	case 0x25:
		return "curvedConnector2"
	case 0x26:
		return "curvedConnector3"
	case 0x27:
		return "curvedConnector4"
	case 0x28:
		return "curvedConnector5"
	default:
		return ""
	}
}

// canonicalLegacyPptConnectorAdjustments projects only the adjustment slots
// that belong to the connector preset itself. Legacy OfficeArt frequently
// carries additional adjustValue properties on connectors as solver/cache
// state; treating all of those as routing guides corrupts canonical IR. The
// actual bent/curved connector guides use the 21600 OfficeArt geometry space
// and default to the DrawingML-equivalent midpoint when omitted.
func canonicalLegacyPptConnectorAdjustments(shapeType uint16, shared *officeartir.Shape) []int {
	count := 0
	switch legacyPptConnectorType(shapeType) {
	case "bentConnector3", "curvedConnector3":
		count = 1
	case "bentConnector4", "curvedConnector4":
		count = 2
	case "bentConnector5", "curvedConnector5":
		count = 3
	default:
		return nil
	}
	out := make([]int, count)
	for i := range out {
		out[i] = 50000
	}
	if shared == nil {
		return out
	}
	values := shared.Geometry.Adjustments
	present := shared.Geometry.AdjustmentPresent
	for i := 0; i < count && i < len(values) && i < len(present); i++ {
		if !present[i] {
			continue
		}
		// Connector adjustment handles are ordinary coordinates in the
		// classic 21600-unit OfficeArt geometry space. They are allowed to
		// extend beyond the nominal shape box, so do not clamp here.
		out[i] = int(math.Round(values[i] * 100000.0 / 21600.0))
	}
	return out
}

func parseLegacyPptConnectorAdjustmentsProperties(props []officeartescher.Property) []int {
	values := make(map[int]int)
	maxIndex := 0
	for _, prop := range props {
		if prop.IsBlip || prop.IsComplex {
			continue
		}
		if prop.ID < legacyOfficeArtPropAdjustValue || prop.ID > legacyOfficeArtPropAdjustValue+9 {
			continue
		}
		index := int(prop.ID-legacyOfficeArtPropAdjustValue) + 1
		values[index] = normalizeLegacyPptConnectorAdjustment(int64(int32(prop.Value)))
		if index > maxIndex {
			maxIndex = index
		}
	}
	if maxIndex == 0 {
		return nil
	}
	out := make([]int, maxIndex)
	for index, value := range values {
		out[index-1] = value
	}
	return out
}

func normalizeLegacyPptConnectorAdjustment(raw int64) int {
	// OfficeArt preset geometry uses the classic 21600-unit shape coordinate
	// space. DrawingML connector helpers use 0..100000 adjustment values.
	if raw >= -21600 && raw <= 21600 {
		return int(math.Round(float64(raw) * 100000.0 / 21600.0))
	}
	if raw > 100000 {
		return 100000
	}
	if raw < -100000 {
		return -100000
	}
	return int(raw)
}

func parseLegacyPptConnectorRules(data []byte) map[uint32]legacyPptConnectorRule {
	rules := map[uint32]legacyPptConnectorRule{}
	walkLegacyPptOfficeArtRecords(data, func(recType, _, _ uint16, body []byte) {
		if recType != legacyOfficeArtFConnectorRule || len(body) < 24 {
			return
		}
		rule := legacyPptConnectorRule{
			startShapeID: binary.LittleEndian.Uint32(body[4:8]),
			endShapeID:   binary.LittleEndian.Uint32(body[8:12]),
			connectorID:  binary.LittleEndian.Uint32(body[12:16]),
			startSite:    binary.LittleEndian.Uint32(body[16:20]),
			endSite:      binary.LittleEndian.Uint32(body[20:24]),
		}
		if rule.connectorID != 0 {
			rules[rule.connectorID] = rule
		}
	})
	if len(rules) == 0 {
		return nil
	}
	return rules
}

// legacyPptConnectionSitePointForShape resolves FConnectorRule site indices against
// the target AutoShape contract. Site counts are shape-specific: treating cpti as
// `site % 4` is only valid for four-site shapes and silently attaches triangle
// connectors to the wrong edge. Keep the rectangle fallback for unknown presets,
// but own the non-four-site contracts here at the legacy OfficeArt boundary.
// legacyPptConnectorRuleSitePointForShape converts the cptiA/cptiB index stored
// by OfficeArt FConnectorRule into the canonical connection-site ordering used by
// the presentation IR. FConnectorRule numbers sites clockwise from the right-hand
// side for the legacy presets, while the canonical helper below numbers them from
// the top counterclockwise. Keeping this conversion at the rule boundary avoids
// corrupting geometry/site contracts used by OOXML and layout.
func legacyPptConnectorRuleSitePointForShape(shape *presentationir.Shape, site uint32) *presentationir.Point {
	if shape == nil {
		return nil
	}
	count := legacyPptConnectionSiteCountForShape(shape)
	if count == 0 {
		count = 4
	}
	canonicalSite := count - 1 - (site % count)
	return legacyPptConnectionSitePointForShape(shape, canonicalSite)
}

func legacyPptConnectionSiteCountForShape(shape *presentationir.Shape) uint32 {
	if shape == nil {
		return 0
	}
	preset := shape.Geometry
	if preset == "" && shape.OfficeArt != nil {
		preset = shape.OfficeArt.Geometry.Preset
	}
	switch preset {
	case "triangle", "rtTriangle":
		return 3
	case "pentagon":
		return 5
	case "hexagon":
		return 6
	case "octagon":
		return 8
	default:
		return 4
	}
}

func legacyPptConnectionSitePointForShape(shape *presentationir.Shape, site uint32) *presentationir.Point {
	if shape == nil {
		return nil
	}
	b := shape.Bounds
	preset := shape.Geometry
	if preset == "" && shape.OfficeArt != nil {
		preset = shape.OfficeArt.Geometry.Preset
	}

	// Prefer a preset-specific site contract. These points are expressed in the
	// shape's local bounding box and transformed below, so solver attachment
	// follows authored flip/rotation instead of attaching to the untransformed box.
	var point presentationir.Point
	var ok bool
	switch preset {
	case "triangle":
		points := []presentationir.Point{
			{X: b.X + b.W/2, Y: b.Y},
			{X: b.X, Y: b.Y + b.H},
			{X: b.X + b.W, Y: b.Y + b.H},
		}
		point, ok = legacyPptIndexedConnectionSite(points, site)
	case "rtTriangle":
		points := []presentationir.Point{
			{X: b.X, Y: b.Y},
			{X: b.X, Y: b.Y + b.H},
			{X: b.X + b.W, Y: b.Y + b.H},
		}
		point, ok = legacyPptIndexedConnectionSite(points, site)
	case "diamond":
		point, ok = legacyPptIndexedConnectionSite(legacyPptRegularPolygonConnectionSites(b, 4), site)
	case "pentagon":
		point, ok = legacyPptIndexedConnectionSite(legacyPptRegularPolygonConnectionSites(b, 5), site)
	case "hexagon":
		point, ok = legacyPptIndexedConnectionSite(legacyPptRegularPolygonConnectionSites(b, 6), site)
	case "octagon":
		point, ok = legacyPptIndexedConnectionSite(legacyPptRegularPolygonConnectionSites(b, 8), site)
	}
	if !ok {
		fallback := legacyPptConnectionSitePoint(b, site)
		if fallback == nil {
			return nil
		}
		point = *fallback
	}
	return legacyPptTransformConnectionSite(shape, point)
}

func legacyPptIndexedConnectionSite(points []presentationir.Point, site uint32) (presentationir.Point, bool) {
	if len(points) == 0 {
		return presentationir.Point{}, false
	}
	return points[int(site)%len(points)], true
}

func legacyPptRegularPolygonConnectionSites(bounds presentationir.Rect, sides int) []presentationir.Point {
	if sides < 3 {
		return nil
	}
	cx := float64(bounds.X) + float64(bounds.W)/2
	cy := float64(bounds.Y) + float64(bounds.H)/2
	rx := float64(bounds.W) / 2
	ry := float64(bounds.H) / 2
	points := make([]presentationir.Point, sides)
	for i := range points {
		// Office site ordering starts at the top and proceeds counterclockwise.
		a := -math.Pi/2 - 2*math.Pi*float64(i)/float64(sides)
		points[i] = presentationir.Point{
			X: int64(math.Round(cx + rx*math.Cos(a))),
			Y: int64(math.Round(cy + ry*math.Sin(a))),
		}
	}
	return points
}

func legacyPptTransformConnectionSite(shape *presentationir.Shape, point presentationir.Point) *presentationir.Point {
	if shape == nil || (!shape.FlipH && !shape.FlipV && shape.RotationDeg == 0) {
		return &point
	}
	b := shape.Bounds
	cx := float64(b.X) + float64(b.W)/2
	cy := float64(b.Y) + float64(b.H)/2
	x := float64(point.X) - cx
	y := float64(point.Y) - cy
	if shape.FlipH {
		x = -x
	}
	if shape.FlipV {
		y = -y
	}
	if shape.RotationDeg != 0 {
		a := shape.RotationDeg * math.Pi / 180
		cosA, sinA := math.Cos(a), math.Sin(a)
		x, y = x*cosA-y*sinA, x*sinA+y*cosA
	}
	out := presentationir.Point{X: int64(math.Round(cx + x)), Y: int64(math.Round(cy + y))}
	return &out
}

func legacyPptConnectionSitePoint(bounds presentationir.Rect, site uint32) *presentationir.Point {
	// Office connection sites are numbered from the top and proceed
	// counterclockwise. OfficeArt connector rules store the same zero-based
	// site index (cptiA/cptiB), so the cardinal fallback is top, left, bottom,
	// right. The previous right/bottom/left/top mapping rotated every attached
	// legacy connector by one side.
	cx := bounds.X + bounds.W/2
	cy := bounds.Y + bounds.H/2
	var point presentationir.Point
	switch site % 4 {
	case 0:
		point = presentationir.Point{X: cx, Y: bounds.Y}
	case 1:
		point = presentationir.Point{X: bounds.X, Y: cy}
	case 2:
		point = presentationir.Point{X: cx, Y: bounds.Y + bounds.H}
	default:
		point = presentationir.Point{X: bounds.X + bounds.W, Y: cy}
	}
	return &point
}

// normalizeLegacyPptConnectorEndpointOwnership closes connector representation
// only after groups/master composition have been materialized into final slide
// coordinates. Before this point a child connector can still be expressed in a
// group-local frame, so comparing its cached solver endpoints to Bounds would be
// premature.
func normalizeLegacyPptConnectorEndpointOwnership(pres *presentationir.Presentation) {
	if pres == nil {
		return
	}
	var visit func([]*presentationir.Shape)
	visit = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			if shape.Kind == presentationir.ShapeConnector {
				canonicalizeLegacyPptConnectorEndpointOwnership(shape)
			}
			visit(shape.Children)
		}
	}
	for _, slide := range pres.Slides {
		if slide != nil {
			visit(slide.Shapes)
		}
	}
}

// canonicalizeLegacyPptConnectorEndpointOwnership removes solver endpoints that
// merely restate the connector's bounded preset frame. PowerPoint binary files
// commonly cache both representations, while converted OOXML keeps only the
// bounded p:cxnSp geometry. Canonical IR should retain explicit endpoints only
// when they carry routing information that cannot be reconstructed from Bounds,
// ConnectorType, flips and ConnectorAdjustments.
func canonicalizeLegacyPptConnectorEndpointOwnership(shape *presentationir.Shape) {
	if shape == nil || shape.ConnectorStart == nil || shape.ConnectorEnd == nil || len(shape.ConnectorPath) > 0 {
		return
	}
	connectorType := shape.ConnectorType
	if connectorType == "" {
		connectorType = "straightConnector1"
	}
	points := drawing.ConnectorWaypoints(
		connectorType,
		drawing.EMUToPoints(shape.Bounds.X), drawing.EMUToPoints(shape.Bounds.Y),
		drawing.EMUToPoints(shape.Bounds.W), drawing.EMUToPoints(shape.Bounds.H),
		shape.ConnectorAdjustments, shape.ConnectorFlipH, shape.ConnectorFlipV,
	)
	if len(points) < 2 {
		return
	}
	const tolerancePt = 0.125 // comfortably above PPT master-unit / EMU roundoff
	start := drawing.Point{X: drawing.EMUToPoints(shape.ConnectorStart.X), Y: drawing.EMUToPoints(shape.ConnectorStart.Y)}
	end := drawing.Point{X: drawing.EMUToPoints(shape.ConnectorEnd.X), Y: drawing.EMUToPoints(shape.ConnectorEnd.Y)}
	first, last := points[0], points[len(points)-1]
	// DrawingML/OfficeArt rotation applies to the already-flipped preset geometry
	// around the connector frame center. Legacy solver records cache those final
	// slide-space endpoints, while converted OOXML commonly preserves only
	// Bounds + rotation. Compare the same effective representation before deciding
	// whether the cached endpoints carry unique routing information.
	if shape.RotationDeg != 0 {
		cx := drawing.EMUToPoints(shape.Bounds.X) + drawing.EMUToPoints(shape.Bounds.W)/2
		cy := drawing.EMUToPoints(shape.Bounds.Y) + drawing.EMUToPoints(shape.Bounds.H)/2
		first = rotateLegacyPptConnectorPoint(first, cx, cy, shape.RotationDeg)
		last = rotateLegacyPptConnectorPoint(last, cx, cy, shape.RotationDeg)
	}
	if math.Hypot(start.X-first.X, start.Y-first.Y) <= tolerancePt && math.Hypot(end.X-last.X, end.Y-last.Y) <= tolerancePt {
		shape.ConnectorStart = nil
		shape.ConnectorEnd = nil
		return
	}

	// Binary PowerPoint can cache solver endpoints for rotated bounded connectors
	// on the *other* diagonal of the connector frame. Those points are not an
	// authored free route: they contain no coordinate information beyond the
	// shape anchor itself. PowerPoint's PPT->PPTX conversion drops exactly this
	// cache and keeps xfrm + prstGeom, so canonical IR must do the same.
	//
	// Compare in the connector's unrotated frame. If the two cached endpoints
	// land on opposite frame corners (either diagonal, either order), the bounded
	// preset remains the source of truth. Endpoints that are inset, offset, or
	// otherwise carry real solver attachment coordinates are preserved.
	if shape.RotationDeg != 0 && legacyPptConnectorEndpointsAreFrameDiagonal(shape, start, end, tolerancePt) {
		shape.ConnectorStart = nil
		shape.ConnectorEnd = nil
	}
}

func legacyPptConnectorEndpointsAreFrameDiagonal(shape *presentationir.Shape, start, end drawing.Point, tolerancePt float64) bool {
	if shape == nil {
		return false
	}
	bx := drawing.EMUToPoints(shape.Bounds.X)
	by := drawing.EMUToPoints(shape.Bounds.Y)
	bw := drawing.EMUToPoints(shape.Bounds.W)
	bh := drawing.EMUToPoints(shape.Bounds.H)
	cx, cy := bx+bw/2, by+bh/2

	// Cached solver points are in final slide coordinates. Undo the authored
	// connector rotation so they can be compared to the bounded preset frame.
	start = rotateLegacyPptConnectorPoint(start, cx, cy, -shape.RotationDeg)
	end = rotateLegacyPptConnectorPoint(end, cx, cy, -shape.RotationDeg)

	tl := drawing.Point{X: bx, Y: by}
	tr := drawing.Point{X: bx + bw, Y: by}
	bl := drawing.Point{X: bx, Y: by + bh}
	br := drawing.Point{X: bx + bw, Y: by + bh}
	near := func(a, b drawing.Point) bool {
		return math.Hypot(a.X-b.X, a.Y-b.Y) <= tolerancePt
	}
	return (near(start, tl) && near(end, br)) ||
		(near(start, br) && near(end, tl)) ||
		(near(start, tr) && near(end, bl)) ||
		(near(start, bl) && near(end, tr))
}

func rotateLegacyPptConnectorPoint(point drawing.Point, cx, cy, rotationDeg float64) drawing.Point {
	if rotationDeg == 0 {
		return point
	}
	radians := rotationDeg * math.Pi / 180
	dx, dy := point.X-cx, point.Y-cy
	return drawing.Point{
		X: cx + dx*math.Cos(radians) - dy*math.Sin(radians),
		Y: cy + dx*math.Sin(radians) + dy*math.Cos(radians),
	}
}

func legacyPptConnectorEndpoints(bounds presentationir.Rect, flipH, flipV bool) (*presentationir.Point, *presentationir.Point) {
	start := presentationir.Point{X: bounds.X, Y: bounds.Y}
	end := presentationir.Point{X: bounds.X + bounds.W, Y: bounds.Y + bounds.H}
	if flipH {
		start.X, end.X = end.X, start.X
	}
	if flipV {
		start.Y, end.Y = end.Y, start.Y
	}
	return &start, &end
}

func collectLegacyPptImageShapesFromEvents(events []OfficeArtEvent, frame legacyPptGroupFrame, colorScheme map[uint8]string, pictureStore map[uint32]*legacyPptPicture, externalObjects map[uint32]legacyPptExternalObject, shapes *[]*presentationir.Shape) {
	frames := []legacyPptGroupFrame{frame}
	_ = walkCollectedOfficeArtEvents(events, OfficeArtConsumerFunc(func(event OfficeArtEvent) error {
		switch {
		case event.GroupStart:
			parent := frames[len(frames)-1]
			next := parent
			if groupFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(event.Payload, parent); ok {
				next = groupFrame
			}
			frames = append(frames, next)
		case event.GroupEnd:
			if len(frames) > 1 {
				frames = frames[:len(frames)-1]
			}
		case event.ShapeStart:
			pictureShape, ok := parseLegacyPptPictureShapeNode(event.ShapeNode, frames[len(frames)-1], colorScheme)
			if !ok {
				return nil
			}
			picture := pictureStore[pictureShape.blipRef]
			// Preserve the semantic picture node even when a damaged or unusual PPT
			// cannot resolve its BLIP payload. Dropping the whole shape here makes a
			// media-resolution failure indistinguishable from absent artwork and prevents
			// later diagnostics/recovery from retaining authored geometry and z-order.
			pictureIR := buildLegacyPptPictureOrExternalObjectShape(len(*shapes)+1, pictureShape, picture, externalObjects[pictureShape.exObjID])
			pictureIR.OfficeArt = pictureShape.officeArt
			applyLegacyPptOfficeArtSurface(pictureIR, pictureShape.officeArt, pictureShape.transform)
			applyLegacyPptAnimationMetadata(pictureIR, pictureShape.animation)
			*shapes = append(*shapes, pictureIR)
		}
		return nil
	}))
}

// extractLegacyPptMSGraphChartData adapts the format-neutral MSGraph parser to
// the presentation IR chart-part contract. All BIFF/MSGraph interpretation
// lives in pkg/msgraph; the PPT parser only identifies and attaches the object.
func extractLegacyPptMSGraphChartData(ext legacyPptExternalObject) []byte {
	if !msgraph.IsProgID(ext.ProgID) || len(ext.Data) == 0 {
		return nil
	}
	data, err := msgraph.MarshalPrimaryChart(ext.Data)
	if err != nil {
		return nil
	}
	return data
}

func buildLegacyPptPictureOrExternalObjectShape(ordinal int, pictureShape legacyPptPictureShape, picture *legacyPptPicture, ext legacyPptExternalObject) *presentationir.Shape {
	if picture == nil {
		picture = &legacyPptPicture{}
	}
	if ext.Kind == presentationir.ShapeOleObject {
		shape := &presentationir.Shape{
			Kind:             presentationir.ShapeOleObject,
			OriginalShapeID:  pictureShape.shapeID,
			Name:             firstLegacyPptNonEmpty(ext.Name, fmt.Sprintf("Legacy OLE Object %d", ordinal)),
			Bounds:           pictureShape.bounds,
			OLEObjectData:    append([]byte(nil), ext.Data...),
			OLEObjectMIME:    ext.MIME,
			OLEObjectName:    ext.Name,
			OLEObjectProgID:  ext.ProgID,
			OLEObjectLinkURL: ext.LinkURL,
			OLEIconData:      append([]byte(nil), picture.Data...),
			OLEIconMIME:      picture.MIME,
		}
		if mathml, err := mathtype.MathMLFromOLE(shape.OLEObjectData); err == nil {
			shape.OLEMathML = mathml
			shape.OLEMathColor = pictureShape.mathColor
		}
		// MSGraph OLE objects carry an actual BIFF workbook/chart model in addition
		// to the cached presentation preview. Promote those objects to the canonical
		// chart kind as soon as the semantic chart can be recovered, while keeping
		// the authored preview as ImageData so the current renderer still has an
		// exact fallback until shared semantic chart rendering takes ownership.
		if chartData := extractLegacyPptMSGraphChartData(ext); len(chartData) > 0 {
			shape.Kind = presentationir.ShapeChart
			shape.ChartData = chartData
			// The preview is optional fallback metadata. Semantic MSGraph recovery must
			// not depend on the BLIP/preview resolving successfully; otherwise valid
			// charts regress back to generic OLE placeholders when a cached preview is
			// absent or damaged.
			if len(picture.Data) > 0 {
				shape.ImageData = append([]byte(nil), picture.Data...)
				shape.ImageMIME = picture.MIME
			}
		}
		return shape
	}
	if ext.Kind == presentationir.ShapeMedia {
		return &presentationir.Shape{
			Kind:            presentationir.ShapeMedia,
			OriginalShapeID: pictureShape.shapeID,
			Name:            firstLegacyPptNonEmpty(ext.Name, fmt.Sprintf("Legacy Media %d", ordinal)),
			Bounds:          pictureShape.bounds,
			MediaKind:       ext.MediaKind,
			MediaData:       append([]byte(nil), ext.Data...),
			MediaLinkURL:    ext.LinkURL,
			MediaMIME:       ext.MIME,
			MediaPosterData: append([]byte(nil), picture.Data...),
			MediaPosterMIME: picture.MIME,
		}
	}
	imageData := append([]byte(nil), picture.Data...)
	imageMIME := picture.MIME
	imageEffects := append([]drawing.Effect(nil), pictureShape.imageEffects...)
	// OfficeArt pictureTransparent is a raster color-key operation. Materialize
	// exact keyed pixels into alpha while the original raster is still available.
	// Leaving this as a CSS/SVG filter makes crop + transparency browser-dependent
	// (and historically left the white Earth canvas / red satellite canvas opaque).
	// Once materialized, the ordinary shared image renderer owns crop, transforms
	// and composition identically for PPT and PPTX.
	imageData, imageMIME, imageEffects = imageutil.MaterializeTransparentColorKeys(imageData, imageMIME, imageEffects)
	if svg, _, _, ok := metafile.ToSVGWithOptions(imageData, imageMIME, metafile.Options{Recolors: pictureShape.metafileRecolors}); ok {
		imageData = svg
		imageMIME = "image/svg+xml"
	}
	return &presentationir.Shape{
		Kind:            presentationir.ShapeImage,
		OriginalShapeID: pictureShape.shapeID,
		Name:            fmt.Sprintf("Legacy Picture %d", ordinal),
		Bounds:          pictureShape.bounds,
		ImageData:       imageData,
		ImageMIME:       imageMIME,
		ImageEffects:    imageEffects,
	}
}

func parseLegacyPptPictureShapeNode(node *officeartescher.ShapeNode, frame legacyPptGroupFrame, colorScheme map[uint8]string) (legacyPptPictureShape, bool) {
	var shape legacyPptPictureShape
	if node == nil {
		return legacyPptPictureShape{}, false
	}
	// Preserve the OfficeArt identity at the picture-family decode boundary.
	// BIG37 uses OriginalShapeID to reconcile richer picture/media/OLE
	// projections with the generic AutoShape projection from the same scene.
	shape.shapeID = node.ShapeID
	shape.officeArt = officeartlegacy.ShapeFromEscher(node)
	shape.transform.FlipH = node.Flags&legacyOfficeArtSpFlagFlipH != 0
	shape.transform.FlipV = node.Flags&legacyOfficeArtSpFlagFlipV != 0

	if len(node.Properties) > 0 {
		if blipRef, ok := parseLegacyPptPictureBlipRefProperties(node.Properties); ok {
			shape.blipRef = blipRef
		}
		parseLegacyPptFOPTPropertiesTransform(node.Properties, nil, &shape.transform)
		shape.imageEffects = parseLegacyPptPictureEffects(node.Properties)
		shape.mathColor = parseLegacyPptPictureRecolor(node.Properties, colorScheme)
	}
	if len(node.ClientData) > 0 {
		if exObjID, ok := parseLegacyPptExObjRef(node.ClientData); ok {
			shape.exObjID = exObjID
		}
		if anim, ok := parseLegacyPptShapeAnimationInfo(node.ClientData); ok {
			shape.animation = anim
		}
		// PowerPoint can recolor the cached WMF/EMF through a RecolorInfoAtom
		// stored in OfficeArtClientData. MathType equations in particular commonly
		// keep black ink in the metafile and map that ink to a slide-scheme color.
		// This mapping is independent of OfficeArt pictureRecolor and is what
		// PowerPoint applies when displaying the authored preview.
		shape.metafileRecolors = parseLegacyPptMetafileRecolors(node.ClientData, colorScheme)
		if shape.mathColor == "" {
			shape.mathColor = legacyPptMetafileForegroundFromRecolors(shape.metafileRecolors)
		}
	}

	placement := resolveLegacyPptShapePlacement(node, frame)

	haveAnchor := placement.valid()
	if haveAnchor {
		shape.bounds = placement.Bounds
	}
	// Picture-family ownership is established by the authored OfficeArt shape,
	// not by successful BLIP resolution. Embedded media/OLE shapes can carry an
	// ExObj reference without a poster BLIP, and damaged decks can retain the
	// picture SpContainer after its BStore entry is lost. Preserve that semantic
	// node so generic AutoShape projection cannot duplicate it.
	if node.ShapeType != legacyOfficeArtShapePicture || (shape.blipRef == 0 && shape.exObjID == 0) || !haveAnchor {
		return legacyPptPictureShape{}, false
	}
	return shape, true
}

func parseLegacyPptExObjRef(data []byte) (uint32, bool) {
	for pos := 0; pos+8 <= len(data); {
		record, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			break
		}
		if record.Type == pptRTExObjRefAtom && len(payload) >= 4 {
			return binary.LittleEndian.Uint32(payload[:4]), true
		}
		pos = next
	}
	return 0, false
}

func parseLegacyPptPictureRecolor(props []officeartescher.Property, colorScheme map[uint8]string) string {
	for _, prop := range props {
		if prop.IsComplex || prop.ID != legacyOfficeArtPropPictureRecolor || prop.Value == 0xFFFFFFFF {
			continue
		}
		// MS-ODRAW pictureRecolor is the authored color used to recolor the
		// picture. MathType OLE previews in legacy PPT use this instead of
		// rewriting the cached WMF/EMF glyph paint. Preserve it explicitly so
		// native MathML does not fall back to the preview's original black ink.
		if color := resolveLegacyPptOfficeArtColorRef(prop.Value, colorScheme); color != "" {
			return color
		}
	}
	return ""
}

func parseLegacyPptMetafileRecolors(clientData []byte, colorScheme map[uint8]string) []metafile.ColorReplacement {
	for pos := 0; pos+8 <= len(clientData); {
		record, payload, next, ok := readPptRecord(clientData, pos)
		if !ok {
			break
		}
		if record.Type == pptRTRecolorInfoAtom {
			return parseLegacyPptRecolorInfo(payload, colorScheme)
		}
		pos = next
	}
	return nil
}

func parseLegacyPptMetafileForegroundRecolor(clientData []byte, colorScheme map[uint8]string) string {
	return legacyPptMetafileForegroundFromRecolors(parseLegacyPptMetafileRecolors(clientData, colorScheme))
}

func legacyPptMetafileForegroundFromRecolors(recolors []metafile.ColorReplacement) string {
	for _, recolor := range recolors {
		if strings.EqualFold(strings.TrimPrefix(recolor.Source, "#"), "000000") {
			return strings.ToUpper(strings.TrimPrefix(recolor.Destination, "#"))
		}
	}
	return ""
}

// parseLegacyPptRecolorInfo decodes PowerPoint's metafile recolor table into
// renderer-independent source/destination paint replacements. RecolorInfoAtom
// entries are 44 bytes: flags, destination color, destination scheme index,
// and a 34-byte source color/brush variant. Color entries (variant 0) can be
// applied directly to vector WMF/EMF paints; fill/brush variants remain
// preserved for later semantic brush support rather than being guessed here.
func parseLegacyPptRecolorInfo(payload []byte, colorScheme map[uint8]string) []metafile.ColorReplacement {
	if len(payload) < 12 {
		return nil
	}
	flags := binary.LittleEndian.Uint16(payload[0:2])
	if flags&0x0001 == 0 { // fShouldRecolor
		return nil
	}
	cColors := int(binary.LittleEndian.Uint16(payload[2:4]))
	cFills := int(binary.LittleEndian.Uint16(payload[4:6]))

	if flags&0x0010 != 0 { // fMonoRecolor
		if destination := legacyPptWideColor(payload[6:12]); destination != "" {
			return []metafile.ColorReplacement{{Source: "000000", Destination: destination}}
		}
		return nil
	}

	const entrySize = 44
	result := make([]metafile.ColorReplacement, 0, cColors)
	for i := 0; i < cColors+cFills; i++ {
		off := 12 + i*entrySize
		if off+entrySize > len(payload) {
			break
		}
		entry := payload[off : off+entrySize]
		if binary.LittleEndian.Uint16(entry[0:2])&0x0001 == 0 { // fDoRecolor
			continue
		}
		variant := entry[10:44]
		if i >= cColors || len(variant) < 8 || binary.LittleEndian.Uint16(variant[0:2]) != 0 {
			continue
		}
		source := legacyPptWideColor(variant[2:8])
		if source == "" {
			continue
		}
		var destination string
		toIndex := entry[8]
		if toIndex < 8 {
			destination = colorScheme[toIndex]
		} else {
			destination = legacyPptWideColor(entry[2:8])
		}
		if destination == "" {
			continue
		}
		result = append(result, metafile.ColorReplacement{Source: source, Destination: destination})
	}
	return result
}

func parseLegacyPptRecolorInfoForeground(payload []byte, colorScheme map[uint8]string) string {
	return legacyPptMetafileForegroundFromRecolors(parseLegacyPptRecolorInfo(payload, colorScheme))
}

func legacyPptWideColor(data []byte) string {
	if len(data) < 6 {
		return ""
	}
	component := func(raw uint16) byte {
		// WideColorStruct stores 8-bit sRGB precision in a 16-bit channel. Older
		// PowerPoint writers use the high byte (0xFF00 for 255); tolerate literal
		// 0..255 values as well for producer compatibility.
		if raw > 0x00FF {
			return byte(raw >> 8)
		}
		return byte(raw)
	}
	r := component(binary.LittleEndian.Uint16(data[0:2]))
	g := component(binary.LittleEndian.Uint16(data[2:4]))
	b := component(binary.LittleEndian.Uint16(data[4:6]))
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

func parseLegacyPptPictureEffects(props []officeartescher.Property) []drawing.Effect {
	var effects []drawing.Effect
	for _, prop := range props {
		if prop.IsComplex || prop.ID != legacyOfficeArtPropPictureTransparent {
			continue
		}
		// MS-ODRAW pictureTransparent is an OfficeArtCOLORREF color key. Pixels
		// matching this color are transparent; it is not a replacement/fill color.
		// Preserve that semantic in the same canonical blip-effect representation
		// used by DrawingML clrChange so layout/rendering stay format-neutral.
		color := resolveLegacyPptOfficeArtColorRef(prop.Value, nil)
		if color == "" {
			continue
		}
		effects = append(effects, drawing.ClrChange{From: color, To: color, UseAlpha: true, ToAlpha: 0})
	}
	return effects
}

func parseLegacyPptPictureBlipRefProperties(props []officeartescher.Property) (uint32, bool) {
	for _, prop := range props {
		if prop.ID == legacyOfficeArtPropPib {
			return prop.Value, true
		}
	}
	return 0, false
}

// legacyPptTransitionType maps the SSlideInfoAtom effectType byte to the
// OOXML transition vocabulary used by SlideTransition.Type. Values come from
// MS-PPT §2.13.21 SlideShowSlideInfoAtomEffectType. The mapping favors
// recognized OOXML names; unknown values map to "" (the IR's "unspecified").
func legacyPptTransitionType(effect byte) string {
	switch effect {
	case 0:
		return "cut"
	case 1:
		return "random"
	case 2:
		return "blinds"
	case 3:
		return "checker"
	case 4:
		return "cover"
	case 5:
		return "dissolve"
	case 6:
		return "fade"
	case 7:
		return "pull"
	case 8:
		return "randomBar"
	case 9:
		return "strips"
	case 10:
		return "wipe"
	case 11:
		return "zoom"
	case 12:
		return "split"
	case 13:
		return "wheel"
	case 17:
		return "push"
	case 18:
		return "newsflash"
	case 19:
		return "circle"
	case 20:
		return "diamond"
	case 21:
		return "plus"
	case 22:
		return "wedge"
	case 23:
		return "fly"
	}
	return ""
}

func legacyPptAnimationEffectName(effect byte) string {
	return legacyPptTransitionType(effect)
}

func normalizeLegacyPptRotatedAnchorBounds(bounds presentationir.Rect, rotation float64) presentationir.Rect {
	// Binary PPT anchors use the orientation quadrant of the rotated shape. When
	// the rotation is closer to 90/270 degrees than to 0/180, the stored anchor
	// axes are transposed relative to canonical DrawingML-style pre-rotation
	// bounds. Convert that representation back to a centered pre-rotation frame.
	//
	// Work in the 0..180 orientation domain instead of comparing sin/cos. At the
	// exact 45/135-degree boundaries both axis choices are equally valid and we
	// preserve the authored anchor. The epsilon prevents floating-point noise at
	// those exact boundaries from silently swapping an otherwise stable frame.
	orientation := math.Mod(rotation, 180)
	if orientation < 0 {
		orientation += 180
	}
	const boundaryEpsilon = 1e-9
	if orientation <= 45+boundaryEpsilon || orientation >= 135-boundaryEpsilon {
		return bounds
	}

	// Swap width/height around the same center. Integer division intentionally
	// truncates toward zero; that matches the integer anchor convention used by
	// legacy PPT and avoids inventing half-unit coordinates.
	return presentationir.Rect{
		X: bounds.X + (bounds.W-bounds.H)/2,
		Y: bounds.Y + (bounds.H-bounds.W)/2,
		W: bounds.H,
		H: bounds.W,
	}
}

func canonicalLegacyPptLeafRotation(rotation float64, flipH, flipV bool) float64 {
	// Legacy OfficeArt leaf transforms compose reflection and rotation in the
	// opposite order from the canonical presentation transform. Conjugating a
	// rotation by one reflection reverses its direction; two reflections commute.
	if flipH != flipV {
		rotation = -rotation
	}
	return presentationir.CanonicalRotationDeg(rotation)
}

// applyLegacyPptShapeTransform copies rotation/flip/line properties from a
// legacy FSP+FOPT decode onto an IR Shape. Line color/width are emitted only
// when the FOPT line booleans either say "use" or, in their absence, when an
// explicit line property was actually present (the zero values mean "unset").
func applyLegacyPptShapeTransform(shape *presentationir.Shape, t legacyPptShapeTransform) {
	if shape == nil {
		return
	}
	if t.HasRotation {
		// Binary PPT stores anchors in the shape's orientation quadrant. Canonical
		// IR, like DrawingML, stores a pre-rotation frame plus RotationDeg. Convert
		// transposed legacy quadrants back to that centered canonical frame before
		// shared layout sees them.
		shape.Bounds = normalizeLegacyPptRotatedAnchorBounds(shape.Bounds, t.Rotation)
		shape.RotationDeg = canonicalLegacyPptLeafRotation(t.Rotation, t.FlipH, t.FlipV)
	}
	if t.FlipH {
		shape.FlipH = true
	}
	if t.FlipV {
		shape.FlipV = true
	}

	// Line disablement is local to the stroke. It must not discard text-body,
	// crop, WordArt or other FOPT semantics carried by the same property table.
	lineEnabled := !t.HasLineEnable || t.LineEnabled
	if lineEnabled {
		if t.LineColor != "" {
			shape.LineColor = t.LineColor
		}
		if t.LineWidthEMU > 0 {
			shape.LineWidthPt = float64(t.LineWidthEMU) / 12700.0
		}
		if t.LineColor != "" || t.LineWidthEMU > 0 || t.LineDash != "" {
			// The shared OfficeArt projection is applied before this compatibility
			// transform. Merge the legacy FOPT values into that canonical line instead
			// of recreating a generic DrawingML default and losing OfficeArt defaults
			// such as round joins.
			ls := drawing.DefaultLineStyle()
			if shape.LineStyle != nil {
				ls = *shape.LineStyle
			}
			if t.LineColor != "" {
				ls.Color = t.LineColor
			}
			if t.LineWidthEMU > 0 {
				ls.Width = float64(t.LineWidthEMU) / 12700.0
			}
			if t.LineDash != "" {
				ls.Dash = t.LineDash
			}
			shape.LineStyle = &ls
		}
		if t.ArrowHead != "" {
			shape.ArrowHead = t.ArrowHead
			if t.ArrowHead == "none" {
				shape.ArrowHeadWidth, shape.ArrowHeadLength = "", ""
			} else {
				if t.ArrowHeadWidth != "" {
					shape.ArrowHeadWidth = t.ArrowHeadWidth
				}
				if t.ArrowHeadLength != "" {
					shape.ArrowHeadLength = t.ArrowHeadLength
				}
				if shape.ArrowHeadWidth == "" {
					shape.ArrowHeadWidth = "med"
				}
				if shape.ArrowHeadLength == "" {
					shape.ArrowHeadLength = "med"
				}
			}
		}
		if t.ArrowTail != "" {
			shape.ArrowTail = t.ArrowTail
			if t.ArrowTail == "none" {
				shape.ArrowTailWidth, shape.ArrowTailLength = "", ""
			} else {
				if t.ArrowTailWidth != "" {
					shape.ArrowTailWidth = t.ArrowTailWidth
				}
				if t.ArrowTailLength != "" {
					shape.ArrowTailLength = t.ArrowTailLength
				}
				if shape.ArrowTailWidth == "" {
					shape.ArrowTailWidth = "med"
				}
				if shape.ArrowTailLength == "" {
					shape.ArrowTailLength = "med"
				}
			}
		}
		shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength = presentationir.CanonicalArrowEnd(shape.ArrowHead, shape.ArrowHeadWidth, shape.ArrowHeadLength)
		shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength = presentationir.CanonicalArrowEnd(shape.ArrowTail, shape.ArrowTailWidth, shape.ArrowTailLength)
	} else {
		shape.LineColor = ""
		shape.LineWidthPt = 0
		shape.LineStyle = nil
		shape.ArrowHead = ""
		shape.ArrowHeadWidth = ""
		shape.ArrowHeadLength = ""
		shape.ArrowTail = ""
		shape.ArrowTailWidth = ""
		shape.ArrowTailLength = ""
	}

	if t.HasTextAnchor {
		shape.TextAnchor = t.TextAnchor
		shape.TextBodyProps.Anchor = t.TextAnchor
	}
	if t.HasAnchorCenter {
		shape.TextBodyProps.AnchorCtr = t.AnchorCenter
	}
	if t.HasTextWrap {
		shape.TextBodyProps.Wrap = t.TextWrap
	}
	if t.HasTextVertical {
		shape.TextBodyProps.Vert = t.TextVertical
	}
	if t.HasTextInsets {
		normalizeLegacyTextInsetPresence(&t)
		shape.HasTextInsets = true
		// OfficeArt insets are independent properties.  Materialize absent edges
		// with the same physical defaults used by the canonical text-frame layer
		// instead of turning an omitted edge into an explicit zero inset.
		shape.TextInsetLeftEMU = 91440
		shape.TextInsetRightEMU = 91440
		shape.TextInsetTopEMU = 45720
		shape.TextInsetBottomEMU = 45720
		if t.HasTextInsetLeft {
			shape.TextInsetLeftEMU = t.TextInsetLeft
		}
		if t.HasTextInsetTop {
			shape.TextInsetTopEMU = t.TextInsetTop
		}
		if t.HasTextInsetRight {
			shape.TextInsetRightEMU = t.TextInsetRight
		}
		if t.HasTextInsetBottom {
			shape.TextInsetBottomEMU = t.TextInsetBottom
		}
	}
	if t.HasAllowOverflow {
		shape.TextAllowOverflow = t.AllowOverflow
	}
	if t.HasAutoGrow {
		shape.TextAutoGrow = t.AutoGrow
	}
	if t.HasImageCrop {
		shape.ImageCropTop = t.CropTop
		shape.ImageCropBottom = t.CropBottom
		shape.ImageCropLeft = t.CropLeft
		shape.ImageCropRight = t.CropRight
	}
	if t.HasGeometryText && t.GeometryText.Text != "" && len(shape.Paragraphs) == 0 {
		shape.Paragraphs = legacyPptGeometryTextParagraphs(t.GeometryText)
	}
}

func legacyPptGeometryTextParagraphs(text officeartlegacy.GeometryText) []*presentationir.Paragraph {
	if text.Text == "" {
		return nil
	}
	align := "l"
	switch text.Align {
	case 1:
		align = "ctr"
	case 2:
		align = "r"
	case 3:
		align = "just"
	}
	run := &presentationir.Run{
		Text:       text.Text,
		FontFamily: text.Font,
		Bold:       text.Bold,
		Italic:     text.Italic,
		BoldSet:    text.Bold,
		ItalicSet:  text.Italic,
	}
	if text.Size > 0 {
		run.FontSize = int((int64(text.Size) * 100) / 65536)
	}
	return []*presentationir.Paragraph{{
		Align: align,
		Runs:  []*presentationir.Run{run},
	}}
}

func applyLegacyPptAnimationMetadata(shape *presentationir.Shape, info legacyPptAnimationInfo) {
	if shape == nil {
		return
	}
	if info.Step > 0 {
		shape.AnimStep = info.Step
	}
	if info.Trigger != "" {
		shape.AnimationTriggers = appendUniqueLegacyString(shape.AnimationTriggers, info.Trigger)
	}
	if info.DelayMs > 0 {
		shape.AnimationDelayMs = info.DelayMs
	}
	if info.Effect != "" {
		shape.EmphasisEffects = append(shape.EmphasisEffects, info.Effect)
	}
}

func appendUniqueLegacyString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// parseLegacyPptFOPTTransform reads rotation, line color/width/dash, and the
// line-enable flag from a FOPT record body. It mutates the receiver in-place
// so that consumers can call it once per FOPT (primary, secondary, tertiary)
// and have the latest properties win, mirroring how PowerPoint resolves shape
// formatting at render time.
func parseLegacyPptFOPTTransform(body []byte, propCount int, colorScheme map[uint8]string, out *legacyPptShapeTransform) {
	if out == nil {
		return
	}
	props, err := officeartescher.ParseFOPT(body, propCount)
	if err != nil {
		return
	}
	parseLegacyPptFOPTPropertiesTransform(props, colorScheme, out)
}

func parseLegacyPptFOPTPropertiesTransform(props []officeartescher.Property, colorScheme map[uint8]string, out *legacyPptShapeTransform) {
	if out == nil {
		return
	}
	resolve := func(raw uint32) string {
		return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
	}
	visual := officeartlegacy.DecodeVisualProperties(props, resolve)
	if geometryText := officeartlegacy.DecodeGeometryText(props); geometryText.Present {
		out.HasGeometryText = true
		out.GeometryText = geometryText
	}

	if visual.HasRotation {
		out.Rotation = visual.RotationDeg
		out.HasRotation = true
	}
	if visual.HasLineColor {
		out.LineColor = visual.LineColor
	}
	if visual.HasLineWidth {
		out.LineWidthEMU = visual.LineWidthEMU
	}
	if visual.HasLineDash {
		out.LineDash = legacyPptDashPattern(visual.LineDash)
	}
	if visual.HasLine {
		out.HasLineEnable = true
		out.LineEnabled = visual.LineEnabled
	}
	// Canonical arrow semantics follow DrawingML and the shared vector builder:
	// ArrowHead decorates the first/start point, ArrowTail the last/end point.
	// OfficeArt lineStartArrow/lineEndArrow already use that same geometric
	// orientation, so preserve the shared Escher decode directly. The older PPT
	// compatibility remap both reversed endpoints and shifted the enum values,
	// causing converted PPT/PPTX pairs to disagree (for example raw 2 is stealth).
	if visual.StartArrow != "" {
		out.ArrowHead = string(visual.StartArrow)
	}
	if visual.EndArrow != "" {
		out.ArrowTail = string(visual.EndArrow)
	}
	if visual.StartArrowWidth != "" {
		out.ArrowHeadWidth = string(visual.StartArrowWidth)
	}
	if visual.StartArrowLength != "" {
		out.ArrowHeadLength = string(visual.StartArrowLength)
	}
	if visual.EndArrowWidth != "" {
		out.ArrowTailWidth = string(visual.EndArrowWidth)
	}
	if visual.EndArrowLength != "" {
		out.ArrowTailLength = string(visual.EndArrowLength)
	}

	text := visual.Text
	if text.HasLeft || text.HasTop || text.HasRight || text.HasBottom {
		out.HasTextInsets = true
		out.HasTextInsetLeft = text.HasLeft
		out.HasTextInsetTop = text.HasTop
		out.HasTextInsetRight = text.HasRight
		out.HasTextInsetBottom = text.HasBottom
		if text.HasLeft {
			out.TextInsetLeft = text.Insets.Left
		}
		if text.HasTop {
			out.TextInsetTop = text.Insets.Top
		}
		if text.HasRight {
			out.TextInsetRight = text.Insets.Right
		}
		if text.HasBottom {
			out.TextInsetBottom = text.Insets.Bottom
		}
	}
	if text.HasAnchor {
		out.HasTextAnchor = true
		switch text.Anchor {
		case officeartir.TextAnchorCenter:
			out.TextAnchor = "ctr"
		case officeartir.TextAnchorBottom:
			out.TextAnchor = "b"
		default:
			out.TextAnchor = "t"
		}
		out.HasAnchorCenter = true
		out.AnchorCenter = text.AnchorCenter
	}
	if text.HasWrap {
		out.HasTextWrap = true
		out.TextWrap = string(text.Wrap)
	}
	if text.HasVert {
		out.HasTextVertical = true
		out.TextVertical = string(text.Vert)
	}
	if text.HasAutoFit {
		// OfficeArt's fitShapeToText ownership is independent of wrapText.
		// Keep the canonical layout policy presence-aware so an explicit local
		// "off" also overrides an inherited spAutoFit contract.
		out.HasAutoGrow = true
		out.AutoGrow = text.AutoFit == officeartir.AutoFitShape
		out.HasAllowOverflow = true
		out.AllowOverflow = text.AutoFit == officeartir.AutoFitNone
	}
	if visual.HasCrop {
		out.HasImageCrop = true
		out.CropTop = visual.Crop.Top
		out.CropBottom = visual.Crop.Bottom
		out.CropLeft = visual.Crop.Left
		out.CropRight = visual.Crop.Right
	}
}

func legacyPptDashPattern(name string) drawing.DashPattern {
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

func legacyPptArrowStyle(raw uint32) string {
	return string(officeartlegacy.PPTArrowKind(raw))
}

func legacyPptArrowSize(raw uint32) string {
	return string(officeartlegacy.PPTArrowSize(raw))
}

// parseLegacyPptFOPTFillColor extracts a solid-fill color from an OfficeArt
// FOPT record body. propCount is the number of 6-byte property entries encoded
// in the verInst field of the record header. Returns "" when the shape has no
// fill, a non-solid fill type, or no explicit fill-color property.
func parseLegacyPptFOPTFillColor(body []byte, propCount int, colorScheme map[uint8]string) (string, bool) {
	props, err := officeartescher.ParseFOPT(body, propCount)
	if err != nil {
		return "", false
	}
	return parseLegacyPptFOPTPropertiesFillColor(props, colorScheme)
}

// Text boxes/placeholders are transparent by default in binary PowerPoint.
// A fillColor property can still be present as an inherited/default OfficeArt value;
// it becomes paint only when fillStyleBooleanProperties explicitly enables fFilled.
func parseLegacyPptFOPTPropertiesTextFillColor(props []officeartescher.Property, colorScheme map[uint8]string) (string, bool) {
	resolve := func(raw uint32) string {
		return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
	}
	visual := officeartlegacy.DecodeVisualProperties(props, resolve)
	// For text containers, only an authored fFilled use/value pair transfers
	// paint ownership. Other fillStyleBoolean bits are frequently present on
	// legacy placeholders and must not turn a cached fillColor into a box.
	explicitFilled := false
	for _, prop := range props {
		if prop.IsComplex || prop.IsBlip || prop.ID != legacyOfficeArtPropFillBools {
			continue
		}
		valueBits := prop.Value & 0xFFFF
		useBits := (prop.Value >> 16) & 0xFFFF
		if useBits&(1<<4) != 0 {
			explicitFilled = valueBits&(1<<4) != 0
		}
	}
	if !explicitFilled || visual.FillType > 0 || !visual.HasFillColor || visual.FillColor == "" {
		return "", false
	}
	return visual.FillColor, true
}

func parseLegacyPptFOPTPropertiesFillColor(props []officeartescher.Property, colorScheme map[uint8]string) (string, bool) {
	resolve := func(raw uint32) string {
		return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
	}
	visual := officeartlegacy.DecodeVisualProperties(props, resolve)
	if visual.HasFill && !visual.FillEnabled {
		return "", false
	}
	if visual.FillType > 0 || !visual.HasFillColor || visual.FillColor == "" {
		return "", false
	}
	return visual.FillColor, true
}

// parseLegacyPptBgFillFromContainer looks for a BackgroundContainer
// (RT_Background = 0x0410) inside a container payload and returns the solid
// fill color when one is explicitly set via BackgroundAtom + PPDrawing.
func parseLegacyPptBgFillFromContainer(payload []byte, colorScheme map[uint8]string) string {
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTBackground {
			if fill := parseLegacyPptBackgroundContainerFill(childPayload, colorScheme); fill != "" {
				return fill
			}
		}
		pos = next
	}
	return ""
}

// parseLegacyPptBackgroundContainerFill extracts an explicit fill color from a
// BackgroundContainer body. The container holds a BackgroundAtom (type 0x0411)
// whose bit 0 signals an explicit client fill, followed by a PPDrawing whose
// OfficeArt FOPT properties carry the fill color.
func parseLegacyPptBackgroundContainerFill(payload []byte, colorScheme map[uint8]string) string {
	clientFillSpecified := false
	var ppDrawingPayload []byte
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch child.Type {
		case pptRTBackgroundAtom:
			if len(childPayload) >= 4 {
				flags := binary.LittleEndian.Uint32(childPayload[:4])
				clientFillSpecified = (flags & 1) != 0
			}
		case pptRTPPDrawing:
			ppDrawingPayload = childPayload
		}
		pos = next
	}
	if !clientFillSpecified || len(ppDrawingPayload) == 0 {
		return ""
	}
	var fillColor string
	walkLegacyPptOfficeArtRecords(ppDrawingPayload, func(recType, inst, _ uint16, body []byte) {
		if recType == officeartescher.RecordFOPT || recType == officeartescher.RecordSecondaryFOPT || recType == officeartescher.RecordTertiaryFOPT {
			// Primary -> secondary -> tertiary is an override cascade. Do not stop
			// after the first color: a later table is authoritative when present.
			if color, ok := parseLegacyPptFOPTFillColor(body, int(inst), colorScheme); ok {
				fillColor = color
			}
		}
	})
	return fillColor
}

// parseLegacyPptBackgroundContainerDrawingFill resolves the complete OfficeArt
// fill surface carried by a BackgroundContainer. Unlike the legacy color helper,
// this preserves gradients/patterns/noFill in canonical DrawingML form.
func parseLegacyPptBackgroundContainerDrawingFill(payload []byte, colorScheme map[uint8]string) drawing.Fill {
	clientFillSpecified := false
	var ppDrawingPayload []byte
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		switch child.Type {
		case pptRTBackgroundAtom:
			if len(childPayload) >= 4 {
				clientFillSpecified = binary.LittleEndian.Uint32(childPayload[:4])&1 != 0
			}
		case pptRTPPDrawing:
			ppDrawingPayload = childPayload
		}
		pos = next
	}
	if !clientFillSpecified || len(ppDrawingPayload) == 0 {
		return nil
	}
	var fill drawing.Fill
	walkLegacyPptOfficeArtRecords(ppDrawingPayload, func(recType, _, _ uint16, body []byte) {
		if recType != legacyOfficeArtSpContainer {
			return
		}
		node, err := officeartescher.ParseShapeContainerPayload(body)
		if err != nil || node == nil {
			return
		}
		shared := officeartlegacy.ShapeFromEscherWithColorResolver(node, func(raw uint32) string {
			return resolveLegacyPptOfficeArtColorRef(raw, colorScheme)
		})
		if shared == nil {
			return
		}
		tmp := &presentationir.Shape{}
		applyLegacyPptOfficeArtVisual(tmp, shared)
		if tmp.Fill != nil {
			fill = tmp.Fill
		}
	})
	return fill
}

func parseLegacyPptBgDrawingFillFromContainer(payload []byte, colorScheme map[uint8]string) drawing.Fill {
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTBackground {
			if fill := parseLegacyPptBackgroundContainerDrawingFill(childPayload, colorScheme); fill != nil {
				return fill
			}
		}
		pos = next
	}
	return nil
}

func parseLegacyPptGroupFrameFromSpgrContainer(data []byte, parent legacyPptGroupFrame) (legacyPptGroupFrame, bool) {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		if recType == legacyOfficeArtSpContainer {
			if frame, ok := parseLegacyPptGroupFrameFromSpContainer(body, parent); ok {
				return frame, true
			}
		}
		if verInst&0x000F == 0x000F {
			if frame, ok := parseLegacyPptGroupFrameFromSpgrContainer(body, parent); ok {
				return frame, true
			}
		}
		off += 8 + recLen
	}
	return legacyPptGroupFrame{}, false
}

func parseLegacyPptGroupFrameFromSpContainer(data []byte, parent legacyPptGroupFrame) (legacyPptGroupFrame, bool) {
	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil || node == nil || node.GroupRect == nil {
		return legacyPptGroupFrame{}, false
	}

	domainIR, ok := officeartlegacy.NormalizeRect(
		int64(node.GroupRect.Left),
		int64(node.GroupRect.Top),
		int64(node.GroupRect.Right),
		int64(node.GroupRect.Bottom),
	)
	if !ok {
		return legacyPptGroupFrame{}, false
	}
	domain := legacyPptRectFromOfficeArt(domainIR)

	var clientAnchorBounds, childAnchorBounds presentationir.Rect
	haveClientAnchor, haveChildAnchor := false, false
	if len(node.ClientAnchor) > 0 {
		if rect, ok := parseLegacyPptClientAnchor(node.ClientAnchor); ok {
			clientAnchorBounds = rect
			haveClientAnchor = true
		}
	}
	if len(node.ChildAnchor) > 0 {
		if rect, ok := parseLegacyPptChildAnchor(node.ChildAnchor); ok {
			// Keep the authored anchor for canonical group IR, while retaining a
			// materialized target for legacy discovery paths outside the IR boundary.
			childAnchorBounds = rect
			haveChildAnchor = true
		}
	}

	var localTarget, target presentationir.Rect
	switch {
	case haveClientAnchor:
		localTarget = clientAnchorBounds
		target = clientAnchorBounds
	case haveChildAnchor:
		localTarget = childAnchorBounds
		target = childAnchorBounds
		if parent.valid {
			if mapped, ok := mapLegacyPptRectThroughGroupFrame(childAnchorBounds, parent); ok {
				target = mapped
			}
		}
	default:
		return legacyPptGroupFrame{}, false
	}
	if localTarget.W == 0 || localTarget.H == 0 {
		return legacyPptGroupFrame{}, false
	}
	return legacyPptGroupFrame{domain: domain, localTarget: localTarget, target: target, valid: true}, true
}

func parseLegacyPptChildAnchor(body []byte) (presentationir.Rect, bool) {
	if len(body) < 16 {
		return presentationir.Rect{}, false
	}
	rect, ok := officeartlegacy.NormalizeRect(
		int64(int32(binary.LittleEndian.Uint32(body[0:4]))),
		int64(int32(binary.LittleEndian.Uint32(body[4:8]))),
		int64(int32(binary.LittleEndian.Uint32(body[8:12]))),
		int64(int32(binary.LittleEndian.Uint32(body[12:16]))),
	)
	return legacyPptRectFromOfficeArt(rect), ok
}

func mapLegacyPptRectThroughGroupFrame(local presentationir.Rect, frame legacyPptGroupFrame) (presentationir.Rect, bool) {
	mapped, ok := officeartlegacy.MapRectThroughGroup(
		legacyPptRectToOfficeArt(local),
		legacyPptGroupFrameToOfficeArt(frame),
	)
	return legacyPptRectFromOfficeArt(mapped), ok
}

func legacyPptCanonicalGroupTransform(frame legacyPptGroupFrame, transform legacyPptShapeTransform) presentationir.VisualTransform {
	rotation := 0.0
	if transform.HasRotation {
		rotation = transform.Rotation
	}
	return presentationir.VisualTransform{
		Offset:      presentationir.Point{X: frame.localTarget.X, Y: frame.localTarget.Y},
		Extent:      presentationir.VisualSize{W: frame.localTarget.W, H: frame.localTarget.H},
		ChildOffset: presentationir.Point{X: frame.domain.X, Y: frame.domain.Y},
		ChildExtent: presentationir.VisualSize{W: frame.domain.W, H: frame.domain.H},
		Rotation:    rotation,
		FlipH:       transform.FlipH,
		FlipV:       transform.FlipV,
	}
}

func parseLegacyPptClientAnchor(body []byte) (presentationir.Rect, bool) {
	// [MS-PPT] OfficeArtClientAnchorData stores both RectStruct (16-byte)
	// and SmallRectStruct (8-byte) as top, left, right, bottom. Coordinates
	// are signed; negative anchors are valid for objects extending off-slide.
	var top, left, right, bottom int64
	switch {
	case len(body) >= 16:
		top = int64(int32(binary.LittleEndian.Uint32(body[0:4])))
		left = int64(int32(binary.LittleEndian.Uint32(body[4:8])))
		right = int64(int32(binary.LittleEndian.Uint32(body[8:12])))
		bottom = int64(int32(binary.LittleEndian.Uint32(body[12:16])))
	case len(body) >= 8:
		top = int64(int16(binary.LittleEndian.Uint16(body[0:2])))
		left = int64(int16(binary.LittleEndian.Uint16(body[2:4])))
		right = int64(int16(binary.LittleEndian.Uint16(body[4:6])))
		bottom = int64(int16(binary.LittleEndian.Uint16(body[6:8])))
	default:
		return presentationir.Rect{}, false
	}
	if right < left {
		left, right = right, left
	}
	if bottom < top {
		top, bottom = bottom, top
	}
	if right == left || bottom == top {
		return presentationir.Rect{}, false
	}
	return presentationir.Rect{
		X: left * defaultSlideWidthEMU / legacyPptMasterWidth,
		Y: top * defaultSlideHeightEMU / legacyPptMasterHeight,
		W: (right - left) * defaultSlideWidthEMU / legacyPptMasterWidth,
		H: (bottom - top) * defaultSlideHeightEMU / legacyPptMasterHeight,
	}, true
}

func walkLegacyPptOfficeArtRecords(data []byte, visit func(recType, inst, version uint16, body []byte)) {
	var walk func([]byte)
	walk = func(buf []byte) {
		for off := 0; off+8 <= len(buf); {
			verInst := binary.LittleEndian.Uint16(buf[off : off+2])
			recType := binary.LittleEndian.Uint16(buf[off+2 : off+4])
			recLen := int(binary.LittleEndian.Uint32(buf[off+4 : off+8]))
			if recLen < 0 || off+8+recLen > len(buf) {
				return
			}
			version := verInst & 0x000F
			inst := verInst >> 4
			body := buf[off+8 : off+8+recLen]
			visit(recType, inst, version, body)
			if version == 0x000F {
				walk(body)
			}
			off += 8 + recLen
		}
	}
	walk(data)
}

func mergeLegacySlideByPersistID(slides []legacySlideText, candidate legacySlideText) bool {
	if candidate.PersistID == 0 {
		return false
	}
	for index := range slides {
		if slides[index].PersistID != candidate.PersistID {
			continue
		}
		slides[index].Fragments = mergeSlideFragments(slides[index].Fragments, candidate.Fragments)
		return true
	}
	return false
}

func mergeSlideFragments(existing, incoming []legacySlideFragment) []legacySlideFragment {
	seen := make(map[string]struct{}, len(existing))
	for _, fragment := range existing {
		seen[fragment.Text] = struct{}{}
	}
	for _, fragment := range incoming {
		if fragment.Text == "" {
			continue
		}
		if _, ok := seen[fragment.Text]; ok {
			continue
		}
		existing = append(existing, fragment)
		seen[fragment.Text] = struct{}{}
	}
	return existing
}

// parseTextRulerAtom decodes TextRulerAtom (0x0FA6) according to POI's model:
// mask (uint16), reserved (uint16), then optional values for default tab,
// level count, tab stops, left margins, and bullet indents.
func parseTextRulerAtom(payload []byte) (legacyPptTextRuler, bool) {
	var ruler legacyPptTextRuler
	if len(payload) < 4 {
		return ruler, false
	}
	mask := binary.LittleEndian.Uint16(payload[0:2])
	pos := 4

	if mask&0x0001 != 0 {
		if pos+2 > len(payload) {
			return ruler, false
		}
		ruler.defaultTabSize = int(int16(binary.LittleEndian.Uint16(payload[pos : pos+2])))
		ruler.hasDefaultTab = true
		pos += 2
	}

	nLevels := 5
	if mask&0x0002 != 0 {
		if pos+2 > len(payload) {
			return ruler, false
		}
		nLevels = int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
		if nLevels < 0 {
			nLevels = 0
		}
		if nLevels > 5 {
			nLevels = 5
		}
		pos += 2
	}

	if mask&0x0004 != 0 {
		if pos+2 > len(payload) {
			return ruler, false
		}
		count := int(binary.LittleEndian.Uint16(payload[pos : pos+2]))
		pos += 2
		if pos+count*4 > len(payload) {
			return ruler, false
		}
		ruler.hasTabs = true
		ruler.tabs = make([]presentationir.TabStop, 0, count)
		for i := 0; i < count; i++ {
			posMu := int16(binary.LittleEndian.Uint16(payload[pos : pos+2]))
			tabType := binary.LittleEndian.Uint16(payload[pos+2 : pos+4])
			pos += 4
			tab := presentationir.TabStop{Pos: int64(legacyPptMasterUnitsToEMU(int(posMu)))}
			switch tabType {
			case 0:
				tab.Algn = "l"
			case 1:
				tab.Algn = "ctr"
			case 2:
				tab.Algn = "r"
			case 3:
				tab.Algn = "dec"
			}
			ruler.tabs = append(ruler.tabs, tab)
		}
	}

	for level := 0; level < nLevels; level++ {
		if mask&(1<<uint(3+level)) != 0 {
			if pos+2 > len(payload) {
				return ruler, false
			}
			ruler.leftMargins[level] = int(int16(binary.LittleEndian.Uint16(payload[pos : pos+2])))
			ruler.hasLeft[level] = true
			pos += 2
		}
	}
	for level := 0; level < nLevels; level++ {
		if mask&(1<<uint(8+level)) != 0 {
			if pos+2 > len(payload) {
				return ruler, false
			}
			ruler.indents[level] = int(int16(binary.LittleEndian.Uint16(payload[pos : pos+2])))
			ruler.hasIndent[level] = true
			pos += 2
		}
	}

	if ruler.hasDefaultTab || ruler.hasTabs {
		return ruler, true
	}
	for i := 0; i < 5; i++ {
		if ruler.hasLeft[i] || ruler.hasIndent[i] {
			return ruler, true
		}
	}
	return ruler, false
}

// applyRulerLevelsToParas applies ruler-defined margins/indents when paragraph
// values are absent and uses ruler left-margins to infer paragraph level.
// TextRulerAtom values use PowerPoint master units (1/8 point).
func legacyPptEMUToNearestMasterUnits(emu int) int {
	if emu >= 0 {
		return (emu*2 + 1587) / 3175
	}
	return -(((-emu)*2 + 1587) / 3175)
}

func applyRulerLevelsToParas(paras []*presentationir.Paragraph, ruler legacyPptTextRuler) {
	if len(paras) == 0 {
		return
	}
	for _, p := range paras {
		if p == nil {
			continue
		}

		if p.HasMarginLeft {
			muVal := legacyPptEMUToNearestMasterUnits(p.MarginLeftEMU)
			best := -1
			bestDist := 0
			for i := 0; i < 5; i++ {
				if !ruler.hasLeft[i] {
					continue
				}
				d := pptAbsInt(muVal - ruler.leftMargins[i])
				if best < 0 || d < bestDist {
					bestDist = d
					best = i
				}
			}
			if best >= 0 {
				p.Level = best
			}
		}

		if !p.HasMarginLeft && p.Level >= 0 && p.Level < 5 && ruler.hasLeft[p.Level] {
			p.MarginLeftEMU = legacyPptMasterUnitsToEMU(ruler.leftMargins[p.Level])
			p.HasMarginLeft = true
		}
		if !p.HasTextIndent && p.Level >= 0 && p.Level < 5 && ruler.hasIndent[p.Level] {
			p.TextIndentEMU = legacyPptMasterUnitsToEMU(ruler.indents[p.Level])
			p.HasTextIndent = true
		}
		// TextRuler owns fallback tabbing only. Paragraph-level PFException
		// values remain authoritative, including an explicitly empty tab list.
		if p.DefaultTabSizeEMU == 0 && ruler.hasDefaultTab && ruler.defaultTabSize > 0 {
			p.DefaultTabSizeEMU = legacyPptMasterUnitsToEMU(ruler.defaultTabSize)
		}
		if len(p.TabStops) == 0 && ruler.hasTabs {
			p.TabStops = append([]presentationir.TabStop(nil), ruler.tabs...)
		}
	}
}

func pptAbsInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// applyStyleTextProp9ToParas applies STPA9 auto-numbering data to paragraphs.
// stpa9Data is a map from indent level to auto-numbering scheme and start number.
func applyStyleTextProp9ToParas(paras []*presentationir.Paragraph, stpa9Data map[int]textPFException9) {
	if len(paras) == 0 || len(stpa9Data) == 0 {
		return
	}

	for _, p := range paras {
		if p == nil {
			continue
		}

		// Get the level for this paragraph (default 0 if not set)
		level := p.Level
		if level < 0 {
			level = 0
		}

		// Look up auto-numbering data for this level
		if exc, ok := stpa9Data[level]; ok {
			if exc.hasBulletAutoNumberSpecified && !exc.hasBulletAutoNumber {
				p.AutoNum = false
				p.AutoNumType = ""
				p.AutoNumStart = 0
				continue
			}
			if exc.hasBulletAutoNumber {
				p.AutoNum = true
				if exc.autoNumberScheme != "" {
					p.AutoNumType = exc.autoNumberScheme
				}
				if exc.autoNumberStart > 0 {
					p.AutoNumStart = exc.autoNumberStart
				}
			}
		}
	}
}

// PptProducer adapts the legacy binary parser to the shared presentation event
// producer contract. It deliberately hides CFB/PowerPoint record details from
// downstream consumers.
type PptProducer struct {
	Data     []byte
	Filename string
}

func (p PptProducer) ProducePresentationEvents(consumer presentationevent.Consumer) error {
	return WalkPptBytes(p.Data, p.Filename, consumer)
}
