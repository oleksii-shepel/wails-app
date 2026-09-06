package word97

import (
	"encoding/binary"
	"testing"
)

func TestParseStyleSheetDecodesSTDNameAndInheritance(t *testing.T){
	// STSHI: cstd=2, cbSTDBaseInFile=10.
	stsh:=make([]byte,0,96)
	stsh=append(stsh,4,0,2,0,10,0)
	appendSTD:=func(sti,kind,based int,name string){
		std:=make([]byte,10)
		binary.LittleEndian.PutUint16(std[0:2],uint16(sti))
		binary.LittleEndian.PutUint16(std[2:4],uint16(kind|(based<<4)))
		binary.LittleEndian.PutUint16(std[4:6],0)
		nameBytes:=make([]byte,2+len([]rune(name))*2+2)
		binary.LittleEndian.PutUint16(nameBytes[0:2],uint16(len([]rune(name))))
		for i,r:=range []rune(name){binary.LittleEndian.PutUint16(nameBytes[2+i*2:],uint16(r))}
		std=append(std,nameBytes...)
		var cb [2]byte;binary.LittleEndian.PutUint16(cb[:],uint16(len(std)));stsh=append(stsh,cb[:]...);stsh=append(stsh,std...);if len(stsh)&1!=0{stsh=append(stsh,0)}
	}
	appendSTD(0,1,0x0FFF,"Normal")
	appendSTD(1,1,0,"Heading 1")
	got:=ParseStyleSheet(stsh)
	if len(got)!=2{t.Fatalf("styles=%#v",got)}
	if got[0].Name!="Normal"||got[0].BasedOnIndex!=-1||got[0].Kind!=1{t.Fatalf("normal=%+v",got[0])}
	if got[1].Name!="Heading 1"||got[1].BasedOnIndex!=0||got[1].STI!=1{t.Fatalf("heading=%+v",got[1])}
}

func TestParseStyleSheetUsesCanonicalPAPXAndCHPXDecoders(t *testing.T){
	// One paragraph STD with PAPX alignment and CHPX bold UPX blocks.
	stsh:=[]byte{4,0,1,0,10,0}
	std:=make([]byte,10);binary.LittleEndian.PutUint16(std[0:2],0);binary.LittleEndian.PutUint16(std[2:4],1|(0x0FFF<<4));binary.LittleEndian.PutUint16(std[4:6],2)
	name:=[]byte{6,0,'N',0,'o',0,'r',0,'m',0,'a',0,'l',0,0,0};std=append(std,name...)
	// UpxPapx: istd prefix + sprmPJc(0x2403)=center(1).
	std=append(std,5,0,0,0,0x03,0x24,0x01,0)
	// UpxChpx: sprmCFBold(0x0835)=true.
	std=append(std,3,0,0x35,0x08,0x01,0)
	var cb [2]byte;binary.LittleEndian.PutUint16(cb[:],uint16(len(std)));stsh=append(stsh,cb[:]...);stsh=append(stsh,std...)
	got:=ParseStyleSheet(stsh);if len(got)!=1{t.Fatalf("styles=%#v",got)}
	if got[0].RawParaFormat.Alignment==""{t.Fatalf("paragraph format=%+v",got[0].RawParaFormat)}
	if got[0].RawRunFormat.Bold==nil||!*got[0].RawRunFormat.Bold{t.Fatalf("run format=%+v",got[0].RawRunFormat)}
	if len(got[0].ParaSprms)==0||len(got[0].RunSprms)==0{t.Fatalf("raw sprms para=%x run=%x",got[0].ParaSprms,got[0].RunSprms)}
}
