package sax

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestShapeLinePatternFillStaysOnLine(t *testing.T) {
	xml := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="7" name="freeform"/></p:nvSpPr><p:spPr><a:custGeom><a:avLst/><a:pathLst><a:path w="1" h="1"><a:moveTo><a:pt x="0" y="0"/></a:moveTo><a:lnTo><a:pt x="1" y="1"/></a:lnTo></a:path></a:pathLst></a:custGeom><a:noFill/><a:ln w="76200"><a:pattFill prst="wdUpDiag"><a:fgClr><a:srgbClr val="FF0000"/></a:fgClr><a:bgClr><a:srgbClr val="FFFF00"/></a:bgClr></a:pattFill><a:prstDash val="solid"/></a:ln></p:spPr></p:sp></p:spTree></p:cSld></p:sld>`
	b := newIRBuilder()
	if err := b.HandleEvent(Event{Type: EventStartSlide, SlideIndex: 0}); err != nil {
		t.Fatal(err)
	}
	if err := walkShapeTreeReader(strings.NewReader(xml), "ppt/slides/slide1.xml", 0, b.HandleEvent); err != nil {
		t.Fatal(err)
	}
	if len(b.slide.Shapes) != 1 {
		t.Fatalf("shapes = %d, want 1", len(b.slide.Shapes))
	}
	s := b.slide.Shapes[0]
	if _, ok := s.Fill.(drawing.NoFill); !ok {
		t.Fatalf("shape fill = %T, want drawing.NoFill", s.Fill)
	}
	if s.LineStyle == nil {
		t.Fatal("line style = nil")
	}
	pf, ok := s.LineStyle.Fill.(drawing.PatternFill)
	if !ok {
		t.Fatalf("line fill = %T, want drawing.PatternFill", s.LineStyle.Fill)
	}
	if pf.Foreground != "FF0000" || pf.Background != "FFFF00" || s.LineColor != "FF0000" || s.LineWidthPt != 6 {
		t.Fatalf("canonical patterned line = fill %#v color %q width %v", pf, s.LineColor, s.LineWidthPt)
	}
}

func TestVisibleDrawingMLLineWithoutWidthUsesPowerPointDefault(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventStartShape, ShapeID: 8, ShapeKind: "cxnSp"})
	must(Event{Type: EventLine, ShapeID: 8, Line: LineProperties{Color: VisualColor{Kind: "srgbClr", Value: "112233"}}})
	if got := b.currentShape().LineWidthPt; got != 0.75 {
		t.Fatalf("implicit line width = %v, want 0.75pt", got)
	}
	must(Event{Type: EventEndShape, ShapeID: 8})
}

func TestZeroDrawingMLLineWidthUsesPowerPointHairlineDefault(t *testing.T) {
	b := newIRBuilder()
	must := func(ev Event) {
		if err := b.HandleEvent(ev); err != nil {
			t.Fatal(err)
		}
	}
	must(Event{Type: EventStartSlide, SlideIndex: 0})
	must(Event{Type: EventStartShape, ShapeID: 9, ShapeKind: "cxnSp"})
	must(Event{Type: EventLine, ShapeID: 9, Line: LineProperties{HasWidth: true, Color: VisualColor{Kind: "srgbClr", Value: "112233"}}})
	if got := b.currentShape().LineWidthPt; got != 0.75 {
		t.Fatalf("zero DrawingML line width = %v, want 0.75pt", got)
	}
}
