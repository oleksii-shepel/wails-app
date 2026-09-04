package legacy

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
)

func legacyStructuredXst(text string) []byte {
	runes := utf16.Encode([]rune(text))
	data := make([]byte, 2+len(runes)*2)
	binary.LittleEndian.PutUint16(data[0:2], uint16(len(runes)))
	for index, value := range runes {
		binary.LittleEndian.PutUint16(data[2+index*2:4+index*2], value)
	}
	return data
}

func legacyStructuredLVLRecord(nfc byte, start int32, flags byte, restartLimit byte, text string) []byte {
	xst := legacyStructuredXst(text)
	data := make([]byte, legacyLvlfSize+len(xst))
	binary.LittleEndian.PutUint32(data[0:4], uint32(start))
	data[4] = nfc
	data[5] = flags
	data[24] = 0
	data[25] = 0
	data[26] = restartLimit
	data[27] = 0
	copy(data[legacyLvlfSize:], xst)
	return data
}

func legacyStructuredPlfLstBlob(lsid uint32, simple bool, levels ...[]byte) []byte {
	data := make([]byte, 2+legacyLstfSize)
	binary.LittleEndian.PutUint16(data[0:2], 1)
	binary.LittleEndian.PutUint32(data[2:6], lsid)
	if simple {
		data[28] = legacyLSTFFlagSimpleList
	}
	for _, level := range levels {
		data = append(data, level...)
	}
	return data
}

func legacyStructuredLFOLVL(level byte, start int32, setStart bool, lvlOverride []byte) []byte {
	data := make([]byte, legacyLfolvlSize)
	binary.LittleEndian.PutUint32(data[0:4], uint32(start))
	flags := uint32(level & 0x0F)
	if setStart {
		flags |= 1 << 4
	}
	if len(lvlOverride) > 0 {
		flags |= 1 << 5
	}
	binary.LittleEndian.PutUint32(data[4:8], flags)
	return append(data, lvlOverride...)
}

func legacyStructuredPlfLfoBlob(lsid uint32, overrides ...[]byte) []byte {
	data := make([]byte, 4+legacyLfoSize+4)
	binary.LittleEndian.PutUint32(data[0:4], 1)
	binary.LittleEndian.PutUint32(data[4:8], lsid)
	data[16] = byte(len(overrides))
	binary.LittleEndian.PutUint32(data[20:24], 0xFFFFFFFF)
	for _, override := range overrides {
		data = append(data, override...)
	}
	return data
}

func TestParseLegacyPlfLfoBlob(t *testing.T) {
	blob := make([]byte, 12)
	binary.LittleEndian.PutUint32(blob[0:4], 2)
	binary.LittleEndian.PutUint32(blob[4:8], 1001)
	binary.LittleEndian.PutUint32(blob[8:12], 2002)

	got := parseLegacyPlfLfoBlob(blob)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[1] != 1001 || got[2] != 2002 {
		t.Fatalf("got = %#v, want ilfo1->1001 ilfo2->2002", got)
	}
}

func TestParseLegacyPlfLfoStartOverrides(t *testing.T) {
	blob := make([]byte, 30)
	binary.LittleEndian.PutUint32(blob[0:4], 2)
	binary.LittleEndian.PutUint32(blob[4:8], 1001)
	binary.LittleEndian.PutUint32(blob[8:12], 2002)
	copy(blob[12:21], []byte{5, 2, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[21:30], []byte{0, 0, 4, 0, 0, 0, 0, 0, 0})

	got := parseLegacyPlfLfoStartOverrides(blob)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[legacyListKey(1, 0)] != 5 {
		t.Fatalf("start override ilfo1 level0 = %d, want 5", got[legacyListKey(1, 0)])
	}
	if got[legacyListKey(1, 1)] != 2 {
		t.Fatalf("start override ilfo1 level1 = %d, want 2", got[legacyListKey(1, 1)])
	}
	if got[legacyListKey(2, 2)] != 4 {
		t.Fatalf("start override ilfo2 level2 = %d, want 4", got[legacyListKey(2, 2)])
	}
}

func TestParseLegacyPlfLfoStartOverridesSupportsClearSentinel(t *testing.T) {
	blob := make([]byte, 17)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 1001)
	copy(blob[8:17], []byte{0xFE, 0xFF, 5, 0, 0, 0, 0, 0, 0})

	got := parseLegacyPlfLfoStartOverrides(blob)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[legacyListKey(1, 0)] != legacyListOverrideClear {
		t.Fatalf("start override ilfo1 level0 = %d, want clear sentinel", got[legacyListKey(1, 0)])
	}
	if got[legacyListKey(1, 2)] != 5 {
		t.Fatalf("start override ilfo1 level2 = %d, want 5", got[legacyListKey(1, 2)])
	}
}

