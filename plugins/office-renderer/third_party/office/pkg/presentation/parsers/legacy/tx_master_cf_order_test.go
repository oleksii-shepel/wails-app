package legacy

import (
	"encoding/binary"
	"testing"
)

func TestTxMasterCFExceptionUsesStructureOrderAndZeroWidthExtensionMasks(t *testing.T) {
	const masks = uint32(
		(1 << 0) | // bold -> fontStyle
			(1 << 16) | // fontRef
			(1 << 17) | // size
			(1 << 18) | // color
			(1 << 19) | // position
			(1 << 20) | // pp10ext: zero-width in TextCFException
			(1 << 21) | // oldEAFontRef
			(1 << 22) | // ansiFontRef
			(1 << 23) | // symbolFontRef
			(1 << 24) | // newEATypeface: zero-width here
			(1 << 25) | // csTypeface: zero-width here
			(1 << 26)) // pp11ext: zero-width here

	payload := make([]byte, 4)
	binary.LittleEndian.PutUint32(payload, masks)
	appendWord := func(v uint16) {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], v)
		payload = append(payload, b[:]...)
	}

	appendWord(1)                                     // fontStyle: bold
	appendWord(0)                                     // fontRef -> Times New Roman
	appendWord(2)                                     // oldEAFontRef -> MS Gothic
	appendWord(1)                                     // ansiFontRef -> Arial
	appendWord(3)                                     // symbolFontRef -> Symbol
	appendWord(44)                                    // fontSize, points
	payload = append(payload, 0x12, 0x34, 0x56, 0xFE) // direct RGB
	appendWord(uint16(int16ToBits(-25)))

	fonts := map[uint16]string{0: "Times New Roman", 1: "Arial", 2: "MS Gothic", 3: "Symbol"}
	run, pos, ok := parseCFExceptionSpecWithTheme(payload, 0, fonts, nil)
	if !ok || pos != len(payload) {
		t.Fatalf("parse = ok:%v pos:%d, want true/%d", ok, pos, len(payload))
	}
	if run.Bold == nil || !*run.Bold {
		t.Fatalf("bold = %#v, want explicit true", run.Bold)
	}
	if run.FontFamily != "Times New Roman" || run.EAFontFamily != "MS Gothic" || run.AnsiFontFamily != "Arial" || run.SymbolFontFamily != "Symbol" {
		t.Fatalf("fonts = generic:%q ea:%q ansi:%q symbol:%q", run.FontFamily, run.EAFontFamily, run.AnsiFontFamily, run.SymbolFontFamily)
	}
	if run.FontSize != 4400 || run.Color != "123456" || run.BaselineShiftPct != -25 || !run.Subscript || run.Superscript {
		t.Fatalf("metrics = size:%d color:%q baseline:%g super:%v sub:%v", run.FontSize, run.Color, run.BaselineShiftPct, run.Superscript, run.Subscript)
	}
}
