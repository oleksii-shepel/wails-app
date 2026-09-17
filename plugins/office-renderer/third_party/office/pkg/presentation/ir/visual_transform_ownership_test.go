package ir

import (
	"os"
	"path/filepath"
	"testing"
)

// Canonical visual composition is presentation IR. Keep a single format-neutral
// transform model rather than allowing a parallel presentation/composition model
// to grow beside IR.
func TestCanonicalVisualCompositionHasSingleIROwner(t *testing.T) {
	legacy := filepath.Join("..", "composition")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("parallel canonical composition package still exists at %s; visual composition belongs to presentation/ir", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", legacy, err)
	}
}
