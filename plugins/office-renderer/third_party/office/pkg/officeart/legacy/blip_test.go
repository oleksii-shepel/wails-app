package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/officeart/legacy/escher"
)

func TestExtractBLIPPNG(t *testing.T) {
	png := []byte{
		0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
		0, 0, 0, 0, 'I', 'E', 'N', 'D', 0, 0, 0, 0,
	}
	payload := append(make([]byte, 17), png...)
	got := ExtractBLIP(escher.RecordBlipPNG, payload)
	if got.MIME != "image/png" {
		t.Fatalf("mime = %q", got.MIME)
	}
	if len(got.Data) != len(png) {
		t.Fatalf("data len = %d want %d", len(got.Data), len(png))
	}
}

func TestSniffWMF(t *testing.T) {
	wmf := make([]byte, 18)
	binary.LittleEndian.PutUint16(wmf[0:2], 1)
	binary.LittleEndian.PutUint16(wmf[2:4], 9)
	binary.LittleEndian.PutUint16(wmf[4:6], 0x0300)
	if got := SniffImageMIME(wmf); got != "image/x-wmf" {
		t.Fatalf("mime = %q", got)
	}
}

func TestScanJPEGEndSkipsEmbeddedEOIInsideApplicationSegment(t *testing.T) {
	// APP1 deliberately contains an entire tiny JPEG-like byte sequence. The
	// inner FF D9 is metadata and must not terminate the outer JPEG.
	jpeg := []byte{
		0xFF, 0xD8,
		0xFF, 0xE1, 0x00, 0x08,
		0xFF, 0xD8, 0xFF, 0xD9, 'x', 'y',
		0xFF, 0xDA, 0x00, 0x08, 1, 1, 0, 0, 0, 0,
		0x11, 0x22, 0xFF, 0x00, 0xD9, 0x33,
		0xFF, 0xD9,
		0xAA, 0xBB,
	}
	end, ok := scanJPEGEnd(jpeg)
	if !ok {
		t.Fatal("scanJPEGEnd rejected structurally valid JPEG marker stream")
	}
	want := len(jpeg) - 2
	if end != want {
		t.Fatalf("end = %d want outer EOI at %d", end, want)
	}
}

func TestScanJPEGEndRejectsTruncatedLengthDelimitedSegment(t *testing.T) {
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x20, 1, 2, 3, 0xFF, 0xD9}
	if end, ok := scanJPEGEnd(jpeg); ok {
		t.Fatalf("scanJPEGEnd accepted truncated APP segment at %d", end)
	}
}
