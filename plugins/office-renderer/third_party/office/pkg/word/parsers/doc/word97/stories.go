package word97

import (
	"io"

	docbinary "office/pkg/word/parsers/doc/binary"
)

// StoryRange is an absolute WordDocument CP interval.
type StoryRange struct {
	StartCP int
	EndCP   int
}

// ReferencedStoryRange binds a main-story reference CP to the corresponding
// side-story interval and stable one-based semantic ID.
type ReferencedStoryRange struct {
	ID          int
	ReferenceCP int
	Range       StoryRange
}

// SideStoryRanges describes footnote, endnote and comment ranges without
// materializing their content.
type SideStoryRanges struct {
	Footnotes []ReferencedStoryRange
	Endnotes  []ReferencedStoryRange
	Comments  []ReferencedStoryRange
}

// HeaderFooterRanges describes the six separator stories followed by six
// header/footer slots per section in PlcfHdd order.
type HeaderFooterRanges struct {
	NoteStories  [6]StoryRange
	Sections     [][6]StoryRange
	SectionCount int
}

// LoadSideStoryRanges discovers all fixed Word97 side-story PLC pairs and
// projects their relative CPs to absolute WordDocument story ranges.
func LoadSideStoryRanges(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, counts docbinary.StoryCounts) SideStoryRanges {
	footnotes := loadReferencedStoryRanges(table, tableSize, fib, layout, 2, 3, counts.Text, false)
	endnoteStart := counts.Text + counts.Ftn + counts.Hdr + counts.Mcr + counts.Atn
	endnotes := loadReferencedStoryRanges(table, tableSize, fib, layout, 46, 47, endnoteStart, false)
	if len(endnotes) == 0 {
		endnotes = loadReferencedStoryRanges(table, tableSize, fib, layout, 34, 35, endnoteStart, false)
	}
	commentStart := counts.Text + counts.Ftn + counts.Hdr + counts.Mcr
	comments := loadReferencedStoryRanges(table, tableSize, fib, layout, 4, 5, commentStart, true)
	if len(comments) == 0 {
		comments = loadReferencedStoryRanges(table, tableSize, fib, layout, 20, 21, commentStart, true)
	}

	endnotes = boundReferencedStoryRanges(endnotes, endnoteStart, counts.Edn)
	comments = boundReferencedStoryRanges(comments, commentStart, counts.Atn)
	return SideStoryRanges{Footnotes: footnotes, Endnotes: endnotes, Comments: comments}
}

func boundReferencedStoryRanges(ranges []ReferencedStoryRange, storyStart, storyLen int) []ReferencedStoryRange {
	if len(ranges) == 0 {
		return nil
	}
	// Some valid/older producers omit the corresponding ccp* length while still
	// publishing a complete PLC pair. In that case the PLC itself is the only
	// usable bound, so retain it rather than discarding discovered stories.
	if storyLen <= 0 {
		return ranges
	}
	storyEnd := storyStart + storyLen
	for _, item := range ranges {
		if item.Range.StartCP < storyStart || item.Range.EndCP < item.Range.StartCP || item.Range.EndCP > storyEnd {
			return nil
		}
	}
	return ranges
}

func loadReferencedStoryRanges(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, refIndex, textIndex, storyStart int, annotation bool) []ReferencedStoryRange {
	refBlob, ok := readTableRange(table, tableSize, fib, layout, refIndex, 4)
	if !ok {
		return nil
	}
	textBlob, ok := readTableRange(table, tableSize, fib, layout, textIndex, 8)
	if !ok {
		return nil
	}
	var refs []int
	if annotation {
		refs = docbinary.ParseAnnotationRefCPs(refBlob)
		if len(refs) == 0 {
			refs = docbinary.ParseNoteRefCPs(refBlob)
		}
	} else {
		refs = docbinary.ParseNoteRefCPs(refBlob)
	}
	text := docbinary.ParseNoteTextCPs(textBlob)
	if len(refs) == 0 || len(text) < 2 {
		return nil
	}
	count := len(refs)
	if len(text)-1 < count {
		count = len(text) - 1
	}
	out := make([]ReferencedStoryRange, 0, count)
	for i := 0; i < count; i++ {
		start, end := storyStart+text[i], storyStart+text[i+1]
		if end < start {
			end = start
		}
		out = append(out, ReferencedStoryRange{ID: i + 1, ReferenceCP: refs[i], Range: StoryRange{StartCP: start, EndCP: end}})
	}
	return out
}

