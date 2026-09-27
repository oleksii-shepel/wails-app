package metafile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"
)

func testEMFPlusRecord(typ uint16, flags uint16, data []byte) []byte {
	size := 12 + len(data)
	if rem := size % 4; rem != 0 {
		size += 4 - rem
	}
	rec := make([]byte, size)
	binary.LittleEndian.PutUint16(rec[0:2], typ)
	binary.LittleEndian.PutUint16(rec[2:4], flags)
	binary.LittleEndian.PutUint32(rec[4:8], uint32(size))
	binary.LittleEndian.PutUint32(rec[8:12], uint32(len(data)))
	copy(rec[12:], data)
	return rec
}

func testEMFPlusComment(records ...[]byte) []byte {
	var stream []byte
	for _, r := range records {
		stream = append(stream, r...)
	}
	dataSize := 4 + len(stream)
	payload := make([]byte, 4+dataSize)
	binary.LittleEndian.PutUint32(payload[0:4], uint32(dataSize))
	binary.LittleEndian.PutUint32(payload[4:8], 0x2B464D45)
	copy(payload[8:], stream)
	return testEMFRecord(emrGDIComment, payload)
}

func testEMFPlusHeader(flags uint16) []byte {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 1)
	binary.LittleEndian.PutUint32(data[8:12], 96)
	binary.LittleEndian.PutUint32(data[12:16], 96)
	return testEMFPlusRecord(emfPlusHeader, flags, data)
}

func testEMFPlusSolidBrushObject(id uint8, argb uint32) []byte {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 0)
	binary.LittleEndian.PutUint32(data[8:12], argb)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectBrush)<<8|uint16(id), data)
}

func testEMFPlusPathObject(id uint8, points [][2]float32, types []byte) []byte {
	data := make([]byte, 12+len(points)*8+len(types))
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(points)))
	for i, pt := range points {
		off := 12 + i*8
		binary.LittleEndian.PutUint32(data[off:off+4], math.Float32bits(pt[0]))
		binary.LittleEndian.PutUint32(data[off+4:off+8], math.Float32bits(pt[1]))
	}
	copy(data[12+len(points)*8:], types)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectPath)<<8|uint16(id), data)
}

func testEMFPlusSimplePenObject(id uint8, width float32, argb uint32) []byte {
	data := make([]byte, 32)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 0)
	binary.LittleEndian.PutUint32(data[8:12], 0)  // PenDataFlags
	binary.LittleEndian.PutUint32(data[12:16], 2) // UnitPixel
	binary.LittleEndian.PutUint32(data[16:20], math.Float32bits(width))
	binary.LittleEndian.PutUint32(data[20:24], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[24:28], 0) // solid brush
	binary.LittleEndian.PutUint32(data[28:32], argb)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectPen)<<8|uint16(id), data)
}

func TestEMFPlusPureStreamRendersSolidFillPath(t *testing.T) {
	path := testEMFPlusPathObject(2, [][2]float32{{10, 10}, {40, 10}, {40, 30}}, []byte{0, 1, 0x81})
	fillData := make([]byte, 4)
	binary.LittleEndian.PutUint32(fillData, 1)
	fill := testEMFPlusRecord(emfPlusFillPath, 2, fillData)
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusSolidBrushObject(1, 0xFFFF0000),
		path,
		fill,
		testEMFPlusRecord(emfPlusEndOfFile, 0, nil),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `data-emf-plus-mode="only"`) || !strings.Contains(out, `data-emf-plus-rendered="true"`) {
		t.Fatalf("EMF+ ownership metadata missing: %s", out)
	}
	if !strings.Contains(out, `d="M 10.000 10.000 L 40.000 10.000 L 40.000 30.000 Z"`) || !strings.Contains(out, `fill="#FF0000"`) {
		t.Fatalf("EMF+ path fill missing: %s", out)
	}
}

func TestEMFPlusWorldTransformAppliesToDrawPath(t *testing.T) {
	path := testEMFPlusPathObject(2, [][2]float32{{1, 2}, {5, 2}}, []byte{0, 1})
	transform := make([]byte, 24)
	vals := []float32{2, 0, 0, 3, 7, 11}
	for i, v := range vals {
		binary.LittleEndian.PutUint32(transform[i*4:i*4+4], math.Float32bits(v))
	}
	drawData := make([]byte, 4)
	binary.LittleEndian.PutUint32(drawData, 3)
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusSimplePenObject(3, 2, 0xFF123456),
		path,
		testEMFPlusRecord(emfPlusSetWorldTransform, 0, transform),
		testEMFPlusRecord(emfPlusDrawPath, 2, drawData),
		testEMFPlusRecord(emfPlusEndOfFile, 0, nil),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `stroke="#123456"`) || !strings.Contains(out, `stroke-width="2.000"`) || !strings.Contains(out, `transform="matrix(2.000000 0.000000 0.000000 3.000000 7.000000 11.000000)"`) {
		t.Fatalf("EMF+ transformed pen path missing: %s", out)
	}
}

func TestEMFPlusDualKeepsClassicFallback(t *testing.T) {
	fillData := make([]byte, 4)
	binary.LittleEndian.PutUint32(fillData, 0xFF00FF00)
	emf := testEMFWithRecords(100, 60,
		testEMFPlusComment(testEMFPlusHeader(1), testEMFPlusRecord(emfPlusClear, 0, fillData)),
		testEMFRectRecord(emrRectangle, 5, 5, 20, 20),
	)
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `data-emf-plus-mode="dual"`) || !strings.Contains(out, `data-emf-plus-rendered="false"`) {
		t.Fatalf("dual metadata missing: %s", out)
	}
	if !strings.Contains(out, `<rect x="5.000" y="5.000"`) || strings.Contains(out, `fill="#00FF00"`) {
		t.Fatalf("dual stream did not preserve classic fallback: %s", out)
	}
}

func testEMFPlusSaveRecord(typ uint16, index uint32) []byte {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, index)
	return testEMFPlusRecord(typ, 0, data)
}

func TestEMFPlusSaveRestoreRestoresTransform(t *testing.T) {
	path := testEMFPlusPathObject(2, [][2]float32{{0, 0}, {10, 0}}, []byte{0, 1})
	drawData := make([]byte, 4)
	binary.LittleEndian.PutUint32(drawData, 3)
	translate := make([]byte, 8)
	binary.LittleEndian.PutUint32(translate[0:4], math.Float32bits(25))
	binary.LittleEndian.PutUint32(translate[4:8], math.Float32bits(5))
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusSimplePenObject(3, 1, 0xFF000000),
		path,
		testEMFPlusSaveRecord(emfPlusSave, 7),
		testEMFPlusRecord(emfPlusTranslateWorldTransform, 0x2000, translate),
		testEMFPlusRecord(emfPlusDrawPath, 2, drawData),
		testEMFPlusSaveRecord(emfPlusRestore, 7),
		testEMFPlusRecord(emfPlusDrawPath, 2, drawData),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `25.000000 5.000000`) || !strings.Contains(out, `1.000000 0.000000 0.000000 1.000000 0.000000 0.000000`) {
		t.Fatalf("save/restore transforms missing: %s", out)
	}
}

