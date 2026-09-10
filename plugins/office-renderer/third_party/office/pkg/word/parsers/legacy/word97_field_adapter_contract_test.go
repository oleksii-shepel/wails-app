package legacy

import (
	"testing"

	docformat "office/pkg/word/parsers/doc/format"
	word97 "office/pkg/word/parsers/doc/word97"
)

func TestAdaptCanonicalMaterializedFieldsPreservesRecursiveSemanticTree(t *testing.T) {
	src := []*docformat.MaterializedField[word97.Run]{
		nil,
		{
			Kind:      "HYPERLINK",
			Instr:     `HYPERLINK "https://example.test" \\l "target"`,
			Arg:       "https://example.test",
			Anchor:    "target",
			Target:    "https://example.test",
			Children:  []word97.Run{{Text: "display", Kind: "text"}},
			InstrRuns: []word97.Run{{Text: "instruction", Kind: "text"}},
		},
	}

	got := adaptCanonicalMaterializedFields(src)
	if len(got) != 1 {
		t.Fatalf("fields = %d, want 1", len(got))
	}
	field := got[0]
	if field.Kind != "HYPERLINK" || field.Arg != "https://example.test" || field.Anchor != "target" || field.Target != "https://example.test" {
		t.Fatalf("field semantic metadata = %+v", field)
	}
	if len(field.Children) != 1 || field.Children[0].Text != "display" {
		t.Fatalf("children = %+v", field.Children)
	}
	if len(field.InstrRuns) != 1 || field.InstrRuns[0].Text != "instruction" {
		t.Fatalf("instruction runs = %+v", field.InstrRuns)
	}
}
