package legacy

import (
	"encoding/binary"
	"testing"

	presentationir "office/pkg/presentation/ir"
)

// buildCFRecord builds a minimal STPA-style entry: cchRun (2 bytes) +
// TextCFException payload (masks + optional fields).
func buildCFRecord(cch uint16, masks uint32, extra []byte) []byte {
	b := make([]byte, 2+4+len(extra))
	binary.LittleEndian.PutUint16(b[0:2], cch)
	binary.LittleEndian.PutUint32(b[2:6], masks)
	copy(b[6:], extra)
	return b
}

// buildPFRecord builds a minimal PF entry: cchPara (2 bytes) +
// TextPFException payload (4-byte masks + optional fields).
func buildPFRecord(cch uint16, masks uint32, extra []byte) []byte {
	b := make([]byte, 2+4+len(extra))
	binary.LittleEndian.PutUint16(b[0:2], cch)
	binary.LittleEndian.PutUint32(b[2:6], masks)
	copy(b[6:], extra)
	return b
}

// --- TextCFException unit tests ---

func TestParseCFExceptionBoldItalic(t *testing.T) {
	// masks bit 0 (bold) + bit 1 (italic) both set → grpfBasicAtom must follow.
	// grpfBasicAtom = 0x03 (bold=true, italic=true).
	masks := uint32(0x03) // bits 0+1
	grpf := []byte{0x03, 0x00}
	stpa := append(marshalUint32LE(masks), grpf...)
	run, newPos, ok := parseCFException(stpa, 0)
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if newPos != len(stpa) {
		t.Fatalf("newPos = %d, want %d", newPos, len(stpa))
	}
	if run.Bold == nil || !*run.Bold {
		t.Fatalf("Bold = %v, want true", run.Bold)
	}
	if run.Italic == nil || !*run.Italic {
		t.Fatalf("Italic = %v, want true", run.Italic)
	}
	if run.Strike != nil {
		t.Fatalf("Strike = %v, want nil (not set)", run.Strike)
	}
}

func TestParseCFExceptionUnderlineStrike(t *testing.T) {
	// masks bit 2 (underline) + bit 7 (strikeout) → grpfBasicAtom 0x84.
	masks := uint32((1 << 2) | (1 << 7))
	grpf := []byte{0x84, 0x00} // bits 2 and 7 set
	stpa := append(marshalUint32LE(masks), grpf...)
	run, _, ok := parseCFException(stpa, 0)
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if run.Underline == nil || !*run.Underline {
		t.Fatalf("Underline = %v, want true", run.Underline)
	}
	if run.Strike == nil || !*run.Strike {
		t.Fatalf("Strike = %v, want true", run.Strike)
	}
}

func TestParseCFExceptionFontSize(t *testing.T) {
	// masks bit 10 (fontSize) only; no grpfBasicAtom.
	masks := uint32(1 << 10)
	extra := marshalUint16LE(2400) // 24pt in hundredths of a point
	stpa := append(marshalUint32LE(masks), extra...)
	run, newPos, ok := parseCFException(stpa, 0)
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if newPos != len(stpa) {
		t.Fatalf("newPos = %d, want %d", newPos, len(stpa))
	}
	if run.FontSize != 2400 {
		t.Fatalf("FontSize = %d, want 2400", run.FontSize)
	}
}

func TestParseCFExceptionNegativeFontSizeIgnored(t *testing.T) {
	// A negative fontSize (0xFFFF = -1) means "inherit"; we expect FontSize 0.
	masks := uint32(1 << 10)
	extra := marshalUint16LE(0xFFFF)
	stpa := append(marshalUint32LE(masks), extra...)
	run, _, ok := parseCFException(stpa, 0)
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if run.FontSize != 0 {
		t.Fatalf("FontSize = %d, want 0 for inherit (-1)", run.FontSize)
	}
}

func TestParseCFExceptionColorRGB(t *testing.T) {
	// masks bit 11 (color): R=0x11, G=0x22, B=0x33, index=0xFE (explicit RGB).
	masks := uint32(1 << 11)
	stpa := append(marshalUint32LE(masks), 0x11, 0x22, 0x33, 0xFE)
	run, _, ok := parseCFException(stpa, 0)
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if run.Color != "112233" {
		t.Fatalf("Color = %q, want %q", run.Color, "112233")
	}
}

func TestParseCFExceptionColorSchemeWithoutPaletteStaysEmpty(t *testing.T) {
	// index 0x05 means scheme color; without a parsed palette, Color remains empty.
	masks := uint32(1 << 11)
	stpa := append(marshalUint32LE(masks), 0x00, 0x00, 0x00, 0x05)
	run, _, ok := parseCFException(stpa, 0)
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if run.Color != "" {
		t.Fatalf("Color = %q, want empty for scheme color", run.Color)
	}
}

func TestParseCFExceptionColorSchemeResolvesFromPalette(t *testing.T) {
	masks := uint32(1 << 11)
	stpa := append(marshalUint32LE(masks), 0x00, 0x00, 0x00, 0x05)
	run, _, ok := parseCFExceptionWithTheme(stpa, 0, nil, map[uint8]string{5: "C0504D"})
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if run.Color != "C0504D" {
		t.Fatalf("Color = %q, want %q", run.Color, "C0504D")
	}
}

func TestParseCFExceptionTypefaceResolvesFontFamily(t *testing.T) {
	// masks bit 9 (typeface): 2-byte font index resolved through the font table.
	masks := uint32(1 << 9)
	stpa := append(marshalUint32LE(masks), 0x02, 0x00)
	run, newPos, ok := parseCFExceptionWithFonts(stpa, 0, map[uint16]string{2: "Times New Roman"})
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if newPos != len(stpa) {
		t.Fatalf("newPos = %d, want %d", newPos, len(stpa))
	}
	if run.FontFamily != "Times New Roman" {
		t.Fatalf("FontFamily = %q, want %q", run.FontFamily, "Times New Roman")
	}
}

