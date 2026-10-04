package parts_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/diag"
	"office/pkg/ooxml/opc"
	"office/pkg/sheet/parsers/ooxml/parts"
)

func TestXlsxCorpusLoad(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "xlsx")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	var count int
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".xlsx") {
			continue
		}
		// Excel drops an owner lock file next to an open workbook. It is not a
		// ZIP package (and may be locked by the running Office process), so a
		// fixture left open must not fail the corpus run.
		if strings.HasPrefix(entry.Name(), "~$") {
			continue
		}
		count++
		path := filepath.Join(fixturesDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			pkg, err := opc.Open(path)
			if err != nil {
				if opc.IsEncryptedPackageError(err) {
					t.Skip("encrypted package requires password")
				}
				t.Fatalf("open package: %v", err)
			}
			xlPkg, err := parts.LoadPackage(pkg)
			if err != nil {
				t.Fatalf("load package: %v", err)
			}
			if xlPkg.Workbook == nil {
				t.Fatal("workbook is nil")
			}
			if len(xlPkg.Workbook.Sheets) == 0 {
				t.Fatal("no sheets in workbook")
			}
			if len(xlPkg.Worksheets) == 0 {
				t.Fatal("no worksheets loaded")
			}
			for _, ws := range xlPkg.Worksheets {
				if ws.Sheet == nil {
					t.Errorf("worksheet %q (relId=%s) not loaded", ws.Name, ws.RelID)
				}
			}
		})
	}

	if count == 0 {
		t.Fatal("no xlsx fixtures found")
	}
}

func TestLoadPackageReportsUnsupportedFeatureDiagnostics(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/drawings/drawing1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Sheet1" sheetId="1" r:id="rIdSheet1"/>
  </sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSheet1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheetData>
    <row r="1"><c r="A1" t="str"><v>ok</v></c></row>
  </sheetData>
  <drawing r:id="rIdDrawing1"/>
  <legacyDrawing r:id="rIdLegacy1"/>
  <oleObjects>
    <oleObject r:id="rIdOle1" progId="Excel.Sheet.12"/>
  </oleObjects>
</worksheet>`,
		"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdDrawing1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>
  <Relationship Id="rIdLegacy1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/vmlDrawing" Target="../drawings/vmlDrawing1.vml"/>
</Relationships>`,
		"xl/drawings/drawing1.xml": `<xdr:wsDr
  xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"
  xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
  xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <xdr:oneCellAnchor>
    <xdr:from><xdr:col>0</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>0</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>
    <xdr:ext cx="914400" cy="914400"/>
    <xdr:graphicFrame>
      <xdr:nvGraphicFramePr><xdr:cNvPr id="1" name="Sales Chart"/></xdr:nvGraphicFramePr>
      <a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rIdChart1"/></a:graphicData></a:graphic>
    </xdr:graphicFrame>
  </xdr:oneCellAnchor>
  <xdr:twoCellAnchor>
    <xdr:from><xdr:col>1</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>1</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>
    <xdr:to><xdr:col>2</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>3</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:to>
    <xdr:graphicFrame>
      <xdr:nvGraphicFramePr><xdr:cNvPr id="2" name="Process Diagram"/></xdr:nvGraphicFramePr>
      <a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/diagram"/></a:graphic>
    </xdr:graphicFrame>
  </xdr:twoCellAnchor>
</xdr:wsDr>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	for _, code := range []string{
		"sheet/unsupported-smartart",
		"sheet/unsupported-vml",
		"sheet/unsupported-ole",
	} {
		if !hasDiagnosticCode(xlPkg.Diagnostics, code) {
			t.Fatalf("missing diagnostic code %q in %+v", code, xlPkg.Diagnostics)
		}
	}
}

func TestLoadPackageLegacyVMLNotes(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="vml" ContentType="application/vnd.openxmlformats-officedocument.vmlDrawing"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/comments1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.comments+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rIdSheet1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSheet1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheetData><row r="1"><c r="A1" t="str"><v>ok</v></c></row></sheetData>
  <legacyDrawing r:id="rIdLegacy1"/>
</worksheet>`,
		"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdLegacy1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/vmlDrawing" Target="../drawings/vmlDrawing1.vml"/>
  <Relationship Id="rIdComment1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="../comments1.xml"/>
</Relationships>`,
		"xl/comments1.xml": `<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><authors><author/></authors><commentList><comment ref="C25" authorId="0"><text><t>Legacy note</t></text></comment></commentList></comments>`,
		"xl/drawings/vmlDrawing1.vml": `<?xml version="1.0" encoding="UTF-8"?>
<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:x="urn:schemas-microsoft-com:office:excel">
  <v:shape id="_x0000_s1025" type="#_x0000_t202">
    <x:ClientData ObjectType="Note">
      <x:Anchor>3, 15, 25, 10, 7, 15, 27, 4</x:Anchor>
      <x:Row>24</x:Row>
      <x:Column>2</x:Column>
    </x:ClientData>
  </v:shape>
</xml>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if hasDiagnosticCode(xlPkg.Diagnostics, "sheet/unsupported-vml") {
		t.Fatalf("did not expect unsupported-vml diagnostic when legacy notes are parsed: %+v", xlPkg.Diagnostics)
	}
	if len(xlPkg.Worksheets) != 1 || xlPkg.Worksheets[0].LegacyNotes["C25"].Ref != "C25" {
		t.Fatalf("expected legacy note metadata for C25, got %+v", xlPkg.Worksheets[0].LegacyNotes)
	}
}

