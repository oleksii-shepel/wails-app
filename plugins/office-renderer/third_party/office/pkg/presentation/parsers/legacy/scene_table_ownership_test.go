package legacy

import (
	presentationir "office/pkg/presentation/ir"
	"testing"
)

func TestSceneTableOwnershipRejectsDiagramTextConnectedByLines(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{bounds: presentationir.Rect{X: 0, Y: 0, W: 100, H: 40}, outlineRef: -1},
		{bounds: presentationir.Rect{X: 300, Y: 100, W: 100, H: 40}, outlineRef: -1},
		{bounds: presentationir.Rect{X: 600, Y: 200, W: 100, H: 40}, outlineRef: -1},
	}
	lines := []presentationir.Rect{
		{X: 100, Y: 20, W: 200, H: 80},
		{X: 400, Y: 120, W: 200, H: 80},
	}
	if legacyPptSceneGroupOwnsTable(false, cells, lines) {
		t.Fatal("diagram connectors must not transfer scene ownership to a table")
	}
	if !legacyPptLooksLikeTableGroup(cells, lines) {
		t.Fatal("compatibility candidate discovery should remain permissive")
	}
}

func TestSceneTableOwnershipAcceptsCoherentSparseGrid(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{bounds: presentationir.Rect{X: 0, Y: 0, W: 100, H: 40}, outlineRef: -1},
		{bounds: presentationir.Rect{X: 100, Y: 40, W: 100, H: 40}, outlineRef: -1},
	}
	lines := []presentationir.Rect{
		{X: 100, Y: 0, W: 0, H: 80},
		{X: 0, Y: 40, W: 200, H: 0},
	}
	if !legacyPptSceneGroupOwnsTable(false, cells, lines) {
		t.Fatal("aligned row/column grid evidence should retain sparse legacy table ownership")
	}
}

func TestSceneTableOwnershipRejectsSingleAxisBoxDiagram(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{bounds: presentationir.Rect{X: 0, Y: 0, W: 100, H: 40}, outlineRef: -1},
		{bounds: presentationir.Rect{X: 100, Y: 0, W: 100, H: 40}, outlineRef: -1},
		{bounds: presentationir.Rect{X: 200, Y: 0, W: 100, H: 40}, outlineRef: -1},
		{bounds: presentationir.Rect{X: 300, Y: 0, W: 100, H: 40}, outlineRef: -1},
	}
	if legacyPptSceneGroupOwnsTable(false, cells, nil) {
		t.Fatal("one-row aligned diagram boxes must remain ordinary grouped shapes")
	}
}

func TestTableAxisPositionsClusterLegacyAnchorJitter(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{bounds: presentationir.Rect{X: 0, Y: 1000000, W: 1000000, H: 600000}},
		{bounds: presentationir.Rect{X: 1200000, Y: 1011000, W: 1000000, H: 600000}},
		{bounds: presentationir.Rect{X: 2400000, Y: 1000000, W: 1000000, H: 600000}},
		{bounds: presentationir.Rect{X: 3600000, Y: 1011000, W: 1000000, H: 600000}},
	}
	if got := len(legacyPptTableAxisPositions(cells, false)); got != 1 {
		t.Fatalf("row positions = %d, want 1 after clustering anchor jitter", got)
	}
	if legacyPptTableCellGridHasTwoAxes(cells) {
		t.Fatal("single-row diagram must not gain a second table axis from anchor jitter")
	}
}

func TestSceneTableOwnershipRejectsOutlineReferencedDiagram(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{bounds: presentationir.Rect{X: 0, Y: 0, W: 100, H: 40}, outlineRef: 0},
		{bounds: presentationir.Rect{X: 120, Y: 0, W: 100, H: 40}, outlineRef: 1},
		{bounds: presentationir.Rect{X: 240, Y: 0, W: 100, H: 40}, outlineRef: 2},
		{bounds: presentationir.Rect{X: 360, Y: 0, W: 100, H: 40}, outlineRef: 3},
	}
	if legacyPptSceneGroupOwnsTable(false, cells, nil) {
		t.Fatal("OutlineTextRef identities must not turn a one-row diagram into a semantic table")
	}
}
