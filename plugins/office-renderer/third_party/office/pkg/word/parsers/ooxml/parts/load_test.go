package parts_test

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"office/pkg/diag"
	"office/pkg/ooxml/opc"
	"office/pkg/word/parsers/ooxml/parts"
)

func TestLoadPackageParsesWordParts(t *testing.T) {
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
      <w:pPr>
        <w:pStyle w:val="Normal"/>
        <w:numPr>
          <w:ilvl w:val="0"/>
          <w:numId w:val="7"/>
        </w:numPr>
      </w:pPr>
      <w:r>
        <w:rPr><w:b/></w:rPr>
        <w:t>Hello</w:t>
      </w:r>
      <w:hyperlink r:id="rId9">
        <w:r><w:t> link</w:t></w:r>
      </w:hyperlink>
      <w:r><w:tab/></w:r>
      <w:r><w:br w:type="page"/></w:r>
    </w:p>
    <w:sectPr>
      <w:headerReference w:type="default" r:id="rId10"/>
      <w:footerReference w:type="default" r:id="rId11"/>
    </w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
  <Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
  <Relationship Id="rId10" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
  <Relationship Id="rId11" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
</Relationships>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Normal" w:default="1">
    <w:name w:val="Normal"/>
  </w:style>
</w:styles>`,
		"word/numbering.xml": `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="1">
    <w:lvl w:ilvl="0">
      <w:start w:val="1"/>
      <w:numFmt w:val="bullet"/>
      <w:lvlText w:val="*"/>
    </w:lvl>
  </w:abstractNum>
  <w:num w:numId="7">
    <w:abstractNumId w:val="1"/>
  </w:num>
</w:numbering>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Header text</w:t></w:r></w:p>
  <w:tbl>
    <w:tr>
      <w:tc><w:p><w:r><w:t>Header cell</w:t></w:r></w:p></w:tc>
    </w:tr>
  </w:tbl>
</w:hdr>`,
		"word/footer1.xml": `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Footer text</w:t></w:r></w:p>
</w:ftr>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	if wordPkg.MainPartName != "word/document.xml" {
		t.Fatalf("main part = %q", wordPkg.MainPartName)
	}
	if wordPkg.Document == nil || wordPkg.Document.Body == nil {
		t.Fatal("document body not parsed")
	}
	if got := len(wordPkg.Document.Body.Paragraphs); got != 1 {
		t.Fatalf("paragraph count = %d, want 1", got)
	}
	paragraph := wordPkg.Document.Body.Paragraphs[0]
	if paragraph == nil {
		t.Fatal("body paragraph missing")
	}
	if got := len(paragraph.Inlines); got != 4 {
		t.Fatalf("inline count = %d, want 4", got)
	}
	if paragraph.Inlines[0].Kind != parts.ParagraphInlineRun || paragraph.Inlines[1].Kind != parts.ParagraphInlineHyperlink || paragraph.Inlines[2].Kind != parts.ParagraphInlineRun || paragraph.Inlines[3].Kind != parts.ParagraphInlineRun {
		t.Fatalf("inline order kinds are wrong")
	}
	if paragraph.Runs[0].Properties == nil || paragraph.Runs[0].Properties.Bold == nil {
		t.Fatal("run properties not parsed")
	}
	if got := len(paragraph.Runs[0].Contents); got != 1 || paragraph.Runs[0].Contents[0].Kind != parts.RunContentText {
		t.Fatalf("first run contents = %#v", paragraph.Runs[0].Contents)
	}
	if got := len(paragraph.Runs[1].Contents); got != 1 || paragraph.Runs[1].Contents[0].Kind != parts.RunContentTab {
		t.Fatalf("tab run contents = %#v", paragraph.Runs[1].Contents)
	}
	if got := len(paragraph.Runs[2].Contents); got != 1 || paragraph.Runs[2].Contents[0].Kind != parts.RunContentBreak || paragraph.Runs[2].Contents[0].Break == nil || paragraph.Runs[2].Contents[0].Break.Type != "page" {
		t.Fatalf("break run contents = %#v", paragraph.Runs[2].Contents)
	}
	if got := len(wordPkg.Document.Body.Blocks); got != 2 {
		t.Fatalf("body block count = %d, want 2", got)
	}
	if wordPkg.Document.Body.Blocks[0].Kind != parts.BodyBlockParagraph {
		t.Fatalf("first block kind = %q", wordPkg.Document.Body.Blocks[0].Kind)
	}
	if wordPkg.Document.Body.Blocks[1].Kind != parts.BodyBlockSection {
		t.Fatalf("second block kind = %q", wordPkg.Document.Body.Blocks[1].Kind)
	}
	if wordPkg.Styles == nil || len(wordPkg.Styles.Styles) != 1 {
		t.Fatal("styles not parsed")
	}
	if wordPkg.Numbering == nil || len(wordPkg.Numbering.Nums) != 1 {
		t.Fatal("numbering not parsed")
	}
	if got := len(wordPkg.Headers); got != 1 {
		t.Fatalf("header count = %d, want 1", got)
	}
	header := wordPkg.Headers["word/header1.xml"]
	if header == nil {
		t.Fatal("resolved header not found")
	}
	if got := len(header.Blocks); got != 2 {
		t.Fatalf("header block count = %d, want 2", got)
	}
	if header.Blocks[0].Kind != parts.ContentBlockParagraph || header.Blocks[1].Kind != parts.ContentBlockTable {
		t.Fatalf("header block order = %q, %q", header.Blocks[0].Kind, header.Blocks[1].Kind)
	}
	if got := len(wordPkg.Footers); got != 1 {
		t.Fatalf("footer count = %d, want 1", got)
	}
	if got := len(wordPkg.ResolvedHeaders); got != 1 {
		t.Fatalf("resolved headers = %d, want 1", got)
	}
	if got := len(wordPkg.ResolvedFooters); got != 1 {
		t.Fatalf("resolved footers = %d, want 1", got)
	}
	if wordPkg.ResolvedHeaders[0].TargetPartName != "word/header1.xml" {
		t.Fatalf("resolved header target = %q", wordPkg.ResolvedHeaders[0].TargetPartName)
	}
	if wordPkg.ResolvedFooters[0].TargetPartName != "word/footer1.xml" {
		t.Fatalf("resolved footer target = %q", wordPkg.ResolvedFooters[0].TargetPartName)
	}
}

func TestLoadPackageParsesGlossaryDocument(t *testing.T) {
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
  <w:body><w:p><w:r><w:t>Main doc</w:t></w:r></w:p></w:body>
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
        <w:behavior w:val="content"/>
      </w:docPartPr>
      <w:docPartBody>
        <w:p><w:r><w:t>Clause body text</w:t></w:r></w:p>
      </w:docPartBody>
    </w:docPart>
  </w:docParts>
</w:glossaryDocument>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	if wordPkg.Glossary == nil {
		t.Fatal("glossary document not parsed")
	}
	if wordPkg.GlossaryPartName != "word/glossary/document.xml" {
		t.Fatalf("glossary part name = %q", wordPkg.GlossaryPartName)
	}
	if len(wordPkg.Glossary.DocParts) != 1 {
		t.Fatalf("doc part count = %d, want 1", len(wordPkg.Glossary.DocParts))
	}
	entry := wordPkg.Glossary.DocParts[0]
	if entry.Name == nil || entry.Name.Val != "Disclaimer" {
		t.Fatalf("glossary name = %+v", entry.Name)
	}
	if entry.Gallery == nil || entry.Gallery.Val != "AutoText" {
		t.Fatalf("glossary category name = %+v", entry.Gallery)
	}
	if entry.DocPartGallery == nil || entry.DocPartGallery.Val != "autoText" {
		t.Fatalf("glossary docPartGallery = %+v", entry.DocPartGallery)
	}
	if entry.Body == nil || len(entry.Body.Blocks) != 1 || entry.Body.Blocks[0].Kind != parts.ContentBlockParagraph {
		t.Fatalf("glossary body blocks = %+v", entry.Body)
	}
	if entry.Body.Paragraphs[0].Runs[0].Contents[0].Text == nil || entry.Body.Paragraphs[0].Runs[0].Contents[0].Text.Value != "Clause body text" {
		t.Fatalf("glossary body paragraph = %+v", entry.Body.Paragraphs[0])
	}
}

func TestLoadPackageRejectsNonWordPackage(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"/>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	if _, err := parts.LoadPackage(pkg); err == nil {
		t.Fatal("expected non-word package rejection")
	}
}

func TestLoadPackageIgnoresLastRenderedPageBreak(t *testing.T) {
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
        <w:lastRenderedPageBreak/>
        <w:t>After marker</w:t>
      </w:r>
    </w:p>
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

	paragraph := wordPkg.Document.Body.Paragraphs[0]
	if got := len(paragraph.Runs); got != 1 {
		t.Fatalf("run count = %d, want 1", got)
	}
	if got := len(paragraph.Runs[0].Contents); got != 2 {
		t.Fatalf("content count = %d, want 2", got)
	}
	if paragraph.Runs[0].Contents[0].Kind != parts.RunContentLastRenderedPageBreak {
		t.Fatalf("content kind = %v, want lastRenderedPageBreak", paragraph.Runs[0].Contents[0].Kind)
	}
	if paragraph.Runs[0].Contents[1].Kind != parts.RunContentText {
		t.Fatalf("content kind = %v, want text", paragraph.Runs[0].Contents[1].Kind)
	}
}

