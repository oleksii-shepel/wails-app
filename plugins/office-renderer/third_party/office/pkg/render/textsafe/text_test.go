package textsafe

import "testing"

func TestVisibleDropsUnresolvedControls(t *testing.T) {
	got := Visible("A\x00\x01\x02B\x08C\x0bD\x0cE\x13F\x14G\x15H\x7fI")
	if got != "ABCDEFGHI" {
		t.Fatalf("Visible = %q, want %q", got, "ABCDEFGHI")
	}
}

func TestVisiblePreservesTextWhitespaceAndUnicode(t *testing.T) {
	const input = "one\ttwo\nthree\rfour — ✓"
	if got := Visible(input); got != input {
		t.Fatalf("Visible = %q, want %q", got, input)
	}
}
