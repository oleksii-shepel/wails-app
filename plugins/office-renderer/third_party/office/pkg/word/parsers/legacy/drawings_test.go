package legacy

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	wordir "office/pkg/word/ir"
	"office/pkg/word/render"
)

func TestParseDocCuratedPngPictureFixtureRendersLegacyDrawingImage(t *testing.T) {
	path := "../../../../testdata/doc/apache-PngPicture.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}

	foundPNG := false
	floatingCount := 0
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Kind != "image" || run.Image == nil {
				continue
			}
			if run.Image.MIMEType != "image/png" {
				continue
			}
			if len(run.Image.Data) == 0 {
				t.Fatal("expected legacy drawing image payload")
			}
			if run.Image.Width == 0 || run.Image.Height == 0 {
				t.Fatalf("image size = (%d,%d), want non-zero", run.Image.Width, run.Image.Height)
			}
			if run.Image.Floating {
				floatingCount++
				if run.Image.WrapType != "square" {
					t.Fatalf("floating image wrap = %q, want %q", run.Image.WrapType, "square")
				}
				if run.Image.BehindDoc {
					t.Fatalf("floating image BehindDoc = %v, want false", run.Image.BehindDoc)
				}
				if run.Image.XRelative != "page" || run.Image.YRelative != "page" {
					t.Fatalf("floating image relatives = (%q,%q), want (page,page)", run.Image.XRelative, run.Image.YRelative)
				}
				if run.Image.X <= 0 {
					t.Fatalf("floating image X = %d, want positive page-relative offset", run.Image.X)
				}
			}
			foundPNG = true
		}
	}
	if !foundPNG {
		t.Fatalf("expected extracted png drawing image in paragraphs: %#v", doc.Paragraphs)
	}
	if floatingCount != 1 {
		t.Fatalf("floating png image count = %d, want 1", floatingCount)
	}
	for _, paragraph := range doc.Paragraphs {
		if strings.ContainsRune(paragraph.Text, '\b') {
			t.Fatalf("paragraph %q still contains unresolved drawing anchor", paragraph.Text)
		}
	}

	html, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML(%s): %v", path, err)
	}
	if !strings.Contains(html, "The quick brown fox jumps over the lazy dog") {
		t.Fatalf("HTML(%s): expected fixture body text in output", path)
	}
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Fatalf("HTML(%s): expected rendered png drawing image in output", path)
	}
}

func TestParseDocLegacyDrawingShapeWithoutImagesCreatesShapeRun(t *testing.T) {
	const (
		cbRgFcLcb       = 31
		plcfSpaFibIndex = 30
	)
	plcfSpa := buildLegacySingleAnchorPlcfSpa(1, 2, 0x11223344, 100, 200, 400, 800, 0x0050)
	officeArt := legacyTestOfficeArtShapeSpContainer(5, 0x11223344, 0x80, 30*65536, 0x00FF0000, 0x000000FF, 25400)
	wordStream := make([]byte, cbRgFcLcb*8+len(officeArt))
	copy(wordStream[cbRgFcLcb*8:], officeArt)
	binary.LittleEndian.PutUint32(wordStream[plcfSpaFibIndex*8:], 0)
	binary.LittleEndian.PutUint32(wordStream[plcfSpaFibIndex*8+4:], uint32(len(plcfSpa)))

	runs := []docRun{{Text: "A" + string(legacyDrawingAnchorChar), Spec: true, StartCP: 0}}
	processed := processLegacyDrawingAnchors(runs, wordStream, plcfSpa, 0, cbRgFcLcb, 2, nil)
	if len(processed) != 2 {
		t.Fatalf("processed runs = %#v, want text + shape", processed)
	}
	if processed[0].Text != "A" || processed[1].Kind != "shape" || processed[1].Shape == nil {
		t.Fatalf("processed runs = %#v, want text run then shape run", processed)
	}
	shape := processed[1].Shape
	if !shape.Floating || shape.Kind != "triangle" {
		t.Fatalf("shape = %#v, want floating triangle", shape)
	}
	if shape.Geometry == nil || shape.Geometry.Name != "triangle" {
		t.Fatalf("shape geometry = %#v, want triangle preset", shape.Geometry)
	}
	if shape.FillColor != "FF0000" || shape.StrokeColor != "0000FF" {
		t.Fatalf("shape colors = fill:%q stroke:%q, want FF0000 / 0000FF", shape.FillColor, shape.StrokeColor)
	}
	if shape.StrokeWidth != 25400 {
		t.Fatalf("shape stroke width = %d, want 25400", shape.StrokeWidth)
	}
	if shape.RotationDeg != 30 {
		t.Fatalf("shape rotation = %.2f, want 30", shape.RotationDeg)
	}
	if !shape.FlipH {
		t.Fatalf("shape FlipH = false, want true")
	}

	doc := buildLegacyDocFromRuns("shape.doc", processed)
	if strings.ContainsRune(doc.Paragraphs[0].Text, '\b') {
		t.Fatalf("paragraph text %q still contains unresolved drawing anchor", doc.Paragraphs[0].Text)
	}
	html, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML(shape.doc): %v", err)
	}
	if !strings.Contains(html, `class="shape-triangle"`) {
		if !strings.Contains(html, `class="shape__vector"`) {
			t.Fatalf("HTML(shape.doc): expected vector legacy shape in output")
		}
	}
}