func TestEMFPlusSetClipPathAppliesToDrawing(t *testing.T) {
	clipPath := testEMFPlusPathObject(1, [][2]float32{{0, 0}, {20, 0}, {20, 20}, {0, 20}}, []byte{0, 1, 1, 0x81})
	drawPath := testEMFPlusPathObject(2, [][2]float32{{0, 10}, {40, 10}}, []byte{0, 1})
	drawData := make([]byte, 4)
	binary.LittleEndian.PutUint32(drawData, 3)
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(
		testEMFPlusHeader(0), testEMFPlusSimplePenObject(3, 1, 0xFF112233), clipPath, drawPath,
		testEMFPlusRecord(emfPlusSetClipPath, 1, nil),
		testEMFPlusRecord(emfPlusDrawPath, 2, drawData),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<mask id="emfplus-mask-1"`) || !strings.Contains(out, `mask="url(#emfplus-mask-1)"`) {
		t.Fatalf("clip path not applied: %s", out)
	}
}

func testTinyPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	img.SetNRGBA(0, 1, color.NRGBA{B: 255, A: 255})
	img.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, A: 255})
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testEMFPlusCompressedImageObject(id uint8, payload []byte) []byte {
	data := make([]byte, 28+len(payload))
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 1) // ImageDataTypeBitmap
	binary.LittleEndian.PutUint32(data[8:12], 2)
	binary.LittleEndian.PutUint32(data[12:16], 2)
	binary.LittleEndian.PutUint32(data[24:28], 1) // BitmapDataTypeCompressed
	copy(data[28:], payload)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectImage)<<8|uint16(id), data)
}

func TestEMFPlusDrawImageRendersCompressedBitmap(t *testing.T) {
	data := make([]byte, 40)
	binary.LittleEndian.PutUint32(data[4:8], 2) // SrcUnitPixel
	for i, v := range []float32{0, 0, 2, 2, 10, 12, 30, 20} {
		binary.LittleEndian.PutUint32(data[8+i*4:12+i*4], math.Float32bits(v))
	}
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusCompressedImageObject(4, testTinyPNG(t)), testEMFPlusRecord(emfPlusDrawImage, 4, data)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `data:image/png;base64,`) || !strings.Contains(out, `viewBox="0.000 0.000 2.000 2.000"`) || !strings.Contains(out, `matrix(15.000000`) {
		t.Fatalf("draw image missing: %s", out)
	}
}

func TestEMFPlusContinuedImageObjectAssemblesBeforeDrawing(t *testing.T) {
	pngData := testTinyPNG(t)
	obj := make([]byte, 28+len(pngData))
	binary.LittleEndian.PutUint32(obj[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(obj[4:8], 1)
	binary.LittleEndian.PutUint32(obj[8:12], 2)
	binary.LittleEndian.PutUint32(obj[12:16], 2)
	binary.LittleEndian.PutUint32(obj[24:28], 1)
	copy(obj[28:], pngData)
	cut := len(obj) / 2
	first := make([]byte, 4+cut)
	binary.LittleEndian.PutUint32(first[:4], uint32(len(obj)))
	copy(first[4:], obj[:cut])
	second := append([]byte(nil), obj[cut:]...)
	flags := uint16(emfPlusObjectImage)<<8 | 5
	data := make([]byte, 40)
	binary.LittleEndian.PutUint32(data[4:8], 2)
	for i, v := range []float32{0, 0, 2, 2, 0, 0, 20, 20} {
		binary.LittleEndian.PutUint32(data[8+i*4:12+i*4], math.Float32bits(v))
	}
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusRecord(emfPlusObject, flags|0x8000, first), testEMFPlusRecord(emfPlusObject, flags, second), testEMFPlusRecord(emfPlusDrawImage, 5, data)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	if !strings.Contains(string(svg), `data:image/png;base64,`) {
		t.Fatalf("continued image was not assembled: %s", svg)
	}
}

func TestEMFPlusDrawImagePointsUsesParallelogram(t *testing.T) {
	data := make([]byte, 52)
	binary.LittleEndian.PutUint32(data[4:8], 2) // SrcUnitPixel
	for i, v := range []float32{0, 0, 2, 2} {
		binary.LittleEndian.PutUint32(data[8+i*4:12+i*4], math.Float32bits(v))
	}
	binary.LittleEndian.PutUint32(data[24:28], 3)
	pts := []float32{10, 10, 30, 12, 12, 40}
	for i, v := range pts {
		binary.LittleEndian.PutUint32(data[28+i*4:32+i*4], math.Float32bits(v))
	}
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusCompressedImageObject(4, testTinyPNG(t)), testEMFPlusRecord(emfPlusDrawImagePoints, 4, data)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `matrix(10.000000 1.000000 1.000000 15.000000 10.000000 10.000000)`) {
		t.Fatalf("parallelogram transform missing: %s", out)
	}
}

func TestEMFPlusBeginContainerRestoresByStackIndex(t *testing.T) {
	path := testEMFPlusPathObject(2, [][2]float32{{0, 0}, {5, 0}}, []byte{0, 1})
	draw := make([]byte, 4)
	binary.LittleEndian.PutUint32(draw, 3)
	container := make([]byte, 36)
	vals := []float32{0, 0, 10, 10, 0, 0, 20, 30}
	for i, v := range vals {
		binary.LittleEndian.PutUint32(container[i*4:i*4+4], math.Float32bits(v))
	}
	binary.LittleEndian.PutUint32(container[32:36], 9)
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusSimplePenObject(3, 1, 0xFF000000), path, testEMFPlusRecord(emfPlusBeginContainer, 2, container), testEMFPlusRecord(emfPlusDrawPath, 2, draw), testEMFPlusSaveRecord(emfPlusEndContainer, 9), testEMFPlusRecord(emfPlusDrawPath, 2, draw)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `matrix(2.000000 0.000000 0.000000 3.000000`) || !strings.Contains(out, `matrix(1.000000 0.000000 0.000000 1.000000 0.000000 0.000000)`) {
		t.Fatalf("container state missing: %s", out)
	}
}

func testEMFPlusLinearGradientBrushObject(id uint8, rect [4]float32, start, end uint32) []byte {
	data := make([]byte, 48)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 4) // LinearGradientBrush
	binary.LittleEndian.PutUint32(data[8:12], 0)
	binary.LittleEndian.PutUint32(data[12:16], 0) // WrapModeClamp
	for i, v := range rect {
		binary.LittleEndian.PutUint32(data[16+i*4:20+i*4], math.Float32bits(v))
	}
	binary.LittleEndian.PutUint32(data[32:36], start)
	binary.LittleEndian.PutUint32(data[36:40], end)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectBrush)<<8|uint16(id), data)
}

func testEMFPlusTextureBrushObject(id uint8, pngData []byte) []byte {
	img := make([]byte, 28+len(pngData))
	binary.LittleEndian.PutUint32(img[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(img[4:8], 1)
	binary.LittleEndian.PutUint32(img[8:12], 2)
	binary.LittleEndian.PutUint32(img[12:16], 2)
	binary.LittleEndian.PutUint32(img[24:28], 1)
	copy(img[28:], pngData)
	data := make([]byte, 16+len(img))
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 2) // TextureBrush
	binary.LittleEndian.PutUint32(data[8:12], 0)
	binary.LittleEndian.PutUint32(data[12:16], 0)
	copy(data[16:], img)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectBrush)<<8|uint16(id), data)
}

func TestEMFPlusLinearGradientBrushFillsEllipse(t *testing.T) {
	fill := make([]byte, 20)
	binary.LittleEndian.PutUint32(fill[:4], 6)
	for i, v := range []float32{10, 12, 40, 20} {
		binary.LittleEndian.PutUint32(fill[4+i*4:8+i*4], math.Float32bits(v))
	}
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusLinearGradientBrushObject(6, [4]float32{10, 12, 40, 20}, 0xFFFF0000, 0xFF0000FF), testEMFPlusRecord(emfPlusFillEllipse, 0, fill)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<linearGradient id="emfplus-gradient-1"`) || !strings.Contains(out, `fill="url(#emfplus-gradient-1)"`) || !strings.Contains(out, `<ellipse`) {
		t.Fatalf("gradient ellipse missing: %s", out)
	}
}

func TestEMFPlusTextureBrushFillsPolygon(t *testing.T) {
	data := make([]byte, 8+3*8)
	binary.LittleEndian.PutUint32(data[:4], 7)
	binary.LittleEndian.PutUint32(data[4:8], 3)
	pts := []float32{0, 0, 30, 0, 10, 20}
	for i, v := range pts {
		binary.LittleEndian.PutUint32(data[8+i*4:12+i*4], math.Float32bits(v))
	}
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusTextureBrushObject(7, testTinyPNG(t)), testEMFPlusRecord(emfPlusFillPolygon, 0, data)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<pattern id="emfplus-pattern-1"`) || !strings.Contains(out, `fill="url(#emfplus-pattern-1)"`) || !strings.Contains(out, `<polygon`) {
		t.Fatalf("texture polygon missing: %s", out)
	}
}

func TestEMFPlusCompressedDrawRectsAndArc(t *testing.T) {
	rects := make([]byte, 12)
	binary.LittleEndian.PutUint32(rects[:4], 1)
	for i, v := range []int16{5, 6, 20, 12} {
		binary.LittleEndian.PutUint16(rects[4+i*2:6+i*2], uint16(v))
	}
	arc := make([]byte, 16)
	binary.LittleEndian.PutUint32(arc[0:4], math.Float32bits(0))
	binary.LittleEndian.PutUint32(arc[4:8], math.Float32bits(90))
	for i, v := range []int16{10, 10, 30, 20} {
		binary.LittleEndian.PutUint16(arc[8+i*2:10+i*2], uint16(v))
	}
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusSimplePenObject(3, 2, 0xFF123456), testEMFPlusRecord(emfPlusDrawRects, 0x4000|3, rects), testEMFPlusRecord(emfPlusDrawArc, 0x4000|3, arc)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `x="5.000" y="6.000" width="20.000" height="12.000"`) || !strings.Contains(out, `A 15.000 10.000`) || !strings.Contains(out, `stroke="#123456"`) {
		t.Fatalf("compressed primitives missing: %s", out)
	}
}

func testEMFPlusRectRegionObject(id uint8, r [4]float32) []byte {
	data := make([]byte, 28)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 0) // root-only region: zero child nodes
	binary.LittleEndian.PutUint32(data[8:12], 0x10000000)
	for i, v := range r {
		binary.LittleEndian.PutUint32(data[12+i*4:16+i*4], math.Float32bits(v))
	}
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectRegion)<<8|uint16(id), data)
}

func testEMFPlusImageAttributesObject(id uint8, wrap uint32, clamp uint32) []byte {
	data := make([]byte, 24)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 0)
	binary.LittleEndian.PutUint32(data[8:12], wrap)
	binary.LittleEndian.PutUint32(data[12:16], clamp)
	binary.LittleEndian.PutUint32(data[16:20], 0) // RectClamp
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectImageAttributes)<<8|uint16(id), data)
}

func TestEMFPlusFillRegionAndSetClipRegionUseRegionObject(t *testing.T) {
	fill := make([]byte, 4)
	binary.LittleEndian.PutUint32(fill, 2)
	drawPath := testEMFPlusPathObject(3, [][2]float32{{0, 10}, {50, 10}}, []byte{0, 1})
	draw := make([]byte, 4)
	binary.LittleEndian.PutUint32(draw, 4)
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusRectRegionObject(1, [4]float32{5, 6, 20, 15}),
		testEMFPlusSolidBrushObject(2, 0xFF336699),
		testEMFPlusSimplePenObject(4, 1, 0xFF000000),
		drawPath,
		testEMFPlusRecord(emfPlusFillRegion, 1, fill),
		testEMFPlusRecord(emfPlusSetClipRegion, 1, nil),
		testEMFPlusRecord(emfPlusDrawPath, 3, draw),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `x="5.000" y="6.000" width="20.000" height="15.000"`) || !strings.Contains(out, `fill="#336699"`) {
		t.Fatalf("region fill missing: %s", out)
	}
	if !strings.Contains(out, `mask="url(#emfplus-mask-`) {
		t.Fatalf("region clip missing: %s", out)
	}
}

func TestEMFPlusImageAttributesClampColorAppliesToDrawImage(t *testing.T) {
	data := make([]byte, 40)
	binary.LittleEndian.PutUint32(data[0:4], 8) // ImageAttributesID
	binary.LittleEndian.PutUint32(data[4:8], 2) // SrcUnitPixel
	for i, v := range []float32{-1, -1, 4, 4, 10, 12, 30, 20} {
		binary.LittleEndian.PutUint32(data[8+i*4:12+i*4], math.Float32bits(v))
	}
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusCompressedImageObject(4, testTinyPNG(t)),
		testEMFPlusImageAttributesObject(8, 4, 0xFFFF00FF), // WrapModeClamp
		testEMFPlusRecord(emfPlusDrawImage, 4, data),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `fill="#FF00FF"`) || !strings.Contains(out, `data:image/png;base64,`) {
		t.Fatalf("image attributes clamp missing: %s", out)
	}
}

