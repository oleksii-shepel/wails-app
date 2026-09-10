package legacy

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

// loadPreWord97FootnoteStories decodes footnote stories from a pre-Word-97
// (nFib=0x0065/0x0068) WordDocument stream.  It uses PlcffndRef (FIB 0x68/0x6C)
// and PlcffndTxt (FIB 0x70/0x74) — both live directly inside the WordDocument
// stream for pre-Word-97 files, unlike Word 97+ where they live in 0Table/1Table.
func loadPreWord97FootnoteStories(wordStream []byte) *legacyNoteData {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.CcpFtn <= 0 {
		return nil
	}
	return buildPreWord97NoteData(wordStream, span,
		span.FtnRefTableOffset, span.FtnRefTableSize,
		span.FtnTxtTableOffset, span.FtnTxtTableSize,
		span.CcpText,
	)
}

func loadPreWord97EndnoteStories(wordStream []byte) *legacyNoteData {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.CcpEdn <= 0 {
		return nil
	}
	storyStart := span.CcpText + span.CcpFtn + span.CcpHdd + span.CcpMcr + span.CcpAtn
	if storyStart < 0 || span.Start+storyStart+span.CcpEdn > len(wordStream) {
		return nil
	}
	if span.EdnRefTableOffset > 0 && span.EdnRefTableSize > 0 && span.EdnRefTableOffset+span.EdnRefTableSize <= len(wordStream) {
		return buildPreWord97NoteData(wordStream, span,
			span.EdnRefTableOffset, span.EdnRefTableSize,
			span.EdnTxtTableOffset, span.EdnTxtTableSize,
			storyStart,
		)
	}

	const fibFcLcb95Base = 0x58
	const fibFcLcb95End = 0x160
	maxEntries := (fibFcLcb95End - fibFcLcb95Base) / 8
	type plcCandidate struct {
		entry, off, sz int
		cps            []int
	}
	var refs, texts []plcCandidate
	for entry := 0; entry < maxEntries; entry++ {
		offPos := fibFcLcb95Base + entry*8
		fc := int(binary.LittleEndian.Uint32(wordStream[offPos : offPos+4]))
		lcb := int(binary.LittleEndian.Uint32(wordStream[offPos+4 : offPos+8]))
		if lcb <= 0 || fc < 0 || fc+lcb > len(wordStream) || lcb < 8 || lcb%4 != 0 || lcb > 1024 {
			continue
		}
		plc := wordStream[fc : fc+lcb]
		if refCPs := docbinary.ParseNoteRefCPs(plc); len(refCPs) > 0 {
			okRange := true
			for _, cp := range refCPs {
				if cp < 0 || cp > span.CcpText {
					okRange = false
					break
				}
			}
			if okRange {
				refs = append(refs, plcCandidate{entry: entry, off: fc, sz: lcb, cps: refCPs})
			}
		}
		if textCPs := docbinary.ParseNoteTextCPs(plc); len(textCPs) >= 2 {
			if textCPs[0] != 0 || textCPs[len(textCPs)-1] != span.CcpEdn {
				continue
			}
			okMonotonic, hasPositiveSpan := true, false
			for i := 0; i < len(textCPs)-1; i++ {
				if textCPs[i] > textCPs[i+1] {
					okMonotonic = false
					break
				}
				if textCPs[i] < textCPs[i+1] {
					hasPositiveSpan = true
				}
			}
			if okMonotonic && hasPositiveSpan {
				texts = append(texts, plcCandidate{entry: entry, off: fc, sz: lcb, cps: textCPs})
			}
		}
	}
	bestScore := int(^uint(0) >> 1)
	bestRef, bestText, bestOK := plcCandidate{}, plcCandidate{}, false
	for _, ref := range refs {
		for _, text := range texts {
			if ref.off == text.off {
				continue
			}
			n := len(ref.cps)
			if len(text.cps)-1 < n {
				n = len(text.cps) - 1
			}
			if n <= 0 {
				continue
			}
			score := absInt(ref.entry-text.entry)*16 + absInt(ref.off-text.off)
			if ref.entry == 2 || ref.entry == 3 || ref.entry == 4 || ref.entry == 5 || text.entry == 2 || text.entry == 3 || text.entry == 4 || text.entry == 5 {
				score += 1 << 20
			}
			if score < bestScore {
				bestScore, bestRef, bestText, bestOK = score, ref, text, true
			}
		}
	}
	if !bestOK {
		return nil
	}
	return buildPreWord97NoteData(wordStream, span, bestRef.off, bestRef.sz, bestText.off, bestText.sz, storyStart)
}

func loadPreWord97AnnotationStories(wordStream []byte) *legacyNoteData {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.CcpAtn <= 0 {
		return nil
	}
	atnStart := span.CcpText + span.CcpFtn + span.CcpHdd + span.CcpMcr
	return buildPreWord97NoteData(wordStream, span,
		span.AtnRefTableOffset, span.AtnRefTableSize,
		span.AtnTxtTableOffset, span.AtnTxtTableSize,
		atnStart,
	)
}

