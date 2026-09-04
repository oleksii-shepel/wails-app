package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDocumentContentControlsEmpty(t *testing.T) {
	d := NewDocument()
	d.AddParagraph().AddRun().SetText("hello")
	ccs := d.ContentControls()
	if len(ccs) != 0 {
		t.Fatalf("expected 0 content controls, got %d", len(ccs))
	}
}

func TestDocumentContentControlsBody(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("hello")
	r.SetContentControl(&wordir.ContentControl{ID: 1, Tag: "tag1", Type: "plainText"})

	ccs := d.ContentControls()
	if len(ccs) != 1 {
		t.Fatalf("expected 1 content control, got %d", len(ccs))
	}
	cc := ccs[0]
	if cc.Location != "body" {
		t.Fatalf("location = %q, want body", cc.Location)
	}
	if cc.Control.ID() != 1 {
		t.Fatalf("id = %d, want 1", cc.Control.ID())
	}
	if cc.Control.Tag() != "tag1" {
		t.Fatalf("tag = %q, want tag1", cc.Control.Tag())
	}
	if cc.Control.Type() != "plainText" {
		t.Fatalf("type = %q, want plainText", cc.Control.Type())
	}
}

func TestDocumentContentControlsNestedStructures(t *testing.T) {
	d := NewDocument()

	hostPara := d.AddParagraph()
	hostRun := hostPara.AddRun()
	hostRun.ir().TextBox = &wordir.TextBox{
		Blocks: []wordir.Block{{
			Kind: "paragraph",
			Paragraph: &wordir.Paragraph{
				Runs: []wordir.Run{{
					Kind:           "run",
					ContentControl: &wordir.ContentControl{ID: 10, Tag: "textbox-run", Type: "plainText"},
				}},
			},
		}},
	}
	hostRun.ir().Shape = &wordir.Shape{
		TextBox: &wordir.TextBox{
			Blocks: []wordir.Block{{
				Kind: "paragraph",
				Paragraph: &wordir.Paragraph{
					Runs: []wordir.Run{{
						Kind:           "run",
						ContentControl: &wordir.ContentControl{ID: 11, Tag: "shape-textbox-run", Type: "richText"},
					}},
				},
			}},
		},
	}

	tbl := d.AddTable()
	cell := tbl.AddRow().AddCell()
	cell.SetContentControl(&wordir.ContentControl{ID: 12, Tag: "cell", Type: "group"})
	cellPara := cell.AddParagraph()
	cellRun := cellPara.AddRun()
	cellRun.SetText("inside cell")
	cellRun.SetContentControl(&wordir.ContentControl{ID: 13, Tag: "cell-run", Type: "plainText"})

	ccs := d.ContentControls()
	if len(ccs) != 4 {
		t.Fatalf("expected 4 content controls, got %d", len(ccs))
	}
	wantTags := []string{"textbox-run", "shape-textbox-run", "cell", "cell-run"}
	for i, want := range wantTags {
		if ccs[i].Control == nil || ccs[i].Control.Tag() != want {
			t.Fatalf("content control %d tag = %v, want %q", i, ccs[i].Control, want)
		}
		if ccs[i].Location != "body" {
			t.Fatalf("content control %d location = %q, want body", i, ccs[i].Location)
		}
	}
}

func TestDocumentContentControlsByTag(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("a")
	r.SetContentControl(&wordir.ContentControl{ID: 1, Tag: "alpha", Type: "plainText"})

	r2 := p.AddRun()
	r2.SetText("b")
	r2.SetContentControl(&wordir.ContentControl{ID: 2, Tag: "beta", Type: "richText"})

	alpha := d.ContentControlsByTag("alpha")
	if len(alpha) != 1 {
		t.Fatalf("alpha count = %d, want 1", len(alpha))
	}
	beta := d.ContentControlsByTag("beta")
	if len(beta) != 1 {
		t.Fatalf("beta count = %d, want 1", len(beta))
	}
	gamma := d.ContentControlsByTag("gamma")
	if len(gamma) != 0 {
		t.Fatalf("gamma count = %d, want 0", len(gamma))
	}
}