func TestLoadPackageParsesDigitalSignatures(t *testing.T) {
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
    <Reference URI="">
      <DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>
      <DigestValue>dGVzdA==</DigestValue>
    </Reference>
  </SignedInfo>
  <SignatureValue>YWJj</SignatureValue>
</Signature>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	if !wordPkg.HasSignatures {
		t.Fatal("expected HasSignatures = true")
	}
	if len(wordPkg.Signatures) != 1 {
		t.Fatalf("signature count = %d, want 1", len(wordPkg.Signatures))
	}
	if got := wordPkg.Signatures[0].PartName; got != "_xmlsignatures/sig1.xml" {
		t.Fatalf("signature part = %q, want %q", got, "_xmlsignatures/sig1.xml")
	}
	if wordPkg.Signatures[0].Signature == nil {
		t.Fatal("expected parsed XML-DSig envelope")
	}
	if got := wordPkg.Signatures[0].SignatureMethod(); got != "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256" {
		t.Fatalf("signature method = %q", got)
	}
	if len(wordPkg.SignatureVerifications) != 1 {
		t.Fatalf("signature verification count = %d, want 1", len(wordPkg.SignatureVerifications))
	}
	if wordPkg.SignatureVerifications[0].Valid {
		t.Fatalf("expected invalid verification result for unsigned test fixture, got %+v", wordPkg.SignatureVerifications[0])
	}
	if wordPkg.SignatureVerifications[0].TrustValid {
		t.Fatalf("expected TrustValid = false for invalid signature fixture, got %+v", wordPkg.SignatureVerifications[0])
	}
	if wordPkg.SignatureVerifications[0].RevocationChecked {
		t.Fatalf("expected RevocationChecked = false for fixture without CRL, got %+v", wordPkg.SignatureVerifications[0])
	}
}

func TestLoadPackageParsesAutoTextRunContent(t *testing.T) {
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
        <w:dayShort/>
        <w:dayLong/>
        <w:monthShort/>
        <w:monthLong/>
        <w:yearShort/>
        <w:yearLong/>
        <w:pgNum/>
      </w:r>
    </w:p>
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

	paragraph := wordPkg.Document.Body.Paragraphs[0]
	if got := len(paragraph.Runs); got != 1 {
		t.Fatalf("run count = %d, want 1", got)
	}
	want := []parts.RunContentKind{
		parts.RunContentDayShort,
		parts.RunContentDayLong,
		parts.RunContentMonthShort,
		parts.RunContentMonthLong,
		parts.RunContentYearShort,
		parts.RunContentYearLong,
		parts.RunContentPgNum,
	}
	if got := len(paragraph.Runs[0].Contents); got != len(want) {
		t.Fatalf("content count = %d, want %d", got, len(want))
	}
	for i, kind := range want {
		if paragraph.Runs[0].Contents[i].Kind != kind {
			t.Fatalf("content[%d] kind = %v, want %v", i, paragraph.Runs[0].Contents[i].Kind, kind)
		}
	}
}

func TestLoadPackageParsesParagraphRangeMarkup(t *testing.T) {
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
      <w:permStart w:id="1" w:edGrp="administrators"/>
      <w:r><w:t>Restricted</w:t></w:r>
      <w:permEnd w:id="1"/>
      <w:customXmlInsRangeStart w:id="2" w:author="Alice" w:date="2026-01-01T00:00:00Z"/>
      <w:r><w:t>Inserted</w:t></w:r>
      <w:customXmlInsRangeEnd w:id="2"/>
      <w:customXmlDelRangeStart w:id="3" w:author="Bob"/>
      <w:customXmlDelRangeEnd w:id="3"/>
      <w:customXmlMoveFromRangeStart w:id="4" w:author="Alice"/>
      <w:customXmlMoveFromRangeEnd w:id="4"/>
      <w:customXmlMoveToRangeStart w:id="5" w:author="Alice"/>
      <w:customXmlMoveToRangeEnd w:id="5"/>
    </w:p>
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

	paragraph := wordPkg.Document.Body.Paragraphs[0]
	if got := len(paragraph.PermStarts); got != 1 {
		t.Fatalf("permStart count = %d, want 1", got)
	}
	if paragraph.PermStarts[0].ID != 1 || paragraph.PermStarts[0].EdGrp != "administrators" {
		t.Fatalf("permStart = %+v, want ID=1 EdGrp=administrators", paragraph.PermStarts[0])
	}
	if got := len(paragraph.PermEnds); got != 1 {
		t.Fatalf("permEnd count = %d, want 1", got)
	}
	if paragraph.PermEnds[0].ID != 1 {
		t.Fatalf("permEnd.ID = %d, want 1", paragraph.PermEnds[0].ID)
	}
	if got := len(paragraph.CustomXmlInsRangeStarts); got != 1 {
		t.Fatalf("customXmlInsRangeStart count = %d, want 1", got)
	}
	if paragraph.CustomXmlInsRangeStarts[0].ID != 2 || paragraph.CustomXmlInsRangeStarts[0].Author != "Alice" {
		t.Fatalf("customXmlInsRangeStart = %+v", paragraph.CustomXmlInsRangeStarts[0])
	}
	if got := len(paragraph.CustomXmlInsRangeEnds); got != 1 {
		t.Fatalf("customXmlInsRangeEnd count = %d, want 1", got)
	}
	if got := len(paragraph.CustomXmlDelRangeStarts); got != 1 {
		t.Fatalf("customXmlDelRangeStart count = %d, want 1", got)
	}
	if got := len(paragraph.CustomXmlDelRangeEnds); got != 1 {
		t.Fatalf("customXmlDelRangeEnd count = %d, want 1", got)
	}
	if got := len(paragraph.CustomXmlMoveFromRangeStarts); got != 1 {
		t.Fatalf("customXmlMoveFromRangeStart count = %d, want 1", got)
	}
	if got := len(paragraph.CustomXmlMoveFromRangeEnds); got != 1 {
		t.Fatalf("customXmlMoveFromRangeEnd count = %d, want 1", got)
	}
	if got := len(paragraph.CustomXmlMoveToRangeStarts); got != 1 {
		t.Fatalf("customXmlMoveToRangeStart count = %d, want 1", got)
	}
	if got := len(paragraph.CustomXmlMoveToRangeEnds); got != 1 {
		t.Fatalf("customXmlMoveToRangeEnd count = %d, want 1", got)
	}
}

func TestLoadPackageParsesFldCharDataAndNumberingChange(t *testing.T) {
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
        <w:fldChar w:fldCharType="begin">
          <w:fldData>cached</w:fldData>
          <w:numberingChange w:id="1" w:author="Alice" w:original="1"/>
        </w:fldChar>
      </w:r>
    </w:p>
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

	paragraph := wordPkg.Document.Body.Paragraphs[0]
	if got := len(paragraph.Runs); got != 1 {
		t.Fatalf("run count = %d, want 1", got)
	}
	if got := len(paragraph.Runs[0].Contents); got != 1 {
		t.Fatalf("content count = %d, want 1", got)
	}
	fc := paragraph.Runs[0].Contents[0].FldChar
	if fc == nil {
		t.Fatal("expected fldChar")
	}
	if fc.FldData != "cached" {
		t.Fatalf("fldData = %q, want cached", fc.FldData)
	}
	if fc.NumberingChange == nil {
		t.Fatal("expected numberingChange")
	}
	if fc.NumberingChange.ID != 1 || fc.NumberingChange.Author != "Alice" || fc.NumberingChange.Original != "1" {
		t.Fatalf("numberingChange = %+v", fc.NumberingChange)
	}
}

func TestLoadPackageParsesPropertyChangeWithIDAndRPr(t *testing.T) {
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
        <w:rPr>
          <w:rPrChange w:id="42" w:author="Alice" w:date="2026-01-01T00:00:00Z">
            <w:rPr>
              <w:b/>
              <w:sz w:val="24"/>
            </w:rPr>
          </w:rPrChange>
        </w:rPr>
        <w:t>Hello</w:t>
      </w:r>
    </w:p>
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

	paragraph := wordPkg.Document.Body.Paragraphs[0]
	if got := len(paragraph.Runs); got != 1 {
		t.Fatalf("run count = %d, want 1", got)
	}
	rp := paragraph.Runs[0].Properties
	if rp == nil {
		t.Fatal("expected run properties")
	}
	if rp.PropertyChange == nil {
		t.Fatal("expected property change")
	}
	if rp.PropertyChange.ID != 42 {
		t.Fatalf("property change ID = %d, want 42", rp.PropertyChange.ID)
	}
	if rp.PropertyChange.Author != "Alice" {
		t.Fatalf("property change author = %q, want Alice", rp.PropertyChange.Author)
	}
	if rp.PropertyChange.RunProperties == nil {
		t.Fatal("expected inner run properties")
	}
	if rp.PropertyChange.RunProperties.Bold == nil {
		t.Fatal("expected inner bold")
	}
	if rp.PropertyChange.RunProperties.FontSize == nil || rp.PropertyChange.RunProperties.FontSize.Val != 24 {
		t.Fatalf("inner font size = %+v, want 24", rp.PropertyChange.RunProperties.FontSize)
	}
}

