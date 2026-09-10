package layout

import (
	"math"
	"strings"
	"testing"

	resolve "office/pkg/word/ir"
)

func fidelityParagraph(text string) *resolve.Paragraph {
	return &resolve.Paragraph{
		Text: text,
		Runs: []resolve.Run{{Kind: "run", Text: text}},
	}
}

func fidelityLineText(runs []resolve.Run) string {
	var b strings.Builder
	for _, run := range runs {
		b.WriteString(run.Text)
	}
	return strings.TrimSpace(b.String())
}

func TestPhysicalLineFlowUsesBothSidesBeforeAdvancingVertically(t *testing.T) {
	p := fidelityParagraph("a b")

	// Each slot can hold one token, but not both. Crossing from slot 0 to
	// slot 1 must remain on the same physical line.
	plan := paragraphLinePlan{Bands: []lineBand{
		{
			Top: 0, Bottom: 12,
			Spans: []lineSpan{
				{Left: 0, Right: 10},
				{Left: 90, Right: 100},
			},
		},
	}}

	flow := breakParagraphAcrossSlots(p, plan.SlotPlan(), 11)

	if len(flow.Lines) != 1 {
		t.Fatalf("physical lines = %d, want 1", len(flow.Lines))
	}
	if len(flow.Lines[0].Slots) != 2 {
		t.Fatalf("slots = %d, want 2", len(flow.Lines[0].Slots))
	}

	if got := fidelityLineText(flow.Lines[0].Slots[0].Runs); got != "a" {
		t.Fatalf("left slot text = %q, want a", got)
	}
	if got := fidelityLineText(flow.Lines[0].Slots[1].Runs); got != "b" {
		t.Fatalf("right slot text = %q, want b", got)
	}

	if len(flow.LineMetrics()) != 1 {
		t.Fatalf("line metrics = %d, want 1 physical metric", len(flow.LineMetrics()))
	}
}

func TestPhysicalLineFlowAdvancesOnlyAfterAllSlotsAreFull(t *testing.T) {
	p := fidelityParagraph("one two three four five six seven eight")

	plan := paragraphLinePlan{Bands: []lineBand{
		{
			Top: 0, Bottom: 12,
			Spans: []lineSpan{
				{Left: 0, Right: 30},
				{Left: 70, Right: 100},
			},
		},
		{
			Top: 12, Bottom: 24,
			Spans: []lineSpan{{Left: 0, Right: 100}},
		},
	}}

	flow := breakParagraphAcrossSlots(p, plan.SlotPlan(), 11)

	if len(flow.LineRuns()) < 2 {
		t.Fatalf("physical lines = %d, want continuation line", len(flow.LineRuns()))
	}

	first := fidelityLineText(flow.LineRuns()[0])
	if first == "" {
		t.Fatal("first physical line is empty")
	}

	all := ""
	for _, line := range flow.LineRuns() {
		all += " " + fidelityLineText(line)
	}
	for _, word := range []string{
		"one", "two", "three", "four",
		"five", "six", "seven", "eight",
	} {
		if !strings.Contains(all, word) {
			t.Fatalf("paragraph flow lost %q: %q", word, all)
		}
	}
}

func TestPhysicalLineFlowContinuesBeyondProvisionalBands(t *testing.T) {
	p := fidelityParagraph(
		"one two three four five six seven eight nine ten eleven twelve",
	)

	// Only one provisional band. The breaker must extend vertically rather
	// than dropping remaining tokens.
	plan := paragraphLinePlan{Bands: []lineBand{
		{
			Top: 0, Bottom: 12,
			Spans: []lineSpan{{Left: 0, Right: 35}},
		},
	}}

	flow := breakParagraphAcrossSlots(p, plan.SlotPlan(), 11)

	if len(flow.LineRuns()) <= 1 {
		t.Fatalf("physical lines = %d, want dynamic continuation", len(flow.LineRuns()))
	}

	var text strings.Builder
	for _, line := range flow.LineRuns() {
		text.WriteByte(' ')
		text.WriteString(fidelityLineText(line))
	}

	for _, word := range []string{
		"one", "six", "twelve",
	} {
		if !strings.Contains(text.String(), word) {
			t.Fatalf("dynamic continuation lost %q: %q", word, text.String())
		}
	}

	if len(flow.LineMetrics()) != len(flow.LineRuns()) {
		t.Fatalf(
			"metrics=%d runs=%d; physical ownership diverged",
			len(flow.LineMetrics()), len(flow.LineRuns()),
		)
	}
}

