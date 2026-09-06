package word97

import (
	"fmt"
	"io"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	docformat "office/pkg/word/parsers/doc/format"
)

type rangeCursor struct { inner Cursor; startCP, endCP int }
func (c *rangeCursor) Next() (Run, bool, error) {
	for {
		run,ok,err:=c.inner.Next(); if err!=nil||!ok{return run,ok,err}
		if run.Kind!=""{if run.StartCP>=c.startCP&&run.StartCP<c.endCP{return run,true,nil};continue}
		runes:=[]rune(run.Text); runEnd:=run.StartCP+len(runes)
		if runEnd<=c.startCP||run.StartCP>=c.endCP{continue}
		lo:=0;if c.startCP>run.StartCP{lo=c.startCP-run.StartCP};hi:=len(runes);if c.endCP<runEnd{hi=c.endCP-run.StartCP};if lo>=hi{continue}
		run.Text=string(runes[lo:hi]);run.StartCP+=lo;return run,true,nil
	}
}

type storyTableFrame struct { table wordir.Table; row *wordir.TableRow; cell *wordir.TableCell }
type storyBuilder struct { story wordir.Story; paragraph *wordir.Paragraph; tables []*storyTableFrame }

func MaterializeRunRangeFromCLX(word io.ReaderAt, wordSize int64, clx []byte, chpx []docformat.CHPXEntry, papx []docformat.PAPXEntry, r StoryRange) []Run {
	complex:=docbinary.ParseComplexFileTable(clx);if complex==nil||len(complex.PlcPcd)==0{return nil}
	return MaterializeRunRange(word,wordSize,&OpenCore{Complex:complex,CHPX:chpx,PAPX:papx},r)
}

func MaterializeRunRange(word io.ReaderAt, wordSize int64, opened *OpenCore, r StoryRange) []Run {
	if word==nil||opened==nil||opened.Complex==nil||r.StartCP>=r.EndCP{return nil}
	piece,err:=NewPieceReaderAtCursor(word,wordSize,opened.Complex.PlcPcd,opened.CHPX,opened.PAPX,nil);if err!=nil{return nil}
	var cursor Cursor=&rangeCursor{inner:piece,startCP:r.StartCP,endCP:r.EndCP}
	cursor=NewRevisionCursor(cursor,opened.RevisionAuthors)
	var out []Run
	for{run,ok,err:=cursor.Next();if err!=nil{return nil};if !ok{break};out=append(out,run)}
	return out
}

func MaterializeStoryRange(word io.ReaderAt, wordSize int64, opened *OpenCore, r StoryRange) wordir.Story {
	if word==nil||opened==nil||opened.Complex==nil||r.StartCP>=r.EndCP{return wordir.Story{}}
	piece,err:=NewPieceReaderAtCursor(word,wordSize,opened.Complex.PlcPcd,opened.CHPX,opened.PAPX,nil);if err!=nil{return wordir.Story{}}
	var cursor Cursor=&rangeCursor{inner:piece,startCP:r.StartCP,endCP:r.EndCP}
	cursor=NewRevisionCursor(cursor,opened.RevisionAuthors)
	cursor=NewFieldCursor(cursor,ParseFieldInstruction)
	if opened.Styles!=nil{cursor=NewStyleCursor(cursor,opened.Styles.ResolveParagraphFormat,nil)}
	b:=&storyBuilder{};if err:=Walk(cursor,nil,b.handle);err!=nil{return wordir.Story{}};return b.story
}

func storyRunFromEvent(event Event, kind, text string) wordir.Run {
	r:=wordir.Run{Kind:kind,Text:text,Revision:event.Revision,RevisionAuthor:event.RevisionAuthor,RevisionDate:event.RevisionDate,PropertyRevision:cloneStoryRevision(event.RunPropertyRevision),DisplayFieldRevision:cloneStoryRevision(event.DisplayFieldRevision),FieldKind:event.FieldKind,FieldInstr:event.FieldInstr,FieldArg:event.FieldArg,Target:event.Target,Anchor:event.Anchor,Source:event.RunSource}
	if event.RunFormat!=nil{r.Format=*event.RunFormat};return r
}
func cloneStoryRevision(mark *wordir.RevisionMark)*wordir.RevisionMark{if mark==nil{return nil};v:=*mark;return &v}

