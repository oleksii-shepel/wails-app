package docformat

import (
	"reflect"

	wordir "office/pkg/word/ir"
)

func (p *TableRowProps) PhysicalCellCount() int {
	if p == nil || len(p.CellBoundaries) < 2 { return 0 }
	return len(p.CellBoundaries) - 1
}
func (p *TableRowProps) HMergeState(index int) byte { if p==nil||p.HMerge==nil{return 0};return p.HMerge[index] }
func (p *TableRowProps) VMergeState(index int) byte { if p==nil||p.VMerge==nil{return 0};return p.VMerge[index] }
func (p *TableRowProps) SetHMerge(first,lim int){if p==nil||first<0||lim<=first{return};if p.HMerge==nil{p.HMerge=make(map[int]byte,lim-first)};for i:=first;i<lim;i++{if i==first{p.HMerge[i]=2}else{p.HMerge[i]=1}}}
func (p *TableRowProps) ClearHMerge(first,lim int){if p==nil||p.HMerge==nil||lim<=first{return};for i:=first;i<lim;i++{delete(p.HMerge,i)}}
func (p *TableRowProps) SetVMerge(index int,state byte){if p==nil||index<0{return};if state==0{if p.VMerge!=nil{delete(p.VMerge,index)};return};if p.VMerge==nil{p.VMerge=make(map[int]byte,1)};p.VMerge[index]=state}
func (p *TableRowProps) CellFormat(index int) wordir.TableCellFormat { if p==nil||p.CellFormats==nil{return wordir.TableCellFormat{}};return CloneTableCellFormat(p.CellFormats[index]) }
func (p *TableRowProps) SetCellFormat(index int,format wordir.TableCellFormat){if p==nil||index<0||!TableCellFormatMeaningful(format){return};if p.CellFormats==nil{p.CellFormats=make(map[int]wordir.TableCellFormat,1)};p.CellFormats[index]=MergeTableCellFormat(p.CellFormats[index],format)}
func (p *TableRowProps) ApplyCellFormatRange(first,lim int,format wordir.TableCellFormat,oddOnly bool){if p==nil||lim<=first||!TableCellFormatMeaningful(format){return};step:=1;if oddOnly{step=2};for i:=first;i<lim;i+=step{p.SetCellFormat(i,format)}}

func MergeTableRowProps(base,overlay *TableRowProps)*TableRowProps{
	if base==nil{return overlay.Clone()};if overlay==nil{return base.Clone()};out:=base.Clone();if out==nil{out=&TableRowProps{}}
	for i,v:=range overlay.HMerge{if out.HMerge==nil{out.HMerge=map[int]byte{}};out.HMerge[i]=v};for i,v:=range overlay.VMerge{if out.VMerge==nil{out.VMerge=map[int]byte{}};out.VMerge[i]=v};for i,v:=range overlay.CellFormats{out.SetCellFormat(i,v)}
	if len(out.CellBoundaries)==0&&len(overlay.CellBoundaries)>0{out.CellBoundaries=append([]int(nil),overlay.CellBoundaries...)};if out.TableJc==""{out.TableJc=overlay.TableJc}
	copyIntPtrIfNil(&out.TableDxaLeft,overlay.TableDxaLeft);copyIntPtrIfNil(&out.TableDxaGap,overlay.TableDxaGap);copyIntPtrIfNil(&out.RowHeight,overlay.RowHeight);copyBoolPtrIfNil(&out.CantSplit,overlay.CantSplit);copyBoolPtrIfNil(&out.TableHeader,overlay.TableHeader)
	if out.TableBorders==nil&&overlay.TableBorders!=nil{out.TableBorders=CloneTableBorders(overlay.TableBorders)}
	copyIntPtrIfNil(&out.CellPadTop,overlay.CellPadTop);copyIntPtrIfNil(&out.CellPadBottom,overlay.CellPadBottom);copyIntPtrIfNil(&out.CellPadLeft,overlay.CellPadLeft);copyIntPtrIfNil(&out.CellPadRight,overlay.CellPadRight);copyIntPtrIfNil(&out.TableWidth,overlay.TableWidth)
	if out.TableWidthType==""{out.TableWidthType=overlay.TableWidthType};copyBoolPtrIfNil(&out.AutoFit,overlay.AutoFit);copyBoolPtrIfNil(&out.BidiVisual,overlay.BidiVisual);if out.FloatProps==nil&&overlay.FloatProps!=nil{v:=*overlay.FloatProps;out.FloatProps=&v};copyBoolPtrIfNil(&out.NoOverlap,overlay.NoOverlap)
	if out.PropertyRevision==nil&&overlay.PropertyRevision!=nil{v:=*overlay.PropertyRevision;out.PropertyRevision=&v};if out.ResolvedPropertyRevision==nil&&overlay.ResolvedPropertyRevision!=nil{v:=*overlay.ResolvedPropertyRevision;out.ResolvedPropertyRevision=&v};return out
}