func TestDocumentContentControlByID(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("a")
	r.SetContentControl(&wordir.ContentControl{ID: 42, Tag: "answer"})

	cc := d.ContentControlByID(42)
	if cc == nil {
		t.Fatal("expected to find content control by ID")
	}
	if cc.Tag() != "answer" {
		t.Fatalf("tag = %q, want answer", cc.Tag())
	}
	if d.ContentControlByID(99) != nil {
		t.Fatal("expected nil for unknown ID")
	}
}

func TestContentControlSetters(t *testing.T) {
	cc := &ContentControl{ir: &wordir.ContentControl{
		ID:                        1,
		Tag:                       "old",
		Title:                     "Old",
		DateFormat:                "yyyy-MM-dd",
		DateFullDate:              "2026-07-12T00:00:00Z",
		DataXPath:                 "/root/customer",
		DataStoreID:               "{store-1}",
		PlaceholderDocPart:        "DefaultPlaceholder",
		PlaceholderDocPartGallery: "Quick Parts",
		ListItems:                 []wordir.SdtListItem{{DisplayText: "A", Value: "1"}, {DisplayText: "B", Value: "2"}},
	}}
	cc.SetTag("new")
	if cc.Tag() != "new" {
		t.Fatalf("tag = %q, want new", cc.Tag())
	}
	cc.SetTitle("New")
	if cc.Title() != "New" {
		t.Fatalf("title = %q, want New", cc.Title())
	}
	cc.SetChecked(true)
	if !cc.Checked() {
		t.Fatal("expected checked")
	}
	cc.SetShowingPlaceholder(true)
	if !cc.IsShowingPlaceholder() {
		t.Fatal("expected showing placeholder")
	}
	cc.SetDataValue("val")
	if cc.DataValue() != "val" {
		t.Fatalf("dataValue = %q, want val", cc.DataValue())
	}
	if cc.DateFormat() != "yyyy-MM-dd" {
		t.Fatalf("dateFormat = %q", cc.DateFormat())
	}
	if cc.DateFullDate() != "2026-07-12T00:00:00Z" {
		t.Fatalf("dateFullDate = %q", cc.DateFullDate())
	}
	xpath, storeID := cc.DataBinding()
	if xpath != "/root/customer" || storeID != "{store-1}" {
		t.Fatalf("dataBinding = %q / %q", xpath, storeID)
	}
	docPart, gallery := cc.Placeholder()
	if docPart != "DefaultPlaceholder" || gallery != "Quick Parts" {
		t.Fatalf("placeholder = %q / %q", docPart, gallery)
	}
	items := cc.ListItems()
	if len(items) != 2 || items[0].DisplayText != "A" || items[1].Value != "2" {
		t.Fatalf("listItems = %#v", items)
	}
	items[0].DisplayText = "Mutated"
	if cc.ListItems()[0].DisplayText != "A" {
		t.Fatal("ListItems should return a copy")
	}
}

func TestContentControlNilSafety(t *testing.T) {
	var cc *ContentControl
	if cc.ID() != 0 {
		t.Fatal("nil ID should be 0")
	}
	if cc.Tag() != "" {
		t.Fatal("nil Tag should be empty")
	}
	if cc.Title() != "" {
		t.Fatal("nil Title should be empty")
	}
	if cc.Type() != "" {
		t.Fatal("nil Type should be empty")
	}
	if cc.Checked() {
		t.Fatal("nil Checked should be false")
	}
	if cc.IsShowingPlaceholder() {
		t.Fatal("nil IsShowingPlaceholder should be false")
	}
	if cc.DataValue() != "" {
		t.Fatal("nil DataValue should be empty")
	}
	if cc.DateFormat() != "" || cc.DateFullDate() != "" {
		t.Fatal("nil date metadata should be empty")
	}
	xpath, storeID := cc.DataBinding()
	if xpath != "" || storeID != "" {
		t.Fatal("nil data binding should be empty")
	}
	docPart, gallery := cc.Placeholder()
	if docPart != "" || gallery != "" {
		t.Fatal("nil placeholder should be empty")
	}
	if cc.ListItems() != nil {
		t.Fatal("nil list items should be nil")
	}
	// Setters on nil should not panic
	cc.SetTag("x")
	cc.SetTitle("x")
	cc.SetChecked(true)
	cc.SetShowingPlaceholder(true)
	cc.SetDataValue("x")
}

