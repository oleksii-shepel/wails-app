package layout

import (
	"math"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestTextVisualPlanMeasuresFrameFromSamePhysicalParagraphLines(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "one two three four five", FontSize: 1200, FontFamily: "Arial"}}}
	shape := &ir.Shape{Bounds: ir.Rect{W: 12700 * 90, H: 12700 * 80}, Paragraphs: []*ir.Paragraph{p}}
	plan := ResolveTextVisualPlan(shape)
	if len(plan.Paragraphs) != 1 || !plan.Paragraphs[0].LinesMeasured {
		t.Fatalf("paragraph lines not materialized: %+v", plan.Paragraphs)
	}
	if !plan.Layout.Measured {
		t.Fatal("frame must be measured from physical paragraph lines")
	}
	if got, want := plan.Layout.ContentHeightPt, physicalParagraphLinesHeight(plan.Paragraphs[0].Lines); got != want {
		t.Fatalf("content height %.3f want %.3f", got, want)
	}
}

func TestNoWrapStillMaterializesAuthoredPhysicalLines(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "Clock Data\nEphemeris", FontSize: 1600, FontFamily: "Arial"}}}
	shape := &ir.Shape{Bounds: ir.Rect{W: 12700 * 40, H: 12700 * 80}, TextBodyProps: drawing.TextBodyProperties{Wrap: "none"}, Paragraphs: []*ir.Paragraph{p}}
	plan := ResolveTextVisualPlan(shape)
	if len(plan.Paragraphs) != 1 || !plan.Paragraphs[0].LinesMeasured {
		t.Fatal("noWrap must retain physical authored lines")
	}
	lines := plan.Paragraphs[0].Lines
	if len(lines) != 2 || lines[0].BreakAfter != "authored" {
		t.Fatalf("authored lines = %+v", lines)
	}
	if lines[0].Fragments[0].Text != "Clock Data" {
		t.Fatalf("first line = %q", lines[0].Fragments[0].Text)
	}
}

func TestMultiColumnParagraphUsesPhysicalColumnWidth(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 12700 * 200, H: 12700 * 100}, TextBodyProps: drawing.TextBodyProperties{Columns: 2, ColumnSpacingEMU: 12700 * 10}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "alpha beta gamma delta epsilon", FontSize: 1200}}}}}
	plan := ResolveTextVisualPlan(shape)
	if plan.Flow.ColumnWidthPt <= 0 || plan.Flow.LineExtentPt != plan.Flow.ColumnWidthPt {
		t.Fatalf("flow=%+v", plan.Flow)
	}
	if plan.Paragraphs[0].Metrics.AvailableWidthPt > plan.Flow.ColumnWidthPt+.001 {
		t.Fatalf("paragraph width %.3f exceeds column %.3f", plan.Paragraphs[0].Metrics.AvailableWidthPt, plan.Flow.ColumnWidthPt)
	}
}

func TestNormAutofitWithoutAuthoredScaleKeepsAuthoredFontSize(t *testing.T) {
	shape := &ir.Shape{
		Bounds:        ir.Rect{W: 12700 * 180, H: 12700 * 70},
		TextBodyProps: drawing.TextBodyProperties{AutofitType: "normAutofit"},
		Paragraphs:    []*ir.Paragraph{{Runs: []*ir.Run{{Text: "From Alexandria Egypt", FontSize: 2400, FontFamily: "Arial"}}}},
	}
	plan := ResolveTextVisualPlan(shape)
	if plan.Frame.FontScale != 1 {
		t.Fatalf("normAutofit font scale = %.4f, want authored/default 1.0 when fontScale is absent", plan.Frame.FontScale)
	}
}

// PowerPoint ignores space-before on the first paragraph of a text body. Slide 5
// of the 23884 fixture drifts ~6.9pt below its authored frame without this.
func TestFirstBodyParagraphDropsAuthoredSpaceBefore(t *testing.T) {
	newBody := func() []*ir.Paragraph {
		return []*ir.Paragraph{
			{SpaceBefore: 688, HasSpaceBefore: true, Runs: []*ir.Run{{Text: "GPS Block I", FontSize: 2000, FontFamily: "Times New Roman"}}},
			{SpaceBefore: 588, HasSpaceBefore: true, Runs: []*ir.Run{{Text: "02/78", FontSize: 1800, FontFamily: "Times New Roman"}}},
		}
	}
	shape := &ir.Shape{Bounds: ir.Rect{W: 12700 * 180, H: 12700 * 80}, Paragraphs: newBody()}
	plan := ResolveTextVisualPlan(shape)
	if len(plan.Paragraphs) != 2 {
		t.Fatalf("paragraph plans = %d, want 2", len(plan.Paragraphs))
	}
	if plan.Paragraphs[0].Metrics.HasSpaceBefore || plan.Paragraphs[0].Metrics.SpaceBeforePt != 0 {
		t.Fatalf("first paragraph space-before = %+v, want suppressed", plan.Paragraphs[0].Metrics)
	}
	if !plan.Paragraphs[1].Metrics.HasSpaceBefore || math.Abs(plan.Paragraphs[1].Metrics.SpaceBeforePt-5.88) > .001 {
		t.Fatalf("following paragraph space-before = %.3f, want authored 5.88", plan.Paragraphs[1].Metrics.SpaceBeforePt)
	}
	// Frame measurement must consume the same suppressed value.
	withLead := physicalParagraphLinesHeight(plan.Paragraphs[0].Lines) + 5.88 + physicalParagraphLinesHeight(plan.Paragraphs[1].Lines)
	if math.Abs(plan.Layout.ContentHeightPt-withLead) > .001 {
		t.Fatalf("content height = %.3f, want %.3f", plan.Layout.ContentHeightPt, withLead)
	}
}

