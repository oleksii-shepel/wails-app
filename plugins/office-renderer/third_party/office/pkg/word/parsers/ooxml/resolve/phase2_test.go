package resolve_test

// Phase-2 completion tests: font names, table properties, DrawingML images.

import (
	"archive/zip"
	"bytes"
	"math"
	"strings"
	"testing"

	dml "office/pkg/ooxml/drawing"
	"office/pkg/ooxml/opc"
	"office/pkg/word/parsers/ooxml/parts"
	"office/pkg/word/parsers/ooxml/resolve"
	"office/pkg/word/render"
)

// ---- font names -------------------------------------------------------

func TestResolveFontNames(t *testing.T) {
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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:rPr>
          <w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:eastAsia="SimSun" w:cs="Times New Roman"/>
        </w:rPr>
        <w:t>fonted run</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	f := doc.Paragraphs[0].Runs[0].Format
	if f.FontASCII != "Arial" {
		t.Errorf("FontASCII = %q, want Arial", f.FontASCII)
	}
	if f.FontHAnsi != "Arial" {
		t.Errorf("FontHAnsi = %q, want Arial", f.FontHAnsi)
	}
	if f.FontEastAsia != "SimSun" {
		t.Errorf("FontEastAsia = %q, want SimSun", f.FontEastAsia)
	}
	if f.FontCS != "Times New Roman" {
		t.Errorf("FontCS = %q, want Times New Roman", f.FontCS)
	}
}

// ---- table properties -------------------------------------------------

func TestResolveTableProperties(t *testing.T) {
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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:tbl>
      <w:tblPr>
        <w:tblW w:w="9360" w:type="dxa"/>
        <w:jc w:val="center"/>
      </w:tblPr>
      <w:tr>
        <w:trPr>
          <w:trHeight w:val="567" w:hRule="exact"/>
        </w:trPr>
        <w:tc>
          <w:tcPr>
            <w:tcW w:w="4680" w:type="dxa"/>
            <w:gridSpan w:val="2"/>
            <w:vAlign w:val="center"/>
		    <w:textDirection w:val="tbRl"/>
            <w:shd w:val="clear" w:color="auto" w:fill="FF0000"/>
          </w:tcPr>
          <w:p><w:r><w:t>A</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Tables) == 0 {
		t.Fatal("expected at least one table")
	}
	tbl := doc.Tables[0]

	if tbl.Format.Width == nil || *tbl.Format.Width != 9360 {
		t.Errorf("table width = %v, want 9360", tbl.Format.Width)
	}
	if tbl.Format.WidthType != "dxa" {
		t.Errorf("table width type = %q, want dxa", tbl.Format.WidthType)
	}
	if tbl.Format.Alignment != "center" {
		t.Errorf("table alignment = %q, want center", tbl.Format.Alignment)
	}

	if len(tbl.Rows) == 0 {
		t.Fatal("expected at least one row")
	}
	row := tbl.Rows[0]

	if row.Format.Height == nil || *row.Format.Height != 567 {
		t.Errorf("row height = %v, want 567", row.Format.Height)
	}
	if row.Format.HeightRule != "exact" {
		t.Errorf("row height rule = %q, want exact", row.Format.HeightRule)
	}

	if len(row.Cells) == 0 {
		t.Fatal("expected at least one cell")
	}
	cell := row.Cells[0]

	if cell.Format.Width == nil || *cell.Format.Width != 4680 {
		t.Errorf("cell width = %v, want 4680", cell.Format.Width)
	}
	if cell.Format.GridSpan != 2 {
		t.Errorf("cell gridSpan = %d, want 2", cell.Format.GridSpan)
	}
	if cell.Format.VertAlign != "center" {
		t.Errorf("cell vertAlign = %q, want center", cell.Format.VertAlign)
	}
	if cell.Format.TextDirection != "tbRl" {
		t.Errorf("cell textDirection = %q, want tbRl", cell.Format.TextDirection)
	}
	if cell.Format.Shading != "FF0000" {
		t.Errorf("cell shading = %q, want FF0000", cell.Format.Shading)
	}
}

func TestResolveTableGridIndentAndCellMargins(t *testing.T) {
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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:body>
		<w:tbl>
			<w:tblPr>
				<w:tblInd w:w="108" w:type="dxa"/>
				<w:tblCellMar>
					<w:top w:w="20" w:type="dxa"/>
					<w:left w:w="40" w:type="dxa"/>
					<w:bottom w:w="60" w:type="dxa"/>
					<w:right w:w="80" w:type="dxa"/>
				</w:tblCellMar>
			</w:tblPr>
			<w:tblGrid>
				<w:gridCol w:w="1200"/>
				<w:gridCol w:w="2400"/>
			</w:tblGrid>
			<w:tr>
				<w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc>
				<w:tc><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc>
			</w:tr>
		</w:tbl>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Tables) != 1 {
		t.Fatalf("table count = %d, want 1", len(doc.Tables))
	}
	f := doc.Tables[0].Format
	if len(f.ColumnWidths) != 2 || f.ColumnWidths[0] != 1200 || f.ColumnWidths[1] != 2400 {
		t.Fatalf("column widths = %#v, want [1200 2400]", f.ColumnWidths)
	}
	if f.Indent != 108 {
		t.Fatalf("indent = %d, want 108", f.Indent)
	}
	if f.CellPadTop != 20 || f.CellPadLeft != 40 || f.CellPadBottom != 60 || f.CellPadRight != 80 {
		t.Fatalf("cell padding = top:%d left:%d bottom:%d right:%d, want 20/40/60/80", f.CellPadTop, f.CellPadLeft, f.CellPadBottom, f.CellPadRight)
	}
}

func TestResolveTableLayoutAndRowGridOffsets(t *testing.T) {
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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:body>
		<w:tbl>
			<w:tblPr>
				<w:tblLayout w:type="fixed"/>
			</w:tblPr>
			<w:tr>
				<w:trPr>
					<w:gridBefore w:val="1"/>
					<w:gridAfter w:val="2"/>
					<w:wBefore w:w="1200" w:type="dxa"/>
					<w:wAfter w:w="1800" w:type="dxa"/>
				</w:trPr>
				<w:tc><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc>
			</w:tr>
		</w:tbl>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Tables) != 1 {
		t.Fatalf("table count = %d, want 1", len(doc.Tables))
	}
	tbl := doc.Tables[0]
	if tbl.Format.Layout != "fixed" {
		t.Fatalf("table layout = %q, want fixed", tbl.Format.Layout)
	}
	if len(tbl.Rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(tbl.Rows))
	}
	row := tbl.Rows[0]
	if row.Format.GridBefore != 1 || row.Format.GridAfter != 2 {
		t.Fatalf("row grid offsets = before:%d after:%d, want 1/2", row.Format.GridBefore, row.Format.GridAfter)
	}
	if row.Format.WidthBefore == nil || *row.Format.WidthBefore != 1200 || row.Format.WidthBeforeType != "dxa" {
		t.Fatalf("row width before = %v/%q, want 1200/dxa", row.Format.WidthBefore, row.Format.WidthBeforeType)
	}
	if row.Format.WidthAfter == nil || *row.Format.WidthAfter != 1800 || row.Format.WidthAfterType != "dxa" {
		t.Fatalf("row width after = %v/%q, want 1800/dxa", row.Format.WidthAfter, row.Format.WidthAfterType)
	}
}

// ---- DrawingML inline image -------------------------------------------

func TestResolveDrawingMLInlineImage(t *testing.T) {
	// A minimal 1×1 white PNG (67 bytes).
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR length + type
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, // bit depth, color, CRC
		0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, // IDAT length + type
		0x78, 0x9c, 0x62, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, // compressed data
		0xe2, 0x21, 0xbc, 0x33, // CRC
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82, // IEND
	}

	textFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/media/image1.png" ContentType="image/png"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
            <wp:extent cx="914400" cy="914400"/>
            <wp:docPr id="1" name="Picture 1" descr="A tiny image"/>
            <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
              <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
                <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
                  <pic:blipFill>
                    <a:blip r:embed="rId5"/>
                  </pic:blipFill>
                </pic:pic>
              </a:graphicData>
            </a:graphic>
          </wp:inline>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
</Relationships>`,
	}

	data := buildPackageWithBinary(t, textFiles, map[string][]byte{
		"word/media/image1.png": minimalPNG,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	run := doc.Paragraphs[0].Runs[0]
	if run.Kind != "image" {
		t.Fatalf("run.Kind = %q, want image", run.Kind)
	}
	img := run.Image
	if img == nil {
		t.Fatal("run.Image is nil")
	}
	if img.MIMEType != "image/png" {
		t.Errorf("MIMEType = %q, want image/png", img.MIMEType)
	}
	if len(img.Data) == 0 {
		t.Error("image Data is empty")
	}
	if img.Width != 914400 {
		t.Errorf("Width = %d, want 914400 EMU", img.Width)
	}
	if img.Height != 914400 {
		t.Errorf("Height = %d, want 914400 EMU", img.Height)
	}
	if img.Alt != "A tiny image" {
		t.Errorf("Alt = %q, want A tiny image", img.Alt)
	}
	if img.Floating {
		t.Error("Floating should be false for wp:inline")
	}
}

func TestResolveHeaderScopedRelationships(t *testing.T) {
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54,
		0x78, 0x9c, 0x62, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01,
		0xe2, 0x21, 0xbc, 0x33,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	textFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
  <Override PartName="/word/media/image1.png" ContentType="image/png"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:r><w:t>Body text</w:t></w:r></w:p>
    <w:sectPr>
      <w:headerReference w:type="default" r:id="rIdHdr"/>
    </w:sectPr>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdHdr" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
</Relationships>`,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:p>
    <w:hyperlink r:id="rIdHdrLink">
      <w:r><w:t>Header link</w:t></w:r>
    </w:hyperlink>
  </w:p>
  <w:p>
    <w:r>
      <w:drawing>
        <wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
          <wp:extent cx="914400" cy="914400"/>
          <wp:docPr id="1" name="Header Picture" descr="Header scoped image"/>
          <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
            <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
              <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
                <pic:blipFill>
                  <a:blip r:embed="rIdHdrImage"/>
                </pic:blipFill>
              </pic:pic>
            </a:graphicData>
          </a:graphic>
        </wp:inline>
      </w:drawing>
    </w:r>
  </w:p>