func TestParseLegacyPlfLfoFormatOverrides(t *testing.T) {
	blob := make([]byte, 26)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 1001)
	copy(blob[8:17], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[17:26], []byte{2, 4, 255, 255, 255, 255, 255, 255, 255})

	got := parseLegacyPlfLfoFormatOverrides(blob)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[legacyListKey(1, 0)] != "lowerRoman" {
		t.Fatalf("format override ilfo1 level0 = %q, want lowerRoman", got[legacyListKey(1, 0)])
	}
	if got[legacyListKey(1, 1)] != "lowerLetter" {
		t.Fatalf("format override ilfo1 level1 = %q, want lowerLetter", got[legacyListKey(1, 1)])
	}
}

func TestParseLegacyPlfLfoRestartOverrides(t *testing.T) {
	blob := make([]byte, 35)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 1001)
	copy(blob[8:17], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[17:26], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[26:35], []byte{1, 0, 1, 0, 0, 0, 0, 0, 0})

	got := parseLegacyPlfLfoRestartOverrides(blob)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[legacyListKey(1, 0)] != 1 || got[legacyListKey(1, 2)] != 1 {
		t.Fatalf("restart overrides = %#v, want ilfo1 levels 0 and 2", got)
	}
}

func TestParseLegacyPlfLfoRestartOverridesSupportsClearSentinel(t *testing.T) {
	blob := make([]byte, 35)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 1001)
	copy(blob[8:17], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[17:26], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[26:35], []byte{0xFE, 0xFF, 1, 0, 0, 0, 0, 0, 0})

	got := parseLegacyPlfLfoRestartOverrides(blob)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[legacyListKey(1, 0)] != legacyListOverrideClear {
		t.Fatalf("restart override ilfo1 level0 = %d, want clear sentinel", got[legacyListKey(1, 0)])
	}
	if got[legacyListKey(1, 2)] != 1 {
		t.Fatalf("restart override ilfo1 level2 = %d, want 1", got[legacyListKey(1, 2)])
	}
}

func TestParseLegacyPlfLfoRestartOnLevelOverrides(t *testing.T) {
	blob := make([]byte, 44)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 1001)
	copy(blob[8:17], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[17:26], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[26:35], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[35:44], []byte{0, 255, 1, 255, 255, 255, 255, 255, 255})

	got := parseLegacyPlfLfoRestartOnLevelOverrides(blob)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[legacyListKey(1, 0)] != 0 {
		t.Fatalf("restart target ilfo1 level0 = %d, want 0", got[legacyListKey(1, 0)])
	}
	if got[legacyListKey(1, 2)] != 1 {
		t.Fatalf("restart target ilfo1 level2 = %d, want 1", got[legacyListKey(1, 2)])
	}
}

