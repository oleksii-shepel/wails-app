package legacy

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
)

func loadDocComments(wordStream, tableStream, clxData []byte, fibBase, cbRgFcLcb int, counts legacyStoryCounts, chpxEntries []chpxEntry, papxEntries []papxEntry) *legacyNoteData {
	// MS-DOC FibRgFcLcb97 fixed indices for annotation PLCs (PLCFANDREF=4, PLCFANDTXT=5).
	// Try these first so files that have correct PLCs but ccpAtn=0 (e.g. LibreOffice)
	// are handled, then fall back to the heuristic scan when ccpAtn is known.
	const commentRefIndex = 4
	const commentTextIndex = 5
	storyStart := counts.Text + counts.Ftn + counts.Hdr + counts.Mcr
	if refFC, refLCB, ok1 := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, commentRefIndex); ok1 && refLCB > 0 && refFC >= 0 && refFC+refLCB <= len(tableStream) {
		if textFC, textLCB, ok2 := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, commentTextIndex); ok2 && textLCB > 0 && textFC >= 0 && textFC+textLCB <= len(tableStream) {
			refCPs := parseLegacyAnnotationRefCPs(tableStream[refFC : refFC+refLCB])
			textCPs := parseLegacyNoteTextCPs(tableStream[textFC : textFC+textLCB])
			if len(refCPs) > 0 && len(textCPs) >= 2 {
				// Some endnote-only fixtures expose endnote PLC-like ranges at comment
				// slots while ccpAtn remains zero. Reject this pattern to avoid
				// misclassifying endnote story text as comments.
				if isLikelyEndnotePLCInCommentSlots(counts, textCPs) {
					return nil
				}
				noteCount := len(refCPs)
				if len(textCPs)-1 < noteCount {
					noteCount = len(textCPs) - 1
				}
				if noteCount > 0 {
					data := buildLegacyNoteData(wordStream, clxData, storyStart, refCPs[:noteCount], textCPs[:noteCount+1], chpxEntries, papxEntries)
					if data != nil {
						authorByID, dateByID := parseATRDPre10Records(tableStream, wordStream, fibBase, cbRgFcLcb, noteCount)
						if len(authorByID) > 0 {
							data.AuthorByID = authorByID
						}
						if len(dateByID) > 0 {
							data.DateByID = dateByID
						}
						return data
					}
				}
			}
		}
	}
	refCPs, textCPs, ok := findLegacyNotePLCPair(wordStream, tableStream, fibBase, cbRgFcLcb, counts.Text, counts.Atn, legacyCommentPLCFExcludedIndexes())
	if !ok && counts.Atn <= 0 {
		// Some fixtures (e.g. LibreOffice-generated) leave ccpAtn at zero while still
		// carrying annotation PLCs. Try searching all FIB entries for matching ref/text pairs
		// without requiring a known story length.
		refCPs, textCPs, ok = findLegacyNotePLCPairAnyStoryLen(wordStream, tableStream, fibBase, cbRgFcLcb, counts.Text, legacyCommentPLCFExcludedIndexes())
	}
	if !ok {
		return nil
	}
	if isLikelyEndnotePLCInCommentSlots(counts, textCPs) {
		return nil
	}
	noteCount := len(refCPs)
	if len(textCPs)-1 < noteCount {
		noteCount = len(textCPs) - 1
	}
	if noteCount <= 0 {
		return nil
	}
	data := buildLegacyNoteData(wordStream, clxData, storyStart, refCPs[:noteCount], textCPs[:noteCount+1], chpxEntries, papxEntries)
	if data == nil {
		return nil
	}
	// Try to read ATRDPre10 records from PlcfAtnBkf (FIB index 4).
	authorByID, dateByID := parseATRDPre10Records(tableStream, wordStream, fibBase, cbRgFcLcb, noteCount)
	if len(authorByID) > 0 {
		data.AuthorByID = authorByID
	}
	if len(dateByID) > 0 {
		data.DateByID = dateByID
	}
	return data
}

// legacyCommentPLCFExcludedIndexes returns FIB slots whose PLC layouts can
// resemble annotation ref/text PLCs but belong to other stories. Textbox PLCs
// are especially dangerous when ccpAtn==0: PlcftxbxTxt starts at CP 0 and can
// otherwise be selected by the any-story heuristic, causing textbox contents
// to reappear as bogus comments in OfficeArt-heavy documents.
func legacyCommentPLCFExcludedIndexes() map[int]bool {
	return map[int]bool{
		2: true, 3: true, // footnotes
		34: true, 35: true, // endnotes
		56: true, 57: true, // main-story textbox text / break descriptors
		58: true, 59: true, // header textbox text / break descriptors
	}
}