</w:hdr>`,
		"word/_rels/header1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdHdrLink" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://header.example.com" TargetMode="External"/>
  <Relationship Id="rIdHdrImage" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
</Relationships>`,
	}

	data := buildPackageWithBinary(t, textFiles, map[string][]byte{
		"word/media/image1.png": minimalPNG,
	})
	doc := resolveDoc(t, data)

	header, ok := doc.Headers["word/header1.xml"]
	if !ok {
		t.Fatal("resolved header story missing")
	}
	if len(header.Paragraphs) < 2 {
		t.Fatalf("header paragraph count = %d, want at least 2", len(header.Paragraphs))
	}
	if runs := header.Paragraphs[0].Runs; len(runs) != 1 || runs[0].Kind != "hyperlink" || runs[0].Target != "https://header.example.com" {
		t.Fatalf("header hyperlink run = %+v", runs)
	}
	if runs := header.Paragraphs[1].Runs; len(runs) != 1 || runs[0].Kind != "image" || runs[0].Image == nil {
		t.Fatalf("header image run = %+v", runs)
	} else {
		if runs[0].Image.Alt != "Header scoped image" {
			t.Fatalf("header image alt = %q, want Header scoped image", runs[0].Image.Alt)
		}
		if runs[0].Image.MIMEType != "image/png" || len(runs[0].Image.Data) == 0 {
			t.Fatalf("header image payload = %+v", runs[0].Image)
		}
	}
}

func TestResolveDocumentBackgroundImage(t *testing.T) {
	data := buildPackageWithBinary(t,
		map[string]string{
			"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="png" ContentType="image/png"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
			"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
			"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdBg" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/bg.png"/>
</Relationships>`,
			"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
    xmlns:v="urn:schemas-microsoft-com:vml">
  <w:background w:color="F0F0F0">
    <v:background>
      <v:fill r:id="rIdBg" type="frame"/>
    </v:background>
  </w:background>
  <w:body>
    <w:p><w:r><w:t>Hello</w:t></w:r></w:p>
    <w:sectPr/>
  </w:body>
</w:document>`,
		},
		map[string][]byte{
			"word/media/bg.png": []byte("bg-image"),
		},
	)

	doc := resolveDoc(t, data)
	if doc.BackgroundColor != "F0F0F0" {
		t.Fatalf("BackgroundColor = %q, want F0F0F0", doc.BackgroundColor)
	}
	if doc.BackgroundImage == nil {
		t.Fatal("expected BackgroundImage to be resolved")
	}
	if doc.BackgroundImage.MIMEType != "image/png" {
		t.Fatalf("BackgroundImage.MIMEType = %q, want image/png", doc.BackgroundImage.MIMEType)
	}
	if string(doc.BackgroundImage.Data) != "bg-image" {
		t.Fatalf("BackgroundImage.Data = %q, want bg-image", string(doc.BackgroundImage.Data))
	}
}

func TestResolveNoteAndCommentScopedHyperlinks(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/footnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"/>
  <Override PartName="/word/comments.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"/>
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
  <Relationship Id="rIdFootnotes" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/>
  <Relationship Id="rIdComments" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/>
</Relationships>`,
		"word/footnotes.xml": `<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:footnote w:id="1">
    <w:p>
      <w:hyperlink r:id="rIdFootLink">
        <w:r><w:t>Footnote link</w:t></w:r>
      </w:hyperlink>
    </w:p>
  </w:footnote>
</w:footnotes>`,
		"word/_rels/footnotes.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdFootLink" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://footnote.example.com" TargetMode="External"/>
</Relationships>`,
		"word/comments.xml": `<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:comment w:id="1" w:author="Alice">
    <w:p>
      <w:hyperlink r:id="rIdCommentLink">
        <w:r><w:t>Comment link</w:t></w:r>
      </w:hyperlink>
    </w:p>
  </w:comment>
</w:comments>`,
		"word/_rels/comments.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdCommentLink" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://comment.example.com" TargetMode="External"/>
</Relationships>`,
	})

	doc := resolveDoc(t, data)

	footnote, ok := doc.Footnotes[1]
	if !ok || len(footnote.Paragraphs) == 0 || len(footnote.Paragraphs[0].Runs) != 1 {
		t.Fatalf("resolved footnote story = %+v", footnote)
	}
	if run := footnote.Paragraphs[0].Runs[0]; run.Kind != "hyperlink" || run.Target != "https://footnote.example.com" {
		t.Fatalf("resolved footnote hyperlink = %+v", run)
	}

	comment, ok := doc.Comments[1]
	if !ok || len(comment.Paragraphs) == 0 || len(comment.Paragraphs[0].Runs) != 1 {
		t.Fatalf("resolved comment story = %+v", comment)
	}
	if run := comment.Paragraphs[0].Runs[0]; run.Kind != "hyperlink" || run.Target != "https://comment.example.com" {
		t.Fatalf("resolved comment hyperlink = %+v", run)
	}
}

func TestResolveInlinePictureSDTCarriesContentControlToImageRun(t *testing.T) {
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54,
		0x78, 0x9c, 0x62, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01,
		0xe2, 0x21, 0xbc, 0x33,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	textFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/media/image1.png" ContentType="image/png"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:sdt>
        <w:sdtPr>
          <w:alias w:val="Photo"/>
          <w:tag w:val="photo"/>
          <w:picture/>
        </w:sdtPr>
        <w:sdtContent>
          <w:r>
            <w:drawing>
              <wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
                <wp:extent cx="914400" cy="914400"/>
                <wp:docPr id="1" name="Picture 1" descr="A controlled image"/>
                <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
                  <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
                    <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
                      <pic:blipFill>
                        <a:blip r:embed="rId5"/>
                      </pic:blipFill>
                    </pic:pic>
                  </a:graphicData>
                </a:graphic>
              </wp:inline>
            </w:drawing>
          </w:r>
        </w:sdtContent>
      </w:sdt>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
</Relationships>`,
	}

	data := buildPackageWithBinary(t, textFiles, map[string][]byte{
		"word/media/image1.png": minimalPNG,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	run := doc.Paragraphs[0].Runs[0]
	if run.Kind != "image" || run.Image == nil {
		t.Fatalf("run = %+v, want image run", run)
	}
	if run.ContentControl == nil {
		t.Fatal("image run ContentControl is nil")
	}
	if run.ContentControl.Type != "picture" || run.ContentControl.Tag != "photo" || run.ContentControl.Title != "Photo" {
		t.Fatalf("image run ContentControl = %+v", run.ContentControl)
	}
}

func TestResolveDrawingMLPictureCropAndTransform(t *testing.T) {
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54,
		0x78, 0x9c, 0x62, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01,
		0xe2, 0x21, 0xbc, 0x33,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	textFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/media/image1.png" ContentType="image/png"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
            <wp:extent cx="1828800" cy="914400"/>
            <wp:docPr id="1" name="Picture 1" descr="Cropped picture"/>
            <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
              <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
                <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
                  <pic:blipFill>
                    <a:blip r:embed="rId5"/>
                    <a:srcRect l="10000" t="5000" r="20000" b="15000"/>
                  </pic:blipFill>
                  <pic:spPr>
                    <a:xfrm rot="5400000" flipH="1" flipV="1">
                      <a:off x="0" y="0"/>
                      <a:ext cx="1828800" cy="914400"/>
                    </a:xfrm>
                  </pic:spPr>
                </pic:pic>
              </a:graphicData>
            </a:graphic>
          </wp:inline>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
</Relationships>`,
	}

	doc := resolveDoc(t, buildPackageWithBinary(t, textFiles, map[string][]byte{
		"word/media/image1.png": minimalPNG,
	}))
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	img := doc.Paragraphs[0].Runs[0].Image
	if img == nil {
		t.Fatal("expected resolved image")
	}
	if math.Abs(img.CropLeft-0.10) > 1e-9 || math.Abs(img.CropTop-0.05) > 1e-9 ||
		math.Abs(img.CropRight-0.20) > 1e-9 || math.Abs(img.CropBottom-0.15) > 1e-9 {
		t.Fatalf("unexpected crop = left %.3f top %.3f right %.3f bottom %.3f", img.CropLeft, img.CropTop, img.CropRight, img.CropBottom)
	}
	if math.Abs(img.RotationDeg-90.0) > 1e-9 {
		t.Fatalf("rotation = %.2f, want 90", img.RotationDeg)
	}
	if !img.FlipH || !img.FlipV {
		t.Fatalf("flip flags = H:%v V:%v, want true/true", img.FlipH, img.FlipV)
	}
}

func TestResolveDrawingMLPictureAdjustments(t *testing.T) {
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54,
		0x78, 0x9c, 0x62, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01,
		0xe2, 0x21, 0xbc, 0x33,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	textFiles := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/media/image1.png" ContentType="image/png"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing">
            <wp:extent cx="1828800" cy="914400"/>
            <wp:docPr id="1" name="Picture 1"/>
            <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
              <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
                <pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
                  <pic:blipFill>
                    <a:blip r:embed="rId5">
                      <a:lum bright="150000" contrast="80000"/>
                      <a:alphaMod val="50000"/>
	                  <a:grayscl/>
                    </a:blip>
                  </pic:blipFill>
                  <pic:spPr>
                    <a:xfrm>
                      <a:off x="0" y="0"/>
                      <a:ext cx="1828800" cy="914400"/>
                    </a:xfrm>
                  </pic:spPr>
                </pic:pic>
              </a:graphicData>
            </a:graphic>
          </wp:inline>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
</Relationships>`,
	}

	doc := resolveDoc(t, buildPackageWithBinary(t, textFiles, map[string][]byte{
		"word/media/image1.png": minimalPNG,
	}))
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	img := doc.Paragraphs[0].Runs[0].Image
	if img == nil {
		t.Fatal("expected resolved image")
	}
	if math.Abs(img.Brightness-2.5) > 1e-9 {
		t.Fatalf("brightness = %.2f, want 2.5", img.Brightness)
	}
	if math.Abs(img.Contrast-5.0) > 1e-9 {
		// Office renders +80% contrast as a 5x scale (1/(1-0.8)); the authored
		// percentage is asymptotic, not a linear offset.
		t.Fatalf("contrast = %.2f, want 5.0", img.Contrast)
	}
	if math.Abs(img.Alpha-0.5) > 1e-9 {
		t.Fatalf("alpha = %.2f, want 0.5", img.Alpha)
	}
	if !img.Grayscale {
		t.Fatal("grayscale = false, want true")
	}
}

