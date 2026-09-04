package edit

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
)

func TestNewDocument(t *testing.T) {
	d := NewDocument()
	if d.IR == nil {
		t.Fatal("nil IR")
	}
	if len(d.IR.Blocks) != 0 {
		t.Fatalf("blocks = %d, want 0", len(d.IR.Blocks))
	}
}

func TestDocumentAddParagraph(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	if p == nil {
		t.Fatal("nil paragraph")
	}
	if len(d.IR.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].Kind != "paragraph" {
		t.Fatalf("block kind = %q, want paragraph", d.IR.Blocks[0].Kind)
	}
	if len(d.IR.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(d.IR.Paragraphs))
	}

	// Add a run and verify canonical data.
	r := p.AddRun()
	r.SetText("hello")
	if p.Runs()[0].Text() != "hello" {
		t.Fatalf("run text = %q, want hello", p.Runs()[0].Text())
	}
	// Block pointer should still be consistent.
	if d.IR.Blocks[0].Paragraph == nil || d.IR.Blocks[0].Paragraph.Runs[0].Text != "hello" {
		t.Fatal("block paragraph pointer out of sync")
	}
}

func TestDocumentInsertParagraphAt(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("first")
	p2 := d.InsertParagraphAt(0)
	p2.SetText("second")

	if len(d.IR.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].Paragraph == nil || d.IR.Blocks[0].Paragraph.Text != "second" {
		t.Fatalf("block[0] text = %q, want second", d.IR.Blocks[0].Paragraph.Text)
	}
	if d.IR.Blocks[1].Paragraph == nil || d.IR.Blocks[1].Paragraph.Text != "first" {
		t.Fatalf("block[1] text = %q, want first", d.IR.Blocks[1].Paragraph.Text)
	}
	// Verify indices are sequential.
	for i, b := range d.IR.Blocks {
		if b.Index != i {
			t.Fatalf("block[%d].Index = %d, want %d", i, b.Index, i)
		}
	}
}

func TestDocumentRemoveParagraph(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p2 := d.AddParagraph()
	d.RemoveParagraph(p1)

	if len(d.IR.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].Paragraph != p2.ir {
		t.Fatal("remaining block should point to p2")
	}
}

func TestDocumentAddTable(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	if tbl == nil {
		t.Fatal("nil table")
	}
	row := tbl.AddRow()
	if row == nil {
		t.Fatal("nil row")
	}
	cell := row.AddCell()
	if cell == nil {
		t.Fatal("nil cell")
	}
	p := cell.AddParagraph()
	p.SetText("cell text")

	if len(d.IR.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].Kind != "table" {
		t.Fatalf("block kind = %q, want table", d.IR.Blocks[0].Kind)
	}
}

func TestDocumentParagraphsOrder(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("a")
	d.AddTable()
	p2 := d.AddParagraph()
	p2.SetText("b")

	paras := d.Paragraphs()
	if len(paras) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(paras))
	}
	if paras[0].Text() != "a" || paras[1].Text() != "b" {
		t.Fatalf("paragraph order wrong: %q, %q", paras[0].Text(), paras[1].Text())
	}
}

func TestRunFormatting(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()

	r.SetBold(true)
	if !r.Bold() {
		t.Fatal("bold not set")
	}
	r.SetItalic(true)
	if !r.Italic() {
		t.Fatal("italic not set")
	}
	r.SetFontSize(24)
	if r.FontSize() != 24 {
		t.Fatal("font size not set")
	}
	r.SetColor("FF0000")
	if r.Color() != "FF0000" {
		t.Fatal("color not set")
	}
	r.SetSubscript(true)
	if r.ir().Format.VertAlignVal != "subscript" {
		t.Fatal("subscript not set")
	}
	r.SetSuperscript(true)
	if r.ir().Format.VertAlignVal != "superscript" {
		t.Fatal("superscript not set")
	}
}

func TestParagraphFormatting(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.SetAlignment("center")
	p.SetSpacingBefore(120)
	p.SetIndentFirstLine(240)

	if p.ir.Format.Alignment != "center" {
		t.Fatalf("alignment = %q, want center", p.ir.Format.Alignment)
	}
	if p.ir.Format.SpacingBefore == nil || *p.ir.Format.SpacingBefore != 120 {
		t.Fatalf("spacing before = %v, want 120", p.ir.Format.SpacingBefore)
	}
	if p.ir.Format.IndentFirstLine == nil || *p.ir.Format.IndentFirstLine != 240 {
		t.Fatalf("indent first = %v, want 240", p.ir.Format.IndentFirstLine)
	}
}

func TestDocumentMetadata(t *testing.T) {
	d := NewDocument()
	d.SetTitle("Test")
	d.SetAuthor("Alice")
	d.SetSubject("Demo")
	if d.IR.Title != "Test" {
		t.Fatalf("title = %q", d.IR.Title)
	}
	if d.IR.Author != "Alice" {
		t.Fatalf("author = %q", d.IR.Author)
	}
	if d.IR.Subject != "Demo" {
		t.Fatalf("subject = %q", d.IR.Subject)
	}
}

func TestOpenIR(t *testing.T) {
	irDoc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			{Index: 5, Runs: []wordir.Run{{Text: "existing"}}},
		},
	}
	d := OpenIR(irDoc)
	if d.nextParaIndex != 6 {
		t.Fatalf("nextParaIndex = %d, want 6", d.nextParaIndex)
	}
	p := d.AddParagraph()
	if p.ir.Index != 6 {
		t.Fatalf("new paragraph index = %d, want 6", p.ir.Index)
	}
}

func TestToOOXML(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.SetText("Hello world")
	pkg, err := d.ToOOXML()
	if err != nil {
		t.Fatalf("ToOOXML: %v", err)
	}
	if pkg == nil || pkg.Document == nil {
		t.Fatal("nil package or document")
	}
	bodyBlocks := pkg.Document.Body.Blocks
	if len(bodyBlocks) == 0 {
		t.Fatal("no body blocks")
	}
	if bodyBlocks[0].Paragraph == nil {
		t.Fatal("first block is not a paragraph")
	}
	if len(bodyBlocks[0].Paragraph.Runs) == 0 {
		t.Fatal("no runs in paragraph")
	}
	if bodyBlocks[0].Paragraph.Runs[0].Contents[0].Text.Value != "Hello world" {
		t.Fatalf("text = %q, want Hello world", bodyBlocks[0].Paragraph.Runs[0].Contents[0].Text.Value)
	}
}

func TestDocumentRemoveTable(t *testing.T) {
	d := NewDocument()
	t1 := d.AddTable()
	t2 := d.AddTable()
	d.RemoveTable(t1)

	if len(d.IR.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].Table != t2.ir {
		t.Fatal("remaining block should point to t2")
	}
}

func TestDocumentInsertTableAt(t *testing.T) {
	d := NewDocument()
	d.AddParagraph()
	tbl := d.InsertTableAt(0)
	if tbl == nil {
		t.Fatal("nil table")
	}
	if len(d.IR.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].Kind != "table" {
		t.Fatalf("block[0] kind = %q, want table", d.IR.Blocks[0].Kind)
	}
	if d.IR.Blocks[1].Kind != "paragraph" {
		t.Fatalf("block[1] kind = %q, want paragraph", d.IR.Blocks[1].Kind)
	}
}

func TestStoryEditor(t *testing.T) {
	d := NewDocument()
	se := d.AddHeader("default")
	p := se.AddParagraph()
	p.SetText("header text")
	se.SetAuthor("Alice")

	// Flush and verify IR.
	d.flushStoryPtrs()
	if len(d.IR.Headers["default"].Paragraphs) != 1 {
		t.Fatalf("header paragraphs = %d, want 1", len(d.IR.Headers["default"].Paragraphs))
	}
	if d.IR.Headers["default"].Paragraphs[0].Text != "header text" {
		t.Fatalf("header text = %q, want header text", d.IR.Headers["default"].Paragraphs[0].Text)
	}
}

func TestDocumentSections(t *testing.T) {
	d := NewDocument()
	s1 := d.AddSection()
	s1.SetPageSize(12240, 15840)
	s1.SetPageMargins(1440, 1440, 1440, 1440)
	s1.SetBreakKind("nextPage")

	if len(d.IR.Sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(d.IR.Sections))
	}
	sec := d.IR.Sections[0]
	if sec.PageSize == nil || sec.PageSize.Width != 12240 || sec.PageSize.Height != 15840 {
		t.Fatalf("page size = %+v, want 12240x15840", sec.PageSize)
	}
	if sec.PageMargin == nil || sec.PageMargin.Top != 1440 {
		t.Fatalf("top margin = %d, want 1440", sec.PageMargin.Top)
	}
	if sec.Type != "nextPage" {
		t.Fatalf("break kind = %q, want nextPage", sec.Type)
	}

	secs := d.Sections()
	if len(secs) != 1 {
		t.Fatalf("Sections() = %d, want 1", len(secs))
	}
}

