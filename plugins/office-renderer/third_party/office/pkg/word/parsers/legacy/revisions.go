package legacy

import (
	"fmt"

	wordir "office/pkg/word/ir"
)

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
