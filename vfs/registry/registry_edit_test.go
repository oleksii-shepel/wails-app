//go:build windows

package registry

import "testing"

func TestParseRegistryIntegerAcceptsDisplayedDWORD(t *testing.T) {
	got, err := parseRegistryInteger("2 (0x00000002)", 32)
	if err != nil {
		t.Fatal(err)
	}
	if got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
}

func TestParseRegistryBytes(t *testing.T) {
	got, err := parseRegistryBytes("DE AD BE EF")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if len(got) != len(want) {
		t.Fatalf("got %d bytes, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d: got %02X, want %02X", i, got[i], want[i])
		}
	}
}
