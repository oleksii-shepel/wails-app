package legacy

import (
	"encoding/binary"
	"os"
	"slices"
	"testing"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

func TestLoadPreWord97PapxEntriesWord95ErrRecoversFirstParagraphRanges(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	raw, err := os.ReadFile("../../../../testdata/doc/word95_cyrillic.doc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		t.Fatal("InspectPreWord97BodySpan = !ok")
	}
	entries := loadPreWord97PapxEntries(wordStream, span)
	if len(entries) == 0 {
		t.Fatal("loadPreWord97PapxEntries = nil, want old PAP paragraph ranges")
	}
	runs := extractPreWord97PapParagraphRuns(wordStream, span)
	if len(runs) == 0 {
		t.Fatal("extractPreWord97PapParagraphRuns = nil, want decoded paragraph runs")
	}
	var got []string
	for _, run := range runs {
		text := normalizeLegacyText(run.Text)
		if text == "" {
			continue
		}
		got = append(got, text)
		if len(got) == 8 {
			break
		}
	}
	want := []string{
		"Компания Аскат М",
		"предлагает со склада в Москве следующий ассортимент настоящих молдавских вин:",
		"ВИНА ОРДИНАРНЫЕ БЕЛЫЕ",
		"Наименование\nпродукции",
		"Изготовитель",
		"Тип бутылки",
		"Объем",
		"Цена (руб.)",
	}
	if len(got) != len(want) {
		t.Fatalf("meaningful old PAP runs = %d, want %d (%#v)", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("run %d = %q, want %q", index, got[index], want[index])
		}
	}
	if entries[0].StartCP != 0 {
		t.Fatalf("first StartCP = %d, want 0", entries[0].StartCP)
	}
	if entries[0].EndCP <= entries[0].StartCP {
		t.Fatalf("first range = %d..%d, want increasing CP range", entries[0].StartCP, entries[0].EndCP)
	}
}

func TestGuessPreWord97CharsetWord95ErrUsesCyrillicFontTable(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	raw, err := os.ReadFile("../../../../testdata/doc/word95_cyrillic.doc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		t.Fatal("InspectPreWord97BodySpan = !ok")
	}
	charset := guessPreWord97Charset(wordStream, span)
	if charset == nil {
		t.Fatal("guessPreWord97Charset = nil, want Cyrillic charset from OldFontTable")
	}
	decoded := decodePreWord97SingleByteTextWithCharset(wordStream[span.Start:span.Start+8], charset)
	if decoded != "Компания" {
		t.Fatalf("font-table decode = %q, want %q", decoded, "Компания")
	}
}

func TestLoadPreWord97PapxEntriesWord95ErrObservedLegacyTogglePairs(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	raw, err := os.ReadFile("../../../../testdata/doc/word95_cyrillic.doc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		t.Fatal("InspectPreWord97BodySpan = !ok")
	}
	pairs := collectPreWord97LegacyTogglePairs(wordStream, span)
	if got := pairs[[2]byte{0x18, 0x01}]; got == 0 {
		t.Fatalf("legacy toggle pair 0x18 0x01 count = %d, want > 0", got)
	}
}

func TestLoadPreWord97PapxEntriesWord95ErrToggle0x19CoOccursWithCellMarkBytes(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	raw, err := os.ReadFile("../../../../testdata/doc/word95_cyrillic.doc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		t.Fatal("InspectPreWord97BodySpan = !ok")
	}
	entries := collectPreWord97PapxEntriesWithToggle(wordStream, span, 0x19)
	if len(entries) == 0 {
		t.Skip("no 0x19 toggle pairs found in fixture; investigation not possible")
	}
	withCellMark := 0
	withoutCellMark := 0
	for _, fc := range entries {
		if fc[1] > fc[0] && fc[1] <= len(wordStream) && wordStream[fc[1]-1] == 0x07 {
			withCellMark++
		} else {
			withoutCellMark++
		}
	}
	t.Logf("0x19 toggle paragraphs: total=%d ending-in-0x07=%d not-0x07=%d", len(entries), withCellMark, withoutCellMark)
	if withCellMark == 0 {
		t.Errorf("0x19 toggle found but none of %d paragraphs end with 0x07; may not be fTtp", len(entries))
	}
}

// collectPreWord97PapxEntriesWithToggle returns the [startFC, endFC] pairs for all
// paragraphs whose grpprl contains the one-byte toggle [opcode, 0x01].
func collectPreWord97PapxEntriesWithToggle(wordStream []byte, span PreWord97BodySpan, opcode byte) [][2]int {
	var out [][2]int
	if span.PapTableOffset <= 0 || span.PapTableSize < 10 || span.PapTableOffset+span.PapTableSize > len(wordStream) {
		return out
	}
	plc := wordStream[span.PapTableOffset : span.PapTableOffset+span.PapTableSize]
	n := (len(plc) - 4) / 6
	if n <= 0 {
		return out
	}
	cpBytes := (n + 1) * 4
	if cpBytes+n*2 > len(plc) {
		return out
	}
	for index := 0; index < n; index++ {
		pn := int(binary.LittleEndian.Uint16(plc[cpBytes+index*2 : cpBytes+index*2+2]))
		pageOff := pn * 512
		if pageOff < 0 || pageOff+512 > len(wordStream) {
			continue
		}
		page := wordStream[pageOff : pageOff+512]
		cpara := int(page[511])
		if cpara == 0 {
			continue
		}
		rgbxStart := (cpara + 1) * 4
		if rgbxStart+cpara*7 > 511 {
			continue
		}
		for paraIndex := 0; paraIndex < cpara; paraIndex++ {
			startFC := int(binary.LittleEndian.Uint32(page[paraIndex*4 : paraIndex*4+4]))
			endFC := int(binary.LittleEndian.Uint32(page[(paraIndex+1)*4 : (paraIndex+1)*4+4]))
			bOffset := int(page[rgbxStart+paraIndex*7])
			if bOffset == 0 {
				continue
			}
			off := bOffset * 2
			if off >= 511 {
				continue
			}
			cb := int(page[off])
			if cb < 2 {
				continue
			}
			grpOff := off + 3
			grpLen := 2*cb - 3
			if grpLen <= 0 || grpOff+grpLen > 512 {
				continue
			}
			grpprl := page[grpOff : grpOff+grpLen]
			for pi := 0; pi+1 < len(grpprl); pi += 2 {
				if grpprl[pi] == opcode && grpprl[pi+1] == 0x01 {
					out = append(out, [2]int{startFC, endFC})
					break
				}
			}
		}
	}
	return out
}

func TestParsePreWord97PapxGrpprlWithMetadataLegacyInTableToggle(t *testing.T) {
	format, meta, inTableHint, rowMarkHint := parsePreWord97PapxGrpprlWithMetadata([]byte{0x18, 0x01})
	if !inTableHint {
		t.Fatal("legacy in-table hint = false, want true")
	}
	if rowMarkHint {
		t.Fatal("legacy row-mark hint = true, want false")
	}
	if !format.InTable {
		t.Fatal("format.InTable = false, want true")
	}
	if meta.TableDepth != 0 {
		t.Fatalf("meta.TableDepth = %d, want 0", meta.TableDepth)
	}
	if meta.TableRowProps != nil {
		t.Fatalf("meta.TableRowProps = %#v, want nil", meta.TableRowProps)
	}
}

func TestParsePreWord97PapxGrpprlWithMetadataLegacyRowMarkToggle(t *testing.T) {
	format, meta, inTableHint, rowMarkHint := parsePreWord97PapxGrpprlWithMetadata([]byte{0x19, 0x01})
	if inTableHint {
		t.Fatal("legacy in-table hint = true, want false")
	}
	if !rowMarkHint {
		t.Fatal("legacy row-mark hint = false, want true")
	}
	if format.IsTableTrailingParagraph {
		t.Fatal("format.IsTableTrailingParagraph = true, want false for provenance-only mapping")
	}
	if meta.TableDepth != 0 {
		t.Fatalf("meta.TableDepth = %d, want 0", meta.TableDepth)
	}
}

func TestLoadPreWord97PapxEntriesWord95ErrCapturesLegacyRowMarkHints(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	raw, err := os.ReadFile("../../../../testdata/doc/word95_cyrillic.doc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		t.Fatal("InspectPreWord97BodySpan = !ok")
	}
	entries := loadPreWord97PapxEntries(wordStream, span)
	count := 0
	for _, entry := range entries {
		if entry.LegacyRowMarkHint {
			count++
		}
	}
	if count == 0 {
		t.Fatal("legacy row-mark hints = 0, want > 0")
	}
}

func TestExtractPreWord97PapParagraphRunsWord95ErrPropagatesLegacyRowMarkHints(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	raw, err := os.ReadFile("../../../../testdata/doc/word95_cyrillic.doc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		t.Fatal("InspectPreWord97BodySpan = !ok")
	}
	runs := extractPreWord97PapParagraphRuns(wordStream, span)
	count := 0
	for _, run := range runs {
		if run.LegacyRowMarkHint {
			count++
		}
	}
	if count == 0 {
		t.Fatal("legacy row-mark hint runs = 0, want > 0")
	}
}

func TestCollapsePreWord97PapRunsWord95ErrRetainsLegacyRowMarkHints(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	raw, err := os.ReadFile("../../../../testdata/doc/word95_cyrillic.doc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		t.Fatal("InspectPreWord97BodySpan = !ok")
	}
	runs := collapsePreWord97PapRuns(wordStream, span)
	count := 0
	for _, run := range runs {
		if run.LegacyRowMarkHint {
			count++
		}
	}
	if count == 0 {
		t.Fatal("collapsed legacy row-mark hints = 0, want > 0")
	}
}

func collectPreWord97LegacyTogglePairs(wordStream []byte, span PreWord97BodySpan) map[[2]byte]int {
	out := map[[2]byte]int{}
	if span.PapTableOffset <= 0 || span.PapTableSize < 10 || span.PapTableOffset+span.PapTableSize > len(wordStream) {
		return out
	}
	plc := wordStream[span.PapTableOffset : span.PapTableOffset+span.PapTableSize]
	n := (len(plc) - 4) / 6
	if n <= 0 {
		return out
	}
	cpBytes := (n + 1) * 4
	if cpBytes+n*2 > len(plc) {
		return out
	}
	for index := 0; index < n; index++ {
		pn := int(binary.LittleEndian.Uint16(plc[cpBytes+index*2 : cpBytes+index*2+2]))
		pageOff := pn * 512
		if pageOff < 0 || pageOff+512 > len(wordStream) {
			continue
		}
		page := wordStream[pageOff : pageOff+512]
		cpara := int(page[511])
		if cpara == 0 {
			continue
		}
		rgbxStart := (cpara + 1) * 4
		if rgbxStart+cpara*7 > 511 {
			continue
		}
		for paraIndex := 0; paraIndex < cpara; paraIndex++ {
			bOffset := int(page[rgbxStart+paraIndex*7])
			if bOffset == 0 {
				continue
			}
			off := bOffset * 2
			if off >= 511 {
				continue
			}
			cb := int(page[off])
			if cb < 2 {
				continue
			}
			grpOff := off + 3
			grpLen := 2*cb - 3
			if grpLen <= 0 || grpOff+grpLen > 512 {
				continue
			}
			grpprl := page[grpOff : grpOff+grpLen]
			for pairIndex := 0; pairIndex+1 < len(grpprl); pairIndex += 2 {
				pair := [2]byte{grpprl[pairIndex], grpprl[pairIndex+1]}
				out[pair]++
			}
		}
	}
	return out
}

func TestInspectPreWord97SecondaryStyleNamesWord95Fixtures(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path      string
		wantNames []string
	}{
		{path: "../../../../testdata/doc/apache-Word95.doc", wantNames: []string{"Default Paragraph Font", "Heading", "Text body", "Caption", "Index"}},
		{path: "../../../../testdata/doc/apache-Word6.doc", wantNames: []string{"Default Paragraph Font", "Heading", "Text body", "List", "Caption", "Index"}},
	}
	for _, test := range tests {
		raw, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", test.path, err)
		}
		f, err := cfb.OpenBytes(raw)
		if err != nil {
			t.Fatalf("OpenBytes(%s): %v", test.path, err)
		}
		wordStream, ok := f.Stream("WordDocument")
		if !ok {
			t.Fatalf("Stream(%s, WordDocument): missing", test.path)
		}
		names := InspectPreWord97SecondaryStyleNames(wordStream)
		if len(names) == 0 {
			t.Fatalf("InspectPreWord97SecondaryStyleNames(%s) = nil", test.path)
		}
		for _, want := range test.wantNames {
			if !slices.Contains(names, want) {
				t.Fatalf("InspectPreWord97SecondaryStyleNames(%s) = %#v, missing %q", test.path, names, want)
			}
		}
	}
}

