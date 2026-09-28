//go:build bindings

package main

import "testing"

func TestBindingsModeNeverRelaunchesElevated(t *testing.T) {
	handled, err := maybeRelaunchElevatedForStartup()
	if err != nil {
		t.Fatalf("bindings startup elevation returned an error: %v", err)
	}
	if handled {
		t.Fatal("bindings mode must not relaunch the Manager elevated")
	}
}