func TestResolveDrawingMLTextBoxTransform(t *testing.T) {
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
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
    xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
    xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
            <wp:simplePos x="0" y="0"/>
            <wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
            <wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
            <wp:extent cx="914400" cy="457200"/>
            <wp:wrapNone/>
            <wp:docPr id="1" name="Textbox 1"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
                <wps:wsp>
                  <wps:txbx>
                    <w:txbxContent>
                      <w:p><w:r><w:t>Rotated box</w:t></w:r></w:p>
                    </w:txbxContent>
                  </wps:txbx>
                  <wps:spPr>
                    <a:xfrm rot="2700000" flipH="1">
                      <a:off x="0" y="0"/>
                      <a:ext cx="914400" cy="457200"/>
                    </a:xfrm>
                    <a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>
                    <a:solidFill><a:srgbClr val="FFF2CC"/></a:solidFill>
                  </wps:spPr>
                </wps:wsp>
              </a:graphicData>
            </a:graphic>
          </wp:anchor>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	tb := doc.Paragraphs[0].Runs[0].TextBox
	if tb == nil {
		t.Fatal("expected resolved textbox")
	}
	if math.Abs(tb.RotationDeg-45.0) > 1e-9 {
		t.Fatalf("rotation = %.2f, want 45", tb.RotationDeg)
	}
	if !tb.FlipH || tb.FlipV {
		t.Fatalf("flip flags = H:%v V:%v, want true/false", tb.FlipH, tb.FlipV)
	}
}

func TestResolveAlternateContentVMLTextBoxVerticalFlow(t *testing.T) {
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
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
    xmlns:v="urn:schemas-microsoft-com:vml">
  <w:body>
    <w:p>
      <w:r>
        <mc:AlternateContent>
          <mc:Fallback>
            <v:shape style="position:absolute;left:12pt;top:24pt;width:72pt;height:144pt" fillcolor="#FFF2CC">
              <v:textbox style="mso-layout-flow-alt:top-to-bottom">
                <w:txbxContent>
                  <w:p><w:r><w:t>VML fallback box</w:t></w:r></w:p>
                </w:txbxContent>
              </v:textbox>
            </v:shape>
          </mc:Fallback>
        </mc:AlternateContent>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	tb := doc.Paragraphs[0].Runs[0].TextBox
	if tb == nil {
		t.Fatal("expected resolved VML textbox")
	}
	if !tb.Floating {
		t.Fatal("expected VML textbox to be floating for absolute-positioned shape")
	}
	if tb.Vert != "vert" {
		t.Fatalf("Vert = %q, want vert", tb.Vert)
	}
	if tb.FillColor != "FFF2CC" {
		t.Fatalf("FillColor = %q, want FFF2CC", tb.FillColor)
	}
	if tb.Width != 914400 || tb.Height != 1828800 {
		t.Fatalf("size = (%d,%d), want (914400,1828800)", tb.Width, tb.Height)
	}
	if tb.X != 152400 || tb.Y != 304800 {
		t.Fatalf("position = (%d,%d), want (152400,304800)", tb.X, tb.Y)
	}
	if len(tb.Blocks) == 0 || tb.Blocks[0].Paragraph == nil || tb.Blocks[0].Paragraph.Text != "VML fallback box" {
		t.Fatalf("textbox blocks = %#v, want resolved paragraph text", tb.Blocks)
	}
}

func TestResolvePictVMLTextBoxVerticalFlow(t *testing.T) {
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
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:v="urn:schemas-microsoft-com:vml">
  <w:body>
    <w:p>
      <w:r>
        <w:pict>
          <v:shape style="width:72pt;height:144pt;layout-flow:vertical">
            <v:textbox>
              <w:txbxContent>
                <w:p><w:r><w:t>VML pict box</w:t></w:r></w:p>
              </w:txbxContent>
            </v:textbox>
          </v:shape>
        </w:pict>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	tb := doc.Paragraphs[0].Runs[0].TextBox
	if tb == nil {
		t.Fatal("expected resolved pict VML textbox")
	}
	if tb.Vert != "vert" {
		t.Fatalf("Vert = %q, want vert", tb.Vert)
	}
	if tb.Floating {
		t.Fatal("did not expect pict VML textbox without absolute positioning to float")
	}
	if len(tb.Blocks) == 0 || tb.Blocks[0].Paragraph == nil || tb.Blocks[0].Paragraph.Text != "VML pict box" {
		t.Fatalf("textbox blocks = %#v, want resolved paragraph text", tb.Blocks)
	}
}

func TestResolveDrawingMLShapeTransform(t *testing.T) {
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
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
    xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
    xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
            <wp:simplePos x="0" y="0"/>
            <wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
            <wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
            <wp:extent cx="914400" cy="457200"/>
            <wp:wrapNone/>
            <wp:docPr id="1" name="Shape 1"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
                <wps:wsp>
                  <wps:spPr>
                    <a:xfrm rot="10800000" flipV="1">
                      <a:off x="0" y="0"/>
                      <a:ext cx="914400" cy="457200"/>
                    </a:xfrm>
                    <a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>
                    <a:solidFill><a:srgbClr val="CFE2F3"/></a:solidFill>
                  </wps:spPr>
                </wps:wsp>
              </a:graphicData>
            </a:graphic>
          </wp:anchor>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	shape := doc.Paragraphs[0].Runs[0].Shape
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	if math.Abs(shape.RotationDeg-180.0) > 1e-9 {
		t.Fatalf("rotation = %.2f, want 180", shape.RotationDeg)
	}
	if shape.FlipH || !shape.FlipV {
		t.Fatalf("flip flags = H:%v V:%v, want false/true", shape.FlipH, shape.FlipV)
	}
	if shape.Geometry == nil || shape.Geometry.Name != "ellipse" {
		t.Fatalf("geometry = %#v, want ellipse", shape.Geometry)
	}
}

func TestResolveDrawingMLEffects(t *testing.T) {
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
    xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
            <wp:simplePos x="0" y="0"/>
            <wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
            <wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
            <wp:extent cx="914400" cy="457200"/>
            <wp:wrapNone/>
            <wp:docPr id="1" name="Shape 1"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
                <wps:wsp>
                  <wps:spPr>
                    <a:xfrm>
                      <a:off x="0" y="0"/>
                      <a:ext cx="914400" cy="457200"/>
                    </a:xfrm>
                    <a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>
                    <a:solidFill><a:srgbClr val="CFE2F3"/></a:solidFill>
                    <a:effectLst>
                      <a:glow rad="50800"><a:srgbClr val="FF0000"><a:alpha val="60000"/></a:srgbClr></a:glow>
                      <a:reflection blurRad="25400" dist="38100" dir="5400000" stA="25000"/>
                    </a:effectLst>
                  </wps:spPr>
                </wps:wsp>
              </a:graphicData>
            </a:graphic>
          </wp:anchor>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	var shape *resolve.Shape
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Shape != nil {
				shape = run.Shape
				break
			}
		}
		if shape != nil {
			break
		}
	}
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	if len(shape.Effects.Effects) != 2 {
		t.Fatalf("effect count = %d, want 2", len(shape.Effects.Effects))
	}
	glow, ok := shape.Effects.Effects[0].(dml.Glow)
	if !ok {
		t.Fatalf("effect type = %T, want drawing.Glow", shape.Effects.Effects[0])
	}
	if glow.Color != "FF0000" || glow.Alpha != 0.6 || math.Abs(glow.Radius-4.0) > 1e-9 {
		t.Fatalf("unexpected glow = %#v", glow)
	}
	reflection, ok := shape.Effects.Effects[1].(dml.Reflection)
	if !ok {
		t.Fatalf("effect type = %T, want drawing.Reflection", shape.Effects.Effects[1])
	}
	if reflection.Alpha != 0.25 || math.Abs(reflection.Blur-2.0) > 1e-9 || math.Abs(reflection.Distance-3.0) > 1e-9 {
		t.Fatalf("unexpected reflection = %#v", reflection)
	}
}

