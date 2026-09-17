package legacy

import (
	"encoding/binary"
	"reflect"
	"testing"
)

func TestWalkRecordEventsIsForwardNestedAndBalanced(t *testing.T) {
	atom := pptRecordBytes(0, 0, pptRTTextCharsAtom, []byte{1, 2})
	container := pptRecordBytes(0x0f, 0, pptRTSlide, atom)
	var got []uint16
	var starts []bool
	var depths []int
	if err := WalkRecordEvents(container, RecordConsumerFunc(func(e RecordEvent) error {
		got = append(got, e.Type)
		starts = append(starts, e.Start)
		depths = append(depths, e.Depth)
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []uint16{pptRTSlide, pptRTTextCharsAtom, pptRTSlide}) {
		t.Fatalf("types=%v", got)
	}
	if !reflect.DeepEqual(starts, []bool{true, false, false}) {
		t.Fatalf("starts=%v", starts)
	}
	if !reflect.DeepEqual(depths, []int{0, 1, 0}) {
		t.Fatalf("depths=%v", depths)
	}
}

func pptRecordBytes(version, instance, typ uint16, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(b[0:2], (instance<<4)|(version&0x0f))
	binary.LittleEndian.PutUint16(b[2:4], typ)
	binary.LittleEndian.PutUint32(b[4:8], uint32(len(payload)))
	copy(b[8:], payload)
	return b
}