func TestEnsureParagraphStyle(t *testing.T) {
	d := NewDocument()
	s := d.EnsureParagraphStyle("Heading1", "Heading 1")
	if s.ID() != "Heading1" || s.Name() != "Heading 1" {
		t.Fatalf("style = id=%q name=%q", s.ID(), s.Name())
	}
	if d.IR.Styles["Heading1"].Name != "Heading 1" {
		t.Fatalf("IR style name = %q", d.IR.Styles["Heading1"].Name)
	}

	// Verify post-creation mutations are persisted.
	s.SetAlignment("center")
	s.SetSpacingBefore(240)
	s.SetRunBold(true)
	if d.IR.Styles["Heading1"].Format.Alignment != "center" {
		t.Fatal("alignment not persisted")
	}
	if d.IR.Styles["Heading1"].Format.SpacingBefore == nil || *d.IR.Styles["Heading1"].Format.SpacingBefore != 240 {
		t.Fatal("spacing before not persisted")
	}
	if d.IR.Styles["Heading1"].RunFormat.Bold == nil || !*d.IR.Styles["Heading1"].RunFormat.Bold {
		t.Fatal("run bold not persisted")
	}
}

func TestEnsureCharacterStyle(t *testing.T) {
	d := NewDocument()
	s := d.EnsureCharacterStyle("Emphasis", "Emphasis")
	if s.ID() != "Emphasis" || s.Name() != "Emphasis" {
		t.Fatalf("char style = id=%q name=%q", s.ID(), s.Name())
	}

	s.SetItalic(true)
	s.SetFontSize(20)
	if d.IR.CharacterStyles["Emphasis"].Format.Italic == nil || !*d.IR.CharacterStyles["Emphasis"].Format.Italic {
		t.Fatal("italic not persisted")
	}
	if d.IR.CharacterStyles["Emphasis"].Format.FontSize == nil || *d.IR.CharacterStyles["Emphasis"].Format.FontSize != 20 {
		t.Fatal("font size not persisted")
	}
}

func TestParagraphRemoveRun(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("a")
	r2 := p.AddRun()
	r2.SetText("b")
	p.RemoveRun(r1)
	if len(p.Runs()) != 1 {
		t.Fatalf("runs = %d, want 1", len(p.Runs()))
	}
	if p.Runs()[0].Text() != "b" {
		t.Fatalf("run text = %q, want b", p.Runs()[0].Text())
	}
}

func TestRunCharacterStyleID(t *testing.T) {
	d := NewDocument()
	d.EnsureCharacterStyle("Emphasis", "Emphasis")
	run := d.AddParagraph().AddRun()
	run.SetStyleID(" Emphasis ")
	if got := run.StyleID(); got != "Emphasis" {
		t.Fatalf("StyleID = %q, want Emphasis", got)
	}
}

func TestValidateRunCharacterStyleReference(t *testing.T) {
	d := NewDocument()
	run := d.AddParagraph().AddRun()
	run.SetText("styled")
	run.SetStyleID("MissingStyle")
	issues := d.Validate()
	found := false
	for _, issue := range issues {
		if strings.Contains(issue, `references missing character style "MissingStyle"`) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("issues=%v, want missing character-style reference", issues)
	}
}

func TestParagraphInsertRunAt(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("a")
	r2 := p.AddRun()
	r2.SetText("b")

	// Insert at beginning
	ri := p.InsertRunAt(0)
	ri.SetText("x")
	if len(p.Runs()) != 3 {
		t.Fatalf("runs = %d, want 3", len(p.Runs()))
	}
	if p.Runs()[0].Text() != "x" || p.Runs()[1].Text() != "a" || p.Runs()[2].Text() != "b" {
		t.Fatalf("runs text = %v, want [x a b]", []string{p.Runs()[0].Text(), p.Runs()[1].Text(), p.Runs()[2].Text()})
	}

	// Insert in middle
	ri2 := p.InsertRunAt(2)
	ri2.SetText("y")
	if len(p.Runs()) != 4 {
		t.Fatalf("runs = %d, want 4", len(p.Runs()))
	}
	if p.Runs()[2].Text() != "y" {
		t.Fatalf("run[2] text = %q, want y", p.Runs()[2].Text())
	}

	// Append via large index
	ri3 := p.InsertRunAt(100)
	ri3.SetText("z")
	if len(p.Runs()) != 5 {
		t.Fatalf("runs = %d, want 5", len(p.Runs()))
	}
	if p.Runs()[4].Text() != "z" {
		t.Fatalf("run[4] text = %q, want z", p.Runs()[4].Text())
	}

	// Negative index inserts at start
	ri4 := p.InsertRunAt(-1)
	ri4.SetText("w")
	if len(p.Runs()) != 6 {
		t.Fatalf("runs = %d, want 6", len(p.Runs()))
	}
	if p.Runs()[0].Text() != "w" {
		t.Fatalf("run[0] text = %q, want w", p.Runs()[0].Text())
	}
}

func TestParagraphRunCountAndRunAt(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.AddRun().SetText("alpha")
	p.AddRun().SetText("beta")

	if p.RunCount() != 2 {
		t.Fatalf("RunCount = %d, want 2", p.RunCount())
	}
	if got := p.RunAt(0); got == nil || got.Text() != "alpha" {
		t.Fatalf("RunAt(0) = %v, want alpha run", got)
	}
	if got := p.RunAt(1); got == nil || got.Text() != "beta" {
		t.Fatalf("RunAt(1) = %v, want beta run", got)
	}
	p.RunAt(1).SetText("bravo")
	if p.Runs()[1].Text() != "bravo" {
		t.Fatalf("mutated run text = %q, want bravo", p.Runs()[1].Text())
	}
	if got := p.RunAt(-1); got != nil {
		t.Fatalf("RunAt(-1) = %v, want nil", got)
	}
	if got := p.RunAt(2); got != nil {
		t.Fatalf("RunAt(2) = %v, want nil", got)
	}

	var nilParagraph *Paragraph
	if nilParagraph.RunCount() != 0 {
		t.Fatalf("nil RunCount = %d, want 0", nilParagraph.RunCount())
	}
	if nilParagraph.RunAt(0) != nil {
		t.Fatal("nil RunAt returned non-nil run")
	}
}

func TestRunTextAt(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("alpha")

	if got := r.TextAt(0); got != "alpha" {
		t.Fatalf("TextAt(0) = %q, want alpha", got)
	}
	if got := r.TextAt(1); got != "" {
		t.Fatalf("TextAt(1) = %q, want empty", got)
	}
	if got := r.TextAt(-1); got != "" {
		t.Fatalf("TextAt(-1) = %q, want empty", got)
	}

	var nilRun *Run
	if got := nilRun.TextAt(0); got != "" {
		t.Fatalf("nil TextAt(0) = %q, want empty", got)
	}
}

func TestParagraphRemoveRunAt(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("a")
	r2 := p.AddRun()
	r2.SetText("b")
	r3 := p.AddRun()
	r3.SetText("c")

	p.RemoveRunAt(1)
	if len(p.Runs()) != 2 {
		t.Fatalf("runs = %d, want 2", len(p.Runs()))
	}
	if p.Runs()[0].Text() != "a" || p.Runs()[1].Text() != "c" {
		t.Fatalf("runs text = %v, want [a c]", []string{p.Runs()[0].Text(), p.Runs()[1].Text()})
	}

	// Out of range is no-op
	p.RemoveRunAt(100)
	if len(p.Runs()) != 2 {
		t.Fatalf("runs = %d, want 2 after noop", len(p.Runs()))
	}
	p.RemoveRunAt(-1)
	if len(p.Runs()) != 2 {
		t.Fatalf("runs = %d, want 2 after noop", len(p.Runs()))
	}
}

func TestParagraphAddTabAndPageBreak(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	tab := p.AddTab()
	if tab == nil || tab.ir().Kind != "tab" {
		t.Fatal("expected tab run")
	}
	brk := p.AddPageBreak()
	if brk == nil || brk.ir().Kind != "pageBreak" {
		t.Fatal("expected pageBreak run")
	}
}

