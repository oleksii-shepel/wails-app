package layout

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestPhysicalVisualOwnsTextFrameFlowAndMeasurement(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 914400 * 3, H: 914400 * 2},
		TextBodyProps: drawing.TextBodyProperties{
			Anchor: "ctr", Wrap: "none", Vert: "vert270", Columns: 2,
			ColumnSpacingEMU: 91440,
		},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "Physical text", FontSize: 1800}}}},
	}
	items := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(items) != 1 || items[0].Text == nil {
		t.Fatalf("physical text plan missing: %+v", items)
	}
	plan := items[0].Text
	if plan.Frame.VerticalJustify != "center" || !plan.Flow.NoWrap {
		t.Fatalf("frame/flow semantics lost: %+v", plan)
	}
	if plan.Flow.WritingMode != "vertical-lr" || plan.Flow.RotationDeg != 180 {
		t.Fatalf("vert270 physical composition = %+v", plan.Flow)
	}
	if plan.Flow.Columns != 2 || plan.Flow.ColumnGapPt <= 0 {
		t.Fatalf("column composition lost: %+v", plan.Flow)
	}
}