func TestParseLegacyPlfLfoRestartOnLevelOverridesSupportsClearSentinel(t *testing.T) {
	blob := make([]byte, 44)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 1001)
	copy(blob[8:17], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[17:26], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[26:35], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(blob[35:44], []byte{0xFE, 0xFF, 1, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	got := parseLegacyPlfLfoRestartOnLevelOverrides(blob)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[legacyListKey(1, 0)] != legacyListOverrideClear {
		t.Fatalf("restart target ilfo1 level0 = %d, want clear sentinel", got[legacyListKey(1, 0)])
	}
	if got[legacyListKey(1, 2)] != 1 {
		t.Fatalf("restart target ilfo1 level2 = %d, want 1", got[legacyListKey(1, 2)])
	}
}

func TestParseLegacyPlfLstBlob(t *testing.T) {
	blob := make([]byte, 17)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 3003)
	copy(blob[8:17], []byte{4, 2, 0, 0, 0, 0, 0, 0, 0})

	got := parseLegacyPlfLstBlob(blob)
	levels, ok := got[3003]
	if !ok {
		t.Fatalf("missing lsid 3003 in %#v", got)
	}
	if len(levels.format) != 9 {
		t.Fatalf("len(levels.format) = %d, want 9", len(levels.format))
	}
	if levels.format[0] != 4 || levels.format[1] != 2 || levels.format[2] != 0 {
		t.Fatalf("levels prefix = %v, want [4 2 0]", levels.format[:3])
	}
	if len(levels.start) != 9 {
		t.Fatalf("len(levels.start) = %d, want 9", len(levels.start))
	}
}

func TestParseLegacyPlfLstBlobWithStarts(t *testing.T) {
	blob := make([]byte, 26)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 5005)
	copy(blob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(blob[17:26], []byte{3, 1, 1, 1, 1, 1, 1, 1, 1})

	got := parseLegacyPlfLstBlob(blob)
	levels, ok := got[5005]
	if !ok {
		t.Fatalf("missing lsid 5005 in %#v", got)
	}
	if levels.start[0] != 3 {
		t.Fatalf("levels.start[0] = %d, want 3", levels.start[0])
	}
	if levels.start[1] != 1 {
		t.Fatalf("levels.start[1] = %d, want 1", levels.start[1])
	}
}

func TestParseLegacyPlfLstBlobWithStartsAndRestarts(t *testing.T) {
	blob := make([]byte, 35)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 6006)
	copy(blob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(blob[17:26], []byte{1, 1, 1, 1, 1, 1, 1, 1, 1})
	copy(blob[26:35], []byte{0, 1, 0, 0, 0, 0, 0, 0, 0})

	got := parseLegacyPlfLstBlob(blob)
	levels, ok := got[6006]
	if !ok {
		t.Fatalf("missing lsid 6006 in %#v", got)
	}
	if len(levels.restart) != 9 {
		t.Fatalf("len(levels.restart) = %d, want 9", len(levels.restart))
	}
	if levels.restart[0] {
		t.Fatal("levels.restart[0] = true, want false")
	}
	if !levels.restart[1] {
		t.Fatal("levels.restart[1] = false, want true")
	}
}

func TestParseLegacyPlfLstBlobWithRestartTargets(t *testing.T) {
	blob := make([]byte, 44)
	binary.LittleEndian.PutUint32(blob[0:4], 1)
	binary.LittleEndian.PutUint32(blob[4:8], 7007)
	copy(blob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(blob[17:26], []byte{1, 1, 1, 1, 1, 1, 1, 1, 1})
	copy(blob[26:35], []byte{0, 1, 1, 0, 0, 0, 0, 0, 0})
	copy(blob[35:44], []byte{255, 0, 1, 255, 255, 255, 255, 255, 255})

	got := parseLegacyPlfLstBlob(blob)
	levels, ok := got[7007]
	if !ok {
		t.Fatalf("missing lsid 7007 in %#v", got)
	}
	if levels.restartOnLevel[0] != -1 {
		t.Fatalf("levels.restartOnLevel[0] = %d, want -1", levels.restartOnLevel[0])
	}
	if levels.restartOnLevel[1] != 0 {
		t.Fatalf("levels.restartOnLevel[1] = %d, want 0", levels.restartOnLevel[1])
	}
	if levels.restartOnLevel[2] != 1 {
		t.Fatalf("levels.restartOnLevel[2] = %d, want 1", levels.restartOnLevel[2])
	}
}

func TestParseLegacyPlfLstBlobParsesStructuredRecords(t *testing.T) {
	blob := legacyStructuredPlfLstBlob(
		808,
		false,
		legacyStructuredLVLRecord(0, 3, 0, 0, "%1."),
		legacyStructuredLVLRecord(2, 1, 0, 0, "%2."),
		legacyStructuredLVLRecord(4, 1, legacyLVLFFlagNoRestart, 1, "%3."),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
	)

	got := parseLegacyPlfLstBlob(blob)
	levels, ok := got[808]
	if !ok {
		t.Fatalf("missing lsid 808 in %#v", got)
	}
	if got := legacyNfcToListFormat(levels.format[0]); got != "decimal" {
		t.Fatalf("format[0] = %q, want decimal", got)
	}
	if got := levels.start[0]; got != 3 {
		t.Fatalf("start[0] = %d, want 3", got)
	}
	if got := legacyNfcToListFormat(levels.format[1]); got != "lowerRoman" {
		t.Fatalf("format[1] = %q, want lowerRoman", got)
	}
	if !levels.restart[1] {
		t.Fatal("restart[1] = false, want default restart from structured LVLF")
	}
	if got := levels.restartOnLevel[2]; got != 0 {
		t.Fatalf("restartOnLevel[2] = %d, want 0 from ilvlRestartLim=1", got)
	}
}

func TestLoadLegacyListTablesParsesStructuredFibListEntries(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := legacyStructuredPlfLfoBlob(909)
	lstBlob := legacyStructuredPlfLstBlob(
		909,
		false,
		legacyStructuredLVLRecord(0, 4, 0, 0, "%1."),
		legacyStructuredLVLRecord(2, 1, 0, 0, "%2."),
		legacyStructuredLVLRecord(4, 1, legacyLVLFFlagNoRestart, 1, "%3."),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
	)

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), 2+legacyLstfSize)

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded structured list tables")
	}
	if got := tables.startFor(1, 0); got != 4 {
		t.Fatalf("startFor(1,0) = %d, want 4 from appended structured LVL", got)
	}
	if got := tables.formatFor(1, 1); got != "lowerRoman" {
		t.Fatalf("formatFor(1,1) = %q, want lowerRoman", got)
	}
	if got, ok := tables.restartOnLevelFor(1, 2); !ok || got != 0 {
		t.Fatalf("restartOnLevelFor(1,2) = (%d,%v), want (0,true)", got, ok)
	}
	if tables.restartFor(1, 2) {
		t.Fatal("restartFor(1,2) = true, want threshold-based structured restart only")
	}
}

func TestLoadLegacyListTablesDoesNotTrustScannedStartMetadata(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := legacyStructuredPlfLfoBlob(4242)
	lstBlob := legacyStructuredPlfLstBlob(
		4242,
		false,
		legacyStructuredLVLRecord(0, 5, 0, 0, "%1."),
		legacyStructuredLVLRecord(4, 1, 0, 0, "%2."),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
	)

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)
	putFibFcLcb(wordStream, fibBase, 30, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, 31, len(lfoBlob), len(lstBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want scanned list formats")
	}
	if got := tables.formatFor(1, 0); got != "decimal" {
		t.Fatalf("formatFor(1,0) = %q, want decimal from scanned format data", got)
	}
	if got := tables.startFor(1, 0); got != 0 {
		t.Fatalf("startFor(1,0) = %d, want scanned start metadata ignored", got)
	}
	if tables.restartFor(1, 0) {
		t.Fatal("restartFor(1,0) = true, want scanned restart metadata ignored")
	}
	if _, ok := tables.restartOnLevelFor(1, 0); ok {
		t.Fatal("restartOnLevelFor(1,0) present, want scanned restart threshold ignored")
	}
}

