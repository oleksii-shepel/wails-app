package resolve_test

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"office/pkg/ooxml/opc"
	"office/pkg/word/parsers/ooxml/parts"
	"office/pkg/word/parsers/ooxml/resolve"
	"office/pkg/word/render"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runBool reads a *bool run format field: nil → false.
func runBool(b *bool) bool { return b != nil && *b }

func TestResolvePackageBuildsSemanticDocument(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
  <Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
  <Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:r><w:t>Default style paragraph</w:t></w:r>
    </w:p>
    <w:tbl>
      <w:tblPr>
        <w:tblCellSpacing w:w="100" w:type="dxa"/>
      </w:tblPr>
      <w:tr>
        <w:tc>
          <w:p><w:r><w:t>Cell A1</w:t></w:r></w:p>
          <w:tbl>
            <w:tr>
              <w:tc><w:p><w:r><w:t>Nested A1</w:t></w:r></w:p></w:tc>
            </w:tr>
          </w:tbl>
        </w:tc>
        <w:tc>
          <w:p><w:r><w:t>Cell B1</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
      <w:tr>
        <w:tc>
          <w:p><w:r><w:t>Cell A2</w:t></w:r></w:p>
        </w:tc>
        <w:tc>
          <w:p><w:r><w:t>Cell B2</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
    <w:p>
      <w:pPr>
        <w:pStyle w:val="Heading1"/>
        <w:numPr>
          <w:ilvl w:val="1"/>
          <w:numId w:val="9"/>
        </w:numPr>
        <w:jc w:val="right"/>
      </w:pPr>
      <w:r>
        <w:rPr>
          <w:b/>
          <w:color w:val="FF0000"/>
        </w:rPr>
        <w:t>Hello </w:t>
      </w:r>
      <w:hyperlink r:id="rId42">
        <w:r>
          <w:rPr>
            <w:i/>
            <w:u w:val="single" w:color="00FF00"/>
          </w:rPr>
          <w:t>world</w:t>
        </w:r>
      </w:hyperlink>
      <w:r><w:tab/></w:r>
      <w:r>
        <w:rPr><w:highlight w:val="yellow"/></w:rPr>
        <w:br w:type="page"/>
      </w:r>
    </w:p>
    <w:sectPr>
      <w:headerReference w:type="default" r:id="rId10"/>
      <w:footerReference w:type="default" r:id="rId11"/>
      <w:pgSz w:w="15840" w:h="12240" w:orient="landscape"/>
      <w:pgMar w:top="1440" w:right="720" w:bottom="1440" w:left="720" w:header="720" w:footer="720" w:gutter="0"/>
      <w:cols w:num="2" w:space="720" w:sep="1"/>
    </w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
  <Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
  <Relationship Id="rId11" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
  <Relationship Id="rId42" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
</Relationships>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:pPrDefault>
      <w:pPr>
        <w:spacing w:line="360" w:lineRule="auto"/>
        <w:ind w:firstLine="240"/>
      </w:pPr>
    </w:pPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:styleId="Normal" w:default="1">
    <w:name w:val="Normal"/>
    <w:pPr>
      <w:jc w:val="both"/>
      <w:spacing w:after="200"/>
      <w:ind w:left="120"/>
    </w:pPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="BaseHeading">
    <w:name w:val="Base Heading"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:spacing w:before="240"/>
    </w:pPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="Heading 1"/>
    <w:basedOn w:val="BaseHeading"/>
    <w:pPr>
      <w:jc w:val="center"/>
      <w:ind w:left="480"/>
    </w:pPr>
  </w:style>
</w:styles>`,
		"word/numbering.xml": `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="1">
      <w:start w:val="3"/>
      <w:numFmt w:val="decimal"/>
      <w:lvlText w:val="%2."/>
    </w:lvl>
  </w:abstractNum>
  <w:num w:numId="9">
    <w:abstractNumId w:val="0"/>
  </w:num>
</w:numbering>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Header</w:t></w:r></w:p>
  <w:tbl>
    <w:tr>
      <w:tc><w:p><w:r><w:t>Header table</w:t></w:r></w:p></w:tc>
    </w:tr>
  </w:tbl>
</w:hdr>`,
		"word/footer1.xml": `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Footer</w:t></w:r></w:p>
</w:ftr>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}

	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if got := len(document.Paragraphs); got != 2 {
		t.Fatalf("paragraph count = %d, want 2", got)
	}
	if got := len(document.Blocks); got != 4 {
		t.Fatalf("block count = %d, want 4", got)
	}
	if document.Blocks[0].Kind != "paragraph" || document.Blocks[1].Kind != "table" || document.Blocks[2].Kind != "paragraph" || document.Blocks[3].Kind != "section" {
		t.Fatalf("block order = %#v", []string{document.Blocks[0].Kind, document.Blocks[1].Kind, document.Blocks[2].Kind, document.Blocks[3].Kind})
	}
	// Paragraph indices must be unique across the whole document (including cell paragraphs).
	// Table index uses block-sequence position.
	if document.Tables[0].Index != 1 {
		t.Fatalf("table block index = %d, want 1", document.Tables[0].Index)
	}
	if document.Paragraphs[0].Index == document.Paragraphs[1].Index {
		t.Fatalf("top-level paragraphs share index %d", document.Paragraphs[0].Index)
	}
	if document.DefaultParagraphStyleID != "Normal" {
		t.Fatalf("default paragraph style = %q", document.DefaultParagraphStyleID)
	}
	if document.DefaultParagraphFormat.LineSpacing == nil || *document.DefaultParagraphFormat.LineSpacing != 360 || document.DefaultParagraphFormat.LineRule != "auto" {
		t.Fatalf("default paragraph format = %+v", document.DefaultParagraphFormat)
	}
	if document.DefaultParagraphFormat.IndentFirstLine == nil || *document.DefaultParagraphFormat.IndentFirstLine != 240 {
		t.Fatalf("default paragraph format = %+v", document.DefaultParagraphFormat)
	}
	if document.Styles["Heading1"].Name != "Heading 1" {
		t.Fatalf("heading style name = %q", document.Styles["Heading1"].Name)
	}
	if document.Styles["Heading1"].BasedOn != "BaseHeading" {
		t.Fatalf("heading basedOn = %q", document.Styles["Heading1"].BasedOn)
	}
	if got := document.Styles["Heading1"].StylePath; len(got) != 3 || got[0] != "Heading1" || got[1] != "BaseHeading" || got[2] != "Normal" {
		t.Fatalf("heading style path = %#v", got)
	}
	if document.Styles["Heading1"].Format.Alignment != "center" {
		t.Fatalf("heading style format alignment = %q", document.Styles["Heading1"].Format.Alignment)
	}
	if document.Styles["Heading1"].Format.SpacingBefore == nil || *document.Styles["Heading1"].Format.SpacingBefore != 240 {
		t.Fatalf("heading style format = %+v", document.Styles["Heading1"].Format)
	}
	if document.Styles["Heading1"].Format.SpacingAfter == nil || *document.Styles["Heading1"].Format.SpacingAfter != 200 {
		t.Fatalf("heading style format = %+v", document.Styles["Heading1"].Format)
	}
	if document.Styles["Heading1"].Format.IndentLeft == nil || *document.Styles["Heading1"].Format.IndentLeft != 480 {
		t.Fatalf("heading style format = %+v", document.Styles["Heading1"].Format)
	}
	if document.Styles["Heading1"].Format.LineSpacing == nil || *document.Styles["Heading1"].Format.LineSpacing != 360 {
		t.Fatalf("heading style format = %+v", document.Styles["Heading1"].Format)
	}
	if document.Paragraphs[0].StyleID != "Normal" || document.Paragraphs[0].StyleName != "Normal" {
		t.Fatalf("default paragraph style resolution failed: id=%q name=%q", document.Paragraphs[0].StyleID, document.Paragraphs[0].StyleName)
	}
	if got := document.Paragraphs[0].StylePath; len(got) != 1 || got[0] != "Normal" {
		t.Fatalf("default paragraph style path = %#v", got)
	}
	if document.Paragraphs[0].Format.Alignment != "both" {
		t.Fatalf("default paragraph format alignment = %q", document.Paragraphs[0].Format.Alignment)
	}
	if document.Paragraphs[0].Format.SpacingAfter == nil || *document.Paragraphs[0].Format.SpacingAfter != 200 {
		t.Fatalf("default paragraph format = %+v", document.Paragraphs[0].Format)
	}
	if document.Paragraphs[0].Format.IndentLeft == nil || *document.Paragraphs[0].Format.IndentLeft != 120 {
		t.Fatalf("default paragraph format = %+v", document.Paragraphs[0].Format)
	}
	if document.Paragraphs[0].Format.IndentFirstLine == nil || *document.Paragraphs[0].Format.IndentFirstLine != 240 {
		t.Fatalf("default paragraph format = %+v", document.Paragraphs[0].Format)
	}
	if document.Paragraphs[0].Text != "Default style paragraph" {
		t.Fatalf("default paragraph text = %q", document.Paragraphs[0].Text)
	}
	if document.Paragraphs[1].StyleID != "Heading1" || document.Paragraphs[1].StyleName != "Heading 1" {
		t.Fatalf("explicit paragraph style resolution failed: id=%q name=%q", document.Paragraphs[1].StyleID, document.Paragraphs[1].StyleName)
	}
	if got := document.Paragraphs[1].StylePath; len(got) != 3 || got[0] != "Heading1" || got[1] != "BaseHeading" || got[2] != "Normal" {
		t.Fatalf("explicit paragraph style path = %#v", got)
	}
	if document.Paragraphs[1].Format.Alignment != "right" {
		t.Fatalf("explicit paragraph format alignment = %q", document.Paragraphs[1].Format.Alignment)
	}
	if document.Paragraphs[1].Format.SpacingBefore == nil || *document.Paragraphs[1].Format.SpacingBefore != 240 {
		t.Fatalf("explicit paragraph format = %+v", document.Paragraphs[1].Format)
	}
	if document.Paragraphs[1].Format.SpacingAfter == nil || *document.Paragraphs[1].Format.SpacingAfter != 200 {
		t.Fatalf("explicit paragraph format = %+v", document.Paragraphs[1].Format)
	}
	if document.Paragraphs[1].Format.IndentLeft == nil || *document.Paragraphs[1].Format.IndentLeft != 480 {
		t.Fatalf("explicit paragraph format = %+v", document.Paragraphs[1].Format)
	}
	if document.Paragraphs[1].Format.LineSpacing == nil || *document.Paragraphs[1].Format.LineSpacing != 360 {
		t.Fatalf("explicit paragraph format = %+v", document.Paragraphs[1].Format)
	}
	if document.Paragraphs[1].Numbering == nil || document.Paragraphs[1].Numbering.NumID != 9 || document.Paragraphs[1].Numbering.Level != 1 {
		t.Fatal("numbering not normalized")
	}
	if document.Paragraphs[1].List == nil {
		t.Fatal("list marker not derived")
	}
	if document.Paragraphs[1].List.Kind != "ordered" {
		t.Fatalf("list kind = %q", document.Paragraphs[1].List.Kind)
	}
	if document.Paragraphs[1].List.Level != 1 || document.Paragraphs[1].List.Start != 3 {
		t.Fatalf("list marker = %+v", document.Paragraphs[1].List)
	}
	if document.Paragraphs[1].List.Format != "decimal" || document.Paragraphs[1].List.Template != "%2." {
		t.Fatalf("list marker = %+v", document.Paragraphs[1].List)
	}
	if document.Paragraphs[1].Numbering.AbstractNumID != 0 {
		t.Fatalf("abstract num = %d, want 0", document.Paragraphs[1].Numbering.AbstractNumID)
	}
	if document.Paragraphs[1].Numbering.Start != 3 {
		t.Fatalf("start = %d, want 3", document.Paragraphs[1].Numbering.Start)
	}
	if document.Paragraphs[1].Numbering.Format != "decimal" {
		t.Fatalf("format = %q", document.Paragraphs[1].Numbering.Format)
	}
	if document.Paragraphs[1].Numbering.Text != "%2." {
		t.Fatalf("text pattern = %q", document.Paragraphs[1].Numbering.Text)
	}
	if document.Paragraphs[1].Text != "Hello world\t\n" {
		t.Fatalf("text = %q", document.Paragraphs[1].Text)
	}
	if got := len(document.Paragraphs[1].Runs); got != 4 {
		t.Fatalf("run count = %d, want 4", got)
	}
	if document.Paragraphs[1].Runs[0].Kind != "run" || !runBool(document.Paragraphs[1].Runs[0].Format.Bold) || document.Paragraphs[1].Runs[0].Format.Color != "FF0000" {
		t.Fatalf("first run formatting = %+v", document.Paragraphs[1].Runs[0])
	}
	if document.Paragraphs[1].Runs[1].Kind != "hyperlink" || !runBool(document.Paragraphs[1].Runs[1].Format.Italic) || document.Paragraphs[1].Runs[1].Format.Underline != "single" || document.Paragraphs[1].Runs[1].Format.UnderlineColor != "00FF00" {
		t.Fatalf("hyperlink run formatting = %+v", document.Paragraphs[1].Runs[1])
	}
	if document.Paragraphs[1].Runs[1].Target != "https://example.com" || document.Paragraphs[1].Runs[1].Text != "world" {
		t.Fatalf("hyperlink run = %+v", document.Paragraphs[1].Runs[1])
	}
	if document.Paragraphs[1].Runs[2].Kind != "tab" || document.Paragraphs[1].Runs[2].Text != "\t" {
		t.Fatalf("tab run = %+v", document.Paragraphs[1].Runs[2])
	}
	if document.Paragraphs[1].Runs[3].Kind != "break" || document.Paragraphs[1].Runs[3].BreakType != "page" || document.Paragraphs[1].Runs[3].Format.Highlight != "yellow" {
		t.Fatalf("break run = %+v", document.Paragraphs[1].Runs[3])
	}
	if got := len(document.Tables); got != 1 {
		t.Fatalf("table count = %d, want 1", got)
	}
	if got := document.Tables[0].Format.CellSpacing; got != 100 {
		t.Fatalf("table cell spacing = %d, want 100", got)
	}
	if got := len(document.Tables[0].Rows); got != 2 {
		t.Fatalf("table row count = %d, want 2", got)
	}
	if got := len(document.Tables[0].Rows[0].Cells); got != 2 {
		t.Fatalf("table cell count = %d, want 2", got)
	}
	if document.Tables[0].Rows[0].Cells[0].Text != "Cell A1\nNested A1" {
		t.Fatalf("first cell text = %q", document.Tables[0].Rows[0].Cells[0].Text)
	}
	if got := len(document.Tables[0].Rows[0].Cells[0].Blocks); got != 2 {
		t.Fatalf("nested cell block count = %d, want 2", got)
	}
	if document.Tables[0].Rows[0].Cells[0].Blocks[0].Kind != "paragraph" || document.Tables[0].Rows[0].Cells[0].Blocks[1].Kind != "table" {
		t.Fatalf("nested cell block order = %#v", []string{document.Tables[0].Rows[0].Cells[0].Blocks[0].Kind, document.Tables[0].Rows[0].Cells[0].Blocks[1].Kind})
	}
	if got := len(document.Tables[0].Rows[0].Cells[0].Tables); got != 1 {
		t.Fatalf("nested cell table count = %d, want 1", got)
	}
	if document.Tables[0].Rows[0].Cells[0].Tables[0].Text != "Nested A1" {
		t.Fatalf("nested table text = %q", document.Tables[0].Rows[0].Cells[0].Tables[0].Text)
	}
	if document.Tables[0].Rows[1].Cells[1].Text != "Cell B2" {
		t.Fatalf("last cell text = %q", document.Tables[0].Rows[1].Cells[1].Text)
	}
	if got := len(document.Sections); got != 1 {
		t.Fatalf("section count = %d, want 1", got)
	}
	if got := len(document.Sections[0].Headers); got != 1 {
		t.Fatalf("section headers = %d, want 1", got)
	}
	if document.Sections[0].PageSize == nil || document.Sections[0].PageSize.Width != 15840 || document.Sections[0].PageSize.Height != 12240 || document.Sections[0].PageSize.Orient != "landscape" {
		t.Fatalf("page size not resolved: %+v", document.Sections[0].PageSize)
	}
	if document.Sections[0].PageMargin == nil || document.Sections[0].PageMargin.Top != 1440 || document.Sections[0].PageMargin.Left != 720 {
		t.Fatalf("page margins not resolved: %+v", document.Sections[0].PageMargin)
	}
	if document.Sections[0].Columns == nil || document.Sections[0].Columns.Num != 2 || document.Sections[0].Columns.Space != 720 || !document.Sections[0].Columns.Sep {
		t.Fatalf("columns not resolved: %+v", document.Sections[0].Columns)
	}
	if document.Blocks[3].Section == nil || document.Blocks[3].Section.PageSize == nil || document.Blocks[3].Section.PageSize.Width != 15840 {
		t.Fatalf("section block not resolved: %+v", document.Blocks[3].Section)
	}
	if got := len(document.Headers["word/header1.xml"].Paragraphs); got != 1 {
		t.Fatalf("resolved header paragraphs = %d, want 1", got)
	}
	if got := len(document.Headers["word/header1.xml"].Tables); got != 1 {
		t.Fatalf("resolved header tables = %d, want 1", got)
	}
	if got := len(document.Headers["word/header1.xml"].Blocks); got != 2 {
		t.Fatalf("resolved header blocks = %d, want 2", got)
	}
	if document.Headers["word/header1.xml"].Blocks[0].Kind != "paragraph" || document.Headers["word/header1.xml"].Blocks[1].Kind != "table" {
		t.Fatalf("resolved header block order = %#v", []string{document.Headers["word/header1.xml"].Blocks[0].Kind, document.Headers["word/header1.xml"].Blocks[1].Kind})
	}
	if document.Headers["word/header1.xml"].Paragraphs[0].Text != "Header" {
		t.Fatalf("header text = %q", document.Headers["word/header1.xml"].Paragraphs[0].Text)
	}
	if document.Headers["word/header1.xml"].Tables[0].Text != "Header table" {
		t.Fatalf("header table text = %q", document.Headers["word/header1.xml"].Tables[0].Text)
	}
	if document.Footers["word/footer1.xml"].Paragraphs[0].Text != "Footer" {
		t.Fatalf("footer text = %q", document.Footers["word/footer1.xml"].Paragraphs[0].Text)
	}
}

