package legacy

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"

	wordir "office/pkg/word/ir"
)

// loadLegacySttbfRMark reads SttbfRMark from FibRgFcLcb97[51] and returns the
// indexed list of revision-mark author names. Returns nil if the FIB index,
// table-stream pointer, or STTB header is too short to be trustworthy.
//
// SttbfRMark layout (MS-DOC §2.7.5 + extended STTB format §2.7.4):
//
//	2 bytes : 0xFFFF marker (extended/Unicode strings)
//	2 bytes : cData (number of strings, uint16; cDataLength = 2 for SttbfRMark)
//	2 bytes : cbExtra (per-string trailing extra-data byte count; usually 0)
//	for each cData entries:
//	    2 bytes : cchData (string length in UTF-16 chars)
//	    cchData * 2 bytes : UTF-16LE string body
//	    cbExtra bytes : trailing extra data (skipped)
//
// Non-extended (8-bit) STTBs are not produced by Word for SttbfRMark in
// practice; we still tolerate the case by returning an empty slice rather than
// erroring.
func loadLegacySttbfRMark(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []string {
	const sttbfRMarkFibIndex = 51
	if sttbfRMarkFibIndex >= cbRgFcLcb || fibBase+(sttbfRMarkFibIndex+1)*8 > len(wordStream) {
		return nil
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+sttbfRMarkFibIndex*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+sttbfRMarkFibIndex*8+4:]))
	if lcb < 6 || fc < 0 || fc+lcb > len(tableStream) {
		return nil
	}
	buf := tableStream[fc : fc+lcb]
	pos := 0
	if binary.LittleEndian.Uint16(buf[pos:]) != 0xFFFF {
		return nil
	}
	pos += 2
	if pos+2 > len(buf) {
		return nil
	}
	cData := int(binary.LittleEndian.Uint16(buf[pos:]))
	pos += 2
	if pos+2 > len(buf) {
		return nil
	}
	cbExtra := int(binary.LittleEndian.Uint16(buf[pos:]))
	pos += 2
	out := make([]string, 0, cData)
	for i := 0; i < cData; i++ {
		if pos+2 > len(buf) {
			break
		}
		cch := int(int16(binary.LittleEndian.Uint16(buf[pos:])))
		pos += 2
		if cch < 0 {
			out = append(out, "")
			continue
		}
		if pos+cch*2 > len(buf) {
			break
		}
		runes := make([]uint16, cch)
		for j := 0; j < cch; j++ {
			runes[j] = binary.LittleEndian.Uint16(buf[pos+j*2:])
		}
		out = append(out, string(utf16.Decode(runes)))
		pos += cch * 2
		if pos+cbExtra > len(buf) {
			break
		}
		pos += cbExtra
	}
	return out
}

// formatLegacyRevisionDate decodes a 32-bit Word DTTM into ISO-8601 date.
// MS-DOC §2.9.51 DTTM packed bit layout (LSB → MSB):
//
//	mint  : 6 bits  (minute, 0–59)
//	hr    : 5 bits  (hour, 0–23)
//	dom   : 5 bits  (day-of-month, 1–31)
//	mon   : 4 bits  (month, 1–12)
//	yr    : 9 bits  (year offset from 1900)
//	wdy   : 3 bits  (day-of-week)
//
// We expose the date portion only (YYYY-MM-DD) — that's the resolution most
// Word UIs surface for tracked changes. Returns "" when the packed value is
// zero (no date set) or yields a sentinel month/day.
func formatLegacyRevisionDate(dttm uint32) string {
	if dttm == 0 {
		return ""
	}
	dom := int((dttm >> 11) & 0x1F)
	mon := int((dttm >> 16) & 0x0F)
	yr := int((dttm >> 20) & 0x1FF)
	if mon < 1 || mon > 12 || dom < 1 || dom > 31 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", 1900+yr, mon, dom)
}

// formatLegacyDttm decodes a 32-bit Word DTTM into a full ISO-8601 datetime
// string (YYYY-MM-DDTHH:MM:SS). Returns "" when the packed value is zero or
// yields an invalid date/time component.
func formatLegacyDttm(dttm uint32) string {
	if dttm == 0 {
		return ""
	}
	mint := int(dttm & 0x3F)
	hr := int((dttm >> 6) & 0x1F)
	dom := int((dttm >> 11) & 0x1F)
	mon := int((dttm >> 16) & 0x0F)
	yr := int((dttm >> 20) & 0x1FF)
	if mon < 1 || mon > 12 || dom < 1 || dom > 31 || hr > 23 || mint > 59 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d", 1900+yr, mon, dom, hr, mint, 0)
}

// resolveLegacyRevisionAuthor returns the author name at ibst, or "" if out of
// range. SttbfRMark indices are uint16; a value larger than the loaded slice
// length means the document referenced a non-existent author entry.
func resolveLegacyRevisionAuthor(authors []string, ibst uint16, has bool) string {
	if !has || int(ibst) >= len(authors) {
		return ""
	}
	return authors[ibst]
}

func resolveLegacyPropertyRevisionMark(authors []string, present bool, ibst uint16, hasIbst bool, dttm uint32, hasDttm bool) *wordir.RevisionMark {
	if !present {
		return nil
	}
	mark := &wordir.RevisionMark{}
	if hasDttm {
		mark.Date = formatLegacyRevisionDate(dttm)
	}
	if len(authors) > 0 {
		if !hasIbst {
			ibst = 0
			hasIbst = true
		}
		mark.Author = resolveLegacyRevisionAuthor(authors, ibst, hasIbst)
	}
	return mark
}

func resolveLegacyPendingRevisionMark(authors []string, pending *legacyPendingRevisionMark) *wordir.RevisionMark {
	if pending == nil {
		return nil
	}
	return resolveLegacyPropertyRevisionMark(authors, pending.Present, pending.Ibst, pending.HasIbst, pending.Dttm, pending.HasDttm)
}