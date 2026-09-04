package legacy

import (
	"encoding/binary"
	"fmt"
	"strings"

	wordir "office/pkg/word/ir"
)

type legacyStyleSheet struct {
	DefaultParagraphStyleID string
	ParagraphStyles         map[string]wordir.Style
	CharacterStyles         map[string]wordir.CharacterStyle
	paragraphByIndex        map[int]wordir.Style
	characterByIndex        map[int]wordir.CharacterStyle
	// StructuredPapx is true for Word97+ documents where the binary PAPx
	// reliably encodes the InTable flag, so heuristic table detection should be
	// suppressed. For pre-Word-97 documents the flag is false and heuristic
	// table detection remains active.
	StructuredPapx bool
}

type legacyRawStyle struct {
	Index         int
	STI           int
	Kind          int
	Name          string
	BasedOnIndex  int
	LinkedIndex   int
	RawParaFormat wordir.ParagraphFormat
	RawRunFormat  wordir.RunFormat
	// ParaSprms stores the raw paragraph-property grpprl bytes from the style's LPUpx.
	ParaSprms []byte
	// RunSprms stores the raw character-property grpprl bytes from the style's LPUpx.
	RunSprms []byte
}

func loadLegacyStyleSheet(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) *legacyStyleSheet {
	const stshFibIndex = 1
	if stshFibIndex >= cbRgFcLcb || fibBase+(stshFibIndex+1)*8 > len(wordStream) {
		return nil
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+stshFibIndex*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+stshFibIndex*8+4:]))
	if lcb < 6 || fc < 0 || fc+lcb > len(tableStream) {
		return nil
	}

	stsh := tableStream[fc : fc+lcb]
	if len(stsh) < 6 {
		return nil
	}
	cbStshi := int(binary.LittleEndian.Uint16(stsh[0:2]))
	if cbStshi < 0 || 2+cbStshi > len(stsh) || cbStshi < 4 {
		return nil
	}
	stshi := stsh[2 : 2+cbStshi]
	cstd := int(binary.LittleEndian.Uint16(stshi[0:2]))
	cbSTDBaseInFile := int(binary.LittleEndian.Uint16(stshi[2:4]))
	if cstd <= 0 || cbSTDBaseInFile < 10 {
		return nil
	}

	pos := 2 + cbStshi
	rawStyles := make([]legacyRawStyle, 0, cstd)
	for istd := 0; istd < cstd && pos+2 <= len(stsh); istd++ {
		cbStd := int(binary.LittleEndian.Uint16(stsh[pos : pos+2]))
		pos += 2
		if cbStd == 0 {
			continue
		}
		if cbStd < 0 || pos+cbStd > len(stsh) {
			break
		}
		std := stsh[pos : pos+cbStd]
		pos += cbStd
		if pos&1 != 0 {
			pos++
		}
		if raw, ok := parseLegacySTD(std, istd, cbSTDBaseInFile); ok {
			rawStyles = append(rawStyles, raw)
		}
	}

	if len(rawStyles) == 0 {
		return nil
	}
	ss := buildLegacyStyleSheet(rawStyles)
	if ss != nil {
		ss.StructuredPapx = true
	}
	return ss
}

func parseLegacySTD(std []byte, istd, cbSTDBaseInFile int) (legacyRawStyle, bool) {
	if len(std) < 10 || cbSTDBaseInFile < 10 || cbSTDBaseInFile > len(std) {
		return legacyRawStyle{}, false
	}

	w0 := binary.LittleEndian.Uint16(std[0:2])
	w1 := binary.LittleEndian.Uint16(std[2:4])
	w2 := binary.LittleEndian.Uint16(std[4:6])

	raw := legacyRawStyle{
		Index:        istd,
		STI:          int(w0 & 0x0FFF),
		Kind:         int(w1 & 0x000F),
		BasedOnIndex: int(w1 >> 4),
		LinkedIndex:  -1,
	}
	cupx := int(w2 & 0x000F)
	if raw.BasedOnIndex == 0x0FFF {
		raw.BasedOnIndex = -1
	}
	if cbSTDBaseInFile >= 12 && len(std) >= 12 {
		linkWord := binary.LittleEndian.Uint16(std[10:12])
		raw.LinkedIndex = int(linkWord & 0x0FFF)
		if raw.LinkedIndex == 0 {
			raw.LinkedIndex = -1
		}
	}

	pos := cbSTDBaseInFile
	name, consumed, ok := parseLegacyXstz(std[pos:])
	if !ok {
		return legacyRawStyle{}, false
	}
	raw.Name = name
	pos += consumed

	switch raw.Kind {
	case 1: // paragraph
		if cupx >= 1 && pos < len(std) {
			raw.RawParaFormat, raw.ParaSprms, consumed = parseLegacyLPUpxPapxWithRawForStyle(std[pos:], istd)
			pos += consumed
		}
		if cupx >= 2 && pos < len(std) {
			raw.RawRunFormat, raw.RunSprms, consumed = parseLegacyLPUpxChpxWithRaw(std[pos:])
		}
		return raw, true
	case 2: // character
		if cupx >= 1 && pos < len(std) {
			raw.RawRunFormat, raw.RunSprms, _ = parseLegacyLPUpxChpxWithRaw(std[pos:])
		}
		return raw, true
	case 3: // table
		return raw, true
	default:
		return legacyRawStyle{}, false
	}
}

