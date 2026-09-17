package render

import (
	"strings"
	"testing"
)

func TestLucidaSansUnicodeDoesNotReceiveMonospaceFallback(t *testing.T) {
	got := cssFontFamilyValue("Lucida Sans Unicode", true)
	if strings.Contains(got, "Consolas") || strings.Contains(got, "Courier New") || strings.Contains(got, "monospace") {
		t.Fatalf("Lucida Sans Unicode fallback stack = %q, must remain proportional", got)
	}
	if !strings.Contains(got, "Arial") || !strings.Contains(got, "sans-serif") {
		t.Fatalf("Lucida Sans Unicode fallback stack = %q, want proportional sans-serif fallbacks", got)
	}
}

func TestPlayfairDisplayFallbackStaysInSerifMetricClass(t *testing.T) {
	got := cssFontFamilyValue("Playfair Display", true)
	if strings.Contains(got, "Arial Narrow") || strings.Contains(got, "sans-serif") {
		t.Fatalf("Playfair Display fallback stack = %q, must not switch to condensed sans metrics", got)
	}
	if !strings.Contains(got, "Times New Roman") || !strings.Contains(got, "serif") {
		t.Fatalf("Playfair Display fallback stack = %q, want serif metric-compatible fallbacks", got)
	}
}

func TestCambriaFallbackStaysInSerifMetricClass(t *testing.T) {
	got := cssFontFamilyValue("Cambria", true)
	if !strings.Contains(got, "Times New Roman") || !strings.Contains(got, "serif") {
		t.Fatalf("Cambria fallback stack = %q, want serif metric-compatible fallbacks", got)
	}
}