func TestParseCFExceptionAllBasicFlags(t *testing.T) {
	// bits 0-2 + 7 all set; grpfBasicAtom has bold+italic+underline+strikeout.
	masks := uint32((1 << 0) | (1 << 1) | (1 << 2) | (1 << 7))
	grpf := uint16((1 << 0) | (1 << 1) | (1 << 2) | (1 << 7))
	stpa := append(marshalUint32LE(masks), marshalUint16LE(grpf)...)
	run, _, ok := parseCFException(stpa, 0)
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if run.Bold == nil || !*run.Bold {
		t.Fatalf("Bold = %v, want true", run.Bold)
	}
	if run.Italic == nil || !*run.Italic {
		t.Fatalf("Italic = %v, want true", run.Italic)
	}
	if run.Underline == nil || !*run.Underline {
		t.Fatalf("Underline = %v, want true", run.Underline)
	}
	if run.Strike == nil || !*run.Strike {
		t.Fatalf("Strike = %v, want true", run.Strike)
	}
}

func TestParseCFExceptionBoldFalse(t *testing.T) {
	// masks bit 0 set, but grpfBasicAtom bit 0 = 0 → bold = false (not nil).
	masks := uint32(1 << 0)
	stpa := append(marshalUint32LE(masks), 0x00, 0x00)
	run, _, ok := parseCFException(stpa, 0)
	if !ok {
		t.Fatal("parseCFException returned ok=false")
	}
	if run.Bold == nil || *run.Bold {
		t.Fatalf("Bold = %v, want *false", run.Bold)
	}
}

// --- TextPFException unit tests ---

func TestParsePFExceptionNullMasks(t *testing.T) {
	// masks = 0 → no optional fields, position advances by 4 only.
	stpa := marshalUint32LE(0)
	entry, newPos, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if newPos != 4 {
		t.Fatalf("newPos = %d, want 4", newPos)
	}
	if entry.Align != "" {
		t.Fatalf("Align = %q, want empty", entry.Align)
	}
	if entry.HasBullet {
		t.Fatal("HasBullet should be false")
	}
}

func TestParsePFExceptionTextAlignment(t *testing.T) {
	// masks bit 16 (textAlignment) = center (1).
	masks := uint32(1 << 16)
	stpa := append(marshalUint32LE(masks), marshalUint16LE(1)...) // 1 = center
	entry, newPos, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if newPos != len(stpa) {
		t.Fatalf("newPos = %d, want %d", newPos, len(stpa))
	}
	if entry.Align != "ctr" {
		t.Fatalf("Align = %q, want %q", entry.Align, "ctr")
	}
}

func TestParsePFExceptionAlignLeft(t *testing.T) {
	masks := uint32(1 << 16)
	stpa := append(marshalUint32LE(masks), marshalUint16LE(0)...) // 0 = left
	entry, _, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if entry.Align != "l" {
		t.Fatalf("Align = %q, want %q", entry.Align, "l")
	}
}

func TestParsePFExceptionAlignRight(t *testing.T) {
	masks := uint32(1 << 16)
	stpa := append(marshalUint32LE(masks), marshalUint16LE(2)...) // 2 = right
	entry, _, _ := parsePFException(stpa, 0)
	if entry.Align != "r" {
		t.Fatalf("Align = %q, want %q", entry.Align, "r")
	}
}

func TestParsePFExceptionAlignJustify(t *testing.T) {
	masks := uint32(1 << 16)
	stpa := append(marshalUint32LE(masks), marshalUint16LE(3)...) // 3 = justify
	entry, _, _ := parsePFException(stpa, 0)
	if entry.Align != "just" {
		t.Fatalf("Align = %q, want %q", entry.Align, "just")
	}
}

func TestParsePFExceptionTextDirectionRightToLeft(t *testing.T) {
	masks := uint32(1 << 21)
	stpa := append(marshalUint32LE(masks), marshalUint16LE(2)...)
	entry, newPos, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if newPos != len(stpa) {
		t.Fatalf("newPos = %d, want %d", newPos, len(stpa))
	}
	if entry.TextDirection != "rtl" {
		t.Fatalf("TextDirection = %q, want %q", entry.TextDirection, "rtl")
	}
}

func TestParsePFExceptionBulletChar(t *testing.T) {
	// masks: bit 0 (hasBullet) + bit 7 (bulletChar).
	// bulletFlags word: bit 0 = 1 (fHasBullet).
	// bulletChar = 0x2022 (•).
	masks := uint32((1 << 0) | (1 << 7))
	stpa := marshalUint32LE(masks)
	stpa = append(stpa, marshalUint16LE(0x0001)...) // bulletFlags: hasBullet=true
	stpa = append(stpa, marshalUint16LE(0x2022)...) // bulletChar = '•'
	entry, newPos, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if newPos != len(stpa) {
		t.Fatalf("newPos = %d, want %d", newPos, len(stpa))
	}
	if !entry.HasBullet {
		t.Fatal("HasBullet should be true")
	}
	if entry.BulletChar != '•' {
		t.Fatalf("BulletChar = %U, want U+2022 (•)", entry.BulletChar)
	}
}

func TestParsePFExceptionLeftMarginAndIndent(t *testing.T) {
	// bit 8 (leftMargin) + bit 9 (indent): each 4 bytes.
	masks := uint32((1 << 8) | (1 << 9))
	stpa := marshalUint32LE(masks)
	stpa = append(stpa, 0x00, 0x01, 0x00, 0x00) // leftMargin = 256 master units
	stpa = append(stpa, 0x80, 0x00, 0x00, 0x00) // indent = 128 master units
	_, newPos, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if newPos != len(stpa) {
		t.Fatalf("newPos = %d, want %d", newPos, len(stpa))
	}
}

// --- splitPptRawParagraphRunes unit tests ---

func TestSplitPptRawParagraphRunesSingleParagraph(t *testing.T) {
	paras := splitPptRawParagraphRunes("Hello")
	if len(paras) != 1 {
		t.Fatalf("len = %d, want 1", len(paras))
	}
	if string(paras[0]) != "Hello" {
		t.Fatalf("para[0] = %q, want %q", string(paras[0]), "Hello")
	}
}

func TestSplitPptRawParagraphRunesMultipleParagraphs(t *testing.T) {
	paras := splitPptRawParagraphRunes("Hello\rWorld")
	if len(paras) != 2 {
		t.Fatalf("len = %d, want 2", len(paras))
	}
	if string(paras[0]) != "Hello\r" {
		t.Fatalf("para[0] = %q, want %q", string(paras[0]), "Hello\\r")
	}
	if string(paras[1]) != "World" {
		t.Fatalf("para[1] = %q, want %q", string(paras[1]), "World")
	}
}

