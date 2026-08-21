//go:build windows

package platform

import (
	"strings"
	"testing"
)

func TestPortFromDWORDPreservesNetworkByteOrder(t *testing.T) {
	// MIB_* tables expose ports in network byte order in the low WORD.
	if got := portFromDWORD(0x5000); got != 80 {
		t.Fatalf("portFromDWORD(0x5000) = %d, want 80", got)
	}
	if got := portFromDWORD(0xbb01); got != 443 {
		t.Fatalf("portFromDWORD(0xbb01) = %d, want 443", got)
	}
}

func TestTCPStateNames(t *testing.T) {
	if got := tcpState(2); got != "Listen" {
		t.Fatalf("tcpState(2) = %q, want Listen", got)
	}
	if got := tcpState(5); got != "Established" {
		t.Fatalf("tcpState(5) = %q, want Established", got)
	}
}

func TestNetworkSurfaceUsesNativeTables(t *testing.T) {
	records, err := ListNetworkSurface()
	if err != nil {
		t.Fatalf("ListNetworkSurface: %v", err)
	}
	for _, record := range records {
		kind := strings.TrimSpace(record.Fields["kind"].(string))
		if kind != "Adapter" && kind != "TCP" && kind != "UDP" {
			t.Fatalf("unexpected network kind %q", kind)
		}
	}
}
