package legacy

import (
	"encoding/binary"
	"testing"
)

func TestChpxOperandSizeCoversMSDOCSpraTable(t *testing.T) {
	tests := []struct {
		name string
		code uint16
		data []byte
		pos  int
		want int
	}{
		{name: "spra0 toggle operand", code: 0x0000, data: []byte{0x01}, want: 1},
		{name: "spra1 one byte", code: 0x2000, data: []byte{0x7F}, want: 1},
		{name: "spra2 two bytes", code: 0x4000, data: []byte{0x01, 0x02}, want: 2},
		{name: "spra3 four bytes", code: 0x6000, data: []byte{0x01, 0x02, 0x03, 0x04}, want: 4},
		{name: "spra4 fixed two bytes", code: 0x8000, data: []byte{0x01, 0x02}, want: 2},
		{name: "spra5 fixed two bytes", code: 0xA000, data: []byte{0x01, 0x02}, want: 2},
		{name: "spra6 generic one byte count", code: 0xC000, data: []byte{0x03, 0xAA, 0xBB, 0xCC}, want: 4},
		{name: "spra7 three bytes", code: 0xE000, data: []byte{0x01, 0x02, 0x03}, want: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := chpxOperandSize(tc.code, tc.data, tc.pos); got != tc.want {
				t.Fatalf("chpxOperandSize(%#04x) = %d, want %d", tc.code, got, tc.want)
			}
		})
	}
}

func TestChpxOperandSizeRejectsTruncatedVariableExceptions(t *testing.T) {
	if got := chpxOperandSize(0xC000, nil, 0); got != -1 {
		t.Fatalf("generic spra6 truncated = %d, want -1", got)
	}
	if got := chpxOperandSize(0xD608, []byte{0x04}, 0); got != -1 {
		t.Fatalf("sprmTDefTable truncated cb = %d, want -1", got)
	}
	if got := chpxOperandSize(0xC615, []byte{0x02, 0x00}, 0); got != -1 {
		t.Fatalf("sprmPChgTabs truncated = %d, want -1", got)
	}
}

func TestChpxOperandSizeTDefTableUsesTwoByteCb(t *testing.T) {
	operand := make([]byte, 261)
	binary.LittleEndian.PutUint16(operand[0:2], 260)

	if got := chpxOperandSize(0xD608, operand, 0); got != 261 {
		t.Fatalf("sprmTDefTable size = %d, want 261", got)
	}
}

func TestPapxGrpprlContinuesAfterLargeTDefTable(t *testing.T) {
	// Regression guard for the old bug: sprmTDefTable is spra=6, but its operand
	// starts with a two-byte cb. When cb is >255, treating it as the generic
	// one-byte-count spra=6 form misaligns the next Prl and drops later PAP data.
	grpprl := make([]byte, 0, 2+261+3)
	grpprl = append(grpprl, 0x08, 0xD6) // sprmTDefTable
	operand := make([]byte, 261)
	binary.LittleEndian.PutUint16(operand[0:2], 260)
	grpprl = append(grpprl, operand...)
	grpprl = append(grpprl, 0x03, 0x24, 0x02) // sprmPJc = right

	format, _ := parsePapxGrpprlWithMetadata(grpprl)
	if format.Alignment != "right" {
		t.Fatalf("Alignment = %q, want right", format.Alignment)
	}
}

func TestSepxGrpprlContinuesAfterGenericSpra6(t *testing.T) {
	// Section parser uses the same shared operand-size helper. Verify a generic
	// spra=6 unknown SPRM is skipped and a following known SEP SPRM is still read.
	grpprl := []byte{
		0x00, 0xC0, 0x03, 0xAA, 0xBB, 0xCC, // unknown generic spra=6, cb=3
		0x1F, 0xB0, 0x00, 0x30, // sprmSXaPage = 12288 twips
	}

	sec := parseSepxGrpprl(grpprl)
	if sec.PageSize == nil || sec.PageSize.Width != 12288 {
		t.Fatalf("PageSize.Width = %d, want 12288", sec.PageSize.Width)
	}
}