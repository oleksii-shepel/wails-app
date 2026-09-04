package legacy

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

type legacyHeaderFooterData struct {
	Headers                    map[string]wordir.Story
	Footers                    map[string]wordir.Story
	SectionHeaders             [][]wordir.Reference
	SectionFooters             [][]wordir.Reference
	FootnoteSeparator          wordir.Story
	FootnoteContinuationSep    wordir.Story
	FootnoteContinuationNotice wordir.Story
	EndnoteSeparator           wordir.Story
	EndnoteContinuationSep     wordir.Story
	EndnoteContinuationNotice  wordir.Story
}

// loadDocHeadersFooters extracts header/footer stories from the header
// document (ccpHdr). It prefers PlcfHdd when present, falling back to the
// earlier line-splitting heuristic if the story PLC is absent or malformed.
// For pre-Word-97 documents (signature 0xA5DC / 0xA5DB) it dispatches to
// loadPreWord97HeaderFooterStories instead.
func loadDocHeadersFooters(wordStream []byte, f *cfb.File, sectionCount int) *legacyHeaderFooterData {
	if len(wordStream) < 34 {
		return nil
	}
	sig16 := binary.LittleEndian.Uint16(wordStream[0:2])
	if sig16 == 0xA5DC || sig16 == 0xA5DB {
		return loadPreWord97HeaderFooterStories(wordStream)
	}
	if sig16 != 0xA5EC {
		return nil
	}
	if binary.LittleEndian.Uint16(wordStream[2:4]) < 0x00C1 {
		return nil
	}
	flags := binary.LittleEndian.Uint16(wordStream[10:12])
	tableStreamName := "0Table"
	if flags&(1<<9) != 0 {
		tableStreamName = "1Table"
	}
	tableStream, ok := f.Stream(tableStreamName)
	if !ok || len(tableStream) == 0 {
		return nil
	}

	// Navigate past FibBase, FibRgW97 to reach FibRgLw97.
	pos := 32
	if pos+2 > len(wordStream) {
		return nil
	}
	csw := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	lw97Base := pos + 2 + csw*2 + 2
	if lw97Base+24 > len(wordStream) {
		return nil
	}
	ccpText := int(binary.LittleEndian.Uint32(wordStream[lw97Base+12:]))
	ccpFtn := int(binary.LittleEndian.Uint32(wordStream[lw97Base+16:]))
	ccpHdr := int(binary.LittleEndian.Uint32(wordStream[lw97Base+20:]))
	if ccpHdr <= 0 {
		return nil
	}

	// Navigate past FibRgLw97 to FibRgFcLcb97.
	cslw := int(binary.LittleEndian.Uint16(wordStream[pos+2+csw*2:]))
	pos = pos + 2 + csw*2 + 2 + cslw*4
	if pos+2 > len(wordStream) {
		return nil
	}
	cbRgFcLcb := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	pos += 2
	fibBase := pos

	const clxIndex = 33
	if clxIndex >= cbRgFcLcb || fibBase+(clxIndex+1)*8 > len(wordStream) {
		return nil
	}
	fcClx := int(binary.LittleEndian.Uint32(wordStream[fibBase+clxIndex*8:]))
	lcbClx := int(binary.LittleEndian.Uint32(wordStream[fibBase+clxIndex*8+4:]))
	if lcbClx <= 0 || fcClx < 0 || fcClx+lcbClx > len(tableStream) {
		return nil
	}

	plcPcd := extractPlcPcdFromClx(tableStream[fcClx : fcClx+lcbClx])
	if plcPcd == nil {
		return nil
	}

	hdrStart := ccpText + ccpFtn
	chpxEntries := loadChpxEntries(wordStream, tableStream, fibBase, cbRgFcLcb)
	papxEntries := loadPapxEntries(wordStream, tableStream, fibBase, cbRgFcLcb)
	clxData := tableStream[fcClx : fcClx+lcbClx]

	if data := loadDocHeadersFootersViaPlcfHdd(wordStream, tableStream, plcPcd, clxData, fibBase, cbRgFcLcb, hdrStart, ccpHdr, sectionCount, chpxEntries, papxEntries); data != nil {
		return data
	}

	raw := extractDocTextInCPRange(wordStream, plcPcd, hdrStart, hdrStart+ccpHdr)
	if scoreLegacyText(raw) == 0 {
		return nil
	}
	parts := splitHdrFtrStory(raw)
	hdrText := joinHdrFtrParts(parts, []int{0, 1, 4})
	ftrText := joinHdrFtrParts(parts, []int{2, 3, 5})
	return buildLegacyHeaderFooterDataFromFallback(hdrText, ftrText, sectionCount)
}