func (b *storyBuilder) handle(event Event) error {
	switch event.Type {
	case EventStartDocument,EventEndDocument,EventBookmarkEnd,EventSection:return nil
	case EventStartParagraph:
		if b.paragraph!=nil{if err:=b.finishParagraph();err!=nil{return err}}
		p:=&wordir.Paragraph{Source:event.ParagraphSource,PropertyRevision:cloneStoryRevision(event.ParagraphPropertyRevision)};if event.ParagraphFormat!=nil{p.Format=*event.ParagraphFormat};if p.Format.BinaryFrame!=nil{frame:=*p.Format.BinaryFrame;p.Frame=&frame;p.Format.BinaryFrame=nil};if event.ParagraphNumbering!=nil{p.Numbering=&wordir.Numbering{NumID:event.ParagraphNumbering.NumID,Level:event.ParagraphNumbering.Level}};b.paragraph=p;return nil
	case EventText:
		b.ensureParagraph();run:=storyRunFromEvent(event,"run",event.Text);b.paragraph.Text+=event.Text;b.paragraph.Runs=append(b.paragraph.Runs,run);return nil
	case EventTab:
		b.ensureParagraph();run:=storyRunFromEvent(event,"tab","\t");b.paragraph.Text+="\t";b.paragraph.Runs=append(b.paragraph.Runs,run);return nil
	case EventBreak:
		b.ensureParagraph();run:=storyRunFromEvent(event,"break","");run.BreakType=event.BreakType;b.paragraph.Runs=append(b.paragraph.Runs,run);return nil
	case EventDrawing:
		b.ensureParagraph();kind:="shape";if event.Image!=nil{kind="image"};run:=storyRunFromEvent(event,kind,"");run.Image=event.Image;run.Shape=event.Shape;b.paragraph.Runs=append(b.paragraph.Runs,run);return nil
	case EventEmbeddedObject:
		b.ensureParagraph();run:=storyRunFromEvent(event,"olePlaceholder","Embedded Object");run.OLEMIME="application/octet-stream";run.OLEType="Embed";if event.EmbeddedRun!=nil{run=*event.EmbeddedRun;run.Revision=event.Revision;run.RevisionAuthor=event.RevisionAuthor;run.RevisionDate=event.RevisionDate;run.PropertyRevision=cloneStoryRevision(event.RunPropertyRevision);run.DisplayFieldRevision=cloneStoryRevision(event.DisplayFieldRevision)};if event.RunSource!=nil{run.Source=event.RunSource};if event.RunFormat!=nil{run.Format=*event.RunFormat};b.paragraph.Runs=append(b.paragraph.Runs,run);return nil
	case EventBookmarkStart:
		b.ensureParagraph();if event.BookmarkName!=""{b.paragraph.Bookmarks=appendUniqueStoryValue(b.paragraph.Bookmarks,event.BookmarkName)};return nil
	case EventFootnoteReference,EventEndnoteReference:
		b.ensureParagraph();kind:="footnoteRef";if event.Type==EventEndnoteReference{kind="endnoteRef"};run:=storyRunFromEvent(event,kind,"");run.NoteID=event.NoteID;b.paragraph.Runs=append(b.paragraph.Runs,run);return nil
	case EventCommentReference:
		b.ensureParagraph();run:=storyRunFromEvent(event,"commentRef","");run.CommentID=event.CommentID;b.paragraph.Runs=append(b.paragraph.Runs,run);return nil
	case EventEndParagraph:return b.finishParagraph()
	case EventStartTable:
		if b.paragraph!=nil{if err:=b.finishParagraph();err!=nil{return err}};b.tables=append(b.tables,&storyTableFrame{});return nil
	case EventStartTableRow:
		f,err:=b.currentTable();if err!=nil{return err};if f.row!=nil{return fmt.Errorf("doc word97 story: row started before previous row ended")};f.row=&wordir.TableRow{};return nil
	case EventStartTableCell:
		f,err:=b.currentTable();if err!=nil{return err};if f.row==nil||f.cell!=nil{return fmt.Errorf("doc word97 story: invalid cell start")};f.cell=&wordir.TableCell{};return nil
	case EventEndTableCell:return b.finishCell()
	case EventEndTableRow:return b.finishRow()
	case EventEndTable:return b.finishTable()
	default:return nil
	}
}