func TestEMFPlusCompositingStateIsSavedAndRestored(t *testing.T) {
	path := testEMFPlusPathObject(2, [][2]float32{{0, 0}, {10, 0}}, []byte{0, 1})
	draw := make([]byte, 4)
	binary.LittleEndian.PutUint32(draw, 3)
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(
		testEMFPlusHeader(0), testEMFPlusSimplePenObject(3, 1, 0xFF000000), path,
		testEMFPlusSaveRecord(emfPlusSave, 9),
		testEMFPlusRecord(emfPlusSetCompositingMode, 1, nil),
		testEMFPlusRecord(emfPlusSetCompositingQuality, 3, nil),
		testEMFPlusRecord(emfPlusDrawPath, 2, draw),
		testEMFPlusSaveRecord(emfPlusRestore, 9),
		testEMFPlusRecord(emfPlusDrawPath, 2, draw),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if strings.Count(out, `data-emfplus-compositing="source-copy"`) != 1 || strings.Count(out, `data-emfplus-compositing-quality="3"`) != 1 {
		t.Fatalf("compositing save/restore state wrong: %s", out)
	}
}

func testEMFPlusRegionBinaryNode(typ uint32, left, right []byte) []byte {
	b := make([]byte, 4+len(left)+len(right))
	binary.LittleEndian.PutUint32(b[:4], typ)
	copy(b[4:], left)
	copy(b[4+len(left):], right)
	return b
}

func testEMFPlusRegionRectNode(r [4]float32) []byte {
	b := make([]byte, 20)
	binary.LittleEndian.PutUint32(b[:4], 0x10000000)
	for i, v := range r {
		binary.LittleEndian.PutUint32(b[4+i*4:8+i*4], math.Float32bits(v))
	}
	return b
}

func testEMFPlusRegionObject(id uint8, root []byte) []byte {
	data := make([]byte, 8+len(root))
	binary.LittleEndian.PutUint32(data[:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 2)
	copy(data[8:], root)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectRegion)<<8|uint16(id), data)
}

func TestEMFPlusRegionBooleanMasksCoverXorExcludeComplement(t *testing.T) {
	left := testEMFPlusRegionRectNode([4]float32{0, 0, 20, 20})
	right := testEMFPlusRegionRectNode([4]float32{10, 0, 20, 20})
	for _, tc := range []struct {
		name string
		typ  uint32
	}{
		{"xor", 3},
		{"exclude", 4},
		{"complement", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fill := make([]byte, 4)
			binary.LittleEndian.PutUint32(fill, 0xFF336699)
			root := testEMFPlusRegionBinaryNode(tc.typ, left, right)
			emf := testEMFWithRecords(100, 60, testEMFPlusComment(
				testEMFPlusHeader(0),
				testEMFPlusRegionObject(1, root),
				testEMFPlusRecord(emfPlusFillRegion, 0x8000|1, fill),
			))
			svg, _, _, ok := emfToSVG(emf)
			if !ok {
				t.Fatal("EMF conversion failed")
			}
			out := string(svg)
			if !strings.Contains(out, `<mask id="emfplus-mask-3"`) || !strings.Contains(out, `mask="url(#emfplus-mask-3)"`) {
				t.Fatalf("boolean region mask missing: %s", out)
			}
			if !strings.Contains(out, `fill="#336699"`) {
				t.Fatalf("region paint missing: %s", out)
			}
		})
	}
}

