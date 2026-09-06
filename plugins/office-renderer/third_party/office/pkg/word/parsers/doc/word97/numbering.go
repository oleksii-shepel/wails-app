package word97

import (
	"encoding/binary"
	"io"
	"sort"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	docformat "office/pkg/word/parsers/doc/format"
)

const (
	fibPlfLfo = 20
	fibPlfLst = 21
)

type NumberingLevel struct {
	Level           int
	Start           int
	Format          string
	Text            string
	IndentLeft      int
	IndentFirstLine int
	IndentHanging   int
	RestartOnLevel  int
	NoRestart       bool
	RunFormat       *wordir.RunFormat
}

type NumberingInstance struct {
	NumID         int
	AbstractNumID int
	Overrides     map[int]wordir.LegacyNumberingOverride
}

type NumberingCatalog struct {
	Abstract  map[int][]NumberingLevel
	Instances map[int]NumberingInstance
}

func LoadNumberingCatalogReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) *NumberingCatalog {
	lst, okLst := readTableRange(table, tableSize, fib, layout, fibPlfLst, 2)
	lfo, okLfo := readTableRange(table, tableSize, fib, layout, fibPlfLfo, 4)
	if !okLst || !okLfo {
		return nil
	}
	abstract, ok := parsePlfLst(lst)
	if !ok || len(abstract) == 0 {
		return nil
	}
	instances, ok := parsePlfLfo(lfo)
	if !ok || len(instances) == 0 {
		return nil
	}
	return &NumberingCatalog{Abstract: abstract, Instances: instances}
}

func parsePlfLst(blob []byte) (map[int][]NumberingLevel, bool) {
	// MS-DOC PlfLst begins with cLst, an unsigned 16-bit count, followed by
	// cLst fixed-size LSTF records and then their variable-length LVL records.
	if len(blob) < 2 {
		return nil, false
	}
	count := int(binary.LittleEndian.Uint16(blob[:2]))
	if count <= 0 || count > 4096 || 2+count*28 > len(blob) {
		return nil, false
	}
	type header struct {
		lsid   int
		simple bool
	}
	headers := make([]header, count)
	pos := 2
	for i := 0; i < count; i++ {
		rec := blob[pos : pos+28]
		// LSTF.fSimpleList is bit 0 of the final flags byte. The writer leaves
		// it clear and emits nine levels even for a one-level semantic list.
		headers[i] = header{lsid: int(binary.LittleEndian.Uint32(rec[:4])), simple: rec[26]&0x01 != 0}
		pos += 28
	}
	out := make(map[int][]NumberingLevel, count)
	for _, h := range headers {
		levels := 9
		if h.simple {
			levels = 1
		}
		decoded := make([]NumberingLevel, 0, levels)
		for level := 0; level < levels; level++ {
			entry, consumed, ok := parseLVL(blob[pos:], level)
			if !ok {
				return nil, false
			}
			decoded = append(decoded, entry)
			pos += consumed
		}
		out[h.lsid] = decoded
	}
	return out, true
}

func parseLVL(blob []byte, level int) (NumberingLevel, int, bool) {
	if len(blob) < 28 {
		return NumberingLevel{}, 0, false
	}
	start := int(int32(binary.LittleEndian.Uint32(blob[:4])))
	nfc := blob[4]
	flags := blob[5]
	rgbxchNums := append([]byte(nil), blob[6:15]...)
	cbChpx := int(blob[24])
	cbPapx := int(blob[25])
	restartLim := int(blob[26])
	pos := 28
	if pos+cbPapx+cbChpx+2 > len(blob) {
		return NumberingLevel{}, 0, false
	}
	papx := append([]byte(nil), blob[pos:pos+cbPapx]...)
	pos += cbPapx
	chpx := append([]byte(nil), blob[pos:pos+cbChpx]...)
	pos += cbChpx
	text, consumed, ok := parseLevelXst(blob[pos:], rgbxchNums)
	if !ok {
		return NumberingLevel{}, 0, false
	}
	pos += consumed

	entry := NumberingLevel{Level: level, Start: start, Format: numberingFormat(nfc), Text: text, RestartOnLevel: -1, NoRestart: flags&(1<<3) != 0}
	// ilvlRestartLim is encoded as level+1; zero means no explicit restart
	// limit. Normalize back to the zero-based semantic level.
	if restartLim > 0 && restartLim != 0xFF {
		entry.RestartOnLevel = restartLim - 1
	}
	if len(papx) > 0 {
		pf, _ := docformat.DecodePAPX(papx)
		if pf.IndentLeft != nil {
			entry.IndentLeft = *pf.IndentLeft
		}
		if pf.IndentFirstLine != nil {
			entry.IndentFirstLine = *pf.IndentFirstLine
			if *pf.IndentFirstLine < 0 {
				entry.IndentHanging = -*pf.IndentFirstLine
			}
		}
		if pf.IndentHanging != nil {
			entry.IndentHanging = *pf.IndentHanging
		}
	}
	if len(chpx) > 0 {
		rf := docformat.DecodeCHPX(chpx, nil).Format
		entry.RunFormat = cloneNumberingRunFormat(&rf)
	}
	return entry, pos, true
}

