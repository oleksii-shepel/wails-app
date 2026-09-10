package layout

import "testing"

func TestTableFlowLimitPreservesReservedBodyRegion(t *testing.T) {
	geom := PageGeometry{Height: 200, MarginTop: 20, MarginBottom: 20}
	if got, want := tableFlowLimitHeight(geom.Height-geom.MarginTop-geom.MarginBottom, 35), 125.0; got != want {
		t.Fatalf("table flow limit = %.2fpt, want %.2fpt", got, want)
	}
}

func TestTableFlowLimitNeverBecomesNonPositive(t *testing.T) {
	geom := PageGeometry{Height: 100, MarginTop: 40, MarginBottom: 40}
	if got := tableFlowLimitHeight(geom.Height-geom.MarginTop-geom.MarginBottom, 50); got != 1 {
		t.Fatalf("table flow limit = %.2fpt, want 1pt safety floor", got)
	}
}
