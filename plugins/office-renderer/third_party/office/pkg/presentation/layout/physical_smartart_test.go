package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalSmartArtPlanOwnsNodeContent(t *testing.T) {
	p := &ir.Paragraph{}
	shape := &ir.Shape{Kind: ir.ShapeSmartArt, Paragraphs: []*ir.Paragraph{p}}
	plan := ResolvePhysicalSmartArtPlan(shape)
	if plan == nil || len(plan.Paragraphs) != 1 || plan.Paragraphs[0] != p {
		t.Fatalf("physical SmartArt content = %#v", plan)
	}
	shape.Paragraphs = nil
	if len(plan.Paragraphs) != 1 {
		t.Fatal("physical SmartArt plan must own its paragraph slice")
	}
}

func TestMaterializePhysicalVisualTreeCarriesSmartArtPlan(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeSmartArt, Bounds: ir.Rect{W: 127000, H: 63500}, Paragraphs: []*ir.Paragraph{{}}}
	items := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(items) != 1 || items[0].SmartArt == nil || len(items[0].SmartArt.Paragraphs) != 1 {
		t.Fatalf("materialized SmartArt = %#v", items)
	}
}
