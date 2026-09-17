package layout

import (
	"os"
	"strings"
	"testing"
)

func TestPhysicalVisualFamiliesReuseMaterializedComposition(t *testing.T) {
	data, err := os.ReadFile("physical_visual.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func MaterializePhysicalVisualTree(")
	if start < 0 {
		t.Fatal("cannot locate MaterializePhysicalVisualTree source")
	}
	relEnd := strings.Index(src[start:], "\nfunc resolvePhysicalClipPlan(")
	if relEnd < 0 {
		t.Fatal("cannot locate end of MaterializePhysicalVisualTree source")
	}
	body := src[start : start+relEnd]
	if strings.Count(body, "materializePhysicalCompositionPlan(") != 1 {
		t.Fatal("physical visual tree must materialize common composition exactly once per visual")
	}
	for _, required := range []string{
		"resolveImageVisualPlanWithComposition(shape, composition)",
		"resolveConnectorVisualPlanWithComposition(shape, composition)",
		"resolveFreeConnectorVisualPlanWithComposition(shape, composition)",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("physical visual materialization does not reuse composition through %q", required)
		}
	}
}
