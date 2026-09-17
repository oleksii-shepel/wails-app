package layout

import "testing"

func TestClass18DisjointAncestorClipProducesEmptyPhysicalPaint(t *testing.T) {
	paint := PhysicalPaintBoundsPlan{LeftPt: 100, TopPt: 100, WidthPt: 20, HeightPt: 20}
	clips := []PhysicalAncestorClipPlan{{
		Bounds: PhysicalPaintBoundsPlan{LeftPt: 0, TopPt: 0, WidthPt: 50, HeightPt: 50},
		Intersects: false,
	}}
	got, clipped := resolveClippedPhysicalPaintBounds(paint, clips)
	if !clipped || !got.Empty() {
		t.Fatalf("disjoint clip = %+v, clipped=%v; want empty/clipped", got, clipped)
	}
}

func TestClass18PartialAncestorClipOwnsReducedPaintEnvelope(t *testing.T) {
	paint := PhysicalPaintBoundsPlan{LeftPt: 40, TopPt: 40, WidthPt: 30, HeightPt: 30}
	clips := []PhysicalAncestorClipPlan{{
		Bounds: PhysicalPaintBoundsPlan{LeftPt: 0, TopPt: 0, WidthPt: 50, HeightPt: 50},
		Intersects: true,
	}}
	got, clipped := resolveClippedPhysicalPaintBounds(paint, clips)
	if !clipped || got.LeftPt != 40 || got.TopPt != 40 || got.WidthPt != 10 || got.HeightPt != 10 {
		t.Fatalf("partial clip = %+v, clipped=%v; want 40,40 10x10", got, clipped)
	}
}
