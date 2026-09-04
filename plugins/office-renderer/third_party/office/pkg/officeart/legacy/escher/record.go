package escher

import (
	"encoding/binary"
	"fmt"
)

const HeaderSize = 8

type Header struct {
	Version  uint8
	Instance uint16
	Type     uint16
	Length   uint32
}

func (h Header) IsContainer() bool {
	return h.Version == ContainerVersion
}

func ParseHeader(data []byte) (Header, error) {
	if len(data) < HeaderSize {
		return Header{}, fmt.Errorf("escher: short record header: %d bytes", len(data))
	}
	verInst := binary.LittleEndian.Uint16(data[0:2])
	return Header{
		Version:  uint8(verInst & 0x000F),
		Instance: verInst >> 4,
		Type:     binary.LittleEndian.Uint16(data[2:4]),
		Length:   binary.LittleEndian.Uint32(data[4:8]),
	}, nil
}

type Record struct {
	Header   Header
	Offset   int
	Data     []byte
	Children []Record
}

// ParseRecords parses a contiguous Escher record stream. Unknown records are
// preserved as raw Data and never treated as errors solely because of type.
func ParseRecords(data []byte) ([]Record, error) {
	var out []Record
	for off := 0; off < len(data); {
		if len(data)-off < HeaderSize {
			return nil, fmt.Errorf("escher: %d trailing bytes at offset %d", len(data)-off, off)
		}
		h, err := ParseHeader(data[off:])
		if err != nil {
			return nil, err
		}
		end64 := uint64(off) + HeaderSize + uint64(h.Length)
		if end64 > uint64(len(data)) {
			return nil, fmt.Errorf("escher: record 0x%04X at %d overruns stream: need %d, have %d", h.Type, off, end64, len(data))
		}
		end := int(end64)
		payload := data[off+HeaderSize : end]
		rec := Record{
			Header: h,
			Offset: off,
			// Preserve the exact payload for every record, including containers.
			// Children are a parsed view of this same byte range, not a replacement
			// for it. Format adapters such as XLS need the raw SpContainer payload
			// to apply application-specific ClientAnchor/OBJ ownership rules.
			Data: append([]byte(nil), payload...),
		}
		if h.IsContainer() {
			children, err := ParseRecords(payload)
			if err != nil {
				return nil, fmt.Errorf("escher: container 0x%04X at %d: %w", h.Type, off, err)
			}
			rec.Children = children
		}
		out = append(out, rec)
		off = end
	}
	return out, nil
}
