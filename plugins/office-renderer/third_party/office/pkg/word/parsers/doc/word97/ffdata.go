package word97

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
)

// readNilPICFAndBinData reads the Nil-PICF wrapper that precedes binary
// data (including FFData) in the Word Data stream.  The wrapper consists of:
//
//	4 bytes  lcb      total size of the structure
//	2 bytes  cbHeader header size (expected 0x44 = 68)
//	62 bytes reserved / ignored
//	...      binary payload (lcb - cbHeader bytes)
//
// It returns the payload bytes and true when the header looks valid.
func readNilPICFAndBinData(dataStream []byte, picOffset int) ([]byte, bool) {
	if picOffset < 0 || picOffset+6 > len(dataStream) {
		return nil, false
	}
	lcb := int(binary.LittleEndian.Uint32(dataStream[picOffset : picOffset+4]))
	cbHeader := int(binary.LittleEndian.Uint16(dataStream[picOffset+4 : picOffset+6]))
	if cbHeader != 0x44 {
		return nil, false
	}
	if picOffset+cbHeader > len(dataStream) {
		return nil, false
	}
	binaryLength := lcb - cbHeader
	if binaryLength < 0 || picOffset+cbHeader+binaryLength > len(dataStream) {
		return nil, false
	}
	payload := make([]byte, binaryLength)
	copy(payload, dataStream[picOffset+cbHeader:picOffset+cbHeader+binaryLength])
	return payload, true
}

// readXstz reads a counted Unicode string with a null terminator (Xstz).
// Returns the decoded string and the total bytes consumed.
func readXstz(data []byte, offset int) (string, int) {
	if offset+2 > len(data) {
		return "", 0
	}
	cch := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	size := 2 + cch*2 + 2 // cch + characters + null terminator
	if offset+size > len(data) {
		return "", 0
	}
	runes := make([]rune, cch)
	for i := 0; i < cch; i++ {
		runes[i] = rune(binary.LittleEndian.Uint16(data[offset+2+i*2:]))
	}
	return string(runes), size
}

// readSttb reads an extended-character string table (STTB with fExtend).
// The drop-down list STTB has no extra data (cbExtra == 0).
// Returns the strings and the total bytes consumed.
func readSttb(data []byte, offset int) ([]string, int) {
	if offset+2 > len(data) {
		return nil, 0
	}
	ffff := binary.LittleEndian.Uint16(data[offset : offset+2])
	if ffff != 0xFFFF {
		return nil, 0
	}
	offset += 2
	if offset+4 > len(data) {
		return nil, 0
	}
	cData := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	cbExtra := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	start := offset - 6 // total consumed will be offset - start
	result := make([]string, 0, cData)
	for i := 0; i < cData; i++ {
		if offset+2 > len(data) {
			break
		}
		cchData := int(int16(binary.LittleEndian.Uint16(data[offset : offset+2])))
		offset += 2
		if cchData < 0 {
			continue
		}
		if offset+cchData*2 > len(data) {
			break
		}
		runes := make([]rune, cchData)
		for j := 0; j < cchData; j++ {
			runes[j] = rune(binary.LittleEndian.Uint16(data[offset+j*2:]))
		}
		result = append(result, string(runes))
		offset += cchData * 2
		offset += cbExtra
	}
	return result, offset - start
}

// readFFData parses the binary FFData structure from the Word Data stream
// at the given picOffset (from sprmCPicLocation / sprmCFcObj).  It returns a
// populated FormField when the structure is valid, or nil otherwise.
func readFFData(dataStream []byte, picOffset int) *wordir.FormField {
	payload, ok := readNilPICFAndBinData(dataStream, picOffset)
	if !ok || len(payload) < 10 {
		return nil
	}

	// FFDataBase (10 bytes)
	version := binary.LittleEndian.Uint32(payload[0:4])
	if version != 0xFFFFFFFF {
		return nil
	}
	bits := binary.LittleEndian.Uint16(payload[4:6])
	iType := bits & 0x0003
	cch := int(binary.LittleEndian.Uint16(payload[6:8]))
	// hps := int(binary.LittleEndian.Uint16(payload[8:10])) // checkbox size, unused

	offset := 10

	// xstzName
	name, n := readXstz(payload, offset)
	if n == 0 {
		return nil
	}
	offset += n

	ff := &wordir.FormField{}

	switch iType {
	case 0: // ITYPE_TEXT
		ff.Type = "text"
		ff.MaxLength = cch
		if offset < len(payload) {
			defText, n := readXstz(payload, offset)
			if n > 0 {
				ff.DefaultText = defText
				offset += n
			}
		}
	case 1: // ITYPE_CHCK
		ff.Type = "checkbox"
		if offset+2 <= len(payload) {
			ff.Checked = binary.LittleEndian.Uint16(payload[offset:offset+2]) != 0
			offset += 2
		}
	case 2: // ITYPE_DROP
		ff.Type = "dropdown"
		if offset+2 <= len(payload) {
			ff.DefaultIdx = int(binary.LittleEndian.Uint16(payload[offset : offset+2]))
			offset += 2
		}
	default:
		return nil
	}

	ff.Name = name

	// Skip xstzTextFormat, xstzHelpText, xstzStatText, xstzEntryMcr, xstzExitMcr
	for i := 0; i < 5; i++ {
		if offset >= len(payload) {
			break
		}
		_, n := readXstz(payload, offset)
		if n == 0 {
			break
		}
		offset += n
	}

	// For dropdown, read the STTB list
	if iType == 2 && offset < len(payload) {
		items, _ := readSttb(payload, offset)
		if len(items) > 0 {
			ff.Items = items
		}
	}

	return ff
}
