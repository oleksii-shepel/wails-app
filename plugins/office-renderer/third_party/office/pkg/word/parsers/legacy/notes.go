package legacy

import (
	"encoding/binary"
	"strings"
	"unicode"
	"unicode/utf8"

	wordir "office/pkg/word/ir"
)

type legacyNoteData struct {
	RefByCP    map[int]int
	Stories    map[int]wordir.Story
	AuthorByID map[int]string // comment ID → author name/initials (optional)
	DateByID   map[int]string // comment ID → date string "YYYY-MM-DD" (optional)
}

func loadDocFootnotes(wordStream, tableStream, clxData []byte, fibBase, cbRgFcLcb, ccpText int, chpxEntries []chpxEntry, papxEntries []papxEntry) *legacyNoteData {
	const (
		footnoteRefIndex  = 2
		footnoteTextIndex = 3
	)
	refFC, refLCB, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, footnoteRefIndex)
	if !ok {
		return nil
	}
	textFC, textLCB, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, footnoteTextIndex)
	if !ok {
		return nil
	}
	if refLCB <= 0 || textLCB <= 0 || refFC < 0 || textFC < 0 || refFC+refLCB > len(tableStream) || textFC+textLCB > len(tableStream) {
		return nil
	}
	refCPs := parseLegacyNoteRefCPs(tableStream[refFC : refFC+refLCB])
	textCPs := parseLegacyNoteTextCPs(tableStream[textFC : textFC+textLCB])
	if len(refCPs) == 0 || len(textCPs) < 2 {
		return nil
	}

	noteCount := len(refCPs)
	if len(textCPs)-1 < noteCount {
		noteCount = len(textCPs) - 1
	}
	if noteCount <= 0 {
		return nil
	}
	return buildLegacyNoteData(wordStream, clxData, ccpText, refCPs[:noteCount], textCPs[:noteCount+1], chpxEntries, papxEntries)
}

func loadDocEndnotes(wordStream, tableStream, clxData []byte, fibBase, cbRgFcLcb int, counts legacyStoryCounts, chpxEntries []chpxEntry, papxEntries []papxEntry) *legacyNoteData {
	// MS-DOC FibRgFcLcb97 fixed indices for endnote PLCs (PLCFENDREF=46, PLCFENDTXT=47).
	// Try these first so files that have correct PLCs but ccpEdn=0 (e.g. LibreOffice)
	// are handled, then fall back to the heuristic scan when ccpEdn is known.
	const endnoteRefIndex = 46
	const endnoteTextIndex = 47
	storyStart := counts.Text + counts.Ftn + counts.Hdr + counts.Mcr + counts.Atn
	if refFC, refLCB, ok1 := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, endnoteRefIndex); ok1 && refLCB > 0 && refFC >= 0 && refFC+refLCB <= len(tableStream) {
		if textFC, textLCB, ok2 := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, endnoteTextIndex); ok2 && textLCB > 0 && textFC >= 0 && textFC+textLCB <= len(tableStream) {
			refCPs := parseLegacyNoteRefCPs(tableStream[refFC : refFC+refLCB])
			textCPs := parseLegacyNoteTextCPs(tableStream[textFC : textFC+textLCB])
			if len(refCPs) > 0 && len(textCPs) >= 2 {
				noteCount := len(refCPs)
				if len(textCPs)-1 < noteCount {
					noteCount = len(textCPs) - 1
				}
				if noteCount > 0 {
					if data := buildLegacyNoteData(wordStream, clxData, storyStart, refCPs[:noteCount], textCPs[:noteCount+1], chpxEntries, papxEntries); data != nil {
						return data
					}
				}
			}
		}
	}
	if counts.Edn <= 0 {
		return nil
	}
	refCPs, textCPs, ok := findLegacyNotePLCPair(wordStream, tableStream, fibBase, cbRgFcLcb, counts.Text, counts.Edn, map[int]bool{2: true, 3: true})
	if !ok {
		// Some producer variants write endnote PLCs in slots typically used for
		// footnotes. Retry without slot exclusions when the strict pass fails.
		refCPs, textCPs, ok = findLegacyNotePLCPair(wordStream, tableStream, fibBase, cbRgFcLcb, counts.Text, counts.Edn, nil)
	}
	if !ok {
		return nil
	}
	noteCount := len(refCPs)
	if len(textCPs)-1 < noteCount {
		noteCount = len(textCPs) - 1
	}
	if noteCount <= 0 {
		return nil
	}
	return buildLegacyNoteData(wordStream, clxData, storyStart, refCPs[:noteCount], textCPs[:noteCount+1], chpxEntries, papxEntries)
}

