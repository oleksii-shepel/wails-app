package sax_test

import (
	"archive/zip"
	"bytes"
	"strconv"
	"testing"

	"office/pkg/word/parsers/ooxml/sax"
)

func TestWalkBytesEmitsDocumentEvents(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>Hello</w:t></w:r>
      <w:hyperlink>
        <w:r><w:t> world</w:t></w:r>
      </w:hyperlink>
      <w:r><w:tab/></w:r>
      <w:r><w:br w:type="page"/></w:r>
      <w:fldSimple w:instr='MERGEFIELD Name'>
        <w:r><w:t>Alice</w:t></w:r>
      </w:fldSimple>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc>
          <w:p><w:r><w:t>Cell</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
	})

	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		events = append(events, event)
		return nil
	}), sax.Options{IncludeFieldInstructions: true})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}

	got := simplify(events)
	want := []string{
		"startStory:document:word/document.xml",
		"startDocument",
		"startParagraph:Heading1",
		"text:Hello",
		"startHyperlink:::",
		"text: world",
		"endHyperlink:::",
		"tab",
		"break:page",
		"fieldInstruction:MERGEFIELD Name",
		"text:Alice",
		"endParagraph:Heading1",
		"startTable",
		"startTableRow",
		"startTableCell",
		"startParagraph:",
		"text:Cell",
		"endParagraph:",
		"endTableCell",
		"endTableRow",
		"endTable",
		"endDocument",
		"endStory:document:word/document.xml",
	}
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d\n got: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q\nfull: %#v", i, got[i], want[i], got)
		}
	}
}

func TestWalkBytesCanOmitFieldInstructions(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r>
      <w:r><w:t>Alice</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`,
	})

	var fieldInstructions int
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		if event.Type == sax.EventFieldInstruction {
			fieldInstructions++
		}
		return nil
	}), sax.Options{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}
	if fieldInstructions != 0 {
		t.Fatalf("field instruction events = %d, want 0", fieldInstructions)
	}
}

func TestWalkStreamEmitsFieldInstructionRunContent(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r>
      <w:r><w:t>Alice</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`,
	})

	var instructions []string
	var texts []string
	err := sax.WalkStream(bytes.NewBuffer(data), sax.HandlerFunc(func(event sax.Event) error {
		switch event.Type {
		case sax.EventFieldInstruction:
			instructions = append(instructions, event.FieldInstr)
		case sax.EventText:
			texts = append(texts, event.Text)
		}
		return nil
	}), sax.Options{IncludeFieldInstructions: true})
	if err != nil {
		t.Fatalf("WalkStream: %v", err)
	}
	if len(instructions) != 1 || instructions[0] != " MERGEFIELD Name " {
		t.Fatalf("instructions = %#v, want raw instrText payload", instructions)
	}
	if len(texts) != 1 || texts[0] != "Alice" {
		t.Fatalf("texts = %#v, want [Alice]", texts)
	}
}

