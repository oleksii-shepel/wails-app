package sax

import "testing"

func TestIRBuilderResolvesSchemeColorsThroughEffectivePartColorMaps(t *testing.T) {
	b := newIRBuilder()
	b.themeColors = map[string]string{
		"dk1": "111111", "lt1": "EEEEEE", "accent1": "AA0000", "accent2": "00AA00", "accent3": "0000AA",
	}
	b.masterColorMap = map[string]string{"tx1": "dk1", "accent1": "accent1"}
	b.layoutColorMap = cloneStringMap(b.masterColorMap)
	b.layoutColorMap["accent1"] = "accent2"
	b.slideColorMap = cloneStringMap(b.layoutColorMap)
	b.slideColorMap["accent1"] = "accent3"

	b.activeColorScope = "master"
	if got := b.resolveVisualColor(VisualColor{Kind: "schemeClr", Value: "accent1"}); got != "AA0000" {
		t.Fatalf("master accent1 = %q, want AA0000", got)
	}
	b.activeColorScope = "layout"
	if got := b.resolveVisualColor(VisualColor{Kind: "schemeClr", Value: "accent1"}); got != "00AA00" {
		t.Fatalf("layout accent1 = %q, want 00AA00", got)
	}
	b.activeColorScope = "slide"
	if got := b.resolveVisualColor(VisualColor{Kind: "schemeClr", Value: "accent1"}); got != "0000AA" {
		t.Fatalf("slide accent1 = %q, want 0000AA", got)
	}
	if got := b.resolveVisualColor(VisualColor{Kind: "schemeClr", Value: "tx1"}); got != "111111" {
		t.Fatalf("slide tx1 = %q, want master-mapped dk1 color 111111", got)
	}
}

func TestIRBuilderColorMapEventsCascadeMasterLayoutSlide(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventStartMaster, Scope: "master"})
	must(Event{Type: EventColorMap, Scope: "master", ColorMap: map[string]string{"tx1": "dk1", "accent1": "accent1"}})
	must(Event{Type: EventEndMaster})
	must(Event{Type: EventStartLayout, Scope: "layout"})
	must(Event{Type: EventColorMap, Scope: "layout", ColorMap: map[string]string{"accent1": "accent2"}})
	must(Event{Type: EventEndLayout})
	must(Event{Type: EventColorMap, Scope: "slide", ColorMap: map[string]string{"accent1": "accent3"}})

	if b.slideColorMap["tx1"] != "dk1" || b.slideColorMap["accent1"] != "accent3" {
		t.Fatalf("effective slide color map = %#v", b.slideColorMap)
	}
}
