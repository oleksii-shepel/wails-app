package legacy

import wordir "office/pkg/word/ir"

type legacyWord97Enrichment struct {
	Footnotes      map[int]wordir.Story
	Endnotes       map[int]wordir.Story
	Comments       map[int]wordir.Story
	BookmarkStarts map[int][]string
	BookmarkEnds   map[int][]string
	TextBoxes      []*wordir.TextBox
}

// applyCanonicalWord97Enrichment projects document-wide canonical Word97
// metadata onto the temporary full-buffer run slice. Structural decoding has
// already happened in doc/word97; this function performs compatibility-only
// run injection/adaptation without re-reading FIB, CLX, note, bookmark, or
// OfficeArt tables.
func applyCanonicalWord97Enrichment(runs []docRun, core *legacyDocStreamCore) ([]docRun, legacyWord97Enrichment) {
	if core == nil {
		return runs, legacyWord97Enrichment{}
	}

	if len(core.FootnoteRefs) > 0 {
		runs = injectDocNoteRefs(runs, core.FootnoteRefs, "footnoteRef")
	}
	if len(core.EndnoteRefs) > 0 {
		runs = injectDocNoteRefs(runs, core.EndnoteRefs, "endnoteRef")
	}
	if len(core.CommentRefs) > 0 {
		runs = injectDocCommentRefs(runs, core.CommentRefs)
	}

	// Inline-object and FSPA anchor characters still need to be transformed into
	// compatibility docRun records, but their payloads are already canonical.
	runs = processLegacyObjectAnchors(runs, core.InlineImages)
	runs = applyResolvedWord97Drawings(runs, core.Drawings)

	return runs, legacyWord97Enrichment{
		Footnotes:      core.Footnotes,
		Endnotes:       core.Endnotes,
		Comments:       core.Comments,
		BookmarkStarts: core.BookmarkStarts,
		BookmarkEnds:   core.BookmarkEnds,
		TextBoxes:      core.TextBoxes,
	}
}