func TestResolvePackageAppliesConditionalTableCellStyles(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:tbl>
      <w:tblPr>
        <w:tblStyle w:val="StyledTable"/>
        <w:tblLook w:firstRow="1" w:lastRow="0" w:firstColumn="0" w:lastColumn="0" w:noHBand="1" w:noVBand="1"/>
      </w:tblPr>
      <w:tblGrid><w:gridCol w:w="2400"/></w:tblGrid>
      <w:tr>
        <w:tc>
          <w:p><w:r><w:t>Header</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="table" w:styleId="StyledTable">
    <w:name w:val="StyledTable"/>
    <w:tblStylePr w:type="firstRow">
      <w:rPr><w:color w:val="FFFFFF"/></w:rPr>
      <w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="4F81BD"/></w:tcPr>
    </w:tblStylePr>
  </w:style>
</w:styles>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}
	doc, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(doc.Tables) != 1 || len(doc.Tables[0].Rows) != 1 || len(doc.Tables[0].Rows[0].Cells) != 1 {
		t.Fatalf("unexpected resolved table shape: %#v", doc.Tables)
	}
	cell := doc.Tables[0].Rows[0].Cells[0]
	if cell.Format.Shading != "4F81BD" {
		t.Fatalf("cell shading = %q, want 4F81BD", cell.Format.Shading)
	}
	if len(cell.Paragraphs) != 1 || len(cell.Paragraphs[0].Runs) != 1 {
		t.Fatalf("unexpected resolved cell paragraph shape: %#v", cell.Paragraphs)
	}
	if cell.Paragraphs[0].Runs[0].Format.Color != "FFFFFF" {
		t.Fatalf("header run color = %q, want FFFFFF", cell.Paragraphs[0].Runs[0].Format.Color)
	}
}

func TestResolvePackageUsesPackedTableLookFlagsWhenExplicitFlagsAreAbsent(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:tbl>
      <w:tblPr>
        <w:tblStyle w:val="StyledTable"/>
        <w:tblLook w:val="0660"/>
      </w:tblPr>
      <w:tblGrid>
        <w:gridCol w:w="2400"/>
        <w:gridCol w:w="2400"/>
      </w:tblGrid>
      <w:tr>
        <w:tc><w:p><w:r><w:t>College</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>Change</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:p><w:r><w:t>Cedar University</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>+7</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="table" w:styleId="StyledTable">
    <w:name w:val="StyledTable"/>
    <w:tblStylePr w:type="firstRow">
      <w:rPr><w:color w:val="FFFFFF"/></w:rPr>
      <w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="4BACC6"/></w:tcPr>
    </w:tblStylePr>
    <w:tblStylePr w:type="firstCol">
      <w:rPr><w:b/></w:rPr>
      <w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="FF0000"/></w:tcPr>
    </w:tblStylePr>
    <w:tblStylePr w:type="lastCol">
      <w:rPr><w:b/></w:rPr>
      <w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="00FF00"/></w:tcPr>
    </w:tblStylePr>
  </w:style>
</w:styles>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}
	doc, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	bodyFirst := doc.Tables[0].Rows[1].Cells[0]
	bodyLast := doc.Tables[0].Rows[1].Cells[1]
	if bodyFirst.Format.Shading != "" {
		t.Fatalf("body first-column shading = %q, want empty because 0660 disables firstCol", bodyFirst.Format.Shading)
	}
	if bodyLast.Format.Shading != "" {
		t.Fatalf("body last-column shading = %q, want empty because 0660 disables lastCol", bodyLast.Format.Shading)
	}
	if got := doc.Tables[0].Rows[0].Cells[0].Format.Shading; got != "4BACC6" {
		t.Fatalf("header shading = %q, want 4BACC6", got)
	}
}

func TestResolveTableOverlap(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:tbl>
      <w:tblPr>
        <w:tblOverlap w:val="never"/>
      </w:tblPr>
      <w:tr><w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc></w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
	})
	doc := resolveDoc(t, data)
	if len(doc.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(doc.Tables))
	}
	if doc.Tables[0].Format.Overlap != "never" {
		t.Errorf("Overlap = %q, want never", doc.Tables[0].Format.Overlap)
	}
}

func TestResolvePackagePreservesParagraphSectionBreaks(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t>First section</w:t></w:r>
      <w:pPr>
        <w:sectPr>
          <w:type w:val="nextPage"/>
          <w:pgSz w:w="12000" w:h="16000"/>
          <w:pgMar w:top="1000" w:right="1000" w:bottom="1000" w:left="1000"/>
        </w:sectPr>
      </w:pPr>
    </w:p>
    <w:p>
      <w:r><w:t>Second section</w:t></w:r>
    </w:p>
    <w:sectPr>
      <w:type w:val="continuous"/>
      <w:pgSz w:w="14000" w:h="18000"/>
      <w:pgMar w:top="1200" w:right="1200" w:bottom="1200" w:left="1200"/>
    </w:sectPr>
  </w:body>
</w:document>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}

	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if got := len(document.Blocks); got != 4 {
		t.Fatalf("block count = %d, want 4", got)
	}
	if document.Blocks[0].Kind != "paragraph" || document.Blocks[1].Kind != "section" || document.Blocks[2].Kind != "paragraph" || document.Blocks[3].Kind != "section" {
		t.Fatalf("block order = %#v", []string{document.Blocks[0].Kind, document.Blocks[1].Kind, document.Blocks[2].Kind, document.Blocks[3].Kind})
	}
	if got := len(document.Sections); got != 2 {
		t.Fatalf("section count = %d, want 2", got)
	}
	if document.Sections[0].Type != "nextPage" || document.Sections[0].PageSize == nil || document.Sections[0].PageSize.Width != 12000 {
		t.Fatalf("first section = %+v", document.Sections[0])
	}
	if document.Sections[1].Type != "continuous" || document.Sections[1].PageSize == nil || document.Sections[1].PageSize.Width != 14000 {
		t.Fatalf("second section = %+v", document.Sections[1])
	}
}

func TestResolvePackageIncludesComments(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/comments.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:r><w:t>Body text</w:t></w:r>
      <w:r><w:commentReference w:id="7"/></w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdComments" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/>
</Relationships>`,
		"word/comments.xml": `<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:comment w:id="7" w:author="Reviewer">
    <w:p><w:r><w:t>Comment paragraph</w:t></w:r></w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>Comment cell</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:comment>
</w:comments>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}

	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if got := len(document.Comments); got != 1 {
		t.Fatalf("comment story count = %d, want 1", got)
	}
	comment := document.Comments[7]
	if got := len(comment.Blocks); got != 2 {
		t.Fatalf("comment block count = %d, want 2", got)
	}
	if comment.Blocks[0].Kind != "paragraph" || comment.Blocks[1].Kind != "table" {
		t.Fatalf("comment block kinds = %#v", []string{comment.Blocks[0].Kind, comment.Blocks[1].Kind})
	}
	if comment.Text != "Comment paragraph\nComment cell" {
		t.Fatalf("comment text = %q", comment.Text)
	}
	runs := document.Paragraphs[0].Runs
	if got := len(runs); got != 2 {
		t.Fatalf("paragraph run count = %d, want 2", got)
	}
	if runs[1].Kind != "commentRef" || runs[1].CommentID != 7 {
		t.Fatalf("comment ref run = %+v", runs[1])
	}
}

