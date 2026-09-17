package layout

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

func TestTextBoxVisualPlanOwnsGeometryAwareInsetsAndVerticalPlacement(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 1270000}}
	frame := TextFrameMetrics{InsetTopPt: 3.6, InsetRightPt: 7.2, InsetBottomPt: 3.6, InsetLeftPt: 7.2}
	layout := TextFrameLayout{Measured: true, ContentHeightPt: 20, VerticalOffsetPt: 8}
	visual := ShapeVisualPlan{Geometry: GeometryVisualPlan{ContentInsetTop: .20, ContentInsetLeft: .15}}
	got := ResolveTextBoxVisualPlan(shape, frame, layout, visual)
	// 1,270,000 EMU = 100pt. Geometry insets must win when larger.
	if math.Abs(got.InsetTopPt-20) > .001 || math.Abs(got.InsetLeftPt-15) > .001 {
		t.Fatalf("effective insets = %+v", got)
	}
	if got.ContentTopPt != 8 {
		t.Fatalf("content top = %.2f, want 8", got.ContentTopPt)
	}
}

func TestTextBoxVisualPlanOwnsAutogrowAndClipPolicy(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{H: 635000}, TextBodyProps: drawing.TextBodyProperties{AutofitType: "spAutoFit"}}
	frame := ResolveTextFrameMetrics(shape)
	measured := TextFrameLayout{Measured: true, ContentHeightPt: 80}
	got := ResolveTextBoxVisualPlan(shape, frame, measured, ShapeVisualPlan{})
	if !got.ContentAutoHeight || !got.OverflowVisible || got.ClipContent {
		t.Fatalf("autogrow box policy = %+v", got)
	}
	if got.MinOuterHeightPt <= drawing.EMUToPoints(shape.Bounds.H) {
		t.Fatalf("autogrow min height = %.2f, want greater than authored height", got.MinOuterHeightPt)
	}

	fixed := ResolveTextBoxVisualPlan(&ir.Shape{Bounds: ir.Rect{H: 635000}}, ResolveTextFrameMetrics(&ir.Shape{}), TextFrameLayout{}, ShapeVisualPlan{})
	if fixed.ContentAutoHeight || fixed.OverflowVisible || !fixed.ClipContent {
		t.Fatalf("fixed box policy = %+v", fixed)
	}
}

func TestTextVisualPlanOwnsParagraphPhysicalPlans(t *testing.T) {
	shape := &ir.Shape{
		Bounds:     ir.Rect{W: 2540000, H: 1270000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "Clock Data", FontSize: 1600, FontFamily: "Arial"}}}},
	}
	got := ResolveTextVisualPlan(shape)
	if len(got.Paragraphs) != 1 {
		t.Fatalf("paragraph plans = %d, want 1", len(got.Paragraphs))
	}
	if !got.Paragraphs[0].LinesMeasured || len(got.Paragraphs[0].Lines) == 0 {
		t.Fatalf("paragraph physical lines not materialized: %+v", got.Paragraphs[0])
	}
}

func TestTextBoxVisualPlanNoAutofitPreservesAuthoredFrameHeight(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{H: 1270000}, TextAllowOverflow: true}
	frame := ResolveTextFrameMetrics(shape)
	if !frame.AllowOverflow || frame.AutoGrow {
		t.Fatalf("no-autofit frame metrics = %+v", frame)
	}
	got := ResolveTextBoxVisualPlan(shape, frame, TextFrameLayout{Measured: true, ContentHeightPt: 20, VerticalOffsetPt: 72.8}, ShapeVisualPlan{})
	if got.ContentAutoHeight {
		t.Fatalf("no-autofit must preserve authored frame height: %+v", got)
	}
	if !got.OverflowVisible || got.ClipContent {
		t.Fatalf("no-autofit overflow policy = %+v", got)
	}
	if got.OuterHeightPt != 100 {
		t.Fatalf("outer height = %.2f, want authored 100pt", got.OuterHeightPt)
	}
}

