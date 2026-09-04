package edit

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDocumentMetadataSetters(t *testing.T) {
	d := NewDocument()
	d.SetTitle("Title")
	d.SetAuthor("Author")
	d.SetSubject("Subject")
	d.SetDescription("Description")
	d.SetKeywords("Keywords")
	d.SetCompany("Company")
	d.SetManager("Manager")
	d.SetTemplate("Template")
	d.SetApplication("Application")

	if d.IR.Title != "Title" {
		t.Fatalf("title = %q", d.IR.Title)
	}
	if d.IR.Author != "Author" {
		t.Fatalf("author = %q", d.IR.Author)
	}
	if d.IR.Subject != "Subject" {
		t.Fatalf("subject = %q", d.IR.Subject)
	}
	if d.IR.Description != "Description" {
		t.Fatalf("description = %q", d.IR.Description)
	}
	if d.IR.Keywords != "Keywords" {
		t.Fatalf("keywords = %q", d.IR.Keywords)
	}
	if d.IR.Company != "Company" {
		t.Fatalf("company = %q", d.IR.Company)
	}
	if d.IR.Manager != "Manager" {
		t.Fatalf("manager = %q", d.IR.Manager)
	}
	if d.IR.Template != "Template" {
		t.Fatalf("template = %q", d.IR.Template)
	}
	if d.IR.Application != "Application" {
		t.Fatalf("application = %q", d.IR.Application)
	}
}

func TestDocumentTables(t *testing.T) {
	d := NewDocument()
	if len(d.Tables()) != 0 {
		t.Fatalf("tables = %d, want 0", len(d.Tables()))
	}

	tbl1 := d.AddTable()
	tbl2 := d.AddTable()

	tables := d.Tables()
	if len(tables) != 2 {
		t.Fatalf("tables = %d, want 2", len(tables))
	}
	if tables[0].ir != tbl1.ir {
		t.Fatal("first table mismatch")
	}
	if tables[1].ir != tbl2.ir {
		t.Fatal("second table mismatch")
	}
}

func TestDocumentTableIterator(t *testing.T) {
	d := NewDocument()
	first := d.AddTable()
	d.AddParagraph().SetText("between")
	second := d.AddTable()

	it := d.TableIterator()
	if !it.HasNext() {
		t.Fatal("expected first table")
	}
	if got := it.Next(); got == nil || got.ir != first.ir {
		t.Fatal("first iterator table mismatch")
	}
	if !it.HasNext() {
		t.Fatal("expected second table")
	}
	if got := it.Next(); got == nil || got.ir != second.ir {
		t.Fatal("second iterator table mismatch")
	}
	if it.HasNext() {
		t.Fatal("expected exhausted iterator")
	}
	if got := it.Next(); got != nil {
		t.Fatalf("exhausted Next = %v, want nil", got)
	}

	var nilIterator *TableIterator
	if nilIterator.HasNext() {
		t.Fatal("nil iterator HasNext = true")
	}
	if got := nilIterator.Next(); got != nil {
		t.Fatalf("nil iterator Next = %v, want nil", got)
	}
}

func TestDocumentAddFooter(t *testing.T) {
	d := NewDocument()
	se := d.AddFooter("default")
	if se == nil {
		t.Fatal("nil story editor")
	}
	p := se.AddParagraph()
	p.SetText("footer text")
	if len(d.footerPtrs) != 1 {
		t.Fatalf("footer ptrs = %d, want 1", len(d.footerPtrs))
	}
	if d.footerPtrs["default"].Paragraphs[0].Runs[0].Text != "footer text" {
		t.Fatal("footer text not stored")
	}
}

func TestDocumentAddFootnote(t *testing.T) {
	d := NewDocument()
	se := d.AddFootnote(1)
	if se == nil {
		t.Fatal("nil story editor")
	}
	p := se.AddParagraph()
	p.SetText("footnote text")
	if len(d.footnotePtrs) != 1 {
		t.Fatalf("footnote ptrs = %d, want 1", len(d.footnotePtrs))
	}
	if d.footnotePtrs[1].Paragraphs[0].Runs[0].Text != "footnote text" {
		t.Fatal("footnote text not stored")
	}
}