func TestRunGetters(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetBold(true)
	r.SetItalic(true)
	r.SetFontSize(24)
	r.SetColor("FF0000")
	r.SetFontFamily("Arial")
	if !r.Bold() {
		t.Fatal("Bold() = false")
	}
	if !r.Italic() {
		t.Fatal("Italic() = false")
	}
	if r.FontSize() != 24 {
		t.Fatalf("FontSize() = %d, want 24", r.FontSize())
	}
	if r.Color() != "FF0000" {
		t.Fatalf("Color() = %q, want FF0000", r.Color())
	}
	if r.FontFamily() != "Arial" {
		t.Fatalf("FontFamily() = %q, want Arial", r.FontFamily())
	}
}

func TestDocumentRemoveSection(t *testing.T) {
	d := NewDocument()
	s1 := d.AddSection()
	d.AddSection()
	d.RemoveSection(s1)
	if len(d.Sections()) != 1 {
		t.Fatalf("sections = %d, want 1", len(d.Sections()))
	}
}

func TestSectionNewExtendedSetters(t *testing.T) {
	d := NewDocument()
	s := d.AddSection()
	s.SetTextDirection("tbRl")
	s.SetPageNumberFormat("upperRoman")
	s.SetLineNumbering(5, 1, "newPage")
	s.SetDocGrid(360, 180)

	if s.ir().TextDirection != "tbRl" {
		t.Fatalf("text direction = %q", s.ir().TextDirection)
	}
	if s.ir().PageNumberFormat != "upperRoman" {
		t.Fatalf("page number format = %q", s.ir().PageNumberFormat)
	}
	if s.ir().LineNumbering == nil || s.ir().LineNumbering.CountBy != 5 {
		t.Fatal("line numbering not set")
	}
	if s.ir().DocGrid == nil || s.ir().DocGrid.LinePitch != 360 {
		t.Fatal("doc grid not set")
	}
}

func TestTableFormatting(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	row.SetHeight(360)
	row.SetHeaderRow(true)

	cell := row.AddCell()
	cell.SetWidth(1440)
	cell.SetVerticalAlignment("center")
	cell.SetBackground("FFFF00")
	cell.SetBorderTop("single", 1.0, "000000")
	cell.SetBorderBottom("single", 1.0, "000000")
	cell.SetBorderLeft("single", 1.0, "000000")
	cell.SetBorderRight("single", 1.0, "000000")
	cell.SetGridSpan(2)

	if ir := row.ir(); ir == nil || ir.Format.Height == nil || *ir.Format.Height != 360 {
		t.Fatal("row height not set")
	}
	if ir := row.ir(); ir == nil || !ir.Format.IsHeader {
		t.Fatal("header row not set")
	}

	cir := cell.ir()
	if cir == nil || cir.Format.Width == nil || *cir.Format.Width != 1440 {
		t.Fatal("cell width not set")
	}
	if cir.Format.VertAlign != "center" {
		t.Fatalf("cell valign = %q, want center", cir.Format.VertAlign)
	}
	if cir.Format.Shading != "FFFF00" {
		t.Fatalf("cell shading = %q, want FFFF00", cir.Format.Shading)
	}
	if cir.Format.Borders == nil || cir.Format.Borders.Top == nil || cir.Format.Borders.Top.Val != "single" {
		t.Fatal("cell top border not set")
	}
	if cir.Format.GridSpan != 2 {
		t.Fatalf("gridSpan = %d, want 2", cir.Format.GridSpan)
	}
}

func TestTableCellParagraphs(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	p1 := cell.AddParagraph()
	p1.SetText("first")
	p2 := cell.AddParagraph()
	p2.SetText("second")

	paras := cell.Paragraphs()
	if len(paras) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(paras))
	}
	if paras[0].Text() != "first" {
		t.Fatalf("para[0] text = %q, want first", paras[0].Text())
	}
	if paras[1].Text() != "second" {
		t.Fatalf("para[1] text = %q, want second", paras[1].Text())
	}
	if cell.Text() != "first\nsecond" {
		t.Fatalf("cell text = %q, want first\\nsecond", cell.Text())
	}
}

func TestInsertSectionAt(t *testing.T) {
	d := NewDocument()
	s0 := d.AddSection()
	s0.SetBreakKind("nextPage")
	s2 := d.AddSection()
	s2.SetBreakKind("continuous")

	// Insert in the middle.
	s1 := d.InsertSectionAt(1)
	s1.SetBreakKind("oddPage")

	if len(d.IR.Sections) != 3 {
		t.Fatalf("sections = %d, want 3", len(d.IR.Sections))
	}
	if d.IR.Sections[0].Type != "nextPage" {
		t.Fatalf("section[0] type = %q", d.IR.Sections[0].Type)
	}
	if d.IR.Sections[1].Type != "oddPage" {
		t.Fatalf("section[1] type = %q", d.IR.Sections[1].Type)
	}
	if d.IR.Sections[2].Type != "continuous" {
		t.Fatalf("section[2] type = %q", d.IR.Sections[2].Type)
	}
}

func TestNumberingDefinition(t *testing.T) {
	d := NewDocument()
	nd := d.EnsureNumberingDefinition(1, 0)
	if nd.NumID() != 1 {
		t.Fatalf("NumID = %d, want 1", nd.NumID())
	}
	if nd.AbstractNumID() != 0 {
		t.Fatalf("AbstractNumID = %d, want 0", nd.AbstractNumID())
	}

	lvl := nd.SetLevel(0, "decimal", "%1.")
	if lvl == nil {
		t.Fatal("SetLevel returned nil")
	}
	if lvl.Format != "decimal" || lvl.Text != "%1." {
		t.Fatalf("level format=%q text=%q", lvl.Format, lvl.Text)
	}

	// Updating an existing level should not create a duplicate.
	nd.SetLevel(0, "upperRoman", "%1)")
	if len(nd.Levels()) != 1 {
		t.Fatalf("levels = %d, want 1", len(nd.Levels()))
	}
	if nd.Level(0).Format != "upperRoman" {
		t.Fatalf("level format = %q", nd.Level(0).Format)
	}

	nd.SetLevel(1, "lowerLetter", "%2)")
	if len(nd.Levels()) != 2 {
		t.Fatalf("levels = %d, want 2", len(nd.Levels()))
	}
}

func TestNumberingRoundTrip(t *testing.T) {
	d := NewDocument()
	nd := d.EnsureNumberingDefinition(1, 0)
	nd.SetLevel(0, "decimal", "%1.")

	p := d.AddParagraph()
	p.SetNumbering(1, 0)
	p.SetText("First item")

	pkg, err := d.ToOOXML()
	if err != nil {
		t.Fatalf("ToOOXML: %v", err)
	}
	if pkg.Numbering == nil {
		t.Fatal("no numbering part emitted")
	}
	if len(pkg.Numbering.AbstractNums) == 0 {
		t.Fatal("no abstract nums emitted")
	}
	if len(pkg.Numbering.Nums) == 0 {
		t.Fatal("no nums emitted")
	}
	found := false
	for _, n := range pkg.Numbering.Nums {
		if n.ID == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("num with ID 1 not found")
	}
}

func TestTableRemoveRow(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	r1 := tbl.AddRow()
	_ = r1
	r2 := tbl.AddRow()
	tbl.RemoveRow(r1)
	if len(tbl.Rows()) != 1 {
		t.Fatalf("rows = %d, want 1", len(tbl.Rows()))
	}
	// r2's index is now out of bounds; ir() should return nil.
	if r2.ir() != nil {
		t.Fatal("expected r2.ir() to be nil after preceding row removal")
	}
}

func TestTableInsertRowAt(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	tbl.AddRow()
	tbl.AddRow()
	mid := tbl.InsertRowAt(1)
	if len(tbl.Rows()) != 3 {
		t.Fatalf("rows = %d, want 3", len(tbl.Rows()))
	}
	if mid.index != 1 {
		t.Fatalf("inserted row index = %d, want 1", mid.index)
	}
}

func TestTableRowRemoveCell(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	c1 := row.AddCell()
	c2 := row.AddCell()
	row.RemoveCell(c1)
	if len(row.Cells()) != 1 {
		t.Fatalf("cells = %d, want 1", len(row.Cells()))
	}
	// c2's index is now out of bounds; ir() should return nil.
	if c2.ir() != nil {
		t.Fatal("expected c2.ir() to be nil after preceding cell removal")
	}
}

func TestTableRowInsertCellAt(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	row.AddCell()
	row.AddCell()
	mid := row.InsertCellAt(1)
	if len(row.Cells()) != 3 {
		t.Fatalf("cells = %d, want 3", len(row.Cells()))
	}
	if mid.index != 1 {
		t.Fatalf("inserted cell index = %d, want 1", mid.index)
	}
}

