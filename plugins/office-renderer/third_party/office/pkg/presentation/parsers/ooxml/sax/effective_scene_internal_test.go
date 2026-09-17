package sax

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestAppendDecorativeShapesPrunesNestedPlaceholderTemplates(t *testing.T) {
	source := []*ir.Shape{{
		Name: "master-group",
		Kind: ir.ShapeGroup,
		Children: []*ir.Shape{
			{Name: "body-template", Kind: ir.ShapeText, Placeholder: ir.PlaceholderBody},
			{Name: "master-decoration", Kind: ir.ShapeText},
		},
	}}

	got := appendDecorativeShapes(nil, source)
	if len(got) != 1 || len(got[0].Children) != 1 {
		t.Fatalf("effective inherited graph = %#v, want one group with one paint child", got)
	}
	if got[0].Children[0].Name != "master-decoration" {
		t.Fatalf("paint child = %q, want master-decoration", got[0].Children[0].Name)
	}
	if len(source[0].Children) != 2 {
		t.Fatalf("source template graph mutated: children=%d", len(source[0].Children))
	}
}

func TestAppendDecorativeShapesPrunesNestedMasterEditingPrompts(t *testing.T) {
	prompt := &ir.Shape{
		Name:       "editing-prompt",
		Kind:       ir.ShapeText,
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "Click to edit Master text styles"}}}},
	}
	source := []*ir.Shape{{
		Name: "master-group",
		Kind: ir.ShapeGroup,
		Children: []*ir.Shape{
			prompt,
			{Name: "University", Kind: ir.ShapeText},
		},
	}}

	got := appendDecorativeShapes(nil, source)
	if len(got) != 1 || len(got[0].Children) != 1 || got[0].Children[0].Name != "University" {
		t.Fatalf("effective inherited graph = %#v, want only legitimate grouped artwork", got)
	}
}

func TestAppendDecorativeShapesDropsTemplateOnlyGroups(t *testing.T) {
	source := []*ir.Shape{{
		Name:     "template-group",
		Kind:     ir.ShapeGroup,
		Children: []*ir.Shape{{Name: "title-template", Kind: ir.ShapeText, Placeholder: ir.PlaceholderTitle}},
	}}

	if got := appendDecorativeShapes(nil, source); len(got) != 0 {
		t.Fatalf("template-only group materialized as paint: %#v", got)
	}
}
