package legacy

import (
	"bytes"
	"encoding/binary"
	"sort"
	"strings"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
)

type legacyNumberingWriteLevel struct {
	Level      int
	Start      int
	Format     string
	Text       string
	IndentLeft int
	RunFormat  *wordir.RunFormat
}

type legacyNumberingWriteDefinition struct {
	NumID         int
	AbstractNumID int
	LSID          uint32
	Levels        []legacyNumberingWriteLevel
	Overrides     map[int]wordir.LegacyNumberingOverride
}

type legacyNumberingWriteCatalog struct {
	Definitions  []legacyNumberingWriteDefinition
	IlfoByNumID  map[int]int
	FallbackIlfo int
}

func (c legacyNumberingWriteCatalog) ilfoForNumID(numID int) int {
	if numID > 0 {
		if ilfo, ok := c.IlfoByNumID[numID]; ok {
			return ilfo
		}
	}
	return c.FallbackIlfo
}

func (c legacyNumberingWriteCatalog) ilfoForParagraph(p *wordir.Paragraph) int {
	if p == nil {
		return 0
	}
	if p.Numbering != nil {
		return c.ilfoForNumID(p.Numbering.NumID)
	}
	if len(c.Definitions) == 1 {
		return 1
	}
	return c.FallbackIlfo
}

func buildLegacyNumberingWriteCatalog(doc *wordir.Document) legacyNumberingWriteCatalog {
	c := legacyNumberingWriteCatalog{IlfoByNumID: map[int]int{}}
	if doc == nil {
		return c
	}

	defsByNumID := map[int]legacyNumberingWriteDefinition{}
	for _, def := range doc.NumberingDefinitions {
		numID := def.NumID
		if numID <= 0 {
			continue
		}
		abstract := def.AbstractNumID
		if abstract <= 0 {
			abstract = numID
		}
		levels := make([]legacyNumberingWriteLevel, 0, len(def.Levels))
		for _, level := range def.Levels {
			levels = append(levels, legacyNumberingWriteLevel{
				Level:      clampLegacyListLevel(level.Level),
				Start:      positiveLegacyListStart(level.Start),
				Format:     normalizeLegacyListFormat(level.Format),
				Text:       normalizeLegacyListTemplate(level.Text, level.Level, level.Format),
				IndentLeft: level.IndentLeft,
				RunFormat:  cloneLegacyRunFormatPtr(level.RunFormat),
			})
		}
		defsByNumID[numID] = legacyNumberingWriteDefinition{
			NumID: numID, AbstractNumID: abstract, LSID: legacyNumberingLSID(abstract, numID),
			Levels: normalizeLegacyWriteLevels(levels), Overrides: map[int]wordir.LegacyNumberingOverride{},
		}
	}

	// Legacy .doc parsing primarily leaves paragraph-level Numbering values rather
	// than a persistent NumberingDefinitions catalog. Synthesize definitions from
	// observed paragraphs so a read -> write -> read cycle still retains lists.
	for _, paragraph := range legacyDocumentParagraphs(doc) {
		if paragraph == nil || (paragraph.Numbering == nil && paragraph.List == nil) {
			continue
		}
		numID := 1
		abstract := 1
		level := 0
		start := 1
		format := ""
		text := ""
		indent := 0
		var runFormat *wordir.RunFormat
		if paragraph.Numbering != nil {
			if paragraph.Numbering.NumID > 0 {
				numID = paragraph.Numbering.NumID
			}
			if paragraph.Numbering.AbstractNumID > 0 {
				abstract = paragraph.Numbering.AbstractNumID
			} else {
				abstract = numID
			}
			level = clampLegacyListLevel(paragraph.Numbering.Level)
			start = positiveLegacyListStart(paragraph.Numbering.Start)
			format = normalizeLegacyListFormat(paragraph.Numbering.Format)
			text = normalizeLegacyListTemplate(paragraph.Numbering.Text, level, format)
			indent = paragraph.Numbering.IndentLeft
			runFormat = cloneLegacyRunFormatPtr(paragraph.Numbering.RunFormat)
		} else if paragraph.List != nil {
			level = clampLegacyListLevel(paragraph.List.Level)
			start = positiveLegacyListStart(paragraph.List.Start)
			format = normalizeLegacyListFormat(paragraph.List.Format)
			if paragraph.List.Kind == "bullet" {
				format = "bullet"
			}
			text = normalizeLegacyListTemplate(paragraph.List.Template, level, format)
			if paragraph.Format.IndentLeft != nil {
				indent = *paragraph.Format.IndentLeft
			}
		}
		if format == "" {
			format = "decimal"
		}

		def, ok := defsByNumID[numID]
		if !ok {
			def = legacyNumberingWriteDefinition{
				NumID: numID, AbstractNumID: abstract, LSID: legacyNumberingLSID(abstract, numID),
				Overrides: map[int]wordir.LegacyNumberingOverride{},
			}
		}
		if def.Overrides == nil {
			def.Overrides = map[int]wordir.LegacyNumberingOverride{}
		}
		if paragraph.Numbering != nil && paragraph.Numbering.LegacyOverride != nil {
			def.Overrides[level] = cloneLegacyNumberingOverride(*paragraph.Numbering.LegacyOverride)
		}
		def.Levels = mergeLegacyWriteLevel(def.Levels, legacyNumberingWriteLevel{
			Level: level, Start: start, Format: format, Text: text,
			IndentLeft: indent, RunFormat: runFormat,
		})
		def.Levels = normalizeLegacyWriteLevels(def.Levels)
		defsByNumID[numID] = def
	}

	numIDs := make([]int, 0, len(defsByNumID))
	for numID := range defsByNumID {
		numIDs = append(numIDs, numID)
	}
	sort.Ints(numIDs)
	for _, numID := range numIDs {
		def := defsByNumID[numID]
		if len(def.Levels) == 0 {
			continue
		}
		c.Definitions = append(c.Definitions, def)
		c.IlfoByNumID[numID] = len(c.Definitions) // ilfo is 1-based LFO index
	}
	if len(c.Definitions) == 1 {
		c.FallbackIlfo = 1
	}
	return c
}