func buildPreWord97NoteData(wordStream []byte, span PreWord97BodySpan, refOff, refSz, txtOff, txtSz, storyStart int) *legacyNoteData {
	if refSz <= 0 || txtSz <= 0 || refOff < 0 || refOff+refSz > len(wordStream) || txtOff < 0 || txtOff+txtSz > len(wordStream) {
		return nil
	}
	refCPs := docbinary.ParseNoteRefCPs(wordStream[refOff : refOff+refSz])
	textCPs := docbinary.ParseNoteTextCPs(wordStream[txtOff : txtOff+txtSz])
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
	refCPs, textCPs = refCPs[:noteCount], textCPs[:noteCount+1]
	preferred := guessPreWord97Charset(wordStream, span)
	data := &legacyNoteData{RefByCP: make(map[int]int, noteCount), Stories: make(map[int]wordir.Story, noteCount)}
	for i, refCP := range refCPs {
		id := i + 1
		data.RefByCP[refCP] = id
		cpStart, cpEnd := storyStart+textCPs[i], storyStart+textCPs[i+1]
		byteStart, byteEnd := span.Start+cpStart, span.Start+cpEnd
		if byteStart < 0 || byteEnd > len(wordStream) || byteEnd <= byteStart {
			continue
		}
		raw := stripPreWord97NoteMarker(decodePreWord97SingleByteTextWithCharset(wordStream[byteStart:byteEnd], preferred))
		if raw != "" {
			data.Stories[id] = wordir.Story{Text: raw}
		}
	}
	if len(data.RefByCP) == 0 && len(data.Stories) == 0 {
		return nil
	}
	return data
}

func stripPreWord97NoteMarker(s string) string {
	for len(s) > 0 && s[0] == '\x02' {
		s = s[1:]
	}
	return s
}

func preWord97NoteDataAsStoryMap(nd *legacyNoteData) map[int]wordir.Story {
	if nd == nil || len(nd.Stories) == 0 {
		return nil
	}
	return nd.Stories
}

func preWord97InjectNoteRefs(runs []binaryWordRun, nd *legacyNoteData, kind string) []binaryWordRun {
	if nd == nil || len(nd.RefByCP) == 0 {
		return runs
	}
	out := make([]binaryWordRun, 0, len(runs)+len(nd.RefByCP))
	for _, r := range runs {
		cp := r.StartCP
		runes := []rune(r.Text)
		segStart, segCP := 0, cp
		for i, ch := range runes {
			if id, ok := nd.RefByCP[cp+i]; ok {
				if i > segStart {
					seg := r
					seg.Text = string(runes[segStart:i])
					seg.StartCP = segCP
					out = append(out, seg)
				}
				ref := r
				ref.Text = string(ch)
				ref.StartCP = cp + i
				ref.Kind = kind
				ref.NoteID = id
				out = append(out, ref)
				segStart, segCP = i+1, cp+i+1
			}
		}
		if segStart == 0 {
			out = append(out, r)
		} else if segStart < len(runes) {
			seg := r
			seg.Text = string(runes[segStart:])
			seg.StartCP = segCP
			out = append(out, seg)
		}
	}
	return out
}

func readPreWord97PlcfInt32(wordStream []byte, off, sz int) []int32 {
	if sz <= 0 || off < 0 || off+sz > len(wordStream) || sz%4 != 0 {
		return nil
	}
	n := sz / 4
	out := make([]int32, n)
	for i := 0; i < n; i++ {
		out[i] = int32(binary.LittleEndian.Uint32(wordStream[off+i*4:]))
	}
	return out
}

func loadPreWord97BookmarkData(wordStream []byte) (map[int][]string, map[string]int) {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.BkmkStartTableOffset <= 0 || span.BkmkStartTableSize <= 0 || span.BkmkStartTableOffset+span.BkmkStartTableSize > len(wordStream) {
		return nil, nil
	}
	startBytes := wordStream[span.BkmkStartTableOffset : span.BkmkStartTableOffset+span.BkmkStartTableSize]
	starts := docbinary.ParseBookmarkStartCPs(startBytes, span.CcpText)
	if len(starts) == 0 {
		return nil, nil
	}
	var names []string
	if span.BkmkNameTableOffset > 0 && span.BkmkNameTableSize > 0 && span.BkmkNameTableOffset+span.BkmkNameTableSize <= len(wordStream) {
		names = docbinary.ParseBookmarkNames(wordStream[span.BkmkNameTableOffset : span.BkmkNameTableOffset+span.BkmkNameTableSize])
	}
	if len(names) == 0 || len(names) != len(starts) {
		return nil, nil
	}
	var ends []int
	if span.BkmkEndTableOffset > 0 && span.BkmkEndTableSize > 0 && span.BkmkEndTableOffset+span.BkmkEndTableSize <= len(wordStream) {
		endBytes := wordStream[span.BkmkEndTableOffset : span.BkmkEndTableOffset+span.BkmkEndTableSize]
		ends = docbinary.ParseBookmarkEndCPs(endBytes, len(starts), span.CcpText)
	}
	startMap := make(map[int][]string, len(starts))
	endMap := make(map[string]int, len(starts))
	for i, name := range names {
		startMap[starts[i]] = append(startMap[starts[i]], name)
		if i < len(ends) {
			endMap[name] = ends[i]
		}
	}
	if len(startMap) == 0 {
		return nil, nil
	}
	if len(endMap) == 0 {
		endMap = nil
	}
	return startMap, endMap
}