func loadDocHeadersFootersViaPlcfHdd(wordStream, tableStream, plcPcd, clxData []byte, fibBase, cbRgFcLcb, hdrStart, ccpHdr, sectionCount int, chpxEntries []chpxEntry, papxEntries []papxEntry) *legacyHeaderFooterData {
	const hddIndex = 11
	if hddIndex >= cbRgFcLcb || fibBase+(hddIndex+1)*8 > len(wordStream) {
		return nil
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+hddIndex*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+hddIndex*8+4:]))
	if lcb < 32 || fc < 0 || fc+lcb > len(tableStream) || lcb%4 != 0 {
		return nil
	}

	cpCount := lcb / 4
	if cpCount < 8 {
		return nil
	}
	cps := make([]int, cpCount)
	for i := range cps {
		cps[i] = int(binary.LittleEndian.Uint32(tableStream[fc+i*4 : fc+(i+1)*4]))
	}
	storyCount := len(cps) - 2
	if storyCount <= 0 {
		return nil
	}

	groupCount := 0
	if storyCount > 6 {
		groupCount = (storyCount - 6) / 6
	}
	if sectionCount <= 0 {
		sectionCount = groupCount
	}
	if groupCount > 0 && sectionCount < groupCount {
		sectionCount = groupCount
	}

	noteStories := [6]wordir.Story{}
	for i := 0; i < 6 && i < storyCount; i++ {
		start := cps[i]
		end := cps[i+1]
		if i == storyCount-1 {
			end++
		}
		if start < 0 {
			start = 0
		}
		if end < start {
			end = start
		}
		if end > ccpHdr {
			end = ccpHdr
		}
		noteStories[i] = buildHdrFtrStoryFromRange(wordStream, plcPcd, clxData, hdrStart+start, hdrStart+end, chpxEntries, papxEntries)
	}

	slots := make([][6]wordir.Story, groupCount)
	for group := 0; group < groupCount; group++ {
		for slot := 0; slot < 6; slot++ {
			storyIndex := 6 + group*6 + slot
			start := cps[storyIndex]
			end := cps[storyIndex+1]
			if storyIndex == storyCount-1 {
				end++
			}
			if start < 0 {
				start = 0
			}
			if end < start {
				end = start
			}
			if end > ccpHdr {
				end = ccpHdr
			}
			slots[group][slot] = buildHdrFtrStoryFromRange(wordStream, plcPcd, clxData, hdrStart+start, hdrStart+end, chpxEntries, papxEntries)
		}
	}
	return buildLegacyHeaderFooterDataFromSlots(noteStories, slots, sectionCount)
}

func buildLegacyHeaderFooterDataFromFallback(hdrText, ftrText string, sectionCount int) *legacyHeaderFooterData {
	if hdrText == "" && ftrText == "" {
		return nil
	}
	if sectionCount <= 0 {
		sectionCount = 1
	}
	data := &legacyHeaderFooterData{
		SectionHeaders: make([][]wordir.Reference, sectionCount),
		SectionFooters: make([][]wordir.Reference, sectionCount),
	}
	if hdrText != "" {
		data.Headers = map[string]wordir.Story{
			"default": buildHdrFtrStory(hdrText),
		}
		data.SectionHeaders[0] = []wordir.Reference{{Type: "default", PartName: "default"}}
	}
	if ftrText != "" {
		data.Footers = map[string]wordir.Story{
			"default": buildHdrFtrStory(ftrText),
		}
		data.SectionFooters[0] = []wordir.Reference{{Type: "default", PartName: "default"}}
	}
	for i := 1; i < sectionCount; i++ {
		if len(data.SectionHeaders[0]) > 0 {
			data.SectionHeaders[i] = append([]wordir.Reference(nil), data.SectionHeaders[i-1]...)
		}
		if len(data.SectionFooters[0]) > 0 {
			data.SectionFooters[i] = append([]wordir.Reference(nil), data.SectionFooters[i-1]...)
		}
	}
	return data
}