func TestWalkBytesIncludesHeadersAndFooters(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
    <w:sectPr>
      <w:headerReference w:type="default" r:id="rId10"/>
      <w:footerReference w:type="default" r:id="rId11"/>
    </w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
  <Relationship Id="rId11" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
</Relationships>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Header</w:t></w:r></w:p>
</w:hdr>`,
		"word/footer1.xml": `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Footer</w:t></w:r></w:p>
</w:ftr>`,
	})

	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		events = append(events, event)
		return nil
	}), sax.Options{IncludeHeadersAndFooters: true})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}

	got := simplify(events)
	want := []string{
		"startStory:document:word/document.xml",
		"startDocument",
		"startParagraph:",
		"text:Body",
		"endParagraph:",
		"endDocument",
		"endStory:document:word/document.xml",
		"startStory:header:word/header1.xml",
		"startParagraph:",
		"text:Header",
		"endParagraph:",
		"endStory:header:word/header1.xml",
		"startStory:footer:word/footer1.xml",
		"startParagraph:",
		"text:Footer",
		"endParagraph:",
		"endStory:footer:word/footer1.xml",
	}
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d\n got: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q\nfull: %#v", i, got[i], want[i], got)
		}
	}
}

func TestWalkBytesIncludesFootnotesEndnotesAndComments(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId20" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>
  <Relationship Id="rId21" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes" Target="endnotes.xml"/>
  <Relationship Id="rId22" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/>
</Relationships>`,
		"word/footnotes.xml": `<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:footnote w:id="-1" w:type="separator">
    <w:p><w:r><w:t>Skip me</w:t></w:r></w:p>
  </w:footnote>
  <w:footnote w:id="1">
    <w:p><w:r><w:t>Footnote</w:t></w:r></w:p>
  </w:footnote>
</w:footnotes>`,
		"word/endnotes.xml": `<w:endnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:endnote w:id="2">
    <w:p><w:r><w:t>Endnote</w:t></w:r></w:p>
  </w:endnote>
</w:endnotes>`,
		"word/comments.xml": `<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:comment w:id="3" w:author="A">
    <w:p><w:r><w:t>Comment</w:t></w:r></w:p>
  </w:comment>
</w:comments>`,
	})

	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		events = append(events, event)
		return nil
	}), sax.Options{
		IncludeFootnotes: true,
		IncludeEndnotes:  true,
		IncludeComments:  true,
	})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}

	got := simplify(events)
	want := []string{
		"startStory:document:word/document.xml",
		"startDocument",
		"startParagraph:",
		"text:Body",
		"endParagraph:",
		"endDocument",
		"endStory:document:word/document.xml",
		"startStory:footnote:word/footnotes.xml",
		"startNote:1:",
		"startParagraph:",
		"text:Footnote",
		"endParagraph:",
		"endNote:1:",
		"endStory:footnote:word/footnotes.xml",
		"startStory:endnote:word/endnotes.xml",
		"startNote:2:",
		"startParagraph:",
		"text:Endnote",
		"endParagraph:",
		"endNote:2:",
		"endStory:endnote:word/endnotes.xml",
		"startStory:comment:word/comments.xml",
		"startComment:3:A:",
		"startParagraph:",
		"text:Comment",
		"endParagraph:",
		"endComment:3:A:",
		"endStory:comment:word/comments.xml",
	}
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d\n got: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q\nfull: %#v", i, got[i], want[i], got)
		}
	}
}

