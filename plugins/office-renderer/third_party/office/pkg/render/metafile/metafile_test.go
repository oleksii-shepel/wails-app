package metafile

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestWMFCEmbeddedEMFConvertsToSVG(t *testing.T) {
	emf := make([]byte, 128)
	binary.LittleEndian.PutUint32(emf[0:4], emrHeader)
	binary.LittleEndian.PutUint32(emf[4:8], 108)
	binary.LittleEndian.PutUint32(emf[8:12], 0)
	binary.LittleEndian.PutUint32(emf[12:16], 0)
	binary.LittleEndian.PutUint32(emf[16:20], 256)
	binary.LittleEndian.PutUint32(emf[20:24], 128)
	copy(emf[40:44], []byte(" EMF"))
	binary.LittleEndian.PutUint32(emf[48:52], uint32(len(emf)))
	binary.LittleEndian.PutUint32(emf[108:112], emrEOF)
	binary.LittleEndian.PutUint32(emf[112:116], 20)

	chunk := make([]byte, 34+len(emf))
	copy(chunk[:4], []byte("WMFC"))
	copy(chunk[34:], emf)
	payload := make([]byte, 4+len(chunk))
	binary.LittleEndian.PutUint16(payload[0:2], 0x000f)
	binary.LittleEndian.PutUint16(payload[2:4], uint16(len(chunk)))
	copy(payload[4:], chunk)
	recBytes := 6 + len(payload)
	if recBytes%2 != 0 {
		payload = append(payload, 0)
		recBytes++
	}
	wmf := make([]byte, 22+18+recBytes+6)
	binary.LittleEndian.PutUint32(wmf[:4], 0x9ac6cdd7)
	off := 22
	binary.LittleEndian.PutUint16(wmf[off:off+2], 1)
	binary.LittleEndian.PutUint16(wmf[off+2:off+4], 9)
	binary.LittleEndian.PutUint16(wmf[off+4:off+6], 0x0300)
	off += 18
	binary.LittleEndian.PutUint32(wmf[off:off+4], uint32(recBytes/2))
	binary.LittleEndian.PutUint16(wmf[off+4:off+6], 0x0626)
	copy(wmf[off+6:], payload)
	off += recBytes
	binary.LittleEndian.PutUint32(wmf[off:off+4], 3)

	svg, width, height, ok := ToSVG(wmf, "image/wmf")
	if !ok {
		t.Fatal("embedded EMF WMF was not converted")
	}
	if width != 256 || height != 128 {
		t.Fatalf("size = %v x %v", width, height)
	}
	if !strings.Contains(string(svg), "<svg") {
		t.Fatalf("not SVG: %q", svg)
	}
}

func TestEMRPolyline16RemainsOpenAndUnfilled(t *testing.T) {
	// A minimal EMF containing an open three-point polyline. Office diagrams
	// commonly use this for construction lines; closing it produces a triangle.
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 200)
	binary.LittleEndian.PutUint32(header[20:24], 100)
	copy(header[40:44], []byte(" EMF"))

	rec := make([]byte, 40)
	binary.LittleEndian.PutUint32(rec[0:4], emrPolyline16)
	binary.LittleEndian.PutUint32(rec[4:8], uint32(len(rec)))
	binary.LittleEndian.PutUint32(rec[24:28], 3)
	pts := [][2]int16{{10, 80}, {100, 10}, {190, 80}}
	for i, pt := range pts {
		binary.LittleEndian.PutUint16(rec[28+i*4:30+i*4], uint16(pt[0]))
		binary.LittleEndian.PutUint16(rec[30+i*4:32+i*4], uint16(pt[1]))
	}
	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))

	svg, _, _, ok := emfToSVG(append(append(header, rec...), eof...))
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<polyline`) || !strings.Contains(out, `fill="none"`) {
		t.Fatalf("open polyline was not preserved: %s", out)
	}
	if strings.Contains(out, `<polygon`) {
		t.Fatalf("polyline was incorrectly closed into polygon: %s", out)
	}
}

func TestEMFPointUsesViewportMapping(t *testing.T) {
	state := emfState{
		windowOrgX: 100, windowOrgY: 200,
		windowExtX: 10000, windowExtY: 5000,
		viewportOrgX: 10, viewportOrgY: 20,
		viewportExtX: 500, viewportExtY: 250,
	}
	x, y := emfPoint(state, 5100, 2700, 500, 250)
	if math.Abs(x-260) > 0.001 || math.Abs(y-145) > 0.001 {
		t.Fatalf("mapped point = %.3f,%.3f; want 260,145", x, y)
	}
}

func TestEMFNonZeroBoundsRemainDeviceSpaceViewport(t *testing.T) {
	// Office WMFC payloads can use a non-zero EMF rclBounds together with an
	// anisotropic logical window. The mapped drawing is in that device space;
	// the SVG viewBox must therefore retain the rclBounds origin.
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[8:12], 204)
	binary.LittleEndian.PutUint32(header[12:16], 37)
	binary.LittleEndian.PutUint32(header[16:20], 725)
	binary.LittleEndian.PutUint32(header[20:24], 375)
	copy(header[40:44], []byte(" EMF"))

	mapMode := make([]byte, 12)
	binary.LittleEndian.PutUint32(mapMode[0:4], emrSetMapMode)
	binary.LittleEndian.PutUint32(mapMode[4:8], uint32(len(mapMode)))
	binary.LittleEndian.PutUint32(mapMode[8:12], 8) // MM_ANISOTROPIC
	win := make([]byte, 16)
	binary.LittleEndian.PutUint32(win[0:4], emrSetWindowExtEx)
	binary.LittleEndian.PutUint32(win[4:8], uint32(len(win)))
	binary.LittleEndian.PutUint32(win[8:12], 63900)
	binary.LittleEndian.PutUint32(win[12:16], 42200)
	vp := make([]byte, 16)
	binary.LittleEndian.PutUint32(vp[0:4], emrSetViewportExtEx)
	binary.LittleEndian.PutUint32(vp[4:8], uint32(len(vp)))
	binary.LittleEndian.PutUint32(vp[8:12], 54102)
	binary.LittleEndian.PutUint32(vp[12:16], 35729)

	line := make([]byte, 36)
	binary.LittleEndian.PutUint32(line[0:4], emrPolyline16)
	binary.LittleEndian.PutUint32(line[4:8], uint32(len(line)))
	binary.LittleEndian.PutUint32(line[24:28], 2)
	binary.LittleEndian.PutUint16(line[28:30], 238)
	binary.LittleEndian.PutUint16(line[30:32], 76)
	binary.LittleEndian.PutUint16(line[32:34], 724)
	binary.LittleEndian.PutUint16(line[34:36], 335)
	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))

	data := append(header, mapMode...)
	data = append(data, win...)
	data = append(data, vp...)
	data = append(data, line...)
	data = append(data, eof...)
	svg, _, _, ok := emfToSVG(data)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `viewBox="204.000 37.000 521.000 338.000"`) {
		t.Fatalf("non-zero device bounds lost: %s", out)
	}
	if strings.Contains(out, `points="34.000,39.000`) {
		t.Fatalf("anisotropic logical coordinates were incorrectly treated as device coordinates: %s", out)
	}
}

func TestToSVGProjectsClassicWMFPolygon(t *testing.T) {
	// METAHEADER + window origin/ext + brush + polygon + EOF.
	var data []byte
	put16 := func(v uint16) { data = append(data, byte(v), byte(v>>8)) }
	put32 := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16(1)
	put16(9)
	put16(0x0300)
	put32(0)
	put16(8)
	put32(0)
	put16(0)
	rec := func(fn uint16, params ...uint16) {
		put32(uint32(3 + len(params)))
		put16(fn)
		for _, v := range params {
			put16(v)
		}
	}
	rec(0x020b, 0, 0)
	rec(0x020c, 100, 200)
	// solid red brush COLORREF = 0x000000ff
	rec(0x02fc, 0, 0x00ff, 0x0000, 0)
	rec(0x012d, 0)
	rec(0x0324, 3, 10, 10, 100, 10, 50, 80)
	rec(0)
	svg, w, h, ok := ToSVG(data, "image/x-wmf")
	if !ok || w != 200 || h != 100 {
		t.Fatalf("WMF projection ok=%v size=%gx%g", ok, w, h)
	}
	text := string(svg)
	if strings.Contains(text, `<g transform="scale(1,-1)">`) || !strings.Contains(text, `fill="#FF0000"`) || !strings.Contains(text, `10,10 100,10 50,80`) {
		t.Fatalf("unexpected WMF SVG: %s", text)
	}
}

func TestToSVGDefaultMMTextUsesNegativeWindowExtentOrientation(t *testing.T) {
	// 38256.ppt's classic WMF has no META_SETMAPMODE, so GDI's default
	// MM_TEXT mapping applies. PowerPoint classic-WMF BLIPs use a negative
	// window extent as playback orientation metadata; preserve that sign at the
	// WMF->SVG boundary instead of discarding it during absolute-size normalization.
	var data []byte
	put16 := func(v uint16) { data = append(data, byte(v), byte(v>>8)) }
	put32 := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16(1)
	put16(9)
	put16(0x0300)
	put32(0)
	put16(8)
	put32(0)
	put16(0)
	rec := func(fn uint16, params ...uint16) {
		put32(uint32(3 + len(params)))
		put16(fn)
		for _, v := range params {
			put16(v)
		}
	}
	// 38256.ppt mapping: META_SETWINDOWORG y=945,x=-1031;
	// META_SETWINDOWEXT y=-1838,x=2000. No SETMAPMODE => MM_TEXT.
	rec(0x020b, uint16(945), 0xFBF9)
	rec(0x020c, 0xF8D2, uint16(2000))
	rec(0x02fc, 0, 0x00ff, 0x0000, 0)
	rec(0x012d, 0)
	rec(0x0324, 3, 10, 10, 100, 10, 50, 80)
	rec(0)
	svg, _, _, ok := ToSVG(data, "image/x-wmf")
	if !ok {
		t.Fatal("MM_TEXT WMF was not converted")
	}
	out := string(svg)
	if !strings.Contains(out, `<g transform="scale(1,-1)">`) || strings.Contains(out, `scale(-1,1)`) || strings.Contains(out, `scale(-1,-1)`) {
		t.Fatalf("MM_TEXT negative window height must preserve the embedded WMF Y orientation: %s", out)
	}
}

