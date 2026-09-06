package word97

import (
	"fmt"

	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

type revisionCursor struct {
	inner   Cursor
	authors []string
}

// NewRevisionCursor resolves Word97 tracked-change author/date indices before
// runs enter the semantic event pipeline. Raw ibst/DTTM provenance remains on
// Run and RunSource for lossless round-tripping.
func NewRevisionCursor(inner Cursor, authors []string) Cursor {
	if inner == nil {
		return nil
	}
	return &revisionCursor{inner: inner, authors: append([]string(nil), authors...)}
}

func (c *revisionCursor) Next() (Run, bool, error) {
	run, ok, err := c.inner.Next()
	if err != nil || !ok {
		return run, ok, err
	}
	resolveRunRevisions(&run, c.authors)
	return run, true, nil
}

func resolveRunRevisions(run *Run, authors []string) {
	if run == nil {
		return
	}
	if run.HasDttmRMark {
		run.RevisionDate = formatRevisionDate(run.DttmRMark)
	}
	if run.Revision != "" && len(authors) > 0 {
		ibst := uint16(0)
		if run.HasIbstRMark {
			ibst = run.IbstRMark
		}
		run.RevisionAuthor = revisionAuthor(authors, ibst, true)
	}
	if run.HasPropRMark {
		run.ParagraphPropertyRevision = resolveRevisionMark(authors, run.PropRMarkIbst, run.HasPropRMarkIbst, run.PropRMarkDttm, run.HasPropRMarkDttm)
	}
	if run.HasRunPropRMark {
		run.RunPropertyRevision = resolveRevisionMark(authors, run.RunPropRMarkIbst, run.HasRunPropRMarkIbst, run.RunPropRMarkDttm, run.HasRunPropRMarkDttm)
	}
	if run.HasDispFldRMark {
		run.DisplayFieldRevision = resolveRevisionMark(authors, run.DispFldRMarkIbst, run.HasDispFldRMarkIbst, run.DispFldRMarkDttm, run.HasDispFldRMarkDttm)
	}
	if run.TableProps != nil && run.TableProps.PropertyRevision != nil {
		pending := run.TableProps.PropertyRevision
		run.TableProps.ResolvedPropertyRevision = resolvePendingRevisionMark(authors, pending)
	}
}

func resolvePendingRevisionMark(authors []string, pending *docformat.PendingRevisionMark) *wordir.RevisionMark {
	if pending == nil || !pending.Present {
		return nil
	}
	return resolveRevisionMark(authors, pending.Ibst, pending.HasIbst, pending.Dttm, pending.HasDttm)
}

func resolveRevisionMark(authors []string, ibst uint16, hasIbst bool, dttm uint32, hasDttm bool) *wordir.RevisionMark {
	mark := &wordir.RevisionMark{}
	if hasDttm {
		mark.Date = formatRevisionDate(dttm)
	}
	if len(authors) > 0 {
		if !hasIbst {
			ibst = 0
			hasIbst = true
		}
		mark.Author = revisionAuthor(authors, ibst, hasIbst)
	}
	return mark
}

func revisionAuthor(authors []string, ibst uint16, has bool) string {
	if !has || int(ibst) >= len(authors) {
		return ""
	}
	return authors[ibst]
}

func formatRevisionDate(dttm uint32) string {
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
