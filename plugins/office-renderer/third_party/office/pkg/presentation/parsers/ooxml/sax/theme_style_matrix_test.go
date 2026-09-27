package sax

import (
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestIRBuilderMaterializesThemeLineStylePropertiesForLineRef(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventThemeColor, ThemeSlot: "accent2", Color: VisualColor{Kind: "srgbClr", Value: "CC6600"}})
	must(Event{Type: EventThemeLineStyle, StyleIndex: 2, Line: LineProperties{WidthEMU: 38100, Color: VisualColor{Kind: "schemeClr", Value: "accent2"}, Cap: "round", Join: "bevel", Dash: "dashDot", Compound: "dbl", HeadType: "triangle", HeadWidth: "lg", HeadLength: "sm", TailType: "oval", TailWidth: "sm", TailLength: "lg"}})
	must(Event{Type: EventStartShape, ShapeID: 7, ShapeKind: "shape"})
	must(Event{Type: EventLineStyleRef, ShapeID: 7, StyleIndex: 2, Color: VisualColor{Kind: "schemeClr", Value: "phClr"}})
	s := b.currentShape()
	if s == nil || s.LineStyle == nil {
		t.Fatal("theme line style was not materialized")
	}
	if s.LineColor != "CC6600" || s.LineWidthPt != 3 {
		t.Fatalf("line = color %q width %v", s.LineColor, s.LineWidthPt)
	}
	if s.LineStyle.Cap != drawing.LineCapRound || s.LineStyle.Join != drawing.LineJoinBevel || s.LineStyle.Dash != drawing.DashPatternDashDot || s.LineStyle.CompoundType != "dbl" {
		t.Fatalf("line style = %+v", *s.LineStyle)
	}
	if s.ArrowHead != "triangle" || s.ArrowHeadWidth != "lg" || s.ArrowHeadLength != "sm" || s.ArrowTail != "oval" || s.ArrowTailWidth != "sm" || s.ArrowTailLength != "lg" {
		t.Fatalf("theme arrowheads = head(%q,%q,%q) tail(%q,%q,%q)", s.ArrowHead, s.ArrowHeadWidth, s.ArrowHeadLength, s.ArrowTail, s.ArrowTailWidth, s.ArrowTailLength)
	}
}

func TestIRBuilderDirectLineOverridesThemeLineStyleRef(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventThemeLineStyle, StyleIndex: 1, Line: LineProperties{WidthEMU: 38100, Color: VisualColor{Kind: "srgbClr", Value: "CC6600"}, Dash: "dash"}})
	must(Event{Type: EventStartShape, ShapeID: 9, ShapeKind: "shape"})
	must(Event{Type: EventLine, ShapeID: 9, Line: LineProperties{WidthEMU: 12700, Color: VisualColor{Kind: "srgbClr", Value: "112233"}}})
	must(Event{Type: EventLineStyleRef, ShapeID: 9, StyleIndex: 1, Color: VisualColor{Kind: "schemeClr", Value: "phClr"}})
	s := b.currentShape()
	if s.LineColor != "112233" || s.LineWidthPt != 1 {
		t.Fatalf("direct line overwritten: color=%q width=%v", s.LineColor, s.LineWidthPt)
	}
}

func TestIRBuilderCanonicalizesRawOOXMLRoundLineCapToken(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventStartShape, ShapeID: 11, ShapeKind: "shape"})
	must(Event{Type: EventLine, ShapeID: 11, Line: LineProperties{WidthEMU: 38100, Cap: "rnd", Join: "round", Color: VisualColor{Kind: "srgbClr", Value: "000000"}}})
	s := b.currentShape()
	if s == nil || s.LineStyle == nil {
		t.Fatal("direct OOXML line was not materialized")
	}
	if s.LineStyle.Cap != drawing.LineCapRound {
		t.Fatalf("line cap = %q, want %q", s.LineStyle.Cap, drawing.LineCapRound)
	}
	if s.LineStyle.Join != drawing.LineJoinRound {
		t.Fatalf("line join = %q, want %q", s.LineStyle.Join, drawing.LineJoinRound)
	}
}
