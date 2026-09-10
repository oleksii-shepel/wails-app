package legacy

import (
	docformat "office/pkg/word/parsers/doc/format"
	word97 "office/pkg/word/parsers/doc/word97"
)

// adaptCanonicalWord97Runs is the single collection-level semantic adapter
// from canonical Word97 runs to the format-neutral binary Word assembler.
// Binary parsing and Word97 enrichment remain on the canonical side.
func adaptCanonicalWord97Runs(src []word97.Run) []binaryWordRun {
	if len(src) == 0 {
		return nil
	}
	out := make([]binaryWordRun, 0, len(src))
	for _, run := range src {
		out = append(out, adaptCanonicalWord97Run(run))
	}
	return out
}

// adaptCanonicalWord97Run maps canonical Word97 semantics into the shared
// binary Word assembly model. It performs no binary decoding. Materialized
// fields already use the version-neutral docformat carrier; only their run
// payloads cross the remaining Word97 -> binary Word run boundary here.
func adaptCanonicalWord97Run(r word97.Run) binaryWordRun {
	return binaryWordRun{
		Text:                 r.Text,
		Kind:                 r.Kind,
		Format:               r.Format,
		ParaFormat:           r.ParagraphFormat,
		TableDepth:           r.TableDepth,
		LegacyInTableHint:    r.LegacyInTableHint,
		LegacyRowMarkHint:    r.LegacyRowMarkHint,
		TableProps:           r.TableProps.Clone(),
		StartCP:              r.StartCP,
		Revision:             r.Revision,
		RevisionAuthor:       r.RevisionAuthor,
		RevisionDate:         r.RevisionDate,
		PropertyRevision:     r.ParagraphPropertyRevision,
		RunPropertyRevision:  r.RunPropertyRevision,
		DisplayFieldRevision: r.DisplayFieldRevision,
		FieldKind:            r.FieldKind,
		FieldInstr:           r.FieldInstr,
		FieldArg:             r.FieldArg,
		Target:               r.Target,
		Anchor:               r.Anchor,
		NoteID:               r.NoteID,
		CommentID:            r.CommentID,
		Image:                r.Image,
		Shape:                r.Shape,
		EmbeddedRun:          r.EmbeddedRun,
		FormField:            r.FormField,
		Fields:               adaptCanonicalMaterializedFields(r.Fields),
		IbstRMark:            r.IbstRMark,
		HasIbstRMark:         r.HasIbstRMark,
		DttmRMark:            r.DttmRMark,
		HasDttmRMark:         r.HasDttmRMark,
		PropRMarkIbst:        r.PropRMarkIbst,
		HasPropRMarkIbst:     r.HasPropRMarkIbst,
		PropRMarkDttm:        r.PropRMarkDttm,
		HasPropRMarkDttm:     r.HasPropRMarkDttm,
		HasPropRMark:         r.HasPropRMark,
		RunPropRMarkIbst:     r.RunPropRMarkIbst,
		HasRunPropRMarkIbst:  r.HasRunPropRMarkIbst,
		RunPropRMarkDttm:     r.RunPropRMarkDttm,
		HasRunPropRMarkDttm:  r.HasRunPropRMarkDttm,
		HasRunPropRMark:      r.HasRunPropRMark,
		DispFldRMarkIbst:     r.DispFldRMarkIbst,
		HasDispFldRMarkIbst:  r.HasDispFldRMarkIbst,
		DispFldRMarkDttm:     r.DispFldRMarkDttm,
		HasDispFldRMarkDttm:  r.HasDispFldRMarkDttm,
		HasDispFldRMark:      r.HasDispFldRMark,
		Spec:                 r.Spec,
		Data:                 r.Data,
		Obj:                  r.Obj,
		ChpxSprms:            r.ChpxSprms,
		PapxSprms:            r.PapxSprms,
	}
}

func adaptCanonicalMaterializedFields(src []*docformat.MaterializedField[word97.Run]) []*binaryWordField {
	if len(src) == 0 {
		return nil
	}
	out := make([]*binaryWordField, 0, len(src))
	for _, f := range src {
		if f == nil {
			continue
		}
		out = append(out, &binaryWordField{
			Kind:      f.Kind,
			Instr:     f.Instr,
			Arg:       f.Arg,
			Anchor:    f.Anchor,
			Target:    f.Target,
			Children:  adaptCanonicalWord97Runs(f.Children),
			InstrRuns: adaptCanonicalWord97Runs(f.InstrRuns),
		})
	}
	return out
}
