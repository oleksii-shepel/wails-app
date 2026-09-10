package legacy

import (
	word97 "office/pkg/word/parsers/doc/word97"
)

// word97BinaryDocumentInput projects canonical Word97 document-wide semantics
// directly into the same neutral assembler input produced by genuine pre-97.
// It performs no binary decoding and owns no intermediate Word97 carrier.
func word97BinaryDocumentInput(snapshot *word97.Snapshot, runs []binaryWordRun) binaryWordDocumentInput {
	if snapshot == nil || snapshot.Core == nil {
		return binaryWordDocumentInput{Runs: runs}
	}
	core := snapshot.Core
	return binaryWordDocumentInput{
		Runs:           runs,
		BookmarkStarts: core.Metadata.BookmarkStarts,
		BookmarkEnds:   word97BookmarkEndCPByName(core.Metadata.BookmarkEnds),
		ListCatalog:    binaryWordListCatalogFromWord97(core.Numbering),
		StyleCatalog:   binaryWordStyleCatalogFromWord97(core.Styles),
	}
}

func word97BookmarkEndCPByName(ends map[int][]string) map[string]int {
	if len(ends) == 0 {
		return nil
	}
	out := make(map[string]int)
	for cp, names := range ends {
		for _, name := range names {
			if name != "" {
				out[name] = cp
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func binaryWordStyleCatalogFromWord97(catalog *word97.StyleCatalog) *binaryWordStyleCatalog {
	if catalog == nil {
		return nil
	}
	return binaryWordStyleCatalogFromResolved(
		catalog.DefaultParagraphStyleID,
		catalog.ParagraphStyles,
		catalog.CharacterStyles,
		catalog.ParagraphByIndex,
		catalog.CharacterByIndex,
		true,
	)
}
