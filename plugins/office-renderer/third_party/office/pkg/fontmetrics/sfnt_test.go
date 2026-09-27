package fontmetrics

import (
	"encoding/binary"
	"sort"
	"testing"
)

func TestParseSFNTUsesGPOSKernBeforeLegacyKern(t *testing.T) {
	face, ok := ParseSFNT(testSFNTWithPairAdjustments(-100, -150))
	if !ok || face == nil {
		t.Fatal("ParseSFNT failed")
	}
	left, lok := face.RuneToGlyph['A']
	right, rok := face.RuneToGlyph['B']
	if !lok || !rok {
		t.Fatalf("cmap missing A/B: %#v", face.RuneToGlyph)
	}
	// GPOS -150 at 20px / 1000 units-em = -3. Legacy kern (-100) would be -2.
	if got := face.PairAdjustment(left, right, 20); got != -3 {
		t.Fatalf("PairAdjustment(A,B)=%.3f, want -3 from GPOS", got)
	}
	if got, ok := face.AdvanceForGlyph(left, 20); !ok || got != 12 {
		t.Fatalf("AdvanceForGlyph(A)=%.3f,%v want 12,true", got, ok)
	}
}

func testSFNTWithPairAdjustments(kernAB, gposAB int16) []byte {
	tables := map[string][]byte{}
	head := make([]byte, 54)
	binary.BigEndian.PutUint16(head[18:20], 1000)
	tables["head"] = head
	hhea := make([]byte, 36)
	binary.BigEndian.PutUint16(hhea[34:36], 3)
	tables["hhea"] = hhea
	maxp := make([]byte, 6)
	binary.BigEndian.PutUint16(maxp[4:6], 3)
	tables["maxp"] = maxp
	hmtx := make([]byte, 12)
	for i, aw := range []uint16{500, 600, 700} {
		binary.BigEndian.PutUint16(hmtx[i*4:i*4+2], aw)
	}
	tables["hmtx"] = hmtx

	// cmap format 12: A->1, B->2.
	f12 := make([]byte, 40)
	binary.BigEndian.PutUint16(f12[0:2], 12)
	binary.BigEndian.PutUint32(f12[4:8], uint32(len(f12)))
	binary.BigEndian.PutUint32(f12[12:16], 2)
	binary.BigEndian.PutUint32(f12[16:20], uint32('A'))
	binary.BigEndian.PutUint32(f12[20:24], uint32('A'))
	binary.BigEndian.PutUint32(f12[24:28], 1)
	binary.BigEndian.PutUint32(f12[28:32], uint32('B'))
	binary.BigEndian.PutUint32(f12[32:36], uint32('B'))
	binary.BigEndian.PutUint32(f12[36:40], 2)
	cmap := make([]byte, 12+len(f12))
	binary.BigEndian.PutUint16(cmap[2:4], 1)
	binary.BigEndian.PutUint16(cmap[4:6], 3)
	binary.BigEndian.PutUint16(cmap[6:8], 10)
	binary.BigEndian.PutUint32(cmap[8:12], 12)
	copy(cmap[12:], f12)
	tables["cmap"] = cmap

	kern := make([]byte, 24)
	binary.BigEndian.PutUint16(kern[2:4], 1)
	binary.BigEndian.PutUint16(kern[6:8], 20)
	binary.BigEndian.PutUint16(kern[8:10], 0x0001)
	binary.BigEndian.PutUint16(kern[10:12], 1)
	binary.BigEndian.PutUint16(kern[18:20], 1)
	binary.BigEndian.PutUint16(kern[20:22], 2)
	binary.BigEndian.PutUint16(kern[22:24], uint16(kernAB))
	tables["kern"] = kern
	tables["GPOS"] = testGPOSPair(1, 2, gposAB)

	order := make([]string, 0, len(tables))
	for tag := range tables {
		order = append(order, tag)
	}
	sort.Strings(order)
	header := 12 + len(order)*16
	total := header
	for _, tag := range order {
		total += (len(tables[tag]) + 3) &^ 3
	}
	out := make([]byte, total)
	binary.BigEndian.PutUint32(out[0:4], 0x00010000)
	binary.BigEndian.PutUint16(out[4:6], uint16(len(order)))
	off := header
	for i, tag := range order {
		d := tables[tag]
		r := 12 + i*16
		copy(out[r:r+4], []byte(tag))
		binary.BigEndian.PutUint32(out[r+8:r+12], uint32(off))
		binary.BigEndian.PutUint32(out[r+12:r+16], uint32(len(d)))
		copy(out[off:], d)
		off += (len(d) + 3) &^ 3
	}
	return out
}

func testGPOSPair(left, right uint16, adjust int16) []byte {
	// Header(10), FeatureList, LookupList, lookup type2, PairPos format1, coverage, pairset.
	g := make([]byte, 64)
	binary.BigEndian.PutUint32(g[0:4], 0x00010000)
	binary.BigEndian.PutUint16(g[6:8], 10)  // feature list
	binary.BigEndian.PutUint16(g[8:10], 24) // lookup list
	// FeatureList at 10: one 'kern' record -> feature table at +8.
	binary.BigEndian.PutUint16(g[10:12], 1)
	copy(g[12:16], []byte("kern"))
	binary.BigEndian.PutUint16(g[16:18], 8)
	binary.BigEndian.PutUint16(g[18:20], 0xffff) // featureParams
	binary.BigEndian.PutUint16(g[20:22], 1)
	binary.BigEndian.PutUint16(g[22:24], 0) // lookup index 0
	// LookupList at 24, lookup at +4=28.
	binary.BigEndian.PutUint16(g[24:26], 1)
	binary.BigEndian.PutUint16(g[26:28], 4)
	binary.BigEndian.PutUint16(g[28:30], 2) // PairPos
	binary.BigEndian.PutUint16(g[30:32], 0)
	binary.BigEndian.PutUint16(g[32:34], 1)
	binary.BigEndian.PutUint16(g[34:36], 8) // subtable at 36
	// PairPos format1: coverage +16 (52), value1 xAdvance, value2 none, one pairset +10 (46).
	binary.BigEndian.PutUint16(g[36:38], 1)
	binary.BigEndian.PutUint16(g[38:40], 16)
	binary.BigEndian.PutUint16(g[40:42], 0x0004)
	binary.BigEndian.PutUint16(g[42:44], 0)
	binary.BigEndian.PutUint16(g[44:46], 1)
	binary.BigEndian.PutUint16(g[46:48], 10)
	// pairset at 46? offset is relative subtable: 36+10 = 46, which overlaps offset array end intentionally valid.
	binary.BigEndian.PutUint16(g[46:48], 1) // pair count (overwrite offset!)
	// Fix: pairset needs to start after offset array, use +18 instead.
	binary.BigEndian.PutUint16(g[46:48], 18)
	ps := 54
	binary.BigEndian.PutUint16(g[ps:ps+2], 1)
	binary.BigEndian.PutUint16(g[ps+2:ps+4], right)
	binary.BigEndian.PutUint16(g[ps+4:ps+6], uint16(adjust))
	// coverage at 52 would overlap pairset; move to +24 = 60.
	binary.BigEndian.PutUint16(g[38:40], 24)
	cov := 60
	binary.BigEndian.PutUint16(g[cov:cov+2], 1)
	binary.BigEndian.PutUint16(g[cov+2:cov+4], 1)
	g = append(g, make([]byte, 2)...)
	binary.BigEndian.PutUint16(g[cov+4:cov+6], left)
	return g
}
