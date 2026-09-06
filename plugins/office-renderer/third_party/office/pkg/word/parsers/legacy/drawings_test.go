package legacy

import (
	"encoding/binary"
	"math"
	"os"
	"strings"
	"testing"

	wordir "office/pkg/word/ir"
	"office/pkg/word/render"
)

func TestParseDocCuratedPngPictureFixtureRendersLegacyDrawingImage(t *testing.T) {
	path := "../../../../testdata/doc/apache-PngPicture.doc"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("optional legacy PNG fixture is not available in this checkout")
		}
		t.Fatalf("stat fixture: %v", err)
	}
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
