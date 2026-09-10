package layout

import (
	"os"
	"strings"
	"testing"
)

func TestFloatingParagraphsUsePhysicalPagination(t *testing.T) {
	data, err := os.ReadFile("layout.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if strings.Contains(src, "p.Format.KeepLines || paragraphHasFloatingContent(p)") {
		t.Fatal("floating paragraphs are still excluded from physical-line pagination")
	}
	for _, required := range []string{
		"if p.Frame != nil || p.Format.KeepLines {",
		"if startLine == 0 && paragraphHasFloatingContent(p) {",
		"fragmentFloats = appendParagraphFloats(",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("floating paragraph pagination missing %q", required)
		}
	}
}