func TestDocumentContentControlsNil(t *testing.T) {
	var d *Document
	if d.ContentControls() != nil {
		t.Fatal("expected nil")
	}
	if d.ContentControlsByTag("x") != nil {
		t.Fatal("expected nil")
	}
	if d.ContentControlByID(1) != nil {
		t.Fatal("expected nil")
	}
}

func TestWrapParagraphInContentControl(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.AddRun().SetText("hello")

	cc := &wordir.ContentControl{ID: 10, Tag: "wrap-test", Type: "richText"}
	if !d.WrapParagraphInContentControl(p, cc) {
		t.Fatal("expected wrap to succeed")
	}

	// The block-level CC should now appear in enumeration.
	locs := d.ContentControls()
	if len(locs) != 1 {
		t.Fatalf("expected 1 content control, got %d", len(locs))
	}
	if locs[0].Control.Tag() != "wrap-test" {
		t.Fatalf("tag = %q, want wrap-test", locs[0].Control.Tag())
	}
	if locs[0].Location != "body" {
		t.Fatalf("location = %q, want body", locs[0].Location)
	}

	// Unwrap should remove it.
	if !d.UnwrapBlockContentControlFromParagraph(p) {
		t.Fatal("expected unwrap to succeed")
	}
	locs = d.ContentControls()
	if len(locs) != 0 {
		t.Fatalf("expected 0 content controls after unwrap, got %d", len(locs))
	}
}

func TestWrapParagraphInContentControlNestedTableCell(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	cell := tbl.AddRow().AddCell()
	p := cell.AddParagraph()
	p.AddRun().SetText("nested")

	cc := &wordir.ContentControl{ID: 30, Tag: "nested-para-wrap", Type: "richText"}
	if !d.WrapParagraphInContentControl(p, cc) {
		t.Fatal("expected nested paragraph wrap to succeed")
	}
	if got := cell.ir().Blocks[0].ContentControl; got != cc {
		t.Fatalf("block content control = %#v, want %#v", got, cc)
	}

	if !d.UnwrapBlockContentControlFromParagraph(p) {
		t.Fatal("expected nested paragraph unwrap to succeed")
	}
	if got := cell.ir().Blocks[0].ContentControl; got != nil {
		t.Fatalf("block content control = %#v, want nil", got)
	}
	if d.UnwrapBlockContentControlFromParagraph(p) {
		t.Fatal("expected false when nested paragraph has no wrapper")
	}
}

func TestParagraphContentControlWrapperMethods(t *testing.T) {
	d := NewDocument()
	header := d.AddHeader("default")
	p := header.AddParagraph()
	p.AddRun().SetText("header")

	if p.ContentControl() != nil {
		t.Fatal("expected no paragraph content control initially")
	}

	cc := &wordir.ContentControl{ID: 31, Tag: "paragraph-wrapper", Type: "richText"}
	if !p.SetContentControl(cc) {
		t.Fatal("expected paragraph wrapper SetContentControl to succeed")
	}
	if got := p.ContentControl(); got == nil || got.Tag() != "paragraph-wrapper" {
		t.Fatalf("paragraph content control = %#v", got)
	}
	if !p.ClearContentControl() {
		t.Fatal("expected paragraph wrapper ClearContentControl to succeed")
	}
	if p.ContentControl() != nil {
		t.Fatal("expected paragraph content control to be cleared")
	}
}

