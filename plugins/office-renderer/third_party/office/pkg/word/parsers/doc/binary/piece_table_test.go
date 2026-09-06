package docbinary

import (
	"encoding/binary"
	"testing"
)

func TestParsePieceTableDecodesCompressedAndUnicodePieces(t *testing.T) {
	plc := make([]byte, 28) // 2 pieces: 3 CPs + 2 PCDs
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], 5)
	binary.LittleEndian.PutUint32(plc[8:12], 9)
	binary.LittleEndian.PutUint32(plc[14:18], 0x40000000|uint32(20*2))
	binary.LittleEndian.PutUint32(plc[22:26], uint32(40))

	table, err := ParsePieceTable(plc)
	if err != nil {
		t.Fatalf("ParsePieceTable: %v", err)
	}
	if table.Count() != 2 {
		t.Fatalf("Count = %d, want 2", table.Count())
	}
	first, ok := table.Piece(0)
	if !ok || first.StartCP != 0 || first.EndCP != 5 || first.ByteOffset != 20 || !first.Compressed {
		t.Fatalf("first = %+v, %v", first, ok)
	}
	second, ok := table.Piece(1)
	if !ok || second.StartCP != 5 || second.EndCP != 9 || second.ByteOffset != 40 || second.Compressed {
		t.Fatalf("second = %+v, %v", second, ok)
	}
}

func TestParsePieceTableRejectsNonMonotonicCPs(t *testing.T) {
	plc := make([]byte, 16)
	binary.LittleEndian.PutUint32(plc[0:4], 5)
	binary.LittleEndian.PutUint32(plc[4:8], 4)
	binary.LittleEndian.PutUint32(plc[10:14], 0x40000000)
	if _, err := ParsePieceTable(plc); err == nil {
		t.Fatal("ParsePieceTable accepted non-monotonic CPs")
	}
}

func TestParsePieceTableRejectsMalformedLength(t *testing.T) {
	if _, err := ParsePieceTable(make([]byte, 17)); err == nil {
		t.Fatal("ParsePieceTable accepted malformed PlcPcd length")
	}
}