func TestClass9VerticalSpAutoFitGrowsPhysicalProgressionWidth(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 635000, H: 1270000},
		TextBodyProps: drawing.TextBodyProperties{AutofitType: "spAutoFit", Vert: "vert"},
	}
	frame := ResolveTextFrameMetrics(shape)
	got := ResolveTextBoxVisualPlan(shape, frame, TextFrameLayout{Measured: true, ContentHeightPt: 80}, ShapeVisualPlan{})
	if !got.ContentAutoWidth || got.ContentAutoHeight {
		t.Fatalf("vertical spAutoFit growth axis = %+v", got)
	}
	if got.MinOuterWidthPt <= drawing.EMUToPoints(shape.Bounds.W) {
		t.Fatalf("vertical spAutoFit min width = %.2f, want greater than authored width", got.MinOuterWidthPt)
	}
	if got.MinOuterHeightPt != 0 {
		t.Fatalf("vertical spAutoFit must not grow height: %+v", got)
	}
}

func TestClass9VerticalAnchoringOffsetsPhysicalProgressionEdge(t *testing.T) {
	for _, tc := range []struct {
		vert string
		wantLeft, wantRight float64
	}{
		{vert: "vert", wantRight: 12},
		{vert: "vert270", wantLeft: 12},
	} {
		shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 1270000}, TextBodyProps: drawing.TextBodyProperties{Vert: tc.vert}}
		got := ResolveTextBoxVisualPlan(shape, ResolveTextFrameMetrics(shape), TextFrameLayout{Measured: true, VerticalOffsetPt: 12}, ShapeVisualPlan{})
		if got.ContentLeftPt != tc.wantLeft || got.ContentRightPt != tc.wantRight || got.ContentTopPt != 0 {
			t.Fatalf("%s progression placement = %+v", tc.vert, got)
		}
	}
}


func TestClass12FixedTextFrameOwnsClippingPolicy(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 635000}}
	frame := ResolveTextFrameMetrics(shape)
	got := ResolveTextBoxVisualPlan(shape, frame, TextFrameLayout{Measured: true, ContentHeightPt: 100}, ShapeVisualPlan{})
	if !got.ClipContent || got.OverflowVisible {
		t.Fatalf("fixed-frame clipping policy = %+v", got)
	}
}


func TestClass12CenteredOverflowKeepsSignedPhysicalAnchor(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 635000}}
	frame := ResolveTextFrameMetrics(shape)
	frame.VerticalJustify = "center"
	flow := ResolveTextFlowMetrics(shape, frame)
	paragraphs := []ParagraphVisualPlan{{
		LinesMeasured: true,
		Lines: []PhysicalTextLine{{HeightPt: 80}},
	}}
	layout := LayoutTextFrameFromParagraphPlansWithFlow(shape, frame, flow, paragraphs)
	if layout.OverflowPt <= 0 || layout.VerticalOffsetPt >= 0 {
		t.Fatalf("centered overflow layout = %+v, want positive overflow and negative offset", layout)
	}
	box := ResolveTextBoxVisualPlan(shape, frame, layout, ShapeVisualPlan{})
	if box.ContentTopPt != layout.VerticalOffsetPt {
		t.Fatalf("box top = %.2f, want signed layout offset %.2f", box.ContentTopPt, layout.VerticalOffsetPt)
	}
}

func TestClass12BottomAnchoredOverflowStartsBeforeFrame(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 635000}}
	frame := ResolveTextFrameMetrics(shape)
	frame.VerticalJustify = "flex-end"
	flow := ResolveTextFlowMetrics(shape, frame)
	paragraphs := []ParagraphVisualPlan{{
		LinesMeasured: true,
		Lines: []PhysicalTextLine{{HeightPt: 80}},
	}}
	layout := LayoutTextFrameFromParagraphPlansWithFlow(shape, frame, flow, paragraphs)
	if layout.OverflowPt <= 0 || layout.VerticalOffsetPt >= 0 {
		t.Fatalf("bottom overflow layout = %+v, want negative start offset", layout)
	}
	if diff := layout.VerticalOffsetPt + layout.OverflowPt; diff < -.001 || diff > .001 {
		t.Fatalf("bottom offset %.2f must equal -overflow %.2f", layout.VerticalOffsetPt, layout.OverflowPt)
	}
}

