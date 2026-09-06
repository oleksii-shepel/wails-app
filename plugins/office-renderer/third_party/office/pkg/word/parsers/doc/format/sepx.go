package docformat

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
)

// DecodeSEPXCore decodes layout-critical Word97 section SPRMs. Rare page-border
// and revision-attribution semantics may be layered by a compatibility caller,
// but page geometry and pagination no longer require the legacy parser.
func DecodeSEPXCore(grpprl []byte) (wordir.Section, *PendingRevisionMark) {
	var sec wordir.Section
	var revision *PendingRevisionMark
	var pageWidth,pageHeight int
	var marginTop,marginBottom,marginLeft,marginRight,marginHeader,marginFooter,marginGutter int
	var orientation byte
	var numCols,colSpacing int
	var colSep bool
	var colEvenlySpaced *bool
	colWidths:=map[int]int{};colSpaces:=map[int]int{}
	hasPage,hasMargin,hasCols:=false,false,false
	var lnCountBy,lnStart,lnDistance int;var lnRestart string;hasLineNumbering:=false
	var gridLinePitch,gridCharSpace int;var gridType string;hasDocGrid:=false
	var pgNumStart int;hasPgNumStart:=false;var pgNumRestart,titlePage bool
	var breakKind,verticalAlign string;var sectionBidi,rtlGutter bool
	var footnotePos byte;hasFtnPos:=false;var footnoteNumStart,endnoteNumStart int;var footnoteRestart,endnoteRestart string
	var endnoteAtSectionEnd,formProt bool

	for pos:=0;pos+2<=len(grpprl);{
		code:=binary.LittleEndian.Uint16(grpprl[pos:pos+2]);pos+=2
		size:=OperandSize(code,grpprl,pos);if size<=0||pos+size>len(grpprl){break};op:=grpprl[pos:pos+size];pos+=size
		switch code{
		case 0xB01F: if len(op)>=2{pageWidth=int(binary.LittleEndian.Uint16(op));hasPage=true}
		case 0xB020: if len(op)>=2{pageHeight=int(binary.LittleEndian.Uint16(op));hasPage=true}
		case 0xB021: if len(op)>=2{marginLeft=int(binary.LittleEndian.Uint16(op));hasMargin=true}
		case 0x9023: if len(op)>=2{marginTop=int(int16(binary.LittleEndian.Uint16(op)));hasMargin=true}
		case 0xB022: if len(op)>=2{marginRight=int(binary.LittleEndian.Uint16(op));hasMargin=true}
		case 0x9024: if len(op)>=2{marginBottom=int(int16(binary.LittleEndian.Uint16(op)));hasMargin=true}
		case 0xB017: if len(op)>=2{marginHeader=int(binary.LittleEndian.Uint16(op));hasMargin=true}
		case 0xB018: if len(op)>=2{marginFooter=int(binary.LittleEndian.Uint16(op));hasMargin=true}
		case 0xB025: if len(op)>=2{marginGutter=int(binary.LittleEndian.Uint16(op));hasMargin=true}
		case 0x301D: if len(op)>=1{orientation=op[0]}
		case 0x500B: if len(op)>=2{numCols=int(int16(binary.LittleEndian.Uint16(op)))+1;hasCols=true}
		case 0x900C: if len(op)>=2{colSpacing=int(int16(binary.LittleEndian.Uint16(op)));hasCols=true}
		case 0x3019: if len(op)>=1{colSep=op[0]!=0;hasCols=true}
		case 0xF203: if len(op)>=3{idx:=int(op[0]);colWidths[idx]=int(binary.LittleEndian.Uint16(op[1:3]));if idx+1>numCols{numCols=idx+1};hasCols=true}
		case 0xF204: if len(op)>=3{idx:=int(op[0]);colSpaces[idx]=int(binary.LittleEndian.Uint16(op[1:3]));if idx+2>numCols{numCols=idx+2};hasCols=true}
		case 0x3005: if len(op)>=1{v:=op[0]!=0;colEvenlySpaced=&v;hasCols=true}
		case 0x5015: if len(op)>=2{lnCountBy=int(binary.LittleEndian.Uint16(op));hasLineNumbering=true}
		case 0x9016: if len(op)>=2{lnDistance=int(int16(binary.LittleEndian.Uint16(op)));hasLineNumbering=true}
		case 0x501B: if len(op)>=2{lnStart=int(int16(binary.LittleEndian.Uint16(op)));hasLineNumbering=true}
		case 0x3013: if len(op)>=1{switch op[0]{case 0:lnRestart="newPage";case 1:lnRestart="newSection";case 2:lnRestart="continuous"};hasLineNumbering=true}
		case 0x9031: if len(op)>=2{gridLinePitch=int(int16(binary.LittleEndian.Uint16(op)));hasDocGrid=true}
		case 0x7030: if len(op)>=4{gridCharSpace=int(int32(binary.LittleEndian.Uint32(op)));hasDocGrid=true}
		case 0x5032: if len(op)>=2{switch binary.LittleEndian.Uint16(op){case 0:gridType="default";case 1:gridType="lines";case 2:gridType="linesAndChars";case 3:gridType="snapToChars"};hasDocGrid=true}
		case 0x501C: if len(op)>=2{pgNumStart=int(binary.LittleEndian.Uint16(op));hasPgNumStart=true}
		case 0x7044: if len(op)>=4{pgNumStart=int(binary.LittleEndian.Uint32(op));hasPgNumStart=true}
		case 0x3011: if len(op)>=1{pgNumRestart=op[0]!=0}
		case 0x300A: if len(op)>=1{titlePage=op[0]!=0}
		case 0x3009: if len(op)>=1{switch op[0]{case 0:breakKind="continuous";case 1:breakKind="nextColumn";case 2:breakKind="nextPage";case 3:breakKind="evenPage";case 4:breakKind="oddPage"}}
		case 0x301A: if len(op)>=1{switch op[0]{case 0:verticalAlign="top";case 1:verticalAlign="center";case 2:verticalAlign="both";case 3:verticalAlign="bottom"}}
		case 0x3228: if len(op)>=1{sectionBidi=op[0]!=0}
		case 0x322A: if len(op)>=1{rtlGutter=op[0]!=0}
		case 0x303B: if len(op)>=1{footnotePos=op[0];hasFtnPos=true}
		case 0x303C: if len(op)>=1{switch op[0]{case 0:footnoteRestart="continuous";case 1:footnoteRestart="eachSect";case 2:footnoteRestart="eachPage"}}
		case 0x303E: if len(op)>=1{switch op[0]{case 0:endnoteRestart="continuous";case 1:endnoteRestart="eachSect"}}
		case 0x503F: if len(op)>=2{footnoteNumStart=int(binary.LittleEndian.Uint16(op))}
		case 0x5041: if len(op)>=2{endnoteNumStart=int(binary.LittleEndian.Uint16(op))}
		case 0x3012: if len(op)>=1{endnoteAtSectionEnd=op[0]!=0}
		case 0x3006: if len(op)>=1{formProt=op[0]!=0}
		case 0xD227,0xD243:
			payload:=op;if len(op)>=8&&int(op[0])>=7{payload=op[1:]};if len(payload)>=7{revision=&PendingRevisionMark{Present:payload[0]!=0,Ibst:binary.LittleEndian.Uint16(payload[1:3]),HasIbst:true,Dttm:binary.LittleEndian.Uint32(payload[3:7]),HasDttm:true}}else if len(payload)>=1{revision=&PendingRevisionMark{Present:payload[0]!=0}}
		}
	}
	if hasPage{orient:="portrait";if orientation!=0{orient="landscape"};sec.PageSize=&wordir.PageSize{Width:pageWidth,Height:pageHeight,Orient:orient}}
	if hasMargin{sec.PageMargin=&wordir.PageMargin{Top:marginTop,Bottom:marginBottom,Left:marginLeft,Right:marginRight,Header:marginHeader,Footer:marginFooter,Gutter:marginGutter,RtlGutter:rtlGutter}}
	if hasCols{if numCols<1{numCols=1};cols:=&wordir.Columns{Num:numCols,Space:colSpacing,Sep:colSep,EvenlySpaced:colEvenlySpaced};if len(colWidths)>0{cols.Widths=make([]int,numCols);for i,v:=range colWidths{if i>=0&&i<len(cols.Widths){cols.Widths[i]=v}}};if len(colSpaces)>0&&numCols>1{cols.Spaces=make([]int,numCols-1);for i,v:=range colSpaces{if i>=0&&i<len(cols.Spaces){cols.Spaces[i]=v}}};sec.Columns=cols}
	if hasLineNumbering{sec.LineNumbering=&wordir.LineNumbering{CountBy:lnCountBy,Start:lnStart,Restart:lnRestart,Distance:lnDistance}}
	if hasDocGrid{sec.DocGrid=&wordir.DocGrid{LinePitch:gridLinePitch,CharSpace:gridCharSpace,Type:gridType}}
	if hasPgNumStart{sec.PageNumberStart=&pgNumStart};sec.PageNumberRestart=pgNumRestart;sec.TitlePage=titlePage;sec.BreakKind=breakKind;sec.Type=breakKind;sec.VerticalAlign=verticalAlign;sec.Bidi=sectionBidi
	if hasFtnPos{switch footnotePos{case 0:sec.FootnotePos="sectEnd";case 2:sec.FootnotePos="belowText";default:sec.FootnotePos="pageBottom"}}
	sec.FootnoteNumStart=footnoteNumStart;sec.FootnoteRestart=footnoteRestart;sec.EndnoteNumStart=endnoteNumStart;sec.EndnoteRestart=endnoteRestart;sec.EndnoteAtSectionEnd=endnoteAtSectionEnd;sec.FormProt=formProt
	return sec,revision
}
