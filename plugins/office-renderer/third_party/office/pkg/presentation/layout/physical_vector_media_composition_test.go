package layout

import (
	"office/pkg/presentation/ir"
	"testing"
)

func TestPhysicalVectorPaintBoundsIncludeStrokeAndArrowEnvelope(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeConnector, Bounds: ir.Rect{X: 12700 * 10, Y: 12700 * 20, W: 12700 * 100, H: 12700 * 40}, LineWidthPt: 4, ArrowHead: "triangle", ArrowHeadWidth: "lg", ArrowHeadLength: "lg"}
	got := resolvePhysicalVectorPaintBounds(shape, physicalBoundsFromRect(shape.Bounds), PhysicalOverflowPlan{})
	if got.LeftPt >= 10 || got.TopPt >= 20 || got.WidthPt <= 100 || got.HeightPt <= 40 {
		t.Fatalf("vector paint envelope = %+v, want stroke/arrow expansion", got)
	}
}

func TestPhysicalEndpointConnectorOwnsSlideSpacePaintEnvelope(t *testing.T) {
	start := ir.Point{X: 12700 * 25, Y: 12700 * 35}
	end := ir.Point{X: 12700 * 125, Y: 12700 * 85}
	shape := &ir.Shape{Kind: ir.ShapeConnector, ConnectorStart: &start, ConnectorEnd: &end, LineWidthPt: 2, ArrowHead: "triangle", ArrowHeadWidth: "med", ArrowHeadLength: "med"}
	visuals := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(visuals) != 1 || visuals[0].FreeConnector == nil {
		t.Fatalf("free connector not materialized: %+v", visuals)
	}
	b := visuals[0].Drawing.Paint.Bounds
	if b.Empty() || b.LeftPt >= 25 || b.TopPt >= 35 || b.WidthPt <= 100 || b.HeightPt <= 50 {
		t.Fatalf("endpoint connector paint envelope = %+v, want path/stroke/arrow bounds", b)
	}
}

func TestPhysicalImageCompositionKeepsCropInsideSharedPaintContract(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeImage, Bounds: ir.Rect{X: 12700 * 10, Y: 12700 * 20, W: 12700 * 200, H: 12700 * 100}, ImageCropLeft: .25, ImageCropRight: .10, RotationDeg: 15}
	visuals := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(visuals) != 1 || visuals[0].Image == nil {
		t.Fatal("image plan missing")
	}
	if visuals[0].Image.Composition.Paint.Bounds.Empty() {
		t.Fatal("image paint bounds missing")
	}
	if visuals[0].Image.SourceRect.Left != .25 || visuals[0].Image.SourceRect.Right != .10 {
		t.Fatalf("crop source rect = %+v", visuals[0].Image.SourceRect)
	}
	if visuals[0].Image.Composition.Transform.RotationDeg != 15 {
		t.Fatalf("rotation = %v", visuals[0].Image.Composition.Transform.RotationDeg)
	}
}

func TestPhysicalEndpointConnectorPreservesEffectOverflow(t *testing.T) {
	start := ir.Point{X: 12700 * 25, Y: 12700 * 35}
	end := ir.Point{X: 12700 * 125, Y: 12700 * 85}
	shape := &ir.Shape{Kind: ir.ShapeConnector, ConnectorStart: &start, ConnectorEnd: &end, LineWidthPt: 2}
	overflow := PhysicalOverflowPlan{LeftPt: 7, TopPt: 11, RightPt: 13, BottomPt: 17}
	got := resolvePhysicalVectorPaintBounds(shape, PhysicalPaintBoundsPlan{}, overflow)
	if got.LeftPt > 17 || got.TopPt > 23 || got.LeftPt+got.WidthPt < 139 || got.TopPt+got.HeightPt < 103 {
		t.Fatalf("endpoint connector effect envelope = %+v, want path plus asymmetric effect overflow", got)
	}
}

func TestPhysicalEndpointConnectorUnionsAuthoredFrameAndPathEnvelope(t *testing.T) {
	start := ir.Point{X: 12700 * 100, Y: 12700 * 100}
	end := ir.Point{X: 12700 * 200, Y: 12700 * 200}
	shape := &ir.Shape{Kind: ir.ShapeConnector, ConnectorStart: &start, ConnectorEnd: &end, LineWidthPt: 2}
	base := PhysicalPaintBoundsPlan{LeftPt: 10, TopPt: 20, WidthPt: 30, HeightPt: 40}
	got := resolvePhysicalVectorPaintBounds(shape, base, PhysicalOverflowPlan{})
	if got.LeftPt > 10 || got.TopPt > 20 || got.LeftPt+got.WidthPt < 201 || got.TopPt+got.HeightPt < 201 {
		t.Fatalf("connector union envelope = %+v, want authored frame and endpoint path", got)
	}
}
