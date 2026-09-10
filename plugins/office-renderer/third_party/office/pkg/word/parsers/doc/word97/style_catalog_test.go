package word97

import (
	"testing"
	wordir "office/pkg/word/ir"
)

func intp(v int)*int{return &v}
func boolp(v bool)*bool{return &v}

func TestBuildStyleCatalogResolvesInheritanceAndDefaults(t *testing.T){
	raw:=[]RawStyle{
		{Index:0,STI:0,Kind:1,Name:"Normal",BasedOnIndex:-1,RawParaFormat:wordir.ParagraphFormat{SpacingAfter:intp(120)},RawRunFormat:wordir.RunFormat{FontSize:intp(22)},ParaSprms:[]byte{1},RunSprms:[]byte{2}},
		{Index:1,STI:1,Kind:1,Name:"Heading 1",BasedOnIndex:0,LinkedIndex:2,RawParaFormat:wordir.ParagraphFormat{KeepNext:true},RawRunFormat:wordir.RunFormat{Bold:boolp(true)}},
		{Index:2,STI:65,Kind:2,Name:"Default Paragraph Font",BasedOnIndex:-1,RawRunFormat:wordir.RunFormat{Italic:boolp(true)}},
	}
	c:=BuildStyleCatalog(raw);if c==nil{t.Fatal("nil catalog")};if c.DefaultParagraphStyleID!="Normal"{t.Fatalf("default paragraph=%q",c.DefaultParagraphStyleID)};if c.DefaultCharacterStyleID!="DefaultParagraphFont"{t.Fatalf("default character=%q",c.DefaultCharacterStyleID)}
	h:=c.ParagraphStyles["Heading1"];if h.BasedOn!="Normal"||h.LinkedCharID!="DefaultParagraphFont"{t.Fatalf("heading links=%+v",h)};if h.Format.SpacingAfter==nil||*h.Format.SpacingAfter!=120||!h.Format.KeepNext{t.Fatalf("effective para=%+v",h.Format)};if h.RunFormat.FontSize==nil||*h.RunFormat.FontSize!=22||h.RunFormat.Bold==nil||!*h.RunFormat.Bold{t.Fatalf("effective run=%+v",h.RunFormat)}
}

func TestStyleCatalogApplyToDocumentResolvesBinaryStyleIDs(t *testing.T){
	c:=BuildStyleCatalog([]RawStyle{{Index:0,STI:0,Kind:1,Name:"Normal",BasedOnIndex:-1},{Index:3,STI:65,Kind:2,Name:"Default Paragraph Font",BasedOnIndex:-1}});pidx:=0;ridx:=3;doc:=&wordir.Document{Paragraphs:[]wordir.Paragraph{{Source:&wordir.ParagraphSource{StyleIndex:&pidx},Runs:[]wordir.Run{{Source:&wordir.RunSource{StyleIndex:&ridx}}}}}};c.ApplyToDocument(doc);if doc.Paragraphs[0].StyleID!="Normal"{t.Fatalf("paragraph style=%q",doc.Paragraphs[0].StyleID)};if doc.Paragraphs[0].Runs[0].StyleID!="DefaultParagraphFont"{t.Fatalf("run style=%q",doc.Paragraphs[0].Runs[0].StyleID)}
}

func TestApplyParagraphStylePreservesDirectFormatting(t *testing.T){
	r:=Run{ParagraphFormat:wordir.ParagraphFormat{SpacingAfter:intp(240)}};ApplyParagraphStyle(&r,wordir.ParagraphFormat{Alignment:"center",SpacingAfter:intp(120),KeepNext:true});if r.ParagraphFormat.Alignment!="center"||r.ParagraphFormat.SpacingAfter==nil||*r.ParagraphFormat.SpacingAfter!=240||!r.ParagraphFormat.KeepNext{t.Fatalf("merged=%+v",r.ParagraphFormat)}
}


func TestStyleCatalogAppliesParagraphIdentityToHeaderFooterStories(t *testing.T) {
	index := 17
	style := wordir.Style{ID: "legacy-istd-17", Name: "Header"}
	c := &StyleCatalog{
		ParagraphByIndex: map[int]wordir.Style{17: style},
		ParagraphStyles:  map[string]wordir.Style{style.ID: style},
	}
	headerParagraph := wordir.Paragraph{
		Source: &wordir.ParagraphSource{StyleIndex: &index},
		Runs:   []wordir.Run{{Kind: "run", Text: "header"}},
	}
	footerParagraph := headerParagraph
	doc := &wordir.Document{
		Headers: map[string]wordir.Story{"h": {Paragraphs: []wordir.Paragraph{headerParagraph}, Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &headerParagraph}}}},
		Footers: map[string]wordir.Story{"f": {Paragraphs: []wordir.Paragraph{footerParagraph}, Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &footerParagraph}}}},
	}
	c.ApplyToDocument(doc)
	if got := doc.Headers["h"].Paragraphs[0].StyleID; got != style.ID {
		t.Fatalf("header StyleID = %q, want %q", got, style.ID)
	}
	if got := doc.Footers["f"].Paragraphs[0].StyleID; got != style.ID {
		t.Fatalf("footer StyleID = %q, want %q", got, style.ID)
	}
}
