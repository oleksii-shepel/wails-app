package legacy

import "testing"

func TestLegacyFieldRunCursorAnnotatesCachedHyperlinkResult(t *testing.T) {
	inner := newLegacyRunSliceCursor([]docRun{
		{Text: "before "},
		{Text: "\x13 HYPERLINK \"https://example.com\" \x14Example\x15 after"},
	})
	cursor := newLegacyFieldRunCursor(inner)

	first, ok, err := cursor.Next()
	if err != nil || !ok || first.Text != "before " {
		t.Fatalf("first=%#v ok=%v err=%v", first, ok, err)
	}
	field, ok, err := cursor.Next()
	if err != nil || !ok {
		t.Fatalf("field ok=%v err=%v", ok, err)
	}
	if field.Text != "Example" || field.FieldKind != "HYPERLINK" || field.Target != "https://example.com" {
		t.Fatalf("field=%#v", field)
	}
	after, ok, err := cursor.Next()
	if err != nil || !ok || after.Text != " after" {
		t.Fatalf("after=%#v ok=%v err=%v", after, ok, err)
	}
}

func TestLegacyFieldRunCursorSuppressesInstructionText(t *testing.T) {
	cursor := newLegacyFieldRunCursor(newLegacyRunSliceCursor([]docRun{
		{Text: "\x13 PAGE \x1412\x15"},
	}))
	run, ok, err := cursor.Next()
	if err != nil || !ok {
		t.Fatalf("run ok=%v err=%v", ok, err)
	}
	if run.Text != "12" || run.FieldKind != "PAGE" || run.FieldInstr != "PAGE" {
		t.Fatalf("run=%#v", run)
	}
	if _, ok, err = cursor.Next(); err != nil || ok {
		t.Fatalf("EOF ok=%v err=%v", ok, err)
	}
}
