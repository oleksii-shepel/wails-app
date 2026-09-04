package legacy

import (
	"encoding/binary"
	"strings"
	"unicode"
)

// loadDocBookmarks returns two maps from the PlcfBkf/SttbfBkmk/PlcfBkl tables:
//   - starts: startCP → []bookmarkName (for paragraph-level anchor attachment)
//   - ends: bookmarkName → endCP (exclusive end position; useful for span computation)
func loadDocBookmarks(wordStream, tableStream []byte, fibBase, cbRgFcLcb, ccpText int) (map[int][]string, map[string]int) {
	names, starts, ends, ok := findLegacyBookmarkData(wordStream, tableStream, fibBase, cbRgFcLcb, ccpText)
	if !ok || len(names) == 0 || len(starts) == 0 {
		return nil, nil
	}
	count := len(names)
	if len(starts) < count {
		count = len(starts)
	}
	if count == 0 {
		return nil, nil
	}
	startMap := make(map[int][]string, count)
	endMap := make(map[string]int, count)
	for i := 0; i < count; i++ {
		name := strings.TrimSpace(names[i])
		if name == "" {
			continue
		}
		startMap[starts[i]] = append(startMap[starts[i]], name)
		if i < len(ends) {
			endMap[name] = ends[i]
		}
	}
	if len(startMap) == 0 {
		return nil, nil
	}
	if len(endMap) == 0 {
		endMap = nil
	}
	return startMap, endMap
}

func findLegacyBookmarkData(wordStream, tableStream []byte, fibBase, cbRgFcLcb, ccpText int) ([]string, []int, []int, bool) {
	type nameCandidate struct {
		index int
		names []string
	}
	type plcCandidate struct {
		index int
		cps   []int
	}
	var names []nameCandidate
	var plcs []plcCandidate
	for index := 0; index < cbRgFcLcb; index++ {
		fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, index)
		if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
			continue
		}
		entry := tableStream[fc : fc+lcb]
		if decoded := parseLegacyBookmarkNameTable(entry); len(decoded) > 0 {
			names = append(names, nameCandidate{index: index, names: decoded})
		}
		if cps := parseLegacyBookmarkStartPLC(entry, ccpText); len(cps) > 0 {
			plcs = append(plcs, plcCandidate{index: index, cps: cps})
		}
	}
	bestScore := int(^uint(0) >> 1)
	var bestNames []string
	var bestStarts []int
	for _, name := range names {
		for _, plc := range plcs {
			if len(name.names) != len(plc.cps) {
				continue
			}
			score := absInt(name.index - plc.index)
			if score >= bestScore {
				continue
			}
			bestScore = score
			bestNames = name.names
			bestStarts = plc.cps
		}
	}
	if bestScore == int(^uint(0)>>1) {
		return nil, nil, nil, false
	}

	// Load end CPs from PlcfBkl at the known FIB index 20 (MS-DOC §2.9.190).
	// PlcfBkl is a pure CP array: (n+1) 4-byte values where n = bookmark count,
	// with the last value being a terminator. No descriptor data follows the CPs.
	var bestEnds []int
	const plcfBklFibIndex = 20
	if fc, lcb, ok := readFibFcLcbEntry(wordStream, fibBase, cbRgFcLcb, plcfBklFibIndex); ok && lcb > 0 && fc >= 0 && fc+lcb <= len(tableStream) {
		bestEnds = parseLegacyBookmarkEndPLC(tableStream[fc:fc+lcb], len(bestStarts), ccpText)
	}

	return bestNames, bestStarts, bestEnds, true
}

func parseLegacyBookmarkStartPLC(plc []byte, maxCP int) []int {
	if len(plc) < 12 || (len(plc)-4)%8 != 0 {
		return nil
	}
	count := (len(plc) - 4) / 8
	cps := make([]int, count)
	for i := 0; i < count; i++ {
		cp := int(binary.LittleEndian.Uint32(plc[i*4 : (i+1)*4]))
		if cp < 0 || cp > maxCP {
			return nil
		}
		if i > 0 && cp < cps[i-1] {
			return nil
		}
		cps[i] = cp
	}
	return cps
}

// parseLegacyBookmarkEndPLC parses a PlcfBkl end-position array.
// PlcfBkl contains (n+1) 4-byte CPs where n is the bookmark count; the last
// value is a terminator and is not returned. n must equal the number of
// bookmarks previously read from PlcfBkf.
func parseLegacyBookmarkEndPLC(plc []byte, n, maxCP int) []int {
	if n <= 0 {
		return nil
	}
	// PlcfBkl must hold at least (n+1) 4-byte entries.
	if len(plc) < (n+1)*4 {
		return nil
	}
	cps := make([]int, n)
	for i := 0; i < n; i++ {
		cp := int(binary.LittleEndian.Uint32(plc[i*4 : (i+1)*4]))
		if cp < 0 || cp > maxCP {
			return nil
		}
		cps[i] = cp
	}
	return cps
}

func parseLegacyBookmarkNameTable(data []byte) []string {
	if len(data) < 6 {
		return nil
	}
	fExtend := binary.LittleEndian.Uint16(data[0:2])
	cData := int(binary.LittleEndian.Uint16(data[2:4]))
	cbExtra := int(binary.LittleEndian.Uint16(data[4:6]))
	if cData <= 0 || cData > 4096 || cbExtra < 0 || cbExtra > 64 {
		return nil
	}
	pos := 6
	names := make([]string, 0, cData)
	for i := 0; i < cData; i++ {
		if pos+2 > len(data) {
			return nil
		}
		cch := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
		pos += 2
		if cch < 0 || cch > 1024 {
			return nil
		}
		byteLen := cch
		if fExtend == 0xFFFF {
			byteLen *= 2
		}
		if pos+byteLen+cbExtra > len(data) {
			return nil
		}
		var name string
		if fExtend == 0xFFFF {
			name = decodeUTF16LE(data[pos : pos+byteLen])
		} else {
			name = decodeWindows1252(data[pos : pos+byteLen])
		}
		if !isLegacyBookmarkName(name) {
			return nil
		}
		names = append(names, name)
		pos += byteLen + cbExtra
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

func isLegacyBookmarkName(name string) bool {
	if name == "" || len(name) > 256 {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			if i == 0 && unicode.IsDigit(r) {
				return false
			}
		default:
			return false
		}
	}
	return true
}