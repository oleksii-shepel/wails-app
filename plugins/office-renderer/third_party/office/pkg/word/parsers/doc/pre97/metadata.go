package pre97

import (
	"encoding/binary"
	"strings"
	"unicode"
)

// StoryRange is an absolute character range relative to the start of the
// WordDocument text area (Span.Start). EndCP is exclusive.
type StoryRange struct {
	StartCP int
	EndCP   int
}

// ReferencedStoryRange binds a main-story reference CP to one side-story range.
type ReferencedStoryRange struct {
	ID          int
	ReferenceCP int
	Range       StoryRange
}

// StoryRanges contains the canonical side-story plan for Word 6/95.
type StoryRanges struct {
	Footnotes []ReferencedStoryRange
	Endnotes  []ReferencedStoryRange
	Comments  []ReferencedStoryRange
}

// BookmarkData is the canonical bookmark event plan. Ends are keyed by CP so
// callers do not need to invert a name->CP map while building a stream cursor.
type BookmarkData struct {
	Starts map[int][]string
	Ends   map[int][]string
}

// HeaderFooterRanges contains the six classic Word header/footer slots.
// Slots: even header, odd header, even footer, odd footer, first header,
// first footer.
type HeaderFooterRanges struct {
	Slots [6]StoryRange
}

// SectionRecord preserves the section CP bounds and raw old-format SEPX bytes.
// Semantic SEPX projection can evolve independently from this structural plan.
type SectionRecord struct {
	StartCP int
	EndCP   int
	Sepx    []byte
}

// LoadMetadata reads bounded structural metadata from the WordDocument stream.
// It intentionally does not materialize stories or interpret SEPX semantics.
func LoadMetadata(word []byte, span Span) (BookmarkData, StoryRanges, HeaderFooterRanges, []SectionRecord) {
	return LoadBookmarks(word, span), LoadStoryRanges(word, span), LoadHeaderFooterRanges(word, span), LoadSectionRecords(word, span)
}

func LoadStoryRanges(word []byte, span Span) StoryRanges {
	var out StoryRanges
	out.Footnotes = loadReferencedRanges(word, span.FtnRefTableOffset, span.FtnRefTableSize, span.FtnTxtTableOffset, span.FtnTxtTableSize, span.CcpText)
	atnStart := span.CcpText + span.CcpFtn + span.CcpHdd + span.CcpMcr
	out.Comments = loadReferencedRanges(word, span.AtnRefTableOffset, span.AtnRefTableSize, span.AtnTxtTableOffset, span.AtnTxtTableSize, atnStart)
	endStart := span.CcpText + span.CcpFtn + span.CcpHdd + span.CcpMcr + span.CcpAtn
	out.Endnotes = loadReferencedRanges(word, span.EdnRefTableOffset, span.EdnRefTableSize, span.EdnTxtTableOffset, span.EdnTxtTableSize, endStart)
	return out
}

func loadReferencedRanges(word []byte, refOff, refSize, textOff, textSize, storyStart int) []ReferencedStoryRange {
	ref := bounded(word, refOff, refSize)
	text := bounded(word, textOff, textSize)
	if len(ref) == 0 || len(text) == 0 || storyStart < 0 {
		return nil
	}
	refs := parseNoteRefCPs(ref)
	cps := parseNoteTextCPs(text)
	if len(refs) == 0 || len(cps) < 2 {
		return nil
	}
	n := len(refs)
	if len(cps)-1 < n {
		n = len(cps) - 1
	}
	out := make([]ReferencedStoryRange, 0, n)
	for i := 0; i < n; i++ {
		start, end := storyStart+cps[i], storyStart+cps[i+1]
		if start < 0 || end <= start {
			continue
		}
		out = append(out, ReferencedStoryRange{ID: i + 1, ReferenceCP: refs[i], Range: StoryRange{StartCP: start, EndCP: end}})
	}
	return out
}

func LoadHeaderFooterRanges(word []byte, span Span) HeaderFooterRanges {
	var out HeaderFooterRanges
	plc := bounded(word, span.HddTableOffset, span.HddTableSize)
	if span.CcpHdd <= 0 || len(plc) < 12 || len(plc)%4 != 0 {
		return out
	}
	count := len(plc) / 4
	if count < 3 {
		return out
	}
	cps := make([]int, count)
	for i := range cps {
		cps[i] = int(int32(binary.LittleEndian.Uint32(plc[i*4 : i*4+4])))
	}
	storyStart := span.CcpText + span.CcpFtn
	n := count - 2
	for i := 0; i < len(out.Slots) && i < n; i++ {
		start, end := cps[i], cps[i+1]
		if start < 0 || end <= start {
			continue
		}
		if end > span.CcpHdd {
			end = span.CcpHdd
		}
		if end > start {
			out.Slots[i] = StoryRange{StartCP: storyStart + start, EndCP: storyStart + end}
		}
	}
	return out
}

