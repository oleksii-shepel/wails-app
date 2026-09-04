package legacy

// stpa.go — StyleTextPropAtom (0x0FA1) parser for MS-PPT binary format.
//
// The atom immediately follows a TextCharsAtom or TextBytesAtom and carries
// two sequential arrays:
//
//  1. rgParaStyle: one entry per paragraph in the preceding text body;
//     each entry is (cchPara uint16, TextPFException).
//  2. rgRunStyle: entries covering all characters in the text body;
//     each entry is (cchRun uint16, TextCFException).
//
// The total cchPara values sum to the total character count of the raw text
// (including the CR paragraph separators).  The same total applies to cchRun.

import (
	"encoding/binary"
	"fmt"

	presentationir "office/pkg/presentation/ir"
)

// stpaCFRun holds one TextCFException run's character count and resolved
// formatting.  All pointer fields are nil when the corresponding property is
// absent (inherited from a default/master style).
type stpaCFRun struct {
	CharCount   int
	Bold        *bool
	Italic      *bool
	Underline   *bool
	Strike      *bool
	FontSize    int    // hundredths of a point; 0 = inherited
	FontFamily  string // resolved font family; "" = inherited / unknown
	Color       string // sRGB hex (e.g. "FF0000"); "" = inherited
	Superscript bool   // baseline offset > 0 in TextCFException
	Subscript   bool   // baseline offset < 0
}

// stpaPFEntry holds the resolved paragraph formatting extracted from one
// TextPFException entry.
type stpaPFEntry struct {
	CharCount     int
	Align         string // "l", "ctr", "r", "just", or "" (inherited)
	TextDirection string // "ltr", "rtl", or "" (inherited)
	HasBullet     bool
	BulletChar    rune // explicit bullet character; 0 = none / not set
	AutoNum       bool
	AutoNumType   string
	AutoNumStart  int
	BulletColor   string // explicit bullet color as hex RGB (e.g. "FF0000"); "" = inherited
	BulletSize    int    // bullet font size in hundredths of a point (positive); 0 = not set
	BulletSizePct int    // bullet size as % of run when negative bullet-size sentinel was set
	BulletFont    string // resolved bullet font family from fontTable; "" = inherited
	// Right margin and default tab size are STPA paragraph properties at TextPFException
	// mask bits 9 / 15 (MS-PPT §2.9.20). Master units → EMU = value * 127.
	MarginRightEMU    int
	HasMarginRight    bool
	DefaultTabSizeEMU int
	HasDefaultTabSize bool
	// FontAlign is STPA mask bit 16: vertical alignment of glyphs within the line.
	// Values: "t" (top), "ctr" (center), "base" (baseline), "b" (bottom).
	FontAlign string
	// TabStops carries the explicit tab stops from STPA mask bit 20. Each tab is
	// 4 bytes: position (int16, master units) + type (uint16) where type maps to
	// the OOXML alignment vocabulary.
	TabStops []presentationir.TabStop
	// Paragraph spacing.  Positive values are in master units (1 master unit = 0.01 pt,
	// i.e. 1 hundredth of a point).  Negative values are percentages converted below.
	SpaceBefore    int // hundredths of a point; valid when HasSpaceBefore
	HasSpaceBefore bool
	SpaceBeforePct int // thousandths of percent; non-zero when spBefore was negative
	SpaceAfter     int // hundredths of a point; valid when HasSpaceAfter
	HasSpaceAfter  bool
	SpaceAfterPct  int // thousandths of percent; non-zero when spAfter was negative
	// Line spacing.  Only the percentage case (negative lnSpc) is mapped to the IR.
	LineSpacingPct int // thousandths of percent; valid when HasLineSpacing
	HasLineSpacing bool
	// Paragraph indent / margin in EMU.  1 master unit = 127 EMU.
	MarginLeftEMU int // valid when HasMarginLeft
	HasMarginLeft bool
	TextIndentEMU int // valid when HasTextIndent (negative = hanging indent)
	HasTextIndent bool
}

// parseStyleTextPropAtom parses a StyleTextPropAtom payload (stpa) using
// rawText (decoded text from the preceding text atom, with '\r' paragraph
// separators preserved) and returns a slice of formatted paragraphs.
//
// Returns nil on parse failure; callers should fall back to plain-text
// paragraph building in that case.
func parseStyleTextPropAtom(rawText string, stpa []byte) []*presentationir.Paragraph {
	return parseStyleTextPropAtomWithTheme(rawText, stpa, nil, nil)
}

func parseStyleTextPropAtomWithFonts(rawText string, stpa []byte, fontTable map[uint16]string) []*presentationir.Paragraph {
	return parseStyleTextPropAtomWithTheme(rawText, stpa, fontTable, nil)
}

func parseStyleTextPropAtomWithTheme(rawText string, stpa []byte, fontTable map[uint16]string, colorScheme map[uint8]string) []*presentationir.Paragraph {
	if paragraphs := parseStyleTextPropAtomSpecWithTheme(rawText, stpa, fontTable, colorScheme); paragraphs != nil {
		return paragraphs
	}
	return parseStyleTextPropAtomLegacyWithTheme(rawText, stpa, fontTable, colorScheme)
}