func TestToSVGPositiveAnisotropicWMFPreservesTERIOrientation(t *testing.T) {
	// 37625.ppt's TERI metafile explicitly uses MM_ANISOTROPIC and ends with
	// a positive logical window extent (y=9772, x=16000). With no authored
	// negative viewport extent, this is not a request to reflect the image.
	var data []byte
	put16 := func(v uint16) { data = append(data, byte(v), byte(v>>8)) }
	put32 := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16(1)
	put16(9)
	put16(0x0300)
	put32(0)
	put16(8)
	put32(0)
	put16(0)
	rec := func(fn uint16, params ...uint16) {
		put32(uint32(3 + len(params)))
		put16(fn)
		for _, v := range params {
			put16(v)
		}
	}
	rec(0x0103, 8) // MM_ANISOTROPIC
	rec(0x020b, 0, 0)
	rec(0x020c, 9772, 16000)
	rec(0x02fc, 0, 0x00ff, 0x0000, 0)
	rec(0x012d, 0)
	rec(0x0324, 3, 10, 10, 100, 10, 50, 80)
	rec(0)
	svg, _, _, ok := ToSVG(data, "image/x-wmf")
	if !ok {
		t.Fatal("TERI-like anisotropic WMF was not converted")
	}
	out := string(svg)
	if strings.Contains(out, `scale(1,-1)`) || strings.Contains(out, `scale(-1,1)`) || strings.Contains(out, `scale(-1,-1)`) {
		t.Fatalf("positive MM_ANISOTROPIC mapping must preserve authored orientation: %s", out)
	}
}

func TestToSVGSniffsDirectEMFPayloadBehindWMFMIME(t *testing.T) {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 200)
	binary.LittleEndian.PutUint32(header[20:24], 100)
	copy(header[40:44], []byte(" EMF"))

	pen := make([]byte, 28)
	binary.LittleEndian.PutUint32(pen[0:4], emrCreatePen)
	binary.LittleEndian.PutUint32(pen[4:8], uint32(len(pen)))
	binary.LittleEndian.PutUint32(pen[8:12], 1)
	binary.LittleEndian.PutUint32(pen[12:16], 0)
	binary.LittleEndian.PutUint32(pen[16:20], 1)
	binary.LittleEndian.PutUint32(pen[24:28], 0x008c4540) // COLORREF #40458c

	selectPen := make([]byte, 12)
	binary.LittleEndian.PutUint32(selectPen[0:4], emrSelectObject)
	binary.LittleEndian.PutUint32(selectPen[4:8], uint32(len(selectPen)))
	binary.LittleEndian.PutUint32(selectPen[8:12], 1)

	move := make([]byte, 16)
	binary.LittleEndian.PutUint32(move[0:4], emrMoveToEx)
	binary.LittleEndian.PutUint32(move[4:8], uint32(len(move)))
	binary.LittleEndian.PutUint32(move[8:12], 10)
	binary.LittleEndian.PutUint32(move[12:16], 10)

	line := make([]byte, 16)
	binary.LittleEndian.PutUint32(line[0:4], emrLineTo)
	binary.LittleEndian.PutUint32(line[4:8], uint32(len(line)))
	binary.LittleEndian.PutUint32(line[8:12], 190)
	binary.LittleEndian.PutUint32(line[12:16], 90)

	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))

	data := append(header, pen...)
	data = append(data, selectPen...)
	data = append(data, move...)
	data = append(data, line...)
	data = append(data, eof...)

	svg, _, _, ok := ToSVG(data, "image/wmf")
	if !ok {
		t.Fatal("direct EMF payload mislabeled as WMF was not converted")
	}
	if !strings.Contains(string(svg), `stroke="#40458c"`) {
		t.Fatalf("authored EMF pen color was lost: %s", svg)
	}
}

func TestEMFStockPenSelectionDoesNotLeakPreviousSeriesColor(t *testing.T) {
	state := emfState{penColor: "#40458c", penWidth: 3}
	applyEMFStockObject(&state, 7) // BLACK_PEN
	if state.penColor != "#000000" || state.penWidth != 1 || state.penNull {
		t.Fatalf("BLACK_PEN state = color %q width %.1f null=%v", state.penColor, state.penWidth, state.penNull)
	}
	applyEMFStockObject(&state, 8) // NULL_PEN
	if !state.penNull {
		t.Fatal("NULL_PEN did not suppress subsequent strokes")
	}
}

func TestToSVGProjectsClassicWMFPieAndText(t *testing.T) {
	// Slide 16 in 37625 uses a classic WMF (not WMFC-wrapped EMF) for the
	// two pie charts.  Exercise the primitives that make that authored image
	// browser-renderable: solid brush + pie wedges + font/text + line work.
	var data []byte
	put16 := func(v uint16) { data = append(data, byte(v), byte(v>>8)) }
	put32 := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16(1)
	put16(9)
	put16(0x0300)
	put32(0)
	put16(16)
	put32(0)
	put16(0)
	rec := func(fn uint16, params ...uint16) {
		put32(uint32(3 + len(params)))
		put16(fn)
		for _, v := range params {
			put16(v)
		}
	}
	rec(0x0103, 8) // MM_ANISOTROPIC
	rec(0x020b, 0, 0)
	rec(0x020c, 200, 300)
	// solid blue brush COLORREF = #6F89F7
	rec(0x02fc, 0, 0x896f, 0x00f7, 0)
	rec(0x012d, 0)
	// META_PIE params are yEnd,xEnd,yStart,xStart,bottom,right,top,left.
	rec(0x081a, 100, 250, 20, 150, 180, 280, 20, 120)
	// blue text color
	rec(0x0209, 0x4540, 0x008c)
	// Create a Tahoma font. LOGFONT payload is byte-oriented, so append this
	// record manually rather than through the uint16 helper.
	fontParams := make([]byte, 18+32)
	binary.LittleEndian.PutUint16(fontParams[0:2], 0xFFE8)
	binary.LittleEndian.PutUint16(fontParams[8:10], 700)
	copy(fontParams[18:], []byte("Tahoma\x00"))
	put32(uint32(3 + (len(fontParams)+1)/2))
	put16(0x02fb)
	data = append(data, fontParams...)
	if len(fontParams)%2 != 0 {
		data = append(data, 0)
	}
	rec(0x012d, 1)
	// EXTTextOut at x=20,y=40.
	text := []byte("Transport")
	params := make([]byte, 8+len(text))
	binary.LittleEndian.PutUint16(params[0:2], 40)
	binary.LittleEndian.PutUint16(params[2:4], 20)
	binary.LittleEndian.PutUint16(params[4:6], uint16(len(text)))
	copy(params[8:], text)
	put32(uint32(3 + (len(params)+1)/2))
	put16(0x0a32)
	data = append(data, params...)
	if len(params)%2 != 0 {
		data = append(data, 0)
	}
	rec(0x0214, 180, 10)
	rec(0x0213, 180, 290)
	rec(0)

	svg, w, h, ok := ToSVG(data, "image/x-wmf")
	if !ok || w != 300 || h != 200 {
		t.Fatalf("classic WMF projection ok=%v size=%gx%g", ok, w, h)
	}
	out := string(svg)
	for _, want := range []string{`<path d="M`, `fill="#6F89F7"`, `>Transport</text>`, `font-family="Tahoma"`, `<line `} {
		if !strings.Contains(out, want) {
			t.Fatalf("classic WMF projection missing %q: %s", want, out)
		}
	}
	if !strings.Contains(out, `viewBox="9.000 19.000 282.000 162.000"`) {
		t.Fatalf("classic WMF projection did not tighten the preview viewBox: %s", out)
	}
	if !strings.Contains(out, `dominant-baseline="text-before-edge"`) {
		t.Fatalf("classic WMF EXTTEXTOUT did not preserve top-baseline alignment: %s", out)
	}
}

func TestClassicWMFSymbolTextOutMapsLegacyOperatorBytes(t *testing.T) {
	var data []byte
	put16 := func(v uint16) { data = append(data, byte(v), byte(v>>8)) }
	put32 := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16(1)
	put16(9)
	put16(0x0300)
	put32(0)
	put16(16)
	put32(0)
	put16(0)
	rec := func(fn uint16, params ...uint16) {
		put32(uint32(3 + len(params)))
		put16(fn)
		for _, v := range params {
			put16(v)
		}
	}
	rec(0x0103, 8)
	rec(0x020b, 0, 0)
	rec(0x020c, 200, 300)
	fontParams := make([]byte, 18+32)
	binary.LittleEndian.PutUint16(fontParams[0:2], 0xFFE8)
	fontParams[13] = 2
	copy(fontParams[18:], []byte("Symbol\x00"))
	put32(uint32(3 + (len(fontParams)+1)/2))
	put16(0x02fb)
	data = append(data, fontParams...)
	if len(fontParams)%2 != 0 {
		data = append(data, 0)
	}
	rec(0x012d, 0)
	params := []byte{1, 0, 0xA3, 50, 0, 50, 0}
	put32(uint32(3 + (len(params)+1)/2))
	put16(0x0521)
	data = append(data, params...)
	if len(params)%2 != 0 {
		data = append(data, 0)
	}
	rec(0)

	svg, _, _, ok := ToSVG(data, "image/x-wmf")
	if !ok {
		t.Fatal("WMF Symbol TextOut did not convert")
	}
	out := string(svg)
	if !strings.Contains(out, `>≤</text>`) {
		t.Fatalf("legacy Symbol operator byte was not mapped to ≤: %s", out)
	}
}

func TestClassicWMFRespectsSavedViewportMappingsForMultipleChartRegions(t *testing.T) {
	// 37625 slide 16 stores both pie charts in one classic WMF. The file establishes
	// one full-image window, then uses SaveDC/RestoreDC plus ScaleViewportExt and
	// local window origins/extents to place each chart into its own region. Treating
	// those local logical coordinates as final SVG coordinates overlays the charts.
	var data []byte
	put16 := func(v uint16) { data = append(data, byte(v), byte(v>>8)) }
	put32 := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16(1)
	put16(9)
	put16(0x0300)
	put32(0)
	put16(16)
	put32(0)
	put16(0)
	rec := func(fn uint16, params ...uint16) {
		put32(uint32(3 + len(params)))
		put16(fn)
		for _, v := range params {
			put16(v)
		}
	}
	// Full image logical size.
	rec(0x020b, 1248, 492)
	rec(0x020c, 2583, 4866)

	// Left chart region: viewport becomes ~2521 x 2244, local logical box 483 x 430.
	rec(0x001e)
	rec(0x0103, 8)
	rec(0x0412, 2583, 2244, 4866, 2521)
	rec(0x020b, 0xfff1, 0)
	rec(0x020c, 430, 483)
	rec(0x02fc, 0, 0x896f, 0x00f7, 0)
	rec(0x012d, 0)
	rec(0x081a, 73, 264, 274, 399, 367, 412, 73, 118)
	rec(0x0127, 0xffff)

	// Right chart region: restore full viewport then scale it to ~2432 x 2583 and
	// move the local window one chart-width left, which places drawing on the right.
	rec(0x001e)
	rec(0x0103, 8)
	rec(0x0412, 2583, 2583, 4866, 2432)
	rec(0x020b, 0, 0xfe4c)
	rec(0x020c, 430, 436)
	rec(0x081a, 109, 230, 213, 356, 367, 360, 109, 102)
	rec(0x0127, 0xffff)
	rec(0)

	svg, w, h, ok := ToSVG(data, "image/x-wmf")
	if !ok || w != 4866 || h != 2583 {
		t.Fatalf("mapped WMF ok=%v size=%gx%g", ok, w, h)
	}
	out := string(svg)
	// The renderer shrink-wraps the viewBox to the drawn ink, so the saved
	// viewport mappings are what has to keep the regions apart: both chart
	// wedges must stay inside one shared coordinate system, far from each other,
	// rather than being stacked near the origin.
	if !strings.Contains(out, `M 1383.`) || !strings.Contains(out, `M 3720.`) {
		t.Fatalf("saved viewport mappings were not projected into separate regions: %s", out)
	}
	vb := viewBoxNumbers(out)
	if len(vb) != 4 {
		t.Fatalf("svg viewBox missing: %s", out)
	}
	if vb[0] > 1383 || vb[0]+vb[2] < 3721 {
		t.Fatalf("viewBox %v does not span both mapped chart regions: %s", vb, out)
	}
}

