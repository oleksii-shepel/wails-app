package legacy

import (
	"math"
	"testing"

	officeartir "office/pkg/officeart/ir"
	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
)

func TestBIG39OfficeArtVisualBridgeCarriesWholeAutoShapeSurface(t *testing.T) {
	shape := &presentationir.Shape{}
	shared := &officeartir.Shape{
		Transform: officeartir.Transform{RotationDeg: 37.5, FlipH: true, FlipV: true},
		Fill: officeartir.Fill{Kind: officeartir.FillGradient, Gradient: &officeartir.GradientFill{
			Type: "linear", Angle: 42,
			Stops: []officeartir.GradientStop{{Position: 0, Color: "112233", Alpha: .4}, {Position: 1, Color: "AABBCC", Alpha: 1}},
		}},
		Line:  officeartir.Line{Visible: true, Color: "445566", Alpha: .5, WidthPt: 2.25, Dash: "dashDot", Cap: "round", Join: "bevel"},
		Image: &officeartir.Image{DPI: 144, Crop: officeartir.Crop{Left: .1, Top: .2, Right: .3, Bottom: .4}},
	}
	applyLegacyPptOfficeArtVisual(shape, shared)
	if shape.RotationDeg != 37.5 || !shape.FlipH || !shape.FlipV {
		t.Fatalf("transform = %#v", shape)
	}
	g, ok := shape.Fill.(drawing.GradientFill)
	if !ok || g.Angle != 42 || len(g.Stops) != 2 || g.Stops[0].Alpha != .4 {
		t.Fatalf("gradient = %#v", shape.Fill)
	}
	if shape.LineStyle == nil || shape.LineStyle.Color != "445566" || shape.LineStyle.Width != 2.25 || shape.LineStyle.Dash != drawing.DashPatternDashDot || shape.LineStyle.Cap != drawing.LineCapRound || shape.LineStyle.Join != drawing.LineJoinBevel {
		t.Fatalf("line = %#v", shape.LineStyle)
	}
	if shape.ImageDPI != 144 || shape.ImageCropLeft != .1 || shape.ImageCropTop != .2 || shape.ImageCropRight != .3 || shape.ImageCropBottom != .4 {
		t.Fatalf("image semantics = %#v", shape)
	}
}

func TestBIG39OfficeArtVisualBridgePreservesNoFillAndSolidAlpha(t *testing.T) {
	shape := &presentationir.Shape{FillColor: "FFFFFF", Fill: drawing.SolidFill{Color: "FFFFFF", Alpha: 1}}
	applyLegacyPptOfficeArtVisual(shape, &officeartir.Shape{Fill: officeartir.Fill{Kind: officeartir.FillNone}})
	if shape.FillColor != "" {
		t.Fatalf("no-fill retained FillColor %q", shape.FillColor)
	}
	if _, ok := shape.Fill.(drawing.NoFill); !ok {
		t.Fatalf("no-fill = %#v", shape.Fill)
	}
	applyLegacyPptOfficeArtVisual(shape, &officeartir.Shape{Fill: officeartir.Fill{Kind: officeartir.FillSolid, Color: "123456", Alpha: .25}})
	solid, ok := shape.Fill.(drawing.SolidFill)
	if !ok || solid.Color != "123456" || solid.Alpha != .25 {
		t.Fatalf("solid = %#v", shape.Fill)
	}
}

func TestLegacyOfficeArtVisualBridgePreservesPictureAdjustments(t *testing.T) {
	shape := &presentationir.Shape{}
	shared := &officeartir.Shape{Image: &officeartir.Image{
		Brightness: 1.5,
		Contrast:   0.75,
		Grayscale:  true,
		BiLevel:    true,
	}}

	applyLegacyPptOfficeArtVisual(shape, shared)

	if len(shape.ImageEffects) != 3 {
		t.Fatalf("image effects = %#v, want luminance + grayscale + bilevel", shape.ImageEffects)
	}
	lum, ok := shape.ImageEffects[0].(drawing.Lum)
	if !ok || lum.Brightness != 50000 || lum.Contrast != -25000 {
		t.Fatalf("luminance = %#v", shape.ImageEffects[0])
	}
	if _, ok := shape.ImageEffects[1].(drawing.Grayscale); !ok {
		t.Fatalf("grayscale = %#v", shape.ImageEffects[1])
	}
	bi, ok := shape.ImageEffects[2].(drawing.BiLevel)
	if !ok || bi.Threshold != 50000 {
		t.Fatalf("bilevel = %#v", shape.ImageEffects[2])
	}
}

