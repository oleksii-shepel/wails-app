package drawing

import (
	"reflect"
	"testing"
)

func TestClass13ArcDoesNotBorrowPieSectorGeometry(t *testing.T) {
	arc, _ := (&PresetGeometry{Name: "arc", Adjustments: []float64{0.10, 0.70}}).GetPath(21600, 10800)
	pie, _ := (&PresetGeometry{Name: "pie", Adjustments: []float64{0.10, 0.70}}).GetPath(21600, 10800)
	if reflect.DeepEqual(arc, pie) {
		t.Fatal("arc still aliases pie geometry")
	}
	for _, c := range arc.Commands {
		if c.Type == "Z" {
			t.Fatal("arc must remain open")
		}
	}
	if len(arc.Commands) != 2 || arc.Commands[0].Type != "M" || arc.Commands[1].Type != "A" {
		t.Fatalf("unexpected arc topology: %#v", arc.Commands)
	}
}

func TestClass13IrregularSealsOwnDistinctBurstGeometry(t *testing.T) {
	a, _ := (&PresetGeometry{Name: "irregularSeal1"}).GetPath(21600, 10800)
	b, _ := (&PresetGeometry{Name: "irregularSeal2"}).GetPath(21600, 10800)
	if reflect.DeepEqual(a, b) {
		t.Fatal("irregularSeal1 and irregularSeal2 still share one approximation")
	}
	if len(a.Commands) < 30 || len(b.Commands) < 30 {
		t.Fatal("seal geometry lost authored burst topology")
	}
}

func TestClass13MathPlusOwnsAdjustmentAndNonSquareBounds(t *testing.T) {
	thin, _ := (&PresetGeometry{Name: "mathPlus", Adjustments: []float64{0.12}}).GetPath(21600, 10800)
	thick, _ := (&PresetGeometry{Name: "mathPlus", Adjustments: []float64{0.40}}).GetPath(21600, 10800)
	generic, _ := (&PresetGeometry{Name: "cross"}).GetPath(21600, 10800)
	if reflect.DeepEqual(thin, thick) {
		t.Fatal("mathPlus adjustment does not affect geometry")
	}
	if reflect.DeepEqual(thin, generic) {
		t.Fatal("mathPlus still borrows generic cross geometry")
	}
}
