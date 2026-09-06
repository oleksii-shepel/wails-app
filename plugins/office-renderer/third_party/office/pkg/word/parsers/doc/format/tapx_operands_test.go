package docformat

import (
	"encoding/binary"
	"testing"
)

func TestDecodePAPXCoreAdvancedTableMergeAndWidth(t *testing.T) {
	grp := make([]byte, 0, 16)
	grp = append(grp, 0x24, 0x56, 0x00, 0x02) // sprmTMerge: cells [0,2)
	grp = append(grp, 0x35, 0xD6, 0x05, 0x00, 0x02, 0x03, 0xB0, 0x04) // sprmTCellWidth: 1200 dxa
	_, meta := DecodePAPXCore(grp)
	if meta.TableRowProps == nil { t.Fatal("missing table row props") }
	if got := meta.TableRowProps.HMergeState(0); got != 2 { t.Fatalf("merge start=%d, want 2", got) }
	if got := meta.TableRowProps.HMergeState(1); got != 1 { t.Fatalf("merge continuation=%d, want 1", got) }
	for i:=0;i<2;i++ {
		f:=meta.TableRowProps.CellFormat(i)
		if f.Width==nil || *f.Width!=1200 || f.WidthType!="dxa" { t.Fatalf("cell %d width=%v type=%q",i,f.Width,f.WidthType) }
	}
}

func TestParseTDefTableOperandPreservesGridAndCellFlags(t *testing.T) {
	op:=make([]byte, 3+(2+1)*2+2*20)
	op[0]=byte(len(op)-1)
	op[2]=2
	binary.LittleEndian.PutUint16(op[3:5],0)
	binary.LittleEndian.PutUint16(op[5:7],1000)
	binary.LittleEndian.PutUint16(op[7:9],2200)
	tc:=9
	flags:=uint16(2 | (1<<7) | (3<<9))
	binary.LittleEndian.PutUint16(op[tc:tc+2],flags)
	binary.LittleEndian.PutUint16(op[tc+2:tc+4],1000)
	p:=parseTDefTableOperand(op)
	if p==nil { t.Fatal("expected row props") }
	if len(p.CellBoundaries)!=3 || p.CellBoundaries[2]!=2200 { t.Fatalf("boundaries=%v",p.CellBoundaries) }
	if p.HMergeState(0)!=2 { t.Fatalf("hmerge=%d",p.HMergeState(0)) }
	f:=p.CellFormat(0)
	if f.VertAlign!="center" || f.WidthType!="dxa" || f.Width==nil || *f.Width!=1000 { t.Fatalf("format=%+v",f) }
}
