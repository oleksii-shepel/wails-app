package legacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"office/pkg/word/render"
)

// replacementLegacyDOCFixtures is the intentional Word 97+ regression corpus.
// OfficeArt-heavy fixtures are kept out of this contract until the OfficeArt
// pass is re-enabled; they are still discovered by the general corpus runner.
var replacementLegacyDOCFixtures = []string{
	"legacy-columns.doc",
	"legacy-header-footer-fields.doc",
	"legacy-hyperlinks-fields.doc",
	"legacy-images-floating.doc",
	"legacy-images-inline.doc",
	"legacy-layout-numbered-paragraphs.doc",
	"legacy-layout-pagination.doc",
	"legacy-layout-sections.doc",
	"legacy-layout-stress.doc",
	"legacy-layout-tables.doc",
	"legacy-lists.doc",
	"legacy-mixed-page.doc",
	"legacy-notes-bookmarks-comments.doc",
	"legacy-page-breaks.doc",
	"legacy-table-pagination.doc",
	"legacy-tabs-borders.doc",
	"legacy-word97-layout-matrix.doc",
}

func TestReplacementLegacyDOCCorpusParsesAndRenders(t *testing.T) {
	for _, name := range replacementLegacyDOCFixtures {
		name := name
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "..", "testdata", "doc", name)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("replacement DOC fixture %s is required: %v", name, err)
			}
			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatalf("ParseDoc(%s): %v", path, err)
			}
			if doc == nil {
				t.Fatalf("ParseDoc(%s) returned nil document", path)
			}
			if len(doc.Paragraphs) == 0 && len(doc.Tables) == 0 {
				t.Fatalf("ParseDoc(%s) produced no body content", path)
			}
			html, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML(%s): %v", path, err)
			}
			if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "<body") {
				t.Fatalf("HTML(%s) has an invalid document envelope", path)
			}
		})
	}
}

func TestReplacementLegacyDOCCorpusCarriesExpectedStructures(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T, path string)
	}{
		{"legacy-lists.doc", func(t *testing.T, path string) {
			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range doc.Paragraphs {
				if p.List != nil {
					return
				}
			}
			t.Fatal("expected parsed list metadata")
		}},
		{"legacy-layout-tables.doc", func(t *testing.T, path string) {
			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Tables) == 0 {
				t.Fatal("expected at least one parsed table")
			}
		}},
		{"legacy-images-inline.doc", func(t *testing.T, path string) {
			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range doc.Paragraphs {
				for _, r := range p.Runs {
					if r.Image != nil && !r.Image.Floating {
						return
					}
				}
			}
			t.Fatal("expected at least one inline image")
		}},
		{"legacy-images-floating.doc", func(t *testing.T, path string) {
			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, p := range doc.Paragraphs {
				for _, r := range p.Runs {
					if r.Image != nil && r.Image.Floating {
						count++
					}
				}
			}
			if count != 3 {
				t.Fatalf("floating images = %d, want 3 picture-frame instances", count)
			}
		}},
		{"legacy-header-footer-fields.doc", func(t *testing.T, path string) {
			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Headers) == 0 || len(doc.Footers) == 0 {
				t.Fatalf("headers=%d footers=%d, want both", len(doc.Headers), len(doc.Footers))
			}
		}},
		{"legacy-layout-sections.doc", func(t *testing.T, path string) {
			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Sections) < 2 {
				t.Fatalf("sections=%d, want >=2", len(doc.Sections))
			}
		}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "..", "testdata", "doc", tc.name)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("replacement DOC fixture %s is required: %v", tc.name, err)
			}
			tc.check(t, path)
		})
	}
}