func buildLegacyNoteData(wordStream, clxData []byte, storyStart int, refCPs []int, textCPs []int, chpxEntries []chpxEntry, papxEntries []papxEntry) *legacyNoteData {
	data := &legacyNoteData{
		RefByCP: make(map[int]int, len(refCPs)),
		Stories: make(map[int]wordir.Story, len(refCPs)),
	}
	for i := range refCPs {
		id := i + 1
		data.RefByCP[refCPs[i]] = id
		startCP := storyStart + textCPs[i]
		endCP := storyStart + textCPs[i+1]
		runs := assembleDocRunsFromClxRange(wordStream, clxData, startCP, endCP, chpxEntries, papxEntries)
		if len(runs) == 0 {
			continue
		}
		runs = stripLeadingLegacyNoteMarker(runs)
		data.Stories[id] = buildLegacyStoryFromRuns(runs)
	}
	if len(data.RefByCP) == 0 && len(data.Stories) == 0 {
		return nil
	}
	return data
}

func findLegacyNotePLCPair(wordStream, tableStream []byte, fibBase, cbRgFcLcb, maxRefCP, storyLen int, excluded map[int]bool) ([]int, []int, bool) {
	type refCandidate struct {
		index int
		cps   []int
	}
	type textCandidate struct {
		index int
		cps   []int
	}
	var refs []refCandidate
	var texts []textCandidate
	for index := 0; index < cbRgFcLcb; index++ {
		if excluded[index] {
			continue
		}
		fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
		if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
			continue
		}
		plc := tableStream[fc : fc+lcb]
		if cps := parseLegacyNoteTextCPs(plc); len(cps) >= 2 && cps[0] == 0 && (storyLen <= 0 || cps[len(cps)-1] == storyLen) {
			texts = append(texts, textCandidate{index: index, cps: cps})
		}
		if cps := parseLegacyNoteRefCPs(plc); len(cps) > 0 && allLegacyNoteRefCPsWithin(cps, maxRefCP) {
			refs = append(refs, refCandidate{index: index, cps: cps})
		}
	}
	bestScore := int(^uint(0) >> 1)
	var bestRef []int
	var bestText []int
	for _, text := range texts {
		wantCount := len(text.cps) - 1
		for _, ref := range refs {
			if ref.index == text.index || len(ref.cps) != wantCount {
				continue
			}
			score := absInt(text.index - ref.index)
			if text.index < ref.index {
				score += cbRgFcLcb
			}
			if score >= bestScore {
				continue
			}
			bestScore = score
			bestRef = ref.cps
			bestText = text.cps
		}
	}
	return bestRef, bestText, bestScore != int(^uint(0)>>1)
}

func allLegacyNoteRefCPsWithin(cps []int, max int) bool {
	for _, cp := range cps {
		if cp < 0 || cp > max {
			return false
		}
	}
	return true
}

// parseLegacyAnnotationRefCPs extracts body-text anchor CPs from a PLCFANDREF
// (MS-DOC FibRgFcLcb97 index 4). The PLC layout is (N+1) × 4-byte CPs followed
// by N × 30-byte ATRDPre10 records (MS-DOC §2.9.3: 22-byte xstUsrInitl + 2-byte
// ibst + 2-byte grfbmc + 4-byte dttm = 30), so the CP count is (len-4)/34.
// Falls back to parseLegacyNoteRefCPs for PLC data that does not match this layout.
func parseLegacyAnnotationRefCPs(plc []byte) []int {
	const atrdPre10Size = 30
	const recSize = 4 + atrdPre10Size // 34 bytes per entry
	if len(plc) >= 4+recSize {
		n := (len(plc) - 4) / recSize
		if n > 0 && n*recSize+4 == len(plc) {
			cps := make([]int, n)
			for i := 0; i < n; i++ {
				cps[i] = int(binary.LittleEndian.Uint32(plc[i*4 : i*4+4]))
			}
			return cps
		}
	}
	return parseLegacyNoteRefCPs(plc)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// findLegacyNotePLCPairAnyStoryLen searches for matching ref/text PLCs without
// requiring a known story length. It uses the final CP from candidate text PLCs
// (which represent the discovered story length) to match against refs that fall
// within that story. Useful for files like LibreOffice exports where ccpAtn=0
// or ccpEdn=0 even though note PLCs exist.
func findLegacyNotePLCPairAnyStoryLen(wordStream, tableStream []byte, fibBase, cbRgFcLcb, maxRefCP int, excluded map[int]bool) ([]int, []int, bool) {
	type refCandidate struct {
		index int
		cps   []int
	}
	type textCandidate struct {
		index int
		cps   []int
	}
	var refs []refCandidate
	var texts []textCandidate
	for index := 0; index < cbRgFcLcb; index++ {
		if excluded[index] {
			continue
		}
		fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
		if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
			continue
		}
		plc := tableStream[fc : fc+lcb]
		// Accept any text PLC that starts at 0 (will discover storyLen from cps[len-1]).
		if cps := parseLegacyNoteTextCPs(plc); len(cps) >= 2 && cps[0] == 0 {
			texts = append(texts, textCandidate{index: index, cps: cps})
		}
		if cps := parseLegacyNoteRefCPs(plc); len(cps) > 0 && allLegacyNoteRefCPsWithin(cps, maxRefCP) {
			refs = append(refs, refCandidate{index: index, cps: cps})
		}
	}
	bestScore := int(^uint(0) >> 1)
	var bestRef []int
	var bestText []int
	for _, text := range texts {
		wantCount := len(text.cps) - 1
		storyLen := text.cps[len(text.cps)-1]
		for _, ref := range refs {
			if ref.index == text.index || len(ref.cps) != wantCount {
				continue
			}
			// Check that all ref CPs fall within [0, storyLen].
			allInRange := true
			for _, cp := range ref.cps {
				if cp < 0 || cp > storyLen {
					allInRange = false
					break
				}
			}
			if !allInRange {
				continue
			}
			score := absInt(text.index - ref.index)
			if text.index < ref.index {
				score += cbRgFcLcb
			}
			if score >= bestScore {
				continue
			}
			bestScore = score
			bestRef = ref.cps
			bestText = text.cps
		}
	}
	return bestRef, bestText, bestScore != int(^uint(0)>>1)
}

