package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestParagraphSlotPlanPreservesBothSidesOfFloatOnOnePhysicalLine(t *testing.T) {
	plan := paragraphLinePlan{Bands: []lineBand{
		{Top: 10, Bottom: 22, Spans: []lineSpan{{Left: 0, Right: 30}, {Left: 70, Right: 100}}},
	}}

	slots := plan.SlotPlan()
	if slots.PhysicalLineCount != 1 {
		t.Fatalf("PhysicalLineCount = %d, want 1", slots.PhysicalLineCount)
	}
	if len(slots.Slots) != 2 {
		t.Fatalf("slots = %d, want 2", len(slots.Slots))
	}
	if slots.Slots[0].BandIndex != 0 || slots.Slots[1].BandIndex != 0 {
		t.Fatalf("band indexes = %d,%d, want 0,0", slots.Slots[0].BandIndex, slots.Slots[1].BandIndex)
	}
	if slots.Slots[0].Width() != 30 || slots.Slots[1].Width() != 30 {
		t.Fatalf("slot widths = %.3f,%.3f, want 30,30",
			slots.Slots[0].Width(), slots.Slots[1].Width())
	}
}

func TestParagraphSlotPlanDoesNotCreatePhantomVerticalHeight(t *testing.T) {
	plan := paragraphLinePlan{Bands: []lineBand{
		{Top: 0, Bottom: 12, Spans: []lineSpan{{Left: 0, Right: 20}, {Left: 80, Right: 100}}},
		{Top: 12, Bottom: 24, Spans: []lineSpan{{Left: 0, Right: 100}}},
	}}

	if got := plan.PhysicalHeight(0, 2); got != 12 {
		t.Fatalf("height of two slots on first physical line = %v, want 12", got)
	}
	if got := plan.PhysicalHeight(0, 3); got != 24 {
		t.Fatalf("height through second physical line = %v, want 24", got)
	}
}

func TestParagraphSlotPlanMapsPaginationLineRangeBackToSlots(t *testing.T) {
	plan := paragraphLinePlan{Bands: []lineBand{
		{Top: 0, Bottom: 10, Spans: []lineSpan{{Left: 0, Right: 25}, {Left: 75, Right: 100}}},
		{Top: 10, Bottom: 20, Spans: []lineSpan{{Left: 0, Right: 100}}},
		{Top: 20, Bottom: 30, Spans: []lineSpan{{Left: 0, Right: 40}, {Left: 60, Right: 100}}},
	}}

	slots := plan.SlotPlan()
	start, end, ok := slots.SlotRangeForPhysicalLines(1, 3)
	if !ok {
		t.Fatal("expected slot range")
	}
	if start != 2 || end != 5 {
		t.Fatalf("slot range = %d..%d, want 2..5", start, end)
	}
	firstLine, lastLine, ok := slots.PhysicalLineRange(start, end)
	if !ok || firstLine != 1 || lastLine != 3 {
		t.Fatalf("physical range = %d..%d, %v, want 1..3, true", firstLine, lastLine, ok)
	}
}

func TestParagraphSlotPlanSkipsBlockedBandButKeepsPhysicalOrdinal(t *testing.T) {
	plan := paragraphLinePlan{Bands: []lineBand{
		{Top: 0, Bottom: 10, Spans: nil},
		{Top: 10, Bottom: 20, Spans: []lineSpan{{Left: 0, Right: 100}}},
	}}

	slots := plan.SlotPlan()
	if slots.PhysicalLineCount != 2 {
		t.Fatalf("PhysicalLineCount = %d, want 2", slots.PhysicalLineCount)
	}
	if len(slots.Slots) != 1 || slots.Slots[0].BandIndex != 1 {
		t.Fatalf("slots = %#v, want one slot on physical line 1", slots.Slots)
	}
}

func TestParagraphPhysicalRunLinesFoldsTwoSlotsIntoOneLine(t *testing.T) {
	p := &resolve.Paragraph{
		Text: "one two three four",
		Runs: []resolve.Run{{Kind: "run", Text: "one two three four"}},
	}
	plan := paragraphLinePlan{Bands: []lineBand{
		{Top: 0, Bottom: 12, Spans: []lineSpan{{Left: 0, Right: 30}, {Left: 70, Right: 100}}},
		{Top: 12, Bottom: 24, Spans: []lineSpan{{Left: 0, Right: 100}}},
	}}

	lines := paragraphPhysicalRunLines(p, plan, 11)
	if len(lines) != 2 {
		t.Fatalf("physical lines = %d, want 2", len(lines))
	}
	if got := paragraphTextFromRuns(lines[0]); got == "" {
		t.Fatal("first physical line lost both float-side slots")
	}
	if got := paragraphTextFromRuns(append(append([]resolve.Run{}, lines[0]...), lines[1]...)); got == "" {
		t.Fatal("physical line folding lost paragraph content")
	}
}

func TestParagraphPhysicalRunLinesKeepsBlockedLineOrdinal(t *testing.T) {
	p := &resolve.Paragraph{Text: "hello", Runs: []resolve.Run{{Kind: "run", Text: "hello"}}}
	plan := paragraphLinePlan{Bands: []lineBand{
		{Top: 0, Bottom: 10, Spans: nil},
		{Top: 10, Bottom: 20, Spans: []lineSpan{{Left: 0, Right: 100}}},
	}}
	lines := paragraphPhysicalRunLines(p, plan, 11)
	if len(lines) != 2 {
		t.Fatalf("physical lines = %d, want 2", len(lines))
	}
	if len(lines[0]) != 0 {
		t.Fatalf("blocked physical line contains %d runs", len(lines[0]))
	}
	if got := paragraphTextFromRuns(lines[1]); got != "hello" {
		t.Fatalf("second physical line text = %q, want hello", got)
	}
}

func TestParagraphLinePlanForFlowOwnsFirstAndContinuationGeometry(t *testing.T) {
	plan := paragraphLinePlanForFlow(20, 80, 10, 100, 30, []float64{12, 14}, 12, nil)
	if len(plan.Bands) != 2 {
		t.Fatalf("bands = %d, want 2", len(plan.Bands))
	}
	if got := plan.Bands[0].Spans[0]; got.Left != 20 || got.Right != 100 {
		t.Fatalf("first span = %#v, want 20..100", got)
	}
	if got := plan.Bands[1].Spans[0]; got.Left != 10 || got.Right != 110 {
		t.Fatalf("continuation span = %#v, want 10..110", got)
	}
	if plan.Bands[0].Top != 30 || plan.Bands[0].Bottom != 42 || plan.Bands[1].Bottom != 56 {
		t.Fatalf("vertical bands = %#v", plan.Bands)
	}
}
