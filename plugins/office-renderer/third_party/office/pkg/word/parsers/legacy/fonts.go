package legacy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	word97 "office/pkg/word/parsers/doc/word97"
)

// legacyFontEntry pairs an FFN-derived display name with its OOXML-flavored
// metadata (family/pitch/charset/PANOSE) so callers can build wordir.Document
// FontFamilies maps without re-parsing the SttbfFfn blob.
type legacyFontEntry struct {
	Name string
	Info wordir.FontFamilyInfo
}

// Fixture builders still use the historical legacy name, but the descriptor
// shape itself is owned by the canonical Word97 reader.
type legacyEmbeddedFontDescriptor = word97.EmbeddedFontDescriptor

// loadLegacyFontNames resolves the legacy DOC font table into a font-indexed
// slice of names. Word 97 stores font names in SttbfFfn; when the nominal FIB
// slot is unavailable or malformed, fall back to scanning table-stream entries
// for a plausible FFN sequence.
func loadLegacyFontNames(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []string {
	entries := loadLegacyFontEntries(wordStream, tableStream, fibBase, cbRgFcLcb)
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, len(entries))
	for i := range entries {
		names[i] = entries[i].Name
	}
	return names
}

// loadLegacyFontEntries resolves SttbfFfn into ordered (name, FontFamilyInfo)
// pairs. Returns nil when no plausible FFN sequence can be located.
func loadLegacyFontEntries(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []legacyFontEntry {
	const fontFibIndex = 15
	tryEntry := func(index int) []legacyFontEntry {
		fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
		if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
			return nil
		}
		return parseLegacyFontTableBlobEntries(tableStream[fc : fc+lcb])
	}

	best := tryEntry(fontFibIndex)
	bestScore := scoreLegacyFontEntries(best)
	for index := 0; index < cbRgFcLcb; index++ {
		if index == fontFibIndex {
			continue
		}
		entries := tryEntry(index)
		if score := scoreLegacyFontEntries(entries); score > bestScore {
			best = entries
			bestScore = score
		}
	}
	if bestScore == 0 {
		return nil
	}
	return best
}

// loadLegacyDocEmbeddedFonts is retained only as a fixture/test adapter. The
// descriptor parsing, payload slicing and EOT detection are all owned by the
// canonical Word97 reader.
func loadLegacyDocEmbeddedFonts(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []*wordir.EmbeddedFont {
	layout := docbinary.Word97Layout{FibBase: fibBase, CbRgFcLcb: cbRgFcLcb}
	return word97.LoadEmbeddedFontsReaderAt(
		bytes.NewReader(wordStream), int64(len(wordStream)),
		bytes.NewReader(tableStream), int64(len(tableStream)),
		wordStream, layout,
	)
}

func parseLegacyFontTableBlob(blob []byte) []string {
	entries := parseLegacyFontTableBlobEntries(blob)
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, len(entries))
	for i := range entries {
		names[i] = entries[i].Name
	}
	return names
}

func parseLegacyFontTableBlobEntries(blob []byte) []legacyFontEntry {
	for _, start := range []int{0, 2, 4, 6} {
		if start >= len(blob) {
			continue
		}
		if entries := parseLegacyFFNRecordEntries(blob[start:]); scoreLegacyFontEntries(entries) > 0 {
			return entries
		}
	}
	return nil
}

func parseLegacyFFNRecords(blob []byte) []string {
	entries := parseLegacyFFNRecordEntries(blob)
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, len(entries))
	for i := range entries {
		names[i] = entries[i].Name
	}
	return names
}

func parseLegacyFFNRecordEntries(blob []byte) []legacyFontEntry {
	const ffnFixedSize = 40
	var entries []legacyFontEntry
	for pos := 0; pos < len(blob); {
		if pos+1 > len(blob) {
			break
		}
		recSize := int(blob[pos]) + 1
		if recSize <= ffnFixedSize || pos+recSize > len(blob) {
			break
		}
		rec := blob[pos : pos+recSize]
		if name := parseLegacyFFNName(rec); name != "" {
			entries = append(entries, legacyFontEntry{Name: name, Info: parseLegacyFFNInfo(rec)})
		}
		pos += recSize
	}
	return entries
}