func parseLegacyXstz(data []byte) (string, int, bool) {
	if len(data) < 4 {
		return "", 0, false
	}
	cch := int(binary.LittleEndian.Uint16(data[0:2]))
	need := 2 + cch*2 + 2
	if cch < 0 || need > len(data) {
		return "", 0, false
	}
	runes := make([]rune, 0, cch)
	for i := 0; i < cch; i++ {
		runes = append(runes, rune(binary.LittleEndian.Uint16(data[2+i*2:])))
	}
	name := string(runes)
	if idx := strings.IndexRune(name, ','); idx >= 0 {
		name = name[:idx]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", 0, false
	}
	return name, need, true
}

func parseLegacyLPUpxPapx(data []byte) (wordir.ParagraphFormat, int) {
	if len(data) < 2 {
		return wordir.ParagraphFormat{}, len(data)
	}
	cbUpx := int(binary.LittleEndian.Uint16(data[0:2]))
	if cbUpx < 0 || 2+cbUpx > len(data) {
		return wordir.ParagraphFormat{}, len(data)
	}
	consumed := 2 + cbUpx
	if consumed&1 != 0 {
		consumed++
	}
	return parsePapxGrpprl(data[2 : 2+cbUpx]), consumed
}

func parseLegacyLPUpxChpx(data []byte) (wordir.RunFormat, int) {
	if len(data) < 2 {
		return wordir.RunFormat{}, len(data)
	}
	cbUpx := int(binary.LittleEndian.Uint16(data[0:2]))
	if cbUpx < 0 || 2+cbUpx > len(data) {
		return wordir.RunFormat{}, len(data)
	}
	consumed := 2 + cbUpx
	if consumed&1 != 0 {
		consumed++
	}
	return parseChpxGrpprl(data[2 : 2+cbUpx]), consumed
}

// parseLegacyLPUpxPapxWithRaw returns the parsed paragraph format, the raw
// grpprl bytes, and the consumed byte count.
func parseLegacyLPUpxPapxWithRaw(data []byte) (wordir.ParagraphFormat, []byte, int) {
	if len(data) < 2 {
		return wordir.ParagraphFormat{}, nil, len(data)
	}
	cbUpx := int(binary.LittleEndian.Uint16(data[0:2]))
	if cbUpx < 0 || 2+cbUpx > len(data) {
		return wordir.ParagraphFormat{}, nil, len(data)
	}
	consumed := 2 + cbUpx
	if consumed&1 != 0 {
		consumed++
	}
	sprms := data[2 : 2+cbUpx]
	return parsePapxGrpprl(sprms), sprms, consumed
}

// parseLegacyLPUpxPapxWithRawForStyle parses the paragraph UPX stored inside a
// Word 97+ STD. The UpxPapx payload starts with a 2-byte style index (istd)
// followed by the paragraph-property grpprl. Feeding that prefix into the SPRM
// decoder shifts the stream and silently drops style-level spacing, indents, and
// paragraph toggles.
//
// Historical synthetic fixtures in this package omitted the prefix, so strip it
// only when it matches the STD index that owns this UPX.
func parseLegacyLPUpxPapxWithRawForStyle(data []byte, istd int) (wordir.ParagraphFormat, []byte, int) {
	if len(data) < 2 {
		return wordir.ParagraphFormat{}, nil, len(data)
	}
	cbUpx := int(binary.LittleEndian.Uint16(data[0:2]))
	if cbUpx < 0 || 2+cbUpx > len(data) {
		return wordir.ParagraphFormat{}, nil, len(data)
	}
	consumed := 2 + cbUpx
	if consumed&1 != 0 {
		consumed++
	}
	payload := data[2 : 2+cbUpx]
	sprms := payload
	if len(payload) >= 2 && int(binary.LittleEndian.Uint16(payload[:2])) == istd {
		sprms = payload[2:]
	}
	cloned := append([]byte(nil), sprms...)
	return parsePapxGrpprl(cloned), cloned, consumed
}