func TestResolvePackageIncludesCoreProperties(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rIdCore" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
</Relationships>`,
		"word/document.xml":            `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"docProps/core.xml": `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <dc:title>Quarterly Report</dc:title>
  <dc:subject>Finance</dc:subject>
  <dc:creator>Alex</dc:creator>
  <cp:keywords>q1,report</cp:keywords>
  <cp:lastModifiedBy>Editor</cp:lastModifiedBy>
</cp:coreProperties>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if document.Title != "Quarterly Report" || document.Subject != "Finance" || document.Author != "Alex" {
		t.Fatalf("document metadata = title:%q subject:%q author:%q", document.Title, document.Subject, document.Author)
	}
	if document.Keywords != "q1,report" || document.LastModifiedBy != "Editor" {
		t.Fatalf("document metadata = keywords:%q lastModifiedBy:%q", document.Keywords, document.LastModifiedBy)
	}
}

func TestResolvePackageTagsAdditionalDocxFieldKinds(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:fldSimple w:instr='FILLIN "Name?" "Jane Smith"'>
        <w:r><w:t>Jane Smith</w:t></w:r>
      </w:fldSimple>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:fldSimple w:instr="CREATEDATE">
        <w:r><w:t>2006-04-26</w:t></w:r>
      </w:fldSimple>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:fldSimple w:instr="SAVEDATE">
        <w:r><w:t>2006-04-27</w:t></w:r>
      </w:fldSimple>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:fldSimple w:instr="PRINTDATE">
        <w:r><w:t>2006-04-28</w:t></w:r>
      </w:fldSimple>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 7 {
		t.Fatalf("run count = %d, want 7", len(runs))
	}
	if runs[0].FieldKind != "FILLIN" || runs[0].FieldArg != "Jane Smith" {
		t.Fatalf("fillin run = %+v", runs[0])
	}
	if runs[2].FieldKind != "CREATEDATE" || runs[2].Text != "2006-04-26" {
		t.Fatalf("createdate run = %+v", runs[2])
	}
	if runs[4].FieldKind != "SAVEDATE" || runs[4].Text != "2006-04-27" {
		t.Fatalf("savedate run = %+v", runs[4])
	}
	if runs[6].FieldKind != "PRINTDATE" || runs[6].Text != "2006-04-28" {
		t.Fatalf("printdate run = %+v", runs[6])
	}
}

func TestResolvePackageTagsDocxFieldFamiliesForCachedResults(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:fldSimple w:instr='SEQ Figure'>
        <w:r><w:t>1</w:t></w:r>
      </w:fldSimple>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:fldSimple w:instr='ASK CustomerName "Prompt"'>
        <w:r><w:t>Alice</w:t></w:r>
      </w:fldSimple>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:fldSimple w:instr='MERGEFIELD CustomerName \\* MERGEFORMAT'>
        <w:r><w:t>Alice</w:t></w:r>
      </w:fldSimple>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:fldSimple w:instr='AUTOTEXT "Disclaimer"'>
        <w:r><w:t>Disclaimer text</w:t></w:r>
      </w:fldSimple>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:fldSimple w:instr='LINK Excel.Sheet.12 "C:\\data.xlsx" "Sheet1!R1C1"'>
        <w:r><w:t>Linked sheet</w:t></w:r>
      </w:fldSimple>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 9 {
		t.Fatalf("run count = %d, want 9", len(runs))
	}
	if runs[0].FieldKind != "SEQ" || runs[0].FieldArg != "Figure" || runs[0].Text != "1" {
		t.Fatalf("seq run = %+v", runs[0])
	}
	if runs[2].FieldKind != "ASK" || runs[2].FieldArg != "CustomerName" || runs[2].Text != "Alice" {
		t.Fatalf("ask run = %+v", runs[2])
	}
	if runs[4].FieldKind != "MERGEFIELD" || runs[4].FieldArg != "CustomerName" || runs[4].Text != "Alice" {
		t.Fatalf("mergefield run = %+v", runs[4])
	}
	if runs[6].FieldKind != "AUTOTEXT" || runs[6].FieldArg != "Disclaimer" || runs[6].Text != "Disclaimer text" {
		t.Fatalf("autotext run = %+v", runs[6])
	}
	if runs[8].FieldKind != "LINK" || runs[8].FieldArg != `LINK Excel.Sheet.12 "C:\\data.xlsx" "Sheet1!R1C1"` || runs[8].Text != "Linked sheet" {
		t.Fatalf("link run = %+v", runs[8])
	}
}

func TestResolvePackageExpandsAutoTextFromGlossary(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:fldSimple w:instr='AUTOTEXT "Disclaimer"'/>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="Disclaimer"/>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>Clause body text</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].FieldKind != "AUTOTEXT" || runs[0].FieldArg != "Disclaimer" {
		t.Fatalf("autotext run = %+v", runs[0])
	}
	if runs[0].Text != "Clause body text" {
		t.Fatalf("autotext text = %q, want %q", runs[0].Text, "Clause body text")
	}
}

func TestResolvePackageExpandsRichAutoTextFromGlossary(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:fldSimple w:instr='AUTOTEXT "Disclaimer"'/>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="Disclaimer"/>
        <w:category><w:name w:val="AutoText"/><w:gallery w:val="autoText"/></w:category>
      </w:docPartPr>
      <w:docPartBody>
        <w:p>
          <w:r><w:rPr><w:b/></w:rPr><w:t>Bold</w:t></w:r>
          <w:r><w:rPr><w:i/></w:rPr><w:t>Italic</w:t></w:r>
        </w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 2 {
		t.Fatalf("run count = %d, want 2", len(runs))
	}
	if runs[0].Text != "Bold" || runs[0].Format.Bold == nil || !*runs[0].Format.Bold {
		t.Fatalf("run[0] = %+v, want bold 'Bold'", runs[0])
	}
	if runs[0].FieldKind != "AUTOTEXT" || runs[0].FieldArg != "Disclaimer" {
		t.Fatalf("run[0] field metadata = %q/%q, want AUTOTEXT/Disclaimer", runs[0].FieldKind, runs[0].FieldArg)
	}
	if runs[1].Text != "Italic" || runs[1].Format.Italic == nil || !*runs[1].Format.Italic {
		t.Fatalf("run[1] = %+v, want italic 'Italic'", runs[1])
	}
	if runs[1].FieldKind != "AUTOTEXT" || runs[1].FieldArg != "Disclaimer" {
		t.Fatalf("run[1] field metadata = %q/%q, want AUTOTEXT/Disclaimer", runs[1].FieldKind, runs[1].FieldArg)
	}
}

func TestResolvePackageExpandsMultiParagraphAutoTextFromGlossary(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:fldSimple w:instr='AUTOTEXT "Disclaimer"'/>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="Disclaimer"/>
        <w:category><w:name w:val="AutoText"/><w:gallery w:val="autoText"/></w:category>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>Line one</w:t></w:r></w:p>
        <w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Line two</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if len(document.Blocks) != 2 || len(document.Paragraphs) != 2 {
		t.Fatalf("blocks/paragraphs = %d/%d, want 2/2 structural AUTOTEXT blocks", len(document.Blocks), len(document.Paragraphs))
	}
	if document.Blocks[0].Kind != "paragraph" || document.Blocks[1].Kind != "paragraph" {
		t.Fatalf("block kinds = %q/%q, want paragraph/paragraph", document.Blocks[0].Kind, document.Blocks[1].Kind)
	}
	if document.Blocks[0].FieldKind != "AUTOTEXT" || document.Blocks[1].FieldKind != "AUTOTEXT" {
		t.Fatalf("expected AUTOTEXT block provenance: %+v", document.Blocks)
	}
	if document.Paragraphs[0].Text != "Line one" {
		t.Fatalf("paragraph[0] = %+v, want first glossary paragraph", document.Paragraphs[0])
	}
	if document.Paragraphs[1].Text != "Line two" || len(document.Paragraphs[1].Runs) != 1 ||
		document.Paragraphs[1].Runs[0].Format.Bold == nil || !*document.Paragraphs[1].Runs[0].Format.Bold {
		t.Fatalf("paragraph[1] = %+v, want bold second glossary paragraph", document.Paragraphs[1])
	}
	if document.Paragraphs[0].Index == document.Paragraphs[1].Index {
		t.Fatalf("expanded paragraphs reused index %d", document.Paragraphs[0].Index)
	}
}

func TestResolvePackageExpandsBlockAutoTextTableStructurally(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels":                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":            `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:fldSimple w:instr='AUTOTEXT "ClauseBlock"'/></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/></Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docParts><w:docPart><w:docPartPr><w:name w:val="ClauseBlock"/><w:category><w:gallery w:val="autoText"/></w:category></w:docPartPr><w:docPartBody>
<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:t>Heading</w:t></w:r></w:p>
<w:tbl><w:tblPr><w:tblW w:w="5000" w:type="dxa"/></w:tblPr><w:tr><w:tc><w:p><w:r><w:t>Cell A</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Cell B</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
<w:p><w:r><w:t>Tail</w:t></w:r></w:p>
</w:docPartBody></w:docPart></w:docParts></w:glossaryDocument>`,
	})
	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if len(document.Blocks) != 3 || document.Blocks[0].Kind != "paragraph" || document.Blocks[1].Kind != "table" || document.Blocks[2].Kind != "paragraph" {
		t.Fatalf("block structure = %+v, want paragraph/table/paragraph", document.Blocks)
	}
	if len(document.Tables) != 1 || document.Tables[0].Text != "Cell A | Cell B" {
		t.Fatalf("table = %+v, want resolved glossary table", document.Tables)
	}
	if document.Paragraphs[0].Format.Alignment != "center" {
		t.Fatalf("heading alignment = %q, want center", document.Paragraphs[0].Format.Alignment)
	}
	for i, block := range document.Blocks {
		if block.FieldKind != "AUTOTEXT" {
			t.Fatalf("block[%d] field kind = %q", i, block.FieldKind)
		}
		if block.FieldIndex != 0 {
			t.Fatalf("block[%d] field index = %d, want 0", i, block.FieldIndex)
		}
	}
	if len(document.Fields) != 1 || document.Fields[0].Kind != "AUTOTEXT" || len(document.Fields[0].Blocks) != 3 {
		t.Fatalf("story field provenance = %+v", document.Fields)
	}
	html, err := render.HTML(document)
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	if !strings.Contains(html, "<table") || !strings.Contains(html, "Cell A") || !strings.Contains(html, "Cell B") {
		t.Fatalf("expected structural AUTOTEXT table in HTML: %s", html)
	}
}

func TestResolvePackageKeepsInlineAutoTextInline(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml":          `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/></Types>`,
		"_rels/.rels":                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":            `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t xml:space="preserve">Before </w:t></w:r><w:fldSimple w:instr='AUTOTEXT "InlineEntry"'/><w:r><w:t xml:space="preserve"> after</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/></Relationships>`,
		"word/glossary/document.xml":   `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docParts><w:docPart><w:docPartPr><w:name w:val="InlineEntry"/><w:category><w:gallery w:val="autoText"/></w:category></w:docPartPr><w:docPartBody><w:p><w:r><w:t>One</w:t></w:r></w:p><w:p><w:r><w:t>Two</w:t></w:r></w:p></w:docPartBody></w:docPart></w:docParts></w:glossaryDocument>`,
	})
	opcPkg, _ := opc.OpenBytes(data)
	wordPkg, _ := parts.LoadPackage(opcPkg)
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(document.Blocks) != 1 || len(document.Paragraphs) != 1 {
		t.Fatalf("inline AUTOTEXT unexpectedly expanded blocks: blocks=%d paragraphs=%d", len(document.Blocks), len(document.Paragraphs))
	}
	if got := document.Paragraphs[0].Text; got != "Before One\nTwo after" {
		t.Fatalf("paragraph text = %q, want inline flattened AutoText", got)
	}
}

func TestResolvePackageAutoTextGalleryFiltering(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:fldSimple w:instr='AUTOTEXT "Disclaimer"'/>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="Disclaimer"/>
        <w:category><w:name w:val="AutoText"/><w:gallery w:val="autoText"/></w:category>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>AutoText version</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="Disclaimer"/>
        <w:category><w:name w:val="Cover Pages"/><w:gallery w:val="coverPg"/></w:category>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>Cover version</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].Text != "AutoText version" {
		t.Fatalf("autotext text = %q, want %q", runs[0].Text, "AutoText version")
	}
}

func TestResolvePackageExpandsComplexFieldAutoTextFromGlossary(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:fldChar w:fldCharType="begin"/>
      </w:r>
      <w:r>
        <w:instrText>AUTOTEXT &quot;Disclaimer&quot;</w:instrText>
      </w:r>
      <w:r>
        <w:fldChar w:fldCharType="separate"/>
      </w:r>
      <w:r>
        <w:fldChar w:fldCharType="end"/>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="Disclaimer"/>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>Clause body text</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].FieldKind != "AUTOTEXT" || runs[0].FieldArg != "Disclaimer" {
		t.Fatalf("autotext run = %+v", runs[0])
	}
	if runs[0].Text != "Clause body text" {
		t.Fatalf("autotext text = %q, want %q", runs[0].Text, "Clause body text")
	}
}

func TestResolvePackageExpandsDocPartSdtFromGlossary(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:sdt>
      <w:sdtPr>
        <w:id w:val="8"/>
        <w:alias w:val="Building Block"/>
        <w:tag w:val="docpart"/>
        <w:docPartObj>
          <w:docPartGallery w:val="Cover Pages"/>
          <w:docPartCategory w:val="General"/>
          <w:docPartUnique/>
        </w:docPartObj>
        <w:placeholder>
          <w:docPart w:val="DefaultCoverPage"/>
          <w:docPartGallery w:val="Cover Pages"/>
        </w:placeholder>
        <w:showingPlcHdr/>
      </w:sdtPr>
      <w:sdtContent>
        <w:p><w:r><w:t>Placeholder cover</w:t></w:r></w:p>
      </w:sdtContent>
    </w:sdt>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="DefaultCoverPage"/>
        <w:category><w:name w:val="General"/></w:category>
        <w:behavior w:val="content"/>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>Expanded cover body</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if len(document.Blocks) != 1 || document.Blocks[0].Paragraph == nil {
		t.Fatalf("document blocks = %#v, want one expanded paragraph block", document.Blocks)
	}
	block := document.Blocks[0]
	if block.ContentControl == nil || block.ContentControl.Type != "docPartObj" {
		t.Fatalf("block content control = %+v", block.ContentControl)
	}
	if block.ContentControl.PlaceholderDocPart != "DefaultCoverPage" || block.ContentControl.DocPartCategory != "General" {
		t.Fatalf("docPart metadata = %+v", block.ContentControl)
	}
	if block.Paragraph.Text != "Expanded cover body" {
		t.Fatalf("expanded paragraph text = %q, want %q", block.Paragraph.Text, "Expanded cover body")
	}
}

func TestResolvePackageExpandsInlineDocPartSdtFromGlossary(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t>Before </w:t></w:r>
      <w:sdt>
        <w:sdtPr>
          <w:docPartObj>
            <w:docPartGallery w:val="Quick Parts"/>
            <w:docPartCategory w:val="General"/>
          </w:docPartObj>
          <w:placeholder>
            <w:docPart w:val="InlinePart"/>
          </w:placeholder>
          <w:showingPlcHdr/>
        </w:sdtPr>
        <w:sdtContent>
          <w:r><w:t>Placeholder</w:t></w:r>
        </w:sdtContent>
      </w:sdt>
      <w:r><w:t> After</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="InlinePart"/>
        <w:category><w:name w:val="General"/></w:category>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>Expanded inline</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if len(document.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(document.Paragraphs))
	}
	runs := document.Paragraphs[0].Runs
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs (before, expanded, after), got %d: %+v", len(runs), runs)
	}
	if runs[0].Text != "Before " {
		t.Errorf("run[0] text = %q, want 'Before '", runs[0].Text)
	}
	if runs[1].Text != "Expanded inline" {
		t.Errorf("run[1] text = %q, want 'Expanded inline'", runs[1].Text)
	}
	if runs[1].ContentControl == nil || runs[1].ContentControl.Type != "docPartObj" {
		t.Errorf("run[1] content control = %+v, want docPartObj", runs[1].ContentControl)
	}
	if runs[2].Text != " After" {
		t.Errorf("run[2] text = %q, want ' After'", runs[2].Text)
	}
}

func TestResolvePackageExpandsInlineDocPartSdtFromMultiParagraphGlossary(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:t>Before </w:t></w:r>
      <w:sdt>
        <w:sdtPr>
          <w:docPartObj>
            <w:docPartGallery w:val="Quick Parts"/>
            <w:docPartCategory w:val="General"/>
          </w:docPartObj>
          <w:placeholder>
            <w:docPart w:val="InlinePart"/>
          </w:placeholder>
          <w:showingPlcHdr/>
        </w:sdtPr>
        <w:sdtContent>
          <w:r><w:t>Placeholder</w:t></w:r>
        </w:sdtContent>
      </w:sdt>
      <w:r><w:t> After</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr>
        <w:name w:val="InlinePart"/>
        <w:category><w:name w:val="General"/></w:category>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>Expanded inline</w:t></w:r></w:p>
        <w:p><w:r><w:rPr><w:i/></w:rPr><w:t>Second line</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 5 {
		t.Fatalf("expected 5 runs (before, expanded, break, expanded, after), got %d: %+v", len(runs), runs)
	}
	if runs[1].Text != "Expanded inline" {
		t.Fatalf("run[1] = %+v, want first glossary line", runs[1])
	}
	if runs[2].Kind != "break" || runs[2].ContentControl == nil || runs[2].ContentControl.Type != "docPartObj" {
		t.Fatalf("run[2] = %+v, want docPart break run", runs[2])
	}
	if runs[3].Text != "Second line" || runs[3].Format.Italic == nil || !*runs[3].Format.Italic {
		t.Fatalf("run[3] = %+v, want italic second glossary line", runs[3])
	}
}

func TestResolvePackageBindsPlainTextAndDateContentControls(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:sdt>
        <w:sdtPr>
          <w:text/>
          <w:dataBinding w:xpath="/root/customer" w:storeItemID="{store-1}"/>
        </w:sdtPr>
        <w:sdtContent><w:r><w:t>Placeholder</w:t></w:r></w:sdtContent>
      </w:sdt>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:sdt>
        <w:sdtPr>
          <w:date>
            <w:dateFormat w:val="MMMM d, yyyy"/>
          </w:date>
          <w:dataBinding w:xpath="/root/date" w:storeItemID="{store-1}"/>
        </w:sdtPr>
        <w:sdtContent><w:r><w:t>Old Date</w:t></w:r></w:sdtContent>
      </w:sdt>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"customXml/item1.xml":          `<root><customer>Bound Customer</customer><date>2024-02-20T00:00:00Z</date></root>`,
		"customXml/_rels/item1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXmlProps" Target="itemProps1.xml"/>
</Relationships>`,
		"customXml/itemProps1.xml": `<ds:datastoreItem xmlns:ds="http://schemas.openxmlformats.org/officeDocument/2006/customXml" ds:itemID="{store-1}"/>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 3 {
		t.Fatalf("run count = %d, want 3", len(runs))
	}
	if runs[0].ContentControl == nil || runs[0].ContentControl.DataValue != "Bound Customer" {
		t.Fatalf("plainText content control = %+v", runs[0].ContentControl)
	}
	if runs[2].ContentControl == nil || runs[2].ContentControl.DataValue != "2024-02-20T00:00:00Z" {
		t.Fatalf("date content control = %+v", runs[2].ContentControl)
	}
	if runs[2].ContentControl.DateFullDate != "2024-02-20T00:00:00Z" {
		t.Fatalf("bound date fullDate = %q", runs[2].ContentControl.DateFullDate)
	}
}

func TestResolvePackageBindsAttributeXPath(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:sdt>
        <w:sdtPr>
          <w:text/>
          <w:dataBinding w:xpath="/root/customer/@name" w:storeItemID="{store-1}"/>
        </w:sdtPr>
        <w:sdtContent><w:r><w:t>Placeholder</w:t></w:r></w:sdtContent>
      </w:sdt>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"customXml/item1.xml":          `<root><customer name="AliceAttr">AliceText</customer></root>`,
		"customXml/_rels/item1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXmlProps" Target="itemProps1.xml"/>
</Relationships>`,
		"customXml/itemProps1.xml": `<ds:datastoreItem xmlns:ds="http://schemas.openxmlformats.org/officeDocument/2006/customXml" ds:itemID="{store-1}"/>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].ContentControl == nil || runs[0].ContentControl.DataValue != "AliceAttr" {
		t.Fatalf("attribute binding = %+v, want DataValue='AliceAttr'", runs[0].ContentControl)
	}
}

func TestResolvePackagePreservesCustomXmlMetadata(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:customXml w:uri="http://example.com/block" w:element="outerBlock">
      <w:customXmlPr>
        <w:attr w:uri="urn:attrs" w:name="role" w:val="block"/>
      </w:customXmlPr>
      <w:p><w:r><w:t>Block custom xml</w:t></w:r></w:p>
    </w:customXml>
    <w:p>
      <w:customXml w:uri="http://example.com/inline" w:element="inlineNode">
        <w:customXmlPr>
          <w:attr w:uri="urn:attrs" w:name="role" w:val="inline"/>
        </w:customXmlPr>
        <w:r><w:t>Inline custom xml</w:t></w:r>
      </w:customXml>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc>
          <w:customXml w:uri="http://example.com/cell" w:element="cellNode">
            <w:customXmlPr>
              <w:attr w:uri="urn:attrs" w:name="role" w:val="cell"/>
            </w:customXmlPr>
            <w:p><w:r><w:t>Cell custom xml</w:t></w:r></w:p>
          </w:customXml>
        </w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if len(document.Blocks) != 3 {
		t.Fatalf("block count = %d, want 3", len(document.Blocks))
	}
	if document.Blocks[0].CustomXml == nil {
		t.Fatalf("block customXml metadata missing")
	}
	if got := document.Blocks[0].CustomXml.URI; got != "http://example.com/block" {
		t.Fatalf("block customXml URI = %q, want %q", got, "http://example.com/block")
	}
	if got := document.Blocks[0].CustomXml.Element; got != "outerBlock" {
		t.Fatalf("block customXml element = %q, want %q", got, "outerBlock")
	}
	if len(document.Blocks[0].CustomXml.Attributes) != 1 || document.Blocks[0].CustomXml.Attributes[0].Value != "block" {
		t.Fatalf("block customXml attrs = %+v", document.Blocks[0].CustomXml.Attributes)
	}

	if len(document.Paragraphs) != 2 {
		t.Fatalf("paragraph count = %d, want 2", len(document.Paragraphs))
	}
	inlineRuns := document.Paragraphs[1].Runs
	if len(inlineRuns) != 1 {
		t.Fatalf("inline paragraph run count = %d, want 1", len(inlineRuns))
	}
	if inlineRuns[0].CustomXml == nil {
		t.Fatalf("inline customXml metadata missing")
	}
	if got := inlineRuns[0].CustomXml.URI; got != "http://example.com/inline" {
		t.Fatalf("inline customXml URI = %q, want %q", got, "http://example.com/inline")
	}
	if got := inlineRuns[0].CustomXml.Element; got != "inlineNode" {
		t.Fatalf("inline customXml element = %q, want %q", got, "inlineNode")
	}
	if len(inlineRuns[0].CustomXml.Attributes) != 1 || inlineRuns[0].CustomXml.Attributes[0].Value != "inline" {
		t.Fatalf("inline customXml attrs = %+v", inlineRuns[0].CustomXml.Attributes)
	}

	if len(document.Tables) != 1 || len(document.Tables[0].Rows) != 1 || len(document.Tables[0].Rows[0].Cells) != 1 {
		t.Fatalf("unexpected table shape")
	}
	cellBlocks := document.Tables[0].Rows[0].Cells[0].Blocks
	if len(cellBlocks) != 1 {
		t.Fatalf("cell block count = %d, want 1", len(cellBlocks))
	}
	if cellBlocks[0].CustomXml == nil {
		t.Fatalf("cell customXml metadata missing")
	}
	if got := cellBlocks[0].CustomXml.URI; got != "http://example.com/cell" {
		t.Fatalf("cell customXml URI = %q, want %q", got, "http://example.com/cell")
	}
	if got := cellBlocks[0].CustomXml.Element; got != "cellNode" {
		t.Fatalf("cell customXml element = %q, want %q", got, "cellNode")
	}
	if len(cellBlocks[0].CustomXml.Attributes) != 1 || cellBlocks[0].CustomXml.Attributes[0].Value != "cell" {
		t.Fatalf("cell customXml attrs = %+v", cellBlocks[0].CustomXml.Attributes)
	}
}

func TestResolvePackageCapturesComplexFieldKindsAndTargets(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:fldChar w:fldCharType="begin"/></w:r>
      <w:r><w:instrText xml:space="preserve"> HYPERLINK "https://example.com" \l "sec1" </w:instrText></w:r>
      <w:r><w:fldChar w:fldCharType="separate"/></w:r>
      <w:r><w:t>click</w:t></w:r>
      <w:r><w:fldChar w:fldCharType="end"/></w:r>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:r><w:fldChar w:fldCharType="begin"/></w:r>
      <w:r><w:instrText xml:space="preserve"> REF Bookmark1 \h </w:instrText></w:r>
      <w:r><w:fldChar w:fldCharType="separate"/></w:r>
      <w:r><w:t>see bookmark</w:t></w:r>
      <w:r><w:fldChar w:fldCharType="end"/></w:r>
      <w:r><w:t xml:space="preserve"> </w:t></w:r>
      <w:r><w:fldChar w:fldCharType="begin"/></w:r>
      <w:r><w:instrText xml:space="preserve"> PAGEREF Bookmark2 \h </w:instrText></w:r>
      <w:r><w:fldChar w:fldCharType="separate"/></w:r>
      <w:r><w:t>12</w:t></w:r>
      <w:r><w:fldChar w:fldCharType="end"/></w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 5 {
		t.Fatalf("run count = %d, want 5", len(runs))
	}
	if runs[0].FieldKind != "HYPERLINK" || runs[0].FieldArg != "https://example.com" || runs[0].Target != "https://example.com#sec1" || runs[0].Anchor != "sec1" || runs[0].Kind != "hyperlink" {
		t.Fatalf("hyperlink field run = %+v", runs[0])
	}
	if runs[2].FieldKind != "REF" || runs[2].FieldArg != "Bookmark1" || runs[2].Target != "#Bookmark1" || runs[2].Anchor != "Bookmark1" {
		t.Fatalf("ref field run = %+v", runs[2])
	}
	if runs[4].FieldKind != "PAGEREF" || runs[4].FieldArg != "Bookmark2" || runs[4].Target != "#Bookmark2" || runs[4].Anchor != "Bookmark2" {
		t.Fatalf("pageref field run = %+v", runs[4])
	}
}

func TestResolvePackageCarriesForwardUnsupportedMarkupDiagnostics(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:unsupportedRunThing/>
        <w:t>ok</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if len(document.Diagnostics) == 0 {
		t.Fatal("expected diagnostics on resolved document")
	}
	if got, want := document.Diagnostics[0].Code, "word.unsupported_markup"; got != want {
		t.Fatalf("diagnostic code = %q, want %q", got, want)
	}
}

func TestResolvePackagePreservesDigitalSignatures(t *testing.T) {
	certDER := generateSignatureTestCert(t)
	certB64 := base64.StdEncoding.EncodeToString(certDER)

	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/_xmlsignatures/origin.sigs" ContentType="application/vnd.openxmlformats-package.digital-signature-origin"/>
  <Override PartName="/_xmlsignatures/sig1.xml" ContentType="application/vnd.openxmlformats-package.digital-signature-xmlsignature+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rIdSigOrigin" Type="http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/origin" Target="_xmlsignatures/origin.sigs"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Signed</w:t></w:r></w:p></w:body>
</w:document>`,
		"_xmlsignatures/origin.sigs": ``,
		"_xmlsignatures/_rels/origin.sigs.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdsig1" Type="http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/signature" Target="sig1.xml"/>
</Relationships>`,
		"_xmlsignatures/sig1.xml": `<Signature xmlns="http://www.w3.org/2000/09/xmldsig#" Id="sig1">
  <SignedInfo>
    <CanonicalizationMethod Algorithm="http://www.w3.org/TR/2001/REC-xml-c14n-20010315"/>
    <SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>
    <Reference URI="/word/document.xml" Type="http://schemas.openxmlformats.org/package/2006/RelationshipTransform">
      <DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
      <DigestValue>dGVzdA==</DigestValue>
    </Reference>
  </SignedInfo>
  <SignatureValue>YWJj</SignatureValue>
  <KeyInfo>
    <X509Data>
      <X509Certificate>` + certB64 + `</X509Certificate>
    </X509Data>
  </KeyInfo>
</Signature>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}

	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if !document.HasSignatures {
		t.Fatal("expected HasSignatures = true")
	}
	if len(document.DigitalSignatures) != 1 {
		t.Fatalf("digital signatures = %d, want 1", len(document.DigitalSignatures))
	}
	signature := document.DigitalSignatures[0]
	if signature.ID != "sig1" {
		t.Fatalf("signature ID = %q, want sig1", signature.ID)
	}
	if signature.SignatureMethod != "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256" {
		t.Fatalf("signature method = %q", signature.SignatureMethod)
	}
	if signature.CanonicalizationMethod != "http://www.w3.org/TR/2001/REC-xml-c14n-20010315" {
		t.Fatalf("canonicalization method = %q", signature.CanonicalizationMethod)
	}
	if len(signature.References) != 1 {
		t.Fatalf("signature references = %d, want 1", len(signature.References))
	}
	if signature.References[0].URI != "/word/document.xml" {
		t.Fatalf("reference URI = %q, want /word/document.xml", signature.References[0].URI)
	}
	if signature.References[0].DigestMethod != "http://www.w3.org/2001/04/xmlenc#sha256" {
		t.Fatalf("reference digest method = %q", signature.References[0].DigestMethod)
	}
	if signature.Signer == nil {
		t.Fatal("expected signer metadata")
	}
	if signature.Signer.Subject != "CN=test-signer" {
		t.Fatalf("signer subject = %q, want CN=test-signer", signature.Signer.Subject)
	}
	if len(signature.Certificates) != 1 {
		t.Fatalf("certificate count = %d, want 1", len(signature.Certificates))
	}
	if signature.Certificates[0].SerialNumber != "1" {
		t.Fatalf("certificate serial = %q, want 1", signature.Certificates[0].SerialNumber)
	}
	if signature.Valid {
		t.Fatalf("expected verification to fail for fixture with placeholder SignatureValue, got %+v", signature)
	}
	if signature.SignatureValid {
		t.Fatalf("expected SignatureValid = false for fixture with placeholder SignatureValue, got %+v", signature)
	}
	if signature.TrustValid {
		t.Fatalf("expected TrustValid = false for fixture with placeholder SignatureValue, got %+v", signature)
	}
	if signature.RevocationChecked {
		t.Fatalf("expected RevocationChecked = false for fixture without CRL, got %+v", signature)
	}
	if signature.TrustedTimestamp {
		t.Fatalf("expected TrustedTimestamp = false for fixture without SignatureTime, got %+v", signature)
	}
	if len(signature.VerificationErrors) == 0 {
		t.Fatalf("expected verification errors for invalid signature fixture, got %+v", signature)
	}
}

