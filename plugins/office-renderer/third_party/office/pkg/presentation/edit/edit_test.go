package edit

import (
	"bytes"
	"testing"

	"office/pkg/ooxml/opc"
	presir "office/pkg/presentation/ir"
	presparts "office/pkg/presentation/parsers/ooxml/parts"
)

func TestNewPresentation(t *testing.T) {
	p := NewPresentation()
	if p == nil || p.IR == nil {
		t.Fatal("expected non-nil presentation")
	}
	w, h := p.SlideSize()
	if w != 12192000 || h != 6858000 {
		t.Fatalf("unexpected slide size: %dx%d", w, h)
	}
	if p.SlideCount() != 0 {
		t.Fatalf("expected 0 slides, got %d", p.SlideCount())
	}
}

func TestAddAndRemoveSlides(t *testing.T) {
	p := NewPresentation()
	s1 := p.AddBlankSlide()
	s2 := p.AddBlankSlide()
	if p.SlideCount() != 2 {
		t.Fatalf("expected 2 slides, got %d", p.SlideCount())
	}
	if s1.Index() != 0 || s2.Index() != 1 {
		t.Fatalf("unexpected slide indexes: %d, %d", s1.Index(), s2.Index())
	}

	p.RemoveSlide(0)
	if p.SlideCount() != 1 {
		t.Fatalf("expected 1 slide after removal, got %d", p.SlideCount())
	}
	if p.Slide(0).Index() != 0 {
		t.Fatalf("expected reindexed slide 0, got %d", p.Slide(0).Index())
	}
}

func TestMoveSlide(t *testing.T) {
	p := NewPresentation()
	s1 := p.AddBlankSlide()
	s1.SetName("first")
	s2 := p.AddBlankSlide()
	s2.SetName("second")
	s3 := p.AddBlankSlide()
	s3.SetName("third")

	p.MoveSlide(0, 2)
	if p.Slide(0).Name() != "second" {
		t.Fatalf("expected second at 0, got %s", p.Slide(0).Name())
	}
	if p.Slide(2).Name() != "first" {
		t.Fatalf("expected first at 2, got %s", p.Slide(2).Name())
	}
}

func TestSlideAddTextBox(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(100, 200, 3000, 2000)
	if shape == nil {
		t.Fatal("expected shape")
	}
	if shape.Kind() != presir.ShapeText {
		t.Fatalf("expected text shape, got %v", shape.Kind())
	}
	x, y, w, h := shape.Bounds()
	if x != 100 || y != 200 || w != 3000 || h != 2000 {
		t.Fatalf("unexpected bounds: %d,%d %dx%d", x, y, w, h)
	}

	shape.SetText("Hello, World!")
	if shape.Text() != "Hello, World!" {
		t.Fatalf("shape text = %q", shape.Text())
	}

	para := shape.AddParagraph()
	run := para.AddRun("Second paragraph")
	run.SetBold(true)
	run.SetColor("FF0000")
	if run.Bold() != true {
		t.Fatal("expected bold run")
	}
	if run.Color() != "FF0000" {
		t.Fatalf("run color = %q", run.Color())
	}
}

func TestSlideAddShape(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddShape("ellipse", 0, 0, 1000, 1000)
	if shape == nil {
		t.Fatal("expected shape")
	}
	shape.SetFillSolid("00FF00")
	shape.SetLine("000000", 1.5)
	if shape.FillColor() != "00FF00" {
		t.Fatalf("fill color = %q", shape.FillColor())
	}
	c, w := shape.Line()
	if c != "000000" || w != 1.5 {
		t.Fatalf("line = %q, %f", c, w)
	}
}

func TestSlideAddTable(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(2, 3, 0, 0, 6000, 4000)
	if tbl == nil {
		t.Fatal("expected table")
	}
	if tbl.RowCount() != 2 || tbl.ColumnCount() != 3 {
		t.Fatalf("unexpected table size: %dx%d", tbl.RowCount(), tbl.ColumnCount())
	}

	cell := tbl.Cell(1, 2)
	if cell == nil {
		t.Fatal("expected cell")
	}
	cell.SetText("cell text")
	if cell.Text() != "cell text" {
		t.Fatalf("cell text = %q", cell.Text())
	}
}

func minimalPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0x0F, 0x00, 0x00,
		0x01, 0x01, 0x00, 0x05, 0x18, 0xD8, 0x4D, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
}

func TestSlideAddImage(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddImage(minimalPNG(), "image/png", 100, 100, 1000, 1000)
	if shape == nil {
		t.Fatal("expected image shape")
	}
	if shape.Kind() != presir.ShapeImage {
		t.Fatalf("expected image shape, got %v", shape.Kind())
	}
}

func TestRoundTripPPTX(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.SetBackgroundColor("F0F0F0")
	shape := s.AddTextBox(100, 100, 5000, 2000)
	shape.SetText("Round-trip test")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty pptx output")
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	if loaded.SlideCount() != 1 {
		t.Fatalf("expected 1 slide after load, got %d", loaded.SlideCount())
	}
	loadedSlide := loaded.Slide(0)
	if loadedSlide.BackgroundColor() != "F0F0F0" {
		t.Fatalf("background color = %q", loadedSlide.BackgroundColor())
	}
	shapes := loadedSlide.Shapes()
	if len(shapes) == 0 {
		t.Fatal("expected at least one shape after load")
	}
	if shapes[0].Text() != "Round-trip test" {
		t.Fatalf("shape text = %q", shapes[0].Text())
	}
}

func TestRoundTripImage(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	png := minimalPNG()
	shape := s.AddImage(png, "image/png", 100, 100, 1000, 1000)
	if shape == nil {
		t.Fatal("expected image shape")
	}

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	loadedShape := shapes[0]
	if loadedShape.Kind() != presir.ShapeImage {
		t.Fatalf("kind = %v", loadedShape.Kind())
	}
	x, y, w, h := loadedShape.Bounds()
	if x != 100 || y != 100 || w != 1000 || h != 1000 {
		t.Fatalf("bounds = %d,%d %dx%d", x, y, w, h)
	}
}

func TestRoundTripTextFormatting(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 5000, 2000)
	para := shape.AddParagraph()
	run := para.AddRun("Formatted")
	run.SetBold(true)
	run.SetItalic(true)
	run.SetUnderline(true)
	run.SetStrike(true)
	run.SetColor("123456")
	run.SetFontSize(2400)
	run.SetFontFamily("Arial")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	paragraphs := shapes[0].Paragraphs()
	if len(paragraphs) != 1 {
		t.Fatalf("paragraphs = %d", len(paragraphs))
	}
	runs := paragraphs[0].Runs()
	if len(runs) != 1 {
		t.Fatalf("runs = %d", len(runs))
	}
	r := runs[0]
	if r.Text() != "Formatted" {
		t.Fatalf("text = %q", r.Text())
	}
	if !r.Bold() || !r.Italic() || !r.Underline() || !r.Strike() {
		t.Fatalf("formatting flags: bold=%v italic=%v underline=%v strike=%v", r.Bold(), r.Italic(), r.Underline(), r.Strike())
	}
	if r.Color() != "123456" {
		t.Fatalf("color = %q", r.Color())
	}
	if r.FontSize() != 2400 {
		t.Fatalf("font size = %d", r.FontSize())
	}
	if r.FontFamily() != "Arial" {
		t.Fatalf("font family = %q", r.FontFamily())
	}
}

func TestRoundTripShapeFillAndLine(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddShape("ellipse", 0, 0, 1000, 1000)
	shape.SetFillSolid("AABBCC")
	shape.SetLine("112233", 2.5)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	if shapes[0].FillColor() != "AABBCC" {
		t.Fatalf("fill color = %q", shapes[0].FillColor())
	}
	c, w := shapes[0].Line()
	if c != "112233" || w != 2.5 {
		t.Fatalf("line = %q, %f", c, w)
	}
}

