package word97

import (
	"encoding/binary"

	docformat "office/pkg/word/parsers/doc/format"
)

// picLocationFromCHPX returns the fcObj payload of sprmCPicLocation (0x6A03).
// Keeping this projection in the canonical Word97 package lets the sequential
// run stream preserve picture identity without reviving the legacy CHPX model.
func picLocationFromCHPX(grpprl []byte) (int, bool) {
	for pos := 0; pos+2 <= len(grpprl); {
		code := binary.LittleEndian.Uint16(grpprl[pos : pos+2])
		pos += 2
		size := docformat.OperandSize(code, grpprl, pos)
		if size < 0 || pos+size > len(grpprl) {
			return 0, false
		}
		op := grpprl[pos : pos+size]
		pos += size
		if code == 0x6A03 && len(op) >= 4 {
			return int(binary.LittleEndian.Uint32(op[:4])), true
		}
	}
	return 0, false
}