func TestLoadLegacyListTablesParsesStructuredLfoOverrides(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	overrideLvl := legacyStructuredLVLRecord(2, 5, 0, 0, "%1.")
	lfoBlob := legacyStructuredPlfLfoBlob(707, legacyStructuredLFOLVL(0, 0, false, overrideLvl))
	lstBlob := legacyStructuredPlfLstBlob(707, false,
		legacyStructuredLVLRecord(0, 1, 0, 0, "%1."),
		legacyStructuredLVLRecord(4, 1, 0, 0, "%2."),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "•"),
	)

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), 2+legacyLstfSize)

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want structured tables with override")
	}
	if got := tables.formatFor(1, 0); got != "lowerRoman" {
		t.Fatalf("formatFor(1,0) = %q, want lowerRoman from structured LFOLVL override", got)
	}
	if got := tables.startFor(1, 0); got != 5 {
		t.Fatalf("startFor(1,0) = %d, want 5 from structured LFOLVL override", got)
	}
	if !tables.restartFor(1, 0) {
		t.Fatal("restartFor(1,0) = false, want structured override LVL restart semantics")
	}
	if _, ok := tables.restartOnLevelFor(1, 0); ok {
		t.Fatal("restartOnLevelFor(1,0) present, want cleared threshold from structured override LVL")
	}
}

func TestLoadLegacyListTablesResolvesFormatByIlfoLevel(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 8)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 77)

	lstBlob := make([]byte, 17)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 77)
	copy(lstBlob[8:17], []byte{4, 2, 0, 0, 0, 0, 0, 0, 0})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)

	putFibFcLcb(wordStream, fibBase, 20, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, 21, len(lfoBlob), len(lstBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded list tables")
	}
	if got := tables.formatFor(1, 0); got != "lowerLetter" {
		t.Fatalf("formatFor(1,0) = %q, want lowerLetter", got)
	}
	if got := tables.formatFor(1, 1); got != "lowerRoman" {
		t.Fatalf("formatFor(1,1) = %q, want lowerRoman", got)
	}
	if got := tables.formatFor(1, 2); got != "decimal" {
		t.Fatalf("formatFor(1,2) = %q, want decimal", got)
	}
	if got := tables.startFor(1, 0); got != 0 {
		t.Fatalf("startFor(1,0) = %d, want 0 for no explicit starts", got)
	}
}

func TestLoadLegacyListTablesResolvesStartByIlfoLevel(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 8)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 88)

	lstBlob := make([]byte, 26)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 88)
	copy(lstBlob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[17:26], []byte{3, 2, 1, 1, 1, 1, 1, 1, 1})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)

	putFibFcLcb(wordStream, fibBase, 20, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, 21, len(lfoBlob), len(lstBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded list tables")
	}
	if got := tables.startFor(1, 0); got != 3 {
		t.Fatalf("startFor(1,0) = %d, want 3", got)
	}
	if got := tables.startFor(1, 1); got != 2 {
		t.Fatalf("startFor(1,1) = %d, want 2", got)
	}
}

func TestLoadLegacyListTablesPrefersLfoStartOverrides(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 17)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 123)
	copy(lfoBlob[8:17], []byte{7, 0, 0, 0, 0, 0, 0, 0, 0})

	lstBlob := make([]byte, 26)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 123)
	copy(lstBlob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[17:26], []byte{3, 2, 1, 1, 1, 1, 1, 1, 1})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)

	putFibFcLcb(wordStream, fibBase, 20, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, 21, len(lfoBlob), len(lstBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded list tables")
	}
	if got := tables.startFor(1, 0); got != 7 {
		t.Fatalf("startFor(1,0) = %d, want LFO override 7", got)
	}
	if got := tables.startFor(1, 1); got != 2 {
		t.Fatalf("startFor(1,1) = %d, want LST fallback 2", got)
	}
}