func TestRoundTripTableCellText(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	tbl := s.AddTable(2, 2, 0, 0, 4000, 4000)
	tbl.Cell(0, 0).SetText("top-left")
	tbl.Cell(1, 1).SetText("bottom-right")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	loadedTbl := shapes[0].Table()
	if loadedTbl == nil {
		t.Fatal("expected table")
	}
	if loadedTbl.RowCount() != 2 || loadedTbl.ColumnCount() != 2 {
		t.Fatalf("table size = %dx%d", loadedTbl.RowCount(), loadedTbl.ColumnCount())
	}
	if loadedTbl.Cell(0, 0).Text() != "top-left" {
		t.Fatalf("cell(0,0) = %q", loadedTbl.Cell(0, 0).Text())
	}
	if loadedTbl.Cell(1, 1).Text() != "bottom-right" {
		t.Fatalf("cell(1,1) = %q", loadedTbl.Cell(1, 1).Text())
	}
}

func TestSlideTransition(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.SetTransition(&presir.SlideTransition{
		Type:     "fade",
		Spd:      "fast",
		AdvClick: false,
		AdvTmMs:  2000,
	})

	tr := s.Transition()
	if tr == nil {
		t.Fatal("expected transition")
	}
	if tr.Type != "fade" || tr.Spd != "fast" || tr.AdvClick != false || tr.AdvTmMs != 2000 {
		t.Fatalf("transition = %+v", tr)
	}
}

func TestRoundTripTransition(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.SetTransition(&presir.SlideTransition{
		Type:     "push",
		Spd:      "med",
		AdvClick: true,
		AdvTmMs:  5000,
	})

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	tr := loaded.Slide(0).Transition()
	if tr == nil {
		t.Fatal("expected transition after load")
	}
	if tr.Type != "push" || tr.Spd != "med" || !tr.AdvClick || tr.AdvTmMs != 5000 {
		t.Fatalf("transition = %+v", tr)
	}
}

func TestSlideAddGroup(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	g := s.AddGroup(0, 0, 5000, 5000)
	if g == nil {
		t.Fatal("expected group")
	}
	x, y, w, h := g.Bounds()
	if x != 0 || y != 0 || w != 5000 || h != 5000 {
		t.Fatalf("bounds = %d,%d %dx%d", x, y, w, h)
	}

	child := g.AddTextBox(100, 100, 1000, 1000)
	if child == nil {
		t.Fatal("expected child shape")
	}
	child.SetText("grouped")
	if len(g.Shapes()) != 1 {
		t.Fatalf("group children = %d", len(g.Shapes()))
	}
}

func TestRoundTripGroup(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	g := s.AddGroup(0, 0, 5000, 5000)
	box := g.AddTextBox(100, 100, 1000, 1000)
	box.SetText("inside group")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	if shapes[0].Kind() != presir.ShapeGroup {
		t.Fatalf("kind = %v", shapes[0].Kind())
	}
	lg := shapes[0].Group()
	if lg == nil {
		t.Fatal("expected group accessor")
	}
	children := lg.Shapes()
	if len(children) != 1 {
		t.Fatalf("group children = %d", len(children))
	}
	if children[0].Text() != "inside group" {
		t.Fatalf("child text = %q", children[0].Text())
	}
}

func TestSlideAddConnector(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	c := s.AddConnector(0, 0, 1000, 1000)
	if c == nil {
		t.Fatal("expected connector")
	}
	if c.Kind() != presir.ShapeConnector {
		t.Fatalf("kind = %v", c.Kind())
	}
	c.SetLine("FF0000", 2)
	c.SetArrowHead("arrow")
	if c.ArrowHead() != "arrow" {
		t.Fatalf("arrow head = %q", c.ArrowHead())
	}
}

func TestRoundTripConnector(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	c := s.AddConnector(0, 0, 1000, 1000)
	c.SetLine("00FF00", 1.5)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	if shapes[0].Kind() != presir.ShapeConnector {
		t.Fatalf("kind = %v", shapes[0].Kind())
	}
	color, width := shapes[0].Line()
	if color != "00FF00" || width != 1.5 {
		t.Fatalf("line = %q, %f", color, width)
	}
}

