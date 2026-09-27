package metafile

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"strings"
	"unicode/utf16"

	"office/pkg/fontmetrics"
)

const (
	emfPlusHeader                  = 0x4001
	emfPlusEndOfFile               = 0x4002
	emfPlusObject                  = 0x4008
	emfPlusClear                   = 0x4009
	emfPlusFillRects               = 0x400A
	emfPlusDrawRects               = 0x400B
	emfPlusFillPolygon             = 0x400C
	emfPlusDrawLines               = 0x400D
	emfPlusFillEllipse             = 0x400E
	emfPlusDrawEllipse             = 0x400F
	emfPlusFillPie                 = 0x4010
	emfPlusDrawPie                 = 0x4011
	emfPlusDrawArc                 = 0x4012
	emfPlusFillRegion              = 0x4013
	emfPlusFillPath                = 0x4014
	emfPlusDrawPath                = 0x4015
	emfPlusFillClosedCurve         = 0x4016
	emfPlusDrawClosedCurve         = 0x4017
	emfPlusDrawCurve               = 0x4018
	emfPlusDrawBeziers             = 0x4019
	emfPlusDrawImage               = 0x401A
	emfPlusDrawImagePoints         = 0x401B
	emfPlusDrawString              = 0x401C
	emfPlusSetTextRenderingHint    = 0x401F
	emfPlusSetTextContrast         = 0x4020
	emfPlusSetCompositingMode      = 0x4023
	emfPlusSetCompositingQuality   = 0x4024
	emfPlusSave                    = 0x4025
	emfPlusRestore                 = 0x4026
	emfPlusBeginContainer          = 0x4027
	emfPlusBeginContainerNoParams  = 0x4028
	emfPlusEndContainer            = 0x4029
	emfPlusSetWorldTransform       = 0x402A
	emfPlusResetWorldTransform     = 0x402B
	emfPlusMultiplyWorldTransform  = 0x402C
	emfPlusTranslateWorldTransform = 0x402D
	emfPlusScaleWorldTransform     = 0x402E
	emfPlusRotateWorldTransform    = 0x402F
	emfPlusSetPageTransform        = 0x4030
	emfPlusResetClip               = 0x4031
	emfPlusSetClipRect             = 0x4032
	emfPlusSetClipPath             = 0x4033
	emfPlusSetClipRegion           = 0x4034
	emfPlusOffsetClip              = 0x4035
	emfPlusDrawDriverString        = 0x4036
	emfPlusStrokeFillPath          = 0x4037
	emfPlusSerializableObject      = 0x4038
	emfPlusSetRenderingOrigin      = 0x401D
	emfPlusSetAntiAliasMode        = 0x401E
	emfPlusSetInterpolationMode    = 0x4021
	emfPlusSetPixelOffsetMode      = 0x4022

	emfPlusObjectBrush           = 1
	emfPlusObjectPen             = 2
	emfPlusObjectPath            = 3
	emfPlusObjectRegion          = 4
	emfPlusObjectImage           = 5
	emfPlusObjectFont            = 6
	emfPlusObjectStringFormat    = 7
	emfPlusObjectImageAttributes = 8
	emfPlusObjectCustomLineCap   = 9
)

type emfPlusRecord struct {
	typ      uint16
	flags    uint16
	size     uint32
	dataSize uint32
	data     []byte
}

type emfPlusPathObject struct {
	d string
}

type emfPlusBrushObject struct {
	paint string
}

type emfPlusPenObject struct {
	paint        string
	width        float64
	lineCap      string
	lineJoin     string
	miterLimit   float64
	dashArray    []float64
	dashOffset   float64
	compoundLine []float64
	startMarker  string
	endMarker    string
}
type emfPlusCustomLineCapObject struct {
	markerPath string
	filled     bool
	width      float64
	height     float64
	widthScale float64
	baseInset  float64
	lineJoin   string
	miterLimit float64
}
type emfPlusImageObject struct {
	uri    string
	width  int
	height int
}

type emfPlusFontObject struct {
	family string
	size   float64
	style  uint32
}

type emfPlusCharacterRange struct {
	first  int32
	length int32
}

type emfPlusStringFormatObject struct {
	flags          uint32
	align          uint32
	lineAlign      uint32
	firstTabOffset float64
	hotkeyPrefix   uint32
	leadingMargin  float64
	trailingMargin float64
	tracking       float64
	trimming       uint32
	tabStops       []float64
	ranges         []emfPlusCharacterRange
}

type emfPlusImageAttributesObject struct {
	wrapMode    uint32
	clampColor  string
	objectClamp int32
}

type emfPlusRegionNode struct {
	typ         uint32
	rect        emfPlusRectF
	path        emfPlusPathObject
	left, right *emfPlusRegionNode
}

type emfPlusRegionObject struct {
	root *emfPlusRegionNode
}

type emfPlusObjectEntry struct {
	kind       uint8
	path       emfPlusPathObject
	brush      emfPlusBrushObject
	pen        emfPlusPenObject
	image      emfPlusImageObject
	font       emfPlusFontObject
	stringFmt  emfPlusStringFormatObject
	region     emfPlusRegionObject
	imageAttrs emfPlusImageAttributesObject
	customCap  emfPlusCustomLineCapObject
}

type emfPlusContinuation struct {
	kind  uint8
	total int
	data  []byte
}

type emfPlusState struct {
	world              emfTransform
	pageUnit           uint8
	pageScale          float64
	clips              []string
	compositingMode    uint8
	compositingQuality uint8
	textRenderingHint  uint8
	textContrast       uint16
	renderingOriginX   int32
	renderingOriginY   int32
	antiAliasMode      uint8
	interpolationMode  uint8
	pixelOffsetMode    uint8
}

type emfPlusRectF struct {
	x float64
	y float64
	w float64
	h float64
}

type emfPlusRealizedFont struct {
	family string
	bold   bool
	italic bool
	face   *fontmetrics.Face
}

type emfPlusPlayback struct {
	present            bool
	dual               bool
	world              emfTransform
	objects            map[uint8]emfPlusObjectEntry
	continuations      map[uint8]emfPlusContinuation
	saved              map[uint32]emfPlusState
	clips              []string
	defs               strings.Builder
	body               strings.Builder
	clipSerial         int
	brushSerial        int
	compositingMode    uint8
	compositingQuality uint8
	pageUnit           uint8
	pageScale          float64
	textRenderingHint  uint8
	textContrast       uint16
	renderingOriginX   int32
	renderingOriginY   int32
	antiAliasMode      uint8
	interpolationMode  uint8
	pixelOffsetMode    uint8
	effectFilter       string
	drawn              bool
	fonts              []emfPlusRealizedFont
}

func emfPlusFromEMF(data []byte, opts Options) emfPlusPlayback {
	p := emfPlusPlayback{world: emfIdentityTransform(), pageScale: 1, objects: map[uint8]emfPlusObjectEntry{}, continuations: map[uint8]emfPlusContinuation{}, saved: map[uint32]emfPlusState{}, fonts: emfPlusRealizeFonts(opts.Fonts)}
	stream := emfPlusCommentStream(data)
	if len(stream) == 0 {
		return p
	}
	p.present = true
	for off := 0; off+12 <= len(stream); {
		rec, ok := emfPlusReadRecord(stream[off:])
		if !ok {
			break
		}
		p.play(rec)
		off += int(rec.size)
		if rec.typ == emfPlusEndOfFile {
			break
		}
	}
	return p
}

func emfPlusCommentStream(data []byte) []byte {
	var out []byte
	for off := 0; off+12 <= len(data); {
		typ := binary.LittleEndian.Uint32(data[off : off+4])
		sz := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if sz < 8 || off+sz > len(data) {
			break
		}
		if typ == emrGDIComment && sz >= 16 {
			dataSize := int(binary.LittleEndian.Uint32(data[off+8 : off+12]))
			if dataSize >= 4 && 12+dataSize <= sz && binary.LittleEndian.Uint32(data[off+12:off+16]) == 0x2B464D45 {
				out = append(out, data[off+16:off+12+dataSize]...)
			}
		}
		off += sz
	}
	return out
}

func emfPlusReadRecord(data []byte) (emfPlusRecord, bool) {
	if len(data) < 12 {
		return emfPlusRecord{}, false
	}
	rec := emfPlusRecord{
		typ:      binary.LittleEndian.Uint16(data[0:2]),
		flags:    binary.LittleEndian.Uint16(data[2:4]),
		size:     binary.LittleEndian.Uint32(data[4:8]),
		dataSize: binary.LittleEndian.Uint32(data[8:12]),
	}
	if rec.size < 12 || int(rec.size) > len(data) || rec.dataSize > rec.size-12 {
		return emfPlusRecord{}, false
	}
	rec.data = data[12 : 12+rec.dataSize]
	return rec, true
}

func (p *emfPlusPlayback) play(rec emfPlusRecord) {
	switch rec.typ {
	case emfPlusHeader:
		// Header flags bit 0 identifies EMF+ Dual. Until EMF+ coverage is complete,
		// dual streams retain the authoritative classic EMF fallback.
		p.dual = rec.flags&0x0001 != 0
	case emfPlusObject:
		p.defineObject(rec)
	case emfPlusSave:
		p.saveState(rec)
	case emfPlusRestore:
		p.restoreState(rec)
	case emfPlusBeginContainer:
		p.beginContainer(rec)
	case emfPlusBeginContainerNoParams:
		p.saveState(rec)
	case emfPlusEndContainer:
		p.restoreState(rec)
	case emfPlusSetWorldTransform:
		if t, ok := emfPlusTransform(rec.data); ok {
			p.world = t
		}
	case emfPlusResetWorldTransform:
		p.world = emfIdentityTransform()
	case emfPlusMultiplyWorldTransform:
		if t, ok := emfPlusTransform(rec.data); ok {
			if rec.flags&0x2000 != 0 { // A: post multiply
				p.world = emfMultiplyTransform(p.world, t)
			} else {
				p.world = emfMultiplyTransform(t, p.world)
			}
		}
	case emfPlusTranslateWorldTransform:
		if len(rec.data) >= 8 {
			t := emfTransform{m11: 1, m22: 1, dx: float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[0:4]))), dy: float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[4:8])))}
			p.combineTransform(t, rec.flags)
		}
	case emfPlusScaleWorldTransform:
		if len(rec.data) >= 8 {
			t := emfTransform{m11: float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[0:4]))), m22: float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[4:8])))}
			p.combineTransform(t, rec.flags)
		}
	case emfPlusRotateWorldTransform:
		if len(rec.data) >= 4 {
			deg := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[0:4])))
			rad := deg * math.Pi / 180
			c, s := math.Cos(rad), math.Sin(rad)
			p.combineTransform(emfTransform{m11: c, m12: s, m21: -s, m22: c}, rec.flags)
		}
	case emfPlusSetPageTransform:
		if len(rec.data) >= 4 {
			p.pageUnit = uint8(rec.flags & 0xff)
			p.pageScale = float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[:4])))
			if p.pageScale == 0 {
				p.pageScale = 1
			}
		}
	case emfPlusClear:
		if len(rec.data) >= 4 {
			fmt.Fprintf(&p.body, `<rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="%s"%s/>`, emfPlusARGB(binary.LittleEndian.Uint32(rec.data[:4])), p.clipAttr())
			p.drawn = true
		}
	case emfPlusFillRects:
		p.fillRects(rec)
	case emfPlusDrawRects:
		p.drawRects(rec)
	case emfPlusFillPolygon:
		p.fillPolygon(rec)
	case emfPlusDrawLines:
		p.drawLines(rec)
	case emfPlusFillEllipse:
		p.fillEllipse(rec)
	case emfPlusDrawEllipse:
		p.drawEllipse(rec)
	case emfPlusFillPie:
		p.fillPie(rec)
	case emfPlusDrawPie:
		p.drawPie(rec)
	case emfPlusDrawArc:
		p.drawArc(rec)
	case emfPlusFillRegion:
		p.fillRegion(rec)
	case emfPlusFillPath:
		p.fillPath(rec)
	case emfPlusDrawPath:
		p.drawPath(rec)
	case emfPlusFillClosedCurve:
		p.fillClosedCurve(rec)
	case emfPlusDrawClosedCurve:
		p.drawClosedCurve(rec)
	case emfPlusDrawCurve:
		p.drawCurve(rec)
	case emfPlusDrawBeziers:
		p.drawBeziers(rec)
	case emfPlusDrawImage:
		p.drawImage(rec)
	case emfPlusDrawImagePoints:
		p.drawImagePoints(rec)
	case emfPlusDrawString:
		p.drawString(rec)
	case emfPlusDrawDriverString:
		p.drawDriverString(rec)
	case emfPlusStrokeFillPath:
		p.strokeFillPath(rec)
	case emfPlusSerializableObject:
		p.serializableObject(rec)
	case emfPlusSetRenderingOrigin:
		if len(rec.data) >= 8 {
			p.renderingOriginX = int32(binary.LittleEndian.Uint32(rec.data[:4]))
			p.renderingOriginY = int32(binary.LittleEndian.Uint32(rec.data[4:8]))
		}
	case emfPlusSetAntiAliasMode:
		p.antiAliasMode = uint8(rec.flags & 0xff)
	case emfPlusSetInterpolationMode:
		p.interpolationMode = uint8(rec.flags & 0xff)
	case emfPlusSetPixelOffsetMode:
		p.pixelOffsetMode = uint8(rec.flags & 0xff)
	case emfPlusSetTextRenderingHint:
		p.textRenderingHint = uint8(rec.flags & 0xff)
	case emfPlusSetTextContrast:
		p.textContrast = uint16(rec.flags & 0x0fff)
	case emfPlusSetCompositingMode:
		p.compositingMode = uint8(rec.flags & 0xff)
	case emfPlusSetCompositingQuality:
		p.compositingQuality = uint8(rec.flags & 0xff)
	case emfPlusResetClip:
		p.clips = nil
	case emfPlusSetClipRect:
		p.setClipRect(rec)
	case emfPlusSetClipPath:
		p.setClipPath(rec)
	case emfPlusSetClipRegion:
		p.setClipRegion(rec)
	case emfPlusOffsetClip:
		p.offsetClip(rec)
	}
}

