package legacy

import word97 "office/pkg/word/parsers/doc/word97"

func legacyStyleSheetFromCatalog(catalog *word97.StyleCatalog) *legacyStyleSheet {
	if catalog==nil{return nil}
	ss:=&legacyStyleSheet{
		DefaultParagraphStyleID:catalog.DefaultParagraphStyleID,
		ParagraphStyles:catalog.ParagraphStyles,
		CharacterStyles:catalog.CharacterStyles,
		paragraphByIndex:catalog.ParagraphByIndex,
		characterByIndex:catalog.CharacterByIndex,
		StructuredPapx:true,
	}
	return ss
}
