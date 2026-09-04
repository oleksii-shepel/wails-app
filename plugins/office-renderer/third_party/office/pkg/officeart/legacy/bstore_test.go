package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/officeart/legacy/escher"
)

func testEscherRecord(ver uint16, inst uint16, typ uint16, body []byte) []byte {
	out := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint16(out[0:2], (inst<<4)|(ver&0xF))
	binary.LittleEndian.PutUint16(out[2:4], typ)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(body)))
	copy(out[8:], body)
	return out
}

func tinyPNG() []byte {
	return []byte{
		0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
		0, 0, 0, 0, 'I', 'E', 'N', 'D', 0, 0, 0, 0,
	}
}

func TestParseBStoreInlineBLIP(t *testing.T) {
	png := tinyPNG()
	blip := testEscherRecord(0, 0, escher.RecordBlipPNG, append(make([]byte, 17), png...))
	bseBody := append(make([]byte, 36), blip...)
	bse := testEscherRecord(2, 0, escher.RecordBSE, bseBody)

	store := ParseBStore(bse, nil)
	media, ok := store.Media(1)
	if !ok || media.MIME != "image/png" || len(media.Data) != len(png) {
		t.Fatalf("media = %#v ok=%v", media, ok)
	}
}

func TestParseBSEDelayedBLIP(t *testing.T) {
	png := tinyPNG()
	raw := testEscherRecord(0, 0, escher.RecordBlipPNG, append(make([]byte, 17), png...))
	body := make([]byte, 36)
	binary.LittleEndian.PutUint32(body[20:24], uint32(len(raw)))
	binary.LittleEndian.PutUint32(body[28:32], 123)

	entry, ok := ParseBSE(body, func(offset, size uint32) []byte {
		if offset != 123 || size != uint32(len(raw)) {
			t.Fatalf("resolver offset=%d size=%d", offset, size)
		}
		return raw
	})
	if !ok || !entry.Delayed || entry.Media.MIME != "image/png" {
		t.Fatalf("entry = %#v ok=%v", entry, ok)
	}
}