func (p *emfPlusPlayback) combineTransform(t emfTransform, flags uint16) {
	if flags&0x2000 != 0 {
		p.world = emfMultiplyTransform(p.world, t)
	} else {
		p.world = emfMultiplyTransform(t, p.world)
	}
}

func emfPlusTransform(data []byte) (emfTransform, bool) {
	if len(data) < 24 {
		return emfTransform{}, false
	}
	f := func(off int) float64 {
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off : off+4])))
	}
	return emfTransform{m11: f(0), m12: f(4), m21: f(8), m22: f(12), dx: f(16), dy: f(20)}, true
}

func (p *emfPlusPlayback) defineObject(rec emfPlusRecord) {
	id := uint8(rec.flags & 0x00ff)
	kind := uint8((rec.flags >> 8) & 0x007f)
	continued := rec.flags&0x8000 != 0
	if continued {
		if len(rec.data) < 4 {
			return
		}
		total := int(binary.LittleEndian.Uint32(rec.data[:4]))
		part := append([]byte(nil), rec.data[4:]...)
		c := p.continuations[id]
		if c.kind != kind || c.total != total {
			c = emfPlusContinuation{kind: kind, total: total}
		}
		c.data = append(c.data, part...)
		p.continuations[id] = c
		if total > 0 && len(c.data) >= total {
			p.defineObjectData(id, kind, c.data[:total])
			delete(p.continuations, id)
		}
		return
	}
	if c, ok := p.continuations[id]; ok && c.kind == kind {
		c.data = append(c.data, rec.data...)
		data := c.data
		if c.total > 0 && len(data) > c.total {
			data = data[:c.total]
		}
		p.defineObjectData(id, kind, data)
		delete(p.continuations, id)
		return
	}
	p.defineObjectData(id, kind, rec.data)
}

func (p *emfPlusPlayback) defineObjectData(id, kind uint8, data []byte) {
	switch kind {
	case emfPlusObjectBrush:
		if brush, ok := p.emfPlusParseBrush(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, brush: brush}
		}
	case emfPlusObjectPen:
		if pen, ok := p.emfPlusParsePen(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, pen: pen}
		}
	case emfPlusObjectPath:
		if path, ok := emfPlusParsePath(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, path: path}
		}
	case emfPlusObjectRegion:
		if region, ok := emfPlusParseRegion(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, region: region}
		}
	case emfPlusObjectImage:
		if image, ok := emfPlusParseImage(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, image: image}
		}
	case emfPlusObjectFont:
		if font, ok := emfPlusParseFont(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, font: font}
		}
	case emfPlusObjectStringFormat:
		if format, ok := emfPlusParseStringFormat(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, stringFmt: format}
		}
	case emfPlusObjectImageAttributes:
		if attrs, ok := emfPlusParseImageAttributes(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, imageAttrs: attrs}
		}
	case emfPlusObjectCustomLineCap:
		if cap, ok := emfPlusParseCustomLineCap(data); ok {
			p.objects[id] = emfPlusObjectEntry{kind: kind, customCap: cap}
		}
	}
}

func (p *emfPlusPlayback) emfPlusParseBrush(data []byte) (emfPlusBrushObject, bool) {
	if len(data) < 12 {
		return emfPlusBrushObject{}, false
	}
	switch binary.LittleEndian.Uint32(data[4:8]) {
	case 0:
		return emfPlusParseSolidBrush(data)
	case 2: // TextureBrush
		if len(data) < 16 {
			return emfPlusBrushObject{}, false
		}
		flags := binary.LittleEndian.Uint32(data[8:12])
		off := 16
		var t emfTransform
		hasTransform := flags&0x2 != 0
		if hasTransform {
			var ok bool
			if len(data) < off+24 {
				return emfPlusBrushObject{}, false
			}
			t, ok = emfPlusTransform(data[off : off+24])
			if !ok {
				return emfPlusBrushObject{}, false
			}
			off += 24
		}
		img, ok := emfPlusParseImage(data[off:])
		if !ok || img.uri == "" || img.width <= 0 || img.height <= 0 {
			return emfPlusBrushObject{}, false
		}
		p.brushSerial++
		id := fmt.Sprintf("emfplus-pattern-%d", p.brushSerial)
		transform := ""
		if hasTransform {
			transform = fmt.Sprintf(` patternTransform="%s"`, emfPlusSVGMatrix(t))
		}
		fmt.Fprintf(&p.defs, `<pattern id="%s" patternUnits="userSpaceOnUse" width="%d" height="%d"%s><image x="0" y="0" width="%d" height="%d" href="%s"/></pattern>`, id, img.width, img.height, transform, img.width, img.height, img.uri)
		return emfPlusBrushObject{paint: "url(#" + id + ")"}, true
	case 4: // LinearGradientBrush
		if len(data) < 48 {
			return emfPlusBrushObject{}, false
		}
		flags := binary.LittleEndian.Uint32(data[8:12])
		// Preset colors/blend factors alter the color function. Do not flatten them.
		if flags&(0x4|0x8|0x10) != 0 {
			return emfPlusBrushObject{}, false
		}
		r, ok := emfPlusReadRectF(data[16:32])
		if !ok {
			return emfPlusBrushObject{}, false
		}
		start := emfPlusARGB(binary.LittleEndian.Uint32(data[32:36]))
		end := emfPlusARGB(binary.LittleEndian.Uint32(data[36:40]))
		off := 48
		transform := ""
		if flags&0x2 != 0 {
			if len(data) < off+24 {
				return emfPlusBrushObject{}, false
			}
			t, ok := emfPlusTransform(data[off : off+24])
			if !ok {
				return emfPlusBrushObject{}, false
			}
			transform = fmt.Sprintf(` gradientTransform="%s"`, emfPlusSVGMatrix(t))
		}
		p.brushSerial++
		id := fmt.Sprintf("emfplus-gradient-%d", p.brushSerial)
		fmt.Fprintf(&p.defs, `<linearGradient id="%s" gradientUnits="userSpaceOnUse" x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f"%s><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>`, id, r.x, r.y, r.x+r.w, r.y+r.h, transform, start, end)
		return emfPlusBrushObject{paint: "url(#" + id + ")"}, true
	}
	return emfPlusBrushObject{}, false
}

func (p *emfPlusPlayback) brushPaint(flags uint16, id uint32) string {
	if flags&0x8000 != 0 {
		return emfPlusARGB(id)
	}
	obj := p.objects[uint8(id&0xff)]
	if obj.kind == emfPlusObjectBrush {
		return obj.brush.paint
	}
	return ""
}

func (p *emfPlusPlayback) penFor(flags uint16) (emfPlusPenObject, bool) {
	obj := p.objects[uint8(flags&0xff)]
	return obj.pen, obj.kind == emfPlusObjectPen && obj.pen.paint != ""
}

func emfPlusReadRect(data []byte, compressed bool) (emfPlusRectF, int, bool) {
	if compressed {
		if len(data) < 8 {
			return emfPlusRectF{}, 0, false
		}
		return emfPlusRectF{x: float64(int16(binary.LittleEndian.Uint16(data[0:2]))), y: float64(int16(binary.LittleEndian.Uint16(data[2:4]))), w: float64(int16(binary.LittleEndian.Uint16(data[4:6]))), h: float64(int16(binary.LittleEndian.Uint16(data[6:8])))}, 8, true
	}
	r, ok := emfPlusReadRectF(data)
	return r, 16, ok
}

func emfPlusReadPoints(data []byte, count int, compressed bool) ([][2]float64, int, bool) {
	if count <= 0 {
		return nil, 0, false
	}
	step := 8
	if compressed {
		step = 4
	}
	if len(data) < count*step {
		return nil, 0, false
	}
	pts := make([][2]float64, count)
	for i := 0; i < count; i++ {
		if compressed {
			pts[i] = [2]float64{float64(int16(binary.LittleEndian.Uint16(data[i*4 : i*4+2]))), float64(int16(binary.LittleEndian.Uint16(data[i*4+2 : i*4+4])))}
		} else {
			pts[i] = [2]float64{float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*8 : i*8+4]))), float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*8+4 : i*8+8])))}
		}
	}
	return pts, count * step, true
}

func emfPlusReadPointsRecord(data []byte, count int, flags uint16) ([][2]float64, int, bool) {
	if flags&0x0800 != 0 {
		return emfPlusReadRelativePoints(data, count)
	}
	return emfPlusReadPoints(data, count, flags&0x4000 != 0)
}

