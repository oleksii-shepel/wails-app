package docformat

import (
	"encoding/binary"
	"fmt"
	"sort"

	wordir "office/pkg/word/ir"
)

// DecodePAPX is the canonical Word97+ PAPX/TAPX semantic decoder used by the
// sequential DOC parser. DecodePAPXCore owns the common paragraph/table flags;
// this pass fills the parity-sensitive long tail (tabs, frames, borders,
// shading, floating tables, and advanced cell formatting).
func DecodePAPX(grpprl []byte) (wordir.ParagraphFormat, PAPXMetadata) {
	out, meta := DecodePAPXCore(grpprl)
	for pos := 0; pos+2 <= len(grpprl); {
		code := binary.LittleEndian.Uint16(grpprl[pos : pos+2])
		pos += 2
		size := OperandSize(code, grpprl, pos)
		if code == 0x8426 || code == 0x6412 || code == 0x6649 || code == 0x664A { size = 4 }
		if size < 0 || pos+size > len(grpprl) { break }
		op := grpprl[pos : pos+size]
		pos += size

		switch code {
		case 0xC615: // sprmPChgTabs
			out.TabStops = applyTabChange(out.TabStops, op, 2)
		case 0x2423: // sprmPWr
			if len(op) > 0 { ensureFrame(&out).WrapType = frameWrapType(op[0]) }
		case 0x465F, 0x4610: // sprmPNest / sprmPNest80
			if len(op) >= 2 { addLeftIndent(&out, int(int16(binary.LittleEndian.Uint16(op)))) }
		case 0x8418: // sprmPDxaAbs
			if len(op) >= 2 { ensureFrame(&out).X = int(int16(binary.LittleEndian.Uint16(op))) }
		case 0x8419: // sprmPDyaAbs
			if len(op) >= 2 { ensureFrame(&out).Y = int(int16(binary.LittleEndian.Uint16(op))) }
		case 0x841A: // sprmPDxaWidth
			if len(op) >= 2 { v:=int(int16(binary.LittleEndian.Uint16(op))); if v<0 { v=-v }; ensureFrame(&out).W=v }
		case 0x261B: // sprmPPc
			if len(op)>0 { f:=ensureFrame(&out); f.XAlign=frameXAlign(op[0]&0x03); f.YAlign=frameYAlign((op[0]>>2)&0x03) }
		case 0x442B: // sprmPWHeightAbs
			if len(op)>=2 { v:=int(int16(binary.LittleEndian.Uint16(op))); if v<0 { v=-v }; ensureFrame(&out).H=v }
		case 0x442C: // sprmPDcs
			if len(op)>=2 { d:=binary.LittleEndian.Uint16(op); f:=ensureFrame(&out); switch byte(d&0x07){case 0:f.DropCap="none";case 2:f.DropCap="margin";default:f.DropCap="drop"}; f.Lines=int((d>>3)&0x1f) }
		case 0x842E: // sprmPDyaFromText
			if len(op)>=2 { v:=int(int16(binary.LittleEndian.Uint16(op))); if v<0 {v=-v}; ensureFrame(&out).VSpace=v }
		case 0x842F: // sprmPDxaFromText
			if len(op)>=2 { v:=int(int16(binary.LittleEndian.Uint16(op))); if v<0 {v=-v}; ensureFrame(&out).HSpace=v }
		case 0x443A: // sprmPFrameTextFlow
			if len(op)>=2 { ensureFrame(&out).TextDirection=tableTextDirectionFromFlow(binary.LittleEndian.Uint16(op)) }
		case 0x4439: // sprmPWAlignFont
			if len(op)>=2 { out.FontAlign=fontAlign(binary.LittleEndian.Uint16(op)) }
		case 0x6424,0x6425,0x6426,0x6427: // paragraph BRC80 sides
			if side:=parseBRC80(op); side!=nil { ensureParagraphBorders(&out, code, side) }
		case 0xC64E,0xC64F,0xC650,0xC651: // paragraph BRC sides
			if side:=parseBRC(op); side!=nil { ensureParagraphBorders(&out, code, side) }
		case 0x442D: // sprmPShd80
			if shade:=shd80(op); shade!="" { out.Shading=shade }
		case 0xC60D,0xC64D: // sprmPShd
			if shade:=paragraphShading(op); shade!="" { out.Shading=shade }
		case 0x360D: // sprmTPc
			applyTablePositionCode(op, &meta)
		case 0x940E: // sprmTDxaAbs
			if len(op)>=2 { applyTableAbsX(int(int16(binary.LittleEndian.Uint16(op))), ensureTableFloat(&meta)) }
		case 0x940F: // sprmTDyaAbs
			if len(op)>=2 { applyTableAbsY(int(int16(binary.LittleEndian.Uint16(op))), ensureTableFloat(&meta)) }
		case 0x9410:
			if len(op)>=2 { ensureTableFloat(&meta).LeftFromText=int(binary.LittleEndian.Uint16(op)) }
		case 0x9411:
			if len(op)>=2 { ensureTableFloat(&meta).TopFromText=int(binary.LittleEndian.Uint16(op)) }
		case 0x941E:
			if len(op)>=2 { ensureTableFloat(&meta).RightFromText=int(binary.LittleEndian.Uint16(op)) }
		case 0x941F:
			if len(op)>=2 { ensureTableFloat(&meta).BottomFromText=int(binary.LittleEndian.Uint16(op)) }
		case 0x3465:
			if len(op)>0 { v:=op[0]!=0; ensureRow(&meta).NoOverlap=&v }
		case 0xD605: // sprmTTableBorders
			if b:=parseTableBorders(op); b!=nil { ensureRow(&meta).TableBorders=b }
		case 0xD609: // sprmTDefTableShd80
			applyDefaultShading80(op, 0, &meta)
		case 0xD612,0xD670:
			applyDefaultShading(op, 0, &meta)
		case 0xD616,0xD671:
			applyDefaultShading(op, 22, &meta)
		case 0xD60C,0xD672:
			applyDefaultShading(op, 44, &meta)
		case 0xD620: // sprmTSetBrc80
			applyCellBorderOperand(op, true, &meta)
		case 0xD62F: // sprmTSetBrc
			applyCellBorderOperand(op, false, &meta)
		case 0xD62D: // sprmTSetShd
			applyCellShadeOperand(op, false, &meta)
		case 0xD62E: // sprmTSetShdOdd
			applyCellShadeOperand(op, true, &meta)
		}
	}
	return out, meta
}