func parsePlfLfo(blob []byte) (map[int]NumberingInstance, bool) {
	if len(blob) < 4 {
		return nil, false
	}
	count := int(binary.LittleEndian.Uint32(blob[:4]))
	if count <= 0 || count > 4096 || 4+count*16 > len(blob) {
		return nil, false
	}
	type lfoHeader struct {
		lsid    int
		clfolvl int
	}
	headers := make([]lfoHeader, count)
	pos := 4
	for i := 0; i < count; i++ {
		rec := blob[pos : pos+16]
		headers[i] = lfoHeader{lsid: int(binary.LittleEndian.Uint32(rec[:4])), clfolvl: int(rec[12])}
		pos += 16
	}
	out := make(map[int]NumberingInstance, count)
	for i, h := range headers {
		inst := NumberingInstance{NumID: i + 1, AbstractNumID: h.lsid}
		if h.clfolvl > 0 {
			inst.Overrides = map[int]wordir.LegacyNumberingOverride{}
		}

		// Each LFO is followed by a four-byte grfhic/lfoData sentinel before its
		// LFOLVL array. Word and our writer use 0xFFFFFFFF here. Tolerate other
		// values but always consume the field because it is part of the layout.
		if pos+4 > len(blob) {
			return nil, false
		}
		pos += 4

		for j := 0; j < h.clfolvl; j++ {
			if pos+8 > len(blob) {
				return nil, false
			}
			rec := blob[pos : pos+8]
			start := int(int32(binary.LittleEndian.Uint32(rec[:4])))
			packed := binary.LittleEndian.Uint32(rec[4:8])
			level := int(packed & 0x0F)
			startAt := packed&(1<<4) != 0
			formatting := packed&(1<<5) != 0
			pos += 8
			if level < 0 || level > 8 {
				return nil, false
			}
			var override wordir.LegacyNumberingOverride
			has := false
			if startAt {
				v := start
				override.Start = &v
				has = true
			}
			if formatting {
				lvl, consumed, ok := parseLVL(blob[pos:], level)
				if !ok {
					return nil, false
				}
				pos += consumed
				if lvl.Format != "" {
					override.Format = lvl.Format
					has = true
				}
				if override.Start == nil && lvl.Start > 0 {
					v := lvl.Start
					override.Start = &v
					has = true
				}
				if lvl.RestartOnLevel >= 0 {
					v := lvl.RestartOnLevel
					override.RestartOnLevel = &v
					has = true
				}
				if lvl.NoRestart {
					v := false
					override.Restart = &v
				}
			}
			if has {
				inst.Overrides[level] = override
			}
		}
		out[inst.NumID] = inst
	}
	return out, true
}

func parseLevelXst(blob []byte, rgbxchNums []byte) (string, int, bool) {
	if len(blob) < 2 {
		return "", 0, false
	}
	cch := int(binary.LittleEndian.Uint16(blob[:2]))
	need := 2 + cch*2
	if cch < 0 || need > len(blob) {
		return "", 0, false
	}
	units := make([]uint16, cch)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(blob[2+i*2:])
	}
	return reconstructLevelText(units, rgbxchNums), need, true
}

// reconstructLevelText uses LVLF.rgbxchNums, the authoritative zero-based
// UTF-16 code-unit offsets of numbering placeholders. The character stored in
// the XST at a placeholder position is display data (often '1', '2', ...), not
// the semantic level identity. Reconstructing from offsets preserves templates
// such as %1.%2. instead of freezing them to literal text like 1.2.
func reconstructLevelText(units []uint16, rgbxchNums []byte) string {
	if len(units) == 0 {
		return ""
	}
	placeholderAt := make(map[int]int, len(rgbxchNums))
	for level, raw := range rgbxchNums {
		if raw == 0xFF {
			continue
		}
		pos := int(raw)
		if pos < 0 || pos >= len(units) {
			continue
		}
		if _, exists := placeholderAt[pos]; !exists {
			placeholderAt[pos] = level
		}
	}

	out := make([]rune, 0, len(units)+len(placeholderAt))
	chunkStart := 0
	flush := func(end int) {
		if end <= chunkStart {
			return
		}
		out = append(out, utf16.Decode(units[chunkStart:end])...)
	}
	for i := 0; i < len(units); i++ {
		level, ok := placeholderAt[i]
		if !ok {
			continue
		}
		flush(i)
		out = append(out, '%', rune('1'+level))
		chunkStart = i + 1
	}
	flush(len(units))
	return string(out)
}

