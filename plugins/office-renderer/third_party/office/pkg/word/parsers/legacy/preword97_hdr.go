package legacy

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
)

// loadPreWord97HeaderFooterStories decodes header/footer stories from a
// pre-Word-97 (nFib=0x0065/0x0068) WordDocument stream. It uses the HDD PLC
// at FIB offset 0xB0 and the header/footer character count (ccpHdd) from
// FibRgLw95 offset +0x1C (absolute 0x3C).
//
// The pre-Word-97 plcfHdd layout has no leading separator slots. Slot indices
// map directly to header/footer types:
//
//	0 = even-page header
//	1 = odd-page (default) header
//	2 = even-page footer
//	3 = odd-page (default) footer
//	4 = first-page header
//	5 = first-page footer
func loadPreWord97HeaderFooterStories(wordStream []byte) *legacyHeaderFooterData {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok || span.CcpHdd <= 0 {
		return nil
	}
	if span.HddTableOffset <= 0 || span.HddTableSize < 8 ||
		span.HddTableOffset+span.HddTableSize > len(wordStream) ||
		span.HddTableSize%4 != 0 {
		return nil
	}

	// Header/footer story starts immediately after main text + footnote text.
	hdrStart := span.Start + span.CcpText + span.CcpFtn
	if hdrStart+span.CcpHdd > len(wordStream) {
		return nil
	}

	// HDD PLC: array of int32 CPs relative to the h/f story start.
	// Stored directly in the WordDocument stream (not in a table stream).
	hddData := wordStream[span.HddTableOffset : span.HddTableOffset+span.HddTableSize]
	cpCount := len(hddData) / 4
	if cpCount < 3 { // need at least 2 valid entries + limit marker
		return nil
	}
	cps := make([]int, cpCount)
	for i := range cps {
		cps[i] = int(int32(binary.LittleEndian.Uint32(hddData[i*4:])))
	}

	// n+2 CPs (per MS-DOC HeaderStories spec) → n valid story slots.
	n := cpCount - 2
	if n <= 0 {
		return nil
	}

	preferred := guessPreWord97Charset(wordStream, span)

	var group [6]wordir.Story
	for i := 0; i < 6 && i < n; i++ {
		cpStart := cps[i]
		cpEnd := cps[i+1]
		if cpStart < 0 || cpEnd <= cpStart {
			continue
		}
		if cpEnd > span.CcpHdd {
			cpEnd = span.CcpHdd
		}
		start := hdrStart + cpStart
		end := hdrStart + cpEnd
		if end > len(wordStream) || end <= start {
			continue
		}
		raw := decodePreWord97SingleByteTextWithCharset(wordStream[start:end], preferred)
		group[i] = buildHdrFtrStoryFromRaw(raw)
	}

	hasContent := false
	for _, s := range group {
		if s.Text != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return nil
	}

	var noteStories [6]wordir.Story // no separator stories in pre-Word-97
	return buildLegacyHeaderFooterDataFromSlots(noteStories, [][6]wordir.Story{group}, 1)
}