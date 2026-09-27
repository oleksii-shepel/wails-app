package officeart

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	wordir "office/pkg/word/ir"
)

func TestProjectShapeCarriesSharedVisuals(t *testing.T) {
	catalog := Catalog{Shapes: map[uint32]*officeartir.Shape{
		7: {
			ID:        7,
			Geometry:  officeartir.Geometry{Kind: officeartir.GeometryPreset, Preset: "ellipse", Adjustments: []float64{0.25}},
			Transform: officeartir.Transform{RotationDeg: 30, FlipH: true, FlipV: true},
			Fill:      officeartir.Fill{Kind: officeartir.FillSolid, Color: "112233"},
			Line:      officeartir.Line{Visible: true, Color: "445566", WidthPt: 2},
		},
	}}

	got := catalog.ProjectShape(7)
	if got == nil {
		t.Fatal("shape not projected")
	}
	if got.Kind != "ellipse" || got.Geometry == nil || got.Geometry.Name != "ellipse" {
		t.Fatalf("geometry=%q %+v", got.Kind, got.Geometry)
	}
	if got.RotationDeg != 30 || !got.FlipH || !got.FlipV {
		t.Fatalf("transform rot=%v flip=%v/%v", got.RotationDeg, got.FlipH, got.FlipV)
	}
	if got.FillColor != "112233" || got.StrokeColor != "445566" || got.StrokeWidth != 25400 {
		t.Fatalf("style fill=%q stroke=%q/%d", got.FillColor, got.StrokeColor, got.StrokeWidth)
	}
	if got.OfficeArt != catalog.Shapes[7] {
		t.Fatal("shared OfficeArt provenance not retained")
	}
}

func TestProjectShapeCarriesTextboxBody(t *testing.T) {
	catalog := Catalog{Shapes: map[uint32]*officeartir.Shape{
		9: {
			ID: 9,
			Text: &officeartir.TextBody{
				Insets:       officeartir.Insets{Left: 12700, Top: 25400},
				Wrap:         officeartir.TextWrapNone,
				Vert:         officeartir.TextVertical270,
				Anchor:       officeartir.TextAnchorCenter,
				AnchorCenter: true,
				AutoFit:      officeartir.AutoFitNormal,
			},
		},
	}}

	got := catalog.ProjectShape(9)
	if got == nil || got.TextBox == nil {
		t.Fatal("textbox not projected")
	}
	tb := got.TextBox
	if tb.LegacyShapeID != 9 || tb.PaddingLeft != 1 || tb.PaddingTop != 2 {
		t.Fatalf("textbox identity/insets=%+v", tb)
	}
	if !tb.NoWrap || !tb.HasWrapSetting || tb.Vert != "vert270" || !tb.HasVertSetting {
		t.Fatalf("textbox wrap/vert=%+v", tb)
	}
	if tb.Anchor != "center" || !tb.AnchorCenter || !tb.HasAnchorSetting || tb.Autofit != "norm" {
		t.Fatalf("textbox anchor/autofit=%+v", tb)
	}
}

func TestMergeTextBoxVisualPreservesAuthoredZeroValues(t *testing.T) {
	incoming := &wordir.TextBox{PaddingLeft: 7.2, NoWrap: true}
	visual := &wordir.TextBox{
		PaddingLeft:    0,
		HasPaddingLeft: true,
		NoWrap:         false,
		HasWrapSetting: true,
	}
	MergeTextBoxVisual(incoming, visual)
	if incoming.PaddingLeft != 0 || !incoming.HasPaddingLeft {
		t.Fatalf("left inset=%v present=%v", incoming.PaddingLeft, incoming.HasPaddingLeft)
	}
	if incoming.NoWrap {
		t.Fatal("authored wrap=false was lost")
	}
}

func TestAttachTextBoxesUsesAuthoritativeShapeID(t *testing.T) {
	shape := &wordir.Shape{LegacyShapeID: 20, TextBox: &wordir.TextBox{LegacyShapeID: 20, Vert: "vert"}}
	first := &wordir.TextBox{LegacyShapeID: 10, Blocks: []wordir.Block{{Kind: "paragraph"}}}
	second := &wordir.TextBox{LegacyShapeID: 20, Blocks: []wordir.Block{{Kind: "paragraph"}}}
	remaining := AttachTextBoxes(shape, []*wordir.TextBox{first, second})
	if shape.TextBox != second {
		t.Fatal("shape did not attach matching textbox story")
	}
	if shape.TextBox.Vert != "vert" {
		t.Fatal("shape-authored textbox visual was not merged")
	}
	if len(remaining) != 1 || remaining[0] != first {
		t.Fatalf("remaining=%#v", remaining)
	}
}

