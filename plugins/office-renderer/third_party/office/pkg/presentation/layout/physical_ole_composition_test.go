package layout

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestPhysicalOLEPlanOwnsFrameTransformEffectsAndPaint(t *testing.T) {
	shape := &ir.Shape{
		Kind:        ir.ShapeOleObject,
		Bounds:      ir.Rect{X: 12700, Y: 25400, W: 127000, H: 63500},
		RotationDeg: 31, FlipH: true,
		Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Glow{Radius: 3}}},
	}
	visuals := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(visuals) != 1 || visuals[0].OLE == nil {
		t.Fatal("OLE physical plan missing")
	}
	got := visuals[0].OLE
	if got.FrameBounds.LeftPt != 1 || got.FrameBounds.TopPt != 2 || got.FrameBounds.WidthPt != 10 || got.FrameBounds.HeightPt != 5 {
		t.Fatalf("OLE frame = %+v", got.FrameBounds)
	}
	if got.Transform.RotationDeg != 31 || !got.Transform.FlipH || got.Transform.FlipV {
		t.Fatalf("OLE transform = %+v", got.Transform)
	}
	if !got.Effects.HasEffects {
		t.Fatal("OLE effects were not materialized")
	}
	if got.Paint.ZIndex != 1 || !got.Paint.OverflowVisible {
		t.Fatalf("OLE paint = %+v", got.Paint)
	}
}


func TestClass15PhysicalOLEPlanOwnsRenderablePayload(t *testing.T) {
	shape := &ir.Shape{
		Kind: ir.ShapeOleObject,
		OLEObjectName: "Sheet", OLEObjectProgID: "Excel.Sheet.12",
		OLEObjectLinkURL: "https://example.test/object",
		OLEObjectMIME: "application/vnd.ms-office", OLEObjectData: []byte{1, 2, 3},
		OLEIconMIME: "image/png", OLEIconData: []byte{4, 5, 6},
	}
	plan := ResolvePhysicalOLEPlan(shape)
	if plan == nil || plan.Name != "Sheet" || plan.ProgID != "Excel.Sheet.12" ||
		plan.LinkURL == "" || plan.IconMIME != "image/png" || len(plan.IconData) != 3 {
		t.Fatalf("OLE physical ownership incomplete: %+v", plan)
	}
	shape.OLEObjectData[0], shape.OLEIconData[0] = 9, 9
	if plan.ObjectData[0] != 1 || plan.IconData[0] != 4 {
		t.Fatal("physical OLE plan aliases mutable canonical payload")
	}
}