func TestSplitPptRawParagraphRunesTrailingCR(t *testing.T) {
	paras := splitPptRawParagraphRunes("Hello\r")
	if len(paras) != 1 {
		t.Fatalf("len = %d, want 1", len(paras))
	}
	if string(paras[0]) != "Hello\r" {
		t.Fatalf("para[0] = %q", string(paras[0]))
	}
}

// --- parseStyleTextPropAtom integration tests ---

func TestParseStyleTextPropAtomBoldRun(t *testing.T) {
	// Single paragraph "Hi", one bold CF run covering both chars + CR.
	rawText := "Hi\r"

	// PF section: one entry for the paragraph (cchPara=3, masks=0 → no fields).
	pf := buildPFRecord(3, 0, nil)

	// CF section: one run covering 3 chars, bold=true (masks bit 0, grpfBasicAtom 0x01).
	cf := buildCFRecord(3, 0x01, []byte{0x01, 0x00})

	stpa := append(pf, cf...)
	paras := parseStyleTextPropAtom(rawText, stpa)
	if paras == nil {
		t.Fatal("parseStyleTextPropAtom returned nil")
	}
	if len(paras) != 1 {
		t.Fatalf("len(paras) = %d, want 1", len(paras))
	}
	if len(paras[0].Runs) == 0 {
		t.Fatal("no runs in paragraph")
	}
	run := paras[0].Runs[0]
	if run.Text != "Hi" {
		t.Fatalf("run.Text = %q, want %q", run.Text, "Hi")
	}
	if !run.Bold {
		t.Fatal("run.Bold should be true")
	}
}

func TestParseStyleTextPropAtomMultipleParagraphs(t *testing.T) {
	// Two paragraphs: "Title\r" (center-aligned) and "Body".
	rawText := "Title\rBody"

	// PF: para1 (cchPara=6, textAlignment=center), para2 (cchPara=4, no alignment).
	pf1 := buildPFRecord(6, uint32(1<<16), marshalUint16LE(1)) // center
	pf2 := buildPFRecord(4, 0, nil)

	// CF: one unformatted run covering all 10 chars.
	cf := buildCFRecord(10, 0, nil)

	stpa := append(append(pf1, pf2...), cf...)
	paras := parseStyleTextPropAtom(rawText, stpa)
	if len(paras) != 2 {
		t.Fatalf("len(paras) = %d, want 2", len(paras))
	}
	if paras[0].Align != "ctr" {
		t.Fatalf("paras[0].Align = %q, want %q", paras[0].Align, "ctr")
	}
	if paras[1].Align != "" {
		t.Fatalf("paras[1].Align = %q, want empty", paras[1].Align)
	}
	if paragraphText(paras[0]) != "Title" {
		t.Fatalf("paras[0] text = %q, want %q", paragraphText(paras[0]), "Title")
	}
	if paragraphText(paras[1]) != "Body" {
		t.Fatalf("paras[1] text = %q, want %q", paragraphText(paras[1]), "Body")
	}
}

func TestParseStyleTextPropAtomMixedRunFormats(t *testing.T) {
	// One paragraph "AB": run1 (char 0 = 'A') bold, run2 (char 1 = 'B') italic.
	rawText := "AB"

	// PF: one para, 2 chars (no trailing CR), masks=0.
	pf := buildPFRecord(2, 0, nil)

	// CF run1: cch=1, bold.
	cf1 := buildCFRecord(1, 0x01, []byte{0x01, 0x00})
	// CF run2: cch=1, italic.
	cf2 := buildCFRecord(1, 0x02, []byte{0x02, 0x00})

	stpa := append(append(pf, cf1...), cf2...)
	paras := parseStyleTextPropAtom(rawText, stpa)
	if paras == nil {
		t.Fatal("parseStyleTextPropAtom returned nil")
	}
	if len(paras[0].Runs) != 2 {
		t.Fatalf("runs count = %d, want 2", len(paras[0].Runs))
	}
	if paras[0].Runs[0].Text != "A" || !paras[0].Runs[0].Bold {
		t.Fatalf("run[0] = %+v, want Bold A", paras[0].Runs[0])
	}
	if paras[0].Runs[1].Text != "B" || !paras[0].Runs[1].Italic {
		t.Fatalf("run[1] = %+v, want Italic B", paras[0].Runs[1])
	}
}

func TestParseStyleTextPropAtomBulletChar(t *testing.T) {
	// One paragraph with a bullet char (•).
	rawText := "Item\r"

	// PF: bit 0 (hasBullet) + bit 7 (bulletChar); bulletFlags=0x01, bulletChar='•'.
	pfMasks := uint32((1 << 0) | (1 << 7))
	pfExtra := append(marshalUint16LE(0x0001), marshalUint16LE(0x2022)...)
	pf := buildPFRecord(5, pfMasks, pfExtra)

	// CF: one run, no formatting.
	cf := buildCFRecord(5, 0, nil)

	stpa := append(pf, cf...)
	paras := parseStyleTextPropAtom(rawText, stpa)
	if paras == nil {
		t.Fatal("parseStyleTextPropAtom returned nil")
	}
	if paras[0].BulletChar != "•" {
		t.Fatalf("BulletChar = %q, want %q", paras[0].BulletChar, "•")
	}
}

func TestParseStyleTextPropAtomFontSizeAndColor(t *testing.T) {
	// One run with fontSize=3600 (36pt) and color=FF0000.
	rawText := "Hi"

	pf := buildPFRecord(2, 0, nil)
	cfMasks := uint32((1 << 10) | (1 << 11)) // fontSize + color
	cfExtra := marshalUint16LE(3600)
	cfExtra = append(cfExtra, 0xFF, 0x00, 0x00, 0xFE) // explicit sRGB
	cf := buildCFRecord(2, cfMasks, cfExtra)

	stpa := append(pf, cf...)
	paras := parseStyleTextPropAtom(rawText, stpa)
	if len(paras) == 0 || len(paras[0].Runs) == 0 {
		t.Fatal("no paragraphs/runs")
	}
	run := paras[0].Runs[0]
	if run.FontSize != 3600 {
		t.Fatalf("FontSize = %d, want 3600", run.FontSize)
	}
	if run.Color != "FF0000" {
		t.Fatalf("Color = %q, want FF0000", run.Color)
	}
}

