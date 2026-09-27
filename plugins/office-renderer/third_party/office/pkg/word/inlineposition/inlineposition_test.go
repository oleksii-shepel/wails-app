package inlineposition

import (
	"math"
	"testing"

	resolve "office/pkg/word/ir"
)

func TestNextTabStopAndAlignedAdvance(t *testing.T) {
	stops := []resolve.TabStop{{Pos: 1440, Algn: "right", Leader: "dot"}}
	stop := NextTabStop(stops, 20, 720)
	if stop.PositionPt != 72 || stop.Alignment != "right" || stop.Leader != "dot" {
		t.Fatalf("stop = %+v", stop)
	}
	if got := AlignedTabAdvance(stop, 20, 10, 4); math.Abs(got-42) > 0.001 {
		t.Fatalf("right advance = %.3f, want 42", got)
	}
	stop.Alignment = "center"
	if got := AlignedTabAdvance(stop, 20, 10, 4); math.Abs(got-47) > 0.001 {
		t.Fatalf("center advance = %.3f, want 47", got)
	}
	stop.Alignment = "decimal"
	if got := AlignedTabAdvance(stop, 20, 10, 4); math.Abs(got-48) > 0.001 {
		t.Fatalf("decimal advance = %.3f, want 48", got)
	}
}

func TestNextTabStopUsesDefaultGrid(t *testing.T) {
	stop := NextTabStop(nil, 40, 720)
	if math.Abs(stop.PositionPt-72) > 0.001 {
		t.Fatalf("position = %.3f, want 72", stop.PositionPt)
	}
}

func TestAdvanceStateRelativeAndAbsoluteOffsets(t *testing.T) {
	s := AdvanceState{InlineXPt: 30}
	s.Apply(ParseAdvance(`ADVANCE \r 12 \u 3`))
	if s.HorizontalPt != 12 || s.VerticalPt != 3 || s.InlineXPt != 42 {
		t.Fatalf("relative state = %+v", s)
	}
	s.Apply(ParseAdvance(`ADVANCE \x 60 \y 8`))
	if s.HorizontalPt != 18 || s.VerticalPt != -8 || s.InlineXPt != 60 {
		t.Fatalf("absolute state = %+v", s)
	}
}
