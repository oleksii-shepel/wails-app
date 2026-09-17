package layout

import (
	"os"
	"strings"
	"testing"
)

func TestTextShapeVisualCompositionHelperIsNotExported(t *testing.T) {
	data, err := os.ReadFile("text_visual.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if strings.Contains(source, "func ResolveTextVisualPlanWithShapeVisual(") {
		t.Fatal("layout retained transitional exported text composition helper")
	}
	if !strings.Contains(source, "func resolveTextVisualPlanWithShapeVisual(") {
		t.Fatal("layout lost internal text composition helper")
	}
}
