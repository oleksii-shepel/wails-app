package word97

import (
	"encoding/binary"
	"io"
	"strings"

	docbinary "office/pkg/word/parsers/doc/binary"
)

// Metadata contains document-wide CP metadata that can be discovered without
// materializing the WordDocument or table stream.
type Metadata struct {
	StoryCounts    docbinary.StoryCounts
	BookmarkStarts map[int][]string
	BookmarkEnds   map[int][]string
	FootnoteRefs   map[int]int
	EndnoteRefs    map[int]int
	CommentRefs    map[int]int
}

// LoadMetadataReaderAt discovers Word97 story counts, bookmarks and note/comment
// reference positions using only FIB-addressed table ranges.
func LoadMetadataReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) Metadata {
	m := Metadata{StoryCounts: docbinary.ParseStoryCounts(fib, layout.StoryCountsBase, layout.StoryCountSlots)}
	m.BookmarkStarts, m.BookmarkEnds = LoadBookmarksReaderAt(table, tableSize, fib, layout, m.StoryCounts.Text)
	m.FootnoteRefs = LoadReferenceMapReaderAt(table, tableSize, fib, layout, 2, false)
	m.EndnoteRefs = LoadReferenceMapReaderAt(table, tableSize, fib, layout, 46, false)
	m.CommentRefs = LoadReferenceMapReaderAt(table, tableSize, fib, layout, 4, true)
	return m
}

// LoadReferenceMapReaderAt loads one Word97 note/comment reference PLC and
// projects physical reference CPs to stable one-based semantic IDs.
func LoadReferenceMapReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, index int, annotation bool) map[int]int {
	blob, ok := readTableRange(table, tableSize, fib, layout, index, 4)
	if !ok {
		return nil
	}
	var cps []int
	if annotation {
		cps = docbinary.ParseAnnotationRefCPs(blob)
		// Some Word 97-era producers and our compatibility fixtures encode an
		// annotation reference PLC using the compact note-reference record shape.
		// Accept that representation canonically instead of forcing legacy callers
		// to rediscover the same CPs with a second parser.
		if len(cps) == 0 {
			cps = docbinary.ParseNoteRefCPs(blob)
		}
	} else {
		cps = docbinary.ParseNoteRefCPs(blob)
	}
	if len(cps) == 0 {
		return nil
	}
	out := make(map[int]int, len(cps))
	for i, cp := range cps {
		out[cp] = i + 1
	}
	return out
}

// LoadBookmarksReaderAt discovers bookmark names/start/end CPs from the Word97
// FIB-addressed tables. The candidate pairing heuristic lives here so legacy
// compatibility consumers do not need to reimplement structural discovery.
func LoadBookmarksReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, maxCP int) (map[int][]string, map[int][]string) {
	type namesCandidate struct {
		index int
		names []string
	}
	type startsCandidate struct {
		index int
		cps   []int
	}
	var names []namesCandidate
	var starts []startsCandidate
	for index := 0; index < layout.CbRgFcLcb; index++ {
		blob, ok := readTableRange(table, tableSize, fib, layout, index, 1)
		if !ok {
			continue
		}
		if n := docbinary.ParseBookmarkNames(blob); len(n) > 0 {
			names = append(names, namesCandidate{index: index, names: n})
		}
		if cps := docbinary.ParseBookmarkStartCPs(blob, maxCP); len(cps) > 0 {
			starts = append(starts, startsCandidate{index: index, cps: cps})
		}
	}
	best := int(^uint(0) >> 1)
	var bestNames []string
	var bestStarts []int
	for _, n := range names {
		for _, s := range starts {
			if len(n.names) != len(s.cps) {
				continue
			}
			score := n.index - s.index
			if score < 0 {
				score = -score
			}
			if score < best {
				best = score
				bestNames = n.names
				bestStarts = s.cps
			}
		}
	}
	if len(bestNames) == 0 {
		return nil, nil
	}

	var ends []int
	if blob, ok := readTableRange(table, tableSize, fib, layout, 20, 4); ok {
		ends = docbinary.ParseBookmarkEndCPs(blob, len(bestStarts), maxCP)
	}
	startMap := make(map[int][]string, len(bestNames))
	endMap := make(map[int][]string, len(bestNames))
	for i, name := range bestNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		startMap[bestStarts[i]] = append(startMap[bestStarts[i]], name)
		if i < len(ends) {
			endMap[ends[i]] = append(endMap[ends[i]], name)
		}
	}
	if len(startMap) == 0 {
		startMap = nil
	}
	if len(endMap) == 0 {
		endMap = nil
	}
	return startMap, endMap
}

func readTableRange(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, index, minSize int) ([]byte, bool) {
	if table == nil || index < 0 || index >= layout.CbRgFcLcb {
		return nil, false
	}
	off := layout.FibBase + index*8
	if off < 0 || off+8 > len(fib) {
		return nil, false
	}
	fc := int64(binary.LittleEndian.Uint32(fib[off : off+4]))
	lcb := int64(binary.LittleEndian.Uint32(fib[off+4 : off+8]))
	if lcb < int64(minSize) || fc < 0 || fc+lcb > tableSize {
		return nil, false
	}
	buf := make([]byte, int(lcb))
	n, err := table.ReadAt(buf, fc)
	if err != nil && err != io.EOF {
		return nil, false
	}
	if n != len(buf) {
		return nil, false
	}
	return buf, true
}
