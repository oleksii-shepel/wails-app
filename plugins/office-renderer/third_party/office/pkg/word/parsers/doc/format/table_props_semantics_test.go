package docformat

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestTableRowPropsOwnsMergeAndCellSemantics(t *testing.T) {
	p:=&TableRowProps{CellBoundaries:[]int{0,1000,2200}}
	p.SetHMerge(0,2)
	p.SetVMerge(1,2)
	width:=1200
	p.SetCellFormat(1,wordir.TableCellFormat{Width:&width,WidthType:"dxa",GridSpan:2})
	if p.PhysicalCellCount()!=2 || p.HMergeState(0)!=2 || p.HMergeState(1)!=1 || p.VMergeState(1)!=2 { t.Fatalf("row semantics=%+v",p) }
	cell:=p.CellFormat(1)
	if cell.Width==nil || *cell.Width!=1200 || cell.WidthType!="dxa" || cell.GridSpan!=2 { t.Fatalf("cell=%+v",cell) }
	p.ClearHMerge(0,2)
	if p.HMergeState(0)!=0 || p.HMergeState(1)!=0 { t.Fatalf("hmerge not cleared: %+v",p.HMerge) }
}

func TestMergeTableRowPropsPreservesBaseAndAddsOverlay(t *testing.T) {
	left:=100
	height:=240
	base:=&TableRowProps{TableDxaLeft:&left,CellBoundaries:[]int{0,1000}}
	overlay:=&TableRowProps{RowHeight:&height,HMerge:map[int]byte{0:2}}
	got:=MergeTableRowProps(base,overlay)
	if got==base || got==overlay { t.Fatal("merge did not clone") }
	if got.TableDxaLeft==nil || *got.TableDxaLeft!=100 || got.RowHeight==nil || *got.RowHeight!=240 || got.HMergeState(0)!=2 { t.Fatalf("merged=%+v",got) }
}

func TestTableRowPropsCloneOwnsScalarPointers(t *testing.T) {
	left:=100
	header:=true
	p:=&TableRowProps{TableDxaLeft:&left,TableHeader:&header}
	clone:=p.Clone()
	if clone==nil || clone.TableDxaLeft==p.TableDxaLeft || clone.TableHeader==p.TableHeader { t.Fatalf("clone shares scalar pointers: original=%+v clone=%+v",p,clone) }
	*clone.TableDxaLeft=900
	*clone.TableHeader=false
	if *p.TableDxaLeft!=100 || !*p.TableHeader { t.Fatalf("clone mutation leaked into source: %+v",p) }
}

func TestTableRowPropsConflictTreatsRowScopedPresenceAsConflict(t *testing.T) {
	header:=true
	if !TableRowPropsConflict(&TableRowProps{},&TableRowProps{TableHeader:&header}) { t.Fatal("nil vs set row header must conflict") }
	if TableRowPropsConflict(&TableRowProps{HMerge:map[int]byte{0:2}},&TableRowProps{HMerge:map[int]byte{0:2}}) { t.Fatal("equal merge state conflicted") }
	if !TableRowPropsConflict(&TableRowProps{HMerge:map[int]byte{0:2}},&TableRowProps{HMerge:map[int]byte{0:1}}) { t.Fatal("different merge states did not conflict") }
}

func TestMergeTableCellFormatOwnsWidthMergeAndBorders(t *testing.T) {
	w:=1000
	base:=wordir.TableCellFormat{Width:&w,WidthType:"dxa",Borders:&wordir.CellBorders{Left:&wordir.BorderSide{Val:"single"}}}
	overlay:=wordir.TableCellFormat{GridSpan:3,Borders:&wordir.CellBorders{Right:&wordir.BorderSide{Val:"double"}}}
	got:=MergeTableCellFormat(base,overlay)
	if got.Width==nil || *got.Width!=1000 || got.GridSpan!=3 || got.Borders==nil || got.Borders.Left==nil || got.Borders.Left.Val!="single" || got.Borders.Right==nil || got.Borders.Right.Val!="double" { t.Fatalf("merged cell=%+v",got) }
}
