package docbinary

import "encoding/binary"

// ComplexFileTable is the subset of the DOC Clx needed by the sequential
// reader: formatting grpprls plus the PlcPcd piece table payload.
type ComplexFileTable struct {
	Grpprls [][]byte
	PlcPcd  []byte
}

// ParseComplexFileTable decodes the Word binary Clx structure without
// materializing document text or semantic IR.
func ParseComplexFileTable(clxData []byte) *ComplexFileTable {
	if len(clxData) == 0 {
		return nil
	}
	var out ComplexFileTable
	pos := 0
	for pos < len(clxData) {
		switch clxData[pos] {
		case 0x01: // Prc
			if pos+3 > len(clxData) {
				return nil
			}
			cbGrpprl := int(int16(binary.LittleEndian.Uint16(clxData[pos+1 : pos+3])))
			if cbGrpprl < 0 || pos+3+cbGrpprl > len(clxData) {
				return nil
			}
			out.Grpprls = append(out.Grpprls, append([]byte(nil), clxData[pos+3:pos+3+cbGrpprl]...))
			pos += 3 + cbGrpprl
		case 0x02: // Pcdt
			if pos+5 > len(clxData) {
				return nil
			}
			lcb := int(binary.LittleEndian.Uint32(clxData[pos+1 : pos+5]))
			if lcb <= 0 || pos+5+lcb > len(clxData) {
				return nil
			}
			out.PlcPcd = append([]byte(nil), clxData[pos+5:pos+5+lcb]...)
			pos = len(clxData)
		default:
			return nil
		}
	}
	if len(out.PlcPcd) == 0 {
		return nil
	}
	return &out
}