func TestDocumentAddEndnote(t *testing.T) {
	d := NewDocument()
	se := d.AddEndnote(2)
	if se == nil {
		t.Fatal("nil story editor")
	}
	p := se.AddParagraph()
	p.SetText("endnote text")
	if len(d.endnotePtrs) != 1 {
		t.Fatalf("endnote ptrs = %d, want 1", len(d.endnotePtrs))
	}
	if d.endnotePtrs[2].Paragraphs[0].Runs[0].Text != "endnote text" {
		t.Fatal("endnote text not stored")
	}
}

func TestDocumentAddComment(t *testing.T) {
	d := NewDocument()
	se := d.AddComment(5)
	if se == nil {
		t.Fatal("nil story editor")
	}
	p := se.AddParagraph()
	p.SetText("comment text")
	se.SetAuthor("Alice")
	se.SetDate("2024-01-01")
	if len(d.commentPtrs) != 1 {
		t.Fatalf("comment ptrs = %d, want 1", len(d.commentPtrs))
	}
	story := d.commentPtrs[5]
	if story.Paragraphs[0].Runs[0].Text != "comment text" {
		t.Fatal("comment text not stored")
	}
	if story.Author != "Alice" {
		t.Fatalf("author = %q", story.Author)
	}
	if story.Date != "2024-01-01" {
		t.Fatalf("date = %q", story.Date)
	}
}

func TestDocumentStoryLists(t *testing.T) {
	d := NewDocument()
	d.AddHeader("first").AddParagraph().SetText("first header")
	d.AddHeader("default").AddParagraph().SetText("default header")
	d.AddFooter("even").AddParagraph().SetText("even footer")
	d.AddFooter("default").AddParagraph().SetText("default footer")
	d.AddFootnote(7).AddParagraph().SetText("footnote 7")
	d.AddFootnote(2).AddParagraph().SetText("footnote 2")
	d.AddEndnote(4).AddParagraph().SetText("endnote 4")
	d.AddEndnote(1).AddParagraph().SetText("endnote 1")
	d.AddComment(5).AddParagraph().SetText("comment 5")
	d.AddComment(3).AddParagraph().SetText("comment 3")

	headers := d.Headers()
	if len(headers) != 2 {
		t.Fatalf("headers = %d, want 2", len(headers))
	}
	if headers[0].Type != "default" || headers[1].Type != "first" {
		t.Fatalf("header order = %q, %q; want default, first", headers[0].Type, headers[1].Type)
	}
	headers[0].Story.AddParagraph().SetText("mutated")
	if got := d.Header("default").Paragraphs()[1].Text(); got != "mutated" {
		t.Fatalf("mutated header text = %q, want mutated", got)
	}

	footers := d.Footers()
	if len(footers) != 2 {
		t.Fatalf("footers = %d, want 2", len(footers))
	}
	if footers[0].Type != "default" || footers[1].Type != "even" {
		t.Fatalf("footer order = %q, %q; want default, even", footers[0].Type, footers[1].Type)
	}

	footnotes := d.Footnotes()
	if len(footnotes) != 2 {
		t.Fatalf("footnotes = %d, want 2", len(footnotes))
	}
	if footnotes[0].ID != 2 || footnotes[1].ID != 7 {
		t.Fatalf("footnote order = %d, %d; want 2, 7", footnotes[0].ID, footnotes[1].ID)
	}
	footnotes[0].Story.AddParagraph().SetText("mutated footnote")
	if got := d.Footnote(2).Paragraphs()[1].Text(); got != "mutated footnote" {
		t.Fatalf("mutated footnote text = %q, want mutated footnote", got)
	}

	endnotes := d.Endnotes()
	if len(endnotes) != 2 {
		t.Fatalf("endnotes = %d, want 2", len(endnotes))
	}
	if endnotes[0].ID != 1 || endnotes[1].ID != 4 {
		t.Fatalf("endnote order = %d, %d; want 1, 4", endnotes[0].ID, endnotes[1].ID)
	}

	comments := d.Comments()
	if len(comments) != 2 {
		t.Fatalf("comments = %d, want 2", len(comments))
	}
	if comments[0].ID != 3 || comments[1].ID != 5 {
		t.Fatalf("comment order = %d, %d; want 3, 5", comments[0].ID, comments[1].ID)
	}
}