func ensureFrame(out *wordir.ParagraphFormat) *wordir.FrameProps {
	if out.BinaryFrame==nil { out.BinaryFrame=&wordir.FrameProps{HAnchor:"margin",VAnchor:"margin"} }
	return out.BinaryFrame
}
func frameWrapType(v byte) string { switch v {case 1:return "notBeside";case 3:return "none";case 4:return "tight";case 5:return "around"}; return "around" }
func frameXAlign(v byte) string { switch v {case 0:return "left";case 1:return "center";case 2:return "right"}; return "" }
func frameYAlign(v byte) string { switch v {case 0:return "top";case 1:return "center";case 2:return "bottom"}; return "" }
func fontAlign(v uint16) string { switch v {case 0:return "auto";case 1:return "top";case 2:return "center";case 3:return "baseline";case 4:return "bottom"}; return "" }
func addLeftIndent(out *wordir.ParagraphFormat, delta int) { if out.IndentLeft!=nil { v:=*out.IndentLeft+delta; out.IndentLeft=&v } else { out.IndentLeft=&delta } }

func ensureParagraphBorders(out *wordir.ParagraphFormat, code uint16, side *wordir.BorderSide) {
	if out.Borders==nil { out.Borders=&wordir.ParagraphBorders{} }
	switch code { case 0x6424,0xC64E:out.Borders.Top=side; case 0x6425,0xC64F:out.Borders.Left=side; case 0x6426,0xC650:out.Borders.Bottom=side; case 0x6427,0xC651:out.Borders.Right=side }
}
func paragraphShading(op []byte) string {
	p:=variablePayload(op); if len(p)<10{return ""}; ipat:=binary.LittleEndian.Uint16(p[8:10]); if ipat==0||ipat==0xffff{return ""}; cr:=binary.LittleEndian.Uint32(p[4:8]); if ipat==1 {cr=binary.LittleEndian.Uint32(p[:4])}; return colorRef(cr)
}

func ensureTableFloat(meta *PAPXMetadata) *wordir.TableFloatProps {
	r:=ensureRow(meta); if r.FloatProps==nil { r.FloatProps=&wordir.TableFloatProps{HorzAnchor:"text",VertAnchor:"margin"} }; return r.FloatProps
}
func tableHAnchor(v byte) string { switch v {case 0:return "text";case 1:return "margin";case 2:return "page"}; return "" }
func tableVAnchor(v byte) string { switch v {case 0:return "margin";case 1:return "page";case 2:return "text"}; return "" }
func applyTablePositionCode(op []byte, meta *PAPXMetadata) { if len(op)==0{return}; h:=op[0]&0x03; v:=(op[0]>>2)&0x03; if h==3&&v==3{return}; f:=ensureTableFloat(meta); if h!=3{f.HorzAnchor=tableHAnchor(h)}; if v!=3{f.VertAnchor=tableVAnchor(v)} }
func applyTableAbsX(v int, f *wordir.TableFloatProps) { switch v {case 0:f.XAlign="left";case -4:f.XAlign="center";case -8:f.XAlign="right";case -12:f.XAlign="inside";case -16:f.XAlign="outside";default:f.X=v} }
func applyTableAbsY(v int, f *wordir.TableFloatProps) { switch v {case 0:f.YAlign="inline";case -4:f.YAlign="top";case -8:f.YAlign="center";case -12:f.YAlign="bottom";case -16:f.YAlign="inside";case -20:f.YAlign="outside";default:f.Y=v} }

