package legacy

import pre97 "office/pkg/word/parsers/doc/pre97"

// legacyStyleSheetFromPre97Catalog is a compatibility projection only. The
// Word 6/95 STSH is parsed and inheritance-resolved by doc/pre97.
func legacyStyleSheetFromPre97Catalog(c *pre97.StyleCatalog) *legacyStyleSheet {
	if c == nil { return nil }
	return &legacyStyleSheet{
		DefaultParagraphStyleID: c.DefaultParagraphStyleID,
		ParagraphStyles: c.ParagraphStyles,
		CharacterStyles: c.CharacterStyles,
		paragraphByIndex: c.ParagraphByIndex,
		characterByIndex: c.CharacterByIndex,
		StructuredPapx: false,
	}
}
