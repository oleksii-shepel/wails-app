package word97

import (
	"bytes"
	"encoding/binary"
	"testing"

	docbinary "office/pkg/word/parsers/doc/binary"
)

func testStoryLayout() ([]byte, docbinary.Word97Layout) {
	layout:=docbinary.Word97Layout{FibBase:32,CbRgFcLcb:60}
	return make([]byte,layout.FibBase+layout.CbRgFcLcb*8),layout
}

func putStoryTableRange(fib []byte, layout docbinary.Word97Layout, table []byte, index, fc int, data []byte) {
	copy(table[fc:],data)
	off:=layout.FibBase+index*8
	binary.LittleEndian.PutUint32(fib[off:off+4],uint32(fc))
	binary.LittleEndian.PutUint32(fib[off+4:off+8],uint32(len(data)))
}

func cpArray(values ...uint32) []byte { out:=make([]byte,len(values)*4);for i,v:=range values{binary.LittleEndian.PutUint32(out[i*4:],v)};return out }

func noteRefPLC(cp uint32) []byte { out:=make([]byte,10);binary.LittleEndian.PutUint32(out[0:],cp);binary.LittleEndian.PutUint32(out[4:],cp+1);return out }
func annotationRefPLC(cp uint32) []byte { out:=make([]byte,38);binary.LittleEndian.PutUint32(out[0:],cp);binary.LittleEndian.PutUint32(out[4:],cp+1);return out }

func TestLoadSideStoryRangesUsesAbsoluteStoryBases(t *testing.T) {
	fib,layout:=testStoryLayout();table:=make([]byte,512)
	putStoryTableRange(fib,layout,table,2,16,noteRefPLC(4));putStoryTableRange(fib,layout,table,3,32,cpArray(0,3))
	putStoryTableRange(fib,layout,table,46,64,noteRefPLC(8));putStoryTableRange(fib,layout,table,47,80,cpArray(0,5))
	putStoryTableRange(fib,layout,table,4,112,annotationRefPLC(12));putStoryTableRange(fib,layout,table,5,160,cpArray(0,7))
	counts:=docbinary.StoryCounts{Text:100,Ftn:10,Hdr:20,Mcr:2,Atn:7}
	got:=LoadSideStoryRanges(bytes.NewReader(table),int64(len(table)),fib,layout,counts)
	if len(got.Footnotes)!=1 || got.Footnotes[0].Range!=(StoryRange{100,103}) { t.Fatalf("footnotes=%+v",got.Footnotes) }
	if len(got.Comments)!=1 || got.Comments[0].Range!=(StoryRange{132,139}) { t.Fatalf("comments=%+v",got.Comments) }
	if len(got.Endnotes)!=1 || got.Endnotes[0].Range!=(StoryRange{139,144}) { t.Fatalf("endnotes=%+v",got.Endnotes) }
}

func TestLoadHeaderFooterRangesGroupsSixSlotsPerSection(t *testing.T) {
	fib,layout:=testStoryLayout();table:=make([]byte,512)
	// Six separator stories + six section slots + two terminal CP values.
	values:=make([]uint32,14);for i:=range values{values[i]=uint32(i*2)}
	putStoryTableRange(fib,layout,table,11,32,cpArray(values...))
	counts:=docbinary.StoryCounts{Text:100,Ftn:10,Hdr:40}
	got:=LoadHeaderFooterRanges(bytes.NewReader(table),int64(len(table)),fib,layout,counts,1)
	if got==nil || got.SectionCount!=1 || len(got.Sections)!=1 { t.Fatalf("plan=%+v",got) }
	if got.NoteStories[0]!=(StoryRange{110,112}) { t.Fatalf("note0=%+v",got.NoteStories[0]) }
	if got.Sections[0][0]!=(StoryRange{122,124}) || got.Sections[0][5]!=(StoryRange{132,134}) { t.Fatalf("slots=%+v",got.Sections[0]) }
}

func TestLoadSectionRecordsReadsSepxFromWordDocument(t *testing.T) {
	fib,layout:=testStoryLayout();table:=make([]byte,128);word:=make([]byte,128)
	plc:=make([]byte,20);binary.LittleEndian.PutUint32(plc[0:],0);binary.LittleEndian.PutUint32(plc[4:],20);binary.LittleEndian.PutUint32(plc[10:],40)
	putStoryTableRange(fib,layout,table,6,16,plc)
	binary.LittleEndian.PutUint16(word[40:],3);copy(word[42:],[]byte{1,2,3})
	got:=LoadSectionRecords(bytes.NewReader(word),int64(len(word)),bytes.NewReader(table),int64(len(table)),fib,layout)
	if len(got)!=1 || got[0].Descriptor.StartCP!=0 || got[0].Descriptor.EndCP!=20 || !bytes.Equal(got[0].Sepx,[]byte{1,2,3}) { t.Fatalf("records=%+v",got) }
}