func TestAttachTextBoxesDoesNotRequireVisualPlaceholder(t *testing.T) {
	shape := &wordir.Shape{LegacyShapeID: 77, Kind: "rect", RotationDeg: 45}
	story := &wordir.TextBox{LegacyShapeID: 77, Blocks: []wordir.Block{{Kind: "paragraph"}}}
	remaining := AttachTextBoxes(shape, []*wordir.TextBox{story})
	if len(remaining) != 0 || shape.TextBox != story {
		t.Fatal("authoritative textbox story was not attached directly")
	}
	if shape.RotationDeg != 45 {
		t.Fatalf("shape rotation changed to %v", shape.RotationDeg)
	}
}

func TestAttachTextBoxesRecursesIntoGroups(t *testing.T) {
	root := &wordir.Shape{Kind: "group", Children: []wordir.Shape{{LegacyShapeID: 33, Kind: "rect"}}}
	story := &wordir.TextBox{LegacyShapeID: 33, Blocks: []wordir.Block{{Kind: "paragraph"}}}
	remaining := AttachTextBoxes(root, []*wordir.TextBox{story})
	if len(remaining) != 0 || root.Children[0].TextBox != story {
		t.Fatal("nested textbox story was not attached to owning child")
	}
}

func TestProjectShapeCarriesGeometryText(t *testing.T) {
	catalog := Catalog{Shapes: map[uint32]*officeartir.Shape{
		12: {
			ID: 12,
			GeometryText: &officeartir.GeometryText{
				Text:          "WordArt",
				FontFamily:    "Arial",
				FontSizePt:    24,
				Spacing:       1.1,
				Align:         1,
				Warp:          "textWave1",
				Bold:          true,
				Italic:        true,
				Underline:     true,
				Strikethrough: true,
			},
		},
	}}
	got := catalog.ProjectShape(12)
	if got == nil || got.TextPath == nil {
		t.Fatal("geometry text not projected")
	}
	if got.TextPath.String != "WordArt" || got.TextPath.Font != "Arial" || got.TextPath.SizePt != 24 || got.TextPath.Warp != "textWave1" {
		t.Fatalf("text path=%+v", got.TextPath)
	}
	if !got.TextPath.Bold || !got.TextPath.Italic || !got.TextPath.Underline || !got.TextPath.Strikethrough {
		t.Fatalf("text path formatting=%+v", got.TextPath)
	}
}

func TestProjectShapeCarriesDOCGroupChildGeometry(t *testing.T) {
	child := &officeartir.Shape{
		ID:        2,
		Transform: officeartir.Transform{Rect: officeartir.Rect{X: 10, Y: 20, W: 30, H: 40}},
		Geometry:  officeartir.Geometry{Kind: officeartir.GeometryPreset, Preset: "rect"},
	}
	root := &officeartir.Shape{ID: 1, Kind: officeartir.ShapeGroup, Children: []*officeartir.Shape{child}}
	catalog := Catalog{Shapes: map[uint32]*officeartir.Shape{1: root, 2: child}}

	got := catalog.ProjectShape(1)
	if got == nil || got.Kind != "group" || len(got.Children) != 1 {
		t.Fatalf("group=%+v", got)
	}
	c := got.Children[0]
	if c.X != 10 || c.Y != 20 || c.Width != 30 || c.Height != 40 {
		t.Fatalf("child geometry=%+v", c)
	}
}

func TestApplyImageVisualCarriesTransformCropAndAdjustments(t *testing.T) {
	catalog := Catalog{Shapes: map[uint32]*officeartir.Shape{
		5: {
			ID:        5,
			Transform: officeartir.Transform{RotationDeg: 45, FlipH: true},
			Image: &officeartir.Image{
				Crop:       officeartir.Crop{Left: .1, Top: .2, Right: .3, Bottom: .4},
				Brightness: 1.25,
				Contrast:   .75,
				Grayscale:  true,
				BiLevel:    true,
			},
			Line: officeartir.Line{Visible: true, Color: "ABCDEF", WidthPt: 1.5},
		},
	}}
	image := &wordir.Image{}
	if !catalog.ApplyImageVisual(5, image) {
		t.Fatal("image visual not applied")
	}
	if image.RotationDeg != 45 || !image.FlipH || image.FlipV {
		t.Fatalf("image transform=%+v", image)
	}
	if image.CropLeft != .1 || image.CropTop != .2 || image.CropRight != .3 || image.CropBottom != .4 {
		t.Fatalf("image crop=%+v", image)
	}
	if image.Brightness != 1.25 || image.Contrast != .75 || !image.Grayscale || !image.BiLevel {
		t.Fatalf("image adjustments=%+v", image)
	}
	if image.BorderColor != "ABCDEF" || image.BorderWidth != 1.5 {
		t.Fatalf("image border=%q/%v", image.BorderColor, image.BorderWidth)
	}
}

func TestWordLegacyRotationUsesScreenDirection(t *testing.T) {
	if got := wordLegacyRotationDeg(45); got != -45 {
		t.Fatalf("rotation = %v, want -45", got)
	}
	if got := wordLegacyRotationDeg(315); got != 45 {
		t.Fatalf("rotation = %v, want 45", got)
	}
}
