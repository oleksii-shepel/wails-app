package legacy

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"office/pkg/officeart/legacy/escher"
)

func geometryTextUTF16(s string) []byte {
	u := utf16.Encode([]rune(s + "\x00"))
	out := make([]byte, len(u)*2)
	for i, v := range u {
		binary.LittleEndian.PutUint16(out[i*2:], v)
	}
	return out
}

func TestDecodeGeometryTextReadsUnicodeAndFont(t *testing.T) {
	got := DecodeGeometryText([]escher.Property{
		{ID: escher.PropGTextUnicode, IsComplex: true, Complex: geometryTextUTF16("Legacy WordArt")},
		{ID: escher.PropGTextFont, IsComplex: true, Complex: geometryTextUTF16("Arial Black")},
		{ID: escher.PropGTextAlign, Value: 1},
		{ID: escher.PropGTextSize, Value: 36 << 16},
	})
	if !got.Present || got.Text != "Legacy WordArt" || got.Font != "Arial Black" {
		t.Fatalf("geometry text = %+v", got)
	}
	if got.Align != 1 || got.Size != 36<<16 {
		t.Fatalf("geometry text metrics = %+v", got)
	}
}

func TestDecodeGeometryTextUnicodeImpliesPresence(t *testing.T) {
	got := DecodeGeometryText([]escher.Property{
		{ID: escher.PropGTextUnicode, IsComplex: true, Complex: geometryTextUTF16("WordArt")},
	})
	if !got.Present || got.Text != "WordArt" {
		t.Fatalf("geometry text = %+v", got)
	}
}