func TestLoadPackageParsesTablePropertyChanges(t *testing.T) {
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
        <w:tblPrChange w:id="1" w:author="Alice">
          <w:tblPrEx>
            <w:tblW w:w="5000" w:type="pct"/>
            <w:jc w:val="center"/>
          </w:tblPrEx>
        </w:tblPrChange>
      </w:tblPr>
      <w:tblGrid><w:gridCol w:w="1000"/></w:tblGrid>
      <w:tr>
        <w:trPr>
          <w:trPrChange w:id="2" w:author="Bob">
            <w:trPr>
              <w:trHeight w:val="400" w:hRule="atLeast"/>
              <w:tblHeader/>
            </w:trPr>
          </w:trPrChange>
        </w:trPr>
        <w:tc><w:p><w:r><w:t>X</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
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

	table := wordPkg.Document.Body.Tables[0]
	if table.Properties == nil || table.Properties.PropertyChange == nil {
		t.Fatal("expected tblPrChange")
	}
	if table.Properties.PropertyChange.ID != 1 || table.Properties.PropertyChange.Author != "Alice" {
		t.Fatalf("tblPrChange = %+v", table.Properties.PropertyChange)
	}
	if table.Properties.PropertyChange.TableProperties == nil {
		t.Fatal("expected inner tblPrEx")
	}
	if table.Properties.PropertyChange.TableProperties.Width == nil || table.Properties.PropertyChange.TableProperties.Width.Width != 5000 {
		t.Fatalf("inner tblW = %+v", table.Properties.PropertyChange.TableProperties.Width)
	}
	if table.Properties.PropertyChange.TableProperties.Alignment == nil || table.Properties.PropertyChange.TableProperties.Alignment.Val != "center" {
		t.Fatalf("inner jc = %+v", table.Properties.PropertyChange.TableProperties.Alignment)
	}

	row := table.Rows[0]
	if row.Properties == nil || row.Properties.PropertyChange == nil {
		t.Fatal("expected trPrChange")
	}
	if row.Properties.PropertyChange.ID != 2 || row.Properties.PropertyChange.Author != "Bob" {
		t.Fatalf("trPrChange = %+v", row.Properties.PropertyChange)
	}
	if row.Properties.PropertyChange.TableRowProperties == nil {
		t.Fatal("expected inner trPr")
	}
	if row.Properties.PropertyChange.TableRowProperties.Height == nil || row.Properties.PropertyChange.TableRowProperties.Height.Val != 400 {
		t.Fatalf("inner trHeight = %+v", row.Properties.PropertyChange.TableRowProperties.Height)
	}
	if row.Properties.PropertyChange.TableRowProperties.IsHeader == nil {
		t.Fatal("expected inner tblHeader")
	}
}

func TestLoadPackageParsesSettingsSurface(t *testing.T) {
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
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>X</w:t></w:r></w:p></w:body></w:document>`,
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:writeProtection w:recommended="1"/>
  <w:view w:val="print"/>
  <w:zoom w:percent="100"/>
  <w:removePersonalInformation/>
  <w:removeDateAndTime/>
  <w:doNotDisplayPageBoundaries/>
  <w:displayBackgroundShape/>
  <w:printPostScriptOverText/>
  <w:printFractionalCharacterWidth/>
  <w:printFormsData/>
  <w:embedTrueTypeFonts/>
  <w:embedSystemFonts/>
  <w:saveSubsetFonts/>
  <w:saveFormsData/>
  <w:mirrorMargins/>
  <w:alignBordersAndEdges/>
  <w:bordersDoNotSurroundHeader/>
  <w:bordersDoNotSurroundFooter/>
  <w:gutterAtTop/>
  <w:hideSpellingErrors/>
  <w:hideGrammaticalErrors/>
  <w:formsDesign/>
  <w:linkStyles/>
  <w:stylePaneFormatFilter w:val="headingStyles" />
  <w:stylePaneSortMethod w:val="priority"/>
  <w:documentType w:val="letter"/>
  <w:mailMerge w:mainDocumentType="letters"/>
  <w:revisionView/>
  <w:trackRevisions/>
  <w:doNotTrackMoves/>
  <w:doNotTrackFormatting/>
  <w:documentProtection w:edit="forms"/>
  <w:autoFormatOverride/>
  <w:styleLockTheme/>
  <w:styleLockQFSet/>
  <w:defaultTabStop w:val="720"/>
  <w:autoHyphenation/>
  <w:consecutiveHyphenLimit w:val="14"/>
  <w:hyphenationZone w:val="360"/>
  <w:doNotHyphenateCaps/>
  <w:showEnvelope/>
  <w:summaryLength w:val="50"/>
  <w:clickAndTypeStyle w:val="Normal"/>
  <w:defaultTableStyle w:val="TableNormal"/>
  <w:evenAndOddHeaders/>
  <w:bookFoldRevPrinting/>
  <w:bookFoldPrinting/>
  <w:bookFoldPrintingSheets w:val="4"/>
  <w:displayHorizontalDrawingGridEvery w:val="2"/>
  <w:displayVerticalDrawingGridEvery w:val="2"/>
  <w:doNotUseMarginsForDrawingGridOrigin/>
  <w:drawingGridHorizontalSpacing w:val="120"/>
  <w:drawingGridVerticalSpacing w:val="120"/>
  <w:drawingGridHorizontalOrigin w:val="0"/>
  <w:drawingGridVerticalOrigin w:val="0"/>
  <w:doNotShadeFormData/>
  <w:noPunctuationKerning/>
  <w:characterSpacingControl w:val="compressPunctuation"/>
  <w:printTwoOnOne/>
  <w:strictFirstAndLastChars/>
  <w:savePreviewPicture/>
  <w:doNotValidateAgainstSchema/>
  <w:saveInvalidXml/>
  <w:ignoreMixedContent/>
  <w:alwaysShowPlaceholderText/>
  <w:doNotDemarcateInvalidXml/>
  <w:saveXmlDataOnly/>
  <w:useXSLTWhenSaving/>
  <w:alwaysMergeEmptyNamespace/>
  <w:updateFields/>
  <w:hdrShapeDefaults/>
  <w:footnotePr><w:pos w:val="bottmPage"/></w:footnotePr>
  <w:endnotePr><w:pos w:val="docEnd"/></w:endnotePr>
  <w:compat>
    <w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/>
    <w:noTabHangInd/>
    <w:allowPNG/>
  </w:compat>
  <w:docVars>
    <w:docVar w:name="Version" w:val="1.0"/>
  </w:docVars>
  <w:rsids><w:rsidRoot w:val="12345678"/></w:rsids>
  <w:themeFontLang w:val="en-US" w:eastAsia="ja-JP"/>
  <w:clrSchemeMapping w:bg1="light1" w:t1="dark1" w:accent1="accent1"/>
  <w:doNotIncludeSubdocsInStats/>
  <w:doNotAutoCompressPictures/>
  <w:forceUpgrade/>
  <w:attachedTemplate r:id="rId1"/>
  <w:saveThroughXslt r:id="rId1" w:solutionID="{guid}"/>
  <w:attachedSchema w:val="http://example.com/schema"/>
  <w:smartTagType w:namespaceuri="http://example.com" w:name="Person"/>
  <w:shapeDefaults/>
  <w:decimalSymbol w:val="."/>
  <w:listSeparator w:val=","/>
</w:settings>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	if wordPkg.Settings == nil {
		t.Fatal("expected settings")
	}
	s := wordPkg.Settings
	if s.RemovePersonalInformation == nil {
		t.Error("removePersonalInformation")
	}
	if s.DisplayBackgroundShape == nil {
		t.Error("displayBackgroundShape")
	}
	if s.AlignBordersAndEdges == nil {
		t.Error("alignBordersAndEdges")
	}
	if s.HideSpellingErrors == nil {
		t.Error("hideSpellingErrors")
	}
	if s.FormsDesign == nil {
		t.Error("formsDesign")
	}
	if s.LinkStyles == nil {
		t.Error("linkStyles")
	}
	if s.RevisionView == nil {
		t.Error("revisionView")
	}
	if s.AutoFormatOverride == nil {
		t.Error("autoFormatOverride")
	}
	if s.ShowEnvelope == nil {
		t.Error("showEnvelope")
	}
	if s.PrintTwoOnOne == nil {
		t.Error("printTwoOnOne")
	}
	if s.StrictFirstAndLastChars == nil {
		t.Error("strictFirstAndLastChars")
	}
	if s.DoNotShadeFormData == nil {
		t.Error("doNotShadeFormData")
	}
	if s.NoPunctuationKerning == nil {
		t.Error("noPunctuationKerning")
	}
	if s.SavePreviewPicture == nil {
		t.Error("savePreviewPicture")
	}
	if s.DoNotValidateAgainstSchema == nil {
		t.Error("doNotValidateAgainstSchema")
	}
	if s.SaveInvalidXml == nil {
		t.Error("saveInvalidXml")
	}
	if s.IgnoreMixedContent == nil {
		t.Error("ignoreMixedContent")
	}
	if s.AlwaysShowPlaceholderText == nil {
		t.Error("alwaysShowPlaceholderText")
	}
	if s.DoNotDemarcateInvalidXml == nil {
		t.Error("doNotDemarcateInvalidXml")
	}
	if s.SaveXmlDataOnly == nil {
		t.Error("saveXmlDataOnly")
	}
	if s.UseXSLTWhenSaving == nil {
		t.Error("useXSLTWhenSaving")
	}
	if s.AlwaysMergeEmptyNamespace == nil {
		t.Error("alwaysMergeEmptyNamespace")
	}
	if s.DoNotIncludeSubdocsInStats == nil {
		t.Error("doNotIncludeSubdocsInStats")
	}
	if s.DoNotAutoCompressPictures == nil {
		t.Error("doNotAutoCompressPictures")
	}
	if s.BookFoldPrinting == nil {
		t.Error("bookFoldPrinting")
	}
	if s.ForceUpgrade == nil {
		t.Error("forceUpgrade")
	}
	if s.StylePaneFormatFilter == nil || s.StylePaneFormatFilter.Val != "headingStyles" {
		t.Errorf("stylePaneFormatFilter = %+v", s.StylePaneFormatFilter)
	}
	if s.StylePaneSortMethod == nil || s.StylePaneSortMethod.Val != "priority" {
		t.Errorf("stylePaneSortMethod = %+v", s.StylePaneSortMethod)
	}
	if s.AttachedTemplate == nil || s.AttachedTemplate.RelID != "rId1" {
		t.Errorf("attachedTemplate = %+v", s.AttachedTemplate)
	}
	if s.SaveThroughXslt == nil || s.SaveThroughXslt.RelID != "rId1" {
		t.Errorf("saveThroughXslt = %+v", s.SaveThroughXslt)
	}
	if len(s.DocVars) != 1 || s.DocVars[0].Name != "Version" {
		t.Errorf("docVars = %+v", s.DocVars)
	}
	if len(s.AttachedSchemas) != 1 || s.AttachedSchemas[0].Val != "http://example.com/schema" {
		t.Errorf("attachedSchemas = %+v", s.AttachedSchemas)
	}
	if s.ColorSchemeMapping == nil || s.ColorSchemeMapping.BG1 != "light1" {
		t.Errorf("clrSchemeMapping = %+v", s.ColorSchemeMapping)
	}
	if s.ReadModeInkLockDown != nil {
		t.Error("unexpected readModeInkLockDown")
	}
	if len(s.SmartTagTypes) != 1 || s.SmartTagTypes[0].Name != "Person" {
		t.Errorf("smartTagTypes = %+v", s.SmartTagTypes)
	}
	if s.ShapeDefaults == nil {
		t.Error("shapeDefaults")
	}
	if s.ClickAndTypeStyle == nil || s.ClickAndTypeStyle.Val != "Normal" {
		t.Errorf("clickAndTypeStyle = %+v", s.ClickAndTypeStyle)
	}
	if s.SummaryLength == nil || s.SummaryLength.Val != 50 {
		t.Errorf("summaryLength = %+v", s.SummaryLength)
	}
	if s.DrawingGridHorizontalSpacing == nil || s.DrawingGridHorizontalSpacing.Val != 120 {
		t.Errorf("drawingGridHorizontalSpacing = %+v", s.DrawingGridHorizontalSpacing)
	}
	if s.DrawingGridHorizontalOrigin == nil || s.DrawingGridHorizontalOrigin.Val != 0 {
		t.Errorf("drawingGridHorizontalOrigin = %+v", s.DrawingGridHorizontalOrigin)
	}
	if s.Compat == nil || len(s.Compat.Settings) < 2 {
		t.Errorf("compat settings = %+v", s.Compat)
	}
}

func TestLoadPackageReportsUnsupportedMarkupDiagnostics(t *testing.T) {
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
    <w:unsupportedBodyThing/>
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

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	if len(wordPkg.Diagnostics) < 2 {
		t.Fatalf("diagnostic count = %d, want at least 2", len(wordPkg.Diagnostics))
	}
	if !hasDiagnosticPath(wordPkg.Diagnostics, "body/unsupportedBodyThing") {
		t.Fatalf("missing unsupported body markup diagnostic: %+v", wordPkg.Diagnostics)
	}
	if !hasDiagnosticPath(wordPkg.Diagnostics, "run/unsupportedRunThing") {
		t.Fatalf("missing unsupported run markup diagnostic: %+v", wordPkg.Diagnostics)
	}
}

func TestLoadPackageReportsUnsupportedFeatureDiagnostics(t *testing.T) {
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
		"word/document.xml": `<w:document
  xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
  xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
  xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
  xmlns:v="urn:schemas-microsoft-com:vml"
  xmlns:o="urn:schemas-microsoft-com:office:office">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:inline>
            <a:graphic>
              <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram"/>
            </a:graphic>
          </wp:inline>
        </w:drawing>
      </w:r>
      <w:r>
        <mc:AlternateContent>
          <mc:Fallback>
            <v:shape><v:imagedata r:id="rIdImg1"/></v:shape>
          </mc:Fallback>
        </mc:AlternateContent>
      </w:r>
      <w:r>
        <w:object><o:OLEObject ProgID="Excel.Sheet.12" r:id="rOle1"/></w:object>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	for _, code := range []string{
		"word/unsupported-vml",
		"word/unsupported-ole",
	} {
		if !hasDiagnosticCode(wordPkg.Diagnostics, code) {
			t.Fatalf("missing diagnostic code %q in %+v", code, wordPkg.Diagnostics)
		}
	}
	// Verify OLE object RelID is captured.
	if len(wordPkg.Document.Body.Paragraphs) == 0 {
		t.Fatal("expected at least one paragraph")
	}
	lastPara := wordPkg.Document.Body.Paragraphs[len(wordPkg.Document.Body.Paragraphs)-1]
	if len(lastPara.Runs) == 0 {
		t.Fatal("expected at least one run in last paragraph")
	}
	lastRun := lastPara.Runs[len(lastPara.Runs)-1]
	if len(lastRun.Contents) != 1 {
		t.Fatalf("expected 1 content in last run, got %d", len(lastRun.Contents))
	}
	content := lastRun.Contents[0]
	if string(content.Kind) != "ole" {
		t.Fatalf("expected OLE content, got %q", content.Kind)
	}
	if content.OLE == nil {
		t.Fatal("expected OLE object")
	}
	if content.OLE.ProgID != "Excel.Sheet.12" {
		t.Fatalf("ProgID = %q, want Excel.Sheet.12", content.OLE.ProgID)
	}
	if content.OLE.RelID != "rOle1" {
		t.Fatalf("RelID = %q, want rOle1", content.OLE.RelID)
	}
}

