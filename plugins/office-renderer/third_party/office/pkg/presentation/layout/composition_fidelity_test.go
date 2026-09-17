package layout

import (
	"office/pkg/presentation/ir"
	"testing"
)

func TestPhysicalVisualCarriesPaintClipAndOverflowContract(t *testing.T) {
	shapes := []*ir.Shape{
		{Kind: ir.ShapeText, Geometry: "ellipse", Bounds: ir.Rect{W: 100, H: 100}},
		{Kind: ir.ShapeText, Bounds: ir.Rect{W: 100, H: 100}},
	}
	got := MaterializePhysicalVisualTree(shapes)
	if len(got) != 2 || got[0].Drawing.Paint.ZIndex != 1 || got[1].Drawing.Paint.ZIndex != 2 {
		t.Fatalf("paint order = %#v", got)
	}
	if got[0].Drawing.Paint.Clip.Kind != "ellipse" || got[0].Drawing.Paint.Clip.CSSPath != "ellipse(50% 50% at 50% 50%)" || !got[0].Drawing.Paint.Clip.ClipContent {
		t.Fatalf("ellipse physical clip = %#v", got[0].Drawing.Paint.Clip)
	}
}

func TestImageVisualPlanCarriesShapeClipWithCrop(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeImage, Geometry: "ellipse", ImageCropLeft: .1, ImageCropRight: .2}
	got := ResolveImageVisualPlan(shape)
	if got.ClipKind != "ellipse" || got.ClipPath == "" {
		t.Fatalf("image clip = %#v", got)
	}
	if got.ImageWidth <= 100 || got.ImageLeft >= 0 {
		t.Fatalf("crop not materialized with clip: %#v", got)
	}
}