func legacyDocumentParagraphs(doc *wordir.Document) []*wordir.Paragraph {
	if doc == nil {
		return nil
	}
	out := make([]*wordir.Paragraph, 0, len(doc.Paragraphs))
	for i := range doc.Blocks {
		if doc.Blocks[i].Paragraph != nil {
			out = append(out, doc.Blocks[i].Paragraph)
		}
		if doc.Blocks[i].Table != nil {
			for r := range doc.Blocks[i].Table.Rows {
				for c := range doc.Blocks[i].Table.Rows[r].Cells {
					for p := range doc.Blocks[i].Table.Rows[r].Cells[c].Paragraphs {
						out = append(out, &doc.Blocks[i].Table.Rows[r].Cells[c].Paragraphs[p])
					}
				}
			}
		}
	}
	if len(out) == 0 {
		for i := range doc.Paragraphs {
			out = append(out, &doc.Paragraphs[i])
		}
	}
	return out
}

func buildLegacyPlfLstData(c legacyNumberingWriteCatalog) []byte {
	if len(c.Definitions) == 0 {
		return nil
	}

	// PlfLst stores unique abstract list definitions. Multiple numbering
	// instances can point to the same LSID through their LFO entry.
	byLSID := map[uint32]legacyNumberingWriteDefinition{}
	for _, def := range c.Definitions {
		if _, exists := byLSID[def.LSID]; !exists {
			byLSID[def.LSID] = def
		}
	}
	lsids := make([]uint32, 0, len(byLSID))
	for lsid := range byLSID {
		lsids = append(lsids, lsid)
	}
	sort.Slice(lsids, func(i, j int) bool { return lsids[i] < lsids[j] })

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(lsids)))

	// LSTF array first.
	for _, lsid := range lsids {
		def := byLSID[lsid]
		lstf := make([]byte, legacyLstfSize)
		binary.LittleEndian.PutUint32(lstf[0:4], lsid)
		// tplc and rgistd are left at zero. A nine-level list is used even for
		// single-level definitions because it is accepted by Word and avoids
		// inventing simple-list flags when the IR does not carry that distinction.
		_ = def
		out.Write(lstf)
	}

	// LVL records follow all LSTFs.
	for _, lsid := range lsids {
		def := byLSID[lsid]
		levels := indexLegacyWriteLevels(def.Levels)
		for level := 0; level < 9; level++ {
			lvl, ok := levels[level]
			if !ok {
				lvl = legacyNumberingWriteLevel{
					Level: level, Start: 1, Format: defaultLegacyListFormat(level),
					Text: defaultLegacyListTemplate(level),
				}
			}
			out.Write(buildLegacyLVLRecord(lvl))
		}
	}
	return out.Bytes()
}