func buildLegacyHeaderFooterDataFromSlots(noteStories [6]wordir.Story, slots [][6]wordir.Story, sectionCount int) *legacyHeaderFooterData {
	if sectionCount <= 0 {
		sectionCount = len(slots)
	}
	data := &legacyHeaderFooterData{
		Headers:        map[string]wordir.Story{},
		Footers:        map[string]wordir.Story{},
		SectionHeaders: make([][]wordir.Reference, sectionCount),
		SectionFooters: make([][]wordir.Reference, sectionCount),
	}
	prevHeaders := map[string]wordir.Reference{}
	prevFooters := map[string]wordir.Reference{}
	for sectionIndex := 0; sectionIndex < sectionCount; sectionIndex++ {
		var group [6]wordir.Story
		if sectionIndex < len(slots) {
			group = slots[sectionIndex]
		}
		data.SectionHeaders[sectionIndex] = buildLegacyHeaderReferences(data.Headers, group, sectionIndex, prevHeaders)
		data.SectionFooters[sectionIndex] = buildLegacyFooterReferences(data.Footers, group, sectionIndex, prevFooters)
	}
	if noteStories[3].Text != "" {
		data.EndnoteSeparator = noteStories[3]
	}
	if noteStories[4].Text != "" {
		data.EndnoteContinuationSep = noteStories[4]
	}
	if noteStories[5].Text != "" {
		data.EndnoteContinuationNotice = noteStories[5]
	}
	if noteStories[0].Text != "" {
		data.FootnoteSeparator = noteStories[0]
	}
	if noteStories[1].Text != "" {
		data.FootnoteContinuationSep = noteStories[1]
	}
	if noteStories[2].Text != "" {
		data.FootnoteContinuationNotice = noteStories[2]
	}
	if len(data.Headers) == 0 {
		data.Headers = nil
	}
	if len(data.Footers) == 0 {
		data.Footers = nil
	}
	if data.Headers == nil && data.Footers == nil &&
		data.FootnoteSeparator.Text == "" &&
		data.FootnoteContinuationSep.Text == "" &&
		data.FootnoteContinuationNotice.Text == "" &&
		data.EndnoteSeparator.Text == "" &&
		data.EndnoteContinuationSep.Text == "" &&
		data.EndnoteContinuationNotice.Text == "" {
		return nil
	}
	return data
}

func buildLegacyHeaderReferences(stories map[string]wordir.Story, group [6]wordir.Story, sectionIndex int, prev map[string]wordir.Reference) []wordir.Reference {
	specs := []struct {
		slot    int
		refType string
		label   string
	}{
		{slot: 0, refType: "even", label: "even"},
		{slot: 1, refType: "default", label: "default"},
		{slot: 4, refType: "first", label: "first"},
	}
	return buildLegacyHeaderFooterRefs(stories, group, sectionIndex, prev, "header", specs)
}

func buildLegacyFooterReferences(stories map[string]wordir.Story, group [6]wordir.Story, sectionIndex int, prev map[string]wordir.Reference) []wordir.Reference {
	specs := []struct {
		slot    int
		refType string
		label   string
	}{
		{slot: 2, refType: "even", label: "even"},
		{slot: 3, refType: "default", label: "default"},
		{slot: 5, refType: "first", label: "first"},
	}
	return buildLegacyHeaderFooterRefs(stories, group, sectionIndex, prev, "footer", specs)
}

func buildLegacyHeaderFooterRefs(stories map[string]wordir.Story, group [6]wordir.Story, sectionIndex int, prev map[string]wordir.Reference, kind string, specs []struct {
	slot    int
	refType string
	label   string
}) []wordir.Reference {
	refs := make([]wordir.Reference, 0, len(specs))
	for _, spec := range specs {
		story := group[spec.slot]
		if story.Text == "" {
			if ref, ok := prev[spec.refType]; ok {
				refs = append(refs, ref)
			}
			continue
		}
		partName := fmt.Sprintf("legacy-%s-s%d-%s", kind, sectionIndex+1, spec.label)
		stories[partName] = story
		ref := wordir.Reference{Type: spec.refType, PartName: partName}
		refs = append(refs, ref)
		prev[spec.refType] = ref
	}
	return refs
}

// extractPlcPcdFromClx walks a Clx structure and returns the raw PlcPcd bytes.
func extractPlcPcdFromClx(clx []byte) []byte {
	cft := loadLegacyComplexFileTable(clx)
	if cft == nil {
		return nil
	}
	return cft.PlcPcd
}