func TestEMFPlusClipCombineModesUseMaskAlgebra(t *testing.T) {
	pathObj := testEMFPlusPathObject(3, [][2]float32{{0, 0}, {60, 0}, {60, 40}, {0, 40}}, []byte{0, 1, 1, 0x81})
	draw := make([]byte, 4)
	binary.LittleEndian.PutUint32(draw, 4)
	for mode := uint16(1); mode <= 5; mode++ {
		t.Run(fmt.Sprintf("mode-%d", mode), func(t *testing.T) {
			r1 := make([]byte, 16)
			r2 := make([]byte, 16)
			for i, v := range []float32{0, 0, 30, 30} {
				binary.LittleEndian.PutUint32(r1[i*4:i*4+4], math.Float32bits(v))
			}
			for i, v := range []float32{15, 0, 30, 30} {
				binary.LittleEndian.PutUint32(r2[i*4:i*4+4], math.Float32bits(v))
			}
			emf := testEMFWithRecords(100, 60, testEMFPlusComment(
				testEMFPlusHeader(0),
				testEMFPlusSimplePenObject(4, 1, 0xFF000000),
				pathObj,
				testEMFPlusRecord(emfPlusSetClipRect, 0<<8, r1),
				testEMFPlusRecord(emfPlusSetClipRect, mode<<8, r2),
				testEMFPlusRecord(emfPlusDrawPath, 3, draw),
			))
			svg, _, _, ok := emfToSVG(emf)
			if !ok {
				t.Fatal("EMF conversion failed")
			}
			out := string(svg)
			if !strings.Contains(out, `mask="url(#emfplus-mask-3)"`) {
				t.Fatalf("combined clip mask missing: %s", out)
			}
		})
	}
}

func testEMFPlusFontObject(id uint8, family string, size float32, style uint32) []byte {
	r := []rune(family)
	data := make([]byte, 24+len(r)*2)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], math.Float32bits(size))
	binary.LittleEndian.PutUint32(data[8:12], 2) // UnitPixel
	binary.LittleEndian.PutUint32(data[12:16], style)
	binary.LittleEndian.PutUint32(data[20:24], uint32(len(r)))
	for i, ch := range r {
		binary.LittleEndian.PutUint16(data[24+i*2:26+i*2], uint16(ch))
	}
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectFont)<<8|uint16(id), data)
}

func testEMFPlusStringFormatObject(id uint8, flags, align, lineAlign uint32, tracking float32) []byte {
	data := make([]byte, 60)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], flags)
	binary.LittleEndian.PutUint32(data[12:16], align)
	binary.LittleEndian.PutUint32(data[16:20], lineAlign)
	binary.LittleEndian.PutUint32(data[44:48], math.Float32bits(tracking))
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectStringFormat)<<8|uint16(id), data)
}

func testUTF16LE(s string) []byte {
	r := []rune(s)
	out := make([]byte, len(r)*2)
	for i, ch := range r {
		binary.LittleEndian.PutUint16(out[i*2:i*2+2], uint16(ch))
	}
	return out
}

func testEMFPlusDrawString(fontID uint8, formatID uint32, brushColor uint32, text string, rect [4]float32) []byte {
	str := testUTF16LE(text)
	data := make([]byte, 28+len(str))
	binary.LittleEndian.PutUint32(data[0:4], brushColor)
	binary.LittleEndian.PutUint32(data[4:8], formatID)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len([]rune(text))))
	for i, v := range rect {
		binary.LittleEndian.PutUint32(data[12+i*4:16+i*4], math.Float32bits(v))
	}
	copy(data[28:], str)
	return testEMFPlusRecord(emfPlusDrawString, 0x8000|uint16(fontID), data)
}

func TestEMFPlusDrawStringUsesFontStringFormatAndLayoutClip(t *testing.T) {
	emf := testEMFWithRecords(160, 100, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "Arial", 20, 0x1|0x2|0x4),
		testEMFPlusStringFormatObject(3, 0, 1, 1, 1.15),
		testEMFPlusDrawString(2, 3, 0xFF336699, "Hello", [4]float32{10, 20, 100, 50}),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	for _, want := range []string{`font-family="Arial"`, `font-size="20.000"`, `font-weight="bold"`, `font-style="italic"`, `text-decoration="underline"`, `text-anchor="middle"`, `fill="#336699"`, `letter-spacing="3.000"`, `mask="url(#emfplus-mask-`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestEMFPlusDrawStringHonorsRTLAndNoClip(t *testing.T) {
	emf := testEMFWithRecords(160, 100, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "Tahoma", 14, 0),
		testEMFPlusStringFormatObject(3, 0x00004001, 2, 0, 1),
		testEMFPlusDrawString(2, 3, 0xFF000000, "abc", [4]float32{10, 10, 80, 30}),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `direction="rtl"`) || !strings.Contains(out, `text-anchor="end"`) {
		t.Fatalf("RTL/far alignment missing: %s", out)
	}
	if strings.Contains(out, `mask="url(#emfplus-mask-`) {
		t.Fatalf("NoClip unexpectedly emitted layout mask: %s", out)
	}
}

func testEMFPlusDrawDriverString(fontID uint8, color uint32, text string, pts [][2]float32, cmap bool) []byte {
	glyphs := testUTF16LE(text)
	count := len([]rune(text))
	data := make([]byte, 16+len(glyphs)+len(pts)*8)
	binary.LittleEndian.PutUint32(data[0:4], color)
	if cmap {
		binary.LittleEndian.PutUint32(data[4:8], 1)
	}
	binary.LittleEndian.PutUint32(data[12:16], uint32(count))
	copy(data[16:], glyphs)
	off := 16 + len(glyphs)
	for i, pt := range pts {
		binary.LittleEndian.PutUint32(data[off+i*8:off+i*8+4], math.Float32bits(pt[0]))
		binary.LittleEndian.PutUint32(data[off+i*8+4:off+i*8+8], math.Float32bits(pt[1]))
	}
	return testEMFPlusRecord(emfPlusDrawDriverString, 0x8000|uint16(fontID), data)
}

func TestEMFPlusDrawDriverStringUsesAuthoredUnicodeGlyphPositions(t *testing.T) {
	emf := testEMFWithRecords(160, 100, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "Arial", 16, 0),
		testEMFPlusDrawDriverString(2, 0xFF000000, "AB", [][2]float32{{10, 20}, {35, 25}}, true),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `x="10.000" y="20.000"`) || !strings.Contains(out, `>A</text>`) || !strings.Contains(out, `x="35.000" y="25.000"`) || !strings.Contains(out, `>B</text>`) {
		t.Fatalf("driver-string positions missing: %s", out)
	}
}

func TestEMFPlusDrawDriverStringDoesNotTreatGlyphIndicesAsUnicode(t *testing.T) {
	emf := testEMFWithRecords(160, 100, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "Arial", 16, 0),
		testEMFPlusDrawDriverString(2, 0xFF000000, "AB", [][2]float32{{10, 20}, {35, 25}}, false),
	))
	svg, _, _, ok := emfToSVG(emf)
	if ok && strings.Contains(string(svg), `>A</text>`) {
		t.Fatalf("raw glyph indices were incorrectly rendered as Unicode: %s", svg)
	}
}

func testEMFPlusRichStringFormatObject(id uint8, flags, align, lineAlign, hotkey, trimming uint32, firstTab float32, tabs []float32) []byte {
	data := make([]byte, 60+len(tabs)*4)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], flags)
	binary.LittleEndian.PutUint32(data[12:16], align)
	binary.LittleEndian.PutUint32(data[16:20], lineAlign)
	binary.LittleEndian.PutUint32(data[28:32], math.Float32bits(firstTab))
	binary.LittleEndian.PutUint32(data[32:36], hotkey)
	binary.LittleEndian.PutUint32(data[44:48], math.Float32bits(1))
	binary.LittleEndian.PutUint32(data[48:52], trimming)
	binary.LittleEndian.PutUint32(data[52:56], uint32(len(tabs)))
	for i, v := range tabs {
		binary.LittleEndian.PutUint32(data[60+i*4:64+i*4], math.Float32bits(v))
	}
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectStringFormat)<<8|uint16(id), data)
}