func TestWrapTableInContentControl(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	tbl.AddRow().AddCell().SetText("cell")

	cc := &wordir.ContentControl{ID: 20, Tag: "table-wrap", Type: "richText"}
	if !d.WrapTableInContentControl(tbl, cc) {
		t.Fatal("expected wrap to succeed")
	}

	locs := d.ContentControls()
	if len(locs) != 1 {
		t.Fatalf("expected 1 content control, got %d", len(locs))
	}
	if locs[0].Control.Tag() != "table-wrap" {
		t.Fatalf("tag = %q, want table-wrap", locs[0].Control.Tag())
	}

	// Unwrap should remove it.
	if !d.UnwrapBlockContentControlFromTable(tbl) {
		t.Fatal("expected unwrap to succeed")
	}
	locs = d.ContentControls()
	if len(locs) != 0 {
		t.Fatalf("expected 0 content controls after unwrap, got %d", len(locs))
	}
}

func TestWrapTableInContentControlNestedTableCell(t *testing.T) {
	d := NewDocument()
	outer := d.AddTable()
	cell := outer.AddRow().AddCell()
	cellIR := cell.ir()
	nestedIR := wordir.Table{}
	cellIR.Blocks = []wordir.Block{{
		Index: 0,
		Kind:  "table",
		Table: &nestedIR,
	}}
	cell.rebuildFlatSlices()
	nested := &Table{ir: cellIR.Blocks[0].Table, doc: d}
	nested.AddRow().AddCell().SetText("inner")

	cc := &wordir.ContentControl{ID: 40, Tag: "nested-table-wrap", Type: "group"}
	if !d.WrapTableInContentControl(nested, cc) {
		t.Fatal("expected nested table wrap to succeed")
	}
	if got := cell.ir().Blocks[0].ContentControl; got != cc {
		t.Fatalf("block content control = %#v, want %#v", got, cc)
	}

	if !d.UnwrapBlockContentControlFromTable(nested) {
		t.Fatal("expected nested table unwrap to succeed")
	}
	if got := cell.ir().Blocks[0].ContentControl; got != nil {
		t.Fatalf("block content control = %#v, want nil", got)
	}
	if d.UnwrapBlockContentControlFromTable(nested) {
		t.Fatal("expected false when nested table has no wrapper")
	}
}

func TestTableContentControlWrapperMethods(t *testing.T) {
	d := NewDocument()
	header := d.AddHeader("default")
	tbl := header.AddTable()
	tbl.AddRow().AddCell().SetText("header cell")

	if tbl.ContentControl() != nil {
		t.Fatal("expected no table content control initially")
	}

	cc := &wordir.ContentControl{ID: 41, Tag: "table-wrapper", Type: "group"}
	if !tbl.SetContentControl(cc) {
		t.Fatal("expected table wrapper SetContentControl to succeed")
	}
	if got := tbl.ContentControl(); got == nil || got.Tag() != "table-wrapper" {
		t.Fatalf("table content control = %#v", got)
	}
	if !tbl.ClearContentControl() {
		t.Fatal("expected table wrapper ClearContentControl to succeed")
	}
	if tbl.ContentControl() != nil {
		t.Fatal("expected table content control to be cleared")
	}
}

func TestWrapParagraphInContentControlNil(t *testing.T) {
	var d *Document
	var p *Paragraph
	if d.WrapParagraphInContentControl(p, nil) {
		t.Fatal("expected false for nil document")
	}
	if d.WrapTableInContentControl(nil, nil) {
		t.Fatal("expected false for nil document")
	}
	if d.UnwrapBlockContentControlFromParagraph(p) {
		t.Fatal("expected false for nil document")
	}
	if d.UnwrapBlockContentControlFromTable(nil) {
		t.Fatal("expected false for nil document")
	}
}

func TestWrapParagraphInContentControlNotFound(t *testing.T) {
	d := NewDocument()
	// Create an orphan paragraph not in the document body.
	orphan := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "orphan"}}}
	p := &Paragraph{ir: &orphan}
	cc := &wordir.ContentControl{ID: 99}
	if d.WrapParagraphInContentControl(p, cc) {
		t.Fatal("expected false for orphan paragraph")
	}
}