// extractDocTextInCPRange extracts and decodes text from the PlcPcd for the
// CP range [startCP, endCP). Returns text with line/story separators preserved
// so header/footer slot positions remain recoverable.
func extractDocTextInCPRange(wordStream, plc []byte, startCP, endCP int) string {
	pieceCount := (len(plc) - 4) / 12
	if pieceCount <= 0 || startCP >= endCP {
		return ""
	}
	cpBase := 4 * (pieceCount + 1)
	charPos := make([]int, pieceCount+1)
	for i := range charPos {
		charPos[i] = int(binary.LittleEndian.Uint32(plc[i*4:]))
	}

	var text strings.Builder
	for i := 0; i < pieceCount; i++ {
		pieceStart := charPos[i]
		pieceEnd := charPos[i+1]

		overlapStart := pieceStart
		if startCP > overlapStart {
			overlapStart = startCP
		}
		overlapEnd := pieceEnd
		if endCP < overlapEnd {
			overlapEnd = endCP
		}
		if overlapStart >= overlapEnd {
			continue
		}
		offsetInPiece := overlapStart - pieceStart
		charCount := overlapEnd - overlapStart

		pcd := plc[cpBase+i*8 : cpBase+(i+1)*8]
		fcRaw := binary.LittleEndian.Uint32(pcd[2:6])
		compressed := fcRaw&0x40000000 != 0
		baseOff := int(fcRaw & 0x3FFFFFFF)

		if compressed {
			baseOff /= 2
			byteStart := baseOff + offsetInPiece
			byteEnd := byteStart + charCount
			if byteStart < 0 || byteEnd > len(wordStream) {
				continue
			}
			text.WriteString(decodeWindows1252(wordStream[byteStart:byteEnd]))
		} else {
			byteStart := baseOff + offsetInPiece*2
			byteEnd := byteStart + charCount*2
			if byteStart < 0 || byteEnd > len(wordStream) {
				continue
			}
			text.WriteString(decodeUTF16LE(wordStream[byteStart:byteEnd]))
		}
	}
	return normalizeLegacyStoryText(text.String())
}

// splitHdrFtrStory splits the raw header story into up to 6 sub-parts
// separated by paragraph/story terminators while preserving empty slots.
// Up to 6 parts are returned (even-hdr, odd-hdr, even-ftr, odd-ftr, 1st-hdr, 1st-ftr).
func splitHdrFtrStory(raw string) []string {
	parts := strings.Split(raw, "\n")
	result := make([]string, 0, 6)
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
		if len(result) == 6 {
			break
		}
	}
	return result
}

// joinHdrFtrParts concatenates selected parts at the given indices.
func joinHdrFtrParts(parts []string, indices []int) string {
	var segs []string
	for _, idx := range indices {
		if idx < len(parts) && parts[idx] != "" {
			segs = append(segs, parts[idx])
		}
	}
	return strings.Join(segs, "\n")
}

// buildHdrFtrStory wraps plain text in a wordir.Story with a single paragraph.
func buildHdrFtrStory(text string) wordir.Story {
	text = normalizeLegacyText(text)
	para := wordir.Paragraph{
		Runs: []wordir.Run{{Text: text}},
		Text: text,
	}
	return wordir.Story{
		Paragraphs: []wordir.Paragraph{para},
		Text:       text,
	}
}

func buildHdrFtrStoryFromRaw(raw string) wordir.Story {
	runs := applyDocFieldsToRuns([]docRun{{Text: raw}})
	return buildHdrFtrStoryFromRuns(runs)
}

func buildHdrFtrStoryFromRange(wordStream, plcPcd, clxData []byte, startCP, endCP int, chpxEntries []chpxEntry, papxEntries []papxEntry) wordir.Story {
	runs, ok := assembleDocRunsFromPlcPcdRange(wordStream, plcPcd, startCP, endCP, chpxEntries, papxEntries)
	if !ok && len(clxData) > 0 {
		runs = assembleDocRunsFromClxRange(wordStream, clxData, startCP, endCP, chpxEntries, papxEntries)
	}
	if len(runs) == 0 {
		raw := extractDocTextInCPRange(wordStream, plcPcd, startCP, endCP)
		return buildHdrFtrStoryFromRaw(raw)
	}
	return buildHdrFtrStoryFromRuns(runs)
}

