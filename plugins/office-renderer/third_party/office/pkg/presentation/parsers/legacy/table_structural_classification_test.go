package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestLegacySparseTableRequiresGridLinesOnBothCellAxes(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{bounds: presentationir.Rect{X: 0, Y: 0, W: 100, H: 40}},
		{bounds: presentationir.Rect{X: 100, Y: 40, W: 100, H: 40}},
	}
	lines := []presentationir.Rect{
		{X: 100, Y: 0, W: 0, H: 80},
		{X: 0, Y: 40, W: 200, H: 0},
	}
	if !legacyPptGridLinesSupportCellLattice(cells, lines) {
		t.Fatal("sparse table grid lines aligned to cell boundaries must be accepted")
	}
}

func TestLegacyDiagramConnectorsDoNotBecomeStructuralTableGridEvidence(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{bounds: presentationir.Rect{X: 0, Y: 0, W: 100, H: 40}},
		{bounds: presentationir.Rect{X: 300, Y: 200, W: 100, H: 40}},
		{bounds: presentationir.Rect{X: 600, Y: 400, W: 100, H: 40}},
	}
	// These are plausible diagram connectors, but they do not form horizontal and
	// vertical rules coincident with the text-box cell boundaries.
	lines := []presentationir.Rect{
		{X: 100, Y: 20, W: 200, H: 80},
		{X: 400, Y: 220, W: 200, H: 80},
		{X: 200, Y: 100, W: 0, H: 100},
	}
	if legacyPptGridLinesSupportCellLattice(cells, lines) {
		t.Fatal("diagram connectors must not be accepted as table grid evidence")
	}
	// Compatibility candidate discovery intentionally remains permissive: old PPT
	// producers can omit enough geometry that line presence is the only surviving
	// table hint. This helper may therefore return true here. Group ownership is
	// decided separately by recoverLegacyPptGroupIsTableLike and later provenance.
	if !legacyPptLooksLikeTableGroup(cells, lines) {
		t.Fatal("weak legacy geometry must remain eligible as a compatibility table candidate")
	}
}

func TestLegacyDenseCellLatticeDoesNotRequireGridLines(t *testing.T) {
	cells := []legacyPptTableCellShape{
		{bounds: presentationir.Rect{X: 0, Y: 0, W: 100, H: 40}},
		{bounds: presentationir.Rect{X: 100, Y: 0, W: 100, H: 40}},
		{bounds: presentationir.Rect{X: 0, Y: 40, W: 100, H: 40}},
		{bounds: presentationir.Rect{X: 100, Y: 40, W: 100, H: 40}},
	}
	if !legacyPptLooksLikeTableGroup(cells, nil) {
		t.Fatal("dense authored cell lattice must remain a compatibility table")
	}
}
