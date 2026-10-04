package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLSerializesPhysicalTextLineHeight(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{"ppt-physical-line", "line-height:%.4fpt", "height:%.4fpt", "min-height:%.4fpt", "overflow:visible", "line.HeightPt"} {
		if !strings.Contains(src, required) {
			t.Fatalf("HTML physical-line serializer missing %q", required)
		}
	}
}

func TestHTMLPhysicalLineAdvanceLocksLayoutWithoutClippingInk(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, `line-height:%.4fpt;height:%.4fpt`) {
		t.Fatal("measured line advance must lock the browser block height")
	}
	if !strings.Contains(src, `min-height:%.4fpt;overflow:visible`) {
		t.Fatal("physical lines must preserve advance while allowing glyph ink outside it")
	}
}
