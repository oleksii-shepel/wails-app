package word97

import (
	"encoding/binary"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDecodeDocumentSettingsCoreDOP(t *testing.T){
	dop:=make([]byte,0x220);binary.LittleEndian.PutUint16(dop[0x0A:0x0C],720);binary.LittleEndian.PutUint16(dop[0x02:0x04],uint16((3<<2)|1));dop[0x09]=0x22;dop[0x06]=0x80;binary.LittleEndian.PutUint32(dop[0x1A:0x1E],42);binary.LittleEndian.PutUint16(dop[0x3E:0x40],uint16((125<<3)|1));dop[0x54]=0x00;dop[0x55]=0x80;dop[0x1FC]=0x05
	s:=DecodeDocumentSettings(dop);if s.DefaultTabStop!=720||s.FootnoteNumberStart!=3||s.FootnoteRestart!="eachSect"{t.Fatalf("settings=%+v",s)};if !s.ProtectionEnabled||!s.MirrorMargins||!s.TrackChangesEnabled{t.Fatalf("flags=%+v",s)};if s.WordCount!=42||s.ZoomPercent!=125||s.ViewMode!="print"{t.Fatalf("metadata=%+v",s)};if s.Compatibility==nil||!s.Compatibility.SpaceLikeWW8||!s.Compatibility.ShapeLayoutLikeWW8||!s.Compatibility.DoNotUseHTMLParagraphAutoSpacing{t.Fatalf("compat=%+v",s.Compatibility)}
	var doc wordir.Document;s.Apply(&doc);if doc.DefaultTabStop!=720||doc.WordCount!=42||doc.Compatibility==nil{t.Fatalf("doc=%+v",doc)}
}

func TestDecodeDocumentSettingsDTTM(t *testing.T){
	v:=uint32(30)|(14<<6)|(15<<11)|(3<<16)|(124<<20);dop:=make([]byte,0x1A);binary.LittleEndian.PutUint32(dop[0x0E:0x12],v);if got:=DecodeDocumentSettings(dop).CreatedAt;got!="2024-03-15T14:30:00"{t.Fatalf("CreatedAt=%q",got)}
}
