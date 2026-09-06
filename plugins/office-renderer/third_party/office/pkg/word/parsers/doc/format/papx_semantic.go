package docformat

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
)

// DecodePAPXCore decodes Word97+ paragraph and table semantics that are
// independent of the legacy materializer. Advanced TAPX row/cell operands are
// handled in this package as part of the same pass.
func DecodePAPXCore(grpprl []byte) (wordir.ParagraphFormat, PAPXMetadata) {
	var out wordir.ParagraphFormat
	var meta PAPXMetadata
	for pos:=0; pos+2<=len(grpprl); {
		code:=binary.LittleEndian.Uint16(grpprl[pos:pos+2]); pos+=2
		size:=OperandSize(code,grpprl,pos)
		if code==0x8426 || code==0x6412 || code==0x6649 || code==0x664A { size=4 }
		if size<0 || pos+size>len(grpprl) { break }
		op:=grpprl[pos:pos+size]; pos+=size
		switch code {
		case 0x260A:
			if len(op)>0 { v:=int(op[0]); out.BinaryListLevel=&v }
		case 0x460B:
			if len(op)>=2 { v:=int(binary.LittleEndian.Uint16(op)); out.BinaryListOverride=&v }
		case 0x2403,0x2461:
			if len(op)>0 { out.Alignment=paragraphAlignment(op[0]) }
		case 0x2405:
			if len(op)>0 { out.KeepLines=op[0]!=0 }
		case 0x2406:
			if len(op)>0 { out.KeepNext=op[0]!=0 }
		case 0x2407:
			if len(op)>0 { out.PageBreakBefore=op[0]!=0 }
		case 0x840E,0x845E:
			if len(op)>=2 { v:=int(int16(binary.LittleEndian.Uint16(op))); out.IndentLeft=&v }
		case 0x840F,0x845D:
			if len(op)>=2 { v:=int(int16(binary.LittleEndian.Uint16(op))); out.IndentRight=&v }
		case 0x8411,0x8460:
			if len(op)>=2 { applyFirstLineIndent(&out,int(int16(binary.LittleEndian.Uint16(op)))) }
		case 0xA413:
			if len(op)>=2 { v:=int(binary.LittleEndian.Uint16(op)); if v>0 { out.SpacingBefore=&v } }
		case 0xA414:
			if len(op)>=2 { v:=int(binary.LittleEndian.Uint16(op)); if v>0 { out.SpacingAfter=&v } }
		case 0x8426,0x6412:
			if len(op)>=4 { applyLineSpacing(&out,op) }
		case 0x2416:
			if len(op)>0 { out.InTable=op[0]!=0 }
		case 0x2417:
			if len(op)>0 { out.IsTableTrailingParagraph=op[0]!=0 }
		case 0x244B:
			if len(op)>0 { out.IsTableCellTrailingParagraph=op[0]!=0 }
		case 0x244C:
			if len(op)>0 { out.IsTableTrailingParagraph=op[0]!=0 }
		case 0x240C:
			if len(op)>0 { out.NoLineNumber=op[0]!=0 }
		case 0x242A:
			if len(op)>0 { out.NoAutoHyphenation=op[0]!=0 }
		case 0x2431:
			if len(op)>0 { out.WidowControl=op[0]!=0 }
		case 0x2441:
			if len(op)>0 { out.Bidi=op[0]!=0 }
		case 0x246D:
			if len(op)>0 { out.ContextualSpacing=op[0]!=0 }
		case 0x2470:
			if len(op)>0 { out.MirrorIndents=op[0]!=0 }
		case 0x2462:
			if len(op)>0 { out.NoAllowOverlap=op[0]!=0 }
		case 0x245B:
			if len(op)>0 { out.SpacingBeforeAuto=op[0]!=0 }
		case 0x245C:
			if len(op)>0 { out.SpacingAfterAuto=op[0]!=0 }
		case 0x2640:
			if len(op)>0 { v:=int(op[0]); out.OutlineLevel=&v }
		case 0x6649:
			if len(op)>=4 { meta.TableDepth=int(binary.LittleEndian.Uint32(op)) }
		case 0x664A:
			if len(op)>=4 { meta.TableDepth+=int(int32(binary.LittleEndian.Uint32(op))) }
		case 0x5400,0x549A:
			if len(op)>0 { ensureRow(&meta).TableJc=tableJustification(op[0]) }
		case 0x9407:
			if len(op)>=2 { v:=int(int16(binary.LittleEndian.Uint16(op))); ensureRow(&meta).RowHeight=&v }
		case 0x3403:
			if len(op)>0 { v:=op[0]!=0; ensureRow(&meta).CantSplit=&v }
		case 0x3404:
			if len(op)>0 { v:=op[0]!=0; ensureRow(&meta).TableHeader=&v }
		case 0x9601:
			if len(op)>=2 { v:=int(int16(binary.LittleEndian.Uint16(op))); ensureRow(&meta).TableDxaLeft=&v }
		case 0x9602:
			if len(op)>=2 { v:=int(int16(binary.LittleEndian.Uint16(op))); ensureRow(&meta).TableDxaGap=&v }
		case 0x3615:
			if len(op)>0 { v:=op[0]!=0; ensureRow(&meta).AutoFit=&v }
		case 0x560B,0x5664:
			if len(op)>=2 { v:=binary.LittleEndian.Uint16(op)!=0; ensureRow(&meta).BidiVisual=&v }
		case 0xF614:
			if len(op)>=3 { applyTableWidth(ensureRow(&meta),op) }
		case 0xC63F,0xC66F:
			if mark:=decodePendingRevision(op); mark!=nil { meta.PropertyRevision=mark }
		case 0xD667:
			if mark:=decodePendingRevision(op); mark!=nil { ensureRow(&meta).PropertyRevision=mark }
		default:
			applyAdvancedTAPX(code,op,&meta)
		}
	}
	return out,meta
}