func TestPhysicalLineMetricsMergeAcrossBothSlots(t *testing.T) {
	p := &resolve.Paragraph{
		Runs: []resolve.Run{
			{Kind: "run", Text: "left"},
			{
				Kind: "image",
				Image: &resolve.Image{
					Width:  12700,
					Height: 381000, // 30pt
				},
			},
		},
	}

	plan := paragraphLinePlan{Bands: []lineBand{
		{
			Top: 0, Bottom: 12,
			Spans: []lineSpan{
				{Left: 0, Right: 30},
				{Left: 70, Right: 100},
			},
		},
	}}

	flow := breakParagraphAcrossSlots(p, plan.SlotPlan(), 11)

	if len(flow.LineMetrics()) != 1 {
		t.Fatalf("metrics = %d, want one physical-line metric", len(flow.LineMetrics()))
	}

	if got := flow.LineMetrics()[0].maxInlineHeightPt; got < 29.9 || got > 30.1 {
		t.Fatalf("inline image height = %.3f, want 30pt", got)
	}
}

func TestPhysicalLineMetricsUseTallestInlineObjectOnlyOnce(t *testing.T) {
	p := &resolve.Paragraph{
		Runs: []resolve.Run{
			{
				Kind: "image",
				Image: &resolve.Image{
					Width:  12700,
					Height: 254000, // 20pt
				},
			},
			{
				Kind: "image",
				Image: &resolve.Image{
					Width:  12700,
					Height: 381000, // 30pt
				},
			},
		},
	}

	plan := paragraphLinePlan{Bands: []lineBand{
		{
			Top: 0, Bottom: 12,
			Spans: []lineSpan{
				{Left: 0, Right: 20},
				{Left: 80, Right: 100},
			},
		},
	}}

	flow := breakParagraphAcrossSlots(p, plan.SlotPlan(), 11)

	if len(flow.LineMetrics()) != 1 {
		t.Fatalf("metrics = %d, want one physical line", len(flow.LineMetrics()))
	}

	// Physical-line height is max(), never sum(slot heights).
	got := flow.LineMetrics()[0].maxInlineHeightPt
	if got < 29.9 || got > 30.1 {
		t.Fatalf("inline height = %.3f, want tallest object = 30pt", got)
	}
}

func TestPhysicalLinePaginationRangeNeverSplitsSiblingSlots(t *testing.T) {
	plan := paragraphLinePlan{Bands: []lineBand{
		{
			Top: 0, Bottom: 12,
			Spans: []lineSpan{
				{Left: 0, Right: 30},
				{Left: 70, Right: 100},
			},
		},
		{
			Top: 12, Bottom: 24,
			Spans: []lineSpan{{Left: 0, Right: 100}},
		},
	}}

	slots := plan.SlotPlan()

	start, end, ok := slots.SlotRangeForPhysicalLines(0, 1)
	if !ok {
		t.Fatal("physical first line has no slot range")
	}
	if start != 0 || end != 2 {
		t.Fatalf(
			"first physical line owns slots %d..%d, want 0..2",
			start, end,
		)
	}

	lineStart, lineEnd, ok := slots.PhysicalLineRange(start, end)
	if !ok || lineStart != 0 || lineEnd != 1 {
		t.Fatalf(
			"slot range maps to physical lines %d..%d, ok=%v; want 0..1",
			lineStart, lineEnd, ok,
		)
	}

	if got := plan.PhysicalHeight(start, end); got != 12 {
		t.Fatalf("first physical line height = %.3f, want 12", got)
	}
}

