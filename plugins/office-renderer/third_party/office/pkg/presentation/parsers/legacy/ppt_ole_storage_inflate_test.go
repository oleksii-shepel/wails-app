package legacy

import (
	"bytes"
	"compress/zlib"
	"testing"
)

func TestInflateLegacyPptPayloadLimitStopsAtBound(t *testing.T) {
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	payload := bytes.Repeat([]byte{0xAB}, 64*1024)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("compress: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close compressor: %v", err)
	}

	got := inflateLegacyPptPayloadLimit(compressed.Bytes(), 1024)
	if len(got) != 1024 {
		t.Fatalf("inflated %d bytes, want the 1024-byte bound", len(got))
	}
	for i, b := range got {
		if b != 0xAB {
			t.Fatalf("byte %d = %#x, want 0xAB", i, b)
		}
	}

	full := inflateLegacyPptPayloadLimit(compressed.Bytes(), len(payload))
	if len(full) != len(payload) {
		t.Fatalf("inflated %d bytes, want %d", len(full), len(payload))
	}
}

func TestInflateLegacyPptPayloadRejectsDamagedStream(t *testing.T) {
	if got := inflateLegacyPptPayloadLimit([]byte{0x01, 0x02, 0x03}, 4096); got != nil {
		t.Fatalf("damaged stream inflated to %d bytes, want nil", len(got))
	}
	if got := inflateLegacyPptPayloadLimit(nil, 4096); got != nil {
		t.Fatalf("empty payload inflated to %d bytes, want nil", len(got))
	}
}