func generateSignatureTestCert(t *testing.T) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-signer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return der
}

func buildPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

// TestResolveCharacterStyleInheritance verifies that character styles and
// their basedOn chains are indexed and resolved correctly, and that the
// four-layer run-format inheritance (docDefaults → paragraph style rPr →
// character style chain → direct rPr) produces the right effective format.
func TestResolveCharacterStyleInheritance(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml"   ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="MyPara"/></w:pPr>
      <w:r><w:t>plain</w:t></w:r>
      <w:r>
        <w:rPr><w:rStyle w:val="Emphasis"/></w:rPr>
        <w:t>emphasis</w:t>
      </w:r>
      <w:r>
        <w:rPr><w:rStyle w:val="StrongEmphasis"/></w:rPr>
        <w:t>strong</w:t>
      </w:r>
      <w:r>
        <w:rPr>
          <w:rStyle w:val="Emphasis"/>
          <w:b/>
        </w:rPr>
        <w:t>bold-emphasis</w:t>
      </w:r>
      <w:r>
        <w:rPr><w:vertAlign w:val="superscript"/></w:rPr>
        <w:t>super</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:sz w:val="24"/>
      </w:rPr>
    </w:rPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:styleId="MyPara" w:default="1">
    <w:name w:val="MyPara"/>
    <w:rPr>
      <w:color w:val="444444"/>
    </w:rPr>
  </w:style>
  <w:style w:type="character" w:styleId="Emphasis">
    <w:name w:val="Emphasis"/>
    <w:rPr>
      <w:i/>
    </w:rPr>
  </w:style>
  <w:style w:type="character" w:styleId="StrongEmphasis">
    <w:name w:val="Strong Emphasis"/>
    <w:basedOn w:val="Emphasis"/>
    <w:rPr>
      <w:b/>
    </w:rPr>
  </w:style>
</w:styles>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	// --- Character style index ---
	emphasis, ok := document.CharacterStyles["Emphasis"]
	if !ok {
		t.Fatal("Emphasis character style not indexed")
	}
	if emphasis.Name != "Emphasis" {
		t.Fatalf("Emphasis name = %q", emphasis.Name)
	}
	if emphasis.BasedOn != "" {
		t.Fatalf("Emphasis basedOn = %q, want empty", emphasis.BasedOn)
	}
	if got := len(emphasis.StylePath); got != 1 || emphasis.StylePath[0] != "Emphasis" {
		t.Fatalf("Emphasis style path = %v", emphasis.StylePath)
	}
	// Emphasis.Format starts from docDefault (fontSize=24) then applies italic=true.
	if emphasis.Format.FontSize == nil || *emphasis.Format.FontSize != 24 {
		t.Fatalf("Emphasis format fontSize = %v, want 24", emphasis.Format.FontSize)
	}
	if !runBool(emphasis.Format.Italic) {
		t.Fatalf("Emphasis format italic = %v, want true", emphasis.Format.Italic)
	}
	if runBool(emphasis.Format.Bold) {
		t.Fatalf("Emphasis format bold = %v, want nil/false", emphasis.Format.Bold)
	}

	strongEm, ok := document.CharacterStyles["StrongEmphasis"]
	if !ok {
		t.Fatal("StrongEmphasis character style not indexed")
	}
	if strongEm.BasedOn != "Emphasis" {
		t.Fatalf("StrongEmphasis basedOn = %q", strongEm.BasedOn)
	}
	if got := len(strongEm.StylePath); got != 2 {
		t.Fatalf("StrongEmphasis style path len = %d, want 2", got)
	}
	// StrongEmphasis.Format inherits italic from Emphasis + adds bold.
	if !runBool(strongEm.Format.Bold) {
		t.Fatalf("StrongEmphasis format bold = %v, want true", strongEm.Format.Bold)
	}
	if !runBool(strongEm.Format.Italic) {
		t.Fatalf("StrongEmphasis format italic = %v, want true (inherited)", strongEm.Format.Italic)
	}

	// --- Default run format ---
	if document.DefaultRunFormat.FontSize == nil || *document.DefaultRunFormat.FontSize != 24 {
		t.Fatalf("default run format fontSize = %v, want 24", document.DefaultRunFormat.FontSize)
	}

	// --- Paragraph style run format ---
	myPara := document.Styles["MyPara"]
	if myPara.RunFormat.Color != "444444" {
		t.Fatalf("MyPara run format color = %q, want 444444", myPara.RunFormat.Color)
	}
	if myPara.RunFormat.FontSize == nil || *myPara.RunFormat.FontSize != 24 {
		t.Fatalf("MyPara run format fontSize = %v, want 24 (from docDefaults)", myPara.RunFormat.FontSize)
	}

	// --- Effective run format per run ---
	if got := len(document.Paragraphs); got != 1 {
		t.Fatalf("paragraph count = %d, want 1", got)
	}
	runs := document.Paragraphs[0].Runs
	if got := len(runs); got != 4 {
		t.Fatalf("run count = %d, want 4", got)
	}

	// run[0]: plain — inherits docDefault + paragraph style rPr only.
	plain := runs[0]
	if plain.Text != "plain" {
		t.Fatalf("run[0] text = %q", plain.Text)
	}
	if plain.Format.Color != "444444" {
		t.Fatalf("run[0] color = %q, want 444444", plain.Format.Color)
	}
	if plain.Format.FontSize == nil || *plain.Format.FontSize != 24 {
		t.Fatalf("run[0] fontSize = %v, want 24", plain.Format.FontSize)
	}
	if runBool(plain.Format.Bold) || runBool(plain.Format.Italic) {
		t.Fatalf("run[0] bold/italic should be unset, got bold=%v italic=%v", plain.Format.Bold, plain.Format.Italic)
	}

	// run[1]: rStyle=Emphasis → italic + inherited color/fontSize.
	em := runs[1]
	if em.Text != "emphasis" {
		t.Fatalf("run[1] text = %q", em.Text)
	}
	if !runBool(em.Format.Italic) {
		t.Fatalf("run[1] italic = %v, want true", em.Format.Italic)
	}
	if runBool(em.Format.Bold) {
		t.Fatalf("run[1] bold = %v, want nil/false", em.Format.Bold)
	}
	if em.Format.Color != "444444" {
		t.Fatalf("run[1] color = %q, want 444444", em.Format.Color)
	}
	if em.Format.FontSize == nil || *em.Format.FontSize != 24 {
		t.Fatalf("run[1] fontSize = %v, want 24", em.Format.FontSize)
	}

	// run[2]: StrongEmphasis (bold+italic) coalesced with Emphasis+direct-bold (bold+italic)
	// — both compute to identical formats so they merge into one run.
	strong := runs[2]
	if strong.Text != "strongbold-emphasis" {
		t.Fatalf("run[2] text = %q, want strongbold-emphasis", strong.Text)
	}
	if !runBool(strong.Format.Bold) {
		t.Fatalf("run[2] bold = %v, want true", strong.Format.Bold)
	}
	if !runBool(strong.Format.Italic) {
		t.Fatalf("run[2] italic = %v, want true (from Emphasis ancestor)", strong.Format.Italic)
	}
	if strong.Format.Color != "444444" {
		t.Fatalf("run[2] color = %q, want 444444", strong.Format.Color)
	}

	// run[3]: direct vertAlign=superscript.
	sup := runs[3]
	if sup.Text != "super" {
		t.Fatalf("run[3] text = %q", sup.Text)
	}
	if sup.Format.VertAlignVal != "superscript" {
		t.Fatalf("run[3] vertAlign = %q, want superscript", sup.Format.VertAlignVal)
	}
}

