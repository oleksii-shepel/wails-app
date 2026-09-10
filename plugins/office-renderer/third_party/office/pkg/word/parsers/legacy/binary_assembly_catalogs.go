package legacy

import wordir "office/pkg/word/ir"

// binaryWordStyleCatalogFromResolved constructs an assembly style catalog from
// already-resolved semantic styles. It performs no binary decoding.
func binaryWordStyleCatalogFromResolved(
	defaultParagraphStyleID string,
	paragraphStyles map[string]wordir.Style,
	characterStyles map[string]wordir.CharacterStyle,
	paragraphByIndex map[int]wordir.Style,
	characterByIndex map[int]wordir.CharacterStyle,
	structuredPapx bool,
) *binaryWordStyleCatalog {
	return &binaryWordStyleCatalog{
		DefaultParagraphStyleID: defaultParagraphStyleID,
		ParagraphStyles:         paragraphStyles,
		CharacterStyles:         characterStyles,
		paragraphByIndex:        paragraphByIndex,
		characterByIndex:        characterByIndex,
		StructuredPapx:          structuredPapx,
	}
}