func TestLoadPackageLoadsSmartArtDataParts(t *testing.T) {
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
		"word/document.xml": `<w:document
  xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
  xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:inline>
            <wp:extent cx="3657600" cy="2743200"/>
            <wp:docPr id="1" name="SmartArt 1"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram">
                <dgm:relIds r:dm="rIdSmart1"/>
              </a:graphicData>
            </a:graphic>
          </wp:inline>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSmart1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="diagrams/data1.xml"/>
</Relationships>`,
		"word/diagrams/data1.xml": `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram">
  <dgm:ptLst>
    <dgm:pt id="0" type="node"><dgm:prSet><dgm:fld>CEO</dgm:fld></dgm:prSet></dgm:pt>
  </dgm:ptLst>
</dgm:dataModel>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	if data, ok := wordPkg.SmartArtPartFor(wordPkg.MainPartName, "rIdSmart1"); !ok || !strings.Contains(string(data), "CEO") {
		t.Fatalf("smartart data model not loaded: %q", string(data))
	}
	if hasDiagnosticCode(wordPkg.Diagnostics, "word/unsupported-smartart") {
		t.Fatalf("unexpected smartart diagnostic: %+v", wordPkg.Diagnostics)
	}
}

func TestLoadPackageParsesCommentsAndCommentReferences(t *testing.T) {
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
      <w:commentRangeStart w:id="1"/>
      <w:r><w:t>Annotated</w:t></w:r>
      <w:commentRangeEnd w:id="1"/>
      <w:r><w:commentReference w:id="1"/></w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdComments" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/>
</Relationships>`,
		"word/comments.xml": `<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:comment w:id="1" w:author="Reviewer" w:date="2026-04-14T10:00:00Z">
    <w:p><w:r><w:t>Comment body</w:t></w:r></w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>Comment cell</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:comment>
</w:comments>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	if wordPkg.Comments == nil || len(wordPkg.Comments.Comments) != 1 {
		t.Fatalf("comments part = %+v", wordPkg.Comments)
	}
	comment := wordPkg.Comments.Comments[0]
	if comment.Author != "Reviewer" {
		t.Fatalf("comment author = %q, want Reviewer", comment.Author)
	}
	if got := len(comment.Blocks); got != 2 {
		t.Fatalf("comment block count = %d, want 2", got)
	}
	if comment.Blocks[0].Kind != parts.ContentBlockParagraph || comment.Blocks[1].Kind != parts.ContentBlockTable {
		t.Fatalf("comment block kinds = %#v", []parts.ContentBlockKind{comment.Blocks[0].Kind, comment.Blocks[1].Kind})
	}
	run := wordPkg.Document.Body.Paragraphs[0].Runs[1]
	if got := len(run.Contents); got != 1 || run.Contents[0].Kind != parts.RunContentCommentRef || run.Contents[0].CommentRef == nil || run.Contents[0].CommentRef.ID != 1 {
		t.Fatalf("comment reference run contents = %#v", run.Contents)
	}
}

func TestLoadPackageReportsSpecPartUnsupportedMarkupDiagnostics(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
  <Override PartName="/word/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
  <Override PartName="/word/footnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"/>
  <Override PartName="/word/endnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.endnotes+xml"/>
  <Override PartName="/word/comments.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:hyperlink r:id="rId42">
        <w:r><w:t>link</w:t></w:r>
        <w:smartTag/>
      </w:hyperlink>
      <w:r><w:commentReference w:id="1"/></w:r>
    </w:p>
    <w:tbl>
      <w:tblPr><w:unsupportedTblPr/></w:tblPr>
      <w:tblGrid><w:gridCol w:w="2400"/></w:tblGrid>
      <w:tr>
        <w:trPr><w:unsupportedTrPr/></w:trPr>
        <w:tc>
          <w:tcPr><w:unsupportedTcPr/></w:tcPr>
          <w:p><w:r><w:t>cell</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
    <w:sectPr><w:unsupportedSectPr/></w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
  <Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>
  <Relationship Id="rId6" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes" Target="endnotes.xml"/>
  <Relationship Id="rId7" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/>
  <Relationship Id="rId42" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
</Relationships>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:latentStyles/>
  <w:docDefaults><w:unsupportedDocDefaults/></w:docDefaults>
  <w:style w:type="paragraph" w:styleId="Normal" w:default="1">
    <w:name w:val="Normal"/>
    <w:unsupportedStyle/>
  </w:style>
</w:styles>`,
		"word/numbering.xml": `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:unsupportedNumbering/>
  <w:abstractNum w:abstractNumId="1">
    <w:lvl w:ilvl="0">
      <w:start w:val="1"/>
      <w:unsupportedLvl/>
    </w:lvl>
    <w:unsupportedAbstractNum/>
  </w:abstractNum>
  <w:num w:numId="1">
    <w:abstractNumId w:val="1"/>
    <w:lvlOverride w:ilvl="0"><w:unsupportedLvlOverride/></w:lvlOverride>
    <w:unsupportedNum/>
  </w:num>
</w:numbering>`,
		"word/theme/theme1.xml": `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">
  <a:themeElements>
    <a:clrScheme name="Office">
      <a:dk1><a:unsupportedClr/></a:dk1>
      <a:unsupportedColorScheme/>
    </a:clrScheme>
    <a:fontScheme name="Office">
      <a:majorFont><a:unsupportedFontSet/></a:majorFont>
      <a:unsupportedFontScheme/>
    </a:fontScheme>
    <a:unsupportedThemeElements/>
  </a:themeElements>
  <a:unsupportedTheme/>
</a:theme>`,
		"word/footnotes.xml": `<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:separator/>
  <w:footnote w:id="2">
    <w:unsupportedNote/>
    <w:p><w:r><w:t>Footnote text</w:t></w:r></w:p>
  </w:footnote>
</w:footnotes>`,
		"word/endnotes.xml": `<w:endnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:endnote w:id="3">
    <w:unsupportedEndnote/>
    <w:p><w:r><w:t>Endnote text</w:t></w:r></w:p>
  </w:endnote>
</w:endnotes>`,
		"word/comments.xml": `<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:commentEx/>
  <w:comment w:id="1">
    <w:unsupportedComment/>
    <w:p><w:r><w:t>Comment text</w:t></w:r></w:p>
  </w:comment>
</w:comments>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	// tblGrid should be parsed into Grid.Columns, not flagged as unsupported
	if tables := wordPkg.Document.Body.Tables; len(tables) == 0 {
		t.Fatalf("expected document to contain a table")
	} else if g := tables[0].Grid; g == nil || len(g.Columns) != 1 || g.Columns[0] != 2400 {
		t.Fatalf("expected tblGrid to parse 1 column of 2400 twips, got %+v", g)
	}

	for _, path := range []string{
		"docDefaults/unsupportedDocDefaults",
		"style/unsupportedStyle",
		"numbering/unsupportedNumbering",
		"abstractNum/unsupportedAbstractNum",
		"lvl/unsupportedLvl",
		"lvlOverride/unsupportedLvlOverride",
		"num/unsupportedNum",
		"theme/unsupportedTheme",
		"themeElements/unsupportedThemeElements",
		"clrScheme/unsupportedColorScheme",
		"theme color/unsupportedClr",
		"fontScheme/unsupportedFontScheme",
		"theme font set/unsupportedFontSet",
		"hyperlink/smartTag",
		"tblPr/unsupportedTblPr",
		"trPr/unsupportedTrPr",
		"tcPr/unsupportedTcPr",
		"sectPr/unsupportedSectPr",
		"footnotes/separator",
		"footnote/unsupportedNote",
		"endnote/unsupportedEndnote",
		"comments/commentEx",
		"comment/unsupportedComment",
	} {
		if !hasDiagnosticPath(wordPkg.Diagnostics, path) {
			t.Fatalf("missing diagnostic path %q in %+v", path, wordPkg.Diagnostics)
		}
	}
}

func TestLoadPackageSilencesExpectedOOXMLMetadata(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
	<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
	<Default Extension="xml" ContentType="application/xml"/>
	<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
	<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
	<Override PartName="/word/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:body>
		<w:p>
			<w:r><w:endnoteReference w:id="3"/></w:r>
		</w:p>
		<w:tbl>
			<w:tblPr>
				<w:tblInd w:w="108" w:type="dxa"/>
				<w:tblCellSpacing w:w="100" w:type="dxa"/>
				<w:tblCellMar>
					<w:top w:w="20" w:type="dxa"/>
					<w:left w:w="40" w:type="dxa"/>
					<w:bottom w:w="60" w:type="dxa"/>
					<w:right w:w="80" w:type="dxa"/>
				</w:tblCellMar>
				<w:tblLayout w:type="fixed"/>
				<w:tblOverlap w:val="never"/>
				<w:tblpPr w:leftFromText="0"/>
				<w:tblPrChange w:id="7" w:author="table" w:date="2026-04-14T10:00:00Z"><w:tblPr/></w:tblPrChange>
			</w:tblPr>
			<w:tblGrid>
				<w:gridCol w:w="1200"/>
			</w:tblGrid>
			<w:tr>
				<w:trPr>
					<w:gridBefore w:val="1"/>
					<w:gridAfter w:val="0"/>
					<w:wBefore w:w="0" w:type="dxa"/>
					<w:wAfter w:w="0" w:type="dxa"/>
					<w:ins w:id="1" w:author="tester" w:date="2026-04-14T10:00:00Z"/>
					<w:trPrChange w:id="8" w:author="row" w:date="2026-04-14T10:00:00Z"><w:trPr/></w:trPrChange>
				</w:trPr>
				<w:tc>
					<w:tcPr><w:tcPrChange w:id="9" w:author="cell" w:date="2026-04-14T10:00:00Z"><w:tcPr/></w:tcPrChange></w:tcPr>
					<w:p><w:r><w:t>cell</w:t></w:r></w:p>
				</w:tc>
			</w:tr>
		</w:tbl>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
	<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
</Relationships>`,
		"word/numbering.xml": `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:numPicBullet/>
	<w:abstractNum w:abstractNumId="1">
		<w:nsid w:val="12345678"/>
		<w:multiLevelType w:val="hybridMultilevel"/>
		<w:tmpl w:val="ABCDEF12"/>
		<w:lvl w:ilvl="0">
			<w:start w:val="1"/>
			<w:numFmt w:val="decimal"/>
			<w:lvlText w:val="%1."/>
			<w:lvlPicBulletId w:val="1"/>
			<w:isLgl/>
			<w:suff w:val="space"/>
			<w:pPr>
				<w:tabs><w:tab w:val="left" w:pos="720"/></w:tabs>
				<w:jc w:val="left"/>
			</w:pPr>
		</w:lvl>
	</w:abstractNum>
	<w:num w:numId="1"><w:abstractNumId w:val="1"/></w:num>
</w:numbering>`,
		"word/theme/theme1.xml": `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">
	<a:themeElements>
		<a:clrScheme name="Office">
			<a:dk1><a:srgbClr val="000000"/></a:dk1>
			<a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
		</a:clrScheme>
		<a:fontScheme name="Office">
			<a:majorFont><a:latin typeface="Calibri"/></a:majorFont>
			<a:minorFont><a:latin typeface="Calibri"/></a:minorFont>
		</a:fontScheme>
		<a:fmtScheme name="Office"/>
		<a:extLst/>
	</a:themeElements>
	<a:objectDefaults>
		<a:spDef>
			<a:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:spPr>
		</a:spDef>
		<a:lnDef><a:ln w="12700"/></a:lnDef>
		<a:txDef><a:bodyPr/></a:txDef>
	</a:objectDefaults>
	<a:extraClrSchemeLst>
		<a:extraClrScheme>
			<a:clrScheme name="Alt Office">
				<a:accent1><a:srgbClr val="123456"/></a:accent1>
			</a:clrScheme>
			<a:clrMap bg1="lt1" tx1="dk1"/>
		</a:extraClrScheme>
	</a:extraClrSchemeLst>
	<a:extLst/>
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

	for _, path := range []string{
		"abstractNum/nsid",
		"abstractNum/multiLevelType",
		"abstractNum/tmpl",
		"numbering/numPicBullet",
		"lvl/lvlPicBulletId",
		"lvl/isLgl",
		"lvl/suff",
		"lvlPPr/tabs",
		"lvlPPr/jc",
		"theme/objectDefaults",
		"theme/extraClrSchemeLst",
		"theme/extLst",
		"themeElements/fmtScheme",
		"themeElements/extLst",
		"tblPr/tblLayout",
		"tblPr/tblOverlap",
		"tblPr/tblpPr",
		"tblPr/tblPrChange",
		"tblPr/tblCellSpacing",
		"trPr/gridBefore",
		"trPr/gridAfter",
		"trPr/wBefore",
		"trPr/wAfter",
		"trPr/ins",
		"trPr/trPrChange",
		"tcPr/tcPrChange",
		"run/endnoteReference",
		"run/endnoteRef",
	} {
		if hasDiagnosticPath(wordPkg.Diagnostics, path) {
			t.Fatalf("unexpected diagnostic path %q in %+v", path, wordPkg.Diagnostics)
		}
	}

	if got := len(wordPkg.Document.Body.Tables); got != 1 {
		t.Fatalf("table count = %d, want 1", got)
	}
	if wordPkg.Theme == nil || wordPkg.Theme.ObjectDefs == nil {
		t.Fatalf("theme object defaults = %#v, want preserved defaults", wordPkg.Theme)
	}
	if wordPkg.Theme.ObjectDefs.ShapeDefault == nil || wordPkg.Theme.ObjectDefs.LineDefault == nil || wordPkg.Theme.ObjectDefs.TextDefault == nil {
		t.Fatalf("object defaults = %#v, want shape/line/text defaults", wordPkg.Theme.ObjectDefs)
	}
	if wordPkg.Theme.ObjectDefs.LineDefault.Ln == nil || wordPkg.Theme.ObjectDefs.LineDefault.Ln.W != 12700 {
		t.Fatalf("line default = %#v, want width 12700", wordPkg.Theme.ObjectDefs.LineDefault)
	}
	if wordPkg.Theme.ExtraColors == nil || len(wordPkg.Theme.ExtraColors.Schemes) != 1 {
		t.Fatalf("extra colors = %#v, want one alternate scheme", wordPkg.Theme.ExtraColors)
	}
	alt := wordPkg.Theme.ExtraColors.Schemes[0]
	if alt == nil || alt.ClrScheme == nil || alt.ClrScheme.Accent1 == nil || alt.ClrScheme.Accent1.SRGB == nil || alt.ClrScheme.Accent1.SRGB.Val != "123456" {
		t.Fatalf("alternate color scheme = %#v, want accent1 123456", alt)
	}
	if alt.ClrMap == nil || alt.ClrMap.Bg1 != "lt1" || alt.ClrMap.Tx1 != "dk1" {
		t.Fatalf("alternate color map = %#v, want bg1=lt1 tx1=dk1", alt.ClrMap)
	}
	table := wordPkg.Document.Body.Tables[0]
	if table.Grid == nil || len(table.Grid.Columns) != 1 || table.Grid.Columns[0] != 1200 {
		t.Fatalf("tblGrid = %+v, want 1 column of 1200 twips", table.Grid)
	}
	if table.Properties == nil || table.Properties.Indent == nil || table.Properties.Indent.Width != 108 {
		t.Fatalf("tblInd = %+v, want 108 twips", table.Properties)
	}
	if table.Properties.Layout == nil || table.Properties.Layout.Type != "fixed" {
		t.Fatalf("tblLayout = %+v, want fixed", table.Properties.Layout)
	}
	if table.Properties.CellSpacing == nil || table.Properties.CellSpacing.Width != 100 || table.Properties.CellSpacing.Type != "dxa" {
		t.Fatalf("tblCellSpacing = %+v, want 100 dxa", table.Properties.CellSpacing)
	}
	if table.Properties.PropertyChange == nil || table.Properties.PropertyChange.Author != "table" {
		t.Fatalf("tblPrChange = %+v, want author table", table.Properties.PropertyChange)
	}
	if table.Properties.CellMargin == nil || table.Properties.CellMargin.Top == nil || table.Properties.CellMargin.Top.Width != 20 {
		t.Fatalf("tblCellMar = %+v, want top padding of 20 twips", table.Properties.CellMargin)
	}
	if len(table.Rows) != 1 || table.Rows[0].Properties == nil {
		t.Fatalf("row properties = %+v, want one row with trPr", table.Rows)
	}
	rowProps := table.Rows[0].Properties
	if rowProps.GridBefore == nil || rowProps.GridBefore.Val != 1 {
		t.Fatalf("gridBefore = %+v, want 1", rowProps.GridBefore)
	}
	if rowProps.GridAfter == nil || rowProps.GridAfter.Val != 0 {
		t.Fatalf("gridAfter = %+v, want 0", rowProps.GridAfter)
	}
	if rowProps.WidthBefore == nil || rowProps.WidthBefore.Width != 0 || rowProps.WidthBefore.Type != "dxa" {
		t.Fatalf("wBefore = %+v, want 0 dxa", rowProps.WidthBefore)
	}
	if rowProps.WidthAfter == nil || rowProps.WidthAfter.Width != 0 || rowProps.WidthAfter.Type != "dxa" {
		t.Fatalf("wAfter = %+v, want 0 dxa", rowProps.WidthAfter)
	}
	if rowProps.PropertyChange == nil || rowProps.PropertyChange.Author != "row" {
		t.Fatalf("trPrChange = %+v, want author row", rowProps.PropertyChange)
	}
	if len(table.Rows[0].Cells) != 1 || table.Rows[0].Cells[0].Properties == nil || table.Rows[0].Cells[0].Properties.PropertyChange == nil || table.Rows[0].Cells[0].Properties.PropertyChange.Author != "cell" {
		t.Fatalf("tcPrChange = %+v, want author cell", table.Rows[0].Cells)
	}

	runs := wordPkg.Document.Body.Paragraphs[0].Runs
	if len(runs) != 1 || len(runs[0].Contents) != 1 || runs[0].Contents[0].Kind != parts.RunContentEndnoteRef {
		t.Fatalf("endnote reference contents = %#v", runs)
	}
}

func TestLoadPackagePreservesStructuredDocumentTags(t *testing.T) {
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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:sdt>
      <w:sdtPr>
        <w:id w:val="7"/>
        <w:alias w:val="Customer"/>
        <w:tag w:val="customer"/>
        <w:text/>
        <w:showingPlcHdr/>
        <w:dataBinding w:xpath="/root/customer" w:storeItemID="{store-1}"/>
      </w:sdtPr>
      <w:sdtContent>
        <w:p><w:r><w:t>Alice</w:t></w:r></w:p>
      </w:sdtContent>
    </w:sdt>
    <w:sdt>
      <w:sdtPr>
        <w:id w:val="8"/>
        <w:alias w:val="Building Block"/>
        <w:tag w:val="docpart"/>
        <w:docPartObj>
          <w:docPartGallery w:val="Table of Contents"/>
          <w:docPartCategory w:val="General"/>
          <w:docPartUnique/>
        </w:docPartObj>
      </w:sdtPr>
      <w:sdtContent>
        <w:p><w:r><w:t>Table of Contents</w:t></w:r></w:p>
      </w:sdtContent>
    </w:sdt>
    <w:p>
      <w:sdt>
        <w:sdtPr>
          <w:alias w:val="Status"/>
          <w:tag w:val="status"/>
          <w:checkBox><w:checked w:val="1"/></w:checkBox>
        </w:sdtPr>
        <w:sdtContent>
          <w:r><w:t>Done</w:t></w:r>
        </w:sdtContent>
      </w:sdt>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"word/styles.xml": `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Normal" w:default="1"><w:name w:val="Normal"/></w:style>
</w:styles>`,
		"customXml/item1.xml": `<root><customer>Bound Customer</customer></root>`,
		"customXml/_rels/item1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXmlProps" Target="itemProps1.xml"/>
</Relationships>`,
		"customXml/itemProps1.xml": `<ds:datastoreItem xmlns:ds="http://schemas.openxmlformats.org/officeDocument/2006/customXml" ds:itemID="{store-1}"/>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	body := wordPkg.Document.Body
	if body == nil {
		t.Fatal("document body not parsed")
	}
	if len(body.Blocks) != 3 {
		t.Fatalf("body block count = %d, want 3", len(body.Blocks))
	}
	if len(body.Paragraphs) != 3 {
		t.Fatalf("body paragraph count = %d, want 3", len(body.Paragraphs))
	}

	sdtBlock := body.Blocks[0]
	if sdtBlock.Kind != parts.BodyBlockSDT || sdtBlock.SdtProps == nil {
		t.Fatalf("first block = %#v, want body SDT with properties", sdtBlock)
	}
	if sdtBlock.SdtProps.ID != 7 || sdtBlock.SdtProps.Alias != "Customer" || sdtBlock.SdtProps.Tag != "customer" {
		t.Fatalf("body SDT props = %#v", sdtBlock.SdtProps)
	}
	if sdtBlock.SdtProps.Type != "plainText" || !sdtBlock.SdtProps.ShowingPlcHdr {
		t.Fatalf("body SDT type/placeholder = %#v", sdtBlock.SdtProps)
	}
	if sdtBlock.SdtProps.DataXPath != "/root/customer" || sdtBlock.SdtProps.DataStoreID != "{store-1}" {
		t.Fatalf("body SDT binding = %#v", sdtBlock.SdtProps)
	}
	if len(sdtBlock.SdtBlocks) != 1 || sdtBlock.SdtBlocks[0].Kind != parts.BodyBlockParagraph {
		t.Fatalf("body SDT children = %#v", sdtBlock.SdtBlocks)
	}
	if len(sdtBlock.SdtBlocks[0].Paragraph.Runs) != 1 || len(sdtBlock.SdtBlocks[0].Paragraph.Runs[0].Contents) != 1 {
		t.Fatalf("body SDT paragraph contents = %#v", sdtBlock.SdtBlocks[0].Paragraph)
	}
	if text := sdtBlock.SdtBlocks[0].Paragraph.Runs[0].Contents[0].Text; text == nil || text.Value != "Alice" {
		t.Fatalf("body SDT text = %#v, want Alice", text)
	}

	docPartBlock := body.Blocks[1]
	if docPartBlock.Kind != parts.BodyBlockSDT || docPartBlock.SdtProps == nil {
		t.Fatalf("second block = %#v, want body SDT with doc-part properties", docPartBlock)
	}
	if docPartBlock.SdtProps.Type != "docPartObj" || docPartBlock.SdtProps.DocPartGallery != "Table of Contents" || docPartBlock.SdtProps.DocPartCategory != "General" || !docPartBlock.SdtProps.DocPartUnique {
		t.Fatalf("docPart SDT props = %#v", docPartBlock.SdtProps)
	}
	if len(docPartBlock.SdtBlocks) != 1 || docPartBlock.SdtBlocks[0].Kind != parts.BodyBlockParagraph {
		t.Fatalf("docPart SDT children = %#v", docPartBlock.SdtBlocks)
	}

	para := body.Blocks[2].Paragraph
	if para == nil || len(para.Inlines) != 1 {
		t.Fatalf("third paragraph inlines = %#v", para)
	}
	inlineSdt := para.Inlines[0]
	if inlineSdt.Kind != parts.ParagraphInlineSDT || inlineSdt.SdtProps == nil {
		t.Fatalf("inline SDT = %#v", inlineSdt)
	}
	if inlineSdt.SdtProps.Type != "checkBox" || !inlineSdt.SdtProps.Checked || inlineSdt.SdtProps.Tag != "status" || inlineSdt.SdtProps.Alias != "Status" {
		t.Fatalf("inline SDT props = %#v", inlineSdt.SdtProps)
	}
	if len(inlineSdt.SdtInlines) != 1 || inlineSdt.SdtInlines[0].Kind != parts.ParagraphInlineRun {
		t.Fatalf("inline SDT children = %#v", inlineSdt.SdtInlines)
	}
	if text := inlineSdt.SdtInlines[0].Run.Contents[0].Text; text == nil || text.Value != "Done" {
		t.Fatalf("inline SDT text = %#v, want Done", text)
	}
	if got := string(wordPkg.CustomXMLParts["{STORE-1}"]); got != `<root><customer>Bound Customer</customer></root>` {
		t.Fatalf("custom XML data = %q", got)
	}
}

func hasDiagnosticPath(diagnostics diag.List, path string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Location.Path == path {
			return true
		}
	}
	return false
}

func hasDiagnosticCode(diagnostics diag.List, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func TestLoadPackageLoadsEmbeddedFonts(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="ttf" ContentType="application/x-font-ttf"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Hello</w:t></w:r></w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rFont1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fontTable.xml"/>
</Relationships>`,
		"word/fontTable.xml": `<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:font w:name="MyFont">
    <w:embedRegular r:id="rFontEmbed1" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
  </w:font>
</w:fonts>`,
		"word/_rels/fontTable.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rFontEmbed1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fonts/font1.ttf"/>
</Relationships>`,
		"word/fonts/font1.ttf": "dummy font data",
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open OPC package: %v", err)
	}

	pkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}

	if pkg.FontTable == nil {
		t.Fatal("FontTable is nil")
	}
	if len(pkg.FontTable.Fonts) != 1 {
		t.Fatalf("FontTable.Fonts = %d, want 1", len(pkg.FontTable.Fonts))
	}
	if pkg.FontTable.Fonts[0].Name != "MyFont" {
		t.Errorf("Font name = %q, want MyFont", pkg.FontTable.Fonts[0].Name)
	}

	if len(pkg.EmbeddedFonts) != 1 {
		t.Fatalf("EmbeddedFonts = %d, want 1", len(pkg.EmbeddedFonts))
	}
	ef := pkg.EmbeddedFonts[0]
	if ef.Typeface != "MyFont" {
		t.Errorf("Typeface = %q, want MyFont", ef.Typeface)
	}
	if ef.Regular == nil {
		t.Fatal("Regular font part is nil")
	}
	if ef.Regular.Name != "word/fonts/font1.ttf" {
		t.Errorf("Regular.Name = %q, want word/fonts/font1.ttf", ef.Regular.Name)
	}
	if string(ef.Regular.Data) != "dummy font data" {
		t.Errorf("Regular.Data = %q, want dummy font data", string(ef.Regular.Data))
	}
}

func TestLoadPackageLoadsEmbeddedFonts_NoFonts(t *testing.T) {
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
    <w:p><w:r><w:t>Hello</w:t></w:r></w:p>
  </w:body>
</w:document>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open OPC package: %v", err)
	}

	pkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}

	if pkg.FontTable != nil {
		t.Error("FontTable should be nil when no font relationship exists")
	}
	if len(pkg.EmbeddedFonts) != 0 {
		t.Errorf("EmbeddedFonts = %d, want 0", len(pkg.EmbeddedFonts))
	}
}

func TestLoadPackageLoadsEmbeddedFonts_EmptyFontTable(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Hello</w:t></w:r></w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rFont1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fontTable.xml"/>
</Relationships>`,
		"word/fontTable.xml": `<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
</w:fonts>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open OPC package: %v", err)
	}

	pkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}

	if pkg.FontTable == nil {
		t.Fatal("FontTable is nil")
	}
	if len(pkg.FontTable.Fonts) != 0 {
		t.Errorf("FontTable.Fonts = %d, want 0", len(pkg.FontTable.Fonts))
	}
	if len(pkg.EmbeddedFonts) != 0 {
		t.Errorf("EmbeddedFonts = %d, want 0", len(pkg.EmbeddedFonts))
	}
}

func TestScopedRelationshipsIsolateSameRelIDAcrossParts(t *testing.T) {
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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:t>doc</w:t></w:r></w:p>
    <w:sectPr><w:headerReference w:type="default" r:id="rIdHdr"/></w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://doc.example.com" TargetMode="External"/>
  <Relationship Id="rIdHdr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
</Relationships>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:p><w:hyperlink r:id="rId1"><w:r><w:t>header link</w:t></w:r></w:hyperlink></w:p>
</w:hdr>`,
		"word/_rels/header1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://header.example.com" TargetMode="External"/>
</Relationships>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	// Same bare rel ID "rId1" in different parts must resolve to different URLs.
	if got := wordPkg.HyperlinkTarget("word/document.xml", "rId1"); got != "https://doc.example.com" {
		t.Errorf("document hyperlink rId1 = %q, want https://doc.example.com", got)
	}
	if got := wordPkg.HyperlinkTarget("word/header1.xml", "rId1"); got != "https://header.example.com" {
		t.Errorf("header hyperlink rId1 = %q, want https://header.example.com", got)
	}
}

