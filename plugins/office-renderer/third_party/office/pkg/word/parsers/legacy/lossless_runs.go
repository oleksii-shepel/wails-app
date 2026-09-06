package legacy

import (
	"bytes"

	word97 "office/pkg/word/parsers/doc/word97"
)

// materializeCanonicalLosslessRuns adapts one CP range from the canonical
// Word97 piece/format/revision reader back into the temporary legacy run shape
// used by the remaining lossless enrichment pipeline. CLX/PieceTable/FKP and
// tracked-change attribution ownership are entirely canonical.
func materializeCanonicalLosslessRuns(wordStream, clxData []byte, startCP, endCP int, chpxEntries []chpxEntry, papxEntries []papxEntry, revisionAuthors []string) []docRun {
	if startCP >= endCP {
		return nil
	}
	canonical := word97.MaterializeResolvedRunRangeFromCLX(
		bytes.NewReader(wordStream),
		int64(len(wordStream)),
		clxData,
		toSequentialCHPXEntries(chpxEntries),
		toSequentialPAPXEntries(papxEntries),
		revisionAuthors,
		word97.StoryRange{StartCP: startCP, EndCP: endCP},
	)
	if len(canonical) == 0 {
		return nil
	}
	out := make([]docRun, 0, len(canonical))
	for _, run := range canonical {
		out = append(out, legacyRunFromCanonicalWord97(run))
	}
	return out
}

func legacyRunFromCanonicalWord97(r word97.Run) docRun {
	return docRun{
		Text: r.Text,
		Kind: r.Kind,
		Format: r.Format,
		ParaFormat: r.ParagraphFormat,
		TableDepth: r.TableDepth,
		LegacyInTableHint: r.LegacyInTableHint,
		LegacyRowMarkHint: r.LegacyRowMarkHint,
		TableProps: legacyTableRowPropsFromCanonical(r.TableProps),
		StartCP: r.StartCP,
		Revision: r.Revision,
		RevisionAuthor: r.RevisionAuthor,
		RevisionDate: r.RevisionDate,
		PropertyRevision: r.ParagraphPropertyRevision,
		RunPropertyRevision: r.RunPropertyRevision,
		DisplayFieldRevision: r.DisplayFieldRevision,
		FieldKind: r.FieldKind,
		FieldInstr: r.FieldInstr,
		FieldArg: r.FieldArg,
		Target: r.Target,
		Anchor: r.Anchor,
		NoteID: r.NoteID,
		CommentID: r.CommentID,
		Image: r.Image,
		Shape: r.Shape,
		IbstRMark: r.IbstRMark,
		HasIbstRMark: r.HasIbstRMark,
		DttmRMark: r.DttmRMark,
		HasDttmRMark: r.HasDttmRMark,
		PropRMarkIbst: r.PropRMarkIbst,
		HasPropRMarkIbst: r.HasPropRMarkIbst,
		PropRMarkDttm: r.PropRMarkDttm,
		HasPropRMarkDttm: r.HasPropRMarkDttm,
		HasPropRMark: r.HasPropRMark,
		RunPropRMarkIbst: r.RunPropRMarkIbst,
		HasRunPropRMarkIbst: r.HasRunPropRMarkIbst,
		RunPropRMarkDttm: r.RunPropRMarkDttm,
		HasRunPropRMarkDttm: r.HasRunPropRMarkDttm,
		HasRunPropRMark: r.HasRunPropRMark,
		DispFldRMarkIbst: r.DispFldRMarkIbst,
		HasDispFldRMarkIbst: r.HasDispFldRMarkIbst,
		DispFldRMarkDttm: r.DispFldRMarkDttm,
		HasDispFldRMarkDttm: r.HasDispFldRMarkDttm,
		HasDispFldRMark: r.HasDispFldRMark,
		Spec: r.Spec,
		Data: r.Data,
		Obj: r.Obj,
		ChpxSprms: r.ChpxSprms,
		PapxSprms: r.PapxSprms,
	}
}