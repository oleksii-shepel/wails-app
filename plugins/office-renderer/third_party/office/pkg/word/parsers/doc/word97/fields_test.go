package word97

import "testing"

func TestFieldCursorSuppressesInstructionAndAnnotatesResult(t *testing.T) {
	inner:=&sliceCursor{runs:[]Run{{Text:"\x13 HYPERLINK \"https://example.com\" \x14Example\x15",StartCP:5}}}
	parse:=func(instr string) Field { return Field{Kind:"HYPERLINK",Instr:instr,Target:"https://example.com"} }
	got:=collectCursor(t,NewFieldCursor(inner,parse))
	if len(got)!=1 { t.Fatalf("runs=%#v",got) }
	if got[0].Text!="Example" || got[0].FieldKind!="HYPERLINK" || got[0].Target!="https://example.com" { t.Fatalf("field result=%#v",got[0]) }
}

func TestFieldCursorPreservesNestedFieldResult(t *testing.T) {
	inner:=&sliceCursor{runs:[]Run{{Text:"\x13 IF 1 = 1 \x14A \x13 PAGE \x142\x15 B\x15",StartCP:0}}}
	parse:=func(instr string) Field { if instr=="PAGE" { return Field{Kind:"PAGE",Instr:instr} }; return Field{Kind:"IF",Instr:instr} }
	got:=collectCursor(t,NewFieldCursor(inner,parse))
	var text string
	for _,r:=range got { text+=r.Text }
	if text!="A 2 B" { t.Fatalf("visible text=%q runs=%#v",text,got) }
}
