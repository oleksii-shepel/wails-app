package legacy

import (
	"bytes"
	"testing"
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