// parseLegacyLPUpxChpxWithRaw returns the parsed run format, the raw grpprl
// bytes, and the consumed byte count.
func parseLegacyLPUpxChpxWithRaw(data []byte) (wordir.RunFormat, []byte, int) {
	if len(data) < 2 {
		return wordir.RunFormat{}, nil, len(data)
	}
	cbUpx := int(binary.LittleEndian.Uint16(data[0:2]))
	if cbUpx < 0 || 2+cbUpx > len(data) {
		return wordir.RunFormat{}, nil, len(data)
	}
	consumed := 2 + cbUpx
	if consumed&1 != 0 {
		consumed++
	}
	sprms := data[2 : 2+cbUpx]
	return parseChpxGrpprl(sprms), sprms, consumed
}

// parseLegacyLPUpxPapxPre97 reads a Word6/95 PAP UPX entry. Unlike the
// Word97+ variant, the grpprl uses the old 1-byte opcode table.
func parseLegacyLPUpxPapxPre97(data []byte) (wordir.ParagraphFormat, int) {
	if len(data) < 2 {
		return wordir.ParagraphFormat{}, len(data)
	}
	cbUpx := int(binary.LittleEndian.Uint16(data[0:2]))
	if cbUpx < 0 || 2+cbUpx > len(data) {
		return wordir.ParagraphFormat{}, len(data)
	}
	consumed := 2 + cbUpx
	if consumed&1 != 0 {
		consumed++
	}
	format, _, _, _ := parsePreWord97PapxGrpprl(data[2 : 2+cbUpx])
	return format, consumed
}

// parseLegacyLPUpxChpxPre97 reads a Word6/95 CHP UPX entry. Unlike the
// Word97+ variant, the grpprl uses the old 1-byte opcode table.
func parseLegacyLPUpxChpxPre97(data []byte, fontNames []string) (wordir.RunFormat, int) {
	if len(data) < 2 {
		return wordir.RunFormat{}, len(data)
	}
	cbUpx := int(binary.LittleEndian.Uint16(data[0:2]))
	if cbUpx < 0 || 2+cbUpx > len(data) {
		return wordir.RunFormat{}, len(data)
	}
	consumed := 2 + cbUpx
	if consumed&1 != 0 {
		consumed++
	}
	return parsePreWord97ChpxGrpprl(data[2:2+cbUpx], fontNames), consumed
}

// parseLegacyPascalStringName parses a Word6/95 style name encoded as a
// length-prefixed single-byte ASCII string: 1-byte cch, cch characters, one
// NUL terminator, with the entire block padded to an even byte count.
// Returns the name, consumed byte count, and ok=true on success.
func parseLegacyPascalStringName(data []byte) (string, int, bool) {
	if len(data) < 2 {
		return "", 0, false
	}
	cch := int(data[0])
	need := 1 + cch + 1 // cch byte + chars + NUL
	if need > len(data) {
		return "", 0, false
	}
	name := string(data[1 : 1+cch])
	if idx := strings.IndexByte(name, ','); idx >= 0 {
		name = name[:idx]
	}
	name = strings.TrimSpace(name)
	// Pad to even byte count (Word6/95 aligns name blocks to 2-byte boundaries).
	consumed := need
	if consumed&1 != 0 {
		consumed++
	}
	if name == "" {
		return "", 0, false
	}
	return name, consumed, true
}

// parseLegacySTDPre97 parses a Word6/95 STD (style descriptor) entry.
// Word6/95 uses cbSTDBaseInFile=8 (4 words; no grfstd field present in
// Word97+) and stores style names as Pascal strings (parseLegacyPascalStringName)
// rather than the Unicode XSTZ used by Word97+. The linked-style word is
// absent when cbSTDBaseInFile < 12.
// fontNames is the document font table (may be nil); it is used to resolve
// font-index opcodes in the CHPx UPX entries.
func parseLegacySTDPre97(std []byte, istd, cbSTDBaseInFile int, fontNames []string) (legacyRawStyle, bool) {
	if cbSTDBaseInFile < 8 || cbSTDBaseInFile > len(std) {
		return legacyRawStyle{}, false
	}
	w0 := binary.LittleEndian.Uint16(std[0:2])
	w1 := binary.LittleEndian.Uint16(std[2:4])
	w2 := binary.LittleEndian.Uint16(std[4:6])

	raw := legacyRawStyle{
		Index:        istd,
		STI:          int(w0 & 0x0FFF),
		Kind:         int(w1 & 0x000F),
		BasedOnIndex: int(w1 >> 4),
		LinkedIndex:  -1,
	}
	cupx := int(w2 & 0x000F)
	if raw.BasedOnIndex == 0x0FFF {
		raw.BasedOnIndex = -1
	}

	pos := cbSTDBaseInFile
	name, consumed, ok := parseLegacyPascalStringName(std[pos:])
	if !ok {
		return legacyRawStyle{}, false
	}
	raw.Name = name
	pos += consumed

	var convConsumed int
	switch raw.Kind {
	case 1: // paragraph
		if cupx >= 1 && pos < len(std) {
			raw.RawParaFormat, convConsumed = parseLegacyLPUpxPapxPre97(std[pos:])
			pos += convConsumed
		}
		if cupx >= 2 && pos < len(std) {
			raw.RawRunFormat, _ = parseLegacyLPUpxChpxPre97(std[pos:], fontNames)
		}
		return raw, true
	case 2: // character
		if cupx >= 1 && pos < len(std) {
			raw.RawRunFormat, _ = parseLegacyLPUpxChpxPre97(std[pos:], fontNames)
		}
		return raw, true
	case 3: // table
		return raw, true
	default:
		return legacyRawStyle{}, false
	}
}