func parseStyleTextPropAtomSpecWithTheme(rawText string, stpa []byte, fontTable map[uint16]string, colorScheme map[uint8]string) []*presentationir.Paragraph {
	if len(stpa) == 0 || rawText == "" {
		return nil
	}

	// PPT text bodies always include an implicit terminating paragraph mark that
	// is not stored in the preceding TextBytes/TextChars atom but is counted by
	// TextPFRun/TextCFRun counts.
	paraRunes := splitPptStyleParagraphRunes(rawText)
	if len(paraRunes) == 0 {
		return nil
	}

	// Compute the total character count including the implicit terminating CR.
	totalChars := 0
	for _, pr := range paraRunes {
		totalChars += len(pr)
	}
	pos := 0

	pfRuns := make([]textPFRun, 0, len(paraRunes))
	consumedParaChars := 0
	for consumedParaChars < totalChars {
		if pos+10 > len(stpa) {
			return nil
		}
		count := int(binary.LittleEndian.Uint32(stpa[pos : pos+4]))
		pos += 4
		level := int(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if count <= 0 {
			return nil
		}
		entry, newPos, ok := parsePFExceptionSpecWithTheme(stpa, pos, colorScheme, fontTable)
		if !ok || newPos > len(stpa) {
			return nil
		}
		entry.CharCount = count
		pfRuns = append(pfRuns, textPFRun{count: count, level: level, entry: entry})
		pos = newPos
		consumedParaChars += count
	}
	if consumedParaChars != totalChars {
		return nil
	}

	cfRuns := make([]stpaCFRun, 0, len(paraRunes)*2)
	consumed := 0
	for consumed < totalChars {
		if pos+8 > len(stpa) {
			return nil
		}
		cchRun := int(binary.LittleEndian.Uint32(stpa[pos : pos+4]))
		pos += 4
		if cchRun <= 0 {
			return nil
		}
		run, newPos, ok := parseCFExceptionSpecWithTheme(stpa, pos, fontTable, colorScheme)
		if !ok || newPos > len(stpa) {
			return nil
		}
		run.CharCount = cchRun
		cfRuns = append(cfRuns, run)
		consumed += cchRun
		pos = newPos
	}
	if consumed != totalChars {
		return nil
	}

	// Build presentationir.Paragraph for each paragraph using the PF entries
	// and overlapping CF runs.
	paragraphs := make([]*presentationir.Paragraph, 0, len(paraRunes))
	globalPos := 0 // offset into the flat character stream
	for _, pr := range paraRunes {
		pfRun, ok := pfRunForCharPos(pfRuns, globalPos)
		if !ok {
			return nil
		}
		pf := pfRun.entry

		// Strip the trailing CR from the paragraph text for rendering.
		displayRunes := pr
		if len(displayRunes) > 0 && displayRunes[len(displayRunes)-1] == '\r' {
			displayRunes = displayRunes[:len(displayRunes)-1]
		}

		paraText := string(displayRunes)
		paraLen := len(pr) // includes the trailing CR for char-position accounting

		runs := buildRunsFromCF(paraText, cfRuns, globalPos, globalPos+paraLen)
		globalPos += paraLen

		para := &presentationir.Paragraph{
			Level:         pfRun.level,
			Align:         pf.Align,
			TextDirection: pf.TextDirection,
			BulletChar:    bulletCharString(pf.BulletChar),
			Runs:          runs,
		}
		if pf.AutoNum {
			para.AutoNum = true
			para.AutoNumType = pf.AutoNumType
			para.AutoNumStart = pf.AutoNumStart
		}
		if pf.BulletColor != "" {
			para.BulletColor = pf.BulletColor
		}
		if pf.BulletSize > 0 {
			para.BulletSize = pf.BulletSize
		}
		if pf.HasSpaceBefore {
			para.SpaceBefore = pf.SpaceBefore
			para.HasSpaceBefore = true
		} else if pf.SpaceBeforePct > 0 {
			para.SpaceBeforePct = pf.SpaceBeforePct
			para.HasSpaceBefore = true
		}
		if pf.HasSpaceAfter {
			para.SpaceAfter = pf.SpaceAfter
			para.HasSpaceAfter = true
		} else if pf.SpaceAfterPct > 0 {
			para.SpaceAfterPct = pf.SpaceAfterPct
			para.HasSpaceAfter = true
		}
		if pf.HasLineSpacing {
			para.LineSpacingPct = pf.LineSpacingPct
			para.HasLineSpacing = true
		}
		if pf.HasMarginLeft {
			para.MarginLeftEMU = pf.MarginLeftEMU
			para.HasMarginLeft = true
		}
		if pf.HasTextIndent {
			para.TextIndentEMU = pf.TextIndentEMU
			para.HasTextIndent = true
		}
		if pf.HasMarginRight {
			para.MarginRightEMU = pf.MarginRightEMU
		}
		if pf.HasDefaultTabSize {
			para.DefaultTabSizeEMU = pf.DefaultTabSizeEMU
		}
		if pf.FontAlign != "" {
			para.FontAlign = pf.FontAlign
		}
		if len(pf.TabStops) > 0 {
			para.TabStops = append(para.TabStops, pf.TabStops...)
		}
		if pf.BulletSizePct != 0 {
			para.BulletSizePct = pf.BulletSizePct
		}
		if pf.BulletFont != "" {
			para.BulletFontFamily = pf.BulletFont
		}
		paragraphs = append(paragraphs, para)
	}

	// Drop all-empty results so callers keep the plain-text fallback.
	nonEmpty := false
	for _, p := range paragraphs {
		if len(p.Runs) > 0 {
			nonEmpty = true
			break
		}
	}
	if !nonEmpty {
		return nil
	}
	return paragraphs
}

func parseStyleTextPropAtomLegacyWithTheme(rawText string, stpa []byte, fontTable map[uint16]string, colorScheme map[uint8]string) []*presentationir.Paragraph {
	if len(stpa) == 0 || rawText == "" {
		return nil
	}
	paraRunes := splitPptRawParagraphRunes(rawText)
	if len(paraRunes) == 0 {
		return nil
	}
	totalChars := 0
	for _, pr := range paraRunes {
		totalChars += len(pr)
	}
	nParas := len(paraRunes)
	pos := 0
	pfEntries := make([]stpaPFEntry, 0, nParas)
	for i := 0; i < nParas && pos < len(stpa); i++ {
		if pos+2 > len(stpa) {
			break
		}
		cchPara := int(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		entry, newPos, ok := parsePFExceptionLegacyWithTheme(stpa, pos, colorScheme, fontTable)
		if !ok || newPos > len(stpa) {
			for j := len(pfEntries); j < nParas; j++ {
				pfEntries = append(pfEntries, stpaPFEntry{})
			}
			pos = newPos
			break
		}
		entry.CharCount = cchPara
		pfEntries = append(pfEntries, entry)
		pos = newPos
	}
	for len(pfEntries) < nParas {
		pfEntries = append(pfEntries, stpaPFEntry{})
	}
	cfRuns := make([]stpaCFRun, 0, nParas*2)
	consumed := 0
	for consumed < totalChars && pos+2 <= len(stpa) {
		cchRun := int(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if cchRun <= 0 {
			break
		}
		run, newPos, ok := parseCFExceptionLegacyWithTheme(stpa, pos, fontTable, colorScheme)
		if !ok || newPos > len(stpa) {
			break
		}
		run.CharCount = cchRun
		cfRuns = append(cfRuns, run)
		consumed += cchRun
		pos = newPos
	}
	paragraphs := make([]*presentationir.Paragraph, 0, nParas)
	globalPos := 0
	for i, pr := range paraRunes {
		pf := pfEntries[i]
		displayRunes := pr
		if len(displayRunes) > 0 && displayRunes[len(displayRunes)-1] == '\r' {
			displayRunes = displayRunes[:len(displayRunes)-1]
		}
		paraText := string(displayRunes)
		paraLen := len(pr)
		runs := buildRunsFromCF(paraText, cfRuns, globalPos, globalPos+paraLen)
		globalPos += paraLen
		para := &presentationir.Paragraph{Align: pf.Align, TextDirection: pf.TextDirection, BulletChar: bulletCharString(pf.BulletChar), Runs: runs}
		if pf.BulletColor != "" {
			para.BulletColor = pf.BulletColor
		}
		if pf.BulletSize > 0 {
			para.BulletSize = pf.BulletSize
		}
		if pf.HasSpaceBefore {
			para.SpaceBefore = pf.SpaceBefore
			para.HasSpaceBefore = true
		} else if pf.SpaceBeforePct > 0 {
			para.SpaceBeforePct = pf.SpaceBeforePct
			para.HasSpaceBefore = true
		}
		if pf.HasSpaceAfter {
			para.SpaceAfter = pf.SpaceAfter
			para.HasSpaceAfter = true
		} else if pf.SpaceAfterPct > 0 {
			para.SpaceAfterPct = pf.SpaceAfterPct
			para.HasSpaceAfter = true
		}
		if pf.HasLineSpacing {
			para.LineSpacingPct = pf.LineSpacingPct
			para.HasLineSpacing = true
		}
		if pf.HasMarginLeft {
			para.MarginLeftEMU = pf.MarginLeftEMU
			para.HasMarginLeft = true
		}
		if pf.HasTextIndent {
			para.TextIndentEMU = pf.TextIndentEMU
			para.HasTextIndent = true
		}
		if pf.HasMarginRight {
			para.MarginRightEMU = pf.MarginRightEMU
		}
		if pf.HasDefaultTabSize {
			para.DefaultTabSizeEMU = pf.DefaultTabSizeEMU
		}
		if pf.FontAlign != "" {
			para.FontAlign = pf.FontAlign
		}
		if len(pf.TabStops) > 0 {
			para.TabStops = append(para.TabStops, pf.TabStops...)
		}
		if pf.BulletSizePct != 0 {
			para.BulletSizePct = pf.BulletSizePct
		}
		if pf.BulletFont != "" {
			para.BulletFontFamily = pf.BulletFont
		}
		paragraphs = append(paragraphs, para)
	}
	nonEmpty := false
	for _, p := range paragraphs {
		if len(p.Runs) > 0 {
			nonEmpty = true
			break
		}
	}
	if !nonEmpty {
		return nil
	}
	return paragraphs
}

type textPFRun struct {
	count int
	level int
	entry stpaPFEntry
}

func splitPptStyleParagraphRunes(rawText string) [][]rune {
	paras := splitPptRawParagraphRunes(rawText)
	if len(paras) == 0 {
		return nil
	}
	last := paras[len(paras)-1]
	if len(last) == 0 || last[len(last)-1] != '\r' {
		paras[len(paras)-1] = append(append([]rune(nil), last...), '\r')
	}
	return paras
}

func pfRunForCharPos(runs []textPFRun, charPos int) (textPFRun, bool) {
	start := 0
	for _, run := range runs {
		end := start + run.count
		if charPos >= start && charPos < end {
			return run, true
		}
		start = end
	}
	return textPFRun{}, false
}

// buildRunsFromCF assembles a slice of presentationir.Run for the characters
// in [start, end) of the flat character stream, applying the CF run
// formatting that covers each character.  Characters before the first CF run
// or after the last receive no formatting.
func buildRunsFromCF(paraText string, cfRuns []stpaCFRun, start, end int) []*presentationir.Run {
	if paraText == "" {
		return []*presentationir.Run{{Text: ""}}
	}
	if len(cfRuns) == 0 {
		return []*presentationir.Run{{Text: paraText}}
	}

	// Compute the CF index and offset within that CF run for each character
	// position.  cfPos[i] is the index into cfRuns for global char position i.
	type cfSpan struct {
		startPos int // global inclusive
		endPos   int // global exclusive
		runIdx   int // index into cfRuns (-1 = no run)
	}
	var spans []cfSpan
	cfGlobal := 0
	for idx, cf := range cfRuns {
		spans = append(spans, cfSpan{cfGlobal, cfGlobal + cf.CharCount, idx})
		cfGlobal += cf.CharCount
	}

	// Walk the paragraph's runes and assign each to a CF run.
	type pendingRun struct {
		text  string
		cfIdx int
	}

	paraRunes := []rune(paraText)
	var parts []pendingRun
	charPos := start
	for _, r := range paraRunes {
		cfIdx := -1
		for _, sp := range spans {
			if charPos >= sp.startPos && charPos < sp.endPos {
				cfIdx = sp.runIdx
				break
			}
		}
		// Merge into the previous run if same cfIdx.
		if len(parts) > 0 && parts[len(parts)-1].cfIdx == cfIdx {
			parts[len(parts)-1].text += string(r)
		} else {
			parts = append(parts, pendingRun{text: string(r), cfIdx: cfIdx})
		}
		charPos++
	}

	runs := make([]*presentationir.Run, 0, len(parts))
	for _, p := range parts {
		irRun := &presentationir.Run{Text: p.text}
		if p.cfIdx >= 0 && p.cfIdx < len(cfRuns) {
			cf := cfRuns[p.cfIdx]
			if cf.Bold != nil {
				irRun.Bold = *cf.Bold
				irRun.BoldSet = true
			}
			if cf.Italic != nil {
				irRun.Italic = *cf.Italic
				irRun.ItalicSet = true
			}
			if cf.Underline != nil {
				irRun.Underline = *cf.Underline
				irRun.UnderlineSet = true
			}
			if cf.Strike != nil {
				irRun.Strike = *cf.Strike
				irRun.StrikeSet = true
			}
			if cf.FontSize > 0 {
				irRun.FontSize = cf.FontSize
			}
			if cf.FontFamily != "" {
				irRun.FontFamily = cf.FontFamily
			}
			if cf.Color != "" {
				irRun.Color = cf.Color
			}
			if cf.Superscript {
				irRun.Superscript = true
			}
			if cf.Subscript {
				irRun.Subscript = true
			}
		}
		runs = append(runs, irRun)
	}
	return runs
}

// parsePFException parses a TextPFException starting at stpa[pos].
// Returns the parsed entry, the new position, and ok=true on success.
//
// TextPFException layout (MS-PPT §2.9.174):
//
//	pf1 (2 bytes) + pf2 (2 bytes) = combined 32-bit masks
//
//	pf1 field bits (low 16):
//	  bits 0-3 (hasBullet/bulletHasFont/bulletHasColor/bulletHasSize):
//	            if ANY set → 2-byte bulletFlags word
//	  bit  4  (bulletFont)  → uint16
//	  bit  5  (bulletColor) → 4 bytes
//	  bit  6  (bulletSize)  → int16
//	  bit  7  (bulletChar)  → uint16
//	  bit  8  (leftMargin)  → int32 (4 bytes)
//	  bit  9  (indent)      → int32 (4 bytes)
//	  bit  10 (defaultTabSize) → int16
//	  bit  11 (fontAlign)   → uint16
//	  bits 12-14 (wrapFlags): any set → 2-byte flags
//	  bit  15 (tabStops)    → uint16 count + count*4 bytes
//
//	pf2 field bits (high 16, i.e. uint32 bits 16-31):
//	  bit 16 (textAlignment) → uint16 (0=left,1=ctr,2=right,3=just,4=distr)
//	  bit 17 (lineSpacing)   → int16
//	  bit 18 (spBefore)      → int16
//	  bit 19 (spAfter)       → int16
//	  bits 20+ are additional props not needed for the first pass
func parsePFException(stpa []byte, pos int) (stpaPFEntry, int, bool) {
	return parsePFExceptionLegacyWithTheme(stpa, pos, nil, nil)
}

func parsePFExceptionWithTheme(stpa []byte, pos int, colorScheme map[uint8]string) (stpaPFEntry, int, bool) {
	return parsePFExceptionLegacyWithTheme(stpa, pos, colorScheme, nil)
}

func parsePFExceptionSpecWithTheme(stpa []byte, pos int, colorScheme map[uint8]string, fontTable map[uint16]string) (stpaPFEntry, int, bool) {
	if pos+4 > len(stpa) {
		return stpaPFEntry{}, pos, false
	}
	masks := binary.LittleEndian.Uint32(stpa[pos : pos+4])
	pos += 4

	var entry stpaPFEntry

	// pf1 bits 0-3: bullet-related boolean flags → 2-byte bulletFlags word.
	if masks&0x0F != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		bulletFlags := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		entry.HasBullet = (bulletFlags & 0x01) != 0
	}

	// bit 4: bulletFontRef -> uint16 font index into the document FontCollection.
	if masks&(1<<4) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if fontTable != nil {
			if name, ok := fontTable[fontRef]; ok {
				entry.BulletFont = name
			}
		}
	}

	// bit 5: bulletColor -> 4 bytes (R, G, B, extra)
	if masks&(1<<5) != 0 {
		if pos+4 > len(stpa) {
			return entry, pos, false
		}
		r, g, b, extra := stpa[pos], stpa[pos+1], stpa[pos+2], stpa[pos+3]
		pos += 4
		if color := resolveLegacyPptColorRef(r, g, b, extra, colorScheme); color != "" {
			entry.BulletColor = color
		}
	}

	// bit 6: bulletSize -> int16.
	// Positive values are percentages of the run font size (1..400 typically);
	// negative values are absolute point sizes encoded as -hps.
	if masks&(1<<6) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		sz := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if sz > 0 {
			entry.BulletSizePct = int(sz)
		} else if sz < 0 {
			entry.BulletSize = int(-sz) * 100
		}
	}

	// bit 7: bulletChar -> uint16
	if masks&(1<<7) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		ch := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if ch > 0 {
			entry.BulletChar = rune(ch)
		}
	}

	// bit 8: leftMargin -> int16 master units.
	if masks&(1<<8) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		entry.MarginLeftEMU = int(v) * 127
		entry.HasMarginLeft = true
	}

	// bit 10: indent -> int16 master units.
	if masks&(1<<10) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		entry.TextIndentEMU = int(v) * 127
		entry.HasTextIndent = true
	}

	// bit 11: textAlignment -> uint16
	if masks&(1<<11) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		align := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		entry.Align = pptAlignString(align)
	}

	// bit 12: lineSpacing -> int16.
	if masks&(1<<12) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if v > 0 {
			entry.LineSpacingPct = int(v) * 1000
			entry.HasLineSpacing = true
		} else if v < 0 {
			entry.LineSpacingPct = int(-v) * 1000
			entry.HasLineSpacing = true
		}
	}

	// bit 13: spaceBefore -> int16.
	if masks&(1<<13) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if v > 0 {
			entry.SpaceBefore = int(v)
			entry.HasSpaceBefore = true
		} else if v < 0 {
			entry.SpaceBeforePct = int(-v) * 1000
		}
	}

	// bit 14: spaceAfter -> int16.
	if masks&(1<<14) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if v > 0 {
			entry.SpaceAfter = int(v)
			entry.HasSpaceAfter = true
		} else if v < 0 {
			entry.SpaceAfterPct = int(-v) * 1000
		}
	}

	// bit 15: defaultTabSize -> int16 master units.
	if masks&(1<<15) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if v > 0 {
			entry.DefaultTabSizeEMU = int(v) * 127
			entry.HasDefaultTabSize = true
		}
	}

	// bit 16: fontAlign -> uint16. 0=baseline, 1=top, 2=center, 3=bottom (POI HSLF mapping).
	if masks&(1<<16) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		fa := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		switch fa {
		case 0:
			entry.FontAlign = "base"
		case 1:
			entry.FontAlign = "t"
		case 2:
			entry.FontAlign = "ctr"
		case 3:
			entry.FontAlign = "b"
		}
	}

	// bits 17-19: wrapFlags -> uint16.
	if masks&((1<<17)|(1<<18)|(1<<19)) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		pos += 2
	}

	// bit 20: tabStops -> count + count*4 bytes (each: position int16 master units + type uint16).
	if masks&(1<<20) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		count := int(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if pos+count*4 > len(stpa) {
			return entry, pos, false
		}
		entry.TabStops = make([]presentationir.TabStop, 0, count)
		for i := 0; i < count; i++ {
			posMu := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
			tabType := binary.LittleEndian.Uint16(stpa[pos+2 : pos+4])
			pos += 4
			ts := presentationir.TabStop{Pos: int64(posMu) * 127}
			switch tabType {
			case 0:
				ts.Algn = "l"
			case 1:
				ts.Algn = "ctr"
			case 2:
				ts.Algn = "r"
			case 3:
				ts.Algn = "dec"
			}
			entry.TabStops = append(entry.TabStops, ts)
		}
	}

	// bit 21: textDirection -> uint16.
	if masks&(1<<21) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		direction := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		entry.TextDirection = legacyPptParagraphDirection(direction)
	}

	return entry, pos, true
}

