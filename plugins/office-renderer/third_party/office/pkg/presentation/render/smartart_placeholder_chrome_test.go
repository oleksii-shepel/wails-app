package render

import (
	"os"
	"strings"
	"testing"
)

func TestSmartArtRendererDoesNotSynthesizeEditorPicturePlaceholderChrome(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "smartart-picture-placeholder") {
		t.Fatal("HTML renderer must not synthesize editor-only picture placeholder glyphs")
	}
}
