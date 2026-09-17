package layout

import (
	"math"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalAncestorClipPreservesTransformedClipGeometry(t *testing.T) {
	group := &ir.Shape{
		Kind:     ir.ShapeGroup,
		Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset:      ir.Point{X: 127000, Y: 254000},
			Extent:      ir.VisualSize{W: 508000, H: 254000},
			ChildExtent: ir.VisualSize{W: 508000, H: 254000},
			Rotation:    30,
		},
		Children: []*ir.Shape{{Kind: ir.ShapeText, Bounds: ir.Rect{W: 508000, H: 254000}}},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 || len(got[0].Drawing.Paint.AncestorClips) != 1 {
		t.Fatalf("physical clips = %#v, want one", got)
	}
	clip := got[0].Drawing.Paint.AncestorClips[0]
	if math.Abs(clip.RotationDeg-30) > 0.001 {
		t.Fatalf("clip rotation = %.3f, want 30", clip.RotationDeg)
	}
	if len(clip.Polygon) != 32 {
		t.Fatalf("ellipse polygon points = %d, want 32", len(clip.Polygon))
	}
}

func TestAxisAlignedEllipseAncestorClipKeepsPhysicalGeometryForVisibility(t *testing.T) {
	bounds := PhysicalPaintBoundsPlan{LeftPt: 1, TopPt: 2, WidthPt: 20, HeightPt: 10}
	got := physicalAncestorClipPolygon(bounds, "ellipse", 0, false, false)
	if len(got) != 32 {
		t.Fatalf("axis-aligned ellipse polygon points = %d, want 32", len(got))
	}
	if envelope := physicalClipPolygonBounds(got); math.Abs(envelope.LeftPt-1) > 1e-9 || math.Abs(envelope.TopPt-2) > 1e-9 || math.Abs(envelope.WidthPt-20) > 1e-9 || math.Abs(envelope.HeightPt-10) > 1e-9 {
		t.Fatalf("axis-aligned ellipse polygon envelope = %+v, want original bounds", envelope)
	}
}

func TestAxisAlignedRectAncestorClipKeepsCompactGeometry(t *testing.T) {
	bounds := PhysicalPaintBoundsPlan{LeftPt: 1, TopPt: 2, WidthPt: 20, HeightPt: 10}
	if got := physicalAncestorClipPolygon(bounds, "rect", 0, false, false); got != nil {
		t.Fatalf("axis-aligned rectangular clip polygon = %#v, want compact nil geometry", got)
	}
}