func parsePFExceptionLegacyWithTheme(stpa []byte, pos int, colorScheme map[uint8]string, fontTable map[uint16]string) (stpaPFEntry, int, bool) {
	_ = fontTable // legacy fallback layout doesn't expose a font index slot
	if pos+4 > len(stpa) {
		return stpaPFEntry{}, pos, false
	}
	masks := binary.LittleEndian.Uint32(stpa[pos : pos+4])
	pos += 4

	var entry stpaPFEntry
	if masks&0x0F != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		bulletFlags := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		entry.HasBullet = (bulletFlags & 0x01) != 0
	}
	if masks&(1<<4) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		pos += 2
	}
	if masks&(1<<5) != 0 {
		if pos+4 > len(stpa) {
			return entry, pos, false
		}
		r, g, b, extra := stpa[pos], stpa[pos+1], stpa[pos+2], stpa[pos+3]
		pos += 4
		if color := resolveLegacyPptColorRef(r, g, b, extra, colorScheme); color != "" {
			entry.BulletColor = color
		}
	}
	if masks&(1<<6) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		sz := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if sz > 0 {
			entry.BulletSize = int(sz)
		}
	}
	if masks&(1<<7) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		ch := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if ch > 0 {
			entry.BulletChar = rune(ch)
		}
	}
	if masks&(1<<8) != 0 {
		if pos+4 > len(stpa) {
			return entry, pos, false
		}
		v := int32(binary.LittleEndian.Uint32(stpa[pos : pos+4]))
		pos += 4
		entry.MarginLeftEMU = int(v) * 127
		entry.HasMarginLeft = true
	}
	if masks&(1<<9) != 0 {
		if pos+4 > len(stpa) {
			return entry, pos, false
		}
		v := int32(binary.LittleEndian.Uint32(stpa[pos : pos+4]))
		pos += 4
		entry.TextIndentEMU = int(v) * 127
		entry.HasTextIndent = true
	}
	if masks&(1<<10) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		pos += 2
	}
	if masks&(1<<11) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		pos += 2
	}
	if masks&(7<<12) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		pos += 2
	}
	if masks&(1<<15) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		count := int(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		skip := count * 4
		if pos+skip > len(stpa) {
			return entry, pos, false
		}
		pos += skip
	}
	if masks&(1<<17) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if v < 0 {
			entry.LineSpacingPct = int(-v) * 1000
			entry.HasLineSpacing = true
		}
	}

	if masks&(1<<16) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		align := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		entry.Align = pptAlignString(align)
	}
	if masks&(1<<18) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if v > 0 {
			entry.SpaceBefore = int(v)
			entry.HasSpaceBefore = true
		} else if v < 0 {
			entry.SpaceBeforePct = int(-v) * 1000
		}
	}

	if masks&(1<<19) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if v > 0 {
			entry.SpaceAfter = int(v)
			entry.HasSpaceAfter = true
		} else if v < 0 {
			entry.SpaceAfterPct = int(-v) * 1000
		}
	}

	for bit := uint(20); bit <= 31; bit++ {
		if masks&(1<<bit) != 0 {
			if pos+2 > len(stpa) {
				return entry, pos, false
			}
			if bit == 21 {
				direction := binary.LittleEndian.Uint16(stpa[pos : pos+2])
				entry.TextDirection = legacyPptParagraphDirection(direction)
			}
			pos += 2
		}
	}

	return entry, pos, true
}

