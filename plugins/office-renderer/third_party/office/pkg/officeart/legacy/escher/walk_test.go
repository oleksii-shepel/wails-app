package escher

import "testing"

func TestWalkStreamVisitsNestedRecords(t *testing.T) {
	child := makeRecord(2, 1, RecordFSP, make([]byte, 8))
	sp := makeRecord(ContainerVersion, 0, RecordSpContainer, child)
	var types []uint16
	if err := WalkStream(sp, func(rec Record) error {
		types = append(types, rec.Header.Type)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(types) != 2 || types[0] != RecordSpContainer || types[1] != RecordFSP {
		t.Fatalf("types = %#v", types)
	}
}

func TestScanFindsEmbeddedContainer(t *testing.T) {
	child := makeRecord(2, 1, RecordFSP, make([]byte, 8))
	sp := makeRecord(ContainerVersion, 0, RecordSpContainer, child)
	data := append([]byte{9, 8, 7, 6, 5}, sp...)
	got := Scan(data, func(h Header) bool { return h.Type == RecordSpContainer })
	if len(got) != 1 || got[0].Offset != 5 {
		t.Fatalf("scan = %#v", got)
	}
}