func TestScopedRelationshipsIsolateMediaAcrossParts(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="png" ContentType="image/png"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:t>doc</w:t></w:r></w:p>
    <w:sectPr><w:headerReference w:type="default" r:id="rIdHdr"/></w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/doc.png"/>
  <Relationship Id="rIdHdr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
</Relationships>`,
		"word/media/doc.png": `PNGdoc`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:p><w:r><w:drawing><wp:inline><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:blipFill><a:blip r:embed="rId1"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>
</w:hdr>`,
		"word/_rels/header1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/hdr.png"/>
</Relationships>`,
		"word/media/hdr.png": `PNGhdr`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}

	docMedia, ok := wordPkg.MediaPartFor("word/document.xml", "rId1")
	if !ok || string(docMedia.Data) != "PNGdoc" {
		t.Errorf("document media rId1 = %q, want PNGdoc", string(docMedia.Data))
	}
	hdrMedia, ok := wordPkg.MediaPartFor("word/header1.xml", "rId1")
	if !ok || string(hdrMedia.Data) != "PNGhdr" {
		t.Errorf("header media rId1 = %q, want PNGhdr", string(hdrMedia.Data))
	}
}

func TestScopedRelationshipsRoundTripDuplicateRelIDs(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
  <Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:hyperlink r:id="rId1"><w:r><w:t>doc link</w:t></w:r></w:hyperlink></w:p>
    <w:sectPr>
      <w:headerReference w:type="default" r:id="rIdHdr"/>
      <w:footerReference w:type="default" r:id="rIdFtr"/>
    </w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://doc.example.com" TargetMode="External"/>
  <Relationship Id="rIdHdr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
  <Relationship Id="rIdFtr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
</Relationships>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:p><w:hyperlink r:id="rId1"><w:r><w:t>header link</w:t></w:r></w:hyperlink></w:p>
</w:hdr>`,
		"word/_rels/header1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://header.example.com" TargetMode="External"/>
</Relationships>`,
		"word/footer1.xml": `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:p><w:hyperlink r:id="rId1"><w:r><w:t>footer link</w:t></w:r></w:hyperlink></w:p>
</w:ftr>`,
		"word/_rels/footer1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://footer.example.com" TargetMode="External"/>
</Relationships>`,
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	verify := func(prefix string, pkg *parts.Package) {
		if got := pkg.HyperlinkTarget("word/document.xml", "rId1"); got != "https://doc.example.com" {
			t.Errorf("%s document hyperlink rId1 = %q, want https://doc.example.com", prefix, got)
		}
		if got := pkg.HyperlinkTarget("word/header1.xml", "rId1"); got != "https://header.example.com" {
			t.Errorf("%s header hyperlink rId1 = %q, want https://header.example.com", prefix, got)
		}
		if got := pkg.HyperlinkTarget("word/footer1.xml", "rId1"); got != "https://footer.example.com" {
			t.Errorf("%s footer hyperlink rId1 = %q, want https://footer.example.com", prefix, got)
		}
	}
	verify("load", wordPkg)

	// Write and reload.
	var buf bytes.Buffer
	if err := parts.WritePackage(wordPkg, &buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	opcPkg2, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written: %v", err)
	}
	reloaded, err := parts.LoadPackage(opcPkg2)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	verify("round-trip", reloaded)
}

