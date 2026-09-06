package docformat

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
)

func parseTDefTableOperand(op []byte) *TableRowProps {
	if len(op) < 5 { return nil }
	colCount := int(op[2]); if colCount <= 0 { return nil }
	tcOffset := 3 + (colCount+1)*2; if tcOffset > len(op) { return nil }
	props := &TableRowProps{CellBoundaries: make([]int,colCount+1)}
	for i:=0;i<=colCount;i++ { off:=3+i*2; if off+2>len(op){props.CellBoundaries=nil;break}; props.CellBoundaries[i]=int(int16(binary.LittleEndian.Uint16(op[off:off+2]))) }
	if props.ValidCellBoundaries()==nil { props.CellBoundaries=nil }
	for i:=0;i<colCount;i++ {
		off:=tcOffset+i*20; if off+20>len(op){break}
		flags:=binary.LittleEndian.Uint16(op[off:off+2])
		switch byte(flags&0x3) {
		case 1:
			if props.HMerge==nil { props.HMerge=map[int]byte{} }
			props.HMerge[i]=1
		case 2,3:
			if props.HMerge==nil { props.HMerge=map[int]byte{} }
			props.HMerge[i]=2
		}
		switch byte((flags>>5)&0x3) { case 1: props.SetVMerge(i,1); case 3: props.SetVMerge(i,3) }
		var f wordir.TableCellFormat
		f.TextDirection=tableTextDirectionFromFlow(uint16((flags>>2)&0x7))
		switch byte((flags>>7)&0x3) { case 1:f.VertAlign="center"; case 2:f.VertAlign="bottom" }
		switch byte((flags>>9)&0x7) { case 2:f.WidthType="pct"; case 3:f.WidthType="dxa" }
		if f.WidthType!="" { w:=int(binary.LittleEndian.Uint16(op[off+2:off+4])); f.Width=&w }
		b:=&wordir.CellBorders{Top:parseBRC80MayBeNil(op[off+4:off+8]),Left:parseBRC80MayBeNil(op[off+8:off+12]),Bottom:parseBRC80MayBeNil(op[off+12:off+16]),Right:parseBRC80MayBeNil(op[off+16:off+20])}
		if cellBordersVisible(b){f.Borders=b}
		props.SetCellFormat(i,f)
	}
	if !props.HasData(){return nil}; return props
}

func parseTablePaddingDefault(op []byte) *TableRowProps {
	if len(op)<7 || int(op[0])<6 || len(op)<1+int(op[0]) { return nil }
	p:=op[1:]; if len(p)<6 || p[3]!=3 { return nil }
	w:=int(int16(binary.LittleEndian.Uint16(p[4:6]))); if w<=0{return nil}
	out:=&TableRowProps{}
	if p[2]&1!=0 { v:=w; out.CellPadTop=&v }; if p[2]&2!=0 { v:=w; out.CellPadLeft=&v }; if p[2]&4!=0 { v:=w; out.CellPadBottom=&v }; if p[2]&8!=0 { v:=w; out.CellPadRight=&v }
	if !out.HasData(){return nil}; return out
}

func parseCellRange(op []byte) (int,int,bool) { if len(op)<2{return 0,0,false}; a,b:=int(op[0]),int(op[1]); if b<=a{return 0,0,false}; return a,b,true }
func tableTextDirectionFromFlow(v uint16) string { switch v {case 1:return "tbRl";case 3:return "btLr";case 5:return "tbRlV"}; return "" }
func cellBordersVisible(b *wordir.CellBorders) bool { return b!=nil && (b.Top!=nil||b.Left!=nil||b.Bottom!=nil||b.Right!=nil) }

func parseTableCellWidth(op []byte)(int,int,wordir.TableCellFormat,bool){
	if len(op)<6||op[0]!=5{return 0,0,wordir.TableCellFormat{},false}; a,b,ok:=parseCellRange(op[1:3]); if !ok{return 0,0,wordir.TableCellFormat{},false}
	f:=wordir.TableCellFormat{}; switch op[3]{case 2:f.WidthType="pct";case 3,0x13:f.WidthType="dxa";default:return 0,0,f,false}; w:=int(int16(binary.LittleEndian.Uint16(op[4:6]))); if w<0{w=0}; f.Width=&w; return a,b,f,true
}
func parseTableVertAlign(op []byte)(int,int,string,bool){ if len(op)<4||op[0]!=3{return 0,0,"",false};a,b,ok:=parseCellRange(op[1:3]);if !ok{return 0,0,"",false};v:="top";if op[3]==1{v="center"}else if op[3]==2{v="bottom"};return a,b,v,true }
func parseTableTextFlow(op []byte)(int,int,string,bool){ if len(op)<4{return 0,0,"",false};a,b,ok:=parseCellRange(op[:2]);if !ok{return 0,0,"",false};d:=tableTextDirectionFromFlow(binary.LittleEndian.Uint16(op[2:4]));if d==""{return 0,0,"",false};return a,b,d,true }

func applyAdvancedTAPX(code uint16, op []byte, meta *PAPXMetadata) {
	switch code {
	case 0xD634:
		if p:=parseTablePaddingDefault(op);p!=nil { r:=ensureRow(meta); if p.CellPadTop!=nil{r.CellPadTop=p.CellPadTop}; if p.CellPadBottom!=nil{r.CellPadBottom=p.CellPadBottom}; if p.CellPadLeft!=nil{r.CellPadLeft=p.CellPadLeft}; if p.CellPadRight!=nil{r.CellPadRight=p.CellPadRight} }
	case 0x5624:
		if len(op)>=2&&op[1]>op[0]{ensureRow(meta).SetHMerge(int(op[0]),int(op[1]))}
	case 0x5625:
		if len(op)>=2&&op[1]>=op[0]{ensureRow(meta).ClearHMerge(int(op[0]),int(op[1]))}
	case 0xD62B:
		if len(op)>=3&&op[0]==2{ensureRow(meta).SetVMerge(int(op[1]),op[2])}
	case 0xD608:
		if p:=parseTDefTableOperand(op);p!=nil { meta.TableRowProps=MergeTableRowProps(meta.TableRowProps,p) }
	case 0xD62C:
		if a,b,v,ok:=parseTableVertAlign(op);ok{ensureRow(meta).ApplyCellFormatRange(a,b,wordir.TableCellFormat{VertAlign:v},false)}
	case 0x7629:
		if a,b,v,ok:=parseTableTextFlow(op);ok{ensureRow(meta).ApplyCellFormatRange(a,b,wordir.TableCellFormat{TextDirection:v},false)}
	case 0xD635:
		if a,b,f,ok:=parseTableCellWidth(op);ok{ensureRow(meta).ApplyCellFormatRange(a,b,f,false)}
	}
}