func (p *emfPlusPlayback) fillRects(rec emfPlusRecord) {
	if len(rec.data) < 8 {
		return
	}
	paint := p.brushPaint(rec.flags, binary.LittleEndian.Uint32(rec.data[:4]))
	if paint == "" {
		return
	}
	n := int(binary.LittleEndian.Uint32(rec.data[4:8]))
	off := 8
	compressed := rec.flags&0x4000 != 0
	for i := 0; i < n; i++ {
		r, used, ok := emfPlusReadRect(rec.data[off:], compressed)
		if !ok {
			return
		}
		off += used
		fmt.Fprintf(&p.body, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="%s" transform="%s"%s/>`, r.x, r.y, r.w, r.h, paint, emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
		p.drawn = true
	}
}
func (p *emfPlusPlayback) drawRects(rec emfPlusRecord) {
	pen, ok := p.penFor(rec.flags)
	if !ok || len(rec.data) < 4 {
		return
	}
	n := int(binary.LittleEndian.Uint32(rec.data[:4]))
	off := 4
	compressed := rec.flags&0x4000 != 0
	for i := 0; i < n; i++ {
		r, used, ok := emfPlusReadRect(rec.data[off:], compressed)
		if !ok {
			return
		}
		off += used
		fmt.Fprintf(&p.body, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, r.x, r.y, r.w, r.h, pen.paint, pen.width, pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
		p.drawn = true
	}
}
func (p *emfPlusPlayback) fillPolygon(rec emfPlusRecord) {
	if len(rec.data) < 8 {
		return
	}
	paint := p.brushPaint(rec.flags, binary.LittleEndian.Uint32(rec.data[:4]))
	if paint == "" {
		return
	}
	n := int(binary.LittleEndian.Uint32(rec.data[4:8]))
	pts, _, ok := emfPlusReadPointsRecord(rec.data[8:], n, rec.flags)
	if !ok {
		return
	}
	var b strings.Builder
	for i, q := range pts {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%.3f,%.3f", q[0], q[1])
	}
	fmt.Fprintf(&p.body, `<polygon points="%s" fill="%s" transform="%s"%s/>`, b.String(), paint, emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func (p *emfPlusPlayback) drawLines(rec emfPlusRecord) {
	pen, ok := p.penFor(rec.flags)
	if !ok || len(rec.data) < 4 {
		return
	}
	n := int(binary.LittleEndian.Uint32(rec.data[:4]))
	pts, _, ok := emfPlusReadPointsRecord(rec.data[4:], n, rec.flags)
	if !ok {
		return
	}
	var b strings.Builder
	for i, q := range pts {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%.3f,%.3f", q[0], q[1])
	}
	fmt.Fprintf(&p.body, `<polyline points="%s" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, b.String(), pen.paint, pen.width, pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func emfPlusCardinalPath(pts [][2]float64, tension float64, closed bool, offset, segments int) (string, bool) {
	if len(pts) < 2 {
		return "", false
	}
	if tension == 0 {
		var b strings.Builder
		start := 0
		end := len(pts) - 1
		if !closed {
			if offset < 0 {
				offset = 0
			}
			if offset > len(pts)-2 {
				return "", false
			}
			start = offset
			if segments <= 0 || start+segments > len(pts)-1 {
				segments = len(pts) - 1 - start
			}
			end = start + segments
		}
		fmt.Fprintf(&b, "M %.3f %.3f", pts[start][0], pts[start][1])
		for i := start + 1; i <= end; i++ {
			fmt.Fprintf(&b, " L %.3f %.3f", pts[i][0], pts[i][1])
		}
		if closed {
			b.WriteString(" Z")
		}
		return b.String(), true
	}
	scale := tension / 3.0
	var b strings.Builder
	if closed {
		n := len(pts)
		fmt.Fprintf(&b, "M %.3f %.3f", pts[0][0], pts[0][1])
		for i := 0; i < n; i++ {
			p0 := pts[(i-1+n)%n]
			p1 := pts[i]
			p2 := pts[(i+1)%n]
			p3 := pts[(i+2)%n]
			c1x := p1[0] + (p2[0]-p0[0])*scale
			c1y := p1[1] + (p2[1]-p0[1])*scale
			c2x := p2[0] - (p3[0]-p1[0])*scale
			c2y := p2[1] - (p3[1]-p1[1])*scale
			fmt.Fprintf(&b, " C %.3f %.3f %.3f %.3f %.3f %.3f", c1x, c1y, c2x, c2y, p2[0], p2[1])
		}
		b.WriteString(" Z")
		return b.String(), true
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(pts)-2 {
		return "", false
	}
	if segments <= 0 || offset+segments > len(pts)-1 {
		segments = len(pts) - 1 - offset
	}
	fmt.Fprintf(&b, "M %.3f %.3f", pts[offset][0], pts[offset][1])
	for i := offset; i < offset+segments; i++ {
		p1 := pts[i]
		p2 := pts[i+1]
		p0 := p1
		p3 := p2
		if i > 0 {
			p0 = pts[i-1]
		}
		if i+2 < len(pts) {
			p3 = pts[i+2]
		}
		c1x := p1[0] + (p2[0]-p0[0])*scale
		c1y := p1[1] + (p2[1]-p0[1])*scale
		c2x := p2[0] - (p3[0]-p1[0])*scale
		c2y := p2[1] - (p3[1]-p1[1])*scale
		fmt.Fprintf(&b, " C %.3f %.3f %.3f %.3f %.3f %.3f", c1x, c1y, c2x, c2y, p2[0], p2[1])
	}
	return b.String(), true
}

func (p *emfPlusPlayback) fillClosedCurve(rec emfPlusRecord) {
	if len(rec.data) < 12 {
		return
	}
	paint := p.brushPaint(rec.flags, binary.LittleEndian.Uint32(rec.data[:4]))
	if paint == "" {
		return
	}
	tension := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[4:8])))
	n := int(binary.LittleEndian.Uint32(rec.data[8:12]))
	pts, _, ok := emfPlusReadPointsRecord(rec.data[12:], n, rec.flags)
	if !ok {
		return
	}
	d, ok := emfPlusCardinalPath(pts, tension, true, 0, 0)
	if !ok {
		return
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="%s" transform="%s"%s/>`, d, paint, emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func (p *emfPlusPlayback) drawClosedCurve(rec emfPlusRecord) {
	pen, ok := p.penFor(rec.flags)
	if !ok || len(rec.data) < 8 {
		return
	}
	tension := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[:4])))
	n := int(binary.LittleEndian.Uint32(rec.data[4:8]))
	pts, _, ok := emfPlusReadPointsRecord(rec.data[8:], n, rec.flags)
	if !ok {
		return
	}
	d, ok := emfPlusCardinalPath(pts, tension, true, 0, 0)
	if !ok {
		return
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, d, pen.paint, pen.width, pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func (p *emfPlusPlayback) drawCurve(rec emfPlusRecord) {
	pen, ok := p.penFor(rec.flags)
	if !ok || len(rec.data) < 16 {
		return
	}
	tension := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[:4])))
	off := int(binary.LittleEndian.Uint32(rec.data[4:8]))
	seg := int(binary.LittleEndian.Uint32(rec.data[8:12]))
	n := int(binary.LittleEndian.Uint32(rec.data[12:16]))
	pts, _, ok := emfPlusReadPointsRecord(rec.data[16:], n, rec.flags)
	if !ok {
		return
	}
	d, ok := emfPlusCardinalPath(pts, tension, false, off, seg)
	if !ok {
		return
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, d, pen.paint, pen.width, pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}

func (p *emfPlusPlayback) fillEllipse(rec emfPlusRecord) {
	if len(rec.data) < 4 {
		return
	}
	paint := p.brushPaint(rec.flags, binary.LittleEndian.Uint32(rec.data[:4]))
	if paint == "" {
		return
	}
	r, _, ok := emfPlusReadRect(rec.data[4:], rec.flags&0x4000 != 0)
	if !ok {
		return
	}
	fmt.Fprintf(&p.body, `<ellipse cx="%.3f" cy="%.3f" rx="%.3f" ry="%.3f" fill="%s" transform="%s"%s/>`, r.x+r.w/2, r.y+r.h/2, math.Abs(r.w/2), math.Abs(r.h/2), paint, emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func (p *emfPlusPlayback) drawEllipse(rec emfPlusRecord) {
	pen, ok := p.penFor(rec.flags)
	if !ok {
		return
	}
	r, _, ok := emfPlusReadRect(rec.data, rec.flags&0x4000 != 0)
	if !ok {
		return
	}
	fmt.Fprintf(&p.body, `<ellipse cx="%.3f" cy="%.3f" rx="%.3f" ry="%.3f" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, r.x+r.w/2, r.y+r.h/2, math.Abs(r.w/2), math.Abs(r.h/2), pen.paint, pen.width, pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func emfPlusArcPath(r emfPlusRectF, start, sweep float64, pie bool) string {
	cx, cy := r.x+r.w/2, r.y+r.h/2
	rx, ry := math.Abs(r.w/2), math.Abs(r.h/2)
	a0 := start * math.Pi / 180
	a1 := (start + sweep) * math.Pi / 180
	x0, y0 := cx+rx*math.Cos(a0), cy+ry*math.Sin(a0)
	x1, y1 := cx+rx*math.Cos(a1), cy+ry*math.Sin(a1)
	large := 0
	if math.Abs(sweep) > 180 {
		large = 1
	}
	sw := 1
	if sweep < 0 {
		sw = 0
	}
	if pie {
		return fmt.Sprintf("M %.3f %.3f L %.3f %.3f A %.3f %.3f 0 %d %d %.3f %.3f Z", cx, cy, x0, y0, rx, ry, large, sw, x1, y1)
	}
	return fmt.Sprintf("M %.3f %.3f A %.3f %.3f 0 %d %d %.3f %.3f", x0, y0, rx, ry, large, sw, x1, y1)
}
func (p *emfPlusPlayback) fillPie(rec emfPlusRecord) {
	if len(rec.data) < 12 {
		return
	}
	paint := p.brushPaint(rec.flags, binary.LittleEndian.Uint32(rec.data[:4]))
	if paint == "" {
		return
	}
	start := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[4:8])))
	sweep := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[8:12])))
	r, _, ok := emfPlusReadRect(rec.data[12:], rec.flags&0x4000 != 0)
	if !ok {
		return
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="%s" stroke="none" transform="%s"%s/>`, emfPlusArcPath(r, start, sweep, true), paint, emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func (p *emfPlusPlayback) drawPie(rec emfPlusRecord) {
	pen, ok := p.penFor(rec.flags)
	if !ok || len(rec.data) < 8 {
		return
	}
	start := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[:4])))
	sweep := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[4:8])))
	r, _, ok := emfPlusReadRect(rec.data[8:], rec.flags&0x4000 != 0)
	if !ok {
		return
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, emfPlusArcPath(r, start, sweep, true), pen.paint, pen.width, pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func (p *emfPlusPlayback) drawArc(rec emfPlusRecord) {
	pen, ok := p.penFor(rec.flags)
	if !ok || len(rec.data) < 8 {
		return
	}
	start := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[:4])))
	sweep := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[4:8])))
	r, _, ok := emfPlusReadRect(rec.data[8:], rec.flags&0x4000 != 0)
	if !ok {
		return
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, emfPlusArcPath(r, start, sweep, false), pen.paint, pen.width, pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}
func (p *emfPlusPlayback) drawBeziers(rec emfPlusRecord) {
	pen, ok := p.penFor(rec.flags)
	if !ok || len(rec.data) < 4 {
		return
	}
	n := int(binary.LittleEndian.Uint32(rec.data[:4]))
	pts, _, ok := emfPlusReadPointsRecord(rec.data[4:], n, rec.flags)
	if !ok || len(pts) < 4 || (len(pts)-1)%3 != 0 {
		return
	}
	var d strings.Builder
	fmt.Fprintf(&d, "M %.3f %.3f", pts[0][0], pts[0][1])
	for i := 1; i+2 < len(pts); i += 3 {
		fmt.Fprintf(&d, " C %.3f %.3f %.3f %.3f %.3f %.3f", pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1], pts[i+2][0], pts[i+2][1])
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, d.String(), pen.paint, pen.width, pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}

func emfPlusParseSolidBrush(data []byte) (emfPlusBrushObject, bool) {
	// EmfPlusBrush: Version, BrushType, BrushData. BrushTypeSolidColor == 0.
	if len(data) < 12 || binary.LittleEndian.Uint32(data[4:8]) != 0 {
		return emfPlusBrushObject{}, false
	}
	return emfPlusBrushObject{paint: emfPlusARGB(binary.LittleEndian.Uint32(data[8:12]))}, true
}

const (
	emfPlusPenDataTransform      = 0x00000001
	emfPlusPenDataStartCap       = 0x00000002
	emfPlusPenDataEndCap         = 0x00000004
	emfPlusPenDataJoin           = 0x00000008
	emfPlusPenDataMiterLimit     = 0x00000010
	emfPlusPenDataLineStyle      = 0x00000020
	emfPlusPenDataDashedLineCap  = 0x00000040
	emfPlusPenDataDashedLineOff  = 0x00000080
	emfPlusPenDataDashedLine     = 0x00000100
	emfPlusPenDataNonCenter      = 0x00000200
	emfPlusPenDataCompoundLine   = 0x00000400
	emfPlusPenDataCustomStartCap = 0x00000800
	emfPlusPenDataCustomEndCap   = 0x00001000
)

func emfPlusLineCapSVG(cap int32) string {
	switch cap {
	case 1:
		return "square"
	case 2:
		return "round"
	default:
		return "butt"
	}
}

func emfPlusLineJoinSVG(join int32) string {
	switch join {
	case 1:
		return "bevel"
	case 2:
		return "round"
	default:
		return "miter"
	}
}

func (p *emfPlusPlayback) definePenCapMarker(cap int32, paint string, start bool) string {
	var shape string
	switch cap {
	case 3, 0x14: // Triangle / ArrowAnchor
		shape = fmt.Sprintf(`<path d="M 0 -3 L 6 0 L 0 3 Z" fill="%s"/>`, paint)
	case 0x11: // SquareAnchor
		shape = fmt.Sprintf(`<rect x="0" y="-3" width="6" height="6" fill="%s"/>`, paint)
	case 0x12: // RoundAnchor
		shape = fmt.Sprintf(`<circle cx="3" cy="0" r="3" fill="%s"/>`, paint)
	case 0x13: // DiamondAnchor
		shape = fmt.Sprintf(`<path d="M 0 0 L 3 -3 L 6 0 L 3 3 Z" fill="%s"/>`, paint)
	default:
		return ""
	}
	p.brushSerial++
	id := fmt.Sprintf("emfplus-pen-cap-%d", p.brushSerial)
	orient := "auto"
	refX := 0.0
	if start {
		orient = "auto-start-reverse"
		refX = 0
	} else {
		refX = 6
	}
	fmt.Fprintf(&p.defs, `<marker id="%s" markerUnits="strokeWidth" markerWidth="6" markerHeight="6" refX="%.1f" refY="0" orient="%s" viewBox="0 -3 6 6">%s</marker>`, id, refX, orient, shape)
	return id
}

func emfPlusParseCustomLineCap(data []byte) (emfPlusCustomLineCapObject, bool) {
	if len(data) < 8 {
		return emfPlusCustomLineCapObject{}, false
	}
	typ := int32(binary.LittleEndian.Uint32(data[4:8]))
	buf := data[8:]
	switch typ {
	case 1:
		if len(buf) < 52 {
			return emfPlusCustomLineCapObject{}, false
		}
		f32 := func(off int) float64 {
			return float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[off : off+4])))
		}
		w, h := f32(0), f32(4)
		filled := binary.LittleEndian.Uint32(buf[12:16]) != 0
		join := emfPlusLineJoinSVG(int32(binary.LittleEndian.Uint32(buf[24:28])))
		miter := f32(28)
		widthScale := f32(32)
		if w <= 0 {
			w = 3
		}
		if h <= 0 {
			h = 3
		}
		if widthScale <= 0 {
			widthScale = 1
		}
		path := fmt.Sprintf("M 0 %.3f L %.3f 0 L 0 %.3f Z", -h/2, w, h/2)
		return emfPlusCustomLineCapObject{markerPath: path, filled: filled, width: w, height: h, widthScale: widthScale, lineJoin: join, miterLimit: miter}, true
	case 0:
		if len(buf) < 48 {
			return emfPlusCustomLineCapObject{}, false
		}
		flags := binary.LittleEndian.Uint32(buf[0:4])
		cap := emfPlusCustomLineCapObject{
			baseInset:  float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[8:12]))),
			lineJoin:   emfPlusLineJoinSVG(int32(binary.LittleEndian.Uint32(buf[20:24]))),
			miterLimit: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[24:28]))),
			widthScale: float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[28:32]))),
		}
		if cap.widthScale <= 0 {
			cap.widthScale = 1
		}
		off := 48
		if flags&0x1 != 0 {
			if off+4 > len(buf) {
				return emfPlusCustomLineCapObject{}, false
			}
			n := int(int32(binary.LittleEndian.Uint32(buf[off : off+4])))
			off += 4
			if n < 0 || off+n > len(buf) {
				return emfPlusCustomLineCapObject{}, false
			}
			if path, ok := emfPlusParsePath(buf[off : off+n]); ok {
				cap.markerPath = path.d
				cap.filled = true
			}
			off += n
		}
		if flags&0x2 != 0 {
			if off+4 > len(buf) {
				return emfPlusCustomLineCapObject{}, false
			}
			n := int(int32(binary.LittleEndian.Uint32(buf[off : off+4])))
			off += 4
			if n < 0 || off+n > len(buf) {
				return emfPlusCustomLineCapObject{}, false
			}
			if cap.markerPath == "" {
				if path, ok := emfPlusParsePath(buf[off : off+n]); ok {
					cap.markerPath = path.d
				}
			}
		}
		if cap.markerPath == "" {
			return emfPlusCustomLineCapObject{}, false
		}
		cap.width, cap.height = 16, 16
		return cap, true
	}
	return emfPlusCustomLineCapObject{}, false
}

func (p *emfPlusPlayback) defineCustomCapMarker(cap emfPlusCustomLineCapObject, paint string, start bool) string {
	if cap.markerPath == "" {
		return ""
	}
	p.brushSerial++
	id := fmt.Sprintf("emfplus-custom-cap-%d", p.brushSerial)
	orient := "auto"
	if start {
		orient = "auto-start-reverse"
	}
	w, h := cap.width, cap.height
	if w <= 0 {
		w = 16
	}
	if h <= 0 {
		h = 16
	}
	fill, stroke := "none", paint
	if cap.filled {
		fill, stroke = paint, "none"
	}
	join := cap.lineJoin
	if join == "" {
		join = "miter"
	}
	miter := cap.miterLimit
	if miter <= 0 {
		miter = 10
	}
	fmt.Fprintf(&p.defs, `<marker id="%s" markerUnits="strokeWidth" markerWidth="%.3f" markerHeight="%.3f" refX="%.3f" refY="0" orient="%s" viewBox="%.3f %.3f %.3f %.3f" overflow="visible"><path d="%s" fill="%s" stroke="%s" stroke-width="1" stroke-linejoin="%s" stroke-miterlimit="%.3f"/></marker>`, id, w*cap.widthScale, h*cap.widthScale, cap.baseInset, orient, -w/2, -h/2, w, h, cap.markerPath, fill, stroke, join, miter)
	return id
}

func (p *emfPlusPlayback) emfPlusParsePen(data []byte) (emfPlusPenObject, bool) {
	if len(data) < 32 || binary.LittleEndian.Uint32(data[4:8]) != 0 {
		return emfPlusPenObject{}, false
	}
	flags := binary.LittleEndian.Uint32(data[8:12])
	width := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[16:20])))
	if width <= 0 {
		width = 1
	}
	pen := emfPlusPenObject{width: width, lineCap: "butt", lineJoin: "miter", miterLimit: 10}
	off := 20
	need := func(n int) bool { return off+n <= len(data) }
	readI32 := func() (int32, bool) {
		if !need(4) {
			return 0, false
		}
		v := int32(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		return v, true
	}
	readF32 := func() (float64, bool) {
		if !need(4) {
			return 0, false
		}
		v := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off : off+4])))
		off += 4
		return v, true
	}
	if flags&emfPlusPenDataTransform != 0 {
		if !need(24) {
			return emfPlusPenObject{}, false
		}
		// Pen transform affects the pen coordinate system. SVG has no independent
		// stroke transform, so preserve ownership metadata by consuming it here;
		// geometry remains under the record/world transform.
		off += 24
	}
	startCap, endCap := int32(0), int32(0)
	if flags&emfPlusPenDataStartCap != 0 {
		v, ok := readI32()
		if !ok {
			return emfPlusPenObject{}, false
		}
		startCap = v
		pen.lineCap = emfPlusLineCapSVG(v)
	}
	if flags&emfPlusPenDataEndCap != 0 {
		v, ok := readI32()
		if !ok {
			return emfPlusPenObject{}, false
		}
		endCap = v
		if flags&emfPlusPenDataStartCap == 0 {
			pen.lineCap = emfPlusLineCapSVG(v)
		}
	}
	if flags&emfPlusPenDataJoin != 0 {
		v, ok := readI32()
		if !ok {
			return emfPlusPenObject{}, false
		}
		pen.lineJoin = emfPlusLineJoinSVG(v)
	}
	if flags&emfPlusPenDataMiterLimit != 0 {
		v, ok := readF32()
		if !ok {
			return emfPlusPenObject{}, false
		}
		if v > 0 {
			pen.miterLimit = v
		}
	}
	lineStyle := int32(0)
	if flags&emfPlusPenDataLineStyle != 0 {
		v, ok := readI32()
		if !ok {
			return emfPlusPenObject{}, false
		}
		lineStyle = v
	}
	if flags&emfPlusPenDataDashedLineCap != 0 {
		if _, ok := readI32(); !ok {
			return emfPlusPenObject{}, false
		}
	}
	if flags&emfPlusPenDataDashedLineOff != 0 {
		v, ok := readF32()
		if !ok {
			return emfPlusPenObject{}, false
		}
		pen.dashOffset = v * width
	}
	if flags&emfPlusPenDataDashedLine != 0 {
		if !need(4) {
			return emfPlusPenObject{}, false
		}
		count := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if count < 0 || !need(count*4) {
			return emfPlusPenObject{}, false
		}
		pen.dashArray = make([]float64, count)
		for i := range pen.dashArray {
			pen.dashArray[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off+i*4:off+i*4+4]))) * width
		}
		off += count * 4
	} else {
		switch lineStyle {
		case 1:
			pen.dashArray = []float64{3 * width, width}
		case 2:
			pen.dashArray = []float64{width, width}
		case 3:
			pen.dashArray = []float64{3 * width, width, width, width}
		case 4:
			pen.dashArray = []float64{3 * width, width, width, width, width, width}
		}
	}
	if flags&emfPlusPenDataNonCenter != 0 {
		if _, ok := readI32(); !ok {
			return emfPlusPenObject{}, false
		}
	}
	if flags&emfPlusPenDataCompoundLine != 0 {
		if !need(4) {
			return emfPlusPenObject{}, false
		}
		count := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if count < 2 || !need(count*4) {
			return emfPlusPenObject{}, false
		}
		pen.compoundLine = make([]float64, count)
		last := -1.0
		for i := range pen.compoundLine {
			v := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off+i*4 : off+i*4+4])))
			if v < 0 || v > 1 || v < last {
				return emfPlusPenObject{}, false
			}
			pen.compoundLine[i] = v
			last = v
		}
		off += count * 4
	}
	var customStart, customEnd *emfPlusCustomLineCapObject
	readCustomCap := func() (*emfPlusCustomLineCapObject, bool) {
		if !need(4) {
			return nil, false
		}
		n := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if n <= 0 || !need(n) {
			return nil, false
		}
		cap, ok := emfPlusParseCustomLineCap(data[off : off+n])
		off += n
		if !ok {
			return nil, false
		}
		return &cap, true
	}
	if flags&emfPlusPenDataCustomStartCap != 0 {
		var ok bool
		customStart, ok = readCustomCap()
		if !ok {
			return emfPlusPenObject{}, false
		}
	}
	if flags&emfPlusPenDataCustomEndCap != 0 {
		var ok bool
		customEnd, ok = readCustomCap()
		if !ok {
			return emfPlusPenObject{}, false
		}
	}
	brush, ok := p.emfPlusParseBrush(data[off:])
	if !ok {
		return emfPlusPenObject{}, false
	}
	pen.paint = brush.paint
	pen.startMarker = p.definePenCapMarker(startCap, pen.paint, true)
	pen.endMarker = p.definePenCapMarker(endCap, pen.paint, false)
	if customStart != nil {
		pen.startMarker = p.defineCustomCapMarker(*customStart, pen.paint, true)
	}
	if customEnd != nil {
		pen.endMarker = p.defineCustomCapMarker(*customEnd, pen.paint, false)
	}
	return pen, true
}

func (pen emfPlusPenObject) svgAttrs() string {
	var b strings.Builder
	if pen.lineCap != "" {
		fmt.Fprintf(&b, ` stroke-linecap="%s"`, pen.lineCap)
	}
	if pen.lineJoin != "" {
		fmt.Fprintf(&b, ` stroke-linejoin="%s"`, pen.lineJoin)
	}
	if pen.miterLimit > 0 {
		fmt.Fprintf(&b, ` stroke-miterlimit="%.3f"`, pen.miterLimit)
	}
	if len(pen.dashArray) > 0 {
		parts := make([]string, len(pen.dashArray))
		for i, v := range pen.dashArray {
			parts[i] = fmt.Sprintf("%.3f", v)
		}
		fmt.Fprintf(&b, ` stroke-dasharray="%s"`, strings.Join(parts, " "))
		if pen.dashOffset != 0 {
			fmt.Fprintf(&b, ` stroke-dashoffset="%.3f"`, pen.dashOffset)
		}
	}
	if pen.startMarker != "" {
		fmt.Fprintf(&b, ` marker-start="url(#%s)"`, pen.startMarker)
	}
	if pen.endMarker != "" {
		fmt.Fprintf(&b, ` marker-end="url(#%s)"`, pen.endMarker)
	}
	if len(pen.compoundLine) > 0 {
		parts := make([]string, len(pen.compoundLine))
		for i, v := range pen.compoundLine {
			parts[i] = fmt.Sprintf("%.6f", v)
		}
		fmt.Fprintf(&b, ` data-emfplus-compound-line="%s"`, strings.Join(parts, ","))
	}
	return b.String()
}

func emfPlusReadRelativeInt(data []byte) (int, int, bool) {
	if len(data) == 0 {
		return 0, 0, false
	}
	if data[0]&0x80 == 0 { // EmfPlusInteger7: low seven bits are signed.
		v := int(data[0] & 0x7f)
		if v&0x40 != 0 {
			v -= 0x80
		}
		return v, 1, true
	}
	if len(data) < 2 {
		return 0, 0, false
	}
	// EmfPlusInteger15: the high bit marks the two-byte form; the remaining
	// fifteen bits are a signed integer, little-endian in 7+8 chunks.
	v := int(data[0]&0x7f) | int(data[1])<<7
	if v&0x4000 != 0 {
		v -= 0x8000
	}
	return v, 2, true
}

func emfPlusReadRelativePoints(data []byte, count int) ([][2]float64, int, bool) {
	if count <= 0 {
		return nil, 0, false
	}
	pts := make([][2]float64, count)
	off := 0
	x, y := 0, 0
	for i := 0; i < count; i++ {
		dx, used, ok := emfPlusReadRelativeInt(data[off:])
		if !ok {
			return nil, 0, false
		}
		off += used
		dy, used, ok := emfPlusReadRelativeInt(data[off:])
		if !ok {
			return nil, 0, false
		}
		off += used
		x += dx
		y += dy
		pts[i] = [2]float64{float64(x), float64(y)}
	}
	return pts, off, true
}

func emfPlusReadPathPointTypesRLE(data []byte, count int) ([]byte, int, bool) {
	if count <= 0 {
		return nil, 0, false
	}
	out := make([]byte, 0, count)
	off := 0
	for len(out) < count {
		if off+2 > len(data) {
			return nil, 0, false
		}
		run := int(data[off] & 0x3f)
		bezier := data[off]&0x80 != 0
		typ := data[off+1]
		off += 2
		if run <= 0 || len(out)+run > count {
			return nil, 0, false
		}
		if bezier {
			typ = (typ & 0xf0) | 0x03
		}
		for i := 0; i < run; i++ {
			out = append(out, typ)
		}
	}
	return out, off, true
}

func emfPlusParsePath(data []byte) (emfPlusPathObject, bool) {
	if len(data) < 12 {
		return emfPlusPathObject{}, false
	}
	count := int(binary.LittleEndian.Uint32(data[4:8]))
	flags := binary.LittleEndian.Uint32(data[8:12])
	if count <= 0 {
		return emfPlusPathObject{}, false
	}
	off := 12
	var points [][2]float64
	if flags&0x0800 != 0 { // R: relative PointR coordinates + RLE point types.
		var used int
		var ok bool
		points, used, ok = emfPlusReadRelativePoints(data[off:], count)
		if !ok {
			return emfPlusPathObject{}, false
		}
		off += used
	} else if flags&0x4000 != 0 { // compressed absolute int16 points
		if off+count*4 > len(data) {
			return emfPlusPathObject{}, false
		}
		points = make([][2]float64, count)
		for i := range points {
			points[i][0] = float64(int16(binary.LittleEndian.Uint16(data[off+i*4 : off+i*4+2])))
			points[i][1] = float64(int16(binary.LittleEndian.Uint16(data[off+i*4+2 : off+i*4+4])))
		}
		off += count * 4
	} else {
		if off+count*8 > len(data) {
			return emfPlusPathObject{}, false
		}
		points = make([][2]float64, count)
		for i := range points {
			points[i][0] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off+i*8 : off+i*8+4])))
			points[i][1] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off+i*8+4 : off+i*8+8])))
		}
		off += count * 8
	}
	var types []byte
	if flags&0x0800 != 0 {
		var used int
		var ok bool
		types, used, ok = emfPlusReadPathPointTypesRLE(data[off:], count)
		if !ok {
			return emfPlusPathObject{}, false
		}
		off += used
	} else {
		if off+count > len(data) {
			return emfPlusPathObject{}, false
		}
		types = data[off : off+count]
	}
	var d strings.Builder
	for i := 0; i < count; {
		typ := types[i] & 0x0f
		closeFigure := types[i]&0x80 != 0
		switch typ {
		case 0: // Start
			fmt.Fprintf(&d, "M %.3f %.3f", points[i][0], points[i][1])
			i++
		case 1: // Line
			fmt.Fprintf(&d, " L %.3f %.3f", points[i][0], points[i][1])
			i++
		case 3: // Bezier uses groups of three points
			if i+2 >= count || types[i+1]&0x0f != 3 || types[i+2]&0x0f != 3 {
				return emfPlusPathObject{}, false
			}
			fmt.Fprintf(&d, " C %.3f %.3f %.3f %.3f %.3f %.3f", points[i][0], points[i][1], points[i+1][0], points[i+1][1], points[i+2][0], points[i+2][1])
			closeFigure = types[i+2]&0x80 != 0
			i += 3
		default:
			return emfPlusPathObject{}, false
		}
		if closeFigure {
			d.WriteString(" Z")
		}
	}
	return emfPlusPathObject{d: strings.TrimSpace(d.String())}, d.Len() > 0
}

func (p *emfPlusPlayback) fillPath(rec emfPlusRecord) {
	path := p.objects[uint8(rec.flags&0xff)]
	if path.kind != emfPlusObjectPath || path.path.d == "" || len(rec.data) < 4 {
		return
	}
	brushID := binary.LittleEndian.Uint32(rec.data[:4])
	paint := ""
	if rec.flags&0x8000 != 0 {
		paint = emfPlusARGB(brushID)
	} else if brush := p.objects[uint8(brushID&0xff)]; brush.kind == emfPlusObjectBrush {
		paint = brush.brush.paint
	}
	if paint == "" {
		return
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="%s" stroke="none" transform="%s"%s/>`, path.path.d, paint, emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}