func TestParseStyleTextPropAtomEmptyPayloadReturnsNil(t *testing.T) {
	paras := parseStyleTextPropAtom("Hello", nil)
	if paras != nil {
		t.Fatal("expected nil for empty stpa")
	}
}

func TestParseStyleTextPropAtomEmptyTextReturnsNil(t *testing.T) {
	paras := parseStyleTextPropAtom("", []byte{0x00, 0x00, 0x00, 0x00})
	if paras != nil {
		t.Fatal("expected nil for empty rawText")
	}
}

// --- pptAlignString tests ---

func TestPptAlignString(t *testing.T) {
	cases := []struct {
		v    uint16
		want string
	}{
		{0, "l"},
		{1, "ctr"},
		{2, "r"},
		{3, "just"},
		{4, "just"},
		{5, ""},
		{99, ""},
	}
	for _, c := range cases {
		got := pptAlignString(c.v)
		if got != c.want {
			t.Errorf("pptAlignString(%d) = %q, want %q", c.v, got, c.want)
		}
	}
}

// --- helpers ---

func marshalUint32LE(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func marshalUint16LE(v uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

func marshalInt16LE(v int16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	return b
}

func marshalInt32LE(v int32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return b
}

// --- TextPFException paragraph-spacing tests ---

func TestParsePFExceptionSpaceBeforeAfterAbsolute(t *testing.T) {
	// MS-PPT ParaSpacing: negative values are absolute master units.
	masks := uint32(1<<18 | 1<<19)
	extra := append(marshalInt16LE(-200), marshalInt16LE(-100)...)
	stpa := append(marshalUint32LE(masks), extra...)
	entry, _, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if !entry.HasSpaceBefore || entry.SpaceBefore != 200 || entry.SpaceBeforePct != 0 {
		t.Fatalf("space before = abs:%d has:%v pct:%d, want 200/true/0", entry.SpaceBefore, entry.HasSpaceBefore, entry.SpaceBeforePct)
	}
	if !entry.HasSpaceAfter || entry.SpaceAfter != 100 || entry.SpaceAfterPct != 0 {
		t.Fatalf("space after = abs:%d has:%v pct:%d, want 100/true/0", entry.SpaceAfter, entry.HasSpaceAfter, entry.SpaceAfterPct)
	}
}

func TestParsePFExceptionSpaceBeforeAfterPct(t *testing.T) {
	// MS-PPT ParaSpacing: non-negative values are percentages of text line height.
	masks := uint32(1<<18 | 1<<19)
	extra := append(marshalInt16LE(125), marshalInt16LE(50)...)
	stpa := append(marshalUint32LE(masks), extra...)
	entry, _, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if entry.HasSpaceBefore || entry.SpaceBeforePct != 125000 {
		t.Fatalf("space before = has:%v pct:%d, want false/125000", entry.HasSpaceBefore, entry.SpaceBeforePct)
	}
	if entry.HasSpaceAfter || entry.SpaceAfterPct != 50000 {
		t.Fatalf("space after = has:%v pct:%d, want false/50000", entry.HasSpaceAfter, entry.SpaceAfterPct)
	}
}

func TestParsePFExceptionLineSpacingPct(t *testing.T) {
	masks := uint32(1 << 17)
	stpa := append(marshalUint32LE(masks), marshalInt16LE(150)...)
	entry, _, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if !entry.HasLineSpacing || entry.LineSpacingPct != 150000 || entry.LineSpacingPt != 0 {
		t.Fatalf("line spacing = has:%v pct:%d pt:%d, want true/150000/0", entry.HasLineSpacing, entry.LineSpacingPct, entry.LineSpacingPt)
	}
}

func TestParsePFExceptionLineSpacingAbsolute(t *testing.T) {
	masks := uint32(1 << 17)
	stpa := append(marshalUint32LE(masks), marshalInt16LE(-240)...)
	entry, _, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if !entry.HasLineSpacing || entry.LineSpacingPt != 240 || entry.LineSpacingPct != 0 {
		t.Fatalf("line spacing = has:%v pt:%d pct:%d, want true/240/0", entry.HasLineSpacing, entry.LineSpacingPt, entry.LineSpacingPct)
	}
}

func TestParsePFExceptionBulletColorAndSize(t *testing.T) {
	// bit 5 (bulletColor) + bit 6 (bulletSize).
	masks := uint32(1<<5 | 1<<6)
	// bulletColor: R=255, G=128, B=0, index=0xFE (explicit RGB)
	extra := []byte{0xFF, 0x80, 0x00, 0xFE}
	// bulletSize: -18 means an absolute 18 pt marker per MS-PPT BulletSize.
	extra = append(extra, marshalInt16LE(-18)...)
	stpa := append(marshalUint32LE(masks), extra...)
	entry, _, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if entry.BulletColor != "FF8000" {
		t.Fatalf("BulletColor = %q, want FF8000", entry.BulletColor)
	}
	if entry.BulletSize != 1800 {
		t.Fatalf("BulletSize = %d, want 1800", entry.BulletSize)
	}
}

func TestParsePFExceptionBulletColorSchemeWithoutPaletteStaysEmpty(t *testing.T) {
	// index 0x06 means scheme color; without a parsed palette, BulletColor remains empty.
	masks := uint32(1 << 5)
	extra := []byte{0x00, 0x00, 0x00, 0x06}
	stpa := append(marshalUint32LE(masks), extra...)
	entry, _, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if entry.BulletColor != "" {
		t.Fatalf("BulletColor = %q, want empty (scheme color)", entry.BulletColor)
	}
}

func TestParsePFExceptionBulletColorSchemeResolvesFromPalette(t *testing.T) {
	masks := uint32(1 << 5)
	extra := []byte{0x00, 0x00, 0x00, 0x06}
	stpa := append(marshalUint32LE(masks), extra...)
	entry, _, ok := parsePFExceptionWithTheme(stpa, 0, map[uint8]string{6: "0000FF"})
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if entry.BulletColor != "0000FF" {
		t.Fatalf("BulletColor = %q, want %q", entry.BulletColor, "0000FF")
	}
}

func TestParsePFExceptionMarginAndIndent(t *testing.T) {
	// bits 8 (leftMargin) + 9 (indent).  1000 master units × 127 = 127000 EMU.
	masks := uint32(1<<8 | 1<<9)
	extra := append(marshalInt32LE(1000), marshalInt32LE(-500)...)
	stpa := append(marshalUint32LE(masks), extra...)
	entry, _, ok := parsePFException(stpa, 0)
	if !ok {
		t.Fatal("parsePFException returned ok=false")
	}
	if !entry.HasMarginLeft {
		t.Fatal("HasMarginLeft should be true")
	}
	if entry.MarginLeftEMU != 127000 {
		t.Fatalf("MarginLeftEMU = %d, want 127000", entry.MarginLeftEMU)
	}
	if !entry.HasTextIndent {
		t.Fatal("HasTextIndent should be true")
	}
	if entry.TextIndentEMU != -63500 {
		t.Fatalf("TextIndentEMU = %d, want -63500", entry.TextIndentEMU)
	}
}

// --- StyleTextProp9Atom unit tests ---

func TestParseStyleTextProp9AtomEmptyPayload(t *testing.T) {
	result := parseStyleTextProp9Atom(nil)
	if result != nil {
		t.Fatalf("expected nil result for empty payload, got %v", result)
	}
}

func TestParseStyleTextProp9AtomSingleLevelArabicPeriod(t *testing.T) {
	// Level 0: mask3=0x00, mask4=0x03 (fBulletHasAutoNumber + scheme/start present)
	// hasAutoNum=1, schemeID=3 (arabicPeriod), startNum=1
	payload := []byte{
		0x00, 0x00, 0x00, 0x03, // mask3=0, mask4=0x03
		0x01, 0x00, // hasAutoNum=1
		0x03, 0x00, 0x01, 0x00, // schemeID=3 (arabicPeriod), startNum=1
	}
	result := parseStyleTextProp9Atom(payload)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	e, ok := result[0]
	if !ok {
		t.Fatal("expected entry at level 0")
	}
	if !e.hasBulletAutoNumber {
		t.Fatal("hasBulletAutoNumber should be true")
	}
	if e.autoNumberScheme != "arabicPeriod" {
		t.Fatalf("autoNumberScheme = %q, want %q", e.autoNumberScheme, "arabicPeriod")
	}
	if e.autoNumberStart != 1 {
		t.Fatalf("autoNumberStart = %d, want 1", e.autoNumberStart)
	}
}

func TestParseStyleTextProp9AtomAutoNumWithCustomStart(t *testing.T) {
	// mask4=0x03: hasAutoNum=1, schemeID=2 (arabicParenRight), startNum=5
	payload := []byte{
		0x00, 0x00, 0x00, 0x03,
		0x01, 0x00, // hasAutoNum=1
		0x02, 0x00, 0x05, 0x00, // schemeID=2, startNum=5
	}
	result := parseStyleTextProp9Atom(payload)
	e := result[0]
	if e.autoNumberScheme != "arabicParenR" {
		t.Fatalf("autoNumberScheme = %q, want %q", e.autoNumberScheme, "arabicParenR")
	}
	if e.autoNumberStart != 5 {
		t.Fatalf("autoNumberStart = %d, want 5", e.autoNumberStart)
	}
}

func TestParseStyleTextProp9AtomAutoNumFalseSkipsScheme(t *testing.T) {
	// hasAutoNum=0 but scheme fields present: scheme should stay empty
	payload := []byte{
		0x00, 0x00, 0x00, 0x03,
		0x00, 0x00, // hasAutoNum=0
		0x03, 0x00, 0x01, 0x00, // schemeID=3, startNum=1 (payload preserved; enable flag is false)
	}
	result := parseStyleTextProp9Atom(payload)
	e := result[0]
	if e.hasBulletAutoNumber {
		t.Fatal("hasBulletAutoNumber should be false")
	}
	if e.autoNumberScheme != "arabicPeriod" {
		t.Fatalf("autoNumberScheme = %q, want preserved arabicPeriod payload", e.autoNumberScheme)
	}
	if !e.hasBulletAutoNumberSpecified || e.hasBulletAutoNumber {
		t.Fatalf("auto-number enable provenance = %#v, want explicit false", e)
	}
}

func TestParseStyleTextProp9AtomDoesNotInventSchemeWhenOmitted(t *testing.T) {
	// hasAutoNum=1 but no scheme/start fields (mask4=0x02 only)
	payload := []byte{
		0x00, 0x00, 0x00, 0x02,
		0x01, 0x00, // hasAutoNum=1
	}
	result := parseStyleTextProp9Atom(payload)
	e := result[0]
	if !e.hasBulletAutoNumberSpecified || !e.hasBulletAutoNumber {
		t.Fatalf("auto-number presence = %#v", e)
	}
	if e.autoNumberScheme != "" || e.autoNumberStart != 0 {
		t.Fatalf("omitted scheme was manufactured: %#v", e)
	}
}

func TestParseStyleTextProp9AtomMultipleLevels(t *testing.T) {
	// Two levels: level 0 with autoNum, level 1 without
	level0 := []byte{0x00, 0x00, 0x00, 0x03, 0x01, 0x00, 0x03, 0x00, 0x01, 0x00}
	level1 := []byte{0x00, 0x00, 0x00, 0x00} // no fields
	payload := append(level0, level1...)
	result := parseStyleTextProp9Atom(payload)
	if _, ok := result[0]; !ok {
		t.Fatal("expected entry at level 0")
	}
	if _, ok := result[1]; !ok {
		t.Fatal("expected entry at level 1")
	}
	if result[0].autoNumberScheme != "arabicPeriod" {
		t.Fatalf("level 0 autoNumberScheme = %q, want arabicPeriod", result[0].autoNumberScheme)
	}
	if result[1].hasBulletAutoNumber {
		t.Fatal("level 1 hasBulletAutoNumber should be false")
	}
}

func TestApplyTextPFException9ToEntryMergesAutoNum(t *testing.T) {
	entry := &stpaPFEntry{}
	pp9Data := map[int]textPFException9{
		0: {hasBulletAutoNumber: true, autoNumberScheme: "arabicPeriod", autoNumberStart: 3},
	}
	applyTextPFException9ToEntry(entry, 0, pp9Data)
	if !entry.AutoNum {
		t.Fatal("AutoNum should be true after apply")
	}
	if entry.AutoNumType != "arabicPeriod" {
		t.Fatalf("AutoNumType = %q, want arabicPeriod", entry.AutoNumType)
	}
	if entry.AutoNumStart != 3 {
		t.Fatalf("AutoNumStart = %d, want 3", entry.AutoNumStart)
	}
}

func TestApplyTextPFException9ToEntryNoMatchingLevel(t *testing.T) {
	entry := &stpaPFEntry{}
	pp9Data := map[int]textPFException9{
		1: {hasBulletAutoNumber: true, autoNumberScheme: "arabicPeriod", autoNumberStart: 1},
	}
	applyTextPFException9ToEntry(entry, 0, pp9Data) // level 0 not in map
	if entry.AutoNum {
		t.Fatal("AutoNum should remain false when level not in map")
	}
}

// --- StyleTextPropAtom spec-path + boundary/continuation regressions ---

func buildSpecPFRecord(count uint32, level uint16, masks uint32, extra []byte) []byte {
	b := make([]byte, 4+2+4+len(extra))
	binary.LittleEndian.PutUint32(b[0:4], count)
	binary.LittleEndian.PutUint16(b[4:6], level)
	binary.LittleEndian.PutUint32(b[6:10], masks)
	copy(b[10:], extra)
	return b
}

func buildSpecCFRecord(cch uint32, masks uint32, extra []byte) []byte {
	b := make([]byte, 4+4+len(extra))
	binary.LittleEndian.PutUint32(b[0:4], cch)
	binary.LittleEndian.PutUint32(b[4:8], masks)
	copy(b[8:], extra)
	return b
}

func TestParseStyleTextPropAtomSpecPathExactCounts(t *testing.T) {
	// Two paragraphs: "Hi\r" (3 chars) + "Bye" (implicit CR → 4 chars).
	// totalChars = 7.  Uses spec-format 4-byte counts.
	rawText := "Hi\rBye"

	pf1 := buildSpecPFRecord(3, 0, 0, nil)
	pf2 := buildSpecPFRecord(4, 0, 0, nil)
	cf1 := buildSpecCFRecord(3, 0x01, []byte{0x01, 0x00}) // bold
	cf2 := buildSpecCFRecord(4, 0x02, []byte{0x02, 0x00}) // italic

	stpa := append(append(pf1, pf2...), append(cf1, cf2...)...)
	paras := parseStyleTextPropAtom(rawText, stpa)
	if paras == nil {
		t.Fatal("expected spec path to succeed")
	}
	if len(paras) != 2 {
		t.Fatalf("len = %d, want 2", len(paras))
	}
	if paragraphText(paras[0]) != "Hi" {
		t.Fatalf("para0 text = %q, want Hi", paragraphText(paras[0]))
	}
	if len(paras[0].Runs) != 1 || !paras[0].Runs[0].Bold {
		t.Fatalf("para0 run should be bold")
	}
	if paragraphText(paras[1]) != "Bye" {
		t.Fatalf("para1 text = %q, want Bye", paragraphText(paras[1]))
	}
	if len(paras[1].Runs) != 1 || !paras[1].Runs[0].Italic {
		t.Fatalf("para1 run should be italic")
	}
}

func TestParseStyleTextPropAtomSpecPathFallsBackToLegacy(t *testing.T) {
	// Pass a legacy-format STPA (2-byte counts) to the top-level parser.
	// Spec path should fail because the CF section is too short for 4-byte
	// cch + masks + extra, then legacy recovers correctly.
	rawText := "Hi"
	pf := buildPFRecord(3, 0, nil)                   // legacy PF cch=3 (covers Hi + implicit CR)
	cf := buildCFRecord(3, 0x01, []byte{0x01, 0x00}) // legacy CF bold
	stpa := append(pf, cf...)
	paras := parseStyleTextPropAtom(rawText, stpa)
	if paras == nil {
		t.Fatal("expected legacy fallback to succeed")
	}
	if len(paras) != 1 {
		t.Fatalf("len = %d, want 1", len(paras))
	}
	if paragraphText(paras[0]) != "Hi" {
		t.Fatalf("text = %q, want Hi", paragraphText(paras[0]))
	}
	if len(paras[0].Runs) != 1 || !paras[0].Runs[0].Bold {
		t.Fatal("run should be bold")
	}
}

func TestParseStyleTextPropAtomLegacyPartialCFRunCoverage(t *testing.T) {
	// Text "ABCD" (4 chars). Only first 2 chars have CF formatting.
	// Remaining text should emit as a plain continuation run.
	rawText := "ABCD"
	pf := buildPFRecord(4, 0, nil)
	cf := buildCFRecord(2, 0x01, []byte{0x01, 0x00}) // bold, covers AB only
	stpa := append(pf, cf...)
	paras := parseStyleTextPropAtom(rawText, stpa)
	if paras == nil {
		t.Fatal("expected legacy fallback to succeed")
	}
	if len(paras) != 1 {
		t.Fatalf("len = %d, want 1", len(paras))
	}
	runs := paras[0].Runs
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	if runs[0].Text != "AB" || !runs[0].Bold {
		t.Fatalf("run0 = %+v, want bold AB", runs[0])
	}
	if runs[1].Text != "CD" || runs[1].Bold {
		t.Fatalf("run1 = %+v, want plain CD", runs[1])
	}
}

func TestParseStyleTextPropAtomLegacyMissingCFRuns(t *testing.T) {
	// Text with PF but zero CF runs → single plain-text paragraph.
	rawText := "Hi"
	pf := buildPFRecord(2, 0, nil)
	stpa := pf
	paras := parseStyleTextPropAtom(rawText, stpa)
	if paras == nil {
		t.Fatal("expected legacy fallback to succeed")
	}
	if len(paras) != 1 {
		t.Fatalf("len = %d, want 1", len(paras))
	}
	runs := paras[0].Runs
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if runs[0].Text != "Hi" || runs[0].Bold {
		t.Fatalf("run = %+v, want plain Hi", runs[0])
	}
}
func TestChooseStyleTextPropCandidateRejectsMisdecodedOversizeCFRun(t *testing.T) {
	spec := []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Need", FontSize: 72000, Color: "400202"}}}}
	legacy := []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Need", FontSize: 1800}}}}

	got := chooseStyleTextPropCandidate(spec, legacy)
	if len(got) != 1 || len(got[0].Runs) != 1 || got[0].Runs[0].FontSize != 1800 {
		t.Fatalf("candidate selection kept misdecoded 720pt run: %#v", got)
	}
}

func TestChooseStyleTextPropCandidateKeepsValidSpecCandidate(t *testing.T) {
	spec := []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Title", FontSize: 4400}}}}
	legacy := []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Title", FontSize: 1800}}}}

	got := chooseStyleTextPropCandidate(spec, legacy)
	if len(got) != 1 || len(got[0].Runs) != 1 || got[0].Runs[0].FontSize != 4400 {
		t.Fatalf("candidate selection replaced valid spec run: %#v", got)
	}
}

func TestStyleTextPropCandidatePenaltyCatchesControlBulletAndDecorationCollision(t *testing.T) {
	p := &presentationir.Paragraph{
		BulletChar: string(rune(0x18)),
		Runs: []*presentationir.Run{{
			Text: "nested", Bold: true, BoldSet: true, Italic: true, ItalicSet: true,
			Underline: true, UnderlineSet: true, Strike: true, StrikeSet: true,
		}},
	}
	if got := styleTextPropCandidatePenalty([]*presentationir.Paragraph{p}); got < 12 {
		t.Fatalf("penalty = %d, want control-bullet + decoration collision", got)
	}
}

func TestStyleTextPropCandidateRejectsBOMBulletCompatibilityMisread(t *testing.T) {
	bad := []*presentationir.Paragraph{{
		BulletChar: "\uFEFF",
		Runs:       []*presentationir.Run{{Text: "Global Positioning System", FontFamily: "Symbol", Bold: true, BoldSet: true, Italic: true, ItalicSet: true, Underline: true, UnderlineSet: true, Strike: true, StrikeSet: true}},
	}}
	good := []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Global Positioning System"}}}}
	got := chooseStyleTextPropCandidateForText("Global Positioning System", bad, good)
	if len(got) != 1 || got[0] != good[0] {
		t.Fatal("compatibility candidate with BOM bullet/decorated Symbol misread must not win")
	}
}

func TestStyleTextPropCandidateAllowsAuthoredSymbolGlyphRun(t *testing.T) {
	symbol := []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "αβγ", FontFamily: "Symbol"}}}}
	plain := []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "αβγ"}}}}
	got := chooseStyleTextPropCandidateForText("αβγ", symbol, plain)
	if len(got) != 1 || got[0] != symbol[0] {
		t.Fatal("legitimate non-Latin Symbol run should not be rejected")
	}
}