func TestPhysicalLineFlowHonorsExplicitLineBreakBeforeUsingSiblingSlot(t *testing.T) {
	p := &resolve.Paragraph{
		Runs: []resolve.Run{
			{Kind: "run", Text: "left"},
			{Kind: "break", BreakType: "line"},
			{Kind: "run", Text: "next"},
		},
	}
	plan := paragraphLinePlan{Bands: []lineBand{
		{Top: 0, Bottom: 12, Spans: []lineSpan{{Left: 0, Right: 30}, {Left: 70, Right: 100}}},
		{Top: 12, Bottom: 24, Spans: []lineSpan{{Left: 0, Right: 100}}},
	}}

	flow := breakParagraphAcrossSlots(p, plan.SlotPlan(), 11)
	if len(flow.Lines) != 2 {
		t.Fatalf("physical lines = %d, want 2", len(flow.Lines))
	}
	if got := fidelityLineText(flow.Lines[0].Runs()); got != "left" {
		t.Fatalf("first line text = %q, want left", got)
	}
	if got := fidelityLineText(flow.Lines[1].Runs()); got != "next" {
		t.Fatalf("second line text = %q, want next", got)
	}
	if len(flow.Lines[0].Slots) != 1 {
		t.Fatalf("explicit break consumed sibling slot: slots = %d, want 1", len(flow.Lines[0].Slots))
	}
	firstRuns := flow.Lines[0].Runs()
	if len(firstRuns) != 1 || firstRuns[0].Kind != "run" {
		t.Fatalf("first line runs = %#v, want consumed layout break", firstRuns)
	}
}

func TestPhysicalLineFlowConsumesTextWrappingBreakForSingleSpanFlow(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{
		{Kind: "run", Text: "before"},
		{Kind: "break", BreakType: "textWrapping"},
		{Kind: "run", Text: "after"},
	}}
	plan := paragraphLinePlan{Bands: []lineBand{
		{Top: 0, Bottom: 12, Spans: []lineSpan{{Left: 0, Right: 100}}},
		{Top: 12, Bottom: 24, Spans: []lineSpan{{Left: 0, Right: 100}}},
	}}
	flow := breakParagraphAcrossSlots(p, plan.SlotPlan(), 11)
	if len(flow.Lines) != 2 {
		t.Fatalf("physical lines = %d, want 2", len(flow.Lines))
	}
	if got := fidelityLineText(flow.Lines[0].Runs()); got != "before" {
		t.Fatalf("first line text = %q, want before", got)
	}
	if got := fidelityLineText(flow.Lines[1].Runs()); got != "after" {
		t.Fatalf("second line text = %q, want after", got)
	}
}

func TestSpatialTokenizerDoesNotInventSpaceAcrossRunBoundary(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{
		{Kind: "run", Text: "Hello"},
		{Kind: "run", Text: "World"},
	}}
	tokens := paragraphSpatialRenderTokens(p, 11)
	var got strings.Builder
	for _, token := range tokens {
		if token.separated {
			got.WriteByte(' ')
		}
		got.WriteString(token.run.Text)
	}
	if got.String() != "HelloWorld" {
		t.Fatalf("spatial token text = %q, want %q", got.String(), "HelloWorld")
	}
}

func TestSpatialTokenizerPreservesWhitespaceAcrossRunBoundary(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{
		{Kind: "run", Text: "Hello "},
		{Kind: "run", Text: "World"},
	}}
	tokens := paragraphSpatialRenderTokens(p, 11)
	var got strings.Builder
	for _, token := range tokens {
		if token.separated {
			got.WriteByte(' ')
		}
		got.WriteString(token.run.Text)
	}
	if got.String() != "Hello World" {
		t.Fatalf("spatial token text = %q, want %q", got.String(), "Hello World")
	}
}

func TestParagraphAffectedByActiveWrapFollowsFloatBeyondAnchorParagraph(t *testing.T) {
	geom := PageGeometry{
		Width: 612, Height: 792,
		MarginLeft: 72, MarginRight: 72,
		ColumnCount: 1, ColumnWidthPt: 468,
	}
	fl := FloatBox{
		Image: &resolve.Image{Floating: true, WrapType: "square"},
		X:     200, Y: 100, Width: 100, Height: 120,
	}
	if !paragraphAffectedByActiveWrap(geom, 0, 150, 20, []FloatBox{fl}) {
		t.Fatal("paragraph overlapping an earlier square float must inherit physical wrap ownership")
	}
	if paragraphAffectedByActiveWrap(geom, 0, 230, 20, []FloatBox{fl}) {
		t.Fatal("paragraph below the float must remain browser-owned")
	}
}