func TestWalkBytesEmitsSemanticInlineEvents(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:bookmarkStart w:id="10" w:name="bm1"/>
      <w:hyperlink r:id="rId42" w:anchor="anchor1">
        <w:r><w:t>Link</w:t></w:r>
      </w:hyperlink>
      <w:r><w:fldChar w:fldCharType="begin"/></w:r>
      <w:r><w:instrText xml:space="preserve"> REF bm1 </w:instrText></w:r>
      <w:r><w:fldChar w:fldCharType="separate"/></w:r>
      <w:r><w:t>Result</w:t></w:r>
      <w:r><w:fldChar w:fldCharType="end"/></w:r>
      <w:r><w:footnoteReference w:id="1"/></w:r>
      <w:r><w:endnoteReference w:id="2"/></w:r>
      <w:r><w:commentReference w:id="3"/></w:r>
      <w:bookmarkEnd w:id="10"/>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId42" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
</Relationships>`,
	})

	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		events = append(events, event)
		return nil
	}), sax.Options{IncludeFieldInstructions: true})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}

	got := simplify(events)
	want := []string{
		"startStory:document:word/document.xml",
		"startDocument",
		"startParagraph:",
		"bookmarkStart:10:bm1",
		"startHyperlink:rId42:anchor1:https://example.com",
		"text:Link",
		"endHyperlink:rId42:anchor1:https://example.com",
		"fieldChar:begin",
		"fieldInstruction: REF bm1 ",
		"fieldChar:separate",
		"text:Result",
		"fieldChar:end",
		"footnoteReference:1",
		"endnoteReference:2",
		"commentReference:3",
		"bookmarkEnd:10:",
		"endParagraph:",
		"endDocument",
		"endStory:document:word/document.xml",
	}
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d\n got: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q\nfull: %#v", i, got[i], want[i], got)
		}
	}
}

func TestWalkBytesEmitsRevisionRangeAndRunTokenEvents(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:commentRangeStart w:id="5"/>
      <w:ins w:id="7" w:author="Alice" w:date="2026-06-03T10:00:00Z">
        <w:r><w:t>Ins</w:t></w:r>
      </w:ins>
      <w:del w:id="8" w:author="Bob" w:date="2026-06-03T10:01:00Z">
        <w:r><w:delText>Del</w:delText></w:r>
      </w:del>
      <w:moveFrom w:id="9" w:author="Carol" w:date="2026-06-03T10:02:00Z">
        <w:r><w:t>From</w:t></w:r>
      </w:moveFrom>
      <w:moveTo w:id="10" w:author="Dave" w:date="2026-06-03T10:03:00Z">
        <w:r><w:t>To</w:t></w:r>
      </w:moveTo>
      <w:r><w:delInstrText xml:space="preserve"> OLD </w:delInstrText></w:r>
      <w:r><w:sym w:font="Wingdings" w:char="F041"/></w:r>
      <w:r><w:softHyphen/></w:r>
      <w:r><w:noBreakHyphen/></w:r>
      <w:r><w:ptab w:alignment="left" w:relativeTo="margin" w:leader="none"/></w:r>
      <w:r><w:col/></w:r>
      <w:r><w:lastRenderedPageBreak/></w:r>
      <w:r><w:commentRangeEnd w:id="5"/></w:r>
    </w:p>
  </w:body>
</w:document>`,
	})

	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		events = append(events, event)
		return nil
	}), sax.Options{IncludeFieldInstructions: true})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}

	got := simplify(events)
	want := []string{
		"startStory:document:word/document.xml",
		"startDocument",
		"startParagraph:",
		"commentRangeStart:5",
		"startRevision:ins:7:Alice:2026-06-03T10:00:00Z",
		"text:Ins",
		"endRevision:ins:7:Alice:2026-06-03T10:00:00Z",
		"startRevision:del:8:Bob:2026-06-03T10:01:00Z",
		"text:Del",
		"endRevision:del:8:Bob:2026-06-03T10:01:00Z",
		"startRevision:moveFrom:9:Carol:2026-06-03T10:02:00Z",
		"text:From",
		"endRevision:moveFrom:9:Carol:2026-06-03T10:02:00Z",
		"startRevision:moveTo:10:Dave:2026-06-03T10:03:00Z",
		"text:To",
		"endRevision:moveTo:10:Dave:2026-06-03T10:03:00Z",
		"fieldInstruction: OLD ",
		"symbol:Wingdings:F041:A",
		"softHyphen",
		"noBreakHyphen",
		"tab",
		"break:column",
		"break:renderedPage",
		"commentRangeEnd:5",
		"endParagraph:",
		"endDocument",
		"endStory:document:word/document.xml",
	}
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d\n got: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q\nfull: %#v", i, got[i], want[i], got)
		}
	}
}

