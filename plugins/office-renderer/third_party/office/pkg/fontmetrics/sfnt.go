package fontmetrics

import "encoding/binary"

// Face owns format-neutral SFNT metrics shared by Office renderers.
type Face struct {
	UnitsPerEm  uint16
	RuneToGlyph map[rune]uint16
	GlyphToRune map[uint16]rune
	Advances    []uint16
	kern        map[uint32]int16
	gpos        map[uint32]int16
}

func ParseSFNT(data []byte) (*Face, bool) {
	if len(data) < 12 {
		return nil, false
	}
	nTables := int(binary.BigEndian.Uint16(data[4:6]))
	if nTables <= 0 || 12+nTables*16 > len(data) {
		return nil, false
	}
	table := func(tag string) []byte {
		for i := 0; i < nTables; i++ {
			o := 12 + i*16
			if string(data[o:o+4]) != tag {
				continue
			}
			start := int(binary.BigEndian.Uint32(data[o+8 : o+12]))
			length := int(binary.BigEndian.Uint32(data[o+12 : o+16]))
			if start < 0 || length < 0 || start+length > len(data) {
				return nil
			}
			return data[start : start+length]
		}
		return nil
	}
	head, hhea, maxp, hmtx, cmap := table("head"), table("hhea"), table("maxp"), table("hmtx"), table("cmap")
	if len(head) < 20 || len(hhea) < 36 || len(maxp) < 6 || len(cmap) < 4 {
		return nil, false
	}
	units := binary.BigEndian.Uint16(head[18:20])
	glyphCount := int(binary.BigEndian.Uint16(maxp[4:6]))
	hMetricCount := int(binary.BigEndian.Uint16(hhea[34:36]))
	if units == 0 || glyphCount <= 0 || hMetricCount <= 0 || len(hmtx) < hMetricCount*4 {
		return nil, false
	}
	advances := make([]uint16, glyphCount)
	var last uint16
	for i := 0; i < glyphCount; i++ {
		if i < hMetricCount {
			last = binary.BigEndian.Uint16(hmtx[i*4 : i*4+2])
		}
		advances[i] = last
	}
	r2g, g2r := parseCmap(cmap, glyphCount)
	if len(r2g) == 0 {
		return nil, false
	}
	return &Face{
		UnitsPerEm:  units,
		RuneToGlyph: r2g,
		GlyphToRune: g2r,
		Advances:    advances,
		kern:        parseKern(table("kern")),
		gpos:        parseGPOSKern(table("GPOS")),
	}, true
}

func (f *Face) AdvanceForGlyph(gid uint16, size float64) (float64, bool) {
	if f == nil || f.UnitsPerEm == 0 || int(gid) >= len(f.Advances) {
		return 0, false
	}
	return float64(f.Advances[gid]) / float64(f.UnitsPerEm) * size, true
}

// PairAdjustment returns the horizontal change to the next glyph origin.
// GPOS 'kern' positioning takes precedence over legacy kern for the same pair.
func (f *Face) PairAdjustment(left, right uint16, size float64) float64 {
	if f == nil || f.UnitsPerEm == 0 {
		return 0
	}
	key := uint32(left)<<16 | uint32(right)
	value, ok := f.gpos[key]
	if !ok {
		value = f.kern[key]
	}
	return float64(value) / float64(f.UnitsPerEm) * size
}

func parseKern(data []byte) map[uint32]int16 {
	out := map[uint32]int16{}
	if len(data) < 4 || binary.BigEndian.Uint16(data[0:2]) != 0 {
		return out
	}
	n := int(binary.BigEndian.Uint16(data[2:4]))
	off := 4
	for i := 0; i < n && off+6 <= len(data); i++ {
		length := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		coverage := binary.BigEndian.Uint16(data[off+4 : off+6])
		if length < 14 || off+length > len(data) {
			break
		}
		format := uint8(coverage >> 8)
		horizontal := coverage&0x0001 != 0
		crossStream := coverage&0x0004 != 0
		override := coverage&0x0008 != 0
		if format == 0 && horizontal && !crossStream {
			nPairs := int(binary.BigEndian.Uint16(data[off+6 : off+8]))
			pos := off + 14
			for j := 0; j < nPairs && pos+6 <= off+length; j++ {
				left := binary.BigEndian.Uint16(data[pos : pos+2])
				right := binary.BigEndian.Uint16(data[pos+2 : pos+4])
				value := int16(binary.BigEndian.Uint16(data[pos+4 : pos+6]))
				key := uint32(left)<<16 | uint32(right)
				if override {
					out[key] = value
				} else {
					out[key] += value
				}
				pos += 6
			}
		}
		off += length
	}
	return out
}

