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

func TestDecodeGeometryTextReadsBooleanSemantics(t *testing.T) {
	// Upper word is fUse*, lower word is the corresponding value bitmap.
	const bits = (1 << 2) | (1 << 3) | (1 << 5) | (1 << 7) | (1 << 10) | (1 << 11) | (1 << 12) | (1 << 15)
	got := DecodeGeometryText([]escher.Property{{
		ID: escher.PropGTextBooleans, Value: uint32(bits) | uint32(bits)<<16,
	}})
	if !got.Vertical || !got.Kerning || !got.Stretch || !got.BestFit || !got.Bold || !got.Italic || !got.Underline || !got.Strikethrough {
		t.Fatalf("geometry text booleans = %+v", got)
	}
	if got.Tight || got.ShrinkFit || got.Normalize {
		t.Fatalf("unexpected geometry text booleans = %+v", got)
	}
}

func TestWordArtWarpNameCoversOfficeArtTextEffectRange(t *testing.T) {
	for _, tc := range []struct {
		shapeType uint16
		want      string
	}{
		{24, "textSimple"},
		{31, "textOnRing"},
		{136, "textPlain"},
		{144, "textArchUpCurve"},
		{156, "textWave1"},
		{175, "textCanDown"},
		{23, ""},
		{32, ""},
		{135, ""},
		{176, ""},
	} {
		if got := WordArtWarpName(tc.shapeType); got != tc.want {
			t.Fatalf("WordArtWarpName(%d) = %q, want %q", tc.shapeType, got, tc.want)
		}
	}
}

func TestWordArtWarpNameUsesBothMSOSPTTextRanges(t *testing.T) {
	cases := map[uint16]string{
		24:  "textSimple",
		31:  "textOnRing",
		136: "textPlain",
		144: "textArchUpCurve",
		156: "textWave1",
		175: "textCanDown",
	}
	for id, want := range cases {
		if got := WordArtWarpName(id); got != want {
			t.Fatalf("WordArtWarpName(%d) = %q, want %q", id, got, want)
		}
	}
	for _, id := range []uint16{23, 32, 135, 176} {
		if got := WordArtWarpName(id); got != "" {
			t.Fatalf("WordArtWarpName(%d) = %q, want empty", id, got)
		}
	}
}
