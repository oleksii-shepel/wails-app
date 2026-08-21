//go:build windows

package platform

import "testing"

func TestAdapterStatusLabels(t *testing.T) {
	if got := adapterOperStatus(1); got != "Up" {
		t.Fatalf("adapterOperStatus(1) = %q", got)
	}
	if got := adapterAdminStatus(ifAdminStatusDown); got != "Disabled" {
		t.Fatalf("adapterAdminStatus(down) = %q", got)
	}
}

func TestFormatMAC(t *testing.T) {
	got := formatMAC([]byte{0x00, 0x11, 0x22, 0xaa, 0xbb, 0xcc})
	if got != "00:11:22:AA:BB:CC" {
		t.Fatalf("formatMAC = %q", got)
	}
}

func TestSpeedMaximum(t *testing.T) {
	if got := maxU64(100, 200); got != 200 {
		t.Fatalf("maxU64 = %d", got)
	}
}