func legacyPptParagraphDirection(value uint16) string {
	switch int16(value) {
	case 1:
		return "ltr"
	case 2:
		return "rtl"
	default:
		return ""
	}
}

// parseCFException parses a TextCFException starting at stpa[pos].
// Returns the run formatting, the new position, and ok=true on success.
//
// TextCFException layout (MS-PPT §2.9.41):
//
//	masks (4 bytes, uint32)
//	  bits 0-8: if ANY set → 2-byte grpfBasicAtom:
//	              bit 0 = bold, bit 1 = italic, bit 2 = underline,
//	              bit 3 = shadow, bit 7 = strikeout, bit 8 = emboss
//	  bit 9  (typeface)  → uint16 font index
//	  bit 10 (fontSize)  → uint16 in hundredths of a point
//	  bit 11 (color)     → 4 bytes (R, G, B, extra)
//	  bit 12 (position)  → int16 baseline offset
//	  bit 13 (pp10ext)   → uint32 (4 bytes)
//	  bit 14 (oldEATypeface) → uint16
//	  bit 15 (ansiTypeface)  → uint16
//	  bit 16 (symbolTypeface)→ uint16
func parseCFException(stpa []byte, pos int) (stpaCFRun, int, bool) {
	return parseCFExceptionLegacyWithTheme(stpa, pos, nil, nil)
}