func legacyTestOfficeArtShapeSpContainer(shapeType uint16, spid uint32, spFlags uint32, rotationRaw uint32, lineColorRaw uint32, fillColorRaw uint32, lineWidth uint32) []byte {
	fsp := legacyTestOfficeArtRecord((shapeType<<4)|0x2, legacyOfficeArtFSPRecord, legacyTestOfficeArtFSPPayloadWithFlags(spid, spFlags))
	fopt := legacyTestOfficeArtRecord((5<<4)|0x3, legacyOfficeArtFOPTRecord, legacyTestOfficeArtShapePropertyPayload(rotationRaw, lineColorRaw, fillColorRaw, lineWidth))
	payload := append(fsp, fopt...)
	return legacyTestOfficeArtRecord(0x000F, legacyOfficeArtSpContainer, payload)
}

func legacyTestOfficeArtFSPPayloadWithFlags(spid, flags uint32) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload[:4], spid)
	binary.LittleEndian.PutUint32(payload[4:8], flags)
	return payload
}

func legacyTestOfficeArtShapePropertyPayload(rotationRaw uint32, lineColorRaw uint32, fillColorRaw uint32, lineWidth uint32) []byte {
	payload := make([]byte, 30)
	legacyWriteOfficeArtProperty(payload[0:6], legacyOfficeArtRotationProperty, rotationRaw)
	legacyWriteOfficeArtProperty(payload[6:12], legacyOfficeArtLineColorProperty, lineColorRaw)
	legacyWriteOfficeArtProperty(payload[12:18], legacyOfficeArtFillColorProperty, fillColorRaw)
	legacyWriteOfficeArtProperty(payload[18:24], legacyOfficeArtLineWidthProperty, lineWidth)
	legacyWriteOfficeArtProperty(payload[24:30], legacyOfficeArtLineBoolsProperty, (1<<3)|(1<<19))
	return payload
}

func legacyWriteOfficeArtProperty(dst []byte, opid uint16, value uint32) {
	binary.LittleEndian.PutUint16(dst[:2], opid)
	binary.LittleEndian.PutUint32(dst[2:6], value)
}
func TestLegacyOfficeArtPicturePropertiesPreserveCropAndFilters(t *testing.T) {
	payload := make([]byte, 8*6)
	legacyWriteOfficeArtProperty(payload[0:6], legacyOfficeArtCropFromTopProperty, uint32(int32(3277)))     // ~5%
	legacyWriteOfficeArtProperty(payload[6:12], legacyOfficeArtCropFromBottomProperty, uint32(int32(5243))) // ~8%
	legacyWriteOfficeArtProperty(payload[12:18], legacyOfficeArtCropFromLeftProperty, uint32(int32(3277)))  // ~5%
	legacyWriteOfficeArtProperty(payload[18:24], legacyOfficeArtCropFromRightProperty, uint32(int32(3932))) // ~6%
	legacyWriteOfficeArtProperty(payload[24:30], legacyOfficeArtPictureBrightnessProperty, uint32(int32(8192)))
	legacyWriteOfficeArtProperty(payload[30:36], legacyOfficeArtPictureContrastProperty, 0x0000C000)
	legacyWriteOfficeArtProperty(payload[36:42], legacyOfficeArtPictureGrayProperty, 1)
	legacyWriteOfficeArtProperty(payload[42:48], legacyOfficeArtPictureBiLevelProperty, 1)

	var props legacyOfficeArtShapeProps
	legacyApplyOfficeArtPropertyTable(payload, 8, &props)

	if props.CropTop < 0.049 || props.CropTop > 0.051 || props.CropBottom < 0.079 || props.CropBottom > 0.081 {
		t.Fatalf("vertical crop = top %.4f bottom %.4f", props.CropTop, props.CropBottom)
	}
	if props.CropLeft < 0.049 || props.CropLeft > 0.051 || props.CropRight < 0.059 || props.CropRight > 0.061 {
		t.Fatalf("horizontal crop = left %.4f right %.4f", props.CropLeft, props.CropRight)
	}
	if props.Brightness != 1.25 {
		t.Fatalf("brightness = %.4f, want 1.25", props.Brightness)
	}
	if props.Contrast != 0.75 {
		t.Fatalf("contrast = %.4f, want 0.75", props.Contrast)
	}
	if !props.Grayscale || !props.BiLevel {
		t.Fatalf("gray/bilevel = %v/%v, want true/true", props.Grayscale, props.BiLevel)
	}
}

