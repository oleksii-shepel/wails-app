package pre97

import (
	"encoding/binary"
	"testing"
)

func TestOpenCoreBuildsSimplePieceAndFonts(t *testing.T) {
	word := make([]byte, 2048)
	binary.LittleEndian.PutUint16(word[0:2], 0xA5DC)
	binary.LittleEndian.PutUint16(word[2:4], 0x0065)
	binary.LittleEndian.PutUint32(word[24:28], 512)
	binary.LittleEndian.PutUint32(word[28:32], 516)
	binary.LittleEndian.PutUint32(word[52:56], 4)

	// Old FFN table at 1024: two-byte redundant length, one 12-byte record.
	binary.LittleEndian.PutUint32(word[0xD0:0xD4], 1024)
	binary.LittleEndian.PutUint32(word[0xD4:0xD8], 15)
	word[1024] = 13
	word[1026] = 11 // record length begins after two-byte table header
	word[1026+4] = 0xCC
	copy(word[1026+6:], []byte("Arial\x00"))

	core, ok := OpenCore(word)
	if !ok {
		t.Fatal("OpenCore rejected structurally valid old DOC")
	}
	if core.Span.Start != 512 || core.Span.BodyEnd != 516 {
		t.Fatalf("span = %d..%d", core.Span.Start, core.Span.BodyEnd)
	}
	if len(core.PiecePLC) != 16 {
		t.Fatalf("piece PLC len = %d, want 16", len(core.PiecePLC))
	}
	if len(core.Fonts) != 1 || core.Fonts[0].Name != "Arial" || core.Fonts[0].Charset != 0xCC {
		t.Fatalf("fonts = %#v", core.Fonts)
	}
	if core.PreferredCharset == nil {
		t.Fatal("expected Windows-1251 preferred charset")
	}
}

func TestOpenCoreKeepsMalformedComplexCore(t *testing.T) {
	word := make([]byte, 1024)
	binary.LittleEndian.PutUint16(word[0:2], 0xA5DC)
	binary.LittleEndian.PutUint16(word[2:4], 0x0065)
	word[0x0B] = 0x04
	binary.LittleEndian.PutUint32(word[24:28], 512)
	binary.LittleEndian.PutUint32(word[28:32], 516)
	binary.LittleEndian.PutUint32(word[52:56], 4)
	binary.LittleEndian.PutUint32(word[0x160:0x164], 700)
	word[700] = 0x7f // invalid CLX record kind

	core, ok := OpenCore(word)
	if !ok || core == nil {
		t.Fatal("valid FIB should still produce a core")
	}
	if !core.Span.FComplex {
		t.Fatal("expected complex flag")
	}
	if len(core.PiecePLC) != 0 {
		t.Fatalf("malformed CLX produced piece PLC: %x", core.PiecePLC)
	}
}
