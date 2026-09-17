package layout

import (
	"math"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalAncestorPaintUsesRotatedGroupEnvelope(t *testing.T) {
	group := &ir.Shape{
		Kind: ir.ShapeGroup,
		GroupTransform: &ir.VisualTransform{
			Offset:      ir.Point{X: 12700, Y: 12700},
			Extent:      ir.VisualSize{W: 127000, H: 63500},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 127000, H: 63500},
			Rotation: 90,
		},
		Children: []*ir.Shape{{Kind: ir.ShapeText, Bounds: ir.Rect{X: 0, Y: 0, W: 127000, H: 63500}}},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 || len(got[0].Drawing.Paint.Ancestors) != 1 {
		t.Fatalf("physical ancestry = %#v", got)
	}
	b := got[0].Drawing.Paint.Ancestors[0].Bounds
	if math.Abs(b.WidthPt-5) > .0001 || math.Abs(b.HeightPt-10) > .0001 {
		t.Fatalf("rotated group paint envelope = %+v, want 5x10pt", b)
	}
}

func TestRotatedAncestorClipUsesTransformedEnvelopeForClippedPaint(t *testing.T) {
	group := &ir.Shape{
		Kind: ir.ShapeGroup, Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset:      ir.Point{X: 127000, Y: 127000},
			Extent:      ir.VisualSize{W: 127000, H: 63500},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 127000, H: 63500},
			Rotation: 90,
		},
		Children: []*ir.Shape{{Kind: ir.ShapeText, Bounds: ir.Rect{X: -127000, Y: -127000, W: 381000, H: 317500}}},
	}
	// Ellipse group geometry is an explicit authored clip in the canonical visual contract.
	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 || len(got[0].Drawing.Paint.AncestorClips) != 1 {
		t.Fatalf("ancestor clips = %#v", got)
	}
	clip := got[0].Drawing.Paint.AncestorClips[0]
	if len(clip.Polygon) < 16 {
		t.Fatalf("rotated ellipse clip polygon = %#v, want transformed polygon", clip.Polygon)
	}
	clipped := got[0].Drawing.Paint.ClippedBounds
	if math.Abs(clipped.WidthPt-5) > .0001 || math.Abs(clipped.HeightPt-10) > .0001 {
		t.Fatalf("rotated clipped paint envelope = %+v, want 5x10pt", clipped)
	}
}
