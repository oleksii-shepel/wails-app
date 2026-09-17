package render

import (
	"os"
	"strings"
	"testing"
)

func TestRendererDoesNotReinterpretCanonicalPictureCropOrFillRect(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{"shape.ImageCropLeft", "shape.ImageCropTop", "shape.ImageCropRight", "shape.ImageCropBottom", "shape.ImageFillLeft", "shape.ImageFillTop", "shape.ImageFillRight", "shape.ImageFillBottom"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer reinterprets canonical picture composition through %q", forbidden)
		}
	}
	if !strings.Contains(src, "item.Image") {
		t.Fatal("renderer must consume the layout-owned physical image plan")
	}
}