func paragraphAlignment(v byte) string { switch v { case 1:return "center"; case 2:return "right"; case 3:return "both" }; return "" }
func tableJustification(v byte) string { switch v { case 0:return "left"; case 1:return "center"; case 2:return "right" }; return "" }
func ensureRow(m *PAPXMetadata) *TableRowProps { if m.TableRowProps==nil { m.TableRowProps=&TableRowProps{} }; return m.TableRowProps }
func applyFirstLineIndent(out *wordir.ParagraphFormat,v int) { if v<0 { h:=-v; out.IndentHanging=&h } else if v>0 { out.IndentFirstLine=&v } }
func applyLineSpacing(out *wordir.ParagraphFormat,op []byte) { dy:=int(int16(binary.LittleEndian.Uint16(op[:2]))); mult:=int(int16(binary.LittleEndian.Uint16(op[2:4]))); if mult!=0 { v:=dy; out.LineSpacing=&v; out.LineRule="auto" } else if dy>0 { v:=dy; out.LineSpacing=&v; out.LineRule="exact" } else if dy<0 { v:=-dy; out.LineSpacing=&v; out.LineRule="atLeast" } }
func applyTableWidth(row *TableRowProps,op []byte) { w:=int(binary.LittleEndian.Uint16(op[1:3])); switch op[0] { case 1:row.TableWidthType="nil"; case 2:row.TableWidthType="auto"; case 3:row.TableWidthType="dxa"; row.TableWidth=&w; case 4:row.TableWidthType="pct"; row.TableWidth=&w } }
func decodePendingRevision(op []byte) *PendingRevisionMark { payload:=op; if len(payload)>=8 && int(payload[0])>=7 { payload=payload[1:] }; if len(payload)<7 { return nil }; return &PendingRevisionMark{Present:payload[0]!=0,Ibst:binary.LittleEndian.Uint16(payload[1:3]),HasIbst:true,Dttm:binary.LittleEndian.Uint32(payload[3:7]),HasDttm:true} }
