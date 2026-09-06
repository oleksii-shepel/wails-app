package docformat

import (
	"encoding/binary"
	"testing"

	wordir "office/pkg/word/ir"
)

func papxSprm(code uint16, op ...byte) []byte {
	b:=make([]byte,2+len(op)); binary.LittleEndian.PutUint16(b,code); copy(b[2:],op); return b
}

func TestDecodePAPXCoreParagraphAndTableMetadata(t *testing.T) {
	var grp []byte
	grp=append(grp,papxSprm(0x2403,1)...)
	grp=append(grp,papxSprm(0x2405,1)...)
	grp=append(grp,papxSprm(0x840E,0xD0,0x07)...)
	grp=append(grp,papxSprm(0x2416,1)...)
	grp=append(grp,papxSprm(0x6649,2,0,0,0)...)
	grp=append(grp,papxSprm(0x3403,1)...)
	format,meta:=DecodePAPXCore(grp)
	if format.Alignment!="center" || !format.KeepLines || format.IndentLeft==nil || *format.IndentLeft!=2000 || !format.InTable { t.Fatalf("paragraph decode mismatch: %+v",format) }
	if meta.TableDepth!=2 || meta.TableRowProps==nil || meta.TableRowProps.CantSplit==nil || !*meta.TableRowProps.CantSplit { t.Fatalf("table metadata mismatch: %+v",meta) }
}

func TestDecodePAPXPreservesRawSHDColorBytes(t *testing.T) {
	// sprmTSetShd operand: cb=12, cells [0,2), then a 10-byte solid SHD.
	// The SHD begins with 0xFF (red component of FFF2CC); that byte is data,
	// not another variable-length prefix.
	shd:=[]byte{0xFF,0xF2,0xCC,0x00,0xFF,0xF2,0xCC,0x00,0x01,0x00}
	op:=append([]byte{12,0,2},shd...)
	grp:=papxSprm(0xD62D,op...)
	_,meta:=DecodePAPX(grp)
	if meta.TableRowProps==nil { t.Fatal("missing table row props") }
	for i:=0;i<2;i++ {
		if got:=meta.TableRowProps.CellFormat(i).Shading;got!="FFF2CC" { t.Fatalf("cell %d shading = %q, want FFF2CC",i,got) }
	}
}

func TestSequentialTableRowOperations(t *testing.T) {
	p:=&TableRowProps{CellBoundaries:[]int{0,1000,2200}}
	p.SetHMerge(0,2)
	p.SetVMerge(1,3)
	w:=1200
	p.SetCellFormat(1,wordir.TableCellFormat{Width:&w,WidthType:"dxa",VertAlign:"center"})
	if p.PhysicalCellCount()!=2 || p.HMergeState(0)!=2 || p.HMergeState(1)!=1 || p.VMergeState(1)!=3 { t.Fatalf("merge state mismatch: %+v",p) }
	f:=p.CellFormat(1)
	if f.Width==nil || *f.Width!=1200 || f.VertAlign!="center" { t.Fatalf("cell format mismatch: %+v",f) }
	clone:=p.Clone(); p.ClearHMerge(0,2)
	if clone.HMergeState(0)!=2 || p.HMergeState(0)!=0 { t.Fatal("clone/clear should be independent") }
}

func TestMergeSequentialTableRowProps(t *testing.T) {
	left:=120; right:=240; split:=true
	base:=&TableRowProps{TableDxaLeft:&left}
	overlay:=&TableRowProps{TableDxaGap:&right,CantSplit:&split,TableJc:"center"}
	got:=MergeTableRowProps(base,overlay)
	if got.TableDxaLeft==nil || *got.TableDxaLeft!=120 || got.TableDxaGap==nil || *got.TableDxaGap!=240 || got.TableJc!="center" || got.CantSplit==nil || !*got.CantSplit { t.Fatalf("merge mismatch: %+v",got) }
}
