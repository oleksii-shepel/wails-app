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

	officeartir "office/pkg/officeart/ir"
	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
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

	legacyOfficeArtShapePicture = 75
	legacyOfficeArtPropPib      = 0x0104

	// EscherSpRecord flags (MS-ODRAW §2.2.38).
	legacyOfficeArtSpFlagChild = 0x02 // shape is a child of a group shape
	legacyOfficeArtSpFlagFlipV = 0x40 // bit 6 = fFlipV
	legacyOfficeArtSpFlagFlipH = 0x80 // bit 7 = fFlipH

	// OfficeArt fill property IDs (MS-ODRAW §2.3.7).
	legacyOfficeArtPropFillType        = 0x0180 // fillType: 0=solid
	legacyOfficeArtPropFillColor       = 0x0181 // fillColor: COLORREF (R,G,B,extra)
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

type legacySlideFragment struct {
	Text         string                      // normalized (for rendering fallback)
	RawText      string                      // decoded text preserving \r (for STPA parsing)
	TextType     int                         // pptTextType* constant; pptTextTypeUnknown when no TextHeaderAtom precedes
	ShapeID      uint32                      // explicit ESCHER shape ID when available; 0 when unknown
	OutlineRef   int                         // per-slide OutlineTextRefAtom value; -1 when unknown
	Paragraphs   []*presentationir.Paragraph // non-nil when StyleTextPropAtom was parsed
	pendingRuler *legacyPptTextRuler         // ruler received before STPA; applied once STPA creates paragraphs
	pendingSTP9  interface{}                 // auto-numbering from STPA9; map[int]int encoded or marker for later application
}

type legacySlideText struct {
	PersistID         int
	MasterID          int // Master slide ID from SlideAtom; -2147483648 (0x80000000) means use MainMaster
	LayoutType        string
	NotesPersistID    int
	Fragments         []legacySlideFragment
	NotesHeaderFooter legacyPptHeaderFooterInfo
	Transition        *presentationir.SlideTransition
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
	blipRef   uint32
	bounds    presentationir.Rect
	transform legacyPptShapeTransform
	animation legacyPptAnimationInfo
	exObjID   uint32
}

