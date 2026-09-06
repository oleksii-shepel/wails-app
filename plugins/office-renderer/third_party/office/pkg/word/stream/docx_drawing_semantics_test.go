package stream

import (
	"testing"

	"office/pkg/ooxml/chart"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestDOCXDrawingSemanticHandlerAppendsChart(t *testing.T) {
	xmlData, err := chart.MarshalChart(&chart.Chart{ChartType: chart.ChartBar, Title: "Sales"})
	if err != nil { t.Fatalf("MarshalChart: %v", err) }
	session := NewDocumentSession()
	layout := &docxLayoutHandler{session: session, media: &docxMediaLoader{cache: map[string][]byte{"word/charts/chart1.xml": xmlData}}}
	inner := wrapDOCXLayoutSemantics(layout)
	h := wrapDOCXDrawingSemantics(inner, layout)
	event := Event{Type: EventDrawing, Raw: wordsax.Event{
		Type: wordsax.EventDrawing,
		DrawingGraphicKind: "chart",
		RelatedParts: []wordsax.RelatedPart{{
			RelationshipType: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart",
			Target: "word/charts/chart1.xml",
		}},
	}}
	if err := h.HandleEvent(event); err != nil { t.Fatalf("HandleEvent: %v", err) }
	if session.paragraph == nil || len(session.paragraph.Runs) != 1 { t.Fatalf("paragraph = %#v", session.paragraph) }
	run := session.paragraph.Runs[0]
	if run.Kind != "chart" || run.Chart == nil || run.Chart.Title != "Sales" { t.Fatalf("run = %#v", run) }
}

func TestDOCXDrawingSemanticHandlerAppendsAltChunkInsideTableCell(t *testing.T) {
	session := NewDocumentSession()
	layout := &docxLayoutHandler{session: session}
	semantic := wrapDOCXLayoutSemantics(layout)
	h := wrapDOCXDrawingSemantics(semantic, layout)
	if err := session.HandleEvent(Event{Type: EventStartTable, Story: StoryDocument}); err != nil { t.Fatal(err) }
	if err := session.HandleEvent(Event{Type: EventStartTableRow, Story: StoryDocument}); err != nil { t.Fatal(err) }
	if err := session.HandleEvent(Event{Type: EventStartTableCell, Story: StoryDocument}); err != nil { t.Fatal(err) }
	payload := []byte("<html><body>chunk</body></html>")
	if err := h.HandleEvent(Event{Type: EventAltChunk, Story: StoryDocument, Raw: wordsax.Event{Type: wordsax.EventAltChunk, PayloadData: payload, PayloadContentType: "text/html"}}); err != nil { t.Fatalf("HandleEvent: %v", err) }
	frame := session.topTable()
	if frame == nil || frame.cell == nil || len(frame.cell.Blocks) != 1 { t.Fatalf("frame = %#v", frame) }
	block := frame.cell.Blocks[0]
	if block.Kind != "altChunk" || block.AltChunk == nil || string(block.AltChunk.Data) != string(payload) { t.Fatalf("block = %#v", block) }
}
