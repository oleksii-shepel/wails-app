package escher

import (
	"encoding/binary"
	"testing"
)

func TestParseFOPTComplexProperty(t *testing.T) {
	data := make([]byte, 6+3)
	opid := uint16(0x0123) | propertyComplexFlag
	binary.LittleEndian.PutUint16(data[0:2], opid)
	binary.LittleEndian.PutUint32(data[2:6], 3)
	copy(data[6:], []byte{1, 2, 3})

	props, err := ParseFOPT(data, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 1 || props[0].ID != 0x0123 || !props[0].IsComplex {
		t.Fatalf("props = %#v", props)
	}
	if got := props[0].Complex; len(got) != 3 || got[2] != 3 {
		t.Fatalf("complex = %v", got)
	}
}
