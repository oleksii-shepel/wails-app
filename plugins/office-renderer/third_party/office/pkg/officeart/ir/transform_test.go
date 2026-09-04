package ir

import "testing"

func TestEffectiveMatrixComposesParentAndLocal(t *testing.T) {
	parent := Matrix{A: 1, D: 1, E: 10, F: 20}
	local := Matrix{A: 1, D: 1, E: 5, F: 7}
	got := EffectiveMatrix([]Matrix{parent}, local)
	if got.E != 15 || got.F != 27 {
		t.Fatalf("translation = (%g,%g), want (15,27)", got.E, got.F)
	}
}