func parseCFExceptionWithFonts(stpa []byte, pos int, fontTable map[uint16]string) (stpaCFRun, int, bool) {
	return parseCFExceptionLegacyWithTheme(stpa, pos, fontTable, nil)
}

func parseCFExceptionWithTheme(stpa []byte, pos int, fontTable map[uint16]string, colorScheme map[uint8]string) (stpaCFRun, int, bool) {
	return parseCFExceptionLegacyWithTheme(stpa, pos, fontTable, colorScheme)
}

func parseCFExceptionSpecWithTheme(stpa []byte, pos int, fontTable map[uint16]string, colorScheme map[uint8]string) (stpaCFRun, int, bool) {
	if pos+4 > len(stpa) {
		return stpaCFRun{}, pos, false
	}
	masks := binary.LittleEndian.Uint32(stpa[pos : pos+4])
	pos += 4

	var run stpaCFRun

	styleMask := uint32((1 << 0) | (1 << 1) | (1 << 2) | (1 << 4) | (1 << 5) | (1 << 7) | (1 << 9) | (0xF << 10))
	if masks&styleMask != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		grpf := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if masks&(1<<0) != 0 {
			b := (grpf & (1 << 0)) != 0
			run.Bold = &b
		}
		if masks&(1<<1) != 0 {
			it := (grpf & (1 << 1)) != 0
			run.Italic = &it
		}
		if masks&(1<<2) != 0 {
			ul := (grpf & (1 << 2)) != 0
			run.Underline = &ul
		}
		// grpfBasicAtom bit 7 = strikethrough (MS-PPT §2.9.18 TextCFException).
		if masks&(1<<7) != 0 {
			st := (grpf & (1 << 7)) != 0
			run.Strike = &st
		}
		// The binary PPT CFStyle structure does not expose strikethrough here;
		// newer round-trip atoms carry additional run metadata separately.
	}

	// bit 16: typeface -> uint16 font index
	if masks&(1<<16) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if fontTable != nil {
			if family, ok := fontTable[fontRef]; ok {
				run.FontFamily = family
			}
		}
	}

	// bit 17: fontSize -> int16 points.
	if masks&(1<<17) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		sz := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if sz > 0 {
			run.FontSize = int(sz) * 100
		}
	}

	// bit 18: color -> 4 bytes (R, G, B, extra)
	if masks&(1<<18) != 0 {
		if pos+4 > len(stpa) {
			return run, pos, false
		}
		r, g, b, extra := stpa[pos], stpa[pos+1], stpa[pos+2], stpa[pos+3]
		pos += 4
		if color := resolveLegacyPptColorRef(r, g, b, extra, colorScheme); color != "" {
			run.Color = color
		}
	}

	// bit 19: position/baseline -> int16 percent. Positive = superscript,
	// negative = subscript, 0 = baseline.
	if masks&(1<<19) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		baseline := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if baseline > 0 {
			run.Superscript = true
		} else if baseline < 0 {
			run.Subscript = true
		}
	}

	// bit 20: pp10ext -> uint32 (skip)
	if masks&(1<<20) != 0 {
		if pos+4 > len(stpa) {
			return run, pos, false
		}
		pos += 4
	}

	// bit 21: oldEATypeface -> uint16.
	if masks&(1<<21) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if run.FontFamily == "" && fontTable != nil {
			if family, ok := fontTable[fontRef]; ok {
				run.FontFamily = family
			}
		}
	}

	// bit 22: ansiTypeface -> uint16.
	if masks&(1<<22) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if run.FontFamily == "" && fontTable != nil {
			if family, ok := fontTable[fontRef]; ok {
				run.FontFamily = family
			}
		}
	}

	// bit 23: symbolTypeface -> uint16.
	if masks&(1<<23) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if run.FontFamily == "" && fontTable != nil {
			if family, ok := fontTable[fontRef]; ok {
				run.FontFamily = family
			}
		}
	}
	if masks&(1<<24) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		pos += 2
	}
	if masks&(1<<25) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		pos += 2
	}
	if masks&(1<<26) != 0 {
		if pos+4 > len(stpa) {
			return run, pos, false
		}
		pos += 4
	}

	return run, pos, true
}