func buildLegacyPlfLfoData(c legacyNumberingWriteCatalog) []byte {
	if len(c.Definitions) == 0 {
		return nil
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(c.Definitions)))
	for _, def := range c.Definitions {
		entry := make([]byte, legacyLfoSize)
		binary.LittleEndian.PutUint32(entry[0:4], def.LSID)
		count := 0
		for level := range def.Overrides {
			if level >= 0 && level <= 8 {
				count++
			}
		}
		entry[12] = byte(count)
		out.Write(entry)
	}
	for _, def := range c.Definitions {
		_ = binary.Write(&out, binary.LittleEndian, uint32(0xFFFFFFFF))
		levels := make([]int, 0, len(def.Overrides))
		for level := range def.Overrides {
			if level >= 0 && level <= 8 {
				levels = append(levels, level)
			}
		}
		sort.Ints(levels)
		baseLevels := indexLegacyWriteLevels(def.Levels)
		for _, level := range levels {
			override := def.Overrides[level]
			base := baseLevels[level]
			out.Write(buildLegacyLFOLVLRecord(level, override, base))
		}
	}
	return out.Bytes()
}

func buildLegacyLFOLVLRecord(level int, override wordir.LegacyNumberingOverride, base legacyNumberingWriteLevel) []byte {
	header := make([]byte, legacyLfolvlSize)
	flags := uint32(clampLegacyListLevel(level))
	if override.Start != nil && override.Format == "" && override.Restart == nil && override.RestartOnLevel == nil {
		binary.LittleEndian.PutUint32(header[0:4], uint32(*override.Start))
		flags |= 1 << 4 // fStartAt
		binary.LittleEndian.PutUint32(header[4:8], flags)
		return header
	}

	// Restart/format semantics are carried by the embedded LVL, so mark this as
	// a formatting override and start from the abstract level's known values.
	flags |= 1 << 5 // fFormatting
	binary.LittleEndian.PutUint32(header[4:8], flags)
	lvl := base
	lvl.Level = clampLegacyListLevel(level)
	if override.Start != nil {
		lvl.Start = *override.Start
	}
	if override.Format != "" {
		lvl.Format = override.Format
	}
	return append(header, buildLegacyLVLRecordWithRestart(lvl, override.Restart, override.RestartOnLevel)...)
}

func buildLegacyLVLRecordWithRestart(level legacyNumberingWriteLevel, restart *bool, restartOnLevel *int) []byte {
	record := buildLegacyLVLRecord(level)
	if len(record) < legacyLvlfSize {
		return record
	}
	if restart != nil && *restart {
		record[5] &^= legacyLVLFFlagNoRestart
		record[26] = 0
		return record
	}
	record[5] |= legacyLVLFFlagNoRestart
	if restartOnLevel != nil && *restartOnLevel >= 0 && *restartOnLevel <= 7 {
		record[26] = byte(*restartOnLevel + 1)
	}
	return record
}

