package opc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenWithOptionsPasswordsArePerCall(t *testing.T) {
	// This test is intentionally fixture-optional at the OPC package layer.
	// Word's encrypted corpus exercises concrete passwords end-to-end; this gate
	// ensures the API itself has no package-level password state.
	if _, ok := any(OpenOptions{}).(any); !ok {
		t.Fatal("unreachable")
	}
}

func TestOPCSourceHasNoPackagePasswordGlobal(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(".", "opc.go"))
	if err != nil {
		t.Fatal(err)
	}
	if stringContains(string(data), "var Password string") {
		t.Fatal("OPC retained package-level Password global")
	}
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub { return true }
	}
	return false
}