func TestLoadLegacyListTablesPrefersLfoFormatAndRestartOverrides(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 44)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 222)
	copy(lfoBlob[8:17], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(lfoBlob[17:26], []byte{2, 255, 255, 255, 255, 255, 255, 255, 255})
	copy(lfoBlob[26:35], []byte{1, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(lfoBlob[35:44], []byte{0, 255, 255, 255, 255, 255, 255, 255, 255})

	lstBlob := make([]byte, 44)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 222)
	copy(lstBlob[8:17], []byte{4, 2, 0, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[17:26], []byte{1, 1, 1, 1, 1, 1, 1, 1, 1})
	copy(lstBlob[26:35], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[35:44], []byte{255, 255, 255, 255, 255, 255, 255, 255, 255})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)

	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), len(lstBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded list tables")
	}
	if got := tables.formatFor(1, 0); got != "lowerRoman" {
		t.Fatalf("formatFor(1,0) = %q, want LFO override lowerRoman", got)
	}
	if !tables.restartFor(1, 0) {
		t.Fatal("restartFor(1,0) = false, want true from LFO override")
	}
	if got, ok := tables.restartOnLevelFor(1, 0); !ok || got != 0 {
		t.Fatalf("restartOnLevelFor(1,0) = (%d,%v), want (0,true)", got, ok)
	}
	if got := tables.formatFor(1, 1); got != "lowerRoman" {
		t.Fatalf("formatFor(1,1) = %q, want LST fallback lowerRoman", got)
	}
	if tables.restartFor(1, 1) {
		t.Fatal("restartFor(1,1) = true, want false")
	}
	if _, ok := tables.restartOnLevelFor(1, 1); ok {
		t.Fatal("restartOnLevelFor(1,1) present, want absent")
	}
}

func TestLoadLegacyListTablesAllowsLfoClearOverrides(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 44)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 515)
	copy(lfoBlob[8:17], []byte{0xFE, 7, 0, 0, 0, 0, 0, 0, 0})
	copy(lfoBlob[17:26], []byte{255, 255, 255, 255, 255, 255, 255, 255, 255})
	copy(lfoBlob[26:35], []byte{0xFE, 1, 0, 0, 0, 0, 0, 0, 0})
	copy(lfoBlob[35:44], []byte{0xFE, 1, 255, 255, 255, 255, 255, 255, 255})

	lstBlob := make([]byte, 44)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 515)
	copy(lstBlob[8:17], []byte{4, 2, 0, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[17:26], []byte{3, 2, 1, 1, 1, 1, 1, 1, 1})
	copy(lstBlob[26:35], []byte{1, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[35:44], []byte{0, 255, 255, 255, 255, 255, 255, 255, 255})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)

	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), len(lstBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded list tables")
	}
	if got := tables.startFor(1, 0); got != 0 {
		t.Fatalf("startFor(1,0) = %d, want cleared start fallback", got)
	}
	if got := tables.startFor(1, 1); got != 7 {
		t.Fatalf("startFor(1,1) = %d, want explicit LFO start 7", got)
	}
	if tables.restartFor(1, 0) {
		t.Fatal("restartFor(1,0) = true, want cleared restart")
	}
	if !tables.restartFor(1, 1) {
		t.Fatal("restartFor(1,1) = false, want explicit LFO restart")
	}
	if _, ok := tables.restartOnLevelFor(1, 0); ok {
		t.Fatal("restartOnLevelFor(1,0) present, want cleared target")
	}
	if got, ok := tables.restartOnLevelFor(1, 1); !ok || got != 1 {
		t.Fatalf("restartOnLevelFor(1,1) = (%d,%v), want (1,true)", got, ok)
	}
}