func buildLegacyLVLRecord(level legacyNumberingWriteLevel) []byte {
	papx := []byte{}
	if level.IndentLeft != 0 {
		papx = appendSPRM16(papx, 0x840E, level.IndentLeft)
		// Word list markers generally hang from the left indent. We do not
		// synthesize a hanging amount because the normalized numbering model
		// only carries IndentLeft.
	}
	chpx := []byte{}
	if level.RunFormat != nil {
		chpx = encodeChpxGrpprl(*level.RunFormat)
	}
	if len(papx) > 255 {
		papx = papx[:255]
	}
	if len(chpx) > 255 {
		chpx = chpx[:255]
	}

	template := normalizeLegacyListTemplate(level.Text, level.Level, level.Format)
	xst, rgbx := encodeLegacyListTemplateXst(template)
	header := make([]byte, legacyLvlfSize)
	binary.LittleEndian.PutUint32(header[0:4], uint32(positiveLegacyListStart(level.Start)))
	header[4] = legacyListFormatNFC(level.Format)
	// fNoRestart is deliberately set: the normalized IR does not retain enough
	// information to prove Word's ilvlRestartLim semantics.
	header[5] = legacyLVLFFlagNoRestart
	copy(header[6:15], rgbx[:])
	header[24] = byte(len(chpx))
	header[25] = byte(len(papx))
	header[26] = 0
	header[27] = 0

	out := make([]byte, 0, len(header)+len(papx)+len(chpx)+len(xst))
	out = append(out, header...)
	out = append(out, papx...)
	out = append(out, chpx...)
	out = append(out, xst...)
	return out
}

func encodeLegacyListTemplateXst(template string) ([]byte, [9]byte) {
	var rgb [9]byte
	for i := range rgb {
		rgb[i] = 0xFF
	}
	template = strings.TrimSpace(template)
	if template == "" {
		template = "%1."
	}
	runes := []rune(template)
	outRunes := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		if runes[i] == '%' && i+1 < len(runes) && runes[i+1] >= '1' && runes[i+1] <= '9' {
			level := int(runes[i+1] - '1')
			// MS-DOC rgbxchNums stores a zero-based UTF-16 code-unit offset;
			// 0xFF marks a level that is not referenced by this template.
			// The placeholder code unit itself is semantically ignored.
			if level >= 0 && level < len(rgb) {
				rgb[level] = byte(len(utf16.Encode(outRunes)))
			}
			outRunes = append(outRunes, rune('1'+level))
			i++
			continue
		}
		outRunes = append(outRunes, runes[i])
	}
	encoded := utf16.Encode(outRunes)
	xst := make([]byte, 2+len(encoded)*2)
	binary.LittleEndian.PutUint16(xst[0:2], uint16(len(encoded)))
	for i, value := range encoded {
		binary.LittleEndian.PutUint16(xst[2+i*2:4+i*2], value)
	}
	return xst, rgb
}

func legacyListFormatNFC(format string) byte {
	switch normalizeLegacyListFormat(format) {
	case "decimal":
		return 0
	case "upperRoman":
		return 1
	case "lowerRoman":
		return 2
	case "upperLetter":
		return 3
	case "lowerLetter":
		return 4
	case "bullet":
		return 0x17
	default:
		return 0
	}
}

func normalizeLegacyListFormat(format string) string {
	switch strings.TrimSpace(format) {
	case "decimal", "upperRoman", "lowerRoman", "upperLetter", "lowerLetter", "bullet":
		return strings.TrimSpace(format)
	case "":
		return ""
	default:
		// The legacy binary reader currently exposes this same supported subset.
		return "decimal"
	}
}

