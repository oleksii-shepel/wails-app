package doc

import (
	"fmt"

	wordir "office/pkg/word/ir"
)

type materializeTableFrame struct {
	table wordir.Table
	row   *wordir.TableRow
	cell  *wordir.TableCell
}

type materializer struct {
	doc       wordir.Document
	paragraph *wordir.Paragraph
	tables    []*materializeTableFrame
}

func newMaterializer(name string) *materializer { return &materializer{doc: wordir.Document{Filename: name}} }
func (m *materializer) Document() *wordir.Document { if m == nil { return nil }; return &m.doc }

func runFromEvent(event StreamEvent, kind, text string) wordir.Run {
	r := wordir.Run{
		Kind: kind, Text: text,
		Revision: event.Revision, RevisionAuthor: event.RevisionAuthor, RevisionDate: event.RevisionDate,
		PropertyRevision: cloneRevisionMark(event.RunPropertyRevision),
		DisplayFieldRevision: cloneRevisionMark(event.DisplayFieldRevision),
		FieldKind:event.FieldKind, FieldInstr:event.FieldInstr, FieldArg:event.FieldArg,
		Target:event.Target, Anchor:event.Anchor, Source:event.RunSource,
	}
	if event.RunFormat != nil { r.Format = *event.RunFormat }
	return r
}

func cloneRevisionMark(mark *wordir.RevisionMark) *wordir.RevisionMark {
	if mark == nil { return nil }
	copyMark := *mark
	return &copyMark
}

