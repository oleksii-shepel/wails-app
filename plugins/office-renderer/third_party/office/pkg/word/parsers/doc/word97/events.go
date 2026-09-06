package word97

import (
	"fmt"
	"strings"

	wordevent "office/pkg/word/event"
	wordir "office/pkg/word/ir"
)

type EventType = wordevent.Type

const (
	EventStartDocument     = wordevent.StartDocument
	EventEndDocument       = wordevent.EndDocument
	EventStartParagraph    = wordevent.StartParagraph
	EventEndParagraph      = wordevent.EndParagraph
	EventStartTable        = wordevent.StartTable
	EventEndTable          = wordevent.EndTable
	EventStartTableRow     = wordevent.StartTableRow
	EventEndTableRow       = wordevent.EndTableRow
	EventStartTableCell    = wordevent.StartTableCell
	EventEndTableCell      = wordevent.EndTableCell
	EventText              = wordevent.Text
	EventTab               = wordevent.Tab
	EventBreak             = wordevent.Break
	EventDrawing           = wordevent.Drawing
	EventEmbeddedObject    = wordevent.EmbeddedObject
	EventBookmarkStart     = wordevent.BookmarkStart
	EventBookmarkEnd       = wordevent.BookmarkEnd
	EventFootnoteReference = wordevent.FootnoteReference
	EventEndnoteReference  = wordevent.EndnoteReference
	EventCommentReference  = wordevent.CommentReference
	EventSection           = wordevent.Section
)

type Context struct {
	Headers map[string]wordir.Story
	Footers map[string]wordir.Story
	FootnoteSeparator          *wordir.Story
	FootnoteContinuationSep    *wordir.Story
	FootnoteContinuationNotice *wordir.Story
	EndnoteSeparator           *wordir.Story
	EndnoteContinuationSep     *wordir.Story
	EndnoteContinuationNotice  *wordir.Story
	Sections      []wordir.Section
	FootnoteRefs  map[int]int
	EndnoteRefs   map[int]int
	CommentRefs   map[int]int
	Footnotes     map[int]wordir.Story
	Endnotes      map[int]wordir.Story
	Comments      map[int]wordir.Story
	OLEByCP       map[int]*wordir.Run
	Drawings DrawingProjection
}

type Event = wordevent.Event
type EventHandler = wordevent.Handler

func numberingReference(run Run) *wordir.NumberingReference {
	if run.ParagraphFormat.BinaryListOverride == nil || run.ParagraphFormat.BinaryListLevel == nil { return nil }
	ilfo := *run.ParagraphFormat.BinaryListOverride
	level := *run.ParagraphFormat.BinaryListLevel
	if ilfo <= 0 || level < 0 { return nil }
	return &wordir.NumberingReference{NumID:ilfo, Level:level}
}

func tableDepth(run Run) int {
	if run.TableDepth > 0 { return run.TableDepth }
	if run.ParagraphFormat.InTable || run.LegacyInTableHint || strings.ContainsRune(run.Text, '\a') { return 1 }
	return 0
}

func validateRun(run Run) error {
	// fData is a generic data-bearing flag. Fixed Word97 pictures are identified
	// and either resolved or rejected earlier by the drawing cursor while
	// sprmCPicLocation semantics are still available.
	if run.Spec && run.Obj { return UnsupportedError{Feature:"OLE object anchor"} }
	if strings.ContainsAny(run.Text, "\x13\x14\x15") { return UnsupportedError{Feature:"field"} }
	return nil
}

func applyRunSemantics(ev *Event, run Run) {
	if ev == nil { return }
	ev.Revision = run.Revision
	ev.RevisionAuthor = run.RevisionAuthor
	ev.RevisionDate = run.RevisionDate
	ev.RunPropertyRevision = run.RunPropertyRevision
	ev.DisplayFieldRevision = run.DisplayFieldRevision
}

func runEvent(typ EventType, run Run) Event {
	format := run.Format
	ev := Event{Type:typ, RunFormat:&format, RunSource:runSourceFromRun(run)}
	applyRunSemantics(&ev, run)
	return ev
}

