package metafile

import (
	"encoding/binary"
	"math"
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