func TestNormalizePptSoftLineBreaksDoesNotCreateParagraphMark(t *testing.T) {
	if got := normalizePptSoftLineBreaks("Alpha\vBeta\fGamma"); got != "Alpha\nBeta\nGamma" {
		t.Fatalf("soft breaks = %q", got)
	}
}

func TestPFExceptionNormalizesBulletSizePercentageToCanonicalUnits(t *testing.T) {
	// PFMasks bulletHasSize (bit 3) makes BulletFlags.fBulletHasSize valid;
	// bulletSize (bit 6) then carries the 125% payload.
	data := make([]byte, 0, 8)
	data = binary.LittleEndian.AppendUint32(data, (1<<3)|(1<<6))
	data = binary.LittleEndian.AppendUint16(data, 1<<3) // fBulletHasSize=true
	data = binary.LittleEndian.AppendUint16(data, uint16(125))
	got, _, ok := parsePFExceptionSpecWithTheme(data, 0, nil, nil)
	if !ok {
		t.Fatal("parse failed")
	}
	if got.BulletSizePct != 125000 || got.BulletSize != 0 {
		t.Fatalf("bullet size = pct %d / abs %d, want 125000 / 0", got.BulletSizePct, got.BulletSize)
	}
}

func TestPFExceptionNormalizesAbsoluteBulletSizeToHundredthsOfPoint(t *testing.T) {
	data := make([]byte, 0, 8)
	data = binary.LittleEndian.AppendUint32(data, (1<<3)|(1<<6))
	data = binary.LittleEndian.AppendUint16(data, 1<<3) // fBulletHasSize=true
	data = binary.LittleEndian.AppendUint16(data, uint16(0xffee))
	got, _, ok := parsePFExceptionSpecWithTheme(data, 0, nil, nil)
	if !ok {
		t.Fatal("parse failed")
	}
	if got.BulletSize != 1800 || got.BulletSizePct != 0 {
		t.Fatalf("bullet size = abs %d / pct %d, want 1800 / 0", got.BulletSize, got.BulletSizePct)
	}
}