func TestLoadPackageParsesProtectionMetadata(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <workbookProtection lockStructure="1" lockWindows="1" workbookPassword="ABCD" algorithmName="SHA-512" hashValue="deadbeef" saltValue="c2FsdA==" spinCount="100000"/>
  <sheets><sheet name="Protected" sheetId="1" r:id="rIdSheet1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSheet1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetProtection sheet="1" objects="1" scenarios="1" formatCells="0" formatColumns="1" insertRows="1" sort="1" autoFilter="1" selectLockedCells="1" selectUnlockedCells="1" password="C0DE" algorithmName="SHA-512" hashValue="beadfeed" saltValue="c2hlZXQ=" spinCount="25000"/>
  <sheetData><row r="1"><c r="A1" t="str"><v>ok</v></c></row></sheetData>
</worksheet>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if xlPkg.Workbook == nil || xlPkg.Workbook.Protection == nil {
		t.Fatal("expected workbook protection metadata")
	}
	wp := xlPkg.Workbook.Protection
	if !wp.LockStructure || !wp.LockWindows || wp.WorkbookPassword != "ABCD" || wp.AlgorithmName != "SHA-512" || wp.SpinCount != 100000 {
		t.Fatalf("unexpected workbook protection: %+v", wp)
	}
	if len(xlPkg.Worksheets) != 1 || xlPkg.Worksheets[0].Sheet == nil || xlPkg.Worksheets[0].Sheet.SheetProtection == nil {
		t.Fatalf("expected worksheet protection metadata, got %+v", xlPkg.Worksheets)
	}
	sp := xlPkg.Worksheets[0].Sheet.SheetProtection
	if !sp.Sheet || !sp.Objects || !sp.Scenarios || !sp.FormatColumns || !sp.InsertRows || !sp.Sort || !sp.AutoFilter || !sp.SelectLockedCells || !sp.SelectUnlockedCells {
		t.Fatalf("expected worksheet protection flags, got %+v", sp)
	}
	if sp.Password != "C0DE" || sp.AlgorithmName != "SHA-512" || sp.HashValue != "beadfeed" || sp.SpinCount != 25000 {
		t.Fatalf("unexpected worksheet protection metadata: %+v", sp)
	}
}