func isLikelyEndnotePLCInCommentSlots(counts legacyStoryCounts, textCPs []int) bool {
	if counts.Atn != 0 || counts.Edn <= 0 || len(textCPs) < 2 {
		return false
	}
	if textCPs[0] != 0 {
		return false
	}
	return textCPs[len(textCPs)-1] == counts.Edn
}

func injectDocCommentRefs(runs []docRun, refByCP map[int]int) []docRun {
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
			commentID, ok := refByCP[cp]
			if ok {
				// PLCFANDREF CPs are insertion anchors in the main story; the
				// annotation reference is not a story character. Preserve the rune
				// at the anchor CP and insert the synthetic reference before it.
				handled = true
				if i > segmentStart {
					out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:i]), segmentStartCP))
				}
				out = append(out, docRun{Kind: "commentRef", CommentID: commentID, Format: r.Format, ParaFormat: r.ParaFormat, StartCP: cp, Revision: r.Revision})
				segmentStart = i
				segmentStartCP = cp
			}
			cp++
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

func commentStoriesFromNoteData(data *legacyNoteData) map[int]wordir.Story {
	if data == nil || len(data.Stories) == 0 {
		return nil
	}
	out := make(map[int]wordir.Story, len(data.Stories))
	for id, story := range data.Stories {
		if author, ok := data.AuthorByID[id]; ok {
			story.Author = author
		}
		if date, ok := data.DateByID[id]; ok {
			story.Date = date
		}
		out[id] = story
	}
	return out
}

// parseATRDPre10Records reads ATRDPre10 annotation records from PlcfAtnBkf
// (FIB index 4). Each record is 30 bytes per MS-DOC §2.9.3: 22-byte xstUsrInitl +
// 2-byte ibst + 2-byte grfbmc + 4-byte dttm. ibst/grfbmc are skipped for author/date.
// Returns maps from 1-based comment ID to author initials and ISO date string.
func parseATRDPre10Records(tableStream, wordStream []byte, fibBase, cbRgFcLcb, noteCount int) (map[int]string, map[int]string) {
	const plcfAtnBkfIndex = 4
	const atrdPre10Size = 30 // 22-byte xstUsrInitl + 2-byte ibst + 2-byte grfbmc + 4-byte dttm
	const xstSize = 22       // 2-byte cch + 20-byte (10 × UTF-16LE)

	fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, plcfAtnBkfIndex)
	if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
		return nil, nil
	}
	plc := tableStream[fc : fc+lcb]
	// PLC layout: (N+1)*4 bytes of CPs, then N*30 bytes of ATRDPre10 records.
	// Infer record count from PLC size, cross-check against noteCount.
	n := (lcb - 4) / (4 + atrdPre10Size)
	if n <= 0 {
		return nil, nil
	}
	if n > noteCount {
		n = noteCount
	}
	dataOffset := (n + 1) * 4 // CPs occupy (n+1)*4 bytes
	if dataOffset+n*atrdPre10Size > len(plc) {
		return nil, nil
	}

	authorByID := make(map[int]string, n)
	dateByID := make(map[int]string, n)
	for i := 0; i < n; i++ {
		rec := plc[dataOffset+i*atrdPre10Size : dataOffset+(i+1)*atrdPre10Size]
		// Parse xstUsrInitl: cch (2 bytes LE) followed by up to 10 UTF-16LE chars.
		cch := int(binary.LittleEndian.Uint16(rec[0:2]))
		if cch > 10 {
			cch = 10
		}
		if cch > 0 {
			u16 := make([]uint16, cch)
			for j := 0; j < cch; j++ {
				u16[j] = binary.LittleEndian.Uint16(rec[2+j*2 : 4+j*2])
			}
			authorByID[i+1] = string(utf16.Decode(u16))
		}
		// Parse DTTM (4 bytes at offset xstSize+4, after ibst+grfbmc).
		dttm := binary.LittleEndian.Uint32(rec[xstSize+4 : xstSize+8])
		if dttm != 0 {
			minutes := int(dttm & 0x3F)
			hour := int((dttm >> 6) & 0x1F)
			day := int((dttm >> 11) & 0x1F)
			month := int((dttm >> 16) & 0xF)
			year := int((dttm>>20)&0x1FF) + 1900
			_ = minutes
			_ = hour
			if year > 1900 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
				dateByID[i+1] = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
			}
		}
	}
	return authorByID, dateByID
}
