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

func TestParseFOPTArrayPropertyCanExcludeSixByteArrayHeaderFromComplexSize(t *testing.T) {
	// Real OfficeArt freeforms commonly encode pVertices this way: op equals
	// nElems*actualElementSize, while the serialized complex payload still
	// carries the six-byte array header. A following complex property proves
	// that ParseFOPT advances by the full payload, not the truncated op value.
	const propCount = 2
	tableBytes := propCount * 6
	vertices := make([]byte, 6+2*4)
	binary.LittleEndian.PutUint16(vertices[0:2], 2)
	binary.LittleEndian.PutUint16(vertices[2:4], 2)
	binary.LittleEndian.PutUint16(vertices[4:6], 0xFFF0) // actual element size = 4
	copy(vertices[6:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
	segments := make([]byte, 6+2*2)
	binary.LittleEndian.PutUint16(segments[0:2], 2)
	binary.LittleEndian.PutUint16(segments[2:4], 2)
	binary.LittleEndian.PutUint16(segments[4:6], 2)
	copy(segments[6:], []byte{9, 10, 11, 12})

	data := make([]byte, tableBytes+len(vertices)+len(segments))
	binary.LittleEndian.PutUint16(data[0:2], PropVertices|propertyComplexFlag|propertyBlipFlag)
	binary.LittleEndian.PutUint32(data[2:6], 8) // excludes array header
	binary.LittleEndian.PutUint16(data[6:8], PropSegmentInfo|propertyComplexFlag|propertyBlipFlag)
	binary.LittleEndian.PutUint32(data[8:12], uint32(len(segments))) // includes header
	copy(data[tableBytes:], vertices)
	copy(data[tableBytes+len(vertices):], segments)

	props, err := ParseFOPT(data, propCount)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(props[0].Complex); got != len(vertices) {
		t.Fatalf("vertices complex bytes = %d, want %d", got, len(vertices))
	}
	if got := len(props[1].Complex); got != len(segments) {
		t.Fatalf("segments complex bytes = %d, want %d", got, len(segments))
	}
	if props[1].Complex[6] != 9 {
		t.Fatalf("following complex property is misaligned: %v", props[1].Complex)
	}
}

func TestOfficeArtArrayElementSizeDecodesNegativeCompactEncoding(t *testing.T) {
	if got := OfficeArtArrayElementSize(0xFFF0); got != 4 {
		t.Fatalf("0xFFF0 element size = %d, want 4", got)
	}
	if got := OfficeArtArrayElementSize(8); got != 8 {
		t.Fatalf("8 element size = %d, want 8", got)
	}
}