func TestLegacyPFExceptionUsesSameBulletSizeContract(t *testing.T) {
	positive := make([]byte, 0, 6)
	positive = binary.LittleEndian.AppendUint32(positive, 1<<6)
	positive = binary.LittleEndian.AppendUint16(positive, uint16(80))
	got, _, ok := parsePFExceptionLegacyWithTheme(positive, 0, nil, nil)
	if !ok || got.BulletSizePct != 80000 || got.BulletSize != 0 {
		t.Fatalf("positive legacy bullet size = %#v, want 80%% canonical percentage", got)
	}

	negative := make([]byte, 0, 6)
	negative = binary.LittleEndian.AppendUint32(negative, 1<<6)
	negative = binary.LittleEndian.AppendUint16(negative, uint16(0xffe8))
	got, _, ok = parsePFExceptionLegacyWithTheme(negative, 0, nil, nil)
	if !ok || got.BulletSize != 2400 || got.BulletSizePct != 0 {
		t.Fatalf("negative legacy bullet size = %#v, want 24pt absolute", got)
	}
}

func TestPFExceptionConsumesCombinedBulletFieldsInMSPPTStructureOrder(t *testing.T) {
	// MS-PPT 2.9.20: bulletFlags, bulletChar, bulletFontRef, bulletSize,
	// bulletColor. PFMasks bit order is not the serialized field order.
	masks := uint32(0x0f | (1 << 4) | (1 << 5) | (1 << 6) | (1 << 7))
	data := binary.LittleEndian.AppendUint32(nil, masks)
	data = binary.LittleEndian.AppendUint16(data, 0x000f) // all BulletFlags valid
	data = binary.LittleEndian.AppendUint16(data, 0x2022) // bulletChar
	data = binary.LittleEndian.AppendUint16(data, 7)      // bulletFontRef
	data = binary.LittleEndian.AppendUint16(data, 125)    // bulletSize = 125%
	data = append(data, 0x11, 0x22, 0x33, 0xFE)           // bulletColor: explicit sRGB

	got, pos, ok := parsePFExceptionSpecWithTheme(data, 0, nil, map[uint16]string{7: "Wingdings"})
	if !ok || pos != len(data) {
		t.Fatalf("parse = ok %v pos %d/%d", ok, pos, len(data))
	}
	if !got.HasBullet || got.BulletChar != '•' || got.BulletFont != "Wingdings" {
		t.Fatalf("bullet identity = %#v", got)
	}
	if got.BulletSizePct != 125000 || got.BulletSize != 0 {
		t.Fatalf("bullet size = pct %d abs %d", got.BulletSizePct, got.BulletSize)
	}
	if got.BulletColor != "112233" {
		t.Fatalf("bullet color = %q, want 112233", got.BulletColor)
	}
}


