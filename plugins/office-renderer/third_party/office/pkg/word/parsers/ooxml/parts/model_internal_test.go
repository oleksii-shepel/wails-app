package parts

import (
	"bytes"
	"encoding/xml"
	"math"
	"strings"
	"testing"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
)

// newDecoderForElement creates an xml.Decoder positioned at the first StartElement
// with the given local name in the provided XML string.
func newDecoderForElement(t *testing.T, xmlStr, local string) (*xml.Decoder, xml.StartElement) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(xmlStr))
	for {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("token error: %v", err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == local {
			return dec, se
		}
	}
}

func TestDecodeInsElement(t *testing.T) {
	xmlStr := `<w:ins xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:author="Alice" w:date="2024-01-01T00:00:00Z">
		<w:r><w:t>inserted</w:t></w:r>
	</w:ins>`
	dec, start := newDecoderForElement(t, xmlStr, "ins")
	var p Paragraph
	if err := decodeInsElement(dec, start, &p); err != nil {
		t.Fatalf("decodeInsElement: %v", err)
	}
	if len(p.Inlines) != 1 {
		t.Fatalf("expected 1 inline, got %d", len(p.Inlines))
	}
	inl := p.Inlines[0]
	if inl.Kind != ParagraphInlineIns {
		t.Errorf("kind = %q, want %q", inl.Kind, ParagraphInlineIns)
	}
	if inl.RevisionAuthor != "Alice" {
		t.Errorf("author = %q, want Alice", inl.RevisionAuthor)
	}
	if inl.Run == nil || len(inl.Run.Contents) != 1 || inl.Run.Contents[0].Text == nil || inl.Run.Contents[0].Text.Value != "inserted" {
		t.Errorf("run text not parsed correctly")
	}
	// Ins runs should also appear in p.Runs
	if len(p.Runs) != 1 {
		t.Errorf("expected 1 run in p.Runs, got %d", len(p.Runs))
	}
}

func TestDecodeDelElement(t *testing.T) {
	xmlStr := `<w:del xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:author="Bob" w:date="2024-02-02T00:00:00Z">
		<w:r><w:t>deleted</w:t></w:r>
	</w:del>`
	dec, start := newDecoderForElement(t, xmlStr, "del")
	var p Paragraph
	if err := decodeDelElement(dec, start, &p); err != nil {
		t.Fatalf("decodeDelElement: %v", err)
	}
	if len(p.Inlines) != 1 {
		t.Fatalf("expected 1 inline, got %d", len(p.Inlines))
	}
	inl := p.Inlines[0]
	if inl.Kind != ParagraphInlineDel {
		t.Errorf("kind = %q, want %q", inl.Kind, ParagraphInlineDel)
	}
	if inl.RevisionAuthor != "Bob" {
		t.Errorf("author = %q, want Bob", inl.RevisionAuthor)
	}
	// Deleted runs should NOT appear in p.Runs
	if len(p.Runs) != 0 {
		t.Errorf("expected 0 runs in p.Runs, got %d", len(p.Runs))
	}
}

func TestDecodeMoveFromElement(t *testing.T) {
	xmlStr := `<w:moveFrom xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:author="Carol" w:date="2024-03-03T00:00:00Z" w:id="42">
		<w:r><w:t>moved away</w:t></w:r>
	</w:moveFrom>`
	dec, start := newDecoderForElement(t, xmlStr, "moveFrom")
	var p Paragraph
	if err := decodeMoveFromElement(dec, start, &p); err != nil {
		t.Fatalf("decodeMoveFromElement: %v", err)
	}
	if len(p.Inlines) != 1 {
		t.Fatalf("expected 1 inline, got %d", len(p.Inlines))
	}
	inl := p.Inlines[0]
	if inl.Kind != ParagraphInlineMoveFrom {
		t.Errorf("kind = %q, want %q", inl.Kind, ParagraphInlineMoveFrom)
	}
	if inl.RevisionAuthor != "Carol" {
		t.Errorf("author = %q, want Carol", inl.RevisionAuthor)
	}
	if inl.MoveID != 42 {
		t.Errorf("moveID = %d, want 42", inl.MoveID)
	}
}

func TestDecodeMoveToElement(t *testing.T) {
	xmlStr := `<w:moveTo xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:author="Dave" w:date="2024-04-04T00:00:00Z" w:id="99">
		<w:r><w:t>moved here</w:t></w:r>
	</w:moveTo>`
	dec, start := newDecoderForElement(t, xmlStr, "moveTo")
	var p Paragraph
	if err := decodeMoveToElement(dec, start, &p); err != nil {
		t.Fatalf("decodeMoveToElement: %v", err)
	}
	if len(p.Inlines) != 1 {
		t.Fatalf("expected 1 inline, got %d", len(p.Inlines))
	}
	inl := p.Inlines[0]
	if inl.Kind != ParagraphInlineMoveTo {
		t.Errorf("kind = %q, want %q", inl.Kind, ParagraphInlineMoveTo)
	}
	if inl.RevisionAuthor != "Dave" {
		t.Errorf("author = %q, want Dave", inl.RevisionAuthor)
	}
	if inl.MoveID != 99 {
		t.Errorf("moveID = %d, want 99", inl.MoveID)
	}
}

func TestDecodePictElement_VMLHorzRule(t *testing.T) {
	xmlStr := `<w:pict xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
		<v:rect o:hr="t" fillcolor="red" o:hralign="center"/>
	</w:pict>`
	dec, start := newDecoderForElement(t, xmlStr, "pict")
	var r Run
	if err := decodePictElement(dec, start, &r); err != nil {
		t.Fatalf("decodePictElement: %v", err)
	}
	if len(r.Contents) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(r.Contents))
	}
	rc := r.Contents[0]
	if rc.Kind != RunContentVMLHorzRule {
		t.Errorf("kind = %q, want %q", rc.Kind, RunContentVMLHorzRule)
	}
	if rc.VMLHorzRuleColor != "red" {
		t.Errorf("color = %q, want red", rc.VMLHorzRuleColor)
	}
	if rc.VMLHorzRuleAlign != "center" {
		t.Errorf("align = %q, want center", rc.VMLHorzRuleAlign)
	}
}

func TestDecodePictElement_VMLImage(t *testing.T) {
	xmlStr := `<w:pict xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
		<v:shape style="width:100px;height:100px">
			<v:imagedata r:id="rId1"/>
		</v:shape>
	</w:pict>`
	dec, start := newDecoderForElement(t, xmlStr, "pict")
	var r Run
	if err := decodePictElement(dec, start, &r); err != nil {
		t.Fatalf("decodePictElement: %v", err)
	}
	if len(r.Contents) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(r.Contents))
	}
	rc := r.Contents[0]
	if rc.Kind != RunContentVMLImage {
		t.Errorf("kind = %q, want %q", rc.Kind, RunContentVMLImage)
	}
	if rc.VMLRelID != "rId1" {
		t.Errorf("relID = %q, want rId1", rc.VMLRelID)
	}
}

func TestDecodePictElement_VMLTextBox(t *testing.T) {
	xmlStr := `<w:pict xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml">
		<v:shape style="width:200px;height:50px" fillcolor="blue">
			<v:textbox style="mso-fit-shape-to-text:t">
				<w:txbxContent>
					<w:p><w:r><w:t>box text</w:t></w:r></w:p>
				</w:txbxContent>
			</v:textbox>
		</v:shape>
	</w:pict>`
	dec, start := newDecoderForElement(t, xmlStr, "pict")
	var r Run
	if err := decodePictElement(dec, start, &r); err != nil {
		t.Fatalf("decodePictElement: %v", err)
	}
	if len(r.Contents) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(r.Contents))
	}
	rc := r.Contents[0]
	if rc.Kind != RunContentVMLTextBox {
		t.Errorf("kind = %q, want %q", rc.Kind, RunContentVMLTextBox)
	}
	if rc.VMLFillColor != "blue" {
		t.Errorf("fillColor = %q, want blue", rc.VMLFillColor)
	}
	if rc.VMLTextBox == nil {
		t.Fatalf("textbox content nil")
	}
	tbText := rc.VMLTextBox.Paragraphs[0].Runs[0].Contents[0].Text
	if len(rc.VMLTextBox.Paragraphs) != 1 || tbText == nil || tbText.Value != "box text" {
		t.Errorf("textbox inner text not parsed")
	}
}

func TestDecodePictElement_VMLBasicShape(t *testing.T) {
	xmlStr := `<w:pict xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml">
		<v:oval style="position:absolute;left:12pt;top:24pt;width:72pt;height:36pt" fillcolor="#00FF00" strokecolor="#112233" strokeweight="2pt"/>
	</w:pict>`
	dec, start := newDecoderForElement(t, xmlStr, "pict")
	var r Run
	if err := decodePictElement(dec, start, &r); err != nil {
		t.Fatalf("decodePictElement: %v", err)
	}
	if len(r.Contents) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(r.Contents))
	}
	rc := r.Contents[0]
	if rc.Kind != RunContentVMLShape {
		t.Fatalf("kind = %q, want %q", rc.Kind, RunContentVMLShape)
	}
	if rc.VMLShapeType != "oval" {
		t.Errorf("shapeType = %q, want oval", rc.VMLShapeType)
	}
	if rc.VMLFillColor != "#00FF00" {
		t.Errorf("fillColor = %q, want #00FF00", rc.VMLFillColor)
	}
	if rc.VMLStrokeColor != "#112233" {
		t.Errorf("strokeColor = %q, want #112233", rc.VMLStrokeColor)
	}
	if rc.VMLStrokeWeight != "2pt" {
		t.Errorf("strokeWeight = %q, want 2pt", rc.VMLStrokeWeight)
	}
}

func TestDecodePictElement_VMLLineShape(t *testing.T) {
	xmlStr := `<w:pict xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml">
		<v:line from="0pt,0pt" to="0pt,48pt" strokecolor="#AA0000" strokeweight="1.5pt"/>
	</w:pict>`
	dec, start := newDecoderForElement(t, xmlStr, "pict")
	var r Run
	if err := decodePictElement(dec, start, &r); err != nil {
		t.Fatalf("decodePictElement: %v", err)
	}
	if len(r.Contents) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(r.Contents))
	}
	rc := r.Contents[0]
	if rc.Kind != RunContentVMLShape {
		t.Fatalf("kind = %q, want %q", rc.Kind, RunContentVMLShape)
	}
	if rc.VMLShapeType != "line" {
		t.Errorf("shapeType = %q, want line", rc.VMLShapeType)
	}
	if rc.VMLFrom != "0pt,0pt" || rc.VMLTo != "0pt,48pt" {
		t.Errorf("line endpoints = (%q,%q), want (0pt,0pt)-(0pt,48pt)", rc.VMLFrom, rc.VMLTo)
	}
}

func TestDecodePictElement_VMLGroup(t *testing.T) {
	xmlStr := `<w:pict xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml">
		<v:group style="position:relative;width:100pt;height:60pt">
			<v:rect style="width:40pt;height:20pt" fillcolor="#ABCDEF" strokecolor="#112233"/>
			<v:polyline style="width:50pt;height:30pt" points="0,0,50,0,50,30" strokecolor="#445566"/>
			<v:shape style="width:30pt;height:18pt" fillcolor="#EEEEEE">
				<v:textbox><w:txbxContent><w:p><w:r><w:t>inside</w:t></w:r></w:p></w:txbxContent></v:textbox>
			</v:shape>
		</v:group>
	</w:pict>`
	dec, start := newDecoderForElement(t, xmlStr, "pict")
	var r Run
	if err := decodePictElement(dec, start, &r); err != nil {
		t.Fatalf("decodePictElement: %v", err)
	}
	if len(r.Contents) != 3 {
		t.Fatalf("expected 3 run contents, got %d", len(r.Contents))
	}
	var sawRect, sawPolyline, sawTextbox bool
	for _, rc := range r.Contents {
		switch {
		case rc.Kind == RunContentVMLShape && rc.VMLShapeType == "rect":
			sawRect = true
		case rc.Kind == RunContentVMLShape && rc.VMLShapeType == "polyline":
			sawPolyline = true
		case rc.Kind == RunContentVMLTextBox && rc.VMLTextBox != nil:
			sawTextbox = true
			if got := rc.VMLTextBox.Paragraphs[0].Runs[0].Contents[0].Text.Value; got != "inside" {
				t.Fatalf("textbox text = %q, want inside", got)
			}
		}
	}
	if !sawRect || !sawPolyline || !sawTextbox {
		t.Fatalf("missing expected VML group contents: rect=%v polyline=%v textbox=%v, contents=%#v", sawRect, sawPolyline, sawTextbox, r.Contents)
	}
}

func TestVMLShapeTypeChildrenUnmarshal(t *testing.T) {
	xmlStr := `<v:shapetype xmlns:v="urn:schemas-microsoft-com:vml" id="_x0000_t75" path="m,l,21600r21600,l21600,xe">
		<v:stroke color="#123456" weight="1pt"/>
		<v:fill color="#ABCDEF"/>
		<v:path v="m,l,21600r21600,l21600,xe" textpathok="t"/>
		<v:textpath on="t" string="Hello"/>
		<v:formulas><v:f eqn="sum @0 0 10800"/></v:formulas>
		<v:handles><v:h position="#0,bottomRight"/></v:handles>
	</v:shapetype>`
	var shapeType VMLShapeType
	if err := xml.Unmarshal([]byte(xmlStr), &shapeType); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if shapeType.Stroke == nil || shapeType.Stroke.Color != "#123456" {
		t.Fatalf("stroke = %+v, want color #123456", shapeType.Stroke)
	}
	if shapeType.Fill == nil || shapeType.Fill.Color != "#ABCDEF" {
		t.Fatalf("fill = %+v, want color #ABCDEF", shapeType.Fill)
	}
	if shapeType.Path == nil || shapeType.Path.TextPathOK != "t" {
		t.Fatalf("path = %+v, want textpathok=t", shapeType.Path)
	}
	if shapeType.TextPath == nil || shapeType.TextPath.String != "Hello" {
		t.Fatalf("textpath = %+v, want Hello", shapeType.TextPath)
	}
	if shapeType.Formulas == nil || len(shapeType.Formulas.Entries) != 1 {
		t.Fatalf("formulas = %+v, want 1 entry", shapeType.Formulas)
	}
	if shapeType.Handles == nil || len(shapeType.Handles.Entries) != 1 {
		t.Fatalf("handles = %+v, want 1 entry", shapeType.Handles)
	}
}

func TestRunUnmarshalAlternateContentVMLFallbackGroup(t *testing.T) {
	xmlStr := `<w:r xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
		xmlns:v="urn:schemas-microsoft-com:vml">
		<mc:AlternateContent>
			<mc:Fallback>
				<v:group>
					<v:line from="0pt,0pt" to="48pt,0pt" strokecolor="#AA0000"/>
				</v:group>
			</mc:Fallback>
		</mc:AlternateContent>
	</w:r>`
	var r Run
	if err := xml.Unmarshal([]byte(xmlStr), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(r.Contents) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(r.Contents))
	}
	if rc := r.Contents[0]; rc.Kind != RunContentVMLShape || rc.VMLShapeType != "line" {
		t.Fatalf("content = %#v, want VML line shape", rc)
	}
}

func TestDecodeRubyElement(t *testing.T) {
	xmlStr := `<w:ruby xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:rubyBase>
			<w:r><w:t>base</w:t></w:r>
		</w:rubyBase>
		<w:rt>
			<w:r><w:t>annotation</w:t></w:r>
		</w:rt>
	</w:ruby>`
	dec, start := newDecoderForElement(t, xmlStr, "ruby")
	ruby, err := decodeRubyElement(dec, start)
	if err != nil {
		t.Fatalf("decodeRubyElement: %v", err)
	}
	if ruby == nil {
		t.Fatal("ruby is nil")
	}
	if len(ruby.BaseRuns) != 1 || ruby.BaseRuns[0].Contents[0].Text == nil || ruby.BaseRuns[0].Contents[0].Text.Value != "base" {
		t.Errorf("base runs not parsed correctly: %+v", ruby.BaseRuns)
	}
	if len(ruby.TextRuns) != 1 || ruby.TextRuns[0].Contents[0].Text == nil || ruby.TextRuns[0].Contents[0].Text.Value != "annotation" {
		t.Errorf("text runs not parsed correctly: %+v", ruby.TextRuns)
	}
}

func TestDecodeSdtCellContent(t *testing.T) {
	xmlStr := `<w:sdtContent xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:p><w:r><w:t>para in sdt</w:t></w:r></w:p>
		<w:tbl><w:tr><w:tc><w:p><w:r><w:t>cell</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
	</w:sdtContent>`
	dec, start := newDecoderForElement(t, xmlStr, "sdtContent")
	var diagnostics diag.List
	blocks, err := decodeSdtCellContent(dec, start, &diagnostics)
	if err != nil {
		t.Fatalf("decodeSdtCellContent: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Kind != ContentBlockParagraph {
		t.Errorf("block[0].kind = %q, want paragraph", blocks[0].Kind)
	}
	if blocks[1].Kind != ContentBlockTable {
		t.Errorf("block[1].kind = %q, want table", blocks[1].Kind)
	}
}

func TestDecodeSdtCellBlocks(t *testing.T) {
	xmlStr := `<w:sdt xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:sdtPr>
			<w:alias w:val="TestAlias"/>
		</w:sdtPr>
		<w:sdtContent>
			<w:p><w:r><w:t>nested para</w:t></w:r></w:p>
		</w:sdtContent>
	</w:sdt>`
	dec, start := newDecoderForElement(t, xmlStr, "sdt")
	var diagnostics diag.List
	block, err := decodeSdtCellBlocks(dec, start, &diagnostics)
	if err != nil {
		t.Fatalf("decodeSdtCellBlocks: %v", err)
	}
	if block.Kind != ContentBlockSDT {
		t.Errorf("kind = %q, want sdt", block.Kind)
	}
	if block.SdtProps == nil || block.SdtProps.Alias != "TestAlias" {
		t.Errorf("sdt alias not parsed correctly: %+v", block.SdtProps)
	}
	if len(block.SdtBlocks) != 1 || block.SdtBlocks[0].Kind != ContentBlockParagraph {
		t.Errorf("sdt blocks not parsed correctly: %+v", block.SdtBlocks)
	}
}

func TestBoolValUnmarshalXMLAttr(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"", true},
		{"true", true},
		{"1", true},
		{"on", true},
		{"0", false},
		{"false", false},
		{"off", false},
		{"maybe", false},
	}
	for _, tt := range tests {
		var b BoolVal
		attr := xml.Attr{Name: xml.Name{Local: "val"}, Value: tt.val}
		if err := b.UnmarshalXMLAttr(attr); err != nil {
			t.Fatalf("UnmarshalXMLAttr(%q): %v", tt.val, err)
		}
		if bool(b) != tt.want {
			t.Errorf("UnmarshalXMLAttr(%q) = %v, want %v", tt.val, b, tt.want)
		}
	}
}

func TestExtFromContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want string
	}{
		{"image/png", ".png"},
		{"image/jpeg", ".jpg"},
		{"image/jpg", ".jpg"},
		{"image/gif", ".gif"},
		{"image/bmp", ".bmp"},
		{"application/octet-stream", ""},
	}
	for _, tt := range tests {
		got := extFromContentType(tt.ct)
		if got != tt.want {
			t.Errorf("extFromContentType(%q) = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestDecodeSdtListItems(t *testing.T) {
	xmlStr := `<w:dropDownList xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:listItem w:displayText="First" w:value="1"/>
		<w:listItem w:displayText="Second" w:value="2"/>
		<w:listItem w:value="noDisplay"/>
	</w:dropDownList>`
	dec, start := newDecoderForElement(t, xmlStr, "dropDownList")
	items, err := decodeSdtListItems(dec, start)
	if err != nil {
		t.Fatalf("decodeSdtListItems: %v", err)
	}
	want := []SdtListItem{{DisplayText: "First", Value: "1"}, {DisplayText: "Second", Value: "2"}, {DisplayText: "", Value: "noDisplay"}}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i, v := range want {
		if items[i].DisplayText != v.DisplayText || items[i].Value != v.Value {
			t.Errorf("item[%d] = %+v, want %+v", i, items[i], v)
		}
	}
}

func TestDecodeSdtDateProperties(t *testing.T) {
	xmlStr := `<w:date xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:fullDate="2024-01-01T00:00:00Z">
		<w:dateFormat w:val="M/d/yyyy"/>
		<w:lid w:val="en-US"/>
		<w:calendar w:val="gregorian"/>
	</w:date>`
	dec, start := newDecoderForElement(t, xmlStr, "date")
	props, err := decodeSdtDateProperties(dec, start)
	if err != nil {
		t.Fatalf("decodeSdtDateProperties: %v", err)
	}
	if props.Format != "M/d/yyyy" {
		t.Errorf("Format = %q, want M/d/yyyy", props.Format)
	}
	if props.FullDate != "2024-01-01T00:00:00Z" {
		t.Errorf("FullDate = %q, want 2024-01-01T00:00:00Z", props.FullDate)
	}
	if props.Language != "en-US" {
		t.Errorf("Language = %q, want en-US", props.Language)
	}
	if props.Calendar != "gregorian" {
		t.Errorf("Calendar = %q, want gregorian", props.Calendar)
	}
}

func TestDecodeSdtDocPartProperties(t *testing.T) {
	xmlStr := `<w:docPartObj xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:docPartGallery w:val="Table of Contents"/>
		<w:docPartCategory w:val="General"/>
		<w:docPartUnique/>
	</w:docPartObj>`
	dec, start := newDecoderForElement(t, xmlStr, "docPartObj")
	props, err := decodeSdtDocPartProperties(dec, start)
	if err != nil {
		t.Fatalf("decodeSdtDocPartProperties: %v", err)
	}
	if props.Gallery != "Table of Contents" {
		t.Errorf("Gallery = %q, want Table of Contents", props.Gallery)
	}
	if props.Category != "General" {
		t.Errorf("Category = %q, want General", props.Category)
	}
	if !props.Unique {
		t.Error("Unique = false, want true")
	}
}

func TestWpsBodyPropertiesUnmarshalXML(t *testing.T) {
	xmlStr := `<wps:bodyPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" lIns="100000" tIns="200000" rIns="300000" bIns="400000" vert="vert270" wrap="square"/>`
	var bp WpsBodyProperties
	if err := xml.Unmarshal([]byte(xmlStr), &bp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if bp.LIns != 100000 {
		t.Errorf("LIns = %d, want 100000", bp.LIns)
	}
	if bp.TIns != 200000 {
		t.Errorf("TIns = %d, want 200000", bp.TIns)
	}
	if bp.RIns != 300000 {
		t.Errorf("RIns = %d, want 300000", bp.RIns)
	}
	if bp.BIns != 400000 {
		t.Errorf("BIns = %d, want 400000", bp.BIns)
	}
	if bp.Vert != "vert270" {
		t.Errorf("Vert = %q, want vert270", bp.Vert)
	}
	if bp.Wrap != "square" {
		t.Errorf("Wrap = %q, want square", bp.Wrap)
	}
}

func TestWpsBodyPropertiesUnmarshalXML_Defaults(t *testing.T) {
	xmlStr := `<wps:bodyPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"/>`
	var bp WpsBodyProperties
	if err := xml.Unmarshal([]byte(xmlStr), &bp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if bp.LIns != EmuAbsent {
		t.Errorf("LIns = %d, want EmuAbsent (%d)", bp.LIns, EmuAbsent)
	}
	if bp.TIns != EmuAbsent {
		t.Errorf("TIns = %d, want EmuAbsent (%d)", bp.TIns, EmuAbsent)
	}
}

func TestCaptureRawXMLElement(t *testing.T) {
	xmlStr := `<w:custom xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" attr="val"><w:child>text</w:child></w:custom>`
	dec, start := newDecoderForElement(t, xmlStr, "custom")
	raw, err := captureRawXMLElement(dec, start)
	if err != nil {
		t.Fatalf("captureRawXMLElement: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("raw bytes empty")
	}
	if !strings.Contains(string(raw), "child") {
		t.Errorf("raw XML missing child element: %s", string(raw))
	}
}

func TestNoteParagraphs(t *testing.T) {
	if noteParagraphs(nil) != nil {
		t.Error("noteParagraphs(nil) should be nil")
	}
	n := &Note{Paragraphs: []*Paragraph{{}}}
	if len(noteParagraphs(n)) != 1 {
		t.Error("noteParagraphs should return note paragraphs")
	}
}

func TestCommentParagraphs(t *testing.T) {
	if commentParagraphs(nil) != nil {
		t.Error("commentParagraphs(nil) should be nil")
	}
	c := &Comment{Paragraphs: []*Paragraph{{}}}
	if len(commentParagraphs(c)) != 1 {
		t.Error("commentParagraphs should return comment paragraphs")
	}
}

func TestFldCharUnmarshalXML_WithFormField(t *testing.T) {
	xmlStr := `<w:fldChar xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:fldCharType="begin">
		<w:ffData>
			<w:name w:val="TextField1"/>
			<w:textInput>
				<w:default w:val="Hello"/>
				<w:maxLength w:val="50"/>
			</w:textInput>
		</w:ffData>
	</w:fldChar>`
	var fc FldChar
	if err := xml.Unmarshal([]byte(xmlStr), &fc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if fc.Type != "begin" {
		t.Errorf("Type = %q, want begin", fc.Type)
	}
	if fc.FormField == nil {
		t.Fatal("FormField is nil")
	}
	if fc.FormField.Name != "TextField1" {
		t.Errorf("Name = %q, want TextField1", fc.FormField.Name)
	}
	if fc.FormField.Kind != "text" {
		t.Errorf("Kind = %q, want text", fc.FormField.Kind)
	}
	if fc.FormField.DefaultTxt != "Hello" {
		t.Errorf("DefaultTxt = %q, want Hello", fc.FormField.DefaultTxt)
	}
	if fc.FormField.MaxLength != 50 {
		t.Errorf("MaxLength = %d, want 50", fc.FormField.MaxLength)
	}
}

func TestFldCharUnmarshalXML_CheckBox(t *testing.T) {
	xmlStr := `<w:fldChar xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:fldCharType="begin">
		<w:ffData>
			<w:name w:val="Check1"/>
			<w:checkBox>
				<w:checked w:val="1"/>
				<w:default w:val="1"/>
			</w:checkBox>
		</w:ffData>
	</w:fldChar>`
	var fc FldChar
	if err := xml.Unmarshal([]byte(xmlStr), &fc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if fc.FormField == nil {
		t.Fatal("FormField is nil")
	}
	if fc.FormField.Kind != "checkbox" {
		t.Errorf("Kind = %q, want checkbox", fc.FormField.Kind)
	}
	if !fc.FormField.Checked {
		t.Error("Checked should be true")
	}
}

func TestFldCharUnmarshalXML_DropDown(t *testing.T) {
	xmlStr := `<w:fldChar xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:fldCharType="begin">
		<w:ffData>
			<w:name w:val="Drop1"/>
			<w:ddList>
				<w:listEntry w:val="A"/>
				<w:listEntry w:val="B"/>
				<w:defaultVal w:val="1"/>
			</w:ddList>
		</w:ffData>
	</w:fldChar>`
	var fc FldChar
	if err := xml.Unmarshal([]byte(xmlStr), &fc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if fc.FormField == nil {
		t.Fatal("FormField is nil")
	}
	if fc.FormField.Kind != "dropdown" {
		t.Errorf("Kind = %q, want dropdown", fc.FormField.Kind)
	}
	want := []string{"A", "B"}
	if len(fc.FormField.DropItems) != len(want) {
		t.Fatalf("DropItems = %v, want %v", fc.FormField.DropItems, want)
	}
	for i, v := range want {
		if fc.FormField.DropItems[i] != v {
			t.Errorf("DropItems[%d] = %q, want %q", i, fc.FormField.DropItems[i], v)
		}
	}
	if fc.FormField.DropDefIdx != 1 {
		t.Errorf("DropDefIdx = %d, want 1", fc.FormField.DropDefIdx)
	}
}

func TestFldCharUnmarshalXML_NoFFData(t *testing.T) {
	xmlStr := `<w:fldChar xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:fldCharType="end"/>`
	var fc FldChar
	if err := xml.Unmarshal([]byte(xmlStr), &fc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if fc.Type != "end" {
		t.Errorf("Type = %q, want end", fc.Type)
	}
	if fc.FormField != nil {
		t.Error("FormField should be nil when no ffData present")
	}
}

func TestFormFieldData_CheckBoxVariants(t *testing.T) {
	// checked with val="0" → false
	xmlStr := `<w:ffData xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:checkBox>
			<w:checked w:val="0"/>
		</w:checkBox>
	</w:ffData>`
	dec, start := newDecoderForElement(t, xmlStr, "ffData")
	var ffd FormFieldData
	if err := ffd.unmarshal(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ffd.Checked {
		t.Error("Checked should be false when val=0")
	}

	// checked element present with no val attribute → true
	xmlStr2 := `<w:ffData xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:checkBox>
			<w:checked/>
		</w:checkBox>
	</w:ffData>`
	dec2, start2 := newDecoderForElement(t, xmlStr2, "ffData")
	var ffd2 FormFieldData
	if err := ffd2.unmarshal(dec2, start2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !ffd2.Checked {
		t.Error("Checked should be true when checked element has no val")
	}
}

func TestCollectContentBlockParagraphs(t *testing.T) {
	// Single paragraph block
	p := collectContentBlockParagraphs(ContentBlock{Kind: ContentBlockParagraph, Paragraph: &Paragraph{}})
	if len(p) != 1 {
		t.Errorf("single para: got %d, want 1", len(p))
	}
	// SDT wrapping nested paragraphs
	sdt := ContentBlock{Kind: ContentBlockSDT, SdtBlocks: []ContentBlock{
		{Kind: ContentBlockParagraph, Paragraph: &Paragraph{}},
		{Kind: ContentBlockParagraph, Paragraph: &Paragraph{}},
	}}
	p = collectContentBlockParagraphs(sdt)
	if len(p) != 2 {
		t.Errorf("nested para: got %d, want 2", len(p))
	}
}

func TestCollectContentBlockTables(t *testing.T) {
	// Single table block
	tbl := collectContentBlockTables(ContentBlock{Kind: ContentBlockTable, Table: &Table{}})
	if len(tbl) != 1 {
		t.Errorf("single table: got %d, want 1", len(tbl))
	}
	// SDT wrapping nested table
	sdt := ContentBlock{Kind: ContentBlockSDT, SdtBlocks: []ContentBlock{
		{Kind: ContentBlockTable, Table: &Table{}},
	}}
	tbl = collectContentBlockTables(sdt)
	if len(tbl) != 1 {
		t.Errorf("nested table: got %d, want 1", len(tbl))
	}
}

func TestCollectParagraphsDiagnostics(t *testing.T) {
	p := &Paragraph{
		Diagnostics: diag.List{{Code: "p/code"}},
		Runs: []*Run{
			{Diagnostics: diag.List{{Code: "r/code"}}},
		},
		Hyperlinks: []*Hyperlink{
			{Diagnostics: diag.List{{Code: "h/code"}}},
		},
	}
	d := collectParagraphsDiagnostics(p)
	codes := make(map[string]bool)
	for _, diag := range d {
		codes[diag.Code] = true
	}
	for _, want := range []string{"p/code", "r/code", "h/code"} {
		if !codes[want] {
			t.Errorf("missing diagnostic code %q", want)
		}
	}
}

func TestRunDefaultsUnmarshalXML(t *testing.T) {
	xmlStr := `<w:rPrDefault xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:rPr><w:b/><w:sz w:val="24"/></w:rPr>
	</w:rPrDefault>`
	var rd RunDefaults
	if err := xml.Unmarshal([]byte(xmlStr), &rd); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if rd.Properties == nil {
		t.Fatal("Properties nil")
	}
	if rd.Properties.Bold == nil {
		t.Error("Expected bold")
	}
	if rd.Properties.FontSize == nil || rd.Properties.FontSize.Val != 24 {
		t.Errorf("FontSize = %v, want 24", rd.Properties.FontSize)
	}
}

func TestParagraphDefaultsUnmarshalXML(t *testing.T) {
	xmlStr := `<w:pPrDefault xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:pPr><w:jc w:val="center"/></w:pPr>
	</w:pPrDefault>`
	var pd ParagraphDefaults
	if err := xml.Unmarshal([]byte(xmlStr), &pd); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if pd.Properties == nil {
		t.Fatal("Properties nil")
	}
	if pd.Properties.Justification == nil || pd.Properties.Justification.Val != "center" {
		t.Errorf("Justification = %v, want center", pd.Properties.Justification)
	}
}

func TestHyperlinkMarshalXML(t *testing.T) {
	h := Hyperlink{
		ID:     "rId1",
		Anchor: "bookmark1",
		Runs:   []*Run{{Contents: []RunContent{{Text: &Text{Value: "link text"}}}}},
	}
	out, err := xml.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// The relationships namespace may use a generated prefix; just look for the id value.
	if !strings.Contains(string(out), `id="rId1"`) {
		t.Errorf("missing id: %s", string(out))
	}
	if !strings.Contains(string(out), `anchor="bookmark1"`) {
		t.Errorf("missing anchor: %s", string(out))
	}
	// Verify a run child is emitted (exact text serialization is Run's responsibility).
	if !strings.Contains(string(out), "<r ") {
		t.Errorf("missing run element: %s", string(out))
	}
}

func TestParagraphPropertiesUnmarshalXML(t *testing.T) {
	xmlStr := `<w:pPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:pStyle w:val="Heading1"/>
		<w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr>
		<w:jc w:val="center"/>
		<w:spacing w:before="240" w:after="120"/>
		<w:ind w:left="720" w:right="360"/>
		<w:shd w:val="clear" w:fill="FF0000"/>
		<w:contextualSpacing/>
		<w:snapToGrid w:val="0"/>
		<w:keepNext/>
		<w:keepLines/>
		<w:widowControl/>
		<w:pageBreakBefore/>
		<w:pBdr><w:top w:val="single" w:sz="4"/></w:pBdr>
		<w:tabs><w:tab w:val="left" w:pos="720"/></w:tabs>
		<w:bidi/>
		<w:mirrorIndents/>
		<w:noAllowOverlap/>
		<suppressLineNumbers/>
		<w:textAlignment w:val="auto"/>
		<w:kinsoku/>
		<w:wordWrap/>
		<w:overflowPunct/>
		<w:topLinePunct/>
		<w:autoSpaceDE/>
		<w:autoSpaceDN/>
		<w:outlineLvl w:val="1"/>
		<w:rPr><w:b/></w:rPr>
		<w:unsupportedPr/>
	</w:pPr>`
	var p ParagraphProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Style == nil || p.Style.Val != "Heading1" {
		t.Errorf("Style = %v", p.Style)
	}
	if p.Numbering == nil {
		t.Error("Numbering nil")
	}
	if p.Justification == nil || p.Justification.Val != "center" {
		t.Errorf("Justification = %v", p.Justification)
	}
	if p.Spacing == nil {
		t.Error("Spacing nil")
	}
	if p.Indentation == nil {
		t.Error("Indentation nil")
	}
	if p.Shading == nil {
		t.Error("Shading nil")
	}
	if p.ContextualSpacing == nil {
		t.Error("ContextualSpacing nil")
	}
	if p.SnapToGrid == nil {
		t.Error("SnapToGrid nil")
	}
	if p.KeepNext == nil {
		t.Error("KeepNext nil")
	}
	if p.KeepLines == nil {
		t.Error("KeepLines nil")
	}
	if p.WidowControl == nil {
		t.Error("WidowControl nil")
	}
	if p.PageBreakBefore == nil {
		t.Error("PageBreakBefore nil")
	}
	if p.Borders == nil {
		t.Error("Borders nil")
	}
	if len(p.Tabs) != 1 {
		t.Errorf("Tabs = %d", len(p.Tabs))
	}
	if p.Bidi == nil {
		t.Error("Bidi nil")
	}
	if p.MirrorIndents == nil {
		t.Error("MirrorIndents nil")
	}
	if p.NoAllowOverlap == nil {
		t.Error("NoAllowOverlap nil")
	}
	if p.TextAlignment == nil || p.TextAlignment.Val != "auto" {
		t.Errorf("TextAlignment = %v", p.TextAlignment)
	}
	if p.Kinsoku == nil {
		t.Error("Kinsoku nil")
	}
	if p.WordWrap == nil {
		t.Error("WordWrap nil")
	}
	if p.OverflowPunct == nil {
		t.Error("OverflowPunct nil")
	}
	if p.TopLinePunct == nil {
		t.Error("TopLinePunct nil")
	}
	if p.AutoSpaceDE == nil {
		t.Error("AutoSpaceDE nil")
	}
	if p.AutoSpaceDN == nil {
		t.Error("AutoSpaceDN nil")
	}
	if p.OutlineLvl == nil || p.OutlineLvl.Val != 1 {
		t.Errorf("OutlineLvl = %v", p.OutlineLvl)
	}
	if p.RunProps == nil || p.RunProps.Bold == nil {
		t.Error("RunProps nil or not bold")
	}
}

func TestParagraphUnmarshalXML_Branches(t *testing.T) {
	xmlStr := `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:pPr><w:jc w:val="right"/></w:pPr>
		<w:r><w:t>run</w:t></w:r>
		<w:hyperlink r:id="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:r><w:t>link</w:t></w:r></w:hyperlink>
		<w:bookmarkStart w:name="bm1"/>
		<w:bookmarkEnd/>
		<w:commentRangeStart/>
		<w:commentRangeEnd/>
		<w:proofErr/>
		<w:fldSimple w:instr="DATE"><w:r><w:t>field</w:t></w:r></w:fldSimple>
		<w:ruby><w:rubyBase><w:r><w:t>base</w:t></w:r></w:rubyBase><w:rt><w:r><w:t>rt</w:t></w:r></w:rt></w:ruby>
		<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><m:r><m:t>x</m:t></m:r></m:oMath>
		<w:unsupportedParagraphThing/>
	</w:p>`
	var p Paragraph
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Properties == nil || p.Properties.Justification == nil || p.Properties.Justification.Val != "right" {
		t.Error("pPr not parsed")
	}
	if len(p.Runs) != 1 {
		t.Errorf("Runs = %d", len(p.Runs))
	}
	if len(p.Hyperlinks) != 1 {
		t.Errorf("Hyperlinks = %d", len(p.Hyperlinks))
	}
	if len(p.Bookmarks) != 1 || p.Bookmarks[0] != "bm1" {
		t.Errorf("Bookmarks = %v", p.Bookmarks)
	}
	// Find field inline
	var foundField, foundRuby, foundMath bool
	for _, inl := range p.Inlines {
		switch inl.Kind {
		case ParagraphInlineField:
			foundField = true
		case ParagraphInlineRuby:
			foundRuby = true
		case ParagraphInlineMath:
			foundMath = true
		}
	}
	if !foundField {
		t.Error("missing field inline")
	}
	if !foundRuby {
		t.Error("missing ruby inline")
	}
	if !foundMath {
		t.Error("missing math inline")
	}
	// customXml should produce a diagnostic
	if len(p.Diagnostics) == 0 {
		t.Error("expected diagnostic for unsupported paragraph child")
	}
}

func TestTablePropertiesUnmarshalXML(t *testing.T) {
	xmlStr := `<w:tblPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:tblW w:w="5000" w:type="pct"/>
		<w:jc w:val="center"/>
		<w:tblBorders><w:top w:val="single" w:sz="4"/></w:tblBorders>
		<w:tblStyle w:val="TableGrid"/>
		<w:shd w:val="clear" w:fill="EEEEEE"/>
		<w:tblLook w:firstRow="1"/>
		<w:tblLayout w:type="fixed"/>
		<w:tblInd w:w="720" w:type="dxa"/>
		<w:tblCellSpacing w:w="120" w:type="dxa"/>
		<w:tblCellMar><w:top w:w="100" w:type="dxa"/></w:tblCellMar>
		<w:tblpPr horzAnchor="page" vertAnchor="page" tblpX="100" tblpY="200"/>
		<w:tblOverlap w:val="never"/>
		<w:tblPrChange w:author="X"/>
		<w:unsupportedTblPr/>
	</w:tblPr>`
	var p TableProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Width == nil || p.Width.Width != 5000 {
		t.Errorf("Width = %v", p.Width)
	}
	if p.Alignment == nil || p.Alignment.Val != "center" {
		t.Errorf("Alignment = %v", p.Alignment)
	}
	if p.Borders == nil {
		t.Error("Borders nil")
	}
	if p.Style == nil || p.Style.Val != "TableGrid" {
		t.Errorf("Style = %v", p.Style)
	}
	if p.Shading == nil {
		t.Error("Shading nil")
	}
	if p.Look == nil {
		t.Error("Look nil")
	}
	if p.Layout == nil || p.Layout.Type != "fixed" {
		t.Errorf("Layout = %v", p.Layout)
	}
	if p.Indent == nil {
		t.Error("Indent nil")
	}
	if p.CellSpacing == nil {
		t.Error("CellSpacing nil")
	}
	if p.CellMargin == nil {
		t.Error("CellMargin nil")
	}
	if p.FloatProps == nil || p.FloatProps.HorzAnchor != "page" {
		t.Errorf("FloatProps = %v", p.FloatProps)
	}
	if p.PropertyChange == nil || p.PropertyChange.Author != "X" {
		t.Errorf("PropertyChange = %v", p.PropertyChange)
	}
	if len(p.Diagnostics) == 0 {
		t.Error("expected diagnostic for unsupported tblPr child")
	}
}

func TestTableCellPropertiesUnmarshalXML(t *testing.T) {
	xmlStr := `<w:tcPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:tcW w:w="2000" w:type="dxa"/>
		<w:gridSpan w:val="2"/>
		<w:vMerge w:val="restart"/>
		<w:tcBorders><w:top w:val="single" w:sz="4"/></w:tcBorders>
		<w:tcMar><w:top w:w="50" w:type="dxa"/></w:tcMar>
		<w:shd w:val="clear" w:fill="CCCCCC"/>
		<w:vAlign w:val="center"/>
		<w:textDirection w:val="btLr"/>
		<w:cnfStyle w:val="000000100000"/>
		<w:noWrap/>
		<w:tcPrChange w:author="Y"/>
		<w:unsupportedTcPr/>
	</w:tcPr>`
	var p TableCellProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Width == nil || p.Width.Width != 2000 {
		t.Errorf("Width = %v", p.Width)
	}
	if p.GridSpan == nil || p.GridSpan.Val != 2 {
		t.Errorf("GridSpan = %v", p.GridSpan)
	}
	if p.VMerge == nil || p.VMerge.Val != "restart" {
		t.Errorf("VMerge = %v", p.VMerge)
	}
	if p.Borders == nil {
		t.Error("Borders nil")
	}
	if p.Margins == nil {
		t.Error("Margins nil")
	}
	if p.Shading == nil {
		t.Error("Shading nil")
	}
	if p.VertAlign == nil || p.VertAlign.Val != "center" {
		t.Errorf("VertAlign = %v", p.VertAlign)
	}
	if p.TextDirection == nil || p.TextDirection.Val != "btLr" {
		t.Errorf("TextDirection = %v", p.TextDirection)
	}
	if p.Conditional == nil {
		t.Error("Conditional nil")
	}
	if p.NoWrap == nil {
		t.Error("NoWrap nil")
	}
	if p.PropertyChange == nil || p.PropertyChange.Author != "Y" {
		t.Errorf("PropertyChange = %v", p.PropertyChange)
	}
	if len(p.Diagnostics) == 0 {
		t.Error("expected diagnostic for unsupported tcPr child")
	}
}

func TestTableRowPropertiesUnmarshalXML(t *testing.T) {
	xmlStr := `<w:trPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:trHeight w:val="400" w:hRule="atLeast"/>
		<w:tblHeader/>
		<w:cantSplit/>
		<w:gridBefore w:val="1"/>
		<w:gridAfter w:val="2"/>
		<w:wBefore w:w="100" w:type="dxa"/>
		<w:wAfter w:w="200" w:type="dxa"/>
		<w:jc w:val="center"/>
		<w:trPrChange w:author="Z"/>
		<w:unsupportedTrPr/>
	</w:trPr>`
	var p TableRowProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Height == nil || p.Height.Val != 400 {
		t.Errorf("Height = %v", p.Height)
	}
	if p.IsHeader == nil {
		t.Error("IsHeader nil")
	}
	if p.CantSplit == nil {
		t.Error("CantSplit nil")
	}
	if p.GridBefore == nil || p.GridBefore.Val != 1 {
		t.Errorf("GridBefore = %v", p.GridBefore)
	}
	if p.GridAfter == nil || p.GridAfter.Val != 2 {
		t.Errorf("GridAfter = %v", p.GridAfter)
	}
	if p.WidthBefore == nil {
		t.Error("WidthBefore nil")
	}
	if p.WidthAfter == nil {
		t.Error("WidthAfter nil")
	}
	if p.PropertyChange == nil || p.PropertyChange.Author != "Z" {
		t.Errorf("PropertyChange = %v", p.PropertyChange)
	}
	if len(p.Diagnostics) == 0 {
		t.Error("expected diagnostic for unsupported trPr child")
	}
}

func TestSettingsExpansion(t *testing.T) {
	xmlStr := `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:documentProtection edit="forms" enforcement="1" hashValue="abc" saltValue="xyz" spinCount="100000"/>
		<w:characterSpacingControl w:val="compressPunctuation"/>
		<w:view w:val="web"/>
		<w:autoHyphenation w:val="1"/>
		<w:consecutiveHyphenLimit w:val="3"/>
		<w:hyphenationZone w:val="360"/>
		<w:doNotHyphenateCaps/>
		<w:evenAndOddHeaders/>
		<w:updateFields w:val="true"/>
		<w:themeFontLang w:val="en-US" w:eastAsia="ja-JP" w:bidi="ar-SA"/>
		<w:proofState w:spelling="clean" w:grammar="dirty"/>
		<w:mailMerge w:mainDocumentType="formLetters" w:linkToQuery="1" w:dataType="text" w:connectString="conn" w:query="SELECT *" w:dataSource="data.doc" w:headerSource="header.doc" w:destination="newDocument"/>
		<w:documentType w:val="letter"/>
		<w:mirrorMargins/>
		<w:doNotDisplayPageBoundaries w:val="1"/>
		<w:bordersDoNotSurroundFooter/>
		<w:bordersDoNotSurroundHeader/>
		<w:doNotTrackFormatting w:val="true"/>
		<w:doNotTrackMoves/>
	</w:settings>`
	var s Settings
	if err := xml.Unmarshal([]byte(xmlStr), &s); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if s.DocumentProtection == nil {
		t.Fatal("DocumentProtection nil")
	}
	if s.DocumentProtection.Edit != "forms" {
		t.Errorf("DocumentProtection.Edit = %q", s.DocumentProtection.Edit)
	}
	if !s.DocumentProtection.Enforcement {
		t.Error("DocumentProtection.Enforcement should be true")
	}
	if s.DocumentProtection.HashValue != "abc" {
		t.Errorf("DocumentProtection.HashValue = %q", s.DocumentProtection.HashValue)
	}
	if s.CharacterSpacingControl == nil || s.CharacterSpacingControl.Val != "compressPunctuation" {
		t.Errorf("CharacterSpacingControl = %v", s.CharacterSpacingControl)
	}
	if s.View == nil || s.View.Val != "web" {
		t.Errorf("View = %v", s.View)
	}
	if s.AutoHyphenation == nil || !onOffBool(s.AutoHyphenation) {
		t.Error("AutoHyphenation should be true")
	}
	if s.ConsecutiveHyphenLimit == nil || s.ConsecutiveHyphenLimit.Val != 3 {
		t.Errorf("ConsecutiveHyphenLimit = %v", s.ConsecutiveHyphenLimit)
	}
	if s.HyphenationZone == nil || s.HyphenationZone.Val != 360 {
		t.Errorf("HyphenationZone = %v", s.HyphenationZone)
	}
	if s.DoNotHyphenateCaps == nil || !onOffBool(s.DoNotHyphenateCaps) {
		t.Error("DoNotHyphenateCaps should be true")
	}
	if s.EvenAndOddHeaders == nil || !onOffBool(s.EvenAndOddHeaders) {
		t.Error("EvenAndOddHeaders should be true")
	}
	if s.UpdateFields == nil || !onOffBool(s.UpdateFields) {
		t.Error("UpdateFields should be true")
	}
	if s.ThemeFontLang == nil || s.ThemeFontLang.Val != "en-US" {
		t.Errorf("ThemeFontLang = %v", s.ThemeFontLang)
	}
	if s.ProofState == nil || s.ProofState.Spelling != "clean" || s.ProofState.Grammar != "dirty" {
		t.Errorf("ProofState = %v", s.ProofState)
	}
	if s.MailMerge == nil || s.MailMerge.MainDocumentType != "formLetters" || !s.MailMerge.LinkToQuery {
		t.Errorf("MailMerge = %v", s.MailMerge)
	}
	if s.DocumentType == nil || s.DocumentType.Val != "letter" {
		t.Errorf("DocumentType = %v", s.DocumentType)
	}
	if s.MirrorMargins == nil || !onOffBool(s.MirrorMargins) {
		t.Error("MirrorMargins should be true")
	}
	if s.DoNotDisplayPageBoundaries == nil || !onOffBool(s.DoNotDisplayPageBoundaries) {
		t.Error("DoNotDisplayPageBoundaries should be true")
	}
	if s.BordersDoNotSurroundFooter == nil || !onOffBool(s.BordersDoNotSurroundFooter) {
		t.Error("BordersDoNotSurroundFooter should be true")
	}
	if s.BordersDoNotSurroundHeader == nil || !onOffBool(s.BordersDoNotSurroundHeader) {
		t.Error("BordersDoNotSurroundHeader should be true")
	}
	if s.DoNotTrackFormatting == nil || !onOffBool(s.DoNotTrackFormatting) {
		t.Error("DoNotTrackFormatting should be true")
	}
	if s.DoNotTrackMoves == nil || !onOffBool(s.DoNotTrackMoves) {
		t.Error("DoNotTrackMoves should be true")
	}
}

func onOffBool(o *OnOff) bool {
	if o == nil {
		return false
	}
	return o.Val == nil || bool(*o.Val)
}

func TestParagraphCommentRanges(t *testing.T) {
	xmlStr := `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:commentRangeStart w:id="1"/>
		<w:r><w:t>text</w:t></w:r>
		<w:commentRangeEnd w:id="1"/>
		<w:commentRangeStart w:id="2"/>
		<w:commentRangeEnd w:id="2"/>
	</w:p>`
	var p Paragraph
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(p.CommentRangeStarts) != 2 || p.CommentRangeStarts[0] != 1 || p.CommentRangeStarts[1] != 2 {
		t.Errorf("CommentRangeStarts = %v", p.CommentRangeStarts)
	}
	if len(p.CommentRangeEnds) != 2 || p.CommentRangeEnds[0] != 1 || p.CommentRangeEnds[1] != 2 {
		t.Errorf("CommentRangeEnds = %v", p.CommentRangeEnds)
	}
}

func TestTablePropertiesExpansion(t *testing.T) {
	xmlStr := `<w:tblPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:tblOverlap w:val="never"/>
		<w:tblCaption w:val="Table 1"/>
		<w:tblDescription w:val="Sales data"/>
		<w:bidiVisual/>
	</w:tblPr>`
	var p TableProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Overlap == nil || p.Overlap.Val != "never" {
		t.Errorf("Overlap = %v", p.Overlap)
	}
	if p.Caption != "Table 1" {
		t.Errorf("Caption = %q", p.Caption)
	}
	if p.Description != "Sales data" {
		t.Errorf("Description = %q", p.Description)
	}
	if p.BidiVisual == nil || !onOffBool(p.BidiVisual) {
		t.Error("BidiVisual should be true")
	}
}

func TestExtendedPropertiesUnmarshal(t *testing.T) {
	xmlStr := `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
		<Application>Microsoft Office Word</Application>
		<AppVersion>16.0000</AppVersion>
		<Company>Contoso</Company>
		<Manager>Alice</Manager>
		<Pages>10</Pages>
		<Words>5000</Words>
		<Characters>30000</Characters>
		<Lines>400</Lines>
		<Paragraphs>100</Paragraphs>
		<TotalTime>120</TotalTime>
		<Template>Normal.dotm</Template>
	</Properties>`
	var props ExtendedProperties
	if err := xml.Unmarshal([]byte(xmlStr), &props); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if props.Application != "Microsoft Office Word" {
		t.Errorf("Application = %q", props.Application)
	}
	if props.AppVersion != "16.0000" {
		t.Errorf("AppVersion = %q", props.AppVersion)
	}
	if props.Company != "Contoso" {
		t.Errorf("Company = %q", props.Company)
	}
	if props.Manager != "Alice" {
		t.Errorf("Manager = %q", props.Manager)
	}
	if props.Pages != 10 {
		t.Errorf("Pages = %d", props.Pages)
	}
	if props.Words != 5000 {
		t.Errorf("Words = %d", props.Words)
	}
	if props.Characters != 30000 {
		t.Errorf("Characters = %d", props.Characters)
	}
	if props.Lines != 400 {
		t.Errorf("Lines = %d", props.Lines)
	}
	if props.Paragraphs != 100 {
		t.Errorf("Paragraphs = %d", props.Paragraphs)
	}
	if props.TotalTime != 120 {
		t.Errorf("TotalTime = %d", props.TotalTime)
	}
	if props.Template != "Normal.dotm" {
		t.Errorf("Template = %q", props.Template)
	}
}

func TestCustomPropertiesUnmarshal(t *testing.T) {
	xmlStr := `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
		<property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="2" name="Project">
			<vt:lpwstr>Alpha</vt:lpwstr>
		</property>
		<property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="3" name="Version">
			<vt:i4>42</vt:i4>
		</property>
	</Properties>`
	var props CustomProperties
	if err := xml.Unmarshal([]byte(xmlStr), &props); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(props.Properties) != 2 {
		t.Fatalf("expected 2 properties, got %d", len(props.Properties))
	}
	if props.Properties[0].Name != "Project" {
		t.Errorf("Property[0].Name = %q", props.Properties[0].Name)
	}
	if props.Properties[1].Name != "Version" {
		t.Errorf("Property[1].Name = %q", props.Properties[1].Name)
	}
}

func TestTableRowPropertiesExpansion(t *testing.T) {
	xmlStr := `<w:trPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:hidden/>
		<w:jc w:val="center"/>
		<w:cnfStyle w:val="000000100000"/>
	</w:trPr>`
	var p TableRowProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Hidden == nil || !onOffBool(p.Hidden) {
		t.Error("Hidden should be true")
	}
	if p.Justification == nil || p.Justification.Val != "center" {
		t.Errorf("Justification = %v", p.Justification)
	}
	if p.Conditional == nil || p.Conditional.Val != "000000100000" {
		t.Errorf("Conditional = %v", p.Conditional)
	}
}

func TestTableCellPropertiesExpansion(t *testing.T) {
	xmlStr := `<w:tcPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:hMerge w:val="restart"/>
		<w:cellIns w:author="Alice" w:date="2024-01-01T00:00:00Z" w:id="1"/>
		<w:cellDel w:author="Bob" w:date="2024-02-02T00:00:00Z" w:id="2"/>
		<w:cellMerge w:author="Carol" w:date="2024-03-03T00:00:00Z" w:id="3"/>
	</w:tcPr>`
	var p TableCellProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.HMerge == nil || p.HMerge.Val != "restart" {
		t.Errorf("HMerge = %v", p.HMerge)
	}
	if p.CellIns == nil || p.CellIns.Author != "Alice" || p.CellIns.ID != 1 {
		t.Errorf("CellIns = %v", p.CellIns)
	}
	if p.CellDel == nil || p.CellDel.Author != "Bob" || p.CellDel.ID != 2 {
		t.Errorf("CellDel = %v", p.CellDel)
	}
	if p.CellMerge == nil || p.CellMerge.Author != "Carol" || p.CellMerge.ID != 3 {
		t.Errorf("CellMerge = %v", p.CellMerge)
	}
}

func TestSectionPropertiesExpansion(t *testing.T) {
	xmlStr := `<w:sectPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:rtlGutter/>
		<w:paperSrc w:first="15" w:other="16"/>
	</w:sectPr>`
	var s SectionProperties
	if err := xml.Unmarshal([]byte(xmlStr), &s); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if s.RtlGutter == nil || !onOffBool(s.RtlGutter) {
		t.Error("RtlGutter should be true")
	}
	if s.PaperSrc == nil || s.PaperSrc.First != 15 || s.PaperSrc.Other != 16 {
		t.Errorf("PaperSrc = %v", s.PaperSrc)
	}
}

func TestDrawingAnchorFullParsing(t *testing.T) {
	xmlStr := `<wp:anchor xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
		relativeHeight="1" behindDoc="1" distT="100" distB="200" distL="300" distR="400"
		simplePos="1" locked="1" layoutInCell="1" allowOverlap="1">
		<wp:simplePos x="500" y="600"/>
		<wp:extent cx="700" cy="800"/>
		<wp:effectExtent l="10" t="20" r="30" b="40"/>
		<wp:docPr id="1" name="pic" descr="desc"/>
		<wp:cNvPr id="2" name="cNvName"/>
		<wp:positionH relativeFrom="page"><wp:align>center</wp:align></wp:positionH>
		<wp:positionV relativeFrom="page"><wp:posOffset>900</wp:posOffset></wp:positionV>
		<wp:sizeRelH relativeFrom="margin"><wp:pct>50000</wp:pct></wp:sizeRelH>
		<wp:sizeRelV relativeFrom="page"><wp:pct>75000</wp:pct></wp:sizeRelV>
		<wp:wrapSquare wrapText="bothSides"/>
	</wp:anchor>`
	var a DrawingAnchor
	if err := xml.Unmarshal([]byte(xmlStr), &a); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if a.RelativeHeight != 1 {
		t.Errorf("RelativeHeight = %d, want 1", a.RelativeHeight)
	}
	if a.BehindDoc != 1 {
		t.Errorf("BehindDoc = %d, want 1", a.BehindDoc)
	}
	if a.DistT != 100 {
		t.Errorf("DistT = %d, want 100", a.DistT)
	}
	if a.DistB != 200 {
		t.Errorf("DistB = %d, want 200", a.DistB)
	}
	if a.DistL != 300 {
		t.Errorf("DistL = %d, want 300", a.DistL)
	}
	if a.DistR != 400 {
		t.Errorf("DistR = %d, want 400", a.DistR)
	}
	if !a.SimplePosAttr {
		t.Error("SimplePosAttr should be true")
	}
	if !a.Locked {
		t.Error("Locked should be true")
	}
	if !a.LayoutInCell {
		t.Error("LayoutInCell should be true")
	}
	if !a.AllowOverlap {
		t.Error("AllowOverlap should be true")
	}
	if a.SimplePos == nil || a.SimplePos.X != 500 || a.SimplePos.Y != 600 {
		t.Errorf("SimplePos = %v", a.SimplePos)
	}
	if a.Extent == nil || a.Extent.CX != 700 || a.Extent.CY != 800 {
		t.Errorf("Extent = %v", a.Extent)
	}
	if a.EffectExtent == nil || a.EffectExtent.L != 10 || a.EffectExtent.T != 20 || a.EffectExtent.R != 30 || a.EffectExtent.B != 40 {
		t.Errorf("EffectExtent = %v", a.EffectExtent)
	}
	if a.DocPr == nil || a.DocPr.ID != 1 || a.DocPr.Name != "pic" || a.DocPr.Desc != "desc" {
		t.Errorf("DocPr = %v", a.DocPr)
	}
	if a.CNvPr == nil || a.CNvPr.ID != 2 || a.CNvPr.Name != "cNvName" {
		t.Errorf("CNvPr = %v", a.CNvPr)
	}
	if a.PositionH == nil || a.PositionH.RelativeFrom != "page" || a.PositionH.Align != "center" {
		t.Errorf("PositionH = %v", a.PositionH)
	}
	if a.PositionV == nil || a.PositionV.RelativeFrom != "page" || a.PositionV.PosOffset != 900 {
		t.Errorf("PositionV = %v", a.PositionV)
	}
	if a.SizeRelH == nil || a.SizeRelH.RelativeFrom != "margin" || a.SizeRelH.Pct != 50000 {
		t.Errorf("SizeRelH = %v", a.SizeRelH)
	}
	if a.SizeRelV == nil || a.SizeRelV.RelativeFrom != "page" || a.SizeRelV.Pct != 75000 {
		t.Errorf("SizeRelV = %v", a.SizeRelV)
	}
	if a.WrapSquare == nil || a.WrapSquare.WrapText != "bothSides" {
		t.Errorf("WrapSquare = %v", a.WrapSquare)
	}
}

func TestDrawingPositionPct(t *testing.T) {
	xmlStr := `<wp:positionH xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" relativeFrom="margin"><wp:pct>25000</wp:pct></wp:positionH>`
	var p DrawingPosition
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.RelativeFrom != "margin" {
		t.Errorf("RelativeFrom = %q, want margin", p.RelativeFrom)
	}
	if p.Pct != 25000 {
		t.Errorf("Pct = %d, want 25000", p.Pct)
	}
	if p.PosOffset != 0 {
		t.Errorf("PosOffset = %d, want 0", p.PosOffset)
	}
	if p.Align != "" {
		t.Errorf("Align = %q, want empty", p.Align)
	}
}

func TestDrawingInlineEffectExtent(t *testing.T) {
	xmlStr := `<wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
		<wp:extent cx="100" cy="200"/>
		<wp:effectExtent l="5" t="10" r="15" b="20"/>
		<wp:docPr id="2" name="inlinePic"/>
	</wp:inline>`
	var i DrawingInline
	if err := xml.Unmarshal([]byte(xmlStr), &i); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if i.Extent == nil || i.Extent.CX != 100 || i.Extent.CY != 200 {
		t.Errorf("Extent = %v", i.Extent)
	}
	if i.EffectExtent == nil || i.EffectExtent.L != 5 || i.EffectExtent.T != 10 || i.EffectExtent.R != 15 || i.EffectExtent.B != 20 {
		t.Errorf("EffectExtent = %v", i.EffectExtent)
	}
	if i.DocPr == nil || i.DocPr.ID != 2 || i.DocPr.Name != "inlinePic" {
		t.Errorf("DocPr = %v", i.DocPr)
	}
}

func TestDrawingAnchorDefaults(t *testing.T) {
	xmlStr := `<wp:anchor xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" behindDoc="0">
		<wp:extent cx="1" cy="1"/>
	</wp:anchor>`
	var a DrawingAnchor
	if err := xml.Unmarshal([]byte(xmlStr), &a); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if a.SimplePosAttr {
		t.Error("SimplePosAttr should be false by default")
	}
	if a.Locked {
		t.Error("Locked should be false by default")
	}
	if a.LayoutInCell {
		t.Error("LayoutInCell should be false by default")
	}
	if a.AllowOverlap {
		t.Error("AllowOverlap should be false by default")
	}
	if a.BehindDoc != 0 {
		t.Errorf("BehindDoc = %d, want 0", a.BehindDoc)
	}
}

func TestWpsShapePropertiesGradientFill(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:gradFill flip="xy" rotWithShape="1">
			<a:gsLst>
				<a:gs pos="0">
					<a:srgbClr val="FF0000"/>
				</a:gs>
				<a:gs pos="50000">
					<a:srgbClr val="00FF00"/>
				</a:gs>
				<a:gs pos="100000">
					<a:srgbClr val="0000FF"/>
				</a:gs>
			</a:gsLst>
			<a:lin ang="5400000" scaled="1"/>
		</a:gradFill>
	</wps:spPr>`
	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	gf, ok := p.Fill.(*drawing.GradientFill)
	if !ok {
		t.Fatalf("expected *drawing.GradientFill, got %T", p.Fill)
	}
	if gf.Flip != "xy" {
		t.Errorf("Flip = %q, want xy", gf.Flip)
	}
	if !gf.RotateWithShape {
		t.Error("RotateWithShape should be true")
	}
	if gf.Angle != 90.0 {
		t.Errorf("Angle = %v, want 90.0", gf.Angle)
	}
	if len(gf.Stops) != 3 {
		t.Fatalf("expected 3 stops, got %d", len(gf.Stops))
	}
	if gf.Stops[0].Position != 0.0 || gf.Stops[0].Color != "FF0000" {
		t.Errorf("stop[0] = %+v", gf.Stops[0])
	}
	if gf.Stops[1].Position != 0.5 || gf.Stops[1].Color != "00FF00" {
		t.Errorf("stop[1] = %+v", gf.Stops[1])
	}
	if gf.Stops[2].Position != 1.0 || gf.Stops[2].Color != "0000FF" {
		t.Errorf("stop[2] = %+v", gf.Stops[2])
	}
}

func TestWpsShapePropertiesGradientFillRadial(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:gradFill>
			<a:gsLst>
				<a:gs pos="0">
					<a:schemeClr val="accent1"/>
				</a:gs>
				<a:gs pos="100000">
					<a:srgbClr val="FFFFFF"/>
				</a:gs>
			</a:gsLst>
			<a:path path="circle"/>
		</a:gradFill>
	</wps:spPr>`
	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	gf, ok := p.Fill.(*drawing.GradientFill)
	if !ok {
		t.Fatalf("expected *drawing.GradientFill, got %T", p.Fill)
	}
	if gf.GradientType != drawing.GradientTypeRadial {
		t.Errorf("GradientType = %q, want radial", gf.GradientType)
	}
	if len(gf.Stops) != 2 {
		t.Fatalf("expected 2 stops, got %d", len(gf.Stops))
	}
	if gf.Stops[0].Color != "accent1" {
		t.Errorf("stop[0].Color = %q, want accent1", gf.Stops[0].Color)
	}
	if gf.Stops[1].Color != "FFFFFF" {
		t.Errorf("stop[1].Color = %q, want FFFFFF", gf.Stops[1].Color)
	}
}

func TestWpsShapePropertiesPatternFill(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:pattFill prst="diagCross">
			<a:fgClr>
				<a:srgbClr val="FF0000"/>
			</a:fgClr>
			<a:bgClr>
				<a:srgbClr val="00FF00"/>
			</a:bgClr>
		</a:pattFill>
	</wps:spPr>`
	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	pf, ok := p.Fill.(*drawing.PatternFill)
	if !ok {
		t.Fatalf("expected *drawing.PatternFill, got %T", p.Fill)
	}
	if pf.PatternType != drawing.PatternTypeDiagonalCross {
		t.Errorf("PatternType = %q, want diagCross", pf.PatternType)
	}
	if pf.Foreground != "FF0000" {
		t.Errorf("Foreground = %q, want FF0000", pf.Foreground)
	}
	if pf.Background != "00FF00" {
		t.Errorf("Background = %q, want 00FF00", pf.Background)
	}
}

func TestDrawingDocPrExpansion(t *testing.T) {
	xmlStr := `<wp:docPr xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
		id="5" name="image.png" descr="description" title="title text" hidden="1"/>`
	var d DrawingDocPr
	if err := xml.Unmarshal([]byte(xmlStr), &d); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if d.ID != 5 {
		t.Errorf("ID = %d, want 5", d.ID)
	}
	if d.Name != "image.png" {
		t.Errorf("Name = %q, want image.png", d.Name)
	}
	if d.Desc != "description" {
		t.Errorf("Desc = %q, want description", d.Desc)
	}
	if d.Title != "title text" {
		t.Errorf("Title = %q, want title text", d.Title)
	}
	if !d.Hidden {
		t.Error("Hidden should be true")
	}
}

func TestDrawingDocPrDefaults(t *testing.T) {
	xmlStr := `<wp:docPr xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" id="1" name="pic"/>`
	var d DrawingDocPr
	if err := xml.Unmarshal([]byte(xmlStr), &d); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if d.Desc != "" {
		t.Errorf("Desc = %q, want empty", d.Desc)
	}
	if d.Title != "" {
		t.Errorf("Title = %q, want empty", d.Title)
	}
	if d.Hidden {
		t.Error("Hidden should be false by default")
	}
}

func TestBackgroundCapturesImageRelationship(t *testing.T) {
	xmlStr := `<w:background xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		xmlns:v="urn:schemas-microsoft-com:vml"
		w:color="F0F0F0" w:themeColor="accent1">
		<v:background>
			<v:fill r:id="rIdBg" type="frame"/>
		</v:background>
	</w:background>`
	var bg Background
	if err := xml.Unmarshal([]byte(xmlStr), &bg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if bg.Color != "F0F0F0" {
		t.Errorf("Color = %q, want F0F0F0", bg.Color)
	}
	if bg.ThemeColor != "accent1" {
		t.Errorf("ThemeColor = %q, want accent1", bg.ThemeColor)
	}
	if bg.ImageRelID != "rIdBg" {
		t.Errorf("ImageRelID = %q, want rIdBg", bg.ImageRelID)
	}
}

func TestFormFieldDataExpansion(t *testing.T) {
	xmlStr := `<w:ffData xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:name w:val="Field1"/>
		<w:enabled w:val="0"/>
		<w:calcOnExit w:val="1"/>
		<w:textInput>
			<w:default w:val="hello"/>
			<w:maxLength w:val="50"/>
			<w:type w:val="number"/>
			<w:format w:val="#,##0"/>
		</w:textInput>
	</w:ffData>`
	var ffd FormFieldData
	dec, start := newDecoderForElement(t, xmlStr, "ffData")
	if err := ffd.unmarshal(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ffd.Name != "Field1" {
		t.Errorf("Name = %q, want Field1", ffd.Name)
	}
	if ffd.Enabled {
		t.Error("Enabled should be false")
	}
	if !ffd.CalcOnExit {
		t.Error("CalcOnExit should be true")
	}
	if ffd.Kind != "text" {
		t.Errorf("Kind = %q, want text", ffd.Kind)
	}
	if ffd.DefaultTxt != "hello" {
		t.Errorf("DefaultTxt = %q, want hello", ffd.DefaultTxt)
	}
	if ffd.MaxLength != 50 {
		t.Errorf("MaxLength = %d, want 50", ffd.MaxLength)
	}
	if ffd.TextType != "number" {
		t.Errorf("TextType = %q, want number", ffd.TextType)
	}
	if ffd.TextFormat != "#,##0" {
		t.Errorf("TextFormat = %q, want #,##0", ffd.TextFormat)
	}
}

func TestFormFieldDataCheckboxExpansion(t *testing.T) {
	xmlStr := `<w:ffData xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:checkBox>
			<w:size w:val="24"/>
			<w:sizeAuto/>
			<w:checked w:val="1"/>
		</w:checkBox>
	</w:ffData>`
	var ffd FormFieldData
	dec, start := newDecoderForElement(t, xmlStr, "ffData")
	if err := ffd.unmarshal(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ffd.Kind != "checkbox" {
		t.Errorf("Kind = %q, want checkbox", ffd.Kind)
	}
	if ffd.CheckboxSize != 24 {
		t.Errorf("CheckboxSize = %d, want 24", ffd.CheckboxSize)
	}
	if !ffd.CheckboxSizeAuto {
		t.Error("CheckboxSizeAuto should be true")
	}
	if !ffd.Checked {
		t.Error("Checked should be true")
	}
}

func TestFormFieldDataEnabledDefault(t *testing.T) {
	xmlStr := `<w:ffData xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:textInput/>
	</w:ffData>`
	var ffd FormFieldData
	dec, start := newDecoderForElement(t, xmlStr, "ffData")
	if err := ffd.unmarshal(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !ffd.Enabled {
		t.Error("Enabled should default to true when absent")
	}
}

func TestWpsBodyPropertiesExpansion(t *testing.T) {
	xmlStr := `<wps:bodyPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		lIns="100000" tIns="200000" rIns="300000" bIns="400000"
		vert="vert" wrap="square"
		anchor="ctr" anchorCtr="1" rot="5400000" upright="1"/>`
	var b WpsBodyProperties
	if err := xml.Unmarshal([]byte(xmlStr), &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b.LIns != 100000 {
		t.Errorf("LIns = %d, want 100000", b.LIns)
	}
	if b.TIns != 200000 {
		t.Errorf("TIns = %d, want 200000", b.TIns)
	}
	if b.RIns != 300000 {
		t.Errorf("RIns = %d, want 300000", b.RIns)
	}
	if b.BIns != 400000 {
		t.Errorf("BIns = %d, want 400000", b.BIns)
	}
	if b.Vert != "vert" {
		t.Errorf("Vert = %q, want vert", b.Vert)
	}
	if b.Wrap != "square" {
		t.Errorf("Wrap = %q, want square", b.Wrap)
	}
	if b.Anchor != "ctr" {
		t.Errorf("Anchor = %q, want ctr", b.Anchor)
	}
	if !b.AnchorCtr {
		t.Error("AnchorCtr should be true")
	}
	if b.Rot != 5400000 {
		t.Errorf("Rot = %d, want 5400000", b.Rot)
	}
	if !b.Upright {
		t.Error("Upright should be true")
	}
}

func TestWpsBodyPropertiesDefaults(t *testing.T) {
	xmlStr := `<wps:bodyPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"/>`
	var b WpsBodyProperties
	if err := xml.Unmarshal([]byte(xmlStr), &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b.Anchor != "" {
		t.Errorf("Anchor = %q, want empty", b.Anchor)
	}
	if b.AnchorCtr {
		t.Error("AnchorCtr should be false by default")
	}
	if b.Rot != 0 {
		t.Errorf("Rot = %d, want 0", b.Rot)
	}
	if b.Upright {
		t.Error("Upright should be false by default")
	}
}

func TestWord2010TextFillGradient(t *testing.T) {
	xmlStr := `<w:textFill xmlns:w="http://schemas.microsoft.com/office/word/2010/wordml" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:gradFill>
			<a:gsLst>
				<a:gs pos="0"><a:srgbClr val="FF0000"/></a:gs>
				<a:gs pos="100000"><a:srgbClr val="0000FF"/></a:gs>
			</a:gsLst>
			<a:lin ang="0"/>
		</a:gradFill>
	</w:textFill>`
	var tf Word2010TextFill
	if err := xml.Unmarshal([]byte(xmlStr), &tf); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if tf.GradFill == nil {
		t.Fatal("expected GradFill to be parsed")
	}
	if len(tf.GradFill.StopList) != 2 {
		t.Errorf("expected 2 stops, got %d", len(tf.GradFill.StopList))
	}
}

func TestWord2010TextFillNoFill(t *testing.T) {
	xmlStr := `<w:textFill xmlns:w="http://schemas.microsoft.com/office/word/2010/wordml">
		<a:noFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>
	</w:textFill>`
	var tf Word2010TextFill
	if err := xml.Unmarshal([]byte(xmlStr), &tf); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if tf.NoFill == nil {
		t.Error("expected NoFill to be parsed")
	}
	if tf.GradFill != nil {
		t.Error("expected GradFill to be nil")
	}
}

func TestPictureShapePropsLine(t *testing.T) {
	xmlStr := `<pic:spPr xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:xfrm rot="5400000" flipH="1" flipV="0"/>
		<a:ln w="12700">
			<a:solidFill>
				<a:srgbClr val="FF0000"/>
			</a:solidFill>
		</a:ln>
	</pic:spPr>`
	var p PictureShapeProps
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Xfrm == nil || p.Xfrm.Rot != 5400000 || !p.Xfrm.FlipH || p.Xfrm.FlipV {
		t.Errorf("Xfrm = %+v", p.Xfrm)
	}
	if p.Ln == nil {
		t.Fatal("expected Ln to be parsed")
	}
	if p.Ln.W != 12700 {
		t.Errorf("Ln.W = %d, want 12700", p.Ln.W)
	}
	if p.Ln.SolidFill == nil || p.Ln.SolidFill.SRGBClr == nil || p.Ln.SolidFill.SRGBClr.Val != "FF0000" {
		t.Errorf("Ln.SolidFill = %+v", p.Ln.SolidFill)
	}
}

func TestRunSoftHyphen(t *testing.T) {
	xmlStr := `<w:r xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:softHyphen/></w:r>`
	var r Run
	if err := xml.Unmarshal([]byte(xmlStr), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(r.Contents) != 1 || r.Contents[0].Kind != RunContentSoftHyphen {
		t.Errorf("expected softHyphen content, got %+v", r.Contents)
	}
}

func TestRunNoBreakHyphen(t *testing.T) {
	xmlStr := `<w:r xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:noBreakHyphen/></w:r>`
	var r Run
	if err := xml.Unmarshal([]byte(xmlStr), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(r.Contents) != 1 || r.Contents[0].Kind != RunContentNoBreakHyphen {
		t.Errorf("expected noBreakHyphen content, got %+v", r.Contents)
	}
}

func TestRunCR(t *testing.T) {
	xmlStr := `<w:r xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:cr/></w:r>`
	var r Run
	if err := xml.Unmarshal([]byte(xmlStr), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(r.Contents) != 1 || r.Contents[0].Kind != RunContentCR {
		t.Errorf("expected cr content, got %+v", r.Contents)
	}
}

func TestRunBookmarkStart(t *testing.T) {
	xmlStr := `<w:r xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:bookmarkStart w:name="myBookmark"/>
		<w:t>text</w:t>
	</w:r>`
	var r Run
	if err := xml.Unmarshal([]byte(xmlStr), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(r.BookmarkStarts) != 1 || r.BookmarkStarts[0] != "myBookmark" {
		t.Errorf("BookmarkStarts = %v, want [myBookmark]", r.BookmarkStarts)
	}
}

func TestRunCommentRanges(t *testing.T) {
	xmlStr := `<w:r xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:commentRangeStart w:id="5"/>
		<w:t>text</w:t>
		<w:commentRangeEnd w:id="5"/>
	</w:r>`
	var r Run
	if err := xml.Unmarshal([]byte(xmlStr), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(r.CommentRangeStarts) != 1 || r.CommentRangeStarts[0] != 5 {
		t.Errorf("CommentRangeStarts = %v, want [5]", r.CommentRangeStarts)
	}
	if len(r.CommentRangeEnds) != 1 || r.CommentRangeEnds[0] != 5 {
		t.Errorf("CommentRangeEnds = %v, want [5]", r.CommentRangeEnds)
	}
}

func TestParagraphRunBookmarksAndComments(t *testing.T) {
	xmlStr := `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:r>
			<w:bookmarkStart w:name="bm1"/>
			<w:commentRangeStart w:id="1"/>
			<w:t>hello</w:t>
		</w:r>
	</w:p>`
	var p Paragraph
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(p.Bookmarks) != 1 || p.Bookmarks[0] != "bm1" {
		t.Errorf("Bookmarks = %v, want [bm1]", p.Bookmarks)
	}
	if len(p.CommentRangeStarts) != 1 || p.CommentRangeStarts[0] != 1 {
		t.Errorf("CommentRangeStarts = %v, want [1]", p.CommentRangeStarts)
	}
}

func TestParagraphBordersBetweenBar(t *testing.T) {
	xmlStr := `<w:pBdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:top w:val="single" w:sz="4" w:space="1"/>
		<w:left w:val="single" w:sz="4" w:space="1"/>
		<w:bottom w:val="single" w:sz="4" w:space="1"/>
		<w:right w:val="single" w:sz="4" w:space="1"/>
		<w:between w:val="dashed" w:sz="6" w:space="2"/>
		<w:bar w:val="double" w:sz="8" w:space="3"/>
	</w:pBdr>`
	var b ParagraphBorders
	if err := xml.Unmarshal([]byte(xmlStr), &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b.Top == nil || b.Top.Val != "single" {
		t.Errorf("Top = %v", b.Top)
	}
	if b.Left == nil || b.Left.Val != "single" {
		t.Errorf("Left = %v", b.Left)
	}
	if b.Bottom == nil || b.Bottom.Val != "single" {
		t.Errorf("Bottom = %v", b.Bottom)
	}
	if b.Right == nil || b.Right.Val != "single" {
		t.Errorf("Right = %v", b.Right)
	}
	if b.Between == nil || b.Between.Val != "dashed" || b.Between.Size != 6 {
		t.Errorf("Between = %v", b.Between)
	}
	if b.Bar == nil || b.Bar.Val != "double" || b.Bar.Size != 8 {
		t.Errorf("Bar = %v", b.Bar)
	}
}

func TestCustomPropertiesTypedValues(t *testing.T) {
	xmlStr := `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
		<property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="2" name="Website">
			<vt:lpwstr>www.sample-files.com</vt:lpwstr>
		</property>
		<property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="3" name="Count">
			<vt:i4>42</vt:i4>
		</property>
		<property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="4" name="Active">
			<vt:bool>true</vt:bool>
		</property>
	</Properties>`
	var props CustomProperties
	if err := xml.Unmarshal([]byte(xmlStr), &props); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(props.Properties) != 3 {
		t.Fatalf("expected 3 properties, got %d", len(props.Properties))
	}
	if props.Properties[0].Name != "Website" || props.Properties[0].Value != "www.sample-files.com" {
		t.Errorf("Website = %q, want www.sample-files.com", props.Properties[0].Value)
	}
	if props.Properties[1].Name != "Count" || props.Properties[1].Value != "42" {
		t.Errorf("Count = %q, want 42", props.Properties[1].Value)
	}
	if props.Properties[2].Name != "Active" || props.Properties[2].Value != "true" {
		t.Errorf("Active = %q, want true", props.Properties[2].Value)
	}
}

func TestBodyMarshalXMLOrdering(t *testing.T) {
	sp := &SectionProperties{Type: &StringVal{Val: "continuous"}}
	body := Body{
		Blocks: []BodyBlock{
			{Kind: BodyBlockParagraph, Paragraph: &Paragraph{Runs: []*Run{{Text: []*Text{{Value: "p1"}}}}}},
			{Kind: BodyBlockTable, Table: &Table{Rows: []*TableRow{{}}}},
			{Kind: BodyBlockParagraph, Paragraph: &Paragraph{Runs: []*Run{{Text: []*Text{{Value: "p2"}}}}}},
			{Kind: BodyBlockSection, Section: sp},
		},
		Section: sp,
	}
	data, err := xml.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	xmlStr := string(data)
	// Count occurrences to ensure sectPr emitted exactly once
	sectCount := strings.Count(xmlStr, "<sectPr")
	if sectCount != 1 {
		t.Fatalf("expected 1 sectPr, got %d in %s", sectCount, xmlStr)
	}
	p1Idx := strings.Index(xmlStr, "<p ")
	tblIdx := strings.Index(xmlStr, "<tbl")
	p2Idx := strings.LastIndex(xmlStr, "<p ")
	sectIdx := strings.Index(xmlStr, "<sectPr")
	if p1Idx == -1 || tblIdx == -1 || sectIdx == -1 {
		t.Fatalf("missing elements in output: %s", xmlStr)
	}
	// Ensure ordering is p1, tbl, p2, sectPr
	if !(p1Idx < tblIdx && tblIdx < p2Idx && p2Idx < sectIdx) {
		t.Errorf("wrong ordering: p1=%d tbl=%d p2=%d sect=%d in %s", p1Idx, tblIdx, p2Idx, sectIdx, xmlStr)
	}
}

func TestBodyMarshalXMLFallback(t *testing.T) {
	body := Body{
		Paragraphs: []*Paragraph{{Runs: []*Run{{Text: []*Text{{Value: "p1"}}}}}},
		Tables:     []*Table{{Rows: []*TableRow{{}}}},
		Section:    &SectionProperties{Type: &StringVal{Val: "nextPage"}},
	}
	data, err := xml.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	xmlStr := string(data)
	if !strings.Contains(xmlStr, "<p ") || !strings.Contains(xmlStr, "<tbl") || !strings.Contains(xmlStr, "<sectPr") {
		t.Errorf("missing elements in fallback output: %s", xmlStr)
	}
}

func TestParagraphBookmarkMarshal(t *testing.T) {
	p := Paragraph{
		BookmarkStartElems: []BookmarkStart{{ID: 1, Name: "bm1"}, {ID: 2, Name: "bm2"}},
		BookmarkEndElems:   []BookmarkEnd{{ID: 1}, {ID: 2}},
	}
	data, err := xml.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	xmlStr := string(data)
	if !strings.Contains(xmlStr, `id="1" name="bm1"`) {
		t.Errorf("missing bookmarkStart bm1: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `bookmarkEnd`) || !strings.Contains(xmlStr, `id="1"`) {
		t.Errorf("missing bookmarkEnd id=1: %s", xmlStr)
	}
}

func TestRunContentMathMarshal(t *testing.T) {
	rc := RunContent{
		Kind:   RunContentMath,
		MathML: `<m:r><m:t>x</m:t></m:r>`,
	}
	run := Run{Contents: []RunContent{rc}}
	data, err := xml.Marshal(run)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	xmlStr := string(data)
	mNS := "http://schemas.openxmlformats.org/officeDocument/2006/math"
	if !strings.Contains(xmlStr, `<oMath xmlns="`+mNS+`"><m:r><m:t>x</m:t></m:r></oMath>`) {
		t.Errorf("expected math element, got: %s", xmlStr)
	}
}

func TestAltChunkParsing(t *testing.T) {
	xmlStr := `<w:body xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
		<w:altChunk r:id="rId5"/>
		<w:altChunk r:id="rId6">
			<w:altChunkPr>
				<w:matchSrc w:val="1"/>
			</w:altChunkPr>
		</w:altChunk>
	</w:body>`
	var b Body
	if err := xml.Unmarshal([]byte(xmlStr), &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(b.AltChunks) != 2 {
		t.Fatalf("expected 2 altChunks, got %d", len(b.AltChunks))
	}
	if b.AltChunks[0].RelID != "rId5" {
		t.Errorf("altChunk[0].RelID = %q, want rId5", b.AltChunks[0].RelID)
	}
	if b.AltChunks[1].RelID != "rId6" {
		t.Errorf("altChunk[1].RelID = %q, want rId6", b.AltChunks[1].RelID)
	}
	if b.AltChunks[1].Properties == nil || b.AltChunks[1].Properties.MatchSrc == nil || !onOffBool(b.AltChunks[1].Properties.MatchSrc) {
		t.Errorf("altChunk[1].Properties.MatchSrc should be true")
	}
	if len(b.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(b.Blocks))
	}
	if b.Blocks[0].Kind != BodyBlockAltChunk || b.Blocks[0].AltChunk == nil {
		t.Errorf("block[0] kind = %q, want altChunk", b.Blocks[0].Kind)
	}
}

func TestParagraphMoveRangeMarkers(t *testing.T) {
	xmlStr := `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:moveFromRangeStart w:id="1" w:author="Alice" w:date="2024-01-01" w:name="move1"/>
		<w:moveFromRangeEnd w:id="1"/>
		<w:moveToRangeStart w:id="2" w:author="Bob" w:date="2024-02-01"/>
		<w:moveToRangeEnd w:id="2"/>
		<w:r><w:t>text</w:t></w:r>
	</w:p>`
	var p Paragraph
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(p.MoveFromRangeStarts) != 1 {
		t.Fatalf("expected 1 moveFromRangeStart, got %d", len(p.MoveFromRangeStarts))
	}
	if p.MoveFromRangeStarts[0].ID != 1 || p.MoveFromRangeStarts[0].Author != "Alice" || p.MoveFromRangeStarts[0].Name != "move1" {
		t.Errorf("moveFromRangeStart = %+v", p.MoveFromRangeStarts[0])
	}
	if len(p.MoveFromRangeEnds) != 1 || p.MoveFromRangeEnds[0].ID != 1 {
		t.Errorf("moveFromRangeEnd = %+v", p.MoveFromRangeEnds)
	}
	if len(p.MoveToRangeStarts) != 1 || p.MoveToRangeStarts[0].ID != 2 || p.MoveToRangeStarts[0].Author != "Bob" {
		t.Errorf("moveToRangeStart = %+v", p.MoveToRangeStarts[0])
	}
	if len(p.MoveToRangeEnds) != 1 || p.MoveToRangeEnds[0].ID != 2 {
		t.Errorf("moveToRangeEnd = %+v", p.MoveToRangeEnds)
	}
}

func TestRefID(t *testing.T) {
	if got := refID(nil); got != "" {
		t.Errorf("refID(nil) = %q, want empty", got)
	}
	if got := refID(&FontEmbedRef{RelID: "rFont1"}); got != "rFont1" {
		t.Errorf("refID(FontEmbedRef{RelID: \"rFont1\"}) = %q, want rFont1", got)
	}
	if got := refID(&FontEmbedRef{}); got != "" {
		t.Errorf("refID(FontEmbedRef{}) = %q, want empty", got)
	}
}

func TestPictureNonVisualPropsParsing(t *testing.T) {
	xmlStr := `<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
		<pic:nvPicPr>
			<pic:cNvPr id="1" name="Picture 1"/>
			<pic:cNvPicPr/>
		</pic:nvPicPr>
		<pic:blipFill>
			<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" r:embed="rId1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
		</pic:blipFill>
		<pic:spPr/>
	</pic:pic>`
	var pic Picture
	if err := xml.Unmarshal([]byte(xmlStr), &pic); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if pic.NvPicPr == nil {
		t.Fatal("NvPicPr = nil")
	}
	if pic.NvPicPr.CNvPr == nil || pic.NvPicPr.CNvPr.ID != 1 || pic.NvPicPr.CNvPr.Name != "Picture 1" {
		t.Errorf("CNvPr = %v", pic.NvPicPr.CNvPr)
	}
	if pic.NvPicPr.CNvPicPr == nil {
		t.Error("CNvPicPr = nil")
	}
}

func TestWpsDrawingEffectList(t *testing.T) {
	effects := wpsDrawingEffectList(
		&wpsOuterShadowEffect{
			BlurRad: 12700,
			Dist:    25400,
			Dir:     5400000,
			SrgbClr: &wpsEffectColorRef{Val: "FF0000", Alpha: &wpsEffectAlpha{Val: 50000}},
		},
		&wpsInnerShadowEffect{
			BlurRad:   25400,
			Dist:      12700,
			Dir:       2700000,
			SchemeClr: &wpsEffectSchemeColor{Val: "accent1", Alpha: &wpsEffectAlpha{Val: 25000}},
		},
		&wpsBlurEffect{Rad: 38100},
		&wpsGlowEffect{
			Rad:       25400,
			SchemeClr: &wpsEffectSchemeColor{Val: "accent2", Alpha: &wpsEffectAlpha{Val: 75000}},
		},
		&wpsSoftEdgeEffect{Rad: 12700},
		&wpsReflectionEffect{BlurRad: 12700, Dist: 25400, Dir: 16200000, StA: 60000},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		map[string]string{"accent1": "112233", "accent2": "AABBCC"},
	).Effects

	if len(effects) != 6 {
		t.Fatalf("effect count = %d, want 6", len(effects))
	}

	outer, ok := effects[0].(drawing.Shadow)
	if !ok {
		t.Fatalf("effect[0] = %T, want drawing.Shadow", effects[0])
	}
	if outer.Color != "FF0000" || !almostEqual(outer.Alpha, 0.5) || !almostEqual(outer.Blur, 1) || !almostEqual(outer.Distance, 2) || !almostEqual(outer.Angle, 90) {
		t.Fatalf("outer shadow = %+v", outer)
	}

	inner, ok := effects[1].(drawing.InnerShadow)
	if !ok {
		t.Fatalf("effect[1] = %T, want drawing.InnerShadow", effects[1])
	}
	if inner.Color != "112233" || !almostEqual(inner.Alpha, 0.25) || !almostEqual(inner.Blur, 2) || !almostEqual(inner.Distance, 1) || !almostEqual(inner.Angle, 45) {
		t.Fatalf("inner shadow = %+v", inner)
	}

	glow, ok := effects[2].(drawing.Glow)
	if !ok {
		t.Fatalf("effect[2] = %T, want drawing.Glow", effects[2])
	}
	if glow.Color != "AABBCC" || !almostEqual(glow.Alpha, 0.75) || !almostEqual(glow.Radius, 2) {
		t.Fatalf("glow = %+v", glow)
	}

	softEdge, ok := effects[3].(drawing.SoftEdge)
	if !ok {
		t.Fatalf("effect[3] = %T, want drawing.SoftEdge", effects[3])
	}
	if !almostEqual(softEdge.Radius, 1) {
		t.Fatalf("softEdge = %+v", softEdge)
	}

	reflection, ok := effects[4].(drawing.Reflection)
	if !ok {
		t.Fatalf("effect[4] = %T, want drawing.Reflection", effects[4])
	}
	if !almostEqual(reflection.Alpha, 0.6) || !almostEqual(reflection.Blur, 1) || !almostEqual(reflection.Distance, 2) || !almostEqual(reflection.Direction, 270) || !reflection.Fade {
		t.Fatalf("reflection = %+v", reflection)
	}

	blur, ok := effects[5].(drawing.Blur)
	if !ok {
		t.Fatalf("effect[5] = %T, want drawing.Blur", effects[5])
	}
	if !almostEqual(blur.Radius, 3) {
		t.Fatalf("blur = %+v", blur)
	}
}

func TestWpsDrawingEffectListExtendedEffects(t *testing.T) {
	effects := wpsDrawingEffectList(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		&wpsPresetShadowEffect{Prst: "shdw2", Dist: 12700, Dir: 5400000, SrgbClr: &wpsEffectColorRef{Val: "445566"}},
		&wpsBiLevelEffect{Thresh: 33000},
		&wpsAlphaEffect{Val: 50000},
		&wpsAlphaEffect{Val: -10000},
		&wpsAlphaEffect{Val: 80000},
		&wpsOutsetEffect{Rad: 25400},
		&wpsAlphaEffect{Val: 25000},
		&struct{}{},
		&struct{}{},
		&struct{}{},
		&struct{}{},
		&wpsBiLevelEffect{Thresh: 42000},
		&wpsLumEffect{Bright: 120000, Contrast: 80000},
		&wpsHSLEffect{Hue: 1800000, Sat: 50000, Lum: 125000},
		&struct{}{},
		&struct{}{},
		&wpsValueEffect{Val: 25000},
		&wpsDuotoneEffect{
			SRGBClrs: []*ThemeSRGBColor{{Val: "111111"}, {Val: "EEEEEE"}},
		},
		&wpsClrChangeEffect{
			ClrFrom: &wpsColorChoiceRaw{SRGBClr: &ThemeSRGBColor{Val: "222222"}},
			ClrTo:   &wpsColorChoiceRaw{SRGBClr: &ThemeSRGBColor{Val: "DDDDDD"}},
		},
		&wpsClrReplEffect{SRGBClr: &ThemeSRGBColor{Val: "ABC123"}},
		&wpsFillOverlayEffect{
			Blend:     "mult",
			SolidFill: &drawingxml.SolidFill{SRGBClr: &drawingxml.SRGBClr{Val: "ABCDEF"}},
		},
		nil,
	).Effects

	if len(effects) != 21 {
		t.Fatalf("effect count = %d, want 21", len(effects))
	}

	if shadow, ok := effects[0].(drawing.Shadow); !ok || shadow.Color != "445566" || !almostEqual(shadow.Distance, 1) {
		t.Fatalf("preset shadow = %#v", effects[0])
	}
	if alpha, ok := effects[1].(drawing.AlphaBiLevel); !ok || alpha.Threshold != 33000 {
		t.Fatalf("alphaBiLevel = %#v", effects[1])
	}
	if alpha, ok := effects[2].(drawing.AlphaMod); !ok || alpha.Amount != 50000 {
		t.Fatalf("alphaMod = %#v", effects[1])
	}
	if alpha, ok := effects[3].(drawing.AlphaOff); !ok || alpha.Amount != -10000 {
		t.Fatalf("alphaOff = %#v", effects[3])
	}
	if alpha, ok := effects[4].(drawing.AlphaModFix); !ok || alpha.Amount != 80000 {
		t.Fatalf("alphaModFix = %#v", effects[4])
	}
	if alpha, ok := effects[5].(drawing.AlphaOutset); !ok || !almostEqual(alpha.Radius, 2) {
		t.Fatalf("alphaOutset = %#v", effects[5])
	}
	if alpha, ok := effects[6].(drawing.AlphaRepl); !ok || alpha.Alpha != 25000 {
		t.Fatalf("alphaRepl = %#v", effects[6])
	}
	if _, ok := effects[7].(drawing.AlphaInv); !ok {
		t.Fatalf("alphaInv = %#v", effects[7])
	}
	if _, ok := effects[8].(drawing.AlphaCeiling); !ok {
		t.Fatalf("alphaCeiling = %#v", effects[8])
	}
	if _, ok := effects[9].(drawing.AlphaFloor); !ok {
		t.Fatalf("alphaFloor = %#v", effects[9])
	}
	if _, ok := effects[10].(drawing.Grayscale); !ok {
		t.Fatalf("grayscale = %#v", effects[10])
	}
	if biLevel, ok := effects[11].(drawing.BiLevel); !ok || biLevel.Threshold != 42000 {
		t.Fatalf("biLevel = %#v", effects[11])
	}
	if lum, ok := effects[12].(drawing.Lum); !ok || lum.Brightness != 120000 || lum.Contrast != 80000 {
		t.Fatalf("lum = %#v", effects[12])
	}
	if hsl, ok := effects[13].(drawing.HSLEffect); !ok || hsl.Hue != 1800000 || hsl.Sat != 50000 || hsl.Lum != 125000 {
		t.Fatalf("hsl = %#v", effects[13])
	}

	if _, ok := effects[14].(drawing.DeSat); !ok {
		t.Fatalf("deSat = %#v", effects[14])
	}
	if _, ok := effects[15].(drawing.DeSatLow); !ok {
		t.Fatalf("deSatLow = %#v", effects[15])
	}
	if deSatMod, ok := effects[16].(drawing.DeSatMod); !ok || deSatMod.Amount != 25000 {
		t.Fatalf("deSatMod = %#v", effects[16])
	}
	if duotone, ok := effects[17].(drawing.Duotone); !ok || duotone.Color1 != "111111" || duotone.Color2 != "EEEEEE" {
		t.Fatalf("duotone = %#v", effects[17])
	}
	if clrChange, ok := effects[18].(drawing.ClrChange); !ok || clrChange.From != "222222" || clrChange.To != "DDDDDD" {
		t.Fatalf("clrChange = %#v", effects[18])
	}
	if clrRepl, ok := effects[19].(drawing.ClrRepl); !ok || clrRepl.Color != "ABC123" {
		t.Fatalf("clrRepl = %#v", effects[19])
	}
	fillOverlay, ok := effects[20].(drawing.FillOverlay)
	if !ok {
		t.Fatalf("fillOverlay = %#v", effects[20])
	}
	solid, ok := fillOverlay.Fill.(*drawing.SolidFill)
	if !ok || solid.Color != "ABCDEF" || fillOverlay.BlendMode != "mult" {
		t.Fatalf("fillOverlay fill = %#v", fillOverlay)
	}
}

func TestWpsShapePropertiesEffectDag(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectDag type="sib">
			<a:outerShdw blurRad="12700" dist="25400" dir="5400000">
				<a:srgbClr val="FF0000"/>
			</a:outerShdw>
			<a:cont type="tree">
				<a:glow rad="25400">
					<a:srgbClr val="00FF00"/>
				</a:glow>
				<a:xfrm sx="50000" sy="125000" tx="12700" ty="-12700"/>
			</a:cont>
			<a:fill>
				<a:solidFill>
					<a:srgbClr val="ABCDEF"/>
				</a:solidFill>
			</a:fill>
			<a:relOff tx="25000" ty="-10000"/>
			<a:blend blend="mult">
				<a:cont>
					<a:blur rad="38100"/>
				</a:cont>
			</a:blend>
		</a:effectDag>
	</wps:spPr>`

	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	effects := p.DrawingEffectList(nil).Effects
	if len(effects) != 7 {
		t.Fatalf("effect count = %d, want 7", len(effects))
	}
	if shadow, ok := effects[0].(drawing.Shadow); !ok || shadow.Color != "FF0000" || !almostEqual(shadow.Blur, 1) {
		t.Fatalf("effect[0] = %#v", effects[0])
	}
	if glow, ok := effects[1].(drawing.Glow); !ok || glow.Color != "00FF00" || !almostEqual(glow.Radius, 2) {
		t.Fatalf("effect[1] = %#v", effects[1])
	}
	if xfrm, ok := effects[2].(drawing.TransformEffect); !ok || !almostEqual(xfrm.ScaleX, 0.5) || !almostEqual(xfrm.ScaleY, 1.25) || !almostEqual(xfrm.TranslateX, 1) || !almostEqual(xfrm.TranslateY, -1) {
		t.Fatalf("effect[2] = %#v", effects[2])
	}
	fillOverlay, ok := effects[3].(drawing.FillOverlay)
	if !ok {
		t.Fatalf("effect[3] = %#v", effects[3])
	}
	if solid, ok := fillOverlay.Fill.(*drawing.SolidFill); !ok || solid.Color != "ABCDEF" || fillOverlay.BlendMode != "over" {
		t.Fatalf("effect[3] fill = %#v", effects[3])
	}
	if relOff, ok := effects[4].(drawing.RelativeOffset); !ok || math.Abs(relOff.X-0.25) > 1e-9 || math.Abs(relOff.Y+0.10) > 1e-9 {
		t.Fatalf("effect[4] = %#v", effects[4])
	}
	if blur, ok := effects[5].(drawing.Blur); !ok || !almostEqual(blur.Radius, 3) {
		t.Fatalf("effect[5] = %#v", effects[5])
	}
	if blend, ok := effects[6].(drawing.BlendMode); !ok || blend.Mode != "mult" || blend.ContainerType != "sib" {
		t.Fatalf("effect[6] = %#v", effects[6])
	}
}

func TestWpsShapePropertiesEffectDagAlphaModContainer(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectDag>
			<a:alphaMod>
				<a:cont>
					<a:alphaRepl val="25000"/>
				</a:cont>
			</a:alphaMod>
		</a:effectDag>
	</wps:spPr>`

	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	effects := p.DrawingEffectList(nil).Effects
	if len(effects) != 1 {
		t.Fatalf("effect count = %d, want 1", len(effects))
	}
	alpha, ok := effects[0].(drawing.AlphaModFix)
	if !ok || alpha.Amount != 25000 || alpha.ContainerType != "sib" {
		t.Fatalf("effect[0] = %#v, want drawing.AlphaModFix(25000)", effects[0])
	}
}

func TestWpsShapePropertiesEffectDagTreeBlendAndAlphaPreserveContainerType(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectDag type="tree">
			<a:alphaMod>
				<a:cont>
					<a:alphaRepl val="50000"/>
				</a:cont>
			</a:alphaMod>
			<a:blend blend="mult">
				<a:cont>
					<a:blur rad="38100"/>
				</a:cont>
			</a:blend>
		</a:effectDag>
	</wps:spPr>`

	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	effects := p.DrawingEffectList(nil).Effects
	if len(effects) != 3 {
		t.Fatalf("effect count = %d, want 3", len(effects))
	}
	alpha, ok := effects[0].(drawing.AlphaModFix)
	if !ok || alpha.Amount != 50000 || alpha.ContainerType != "tree" {
		t.Fatalf("effect[0] = %#v", effects[0])
	}
	blur, ok := effects[1].(drawing.Blur)
	if !ok || !almostEqual(blur.Radius, 3) || blur.ContainerType != "sib" {
		t.Fatalf("effect[1] = %#v", effects[1])
	}
	blend, ok := effects[2].(drawing.BlendMode)
	if !ok || blend.Mode != "mult" || blend.ContainerType != "tree" {
		t.Fatalf("effect[2] = %#v", effects[2])
	}
}

func TestWpsShapePropertiesEffectDagBlendWrapsFillOverlay(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectDag>
			<a:blend blend="screen">
				<a:cont>
					<a:fill>
						<a:solidFill>
							<a:srgbClr val="ABCDEF"/>
						</a:solidFill>
					</a:fill>
				</a:cont>
			</a:blend>
		</a:effectDag>
	</wps:spPr>`

	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	effects := p.DrawingEffectList(nil).Effects
	if len(effects) != 1 {
		t.Fatalf("effect count = %d, want 1", len(effects))
	}
	fillOverlay, ok := effects[0].(drawing.FillOverlay)
	if !ok {
		t.Fatalf("effect[0] = %#v, want drawing.FillOverlay", effects[0])
	}
	solid, ok := fillOverlay.Fill.(*drawing.SolidFill)
	if !ok || solid.Color != "ABCDEF" || fillOverlay.BlendMode != "screen" || fillOverlay.ContainerType != "sib" {
		t.Fatalf("effect[0] = %#v", fillOverlay)
	}
}

func TestWpsShapePropertiesEffectDagBlendWrapsMultipleFillOverlays(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectDag>
			<a:blend blend="screen">
				<a:cont>
					<a:fill>
						<a:solidFill>
							<a:srgbClr val="ABCDEF"/>
						</a:solidFill>
					</a:fill>
					<a:fill>
						<a:solidFill>
							<a:srgbClr val="123456"/>
						</a:solidFill>
					</a:fill>
				</a:cont>
			</a:blend>
		</a:effectDag>
	</wps:spPr>`

	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	effects := p.DrawingEffectList(nil).Effects
	if len(effects) != 2 {
		t.Fatalf("effect count = %d, want 2", len(effects))
	}
	fo0, ok0 := effects[0].(drawing.FillOverlay)
	if !ok0 || fo0.BlendMode != "screen" {
		t.Fatalf("effect[0] = %#v, want FillOverlay with blendMode=screen", effects[0])
	}
	fo1, ok1 := effects[1].(drawing.FillOverlay)
	if !ok1 || fo1.BlendMode != "screen" {
		t.Fatalf("effect[1] = %#v, want FillOverlay with blendMode=screen", effects[1])
	}
}

func TestWpsShapePropertiesEffectDagPreservesContainerTypeOnShadowAndGlow(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectDag type="tree">
			<a:glow rad="25400">
				<a:srgbClr val="00FF00"/>
			</a:glow>
			<a:outerShdw blurRad="12700" dist="25400" dir="5400000">
				<a:srgbClr val="000000"/>
			</a:outerShdw>
		</a:effectDag>
	</wps:spPr>`

	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	effects := p.DrawingEffectList(nil).Effects
	if len(effects) != 2 {
		t.Fatalf("effect count = %d, want 2", len(effects))
	}
	glow, ok := effects[0].(drawing.Glow)
	if !ok || glow.ContainerType != "tree" || glow.Color != "00FF00" {
		t.Fatalf("effect[0] = %#v", effects[0])
	}
	shadow, ok := effects[1].(drawing.Shadow)
	if !ok || shadow.ContainerType != "tree" || shadow.Color != "000000" {
		t.Fatalf("effect[1] = %#v", effects[1])
	}
}

func TestWpsShapePropertiesEffectDagEffectReference(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectDag>
			<a:cont name="sharedGlow">
				<a:glow rad="25400">
					<a:srgbClr val="00FF00"/>
				</a:glow>
			</a:cont>
			<a:effect ref="sharedGlow"/>
		</a:effectDag>
	</wps:spPr>`

	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	effects := p.DrawingEffectList(nil).Effects
	if len(effects) != 2 {
		t.Fatalf("effect count = %d, want 2", len(effects))
	}
	for i, effect := range effects {
		glow, ok := effect.(drawing.Glow)
		if !ok || glow.Color != "00FF00" || !almostEqual(glow.Radius, 2) {
			t.Fatalf("effect[%d] = %#v", i, effect)
		}
	}
}

func TestWpsShapePropertiesEffectDagEffectReferenceCycle(t *testing.T) {
	xmlStr := `<wps:spPr xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:effectDag>
			<a:cont name="loop">
				<a:effect ref="loop"/>
			</a:cont>
			<a:effect ref="loop"/>
		</a:effectDag>
	</wps:spPr>`

	var p WpsShapeProperties
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	effects := p.DrawingEffectList(nil).Effects
	if len(effects) != 0 {
		t.Fatalf("effect count = %d, want 0", len(effects))
	}
}

func TestWpsShapePropertiesDrawingEffectListAndThreeD(t *testing.T) {
	shape := &WpsShapeProperties{
		Scene3D: &wpsScene3D{
			Camera: &wpsSceneCamera{
				Prst: "perspectiveAbove",
				Rot:  &wpsSceneRotation{Lat: 5400000, Lon: 10800000, Rev: 1800000},
			},
			LightRig: &wpsSceneLightRig{Rig: "threePt", Dir: "t"},
		},
		Shape3D: &wpsShape3DRaw{
			Z:            12700,
			ExtrusionH:   25400,
			ContourW:     6350,
			PrstMaterial: "metal",
			BevelT:       &wpsShape3DBevelRaw{W: 12700, H: 6350},
			BevelB:       &wpsShape3DBevelRaw{W: 25400, H: 12700},
			ContourClr:   &wpsShape3DColorRef{SchemeClr: &DrawingSchemeClr{Val: "accent1"}},
			ExtrusionClr: &wpsShape3DColorRef{SRGBClr: &ThemeSRGBColor{Val: "ABCDEF"}},
		},
		Glow: &wpsGlowEffect{Rad: 12700, SrgbClr: &wpsEffectColorRef{Val: "00FF00"}},
	}

	if got := wpsDrawingThreeDEffect(nil, nil, nil); got != nil {
		t.Fatalf("wpsDrawingThreeDEffect(nil,nil,nil) = %+v, want nil", got)
	}

	effects := shape.DrawingEffectList(map[string]string{"accent1": "123456"}).Effects
	if len(effects) != 2 {
		t.Fatalf("effect count = %d, want 2", len(effects))
	}

	threeD, ok := effects[0].(drawing.ThreeD)
	if !ok {
		t.Fatalf("effect[0] = %T, want drawing.ThreeD", effects[0])
	}
	if threeD.CameraPreset != "perspectiveAbove" || threeD.LightRig != "threePt" || threeD.LightDirection != "t" || threeD.Material != "metal" {
		t.Fatalf("threeD scene/material = %+v", threeD)
	}
	if !almostEqual(threeD.RotationX, 90) || !almostEqual(threeD.RotationY, 180) || !almostEqual(threeD.RotationZ, 30) {
		t.Fatalf("threeD rotations = %+v", threeD)
	}
	if !almostEqual(threeD.ExtrusionDepth, 2) || !almostEqual(threeD.ContourWidth, 0.5) {
		t.Fatalf("threeD depth/contour = %+v", threeD)
	}
	if !almostEqual(threeD.BevelTopWidth, 1) || !almostEqual(threeD.BevelTopHeight, 0.5) || !almostEqual(threeD.BevelBottomWidth, 2) || !almostEqual(threeD.BevelBottomHeight, 1) {
		t.Fatalf("threeD bevels = %+v", threeD)
	}
	if threeD.ContourColor != "123456" || threeD.ExtrusionColor != "ABCDEF" {
		t.Fatalf("threeD colors = %+v", threeD)
	}

	glow, ok := effects[1].(drawing.Glow)
	if !ok {
		t.Fatalf("effect[1] = %T, want drawing.Glow", effects[1])
	}
	if glow.Color != "00FF00" || !almostEqual(glow.Radius, 1) {
		t.Fatalf("glow = %+v", glow)
	}
}

func TestWpsDrawingFillAppliesColorTransforms(t *testing.T) {
	fill := wpsDrawingFill(&wpsFillOverlayEffect{
		SolidFill: &drawingxml.SolidFill{
			SchemeClr: &drawingxml.SchemeClr{
				Val:      "accent1",
				AlphaMod: &drawingxml.ColorTransform{Val: 50000},
				Shade:    &drawingxml.ColorTransform{Val: 25000},
				Tint:     &drawingxml.ColorTransform{Val: 12500},
				RedOff:   &drawingxml.ColorTransform{Val: 10000},
				GreenMod: &drawingxml.ColorTransform{Val: 50000},
				Blue:     &drawingxml.ColorTransform{Val: 25000},
				Hue:      &drawingxml.ColorTransform{Val: 7200000},
				Sat:      &drawingxml.ColorTransform{Val: 25000},
				Lum:      &drawingxml.ColorTransform{Val: 75000},
				Gray:     &struct{}{},
				Comp:     &struct{}{},
				Inv:      &struct{}{},
				Gamma:    &struct{}{},
				InvGamma: &struct{}{},
			},
		},
	}, map[string]string{"accent1": "FF0000"})

	solid, ok := fill.(*drawing.SolidFill)
	if !ok {
		t.Fatalf("fill = %T, want *drawing.SolidFill", fill)
	}
	if solid.Color == "" || solid.Color == "FF0000" {
		t.Fatalf("solid.Color = %q, want transformed resolved color", solid.Color)
	}
	if !solid.Gray {
		t.Fatalf("solid.Gray = false, want true")
	}
	if !solid.HasShade || !almostEqual(solid.Shade, 0.25) {
		t.Fatalf("solid.Shade = %v (set=%v), want 0.25 and true", solid.Shade, solid.HasShade)
	}
	if !solid.HasTint || !almostEqual(solid.Tint, 0.125) {
		t.Fatalf("solid.Tint = %v (set=%v), want 0.125 and true", solid.Tint, solid.HasTint)
	}
	if !solid.HasRedOff || !almostEqual(solid.RedOff, 0.1) {
		t.Fatalf("solid.RedOff = %v (set=%v), want 0.1 and true", solid.RedOff, solid.HasRedOff)
	}
	if !solid.HasGreenMod || !almostEqual(solid.GreenMod, 0.5) {
		t.Fatalf("solid.GreenMod = %v (set=%v), want 0.5 and true", solid.GreenMod, solid.HasGreenMod)
	}
	if !solid.HasBlue || !almostEqual(solid.Blue, 0.25) {
		t.Fatalf("solid.Blue = %v (set=%v), want 0.25 and true", solid.Blue, solid.HasBlue)
	}
	if !solid.HasHue || !almostEqual(solid.Hue, 120) {
		t.Fatalf("solid.Hue = %v (set=%v), want 120 and true", solid.Hue, solid.HasHue)
	}
	if !solid.HasSat || !almostEqual(solid.Sat, 0.25) {
		t.Fatalf("solid.Sat = %v (set=%v), want 0.25 and true", solid.Sat, solid.HasSat)
	}
	if !solid.HasLum || !almostEqual(solid.Lum, 0.75) {
		t.Fatalf("solid.Lum = %v (set=%v), want 0.75 and true", solid.Lum, solid.HasLum)
	}
	if !solid.Comp || !solid.Inv || !solid.Gamma || !solid.InvGamma {
		t.Fatalf("solid boolean transforms = %+v", solid)
	}
	if !almostEqual(solid.ResolvedAlpha(), 0.5) {
		t.Fatalf("solid alpha = %v, want 0.5", solid.ResolvedAlpha())
	}

	gradient := wpsDrawingFill(&wpsFillOverlayEffect{
		GradFill: &drawingxml.GradientFill{
			StopList: []drawingxml.GradientStop{{
				Pos: 0,
				SchemeClr: &drawingxml.SchemeClr{
					Val:      "accent2",
					AlphaMod: &drawingxml.ColorTransform{Val: 25000},
					Tint:     &drawingxml.ColorTransform{Val: 50000},
					BlueOff:  &drawingxml.ColorTransform{Val: 25000},
					Hue:      &drawingxml.ColorTransform{Val: 1800000},
					Gray:     &struct{}{},
				},
			}},
		},
	}, map[string]string{"accent2": "00FF00"})
	grad, ok := gradient.(*drawing.GradientFill)
	if !ok {
		t.Fatalf("gradient fill = %T, want *drawing.GradientFill", gradient)
	}
	if len(grad.Stops) != 1 || grad.Stops[0].Color == "" || grad.Stops[0].Color == "accent2" {
		t.Fatalf("gradient stops = %+v, want resolved stop color", grad.Stops)
	}
	if grad.Stops[0].Color == "00FF00" {
		t.Fatalf("gradient stop color = %q, want transformed color", grad.Stops[0].Color)
	}
	if !almostEqual(grad.Stops[0].Alpha, 0.25) {
		t.Fatalf("gradient alpha = %v, want 0.25", grad.Stops[0].Alpha)
	}
}

func TestGeometryGuidesAndCustomValueEvaluation(t *testing.T) {
	guides := geometryGuidesToDrawing([]wpsGeometryGuideRaw{
		{Name: "pct", Fmla: "val 50000"},
		{Name: "abs", Fmla: "val 144000"},
		{Name: "expr", Fmla: "wd/2"},
		{Name: "bad", Fmla: "val nope"},
	})
	if len(guides) != 4 {
		t.Fatalf("guide count = %d, want 4", len(guides))
	}
	if !almostEqual(guides[0].Value, 50000) {
		t.Fatalf("guide[0] = %+v, want val 50000", guides[0])
	}
	if !almostEqual(guides[1].Value, 144000) {
		t.Fatalf("guide[1] = %+v, want absolute value", guides[1])
	}
	if guides[2].Formula != "wd/2" {
		t.Fatalf("guide[2] = %+v, want formula wd/2", guides[2])
	}
	if guides[3].Formula != "val nope" {
		t.Fatalf("guide[3] = %+v, want formula fallback", guides[3])
	}

	ctx := drawing.GuideContext{
		Width:  200,
		Height: 100,
		Guides: map[string]float64{"adj1": 50},
	}
	if got, err := drawingxml.EvaluateCustomValue("wd/2", ctx); err != nil || !almostEqual(got, 100) {
		t.Fatalf("EvaluateCustomValue(wd/2) = (%v, %v), want 100", got, err)
	}
	if got, err := drawingxml.EvaluateCustomValue("adj1+10", ctx); err != nil || !almostEqual(got, 60) {
		t.Fatalf("EvaluateCustomValue(adj1+10) = (%v, %v), want 60", got, err)
	}
	pt, err := drawingxml.EvaluateCustomPoint(FormulaPoint{X: "adj1", Y: "ht/2"}, ctx)
	if err != nil {
		t.Fatalf("evaluateCustomPoint: %v", err)
	}
	if !almostEqual(pt.X, 50) || !almostEqual(pt.Y, 50) {
		t.Fatalf("point = %+v, want {50,50}", pt)
	}
	if _, err := drawingxml.EvaluateCustomValue("unknownGuide", ctx); err == nil {
		t.Fatal("EvaluateCustomValue(unknownGuide) error = nil, want error")
	}
}

func TestCustomGeometryDrawingPath(t *testing.T) {
	geom := (&CustomGeometry{
		AvLst: []wpsGeometryGuideRaw{{Name: "halfWidth", Fmla: "val 50000"}},
		GdLst: []wpsGeometryGuideRaw{{Name: "halfWidthAbs", Fmla: "wd/2"}},
		Paths: []CustomPath{{
			W:    200,
			H:    100,
			Fill: "none",
			Commands: []CustomPathCommand{
				{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
				{Type: "L", Points: []FormulaPoint{{X: "halfWidthAbs", Y: "ht"}}},
				{Type: "A", Args: []string{"50", "25", "0", "10800000"}},
				{Type: "Z"},
			},
		}},
	}).DrawingGeometry()

	if geom == nil {
		t.Fatal("DrawingGeometry returned nil")
	}
	if geom.Type != drawing.GeometryTypeCustom {
		t.Fatalf("geometry type = %q, want custom", geom.Type)
	}
	if len(geom.Guides) != 2 {
		t.Fatalf("guide count = %d, want 2", len(geom.Guides))
	}
	if len(geom.Paths) != 1 {
		t.Fatalf("path count = %d, want 1", len(geom.Paths))
	}
	path := geom.Paths[0]
	if path.FillRule != "nonzero" {
		t.Fatalf("fill rule = %q, want nonzero", path.FillRule)
	}
	if len(path.Commands) != 4 {
		t.Fatalf("command count = %d, want 4", len(path.Commands))
	}
	if path.Commands[0].Type != "M" || !almostEqual(path.Commands[0].Points[0].X, 0) || !almostEqual(path.Commands[0].Points[0].Y, 0) {
		t.Fatalf("move command = %+v", path.Commands[0])
	}
	if path.Commands[1].Type != "L" || !almostEqual(path.Commands[1].Points[0].X, 0.5) || !almostEqual(path.Commands[1].Points[0].Y, 1) {
		t.Fatalf("line command = %+v", path.Commands[1])
	}
	if path.Commands[2].Type != "A" {
		t.Fatalf("arc command type = %q, want A", path.Commands[2].Type)
	}
	if len(path.Commands[2].Args) != 5 || !almostEqual(path.Commands[2].Args[0], 0.25) || !almostEqual(path.Commands[2].Args[1], 0.25) || !almostEqual(path.Commands[2].Args[3], 0) || !almostEqual(path.Commands[2].Args[4], 1) {
		t.Fatalf("arc args = %#v", path.Commands[2].Args)
	}
	if !almostEqual(path.Commands[2].Points[0].X, 0) || !almostEqual(path.Commands[2].Points[0].Y, 1) {
		t.Fatalf("arc end point = %+v", path.Commands[2].Points[0])
	}
	if path.Commands[3].Type != "Z" {
		t.Fatalf("close command type = %q, want Z", path.Commands[3].Type)
	}
}

func TestCustomGeometryAbsoluteValGuides(t *testing.T) {
	geom := (&CustomGeometry{
		GdLst: []wpsGeometryGuideRaw{
			{Name: "halfW", Fmla: "val 50000"},
			{Name: "quarterH", Fmla: "val 25000"},
		},
		Paths: []CustomPath{{
			W: 200, H: 100,
			Commands: []CustomPathCommand{
				{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
				{Type: "L", Points: []FormulaPoint{{X: "halfW", Y: "quarterH"}}},
				{Type: "Z"},
			},
		}},
	}).DrawingGeometry()

	if geom == nil {
		t.Fatal("DrawingGeometry returned nil")
	}
	if len(geom.Paths) != 1 {
		t.Fatalf("path count = %d, want 1", len(geom.Paths))
	}
	path := geom.Paths[0]
	if len(path.Commands) != 3 {
		t.Fatalf("command count = %d, want 3", len(path.Commands))
	}
	// halfW = 50000 absolute, normalized by width 200 => 250
	if !almostEqual(path.Commands[1].Points[0].X, 250) {
		t.Errorf("line x = %v, want 250", path.Commands[1].Points[0].X)
	}
	// quarterH = 25000 absolute, normalized by height 100 => 250
	if !almostEqual(path.Commands[1].Points[0].Y, 250) {
		t.Errorf("line y = %v, want 250", path.Commands[1].Points[0].Y)
	}
}

func TestCustomGeometryNamedConstants(t *testing.T) {
	geom := (&CustomGeometry{
		GdLst: []wpsGeometryGuideRaw{
			{Name: "useSsd", Fmla: "ssd/2"},
			{Name: "useLsd", Fmla: "lsd/2"},
			{Name: "useCd2", Fmla: "cd2"},
		},
		Paths: []CustomPath{{
			W: 200, H: 100,
			Commands: []CustomPathCommand{
				{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
				{Type: "L", Points: []FormulaPoint{{X: "useSsd", Y: "useLsd"}}},
				{Type: "L", Points: []FormulaPoint{{X: "l", Y: "t"}}},
				{Type: "L", Points: []FormulaPoint{{X: "r", Y: "b"}}},
				{Type: "L", Points: []FormulaPoint{{X: "useCd2", Y: "0"}}},
				{Type: "Z"},
			},
		}},
	}).DrawingGeometry()

	if geom == nil {
		t.Fatal("DrawingGeometry returned nil")
	}
	if len(geom.Paths) != 1 {
		t.Fatalf("path count = %d, want 1", len(geom.Paths))
	}
	path := geom.Paths[0]
	if len(path.Commands) != 6 {
		t.Fatalf("command count = %d, want 6", len(path.Commands))
	}
	// ssd = min(200, 100) = 100, /2 = 50, normalized by width 200 => 0.25
	if !almostEqual(path.Commands[1].Points[0].X, 0.25) {
		t.Errorf("ssd point x = %v, want 0.25", path.Commands[1].Points[0].X)
	}
	// lsd = max(200, 100) = 200, /2 = 100, normalized by height 100 => 1.0
	if !almostEqual(path.Commands[1].Points[0].Y, 1.0) {
		t.Errorf("lsd point y = %v, want 1.0", path.Commands[1].Points[0].Y)
	}
	// l = 0, t = 0
	if !almostEqual(path.Commands[2].Points[0].X, 0) || !almostEqual(path.Commands[2].Points[0].Y, 0) {
		t.Errorf("lt point = %+v, want {0,0}", path.Commands[2].Points[0])
	}
	// r = width = 200, normalized => 1.0; b = height = 100, normalized => 1.0
	if !almostEqual(path.Commands[3].Points[0].X, 1.0) || !almostEqual(path.Commands[3].Points[0].Y, 1.0) {
		t.Errorf("rb point = %+v, want {1,1}", path.Commands[3].Points[0])
	}
	// cd2 = pi/2 ≈ 1.5708, normalized by width 200 => ~0.00785
	if !almostEqual(path.Commands[4].Points[0].X, math.Pi/2/200) {
		t.Errorf("cd2 point x = %v, want %v", path.Commands[4].Points[0].X, math.Pi/2/200)
	}
}

func TestCustomGeometryDiagnostics(t *testing.T) {
	geom, diags := (&CustomGeometry{
		GdLst: []wpsGeometryGuideRaw{{Name: "good", Fmla: "val 10"}},
		Paths: []CustomPath{
			{
				W: 100, H: 100,
				Commands: []CustomPathCommand{
					{Type: "M", Points: []FormulaPoint{{X: "0", Y: "0"}}},
					{Type: "L", Points: []FormulaPoint{{X: "good", Y: "good"}}},
				},
			},
			{
				W: 100, H: 100,
				Commands: []CustomPathCommand{
					{Type: "M", Points: []FormulaPoint{{X: "badFormula!!!", Y: "0"}}},
				},
			},
		},
	}).DrawingGeometryWithDiagnostics()

	if geom == nil {
		t.Fatal("DrawingGeometryWithDiagnostics returned nil")
	}
	if len(geom.Paths) != 1 {
		t.Fatalf("path count = %d, want 1 (bad path dropped)", len(geom.Paths))
	}
	if len(diags) != 1 {
		t.Fatalf("diagnostic count = %d, want 1", len(diags))
	}
	if diags[0].Code != "custom-geometry-path-failed" {
		t.Errorf("diagnostic code = %q, want custom-geometry-path-failed", diags[0].Code)
	}
}

func almostEqual(got, want float64) bool {
	return math.Abs(got-want) < 0.001
}

func TestDecodeSdtPrCheckbox14(t *testing.T) {
	xmlStr := `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">
		<w14:checkbox><w14:checked w14:val="1"/></w14:checkbox>
	</w:sdtPr>`
	dec, start := newDecoderForElement(t, xmlStr, "sdtPr")
	props, err := decodeSdtPr(dec, start)
	if err != nil {
		t.Fatalf("decodeSdtPr: %v", err)
	}
	if props.Type != "checkBox" {
		t.Errorf("type = %q, want checkBox", props.Type)
	}
	if !props.Checked14 {
		t.Errorf("Checked14 = false, want true")
	}
}

func TestDecodeSdtPrCheckbox14Unchecked(t *testing.T) {
	xmlStr := `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">
		<w14:checkbox><w14:checked w14:val="0"/></w14:checkbox>
	</w:sdtPr>`
	dec, start := newDecoderForElement(t, xmlStr, "sdtPr")
	props, err := decodeSdtPr(dec, start)
	if err != nil {
		t.Fatalf("decodeSdtPr: %v", err)
	}
	if props.Type != "checkBox" {
		t.Errorf("type = %q, want checkBox", props.Type)
	}
	if props.Checked14 {
		t.Errorf("Checked14 = true, want false")
	}
}

func TestDecodeSdtPrPlaceholder(t *testing.T) {
	xmlStr := `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:placeholder>
			<w:docPart w:val="DefaultPlaceholder_-1854013440"/>
		</w:placeholder>
		<w:showingPlcHdr/>
	</w:sdtPr>`
	dec, start := newDecoderForElement(t, xmlStr, "sdtPr")
	props, err := decodeSdtPr(dec, start)
	if err != nil {
		t.Fatalf("decodeSdtPr: %v", err)
	}
	if !props.ShowingPlcHdr {
		t.Errorf("ShowingPlcHdr = false, want true")
	}
	if props.PlaceholderDocPart != "DefaultPlaceholder_-1854013440" {
		t.Errorf("PlaceholderDocPart = %q, want DefaultPlaceholder_-1854013440", props.PlaceholderDocPart)
	}
}

func TestDecodeSdtPrPlaceholderGallery(t *testing.T) {
	xmlStr := `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:placeholder>
			<w:docPartGallery w:val="Quick Parts"/>
		</w:placeholder>
	</w:sdtPr>`
	dec, start := newDecoderForElement(t, xmlStr, "sdtPr")
	props, err := decodeSdtPr(dec, start)
	if err != nil {
		t.Fatalf("decodeSdtPr: %v", err)
	}
	if props.PlaceholderDocPartGallery != "Quick Parts" {
		t.Errorf("PlaceholderDocPartGallery = %q, want Quick Parts", props.PlaceholderDocPartGallery)
	}
}

func TestDecodeSdtPrAdditionalTypes(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		wantType string
	}{
		{"docPartList", `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docPartList/></w:sdtPr>`, "docPartList"},
		{"citation", `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:citation/></w:sdtPr>`, "citation"},
		{"equation", `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:equation/></w:sdtPr>`, "equation"},
		{"group", `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:group/></w:sdtPr>`, "group"},
		{"repeatingSection", `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:repeatingSection/></w:sdtPr>`, "repeatingSection"},
		{"repeatingSectionItem", `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:repeatingSectionItem/></w:sdtPr>`, "repeatingSectionItem"},
		{"contentPart", `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:contentPart/></w:sdtPr>`, "contentPart"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec, start := newDecoderForElement(t, tt.xml, "sdtPr")
			props, err := decodeSdtPr(dec, start)
			if err != nil {
				t.Fatalf("decodeSdtPr: %v", err)
			}
			if props.Type != tt.wantType {
				t.Errorf("type = %q, want %q", props.Type, tt.wantType)
			}
		})
	}
}

func TestEncodeSdtPrCheckbox14(t *testing.T) {
	props := &SdtProperties{Type: "checkBox", Checked14: true}
	var buf strings.Builder
	e := xml.NewEncoder(&buf)
	if err := marshalSdtProperties(e, props, "http://schemas.openxmlformats.org/wordprocessingml/2006/main"); err != nil {
		t.Fatalf("marshalSdtProperties: %v", err)
	}
	if err := e.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "checkbox") {
		t.Errorf("expected w14:checkbox in output; got:\n%s", out)
	}
	if !strings.Contains(out, "checked") {
		t.Errorf("expected checked in output; got:\n%s", out)
	}
}

func TestEncodeSdtPrPlaceholder(t *testing.T) {
	props := &SdtProperties{
		PlaceholderDocPart:        "DefaultPlaceholder",
		PlaceholderDocPartGallery: "Quick Parts",
	}
	var buf strings.Builder
	e := xml.NewEncoder(&buf)
	if err := marshalSdtProperties(e, props, "http://schemas.openxmlformats.org/wordprocessingml/2006/main"); err != nil {
		t.Fatalf("marshalSdtProperties: %v", err)
	}
	if err := e.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<placeholder ") {
		t.Errorf("expected placeholder element in output; got:\n%s", out)
	}
	if !strings.Contains(out, `val="DefaultPlaceholder"`) {
		t.Errorf("expected docPart val in output; got:\n%s", out)
	}
	if !strings.Contains(out, `val="Quick Parts"`) {
		t.Errorf("expected docPartGallery val in output; got:\n%s", out)
	}
}

func TestCompatSettingsUnmarshalDirectChildren(t *testing.T) {
	xmlStr := `<w:compat xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:noTabHangInd/>
		<w:alignTablesRowByRow w:val="true"/>
		<w:allowPNG w:val="1"/>
		<w:autoSpaceLikeWord95 w:val="false"/>
		<w:suppressTopSpacing/>
	</w:compat>`
	dec, start := newDecoderForElement(t, xmlStr, "compat")
	var cs CompatSettings
	if err := cs.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("UnmarshalXML failed: %v", err)
	}
	if len(cs.Settings) != 5 {
		t.Fatalf("expected 5 settings, got %d", len(cs.Settings))
	}
	want := map[string]string{
		"noTabHangInd":        "1",
		"alignTablesRowByRow": "1",
		"allowPNG":            "1",
		"autoSpaceLikeWord95": "0",
		"suppressTopSpacing":  "1",
	}
	got := make(map[string]string, len(cs.Settings))
	for _, s := range cs.Settings {
		got[s.Name] = s.Value
	}
	for name, val := range want {
		if got[name] != val {
			t.Errorf("%s: got %q, want %q", name, got[name], val)
		}
	}
}

func TestCustomXmlBodyBlocks(t *testing.T) {
	xmlStr := `<w:body xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:customXml w:uri="http://example.com" w:element="myElement">
			<w:customXmlPr>
				<w:attr w:uri="http://example.com" w:name="attr1" w:val="value1"/>
			</w:customXmlPr>
			<w:p><w:r><w:t>inside customXml</w:t></w:r></w:p>
			<w:tbl><w:tr><w:tc><w:p><w:r><w:t>table in customXml</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
		</w:customXml>
	</w:body>`
	dec, start := newDecoderForElement(t, xmlStr, "body")
	var b Body
	if err := b.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("UnmarshalXML failed: %v", err)
	}
	if len(b.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(b.Paragraphs))
	}
	if len(b.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(b.Tables))
	}
	if len(b.Blocks) != 1 {
		t.Fatalf("expected 1 customXml block, got %d", len(b.Blocks))
	}
	if b.Blocks[0].Kind != BodyBlockCustomXml {
		t.Fatalf("expected BodyBlockCustomXml, got %v", b.Blocks[0].Kind)
	}
	if b.Blocks[0].CustomXml == nil || b.Blocks[0].CustomXml.URI != "http://example.com" || b.Blocks[0].CustomXml.Element != "myElement" {
		t.Fatalf("customXml metadata mismatch: %+v", b.Blocks[0].CustomXml)
	}
	if len(b.Blocks[0].CustomXmlBlocks) != 2 {
		t.Fatalf("expected 2 child blocks, got %d", len(b.Blocks[0].CustomXmlBlocks))
	}
	if b.Blocks[0].CustomXml.Properties == nil || len(b.Blocks[0].CustomXml.Properties.Attributes) != 1 {
		t.Fatalf("expected 1 customXml attr, got %+v", b.Blocks[0].CustomXml.Properties)
	}
	childText := b.Blocks[0].CustomXmlBlocks[0].Paragraph.Runs[0].Contents[0].Text
	if childText == nil || childText.Value != "inside customXml" {
		t.Errorf("paragraph text = %v, want %q", childText, "inside customXml")
	}
}

func TestCustomXmlParagraphInlines(t *testing.T) {
	xmlStr := `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:customXml w:uri="http://example.com" w:element="myInline">
			<w:customXmlPr>
				<w:attr w:uri="http://example.com" w:name="attr1" w:val="value1"/>
			</w:customXmlPr>
			<w:r><w:t>run in customXml</w:t></w:r>
			<w:r><w:t>second run</w:t></w:r>
		</w:customXml>
	</w:p>`
	var p Paragraph
	if err := xml.Unmarshal([]byte(xmlStr), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(p.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(p.Runs))
	}
	if len(p.Inlines) != 1 {
		t.Fatalf("expected 1 customXml inline, got %d", len(p.Inlines))
	}
	if p.Inlines[0].Kind != ParagraphInlineCustomXml {
		t.Fatalf("expected ParagraphInlineCustomXml, got %v", p.Inlines[0].Kind)
	}
	if p.Inlines[0].CustomXml == nil || p.Inlines[0].CustomXml.URI != "http://example.com" || p.Inlines[0].CustomXml.Element != "myInline" {
		t.Fatalf("customXml metadata mismatch: %+v", p.Inlines[0].CustomXml)
	}
	if len(p.Inlines[0].CustomXmlInlines) != 2 {
		t.Fatalf("expected 2 child inlines, got %d", len(p.Inlines[0].CustomXmlInlines))
	}
	if p.Inlines[0].CustomXml.Properties == nil || len(p.Inlines[0].CustomXml.Properties.Attributes) != 1 {
		t.Fatalf("expected 1 customXml attr, got %+v", p.Inlines[0].CustomXml.Properties)
	}
	if p.Runs[0].Contents[0].Text == nil || p.Runs[0].Contents[0].Text.Value != "run in customXml" {
		t.Errorf("run0 text = %v, want %q", p.Runs[0].Contents[0].Text, "run in customXml")
	}
	if p.Runs[1].Contents[0].Text == nil || p.Runs[1].Contents[0].Text.Value != "second run" {
		t.Errorf("run1 text = %v, want %q", p.Runs[1].Contents[0].Text, "second run")
	}
}

func TestCustomXmlNested(t *testing.T) {
	xmlStr := `<w:body xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:customXml w:uri="http://example.com" w:element="outer">
			<w:customXml w:uri="http://example.com" w:element="inner">
				<w:p><w:r><w:t>nested</w:t></w:r></w:p>
			</w:customXml>
		</w:customXml>
	</w:body>`
	dec, start := newDecoderForElement(t, xmlStr, "body")
	var b Body
	if err := b.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("UnmarshalXML failed: %v", err)
	}
	if len(b.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(b.Paragraphs))
	}
	if len(b.Blocks) != 1 {
		t.Fatalf("expected 1 customXml block, got %d", len(b.Blocks))
	}
	if b.Blocks[0].Kind != BodyBlockCustomXml {
		t.Fatalf("expected BodyBlockCustomXml, got %v", b.Blocks[0].Kind)
	}
	if len(b.Blocks[0].CustomXmlBlocks) != 1 {
		t.Fatalf("expected 1 child block, got %d", len(b.Blocks[0].CustomXmlBlocks))
	}
	if b.Blocks[0].CustomXmlBlocks[0].Kind != BodyBlockCustomXml {
		t.Fatalf("expected nested BodyBlockCustomXml, got %v", b.Blocks[0].CustomXmlBlocks[0].Kind)
	}
	childText := b.Blocks[0].CustomXmlBlocks[0].CustomXmlBlocks[0].Paragraph.Runs[0].Contents[0].Text
	if childText == nil || childText.Value != "nested" {
		t.Errorf("text = %v, want %q", childText, "nested")
	}
}

func TestTableCellCustomXml(t *testing.T) {
	xmlStr := `<w:tc xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:customXml w:uri="http://example.com" w:element="cellXml">
			<w:p><w:r><w:t>cell para</w:t></w:r></w:p>
		</w:customXml>
	</w:tc>`
	var tc TableCell
	if err := xml.Unmarshal([]byte(xmlStr), &tc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(tc.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(tc.Paragraphs))
	}
	if tc.Paragraphs[0].Runs[0].Contents[0].Text == nil || tc.Paragraphs[0].Runs[0].Contents[0].Text.Value != "cell para" {
		t.Errorf("text = %v, want %q", tc.Paragraphs[0].Runs[0].Contents[0].Text, "cell para")
	}
	if len(tc.Blocks) != 1 {
		t.Fatalf("expected 1 customXml block, got %d", len(tc.Blocks))
	}
	if tc.Blocks[0].Kind != ContentBlockCustomXml {
		t.Fatalf("expected ContentBlockCustomXml, got %v", tc.Blocks[0].Kind)
	}
	if tc.Blocks[0].CustomXml == nil || tc.Blocks[0].CustomXml.URI != "http://example.com" || tc.Blocks[0].CustomXml.Element != "cellXml" {
		t.Fatalf("customXml metadata mismatch: %+v", tc.Blocks[0].CustomXml)
	}
}

func TestEncodeSdtPrListItemsWithValues(t *testing.T) {
	props := &SdtProperties{
		Type:      "dropDownList",
		ListItems: []SdtListItem{{DisplayText: "Red", Value: "R"}, {DisplayText: "Green", Value: "G"}},
	}
	var buf strings.Builder
	e := xml.NewEncoder(&buf)
	if err := marshalSdtProperties(e, props, "http://schemas.openxmlformats.org/wordprocessingml/2006/main"); err != nil {
		t.Fatalf("marshalSdtProperties: %v", err)
	}
	if err := e.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `displayText="Red"`) || !strings.Contains(out, `value="R"`) {
		t.Errorf("expected Red with value=R in output; got:\n%s", out)
	}
	if !strings.Contains(out, `displayText="Green"`) || !strings.Contains(out, `value="G"`) {
		t.Errorf("expected Green with value=G in output; got:\n%s", out)
	}
}
func TestStylePreservesAliasesAndUnsupportedChildren(t *testing.T) {
	const input = `<w:style xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml" w:type="paragraph" w:styleId="Custom"><w:name w:val="Custom"/><w:altName w:val="Alternate"/><w:aliases w:val="Alias One,Alias Two"/><w15:appearance w15:val="42"/></w:style>`
	var style Style
	if err := xml.Unmarshal([]byte(input), &style); err != nil {
		t.Fatalf("unmarshal style: %v", err)
	}
	if style.AltName == nil || style.AltName.Val != "Alternate" {
		t.Fatalf("altName = %#v", style.AltName)
	}
	if style.Aliases == nil || style.Aliases.Val != "Alias One,Alias Two" {
		t.Fatalf("aliases = %#v", style.Aliases)
	}
	if len(style.Unknown) != 1 || style.Unknown[0].XMLName.Local != "appearance" {
		t.Fatalf("unknown style children = %#v", style.Unknown)
	}
	data, err := xml.Marshal(style)
	if err != nil {
		t.Fatalf("marshal style: %v", err)
	}
	if !bytes.Contains(data, []byte("appearance")) || !bytes.Contains(data, []byte("Alternate")) {
		t.Fatalf("marshaled style lost preserved markup: %s", data)
	}
}

func TestSettingsAndSectionPreserveUnsupportedChildren(t *testing.T) {
	const settingsXML = `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml"><w:trackRevisions/><w15:chartTrackingRefBased/></w:settings>`
	var settings Settings
	if err := xml.Unmarshal([]byte(settingsXML), &settings); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if len(settings.Unknown) != 1 || settings.Unknown[0].XMLName.Local != "chartTrackingRefBased" {
		t.Fatalf("unknown settings = %#v", settings.Unknown)
	}
	settingsOut, err := xml.Marshal(settings)
	if err != nil || !bytes.Contains(settingsOut, []byte("chartTrackingRefBased")) {
		t.Fatalf("settings preservation failed: %v %s", err, settingsOut)
	}

	const sectionXML = `<w:sectPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w16="http://schemas.microsoft.com/office/word/2018/wordml"><w:pgSz w:w="12240" w:h="15840"/><w16:gutterAtTop w16:val="1"/></w:sectPr>`
	var section SectionProperties
	if err := xml.Unmarshal([]byte(sectionXML), &section); err != nil {
		t.Fatalf("unmarshal section: %v", err)
	}
	if len(section.Unknown) != 1 || section.Unknown[0].XMLName.Local != "gutterAtTop" {
		t.Fatalf("unknown section = %#v", section.Unknown)
	}
	sectionOut, err := xml.Marshal(section)
	if err != nil || !bytes.Contains(sectionOut, []byte("gutterAtTop")) {
		t.Fatalf("section preservation failed: %v %s", err, sectionOut)
	}
}

func TestVMLShapePreservesUnknownAttributesAndChildren(t *testing.T) {
	const input = `<v:shape xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" id="s1" o:allowincell="f"><v:fill color2="#112233"/><o:lock v:ext="edit" aspectratio="t"/></v:shape>`
	var shape VMLShape
	if err := xml.Unmarshal([]byte(input), &shape); err != nil {
		t.Fatalf("unmarshal vml shape: %v", err)
	}
	if len(shape.UnknownAttrs) == 0 {
		t.Fatal("unknown VML attribute was not retained")
	}
	if len(shape.Unknown) != 1 || shape.Unknown[0].XMLName.Local != "lock" {
		t.Fatalf("unknown VML children = %#v", shape.Unknown)
	}
	out, err := xml.Marshal(shape)
	if err != nil {
		t.Fatalf("marshal vml shape: %v", err)
	}
	if !bytes.Contains(out, []byte("allowincell")) || !bytes.Contains(out, []byte("lock")) || !bytes.Contains(out, []byte("aspectratio")) {
		t.Fatalf("VML extension markup lost: %s", out)
	}
}

func TestBodyPreservesUnsupportedBlocksAndWritesAltChunkAndCustomXml(t *testing.T) {
	const input = `<w:body xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml"><w:p><w:r><w:t>A</w:t></w:r></w:p><w:altChunk xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rChunk"/><w:customXml w:uri="urn:test" w:element="item"><w:p><w:r><w:t>B</w:t></w:r></w:p></w:customXml><w15:repeatingSection/><w:sectPr/></w:body>`
	var body Body
	if err := xml.Unmarshal([]byte(input), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(body.Blocks) != 5 {
		t.Fatalf("body blocks = %d, want 5", len(body.Blocks))
	}
	if body.Blocks[1].Kind != BodyBlockAltChunk || body.Blocks[2].Kind != BodyBlockCustomXml || body.Blocks[3].Kind != BodyBlockRaw {
		t.Fatalf("body block order/kinds = %#v", body.Blocks)
	}
	out, err := xml.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	for _, token := range [][]byte{[]byte("altChunk"), []byte("customXml"), []byte("repeatingSection")} {
		if !bytes.Contains(out, token) {
			t.Fatalf("body output lost %q: %s", token, out)
		}
	}
}

func TestColumnsPreserveExplicitPhysicalGeometry(t *testing.T) {
	var cols Columns
	if err := xml.Unmarshal([]byte(`<w:cols xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:num="3" w:equalWidth="0" w:sep="1"><w:col w:w="1800" w:space="240"/><w:col w:w="2600" w:space="360"/><w:col w:w="3200"/></w:cols>`), &cols); err != nil {
		t.Fatalf("unmarshal columns: %v", err)
	}
	if cols.Num != 3 || !cols.Sep || cols.EqualWidth == nil || bool(*cols.EqualWidth) {
		t.Fatalf("column attributes = %+v, want num=3 sep=true equalWidth=false", cols)
	}
	if len(cols.Cols) != 3 {
		t.Fatalf("column tracks = %d, want 3", len(cols.Cols))
	}
	if cols.Cols[0].Width != 1800 || cols.Cols[0].Space != 240 || cols.Cols[1].Width != 2600 || cols.Cols[1].Space != 360 || cols.Cols[2].Width != 3200 {
		t.Fatalf("explicit column geometry = %+v", cols.Cols)
	}
}

func TestBlipParsesWordPictureAdjustmentEffects(t *testing.T) {
	var blip Blip
	xmlText := `<a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="rId7"><a:grayscl/><a:biLevel thresh="42000"/><a:lum bright="25000" contrast="-10000"/><a:alphaModFix amt="75000"/><a:alphaRepl a="60000"/></a:blip>`
	if err := xml.Unmarshal([]byte(xmlText), &blip); err != nil {
		t.Fatalf("unmarshal blip: %v", err)
	}
	if blip.Grayscale == nil {
		t.Fatal("a:grayscl was not parsed")
	}
	if blip.BiLevel == nil || blip.BiLevel.Thresh != 42000 {
		t.Fatalf("biLevel = %#v", blip.BiLevel)
	}
	if blip.Lum == nil || blip.Lum.Bright != 25000 || blip.Lum.Contrast != -10000 {
		t.Fatalf("lum = %#v", blip.Lum)
	}
	if blip.AlphaModFix == nil || blip.AlphaModFix.Amt != 75000 {
		t.Fatalf("alphaModFix = %#v", blip.AlphaModFix)
	}
	if blip.AlphaRepl == nil || blip.AlphaRepl.A != 60000 {
		t.Fatalf("alphaRepl = %#v", blip.AlphaRepl)
	}
}

func TestDecodeSdtPrPreservesLockAppearanceTemporaryAndMultiline(t *testing.T) {
	const src = `<w:sdtPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml"><w:alias w:val="Notes"/><w:text w:multiLine="1"/><w:lock w:val="sdtContentLocked"/><w:temporary/><w15:appearance w15:val="tags"/><w15:color w15:val="5B9BD5"/></w:sdtPr>`
	dec := xml.NewDecoder(strings.NewReader(src))
	startTok, err := dec.Token()
	if err != nil {
		t.Fatal(err)
	}
	props, err := decodeSdtPr(dec, startTok.(xml.StartElement))
	if err != nil {
		t.Fatalf("decodeSdtPr: %v", err)
	}
	if props.Type != "plainText" || !props.PlainTextMultiLine {
		t.Fatalf("plain text metadata = %+v", props)
	}
	if props.Lock != "sdtContentLocked" || !props.Temporary || props.Appearance != "tags" || props.Color != "5B9BD5" {
		t.Fatalf("extended SDT metadata = %+v", props)
	}
}

func TestMarshalSdtPropertiesPreservesExtendedControlMetadata(t *testing.T) {
	props := &SdtProperties{Type: "plainText", PlainTextMultiLine: true, Lock: "contentLocked", Temporary: true, Appearance: "boundingBox", Color: "FF0000"}
	var b strings.Builder
	e := xml.NewEncoder(&b)
	if err := marshalSdtProperties(e, props, "http://schemas.openxmlformats.org/wordprocessingml/2006/main"); err != nil {
		t.Fatal(err)
	}
	if err := e.Flush(); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{`multiLine="1"`, `val="contentLocked"`, `temporary`, `appearance`, `boundingBox`, `color`, `FF0000`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %s", want, out)
		}
	}
}