// loadPreWord97StyleSheet decodes the Word6/95 style table (STSH) from the
// secondary span of the WordDocument stream. Pre-Word-97 documents do not have
// a separate 0Table/1Table stream; the STSH lives in the WordDocument stream
// at SecondaryStart. The STSH uses cbSTDBaseInFile=8 (no grfstd word) and
// Pascal-style (single-byte, length-prefixed) name strings.
func loadPreWord97StyleSheet(wordStream []byte, span PreWord97BodySpan) *legacyStyleSheet {
	start := span.SecondaryStart
	size := span.SecondarySize
	if start <= 0 || size < 6 || start+size > len(wordStream) {
		return nil
	}
	stsh := wordStream[start : start+size]

	// First 2 bytes are cbStshi (size of the stshi header, not counting itself).
	if len(stsh) < 2 {
		return nil
	}
	cbStshi := int(binary.LittleEndian.Uint16(stsh[0:2]))
	if cbStshi < 4 || 2+cbStshi > len(stsh) {
		return nil
	}
	stshi := stsh[2 : 2+cbStshi]
	cstd := int(binary.LittleEndian.Uint16(stshi[0:2]))
	cbSTDBaseInFile := int(binary.LittleEndian.Uint16(stshi[2:4]))
	if cstd <= 0 || cbSTDBaseInFile < 8 || cbSTDBaseInFile > 20 {
		return nil
	}

	// Load font names so that CHPx UPX font-index opcodes can be resolved during
	// style parsing.  Font entries are in the WordDocument stream at FontTableOffset.
	var fontNames []string
	if fonts := LoadPreWord97FontTable(wordStream, span.FontTableOffset, span.FontTableSize); len(fonts) > 0 {
		fontNames = make([]string, len(fonts))
		for i, f := range fonts {
			fontNames[i] = f.Name
		}
	}

	pos := 2 + cbStshi
	rawStyles := make([]legacyRawStyle, 0, cstd)
	for istd := 0; istd < cstd && pos+2 <= len(stsh); istd++ {
		cbStd := int(binary.LittleEndian.Uint16(stsh[pos : pos+2]))
		pos += 2
		if cbStd == 0 {
			continue
		}
		if pos+cbStd > len(stsh) {
			break
		}
		std := stsh[pos : pos+cbStd]
		pos += cbStd
		if raw, ok := parseLegacySTDPre97(std, istd, cbSTDBaseInFile, fontNames); ok {
			rawStyles = append(rawStyles, raw)
		}
	}
	if len(rawStyles) == 0 {
		return nil
	}
	return buildLegacyStyleSheet(rawStyles)
}