func TestTableRowMergeCells(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	row.AddCell()
	row.AddCell()
	row.AddCell()
	row.MergeCells(0, 1)
	if len(row.Cells()) != 2 {
		t.Fatalf("cells = %d, want 2", len(row.Cells()))
	}
	if row.Cells()[0].ir().Format.GridSpan != 2 {
		t.Fatalf("gridSpan = %d, want 2", row.Cells()[0].ir().Format.GridSpan)
	}
}

func TestTableCellVMerge(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	cell.SetVMerge("restart")
	if cell.ir().Format.VMerge != "restart" {
		t.Fatalf("vMerge = %q", cell.ir().Format.VMerge)
	}
}

func TestBodyElements(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	_ = d.AddTable()
	_ = d.AddParagraph()

	elems := d.BodyElements()
	if len(elems) != 3 {
		t.Fatalf("body elements = %d, want 3", len(elems))
	}
	if elems[0].ElementType() != "paragraph" {
		t.Fatalf("elem[0] type = %q, want paragraph", elems[0].ElementType())
	}
	if elems[1].ElementType() != "table" {
		t.Fatalf("elem[1] type = %q, want table", elems[1].ElementType())
	}
	if elems[2].ElementType() != "paragraph" {
		t.Fatalf("elem[2] type = %q, want paragraph", elems[2].ElementType())
	}

	// Verify the returned wrappers are functional
	para, ok := elems[0].(*Paragraph)
	if !ok {
		t.Fatalf("elem[0] not *Paragraph")
	}
	para.SetText("hello")
	if p1.Text() != "hello" {
		t.Fatalf("paragraph text = %q", p1.Text())
	}
}

func TestTableMergeCellsHorizontally(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	for i := 0; i < 4; i++ {
		c := row.AddCell()
		c.SetText(string('a' + byte(i)))
	}
	tbl.MergeCellsHorizontally(0, 1, 3)
	cells := row.Cells()
	if len(cells) != 2 {
		t.Fatalf("cells = %d, want 2", len(cells))
	}
	if cells[0].ir().Format.GridSpan != 0 {
		t.Fatalf("cell[0] gridSpan = %d, want 0 (unset)", cells[0].ir().Format.GridSpan)
	}
	if cells[1].ir().Format.GridSpan != 3 {
		t.Fatalf("cell[1] gridSpan = %d, want 3", cells[1].ir().Format.GridSpan)
	}
}

func TestTableMergeCellsVertically(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	for i := 0; i < 3; i++ {
		row := tbl.AddRow()
		for j := 0; j < 2; j++ {
			c := row.AddCell()
			c.SetText(string('a' + byte(i*2+j)))
		}
	}
	tbl.MergeCellsVertically(0, 0, 2)
	for r := 0; r < 3; r++ {
		cell := tbl.Rows()[r].Cells()[0]
		if r == 0 {
			if cell.ir().Format.VMerge != "restart" {
				t.Fatalf("row %d vMerge = %q, want restart", r, cell.ir().Format.VMerge)
			}
		} else {
			if cell.ir().Format.VMerge != "continue" {
				t.Fatalf("row %d vMerge = %q, want continue", r, cell.ir().Format.VMerge)
			}
			if cell.Text() != "" {
				t.Fatalf("row %d cell text = %q, want empty", r, cell.Text())
			}
		}
	}
}

func TestRunAddImage(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.AddImage([]byte("fake-png-data"), "image/png", 914400, 457200)
	if r.ir().Kind != "image" {
		t.Fatalf("run kind = %q, want image", r.ir().Kind)
	}
	if r.ir().Image == nil {
		t.Fatal("image not set")
	}
	if r.ir().Image.MIMEType != "image/png" {
		t.Fatalf("mime = %q", r.ir().Image.MIMEType)
	}
	if r.ir().Image.Width != 914400 || r.ir().Image.Height != 457200 {
		t.Fatalf("size = %dx%d", r.ir().Image.Width, r.ir().Image.Height)
	}
}

func TestRunHyperlink(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("click here")
	r.SetHyperlink("https://example.com")
	if r.ir().Kind != "hyperlink" {
		t.Fatalf("run kind = %q, want hyperlink", r.ir().Kind)
	}
	if r.Hyperlink() != "https://example.com" {
		t.Fatalf("hyperlink = %q", r.Hyperlink())
	}

	r.SetAnchorBookmark("_bookmark1")
	if r.AnchorBookmark() != "_bookmark1" {
		t.Fatalf("anchor = %q", r.AnchorBookmark())
	}
}

func TestParagraphBookmarks(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.AddBookmarkStart("bm1")
	p.AddBookmarkStart("bm2")
	if len(p.BookmarkStarts()) != 2 {
		t.Fatalf("bookmarks = %d, want 2", len(p.BookmarkStarts()))
	}
	if p.BookmarkStarts()[0] != "bm1" || p.BookmarkStarts()[1] != "bm2" {
		t.Fatalf("bookmarks = %v", p.BookmarkStarts())
	}

	p.RemoveBookmarkStart("bm1")
	if len(p.BookmarkStarts()) != 1 || p.BookmarkStarts()[0] != "bm2" {
		t.Fatalf("after remove bookmarks = %v", p.BookmarkStarts())
	}

	// Removing non-existent is no-op
	p.RemoveBookmarkStart("bm3")
	if len(p.BookmarkStarts()) != 1 {
		t.Fatalf("after noop remove bookmarks = %d", len(p.BookmarkStarts()))
	}
}

func TestAutoIDManagement(t *testing.T) {
	irDoc := &wordir.Document{
		Footnotes: map[int]wordir.Story{1: {}, 5: {}},
		Endnotes:  map[int]wordir.Story{2: {}, 3: {}},
		Comments:  map[int]wordir.Story{10: {}},
	}
	d := OpenIR(irDoc)
	if d.NextFootnoteID() != 6 {
		t.Fatalf("NextFootnoteID = %d, want 6", d.NextFootnoteID())
	}
	if d.NextEndnoteID() != 4 {
		t.Fatalf("NextEndnoteID = %d, want 4", d.NextEndnoteID())
	}
	if d.NextCommentID() != 11 {
		t.Fatalf("NextCommentID = %d, want 11", d.NextCommentID())
	}

	// After adding a new footnote, next ID should increment
	d.AddFootnote(d.NextFootnoteID())
	if d.NextFootnoteID() != 7 {
		t.Fatalf("NextFootnoteID after add = %d, want 7", d.NextFootnoteID())
	}
}

func TestStoryGettersAndRemovers(t *testing.T) {
	d := NewDocument()

	// Add stories
	d.AddHeader("default").AddParagraph().SetText("hdr")
	d.AddFooter("default").AddParagraph().SetText("ftr")
	d.AddFootnote(1).AddParagraph().SetText("fn")
	d.AddEndnote(1).AddParagraph().SetText("en")
	d.AddComment(1).AddParagraph().SetText("cm")

	// Verify getters
	if h := d.Header("default"); h == nil {
		t.Fatal("expected header")
	}
	if f := d.Footer("default"); f == nil {
		t.Fatal("expected footer")
	}
	if fn := d.Footnote(1); fn == nil {
		t.Fatal("expected footnote")
	}
	if en := d.Endnote(1); en == nil {
		t.Fatal("expected endnote")
	}
	if c := d.Comment(1); c == nil {
		t.Fatal("expected comment")
	}

	// Verify nil for missing
	if d.Header("missing") != nil {
		t.Fatal("expected nil for missing header")
	}
	if d.Footnote(999) != nil {
		t.Fatal("expected nil for missing footnote")
	}

	// Remove stories
	d.RemoveHeader("default")
	d.RemoveFooter("default")
	d.RemoveFootnote(1)
	d.RemoveEndnote(1)
	d.RemoveComment(1)

	// Verify removal
	if d.Header("default") != nil {
		t.Fatal("expected header removed")
	}
	if d.Footer("default") != nil {
		t.Fatal("expected footer removed")
	}
	if d.Footnote(1) != nil {
		t.Fatal("expected footnote removed")
	}
	if d.Endnote(1) != nil {
		t.Fatal("expected endnote removed")
	}
	if d.Comment(1) != nil {
		t.Fatal("expected comment removed")
	}

	// Verify IR maps are clean
	d.flushStoryPtrs()
	if len(d.IR.Headers) != 0 {
		t.Fatalf("headers = %d, want 0", len(d.IR.Headers))
	}
	if len(d.IR.Footers) != 0 {
		t.Fatalf("footers = %d, want 0", len(d.IR.Footers))
	}
	if len(d.IR.Footnotes) != 0 {
		t.Fatalf("footnotes = %d, want 0", len(d.IR.Footnotes))
	}
	if len(d.IR.Endnotes) != 0 {
		t.Fatalf("endnotes = %d, want 0", len(d.IR.Endnotes))
	}
	if len(d.IR.Comments) != 0 {
		t.Fatalf("comments = %d, want 0", len(d.IR.Comments))
	}
}

