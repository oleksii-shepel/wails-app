package legacy

import (
	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

// legacyTableRowProps remains only as the historical storage shape required by
// the lossless/pre-97 compatibility surface. All TAP semantics and cloning live
// in docformat.TableRowProps.
type legacyTableRowProps struct {
	cellBoundaries []int
	hMerge map[int]byte
	vMerge map[int]byte
	cellFormats map[int]wordir.TableCellFormat
	tableJc string
	tableDxaLeft *int
	tableDxaGap *int
	rowHeight *int
	cantSplit *bool
	tableHeader *bool
	tableBorders *wordir.TableBorders
	cellPadTop *int
	cellPadBottom *int
	cellPadLeft *int
	cellPadRight *int
	tableWidth *int
	tableWidthType string
	autoFit *bool
	bidiVisual *bool
	floatProps *wordir.TableFloatProps
	noOverlap *bool
	propertyRevision *legacyPendingRevisionMark
	resolvedPropertyRevision *wordir.RevisionMark
}

func (p *legacyTableRowProps) canonical() *docformat.TableRowProps {
	if p==nil{return nil}
	out:=&docformat.TableRowProps{CellBoundaries:append([]int(nil),p.cellBoundaries...),TableJc:p.tableJc,TableDxaLeft:p.tableDxaLeft,TableDxaGap:p.tableDxaGap,RowHeight:p.rowHeight,CantSplit:p.cantSplit,TableHeader:p.tableHeader,TableBorders:p.tableBorders,CellPadTop:p.cellPadTop,CellPadBottom:p.cellPadBottom,CellPadLeft:p.cellPadLeft,CellPadRight:p.cellPadRight,TableWidth:p.tableWidth,TableWidthType:p.tableWidthType,AutoFit:p.autoFit,BidiVisual:p.bidiVisual,FloatProps:p.floatProps,NoOverlap:p.noOverlap,PropertyRevision:toSequentialRevision(p.propertyRevision),ResolvedPropertyRevision:p.resolvedPropertyRevision}
	if p.hMerge!=nil{out.HMerge=make(map[int]byte,len(p.hMerge));for k,v:=range p.hMerge{out.HMerge[k]=v}}
	if p.vMerge!=nil{out.VMerge=make(map[int]byte,len(p.vMerge));for k,v:=range p.vMerge{out.VMerge[k]=v}}
	if p.cellFormats!=nil{out.CellFormats=make(map[int]wordir.TableCellFormat,len(p.cellFormats));for k,v:=range p.cellFormats{out.CellFormats[k]=docformat.CloneTableCellFormat(v)}}
	return out.Clone()
}

func legacyTableRowPropsFromCanonical(p *docformat.TableRowProps)*legacyTableRowProps{
	if p==nil{return nil};p=p.Clone();out:=&legacyTableRowProps{cellBoundaries:append([]int(nil),p.CellBoundaries...),tableJc:p.TableJc,tableDxaLeft:p.TableDxaLeft,tableDxaGap:p.TableDxaGap,rowHeight:p.RowHeight,cantSplit:p.CantSplit,tableHeader:p.TableHeader,tableBorders:p.TableBorders,cellPadTop:p.CellPadTop,cellPadBottom:p.CellPadBottom,cellPadLeft:p.CellPadLeft,cellPadRight:p.CellPadRight,tableWidth:p.TableWidth,tableWidthType:p.TableWidthType,autoFit:p.AutoFit,bidiVisual:p.BidiVisual,floatProps:p.FloatProps,noOverlap:p.NoOverlap,propertyRevision:fromSequentialRevision(p.PropertyRevision),resolvedPropertyRevision:p.ResolvedPropertyRevision}
	if p.HMerge!=nil{out.hMerge=make(map[int]byte,len(p.HMerge));for k,v:=range p.HMerge{out.hMerge[k]=v}}
	if p.VMerge!=nil{out.vMerge=make(map[int]byte,len(p.VMerge));for k,v:=range p.VMerge{out.vMerge[k]=v}}
	if p.CellFormats!=nil{out.cellFormats=make(map[int]wordir.TableCellFormat,len(p.CellFormats));for k,v:=range p.CellFormats{out.cellFormats[k]=docformat.CloneTableCellFormat(v)}}
	return out
}

func (p *legacyTableRowProps) replaceCanonical(c *docformat.TableRowProps){if p==nil{return};next:=legacyTableRowPropsFromCanonical(c);if next==nil{*p=legacyTableRowProps{};return};*p=*next}
func (p *legacyTableRowProps) clone()*legacyTableRowProps{return legacyTableRowPropsFromCanonical(p.canonical())}
func (p *legacyTableRowProps) hasData()bool{return p.canonical().HasData()}
func (p *legacyTableRowProps) maxCellIndex()int{return p.canonical().MaxCellIndex()}
func (p *legacyTableRowProps) physicalCellCount()int{return p.canonical().PhysicalCellCount()}
func (p *legacyTableRowProps) validCellBoundaries()[]int{if p==nil{return nil};return p.canonical().ValidCellBoundaries()}
func (p *legacyTableRowProps) hMergeState(index int)byte{return p.canonical().HMergeState(index)}
func (p *legacyTableRowProps) vMergeState(index int)byte{return p.canonical().VMergeState(index)}
func (p *legacyTableRowProps) setHMerge(first,lim int){if p==nil{return};c:=p.canonical();c.SetHMerge(first,lim);p.replaceCanonical(c)}
func (p *legacyTableRowProps) clearHMerge(first,lim int){if p==nil{return};c:=p.canonical();c.ClearHMerge(first,lim);p.replaceCanonical(c)}
func (p *legacyTableRowProps) setVMerge(index int,state byte){if p==nil{return};c:=p.canonical();c.SetVMerge(index,state);p.replaceCanonical(c)}
func (p *legacyTableRowProps) cellFormat(index int)wordir.TableCellFormat{return p.canonical().CellFormat(index)}
func (p *legacyTableRowProps) setCellFormat(index int,format wordir.TableCellFormat){if p==nil{return};c:=p.canonical();c.SetCellFormat(index,format);p.replaceCanonical(c)}
func (p *legacyTableRowProps) applyCellFormatRange(first,lim int,format wordir.TableCellFormat,oddOnly bool){if p==nil{return};c:=p.canonical();c.ApplyCellFormatRange(first,lim,format,oddOnly);p.replaceCanonical(c)}
func mergeLegacyTableRowProps(base,overlay *legacyTableRowProps)*legacyTableRowProps{return legacyTableRowPropsFromCanonical(docformat.MergeTableRowProps(base.canonical(),overlay.canonical()))}
func legacyTableRowPropsConflict(current,next *legacyTableRowProps)bool{return docformat.TableRowPropsConflict(current.canonical(),next.canonical())}

func cloneLegacyTableCellFormat(format wordir.TableCellFormat)wordir.TableCellFormat{return docformat.CloneTableCellFormat(format)}
func cloneLegacyCellBorders(b *wordir.CellBorders)*wordir.CellBorders{return docformat.CloneCellBorders(b)}
func cloneLegacyTableBorders(b *wordir.TableBorders)*wordir.TableBorders{return docformat.CloneTableBorders(b)}
func cloneLegacyBorderSide(side *wordir.BorderSide)*wordir.BorderSide{return docformat.CloneBorderSide(side)}
func mergeLegacyTableCellFormat(base,overlay wordir.TableCellFormat)wordir.TableCellFormat{return docformat.MergeTableCellFormat(base,overlay)}
func mergeLegacyCellBorders(base,overlay *wordir.CellBorders)*wordir.CellBorders{return docformat.MergeTableCellFormat(wordir.TableCellFormat{Borders:base},wordir.TableCellFormat{Borders:overlay}).Borders}
func legacyTableCellFormatMeaningful(format wordir.TableCellFormat)bool{return docformat.TableCellFormatMeaningful(format)}
