package edit_test

import (
	"bytes"
	"testing"

	drawing "office/pkg/ooxml/drawing"
	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
)

func TestSheetImageInsertionRoundTrip(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	image := []byte{0x89, 0x50, 0x4e, 0x47}
	if _, err := sheet.AddImageOneCell("Logo", image, "image/png", "Company logo", 1, 1, 914400, 457200); err != nil {
		t.Fatalf("AddImageOneCell: %v", err)
	}
	if _, err := sheet.AddShape("Callout", "rect", "Hello", "FF0000", "000000", 1, sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorTwoCell,
		FromCell: sheetir.DrawingCellAnchor{Row: 2, Col: 2},
		ToCell:   sheetir.DrawingCellAnchor{Row: 4, Col: 4},
	}); err != nil {
		t.Fatalf("AddShape: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	drawings := roundTrip.Sheets[0].Drawings
	if len(drawings) != 2 {
		t.Fatalf("drawings = %d, want 2", len(drawings))
	}
	if drawings[0].Type != sheetir.DrawingTypeImage || drawings[0].Image == nil || drawings[0].Image.AltText != "Company logo" {
		t.Errorf("image drawing = %#v", drawings[0])
	}
	if drawings[1].Type != sheetir.DrawingTypeShape || drawings[1].Shape == nil || drawings[1].Shape.Text != "Hello" {
		t.Errorf("shape drawing = %#v", drawings[1])
	}
}

func TestSheetRemoveDrawing(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if _, err := sheet.AddImageAbsolute("Logo", []byte{1, 2, 3}, "image/png", "", 0, 0, 100, 100); err != nil {
		t.Fatalf("AddImageAbsolute: %v", err)
	}
	if !sheet.RemoveDrawing("Logo") {
		t.Fatalf("RemoveDrawing returned false")
	}
	if len(sheet.Drawings()) != 0 {
		t.Fatalf("drawings = %d, want 0", len(sheet.Drawings()))
	}
}

func TestOpenXLSXExistingImageRemovalRoundTrip(t *testing.T) {
	wb := openFixtureWorkbook(t, "sample-5.xlsx")
	name := "Screenshot of Calendar for February using the Streamline theme"
	sheet := findSheetDrawing(t, wb, name)
	if !sheet.RemoveDrawing(name) {
		t.Fatal("RemoveDrawing returned false")
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	for _, rtSheet := range roundTrip.Sheets {
		for _, drawing := range rtSheet.Drawings {
			if drawing != nil && drawing.Name == name {
				t.Fatalf("removed drawing still present: %#v", drawing)
			}
		}
	}
}

func TestSheetConnectorAndRichTextShapeRoundTrip(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if _, err := sheet.AddConnector("Flow", 1, 1, 4, 3, "00AAFF", 2); err != nil {
		t.Fatalf("AddConnector: %v", err)
	}
	if err := sheet.SetConnectorType("Flow", "bentConnector3"); err != nil {
		t.Fatalf("SetConnectorType: %v", err)
	}
	if err := sheet.SetConnectorLineStyle("Flow", &drawing.LineStyle{
		Width: 3,
		Color: "00AAFF",
		Dash:  drawing.DashPatternDashDot,
		Cap:   drawing.LineCapRound,
		Join:  drawing.LineJoinBevel,
		HeadEnd: &drawing.Arrowhead{
			Type:   drawing.ArrowheadTriangle,
			Size:   drawing.ArrowheadSizeMedium,
			Length: drawing.ArrowheadSizeMedium,
		},
		TailEnd: &drawing.Arrowhead{
			Type:   drawing.ArrowheadArrow,
			Size:   drawing.ArrowheadSizeLarge,
			Length: drawing.ArrowheadSizeLarge,
		},
	}); err != nil {
		t.Fatalf("SetConnectorLineStyle: %v", err)
	}
	runs := []*sheetir.RichTextRun{
		{Text: "Bold", Bold: true, FontColor: "FF0000", FontSizePt: 14},
		{Text: " normal", FontFamily: "Aptos"},
	}
	if _, err := sheet.AddRichTextShape("Label", "rect", "FFFFFF", "000000", 1, runs, sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorOneCell,
		FromCell: sheetir.DrawingCellAnchor{Row: 5, Col: 2},
		Width:    1800000,
		Height:   600000,
	}); err != nil {
		t.Fatalf("AddRichTextShape: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	drawings := roundTrip.Sheets[0].Drawings
	if len(drawings) != 2 {
		t.Fatalf("drawings = %d, want 2", len(drawings))
	}
	connector := findDrawing(t, drawings, "Flow")
	if connector.Type != sheetir.DrawingTypeConnector || connector.Shape == nil {
		t.Fatalf("connector drawing = %#v", connector)
	}
	if connector.Position.Anchor != sheetir.AnchorTwoCell || connector.Position.ToCell.Row != 4 || connector.Position.ToCell.Col != 3 {
		t.Errorf("connector position = %#v", connector.Position)
	}
	if connector.Shape.ConnectorType != "bentConnector3" {
		t.Fatalf("connector type = %q, want bentConnector3", connector.Shape.ConnectorType)
	}
	if connector.Shape.LineStyle == nil {
		t.Fatalf("connector line style missing: %#v", connector.Shape)
	}
	if connector.Shape.LineStyle.Dash != drawing.DashPatternDashDot || connector.Shape.LineStyle.Cap != drawing.LineCapRound || connector.Shape.LineStyle.Join != drawing.LineJoinBevel {
		t.Fatalf("connector line style = %#v", connector.Shape.LineStyle)
	}
	if connector.Shape.LineStyle.HeadEnd == nil || connector.Shape.LineStyle.HeadEnd.Type != drawing.ArrowheadTriangle {
		t.Fatalf("connector head end = %#v", connector.Shape.LineStyle.HeadEnd)
	}
	if connector.Shape.LineStyle.TailEnd == nil || connector.Shape.LineStyle.TailEnd.Type != drawing.ArrowheadArrow {
		t.Fatalf("connector tail end = %#v", connector.Shape.LineStyle.TailEnd)
	}
	label := findDrawing(t, drawings, "Label").Shape
	if label == nil || label.Text != "Bold normal" {
		t.Fatalf("label = %#v", label)
	}
	if len(label.RichTextRuns) != 2 {
		t.Fatalf("rich text runs = %d, want 2", len(label.RichTextRuns))
	}
	if !label.RichTextRuns[0].Bold || label.RichTextRuns[0].FontColor != "FF0000" || label.RichTextRuns[0].FontSizePt != 14 {
		t.Errorf("first run = %#v", label.RichTextRuns[0])
	}
	if label.RichTextRuns[1].FontFamily != "Aptos" {
		t.Errorf("second run = %#v", label.RichTextRuns[1])
	}
}

func TestSheetGroupedDrawingRoundTrip(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	group, err := sheet.AddGroup("Cluster", sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorOneCell,
		FromCell: sheetir.DrawingCellAnchor{Row: 2, Col: 2},
		Width:    3600000,
		Height:   1800000,
	})
	if err != nil {
		t.Fatalf("AddGroup: %v", err)
	}
	if _, err := group.AddShape("Box", "rect", "Inner", "FFF4CC", "000000", 1, 120000, 180000, 1400000, 650000); err != nil {
		t.Fatalf("group AddShape: %v", err)
	}
	if _, err := group.AddConnector("Arrow", 250000, 980000, 1500000, 300000, "0088CC", 2); err != nil {
		t.Fatalf("group AddConnector: %v", err)
	}
	if err := group.SetConnectorType("Arrow", "bentConnector3"); err != nil {
		t.Fatalf("group SetConnectorType: %v", err)
	}
	if err := group.SetConnectorLineStyle("Arrow", &drawing.LineStyle{
		Width: 2.5,
		Color: "0088CC",
		Dash:  drawing.DashPatternDash,
	}); err != nil {
		t.Fatalf("group SetConnectorLineStyle: %v", err)
	}
	nested, err := group.AddGroup("Nested", 1900000, 250000, 1200000, 900000)
	if err != nil {
		t.Fatalf("group AddGroup: %v", err)
	}
	if _, err := nested.AddShape("Badge", "ellipse", "Hi", "CCFFCC", "006600", 1, 100000, 120000, 500000, 420000); err != nil {
		t.Fatalf("nested AddShape: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	drawings := roundTrip.Sheets[0].Drawings
	if len(drawings) != 1 {
		t.Fatalf("drawings = %d, want 1", len(drawings))
	}
	cluster := drawings[0]
	if cluster.Type != sheetir.DrawingTypeGroup || cluster.Group == nil {
		t.Fatalf("group drawing = %#v", cluster)
	}
	if cluster.Position.Width != 3600000 || cluster.Position.Height != 1800000 {
		t.Fatalf("group position = %#v", cluster.Position)
	}
	if len(cluster.Group.Children) != 3 {
		t.Fatalf("group children = %d, want 3", len(cluster.Group.Children))
	}
	box := findDrawing(t, cluster.Group.Children, "Box")
	if box.Type != sheetir.DrawingTypeShape || box.Shape == nil || box.Shape.Text != "Inner" {
		t.Fatalf("box = %#v", box)
	}
	if box.Position.Anchor != sheetir.AnchorAbsolute || box.Position.X != 120000 || box.Position.Width != 1400000 {
		t.Fatalf("box position = %#v", box.Position)
	}
	arrow := findDrawing(t, cluster.Group.Children, "Arrow")
	if arrow.Type != sheetir.DrawingTypeConnector || arrow.Shape == nil {
		t.Fatalf("arrow = %#v", arrow)
	}
	if arrow.Shape.ConnectorType != "bentConnector3" {
		t.Fatalf("arrow connector type = %q", arrow.Shape.ConnectorType)
	}
	if arrow.Shape.LineStyle == nil || arrow.Shape.LineStyle.Dash != drawing.DashPatternDash {
		t.Fatalf("arrow line style = %#v", arrow.Shape.LineStyle)
	}
	nestedGroup := findDrawing(t, cluster.Group.Children, "Nested")
	if nestedGroup.Type != sheetir.DrawingTypeGroup || nestedGroup.Group == nil || len(nestedGroup.Group.Children) != 1 {
		t.Fatalf("nested group = %#v", nestedGroup)
	}
	badge := findDrawing(t, nestedGroup.Group.Children, "Badge")
	if badge.Type != sheetir.DrawingTypeShape || badge.Shape == nil || badge.Shape.Text != "Hi" {
		t.Fatalf("badge = %#v", badge)
	}
}

func newWorkbookWithSheet(t *testing.T) *edit.Workbook {
	t.Helper()
	wb := edit.NewWorkbook()
	if _, err := wb.CreateSheet("Data"); err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	return wb
}

func findDrawing(t *testing.T, drawings []*sheetir.Drawing, name string) *sheetir.Drawing {
	t.Helper()
	for _, drawing := range drawings {
		if drawing.Name == name {
			return drawing
		}
	}
	t.Fatalf("drawing %q not found", name)
	return nil
}

func findSheetDrawing(t *testing.T, wb *edit.Workbook, drawingName string) *edit.Sheet {
	t.Helper()
	for _, sheet := range wb.Sheets() {
		if _, ok := sheet.Drawing(drawingName); ok {
			return sheet
		}
	}
	t.Fatalf("drawing %q not found", drawingName)
	return nil
}
func TestAddOLEObject(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	s, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddOLEObject("Object 1", "Package", []byte("payload"), "application/vnd.openxmlformats-officedocument.oleObject", sheetir.DrawingPosition{Anchor: sheetir.AnchorOneCell, FromCell: sheetir.DrawingCellAnchor{Row: 2, Col: 2}, Width: 914400, Height: 457200})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.OLEObjects(); len(got) != 1 || string(got[0].OLE.Data) != "payload" {
		t.Fatalf("OLE=%#v", got)
	}
}