func TestResolveDrawingMLExtendedEffects(t *testing.T) {
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
    xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
            <wp:simplePos x="0" y="0"/>
            <wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
            <wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
            <wp:extent cx="914400" cy="457200"/>
            <wp:wrapNone/>
            <wp:docPr id="1" name="Shape 2"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
                <wps:wsp>
                  <wps:spPr>
                    <a:xfrm>
                      <a:off x="0" y="0"/>
                      <a:ext cx="914400" cy="457200"/>
                    </a:xfrm>
                    <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
                    <a:solidFill><a:srgbClr val="CFE2F3"/></a:solidFill>
                    <a:effectLst>
                      <a:alphaBiLevel thresh="33000"/>
                      <a:alphaMod val="50000"/>
                      <a:alphaOutset rad="25400"/>
                      <a:grayscl/>
                      <a:lum bright="120000" contrast="80000"/>
                      <a:duotone>
                        <a:srgbClr val="111111"/>
                        <a:srgbClr val="EEEEEE"/>
                      </a:duotone>
                      <a:clrChange>
                        <a:clrFrom><a:srgbClr val="222222"/></a:clrFrom>
                        <a:clrTo><a:srgbClr val="DDDDDD"/></a:clrTo>
                      </a:clrChange>
                      <a:clrRepl><a:srgbClr val="ABC123"/></a:clrRepl>
                      <a:fillOverlay blend="mult">
                        <a:solidFill><a:srgbClr val="00FF00"/></a:solidFill>
                      </a:fillOverlay>
                    </a:effectLst>
                  </wps:spPr>
                </wps:wsp>
              </a:graphicData>
            </a:graphic>
          </wp:anchor>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	var shape *resolve.Shape
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Shape != nil {
				shape = run.Shape
				break
			}
		}
		if shape != nil {
			break
		}
	}
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	if len(shape.Effects.Effects) != 9 {
		t.Fatalf("effect count = %d, want 9", len(shape.Effects.Effects))
	}
	if alpha, ok := shape.Effects.Effects[0].(dml.AlphaBiLevel); !ok || alpha.Threshold != 33000 {
		t.Fatalf("alpha bi-level effect = %#v", shape.Effects.Effects[0])
	}
	if alpha, ok := shape.Effects.Effects[1].(dml.AlphaMod); !ok || alpha.Amount != 50000 {
		t.Fatalf("alpha effect = %#v", shape.Effects.Effects[1])
	}
	if alpha, ok := shape.Effects.Effects[2].(dml.AlphaOutset); !ok || math.Abs(alpha.Radius-2.0) > 1e-9 {
		t.Fatalf("alpha outset effect = %#v", shape.Effects.Effects[2])
	}
	if _, ok := shape.Effects.Effects[3].(dml.Grayscale); !ok {
		t.Fatalf("grayscale effect = %#v", shape.Effects.Effects[3])
	}
	if lum, ok := shape.Effects.Effects[4].(dml.Lum); !ok || lum.Brightness != 120000 || lum.Contrast != 80000 {
		t.Fatalf("lum effect = %#v", shape.Effects.Effects[4])
	}
	if duotone, ok := shape.Effects.Effects[5].(dml.Duotone); !ok || duotone.Color1 != "111111" || duotone.Color2 != "EEEEEE" {
		t.Fatalf("duotone effect = %#v", shape.Effects.Effects[5])
	}
	if clrChange, ok := shape.Effects.Effects[6].(dml.ClrChange); !ok || clrChange.From != "222222" || clrChange.To != "DDDDDD" {
		t.Fatalf("clrChange effect = %#v", shape.Effects.Effects[6])
	}
	if clrRepl, ok := shape.Effects.Effects[7].(dml.ClrRepl); !ok || clrRepl.Color != "ABC123" {
		t.Fatalf("clrRepl effect = %#v", shape.Effects.Effects[7])
	}
	fillOverlay, ok := shape.Effects.Effects[8].(dml.FillOverlay)
	if !ok {
		t.Fatalf("fillOverlay effect = %#v", shape.Effects.Effects[8])
	}
	fill, ok := fillOverlay.Fill.(*dml.SolidFill)
	if !ok || fill.Color != "00FF00" || fillOverlay.BlendMode != "mult" {
		t.Fatalf("fillOverlay payload = %#v", fillOverlay)
	}
}

func TestResolveDrawingMLThreeDShapeEffects(t *testing.T) {
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
		"word/document.xml": `<w:document
		xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
	<w:body>
		<w:p>
			<w:r>
				<w:drawing>
					<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
						<wp:simplePos x="0" y="0"/>
						<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
						<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
						<wp:extent cx="914400" cy="457200"/>
						<wp:wrapNone/>
						<wp:docPr id="1" name="3D Shape"/>
						<a:graphic>
							<a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
								<wps:wsp>
									<wps:spPr>
										<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>
										<a:solidFill><a:srgbClr val="FF7043"/></a:solidFill>
										<a:scene3d>
											<a:camera prst="legacyPerspectiveTopLeft"><a:rot lat="-900000" lon="-600000"/></a:camera>
											<a:lightRig rig="threePt" dir="tl"/>
										</a:scene3d>
										<a:sp3d extrusionH="76200" contourW="25400" prstMaterial="metal">
											<a:bevelT w="38100" h="12700"/>
											<a:contourClr><a:srgbClr val="D9D9D9"/></a:contourClr>
											<a:extrusionClr><a:srgbClr val="595959"/></a:extrusionClr>
										</a:sp3d>
									</wps:spPr>
								</wps:wsp>
							</a:graphicData>
						</a:graphic>
					</wp:anchor>
				</w:drawing>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	var shape *resolve.Shape
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Shape != nil {
				shape = run.Shape
				break
			}
		}
		if shape != nil {
			break
		}
	}
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	var threeD dml.ThreeD
	found := false
	for _, effect := range shape.Effects.Effects {
		if value, ok := effect.(dml.ThreeD); ok {
			threeD = value
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected drawing.ThreeD in effects: %#v", shape.Effects.Effects)
	}
	if threeD.CameraPreset != "legacyPerspectiveTopLeft" || threeD.LightDirection != "tl" || threeD.Material != "metal" {
		t.Fatalf("unexpected 3D effect = %#v", threeD)
	}
	if math.Abs(threeD.ExtrusionDepth-6.0) > 1e-9 || math.Abs(threeD.ContourWidth-2.0) > 1e-9 || math.Abs(threeD.BevelTopHeight-1.0) > 1e-9 {
		t.Fatalf("unexpected 3D dimensions = %#v", threeD)
	}
}

func TestResolveDrawingMLSmartArt(t *testing.T) {
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
						<wp:docPr id="1" name="Org Chart"/>
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
		<dgm:pt id="0" type="root"><dgm:prSet><dgm:fld>CEO</dgm:fld></dgm:prSet></dgm:pt>
		<dgm:pt id="1" type="node"><dgm:prSet><dgm:fld>VP Sales</dgm:fld></dgm:prSet></dgm:pt>
	</dgm:ptLst>
	<dgm:cxnLst>
		<dgm:cxn srcId="0" destId="1"/>
	</dgm:cxnLst>
</dgm:dataModel>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	img := doc.Paragraphs[0].Runs[0].Image
	if img == nil {
		t.Fatal("expected resolved smartart image")
	}
	if img.MIMEType != "image/svg+xml" {
		t.Fatalf("mime type = %q, want image/svg+xml", img.MIMEType)
	}
	if img.Alt != "Org Chart" {
		t.Fatalf("alt = %q, want Org Chart", img.Alt)
	}
	svg := string(img.Data)
	if !strings.Contains(svg, "CEO") || !strings.Contains(svg, "VP Sales") {
		t.Fatalf("smartart svg missing node text: %s", svg)
	}
	if !strings.Contains(svg, "<line") {
		t.Fatalf("smartart svg missing connector lines: %s", svg)
	}
}

func TestResolveDrawingMLCustomGeometryShape(t *testing.T) {
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
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
    xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
    xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
            <wp:simplePos x="0" y="0"/>
            <wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
            <wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
            <wp:extent cx="914400" cy="914400"/>
            <wp:wrapNone/>
            <wp:docPr id="1" name="Shape 1"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
                <wps:wsp>
                  <wps:spPr>
                    <a:custGeom>
                      <a:pathLst>
                        <a:path w="100000" h="100000">
                          <a:moveTo><a:pt x="0" y="0"/></a:moveTo>
                          <a:lnTo><a:pt x="100000" y="0"/></a:lnTo>
                          <a:lnTo><a:pt x="50000" y="100000"/></a:lnTo>
                          <a:close/>
                        </a:path>
                      </a:pathLst>
                    </a:custGeom>
                    <a:solidFill><a:srgbClr val="CFE2F3"/></a:solidFill>
                  </wps:spPr>
                </wps:wsp>
              </a:graphicData>
            </a:graphic>
          </wp:anchor>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	shape := doc.Paragraphs[0].Runs[0].Shape
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	if shape.GeometryData == nil {
		t.Fatal("GeometryData = nil, want custom geometry preserved")
	}
	if shape.GeometryData.Type != dml.GeometryTypeCustom {
		t.Fatalf("geometry type = %q, want custom", shape.GeometryData.Type)
	}
	if len(shape.GeometryData.Paths) != 1 {
		t.Fatalf("path count = %d, want 1", len(shape.GeometryData.Paths))
	}
}

func TestResolveDrawingMLPresetGeometryAdjustHandles(t *testing.T) {
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
		"word/document.xml": `<w:document
    xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
    xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
    xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
    xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
  <w:body>
    <w:p>
      <w:r>
        <w:drawing>
          <wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
            <wp:simplePos x="0" y="0"/>
            <wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
            <wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
            <wp:extent cx="914400" cy="914400"/>
            <wp:wrapNone/>
            <wp:docPr id="1" name="Shape 1"/>
            <a:graphic>
              <a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
                <wps:wsp>
                  <wps:spPr>
                    <a:prstGeom prst="roundRect">
                      <a:avLst><a:gd name="adj1" fmla="val 25000"/></a:avLst>
                      <a:ahLst>
                        <a:ahXY gdRefX="adj1" minX="0" maxX="50000">
                          <a:pos x="adj1" y="0"/>
                        </a:ahXY>
                      </a:ahLst>
                    </a:prstGeom>
                    <a:solidFill><a:srgbClr val="CFE2F3"/></a:solidFill>
                  </wps:spPr>
                </wps:wsp>
              </a:graphicData>
            </a:graphic>
          </wp:anchor>
        </w:drawing>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	shape := doc.Paragraphs[0].Runs[0].Shape
	if shape == nil || shape.Geometry == nil {
		t.Fatal("expected resolved preset shape geometry")
	}
	if len(shape.Geometry.Handles) != 1 {
		t.Fatalf("handle count = %d, want 1", len(shape.Geometry.Handles))
	}
	if got := shape.Geometry.Handles[0]; got.Kind != "ahXY" || got.GDRefX != "adj1" || got.PosX != "adj1" {
		t.Fatalf("handle = %+v, want ahXY bound to adj1", got)
	}
}

