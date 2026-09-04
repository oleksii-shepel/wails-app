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
