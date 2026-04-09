package converter

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyDocumentFormat(t *testing.T) {
	tests := []struct {
		ext  string
		want documentFormatKind
	}{
		{ext: ".docx", want: documentFormatOffice},
		{ext: "epub", want: documentFormatEpub},
		{ext: "djvu", want: documentFormatDjvu},
		{ext: "txt", want: documentFormatUnknown},
	}

	for _, tt := range tests {
		if got := classifyDocumentFormat(tt.ext); got != tt.want {
			t.Fatalf("classifyDocumentFormat(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}

func TestNormalizeOutputExtension(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{format: "jpeg", want: "jpg"},
		{format: "tiff", want: "tif"},
		{format: "PNG", want: "png"},
		{format: "webp", want: "webp"},
	}

	for _, tt := range tests {
		if got := normalizeOutputExtension(tt.format); got != tt.want {
			t.Fatalf("normalizeOutputExtension(%q) = %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestConvertFormatFallsBackToJPEG(t *testing.T) {
	service := &Service{}
	service.detectOnce.Do(func() {})

	inputPath := writePNGFixture(t)
	converted, err := service.ConvertFormat(inputPath, "jpeg")
	if err != nil {
		t.Fatalf("ConvertFormat returned error: %v", err)
	}

	_, format, err := image.Decode(bytes.NewReader(converted))
	if err != nil {
		t.Fatalf("image.Decode returned error: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("decoded format = %q, want %q", format, "jpeg")
	}
}

func TestCreateThumbnailFallsBackToJPEG(t *testing.T) {
	service := &Service{}
	service.detectOnce.Do(func() {})

	inputPath := writePNGFixture(t)
	thumbnail, err := service.CreateThumbnail(inputPath, "jpeg", 8, 8)
	if err != nil {
		t.Fatalf("CreateThumbnail returned error: %v", err)
	}

	img, format, err := image.Decode(bytes.NewReader(thumbnail))
	if err != nil {
		t.Fatalf("image.Decode returned error: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("decoded format = %q, want %q", format, "jpeg")
	}
	if img.Bounds().Dx() > 8 || img.Bounds().Dy() > 8 {
		t.Fatalf("thumbnail bounds = %v, want max 8x8", img.Bounds())
	}
}

func writePNGFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 20), B: 180, A: 255})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("os.Create returned error: %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode returned error: %v", err)
	}

	return path
}