func TestResolveDrawingMLThemeColorTransforms(t *testing.T) {
	textFiles := map[string]string{
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
		"word/document.xml": `<w:document
		xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
	<w:body>
		<w:p>
			<w:r>
				<w:drawing>
					<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
						<wp:simplePos x="0" y="0"/>
						<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
						<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
						<wp:extent cx="914400" cy="0"/>
						<wp:wrapNone/>
						<wp:docPr id="1" name="Line 1"/>
						<a:graphic>
							<a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
								<wps:wsp>
									<wps:cNvCnPr/>
									<wps:spPr>
										<a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="0"/></a:xfrm>
										<a:prstGeom prst="line"><a:avLst/></a:prstGeom>
										<a:ln w="28575">
											<a:solidFill>
												<a:schemeClr val="tx1">
													<a:lumMod val="95000"/>
													<a:lumOff val="5000"/>
												</a:schemeClr>
											</a:solidFill>
										</a:ln>
									</wps:spPr>
								</wps:wsp>
							</a:graphicData>
						</a:graphic>
					</wp:anchor>
				</w:drawing>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
</Relationships>`,
		"word/theme/theme1.xml": `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Test Theme">
	<a:themeElements>
		<a:clrScheme name="Test">
			<a:dk1><a:srgbClr val="000000"/></a:dk1>
			<a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
			<a:dk2><a:srgbClr val="222222"/></a:dk2>
			<a:lt2><a:srgbClr val="EEEEEE"/></a:lt2>
			<a:accent1><a:srgbClr val="FF0000"/></a:accent1>
			<a:accent2><a:srgbClr val="00FF00"/></a:accent2>
			<a:accent3><a:srgbClr val="0000FF"/></a:accent3>
			<a:accent4><a:srgbClr val="FFFF00"/></a:accent4>
			<a:accent5><a:srgbClr val="FF00FF"/></a:accent5>
			<a:accent6><a:srgbClr val="00FFFF"/></a:accent6>
			<a:hlink><a:srgbClr val="0000EE"/></a:hlink>
			<a:folHlink><a:srgbClr val="551A8B"/></a:folHlink>
		</a:clrScheme>
		<a:fontScheme name="Test">
			<a:majorFont><a:latin typeface="Calibri"/></a:majorFont>
			<a:minorFont><a:latin typeface="Calibri"/></a:minorFont>
		</a:fontScheme>
		<a:fmtScheme name="Test"/>
	</a:themeElements>
</a:theme>`,
	}

	doc := resolveDoc(t, buildPackageWithBinary(t, textFiles, nil))
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected a resolved paragraph run")
	}
	run := doc.Paragraphs[0].Runs[0]
	if run.Kind != "shape" || run.Shape == nil {
		t.Fatalf("run = %+v, want shape", run)
	}
	if run.Shape.Kind != "line" {
		t.Fatalf("shape.Kind = %q, want line", run.Shape.Kind)
	}
	if run.Shape.StrokeColor != "0D0D0D" {
		t.Fatalf("stroke color = %q, want 0D0D0D", run.Shape.StrokeColor)
	}
}

// ---- HTML renderer tests for new features -----------------------------

func TestHTMLFontFamily(t *testing.T) {
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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/></w:rPr>
        <w:t>calibri text</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})
	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	if !containsStr(out, "font-family:Calibri,Arial,sans-serif") {
		t.Errorf("expected Calibri font-family cascade in output, got:\n%s", out)
	}
}