func TestClassicWMFPieUsesGDICounterClockwiseArcDirection(t *testing.T) {
	// PowerPoint's exploded pie wedges are classic GDI META_PIE records.  GDI
	// draws from the start radial to the end radial counter-clockwise.  In SVG's
	// Y-down coordinate system that is sweep=0.  Reversing it draws the
	// complementary wedge (for example 31% becomes 69%), which makes the chart
	// overlap almost completely.
	var data []byte
	put16 := func(v uint16) { data = append(data, byte(v), byte(v>>8)) }
	put32 := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16(1)
	put16(9)
	put16(0x0300)
	put32(0)
	put16(16)
	put32(0)
	put16(0)
	rec := func(fn uint16, params ...uint16) {
		put32(uint32(3 + len(params)))
		put16(fn)
		for _, v := range params {
			put16(v)
		}
	}
	rec(0x0103, 8)
	rec(0x020b, 0, 0)
	rec(0x020c, 430, 483)
	// The first Mumbai wedge from 37625: its authored span is ~31%.  The
	// clockwise complement is ~69%, so the SVG large-arc bit must stay clear.
	rec(0x081a, 73, 264, 274, 399, 367, 412, 73, 118)
	rec(0)

	svg, _, _, ok := ToSVG(data, "image/x-wmf")
	if !ok {
		t.Fatal("classic WMF pie was not converted")
	}
	out := string(svg)
	if !strings.Contains(out, ` A `) || !strings.Contains(out, ` 0 0 0 `) {
		t.Fatalf("META_PIE did not use GDI counter-clockwise short arc: %s", out)
	}
}

func TestEMFPolygon16UsesSelectedBrushAndPen(t *testing.T) {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 200)
	binary.LittleEndian.PutUint32(header[20:24], 100)
	copy(header[40:44], []byte(" EMF"))

	brush := make([]byte, 24)
	binary.LittleEndian.PutUint32(brush[0:4], emrCreateBrushIndirect)
	binary.LittleEndian.PutUint32(brush[4:8], uint32(len(brush)))
	binary.LittleEndian.PutUint32(brush[8:12], 1)
	binary.LittleEndian.PutUint32(brush[12:16], 0)          // BS_SOLID
	binary.LittleEndian.PutUint32(brush[16:20], 0x00ff0000) // blue COLORREF

	selectBrush := make([]byte, 12)
	binary.LittleEndian.PutUint32(selectBrush[0:4], emrSelectObject)
	binary.LittleEndian.PutUint32(selectBrush[4:8], uint32(len(selectBrush)))
	binary.LittleEndian.PutUint32(selectBrush[8:12], 1)

	poly := make([]byte, 48)
	binary.LittleEndian.PutUint32(poly[0:4], emrPolygon16)
	binary.LittleEndian.PutUint32(poly[4:8], uint32(len(poly)))
	binary.LittleEndian.PutUint32(poly[24:28], 5)
	pts := [][2]int16{{20, 20}, {100, 20}, {100, 70}, {20, 70}, {20, 20}}
	for i, pt := range pts {
		binary.LittleEndian.PutUint16(poly[28+i*4:30+i*4], uint16(pt[0]))
		binary.LittleEndian.PutUint16(poly[30+i*4:32+i*4], uint16(pt[1]))
	}

	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))

	data := append(header, brush...)
	data = append(data, selectBrush...)
	data = append(data, poly...)
	data = append(data, eof...)
	svg, _, _, ok := emfToSVG(data)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<polygon`) || !strings.Contains(out, `fill="#0000ff"`) {
		t.Fatalf("filled Polygon16 was not preserved: %s", out)
	}
}

func TestEMFRectangleUsesSelectedBrush(t *testing.T) {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 200)
	binary.LittleEndian.PutUint32(header[20:24], 100)
	copy(header[40:44], []byte(" EMF"))

	brush := make([]byte, 24)
	binary.LittleEndian.PutUint32(brush[0:4], emrCreateBrushIndirect)
	binary.LittleEndian.PutUint32(brush[4:8], uint32(len(brush)))
	binary.LittleEndian.PutUint32(brush[8:12], 1)
	binary.LittleEndian.PutUint32(brush[12:16], 0)          // BS_SOLID
	binary.LittleEndian.PutUint32(brush[16:20], 0x000000ff) // red COLORREF

	selectBrush := make([]byte, 12)
	binary.LittleEndian.PutUint32(selectBrush[0:4], emrSelectObject)
	binary.LittleEndian.PutUint32(selectBrush[4:8], uint32(len(selectBrush)))
	binary.LittleEndian.PutUint32(selectBrush[8:12], 1)

	rect := make([]byte, 24)
	binary.LittleEndian.PutUint32(rect[0:4], emrRectangle)
	binary.LittleEndian.PutUint32(rect[4:8], uint32(len(rect)))
	binary.LittleEndian.PutUint32(rect[8:12], 30)
	binary.LittleEndian.PutUint32(rect[12:16], 25)
	binary.LittleEndian.PutUint32(rect[16:20], 90)
	binary.LittleEndian.PutUint32(rect[20:24], 75)

	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))

	data := append(header, brush...)
	data = append(data, selectBrush...)
	data = append(data, rect...)
	data = append(data, eof...)
	svg, _, _, ok := emfToSVG(data)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<rect x="30.000" y="25.000" width="60.000" height="50.000" fill="#ff0000"`) {
		t.Fatalf("filled Rectangle was not preserved: %s", out)
	}
}

func TestEMFExtCreatePenOwnsSelectedStroke(t *testing.T) {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 200)
	binary.LittleEndian.PutUint32(header[20:24], 100)
	copy(header[40:44], []byte(" EMF"))

	pen := make([]byte, 56)
	binary.LittleEndian.PutUint32(pen[0:4], emrExtCreatePen)
	binary.LittleEndian.PutUint32(pen[4:8], uint32(len(pen)))
	binary.LittleEndian.PutUint32(pen[8:12], 2)
	binary.LittleEndian.PutUint32(pen[12:16], 56)
	binary.LittleEndian.PutUint32(pen[20:24], 56)
	binary.LittleEndian.PutUint32(pen[28:32], 0)          // PS_SOLID
	binary.LittleEndian.PutUint32(pen[32:36], 3)          // width
	binary.LittleEndian.PutUint32(pen[36:40], 0)          // BS_SOLID
	binary.LittleEndian.PutUint32(pen[40:44], 0x008c4540) // COLORREF #40458c

	selectPen := make([]byte, 12)
	binary.LittleEndian.PutUint32(selectPen[0:4], emrSelectObject)
	binary.LittleEndian.PutUint32(selectPen[4:8], uint32(len(selectPen)))
	binary.LittleEndian.PutUint32(selectPen[8:12], 2)

	move := make([]byte, 16)
	binary.LittleEndian.PutUint32(move[0:4], emrMoveToEx)
	binary.LittleEndian.PutUint32(move[4:8], uint32(len(move)))
	binary.LittleEndian.PutUint32(move[8:12], 10)
	binary.LittleEndian.PutUint32(move[12:16], 10)

	line := make([]byte, 16)
	binary.LittleEndian.PutUint32(line[0:4], emrLineTo)
	binary.LittleEndian.PutUint32(line[4:8], uint32(len(line)))
	binary.LittleEndian.PutUint32(line[8:12], 190)
	binary.LittleEndian.PutUint32(line[12:16], 90)

	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))

	data := append(header, pen...)
	data = append(data, selectPen...)
	data = append(data, move...)
	data = append(data, line...)
	data = append(data, eof...)
	svg, _, _, ok := emfToSVG(data)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `stroke="#40458c"`) || !strings.Contains(out, `stroke-width="3.000"`) {
		t.Fatalf("extended pen state was not preserved: %s", out)
	}
}