func TestLoadLegacyListTablesPrefersKnownFibListEntries(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	exactLfoBlob := make([]byte, 17)
	binary.LittleEndian.PutUint32(exactLfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(exactLfoBlob[4:8], 333)
	copy(exactLfoBlob[8:17], []byte{3, 0, 0, 0, 0, 0, 0, 0, 0})

	exactLstBlob := make([]byte, 26)
	binary.LittleEndian.PutUint32(exactLstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(exactLstBlob[4:8], 333)
	copy(exactLstBlob[8:17], []byte{4, 2, 0, 0, 0, 0, 0, 0, 0})
	copy(exactLstBlob[17:26], []byte{1, 1, 1, 1, 1, 1, 1, 1, 1})

	bogusBlob := make([]byte, 44)
	binary.LittleEndian.PutUint32(bogusBlob[0:4], 1)
	binary.LittleEndian.PutUint32(bogusBlob[4:8], 444)
	copy(bogusBlob[8:17], []byte{9, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(bogusBlob[17:26], []byte{2, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(bogusBlob[26:35], []byte{1, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(bogusBlob[35:44], []byte{0, 255, 255, 255, 255, 255, 255, 255, 255})

	off := 0
	tableStream := make([]byte, len(exactLfoBlob)+len(exactLstBlob)+len(bogusBlob))
	copy(tableStream[off:], exactLfoBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, off, len(exactLfoBlob))
	off += len(exactLfoBlob)
	copy(tableStream[off:], exactLstBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, off, len(exactLstBlob))
	off += len(exactLstBlob)
	copy(tableStream[off:], bogusBlob)
	putFibFcLcb(wordStream, fibBase, 30, off, len(bogusBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded list tables")
	}
	if got := tables.startFor(1, 0); got != 3 {
		t.Fatalf("startFor(1,0) = %d, want exact PlfLfo start 3", got)
	}
	if got := tables.formatFor(1, 0); got != "lowerLetter" {
		t.Fatalf("formatFor(1,0) = %q, want exact PlfLst format lowerLetter", got)
	}
}

func TestLoadLegacyListTablesAnchorsStructuredListDataWithPositiveLcbSlots(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := legacyStructuredPlfLfoBlob(606, legacyStructuredLFOLVL(0, 5, true, nil))
	lstBlob := legacyStructuredPlfLstBlob(606, true, legacyStructuredLVLRecord(3, 1, 0, 0, "%1."))

	lfoAnchor := 12
	lfoBlobOffset := lfoAnchor + 4
	lstAnchor := lfoBlobOffset + len(lfoBlob) + 9
	lstBlobOffset := lstAnchor + 6
	tableStream := make([]byte, lstBlobOffset+len(lstBlob)+8)
	copy(tableStream[lfoBlobOffset:], lfoBlob)
	copy(tableStream[lstBlobOffset:], lstBlob)

	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, lfoAnchor, 4)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, lstAnchor, 6)

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want anchored structured list recovery")
	}
	if got := tables.startFor(1, 0); got != 5 {
		t.Fatalf("startFor(1,0) = %d, want anchored LFO start 5", got)
	}
	if got := tables.formatFor(1, 0); got != "upperLetter" {
		t.Fatalf("formatFor(1,0) = %q, want anchored LST format upperLetter", got)
	}
}

func TestLoadLegacyListTablesAnchorsStructuredListDataWithMixedAnchorLengths(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := legacyStructuredPlfLfoBlob(707, legacyStructuredLFOLVL(0, 9, true, nil))
	lstBlob := legacyStructuredPlfLstBlob(707, true, legacyStructuredLVLRecord(3, 1, 0, 0, "%1."))

	lfoAnchor := 18
	lfoBlobOffset := lfoAnchor + 5
	lstBlobOffset := lfoBlobOffset + len(lfoBlob) + 11
	tableStream := make([]byte, lstBlobOffset+len(lstBlob)+8)
	copy(tableStream[lfoBlobOffset:], lfoBlob)
	copy(tableStream[lstBlobOffset:], lstBlob)

	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, lfoAnchor, 5)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, lstBlobOffset, 0)

	anchoredLfo, anchoredLst := legacyAnchoredListPair(wordStream, tableStream, fibBase, cbRgFcLcb)
	if anchoredLfo == nil || anchoredLst == nil {
		t.Fatal("anchored candidates = nil, want mixed-length anchors to recover both structured blobs")
	}
	if got := anchoredLfo.startByIlfoLevel[legacyListKey(1, 0)]; got != 9 {
		t.Fatalf("anchored LFO start = %d, want 9", got)
	}
	if got := legacyNfcToListFormat(anchoredLst.levels[707].format[0]); got != "upperLetter" {
		t.Fatalf("anchored LST format = %q, want upperLetter", got)
	}
}

func TestLoadLegacyListTablesResolvesRestartByIlfoLevel(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 8)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 99)

	lstBlob := make([]byte, 35)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 99)
	copy(lstBlob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[17:26], []byte{1, 1, 1, 1, 1, 1, 1, 1, 1})
	copy(lstBlob[26:35], []byte{0, 1, 0, 0, 0, 0, 0, 0, 0})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)

	putFibFcLcb(wordStream, fibBase, 20, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, 21, len(lfoBlob), len(lstBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded list tables")
	}
	if tables.restartFor(1, 0) {
		t.Fatal("restartFor(1,0) = true, want false")
	}
	if !tables.restartFor(1, 1) {
		t.Fatal("restartFor(1,1) = false, want true")
	}
}

func TestLoadLegacyListTablesResolvesRestartOnLevelByIlfoLevel(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 8)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 111)

	lstBlob := make([]byte, 44)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 111)
	copy(lstBlob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[17:26], []byte{1, 1, 1, 1, 1, 1, 1, 1, 1})
	copy(lstBlob[26:35], []byte{0, 1, 1, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[35:44], []byte{255, 0, 1, 255, 255, 255, 255, 255, 255})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)

	putFibFcLcb(wordStream, fibBase, 20, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, 21, len(lfoBlob), len(lstBlob))

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if tables == nil {
		t.Fatal("tables = nil, want decoded list tables")
	}
	if _, ok := tables.restartOnLevelFor(1, 0); ok {
		t.Fatal("restartOnLevelFor(1,0) present, want absent")
	}
	if got, ok := tables.restartOnLevelFor(1, 1); !ok || got != 0 {
		t.Fatalf("restartOnLevelFor(1,1) = (%d,%v), want (0,true)", got, ok)
	}
	if got, ok := tables.restartOnLevelFor(1, 2); !ok || got != 1 {
		t.Fatalf("restartOnLevelFor(1,2) = (%d,%v), want (1,true)", got, ok)
	}
}

