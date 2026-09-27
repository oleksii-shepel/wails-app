package metafile

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"

	"office/pkg/fontmetrics"
)

const (
	emrHeader                  = 1
	emrSetWindowExtEx          = 9
	emrSetPixelV               = 15
	emrSetMapperFlags          = 16
	emrSetWindowOrgEx          = 10
	emrSetMapMode              = 17
	emrSetViewportExtEx        = 11
	emrSetViewportOrgEx        = 12
	emrSetBrushOrgEx           = 13
	emrEOF                     = 14
	emrSetBkMode               = 18
	emrSetPolyFillMode         = 19
	emrSetTextAlign            = 22
	emrSetTextColor            = 24
	emrSetBkColor              = 25
	emrOffsetClipRgn           = 26
	emrMoveToEx                = 27
	emrExcludeClipRect         = 29
	emrIntersectClipRect       = 30
	emrSaveDC                  = 33
	emrRestoreDC               = 34
	emrSetWorldTransform       = 35
	emrModifyWorldTransform    = 36
	emrSelectObject            = 37
	emrCreatePen               = 38
	emrCreateBrushIndirect     = 39
	emrDeleteObject            = 40
	emrAngleArc                = 41
	emrArcTo                   = 55
	emrLineTo                  = 54
	emrBeginPath               = 59
	emrEndPath                 = 60
	emrCloseFigure             = 61
	emrFillPath                = 62
	emrStrokeAndFillPath       = 63
	emrStrokePath              = 64
	emrSelectClipPath          = 67
	emrAbortPath               = 68
	emrFillRgn                 = 71
	emrPaintRgn                = 74
	emrExtSelectClipRgn        = 75
	emrPolyBezier              = 2
	emrPolygon                 = 3
	emrPolyline                = 4
	emrPolyBezierTo            = 5
	emrPolylineTo              = 6
	emrPolyPolyline            = 7
	emrPolyPolygon             = 8
	emrEllipse                 = 42
	emrRectangle               = 43
	emrRoundRect               = 44
	emrArc                     = 45
	emrChord                   = 46
	emrPie                     = 47
	emrSetArcDirection         = 57
	emrPolyBezier16            = 85
	emrPolygon16               = 86
	emrPolyline16              = 87
	emrPolyBezierTo16          = 88
	emrPolylineTo16            = 89
	emrPolyPolyline16          = 90
	emrPolyPolygon16           = 91
	emrExtCreateFontIndirectW  = 82
	emrExtTextOutW             = 84
	emrPolyTextOutW            = 97
	emrSmallTextOut            = 108
	emrAlphaBlend              = 114
	emrSetLayout               = 115
	emrTransparentBlt          = 116
	emrGDIComment              = 70
	emrBitBlt                  = 76
	emrStretchBlt              = 77
	emrMaskBlt                 = 78
	emrPlgBlt                  = 79
	emrSetDIBitsToDevice       = 80
	emrStretchDIBits           = 81
	emrCreateMonoBrush         = 93
	emrCreateDIBPatternBrushPt = 94
	emrExtCreatePen            = 95
	emrGradientFill            = 118
	emrSetTextJustification    = 120
	emfROPSourceCopy           = 0x00CC0020
)

type emfObject struct {
	kind        string
	color       string
	width       float64
	cosmetic    bool // LOGPEN width == 0: one device pixel, independent of logical/viewBox scaling
	null        bool
	brushStyle  uint32
	hatch       uint32
	patternURI  string
	patternW    int
	patternH    int
	fontFamily  string
	fontSize    float64
	fontWeight  string
	italic      bool
	fontCharset byte
	underline   bool
	strikeout   bool
	escapement  int32 // tenths of a degree, LOGFONT semantics
	orientation int32
}

type emfTransform struct {
	m11 float64
	m12 float64
	m21 float64
	m22 float64
	dx  float64
	dy  float64
}

type emfState struct {
	textColor       string
	bkColor         string
	transparent     bool
	penColor        string
	penWidth        float64
	penNull         bool
	brushColor      string
	brushPaint      string
	brushNull       bool
	brushOrgX       float64
	brushOrgY       float64
	brushStyle      uint32
	brushHatch      uint32
	brushPatternURI string
	brushPatternW   int
	brushPatternH   int
	fontFamily      string
	fontSize        float64
	fontWeight      string
	fontItalic      bool
	fontUnderline   bool
	fontStrikeout   bool
	fontEscapement  int32
	fontOrientation int32
	textJustExtra   int32
	textJustCount   int32
	mapperFlags     uint32
	layoutMode      uint32
	currentX        float64
	currentY        float64
	windowOrgX      float64
	windowOrgY      float64
	windowExtX      float64
	windowExtY      float64
	viewportOrgX    float64
	viewportOrgY    float64
	viewportExtX    float64
	viewportExtY    float64
	mapMode         int32
	arcDirection    uint32
	textAlign       uint32
	polyFillMode    uint32
	clipChain       []string
	clipRects       []emfRect
	clipRectsKnown  bool
	world           emfTransform
	deviceOrgX      float64
	deviceOrgY      float64
}

type emfRect struct{ x1, y1, x2, y2 float64 }

type emfPathState struct {
	active bool
	d      strings.Builder
}

func (p *emfPathState) reset(active bool) {
	p.active = active
	p.d.Reset()
}

func (p *emfPathState) empty() bool {
	return strings.TrimSpace(p.d.String()) == ""
}

func (p *emfPathState) command(format string, args ...any) {
	if p.d.Len() > 0 {
		p.d.WriteByte(' ')
	}
	fmt.Fprintf(&p.d, format, args...)
}

func emfCloneClipChain(in []string) []string {
	return append([]string(nil), in...)
}

func emfCloneRects(in []emfRect) []emfRect { return append([]emfRect(nil), in...) }

func emfSyncClipGroups(body *strings.Builder, current *[]string, target []string) {
	common := 0
	for common < len(*current) && common < len(target) && (*current)[common] == target[common] {
		common++
	}
	for i := len(*current); i > common; i-- {
		body.WriteString("</g>")
	}
	for i := common; i < len(target); i++ {
		fmt.Fprintf(body, `<g clip-path="url(#%s)">`, target[i])
	}
	*current = emfCloneClipChain(target)
}

func emfDefineClipPath(body *strings.Builder, openGroups *[]string, id, element string) {
	previous := emfCloneClipChain(*openGroups)
	emfSyncClipGroups(body, openGroups, nil)
	fmt.Fprintf(body, `<defs><clipPath id="%s">%s</clipPath></defs>`, id, element)
	emfSyncClipGroups(body, openGroups, previous)
}

// FontFace supplies an actual font program for metafile text realization.
// Family/style identify the corresponding EMF+ Font object; Data should be an
// sfnt TrueType/OpenType payload. Metafile conversion remains independent from
// document parsers: callers that own embedded/private fonts opt in explicitly.
type FontFace struct {
	Family string
	Bold   bool
	Italic bool
	Data   []byte
}

// Options carries optional assets that are not embedded in the metafile stream.
type ColorReplacement struct {
	Source      string
	Destination string
}

type Options struct {
	Fonts    []FontFace
	Recolors []ColorReplacement
}

// ToSVG converts browser-unsupported Office metafiles into SVG. WMF files
// that carry the standard WMFC embedded-EMF escape are reconstructed before
// using the same EMF projection path.
func ToSVG(data []byte, mime string) ([]byte, float64, float64, bool) {
	return ToSVGWithOptions(data, mime, Options{})
}

// ToSVGWithOptions is ToSVG with optional font realization assets.
func ToSVGWithOptions(data []byte, mime string, opts Options) ([]byte, float64, float64, bool) {
	svg, width, height, ok := toSVGWithOptionsRaw(data, mime, opts)
	if !ok {
		return nil, 0, 0, false
	}
	return applySVGColorReplacements(svg, opts.Recolors), width, height, true
}

func toSVGWithOptionsRaw(data []byte, mime string, opts Options) ([]byte, float64, float64, bool) {
	// Office packages sometimes keep an EMF payload under a .wmf relationship
	// target (37625.pptx does this for its embedded OLE chart previews).  The
	// payload signature is authoritative: dispatching only from the extension MIME
	// either leaves the preview browser-dependent or routes it through the classic
	// WMF decoder and loses authored GDI pen state.
	if looksLikeEMF(data) {
		return emfToSVGWithOptions(data, opts)
	}
	switch mime {
	case "image/emf", "image/x-emf":
		return emfToSVGWithOptions(data, opts)
	case "image/wmf", "image/x-wmf":
		if emf, ok := embeddedEMFFromWMF(data); ok {
			return emfToSVGWithOptions(emf, opts)
		}
		return wmfToSVGWithOptions(data, opts)
	default:
		return nil, 0, 0, false
	}
}

func wmfFontCatalog(faces []FontFace) *fontmetrics.Catalog {
	if len(faces) == 0 {
		return nil
	}
	catalog := &fontmetrics.Catalog{Faces: make([]fontmetrics.NamedFace, 0, len(faces))}
	for _, face := range faces {
		parsed, ok := fontmetrics.ParseSFNT(face.Data)
		if !ok {
			continue
		}
		weight := 400
		if face.Bold {
			weight = 700
		}
		catalog.Faces = append(catalog.Faces, fontmetrics.NamedFace{
			Family: face.Family,
			Weight: weight,
			Italic: face.Italic,
			Face:   parsed,
		})
	}
	if len(catalog.Faces) == 0 {
		return nil
	}
	return catalog
}

func wmfApproxAdvanceFactor(family string, r rune) float64 {
	f := strings.ToLower(strings.TrimSpace(family))
	serif := strings.Contains(f, "times") || strings.Contains(f, "cambria") || strings.Contains(f, "serif")
	if unicode.IsSpace(r) {
		if serif {
			return 0.25
		}
		return 0.278
	}
	if strings.Contains(f, "symbol") || strings.Contains(f, "wingdings") || strings.Contains(f, "webdings") {
		return 1
	}
	if r > 0x2fff {
		return 1
	}
	if unicode.IsDigit(r) {
		if serif {
			return 0.50
		}
		return 0.556
	}
	if unicode.IsUpper(r) {
		return 0.667
	}
	if unicode.IsLower(r) {
		if serif {
			return 0.444
		}
		return 0.50
	}
	if unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return 0.278
	}
	return 0.50
}

func wmfApproxTextAdvance(text, family string, fontSize, letterSpacing float64) float64 {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	total := 0.0
	for i, r := range runes {
		total += wmfApproxAdvanceFactor(family, r) * fontSize
		if i+1 < len(runes) {
			total += letterSpacing
		}
	}
	return total
}

func wmfTextAdvance(catalog *fontmetrics.Catalog, text string, font wmfObject, fontSize, letterSpacing float64) float64 {
	if catalog != nil {
		advance, ok := catalog.MeasureText(text, font.fontFamily, font.fontWeight == "700", font.italic, fontSize)
		if ok {
			if letterSpacing != 0 {
				advance += float64(max(0, len([]rune(text))-1)) * letterSpacing
			}
			return advance
		}
	}
	return wmfApproxTextAdvance(text, font.fontFamily, fontSize, letterSpacing)
}

func applySVGColorReplacements(svg []byte, replacements []ColorReplacement) []byte {
	if len(svg) == 0 || len(replacements) == 0 {
		return svg
	}
	text := string(svg)
	for _, replacement := range replacements {
		source := strings.TrimPrefix(strings.TrimSpace(replacement.Source), "#")
		destination := strings.TrimPrefix(strings.TrimSpace(replacement.Destination), "#")
		if len(source) != 6 || len(destination) != 6 {
			continue
		}
		for _, candidate := range []string{"#" + strings.ToLower(source), "#" + strings.ToUpper(source)} {
			text = strings.ReplaceAll(text, candidate, "#"+strings.ToUpper(destination))
		}
	}
	return []byte(text)
}

func looksLikeEMF(data []byte) bool {
	return len(data) >= 44 &&
		binary.LittleEndian.Uint32(data[0:4]) == emrHeader &&
		string(data[40:44]) == " EMF"
}

func embeddedEMFFromWMF(data []byte) ([]byte, bool) {
	off := 0
	if len(data) >= 22 && binary.LittleEndian.Uint32(data[:4]) == 0x9ac6cdd7 {
		off = 22
	}
	if len(data) < off+18 {
		return nil, false
	}
	off += 18
	var emf []byte
	for off+6 <= len(data) {
		words := int(binary.LittleEndian.Uint32(data[off : off+4]))
		fn := binary.LittleEndian.Uint16(data[off+4 : off+6])
		if words < 3 || off+words*2 > len(data) {
			break
		}
		rec := data[off+6 : off+words*2]
		if fn == 0x0626 && len(rec) >= 4 {
			escape := binary.LittleEndian.Uint16(rec[:2])
			count := int(binary.LittleEndian.Uint16(rec[2:4]))
			if escape == 0x000f && count >= 34 && len(rec) >= 4+count {
				chunk := rec[4 : 4+count]
				if len(chunk) >= 34 && string(chunk[:4]) == "WMFC" {
					emf = append(emf, chunk[34:]...)
				}
			}
		}
		off += words * 2
		if fn == 0 {
			break
		}
	}
	if len(emf) < 44 || binary.LittleEndian.Uint32(emf[:4]) != emrHeader || string(emf[40:44]) != " EMF" {
		return nil, false
	}
	return emf, true
}

type wmfObject struct {
	kind        string
	color       string
	width       float64
	cosmetic    bool // LOGPEN width == 0: one device pixel, independent of logical/viewBox scaling
	null        bool
	fontFamily  string
	fontSize    float64
	fontWeight  string
	italic      bool
	fontCharset byte
	underline   bool
	strikeout   bool
	escapement  int32 // tenths of a degree, LOGFONT semantics
	orientation int32
}

// wmfToSVG projects the browser-unsupported WMF drawing primitives used by
// legacy Office clip-art into SVG. Object handles follow the WMF object-table
// contract: create records claim the first free slot and DeleteObject releases it.
func wmfToSVG(data []byte) ([]byte, float64, float64, bool) {
	return wmfToSVGWithOptions(data, Options{})
}