func TestPFExceptionBulletPayloadIsConsumedButIgnoredWhenBulletFlagsDoNotValidateIt(t *testing.T) {
	// All payload fields physically exist, but PFMasks only validates fHasBullet.
	// MS-PPT requires bullet font/color/size to be semantically ignored unless
	// their corresponding BulletFlags validity bits are themselves valid and true.
	masks := uint32((1 << 0) | (1 << 4) | (1 << 5) | (1 << 6) | (1 << 7))
	data := binary.LittleEndian.AppendUint32(nil, masks)
	data = binary.LittleEndian.AppendUint16(data, 0x0001) // only fHasBullet=true is valid
	data = binary.LittleEndian.AppendUint16(data, 0x2022)
	data = binary.LittleEndian.AppendUint16(data, 7)
	data = binary.LittleEndian.AppendUint16(data, 125)
	data = append(data, 0x11, 0x22, 0x33, 0xFE)

	got, pos, ok := parsePFExceptionSpecWithTheme(data, 0, nil, map[uint16]string{7: "Wingdings"})
	if !ok || pos != len(data) {
		t.Fatalf("parse = ok %v pos %d/%d", ok, pos, len(data))
	}
	if !got.HasBulletSpecified || !got.HasBullet || got.BulletChar != '•' {
		t.Fatalf("bullet presence/char = %#v", got)
	}
	if got.BulletFont != "" || got.BulletColor != "" || got.BulletSize != 0 || got.BulletSizePct != 0 {
		t.Fatalf("invalid bullet payload leaked into semantics: %#v", got)
	}
}

