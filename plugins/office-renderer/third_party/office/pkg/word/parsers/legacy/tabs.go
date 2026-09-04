package legacy

import (
	"encoding/binary"
	"sort"

	wordir "office/pkg/word/ir"
)

func cloneLegacyTabStops(stops []wordir.TabStop) []wordir.TabStop {
	if len(stops) == 0 {
		return nil
	}
	out := make([]wordir.TabStop, len(stops))
	copy(out, stops)
	return out
}

func applyLegacyTabChange(base []wordir.TabStop, payload []byte, deleteEntryWidth int) []wordir.TabStop {
	if len(payload) < 2 || deleteEntryWidth < 2 {
		return cloneLegacyTabStops(base)
	}
	delCount := int(payload[0])
	offset := 1
	deleteBytes := delCount * deleteEntryWidth
	if offset+deleteBytes >= len(payload) {
		return cloneLegacyTabStops(base)
	}

	tabMap := make(map[int]wordir.TabStop, len(base))
	for _, tab := range base {
		tabMap[tab.Pos] = tab
	}

	for i := 0; i < delCount; i++ {
		entry := payload[offset+i*deleteEntryWidth:]
		if len(entry) < 2 {
			return cloneLegacyTabStops(base)
		}
		pos := int(int16(binary.LittleEndian.Uint16(entry[:2])))
		delete(tabMap, pos)
	}
	offset += deleteBytes

	addCount := int(payload[offset])
	offset++
	if offset+addCount*2+addCount > len(payload) {
		return cloneLegacyTabStops(base)
	}
	descOffset := offset + addCount*2
	for i := 0; i < addCount; i++ {
		pos := int(int16(binary.LittleEndian.Uint16(payload[offset+i*2 : offset+i*2+2])))
		if pos <= 0 {
			continue
		}
		desc := payload[descOffset+i]
		tabMap[pos] = wordir.TabStop{
			Pos: pos, Algn: legacyTabAlignment(desc), Leader: legacyTabLeader(desc),
		}
	}
	if len(tabMap) == 0 {
		return nil
	}

	positions := make([]int, 0, len(tabMap))
	for pos := range tabMap {
		positions = append(positions, pos)
	}
	sort.Ints(positions)
	out := make([]wordir.TabStop, 0, len(positions))
	for _, pos := range positions {
		out = append(out, tabMap[pos])
	}
	return out
}

func legacyTabAlignment(desc byte) string {
	switch desc & 0x07 {
	case 1:
		return "center"
	case 2:
		return "right"
	case 3:
		return "decimal"
	case 4:
		return "bar"
	default:
		return "left"
	}
}
func legacyTabLeader(desc byte) string {
	switch (desc >> 3) & 0x07 {
	case 1:
		return "dot"
	case 2:
		return "hyphen"
	case 3:
		return "underscore"
	case 5:
		return "middleDot"
	default:
		return ""
	}
}
