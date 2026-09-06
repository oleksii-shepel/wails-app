package pre97

import (
	"encoding/binary"
	"testing"
)

func TestLoadCHPXEntriesOwnsOldFKPStructure(t *testing.T) {
	word := make([]byte, 2048)
	span := Span{ChpTableOffset: 0x100, ChpTableSize: 10}
	plc := word[span.ChpTableOffset : span.ChpTableOffset+span.ChpTableSize]
	binary.LittleEndian.PutUint16(plc[8:10], 2)

	page := word[1024:1536]
	binary.LittleEndian.PutUint32(page[0:4], 100)
	binary.LittleEndian.PutUint32(page[4:8], 110)
	page[8] = 20
	page[40] = 2
	page[41] = 0x35
	page[42] = 0x01
	page[511] = 1

	entries := LoadCHPXEntries(word, span)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].FC != 100 {
		t.Fatalf("FC = %d, want 100", entries[0].FC)
	}
	if got := entries[0].Sprms; len(got) != 2 || got[0] != 0x35 || got[1] != 0x01 {
		t.Fatalf("Sprms = %v, want [53 1]", got)
	}
}

func TestLoadPAPXEntriesOwnsOldFKPStructure(t *testing.T) {
	word := make([]byte, 2560)
	span := Span{Start: 100, BodyEnd: 200, PapTableOffset: 0x120, PapTableSize: 10}
	plc := word[span.PapTableOffset : span.PapTableOffset+span.PapTableSize]
	binary.LittleEndian.PutUint16(plc[8:10], 3)

	page := word[1536:2048]
	binary.LittleEndian.PutUint32(page[0:4], 90)
	binary.LittleEndian.PutUint32(page[4:8], 120)
	page[8] = 20
	page[40] = 3
	page[41] = 7
	page[43] = 0x18
	page[44] = 0x01
	page[45] = 0x05
	page[511] = 1

	entries := LoadPAPXEntries(word, span)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.StartFC != 100 || entry.EndFC != 120 || entry.StartCP != 0 || entry.EndCP != 20 {
		t.Fatalf("range = FC %d..%d CP %d..%d", entry.StartFC, entry.EndFC, entry.StartCP, entry.EndCP)
	}
	if entry.StyleIndex == nil || *entry.StyleIndex != 7 {
		t.Fatalf("StyleIndex = %v, want 7", entry.StyleIndex)
	}
	if got := entry.Sprms; len(got) != 3 || got[0] != 0x18 || got[1] != 0x01 || got[2] != 0x05 {
		t.Fatalf("Sprms = %v", got)
	}
}
