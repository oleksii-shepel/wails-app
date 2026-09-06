package word97

import "testing"

func TestNormalizeWordControlTextPromotesBreaksAndSpecialHyphens(t *testing.T) {
	got := normalizeWordControlText("a\nb\x0ec\x1ed\x1fe")
	want := "a\vb\fc\u2011d\u00ade"
	if got != want {
		t.Fatalf("normalizeWordControlText = %q, want %q", got, want)
	}
}

func TestNormalizeWordControlTextConsumesUnresolvedObjectControls(t *testing.T) {
	got := normalizeWordControlText("a\x00\x01\x02\x03\x04\x05\x06\x08b")
	if got != "ab" {
		t.Fatalf("normalizeWordControlText = %q, want ab", got)
	}
}

func TestNormalizeWordControlTextPreservesSemanticControlsForWalk(t *testing.T) {
	const input = "a\tb\vc\fd\re\af\x13g\x14h\x15i"
	if got := normalizeWordControlText(input); got != input {
		t.Fatalf("normalizeWordControlText = %q, want %q", got, input)
	}
}
