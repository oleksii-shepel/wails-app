package mathtype

import (
	"fmt"
	"office/pkg/ooxml/cfb"
)

// ExtractMTEF opens a MathType/Equation Editor OLE compound object and returns
// the native MTEF stream. Equation Native begins with EQNOLEFILEHDR; its cbHdr
// field gives the byte offset of MTEF data (28 for the common header).
func ExtractMTEF(ole []byte) ([]byte, error) {
	f, err := cfb.OpenBytes(ole)
	if err != nil {
		return nil, err
	}
	raw, ok := f.Stream("Equation Native")
	if !ok {
		// Older MathType/Equation Editor objects embedded by PowerPoint commonly
		// store their native payload in the OLE1 Ole10Native stream instead of
		// exposing an Equation Native stream.  The stream starts with a little-
		// endian DWORD containing the payload size; the payload itself starts
		// directly with the MTEF version byte.
		if native, nativeOK := f.Stream("\x01Ole10Native"); nativeOK && len(native) >= 5 {
			n := int(uint32(native[0]) | uint32(native[1])<<8 | uint32(native[2])<<16 | uint32(native[3])<<24)
			if n > 0 && n <= len(native)-4 && (native[4] == 3 || native[4] == 4 || native[4] == 5) {
				return append([]byte(nil), native[4:4+n]...), nil
			}
		}
		return nil, fmt.Errorf("mathtype: Equation Native or Ole10Native stream not found")
	}
	if len(raw) < 6 {
		return nil, ErrInvalidMTEF
	}
	offs := []int{28, 0}
	if len(raw) >= 4 {
		n := int(uint16(raw[2]) | uint16(raw[3])<<8)
		if n > 0 && n < len(raw) {
			offs = append([]int{n}, offs...)
		}
	}
	for _, o := range offs {
		if o+5 <= len(raw) && (raw[o] == 3 || raw[o] == 4 || raw[o] == 5) {
			return append([]byte(nil), raw[o:]...), nil
		}
	}
	return nil, ErrInvalidMTEF
}
func MathMLFromOLE(ole []byte) (string, error) {
	data, err := ExtractMTEF(ole)
	if err != nil {
		return "", err
	}
	return MathML(data)
}