type legacyPptConnectorShape struct {
	shapeID     uint32
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
	domain presentationir.Rect
	target presentationir.Rect
	valid  bool
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
	textType       int
	paragraphs     []*presentationir.Paragraph
	inlineFallback []*presentationir.Paragraph
	fillColor      string // sRGB hex from OfficeArt FOPT solid fill; "" when absent
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
	HasTextAnchor    bool
	TextAnchor       string // "t"|"ctr"|"b"
	HasTextInsets    bool
	TextInsetLeft    int64
	TextInsetTop     int64
	TextInsetRight   int64
	TextInsetBottom  int64
	HasAllowOverflow bool
	AllowOverflow    bool
	HasAutoGrow      bool
	AutoGrow         bool

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
	leftMargins [5]int
	hasLeft     [5]bool
	indents     [5]int
	hasIndent   [5]bool
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
	decryptedStream, decryptedPics, decryptErr := maybeDecryptPptStream(f, stream)
	if decryptErr != nil {
		return nil, decryptErr
	}
	stream = decryptedStream
	colorScheme := parseLegacyPptColorScheme(stream)
	fontTable := parseLegacyPptFontTable(stream)
	var slides []legacySlideText
	var slideShapes map[int][]*presentationir.Shape
	var theme *presentationir.Theme
	var masterTextStyles legacyPptMasterTextStyles
	var masterStylesByID map[int]legacyPptMasterTextStyles
	var masterBgFill string
	var exHyperlinks map[uint32]string
	var slideHeaderFooterDefaults legacyPptHeaderFooterInfo
	var slideHeaderFooters map[int]legacyPptHeaderFooterInfo
	var notesHeaderFooterDefaults legacyPptHeaderFooterInfo
	var notesHeaderFooters map[int]legacyPptHeaderFooterInfo
	if ctx, ok := extractPptPersistContext(f, stream); ok {
		exHyperlinks = parseLegacyPptExHyperlinkURLs(stream, ctx.documentOffset)
		theme = parseLegacyPptTheme(stream, ctx.documentOffset, colorScheme, fontTable)
		masterBgFill = parseLegacyPptMasterBgFill(stream, ctx.documentOffset, colorScheme)
		masterTextStyles = parseLegacyPptMasterTextStyles(stream, ctx.documentOffset, fontTable, colorScheme)
		masterStylesByID = parseLegacyPptMasterStylesByID(stream, ctx.documentOffset, ctx.persistDirectory, ctx.masterEntries, fontTable, colorScheme)
		slideHeaderFooterDefaults = parseLegacyPptDocumentHeaderFooterInfo(stream, ctx.documentOffset)
		notesHeaderFooterDefaults = parseLegacyPptDocumentNotesHeaderFooterInfo(stream, ctx.documentOffset)
		slideHeaderFooters = parseLegacyPptSlideHeaderFooterInfo(stream, ctx.persistDirectory, ctx.slidePersistIDs)
		notesHeaderFooters = mapLegacyPptPersistHeaderFooterInfos(ctx.notesPersistIDs, parseLegacyPptPersistHeaderFooterInfos(stream, ctx.persistDirectory, ctx.notesPersistIDs))
		slides = extractSlideTextsFromDocument(stream, ctx.documentOffset, ctx.notesListInstance, ctx.persistDirectory, fontTable, colorScheme, exHyperlinks)
		slideShapes = parseLegacyPptSlideTextShapes(stream, ctx.persistDirectory, ctx.slidePersistIDs, slides, fontTable, colorScheme, exHyperlinks)
		slideTableShapes := parseLegacyPptSlideTableShapes(stream, ctx.persistDirectory, ctx.slidePersistIDs, fontTable, colorScheme, exHyperlinks)
		slideShapes = mergeLegacyShapeMaps(slideShapes, slideTableShapes)
		externalObjects := parseLegacyPptExternalObjects(stream, ctx.documentOffset, ctx.persistDirectory)
		var pictureStore map[uint32]*legacyPptPicture
		picturesStream := decryptedPics
		if picturesStream == nil {
			if ps, ok := f.Stream("Pictures"); ok {
				picturesStream = ps
			}
		}
		if picturesStream != nil {
			pictureStore = parseLegacyPptPictureStore(stream, ctx.documentOffset, picturesStream)
			slideImageShapes := parseLegacyPptSlideImageShapes(stream, ctx.persistDirectory, ctx.slidePersistIDs, pictureStore, externalObjects)
			slideShapes = mergeLegacyShapeMaps(slideShapes, slideImageShapes)
		}
		slideConnectorShapes := parseLegacyPptSlideConnectorShapes(stream, ctx.persistDirectory, ctx.slidePersistIDs, colorScheme, slideShapes)
		slideShapes = mergeLegacyShapeMaps(slideShapes, slideConnectorShapes)
		slideGroupShapes := parseLegacyPptSlideGroupShapes(stream, ctx.persistDirectory, ctx.slidePersistIDs, slideShapes, fontTable, colorScheme, exHyperlinks)
		slideShapes = filterLegacyGroupedShapeDuplicates(slideShapes, slideGroupShapes)
		slideShapes = mergeLegacyShapeMaps(slideShapes, slideGroupShapes)
		slides = mergeLegacySlidesInPersistOrder(ctx.slidePersistIDs, slides, slideShapes)
	} else {
		slides = extractSlideTextsFallback(stream, fontTable, colorScheme, exHyperlinks)
	}
	slides = attachNotesHeaderFooterInfoByPersistID(slides, notesHeaderFooters, notesHeaderFooterDefaults)
	if len(slides) == 0 {
		text := fallbackPptText(stream)
		if text == "" {
			return nil, fmt.Errorf("legacy ppt: no readable text found")
		}
		slides = buildLegacyPptTextOnlySlides(text)
	}
	return buildPresentationWithExtraShapes(slides, slideShapes, slideHeaderFooters, slideHeaderFooterDefaults, filename, theme, masterStylesByID, masterTextStyles, masterBgFill), nil
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

func parseLegacyPptTheme(stream []byte, documentOffset int, colorScheme map[uint8]string, fontTable map[uint16]string) *presentationir.Theme {
	if len(colorScheme) == 0 {
		return nil
	}
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
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

func parseLegacyPptExHyperlinkURLs(stream []byte, documentOffset int) map[uint32]string {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return nil
	}
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

func parseLegacyPptExternalObjects(stream []byte, documentOffset int, persistDirectory map[int]int) map[uint32]legacyPptExternalObject {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return nil
	}
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
		r, err := zlib.NewReader(bytes.NewReader(payload[4:]))
		if err != nil {
			return nil
		}
		defer r.Close()
		var out bytes.Buffer
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
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
		}
		return out.Bytes()
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
func parseLegacyPptMasterStylesByID(stream []byte, documentOffset int, persistDirectory map[int]int, masterEntries []legacyPptMasterEntry, fontTable map[uint16]string, colorScheme map[uint8]string) map[int]legacyPptMasterTextStyles {
	if len(masterEntries) == 0 || len(persistDirectory) == 0 {
		return nil
	}
	// Build the Document-level defaults bundle (same as in parseLegacyPptMasterTextStyles).
	var docBundle legacyPptMasterTextBundle
	if docRecord, docPayload, _, ok := readPptRecord(stream, documentOffset); ok && docRecord.Type == pptRTDocument && docRecord.Version == 0x0F {
		for pos := 0; pos+8 <= len(docPayload); {
			child, childPayload, next, ok := readPptRecord(docPayload, pos)
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
	}
	result := make(map[int]legacyPptMasterTextStyles, len(masterEntries))
	for _, entry := range masterEntries {
		streamOff, ok := persistDirectory[entry.RefID]
		if !ok {
			continue
		}
		mRecord, mPayload, _, ok := readPptRecord(stream, streamOff)
		if !ok {
			continue
		}
		// Accept both the full MainMaster content record (0x03F8) and the
		// Document-embedded MainMasterContainer stub (0x07D0); both carry
		// TextMasterStyleAtom children.
		if mRecord.Type != pptRTMainMasterContent && mRecord.Type != pptRTMainMaster {
			continue
		}
		masterBundle := parseLegacyPptMainMasterTextBundle(mPayload, fontTable, colorScheme)
		styles := composeLegacyPptMasterTextStyles(docBundle, masterBundle)
		if len(styles) == 0 {
			styles = parseLegacyPptDirectMasterTextStyles(mPayload, fontTable, colorScheme)
		}
		if len(styles) > 0 {
			result[entry.SlideIdentifier] = styles
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
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

func parseLegacyPptTextMasterStyle10s(payload []byte, fontTable map[uint16]string) legacyPptMasterTextStyles {
	styles := legacyPptMasterTextStyles{}
	walkPptRecords(payload, func(record pptRecord, childPayload []byte) {
		if record.Type != pptRTTextMasterStyle10 {
			return
		}
		textType, levels, ok := parseLegacyPptTextMasterStyle10(record, childPayload, fontTable)
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

func mergeLegacyPptMasterParagraphStyle(primary, fallback stpaPFEntry) stpaPFEntry {
	if primary.Align == "" {
		primary.Align = fallback.Align
	}
	if primary.TextDirection == "" {
		primary.TextDirection = fallback.TextDirection
	}
	if !primary.HasBullet && primary.BulletChar == 0 && !primary.AutoNum {
		primary.HasBullet = fallback.HasBullet
		primary.BulletChar = fallback.BulletChar
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
	if !primary.AutoNum && primary.BulletChar == 0 && fallback.AutoNum {
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

func legacyPptMasterTextLevelEmpty(level legacyPptMasterTextLevel) bool {
	paragraph := level.paragraph
	run := level.run
	return paragraph.Align == "" && paragraph.TextDirection == "" && !paragraph.HasBullet && paragraph.BulletChar == 0 && !paragraph.AutoNum && paragraph.BulletColor == "" && paragraph.BulletSize == 0 && !paragraph.HasSpaceBefore && paragraph.SpaceBeforePct == 0 && !paragraph.HasSpaceAfter && paragraph.SpaceAfterPct == 0 && !paragraph.HasLineSpacing && !paragraph.HasMarginLeft && !paragraph.HasTextIndent && run.Bold == nil && run.Italic == nil && run.Underline == nil && run.Strike == nil && run.FontSize == 0 && run.FontFamily == "" && run.Color == ""
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

func extractSlideTexts(f *cfb.File, stream []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	if f != nil {
		if currentUser, ok := f.Stream("Current User"); ok {
			if slides := extractSlideTextsFromCurrentUser(currentUser, stream, fontTable, colorScheme, exHyperlinks); len(slides) > 0 {
				return slides
			}
		}
	}
	return extractSlideTextsFallback(stream, fontTable, colorScheme, exHyperlinks)
}

func extractSlideTextsFromCurrentUser(currentUser, stream []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacySlideText {
	currentEditOffset, ok := parseCurrentUserOffset(currentUser)
	if !ok {
		return nil
	}
	userEdits, ok := collectUserEdits(stream, currentEditOffset)
	if !ok || len(userEdits) == 0 {
		return nil
	}
	persistDirectory := buildPersistDirectory(stream, userEdits)
	if len(persistDirectory) == 0 {
		return nil
	}
	documentOffset, ok := persistDirectory[userEdits[0].DocPersistIDRef]
	if !ok {
		return nil
	}
	return extractSlideTextsFromDocument(stream, documentOffset, selectLegacyPptNotesSlideListInstance(stream, documentOffset, persistDirectory), persistDirectory, fontTable, colorScheme, exHyperlinks)
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
		shapes := parseLegacyPptTextShapesFromSlide(payload, notes[index], fontTable, colorScheme, exHyperlinks)
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
			// SSlideInfoAtom (MS-PPT §2.5.27) carries slide transition metadata.
			// Payload (12 bytes): slideTime int32, soundIdRef int32, effectDirection u8,
			// effectType u8, flags u16. fAutoAdvance = bit 5; fHidden = bit 1.
			if current >= 0 && len(payload) >= 12 {
				slideTime := int32(binary.LittleEndian.Uint32(payload[0:4]))
				effectType := payload[9]
				flags := binary.LittleEndian.Uint16(payload[10:12])
				autoAdvance := flags&(1<<5) != 0
				transition := &presentationir.SlideTransition{
					Type:     legacyPptTransitionType(effectType),
					AdvClick: flags&(1<<0) != 0 || flags == 0, // fManualAdvance — default true
				}
				if autoAdvance && slideTime > 0 {
					transition.AdvTmMs = int(slideTime)
				}
				slides[current].Transition = transition
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
		case unicode.IsPrint(r):
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

func buildPresentation(slides []legacySlideText, filename string) *presentationir.Presentation {
	return buildPresentationWithExtraShapes(slides, nil, nil, legacyPptHeaderFooterInfo{}, filename, nil, nil, nil, "")
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

func buildPresentationWithExtraShapes(slides []legacySlideText, extraShapes map[int][]*presentationir.Shape, slideHeaderFooters map[int]legacyPptHeaderFooterInfo, slideHeaderFooterDefaults legacyPptHeaderFooterInfo, filename string, theme *presentationir.Theme, masterStylesByID map[int]legacyPptMasterTextStyles, masterTextStyles legacyPptMasterTextStyles, masterBgFill string) *presentationir.Presentation {
	slides, slideHeaderFooters = normalizeLegacyPptHeaderFooterOnlySlides(slides, extraShapes, slideHeaderFooters)
	presentation := &presentationir.Presentation{
		SlideWidthEMU:  defaultSlideWidthEMU,
		SlideHeightEMU: defaultSlideHeightEMU,
		Slides:         make([]*presentationir.Slide, 0, len(slides)),
		Theme:          theme,
	}
	var masterBackgroundColor string
	if masterBgFill != "" {
		masterBackgroundColor = masterBgFill
	} else {
		masterBackgroundColor = legacyPptMasterBackgroundColor(theme)
	}
	for index, slideText := range slides {
		// Select the master text styles for this slide: use the master referenced
		// by SlideAtom.masterID when available (POI HSLFSlide.getMasterSheet()
		// parity); fall back to the Document-level stream-walk styles otherwise.
		slideStyles := masterTextStyles
		if slideText.MasterID != -1 {
			if s, ok := masterStylesByID[slideText.MasterID]; ok {
				slideStyles = s
			}
		}
		headerFooterInfo := slideHeaderFooterDefaults.mergedWith(slideHeaderFooters[slideText.PersistID])
		notesParagraphs := extractLegacyNotesParagraphs(slideText)
		hasNotesContent := len(notesParagraphs) > 0 || slideText.NotesHeaderFooter.hasRenderableContent()
		baseShapes := make([]*presentationir.Shape, 0, 6)
		anchoredTextShapes, nonTextShapes := splitLegacyExtraShapes(extraShapes[slideText.PersistID])
		if len(anchoredTextShapes) > 0 {
			baseShapes = append(baseShapes, cloneLegacyExtraShapes(index, len(baseShapes)+1, anchoredTextShapes)...)
		} else if len(slideText.Fragments) > 0 {
			baseShapes = append(baseShapes, buildLegacySlideShapes(index, slideText, filename)...)
		}
		if extras := cloneLegacyExtraShapes(index, len(baseShapes)+1, nonTextShapes); len(extras) > 0 {
			baseShapes = append(baseShapes, extras...)
		}
		if len(baseShapes) == 0 {
			if !headerFooterInfo.hasRenderableContent() && !hasNotesContent && legacyPptSlideHasOnlyPlaceholderPrompts(slideText) {
				presentation.Slides = append(presentation.Slides, &presentationir.Slide{
					Index:      index,
					MasterID:   slideText.MasterID,
					LayoutType: slideText.LayoutType,
					BgColor:    masterBackgroundColor,
					Transition: slideText.Transition,
				})
			}
			if !headerFooterInfo.hasRenderableContent() && !hasNotesContent {
				continue
			}
		}
		header, shapes, footer, dateTime, slideNumber := splitLegacySlideShapes(baseShapes)
		header, footer, dateTime, slideNumber = synthesizeLegacyHeaderFooterShapes(index, headerFooterInfo, header, footer, dateTime, slideNumber)
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
		for _, shape := range shapes {
			applyLegacyPptMasterTextStylesToShape(shape, slideStyles, legacyPptMasterTextTypeForShape(shape))
		}
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
		presentation.Slides = append(presentation.Slides, &presentationir.Slide{
			Index:             index,
			MasterID:          slideText.MasterID,
			LayoutType:        slideText.LayoutType,
			BgColor:           masterBackgroundColor,
			Shapes:            shapes,
			Header:            header,
			Footer:            footer,
			DateTime:          dateTime,
			SlideNumber:       slideNumber,
			Notes:             notes,
			Transition:        slideText.Transition,
			AnimationTimeline: buildLegacySlideAnimationTimeline(header, shapes, footer, dateTime, slideNumber),
		})
	}
	return presentation
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
		hasContent := len(slide.Fragments) > 0 || len(extraShapes[persistID]) > 0
		info := normalized[persistID]
		if !hasContent && info.hasRenderableContent() && len(out) > 0 {
			lastPersistID := out[len(out)-1].PersistID
			normalized[lastPersistID] = normalized[lastPersistID].mergedWith(info)
			delete(normalized, persistID)
			continue
		}
		out = append(out, slide)
	}
	return out, normalized
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
	case presentationir.PlaceholderFooter, presentationir.PlaceholderDateTime, presentationir.PlaceholderSlideNumber:
		return pptTextTypeOther
	default:
		return pptTextTypeUnknown
	}
}

func parseLegacyPptDocumentHeaderFooterInfo(stream []byte, documentOffset int) legacyPptHeaderFooterInfo {
	return parseLegacyPptDocumentHeaderFooterInfoByInstance(stream, documentOffset, 3)
}

func parseLegacyPptDocumentNotesHeaderFooterInfo(stream []byte, documentOffset int) legacyPptHeaderFooterInfo {
	return parseLegacyPptDocumentHeaderFooterInfoByInstance(stream, documentOffset, 4)
}

func parseLegacyPptDocumentHeaderFooterInfoByInstance(stream []byte, documentOffset int, instance uint16) legacyPptHeaderFooterInfo {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return legacyPptHeaderFooterInfo{}
	}
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

func parseLegacyPptSlideHeaderFooterInfo(stream []byte, persistDirectory map[int]int, slidePersistIDs []int) map[int]legacyPptHeaderFooterInfo {
	if len(slidePersistIDs) == 0 || len(persistDirectory) == 0 {
		return nil
	}
	infos := parseLegacyPptPersistHeaderFooterInfos(stream, persistDirectory, slidePersistIDs)
	out := map[int]legacyPptHeaderFooterInfo{}
	for index, persistID := range slidePersistIDs {
		if index >= len(infos) {
			continue
		}
		info := infos[index]
		if info.hasRenderableContent() {
			out[persistID] = info
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptPersistHeaderFooterInfos(stream []byte, persistDirectory map[int]int, persistIDs []int) []legacyPptHeaderFooterInfo {
	if len(persistIDs) == 0 || len(persistDirectory) == 0 {
		return nil
	}
	infos := make([]legacyPptHeaderFooterInfo, len(persistIDs))
	hasAny := false
	for index, persistID := range persistIDs {
		persistOffset, ok := persistDirectory[persistID]
		if !ok {
			continue
		}
		_, payload, _, ok := readPptRecord(stream, persistOffset)
		if !ok {
			continue
		}
		info := parseLegacyPptHeaderFooterInfoFromPersist(payload)
		if info.hasRenderableContent() {
			infos[index] = info
			hasAny = true
		}
	}
	if !hasAny {
		return nil
	}
	return infos
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
		if info := infos[index]; info.hasRenderableContent() {
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

func synthesizeLegacyHeaderFooterShapes(slideIndex int, info legacyPptHeaderFooterInfo, header, footer, dateTime, slideNumber *presentationir.Shape) (*presentationir.Shape, *presentationir.Shape, *presentationir.Shape, *presentationir.Shape) {
	nextShapeID := 89
	if header == nil && info.headerEnabled() && info.headerText != "" {
		header = newLegacySlideShape(slideIndex, nextShapeID, "hdr", presentationir.PlaceholderOther, legacyHeaderBounds(), buildTextParagraphs(info.headerText))
		nextShapeID++
	}
	if footer == nil && info.footerEnabled() && info.footerText != "" {
		footer = newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Footer %d", slideIndex+1), presentationir.PlaceholderFooter, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderFooter), buildTextParagraphs(info.footerText))
		nextShapeID++
	}
	if dateTime == nil && info.dateEnabled() {
		if info.dateText != "" {
			dateTime = newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Date %d", slideIndex+1), presentationir.PlaceholderDateTime, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime), buildTextParagraphs(info.dateText))
		} else if info.flags&legacyPptHFFlagTodayDate != 0 {
			dateTime = newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Date %d", slideIndex+1), presentationir.PlaceholderDateTime, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderDateTime), []*presentationir.Paragraph{{Runs: []*presentationir.Run{{FieldKind: "date"}}}})
		}
		if dateTime != nil {
			nextShapeID++
		}
	}
	if slideNumber == nil && info.slideNumberEnabled() {
		slideNumber = newLegacySlideShape(slideIndex, nextShapeID, fmt.Sprintf("Legacy Slide Number %d", slideIndex+1), presentationir.PlaceholderSlideNumber, legacyFooterBoundsForPlaceholder(presentationir.PlaceholderSlideNumber), []*presentationir.Paragraph{{Runs: []*presentationir.Run{{FieldKind: "slidenum"}}}})
	}
	return header, footer, dateTime, slideNumber
}

func applyLegacyPptMasterTextStylesToShape(shape *presentationir.Shape, styles legacyPptMasterTextStyles, textType int) {
	if shape == nil {
		return
	}
	if shape.Kind == presentationir.ShapeGroup {
		for _, child := range shape.Children {
			applyLegacyPptMasterTextStylesToShape(child, styles, legacyPptMasterTextTypeForShape(child))
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
		inheritedOnly := paragraphLooksUnstyled(paragraph)
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
			return legacyPptMasterTextLevelsWithBuiltInFallback(textType, levels)
		}
	}
	switch textType {
	case pptTextTypeTitle:
		if levels, ok := styles[pptTextTypeCenterTitle]; ok && legacyPptMasterTextLevelsUsable(levels) {
			return legacyPptMasterTextLevelsWithBuiltInFallback(textType, levels)
		}
	case pptTextTypeCenterBody:
		if levels, ok := styles[pptTextTypeBody]; ok && legacyPptMasterTextLevelsUsable(levels) {
			return levels
		}
	case pptTextTypeCenterTitle:
		if levels, ok := styles[pptTextTypeTitle]; ok && legacyPptMasterTextLevelsUsable(levels) {
			return legacyPptMasterTextLevelsWithBuiltInFallback(textType, levels)
		}
	case pptTextTypeUnknown:
		// POI HSLFSlideMaster.getPropCollection maps null txtype → BODY fallback.
		// Shapes without an explicit TextHeaderAtom (textType = -1) should inherit
		// body master defaults rather than receiving no styling at all.
		if levels, ok := styles[pptTextTypeBody]; ok && legacyPptMasterTextLevelsUsable(levels) {
			return levels
		}
	}
	return legacyPptMasterTextLevelsWithBuiltInFallback(textType, primary)
}

func legacyPptMasterTextLevelsUsable(levels map[int]legacyPptMasterTextLevel) bool {
	if len(levels) == 0 {
		return false
	}
	for _, level := range levels {
		if !legacyPptMasterTextLevelEmpty(level) {
			return true
		}
	}
	return false
}

func legacyPptMasterTextLevelsHaveRunStyling(levels map[int]legacyPptMasterTextLevel) bool {
	for _, level := range levels {
		run := level.run
		if run.Bold != nil || run.Italic != nil || run.Underline != nil || run.Strike != nil || run.FontSize > 0 || run.FontFamily != "" || run.Color != "" {
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

func legacyPptBuiltInMasterTextLevel(textType int) (legacyPptMasterTextLevel, bool) {
	switch textType {
	case pptTextTypeTitle, pptTextTypeCenterTitle:
		bold := true
		return legacyPptMasterTextLevel{
			paragraph: stpaPFEntry{Align: "ctr"},
			run:       stpaCFRun{Bold: &bold, FontSize: 4400},
		}, true
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
	if paragraph.Align != "" || paragraph.TextDirection != "" || paragraph.BulletChar != "" || paragraph.AutoNum || paragraph.BulletColor != "" || paragraph.BulletSize > 0 || paragraph.HasSpaceBefore || paragraph.HasSpaceAfter || paragraph.HasLineSpacing || paragraph.HasMarginLeft || paragraph.HasTextIndent {
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
	if merged.BulletChar == "" {
		merged.BulletChar = bulletCharString(master.paragraph.BulletChar)
	}
	if !merged.AutoNum && merged.BulletChar == "" && master.paragraph.AutoNum {
		merged.AutoNum = true
		merged.AutoNumType = master.paragraph.AutoNumType
		merged.AutoNumStart = master.paragraph.AutoNumStart
	}
	if merged.BulletColor == "" {
		merged.BulletColor = master.paragraph.BulletColor
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
		merged.HasLineSpacing = true
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
		clone.Paragraphs = append([]*presentationir.Paragraph(nil), shape.Paragraphs...)
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
	lines := strings.Split(normalizePptText(text), "\n")
	paragraphs := make([]*presentationir.Paragraph, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
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

func scoreSlides(slides []legacySlideText) int {
	score := 0
	for _, slide := range slides {
		for _, fragment := range slide.Fragments {
			score += len(fragment.Text)
		}
	}
	return score
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

func mergeLegacySlidesInPersistOrder(order []int, slides []legacySlideText, extraShapes map[int][]*presentationir.Shape) []legacySlideText {
	if len(order) == 0 {
		return slides
	}
	byPersistID := make(map[int]legacySlideText, len(slides))
	for _, slide := range slides {
		if slide.PersistID != 0 {
			byPersistID[slide.PersistID] = slide
		}
	}
	seen := map[int]struct{}{}
	out := make([]legacySlideText, 0, len(order))
	for _, persistID := range order {
		slide, ok := byPersistID[persistID]
		if !ok {
			if len(extraShapes[persistID]) == 0 {
				continue
			}
			slide = legacySlideText{PersistID: persistID}
		}
		out = append(out, slide)
		seen[persistID] = struct{}{}
	}
	for _, slide := range slides {
		if slide.PersistID != 0 {
			if _, ok := seen[slide.PersistID]; ok {
				continue
			}
		}
		out = append(out, slide)
	}
	return out
}

func mergeLegacyShapeMaps(dst, src map[int][]*presentationir.Shape) map[int][]*presentationir.Shape {
	if len(src) == 0 {
		return dst
	}
	if len(dst) == 0 {
		out := make(map[int][]*presentationir.Shape, len(src))
		for persistID, shapes := range src {
			out[persistID] = append([]*presentationir.Shape(nil), shapes...)
		}
		return out
	}
	for persistID, shapes := range src {
		dst[persistID] = append(dst[persistID], shapes...)
	}
	return dst
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

func parseLegacyPptSlideGroupShapes(stream []byte, persistDirectory map[int]int, slidePersistIDs []int, resolved map[int][]*presentationir.Shape, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) map[int][]*presentationir.Shape {
	if len(slidePersistIDs) == 0 || len(persistDirectory) == 0 || len(resolved) == 0 {
		return nil
	}
	out := map[int][]*presentationir.Shape{}
	for _, persistID := range slidePersistIDs {
		slideOffset, ok := persistDirectory[persistID]
		if !ok {
			continue
		}
		record, payload, _, ok := readPptRecord(stream, slideOffset)
		if !ok || record.Type != pptRTSlide {
			continue
		}
		byID := legacyPptShapeIndexByOriginalID(resolved[persistID])
		if len(byID) == 0 {
			continue
		}
		groups := parseLegacyPptGroupShapesFromSlide(payload, byID, fontTable, colorScheme, exHyperlinks)
		if len(groups) > 0 {
			out[persistID] = groups
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

func parseLegacyPptGroupShapesFromSlide(payload []byte, resolvedByID map[uint32]*presentationir.Shape, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []*presentationir.Shape {
	if len(payload) == 0 || len(resolvedByID) == 0 {
		return nil
	}
	var groups []*presentationir.Shape
	ordinal := 1
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTPPDrawing {
			collectLegacyPptGroupShapesFromOfficeArt(childPayload, legacyPptGroupFrame{}, resolvedByID, fontTable, colorScheme, exHyperlinks, &groups, &ordinal)
		}
		pos = next
	}
	if len(groups) == 0 {
		return nil
	}
	return groups
}

func collectLegacyPptGroupShapesFromOfficeArt(data []byte, frame legacyPptGroupFrame, resolvedByID map[uint32]*presentationir.Shape, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, groups *[]*presentationir.Shape, ordinal *int) {
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		if recType == legacyOfficeArtSpgrContainer {
			if groupShape, ok := parseLegacyPptGroupShapeFromContainer(body, frame, resolvedByID, fontTable, colorScheme, exHyperlinks, *ordinal); ok {
				*groups = append(*groups, groupShape)
				*ordinal = *ordinal + 1
			}
		}
		off += 8 + recLen
	}
}

func parseLegacyPptGroupShapeFromContainer(data []byte, parent legacyPptGroupFrame, resolvedByID map[uint32]*presentationir.Shape, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, ordinal int) (*presentationir.Shape, bool) {
	if legacyPptGroupIsTableLike(data, parent, fontTable, colorScheme, exHyperlinks) {
		return nil, false
	}
	meta, frame, ok := parseLegacyPptGroupShapeMetaFromSpgrContainer(data, parent, exHyperlinks)
	if !ok {
		return nil, false
	}
	children := collectLegacyPptGroupChildren(data, frame, resolvedByID, fontTable, colorScheme, exHyperlinks)
	if len(children) == 0 {
		return nil, false
	}
	for _, child := range children {
		applyLegacyPptGroupTransformToShape(child, meta.bounds, meta.transform)
	}
	group := &presentationir.Shape{
		Kind:            presentationir.ShapeGroup,
		Name:            fmt.Sprintf("Legacy Group %d", ordinal),
		OriginalShapeID: meta.shapeID,
		Bounds:          meta.bounds,
		Children:        children,
		HyperlinkURL:    meta.hyperlinkURL,
		ActionLink:      meta.actionLink,
	}
	applyLegacyPptShapeTransform(group, meta.transform)
	applyLegacyPptAnimationMetadata(group, meta.animation)
	return group, true
}

func collectLegacyPptGroupChildren(data []byte, frame legacyPptGroupFrame, resolvedByID map[uint32]*presentationir.Shape, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []*presentationir.Shape {
	var children []*presentationir.Shape
	skippedRoot := false
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		switch recType {
		case legacyOfficeArtSpgrContainer:
			if legacyPptGroupIsTableLike(body, frame, fontTable, colorScheme, exHyperlinks) {
				if tableShape, ok := parseLegacyPptTableShapeFromGroupContainer(body, frame, fontTable, colorScheme, exHyperlinks, len(children)+1); ok {
					children = append(children, tableShape)
				}
			} else if groupShape, ok := parseLegacyPptGroupShapeFromContainer(body, frame, resolvedByID, fontTable, colorScheme, exHyperlinks, len(children)+1); ok {
				children = append(children, groupShape)
			}
		case legacyOfficeArtSpContainer:
			if !skippedRoot && legacyPptSpContainerHasGroupRect(body) {
				skippedRoot = true
				off += 8 + recLen
				continue
			}
			if shapeID, ok := legacyPptSpContainerShapeID(body); ok {
				if resolved, ok := resolvedByID[shapeID]; ok {
					if clone := cloneLegacyShapeRecursive(resolved, nil); clone != nil {
						children = append(children, clone)
					}
				}
			}
		}
		off += 8 + recLen
	}
	return children
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
	meta := legacyPptGroupShapeMeta{bounds: frame.target}
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
		case legacyOfficeArtFOPT:
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

func parseLegacyPptSlideTextShapes(stream []byte, persistDirectory map[int]int, slidePersistIDs []int, slides []legacySlideText, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) map[int][]*presentationir.Shape {
	if len(slidePersistIDs) == 0 || len(persistDirectory) == 0 {
		return nil
	}
	byPersistID := make(map[int]legacySlideText, len(slides))
	for _, slide := range slides {
		if slide.PersistID != 0 {
			byPersistID[slide.PersistID] = slide
		}
	}
	out := map[int][]*presentationir.Shape{}
	for _, persistID := range slidePersistIDs {
		slideOffset, ok := persistDirectory[persistID]
		if !ok {
			continue
		}
		record, payload, _, ok := readPptRecord(stream, slideOffset)
		if !ok || record.Type != pptRTSlide {
			continue
		}
		shapes := parseLegacyPptTextShapesFromSlide(payload, byPersistID[persistID], fontTable, colorScheme, exHyperlinks)
		if len(shapes) > 0 {
			out[persistID] = shapes
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptTextShapesFromSlide(payload []byte, slideText legacySlideText, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []*presentationir.Shape {
	if len(payload) == 0 {
		return nil
	}
	metas := parseLegacyPptTextShapeContainers(payload, fontTable, colorScheme, exHyperlinks)
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
		placeholder := meta.placeholder
		if placeholder == presentationir.PlaceholderNone {
			placeholder = legacyPptPlaceholderFromTextType(meta.textType, hasTitleContext)
		}
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
			if len(paragraphs) == 0 {
				paragraphs = consumeLegacyPptPlaceholderParagraphs(slideText, usedFragments, placeholder, hasTitleContext)
			}
			if len(paragraphs) == 0 && meta.textType != pptTextTypeUnknown {
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
		shape := &presentationir.Shape{
			Kind:            presentationir.ShapeText,
			OriginalShapeID: meta.shapeID,
			Name:            legacyPptTextShapeName(index+1, placeholder),
			Placeholder:     placeholder,
			Bounds:          meta.bounds,
			FillColor:       meta.fillColor,
			Paragraphs:      paragraphs,
			HyperlinkURL:    meta.hyperlinkURL,
			ActionLink:      meta.actionLink,
		}
		if meta.fillColor != "" {
			shape.Fill = &drawing.SolidFill{Color: meta.fillColor, Alpha: 1.0}
		}
		applyLegacyPptShapeTransform(shape, meta.transform)
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

func parseLegacyPptTextShapeContainers(payload []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []legacyPptTextShape {
	var shapes []legacyPptTextShape
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTPPDrawing {
			collectLegacyPptTextShapesFromOfficeArt(childPayload, legacyPptGroupFrame{}, fontTable, colorScheme, exHyperlinks, &shapes)
		}
		pos = next
	}
	return shapes
}

func collectLegacyPptTextShapesFromOfficeArt(data []byte, frame legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, shapes *[]legacyPptTextShape) {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		version := verInst & 0x000F
		body := data[off+8 : off+8+recLen]
		switch recType {
		case legacyOfficeArtSpgrContainer:
			if legacyPptGroupIsTableLike(body, frame, fontTable, colorScheme, exHyperlinks) {
				off += 8 + recLen
				continue
			}
			nextFrame := frame
			if groupFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(body, frame); ok {
				nextFrame = groupFrame
			}
			collectLegacyPptTextShapesFromOfficeArt(body, nextFrame, fontTable, colorScheme, exHyperlinks, shapes)
		case legacyOfficeArtSpContainer:
			shape, ok := parseLegacyPptTextShapeContainer(body, frame, fontTable, colorScheme, exHyperlinks)
			if ok {
				*shapes = append(*shapes, shape)
			}
		default:
			if version == 0x000F {
				collectLegacyPptTextShapesFromOfficeArt(body, frame, fontTable, colorScheme, exHyperlinks, shapes)
			}
		}
		off += 8 + recLen
	}
}

func parseLegacyPptTextShapeContainer(data []byte, frame legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) (legacyPptTextShape, bool) {
	var shape legacyPptTextShape
	shape.outlineRef = -1
	shape.textType = pptTextTypeUnknown

	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil || node == nil {
		return legacyPptTextShape{}, false
	}
	shape.shapeID = node.ShapeID
	shape.transform.FlipH = node.Flags&legacyOfficeArtSpFlagFlipH != 0
	shape.transform.FlipV = node.Flags&legacyOfficeArtSpFlagFlipV != 0

	var clientAnchorBounds, childAnchorBounds presentationir.Rect
	haveClientAnchor, haveChildAnchor := false, false
	if len(node.ClientAnchor) > 0 {
		if bounds, ok := parseLegacyPptClientAnchor(node.ClientAnchor); ok {
			clientAnchorBounds = bounds
			haveClientAnchor = true
		}
	}
	if len(node.ChildAnchor) > 0 {
		if localBounds, ok := parseLegacyPptChildAnchor(node.ChildAnchor); ok {
			if mappedBounds, ok := mapLegacyPptRectThroughGroupFrame(localBounds, frame); ok {
				childAnchorBounds = mappedBounds
				haveChildAnchor = true
			}
		}
	}

	if len(node.ClientData) > 0 {
		if placeholder, ok := parseLegacyPptPlaceholderClientData(node.ClientData); ok {
			shape.placeholder = placeholder
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
		if color, ok := parseLegacyPptFOPTPropertiesFillColor(node.Properties, colorScheme); ok {
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
		}
		if inlineFrags, _ := parseLegacyPptTextBoxFragmentsRecursive(node.ClientTextbox, fontTable, colorScheme, exHyperlinks); len(inlineFrags) > 0 {
			if shape.textType == pptTextTypeUnknown {
				shape.textType = dominantLegacyPptTextType(inlineFrags)
			}
			shape.inlineFallback = legacyPptFragmentsParagraphs(inlineFrags)
		}
	}

	haveAnchor := false
	if node.Flags&legacyOfficeArtSpFlagChild != 0 && haveChildAnchor {
		shape.bounds = childAnchorBounds
		haveAnchor = true
	} else if haveClientAnchor {
		shape.bounds = clientAnchorBounds
		haveAnchor = true
	} else if haveChildAnchor {
		shape.bounds = childAnchorBounds
		haveAnchor = true
	}
	if !haveAnchor || (!sawTextbox && len(shape.paragraphs) == 0 && len(shape.inlineFallback) == 0) {
		return legacyPptTextShape{}, false
	}

	return shape, true
}

func parseLegacyPptPlaceholderClientData(data []byte) (presentationir.PlaceholderKind, bool) {
	for pos := 0; pos+8 <= len(data); {
		record, payload, next, ok := readPptRecord(data, pos)
		if !ok {
			break
		}
		if record.Type == pptRTPlaceholderAtom {
			if placeholder, ok := parseLegacyPptPlaceholderAtom(payload); ok {
				return placeholder, true
			}
		}
		pos = next
	}
	return presentationir.PlaceholderNone, false
}

func parseLegacyPptPlaceholderAtom(payload []byte) (presentationir.PlaceholderKind, bool) {
	if len(payload) < 4 {
		return presentationir.PlaceholderNone, false
	}
	// placementId (bytes 0–3) is a per-slide shape-sequence index.
	// 0xFFFFFFFF is the sentinel meaning "not a placeholder shape".
	if binary.LittleEndian.Uint32(payload[:4]) == 0xFFFFFFFF {
		return presentationir.PlaceholderNone, false
	}
	if len(payload) < 5 {
		return presentationir.PlaceholderNone, false
	}
	// placeholderId (byte 4) encodes the placeholder type per MS-PPT
	// OEPlaceholderAtom PlaceholderEnum (spec section 2.9.29).
	switch payload[4] {
	case 0x0D: // PT_Title
		return presentationir.PlaceholderTitle, true
	case 0x0E: // PT_Body
		return presentationir.PlaceholderBody, true
	case 0x0F: // PT_CenterTitle
		return presentationir.PlaceholderTitle, true
	case 0x10: // PT_SubTitle
		return presentationir.PlaceholderSubtitle, true
	case 0x11: // PT_VerticalTitle
		return presentationir.PlaceholderTitle, true
	case 0x12: // PT_VerticalBody
		return presentationir.PlaceholderBody, true
	case 0x07: // PT_MasterDate
		return presentationir.PlaceholderDateTime, true
	case 0x08: // PT_MasterSlideNumber
		return presentationir.PlaceholderSlideNumber, true
	case 0x09: // PT_MasterFooter
		return presentationir.PlaceholderFooter, true
	case 0x0C: // PT_NotesBody
		return presentationir.PlaceholderBody, true
	default:
		return presentationir.PlaceholderNone, false
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
	case pptTextTypeBody, pptTextTypeOther, pptTextTypeUnknown:
		return presentationir.PlaceholderBody
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

func parseLegacyPptPictureStore(stream []byte, documentOffset int, picturesStream []byte) map[uint32]*legacyPptPicture {
	if len(picturesStream) == 0 {
		return nil
	}
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
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

func parseLegacyPptBSEPicture(body []byte, picturesStream []byte) *legacyPptPicture {
	resolver := func(offset, size uint32) []byte {
		start := uint64(offset)
		end := start + uint64(size)
		if end > uint64(len(picturesStream)) || start >= end {
			return nil
		}
		return picturesStream[int(start):int(end)]
	}
	entry, ok := officeartlegacy.ParseBSE(body, resolver)
	if !ok || len(entry.Media.Data) == 0 || entry.Media.MIME == "" {
		return nil
	}
	return &legacyPptPicture{
		Data: append([]byte(nil), entry.Media.Data...),
		MIME: entry.Media.MIME,
	}
}

func extractLegacyPptPictureBlob(data []byte) ([]byte, string) {
	media := officeartlegacy.ExtractBLIPStream(data)
	return media.Data, media.MIME
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

func extractLegacyPptOfficeArtBlipRaster(data []byte) []byte {
	blob, _ := officeartlegacy.ExtractRaster(data)
	return blob
}

func extractLegacyPptOfficeArtBlipVector(payload []byte, wantMIME string) []byte {
	var recType uint16
	switch wantMIME {
	case "image/x-emf":
		recType = legacyOfficeArtBlipEMF
	case "image/x-wmf":
		recType = legacyOfficeArtBlipWMF
	default:
		return nil
	}
	media := officeartlegacy.ExtractBLIP(recType, payload)
	if media.MIME != wantMIME {
		return nil
	}
	return media.Data
}

func legacyPptExtractVectorBlob(data []byte, wantMIME string) []byte {
	start, ok := legacyPptVectorImageStart(data, wantMIME)
	if !ok || start < 0 || start >= len(data) {
		return nil
	}
	return data[start:]
}

func legacyPptVectorImageStart(data []byte, wantMIME string) (int, bool) {
	switch wantMIME {
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
		if legacyPptLooksLikeWMF(data) {
			return 0, true
		}
		if index := bytes.Index(data, []byte{0xD7, 0xCD, 0xC6, 0x9A}); index >= 0 {
			return index, true
		}
		for index := 0; index+18 <= len(data); index++ {
			if legacyPptLooksLikeWMF(data[index:]) {
				return index, true
			}
		}
	}
	return 0, false
}

func legacyPptLooksLikeWMF(data []byte) bool {
	if len(data) < 18 {
		return false
	}
	typeVal := binary.LittleEndian.Uint16(data[0:2])
	headerSize := binary.LittleEndian.Uint16(data[2:4])
	version := binary.LittleEndian.Uint16(data[4:6])
	return (typeVal == 1 || typeVal == 2) && headerSize == 9 && (version == 0x0100 || version == 0x0300)
}

func legacyPptInflate(data []byte) []byte {
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

func parseLegacyPptSlideImageShapes(stream []byte, persistDirectory map[int]int, slidePersistIDs []int, pictureStore map[uint32]*legacyPptPicture, externalObjects map[uint32]legacyPptExternalObject) map[int][]*presentationir.Shape {
	if len(slidePersistIDs) == 0 || len(persistDirectory) == 0 || len(pictureStore) == 0 {
		return nil
	}
	out := map[int][]*presentationir.Shape{}
	for _, persistID := range slidePersistIDs {
		slideOffset, ok := persistDirectory[persistID]
		if !ok {
			continue
		}
		record, payload, _, ok := readPptRecord(stream, slideOffset)
		if !ok || record.Type != pptRTSlide {
			continue
		}
		shapes := parseLegacyPptImageShapesFromSlide(payload, pictureStore, externalObjects)
		if len(shapes) > 0 {
			out[persistID] = shapes
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptSlideTableShapes(stream []byte, persistDirectory map[int]int, slidePersistIDs []int, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) map[int][]*presentationir.Shape {
	if len(slidePersistIDs) == 0 || len(persistDirectory) == 0 {
		return nil
	}
	out := map[int][]*presentationir.Shape{}
	for _, persistID := range slidePersistIDs {
		slideOffset, ok := persistDirectory[persistID]
		if !ok {
			continue
		}
		record, payload, _, ok := readPptRecord(stream, slideOffset)
		if !ok || record.Type != pptRTSlide {
			continue
		}
		shapes := parseLegacyPptTableShapesFromSlide(payload, fontTable, colorScheme, exHyperlinks)
		if len(shapes) > 0 {
			out[persistID] = shapes
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptTableShapesFromSlide(payload []byte, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) []*presentationir.Shape {
	if len(payload) == 0 {
		return nil
	}
	var shapes []*presentationir.Shape
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTPPDrawing {
			collectLegacyPptTableShapesFromOfficeArt(childPayload, legacyPptGroupFrame{}, fontTable, colorScheme, exHyperlinks, &shapes)
		}
		pos = next
	}
	if len(shapes) == 0 {
		return nil
	}
	return shapes
}

func collectLegacyPptTableShapesFromOfficeArt(data []byte, frame legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, shapes *[]*presentationir.Shape) {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		version := verInst & 0x000F
		body := data[off+8 : off+8+recLen]
		switch recType {
		case legacyOfficeArtSpgrContainer:
			if tableShape, ok := parseLegacyPptTableShapeFromGroupContainer(body, frame, fontTable, colorScheme, exHyperlinks, len(*shapes)+1); ok {
				*shapes = append(*shapes, tableShape)
				off += 8 + recLen
				continue
			}
			nextFrame := frame
			if groupFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(body, frame); ok {
				nextFrame = groupFrame
			}
			collectLegacyPptTableShapesFromOfficeArt(body, nextFrame, fontTable, colorScheme, exHyperlinks, shapes)
		default:
			if version == 0x000F {
				collectLegacyPptTableShapesFromOfficeArt(body, frame, fontTable, colorScheme, exHyperlinks, shapes)
			}
		}
		off += 8 + recLen
	}
}

func parseLegacyPptTableShapeFromGroupContainer(data []byte, parent legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, ordinal int) (*presentationir.Shape, bool) {
	tableFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(data, parent)
	if !ok {
		return nil, false
	}
	groupShapeID, _ := legacyPptGroupRootShapeID(data)
	var cells []legacyPptTableCellShape
	lineCount := 0
	collectLegacyPptTableCellsFromGroup(data, tableFrame, fontTable, colorScheme, exHyperlinks, &cells, &lineCount)
	if len(cells) == 0 {
		return nil, false
	}
	if !legacyPptGroupContainerIsTable(data) && !legacyPptLooksLikeTableGroup(cells, lineCount) {
		return nil, false
	}
	table, bounds, ok := buildLegacyPptTableFromCells(cells, tableFrame.target)
	if !ok || table == nil || len(table.Rows) == 0 {
		return nil, false
	}
	return &presentationir.Shape{
		Kind:            presentationir.ShapeTable,
		Name:            fmt.Sprintf("Legacy Table %d", ordinal),
		OriginalShapeID: groupShapeID,
		Bounds:          bounds,
		Table:           table,
	}, true
}

func legacyPptGroupIsTableLike(data []byte, parent legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string) bool {
	if legacyPptGroupContainerIsTable(data) {
		return true
	}
	frame, ok := parseLegacyPptGroupFrameFromSpgrContainer(data, parent)
	if !ok {
		return false
	}
	var cells []legacyPptTableCellShape
	lineCount := 0
	collectLegacyPptTableCellsFromGroup(data, frame, fontTable, colorScheme, exHyperlinks, &cells, &lineCount)
	return legacyPptLooksLikeTableGroup(cells, lineCount)
}

func legacyPptGroupContainerIsTable(data []byte) bool {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		if recType == legacyOfficeArtSpContainer && legacyPptSpContainerIsTable(body) {
			return true
		}
		if verInst&0x000F == 0x000F && legacyPptGroupContainerIsTable(body) {
			return true
		}
		off += 8 + recLen
	}
	return false
}

func legacyPptSpContainerIsTable(data []byte) bool {
	var sawGroupRect bool
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		body := data[off+8 : off+8+recLen]
		switch recType {
		case legacyOfficeArtSpgr:
			sawGroupRect = true
		case legacyOfficeArtFOPT:
			if sawGroupRect && legacyPptFOPTMarksTable(body, int(verInst>>4)) {
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
		value := binary.LittleEndian.Uint32(body[offset+2 : offset+6])
		return value&1 == 1
	}
	return false
}

func collectLegacyPptTableCellsFromGroup(data []byte, frame legacyPptGroupFrame, fontTable map[uint16]string, colorScheme map[uint8]string, exHyperlinks map[uint32]string, cells *[]legacyPptTableCellShape, lineCount *int) {
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
			collectLegacyPptTableCellsFromGroup(body, nextFrame, fontTable, colorScheme, exHyperlinks, cells, lineCount)
		case legacyOfficeArtSpContainer:
			if legacyPptSpContainerHasGroupRect(body) {
				off += 8 + recLen
				continue
			}
			if shapeType, ok := legacyPptSpContainerShapeType(body); ok && shapeType == 20 {
				if lineCount != nil {
					*lineCount++
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

func legacyPptLooksLikeTableGroup(cells []legacyPptTableCellShape, lineCount int) bool {
	if len(cells) < 2 {
		return false
	}
	colPos := sortedLegacyPptTablePositions(cells, func(cell legacyPptTableCellShape) int64 { return cell.bounds.X })
	rowPos := sortedLegacyPptTablePositions(cells, func(cell legacyPptTableCellShape) int64 { return cell.bounds.Y })
	if len(colPos) < 2 && len(rowPos) < 2 {
		return false
	}
	if lineCount > 0 {
		return true
	}
	return len(cells) >= 6 && len(colPos) >= 2 && len(rowPos) >= 2
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
	colPos := sortedLegacyPptTablePositions(cells, func(cell legacyPptTableCellShape) int64 { return cell.bounds.X })
	rowPos := sortedLegacyPptTablePositions(cells, func(cell legacyPptTableCellShape) int64 { return cell.bounds.Y })
	if len(colPos) == 0 || len(rowPos) == 0 {
		return nil, presentationir.Rect{}, false
	}
	colIndex := make(map[int64]int, len(colPos))
	for i, pos := range colPos {
		colIndex[pos] = i
	}
	rowIndex := make(map[int64]int, len(rowPos))
	for i, pos := range rowPos {
		rowIndex[pos] = i
	}
	grid := make([][]*presentationir.TableCell, len(rowPos))
	for r := range grid {
		grid[r] = make([]*presentationir.TableCell, len(colPos))
	}
	rowHeights := make([]int64, len(rowPos))
	colWidths := make([]int64, len(colPos))
	for _, cell := range cells {
		r, okRow := rowIndex[cell.bounds.Y]
		c, okCol := colIndex[cell.bounds.X]
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

func sortedLegacyPptTablePositions(cells []legacyPptTableCellShape, project func(legacyPptTableCellShape) int64) []int64 {
	set := map[int64]struct{}{}
	for _, cell := range cells {
		set[project(cell)] = struct{}{}
	}
	out := make([]int64, 0, len(set))
	for pos := range set {
		out = append(out, pos)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
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

func parseLegacyPptSlideConnectorShapes(stream []byte, persistDirectory map[int]int, slidePersistIDs []int, colorScheme map[uint8]string, existing map[int][]*presentationir.Shape) map[int][]*presentationir.Shape {
	if len(slidePersistIDs) == 0 || len(persistDirectory) == 0 {
		return nil
	}
	out := map[int][]*presentationir.Shape{}
	for _, persistID := range slidePersistIDs {
		slideOffset, ok := persistDirectory[persistID]
		if !ok {
			continue
		}
		record, payload, _, ok := readPptRecord(stream, slideOffset)
		if !ok || record.Type != pptRTSlide {
			continue
		}
		shapeIndex := legacyPptShapeIndexByOriginalID(existing[persistID])
		shapes := parseLegacyPptConnectorShapesFromSlideWithTargets(payload, colorScheme, shapeIndex)
		if len(shapes) > 0 {
			out[persistID] = shapes
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyPptConnectorShapesFromSlide(payload []byte, colorScheme map[uint8]string) []*presentationir.Shape {
	return parseLegacyPptConnectorShapesFromSlideWithTargets(payload, colorScheme, nil)
}

func parseLegacyPptConnectorShapesFromSlideWithTargets(payload []byte, colorScheme map[uint8]string, shapeIndex map[uint32]*presentationir.Shape) []*presentationir.Shape {
	if len(payload) == 0 {
		return nil
	}
	var shapes []*presentationir.Shape
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTPPDrawing {
			rules := parseLegacyPptConnectorRules(childPayload)
			collectLegacyPptConnectorShapesFromOfficeArt(childPayload, legacyPptGroupFrame{}, colorScheme, rules, shapeIndex, &shapes)
		}
		pos = next
	}
	if len(shapes) == 0 {
		return nil
	}
	return shapes
}

func collectLegacyPptConnectorShapesFromOfficeArt(data []byte, frame legacyPptGroupFrame, colorScheme map[uint8]string, rules map[uint32]legacyPptConnectorRule, shapeIndex map[uint32]*presentationir.Shape, shapes *[]*presentationir.Shape) {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		version := verInst & 0x000F
		body := data[off+8 : off+8+recLen]
		switch recType {
		case legacyOfficeArtSpgrContainer:
			if legacyPptGroupContainerIsTable(body) {
				off += 8 + recLen
				continue
			}
			nextFrame := frame
			if groupFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(body, frame); ok {
				nextFrame = groupFrame
			}
			collectLegacyPptConnectorShapesFromOfficeArt(body, nextFrame, colorScheme, rules, shapeIndex, shapes)
		case legacyOfficeArtSpContainer:
			connector, ok := parseLegacyPptConnectorShapeContainer(body, frame, colorScheme)
			if ok {
				start, end := legacyPptConnectorEndpoints(connector.bounds, connector.transform.FlipH, connector.transform.FlipV)
				if rule, ok := rules[connector.shapeID]; ok {
					if target := shapeIndex[rule.startShapeID]; target != nil {
						start = legacyPptConnectionSitePoint(target.Bounds, rule.startSite)
					}
					if target := shapeIndex[rule.endShapeID]; target != nil {
						end = legacyPptConnectionSitePoint(target.Bounds, rule.endSite)
					}
				}
				connectorIR := &presentationir.Shape{
					Kind:                 presentationir.ShapeConnector,
					Name:                 fmt.Sprintf("Legacy Connector %d", len(*shapes)+1),
					OriginalShapeID:      connector.shapeID,
					Bounds:               connector.bounds,
					ConnectorType:        legacyPptConnectorType(connector.shapeType),
					ConnectorFlipH:       connector.transform.FlipH,
					ConnectorFlipV:       connector.transform.FlipV,
					ConnectorAdjustments: append([]int(nil), connector.adjustments...),
					ConnectorStart:       start,
					ConnectorEnd:         end,
				}
				applyLegacyPptShapeTransform(connectorIR, connector.transform)
				applyLegacyPptAnimationMetadata(connectorIR, connector.animation)
				*shapes = append(*shapes, connectorIR)
			}
		default:
			if version == 0x000F {
				collectLegacyPptConnectorShapesFromOfficeArt(body, frame, colorScheme, rules, shapeIndex, shapes)
			}
		}
		off += 8 + recLen
	}
}

func parseLegacyPptConnectorShapeContainer(data []byte, frame legacyPptGroupFrame, colorScheme map[uint8]string) (legacyPptConnectorShape, bool) {
	var shape legacyPptConnectorShape
	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil || node == nil {
		return legacyPptConnectorShape{}, false
	}
	shape.shapeID = node.ShapeID
	shape.shapeType = node.ShapeType
	shape.transform.FlipH = node.Flags&legacyOfficeArtSpFlagFlipH != 0
	shape.transform.FlipV = node.Flags&legacyOfficeArtSpFlagFlipV != 0

	if len(node.Properties) > 0 {
		parseLegacyPptFOPTPropertiesTransform(node.Properties, colorScheme, &shape.transform)
		shape.adjustments = parseLegacyPptConnectorAdjustmentsProperties(node.Properties)
	}
	if len(node.ClientData) > 0 {
		if anim, ok := parseLegacyPptShapeAnimationInfo(node.ClientData); ok {
			shape.animation = anim
		}
	}

	var clientAnchorBounds, childAnchorBounds presentationir.Rect
	haveClientAnchor, haveChildAnchor := false, false
	if len(node.ClientAnchor) > 0 {
		if bounds, ok := parseLegacyPptClientAnchor(node.ClientAnchor); ok {
			clientAnchorBounds = bounds
			haveClientAnchor = true
		}
	}
	if len(node.ChildAnchor) > 0 {
		if localBounds, ok := parseLegacyPptChildAnchor(node.ChildAnchor); ok {
			if mappedBounds, ok := mapLegacyPptRectThroughGroupFrame(localBounds, frame); ok {
				childAnchorBounds = mappedBounds
				haveChildAnchor = true
			}
		}
	}

	haveAnchor := false
	if node.Flags&legacyOfficeArtSpFlagChild != 0 && haveChildAnchor {
		shape.bounds = childAnchorBounds
		haveAnchor = true
	} else if haveClientAnchor {
		shape.bounds = clientAnchorBounds
		haveAnchor = true
	} else if haveChildAnchor {
		shape.bounds = childAnchorBounds
		haveAnchor = true
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

func parseLegacyPptConnectorAdjustments(body []byte, propCount int) []int {
	if propCount <= 0 || len(body) < propCount*6 {
		return nil
	}
	values := make(map[int]int)
	maxIndex := 0
	for index := 0; index < propCount; index++ {
		offset := index * 6
		opid := binary.LittleEndian.Uint16(body[offset:offset+2]) & 0x3FFF
		if opid < legacyOfficeArtPropAdjustValue || opid > legacyOfficeArtPropAdjustValue+9 {
			continue
		}
		adjIndex := int(opid-legacyOfficeArtPropAdjustValue) + 1
		raw := int64(int32(binary.LittleEndian.Uint32(body[offset+2 : offset+6])))
		values[adjIndex] = normalizeLegacyPptConnectorAdjustment(raw)
		if adjIndex > maxIndex {
			maxIndex = adjIndex
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

func legacyPptConnectionSitePoint(bounds presentationir.Rect, site uint32) *presentationir.Point {
	// Standard Office preset shapes enumerate the four cardinal sites clockwise
	// from the right edge. Shapes with additional sites still get a stable
	// cardinal fallback by cycling the site index.
	cx := bounds.X + bounds.W/2
	cy := bounds.Y + bounds.H/2
	var point presentationir.Point
	switch site % 4 {
	case 0:
		point = presentationir.Point{X: bounds.X + bounds.W, Y: cy}
	case 1:
		point = presentationir.Point{X: cx, Y: bounds.Y + bounds.H}
	case 2:
		point = presentationir.Point{X: bounds.X, Y: cy}
	default:
		point = presentationir.Point{X: cx, Y: bounds.Y}
	}
	return &point
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

func parseLegacyPptImageShapesFromSlide(payload []byte, pictureStore map[uint32]*legacyPptPicture, externalObjects map[uint32]legacyPptExternalObject) []*presentationir.Shape {
	if len(payload) == 0 || len(pictureStore) == 0 {
		return nil
	}
	var shapes []*presentationir.Shape
	for pos := 0; pos+8 <= len(payload); {
		record, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if record.Type == pptRTPPDrawing {
			collectLegacyPptImageShapesFromOfficeArt(childPayload, legacyPptGroupFrame{}, pictureStore, externalObjects, &shapes)
		}
		pos = next
	}
	if len(shapes) == 0 {
		return nil
	}
	return shapes
}

func collectLegacyPptImageShapesFromOfficeArt(data []byte, frame legacyPptGroupFrame, pictureStore map[uint32]*legacyPptPicture, externalObjects map[uint32]legacyPptExternalObject, shapes *[]*presentationir.Shape) {
	for off := 0; off+8 <= len(data); {
		verInst := binary.LittleEndian.Uint16(data[off : off+2])
		recType := binary.LittleEndian.Uint16(data[off+2 : off+4])
		recLen := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recLen < 0 || off+8+recLen > len(data) {
			break
		}
		version := verInst & 0x000F
		body := data[off+8 : off+8+recLen]
		switch recType {
		case legacyOfficeArtSpgrContainer:
			nextFrame := frame
			if groupFrame, ok := parseLegacyPptGroupFrameFromSpgrContainer(body, frame); ok {
				nextFrame = groupFrame
			}
			collectLegacyPptImageShapesFromOfficeArt(body, nextFrame, pictureStore, externalObjects, shapes)
		case legacyOfficeArtSpContainer:
			pictureShape, ok := parseLegacyPptPictureShapeContainer(body, frame)
			if !ok {
				break
			}
			picture, ok := pictureStore[pictureShape.blipRef]
			if !ok || picture == nil || len(picture.Data) == 0 || picture.MIME == "" {
				break
			}
			pictureIR := buildLegacyPptPictureOrExternalObjectShape(len(*shapes)+1, pictureShape, picture, externalObjects[pictureShape.exObjID])
			applyLegacyPptShapeTransform(pictureIR, pictureShape.transform)
			applyLegacyPptAnimationMetadata(pictureIR, pictureShape.animation)
			*shapes = append(*shapes, pictureIR)
		default:
			if version == 0x000F {
				collectLegacyPptImageShapesFromOfficeArt(body, frame, pictureStore, externalObjects, shapes)
			}
		}
		off += 8 + recLen
	}
}

func buildLegacyPptPictureOrExternalObjectShape(ordinal int, pictureShape legacyPptPictureShape, picture *legacyPptPicture, ext legacyPptExternalObject) *presentationir.Shape {
	if picture == nil {
		return nil
	}
	if ext.Kind == presentationir.ShapeOleObject {
		return &presentationir.Shape{
			Kind:             presentationir.ShapeOleObject,
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
	}
	if ext.Kind == presentationir.ShapeMedia {
		return &presentationir.Shape{
			Kind:            presentationir.ShapeMedia,
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
	return &presentationir.Shape{
		Kind:      presentationir.ShapeImage,
		Name:      fmt.Sprintf("Legacy Picture %d", ordinal),
		Bounds:    pictureShape.bounds,
		ImageData: append([]byte(nil), picture.Data...),
		ImageMIME: picture.MIME,
	}
}

func parseLegacyPptPictureShapeContainer(data []byte, frame legacyPptGroupFrame) (legacyPptPictureShape, bool) {
	var shape legacyPptPictureShape
	node, err := officeartescher.ParseShapeContainerPayload(data)
	if err != nil || node == nil {
		return legacyPptPictureShape{}, false
	}
	shape.transform.FlipH = node.Flags&legacyOfficeArtSpFlagFlipH != 0
	shape.transform.FlipV = node.Flags&legacyOfficeArtSpFlagFlipV != 0

	if len(node.Properties) > 0 {
		if blipRef, ok := parseLegacyPptPictureBlipRefProperties(node.Properties); ok {
			shape.blipRef = blipRef
		}
		parseLegacyPptFOPTPropertiesTransform(node.Properties, nil, &shape.transform)
	}
	if len(node.ClientData) > 0 {
		if exObjID, ok := parseLegacyPptExObjRef(node.ClientData); ok {
			shape.exObjID = exObjID
		}
		if anim, ok := parseLegacyPptShapeAnimationInfo(node.ClientData); ok {
			shape.animation = anim
		}
	}

	var clientAnchorBounds, childAnchorBounds presentationir.Rect
	haveClientAnchor, haveChildAnchor := false, false
	if len(node.ClientAnchor) > 0 {
		if bounds, ok := parseLegacyPptClientAnchor(node.ClientAnchor); ok {
			clientAnchorBounds = bounds
			haveClientAnchor = true
		}
	}
	if len(node.ChildAnchor) > 0 {
		if localBounds, ok := parseLegacyPptChildAnchor(node.ChildAnchor); ok {
			if mappedBounds, ok := mapLegacyPptRectThroughGroupFrame(localBounds, frame); ok {
				childAnchorBounds = mappedBounds
				haveChildAnchor = true
			}
		}
	}

	haveAnchor := false
	if node.Flags&legacyOfficeArtSpFlagChild != 0 && haveChildAnchor {
		shape.bounds = childAnchorBounds
		haveAnchor = true
	} else if haveClientAnchor {
		shape.bounds = clientAnchorBounds
		haveAnchor = true
	} else if haveChildAnchor {
		shape.bounds = childAnchorBounds
		haveAnchor = true
	}
	if node.ShapeType != legacyOfficeArtShapePicture || shape.blipRef == 0 || !haveAnchor {
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

func parseLegacyPptPictureBlipRef(body []byte, propCount int) (uint32, bool) {
	for index := 0; index < propCount; index++ {
		offset := index * 6
		if offset+6 > len(body) {
			break
		}
		opid := binary.LittleEndian.Uint16(body[offset : offset+2])
		if opid&0x3FFF != legacyOfficeArtPropPib {
			continue
		}
		return binary.LittleEndian.Uint32(body[offset+2 : offset+6]), true
	}
	return 0, false
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

// applyLegacyPptShapeTransform copies rotation/flip/line properties from a
// legacy FSP+FOPT decode onto an IR Shape. Line color/width are emitted only
// when the FOPT line booleans either say "use" or, in their absence, when an
// explicit line property was actually present (the zero values mean "unset").
func applyLegacyPptShapeTransform(shape *presentationir.Shape, t legacyPptShapeTransform) {
	if shape == nil {
		return
	}
	if t.HasRotation {
		shape.RotationDeg = t.Rotation
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
			ls := drawing.DefaultLineStyle()
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
			shape.ArrowHeadWidth = t.ArrowHeadWidth
			shape.ArrowHeadLength = t.ArrowHeadLength
		}
		if t.ArrowTail != "" {
			shape.ArrowTail = t.ArrowTail
			shape.ArrowTailWidth = t.ArrowTailWidth
			shape.ArrowTailLength = t.ArrowTailLength
		}
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
	}
	if t.HasTextInsets {
		shape.TextInsetLeftEMU = t.TextInsetLeft
		shape.TextInsetTopEMU = t.TextInsetTop
		shape.TextInsetRightEMU = t.TextInsetRight
		shape.TextInsetBottomEMU = t.TextInsetBottom
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
		return resolveLegacyPptColorRef(byte(raw), byte(raw>>8), byte(raw>>16), byte(raw>>24), colorScheme)
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
	if visual.StartArrow != "" {
		out.ArrowTail = string(visual.StartArrow)
	}
	if visual.EndArrow != "" {
		out.ArrowHead = string(visual.EndArrow)
	}
	if visual.StartArrowWidth != "" {
		out.ArrowTailWidth = string(visual.StartArrowWidth)
	}
	if visual.StartArrowLength != "" {
		out.ArrowTailLength = string(visual.StartArrowLength)
	}
	if visual.EndArrowWidth != "" {
		out.ArrowHeadWidth = string(visual.EndArrowWidth)
	}
	if visual.EndArrowLength != "" {
		out.ArrowHeadLength = string(visual.EndArrowLength)
	}

	// PowerPoint's binary arrow enum compatibility differs from the generic
	// OfficeArt interpretation for several raw values. Preserve the raw PPT
	// contract after shared visual decoding; shared fill/line/text semantics
	// remain authoritative for all non-arrow properties.
	for _, prop := range props {
		switch prop.ID {
		case officeartescher.PropLineStartArrow:
			out.ArrowTail = legacyPptArrowStyle(prop.Value)
		case officeartescher.PropLineEndArrow:
			out.ArrowHead = legacyPptArrowStyle(prop.Value)
		case officeartescher.PropLineStartArrowWidth:
			out.ArrowTailWidth = legacyPptArrowSize(prop.Value)
		case officeartescher.PropLineStartArrowLength:
			out.ArrowTailLength = legacyPptArrowSize(prop.Value)
		case officeartescher.PropLineEndArrowWidth:
			out.ArrowHeadWidth = legacyPptArrowSize(prop.Value)
		case officeartescher.PropLineEndArrowLength:
			out.ArrowHeadLength = legacyPptArrowSize(prop.Value)
		}
	}

	text := visual.Text
	if text.HasLeft || text.HasTop || text.HasRight || text.HasBottom {
		out.HasTextInsets = true
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
	}
	if text.HasWrap {
		out.HasAllowOverflow = true
		out.AllowOverflow = text.Wrap == officeartir.TextWrapNone
	}
	if text.HasAutoFit {
		out.HasAutoGrow = true
		out.AutoGrow = text.AutoFit == officeartir.AutoFitShape
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

// legacyPptFopt32ToFraction remains as a compatibility wrapper until PPT's
// FOPT parser is moved wholly into pkg/officeart/legacy.
func legacyPptFopt32ToFraction(raw uint32) float64 {
	return officeartlegacy.Fixed16ToFraction(raw)
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

func parseLegacyPptFOPTPropertiesFillColor(props []officeartescher.Property, colorScheme map[uint8]string) (string, bool) {
	resolve := func(raw uint32) string {
		return resolveLegacyPptColorRef(byte(raw), byte(raw>>8), byte(raw>>16), byte(raw>>24), colorScheme)
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

func parseLegacyPptMasterBgFill(stream []byte, documentOffset int, colorScheme map[uint8]string) string {
	record, payload, _, ok := readPptRecord(stream, documentOffset)
	if !ok || record.Type != pptRTDocument || record.Version != 0x0F {
		return ""
	}
	for pos := 0; pos+8 <= len(payload); {
		child, childPayload, next, ok := readPptRecord(payload, pos)
		if !ok {
			break
		}
		if child.Type == pptRTMainMaster {
			if fill := parseLegacyPptBgFillFromContainer(childPayload, colorScheme); fill != "" {
				return fill
			}
		}
		pos = next
	}
	return ""
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
		if recType == legacyOfficeArtFOPT && fillColor == "" {
			if color, ok := parseLegacyPptFOPTFillColor(body, int(inst), colorScheme); ok {
				fillColor = color
			}
		}
	})
	return fillColor
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
			if mapped, ok := mapLegacyPptRectThroughGroupFrame(rect, parent); ok {
				childAnchorBounds = mapped
				haveChildAnchor = true
			}
		}
	}

	var target presentationir.Rect
	switch {
	case haveClientAnchor:
		target = clientAnchorBounds
	case haveChildAnchor:
		target = childAnchorBounds
	default:
		return legacyPptGroupFrame{}, false
	}
	if target.W == 0 || target.H == 0 {
		return legacyPptGroupFrame{}, false
	}
	return legacyPptGroupFrame{domain: domain, target: target, valid: true}, true
}

func parseLegacyPptGroupRect(body []byte) (presentationir.Rect, bool) {
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

func applyLegacyPptGroupTransformToShape(shape *presentationir.Shape, groupBounds presentationir.Rect, transform legacyPptShapeTransform) {
	if shape == nil {
		return
	}
	shape.Bounds = transformLegacyPptRectAroundGroup(shape.Bounds, groupBounds, transform)
	if transform.HasRotation {
		shape.RotationDeg += transform.Rotation
	}
	shape.FlipH = shape.FlipH != transform.FlipH
	shape.FlipV = shape.FlipV != transform.FlipV
	shape.ConnectorFlipH = shape.ConnectorFlipH != transform.FlipH
	shape.ConnectorFlipV = shape.ConnectorFlipV != transform.FlipV
	shape.ConnectorStart = transformLegacyPptPointAroundGroup(shape.ConnectorStart, groupBounds, transform)
	shape.ConnectorEnd = transformLegacyPptPointAroundGroup(shape.ConnectorEnd, groupBounds, transform)
	for index := range shape.ConnectorPath {
		point := shape.ConnectorPath[index]
		mapped := transformLegacyPptPointAroundGroup(&point, groupBounds, transform)
		if mapped != nil {
			shape.ConnectorPath[index] = *mapped
		}
	}
	for _, child := range shape.Children {
		applyLegacyPptGroupTransformToShape(child, groupBounds, transform)
	}
}

func transformLegacyPptRectAroundGroup(rect, groupBounds presentationir.Rect, transform legacyPptShapeTransform) presentationir.Rect {
	if (!transform.HasRotation || transform.Rotation == 0) && !transform.FlipH && !transform.FlipV {
		return rect
	}
	center := presentationir.Point{X: rect.X + rect.W/2, Y: rect.Y + rect.H/2}
	mapped := transformLegacyPptPointAroundGroup(&center, groupBounds, transform)
	if mapped == nil {
		return rect
	}
	return presentationir.Rect{X: mapped.X - rect.W/2, Y: mapped.Y - rect.H/2, W: rect.W, H: rect.H}
}

func transformLegacyPptPointAroundGroup(point *presentationir.Point, groupBounds presentationir.Rect, transform legacyPptShapeTransform) *presentationir.Point {
	if point == nil {
		return nil
	}
	cx := float64(groupBounds.X) + float64(groupBounds.W)/2
	cy := float64(groupBounds.Y) + float64(groupBounds.H)/2
	dx := float64(point.X) - cx
	dy := float64(point.Y) - cy
	if transform.FlipH {
		dx = -dx
	}
	if transform.FlipV {
		dy = -dy
	}
	if transform.HasRotation && transform.Rotation != 0 {
		radians := transform.Rotation * math.Pi / 180
		cosTheta := math.Cos(radians)
		sinTheta := math.Sin(radians)
		dx, dy = dx*cosTheta-dy*sinTheta, dx*sinTheta+dy*cosTheta
	}
	return &presentationir.Point{X: int64(math.Round(cx + dx)), Y: int64(math.Round(cy + dy))}
}

func parseLegacyPptClientAnchor(body []byte) (presentationir.Rect, bool) {
	if len(body) < 8 {
		return presentationir.Rect{}, false
	}
	left := int(binary.LittleEndian.Uint16(body[0:2]))
	top := int(binary.LittleEndian.Uint16(body[2:4]))
	right := int(binary.LittleEndian.Uint16(body[4:6]))
	bottom := int(binary.LittleEndian.Uint16(body[6:8]))
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
		X: int64(left) * defaultSlideWidthEMU / legacyPptMasterWidth,
		Y: int64(top) * defaultSlideHeightEMU / legacyPptMasterHeight,
		W: int64(right-left) * defaultSlideWidthEMU / legacyPptMasterWidth,
		H: int64(bottom-top) * defaultSlideHeightEMU / legacyPptMasterHeight,
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

func legacyPptImageContentType(data []byte) string {
	return officeartlegacy.SniffImageMIME(data)
}

func legacyPptImageEnd(data []byte, mime string) (int, bool) {
	return officeartlegacy.ImageEnd(data, mime)
}

func legacyPptScanPNGEnd(data []byte) (int, bool) {
	if len(data) < 8 {
		return 0, false
	}
	off := 8
	for off+8 <= len(data) {
		chunkLen := int(binary.BigEndian.Uint32(data[off : off+4]))
		if off+12+chunkLen > len(data) {
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

func legacyPptScanJPEGEnd(data []byte) (int, bool) {
	for i := 2; i < len(data)-1; i++ {
		if data[i] == 0xFF && data[i+1] == 0xD9 {
			return i + 2, true
		}
	}
	return 0, false
}

func legacyPptScanGIFEnd(data []byte) (int, bool) {
	for i := 6; i < len(data); i++ {
		if data[i] == 0x3B {
			return i + 1, true
		}
	}
	return 0, false
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
		pos += 2 + count*4
		if pos > len(payload) {
			return ruler, false
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

	for i := 0; i < 5; i++ {
		if ruler.hasLeft[i] || ruler.hasIndent[i] {
			return ruler, true
		}
	}
	return ruler, false
}

// applyRulerLevelsToParas applies ruler-defined margins/indents when paragraph
// values are absent and uses ruler left-margins to infer paragraph level.
// 1 master unit = 127 EMU.
func applyRulerLevelsToParas(paras []*presentationir.Paragraph, ruler legacyPptTextRuler) {
	if len(paras) == 0 {
		return
	}
	for _, p := range paras {
		if p == nil {
			continue
		}

		if p.HasMarginLeft {
			muVal := p.MarginLeftEMU / 127
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
			p.MarginLeftEMU = ruler.leftMargins[p.Level] * 127
			p.HasMarginLeft = true
		}
		if !p.HasTextIndent && p.Level >= 0 && p.Level < 5 && ruler.hasIndent[p.Level] {
			p.TextIndentEMU = ruler.indents[p.Level] * 127
			p.HasTextIndent = true
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
			if exc.hasBulletAutoNumber {
				p.AutoNum = true
				p.AutoNumType = exc.autoNumberScheme
				if exc.autoNumberStart > 0 {
					p.AutoNumStart = exc.autoNumberStart
				}
			}
		}
	}
}