func buildLegacyStyleSheet(rawStyles []legacyRawStyle) *legacyStyleSheet {
	paragraphByIndex := make(map[int]wordir.Style)
	charByIndex := make(map[int]wordir.CharacterStyle)

	for _, raw := range rawStyles {
		switch raw.Kind {
		case 1:
			id := legacyParagraphStyleID(raw)
			entry := wordir.Style{
				ID:           id,
				Name:         raw.Name,
				RawFormat:    raw.RawParaFormat,
				RawRunFormat: raw.RawRunFormat,
				Source: &wordir.StyleSource{
					RawSprms: raw.ParaSprms,
					RunSprms: raw.RunSprms,
				},
			}
			paragraphByIndex[raw.Index] = entry
		case 2:
			id := legacyCharacterStyleID(raw)
			entry := wordir.CharacterStyle{
				ID:           id,
				Name:         raw.Name,
				RawRunFormat: raw.RawRunFormat,
				Source: &wordir.StyleSource{
					RunSprms: raw.RunSprms,
				},
			}
			charByIndex[raw.Index] = entry
		}
	}

	for _, raw := range rawStyles {
		switch raw.Kind {
		case 1:
			entry, ok := paragraphByIndex[raw.Index]
			if !ok {
				continue
			}
			if base, ok := paragraphByIndex[raw.BasedOnIndex]; ok {
				entry.BasedOn = base.ID
			}
			if link, ok := charByIndex[raw.LinkedIndex]; ok {
				entry.LinkedCharID = link.ID
			}
			paragraphByIndex[raw.Index] = entry
		case 2:
			entry, ok := charByIndex[raw.Index]
			if !ok {
				continue
			}
			if base, ok := charByIndex[raw.BasedOnIndex]; ok {
				entry.BasedOn = base.ID
			}
			charByIndex[raw.Index] = entry
		}
	}

	stylesByID := make(map[string]wordir.Style, len(paragraphByIndex))
	for _, style := range paragraphByIndex {
		stylesByID[style.ID] = style
	}
	resolveLegacyStyleInheritance(stylesByID)
	resolveLegacyStyleFormats(stylesByID)

	charStylesByID := make(map[string]wordir.CharacterStyle, len(charByIndex))
	for _, style := range charByIndex {
		charStylesByID[style.ID] = style
	}
	resolveLegacyCharStyleInheritance(charStylesByID)
	resolveLegacyCharStyleFormats(charStylesByID)

	for index, style := range paragraphByIndex {
		if resolved, ok := stylesByID[style.ID]; ok {
			paragraphByIndex[index] = resolved
		}
	}

	defaultParagraphStyleID := ""
	for _, raw := range rawStyles {
		if raw.Kind != 1 {
			continue
		}
		if raw.STI == 0 {
			if style, ok := paragraphByIndex[raw.Index]; ok {
				defaultParagraphStyleID = style.ID
			}
			break
		}
	}

	return &legacyStyleSheet{
		DefaultParagraphStyleID: defaultParagraphStyleID,
		ParagraphStyles:         stylesByID,
		CharacterStyles:         charStylesByID,
		paragraphByIndex:        paragraphByIndex,
		characterByIndex:        charByIndex,
	}
}

func legacyParagraphStyleID(raw legacyRawStyle) string {
	if id := legacyBuiltInParagraphStyleID(raw.STI); id != "" {
		return id
	}
	if raw.Name != "" {
		return raw.Name
	}
	return fmt.Sprintf("legacy-paragraph-style-%d", raw.Index)
}

func legacyCharacterStyleID(raw legacyRawStyle) string {
	if id := legacyBuiltInCharacterStyleID(raw.STI); id != "" {
		return id
	}
	if raw.Name != "" {
		return raw.Name
	}
	return fmt.Sprintf("legacy-character-style-%d", raw.Index)
}

func legacyBuiltInParagraphStyleID(sti int) string {
	switch sti {
	case 0:
		return "Normal"
	case 1:
		return "Heading1"
	case 2:
		return "Heading2"
	case 3:
		return "Heading3"
	case 4:
		return "Heading4"
	case 5:
		return "Heading5"
	case 6:
		return "Heading6"
	case 7:
		return "Heading7"
	case 8:
		return "Heading8"
	case 9:
		return "Heading9"
	case 105:
		return "NoList"
	case 107:
		return "NoSpacing"
	default:
		return ""
	}
}

func legacyBuiltInCharacterStyleID(sti int) string {
	switch sti {
	case 65:
		return "DefaultParagraphFont"
	default:
		return ""
	}
}

func resolveLegacyStyleInheritance(index map[string]wordir.Style) {
	for id, style := range index {
		style.StylePath = buildLegacyStylePath(index, id)
		index[id] = style
	}
}

func resolveLegacyStyleFormats(index map[string]wordir.Style) {
	for id, style := range index {
		paraFormat := legacyCloneParagraphFormat(wordir.ParagraphFormat{})
		runFmt := legacyCloneRunFormat(wordir.RunFormat{})
		for i := len(style.StylePath) - 1; i >= 0; i-- {
			current, ok := index[style.StylePath[i]]
			if !ok {
				continue
			}
			paraFormat = legacyMergeParagraphFormat(paraFormat, current.RawFormat)
			runFmt = legacyMergeRunFormat(runFmt, current.RawRunFormat)
		}
		style.Format = paraFormat
		style.RunFormat = runFmt
		index[id] = style
	}
}