func TestWalkBytesEmitsWrapperAndOpaqueTokenEvents(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:o="urn:schemas-microsoft-com:office:office">
  <w:body>
    <w:sdt>
      <w:sdtPr>
        <w:id w:val="11"/>
        <w:alias w:val="Block SDT"/>
        <w:tag w:val="block-tag"/>
        <w:text/>
      </w:sdtPr>
      <w:sdtContent>
        <w:p><w:r><w:t>Sdt</w:t></w:r></w:p>
      </w:sdtContent>
    </w:sdt>
    <w:customXml w:uri="urn:block" w:element="blockElem">
      <w:p><w:r><w:t>BlockCustom</w:t></w:r></w:p>
    </w:customXml>
    <w:altChunk r:id="rAlt1"/>
    <w:p>
      <w:smartTag w:uri="urn:smart" w:element="person">
        <w:r><w:t>Smart</w:t></w:r>
      </w:smartTag>
      <w:customXml w:uri="urn:inline" w:element="inlineElem">
        <w:r><w:t>Custom</w:t></w:r>
      </w:customXml>
      <w:sdt>
        <w:sdtPr>
          <w:id w:val="12"/>
          <w:tag w:val="inline-tag"/>
          <w:richText/>
        </w:sdtPr>
        <w:sdtContent>
          <w:r><w:t>InlineSdt</w:t></w:r>
        </w:sdtContent>
      </w:sdt>
      <w:dir w:val="rtl"><w:r><w:t>Dir</w:t></w:r></w:dir>
      <w:bdo w:val="ltr"><w:r><w:t>Bdo</w:t></w:r></w:bdo>
      <w:r><w:ruby><w:rubyBase><w:r><w:t>base</w:t></w:r></w:rubyBase><w:rt><w:r><w:t>anno</w:t></w:r></w:rt></w:ruby></w:r>
      <w:r><m:oMath><m:r><m:t>x</m:t></m:r></m:oMath></w:r>
      <w:r><w:drawing><wp:inline/></w:drawing></w:r>
      <w:r><w:object><o:OLEObject ProgID="Excel.Sheet.12"/></w:object></w:r>
      <w:r><w:contentPart r:id="rCp1"/></w:r>
      <w:r><w:annotationRef/></w:r>
      <w:r><w:dayShort/></w:r>
    </w:p>
  </w:body>
</w:document>`,
	})

	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		events = append(events, event)
		return nil
	}), sax.Options{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}

	got := simplify(events)
	want := []string{
		"startStory:document:word/document.xml",
		"startDocument",
		"startContentControl:11:Block SDT:block-tag:plainText",
		"startParagraph:",
		"text:Sdt",
		"endParagraph:",
		"endContentControl:11:Block SDT:block-tag:plainText",
		"startCustomXML:urn:block:blockElem",
		"startParagraph:",
		"text:BlockCustom",
		"endParagraph:",
		"endCustomXML:urn:block:blockElem",
		"altChunk:rAlt1",
		"startParagraph:",
		"startSmartTag:urn:smart:person",
		"text:Smart",
		"endSmartTag:urn:smart:person",
		"startCustomXML:urn:inline:inlineElem",
		"text:Custom",
		"endCustomXML:urn:inline:inlineElem",
		"startContentControl:12::inline-tag:richText",
		"text:InlineSdt",
		"endContentControl:12::inline-tag:richText",
		"startDirection:rtl",
		"text:Dir",
		"endDirection:rtl",
		"startDirection:ltr",
		"text:Bdo",
		"endDirection:ltr",
		"ruby:base:anno",
		"math:false",
		"drawing",
		"embeddedObject:Excel.Sheet.12",
		"contentPart:rCp1",
		"annotationReference",
		"autoText:dayShort",
		"endParagraph:",
		"endDocument",
		"endStory:document:word/document.xml",
	}
	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d\n got: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q\nfull: %#v", i, got[i], want[i], got)
		}
	}
}

func TestExtractTextBytesIncludesRequestedStories(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
    <w:sectPr>
      <w:headerReference w:type="default" r:id="rId10"/>
      <w:footerReference w:type="default" r:id="rId11"/>
    </w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
  <Relationship Id="rId11" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
  <Relationship Id="rId20" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>
  <Relationship Id="rId21" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes" Target="endnotes.xml"/>
  <Relationship Id="rId22" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/>
</Relationships>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Header</w:t></w:r></w:p>
</w:hdr>`,
		"word/footer1.xml": `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Footer</w:t></w:r></w:p>
</w:ftr>`,
		"word/footnotes.xml": `<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:footnote w:id="1">
    <w:p><w:r><w:t>Footnote</w:t></w:r></w:p>
  </w:footnote>
</w:footnotes>`,
		"word/endnotes.xml": `<w:endnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:endnote w:id="2">
    <w:p><w:r><w:t>Endnote</w:t></w:r></w:p>
  </w:endnote>
</w:endnotes>`,
		"word/comments.xml": `<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:comment w:id="3">
    <w:p><w:r><w:t>Comment</w:t></w:r></w:p>
  </w:comment>
</w:comments>`,
	})

	text, err := sax.ExtractTextBytes(data, sax.Options{
		IncludeHeadersAndFooters: true,
		IncludeFootnotes:         true,
		IncludeEndnotes:          true,
		IncludeComments:          true,
	})
	if err != nil {
		t.Fatalf("ExtractTextBytes: %v", err)
	}

	const want = "Body\nA\tB\nHeader\nFooter\nFootnote\nEndnote\nComment"
	if text != want {
		t.Fatalf("text = %q, want %q", text, want)
	}
}

