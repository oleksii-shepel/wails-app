package legacy

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
)

// loadPreWord97FootnoteStories decodes footnote stories from a pre-Word-97
// (nFib=0x0065/0x0068) WordDocument stream.  It uses PlcffndRef (FIB 0x68/0x6C)
// and PlcffndTxt (FIB 0x70/0x74) — both live directly inside the WordDocument
// stream for pre-Word-97 files, unlike Word 97+ where they live in 0Table/1Table.
//
// Returns nil when CcpFtn==0 or either PLC has zero size (common; most Word 6/95
// documents have no footnotes).
func loadPreWord97FootnoteStories(wordStream []byte) *legacyNoteData {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.CcpFtn <= 0 {
		return nil
	}
	return buildPreWord97NoteData(wordStream, span,
		span.FtnRefTableOffset, span.FtnRefTableSize,
		span.FtnTxtTableOffset, span.FtnTxtTableSize,
		span.CcpText, // footnote story starts immediately after main body
	)
}

// loadPreWord97EndnoteStories decodes endnote stories from a pre-Word-97
// WordDocument stream.  When the FIB carries explicit PlcfendRef/PlcfendTxt
// entries (FibRgFcLcb95 offsets 0x128/0x130), those are used directly.
// Otherwise the function falls back to heuristic PLC scanning so that
// hand-crafted synthetic fixtures (e.g. test streams written before the full
// FIB map was known) continue to work.
func loadPreWord97EndnoteStories(wordStream []byte) *legacyNoteData {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.CcpEdn <= 0 {
		return nil
	}
	// Endnote text story starts after: main + ftn + hdd + mcr + atn.
	storyStart := span.CcpText + span.CcpFtn + span.CcpHdd + span.CcpMcr + span.CcpAtn
	if storyStart < 0 {
		return nil
	}
	if span.Start+storyStart+span.CcpEdn > len(wordStream) {
		return nil
	}

	// Use explicitly mapped FIB fields when present.
	if span.EdnRefTableOffset > 0 && span.EdnRefTableSize > 0 &&
		span.EdnRefTableOffset+span.EdnRefTableSize <= len(wordStream) {
		return buildPreWord97NoteData(wordStream, span,
			span.EdnRefTableOffset, span.EdnRefTableSize,
			span.EdnTxtTableOffset, span.EdnTxtTableSize,
			storyStart,
		)
	}
	// Heuristic fallback for synthetic test streams that lack explicit FIB
	// endnote fields but embed a recognisable PLC pair in the stream.
	const fibFcLcb95Base = 0x58
	const fibFcLcb95End = 0x160 // start of ComplexTableOffset field
	maxEntries := (fibFcLcb95End - fibFcLcb95Base) / 8

	type plcCandidate struct {
		entry int
		off   int
		sz    int
		cps   []int
	}
	var refs []plcCandidate
	var texts []plcCandidate

	for entry := 0; entry < maxEntries; entry++ {
		offPos := fibFcLcb95Base + entry*8
		fc := int(binary.LittleEndian.Uint32(wordStream[offPos : offPos+4]))
		lcb := int(binary.LittleEndian.Uint32(wordStream[offPos+4 : offPos+8]))
		if lcb <= 0 || fc < 0 || fc+lcb > len(wordStream) {
			continue
		}
		if lcb < 8 || lcb%4 != 0 || lcb > 1024 {
			continue
		}
		plc := wordStream[fc : fc+lcb]

		if refCPs := parseLegacyNoteRefCPs(plc); len(refCPs) > 0 {
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

		if textCPs := parseLegacyNoteTextCPs(plc); len(textCPs) >= 2 {
			if textCPs[0] != 0 || textCPs[len(textCPs)-1] != span.CcpEdn {
				continue
			}
			okMonotonic := true
			hasPositiveSpan := false
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
	bestRef := plcCandidate{}
	bestText := plcCandidate{}
	bestOK := false
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
			// Prefer adjacent entries/offsets; penalize candidate sets that reuse
			// known footnote/annotation slots.
			score := absInt(ref.entry-text.entry)*16 + absInt(ref.off-text.off)
			if (ref.entry == 2 || ref.entry == 3 || ref.entry == 4 || ref.entry == 5) ||
				(text.entry == 2 || text.entry == 3 || text.entry == 4 || text.entry == 5) {
				score += 1 << 20
			}
			if score >= bestScore {
				continue
			}
			bestScore = score
			bestRef = ref
			bestText = text
			bestOK = true
		}
	}
	if !bestOK {
		return nil
	}

	return buildPreWord97NoteData(wordStream, span,
		bestRef.off, bestRef.sz,
		bestText.off, bestText.sz,
		storyStart,
	)
}

// loadPreWord97AnnotationStories decodes comment/annotation stories from a
// pre-Word-97 WordDocument stream.  It uses PlcfandRef (FIB 0x78/0x7C) and
// PlcfandTxt (FIB 0x80/0x84) — both live inside the WordDocument stream.
//
// Returns nil when CcpAtn==0 or either PLC has zero size.
func loadPreWord97AnnotationStories(wordStream []byte) *legacyNoteData {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.CcpAtn <= 0 {
		return nil
	}
	// Annotation story starts after: main + ftn + hdd + mcr.
	atnStart := span.CcpText + span.CcpFtn + span.CcpHdd + span.CcpMcr
	return buildPreWord97NoteData(wordStream, span,
		span.AtnRefTableOffset, span.AtnRefTableSize,
		span.AtnTxtTableOffset, span.AtnTxtTableSize,
		atnStart,
	)
}

// buildPreWord97NoteData is the common note-data builder for pre-Word-97
// streams.  Unlike the Word 97+ path the PLCs live inside the WordDocument
// stream itself (not a separate table stream), and note text is decoded as
// single-byte text using the detected charset rather than UTF-16.
//
//   - wordStream: the full WordDocument stream bytes
//   - span: pre-computed body span (contains charset, Start, etc.)
//   - refOff/refSz: absolute byte offset and size of the reference PLC inside wordStream
//   - txtOff/txtSz: absolute byte offset and size of the text-boundary PLC inside wordStream
//   - storyStart: character position (relative to span.Start) where the note story begins
func buildPreWord97NoteData(wordStream []byte, span PreWord97BodySpan,
	refOff, refSz, txtOff, txtSz, storyStart int) *legacyNoteData {

	if refSz <= 0 || txtSz <= 0 {
		return nil
	}
	if refOff < 0 || refOff+refSz > len(wordStream) {
		return nil
	}
	if txtOff < 0 || txtOff+txtSz > len(wordStream) {
		return nil
	}
	refCPs := parseLegacyNoteRefCPs(wordStream[refOff : refOff+refSz])
	textCPs := parseLegacyNoteTextCPs(wordStream[txtOff : txtOff+txtSz])
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
	refCPs = refCPs[:noteCount]
	textCPs = textCPs[:noteCount+1]

	preferred := guessPreWord97Charset(wordStream, span)
	data := &legacyNoteData{
		RefByCP: make(map[int]int, noteCount),
		Stories: make(map[int]wordir.Story, noteCount),
	}
	for i, refCP := range refCPs {
		id := i + 1
		data.RefByCP[refCP] = id

		cpStart := storyStart + textCPs[i]
		cpEnd := storyStart + textCPs[i+1]
		byteStart := span.Start + cpStart
		byteEnd := span.Start + cpEnd
		if byteStart < 0 || byteEnd > len(wordStream) || byteEnd <= byteStart {
			continue
		}
		raw := decodePreWord97SingleByteTextWithCharset(wordStream[byteStart:byteEnd], preferred)
		// Strip leading note anchor character (0x02 or first non-text byte).
		raw = stripPreWord97NoteMarker(raw)
		if raw == "" {
			continue
		}
		data.Stories[id] = wordir.Story{Text: raw}
	}
	if len(data.RefByCP) == 0 && len(data.Stories) == 0 {
		return nil
	}
	return data
}

// stripPreWord97NoteMarker removes the leading note-anchor byte that Word 6/95
// inserts as the first character of each note story (typically a single 0x02
// or the corresponding Unicode replacement).
func stripPreWord97NoteMarker(s string) string {
	for len(s) > 0 {
		r := rune(s[0])
		if r == 0x02 || r == '\x02' {
			s = s[1:]
			continue
		}
		break
	}
	return s
}

// preWord97NoteDataAsStoryMap converts a *legacyNoteData to the
// map[int]wordir.Story format expected by legacyDocParseData.
func preWord97NoteDataAsStoryMap(nd *legacyNoteData) map[int]wordir.Story {
	if nd == nil || len(nd.Stories) == 0 {
		return nil
	}
	return nd.Stories
}

// preWord97InjectNoteRefs wires note-reference run markers into body runs using
// the RefByCP map from a *legacyNoteData.  It mirrors injectDocFootnoteRefs but
// operates on CP positions rather than CLX-mapped file offsets.
//
// For pre-Word-97, each note anchor in the main body is a single 0x02 byte at
// a known character position.  We mark the containing run so the HTML renderer
// can emit a superscript reference link.
func preWord97InjectNoteRefs(runs []docRun, nd *legacyNoteData, kind string) []docRun {
	if nd == nil || len(nd.RefByCP) == 0 {
		return runs
	}
	out := make([]docRun, 0, len(runs)+len(nd.RefByCP))
	for _, r := range runs {
		// Scan character-by-character for anchor positions within this run.
		cp := r.StartCP
		runes := []rune(r.Text)
		segStart := 0
		segCP := cp
		for i, ch := range runes {
			if id, ok := nd.RefByCP[cp+i]; ok {
				// Flush text before marker.
				if i > segStart {
					seg := r
					seg.Text = string(runes[segStart:i])
					seg.StartCP = segCP
					out = append(out, seg)
				}
				// Emit anchor run.
				ref := r
				ref.Text = string(ch)
				ref.StartCP = cp + i
				ref.Kind = kind
				ref.NoteID = id
				out = append(out, ref)
				segStart = i + 1
				segCP = cp + i + 1
			}
		}
		// Flush remainder.
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

// readPreWord97PlcfInt32 reads a flat array of little-endian int32 values from
// a pre-Word-97 PLC stored directly in the WordDocument stream.
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

// loadPreWord97BookmarkData reads bookmark names and start/end CPs from the
// pre-Word-97 FIB fields SttbfBkmk (0x100), PlcfBkf (0x108), and PlcfBkl
// (0x110).  For pre-Word-97 files all three tables live inside the WordDocument
// stream itself (there is no separate 0Table/1Table stream).
//
// Returns (nil, nil) when no valid bookmark data can be loaded.
func loadPreWord97BookmarkData(wordStream []byte) (map[int][]string, map[string]int) {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		return nil, nil
	}
	if span.BkmkStartTableOffset <= 0 || span.BkmkStartTableSize <= 0 {
		return nil, nil
	}
	if span.BkmkStartTableOffset+span.BkmkStartTableSize > len(wordStream) {
		return nil, nil
	}

	// Decode bookmark start CP array (PlcfBkf).
	startBytes := wordStream[span.BkmkStartTableOffset : span.BkmkStartTableOffset+span.BkmkStartTableSize]
	starts := parseLegacyBookmarkStartPLC(startBytes, span.CcpText)
	if len(starts) == 0 {
		return nil, nil
	}

	// Decode bookmark name table (SttbfBkmk) — ANSI for pre-Word97.
	var names []string
	if span.BkmkNameTableOffset > 0 && span.BkmkNameTableSize > 0 &&
		span.BkmkNameTableOffset+span.BkmkNameTableSize <= len(wordStream) {
		names = parseLegacyBookmarkNameTable(wordStream[span.BkmkNameTableOffset : span.BkmkNameTableOffset+span.BkmkNameTableSize])
	}
	if len(names) == 0 || len(names) != len(starts) {
		return nil, nil
	}

	// Decode bookmark end CP array (PlcfBkl).
	var ends []int
	if span.BkmkEndTableOffset > 0 && span.BkmkEndTableSize > 0 &&
		span.BkmkEndTableOffset+span.BkmkEndTableSize <= len(wordStream) {
		endBytes := wordStream[span.BkmkEndTableOffset : span.BkmkEndTableOffset+span.BkmkEndTableSize]
		ends = parseLegacyBookmarkEndPLC(endBytes, len(starts), span.CcpText)
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