func wmfToSVGWithOptions(data []byte, opts Options) ([]byte, float64, float64, bool) {
	off := 0
	if len(data) >= 22 && binary.LittleEndian.Uint32(data[:4]) == 0x9ac6cdd7 {
		off = 22
	}
	if len(data) < off+18 || binary.LittleEndian.Uint16(data[off+2:off+4]) != 9 {
		return nil, 0, 0, false
	}
	objects := make([]*wmfObject, int(binary.LittleEndian.Uint16(data[off+10:off+12])))
	if len(objects) == 0 {
		objects = make([]*wmfObject, 32)
	}
	type wmfState struct {
		windowX, windowY     float64
		windowW, windowH     float64
		viewportX, viewportY float64
		viewportW, viewportH float64
		viewportExtSet       bool
		mapMode              int16
		pen, brush, font     wmfObject
		textColor            string
		bkColor              string
		transparent          bool
		polyFillMode         uint16
		textAlign            uint16
		textCharExtra        int16
		currentX, currentY   float64
	}
	state := wmfState{
		mapMode:      1, // MM_TEXT
		pen:          wmfObject{kind: "pen", color: "#000000", width: 1, cosmetic: true},
		brush:        wmfObject{kind: "brush", null: true},
		font:         wmfObject{kind: "font", fontFamily: "sans-serif", fontSize: 12, fontWeight: "400"},
		textColor:    "#000000",
		bkColor:      "#ffffff",
		transparent:  true,
		polyFillMode: 1,
	}
	var stack []wmfState
	baseW, baseH := 0.0, 0.0
	baseReflectX, baseReflectY := false, false
	baseCaptured := false
	fontCatalog := wmfFontCatalog(opts.Fonts)
	var body strings.Builder
	created := false
	boundsSet := false
	minX, minY := 0.0, 0.0
	maxX, maxY := 0.0, 0.0
	includePoint := func(x, y float64) {
		if !boundsSet {
			minX, maxX = x, x
			minY, maxY = y, y
			boundsSet = true
			return
		}
		minX = math.Min(minX, x)
		minY = math.Min(minY, y)
		maxX = math.Max(maxX, x)
		maxY = math.Max(maxY, y)
	}
	includeRect := func(x0, y0, x1, y1 float64) {
		includePoint(math.Min(x0, x1), math.Min(y0, y1))
		includePoint(math.Max(x0, x1), math.Max(y0, y1))
	}
	includeText := func(x, y, fontSize float64, text string, font wmfObject, anchor, baseline string, letterSpacing float64) {
		if len([]rune(text)) == 0 {
			return
		}
		advance := wmfTextAdvance(fontCatalog, text, font, fontSize, letterSpacing)
		x0, x1 := x, x+advance
		switch anchor {
		case "middle":
			x0, x1 = x-advance/2, x+advance/2
		case "end":
			x0, x1 = x-advance, x
		}
		top, bottom := y-fontSize*0.8, y+fontSize*0.2
		switch baseline {
		case "text-before-edge", "hanging":
			top, bottom = y, y+fontSize
		case "middle", "central":
			top, bottom = y-fontSize*0.5, y+fontSize*0.5
		case "text-after-edge":
			top, bottom = y-fontSize, y
		}
		includeRect(x0, top, x1, bottom)
	}
	alloc := func(obj *wmfObject) {
		for i := range objects {
			if objects[i] == nil {
				objects[i] = obj
				return
			}
		}
		objects = append(objects, obj)
	}
	ensureViewport := func() {
		if state.viewportW == 0 && state.windowW != 0 {
			state.viewportW = state.windowW
		}
		if state.viewportH == 0 && state.windowH != 0 {
			state.viewportH = state.windowH
		}
	}
	mapPoint := func(x, y float64) (float64, float64) {
		ww, wh := state.windowW, state.windowH
		if ww == 0 {
			ww = 1
		}
		if wh == 0 {
			wh = 1
		}
		vw, vh := state.viewportW, state.viewportH
		if vw == 0 {
			vw = ww
		}
		if vh == 0 {
			vh = wh
		}
		return state.viewportX + (x-state.windowX)*vw/ww,
			state.viewportY + (y-state.windowY)*vh/wh
	}
	mapScale := func() (float64, float64) {
		ww, wh := state.windowW, state.windowH
		if ww == 0 || wh == 0 {
			return 1, 1
		}
		vw, vh := state.viewportW, state.viewportH
		if vw == 0 {
			vw = ww
		}
		if vh == 0 {
			vh = wh
		}
		return math.Abs(vw / ww), math.Abs(vh / wh)
	}
	penStroke := func() (float64, string) {
		if state.pen.cosmetic {
			// A zero-width WMF LOGPEN is a one-device-pixel cosmetic pen.
			// Keep it one CSS/device pixel when the SVG viewBox is scaled to the
			// slide; treating it as one logical unit makes Office table/grid lines
			// collapse to sub-pixel hairlines.
			return 1, ` vector-effect="non-scaling-stroke"`
		}
		sx, _ := mapScale()
		return math.Max(1, state.pen.width*sx), ""
	}
	off += 18
	for off+6 <= len(data) {
		words := int(binary.LittleEndian.Uint32(data[off : off+4]))
		fn := binary.LittleEndian.Uint16(data[off+4 : off+6])
		if words < 3 || off+words*2 > len(data) {
			break
		}
		params := data[off+6 : off+words*2]
		switch fn {
		case 0x001e: // META_SAVEDC
			stack = append(stack, state)
		case 0x0127: // META_RESTOREDC
			if len(stack) > 0 {
				state = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case 0x0103: // META_SETMAPMODE
			if len(params) >= 2 {
				state.mapMode = int16(binary.LittleEndian.Uint16(params[:2]))
			}
		case 0x020b: // META_SETWINDOWORG: y, x
			if len(params) >= 4 {
				state.windowY = float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				state.windowX = float64(int16(binary.LittleEndian.Uint16(params[2:4])))
			}
		case 0x020c: // META_SETWINDOWEXT: y, x
			if len(params) >= 4 {
				state.windowH = float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				state.windowW = float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				if !baseCaptured && state.windowW != 0 && state.windowH != 0 {
					baseCaptured = true
					baseW, baseH = math.Abs(state.windowW), math.Abs(state.windowH)
					// A metafile playback DC starts with a viewport matching the target
					// surface.  Using the first authored window extent as that target
					// makes later ScaleViewportExt/SaveDC/RestoreDC records meaningful
					// instead of collapsing every local chart into one logical box.
					state.viewportW, state.viewportH = state.windowW, state.windowH
					state.viewportExtSet = true
					baseReflectX = state.windowW < 0
					baseReflectY = state.windowH < 0
				}
			}
		case 0x020d: // META_SETVIEWPORTORG: y, x
			if len(params) >= 4 {
				state.viewportY = float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				state.viewportX = float64(int16(binary.LittleEndian.Uint16(params[2:4])))
			}
		case 0x020e: // META_SETVIEWPORTEXT: y, x
			if len(params) >= 4 {
				state.viewportH = float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				state.viewportW = float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				state.viewportExtSet = state.viewportW != 0 && state.viewportH != 0
			}
		case 0x0410: // META_SCALEWINDOWEXT: yDenom,yNum,xDenom,xNum
			if len(params) >= 8 {
				yDen := float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				yNum := float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				xDen := float64(int16(binary.LittleEndian.Uint16(params[4:6])))
				xNum := float64(int16(binary.LittleEndian.Uint16(params[6:8])))
				if xDen != 0 {
					state.windowW *= xNum / xDen
				}
				if yDen != 0 {
					state.windowH *= yNum / yDen
				}
			}
		case 0x0412: // META_SCALEVIEWPORTEXT: yDenom,yNum,xDenom,xNum
			if len(params) >= 8 {
				ensureViewport()
				yDen := float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				yNum := float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				xDen := float64(int16(binary.LittleEndian.Uint16(params[4:6])))
				xNum := float64(int16(binary.LittleEndian.Uint16(params[6:8])))
				if xDen != 0 {
					state.viewportW *= xNum / xDen
				}
				if yDen != 0 {
					state.viewportH *= yNum / yDen
				}
				state.viewportExtSet = state.viewportW != 0 && state.viewportH != 0
			}
		case 0x02fa: // META_CREATEPENINDIRECT
			if len(params) >= 10 {
				style := binary.LittleEndian.Uint16(params[0:2])
				authoredWidth := int16(binary.LittleEndian.Uint16(params[2:4]))
				width := math.Abs(float64(authoredWidth))
				cosmetic := authoredWidth == 0
				if cosmetic {
					width = 1
				}
				alloc(&wmfObject{kind: "pen", color: wmfColor(params[6:10]), width: width, cosmetic: cosmetic, null: style&0x000f == 5})
			}
		case 0x02fc: // META_CREATEBRUSHINDIRECT
			if len(params) >= 8 {
				style := binary.LittleEndian.Uint16(params[0:2])
				alloc(&wmfObject{kind: "brush", color: wmfColor(params[2:6]), null: style == 1})
			}
		case 0x02fb: // META_CREATEFONTINDIRECT
			if len(params) >= 18 {
				height := math.Abs(float64(int16(binary.LittleEndian.Uint16(params[0:2]))))
				if height == 0 {
					height = 12
				}
				weight := int(binary.LittleEndian.Uint16(params[8:10]))
				face := strings.TrimRight(string(params[18:]), "\x00")
				if i := strings.IndexByte(face, 0); i >= 0 {
					face = face[:i]
				}
				if face == "" {
					face = "sans-serif"
				}
				fontWeight := "400"
				if weight >= 600 {
					fontWeight = "700"
				}
				alloc(&wmfObject{kind: "font", fontFamily: face, fontSize: height, fontWeight: fontWeight, italic: params[10] != 0, fontCharset: params[13]})
			}
		case 0x012d: // META_SELECTOBJECT
			if len(params) >= 2 {
				i := int(binary.LittleEndian.Uint16(params[:2]))
				if i >= 0 && i < len(objects) && objects[i] != nil {
					switch objects[i].kind {
					case "pen":
						state.pen = *objects[i]
					case "brush":
						state.brush = *objects[i]
					case "font":
						state.font = *objects[i]
					}
				}
			}
		case 0x01f0: // META_DELETEOBJECT
			if len(params) >= 2 {
				i := int(binary.LittleEndian.Uint16(params[:2]))
				if i >= 0 && i < len(objects) {
					objects[i] = nil
				}
			}
		case 0x0209: // META_SETTEXTCOLOR
			if len(params) >= 4 {
				state.textColor = wmfColor(params[:4])
			}
		case 0x0201: // META_SETBKCOLOR
			if len(params) >= 4 {
				state.bkColor = wmfColor(params[:4])
			}
		case 0x0102: // META_SETBKMODE
			if len(params) >= 2 {
				state.transparent = binary.LittleEndian.Uint16(params[:2]) == 1
			}
		case 0x0106: // META_SETPOLYFILLMODE
			if len(params) >= 2 {
				state.polyFillMode = binary.LittleEndian.Uint16(params[:2])
			}
		case 0x0108: // META_SETTEXTCHAREXTRA
			if len(params) >= 2 {
				state.textCharExtra = int16(binary.LittleEndian.Uint16(params[:2]))
			}
		case 0x012e: // META_SETTEXTALIGN
			if len(params) >= 2 {
				state.textAlign = binary.LittleEndian.Uint16(params[:2])
			}
		case 0x0214: // META_MOVETO
			if len(params) >= 4 {
				state.currentY = float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				state.currentX = float64(int16(binary.LittleEndian.Uint16(params[2:4])))
			}
		case 0x0213: // META_LINETO
			if len(params) >= 4 {
				y := float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				x := float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				x1, y1 := mapPoint(state.currentX, state.currentY)
				x2, y2 := mapPoint(x, y)
				includeRect(x1, y1, x2, y2)
				stroke := "none"
				if !state.pen.null {
					stroke = state.pen.color
				}
				width, vectorEffect := penStroke()
				fmt.Fprintf(&body, "<line x1=\"%.3f\" y1=\"%.3f\" x2=\"%.3f\" y2=\"%.3f\" stroke=%q stroke-width=\"%.3f\"%s/>\n", x1, y1, x2, y2, stroke, width, vectorEffect)
				state.currentX, state.currentY = x, y
				created = true
			}
		case 0x0418: // META_ELLIPSE: bottom,right,top,left
			if len(params) >= 8 {
				bottom := float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				right := float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				top := float64(int16(binary.LittleEndian.Uint16(params[4:6])))
				left := float64(int16(binary.LittleEndian.Uint16(params[6:8])))
				l, t := mapPoint(left, top)
				r, b := mapPoint(right, bottom)
				includeRect(l, t, r, b)
				fill := "none"
				if !state.brush.null {
					fill = state.brush.color
				}
				stroke := "none"
				if !state.pen.null {
					stroke = state.pen.color
				}
				width, vectorEffect := penStroke()
				fmt.Fprintf(&body, `<ellipse cx="%.3f" cy="%.3f" rx="%.3f" ry="%.3f" fill=%q stroke=%q stroke-width="%.3f"%s/>
`, (l+r)/2, (t+b)/2, math.Abs(r-l)/2, math.Abs(b-t)/2, fill, stroke, width, vectorEffect)
				created = true
			}
		case 0x041b: // META_RECTANGLE: bottom,right,top,left
			if len(params) >= 8 {
				bottom := float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				right := float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				top := float64(int16(binary.LittleEndian.Uint16(params[4:6])))
				left := float64(int16(binary.LittleEndian.Uint16(params[6:8])))
				l, t := mapPoint(left, top)
				r, b := mapPoint(right, bottom)
				includeRect(l, t, r, b)
				fill := "none"
				if !state.brush.null {
					fill = state.brush.color
				}
				stroke := "none"
				if !state.pen.null {
					stroke = state.pen.color
				}
				width, vectorEffect := penStroke()
				fmt.Fprintf(&body, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill=%q stroke=%q stroke-width="%.3f"%s/>
`, math.Min(l, r), math.Min(t, b), math.Abs(r-l), math.Abs(b-t), fill, stroke, width, vectorEffect)
				created = true
			}
		case 0x061c: // META_ROUNDRECT: ellipseH,ellipseW,bottom,right,top,left
			if len(params) >= 12 {
				eh := float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				ew := float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				bottom := float64(int16(binary.LittleEndian.Uint16(params[4:6])))
				right := float64(int16(binary.LittleEndian.Uint16(params[6:8])))
				top := float64(int16(binary.LittleEndian.Uint16(params[8:10])))
				left := float64(int16(binary.LittleEndian.Uint16(params[10:12])))
				l, t := mapPoint(left, top)
				r, b := mapPoint(right, bottom)
				includeRect(l, t, r, b)
				sx, sy := mapScale()
				fill := "none"
				if !state.brush.null {
					fill = state.brush.color
				}
				stroke := "none"
				if !state.pen.null {
					stroke = state.pen.color
				}
				width, vectorEffect := penStroke()
				fmt.Fprintf(&body, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" ry="%.3f" fill=%q stroke=%q stroke-width="%.3f"%s/>
`, math.Min(l, r), math.Min(t, b), math.Abs(r-l), math.Abs(b-t), math.Abs(ew*sx)/2, math.Abs(eh*sy)/2, fill, stroke, width, vectorEffect)
				created = true
			}
		case 0x0325: // META_POLYLINE
			if len(params) >= 2 {
				n := int(binary.LittleEndian.Uint16(params[:2]))
				if n >= 2 && len(params) >= 2+n*4 {
					var pts strings.Builder
					for i := 0; i < n; i++ {
						x := float64(int16(binary.LittleEndian.Uint16(params[2+i*4 : 4+i*4])))
						y := float64(int16(binary.LittleEndian.Uint16(params[4+i*4 : 6+i*4])))
						xd, yd := mapPoint(x, y)
						includePoint(xd, yd)
						if i > 0 {
							pts.WriteByte(' ')
						}
						fmt.Fprintf(&pts, "%.3f,%.3f", xd, yd)
					}
					stroke := "none"
					if !state.pen.null {
						stroke = state.pen.color
					}
					width, vectorEffect := penStroke()
					fmt.Fprintf(&body, `<polyline points=%q fill="none" stroke=%q stroke-width="%.3f"%s/>
`, pts.String(), stroke, width, vectorEffect)
					created = true
				}
			}
		case 0x0538: // META_POLYPOLYGON
			if len(params) >= 2 {
				polyN := int(binary.LittleEndian.Uint16(params[:2]))
				offp := 2
				if polyN > 0 && len(params) >= offp+polyN*2 {
					counts := make([]int, polyN)
					total := 0
					for i := 0; i < polyN; i++ {
						counts[i] = int(binary.LittleEndian.Uint16(params[offp+i*2 : offp+i*2+2]))
						total += counts[i]
					}
					offp += polyN * 2
					if len(params) >= offp+total*4 {
						fill := "none"
						if !state.brush.null {
							fill = state.brush.color
						}
						stroke := "none"
						if !state.pen.null {
							stroke = state.pen.color
						}
						width, vectorEffect := penStroke()
						fillRule := "nonzero"
						if state.polyFillMode == 1 {
							fillRule = "evenodd"
						}
						idx := 0
						for _, n := range counts {
							var pts strings.Builder
							for j := 0; j < n; j++ {
								x := float64(int16(binary.LittleEndian.Uint16(params[offp+(idx+j)*4 : offp+(idx+j)*4+2])))
								y := float64(int16(binary.LittleEndian.Uint16(params[offp+(idx+j)*4+2 : offp+(idx+j)*4+4])))
								xd, yd := mapPoint(x, y)
								includePoint(xd, yd)
								if j > 0 {
									pts.WriteByte(' ')
								}
								fmt.Fprintf(&pts, "%.3f,%.3f", xd, yd)
							}
							idx += n
							fmt.Fprintf(&body, `<polygon points=%q fill=%q fill-rule=%q stroke=%q stroke-width="%.3f"%s/>
`, pts.String(), fill, fillRule, stroke, width, vectorEffect)
							created = true
						}
					}
				}
			}
		case 0x0817, 0x0830, 0x081a: // META_ARC / META_CHORD / META_PIE
			if len(params) >= 16 {
				yEnd := float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				xEnd := float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				yStart := float64(int16(binary.LittleEndian.Uint16(params[4:6])))
				xStart := float64(int16(binary.LittleEndian.Uint16(params[6:8])))
				bottom := float64(int16(binary.LittleEndian.Uint16(params[8:10])))
				right := float64(int16(binary.LittleEndian.Uint16(params[10:12])))
				top := float64(int16(binary.LittleEndian.Uint16(params[12:14])))
				left := float64(int16(binary.LittleEndian.Uint16(params[14:16])))
				l, t := mapPoint(left, top)
				r, b := mapPoint(right, bottom)
				includeRect(l, t, r, b)
				xs, ys := mapPoint(xStart, yStart)
				xe, ye := mapPoint(xEnd, yEnd)
				cx, cy := (l+r)/2, (t+b)/2
				rx, ry := math.Abs(r-l)/2, math.Abs(b-t)/2
				if rx > 0 && ry > 0 {
					a0 := math.Atan2((ys-cy)/ry, (xs-cx)/rx)
					a1 := math.Atan2((ye-cy)/ry, (xe-cx)/rx)
					// Classic GDI draws Pie/Arc records counter-clockwise from the
					// start radial to the end radial. SVG's coordinate system points
					// Y downward, so the corresponding arc uses sweep=0.  Using the
					// clockwise complement (sweep=1) turns a 31% pie slice into 69%%
					// and makes exploded Office charts overlap almost completely.
					delta := a0 - a1
					for delta <= 0 {
						delta += 2 * math.Pi
					}
					large := 0
					if delta > math.Pi {
						large = 1
					}
					fill := "none"
					if fn != 0x0817 && !state.brush.null {
						fill = state.brush.color
					}
					stroke := "none"
					if !state.pen.null {
						stroke = state.pen.color
					}
					width, vectorEffect := penStroke()
					var d string
					switch fn {
					case 0x0817:
						d = fmt.Sprintf("M %.3f %.3f A %.3f %.3f 0 %d 0 %.3f %.3f", xs, ys, rx, ry, large, xe, ye)
					case 0x0830:
						d = fmt.Sprintf("M %.3f %.3f A %.3f %.3f 0 %d 0 %.3f %.3f Z", xs, ys, rx, ry, large, xe, ye)
					default:
						d = fmt.Sprintf("M %.3f %.3f L %.3f %.3f A %.3f %.3f 0 %d 0 %.3f %.3f Z", cx, cy, xs, ys, rx, ry, large, xe, ye)
					}
					fmt.Fprintf(&body, "<path d=%q fill=%q stroke=%q stroke-width=\"%.3f\"%s/>\n", d, fill, stroke, width, vectorEffect)
					created = true
				}
			}
		case 0x0a32: // META_EXTTEXTOUT
			if len(params) >= 8 {
				y := float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				x := float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				n := int(binary.LittleEndian.Uint16(params[4:6]))
				options := binary.LittleEndian.Uint16(params[6:8])
				pos := 8
				if options&0x0006 != 0 && len(params) >= 16 {
					pos = 16
				}
				if n >= 0 && pos+n <= len(params) {
					text := wmfDecodeText(params[pos:pos+n], state.font)
					if text != "" {
						xd, yd := mapPoint(x, y)
						style := "normal"
						if state.font.italic {
							style = "italic"
						}
						_, sy := mapScale()
						fontSize := math.Max(1, state.font.fontSize*sy)
						includeText(xd, yd, fontSize, text, state.font, emfTextAnchor(uint32(state.textAlign)), emfTextBaseline(uint32(state.textAlign)), 0)
						fmt.Fprintf(&body, "<text x=\"%.3f\" y=\"%.3f\" fill=%q font-family=%q font-size=\"%.3f\" font-weight=%q font-style=%q text-anchor=%q dominant-baseline=%q>%s</text>\n", xd, yd, state.textColor, state.font.fontFamily, fontSize, state.font.fontWeight, style, emfTextAnchor(uint32(state.textAlign)), emfTextBaseline(uint32(state.textAlign)), html.EscapeString(text))
						created = true
					}
				}
			}
		case 0x0f43: // META_STRETCHDIB
			if len(params) >= 22 {
				srcH := int(int16(binary.LittleEndian.Uint16(params[6:8])))
				srcW := int(int16(binary.LittleEndian.Uint16(params[8:10])))
				ySrc := int(int16(binary.LittleEndian.Uint16(params[10:12])))
				xSrc := int(int16(binary.LittleEndian.Uint16(params[12:14])))
				dh := float64(int16(binary.LittleEndian.Uint16(params[14:16])))
				dw := float64(int16(binary.LittleEndian.Uint16(params[16:18])))
				yd := float64(int16(binary.LittleEndian.Uint16(params[18:20])))
				xd := float64(int16(binary.LittleEndian.Uint16(params[20:22])))
				if img, ok := wmfDecodePackedDIB(params[22:]); ok {
					if srcW == 0 {
						srcW = img.Bounds().Dx()
					}
					if srcH == 0 {
						srcH = img.Bounds().Dy()
					}
					crop := emfCropImage(img, xSrc, ySrc, srcW, srcH)
					if crop != nil {
						x0, y0 := mapPoint(xd, yd)
						x1, y1 := mapPoint(xd+dw, yd+dh)
						includeRect(x0, y0, x1, y1)
						if uri, ok := wmfPNGURI(crop); ok {
							fmt.Fprintf(&body, `<image x="%.3f" y="%.3f" width="%.3f" height="%.3f" preserveAspectRatio="none" href="%s"/>
`, math.Min(x0, x1), math.Min(y0, y1), math.Abs(x1-x0), math.Abs(y1-y0), uri)
							created = true
						}
					}
				}
			}
		case 0x0d33: // META_SETDIBTODEV
			if len(params) >= 18 {
				h := int(int16(binary.LittleEndian.Uint16(params[10:12])))
				w := int(int16(binary.LittleEndian.Uint16(params[12:14])))
				yd := float64(int16(binary.LittleEndian.Uint16(params[14:16])))
				xd := float64(int16(binary.LittleEndian.Uint16(params[16:18])))
				if img, ok := wmfDecodePackedDIB(params[18:]); ok {
					if w == 0 {
						w = img.Bounds().Dx()
					}
					if h == 0 {
						h = img.Bounds().Dy()
					}
					crop := emfCropImage(img, 0, 0, w, h)
					if crop != nil {
						x0, y0 := mapPoint(xd, yd)
						x1, y1 := mapPoint(xd+float64(w), yd+float64(h))
						includeRect(x0, y0, x1, y1)
						if uri, ok := wmfPNGURI(crop); ok {
							fmt.Fprintf(&body, `<image x="%.3f" y="%.3f" width="%.3f" height="%.3f" preserveAspectRatio="none" href="%s"/>
`, math.Min(x0, x1), math.Min(y0, y1), math.Abs(x1-x0), math.Abs(y1-y0), uri)
							created = true
						}
					}
				}
			}
		case 0x0521: // META_TEXTOUT: string length, bytes, padded, y, x
			if len(params) >= 2 {
				n := int(binary.LittleEndian.Uint16(params[:2]))
				pos := 2
				padded := (n + 1) &^ 1
				if n >= 0 && len(params) >= pos+padded+4 {
					text := wmfDecodeText(params[pos:pos+n], state.font)
					y := float64(int16(binary.LittleEndian.Uint16(params[pos+padded : pos+padded+2])))
					x := float64(int16(binary.LittleEndian.Uint16(params[pos+padded+2 : pos+padded+4])))
					if text != "" {
						xd, yd := mapPoint(x, y)
						_, sy := mapScale()
						extra := float64(state.textCharExtra)
						attrs := ""
						if extra != 0 {
							attrs = fmt.Sprintf(` letter-spacing="%.3f"`, extra)
						}
						fontSize := math.Max(1, state.font.fontSize*sy)
						includeText(xd, yd, fontSize, text, state.font, emfTextAnchor(uint32(state.textAlign)), emfTextBaseline(uint32(state.textAlign)), extra)
						fmt.Fprintf(&body, `<text x="%.3f" y="%.3f" fill=%q font-family=%q font-size="%.3f" font-weight=%q text-anchor=%q dominant-baseline=%q%s>%s</text>
`, xd, yd, state.textColor, state.font.fontFamily, fontSize, state.font.fontWeight, emfTextAnchor(uint32(state.textAlign)), emfTextBaseline(uint32(state.textAlign)), attrs, html.EscapeString(text))
						created = true
					}
				}
			}
		case 0x041f: // META_SETPIXEL
			if len(params) >= 8 {
				color := wmfColor(params[:4])
				y := float64(int16(binary.LittleEndian.Uint16(params[4:6])))
				x := float64(int16(binary.LittleEndian.Uint16(params[6:8])))
				xd, yd := mapPoint(x, y)
				includeRect(xd, yd, xd+1, yd+1)
				fmt.Fprintf(&body, `<rect x="%.3f" y="%.3f" width="1" height="1" fill=%q/>
`, xd, yd, color)
				created = true
			}
		case 0x0324: // META_POLYGON
			if len(params) >= 2 {
				n := int(binary.LittleEndian.Uint16(params[:2]))
				if n >= 2 && len(params) >= 2+n*4 {
					var pts strings.Builder
					for i := 0; i < n; i++ {
						x := float64(int16(binary.LittleEndian.Uint16(params[2+i*4 : 4+i*4])))
						y := float64(int16(binary.LittleEndian.Uint16(params[4+i*4 : 6+i*4])))
						xd, yd := mapPoint(x, y)
						includePoint(xd, yd)
						if i > 0 {
							pts.WriteByte(' ')
						}
						fmt.Fprintf(&pts, "%g,%g", xd, yd)
					}
					fill := "none"
					if !state.brush.null {
						fill = state.brush.color
					}
					stroke := "none"
					if !state.pen.null {
						stroke = state.pen.color
					}
					width, vectorEffect := penStroke()
					fmt.Fprintf(&body, "<polygon points=%q fill=%q stroke=%q stroke-width=\"%.3f\"%s/>\n", pts.String(), fill, stroke, width, vectorEffect)
					created = true
				}
			}
		}
		off += words * 2
		if fn == 0 {
			break
		}
	}
	if !created || !baseCaptured || baseW == 0 || baseH == 0 {
		return nil, 0, 0, false
	}
	viewMinX, viewMinY := 0.0, 0.0
	viewW, viewH := baseW, baseH
	if boundsSet && !baseReflectX && !baseReflectY {
		pad := 1.0
		viewMinX = math.Max(0, minX-pad)
		viewMinY = math.Max(0, minY-pad)
		viewW = math.Min(baseW, maxX+pad) - viewMinX
		viewH = math.Min(baseH, maxY+pad) - viewMinY
		if viewW <= 0 {
			viewMinX, viewW = 0, baseW
		}
		if viewH <= 0 {
			viewMinY, viewH = 0, baseH
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"%.3f %.3f %.3f %.3f\" width=\"100%%\" height=\"100%%\">\n", viewMinX, viewMinY, viewW, viewH)
	if baseReflectX && baseReflectY {
		out.WriteString("<g transform=\"scale(-1,-1)\">\n")
	} else if baseReflectX {
		out.WriteString("<g transform=\"scale(-1,1)\">\n")
	} else if baseReflectY {
		out.WriteString("<g transform=\"scale(1,-1)\">\n")
	}
	out.WriteString(body.String())
	if baseReflectX || baseReflectY {
		out.WriteString("</g>\n")
	}
	out.WriteString("</svg>")
	return []byte(out.String()), baseW, baseH, true
}

func wmfDecodePackedDIB(data []byte) (image.Image, bool) {
	if len(data) < 40 {
		return nil, false
	}
	hs := int(binary.LittleEndian.Uint32(data[:4]))
	if hs < 40 || hs > len(data) {
		return nil, false
	}
	bpp := int(binary.LittleEndian.Uint16(data[14:16]))
	colors := int(binary.LittleEndian.Uint32(data[32:36]))
	if colors == 0 && bpp <= 8 {
		colors = 1 << bpp
	}
	off := hs + colors*4
	compression := binary.LittleEndian.Uint32(data[16:20])
	if (compression == 3 || compression == 6) && hs == 40 {
		if compression == 6 {
			off += 16
		} else {
			off += 12
		}
	}
	if off < hs || off > len(data) {
		return nil, false
	}
	return emfDecodeDIB(data[:off], data[off:])
}
func wmfPNGURI(img image.Image) (string, bool) {
	var b bytes.Buffer
	if img == nil || png.Encode(&b, img) != nil {
		return "", false
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes()), true
}

func wmfColor(data []byte) string {
	if len(data) < 4 {
		return "#000000"
	}
	return fmt.Sprintf("#%02X%02X%02X", data[0], data[1], data[2])
}

func wmfDecodeText(data []byte, font wmfObject) string {
	if len(data) == 0 {
		return ""
	}
	trimmed := bytes.TrimRight(data, "\x00")
	if len(trimmed) == 0 {
		return ""
	}
	family := strings.ToLower(strings.TrimSpace(font.fontFamily))
	if font.fontCharset == 2 || strings.Contains(family, "symbol") {
		var out strings.Builder
		for _, b := range trimmed {
			if mapped, ok := emfLegacySymbolRune(b); ok {
				out.WriteRune(mapped)
			} else {
				out.WriteByte(b)
			}
		}
		return out.String()
	}
	decoded, err := charmap.Windows1252.NewDecoder().Bytes(trimmed)
	if err == nil {
		return string(decoded)
	}
	return string(trimmed)
}

func emfToSVG(data []byte) ([]byte, float64, float64, bool) {
	return emfToSVGWithOptions(data, Options{})
}

func emfToSVGWithOptions(data []byte, opts Options) ([]byte, float64, float64, bool) {
	if len(data) < 108 || binary.LittleEndian.Uint32(data[0:4]) != emrHeader {
		return nil, 0, 0, false
	}
	left := int32(binary.LittleEndian.Uint32(data[8:12]))
	top := int32(binary.LittleEndian.Uint32(data[12:16]))
	right := int32(binary.LittleEndian.Uint32(data[16:20]))
	bottom := int32(binary.LittleEndian.Uint32(data[20:24]))
	width := float64(right - left)
	height := float64(bottom - top)
	if width <= 0 || height <= 0 {
		return nil, 0, 0, false
	}

	state := emfState{textColor: "#000000", bkColor: "#ffffff", transparent: true, penColor: "#000000", penWidth: 1, brushColor: "#ffffff", brushPaint: "#ffffff", brushNull: true, fontFamily: "Arial", fontSize: 12, windowExtX: width, windowExtY: height, viewportExtX: width, viewportExtY: height, arcDirection: 1, polyFillMode: 1, world: emfIdentityTransform(), deviceOrgX: float64(left), deviceOrgY: float64(top)}
	objects := map[uint32]emfObject{}
	stack := []emfState{}
	var body strings.Builder
	var path emfPathState
	var openClipGroups []string
	clipElements := map[string]string{}
	clipSerial := 0
	brushSerial := 0
	gradientSerial := 0
	suppressWorksheetGrid := emfLooksLikeTextGrid(data)
	plus := emfPlusFromEMF(data, opts)

	for off := 0; off+8 <= len(data); {
		recordType := binary.LittleEndian.Uint32(data[off : off+4])
		recordSize := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recordSize < 8 || off+recordSize > len(data) {
			break
		}
		record := data[off : off+recordSize]
		switch recordType {
		case emrSetMapperFlags:
			if len(record) >= 12 {
				state.mapperFlags = binary.LittleEndian.Uint32(record[8:12])
			}
		case emrSetMapMode:
			if len(record) >= 12 {
				state.mapMode = int32(binary.LittleEndian.Uint32(record[8:12]))
			}
		case emrSetWindowExtEx:
			if len(record) >= 16 {
				state.windowExtX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.windowExtY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
			}
		case emrSetWindowOrgEx:
			if len(record) >= 16 {
				state.windowOrgX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.windowOrgY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
			}
		case emrSetViewportExtEx:
			if len(record) >= 16 {
				state.viewportExtX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.viewportExtY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
			}
		case emrSetViewportOrgEx:
			if len(record) >= 16 {
				state.viewportOrgX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.viewportOrgY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
			}
		case emrSetBrushOrgEx:
			if len(record) >= 16 {
				state.brushOrgX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.brushOrgY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
				emfActivateSpecialBrush(&body, &openClipGroups, &state, &brushSerial)
			}
		case emrSetBkMode:
			if len(record) >= 12 {
				state.transparent = binary.LittleEndian.Uint32(record[8:12]) == 1
			}
		case emrSetPolyFillMode:
			if len(record) >= 12 {
				state.polyFillMode = binary.LittleEndian.Uint32(record[8:12])
			}
		case emrSetTextAlign:
			if len(record) >= 12 {
				state.textAlign = binary.LittleEndian.Uint32(record[8:12])
			}
		case emrSetLayout:
			if len(record) >= 12 {
				state.layoutMode = binary.LittleEndian.Uint32(record[8:12])
			}
		case emrSetTextJustification:
			if len(record) >= 16 {
				state.textJustExtra = int32(binary.LittleEndian.Uint32(record[8:12]))
				state.textJustCount = int32(binary.LittleEndian.Uint32(record[12:16]))
			}
		case emrSetTextColor:
			if len(record) >= 12 {
				state.textColor = emfColor(binary.LittleEndian.Uint32(record[8:12]))
			}
		case emrSetBkColor:
			if len(record) >= 12 {
				state.bkColor = emfColor(binary.LittleEndian.Uint32(record[8:12]))
			}
		case emrSetPixelV:
			if len(record) >= 20 {
				x, y := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				fmt.Fprintf(&body, `<rect x="%.3f" y="%.3f" width="1" height="1" fill="%s"/>`, x, y, emfColor(binary.LittleEndian.Uint32(record[16:20])))
			}
		case emrSaveDC:
			saved := state
			saved.clipChain = emfCloneClipChain(state.clipChain)
			saved.clipRects = emfCloneRects(state.clipRects)
			stack = append(stack, saved)
		case emrRestoreDC:
			if len(stack) > 0 {
				state = stack[len(stack)-1]
				state.clipChain = emfCloneClipChain(state.clipChain)
				state.clipRects = emfCloneRects(state.clipRects)
				stack = stack[:len(stack)-1]
				emfSyncClipGroups(&body, &openClipGroups, state.clipChain)
			}
		case emrSetWorldTransform:
			if transform, ok := emfTransformFromRecord(record, 8); ok {
				state.world = transform
			}
		case emrModifyWorldTransform:
			if transform, ok := emfTransformFromRecord(record, 8); ok && len(record) >= 36 {
				switch binary.LittleEndian.Uint32(record[32:36]) {
				case 1: // MWT_IDENTITY
					state.world = emfIdentityTransform()
				case 2: // MWT_LEFTMULTIPLY
					state.world = emfMultiplyTransform(transform, state.world)
				case 3: // MWT_RIGHTMULTIPLY
					state.world = emfMultiplyTransform(state.world, transform)
				}
			}
		case emrCreatePen:
			if len(record) >= 28 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				style := binary.LittleEndian.Uint32(record[12:16])
				penWidth := math.Abs(float64(int32(binary.LittleEndian.Uint32(record[16:20]))))
				if penWidth <= 0 {
					penWidth = 1
				}
				objects[handle] = emfObject{kind: "pen", width: penWidth, color: emfColor(binary.LittleEndian.Uint32(record[24:28])), null: style&0x0000000f == 5}
			}
		case emrExtCreatePen:
			// EMR_EXTCREATEPEN carries a LogPenEx after the fixed bitmap-offset
			// fields. Office chart previews use these pens heavily; ignoring the
			// record leaves subsequent SELECTOBJECT calls unresolved and causes
			// series outlines to inherit whichever pen happened to be selected
			// previously.
			if len(record) >= 52 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				style := binary.LittleEndian.Uint32(record[28:32])
				penWidth := math.Abs(float64(int32(binary.LittleEndian.Uint32(record[32:36]))))
				if penWidth <= 0 {
					penWidth = 1
				}
				objects[handle] = emfObject{kind: "pen", width: penWidth, color: emfColor(binary.LittleEndian.Uint32(record[40:44])), null: style&0x0000000f == 5}
			}
		case emrCreateBrushIndirect:
			if len(record) >= 24 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				style := binary.LittleEndian.Uint32(record[12:16])
				objects[handle] = emfObject{kind: "brush", color: emfColor(binary.LittleEndian.Uint32(record[16:20])), brushStyle: style, hatch: binary.LittleEndian.Uint32(record[20:24]), null: style == 1}
			}
		case emrCreateMonoBrush, emrCreateDIBPatternBrushPt:
			if len(record) >= 32 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				offBMI, cbBMI := binary.LittleEndian.Uint32(record[16:20]), binary.LittleEndian.Uint32(record[20:24])
				offBits, cbBits := binary.LittleEndian.Uint32(record[24:28]), binary.LittleEndian.Uint32(record[28:32])
				if uri, pw, ph, ok := emfDIBDataURI(record, offBMI, cbBMI, offBits, cbBits); ok {
					objects[handle] = emfObject{kind: "brush", brushStyle: 6, patternURI: uri, patternW: pw, patternH: ph}
				}
			}
		case emrSetDIBitsToDevice:
			if len(record) >= 76 {
				xDest := int32(binary.LittleEndian.Uint32(record[24:28]))
				yDest := int32(binary.LittleEndian.Uint32(record[28:32]))
				xSrc := int32(binary.LittleEndian.Uint32(record[32:36]))
				ySrc := int32(binary.LittleEndian.Uint32(record[36:40]))
				cxSrc := int32(binary.LittleEndian.Uint32(record[40:44]))
				cySrc := int32(binary.LittleEndian.Uint32(record[44:48]))
				offBMI := binary.LittleEndian.Uint32(record[48:52])
				cbBMI := binary.LittleEndian.Uint32(record[52:56])
				offBits := binary.LittleEndian.Uint32(record[56:60])
				cbBits := binary.LittleEndian.Uint32(record[60:64])
				cScans := int32(binary.LittleEndian.Uint32(record[72:76]))
				if cScans == 0 {
					cScans = cySrc
				}
				if emfRenderDIB(&body, record, state, width, height, xDest, yDest, cxSrc, cScans, xSrc, ySrc, cxSrc, cScans, offBMI, cbBMI, offBits, cbBits) {
					// DIB playback emits a renderer-owned raster primitive; the surrounding
					// presentation image remains a single SVG artifact.
				}
			}
		case emrStretchDIBits:
			if len(record) >= 80 && binary.LittleEndian.Uint32(record[68:72]) == emfROPSourceCopy {
				emfRenderDIB(&body, record, state, width, height,
					int32(binary.LittleEndian.Uint32(record[24:28])), int32(binary.LittleEndian.Uint32(record[28:32])),
					int32(binary.LittleEndian.Uint32(record[72:76])), int32(binary.LittleEndian.Uint32(record[76:80])),
					int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])),
					int32(binary.LittleEndian.Uint32(record[40:44])), int32(binary.LittleEndian.Uint32(record[44:48])),
					binary.LittleEndian.Uint32(record[48:52]), binary.LittleEndian.Uint32(record[52:56]),
					binary.LittleEndian.Uint32(record[56:60]), binary.LittleEndian.Uint32(record[60:64]))
			}
		case emrBitBlt:
			if len(record) >= 100 && binary.LittleEndian.Uint32(record[40:44]) == emfROPSourceCopy {
				emfRenderDIB(&body, record, state, width, height,
					int32(binary.LittleEndian.Uint32(record[24:28])), int32(binary.LittleEndian.Uint32(record[28:32])),
					int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])),
					int32(binary.LittleEndian.Uint32(record[44:48])), int32(binary.LittleEndian.Uint32(record[48:52])),
					int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])),
					binary.LittleEndian.Uint32(record[84:88]), binary.LittleEndian.Uint32(record[88:92]),
					binary.LittleEndian.Uint32(record[92:96]), binary.LittleEndian.Uint32(record[96:100]))
			}
		case emrStretchBlt:
			if len(record) >= 108 && binary.LittleEndian.Uint32(record[40:44]) == emfROPSourceCopy {
				emfRenderDIB(&body, record, state, width, height,
					int32(binary.LittleEndian.Uint32(record[24:28])), int32(binary.LittleEndian.Uint32(record[28:32])),
					int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])),
					int32(binary.LittleEndian.Uint32(record[44:48])), int32(binary.LittleEndian.Uint32(record[48:52])),
					int32(binary.LittleEndian.Uint32(record[100:104])), int32(binary.LittleEndian.Uint32(record[104:108])),
					binary.LittleEndian.Uint32(record[84:88]), binary.LittleEndian.Uint32(record[88:92]),
					binary.LittleEndian.Uint32(record[92:96]), binary.LittleEndian.Uint32(record[96:100]))
			}
		case emrMaskBlt:
			// MASKBLT can encode arbitrary ternary ROPs. Materialize only the
			// source-copy subset; when a monochrome mask is present, use it as
			// per-pixel alpha rather than pretending the other ROP branches are SVG.
			if len(record) >= 128 && emfMaskBltSourceCopy(binary.LittleEndian.Uint32(record[40:44])) {
				emfRenderMaskedDIB(&body, record, state, width, height,
					int32(binary.LittleEndian.Uint32(record[24:28])), int32(binary.LittleEndian.Uint32(record[28:32])),
					int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])),
					int32(binary.LittleEndian.Uint32(record[44:48])), int32(binary.LittleEndian.Uint32(record[48:52])),
					int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])),
					binary.LittleEndian.Uint32(record[84:88]), binary.LittleEndian.Uint32(record[88:92]),
					binary.LittleEndian.Uint32(record[92:96]), binary.LittleEndian.Uint32(record[96:100]),
					int32(binary.LittleEndian.Uint32(record[100:104])), int32(binary.LittleEndian.Uint32(record[104:108])),
					binary.LittleEndian.Uint32(record[112:116]), binary.LittleEndian.Uint32(record[116:120]),
					binary.LittleEndian.Uint32(record[120:124]), binary.LittleEndian.Uint32(record[124:128]))
			}
		case emrPlgBlt:
			if len(record) >= 140 {
				emfRenderPlgDIB(&body, record, state, width, height,
					[3][2]int32{
						{int32(binary.LittleEndian.Uint32(record[24:28])), int32(binary.LittleEndian.Uint32(record[28:32]))},
						{int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40]))},
						{int32(binary.LittleEndian.Uint32(record[40:44])), int32(binary.LittleEndian.Uint32(record[44:48]))},
					},
					int32(binary.LittleEndian.Uint32(record[48:52])), int32(binary.LittleEndian.Uint32(record[52:56])),
					int32(binary.LittleEndian.Uint32(record[56:60])), int32(binary.LittleEndian.Uint32(record[60:64])),
					binary.LittleEndian.Uint32(record[96:100]), binary.LittleEndian.Uint32(record[100:104]),
					binary.LittleEndian.Uint32(record[104:108]), binary.LittleEndian.Uint32(record[108:112]))
			}
		case emrExtCreateFontIndirectW:
			if len(record) >= 104 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				fontBase := 12
				fontHeight := math.Abs(float64(int32(binary.LittleEndian.Uint32(record[fontBase : fontBase+4]))))
				if fontHeight <= 0 {
					fontHeight = 12
				}
				escapement := int32(binary.LittleEndian.Uint32(record[fontBase+8 : fontBase+12]))
				orientation := int32(binary.LittleEndian.Uint32(record[fontBase+12 : fontBase+16]))
				weight := int32(binary.LittleEndian.Uint32(record[fontBase+16 : fontBase+20]))
				family := utf16FixedString(record[fontBase+28:], 32)
				if family == "" {
					family = "Arial"
				}
				fontWeight := "normal"
				if weight >= 600 {
					fontWeight = "bold"
				}
				objects[handle] = emfObject{
					kind: "font", fontFamily: family, fontSize: fontHeight, fontWeight: fontWeight,
					italic: record[fontBase+20] != 0, underline: record[fontBase+21] != 0, strikeout: record[fontBase+22] != 0,
					escapement: escapement, orientation: orientation,
				}
			}
		case emrSelectObject:
			if len(record) >= 12 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				if handle&0x80000000 != 0 {
					applyEMFStockObject(&state, handle&0x7fffffff)
				} else if object, ok := objects[handle]; ok {
					switch object.kind {
					case "pen":
						state.penColor, state.penWidth, state.penNull = object.color, object.width, object.null
					case "brush":
						state.brushColor, state.brushNull = object.color, object.null
						state.brushPaint = object.color
						state.brushStyle, state.brushHatch = object.brushStyle, object.hatch
						state.brushPatternURI, state.brushPatternW, state.brushPatternH = object.patternURI, object.patternW, object.patternH
						emfActivateSpecialBrush(&body, &openClipGroups, &state, &brushSerial)
					case "font":
						state.fontFamily, state.fontSize = object.fontFamily, object.fontSize
						state.fontWeight, state.fontItalic = object.fontWeight, object.italic
						state.fontUnderline, state.fontStrikeout = object.underline, object.strikeout
						state.fontEscapement, state.fontOrientation = object.escapement, object.orientation
					}
				}
			}
		case emrDeleteObject:
			if len(record) >= 12 {
				delete(objects, binary.LittleEndian.Uint32(record[8:12]))
			}
		case emrBeginPath:
			path.reset(true)
		case emrEndPath:
			path.active = false
		case emrAbortPath:
			path.reset(false)
		case emrOffsetClipRgn:
			if len(record) >= 16 && len(state.clipChain) > 0 {
				x0, y0 := emfPoint(state, 0, 0, width, height)
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				dx, dy := x1-x0, y1-y0
				if state.clipRectsKnown {
					for i := range state.clipRects {
						state.clipRects[i].x1 += dx
						state.clipRects[i].x2 += dx
						state.clipRects[i].y1 += dy
						state.clipRects[i].y2 += dy
					}
					clipSerial++
					id := fmt.Sprintf("emf-clip-%d", clipSerial)
					element := emfRectsElement(state.clipRects)
					emfDefineClipPath(&body, &openClipGroups, id, element)
					clipElements[id] = element
					state.clipChain = []string{id}
				} else {
					next := make([]string, 0, len(state.clipChain))
					for _, oldID := range state.clipChain {
						element, ok := clipElements[oldID]
						if !ok {
							continue
						}
						clipSerial++
						id := fmt.Sprintf("emf-clip-%d", clipSerial)
						shifted := fmt.Sprintf(`<g transform="translate(%.3f %.3f)">%s</g>`, dx, dy, element)
						emfDefineClipPath(&body, &openClipGroups, id, shifted)
						clipElements[id] = shifted
						next = append(next, id)
					}
					state.clipChain = next
				}
				emfSyncClipGroups(&body, &openClipGroups, state.clipChain)
			}
		case emrCloseFigure:
			if path.active && !path.empty() {
				path.command("Z")
			}
		case emrMoveToEx:
			if len(record) >= 16 {
				state.currentX, state.currentY = emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				if path.active {
					path.command("M %.3f %.3f", state.currentX, state.currentY)
				}
			}
		case emrLineTo:
			if len(record) >= 16 {
				x, y := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				if path.active {
					path.command("L %.3f %.3f", x, y)
				} else {
					stroke, strokeWidth := emfStroke(state)
					if stroke != "none" && !(suppressWorksheetGrid && strings.EqualFold(stroke, "#c0c0c0")) {
						fmt.Fprintf(&body, `<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" stroke="%s" stroke-width="%.3f"/>`, state.currentX, state.currentY, x, y, stroke, strokeWidth)
					}
				}
				state.currentX, state.currentY = x, y
			}
		case emrPolyBezier, emrPolygon, emrPolyline, emrPolyBezierTo, emrPolylineTo:
			if len(record) >= 28 {
				count := int(binary.LittleEndian.Uint32(record[24:28]))
				if count > 0 && 28+count*8 <= len(record) {
					points := make([][2]float64, 0, count)
					for i := 0; i < count; i++ {
						xRaw := int32(binary.LittleEndian.Uint32(record[28+i*8 : 32+i*8]))
						yRaw := int32(binary.LittleEndian.Uint32(record[32+i*8 : 36+i*8]))
						x, y := emfPoint(state, xRaw, yRaw, width, height)
						points = append(points, [2]float64{x, y})
					}
					emfConsumePathPoints(&body, &path, &state, recordType, points, suppressWorksheetGrid)
				}
			}
		case emrPolyBezier16, emrPolyBezierTo16, emrPolylineTo16:
			if len(record) >= 28 {
				count := int(binary.LittleEndian.Uint32(record[24:28]))
				if count > 0 && 28+count*4 <= len(record) {
					points := make([][2]float64, 0, count)
					for i := 0; i < count; i++ {
						xRaw := int32(int16(binary.LittleEndian.Uint16(record[28+i*4 : 30+i*4])))
						yRaw := int32(int16(binary.LittleEndian.Uint16(record[30+i*4 : 32+i*4])))
						x, y := emfPoint(state, xRaw, yRaw, width, height)
						points = append(points, [2]float64{x, y})
					}
					emfConsumePathPoints(&body, &path, &state, recordType, points, suppressWorksheetGrid)
				}
			}
		case emrPolyPolyline, emrPolyPolygon:
			if groups, ok := emfPolyPolyPoints(record, state, width, height, false); ok {
				emfRenderPolyGroups(&body, &path, &state, groups, recordType == emrPolyPolygon, suppressWorksheetGrid)
			}
		case emrPolyPolyline16, emrPolyPolygon16:
			if groups, ok := emfPolyPolyPoints(record, state, width, height, true); ok {
				emfRenderPolyGroups(&body, &path, &state, groups, recordType == emrPolyPolygon16, suppressWorksheetGrid)
			}
		case emrPolygon16:
			if len(record) >= 28 {
				count := int(binary.LittleEndian.Uint32(record[24:28]))
				if count >= 3 && 28+count*4 <= len(record) {
					var points strings.Builder
					for i := 0; i < count; i++ {
						xRaw := int32(int16(binary.LittleEndian.Uint16(record[28+i*4 : 30+i*4])))
						yRaw := int32(int16(binary.LittleEndian.Uint16(record[30+i*4 : 32+i*4])))
						x, y := emfPoint(state, xRaw, yRaw, width, height)
						if i > 0 {
							points.WriteByte(' ')
						}
						fmt.Fprintf(&points, "%.3f,%.3f", x, y)
					}
					fill := "none"
					if !state.brushNull {
						fill = emfBrushFill(state)
					}
					stroke, strokeWidth := emfStroke(state)
					if suppressWorksheetGrid && strings.EqualFold(stroke, "#c0c0c0") {
						stroke = "none"
					}
					fmt.Fprintf(&body, `<polygon points="%s" fill="%s" stroke="%s" stroke-width="%.3f"/>`, points.String(), fill, stroke, strokeWidth)
				}
			}
		case emrPolyline16:
			if len(record) >= 28 {
				count := int(binary.LittleEndian.Uint32(record[24:28]))
				if count >= 2 && 28+count*4 <= len(record) {
					var points strings.Builder
					for i := 0; i < count; i++ {
						xRaw := int32(int16(binary.LittleEndian.Uint16(record[28+i*4 : 30+i*4])))
						yRaw := int32(int16(binary.LittleEndian.Uint16(record[30+i*4 : 32+i*4])))
						x, y := emfPoint(state, xRaw, yRaw, width, height)
						if i > 0 {
							points.WriteByte(' ')
						}
						fmt.Fprintf(&points, "%.3f,%.3f", x, y)
					}
					// EMR_POLYLINE16 is an open stroke. Never close it or apply the
					// selected brush: doing so creates phantom triangular fills.
					stroke, strokeWidth := emfStroke(state)
					if stroke != "none" {
						fmt.Fprintf(&body, `<polyline points="%s" fill="none" stroke="%s" stroke-width="%.3f"/>`, points.String(), stroke, strokeWidth)
					}
				}
			}
		case emrRectangle:
			if len(record) >= 24 {
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[16:20])), int32(binary.LittleEndian.Uint32(record[20:24])), width, height)
				rx, ry := math.Min(x1, x2), math.Min(y1, y2)
				rw, rh := math.Abs(x2-x1), math.Abs(y2-y1)
				fill := "none"
				if !state.brushNull {
					fill = emfBrushFill(state)
				}
				stroke, strokeWidth := emfStroke(state)
				if suppressWorksheetGrid && strings.EqualFold(stroke, "#c0c0c0") {
					stroke = "none"
				}
				fmt.Fprintf(&body, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="%s" stroke="%s" stroke-width="%.3f"/>`, rx, ry, rw, rh, fill, stroke, strokeWidth)
			}
		case emrRoundRect:
			if len(record) >= 32 {
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[16:20])), int32(binary.LittleEndian.Uint32(record[20:24])), width, height)
				rx, ry := math.Min(x1, x2), math.Min(y1, y2)
				rw, rh := math.Abs(x2-x1), math.Abs(y2-y1)
				cw := math.Abs(emfMappedDeltaX(state, int32(binary.LittleEndian.Uint32(record[24:28])), width))
				ch := math.Abs(emfMappedDeltaY(state, int32(binary.LittleEndian.Uint32(record[28:32])), height))
				fill := "none"
				if !state.brushNull {
					fill = emfBrushFill(state)
				}
				stroke, strokeWidth := emfStroke(state)
				fmt.Fprintf(&body, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" ry="%.3f" fill="%s" stroke="%s" stroke-width="%.3f"/>`, rx, ry, rw, rh, math.Min(rw/2, cw/2), math.Min(rh/2, ch/2), fill, stroke, strokeWidth)
			}
		case emrEllipse:
			if len(record) >= 24 {
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[16:20])), int32(binary.LittleEndian.Uint32(record[20:24])), width, height)
				fill := "none"
				if !state.brushNull {
					fill = emfBrushFill(state)
				}
				stroke, strokeWidth := emfStroke(state)
				fmt.Fprintf(&body, `<ellipse cx="%.3f" cy="%.3f" rx="%.3f" ry="%.3f" fill="%s" stroke="%s" stroke-width="%.3f"/>`, (x1+x2)/2, (y1+y2)/2, math.Abs(x2-x1)/2, math.Abs(y2-y1)/2, fill, stroke, strokeWidth)
			}
		case emrAngleArc:
			if len(record) >= 28 {
				cx0 := int32(binary.LittleEndian.Uint32(record[8:12]))
				cy0 := int32(binary.LittleEndian.Uint32(record[12:16]))
				radius := float64(binary.LittleEndian.Uint32(record[16:20]))
				start := float64(math.Float32frombits(binary.LittleEndian.Uint32(record[20:24]))) * math.Pi / 180
				sweep := float64(math.Float32frombits(binary.LittleEndian.Uint32(record[24:28])))
				cx, cy := emfPoint(state, cx0, cy0, width, height)
				dx, _ := emfMappedVector(state, radius, 0, width, height)
				_, dy := emfMappedVector(state, 0, radius, width, height)
				rx, ry := math.Abs(dx), math.Abs(dy)
				startX := cx + rx*math.Cos(start)
				startY := cy - ry*math.Sin(start)
				endA := start + sweep*math.Pi/180
				endX := cx + rx*math.Cos(endA)
				endY := cy - ry*math.Sin(endA)
				curX, curY := state.currentX, state.currentY
				stroke, sw := emfStroke(state)
				large := 0
				if math.Abs(sweep) > 180 {
					large = 1
				}
				svgSweep := 0
				if sweep < 0 {
					svgSweep = 1
				}
				fmt.Fprintf(&body, `<path d="M %.3f %.3f L %.3f %.3f A %.3f %.3f 0 %d %d %.3f %.3f" fill="none" stroke="%s" stroke-width="%.3f"/>`, curX, curY, startX, startY, rx, ry, large, svgSweep, endX, endY, stroke, sw)
				// Current position becomes the endpoint in logical coordinates.
				state.currentX, state.currentY = endX, endY
			}
		case emrSetArcDirection:
			if len(record) >= 12 {
				state.arcDirection = binary.LittleEndian.Uint32(record[8:12])
			}
		case emrArcTo:
			if len(record) >= 40 {
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[16:20])), int32(binary.LittleEndian.Uint32(record[20:24])), width, height)
				sx, sy := emfPoint(state, int32(binary.LittleEndian.Uint32(record[24:28])), int32(binary.LittleEndian.Uint32(record[28:32])), width, height)
				ex, ey := emfPoint(state, int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])), width, height)
				cx, cy := (x1+x2)/2, (y1+y2)/2
				rx, ry := math.Abs(x2-x1)/2, math.Abs(y2-y1)/2
				if rx > 0 && ry > 0 {
					sx, sy = emfEllipseIntersection(cx, cy, rx, ry, sx, sy)
					ex, ey = emfEllipseIntersection(cx, cy, rx, ry, ex, ey)
					large, sweep := emfArcFlags(state.arcDirection, cx, cy, rx, ry, sx, sy, ex, ey)
					if path.active {
						path.command("L %.3f %.3f", sx, sy)
						path.command("A %.3f %.3f 0 %d %d %.3f %.3f", rx, ry, large, sweep, ex, ey)
					} else {
						stroke, strokeWidth := emfStroke(state)
						if stroke != "none" {
							fmt.Fprintf(&body, `<path d="M %.3f %.3f L %.3f %.3f A %.3f %.3f 0 %d %d %.3f %.3f" fill="none" stroke="%s" stroke-width="%.3f"/>`, state.currentX, state.currentY, sx, sy, rx, ry, large, sweep, ex, ey, stroke, strokeWidth)
						}
					}
					state.currentX, state.currentY = ex, ey
				}
			}
		case emrArc, emrChord, emrPie:
			if len(record) >= 40 {
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[16:20])), int32(binary.LittleEndian.Uint32(record[20:24])), width, height)
				sx, sy := emfPoint(state, int32(binary.LittleEndian.Uint32(record[24:28])), int32(binary.LittleEndian.Uint32(record[28:32])), width, height)
				ex, ey := emfPoint(state, int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])), width, height)
				cx, cy := (x1+x2)/2, (y1+y2)/2
				rx, ry := math.Abs(x2-x1)/2, math.Abs(y2-y1)/2
				if rx > 0 && ry > 0 {
					sx, sy = emfEllipseIntersection(cx, cy, rx, ry, sx, sy)
					ex, ey = emfEllipseIntersection(cx, cy, rx, ry, ex, ey)
					large, sweep := emfArcFlags(state.arcDirection, cx, cy, rx, ry, sx, sy, ex, ey)
					stroke, strokeWidth := emfStroke(state)
					fill := "none"
					if recordType != emrArc && !state.brushNull {
						fill = emfBrushFill(state)
					}
					if recordType == emrPie {
						fmt.Fprintf(&body, `<path d="M %.3f %.3f L %.3f %.3f A %.3f %.3f 0 %d %d %.3f %.3f Z" fill="%s" stroke="%s" stroke-width="%.3f"/>`, cx, cy, sx, sy, rx, ry, large, sweep, ex, ey, fill, stroke, strokeWidth)
					} else if recordType == emrChord {
						fmt.Fprintf(&body, `<path d="M %.3f %.3f A %.3f %.3f 0 %d %d %.3f %.3f Z" fill="%s" stroke="%s" stroke-width="%.3f"/>`, sx, sy, rx, ry, large, sweep, ex, ey, fill, stroke, strokeWidth)
					} else if stroke != "none" {
						fmt.Fprintf(&body, `<path d="M %.3f %.3f A %.3f %.3f 0 %d %d %.3f %.3f" fill="none" stroke="%s" stroke-width="%.3f"/>`, sx, sy, rx, ry, large, sweep, ex, ey, stroke, strokeWidth)
					}
				}
			}
		case emrExtTextOutW:
			emfRenderEMRTextObject(&body, &openClipGroups, &clipSerial, record, 36, &state, width, height)
		case emrPolyTextOutW:
			if len(record) >= 40 {
				count := int(binary.LittleEndian.Uint32(record[36:40]))
				if count >= 0 && 40+count*40 <= len(record) {
					for i := 0; i < count; i++ {
						emfRenderEMRTextObject(&body, &openClipGroups, &clipSerial, record, 40+i*40, &state, width, height)
					}
				}
			}
		case emrSmallTextOut:
			emfRenderSmallTextOut(&body, &openClipGroups, &clipSerial, record, state, width, height)
		case emrGDIComment:
			// EMF+ records live inside EMR_GDICOMMENT and are intentionally not
			// replayed as classic GDI. emfToSVG records their presence as metadata
			// so a future GDI+ decoder can take ownership without double-painting.
		case emrFillPath, emrStrokePath, emrStrokeAndFillPath:
			if !path.empty() {
				fill := "none"
				stroke := "none"
				strokeWidth := 0.0
				if recordType == emrFillPath || recordType == emrStrokeAndFillPath {
					if !state.brushNull {
						fill = emfBrushFill(state)
					}
				}
				if recordType == emrStrokePath || recordType == emrStrokeAndFillPath {
					stroke, strokeWidth = emfStroke(state)
				}
				fillRule := emfFillRule(state.polyFillMode)
				fmt.Fprintf(&body, `<path d="%s" fill="%s" fill-rule="%s" stroke="%s" stroke-width="%.3f"/>`, path.d.String(), fill, fillRule, stroke, strokeWidth)
				path.reset(false)
			}
		case emrSelectClipPath:
			if len(record) >= 12 && !path.empty() {
				mode := binary.LittleEndian.Uint32(record[8:12])
				if mode == 1 || mode == 5 { // arbitrary path clips keep exact SVG geometry
					clipSerial++
					id := fmt.Sprintf("emf-clip-%d", clipSerial)
					element := fmt.Sprintf(`<path d="%s" clip-rule="%s"/>`, path.d.String(), emfFillRule(state.polyFillMode))
					emfDefineClipPath(&body, &openClipGroups, id, element)
					clipElements[id] = element
					if mode == 5 {
						state.clipChain = []string{id}
					} else {
						state.clipChain = append(emfCloneClipChain(state.clipChain), id)
					}
					state.clipRects = nil
					state.clipRectsKnown = false
					emfSyncClipGroups(&body, &openClipGroups, state.clipChain)
				}
				path.reset(false)
			}
		case emrExtSelectClipRgn:
			if len(record) >= 16 {
				cb := int(binary.LittleEndian.Uint32(record[8:12]))
				mode := binary.LittleEndian.Uint32(record[12:16])
				if cb == 0 && mode == 5 { // RGN_COPY with NULL region resets clipping.
					state.clipChain = nil
					state.clipRects = nil
					state.clipRectsKnown = false
					emfSyncClipGroups(&body, &openClipGroups, nil)
				} else if cb > 0 && 16+cb <= len(record) {
					if incoming, ok := emfRegionRects(record[16:16+cb], state, width, height); ok {
						materialize := false
						var combined []emfRect
						switch {
						case mode == 5 || len(state.clipChain) == 0:
							combined = emfCloneRects(incoming)
							materialize = true
						case state.clipRectsKnown && mode >= 1 && mode <= 4:
							combined = emfCombineRects(state.clipRects, incoming, mode)
							materialize = true
						case mode == 1:
							// Exact intersection remains representable by nested SVG clips.
							element := emfRectsElement(incoming)
							clipSerial++
							id := fmt.Sprintf("emf-clip-%d", clipSerial)
							emfDefineClipPath(&body, &openClipGroups, id, element)
							clipElements[id] = element
							state.clipChain = append(emfCloneClipChain(state.clipChain), id)
							state.clipRectsKnown = false
							emfSyncClipGroups(&body, &openClipGroups, state.clipChain)
						}
						if materialize {
							clipSerial++
							id := fmt.Sprintf("emf-clip-%d", clipSerial)
							element := emfRectsElement(combined)
							emfDefineClipPath(&body, &openClipGroups, id, element)
							clipElements[id] = element
							state.clipChain = []string{id}
							state.clipRects = combined
							state.clipRectsKnown = true
							emfSyncClipGroups(&body, &openClipGroups, state.clipChain)
						}
					}
				}
			}
		case emrFillRgn:
			if len(record) >= 32 {
				cb := int(binary.LittleEndian.Uint32(record[24:28]))
				brush, exists := objects[binary.LittleEndian.Uint32(record[28:32])]
				if cb > 0 && 32+cb <= len(record) && exists && brush.kind == "brush" && !brush.null {
					if element, ok := emfRegionElement(record[32:32+cb], state, width, height); ok {
						fmt.Fprintf(&body, `<g fill="%s" stroke="none">%s</g>`, emfObjectBrushPaint(&body, &openClipGroups, brush, state, &brushSerial), element)
					}
				}
			}
		case emrPaintRgn:
			if len(record) >= 28 && !state.brushNull {
				cb := int(binary.LittleEndian.Uint32(record[24:28]))
				if cb > 0 && 28+cb <= len(record) {
					if element, ok := emfRegionElement(record[28:28+cb], state, width, height); ok {
						fmt.Fprintf(&body, `<g fill="%s" stroke="none">%s</g>`, emfBrushFill(state), element)
					}
				}
			}
		case emrIntersectClipRect:
			if len(record) >= 24 {
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[16:20])), int32(binary.LittleEndian.Uint32(record[20:24])), width, height)
				incoming := []emfRect{emfNormalizeRect(emfRect{x1, y1, x2, y2})}
				if len(state.clipChain) == 0 {
					state.clipRects = emfCloneRects(incoming)
					state.clipRectsKnown = true
				} else if state.clipRectsKnown {
					state.clipRects = emfCombineRects(state.clipRects, incoming, 1)
				}
				clipSerial++
				id := fmt.Sprintf("emf-clip-%d", clipSerial)
				var element string
				if state.clipRectsKnown {
					element = emfRectsElement(state.clipRects)
					state.clipChain = []string{id}
				} else {
					element = emfRectsElement(incoming)
					state.clipChain = append(emfCloneClipChain(state.clipChain), id)
				}
				emfDefineClipPath(&body, &openClipGroups, id, element)
				clipElements[id] = element
				emfSyncClipGroups(&body, &openClipGroups, state.clipChain)
			}
		case emrExcludeClipRect:
			// Region subtraction cannot be represented as a simple nested SVG clip.
			// Leave the current clip unchanged rather than clipping the wrong area.
		case emrAlphaBlend:
			if len(record) >= 108 {
				xDest := int32(binary.LittleEndian.Uint32(record[24:28]))
				yDest := int32(binary.LittleEndian.Uint32(record[28:32]))
				cxDest := int32(binary.LittleEndian.Uint32(record[32:36]))
				cyDest := int32(binary.LittleEndian.Uint32(record[36:40]))
				constant := record[42]
				perPixel := record[43]&1 != 0
				xSrc := int32(binary.LittleEndian.Uint32(record[44:48]))
				ySrc := int32(binary.LittleEndian.Uint32(record[48:52]))
				offBMI := binary.LittleEndian.Uint32(record[84:88])
				cbBMI := binary.LittleEndian.Uint32(record[88:92])
				offBits := binary.LittleEndian.Uint32(record[92:96])
				cbBits := binary.LittleEndian.Uint32(record[96:100])
				cxSrc := int32(binary.LittleEndian.Uint32(record[100:104]))
				cySrc := int32(binary.LittleEndian.Uint32(record[104:108]))
				emfRenderProcessedDIB(&body, record, state, width, height, xDest, yDest, cxDest, cyDest, xSrc, ySrc, cxSrc, cySrc, offBMI, cbBMI, offBits, cbBits, func(c color.NRGBA) color.NRGBA {
					if perPixel {
						a := uint32(c.A)
						if a > 0 && a < 255 {
							c.R = uint8(min(255, uint32(c.R)*255/a))
							c.G = uint8(min(255, uint32(c.G)*255/a))
							c.B = uint8(min(255, uint32(c.B)*255/a))
						}
						c.A = uint8(a * uint32(constant) / 255)
					} else {
						c.A = constant
					}
					return c
				})
			}
		case emrTransparentBlt:
			if len(record) >= 108 {
				xDest := int32(binary.LittleEndian.Uint32(record[24:28]))
				yDest := int32(binary.LittleEndian.Uint32(record[28:32]))
				cxDest := int32(binary.LittleEndian.Uint32(record[32:36]))
				cyDest := int32(binary.LittleEndian.Uint32(record[36:40]))
				keyRaw := binary.LittleEndian.Uint32(record[40:44])
				key := color.NRGBA{R: uint8(keyRaw), G: uint8(keyRaw >> 8), B: uint8(keyRaw >> 16), A: 255}
				xSrc := int32(binary.LittleEndian.Uint32(record[44:48]))
				ySrc := int32(binary.LittleEndian.Uint32(record[48:52]))
				offBMI := binary.LittleEndian.Uint32(record[84:88])
				cbBMI := binary.LittleEndian.Uint32(record[88:92])
				offBits := binary.LittleEndian.Uint32(record[92:96])
				cbBits := binary.LittleEndian.Uint32(record[96:100])
				cxSrc := int32(binary.LittleEndian.Uint32(record[100:104]))
				cySrc := int32(binary.LittleEndian.Uint32(record[104:108]))
				emfRenderProcessedDIB(&body, record, state, width, height, xDest, yDest, cxDest, cyDest, xSrc, ySrc, cxSrc, cySrc, offBMI, cbBMI, offBits, cbBits, func(c color.NRGBA) color.NRGBA {
					if c.R == key.R && c.G == key.G && c.B == key.B {
						c.A = 0
					}
					return c
				})
			}
		case emrGradientFill:
			if len(record) >= 36 {
				emfRenderGradientFill(&body, &openClipGroups, record, state, width, height, &gradientSerial)
			}
		case emrEOF:
			off = len(data)
			continue
		}
		off += recordSize
	}

	emfSyncClipGroups(&body, &openClipGroups, nil)

	var svg strings.Builder
	// EMF drawing coordinates after window/viewport mapping live in device space.
	// rclBounds is only the ink extent, so mapping it onto the destination box
	// stretches the drawing and moves its baseline. PowerPoint maps rclFrame, the
	// declared frame, which is what keeps an inline equation sitting on the
	// surrounding text baseline. rclBounds may still have a non-zero origin, so it
	// remains the fallback whenever no usable frame is declared.
	viewLeft, viewTop, viewW, viewH := float64(left), float64(top), width, height
	overflow := ""
	if fl, ft, fw, fh, ok := emfFrameDeviceRect(data); ok {
		viewLeft, viewTop, viewW, viewH = fl, ft, fw, fh
		// The previous ink-bounds viewBox never clipped. Keep it that way so
		// correcting placement cannot crop glyphs that reach past the frame.
		overflow = ` overflow="visible"`
	}
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%.3f %.3f %.3f %.3f" width="%.3f" height="%.3f" preserveAspectRatio="none"%s>`, viewLeft, viewTop, viewW, viewH, viewW, viewH, overflow)
	if plus.present {
		mode := "only"
		if plus.dual {
			mode = "dual"
		}
		rendered := "false"
		if plus.drawn && !plus.dual {
			rendered = "true"
		}
		fmt.Fprintf(&svg, `<metadata data-emf-plus="present" data-emf-plus-mode="%s" data-emf-plus-rendered="%s"/>`, mode, rendered)
	}
	if plus.present && plus.drawn && !plus.dual {
		if plus.defs.Len() > 0 {
			svg.WriteString(`<defs>`)
			svg.WriteString(plus.defs.String())
			svg.WriteString(`</defs>`)
		}
		svg.WriteString(plus.body.String())
	} else {
		svg.WriteString(body.String())
	}
	svg.WriteString(`</svg>`)
	return []byte(svg.String()), width, height, true
}

