package legacy

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

type legacyNoteData struct {
	RefByCP    map[int]int
	Stories    map[int]wordir.Story
	AuthorByID map[int]string
	DateByID   map[int]string
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

func injectDocNoteRefs(runs []binaryWordRun, refByCP map[int]int, kind string) []binaryWordRun {
	if len(refByCP) == 0 {
		return runs
	}
	out := make([]binaryWordRun, 0, len(runs)+len(refByCP))
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
			out = append(out, binaryWordRun{Kind: kind, NoteID: noteID, Format: r.Format, ParaFormat: r.ParaFormat, StartCP: cp, Revision: r.Revision})
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

func cloneDocRunWithText(src binaryWordRun, text string, startCP int) binaryWordRun {
	return binaryWordRun{Text: text, Format: src.Format, ParaFormat: src.ParaFormat, Target: src.Target, Kind: src.Kind, NoteID: src.NoteID, StartCP: startCP, CommentID: src.CommentID, Revision: src.Revision, Spec: src.Spec, Image: src.Image}
}