func buildHdrFtrStoryFromRuns(runs []docRun) wordir.Story {
	runs = applyDocFieldsToRuns(runs)
	normalized := make([]docRun, 0, len(runs))
	for _, run := range runs {
		if run.Format.Vanish != nil && *run.Format.Vanish {
			continue
		}
		run.Text = normalizeLegacyStoryText(run.Text)
		if run.Text == "" && run.Kind == "" && run.Image == nil {
			continue
		}
		normalized = append(normalized, run)
	}
	normalized = applyLegacyHeaderFooterFieldHeuristics(normalized)
	if len(normalized) == 0 {
		return wordir.Story{}
	}
	lineSlices := splitDocRunsIntoLines(normalized)
	if len(lineSlices) == 0 {
		return wordir.Story{}
	}
	paragraphs := make([]wordir.Paragraph, 0, len(lineSlices))
	blocks := make([]wordir.Block, 0, len(lineSlices))
	textParts := make([]string, 0, len(lineSlices))
	for _, lineRuns := range lineSlices {
		if lineSliceIsLegacyListBoundary(lineRuns) {
			continue
		}
		paragraph, _ := buildLegacyParagraphFromRuns(len(paragraphs), lineRuns, nil, nil, nil, nil)
		if paragraph.Text == "" && len(paragraph.Runs) == 0 {
			continue
		}
		paragraphs = append(paragraphs, paragraph)
		blocks = append(blocks, wordir.Block{
			Index:     len(blocks),
			Kind:      "paragraph",
			Paragraph: &paragraphs[len(paragraphs)-1],
		})
		textParts = append(textParts, paragraphs[len(paragraphs)-1].Text)
	}
	text := strings.TrimSpace(strings.Join(textParts, "\n"))
	if len(paragraphs) == 0 || text == "" {
		return wordir.Story{}
	}
	return wordir.Story{
		Paragraphs: paragraphs,
		Blocks:     blocks,
		Text:       text,
	}
}

func applyLegacyHeaderFooterFieldHeuristics(runs []docRun) []docRun {
	out := make([]docRun, 0, len(runs))
	for _, run := range runs {
		if run.FieldKind != "" || run.Target != "" || run.Kind != "" {
			out = append(out, run)
			continue
		}
		out = append(out, splitLegacyHeaderFooterTextRun(run)...)
	}
	return out
}

func splitLegacyHeaderFooterTextRun(run docRun) []docRun {
	text := run.Text
	if !strings.Contains(text, "PAGE ") && !strings.Contains(text, "FILENAME ") && !strings.Contains(text, "AUTHOR ") && !strings.Contains(text, "DATE ") {
		return []docRun{run}
	}
	out := make([]docRun, 0, 3)
	for {
		idx, kind, prefixLen := findLegacyHeaderFooterField(text)
		if idx < 0 {
			if text != "" {
				run.Text = text
				out = append(out, run)
			}
			return out
		}
		if idx > 0 {
			prefix := run
			prefix.Text = text[:idx]
			out = append(out, prefix)
		}
		rest := text[idx+prefixLen:]
		valueLen := 0
		switch kind {
		case "PAGE", "FILENAME", "AUTHOR":
			for valueLen < len(rest) {
				ch := rest[valueLen]
				if ch == '\t' || ch == '\n' {
					break
				}
				valueLen++
			}
		case "DATE":
			if tab := strings.IndexAny(rest, "\t\n"); tab >= 0 {
				valueLen = tab
			} else {
				valueLen = len(rest)
			}
			if quote := strings.LastIndexByte(rest[:valueLen], '"'); quote >= 0 && quote+1 < valueLen {
				for quote+1 < valueLen && rest[quote+1] == ' ' {
					quote++
				}
				rest = rest[quote+1:]
				valueLen = len(rest)
			}
		}
		if valueLen == 0 {
			run.Text = text
			out = append(out, run)
			return out
		}
		field := run
		field.Text = rest[:valueLen]
		field.Text = strings.TrimSpace(field.Text)
		if kind == "PAGE" {
			field.FieldKind = "PAGE"
			field.FieldArg = ""
			field.Anchor = ""
		}
		out = append(out, field)
		text = rest[valueLen:]
	}
}

func findLegacyHeaderFooterField(text string) (idx int, kind string, prefixLen int) {
	best := -1
	for _, candidate := range []struct {
		kind   string
		prefix string
	}{
		{kind: "PAGE", prefix: "PAGE "},
		{kind: "FILENAME", prefix: "FILENAME "},
		{kind: "AUTHOR", prefix: "AUTHOR "},
		{kind: "DATE", prefix: "DATE "},
	} {
		i := strings.Index(text, candidate.prefix)
		if i < 0 {
			continue
		}
		if best < 0 || i < best {
			best = i
			kind = candidate.kind
			prefixLen = len(candidate.prefix)
		}
	}
	return best, kind, prefixLen
}

func normalizeLegacyStoryText(text string) string {
	text = strings.ReplaceAll(text, "\u0007", "\t")
	text = strings.ReplaceAll(text, "\u000B", "\n")
	text = strings.ReplaceAll(text, "\u000C", "\n")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var b strings.Builder
	for _, r := range text {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r == '\u0000':
			continue
		case unicode.IsPrint(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}