func (p *emfPlusPlayback) drawPath(rec emfPlusRecord) {
	path := p.objects[uint8(rec.flags&0xff)]
	if path.kind != emfPlusObjectPath || path.path.d == "" || len(rec.data) < 4 {
		return
	}
	penID := uint8(binary.LittleEndian.Uint32(rec.data[:4]) & 0xff)
	pen := p.objects[penID]
	if pen.kind != emfPlusObjectPen || pen.pen.paint == "" {
		return
	}
	fmt.Fprintf(&p.body, `<path d="%s" fill="none" stroke="%s" stroke-width="%.3f"%s transform="%s"%s/>`, path.path.d, pen.pen.paint, pen.pen.width, pen.pen.svgAttrs(), emfPlusSVGMatrix(p.effectiveTransform()), p.clipAttr())
	p.drawn = true
}

func (p *emfPlusPlayback) snapshot() emfPlusState {
	return emfPlusState{world: p.world, pageUnit: p.pageUnit, pageScale: p.pageScale, clips: append([]string(nil), p.clips...), compositingMode: p.compositingMode, compositingQuality: p.compositingQuality, textRenderingHint: p.textRenderingHint, textContrast: p.textContrast, renderingOriginX: p.renderingOriginX, renderingOriginY: p.renderingOriginY, antiAliasMode: p.antiAliasMode, interpolationMode: p.interpolationMode, pixelOffsetMode: p.pixelOffsetMode}
}

