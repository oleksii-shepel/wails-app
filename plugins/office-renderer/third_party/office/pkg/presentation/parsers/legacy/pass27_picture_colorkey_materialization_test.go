package legacy

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/imageutil"
)

func TestPass27MaterializesOfficeArtPictureColorKeyIntoRasterAlpha(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{B: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}

	data, mime, effects := imageutil.MaterializeTransparentColorKeys(encoded.Bytes(), "image/png", []drawing.Effect{
		drawing.ClrChange{From: "FF0000", To: "FF0000", UseAlpha: true, ToAlpha: 0},
	})
	if mime != "image/png" {
		t.Fatalf("mime = %q", mime)
	}
	if len(effects) != 0 {
		t.Fatalf("materialized color key remained as renderer effect: %#v", effects)
	}
	got, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, a0 := got.At(0, 0).RGBA()
	_, _, b1, a1 := got.At(1, 0).RGBA()
	if a0 != 0 {
		t.Fatalf("keyed red alpha = %d, want 0", a0)
	}
	if a1 != 0xffff || b1 != 0xffff {
		t.Fatalf("non-keyed blue = blue:%d alpha:%d", b1, a1)
	}
}

func TestPass27KeepsNonColorKeyEffectsAfterRasterMaterialization(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}
	effects := []drawing.Effect{
		drawing.ClrChange{From: "FFFFFF", To: "FFFFFF", UseAlpha: true, ToAlpha: 0},
		drawing.Lum{Brightness: 12000, Contrast: 5000},
	}
	_, _, remaining := imageutil.MaterializeTransparentColorKeys(encoded.Bytes(), "image/png", effects)
	if len(remaining) != 1 || remaining[0].Type() != drawing.EffectTypeLum {
		t.Fatalf("remaining effects = %#v", remaining)
	}
}

func TestPass27UnsupportedRasterRetainsColorKeyFallback(t *testing.T) {
	effects := []drawing.Effect{drawing.ClrChange{From: "FF0000", To: "FF0000", UseAlpha: true, ToAlpha: 0}}
	data := []byte("not-an-image")
	gotData, gotMIME, gotEffects := imageutil.MaterializeTransparentColorKeys(data, "application/octet-stream", effects)
	if !bytes.Equal(gotData, data) || gotMIME != "application/octet-stream" || len(gotEffects) != 1 {
		t.Fatalf("unsupported raster fallback was consumed")
	}
}