func TestEnumerateFields(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.ir.Runs = []wordir.Run{
		{Text: "before", FieldKind: ""},
		{Text: "1", FieldKind: "PAGE", FieldInstr: "PAGE"},
		{Text: "after", FieldKind: ""},
	}

	// Add a field in a header
	h := d.AddHeader("default")
	hp := h.AddParagraph()
	hp.ir.Runs = []wordir.Run{
		{Text: "date", FieldKind: "DATE", FieldInstr: "DATE \\@ \"yyyy-MM-dd\""},
	}

	fields := d.EnumerateFields()
	if len(fields) != 2 {
		t.Fatalf("fields = %d, want 2", len(fields))
	}
	if fields[0].Kind != "PAGE" || fields[0].Location != "body" {
		t.Fatalf("field[0] = %+v, want PAGE in body", fields[0])
	}
	if fields[1].Kind != "DATE" || fields[1].Location != "header" {
		t.Fatalf("field[1] = %+v, want DATE in header", fields[1])
	}
}

func TestStyleEnumeration(t *testing.T) {
	d := NewDocument()
	d.EnsureParagraphStyle("Heading1", "Heading 1")
	d.EnsureParagraphStyle("Normal", "Normal")
	d.EnsureCharacterStyle("Emphasis", "Emphasis")
	d.EnsureTableStyle("TableGrid", "Table Grid")

	if len(d.ParagraphStyles()) != 2 {
		t.Fatalf("paragraph styles = %d, want 2", len(d.ParagraphStyles()))
	}
	if len(d.CharacterStyles()) != 1 {
		t.Fatalf("character styles = %d, want 1", len(d.CharacterStyles()))
	}
	if len(d.TableStyles()) != 1 {
		t.Fatalf("table styles = %d, want 1", len(d.TableStyles()))
	}
}

func TestClearBorders(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.SetBorder("top", "single", 1.0, "000000")
	if p.ir.Format.Borders == nil {
		t.Fatal("expected border set")
	}
	p.ClearBorders()
	if p.ir.Format.Borders != nil {
		t.Fatal("expected borders cleared")
	}

	r := p.AddRun()
	r.SetBorder("single", "000000", 1.0, 2.0)
	r.ClearBorder()
	if r.ir().Format.Borders != nil {
		t.Fatal("expected run border cleared")
	}

	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	cell.SetBorderTop("single", 1.0, "000000")
	cell.ClearBorders()
	if cell.ir().Format.Borders != nil {
		t.Fatal("expected cell borders cleared")
	}

	s := d.AddSection()
	s.SetPageBorders("single", "000000", 1.0, "allPages")
	s.ClearPageBorders()
	if s.ir().PageBorders != nil {
		t.Fatal("expected page borders cleared")
	}
}

func TestCellRemoveParagraph(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	p1 := cell.AddParagraph()
	p1.AddRun().SetText("first")
	p2 := cell.AddParagraph()
	p2.AddRun().SetText("second")

	if len(cell.Paragraphs()) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(cell.Paragraphs()))
	}

	cell.RemoveParagraph(p1)
	if len(cell.Paragraphs()) != 1 {
		t.Fatalf("paragraphs after remove = %d, want 1", len(cell.Paragraphs()))
	}
	if cell.Paragraphs()[0].Text() != "second" {
		t.Fatalf("remaining text = %q, want second", cell.Paragraphs()[0].Text())
	}
}

func TestSectionHeaderFooterRefs(t *testing.T) {
	d := NewDocument()
	s := d.AddSection()
	s.SetHeaderRef("default", "header1")
	s.SetHeaderRef("first", "header2")
	s.SetFooterRef("default", "footer1")

	if len(s.HeaderRefs()) != 2 {
		t.Fatalf("header refs = %d, want 2", len(s.HeaderRefs()))
	}
	if len(s.FooterRefs()) != 1 {
		t.Fatalf("footer refs = %d, want 1", len(s.FooterRefs()))
	}

	s.RemoveHeaderRef("default")
	if len(s.HeaderRefs()) != 1 {
		t.Fatalf("header refs after remove = %d, want 1", len(s.HeaderRefs()))
	}
	if s.HeaderRefs()[0].Type != "first" {
		t.Fatalf("remaining header type = %q, want first", s.HeaderRefs()[0].Type)
	}

	s.RemoveFooterRef("default")
	if len(s.FooterRefs()) != 0 {
		t.Fatalf("footer refs after remove = %d, want 0", len(s.FooterRefs()))
	}
}

func TestContentControl(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("sdt run")
	rcc := &wordir.ContentControl{ID: 2, Tag: "tag2"}
	r.SetContentControl(rcc)
	if r.ContentControl().Tag != "tag2" {
		t.Fatalf("run CC tag = %q", r.ContentControl().Tag)
	}
	r.SetContentControl(nil)
	if r.ContentControl() != nil {
		t.Fatal("expected nil content control on run")
	}
}

func TestRunFields(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()

	r1 := p.AddRun()
	r1.SetPageNumber()
	if r1.FieldInstr() != "PAGE" {
		t.Fatalf("page field = %q", r1.FieldInstr())
	}

	r2 := p.AddRun()
	r2.SetNumPages()
	if r2.FieldInstr() != "NUMPAGES" {
		t.Fatalf("numpages field = %q", r2.FieldInstr())
	}

	r3 := p.AddRun()
	r3.SetDocProperty("Title")
	if r3.FieldInstr() != "DOCPROPERTY Title" {
		t.Fatalf("docproperty field = %q", r3.FieldInstr())
	}
}

func TestFindText(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("hello world")
	p2 := d.AddParagraph()
	p2.SetText("goodbye world")
	p3 := d.AddParagraph()
	p3.SetText("no match")

	found := d.FindText("world")
	if len(found) != 2 {
		t.Fatalf("found = %d, want 2", len(found))
	}
	if found[0].Text() != "hello world" {
		t.Fatalf("found[0] = %q", found[0].Text())
	}
	if found[1].Text() != "goodbye world" {
		t.Fatalf("found[1] = %q", found[1].Text())
	}
}