func (p *emfPlusPlayback) saveState(rec emfPlusRecord) {
	if len(rec.data) < 4 {
		return
	}
	p.saved[binary.LittleEndian.Uint32(rec.data[:4])] = p.snapshot()
}

func (p *emfPlusPlayback) restoreState(rec emfPlusRecord) {
	if len(rec.data) < 4 {
		return
	}
	idx := binary.LittleEndian.Uint32(rec.data[:4])
	if state, ok := p.saved[idx]; ok {
		p.world = state.world
		p.pageUnit = state.pageUnit
		p.pageScale = state.pageScale
		p.clips = append([]string(nil), state.clips...)
		p.compositingMode = state.compositingMode
		p.compositingQuality = state.compositingQuality
		p.textRenderingHint = state.textRenderingHint
		p.textContrast = state.textContrast
		p.renderingOriginX = state.renderingOriginX
		p.renderingOriginY = state.renderingOriginY
		p.antiAliasMode = state.antiAliasMode
		p.interpolationMode = state.interpolationMode
		p.pixelOffsetMode = state.pixelOffsetMode
		delete(p.saved, idx)
	}
}

func (p *emfPlusPlayback) beginContainer(rec emfPlusRecord) {
	if len(rec.data) < 36 {
		return
	}
	dst, ok1 := emfPlusReadRectF(rec.data[:16])
	src, ok2 := emfPlusReadRectF(rec.data[16:32])
	idx := binary.LittleEndian.Uint32(rec.data[32:36])
	if !ok1 || !ok2 || math.Abs(dst.w) < 1e-9 || math.Abs(dst.h) < 1e-9 {
		return
	}
	p.saved[idx] = p.snapshot()
	// EMF+ defines the container transform by mapping DestRect into SrcRect.
	t := emfTransform{m11: src.w / dst.w, m22: src.h / dst.h}
	t.dx = src.x - dst.x*t.m11
	t.dy = src.y - dst.y*t.m22
	p.world = emfMultiplyTransform(t, p.world)
}

func (p *emfPlusPlayback) clipAttr() string {
	var b strings.Builder
	if len(p.clips) > 0 {
		fmt.Fprintf(&b, ` mask="url(#%s)"`, p.clips[len(p.clips)-1])
	}
	b.WriteString(p.compositingAttr())
	b.WriteString(p.graphicsQualityAttrs())
	return b.String()
}

func emfPlusMaskCanvas(fill string) string {
	return fmt.Sprintf(`<rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="%s"/>`, fill)
}

func (p *emfPlusPlayback) defineLeafMask(element string) string {
	p.clipSerial++
	id := fmt.Sprintf("emfplus-mask-%d", p.clipSerial)
	fmt.Fprintf(&p.defs, `<mask id="%s" maskUnits="userSpaceOnUse" x="-1000000" y="-1000000" width="2000000" height="2000000"><rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="black"/><g fill="white" stroke="none">%s</g></mask>`, id, element)
	return id
}

// combineMasks implements GDI+ CombineMode semantics in mask space.
// 0=Replace, 1=Intersect, 2=Union, 3=Xor, 4=Exclude (left-right),
// 5=Complement (right-left).
func (p *emfPlusPlayback) combineMasks(left, right string, mode uint16) string {
	if right == "" {
		return left
	}
	if mode == 0 || left == "" {
		return right
	}
	p.clipSerial++
	id := fmt.Sprintf("emfplus-mask-%d", p.clipSerial)
	canvas := emfPlusMaskCanvas("white")
	fmt.Fprintf(&p.defs, `<mask id="%s" maskUnits="userSpaceOnUse" x="-1000000" y="-1000000" width="2000000" height="2000000"><rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="black"/>`, id)
	switch mode {
	case 1: // intersection
		fmt.Fprintf(&p.defs, `<g mask="url(#%s)"><g mask="url(#%s)">%s</g></g>`, left, right, canvas)
	case 2: // union
		fmt.Fprintf(&p.defs, `<g mask="url(#%s)">%s</g><g mask="url(#%s)">%s</g>`, left, canvas, right, canvas)
	case 3: // xor = union - intersection
		fmt.Fprintf(&p.defs, `<g mask="url(#%s)">%s</g><g mask="url(#%s)">%s</g><g mask="url(#%s)"><g mask="url(#%s)"><rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="black"/></g></g>`, left, canvas, right, canvas, left, right)
	case 4: // exclude = left - right
		fmt.Fprintf(&p.defs, `<g mask="url(#%s)">%s</g><g mask="url(#%s)"><rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="black"/></g>`, left, canvas, right)
	case 5: // complement = right - left
		fmt.Fprintf(&p.defs, `<g mask="url(#%s)">%s</g><g mask="url(#%s)"><rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="black"/></g>`, right, canvas, left)
	default:
		return left
	}
	p.defs.WriteString(`</mask>`)
	return id
}

func (p *emfPlusPlayback) setCombinedClip(right string, mode uint16) {
	left := ""
	if len(p.clips) > 0 {
		left = p.clips[len(p.clips)-1]
	}
	id := p.combineMasks(left, right, mode)
	if id == "" {
		return
	}
	if mode == 0 {
		p.clips = nil
	}
	p.clips = append(p.clips, id)
}

func emfPlusCombineMode(flags uint16) uint16 { return (flags >> 8) & 0x0f }

func (p *emfPlusPlayback) setClipRect(rec emfPlusRecord) {
	r, ok := emfPlusReadRectF(rec.data)
	if !ok {
		return
	}
	mode := emfPlusCombineMode(rec.flags)
	if mode > 5 {
		return
	}
	element := fmt.Sprintf(`<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" transform="%s"/>`, r.x, r.y, r.w, r.h, emfPlusSVGMatrix(p.effectiveTransform()))
	p.setCombinedClip(p.defineLeafMask(element), mode)
}

func (p *emfPlusPlayback) setClipPath(rec emfPlusRecord) {
	path := p.objects[uint8(rec.flags&0xff)]
	if path.kind != emfPlusObjectPath || path.path.d == "" {
		return
	}
	mode := emfPlusCombineMode(rec.flags)
	if mode > 5 {
		return
	}
	element := fmt.Sprintf(`<path d="%s" transform="%s"/>`, path.path.d, emfPlusSVGMatrix(p.effectiveTransform()))
	p.setCombinedClip(p.defineLeafMask(element), mode)
}

func (p *emfPlusPlayback) offsetClip(rec emfPlusRecord) {
	if len(rec.data) < 8 || len(p.clips) == 0 {
		return
	}
	dx := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[0:4])))
	dy := float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[4:8])))
	old := p.clips[len(p.clips)-1]
	p.clipSerial++
	id := fmt.Sprintf("emfplus-mask-%d", p.clipSerial)
	fmt.Fprintf(&p.defs, `<mask id="%s" maskUnits="userSpaceOnUse" x="-1000000" y="-1000000" width="2000000" height="2000000"><rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="black"/><g transform="translate(%.3f %.3f)" mask="url(#%s)">%s</g></mask>`, id, dx, dy, old, emfPlusMaskCanvas("white"))
	p.clips = append(p.clips, id)
}

func emfPlusParseRegion(data []byte) (emfPlusRegionObject, bool) {
	if len(data) < 12 {
		return emfPlusRegionObject{}, false
	}
	// Version + RegionNodeCount precede the root node. RegionNodeCount counts
	// child nodes; parsing is driven by the recursively encoded node tree.
	root, used, ok := emfPlusParseRegionNode(data[8:])
	if !ok || used <= 0 || root == nil {
		return emfPlusRegionObject{}, false
	}
	return emfPlusRegionObject{root: root}, true
}

func emfPlusParseRegionNode(data []byte) (*emfPlusRegionNode, int, bool) {
	if len(data) < 4 {
		return nil, 0, false
	}
	typ := binary.LittleEndian.Uint32(data[:4])
	n := &emfPlusRegionNode{typ: typ}
	switch typ {
	case 1, 2, 3, 4, 5: // And, Or, Xor, Exclude, Complement
		left, lu, ok := emfPlusParseRegionNode(data[4:])
		if !ok {
			return nil, 0, false
		}
		right, ru, ok := emfPlusParseRegionNode(data[4+lu:])
		if !ok {
			return nil, 0, false
		}
		n.left, n.right = left, right
		return n, 4 + lu + ru, true
	case 0x10000000: // Rect
		r, ok := emfPlusReadRectF(data[4:])
		if !ok {
			return nil, 0, false
		}
		n.rect = r
		return n, 20, true
	case 0x10000001: // Path
		if len(data) < 8 {
			return nil, 0, false
		}
		ln := int(int32(binary.LittleEndian.Uint32(data[4:8])))
		if ln <= 0 || 8+ln > len(data) {
			return nil, 0, false
		}
		path, ok := emfPlusParsePath(data[8 : 8+ln])
		if !ok {
			return nil, 0, false
		}
		n.path = path
		return n, 8 + ln, true
	case 0x10000002, 0x10000003: // Empty, Infinite
		return n, 4, true
	default:
		return nil, 0, false
	}
}

func emfPlusGUID(data []byte) (string, bool) {
	if len(data) < 16 {
		return "", false
	}
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		binary.LittleEndian.Uint32(data[0:4]), binary.LittleEndian.Uint16(data[4:6]), binary.LittleEndian.Uint16(data[6:8]),
		data[8], data[9], data[10], data[11], data[12], data[13], data[14], data[15]), true
}

func (p *emfPlusPlayback) strokeFillPath(rec emfPlusRecord) {
	// RecordType 0x4037 is enumerated by MS-EMFPLUS, but the current normative
	// specification does not define its record payload. Do not guess a wire
	// structure and risk consuming subsequent records incorrectly.
}

