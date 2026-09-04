package legacy

import (
	"encoding/binary"

	"office/pkg/officeart/legacy/escher"
)

type BSEntry struct {
	Index    uint32
	WinType  byte
	MacType  byte
	UID      [16]byte
	Tag      uint16
	Size     uint32
	RefCount uint32
	Delay    uint32
	Usage    byte
	Name     []byte
	Media    Media
	Delayed  bool
}

type BStore map[uint32]BSEntry

// DelayedBLIPResolver resolves a BSE foDelay/size pair. PPT commonly stores
// BLIPs in the separate Pictures stream; XLS/DOC commonly embed them inline.
type DelayedBLIPResolver func(offset, size uint32) []byte

func ParseBSE(payload []byte, resolve DelayedBLIPResolver) (BSEntry, bool) {
	if len(payload) < 36 {
		return BSEntry{}, false
	}
	var out BSEntry
	out.WinType = payload[0]
	out.MacType = payload[1]
	copy(out.UID[:], payload[2:18])
	out.Tag = binary.LittleEndian.Uint16(payload[18:20])
	out.Size = binary.LittleEndian.Uint32(payload[20:24])
	out.RefCount = binary.LittleEndian.Uint32(payload[24:28])
	out.Delay = binary.LittleEndian.Uint32(payload[28:32])
	out.Usage = payload[32]

	nameLen := int(payload[33])
	dataOff := 36
	if nameLen > 0 && dataOff+nameLen <= len(payload) {
		out.Name = append([]byte(nil), payload[dataOff:dataOff+nameLen]...)
		dataOff += nameLen
	}

	// Prefer the embedded BLIP when present. Scan rather than assuming exact
	// producer padding between BSE and the BLIP record.
	if dataOff < len(payload) {
		records := escher.Scan(payload[dataOff:], func(h escher.Header) bool {
			return IsBlipRecordType(h.Type)
		})
		for _, rec := range records {
			if rec.Header.IsContainer() {
				continue
			}
			media := ExtractBLIP(rec.Header.Type, rec.Data)
			if len(media.Data) > 0 {
				out.Media = media
				return out, true
			}
		}
	}

	if resolve != nil && out.Size > 0 {
		raw := resolve(out.Delay, out.Size)
		if len(raw) > 0 {
			media := ExtractBLIPStream(raw)
			if len(media.Data) > 0 {
				out.Media = media
				out.Delayed = true
				return out, true
			}
		}
	}
	return out, true
}

// ExtractBLIPStream decodes either a complete OfficeArt BLIP record or a byte
// window containing one. It is useful for PPT's delayed Pictures stream.
func ExtractBLIPStream(data []byte) Media {
	if len(data) >= escher.HeaderSize {
		if h, err := escher.ParseHeader(data); err == nil {
			total := escher.HeaderSize + int(h.Length)
			if total <= len(data) && IsBlipRecordType(h.Type) {
				return ExtractBLIP(h.Type, data[escher.HeaderSize:total])
			}
		}
	}
	records := escher.Scan(data, func(h escher.Header) bool {
		return IsBlipRecordType(h.Type)
	})
	for _, rec := range records {
		if rec.Header.IsContainer() {
			continue
		}
		if media := ExtractBLIP(rec.Header.Type, rec.Data); len(media.Data) > 0 {
			return media
		}
	}
	if blob, mime := ExtractRaster(data); len(blob) > 0 {
		return Media{MIME: mime, Data: blob}
	}
	return Media{}
}

// ParseBStore walks a DrawingGroup/Dgg stream and indexes BSE entries using
// OfficeArt's 1-based pib numbering.
func ParseBStore(data []byte, resolve DelayedBLIPResolver) BStore {
	if len(data) == 0 {
		return nil
	}
	store := BStore{}
	index := uint32(1)
	records := escher.Scan(data, func(h escher.Header) bool {
		return h.Type == escher.RecordBSE
	})
	for _, rec := range records {
		entry, ok := ParseBSE(rec.Data, resolve)
		if !ok {
			continue
		}
		entry.Index = index
		store[index] = entry
		index++
	}
	if len(store) == 0 {
		return nil
	}
	return store
}

func (s BStore) Media(index uint32) (Media, bool) {
	entry, ok := s[index]
	if !ok || len(entry.Media.Data) == 0 {
		return Media{}, false
	}
	return entry.Media, true
}