func TestDocumentStoryListsFromOpenIR(t *testing.T) {
	irDoc := &wordir.Document{
		Headers: map[string]wordir.Story{
			"default": {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "header"}}}}},
		},
		Footers: map[string]wordir.Story{
			"default": {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "footer"}}}}},
		},
		Footnotes: map[int]wordir.Story{
			1: {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "footnote"}}}}},
		},
		Endnotes: map[int]wordir.Story{
			2: {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "endnote"}}}}},
		},
		Comments: map[int]wordir.Story{
			3: {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "comment"}}}}},
		},
	}

	d := OpenIR(irDoc)
	if got := d.Headers()[0].Story.Paragraphs()[0].Text(); got != "header" {
		t.Fatalf("header text = %q, want header", got)
	}
	if got := d.Footers()[0].Story.Paragraphs()[0].Text(); got != "footer" {
		t.Fatalf("footer text = %q, want footer", got)
	}
	if got := d.Footnotes()[0].Story.Paragraphs()[0].Text(); got != "footnote" {
		t.Fatalf("footnote text = %q, want footnote", got)
	}
	if got := d.Endnotes()[0].Story.Paragraphs()[0].Text(); got != "endnote" {
		t.Fatalf("endnote text = %q, want endnote", got)
	}
	if got := d.Comments()[0].Story.Paragraphs()[0].Text(); got != "comment" {
		t.Fatalf("comment text = %q, want comment", got)
	}
}

func TestRunAddImageFromFile(t *testing.T) {
	// Create a temporary 1x1 PNG.
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	if err := r.AddImageFromFile(path); err != nil {
		t.Fatalf("AddImageFromFile: %v", err)
	}
	if r.ir().Kind != "image" {
		t.Fatalf("kind = %q, want image", r.ir().Kind)
	}
	if r.ir().Image == nil {
		t.Fatal("image not set")
	}
	if r.ir().Image.MIMEType != "image/png" {
		t.Fatalf("mime = %q", r.ir().Image.MIMEType)
	}
	if r.ir().Image.Width == 0 || r.ir().Image.Height == 0 {
		t.Fatal("image dimensions not set")
	}
}

func TestRunAddImageFromFileNotFound(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	if err := r.AddImageFromFile("/nonexistent/path.png"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestDocumentImages enumerates inline images across body and stories.
func TestDocumentImages(t *testing.T) {
	d := NewDocument()

	// Body image
	p1 := d.AddParagraph()
	r1 := p1.AddRun()
	r1.ir().Image = &wordir.Image{MIMEType: "image/png", Width: 100, Height: 200, Alt: "body img"}

	// Header image
	h := d.AddHeader("default")
	p2 := h.AddParagraph()
	r2 := p2.AddRun()
	r2.ir().Image = &wordir.Image{MIMEType: "image/jpeg", Width: 300, Height: 400}

	// Footer image
	f := d.AddFooter("default")
	p3 := f.AddParagraph()
	r3 := p3.AddRun()
	r3.ir().Image = &wordir.Image{MIMEType: "image/gif", Width: 50, Height: 50}

	imgs := d.Images()
	if len(imgs) != 3 {
		t.Fatalf("len(imgs) = %d, want 3", len(imgs))
	}

	if imgs[0].Location != "body" {
		t.Fatalf("img0 location = %q, want body", imgs[0].Location)
	}
	if imgs[0].MIMEType != "image/png" {
		t.Fatalf("img0 mime = %q, want image/png", imgs[0].MIMEType)
	}
	if imgs[0].Alt != "body img" {
		t.Fatalf("img0 alt = %q, want body img", imgs[0].Alt)
	}
	if imgs[0].Width != 100 || imgs[0].Height != 200 {
		t.Fatalf("img0 dims = %dx%d, want 100x200", imgs[0].Width, imgs[0].Height)
	}

	if imgs[1].Location != "header" {
		t.Fatalf("img1 location = %q, want header", imgs[1].Location)
	}
	if imgs[1].MIMEType != "image/jpeg" {
		t.Fatalf("img1 mime = %q, want image/jpeg", imgs[1].MIMEType)
	}

	if imgs[2].Location != "footer" {
		t.Fatalf("img2 location = %q, want footer", imgs[2].Location)
	}
	if imgs[2].MIMEType != "image/gif" {
		t.Fatalf("img2 mime = %q, want image/gif", imgs[2].MIMEType)
	}
}

func TestDocumentImagesIncludesTablesAndNestedShapes(t *testing.T) {
	d := NewDocument()

	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	cellRun := cell.AddParagraph().AddRun()
	cellRun.ir().Image = &wordir.Image{MIMEType: "image/png", Width: 10, Height: 20, Alt: "table image"}

	textBoxPara := wordir.Paragraph{
		Runs: []wordir.Run{{
			Image: &wordir.Image{MIMEType: "image/jpeg", Width: 30, Height: 40, Alt: "textbox image"},
		}},
	}
	shapePara := wordir.Paragraph{
		Runs: []wordir.Run{{
			Image: &wordir.Image{MIMEType: "image/gif", Width: 50, Height: 60, Alt: "shape image"},
		}},
	}
	p := d.AddParagraph()
	r := p.AddRun()
	r.ir().TextBox = &wordir.TextBox{
		Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &textBoxPara}},
	}
	r.ir().Shape = &wordir.Shape{
		Children: []wordir.Shape{{
			TextBox: &wordir.TextBox{
				Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &shapePara}},
			},
		}},
	}

	imgs := d.Images()
	if len(imgs) != 3 {
		t.Fatalf("len(imgs) = %d, want 3", len(imgs))
	}
	for i, img := range imgs {
		if img.Location != "body" {
			t.Fatalf("img%d location = %q, want body", i, img.Location)
		}
	}
	if imgs[0].Alt != "table image" {
		t.Fatalf("img0 alt = %q, want table image", imgs[0].Alt)
	}
	if imgs[1].Alt != "textbox image" {
		t.Fatalf("img1 alt = %q, want textbox image", imgs[1].Alt)
	}
	if imgs[2].Alt != "shape image" {
		t.Fatalf("img2 alt = %q, want shape image", imgs[2].Alt)
	}
}

