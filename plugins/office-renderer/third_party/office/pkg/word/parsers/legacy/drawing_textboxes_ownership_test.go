package legacy

import (
	"encoding/binary"
	"testing"
)

func TestParseLegacyTextboxPLCUsesFTXBXSLid(t *testing.T) {
	plc := make([]byte, 4*2+22)
	binary.LittleEndian.PutUint32(plc[0:4], 3)
	binary.LittleEndian.PutUint32(plc[4:8], 9)
	binary.LittleEndian.PutUint32(plc[8+14:8+18], 4242)

	got := parseLegacyTextboxPLC(plc, false)
	if len(got) != 1 {
		t.Fatalf("got %#v", got)
	}
	if got[0].StartCP != 3 || got[0].EndCP != 9 || got[0].ShapeID != 4242 {
		t.Fatalf("got %#v", got[0])
	}
}
