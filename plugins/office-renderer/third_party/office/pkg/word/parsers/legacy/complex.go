package legacy

import docbinary "office/pkg/word/parsers/doc/binary"

type legacyComplexFileTable = docbinary.ComplexFileTable

func loadLegacyComplexFileTable(clxData []byte) *legacyComplexFileTable {
	return docbinary.ParseComplexFileTable(clxData)
}
