package docformat

import (
	"encoding/binary"
	"testing"
)

func fullSepxSprm(code uint16, operand ...byte) []byte {
	out := make([]byte, 2+len(operand))
	binary.LittleEndian.PutUint16(out[:2], code)
	copy(out[2:], operand)
	return out
}

func TestDecodeSEPXOwnsLongTailSectionSemantics(t *testing.T) {
	var grp []byte
	grp = append(grp, fullSepxSprm(0x300E, 1)...)
	grp = append(grp, fullSepxSprm(0x5033, 1, 0)...)
	grp = append(grp, fullSepxSprm(0x5040, 3, 0)...)
	grp = append(grp, fullSepxSprm(0x5042, 4, 0)...)

	sec, _ := DecodeSEPX(grp)
	if sec.PageNumberFormat != "upperRoman" {
		t.Fatalf("PageNumberFormat = %q, want upperRoman", sec.PageNumberFormat)
	}
	if sec.TextDirection != "tbRl" {
		t.Fatalf("TextDirection = %q, want tbRl", sec.TextDirection)
	}
	if sec.FootnoteNumFmt != "upperLetter" {
		t.Fatalf("FootnoteNumFmt = %q, want upperLetter", sec.FootnoteNumFmt)
	}
	if sec.EndnoteNumFmt != "lowerLetter" {
		t.Fatalf("EndnoteNumFmt = %q, want lowerLetter", sec.EndnoteNumFmt)
	}
}

func TestDecodeSEPXOwnsPageBorderMetadata(t *testing.T) {
	// BRC80: width=8 eighth-points (1pt), style=1 (single), color index=2,
	// spacing=3pt. SPgbProp: first-page, offset from edge, behind text.
	var grp []byte
	grp = append(grp, fullSepxSprm(0x702B, 8, 1, 2, 3)...)
	grp = append(grp, fullSepxSprm(0x522F, 0x0D, 0x00)...)

	sec, _ := DecodeSEPX(grp)
	if sec.PageBorders == nil || sec.PageBorders.Top == nil {
		t.Fatal("PageBorders.Top = nil")
	}
	if sec.PageBorders.Display != "firstPage" {
		t.Fatalf("Display = %q, want firstPage", sec.PageBorders.Display)
	}
	if sec.PageBorders.OffsetFrom != "edge" {
		t.Fatalf("OffsetFrom = %q, want edge", sec.PageBorders.OffsetFrom)
	}
	if sec.PageBorders.ZOrder != "back" {
		t.Fatalf("ZOrder = %q, want back", sec.PageBorders.ZOrder)
	}
}

func TestDecodeSEPXPreservesRecognizedUnmappedDiagnostics(t *testing.T) {
	sec, _ := DecodeSEPX(fullSepxSprm(0x5007, 1, 0))
	if len(sec.Diagnostics) != 1 {
		t.Fatalf("Diagnostics = %d, want 1", len(sec.Diagnostics))
	}
	if sec.Diagnostics[0].Code != "legacy.sprm.unmapped" {
		t.Fatalf("diagnostic code = %q", sec.Diagnostics[0].Code)
	}
}
