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
	"strings"
	"unicode"

	presentationir "office/pkg/presentation/ir"
)

// stpaCFRun holds one TextCFException run's character count and resolved
// formatting.  All pointer fields are nil when the corresponding property is
// absent (inherited from a default/master style).
type stpaCFRun struct {
	CharCount        int
	Bold             *bool
	Italic           *bool
	Underline        *bool
	Strike           *bool
	FontSize         int     // hundredths of a point; 0 = inherited
	FontFamily       string  // generic/typeface font family; "" = inherited / unknown
	AnsiFontFamily   string  // ANSI/Latin-specific font family when authored
	EAFontFamily     string  // East-Asian-specific font family when authored
	SymbolFontFamily string  // symbol-specific font family when authored
	Color            string  // sRGB hex (e.g. "FF0000"); "" = inherited
	Superscript      bool    // baseline offset > 0 in TextCFException
	Subscript        bool    // baseline offset < 0
	BaselineShiftPct float64 // authored TextCFException baseline displacement
}

// stpaPFEntry holds the resolved paragraph formatting extracted from one
// TextPFException entry.
type stpaPFEntry struct {
	CharCount     int
	Align         string // "l", "ctr", "r", "just", or "" (inherited)
	TextDirection string // "ltr", "rtl", or "" (inherited)
	HasBullet          bool
	HasBulletSpecified bool // PFMasks.hasBullet: fHasBullet is authoritative, including explicit false
	BulletChar         rune // explicit bullet character; 0 = none / not set
	AutoNum       bool
	AutoNumType   string
	AutoNumStart  int
	BulletColor   string // explicit bullet color as hex RGB (e.g. "FF0000"); "" = inherited
	BulletSize    int    // bullet font size in hundredths of a point (positive); 0 = not set
	BulletSizePct int    // canonical percentage units: 100000 = 100% of first run font size
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
	// Paragraph spacing follows MS-PPT ParaSpacing: non-negative values are percentages
	// of text line height; negative values are absolute master units (1 master unit = 0.01 pt).
	SpaceBefore    int // hundredths of a point; valid when HasSpaceBefore
	HasSpaceBefore bool
	SpaceBeforePct int // thousandths of percent; non-zero when spBefore was positive
	SpaceAfter     int // hundredths of a point; valid when HasSpaceAfter
	HasSpaceAfter  bool
	SpaceAfterPct  int // thousandths of percent; non-zero when spAfter was positive
	// Line spacing follows the same ParaSpacing encoding.
	LineSpacingPct int // thousandths of percent when percentage-based
	LineSpacingPt  int // hundredths of a point when absolute
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
	// A StyleTextPropAtom has existed in more than one on-disk representation.
	// Some compatibility payloads are byte-compatible enough with the newer
	// grammar that a bounds-only parse succeeds while reading CF/PF words at the
	// wrong offsets.  Do not let parser order decide the format: decode both
	// candidates and choose the one whose resulting text semantics are valid.
	spec := parseStyleTextPropAtomSpecWithTheme(rawText, stpa, fontTable, colorScheme)
	legacy := parseStyleTextPropAtomLegacyWithTheme(rawText, stpa, fontTable, colorScheme)
	return chooseStyleTextPropCandidateForText(rawText, spec, legacy)
}

// chooseStyleTextPropCandidate resolves the only intentionally ambiguous part
// of STPA parsing.  It does not sanitize or clamp output.  A candidate that
// decodes values outside the representable PowerPoint text domain is evidence
// that the payload was interpreted with the wrong record grammar, so the other
// complete candidate wins.
func chooseStyleTextPropCandidate(spec, legacy []*presentationir.Paragraph) []*presentationir.Paragraph {
	return chooseStyleTextPropCandidateForText("", spec, legacy)
}

// chooseStyleTextPropCandidateForText selects between the two historical STPA
// encodings using semantic invariants that are independent of a particular
// presentation. A bounds-valid decode is not sufficient: interpreting mask
// words as values can yield technically parseable but impossible text state.
func chooseStyleTextPropCandidateForText(rawText string, spec, legacy []*presentationir.Paragraph) []*presentationir.Paragraph {
	if spec == nil {
		return legacy
	}
	if legacy == nil {
		return spec
	}
	specPenalty := styleTextPropCandidatePenaltyForText(rawText, spec)
	legacyPenalty := styleTextPropCandidatePenaltyForText(rawText, legacy)
	if legacyPenalty < specPenalty {
		return legacy
	}
	return spec
}