func TestEMFPlusDrawStringHandlesHotkeyTabsWrappingAndTrimming(t *testing.T) {
	emf := testEMFWithRecords(200, 120, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "Arial", 10, 0),
		testEMFPlusRichStringFormatObject(3, 0, 0, 0, 1, 3, 12, []float32{24}),
		testEMFPlusDrawString(2, 3, 0xFF000000, "&File\tA very long value", [4]float32{10, 10, 70, 40}),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	out := string(svg)
	if strings.Contains(out, "&amp;File") || !strings.Contains(out, `text-decoration="underline"`) {
		t.Fatalf("hotkey prefix was not materialized: %s", out)
	}
	if !strings.Contains(out, "ile   A") || strings.Count(out, "<tspan") < 4 {
		t.Fatalf("tabs/wrapping were not materialized: %s", out)
	}
}

func TestEMFPlusDrawStringTrimsNoWrapLine(t *testing.T) {
	emf := testEMFWithRecords(160, 80, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "Arial", 10, 0),
		testEMFPlusRichStringFormatObject(3, 0x1000, 0, 0, 0, 3, 0, nil),
		testEMFPlusDrawString(2, 3, 0xFF000000, "abcdefghijklmno", [4]float32{10, 10, 40, 20}),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	if !strings.Contains(string(svg), "…") {
		t.Fatalf("ellipsis trimming missing: %s", svg)
	}
}

func TestEMFPlusTextRenderingStateAndPageTransformParticipateInSaveRestore(t *testing.T) {
	page := make([]byte, 4)
	binary.LittleEndian.PutUint32(page, math.Float32bits(2))
	save := make([]byte, 4)
	binary.LittleEndian.PutUint32(save, 7)
	restore := make([]byte, 4)
	binary.LittleEndian.PutUint32(restore, 7)
	emf := testEMFWithRecords(200, 120, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "Arial", 10, 0),
		testEMFPlusRecord(emfPlusSetPageTransform, 2, page),
		testEMFPlusRecord(emfPlusSetTextRenderingHint, 5, nil),
		testEMFPlusRecord(emfPlusSetTextContrast, 1600, nil),
		testEMFPlusRecord(emfPlusSave, 0, save),
		testEMFPlusRecord(emfPlusSetTextRenderingHint, 2, nil),
		testEMFPlusRecord(emfPlusRestore, 0, restore),
		testEMFPlusDrawString(2, 0xffffffff, 0xFF000000, "Hi", [4]float32{5, 5, 40, 20}),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `matrix(2.000000`) || !strings.Contains(out, `text-rendering="optimizeLegibility"`) || !strings.Contains(out, `data-emfplus-text-contrast="1600"`) {
		t.Fatalf("page/text rendering state missing after restore: %s", out)
	}
}

func testSFNTForAB(t *testing.T) []byte {
	t.Helper()
	return testSFNTForABMetrics(t, 600, 700, 0)
}

func testSFNTForABMetrics(t *testing.T, advanceA, advanceB uint16, kernAB int16) []byte {
	t.Helper()
	tables := map[string][]byte{}
	head := make([]byte, 20)
	binary.BigEndian.PutUint16(head[18:20], 1000)
	tables["head"] = head
	hhea := make([]byte, 36)
	binary.BigEndian.PutUint16(hhea[34:36], 3)
	tables["hhea"] = hhea
	maxp := make([]byte, 6)
	binary.BigEndian.PutUint32(maxp[0:4], 0x00010000)
	binary.BigEndian.PutUint16(maxp[4:6], 3)
	tables["maxp"] = maxp
	hmtx := make([]byte, 12)
	for i, aw := range []uint16{500, advanceA, advanceB} {
		binary.BigEndian.PutUint16(hmtx[i*4:i*4+2], aw)
	}
	tables["hmtx"] = hmtx
	format12 := make([]byte, 40)
	binary.BigEndian.PutUint16(format12[0:2], 12)
	binary.BigEndian.PutUint32(format12[4:8], uint32(len(format12)))
	binary.BigEndian.PutUint32(format12[12:16], 2)
	binary.BigEndian.PutUint32(format12[16:20], uint32('A'))
	binary.BigEndian.PutUint32(format12[20:24], uint32('A'))
	binary.BigEndian.PutUint32(format12[24:28], 1)
	binary.BigEndian.PutUint32(format12[28:32], uint32('B'))
	binary.BigEndian.PutUint32(format12[32:36], uint32('B'))
	binary.BigEndian.PutUint32(format12[36:40], 2)
	cmap := make([]byte, 12+len(format12))
	binary.BigEndian.PutUint16(cmap[2:4], 1)
	binary.BigEndian.PutUint16(cmap[4:6], 3)
	binary.BigEndian.PutUint16(cmap[6:8], 10)
	binary.BigEndian.PutUint32(cmap[8:12], 12)
	copy(cmap[12:], format12)
	tables["cmap"] = cmap
	if kernAB != 0 {
		kern := make([]byte, 24)
		binary.BigEndian.PutUint16(kern[2:4], 1)       // one subtable
		binary.BigEndian.PutUint16(kern[6:8], 20)      // subtable length
		binary.BigEndian.PutUint16(kern[8:10], 0x0001) // horizontal, format 0
		binary.BigEndian.PutUint16(kern[10:12], 1)     // one pair
		binary.BigEndian.PutUint16(kern[18:20], 1)     // A glyph
		binary.BigEndian.PutUint16(kern[20:22], 2)     // B glyph
		binary.BigEndian.PutUint16(kern[22:24], uint16(kernAB))
		tables["kern"] = kern
	}

	order := []string{"cmap", "head", "hhea", "hmtx", "maxp"}
	if _, ok := tables["kern"]; ok {
		order = append(order, "kern")
	}
	dirSize := 12 + len(order)*16
	off := dirSize
	align4 := func(n int) int { return (n + 3) &^ 3 }
	for _, tag := range order {
		off += align4(len(tables[tag]))
	}
	out := make([]byte, off)
	binary.BigEndian.PutUint32(out[0:4], 0x00010000)
	binary.BigEndian.PutUint16(out[4:6], uint16(len(order)))
	pos := dirSize
	for i, tag := range order {
		rec := 12 + i*16
		copy(out[rec:rec+4], tag)
		binary.BigEndian.PutUint32(out[rec+8:rec+12], uint32(pos))
		binary.BigEndian.PutUint32(out[rec+12:rec+16], uint32(len(tables[tag])))
		copy(out[pos:], tables[tag])
		pos += align4(len(tables[tag]))
	}
	return out
}

func testEMFPlusRawDriverString(fontID uint8, color uint32, glyphs []uint16, origin [2]float32, realizedAdvance bool) []byte {
	data := make([]byte, 16+len(glyphs)*2+8)
	binary.LittleEndian.PutUint32(data[0:4], color)
	if realizedAdvance {
		binary.LittleEndian.PutUint32(data[4:8], 0x4)
	}
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(glyphs)))
	for i, gid := range glyphs {
		binary.LittleEndian.PutUint16(data[16+i*2:18+i*2], gid)
	}
	off := 16 + len(glyphs)*2
	binary.LittleEndian.PutUint32(data[off:off+4], math.Float32bits(origin[0]))
	binary.LittleEndian.PutUint32(data[off+4:off+8], math.Float32bits(origin[1]))
	return testEMFPlusRecord(emfPlusDrawDriverString, 0x8000|uint16(fontID), data)
}