func TestClass12ClippingPolicyDoesNotMutateMeasuredLineModel(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 635000}}
	frame := ResolveTextFrameMetrics(shape)
	flow := ResolveTextFlowMetrics(shape, frame)
	paragraphs := []ParagraphVisualPlan{{
		LinesMeasured: true,
		Lines: []PhysicalTextLine{{HeightPt: 30}, {HeightPt: 30}, {HeightPt: 30}},
	}}
	layout := LayoutTextFrameFromParagraphPlansWithFlow(shape, frame, flow, paragraphs)
	box := ResolveTextBoxVisualPlan(shape, frame, layout, ShapeVisualPlan{})
	if !box.ClipContent || box.OverflowVisible {
		t.Fatalf("fixed frame clipping policy = %+v", box)
	}
	if len(paragraphs[0].Lines) != 3 || layout.ContentHeightPt != 90 {
		t.Fatalf("clipping altered physical line model: lines=%d layout=%+v", len(paragraphs[0].Lines), layout)
	}
}


func TestClass12NormAutofitScaleParticipatesInPhysicalWrapping(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 1270000, H: 1270000},
		TextBodyProps: drawing.TextBodyProperties{
			AutofitType: "normAutofit", AutofitFontScale: 50000,
		},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{
			Text: "AAAA AAAA", FontFamily: "Arial", FontSize: 2400,
		}}}},
	}
	plan := ResolveTextVisualPlan(shape)
	if plan.Frame.FontScale != .5 {
		t.Fatalf("font scale = %.3f, want .5", plan.Frame.FontScale)
	}
	if len(plan.Paragraphs) != 1 || !plan.Paragraphs[0].LinesMeasured {
		t.Fatalf("normAutofit did not materialize physical wrapping: %+v", plan.Paragraphs)
	}
}

func TestClass12VerticalOverflowUsesHorizontalProgressionAxis(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 635000, H: 1270000},
		TextBodyProps: drawing.TextBodyProperties{Vert: "vert", Anchor: "ctr"},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{
			Text: "A\nB\nC\nD", FontFamily: "Arial", FontSize: 2400,
		}}}},
	}
	plan := ResolveTextVisualPlan(shape)
	if !plan.Flow.Vertical {
		t.Fatal("expected vertical physical flow")
	}
	if !plan.Layout.Measured {
		t.Fatalf("vertical frame not measured: %+v", plan.Layout)
	}
	if plan.Layout.OverflowPt > 0 && plan.Box.ContentRightPt >= 0 {
		t.Fatalf("centered vertical overflow must move toward progression start: layout=%+v box=%+v", plan.Layout, plan.Box)
	}
}

func TestClass12NoWrapStillMeasuresAuthoredMultipleLinesForOverflow(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 1270000, H: 381000},
		TextBodyProps: drawing.TextBodyProperties{Wrap: "none"},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{
			Text: "first\nsecond\nthird", FontFamily: "Arial", FontSize: 1800,
		}}}},
	}
	plan := ResolveTextVisualPlan(shape)
	if !plan.Flow.NoWrap || len(plan.Paragraphs[0].Lines) != 3 {
		t.Fatalf("noWrap authored lines = %+v", plan.Paragraphs)
	}
	if !plan.Layout.Measured || plan.Layout.ContentHeightPt <= 0 {
		t.Fatalf("noWrap authored lines not included in frame measurement: %+v", plan.Layout)
	}
}