func styleTextPropCandidatePenalty(paragraphs []*presentationir.Paragraph) int {
	return styleTextPropCandidatePenaltyForText("", paragraphs)
}

func styleTextPropCandidatePenaltyForText(rawText string, paragraphs []*presentationir.Paragraph) int {
	penalty := 0
	hasLatinText := false
	for _, r := range rawText {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			hasLatinText = true
			break
		}
	}
	for _, para := range paragraphs {
		if para == nil {
			continue
		}
		if para.BulletChar != "" {
			r := []rune(para.BulletChar)
			if len(r) == 1 && ((r[0] < 0x20 && r[0] != '\t') || r[0] == 0xFEFF || r[0] == 0xFFFE) {
				// BOM/non-character values are text-stream artefacts, never authored bullet glyphs.
				penalty += 64
			}
		}
		for _, run := range para.Runs {
			if run == nil {
				continue
			}
			// PowerPoint's authored font-size domain tops out at 400 pt. Values
			// above that are not "large text" here: they are a signature of a
			// CF mask/value word being consumed by the wrong STPA grammar.
			if run.FontSize > 40000 {
				penalty += 32
			}
			// A compatibility misread commonly turns the same grpf word into all
			// four decorations. Penalize the combination, not any individual
			// authored property.
			if run.BoldSet && run.Bold && run.ItalicSet && run.Italic &&
				run.UnderlineSet && run.Underline && run.StrikeSet && run.Strike {
				penalty += 32
			}
			if hasLatinText && (run.FontFamily == "Symbol" || run.FontFamily == "Wingdings") {
				// Symbol fonts are valid for glyph runs, but applying one to an entire ordinary
				// Latin text body is a strong signature that a font-ref word was read at the
				// wrong compatibility offset. Keep this weaker than structural invalidity.
				penalty += 8
			}
		}
	}
	return penalty
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

		paraText := normalizePptSoftLineBreaks(string(displayRunes))
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
		if pf.HasBulletSpecified && !pf.HasBullet {
			para.BulletNone = true
			para.BulletChar = ""
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
			para.LineSpacingPt = pf.LineSpacingPt
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
		paraText := normalizePptSoftLineBreaks(string(displayRunes))
		paraLen := len(pr)
		runs := buildRunsFromCF(paraText, cfRuns, globalPos, globalPos+paraLen)
		globalPos += paraLen
		para := &presentationir.Paragraph{Align: pf.Align, TextDirection: pf.TextDirection, BulletChar: bulletCharString(pf.BulletChar), Runs: runs}
		if pf.HasBulletSpecified && !pf.HasBullet {
			para.BulletNone = true
			para.BulletChar = ""
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
			para.LineSpacingPt = pf.LineSpacingPt
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

// normalizePptSoftLineBreaks preserves PowerPoint soft line breaks inside a
// paragraph. VT/FF are line breaks, not paragraph marks; canonical layout uses
// \n inside an IR run to distinguish them from physical wrapping.
func normalizePptSoftLineBreaks(text string) string {
	text = strings.ReplaceAll(text, "\u000B", "\n")
	text = strings.ReplaceAll(text, "\u000C", "\n")
	return text
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
		segments := []legacyPptCFFontSegment{{Text: p.text}}
		if p.cfIdx >= 0 && p.cfIdx < len(cfRuns) {
			segments = legacyPptSplitCFRunByTypeface(cfRuns[p.cfIdx], p.text)
		}
		for _, segment := range segments {
			irRun := &presentationir.Run{Text: segment.Text}
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
				if segment.FontFamily != "" {
					irRun.FontFamily = segment.FontFamily
				} else if family := legacyPptCFRunFontFamily(cf, segment.Text); family != "" {
					irRun.FontFamily = family
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
				if cf.BaselineShiftPct != 0 {
					irRun.BaselineShiftPct = cf.BaselineShiftPct
				}
			}
			runs = append(runs, irRun)
		}
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

	// PFMasks bits 0-3 determine which BulletFlags bits are valid.  The
	// BulletFlags word can therefore exist without fHasBullet itself being
	// authoritative (for example, when only bulletHasFont is present).
	var bulletFlags uint16
	if masks&0x0F != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		bulletFlags = binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if masks&(1<<0) != 0 {
			entry.HasBulletSpecified = true
			entry.HasBullet = (bulletFlags & 0x01) != 0
		}
	}
	bulletFontValid := masks&(1<<1) != 0 && bulletFlags&(1<<1) != 0
	bulletColorValid := masks&(1<<2) != 0 && bulletFlags&(1<<2) != 0
	bulletSizeValid := masks&(1<<3) != 0 && bulletFlags&(1<<3) != 0

	// MS-PPT 2.9.20 stores the optional bullet payload in structure order,
	// not PFMasks bit order: bulletChar, bulletFontRef, bulletSize, bulletColor.
	// The masks only say which fields exist. Keeping this order is essential when
	// more than one bullet property is present in the same TextPFException.
	if masks&(1<<7) != 0 { // bulletChar
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		ch := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if ch > 0 {
			entry.BulletChar = rune(ch)
		}
	}

	if masks&(1<<4) != 0 { // bulletFontRef
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
				if bulletFontValid && fontTable != nil {
			if name, ok := fontTable[fontRef]; ok {
				entry.BulletFont = name
			}
		}
	}

	if masks&(1<<6) != 0 { // bulletSize
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		sz := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if bulletSizeValid {
			if sz > 0 {
				entry.BulletSizePct = int(sz) * 1000
			} else if sz < 0 {
				entry.BulletSize = int(-sz) * 100
			}
		}
	}

	if masks&(1<<5) != 0 { // bulletColor
		if pos+4 > len(stpa) {
			return entry, pos, false
		}
		r, g, b, extra := stpa[pos], stpa[pos+1], stpa[pos+2], stpa[pos+3]
		pos += 4
		if bulletColorValid {
			if color := resolveLegacyPptColorRef(r, g, b, extra, colorScheme); color != "" {
				entry.BulletColor = color
			}
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
			entry.LineSpacingPt = int(-v)
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
			entry.SpaceBeforePct = int(v) * 1000
		} else if v < 0 {
			entry.SpaceBefore = int(-v)
			entry.HasSpaceBefore = true
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
			entry.SpaceAfterPct = int(v) * 1000
		} else if v < 0 {
			entry.SpaceAfter = int(-v)
			entry.HasSpaceAfter = true
		}
	}

	// bit 15: defaultTabSize -> int16 master units.
	if masks&(1<<15) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		entry.HasDefaultTabSize = true
		if v > 0 {
			entry.DefaultTabSizeEMU = int(v) * 127
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
		// BulletSize has the same signed contract in the legacy fallback:
		// positive values are percentages of the first run font size, while
		// negative values are absolute point sizes. Normalize percentages to
		// the canonical DrawingML-style units used by presentation IR.
		if sz > 0 {
			entry.BulletSizePct = int(sz) * 1000
		} else if sz < 0 {
			entry.BulletSize = int(-sz) * 100
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
		if v > 0 {
			entry.LineSpacingPct = int(v) * 1000
			entry.HasLineSpacing = true
		} else if v < 0 {
			entry.LineSpacingPt = int(-v)
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
			entry.SpaceBeforePct = int(v) * 1000
		} else if v < 0 {
			entry.SpaceBefore = int(-v)
			entry.HasSpaceBefore = true
		}
	}

	if masks&(1<<19) != 0 {
		if pos+2 > len(stpa) {
			return entry, pos, false
		}
		v := int16(binary.LittleEndian.Uint16(stpa[pos : pos+2]))
		pos += 2
		if v > 0 {
			entry.SpaceAfterPct = int(v) * 1000
		} else if v < 0 {
			entry.SpaceAfter = int(-v)
			entry.HasSpaceAfter = true
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
		run.BaselineShiftPct = float64(baseline)
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
		if fontTable != nil {
			if family, ok := fontTable[fontRef]; ok {
				run.EAFontFamily = family
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
		if fontTable != nil {
			if family, ok := fontTable[fontRef]; ok {
				run.AnsiFontFamily = family
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
		if fontTable != nil {
			if family, ok := fontTable[fontRef]; ok {
				run.SymbolFontFamily = family
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
		run.BaselineShiftPct = float64(baseline)
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
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if fontTable != nil {
			run.EAFontFamily = fontTable[fontRef]
		}
	}
	if masks&(1<<15) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if fontTable != nil {
			run.AnsiFontFamily = fontTable[fontRef]
		}
	}
	if masks&(1<<16) != 0 {
		if pos+2 > len(stpa) {
			return run, pos, false
		}
		fontRef := binary.LittleEndian.Uint16(stpa[pos : pos+2])
		pos += 2
		if fontTable != nil {
			run.SymbolFontFamily = fontTable[fontRef]
		}
	}

	return run, pos, true
}

type legacyPptCFFontSegment struct {
	Text       string
	FontFamily string
}

// legacyPptSplitCFRunByTypeface materializes script-specific font ownership
// inside one TextCFRun. A TextCFException can carry ANSI, East-Asian and symbol
// typefaces simultaneously, so one binary CF run is not necessarily one
// physical font-metric run. Neutral characters stay with the surrounding
// strong-script segment and therefore do not create artificial metric breaks.
func legacyPptSplitCFRunByTypeface(run stpaCFRun, text string) []legacyPptCFFontSegment {
	if text == "" {
		return []legacyPptCFFontSegment{{Text: "", FontFamily: legacyPptCFRunFontFamily(run, "")}}
	}
	runes := []rune(text)
	strong := make([]string, len(runes))
	for i, r := range runes {
		strong[i] = legacyPptCFRuneFontFamily(run, r)
	}
	// Leading neutrals belong to the first strong script; later neutrals keep
	// the preceding script. This mirrors shaping ownership without changing text.
	first := ""
	for _, family := range strong {
		if family != "" {
			first = family
			break
		}
	}
	current := first
	if current == "" {
		current = legacyPptCFRunFontFamily(run, text)
	}
	segments := make([]legacyPptCFFontSegment, 0, 2)
	for i, r := range runes {
		family := strong[i]
		if family == "" {
			family = current
		} else {
			current = family
		}
		if len(segments) == 0 || segments[len(segments)-1].FontFamily != family {
			segments = append(segments, legacyPptCFFontSegment{Text: string(r), FontFamily: family})
		} else {
			segments[len(segments)-1].Text += string(r)
		}
	}
	return segments
}

func legacyPptCFRuneFontFamily(run stpaCFRun, r rune) string {
	if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsDigit(r) {
		return ""
	}
	switch {
	case unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana, unicode.Bopomofo):
		if run.EAFontFamily != "" {
			return run.EAFontFamily
		}
	case unicode.In(r, unicode.Sm, unicode.Sc, unicode.Sk, unicode.So) || (r >= 0xE000 && r <= 0xF8FF):
		if run.SymbolFontFamily != "" {
			return run.SymbolFontFamily
		}
	default:
		if run.AnsiFontFamily != "" {
			return run.AnsiFontFamily
		}
	}
	return run.FontFamily
}

// legacyPptCFRunFontFamily selects the script-specific typeface carried by
// TextCFException before falling back to the generic typeface. MS-PPT stores
// ANSI, East-Asian and symbol references independently; collapsing them while
// decoding changes measured text metrics even when the visible characters are
// otherwise correct.
func legacyPptCFRunFontFamily(run stpaCFRun, text string) string {
	class := 0 // 0 unknown/mixed, 1 ANSI, 2 East Asian, 3 symbol
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsDigit(r) {
			continue
		}
		c := 1
		switch {
		case unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana, unicode.Bopomofo):
			c = 2
		case unicode.In(r, unicode.Sm, unicode.Sc, unicode.Sk, unicode.So) || (r >= 0xE000 && r <= 0xF8FF):
			c = 3
		}
		if class != 0 && class != c {
			class = 0
			break
		}
		class = c
	}
	switch class {
	case 1:
		if run.AnsiFontFamily != "" {
			return run.AnsiFontFamily
		}
	case 2:
		if run.EAFontFamily != "" {
			return run.EAFontFamily
		}
	case 3:
		if run.SymbolFontFamily != "" {
			return run.SymbolFontFamily
		}
	}
	if run.FontFamily != "" {
		return run.FontFamily
	}
	if run.AnsiFontFamily != "" {
		return run.AnsiFontFamily
	}
	if run.EAFontFamily != "" {
		return run.EAFontFamily
	}
	return run.SymbolFontFamily
}

func resolveLegacyPptOfficeArtColorRef(raw uint32, colorScheme map[uint8]string) string {
	r, g, b, flags := byte(raw), byte(raw>>8), byte(raw>>16), byte(raw>>24)
	// OfficeArtCOLORREF uses bit 3 of the high byte as fSchemeIndex. In that
	// form red is the scheme-table index and green/blue are zero.
	if flags&0x08 != 0 && g == 0 && b == 0 {
		return colorScheme[r]
	}
	if flags == 0 {
		return fmt.Sprintf("%02X%02X%02X", r, g, b)
	}
	return ""
}

func resolveLegacyPptColorRef(r, g, b, index byte, colorScheme map[uint8]string) string {
	// MS-PPT ColorIndexStruct: index 0xFE means the preceding RGB bytes are
	// the authored sRGB color. Values 0x00..0x07 are scheme indexes; 0xFF is
	// undefined. Do not infer the scheme index from an RGB component.
	switch index {
	case 0xFE:
		return fmt.Sprintf("%02X%02X%02X", r, g, b)
	case 0xFF:
		return ""
	default:
		if len(colorScheme) == 0 {
			return ""
		}
		return colorScheme[index]
	}
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
	// [MS-PPT] 2.13.28 TextAutoNumberSchemeEnum. Keep this mapping at the
	// binary boundary; downstream layout uses DrawingML-compatible names.
	switch nativeID {
	case 0x0000:
		return "alphaLcPeriod"
	case 0x0001:
		return "alphaUcPeriod"
	case 0x0002:
		return "arabicParenR"
	case 0x0003:
		return "arabicPeriod"
	case 0x0004:
		return "romanLcParenBoth"
	case 0x0005:
		return "romanLcParenR"
	case 0x0006:
		return "romanLcPeriod"
	case 0x0007:
		return "romanUcPeriod"
	case 0x0008:
		return "alphaLcParenBoth"
	case 0x0009:
		return "alphaLcParenR"
	case 0x000A:
		return "alphaUcParenBoth"
	case 0x000B:
		return "alphaUcParenR"
	case 0x000C:
		return "arabicParenBoth"
	case 0x000D:
		return "arabicPlain"
	case 0x000E:
		return "romanUcParenBoth"
	case 0x000F:
		return "romanUcParenR"
	default:
		// The remaining MS-PPT schemes (CJK/Thai/Hindi/etc.) are preserved
		// as unknown here until canonical marker generation supports them;
		// never reinterpret them as a different Latin numbering scheme.
		return ""
	}
}

// TextPFException9 holds per-level auto-numbering data from StyleTextProp9Atom.
type textPFException9 struct {
	hasBulletAutoNumberSpecified bool // masks.bulletHasScheme: explicit true/false is authoritative
	hasBulletAutoNumber          bool
	autoNumberScheme             string // IR AutoNumType value (empty if absent/unsupported)
	autoNumberStart              int    // start number (1-based); valid when scheme is present
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
			entry.hasBulletAutoNumberSpecified = true
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

			if scheme := autoNumberSchemeString(schemeID); scheme != "" {
				entry.autoNumberScheme = scheme
				if startNum >= 1 {
					entry.autoNumberStart = startNum
				}
			}
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

	if pp9, ok := pp9Data[level]; ok {
		if pp9.hasBulletAutoNumberSpecified {
			entry.AutoNum = pp9.hasBulletAutoNumber
			if !pp9.hasBulletAutoNumber {
				entry.AutoNumType = ""
				entry.AutoNumStart = 0
				return
			}
		}
		if pp9.hasBulletAutoNumber && pp9.autoNumberScheme != "" {
			entry.AutoNum = true
			entry.AutoNumType = pp9.autoNumberScheme
			entry.AutoNumStart = pp9.autoNumberStart
		}
	}
}