func TestHTMLFontTableDrivenFallbacks(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml"  ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p>
      <w:r>
        <w:rPr><w:rFonts w:ascii="Acme Body" w:hAnsi="Acme Body"/></w:rPr>
        <w:t>serif</w:t>
      </w:r>
      <w:r>
        <w:rPr><w:rFonts w:ascii="Acme Terminal" w:hAnsi="Acme Terminal"/></w:rPr>
        <w:t>mono</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/fontTable.xml": `<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:font w:name="Acme Body"><w:family w:val="roman"/><w:pitch w:val="variable"/></w:font>
  <w:font w:name="Acme Terminal"><w:family w:val="modern"/><w:pitch w:val="fixed"/></w:font>
</w:fonts>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdFont1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fontTable.xml"/>
</Relationships>`,
	})

	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	checks := []string{
		"font-family:'Acme Body','Times New Roman',Times,serif",
		"font-family:'Acme Terminal',Consolas,'Courier New',monospace",
	}
	for _, want := range checks {
		if !containsStr(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestHTMLFontTablePanoseDrivenFallbacks(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
	<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
	<Default Extension="xml"  ContentType="application/xml"/>
	<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
	<Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:rPr><w:rFonts w:ascii="Acme Sans" w:hAnsi="Acme Sans"/></w:rPr>
				<w:t>sans</w:t>
			</w:r>
			<w:r>
				<w:rPr><w:rFonts w:ascii="Acme Mono" w:hAnsi="Acme Mono"/></w:rPr>
				<w:t>mono</w:t>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/fontTable.xml": `<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:font w:name="Acme Sans"><w:panose1 w:val="020B0604020202020204"/></w:font>
	<w:font w:name="Acme Mono"><w:panose1 w:val="020B0609020202020204"/></w:font>
</w:fonts>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rIdFont1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fontTable.xml"/>
</Relationships>`,
	})

	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	checks := []string{
		"font-family:'Acme Sans',Arial,sans-serif",
		"font-family:'Acme Mono',Consolas,'Courier New',monospace",
	}
	for _, want := range checks {
		if !containsStr(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestHTMLFontTablePanoseStyleFallbacks(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
	<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
	<Default Extension="xml"  ContentType="application/xml"/>
	<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
	<Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:rPr><w:rFonts w:ascii="Acme Oldstyle" w:hAnsi="Acme Oldstyle"/></w:rPr>
				<w:t>oldstyle</w:t>
			</w:r>
			<w:r>
				<w:rPr><w:rFonts w:ascii="Acme Flared" w:hAnsi="Acme Flared"/></w:rPr>
				<w:t>flared</w:t>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/fontTable.xml": `<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:font w:name="Acme Oldstyle"><w:panose1 w:val="02020603050405020304"/></w:font>
	<w:font w:name="Acme Flared"><w:panose1 w:val="020E0603020202020204"/></w:font>
</w:fonts>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rIdFont1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fontTable.xml"/>
</Relationships>`,
	})

	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	checks := []string{
		"font-family:'Acme Oldstyle',Georgia,'Times New Roman',serif",
		"font-family:'Acme Flared','Palatino Linotype','Book Antiqua',serif",
	}
	for _, want := range checks {
		if !containsStr(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestHTMLFontTablePanoseCondensedSansFallbacks(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
	<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
	<Default Extension="xml"  ContentType="application/xml"/>
	<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
	<Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:rPr><w:rFonts w:ascii="Acme Condensed Sans" w:hAnsi="Acme Condensed Sans"/></w:rPr>
				<w:t>condensed</w:t>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/fontTable.xml": `<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:font w:name="Acme Condensed Sans"><w:panose1 w:val="020B0606020202020204"/></w:font>
</w:fonts>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rIdFont1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fontTable.xml"/>
</Relationships>`,
	})

	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	want := "font-family:'Acme Condensed Sans','Arial Narrow',Arial,sans-serif"
	if !containsStr(out, want) {
		t.Fatalf("expected %q in output, got:\n%s", want, out)
	}
}

func TestHTMLFontTablePanoseSymbolFallbacks(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
	<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
	<Default Extension="xml"  ContentType="application/xml"/>
	<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
	<Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:rPr><w:rFonts w:ascii="Acme Symbol" w:hAnsi="Acme Symbol"/></w:rPr>
				<w:t>symbol</w:t>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/fontTable.xml": `<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
	<w:font w:name="Acme Symbol"><w:panose1 w:val="05000000000000000000"/></w:font>
</w:fonts>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rIdFont1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/font" Target="fontTable.xml"/>
</Relationships>`,
	})

	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	want := "font-family:'Acme Symbol','Segoe UI Symbol','Noto Sans Symbols',sans-serif"
	if !containsStr(out, want) {
		t.Fatalf("expected %q in output, got:\n%s", want, out)
	}
}

func TestHTMLOpenTypeRunProperties(t *testing.T) {
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
          <w14:ligatures w14:val="standardContextual"/>
          <w14:numSpacing w14:val="tabular"/>
          <w14:stylisticSets>
            <w14:styleSet w14:id="7" w14:val="1"/>
            <w14:styleSet w14:id="11" w14:val="1"/>
          </w14:stylisticSets>
        </w:rPr>
        <w:t>OpenType</w:t>
      </w:r>
    </w:p>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	checks := []string{
		"font-variant-ligatures:common-ligatures contextual",
		"font-variant-numeric:tabular-nums",
		"font-feature-settings:'ss07' 1, 'ss11' 1",
	}
	for _, want := range checks {
		if !containsStr(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestHTMLRunTextEffects(t *testing.T) {
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
			xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
			mc:Ignorable="w14">
	<w:body>
		<w:p>
			<w:r>
				<w:rPr>
					<w14:textFill>
						<w14:solidFill>
							<w14:srgbClr w14:val="CC3300"/>
						</w14:solidFill>
					</w14:textFill>
					<w14:textOutline w14:w="12700">
						<w14:solidFill>
							<w14:srgbClr w14:val="00AA88"/>
						</w14:solidFill>
					</w14:textOutline>
					<w14:glow w14:rad="38100">
						<w14:srgbClr w14:val="3366FF"><w14:alpha w14:val="60000"/></w14:srgbClr>
					</w14:glow>
					<w14:shadow w14:blurRad="25400" w14:dist="12700" w14:dir="0">
						<w14:srgbClr w14:val="000000"><w14:alpha w14:val="50000"/></w14:srgbClr>
					</w14:shadow>
				</w:rPr>
				<w:t>Effects</w:t>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	checks := []string{
		"color:#CC3300",
		"-webkit-text-stroke:1.00pt #00AA88",
		"text-shadow:1.00pt 0.00pt 2.00pt rgba(0, 0, 0, 0.50), 0 0 3.00pt rgba(51, 102, 255, 0.60)",
	}
	for _, want := range checks {
		if !containsStr(out, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestHTMLTableCellWidth(t *testing.T) {
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
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:tbl>
      <w:tr>
        <w:tc>
          <w:tcPr><w:tcW w:w="2880" w:type="dxa"/></w:tcPr>
          <w:p><w:r><w:t>cell</w:t></w:r></w:p>
        </w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})
	doc := resolveDoc(t, data)
	out := renderHTML(t, doc)
	// 2880 twips / 20 = 144pt
	if !containsStr(out, "width:144.0pt") {
		t.Errorf("expected cell width:144.0pt in output, got:\n%s", out)
	}
}

// ---- local helpers ----------------------------------------------------

func resolveDoc(t *testing.T, data []byte) *resolve.Document {
	t.Helper()
	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	wordPkg, err := parts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load parts: %v", err)
	}
	doc, err := resolve.ResolvePackage(wordPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	return doc
}

func renderHTML(t *testing.T, doc *resolve.Document) string {
	t.Helper()
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	return out
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStrInner(s, sub))
}

func containsStrInner(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// buildPackageWithBinary builds a ZIP combining string text files and binary files.
func buildPackageWithBinary(t *testing.T, textFiles map[string]string, binFiles map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range textFiles {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	for name, data := range binFiles {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

func TestResolveDrawingMLSRGBColorTransforms(t *testing.T) {
	// A shape with an sRGB fill that has shade and tint transforms.
	// Shade=50000 (0.5) halves each channel: FF0000 -> 800000
	// Tint=0 is explicitly zero (handled by HasTint).
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
		xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
	<w:body>
		<w:p>
			<w:r>
				<w:drawing>
					<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
						<wp:simplePos x="0" y="0"/>
						<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
						<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
						<wp:extent cx="914400" cy="914400"/>
						<wp:wrapNone/>
						<wp:docPr id="1" name="Rect 1"/>
						<a:graphic>
							<a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
								<wps:wsp>
									<wps:cNvCnPr/>
									<wps:spPr>
										<a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm>
										<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
										<a:solidFill>
											<a:srgbClr val="FF0000">
												<a:shade val="50000"/>
												<a:tint val="0"/>
											</a:srgbClr>
										</a:solidFill>
									</wps:spPr>
								</wps:wsp>
							</a:graphicData>
						</a:graphic>
					</wp:anchor>
				</w:drawing>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected a resolved paragraph run")
	}
	shape := doc.Paragraphs[0].Runs[0].Shape
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	// Shade 0.5 on FF0000 => 800000; Tint 0 is handled by HasTint=true but factor 0 => white,
	// however the DrawingML tint formula is: channel * factor + 255 * (1 - factor).
	// With factor 0: all channels become 255. But both shade and tint are applied in order.
	// ApplyColorTransforms order: shade first, then tint.
	// After shade 0.5 on FF0000 => 800000. Tint 0 is a no-op..
	if shape.FillColor != "800000" {
		t.Fatalf("fill color = %q, want 800000", shape.FillColor)
	}
}

func TestResolveDrawingMLGroupShape(t *testing.T) {
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
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:drawing>
					<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
						<wp:simplePos x="0" y="0"/>
						<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
						<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
						<wp:extent cx="1828800" cy="914400"/>
						<wp:wrapNone/>
						<wp:docPr id="1" name="Group 1"/>
						<a:graphic>
							<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
								<a:grpSp>
									<a:nvGrpSpPr>
										<a:cNvPr id="2" name="GroupShape"/>
										<a:cNvGrpSpPr/>
									</a:nvGrpSpPr>
									<a:grpSpPr>
										<a:xfrm>
											<a:off x="0" y="0"/>
											<a:ext cx="1828800" cy="914400"/>
											<a:chOff x="0" y="0"/>
											<a:chExt cx="1828800" cy="914400"/>
										</a:xfrm>
									</a:grpSpPr>
									<a:sp>
										<a:nvSpPr>
											<a:cNvPr id="3" name="Rect"/>
											<a:cNvSpPr/>
										</a:nvSpPr>
										<a:spPr>
											<a:xfrm>
												<a:off x="0" y="0"/>
												<a:ext cx="914400" cy="914400"/>
											</a:xfrm>
											<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
											<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
										</a:spPr>
									</a:sp>
									<a:sp>
										<a:nvSpPr>
											<a:cNvPr id="4" name="Ellipse"/>
											<a:cNvSpPr/>
										</a:nvSpPr>
										<a:spPr>
											<a:xfrm>
												<a:off x="914400" y="0"/>
												<a:ext cx="914400" cy="914400"/>
											</a:xfrm>
											<a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom>
											<a:solidFill><a:srgbClr val="00FF00"/></a:solidFill>
										</a:spPr>
									</a:sp>
								</a:grpSp>
							</a:graphicData>
						</a:graphic>
					</wp:anchor>
				</w:drawing>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	shape := doc.Paragraphs[0].Runs[0].Shape
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	if shape.Kind != "group" {
		t.Fatalf("shape.Kind = %q, want group", shape.Kind)
	}
	if len(shape.Children) != 2 {
		t.Fatalf("len(Children) = %d, want 2", len(shape.Children))
	}
	if shape.Children[0].Kind != "rect" {
		t.Fatalf("child[0].Kind = %q, want rect", shape.Children[0].Kind)
	}
	if shape.Children[0].FillColor != "FF0000" {
		t.Fatalf("child[0].FillColor = %q, want FF0000", shape.Children[0].FillColor)
	}
	if shape.Children[0].X != 0 || shape.Children[0].Y != 0 {
		t.Fatalf("child[0].X/Y = %d/%d, want 0/0", shape.Children[0].X, shape.Children[0].Y)
	}
	if shape.Children[0].Width != 914400 || shape.Children[0].Height != 914400 {
		t.Fatalf("child[0].Width/Height = %d/%d, want 914400/914400", shape.Children[0].Width, shape.Children[0].Height)
	}
	if shape.Children[1].Kind != "rect" {
		t.Fatalf("child[1].Kind = %q, want rect", shape.Children[1].Kind)
	}
	if shape.Children[1].FillColor != "00FF00" {
		t.Fatalf("child[1].FillColor = %q, want 00FF00", shape.Children[1].FillColor)
	}
	if shape.Children[1].X != 914400 || shape.Children[1].Y != 0 {
		t.Fatalf("child[1].X/Y = %d/%d, want 914400/0", shape.Children[1].X, shape.Children[1].Y)
	}
}

func TestResolveDrawingMLTextBox(t *testing.T) {
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
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:drawing>
					<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
						<wp:simplePos x="0" y="0"/>
						<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
						<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
						<wp:extent cx="914400" cy="457200"/>
						<wp:wrapNone/>
						<wp:docPr id="1" name="TextShape"/>
						<a:graphic>
							<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
								<a:sp>
									<a:nvSpPr>
										<a:cNvPr id="2" name="Shape"/>
										<a:cNvSpPr/>
									</a:nvSpPr>
									<a:spPr>
										<a:xfrm>
											<a:off x="0" y="0"/>
											<a:ext cx="914400" cy="457200"/>
										</a:xfrm>
										<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
										<a:solidFill><a:srgbClr val="CFE2F3"/></a:solidFill>
									</a:spPr>
									<a:txBody>
										<a:bodyPr/>
										<a:p>
											<a:r><a:t>Hello</a:t></a:r>
											<a:r><a:rPr lang="en-US"/><a:t> World</a:t></a:r>
										</a:p>
										<a:p>
											<a:r><a:t>Second para</a:t></a:r>
										</a:p>
									</a:txBody>
								</a:sp>
							</a:graphicData>
						</a:graphic>
					</wp:anchor>
				</w:drawing>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	tb := doc.Paragraphs[0].Runs[0].TextBox
	if tb == nil {
		t.Fatal("expected resolved text box")
	}
	if len(tb.Blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(tb.Blocks))
	}
	if tb.Blocks[0].Paragraph == nil || len(tb.Blocks[0].Paragraph.Runs) != 2 {
		t.Fatalf("expected paragraph with 2 runs, got %+v", tb.Blocks[0].Paragraph)
	}
	if tb.Blocks[0].Paragraph.Runs[0].Text != "Hello" {
		t.Fatalf("run0 text = %q, want Hello", tb.Blocks[0].Paragraph.Runs[0].Text)
	}
	if tb.Blocks[0].Paragraph.Runs[1].Text != " World" {
		t.Fatalf("run1 text = %q, want ' World'", tb.Blocks[0].Paragraph.Runs[1].Text)
	}
	if tb.Blocks[0].Paragraph.Runs[1].Format.Lang != "en-US" {
		t.Fatalf("run1 lang = %q, want en-US", tb.Blocks[0].Paragraph.Runs[1].Format.Lang)
	}
	if tb.Blocks[1].Paragraph == nil || tb.Blocks[1].Paragraph.Runs[0].Text != "Second para" {
		t.Fatalf("second paragraph text = %q, want Second para", tb.Blocks[1].Paragraph.Runs[0].Text)
	}
	if tb.FillColor != "CFE2F3" {
		t.Fatalf("fill color = %q, want CFE2F3", tb.FillColor)
	}
}

func TestResolveDrawingMLTextBoxStyleFontRef(t *testing.T) {
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
		"word/document.xml": `<w:document
		xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:drawing>
					<wp:inline>
						<wp:extent cx="914400" cy="457200"/>
						<wp:docPr id="1" name="TextShape"/>
						<a:graphic>
							<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
								<a:sp>
									<a:nvSpPr>
										<a:cNvPr id="2" name="Shape"/>
										<a:cNvSpPr/>
									</a:nvSpPr>
									<a:spPr>
										<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
									</a:spPr>
									<a:style>
										<a:fontRef idx="major"><a:schemeClr val="tx1"/></a:fontRef>
									</a:style>
									<a:txBody>
										<a:bodyPr/>
										<a:p>
											<a:r><a:t>Themed</a:t></a:r>
											<a:r>
												<a:rPr><a:latin typeface="Courier New"/></a:rPr>
												<a:t> Explicit</a:t>
											</a:r>
										</a:p>
									</a:txBody>
								</a:sp>
							</a:graphicData>
						</a:graphic>
					</wp:inline>
				</w:drawing>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/theme/theme1.xml": `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office">
	<a:themeElements>
		<a:clrScheme name="Office">
			<a:dk1><a:srgbClr val="000000"/></a:dk1>
			<a:lt1><a:srgbClr val="FFFFFF"/></a:lt1>
		</a:clrScheme>
		<a:fontScheme name="Office Fonts">
			<a:majorFont>
				<a:latin typeface="Arial"/>
			</a:majorFont>
			<a:minorFont>
				<a:latin typeface="Calibri"/>
			</a:minorFont>
		</a:fontScheme>
	</a:themeElements>
</a:theme>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	tb := doc.Paragraphs[0].Runs[0].TextBox
	if tb == nil || len(tb.Blocks) != 1 || tb.Blocks[0].Paragraph == nil {
		t.Fatalf("expected resolved text box paragraph, got %+v", tb)
	}
	runs := tb.Blocks[0].Paragraph.Runs
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
	if runs[0].Format.FontASCII != "Arial" || runs[0].Format.FontHAnsi != "Arial" {
		t.Fatalf("themed run font = %q/%q, want Arial/Arial", runs[0].Format.FontASCII, runs[0].Format.FontHAnsi)
	}
	if runs[1].Format.FontASCII != "Courier New" {
		t.Fatalf("explicit run font = %q, want Courier New", runs[1].Format.FontASCII)
	}
}