func TestResolvePackageCapturesEndnotePlacementFromSettings(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:footnotePr>
    <w:numFmt w:val="upperLetter"/>
    <w:numStart w:val="3"/>
    <w:numRestart w:val="eachPage"/>
  </w:footnotePr>
  <w:endnotePr>
      <w:pos w:val="sectEnd"/>
  	    <w:numFmt w:val="lowerRoman"/>
  	    <w:numStart w:val="5"/>
      <w:numRestart w:val="eachSect"/>
  </w:endnotePr>
</w:settings>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if document.EndnotePosition != "sectEnd" {
		t.Fatalf("endnote position = %q, want sectEnd", document.EndnotePosition)
	}
	if document.FootnoteNumberFormat != "upperLetter" {
		t.Fatalf("footnote number format = %q, want upperLetter", document.FootnoteNumberFormat)
	}
	if document.FootnoteNumberStart != 3 {
		t.Fatalf("footnote number start = %d, want 3", document.FootnoteNumberStart)
	}
	if document.FootnoteRestart != "eachPage" {
		t.Fatalf("footnote restart = %q, want eachPage", document.FootnoteRestart)
	}
	if document.EndnoteNumberFormat != "lowerRoman" {
		t.Fatalf("endnote number format = %q, want lowerRoman", document.EndnoteNumberFormat)
	}
	if document.EndnoteNumberStart != 5 {
		t.Fatalf("endnote number start = %d, want 5", document.EndnoteNumberStart)
	}
	if document.EndnoteRestart != "eachSect" {
		t.Fatalf("endnote restart = %q, want eachSect", document.EndnoteRestart)
	}
}

func TestResolvePackageCapturesZoomAndCompatFromSettings(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:zoom w:percent="120"/>
  <w:compat>
    <w:compatSetting w:name="doNotUseHTMLParagraphAutoSpacing" w:uri="http://schemas.microsoft.com/office/word" w:val="1"/>
    <w:compatSetting w:name="alignBordersAndEdges" w:uri="http://schemas.microsoft.com/office/word" w:val="1"/>
    <w:compatSetting w:name="noLeading" w:uri="http://schemas.microsoft.com/office/word" w:val="0"/>
  </w:compat>
</w:settings>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if document.ZoomPercent != 120 {
		t.Fatalf("ZoomPercent = %d, want 120", document.ZoomPercent)
	}
	if document.Compatibility == nil {
		t.Fatal("Compatibility = nil, want parsed")
	}
	if !document.Compatibility.DoNotUseHTMLParagraphAutoSpacing {
		t.Fatal("DoNotUseHTMLParagraphAutoSpacing = false, want true")
	}
	if !document.Compatibility.AlignBordersAndEdges {
		t.Fatal("AlignBordersAndEdges = false, want true")
	}
	if document.Compatibility.NoLeading {
		t.Fatal("NoLeading = true, want false")
	}
}

func TestResolvePackageCapturesEndnoteSeparatorStories(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/footnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"/>
  <Override PartName="/word/endnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.endnotes+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes" Target="endnotes.xml"/>
</Relationships>`,
		"word/footnotes.xml": `<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:footnote w:id="-1" w:type="separator">
    <w:p><w:r><w:t>Footnote separator</w:t></w:r></w:p>
  </w:footnote>
  <w:footnote w:id="0" w:type="continuationSeparator">
    <w:p><w:r><w:t>Footnote continuation separator</w:t></w:r></w:p>
  </w:footnote>
  <w:footnote w:id="1" w:type="continuationNotice">
    <w:p><w:r><w:t>Footnote continuation notice</w:t></w:r></w:p>
  </w:footnote>
  <w:footnote w:id="2">
    <w:p><w:r><w:t>Normal footnote</w:t></w:r></w:p>
  </w:footnote>
</w:footnotes>`,
		"word/endnotes.xml": `<w:endnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:endnote w:id="-1" w:type="separator">
    <w:p><w:r><w:t>Start separator</w:t></w:r></w:p>
  </w:endnote>
  <w:endnote w:id="0" w:type="continuationSeparator">
    <w:p><w:r><w:t>Continuation separator</w:t></w:r></w:p>
  </w:endnote>
  <w:endnote w:id="1">
    <w:p><w:r><w:t>Normal note</w:t></w:r></w:p>
  </w:endnote>
</w:endnotes>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if got := document.EndnoteSeparator.Blocks[0].Paragraph.Runs[0].Text; got != "Start separator" {
		t.Fatalf("endnote separator text = %q, want Start separator", got)
	}
	if got := document.EndnoteContinuationSep.Blocks[0].Paragraph.Runs[0].Text; got != "Continuation separator" {
		t.Fatalf("endnote continuation separator text = %q, want Continuation separator", got)
	}
	if got := document.FootnoteSeparator.Blocks[0].Paragraph.Runs[0].Text; got != "Footnote separator" {
		t.Fatalf("footnote separator text = %q, want Footnote separator", got)
	}
	if got := document.FootnoteContinuationSep.Blocks[0].Paragraph.Runs[0].Text; got != "Footnote continuation separator" {
		t.Fatalf("footnote continuation separator text = %q, want Footnote continuation separator", got)
	}
	if got := document.FootnoteContinuationNotice.Blocks[0].Paragraph.Runs[0].Text; got != "Footnote continuation notice" {
		t.Fatalf("footnote continuation notice text = %q, want Footnote continuation notice", got)
	}
	if got := document.Endnotes[1].Blocks[0].Paragraph.Runs[0].Text; got != "Normal note" {
		t.Fatalf("normal endnote text = %q, want Normal note", got)
	}
	if got := document.Footnotes[2].Blocks[0].Paragraph.Runs[0].Text; got != "Normal footnote" {
		t.Fatalf("normal footnote text = %q, want Normal footnote", got)
	}
}

func TestResolvePackageCapturesTrackRevisionsFromSettings(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:trackRevisions w:val="true"/>
</w:settings>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if !document.TrackChangesEnabled {
		t.Fatal("TrackChangesEnabled = false, want true")
	}
}

func TestResolvePackageCapturesExtendedPropertiesCounts(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rIdApp" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`,
		"word/document.xml":            `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"docProps/app.xml": `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Words>42</Words>
  <Pages>3</Pages>
  <Paragraphs>5</Paragraphs>
  <Characters>250</Characters>
</Properties>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if document.WordCount != 42 {
		t.Errorf("WordCount = %d, want 42", document.WordCount)
	}
	if document.PageCount != 3 {
		t.Errorf("PageCount = %d, want 3", document.PageCount)
	}
	if document.ParagraphCount != 5 {
		t.Errorf("ParagraphCount = %d, want 5", document.ParagraphCount)
	}
	if document.CharCount != 250 {
		t.Errorf("CharCount = %d, want 250", document.CharCount)
	}
}

func TestResolvePackageCapturesWriteProtectionFromSettings(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:writeProtection w:recommended="1" w:hashValue="abc123" w:saltValue="salt" w:spinCount="100000"/>
</w:settings>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if document.WriteProtection == nil {
		t.Fatal("WriteProtection = nil, want non-nil")
	}
	if !document.WriteProtection.Recommended {
		t.Error("WriteProtection.Recommended = false, want true")
	}
	if document.WriteProtection.HashValue != "abc123" {
		t.Errorf("WriteProtection.HashValue = %q, want abc123", document.WriteProtection.HashValue)
	}
	if document.WriteProtection.SaltValue != "salt" {
		t.Errorf("WriteProtection.SaltValue = %q, want salt", document.WriteProtection.SaltValue)
	}
	if document.WriteProtection.SpinCount != 100000 {
		t.Errorf("WriteProtection.SpinCount = %d, want 100000", document.WriteProtection.SpinCount)
	}
}

func TestResolvePackageCapturesRSIDsFromSettings(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:rsids>
    <w:rsidRoot w:val="00000000"/>
    <w:rsid w:val="00000001"/>
    <w:rsid w:val="00000002"/>
  </w:rsids>
</w:settings>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(document.RevisionSaveIDs) != 2 {
		t.Fatalf("RevisionSaveIDs = %v, want 2 entries", document.RevisionSaveIDs)
	}
	if document.RevisionSaveIDs[0] != "00000001" || document.RevisionSaveIDs[1] != "00000002" {
		t.Errorf("RevisionSaveIDs = %v, want [00000001 00000002]", document.RevisionSaveIDs)
	}
}

func TestResolvePackageCapturesAttachedTemplatePath(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/_rels/settings.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/attachedTemplate" Target="file:///C:/Users/Test/AppData/Roaming/Microsoft/Templates/Normal.dotm" TargetMode="External"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:attachedTemplate r:id="rId1"/>
</w:settings>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	want := "file:///C:/Users/Test/AppData/Roaming/Microsoft/Templates/Normal.dotm"
	if document.TemplatePath != want {
		t.Errorf("TemplatePath = %q, want %q", document.TemplatePath, want)
	}
}

func TestResolvePackageCapturesActiveWritingStyleAppName(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:activeWritingStyle w:lang="en-US" w:vendorID="64" w:appName="MSWord"/>
</w:settings>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(document.ActiveWritingStyles) != 1 {
		t.Fatalf("ActiveWritingStyles = %d, want 1", len(document.ActiveWritingStyles))
	}
	if document.ActiveWritingStyles[0].AppName != "MSWord" {
		t.Errorf("AppName = %q, want MSWord", document.ActiveWritingStyles[0].AppName)
	}
}