func TestParagraphAffectedByActiveWrapIgnoresBehindTextDrawing(t *testing.T) {
	geom := PageGeometry{
		Width: 612, Height: 792,
		MarginLeft: 72, MarginRight: 72,
		ColumnCount: 1, ColumnWidthPt: 468,
	}
	fl := FloatBox{
		Image: &resolve.Image{Floating: true, WrapType: "square", BehindDoc: true},
		X:     200, Y: 100, Width: 100, Height: 120,
		BehindDoc: true,
	}
	if paragraphAffectedByActiveWrap(geom, 0, 150, 20, []FloatBox{fl}) {
		t.Fatal("behind-text drawing must not take physical wrap ownership")
	}
}

func TestSpatialPhysicalFlowUsesSourceRunBoundaryWhitespace(t *testing.T) {
	plan := paragraphSlotPlan{
		PhysicalLineCount: 1,
		Slots: []lineSlot{
			{BandIndex: 0, SlotIndex: 0, Span: lineSpan{Left: 0, Right: 200}},
			{BandIndex: 0, SlotIndex: 1, Span: lineSpan{Left: 300, Right: 500}},
		},
	}
	for _, tc := range []struct {
		name string
		runs []resolve.Run
		want string
	}{
		{"joined", []resolve.Run{{Kind: "run", Text: "Hello"}, {Kind: "run", Text: "World"}}, "HelloWorld"},
		{"separated", []resolve.Run{{Kind: "run", Text: "Hello "}, {Kind: "run", Text: "World"}}, "Hello World"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &resolve.Paragraph{Runs: tc.runs}
			flow := breakParagraphAcrossSlots(p, plan, 11)
			got := paragraphTextFromRuns(paragraphPhysicalLinesRuns(flow.Lines))
			if got != tc.want {
				t.Fatalf("physical text = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParagraphTabAdvanceUsesNextCustomStop(t *testing.T) {
	p := &resolve.Paragraph{}
	p.Format.TabStops = []resolve.TabStop{
		{Pos: 720, Algn: "left"},
		{Pos: 1440, Algn: "left"},
	}
	if got := paragraphTabAdvancePt(p, 10, paragraphFlowContext{}); math.Abs(got-26) > 0.01 {
		t.Fatalf("advance = %.2fpt, want 26pt", got)
	}
	if got := paragraphTabAdvancePt(p, 40, paragraphFlowContext{}); math.Abs(got-32) > 0.01 {
		t.Fatalf("advance = %.2fpt, want 32pt", got)
	}
}

func TestParagraphTabAdvanceFallsBackToWordDefaultGrid(t *testing.T) {
	p := &resolve.Paragraph{}
	if got := paragraphTabAdvancePt(p, 10, paragraphFlowContext{}); math.Abs(got-26) > 0.01 {
		t.Fatalf("advance = %.2fpt, want 26pt", got)
	}
	if got := paragraphTabAdvancePt(p, 36, paragraphFlowContext{}); math.Abs(got-36) > 0.01 {
		t.Fatalf("advance = %.2fpt, want 36pt", got)
	}
}

func TestParagraphFlowContextUsesDocumentDefaultTabStop(t *testing.T) {
	doc := &resolve.Document{DefaultTabStop: 480}
	ctx := paragraphFlowContextForDocument(doc)
	if ctx.DefaultTabStopTwips != 480 {
		t.Fatalf("default tab stop = %d, want 480", ctx.DefaultTabStopTwips)
	}
	p := &resolve.Paragraph{}
	if got := paragraphTabAdvancePt(p, 10, ctx); math.Abs(got-14) > 0.01 {
		t.Fatalf("advance = %.2fpt, want 14pt", got)
	}
}

func TestParagraphTabAdvanceUsesNearestCustomStopRegardlessOfOrder(t *testing.T) {
	p := &resolve.Paragraph{}
	p.Format.TabStops = []resolve.TabStop{
		{Pos: 1440, Algn: "left"},
		{Pos: 720, Algn: "left"},
	}
	if got := paragraphTabAdvancePt(p, 10, paragraphFlowContext{}); math.Abs(got-26) > 0.01 {
		t.Fatalf("advance = %.2fpt, want nearest custom stop at 36pt", got)
	}
}

func TestAlignedTabAdvanceUsesFollowingFieldWidth(t *testing.T) {
	p := &resolve.Paragraph{}
	p.Format.TabStops = []resolve.TabStop{{Pos: 1440, Algn: "right"}}
	tokens := []paragraphRenderToken{
		{tab: true},
		{run: resolve.Run{Kind: "run", Text: "AB"}, widthPt: 20, fontSizePt: 10},
	}
	got := alignedTabAdvancePt(p, tokens, 0, 10, paragraphFlowContext{}, 10)
	if math.Abs(got-42) > 0.01 {
		t.Fatalf("right-tab advance = %.2fpt, want 42pt", got)
	}

	p.Format.TabStops[0].Algn = "center"
	got = alignedTabAdvancePt(p, tokens, 0, 10, paragraphFlowContext{}, 10)
	if math.Abs(got-52) > 0.01 {
		t.Fatalf("center-tab advance = %.2fpt, want 52pt", got)
	}
}

func TestDecimalTabAlignsDecimalSeparator(t *testing.T) {
	p := &resolve.Paragraph{}
	p.Format.TabStops = []resolve.TabStop{{Pos: 1440, Algn: "decimal"}}
	run := resolve.Run{Kind: "run", Text: "12.34"}
	fullWidth, fontSize := runWordWidthPt(run.Format, run.Text, 10)
	tokens := []paragraphRenderToken{
		{tab: true},
		{run: run, widthPt: fullWidth, fontSizePt: fontSize},
	}
	prefixWidth, _ := runWordWidthPt(run.Format, "12", 10)
	got := alignedTabAdvancePt(p, tokens, 0, 10, paragraphFlowContext{}, 10)
	want := 72 - 10 - prefixWidth
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("decimal-tab advance = %.2fpt, want %.2fpt", got, want)
	}
}

func TestParagraphFragmentNeedsSpatialRenderingUsesCanonicalOwnership(t *testing.T) {
	ordinary := &ParagraphFragment{Lines: []ParagraphLineLayout{
		{Slots: []ParagraphSlotLayout{{Left: 72, Right: 540, Top: 72, Bottom: 84}}},
	}}
	if paragraphFragmentNeedsSpatialRendering(ordinary) {
		t.Fatal("ordinary fragment must remain semantic/browser-owned")
	}

	spatial := &ParagraphFragment{
		SpatialRendering: true,
		Lines: []ParagraphLineLayout{{Slots: []ParagraphSlotLayout{
			{Left: 300, Right: 540, Top: 72, Bottom: 84},
		}}},
	}
	if !paragraphFragmentNeedsSpatialRendering(spatial) {
		t.Fatal("canonical one-sided float geometry must transfer rendering ownership")
	}
}

func TestFragmentPhysicalLinesDoNotCarryPriorPageSlots(t *testing.T) {
	first := paragraphFlowMetrics{
		lineHeights: []float64{12, 12, 12},
		physicalLines: []paragraphPhysicalLine{
			{Slots: []paragraphSlotRun{{Slot: lineSlot{Span: lineSpan{Left: 0, Right: 80}}}, {Slot: lineSlot{Span: lineSpan{Left: 120, Right: 200}}}}},
			{Slots: []paragraphSlotRun{{Slot: lineSlot{Span: lineSpan{Left: 0, Right: 80}}}, {Slot: lineSlot{Span: lineSpan{Left: 120, Right: 200}}}}},
			{Slots: []paragraphSlotRun{{Slot: lineSlot{Span: lineSpan{Left: 0, Right: 200}}}}},
		},
	}
	continuation := []paragraphPhysicalLine{
		{Slots: []paragraphSlotRun{{Slot: lineSlot{Span: lineSpan{Left: 0, Right: 200}}}}},
		{Slots: []paragraphSlotRun{{Slot: lineSlot{Span: lineSpan{Left: 0, Right: 200}}}}},
	}
	first.physicalLines = append(first.physicalLines[:1], continuation...)
	got := first.fragmentPhysicalLines(1, 3)
	if len(got) != 2 {
		t.Fatalf("continuation lines = %d, want 2", len(got))
	}
	for i, line := range got {
		if len(line.Slots) != 1 {
			t.Fatalf("continuation line %d slots = %d, want page-local one-slot geometry", i, len(line.Slots))
		}
	}
}

func TestContinuationReflowUsesOnlyRemainingPhysicalRuns(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{
		{Kind: "run", Text: "first"},
		{Kind: "run", Text: "second"},
	}}
	metrics := paragraphFlowMetrics{
		lineHeights: []float64{12, 12},
		physicalLines: []paragraphPhysicalLine{
			{Slots: []paragraphSlotRun{{Runs: []resolve.Run{{Kind: "run", Text: "first"}}}}},
			{Slots: []paragraphSlotRun{{Runs: []resolve.Run{{Kind: "run", Text: "second"}}}}},
		},
	}
	remaining := paragraphPhysicalLinesRuns(metrics.fragmentPhysicalLines(1, len(metrics.lineHeights)))
	if got := paragraphTextFromRuns(remaining); got != "second" {
		t.Fatalf("remaining text = %q, want %q", got, "second")
	}
	continuation := *p
	continuation.Runs = remaining
	continuation.Text = paragraphTextFromRuns(remaining)
	if continuation.Text != "second" {
		t.Fatalf("continuation text = %q, want consumed prefix removed", continuation.Text)
	}
}

func TestTableRowHeightsCanBeRecomputedForContinuationWidth(t *testing.T) {
	table := &resolve.Table{
		Rows: []resolve.TableRow{
			{Cells: []resolve.TableCell{{Blocks: []resolve.Block{{Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "one two three four five six seven eight"}}}}}}}},
			{Cells: []resolve.TableCell{{Blocks: []resolve.Block{{Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "continuation row"}}}}}}}},
		},
	}
	wide := TableRowHeights(nil, table, 300)
	narrow := TableRowHeights(nil, table, 80)
	if len(wide) != 2 || len(narrow) != 2 {
		t.Fatalf("row heights = %d/%d, want 2/2", len(wide), len(narrow))
	}
	if narrow[0] < wide[0] {
		t.Fatalf("narrow row height %.2f < wide row height %.2f", narrow[0], wide[0])
	}
}

