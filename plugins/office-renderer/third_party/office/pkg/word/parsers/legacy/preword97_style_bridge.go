package legacy

import pre97 "office/pkg/word/parsers/doc/pre97"

// binaryWordStyleCatalogFromPre97Catalog projects the already-resolved Word
// 6/95 style catalog directly into the shared assembler's neutral style shape.
// No STSH parsing or inheritance resolution occurs here.
func binaryWordStyleCatalogFromPre97Catalog(c *pre97.StyleCatalog) *binaryWordStyleCatalog {
	if c == nil {
		return nil
	}
	return binaryWordStyleCatalogFromResolved(
		c.DefaultParagraphStyleID,
		c.ParagraphStyles,
		c.CharacterStyles,
		c.ParagraphByIndex,
		c.CharacterByIndex,
		false,
	)
}
