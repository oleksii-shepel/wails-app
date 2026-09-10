package layout

import "testing"

func TestReservedBodyHeightCombinesDynamicRegionsOnce(t *testing.T) {
	geom := PageGeometry{Height: 800, MarginTop: 80, MarginBottom: 100}
	// Static body = 620; 70 footnote + 30 epilogue = 520.
	if got, want := reservedBodyHeight(geom, 70, 30), 520.0; got != want {
		t.Fatalf("reserved body height = %.2fpt, want %.2fpt", got, want)
	}
}

func TestReservedBodyHeightFloorsPathologicalReservations(t *testing.T) {
	geom := PageGeometry{Height: 100, MarginTop: 40, MarginBottom: 40}
	if got := reservedBodyHeight(geom, 15, 20); got != 1 {
		t.Fatalf("reserved body height = %.2fpt, want 1pt progress floor", got)
	}
}

func TestTableFlowLimitUsesCanonicalReservedBodyAccounting(t *testing.T) {
	if got, want := tableFlowLimitHeight(500, 75), 425.0; got != want {
		t.Fatalf("table flow limit = %.2fpt, want %.2fpt", got, want)
	}
	if got := tableFlowLimitHeight(20, 30); got != 1 {
		t.Fatalf("table flow floor = %.2fpt, want 1pt", got)
	}
}

func TestHeaderFooterRemainStaticGeometryReservations(t *testing.T) {
	geom := PageGeometry{Height: 792, MarginTop: 90, MarginBottom: 84}
	got := reservedBodyHeight(geom, 0, 0)
	want := 618.0
	if got != want {
		t.Fatalf("static header/footer body = %.2fpt, want %.2fpt", got, want)
	}
}
