package word97

import (
	"bytes"
	"encoding/binary"
	"testing"

	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

func TestPieceReaderAtCursorSplitsCanonicalFormattingBoundaries(t *testing.T) {
	word := make([]byte, 64)
	copy(word[20:], []byte("hello"))
	plc := make([]byte, 16)
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], 5)
	binary.LittleEndian.PutUint32(plc[10:14], 0x40000000|uint32(20*2))
	bold := true
	italic := true
	chpx := []docformat.CHPXEntry{
		{FC:20,Format:wordir.RunFormat{Bold:&bold}},
		{FC:22,Format:wordir.RunFormat{Italic:&italic}},
	}
	papx := []docformat.PAPXEntry{{FC:20,Format:wordir.ParagraphFormat{Alignment:"center"},Meta:docformat.PAPXMetadata{TableDepth:1}}}
	cursor, err := NewPieceReaderAtCursor(bytes.NewReader(word), int64(len(word)), plc, chpx, papx, nil)
	if err != nil { t.Fatalf("NewPieceReaderAtCursor: %v",err) }
	first,ok,err:=cursor.Next();if err!=nil||!ok{t.Fatalf("first: ok=%v err=%v",ok,err)}
	second,ok,err:=cursor.Next();if err!=nil||!ok{t.Fatalf("second: ok=%v err=%v",ok,err)}
	if first.Text!="he" || first.Format.Bold==nil || !*first.Format.Bold || first.ParagraphFormat.Alignment!="center" || first.TableDepth!=1 { t.Fatalf("first=%+v",first) }
	if second.Text!="llo" || second.Format.Italic==nil || !*second.Format.Italic || second.StartCP!=2 { t.Fatalf("second=%+v",second) }
	if _,ok,err:=cursor.Next();err!=nil||ok{t.Fatalf("end: ok=%v err=%v",ok,err)}
}

func TestPieceReaderAtCursorDecodesUnicodePiece(t *testing.T) {
	word:=make([]byte,64)
	binary.LittleEndian.PutUint16(word[24:26],'A')
	binary.LittleEndian.PutUint16(word[26:28],'Ω')
	plc:=make([]byte,16);binary.LittleEndian.PutUint32(plc[0:4],0);binary.LittleEndian.PutUint32(plc[4:8],2);binary.LittleEndian.PutUint32(plc[10:14],24)
	cursor,err:=NewPieceReaderAtCursor(bytes.NewReader(word),int64(len(word)),plc,nil,nil,nil);if err!=nil{t.Fatal(err)}
	run,ok,err:=cursor.Next();if err!=nil||!ok||run.Text!="AΩ"{t.Fatalf("run=%+v ok=%v err=%v",run,ok,err)}
}