func emfFillRule(mode uint32) string {
	if mode == 2 { // WINDING
		return "nonzero"
	}
	return "evenodd" // ALTERNATE and zero-value fallback
}

// emfFrameDeviceRect converts the header's rclFrame (.01 mm) into the device
// space the records are mapped into. It reports false when the header does not
// carry a usable frame/device/millimetre triple.
func emfFrameDeviceRect(data []byte) (left, top, width, height float64, ok bool) {
	if len(data) < 88 {
		return 0, 0, 0, 0, false
	}
	fl := int32(binary.LittleEndian.Uint32(data[24:28]))
	ft := int32(binary.LittleEndian.Uint32(data[28:32]))
	fr := int32(binary.LittleEndian.Uint32(data[32:36]))
	fb := int32(binary.LittleEndian.Uint32(data[36:40]))
	devX := int32(binary.LittleEndian.Uint32(data[72:76]))
	devY := int32(binary.LittleEndian.Uint32(data[76:80]))
	mmX := int32(binary.LittleEndian.Uint32(data[80:84]))
	mmY := int32(binary.LittleEndian.Uint32(data[84:88]))
	if fr <= fl || fb <= ft || devX <= 0 || devY <= 0 || mmX <= 0 || mmY <= 0 {
		return 0, 0, 0, 0, false
	}
	perX := float64(devX) / (float64(mmX) * 100)
	perY := float64(devY) / (float64(mmY) * 100)
	width = float64(fr-fl) * perX
	height = float64(fb-ft) * perY
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, false
	}
	return float64(fl) * perX, float64(ft) * perY, width, height, true
}

