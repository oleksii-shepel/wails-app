//go:build windows

package app

import (
	"os"
	"strings"
	"testing"
)

func TestNetworkAdapterBindingsRetryOnlyPermissionFailures(t *testing.T) {
	source, err := os.ReadFile("bindings_network_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)

	if !strings.Contains(text, "internalelevation.IsPermissionDenied(err)") {
		t.Fatal("network adapter mutation bindings must gate UAC retry on permission denial")
	}
	for _, op := range []string{
		"network-adapter-enable",
		"network-adapter-disable",
		"network-adapter-renew-dhcp",
	} {
		if !strings.Contains(text, op) {
			t.Fatalf("missing elevated operation %q", op)
		}
	}
	if !strings.Contains(text, "runElevatedRequest(") {
		t.Fatal("network adapter bindings must use Manager's existing elevation request path")
	}
}
