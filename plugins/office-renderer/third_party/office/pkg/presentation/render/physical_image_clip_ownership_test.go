package render

import (
	"os"
	"strings"
	"testing"
)

func TestPictureGeometryClipWrapsDestinationFrame(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, required := range []string{
		`shape__image-clip`,
		`imageClipStyles(imageVisual)`,
		`"inset:0"`,
		`"clip-path:"+visual.ClipPath`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("picture renderer must serialize outer picture geometry clip %q", required)
		}
	}
	frameStart := strings.Index(source, "func imageFrameStyles")
	frameEnd := strings.Index(source[frameStart:], "func imageElementStyles")
	if frameStart < 0 || frameEnd < 0 {
		t.Fatal("image frame style helpers not found")
	}
	frameSource := source[frameStart : frameStart+frameEnd]
	if strings.Contains(frameSource, "clip-path:") {
		t.Fatal("fillRect frame must not redefine picture-shape clip geometry")
	}
}
