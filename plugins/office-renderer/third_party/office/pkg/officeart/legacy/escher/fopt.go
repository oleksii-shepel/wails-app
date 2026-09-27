package escher

import (
	"encoding/binary"
	"fmt"
)

const (
	propertyComplexFlag uint16 = 0x8000
	propertyBlipFlag    uint16 = 0x4000
	propertyIDMask      uint16 = 0x3FFF
)

type Property struct {
	ID        uint16
	Value     uint32
	IsBlip    bool
	IsComplex bool
	Complex   []byte
}

// ParseFOPT parses OfficeArtFOPT/FOPTE data. propCount comes from the record
// header instance field.
func ParseFOPT(data []byte, propCount int) ([]Property, error) {
	if propCount < 0 {
		return nil, fmt.Errorf("escher: negative FOPT property count")
	}
	tableBytes := propCount * 6
	if tableBytes > len(data) {
		return nil, fmt.Errorf("escher: FOPT table needs %d bytes, have %d", tableBytes, len(data))
	}

	props := make([]Property, propCount)
	complexOffset := tableBytes
	for i := 0; i < propCount; i++ {
		base := i * 6
		opid := binary.LittleEndian.Uint16(data[base : base+2])
		value := binary.LittleEndian.Uint32(data[base+2 : base+6])
		props[i] = Property{
			ID:        opid & propertyIDMask,
			Value:     value,
			IsBlip:    opid&propertyBlipFlag != 0,
			IsComplex: opid&propertyComplexFlag != 0,
		}
	}

	for i := range props {
		if !props[i].IsComplex {
			continue
		}
		size := int(props[i].Value)
		if isOfficeArtArrayProperty(props[i].ID) && complexOffset+6 <= len(data) {
			// OfficeArt array properties are a historical exception to the normal
			// FOPT complex-size rule: some producers store only the element bytes
			// in op while the serialized complex payload still begins with the
			// six-byte array header (nElems, nElemsAlloc, cbElem). Detect that
			// form from the header and consume the complete payload so the next
			// complex property stays aligned.
			count := int(binary.LittleEndian.Uint16(data[complexOffset : complexOffset+2]))
			elementSize := OfficeArtArrayElementSize(binary.LittleEndian.Uint16(data[complexOffset+4 : complexOffset+6]))
			if count > 0 && elementSize > 0 && count*elementSize == size && complexOffset+size+6 <= len(data) {
				size += 6
			}
		}
		if size < 0 || complexOffset+size > len(data) {
			return nil, fmt.Errorf("escher: complex FOPT property 0x%04X needs %d bytes at %d, have %d", props[i].ID, size, complexOffset, len(data))
		}
		props[i].Complex = append([]byte(nil), data[complexOffset:complexOffset+size]...)
		complexOffset += size
	}
	return props, nil
}

func PropertyMap(props []Property) map[uint16]Property {
	out := make(map[uint16]Property, len(props))
	for _, prop := range props {
		out[prop.ID] = prop
	}
	return out
}

func isOfficeArtArrayProperty(id uint16) bool {
	switch id {
	case PropVertices, PropSegmentInfo, PropFillShadeColors, 0x01CF, 0x0383, 0x03A0:
		return true
	default:
		return false
	}
}

// officeArtArrayElementSize resolves the peculiar signed cbElem encoding used
// by Escher arrays. Negative values store the real size as (-cbElem)>>2; e.g.
// 0xFFF0 represents a four-byte element.
func OfficeArtArrayElementSize(raw uint16) int {
	signed := int16(raw)
	if signed < 0 {
		return int((-signed) >> 2)
	}
	return int(signed)
}