func parseCmap(cmap []byte, glyphCount int) (map[rune]uint16, map[uint16]rune) {
	r2g := map[rune]uint16{}
	g2r := map[uint16]rune{}
	if len(cmap) < 4 {
		return r2g, g2r
	}
	n := int(binary.BigEndian.Uint16(cmap[2:4]))
	if 4+n*8 > len(cmap) {
		return r2g, g2r
	}
	type subtable struct{ off, score int }
	var subs []subtable
	for i := 0; i < n; i++ {
		o := 4 + i*8
		platform := binary.BigEndian.Uint16(cmap[o : o+2])
		encoding := binary.BigEndian.Uint16(cmap[o+2 : o+4])
		off := int(binary.BigEndian.Uint32(cmap[o+4 : o+8]))
		if off < 0 || off+2 > len(cmap) {
			continue
		}
		score := 1
		switch {
		case platform == 3 && encoding == 10:
			score = 4
		case platform == 0:
			score = 3
		case platform == 3 && (encoding == 1 || encoding == 0):
			score = 2
		}
		subs = append(subs, subtable{off: off, score: score})
	}
	for score := 4; score >= 1; score-- {
		for _, st := range subs {
			if st.score != score {
				continue
			}
			switch binary.BigEndian.Uint16(cmap[st.off : st.off+2]) {
			case 12:
				parseCmap12(cmap[st.off:], glyphCount, r2g, g2r)
			case 4:
				parseCmap4(cmap[st.off:], glyphCount, r2g, g2r)
			}
		}
		if len(r2g) > 0 {
			break
		}
	}
	return r2g, g2r
}

func parseCmap12(data []byte, glyphCount int, r2g map[rune]uint16, g2r map[uint16]rune) {
	if len(data) < 16 || binary.BigEndian.Uint16(data[:2]) != 12 {
		return
	}
	length := int(binary.BigEndian.Uint32(data[4:8]))
	groups := int(binary.BigEndian.Uint32(data[12:16]))
	if length > len(data) || 16+groups*12 > length {
		return
	}
	for i := 0; i < groups; i++ {
		o := 16 + i*12
		start := binary.BigEndian.Uint32(data[o : o+4])
		end := binary.BigEndian.Uint32(data[o+4 : o+8])
		gid0 := binary.BigEndian.Uint32(data[o+8 : o+12])
		if end < start || start > 0x10ffff {
			continue
		}
		if end > 0x10ffff {
			end = 0x10ffff
		}
		for cp := start; cp <= end; cp++ {
			gid := gid0 + cp - start
			if gid != 0 && gid <= 0xffff && int(gid) < glyphCount {
				r, g := rune(cp), uint16(gid)
				if _, ok := r2g[r]; !ok {
					r2g[r] = g
				}
				if _, ok := g2r[g]; !ok {
					g2r[g] = r
				}
			}
			if cp == end {
				break
			}
		}
	}
}

func parseCmap4(data []byte, glyphCount int, r2g map[rune]uint16, g2r map[uint16]rune) {
	if len(data) < 16 || binary.BigEndian.Uint16(data[:2]) != 4 {
		return
	}
	length := int(binary.BigEndian.Uint16(data[2:4]))
	segCount := int(binary.BigEndian.Uint16(data[6:8])) / 2
	if length > len(data) || segCount <= 0 {
		return
	}
	endOff := 14
	startOff := endOff + segCount*2 + 2
	deltaOff := startOff + segCount*2
	rangeOff := deltaOff + segCount*2
	if rangeOff+segCount*2 > length {
		return
	}
	for i := 0; i < segCount; i++ {
		end := binary.BigEndian.Uint16(data[endOff+i*2 : endOff+i*2+2])
		start := binary.BigEndian.Uint16(data[startOff+i*2 : startOff+i*2+2])
		delta := int16(binary.BigEndian.Uint16(data[deltaOff+i*2 : deltaOff+i*2+2]))
		ro := binary.BigEndian.Uint16(data[rangeOff+i*2 : rangeOff+i*2+2])
		if start > end || start == 0xffff {
			continue
		}
		for cp := uint32(start); cp <= uint32(end); cp++ {
			var gid uint16
			if ro == 0 {
				gid = uint16((int32(cp) + int32(delta)) & 0xffff)
			} else {
				base := rangeOff + i*2
				idx := base + int(ro) + int(cp-uint32(start))*2
				if idx+2 > length {
					break
				}
				gid = binary.BigEndian.Uint16(data[idx : idx+2])
				if gid != 0 {
					gid = uint16((int32(gid) + int32(delta)) & 0xffff)
				}
			}
			if gid != 0 && int(gid) < glyphCount {
				r := rune(cp)
				if _, ok := r2g[r]; !ok {
					r2g[r] = gid
				}
				if _, ok := g2r[gid]; !ok {
					g2r[gid] = r
				}
			}
			if cp == uint32(end) {
				break
			}
		}
	}
}

