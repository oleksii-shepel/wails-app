package legacy

import (
	"testing"

	"office/pkg/officeart/legacy/escher"
)

func TestDecodeVisualPropertiesCarriesPictureAdjustments(t *testing.T) {
	got := DecodeVisualProperties([]escher.Property{
		{ID: escher.PropPictureBrightness, Value: uint32(int32(16384))},
		{ID: escher.PropPictureContrast, Value: 32768},
		{ID: escher.PropPictureBooleans, Value: uint32(0x0004 | (0x0004 << 16))},
	}, nil)
	if !got.HasPictureBrightness || got.PictureBrightness != 1.5 {
		t.Fatalf("brightness=%v present=%v", got.PictureBrightness, got.HasPictureBrightness)
	}
	if !got.HasPictureContrast || got.PictureContrast != .5 {
		t.Fatalf("contrast=%v present=%v", got.PictureContrast, got.HasPictureContrast)
	}
	if !got.HasPictureGrayscale || !got.PictureGrayscale {
		t.Fatalf("grayscale=%v present=%v", got.PictureGrayscale, got.HasPictureGrayscale)
	}
}
