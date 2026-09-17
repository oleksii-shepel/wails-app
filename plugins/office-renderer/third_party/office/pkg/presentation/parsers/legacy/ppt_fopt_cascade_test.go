package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func classLegacyRect32(left, top, right, bottom int32) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint32(b[0:4], uint32(left))
	binary.LittleEndian.PutUint32(b[4:8], uint32(top))
	binary.LittleEndian.PutUint32(b[8:12], uint32(right))
	binary.LittleEndian.PutUint32(b[12:16], uint32(bottom))
	return b
}

func classLegacyClientAnchor(top, left, right, bottom int16) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint16(b[0:2], uint16(top))
	binary.LittleEndian.PutUint16(b[2:4], uint16(left))
	binary.LittleEndian.PutUint16(b[4:6], uint16(right))
	binary.LittleEndian.PutUint16(b[6:8], uint16(bottom))
	return b
}

func TestLegacyPptGroupMetaAppliesTertiaryFOPTOverride(t *testing.T) {
	rotation := func(deg int32) []byte {
		body := make([]byte, 6)
		binary.LittleEndian.PutUint16(body[0:2], legacyOfficeArtPropRotation)
		binary.LittleEndian.PutUint32(body[2:6], uint32(deg*65536))
		return body
	}
	spgr := buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0, classLegacyRect32(0, 0, 1000, 1000))
	sp := buildOfficeArtRecord(1, legacyOfficeArtSp, 0, officeArtSpPayload(0x900, 0x01))
	primary := buildOfficeArtRecord(1, officeartescher.RecordFOPT, 0x0003, rotation(15))
	tertiary := buildOfficeArtRecord(1, officeartescher.RecordTertiaryFOPT, 0x0003, rotation(30))
	anchor := buildOfficeArtRecord(0, legacyOfficeArtClientAnchor, 0, classLegacyClientAnchor(0, 0, 1000, 1000))
	payload := append(append(append(append(spgr, sp...), primary...), tertiary...), anchor...)

	meta, _, ok := parseLegacyPptGroupShapeMetaFromSpContainer(payload, legacyPptGroupFrame{}, nil)
	if !ok {
		t.Fatal("group metadata was not decoded")
	}
	if !meta.transform.HasRotation || meta.transform.Rotation != 30 {
		t.Fatalf("rotation = %v (set=%v), want tertiary override 30", meta.transform.Rotation, meta.transform.HasRotation)
	}
}

func TestLegacyPptTableMarkerCanLiveInTertiaryFOPT(t *testing.T) {
	prop := make([]byte, 6)
	binary.LittleEndian.PutUint16(prop[0:2], legacyOfficeArtPropTableProperties)
	binary.LittleEndian.PutUint32(prop[2:6], 1)
	tertiary := buildOfficeArtRecord(1, officeartescher.RecordTertiaryFOPT, 0x0003, prop)
	if !legacyPptSpContainerIsTable(tertiary) {
		t.Fatal("tertiary groupshape.tableProperties was ignored")
	}
}
