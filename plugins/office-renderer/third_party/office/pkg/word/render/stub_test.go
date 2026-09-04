package render

import (
	"strings"
	"testing"

	resolve "office/pkg/word/ir"
)

func TestMathMLEmptyDocument(t *testing.T) {
	doc := &resolve.Document{}
	out, err := MathML(doc, MathMLOptions{})
	if err != nil {
		t.Fatalf("MathML: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output, got %q", out)
	}

	wrapped, err := MathML(doc, MathMLOptions{WrapHTML: true, Title: "Empty"})
	if err != nil {
		t.Fatalf("MathML: %v", err)
	}
	if !strings.HasPrefix(wrapped, "<!DOCTYPE html>") {
		t.Fatalf("expected HTML shell, got %q", wrapped)
	}
	if !strings.Contains(wrapped, "<title>Empty</title>") {
		t.Fatalf("expected title in HTML shell, got %q", wrapped)
	}
}

func TestMathMLCollectsFromAllStories(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "math", MathML: "<m:math>body</m:math>"}},
			}},
		},
		Headers: map[string]resolve.Story{
			"header1": {Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{Kind: "math", MathML: "<m:math>header</m:math>"}},
				}},
			}},
		},
		Footers: map[string]resolve.Story{
			"footer1": {Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{Kind: "math", MathML: "<m:math>footer</m:math>"}},
				}},
			}},
		},
		Footnotes: map[int]resolve.Story{
			1: {Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{Kind: "math", MathML: "<m:math>footnote</m:math>"}},
				}},
			}},
		},
		Endnotes: map[int]resolve.Story{
			1: {Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{Kind: "math", MathML: "<m:math>endnote</m:math>"}},
				}},
			}},
		},
		Comments: map[int]resolve.Story{
			1: {Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{Kind: "math", MathML: "<m:math>comment</m:math>"}},
				}},
			}},
		},
	}

	out, err := MathML(doc, MathMLOptions{})
	if err != nil {
		t.Fatalf("MathML: %v", err)
	}
	for _, expected := range []string{"body", "header", "footer", "footnote", "endnote", "comment"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("missing math from %s in output:\n%s", expected, out)
		}
	}
}

func TestMathMLCollectsFromTables(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{
							{Kind: "paragraph", Paragraph: &resolve.Paragraph{
								Runs: []resolve.Run{{Kind: "math", MathML: "<m:math>cell</m:math>"}},
							}},
						},
					}},
				}},
			}},
		},
	}
	out, err := MathML(doc, MathMLOptions{})
	if err != nil {
		t.Fatalf("MathML: %v", err)
	}
	if !strings.Contains(out, "cell") {
		t.Fatalf("expected math from table cell, got:\n%s", out)
	}
}

func TestMathMLDisplayVsInline(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "math", MathML: `<m:math display="block">block</m:math>`},
					{Kind: "math", MathML: "<m:math>inline</m:math>"},
				},
			}},
		},
	}
	out, err := MathML(doc, MathMLOptions{})
	if err != nil {
		t.Fatalf("MathML: %v", err)
	}
	if !strings.Contains(out, `<div class="math-display">`) {
		t.Fatalf("expected display math wrapper, got:\n%s", out)
	}
	if !strings.Contains(out, "<span><m:math>inline</m:math></span>") {
		t.Fatalf("expected inline math wrapper, got:\n%s", out)
	}
}

func TestMathMLWrapHTMLAndCSS(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "math", MathML: "<m:math>x</m:math>"}},
			}},
		},
	}
	out, err := MathML(doc, MathMLOptions{WrapHTML: true, InlineCSS: true, Title: "Test Math"})
	if err != nil {
		t.Fatalf("MathML: %v", err)
	}
	if !strings.HasPrefix(out, "<!DOCTYPE html>") {
		t.Fatalf("expected HTML shell, got %q", out)
	}
	if !strings.Contains(out, "<title>Test Math</title>") {
		t.Fatalf("expected title, got %q", out)
	}
	if !strings.Contains(out, "<style>") {
		t.Fatalf("expected inline CSS, got %q", out)
	}
	if !strings.Contains(out, "Cambria Math") {
		t.Fatalf("expected Cambria Math font CSS, got %q", out)
	}
}

func TestMathMLEscapesTitle(t *testing.T) {
	doc := &resolve.Document{}
	out, err := MathML(doc, MathMLOptions{WrapHTML: true, Title: "Test <Math> \"Doc\""})
	if err != nil {
		t.Fatalf("MathML: %v", err)
	}
	if strings.Contains(out, "<title>Test <Math>") {
		t.Fatalf("title was not escaped: %q", out)
	}
	if !strings.Contains(out, "<title>Test &lt;Math&gt; &quot;Doc&quot;</title>") {
		t.Fatalf("expected escaped title, got %q", out)
	}
}