func TestLoadPackageParsesThemeColorTransforms(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rIdSheet1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSheet1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rIdTheme1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData><row r="1"><c r="A1" t="str"><v>ok</v></c></row></sheetData>
</worksheet>`,
		"xl/theme/theme1.xml": `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">
  <a:themeElements>
    <a:clrScheme name="Custom">
      <a:lt1><a:srgbClr val="808080"><a:tint val="50000"/></a:srgbClr></a:lt1>
      <a:dk1><a:sysClr val="windowText" lastClr="404040"><a:shade val="50000"/></a:sysClr></a:dk1>
    </a:clrScheme>
  </a:themeElements>
</a:theme>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if xlPkg.Theme == nil {
		t.Fatal("expected theme part")
	}
	if xlPkg.Theme.Elements.ClrScheme.LT1 == nil || xlPkg.Theme.Elements.ClrScheme.LT1.SrgbClr == nil || xlPkg.Theme.Elements.ClrScheme.LT1.SrgbClr.Tint == nil {
		t.Fatalf("expected lt1 srgb tint transform, got %+v", xlPkg.Theme.Elements.ClrScheme.LT1)
	}
	if got := xlPkg.Theme.Elements.ClrScheme.LT1.SrgbClr.Tint.Val; got != 50000 {
		t.Fatalf("lt1 tint = %d, want %d", got, 50000)
	}
	if xlPkg.Theme.Elements.ClrScheme.DK1 == nil || xlPkg.Theme.Elements.ClrScheme.DK1.SysClr == nil || xlPkg.Theme.Elements.ClrScheme.DK1.SysClr.Shade == nil {
		t.Fatalf("expected dk1 sysClr shade transform, got %+v", xlPkg.Theme.Elements.ClrScheme.DK1)
	}
	if got := xlPkg.Theme.Elements.ClrScheme.DK1.SysClr.Shade.Val; got != 50000 {
		t.Fatalf("dk1 shade = %d, want %d", got, 50000)
	}
}

func TestLoadPackageParsesIndexedColors(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rIdSheet1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSheet1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData><row r="1"><c r="A1" t="str"><v>ok</v></c></row></sheetData>
</worksheet>`,
		"xl/styles.xml": `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <colors>
    <indexedColors>
      <rgbColor rgb="FF102030"/>
      <rgbColor rgb="FF405060"/>
    </indexedColors>
  </colors>
</styleSheet>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if xlPkg.Styles == nil || xlPkg.Styles.Colors == nil || xlPkg.Styles.Colors.IndexedColors == nil {
		t.Fatal("expected stylesheet indexed colors")
	}
	if got := len(xlPkg.Styles.Colors.IndexedColors.RGBColors); got != 2 {
		t.Fatalf("indexed color count = %d, want 2", got)
	}
	if got := xlPkg.Styles.Colors.IndexedColors.RGBColors[0].RGB; got != "FF102030" {
		t.Fatalf("indexed color 0 = %q, want %q", got, "FF102030")
	}
	if got := xlPkg.Styles.Colors.IndexedColors.RGBColors[1].RGB; got != "FF405060" {
		t.Fatalf("indexed color 1 = %q, want %q", got, "FF405060")
	}
}

func TestLoadPackageParsesConnectionsMetadata(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/connections.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.connections+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rIdSheet1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSheet1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rIdConn1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/connections" Target="connections.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="str"><v>ok</v></c></row></sheetData></worksheet>`,
		"xl/connections.xml": `<connections xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <connection id="1" name="Sales DB" type="1" description="Warehouse import" refreshOnLoad="1" background="1" saveData="1" sourceFile="sales.odc">
    <dbPr connection="Provider=SQLOLEDB;Data Source=server;Initial Catalog=sales" command="SELECT * FROM Sales" commandType="2"/>
  </connection>
  <connection id="2" name="Rates Feed" type="5" refreshOnLoad="1">
		<webPr url="https://example.com/rates.csv" post="region=us" htmlTables="1" sourceData="1" firstRow="1" textDates="1"/>
  </connection>
</connections>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if xlPkg.Connections == nil || len(xlPkg.Connections.Connections) != 2 {
		t.Fatalf("expected parsed connections, got %+v", xlPkg.Connections)
	}
	first := xlPkg.Connections.Connections[0]
	if first.Name != "Sales DB" || first.SourceFile != "sales.odc" || first.DBPr == nil || first.DBPr.Command != "SELECT * FROM Sales" {
		t.Fatalf("unexpected first connection: %+v", first)
	}
	second := xlPkg.Connections.Connections[1]
	if second.Name != "Rates Feed" || second.WebPr == nil || second.WebPr.URL != "https://example.com/rates.csv" || second.WebPr.Post != "region=us" || !second.WebPr.HtmlTables || !second.WebPr.SourceData || !second.WebPr.FirstRow || !second.WebPr.TextDates {
		t.Fatalf("unexpected second connection: %+v", second)
	}
}