func Walk(cursor Cursor, context *Context, handler EventHandler) error {
	if cursor == nil { return fmt.Errorf("doc word97 stream: nil run cursor") }
	if handler == nil { return fmt.Errorf("doc word97 stream: handler cannot be nil") }
	start := Event{Type:EventStartDocument}
	if context != nil {
		start.Headers=context.Headers; start.Footers=context.Footers
		start.FootnoteSeparator=context.FootnoteSeparator; start.FootnoteContinuationSep=context.FootnoteContinuationSep; start.FootnoteContinuationNotice=context.FootnoteContinuationNotice
		start.EndnoteSeparator=context.EndnoteSeparator; start.EndnoteContinuationSep=context.EndnoteContinuationSep; start.EndnoteContinuationNotice=context.EndnoteContinuationNotice
	}
	if err:=handler(start);err!=nil{return err}
	paragraphOpen:=false; tableLevel:=0; rowOpen:=make([]bool,0,2); cellOpen:=make([]bool,0,2); sectionIndex:=0
	openParagraph:=func(run Run)error{
		if paragraphOpen{return nil}
		paragraphOpen=true
		format:=run.ParagraphFormat
		return handler(Event{Type:EventStartParagraph,ParagraphFormat:&format,ParagraphNumbering:numberingReference(run),ParagraphSource:paragraphSourceFromRun(run),ParagraphPropertyRevision:run.ParagraphPropertyRevision})
	}
	closeParagraph:=func(run Run,force bool)error{if !paragraphOpen{if !force{return nil};if err:=openParagraph(run);err!=nil{return err}};paragraphOpen=false;return handler(Event{Type:EventEndParagraph})}
	ensureTableLevel:=func(depth int)error{for tableLevel<depth{if err:=handler(Event{Type:EventStartTable});err!=nil{return err};tableLevel++;rowOpen=append(rowOpen,false);cellOpen=append(cellOpen,false)};if depth>0{level:=depth-1;if !rowOpen[level]{if err:=handler(Event{Type:EventStartTableRow});err!=nil{return err};rowOpen[level]=true};if !cellOpen[level]{if err:=handler(Event{Type:EventStartTableCell});err!=nil{return err};cellOpen[level]=true}};return nil}
	closeTableToDepth:=func(depth int)error{if paragraphOpen{if err:=closeParagraph(Run{},false);err!=nil{return err}};for tableLevel>depth{level:=tableLevel-1;if cellOpen[level]{if err:=handler(Event{Type:EventEndTableCell});err!=nil{return err};cellOpen[level]=false};if rowOpen[level]{if err:=handler(Event{Type:EventEndTableRow});err!=nil{return err};rowOpen[level]=false};if err:=handler(Event{Type:EventEndTable});err!=nil{return err};tableLevel--;rowOpen=rowOpen[:tableLevel];cellOpen=cellOpen[:tableLevel]};return nil}
	emitSectionsThrough:=func(cp int)error{if context==nil{return nil};for sectionIndex<len(context.Sections){sec:=context.Sections[sectionIndex];end:=0;if sec.Source!=nil{end=sec.Source.EndCP};if end>cp{return nil};if err:=closeTableToDepth(0);err!=nil{return err};copySec:=sec;if err:=handler(Event{Type:EventSection,Section:&copySec});err!=nil{return err};sectionIndex++};return nil}
	emitText:=func(run Run,text string)error{if text==""{return nil};ev:=runEvent(EventText,run);ev.Text=text;ev.FieldKind=run.FieldKind;ev.FieldInstr=run.FieldInstr;ev.FieldArg=run.FieldArg;ev.Target=run.Target;ev.Anchor=run.Anchor;return handler(ev)}
	for {
		run,ok,err:=cursor.Next();if err!=nil{return err};if !ok{break}
		if err:=validateRun(run);err!=nil{return err}
		if err:=emitSectionsThrough(run.StartCP);err!=nil{return err}
		if run.Kind=="bookmarkStart"||run.Kind=="bookmarkEnd"{if err:=openParagraph(run);err!=nil{return err};typ:=EventBookmarkStart;if run.Kind=="bookmarkEnd"{typ=EventBookmarkEnd};if err:=handler(Event{Type:typ,BookmarkName:run.Anchor});err!=nil{return err};continue}
		if run.Kind=="olePlaceholder"{if err:=openParagraph(run);err!=nil{return err};var embedded *wordir.Run;if context!=nil&&context.OLEByCP!=nil{if value:=context.OLEByCP[run.StartCP];value!=nil{copyRun:=*value;embedded=&copyRun}};if embedded==nil{embedded=&wordir.Run{Kind:"olePlaceholder",Text:"Embedded Object",OLEMIME:"application/octet-stream",OLEType:"Embed"}};ev:=runEvent(EventEmbeddedObject,run);embedded.Format=*ev.RunFormat;embedded.Source=ev.RunSource;ev.EmbeddedRun=embedded;if err:=handler(ev);err!=nil{return err};continue}
		if run.Kind=="footnoteRef"||run.Kind=="endnoteRef"||run.Kind=="commentRef"{if err:=openParagraph(run);err!=nil{return err};ev:=runEvent(EventFootnoteReference,run);if run.Kind=="footnoteRef"{ev.Type=EventFootnoteReference;ev.NoteID=run.NoteID;if context!=nil{if story,ok:=context.Footnotes[run.NoteID];ok{s:=story;ev.Story=&s}}}else if run.Kind=="endnoteRef"{ev.Type=EventEndnoteReference;ev.NoteID=run.NoteID;if context!=nil{if story,ok:=context.Endnotes[run.NoteID];ok{s:=story;ev.Story=&s}}}else{ev.Type=EventCommentReference;ev.CommentID=run.CommentID;if context!=nil{if story,ok:=context.Comments[run.CommentID];ok{s:=story;ev.Story=&s}}};if err:=handler(ev);err!=nil{return err};continue}
		depth:=tableDepth(run);if depth<tableLevel{if err:=closeTableToDepth(depth);err!=nil{return err}};if depth>0{if err:=ensureTableLevel(depth);err!=nil{return err}}else if tableLevel>0{if err:=closeTableToDepth(0);err!=nil{return err}}
		text:=run.Text
		if run.Image!=nil||run.Shape!=nil{if err:=openParagraph(run);err!=nil{return err};ev:=runEvent(EventDrawing,run);ev.Image=run.Image;ev.Shape=run.Shape;if err:=handler(ev);err!=nil{return err};text=strings.ReplaceAll(text,"\x08","");text=strings.ReplaceAll(text,"\x01","")}
		for len(text)>0 {
			i:=strings.IndexAny(text,"\r\t\v\f\a")
			if i<0{if err:=openParagraph(run);err!=nil{return err};if err:=emitText(run,text);err!=nil{return err};break}
			if i>0{if err:=openParagraph(run);err!=nil{return err};if err:=emitText(run,text[:i]);err!=nil{return err}}
			ch:=text[i];text=text[i+1:]
			switch ch{
			case '\r':if err:=closeParagraph(run,true);err!=nil{return err}
			case '\a':if !run.ParagraphFormat.IsTableTrailingParagraph{if err:=closeParagraph(run,paragraphOpen);err!=nil{return err}};if tableLevel>0{level:=tableLevel-1;if cellOpen[level]{if err:=handler(Event{Type:EventEndTableCell});err!=nil{return err};cellOpen[level]=false};if run.ParagraphFormat.IsTableTrailingParagraph&&rowOpen[level]{if err:=handler(Event{Type:EventEndTableRow});err!=nil{return err};rowOpen[level]=false}}
			case '\t':if err:=openParagraph(run);err!=nil{return err};ev:=runEvent(EventTab,run);if err:=handler(ev);err!=nil{return err}
			case '\v':if err:=openParagraph(run);err!=nil{return err};ev:=runEvent(EventBreak,run);ev.BreakType="line";if err:=handler(ev);err!=nil{return err}
			case '\f':if err:=openParagraph(run);err!=nil{return err};ev:=runEvent(EventBreak,run);ev.BreakType="page";if err:=handler(ev);err!=nil{return err}
			}
		}
	}
	if paragraphOpen{if err:=closeParagraph(Run{},false);err!=nil{return err}}
	if err:=closeTableToDepth(0);err!=nil{return err}
	if context!=nil{for sectionIndex<len(context.Sections){sec:=context.Sections[sectionIndex];copySec:=sec;if err:=handler(Event{Type:EventSection,Section:&copySec});err!=nil{return err};sectionIndex++}}
	return handler(Event{Type:EventEndDocument})
}
