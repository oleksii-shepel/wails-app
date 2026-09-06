package docformat

import "encoding/binary"

// OperandSize returns the byte length of the operand that follows a Word 97+
// two-byte SPRM code at grpprl[pos]. It implements the generic spra sizing
// rules plus the variable-length exceptions needed by PAP/TAP decoding.
func OperandSize(code uint16, grpprl []byte, pos int) int {
	spra := (code >> 13) & 0x07
	switch spra {
	case 0, 1:
		return 1
	case 2, 4, 5:
		return 2
	case 3:
		return 4
	case 6:
		return variableOperandSize(code, grpprl, pos)
	case 7:
		return 3
	default:
		return -1
	}
}

func variableOperandSize(code uint16, grpprl []byte, pos int) int {
	if pos >= len(grpprl) {
		return -1
	}
	switch code {
	case 0xC615: // sprmPChgTabs
		delCount := int(grpprl[pos])
		addCountPos := pos + 1 + delCount*2
		if addCountPos >= len(grpprl) {
			return -1
		}
		addCount := int(grpprl[addCountPos])
		return 1 + delCount*2 + 1 + addCount*2 + addCount
	case 0xD608: // sprmTDefTable
		if pos+2 > len(grpprl) {
			return -1
		}
		cb := int(binary.LittleEndian.Uint16(grpprl[pos : pos+2]))
		return cb + 1
	default:
		return 1 + int(grpprl[pos])
	}
}

func ToggleBool(b byte) bool { return b != 0 }
