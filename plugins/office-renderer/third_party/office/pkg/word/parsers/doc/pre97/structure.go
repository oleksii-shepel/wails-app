package pre97

import "encoding/binary"

// Span is the structural Word 6/95 body/FIB map needed by the sequential
// decoder. Offsets are absolute positions in the WordDocument stream.
type Span struct {
	Signature      uint16
	NFib           uint16
	Start          int
	FcMac          int
	CcpText        int
	BodyEnd        int
	SecondaryStart int
	SecondarySize  int
	FComplex       bool

	CcpFtn     int
	CcpHdd     int
	CcpMcr     int
	CcpAtn     int
	CcpEdn     int
	CcpTxbx    int
	CcpHdrTxbx int

	SedTableOffset       int
	SedTableSize         int
	FtnRefTableOffset    int
	FtnRefTableSize      int
	FtnTxtTableOffset    int
	FtnTxtTableSize      int
	AtnRefTableOffset    int
	AtnRefTableSize      int
	AtnTxtTableOffset    int
	AtnTxtTableSize      int
	HddTableOffset       int
	HddTableSize         int
	ChpTableOffset       int
	ChpTableSize         int
	PapTableOffset       int
	PapTableSize         int
	FontTableOffset      int
	FontTableSize        int
	BkmkNameTableOffset  int
	BkmkNameTableSize    int
	BkmkStartTableOffset int
	BkmkStartTableSize   int
	BkmkEndTableOffset   int
	BkmkEndTableSize     int
	EdnRefTableOffset    int
	EdnRefTableSize      int
	EdnTxtTableOffset    int
	EdnTxtTableSize      int
	ComplexTableOffset   int
}

// InspectBodySpan decodes the structural Word 6/95 FIB slots used by the
// sequential reader. It deliberately does not interpret formatting semantics.
func InspectBodySpan(word []byte) (Span, bool) {
	if len(word) < 0x60 {
		return Span{}, false
	}
	span := Span{
		Signature:      binary.LittleEndian.Uint16(word[0:2]),
		NFib:           binary.LittleEndian.Uint16(word[2:4]),
		Start:          int(binary.LittleEndian.Uint32(word[24:28])),
		FcMac:          int(binary.LittleEndian.Uint32(word[28:32])),
		CcpText:        int(binary.LittleEndian.Uint32(word[52:56])),
		SecondaryStart: int(binary.LittleEndian.Uint32(word[0x58:0x5c])),
		SecondarySize:  int(binary.LittleEndian.Uint32(word[0x5c:0x60])),
	}
	span.FComplex = word[0x0b]&0x04 != 0

	readPair := func(off int) (int, int) {
		if off < 0 || off+8 > len(word) {
			return 0, 0
		}
		return int(int32(binary.LittleEndian.Uint32(word[off : off+4]))),
			int(int32(binary.LittleEndian.Uint32(word[off+4 : off+8])))
	}
	if len(word) >= 0x54 {
		span.CcpFtn = int(int32(binary.LittleEndian.Uint32(word[0x38:0x3c])))
		span.CcpHdd = int(int32(binary.LittleEndian.Uint32(word[0x3c:0x40])))
		span.CcpMcr = int(int32(binary.LittleEndian.Uint32(word[0x40:0x44])))
		span.CcpAtn = int(int32(binary.LittleEndian.Uint32(word[0x44:0x48])))
		span.CcpEdn = int(int32(binary.LittleEndian.Uint32(word[0x48:0x4c])))
		span.CcpTxbx = int(int32(binary.LittleEndian.Uint32(word[0x4c:0x50])))
		span.CcpHdrTxbx = int(int32(binary.LittleEndian.Uint32(word[0x50:0x54])))
	}
	span.FtnRefTableOffset, span.FtnRefTableSize = readPair(0x68)
	span.FtnTxtTableOffset, span.FtnTxtTableSize = readPair(0x70)
	span.AtnRefTableOffset, span.AtnRefTableSize = readPair(0x78)
	span.AtnTxtTableOffset, span.AtnTxtTableSize = readPair(0x80)
	span.SedTableOffset, span.SedTableSize = readPair(0x88)
	span.HddTableOffset, span.HddTableSize = readPair(0xb0)
	span.ChpTableOffset, span.ChpTableSize = readPair(0xb8)
	span.PapTableOffset, span.PapTableSize = readPair(0xc0)
	span.FontTableOffset, span.FontTableSize = readPair(0xd0)
	span.BkmkNameTableOffset, span.BkmkNameTableSize = readPair(0x100)
	span.BkmkStartTableOffset, span.BkmkStartTableSize = readPair(0x108)
	span.BkmkEndTableOffset, span.BkmkEndTableSize = readPair(0x110)
	span.EdnRefTableOffset, span.EdnRefTableSize = readPair(0x128)
	span.EdnTxtTableOffset, span.EdnTxtTableSize = readPair(0x130)
	if len(word) >= 0x164 {
		span.ComplexTableOffset = int(int32(binary.LittleEndian.Uint32(word[0x160:0x164])))
	}

	if span.Start <= 0 || span.Start >= len(word) {
		return Span{}, false
	}
	if span.FcMac > span.Start && span.FcMac <= len(word) {
		span.BodyEnd = span.FcMac
	}
	if span.CcpText > 0 {
		candidate := span.Start + span.CcpText
		if candidate > span.Start && candidate <= len(word) && (span.BodyEnd == 0 || candidate < span.BodyEnd) {
			span.BodyEnd = candidate
		}
	}
	if span.BodyEnd <= span.Start {
		return Span{}, false
	}
	return span, true
}

// BuildSinglePiecePLC synthesizes the one-piece PlcPcd used by non-complex old
// DOCs. The resulting piece is compressed/single-byte and spans only main text.
func BuildSinglePiecePLC(span Span) []byte {
	charCount := span.BodyEnd - span.Start
	if span.Start <= 0 || charCount <= 0 {
		return nil
	}
	plc := make([]byte, 16)
	binary.LittleEndian.PutUint32(plc[4:8], uint32(charCount))
	fcRaw := uint32(0x40000000) | uint32(span.Start*2)
	binary.LittleEndian.PutUint32(plc[10:14], fcRaw)
	return plc
}

// FKPPageNumbers decodes the old PlcBte-style table used by CHPX/PAPX to
// reference 512-byte formatting pages. It returns only structurally valid pages.
func FKPPageNumbers(word []byte, tableOffset, tableSize int) []int {
	if tableOffset <= 0 || tableSize < 10 || tableOffset+tableSize > len(word) {
		return nil
	}
	plc := word[tableOffset : tableOffset+tableSize]
	n := (len(plc) - 4) / 6
	if n <= 0 {
		return nil
	}
	cpBytes := (n + 1) * 4
	if cpBytes+n*2 > len(plc) {
		return nil
	}
	pages := make([]int, 0, n)
	for i := 0; i < n; i++ {
		page := int(binary.LittleEndian.Uint16(plc[cpBytes+i*2 : cpBytes+i*2+2]))
		off := page * 512
		if off >= 0 && off+512 <= len(word) {
			pages = append(pages, page)
		}
	}
	return pages
}