func TestWalkBytesNormalizesOpaquePayloadMetadata(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="html" ContentType="text/html"/>
  <Default Extension="png" ContentType="image/png"/>
  <Default Extension="bin" ContentType="application/octet-stream"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rAlt1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/afChunk" Target="afchunk/chunk1.html"/>
  <Relationship Id="rImg1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
  <Relationship Id="rOle1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/oleObject1.bin"/>
  <Relationship Id="rPreview1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/preview1.png"/>
  <Relationship Id="rCp1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="wordprocessing/contentPart1.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
  <w:body>
    <w:altChunk r:id="rAlt1">
      <w:altChunkPr><w:matchSrc w:val="1"/></w:altChunkPr>
    </w:altChunk>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:inline>
            <wp:extent cx="914400" cy="1828800"/>
            <wp:docPr id="7" name="Picture 7"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
                <pic:pic>
                  <pic:blipFill>
                    <a:blip r:embed="rImg1"/>
                  </pic:blipFill>
                </pic:pic>
              </a:graphicData>
            </a:graphic>
          </wp:inline>
        </w:drawing>
      </w:r>
      <w:r>
        <w:object>
          <v:shape><v:imagedata r:id="rPreview1"/></v:shape>
          <o:OLEObject ProgID="Excel.Sheet.12" r:id="rOle1"/>
        </w:object>
      </w:r>
      <w:r><w:contentPart r:id="rCp1"/></w:r>
      <w:r><m:oMath><m:r><m:t>x</m:t></m:r><m:r><m:t>+y</m:t></m:r></m:oMath></w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/afchunk/chunk1.html":             "<html>chunk</html>",
		"word/media/image1.png":                "image-bytes",
		"word/media/preview1.png":              "preview-bytes",
		"word/embeddings/oleobject1.bin":       "ole-bytes",
		"word/wordprocessing/contentpart1.xml": "<cp/>",
	})

	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		events = append(events, event)
		return nil
	}), sax.Options{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}

	altChunk := findEvent(t, events, sax.EventAltChunk)
	if altChunk.RelationshipID != "rAlt1" || altChunk.RelationshipType != "http://schemas.openxmlformats.org/officeDocument/2006/relationships/afChunk" || altChunk.RelationshipTarget != "word/afchunk/chunk1.html" || !altChunk.MatchSource {
		t.Fatalf("altChunk = %#v", altChunk)
	}
	if altChunk.AltChunkModel == nil || altChunk.PayloadContentType != "text/html" || altChunk.PayloadSize != int64(len("<html>chunk</html>")) || string(altChunk.PayloadData) != "<html>chunk</html>" {
		t.Fatalf("altChunk payload = %#v", altChunk)
	}

	drawing := findEvent(t, events, sax.EventDrawing)
	if drawing.DrawingKind != "inline" || drawing.DrawingDocID != "7" || drawing.DrawingName != "Picture 7" || drawing.WidthEMU != 914400 || drawing.HeightEMU != 1828800 || drawing.GraphicDataURI != "http://schemas.openxmlformats.org/drawingml/2006/picture" || drawing.DrawingGraphicKind != "picture" {
		t.Fatalf("drawing metadata = %#v", drawing)
	}
	if drawing.DrawingModel == nil || len(drawing.RelatedParts) != 1 || drawing.RelatedParts[0].ContentType != "image/png" {
		t.Fatalf("drawing model = %#v", drawing)
	}
	if drawing.RelationshipID != "rImg1" || drawing.RelationshipType != "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" || drawing.RelationshipTarget != "word/media/image1.png" {
		t.Fatalf("drawing relationship = %#v", drawing)
	}
	if len(drawing.RelatedRelationshipIDs) != 1 || drawing.RelatedRelationshipIDs[0] != "rImg1" || len(drawing.RelatedTargets) != 1 || drawing.RelatedTargets[0] != "word/media/image1.png" {
		t.Fatalf("drawing related targets = %#v", drawing)
	}

	object := findEvent(t, events, sax.EventEmbeddedObject)
	if object.ProgID != "Excel.Sheet.12" || object.RelationshipID != "rPreview1" || object.RelationshipTarget != "word/media/preview1.png" {
		t.Fatalf("embedded object primary metadata = %#v", object)
	}
	if object.EmbeddedObjectModel == nil || len(object.RelatedParts) != 2 || object.RelatedParts[0].ContentType != "image/png" || object.RelatedParts[1].ContentType != "application/octet-stream" {
		t.Fatalf("embedded object model = %#v", object)
	}
	if len(object.RelatedRelationshipIDs) != 2 || object.RelatedRelationshipIDs[0] != "rPreview1" || object.RelatedRelationshipIDs[1] != "rOle1" {
		t.Fatalf("embedded object relationship ids = %#v", object.RelatedRelationshipIDs)
	}
	if len(object.RelatedTargets) != 2 || object.RelatedTargets[0] != "word/media/preview1.png" || object.RelatedTargets[1] != "word/embeddings/oleobject1.bin" {
		t.Fatalf("embedded object targets = %#v", object.RelatedTargets)
	}

	contentPart := findEvent(t, events, sax.EventContentPart)
	if contentPart.RelationshipID != "rCp1" || contentPart.RelationshipTarget != "word/wordprocessing/contentpart1.xml" {
		t.Fatalf("contentPart = %#v", contentPart)
	}
	if contentPart.ContentPartModel == nil || contentPart.PayloadContentType != "application/xml" || string(contentPart.PayloadData) != "<cp/>" {
		t.Fatalf("contentPart payload = %#v", contentPart)
	}

	math := findEvent(t, events, sax.EventMath)
	if math.MathText != "x+y" || math.Display || math.MathModel == nil || math.MathModel.Kind != "oMath" {
		t.Fatalf("math = %#v", math)
	}
}

