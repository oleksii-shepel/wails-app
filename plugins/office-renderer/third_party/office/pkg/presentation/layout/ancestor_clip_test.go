package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalPaintCarriesOnlyExplicitAncestorClips(t *testing.T) {
	group := &ir.Shape{
		Kind:     ir.ShapeGroup,
		Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset:      ir.Point{X: 12700, Y: 12700},
			Extent:      ir.VisualSize{W: 127000, H: 127000},
			ChildOffset: ir.Point{},
			ChildExtent: ir.VisualSize{W: 127000, H: 127000},
		},
		Children: []*ir.Shape{{
			Kind:   ir.ShapeText,
			Bounds: ir.Rect{X: -12700, Y: -12700, W: 152400, H: 152400},
		}},
	}

	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 {
		t.Fatalf("physical visuals = %d, want 1", len(got))
	}
	paint := got[0].Drawing.Paint
	if len(paint.AncestorClips) != 1 {
		t.Fatalf("ancestor clips = %#v, want one explicit group clip", paint.AncestorClips)
	}
	if paint.AncestorClips[0].Kind != "ellipse" {
		t.Fatalf("ancestor clip kind = %q, want ellipse", paint.AncestorClips[0].Kind)
	}
	if !paint.PaintClipped {
		t.Fatal("child paint extending beyond explicit ancestor clip must be marked clipped")
	}
	if paint.ClippedBounds.LeftPt != 1 || paint.ClippedBounds.TopPt != 1 || paint.ClippedBounds.WidthPt != 10 || paint.ClippedBounds.HeightPt != 10 {
		t.Fatalf("clipped bounds = %+v, want 1,1 10x10pt", paint.ClippedBounds)
	}
}

func TestOrdinaryGroupEnvelopeStillDoesNotBecomeClip(t *testing.T) {
	group := &ir.Shape{
		Kind: ir.ShapeGroup,
		GroupTransform: &ir.VisualTransform{
			Offset:      ir.Point{X: 12700, Y: 12700},
			Extent:      ir.VisualSize{W: 127000, H: 127000},
			ChildOffset: ir.Point{},
			ChildExtent: ir.VisualSize{W: 127000, H: 127000},
		},
		Children: []*ir.Shape{{Kind: ir.ShapeText, Bounds: ir.Rect{X: -12700, Y: -12700, W: 152400, H: 152400}}},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 {
		t.Fatalf("physical visuals = %d, want 1", len(got))
	}
	if len(got[0].Drawing.Paint.AncestorClips) != 0 || got[0].Drawing.Paint.PaintClipped {
		t.Fatalf("ordinary group became implicit clip: %+v", got[0].Drawing.Paint)
	}
}

func TestEllipseAncestorClipRejectsPaintInsideEnvelopeButOutsideClip(t *testing.T) {
	group := &ir.Shape{
		Kind: ir.ShapeGroup, Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset: ir.Point{}, Extent: ir.VisualSize{W: 127000, H: 127000},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 127000, H: 127000},
		},
		Children: []*ir.Shape{{Kind: ir.ShapeText, Bounds: ir.Rect{X: 0, Y: 0, W: 12700, H: 12700}}},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 {
		t.Fatalf("physical visuals = %d, want 1", len(got))
	}
	paint := got[0].Drawing.Paint
	if !paint.PaintClipped || !paint.ClippedBounds.Empty() {
		t.Fatalf("corner leaf outside ellipse = %+v, want fully clipped physical bounds", paint)
	}
}

func TestNestedAncestorClipsRejectLeafAfterSecondClip(t *testing.T) {
	inner := &ir.Shape{
		Kind: ir.ShapeGroup, Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset: ir.Point{}, Extent: ir.VisualSize{W: 127000, H: 127000},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 127000, H: 127000},
		},
		Children: []*ir.Shape{{Kind: ir.ShapeText, Bounds: ir.Rect{X: 0, Y: 0, W: 12700, H: 12700}}},
	}
	outer := &ir.Shape{
		Kind: ir.ShapeGroup, Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset: ir.Point{}, Extent: ir.VisualSize{W: 254000, H: 254000},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 254000, H: 254000},
		},
		Children: []*ir.Shape{inner},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{outer})
	if len(got) != 1 || len(got[0].Drawing.Paint.AncestorClips) != 2 {
		t.Fatalf("nested clip plan = %#v", got)
	}
	paint := got[0].Drawing.Paint
	if !paint.PaintClipped || !paint.ClippedBounds.Empty() {
		t.Fatalf("inner ellipse must reject leaf even when outer clip intersects it: %+v", paint)
	}
}
