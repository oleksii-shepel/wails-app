package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func collectEvents(t *testing.T, cursor Cursor, context *Context) []Event {
	t.Helper()
	var events []Event
	if err := Walk(cursor, context, func(event Event) error { events = append(events, event); return nil }); err != nil { t.Fatalf("Walk: %v", err) }
	return events
}

func TestWalkEmitsParagraphTextAndBreaks(t *testing.T) {
	cursor := &sliceCursor{runs: []Run{{Text:"hello\tworld\vnext\r", StartCP:0}}}
	events := collectEvents(t, cursor, nil)
	var types []EventType
	var texts []string
	for _, event := range events { types = append(types, event.Type); if event.Type==EventText { texts=append(texts,event.Text) } }
	if len(events) < 8 || events[0].Type != EventStartDocument || events[len(events)-1].Type != EventEndDocument { t.Fatalf("events=%#v", events) }
	if len(texts)!=3 || texts[0]!="hello" || texts[1]!="world" || texts[2]!="next" { t.Fatalf("texts=%#v", texts) }
	seenTab, seenLine, seenEnd := false,false,false
	for _, event := range events { if event.Type==EventTab { seenTab=true }; if event.Type==EventBreak&&event.BreakType=="line" { seenLine=true }; if event.Type==EventEndParagraph { seenEnd=true } }
	if !seenTab || !seenLine || !seenEnd { t.Fatalf("types=%#v", types) }
}

func TestWalkAttachesReferenceStoriesAndSections(t *testing.T) {
	story := wordir.Story{Paragraphs:[]wordir.Paragraph{{Runs:[]wordir.Run{{Kind:"run",Text:"note"}}}}}
	section := wordir.Section{Source:&wordir.SectionSource{StartCP:0,EndCP:2}}
	cursor := &sliceCursor{runs: []Run{{Text:"ab",StartCP:0},{Kind:"footnoteRef",NoteID:3,StartCP:2},{Text:"c\r",StartCP:2}}}
	context := &Context{Footnotes:map[int]wordir.Story{3:story},Sections:[]wordir.Section{section}}
	events := collectEvents(t,cursor,context)
	seenSection,seenNote:=false,false
	for _, event := range events {
		if event.Type==EventSection && event.Section!=nil && event.Section.Source!=nil && event.Section.Source.EndCP==2 { seenSection=true }
		if event.Type==EventFootnoteReference && event.NoteID==3 && event.Story!=nil && len(event.Story.Paragraphs)==1 && len(event.Story.Paragraphs[0].Runs)==1 && event.Story.Paragraphs[0].Runs[0].Text=="note" { seenNote=true }
	}
	if !seenSection || !seenNote { t.Fatalf("events=%#v",events) }
}

func TestWalkEmitsNestedTableBoundaries(t *testing.T) {
	cursor := &sliceCursor{runs: []Run{
		{Text:"a\a",StartCP:0,TableDepth:1,ParagraphFormat:wordir.ParagraphFormat{InTable:true}},
		{Text:"b\a",StartCP:2,TableDepth:1,ParagraphFormat:wordir.ParagraphFormat{InTable:true,IsTableTrailingParagraph:true}},
	}}
	events:=collectEvents(t,cursor,nil)
	counts:=map[EventType]int{}
	for _,event:=range events{counts[event.Type]++}
	if counts[EventStartTable]!=1 || counts[EventEndTable]!=1 || counts[EventStartTableRow]!=1 || counts[EventEndTableRow]!=1 || counts[EventStartTableCell]!=2 || counts[EventEndTableCell]!=2 { t.Fatalf("counts=%#v events=%#v",counts,events) }
}
