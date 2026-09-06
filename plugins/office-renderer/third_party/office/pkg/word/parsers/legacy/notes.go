package legacy

import (
	"bytes"
	"encoding/binary"
	"strings"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	"office/pkg/word/parsers/doc/word97"
)

type legacyNoteData struct {
	RefByCP    map[int]int
	Stories    map[int]wordir.Story
	AuthorByID map[int]string
	DateByID   map[int]string
}

func loadCanonicalWord97SideStoryRanges(wordStream, tableStream []byte, fibBase, cbRgFcLcb int, counts docbinary.StoryCounts) word97.SideStoryRanges {
	layout := docbinary.Word97Layout{FibBase: fibBase, CbRgFcLcb: cbRgFcLcb}
	return word97.LoadSideStoryRanges(bytes.NewReader(tableStream), int64(len(tableStream)), wordStream, layout, counts)
}

func canonicalStoryCounts(counts legacyStoryCounts) docbinary.StoryCounts {
	return docbinary.StoryCounts{
		Text: counts.Text, Ftn: counts.Ftn, Hdr: counts.Hdr, Mcr: counts.Mcr,
		Atn: counts.Atn, Edn: counts.Edn, Txbx: counts.Txbx, HdrTxbx: counts.HdrTxbx,
	}
}

func buildLegacyNoteDataFromRanges(wordStream, clxData []byte, ranges []word97.ReferencedStoryRange, chpxEntries []chpxEntry, papxEntries []papxEntry) *legacyNoteData {
	if len(ranges) == 0 {
		return nil
	}
	data := &legacyNoteData{RefByCP: make(map[int]int, len(ranges)), Stories: make(map[int]wordir.Story, len(ranges))}
	opened := canonicalLosslessStoryCore(clxData, chpxEntries, papxEntries)
	for _, item := range ranges {
		data.RefByCP[item.ReferenceCP] = item.ID
		if opened == nil {
			continue
		}
		story := materializeCanonicalLosslessStory(wordStream, opened, item.Range.StartCP, item.Range.EndCP)
		if len(story.Blocks) == 0 && len(story.Paragraphs) == 0 && len(story.Tables) == 0 {
			continue
		}
		data.Stories[item.ID] = story
	}
	if len(data.RefByCP) == 0 && len(data.Stories) == 0 {
		return nil
	}
	return data
}

// Shared compatibility FIB accessor. Several old-format adapters still use it;
// structural Word97 metadata discovery no longer does on the normal path.
func readFibFcLcbEntry(wordStream []byte, fibBase, cbRgFcLcb, index int) (int, int, bool) {
	if index >= cbRgFcLcb || fibBase+(index+1)*8 > len(wordStream) {
		return 0, 0, false
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+index*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+index*8+4:]))
	return fc, lcb, true
}

// Compatibility names retained for legacy writer/tests; binary ownership is canonical.
func parseLegacyAnnotationRefCPs(plc []byte) []int { return docbinary.ParseAnnotationRefCPs(plc) }
func parseLegacyNoteRefCPs(plc []byte) []int       { return docbinary.ParseNoteRefCPs(plc) }
func parseLegacyNoteTextCPs(plc []byte) []int      { return docbinary.ParseNoteTextCPs(plc) }

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
	return docRun{Text: text, Format: src.Format, ParaFormat: src.ParaFormat, Target: src.Target, Kind: src.Kind, NoteID: src.NoteID, StartCP: startCP, CommentID: src.CommentID, Revision: src.Revision, Spec: src.Spec, Image: src.Image}
}

func buildLegacyStoryFromRuns(runs []docRun) wordir.Story {
	doc := buildLegacyDocFromRuns("", applyDocFieldsToRuns(runs))
	return wordir.Story{Paragraphs: doc.Paragraphs, Tables: doc.Tables, Blocks: doc.Blocks, Text: normalizeLegacyText(strings.TrimSpace(joinDocRunsText(runs)))}
}
