package doc

import (
	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

func applySequentialDocumentSourceCore(f *cfb.File, opened *word97.OpenCore, doc *wordir.Document) {
	if f == nil || opened == nil || doc == nil {
		return
	}
	doc.Source = opened.Source
	opened.Settings.Apply(doc)
	if len(opened.FontFamilies) > 0 {
		doc.FontFamilies = opened.FontFamilies
	}
	if opened.Styles != nil {
		opened.Styles.ApplyToDocument(doc)
	}
	if opened.Numbering != nil {
		opened.Numbering.ApplyToDocument(doc)
	}
	if preservation := word97.LoadPreservation(f); preservation != nil {
		preservation.ApplyToDocument(doc)
	}
}