func buildLegacyStylePath(index map[string]wordir.Style, id string) []string {
	if id == "" {
		return nil
	}
	path := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	current := id
	for current != "" {
		if _, ok := seen[current]; ok {
			break
		}
		seen[current] = struct{}{}
		path = append(path, current)
		style, ok := index[current]
		if !ok || style.BasedOn == "" {
			break
		}
		current = style.BasedOn
	}
	return path
}

func resolveLegacyCharStyleInheritance(index map[string]wordir.CharacterStyle) {
	for id, style := range index {
		style.StylePath = buildLegacyCharStylePath(index, id)
		index[id] = style
	}
}

func buildLegacyCharStylePath(index map[string]wordir.CharacterStyle, id string) []string {
	if id == "" {
		return nil
	}
	path := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	current := id
	for current != "" {
		if _, ok := seen[current]; ok {
			break
		}
		seen[current] = struct{}{}
		path = append(path, current)
		style, ok := index[current]
		if !ok || style.BasedOn == "" {
			break
		}
		current = style.BasedOn
	}
	return path
}

func resolveLegacyCharStyleFormats(index map[string]wordir.CharacterStyle) {
	for id, style := range index {
		runFmt := legacyCloneRunFormat(wordir.RunFormat{})
		for i := len(style.StylePath) - 1; i >= 0; i-- {
			current, ok := index[style.StylePath[i]]
			if !ok {
				continue
			}
			runFmt = legacyMergeRunFormat(runFmt, current.RawRunFormat)
		}
		style.Format = runFmt
		index[id] = style
	}
}

func legacyMergeRunFormat(base, overlay wordir.RunFormat) wordir.RunFormat {
	r := legacyCloneRunFormat(base)
	if overlay.Bold != nil {
		r.Bold = legacyBoolPtr(*overlay.Bold)
	}
	if overlay.Italic != nil {
		r.Italic = legacyBoolPtr(*overlay.Italic)
	}
	if overlay.Caps != nil {
		r.Caps = legacyBoolPtr(*overlay.Caps)
	}
	if overlay.SmallCaps != nil {
		r.SmallCaps = legacyBoolPtr(*overlay.SmallCaps)
	}
	if overlay.Strike != nil {
		r.Strike = legacyBoolPtr(*overlay.Strike)
	}
	if overlay.Underline != "" {
		r.Underline = overlay.Underline
	}
	if overlay.UnderlineColor != "" {
		r.UnderlineColor = overlay.UnderlineColor
	}
	if overlay.Color != "" {
		r.Color = overlay.Color
	}
	if overlay.Background != "" {
		r.Background = overlay.Background
	}
	if overlay.FontSize != nil {
		r.FontSize = legacyIntPtr(*overlay.FontSize)
	}
	if overlay.FontASCII != "" {
		r.FontASCII = overlay.FontASCII
	}
	if overlay.FontHAnsi != "" {
		r.FontHAnsi = overlay.FontHAnsi
	}
	if overlay.FontEastAsia != "" {
		r.FontEastAsia = overlay.FontEastAsia
	}
	if overlay.FontCS != "" {
		r.FontCS = overlay.FontCS
	}
	if overlay.Highlight != "" {
		r.Highlight = overlay.Highlight
	}
	if overlay.VertAlignVal != "" {
		r.VertAlignVal = overlay.VertAlignVal
	}
	if overlay.Vanish != nil {
		r.Vanish = legacyBoolPtr(*overlay.Vanish)
	}
	if overlay.Lang != "" {
		r.Lang = overlay.Lang
	}
	if overlay.CharacterSpacing != nil {
		r.CharacterSpacing = legacyIntPtr(*overlay.CharacterSpacing)
	}
	if overlay.RTL != nil {
		r.RTL = legacyBoolPtr(*overlay.RTL)
	}
	if overlay.Outline != nil {
		r.Outline = legacyBoolPtr(*overlay.Outline)
	}
	if overlay.Shadow != nil {
		r.Shadow = legacyBoolPtr(*overlay.Shadow)
	}
	if overlay.Emboss != nil {
		r.Emboss = legacyBoolPtr(*overlay.Emboss)
	}
	if overlay.Imprint != nil {
		r.Imprint = legacyBoolPtr(*overlay.Imprint)
	}
	if overlay.EmphasisMark != "" {
		r.EmphasisMark = overlay.EmphasisMark
	}
	return r
}