func TestPFExceptionPreservesExplicitNoBulletProvenance(t *testing.T) {
	masks := uint32(1 << 0) // hasBullet validity only
	data := binary.LittleEndian.AppendUint32(nil, masks)
	data = binary.LittleEndian.AppendUint16(data, 0x0000) // authoritative fHasBullet=false

	got, pos, ok := parsePFExceptionSpecWithTheme(data, 0, nil, nil)
	if !ok || pos != len(data) {
		t.Fatalf("parse = ok %v pos %d/%d", ok, pos, len(data))
	}
	if !got.HasBulletSpecified || got.HasBullet {
		t.Fatalf("explicit no-bullet provenance = %#v", got)
	}
}


func TestAutoNumberSchemeStringUsesMSPPTEnumValues(t *testing.T) {
	cases := map[uint16]string{
		0x0000: "alphaLcPeriod",
		0x0001: "alphaUcPeriod",
		0x0002: "arabicParenR",
		0x0003: "arabicPeriod",
		0x0007: "romanUcPeriod",
		0x000D: "arabicPlain",
		0x000F: "romanUcParenR",
	}
	for id, want := range cases {
		if got := autoNumberSchemeString(id); got != want {
			t.Fatalf("scheme %#04x = %q, want %q", id, got, want)
		}
	}
}

func TestApplyTextPFException9ExplicitFalseClearsAutoNumber(t *testing.T) {
	entry := &stpaPFEntry{AutoNum: true, AutoNumType: "arabicPeriod", AutoNumStart: 4}
	pp9Data := map[int]textPFException9{
		0: {hasBulletAutoNumberSpecified: true, hasBulletAutoNumber: false},
	}
	applyTextPFException9ToEntry(entry, 0, pp9Data)
	if entry.AutoNum || entry.AutoNumType != "" || entry.AutoNumStart != 0 {
		t.Fatalf("explicit no-auto-number did not clear inherited state: %#v", entry)
	}
}


func TestPFExceptionPreservesExplicitZeroDefaultTabSizeProvenance(t *testing.T) {
	data := binary.LittleEndian.AppendUint32(nil, 1<<15)
	data = binary.LittleEndian.AppendUint16(data, 0)
	got, pos, ok := parsePFExceptionSpecWithTheme(data, 0, nil, nil)
	if !ok || pos != len(data) {
		t.Fatalf("parse = ok %v pos %d/%d", ok, pos, len(data))
	}
	if !got.HasDefaultTabSize || got.DefaultTabSizeEMU != 0 {
		t.Fatalf("explicit zero default tab lost: %#v", got)
	}
}

func TestPFExceptionParsesAuthoredTabStopAlignmentAndPosition(t *testing.T) {
	data := binary.LittleEndian.AppendUint32(nil, 1<<20)
	data = binary.LittleEndian.AppendUint16(data, 4)
	for _, tc := range []struct{ pos int16; typ uint16 }{
		{100, 0}, {200, 1}, {300, 2}, {400, 3},
	} {
		data = binary.LittleEndian.AppendUint16(data, uint16(tc.pos))
		data = binary.LittleEndian.AppendUint16(data, tc.typ)
	}
	got, pos, ok := parsePFExceptionSpecWithTheme(data, 0, nil, nil)
	if !ok || pos != len(data) {
		t.Fatalf("parse = ok %v pos %d/%d", ok, pos, len(data))
	}
	want := []string{"l", "ctr", "r", "dec"}
	if len(got.TabStops) != 4 {
		t.Fatalf("tab stops = %#v", got.TabStops)
	}
	for i, stop := range got.TabStops {
		if stop.Algn != want[i] || stop.Pos != int64((i+1)*100*127) {
			t.Fatalf("tab %d = %#v", i, stop)
		}
	}
}