// parseGPOSKern extracts horizontal PairPos adjustments from lookups referenced
// by the OpenType 'kern' feature. PairPos formats 1/2 and Extension lookups are supported.
func parseGPOSKern(g []byte) map[uint32]int16 {
	out := map[uint32]int16{}
	if len(g) < 10 {
		return out
	}
	featureList := int(binary.BigEndian.Uint16(g[6:8]))
	lookupList := int(binary.BigEndian.Uint16(g[8:10]))
	if featureList <= 0 || lookupList <= 0 || featureList+2 > len(g) || lookupList+2 > len(g) {
		return out
	}
	var lookupIDs []uint16
	featureCount := int(binary.BigEndian.Uint16(g[featureList : featureList+2]))
	for i := 0; i < featureCount; i++ {
		o := featureList + 2 + i*6
		if o+6 > len(g) {
			break
		}
		if string(g[o:o+4]) != "kern" {
			continue
		}
		fo := featureList + int(binary.BigEndian.Uint16(g[o+4:o+6]))
		if fo+4 > len(g) {
			continue
		}
		n := int(binary.BigEndian.Uint16(g[fo+2 : fo+4]))
		for j := 0; j < n && fo+4+j*2+2 <= len(g); j++ {
			lookupIDs = append(lookupIDs, binary.BigEndian.Uint16(g[fo+4+j*2:fo+6+j*2]))
		}
	}
	lookupCount := int(binary.BigEndian.Uint16(g[lookupList : lookupList+2]))
	for _, id := range lookupIDs {
		if int(id) >= lookupCount {
			continue
		}
		po := lookupList + 2 + int(id)*2
		if po+2 > len(g) {
			continue
		}
		parseLookup(g, lookupList+int(binary.BigEndian.Uint16(g[po:po+2])), out)
	}
	return out
}

func parseLookup(g []byte, off int, out map[uint32]int16) {
	if off+6 > len(g) {
		return
	}
	typ := binary.BigEndian.Uint16(g[off : off+2])
	n := int(binary.BigEndian.Uint16(g[off+4 : off+6]))
	for i := 0; i < n; i++ {
		po := off + 6 + i*2
		if po+2 > len(g) {
			break
		}
		so := off + int(binary.BigEndian.Uint16(g[po:po+2]))
		switch typ {
		case 2:
			parsePairPos(g, so, out)
		case 9:
			if so+8 <= len(g) && binary.BigEndian.Uint16(g[so:so+2]) == 1 && binary.BigEndian.Uint16(g[so+2:so+4]) == 2 {
				parsePairPos(g, so+int(binary.BigEndian.Uint32(g[so+4:so+8])), out)
			}
		}
	}
}

type valueRecord struct{ xPlacement, xAdvance int16 }

func readValueRecord(g []byte, off int, format uint16) (valueRecord, int) {
	var v valueRecord
	p := off
	read := func() int16 {
		if p+2 > len(g) {
			return 0
		}
		x := int16(binary.BigEndian.Uint16(g[p : p+2]))
		p += 2
		return x
	}
	if format&0x0001 != 0 {
		v.xPlacement = read()
	}
	if format&0x0002 != 0 {
		_ = read()
	}
	if format&0x0004 != 0 {
		v.xAdvance = read()
	}
	if format&0x0008 != 0 {
		_ = read()
	}
	for bit := uint16(0x0010); bit <= 0x0080; bit <<= 1 {
		if format&bit != 0 {
			_ = read()
		}
	}
	return v, p - off
}