func (m *materializer) Handle(event StreamEvent) error {
	if m == nil { return nil }
	switch event.Type {
	case StreamStartDocument:
		if len(event.Headers) > 0 { m.doc.Headers = cloneStoryMap(event.Headers) }
		if len(event.Footers) > 0 { m.doc.Footers = cloneStoryMap(event.Footers) }
		if event.FootnoteSeparator != nil { m.doc.FootnoteSeparator = *event.FootnoteSeparator }
		if event.FootnoteContinuationSep != nil { m.doc.FootnoteContinuationSep = *event.FootnoteContinuationSep }
		if event.FootnoteContinuationNotice != nil { m.doc.FootnoteContinuationNotice = *event.FootnoteContinuationNotice }
		if event.EndnoteSeparator != nil { m.doc.EndnoteSeparator = *event.EndnoteSeparator }
		if event.EndnoteContinuationSep != nil { m.doc.EndnoteContinuationSep = *event.EndnoteContinuationSep }
		if event.EndnoteContinuationNotice != nil { m.doc.EndnoteContinuationNotice = *event.EndnoteContinuationNotice }
		return nil
	case StreamStartParagraph:
		if m.paragraph != nil { if err := m.finishParagraph(); err != nil { return err } }
		p := &wordir.Paragraph{Source:event.ParagraphSource, PropertyRevision:cloneRevisionMark(event.ParagraphPropertyRevision)}
		if event.ParagraphFormat != nil { p.Format = *event.ParagraphFormat }
		if p.Format.BinaryFrame != nil { frame:=*p.Format.BinaryFrame; p.Frame=&frame; p.Format.BinaryFrame=nil }
		if event.ParagraphNumbering != nil { p.Numbering = &wordir.Numbering{NumID:event.ParagraphNumbering.NumID, Level:event.ParagraphNumbering.Level} }
		m.paragraph = p
		return nil
	case StreamText:
		m.ensureParagraph(); r:=runFromEvent(event,"run",event.Text); m.paragraph.Text+=event.Text; m.paragraph.Runs=append(m.paragraph.Runs,r); return nil
	case StreamTab:
		m.ensureParagraph(); r:=runFromEvent(event,"tab","\t"); m.paragraph.Text+="\t"; m.paragraph.Runs=append(m.paragraph.Runs,r); return nil
	case StreamBreak:
		m.ensureParagraph(); r:=runFromEvent(event,"break",""); r.BreakType=event.BreakType; m.paragraph.Runs=append(m.paragraph.Runs,r); return nil
	case StreamDrawing:
		m.ensureParagraph(); kind:="shape"; if event.Image!=nil{kind="image"}; r:=runFromEvent(event,kind,""); r.Image=event.Image; r.Shape=event.Shape; m.paragraph.Runs=append(m.paragraph.Runs,r); return nil
	case StreamEmbeddedObject:
		m.ensureParagraph(); r:=runFromEvent(event,"olePlaceholder","Embedded Object"); r.OLEMIME="application/octet-stream"; r.OLEType="Embed"
		if event.EmbeddedRun != nil { embedded:=*event.EmbeddedRun; embedded.Revision=event.Revision; embedded.RevisionAuthor=event.RevisionAuthor; embedded.RevisionDate=event.RevisionDate; embedded.PropertyRevision=cloneRevisionMark(event.RunPropertyRevision); embedded.DisplayFieldRevision=cloneRevisionMark(event.DisplayFieldRevision); r=embedded }
		if event.RunSource != nil { r.Source=event.RunSource }; if event.RunFormat != nil { r.Format=*event.RunFormat }; m.paragraph.Runs=append(m.paragraph.Runs,r); return nil
	case StreamBookmarkStart:
		m.ensureParagraph(); if event.BookmarkName!=""{m.paragraph.Bookmarks=appendUnique(m.paragraph.Bookmarks,event.BookmarkName)}; return nil
	case StreamBookmarkEnd:
		return nil
	case StreamFootnoteReference, StreamEndnoteReference:
		m.ensureParagraph(); kind:="footnoteRef"; if event.Type==StreamEndnoteReference{kind="endnoteRef"}; r:=runFromEvent(event,kind,""); r.NoteID=event.NoteID; m.paragraph.Runs=append(m.paragraph.Runs,r)
		if event.Story!=nil{if event.Type==StreamFootnoteReference{if m.doc.Footnotes==nil{m.doc.Footnotes=map[int]wordir.Story{}};m.doc.Footnotes[event.NoteID]=*event.Story}else{if m.doc.Endnotes==nil{m.doc.Endnotes=map[int]wordir.Story{}};m.doc.Endnotes[event.NoteID]=*event.Story}}; return nil
	case StreamCommentReference:
		m.ensureParagraph(); r:=runFromEvent(event,"commentRef",""); r.CommentID=event.CommentID; m.paragraph.Runs=append(m.paragraph.Runs,r); if event.Story!=nil{if m.doc.Comments==nil{m.doc.Comments=map[int]wordir.Story{}};m.doc.Comments[event.CommentID]=*event.Story}; return nil
	case StreamEndParagraph:
		return m.finishParagraph()
	case StreamStartTable:
		if m.paragraph != nil { if err := m.finishParagraph(); err != nil { return err } }; m.tables=append(m.tables,&materializeTableFrame{}); return nil
	case StreamStartTableRow:
		f,err:=m.currentTable(); if err!=nil{return err}; if f.row!=nil{return fmt.Errorf("doc materialize: table row started before previous row ended")}; f.row=&wordir.TableRow{}; return nil
	case StreamStartTableCell:
		f,err:=m.currentTable(); if err!=nil{return err}; if f.row==nil{return fmt.Errorf("doc materialize: cell started outside row")}; if f.cell!=nil{return fmt.Errorf("doc materialize: cell started before previous cell ended")}; f.cell=&wordir.TableCell{}; return nil
	case StreamEndTableCell:
		return m.finishCell()
	case StreamEndTableRow:
		return m.finishRow()
	case StreamEndTable:
		return m.finishTable()
	case StreamSection:
		if m.paragraph!=nil{if err:=m.finishParagraph();err!=nil{return err}}; if len(m.tables)>0{return fmt.Errorf("doc materialize: section encountered inside table")}; if event.Section!=nil{sec:=*event.Section;m.doc.Sections=append(m.doc.Sections,sec);copySec:=sec;m.doc.Blocks=append(m.doc.Blocks,wordir.Block{Index:len(m.doc.Blocks),Kind:"section",Section:&copySec})}; return nil
	case StreamEndDocument:
		if m.paragraph!=nil{if err:=m.finishParagraph();err!=nil{return err}}; if len(m.tables)!=0{return fmt.Errorf("doc materialize: document ended with %d open table(s)",len(m.tables))}; return nil
	default:
		return fmt.Errorf("doc materialize: unsupported event %q",event.Type)
	}
}

