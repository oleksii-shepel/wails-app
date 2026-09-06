package docformat

import wordir "office/pkg/word/ir"

// TableRowProps is the sequential DOC parser's canonical TAPX row/table model.
// It intentionally preserves row-local structure (grid, merges, cell formats)
// together with table-level properties physically repeated in TAPX records.
type TableRowProps struct {
	CellBoundaries []int
	HMerge         map[int]byte
	VMerge         map[int]byte
	CellFormats    map[int]wordir.TableCellFormat

	TableJc        string
	TableDxaLeft   *int
	TableDxaGap    *int
	RowHeight      *int
	CantSplit      *bool
	TableHeader    *bool
	TableBorders   *wordir.TableBorders
	CellPadTop     *int
	CellPadBottom  *int
	CellPadLeft    *int
	CellPadRight   *int
	TableWidth     *int
	TableWidthType string
	AutoFit        *bool
	BidiVisual     *bool
	FloatProps     *wordir.TableFloatProps
	NoOverlap      *bool

	PropertyRevision         *PendingRevisionMark
	ResolvedPropertyRevision *wordir.RevisionMark
}

func cloneIntPtr(v *int) *int { if v==nil{return nil}; c:=*v; return &c }
func cloneBoolPtr(v *bool) *bool { if v==nil{return nil}; c:=*v; return &c }

func (p *TableRowProps) Clone() *TableRowProps {
	if p == nil { return nil }
	out := *p
	out.CellBoundaries = append([]int(nil), p.CellBoundaries...)
	if p.HMerge != nil { out.HMerge = make(map[int]byte, len(p.HMerge)); for k,v := range p.HMerge { out.HMerge[k]=v } }
	if p.VMerge != nil { out.VMerge = make(map[int]byte, len(p.VMerge)); for k,v := range p.VMerge { out.VMerge[k]=v } }
	if p.CellFormats != nil { out.CellFormats = make(map[int]wordir.TableCellFormat, len(p.CellFormats)); for k,v := range p.CellFormats { out.CellFormats[k]=CloneTableCellFormat(v) } }
	out.TableDxaLeft=cloneIntPtr(p.TableDxaLeft)
	out.TableDxaGap=cloneIntPtr(p.TableDxaGap)
	out.RowHeight=cloneIntPtr(p.RowHeight)
	out.CantSplit=cloneBoolPtr(p.CantSplit)
	out.TableHeader=cloneBoolPtr(p.TableHeader)
	out.CellPadTop=cloneIntPtr(p.CellPadTop)
	out.CellPadBottom=cloneIntPtr(p.CellPadBottom)
	out.CellPadLeft=cloneIntPtr(p.CellPadLeft)
	out.CellPadRight=cloneIntPtr(p.CellPadRight)
	out.TableWidth=cloneIntPtr(p.TableWidth)
	out.AutoFit=cloneBoolPtr(p.AutoFit)
	out.BidiVisual=cloneBoolPtr(p.BidiVisual)
	out.NoOverlap=cloneBoolPtr(p.NoOverlap)
	if p.TableBorders != nil { out.TableBorders=CloneTableBorders(p.TableBorders) }
	if p.FloatProps != nil { v:=*p.FloatProps; out.FloatProps=&v }
	if p.PropertyRevision != nil { v:=*p.PropertyRevision; out.PropertyRevision=&v }
	if p.ResolvedPropertyRevision != nil { v:=*p.ResolvedPropertyRevision; out.ResolvedPropertyRevision=&v }
	return &out
}

func (p *TableRowProps) HasData() bool {
	if p == nil { return false }
	return len(p.CellBoundaries)>=2 || len(p.HMerge)>0 || len(p.VMerge)>0 || len(p.CellFormats)>0 ||
		p.TableJc!="" || p.TableDxaLeft!=nil || p.TableDxaGap!=nil || p.RowHeight!=nil || p.CantSplit!=nil || p.TableHeader!=nil || p.TableBorders!=nil ||
		p.CellPadTop!=nil || p.CellPadBottom!=nil || p.CellPadLeft!=nil || p.CellPadRight!=nil || p.TableWidth!=nil || p.TableWidthType!="" ||
		p.AutoFit!=nil || p.BidiVisual!=nil || p.FloatProps!=nil || p.NoOverlap!=nil || p.PropertyRevision!=nil || p.ResolvedPropertyRevision!=nil
}

func (p *TableRowProps) MaxCellIndex() int {
	if p == nil { return 0 }
	max := 0
	if len(p.CellBoundaries)>=2 { max=len(p.CellBoundaries)-1 }
	for i := range p.HMerge { if i+1>max { max=i+1 } }
	for i := range p.VMerge { if i+1>max { max=i+1 } }
	for i := range p.CellFormats { if i+1>max { max=i+1 } }
	return max
}

func (p *TableRowProps) ValidCellBoundaries() []int {
	if p == nil || len(p.CellBoundaries)<2 { return nil }
	for i:=1;i<len(p.CellBoundaries);i++ { if p.CellBoundaries[i]<=p.CellBoundaries[i-1] { return nil } }
	return p.CellBoundaries
}
