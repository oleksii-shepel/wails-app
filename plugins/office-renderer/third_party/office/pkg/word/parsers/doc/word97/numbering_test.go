package word97

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

func canonicalTestLVL(start int, nfc byte, template string) []byte {
	header := make([]byte, 28)
	binary.LittleEndian.PutUint32(header[:4], uint32(start))
	header[4] = nfc
	header[5] = 1 << 3 // fNoRestart
	runes := []rune(template)
	outRunes := make([]rune, 0, len(runes))
	placeholderCount := 0
	for i := 0; i < len(runes); i++ {
		if runes[i] == '%' && i+1 < len(runes) && runes[i+1] >= '1' && runes[i+1] <= '9' {
			level := int(runes[i+1] - '1')
			header[6+placeholderCount] = byte(len(utf16.Encode(outRunes)) + 1)
			placeholderCount++
			outRunes = append(outRunes, rune(level))
			i++
			continue
		}
		outRunes = append(outRunes, runes[i])
	}
	units := utf16.Encode(outRunes)
	xst := make([]byte, 2+len(units)*2)
	binary.LittleEndian.PutUint16(xst[:2], uint16(len(units)))
	for i, unit := range units {
		binary.LittleEndian.PutUint16(xst[2+i*2:], unit)
	}
	return append(header, xst...)
}

func canonicalTestPlfLst(lsid uint32, level0, level1 []byte) []byte {
	out := make([]byte, 2+28)
	binary.LittleEndian.PutUint16(out[:2], 1)
	binary.LittleEndian.PutUint32(out[2:6], lsid)
	levels := [][]byte{level0, level1}
	for level := 2; level < 9; level++ {
		levels = append(levels, canonicalTestLVL(1, 0, "%1."))
	}
	for _, lvl := range levels {
		out = append(out, lvl...)
	}
	return out
}

func canonicalTestPlfLfo(lsid uint32) []byte {
	out := make([]byte, 4+16+4)
	binary.LittleEndian.PutUint32(out[:4], 1)
	binary.LittleEndian.PutUint32(out[4:8], lsid)
	binary.LittleEndian.PutUint32(out[20:24], 0xFFFFFFFF)
	return out
}

func TestCanonicalNumberingParsesWriterCompatibleStructuredList(t *testing.T) {
	lst := canonicalTestPlfLst(
		700,
		canonicalTestLVL(3, 0, "%1."),
		canonicalTestLVL(1, 4, "%1.%2."),
	)
	lfo := canonicalTestPlfLfo(700)

	abstract, ok := parsePlfLst(lst)
	if !ok {
		t.Fatal("parsePlfLst failed")
	}
	instances, ok := parsePlfLfo(lfo)
	if !ok {
		t.Fatal("parsePlfLfo failed")
	}
	levels := abstract[700]
	if len(levels) != 9 {
		t.Fatalf("levels=%d, want 9", len(levels))
	}
	if levels[0].Start != 3 || levels[0].Format != "decimal" || levels[0].Text != "%1." {
		t.Fatalf("level0=%+v", levels[0])
	}
	if levels[1].Start != 1 || levels[1].Format != "lowerLetter" || levels[1].Text != "%1.%2." {
		t.Fatalf("level1=%+v", levels[1])
	}
	if instances[1].AbstractNumID != 700 {
		t.Fatalf("instance=%+v", instances[1])
	}

	catalog := &NumberingCatalog{Abstract: abstract, Instances: instances}
	doc := &wordir.Document{Paragraphs: []wordir.Paragraph{
		{Numbering: &wordir.Numbering{NumID: 1, Level: 0}},
		{Numbering: &wordir.Numbering{NumID: 1, Level: 1}},
	}}
	catalog.ApplyToDocument(doc)
	if got := doc.Paragraphs[0].List; got == nil || got.Start != 3 || got.Format != "decimal" || got.Template != "%1." {
		t.Fatalf("level0 list=%+v", got)
	}
	if got := doc.Paragraphs[1].List; got == nil || got.Level != 1 || got.Format != "lowerLetter" || got.Template != "%1.%2." {
		t.Fatalf("level1 list=%+v", got)
	}
}

