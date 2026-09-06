package docbinary

import "encoding/binary"

const (
	FibPlcSpaMom      = 40
	FibPlcSpaHdr      = 41
	FibDggInfo        = 50
	FibPlcftxbxTxt    = 56
	FibPlcfHdrtxbxTxt = 58
)

// FloatingAnchor is the structural FSPA record carried by PlcfSpa.
type FloatingAnchor struct {
	CP     int
	Shape  uint32
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
	Flags  uint16
	Header     bool
	BX         uint8
	BY         uint8
	Wrap       uint8
	WrapSide   uint8
	RcaSimple  bool
	BelowText  bool
	AnchorLock bool
}

// ParsePlcfSpa decodes the Word97 floating-shape PLC for one story domain.
func ParsePlcfSpa(blob []byte, maxCP int) ([]FloatingAnchor, bool) {
	if len(blob) < 34 || (len(blob)-4)%30 != 0 { return nil, false }
	count := (len(blob)-4)/30
	if count <= 0 { return nil, false }
	cpBytes := (count+1)*4
	if cpBytes+count*26 != len(blob) { return nil, false }
	out := make([]FloatingAnchor,0,count)
	prev := -1
	for i:=0;i<count;i++ {
		cp:=int(binary.LittleEndian.Uint32(blob[i*4:(i+1)*4]))
		if cp<0 || cp>maxCP || cp<prev { return nil,false }
		prev=cp
		off:=cpBytes+i*26
		flags:=binary.LittleEndian.Uint16(blob[off+20:off+22])
		out=append(out,FloatingAnchor{
			CP:cp, Shape:binary.LittleEndian.Uint32(blob[off:off+4]),
			Left:int32(binary.LittleEndian.Uint32(blob[off+4:off+8])),
			Top:int32(binary.LittleEndian.Uint32(blob[off+8:off+12])),
			Right:int32(binary.LittleEndian.Uint32(blob[off+12:off+16])),
			Bottom:int32(binary.LittleEndian.Uint32(blob[off+16:off+20])),
			Flags:flags, Header:flags&0x0001!=0,
			BX:uint8((flags>>1)&0x03), BY:uint8((flags>>3)&0x03),
			Wrap:uint8((flags>>5)&0x0F), WrapSide:uint8((flags>>9)&0x0F),
			RcaSimple:flags&0x2000!=0, BelowText:flags&0x4000!=0, AnchorLock:flags&0x8000!=0,
		})
	}
	last:=int(binary.LittleEndian.Uint32(blob[count*4:(count+1)*4]))
	if last<prev { return nil,false }
	return out,true
}

// TextboxRange is one FTXBXS-owned textbox story interval.
type TextboxRange struct {
	StartCP int
	EndCP   int
	ShapeID uint32
	Header  bool
}

// ParseTextboxPLC decodes PlcftxbxTxt / PlcfHdrtxbxTxt ownership ranges.
func ParseTextboxPLC(blob []byte, header bool) []TextboxRange {
	const recordSize=22
	const lidOffset=14
	if len(blob)<4+recordSize || (len(blob)-4)%26!=0 { return nil }
	count:=(len(blob)-4)/26
	if count<=0 { return nil }
	cpBytes:=4*(count+1)
	if cpBytes+count*recordSize!=len(blob){return nil}
	out:=make([]TextboxRange,0,count)
	for i:=0;i<count;i++{
		start:=int(binary.LittleEndian.Uint32(blob[i*4:(i+1)*4]))
		end:=int(binary.LittleEndian.Uint32(blob[(i+1)*4:(i+2)*4]))
		rec:=blob[cpBytes+i*recordSize:cpBytes+(i+1)*recordSize]
		shape:=binary.LittleEndian.Uint32(rec[lidOffset:lidOffset+4])
		if end<=start || shape==0 { continue }
		out=append(out,TextboxRange{StartCP:start,EndCP:end,ShapeID:shape,Header:header})
	}
	return out
}

// FibRange resolves one FibRgFcLcb fc/lcb pair from WordDocument.
func FibRange(wordDocument []byte, fibBase, cbRgFcLcb, index int) (fc, lcb int, ok bool) {
	if index < 0 || index >= cbRgFcLcb || fibBase < 0 { return 0,0,false }
	off := fibBase + index*8
	if off < 0 || off+8 > len(wordDocument) { return 0,0,false }
	fc = int(binary.LittleEndian.Uint32(wordDocument[off:off+4]))
	lcb = int(binary.LittleEndian.Uint32(wordDocument[off+4:off+8]))
	if fc < 0 || lcb <= 0 { return 0,0,false }
	return fc,lcb,true
}

