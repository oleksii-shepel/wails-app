package docbinary

import (
	"encoding/binary"
	"testing"
)

func TestParseStoryCounts(t *testing.T){fib:=make([]byte,64);base:=8;for i,v:=range []uint32{11,2,3,4,5,6,7,8}{binary.LittleEndian.PutUint32(fib[base+(i+3)*4:],v)};got:=ParseStoryCounts(fib,base,11);if got.Text!=11||got.Ftn!=2||got.Hdr!=3||got.Mcr!=4||got.Atn!=5||got.Edn!=6||got.Txbx!=7||got.HdrTxbx!=8{t.Fatalf("counts=%+v",got)}}

func TestParseNoteAndAnnotationReferenceCPs(t *testing.T){note:=make([]byte,16);binary.LittleEndian.PutUint32(note[0:],4);binary.LittleEndian.PutUint32(note[4:],12);binary.LittleEndian.PutUint32(note[8:],20);got:=ParseNoteRefCPs(note);if len(got)!=2||got[0]!=4||got[1]!=12{t.Fatalf("note=%v",got)};ann:=make([]byte,38);binary.LittleEndian.PutUint32(ann[0:],9);binary.LittleEndian.PutUint32(ann[4:],10);got=ParseAnnotationRefCPs(ann);if len(got)!=1||got[0]!=9{t.Fatalf("annotation=%v",got)}}

func TestParseBookmarkPLCs(t *testing.T){start:=make([]byte,20);binary.LittleEndian.PutUint32(start[0:],3);binary.LittleEndian.PutUint32(start[4:],8);binary.LittleEndian.PutUint32(start[8:],12);if got:=ParseBookmarkStartCPs(start,20);len(got)!=2||got[0]!=3||got[1]!=8{t.Fatalf("starts=%v",got)};end:=make([]byte,12);binary.LittleEndian.PutUint32(end[0:],6);binary.LittleEndian.PutUint32(end[4:],11);binary.LittleEndian.PutUint32(end[8:],12);if got:=ParseBookmarkEndCPs(end,2,20);len(got)!=2||got[0]!=6||got[1]!=11{t.Fatalf("ends=%v",got)}}