func LoadSectionRecords(word []byte, span Span) []SectionRecord {
	plc := bounded(word, span.SedTableOffset, span.SedTableSize)
	if len(plc) < 20 {
		return nil
	}
	n := (len(plc) - 4) / 16
	if n <= 0 || (n+1)*4+n*12 > len(plc) {
		return nil
	}
	out := make([]SectionRecord, 0, n)
	for i := 0; i < n; i++ {
		start := int(binary.LittleEndian.Uint32(plc[i*4 : i*4+4]))
		end := int(binary.LittleEndian.Uint32(plc[(i+1)*4 : (i+1)*4+4]))
		sedOff := (n+1)*4 + i*12
		fcSepx := binary.LittleEndian.Uint32(plc[sedOff+2 : sedOff+6])
		var grpprl []byte
		if fcSepx != 0xffffffff {
			off := int(fcSepx)
			if off >= 0 && off+2 <= len(word) {
				cb := int(binary.LittleEndian.Uint16(word[off : off+2]))
				if cb >= 0 && off+2+cb <= len(word) {
					grpprl = append([]byte(nil), word[off+2:off+2+cb]...)
				}
			}
		}
		out = append(out, SectionRecord{StartCP: start, EndCP: end, Sepx: grpprl})
	}
	return out
}

func LoadBookmarks(word []byte, span Span) BookmarkData {
	var out BookmarkData
	starts := parseBookmarkStartPLC(bounded(word, span.BkmkStartTableOffset, span.BkmkStartTableSize), span.CcpText)
	names := parseBookmarkNameTable(bounded(word, span.BkmkNameTableOffset, span.BkmkNameTableSize))
	if len(starts) == 0 || len(starts) != len(names) {
		return out
	}
	ends := parseBookmarkEndPLC(bounded(word, span.BkmkEndTableOffset, span.BkmkEndTableSize), len(starts), span.CcpText)
	out.Starts = make(map[int][]string, len(starts))
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out.Starts[starts[i]] = append(out.Starts[starts[i]], name)
		if i < len(ends) {
			if out.Ends == nil {
				out.Ends = make(map[int][]string, len(ends))
			}
			out.Ends[ends[i]] = append(out.Ends[ends[i]], name)
		}
	}
	if len(out.Starts) == 0 {
		out.Starts = nil
	}
	return out
}

func bounded(word []byte, off, size int) []byte {
	if off <= 0 || size <= 0 || off > len(word) || size > len(word)-off {
		return nil
	}
	return word[off : off+size]
}

func parseNoteRefCPs(plc []byte) []int {
	switch {
	case len(plc) >= 10 && (len(plc)-4)%6 == 0:
		n := (len(plc) - 4) / 6
		out := make([]int, n)
		for i := range out { out[i] = int(binary.LittleEndian.Uint32(plc[i*4 : i*4+4])) }
		return out
	case len(plc) >= 8 && len(plc)%4 == 0:
		n := len(plc)/4 - 1
		out := make([]int, n)
		for i := range out { out[i] = int(binary.LittleEndian.Uint32(plc[i*4 : i*4+4])) }
		return out
	default:
		return nil
	}
}

func parseNoteTextCPs(plc []byte) []int {
	if len(plc) < 8 || len(plc)%4 != 0 { return nil }
	n := len(plc)/4 - 1
	if n <= 0 { return nil }
	out := make([]int, n+1)
	for i := range out { out[i] = int(binary.LittleEndian.Uint32(plc[i*4 : i*4+4])) }
	return out
}

func parseBookmarkStartPLC(plc []byte, maxCP int) []int {
	if len(plc) < 12 || (len(plc)-4)%8 != 0 { return nil }
	n := (len(plc)-4)/8
	out := make([]int, n)
	for i := range out {
		cp := int(binary.LittleEndian.Uint32(plc[i*4:i*4+4]))
		if cp < 0 || cp > maxCP || (i > 0 && cp < out[i-1]) { return nil }
		out[i] = cp
	}
	return out
}

func parseBookmarkEndPLC(plc []byte, n, maxCP int) []int {
	if n <= 0 || len(plc) < (n+1)*4 { return nil }
	out := make([]int, n)
	for i := range out {
		cp := int(binary.LittleEndian.Uint32(plc[i*4:i*4+4]))
		if cp < 0 || cp > maxCP { return nil }
		out[i] = cp
	}
	return out
}

func parseBookmarkNameTable(data []byte) []string {
	if len(data) < 6 { return nil }
	extended := binary.LittleEndian.Uint16(data[0:2])
	count := int(binary.LittleEndian.Uint16(data[2:4]))
	extra := int(binary.LittleEndian.Uint16(data[4:6]))
	if count <= 0 || count > 4096 || extra < 0 || extra > 64 { return nil }
	pos := 6
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if pos+2 > len(data) { return nil }
		cch := int(binary.LittleEndian.Uint16(data[pos:pos+2])); pos += 2
		if cch < 0 || cch > 1024 { return nil }
		byteLen := cch
		if extended == 0xffff { byteLen *= 2 }
		if pos+byteLen+extra > len(data) { return nil }
		var name string
		if extended == 0xffff {
			runes := make([]rune, 0, cch)
			for j := 0; j+1 < byteLen; j += 2 { runes = append(runes, rune(binary.LittleEndian.Uint16(data[pos+j:pos+j+2]))) }
			name = string(runes)
		} else {
			name = decodeWin1252(data[pos:pos+byteLen])
		}
		if !validBookmarkName(name) { return nil }
		out = append(out, name)
		pos += byteLen + extra
	}
	return out
}

func validBookmarkName(name string) bool {
	if name == "" || len(name) > 256 { return false }
	for i, r := range name {
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) { return false }
		if i == 0 && unicode.IsDigit(r) { return false }
	}
	return true
}