func parseCFExceptionLegacyWithTheme(stpa []byte, pos int, fontTable map[uint16]string, colorScheme map[uint8]string) (stpaCFRun, int, bool) {
	if pos+4 > len(stpa) {
		return stpaCFRun{}, pos, false
	}
	masks := binary.LittleEndian.Uint32(stpa[pos : pos+4])
	pos += 4

	var run stpaCFRun
	if masks&0x1FF != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		grpf := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if masks&(1<<0) != 0 {
			b := (grpf & (1 << 0)) != 0
			run.Bold = &b
		}
		if masks&(1<<1) != 0 {
			it := (grpf & (1 << 1)) != 0
			run.Italic = &it
		}
		if masks&(1<<2) != 0 {
			ul := (grpf & (1 << 2)) != 0
			run.Underline = &ul
		}
		if masks&(1<<7) != 0 {
			st := (grpf & (1 << 7)) != 0
			run.Strike = &st
		}
	}
	if masks&(1<<9) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if fontTable != nil {
			if family, ok := fontTable[fontRef]; ok {
				run.FontFamily = family
			}
		}
	}
	if masks&(1<<10) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		sz := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if sz > 0 {
			run.FontSize = int(sz)
		}
	}
	if masks&(1<<11) != 0 {
		if pos+4 > len(stpa) {
			return run, pos, false
		}
		r, g, b, extra := stpa[pos], stpa[pos+1], stpa[pos+2], stpa[pos+3]
		pos += 4
		if color := resolveLegacyPptColorRef(r, g, b, extra, colorScheme); color != "" {
			run.Color = color
		}
	}
	if masks&(1<<12) != 0 {
		// Position/baseline offset (int16 percent). Same semantics as the spec
		// path above: positive = superscript, negative = subscript.
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		baseline := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if baseline > 0 {
			run.Superscript = true
		} else if baseline < 0 {
			run.Subscript = true
		}
	}
	if masks&(1<<13) != 0 {
		if pos+4 > len(stpa) {
			return run, pos, false
		}
		pos += 4
	}
	if masks&(1<<14) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		pos += 2
	}
	if masks&(1<<15) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		pos += 2
	}
	if masks&(1<<16) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		pos += 2
	}

	return run, pos, true
}

