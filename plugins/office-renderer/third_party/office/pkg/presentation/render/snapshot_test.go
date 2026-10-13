package render_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"office/pkg/presentation/edit"
	presir "office/pkg/presentation/ir"
	"office/pkg/presentation/render"
)

func TestPptxHTMLSnapshots(t *testing.T) {
	cases := []string{
		"sample-1",
		"sample-2",
		"sample-3",
		"sample-4",
		"sample-5",
		"sample-8",
		"sample-9",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			got := strings.TrimSpace(renderPptxHTML(t, name+".pptx"))
			snapPath := repoPath(t, "testdata", "pptx-html", name+".html")
			wantBytes, err := os.ReadFile(snapPath)
			if err != nil {
				t.Fatalf("read snapshot %s: %v", snapPath, err)
			}
			want := strings.TrimSpace(strings.ReplaceAll(string(wantBytes), "\r\n", "\n"))
			if got != want {
				t.Fatalf("snapshot mismatch for %s", name)
			}
		})
	}
}

func TestPptxHTMLRunSpansKeepExplicitColor(t *testing.T) {
	cases := []string{
		"sample-1",
		"sample-2",
		"sample-3",
		"sample-4",
		"sample-5",
		"sample-8",
		"sample-9",
	}
	spanRE := regexp.MustCompile(`<span style="([^"]*)">`)
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			html := renderPptxHTML(t, name+".pptx")
			matches := spanRE.FindAllStringSubmatch(html, -1)
			for _, m := range matches {
				if !strings.Contains(m[1], "color:#") {
					t.Fatalf("span missing explicit color in %s: %q", name, m[0])
				}
			}
		})
	}
}

func TestSample1HTMLKeepsAuthoredListMarkers(t *testing.T) {
	data, err := os.ReadFile(repoPath(t, "testdata", "pptx", "sample-1.pptx"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	edited, err := edit.LoadFromPPTXBytes(data)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	bullet := findRenderedFixtureParagraph(t, edited.IR.Slides[1].Shapes, "Progressive enhancement")
	if bullet.BulletChar != "●" {
		t.Fatalf("sample-1 bullet paragraph marker = %#v", bullet)
	}
	number := findRenderedFixtureParagraph(t, edited.IR.Slides[3].Shapes, "Create account")
	if !number.AutoNum || number.AutoNumType != "arabicPeriod" {
		t.Fatalf("sample-1 numbered paragraph marker = %#v", number)
	}

	html := renderPptxHTML(t, "sample-1.pptx")
	// Physical-line composition is allowed to wrap the authored run in a
	// ppt-physical-fragment span. Assert marker identity/style and authored run
	// styling independently instead of requiring them to be adjacent siblings.
	for _, want := range []string{
		`<span class="bullet-marker" style="font-size:24.0pt;font-family:Roboto,Arial,sans-serif;color:#424242;`,
		`>●</span>`,
		`font-size:24.0pt;font-family:Roboto,Arial,sans-serif;color:#424242">Progressive enhancement`,
		`>1.</span>`,
		`font-size:24.0pt;font-family:Roboto,Arial,sans-serif;color:#424242">Create account`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("sample-1 HTML missing list marker fragment %q", want)
		}
	}
}

func findRenderedFixtureParagraph(t *testing.T, shapes []*presir.Shape, needle string) *presir.Paragraph {
	t.Helper()
	if p := findRenderedFixtureParagraphInShapes(shapes, needle); p != nil {
		return p
	}
	t.Fatalf("paragraph containing %q not found", needle)
	return nil
}

func findRenderedFixtureParagraphInShapes(shapes []*presir.Shape, needle string) *presir.Paragraph {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		for _, p := range shape.Paragraphs {
			var text strings.Builder
			for _, r := range p.Runs {
				if r != nil {
					text.WriteString(r.Text)
				}
			}
			if strings.Contains(text.String(), needle) {
				return p
			}
		}
		if p := findRenderedFixtureParagraphInShapes(shape.Children, needle); p != nil {
			return p
		}
	}
	return nil
}

func renderPptxHTML(t *testing.T, pptxName string) string {
	t.Helper()
	return renderPptxHTMLWithOptions(t, pptxName, render.HTMLOptions{})
}

func renderPptxHTMLWithOptions(t *testing.T, pptxName string, opts render.HTMLOptions) string {
	t.Helper()
	data, err := os.ReadFile(repoPath(t, "testdata", "pptx", pptxName))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	edited, err := edit.LoadFromPPTXBytes(data)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	html, _, err := render.HTMLWithOptions(edited.IR, opts)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return html
}