func TestCanonicalNumberingParsesWriterCompatibleBullet(t *testing.T) {
	lst := canonicalTestPlfLst(
		202,
		canonicalTestLVL(1, 23, "•"),
		canonicalTestLVL(1, 0, "%1."),
	)
	abstract, ok := parsePlfLst(lst)
	if !ok {
		t.Fatal("parsePlfLst failed")
	}
	instances, ok := parsePlfLfo(canonicalTestPlfLfo(202))
	if !ok {
		t.Fatal("parsePlfLfo failed")
	}
	catalog := &NumberingCatalog{Abstract: abstract, Instances: instances}
	doc := &wordir.Document{Paragraphs: []wordir.Paragraph{{Numbering: &wordir.Numbering{NumID: 1, Level: 0}}}}
	catalog.ApplyToDocument(doc)
	got := doc.Paragraphs[0].List
	if got == nil || got.Kind != "bullet" || got.Format != "bullet" || got.Template != "•" {
		t.Fatalf("bullet=%+v", got)
	}
}

func TestCanonicalNumberingReadsWord97FibSlotsAndTrailingLVLs(t *testing.T) {
	lst := canonicalTestPlfLst(202, canonicalTestLVL(1, 23, "\uF0B7"), canonicalTestLVL(1, 0, "%1."))
	// Real Word 97 files can leave bullet rgbxchNums zeroed and count only
	// fixed LSTF records in lcbPlfLst, excluding the following LVLs.
	for i := 2 + 28 + 6; i < 2+28+15; i++ {
		lst[i] = 0
	}
	lfo := canonicalTestPlfLfo(202)
	table := append(append([]byte(nil), lst...), lfo...)
	layout := docbinary.Word97Layout{FibBase: 2, CbRgFcLcb: 75}
	fib := make([]byte, layout.FibBase+layout.CbRgFcLcb*8)
	lstPos := layout.FibBase + fibPlfLst*8
	binary.LittleEndian.PutUint32(fib[lstPos:lstPos+4], 0)
	binary.LittleEndian.PutUint32(fib[lstPos+4:lstPos+8], 2+28)
	lfoPos := layout.FibBase + fibPlfLfo*8
	binary.LittleEndian.PutUint32(fib[lfoPos:lfoPos+4], uint32(len(lst)))
	binary.LittleEndian.PutUint32(fib[lfoPos+4:lfoPos+8], uint32(len(lfo)))

	catalog := LoadNumberingCatalogReaderAt(bytes.NewReader(table), int64(len(table)), fib, layout)
	if catalog == nil {
		t.Fatal("numbering catalog was not loaded")
	}
	level := catalog.Abstract[202][0]
	if level.Format != "bullet" || level.Text != "\uF0B7" {
		t.Fatalf("bullet level = %+v", level)
	}
}

func TestCanonicalNumberingParsesPackedLfoStartOverride(t *testing.T) {
	blob := make([]byte, 4+16+4+8)
	binary.LittleEndian.PutUint32(blob[:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 900)
	blob[16] = 1 // LFO.clfolvl at header offset 12
	binary.LittleEndian.PutUint32(blob[20:24], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(blob[24:28], 5)
	binary.LittleEndian.PutUint32(blob[28:32], 1<<4) // ilvl=0 | fStartAt

	instances, ok := parsePlfLfo(blob)
	if !ok {
		t.Fatal("parsePlfLfo failed")
	}
	override := instances[1].Overrides[0]
	if override.Start == nil || *override.Start != 5 {
		t.Fatalf("override=%+v", override)
	}
}

func TestReconstructLevelTextUsesRgbxchNums(t *testing.T) {
	units := []uint16{0, '.', 1, '.'}
	rgb := []byte{1, 3, 0, 0, 0, 0, 0, 0, 0}
	if got := reconstructLevelText(units, rgb); got != "%1.%2." {
		t.Fatalf("template=%q", got)
	}
}