func TestParseLVLFPapxIndentsParsesLeftAndFirstLineIndents(t *testing.T) {
	// Build a minimal PAPx grpprl containing sprmPDxaLeft (0x840E) and
	// sprmPDxaLeft1 (0x8411).  Both SPRMs have a 2-byte operand (type 3).
	grpprl := make([]byte, 8)
	binary.LittleEndian.PutUint16(grpprl[0:2], 0x840E) // sprmPDxaLeft
	binary.LittleEndian.PutUint16(grpprl[2:4], 720)    // 720 twips
	binary.LittleEndian.PutUint16(grpprl[4:6], 0x8411) // sprmPDxaLeft1
	var hangingIndent int16 = -360
	binary.LittleEndian.PutUint16(grpprl[6:8], uint16(hangingIndent)) // -360 twips (hanging)

	gotLeft, gotFirst := parseLVLFPapxIndents(grpprl)
	if gotLeft != 720 {
		t.Fatalf("indentLeft = %d, want 720", gotLeft)
	}
	if gotFirst != -360 {
		t.Fatalf("indentFirstLine = %d, want -360", gotFirst)
	}
}

func TestParseLVLFPapxIndentsIgnoresUnrelatedSPRMs(t *testing.T) {
	// Only sprmPDxaLeft1 present; sprmPDxaLeft should remain 0.
	grpprl := make([]byte, 4)
	binary.LittleEndian.PutUint16(grpprl[0:2], 0x8411)
	var firstLine int16 = -360
	binary.LittleEndian.PutUint16(grpprl[2:4], uint16(firstLine))

	gotLeft, gotFirst := parseLVLFPapxIndents(grpprl)
	if gotLeft != 0 {
		t.Fatalf("indentLeft = %d, want 0 (not present)", gotLeft)
	}
	if gotFirst != -360 {
		t.Fatalf("indentFirstLine = %d, want -360", gotFirst)
	}
}

func TestParseLVLFPapxIndentsHandlesEmptyGrpprl(t *testing.T) {
	gotLeft, gotFirst := parseLVLFPapxIndents(nil)
	if gotLeft != 0 || gotFirst != 0 {
		t.Fatalf("parseLVLFPapxIndents(nil) = (%d, %d), want (0, 0)", gotLeft, gotFirst)
	}
}

func TestLegacyReconcileObservedOrderedListMappingsReordersOrderedProfiles(t *testing.T) {
	bestLfo := &legacyAnchoredLfoCandidate{
		lsidByIlfo: map[int]uint32{
			1: 200,
			2: 100,
			3: 300,
		},
		exact: true,
	}
	bestLst := &legacyAnchoredLstCandidate{
		levels: map[uint32]legacyListLevelData{
			100: {format: []byte{0, 4, 2}},
			200: {format: []byte{0}},
			300: {format: []byte{23, 23, 23}},
		},
	}
	observedLevels := map[int]map[int]struct{}{
		1: {0: {}, 1: {}, 2: {}},
		2: {0: {}},
		3: {0: {}, 1: {}},
	}

	legacyReconcileObservedOrderedListMappings(bestLfo, bestLst, observedLevels)

	if got := bestLfo.lsidByIlfo[1]; got != 100 {
		t.Fatalf("lsidByIlfo[1] = %d, want richer ordered profile 100", got)
	}
	if got := bestLfo.lsidByIlfo[2]; got != 200 {
		t.Fatalf("lsidByIlfo[2] = %d, want simpler ordered profile 200", got)
	}
	if got := bestLfo.lsidByIlfo[3]; got != 300 {
		t.Fatalf("lsidByIlfo[3] = %d, want bullet profile unchanged", got)
	}
}

func TestLegacyReconcileObservedOrderedListMappingsSkipsNonExactCandidates(t *testing.T) {
	bestLfo := &legacyAnchoredLfoCandidate{
		lsidByIlfo: map[int]uint32{
			1: 200,
			2: 100,
		},
		exact: false,
	}
	bestLst := &legacyAnchoredLstCandidate{
		levels: map[uint32]legacyListLevelData{
			100: {format: []byte{0, 4, 2}},
			200: {format: []byte{0}},
		},
	}
	observedLevels := map[int]map[int]struct{}{
		1: {0: {}, 1: {}, 2: {}},
		2: {0: {}},
	}

	legacyReconcileObservedOrderedListMappings(bestLfo, bestLst, observedLevels)

	if got := bestLfo.lsidByIlfo[1]; got != 200 {
		t.Fatalf("lsidByIlfo[1] = %d, want unchanged for non-exact candidate", got)
	}
	if got := bestLfo.lsidByIlfo[2]; got != 100 {
		t.Fatalf("lsidByIlfo[2] = %d, want unchanged for non-exact candidate", got)
	}
}

