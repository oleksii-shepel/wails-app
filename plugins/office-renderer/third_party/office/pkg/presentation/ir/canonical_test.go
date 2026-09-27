package ir

import "testing"

func TestCanonicalRotationDeg(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{0, 0}, {270, -90}, {-90, -90}, {360, 0}, {-360, 0},
		{348.5, -11.5}, {-190, 170}, {180, -180}, {-180, -180},
	}
	for _, tt := range tests {
		if got := CanonicalRotationDeg(tt.in); got != tt.want {
			t.Fatalf("CanonicalRotationDeg(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestCanonicalArrowEnd(t *testing.T) {
	if k, w, l := CanonicalArrowEnd("none", "med", "lg"); k != "" || w != "" || l != "" {
		t.Fatalf("none arrow = (%q,%q,%q)", k, w, l)
	}
	if k, w, l := CanonicalArrowEnd("stealth", "", ""); k != "stealth" || w != "med" || l != "med" {
		t.Fatalf("defaulted arrow = (%q,%q,%q)", k, w, l)
	}
}