func resolveLegacyPptColorRef(r, g, b, extra byte, colorScheme map[uint8]string) string {
	if extra != 0xFE {
		return fmt.Sprintf("%02X%02X%02X", r, g, b)
	}
	if len(colorScheme) == 0 {
		return ""
	}
	if r != 0 || (g == 0 && b == 0) {
		if color, ok := colorScheme[r]; ok {
			return color
		}
	}
	if g != 0 {
		if color, ok := colorScheme[g]; ok {
			return color
		}
	}
	if b != 0 {
		if color, ok := colorScheme[b]; ok {
			return color
		}
	}
	return ""
}

// splitPptRawParagraphRunes splits rawText (with '\r' paragraph separators)
// into per-paragraph rune slices.  Each rune slice includes the trailing '\r'
// if present so that the character count matches the STPA cchPara values.
func splitPptRawParagraphRunes(rawText string) [][]rune {
	runes := []rune(rawText)
	var result [][]rune
	start := 0
	for i, r := range runes {
		if r == '\r' {
			result = append(result, runes[start:i+1]) // include '\r'
			start = i + 1
		}
	}
	// Last paragraph (may not have a trailing '\r').
	if start < len(runes) {
		result = append(result, runes[start:])
	}
	return result
}

// pptAlignString converts a PPT textAlignment value to a presentation/ir
// Align string.
func pptAlignString(v uint16) string {
	switch v {
	case 0:
		return "l"
	case 1:
		return "ctr"
	case 2:
		return "r"
	case 3, 4:
		return "just"
	default:
		return ""
	}
}