func readFibFcLcbEntry(wordStream []byte, fibBase, cbRgFcLcb, index int) (int, int, bool) {
	if index >= cbRgFcLcb || fibBase+(index+1)*8 > len(wordStream) {
		return 0, 0, false
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+index*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+index*8+4:]))
	return fc, lcb, true
}

func parseLegacyNoteRefCPs(plc []byte) []int {
	switch {
	case len(plc) >= 10 && (len(plc)-4)%6 == 0:
		count := (len(plc) - 4) / 6
		cps := make([]int, count)
		for i := 0; i < count; i++ {
			cps[i] = int(binary.LittleEndian.Uint32(plc[i*4 : (i+1)*4]))
		}
		return cps
	case len(plc) >= 8 && len(plc)%4 == 0:
		count := len(plc)/4 - 1
		cps := make([]int, count)
		for i := 0; i < count; i++ {
			cps[i] = int(binary.LittleEndian.Uint32(plc[i*4 : (i+1)*4]))
		}
		return cps
	default:
		return nil
	}
}

func parseLegacyNoteTextCPs(plc []byte) []int {
	if len(plc) < 8 || len(plc)%4 != 0 {
		return nil
	}
	count := len(plc)/4 - 1
	if count <= 0 {
		return nil
	}
	cps := make([]int, count+1)
	for i := range cps {
		cps[i] = int(binary.LittleEndian.Uint32(plc[i*4 : (i+1)*4]))
	}
	return cps
}

func injectDocNoteRefs(runs []docRun, refByCP map[int]int, kind string) []docRun {
	if len(refByCP) == 0 {
		return runs
	}
	out := make([]docRun, 0, len(runs)+len(refByCP))
	for _, r := range runs {
		if r.Kind != "" || r.Text == "" {
			out = append(out, r)
			continue
		}
		runes := []rune(r.Text)
		if len(runes) == 0 {
			out = append(out, r)
			continue
		}
		segmentStart := 0
		segmentStartCP := r.StartCP
		cp := r.StartCP
		handled := false
		for i := range runes {
			noteID, ok := refByCP[cp]
			if !ok {
				cp++
				continue
			}
			handled = true
			if i > segmentStart {
				out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:i]), segmentStartCP))
			}
			out = append(out, docRun{Kind: kind, NoteID: noteID, Format: r.Format, ParaFormat: r.ParaFormat, StartCP: cp, Revision: r.Revision})
			cp++
			segmentStart = i + 1
			segmentStartCP = cp
		}
		if !handled {
			out = append(out, r)
			continue
		}
		if segmentStart < len(runes) {
			out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:]), segmentStartCP))
		}
	}
	return out
}

func cloneDocRunWithText(src docRun, text string, startCP int) docRun {
	return docRun{
		Text:       text,
		Format:     src.Format,
		ParaFormat: src.ParaFormat,
		Target:     src.Target,
		Kind:       src.Kind,
		NoteID:     src.NoteID,
		StartCP:    startCP,
		CommentID:  src.CommentID,
		Revision:   src.Revision,
		Spec:       src.Spec,
		Image:      src.Image,
	}
}

func stripLeadingLegacyNoteMarker(runs []docRun) []docRun {
	for i := range runs {
		if runs[i].Text == "" {
			continue
		}
		first, width := utf8.DecodeRuneInString(runs[i].Text)
		if first == utf8.RuneError && width == 0 {
			return runs
		}
		if first == '\n' || first == '\t' || unicode.IsPrint(first) {
			return runs
		}
		runs[i].Text = runs[i].Text[width:]
		runs[i].StartCP++
		if runs[i].Text == "" {
			return append(append([]docRun(nil), runs[:i]...), runs[i+1:]...)
		}
		return runs
	}
	return runs
}

func buildLegacyStoryFromRuns(runs []docRun) wordir.Story {
	doc := buildLegacyDocFromRuns("", applyDocFieldsToRuns(runs))
	return wordir.Story{
		Paragraphs: doc.Paragraphs,
		Tables:     doc.Tables,
		Blocks:     doc.Blocks,
		Text:       normalizeLegacyText(strings.TrimSpace(joinDocRunsText(runs))),
	}
}