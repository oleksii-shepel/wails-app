package main

import "testing"

func TestInlinePreviewAlwaysShowsDataAndBoundsLargeValues(t *testing.T) {
	got := inlinePreview(regDWORD, []byte{1, 0, 0, 0})
	if got != "1" {
		t.Fatalf("DWORD preview = %q, want 1", got)
	}

	long := make([]byte, 600)
	for i := range long {
		long[i] = 'A'
	}
	got = inlinePreview(regBinary, long)
	if got == "" {
		t.Fatal("large Registry value preview must not be empty")
	}
	if len([]rune(got)) > 241 {
		t.Fatalf("preview too long: %d runes", len([]rune(got)))
	}
}
