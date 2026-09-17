package layout

import (
	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestVerticalTextOwnsPhysicalGlyphAndProgressionAxes(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 2540000, H: 1270000}, TextBodyProps: drawing.TextBodyProperties{Vert: "vert"}}
	frame := ResolveTextFrameMetrics(shape)
	flow := ResolveTextFlowMetrics(shape, frame)
	if !flow.Vertical || flow.WritingMode != "vertical-rl" {
		t.Fatalf("flow = %+v", flow)
	}
	if flow.LineExtentPt <= 0 || flow.ProgressionExtentPt <= 0 {
		t.Fatalf("physical axes = %+v", flow)
	}
}

func TestTextVisualPlanMaterializesVerticalParagraphLines(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 2540000, H: 1270000}, TextBodyProps: drawing.TextBodyProperties{Vert: "vert"}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "Vertical text", FontSize: 1200}}}}}
	plan := ResolveTextVisualPlan(shape)
	if len(plan.Paragraphs) != 1 || !plan.Paragraphs[0].LinesMeasured {
		t.Fatalf("vertical paragraph not materialized: %+v", plan.Paragraphs)
	}
}

func TestVerticalAnchorOffsetsProgressionEdgeNotTop(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 2540000, H: 1270000}, TextBodyProps: drawing.TextBodyProperties{Vert: "vert", Anchor: "ctr"}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "x", FontSize: 1200}}}}}
	plan := ResolveTextVisualPlan(shape)
	if plan.Box.ContentTopPt != 0 {
		t.Fatalf("vertical anchor leaked into top offset: %+v", plan.Box)
	}
	if plan.Box.ContentRightPt <= 0 {
		t.Fatalf("vertical-rl center anchor did not offset right progression edge: %+v", plan.Box)
	}
}

func TestAutoNumberTextIsMaterializedInTextVisualPlan(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 4000000, H: 2000000}, Paragraphs: []*ir.Paragraph{
		{AutoNum: true, AutoNumType: "arabicPeriod", AutoNumStart: 9, Runs: []*ir.Run{{Text: "nine", FontSize: 1200}}},
		{AutoNum: true, AutoNumType: "arabicPeriod", AutoNumStart: 9, Runs: []*ir.Run{{Text: "ten", FontSize: 1200}}},
	}}
	plan := ResolveTextVisualPlan(shape)
	if plan.Paragraphs[0].Marker.Text != "9." || plan.Paragraphs[1].Marker.Text != "10." {
		t.Fatalf("markers = %q, %q", plan.Paragraphs[0].Marker.Text, plan.Paragraphs[1].Marker.Text)
	}
}

func TestExactAutoNumberReflowPreservesPhysicalMarkerFontAndGap(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 1270000}, Paragraphs: []*ir.Paragraph{
		{AutoNum: true, AutoNumType: "arabicPeriod", AutoNumStart: 10, BulletFontFamily: "Arial", BulletSize: 2400, Runs: []*ir.Run{{Text: "word word word", FontSize: 1200}}},
	}}
	plan := ResolveTextVisualPlan(shape)
	if len(plan.Paragraphs) != 1 || plan.Paragraphs[0].Marker.Text != "10." {
		t.Fatalf("exact auto-number marker was not materialized: %+v", plan.Paragraphs)
	}
	marker := plan.Paragraphs[0].Marker
	if marker.FontFamily != "Arial" || marker.GapPt <= 0 {
		t.Fatalf("physical marker font/gap lost during exact reflow: %+v", marker)
	}
	if !plan.Paragraphs[0].LinesMeasured || len(plan.Paragraphs[0].Lines) == 0 {
		t.Fatal("exact auto-number paragraph was not physically reflowed")
	}
}