func TestEMFPlusRawGlyphDriverStringUsesSuppliedSFNTCmapAndMetrics(t *testing.T) {
	emf := testEMFWithRecords(160, 100, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "TestFace", 20, 0),
		testEMFPlusRawDriverString(2, 0xFF000000, []uint16{1, 2}, [2]float32{10, 30}, true),
	))
	svg, _, _, ok := ToSVGWithOptions(emf, "image/emf", Options{Fonts: []FontFace{{Family: "TestFace", Data: testSFNTForAB(t)}}})
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `x="10.000" y="30.000"`) || !strings.Contains(out, `>A</text>`) {
		t.Fatalf("glyph 1 was not cmap-realized: %s", out)
	}
	// Glyph 1 advance is 600/1000 * 20 = 12 user units.
	if !strings.Contains(out, `x="22.000" y="30.000"`) || !strings.Contains(out, `>B</text>`) {
		t.Fatalf("glyph metrics were not used for realized advance: %s", out)
	}
}

func TestEMFPlusRawGlyphDriverStringStillRefusesWithoutFontData(t *testing.T) {
	emf := testEMFWithRecords(160, 100, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "MissingFace", 20, 0),
		testEMFPlusRawDriverString(2, 0xFF000000, []uint16{1, 2}, [2]float32{10, 30}, true),
	))
	svg, _, _, ok := emfToSVG(emf)
	if ok && (strings.Contains(string(svg), `>A</text>`) || strings.Contains(string(svg), `>B</text>`)) {
		t.Fatalf("raw glyphs were rendered without real font data: %s", svg)
	}
}

func testEMFPlusStringFormatWithRanges(id uint8, ranges []emfPlusCharacterRange) []byte {
	data := make([]byte, 60+len(ranges)*8)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[44:48], math.Float32bits(1))
	binary.LittleEndian.PutUint32(data[56:60], uint32(len(ranges)))
	for i, r := range ranges {
		off := 60 + i*8
		binary.LittleEndian.PutUint32(data[off:off+4], uint32(r.first))
		binary.LittleEndian.PutUint32(data[off+4:off+8], uint32(r.length))
	}
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectStringFormat)<<8|uint16(id), data)
}

func TestEMFPlusStringFormatPreservesCharacterRanges(t *testing.T) {
	emf := testEMFWithRecords(160, 80, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "Arial", 12, 0),
		testEMFPlusStringFormatWithRanges(3, []emfPlusCharacterRange{{first: 1, length: 3}, {first: 6, length: 2}}),
		testEMFPlusDrawString(2, 3, 0xFF000000, "abcdefgh", [4]float32{5, 5, 120, 25}),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	if !strings.Contains(string(svg), `data-emfplus-character-ranges="1:3,6:2"`) {
		t.Fatalf("character ranges were not preserved: %s", svg)
	}
}

func TestEMFPlusCharacterRangeRegionsUseSuppliedFontMetrics(t *testing.T) {
	emf := testEMFWithRecords(160, 80, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "TestFace", 20, 0),
		testEMFPlusStringFormatWithRanges(3, []emfPlusCharacterRange{{first: 0, length: 1}, {first: 1, length: 1}}),
		testEMFPlusDrawString(2, 3, 0xFF000000, "AB", [4]float32{10, 10, 100, 30}),
	))
	svg, _, _, ok := ToSVGWithOptions(emf, "image/emf", Options{Fonts: []FontFace{{Family: "TestFace", Data: testSFNTForAB(t)}}})
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `data-emfplus-character-range="0:1"`) || !strings.Contains(out, `width="12.000"`) {
		t.Fatalf("first character range did not use hmtx width: %s", out)
	}
	if !strings.Contains(out, `data-emfplus-character-range="1:1"`) || !strings.Contains(out, `x="22.000"`) {
		t.Fatalf("second character range did not start after measured first glyph: %s", out)
	}
}

func TestEMFPlusRealizedFontPrefersExactStyleWithinFamily(t *testing.T) {
	emf := testEMFWithRecords(160, 100, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "TestFace", 20, 1), // bold
		testEMFPlusRawDriverString(2, 0xFF000000, []uint16{1, 2}, [2]float32{10, 30}, true),
	))
	svg, _, _, ok := ToSVGWithOptions(emf, "image/emf", Options{Fonts: []FontFace{
		{Family: "TestFace", Data: testSFNTForABMetrics(t, 600, 700, 0)},
		{Family: "TestFace", Bold: true, Data: testSFNTForABMetrics(t, 800, 700, 0)},
	}})
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	out := string(svg)
	// Bold A advance is 800/1000 * 20 = 16, so B starts at 26.
	if !strings.Contains(out, `x="26.000" y="30.000"`) {
		t.Fatalf("bold face was not selected exactly: %s", out)
	}
}

func TestEMFPlusLegacyKernPairsAffectRealizedAdvance(t *testing.T) {
	emf := testEMFWithRecords(160, 100, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "TestFace", 20, 0),
		testEMFPlusRawDriverString(2, 0xFF000000, []uint16{1, 2}, [2]float32{10, 30}, true),
	))
	// A advance = 12, AB kern = -2 at 20 user units, so B starts at 20.
	svg, _, _, ok := ToSVGWithOptions(emf, "image/emf", Options{Fonts: []FontFace{{Family: "TestFace", Data: testSFNTForABMetrics(t, 600, 700, -100)}}})
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	if !strings.Contains(string(svg), `x="20.000" y="30.000"`) {
		t.Fatalf("kern pair did not affect driver-string advance: %s", svg)
	}
}

func TestEMFPlusCharacterRangeCrossingLinesProducesMeasuredRegionsPerLine(t *testing.T) {
	emf := testEMFWithRecords(180, 120, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusFontObject(2, "TestFace", 20, 0),
		testEMFPlusStringFormatWithRanges(3, []emfPlusCharacterRange{{first: 1, length: 3}}),
		testEMFPlusDrawString(2, 3, 0xFF000000, "AB\nAB", [4]float32{10, 10, 120, 70}),
	))
	svg, _, _, ok := ToSVGWithOptions(emf, "image/emf", Options{Fonts: []FontFace{{Family: "TestFace", Data: testSFNTForAB(t)}}})
	if !ok {
		t.Fatal("EMF+ conversion failed")
	}
	out := string(svg)
	if strings.Count(out, `data-emfplus-character-range="1:3"`) != 2 {
		t.Fatalf("cross-line range did not produce two measured regions: %s", out)
	}
	if !strings.Contains(out, `data-emfplus-character-range-line="0"`) || !strings.Contains(out, `data-emfplus-character-range-line="1"`) {
		t.Fatalf("range line ownership missing: %s", out)
	}
	if !strings.Contains(out, `width="14.000"`) || !strings.Contains(out, `width="12.000"`) {
		t.Fatalf("line range widths did not use real font metrics: %s", out)
	}
}

func TestEMFPlusCardinalSplineDrawingRecords(t *testing.T) {
	pen := testEMFPlusSimplePenObject(3, 2, 0xFF112233)
	pts := [][2]float32{{10, 10}, {30, 5}, {50, 30}, {70, 10}}
	pointBytes := func(ps [][2]float32) []byte {
		b := make([]byte, len(ps)*8)
		for i, p := range ps {
			binary.LittleEndian.PutUint32(b[i*8:i*8+4], math.Float32bits(p[0]))
			binary.LittleEndian.PutUint32(b[i*8+4:i*8+8], math.Float32bits(p[1]))
		}
		return b
	}
	closed := make([]byte, 8+len(pts)*8)
	binary.LittleEndian.PutUint32(closed[0:4], math.Float32bits(.5))
	binary.LittleEndian.PutUint32(closed[4:8], uint32(len(pts)))
	copy(closed[8:], pointBytes(pts))
	curve := make([]byte, 16+len(pts)*8)
	binary.LittleEndian.PutUint32(curve[0:4], math.Float32bits(.5))
	binary.LittleEndian.PutUint32(curve[4:8], 0)
	binary.LittleEndian.PutUint32(curve[8:12], 3)
	binary.LittleEndian.PutUint32(curve[12:16], uint32(len(pts)))
	copy(curve[16:], pointBytes(pts))
	fill := make([]byte, 12+len(pts)*8)
	binary.LittleEndian.PutUint32(fill[0:4], 0xFF00AA00)
	binary.LittleEndian.PutUint32(fill[4:8], math.Float32bits(.5))
	binary.LittleEndian.PutUint32(fill[8:12], uint32(len(pts)))
	copy(fill[12:], pointBytes(pts))
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), pen, testEMFPlusRecord(emfPlusDrawClosedCurve, 3, closed), testEMFPlusRecord(emfPlusDrawCurve, 3, curve), testEMFPlusRecord(emfPlusFillClosedCurve, 0x8000, fill)))
	svg, _, _, ok := ToSVG(emf, "image/x-emf")
	if !ok {
		t.Fatal("EMF+ cardinal splines did not convert")
	}
	out := string(svg)
	if strings.Count(out, " C ") < 3 || !strings.Contains(out, "#00AA00") || !strings.Contains(out, "#112233") {
		t.Fatalf("cardinal spline playback incomplete: %s", out)
	}
}

