//go:build windows

package platform

import "testing"

func TestShareTypeName(t *testing.T) {
	tests := map[uint32]string{
		0: "Disk",
		1: "Printer",
		2: "Device",
		3: "IPC",
	}
	for value, want := range tests {
		if got := shareTypeName(value); got != want {
			t.Fatalf("shareTypeName(%d) = %q, want %q", value, got, want)
		}
	}
}