func (p *emfPlusPlayback) serializableObject(rec emfPlusRecord) {
	if len(rec.data) < 20 {
		p.effectFilter = ""
		return
	}
	guid, ok := emfPlusGUID(rec.data[:16])
	if !ok {
		p.effectFilter = ""
		return
	}
	n := int(binary.LittleEndian.Uint32(rec.data[16:20]))
	if n < 0 || 20+n > len(rec.data) {
		p.effectFilter = ""
		return
	}
	buf := rec.data[20 : 20+n]
	p.brushSerial++
	id := fmt.Sprintf("emfplus-effect-%d", p.brushSerial)
	switch guid {
	case "633c80a4-1843-482b-9ef2-be2834c5fdd4": // BlurEffect
		if len(buf) < 8 {
			p.effectFilter = ""
			return
		}
		radius := float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[:4])))
		if radius < 0 {
			radius = 0
		}
		fmt.Fprintf(&p.defs, `<filter id="%s" x="-50%%" y="-50%%" width="200%%" height="200%%"><feGaussianBlur stdDeviation="%.3f"/></filter>`, id, radius/2)
	case "718f2615-7933-40e3-a511-5f68fe14dd74": // ColorMatrixEffect
		if len(buf) < 25*4 {
			p.effectFilter = ""
			return
		}
		m := make([]float64, 25)
		for i := range m {
			m[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4 : i*4+4])))
		}
		vals := []float64{m[0], m[5], m[10], m[15], m[20], m[1], m[6], m[11], m[16], m[21], m[2], m[7], m[12], m[17], m[22], m[3], m[8], m[13], m[18], m[23]}
		parts := make([]string, len(vals))
		for i, v := range vals {
			parts[i] = fmt.Sprintf("%.6f", v)
		}
		fmt.Fprintf(&p.defs, `<filter id="%s"><feColorMatrix type="matrix" values="%s"/></filter>`, id, strings.Join(parts, " "))
	case "d3a1dbe1-8ec4-4c17-9f4c-ea97ad1c343d": // BrightnessContrastEffect
		if len(buf) < 8 {
			p.effectFilter = ""
			return
		}
		brightness := float64(int32(binary.LittleEndian.Uint32(buf[0:4]))) / 255.0
		contrast := float64(int32(binary.LittleEndian.Uint32(buf[4:8])))
		if contrast < -100 {
			contrast = -100
		}
		if contrast > 100 {
			contrast = 100
		}
		slope := 1 + contrast/100
		intercept := brightness + .5*(1-slope)
		fmt.Fprintf(&p.defs, `<filter id="%s"><feComponentTransfer><feFuncR type="linear" slope="%.6f" intercept="%.6f"/><feFuncG type="linear" slope="%.6f" intercept="%.6f"/><feFuncB type="linear" slope="%.6f" intercept="%.6f"/></feComponentTransfer></filter>`, id, slope, intercept, slope, intercept, slope, intercept)
	case "537e597d-251e-48da-9664-29ca496b70f8": // ColorBalanceEffect
		if len(buf) < 12 {
			p.effectFilter = ""
			return
		}
		r := float64(int32(binary.LittleEndian.Uint32(buf[0:4]))) / 200.0
		g := float64(int32(binary.LittleEndian.Uint32(buf[4:8]))) / 200.0
		b := float64(int32(binary.LittleEndian.Uint32(buf[8:12]))) / 200.0
		fmt.Fprintf(&p.defs, `<filter id="%s"><feComponentTransfer><feFuncR type="linear" slope="1" intercept="%.6f"/><feFuncG type="linear" slope="1" intercept="%.6f"/><feFuncB type="linear" slope="1" intercept="%.6f"/></feComponentTransfer></filter>`, id, r, g, b)
	case "8b2dd6c3-eb07-4d87-a5f0-7108e26a9c5f": // HueSaturationLightnessEffect
		if len(buf) < 12 {
			p.effectFilter = ""
			return
		}
		hue := float64(int32(binary.LittleEndian.Uint32(buf[0:4])))
		sat := 1 + float64(int32(binary.LittleEndian.Uint32(buf[4:8])))/100
		if sat < 0 {
			sat = 0
		}
		light := float64(int32(binary.LittleEndian.Uint32(buf[8:12]))) / 200.0
		fmt.Fprintf(&p.defs, `<filter id="%s"><feColorMatrix type="hueRotate" values="%.3f"/><feColorMatrix type="saturate" values="%.6f"/><feComponentTransfer><feFuncR type="linear" slope="1" intercept="%.6f"/><feFuncG type="linear" slope="1" intercept="%.6f"/><feFuncB type="linear" slope="1" intercept="%.6f"/></feComponentTransfer></filter>`, id, hue, sat, light, light, light)
	case "99c354ec-2a31-4f3a-8c34-17a803b33a25": // LevelsEffect
		if len(buf) < 12 {
			p.effectFilter = ""
			return
		}
		high := float64(int32(binary.LittleEndian.Uint32(buf[0:4])))
		mid := float64(int32(binary.LittleEndian.Uint32(buf[4:8])))
		shadow := float64(int32(binary.LittleEndian.Uint32(buf[8:12])))
		gamma := 1.0
		if mid != 0 {
			gamma = math.Pow(2, -mid/100)
		}
		slope := 1 + (100-high+shadow)/400
		intercept := -shadow / 500
		fmt.Fprintf(&p.defs, `<filter id="%s"><feComponentTransfer><feFuncR type="gamma" amplitude="%.6f" exponent="%.6f" offset="%.6f"/><feFuncG type="gamma" amplitude="%.6f" exponent="%.6f" offset="%.6f"/><feFuncB type="gamma" amplitude="%.6f" exponent="%.6f" offset="%.6f"/></feComponentTransfer></filter>`, id, slope, gamma, intercept, slope, gamma, intercept, slope, gamma, intercept)
	case "63cbf3ee-c526-402c-8f71-62c540bf5142": // SharpenEffect
		if len(buf) < 8 {
			p.effectFilter = ""
			return
		}
		radius := float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[0:4])))
		amount := float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[4:8])))
		if amount < 0 {
			amount = 0
		}
		if amount > 100 {
			amount = 100
		}
		a := amount / 400
		fmt.Fprintf(&p.defs, `<filter id="%s" data-emfplus-sharpen-radius="%.3f"><feConvolveMatrix order="3" kernelMatrix="0 %.6f 0 %.6f %.6f %.6f 0 %.6f 0" divisor="1" preserveAlpha="true"/></filter>`, id, radius, -a, -a, 1+4*a, -a, -a)
	case "1077af00-2848-4441-9489-44ad4c2d7a2c": // TintEffect
		if len(buf) < 8 {
			p.effectFilter = ""
			return
		}
		hue := float64(int32(binary.LittleEndian.Uint32(buf[0:4])))
		amount := float64(int32(binary.LittleEndian.Uint32(buf[4:8])))
		if amount < -100 {
			amount = -100
		}
		if amount > 100 {
			amount = 100
		}
		sat := 1 - math.Abs(amount)/100
		bias := amount / 200
		fmt.Fprintf(&p.defs, `<filter id="%s"><feColorMatrix type="hueRotate" values="%.3f"/><feColorMatrix type="saturate" values="%.6f"/><feComponentTransfer><feFuncR type="linear" slope="1" intercept="%.6f"/><feFuncG type="linear" slope="1" intercept="%.6f"/><feFuncB type="linear" slope="1" intercept="%.6f"/></feComponentTransfer></filter>`, id, hue, sat, bias, bias, bias)
	default:
		p.effectFilter = ""
		return
	}
	p.effectFilter = id
}

func (p *emfPlusPlayback) effectAttr(enabled bool) string {
	if !enabled || p.effectFilter == "" {
		return ""
	}
	return fmt.Sprintf(` filter="url(#%s)"`, p.effectFilter)
}

func emfPlusParseImageAttributes(data []byte) (emfPlusImageAttributesObject, bool) {
	if len(data) < 24 {
		return emfPlusImageAttributesObject{}, false
	}
	return emfPlusImageAttributesObject{
		wrapMode:    binary.LittleEndian.Uint32(data[8:12]),
		clampColor:  emfPlusARGB(binary.LittleEndian.Uint32(data[12:16])),
		objectClamp: int32(binary.LittleEndian.Uint32(data[16:20])),
	}, true
}

func (p *emfPlusPlayback) regionLeafElement(n *emfPlusRegionNode, transform string) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.typ {
	case 0x10000000:
		return fmt.Sprintf(`<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" transform="%s"/>`, n.rect.x, n.rect.y, n.rect.w, n.rect.h, transform), true
	case 0x10000001:
		return fmt.Sprintf(`<path d="%s" transform="%s"/>`, n.path.d, transform), n.path.d != ""
	case 0x10000002: // Empty
		return "", true
	case 0x10000003: // Infinite
		return `<rect x="-1000000" y="-1000000" width="2000000" height="2000000"/>`, true
	}
	return "", false
}

func (p *emfPlusPlayback) regionMask(n *emfPlusRegionNode, transform string) (string, bool) {
	if n == nil {
		return "", false
	}
	if e, ok := p.regionLeafElement(n, transform); ok {
		if e == "" { // empty region
			p.clipSerial++
			id := fmt.Sprintf("emfplus-mask-%d", p.clipSerial)
			fmt.Fprintf(&p.defs, `<mask id="%s" maskUnits="userSpaceOnUse" x="-1000000" y="-1000000" width="2000000" height="2000000"><rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="black"/></mask>`, id)
			return id, true
		}
		return p.defineLeafMask(e), true
	}
	if n.left == nil || n.right == nil || n.typ < 1 || n.typ > 5 {
		return "", false
	}
	left, lok := p.regionMask(n.left, transform)
	right, rok := p.regionMask(n.right, transform)
	if !lok || !rok {
		return "", false
	}
	return p.combineMasks(left, right, uint16(n.typ)), true
}

func (p *emfPlusPlayback) fillRegion(rec emfPlusRecord) {
	region := p.objects[uint8(rec.flags&0xff)]
	if region.kind != emfPlusObjectRegion || region.region.root == nil || len(rec.data) < 4 {
		return
	}
	paint := p.brushPaint(rec.flags, binary.LittleEndian.Uint32(rec.data[:4]))
	if paint == "" {
		return
	}
	mask, ok := p.regionMask(region.region.root, emfPlusSVGMatrix(p.effectiveTransform()))
	if !ok || mask == "" {
		return
	}
	fmt.Fprintf(&p.body, `<rect x="-1000000" y="-1000000" width="2000000" height="2000000" fill="%s" mask="url(#%s)"%s/>`, paint, mask, p.compositingAttr())
	p.drawn = true
}

func (p *emfPlusPlayback) setClipRegion(rec emfPlusRecord) {
	region := p.objects[uint8(rec.flags&0xff)]
	if region.kind != emfPlusObjectRegion || region.region.root == nil {
		return
	}
	mode := emfPlusCombineMode(rec.flags)
	if mode > 5 {
		return
	}
	mask, ok := p.regionMask(region.region.root, emfPlusSVGMatrix(p.effectiveTransform()))
	if !ok {
		return
	}
	p.setCombinedClip(mask, mode)
}

func (p *emfPlusPlayback) compositingAttr() string {
	var b strings.Builder
	if p.compositingMode == 1 {
		b.WriteString(` data-emfplus-compositing="source-copy"`)
	}
	if p.compositingQuality != 0 {
		fmt.Fprintf(&b, ` data-emfplus-compositing-quality="%d"`, p.compositingQuality)
	}
	return b.String()
}

func emfPlusReadRectF(data []byte) (emfPlusRectF, bool) {
	if len(data) < 16 {
		return emfPlusRectF{}, false
	}
	f := func(off int) float64 {
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off : off+4])))
	}
	return emfPlusRectF{x: f(0), y: f(4), w: f(8), h: f(12)}, true
}

func emfPlusParseImage(data []byte) (emfPlusImageObject, bool) {
	// EmfPlusImage: Version, Type, ImageData. This pass owns bitmap images; metafile
	// image objects remain deferred to recursive metafile playback.
	if len(data) < 28 || binary.LittleEndian.Uint32(data[4:8]) != 1 {
		return emfPlusImageObject{}, false
	}
	width := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	height := int(int32(binary.LittleEndian.Uint32(data[12:16])))
	stride := int(int32(binary.LittleEndian.Uint32(data[16:20])))
	pixelFormat := binary.LittleEndian.Uint32(data[20:24])
	bitmapType := binary.LittleEndian.Uint32(data[24:28])
	payload := data[28:]
	if bitmapType == 1 { // BitmapDataTypeCompressed
		mime := emfPlusImageMIME(payload)
		if mime == "" {
			return emfPlusImageObject{}, false
		}
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(payload)); err == nil {
			width, height = cfg.Width, cfg.Height
		}
		if width <= 0 || height <= 0 {
			return emfPlusImageObject{}, false
		}
		return emfPlusImageObject{uri: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(payload), width: width, height: height}, true
	}
	if bitmapType != 0 || width <= 0 || height == 0 || stride == 0 || int((pixelFormat>>8)&0xff) != 32 {
		return emfPlusImageObject{}, false
	}
	absH := height
	if absH < 0 {
		absH = -absH
	}
	rowBytes := stride
	if rowBytes < 0 {
		rowBytes = -rowBytes
	}
	if rowBytes < width*4 || len(payload) < rowBytes*absH {
		return emfPlusImageObject{}, false
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, absH))
	for y := 0; y < absH; y++ {
		sy := y
		if height > 0 {
			sy = absH - 1 - y
		}
		row := payload[sy*rowBytes : sy*rowBytes+width*4]
		for x := 0; x < width; x++ {
			i := x * 4
			img.Pix[y*img.Stride+x*4+0] = row[i+2]
			img.Pix[y*img.Stride+x*4+1] = row[i+1]
			img.Pix[y*img.Stride+x*4+2] = row[i+0]
			img.Pix[y*img.Stride+x*4+3] = row[i+3]
		}
	}
	var enc bytes.Buffer
	if err := pngEncode(&enc, img); err != nil {
		return emfPlusImageObject{}, false
	}
	return emfPlusImageObject{uri: "data:image/png;base64," + base64.StdEncoding.EncodeToString(enc.Bytes()), width: width, height: absH}, true
}

func emfPlusImageMIME(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}
	return ""
}

func pngEncode(w *bytes.Buffer, img image.Image) error {
	// Kept behind a helper so EMF+ pixel-data handling stays isolated from classic EMF DIB code.
	return png.Encode(w, img)
}

func (p *emfPlusPlayback) drawImage(rec emfPlusRecord) {
	obj := p.objects[uint8(rec.flags&0xff)]
	if obj.kind != emfPlusObjectImage || obj.image.uri == "" || len(rec.data) < 32 {
		return
	}
	src, ok := emfPlusReadRectF(rec.data[8:24])
	if !ok {
		return
	}
	var dst emfPlusRectF
	if rec.flags&0x4000 != 0 { // compressed integer Rect
		if len(rec.data) < 32 {
			return
		}
		dst = emfPlusRectF{x: float64(int16(binary.LittleEndian.Uint16(rec.data[24:26]))), y: float64(int16(binary.LittleEndian.Uint16(rec.data[26:28]))), w: float64(int16(binary.LittleEndian.Uint16(rec.data[28:30]))), h: float64(int16(binary.LittleEndian.Uint16(rec.data[30:32])))}
	} else {
		if len(rec.data) < 40 {
			return
		}
		dst, ok = emfPlusReadRectF(rec.data[24:40])
		if !ok {
			return
		}
	}
	attrs := p.imageAttributes(binary.LittleEndian.Uint32(rec.data[:4]))
	p.emitImage(obj.image, attrs, src, [3][2]float64{{dst.x, dst.y}, {dst.x + dst.w, dst.y}, {dst.x, dst.y + dst.h}}, rec.flags&0x2000 != 0)
}

func (p *emfPlusPlayback) drawImagePoints(rec emfPlusRecord) {
	obj := p.objects[uint8(rec.flags&0xff)]
	if obj.kind != emfPlusObjectImage || obj.image.uri == "" || len(rec.data) < 28 {
		return
	}
	src, ok := emfPlusReadRectF(rec.data[8:24])
	if !ok {
		return
	}
	if binary.LittleEndian.Uint32(rec.data[24:28]) != 3 {
		return
	}
	var pts [3][2]float64
	list, _, ok := emfPlusReadPointsRecord(rec.data[28:], 3, rec.flags)
	if !ok || len(list) != 3 {
		return
	}
	copy(pts[:], list)
	attrs := p.imageAttributes(binary.LittleEndian.Uint32(rec.data[:4]))
	p.emitImage(obj.image, attrs, src, pts, rec.flags&0x2000 != 0)
}

func (p *emfPlusPlayback) imageAttributes(raw uint32) *emfPlusImageAttributesObject {
	id := uint8(raw & 0xff)
	if raw == 0xffffffff {
		return nil
	}
	obj := p.objects[id]
	if obj.kind != emfPlusObjectImageAttributes {
		return nil
	}
	a := obj.imageAttrs
	return &a
}

