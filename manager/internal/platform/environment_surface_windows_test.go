//go:build windows

package platform

import "testing"

func TestEnvironmentRegistryValueType(t *testing.T) {
	if got := environmentRegistryValueType(2); got != "Expandable" {
		t.Fatalf("EXPAND_SZ = %q", got)
	}
	if got := environmentRegistryValueType(1); got != "String" {
		t.Fatalf("SZ = %q", got)
	}
}

func TestEnvironmentScopeOrder(t *testing.T) {
	if environmentScopeOrder("Process") >= environmentScopeOrder("User") {
		t.Fatal("Process must sort before User")
	}
	if environmentScopeOrder("User") >= environmentScopeOrder("System") {
		t.Fatal("User must sort before System")
	}
}
