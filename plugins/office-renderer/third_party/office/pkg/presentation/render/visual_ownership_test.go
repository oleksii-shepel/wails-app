package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLDoesNotOwnPictureCropOrFillRectMaterialization(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"shape.ImageCropLeft", "shape.ImageCropTop", "shape.ImageCropRight", "shape.ImageCropBottom",
		"shape.ImageFillLeft", "shape.ImageFillTop", "shape.ImageFillRight", "shape.ImageFillBottom",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("HTML renderer still owns canonical picture placement via %q", forbidden)
		}
	}
}