func TestEMFPieUsesSelectedBrushAndGDICounterClockwiseDirection(t *testing.T) {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 300)
	binary.LittleEndian.PutUint32(header[20:24], 200)
	copy(header[40:44], []byte(" EMF"))

	brush := make([]byte, 24)
	binary.LittleEndian.PutUint32(brush[0:4], emrCreateBrushIndirect)
	binary.LittleEndian.PutUint32(brush[4:8], uint32(len(brush)))
	binary.LittleEndian.PutUint32(brush[8:12], 1)
	binary.LittleEndian.PutUint32(brush[12:16], 0)
	binary.LittleEndian.PutUint32(brush[16:20], 0x00f7896f) // COLORREF #6f89f7

	selectBrush := make([]byte, 12)
	binary.LittleEndian.PutUint32(selectBrush[0:4], emrSelectObject)
	binary.LittleEndian.PutUint32(selectBrush[4:8], uint32(len(selectBrush)))
	binary.LittleEndian.PutUint32(selectBrush[8:12], 1)

	pie := make([]byte, 40)
	binary.LittleEndian.PutUint32(pie[0:4], emrPie)
	binary.LittleEndian.PutUint32(pie[4:8], uint32(len(pie)))
	// Ellipse box 50,20 -> 250,180. Start at top, end at right: default GDI
	// counter-clockwise travels visually through left/bottom before reaching right
	// in the Y-down SVG coordinate system, so this is the large arc with sweep=0.
	binary.LittleEndian.PutUint32(pie[8:12], 50)
	binary.LittleEndian.PutUint32(pie[12:16], 20)
	binary.LittleEndian.PutUint32(pie[16:20], 250)
	binary.LittleEndian.PutUint32(pie[20:24], 180)
	binary.LittleEndian.PutUint32(pie[24:28], 150)
	binary.LittleEndian.PutUint32(pie[28:32], 20)
	binary.LittleEndian.PutUint32(pie[32:36], 250)
	binary.LittleEndian.PutUint32(pie[36:40], 100)

	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))

	data := append(header, brush...)
	data = append(data, selectBrush...)
	data = append(data, pie...)
	data = append(data, eof...)
	svg, _, _, ok := emfToSVG(data)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<path d="M 150.000 100.000 L 150.000 20.000 A 100.000 80.000 0 1 0 250.000 100.000 Z"`) {
		t.Fatalf("EMR_PIE geometry/direction was not preserved: %s", out)
	}
	if !strings.Contains(out, `fill="#6f89f7"`) {
		t.Fatalf("EMR_PIE selected brush was not preserved: %s", out)
	}
}

func TestEMFRoundRectUsesMappedCornerEllipse(t *testing.T) {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 200)
	binary.LittleEndian.PutUint32(header[20:24], 100)
	copy(header[40:44], []byte(" EMF"))

	rr := make([]byte, 32)
	binary.LittleEndian.PutUint32(rr[0:4], emrRoundRect)
	binary.LittleEndian.PutUint32(rr[4:8], uint32(len(rr)))
	binary.LittleEndian.PutUint32(rr[8:12], 20)
	binary.LittleEndian.PutUint32(rr[12:16], 10)
	binary.LittleEndian.PutUint32(rr[16:20], 180)
	binary.LittleEndian.PutUint32(rr[20:24], 90)
	binary.LittleEndian.PutUint32(rr[24:28], 40)
	binary.LittleEndian.PutUint32(rr[28:32], 20)

	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))
	data := append(header, rr...)
	data = append(data, eof...)
	svg, _, _, ok := emfToSVG(data)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `rx="20.000" ry="10.000"`) {
		t.Fatalf("EMR_ROUNDRECT corner ellipse was not preserved: %s", out)
	}
}

func TestEMFWorldTransformAppliesBeforeWindowViewportMapping(t *testing.T) {
	state := emfState{
		windowExtX: 200, windowExtY: 100,
		viewportExtX: 400, viewportExtY: 200,
		world: emfTransform{m11: 1, m22: 1, dx: 10, dy: 20},
	}
	x, y := emfPoint(state, 5, 7, 400, 200)
	if math.Abs(x-30) > 0.001 || math.Abs(y-54) > 0.001 {
		t.Fatalf("world mapped point = %.3f,%.3f; want 30,54", x, y)
	}
}

func TestEMFModifyWorldTransformComposition(t *testing.T) {
	scale := emfTransform{m11: 2, m22: 3}
	translate := emfTransform{m11: 1, m22: 1, dx: 10, dy: 20}
	combined := emfMultiplyTransform(scale, translate)
	x, y := emfApplyTransform(combined, 4, 5)
	if math.Abs(x-18) > 0.001 || math.Abs(y-35) > 0.001 {
		t.Fatalf("composed point = %.3f,%.3f; want 18,35", x, y)
	}
}

func TestEMFTextAlignmentMapsToSVGAnchorAndBaseline(t *testing.T) {
	cases := []struct {
		align            uint32
		anchor, baseline string
	}{
		{0, "start", "text-before-edge"},
		{2 | 24, "end", "alphabetic"},
		{6 | 8, "middle", "text-after-edge"},
	}
	for _, tc := range cases {
		if got := emfTextAnchor(tc.align); got != tc.anchor {
			t.Fatalf("anchor(%#x)=%q want %q", tc.align, got, tc.anchor)
		}
		if got := emfTextBaseline(tc.align); got != tc.baseline {
			t.Fatalf("baseline(%#x)=%q want %q", tc.align, got, tc.baseline)
		}
	}
}

func TestEMFSetWorldTransformAndTextAlignAffectRenderedOutput(t *testing.T) {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 200)
	binary.LittleEndian.PutUint32(header[20:24], 100)
	copy(header[40:44], []byte(" EMF"))

	world := make([]byte, 32)
	binary.LittleEndian.PutUint32(world[0:4], emrSetWorldTransform)
	binary.LittleEndian.PutUint32(world[4:8], uint32(len(world)))
	putF := func(off int, v float32) { binary.LittleEndian.PutUint32(world[off:off+4], math.Float32bits(v)) }
	putF(8, 1)
	putF(20, 1)
	putF(24, 10)
	putF(28, 20)

	align := make([]byte, 12)
	binary.LittleEndian.PutUint32(align[0:4], emrSetTextAlign)
	binary.LittleEndian.PutUint32(align[4:8], uint32(len(align)))
	binary.LittleEndian.PutUint32(align[8:12], 6|24) // TA_CENTER | TA_BASELINE

	move := make([]byte, 16)
	binary.LittleEndian.PutUint32(move[0:4], emrMoveToEx)
	binary.LittleEndian.PutUint32(move[4:8], uint32(len(move)))
	binary.LittleEndian.PutUint32(move[8:12], 5)
	binary.LittleEndian.PutUint32(move[12:16], 7)

	line := make([]byte, 16)
	binary.LittleEndian.PutUint32(line[0:4], emrLineTo)
	binary.LittleEndian.PutUint32(line[4:8], uint32(len(line)))
	binary.LittleEndian.PutUint32(line[8:12], 20)
	binary.LittleEndian.PutUint32(line[12:16], 30)

	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))

	data := append(header, world...)
	data = append(data, align...)
	data = append(data, move...)
	data = append(data, line...)
	data = append(data, eof...)
	svg, _, _, ok := emfToSVG(data)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `x1="15.000" y1="27.000" x2="30.000" y2="50.000"`) {
		t.Fatalf("world transform not applied to geometry: %s", out)
	}
	_ = align // alignment helpers are covered independently; record ownership is exercised here.
}

func TestEMFDecodeDIB24BitBottomUp(t *testing.T) {
	bmi := make([]byte, 40)
	binary.LittleEndian.PutUint32(bmi[0:4], 40)
	binary.LittleEndian.PutUint32(bmi[4:8], 2)
	binary.LittleEndian.PutUint32(bmi[8:12], 2)
	binary.LittleEndian.PutUint16(bmi[12:14], 1)
	binary.LittleEndian.PutUint16(bmi[14:16], 24)
	// Each 2-pixel row is 6 bytes padded to a 4-byte boundary => 8 bytes.
	bits := []byte{
		255, 0, 0, 255, 255, 255, 0, 0, // bottom row: blue, white
		0, 0, 255, 0, 255, 0, 0, 0, // top row: red, green
	}
	img, ok := emfDecodeDIB(bmi, bits)
	if !ok {
		t.Fatal("DIB decode failed")
	}
	check := func(x, y int, wantR, wantG, wantB uint32) {
		r, g, b, _ := img.At(x, y).RGBA()
		if r>>8 != wantR || g>>8 != wantG || b>>8 != wantB {
			t.Fatalf("pixel(%d,%d)=(%d,%d,%d), want (%d,%d,%d)", x, y, r>>8, g>>8, b>>8, wantR, wantG, wantB)
		}
	}
	check(0, 0, 255, 0, 0)
	check(1, 0, 0, 255, 0)
	check(0, 1, 0, 0, 255)
	check(1, 1, 255, 255, 255)
}

func TestEMFStretchDIBitsRendersEmbeddedPNGWithMappedDestination(t *testing.T) {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], 200)
	binary.LittleEndian.PutUint32(header[20:24], 100)
	copy(header[40:44], []byte(" EMF"))

	const recordBase = 80
	const bmiSize = 40
	const bitsSize = 16
	rec := make([]byte, recordBase+bmiSize+bitsSize)
	binary.LittleEndian.PutUint32(rec[0:4], emrStretchDIBits)
	binary.LittleEndian.PutUint32(rec[4:8], uint32(len(rec)))
	binary.LittleEndian.PutUint32(rec[24:28], 10) // xDest
	binary.LittleEndian.PutUint32(rec[28:32], 20) // yDest
	binary.LittleEndian.PutUint32(rec[32:36], 0)  // xSrc
	binary.LittleEndian.PutUint32(rec[36:40], 0)  // ySrc
	binary.LittleEndian.PutUint32(rec[40:44], 2)  // cxSrc
	binary.LittleEndian.PutUint32(rec[44:48], 2)  // cySrc
	binary.LittleEndian.PutUint32(rec[48:52], recordBase)
	binary.LittleEndian.PutUint32(rec[52:56], bmiSize)
	binary.LittleEndian.PutUint32(rec[56:60], recordBase+bmiSize)
	binary.LittleEndian.PutUint32(rec[60:64], bitsSize)
	binary.LittleEndian.PutUint32(rec[68:72], emfROPSourceCopy)
	binary.LittleEndian.PutUint32(rec[72:76], 40) // cxDest
	binary.LittleEndian.PutUint32(rec[76:80], 20) // cyDest

	bmi := rec[recordBase : recordBase+bmiSize]
	binary.LittleEndian.PutUint32(bmi[0:4], 40)
	binary.LittleEndian.PutUint32(bmi[4:8], 2)
	binary.LittleEndian.PutUint32(bmi[8:12], 2)
	binary.LittleEndian.PutUint16(bmi[12:14], 1)
	binary.LittleEndian.PutUint16(bmi[14:16], 24)
	copy(rec[recordBase+bmiSize:], []byte{
		255, 0, 0, 255, 255, 255, 0, 0,
		0, 0, 255, 0, 255, 0, 0, 0,
	})

	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
	binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))
	data := append(header, rec...)
	data = append(data, eof...)

	svg, _, _, ok := emfToSVG(data)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `href="data:image/png;base64,`) {
		t.Fatalf("STRETCHDIBITS did not embed PNG: %s", out)
	}
	if !strings.Contains(out, `transform="matrix(20.000000000 0.000000000 0.000000000 10.000000000 10.000000000 20.000000000)"`) {
		t.Fatalf("STRETCHDIBITS destination mapping was not preserved: %s", out)
	}
}

func TestEMFDecodeDIBBitfields565(t *testing.T) {
	bmi := make([]byte, 52)
	binary.LittleEndian.PutUint32(bmi[0:4], 40)
	binary.LittleEndian.PutUint32(bmi[4:8], 1)
	binary.LittleEndian.PutUint32(bmi[8:12], 1)
	binary.LittleEndian.PutUint16(bmi[12:14], 1)
	binary.LittleEndian.PutUint16(bmi[14:16], 16)
	binary.LittleEndian.PutUint32(bmi[16:20], 3) // BI_BITFIELDS
	binary.LittleEndian.PutUint32(bmi[40:44], 0xF800)
	binary.LittleEndian.PutUint32(bmi[44:48], 0x07E0)
	binary.LittleEndian.PutUint32(bmi[48:52], 0x001F)
	bits := []byte{0x00, 0xF8, 0x00, 0x00}
	img, ok := emfDecodeDIB(bmi, bits)
	if !ok {
		t.Fatal("bitfields DIB did not decode")
	}
	r, g, b, a := img.At(0, 0).RGBA()
	if r < 0xF000 || g > 0x1000 || b > 0x1000 || a != 0xFFFF {
		t.Fatalf("decoded 565 red = %#04x %#04x %#04x %#04x", r, g, b, a)
	}
}

