package legacy

import (
	"errors"
	"os"
	"testing"
)

func TestLegacyTextboxProjectionDoesNotDecodeWord97Structure(t *testing.T) {
	_, err := os.Stat("drawing_textboxes.go")
	if err == nil {
		t.Fatal("legacy detached textbox projection must stay removed")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat drawing_textboxes.go: %v", err)
	}
}