// FloatingAnchorsAt loads one story-specific PlcfSpa through the canonical FIB
// location and applies a global CP base when needed by callers.
func FloatingAnchorsAt(wordDocument, table []byte, fibBase, cbRgFcLcb, index, cpBase, storyTextLen int, header bool) []FloatingAnchor {
	fc,lcb,ok:=FibRange(wordDocument,fibBase,cbRgFcLcb,index)
	if !ok || fc+lcb>len(table) { return nil }
	anchors,valid:=ParsePlcfSpa(table[fc:fc+lcb],storyTextLen)
	if !valid || len(anchors)==0 { return nil }
	for i:=range anchors { anchors[i].CP += cpBase; if header { anchors[i].Header=true } }
	return anchors
}

// RecoverFloatingAnchors scans malformed/older FIB slot maps for the most
// plausible PlcfSpa candidate. Observed anchor CPs strongly bias selection so
// callers keep the historical compatibility behavior without owning a second
// PlcfSpa scoring implementation.
func RecoverFloatingAnchors(wordDocument, table []byte, fibBase, cbRgFcLcb, maxCP, skipIndex int, observedCPs []int) []FloatingAnchor {
	observed:=make(map[int]struct{},len(observedCPs))
	for _,cp:=range observedCPs { observed[cp]=struct{}{} }
	bestScore:=0
	var best []FloatingAnchor
	for index:=0;index<cbRgFcLcb;index++ {
		if index==skipIndex { continue }
		fc,lcb,ok:=FibRange(wordDocument,fibBase,cbRgFcLcb,index)
		if !ok || fc+lcb>len(table) { continue }
		anchors,valid:=ParsePlcfSpa(table[fc:fc+lcb],maxCP)
		if !valid || len(anchors)==0 { continue }
		score:=scoreFloatingAnchorCandidate(anchors,observed)
		if score<=bestScore { continue }
		bestScore=score
		best=anchors
	}
	return best
}

func scoreFloatingAnchorCandidate(anchors []FloatingAnchor, observed map[int]struct{}) int {
	if len(anchors)==0 { return 0 }
	score:=0
	for _,anchor:=range anchors {
		if _,ok:=observed[anchor.CP];ok { score+=100 }
		if anchor.Shape!=0 { score++ }
		if anchor.Right>anchor.Left && anchor.Bottom>anchor.Top { score+=2 }
		if anchor.Left>=0 && anchor.Top>=0 { score++ }
	}
	return score
}

// TextboxRanges loads both main and header textbox ownership PLCs from their
// authoritative FibRgFcLcb slots. Returned CPs are story-local.
func TextboxRanges(wordDocument, table []byte, fibBase, cbRgFcLcb int) []TextboxRange {
	var out []TextboxRange
	for _,spec:=range []struct{index int;header bool}{{FibPlcftxbxTxt,false},{FibPlcfHdrtxbxTxt,true}} {
		fc,lcb,ok:=FibRange(wordDocument,fibBase,cbRgFcLcb,spec.index)
		if !ok || fc+lcb>len(table) { continue }
		out=append(out,ParseTextboxPLC(table[fc:fc+lcb],spec.header)...)
	}
	return out
}

// AbsoluteTextboxRanges loads textbox ownership and rebases every interval to
// the document-wide CP domain used by sequential story materialization.
func AbsoluteTextboxRanges(wordDocument, table []byte, fibBase, cbRgFcLcb int, counts StoryCounts) []TextboxRange {
	ranges:=TextboxRanges(wordDocument,table,fibBase,cbRgFcLcb)
	for i:=range ranges {
		base:=TextboxStoryBase(counts,ranges[i].Header)
		ranges[i].StartCP+=base
		ranges[i].EndCP+=base
	}
	return ranges
}

// AnchorMapByCP creates the canonical CP ownership map used by binary DOC
// compatibility paths.
func AnchorMapByCP(anchors []FloatingAnchor) map[int]FloatingAnchor {
	if len(anchors)==0 { return nil }
	out:=make(map[int]FloatingAnchor,len(anchors))
	for _,a:=range anchors { out[a.CP]=a }
	return out
}

// AnchorMapByShapeID creates FSP.spid ownership lookup for textbox attachment.
func AnchorMapByShapeID(anchors []FloatingAnchor) map[uint32]FloatingAnchor {
	if len(anchors)==0 { return nil }
	out:=make(map[uint32]FloatingAnchor,len(anchors))
	for _,a:=range anchors { if a.Shape!=0 { out[a.Shape]=a } }
	if len(out)==0 { return nil }
	return out
}