func emfConsumePathPoints(body *strings.Builder, path *emfPathState, state *emfState, recordType uint32, points [][2]float64, suppressWorksheetGrid bool) {
	if len(points) == 0 || path == nil || state == nil {
		return
	}
	isBezier := recordType == emrPolyBezier || recordType == emrPolyBezierTo || recordType == emrPolyBezier16 || recordType == emrPolyBezierTo16
	isTo := recordType == emrPolyBezierTo || recordType == emrPolylineTo || recordType == emrPolyBezierTo16 || recordType == emrPolylineTo16
	isPolygon := recordType == emrPolygon

	if path.active {
		start := 0
		if !isTo {
			path.command("M %.3f %.3f", points[0][0], points[0][1])
			state.currentX, state.currentY = points[0][0], points[0][1]
			start = 1
		}
		if isBezier {
			for i := start; i+2 < len(points); i += 3 {
				path.command("C %.3f %.3f %.3f %.3f %.3f %.3f", points[i][0], points[i][1], points[i+1][0], points[i+1][1], points[i+2][0], points[i+2][1])
				state.currentX, state.currentY = points[i+2][0], points[i+2][1]
			}
		} else {
			for i := start; i < len(points); i++ {
				path.command("L %.3f %.3f", points[i][0], points[i][1])
				state.currentX, state.currentY = points[i][0], points[i][1]
			}
			if isPolygon {
				path.command("Z")
			}
		}
		return
	}

	// Outside BEGINPATH/ENDPATH retain ordinary immediate-mode GDI behavior.
	if isBezier {
		var d strings.Builder
		start := 0
		if isTo {
			fmt.Fprintf(&d, "M %.3f %.3f", state.currentX, state.currentY)
		} else {
			fmt.Fprintf(&d, "M %.3f %.3f", points[0][0], points[0][1])
			start = 1
		}
		for i := start; i+2 < len(points); i += 3 {
			fmt.Fprintf(&d, " C %.3f %.3f %.3f %.3f %.3f %.3f", points[i][0], points[i][1], points[i+1][0], points[i+1][1], points[i+2][0], points[i+2][1])
			state.currentX, state.currentY = points[i+2][0], points[i+2][1]
		}
		stroke, strokeWidth := emfStroke(*state)
		if stroke != "none" && !(suppressWorksheetGrid && strings.EqualFold(stroke, "#c0c0c0")) {
			fmt.Fprintf(body, `<path d="%s" fill="none" stroke="%s" stroke-width="%.3f"/>`, d.String(), stroke, strokeWidth)
		}
		return
	}

	var pts strings.Builder
	for i, point := range points {
		if i > 0 {
			pts.WriteByte(' ')
		}
		fmt.Fprintf(&pts, "%.3f,%.3f", point[0], point[1])
	}
	stroke, strokeWidth := emfStroke(*state)
	if suppressWorksheetGrid && strings.EqualFold(stroke, "#c0c0c0") {
		stroke = "none"
	}
	if isPolygon {
		fill := "none"
		if !state.brushNull {
			fill = emfBrushFill(*state)
		}
		fmt.Fprintf(body, `<polygon points="%s" fill="%s" fill-rule="%s" stroke="%s" stroke-width="%.3f"/>`, pts.String(), fill, emfFillRule(state.polyFillMode), stroke, strokeWidth)
	} else if stroke != "none" {
		fmt.Fprintf(body, `<polyline points="%s" fill="none" stroke="%s" stroke-width="%.3f"/>`, pts.String(), stroke, strokeWidth)
	}
	last := points[len(points)-1]
	state.currentX, state.currentY = last[0], last[1]
}