func TestWalkBytesEmitsTypedSemanticModels(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">
  <w:body>
    <w:sdt>
      <w:sdtPr>
        <w:id w:val="41"/>
        <w:alias w:val="Picker"/>
        <w:tag w:val="picker-tag"/>
        <w:dropDownList>
          <w:listItem w:displayText="One" w:value="1"/>
          <w:listItem w:displayText="Two" w:value="2"/>
        </w:dropDownList>
        <w:showingPlcHdr/>
        <w:placeholder>
          <w:docPart w:val="PlaceholderDoc"/>
          <w:docPartGallery w:val="bbPlcHdr"/>
        </w:placeholder>
        <w:dataBinding w:xpath="/root/item" w:storeItemID="{ABC}"/>
      </w:sdtPr>
      <w:sdtContent><w:p><w:r><w:t>Pick</w:t></w:r></w:p></w:sdtContent>
    </w:sdt>
    <w:p>
      <w:customXml w:uri="urn:test" w:element="person">
        <w:customXmlPr>
          <w:attr w:uri="urn:meta" w:name="role" w:val="author"/>
        </w:customXmlPr>
        <w:r><w:t>Custom</w:t></w:r>
      </w:customXml>
      <w:r>
        <w:ruby>
          <w:rubyPr>
            <w:hps w:val="12"/>
            <w:hpsBaseText w:val="18"/>
            <w:hpsRaise w:val="3"/>
            <w:lid w:val="ja-JP"/>
            <w:rubyAlign w:val="center"/>
            <w:dirty w:val="1"/>
          </w:rubyPr>
          <w:rubyBase><w:r><w:t>漢字</w:t></w:r></w:rubyBase>
          <w:rt><w:r><w:t>かんじ</w:t></w:r></w:rt>
        </w:ruby>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
	})

	var events []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(event sax.Event) error {
		events = append(events, event)
		return nil
	}), sax.Options{})
	if err != nil {
		t.Fatalf("WalkBytes: %v", err)
	}

	control := findContentControl(t, events, "picker-tag")
	if control.ControlProperties == nil || control.ControlProperties.Type != "dropDownList" || len(control.ControlProperties.ListItems) != 2 || !control.ControlProperties.ShowingPlcHdr || control.ControlProperties.PlaceholderDocPart != "PlaceholderDoc" || control.ControlProperties.PlaceholderDocPartGallery != "bbPlcHdr" || control.ControlProperties.DataXPath != "/root/item" || control.ControlProperties.DataStoreID != "{ABC}" {
		t.Fatalf("content control props = %#v", control)
	}

	custom := findEvent(t, events, sax.EventStartCustomXML)
	if custom.CustomXMLModel == nil || custom.CustomXMLModel.Properties == nil || len(custom.CustomXMLModel.Properties.Attributes) != 1 {
		t.Fatalf("custom xml model = %#v", custom)
	}
	attr := custom.CustomXMLModel.Properties.Attributes[0]
	if attr.URI != "urn:meta" || attr.Name != "role" || attr.Value != "author" {
		t.Fatalf("custom xml attr = %#v", attr)
	}

	ruby := findEvent(t, events, sax.EventRuby)
	if ruby.RubyModel == nil || ruby.RubyModel.RubyPr == nil || ruby.RubyModel.RubyPr.LanguageID == nil || ruby.RubyModel.RubyPr.LanguageID.Val != "ja-JP" {
		t.Fatalf("ruby model = %#v", ruby)
	}
	if ruby.RubyModel.RubyPr.PhoneticGuideTextFontSize == nil || ruby.RubyModel.RubyPr.PhoneticGuideTextFontSize.Val != 12 || ruby.RubyModel.RubyPr.BaseTextFontSize == nil || ruby.RubyModel.RubyPr.BaseTextFontSize.Val != 18 || ruby.RubyModel.RubyPr.PhoneticGuideRaise == nil || ruby.RubyModel.RubyPr.PhoneticGuideRaise.Val != 3 {
		t.Fatalf("ruby sizing = %#v", ruby)
	}
	if ruby.RubyModel.RubyPr.RubyAlign == nil || ruby.RubyModel.RubyPr.RubyAlign.Val != "center" || ruby.RubyModel.RubyPr.Dirty == nil {
		t.Fatalf("ruby alignment = %#v", ruby)
	}
}

