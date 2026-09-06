package word97

import (
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

type FontEntry struct {
	Name string
	Info wordir.FontFamilyInfo
}

type embeddedFontDescriptor struct {
	Offset      int64
	FontIndex   int
	Bold        bool
	Italic      bool
	SubsetOrder uint32
}

// LoadFontFamiliesReaderAt decodes the canonical SttbfFfn table. If the
// nominal FIB slot is malformed, it retains the old compatibility behavior by
// checking other FIB-addressed ranges one at a time rather than materializing
// the table stream.
func LoadFontFamiliesReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) map[string]wordir.FontFamilyInfo {
	entries := loadOrderedFontEntriesReaderAt(table, tableSize, fib, layout)
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]wordir.FontFamilyInfo, len(entries))
	for _, entry := range entries {
		key := strings.ToLower(strings.TrimSpace(entry.Name))
		if key == "" {
			continue
		}
		if _, exists := out[key]; !exists {
			out[key] = entry.Info
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// LoadEmbeddedFontsReaderAt decodes SttbTtmbd descriptors and reads embedded
// font payloads directly from WordDocument. Binary ownership stays in the
// canonical Word97 reader; callers receive renderer-ready IR font faces.
func LoadEmbeddedFontsReaderAt(
	word io.ReaderAt,
	wordSize int64,
	table io.ReaderAt,
	tableSize int64,
	fib []byte,
	layout docbinary.Word97Layout,
) []*wordir.EmbeddedFont {
	if word == nil || wordSize <= 0 || table == nil || tableSize <= 0 {
		return nil
	}
	blob, ok := readTableRange(table, tableSize, fib, layout, fibEmbeddedFonts, 10)
	if !ok {
		return nil
	}
	descriptors := parseEmbeddedFontDescriptors(blob)
	if len(descriptors) == 0 {
		return nil
	}
	fontEntries := loadOrderedFontEntriesReaderAt(table, tableSize, fib, layout)
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
	families := make(map[string]*wordir.EmbeddedFont)
	faces := make(map[string]struct{})
	for i, descriptor := range descriptors {
		if descriptor.FontIndex < 0 || descriptor.FontIndex >= len(fontEntries) {
			continue
		}
		family := strings.TrimSpace(fontEntries[descriptor.FontIndex].Name)
		if family == "" {
			continue
		}
		nextOffset := wordSize
		for j := i + 1; j < len(descriptors); j++ {
			if descriptors[j].Offset > descriptor.Offset {
				nextOffset = descriptors[j].Offset
				break
			}
		}
		data := readEmbeddedFontPayload(word, wordSize, descriptor.Offset, nextOffset)
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
		if descriptor.Bold {
			weight = 700
		}
		style := "normal"
		if descriptor.Italic {
			style = "italic"
		}
		faceKey := fmt.Sprintf("%s|%d|%s", familyKey, weight, style)
		if _, exists := faces[faceKey]; exists {
			continue
		}
		face := &wordir.EmbeddedFontFace{
			Family: family,
			Weight: weight,
			Style:  style,
			Data:   data,
		}
		if looksLikeEOT(data) {
			face.ContentType = "application/vnd.ms-fontobject"
		}
		font.Faces = append(font.Faces, face)
		faces[faceKey] = struct{}{}
	}

	if len(familyOrder) == 0 {
		return nil
	}
	out := make([]*wordir.EmbeddedFont, 0, len(familyOrder))
	for _, key := range familyOrder {
		font := families[key]
		if font != nil && len(font.Faces) > 0 {
			out = append(out, font)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func loadOrderedFontEntriesReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) []FontEntry {
	var best []FontEntry
	bestScore := 0
	try := func(index int) {
		blob, ok := readTableRange(table, tableSize, fib, layout, index, 1)
		if !ok {
			return
		}
		entries := parseFontTable(blob)
		if score := fontScore(entries); score > bestScore {
			best = entries
			bestScore = score
		}
	}
	try(fibFontTable)
	for i := 0; i < layout.CbRgFcLcb; i++ {
		if i != fibFontTable {
			try(i)
		}
	}
	if bestScore == 0 {
		return nil
	}
	return best
}

func parseEmbeddedFontDescriptors(blob []byte) []embeddedFontDescriptor {
	if len(blob) < 10 {
		return nil
	}
	count := int(int16(binary.LittleEndian.Uint16(blob[2:4])))
	if count <= 0 || count > 64 {
		return nil
	}
	start := int(binary.LittleEndian.Uint16(blob[8:10]))
	if start <= 0 {
		start = 10
	}
	const descriptorSize = 12
	if start+count*descriptorSize > len(blob) {
		return nil
	}
	out := make([]embeddedFontDescriptor, 0, count)
	for i := 0; i < count; i++ {
		off := start + i*descriptorSize
		fontOffset := int64(binary.LittleEndian.Uint32(blob[off : off+4]))
		fontIndex := int(int16(binary.LittleEndian.Uint16(blob[off+4 : off+6])))
		flags := binary.LittleEndian.Uint16(blob[off+6 : off+8])
		if fontOffset <= 0 || fontIndex < 0 {
			continue
		}
		out = append(out, embeddedFontDescriptor{
			Offset:      fontOffset,
			FontIndex:   fontIndex,
			Bold:        flags&0x0001 != 0,
			Italic:      flags&0x0002 != 0,
			SubsetOrder: binary.LittleEndian.Uint32(blob[off+8 : off+12]),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func readEmbeddedFontPayload(word io.ReaderAt, wordSize, offset, nextOffset int64) []byte {
	if word == nil || offset < 0 || offset >= wordSize {
		return nil
	}
	end := wordSize
	if nextOffset > offset && nextOffset < end {
		end = nextOffset
	}
	var header [4]byte
	if offset+4 <= wordSize {
		if n, err := word.ReadAt(header[:], offset); n == len(header) && (err == nil || err == io.EOF) {
			size := int64(binary.LittleEndian.Uint32(header[:]))
			if size > 0 && offset+size <= wordSize && offset+size < end {
				end = offset + size
			}
		}
	}
	if end <= offset {
		return nil
	}
	data := make([]byte, end-offset)
	n, err := word.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil
	}
	if n != len(data) {
		return nil
	}
	return data
}

func looksLikeEOT(data []byte) bool {
	if len(data) < 36 {
		return false
	}
	version := binary.LittleEndian.Uint32(data[8:12])
	if version != 0x00010000 && version != 0x00020001 && version != 0x00020002 {
		return false
	}
	return binary.LittleEndian.Uint16(data[34:36]) == 0x504C
}

func parseFontTable(blob []byte) []FontEntry {
	var best []FontEntry
	bestScore := 0
	for _, start := range []int{0, 2, 4, 6} {
		if start >= len(blob) {
			continue
		}
		value := parseFFN(blob[start:])
		if score := fontScore(value); score > bestScore {
			best = value
			bestScore = score
		}
	}
	return best
}

func parseFFN(blob []byte) []FontEntry {
	const fixed = 40
	var out []FontEntry
	for pos := 0; pos < len(blob); {
		size := int(blob[pos]) + 1
		if size <= fixed || pos+size > len(blob) {
			break
		}
		record := blob[pos : pos+size]
		name := fontName(record)
		if name != "" {
			out = append(out, FontEntry{Name: name, Info: fontInfo(record)})
		}
		pos += size
	}
	return out
}

func fontName(record []byte) string {
	if len(record) <= 40 {
		return ""
	}
	data := record[40:]
	limit := len(data) - len(data)%2
	var units []uint16
	zeros := 0
	for i := 0; i+1 < limit; i += 2 {
		value := binary.LittleEndian.Uint16(data[i : i+2])
		if value == 0 {
			break
		}
		units = append(units, value)
		if data[i+1] == 0 {
			zeros++
		}
	}
	if len(units) > 0 && zeros*2 >= len(units) {
		return cleanFontName(string(utf16.Decode(units)))
	}
	end := 0
	for end < len(data) && data[end] != 0 {
		end++
	}
	return cleanFontName(string(data[:end]))
}

func cleanFontName(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || len(value) > 127 {
		return ""
	}
	for _, r := range value {
		if r < 0x20 {
			return ""
		}
	}
	return value
}

func fontInfo(record []byte) wordir.FontFamilyInfo {
	if len(record) < 16 {
		return wordir.FontFamilyInfo{}
	}
	flags := record[1]
	return wordir.FontFamilyInfo{
		Family:  fontFamily((flags >> 4) & 7),
		Pitch:   fontPitch(flags & 3),
		Charset: fontCharset(record[4]),
		Panose1: fontPanose(record[6:16]),
	}
}

func fontFamily(value byte) string {
	switch value {
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
	default:
		return ""
	}
}

func fontPitch(value byte) string {
	switch value {
	case 0:
		return "default"
	case 1:
		return "fixed"
	case 2:
		return "variable"
	default:
		return ""
	}
}

func fontCharset(value byte) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%02X", value)
}

func fontPanose(value []byte) string {
	if len(value) < 10 {
		return ""
	}
	nonZero := false
	for _, b := range value[:10] {
		nonZero = nonZero || b != 0
	}
	if !nonZero {
		return ""
	}
	return fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X", value[0], value[1], value[2], value[3], value[4], value[5], value[6], value[7], value[8], value[9])
}

func fontScore(value []FontEntry) int {
	score := 0
	for _, entry := range value {
		if entry.Name != "" {
			score += 2
		}
		if entry.Info.Family != "" || entry.Info.Pitch != "" {
			score++
		}
	}
	return score
}
