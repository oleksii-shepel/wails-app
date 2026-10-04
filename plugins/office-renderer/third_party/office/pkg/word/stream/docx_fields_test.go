package stream

import (
	"bytes"
	"testing"
)

func TestDOCXLayoutPreservesBookmarkAndComplexFieldResult(t *testing.T) {
	data := makeDOCXBodyFixture(t, `<w:p><w:bookmarkStart w:id="1" w:name="target"/><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>7</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r><w:bookmarkEnd w:id="1"/></w:p>`)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{IncludeFieldInstructions: true})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %#v, want one paragraph", doc)
	}
	p := doc.Paragraphs[0]
	if len(p.Bookmarks) != 1 || p.Bookmarks[0] != "target" {
		t.Fatalf("bookmarks = %#v, want target", p.Bookmarks)
	}
	if len(p.Runs) != 1 {
		t.Fatalf("runs = %#v, want one visible field-result run", p.Runs)
	}
	if p.Runs[0].Text != "7" || p.Runs[0].FieldKind != "PAGE" || p.Runs[0].FieldInstr != "PAGE" {
		t.Fatalf("field result = %#v, want PAGE result 7", p.Runs[0])
	}
}

func TestDOCXRawConsumerStillReceivesFieldAndBookmarkEvents(t *testing.T) {
	data := makeDOCXBodyFixture(t, `<w:p><w:bookmarkStart w:id="1" w:name="target"/><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>7</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r><w:bookmarkEnd w:id="1"/></w:p>`)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{IncludeFieldInstructions: true})
	var bookmarks, instructions, fieldChars int
	if err := source.Walk(HandlerFunc(func(event Event) error {
		switch event.Type {
		case EventBookmarkStart:
			bookmarks++
		case EventFieldInstruction:
			instructions++
		case EventFieldChar:
			fieldChars++
		}
		return nil
	})); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if bookmarks != 1 || instructions != 1 || fieldChars != 3 {
		t.Fatalf("events bookmark/instr/fieldChars = %d/%d/%d, want 1/1/3", bookmarks, instructions, fieldChars)
	}
}

func TestDOCXLayoutPreservesCachedTOCEntriesAndNestedPageRefs(t *testing.T) {
	body := `<w:p>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText> TOC \o "1-3" \h \z \u </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:hyperlink w:anchor="_Toc1">` +
		`<w:r><w:t>Heading One</w:t></w:r>` +
		`<w:r><w:tab/></w:r>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText> PAGEREF _Toc1 \h </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>1</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:hyperlink></w:p>` +
		`<w:p><w:hyperlink w:anchor="_Toc2">` +
		`<w:r><w:t>Heading Two</w:t></w:r>` +
		`<w:r><w:tab/></w:r>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText> PAGEREF _Toc2 \h </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>2</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:hyperlink></w:p>` +
		`<w:p><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`
	data := makeDOCXBodyFixture(t, body)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{IncludeFieldInstructions: true})
	session := NewDocumentSession()
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) < 2 {
		t.Fatalf("paragraphs = %#v, want at least two TOC result paragraphs", doc)
	}

	if len(doc.Fields) != 1 || doc.Fields[0].Kind != "TOC" || len(doc.Fields[0].Blocks) != 2 {
		t.Fatalf("story fields = %#v, want one two-block TOC cached-result field", doc.Fields)
	}
	if len(doc.Blocks) < 2 || doc.Blocks[0].FieldKind != "TOC" || doc.Blocks[1].FieldKind != "TOC" || doc.Blocks[0].FieldIndex != 0 || doc.Blocks[1].FieldIndex != 0 {
		t.Fatalf("TOC block provenance = %#v, want first two blocks attached to field 0", doc.Blocks[:2])
	}

	for pi, p := range doc.Paragraphs {
		for ri, run := range p.Runs {
			if run.FieldKind == "TOC" {
				t.Fatalf("paragraph %d run %d = %#v: cached TOC result must not become an inline TOC field", pi, ri, run)
			}
		}
	}

	for i, want := range []struct {
		text   string
		page   string
		anchor string
	}{{"Heading One", "1", "_Toc1"}, {"Heading Two", "2", "_Toc2"}} {
		p := doc.Paragraphs[i]
		if len(p.Runs) != 3 {
			t.Fatalf("paragraph %d runs = %#v, want text/tab/PAGEREF result", i, p.Runs)
		}
		if p.Runs[0].Text != want.text || p.Runs[0].FieldKind != "" {
			t.Fatalf("paragraph %d entry run = %#v, want cached text without TOC semantics", i, p.Runs[0])
		}
		pageRun := p.Runs[2]
		if pageRun.Text != want.page || pageRun.FieldKind != "PAGEREF" || pageRun.Anchor != want.anchor {
			t.Fatalf("paragraph %d page-ref run = %#v, want PAGEREF %s -> %s", i, pageRun, want.anchor, want.page)
		}
	}
}
