package sax

import "testing"

func TestIRBuilderCarriesLayoutTypeIntoCanonicalSlideAndLayout(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventStartMaster, Scope: "master"})
	must(Event{Type: EventEndMaster, Scope: "master"})
	must(Event{Type: EventStartLayout, Scope: "layout", LayoutType: "obj"})
	must(Event{Type: EventEndLayout, Scope: "layout"})
	must(Event{Type: EventEndSlide, SlideIndex: 0})

	if len(b.slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(b.slides))
	}
	slide := b.slides[0]
	if slide.LayoutType != "titleBody" {
		t.Fatalf("slide LayoutType = %q, want titleBody", slide.LayoutType)
	}
	if slide.Layout == nil || slide.Layout.Type != "titleBody" {
		t.Fatalf("canonical layout = %#v, want type titleBody", slide.Layout)
	}
}