func findEvent(t *testing.T, events []sax.Event, eventType sax.EventType) sax.Event {
	t.Helper()
	for _, event := range events {
		if event.Type == eventType {
			return event
		}
	}
	t.Fatalf("event %q not found in %#v", eventType, events)
	return sax.Event{}
}

func findContentControl(t *testing.T, events []sax.Event, tag string) sax.Event {
	t.Helper()
	for _, event := range events {
		if event.Type == sax.EventStartContentControl && event.ControlTag == tag {
			return event
		}
	}
	t.Fatalf("content control %q not found in %#v", tag, events)
	return sax.Event{}
}

func simplify(events []sax.Event) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		switch event.Type {
		case sax.EventStartStory, sax.EventEndStory:
			out = append(out, string(event.Type)+":"+string(event.Story)+":"+event.PartName)
		case sax.EventStartContentControl, sax.EventEndContentControl:
			out = append(out, string(event.Type)+":"+event.ControlID+":"+event.ControlAlias+":"+event.ControlTag+":"+event.ControlType)
		case sax.EventStartCustomXML, sax.EventEndCustomXML:
			out = append(out, string(event.Type)+":"+event.MarkupURI+":"+event.MarkupElement)
		case sax.EventStartSmartTag, sax.EventEndSmartTag:
			out = append(out, string(event.Type)+":"+event.MarkupURI+":"+event.MarkupElement)
		case sax.EventStartDirection, sax.EventEndDirection:
			out = append(out, string(event.Type)+":"+event.Direction)
		case sax.EventStartHyperlink, sax.EventEndHyperlink:
			out = append(out, string(event.Type)+":"+event.RelationshipID+":"+event.HyperlinkAnchor+":"+event.HyperlinkTarget)
		case sax.EventStartParagraph, sax.EventEndParagraph:
			out = append(out, string(event.Type)+":"+event.StyleID)
		case sax.EventStartRevision, sax.EventEndRevision:
			out = append(out, string(event.Type)+":"+event.RevisionKind+":"+event.RevisionID+":"+event.Author+":"+event.Date)
		case sax.EventBookmarkStart, sax.EventBookmarkEnd:
			out = append(out, string(event.Type)+":"+event.BookmarkID+":"+event.BookmarkName)
		case sax.EventCommentRangeStart, sax.EventCommentRangeEnd:
			out = append(out, string(event.Type)+":"+event.CommentID)
		case sax.EventText:
			out = append(out, "text:"+event.Text)
		case sax.EventBreak:
			out = append(out, "break:"+event.BreakType)
		case sax.EventSymbol:
			out = append(out, "symbol:"+event.SymbolFont+":"+event.SymbolChar+":"+event.Text)
		case sax.EventSoftHyphen, sax.EventNoBreakHyphen:
			out = append(out, string(event.Type))
		case sax.EventAnnotationReference:
			out = append(out, string(event.Type))
		case sax.EventAutoText:
			out = append(out, "autoText:"+event.AutoTextKind)
		case sax.EventFieldInstruction:
			out = append(out, "fieldInstruction:"+event.FieldInstr)
		case sax.EventFieldChar:
			out = append(out, "fieldChar:"+event.FieldCharType)
		case sax.EventFootnoteReference:
			out = append(out, "footnoteReference:"+event.NoteID)
		case sax.EventEndnoteReference:
			out = append(out, "endnoteReference:"+event.NoteID)
		case sax.EventCommentReference:
			out = append(out, "commentReference:"+event.CommentID)
		case sax.EventStartNote, sax.EventEndNote:
			out = append(out, string(event.Type)+":"+event.NoteID+":"+event.NoteType)
		case sax.EventStartComment, sax.EventEndComment:
			out = append(out, string(event.Type)+":"+event.CommentID+":"+event.Author+":"+event.Date)
		case sax.EventRuby:
			out = append(out, "ruby:"+event.RubyBaseText+":"+event.RubyText)
		case sax.EventMath:
			out = append(out, "math:"+strconv.FormatBool(event.Display))
		case sax.EventDrawing:
			out = append(out, "drawing")
		case sax.EventEmbeddedObject:
			out = append(out, "embeddedObject:"+event.ProgID)
		case sax.EventContentPart:
			out = append(out, "contentPart:"+event.RelationshipID)
		case sax.EventAltChunk:
			out = append(out, "altChunk:"+event.RelationshipID)
		default:
			out = append(out, string(event.Type))
		}
	}
	return out
}

func buildPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatalf("Write(%q): %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
	return buffer.Bytes()
}