func parseTableBorders(op []byte) *wordir.TableBorders {
	p:=variablePayload(op); if len(p)<24{return nil}; b:=&wordir.TableBorders{Top:parseBRC80MayBeNil(p[0:4]),Left:parseBRC80MayBeNil(p[4:8]),Bottom:parseBRC80MayBeNil(p[8:12]),Right:parseBRC80MayBeNil(p[12:16]),InsideH:parseBRC80MayBeNil(p[16:20]),InsideV:parseBRC80MayBeNil(p[20:24])}; if b.Top==nil&&b.Left==nil&&b.Bottom==nil&&b.Right==nil&&b.InsideH==nil&&b.InsideV==nil{return nil}; return b
}
func applyDefaultShading80(op []byte, base int, meta *PAPXMetadata) { p:=variablePayload(op); if len(p)%2!=0{return}; for i:=0;i<len(p)/2;i++ { if s:=shd80(p[i*2:i*2+2]);s!=""{ensureRow(meta).SetCellFormat(base+i,wordir.TableCellFormat{Shading:s})} } }
func applyDefaultShading(op []byte, base int, meta *PAPXMetadata) { p:=variablePayload(op); if len(p)%10!=0{return}; for i:=0;i<len(p)/10;i++ { if s:=paragraphShading(p[i*10:i*10+10]);s!=""{ensureRow(meta).SetCellFormat(base+i,wordir.TableCellFormat{Shading:s})} } }
func applyCellShadeOperand(op []byte, odd bool, meta *PAPXMetadata) { p:=variablePayload(op); if len(p)<12{return}; a,b,ok:=parseCellRange(p[:2]); if !ok{return}; s:=paragraphShading(p[2:12]); if s==""{return}; ensureRow(meta).ApplyCellFormatRange(a,b,wordir.TableCellFormat{Shading:s},odd) }
func applyCellBorderOperand(op []byte, old bool, meta *PAPXMetadata) {
	p:=variablePayload(op); need:=11; if old{need=7}; if len(p)<need{return}; a,b,ok:=parseCellRange(p[:2]); if !ok{return}; mask:=p[2]; var side *wordir.BorderSide; if old{side=parseBRC80MayBeNil(p[3:7])}else{side=parseBRC(p[3:11])}; borders:=&wordir.CellBorders{}; if mask&1!=0{borders.Top=side};if mask&2!=0{borders.Left=side};if mask&4!=0{borders.Bottom=side};if mask&8!=0{borders.Right=side}; if !cellBordersVisible(borders){return}; ensureRow(meta).ApplyCellFormatRange(a,b,wordir.TableCellFormat{Borders:borders},false)
}

func applyTabChange(base []wordir.TabStop, payload []byte, deleteEntryWidth int) []wordir.TabStop {
	clone:=func()[]wordir.TabStop{if len(base)==0{return nil};out:=make([]wordir.TabStop,len(base));copy(out,base);return out}
	if len(payload)<2||deleteEntryWidth<2{return clone()}; n:=int(payload[0]); off:=1; bytes:=n*deleteEntryWidth; if off+bytes>=len(payload){return clone()}; m:=make(map[int]wordir.TabStop,len(base));for _,t:=range base{m[t.Pos]=t};for i:=0;i<n;i++{e:=payload[off+i*deleteEntryWidth:];if len(e)<2{return clone()};delete(m,int(int16(binary.LittleEndian.Uint16(e[:2]))))};off+=bytes;add:=int(payload[off]);off++;if off+add*3>len(payload){return clone()};desc:=off+add*2;for i:=0;i<add;i++{pos:=int(int16(binary.LittleEndian.Uint16(payload[off+i*2:off+i*2+2])));if pos<=0{continue};d:=payload[desc+i];m[pos]=wordir.TabStop{Pos:pos,Algn:tabAlignment(d),Leader:tabLeader(d)}};if len(m)==0{return nil};keys:=make([]int,0,len(m));for k:=range m{keys=append(keys,k)};sort.Ints(keys);out:=make([]wordir.TabStop,0,len(keys));for _,k:=range keys{out=append(out,m[k])};return out
}
func tabAlignment(v byte) string { switch v&7 {case 1:return "center";case 2:return "right";case 3:return "decimal";case 4:return "bar"};return "left" }
func tabLeader(v byte) string { switch (v>>3)&7 {case 1:return "dot";case 2:return "hyphen";case 3:return "underscore";case 5:return "middleDot"};return "" }

func _papxNativeCompileGuard() { _ = fmt.Sprintf }