func TestResolvePackageCapturesSpecVanishRunProperty(t *testing.T) {
	body := `<w:r><w:rPr><w:specVanish w:val="1"/></w:rPr><w:t>hidden</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	runs := document.Blocks[0].Paragraph.Runs
	if len(runs) != 0 {
		t.Fatalf("expected specVanish run to be suppressed, got %d runs", len(runs))
	}
}

// ---------------------------------------------------------------------------
// P1: Revision semantics — author/date propagation through IR
// ---------------------------------------------------------------------------

func minimalDocXML(bodyXML string) string {
	return `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body><w:p>` + bodyXML + `</w:p></w:body></w:document>`
}

func minimalPackageFiles(docXML string) map[string]string {
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": docXML,
	}
}

func resolveMinimalDoc(t *testing.T, bodyXML string) *resolve.Document {
	t.Helper()
	data := buildPackage(t, minimalPackageFiles(minimalDocXML(bodyXML)))
	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	return document
}

func TestResolveInsRunCarriesAuthorAndDate(t *testing.T) {
	body := `<w:ins w:id="1" w:author="Alice" w:date="2024-01-15T10:00:00Z">` +
		`<w:r><w:t>inserted text</w:t></w:r>` +
		`</w:ins>`
	document := resolveMinimalDoc(t, body)
	runs := document.Blocks[0].Paragraph.Runs
	if len(runs) == 0 {
		t.Fatal("expected at least one run")
	}
	var insRun *resolve.Run
	for i := range runs {
		if runs[i].Revision == "inserted" {
			insRun = &runs[i]
			break
		}
	}
	if insRun == nil {
		t.Fatal("no inserted run found")
	}
	if got, want := insRun.Text, "inserted text"; got != want {
		t.Errorf("run.Text = %q, want %q", got, want)
	}
	if got, want := insRun.RevisionAuthor, "Alice"; got != want {
		t.Errorf("run.RevisionAuthor = %q, want %q", got, want)
	}
	if got, want := insRun.RevisionDate, "2024-01-15T10:00:00Z"; got != want {
		t.Errorf("run.RevisionDate = %q, want %q", got, want)
	}
}

func TestResolveDelRunCarriesAuthorAndDate(t *testing.T) {
	body := `<w:del w:id="2" w:author="Bob" w:date="2024-02-20T09:00:00Z">` +
		`<w:r><w:delText>deleted text</w:delText></w:r>` +
		`</w:del>`
	document := resolveMinimalDoc(t, body)
	runs := document.Blocks[0].Paragraph.Runs
	var delRun *resolve.Run
	for i := range runs {
		if runs[i].Revision == "deleted" {
			delRun = &runs[i]
			break
		}
	}
	if delRun == nil {
		t.Fatal("no deleted run found")
	}
	if got, want := delRun.RevisionAuthor, "Bob"; got != want {
		t.Errorf("run.RevisionAuthor = %q, want %q", got, want)
	}
	if got, want := delRun.RevisionDate, "2024-02-20T09:00:00Z"; got != want {
		t.Errorf("run.RevisionDate = %q, want %q", got, want)
	}
}

func TestResolveMixedInsDelInSameParagraph(t *testing.T) {
	body := `<w:r><w:t xml:space="preserve">base </w:t></w:r>` +
		`<w:ins w:id="3" w:author="Carol" w:date="2024-03-01T00:00:00Z">` +
		`<w:r><w:t>added</w:t></w:r>` +
		`</w:ins>` +
		`<w:del w:id="4" w:author="Dave" w:date="2024-03-02T00:00:00Z">` +
		`<w:r><w:delText>removed</w:delText></w:r>` +
		`</w:del>`
	document := resolveMinimalDoc(t, body)
	runs := document.Blocks[0].Paragraph.Runs

	var insRun, delRun *resolve.Run
	for i := range runs {
		switch runs[i].Revision {
		case "inserted":
			insRun = &runs[i]
		case "deleted":
			delRun = &runs[i]
		}
	}
	if insRun == nil || delRun == nil {
		t.Fatal("expected both inserted and deleted runs")
	}
	if got := insRun.RevisionAuthor; got != "Carol" {
		t.Errorf("ins author = %q, want Carol", got)
	}
	if got := delRun.RevisionAuthor; got != "Dave" {
		t.Errorf("del author = %q, want Dave", got)
	}
	// base run should have no revision
	baseFound := false
	for i := range runs {
		if runs[i].Text == "base " && runs[i].Revision == "" {
			baseFound = true
			break
		}
	}
	if !baseFound {
		t.Error("expected base run with no revision marker")
	}
}

func TestResolveParagraphBidiSetsBidiField(t *testing.T) {
	body := `<w:pPr><w:bidi/></w:pPr><w:r><w:t>שלום</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	if len(document.Blocks) == 0 || document.Blocks[0].Paragraph == nil {
		t.Fatal("expected a paragraph block")
	}
	if !document.Blocks[0].Paragraph.Format.Bidi {
		t.Error("expected Bidi=true for paragraph with w:bidi element")
	}
}

func TestResolveRunPropertyRevisionCarriesAuthorAndDate(t *testing.T) {
	body := `<w:r>` +
		`<w:rPr><w:rPrChange w:id="5" w:author="Eve" w:date="2024-03-03T00:00:00Z"><w:rPr/></w:rPrChange></w:rPr>` +
		`<w:t>formatted</w:t>` +
		`</w:r>`
	document := resolveMinimalDoc(t, body)
	runs := document.Blocks[0].Paragraph.Runs
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if runs[0].PropertyRevision == nil {
		t.Fatal("run.PropertyRevision = nil, want metadata")
	}
	if got := runs[0].PropertyRevision.Author; got != "Eve" {
		t.Fatalf("run.PropertyRevision.Author = %q, want Eve", got)
	}
	if got := runs[0].PropertyRevision.Date; got != "2024-03-03T00:00:00Z" {
		t.Fatalf("run.PropertyRevision.Date = %q, want 2024-03-03T00:00:00Z", got)
	}
}

func TestResolveRunNoProof(t *testing.T) {
	body := `<w:r><w:rPr><w:noProof/></w:rPr><w:t>proofing off</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	runs := document.Blocks[0].Paragraph.Runs
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if runs[0].Format.NoProof == nil || !*runs[0].Format.NoProof {
		t.Fatalf("run.Format.NoProof = %v, want true", runs[0].Format.NoProof)
	}
}

func TestResolveParagraphPropertyRevisionCarriesAuthorAndDate(t *testing.T) {
	body := `<w:pPr><w:pPrChange w:id="6" w:author="Frank" w:date="2024-03-04T00:00:00Z"><w:pPr/></w:pPrChange></w:pPr>` +
		`<w:r><w:t>paragraph</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	para := document.Blocks[0].Paragraph
	if para.PropertyRevision == nil {
		t.Fatal("paragraph.PropertyRevision = nil, want metadata")
	}
	if got := para.PropertyRevision.Author; got != "Frank" {
		t.Fatalf("paragraph.PropertyRevision.Author = %q, want Frank", got)
	}
	if got := para.PropertyRevision.Date; got != "2024-03-04T00:00:00Z" {
		t.Fatalf("paragraph.PropertyRevision.Date = %q, want 2024-03-04T00:00:00Z", got)
	}
}

func TestResolveSectionPropertyRevisionCarriesAuthorAndDate(t *testing.T) {
	body := `<w:pPr><w:sectPr><w:sectPrChange w:id="7" w:author="Grace" w:date="2024-03-05T00:00:00Z"><w:sectPr/></w:sectPrChange></w:sectPr></w:pPr>` +
		`<w:r><w:t>section break</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	if len(document.Sections) == 0 {
		t.Fatal("document.Sections = 0, want at least 1 section")
	}
	if document.Sections[0].PropertyRevision == nil {
		t.Fatal("section.PropertyRevision = nil, want metadata")
	}
	if got := document.Sections[0].PropertyRevision.Author; got != "Grace" {
		t.Fatalf("section.PropertyRevision.Author = %q, want Grace", got)
	}
	if got := document.Sections[0].PropertyRevision.Date; got != "2024-03-05T00:00:00Z" {
		t.Fatalf("section.PropertyRevision.Date = %q, want 2024-03-05T00:00:00Z", got)
	}
}

func TestResolveSectionPageNumberFormatAndStart(t *testing.T) {
	body := `<w:pPr><w:sectPr><w:pgNumType w:fmt="upperRoman" w:start="7"/></w:sectPr></w:pPr>` +
		`<w:r><w:t>section break</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	if len(document.Sections) == 0 {
		t.Fatal("document.Sections = 0, want at least 1 section")
	}
	if got := document.Sections[0].PageNumberFormat; got != "upperRoman" {
		t.Fatalf("section.PageNumberFormat = %q, want upperRoman", got)
	}
	if document.Sections[0].PageNumberStart == nil || *document.Sections[0].PageNumberStart != 7 {
		t.Fatalf("section.PageNumberStart = %v, want 7", document.Sections[0].PageNumberStart)
	}
}

func TestResolveTableRowAndCellPropertyRevisionCarriesAuthorAndDate(t *testing.T) {
	body := `</w:p><w:tbl>` +
		`<w:tblPr><w:tblPrChange w:id="8" w:author="Hank" w:date="2024-03-06T00:00:00Z"><w:tblPr/></w:tblPrChange></w:tblPr>` +
		`<w:tr>` +
		`<w:trPr><w:trPrChange w:id="9" w:author="Ivy" w:date="2024-03-07T00:00:00Z"><w:trPr/></w:trPrChange></w:trPr>` +
		`<w:tc><w:tcPr><w:tcPrChange w:id="10" w:author="Jade" w:date="2024-03-08T00:00:00Z"><w:tcPr/></w:tcPrChange></w:tcPr><w:p><w:r><w:t>cell</w:t></w:r></w:p></w:tc>` +
		`</w:tr>` +
		`</w:tbl><w:p>`
	document := resolveMinimalDoc(t, body)
	if len(document.Tables) != 1 {
		t.Fatalf("document.Tables = %d, want 1", len(document.Tables))
	}
	table := document.Tables[0]
	if table.PropertyRevision == nil || table.PropertyRevision.Author != "Hank" || table.PropertyRevision.Date != "2024-03-06T00:00:00Z" {
		t.Fatalf("table.PropertyRevision = %#v, want Hank/2024-03-06T00:00:00Z", table.PropertyRevision)
	}
	if len(table.Rows) != 1 {
		t.Fatalf("table rows = %d, want 1", len(table.Rows))
	}
	row := table.Rows[0]
	if row.PropertyRevision == nil || row.PropertyRevision.Author != "Ivy" || row.PropertyRevision.Date != "2024-03-07T00:00:00Z" {
		t.Fatalf("row.PropertyRevision = %#v, want Ivy/2024-03-07T00:00:00Z", row.PropertyRevision)
	}
	if len(row.Cells) != 1 {
		t.Fatalf("row cells = %d, want 1", len(row.Cells))
	}
	cell := row.Cells[0]
	if cell.PropertyRevision == nil || cell.PropertyRevision.Author != "Jade" || cell.PropertyRevision.Date != "2024-03-08T00:00:00Z" {
		t.Fatalf("cell.PropertyRevision = %#v, want Jade/2024-03-08T00:00:00Z", cell.PropertyRevision)
	}
}

func TestResolveParagraphWithoutBidiHasBidiFalse(t *testing.T) {
	body := `<w:r><w:t>Hello</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	if len(document.Blocks) == 0 || document.Blocks[0].Paragraph == nil {
		t.Fatal("expected a paragraph block")
	}
	if document.Blocks[0].Paragraph.Format.Bidi {
		t.Error("expected Bidi=false for paragraph without w:bidi element")
	}
}

func TestResolveRunRTLSetsRTLField(t *testing.T) {
	body := `<w:r><w:rPr><w:rtl/></w:rPr><w:t>مرحبا</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	if len(document.Blocks) == 0 || document.Blocks[0].Paragraph == nil {
		t.Fatal("expected a paragraph block")
	}
	runs := document.Blocks[0].Paragraph.Runs
	if len(runs) == 0 {
		t.Fatal("expected at least one run")
	}
	if runs[0].Format.RTL == nil || !*runs[0].Format.RTL {
		t.Error("expected RTL=true for run with w:rtl element")
	}
}

func TestResolveRunSoftEdgeAndReflection(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
			xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"
			mc:Ignorable="w14"
			xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006">
  <w:body>
    <w:p>
      <w:r>
        <w:rPr>
          <w14:softEdge w14:rad="38100"/>
          <w14:reflection w14:blurRad="25400" w14:dist="12700" w14:dir="5400000" w14:stA="50000"/>
        </w:rPr>
        <w:t>Effects</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
	})
	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	runs := document.Blocks[0].Paragraph.Runs
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	f := runs[0].Format
	if f.TextSoftEdge == nil {
		t.Fatal("expected TextSoftEdge")
	}
	if got := f.TextSoftEdge.Radius; got != 3.0 {
		t.Errorf("TextSoftEdge.Radius = %v, want 3.0", got)
	}
	if f.TextReflection == nil {
		t.Fatal("expected TextReflection")
	}
	if got := f.TextReflection.Alpha; got != 0.5 {
		t.Errorf("TextReflection.Alpha = %v, want 0.5", got)
	}
	if got := f.TextReflection.Blur; got != 2.0 {
		t.Errorf("TextReflection.Blur = %v, want 2.0", got)
	}
	if got := f.TextReflection.Distance; got != 1.0 {
		t.Errorf("TextReflection.Distance = %v, want 1.0", got)
	}
	if got := f.TextReflection.Direction; got != 90.0 {
		t.Errorf("TextReflection.Direction = %v, want 90.0", got)
	}
}

