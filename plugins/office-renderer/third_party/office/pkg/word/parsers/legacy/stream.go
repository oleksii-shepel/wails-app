package legacy

import (
	"encoding/binary"
	"fmt"
	"strings"

	"office/pkg/ooxml/cfb"
	wordevent "office/pkg/word/event"
	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

type StreamEventType = wordevent.Type

const (
	StreamStartDocument = wordevent.StartDocument
	StreamEndDocument = wordevent.EndDocument
	StreamStartParagraph = wordevent.StartParagraph
	StreamEndParagraph = wordevent.EndParagraph
	StreamStartTable = wordevent.StartTable
	StreamEndTable = wordevent.EndTable
	StreamStartTableRow = wordevent.StartTableRow
	StreamEndTableRow = wordevent.EndTableRow
	StreamStartTableCell = wordevent.StartTableCell
	StreamEndTableCell = wordevent.EndTableCell
	StreamText = wordevent.Text
	StreamTab = wordevent.Tab
	StreamBreak = wordevent.Break
	StreamDrawing = wordevent.Drawing
	StreamEmbeddedObject = wordevent.EmbeddedObject
	StreamBookmarkStart = wordevent.BookmarkStart
	StreamBookmarkEnd = wordevent.BookmarkEnd
	StreamFootnoteReference = wordevent.FootnoteReference
	StreamEndnoteReference = wordevent.EndnoteReference
	StreamCommentReference = wordevent.CommentReference
	StreamSection = wordevent.Section
)

type StreamUnsupportedError struct{ Feature string }
func (e StreamUnsupportedError) Error() string { return fmt.Sprintf("legacy doc stream: unsupported %s", e.Feature) }

type StreamEvent = wordevent.Event
type StreamHandler = wordevent.Handler

func validateLegacyStreamRuns(runs []docRun, textBoxCount int) error { for _,run:=range runs{if err:=validateLegacyStreamRun(run);err!=nil{return err}};if textBoxCount>0{return StreamUnsupportedError{Feature:"textbox"}};return nil }
func legacyStreamTableDepth(run docRun) int { if run.TableDepth>0{return run.TableDepth};if run.ParaFormat.InTable||run.LegacyInTableHint||strings.ContainsRune(run.Text,'\a'){return 1};return 0 }

func WalkDocFile(path string, handler StreamHandler) error {
	if handler==nil{return fmt.Errorf("legacy doc stream: handler cannot be nil")}
	f,err:=cfb.OpenFileBacked(path);if err!=nil{return fmt.Errorf("legacy doc stream: open cfb: %w",err)};defer f.Close()
	wordReader,wordSize,ok,readErr:=f.StreamReaderAt("WordDocument");if !ok&&readErr==nil{wordReader,wordSize,ok,readErr=f.StreamReaderAt("Ole")};if readErr!=nil{return fmt.Errorf("legacy doc stream: open WordDocument reader: %w",readErr)}
	if ok&&wordSize>=18{header:=make([]byte,18);n,err:=wordReader.ReadAt(header,0);if err==nil&&n==len(header)&&binary.LittleEndian.Uint16(header[0:2])==legacyDocFibSignature{flags:=binary.LittleEndian.Uint16(header[10:12]);if flags&docFlagEncrypted==0{if version,valid:=legacyDocFibVersion(header);valid&&version>=legacyDocFibWord97{return walkCanonicalWord97ReaderAt(wordReader,wordSize,f,handler)}}}}
	return walkDocCFB(f,handler)
}

func WalkDocBytes(data []byte, handler StreamHandler) error { if handler==nil{return fmt.Errorf("legacy doc stream: handler cannot be nil")};f,err:=cfb.OpenBytes(data);if err!=nil{return fmt.Errorf("legacy doc stream: open cfb: %w",err)};return walkDocCFB(f,handler) }

func walkCanonicalWord97ReaderAt(wordReader interface{ ReadAt([]byte, int64) (int, error) }, wordSize int64, f *cfb.File, handler StreamHandler) error {
	cursor,context,_,err:=word97.OpenReaderAt(wordReader,wordSize,f)
	if err!=nil{return err}
	return word97.Walk(cursor,context,handler)
}

func walkDocCFB(f *cfb.File, handler StreamHandler) error {
	if f==nil{return fmt.Errorf("legacy doc stream: nil cfb")};var err error;f,err=maybeDecryptDocStreams(f);if err!=nil{return err}
	wordStream,ok:=f.Stream("WordDocument");if !ok{wordStream,ok=f.Stream("Ole")};if !ok{return fmt.Errorf("legacy doc stream: missing WordDocument stream")}
	version,valid:=legacyDocFibVersion(wordStream)
	if valid&&version>=legacyDocFibWord97{
		wordReader,wordSize,readerOK,readErr:=f.StreamReaderAt("WordDocument");if !readerOK&&readErr==nil{wordReader,wordSize,readerOK,readErr=f.StreamReaderAt("Ole")};if readErr!=nil{return readErr};if !readerOK{return fmt.Errorf("legacy doc stream: missing Word97 ReaderAt stream")}
		return walkCanonicalWord97ReaderAt(wordReader,wordSize,f,handler)
	}
	var cursor legacyRunCursor;var core *legacyDocStreamCore
	if valid&&version==legacyDocFibWord95||legacyDocLooksPreWord97(wordStream){
		cursor,core,err=newLegacyPreWord97StreamCursor(wordStream,f)
		if err!=nil{return err}
	}else{
		return fmt.Errorf("legacy doc stream: unsupported or unreadable FIB")
	}
	return walkLegacyCursorWithCore(cursor,handler,core)
}

func walkLegacyCursor(cursor legacyRunCursor, handler StreamHandler) error { return walkLegacyCursorWithCore(cursor,handler,nil) }

func walkLegacyCursorWithCore(cursor legacyRunCursor, handler StreamHandler, core *legacyDocStreamCore) error {
	if cursor==nil{return fmt.Errorf("legacy doc stream: nil run cursor")}
	start := StreamEvent{Type:StreamStartDocument}
	if core!=nil {
		start.Headers=core.Headers; start.Footers=core.Footers
		if core.FootnoteSeparator.Text!=""{s:=core.FootnoteSeparator;start.FootnoteSeparator=&s}
		if core.FootnoteContinuationSep.Text!=""{s:=core.FootnoteContinuationSep;start.FootnoteContinuationSep=&s}
		if core.FootnoteContinuationNotice.Text!=""{s:=core.FootnoteContinuationNotice;start.FootnoteContinuationNotice=&s}
		if core.EndnoteSeparator.Text!=""{s:=core.EndnoteSeparator;start.EndnoteSeparator=&s}
		if core.EndnoteContinuationSep.Text!=""{s:=core.EndnoteContinuationSep;start.EndnoteContinuationSep=&s}
		if core.EndnoteContinuationNotice.Text!=""{s:=core.EndnoteContinuationNotice;start.EndnoteContinuationNotice=&s}
	}
	if err:=handler(start);err!=nil{return err}
	paragraphOpen:=false;tableDepth:=0;rowOpen:=make([]bool,0,2);cellOpen:=make([]bool,0,2);sectionIndex:=0
	openParagraph:=func(run docRun)error{if paragraphOpen{return nil};paragraphOpen=true;formatCopy:=run.ParaFormat;return handler(StreamEvent{Type:StreamStartParagraph,ParagraphFormat:&formatCopy,ParagraphNumbering:legacyStreamNumberingReference(run)})}
	closeParagraph:=func(run docRun,force bool)error{if !paragraphOpen{if !force{return nil};if err:=openParagraph(run);err!=nil{return err}};paragraphOpen=false;return handler(StreamEvent{Type:StreamEndParagraph})}
	ensureTableLevel:=func(depth int)error{for tableDepth<depth{if err:=handler(StreamEvent{Type:StreamStartTable});err!=nil{return err};tableDepth++;rowOpen=append(rowOpen,false);cellOpen=append(cellOpen,false)};if depth>0{level:=depth-1;if !rowOpen[level]{if err:=handler(StreamEvent{Type:StreamStartTableRow});err!=nil{return err};rowOpen[level]=true};if !cellOpen[level]{if err:=handler(StreamEvent{Type:StreamStartTableCell});err!=nil{return err};cellOpen[level]=true}};return nil}
	closeTableToDepth:=func(depth int)error{if paragraphOpen{if err:=closeParagraph(docRun{},false);err!=nil{return err}};for tableDepth>depth{level:=tableDepth-1;if cellOpen[level]{if err:=handler(StreamEvent{Type:StreamEndTableCell});err!=nil{return err};cellOpen[level]=false};if rowOpen[level]{if err:=handler(StreamEvent{Type:StreamEndTableRow});err!=nil{return err};rowOpen[level]=false};if err:=handler(StreamEvent{Type:StreamEndTable});err!=nil{return err};tableDepth--;rowOpen=rowOpen[:tableDepth];cellOpen=cellOpen[:tableDepth]};return nil}
	emitSectionsThrough:=func(cp int)error{if core==nil{return nil};for sectionIndex<len(core.Sections){sec:=core.Sections[sectionIndex];end:=0;if sec.Source!=nil{end=sec.Source.EndCP};if end>cp{return nil};if err:=closeTableToDepth(0);err!=nil{return err};copySec:=sec;if err:=handler(StreamEvent{Type:StreamSection,Section:&copySec});err!=nil{return err};sectionIndex++};return nil}
	emitText:=func(run docRun,text string)error{if text==""{return nil};formatCopy:=run.Format;return handler(StreamEvent{Type:StreamText,Text:text,RunFormat:&formatCopy,FieldKind:run.FieldKind,FieldInstr:run.FieldInstr,FieldArg:run.FieldArg,Target:run.Target,Anchor:run.Anchor})}
	for{run,ok,err:=cursor.Next();if err!=nil{return err};if !ok{break};if err:=validateLegacyStreamRun(run);err!=nil{return err};if err:=emitSectionsThrough(run.StartCP);err!=nil{return err}
		if run.Kind=="bookmarkStart"||run.Kind=="bookmarkEnd"{if err:=openParagraph(run);err!=nil{return err};typeID:=StreamBookmarkStart;if run.Kind=="bookmarkEnd"{typeID=StreamBookmarkEnd};if err:=handler(StreamEvent{Type:typeID,BookmarkName:run.Anchor});err!=nil{return err};continue}
		if run.Kind=="olePlaceholder"{if err:=openParagraph(run);err!=nil{return err};var embedded *wordir.Run;if core!=nil&&core.OLEByCP!=nil{if value:=core.OLEByCP[run.StartCP];value!=nil{copyRun:=*value;embedded=&copyRun}};if embedded==nil{embedded=&wordir.Run{Kind:"olePlaceholder",Text:"Embedded Object",OLEMIME:"application/octet-stream",OLEType:"Embed"}};formatCopy:=run.Format;embedded.Format=formatCopy;if err:=handler(StreamEvent{Type:StreamEmbeddedObject,RunFormat:&formatCopy,EmbeddedRun:embedded});err!=nil{return err};continue}
		if run.Kind=="footnoteRef"||run.Kind=="endnoteRef"||run.Kind=="commentRef"{if err:=openParagraph(run);err!=nil{return err};formatCopy:=run.Format;ev:=StreamEvent{RunFormat:&formatCopy};if run.Kind=="footnoteRef"{ev.Type=StreamFootnoteReference;ev.NoteID=run.NoteID;if core!=nil{if story,ok:=core.Footnotes[run.NoteID];ok{s:=story;ev.Story=&s}}}else if run.Kind=="endnoteRef"{ev.Type=StreamEndnoteReference;ev.NoteID=run.NoteID;if core!=nil{if story,ok:=core.Endnotes[run.NoteID];ok{s:=story;ev.Story=&s}}}else{ev.Type=StreamCommentReference;ev.CommentID=run.CommentID;if core!=nil{if story,ok:=core.Comments[run.CommentID];ok{s:=story;ev.Story=&s}}};if err:=handler(ev);err!=nil{return err};continue}
		depth:=legacyStreamTableDepth(run);if depth<tableDepth{if err:=closeTableToDepth(depth);err!=nil{return err}};if depth>0{if err:=ensureTableLevel(depth);err!=nil{return err}}else if tableDepth>0{if err:=closeTableToDepth(0);err!=nil{return err}}
		text:=run.Text;if run.Image!=nil||run.Shape!=nil{if err:=openParagraph(run);err!=nil{return err};formatCopy:=run.Format;if err:=handler(StreamEvent{Type:StreamDrawing,RunFormat:&formatCopy,Image:run.Image,Shape:run.Shape});err!=nil{return err};text=strings.ReplaceAll(text,string(legacyDrawingAnchorChar),"");text=strings.ReplaceAll(text,"\x01","")}
		for len(text)>0{i:=strings.IndexAny(text,"\r\t\v\f\a");if i<0{if err:=openParagraph(run);err!=nil{return err};if err:=emitText(run,text);err!=nil{return err};break};if i>0{if err:=openParagraph(run);err!=nil{return err};if err:=emitText(run,text[:i]);err!=nil{return err}};ch:=text[i];text=text[i+1:];switch ch{case '\r':if err:=closeParagraph(run,true);err!=nil{return err};case '\a':if !run.ParaFormat.IsTableTrailingParagraph{if err:=closeParagraph(run,paragraphOpen);err!=nil{return err}};if tableDepth>0{level:=tableDepth-1;if cellOpen[level]{if err:=handler(StreamEvent{Type:StreamEndTableCell});err!=nil{return err};cellOpen[level]=false};if run.ParaFormat.IsTableTrailingParagraph&&rowOpen[level]{if err:=handler(StreamEvent{Type:StreamEndTableRow});err!=nil{return err};rowOpen[level]=false}};case '\t':if err:=openParagraph(run);err!=nil{return err};formatCopy:=run.Format;if err:=handler(StreamEvent{Type:StreamTab,RunFormat:&formatCopy});err!=nil{return err};case '\v':if err:=openParagraph(run);err!=nil{return err};formatCopy:=run.Format;if err:=handler(StreamEvent{Type:StreamBreak,BreakType:"line",RunFormat:&formatCopy});err!=nil{return err};case '\f':if err:=openParagraph(run);err!=nil{return err};formatCopy:=run.Format;if err:=handler(StreamEvent{Type:StreamBreak,BreakType:"page",RunFormat:&formatCopy});err!=nil{return err}}}}
	if paragraphOpen{if err:=closeParagraph(docRun{},false);err!=nil{return err}};if err:=closeTableToDepth(0);err!=nil{return err};if core!=nil{for sectionIndex<len(core.Sections){sec:=core.Sections[sectionIndex];copySec:=sec;if err:=handler(StreamEvent{Type:StreamSection,Section:&copySec});err!=nil{return err};sectionIndex++}}
	return handler(StreamEvent{Type:StreamEndDocument})
}