func emfPolyPolyPoints(record []byte, state emfState, width, height float64, short bool) ([][][2]float64, bool) {
	if len(record) < 32 {
		return nil, false
	}
	polyCount := int(binary.LittleEndian.Uint32(record[24:28]))
	pointCount := int(binary.LittleEndian.Uint32(record[28:32]))
	if polyCount <= 0 || pointCount <= 0 || 32+polyCount*4 > len(record) {
		return nil, false
	}
	counts := make([]int, polyCount)
	total := 0
	for i := 0; i < polyCount; i++ {
		counts[i] = int(binary.LittleEndian.Uint32(record[32+i*4 : 36+i*4]))
		if counts[i] < 0 || total > pointCount-counts[i] {
			return nil, false
		}
		total += counts[i]
	}
	if total != pointCount {
		return nil, false
	}
	stride := 8
	if short {
		stride = 4
	}
	base := 32 + polyCount*4
	if base+pointCount*stride > len(record) {
		return nil, false
	}
	groups := make([][][2]float64, 0, polyCount)
	pointIndex := 0
	for _, count := range counts {
		group := make([][2]float64, 0, count)
		for i := 0; i < count; i++ {
			off := base + pointIndex*stride
			var xRaw, yRaw int32
			if short {
				xRaw = int32(int16(binary.LittleEndian.Uint16(record[off : off+2])))
				yRaw = int32(int16(binary.LittleEndian.Uint16(record[off+2 : off+4])))
			} else {
				xRaw = int32(binary.LittleEndian.Uint32(record[off : off+4]))
				yRaw = int32(binary.LittleEndian.Uint32(record[off+4 : off+8]))
			}
			x, y := emfPoint(state, xRaw, yRaw, width, height)
			group = append(group, [2]float64{x, y})
			pointIndex++
		}
		groups = append(groups, group)
	}
	return groups, true
}

func emfRenderPolyGroups(body *strings.Builder, path *emfPathState, state *emfState, groups [][][2]float64, polygon, suppressWorksheetGrid bool) {
	if body == nil || path == nil || state == nil {
		return
	}
	if path.active {
		for _, group := range groups {
			if len(group) == 0 {
				continue
			}
			path.command("M %.3f %.3f", group[0][0], group[0][1])
			for _, point := range group[1:] {
				path.command("L %.3f %.3f", point[0], point[1])
			}
			if polygon {
				path.command("Z")
			}
			last := group[len(group)-1]
			state.currentX, state.currentY = last[0], last[1]
		}
		return
	}
	stroke, strokeWidth := emfStroke(*state)
	if suppressWorksheetGrid && strings.EqualFold(stroke, "#c0c0c0") {
		stroke = "none"
	}
	if polygon {
		var d strings.Builder
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			fmt.Fprintf(&d, "M %.3f %.3f", group[0][0], group[0][1])
			for _, point := range group[1:] {
				fmt.Fprintf(&d, " L %.3f %.3f", point[0], point[1])
			}
			d.WriteString(" Z ")
		}
		fill := "none"
		if !state.brushNull {
			fill = emfBrushFill(*state)
		}
		fmt.Fprintf(body, `<path d="%s" fill="%s" fill-rule="%s" stroke="%s" stroke-width="%.3f"/>`, strings.TrimSpace(d.String()), fill, emfFillRule(state.polyFillMode), stroke, strokeWidth)
	} else if stroke != "none" {
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			var points strings.Builder
			for i, point := range group {
				if i > 0 {
					points.WriteByte(' ')
				}
				fmt.Fprintf(&points, "%.3f,%.3f", point[0], point[1])
			}
			fmt.Fprintf(body, `<polyline points="%s" fill="none" stroke="%s" stroke-width="%.3f"/>`, points.String(), stroke, strokeWidth)
		}
	}
}

func emfRegionElement(data []byte, state emfState, width, height float64) (string, bool) {
	// RGNDATAHEADER is 32 bytes followed by nCount RECT structures.
	if len(data) < 32 {
		return "", false
	}
	headerSize := int(binary.LittleEndian.Uint32(data[0:4]))
	regionType := binary.LittleEndian.Uint32(data[4:8])
	count := int(binary.LittleEndian.Uint32(data[8:12]))
	if headerSize < 32 || headerSize > len(data) || regionType != 1 || count <= 0 || headerSize+count*16 > len(data) {
		return "", false
	}
	var element strings.Builder
	for i := 0; i < count; i++ {
		off := headerSize + i*16
		x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(data[off:off+4])), int32(binary.LittleEndian.Uint32(data[off+4:off+8])), width, height)
		x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(data[off+8:off+12])), int32(binary.LittleEndian.Uint32(data[off+12:off+16])), width, height)
		fmt.Fprintf(&element, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f"/>`, math.Min(x1, x2), math.Min(y1, y2), math.Abs(x2-x1), math.Abs(y2-y1))
	}
	return element.String(), element.Len() > 0
}

func emfRenderProcessedDIB(body *strings.Builder, record []byte, state emfState, width, height float64, xDest, yDest, cxDest, cyDest, xSrc, ySrc, cxSrc, cySrc int32, offBMI, cbBMI, offBits, cbBits uint32, process func(color.NRGBA) color.NRGBA) bool {
	img, bmi, ok := emfRecordDIB(record, offBMI, cbBMI, offBits, cbBits)
	if !ok {
		return false
	}
	if cxSrc == 0 {
		cxSrc = int32(img.Bounds().Dx())
	}
	if cySrc == 0 {
		cySrc = int32(img.Bounds().Dy())
	}
	if len(bmi) >= 12 && int32(binary.LittleEndian.Uint32(bmi[8:12])) > 0 {
		ySrc = int32(img.Bounds().Dy()) - ySrc - int32(math.Abs(float64(cySrc)))
	}
	crop := emfCropImage(img, int(xSrc), int(ySrc), int(cxSrc), int(cySrc))
	if crop == nil {
		return false
	}
	out := image.NewNRGBA(image.Rect(0, 0, crop.Bounds().Dx(), crop.Bounds().Dy()))
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			c := color.NRGBAModel.Convert(crop.At(crop.Bounds().Min.X+x, crop.Bounds().Min.Y+y)).(color.NRGBA)
			if process != nil {
				c = process(c)
			}
			out.SetNRGBA(x, y, c)
		}
	}
	var enc bytes.Buffer
	if png.Encode(&enc, out) != nil {
		return false
	}
	x0, y0 := emfPoint(state, xDest, yDest, width, height)
	x1, y1 := emfPoint(state, xDest+cxDest, yDest, width, height)
	x2, y2 := emfPoint(state, xDest, yDest+cyDest, width, height)
	w := float64(out.Bounds().Dx())
	h := float64(out.Bounds().Dy())
	if w <= 0 || h <= 0 {
		return false
	}
	a := (x1 - x0) / w
	b := (y1 - y0) / w
	c := (x2 - x0) / h
	d := (y2 - y0) / h
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(enc.Bytes())
	fmt.Fprintf(body, `<image x="0" y="0" width="%.3f" height="%.3f" preserveAspectRatio="none" transform="matrix(%.9f %.9f %.9f %.9f %.9f %.9f)" href="%s"/>`, w, h, a, b, c, d, x0, y0, uri)
	return true
}

func emfRenderDIB(body *strings.Builder, record []byte, state emfState, width, height float64, xDest, yDest, cxDest, cyDest, xSrc, ySrc, cxSrc, cySrc int32, offBMI, cbBMI, offBits, cbBits uint32) bool {
	if body == nil || offBMI == 0 || cbBMI == 0 || offBits == 0 || cbBits == 0 {
		return false
	}
	bmiEnd := uint64(offBMI) + uint64(cbBMI)
	bitsEnd := uint64(offBits) + uint64(cbBits)
	if bmiEnd > uint64(len(record)) || bitsEnd > uint64(len(record)) {
		return false
	}
	bmi := record[offBMI:bmiEnd]
	img, ok := emfDecodeDIB(bmi, record[offBits:bitsEnd])
	if !ok {
		return false
	}
	if cxSrc == 0 {
		cxSrc = int32(img.Bounds().Dx())
	}
	if cySrc == 0 {
		cySrc = int32(img.Bounds().Dy())
	}
	// Positive BITMAPINFO heights are bottom-up DIBs and GDI source Y is
	// measured from the lower edge. emfDecodeDIB normalizes pixels to a top-left
	// image.Image, so translate the source rectangle before cropping.
	if len(bmi) >= 12 && int32(binary.LittleEndian.Uint32(bmi[8:12])) > 0 {
		ySrc = int32(img.Bounds().Dy()) - ySrc - int32(math.Abs(float64(cySrc)))
	}
	crop := emfCropImage(img, int(xSrc), int(ySrc), int(cxSrc), int(cySrc))
	if crop == nil {
		return false
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, crop); err != nil {
		return false
	}
	x0, y0 := emfPoint(state, xDest, yDest, width, height)
	x1, y1 := emfPoint(state, xDest+cxDest, yDest, width, height)
	x2, y2 := emfPoint(state, xDest, yDest+cyDest, width, height)
	w := float64(crop.Bounds().Dx())
	h := float64(crop.Bounds().Dy())
	if w <= 0 || h <= 0 {
		return false
	}
	a := (x1 - x0) / w
	b := (y1 - y0) / w
	c := (x2 - x0) / h
	d := (y2 - y0) / h
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
	fmt.Fprintf(body, `<image x="0" y="0" width="%.3f" height="%.3f" preserveAspectRatio="none" transform="matrix(%.9f %.9f %.9f %.9f %.9f %.9f)" href="%s"/>`, w, h, a, b, c, d, x0, y0, uri)
	return true
}

func emfCropImage(img image.Image, x, y, w, h int) image.Image {
	if img == nil || w == 0 || h == 0 {
		return nil
	}
	flipX, flipY := w < 0, h < 0
	if w < 0 {
		x += w
		w = -w
	}
	if h < 0 {
		y += h
		h = -h
	}
	b := img.Bounds()
	r := image.Rect(x, y, x+w, y+h).Intersect(b)
	if r.Empty() {
		return nil
	}
	dst := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for yy := 0; yy < r.Dy(); yy++ {
		for xx := 0; xx < r.Dx(); xx++ {
			sx, sy := r.Min.X+xx, r.Min.Y+yy
			dx, dy := xx, yy
			if flipX {
				dx = r.Dx() - 1 - xx
			}
			if flipY {
				dy = r.Dy() - 1 - yy
			}
			dst.Set(dx, dy, img.At(sx, sy))
		}
	}
	return dst
}

