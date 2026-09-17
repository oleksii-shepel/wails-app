package sax

import (
	"strings"
	"testing"
)

func TestShapeLineStyleRefEmitsSchemeColorWithoutDirectLinePaint(t *testing.T) {
	xml := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="7" name="ornament"/></p:nvSpPr><p:spPr><a:prstGeom prst="line"/></p:spPr><p:style><a:lnRef idx="1"><a:schemeClr val="accent2"/></a:lnRef></p:style></p:sp></p:spTree></p:cSld></p:sld>`
	var got *Event
	if err := walkShapeTreeReader(strings.NewReader(xml), "ppt/slideMasters/slideMaster1.xml", 0, func(ev Event) error {
		if ev.Type == EventLineStyleRef {
			copy := ev
			got = &copy
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("missing line style reference event")
	}
	if got.ShapeID != 7 || got.Color.Kind != "schemeClr" || got.Color.Value != "accent2" {
		t.Fatalf("line style ref = %#v", *got)
	}
}

func TestIRBuilderUsesLineStyleRefColorOnlyWhenDirectLineColorIsAbsent(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventStartMaster, Scope: "master"})
	b.themeColors = map[string]string{"accent1": "336699", "accent2": "CC6600"}
	b.masterColorMap = map[string]string{"accent1": "accent2"}
	must(Event{Type: EventStartShape, ShapeID: 7, ShapeKind: "sp"})
	must(Event{Type: EventLineStyleRef, ShapeID: 7, Color: VisualColor{Kind: "schemeClr", Value: "accent1"}})
	if got := b.currentShape().LineColor; got != "CC6600" {
		t.Fatalf("style-ref line color = %q, want CC6600", got)
	}
	must(Event{Type: EventLine, ShapeID: 7, Line: LineProperties{Color: VisualColor{Kind: "srgbClr", Value: "123456"}, WidthEMU: 12700}})
	if got := b.currentShape().LineColor; got != "123456" {
		t.Fatalf("direct line color = %q, want 123456", got)
	}
}

func TestShapeFillStyleRefEmitsSchemeColorWithoutDirectFillPaint(t *testing.T) {
	xml := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="8" name="ornament fill"/></p:nvSpPr><p:spPr><a:prstGeom prst="rect"/></p:spPr><p:style><a:fillRef idx="2"><a:schemeClr val="accent3"/></a:fillRef></p:style></p:sp></p:spTree></p:cSld></p:sld>`
	var got *Event
	if err := walkShapeTreeReader(strings.NewReader(xml), "ppt/slideLayouts/slideLayout1.xml", 0, func(ev Event) error {
		if ev.Type == EventFillStyleRef {
			copy := ev
			got = &copy
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("missing fill style reference event")
	}
	if got.ShapeID != 8 || got.Color.Kind != "schemeClr" || got.Color.Value != "accent3" {
		t.Fatalf("fill style ref = %#v", *got)
	}
}

func TestIRBuilderUsesFillStyleRefColorOnlyWhenDirectFillIsAbsent(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventStartLayout, Scope: "layout"})
	b.themeColors = map[string]string{"accent1": "336699", "accent3": "99CC00"}
	b.layoutColorMap = map[string]string{"accent1": "accent3"}
	must(Event{Type: EventStartShape, ShapeID: 8, ShapeKind: "sp"})
	must(Event{Type: EventFillStyleRef, ShapeID: 8, Color: VisualColor{Kind: "schemeClr", Value: "accent1"}})
	if got := b.currentShape().FillColor; got != "99CC00" {
		t.Fatalf("style-ref fill color = %q, want 99CC00", got)
	}
	must(Event{Type: EventFill, ShapeID: 8, Color: VisualColor{Kind: "srgbClr", Value: "123456"}})
	if got := b.currentShape().FillColor; got != "123456" {
		t.Fatalf("direct fill color = %q, want 123456", got)
	}
}