func TableRowPropsConflict(current,next *TableRowProps)bool{
	if current==nil||next==nil{return false};if current.TableJc!=""&&next.TableJc!=""&&current.TableJc!=next.TableJc{return true};if ptrIntConflict(current.TableDxaLeft,next.TableDxaLeft)||ptrIntConflict(current.TableDxaGap,next.TableDxaGap)||ptrIntConflictStrict(current.RowHeight,next.RowHeight){return true};if ptrBoolConflictStrict(current.CantSplit,next.CantSplit)||ptrBoolConflictStrict(current.TableHeader,next.TableHeader){return true};if current.TableBorders!=nil&&next.TableBorders!=nil&&!reflect.DeepEqual(current.TableBorders,next.TableBorders){return true};if ptrIntConflict(current.CellPadTop,next.CellPadTop)||ptrIntConflict(current.CellPadBottom,next.CellPadBottom)||ptrIntConflict(current.CellPadLeft,next.CellPadLeft)||ptrIntConflict(current.CellPadRight,next.CellPadRight){return true};if ptrIntConflict(current.TableWidth,next.TableWidth)||(current.TableWidthType!=""&&next.TableWidthType!=""&&current.TableWidthType!=next.TableWidthType){return true};if ptrBoolConflict(current.AutoFit,next.AutoFit)||ptrBoolConflict(current.BidiVisual,next.BidiVisual)||ptrBoolConflict(current.NoOverlap,next.NoOverlap){return true};if current.FloatProps!=nil&&next.FloatProps!=nil&&!reflect.DeepEqual(current.FloatProps,next.FloatProps){return true};for i,v:=range next.HMerge{if cur,ok:=current.HMerge[i];ok&&cur!=v{return true}};for i,v:=range next.VMerge{if cur,ok:=current.VMerge[i];ok&&cur!=v{return true}};for i,v:=range next.CellFormats{if cur,ok:=current.CellFormats[i];ok&&!reflect.DeepEqual(cur,v){return true}};return false
}

func CloneTableCellFormat(format wordir.TableCellFormat)wordir.TableCellFormat{out:=format;if format.Width!=nil{v:=*format.Width;out.Width=&v};if format.Borders!=nil{out.Borders=CloneCellBorders(format.Borders)};return out}
func CloneBorderSide(side *wordir.BorderSide)*wordir.BorderSide{if side==nil{return nil};v:=*side;return &v}
func CloneCellBorders(b *wordir.CellBorders)*wordir.CellBorders{if b==nil{return nil};return &wordir.CellBorders{Top:CloneBorderSide(b.Top),Left:CloneBorderSide(b.Left),Bottom:CloneBorderSide(b.Bottom),Right:CloneBorderSide(b.Right)}}
func CloneTableBorders(b *wordir.TableBorders)*wordir.TableBorders{if b==nil{return nil};return &wordir.TableBorders{Top:CloneBorderSide(b.Top),Left:CloneBorderSide(b.Left),Bottom:CloneBorderSide(b.Bottom),Right:CloneBorderSide(b.Right),InsideH:CloneBorderSide(b.InsideH),InsideV:CloneBorderSide(b.InsideV)}}
func MergeTableCellFormat(base,overlay wordir.TableCellFormat)wordir.TableCellFormat{out:=CloneTableCellFormat(base);if overlay.Width!=nil{v:=*overlay.Width;out.Width=&v};if overlay.WidthType!=""{out.WidthType=overlay.WidthType};if overlay.GridSpan>1{out.GridSpan=overlay.GridSpan};if overlay.HMerge{out.HMerge=true};if overlay.VMerge!=""{out.VMerge=overlay.VMerge};if overlay.VertAlign!=""{out.VertAlign=overlay.VertAlign};if overlay.TextDirection!=""{out.TextDirection=overlay.TextDirection};if overlay.Shading!=""{out.Shading=overlay.Shading};if overlay.Borders!=nil{out.Borders=mergeCellBorders(out.Borders,overlay.Borders)};return out}
func TableCellFormatMeaningful(f wordir.TableCellFormat)bool{return f.Width!=nil||f.WidthType!=""||f.GridSpan>1||f.HMerge||f.VMerge!=""||f.VertAlign!=""||f.TextDirection!=""||f.Shading!=""||f.Borders!=nil}
func mergeCellBorders(base,overlay *wordir.CellBorders)*wordir.CellBorders{if overlay==nil{return CloneCellBorders(base)};out:=CloneCellBorders(base);if out==nil{out=&wordir.CellBorders{}};if overlay.Top!=nil{out.Top=CloneBorderSide(overlay.Top)};if overlay.Left!=nil{out.Left=CloneBorderSide(overlay.Left)};if overlay.Bottom!=nil{out.Bottom=CloneBorderSide(overlay.Bottom)};if overlay.Right!=nil{out.Right=CloneBorderSide(overlay.Right)};return out}
func copyIntPtrIfNil(dst **int,src *int){if *dst==nil&&src!=nil{v:=*src;*dst=&v}};func copyBoolPtrIfNil(dst **bool,src *bool){if *dst==nil&&src!=nil{v:=*src;*dst=&v}}
func ptrIntConflict(a,b *int)bool{return a!=nil&&b!=nil&&*a!=*b};func ptrIntConflictStrict(a,b *int)bool{if (a==nil)!=(b==nil){return true};return ptrIntConflict(a,b)};func ptrBoolConflict(a,b *bool)bool{return a!=nil&&b!=nil&&*a!=*b};func ptrBoolConflictStrict(a,b *bool)bool{if (a==nil)!=(b==nil){return true};return ptrBoolConflict(a,b)}