func emfDecodeDIB(bmi, bits []byte) (image.Image, bool) {
	if len(bmi) < 40 {
		return nil, false
	}
	headerSize := int(binary.LittleEndian.Uint32(bmi[0:4]))
	if headerSize < 40 || headerSize > len(bmi) {
		return nil, false
	}
	w := int(int32(binary.LittleEndian.Uint32(bmi[4:8])))
	hRaw := int32(binary.LittleEndian.Uint32(bmi[8:12]))
	if w <= 0 || hRaw == 0 {
		return nil, false
	}
	topDown := hRaw < 0
	h := int(hRaw)
	if h < 0 {
		h = -h
	}
	bpp := int(binary.LittleEndian.Uint16(bmi[14:16]))
	compression := binary.LittleEndian.Uint32(bmi[16:20])
	if compression != 0 && compression != 3 && compression != 6 { // BI_RGB / BI_BITFIELDS / BI_ALPHABITFIELDS
		return nil, false
	}
	if (compression == 3 || compression == 6) && bpp != 16 && bpp != 32 {
		return nil, false
	}
	rowStride := ((w*bpp + 31) / 32) * 4
	if rowStride <= 0 || rowStride*h > len(bits) {
		return nil, false
	}
	redMask, greenMask, blueMask, alphaMask := uint32(0), uint32(0), uint32(0), uint32(0)
	if compression == 3 || compression == 6 {
		maskOff := 40
		if headerSize >= 52 {
			maskOff = 40
		}
		if maskOff+12 > len(bmi) {
			return nil, false
		}
		redMask = binary.LittleEndian.Uint32(bmi[maskOff : maskOff+4])
		greenMask = binary.LittleEndian.Uint32(bmi[maskOff+4 : maskOff+8])
		blueMask = binary.LittleEndian.Uint32(bmi[maskOff+8 : maskOff+12])
		if (compression == 6 || headerSize >= 56) && maskOff+16 <= len(bmi) {
			alphaMask = binary.LittleEndian.Uint32(bmi[maskOff+12 : maskOff+16])
		}
		if redMask == 0 || greenMask == 0 || blueMask == 0 {
			return nil, false
		}
	}
	var palette []color.NRGBA
	if bpp <= 8 {
		colorsUsed := int(binary.LittleEndian.Uint32(bmi[32:36]))
		if colorsUsed == 0 {
			colorsUsed = 1 << bpp
		}
		palOff := headerSize
		if palOff+colorsUsed*4 > len(bmi) {
			return nil, false
		}
		palette = make([]color.NRGBA, colorsUsed)
		for i := range palette {
			b, g, r := bmi[palOff+i*4], bmi[palOff+i*4+1], bmi[palOff+i*4+2]
			palette[i] = color.NRGBA{R: r, G: g, B: b, A: 255}
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcY := y
		if !topDown {
			srcY = h - 1 - y
		}
		row := bits[srcY*rowStride : srcY*rowStride+rowStride]
		for x := 0; x < w; x++ {
			var c color.NRGBA
			switch bpp {
			case 32:
				i := x * 4
				if compression == 3 || compression == 6 {
					v := binary.LittleEndian.Uint32(row[i : i+4])
					c = color.NRGBA{R: emfMaskChannel(v, redMask), G: emfMaskChannel(v, greenMask), B: emfMaskChannel(v, blueMask), A: 255}
					if alphaMask != 0 {
						c.A = emfMaskChannel(v, alphaMask)
					}
				} else {
					c = color.NRGBA{R: row[i+2], G: row[i+1], B: row[i], A: 255}
				}
			case 24:
				i := x * 3
				c = color.NRGBA{R: row[i+2], G: row[i+1], B: row[i], A: 255}
			case 16:
				i := x * 2
				v := uint32(binary.LittleEndian.Uint16(row[i : i+2]))
				if compression == 3 || compression == 6 {
					c = color.NRGBA{R: emfMaskChannel(v, redMask), G: emfMaskChannel(v, greenMask), B: emfMaskChannel(v, blueMask), A: 255}
					if alphaMask != 0 {
						c.A = emfMaskChannel(v, alphaMask)
					}
				} else {
					c = color.NRGBA{R: uint8(((v >> 10) & 0x1f) * 255 / 31), G: uint8(((v >> 5) & 0x1f) * 255 / 31), B: uint8((v & 0x1f) * 255 / 31), A: 255}
				}
			case 8:
				i := int(row[x])
				if i >= len(palette) {
					return nil, false
				}
				c = palette[i]
			case 4:
				v := row[x/2]
				idx := int(v >> 4)
				if x%2 == 1 {
					idx = int(v & 0x0f)
				}
				if idx >= len(palette) {
					return nil, false
				}
				c = palette[idx]
			case 1:
				idx := int((row[x/8] >> uint(7-x%8)) & 1)
				if idx >= len(palette) {
					return nil, false
				}
				c = palette[idx]
			default:
				return nil, false
			}
			out.SetNRGBA(x, y, c)
		}
	}
	return out, true
}

func emfMaskChannel(value, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}
	shift := uint32(0)
	for ((mask >> shift) & 1) == 0 {
		shift++
	}
	field := mask >> shift
	bits := uint32(0)
	for (field & 1) != 0 {
		bits++
		field >>= 1
	}
	if bits == 0 {
		return 0
	}
	max := uint64((uint64(1) << bits) - 1)
	v := uint64((value & mask) >> shift)
	return uint8((v*255 + max/2) / max)
}

func emfMaskBltSourceCopy(rop uint32) bool {
	// The foreground ROP3 byte is the high byte of the low-order word. 0xCC is SRCCOPY.
	return byte(rop>>16) == 0xCC
}

func emfRenderPlgDIB(body *strings.Builder, record []byte, state emfState, width, height float64, dest [3][2]int32, xSrc, ySrc, cxSrc, cySrc int32, offBMI, cbBMI, offBits, cbBits uint32) bool {
	img, bmi, ok := emfRecordDIB(record, offBMI, cbBMI, offBits, cbBits)
	if !ok {
		return false
	}
	if cxSrc == 0 {
		cxSrc = int32(img.Bounds().Dx())
	}
	if cySrc == 0 {
		cySrc = int32(img.Bounds().Dy())
	}
	if len(bmi) >= 12 && int32(binary.LittleEndian.Uint32(bmi[8:12])) > 0 {
		ySrc = int32(img.Bounds().Dy()) - ySrc - int32(math.Abs(float64(cySrc)))
	}
	crop := emfCropImage(img, int(xSrc), int(ySrc), int(cxSrc), int(cySrc))
	if crop == nil {
		return false
	}
	return emfEmitRasterParallelogram(body, crop, state, width, height, dest)
}

func emfRenderMaskedDIB(body *strings.Builder, record []byte, state emfState, width, height float64, xDest, yDest, cxDest, cyDest, xSrc, ySrc, cxSrc, cySrc int32, offBMI, cbBMI, offBits, cbBits uint32, xMask, yMask int32, offMaskBMI, cbMaskBMI, offMaskBits, cbMaskBits uint32) bool {
	img, bmi, ok := emfRecordDIB(record, offBMI, cbBMI, offBits, cbBits)
	if !ok {
		return false
	}
	if cxSrc == 0 {
		cxSrc = int32(img.Bounds().Dx())
	}
	if cySrc == 0 {
		cySrc = int32(img.Bounds().Dy())
	}
	if len(bmi) >= 12 && int32(binary.LittleEndian.Uint32(bmi[8:12])) > 0 {
		ySrc = int32(img.Bounds().Dy()) - ySrc - int32(math.Abs(float64(cySrc)))
	}
	crop := emfCropImage(img, int(xSrc), int(ySrc), int(cxSrc), int(cySrc))
	if crop == nil {
		return false
	}
	if offMaskBMI != 0 && cbMaskBMI != 0 && offMaskBits != 0 && cbMaskBits != 0 {
		mask, maskBMI, mok := emfRecordDIB(record, offMaskBMI, cbMaskBMI, offMaskBits, cbMaskBits)
		if !mok {
			return false
		}
		my := yMask
		if len(maskBMI) >= 12 && int32(binary.LittleEndian.Uint32(maskBMI[8:12])) > 0 {
			my = int32(mask.Bounds().Dy()) - yMask - int32(crop.Bounds().Dy())
		}
		mc := emfCropImage(mask, int(xMask), int(my), crop.Bounds().Dx(), crop.Bounds().Dy())
		if mc == nil {
			return false
		}
		crop = emfApplyAlphaMask(crop, mc)
	}
	dest := [3][2]int32{{xDest, yDest}, {xDest + cxDest, yDest}, {xDest, yDest + cyDest}}
	return emfEmitRasterParallelogram(body, crop, state, width, height, dest)
}

func emfRecordDIB(record []byte, offBMI, cbBMI, offBits, cbBits uint32) (image.Image, []byte, bool) {
	if offBMI == 0 || cbBMI == 0 || offBits == 0 || cbBits == 0 {
		return nil, nil, false
	}
	be, xe := uint64(offBMI)+uint64(cbBMI), uint64(offBits)+uint64(cbBits)
	if be > uint64(len(record)) || xe > uint64(len(record)) {
		return nil, nil, false
	}
	bmi := record[offBMI:be]
	img, ok := emfDecodeDIB(bmi, record[offBits:xe])
	return img, bmi, ok
}

func emfApplyAlphaMask(src, mask image.Image) image.Image {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	mb := mask.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			mr, mg, mbb, _ := mask.At(mb.Min.X+x, mb.Min.Y+y).RGBA()
			lum := (mr*299 + mg*587 + mbb*114) / 1000
			alpha := uint32(a) * uint32(lum) / 65535
			out.SetNRGBA(x, y, color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(alpha >> 8)})
		}
	}
	return out
}

func emfEmitRasterParallelogram(body *strings.Builder, crop image.Image, state emfState, width, height float64, dest [3][2]int32) bool {
	if body == nil || crop == nil {
		return false
	}
	var enc bytes.Buffer
	if err := png.Encode(&enc, crop); err != nil {
		return false
	}
	p0x, p0y := emfPoint(state, dest[0][0], dest[0][1], width, height)
	p1x, p1y := emfPoint(state, dest[1][0], dest[1][1], width, height)
	p2x, p2y := emfPoint(state, dest[2][0], dest[2][1], width, height)
	w, h := float64(crop.Bounds().Dx()), float64(crop.Bounds().Dy())
	if w <= 0 || h <= 0 {
		return false
	}
	a, b := (p1x-p0x)/w, (p1y-p0y)/w
	c, d := (p2x-p0x)/h, (p2y-p0y)/h
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(enc.Bytes())
	fmt.Fprintf(body, `<image x="0" y="0" width="%.3f" height="%.3f" preserveAspectRatio="none" transform="matrix(%.9f %.9f %.9f %.9f %.9f %.9f)" href="%s"/>`, w, h, a, b, c, d, p0x, p0y, uri)
	return true
}

func emfLooksLikeTextGrid(data []byte) bool {
	textCount, lineCount := 0, 0
	for off := 0; off+8 <= len(data); {
		recordType := binary.LittleEndian.Uint32(data[off : off+4])
		recordSize := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recordSize < 8 || off+recordSize > len(data) {
			break
		}
		switch recordType {
		case emrExtTextOutW:
			textCount++
		case emrLineTo:
			lineCount++
		}
		off += recordSize
	}
	return textCount >= 6 && lineCount >= 4
}

func emfBrushFill(state emfState) string {
	if state.brushNull {
		return "none"
	}
	if strings.TrimSpace(state.brushPaint) != "" {
		return state.brushPaint
	}
	return state.brushColor
}

func emfDIBDataURI(record []byte, offBMI, cbBMI, offBits, cbBits uint32) (string, int, int, bool) {
	if offBMI == 0 || cbBMI == 0 || offBits == 0 || cbBits == 0 {
		return "", 0, 0, false
	}
	bmiEnd := uint64(offBMI) + uint64(cbBMI)
	bitsEnd := uint64(offBits) + uint64(cbBits)
	if bmiEnd > uint64(len(record)) || bitsEnd > uint64(len(record)) {
		return "", 0, 0, false
	}
	img, ok := emfDecodeDIB(record[offBMI:bmiEnd], record[offBits:bitsEnd])
	if !ok {
		return "", 0, 0, false
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return "", 0, 0, false
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()), img.Bounds().Dx(), img.Bounds().Dy(), true
}

func emfObjectBrushPaint(body *strings.Builder, openGroups *[]string, object emfObject, state emfState, serial *int) string {
	if object.null {
		return "none"
	}
	if object.patternURI != "" {
		*serial++
		id := fmt.Sprintf("emf-brush-%d", *serial)
		emfDefineImagePattern(body, openGroups, id, object.patternURI, object.patternW, object.patternH, state.brushOrgX, state.brushOrgY)
		return fmt.Sprintf("url(#%s)", id)
	}
	if object.brushStyle == 2 {
		*serial++
		id := fmt.Sprintf("emf-brush-%d", *serial)
		emfDefineHatchPattern(body, openGroups, id, object.color, object.hatch, state.brushOrgX, state.brushOrgY)
		return fmt.Sprintf("url(#%s)", id)
	}
	if object.color != "" {
		return object.color
	}
	return "#000000"
}

func emfActivateSpecialBrush(body *strings.Builder, openGroups *[]string, state *emfState, serial *int) {
	if body == nil || openGroups == nil || state == nil || serial == nil || state.brushNull {
		return
	}
	if state.brushPatternURI != "" {
		*serial++
		id := fmt.Sprintf("emf-brush-%d", *serial)
		emfDefineImagePattern(body, openGroups, id, state.brushPatternURI, state.brushPatternW, state.brushPatternH, state.brushOrgX, state.brushOrgY)
		state.brushPaint = fmt.Sprintf("url(#%s)", id)
		return
	}
	if state.brushStyle == 2 {
		*serial++
		id := fmt.Sprintf("emf-brush-%d", *serial)
		emfDefineHatchPattern(body, openGroups, id, state.brushColor, state.brushHatch, state.brushOrgX, state.brushOrgY)
		state.brushPaint = fmt.Sprintf("url(#%s)", id)
	}
}

func emfDefineImagePattern(body *strings.Builder, openGroups *[]string, id, uri string, tileW, tileH int, orgX, orgY float64) {
	if tileW <= 0 {
		tileW = 8
	}
	if tileH <= 0 {
		tileH = 8
	}
	previous := emfCloneClipChain(*openGroups)
	emfSyncClipGroups(body, openGroups, nil)
	fmt.Fprintf(body, `<defs><pattern id="%s" patternUnits="userSpaceOnUse" width="%d" height="%d" patternTransform="translate(%.3f %.3f)"><image width="%d" height="%d" preserveAspectRatio="none" href="%s"/></pattern></defs>`, id, tileW, tileH, orgX, orgY, tileW, tileH, uri)
	emfSyncClipGroups(body, openGroups, previous)
}

