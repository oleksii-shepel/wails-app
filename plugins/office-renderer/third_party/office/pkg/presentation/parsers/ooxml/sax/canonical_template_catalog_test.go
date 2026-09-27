package sax

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestContextualBuilderPublishesAndReusesCanonicalTemplateParts(t *testing.T) {
	b := newIRBuilder()
	b.masterPart = "ppt/slideMasters/slideMaster1.xml"
	b.layoutPart = "ppt/slideLayouts/slideLayout2.xml"
	b.masterShapes = []*ir.Shape{{ID: 1, Name: "master-art", Kind: ir.ShapeText}}
	b.layoutShapes = []*ir.Shape{{ID: 2, Name: "layout-template", Kind: ir.ShapeText, Placeholder: ir.PlaceholderBody}}

	b.slide = &ir.Slide{Index: 0, LayoutType: "titleBody", Shapes: []*ir.Shape{{ID: 3, Kind: ir.ShapeText}}}
	b.applyContextualInheritance()
	firstMaster, firstLayout := b.slide.Master, b.slide.Layout

	b.slide = &ir.Slide{Index: 1, LayoutType: "titleBody", Shapes: []*ir.Shape{{ID: 4, Kind: ir.ShapeText}}}
	b.applyContextualInheritance()
	if b.slide.Master != firstMaster || b.slide.Layout != firstLayout {
		t.Fatal("replayed SAX context manufactured a per-slide template graph")
	}
	if len(b.presentation.SlideMasters) != 1 || b.presentation.SlideMasters[0] != firstMaster {
		t.Fatalf("master catalog = %#v, want one canonical source master", b.presentation.SlideMasters)
	}
	if len(firstMaster.Layouts) != 1 || firstMaster.Layouts[0] != firstLayout || firstLayout.Master != firstMaster {
		t.Fatal("canonical master/layout ownership graph is inconsistent")
	}
	if firstMaster.Shapes[0] == b.masterShapes[0] || firstLayout.Shapes[0] == b.layoutShapes[0] {
		t.Fatal("canonical template catalog retained mutable SAX context shapes")
	}
}

func TestInheritedEmptyTextScaffoldingStaysTemplateOnly(t *testing.T) {
	empty := &ir.Shape{
		Kind:       ir.ShapeText,
		Paragraphs: []*ir.Paragraph{{}},
		Fill:       drawing.NoFill{},
		LineStyle:  &drawing.LineStyle{Fill: drawing.NoFill{}},
	}
	if got := cloneInheritedPaintShape(empty); got != nil {
		t.Fatalf("empty no-fill/no-line master textbox became effective paint: %#v", got)
	}
	field := cloneShape(empty)
	field.Paragraphs = []*ir.Paragraph{{Runs: []*ir.Run{{FieldKind: "slidenum"}}}}
	if got := cloneInheritedPaintShape(field); got == nil {
		t.Fatal("dynamic field master textbox was incorrectly treated as invisible scaffolding")
	}
}