func TestActiveWrapLifetimeUsesTransformedDrawingBounds(t *testing.T) {
	geom := PageGeometry{
		Width: 612, Height: 792,
		MarginLeft: 72, MarginRight: 72,
		ColumnCount: 1, ColumnWidthPt: 468,
	}
	fl := FloatBox{
		Image: &resolve.Image{Floating: true, WrapType: "square", RotationDeg: 90},
		X:     200, Y: 100, Width: 120, Height: 40,
	}
	// A 90-degree rotation about the center expands the vertical wrap lifetime
	// from authored 100..140 to transformed 60..180.
	if !paragraphAffectedByActiveWrap(geom, 0, 155, 15, []FloatBox{fl}) {
		t.Fatal("rotated drawing must keep wrap ownership through transformed bottom bound")
	}
	if paragraphAffectedByActiveWrap(geom, 0, 185, 15, []FloatBox{fl}) {
		t.Fatal("drawing must release wrap ownership below transformed exclusion")
	}
}

func TestActiveWrapLifetimeIsColumnLocal(t *testing.T) {
	geom := PageGeometry{
		Width: 612, Height: 792,
		MarginLeft: 72, MarginRight: 72,
		ColumnCount: 2, ColumnWidthPt: 216, ColumnGapPt: 36,
	}
	fl := FloatBox{
		Image: &resolve.Image{Floating: true, WrapType: "square"},
		X:     80, Y: 100, Width: 100, Height: 100,
	}
	if !paragraphAffectedByActiveWrap(geom, 0, 120, 20, []FloatBox{fl}) {
		t.Fatal("float intersecting first column must remain active there")
	}
	if paragraphAffectedByActiveWrap(geom, 1, 120, 20, []FloatBox{fl}) {
		t.Fatal("first-column float must not constrain second-column paragraph")
	}
}
