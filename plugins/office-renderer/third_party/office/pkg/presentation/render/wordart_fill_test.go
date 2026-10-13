package render

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	playout "office/pkg/presentation/layout"
)

func encodedPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 0x99, G: 0x66, B: 0x33, A: 0xFF})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// encodedJPEGWithDPI writes a real JPEG and then stamps a JFIF density into its
// APP0 segment, which is where a texture's natural resolution is recorded.
func encodedJPEGWithDPI(t *testing.T, w, h, dpi int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			i++
			continue
		}
		length := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if data[i+1] == 0xE0 && string(data[i+4:i+9]) == "JFIF\x00" {
			data[i+11] = 1 // units: dots per inch
			binary.BigEndian.PutUint16(data[i+12:i+14], uint16(dpi))
			binary.BigEndian.PutUint16(data[i+14:i+16], uint16(dpi))
			return data
		}
		i += 2 + length
	}
	// Go's encoder writes no JFIF segment; a camera's JPEG would carry one, so
	// splice a well-formed one in right after the SOI marker.
	app0 := []byte{0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x00, 0x4B, 0x00, 0x4B, 0x00, 0x00}
	binary.BigEndian.PutUint16(app0[12:14], uint16(dpi))
	binary.BigEndian.PutUint16(app0[14:16], uint16(dpi))
	out := append([]byte(nil), data[:2]...)
	out = append(out, app0...)
	return append(out, data[2:]...)
}

func TestPictureTileSizePtUsesImageResolution(t *testing.T) {
	tests := []struct {
		name           string
		data           []byte
		scaleX, scaleY int
		wantW, wantH   float64
	}{
		{name: "default 96 dpi", data: encodedPNG(t, 128, 64), wantW: 96, wantH: 48},
		{name: "jpeg tagged 75 dpi", data: encodedJPEGWithDPI(t, 128, 128, 75), wantW: 122.88, wantH: 122.88},
		{name: "authored tile scale", data: encodedJPEGWithDPI(t, 128, 128, 75), scaleX: 50000, scaleY: 200000, wantW: 61.44, wantH: 245.76},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fill := drawing.PictureFill{ImageData: tt.data, ContentType: "image/png", Tile: true, TileScaleX: tt.scaleX, TileScaleY: tt.scaleY}
			gotW, gotH := pictureTileSizePt(fill)
			if math.Abs(gotW-tt.wantW) > 0.01 || math.Abs(gotH-tt.wantH) > 0.01 {
				t.Fatalf("tile size = %.2fx%.2f pt, want %.2fx%.2f", gotW, gotH, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestWordArtPatternFillDefinitionConvertsTileToViewportUnits(t *testing.T) {
	fill := drawing.PictureFill{ImageData: encodedPNG(t, 128, 128), ContentType: "image/png", Tile: true}
	// A 128 px tile at the default 96 dpi is 96 pt; a shape 192 x 96 pt has one
	// viewport unit worth 1.92 pt horizontally and 0.96 pt vertically.
	def := wordArtPatternFillDefinition(fill, "wordart-fill-0", 192, 96, 1)
	for _, want := range []string{`id="wordart-fill-0"`, `patternUnits="userSpaceOnUse"`, `width="50.0000"`, `height="100.0000"`, `href="data:image/png;base64,`} {
		if !strings.Contains(def, want) {
			t.Fatalf("pattern definition missing %q: %s", want, def)
		}
	}
}

func TestWordArtPatternFillDefinitionUndoesGlyphStretch(t *testing.T) {
	fill := drawing.PictureFill{ImageData: encodedPNG(t, 128, 128), ContentType: "image/png", Tile: true}
	// The pattern is resolved in the stretched text's coordinate system, so the
	// tile is scaled back out to keep the texture square on screen.
	def := wordArtPatternFillDefinition(fill, "wordart-fill-0", 192, 96, 2)
	if !strings.Contains(def, `height="50.0000"`) {
		t.Fatalf("stretched pattern height not compensated: %s", def)
	}
}

func TestWordArtPatternFillDefinitionRequiresSource(t *testing.T) {
	if def := wordArtPatternFillDefinition(drawing.PictureFill{Tile: true}, "id", 100, 100, 1); def != "" {
		t.Fatalf("pattern without an image = %q, want empty", def)
	}
}

func TestWordArtFitToShapeScaleStretchesCapitalsToTheBox(t *testing.T) {
	// A 54 pt face is 72 viewport units tall; its capitals (0.72 em) must reach
	// 100 units, so the glyphs scale by 100/(72*0.72).
	scale, ok := wordArtFitToShapeScale(playout.TextGeometryVisualPlan{FitToShape: true, FontSizePt: 54})
	if !ok {
		t.Fatal("fit scale not reported for legacy WordArt")
	}
	if math.Abs(scale-100.0/(72*0.72)) > 1e-9 {
		t.Fatalf("fit scale = %.4f, want %.4f", scale, 100.0/(72*0.72))
	}
	if _, ok := wordArtFitToShapeScale(playout.TextGeometryVisualPlan{FontSizePt: 54}); ok {
		t.Fatal("non-WordArt text must not be stretched")
	}
	if _, ok := wordArtFitToShapeScale(playout.TextGeometryVisualPlan{FitToShape: true}); ok {
		t.Fatal("text without an authored size has no base to stretch from")
	}
}