func legacyCloneRunFormat(format wordir.RunFormat) wordir.RunFormat {
	cloned := format
	if format.Bold != nil {
		cloned.Bold = legacyBoolPtr(*format.Bold)
	}
	if format.Italic != nil {
		cloned.Italic = legacyBoolPtr(*format.Italic)
	}
	if format.Caps != nil {
		cloned.Caps = legacyBoolPtr(*format.Caps)
	}
	if format.SmallCaps != nil {
		cloned.SmallCaps = legacyBoolPtr(*format.SmallCaps)
	}
	if format.Strike != nil {
		cloned.Strike = legacyBoolPtr(*format.Strike)
	}
	if format.FontSize != nil {
		cloned.FontSize = legacyIntPtr(*format.FontSize)
	}
	if format.Vanish != nil {
		cloned.Vanish = legacyBoolPtr(*format.Vanish)
	}
	if format.CharacterSpacing != nil {
		cloned.CharacterSpacing = legacyIntPtr(*format.CharacterSpacing)
	}
	if format.RTL != nil {
		cloned.RTL = legacyBoolPtr(*format.RTL)
	}
	if format.Outline != nil {
		cloned.Outline = legacyBoolPtr(*format.Outline)
	}
	if format.Shadow != nil {
		cloned.Shadow = legacyBoolPtr(*format.Shadow)
	}
	if format.Emboss != nil {
		cloned.Emboss = legacyBoolPtr(*format.Emboss)
	}
	if format.Imprint != nil {
		cloned.Imprint = legacyBoolPtr(*format.Imprint)
	}
	return cloned
}

func legacyMergeParagraphFormat(base, overlay wordir.ParagraphFormat) wordir.ParagraphFormat {
	result := legacyCloneParagraphFormat(base)
	if overlay.Alignment != "" {
		result.Alignment = overlay.Alignment
	}
	if overlay.Shading != "" {
		result.Shading = overlay.Shading
	}
	if overlay.SpacingBefore != nil {
		result.SpacingBefore = legacyIntPtr(*overlay.SpacingBefore)
	}
	if overlay.SpacingAfter != nil {
		result.SpacingAfter = legacyIntPtr(*overlay.SpacingAfter)
	}
	if overlay.SpacingBeforeLines != nil {
		result.SpacingBeforeLines = legacyIntPtr(*overlay.SpacingBeforeLines)
	}
	if overlay.SpacingAfterLines != nil {
		result.SpacingAfterLines = legacyIntPtr(*overlay.SpacingAfterLines)
	}
	if overlay.LineSpacing != nil {
		result.LineSpacing = legacyIntPtr(*overlay.LineSpacing)
	}
	if overlay.LineRule != "" {
		result.LineRule = overlay.LineRule
	}
	if overlay.IndentLeft != nil {
		result.IndentLeft = legacyIntPtr(*overlay.IndentLeft)
	}
	if overlay.IndentRight != nil {
		result.IndentRight = legacyIntPtr(*overlay.IndentRight)
	}
	if overlay.IndentFirstLine != nil {
		result.IndentFirstLine = legacyIntPtr(*overlay.IndentFirstLine)
	}
	if overlay.IndentHanging != nil {
		result.IndentHanging = legacyIntPtr(*overlay.IndentHanging)
	}
	if overlay.ContextualSpacing {
		result.ContextualSpacing = true
	}
	if overlay.KeepNext {
		result.KeepNext = true
	}
	if overlay.KeepLines {
		result.KeepLines = true
	}
	if overlay.WidowControl {
		result.WidowControl = true
	}
	if overlay.PageBreakBefore {
		result.PageBreakBefore = true
	}
	if overlay.SnapToGrid {
		result.SnapToGrid = true
	}
	if overlay.Bidi {
		result.Bidi = true
	}
	if len(overlay.TabStops) > 0 {
		result.TabStops = cloneLegacyTabStops(overlay.TabStops)
	}
	if overlay.SpacingBeforeAuto {
		result.SpacingBeforeAuto = true
		result.SpacingBefore = nil
		result.SpacingBeforeLines = nil
	}
	if overlay.SpacingAfterAuto {
		result.SpacingAfterAuto = true
		result.SpacingAfter = nil
		result.SpacingAfterLines = nil
	}
	if overlay.Borders != nil {
		result.Borders = overlay.Borders
	}
	if overlay.InTable {
		result.InTable = true
	}
	if overlay.IsTableTrailingParagraph {
		result.IsTableTrailingParagraph = true
	}
	if overlay.IsTableCellTrailingParagraph {
		result.IsTableCellTrailingParagraph = true
	}
	if overlay.BinaryListLevel != nil {
		result.BinaryListLevel = legacyIntPtr(*overlay.BinaryListLevel)
	}
	if overlay.BinaryListOverride != nil {
		result.BinaryListOverride = legacyIntPtr(*overlay.BinaryListOverride)
	}
	if overlay.BinaryListFormat != nil {
		result.BinaryListFormat = legacyIntPtr(*overlay.BinaryListFormat)
	}
	if overlay.BinaryStyleIndex != nil {
		result.BinaryStyleIndex = legacyIntPtr(*overlay.BinaryStyleIndex)
	}
	if overlay.OutlineLevel != nil {
		result.OutlineLevel = legacyIntPtr(*overlay.OutlineLevel)
	}
	if overlay.BinaryFrame != nil {
		result.BinaryFrame = legacyMergeFrameProps(result.BinaryFrame, overlay.BinaryFrame)
	}
	return result
}