func testEMFPlusRelativePathObject(id uint8) []byte {
	// Absolute points after relative decoding: (10,10), (20,10), (20,20), (10,20).
	points := []byte{10, 10, 10, 0, 0, 10, 0x76, 0} // -10 is signed Integer7 0x76.
	types := []byte{0x41, 0x00, 0x42, 0x01, 0x41, 0x81}
	data := make([]byte, 12+len(points)+len(types))
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 4)
	binary.LittleEndian.PutUint32(data[8:12], 0x0800) // R: PointR + RLE point types.
	copy(data[12:], points)
	copy(data[12+len(points):], types)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectPath)<<8|uint16(id), data)
}

func TestEMFPlusRelativeRLEPathIsDecoded(t *testing.T) {
	fill := make([]byte, 4)
	binary.LittleEndian.PutUint32(fill, 1)
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusSolidBrushObject(1, 0xFF336699),
		testEMFPlusRelativePathObject(2),
		testEMFPlusRecord(emfPlusFillPath, 2, fill),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `d="M 10.000 10.000 L 20.000 10.000 L 20.000 20.000 L 10.000 20.000 Z"`) {
		t.Fatalf("relative/RLE path was not decoded: %s", out)
	}
}

func testEMFPlusAdvancedPenObject(id uint8) []byte {
	const flags = emfPlusPenDataStartCap | emfPlusPenDataEndCap | emfPlusPenDataJoin | emfPlusPenDataMiterLimit | emfPlusPenDataLineStyle | emfPlusPenDataDashedLineOff | emfPlusPenDataDashedLine
	data := make([]byte, 20+4+4+4+4+4+4+4+8+12)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[4:8], 0)
	binary.LittleEndian.PutUint32(data[8:12], flags)
	binary.LittleEndian.PutUint32(data[12:16], 2) // UnitPixel
	binary.LittleEndian.PutUint32(data[16:20], math.Float32bits(2))
	off := 20
	putI := func(v uint32) { binary.LittleEndian.PutUint32(data[off:off+4], v); off += 4 }
	putF := func(v float32) { binary.LittleEndian.PutUint32(data[off:off+4], math.Float32bits(v)); off += 4 }
	putI(2)    // round start cap
	putI(0x14) // arrow anchor end cap
	putI(1)    // bevel join
	putF(4)    // miter limit
	putI(5)    // custom line style; dash data below owns actual pattern
	putF(0.5)  // dash offset in pen-width units
	putI(2)    // dash count
	putF(2)
	putF(1)
	binary.LittleEndian.PutUint32(data[off:off+4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[off+4:off+8], 0)
	binary.LittleEndian.PutUint32(data[off+8:off+12], 0xFF112233)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectPen)<<8|uint16(id), data)
}

