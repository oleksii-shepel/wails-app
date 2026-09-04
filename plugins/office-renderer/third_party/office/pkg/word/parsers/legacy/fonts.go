package legacy

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
)

// legacyFontEntry pairs an FFN-derived display name with its OOXML-flavored
// metadata (family/pitch/charset/PANOSE) so callers can build wordir.Document
// FontFamilies maps without re-parsing the SttbfFfn blob.
type legacyFontEntry struct {
	Name string
	Info wordir.FontFamilyInfo
}

type legacyEmbeddedFontDescriptor struct {
	Offset      int
	FontIndex   int
	Bold        bool
	Italic      bool
	SubsetOrder uint32
}

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

// loadLegacyDocFontFamilies builds a name→FontFamilyInfo map suitable for
// Document.FontFamilies, keyed by lowercased+trimmed font name.
func loadLegacyDocFontFamilies(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) map[string]wordir.FontFamilyInfo {
	entries := loadLegacyFontEntries(wordStream, tableStream, fibBase, cbRgFcLcb)
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]wordir.FontFamilyInfo, len(entries))
	for _, e := range entries {
		key := strings.ToLower(strings.TrimSpace(e.Name))
		if key == "" {
			continue
		}
		if _, exists := out[key]; exists {
			continue
		}
		out[key] = e.Info
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func loadLegacyDocEmbeddedFonts(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []*wordir.EmbeddedFont {
	descriptors := loadLegacyEmbeddedFontDescriptors(wordStream, tableStream, fibBase, cbRgFcLcb)
	if len(descriptors) == 0 {
		return nil
	}
	fontEntries := loadLegacyFontEntries(wordStream, tableStream, fibBase, cbRgFcLcb)
	if len(fontEntries) == 0 {
		return nil
	}
	sort.Slice(descriptors, func(i, j int) bool {
		if descriptors[i].Offset != descriptors[j].Offset {
			return descriptors[i].Offset < descriptors[j].Offset
		}
		return descriptors[i].SubsetOrder < descriptors[j].SubsetOrder
	})

	familyOrder := make([]string, 0, len(descriptors))
	families := map[string]*wordir.EmbeddedFont{}
	faceByKey := map[string]*wordir.EmbeddedFontFace{}
	for i, desc := range descriptors {
		if desc.FontIndex < 0 || desc.FontIndex >= len(fontEntries) {
			continue
		}
		family := strings.TrimSpace(fontEntries[desc.FontIndex].Name)
		if family == "" {
			continue
		}
		nextOffset := len(wordStream)
		for j := i + 1; j < len(descriptors); j++ {
			if descriptors[j].Offset > desc.Offset {
				nextOffset = descriptors[j].Offset
				break
			}
		}
		data := legacyEmbeddedFontDataSlice(wordStream, desc.Offset, nextOffset)
		if len(data) == 0 {
			continue
		}
		familyKey := strings.ToLower(family)
		font := families[familyKey]
		if font == nil {
			font = &wordir.EmbeddedFont{Family: family}
			families[familyKey] = font
			familyOrder = append(familyOrder, familyKey)
		}
		weight := 400
		if desc.Bold {
			weight = 700
		}
		style := "normal"
		if desc.Italic {
			style = "italic"
		}
		faceKey := fmt.Sprintf("%s|%d|%s", familyKey, weight, style)
		if _, exists := faceByKey[faceKey]; exists {
			continue
		}
		face := &wordir.EmbeddedFontFace{
			Family: family,
			Weight: weight,
			Style:  style,
			Data:   data,
		}
		if legacyLooksLikeEOT(data) {
			face.ContentType = "application/vnd.ms-fontobject"
		}
		font.Faces = append(font.Faces, face)
		faceByKey[faceKey] = face
	}
	if len(familyOrder) == 0 {
		return nil
	}
	out := make([]*wordir.EmbeddedFont, 0, len(familyOrder))
	for _, key := range familyOrder {
		font := families[key]
		if font == nil || len(font.Faces) == 0 {
			continue
		}
		out = append(out, font)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func loadLegacyEmbeddedFontDescriptors(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []legacyEmbeddedFontDescriptor {
	const ttmbdFibIndex = 61
	fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, ttmbdFibIndex)
	if !ok || lcb < 10 || fc < 0 || fc+lcb > len(tableStream) {
		return nil
	}
	blob := tableStream[fc : fc+lcb]
	ibstMac := int(int16(binary.LittleEndian.Uint16(blob[2:4])))
	if ibstMac <= 0 || ibstMac > 64 {
		return nil
	}
	start := int(binary.LittleEndian.Uint16(blob[8:10]))
	if start <= 0 {
		start = 10
	}
	const ttmbdSize = 12
	if start+ibstMac*ttmbdSize > len(blob) {
		return nil
	}
	descriptors := make([]legacyEmbeddedFontDescriptor, 0, ibstMac)
	for i := 0; i < ibstMac; i++ {
		off := start + i*ttmbdSize
		fcFont := int(binary.LittleEndian.Uint32(blob[off : off+4]))
		fontIndex := int(int16(binary.LittleEndian.Uint16(blob[off+4 : off+6])))
		flags := binary.LittleEndian.Uint16(blob[off+6 : off+8])
		if fcFont <= 0 || fontIndex < 0 {
			continue
		}
		descriptors = append(descriptors, legacyEmbeddedFontDescriptor{
			Offset:      fcFont,
			FontIndex:   fontIndex,
			Bold:        flags&0x0001 != 0,
			Italic:      flags&0x0002 != 0,
			SubsetOrder: binary.LittleEndian.Uint32(blob[off+8 : off+12]),
		})
	}
	if len(descriptors) == 0 {
		return nil
	}
	return descriptors
}

func legacyEmbeddedFontDataSlice(wordStream []byte, offset, nextOffset int) []byte {
	if offset < 0 || offset >= len(wordStream) {
		return nil
	}
	end := len(wordStream)
	if nextOffset > offset && nextOffset < end {
		end = nextOffset
	}
	if offset+4 <= len(wordStream) {
		if size := int(binary.LittleEndian.Uint32(wordStream[offset : offset+4])); size > 0 && offset+size <= len(wordStream) {
			if candidateEnd := offset + size; candidateEnd < end {
				end = candidateEnd
			}
		}
	}
	if end <= offset {
		return nil
	}
	data := append([]byte(nil), wordStream[offset:end]...)
	if len(data) == 0 {
		return nil
	}
	return data
}

func legacyLooksLikeEOT(data []byte) bool {
	if len(data) < 36 {
		return false
	}
	version := binary.LittleEndian.Uint32(data[8:12])
	if version != 0x00010000 && version != 0x00020001 && version != 0x00020002 {
		return false
	}
	return binary.LittleEndian.Uint16(data[34:36]) == 0x504C
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