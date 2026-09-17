package sax

import "testing"

func TestIRBuilderMaterializesThemeEffectStyleForEffectRef(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		t.Helper()
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide})
	must(Event{Type: EventStartShape, ShapeID: 1})
	must(Event{Type: EventThemeColor, ThemeSlot: "accent2", Color: VisualColor{Kind: "srgbClr", Value: "CC6600"}})
	must(Event{Type: EventThemeEffectStyle, StyleIndex: 2, ThemeEffect: ThemeEffectStyle{Effects: []EffectProperties{{Kind: "glow", Rad: 12700, Color: VisualColor{Kind: "schemeClr", Value: "phClr"}}}}})
	must(Event{Type: EventEffectStyleRef, ShapeID: 1, StyleIndex: 2, Color: VisualColor{Kind: "schemeClr", Value: "accent2"}})
	if got := len(b.currentShape().Effects.Effects); got != 1 {
		t.Fatalf("effects = %d, want 1", got)
	}
}

func TestIRBuilderDirectEffectWinsOverThemeEffectRef(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide})
	must(Event{Type: EventStartShape, ShapeID: 1})
	must(Event{Type: EventThemeEffectStyle, StyleIndex: 1, ThemeEffect: ThemeEffectStyle{Effects: []EffectProperties{{Kind: "glow", Rad: 12700}}}})
	must(Event{Type: EventEffect, ShapeID: 1, Effect: EffectProperties{Kind: "softEdge", Rad: 25400}})
	must(Event{Type: EventEffectStyleRef, ShapeID: 1, StyleIndex: 1})
	if got := len(b.currentShape().Effects.Effects); got != 1 {
		t.Fatalf("effects = %d, want direct effect only", got)
	}
}