func TestResolveParagraphDefaultRunFormatFromParagraphMarkRunProperties(t *testing.T) {
	body := `<w:pPr>
		<w:rPr>
			<w:b/>
			<w:color w:val="FF0000"/>
			<w:sz w:val="48"/>
		</w:rPr>
	</w:pPr>
	<w:r><w:t>Hello</w:t></w:r>`
	document := resolveMinimalDoc(t, body)
	if len(document.Blocks) == 0 || document.Blocks[0].Paragraph == nil {
		t.Fatal("expected a paragraph block")
	}
	p := document.Blocks[0].Paragraph
	if p.DefaultRunFormat == nil {
		t.Fatal("expected DefaultRunFormat to be set from w:pPr/w:rPr")
	}
	if p.DefaultRunFormat.Bold == nil || !*p.DefaultRunFormat.Bold {
		t.Error("expected DefaultRunFormat.Bold = true")
	}
	if p.DefaultRunFormat.Color != "FF0000" {
		t.Errorf("expected DefaultRunFormat.Color = FF0000, got %q", p.DefaultRunFormat.Color)
	}
	if p.DefaultRunFormat.FontSize == nil || *p.DefaultRunFormat.FontSize != 48 {
		t.Errorf("expected DefaultRunFormat.FontSize = 48, got %v", p.DefaultRunFormat.FontSize)
	}
	// The run itself should also inherit the paragraph-mark formatting.
	if len(p.Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	if p.Runs[0].Format.Bold == nil || !*p.Runs[0].Format.Bold {
		t.Error("expected run to inherit bold from paragraph mark run properties")
	}
}

func TestResolveParagraphDefaultRunFormatInheritsStyle(t *testing.T) {
	body := `<w:pPr><w:pStyle w:val="MyStyle"/></w:pPr>
	<w:r><w:t>Styled</w:t></w:r>`
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p>` + body + `</w:p></w:body>
</w:document>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="MyStyle">
    <w:name w:val="My Style"/>
    <w:rPr>
      <w:i/>
      <w:sz w:val="28"/>
    </w:rPr>
  </w:style>
</w:styles>`,
	})
	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(document.Blocks) == 0 || document.Blocks[0].Paragraph == nil {
		t.Fatal("expected a paragraph block")
	}
	p := document.Blocks[0].Paragraph
	if p.DefaultRunFormat == nil {
		t.Fatal("expected DefaultRunFormat to inherit from style")
	}
	if p.DefaultRunFormat.Italic == nil || !*p.DefaultRunFormat.Italic {
		t.Error("expected DefaultRunFormat.Italic = true from style")
	}
	if p.DefaultRunFormat.FontSize == nil || *p.DefaultRunFormat.FontSize != 28 {
		t.Errorf("expected DefaultRunFormat.FontSize = 28 from style, got %v", p.DefaultRunFormat.FontSize)
	}
}

func TestResolveExtendedAndCustomProperties(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/custom-properties" Target="docProps/custom.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>hello</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"docProps/app.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>Microsoft Word</Application>
  <AppVersion>16.0000</AppVersion>
  <Company>Acme Corp</Company>
  <Manager>Bob</Manager>
  <Template>Normal</Template>
</Properties>`,
		"docProps/custom.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="2" name="ProjectID">
    <vt:lpwstr>42</vt:lpwstr>
  </property>
  <property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="3" name="Status">
    <vt:lpwstr>Draft</vt:lpwstr>
  </property>
</Properties>`,
	})

	doc := resolveDoc(t, data)
	if doc.Application != "Microsoft Word" {
		t.Errorf("Application = %q, want Microsoft Word", doc.Application)
	}
	if doc.AppVersion != "16.0000" {
		t.Errorf("AppVersion = %q, want 16.0000", doc.AppVersion)
	}
	if doc.Company != "Acme Corp" {
		t.Errorf("Company = %q, want Acme Corp", doc.Company)
	}
	if doc.Manager != "Bob" {
		t.Errorf("Manager = %q, want Bob", doc.Manager)
	}
	if doc.Template != "Normal" {
		t.Errorf("Template = %q, want Normal", doc.Template)
	}
	if len(doc.CustomProperties) != 2 {
		t.Fatalf("CustomProperties = %d, want 2", len(doc.CustomProperties))
	}
	if doc.CustomProperties["ProjectID"] != "42" {
		t.Errorf("CustomProperties[ProjectID] = %q, want 42", doc.CustomProperties["ProjectID"])
	}
	if doc.CustomProperties["Status"] != "Draft" {
		t.Errorf("CustomProperties[Status] = %q, want Draft", doc.CustomProperties["Status"])
	}
}

func TestResolveDocumentVariablesFromSettings(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docVars>
    <w:docVar w:name="Version" w:val="1.0"/>
    <w:docVar w:name="Environment" w:val="staging"/>
  </w:docVars>
</w:settings>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.DocumentVariables) != 2 {
		t.Fatalf("DocumentVariables = %d, want 2", len(doc.DocumentVariables))
	}
	if doc.DocumentVariables["Version"] != "1.0" {
		t.Fatalf("DocumentVariables[Version] = %q, want 1.0", doc.DocumentVariables["Version"])
	}
	if doc.DocumentVariables["Environment"] != "staging" {
		t.Fatalf("DocumentVariables[Environment] = %q, want staging", doc.DocumentVariables["Environment"])
	}
}

func TestResolveCustomXMLParts(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"customXml/item1.xml":          `<root><customer>Bound Customer</customer></root>`,
		"customXml/_rels/item1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXmlProps" Target="itemProps1.xml"/>
</Relationships>`,
		"customXml/itemProps1.xml": `<ds:datastoreItem xmlns:ds="http://schemas.openxmlformats.org/officeDocument/2006/customXml" ds:itemID="{store-1}"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.CustomXMLParts) != 1 {
		t.Fatalf("CustomXMLParts = %d, want 1", len(doc.CustomXMLParts))
	}
	if got := string(doc.CustomXMLParts["{STORE-1}"]); got != `<root><customer>Bound Customer</customer></root>` {
		t.Fatalf("CustomXMLParts[{STORE-1}] = %q, want payload", got)
	}
}

func TestResolvePackageCapturesDocumentProtectionFromSettings(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:documentProtection w:edit="forms" w:enforcement="1" w:cryptProviderType="rsaAES" w:cryptAlgorithmClass="hash"/>
</w:settings>`,
	})

	doc := resolveDoc(t, data)
	if doc.DocumentProtection == nil {
		t.Fatal("expected DocumentProtection to be set")
	}
	if doc.DocumentProtection.Edit != "forms" {
		t.Errorf("Edit = %q, want forms", doc.DocumentProtection.Edit)
	}
	if !doc.DocumentProtection.Enforcement {
		t.Error("Enforcement = false, want true")
	}
	if !doc.ProtectionEnabled {
		t.Error("ProtectionEnabled = false, want true")
	}
}

func TestResolvePackageCapturesMailMergeFromSettings(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:mailMerge w:mainDocumentType="formLetters" w:linkToQuery="1" w:dataType="database" w:dataSource="source.odc" w:query="SELECT * FROM Table" w:headerSource="header.odt" w:destination="newDocument">
    <w:activeRecord w:val="7"/>
    <w:addressFieldName w:val="HomeAddress"/>
    <w:checkErrors w:val="3"/>
    <w:doNotSuppressBlankLines w:val="1"/>
    <w:dynamicAddress w:val="1"/>
    <w:entryMacro w:val="AutoOpenMerge"/>
    <w:exitMacro w:val="AutoCloseMerge"/>
    <w:fHdr w:val="1"/>
    <w:linkedToFile w:val="1"/>
    <w:mailAsAttachment w:val="1"/>
    <w:mailSubject w:val="Quarterly Update"/>
    <w:mappedName w:val="RecipientName"/>
    <w:types>
      <w:type w:val="personalizedLetters"/>
      <w:type w:val="email"/>
    </w:types>
    <w:recipients>
      <w:recipientData><w:active w:val="1"/></w:recipientData>
      <w:recipientData><w:active w:val="0"/></w:recipientData>
    </w:recipients>
    <w:odso>
      <w:colDelim w:val="124"/>
      <w:dynamicAddress w:val="1"/>
      <w:fHdr w:val="1"/>
      <w:sourceFileName w:val="odso.csv"/>
      <w:udl w:val="Provider=ODBC"/>
      <w:fieldMapData w:mappedName="FirstName" w:name="first_name" w:lang="en-US" w:val="firstName" w:col="1"/>
      <w:fieldMapData w:mappedName="LastName" w:name="last_name" w:lang="en-US" w:val="lastName" w:col="2"/>
      <w:recipientData><w:active w:val="1"/></w:recipientData>
      <w:recipientData><w:active w:val="0"/></w:recipientData>
    </w:odso>
    <w:sourceFileName w:val="records.csv"/>
    <w:storeMappedDataAs w:val="text"/>
    <w:subDoc w:val="PartA"/>
    <w:udl w:val="Provider=SQLOLEDB"/>
    <w:viewMergedData w:val="1"/>
    <w:personal w:val="1"/>
    <w:personalCompose w:val="1"/>
    <w:personalReply w:val="1"/>
  </w:mailMerge>
</w:settings>`,
	})

	doc := resolveDoc(t, data)
	if doc.MailMerge == nil {
		t.Fatal("expected MailMerge to be set")
	}
	if doc.MailMerge.MainDocumentType != "formLetters" {
		t.Errorf("MainDocumentType = %q, want formLetters", doc.MailMerge.MainDocumentType)
	}
	if !doc.MailMerge.LinkToQuery {
		t.Error("LinkToQuery = false, want true")
	}
	if doc.MailMerge.DataType != "database" {
		t.Errorf("DataType = %q, want database", doc.MailMerge.DataType)
	}
	if doc.MailMerge.DataSource != "source.odc" {
		t.Errorf("DataSource = %q, want source.odc", doc.MailMerge.DataSource)
	}
	if doc.MailMerge.Query != "SELECT * FROM Table" {
		t.Errorf("Query = %q, want SELECT * FROM Table", doc.MailMerge.Query)
	}
	if doc.MailMerge.HeaderSource != "header.odt" {
		t.Errorf("HeaderSource = %q, want header.odt", doc.MailMerge.HeaderSource)
	}
	if doc.MailMerge.Destination != "newDocument" {
		t.Errorf("Destination = %q, want newDocument", doc.MailMerge.Destination)
	}
	if doc.MailMerge.ActiveRecord != "7" {
		t.Errorf("ActiveRecord = %q, want 7", doc.MailMerge.ActiveRecord)
	}
	if doc.MailMerge.AddressFieldName != "HomeAddress" {
		t.Errorf("AddressFieldName = %q, want HomeAddress", doc.MailMerge.AddressFieldName)
	}
	if doc.MailMerge.CheckErrors != "3" {
		t.Errorf("CheckErrors = %q, want 3", doc.MailMerge.CheckErrors)
	}
	if !doc.MailMerge.DoNotSuppressBlankLines {
		t.Error("DoNotSuppressBlankLines = false, want true")
	}
	if !doc.MailMerge.DynamicAddress {
		t.Error("DynamicAddress = false, want true")
	}
	if doc.MailMerge.EntryMacro != "AutoOpenMerge" {
		t.Errorf("EntryMacro = %q, want AutoOpenMerge", doc.MailMerge.EntryMacro)
	}
	if doc.MailMerge.ExitMacro != "AutoCloseMerge" {
		t.Errorf("ExitMacro = %q, want AutoCloseMerge", doc.MailMerge.ExitMacro)
	}
	if !doc.MailMerge.DataSourceHasHeaders {
		t.Error("DataSourceHasHeaders = false, want true")
	}
	if !doc.MailMerge.LinkedToFile {
		t.Error("LinkedToFile = false, want true")
	}
	if !doc.MailMerge.MailAsAttachment {
		t.Error("MailAsAttachment = false, want true")
	}
	if doc.MailMerge.MailSubject != "Quarterly Update" {
		t.Errorf("MailSubject = %q, want Quarterly Update", doc.MailMerge.MailSubject)
	}
	if doc.MailMerge.MappedName != "RecipientName" {
		t.Errorf("MappedName = %q, want RecipientName", doc.MailMerge.MappedName)
	}
	if doc.MailMerge.SourceFileName != "records.csv" {
		t.Errorf("SourceFileName = %q, want records.csv", doc.MailMerge.SourceFileName)
	}
	if doc.MailMerge.StoreMappedDataAs != "text" {
		t.Errorf("StoreMappedDataAs = %q, want text", doc.MailMerge.StoreMappedDataAs)
	}
	if doc.MailMerge.SubDoc != "PartA" {
		t.Errorf("SubDoc = %q, want PartA", doc.MailMerge.SubDoc)
	}
	if doc.MailMerge.UDL != "Provider=SQLOLEDB" {
		t.Errorf("UDL = %q, want Provider=SQLOLEDB", doc.MailMerge.UDL)
	}
	if !doc.MailMerge.ViewMergedData {
		t.Error("ViewMergedData = false, want true")
	}
	if !doc.MailMerge.Personal {
		t.Error("Personal = false, want true")
	}
	if !doc.MailMerge.PersonalCompose {
		t.Error("PersonalCompose = false, want true")
	}
	if !doc.MailMerge.PersonalReply {
		t.Error("PersonalReply = false, want true")
	}
	if len(doc.MailMerge.Types) != 2 || doc.MailMerge.Types[0] != "personalizedLetters" || doc.MailMerge.Types[1] != "email" {
		t.Errorf("Types = %#v, want personalizedLetters/email", doc.MailMerge.Types)
	}
	if len(doc.MailMerge.Recipients) != 2 || !doc.MailMerge.Recipients[0].Active || doc.MailMerge.Recipients[1].Active {
		t.Errorf("Recipients = %+v", doc.MailMerge.Recipients)
	}
	if doc.MailMerge.ODSO == nil {
		t.Fatal("expected MailMerge.ODSO to be set")
	}
	if doc.MailMerge.ODSO.ColDelim != 124 || !doc.MailMerge.ODSO.DynamicAddress || !doc.MailMerge.ODSO.FHdr {
		t.Errorf("ODSO core fields = %+v", doc.MailMerge.ODSO)
	}
	if doc.MailMerge.ODSO.SourceFileName != "odso.csv" || doc.MailMerge.ODSO.UDL != "Provider=ODBC" {
		t.Errorf("ODSO source/udl = %+v", doc.MailMerge.ODSO)
	}
	if len(doc.MailMerge.ODSO.FieldMapData) != 2 {
		t.Fatalf("ODSO.FieldMapData count = %d, want 2", len(doc.MailMerge.ODSO.FieldMapData))
	}
	if doc.MailMerge.ODSO.FieldMapData[0].MappedName != "FirstName" || doc.MailMerge.ODSO.FieldMapData[0].Column != 1 {
		t.Errorf("ODSO.FieldMapData[0] = %+v", doc.MailMerge.ODSO.FieldMapData[0])
	}
	if doc.MailMerge.ODSO.FieldMapData[1].MappedName != "LastName" || doc.MailMerge.ODSO.FieldMapData[1].Column != 2 {
		t.Errorf("ODSO.FieldMapData[1] = %+v", doc.MailMerge.ODSO.FieldMapData[1])
	}
	if len(doc.MailMerge.ODSO.RecipientData) != 2 || !doc.MailMerge.ODSO.RecipientData[0].Active || doc.MailMerge.ODSO.RecipientData[1].Active {
		t.Errorf("ODSO.RecipientData = %+v", doc.MailMerge.ODSO.RecipientData)
	}
}

func TestResolvePackageAppliesCornerCellConditionalStyle(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:tbl>
      <w:tblPr>
        <w:tblStyle w:val="StyledTable"/>
        <w:tblLook w:firstRow="1" w:lastRow="1" w:firstColumn="1" w:lastColumn="1" w:noHBand="1" w:noVBand="1"/>
      </w:tblPr>
      <w:tblGrid>
        <w:gridCol w:w="2400"/>
        <w:gridCol w:w="2400"/>
      </w:tblGrid>
      <w:tr>
        <w:tc><w:p><w:r><w:t>NW</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>NE</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:p><w:r><w:t>SW</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>SE</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="table" w:styleId="StyledTable">
    <w:name w:val="StyledTable"/>
    <w:tblStylePr w:type="nwCell">
      <w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="FF0000"/></w:tcPr>
    </w:tblStylePr>
    <w:tblStylePr w:type="seCell">
      <w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="00FF00"/></w:tcPr>
    </w:tblStylePr>
  </w:style>
</w:styles>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}
	doc, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(doc.Tables) != 1 || len(doc.Tables[0].Rows) != 2 || len(doc.Tables[0].Rows[0].Cells) != 2 {
		t.Fatalf("unexpected resolved table shape")
	}
	nw := doc.Tables[0].Rows[0].Cells[0]
	se := doc.Tables[0].Rows[1].Cells[1]
	if nw.Format.Shading != "FF0000" {
		t.Errorf("nwCell shading = %q, want FF0000", nw.Format.Shading)
	}
	if se.Format.Shading != "00FF00" {
		t.Errorf("seCell shading = %q, want 00FF00", se.Format.Shading)
	}
}

func TestResolvePackageInheritsSectionProperties(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>First</w:t></w:r></w:p>
    <w:sectPr>
      <w:pgSz w:w="12240" w:h="15840"/>
      <w:pgMar w:top="720" w:right="720" w:bottom="720" w:left="720" w:header="360" w:footer="360"/>
      <w:cols w:num="2"/>
    </w:sectPr>
    <w:p><w:r><w:t>Second</w:t></w:r></w:p>
    <w:sectPr>
      <w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720"/>
    </w:sectPr>
  </w:body>
</w:document>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}
	doc, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(doc.Sections))
	}
	// First section has explicit page size and columns
	if doc.Sections[0].PageSize == nil || doc.Sections[0].PageSize.Width != 12240 {
		t.Errorf("section 0 page size missing")
	}
	if doc.Sections[0].Columns == nil || doc.Sections[0].Columns.Num != 2 {
		t.Errorf("section 0 columns missing")
	}
	// Second section should inherit page size and columns from first
	if doc.Sections[1].PageSize == nil || doc.Sections[1].PageSize.Width != 12240 {
		t.Errorf("section 1 should inherit page size from section 0")
	}
	if doc.Sections[1].Columns == nil || doc.Sections[1].Columns.Num != 2 {
		t.Errorf("section 1 should inherit columns from section 0")
	}
	// Second section has its own margins
	if doc.Sections[1].PageMargin == nil || doc.Sections[1].PageMargin.Top != 1440 {
		t.Errorf("section 1 margin should be explicit")
	}
}

func TestResolvePackageResolvesThemeFillShading(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:shd w:val="clear" w:color="auto" w:fill="accent1" w:themeFill="accent1"/></w:pPr>
      <w:r><w:t>Hello</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/theme/theme1.xml": `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office">
  <a:themeElements>
    <a:clrScheme name="Office">
      <a:accent1><a:srgbClr val="4472C4"/></a:accent1>
    </a:clrScheme>
  </a:themeElements>
</a:theme>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}
	doc, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].Format.Shading != "4472C4" {
		t.Errorf("paragraph shading = %q, want 4472C4 (resolved theme accent1)", doc.Paragraphs[0].Format.Shading)
	}
}

