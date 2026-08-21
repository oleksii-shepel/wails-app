//go:build windows

package app

import (
	"strings"
	"testing"

	internalelevation "wails-app/internal/elevation"
)

func TestElevatedNetworkAdapterIndex(t *testing.T) {
	index, err := parseElevatedNetworkAdapterIndex("42")
	if err != nil {
		t.Fatal(err)
	}
	if index != 42 {
		t.Fatalf("index = %d, want 42", index)
	}

	for _, value := range []string{"", "0", "-1", "nope"} {
		if _, err := parseElevatedNetworkAdapterIndex(value); err == nil {
			t.Fatalf("parseElevatedNetworkAdapterIndex(%q) unexpectedly succeeded", value)
		}
	}
}

func TestElevatedNetworkAdapterOperationsAreRecognized(t *testing.T) {
	// Invalid adapter indexes prove the wrapper recognizes the operation rather
	// than falling through to the filesystem handler's "unknown elevated op".
	for _, op := range []string{
		"network-adapter-enable",
		"network-adapter-disable",
		"network-adapter-renew-dhcp",
	} {
		response := elevatedRawHandler(internalelevation.Request{
			Op:   op,
			Path: "not-an-index",
		})
		if response.Success {
			t.Fatalf("%s unexpectedly succeeded", op)
		}
		if strings.Contains(response.Error, "unknown elevated op") {
			t.Fatalf("%s fell through to filesystem elevated handler: %s", op, response.Error)
		}
	}
}
