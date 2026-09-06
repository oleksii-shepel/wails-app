package word97

import (
	"io"

	docbinary "office/pkg/word/parsers/doc/binary"
)

// DrawingMetadata is the bounded structural discovery needed before OfficeArt
// payload projection. It deliberately excludes rendering/materialization.
type DrawingMetadata struct {
	MainAnchors   []docbinary.FloatingAnchor
	HeaderAnchors []docbinary.FloatingAnchor
	Textboxes     []docbinary.TextboxRange
	DggInfo       []byte
}

// LoadDrawingMetadataReaderAt discovers all Word97 drawing PLCs through FIB
// ranges without materializing the whole table stream.
func LoadDrawingMetadataReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, counts docbinary.StoryCounts) DrawingMetadata {
	var out DrawingMetadata
	if blob,ok:=readTableRange(table,tableSize,fib,layout,docbinary.FibPlcSpaMom,34);ok{
		if anchors,valid:=docbinary.ParsePlcfSpa(blob,counts.Text);valid{out.MainAnchors=anchors}
	}
	if len(out.MainAnchors)==0 {
		out.MainAnchors=recoverFloatingAnchors(table,tableSize,fib,layout,counts.Text,docbinary.FibPlcSpaMom)
	}
	if blob,ok:=readTableRange(table,tableSize,fib,layout,docbinary.FibPlcSpaHdr,34);ok{
		if anchors,valid:=docbinary.ParsePlcfSpa(blob,counts.Hdr);valid{
			base:=counts.Text+counts.Ftn
			for i:=range anchors{anchors[i].CP+=base;anchors[i].Header=true}
			out.HeaderAnchors=anchors
		}
	}
	for _,spec:=range []struct{index int;header bool}{{docbinary.FibPlcftxbxTxt,false},{docbinary.FibPlcfHdrtxbxTxt,true}}{
		if blob,ok:=readTableRange(table,tableSize,fib,layout,spec.index,30);ok{
			ranges:=docbinary.ParseTextboxPLC(blob,spec.header)
			base:=counts.Text+counts.Ftn+counts.Hdr+counts.Mcr+counts.Atn+counts.Edn
			if spec.header{base+=counts.Txbx}
			for i:=range ranges{ranges[i].StartCP+=base;ranges[i].EndCP+=base}
			out.Textboxes=append(out.Textboxes,ranges...)
		}
	}
	if blob,ok:=readTableRange(table,tableSize,fib,layout,docbinary.FibDggInfo,8);ok{out.DggInfo=blob}
	return out
}

func recoverFloatingAnchors(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, maxCP, skipIndex int) []docbinary.FloatingAnchor {
	bestScore:=-1
	var best []docbinary.FloatingAnchor
	for index:=0;index<layout.CbRgFcLcb;index++{
		if index==skipIndex{continue}
		blob,ok:=readTableRange(table,tableSize,fib,layout,index,34);if !ok{continue}
		anchors,valid:=docbinary.ParsePlcfSpa(blob,maxCP);if !valid||len(anchors)==0{continue}
		score:=0
		for _,a:=range anchors{
			if a.Shape!=0{score++}
			if a.Right>a.Left&&a.Bottom>a.Top{score+=2}
			if a.Left>=0&&a.Top>=0{score++}
		}
		if score>bestScore{bestScore=score;best=anchors}
	}
	return best
}