func TestScopedRelationshipsRoundTripChartParts(t *testing.T) {
	docChart := []byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:barChart><c:ser><c:tx><c:v>DocSeries</c:v></c:tx></c:ser></c:barChart></c:plotArea></c:chart></c:chartSpace>`)
	hdrChart := []byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:barChart><c:ser><c:tx><c:v>HdrSeries</c:v></c:tx></c:ser></c:barChart></c:plotArea></c:chart></c:chartSpace>`)

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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">
      <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
        <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart">
          <c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1"/>
        </a:graphicData>
      </a:graphic>
    </wp:inline></w:drawing></w:r></w:p>
    <w:sectPr><w:headerReference w:type="default" r:id="rIdHdr"/></w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="charts/docChart.xml"/>
  <Relationship Id="rIdHdr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
</Relationships>`,
		"word/charts/docChart.xml": string(docChart),
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:p><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">
    <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
      <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart">
        <c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1"/>
      </a:graphicData>
    </a:graphic>
  </wp:inline></w:drawing></w:r></w:p>
</w:hdr>`,
		"word/_rels/header1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="charts/hdrChart.xml"/>
</Relationships>`,
		"word/charts/hdrChart.xml": string(hdrChart),
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	verify := func(prefix string, pkg *parts.Package) {
		dc, ok := pkg.ChartPartFor("word/document.xml", "rId1")
		if !ok || !strings.Contains(string(dc), "DocSeries") {
			t.Errorf("%s document chart mismatch: %q", prefix, string(dc))
		}
		hc, ok := pkg.ChartPartFor("word/header1.xml", "rId1")
		if !ok || !strings.Contains(string(hc), "HdrSeries") {
			t.Errorf("%s header chart mismatch: %q", prefix, string(hc))
		}
	}
	verify("load", wordPkg)

	var buf bytes.Buffer
	if err := parts.WritePackage(wordPkg, &buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	opcPkg2, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written: %v", err)
	}
	reloaded, err := parts.LoadPackage(opcPkg2)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	verify("round-trip", reloaded)
}

func TestScopedRelationshipsRoundTripSmartArtParts(t *testing.T) {
	docSmart := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:pt modelId="0" type="doc" /></dgm:dataModel>`)
	ftrSmart := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:pt modelId="0" type="ftr" /></dgm:dataModel>`)

	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">
      <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
        <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram">
          <dgm:relIds xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" r:dm="rId1" r:lo="rId1" r:qs="rId1" r:cs="rId1"/>
        </a:graphicData>
      </a:graphic>
    </wp:inline></w:drawing></w:r></w:p>
    <w:sectPr><w:footerReference w:type="default" r:id="rIdFtr"/></w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="diagrams/doc.xml"/>
  <Relationship Id="rIdFtr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
</Relationships>`,
		"word/diagrams/doc.xml": string(docSmart),
		"word/footer1.xml": `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:p><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">
    <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
      <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram">
        <dgm:relIds xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" r:dm="rId1" r:lo="rId1" r:qs="rId1" r:cs="rId1"/>
      </a:graphicData>
    </a:graphic>
  </wp:inline></w:drawing></w:r></w:p>
</w:ftr>`,
		"word/_rels/footer1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="diagrams/ftr.xml"/>
