package word97

import (
	"bytes"
	"encoding/binary"
	"testing"

	docbinary "office/pkg/word/parsers/doc/binary"
)

func TestLoadMetadataReaderAtLoadsStoryAndReferenceMetadata(t *testing.T) {
	fib := make([]byte, 512)
	layout := docbinary.Word97Layout{StoryCountsBase: 16, StoryCountSlots: 11, FibBase: 80, CbRgFcLcb: 50}
	binary.LittleEndian.PutUint32(fib[16+3*4:], 20)
	table := make([]byte, 128)
	put := func(index, fc int, data []byte) {
		copy(table[fc:], data)
		off := layout.FibBase + index*8
		binary.LittleEndian.PutUint32(fib[off:], uint32(fc))
		binary.LittleEndian.PutUint32(fib[off+4:], uint32(len(data)))
	}
	foot := make([]byte, 8)
	binary.LittleEndian.PutUint32(foot[0:], 5)
	binary.LittleEndian.PutUint32(foot[4:], 20)
	put(2, 8, foot)
	end := make([]byte, 8)
	binary.LittleEndian.PutUint32(end[0:], 7)
	binary.LittleEndian.PutUint32(end[4:], 20)
	put(46, 24, end)
	ann := make([]byte, 38)
	binary.LittleEndian.PutUint32(ann[0:], 9)
	binary.LittleEndian.PutUint32(ann[4:], 20)
	put(4, 40, ann)
	got := LoadMetadataReaderAt(bytes.NewReader(table), int64(len(table)), fib, layout)
	if got.StoryCounts.Text != 20 {
		t.Fatalf("counts=%+v", got.StoryCounts)
	}
	if got.FootnoteRefs[5] != 1 || got.EndnoteRefs[7] != 1 || got.CommentRefs[9] != 1 {
		t.Fatalf("refs foot=%v end=%v comment=%v", got.FootnoteRefs, got.EndnoteRefs, got.CommentRefs)
	}
}

func TestLoadBookmarksReaderAtOwnsCandidatePairing(t *testing.T) {
	fib := make([]byte, 256)
	layout := docbinary.Word97Layout{FibBase: 16, CbRgFcLcb: 22}
	table := make([]byte, 256)
	put := func(index, fc int, data []byte) {
		copy(table[fc:], data)
		off := layout.FibBase + index*8
		binary.LittleEndian.PutUint32(fib[off:], uint32(fc))
		binary.LittleEndian.PutUint32(fib[off+4:], uint32(len(data)))
	}

	nameTable := []byte{1, 0, 1, 0, 0, 0, 3, 0, 'B', 'k', 'A'}
	startPLC := make([]byte, 12)
	binary.LittleEndian.PutUint32(startPLC[0:], 5)
	binary.LittleEndian.PutUint32(startPLC[4:], 99)
	endPLC := make([]byte, 8)
	binary.LittleEndian.PutUint32(endPLC[0:], 12)
	binary.LittleEndian.PutUint32(endPLC[4:], 99)
	put(0, 80, nameTable)
	put(1, 96, startPLC)
	put(20, 128, endPLC)

	starts, ends := LoadBookmarksReaderAt(bytes.NewReader(table), int64(len(table)), fib, layout, 100)
	if got := starts[5]; len(got) != 1 || got[0] != "BkA" {
		t.Fatalf("starts=%v", starts)
	}
	if got := ends[12]; len(got) != 1 || got[0] != "BkA" {
		t.Fatalf("ends=%v", ends)
	}
}
