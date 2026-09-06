package legacy

// resolveCanonicalWord97RunRevisions resolves revision metadata directly on the
// materialized run slice. Unlike the old full-buffer assembler, canonical
// Word97 materialization clones table-row metadata per run, so resolution must
// not rely on mutating shared legacy PAPX entry pointers.
func resolveCanonicalWord97RunRevisions(runs []docRun, authors []string) {
	for i := range runs {
		run := &runs[i]
		if run.HasDttmRMark {
			run.RevisionDate = formatLegacyRevisionDate(run.DttmRMark)
		}
		if run.HasPropRMark {
			run.PropertyRevision = resolveLegacyPropertyRevisionMark(
				authors,
				true,
				run.PropRMarkIbst,
				run.HasPropRMarkIbst,
				run.PropRMarkDttm,
				run.HasPropRMarkDttm,
			)
		}
		if run.HasRunPropRMark {
			run.RunPropertyRevision = resolveLegacyPropertyRevisionMark(
				authors,
				true,
				run.RunPropRMarkIbst,
				run.HasRunPropRMarkIbst,
				run.RunPropRMarkDttm,
				run.HasRunPropRMarkDttm,
			)
		}
		if run.HasDispFldRMark {
			run.DisplayFieldRevision = resolveLegacyPropertyRevisionMark(
				authors,
				true,
				run.DispFldRMarkIbst,
				run.HasDispFldRMarkIbst,
				run.DispFldRMarkDttm,
				run.HasDispFldRMarkDttm,
			)
		}
		if len(authors) > 0 && run.Revision != "" {
			ibst := uint16(0)
			if run.HasIbstRMark {
				ibst = run.IbstRMark
			}
			run.RevisionAuthor = resolveLegacyRevisionAuthor(authors, ibst, true)
		}
		if run.TableProps != nil && run.TableProps.propertyRevision != nil {
			run.TableProps.resolvedPropertyRevision = resolveLegacyPendingRevisionMark(authors, run.TableProps.propertyRevision)
		}
	}
}