func TestLoadPackageParsesExternalLinkMetadata(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/externalLinks/externalLink1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.externalLink+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rIdSheet1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSheet1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rIdExt1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/externalLink" Target="externalLinks/externalLink1.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="str"><v>ok</v></c></row></sheetData></worksheet>`,
		"xl/externalLinks/externalLink1.xml": `<externalLink xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <externalBook r:id="rId1">
    <sheetNames><sheetName val="Prices"/></sheetNames>
    <definedNames><definedName name="ExtRange" refersTo="Prices!$A$1:$B$2"/></definedNames>
		<sheetDataSet>
			<sheetData sheetId="0">
				<row r="1">
					<cell r="A1" t="n"><v>7</v></cell>
					<cell r="B1" t="str"><v>North</v></cell>
				</row>
			</sheetData>
		</sheetDataSet>
  </externalBook>
</externalLink>`,
		"xl/externalLinks/_rels/externalLink1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/externalLinkPath" Target="file:///C:/workbooks/source.xlsx" TargetMode="External"/>
</Relationships>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(xlPkg.ExternalLinks) != 1 {
		t.Fatalf("expected 1 external link entry, got %+v", xlPkg.ExternalLinks)
	}
	entry := xlPkg.ExternalLinks[0]
	if entry.Target != "file:///C:/workbooks/source.xlsx" || entry.Link == nil || entry.Link.ExternalBook == nil {
		t.Fatalf("unexpected external link entry: %+v", entry)
	}
	if len(entry.Link.ExternalBook.SheetNames) != 1 || entry.Link.ExternalBook.SheetNames[0].Val != "Prices" {
		t.Fatalf("unexpected external sheet names: %+v", entry.Link.ExternalBook.SheetNames)
	}
	if len(entry.Link.ExternalBook.DefinedNames) != 1 || entry.Link.ExternalBook.DefinedNames[0].Name != "ExtRange" {
		t.Fatalf("unexpected external defined names: %+v", entry.Link.ExternalBook.DefinedNames)
	}
	if entry.Link.ExternalBook.SheetDataSet == nil || len(entry.Link.ExternalBook.SheetDataSet.Sheets) != 1 {
		t.Fatalf("unexpected external sheet data set: %+v", entry.Link.ExternalBook.SheetDataSet)
	}
	sheet := entry.Link.ExternalBook.SheetDataSet.Sheets[0]
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 2 || sheet.Rows[0].Cells[0].Ref != "A1" || sheet.Rows[0].Cells[0].Value != "7" {
		t.Fatalf("unexpected external cached sheet rows: %+v", sheet.Rows)
	}
}

func TestLoadPackageParsesWorksheetShapeEffectsAndThreeD(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/drawings/drawing1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>
</Types>`,
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rIdSheet1"/></sheets>
</workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdSheet1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheetData><row r="1"><c r="A1" t="str"><v>ok</v></c></row></sheetData>
  <drawing r:id="rIdDrawing1"/>
</worksheet>`,
		"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdDrawing1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/>
</Relationships>`,
		"xl/drawings/drawing1.xml": `<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <xdr:oneCellAnchor>
    <xdr:from><xdr:col>0</xdr:col><xdr:colOff>0</xdr:colOff><xdr:row>0</xdr:row><xdr:rowOff>0</xdr:rowOff></xdr:from>
    <xdr:ext cx="1905000" cy="952500"/>
    <xdr:sp>
      <xdr:nvSpPr><xdr:cNvPr id="1" name="ThreeD shape"/></xdr:nvSpPr>
      <xdr:spPr>
        <a:scene3d>
          <a:camera prst="legacyPerspectiveTopLeft"><a:rot lat="-900000" lon="-600000"/></a:camera>
          <a:lightRig rig="threePt" dir="tl"/>
        </a:scene3d>
        <a:sp3d extrusionH="76200" contourW="25400" prstMaterial="metal">
          <a:bevelT w="38100" h="12700"/>
          <a:contourClr><a:srgbClr val="D9D9D9"/></a:contourClr>
          <a:extrusionClr><a:srgbClr val="595959"/></a:extrusionClr>
        </a:sp3d>
        <a:prstGeom prst="rect"/>
        <a:solidFill><a:srgbClr val="FF7043"/></a:solidFill>
        <a:effectLst>
          <a:outerShdw blurRad="38100" dist="25400" dir="2700000"><a:srgbClr val="222222"><a:alpha val="50000"/></a:srgbClr></a:outerShdw>
          <a:glow rad="50800"><a:srgbClr val="FF0000"/></a:glow>
          <a:softEdge rad="12700"/>
          <a:reflection blurRad="25400" dist="38100" dir="5400000" stA="25000"/>
        </a:effectLst>
      </xdr:spPr>
    </xdr:sp>
  </xdr:oneCellAnchor>
</xdr:wsDr>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(xlPkg.Worksheets) != 1 || xlPkg.Worksheets[0].Drawing == nil {
		t.Fatalf("expected worksheet drawing, got %+v", xlPkg.Worksheets)
	}
	anchors := xlPkg.Worksheets[0].Drawing.OneCellAnchors
	if len(anchors) != 1 || anchors[0].Shape == nil {
		t.Fatalf("expected one parsed worksheet shape, got %+v", anchors)
	}
	spPr := anchors[0].Shape.SpPr
	if spPr.Scene3D == nil || spPr.Scene3D.Camera == nil || spPr.Scene3D.LightRig == nil {
		t.Fatalf("expected scene3d camera/lightRig, got %#v", spPr.Scene3D)
	}
	if spPr.Scene3D.Camera.Prst != "legacyPerspectiveTopLeft" || spPr.Scene3D.LightRig.Rig != "threePt" {
		t.Fatalf("unexpected scene3d values: %#v", spPr.Scene3D)
	}
	if spPr.Shape3D == nil || spPr.Shape3D.BevelT == nil {
		t.Fatalf("expected sp3d bevel metadata, got %#v", spPr.Shape3D)
	}
	if spPr.Shape3D.PrstMaterial != "metal" || spPr.Shape3D.ExtrusionH != 76200 || spPr.Shape3D.ContourW != 25400 {
		t.Fatalf("unexpected sp3d values: %#v", spPr.Shape3D)
	}
	if spPr.EffectLst == nil || spPr.EffectLst.OuterShadow == nil || spPr.EffectLst.Glow == nil || spPr.EffectLst.SoftEdge == nil || spPr.EffectLst.Reflection == nil {
		t.Fatalf("expected parsed effectLst, got %#v", spPr.EffectLst)
	}
	if spPr.EffectLst.OuterShadow.SrgbClr == nil || spPr.EffectLst.OuterShadow.SrgbClr.Val != "222222" {
		t.Fatalf("unexpected outer shadow color: %#v", spPr.EffectLst.OuterShadow)
	}
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", ".."))
	parts := append([]string{root}, elems...)
	return filepath.Join(parts...)
}

func hasDiagnosticCode(diagnostics diag.List, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
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
