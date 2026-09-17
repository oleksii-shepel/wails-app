package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesPhysicalGroupAncestry(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if strings.Contains(src, "item.Ancestry") {
		t.Fatal("renderer retained superseded PhysicalVisual ancestry alias")
	}
	if !strings.Contains(src, "item.Drawing.Paint.AncestorClips") {
		t.Fatal("renderer does not consume layout-owned ancestor clips from physical paint")
	}
}
