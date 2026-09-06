package docformat

import (
	"encoding/binary"
	"testing"
)

func sepxSprm(code uint16,op ...byte)[]byte{out:=make([]byte,2+len(op));binary.LittleEndian.PutUint16(out[:2],code);copy(out[2:],op);return out}
func u16(v uint16)[]byte{return []byte{byte(v),byte(v>>8)}}

func TestDecodeSEPXCorePageMarginsColumnsAndBreak(t *testing.T){
	var grp []byte
	grp=append(grp,sepxSprm(0xB01F,u16(12240)...)...)
	grp=append(grp,sepxSprm(0xB020,u16(15840)...)...)
	grp=append(grp,sepxSprm(0x301D,1)...)
	grp=append(grp,sepxSprm(0xB021,u16(1440)...)...)
	grp=append(grp,sepxSprm(0xB022,u16(1080)...)...)
	grp=append(grp,sepxSprm(0x9023,u16(720)...)...)
	grp=append(grp,sepxSprm(0x9024,u16(900)...)...)
	grp=append(grp,sepxSprm(0x500B,u16(1)...)...) // two columns
	grp=append(grp,sepxSprm(0x900C,u16(360)...)...)
	grp=append(grp,sepxSprm(0x3019,1)...)
	grp=append(grp,sepxSprm(0x3009,2)...)
	sec,rev:=DecodeSEPXCore(grp)
	if rev!=nil{t.Fatalf("revision=%+v",rev)}
	if sec.PageSize==nil||sec.PageSize.Width!=12240||sec.PageSize.Height!=15840||sec.PageSize.Orient!="landscape"{t.Fatalf("page=%+v",sec.PageSize)}
	if sec.PageMargin==nil||sec.PageMargin.Left!=1440||sec.PageMargin.Right!=1080||sec.PageMargin.Top!=720||sec.PageMargin.Bottom!=900{t.Fatalf("margins=%+v",sec.PageMargin)}
	if sec.Columns==nil||sec.Columns.Num!=2||sec.Columns.Space!=360||!sec.Columns.Sep{t.Fatalf("columns=%+v",sec.Columns)}
	if sec.BreakKind!="nextPage"||sec.Type!="nextPage"{t.Fatalf("break=%q type=%q",sec.BreakKind,sec.Type)}
}

func TestDecodeSEPXCoreRevisionMetadata(t *testing.T){
	// sprmSPropRMark is a variable-length SPRM: the operand starts with cb,
	// followed by fPropRMark, ibstPropRMark and dttmPropRMark.
	op:=[]byte{7,1,7,0,4,3,2,1}
	grp:=sepxSprm(0xD227,op...)
	_,rev:=DecodeSEPXCore(grp)
	if rev==nil||!rev.Present||!rev.HasIbst||rev.Ibst!=7||!rev.HasDttm||rev.Dttm!=0x01020304{t.Fatalf("revision=%+v",rev)}
}