func emfDefineHatchPattern(body *strings.Builder, openGroups *[]string, id, color string, hatch uint32, orgX, orgY float64) {
	previous := emfCloneClipChain(*openGroups)
	emfSyncClipGroups(body, openGroups, nil)
	fmt.Fprintf(body, `<defs><pattern id="%s" patternUnits="userSpaceOnUse" width="8" height="8" patternTransform="translate(%.3f %.3f)">`, id, orgX, orgY)
	line := func(x1, y1, x2, y2 float64) {
		fmt.Fprintf(body, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`, x1, y1, x2, y2, color)
	}
	switch hatch {
	case 0: // HS_HORIZONTAL
		line(0, 4, 8, 4)
	case 1: // HS_VERTICAL
		line(4, 0, 4, 8)
	case 2: // HS_FDIAGONAL
		line(-2, 8, 8, -2)
		line(2, 10, 10, 2)
	case 3: // HS_BDIAGONAL
		line(-2, 0, 8, 10)
		line(2, -2, 10, 6)
	case 4: // HS_CROSS
		line(0, 4, 8, 4)
		line(4, 0, 4, 8)
	case 5: // HS_DIAGCROSS
		line(-2, 8, 8, -2)
		line(2, 10, 10, 2)
		line(-2, 0, 8, 10)
		line(2, -2, 10, 6)
	default:
		line(0, 4, 8, 4)
	}
	body.WriteString(`</pattern></defs>`)
	emfSyncClipGroups(body, openGroups, previous)
}

type emfGradientVertex struct {
	x, y       int32
	r, g, b, a uint16
}

func emfGradientColor(v emfGradientVertex) color.NRGBA {
	return color.NRGBA{R: uint8(v.r >> 8), G: uint8(v.g >> 8), B: uint8(v.b >> 8), A: 255}
}

func emfSVGColor(v emfGradientVertex) string {
	c := emfGradientColor(v)
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func emfRenderGradientFill(body *strings.Builder, openGroups *[]string, record []byte, state emfState, width, height float64, serial *int) bool {
	if len(record) < 36 || serial == nil {
		return false
	}
	nVert := int(binary.LittleEndian.Uint32(record[24:28]))
	nMesh := int(binary.LittleEndian.Uint32(record[28:32]))
	mode := binary.LittleEndian.Uint32(record[32:36])
	if nVert <= 0 || nMesh <= 0 || 36+nVert*16 > len(record) {
		return false
	}
	vertices := make([]emfGradientVertex, nVert)
	for i := 0; i < nVert; i++ {
		o := 36 + i*16
		vertices[i] = emfGradientVertex{
			x: int32(binary.LittleEndian.Uint32(record[o : o+4])), y: int32(binary.LittleEndian.Uint32(record[o+4 : o+8])),
			r: binary.LittleEndian.Uint16(record[o+8 : o+10]), g: binary.LittleEndian.Uint16(record[o+10 : o+12]), b: binary.LittleEndian.Uint16(record[o+12 : o+14]), a: binary.LittleEndian.Uint16(record[o+14 : o+16]),
		}
	}
	meshOff := 36 + nVert*16
	if mode <= 1 {
		if meshOff+nMesh*8 > len(record) {
			return false
		}
		for i := 0; i < nMesh; i++ {
			o := meshOff + i*8
			i1, i2 := int(binary.LittleEndian.Uint32(record[o:o+4])), int(binary.LittleEndian.Uint32(record[o+4:o+8]))
			if i1 < 0 || i2 < 0 || i1 >= len(vertices) || i2 >= len(vertices) {
				continue
			}
			v1, v2 := vertices[i1], vertices[i2]
			p00x, p00y := emfPoint(state, v1.x, v1.y, width, height)
			p10x, p10y := emfPoint(state, v2.x, v1.y, width, height)
			p11x, p11y := emfPoint(state, v2.x, v2.y, width, height)
			p01x, p01y := emfPoint(state, v1.x, v2.y, width, height)
			*serial++
			id := fmt.Sprintf("emf-gradient-%d", *serial)
			previous := emfCloneClipChain(*openGroups)
			emfSyncClipGroups(body, openGroups, nil)
			if mode == 0 {
				fmt.Fprintf(body, `<defs><linearGradient id="%s" gradientUnits="userSpaceOnUse" x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`, id, p00x, p00y, p10x, p10y, emfSVGColor(v1), emfSVGColor(v2))
			} else {
				fmt.Fprintf(body, `<defs><linearGradient id="%s" gradientUnits="userSpaceOnUse" x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`, id, p00x, p00y, p01x, p01y, emfSVGColor(v1), emfSVGColor(v2))
			}
			emfSyncClipGroups(body, openGroups, previous)
			fmt.Fprintf(body, `<path d="M %.3f %.3f L %.3f %.3f L %.3f %.3f L %.3f %.3f Z" fill="url(#%s)" stroke="none"/>`, p00x, p00y, p10x, p10y, p11x, p11y, p01x, p01y, id)
		}
		return true
	}
	if mode == 2 {
		if meshOff+nMesh*12 > len(record) {
			return false
		}
		for i := 0; i < nMesh; i++ {
			o := meshOff + i*12
			idx := [3]int{int(binary.LittleEndian.Uint32(record[o : o+4])), int(binary.LittleEndian.Uint32(record[o+4 : o+8])), int(binary.LittleEndian.Uint32(record[o+8 : o+12]))}
			if idx[0] >= len(vertices) || idx[1] >= len(vertices) || idx[2] >= len(vertices) {
				continue
			}
			v := [3]emfGradientVertex{vertices[idx[0]], vertices[idx[1]], vertices[idx[2]]}
			p := [3][2]float64{}
			for j := 0; j < 3; j++ {
				p[j][0], p[j][1] = emfPoint(state, v[j].x, v[j].y, width, height)
			}
			emfRenderGradientTriangle(body, p, v)
		}
		return true
	}
	return false
}

func emfRenderGradientTriangle(body *strings.Builder, p [3][2]float64, v [3]emfGradientVertex) bool {
	minX, maxX := p[0][0], p[0][0]
	minY, maxY := p[0][1], p[0][1]
	for i := 1; i < 3; i++ {
		minX = math.Min(minX, p[i][0])
		maxX = math.Max(maxX, p[i][0])
		minY = math.Min(minY, p[i][1])
		maxY = math.Max(maxY, p[i][1])
	}
	w, h := int(math.Ceil(maxX-minX)), int(math.Ceil(maxY-minY))
	if w <= 0 || h <= 0 {
		return false
	}
	scale := 1.0
	if m := math.Max(float64(w), float64(h)); m > 1024 {
		scale = 1024 / m
		w = int(math.Ceil(float64(w) * scale))
		h = int(math.Ceil(float64(h) * scale))
	}
	if w <= 0 || h <= 0 {
		return false
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	den := (p[1][1]-p[2][1])*(p[0][0]-p[2][0]) + (p[2][0]-p[1][0])*(p[0][1]-p[2][1])
	if math.Abs(den) < 1e-9 {
		return false
	}
	c := [3]color.NRGBA{emfGradientColor(v[0]), emfGradientColor(v[1]), emfGradientColor(v[2])}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px := minX + (float64(x)+0.5)/scale
			py := minY + (float64(y)+0.5)/scale
			w0 := ((p[1][1]-p[2][1])*(px-p[2][0]) + (p[2][0]-p[1][0])*(py-p[2][1])) / den
			w1 := ((p[2][1]-p[0][1])*(px-p[2][0]) + (p[0][0]-p[2][0])*(py-p[2][1])) / den
			w2 := 1 - w0 - w1
			if w0 >= -1e-6 && w1 >= -1e-6 && w2 >= -1e-6 {
				img.SetNRGBA(x, y, color.NRGBA{R: uint8(math.Round(w0*float64(c[0].R) + w1*float64(c[1].R) + w2*float64(c[2].R))), G: uint8(math.Round(w0*float64(c[0].G) + w1*float64(c[1].G) + w2*float64(c[2].G))), B: uint8(math.Round(w0*float64(c[0].B) + w1*float64(c[1].B) + w2*float64(c[2].B))), A: 255})
			}
		}
	}
	var enc bytes.Buffer
	if png.Encode(&enc, img) != nil {
		return false
	}
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(enc.Bytes())
	fmt.Fprintf(body, `<image x="%.3f" y="%.3f" width="%.3f" height="%.3f" preserveAspectRatio="none" href="%s"/>`, minX, minY, maxX-minX, maxY-minY, uri)
	return true
}

func emfNormalizeRect(r emfRect) emfRect {
	if r.x2 < r.x1 {
		r.x1, r.x2 = r.x2, r.x1
	}
	if r.y2 < r.y1 {
		r.y1, r.y2 = r.y2, r.y1
	}
	return r
}
func emfPointInRects(x, y float64, rs []emfRect) bool {
	for _, r := range rs {
		r = emfNormalizeRect(r)
		if x >= r.x1 && x < r.x2 && y >= r.y1 && y < r.y2 {
			return true
		}
	}
	return false
}
func emfCombineRects(a, b []emfRect, mode uint32) []emfRect {
	var xs, ys []float64
	for _, set := range [][]emfRect{a, b} {
		for _, r := range set {
			r = emfNormalizeRect(r)
			xs = append(xs, r.x1, r.x2)
			ys = append(ys, r.y1, r.y2)
		}
	}
	if len(xs) == 0 || len(ys) == 0 {
		return nil
	}
	sort.Float64s(xs)
	sort.Float64s(ys)
	xs = emfUniqueFloats(xs)
	ys = emfUniqueFloats(ys)
	var out []emfRect
	for yi := 0; yi+1 < len(ys); yi++ {
		for xi := 0; xi+1 < len(xs); xi++ {
			x0, x1 := xs[xi], xs[xi+1]
			y0, y1 := ys[yi], ys[yi+1]
			if x1 <= x0 || y1 <= y0 {
				continue
			}
			mx, my := (x0+x1)/2, (y0+y1)/2
			ina, inb := emfPointInRects(mx, my, a), emfPointInRects(mx, my, b)
			keep := false
			switch mode {
			case 1:
				keep = ina && inb
			case 2:
				keep = ina || inb
			case 3:
				keep = ina != inb
			case 4:
				keep = ina && !inb
			case 5:
				keep = inb
			}
			if keep {
				out = append(out, emfRect{x0, y0, x1, y1})
			}
		}
	}
	return emfMergeRects(out)
}
func emfUniqueFloats(v []float64) []float64 {
	if len(v) == 0 {
		return nil
	}
	out := []float64{v[0]}
	for _, x := range v[1:] {
		if math.Abs(x-out[len(out)-1]) > 1e-9 {
			out = append(out, x)
		}
	}
	return out
}
func emfMergeRects(in []emfRect) []emfRect {
	// Merge horizontal cells first, then vertical strips with identical X span.
	sort.Slice(in, func(i, j int) bool {
		if math.Abs(in[i].y1-in[j].y1) > 1e-9 {
			return in[i].y1 < in[j].y1
		}
		if math.Abs(in[i].y2-in[j].y2) > 1e-9 {
			return in[i].y2 < in[j].y2
		}
		return in[i].x1 < in[j].x1
	})
	var h []emfRect
	for _, r := range in {
		r = emfNormalizeRect(r)
		n := len(h)
		if n > 0 && math.Abs(h[n-1].y1-r.y1) < 1e-9 && math.Abs(h[n-1].y2-r.y2) < 1e-9 && math.Abs(h[n-1].x2-r.x1) < 1e-9 {
			h[n-1].x2 = r.x2
		} else {
			h = append(h, r)
		}
	}
	sort.Slice(h, func(i, j int) bool {
		if math.Abs(h[i].x1-h[j].x1) > 1e-9 {
			return h[i].x1 < h[j].x1
		}
		if math.Abs(h[i].x2-h[j].x2) > 1e-9 {
			return h[i].x2 < h[j].x2
		}
		return h[i].y1 < h[j].y1
	})
	var out []emfRect
	for _, r := range h {
		n := len(out)
		if n > 0 && math.Abs(out[n-1].x1-r.x1) < 1e-9 && math.Abs(out[n-1].x2-r.x2) < 1e-9 && math.Abs(out[n-1].y2-r.y1) < 1e-9 {
			out[n-1].y2 = r.y2
		} else {
			out = append(out, r)
		}
	}
	return out
}
func emfRectsElement(rs []emfRect) string {
	var b strings.Builder
	for _, r := range rs {
		r = emfNormalizeRect(r)
		fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f"/>`, r.x1, r.y1, r.x2-r.x1, r.y2-r.y1)
	}
	return b.String()
}
func emfRegionRects(data []byte, state emfState, width, height float64) ([]emfRect, bool) {
	if len(data) < 32 {
		return nil, false
	}
	hs := int(binary.LittleEndian.Uint32(data[0:4]))
	typ := binary.LittleEndian.Uint32(data[4:8])
	n := int(binary.LittleEndian.Uint32(data[8:12]))
	if hs < 32 || hs > len(data) || typ != 1 || n <= 0 || hs+n*16 > len(data) {
		return nil, false
	}
	out := make([]emfRect, 0, n)
	for i := 0; i < n; i++ {
		o := hs + i*16
		x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(data[o:o+4])), int32(binary.LittleEndian.Uint32(data[o+4:o+8])), width, height)
		x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(data[o+8:o+12])), int32(binary.LittleEndian.Uint32(data[o+12:o+16])), width, height)
		out = append(out, emfNormalizeRect(emfRect{x1, y1, x2, y2}))
	}
	return out, true
}

func applyEMFStockObject(state *emfState, index uint32) {
	if state == nil {
		return
	}
	switch index {
	case 0: // WHITE_BRUSH
		state.brushColor, state.brushPaint, state.brushNull = "#ffffff", "#ffffff", false
	case 1: // LTGRAY_BRUSH
		state.brushColor, state.brushPaint, state.brushNull = "#c0c0c0", "#c0c0c0", false
	case 2: // GRAY_BRUSH
		state.brushColor, state.brushPaint, state.brushNull = "#808080", "#808080", false
	case 3: // DKGRAY_BRUSH
		state.brushColor, state.brushPaint, state.brushNull = "#404040", "#404040", false
	case 4: // BLACK_BRUSH
		state.brushColor, state.brushPaint, state.brushNull = "#000000", "#000000", false
	case 5: // NULL_BRUSH / HOLLOW_BRUSH
		state.brushNull = true
	case 6: // WHITE_PEN
		state.penColor, state.penWidth, state.penNull = "#ffffff", 1, false
	case 7: // BLACK_PEN
		state.penColor, state.penWidth, state.penNull = "#000000", 1, false
	case 8: // NULL_PEN
		state.penNull = true
	case 13: // SYSTEM_FONT
		state.fontFamily, state.fontWeight, state.fontItalic = "Arial", "normal", false
	}
}

func emfStroke(state emfState) (string, float64) {
	if state.penNull {
		return "none", 0
	}
	return state.penColor, math.Max(0.35, state.penWidth)
}

func emfPoint(state emfState, x, y int32, width, height float64) (float64, float64) {
	// Coordinates in MM_ANISOTROPIC EMFs are logical coordinates too. They must
	// pass through the window -> viewport transform just like other scalable map
	// modes. In particular, Office WMFC payloads commonly use logical extents that
	// map almost exactly onto rclBounds; treating the records as device coordinates
	// collapses the drawing toward the header origin.
	// GDI maps logical window coordinates into viewport/device coordinates.
	// Office-generated EMFs commonly use a large logical window (for example
	// 0.01-mm units) and a much smaller viewport matching the EMF frame. Using
	// the header bounds directly as the window scale shrinks such drawings into
	// the top-left corner (notably WMFC-embedded EMFs in PPTX WMF pictures).
	vx, vy := state.viewportExtX, state.viewportExtY
	if vx == 0 {
		vx = width
	}
	if vy == 0 {
		vy = height
	}
	sx, sy := 1.0, 1.0
	if state.windowExtX != 0 {
		sx = vx / state.windowExtX
	}
	if state.windowExtY != 0 {
		sy = vy / state.windowExtY
	}
	lx, ly := emfApplyTransform(state.world, float64(x), float64(y))
	xd := state.viewportOrgX + (lx-state.windowOrgX)*sx
	yd := state.viewportOrgY + (ly-state.windowOrgY)*sy
	if state.layoutMode&0x00000001 != 0 { // LAYOUT_RTL mirrors the device X axis.
		xd = width - xd
	}
	return xd, yd
}

func emfMappedDeltaX(state emfState, x int32, width float64) float64 {
	dx, _ := emfApplyVector(state.world, float64(x), 0)
	vx := state.viewportExtX
	if vx == 0 {
		vx = width
	}
	if state.windowExtX == 0 {
		return dx
	}
	return dx * vx / state.windowExtX
}

func emfMappedDeltaY(state emfState, y int32, height float64) float64 {
	_, dy := emfApplyVector(state.world, 0, float64(y))
	vy := state.viewportExtY
	if vy == 0 {
		vy = height
	}
	if state.windowExtY == 0 {
		return dy
	}
	return dy * vy / state.windowExtY
}

func emfIdentityTransform() emfTransform {
	return emfTransform{m11: 1, m22: 1}
}

func emfTransformFromRecord(record []byte, off int) (emfTransform, bool) {
	if off < 0 || off+24 > len(record) {
		return emfTransform{}, false
	}
	f := func(p int) float64 { return float64(math.Float32frombits(binary.LittleEndian.Uint32(record[p : p+4]))) }
	return emfTransform{m11: f(off), m12: f(off + 4), m21: f(off + 8), m22: f(off + 12), dx: f(off + 16), dy: f(off + 20)}, true
}

// emfMultiplyTransform composes affine transforms using GDI's row-vector
// convention: applying a followed by b yields a*b.
func emfMultiplyTransform(a, b emfTransform) emfTransform {
	return emfTransform{
		m11: a.m11*b.m11 + a.m12*b.m21,
		m12: a.m11*b.m12 + a.m12*b.m22,
		m21: a.m21*b.m11 + a.m22*b.m21,
		m22: a.m21*b.m12 + a.m22*b.m22,
		dx:  a.dx*b.m11 + a.dy*b.m21 + b.dx,
		dy:  a.dx*b.m12 + a.dy*b.m22 + b.dy,
	}
}

func emfEffectiveTransform(t emfTransform) emfTransform {
	if t.m11 == 0 && t.m12 == 0 && t.m21 == 0 && t.m22 == 0 && t.dx == 0 && t.dy == 0 {
		return emfIdentityTransform()
	}
	return t
}

func emfApplyTransform(t emfTransform, x, y float64) (float64, float64) {
	t = emfEffectiveTransform(t)
	return x*t.m11 + y*t.m21 + t.dx, x*t.m12 + y*t.m22 + t.dy
}

func emfApplyVector(t emfTransform, x, y float64) (float64, float64) {
	t = emfEffectiveTransform(t)
	return x*t.m11 + y*t.m21, x*t.m12 + y*t.m22
}

func emfTextAnchor(align uint32) string {
	switch align & 6 {
	case 2: // TA_RIGHT
		return "end"
	case 6: // TA_CENTER
		return "middle"
	default:
		return "start"
	}
}

func emfTextBaseline(align uint32) string {
	switch align & 24 {
	case 8: // TA_BOTTOM
		return "text-after-edge"
	case 24: // TA_BASELINE
		return "alphabetic"
	default: // TA_TOP
		return "text-before-edge"
	}
}

func emfMappedFontSize(state emfState, logical float64, width, height float64) float64 {
	if logical <= 0 {
		return logical
	}
	// Font height is a logical Y extent. The world transform may rotate it, so
	// use the transformed vector magnitude before page-to-device scaling.
	dx, dy := emfApplyVector(state.world, 0, logical)
	vx, vy := state.viewportExtX, state.viewportExtY
	if vx == 0 {
		vx = width
	}
	if vy == 0 {
		vy = height
	}
	sx, sy := 1.0, 1.0
	if state.windowExtX != 0 {
		sx = vx / state.windowExtX
	}
	if state.windowExtY != 0 {
		sy = vy / state.windowExtY
	}
	return math.Hypot(dx*sx, dy*sy)
}

func emfMappedVector(state emfState, x, y, width, height float64) (float64, float64) {
	dx, dy := emfApplyVector(state.world, x, y)
	vx, vy := state.viewportExtX, state.viewportExtY
	if vx == 0 {
		vx = width
	}
	if vy == 0 {
		vy = height
	}
	sx, sy := 1.0, 1.0
	if state.windowExtX != 0 {
		sx = vx / state.windowExtX
	}
	if state.windowExtY != 0 {
		sy = vy / state.windowExtY
	}
	return dx * sx, dy * sy
}

func emfFontVector(angleTenths int32, magnitude float64) (float64, float64) {
	// LOGFONT escapement/orientation angles are counter-clockwise in tenths of a
	// degree. SVG device coordinates are Y-down, so a positive GDI angle has a
	// negative logical Y component before the world/page transform is applied.
	theta := float64(angleTenths) * math.Pi / 1800.0
	return magnitude * math.Cos(theta), -magnitude * math.Sin(theta)
}

func emfTextMappedAdvance(state emfState, logicalX, logicalY float64, pdy bool, width, height float64) (float64, float64) {
	if !pdy {
		logicalX, logicalY = emfFontVector(state.fontEscapement, logicalX)
	}
	return emfMappedVector(state, logicalX, logicalY, width, height)
}

func emfTextAdvance(record []byte, text string, chars, dxOffset int, options uint32, state emfState, width, height float64) (float64, float64) {
	pdy := options&0x2000 != 0 // ETO_PDY: Dx contains x/y pairs.
	stride := 4
	if pdy {
		stride = 8
	}
	if chars > 0 && dxOffset > 0 && dxOffset+chars*stride <= len(record) {
		var logicalX, logicalY float64
		for i := 0; i < chars; i++ {
			logicalX += float64(int32(binary.LittleEndian.Uint32(record[dxOffset+i*stride : dxOffset+i*stride+4])))
			if pdy {
				logicalY += float64(int32(binary.LittleEndian.Uint32(record[dxOffset+i*stride+4 : dxOffset+i*stride+8])))
			}
		}
		return emfTextMappedAdvance(state, logicalX, logicalY, pdy, width, height)
	}
	// EMR_SETTEXTJUSTIFICATION only contributes spacing at break characters. It
	// does not provide glyph widths, so only its explicit extra amount can be
	// reflected in a current-position update when no Dx array is present.
	if state.textJustExtra != 0 && state.textJustCount > 0 && strings.ContainsRune(text, ' ') {
		return emfTextMappedAdvance(state, float64(state.textJustExtra), 0, false, width, height)
	}
	return 0, 0
}

func emfTextRectPoints(record []byte, state emfState, width, height float64) [4][2]float64 {
	var pts [4][2]float64
	if len(record) < 72 {
		return pts
	}
	left := int32(binary.LittleEndian.Uint32(record[56:60]))
	top := int32(binary.LittleEndian.Uint32(record[60:64]))
	right := int32(binary.LittleEndian.Uint32(record[64:68]))
	bottom := int32(binary.LittleEndian.Uint32(record[68:72]))
	pts[0][0], pts[0][1] = emfPoint(state, left, top, width, height)
	pts[1][0], pts[1][1] = emfPoint(state, right, top, width, height)
	pts[2][0], pts[2][1] = emfPoint(state, right, bottom, width, height)
	pts[3][0], pts[3][1] = emfPoint(state, left, bottom, width, height)
	return pts
}

func emfPointsAttribute(points [4][2]float64) string {
	return fmt.Sprintf("%.3f,%.3f %.3f,%.3f %.3f,%.3f %.3f,%.3f",
		points[0][0], points[0][1], points[1][0], points[1][1],
		points[2][0], points[2][1], points[3][0], points[3][1])
}

func emfRenderTextOpaqueRect(body *strings.Builder, record []byte, state emfState, width, height float64) {
	points := emfTextRectPoints(record, state, width, height)
	fmt.Fprintf(body, `<polygon points="%s" fill="%s" stroke="none"/>`, emfPointsAttribute(points), state.bkColor)
}

func emfTextClipElement(record []byte, state emfState, width, height float64) string {
	return fmt.Sprintf(`<polygon points="%s"/>`, emfPointsAttribute(emfTextRectPoints(record, state, width, height)))
}

func emfUTF16Glyphs(data []byte, chars int) []string {
	if chars <= 0 || len(data) < chars*2 {
		return nil
	}
	glyphs := make([]string, 0, chars)
	for i := 0; i < chars; i++ {
		u := binary.LittleEndian.Uint16(data[i*2 : i*2+2])
		if u >= 0xd800 && u <= 0xdbff && i+1 < chars {
			v := binary.LittleEndian.Uint16(data[(i+1)*2 : (i+1)*2+2])
			if v >= 0xdc00 && v <= 0xdfff {
				r := rune(0x10000 + (uint32(u-0xd800) << 10) + uint32(v-0xdc00))
				glyphs = append(glyphs, string(r))
				i++
				continue
			}
		}
		glyphs = append(glyphs, string(rune(u)))
	}
	return glyphs
}

func emfTextDecoration(state emfState) string {
	parts := make([]string, 0, 2)
	if state.fontUnderline {
		parts = append(parts, "underline")
	}
	if state.fontStrikeout {
		parts = append(parts, "line-through")
	}
	return strings.Join(parts, " ")
}

func emfTextAngle(state emfState, angleTenths int32, width, height float64) float64 {
	vx, vy := emfFontVector(angleTenths, 1)
	dx, dy := emfMappedVector(state, vx, vy, width, height)
	if math.Abs(dx) < 1e-12 && math.Abs(dy) < 1e-12 {
		return 0
	}
	return math.Atan2(dy, dx) * 180 / math.Pi
}

func emfTextRotation(state emfState, width, height float64) float64 {
	return emfTextAngle(state, state.fontOrientation, width, height)
}