func TestRoundTripConnectorDetails(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	c := s.AddConnector(0, 0, 1000, 1000)
	c.SetConnectorType("straight")
	c.SetArrowHead("arrow")
	c.SetArrowTail("triangle")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	conn := shapes[0]
	if conn.ConnectorType() != "straight" {
		t.Fatalf("connector type = %q", conn.ConnectorType())
	}
	if conn.ArrowHead() != "arrow" {
		t.Fatalf("arrow head = %q", conn.ArrowHead())
	}
	if conn.ArrowTail() != "triangle" {
		t.Fatalf("arrow tail = %q", conn.ArrowTail())
	}
}

func TestSlideAddPlaceholder(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	ph := s.AddPlaceholder(presir.PlaceholderTitle, 0, 0, 5000, 1000)
	if ph == nil {
		t.Fatal("expected placeholder")
	}
	if ph.Kind() != presir.ShapeText {
		t.Fatalf("kind = %v", ph.Kind())
	}
	if ph.Placeholder() != presir.PlaceholderTitle {
		t.Fatalf("placeholder = %v", ph.Placeholder())
	}
	ph.SetText("Title Text")
	if ph.Text() != "Title Text" {
		t.Fatalf("text = %q", ph.Text())
	}
}

func TestRoundTripPlaceholder(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	ph := s.AddPlaceholder(presir.PlaceholderBody, 100, 100, 4000, 3000)
	ph.SetText("Body content")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	if shapes[0].Placeholder() != presir.PlaceholderBody {
		t.Fatalf("placeholder = %v", shapes[0].Placeholder())
	}
	if shapes[0].Text() != "Body content" {
		t.Fatalf("text = %q", shapes[0].Text())
	}
}

