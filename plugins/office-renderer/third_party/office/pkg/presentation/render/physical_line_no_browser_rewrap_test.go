package render

import (
	"os"
	"strings"
	"testing"
)

func TestPhysicalLinesDoNotAskBrowserToRewrapMeasuredText(t *testing.T) {
	src, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	const marker = `class=\"ppt-physical-line\" style=\"`
	checked := 0
	for rest := body; ; {
		i := strings.Index(rest, marker)
		if i < 0 {
			break
		}
		rest = rest[i+len(marker):]
		end := strings.Index(rest, `\">`)
		if end < 0 {
			t.Fatal("unterminated physical line style in html.go")
		}
		style := rest[:end]
		checked++
		if strings.Contains(style, "white-space:pre-wrap") {
			t.Fatalf("measured physical lines must not be browser-rewrapped after layout has chosen breaks: %q", style)
		}
		if !strings.Contains(style, "white-space:pre;") {
			t.Fatalf("physical line must keep the breaks layout measured: %q", style)
		}
		if !strings.Contains(style, "display:block;") {
			t.Fatalf("measured physical line should serialize as a non-rewrapping block: %q", style)
		}
	}
	if checked == 0 {
		t.Fatal("html.go no longer serializes physical lines")
	}
}
