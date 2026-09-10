package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

func TestApplySnapshotDocumentStateProjectsCanonicalStructureWithoutAliasing(t *testing.T) {
	ctx := &Context{
		Sections: []wordir.Section{{
			Headers: []wordir.Reference{{PartName: "h1"}},
			Footers: []wordir.Reference{{PartName: "f1"}},
		}},
		Headers:   map[string]wordir.Story{"h1": {Text: "header"}},
		Footers:   map[string]wordir.Story{"f1": {Text: "footer"}},
		Footnotes: map[int]wordir.Story{1: {Text: "note"}},
	}
	snapshot := &Snapshot{
		Core: &OpenCore{
			Metadata: Metadata{BookmarkEnds: map[int][]string{9: {"end"}}},
			Settings: DocumentSettings{DefaultTabStop: 720, Compatibility: &wordir.DocCompatibility{}},
			Layout:   docbinary.Word97Layout{Version: 0x00C1},
			RawDOP:   []byte{1, 2, 3},
			Source:   &wordir.DocumentSource{ComplexFileTable: []byte{4, 5}},
		},
		Context: ctx,
	}
	doc := &wordir.Document{}
	ApplySnapshotDocumentState(doc, snapshot)

	if len(doc.Sections) != 1 || doc.Sections[0].Headers[0].PartName != "h1" {
		t.Fatalf("sections=%#v", doc.Sections)
	}
	if doc.Headers["h1"].Text != "header" || doc.Footers["f1"].Text != "footer" {
		t.Fatalf("stories=%#v/%#v", doc.Headers, doc.Footers)
	}
	if doc.Footnotes[1].Text != "note" || doc.BookmarkEnds["end"] != 9 {
		t.Fatalf("notes/bookmarks=%#v/%#v", doc.Footnotes, doc.BookmarkEnds)
	}
	if doc.DefaultTabStop != 720 || doc.Source == nil || doc.Source.Version != 0x00C1 {
		t.Fatalf("settings/source=%#v/%#v", doc.DefaultTabStop, doc.Source)
	}

	doc.Sections[0].Headers[0].PartName = "changed"
	if ctx.Sections[0].Headers[0].PartName != "h1" {
		t.Fatal("canonical section container was aliased")
	}
	doc.Source.DOP[0] = 9
	if snapshot.Core.RawDOP[0] != 1 {
		t.Fatal("canonical DOP payload was aliased")
	}
}

func TestApplySnapshotDocumentStateInsertsSectionBoundaries(t *testing.T) {
	doc := &wordir.Document{
		Blocks: []wordir.Block{
			{Kind: "paragraph", Paragraph: &wordir.Paragraph{Source: &wordir.ParagraphSource{StartCP: 0, EndCP: 5}}},
			{Kind: "paragraph", Paragraph: &wordir.Paragraph{Source: &wordir.ParagraphSource{StartCP: 10, EndCP: 15}}},
		},
	}
	snapshot := &Snapshot{
		Core: &OpenCore{},
		Context: &Context{Sections: []wordir.Section{
			{Source: &wordir.SectionSource{StartCP: 0, EndCP: 10}},
			{Source: &wordir.SectionSource{StartCP: 10, EndCP: 20}},
		}},
	}
	ApplySnapshotDocumentState(doc, snapshot)
	if len(doc.Blocks) != 3 || doc.Blocks[1].Kind != "section" {
		t.Fatalf("blocks=%#v", doc.Blocks)
	}
}