func TestEMFPlusAdvancedPenDataDrivesSVGStrokeSemantics(t *testing.T) {
	path := testEMFPlusPathObject(2, [][2]float32{{5, 5}, {40, 5}}, []byte{0, 1})
	draw := make([]byte, 4)
	binary.LittleEndian.PutUint32(draw, 3)
	emf := testEMFWithRecords(60, 30, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusAdvancedPenObject(3), path, testEMFPlusRecord(emfPlusDrawPath, 2, draw)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	for _, want := range []string{`stroke-linecap="round"`, `stroke-linejoin="bevel"`, `stroke-miterlimit="4.000"`, `stroke-dasharray="4.000 2.000"`, `stroke-dashoffset="1.000"`, `marker-end="url(#emfplus-pen-cap-`} {
		if !strings.Contains(out, want) {
			t.Fatalf("advanced pen attribute %q missing: %s", want, out)
		}
	}
}

func TestEMFPlusDrawImagePointsAcceptsRelativePoints(t *testing.T) {
	data := make([]byte, 34)
	binary.LittleEndian.PutUint32(data[4:8], 2) // SrcUnitPixel
	for i, v := range []float32{0, 0, 2, 2} {
		binary.LittleEndian.PutUint32(data[8+i*4:12+i*4], math.Float32bits(v))
	}
	binary.LittleEndian.PutUint32(data[24:28], 3)
	copy(data[28:], []byte{10, 10, 10, 0, 0x76, 20}) // (10,10),(20,10),(10,30)
	emf := testEMFWithRecords(100, 60, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusCompressedImageObject(4, testTinyPNG(t)), testEMFPlusRecord(emfPlusDrawImagePoints, 0x0800|4, data)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	if !strings.Contains(string(svg), `matrix(5.000000 0.000000 0.000000 10.000000 10.000000 10.000000)`) {
		t.Fatalf("relative image-point transform missing: %s", svg)
	}
}

func testEMFPlusBlurEffect(radius float32) []byte {
	data := make([]byte, 28)
	copy(data[:16], []byte{0xA4, 0x80, 0x3C, 0x63, 0x43, 0x18, 0x2B, 0x48, 0x9E, 0xF2, 0xBE, 0x28, 0x34, 0xC5, 0xFD, 0xD4})
	binary.LittleEndian.PutUint32(data[16:20], 8)
	binary.LittleEndian.PutUint32(data[20:24], math.Float32bits(radius))
	binary.LittleEndian.PutUint32(data[24:28], 0)
	return testEMFPlusRecord(emfPlusSerializableObject, 0, data)
}

func TestEMFPlusSerializableBlurEffectAppliesToDrawImagePoints(t *testing.T) {
	data := make([]byte, 52)
	binary.LittleEndian.PutUint32(data[4:8], 2)
	for i, v := range []float32{0, 0, 2, 2} {
		binary.LittleEndian.PutUint32(data[8+i*4:12+i*4], math.Float32bits(v))
	}
	binary.LittleEndian.PutUint32(data[24:28], 3)
	for i, v := range []float32{0, 0, 20, 0, 0, 20} {
		binary.LittleEndian.PutUint32(data[28+i*4:32+i*4], math.Float32bits(v))
	}
	emf := testEMFWithRecords(40, 40, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusCompressedImageObject(4, testTinyPNG(t)), testEMFPlusBlurEffect(4), testEMFPlusRecord(emfPlusDrawImagePoints, 0x2000|4, data)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<feGaussianBlur stdDeviation="2.000"`) || !strings.Contains(out, `filter="url(#emfplus-effect-`) {
		t.Fatalf("serializable blur effect missing: %s", out)
	}
}

func TestEMFPlusGraphicsQualityStateIsSavedAndRestored(t *testing.T) {
	origin := make([]byte, 8)
	binary.LittleEndian.PutUint32(origin[0:4], 7)
	binary.LittleEndian.PutUint32(origin[4:8], 9)
	path := testEMFPlusPathObject(2, [][2]float32{{0, 0}, {10, 0}}, []byte{0, 1})
	draw := make([]byte, 4)
	binary.LittleEndian.PutUint32(draw, 3)
	emf := testEMFWithRecords(30, 20, testEMFPlusComment(
		testEMFPlusHeader(0), testEMFPlusSimplePenObject(3, 1, 0xFF000000), path,
		testEMFPlusRecord(emfPlusSetRenderingOrigin, 0, origin),
		testEMFPlusRecord(emfPlusSetAntiAliasMode, 1, nil),
		testEMFPlusSaveRecord(emfPlusSave, 11),
		testEMFPlusRecord(emfPlusSetAntiAliasMode, 0, nil),
		testEMFPlusSaveRecord(emfPlusRestore, 11),
		testEMFPlusRecord(emfPlusDrawPath, 2, draw),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `data-emfplus-antialias-mode="1"`) || !strings.Contains(out, `data-emfplus-rendering-origin="7,9"`) {
		t.Fatalf("graphics quality state was not restored: %s", out)
	}
}

func testEMFPlusColorMatrixEffect(values [25]float32) []byte {
	data := make([]byte, 20+25*4)
	copy(data[:16], []byte{0x15, 0x26, 0x8F, 0x71, 0x33, 0x79, 0xE3, 0x40, 0xA5, 0x11, 0x5F, 0x68, 0xFE, 0x14, 0xDD, 0x74})
	binary.LittleEndian.PutUint32(data[16:20], 25*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(data[20+i*4:24+i*4], math.Float32bits(v))
	}
	return testEMFPlusRecord(emfPlusSerializableObject, 0, data)
}

func TestEMFPlusSerializableColorMatrixEffectAppliesToImage(t *testing.T) {
	var matrix [25]float32
	matrix[0], matrix[6], matrix[12], matrix[18], matrix[24] = 1, 1, 1, 1, 1
	matrix[20] = 0.25 // red translation in GDI+ row-vector layout.
	data := make([]byte, 40)
	binary.LittleEndian.PutUint32(data[4:8], 2) // SrcUnitPixel
	for i, v := range []float32{0, 0, 2, 2, 0, 0, 20, 20} {
		binary.LittleEndian.PutUint32(data[8+i*4:12+i*4], math.Float32bits(v))
	}
	emf := testEMFWithRecords(40, 40, testEMFPlusComment(
		testEMFPlusHeader(0),
		testEMFPlusCompressedImageObject(4, testTinyPNG(t)),
		testEMFPlusColorMatrixEffect(matrix),
		testEMFPlusRecord(emfPlusDrawImage, 0x2000|4, data),
	))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	if !strings.Contains(out, `<feColorMatrix type="matrix"`) || !strings.Contains(out, `1.000000 0.000000 0.000000 0.000000 0.250000`) || !strings.Contains(out, `filter="url(#emfplus-effect-`) {
		t.Fatalf("serializable color-matrix effect missing: %s", out)
	}
}

func testEMFPlusCompoundCustomCapPenObject(id uint8) []byte {
	const flags = emfPlusPenDataCompoundLine | emfPlusPenDataCustomEndCap
	cap := make([]byte, 8+52)
	binary.LittleEndian.PutUint32(cap[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(cap[4:8], 1)
	off := 8
	putF := func(v float32) { binary.LittleEndian.PutUint32(cap[off:off+4], math.Float32bits(v)); off += 4 }
	putI := func(v uint32) { binary.LittleEndian.PutUint32(cap[off:off+4], v); off += 4 }
	putF(3)
	putF(4)
	putF(0)
	putI(1)
	putI(0)
	putI(0)
	putI(2)
	putF(10)
	putF(1)
	putF(0)
	putF(0)
	putF(0)
	putF(0)
	data := make([]byte, 20+4+4*4+4+len(cap)+12)
	binary.LittleEndian.PutUint32(data[0:4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[8:12], flags)
	binary.LittleEndian.PutUint32(data[12:16], 2)
	binary.LittleEndian.PutUint32(data[16:20], math.Float32bits(2))
	off = 20
	binary.LittleEndian.PutUint32(data[off:off+4], 4)
	off += 4
	for _, v := range []float32{0, .25, .75, 1} {
		binary.LittleEndian.PutUint32(data[off:off+4], math.Float32bits(v))
		off += 4
	}
	binary.LittleEndian.PutUint32(data[off:off+4], uint32(len(cap)))
	off += 4
	copy(data[off:off+len(cap)], cap)
	off += len(cap)
	binary.LittleEndian.PutUint32(data[off:off+4], 0xDBC01002)
	binary.LittleEndian.PutUint32(data[off+4:off+8], 0)
	binary.LittleEndian.PutUint32(data[off+8:off+12], 0xFF224466)
	return testEMFPlusRecord(emfPlusObject, uint16(emfPlusObjectPen)<<8|uint16(id), data)
}

func TestEMFPlusCompoundPenAndAdjustableArrowCapAreOwned(t *testing.T) {
	path := testEMFPlusPathObject(2, [][2]float32{{5, 10}, {45, 10}}, []byte{0, 1})
	draw := make([]byte, 4)
	binary.LittleEndian.PutUint32(draw, 3)
	emf := testEMFWithRecords(60, 30, testEMFPlusComment(testEMFPlusHeader(0), testEMFPlusCompoundCustomCapPenObject(3), path, testEMFPlusRecord(emfPlusDrawPath, 2, draw)))
	svg, _, _, ok := emfToSVG(emf)
	if !ok {
		t.Fatal("EMF conversion failed")
	}
	out := string(svg)
	for _, want := range []string{`data-emfplus-compound-line="0.000000,0.250000,0.750000,1.000000"`, `marker-end="url(#emfplus-custom-cap-`, `M 0 -2.000 L 3.000 0 L 0 2.000 Z`} {
		if !strings.Contains(out, want) {
			t.Fatalf("compound/custom-cap output %q missing: %s", want, out)
		}
	}
}

func testEMFPlusEffectData(guid string, payload []byte) []byte {
	var d1 uint32
	var d2, d3 uint16
	var b [8]uint8
	if _, err := fmt.Sscanf(guid, "%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x", &d1, &d2, &d3, &b[0], &b[1], &b[2], &b[3], &b[4], &b[5], &b[6], &b[7]); err != nil {
		panic(err)
	}
	data := make([]byte, 20+len(payload))
	binary.LittleEndian.PutUint32(data[0:4], d1)
	binary.LittleEndian.PutUint16(data[4:6], d2)
	binary.LittleEndian.PutUint16(data[6:8], d3)
	copy(data[8:16], b[:])
	binary.LittleEndian.PutUint32(data[16:20], uint32(len(payload)))
	copy(data[20:], payload)
	return data
}

func TestEMFPlusAdditionalSerializableEffectsProduceOwnedFilters(t *testing.T) {
	mkints := func(vals ...int32) []byte {
		b := make([]byte, 4*len(vals))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[i*4:i*4+4], uint32(v))
		}
		return b
	}
	mkfloats := func(vals ...float32) []byte {
		b := make([]byte, 4*len(vals))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[i*4:i*4+4], math.Float32bits(v))
		}
		return b
	}
	cases := []struct {
		name, guid string
		payload    []byte
		want       string
	}{
		{"brightness-contrast", "d3a1dbe1-8ec4-4c17-9f4c-ea97ad1c343d", mkints(32, 25), `<feComponentTransfer>`},
		{"color-balance", "537e597d-251e-48da-9664-29ca496b70f8", mkints(10, -20, 30), `intercept="0.050000"`},
		{"hsl", "8b2dd6c3-eb07-4d87-a5f0-7108e26a9c5f", mkints(45, 20, 10), `type="hueRotate" values="45.000"`},
		{"levels", "99c354ec-2a31-4f3a-8c34-17a803b33a25", mkints(90, 20, 10), `type="gamma"`},
		{"sharpen", "63cbf3ee-c526-402c-8f71-62c540bf5142", mkfloats(2, 40), `<feConvolveMatrix`},
		{"tint", "1077af00-2848-4441-9489-44ad4c2d7a2c", mkints(30, 25), `type="saturate"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := emfPlusPlayback{}
			p.serializableObject(emfPlusRecord{data: testEMFPlusEffectData(tc.guid, tc.payload)})
			if p.effectFilter == "" || !strings.Contains(p.defs.String(), tc.want) {
				t.Fatalf("effect %s not materialized: filter=%q defs=%s", tc.name, p.effectFilter, p.defs.String())
			}
		})
	}
}
