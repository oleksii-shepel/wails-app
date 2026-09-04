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