func normalizeLegacyListTemplate(text string, level int, format string) string {
	text = strings.TrimSpace(text)
	if text == "binary" {
		text = ""
	}
	if normalizeLegacyListFormat(format) == "bullet" {
		if text == "" || strings.Contains(text, "%") {
			return "•"
		}
		return text
	}
	if text == "" {
		return "%" + string(rune('1'+clampLegacyListLevel(level))) + "."
	}
	// Numbering.Text in normalized IR can be a rendered current marker ("3.")
	// rather than a template. Only preserve it as a template when it actually
	// contains level placeholders.
	if !strings.Contains(text, "%") {
		return "%" + string(rune('1'+clampLegacyListLevel(level))) + "."
	}
	return text
}

func defaultLegacyListFormat(level int) string {
	switch clampLegacyListLevel(level) % 3 {
	case 1:
		return "lowerLetter"
	case 2:
		return "lowerRoman"
	default:
		return "decimal"
	}
}

func defaultLegacyListTemplate(level int) string {
	return "%" + string(rune('1'+clampLegacyListLevel(level))) + "."
}

func legacyNumberingLSID(abstract, numID int) uint32 {
	if abstract > 0 {
		return uint32(abstract)
	}
	if numID > 0 {
		return uint32(numID)
	}
	return 1
}

func positiveLegacyListStart(start int) int {
	if start <= 0 {
		return 1
	}
	return start
}

func clampLegacyListLevel(level int) int {
	if level < 0 {
		return 0
	}
	if level > 8 {
		return 8
	}
	return level
}

func normalizeLegacyWriteLevels(levels []legacyNumberingWriteLevel) []legacyNumberingWriteLevel {
	byLevel := indexLegacyWriteLevels(levels)
	keys := make([]int, 0, len(byLevel))
	for level := range byLevel {
		keys = append(keys, level)
	}
	sort.Ints(keys)
	out := make([]legacyNumberingWriteLevel, 0, len(keys))
	for _, level := range keys {
		v := byLevel[level]
		v.Level = clampLegacyListLevel(v.Level)
		v.Start = positiveLegacyListStart(v.Start)
		if v.Format == "" {
			v.Format = defaultLegacyListFormat(level)
		}
		v.Text = normalizeLegacyListTemplate(v.Text, level, v.Format)
		out = append(out, v)
	}
	return out
}

func indexLegacyWriteLevels(levels []legacyNumberingWriteLevel) map[int]legacyNumberingWriteLevel {
	out := map[int]legacyNumberingWriteLevel{}
	for _, level := range levels {
		level.Level = clampLegacyListLevel(level.Level)
		if _, exists := out[level.Level]; !exists {
			out[level.Level] = level
		}
	}
	return out
}

func mergeLegacyWriteLevel(levels []legacyNumberingWriteLevel, candidate legacyNumberingWriteLevel) []legacyNumberingWriteLevel {
	candidate.Level = clampLegacyListLevel(candidate.Level)
	for i := range levels {
		if clampLegacyListLevel(levels[i].Level) != candidate.Level {
			continue
		}
		// Keep the definition-level values when present, but fill gaps from
		// observed paragraph numbering.
		if levels[i].Start <= 0 {
			levels[i].Start = candidate.Start
		}
		if levels[i].Format == "" {
			levels[i].Format = candidate.Format
		}
		if levels[i].Text == "" {
			levels[i].Text = candidate.Text
		}
		if levels[i].IndentLeft == 0 {
			levels[i].IndentLeft = candidate.IndentLeft
		}
		if levels[i].RunFormat == nil {
			levels[i].RunFormat = candidate.RunFormat
		}
		return levels
	}
	return append(levels, candidate)
}

func cloneLegacyNumberingOverride(in wordir.LegacyNumberingOverride) wordir.LegacyNumberingOverride {
	out := in
	if in.Start != nil {
		v := *in.Start
		out.Start = &v
	}
	if in.Restart != nil {
		v := *in.Restart
		out.Restart = &v
	}
	if in.RestartOnLevel != nil {
		v := *in.RestartOnLevel
		out.RestartOnLevel = &v
	}
	return out
}

func cloneLegacyRunFormatPtr(in *wordir.RunFormat) *wordir.RunFormat {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
