package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLTextShapeDoesNotFallBackToRendererParagraphLayout(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderTextShape(")
	end := strings.Index(src[start:], "\nfunc removeLayoutOwnedTextBodyCSS")
	if start < 0 || end < 0 {
		t.Fatal("renderTextShape boundary not found")
	}
	body := src[start : start+end]
	if strings.Contains(body, "renderParagraph(b,") {
		t.Fatal("renderTextShape retained renderer-side paragraph layout fallback")
	}
	if strings.Contains(body, "PhysicalLineWidth") {
		t.Fatal("renderTextShape retained horizontal line-width ownership")
	}
}
