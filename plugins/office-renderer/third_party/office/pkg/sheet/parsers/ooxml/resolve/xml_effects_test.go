package resolve_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/opc"
	"office/pkg/sheet/parsers/ooxml/parts"
	"office/pkg/sheet/parsers/ooxml/resolve"
)

func TestResolvePackageFromXMLCarriesWorksheetShapeEffectsAndThreeD(t *testing.T) {
	data := buildResolvePackage(t, map[string]string{
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
	wb, err := resolve.ResolvePackage(xlPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	if len(wb.Sheets) != 1 || len(wb.Sheets[0].Drawings) != 1 || wb.Sheets[0].Drawings[0].Shape == nil {
		t.Fatalf("expected resolved worksheet shape drawing, got %+v", wb.Sheets)
	}
	shape := wb.Sheets[0].Drawings[0].Shape
	if len(shape.Effects.Effects) != 5 {
		t.Fatalf("effect count = %d, want 5", len(shape.Effects.Effects))
	}
	threeD, ok := shape.Effects.Effects[0].(drawing.ThreeD)
	if !ok {
		t.Fatalf("3D type = %T, want drawing.ThreeD", shape.Effects.Effects[0])
	}
	if threeD.CameraPreset != "legacyPerspectiveTopLeft" || threeD.LightDirection != "tl" || threeD.Material != "metal" {
		t.Fatalf("unexpected 3D effect = %#v", threeD)
	}
	if threeD.ExtrusionDepth != 6.0 || threeD.ContourWidth != 2.0 || threeD.BevelTopHeight != 1.0 {
		t.Fatalf("unexpected 3D dimensions = %#v", threeD)
	}
	shadow, ok := shape.Effects.Effects[1].(drawing.Shadow)
	if !ok || shadow.Color != "222222" || shadow.Alpha != 0.5 {
		t.Fatalf("unexpected shadow = %#v", shape.Effects.Effects[1])
	}
}

func buildResolvePackage(t *testing.T, files map[string]string) []byte {
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