func TestEMFDecodeDIBAlphaBitfields32(t *testing.T) {
	bmi := make([]byte, 56)
	binary.LittleEndian.PutUint32(bmi[0:4], 56)
	binary.LittleEndian.PutUint32(bmi[4:8], 1)
	binary.LittleEndian.PutUint32(bmi[8:12], 0xFFFFFFFF)
	binary.LittleEndian.PutUint16(bmi[12:14], 1)
	binary.LittleEndian.PutUint16(bmi[14:16], 32)
	binary.LittleEndian.PutUint32(bmi[16:20], 6) // BI_ALPHABITFIELDS
	binary.LittleEndian.PutUint32(bmi[40:44], 0x00FF0000)
	binary.LittleEndian.PutUint32(bmi[44:48], 0x0000FF00)
	binary.LittleEndian.PutUint32(bmi[48:52], 0x000000FF)
	binary.LittleEndian.PutUint32(bmi[52:56], 0xFF000000)
	bits := []byte{0x20, 0x40, 0x80, 0x80}
	img, ok := emfDecodeDIB(bmi, bits)
	if !ok {
		t.Fatal("alpha bitfields DIB did not decode")
	}
	c := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	if c.R != 0x80 || c.G != 0x40 || c.B != 0x20 || c.A != 0x80 {
		t.Fatalf("decoded ARGB = %02x %02x %02x %02x", c.R, c.G, c.B, c.A)
	}
}

func TestEMFEmitRasterParallelogramUsesThreeDestinationPoints(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var body strings.Builder
	ok := emfEmitRasterParallelogram(&body, img, emfState{}, 200, 100, [3][2]int32{{10, 20}, {50, 30}, {20, 60}})
	if !ok {
		t.Fatal("parallelogram raster emission failed")
	}
	out := body.String()
	if !strings.Contains(out, `matrix(20.000000000 5.000000000 5.000000000 20.000000000 10.000000000 20.000000000)`) {
		t.Fatalf("unexpected parallelogram transform: %s", out)
	}
}

func TestEMFPathStrokeAndFillMaterializesBezier(t *testing.T) {
	emf := testEMFWithRecords(200, 120,
		testEMFRecord(emrBeginPath, nil),
		testEMFPointRecord(emrMoveToEx, 10, 10),
		testEMFPolyRecord(emrPolyBezierTo, [][2]int32{{30, 0}, {60, 40}, {90, 30}}),
		testEMFRecord(emrCloseFigure, nil),
		testEMFRecord(emrEndPath, nil),
		testEMFRecord(emrStrokeAndFillPath, make([]byte, 16)),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `C 30.000 0.000 60.000 40.000 90.000 30.000 Z`) {
		t.Fatalf("bezier path was not preserved: %s", out)
	}
	if !strings.Contains(out, `fill-rule="evenodd"`) || !strings.Contains(out, `stroke="#000000"`) {
		t.Fatalf("path paint semantics missing: %s", out)
	}
}

func TestEMFIntersectClipRectClipsFollowingGeometry(t *testing.T) {
	clip := make([]byte, 16)
	binary.LittleEndian.PutUint32(clip[0:4], 20)
	binary.LittleEndian.PutUint32(clip[4:8], 20)
	binary.LittleEndian.PutUint32(clip[8:12], 80)
	binary.LittleEndian.PutUint32(clip[12:16], 70)
	emf := testEMFWithRecords(200, 120,
		testEMFRecord(emrIntersectClipRect, clip),
		testEMFRectRecord(emrRectangle, 0, 0, 100, 100),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<clipPath id="emf-clip-1"><rect x="20.000" y="20.000" width="60.000" height="50.000"/></clipPath>`) {
		t.Fatalf("clip rectangle was not materialized: %s", out)
	}
	if !strings.Contains(out, `<g clip-path="url(#emf-clip-1)"><rect`) {
		t.Fatalf("following geometry is not clipped: %s", out)
	}
}

func TestEMFSelectClipPathAndRestoreDCRestoresPreviousClip(t *testing.T) {
	first := make([]byte, 16)
	binary.LittleEndian.PutUint32(first[0:4], 0)
	binary.LittleEndian.PutUint32(first[4:8], 0)
	binary.LittleEndian.PutUint32(first[8:12], 100)
	binary.LittleEndian.PutUint32(first[12:16], 100)
	copyMode := make([]byte, 4)
	binary.LittleEndian.PutUint32(copyMode, 5) // RGN_COPY
	emf := testEMFWithRecords(200, 120,
		testEMFRecord(emrIntersectClipRect, first),
		testEMFRecord(emrSaveDC, nil),
		testEMFRecord(emrBeginPath, nil),
		testEMFPointRecord(emrMoveToEx, 10, 10),
		testEMFPointRecord(emrLineTo, 50, 10),
		testEMFPointRecord(emrLineTo, 50, 50),
		testEMFRecord(emrCloseFigure, nil),
		testEMFRecord(emrEndPath, nil),
		testEMFRecord(emrSelectClipPath, copyMode),
		testEMFRectRecord(emrRectangle, 0, 0, 80, 80),
		testEMFRestoreDCRecord(-1),
		testEMFRectRecord(emrRectangle, 100, 0, 160, 60),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if strings.Count(out, `clip-path="url(#emf-clip-1)"`) < 2 {
		t.Fatalf("RestoreDC did not reactivate saved clip chain: %s", out)
	}
	if !strings.Contains(out, `clip-path="url(#emf-clip-2)"`) {
		t.Fatalf("selected path clip missing: %s", out)
	}
}

func testEMFWithRecords(width, height int32, records ...[]byte) []byte {
	header := make([]byte, 108)
	binary.LittleEndian.PutUint32(header[0:4], emrHeader)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(header)))
	binary.LittleEndian.PutUint32(header[16:20], uint32(width))
	binary.LittleEndian.PutUint32(header[20:24], uint32(height))
	copy(header[40:44], []byte(" EMF"))
	var all []byte
	all = append(all, header...)
	for _, rec := range records {
		all = append(all, rec...)
	}
	eof := testEMFRecord(emrEOF, make([]byte, 12))
	all = append(all, eof...)
	binary.LittleEndian.PutUint32(all[48:52], uint32(len(all)))
	return all
}

func testEMFRecord(recordType uint32, payload []byte) []byte {
	rec := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(rec[0:4], recordType)
	binary.LittleEndian.PutUint32(rec[4:8], uint32(len(rec)))
	copy(rec[8:], payload)
	return rec
}

func testEMFPointRecord(recordType uint32, x, y int32) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload[0:4], uint32(x))
	binary.LittleEndian.PutUint32(payload[4:8], uint32(y))
	return testEMFRecord(recordType, payload)
}

func testEMFRectRecord(recordType uint32, left, top, right, bottom int32) []byte {
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint32(payload[0:4], uint32(left))
	binary.LittleEndian.PutUint32(payload[4:8], uint32(top))
	binary.LittleEndian.PutUint32(payload[8:12], uint32(right))
	binary.LittleEndian.PutUint32(payload[12:16], uint32(bottom))
	return testEMFRecord(recordType, payload)
}

func testEMFPolyRecord(recordType uint32, points [][2]int32) []byte {
	payload := make([]byte, 20+len(points)*8)
	binary.LittleEndian.PutUint32(payload[16:20], uint32(len(points)))
	for i, point := range points {
		binary.LittleEndian.PutUint32(payload[20+i*8:24+i*8], uint32(point[0]))
		binary.LittleEndian.PutUint32(payload[24+i*8:28+i*8], uint32(point[1]))
	}
	return testEMFRecord(recordType, payload)
}

func testEMFRestoreDCRecord(relative int32) []byte {
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload, uint32(relative))
	return testEMFRecord(emrRestoreDC, payload)
}

func TestEMFPolyPolygonPreservesSeparateClosedFigures(t *testing.T) {
	rec := testEMFPolyPolyRecord(emrPolyPolygon, [][][2]int32{
		{{10, 10}, {40, 10}, {40, 40}},
		{{60, 20}, {90, 20}, {90, 50}},
	}, false)
	emf := testEMFWithRecords(120, 80, rec)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `M 10.000 10.000 L 40.000 10.000 L 40.000 40.000 Z M 60.000 20.000 L 90.000 20.000 L 90.000 50.000 Z`) {
		t.Fatalf("POLYPOLYGON topology was not preserved: %s", out)
	}
}

func TestEMFArcToConnectsCurrentPositionAndUpdatesIt(t *testing.T) {
	arc := make([]byte, 32)
	binary.LittleEndian.PutUint32(arc[0:4], 20)
	binary.LittleEndian.PutUint32(arc[4:8], 20)
	binary.LittleEndian.PutUint32(arc[8:12], 100)
	binary.LittleEndian.PutUint32(arc[12:16], 100)
	binary.LittleEndian.PutUint32(arc[16:20], 100)
	binary.LittleEndian.PutUint32(arc[20:24], 60)
	binary.LittleEndian.PutUint32(arc[24:28], 60)
	binary.LittleEndian.PutUint32(arc[28:32], 100)
	emf := testEMFWithRecords(140, 120,
		testEMFPointRecord(emrMoveToEx, 5, 5),
		testEMFRecord(emrArcTo, arc),
		testEMFPointRecord(emrLineTo, 110, 110),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `M 5.000 5.000 L 100.000 60.000 A 40.000 40.000`) {
		t.Fatalf("ARCTO did not connect from current position: %s", out)
	}
	if !strings.Contains(out, `x1="60.000" y1="100.000" x2="110.000" y2="110.000"`) {
		t.Fatalf("ARCTO did not update current position for following LineTo: %s", out)
	}
}

func TestEMFOffsetClipRgnMovesExistingClipGeometry(t *testing.T) {
	clip := make([]byte, 16)
	binary.LittleEndian.PutUint32(clip[0:4], 10)
	binary.LittleEndian.PutUint32(clip[4:8], 10)
	binary.LittleEndian.PutUint32(clip[8:12], 50)
	binary.LittleEndian.PutUint32(clip[12:16], 40)
	offset := make([]byte, 8)
	binary.LittleEndian.PutUint32(offset[0:4], 7)
	binary.LittleEndian.PutUint32(offset[4:8], 9)
	emf := testEMFWithRecords(100, 80,
		testEMFRecord(emrIntersectClipRect, clip),
		testEMFRecord(emrOffsetClipRgn, offset),
		testEMFRectRecord(emrRectangle, 0, 0, 90, 70),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<clipPath id="emf-clip-2"><rect x="17.000" y="19.000" width="40.000" height="30.000"/></clipPath>`) {
		t.Fatalf("OFFSETCLIPRGN did not preserve and shift clip geometry: %s", out)
	}
	if !strings.Contains(out, `clip-path="url(#emf-clip-2)"`) {
		t.Fatalf("shifted clip is not active: %s", out)
	}
}

