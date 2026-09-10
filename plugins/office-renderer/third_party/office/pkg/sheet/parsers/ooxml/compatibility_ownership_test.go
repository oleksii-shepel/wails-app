package ooxml

import (
	"os"
	"strings"
	"testing"
)

func TestCompatibilityOwnershipInventoryIsGone(t *testing.T) {
	data, err := os.ReadFile("compatibility_ownership.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"type compatibilityOwnership struct", "remainingCompatibilityOwnership",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("obsolete compatibility ownership inventory remains: %q", forbidden)
		}
	}
}
