package legacy

import (
	"os"
	"strings"
	"testing"

	"office/pkg/word/render"
)

func TestParseDocCuratedPngPictureFixtureRendersLegacyDrawingImage(t *testing.T) {
	path := "../../../../testdata/doc/apache-PngPicture.doc"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("optional legacy PNG fixture is not available in this checkout")
		}
		t.Fatalf("stat fixture: %v", err)
	}
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}

	foundPNG := false
	floatingCount := 0
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Kind != "image" || run.Image == nil {
				continue
			}
			if run.Image.MIMEType != "image/png" {
				continue
			}
			if len(run.Image.Data) == 0 {
				t.Fatal("expected legacy drawing image payload")
			}
			if run.Image.Width == 0 || run.Image.Height == 0 {
				t.Fatalf("image size = (%d,%d), want non-zero", run.Image.Width, run.Image.Height)
			}
			if run.Image.Floating {
				floatingCount++
				if run.Image.WrapType != "square" {
					t.Fatalf("floating image wrap = %q, want %q", run.Image.WrapType, "square")
				}
				if run.Image.BehindDoc {
					t.Fatalf("floating image BehindDoc = %v, want false", run.Image.BehindDoc)
				}
				if run.Image.XRelative != "page" || run.Image.YRelative != "page" {
					t.Fatalf("floating image relatives = (%q,%q), want (page,page)", run.Image.XRelative, run.Image.YRelative)
				}
				if run.Image.X <= 0 {
					t.Fatalf("floating image X = %d, want positive page-relative offset", run.Image.X)
				}
			}
			foundPNG = true
		}
	}
	if !foundPNG {
		t.Fatalf("expected extracted png drawing image in paragraphs: %#v", doc.Paragraphs)
	}
	if floatingCount != 1 {
		t.Fatalf("floating png image count = %d, want 1", floatingCount)
	}
	for _, paragraph := range doc.Paragraphs {
		if strings.ContainsRune(paragraph.Text, '\b') {
			t.Fatalf("paragraph %q still contains unresolved drawing anchor", paragraph.Text)
		}
	}

	html, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML(%s): %v", path, err)
	}
	if !strings.Contains(html, "The quick brown fox jumps over the lazy dog") {
		t.Fatalf("HTML(%s): expected fixture body text in output", path)
	}
	if !strings.Contains(html, "data:image/png;base64,") {
		t.Fatalf("HTML(%s): expected rendered png drawing image in output", path)
	}
}