func parsePairPos(g []byte, off int, out map[uint32]int16) {
	if off+10 > len(g) {
		return
	}
	format := binary.BigEndian.Uint16(g[off : off+2])
	coverage := parseCoverage(g, off+int(binary.BigEndian.Uint16(g[off+2:off+4])))
	v1f := binary.BigEndian.Uint16(g[off+4 : off+6])
	v2f := binary.BigEndian.Uint16(g[off+6 : off+8])
	if len(coverage) == 0 {
		return
	}
	switch format {
	case 1:
		count := int(binary.BigEndian.Uint16(g[off+8 : off+10]))
		for i := 0; i < count && i < len(coverage); i++ {
			po := off + 10 + i*2
			if po+2 > len(g) {
				break
			}
			ps := off + int(binary.BigEndian.Uint16(g[po:po+2]))
			if ps+2 > len(g) {
				continue
			}
			n := int(binary.BigEndian.Uint16(g[ps : ps+2]))
			p := ps + 2
			for j := 0; j < n; j++ {
				if p+2 > len(g) {
					break
				}
				second := binary.BigEndian.Uint16(g[p : p+2])
				p += 2
				a, n1 := readValueRecord(g, p, v1f)
				p += n1
				b, n2 := readValueRecord(g, p, v2f)
				p += n2
				out[uint32(coverage[i])<<16|uint32(second)] = a.xAdvance + b.xPlacement
			}
		}
	case 2:
		if off+16 > len(g) {
			return
		}
		class1 := parseClassDef(g, off+int(binary.BigEndian.Uint16(g[off+8:off+10])))
		class2 := parseClassDef(g, off+int(binary.BigEndian.Uint16(g[off+10:off+12])))
		c1n := int(binary.BigEndian.Uint16(g[off+12 : off+14]))
		c2n := int(binary.BigEndian.Uint16(g[off+14 : off+16]))
		p := off + 16
		values := make([][]int16, c1n)
		for i := 0; i < c1n; i++ {
			values[i] = make([]int16, c2n)
			for j := 0; j < c2n; j++ {
				a, n1 := readValueRecord(g, p, v1f)
				p += n1
				b, n2 := readValueRecord(g, p, v2f)
				p += n2
				values[i][j] = a.xAdvance + b.xPlacement
			}
		}
		for _, left := range coverage {
			li := int(class1[left])
			if li >= c1n {
				continue
			}
			for right, rclass := range class2 {
				ri := int(rclass)
				if ri < c2n && values[li][ri] != 0 {
					out[uint32(left)<<16|uint32(right)] = values[li][ri]
				}
			}
		}
	}
}

func parseCoverage(g []byte, off int) []uint16 {
	if off+4 > len(g) {
		return nil
	}
	switch binary.BigEndian.Uint16(g[off : off+2]) {
	case 1:
		n := int(binary.BigEndian.Uint16(g[off+2 : off+4]))
		out := make([]uint16, 0, n)
		for i := 0; i < n && off+4+i*2+2 <= len(g); i++ {
			out = append(out, binary.BigEndian.Uint16(g[off+4+i*2:off+6+i*2]))
		}
		return out
	case 2:
		n := int(binary.BigEndian.Uint16(g[off+2 : off+4]))
		var out []uint16
		for i := 0; i < n; i++ {
			p := off + 4 + i*6
			if p+6 > len(g) {
				break
			}
			s := binary.BigEndian.Uint16(g[p : p+2])
			e := binary.BigEndian.Uint16(g[p+2 : p+4])
			for x := s; x <= e; x++ {
				out = append(out, x)
				if x == e {
					break
				}
			}
		}
		return out
	}
	return nil
}

func parseClassDef(g []byte, off int) map[uint16]uint16 {
	out := map[uint16]uint16{}
	if off+4 > len(g) {
		return out
	}
	switch binary.BigEndian.Uint16(g[off : off+2]) {
	case 1:
		if off+6 > len(g) {
			return out
		}
		start := binary.BigEndian.Uint16(g[off+2 : off+4])
		n := int(binary.BigEndian.Uint16(g[off+4 : off+6]))
		for i := 0; i < n && off+6+i*2+2 <= len(g); i++ {
			out[start+uint16(i)] = binary.BigEndian.Uint16(g[off+6+i*2 : off+8+i*2])
		}
	case 2:
		n := int(binary.BigEndian.Uint16(g[off+2 : off+4]))
		for i := 0; i < n; i++ {
			p := off + 4 + i*6
			if p+6 > len(g) {
				break
			}
			s := binary.BigEndian.Uint16(g[p : p+2])
			e := binary.BigEndian.Uint16(g[p+2 : p+4])
			c := binary.BigEndian.Uint16(g[p+4 : p+6])
			for x := s; x <= e; x++ {
				out[x] = c
				if x == e {
					break
				}
			}
		}
	}
	return out
}