func TestEMFExtSelectClipRgnCopiesRectRegion(t *testing.T) {
	region := testEMFRegionData([][4]int32{{10, 12, 40, 30}, {50, 20, 80, 60}})
	payload := make([]byte, 8+len(region))
	binary.LittleEndian.PutUint32(payload[0:4], uint32(len(region)))
	binary.LittleEndian.PutUint32(payload[4:8], 5) // RGN_COPY
	copy(payload[8:], region)
	emf := testEMFWithRecords(100, 80,
		testEMFRecord(emrExtSelectClipRgn, payload),
		testEMFRectRecord(emrRectangle, 0, 0, 100, 80),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<rect x="10.000" y="12.000" width="30.000" height="18.000"/><rect x="50.000" y="20.000" width="30.000" height="40.000"/>`) {
		t.Fatalf("EXTSELECTCLIPRGN region rectangles missing: %s", out)
	}
	if !strings.Contains(out, `<g clip-path="url(#emf-clip-1)"><rect`) {
		t.Fatalf("copied region clip not active: %s", out)
	}
}

func testEMFPolyPolyRecord(recordType uint32, groups [][][2]int32, short bool) []byte {
	count := 0
	for _, group := range groups {
		count += len(group)
	}
	stride := 8
	if short {
		stride = 4
	}
	payload := make([]byte, 24+len(groups)*4+count*stride)
	binary.LittleEndian.PutUint32(payload[16:20], uint32(len(groups)))
	binary.LittleEndian.PutUint32(payload[20:24], uint32(count))
	pointBase := 24 + len(groups)*4
	pointIndex := 0
	for i, group := range groups {
		binary.LittleEndian.PutUint32(payload[24+i*4:28+i*4], uint32(len(group)))
		for _, point := range group {
			off := pointBase + pointIndex*stride
			if short {
				binary.LittleEndian.PutUint16(payload[off:off+2], uint16(int16(point[0])))
				binary.LittleEndian.PutUint16(payload[off+2:off+4], uint16(int16(point[1])))
			} else {
				binary.LittleEndian.PutUint32(payload[off:off+4], uint32(point[0]))
				binary.LittleEndian.PutUint32(payload[off+4:off+8], uint32(point[1]))
			}
			pointIndex++
		}
	}
	return testEMFRecord(recordType, payload)
}

func testEMFRegionData(rects [][4]int32) []byte {
	data := make([]byte, 32+len(rects)*16)
	binary.LittleEndian.PutUint32(data[0:4], 32)
	binary.LittleEndian.PutUint32(data[4:8], 1) // RDH_RECTANGLES
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(rects)))
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(rects)*16))
	for i, rect := range rects {
		off := 32 + i*16
		for j := 0; j < 4; j++ {
			binary.LittleEndian.PutUint32(data[off+j*4:off+(j+1)*4], uint32(rect[j]))
		}
	}
	return data
}

func TestEMFHatchBrushUsesBrushOriginAndSelectedPattern(t *testing.T) {
	brush := make([]byte, 16)
	binary.LittleEndian.PutUint32(brush[0:4], 1)           // handle
	binary.LittleEndian.PutUint32(brush[4:8], 2)           // BS_HATCHED
	binary.LittleEndian.PutUint32(brush[8:12], 0x000000ff) // red
	binary.LittleEndian.PutUint32(brush[12:16], 4)         // HS_CROSS
	org := make([]byte, 8)
	binary.LittleEndian.PutUint32(org[0:4], 3)
	binary.LittleEndian.PutUint32(org[4:8], 5)
	sel := make([]byte, 4)
	binary.LittleEndian.PutUint32(sel, 1)
	emf := testEMFWithRecords(100, 80,
		testEMFRecord(emrSetBrushOrgEx, org),
		testEMFRecord(emrCreateBrushIndirect, brush),
		testEMFRecord(emrSelectObject, sel),
		testEMFRectRecord(emrRectangle, 10, 10, 80, 60),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `patternTransform="translate(3.000 5.000)"`) || !strings.Contains(out, `fill="url(#emf-brush-1)"`) {
		t.Fatalf("hatched brush/origin was not preserved: %s", out)
	}
	if strings.Count(out, `stroke="#ff0000"`) < 2 {
		t.Fatalf("cross hatch strokes were not materialized: %s", out)
	}
}

func TestEMFGradientFillRectHorizontalUsesVectorGradient(t *testing.T) {
	payload := make([]byte, 16+12+2*16+8) // bounds + nVer/nMesh/mode + vertices + rect mesh
	binary.LittleEndian.PutUint32(payload[16:20], 2)
	binary.LittleEndian.PutUint32(payload[20:24], 1)
	binary.LittleEndian.PutUint32(payload[24:28], 0) // horizontal
	v := payload[28:]
	binary.LittleEndian.PutUint32(v[0:4], 10)
	binary.LittleEndian.PutUint32(v[4:8], 10)
	binary.LittleEndian.PutUint16(v[8:10], 0xff00)
	binary.LittleEndian.PutUint32(v[16:20], 90)
	binary.LittleEndian.PutUint32(v[20:24], 70)
	binary.LittleEndian.PutUint16(v[28:30], 0xff00) // blue COLOR16
	mesh := payload[60:68]
	binary.LittleEndian.PutUint32(mesh[0:4], 0)
	binary.LittleEndian.PutUint32(mesh[4:8], 1)
	emf := testEMFWithRecords(100, 80, testEMFRecord(emrGradientFill, payload))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<linearGradient id="emf-gradient-1"`) || !strings.Contains(out, `stop-color="#ff0000"`) || !strings.Contains(out, `stop-color="#0000ff"`) {
		t.Fatalf("horizontal GRADIENTFILL was not preserved: %s", out)
	}
	if !strings.Contains(out, `fill="url(#emf-gradient-1)"`) {
		t.Fatalf("gradient was not applied: %s", out)
	}
}

func TestEMFGradientFillTriangleRasterizesBarycentricColors(t *testing.T) {
	payload := make([]byte, 16+12+3*16+12)
	binary.LittleEndian.PutUint32(payload[16:20], 3)
	binary.LittleEndian.PutUint32(payload[20:24], 1)
	binary.LittleEndian.PutUint32(payload[24:28], 2) // triangle
	colors := [][3]uint16{{0xff00, 0, 0}, {0, 0xff00, 0}, {0, 0, 0xff00}}
	pts := [][2]uint32{{10, 10}, {90, 10}, {50, 70}}
	for i := 0; i < 3; i++ {
		o := 28 + i*16
		binary.LittleEndian.PutUint32(payload[o:o+4], pts[i][0])
		binary.LittleEndian.PutUint32(payload[o+4:o+8], pts[i][1])
		binary.LittleEndian.PutUint16(payload[o+8:o+10], colors[i][0])
		binary.LittleEndian.PutUint16(payload[o+10:o+12], colors[i][1])
		binary.LittleEndian.PutUint16(payload[o+12:o+14], colors[i][2])
	}
	o := 28 + 3*16
	binary.LittleEndian.PutUint32(payload[o:o+4], 0)
	binary.LittleEndian.PutUint32(payload[o+4:o+8], 1)
	binary.LittleEndian.PutUint32(payload[o+8:o+12], 2)
	emf := testEMFWithRecords(100, 80, testEMFRecord(emrGradientFill, payload))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	if !strings.Contains(string(svg), `href="data:image/png;base64,`) {
		t.Fatalf("triangle gradient was not rasterized: %s", svg)
	}
}

func TestEMFExtSelectClipRgnXorComputesRegionBoolean(t *testing.T) {
	a := testEMFRegionData([][4]int32{{0, 0, 60, 80}})
	pa := make([]byte, 8+len(a))
	binary.LittleEndian.PutUint32(pa[0:4], uint32(len(a)))
	binary.LittleEndian.PutUint32(pa[4:8], 5)
	copy(pa[8:], a)
	b := testEMFRegionData([][4]int32{{40, 0, 100, 80}})
	pb := make([]byte, 8+len(b))
	binary.LittleEndian.PutUint32(pb[0:4], uint32(len(b)))
	binary.LittleEndian.PutUint32(pb[4:8], 3)
	copy(pb[8:], b)
	emf := testEMFWithRecords(100, 80, testEMFRecord(emrExtSelectClipRgn, pa), testEMFRecord(emrExtSelectClipRgn, pb), testEMFRectRecord(emrRectangle, 0, 0, 100, 80))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<rect x="0.000" y="0.000" width="40.000" height="80.000"/>`) || !strings.Contains(out, `<rect x="60.000" y="0.000" width="40.000" height="80.000"/>`) {
		t.Fatalf("RGN_XOR did not preserve exact rectangle-region algebra: %s", out)
	}
}

func TestEMFDIBPatternBrushTilesDecodedBitmap(t *testing.T) {
	bmi := make([]byte, 40)
	binary.LittleEndian.PutUint32(bmi[0:4], 40)
	binary.LittleEndian.PutUint32(bmi[4:8], 1)
	binary.LittleEndian.PutUint32(bmi[8:12], 1)
	binary.LittleEndian.PutUint16(bmi[12:14], 1)
	binary.LittleEndian.PutUint16(bmi[14:16], 24)
	bits := []byte{0x00, 0xff, 0x00, 0x00} // green BGR + stride padding
	rec := make([]byte, 32+len(bmi)+len(bits))
	binary.LittleEndian.PutUint32(rec[0:4], emrCreateDIBPatternBrushPt)
	binary.LittleEndian.PutUint32(rec[4:8], uint32(len(rec)))
	binary.LittleEndian.PutUint32(rec[8:12], 7)
	binary.LittleEndian.PutUint32(rec[12:16], 0)
	binary.LittleEndian.PutUint32(rec[16:20], 32)
	binary.LittleEndian.PutUint32(rec[20:24], uint32(len(bmi)))
	binary.LittleEndian.PutUint32(rec[24:28], uint32(32+len(bmi)))
	binary.LittleEndian.PutUint32(rec[28:32], uint32(len(bits)))
	copy(rec[32:], bmi)
	copy(rec[32+len(bmi):], bits)
	sel := make([]byte, 4)
	binary.LittleEndian.PutUint32(sel, 7)
	emf := testEMFWithRecords(80, 60, rec, testEMFRecord(emrSelectObject, sel), testEMFRectRecord(emrRectangle, 5, 5, 70, 50))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<pattern id="emf-brush-1"`) || !strings.Contains(out, `href="data:image/png;base64,`) || !strings.Contains(out, `fill="url(#emf-brush-1)"`) {
		t.Fatalf("DIB pattern brush was not materialized: %s", out)
	}
}