// bulletCharString returns a non-empty string for a valid bullet rune.
func bulletCharString(r rune) string {
	if r == 0 || r == ' ' {
		return ""
	}
	return string(r)
}

// AutoNumberingScheme enum from POI: maps native IDs to IR string values.
// Per TextPFException9.getAutoNumberScheme() in POI HSLF.
func autoNumberSchemeString(nativeID uint16) string {
	// POI AutoNumberingScheme native IDs (from POI Apache source)
	switch nativeID {
	case 1:
		return "arabicPeriod"      // 1., 2., 3., ...
	case 2:
		return "arabicParenR"      // 1), 2), 3), ...
	case 3:
		return "arabicParenBoth"   // (1), (2), (3), ...
	case 4:
		return "arabicPlain"       // 1, 2, 3, ...
	case 5:
		return "romanUcPeriod"     // I., II., III., ...
	case 6:
		return "romanLcPeriod"     // i., ii., iii., ...
	case 7:
		return "romanUcParenR"     // I), II), III), ...
	case 8:
		return "romanLcParenR"     // i), ii), iii), ...
	case 9:
		return "romanUcParenBoth"  // (I), (II), (III), ...
	case 10:
		return "romanLcParenBoth"  // (i), (ii), (iii), ...
	case 11:
		return "alphaUcPeriod"     // A., B., C., ...
	case 12:
		return "alphaLcPeriod"     // a., b., c., ...
	case 13:
		return "alphaUcParenR"     // A), B), C), ...
	case 14:
		return "alphaLcParenR"     // a), b), c), ...
	case 15:
		return "alphaUcParenBoth"  // (A), (B), (C), ...
	case 16:
		return "alphaLcParenBoth"  // (a), (b), (c), ...
	default:
		return "" // unknown scheme
	}
}

// TextPFException9 holds per-level auto-numbering data from StyleTextProp9Atom.
type textPFException9 struct {
	hasBulletAutoNumber bool
	autoNumberScheme    string // IR AutoNumType value (empty if no auto-numbering)
	autoNumberStart     int    // start number (1-based); valid when autoNumberScheme != ""
}

// parseStyleTextProp9Atom parses a StyleTextProp9Atom (0x0FAC) payload,
// returning a map from level (0-8) to TextPFException9 data.
// Returns nil/empty on parse failure or empty payload.
func parseStyleTextProp9Atom(payload []byte) map[int]textPFException9 {
	if len(payload) == 0 {
		return nil
	}

	result := make(map[int]textPFException9)
	pos := 0
	level := 0

	for pos < len(payload) && level <= 8 {
		if pos+4 > len(payload) {
			// Not enough data for mask3/mask4
			break
		}

		mask3 := payload[pos+2]
		mask4 := payload[pos+3]
		pos += 4

		entry := textPFException9{}

		// Check if bulletBlipRef is present (mask3 & 0x80)
		if (mask3 & 0x80) != 0 {
			if pos+2 > len(payload) {
				break
			}
			pos += 2 // skip bulletBlipRef
		}

		// Check if fBulletHasAutoNumber is present (mask4 & 0x02)
		if (mask4 & 0x02) != 0 {
			if pos+2 > len(payload) {
				break
			}
			hasAutoNum := binary.LittleEndian.Uint16(payload[pos : pos+2])
			entry.hasBulletAutoNumber = (hasAutoNum == 1)
			pos += 2
		}

		// Check if autoNumberScheme + autoNumberStart are present (mask4 & 0x01)
		if (mask4 & 0x01) != 0 {
			if pos+4 > len(payload) {
				break
			}
			schemeID := binary.LittleEndian.Uint16(payload[pos : pos+2])
			startNum := int(binary.LittleEndian.Uint16(payload[pos+2 : pos+4]))
			pos += 4

			// Only set scheme if we have auto-numbering enabled
			if entry.hasBulletAutoNumber {
				if scheme := autoNumberSchemeString(schemeID); scheme != "" {
					entry.autoNumberScheme = scheme
					entry.autoNumberStart = startNum
				}
			}
		}

		// Default auto-numbering scheme when hasBulletAutoNumber=true but scheme not specified
		if entry.hasBulletAutoNumber && entry.autoNumberScheme == "" {
			entry.autoNumberScheme = "arabicPeriod"
			entry.autoNumberStart = 1
		}

		result[level] = entry
		level++
	}

	return result
}

// applyTextPFException9ToEntry merges TextPFException9 auto-numbering data
// into an stpaPFEntry for a given level.
func applyTextPFException9ToEntry(entry *stpaPFEntry, level int, pp9Data map[int]textPFException9) {
	if entry == nil || pp9Data == nil {
		return
	}

	if pp9, ok := pp9Data[level]; ok && pp9.autoNumberScheme != "" {
		entry.AutoNum = true
		entry.AutoNumType = pp9.autoNumberScheme
		entry.AutoNumStart = pp9.autoNumberStart
	}
}