func TestParagraphInsertRunFrom(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("alpha")
	r1.SetBold(true)

	p2 := d.AddParagraph()
	inserted := p2.InsertRunFrom(0, r1)
	if inserted == nil {
		t.Fatal("expected inserted run")
	}
	if inserted.Text() != "alpha" {
		t.Fatalf("text = %q, want alpha", inserted.Text())
	}
	if !inserted.Bold() {
		t.Fatal("expected bold")
	}

	// Insert at negative position → beginning
	r2 := p2.AddRun()
	r2.SetText("beta")
	inserted2 := p2.InsertRunFrom(-1, r1)
	if p2.Runs()[0].Text() != "alpha" {
		t.Fatalf("first run text = %q, want alpha", p2.Runs()[0].Text())
	}

	// Insert beyond length → append
	inserted3 := p2.InsertRunFrom(100, r1)
	last := p2.Runs()[len(p2.Runs())-1]
	if last.Text() != "alpha" {
		t.Fatalf("last run text = %q, want alpha", last.Text())
	}
	_ = inserted2
	_ = inserted3
}

func TestParagraphAddRunFrom(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	r1 := p1.AddRun()
	r1.SetText("hello")
	r1.SetItalic(true)

	p2 := d.AddParagraph()
	added := p2.AddRunFrom(r1)
	if added == nil {
		t.Fatal("expected added run")
	}
	if added.Text() != "hello" {
		t.Fatalf("text = %q, want hello", added.Text())
	}
	if !added.Italic() {
		t.Fatal("expected italic")
	}

	// nil safety
	if p2.AddRunFrom(nil) != nil {
		t.Fatal("expected nil for nil source")
	}
}

func TestDocumentImageRuns(t *testing.T) {
	d := NewDocument()

	p1 := d.AddParagraph()
	r1 := p1.AddRun()
	r1.ir().Image = &wordir.Image{MIMEType: "image/png", Width: 100, Height: 200}

	h := d.AddHeader("default")
	p2 := h.AddParagraph()
	r2 := p2.AddRun()
	r2.ir().Image = &wordir.Image{MIMEType: "image/jpeg", Width: 300, Height: 400}

	// Non-image run should be ignored
	p3 := d.AddParagraph()
	p3.AddRun().SetText("no image")

	irs := d.ImageRuns()
	if len(irs) != 2 {
		t.Fatalf("len(imageRuns) = %d, want 2", len(irs))
	}
	if irs[0].Location != "body" {
		t.Fatalf("img0 location = %q, want body", irs[0].Location)
	}
	if irs[0].Run == nil {
		t.Fatal("expected non-nil run")
	}
	if irs[0].Run.ir().Image == nil {
		t.Fatal("expected image in run")
	}
	if irs[1].Location != "header" {
		t.Fatalf("img1 location = %q, want header", irs[1].Location)
	}
}

