package stream

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wordrender "office/pkg/word/render"
)

func TestApacheStrictHeadingFormats(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "docx", "apache-FloatingPictures-open-xml.docx"))
	if err != nil {
		t.Fatal(err)
	}
	session := NewLayoutSession(1, 0)
	metadata, err := loadDOCXRenderMetadata(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata.Document.Styles) == 0 {
		t.Fatal("Strict metadata did not load any paragraph styles")
	}
	if err := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{}).Walk(session); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		style  string
		size   int
		bold   bool
		italic bool
	}{
		{"Heading1", 32, true, false},
		{"Heading2", 28, true, true},
		{"Heading3", 26, true, false},
	}
	for i, tt := range want {
		p := session.Document().Paragraphs[i+1]
		if p.StyleID != tt.style || len(p.Runs) != 1 {
			t.Fatalf("paragraph %d = style %q with %d runs, want style %q with one run", i+1, p.StyleID, len(p.Runs), tt.style)
		}
		format := p.Runs[0].Format
		italic := format.Italic != nil && *format.Italic
		if format.FontSize == nil || *format.FontSize != tt.size || format.Bold == nil || *format.Bold != tt.bold || italic != tt.italic {
			t.Fatalf("%s format = %#v, want %dpt bold=%t italic=%t", tt.style, p.Runs, tt.size/2, tt.bold, tt.italic)
		}
	}
	html, err := wordrender.HTML(session.Document())
	if err != nil {
		t.Fatal(err)
	}
	if pages := strings.Count(html, `class="page"`); pages != 8 {
		t.Fatalf("rendered page count = %d, want the PDF reference's 8 pages", pages)
	}
}

func TestNormalizeStrictMetadataLexicalValues(t *testing.T) {
	data := normalizeStrictSettingsLexicalValues([]byte(`<w:settings><w:zoom w:percent="100%"/><w:defaultTabStop w:val="36pt"/><w:spacing w:before="5.4pt"/><w:sz w:val="12pt"/></w:settings>`))
	for _, want := range []string{`percent="100"`, `val="720"`, `before="108"`, `sz w:val="24"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("normalized settings %q do not contain %q", data, want)
		}
	}
	theme := normalizeStrictPointValues([]byte(`<a:gs pos="50%"><a:schemeClr><a:lumMod val="110%"/></a:schemeClr></a:gs>`), strictThemePercentageAttribute, 1000)
	for _, want := range []string{`pos="50000"`, `val="110000"`} {
		if !strings.Contains(string(theme), want) {
			t.Fatalf("normalized theme %q does not contain %q", theme, want)
		}
	}
}

func TestSample1PictureBulletUsesNumberingImage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "docx", "sample1.docx"))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := loadDOCXRenderMetadata(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	numbering, ok := metadata.Numbering[4][2]
	if !ok {
		t.Fatal("sample1 picture-bullet numbering level was not resolved")
	}
	picture := numbering.PictureBullet
	if picture == nil || picture.MIMEType != "image/gif" || len(picture.Data) == 0 || picture.Width != 114300 || picture.Height != 114300 {
		t.Fatalf("picture bullet = %#v, want a 9pt image/gif marker", picture)
	}
}