func TestPreWord97SecondaryStyleNameByIndexWord95Fixture(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	raw, err := os.ReadFile("../../../../testdata/doc/apache-Word95.doc")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	mapping := preWord97SecondaryStyleNameByIndex(wordStream)
	if len(mapping) == 0 {
		t.Fatal("preWord97SecondaryStyleNameByIndex = nil")
	}
	if got := mapping[0]; got != "Default Paragraph Font" {
		t.Fatalf("mapping[0] = %q, want Default Paragraph Font", got)
	}
	if got := mapping[1]; got != "Heading" {
		t.Fatalf("mapping[1] = %q, want Heading", got)
	}
}

func TestLegacyTableRowPropsConflictTreatsRowScopedNilVsSetAsConflict(t *testing.T) {
	header := true
	if !legacyTableRowPropsConflict(&legacyTableRowProps{}, &legacyTableRowProps{tableHeader: &header}) {
		t.Fatal("legacyTableRowPropsConflict(nil header vs set header) = false, want true")
	}

	rowHeight := 240
	if !legacyTableRowPropsConflict(&legacyTableRowProps{}, &legacyTableRowProps{rowHeight: &rowHeight}) {
		t.Fatal("legacyTableRowPropsConflict(nil rowHeight vs set rowHeight) = false, want true")
	}
}