func TestLoadLegacyListTablesReconcilesObservedOrderedProfiles(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 12)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 2)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 200)
	binary.LittleEndian.PutUint32(lfoBlob[8:12], 100)

	lstBlob := make([]byte, 30)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 2)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 100)
	copy(lstBlob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	binary.LittleEndian.PutUint32(lstBlob[17:21], 200)
	copy(lstBlob[21:30], []byte{0, 0, 0, 0, 0, 0, 0, 0, 0})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), len(lstBlob))

	ilfo1, ilfo2 := 1, 2
	level0, level1, level2 := 0, 1, 2
	papxEntries := []papxEntry{
		{Format: wordir.ParagraphFormat{BinaryListOverride: &ilfo1, BinaryListLevel: &level0}},
		{Format: wordir.ParagraphFormat{BinaryListOverride: &ilfo1, BinaryListLevel: &level1}},
		{Format: wordir.ParagraphFormat{BinaryListOverride: &ilfo1, BinaryListLevel: &level2}},
		{Format: wordir.ParagraphFormat{BinaryListOverride: &ilfo2, BinaryListLevel: &level0}},
	}

	tables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb, papxEntries)
	if tables == nil {
		t.Fatal("tables = nil, want reconciled exact list tables")
	}
	if got := tables.formatFor(1, 1); got != "lowerLetter" {
		t.Fatalf("formatFor(1,1) = %q, want richer observed profile mapped to ilfo 1", got)
	}
	if got := tables.formatFor(1, 2); got != "lowerRoman" {
		t.Fatalf("formatFor(1,2) = %q, want richer observed profile mapped to ilfo 1", got)
	}
	if got := tables.formatFor(2, 1); got != "decimal" {
		t.Fatalf("formatFor(2,1) = %q, want simpler ordered profile mapped to ilfo 2", got)
	}
}

// legacyStructuredLVLRecordWithChpx builds an LVLF record that includes a CHPx
// grpprl after the 28-byte header. The chpx bytes are passed verbatim.
func legacyStructuredLVLRecordWithChpx(nfc byte, start int32, flags byte, restartLimit byte, text string, chpx []byte) []byte {
	xst := legacyStructuredXst(text)
	data := make([]byte, legacyLvlfSize+len(chpx)+len(xst))
	binary.LittleEndian.PutUint32(data[0:4], uint32(start))
	data[4] = nfc
	data[5] = flags
	data[24] = byte(len(chpx)) // cbGrpprlChpx
	data[25] = 0               // cbGrpprlPapx
	data[26] = restartLimit
	data[27] = 0
	copy(data[legacyLvlfSize:], chpx)
	copy(data[legacyLvlfSize+len(chpx):], xst)
	return data
}

func TestParseLegacyStructuredLVLWithChpx(t *testing.T) {
	// sprmCFBold (0x0835) = 1 byte operand, value 1 => bold on
	chpx := []byte{0x35, 0x08, 0x01}
	lvl := legacyStructuredLVLRecordWithChpx(0x17, 1, 0, 0, "\u2022", chpx)
	info, _, ok := parseLegacyStructuredLVL(lvl)
	if !ok {
		t.Fatal("parseLegacyStructuredLVL failed")
	}
	if !info.hasRunFormat {
		t.Fatal("expected hasRunFormat=true")
	}
	if info.runFormat.Bold == nil || !*info.runFormat.Bold {
		t.Fatal("expected RunFormat.Bold=true from CHPx grpprl")
	}
}

func TestLoadLegacyListTablesExtractsRunFormat(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	// sprmCFBold (0x0835) = 1 byte operand, value 1 => bold on
	chpx := []byte{0x35, 0x08, 0x01}
	lvl := legacyStructuredLVLRecordWithChpx(0x17, 1, 0, 0, "\u2022", chpx)
	lstBlob := legacyStructuredPlfLstBlob(888, true, lvl)
	lfoBlob := legacyStructuredPlfLfoBlob(888, legacyStructuredLFOLVL(0, 0, false, nil))

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), len(lstBlob))

	listTables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if listTables == nil {
		t.Fatal("listTables = nil")
	}
	rf := listTables.runFormatFor(1, 0)
	if rf == nil {
		t.Fatal("expected runFormat for (1,0), got nil")
	}
	if rf.Bold == nil || !*rf.Bold {
		t.Fatalf("expected Bold=true, got %+v", rf.Bold)
	}
}

func TestLegacyOrderedListProfileScorePrefersCompoundTemplates(t *testing.T) {
	compound := legacyListLevelData{
		format:   []byte{0, 0, 0, 0},
		template: []string{"%1.", "%1.%2.", "%1.%2.%3.", "%1.%2.%3.%4."},
	}
	styleCycle := legacyListLevelData{
		format:   []byte{0, 4, 2, 0},
		template: []string{"%1.", "%1.", "%1.", "%1."},
	}
	if got, other := legacyOrderedListProfileScore(compound), legacyOrderedListProfileScore(styleCycle); got <= other {
		t.Fatalf("compound score %d <= style-cycle score %d", got, other)
	}
}

func TestLegacyListLiteralTemplateWinsOverStaleNumericOverride(t *testing.T) {
	for _, template := range []string{"•", "\uf0b7", "◦"} {
		if got := legacyListFormatForTemplate("decimal", template); got != "bullet" {
			t.Fatalf("legacyListFormatForTemplate(decimal, %q) = %q, want bullet", template, got)
		}
	}
	if got := legacyListFormatForTemplate("decimal", "%1."); got != "decimal" {
		t.Fatalf("placeholder template format = %q, want decimal", got)
	}
}
