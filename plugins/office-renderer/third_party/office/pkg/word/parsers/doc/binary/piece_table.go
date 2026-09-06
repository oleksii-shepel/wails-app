package docbinary

import (
	"encoding/binary"
	"fmt"
)

// Piece describes one PCD entry from a DOC PlcPcd. CP coordinates are logical
// character positions; ByteOffset points into the WordDocument stream.
type Piece struct {
	StartCP    int
	EndCP      int
	RawFC      uint32
	ByteOffset int64
	Compressed bool
}

func (p Piece) CharCount() int { return p.EndCP - p.StartCP }

// PieceTable is a validated, immutable view over PlcPcd metadata.
type PieceTable struct {
	pieces []Piece
}

func (t *PieceTable) Count() int {
	if t == nil {
		return 0
	}
	return len(t.pieces)
}

func (t *PieceTable) Piece(index int) (Piece, bool) {
	if t == nil || index < 0 || index >= len(t.pieces) {
		return Piece{}, false
	}
	return t.pieces[index], true
}

// ParsePieceTable validates and decodes a PlcPcd. The binary ownership lives in
// parsers/doc so sequential consumers no longer need to know the CP/PCD layout.
func ParsePieceTable(plc []byte) (*PieceTable, error) {
	if len(plc) < 16 || (len(plc)-4)%12 != 0 {
		return nil, fmt.Errorf("doc stream: invalid or empty PlcPcd")
	}
	pieceCount := (len(plc) - 4) / 12
	cpBase := 4 * (pieceCount + 1)
	if cpBase+pieceCount*8 != len(plc) {
		return nil, fmt.Errorf("doc stream: truncated PlcPcd")
	}

	charPos := make([]int, pieceCount+1)
	for i := range charPos {
		off := i * 4
		if off+4 > len(plc) {
			return nil, fmt.Errorf("doc stream: truncated PlcPcd CP array")
		}
		charPos[i] = int(binary.LittleEndian.Uint32(plc[off : off+4]))
		if i > 0 && charPos[i] < charPos[i-1] {
			return nil, fmt.Errorf("doc stream: non-monotonic PlcPcd CP array")
		}
	}

	pieces := make([]Piece, pieceCount)
	for i := 0; i < pieceCount; i++ {
		pcdOff := cpBase + i*8
		if pcdOff+8 > len(plc) {
			return nil, fmt.Errorf("doc stream: truncated PCD %d", i)
		}
		fcRaw := binary.LittleEndian.Uint32(plc[pcdOff+2 : pcdOff+6])
		compressed := fcRaw&0x40000000 != 0
		byteOff := int64(fcRaw & 0x3FFFFFFF)
		if compressed {
			byteOff /= 2
		}
		pieces[i] = Piece{
			StartCP:    charPos[i],
			EndCP:      charPos[i+1],
			RawFC:      fcRaw,
			ByteOffset: byteOff,
			Compressed: compressed,
		}
	}
	return &PieceTable{pieces: pieces}, nil
}
