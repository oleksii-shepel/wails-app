package legacy

import (
	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

// decodeSequentialWord97Section makes the sequential SEPX decoder authoritative
// for layout-critical section state. The compatibility decoder contributes only
// long-tail properties that are not canonical yet (page borders, numbering
// format names, text direction, diagnostics).
func decodeSequentialWord97Section(grpprl []byte) wordir.Section {
	section,_:=docformat.DecodeSEPXCore(grpprl)
	compat,_:=parseSepxGrpprlWithMetadata(grpprl)
	section.TextDirection=compat.TextDirection
	section.PageNumberFormat=compat.PageNumberFormat
	section.FootnoteNumFmt=compat.FootnoteNumFmt
	section.EndnoteNumFmt=compat.EndnoteNumFmt
	section.PageBorders=compat.PageBorders
	section.Diagnostics=compat.Diagnostics
	return section
}
