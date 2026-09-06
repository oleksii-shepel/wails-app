package docformat

import (
	"encoding/binary"
	"testing"
)

func TestParseCHPXFKPPreservesFCAndRawSprms(t *testing.T) {
	page := make([]byte, 512)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], 100)
	binary.LittleEndian.PutUint32(page[4:8], 110)
	page[8] = 20
	page[40] = 3
	copy(page[41:44], []byte{0x35, 0x08, 0x01})

	entries := ParseCHPXFKP(page, nil, func(grpprl []byte, _ []string) CHPXEntry {
		return CHPXEntry{Spec: true}
	})
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].FC != 100 || !entries[0].Spec {
		t.Fatalf("entry = %#v", entries[0])
	}
	if len(entries[0].Sprms) != 3 || entries[0].Sprms[0] != 0x35 || entries[0].Sprms[2] != 0x01 {
		t.Fatalf("sprms = %#v", entries[0].Sprms)
	}
}

func TestOperandSizeHandlesVariableExceptions(t *testing.T) {
	chgTabs := []byte{1, 10, 0, 1, 20, 0, 0}
	if got := OperandSize(0xC615, chgTabs, 0); got != 7 {
		t.Fatalf("PChgTabs size = %d, want 7", got)
	}

	tdef := []byte{5, 0, 0, 0, 0, 0}
	if got := OperandSize(0xD608, tdef, 0); got != 6 {
		t.Fatalf("TDefTable size = %d, want 6", got)
	}
}

func TestFindCHPXEntryUsesLastEntryAtOrBeforeFC(t *testing.T) {
	entries := []CHPXEntry{{FC: 10}, {FC: 20}, {FC: 30}}
	if got := FindCHPXEntry(entries, 25); got != 1 {
		t.Fatalf("index = %d, want 1", got)
	}
	if got := FindCHPXEntry(entries, 5); got != -1 {
		t.Fatalf("index = %d, want -1", got)
	}
}