func emfLegacySymbolRune(code byte) (rune, bool) {
	// Adobe/Windows Symbol encoding used by Equation Editor/MathType.  EMF
	// serializes these through the F0xx private-use range when lfCharSet is
	// SYMBOL_CHARSET.  Convert the encoded font position to its Unicode
	// meaning so playback does not depend on the legacy Symbol font being
	// installed on the browser host.
	m := map[byte]rune{
		0x22: '\u2200', 0x24: '\u2203', 0x27: '\u220b', 0x2a: '\u2217',
		0x2b: '+', 0x2d: '\u2212', 0x3d: '=',
		0x40: '\u2245',
		0x41: '\u0391', 0x42: '\u0392', 0x43: '\u03a7', 0x44: '\u0394',
		0x45: '\u0395', 0x46: '\u03a6', 0x47: '\u0393', 0x48: '\u0397',
		0x49: '\u0399', 0x4a: '\u03d1', 0x4b: '\u039a', 0x4c: '\u039b',
		0x4d: '\u039c', 0x4e: '\u039d', 0x4f: '\u039f', 0x50: '\u03a0',
		0x51: '\u0398', 0x52: '\u03a1', 0x53: '\u03a3', 0x54: '\u03a4',
		0x55: '\u03a5', 0x56: '\u03c2', 0x57: '\u03a9', 0x58: '\u039e',
		0x59: '\u03a8', 0x5a: '\u0396', 0x5c: '\u2234', 0x5e: '\u22a5',
		0x60: '\u203e',
		0x61: '\u03b1', 0x62: '\u03b2', 0x63: '\u03c7', 0x64: '\u03b4',
		0x65: '\u03b5', 0x66: '\u03c6', 0x67: '\u03b3', 0x68: '\u03b7',
		0x69: '\u03b9', 0x6a: '\u03d5', 0x6b: '\u03ba', 0x6c: '\u03bb',
		0x6d: '\u03bc', 0x6e: '\u03bd', 0x6f: '\u03bf', 0x70: '\u03c0',
		0x71: '\u03b8', 0x72: '\u03c1', 0x73: '\u03c3', 0x74: '\u03c4',
		0x75: '\u03c5', 0x76: '\u03d6', 0x77: '\u03c9', 0x78: '\u03be',
		0x79: '\u03c8', 0x7a: '\u03b6', 0x7e: '\u223c',
		0xa1: '\u03d2', 0xa2: '\u2032', 0xa3: '\u2264', 0xa4: '\u2044',
		0xa5: '\u221e', 0xa6: '\u0192', 0xab: '\u2194', 0xac: '\u2190',
		0xad: '\u2191', 0xae: '\u2192', 0xaf: '\u2193',
		0xb0: '\u00b0', 0xb1: '\u00b1', 0xb2: '\u2033', 0xb3: '\u2265',
		0xb4: '\u00d7', 0xb5: '\u221d', 0xb6: '\u2202', 0xb7: '\u2022',
		0xb8: '\u00f7', 0xb9: '\u2260', 0xba: '\u2261', 0xbb: '\u2248',
		0xbc: '\u2026', 0xbd: '\u23d0', 0xbe: '\u23af',
		0xc0: '\u2135', 0xc1: '\u2111', 0xc2: '\u211c', 0xc3: '\u2118',
		0xc4: '\u2297', 0xc5: '\u2295', 0xc6: '\u2205', 0xc7: '\u2229',
		0xc8: '\u222a', 0xc9: '\u2283', 0xca: '\u2287', 0xcb: '\u2284',
		0xcc: '\u2282', 0xcd: '\u2286', 0xce: '\u2208', 0xcf: '\u2209',
		0xd0: '\u2220', 0xd1: '\u2207', 0xd5: '\u220f', 0xd6: '\u221a',
		0xd7: '\u22c5', 0xd8: '\u00ac', 0xd9: '\u2227', 0xda: '\u2228',
		0xdb: '\u21d4', 0xdc: '\u21d0', 0xdd: '\u21d1', 0xde: '\u21d2', 0xdf: '\u21d3',
		0xe0: '\u25ca', 0xe1: '\u2329', 0xe5: '\u2211',
		0xe6: '\u239b', 0xe7: '\u239c', 0xe8: '\u239d',
		0xe9: '\u23a1', 0xea: '\u23a2', 0xeb: '\u23a3',
		0xec: '\u23a7', 0xed: '\u23a8', 0xee: '\u23a9', 0xef: '\u23aa',
		0xf1: '\u232a', 0xf2: '\u222b', 0xf3: '\u2320', 0xf4: '\u23ae', 0xf5: '\u2321',
		0xf6: '\u239e', 0xf7: '\u239f', 0xf8: '\u23a0',
		0xf9: '\u23a4', 0xfa: '\u23a5', 0xfb: '\u23a6',
		0xfc: '\u23ab', 0xfd: '\u23ac', 0xfe: '\u23ad',
	}
	r, ok := m[code]
	if !ok && code >= 0x20 && code <= 0x7e {
		// Symbol preserves ordinary ASCII punctuation and digits at their
		// standard positions; EMF still serializes them as F0xx PUA values.
		return rune(code), true
	}
	return r, ok
}

func emfPortableMathText(fontFamily, text string) (string, string) {
	if text == "" {
		return fontFamily, text
	}
	mathFamily := `Cambria Math, STIX Two Math, DejaVu Serif, serif`
	switch strings.ToLower(strings.TrimSpace(fontFamily)) {
	case "fences":
		// Legacy Equation Editor/MathType uses the Fences font as a collection
		// of scalable delimiter pieces.  Preserve the piecewise square-bracket
		// cases and normalize the complete delimiters seen in Office previews.
		m := map[rune]rune{
			'a': '(', 'f': ')',
			'L': '\u23a1', 'M': '\u23a2', 'N': '\u23a3',
			'O': '\u23a4', 'P': '\u23a5', 'Q': '\u23a6',
			'z': '\u222b',
		}
		return mathFamily, strings.Map(func(r rune) rune {
			if v, ok := m[r]; ok {
				return v
			}
			return r
		}, text)
	case "mt extra":
		m := map[rune]rune{
			0xf024: '^',      // hat embellishment
			0xf04d: '\u22ee', // vertical ellipsis
		}
		return mathFamily, strings.Map(func(r rune) rune {
			if v, ok := m[r]; ok {
				return v
			}
			return r
		}, text)
	case "symbol", "mt symbol", "euclid symbol":
		changed := false
		mapped := strings.Map(func(r rune) rune {
			if r >= 0xf000 && r <= 0xf0ff {
				if v, ok := emfLegacySymbolRune(byte(r - 0xf000)); ok {
					changed = true
					return v
				}
			}
			return r
		}, text)
		if changed {
			return mathFamily, mapped
		}
		// DrawingML can retain a:sym's Symbol typeface on adjacent ordinary
		// Latin text after splitting a mixed run. Those letters are Unicode
		// English, not Symbol character positions, so rendering them through
		// Symbol turns an entire text block into Greek.
		if strings.IndexFunc(text, func(r rune) bool {
			return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
		}) >= 0 {
			return "Times New Roman", text
		}
	}
	return fontFamily, text
}

// PortableMathText converts legacy Symbol-family private-use positions to
// Unicode and selects a browser-portable math fallback when conversion occurs.
// It is shared by metafile and ordinary presentation-text rendering.
func PortableMathText(fontFamily, text string) (string, string) {
	return emfPortableMathText(fontFamily, text)
}

func emfRenderOneText(body *strings.Builder, glyph string, x, y, angle, fontSize float64, state emfState, rtl bool) {
	fontFamily, glyph := emfPortableMathText(state.fontFamily, glyph)
	style := ""
	if state.fontItalic {
		style += ` font-style="italic"`
	}
	if decoration := emfTextDecoration(state); decoration != "" {
		style += fmt.Sprintf(` text-decoration="%s"`, decoration)
	}
	if rtl {
		style += ` direction="rtl" unicode-bidi="bidi-override"`
	}
	if math.Abs(angle) > 1e-9 {
		style += fmt.Sprintf(` transform="rotate(%.3f %.3f %.3f)"`, angle, x, y)
	}
	fmt.Fprintf(body, `<text x="%.3f" y="%.3f" fill="%s" font-family="%s" font-size="%.3f" font-weight="%s" text-anchor="start" dominant-baseline="%s"%s>%s</text>`,
		x, y, state.textColor, html.EscapeString(fontFamily), fontSize, state.fontWeight, emfTextBaseline(state.textAlign), style, html.EscapeString(glyph))
}

func emfRenderExtTextOut(body *strings.Builder, record []byte, text string, chars, stringOffset, dxOffset int, options uint32, state emfState, x, y, fontSize, width, height float64) {
	if body == nil || text == "" || options&0x0010 != 0 { // ETO_GLYPH_INDEX needs font glyph outlines, not Unicode text.
		return
	}
	rtl := options&0x0080 != 0 || state.layoutMode&0x00000001 != 0
	pdy := options&0x2000 != 0
	stride := 4
	if pdy {
		stride = 8
	}
	hasDx := dxOffset > 0 && chars > 0 && dxOffset+chars*stride <= len(record)
	angle := emfTextRotation(state, width, height)
	if !hasDx && (state.fontOrientation == state.fontEscapement || state.fontOrientation == 0) {
		style := ""
		if state.fontItalic {
			style += ` font-style="italic"`
		}
		if decoration := emfTextDecoration(state); decoration != "" {
			style += fmt.Sprintf(` text-decoration="%s"`, decoration)
		}
		if rtl {
			style += ` direction="rtl" unicode-bidi="bidi-override"`
		}
		if textAngle := emfTextAngle(state, state.fontEscapement, width, height); math.Abs(textAngle) > 1e-9 {
			style += fmt.Sprintf(` transform="rotate(%.3f %.3f %.3f)"`, textAngle, x, y)
		}
		if state.textJustExtra != 0 && state.textJustCount > 0 {
			extraX, _ := emfTextMappedAdvance(state, float64(state.textJustExtra)/float64(state.textJustCount), 0, false, width, height)
			style += fmt.Sprintf(` word-spacing="%.3f"`, extraX)
		}
		fontFamily, portableText := emfPortableMathText(state.fontFamily, text)
		fmt.Fprintf(body, `<text x="%.3f" y="%.3f" fill="%s" font-family="%s" font-size="%.3f" font-weight="%s" text-anchor="%s" dominant-baseline="%s"%s>%s</text>`,
			x, y, state.textColor, html.EscapeString(fontFamily), fontSize, state.fontWeight, emfTextAnchor(state.textAlign), emfTextBaseline(state.textAlign), style, html.EscapeString(portableText))
		return
	}

	glyphs := emfUTF16Glyphs(record[stringOffset:stringOffset+chars*2], chars)
	if len(glyphs) == 0 {
		return
	}
	// Exact Dx playback uses individually positioned glyphs. This avoids browser
	// font kerning/letter-spacing from overriding the authored EMF advances.
	cx, cy := x, y
	for i, glyph := range glyphs {
		emfRenderOneText(body, glyph, cx, cy, angle, fontSize, state, rtl)
		if !hasDx || i >= chars {
			continue
		}
		logicalX := float64(int32(binary.LittleEndian.Uint32(record[dxOffset+i*stride : dxOffset+i*stride+4])))
		logicalY := 0.0
		if pdy {
			logicalY = float64(int32(binary.LittleEndian.Uint32(record[dxOffset+i*stride+4 : dxOffset+i*stride+8])))
		}
		dx, dy := emfTextMappedAdvance(state, logicalX, logicalY, pdy, width, height)
		cx += dx
		cy += dy
	}
}

func emfTextRectPointsAt(record []byte, emrTextOffset int, state emfState, width, height float64) [4][2]float64 {
	var pts [4][2]float64
	if emrTextOffset < 0 || emrTextOffset+36 > len(record) {
		return pts
	}
	left := int32(binary.LittleEndian.Uint32(record[emrTextOffset+20 : emrTextOffset+24]))
	top := int32(binary.LittleEndian.Uint32(record[emrTextOffset+24 : emrTextOffset+28]))
	right := int32(binary.LittleEndian.Uint32(record[emrTextOffset+28 : emrTextOffset+32]))
	bottom := int32(binary.LittleEndian.Uint32(record[emrTextOffset+32 : emrTextOffset+36]))
	pts[0][0], pts[0][1] = emfPoint(state, left, top, width, height)
	pts[1][0], pts[1][1] = emfPoint(state, right, top, width, height)
	pts[2][0], pts[2][1] = emfPoint(state, right, bottom, width, height)
	pts[3][0], pts[3][1] = emfPoint(state, left, bottom, width, height)
	return pts
}

func emfRenderEMRTextObject(body *strings.Builder, openClipGroups *[]string, clipSerial *int, record []byte, emrTextOffset int, state *emfState, width, height float64) {
	if body == nil || state == nil || emrTextOffset < 0 || emrTextOffset+40 > len(record) {
		return
	}
	xRaw := int32(binary.LittleEndian.Uint32(record[emrTextOffset : emrTextOffset+4]))
	yRaw := int32(binary.LittleEndian.Uint32(record[emrTextOffset+4 : emrTextOffset+8]))
	chars := int(binary.LittleEndian.Uint32(record[emrTextOffset+8 : emrTextOffset+12]))
	stringOffset := int(binary.LittleEndian.Uint32(record[emrTextOffset+12 : emrTextOffset+16]))
	options := binary.LittleEndian.Uint32(record[emrTextOffset+16 : emrTextOffset+20])
	dxOffset := int(binary.LittleEndian.Uint32(record[emrTextOffset+36 : emrTextOffset+40]))
	if chars <= 0 || stringOffset < 0 || stringOffset+chars*2 > len(record) {
		return
	}
	text := utf16String(record[stringOffset : stringOffset+chars*2])
	x, y := emfPoint(*state, xRaw, yRaw, width, height)
	if state.textAlign&1 != 0 { // TA_UPDATECP
		x, y = state.currentX, state.currentY
	}
	fontSize := emfMappedFontSize(*state, state.fontSize, width, height)
	if fontSize <= 0 {
		fontSize = 12
	}
	if options&0x0002 != 0 { // ETO_OPAQUE
		pts := emfTextRectPointsAt(record, emrTextOffset, *state, width, height)
		fmt.Fprintf(body, `<polygon points="%s" fill="%s" stroke="none"/>`, emfPointsAttribute(pts), state.bkColor)
	}
	clipID := ""
	if options&0x0004 != 0 { // ETO_CLIPPED
		*clipSerial = *clipSerial + 1
		clipID = fmt.Sprintf("emf-text-clip-%d", *clipSerial)
		pts := emfTextRectPointsAt(record, emrTextOffset, *state, width, height)
		emfDefineClipPath(body, openClipGroups, clipID, fmt.Sprintf(`<polygon points="%s"/>`, emfPointsAttribute(pts)))
	}
	if clipID != "" {
		fmt.Fprintf(body, `<g clip-path="url(#%s)">`, clipID)
	}
	emfRenderExtTextOut(body, record, text, chars, stringOffset, dxOffset, options, *state, x, y, fontSize, width, height)
	if clipID != "" {
		body.WriteString(`</g>`)
	}
	if state.textAlign&1 != 0 {
		advanceX, advanceY := emfTextAdvance(record, text, chars, dxOffset, options, *state, width, height)
		state.currentX += advanceX
		state.currentY += advanceY
	}
}

func emfSmallTextString(record []byte, chars int, options uint32, off int) (string, bool) {
	if chars <= 0 || off < 0 || off > len(record) {
		return "", false
	}
	if options&0x00000200 != 0 { // ETO_SMALL_CHARS: one low byte per UTF-16 code unit.
		if off+chars > len(record) {
			return "", false
		}
		runes := make([]rune, chars)
		for i := 0; i < chars; i++ {
			runes[i] = rune(record[off+i])
		}
		return string(runes), true
	}
	if off+chars*2 > len(record) {
		return "", false
	}
	return utf16String(record[off : off+chars*2]), true
}

func emfRenderSmallTextOut(body *strings.Builder, openClipGroups *[]string, clipSerial *int, record []byte, state emfState, width, height float64) {
	if body == nil || len(record) < 36 {
		return
	}
	xRaw := int32(binary.LittleEndian.Uint32(record[8:12]))
	yRaw := int32(binary.LittleEndian.Uint32(record[12:16]))
	chars := int(binary.LittleEndian.Uint32(record[16:20]))
	options := binary.LittleEndian.Uint32(record[20:24])
	if options&0x0010 != 0 { // ETO_GLYPH_INDEX requires glyph ownership.
		return
	}
	textOff := 36
	var pts [4][2]float64
	hasRect := options&0x00000100 == 0 // ETO_NO_RECT
	if hasRect {
		if len(record) < 52 {
			return
		}
		left := int32(binary.LittleEndian.Uint32(record[36:40]))
		top := int32(binary.LittleEndian.Uint32(record[40:44]))
		right := int32(binary.LittleEndian.Uint32(record[44:48]))
		bottom := int32(binary.LittleEndian.Uint32(record[48:52]))
		pts[0][0], pts[0][1] = emfPoint(state, left, top, width, height)
		pts[1][0], pts[1][1] = emfPoint(state, right, top, width, height)
		pts[2][0], pts[2][1] = emfPoint(state, right, bottom, width, height)
		pts[3][0], pts[3][1] = emfPoint(state, left, bottom, width, height)
		textOff = 52
	}
	text, ok := emfSmallTextString(record, chars, options, textOff)
	if !ok || text == "" {
		return
	}
	x, y := emfPoint(state, xRaw, yRaw, width, height)
	if state.textAlign&1 != 0 {
		x, y = state.currentX, state.currentY
	}
	if hasRect && options&0x0002 != 0 {
		fmt.Fprintf(body, `<polygon points="%s" fill="%s" stroke="none"/>`, emfPointsAttribute(pts), state.bkColor)
	}
	clipID := ""
	if hasRect && options&0x0004 != 0 {
		*clipSerial = *clipSerial + 1
		clipID = fmt.Sprintf("emf-text-clip-%d", *clipSerial)
		emfDefineClipPath(body, openClipGroups, clipID, fmt.Sprintf(`<polygon points="%s"/>`, emfPointsAttribute(pts)))
		fmt.Fprintf(body, `<g clip-path="url(#%s)">`, clipID)
	}
	fontSize := emfMappedFontSize(state, state.fontSize, width, height)
	if fontSize <= 0 {
		fontSize = 12
	}
	rtl := options&0x0080 != 0 || state.layoutMode&1 != 0
	style := ""
	if state.fontItalic {
		style += ` font-style="italic"`
	}
	if decoration := emfTextDecoration(state); decoration != "" {
		style += fmt.Sprintf(` text-decoration="%s"`, decoration)
	}
	if rtl {
		style += ` direction="rtl" unicode-bidi="bidi-override"`
	}
	if angle := emfTextAngle(state, state.fontEscapement, width, height); math.Abs(angle) > 1e-9 {
		style += fmt.Sprintf(` transform="rotate(%.3f %.3f %.3f)"`, angle, x, y)
	}
	fontFamily, portableText := emfPortableMathText(state.fontFamily, text)
	fmt.Fprintf(body, `<text x="%.3f" y="%.3f" fill="%s" font-family="%s" font-size="%.3f" font-weight="%s" text-anchor="%s" dominant-baseline="%s"%s>%s</text>`, x, y, state.textColor, html.EscapeString(fontFamily), fontSize, state.fontWeight, emfTextAnchor(state.textAlign), emfTextBaseline(state.textAlign), style, html.EscapeString(portableText))
	if clipID != "" {
		body.WriteString(`</g>`)
	}
}

func emfHasEMFPlusComments(data []byte) bool {
	for off := 0; off+12 <= len(data); {
		typ := binary.LittleEndian.Uint32(data[off : off+4])
		sz := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if sz < 8 || off+sz > len(data) {
			break
		}
		if typ == emrGDIComment && sz >= 16 {
			dataSize := int(binary.LittleEndian.Uint32(data[off+8 : off+12]))
			if dataSize >= 4 && 12+dataSize <= sz && binary.LittleEndian.Uint32(data[off+12:off+16]) == 0x2B464D45 {
				return true
			}
		}
		off += sz
	}
	return false
}

func emfEllipseIntersection(cx, cy, rx, ry, x, y float64) (float64, float64) {
	dx, dy := x-cx, y-cy
	if dx == 0 && dy == 0 {
		return cx + rx, cy
	}
	scale := 1 / math.Sqrt((dx*dx)/(rx*rx)+(dy*dy)/(ry*ry))
	return cx + dx*scale, cy + dy*scale
}

func emfArcFlags(direction uint32, cx, cy, rx, ry, sx, sy, ex, ey float64) (large, sweep int) {
	a1 := math.Atan2((sy-cy)/ry, (sx-cx)/rx)
	a2 := math.Atan2((ey-cy)/ry, (ex-cx)/rx)
	// GDI's AD_COUNTERCLOCKWISE is 1 and AD_CLOCKWISE is 2. SVG uses a
	// Y-down coordinate system, so GDI counter-clockwise maps to sweep=0.
	clockwise := direction == 2
	delta := a2 - a1
	if clockwise {
		for delta <= 0 {
			delta += 2 * math.Pi
		}
		sweep = 1
	} else {
		delta = -delta
		for delta <= 0 {
			delta += 2 * math.Pi
		}
		sweep = 0
	}
	if delta > math.Pi {
		large = 1
	}
	return large, sweep
}

func emfColor(raw uint32) string {
	return fmt.Sprintf("#%02x%02x%02x", raw&0xff, (raw>>8)&0xff, (raw>>16)&0xff)
}

func utf16String(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+1 < len(data); i += 2 {
		u := binary.LittleEndian.Uint16(data[i : i+2])
		if u == 0 {
			break
		}
		if u >= 0xD800 && u <= 0xDFFF {
			b.WriteRune('�')
		} else {
			b.WriteRune(rune(u))
		}
	}
	return b.String()
}

func utf16FixedString(data []byte, maxUnits int) string {
	if maxUnits <= 0 {
		return ""
	}
	limit := maxUnits * 2
	if len(data) < limit {
		limit = len(data) - len(data)%2
	}
	return strings.TrimSpace(utf16String(data[:limit]))
}
