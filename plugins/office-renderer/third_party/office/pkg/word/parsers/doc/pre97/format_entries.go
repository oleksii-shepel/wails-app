package pre97

import (
	"encoding/binary"
	"sort"
)

// CHPXEntry is one FC-indexed character formatting record decoded from a
// pre-Word-97 ChpxFkp page. Sprms contains the raw old-format grpprl bytes;
// semantic interpretation is intentionally a separate projection step.
type CHPXEntry struct {
	FC    uint32
	Sprms []byte
}

// PAPXEntry is one paragraph formatting range decoded from a pre-Word-97
// PapxFkp page. StyleIndex is the old one-byte istd carried by the PAPX header.
type PAPXEntry struct {
	StartFC    int
	EndFC      int
	StartCP    int
	EndCP      int
	StyleIndex *int
	Sprms      []byte
}

// LoadCHPXEntries expands the CHPX BTE references from the canonical body span
// into sorted raw formatting entries. It owns FKP/BTE structure only; callers
// can project old-SPRM semantics independently.
func LoadCHPXEntries(wordStream []byte, span Span) []CHPXEntry {
	pages := FKPPageNumbers(wordStream, span.ChpTableOffset, span.ChpTableSize)
	if len(pages) == 0 {
		return nil
	}
	var entries []CHPXEntry
	for _, pageNum := range pages {
		pageOff := pageNum * 512
		entries = append(entries, parseCHPXPage(wordStream[pageOff:pageOff+512])...)
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].FC < entries[j].FC })
	write := 0
	for _, entry := range entries {
		if write > 0 && entries[write-1].FC == entry.FC {
			continue
		}
		entries[write] = entry
		write++
	}
	return entries[:write]
}

func parseCHPXPage(page []byte) []CHPXEntry {
	if len(page) < 512 {
		return nil
	}
	crun := int(page[511])
	if crun <= 0 || (crun+1)*4+crun > 511 {
		return nil
	}
	rgbxOff := (crun + 1) * 4
	out := make([]CHPXEntry, 0, crun)
	for i := 0; i < crun; i++ {
		entry := CHPXEntry{FC: binary.LittleEndian.Uint32(page[i*4 : i*4+4])}
		bx := int(page[rgbxOff+i])
		if bx != 0 {
			off := bx * 2
			if off+1 < 512 {
				cb := int(page[off])
				if cb > 0 && off+1+cb <= 512 {
					entry.Sprms = append([]byte(nil), page[off+1:off+1+cb]...)
				}
			}
		}
		out = append(out, entry)
	}
	return out
}

// LoadPAPXEntries expands the PAPX BTE references from the canonical body span
// into sorted raw paragraph formatting entries.
func LoadPAPXEntries(wordStream []byte, span Span) []PAPXEntry {
	pages := FKPPageNumbers(wordStream, span.PapTableOffset, span.PapTableSize)
	if len(pages) == 0 {
		return nil
	}
	var entries []PAPXEntry
	for _, pageNum := range pages {
		pageOff := pageNum * 512
		entries = append(entries, parsePAPXPage(wordStream[pageOff:pageOff+512], span)...)
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].StartFC != entries[j].StartFC {
			return entries[i].StartFC < entries[j].StartFC
		}
		return entries[i].EndFC < entries[j].EndFC
	})
	write := 0
	for _, entry := range entries {
		if write > 0 && entries[write-1].StartFC == entry.StartFC && entries[write-1].EndFC == entry.EndFC {
			continue
		}
		entries[write] = entry
		write++
	}
	return entries[:write]
}

func parsePAPXPage(page []byte, span Span) []PAPXEntry {
	if len(page) < 512 {
		return nil
	}
	cpara := int(page[511])
	if cpara <= 0 {
		return nil
	}
	rgbxStart := (cpara + 1) * 4
	if rgbxStart+cpara*7 > 511 {
		return nil
	}
	out := make([]PAPXEntry, 0, cpara)
	for i := 0; i < cpara; i++ {
		startFC := int(binary.LittleEndian.Uint32(page[i*4 : i*4+4]))
		endFC := int(binary.LittleEndian.Uint32(page[(i+1)*4 : (i+1)*4+4]))
		if endFC <= startFC || endFC <= span.Start || startFC >= span.BodyEnd {
			continue
		}
		if startFC < span.Start {
			startFC = span.Start
		}
		if endFC > span.BodyEnd {
			endFC = span.BodyEnd
		}
		entry := PAPXEntry{StartFC: startFC, EndFC: endFC, StartCP: startFC - span.Start, EndCP: endFC - span.Start}
		bOffset := int(page[rgbxStart+i*7])
		if bOffset != 0 {
			off := bOffset * 2
			if off+1 < 512 {
				cb := int(page[off])
				if cb >= 1 {
					istd := int(page[off+1])
					entry.StyleIndex = &istd
					grpLen := 2*cb - 3
					grpOff := off + 3
					if grpLen > 0 && grpOff+grpLen <= 512 {
						entry.Sprms = append([]byte(nil), page[grpOff:grpOff+grpLen]...)
					}
				}
			}
		}
		out = append(out, entry)
	}
	return out
}