</Relationships>`,
		"word/diagrams/ftr.xml": string(ftrSmart),
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	verify := func(prefix string, pkg *parts.Package) {
		ds, ok := pkg.SmartArtPartFor("word/document.xml", "rId1")
		if !ok || !strings.Contains(string(ds), `type="doc"`) {
			t.Errorf("%s document SmartArt mismatch: %q", prefix, string(ds))
		}
		fs, ok := pkg.SmartArtPartFor("word/footer1.xml", "rId1")
		if !ok || !strings.Contains(string(fs), `type="ftr"`) {
			t.Errorf("%s footer SmartArt mismatch: %q", prefix, string(fs))
		}
	}
	verify("load", wordPkg)

	var buf bytes.Buffer
	if err := parts.WritePackage(wordPkg, &buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	opcPkg2, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written: %v", err)
	}
	reloaded, err := parts.LoadPackage(opcPkg2)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	verify("round-trip", reloaded)
}

func TestScopedRelationshipsSmartArtLayoutStyleColorParts(t *testing.T) {
	docSmart := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:pt modelId="0" type="doc" /></dgm:dataModel>`)
	layoutXML := []byte(`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" dir="lrtb" typeUri="process"/>`)
	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"/>`)
	colorXML := []byte(`<dgm:colorsDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"/>`)

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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">
      <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
        <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram">
          <dgm:relIds xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" r:dm="rId1" r:lo="rId2" r:qs="rId3" r:cs="rId4"/>
        </a:graphicData>
      </a:graphic>
    </wp:inline></w:drawing></w:r></w:p>
    <w:sectPr/>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="diagrams/data.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout" Target="diagrams/layout.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle" Target="diagrams/style.xml"/>
  <Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors" Target="diagrams/color.xml"/>