func TestResolveDrawingMLTable(t *testing.T) {
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
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
	<w:body>
		<w:p>
			<w:r>
				<w:drawing>
					<wp:inline>
						<wp:extent cx="1828800" cy="914400"/>
						<wp:docPr id="1" name="Table"/>
						<a:graphic>
							<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
								<a:tbl>
									<a:tblPr/>
									<a:tblGrid>
										<a:gridCol w="914400"/>
										<a:gridCol w="914400"/>
									</a:tblGrid>
									<a:tr>
										<a:tc>
											<a:txBody>
												<a:bodyPr/>
												<a:p><a:r><a:t>A1</a:t></a:r></a:p>
											</a:txBody>
										</a:tc>
										<a:tc>
											<a:txBody>
												<a:bodyPr/>
												<a:p><a:r><a:t>B1</a:t></a:r></a:p>
											</a:txBody>
										</a:tc>
									</a:tr>
									<a:tr>
										<a:tc>
											<a:txBody>
												<a:bodyPr/>
												<a:p><a:r><a:t>A2</a:t></a:r></a:p>
											</a:txBody>
										</a:tc>
										<a:tc>
											<a:txBody>
												<a:bodyPr/>
												<a:p><a:r><a:t>B2</a:t></a:r></a:p>
											</a:txBody>
										</a:tc>
									</a:tr>
								</a:tbl>
							</a:graphicData>
						</a:graphic>
					</wp:inline>
				</w:drawing>
			</w:r>
		</w:p>
	</w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	tb := doc.Paragraphs[0].Runs[0].TextBox
	if tb == nil {
		t.Fatal("expected resolved text box wrapping table")
	}
	if len(tb.Blocks) != 1 || tb.Blocks[0].Table == nil {
		t.Fatalf("expected 1 table block, got %+v", tb.Blocks)
	}
	table := tb.Blocks[0].Table
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if len(table.Rows[0].Cells) != 2 {
		t.Fatalf("cols = %d, want 2", len(table.Rows[0].Cells))
	}
	if len(table.Rows[0].Cells[0].Paragraphs) != 1 || table.Rows[0].Cells[0].Paragraphs[0].Runs[0].Text != "A1" {
		t.Fatalf("A1 = %q", table.Rows[0].Cells[0].Paragraphs[0].Runs[0].Text)
	}
	if table.Rows[1].Cells[1].Paragraphs[0].Runs[0].Text != "B2" {
		t.Fatalf("B2 = %q", table.Rows[1].Cells[1].Paragraphs[0].Runs[0].Text)
	}
}