func parseLegacyFFNName(rec []byte) string {
	if len(rec) <= 40 {
		return ""
	}
	nameData := rec[40:]
	if name := sanitizeLegacyFontName(parseLegacyFFNXszUTF16(nameData)); name != "" {
		return name
	}
	return sanitizeLegacyFontName(parseLegacyFFNXszSingleByte(nameData))
}

// parseLegacyFFNInfo reads the bit-packed FFN header (MS-DOC §2.9.74) and
// returns the OOXML-flavored hints that mirror w:font's family/pitch/charset/
// w:panose1 children. rec[0] is cbFfnM1; the bit-packed flags live in rec[1],
// chs in rec[4], and the 10-byte PANOSE blob starts at rec[6].
func parseLegacyFFNInfo(rec []byte) wordir.FontFamilyInfo {
	if len(rec) < 16 {
		return wordir.FontFamilyInfo{}
	}
	flags := rec[1]
	prq := flags & 0x03
	ff := (flags >> 4) & 0x07
	chs := rec[4]
	info := wordir.FontFamilyInfo{
		Family:  ffnFamilyHint(ff),
		Pitch:   ffnPitchHint(prq),
		Charset: ffnCharsetHint(chs),
		Panose1: ffnPanoseHex(rec[6:16]),
	}
	return info
}

func ffnFamilyHint(ff byte) string {
	switch ff {
	case 0:
		return "auto"
	case 1:
		return "roman"
	case 2:
		return "swiss"
	case 3:
		return "modern"
	case 4:
		return "script"
	case 5:
		return "decorative"
	}
	return ""
}

func ffnPitchHint(prq byte) string {
	switch prq {
	case 0:
		return "default"
	case 1:
		return "fixed"
	case 2:
		return "variable"
	}
	return ""
}

func ffnCharsetHint(chs byte) string {
	if chs == 0 {
		return ""
	}
	return fmt.Sprintf("%02X", chs)
}

func ffnPanoseHex(panose []byte) string {
	if len(panose) < 10 {
		return ""
	}
	allZero := true
	for i := 0; i < 10; i++ {
		if panose[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return ""
	}
	return fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X",
		panose[0], panose[1], panose[2], panose[3], panose[4],
		panose[5], panose[6], panose[7], panose[8], panose[9])
}

func parseLegacyFFNXszUTF16(data []byte) string {
	limit := len(data) - (len(data) % 2)
	if limit < 2 {
		return ""
	}
	u16 := make([]uint16, 0, limit/2)
	zeroOdd := 0
	for i := 0; i+1 < limit; i += 2 {
		v := binary.LittleEndian.Uint16(data[i : i+2])
		if v == 0 {
			break
		}
		u16 = append(u16, v)
		if data[i+1] == 0 {
			zeroOdd++
		}
	}
	if len(u16) == 0 || zeroOdd*2 < len(u16) {
		return ""
	}
	return string(utf16.Decode(u16))
}

func parseLegacyFFNXszSingleByte(data []byte) string {
	end := bytesIndexByte(data, 0)
	if end < 0 {
		end = len(data)
	}
	return string(data[:end])
}

func sanitizeLegacyFontName(name string) string {
	name = strings.TrimSpace(strings.Trim(name, "\x00"))
	if len(name) == 0 || len(name) > 64 {
		return ""
	}
	letters := 0
	for _, r := range name {
		if r < 0x20 || r == '\uFFFD' {
			return ""
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letters++
			continue
		}
		if strings.ContainsRune(" -_&+.,()/'", r) {
			continue
		}
		return ""
	}
	if letters == 0 {
		return ""
	}
	return name
}

func scoreLegacyFontNames(names []string) int {
	if len(names) == 0 {
		return 0
	}
	score := 0
	for _, name := range names {
		if name == "" {
			continue
		}
		score += len(name)
	}
	return score
}

func scoreLegacyFontEntries(entries []legacyFontEntry) int {
	if len(entries) == 0 {
		return 0
	}
	score := 0
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		score += len(e.Name)
	}
	return score
}

func bytesIndexByte(b []byte, want byte) int {
	for i, v := range b {
		if v == want {
			return i
		}
	}
	return -1
}
