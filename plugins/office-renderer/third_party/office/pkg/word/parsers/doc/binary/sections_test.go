package docbinary

import (
	"encoding/binary"
	"testing"
)

func TestParseSectionDescriptors(t *testing.T){
	plc:=make([]byte,36) // 2 sections: (n+1)*4 CPs + n*12 SEDs
	binary.LittleEndian.PutUint32(plc[0:],0)
	binary.LittleEndian.PutUint32(plc[4:],10)
	binary.LittleEndian.PutUint32(plc[8:],25)
	binary.LittleEndian.PutUint32(plc[14:],100)
	binary.LittleEndian.PutUint32(plc[26:],0xFFFFFFFF)
	got:=ParseSectionDescriptors(plc)
	if len(got)!=2{t.Fatalf("sections=%v",got)}
	if got[0].StartCP!=0||got[0].EndCP!=10||!got[0].HasSepx||got[0].SepxOffset!=100{t.Fatalf("first=%+v",got[0])}
	if got[1].StartCP!=10||got[1].EndCP!=25||got[1].HasSepx{t.Fatalf("second=%+v",got[1])}
}
