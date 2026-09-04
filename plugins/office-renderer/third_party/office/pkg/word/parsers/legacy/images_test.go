package legacy

import (
	"bytes"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDetectLegacyImageMIME(t *testing.T) {
	if got := detectLegacyImageMIME(tinyPNGBytes()); got != "image/png" {
		t.Fatalf("png mime = %q, want image/png", got)
	}
	if got := detectLegacyImageMIME(tinyJPEGBytes()); got != "image/jpeg" {
		t.Fatalf("jpeg mime = %q, want image/jpeg", got)
	}
	if got := detectLegacyImageMIME(tinyGIFBytes()); got != "image/gif" {
		t.Fatalf("gif mime = %q, want image/gif", got)
	}
	if got := detectLegacyImageMIME(tinyBMPBytes()); got != "image/bmp" {
		t.Fatalf("bmp mime = %q, want image/bmp", got)
	}
}

func TestDetectLegacyImageDimensions(t *testing.T) {
	for _, tt := range []struct {
		name string
		blob []byte
		mime string
		w    int
		h    int
	}{
		{name: "png", blob: tinyPNGBytes(), mime: "image/png", w: 1, h: 1},
		{name: "jpeg", blob: tinyJPEGBytes(), mime: "image/jpeg", w: 1, h: 1},
		{name: "gif", blob: tinyGIFBytes(), mime: "image/gif", w: 1, h: 1},
		{name: "bmp", blob: tinyBMPBytes(), mime: "image/bmp", w: 1, h: 1},
	} {
		w, h := detectLegacyImageDimensions(tt.blob, tt.mime)
		if w != tt.w || h != tt.h {
			t.Fatalf("%s dims = (%d,%d), want (%d,%d)", tt.name, w, h, tt.w, tt.h)
		}
	}
}

func TestScanLegacyImageBlobsFindsMultipleFormats(t *testing.T) {
	stream := append([]byte("prefix"), tinyPNGBytes()...)
	stream = append(stream, []byte("middle")...)
	stream = append(stream, tinyJPEGBytes()...)
	stream = append(stream, []byte("tail")...)
	blobs := scanLegacyImageBlobs(stream)
	if len(blobs) != 2 {
		t.Fatalf("blob count = %d, want 2", len(blobs))
	}
	if !bytes.Equal(blobs[0], tinyPNGBytes()) {
		t.Fatal("first blob is not png payload")
	}
	if !bytes.Equal(blobs[1], tinyJPEGBytes()) {
		t.Fatal("second blob is not jpeg payload")
	}
}

func tinyJPEGBytes() []byte {
	return []byte{
		0xFF, 0xD8,
		0xFF, 0xE0, 0x00, 0x10,
		'J', 'F', 'I', 'F', 0x00,
		0x01, 0x01, 0x00,
		0x00, 0x01, 0x00, 0x01,
		0x00, 0x00,
		0xFF, 0xC0, 0x00, 0x11,
		0x08, 0x00, 0x01, 0x00, 0x01,
		0x03,
		0x01, 0x11, 0x00,
		0x02, 0x11, 0x00,
		0x03, 0x11, 0x00,
		0xFF, 0xD9,
	}
}

func tinyGIFBytes() []byte {
	return []byte{
		'G', 'I', 'F', '8', '9', 'a',
		0x01, 0x00, 0x01, 0x00,
		0x80, 0x00, 0x00,
		0x00, 0x00, 0x00,
		0xFF, 0xFF, 0xFF,
		0x21, 0xF9, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x2C, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00,
		0x00,
		0x02, 0x02, 0x44, 0x01, 0x00,
		0x3B,
	}
}

func tinyBMPBytes() []byte {
	return []byte{
		'B', 'M',
		58, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		54, 0x00, 0x00, 0x00,
		40, 0x00, 0x00, 0x00,
		1, 0x00, 0x00, 0x00,
		1, 0x00, 0x00, 0x00,
		1, 0x00,
		24, 0x00,
		0, 0x00, 0x00, 0x00,
		4, 0x00, 0x00, 0x00,
		0x13, 0x0B, 0x00, 0x00,
		0x13, 0x0B, 0x00, 0x00,
		0, 0x00, 0x00, 0x00,
		0, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
}
func TestApplyLegacyInlinePictureDescriptorPreservesWordPictureAdjustments(t *testing.T) {
	img := &wordir.Image{}
	applyLegacyInlinePictureDescriptor(img, legacyInlinePictureDescriptor{
		GoalWidthTwips:  1500,
		GoalHeightTwips: 1125,
		ScaleXPermille:  500,
		ScaleYPermille:  800,
		Props: legacyOfficeArtShapeProps{
			CropTop: 0.07, CropRight: 0.06, CropBottom: 0.08, CropLeft: 0.05,
			Brightness: 1.7, Contrast: 0.3, Grayscale: true, BiLevel: true,
		},
	})
	if got := float64(img.Width) / 12700.0; got != 37.5 {
		t.Fatalf("width = %.1fpt, want 37.5pt", got)
	}
	if got := float64(img.Height) / 12700.0; got != 45.0 {
		t.Fatalf("height = %.1fpt, want 45.0pt", got)
	}
	if !img.Stretch {
		t.Fatal("non-uniform PICF scaling must mark image as stretched")
	}
	if !img.Grayscale || !img.BiLevel || img.Brightness != 1.7 || img.Contrast != 0.3 {
		t.Fatalf("picture adjustments not preserved: %+v", img)
	}
	if img.CropTop != 0.07 || img.CropRight != 0.06 || img.CropBottom != 0.08 || img.CropLeft != 0.05 {
		t.Fatalf("crop not preserved: %+v", img)
	}
}
