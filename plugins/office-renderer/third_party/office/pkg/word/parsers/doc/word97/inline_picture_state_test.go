package word97

import (
	"encoding/binary"
	"testing"
)

func TestResolveInlinePictureClassifiesRecognizedPICFWithoutMediaAsPlaceholder(t *testing.T) {
	data := make([]byte, inlinePICFHeaderSize+50)
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(data)))
	binary.LittleEndian.PutUint16(data[4:6], inlinePICFHeaderSize)
	binary.LittleEndian.PutUint16(data[6:8], 0x0064)
	binary.LittleEndian.PutUint16(data[14:16], 0x0000)

	image, state := resolveInlinePicture(data, 0)
	if image != nil {
		t.Fatalf("image = %#v, want nil placeholder", image)
	}
	if state != inlinePicturePlaceholder {
		t.Fatalf("state = %v, want placeholder", state)
	}
}

func TestResolveInlinePictureRejectsMalformedRecognizedPICF(t *testing.T) {
	data := make([]byte, inlinePICFHeaderSize)
	binary.LittleEndian.PutUint32(data[0:4], 8)
	binary.LittleEndian.PutUint16(data[4:6], inlinePICFHeaderSize)
	binary.LittleEndian.PutUint16(data[6:8], 0x0064)
	binary.LittleEndian.PutUint16(data[14:16], 0x0000)

	image, state := resolveInlinePicture(data, 0)
	if image != nil {
		t.Fatalf("image = %#v, want nil invalid picture", image)
	}
	if state != inlinePictureInvalid {
		t.Fatalf("state = %v, want invalid", state)
	}
}