func minimalChartXML() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"
              xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart>
    <c:plotArea>
      <c:barChart>
        <c:axId val="1"/>
        <c:axId val="2"/>
      </c:barChart>
      <c:catAx><c:axId val="1"/></c:catAx>
      <c:valAx><c:axId val="2"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`)
}

func TestSlideAddChart(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	chart := s.AddChart(minimalChartXML(), 0, 0, 4000, 3000)
	if chart == nil {
		t.Fatal("expected chart")
	}
	if chart.Kind() != presir.ShapeChart {
		t.Fatalf("kind = %v", chart.Kind())
	}
	x, y, w, h := chart.Bounds()
	if x != 0 || y != 0 || w != 4000 || h != 3000 {
		t.Fatalf("bounds = %d,%d %dx%d", x, y, w, h)
	}
}

func TestRoundTripChart(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	chartXML := minimalChartXML()
	s.AddChart(chartXML, 0, 0, 4000, 3000)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	if shapes[0].Kind() != presir.ShapeChart {
		t.Fatalf("kind = %v", shapes[0].Kind())
	}
}

func TestSlideAddComment(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddComment("First note", "Alice", "A", 100, 100)
	s.AddComment("Second note", "Bob", "B", 200, 200)
	if len(s.Comments()) != 2 {
		t.Fatalf("comments = %d", len(s.Comments()))
	}
}

func TestRoundTripComments(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddComment("Check this figure", "Alice", "A", 100, 100)
	s.AddComment("Looks good", "Bob", "B", 200, 200)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	comments := loaded.Slide(0).Comments()
	if len(comments) != 2 {
		t.Fatalf("comments = %d", len(comments))
	}
	found := make(map[string]bool)
	for _, c := range comments {
		found[c.Text] = true
		if c.AuthorName != "Alice" && c.AuthorName != "Bob" {
			t.Fatalf("author = %q", c.AuthorName)
		}
	}
	if !found["Check this figure"] || !found["Looks good"] {
		t.Fatalf("comments lost: %+v", comments)
	}
}

func TestSlideSetNotesText(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.SetNotesText("Speaker notes here")
	if s.NotesText() != "Speaker notes here" {
		t.Fatalf("notes text = %q", s.NotesText())
	}
}

func TestRoundTripNotes(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.SetNotesText("Remember to mention ROI")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	if loaded.Slide(0).NotesText() != "Remember to mention ROI" {
		t.Fatalf("notes text = %q", loaded.Slide(0).NotesText())
	}
}

func TestSlideAddMedia(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	video := s.AddMedia([]byte{0x00, 0x00, 0x00, 0x00}, "video/mp4", "video", 0, 0, 4000, 3000)
	if video == nil {
		t.Fatal("expected media shape")
	}
	if video.Kind() != presir.ShapeMedia {
		t.Fatalf("kind = %v", video.Kind())
	}
}

func TestRoundTripMedia(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddMedia([]byte{0x00, 0x01, 0x02, 0x03}, "audio/mpeg", "audio", 100, 100, 1000, 1000)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	if shapes[0].Kind() != presir.ShapeMedia {
		t.Fatalf("kind = %v", shapes[0].Kind())
	}
}

func TestSlideAddOLEObject(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	obj := s.AddOLEObject([]byte("OLE payload"), "application/vnd.openxmlformats-officedocument.oleObject", "Word.Document.12", "Document", 0, 0, 4000, 3000)
	if obj == nil {
		t.Fatal("expected OLE object shape")
	}
	if obj.Kind() != presir.ShapeOleObject {
		t.Fatalf("kind = %v", obj.Kind())
	}
	obj.SetOLEIcon(minimalPNG(), "image/png")
	if obj.OLEObjectProgID() != "Word.Document.12" {
		t.Fatalf("prog id = %q", obj.OLEObjectProgID())
	}
	if obj.OLEObjectName() != "Document" {
		t.Fatalf("object name = %q", obj.OLEObjectName())
	}
	if string(obj.OLEObjectData()) != "OLE payload" {
		t.Fatalf("payload = %q", string(obj.OLEObjectData()))
	}
	if icon, mime := obj.OLEIcon(); mime != "image/png" || len(icon) == 0 {
		t.Fatalf("icon mime = %q len = %d", mime, len(icon))
	}
}

func TestRoundTripOLEObject(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	obj := s.AddOLEObject([]byte("OLE payload"), "application/vnd.openxmlformats-officedocument.oleObject", "Word.Document.12", "Document", 100, 100, 4000, 3000)
	obj.SetOLEIcon(minimalPNG(), "image/png")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	loadedObj := shapes[0]
	if loadedObj.Kind() != presir.ShapeOleObject {
		t.Fatalf("kind = %v", loadedObj.Kind())
	}
	if loadedObj.OLEObjectProgID() != "Word.Document.12" {
		t.Fatalf("prog id = %q", loadedObj.OLEObjectProgID())
	}
	if loadedObj.OLEObjectName() != "Document" {
		t.Fatalf("object name = %q", loadedObj.OLEObjectName())
	}
	if got := string(loadedObj.OLEObjectData()); got != "OLE payload" {
		t.Fatalf("payload = %q, want OLE payload", got)
	}
	if icon, mime := loadedObj.OLEIcon(); mime != "image/png" || len(icon) == 0 {
		t.Fatalf("icon mime = %q len = %d", mime, len(icon))
	}
}

func TestRoundTripGroupOLEObject(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	g := s.AddGroup(0, 0, 5000, 5000)
	obj := g.AddOLEObject([]byte("OLE payload"), "application/vnd.openxmlformats-officedocument.oleObject", "Package", "Embedded Package", 200, 300, 1000, 1200)
	obj.SetOLEIcon(minimalPNG(), "image/png")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	children := shapes[0].Group().Shapes()
	if len(children) != 1 {
		t.Fatalf("group children = %d", len(children))
	}
	child := children[0]
	if child.Kind() != presir.ShapeOleObject {
		t.Fatalf("kind = %v", child.Kind())
	}
	if child.OLEObjectName() != "Embedded Package" {
		t.Fatalf("object name = %q", child.OLEObjectName())
	}
	if got := string(child.OLEObjectData()); got != "OLE payload" {
		t.Fatalf("payload = %q, want OLE payload", got)
	}
}

func TestShapeAnimationStep(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetAnimationStep(2)
	if shape.AnimationStep() != 2 {
		t.Fatalf("animation step = %d", shape.AnimationStep())
	}
}

func TestRoundTripAnimationStep(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddTextBox(0, 0, 1000, 1000).SetText("visible")
	second := s.AddTextBox(0, 1000, 1000, 1000)
	second.SetText("step 1")
	second.SetAnimationStep(1)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 2 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	// At least one shape should have an animation step.
	found := false
	for _, sh := range shapes {
		if sh.AnimationStep() > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("no animation step found after round-trip")
	}
}

func TestShapeEmphasisEffects(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.AddEmphasisEffect("spin")
	shape.AddEmphasisEffect("growShrink")

	effects := shape.EmphasisEffects()
	if len(effects) != 2 || effects[0] != "spin" || effects[1] != "growShrink" {
		t.Fatalf("emphasis effects = %v", effects)
	}

	shape.ClearEmphasisEffects()
	if len(shape.EmphasisEffects()) != 0 {
		t.Fatalf("expected cleared emphasis effects")
	}
}

func TestShapeMotionPath(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.SetMotionPathWithOptions("M 0 0 L 100000 0", 750, "parent")

	path, dur, origin, ok := shape.MotionPath()
	if !ok {
		t.Fatal("expected motion path")
	}
	if path != "M 0 0 L 100000 0" || dur != 750 || origin != "parent" {
		t.Fatalf("motion path = %q, %d, %q", path, dur, origin)
	}

	shape.ClearMotionPath()
	if _, _, _, ok := shape.MotionPath(); ok {
		t.Fatal("expected motion path cleared")
	}
}

func TestShapeAnimationTriggers(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	shape := s.AddTextBox(0, 0, 1000, 1000)
	shape.AddAnimationTrigger("onClick")
	shape.AddAnimationTrigger("onHover")

	triggers := shape.AnimationTriggers()
	if len(triggers) != 2 || triggers[0] != "onClick" || triggers[1] != "onHover" {
		t.Fatalf("triggers = %v", triggers)
	}

	shape.ClearAnimationTriggers()
	if len(shape.AnimationTriggers()) != 0 {
		t.Fatalf("expected cleared triggers")
	}
}

func TestRoundTripAnimations(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()

	entrance := s.AddTextBox(0, 0, 1000, 1000)
	entrance.SetAnimationStep(1)

	emph := s.AddTextBox(0, 1000, 1000, 1000)
	emph.AddEmphasisEffect("spin")

	motion := s.AddTextBox(0, 2000, 1000, 1000)
	motion.SetMotionPathWithOptions("M 0 0 L 50000 50000", 600, "layout")

	trigger := s.AddTextBox(0, 3000, 1000, 1000)
	trigger.AddAnimationTrigger("onClick")

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}

	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 4 {
		t.Fatalf("shapes = %d", len(shapes))
	}

	var foundStep, foundEmph, foundMotion, foundTrigger bool
	for _, sh := range shapes {
		if sh.AnimationStep() == 1 {
			foundStep = true
		}
		if len(sh.EmphasisEffects()) > 0 {
			foundEmph = true
		}
		if _, _, _, ok := sh.MotionPath(); ok {
			foundMotion = true
		}
		if len(sh.AnimationTriggers()) > 0 {
			foundTrigger = true
		}
	}
	if !foundStep {
		t.Error("entrance animation step lost")
	}
	if !foundEmph {
		t.Error("emphasis effect lost")
	}
	if !foundMotion {
		t.Error("motion path lost")
	}
	if !foundTrigger {
		t.Error("animation trigger lost")
	}
}

func TestRoundTripGroupAnimation(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	g := s.AddGroup(0, 0, 5000, 5000)
	child := g.AddTextBox(100, 100, 1000, 1000)
	child.SetAnimationStep(1)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shapes = %d", len(shapes))
	}
	children := shapes[0].Group().Shapes()
	if len(children) != 1 {
		t.Fatalf("group children = %d", len(children))
	}
	if children[0].AnimationStep() != 1 {
		t.Fatalf("group child animation step = %d", children[0].AnimationStep())
	}
}

func TestRoundTripEmbeddedFont(t *testing.T) {
	p := NewPresentation()
	p.AddBlankSlide()
	fontData := []byte("true font data placeholder")
	p.AddEmbeddedFont("Test Font", fontData)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("LoadFromPPTXBytes: %v", err)
	}
	names := loaded.EmbeddedFontNames()
	if len(names) != 1 || names[0] != "Test Font" {
		t.Fatalf("embedded font names = %v, want [Test Font]", names)
	}
}

func TestEmbeddedFontFaces(t *testing.T) {
	p := NewPresentation()
	p.AddEmbeddedFontFace("Faces", 400, "normal", "", []byte("regular"))
	p.AddEmbeddedFontFace("Faces", 700, "normal", "", []byte("bold"))
	p.AddEmbeddedFontFace("Faces", 400, "italic", "", []byte("italic"))
	p.AddEmbeddedFontFace("Faces", 700, "italic", "", []byte("boldItalic"))

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	partsPkg, err := presparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	if len(partsPkg.EmbeddedFonts) != 1 {
		t.Fatalf("embedded font count = %d, want 1", len(partsPkg.EmbeddedFonts))
	}
	font := partsPkg.EmbeddedFonts[0]
	if font.Typeface != "Faces" {
		t.Errorf("typeface = %q, want Faces", font.Typeface)
	}
	if font.Regular == nil || string(font.Regular.Data) != "regular" {
		t.Error("regular face missing or wrong")
	}
	if font.Bold == nil || string(font.Bold.Data) != "bold" {
		t.Error("bold face missing or wrong")
	}
	if font.Italic == nil || string(font.Italic.Data) != "italic" {
		t.Error("italic face missing or wrong")
	}
	if font.BoldItalic == nil || string(font.BoldItalic.Data) != "boldItalic" {
		t.Error("boldItalic face missing or wrong")
	}
}

func TestRoundTripSlideMasterLayout(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	master := p.AddSlideMaster()
	master.SetName("Custom Master")
	layout := master.AddLayout("title")
	layout.SetName("Title Layout")
	layout.AddTextBox(0, 0, 1000, 1000).SetText("title placeholder")
	s.SetLayout(layout)

	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("SaveAsPPTX: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	partsPkg, err := presparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	if len(partsPkg.SlideMasters) != 1 {
		t.Fatalf("slide master count = %d, want 1", len(partsPkg.SlideMasters))
	}
	customFound := false
	for _, me := range partsPkg.SlideMasters {
		if me == nil || me.Master == nil {
			continue
		}
		if me.Master.CSld.Name == "Custom Master" {
			customFound = true
			if len(me.Layouts) == 0 {
				t.Fatal("custom master has no layouts")
			}
			if me.Layouts[0].Type != "title" {
				t.Fatalf("layout type = %q, want title", me.Layouts[0].Type)
			}
		}
	}
	if !customFound {
		t.Fatal("custom master not found after round-trip")
	}
	if len(partsPkg.Slides) == 0 || partsPkg.Slides[0].Layout == nil {
		t.Fatal("slide layout not preserved")
	}
	if partsPkg.Slides[0].Layout.Type != "title" {
		t.Fatalf("slide layout type = %q, want title", partsPkg.Slides[0].Layout.Type)
	}
}
func TestLinkedOLEObjectRoundTrip(t *testing.T) {
	p := NewPresentation()
	s := p.AddSlide("blank")
	obj := s.AddLinkedOLEObject("https://example.com/report.xlsx", "Excel.Sheet.12", "Linked report", 100000, 200000, 3000000, 2000000)
	if obj == nil || obj.OLEObjectLinkURL() != "https://example.com/report.xlsx" {
		t.Fatalf("linked OLE object = %#v", obj)
	}
	var buf bytes.Buffer
	if err := p.SaveAsPPTX(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadFromPPTXBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	shapes := loaded.Slide(0).Shapes()
	if len(shapes) != 1 {
		t.Fatalf("shape count = %d, want 1", len(shapes))
	}
	got := shapes[0]
	if got.OLEObjectLinkURL() != "https://example.com/report.xlsx" {
		t.Fatalf("link = %q", got.OLEObjectLinkURL())
	}
	if len(got.OLEObjectData()) != 0 {
		t.Fatalf("linked OLE unexpectedly has embedded payload")
	}
}
