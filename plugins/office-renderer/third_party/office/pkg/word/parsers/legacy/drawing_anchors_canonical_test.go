package legacy

import (
	"encoding/binary"
	"testing"
)

func TestLoadLegacyFloatingDrawingAnchorsPrefersWord97PlcfSpaMomSlot40(t *testing.T) {
	const (
		fibBase   = 0
		pairCount = 41
		mainLen   = 20
		shapeID   = 0x11223344
	)
	canonical := buildLegacySingleAnchorPlcfSpa(5, mainLen, shapeID, 120, 640, 2120, 1640, 0x0010)
	decoy := buildLegacySingleAnchorPlcfSpa(5, mainLen, shapeID, 900, 1800, 3900, 4800, 0x0010)
	table := append(append([]byte(nil), decoy...), canonical...)
	word := make([]byte, pairCount*8)
	binary.LittleEndian.PutUint32(word[31*8:], 0)
	binary.LittleEndian.PutUint32(word[31*8+4:], uint32(len(decoy)))
	binary.LittleEndian.PutUint32(word[40*8:], uint32(len(decoy)))
	binary.LittleEndian.PutUint32(word[40*8+4:], uint32(len(canonical)))

	got := loadLegacyFloatingDrawingAnchors(word, table, fibBase, pairCount, mainLen, []int{5})
	anchor, ok := got[5]
	if !ok {
		t.Fatal("canonical anchor CP 5 missing")
	}
	if anchor.Left != 120 || anchor.Top != 640 || anchor.Right != 2120 || anchor.Bottom != 1640 {
		t.Fatalf("anchor = %+v, want slot-40 PlcfSpaMom rectangle", anchor)
	}
}

func TestLoadLegacyFloatingDrawingAnchorsAcceptsLegacySyntheticSlot30(t *testing.T) {
	const (
		fibBase   = 0
		pairCount = 41
		mainLen   = 20
		shapeID   = 0x11223344
	)
	legacy := buildLegacySingleAnchorPlcfSpa(5, mainLen, shapeID, 120, 640, 2120, 1640, 0x0010)
	word := make([]byte, pairCount*8)
	binary.LittleEndian.PutUint32(word[30*8:], 0)
	binary.LittleEndian.PutUint32(word[30*8+4:], uint32(len(legacy)))

	got := loadLegacyFloatingDrawingAnchors(word, legacy, fibBase, pairCount, mainLen, []int{5})
	if _, ok := got[5]; !ok {
		t.Fatal("legacy slot-30 compatibility anchor missing")
	}
}