func numberingFormat(nfc byte) string {
	switch nfc {
	case 0:
		return "decimal"
	case 1:
		return "upperRoman"
	case 2:
		return "lowerRoman"
	case 3:
		return "upperLetter"
	case 4:
		return "lowerLetter"
	case 5:
		return "ordinal"
	case 6:
		return "cardinalText"
	case 7:
		return "ordinalText"
	case 22:
		return "decimalZero"
	case 23:
		return "bullet"
	default:
		return ""
	}
}

func (c *NumberingCatalog) ApplyToDocument(doc *wordir.Document) {
	if c == nil || doc == nil {
		return
	}
	ids := make([]int, 0, len(c.Instances))
	for id := range c.Instances {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	doc.NumberingDefinitions = nil
	for _, id := range ids {
		inst := c.Instances[id]
		def := wordir.NumberingDefinition{NumID: inst.NumID, AbstractNumID: inst.AbstractNumID}
		for _, lvl := range c.Abstract[inst.AbstractNumID] {
			start, format := effectiveLevel(inst, lvl)
			var runFormat *wordir.RunFormat
			if lvl.RunFormat != nil {
				runFormat = cloneNumberingRunFormat(lvl.RunFormat)
			}
			def.Levels = append(def.Levels, wordir.NumberingLevel{Level: lvl.Level, Start: start, Format: format, Text: lvl.Text, IndentLeft: lvl.IndentLeft, RunFormat: runFormat})
		}
		doc.NumberingDefinitions = append(doc.NumberingDefinitions, def)
	}
	for i := range doc.Paragraphs {
		p := &doc.Paragraphs[i]
		if p.Numbering == nil {
			continue
		}
		inst, ok := c.Instances[p.Numbering.NumID]
		if !ok {
			continue
		}
		lvl, ok := findNumberingLevel(c.Abstract[inst.AbstractNumID], p.Numbering.Level)
		if !ok {
			continue
		}
		start, format := effectiveLevel(inst, lvl)
		p.Numbering.AbstractNumID = inst.AbstractNumID
		p.Numbering.Start = start
		p.Numbering.Format = format
		p.Numbering.Text = lvl.Text
		p.Numbering.IndentLeft = lvl.IndentLeft
		p.Numbering.IndentFirstLine = lvl.IndentFirstLine
		p.Numbering.IndentHanging = lvl.IndentHanging
		p.Numbering.RunFormat = cloneNumberingRunFormat(lvl.RunFormat)
		if override, ok := inst.Overrides[lvl.Level]; ok {
			v := cloneLegacyOverride(override)
			p.Numbering.LegacyOverride = &v
		}
		kind := "ordered"
		if format == "bullet" {
			kind = "bullet"
		}
		p.List = &wordir.ListMarker{Kind: kind, Level: lvl.Level, Start: start, Format: format, Template: lvl.Text}
	}
}

func effectiveLevel(inst NumberingInstance, lvl NumberingLevel) (int, string) {
	start, format := lvl.Start, lvl.Format
	if override, ok := inst.Overrides[lvl.Level]; ok {
		if override.Start != nil {
			start = *override.Start
		}
		if override.Format != "" {
			format = override.Format
		}
	}
	return start, format
}

func findNumberingLevel(levels []NumberingLevel, level int) (NumberingLevel, bool) {
	for _, candidate := range levels {
		if candidate.Level == level {
			return candidate, true
		}
	}
	return NumberingLevel{}, false
}

func cloneNumberingRunFormat(src *wordir.RunFormat) *wordir.RunFormat {
	if src == nil {
		return nil
	}
	v := *src
	cloneBool := func(p *bool) *bool { if p == nil { return nil }; x := *p; return &x }
	cloneInt := func(p *int) *int { if p == nil { return nil }; x := *p; return &x }
	v.Bold = cloneBool(src.Bold)
	v.Italic = cloneBool(src.Italic)
	v.Caps = cloneBool(src.Caps)
	v.SmallCaps = cloneBool(src.SmallCaps)
	v.Strike = cloneBool(src.Strike)
	v.FontSize = cloneInt(src.FontSize)
	v.Vanish = cloneBool(src.Vanish)
	v.CharacterSpacing = cloneInt(src.CharacterSpacing)
	v.RTL = cloneBool(src.RTL)
	v.Outline = cloneBool(src.Outline)
	v.Shadow = cloneBool(src.Shadow)
	v.Emboss = cloneBool(src.Emboss)
	v.Imprint = cloneBool(src.Imprint)
	return &v
}

func cloneLegacyOverride(src wordir.LegacyNumberingOverride) wordir.LegacyNumberingOverride {
	out := src
	if src.Start != nil { v := *src.Start; out.Start = &v }
	if src.Restart != nil { v := *src.Restart; out.Restart = &v }
	if src.RestartOnLevel != nil { v := *src.RestartOnLevel; out.RestartOnLevel = &v }
	return out
}