func (p *emfPlusPlayback) emitImage(img emfPlusImageObject, attrs *emfPlusImageAttributesObject, src emfPlusRectF, pts [3][2]float64, effect bool) {
	if src.w == 0 || src.h == 0 || img.width <= 0 || img.height <= 0 {
		return
	}
	// Crop with a nested SVG viewport, then apply the source-to-destination parallelogram transform.
	a := (pts[1][0] - pts[0][0]) / src.w
	b := (pts[1][1] - pts[0][1]) / src.w
	c := (pts[2][0] - pts[0][0]) / src.h
	d := (pts[2][1] - pts[0][1]) / src.h
	e := pts[0][0] - a*src.x - c*src.y
	f := pts[0][1] - b*src.x - d*src.y
	local := emfTransform{m11: a, m12: b, m21: c, m22: d, dx: e, dy: f}
	t := emfMultiplyTransform(p.effectiveTransform(), local)
	clamp := ""
	if attrs != nil && attrs.wrapMode == 4 { // WrapModeClamp
		clamp = fmt.Sprintf(`<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="%s"/>`, src.x, src.y, src.w, src.h, attrs.clampColor)
	}
	fmt.Fprintf(&p.body, `<g transform="%s"%s%s><svg x="%.3f" y="%.3f" width="%.3f" height="%.3f" viewBox="%.3f %.3f %.3f %.3f" preserveAspectRatio="none" overflow="hidden">%s<image x="0" y="0" width="%d" height="%d" preserveAspectRatio="none" href="%s"%s/></svg></g>`, emfPlusSVGMatrix(t), p.clipAttr(), p.effectAttr(effect), src.x, src.y, src.w, src.h, src.x, src.y, src.w, src.h, clamp, img.width, img.height, img.uri, p.imageQualityAttrs())
	p.drawn = true
}

func emfPlusSVGMatrix(t emfTransform) string {
	t = emfEffectiveTransform(t)
	return fmt.Sprintf("matrix(%.6f %.6f %.6f %.6f %.6f %.6f)", t.m11, t.m12, t.m21, t.m22, t.dx, t.dy)
}

func emfPlusARGB(v uint32) string {
	a := uint8(v >> 24)
	r := uint8(v >> 16)
	g := uint8(v >> 8)
	b := uint8(v)
	if a == 0xff {
		return fmt.Sprintf("#%02X%02X%02X", r, g, b)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.4f)", r, g, b, float64(a)/255)
}

func emfPlusRealizeFonts(faces []FontFace) []emfPlusRealizedFont {
	out := make([]emfPlusRealizedFont, 0, len(faces))
	for _, face := range faces {
		parsed, ok := fontmetrics.ParseSFNT(face.Data)
		if !ok {
			continue
		}
		font := emfPlusRealizedFont{family: strings.ToLower(strings.TrimSpace(face.Family)), bold: face.Bold, italic: face.Italic, face: parsed}
		if font.family != "" {
			out = append(out, font)
		}
	}
	return out
}

func (p *emfPlusPlayback) realizedFont(font emfPlusFontObject) *emfPlusRealizedFont {
	family := strings.ToLower(strings.TrimSpace(font.family))
	bold := font.style&0x1 != 0
	italic := font.style&0x2 != 0
	best := -1
	bestScore := 99
	for i := range p.fonts {
		f := &p.fonts[i]
		if f.family != family {
			continue
		}
		score := 0
		if f.bold != bold {
			score += 2
		}
		if f.italic != italic {
			score++
		}
		if score < bestScore {
			best, bestScore = i, score
			if score == 0 {
				break
			}
		}
	}
	if best >= 0 {
		return &p.fonts[best]
	}
	return nil
}

func (f *emfPlusRealizedFont) advanceForGlyph(gid uint16, size float64) (float64, bool) {
	if f == nil || f.face == nil {
		return 0, false
	}
	return f.face.AdvanceForGlyph(gid, size)
}

func (f *emfPlusRealizedFont) kernForGlyphs(left, right uint16, size float64) float64 {
	if f == nil || f.face == nil {
		return 0
	}
	return f.face.PairAdjustment(left, right, size)
}

func emfPlusParseFont(data []byte) (emfPlusFontObject, bool) {
	if len(data) < 24 {
		return emfPlusFontObject{}, false
	}
	emSize := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[4:8])))
	unit := binary.LittleEndian.Uint32(data[8:12])
	style := binary.LittleEndian.Uint32(data[12:16])
	length := int(binary.LittleEndian.Uint32(data[20:24]))
	if length < 0 || 24+length*2 > len(data) {
		return emfPlusFontObject{}, false
	}
	family := emfPlusUTF16(data[24 : 24+length*2])
	if family == "" || emSize <= 0 {
		return emfPlusFontObject{}, false
	}
	return emfPlusFontObject{family: family, size: emSize * emfPlusUnitScale(unit), style: style}, true
}

func emfPlusUnitScale(unit uint32) float64 {
	switch unit {
	case 2: // Pixel
		return 1
	case 3: // Point
		return 96.0 / 72.0
	case 4: // Inch
		return 96
	case 5: // Document (1/300 inch)
		return 96.0 / 300.0
	case 6: // Millimeter
		return 96.0 / 25.4
	default: // World / Display and unknown units remain in user space.
		return 1
	}
}

func emfPlusParseStringFormat(data []byte) (emfPlusStringFormatObject, bool) {
	if len(data) < 60 {
		return emfPlusStringFormatObject{}, false
	}
	f32 := func(off int) float64 {
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(data[off : off+4])))
	}
	tabCount := int(binary.LittleEndian.Uint32(data[52:56]))
	rangeCount := int(binary.LittleEndian.Uint32(data[56:60]))
	if tabCount < 0 || rangeCount < 0 || 60+tabCount*4+rangeCount*8 > len(data) {
		return emfPlusStringFormatObject{}, false
	}
	tabs := make([]float64, tabCount)
	for i := range tabs {
		tabs[i] = f32(60 + i*4)
	}
	rangeOff := 60 + tabCount*4
	ranges := make([]emfPlusCharacterRange, rangeCount)
	for i := range ranges {
		off := rangeOff + i*8
		ranges[i] = emfPlusCharacterRange{
			first:  int32(binary.LittleEndian.Uint32(data[off : off+4])),
			length: int32(binary.LittleEndian.Uint32(data[off+4 : off+8])),
		}
	}
	return emfPlusStringFormatObject{
		flags:          binary.LittleEndian.Uint32(data[4:8]),
		align:          binary.LittleEndian.Uint32(data[12:16]),
		lineAlign:      binary.LittleEndian.Uint32(data[16:20]),
		firstTabOffset: f32(28),
		hotkeyPrefix:   binary.LittleEndian.Uint32(data[32:36]),
		leadingMargin:  f32(36),
		trailingMargin: f32(40),
		tracking:       f32(44),
		trimming:       binary.LittleEndian.Uint32(data[48:52]),
		tabStops:       tabs,
		ranges:         ranges,
	}, true
}

func emfPlusUTF16(data []byte) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	u := make([]uint16, len(data)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}
	return string(utf16.Decode(u))
}

func (p *emfPlusPlayback) textPaint(rec emfPlusRecord, raw uint32) string {
	if rec.flags&0x8000 != 0 {
		return emfPlusARGB(raw)
	}
	if brush := p.objects[uint8(raw&0xff)]; brush.kind == emfPlusObjectBrush {
		return brush.brush.paint
	}
	return ""
}

func emfPlusFontAttrs(font emfPlusFontObject) string {
	var b strings.Builder
	fmt.Fprintf(&b, ` font-family="%s" font-size="%.3f"`, html.EscapeString(font.family), font.size)
	if font.style&0x1 != 0 {
		b.WriteString(` font-weight="bold"`)
	}
	if font.style&0x2 != 0 {
		b.WriteString(` font-style="italic"`)
	}
	var deco []string
	if font.style&0x4 != 0 {
		deco = append(deco, "underline")
	}
	if font.style&0x8 != 0 {
		deco = append(deco, "line-through")
	}
	if len(deco) > 0 {
		fmt.Fprintf(&b, ` text-decoration="%s"`, strings.Join(deco, " "))
	}
	return b.String()
}

func (p *emfPlusPlayback) effectiveTransform() emfTransform {
	scale := p.pageScale
	if scale == 0 {
		scale = 1
	}
	scale *= emfPlusUnitScale(uint32(p.pageUnit))
	if math.Abs(scale-1) < 1e-9 {
		return p.world
	}
	return emfMultiplyTransform(emfTransform{m11: scale, m22: scale}, p.world)
}

func (p *emfPlusPlayback) graphicsQualityAttrs() string {
	var b strings.Builder
	if p.antiAliasMode != 0 {
		fmt.Fprintf(&b, ` data-emfplus-antialias-mode="%d"`, p.antiAliasMode)
		if p.antiAliasMode&0x01 != 0 {
			b.WriteString(` shape-rendering="geometricPrecision"`)
		}
	}
	if p.pixelOffsetMode != 0 {
		fmt.Fprintf(&b, ` data-emfplus-pixel-offset-mode="%d"`, p.pixelOffsetMode)
	}
	if p.renderingOriginX != 0 || p.renderingOriginY != 0 {
		fmt.Fprintf(&b, ` data-emfplus-rendering-origin="%d,%d"`, p.renderingOriginX, p.renderingOriginY)
	}
	return b.String()
}

func (p *emfPlusPlayback) imageQualityAttrs() string {
	var b strings.Builder
	if p.interpolationMode != 0 {
		fmt.Fprintf(&b, ` data-emfplus-interpolation-mode="%d"`, p.interpolationMode)
		switch p.interpolationMode {
		case 1, 2, 3:
			b.WriteString(` image-rendering="pixelated"`)
		default:
			b.WriteString(` image-rendering="auto"`)
		}
	}
	return b.String()
}

func (p *emfPlusPlayback) textQualityAttrs() string {
	var attrs string
	switch p.textRenderingHint {
	case 1, 2:
		attrs += ` shape-rendering="crispEdges" text-rendering="optimizeSpeed"`
	case 3, 5:
		attrs += ` text-rendering="optimizeLegibility"`
	case 4:
		attrs += ` text-rendering="geometricPrecision"`
	}
	if p.textContrast != 0 {
		attrs += fmt.Sprintf(` data-emfplus-text-contrast="%d"`, p.textContrast)
	}
	return attrs
}

func emfPlusHotkeyText(text string, mode uint32) (string, int) {
	if mode == 0 {
		return text, -1
	}
	r := []rune(text)
	out := make([]rune, 0, len(r))
	hot := -1
	for i := 0; i < len(r); i++ {
		if r[i] == '&' && i+1 < len(r) {
			if r[i+1] == '&' {
				out = append(out, '&')
				i++
				continue
			}
			if mode == 1 && hot < 0 {
				hot = len(out)
			}
			continue
		}
		out = append(out, r[i])
	}
	return string(out), hot
}

func emfPlusApproxCharWidth(font emfPlusFontObject, tracking float64) float64 {
	base := font.size * 0.55
	if tracking > 0 {
		base *= tracking
	}
	return math.Max(base, 1)
}

func (p *emfPlusPlayback) textAdvance(text string, font emfPlusFontObject, tracking float64) float64 {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	spacing := 0.0
	if tracking > 0 && math.Abs(tracking-1) > 1e-6 {
		spacing = (tracking - 1) * font.size
	}
	if realized := p.realizedFont(font); realized != nil {
		total := 0.0
		var prev uint16
		prevOK := false
		for i, r := range runes {
			gid, ok := realized.face.RuneToGlyph[r]
			if ok && prevOK {
				total += realized.kernForGlyphs(prev, gid, font.size)
			}
			if !ok {
				total += emfPlusApproxCharWidth(font, 1)
			} else if advance, ok := realized.advanceForGlyph(gid, font.size); ok {
				total += advance
			} else {
				total += emfPlusApproxCharWidth(font, 1)
			}
			prev, prevOK = gid, ok
			if i+1 < len(runes) {
				total += spacing
			}
		}
		return total
	}
	return float64(len(runes)) * emfPlusApproxCharWidth(font, tracking)
}

type emfPlusTextLine struct {
	text       string
	start, end int
}

func emfPlusWrapLineSpans(text string, width float64, font emfPlusFontObject, format emfPlusStringFormatObject) []emfPlusTextLine {
	runes := []rune(text)
	max := 0
	if format.flags&0x1000 == 0 && width > 0 {
		max = int(math.Floor(width / emfPlusApproxCharWidth(font, format.tracking)))
		if max < 1 {
			max = 1
		}
	}
	var lines []emfPlusTextLine
	paraStart := 0
	for paraStart <= len(runes) {
		paraEnd := paraStart
		for paraEnd < len(runes) && runes[paraEnd] != '\n' {
			paraEnd++
		}
		start := paraStart
		if max == 0 {
			lines = append(lines, emfPlusTextLine{text: string(runes[start:paraEnd]), start: start, end: paraEnd})
		} else {
			for paraEnd-start > max {
				cut := start + max
				for i := cut; i > start; i-- {
					if runes[i-1] == ' ' {
						cut = i
						break
					}
				}
				drawEnd := cut
				for drawEnd > start && runes[drawEnd-1] == ' ' {
					drawEnd--
				}
				lines = append(lines, emfPlusTextLine{text: string(runes[start:drawEnd]), start: start, end: drawEnd})
				start = cut
				for start < paraEnd && runes[start] == ' ' {
					start++
				}
			}
			lines = append(lines, emfPlusTextLine{text: string(runes[start:paraEnd]), start: start, end: paraEnd})
		}
		if paraEnd >= len(runes) {
			break
		}
		paraStart = paraEnd + 1
	}
	return lines
}

