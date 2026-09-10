package layout

import (
	"os"
	"strings"
	"testing"
)

func TestTablePaginationUsesReservedFlowLimit(t *testing.T) {
	data, err := os.ReadFile("layout.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"trySplitTableAcrossFlow := func(block *resolve.Block, reserveHeight float64) bool",
		"flowLimitHeight := effectiveContentHeight(currentGeom) - reserveHeight",
		"remainingHeight := flowLimitHeight - currentY",
		"tableReserveHeight = epilogueReserveHeight",
		"trySplitTableAcrossFlow(block, tableReserveHeight)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("table pagination is not using the reserved body-flow boundary %q", required)
		}
	}
}
