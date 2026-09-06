package legacy

import docformat "office/pkg/word/parsers/doc/format"

func toSequentialRevision(mark *legacyPendingRevisionMark) *docformat.PendingRevisionMark {
	if mark == nil {
		return nil
	}
	return &docformat.PendingRevisionMark{
		Present: mark.Present,
		Ibst:    mark.Ibst,
		HasIbst: mark.HasIbst,
		Dttm:    mark.Dttm,
		HasDttm: mark.HasDttm,
	}
}

func fromSequentialRevision(mark *docformat.PendingRevisionMark) *legacyPendingRevisionMark {
	if mark == nil {
		return nil
	}
	return &legacyPendingRevisionMark{
		Present: mark.Present,
		Ibst:    mark.Ibst,
		HasIbst: mark.HasIbst,
		Dttm:    mark.Dttm,
		HasDttm: mark.HasDttm,
	}
}

func fromSequentialCHPX(entry docformat.CHPXEntry) chpxEntry {
	return chpxEntry{
		FC:                   entry.FC,
		Format:               entry.Format,
		Revision:             entry.Revision,
		Spec:                 entry.Spec,
		Data:                 entry.Data,
		Obj:                  entry.Obj,
		IbstRMark:            entry.IbstRMark,
		HasIbstRMark:         entry.HasIbstRMark,
		DttmRMark:            entry.DttmRMark,
		HasDttmRMark:         entry.HasDttmRMark,
		PropertyRevision:     fromSequentialRevision(entry.PropertyRevision),
		DisplayFieldRevision: fromSequentialRevision(entry.DisplayFieldRevision),
		Diagnostics:          entry.Diagnostics,
		Sprms:                append([]byte(nil), entry.Sprms...),
	}
}

func appendSequentialCHPX(dst []chpxEntry, src []docformat.CHPXEntry) []chpxEntry {
	for _, entry := range src {
		dst = append(dst, fromSequentialCHPX(entry))
	}
	return dst
}
