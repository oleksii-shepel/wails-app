package sax

import "testing"

func TestIRBuilderResolvesBlipsInOwningPartRelationshipScope(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, PartName: "ppt/slides/slide1.xml", SlideIndex: 0})
	must(Event{Type: EventRelationshipPayload, PartName: "ppt/slides/slide1.xml", SlideIndex: 0, Scope: "slide", RelID: "rId2", Payload: []byte("slide"), ContentType: "image/png"})
	must(Event{Type: EventRelationshipPayload, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master", RelID: "rId2", Payload: []byte("master"), ContentType: "image/jpeg"})
	must(Event{Type: EventStartMaster, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master"})
	must(Event{Type: EventStartShape, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master", ShapeKind: "pic"})
	must(Event{Type: EventBlip, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master", RelID: "rId2"})
	if got := string(b.currentShape().ImageData); got != "master" {
		t.Fatalf("master rId2 resolved to %q, want master payload", got)
	}
	must(Event{Type: EventEndShape, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master"})
	must(Event{Type: EventEndMaster, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master"})
	must(Event{Type: EventStartShape, PartName: "ppt/slides/slide1.xml", SlideIndex: 0, Scope: "slide", ShapeKind: "pic"})
	must(Event{Type: EventBlip, PartName: "ppt/slides/slide1.xml", SlideIndex: 0, Scope: "slide", RelID: "rId2"})
	if got := string(b.currentShape().ImageData); got != "slide" {
		t.Fatalf("slide rId2 resolved to %q, want slide payload", got)
	}
}

func TestIRBuilderDoesNotBorrowSlidePayloadForUnresolvedMasterBlip(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, PartName: "ppt/slides/slide1.xml", SlideIndex: 0})
	must(Event{Type: EventRelationshipPayload, PartName: "ppt/slides/slide1.xml", SlideIndex: 0, Scope: "slide", RelID: "rId2", Payload: []byte("slide-image"), ContentType: "image/png"})
	must(Event{Type: EventStartMaster, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master"})
	must(Event{Type: EventStartShape, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master", ShapeKind: "pic"})
	must(Event{Type: EventBlip, PartName: "ppt/slideMasters/slideMaster1.xml", SlideIndex: 0, Scope: "master", RelID: "rId2"})
	if got := b.currentShape().ImageData; len(got) != 0 {
		t.Fatalf("unresolved master rId2 borrowed slide payload %q", string(got))
	}
}

func TestIRBuilderDoesNotBorrowSlideRelationshipForUnresolvedLayoutResource(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, PartName: "ppt/slides/slide1.xml", SlideIndex: 0})
	must(Event{Type: EventRelationship, PartName: "ppt/slides/slide1.xml", SlideIndex: 0, Scope: "slide", RelID: "rId7", TargetPart: "ppt/media/slide.png"})
	got := b.relationshipForEvent(Event{PartName: "ppt/slideLayouts/slideLayout1.xml", SlideIndex: 0, Scope: "layout", RelID: "rId7"})
	if got.Target != "" || got.External {
		t.Fatalf("unresolved layout rId7 borrowed slide relationship: %+v", got)
	}
}