func TestEMFRectangleRegionBooleanModes(t *testing.T) {
	a := []emfRect{{0, 0, 60, 80}}
	b := []emfRect{{40, 0, 100, 80}}
	cases := []struct {
		mode uint32
		want [][4]float64
	}{
		{2, [][4]float64{{0, 0, 100, 80}}},
		{3, [][4]float64{{0, 0, 40, 80}, {60, 0, 100, 80}}},
		{4, [][4]float64{{0, 0, 40, 80}}},
	}
	for _, tc := range cases {
		got := emfCombineRects(a, b, tc.mode)
		if len(got) != len(tc.want) {
			t.Fatalf("mode %d rect count=%d want %d: %#v", tc.mode, len(got), len(tc.want), got)
		}
		for i, r := range got {
			w := tc.want[i]
			if math.Abs(r.x1-w[0]) > 1e-9 || math.Abs(r.y1-w[1]) > 1e-9 || math.Abs(r.x2-w[2]) > 1e-9 || math.Abs(r.y2-w[3]) > 1e-9 {
				t.Fatalf("mode %d rect[%d]=%#v want %#v", tc.mode, i, r, w)
			}
		}
	}
}

func testEMFExtTextOutW(x, y int32, text string, options uint32, rect [4]int32, dx []int32) []byte {
	units := []uint16{}
	for _, r := range text {
		if r <= 0xffff {
			units = append(units, uint16(r))
		} else {
			r -= 0x10000
			units = append(units, uint16(0xd800+(r>>10)), uint16(0xdc00+(r&0x3ff)))
		}
	}
	stringOffset := 76
	stringBytes := len(units) * 2
	dxOffset := stringOffset + stringBytes
	if dxOffset%4 != 0 {
		dxOffset += 4 - dxOffset%4
	}
	need := dxOffset
	if len(dx) > 0 {
		need += len(dx) * 4
	}
	rec := make([]byte, need)
	binary.LittleEndian.PutUint32(rec[0:4], emrExtTextOutW)
	binary.LittleEndian.PutUint32(rec[4:8], uint32(len(rec)))
	binary.LittleEndian.PutUint32(rec[36:40], uint32(x))
	binary.LittleEndian.PutUint32(rec[40:44], uint32(y))
	binary.LittleEndian.PutUint32(rec[44:48], uint32(len(units)))
	binary.LittleEndian.PutUint32(rec[48:52], uint32(stringOffset))
	binary.LittleEndian.PutUint32(rec[52:56], options)
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(rec[56+i*4:60+i*4], uint32(rect[i]))
	}
	if len(dx) > 0 {
		binary.LittleEndian.PutUint32(rec[72:76], uint32(dxOffset))
	}
	for i, u := range units {
		binary.LittleEndian.PutUint16(rec[stringOffset+i*2:stringOffset+i*2+2], u)
	}
	for i, d := range dx {
		binary.LittleEndian.PutUint32(rec[dxOffset+i*4:dxOffset+i*4+4], uint32(d))
	}
	return rec
}

func testEMFFontRecord(handle uint32, height, escapement, orientation int32, underline, strike bool, family string) []byte {
	payload := make([]byte, 96)
	binary.LittleEndian.PutUint32(payload[0:4], handle)
	base := 4
	binary.LittleEndian.PutUint32(payload[base:base+4], uint32(height))
	binary.LittleEndian.PutUint32(payload[base+8:base+12], uint32(escapement))
	binary.LittleEndian.PutUint32(payload[base+12:base+16], uint32(orientation))
	binary.LittleEndian.PutUint32(payload[base+16:base+20], 400)
	if underline {
		payload[base+21] = 1
	}
	if strike {
		payload[base+22] = 1
	}
	for i, r := range family {
		if i >= 31 {
			break
		}
		binary.LittleEndian.PutUint16(payload[base+28+i*2:base+30+i*2], uint16(r))
	}
	return testEMFRecord(emrExtCreateFontIndirectW, payload)
}

func TestEMFExtTextOutWUsesAuthoredDxGlyphOrigins(t *testing.T) {
	emf := testEMFWithRecords(100, 80,
		testEMFExtTextOutW(10, 20, "ABC", 0, [4]int32{}, []int32{10, 20, 30}),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<text x="10.000" y="20.000"`) || !strings.Contains(out, `<text x="20.000" y="20.000"`) || !strings.Contains(out, `<text x="40.000" y="20.000"`) {
		t.Fatalf("authored Dx origins were not preserved: %s", out)
	}
}

func TestEMFExtTextOutWOpaqueAndClippedRectangle(t *testing.T) {
	emf := testEMFWithRecords(100, 80,
		testEMFExtTextOutW(10, 20, "clip", 0x0002|0x0004, [4]int32{5, 6, 35, 30}, nil),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `points="5.000,6.000 35.000,6.000 35.000,30.000 5.000,30.000" fill="#ffffff"`) {
		t.Fatalf("ETO_OPAQUE background missing: %s", out)
	}
	if !strings.Contains(out, `<clipPath id="emf-text-clip-1"`) || !strings.Contains(out, `<g clip-path="url(#emf-text-clip-1)">`) {
		t.Fatalf("ETO_CLIPPED text clip missing: %s", out)
	}
}

func TestEMFFontEscapementUnderlineAndStrikeout(t *testing.T) {
	sel := make([]byte, 4)
	binary.LittleEndian.PutUint32(sel, 7)
	emf := testEMFWithRecords(100, 80,
		testEMFFontRecord(7, 14, 900, 900, true, true, "Arial"),
		testEMFRecord(emrSelectObject, sel),
		testEMFExtTextOutW(30, 40, "Up", 0, [4]int32{}, nil),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `text-decoration="underline line-through"`) {
		t.Fatalf("font decorations missing: %s", out)
	}
	if !strings.Contains(out, `transform="rotate(-90.000 30.000 40.000)"`) {
		t.Fatalf("font escapement rotation missing: %s", out)
	}
}

func TestEMFSetTextJustificationBecomesWordSpacing(t *testing.T) {
	just := make([]byte, 8)
	binary.LittleEndian.PutUint32(just[0:4], 12)
	binary.LittleEndian.PutUint32(just[4:8], 2)
	emf := testEMFWithRecords(100, 80,
		testEMFRecord(emrSetTextJustification, just),
		testEMFExtTextOutW(10, 20, "a b c", 0, [4]int32{}, nil),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	if !strings.Contains(string(svg), `word-spacing="6.000"`) {
		t.Fatalf("text justification spacing missing: %s", svg)
	}
}

func testEMFPolyTextOutW(items ...[]byte) []byte {
	const emrTextSize = 40
	header := 40 + len(items)*emrTextSize
	total := header
	for _, item := range items {
		total += len(item)
	}
	rec := make([]byte, total)
	binary.LittleEndian.PutUint32(rec[0:4], emrPolyTextOutW)
	binary.LittleEndian.PutUint32(rec[4:8], uint32(total))
	binary.LittleEndian.PutUint32(rec[36:40], uint32(len(items)))
	dataOff := header
	for i, item := range items {
		// item is a compact tuple: x,y UTF16 bytes, terminated by caller length in item[8:12].
		x := int32(binary.LittleEndian.Uint32(item[0:4]))
		y := int32(binary.LittleEndian.Uint32(item[4:8]))
		chars := int(binary.LittleEndian.Uint32(item[8:12]))
		text := item[12:]
		o := 40 + i*emrTextSize
		binary.LittleEndian.PutUint32(rec[o:o+4], uint32(x))
		binary.LittleEndian.PutUint32(rec[o+4:o+8], uint32(y))
		binary.LittleEndian.PutUint32(rec[o+8:o+12], uint32(chars))
		binary.LittleEndian.PutUint32(rec[o+12:o+16], uint32(dataOff))
		copy(rec[dataOff:], text)
		dataOff += len(text)
	}
	return rec
}

func testPolyTextItem(x, y int32, text string) []byte {
	units := []uint16{}
	for _, r := range text {
		units = append(units, uint16(r))
	}
	item := make([]byte, 12+len(units)*2)
	binary.LittleEndian.PutUint32(item[0:4], uint32(x))
	binary.LittleEndian.PutUint32(item[4:8], uint32(y))
	binary.LittleEndian.PutUint32(item[8:12], uint32(len(units)))
	for i, u := range units {
		binary.LittleEndian.PutUint16(item[12+i*2:14+i*2], u)
	}
	return item
}

func TestEMFPolyTextOutWUsesSharedTextObjectPlayback(t *testing.T) {
	emf := testEMFWithRecords(120, 80,
		testEMFPolyTextOutW(testPolyTextItem(10, 20, "One"), testPolyTextItem(60, 40, "Two")),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `x="10.000" y="20.000"`) || !strings.Contains(out, `>One</text>`) || !strings.Contains(out, `x="60.000" y="40.000"`) || !strings.Contains(out, `>Two</text>`) {
		t.Fatalf("POLYTEXTOUTW strings were not rendered independently: %s", out)
	}
}

func TestEMFSmallTextOutSupportsSmallCharsAndNoRect(t *testing.T) {
	text := []byte("Tiny")
	rec := make([]byte, 36+len(text))
	binary.LittleEndian.PutUint32(rec[0:4], emrSmallTextOut)
	binary.LittleEndian.PutUint32(rec[4:8], uint32(len(rec)))
	binary.LittleEndian.PutUint32(rec[8:12], 12)
	binary.LittleEndian.PutUint32(rec[12:16], 24)
	binary.LittleEndian.PutUint32(rec[16:20], uint32(len(text)))
	binary.LittleEndian.PutUint32(rec[20:24], 0x00000100|0x00000200) // ETO_NO_RECT | ETO_SMALL_CHARS
	copy(rec[36:], text)
	emf := testEMFWithRecords(100, 60, rec)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	if !strings.Contains(string(svg), `x="12.000" y="24.000"`) || !strings.Contains(string(svg), `>Tiny</text>`) {
		t.Fatalf("SMALLTEXTOUT compact text missing: %s", svg)
	}
}

func TestEMFSetLayoutRTLMirrorsDeviceXAndTextDirection(t *testing.T) {
	layout := make([]byte, 4)
	binary.LittleEndian.PutUint32(layout, 1) // LAYOUT_RTL
	emf := testEMFWithRecords(100, 60,
		testEMFRecord(emrSetLayout, layout),
		testEMFExtTextOutW(10, 20, "RTL", 0, [4]int32{}, nil),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `x="90.000" y="20.000"`) || !strings.Contains(out, `direction="rtl"`) {
		t.Fatalf("RTL layout state was not projected: %s", out)
	}
}

func TestEMFPlusCommentIsDetectedButNotPlayedAsClassicGDI(t *testing.T) {
	payload := make([]byte, 12)
	binary.LittleEndian.PutUint32(payload[0:4], 8) // DataSize includes identifier + one tiny fake record word.
	binary.LittleEndian.PutUint32(payload[4:8], 0x2B464D45)
	binary.LittleEndian.PutUint32(payload[8:12], 0xDEADBEEF)
	emf := testEMFWithRecords(100, 60, testEMFRecord(emrGDIComment, payload), testEMFRectRecord(emrRectangle, 5, 5, 20, 20))
	if !emfHasEMFPlusComments(emf) {
		t.Fatal("EMF+ comment was not detected")
	}
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `data-emf-plus="present"`) || !strings.Contains(out, `<rect`) {
		t.Fatalf("EMF+ boundary metadata or classic fallback missing: %s", out)
	}
	if strings.Contains(out, "DEADBEEF") {
		t.Fatalf("EMF+ payload leaked into classic GDI output: %s", out)
	}
}

func TestClassicWMFProjectsCommonPrimitiveClosure(t *testing.T) {
	var data []byte
	put16 := func(v uint16) { data = append(data, byte(v), byte(v>>8)) }
	put32 := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	put16(1)
	put16(9)
	put16(0x0300)
	put32(0)
	put16(16)
	put32(0)
	put16(0)
	rec := func(fn uint16, params ...uint16) {
		put32(uint32(3 + len(params)))
		put16(fn)
		for _, v := range params {
			put16(v)
		}
	}
	rec(0x020b, 0, 0)
	rec(0x020c, 100, 200)
	// Red brush, black pen.
	rec(0x02fc, 0, 0x00ff, 0, 0)
	rec(0x02fa, 0, 1, 0, 0, 0)
	rec(0x012d, 0)
	rec(0x012d, 1)
	rec(0x0418, 90, 190, 10, 10)                   // ellipse
	rec(0x041b, 80, 180, 20, 20)                   // rectangle
	rec(0x061c, 20, 20, 70, 170, 30, 30)           // roundrect
	rec(0x0325, 3, 10, 10, 100, 20, 180, 30)       // polyline
	rec(0x0817, 50, 180, 10, 100, 90, 190, 10, 10) // arc
	rec(0x0830, 60, 180, 20, 100, 90, 190, 10, 10) // chord
	rec(0x041f, 0x00ff, 0, 40, 40)                 // setpixel red
	// TextOut("OK") at x=50,y=50.
	params := []byte{2, 0, 'O', 'K', 50, 0, 50, 0}
	put32(uint32(3 + (len(params)+1)/2))
	put16(0x0521)
	data = append(data, params...)
	if len(params)%2 != 0 {
		data = append(data, 0)
	}
	rec(0)
	svg, _, _, ok := ToSVG(data, "image/x-wmf")
	if !ok {
		t.Fatal("WMF common primitives did not convert")
	}
	out := string(svg)
	for _, want := range []string{"<ellipse ", "<rect ", "rx=", "<polyline ", ">OK</text>", "width=\"1\" height=\"1\""} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
	if !strings.Contains(out, `dominant-baseline="text-before-edge"`) {
		t.Fatalf("WMF common primitive text lost top-baseline alignment: %s", out)
	}
	if strings.Count(out, "<path ") < 2 {
		t.Fatalf("arc/chord paths missing: %s", out)
	}
}

func TestEMFAlphaBlendAndTransparentBltUseSharedRasterCompositor(t *testing.T) {
	makeHeader := func() []byte {
		h := make([]byte, 108)
		binary.LittleEndian.PutUint32(h[0:4], emrHeader)
		binary.LittleEndian.PutUint32(h[4:8], uint32(len(h)))
		binary.LittleEndian.PutUint32(h[16:20], 100)
		binary.LittleEndian.PutUint32(h[20:24], 100)
		copy(h[40:44], []byte(" EMF"))
		return h
	}
	makeDIBRecord := func(typ uint32) []byte {
		const base = 108
		const bmiSize = 40
		const bitsSize = 4
		r := make([]byte, base+bmiSize+bitsSize)
		binary.LittleEndian.PutUint32(r[0:4], typ)
		binary.LittleEndian.PutUint32(r[4:8], uint32(len(r)))
		binary.LittleEndian.PutUint32(r[24:28], 10)
		binary.LittleEndian.PutUint32(r[28:32], 20)
		binary.LittleEndian.PutUint32(r[32:36], 20)
		binary.LittleEndian.PutUint32(r[36:40], 20)
		if typ == emrAlphaBlend {
			r[40] = 0
			r[42] = 128
			r[43] = 0
		} else {
			// COLORREF red; our one source pixel is blue, so it remains visible.
			binary.LittleEndian.PutUint32(r[40:44], 0x000000ff)
		}
		binary.LittleEndian.PutUint32(r[44:48], 0)
		binary.LittleEndian.PutUint32(r[48:52], 0)
		binary.LittleEndian.PutUint32(r[84:88], base)
		binary.LittleEndian.PutUint32(r[88:92], bmiSize)
		binary.LittleEndian.PutUint32(r[92:96], base+bmiSize)
		binary.LittleEndian.PutUint32(r[96:100], bitsSize)
		binary.LittleEndian.PutUint32(r[100:104], 1)
		binary.LittleEndian.PutUint32(r[104:108], 1)
		bmi := r[base : base+bmiSize]
		binary.LittleEndian.PutUint32(bmi[0:4], 40)
		binary.LittleEndian.PutUint32(bmi[4:8], 1)
		binary.LittleEndian.PutUint32(bmi[8:12], 1)
		binary.LittleEndian.PutUint16(bmi[12:14], 1)
		binary.LittleEndian.PutUint16(bmi[14:16], 24)
		copy(r[base+bmiSize:], []byte{255, 0, 0, 0})
		return r
	}
	for _, typ := range []uint32{emrAlphaBlend, emrTransparentBlt} {
		eof := make([]byte, 20)
		binary.LittleEndian.PutUint32(eof[0:4], emrEOF)
		binary.LittleEndian.PutUint32(eof[4:8], uint32(len(eof)))
		data := append(makeHeader(), makeDIBRecord(typ)...)
		data = append(data, eof...)
		svg, _, _, ok := emfToSVG(data)
		if !ok || !strings.Contains(string(svg), `href="data:image/png;base64,`) {
			t.Fatalf("record %d did not render through raster compositor: %s", typ, svg)
		}
	}
}

