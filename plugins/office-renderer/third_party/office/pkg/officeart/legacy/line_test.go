package legacy

import "testing"

func TestDashName(t *testing.T) {
	if got := DashName(7); got != "lgDash" {
		t.Fatalf("DashName(7) = %q", got)
	}
	if got := DashName(0); got != "solid" {
		t.Fatalf("DashName(0) = %q", got)
	}
}