func (m *materializer) ensureParagraph(){ if m.paragraph==nil { m.paragraph=&wordir.Paragraph{} } }
func (m *materializer) topTable()*materializeTableFrame{if len(m.tables)==0{return nil};return m.tables[len(m.tables)-1]}
func (m *materializer) currentTable()(*materializeTableFrame,error){f:=m.topTable();if f==nil{return nil,fmt.Errorf("doc materialize: table boundary outside table")};return f,nil}

func (m *materializer) finishParagraph() error {
	if m.paragraph==nil{return nil}; p:=m.paragraph; m.paragraph=nil; p.Index=len(m.doc.Paragraphs); m.doc.Paragraphs=append(m.doc.Paragraphs,*p); block:=wordir.Block{Kind:"paragraph",Paragraph:p}
	if f:=m.topTable();f!=nil{if f.cell==nil{return fmt.Errorf("doc materialize: paragraph inside table outside cell")};block.Index=len(f.cell.Blocks);f.cell.Paragraphs=append(f.cell.Paragraphs,*p);f.cell.Blocks=append(f.cell.Blocks,block);f.cell.Text+=p.Text;return nil}
	block.Index=len(m.doc.Blocks);m.doc.Blocks=append(m.doc.Blocks,block);return nil
}
func (m *materializer) finishCell()error{if m.paragraph!=nil{if err:=m.finishParagraph();err!=nil{return err}};f,err:=m.currentTable();if err!=nil{return err};if f.row==nil||f.cell==nil{return fmt.Errorf("doc materialize: cell ended without open cell")};f.table.Text+=f.cell.Text;f.row.Cells=append(f.row.Cells,*f.cell);f.cell=nil;return nil}
func (m *materializer) finishRow()error{f,err:=m.currentTable();if err!=nil{return err};if f.cell!=nil{if err:=m.finishCell();err!=nil{return err}};if f.row==nil{return fmt.Errorf("doc materialize: row ended without open row")};f.table.Rows=append(f.table.Rows,*f.row);f.row=nil;return nil}
func (m *materializer) finishTable()error{if m.paragraph!=nil{if err:=m.finishParagraph();err!=nil{return err}};f,err:=m.currentTable();if err!=nil{return err};if f.cell!=nil{if err:=m.finishCell();err!=nil{return err}};if f.row!=nil{if err:=m.finishRow();err!=nil{return err}};table:=f.table;table.Index=len(m.doc.Tables);m.tables=m.tables[:len(m.tables)-1];block:=wordir.Block{Kind:"table",Table:&table};if parent:=m.topTable();parent!=nil{if parent.cell==nil{return fmt.Errorf("doc materialize: nested table ended outside parent cell")};block.Index=len(parent.cell.Blocks);parent.cell.Tables=append(parent.cell.Tables,table);parent.cell.Blocks=append(parent.cell.Blocks,block);return nil};block.Index=len(m.doc.Blocks);m.doc.Tables=append(m.doc.Tables,table);m.doc.Blocks=append(m.doc.Blocks,block);return nil}

func cloneStoryMap(src map[string]wordir.Story)map[string]wordir.Story{if len(src)==0{return nil};out:=make(map[string]wordir.Story,len(src));for k,v:=range src{out[k]=v};return out}
func appendUnique(values []string,value string)[]string{for _,v:=range values{if v==value{return values}};return append(values,value)}
