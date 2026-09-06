package legacy

import docbinary "office/pkg/word/parsers/doc/binary"

// Writer and historical legacy tests keep the old helper vocabulary while
// production textbox ownership is decoded directly by doc/binary.
func parseLegacyTextboxPLC(blob []byte, header bool) []docbinary.TextboxRange {
	return docbinary.ParseTextboxPLC(blob, header)
}