func (b *storyBuilder) ensureParagraph(){if b.paragraph==nil{b.paragraph=&wordir.Paragraph{}}}
func (b *storyBuilder) topTable()*storyTableFrame{if len(b.tables)==0{return nil};return b.tables[len(b.tables)-1]}
func (b *storyBuilder) currentTable()(*storyTableFrame,error){f:=b.topTable();if f==nil{return nil,fmt.Errorf("doc word97 story: table boundary outside table")};return f,nil}
func (b *storyBuilder) finishParagraph()error{if b.paragraph==nil{return nil};p:=b.paragraph;b.paragraph=nil;p.Index=len(b.story.Paragraphs);b.story.Paragraphs=append(b.story.Paragraphs,*p);block:=wordir.Block{Kind:"paragraph",Paragraph:p};if f:=b.topTable();f!=nil{if f.cell==nil{return fmt.Errorf("doc word97 story: paragraph inside table outside cell")};block.Index=len(f.cell.Blocks);f.cell.Paragraphs=append(f.cell.Paragraphs,*p);f.cell.Blocks=append(f.cell.Blocks,block);f.cell.Text+=p.Text;return nil};block.Index=len(b.story.Blocks);b.story.Blocks=append(b.story.Blocks,block);b.story.Text+=p.Text;return nil}
func (b *storyBuilder) finishCell()error{if b.paragraph!=nil{if err:=b.finishParagraph();err!=nil{return err}};f,err:=b.currentTable();if err!=nil{return err};if f.row==nil||f.cell==nil{return fmt.Errorf("doc word97 story: cell ended without open cell")};f.table.Text+=f.cell.Text;f.row.Cells=append(f.row.Cells,*f.cell);f.cell=nil;return nil}
func (b *storyBuilder) finishRow()error{f,err:=b.currentTable();if err!=nil{return err};if f.cell!=nil{if err:=b.finishCell();err!=nil{return err}};if f.row==nil{return fmt.Errorf("doc word97 story: row ended without open row")};f.table.Rows=append(f.table.Rows,*f.row);f.row=nil;return nil}
func (b *storyBuilder) finishTable()error{if b.paragraph!=nil{if err:=b.finishParagraph();err!=nil{return err}};f,err:=b.currentTable();if err!=nil{return err};if f.cell!=nil{if err:=b.finishCell();err!=nil{return err}};if f.row!=nil{if err:=b.finishRow();err!=nil{return err}};table:=f.table;b.tables=b.tables[:len(b.tables)-1];block:=wordir.Block{Kind:"table",Table:&table};if parent:=b.topTable();parent!=nil{if parent.cell==nil{return fmt.Errorf("doc word97 story: nested table outside parent cell")};block.Index=len(parent.cell.Blocks);parent.cell.Tables=append(parent.cell.Tables,table);parent.cell.Blocks=append(parent.cell.Blocks,block);return nil};table.Index=len(b.story.Tables);block.Index=len(b.story.Blocks);b.story.Tables=append(b.story.Tables,table);b.story.Blocks=append(b.story.Blocks,block);b.story.Text+=table.Text;return nil}
func appendUniqueStoryValue(values []string,value string)[]string{for _,current:=range values{if current==value{return values}};return append(values,value)}