func TestFindTextRegex(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("Invoice 2026")
	p2 := d.AddParagraph()
	p2.SetText("Invoice 2025")
	p3 := d.AddParagraph()
	p3.SetText("Memo")

	found, err := d.FindTextRegex(`Invoice 202[56]`)
	if err != nil {
		t.Fatalf("FindTextRegex: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("found = %d, want 2", len(found))
	}
	if found[0].Text() != "Invoice 2026" {
		t.Fatalf("found[0] = %q", found[0].Text())
	}
	if found[1].Text() != "Invoice 2025" {
		t.Fatalf("found[1] = %q", found[1].Text())
	}
}

func TestFindRuns(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetStyle("Heading1")
	r1 := p1.AddRun()
	r1.SetText("Hello")
	r1.SetBold(true)
	r1.SetColor("FF0000")

	p2 := d.AddParagraph()
	r2 := p2.AddRun()
	r2.SetText("Help")
	r2.SetBold(true)
	r2.SetColor("00FF00")

	bold := true
	found, err := d.FindRuns(RunSearchOptions{
		TextPattern:      `Hel.`,
		ParagraphStyleID: "Heading1",
		Bold:             &bold,
		Color:            "FF0000",
	})
	if err != nil {
		t.Fatalf("FindRuns: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("found = %d, want 1", len(found))
	}
	if found[0].Text() != "Hello" {
		t.Fatalf("found[0] = %q", found[0].Text())
	}
}

func TestReplaceAll(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("hello world")
	p2 := d.AddParagraph()
	p2.SetText("world cup")

	n := d.ReplaceAll("world", "earth")
	if n != 2 {
		t.Fatalf("replacements = %d, want 2", n)
	}
	if p1.Text() != "hello earth" {
		t.Fatalf("p1 = %q", p1.Text())
	}
	if p2.Text() != "earth cup" {
		t.Fatalf("p2 = %q", p2.Text())
	}
}

func TestReplaceAllCrossRun(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("hel")
	r2 := p.AddRun()
	r2.SetText("lo wo")
	r3 := p.AddRun()
	r3.SetText("rld")

	// "hello world" is split across 3 runs
	n := p.ReplaceAll("hello world", "hi")
	if n != 1 {
		t.Fatalf("replacements = %d, want 1", n)
	}
	if p.Text() != "hi" {
		t.Fatalf("p text = %q, want hi", p.Text())
	}
}

func TestReplaceAllMultipleCrossRun(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("ab")
	r2 := p.AddRun()
	r2.SetText("cd")
	r3 := p.AddRun()
	r3.SetText("ab")
	r4 := p.AddRun()
	r4.SetText("cd")

	// "abcd" appears twice, each spanning 2 runs
	n := p.ReplaceAll("abcd", "XY")
	if n != 2 {
		t.Fatalf("replacements = %d, want 2", n)
	}
	if p.Text() != "XYXY" {
		t.Fatalf("p text = %q, want XYXY", p.Text())
	}
}

func TestValidate(t *testing.T) {
	d := NewDocument()
	issues := d.Validate()
	// Empty document should have at least "no sections" warning.
	if len(issues) == 0 {
		t.Fatal("expected validation issues for empty document")
	}

	p := d.AddSection()
	_ = p
	p2 := d.AddParagraph()
	p2.SetStyle("MissingStyle")
	p2.SetNumbering(99, 0)
	p3 := d.AddParagraph()
	r := p3.AddRun()
	r.SetHyperlink("")

	issues = d.Validate()
	if len(issues) == 0 {
		t.Fatal("expected validation issues")
	}
	// Should contain: missing style, missing numbering, empty hyperlink, no sections
	// (no sections is fixed by AddSection, so we should have 3 issues)
	found := 0
	for _, issue := range issues {
		if contains(issue, "missing style") || contains(issue, "MissingStyle") {
			found++
		}
		if contains(issue, "numbering") || contains(issue, "99") {
			found++
		}
		if contains(issue, "hyperlink") {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("expected at least 2 specific issues, got %d (issues: %v)", found, issues)
	}
}

func TestValidateAccessibilityChecks(t *testing.T) {
	d := NewDocument()
	d.AddSection()

	p := d.AddParagraph()
	p.ir.Runs = append(p.ir.Runs, wordir.Run{
		Image: &wordir.Image{
			Data:     []byte{1, 2, 3},
			MIMEType: "image/png",
		},
	})

	tbl := d.AddTable()
	row := tbl.AddRow()
	row.AddCell()

	issues := d.Validate()
	var missingAlt, emptyCell bool
	for _, issue := range issues {
		if contains(issue, "image has no alt text or title") {
			missingAlt = true
		}
		if contains(issue, "cell") && contains(issue, "empty") {
			emptyCell = true
		}
	}
	if !missingAlt {
		t.Fatalf("expected missing image alt-text issue, got %v", issues)
	}
	if !emptyCell {
		t.Fatalf("expected empty table-cell issue, got %v", issues)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestDocumentAddStoryField(t *testing.T) {
	d := NewDocument()
	sf := d.AddStoryField("TOC", `TOC \o "1-3" \h \z \u`)
	if sf == nil {
		t.Fatal("nil story field")
	}
	if len(d.IR.Fields) != 1 {
		t.Fatalf("fields = %d, want 1", len(d.IR.Fields))
	}
	if sf.Kind() != "TOC" {
		t.Fatalf("kind = %q, want TOC", sf.Kind())
	}
	if sf.Instr() != `TOC \o "1-3" \h \z \u` {
		t.Fatalf("instr = %q", sf.Instr())
	}

	sf.SetArg(`TOC \o "1-3"`)
	sf.SetAnchor("anchor1")
	sf.SetTarget("#anchor1")
	if sf.Arg() != `TOC \o "1-3"` {
		t.Fatalf("arg = %q", sf.Arg())
	}
	if sf.Anchor() != "anchor1" {
		t.Fatalf("anchor = %q", sf.Anchor())
	}
	if sf.Target() != "#anchor1" {
		t.Fatalf("target = %q", sf.Target())
	}
}

func TestDocumentStoryFields(t *testing.T) {
	d := NewDocument()
	if len(d.StoryFields()) != 0 {
		t.Fatalf("story fields = %d, want 0", len(d.StoryFields()))
	}
	d.AddStoryField("TOC", "TOC \\h")
	d.AddStoryField("INDEX", "INDEX")
	if len(d.StoryFields()) != 2 {
		t.Fatalf("story fields = %d, want 2", len(d.StoryFields()))
	}
	if d.StoryFields()[0].Kind() != "TOC" {
		t.Fatalf("kind = %q, want TOC", d.StoryFields()[0].Kind())
	}
	if d.StoryFields()[1].Kind() != "INDEX" {
		t.Fatalf("kind = %q, want INDEX", d.StoryFields()[1].Kind())
	}
}

func TestStoryFieldAddInstrRun(t *testing.T) {
	d := NewDocument()
	sf := d.AddStoryField("TOC", "TOC")
	r := sf.AddInstrRun()
	if r == nil {
		t.Fatal("nil instr run")
	}
	r.SetText("heading")
	if len(sf.InstrRuns()) != 1 {
		t.Fatalf("instr runs = %d, want 1", len(sf.InstrRuns()))
	}
	if sf.InstrRuns()[0].Text() != "heading" {
		t.Fatalf("text = %q", sf.InstrRuns()[0].Text())
	}
}

func TestStoryFieldAddResultParagraph(t *testing.T) {
	d := NewDocument()
	sf := d.AddStoryField("TOC", "TOC \\h")
	p := sf.AddResultParagraph()
	if p == nil {
		t.Fatal("nil paragraph")
	}
	p.SetText("Chapter 1")

	if sf.ResultBlockCount() != 1 {
		t.Fatalf("result blocks = %d, want 1", sf.ResultBlockCount())
	}
	if len(d.IR.Blocks) != 1 {
		t.Fatalf("document blocks = %d, want 1", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].FieldKind != "TOC" {
		t.Fatalf("block field kind = %q, want TOC", d.IR.Blocks[0].FieldKind)
	}
	if d.IR.Blocks[0].FieldIndex != 0 {
		t.Fatalf("block field index = %d, want 0", d.IR.Blocks[0].FieldIndex)
	}
	if d.IR.Blocks[0].Paragraph == nil || d.IR.Blocks[0].Paragraph.Text != "Chapter 1" {
		t.Fatal("block paragraph text mismatch")
	}
}

func TestStoryFieldAddResultTable(t *testing.T) {
	d := NewDocument()
	sf := d.AddStoryField("TOC", "TOC \\h")
	tbl := sf.AddResultTable()
	if tbl == nil {
		t.Fatal("nil table")
	}
	if sf.ResultBlockCount() != 1 {
		t.Fatalf("result blocks = %d, want 1", sf.ResultBlockCount())
	}
	if len(d.IR.Blocks) != 1 {
		t.Fatalf("document blocks = %d, want 1", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].FieldKind != "TOC" {
		t.Fatalf("block field kind = %q, want TOC", d.IR.Blocks[0].FieldKind)
	}
	if d.IR.Blocks[0].Kind != "table" {
		t.Fatalf("block kind = %q, want table", d.IR.Blocks[0].Kind)
	}
}

func TestStoryFieldMultipleResultBlocks(t *testing.T) {
	d := NewDocument()
	sf := d.AddStoryField("TOC", "TOC \\h")
	p1 := sf.AddResultParagraph()
	p1.SetText("Entry 1")
	p2 := sf.AddResultParagraph()
	p2.SetText("Entry 2")

	if sf.ResultBlockCount() != 2 {
		t.Fatalf("result blocks = %d, want 2", sf.ResultBlockCount())
	}
	if len(d.IR.Blocks) != 2 {
		t.Fatalf("document blocks = %d, want 2", len(d.IR.Blocks))
	}
	if d.IR.Blocks[0].FieldIndex != 0 || d.IR.Blocks[1].FieldIndex != 0 {
		t.Fatalf("field indices = %d, %d, want 0, 0", d.IR.Blocks[0].FieldIndex, d.IR.Blocks[1].FieldIndex)
	}
}

func TestStoryEditorAddStoryField(t *testing.T) {
	d := NewDocument()
	header := d.AddHeader("default")
	sf := header.AddStoryField("TOC", "TOC \\h")
	if sf == nil {
		t.Fatal("nil story field")
	}
	if len(header.StoryFields()) != 1 {
		t.Fatalf("header fields = %d, want 1", len(header.StoryFields()))
	}
	p := sf.AddResultParagraph()
	if p == nil {
		t.Fatal("nil paragraph")
	}
	p.SetText("Header TOC")

	// Flush story pointers back to IR so we can inspect.
	d.flushStoryPtrs()
	story := d.IR.Headers["default"]
	if len(story.Fields) != 1 {
		t.Fatalf("ir fields = %d, want 1", len(story.Fields))
	}
	if len(story.Blocks) != 1 {
		t.Fatalf("ir blocks = %d, want 1", len(story.Blocks))
	}
	if story.Blocks[0].FieldKind != "TOC" {
		t.Fatalf("block field kind = %q, want TOC", story.Blocks[0].FieldKind)
	}
}

func TestStoryFieldNilSafety(t *testing.T) {
	var sf *StoryField
	if sf.Kind() != "" {
		t.Fatalf("kind = %q, want empty", sf.Kind())
	}
	if sf.Instr() != "" {
		t.Fatalf("instr = %q, want empty", sf.Instr())
	}
	if sf.AddInstrRun() != nil {
		t.Fatal("expected nil instr run")
	}
	if sf.AddResultParagraph() != nil {
		t.Fatal("expected nil paragraph")
	}
	if sf.AddResultTable() != nil {
		t.Fatal("expected nil table")
	}
	if sf.ResultBlockCount() != 0 {
		t.Fatalf("count = %d, want 0", sf.ResultBlockCount())
	}
}

func TestParagraphSetLineSpacing(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.SetLineSpacing(360, "auto")
	if p.ir.Format.LineSpacing == nil || *p.ir.Format.LineSpacing != 360 {
		t.Fatalf("line spacing = %v, want 360", p.ir.Format.LineSpacing)
	}
	if p.ir.Format.LineRule != "auto" {
		t.Fatalf("line rule = %q, want auto", p.ir.Format.LineRule)
	}
}

func TestParagraphSetBidi(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.SetBidi(true)
	if !p.ir.Format.Bidi {
		t.Fatal("expected bidi = true")
	}
}

func TestRunSetShadowOutlineEmbossImprint(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetShadow(true)
	r.SetOutline(true)
	r.SetEmboss(true)
	r.SetImprint(true)
	f := p.ir.Runs[0].Format
	if f.Shadow == nil || !*f.Shadow {
		t.Fatal("expected shadow")
	}
	if f.Outline == nil || !*f.Outline {
		t.Fatal("expected outline")
	}
	if f.Emboss == nil || !*f.Emboss {
		t.Fatal("expected emboss")
	}
	if f.Imprint == nil || !*f.Imprint {
		t.Fatal("expected imprint")
	}
}

func TestRunSetFitTextWidth(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()

	r.SetFitTextWidth(7200)
	if got := p.ir.Runs[0].Format.FitTextWidth; got == nil || *got != 7200 {
		t.Fatalf("fit text width = %v, want 7200", got)
	}

	r.SetFitTextWidth(0)
	if p.ir.Runs[0].Format.FitTextWidth != nil {
		t.Fatalf("fit text width = %v, want nil after clear", p.ir.Runs[0].Format.FitTextWidth)
	}
}

func TestRunSetTextShadow(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetTextShadow("FF0000", 2.5, 3.0)
	f := p.ir.Runs[0].Format
	if f.TextShadow == nil {
		t.Fatal("nil text shadow")
	}
	if f.TextShadow.Color != "FF0000" {
		t.Fatalf("color = %q, want FF0000", f.TextShadow.Color)
	}
	if f.TextShadow.Blur != 2.5 {
		t.Fatalf("blur = %v, want 2.5", f.TextShadow.Blur)
	}
	if f.TextShadow.Distance != 3.0 {
		t.Fatalf("distance = %v, want 3.0", f.TextShadow.Distance)
	}
}

func TestRunSetTextOutline(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetTextOutline("0000FF", 1.5)
	f := p.ir.Runs[0].Format
	if f.TextOutline == nil {
		t.Fatal("nil text outline")
	}
	if f.TextOutline.Color != "0000FF" {
		t.Fatalf("color = %q, want 0000FF", f.TextOutline.Color)
	}
	if f.TextOutline.Width != 1.5 {
		t.Fatalf("width = %v, want 1.5", f.TextOutline.Width)
	}
}

func TestTableSetFloatProps(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	props := &wordir.TableFloatProps{
		HorzAnchor: "page",
		VertAnchor: "margin",
		X:          1440,
		Y:          720,
	}
	tbl.SetFloatProps(props)
	if tbl.ir.Format.FloatProps == nil {
		t.Fatal("nil float props")
	}
	if tbl.ir.Format.FloatProps.HorzAnchor != "page" {
		t.Fatalf("horz anchor = %q, want page", tbl.ir.Format.FloatProps.HorzAnchor)
	}
	if tbl.ir.Format.FloatProps.X != 1440 {
		t.Fatalf("x = %d, want 1440", tbl.ir.Format.FloatProps.X)
	}
	// Clearing float props.
	tbl.SetFloatProps(nil)
	if tbl.ir.Format.FloatProps != nil {
		t.Fatal("expected nil float props after clear")
	}
}

// === New edit methods tests ===

func TestTableRowExtendedSetters(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	row.SetGridBefore(2)
	row.SetGridAfter(3)
	row.SetJustification("center")
	row.SetConditionalFormat("firstRow")
	row.SetHidden(true)

	ir := row.ir()
	if ir.Format.GridBefore != 2 {
		t.Fatalf("gridBefore = %d, want 2", ir.Format.GridBefore)
	}
	if ir.Format.GridAfter != 3 {
		t.Fatalf("gridAfter = %d, want 3", ir.Format.GridAfter)
	}
	if ir.Format.Justification != "center" {
		t.Fatalf("justification = %q, want center", ir.Format.Justification)
	}
	if ir.Format.Conditional != "firstRow" {
		t.Fatalf("conditional = %q, want firstRow", ir.Format.Conditional)
	}
	if !ir.Format.Hidden {
		t.Fatal("expected hidden = true")
	}
}

func TestTableCellExtendedSetters(t *testing.T) {
	d := NewDocument()
	tbl := d.AddTable()
	row := tbl.AddRow()
	cell := row.AddCell()
	cell.SetNoWrap(true)
	cell.SetHideMark(true)
	cell.SetCellPadding(10, 20, 30, 40)
	cell.SetBorder("top", "double", 2.0, "FF0000")

	ir := cell.ir()
	if !ir.Format.NoWrap {
		t.Fatal("expected noWrap = true")
	}
	if !ir.Format.HideMark {
		t.Fatal("expected hideMark = true")
	}
	if ir.Format.CellPadTop != 10 || ir.Format.CellPadLeft != 30 || ir.Format.CellPadBottom != 20 || ir.Format.CellPadRight != 40 {
		t.Fatalf("cell padding mismatch: top=%d bottom=%d left=%d right=%d", ir.Format.CellPadTop, ir.Format.CellPadBottom, ir.Format.CellPadLeft, ir.Format.CellPadRight)
	}
	if ir.Format.Borders == nil || ir.Format.Borders.Top == nil || ir.Format.Borders.Top.Val != "double" {
		t.Fatal("cell top border not set")
	}
}

func TestSectionExtendedSetters(t *testing.T) {
	d := NewDocument()
	s := d.AddSection()
	s.SetPaperSource(15, 16)
	s.SetFootnoteProps("belowText", "lowerRoman", 5, "eachSect")
	s.SetEndnoteProps("upperRoman", 10, "eachSect")
	s.SetPageBorderTop("single", 1.5, "000000")
	s.SetPageBorderBottom("double", 2.0, "FF0000")
	s.SetPageBorderLeft("dashed", 1.0, "00FF00")
	s.SetPageBorderRight("dotted", 0.5, "0000FF")
	s.SetRtlGutter(true)
	s.SetNoEndnote(true)

	ir := s.ir()
	if ir.PaperSource == nil || ir.PaperSource.First != 15 || ir.PaperSource.Other != 16 {
		t.Fatal("paper source not set")
	}
	if ir.FootnotePos != "belowText" || ir.FootnoteNumFmt != "lowerRoman" || ir.FootnoteNumStart != 5 || ir.FootnoteRestart != "eachSect" {
		t.Fatal("footnote props not set")
	}
	if ir.EndnoteNumFmt != "upperRoman" || ir.EndnoteNumStart != 10 || ir.EndnoteRestart != "eachSect" {
		t.Fatal("endnote props not set")
	}
	if ir.PageBorders == nil || ir.PageBorders.Top == nil || ir.PageBorders.Bottom.Val != "double" {
		t.Fatal("page borders not set")
	}
	if !ir.PageMargin.RtlGutter {
		t.Fatal("expected RTL gutter")
	}
	if ir.EndnoteAtSectionEnd {
		t.Fatal("expected EndnoteAtSectionEnd = false when SetNoEndnote(true)")
	}
}

func TestDocumentExtendedSetters(t *testing.T) {
	d := NewDocument()
	d.SetCompatibilityMode("15")
	d.SetProofState("clean", "dirty")
	d.SetThemeFontLang("en-US", "ja-JP", "ar-SA")
	d.SetFootnoteProperties("belowText", "decimal", 1, "continuous")
	d.SetEndnoteProperties("lowerRoman", 2, "eachSect")
	d.SetVariable("Version", "1.0")
	d.AddCustomProperty("CustomProp", "CustomValue")
	d.AddCustomXML(" {store-1} ", []byte(`<root><value>Alice</value></root>`))

	if d.IR.CompatibilityMode != "15" {
		t.Fatalf("compatibilityMode = %q, want 15", d.IR.CompatibilityMode)
	}
	if d.IR.ProofState == nil || d.IR.ProofState.Spelling != "clean" || d.IR.ProofState.Grammar != "dirty" {
		t.Fatal("proof state not set")
	}
	if d.IR.ThemeFontLang == nil || d.IR.ThemeFontLang.Val != "en-US" {
		t.Fatal("theme font lang not set")
	}
	if d.IR.FootnoteNumberFormat != "decimal" || d.IR.FootnoteNumberStart != 1 || d.IR.FootnoteRestart != "continuous" {
		t.Fatal("footnote properties not set")
	}
	if d.IR.EndnoteNumberFormat != "lowerRoman" || d.IR.EndnoteNumberStart != 2 || d.IR.EndnoteRestart != "eachSect" {
		t.Fatal("endnote properties not set")
	}
	if d.IR.DocumentVariables["Version"] != "1.0" {
		t.Fatal("document variable not set")
	}
	if d.IR.CustomProperties["CustomProp"] != "CustomValue" {
		t.Fatal("custom property not set")
	}
	if got := string(d.IR.CustomXMLParts["{STORE-1}"]); got != `<root><value>Alice</value></root>` {
		t.Fatalf("custom XML part = %q, want payload", got)
	}
}

func TestDocumentTableStyle(t *testing.T) {
	d := NewDocument()
	ts := d.CreateTableStyle("TableGrid", "Table Grid")
	if ts == nil {
		t.Fatal("nil table style")
	}
	if ts.ID() != "TableGrid" || ts.Name() != "Table Grid" {
		t.Fatalf("table style id=%q name=%q", ts.ID(), ts.Name())
	}

	ts.SetBasedOn("TableNormal")
	if d.IR.Styles["TableGrid"].BasedOn != "TableNormal" {
		t.Fatal("basedOn not set")
	}

	// EnsureTableStyle should return existing style and update its name.
	ts2 := d.EnsureTableStyle("TableGrid", "Updated Name")
	if ts2.Name() != "Updated Name" {
		t.Fatalf("expected updated style name, got %q", ts2.Name())
	}

	styles := d.TableStyles()
	if len(styles) != 1 {
		t.Fatalf("table styles = %d, want 1", len(styles))
	}
}

func TestNumberingDefSetParagraphStyle(t *testing.T) {
	d := NewDocument()
	nd := d.EnsureNumberingDefinition(1, 10)

	nd.SetParagraphStyle(2, "Heading2")

	lvl := nd.Level(2)
	if lvl == nil {
		t.Fatal("expected level 2")
	}
	if lvl.StyleID != "Heading2" {
		t.Fatalf("styleID = %q, want Heading2", lvl.StyleID)
	}
}

func TestRunExtendedSetters(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()

	r.SetRTL(true)
	r.SetBorder("single", "000000", 1.0, 2.0)
	r.SetEastAsianLayout(true, false, true)
	r.SetTextFill("FF0000")
	r.SetTextGlow("00FF00", 3.0)
	r.SetTextSoftEdge(2.5)
	r.SetTextReflection(0.5, 1.0, 2.0, 0.3)
	r.SetLigatures("standard")
	r.SetNumberSpacing("proportional")
	r.SetStylisticSets([]int{1, 3, 5})
	r.SetComplexScript(true)

	f := p.ir.Runs[0].Format
	if f.RTL == nil || !*f.RTL {
		t.Fatal("RTL not set")
	}
	if f.Borders == nil || f.Borders.Top == nil || f.Borders.Top.SpacePt != 2.0 {
		t.Fatal("run border not set")
	}
	if f.FELayout == nil || !f.FELayout.CombineCharacters || f.FELayout.Vertical || !f.FELayout.VerticalCompressed {
		t.Fatal("FE layout not set")
	}
	if sf, ok := f.TextFill.(drawing.SolidFill); !ok || sf.Color != "FF0000" {
		t.Fatal("text fill not set")
	}
	if f.TextGlow == nil || f.TextGlow.Color != "00FF00" {
		t.Fatal("text glow not set")
	}
	if f.TextSoftEdge == nil || f.TextSoftEdge.Radius != 2.5 {
		t.Fatal("text soft edge not set")
	}
	if f.TextReflection == nil || f.TextReflection.Alpha != 0.5 {
		t.Fatal("text reflection not set")
	}
	if f.Ligatures != "standard" {
		t.Fatalf("ligatures = %q, want standard", f.Ligatures)
	}
	if f.NumberSpacing != "proportional" {
		t.Fatalf("numberSpacing = %q, want proportional", f.NumberSpacing)
	}
	if len(f.StylisticSets) != 3 || f.StylisticSets[0] != 1 {
		t.Fatal("stylistic sets not set")
	}
	if f.ComplexScript == nil || !*f.ComplexScript {
		t.Fatal("complex script not set")
	}
}

func TestParagraphExtendedSetters(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.SetSuppressLineNumbers(true)
	p.SetSnapToGrid(true)
	p.SetKinsoku(true)
	p.SetWordWrap(false)
	p.SetOverflowPunct(true)
	p.SetTopLinePunct(true)
	p.SetAutoSpaceDE(true)
	p.SetAutoSpaceDN(true)
	p.SetFontAlign("center")

	f := p.ir.Format
	if !f.NoLineNumber {
		t.Fatal("noLineNumber not set")
	}
	if !f.SnapToGrid {
		t.Fatal("snapToGrid not set")
	}
	if !f.Kinsoku {
		t.Fatal("kinsoku not set")
	}
	if f.WordWrap == nil || *f.WordWrap {
		t.Fatal("wordWrap not set to false")
	}
	if !f.OverflowPunct {
		t.Fatal("overflowPunct not set")
	}
	if !f.TopLinePunct {
		t.Fatal("topLinePunct not set")
	}
	if !f.AutoSpaceDE {
		t.Fatal("autoSpaceDE not set")
	}
	if !f.AutoSpaceDN {
		t.Fatal("autoSpaceDN not set")
	}
	if f.FontAlign != "center" {
		t.Fatalf("fontAlign = %q, want center", f.FontAlign)
	}
}

func TestParagraphSetFrameProps(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	props := &wordir.FrameProps{
		W:        1440,
		H:        720,
		X:        360,
		Y:        180,
		HAnchor:  "page",
		VAnchor:  "margin",
		WrapType: "around",
	}
	p.SetFrameProps(props)
	if p.ir.Frame == nil {
		t.Fatal("frame not set")
	}
	if p.ir.Frame.W != 1440 {
		t.Fatalf("frame width = %d, want 1440", p.ir.Frame.W)
	}
	if p.ir.Frame.WrapType != "around" {
		t.Fatalf("frame wrap = %q, want around", p.ir.Frame.WrapType)
	}

	p.SetFrameProps(nil)
	if p.ir.Frame != nil {
		t.Fatal("expected nil frame after clear")
	}
}

func TestNumberingDefExtendedSetters(t *testing.T) {
	d := NewDocument()
	nd := d.EnsureNumberingDefinition(1, 0)
	nd.SetLevel(0, "decimal", "%1.")
	nd.SetStart(0, 5)
	nd.SetLeftIndent(0, 720)

	lvl := nd.Level(0)
	if lvl == nil {
		t.Fatal("nil level")
	}
	if lvl.Start != 5 {
		t.Fatalf("start = %d, want 5", lvl.Start)
	}
	if lvl.IndentLeft != 720 {
		t.Fatalf("indentLeft = %d, want 720", lvl.IndentLeft)
	}

	// SetStart on a non-existent level should create it.
	nd.SetStart(1, 10)
	if len(nd.Levels()) != 2 {
		t.Fatalf("levels = %d, want 2", len(nd.Levels()))
	}
}