func TestResolvePackageExtractsNumberingLevelRunFormat(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>
      <w:r><w:t>Item</w:t></w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/numbering.xml": `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="1">
    <w:lvl w:ilvl="0">
      <w:numFmt w:val="decimal"/>
      <w:lvlText w:val="%1."/>
      <w:rPr><w:color w:val="FF0000"/><w:sz w:val="28"/></w:rPr>
    </w:lvl>
  </w:abstractNum>
  <w:num w:numId="1"><w:abstractNumId w:val="1"/></w:num>
</w:numbering>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}
	doc, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(doc.Paragraphs) != 1 || doc.Paragraphs[0].Numbering == nil {
		t.Fatalf("expected paragraph with numbering")
	}
	n := doc.Paragraphs[0].Numbering
	if n.RunFormat == nil {
		t.Fatalf("expected numbering level run format")
	}
	if n.RunFormat.Color != "FF0000" {
		t.Errorf("level run color = %q, want FF0000", n.RunFormat.Color)
	}
	if n.RunFormat.FontSize == nil || *n.RunFormat.FontSize != 28 {
		t.Errorf("level run font size = %v, want 28", n.RunFormat.FontSize)
	}
}

func TestResolvePackageCapturesCrossParagraphStoryFieldInBody(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r><w:fldChar w:fldCharType="begin"/></w:r>
      <w:r><w:instrText xml:space="preserve"> TOC \o "1-3" </w:instrText></w:r>
      <w:r><w:fldChar w:fldCharType="separate"/></w:r>
      <w:r><w:t>Entry One</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t>Entry Two</w:t></w:r>
      <w:r><w:fldChar w:fldCharType="end"/></w:r>
    </w:p>
  </w:body>
</w:document>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if len(document.Fields) != 1 {
		t.Fatalf("document fields = %d, want 1", len(document.Fields))
	}
	field := document.Fields[0]
	if field.Kind != "TOC" {
		t.Fatalf("field kind = %q, want TOC", field.Kind)
	}
	if field.Arg != `TOC \o "1-3"` {
		t.Fatalf("field arg = %q, want %q", field.Arg, `TOC \o "1-3"`)
	}
	if len(field.Blocks) != 2 {
		t.Fatalf("field block count = %d, want 2", len(field.Blocks))
	}
	if field.Blocks[0].FieldKind != "TOC" || field.Blocks[1].FieldKind != "TOC" {
		t.Fatalf("field block kinds = %q/%q, want TOC/TOC", field.Blocks[0].FieldKind, field.Blocks[1].FieldKind)
	}
	if field.Blocks[0].Paragraph == nil || field.Blocks[0].Paragraph.Text != "Entry One" {
		t.Fatalf("field block 0 text = %+v, want Entry One", field.Blocks[0].Paragraph)
	}
	if field.Blocks[1].Paragraph == nil || field.Blocks[1].Paragraph.Text != "Entry Two" {
		t.Fatalf("field block 1 text = %+v, want Entry Two", field.Blocks[1].Paragraph)
	}
	if len(document.Paragraphs) != 2 {
		t.Fatalf("paragraph count = %d, want 2", len(document.Paragraphs))
	}
	if document.Paragraphs[0].Text != "Entry One" || document.Paragraphs[1].Text != "Entry Two" {
		t.Fatalf("paragraph texts = %q / %q, want Entry One / Entry Two", document.Paragraphs[0].Text, document.Paragraphs[1].Text)
	}
	if len(document.Paragraphs[0].Runs) != 1 || document.Paragraphs[0].Runs[0].Text != "Entry One" || document.Paragraphs[0].Runs[0].FieldKind != "" {
		t.Fatalf("paragraph 0 runs = %+v, want cached display run without per-run field semantics", document.Paragraphs[0].Runs)
	}
	if len(document.Paragraphs[1].Runs) != 1 || document.Paragraphs[1].Runs[0].Text != "Entry Two" || document.Paragraphs[1].Runs[0].FieldKind != "" {
		t.Fatalf("paragraph 1 runs = %+v, want cached display run without per-run field semantics", document.Paragraphs[1].Runs)
	}
}

func TestResolvePackageCapturesCrossParagraphStoryFieldInHeader(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
    <w:sectPr>
      <w:headerReference w:type="default" r:id="rId10"/>
    </w:sectPr>
  </w:body>
</w:document>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p>
    <w:r><w:fldChar w:fldCharType="begin"/></w:r>
    <w:r><w:instrText xml:space="preserve"> HYPERLINK "https://example.com" </w:instrText></w:r>
    <w:r><w:fldChar w:fldCharType="separate"/></w:r>
    <w:r><w:t>Header Link 1</w:t></w:r>
  </w:p>
  <w:p>
    <w:r><w:t>Header Link 2</w:t></w:r>
    <w:r><w:fldChar w:fldCharType="end"/></w:r>
  </w:p>
</w:hdr>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	header := document.Headers["word/header1.xml"]
	if len(header.Fields) != 1 {
		t.Fatalf("header fields = %d, want 1", len(header.Fields))
	}
	field := header.Fields[0]
	if field.Kind != "HYPERLINK" {
		t.Fatalf("header field kind = %q, want HYPERLINK", field.Kind)
	}
	if field.Target != "https://example.com" {
		t.Fatalf("header field target = %q, want https://example.com", field.Target)
	}
	if len(header.Paragraphs) != 2 {
		t.Fatalf("header paragraphs = %d, want 2", len(header.Paragraphs))
	}
	if header.Paragraphs[0].Text != "Header Link 1" || header.Paragraphs[1].Text != "Header Link 2" {
		t.Fatalf("header paragraph texts = %q / %q", header.Paragraphs[0].Text, header.Paragraphs[1].Text)
	}
	if len(header.Paragraphs[0].Runs) != 1 || header.Paragraphs[0].Runs[0].FieldKind != "" || header.Paragraphs[0].Runs[0].Kind != "hyperlink" || header.Paragraphs[0].Runs[0].Target != "https://example.com" {
		t.Fatalf("header paragraph 0 runs = %+v", header.Paragraphs[0].Runs)
	}
	if len(header.Paragraphs[1].Runs) != 1 || header.Paragraphs[1].Runs[0].FieldKind != "" || header.Paragraphs[1].Runs[0].Kind != "hyperlink" || header.Paragraphs[1].Runs[0].Target != "https://example.com" {
		t.Fatalf("header paragraph 1 runs = %+v", header.Paragraphs[1].Runs)
	}
}

func TestResolvePackagePreservesTheme(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml"   ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body>
</w:document>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault>
    <w:pPrDefault><w:pPr><w:spacing w:after="160"/></w:pPr></w:pPrDefault>
  </w:docDefaults>
</w:styles>`,
		"word/theme/theme1.xml": `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Custom Theme">
  <a:themeElements>
    <a:clrScheme name="Custom">
      <a:dk1><a:sysClr val="windowText" lastClr="1F1F1F"/></a:dk1>
      <a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>
      <a:dk2><a:srgbClr val="2E2E2E"/></a:dk2>
      <a:lt2><a:srgbClr val="F2F2F2"/></a:lt2>
      <a:accent1><a:srgbClr val="FF0000"/></a:accent1>
      <a:accent2><a:srgbClr val="00FF00"/></a:accent2>
      <a:accent3><a:srgbClr val="0000FF"/></a:accent3>
      <a:accent4><a:srgbClr val="FFFF00"/></a:accent4>
      <a:accent5><a:srgbClr val="FF00FF"/></a:accent5>
      <a:accent6><a:srgbClr val="00FFFF"/></a:accent6>
      <a:hlink><a:srgbClr val="0000FF"/></a:hlink>
      <a:folHlink><a:srgbClr val="800080"/></a:folHlink>
    </a:clrScheme>
    <a:fontScheme name="Custom Fonts">
      <a:majorFont>
        <a:latin typeface="Arial"/>
        <a:ea typeface="MS Gothic"/>
        <a:cs typeface="Arial CS"/>
      </a:majorFont>
      <a:minorFont>
        <a:latin typeface="Times New Roman"/>
        <a:ea typeface="MS Mincho"/>
        <a:cs typeface="Times CS"/>
      </a:minorFont>
    </a:fontScheme>
    <a:fmtScheme name="Custom Format">
      <a:fillStyleLst>
        <a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
      </a:fillStyleLst>
      <a:lnStyleLst>
        <a:ln w="12700" cap="flat"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:ln>
      </a:lnStyleLst>
      <a:effectStyleLst>
        <a:effectLst><a:outerShdw blurRad="40000" dist="20000" dir="5400000"><a:srgbClr val="000000"><a:alpha val="40000"/></a:srgbClr></a:outerShdw></a:effectLst>
      </a:effectStyleLst>
      <a:bgFillStyleLst>
        <a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill>
      </a:bgFillStyleLst>
    </a:fmtScheme>
  </a:themeElements>
</a:theme>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}

	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	if document.Theme == nil {
		t.Fatal("expected document.Theme to be populated")
	}
	if document.Theme.Name != "Custom Theme" {
		t.Errorf("theme name = %q, want %q", document.Theme.Name, "Custom Theme")
	}

	cs := document.Theme.ColorScheme
	if cs == nil {
		t.Fatal("expected color scheme")
	}
	if cs["dk1"] != "1F1F1F" {
		t.Errorf("dk1 = %q, want %q", cs["dk1"], "1F1F1F")
	}
	if cs["lt1"] != "FFFFFF" {
		t.Errorf("lt1 = %q, want %q", cs["lt1"], "FFFFFF")
	}
	if cs["accent1"] != "FF0000" {
		t.Errorf("accent1 = %q, want %q", cs["accent1"], "FF0000")
	}
	if cs["hlink"] != "0000FF" {
		t.Errorf("hlink = %q, want %q", cs["hlink"], "0000FF")
	}
	if cs["folHlink"] != "800080" {
		t.Errorf("folHlink = %q, want %q", cs["folHlink"], "800080")
	}

	fs := document.Theme.FontScheme
	if fs.MajorFont.Latin != "Arial" {
		t.Errorf("major latin = %q, want %q", fs.MajorFont.Latin, "Arial")
	}
	if fs.MajorFont.EastAsia != "MS Gothic" {
		t.Errorf("major eastAsia = %q, want %q", fs.MajorFont.EastAsia, "MS Gothic")
	}
	if fs.MajorFont.ComplexScript != "Arial CS" {
		t.Errorf("major cs = %q, want %q", fs.MajorFont.ComplexScript, "Arial CS")
	}
	if fs.MinorFont.Latin != "Times New Roman" {
		t.Errorf("minor latin = %q, want %q", fs.MinorFont.Latin, "Times New Roman")
	}
	if fs.MinorFont.EastAsia != "MS Mincho" {
		t.Errorf("minor eastAsia = %q, want %q", fs.MinorFont.EastAsia, "MS Mincho")
	}
	if fs.MinorFont.ComplexScript != "Times CS" {
		t.Errorf("minor cs = %q, want %q", fs.MinorFont.ComplexScript, "Times CS")
	}

	fmtScheme := document.Theme.FormatScheme
	if len(fmtScheme.FillStyles) != 1 {
		t.Errorf("fill styles count = %d, want 1", len(fmtScheme.FillStyles))
	}
	if len(fmtScheme.LineStyles) != 1 {
		t.Errorf("line styles count = %d, want 1", len(fmtScheme.LineStyles))
	}
	if len(fmtScheme.EffectStyles) != 1 {
		t.Errorf("effect styles count = %d, want 1", len(fmtScheme.EffectStyles))
	}
	if len(fmtScheme.BgFillStyles) != 1 {
		t.Errorf("bg fill styles count = %d, want 1", len(fmtScheme.BgFillStyles))
	}
}

func TestResolvePackageExpandsAutoTextFromAttachedTemplate(t *testing.T) {
	// Build template package with a glossary entry.
	templateData := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml"/>
  <Override PartName="/word/glossary/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.glossary+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Template</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdGlossary" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/glossaryDocument" Target="glossary/document.xml"/>
</Relationships>`,
		"word/glossary/document.xml": `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docParts>
    <w:docPart>
      <w:docPartPr><w:name w:val="Disclaimer"/></w:docPartPr>
      <w:docPartBody><w:p><w:r><w:t>Template clause</w:t></w:r></w:p></w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	// Build main document package that references the template.
	mainData := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:fldSimple w:instr='AUTOTEXT "Disclaimer"'/></w:p></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSettings" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:attachedTemplate r:id="rIdTemplate"/>
</w:settings>`,
		"word/_rels/settings.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdTemplate" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/attachedTemplate" Target="template.dotx" TargetMode="External"/>
</Relationships>`,
	})

	// Write both packages to temp files.
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "main.docx")
	templatePath := filepath.Join(tmpDir, "template.dotx")
	if err := os.WriteFile(mainPath, mainData, 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}
	if err := os.WriteFile(templatePath, templateData, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	opcPkg, err := opc.Open(mainPath)
	if err != nil {
		t.Fatalf("open main: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	wordPkg.SourcePath = mainPath
	document, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}

	runs := document.Paragraphs[0].Runs
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].Text != "Template clause" {
		t.Fatalf("autotext text = %q, want %q", runs[0].Text, "Template clause")
	}
}