func (p *emfPlusPlayback) emitCharacterRangeRegionsForLines(lines []emfPlusTextLine, ranges []emfPlusCharacterRange, font emfPlusFontObject, tracking float64, x, firstBaseline, lineHeight float64, anchor string, transform emfTransform, extraClip string) {
	if len(ranges) == 0 || len(lines) == 0 {
		return
	}
	for lineIndex, line := range lines {
		lineRunes := []rune(line.text)
		if len(lineRunes) == 0 {
			continue
		}
		lineX := x
		total := p.textAdvance(line.text, font, tracking)
		switch anchor {
		case "middle":
			lineX -= total / 2
		case "end":
			lineX -= total
		}
		baseline := firstBaseline + float64(lineIndex)*lineHeight
		for _, rg := range ranges {
			rStart := int(rg.first)
			rEnd := rStart + int(rg.length)
			start := rStart
			if start < line.start {
				start = line.start
			}
			end := rEnd
			if end > line.end {
				end = line.end
			}
			if start >= end {
				continue
			}
			localStart := start - line.start
			localEnd := end - line.start
			prefix := p.textAdvance(string(lineRunes[:localStart]), font, tracking)
			width := p.textAdvance(string(lineRunes[localStart:localEnd]), font, tracking)
			if width <= 0 {
				continue
			}
			fmt.Fprintf(&p.body, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="none" pointer-events="none" data-emfplus-character-range="%d:%d" data-emfplus-character-range-line="%d" transform="%s"%s%s/>`, lineX+prefix, baseline-font.size, width, lineHeight, rg.first, rg.length, lineIndex, emfPlusSVGMatrix(transform), extraClip, p.clipAttr())
		}
	}
}

func (p *emfPlusPlayback) emitCharacterRangeRegions(text string, ranges []emfPlusCharacterRange, font emfPlusFontObject, tracking float64, x, baseline, lineHeight float64, anchor string, transform emfTransform, extraClip string) {
	p.emitCharacterRangeRegionsForLines([]emfPlusTextLine{{text: text, start: 0, end: len([]rune(text))}}, ranges, font, tracking, x, baseline, lineHeight, anchor, transform, extraClip)
}

func emfPlusExpandTabs(text string, format emfPlusStringFormatObject, font emfPlusFontObject) string {
	if !strings.Contains(text, "\t") {
		return text
	}
	cw := emfPlusApproxCharWidth(font, format.tracking)
	var b strings.Builder
	col := 0.0
	tabIndex := 0
	for _, r := range text {
		if r != '\t' {
			b.WriteRune(r)
			if r == '\n' {
				col = 0
			} else {
				col += cw
			}
			continue
		}
		target := format.firstTabOffset
		if tabIndex < len(format.tabStops) {
			target += format.tabStops[tabIndex]
		} else if len(format.tabStops) > 0 {
			target += format.tabStops[len(format.tabStops)-1] * float64(tabIndex+1)
		} else {
			target = col + 8*cw
		}
		if target <= col {
			target = col + 8*cw
		}
		n := int(math.Ceil((target - col) / cw))
		if n < 1 {
			n = 1
		}
		b.WriteString(strings.Repeat(" ", n))
		col += float64(n) * cw
		tabIndex++
	}
	return b.String()
}

func emfPlusWrapLines(text string, width float64, font emfPlusFontObject, format emfPlusStringFormatObject) []string {
	spans := emfPlusWrapLineSpans(text, width, font, format)
	lines := make([]string, len(spans))
	for i := range spans {
		lines[i] = spans[i].text
	}
	return lines
}

func emfPlusTrimLine(line string, max int, mode uint32) string {
	r := []rune(line)
	if max <= 0 || len(r) <= max || mode == 0 {
		return line
	}
	if mode == 1 {
		return string(r[:max])
	}
	if max < 2 {
		return "…"
	}
	cut := max - 1
	if mode == 2 || mode == 4 {
		for i := cut; i > 0; i-- {
			if r[i-1] == ' ' {
				cut = i - 1
				break
			}
		}
	}
	if mode == 5 { // EllipsisPath: preserve the tail after a slash/backslash when possible.
		tail := 0
		for i := len(r) - 1; i >= 0; i-- {
			if r[i] == '/' || r[i] == '\\' {
				tail = i
				break
			}
		}
		if tail > 0 {
			suffix := r[tail:]
			keep := max - 1 - len(suffix)
			if keep > 0 {
				return string(r[:keep]) + "…" + string(suffix)
			}
		}
	}
	return string(r[:cut]) + "…"
}

func (p *emfPlusPlayback) drawString(rec emfPlusRecord) {
	fontObj := p.objects[uint8(rec.flags&0xff)]
	if fontObj.kind != emfPlusObjectFont || len(rec.data) < 28 {
		return
	}
	brushID := binary.LittleEndian.Uint32(rec.data[0:4])
	paint := p.textPaint(rec, brushID)
	if paint == "" {
		return
	}
	formatID := binary.LittleEndian.Uint32(rec.data[4:8])
	length := int(binary.LittleEndian.Uint32(rec.data[8:12]))
	layout, ok := emfPlusReadRectF(rec.data[12:28])
	if !ok || length < 0 || 28+length*2 > len(rec.data) {
		return
	}
	text := emfPlusUTF16(rec.data[28 : 28+length*2])
	if text == "" {
		return
	}
	format := emfPlusStringFormatObject{tracking: 1}
	if formatID != 0xffffffff {
		if obj := p.objects[uint8(formatID&0xff)]; obj.kind == emfPlusObjectStringFormat {
			format = obj.stringFmt
		}
	}
	text, hotkey := emfPlusHotkeyText(text, format.hotkeyPrefix)
	text = emfPlusExpandTabs(text, format, fontObj.font)
	innerW := layout.w - format.leadingMargin - format.trailingMargin
	lineSpans := emfPlusWrapLineSpans(text, innerW, fontObj.font, format)
	lines := make([]string, len(lineSpans))
	for i := range lineSpans {
		lines[i] = lineSpans[i].text
	}
	lineHeight := fontObj.font.size * 1.2
	maxLines := len(lines)
	if layout.h > 0 && format.flags&0x4000 == 0 {
		fit := int(math.Floor(layout.h / lineHeight))
		if fit < maxLines {
			maxLines = fit
		}
	}
	if maxLines < 0 {
		maxLines = 0
	}
	if maxLines < len(lines) && maxLines > 0 {
		lines = lines[:maxLines]
		lineSpans = lineSpans[:maxLines]
	}
	if len(lines) > 0 && format.trimming != 0 {
		maxChars := int(math.Floor(innerW / emfPlusApproxCharWidth(fontObj.font, format.tracking)))
		lines[len(lines)-1] = emfPlusTrimLine(lines[len(lines)-1], maxChars, format.trimming)
	}
	x := layout.x + format.leadingMargin
	y := layout.y + fontObj.font.size
	anchor := "start"
	switch format.align {
	case 1:
		x = layout.x + layout.w/2
		anchor = "middle"
	case 2:
		x = layout.x + layout.w - format.trailingMargin
		anchor = "end"
	}
	totalH := float64(len(lines)) * lineHeight
	switch format.lineAlign {
	case 1:
		y = layout.y + (layout.h-totalH)/2 + fontObj.font.size
	case 2:
		y = layout.y + layout.h - totalH + fontObj.font.size
	}
	attrs := ""
	if format.flags&0x1 != 0 {
		attrs += ` direction="rtl" unicode-bidi="bidi-override"`
	}
	if format.flags&0x2 != 0 {
		attrs += ` writing-mode="vertical-rl"`
	}
	if format.tracking > 0 && math.Abs(format.tracking-1) > 1e-6 {
		fmtVal := (format.tracking - 1) * fontObj.font.size
		attrs += fmt.Sprintf(` letter-spacing="%.3f"`, fmtVal)
	}
	if len(format.ranges) > 0 {
		parts := make([]string, 0, len(format.ranges))
		for _, r := range format.ranges {
			parts = append(parts, fmt.Sprintf("%d:%d", r.first, r.length))
		}
		attrs += fmt.Sprintf(` data-emfplus-character-ranges="%s"`, strings.Join(parts, ","))
	}
	clip := ""
	if format.flags&0x4000 == 0 && layout.w != 0 && layout.h != 0 {
		mask := p.defineLeafMask(fmt.Sprintf(`<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" transform="%s"/>`, layout.x, layout.y, layout.w, layout.h, emfPlusSVGMatrix(p.effectiveTransform())))
		clip = fmt.Sprintf(` mask="url(#%s)"`, mask)
	}
	if len(lines) == 0 {
		return
	}
	quality := p.textQualityAttrs()
	fmt.Fprintf(&p.body, `<text x="%.3f" y="%.3f" fill="%s" text-anchor="%s"%s%s%s transform="%s"%s%s>`, x, y, paint, anchor, emfPlusFontAttrs(fontObj.font), attrs, quality, emfPlusSVGMatrix(p.effectiveTransform()), clip, p.clipAttr())
	for i, line := range lines {
		if i == 0 {
			fmt.Fprintf(&p.body, `<tspan x="%.3f" dy="0">`, x)
		} else {
			fmt.Fprintf(&p.body, `<tspan x="%.3f" dy="%.3f">`, x, lineHeight)
		}
		if hotkey >= 0 && i == 0 {
			rr := []rune(line)
			if hotkey < len(rr) {
				pfx := html.EscapeString(string(rr[:hotkey]))
				key := html.EscapeString(string(rr[hotkey]))
				sfx := html.EscapeString(string(rr[hotkey+1:]))
				fmt.Fprintf(&p.body, `%s<tspan text-decoration="underline">%s</tspan>%s`, pfx, key, sfx)
			} else {
				p.body.WriteString(html.EscapeString(line))
			}
		} else {
			p.body.WriteString(html.EscapeString(line))
		}
		p.body.WriteString(`</tspan>`)
	}
	p.body.WriteString(`</text>`)
	if format.flags&0x2 == 0 && format.trimming == 0 && hotkey < 0 && !strings.Contains(text, "\t") {
		p.emitCharacterRangeRegionsForLines(lineSpans, format.ranges, fontObj.font, format.tracking, x, y, lineHeight, anchor, p.effectiveTransform(), clip)
	}
	p.drawn = true
}

func (p *emfPlusPlayback) drawDriverString(rec emfPlusRecord) {
	fontObj := p.objects[uint8(rec.flags&0xff)]
	if fontObj.kind != emfPlusObjectFont || len(rec.data) < 16 {
		return
	}
	brushID := binary.LittleEndian.Uint32(rec.data[0:4])
	paint := p.textPaint(rec, brushID)
	if paint == "" {
		return
	}
	options := binary.LittleEndian.Uint32(rec.data[4:8])
	matrixPresent := binary.LittleEndian.Uint32(rec.data[8:12]) != 0
	count := int(binary.LittleEndian.Uint32(rec.data[12:16]))
	if count <= 0 || 16+count*2 > len(rec.data) {
		return
	}
	glyphBytes := rec.data[16 : 16+count*2]
	glyphValues := make([]uint16, count)
	for i := range glyphValues {
		glyphValues[i] = binary.LittleEndian.Uint16(glyphBytes[i*2 : i*2+2])
	}
	realized := p.realizedFont(fontObj.font)
	var textRunes []rune
	var advanceGlyphs []uint16
	if options&0x1 != 0 { // DriverStringOptionsCmapLookup: values are Unicode UTF-16.
		textRunes = utf16.Decode(glyphValues)
		if realized != nil {
			advanceGlyphs = make([]uint16, len(textRunes))
			for i, r := range textRunes {
				advanceGlyphs[i] = realized.face.RuneToGlyph[r]
			}
		}
	} else {
		// Without CmapLookup the values are font glyph indices. They are only safe to
		// expose as text after resolving them through the actual supplied font cmap.
		if realized == nil {
			return
		}
		textRunes = make([]rune, count)
		advanceGlyphs = append([]uint16(nil), glyphValues...)
		for i, gid := range glyphValues {
			r, ok := realized.face.GlyphToRune[gid]
			if !ok {
				return
			}
			textRunes[i] = r
		}
	}
	if len(textRunes) == 0 {
		return
	}
	off := 16 + count*2
	posCount := count
	if options&0x4 != 0 { // DriverStringOptionsRealizedAdvance
		posCount = 1
	}
	if off+posCount*8 > len(rec.data) {
		return
	}
	pts := make([][2]float64, posCount)
	for i := range pts {
		pts[i][0] = float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[off+i*8 : off+i*8+4])))
		pts[i][1] = float64(math.Float32frombits(binary.LittleEndian.Uint32(rec.data[off+i*8+4 : off+i*8+8])))
	}
	off += posCount * 8
	local := emfIdentityTransform()
	if matrixPresent {
		if off+24 > len(rec.data) {
			return
		}
		var ok bool
		local, ok = emfPlusTransform(rec.data[off : off+24])
		if !ok {
			return
		}
	}
	transform := emfMultiplyTransform(p.effectiveTransform(), local)
	vertical := options&0x2 != 0
	extra := ""
	if vertical {
		extra = ` writing-mode="vertical-rl"`
	}
	emitRune := func(r rune, x, y float64) {
		fmt.Fprintf(&p.body, `<text x="%.3f" y="%.3f" fill="%s"%s%s%s transform="%s"%s>%s</text>`, x, y, paint, emfPlusFontAttrs(fontObj.font), extra, p.textQualityAttrs(), emfPlusSVGMatrix(transform), p.clipAttr(), html.EscapeString(string(r)))
	}
	if posCount == count {
		for i, r := range textRunes {
			if i >= len(pts) {
				break
			}
			emitRune(r, pts[i][0], pts[i][1])
		}
		p.drawn = true
		return
	}
	if realized != nil && len(advanceGlyphs) == len(textRunes) {
		x, y := pts[0][0], pts[0][1]
		for i, r := range textRunes {
			emitRune(r, x, y)
			advance, ok := realized.advanceForGlyph(advanceGlyphs[i], fontObj.font.size)
			if !ok {
				advance = emfPlusApproxCharWidth(fontObj.font, 1)
			}
			if i+1 < len(advanceGlyphs) {
				advance += realized.kernForGlyphs(advanceGlyphs[i], advanceGlyphs[i+1], fontObj.font.size)
			}
			if vertical {
				y += advance
			} else {
				x += advance
			}
		}
		p.drawn = true
		return
	}
	fmt.Fprintf(&p.body, `<text x="%.3f" y="%.3f" fill="%s"%s%s%s transform="%s"%s>%s</text>`, pts[0][0], pts[0][1], paint, emfPlusFontAttrs(fontObj.font), extra, p.textQualityAttrs(), emfPlusSVGMatrix(transform), p.clipAttr(), html.EscapeString(string(textRunes)))
	p.drawn = true
}