func TestResolveDrawingMLPictureBlipFill(t *testing.T) {
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54,
		0x78, 0x9c, 0x62, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01,
		0xe2, 0x21, 0xbc, 0x33,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	data := buildPackageWithBinary(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
	<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
	<Default Extension="xml" ContentType="application/xml"/>
	<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
	<Override PartName="/word/media/image1.png" ContentType="image/png"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<w:document
			xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
			xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
			xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
			xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<w:body>
			<w:p>
				<w:r>
					<w:drawing>
						<wp:inline>
							<wp:extent cx="1828800" cy="914400"/>
							<wp:docPr id="1" name="Picture 1"/>
							<a:graphic>
								<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
									<a:pic>
										<a:nvPicPr>
											<a:cNvPr id="2" name="Picture"/>
											<a:cNvPicPr/>
										</a:nvPicPr>
										<a:blipFill>
											<a:blip r:embed="rId5"/>
											<a:srcRect l="10000" t="5000" r="20000" b="15000"/>
										</a:blipFill>
										<a:spPr>
											<a:xfrm rot="5400000" flipH="1" flipV="1">
												<a:off x="0" y="0"/>
												<a:ext cx="1828800" cy="914400"/>
											</a:xfrm>
										</a:spPr>
									</a:pic>
								</a:graphicData>
							</a:graphic>
						</wp:inline>
					</w:drawing>
				</w:r>
			</w:p>
		</w:body>
	</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
</Relationships>`,
	}, map[string][]byte{"word/media/image1.png": minimalPNG})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	img := doc.Paragraphs[0].Runs[0].Image
	if img == nil {
		t.Fatal("expected resolved image")
	}
	if math.Abs(img.CropLeft-0.10) > 1e-9 || math.Abs(img.CropTop-0.05) > 1e-9 ||
		math.Abs(img.CropRight-0.20) > 1e-9 || math.Abs(img.CropBottom-0.15) > 1e-9 {
		t.Fatalf("unexpected crop = left %.3f top %.3f right %.3f bottom %.3f", img.CropLeft, img.CropTop, img.CropRight, img.CropBottom)
	}
	if math.Abs(img.RotationDeg-90.0) > 1e-9 {
		t.Fatalf("rotation = %.2f, want 90", img.RotationDeg)
	}
	if !img.FlipH || !img.FlipV {
		t.Fatalf("flip flags = H:%v V:%v, want true/true", img.FlipH, img.FlipV)
	}
}

func TestResolveDrawingMLGroupShapeWithText(t *testing.T) {
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
			xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<w:body>
			<w:p>
				<w:r>
					<w:drawing>
						<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
							<wp:simplePos x="0" y="0"/>
							<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
							<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
							<wp:extent cx="1828800" cy="914400"/>
							<wp:wrapNone/>
							<wp:docPr id="1" name="Group 1"/>
							<a:graphic>
								<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
									<a:grpSp>
										<a:nvGrpSpPr>
											<a:cNvPr id="2" name="GroupShape"/>
											<a:cNvGrpSpPr/>
										</a:nvGrpSpPr>
										<a:grpSpPr>
											<a:xfrm>
												<a:off x="0" y="0"/>
												<a:ext cx="1828800" cy="914400"/>
												<a:chOff x="0" y="0"/>
												<a:chExt cx="1828800" cy="914400"/>
											</a:xfrm>
										</a:grpSpPr>
										<a:sp>
											<a:nvSpPr>
												<a:cNvPr id="3" name="ShapeWithText"/>
												<a:cNvSpPr/>
											</a:nvSpPr>
											<a:spPr>
												<a:xfrm>
													<a:off x="0" y="0"/>
													<a:ext cx="914400" cy="914400"/>
												</a:xfrm>
												<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
												<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
											</a:spPr>
											<a:txBody>
												<a:bodyPr/>
												<a:p><a:r><a:t>Inside group</a:t></a:r></a:p>
											</a:txBody>
										</a:sp>
									</a:grpSp>
								</a:graphicData>
							</a:graphic>
						</wp:anchor>
					</w:drawing>
				</w:r>
			</w:p>
		</w:body>
	</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	shape := doc.Paragraphs[0].Runs[0].Shape
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	if shape.Kind != "group" {
		t.Fatalf("shape.Kind = %q, want group", shape.Kind)
	}
	if len(shape.Children) != 1 {
		t.Fatalf("len(Children) = %d, want 1", len(shape.Children))
	}
	child := shape.Children[0]
	if child.TextBox == nil {
		t.Fatal("expected child shape to have TextBox")
	}
	if len(child.TextBox.Blocks) != 1 || child.TextBox.Blocks[0].Paragraph == nil {
		t.Fatalf("expected 1 paragraph block, got %+v", child.TextBox.Blocks)
	}
	if child.TextBox.Blocks[0].Paragraph.Runs[0].Text != "Inside group" {
		t.Fatalf("text = %q, want Inside group", child.TextBox.Blocks[0].Paragraph.Runs[0].Text)
	}
}

func TestResolveDrawingMLGroupShapeWithChOffChExt(t *testing.T) {
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
			xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<w:body>
			<w:p>
				<w:r>
					<w:drawing>
						<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
							<wp:simplePos x="0" y="0"/>
							<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
							<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
							<wp:extent cx="2000000" cy="1000000"/>
							<wp:wrapNone/>
							<wp:docPr id="1" name="Group 1"/>
							<a:graphic>
								<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
									<a:grpSp>
										<a:nvGrpSpPr>
											<a:cNvPr id="2" name="GroupShape"/>
											<a:cNvGrpSpPr/>
										</a:nvGrpSpPr>
										<a:grpSpPr>
											<a:xfrm>
												<a:off x="0" y="0"/>
												<a:ext cx="2000000" cy="1000000"/>
												<a:chOff x="0" y="0"/>
												<a:chExt cx="4000000" cy="2000000"/>
											</a:xfrm>
										</a:grpSpPr>
										<a:sp>
											<a:nvSpPr>
												<a:cNvPr id="3" name="ChildShape"/>
												<a:cNvSpPr/>
											</a:nvSpPr>
											<a:spPr>
												<a:xfrm>
													<a:off x="1000000" y="500000"/>
													<a:ext cx="1000000" cy="500000"/>
												</a:xfrm>
												<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
												<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
											</a:spPr>
										</a:sp>
									</a:grpSp>
								</a:graphicData>
							</a:graphic>
						</wp:anchor>
					</w:drawing>
				</w:r>
			</w:p>
		</w:body>
	</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	shape := doc.Paragraphs[0].Runs[0].Shape
	if shape == nil {
		t.Fatal("expected resolved shape")
	}
	if shape.Kind != "group" {
		t.Fatalf("shape.Kind = %q, want group", shape.Kind)
	}
	// Top-level group gets its position from the anchor, not xfrm.off
	if shape.Width != 2000000 || shape.Height != 1000000 {
		t.Fatalf("group size = %dx%d, want 2000000x1000000", shape.Width, shape.Height)
	}
	if len(shape.Children) != 1 {
		t.Fatalf("len(Children) = %d, want 1", len(shape.Children))
	}
	child := shape.Children[0]
	// Group scale = ext / chExt = 2000000/4000000 = 0.5, 1000000/2000000 = 0.5
	// Child xfrm: off=(1000000,500000), ext=(1000000,500000)
	// Document coords: group.off + (child.off - chOff) * scale
	// group.off=(0,0), chOff=(0,0):
	//   X = 0 + 1000000 * 0.5 = 500000
	//   Y = 0 + 500000 * 0.5 = 250000
	//   Width = 1000000 * 0.5 = 500000
	//   Height = 500000 * 0.5 = 250000
	if child.X != 500000 {
		t.Fatalf("child.X = %d, want 500000", child.X)
	}
	if child.Y != 250000 {
		t.Fatalf("child.Y = %d, want 250000", child.Y)
	}
	if child.Width != 500000 {
		t.Fatalf("child.Width = %d, want 500000", child.Width)
	}
	if child.Height != 250000 {
		t.Fatalf("child.Height = %d, want 250000", child.Height)
	}
}

func TestResolveDrawingMLGroupShapeWithChOffOffset(t *testing.T) {
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
			xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<w:body>
			<w:p>
				<w:r>
					<w:drawing>
						<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
							<wp:simplePos x="0" y="0"/>
							<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
							<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
							<wp:extent cx="1000000" cy="1000000"/>
							<wp:wrapNone/>
							<wp:docPr id="1" name="Group 1"/>
							<a:graphic>
								<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
									<a:grpSp>
										<a:nvGrpSpPr>
											<a:cNvPr id="2" name="GroupShape"/>
											<a:cNvGrpSpPr/>
										</a:nvGrpSpPr>
										<a:grpSpPr>
											<a:xfrm>
												<a:off x="0" y="0"/>
												<a:ext cx="1000000" cy="1000000"/>
												<a:chOff x="500000" y="500000"/>
												<a:chExt cx="1000000" cy="1000000"/>
											</a:xfrm>
										</a:grpSpPr>
										<a:sp>
											<a:nvSpPr>
												<a:cNvPr id="3" name="ChildShape"/>
												<a:cNvSpPr/>
											</a:nvSpPr>
											<a:spPr>
												<a:xfrm>
													<a:off x="500000" y="500000"/>
													<a:ext cx="500000" cy="500000"/>
												</a:xfrm>
												<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
												<a:solidFill><a:srgbClr val="00FF00"/></a:solidFill>
											</a:spPr>
										</a:sp>
									</a:grpSp>
								</a:graphicData>
							</a:graphic>
						</wp:anchor>
					</w:drawing>
				</w:r>
			</w:p>
		</w:body>
	</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	shape := doc.Paragraphs[0].Runs[0].Shape
	if shape == nil || shape.Kind != "group" {
		t.Fatal("expected resolved group shape")
	}
	if len(shape.Children) != 1 {
		t.Fatalf("len(Children) = %d, want 1", len(shape.Children))
	}
	child := shape.Children[0]
	// scale = 1.0, chOff=(500000,500000)
	// Child xfrm: off=(500000,500000), ext=(500000,500000)
	// Document coords: 0 + (500000 - 500000) * 1.0 = 0
	// Size: 500000 * 1.0 = 500000
	if child.X != 0 {
		t.Fatalf("child.X = %d, want 0", child.X)
	}
	if child.Y != 0 {
		t.Fatalf("child.Y = %d, want 0", child.Y)
	}
	if child.Width != 500000 {
		t.Fatalf("child.Width = %d, want 500000", child.Width)
	}
	if child.Height != 500000 {
		t.Fatalf("child.Height = %d, want 500000", child.Height)
	}
}

func TestResolveDrawingMLNestedGroupShapeTransform(t *testing.T) {
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
			xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<w:body>
			<w:p>
				<w:r>
					<w:drawing>
						<wp:anchor simplePos="0" relativeHeight="1" behindDoc="0" locked="0" layoutInCell="1" allowOverlap="1">
							<wp:simplePos x="0" y="0"/>
							<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>
							<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>
							<wp:extent cx="1000000" cy="1000000"/>
							<wp:wrapNone/>
							<wp:docPr id="1" name="Group 1"/>
							<a:graphic>
								<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/main">
									<a:grpSp>
										<a:nvGrpSpPr>
											<a:cNvPr id="2" name="OuterGroup"/>
											<a:cNvGrpSpPr/>
										</a:nvGrpSpPr>
										<a:grpSpPr>
											<a:xfrm>
												<a:off x="0" y="0"/>
												<a:ext cx="1000000" cy="1000000"/>
												<a:chOff x="0" y="0"/>
												<a:chExt cx="2000000" cy="2000000"/>
											</a:xfrm>
										</a:grpSpPr>
										<a:grpSp>
											<a:nvGrpSpPr>
												<a:cNvPr id="3" name="InnerGroup"/>
												<a:cNvGrpSpPr/>
											</a:nvGrpSpPr>
											<a:grpSpPr>
												<a:xfrm>
													<a:off x="500000" y="500000"/>
													<a:ext cx="1000000" cy="1000000"/>
													<a:chOff x="0" y="0"/>
													<a:chExt cx="1000000" cy="1000000"/>
												</a:xfrm>
											</a:grpSpPr>
											<a:sp>
												<a:nvSpPr>
													<a:cNvPr id="4" name="DeepChild"/>
													<a:cNvSpPr/>
												</a:nvSpPr>
												<a:spPr>
													<a:xfrm>
														<a:off x="100000" y="100000"/>
														<a:ext cx="200000" cy="200000"/>
													</a:xfrm>
													<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
													<a:solidFill><a:srgbClr val="0000FF"/></a:solidFill>
												</a:spPr>
											</a:sp>
										</a:grpSp>
									</a:grpSp>
								</a:graphicData>
							</a:graphic>
						</wp:anchor>
					</w:drawing>
				</w:r>
			</w:p>
		</w:body>
	</w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	doc := resolveDoc(t, data)
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	outer := doc.Paragraphs[0].Runs[0].Shape
	if outer == nil || outer.Kind != "group" {
		t.Fatal("expected resolved outer group shape")
	}
	if len(outer.Children) != 1 {
		t.Fatalf("len(outer.Children) = %d, want 1", len(outer.Children))
	}
	inner := outer.Children[0]
	if inner.Kind != "group" {
		t.Fatalf("inner.Kind = %q, want group", inner.Kind)
	}
	// Outer scale = 1000000/2000000 = 0.5
	// Inner xfrm: off=(500000,500000), ext=(1000000,1000000)
	// Inner document coords: 0 + 500000*0.5 = 250000, size = 1000000*0.5 = 500000
	if inner.X != 250000 {
		t.Fatalf("inner.X = %d, want 250000", inner.X)
	}
	if inner.Y != 250000 {
		t.Fatalf("inner.Y = %d, want 250000", inner.Y)
	}
	if inner.Width != 500000 {
		t.Fatalf("inner.Width = %d, want 500000", inner.Width)
	}
	if inner.Height != 500000 {
		t.Fatalf("inner.Height = %d, want 500000", inner.Height)
	}

	if len(inner.Children) != 1 {
		t.Fatalf("len(inner.Children) = %d, want 1", len(inner.Children))
	}
	leaf := inner.Children[0]
	// Inner scale = 1.0, so combined scale = 0.5 * 1.0 = 0.5
	// Leaf xfrm: off=(100000,100000), ext=(200000,200000)
	// Leaf document coords: innerDocX + (leaf.off - inner.chOff) * combinedScale
	//   = 250000 + (100000 - 0) * 0.5 = 300000
	//   size = 200000 * 0.5 = 100000
	if leaf.X != 300000 {
		t.Fatalf("leaf.X = %d, want 300000", leaf.X)
	}
	if leaf.Y != 300000 {
		t.Fatalf("leaf.Y = %d, want 300000", leaf.Y)
	}
	if leaf.Width != 100000 {
		t.Fatalf("leaf.Width = %d, want 100000", leaf.Width)
	}
	if leaf.Height != 100000 {
		t.Fatalf("leaf.Height = %d, want 100000", leaf.Height)
	}
}
