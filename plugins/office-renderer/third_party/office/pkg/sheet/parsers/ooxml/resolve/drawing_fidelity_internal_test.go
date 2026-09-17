package resolve

import (
	"testing"

	"office/pkg/sheet/parsers/ooxml/parts"
)

func TestResolvePictureDrawingPreservesSourceCropAndAltText(t *testing.T) {
	pic := &parts.XDRPic{
		NvPicPr: parts.XDRNvPicPr{CNvPr: parts.XDRCNvPr{Name: "Picture 1", Descr: "Quarter-cropped photo"}},
		BlipFill: parts.XDRBlipFill{
			Blip:    parts.XDRBlip{Embed: "rId1"},
			SrcRect: &parts.XDRSrcRect{Left: 10000, Top: 20000, Right: 30000, Bottom: 40000},
		},
	}
	d := resolvePictureDrawing(pic, nil)
	if d == nil || d.Image == nil {
		t.Fatal("resolved picture/image = nil")
	}
	if got := d.Image.AltText; got != "Quarter-cropped photo" {
		t.Fatalf("alt text = %q", got)
	}
	if got := []int{d.Image.CropLeft, d.Image.CropTop, d.Image.CropRight, d.Image.CropBottom}; got[0] != 10000 || got[1] != 20000 || got[2] != 30000 || got[3] != 40000 {
		t.Fatalf("crop = %v", got)
	}
}

func TestResolvePictureDrawingClampsInvalidSourceCrop(t *testing.T) {
	pic := &parts.XDRPic{BlipFill: parts.XDRBlipFill{SrcRect: &parts.XDRSrcRect{Left: -1, Right: 120000}}}
	d := resolvePictureDrawing(pic, nil)
	if d.Image.CropLeft != 0 || d.Image.CropRight != 100000 {
		t.Fatalf("clamped crop = left %d right %d", d.Image.CropLeft, d.Image.CropRight)
	}
}

func TestResolveDrawingAnchorBehaviorPreservesEditAs(t *testing.T) {
	cases := []struct {
		editAs string
		want   DrawingAnchorBehavior
	}{
		{"twoCell", AnchorBehaviorMoveAndSize},
		{"oneCell", AnchorBehaviorMoveOnly},
		{"absolute", AnchorBehaviorFixed},
		{"", ""},
		{"unknown", ""},
	}
	for _, tc := range cases {
		if got := resolveAnchorBehavior(tc.editAs); got != tc.want {
			t.Fatalf("resolveAnchorBehavior(%q) = %q, want %q", tc.editAs, got, tc.want)
		}
	}
}

func TestResolveShapeTextPreservesParagraphsAndRunSemantics(t *testing.T) {
	sp := &parts.XDRShape{TxBody: &parts.XDRTxBody{Paragraphs: []parts.XDRTxParagraph{
		{Runs: []parts.XDRTxRun{{T: "First", Properties: &parts.RTProps{Underline: &parts.StringVal{Val: "double"}, VertAlign: &parts.StringVal{Val: "superscript"}}}}},
		{Runs: []parts.XDRTxRun{{T: "Second", Properties: &parts.RTProps{Strike: &struct{}{}}}}},
	}}}
	shape := resolveShapeWithColorMaps(sp, nil, nil, nil, nil)
	if shape.Text != "First\nSecond" {
		t.Fatalf("shape text = %q, want paragraph boundary", shape.Text)
	}
	if len(shape.RichTextRuns) != 3 {
		t.Fatalf("rich runs = %d, want 3 including paragraph boundary", len(shape.RichTextRuns))
	}
	if got := shape.RichTextRuns[0]; !got.Underline || got.UnderlineType != "double" || got.VertAlign != "superscript" {
		t.Fatalf("first run semantics = %+v", got)
	}
	if shape.RichTextRuns[1].Text != "\n" {
		t.Fatalf("paragraph separator = %q", shape.RichTextRuns[1].Text)
	}
	if !shape.RichTextRuns[2].Strike {
		t.Fatal("second paragraph strike was not preserved")
	}
}

func TestResolveChartDrawingPreservesRawPartAndAltTextWhenParserCannotModelChart(t *testing.T) {
	const relID = "rIdChart7"
	raw := []byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:extLst><c:ext uri="unsupported"/></c:extLst></c:chartSpace>`)
	gf := &parts.XDRGraphicFrame{
		NvGraphicFramePr: parts.XDRNvGraphicFramePr{CNvPr: parts.XDRCNvPr{Name: "Unsupported chart", Descr: "Quarterly revenue chart"}},
		Graphic: parts.XDRGraphic{GraphicData: parts.XDRGraphicData{
			URI:   "http://schemas.openxmlformats.org/drawingml/2006/chart",
			Chart: &parts.XDRChart{RelID: relID},
		}},
	}
	d := resolveGraphicFrameDrawing(gf, map[string][]byte{relID: raw}, nil, nil)
	if d == nil || d.Chart == nil {
		t.Fatal("resolved chart = nil")
	}
	if d.Name != "Quarterly revenue chart" || d.Chart.AltText != "Quarterly revenue chart" {
		t.Fatalf("chart identity = (%q, %q)", d.Name, d.Chart.AltText)
	}
	if d.Chart.RelID != relID {
		t.Fatalf("chart rel id = %q, want %q", d.Chart.RelID, relID)
	}
	if string(d.Chart.RawXML) != string(raw) {
		t.Fatalf("raw chart XML was not preserved: %q", d.Chart.RawXML)
	}
}

func TestResolvePictureDrawingPreservesTransform(t *testing.T) {
	pic := &parts.XDRPic{SpPr: parts.XDRSpPr{Xfrm: &parts.XDRTransform2D{Rot: 90 * 60000, FlipH: true, FlipV: true}}}
	d := resolvePictureDrawing(pic, nil)
	if d == nil || d.Image == nil {
		t.Fatal("resolved picture/image = nil")
	}
	if d.Image.RotationDeg != 90 || !d.Image.FlipH || !d.Image.FlipV {
		t.Fatalf("picture transform = rotation %.2f flipH=%v flipV=%v", d.Image.RotationDeg, d.Image.FlipH, d.Image.FlipV)
	}
}

func TestResolveDrawingsRestoresInterleavedDocumentZOrder(t *testing.T) {
	picture := func(name string) *parts.XDRPic {
		return &parts.XDRPic{NvPicPr: parts.XDRNvPicPr{CNvPr: parts.XDRCNvPr{Name: name}}}
	}
	sd := &parts.SpreadsheetDrawing{
		OneCellAnchors:  []parts.XDROneCellAnchor{{DocumentOrder: 2, Pic: picture("top"), Ext: parts.XDRExtent{CX: 9525, CY: 9525}}},
		TwoCellAnchors:  []parts.XDRTwoCellAnchor{{DocumentOrder: 0, Pic: picture("bottom"), From: parts.XDRMarker{}, To: parts.XDRMarker{Col: 1, Row: 1}}},
		AbsoluteAnchors: []parts.XDRAbsoluteAnchor{{DocumentOrder: 1, Pic: picture("middle"), Ext: parts.XDRExtent{CX: 9525, CY: 9525}}},
	}
	got := resolveDrawings(sd, nil, nil, nil, nil, nil, nil)
	if len(got) != 3 {
		t.Fatalf("drawings = %d, want 3", len(got))
	}
	for i, want := range []string{"bottom", "middle", "top"} {
		if got[i].Name != want || got[i].ZOrder != i {
			t.Fatalf("drawing %d = (%q,%d), want (%q,%d)", i, got[i].Name, got[i].ZOrder, want, i)
		}
	}
}
