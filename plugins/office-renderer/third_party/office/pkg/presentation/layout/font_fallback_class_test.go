package layout

import "testing"

func TestResolveFontFallbackClassMatchesMetricFamilies(t *testing.T) {
	cases := map[string]FontFallbackClass{
		"Playfair Display":    FontFallbackSerif,
		"Cambria":             FontFallbackSerif,
		"Source Code Pro":     FontFallbackMono,
		"My Mono Office Face": FontFallbackMono,
		"Oswald":              FontFallbackCondensed,
		"Arial Condensed":     FontFallbackCondensed,
		"Lucida Sans Unicode": FontFallbackSans,
		"Some Missing Font":   FontFallbackSans,
	}
	for family, want := range cases {
		if got := ResolveFontFallbackClass(family); got != want {
			t.Fatalf("ResolveFontFallbackClass(%q) = %q, want %q", family, got, want)
		}
	}
}