// LoadHeaderFooterRanges parses PlcfHdd and returns absolute CP intervals for
// separator stories and per-section header/footer slots. Empty/inherited slots
// remain zero-length ranges and are interpreted by the projection layer.
func LoadHeaderFooterRanges(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, counts docbinary.StoryCounts, sectionCount int) *HeaderFooterRanges {
	if counts.Hdr <= 0 {
		return nil
	}
	plc, ok := readTableRange(table, tableSize, fib, layout, 11, 32)
	if !ok {
		return nil
	}
	cps := docbinary.ParseCPArray(plc)
	if len(cps) < 8 {
		return nil
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
	if groupCount > sectionCount {
		sectionCount = groupCount
	}
	out := &HeaderFooterRanges{Sections: make([][6]StoryRange, groupCount), SectionCount: sectionCount}
	hdrStart := counts.Text + counts.Ftn
	for i := 0; i < 6 && i < storyCount; i++ {
		out.NoteStories[i] = boundedHeaderRange(cps[i], cps[i+1], counts.Hdr, hdrStart)
	}
	for group := 0; group < groupCount; group++ {
		for slot := 0; slot < 6; slot++ {
			idx := 6 + group*6 + slot
			if idx+1 >= len(cps) {
				break
			}
			out.Sections[group][slot] = boundedHeaderRange(cps[idx], cps[idx+1], counts.Hdr, hdrStart)
		}
	}
	return out
}

func boundedHeaderRange(start, end, max, base int) StoryRange {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > max {
		end = max
	}
	return StoryRange{StartCP: base + start, EndCP: base + end}
}

// SectionRecord couples a PlcfSed descriptor with its bounded SEPX grpprl.
type SectionRecord struct {
	Descriptor docbinary.SectionDescriptor
	Sepx       []byte
}

// LoadSectionRecords reads PlcfSed and each referenced SEPX from WordDocument.
// It owns section range/SEPX I/O while format projection remains a separate layer.
func LoadSectionRecords(word io.ReaderAt, wordSize int64, table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) []SectionRecord {
	plc, ok := readTableRange(table, tableSize, fib, layout, 6, 20)
	if !ok {
		return nil
	}
	desc := docbinary.ParseSectionDescriptors(plc)
	if len(desc) == 0 {
		return nil
	}
	out := make([]SectionRecord, 0, len(desc))
	for _, d := range desc {
		record := SectionRecord{Descriptor: d}
		if d.HasSepx {
			if grpprl, ok := ReadSepxGrpprl(word, wordSize, int64(d.SepxOffset)); ok {
				record.Sepx = grpprl
			}
		}
		out = append(out, record)
	}
	return out
}

// ReadSepxGrpprl reads the length-prefixed SEPX property block from the
// WordDocument stream without retaining unrelated bytes.
func ReadSepxGrpprl(word io.ReaderAt, wordSize, off int64) ([]byte, bool) {
	if word == nil || off < 0 || off+2 > wordSize {
		return nil, false
	}
	var hdr [2]byte
	n, err := word.ReadAt(hdr[:], off)
	if n != 2 || (err != nil && err != io.EOF) {
		return nil, false
	}
	cb := int64(uint16(hdr[0]) | uint16(hdr[1])<<8)
	if cb <= 0 || off+2+cb > wordSize {
		return nil, false
	}
	buf := make([]byte, int(cb))
	n, err = word.ReadAt(buf, off+2)
	if n != len(buf) || (err != nil && err != io.EOF) {
		return nil, false
	}
	return buf, true
}