func TestLegacyDrawingAnchorReadsRotationFromTableStreamOfficeArt(t *testing.T) {
	const (
		cbRgFcLcb       = 31
		plcfSpaFibIndex = 30
		shapeID         = uint32(0x11223344)
	)
	plcfSpa := buildLegacySingleAnchorPlcfSpa(1, 2, shapeID, 100, 200, 400, 800, 0x0050)
	officeArt := legacyTestOfficeArtShapeSpContainer(75, shapeID, 0, uint32(int32(335.5*65536)), 0, 0, 0)
	tableStream := append(append([]byte(nil), plcfSpa...), officeArt...)
	wordStream := make([]byte, cbRgFcLcb*8)
	binary.LittleEndian.PutUint32(wordStream[plcfSpaFibIndex*8:], 0)
	binary.LittleEndian.PutUint32(wordStream[plcfSpaFibIndex*8+4:], uint32(len(plcfSpa)))

	images := []*wordir.Image{{MIMEType: "image/png", Data: []byte{1, 2, 3}, Width: 100, Height: 80}}
	runs := []docRun{{Text: "A" + string(legacyDrawingAnchorChar), Spec: true, StartCP: 0}}
	processed := processLegacyDrawingAnchors(runs, wordStream, tableStream, 0, cbRgFcLcb, 2, images)
	if len(processed) != 2 || processed[1].Image == nil {
		t.Fatalf("processed = %#v, want floating image", processed)
	}
	if got := processed[1].Image.RotationDeg; math.Abs(got-335.5) > 0.01 {
		t.Fatalf("rotation = %.3f, want 335.5 from table-stream FOPT", got)
	}
}

func TestLegacyOfficeArtTextFlowReadsTableStream(t *testing.T) {
	const shapeID = uint32(0x11223344)
	flows := extractLegacyDrawingTextFlowsFromStreams(
		[]byte("WordDocument has no OfficeArt"),
		legacyTestOfficeArtTextFlowSpContainer(shapeID, 0x00000002),
	)
	if got := flows[shapeID]; got != "vert270" {
		t.Fatalf("table-stream text flow = %q, want vert270", got)
	}
}

func TestLegacyOfficeArtShapeTreesReadTableStream(t *testing.T) {
	const shapeID = uint32(0x10203040)
	childAnchor := legacyOfficeArtRect{Left: 100, Top: 200, Right: 400, Bottom: 800}
	shape := &wordir.Shape{Kind: "rect", Width: 300, Height: 600}
	nextChildID := shapeID + 1
	tableStream := buildLegacyShapeOfficeArt(shapeID, shape, &childAnchor, &nextChildID)
	trees := extractLegacyDrawingShapeTreesFromStreams([]byte("WordDocument has no OfficeArt"), tableStream)
	tree, ok := trees[shapeID]
	if !ok {
		t.Fatalf("shape tree %08x not found in table stream", shapeID)
	}
	if tree.Bounds == nil || *tree.Bounds != childAnchor {
		t.Fatalf("shape tree bounds = %#v, want %#v", tree.Bounds, childAnchor)
	}
}

func TestLegacyOfficeArtPictureBooleanFlagsDecodeGrayAndBiLevel(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  uint32
		gray bool
		bi   bool
	}{
		{name: "grayscale", raw: 0x00060004, gray: true, bi: false},
		{name: "black-white", raw: 0x00060006, gray: true, bi: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := make([]byte, 6)
			legacyWriteOfficeArtProperty(payload, legacyOfficeArtPictureBoolsProperty, tc.raw)
			var props legacyOfficeArtShapeProps
			legacyApplyOfficeArtPropertyTable(payload, 1, &props)
			if props.Grayscale != tc.gray || props.BiLevel != tc.bi {
				t.Fatalf("gray/bilevel = %v/%v, want %v/%v", props.Grayscale, props.BiLevel, tc.gray, tc.bi)
			}
		})
	}
}