</Relationships>`,
		"word/diagrams/data.xml":   string(docSmart),
		"word/diagrams/layout.xml": string(layoutXML),
		"word/diagrams/style.xml":  string(styleXML),
		"word/diagrams/color.xml":  string(colorXML),
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ds, ok := wordPkg.SmartArtPartFor("word/document.xml", "rId1")
	if !ok || !strings.Contains(string(ds), `type="doc"`) {
		t.Errorf("document SmartArt mismatch: %q", string(ds))
	}
	lo, ok := wordPkg.SmartArtLayoutPartFor("word/document.xml", "rId2")
	if !ok || !strings.Contains(string(lo), "layoutDef") {
		t.Errorf("document layout mismatch: %q", string(lo))
	}
	qs, ok := wordPkg.SmartArtStylePartFor("word/document.xml", "rId3")
	if !ok || !strings.Contains(string(qs), "styleDef") {
		t.Errorf("document style mismatch: %q", string(qs))
	}
	cs, ok := wordPkg.SmartArtColorPartFor("word/document.xml", "rId4")
	if !ok || !strings.Contains(string(cs), "colorsDef") {
		t.Errorf("document color mismatch: %q", string(cs))
	}
}

func TestScopedRelationshipsSmartArtSubPartRelationships(t *testing.T) {
	docSmart := []byte(`<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:pt modelId="0" type="doc" /></dgm:dataModel>`)
	layoutXML := []byte(`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" dir="lrtb" typeUri="process"/>`)
	styleXML := []byte(`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"/>`)
	colorXML := []byte(`<dgm:colorsDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"/>`)

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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">
      <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
        <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram">
          <dgm:relIds xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" r:dm="rId1"/>
        </a:graphicData>
      </a:graphic>
    </wp:inline></w:drawing></w:r></w:p>
    <w:sectPr/>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="diagrams/data.xml"/>
</Relationships>`,
		"word/diagrams/data.xml": string(docSmart),
		"word/diagrams/_rels/data.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdLo" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout" Target="layout.xml"/>
  <Relationship Id="rIdQs" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle" Target="style.xml"/>
  <Relationship Id="rIdCs" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors" Target="color.xml"/>
</Relationships>`,
		"word/diagrams/layout.xml": string(layoutXML),
		"word/diagrams/style.xml":  string(styleXML),
		"word/diagrams/color.xml":  string(colorXML),
	})

	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// The diagram data part relationships should have been loaded automatically.
	dataPartName := "word/diagrams/data.xml"
	lo, ok := wordPkg.SmartArtLayoutPartFor(dataPartName, "rIdLo")
	if !ok || !strings.Contains(string(lo), "layoutDef") {
		t.Errorf("diagram data layout mismatch: %q", string(lo))
	}
	qs, ok := wordPkg.SmartArtStylePartFor(dataPartName, "rIdQs")
	if !ok || !strings.Contains(string(qs), "styleDef") {
		t.Errorf("diagram data style mismatch: %q", string(qs))
	}
	cs, ok := wordPkg.SmartArtColorPartFor(dataPartName, "rIdCs")
	if !ok || !strings.Contains(string(cs), "colorsDef") {
		t.Errorf("diagram data color mismatch: %q", string(cs))
	}
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
func TestSmartArtDiagramDrawingAndMediaRoundTrip(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="png" ContentType="image/png"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/diagrams/data1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawingml.diagramData+xml"/>
  <Override PartName="/word/diagrams/drawing1.xml" ContentType="application/vnd.ms-office.drawingml.diagramDrawing+xml"/>
</Types>`,
		"_rels/.rels":                           `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":                     `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><w:body><w:p><w:r><w:drawing><wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><wp:extent cx="3000000" cy="1800000"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:relIds r:dm="rIdSmart"/></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels":          `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSmart" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="diagrams/data1.xml"/></Relationships>`,
		"word/diagrams/data1.xml":               `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><dgm:ptLst><dgm:pt modelId="1" type="doc"/></dgm:ptLst></dgm:dataModel>`,
		"word/diagrams/_rels/data1.xml.rels":    `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdDrawing" Type="http://schemas.microsoft.com/office/2007/relationships/diagramDrawing" Target="drawing1.xml"/></Relationships>`,
		"word/diagrams/drawing1.xml":            `<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dsp:spTree/></dsp:drawing>`,
		"word/diagrams/_rels/drawing1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdPic" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/smart.png"/></Relationships>`,
		"word/media/smart.png":                  "PNG-smartart",
	})
	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	wordPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	drawing, ok := wordPkg.SmartArtDrawingPartFor("word/diagrams/data1.xml", "rIdDrawing")
	if !ok || !strings.Contains(string(drawing), "dsp:drawing") {
		t.Fatalf("diagram drawing missing: %q", string(drawing))
	}
	media, ok := wordPkg.MediaPartFor("word/diagrams/drawing1.xml", "rIdPic")
	if !ok || string(media.Data) != "PNG-smartart" {
		t.Fatalf("diagram media = %#v", media)
	}

	var buf bytes.Buffer
	if err := parts.WritePackage(wordPkg, &buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	reopened, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	round, err := parts.LoadPackage(reopened)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if drawing, ok := round.SmartArtDrawingPartFor("word/diagrams/data1.xml", "rIdDrawing"); !ok || !strings.Contains(string(drawing), "dsp:drawing") {
		t.Fatalf("roundtrip drawing missing: %q", string(drawing))
	}
	if media, ok := round.MediaPartFor("word/diagrams/drawing1.xml", "rIdPic"); !ok || string(media.Data) != "PNG-smartart" {
		t.Fatalf("roundtrip media = %#v", media)
	}
}
func TestLoadPackageCapturesOpaquePartsAndUnknownRootRelationships(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/vendor/data.bin" ContentType="application/x-vendor-data"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rVendor" Type="urn:vendor:root-relation" Target="word/vendor/data.bin"/>
</Relationships>`,
		"word/document.xml":    `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p/></w:body></w:document>`,
		"word/vendor/data.bin": "opaque-vendor-payload",
	})
	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load word package: %v", err)
	}
	opaque, ok := wordPkg.OpaqueParts["word/vendor/data.bin"]
	if !ok || string(opaque.Data) != "opaque-vendor-payload" || opaque.ContentType != "application/x-vendor-data" {
		t.Fatalf("opaque part = %#v, ok=%v", opaque, ok)
	}
	if wordPkg.RootRelationships == nil {
		t.Fatal("root relationships missing")
	}
	found := false
	for _, rel := range wordPkg.RootRelationships.Relationships {
		if rel != nil && rel.ID == "rVendor" && rel.Type == "urn:vendor:root-relation" {
			found = true
		}
	}
	if !found {
		t.Fatalf("vendor root relationship missing: %#v", wordPkg.RootRelationships.Relationships)
	}
}
