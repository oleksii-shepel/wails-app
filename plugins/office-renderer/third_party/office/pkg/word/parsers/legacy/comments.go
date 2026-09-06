package legacy

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

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

// parseATRDPre10Records reads ATRDPre10 annotation records from PlcfAtnBkf
// (FIB index 4). Each record is 30 bytes per MS-DOC §2.9.3: 22-byte xstUsrInitl +
// 2-byte ibst + 2-byte grfbmc + 4-byte dttm. ibst/grfbmc are skipped for author/date.
func parseATRDPre10Records(tableStream, wordStream []byte, fibBase, cbRgFcLcb, noteCount int) (map[int]string, map[int]string) {
	const plcfAtnBkfIndex = 4
	const atrdPre10Size = 30
	const xstSize = 22

	fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, plcfAtnBkfIndex)
	if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
		return nil, nil
	}
	plc := tableStream[fc : fc+lcb]
	n := (lcb - 4) / (4 + atrdPre10Size)
	if n <= 0 {
		return nil, nil
	}
	if n > noteCount {
		n = noteCount
	}
	dataOffset := (n + 1) * 4
	if dataOffset+n*atrdPre10Size > len(plc) {
		return nil, nil
	}

	authorByID := make(map[int]string, n)
	dateByID := make(map[int]string, n)
	for i := 0; i < n; i++ {
		rec := plc[dataOffset+i*atrdPre10Size : dataOffset+(i+1)*atrdPre10Size]
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
		dttm := binary.LittleEndian.Uint32(rec[xstSize+4 : xstSize+8])
		if dttm != 0 {
			day := int((dttm >> 11) & 0x1F)
			month := int((dttm >> 16) & 0xF)
			year := int((dttm>>20)&0x1FF) + 1900
			if year > 1900 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
				dateByID[i+1] = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
			}
		}
	}
	return authorByID, dateByID
}