func TestLegacyOfficeArtVisualBridgeDoesNotInventDefaultPictureAdjustments(t *testing.T) {
	shape := &presentationir.Shape{}
	applyLegacyPptOfficeArtVisual(shape, &officeartir.Shape{Image: &officeartir.Image{Brightness: 1, Contrast: 1}})
	if len(shape.ImageEffects) != 0 {
		t.Fatalf("default picture adjustments produced effects: %#v", shape.ImageEffects)
	}
}

func TestBIG39OfficeArtVisualBridgeCarriesLegacyRadialGradientCenter(t *testing.T) {
	shape := &presentationir.Shape{}
	shared := &officeartir.Shape{Fill: officeartir.Fill{Kind: officeartir.FillGradient, Gradient: &officeartir.GradientFill{
		Type: "radial", HasFillTo: true, FillToLeft: .5, FillToTop: .5, FillToRight: .5, FillToBottom: .5,
		Stops: []officeartir.GradientStop{{Position: 0, Color: "112233", Alpha: 1}, {Position: 1, Color: "AABBCC", Alpha: 1}},
	}}}
	applyLegacyPptOfficeArtVisual(shape, shared)
	g, ok := shape.Fill.(drawing.GradientFill)
	if !ok || g.FillToRect == nil {
		t.Fatalf("gradient = %#v", shape.Fill)
	}
	if g.FillToRect.X != .5 || g.FillToRect.Y != .5 || g.FillToRect.Width != 0 || g.FillToRect.Height != 0 {
		t.Fatalf("fillToRect = %#v", g.FillToRect)
	}
}

func TestClass4OfficeArtVisualBridgePreservesPictureFillSemantics(t *testing.T) {
	shape := &presentationir.Shape{}
	shared := &officeartir.Shape{Fill: officeartir.Fill{
		Kind:  officeartir.FillPicture,
		Alpha: .65,
		Picture: &officeartir.PictureFill{
			Image: officeartir.Image{
				Data: []byte{1, 2, 3}, MIMEType: "image/png", RelID: "rId7", URL: "media/image.png", DPI: 144,
				Crop: officeartir.Crop{Left: .1, Top: .2, Right: .3, Bottom: .1},
			},
			Tile: &officeartir.Tile{OffsetXEMU: 1200, OffsetYEMU: 3400, ScaleX: .5, ScaleY: 1.25},
		},
	}}

	applyLegacyPptOfficeArtVisual(shape, shared)

	fill, ok := shape.Fill.(drawing.PictureFill)
	if !ok {
		t.Fatalf("fill = %T, want drawing.PictureFill", shape.Fill)
	}
	if fill.ContentType != "image/png" || fill.EmbedID != "rId7" || fill.ImageURL != "media/image.png" || fill.DPI != 144 || fill.Alpha != .65 {
		t.Fatalf("picture identity = %#v", fill)
	}
	if !fill.Tile || fill.Stretch || fill.TileOffsetX != 1200 || fill.TileOffsetY != 3400 || fill.TileScaleX != 50000 || fill.TileScaleY != 125000 {
		t.Fatalf("picture tile = %#v", fill)
	}
	if fill.Crop == nil || math.Abs(fill.Crop.X-.1) > 1e-9 || math.Abs(fill.Crop.Y-.2) > 1e-9 || math.Abs(fill.Crop.Width-.6) > 1e-9 || math.Abs(fill.Crop.Height-.7) > 1e-9 {
		t.Fatalf("picture crop = %#v", fill.Crop)
	}
}

func TestClass4OfficeArtVisualBridgeDefaultsPictureFillToStretchAndOpaque(t *testing.T) {
	shape := &presentationir.Shape{}
	applyLegacyPptOfficeArtVisual(shape, &officeartir.Shape{Fill: officeartir.Fill{
		Kind:    officeartir.FillPicture,
		Picture: &officeartir.PictureFill{Image: officeartir.Image{Data: []byte{1}, MIMEType: "image/png"}},
	}})
	fill, ok := shape.Fill.(drawing.PictureFill)
	if !ok {
		t.Fatalf("fill = %T, want drawing.PictureFill", shape.Fill)
	}
	if !fill.Stretch || fill.Tile || fill.Alpha != 1 {
		t.Fatalf("picture defaults = %#v", fill)
	}
}

func TestClass6OfficeArtVisualBridgePreservesFullyTransparentStroke(t *testing.T) {
	shape := &presentationir.Shape{}
	applyLegacyPptOfficeArtVisual(shape, &officeartir.Shape{Line: officeartir.Line{Visible: true, Color: "445566", Alpha: 0, WidthPt: 2}})
	if shape.LineStyle == nil || shape.LineStyle.Alpha != 0 {
		t.Fatalf("transparent line = %#v", shape.LineStyle)
	}
}
