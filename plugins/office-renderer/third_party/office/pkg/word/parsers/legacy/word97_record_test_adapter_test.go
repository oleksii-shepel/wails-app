package legacy

import (
	"encoding/binary"

	word97 "office/pkg/word/parsers/doc/word97"
)

// loadLegacySttbfRMark exists only for historical synthetic package tests.
// Production Word97 code consumes OpenCore.RevisionAuthors; the actual STTB
// record grammar is delegated to the canonical decoder here as well.
func loadLegacySttbfRMark(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []string {
	const index = 51
	if index >= cbRgFcLcb || fibBase+(index+1)*8 > len(wordStream) {
		return nil
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+index*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+index*8+4:]))
	if lcb < 6 || fc < 0 || fc+lcb > len(tableStream) {
		return nil
	}
	return word97.ParseRevisionAuthors(tableStream[fc : fc+lcb])
}
