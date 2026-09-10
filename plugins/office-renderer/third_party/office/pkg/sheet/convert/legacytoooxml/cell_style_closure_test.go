package legacytoooxml

import (
	"strings"
	"testing"

	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

func TestPhase2ClosureConvertPreservesPatternFillAndAppliedStyleGroups(t *testing.T) {
	wb := &sheetir.Workbook{Sheets: []*sheetir.Sheet{{
		Name: "Sheet1",
		ID:   1,
		Rows: []*sheetir.Row{{Index: 1, Cells: []*sheetir.Cell{{
			Ref:      "A1",
			Kind:     sheetir.CellString,
			StrValue: "styled",
			Style: sheetir.CellStyle{
				Bold:               true,
				FillPattern:        "darkGrid",
				BgColor:            "112233",
				FillPatternBgColor: "445566",
				HAlign:             "distributed",
				WrapText:           true,
				NumFmtCode:         "0.##",
				BorderDiagonal:     sheetir.BorderStyle{Style: "medium", Color: "778899"},
				BorderDiagonalUp:   true,
			},
		}}},
		}}}}

	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(pkg.Styles.CellXfs) < 2 {
		t.Fatalf("cell xfs = %d, want styled xf", len(pkg.Styles.CellXfs))
	}
	xf := pkg.Styles.CellXfs[len(pkg.Styles.CellXfs)-1]
	if !xf.ApplyFont || !xf.ApplyFill || !xf.ApplyBorder || !xf.ApplyAlignment || !xf.ApplyNumberFormat {
		t.Fatalf("apply flags = font:%v fill:%v border:%v alignment:%v numFmt:%v", xf.ApplyFont, xf.ApplyFill, xf.ApplyBorder, xf.ApplyAlignment, xf.ApplyNumberFormat)
	}
	if xf.FillID < 0 || xf.FillID >= len(pkg.Styles.Fills) {
		t.Fatalf("fill id = %d", xf.FillID)
	}
	pf := pkg.Styles.Fills[xf.FillID].PatternFill
	if pf == nil || pf.PatternType != "darkGrid" || pf.FgColor == nil || pf.FgColor.RGB != "112233" || pf.BgColor == nil || pf.BgColor.RGB != "445566" {
		t.Fatalf("pattern fill = %#v", pf)
	}
}

func TestPhase2ClosureXfDedupIncludesApplyFlags(t *testing.T) {
	a := sheetparts.Xf{FontID: 0, XfID: 1}
	b := a
	b.ApplyFont = true
	if xfsEqual(a, b) {
		t.Fatal("xf equality must preserve applyFont reset semantics")
	}

	b = a
	b.ApplyAlignment = true
	if xfsEqual(a, b) {
		t.Fatal("xf equality must preserve applyAlignment semantics")
	}
}

func TestDrawingAnchorMetadataRoundTripsIntoOOXMLParts(t *testing.T) {
	locked, printed := true, false
	position := sheetir.DrawingPosition{
		Behavior:        sheetir.AnchorBehaviorMoveOnly,
		LocksWithSheet:  &locked,
		PrintsWithSheet: &printed,
	}
	if got := drawingEditAs(position.Behavior); got != "oneCell" {
		t.Fatalf("drawingEditAs(move-only) = %q, want oneCell", got)
	}
	client := drawingClientData(position)
	if client == nil || client.LocksWithSheet == nil || !*client.LocksWithSheet {
		t.Fatal("drawing clientData lost fLocksWithSheet=true")
	}
	if client.PrintsWithSheet == nil || *client.PrintsWithSheet {
		t.Fatal("drawing clientData lost fPrintsWithSheet=false")
	}
}

func TestPhase3VMLRoundTripPreservesUnknownVMLAndProjectsEditableObjects(t *testing.T) {
	raw := []byte(`<xml xmlns:v="urn:schemas-microsoft-com:vml"><v:shape id="keep" data-unknown="yes"/><v:shape id="edit" style="width:10pt" data-extra="survives"/></xml>`)
	wb := &sheetir.Workbook{Sheets: []*sheetir.Sheet{{
		Name:             "Sheet1",
		ID:               1,
		LegacyDrawingVML: raw,
		VMLObjects: []*sheetir.VMLObject{{
			ID: "edit", Style: "width:30pt", Text: "Updated",
		}},
	}}}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(pkg.Worksheets) != 1 {
		t.Fatalf("worksheets = %d, want 1", len(pkg.Worksheets))
	}
	out := string(pkg.Worksheets[0].LegacyDrawingVML)
	if !strings.Contains(out, `data-extra="survives"`) {
		t.Fatalf("edited VML object lost unknown XML: %s", out)
	}
	if strings.Contains(out, `id="keep"`) {
		t.Fatalf("removed canonical VML object remained in output: %s", out)
	}
	if !strings.Contains(out, "width:30pt") || !strings.Contains(out, "Updated") {
		t.Fatalf("canonical VML edit was not projected: %s", out)
	}
}
