package word97

import (
	"bytes"
	"encoding/binary"
	"testing"

	docbinary "office/pkg/word/parsers/doc/binary"
)

func drawingSpa(cp,shape uint32)[]byte{
	blob:=make([]byte,34)
	binary.LittleEndian.PutUint32(blob[0:],cp)
	binary.LittleEndian.PutUint32(blob[4:],cp+1)
	off:=8
	binary.LittleEndian.PutUint32(blob[off:],shape)
	binary.LittleEndian.PutUint32(blob[off+4:],100)
	binary.LittleEndian.PutUint32(blob[off+8:],200)
	binary.LittleEndian.PutUint32(blob[off+12:],500)
	binary.LittleEndian.PutUint32(blob[off+16:],700)
	return blob
}

func drawingTextbox(start,end,shape uint32)[]byte{
	blob:=make([]byte,30)
	binary.LittleEndian.PutUint32(blob[0:],start)
	binary.LittleEndian.PutUint32(blob[4:],end)
	binary.LittleEndian.PutUint32(blob[8+14:],shape)
	return blob
}

func TestLoadDrawingMetadataReaderAtRebasesHeaderAndTextboxStories(t *testing.T){
	fib,layout:=testStoryLayout();table:=make([]byte,768)
	putStoryTableRange(fib,layout,table,docbinary.FibPlcSpaMom,16,drawingSpa(5,1001))
	putStoryTableRange(fib,layout,table,docbinary.FibPlcSpaHdr,64,drawingSpa(3,2001))
	putStoryTableRange(fib,layout,table,docbinary.FibPlcftxbxTxt,112,drawingTextbox(1,4,1001))
	putStoryTableRange(fib,layout,table,docbinary.FibPlcfHdrtxbxTxt,160,drawingTextbox(2,6,2001))
	putStoryTableRange(fib,layout,table,docbinary.FibDggInfo,208,[]byte{1,2,3,4,5,6,7,8})
	counts:=docbinary.StoryCounts{Text:100,Ftn:10,Hdr:20,Mcr:2,Atn:3,Edn:4,Txbx:7,HdrTxbx:8}
	got:=LoadDrawingMetadataReaderAt(bytes.NewReader(table),int64(len(table)),fib,layout,counts)
	if len(got.MainAnchors)!=1||got.MainAnchors[0].CP!=5{t.Fatalf("main=%+v",got.MainAnchors)}
	if len(got.HeaderAnchors)!=1||got.HeaderAnchors[0].CP!=113||!got.HeaderAnchors[0].Header{t.Fatalf("header=%+v",got.HeaderAnchors)}
	if len(got.Textboxes)!=2{t.Fatalf("textboxes=%+v",got.Textboxes)}
	base:=100+10+20+2+3+4
	if got.Textboxes[0].StartCP!=base+1||got.Textboxes[0].EndCP!=base+4{t.Fatalf("main textbox=%+v",got.Textboxes[0])}
	if got.Textboxes[1].StartCP!=base+7+2||got.Textboxes[1].EndCP!=base+7+6||!got.Textboxes[1].Header{t.Fatalf("header textbox=%+v",got.Textboxes[1])}
	if !bytes.Equal(got.DggInfo,[]byte{1,2,3,4,5,6,7,8}){t.Fatalf("dgg=%v",got.DggInfo)}
}

func TestLoadDrawingMetadataReaderAtRecoversNoncanonicalMainFSPA(t *testing.T){
	fib,layout:=testStoryLayout();table:=make([]byte,256)
	putStoryTableRange(fib,layout,table,12,32,drawingSpa(7,1234))
	got:=LoadDrawingMetadataReaderAt(bytes.NewReader(table),int64(len(table)),fib,layout,docbinary.StoryCounts{Text:20})
	if len(got.MainAnchors)!=1||got.MainAnchors[0].CP!=7||got.MainAnchors[0].Shape!=1234{t.Fatalf("anchors=%+v",got.MainAnchors)}
}