func legacyCloneParagraphFormat(format wordir.ParagraphFormat) wordir.ParagraphFormat {
	cloned := format
	if format.SpacingBefore != nil {
		cloned.SpacingBefore = legacyIntPtr(*format.SpacingBefore)
	}
	if format.SpacingAfter != nil {
		cloned.SpacingAfter = legacyIntPtr(*format.SpacingAfter)
	}
	if format.SpacingBeforeLines != nil {
		cloned.SpacingBeforeLines = legacyIntPtr(*format.SpacingBeforeLines)
	}
	if format.SpacingAfterLines != nil {
		cloned.SpacingAfterLines = legacyIntPtr(*format.SpacingAfterLines)
	}
	if format.LineSpacing != nil {
		cloned.LineSpacing = legacyIntPtr(*format.LineSpacing)
	}
	if format.IndentLeft != nil {
		cloned.IndentLeft = legacyIntPtr(*format.IndentLeft)
	}
	if format.IndentRight != nil {
		cloned.IndentRight = legacyIntPtr(*format.IndentRight)
	}
	if format.IndentFirstLine != nil {
		cloned.IndentFirstLine = legacyIntPtr(*format.IndentFirstLine)
	}
	if format.IndentHanging != nil {
		cloned.IndentHanging = legacyIntPtr(*format.IndentHanging)
	}
	if format.BinaryListLevel != nil {
		cloned.BinaryListLevel = legacyIntPtr(*format.BinaryListLevel)
	}
	if format.BinaryListOverride != nil {
		cloned.BinaryListOverride = legacyIntPtr(*format.BinaryListOverride)
	}
	if format.BinaryListFormat != nil {
		cloned.BinaryListFormat = legacyIntPtr(*format.BinaryListFormat)
	}
	if format.BinaryStyleIndex != nil {
		cloned.BinaryStyleIndex = legacyIntPtr(*format.BinaryStyleIndex)
	}
	if format.OutlineLevel != nil {
		cloned.OutlineLevel = legacyIntPtr(*format.OutlineLevel)
	}
	if format.BinaryFrame != nil {
		cloned.BinaryFrame = legacyCloneFrameProps(format.BinaryFrame)
	}
	if len(format.TabStops) > 0 {
		cloned.TabStops = cloneLegacyTabStops(format.TabStops)
	}
	return cloned
}

func legacyCloneFrameProps(frame *wordir.FrameProps) *wordir.FrameProps {
	if frame == nil {
		return nil
	}
	cloned := *frame
	return &cloned
}

func legacyMergeFrameProps(base, overlay *wordir.FrameProps) *wordir.FrameProps {
	if base == nil {
		return legacyCloneFrameProps(overlay)
	}
	if overlay == nil {
		return legacyCloneFrameProps(base)
	}
	result := *base
	if overlay.W != 0 {
		result.W = overlay.W
	}
	if overlay.H != 0 {
		result.H = overlay.H
	}
	if overlay.X != 0 {
		result.X = overlay.X
	}
	if overlay.Y != 0 {
		result.Y = overlay.Y
	}
	if overlay.XAlign != "" {
		result.XAlign = overlay.XAlign
	}
	if overlay.YAlign != "" {
		result.YAlign = overlay.YAlign
	}
	if overlay.HAnchor != "" {
		result.HAnchor = overlay.HAnchor
	}
	if overlay.VAnchor != "" {
		result.VAnchor = overlay.VAnchor
	}
	if overlay.WrapType != "" {
		result.WrapType = overlay.WrapType
	}
	if overlay.HSpace != 0 {
		result.HSpace = overlay.HSpace
	}
	if overlay.VSpace != 0 {
		result.VSpace = overlay.VSpace
	}
	if overlay.DropCap != "" {
		result.DropCap = overlay.DropCap
	}
	if overlay.Lines != 0 {
		result.Lines = overlay.Lines
	}
	if overlay.TextDirection != "" {
		result.TextDirection = overlay.TextDirection
	}
	if overlay.AnchorLock {
		result.AnchorLock = overlay.AnchorLock
	}
	return &result
}

func legacyBoolPtr(value bool) *bool {
	v := value
	return &v
}

func legacyIntPtr(value int) *int {
	v := value
	return &v
}