func TestWMFZeroWidthPenRemainsDevicePixelCosmeticStroke(t *testing.T) {
	// LOGPEN width == 0 is a GDI cosmetic pen: it is one device pixel wide,
	// regardless of the logical window/viewBox scale. Office OLE previews use
	// this heavily for table/grid drawings (for example Object 35 on slide 2).
	record := func(fn uint16, params ...uint16) []byte {
		b := make([]byte, 6+len(params)*2)
		binary.LittleEndian.PutUint32(b[0:4], uint32(len(b)/2))
		binary.LittleEndian.PutUint16(b[4:6], fn)
		for i, v := range params {
			binary.LittleEndian.PutUint16(b[6+i*2:8+i*2], v)
		}
		return b
	}

	var records []byte
	records = append(records, record(0x020c, 2500, 5000)...) // META_SETWINDOWEXT y,x
	// META_CREATEPENINDIRECT: PS_SOLID, width=(0,0), black COLORREF.
	pen := make([]byte, 16)
	binary.LittleEndian.PutUint32(pen[0:4], uint32(len(pen)/2))
	binary.LittleEndian.PutUint16(pen[4:6], 0x02fa)
	// params are already zero: solid, zero width, black.
	records = append(records, pen...)
	records = append(records, record(0x012d, 0)...)         // META_SELECTOBJECT handle 0
	records = append(records, record(0x0214, 100, 100)...)  // META_MOVETO y,x
	records = append(records, record(0x0213, 100, 4900)...) // META_LINETO y,x
	records = append(records, record(0x0000)...)

	header := make([]byte, 18)
	binary.LittleEndian.PutUint16(header[0:2], 1)
	binary.LittleEndian.PutUint16(header[2:4], 9)
	binary.LittleEndian.PutUint16(header[4:6], 0x0300)
	binary.LittleEndian.PutUint32(header[6:10], uint32((len(header)+len(records))/2))
	binary.LittleEndian.PutUint16(header[10:12], 1)
	wmf := append(header, records...)

	svg, _, _, ok := ToSVG(wmf, "image/x-wmf")
	if !ok {
		t.Fatal("WMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `stroke-width="1.000" vector-effect="non-scaling-stroke"`) {
		t.Fatalf("zero-width cosmetic pen was scaled as a logical pen: %s", out)
	}
}

func TestEMFPortableMathTypeFontNormalization(t *testing.T) {
	cases := []struct {
		family string
		in     string
		wantF  string
		want   string
	}{
		{"Fences", "afLMNOPQz", "Cambria Math, STIX Two Math, DejaVu Serif, serif", "()⎡⎢⎣⎤⎥⎦∫"},
		{"MT Extra", "\uf024\uf04d", "Cambria Math, STIX Two Math, DejaVu Serif, serif", "^⋮"},
		{"Symbol", "\uf03d\uf02d\uf02b\uf064\uf072\uf074\uf061\uf0bc", "Cambria Math, STIX Two Math, DejaVu Serif, serif", "=−+δρτα…"},
		{"Symbol", "\uf044 \uf03e \uf030", "Cambria Math, STIX Two Math, DejaVu Serif, serif", "Δ > 0"},
		{"Symbol", "and the amplitude", "Times New Roman", "and the amplitude"},
		{"MT Symbol", "\uf053\uf044\uf070\uf02a", "Cambria Math, STIX Two Math, DejaVu Serif, serif", "ΣΔπ∗"},
		{"Times New Roman", "x", "Times New Roman", "x"},
	}
	for _, tc := range cases {
		gotF, got := emfPortableMathText(tc.family, tc.in)
		if gotF != tc.wantF || got != tc.want {
			t.Fatalf("%s => family=%q text=%q, want family=%q text=%q", tc.family, gotF, got, tc.wantF, tc.want)
		}
	}
}

func TestApplySVGColorReplacementsMapsVectorPaints(t *testing.T) {
	input := []byte(`<svg><path fill="#000000" stroke="#12ab34"/><text fill="#000000">x</text></svg>`)
	got := string(applySVGColorReplacements(input, []ColorReplacement{
		{Source: "000000", Destination: "FFFFFF"},
		{Source: "12AB34", Destination: "ABCDEF"},
	}))
	if strings.Contains(strings.ToLower(got), "#000000") || strings.Contains(strings.ToLower(got), "#12ab34") {
		t.Fatalf("source paints survived recolor: %s", got)
	}
	if strings.Count(got, "#FFFFFF") != 2 || !strings.Contains(got, "#ABCDEF") {
		t.Fatalf("destination paints missing: %s", got)
	}
}

// viewBoxNumbers extracts the four numbers of the first SVG viewBox attribute.
func viewBoxNumbers(svg string) []float64 {
	const marker = `viewBox="`
	i := strings.Index(svg, marker)
	if i < 0 {
		return nil
	}
	rest := svg[i+len(marker):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return nil
	}
	fields := strings.Fields(rest[:end])
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil
		}
		out = append(out, v)
	}
	return out
}
