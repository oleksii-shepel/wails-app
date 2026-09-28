//go:build bindings

package app

import "testing"

func TestBindingsModeNeverRelaunchesElevated(t *testing.T) {
	handled, err := MaybeRelaunchElevatedForStartup()
	if err != nil {
		t.Fatalf("bindings startup elevation returned an error: %v", err)
	}
	if handled {
		t.Fatal("bindings mode must not relaunch the Manager elevated")
	}
}
