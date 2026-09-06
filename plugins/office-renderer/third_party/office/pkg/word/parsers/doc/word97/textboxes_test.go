package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

func TestMaterializeTextboxesUsesAbsoluteRangeAndAnchor(t *testing.T) {
	ranges := []docbinary.TextboxRange{{StartCP: 100, EndCP: 108, ShapeID: 7}}
	anchors := map[uint32]docbinary.FloatingAnchor{
		7: {Shape: 7, Left: 10, Top: 20, Right: 110, Bottom: 220},
	}
	var gotStart, gotEnd int
	resolved := MaterializeTextboxes(ranges, anchors, func(startCP, endCP int) wordir.Story {
		gotStart, gotEnd = startCP, endCP
		p := wordir.Paragraph{Runs: []wordir.Run{{Text: "hello"}}}
		return wordir.Story{Paragraphs: []wordir.Paragraph{p}, Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	})
	if gotStart != 100 || gotEnd != 108 {
		t.Fatalf("materialized range = %d..%d, want 100..108", gotStart, gotEnd)
	}
	if len(resolved) != 1 || resolved[0].TextBox == nil {
		t.Fatalf("resolved = %#v", resolved)
	}
	tb := resolved[0].TextBox
	if tb.LegacyShapeID != 7 || !tb.Floating || tb.WrapType != "square" {
		t.Fatalf("textbox = %#v", tb)
	}
	if tb.X == 0 && tb.Y == 0 && tb.Width == 0 && tb.Height == 0 {
		t.Fatalf("anchor placement was not applied: %#v", tb)
	}
}

func TestMaterializeTextboxesSkipsEmptyStories(t *testing.T) {
	resolved := MaterializeTextboxes(
		[]docbinary.TextboxRange{{StartCP: 1, EndCP: 2, ShapeID: 3}},
		nil,
		func(int, int) wordir.Story { return wordir.Story{} },
	)
	if resolved != nil {
		t.Fatalf("resolved = %#v, want nil", resolved)
	}
}