func TestDocumentImageRunsIncludesNestedTextBox(t *testing.T) {
	d := NewDocument()
	nestedPara := wordir.Paragraph{
		Runs: []wordir.Run{{
			Image: &wordir.Image{MIMEType: "image/png", Width: 100, Height: 200},
		}},
	}
	p := d.AddParagraph()
	r := p.AddRun()
	r.ir().TextBox = &wordir.TextBox{
		Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &nestedPara}},
	}

	irs := d.ImageRuns()
	if len(irs) != 1 {
		t.Fatalf("len(imageRuns) = %d, want 1", len(irs))
	}
	if irs[0].Location != "body" {
		t.Fatalf("location = %q, want body", irs[0].Location)
	}
	if irs[0].Run == nil || irs[0].Run.ir().Image == nil {
		t.Fatal("expected nested image run")
	}
	if irs[0].Run.ir().Image.MIMEType != "image/png" {
		t.Fatalf("mime = %q, want image/png", irs[0].Run.ir().Image.MIMEType)
	}
}

func TestDocumentDefaults(t *testing.T) {
	d := NewDocument()
	if d.DefaultParagraphStyleID() != "" {
		t.Fatalf("default para style = %q", d.DefaultParagraphStyleID())
	}
	d.SetDefaultParagraphStyle("Normal")
	if d.DefaultParagraphStyleID() != "Normal" {
		t.Fatalf("default para style = %q", d.DefaultParagraphStyleID())
	}

	// Run format
	rf := wordir.RunFormat{FontASCII: "Arial", Color: "FF0000"}
	d.SetDefaultRunFormat(rf)
	gotRF := d.DefaultRunFormat()
	if gotRF.FontASCII != "Arial" || gotRF.Color != "FF0000" {
		t.Fatalf("default run format = %+v", gotRF)
	}

	// Paragraph format
	pf := wordir.ParagraphFormat{Alignment: "center"}
	d.SetDefaultParagraphFormat(pf)
	gotPF := d.DefaultParagraphFormat()
	if gotPF.Alignment != "center" {
		t.Fatalf("default paragraph format = %+v", gotPF)
	}
}

func TestDocumentDefaultsNilSafety(t *testing.T) {
	var d *Document
	if d.DefaultParagraphStyleID() != "" {
		t.Fatal("expected empty")
	}
	d.SetDefaultParagraphStyle("x")                       // no panic
	d.SetDefaultRunFormat(wordir.RunFormat{})             // no panic
	d.SetDefaultParagraphFormat(wordir.ParagraphFormat{}) // no panic
	rf := d.DefaultRunFormat()
	if rf.FontASCII != "" || rf.Color != "" {
		t.Fatal("expected zero run format")
	}
	pf := d.DefaultParagraphFormat()
	if pf.Alignment != "" {
		t.Fatal("expected zero paragraph format")
	}
}

func TestParagraphIterator(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.AddRun().SetText("first")
	p2 := d.AddParagraph()
	p2.AddRun().SetText("second")

	it := d.ParagraphIterator()
	if !it.HasNext() {
		t.Fatal("expected HasNext")
	}
	got := it.Next()
	if got == nil || got.Text() != "first" {
		t.Fatalf("first = %v", got)
	}
	if !it.HasNext() {
		t.Fatal("expected HasNext after first")
	}
	got = it.Next()
	if got == nil || got.Text() != "second" {
		t.Fatalf("second = %v", got)
	}
	if it.HasNext() {
		t.Fatal("expected no more paragraphs")
	}
	if it.Next() != nil {
		t.Fatal("expected nil after exhaustion")
	}
}

func TestDocumentBookmarks(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.AddBookmarkStart("bm1")
	r := p.AddRun()
	r.SetText("hello")
	p.AddBookmarkStart("bm2")

	bms := d.Bookmarks()
	if len(bms) != 2 {
		t.Fatalf("bookmarks = %d, want 2", len(bms))
	}
	if bms[0].Name != "bm1" || bms[0].Location != "body" {
		t.Fatalf("bm1 = %+v", bms[0])
	}
	if bms[1].Name != "bm2" || bms[1].Location != "body" {
		t.Fatalf("bm2 = %+v", bms[1])
	}
	if d.BookmarkCount() != 2 {
		t.Fatalf("count = %d, want 2", d.BookmarkCount())
	}
}

func TestDocumentFieldsAlias(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.AddRun().SetPageNumber()

	fields := d.Fields()
	if len(fields) != 1 {
		t.Fatalf("fields = %d, want 1", len(fields))
	}
	if fields[0].Kind != "PAGE" {
		t.Fatalf("kind = %q", fields[0].Kind)
	}
}