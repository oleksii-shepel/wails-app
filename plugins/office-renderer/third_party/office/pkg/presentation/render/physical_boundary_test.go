package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesLayoutPhysicalVisualTree(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "MaterializePhysicalVisualTree") {
		t.Fatal("HTML must consume layout-owned physical visual tree")
	}
	if strings.Contains(s, "MaterializeVisualTree(shapes)") || strings.Contains(s, "MaterializeGroupChildren") {
		t.Fatal("HTML retained canonical hierarchy materialization ownership")
	}
}
