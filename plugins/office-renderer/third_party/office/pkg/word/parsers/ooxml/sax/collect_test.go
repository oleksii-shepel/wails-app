package sax_test

import (
	"testing"

	"office/pkg/word/parsers/ooxml/sax"
)

func TestCollectBytesBuildsTypedSummary(t *testing.T) {
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
  <Relationship Id="rLink1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
  <Relationship Id="rAlt1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/afChunk" Target="afchunk/chunk1.html"/>
  <Relationship Id="rImg1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
  <Relationship Id="rOle1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="embeddings/oleObject1.bin"/>
  <Relationship Id="rPreview1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/preview1.png"/>
  <Relationship Id="rCp1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="wordprocessing/contentPart1.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
  <w:body>
    <w:sdt>
      <w:sdtPr>
        <w:id w:val="7"/>
        <w:alias w:val="Wrapper"/>
        <w:tag w:val="wrap"/>
        <w:richText/>
      </w:sdtPr>
      <w:sdtContent>
        <w:p><w:r><w:t>Wrapped</w:t></w:r></w:p>
      </w:sdtContent>
    </w:sdt>
    <w:p>
      <w:r><w:t>Hello </w:t></w:r>
      <w:hyperlink r:id="rLink1"><w:r><w:t>world</w:t></w:r></w:hyperlink>
      <w:r><m:oMath><m:r><m:t>x</m:t></m:r></m:oMath></w:r>
      <w:r>
        <w:drawing>
          <wp:inline>
            <wp:extent cx="914400" cy="914400"/>
            <wp:docPr id="3" name="Picture 3"/>
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
          <v:shape xmlns:v="urn:schemas-microsoft-com:vml"><v:imagedata r:id="rPreview1"/></v:shape>
          <o:OLEObject xmlns:o="urn:schemas-microsoft-com:office:office" ProgID="Excel.Sheet.12" r:id="rOle1"/>
        </w:object>
      </w:r>
      <w:r><w:contentPart r:id="rCp1"/></w:r>
    </w:p>
    <w:altChunk r:id="rAlt1"/>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
		"word/afchunk/chunk1.html":             "<html>chunk</html>",
		"word/media/image1.png":                "image-bytes",
		"word/media/preview1.png":              "preview-bytes",
		"word/embeddings/oleobject1.bin":       "ole-bytes",
		"word/wordprocessing/contentpart1.xml": "<cp/>",
	})

	summary, err := sax.CollectBytes(data, sax.Options{})
	if err != nil {
		t.Fatalf("CollectBytes: %v", err)
	}

	if len(summary.Paragraphs) != 3 {
		t.Fatalf("paragraph count = %d, want 3", len(summary.Paragraphs))
	}
	if summary.Paragraphs[0].Text != "Wrapped" || summary.Paragraphs[0].InTable {
		t.Fatalf("paragraph[0] = %#v", summary.Paragraphs[0])
	}
	if summary.Paragraphs[1].Text != "Hello worldx" || summary.Paragraphs[1].InTable {
		t.Fatalf("paragraph[1] = %#v", summary.Paragraphs[1])
	}
	if summary.Paragraphs[2].Text != "A" || !summary.Paragraphs[2].InTable {
		t.Fatalf("paragraph[2] = %#v", summary.Paragraphs[2])
	}

	if len(summary.Hyperlinks) != 1 || summary.Hyperlinks[0].Text != "world" || summary.Hyperlinks[0].Target != "https://example.com" {
		t.Fatalf("hyperlinks = %#v", summary.Hyperlinks)
	}

	if len(summary.ContentControls) != 1 || summary.ContentControls[0].Text != "Wrapped" || summary.ContentControls[0].Properties == nil || summary.ContentControls[0].Properties.Type != "richText" {
		t.Fatalf("content controls = %#v", summary.ContentControls)
	}

	if len(summary.Math) != 1 || summary.Math[0].Text != "x" || summary.Math[0].Model == nil {
		t.Fatalf("math = %#v", summary.Math)
	}

	if len(summary.Drawings) != 1 || summary.Drawings[0].GraphicKind != "picture" || summary.Drawings[0].Name != "Picture 3" || len(summary.Drawings[0].RelatedParts) != 1 {
		t.Fatalf("drawings = %#v", summary.Drawings)
	}

	if len(summary.EmbeddedObjects) != 1 || summary.EmbeddedObjects[0].ProgID != "Excel.Sheet.12" || len(summary.EmbeddedObjects[0].RelatedParts) != 2 {
		t.Fatalf("embedded objects = %#v", summary.EmbeddedObjects)
	}

	if len(summary.ContentParts) != 1 || summary.ContentParts[0].PayloadContentType != "application/xml" || string(summary.ContentParts[0].PayloadData) != "<cp/>" {
		t.Fatalf("content parts = %#v", summary.ContentParts)
	}

	if len(summary.AltChunks) != 1 || summary.AltChunks[0].PayloadContentType != "text/html" || string(summary.AltChunks[0].PayloadData) != "<html>chunk</html>" {
		t.Fatalf("altChunks = %#v", summary.AltChunks)
	}
}

func TestCollectBytesCanIncludeRelatedStories(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:t>Body</w:t></w:r></w:p>
    <w:sectPr><w:headerReference w:type="default" r:id="rHdr1"/></w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rHdr1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
</Relationships>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:p><w:r><w:t>Header</w:t></w:r></w:p>
</w:hdr>`,
	})

	summary, err := sax.CollectBytes(data, sax.Options{IncludeHeadersAndFooters: true})
	if err != nil {
		t.Fatalf("CollectBytes: %v", err)
	}

	if len(summary.Paragraphs) != 2 {
		t.Fatalf("paragraph count = %d, want 2", len(summary.Paragraphs))
	}
	if summary.Paragraphs[0].Story != sax.StoryDocument || summary.Paragraphs[1].Story != sax.StoryHeader {
		t.Fatalf("paragraph stories = %#v", summary.Paragraphs)
	}
}