func TestMergeLegacyTableRowPropsCombinesCompatibleTableLevelSignals(t *testing.T) {
	left := 720
	gap := 30
	merged := mergeLegacyTableRowProps(
		&legacyTableRowProps{tableJc: "center", tableDxaLeft: &left},
		&legacyTableRowProps{tableDxaGap: &gap},
	)
	if merged == nil {
		t.Fatal("mergeLegacyTableRowProps = nil")
	}
	if merged.tableJc != "center" {
		t.Fatalf("tableJc = %q, want center", merged.tableJc)
	}
	if merged.tableDxaLeft == nil || *merged.tableDxaLeft != 720 {
		t.Fatalf("tableDxaLeft = %v, want 720", merged.tableDxaLeft)
	}
	if merged.tableDxaGap == nil || *merged.tableDxaGap != 30 {
		t.Fatalf("tableDxaGap = %v, want 30", merged.tableDxaGap)
	}
}

func TestMergeLegacyTableRowPropsMergesCellFormatsByIndex(t *testing.T) {
	width := 1200
	left := &legacyTableRowProps{}
	left.setCellFormat(0, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	right := &legacyTableRowProps{}
	right.setCellFormat(0, wordir.TableCellFormat{VertAlign: "center"})

	merged := mergeLegacyTableRowProps(left, right)
	if merged == nil {
		t.Fatal("mergeLegacyTableRowProps = nil")
	}
	format := merged.cellFormat(0)
	if format.Width == nil || *format.Width != 1200 {
		t.Fatalf("cell width = %v, want 1200", format.Width)
	}
	if format.WidthType != "dxa" {
		t.Fatalf("cell widthType = %q, want dxa", format.WidthType)
	}
	if format.VertAlign != "center" {
		t.Fatalf("cell valign = %q, want center", format.VertAlign)
	}
}
