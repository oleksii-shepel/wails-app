package render

import (
	"strings"
	"testing"

	"office/pkg/sheet/parsers/ooxml/formula"
	"office/pkg/sheet/parsers/ooxml/resolve"
)

func TestEvaluateTopBottomRuleUsesResolvedRankMetadata(t *testing.T) {
	cellValues := map[string]cellValueData{
		"A1": {kind: resolve.CellNumber, numValue: 10},
		"A2": {kind: resolve.CellNumber, numValue: 20},
		"A3": {kind: resolve.CellNumber, numValue: 30},
		"A4": {kind: resolve.CellNumber, numValue: 40},
	}
	cellRefs := []string{"A1", "A2", "A3", "A4"}

	topRule := &resolve.CFRule{Type: "top10", SqRef: "A1:A4", Rank: 2}
	if !evaluateTopBottomRule(topRule, "A4", cellValues, cellRefs) {
		t.Fatal("expected highest value to match top-2 rule")
	}
	if !evaluateTopBottomRule(topRule, "A3", cellValues, cellRefs) {
		t.Fatal("expected second-highest value to match top-2 rule")
	}
	if evaluateTopBottomRule(topRule, "A2", cellValues, cellRefs) {
		t.Fatal("expected third-highest value not to match top-2 rule")
	}

	bottomPercentRule := &resolve.CFRule{Type: "top10", SqRef: "A1:A4", Bottom: true, Percent: true, Rank: 25}
	if !evaluateTopBottomRule(bottomPercentRule, "A1", cellValues, cellRefs) {
		t.Fatal("expected lowest value to match bottom 25% rule")
	}
	if evaluateTopBottomRule(bottomPercentRule, "A2", cellValues, cellRefs) {
		t.Fatal("expected second-lowest value not to match bottom 25% rule")
	}
}

func TestResolveCFVOThresholdsUseRangeStatistics(t *testing.T) {
	thresholds := resolveCFVOThresholds([]resolve.CFCfvo{
		{Type: "min"},
		{Type: "percent", Val: "50"},
		{Type: "percentile", Val: "50"},
		{Type: "max"},
	}, []float64{200, 300, 400}, "A1", nil)

	if len(thresholds) != 4 {
		t.Fatalf("expected 4 thresholds, got %d", len(thresholds))
	}
	if thresholds[0] != 200 {
		t.Fatalf("expected min threshold 200, got %v", thresholds[0])
	}
	if thresholds[1] != 300 {
		t.Fatalf("expected 50%% threshold 300, got %v", thresholds[1])
	}
	if thresholds[2] != 300 {
		t.Fatalf("expected 50th percentile threshold 300, got %v", thresholds[2])
	}
	if thresholds[3] != 400 {
		t.Fatalf("expected max threshold 400, got %v", thresholds[3])
	}
}

func TestRenderIconSetUsesHighValueForFirstIcon(t *testing.T) {
	out := renderIconSet(&resolve.CFIconSet{Set: "3TrafficLights1"}, []float64{0, 33, 67}, 90)
	if !strings.Contains(out, "🟢") {
		t.Fatalf("expected high value to use green icon, got %q", out)
	}
}

func TestParseHFSectionsThreeSections(t *testing.T) {
	left, center, right := parseHFSections("&LLeft&CCenter&RRight")
	if left != "Left" {
		t.Errorf("left=%q, want %q", left, "Left")
	}
	if center != "Center" {
		t.Errorf("center=%q, want %q", center, "Center")
	}
	if right != "Right" {
		t.Errorf("right=%q, want %q", right, "Right")
	}
}

func TestParseHFSectionsNoMarkerDefaultsToCenter(t *testing.T) {
	left, center, right := parseHFSections("OnlyText")
	if left != "" {
		t.Errorf("left=%q, want empty", left)
	}
	if center != "OnlyText" {
		t.Errorf("center=%q, want %q", center, "OnlyText")
	}
	if right != "" {
		t.Errorf("right=%q, want empty", right)
	}
}

func TestRenderHFSectionHTMLFormatCodes(t *testing.T) {
	// &&  should produce a literal ampersand.
	out := renderHFSectionHTML("A&&B", "Sheet1", nil, nil)
	if !strings.Contains(out, "A&amp;B") {
		t.Errorf("expected literal ampersand, got %q", out)
	}

	// &A should substitute sheet name.
	out = renderHFSectionHTML("&A", "MySheet", nil, nil)
	if out != "MySheet" {
		t.Errorf("expected sheet name, got %q", out)
	}

	// &P renders a placeholder span.
	out = renderHFSectionHTML("Page &P", "Sheet1", nil, nil)
	if !strings.Contains(out, "hf-page-num") {
		t.Errorf("expected page-num span, got %q", out)
	}

	// Bold toggle wraps text in <b>...</b>.
	out = renderHFSectionHTML("&BHello&B", "Sheet1", nil, nil)
	if !strings.Contains(out, "<b>Hello</b>") {
		t.Errorf("expected bold tags, got %q", out)
	}
}

func TestRenderSheetHeaderFooterEmitsHTMLStructure(t *testing.T) {
	var b strings.Builder
	renderSheetHeaderFooter(&b, "&LLeft&CCenter&RRight", "header", "Sheet1", nil)
	out := b.String()
	if !strings.Contains(out, "sheet-header") {
		t.Errorf("expected sheet-header class, got %q", out)
	}
	if !strings.Contains(out, "sheet-header-left") {
		t.Errorf("expected sheet-header-left, got %q", out)
	}
	if !strings.Contains(out, "Left") {
		t.Errorf("expected Left text in header, got %q", out)
	}
}

func TestRenderSheetHeaderFooterEmptyStringDoesNothing(t *testing.T) {
	var b strings.Builder
	renderSheetHeaderFooter(&b, "", "header", "Sheet1", nil)
	if b.Len() != 0 {
		t.Errorf("expected no output for empty text, got %q", b.String())
	}
}

func TestEvaluateValidationFormulaCrossSheetReference(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{
			{Name: "Sheet1"},
			{
				Name: "Limits",
				Rows: []*resolve.Row{{
					Index: 1,
					Cells: []*resolve.Cell{{Ref: "A1", Kind: resolve.CellNumber, NumValue: 7}},
				}},
			},
		},
	}
	ctx := buildWorkbookFormulaContext(wb)
	got, ok := evaluateValidationFormula(ctx, 0, `'Limits'!$A$1`, "C2", "C2")
	if !ok {
		t.Fatal("expected formula evaluation to succeed")
	}
	if !got.IsNumber() || got.Num != 7 {
		t.Fatalf("got %#v, want number 7", got)
	}
}

func TestEvaluateValidationFormulaNamedCrossSheetCountIf(t *testing.T) {
	localSheetID := 0
	wb := &resolve.Workbook{
		DefinedNames: []*resolve.DefinedName{{
			Name:         "AllowedValues",
			LocalSheetID: &localSheetID,
			Value:        `'Lookup Sheet'!$A$1:$A$3`,
		}},
		Sheets: []*resolve.Sheet{
			{
				Name: "Sheet1",
				Rows: []*resolve.Row{{
					Index: 2,
					Cells: []*resolve.Cell{{Ref: "B2", Kind: resolve.CellString, StrValue: "January"}},
				}, {
					Index: 3,
					Cells: []*resolve.Cell{{Ref: "B3", Kind: resolve.CellString, StrValue: "April"}},
				}},
			},
			{
				Name: "Lookup Sheet",
				Rows: []*resolve.Row{{
					Index: 1,
					Cells: []*resolve.Cell{{Ref: "A1", Kind: resolve.CellString, StrValue: "January"}},
				}, {
					Index: 2,
					Cells: []*resolve.Cell{{Ref: "A2", Kind: resolve.CellString, StrValue: "February"}},
				}, {
					Index: 3,
					Cells: []*resolve.Cell{{Ref: "A3", Kind: resolve.CellString, StrValue: "March"}},
				}},
			},
		},
	}
	ctx := buildWorkbookFormulaContext(wb)
	got, ok := evaluateValidationFormula(ctx, 0, `COUNTIF(AllowedValues,B2)>0`, "B2", "B2")
	if !ok {
		t.Fatal("expected COUNTIF formula evaluation to succeed")
	}
	boolVal, err := got.ToBool()
	if err != nil || !boolVal {
		t.Fatalf("got %#v, want TRUE", got)
	}
	got, ok = evaluateValidationFormula(ctx, 0, `COUNTIF(AllowedValues,B2)>0`, "B2", "B3")
	if !ok {
		t.Fatal("expected translated COUNTIF formula evaluation to succeed")
	}
	boolVal, err = got.ToBool()
	if err != nil {
		t.Fatalf("ToBool: %v", err)
	}
	if boolVal {
		t.Fatalf("got %#v, want FALSE", got)
	}
}

func TestValidationFormulaContextResolvesPlaceholderRange(t *testing.T) {
	ctx := &validationFormulaContext{
		workbook: &validationWorkbookContext{
			wb:      &resolve.Workbook{Sheets: []*resolve.Sheet{{Name: "Sheet1"}}},
			engines: []*formula.Engine{formula.NewEngine()},
		},
		currentSheetIndex: 0,
		currentEngine:     formula.NewEngine(),
		cellTargets: map[string]validationRefTarget{
			"_DVREF1": {sheetIndex: 0, ref: "A1"},
			"_DVREF2": {sheetIndex: 0, ref: "A3"},
		},
	}
	ctx.workbook.engines[0].SetCell("A1", formula.Text("January"))
	ctx.workbook.engines[0].SetCell("A2", formula.Text("February"))
	ctx.workbook.engines[0].SetCell("A3", formula.Text("March"))

	got := ctx.getRange("_DVREF1", "_DVREF2")
	if len(got) != 3 {
		t.Fatalf("len(getRange) = %d, want 3", len(got))
	}
	if got[0].ToText() != "January" || got[2].ToText() != "March" {
		t.Fatalf("unexpected range values: %#v", got)
	}
}

func TestValidationFormulaContextExternalizesQuotedSheetRef(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{
			{Name: "Sheet1"},
			{
				Name: "Lookup Sheet",
				Rows: []*resolve.Row{{
					Index: 1,
					Cells: []*resolve.Cell{{Ref: "A1", Kind: resolve.CellNumber, NumValue: 7}},
				}},
			},
		},
	}
	workbookCtx := buildWorkbookFormulaContext(wb)
	ctx := &validationFormulaContext{
		workbook:          workbookCtx,
		currentSheetIndex: 0,
		currentEngine:     workbookCtx.engineForSheet(0),
	}
	got := ctx.externalizeSheetRefs(`'Lookup Sheet'!$A$1:$A$3`)
	if got != `_DVREF1:_DVREF2` {
		t.Fatalf("externalizeSheetRefs = %q, want %q", got, `_DVREF1:_DVREF2`)
	}
	if len(ctx.cellTargets) != 2 {
		t.Fatalf("len(cellTargets) = %d, want 2", len(ctx.cellTargets))
	}
	if val := ctx.getCell("_DVREF1"); !val.IsNumber() || val.Num != 7 {
		t.Fatalf("getCell(_DVREF1) = %#v, want 7", val)
	}
	val, err := formula.EvaluateInContext("_DVREF1", ctx.getCell, ctx.getRange)
	if err != nil {
		t.Fatalf("EvaluateInContext: %v", err)
	}
	if !val.IsNumber() || val.Num != 7 {
		t.Fatalf("EvaluateInContext(_DVREF1) = %#v, want 7", val)
	}
}

func TestBuildWorkbookFormulaContextConfiguresSheetResolver(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{
			{Name: "Sheet1"},
			{Name: "Lookup Sheet"},
			{Name: "Summary"},
		},
	}
	workbookCtx := buildWorkbookFormulaContext(wb)
	eng := workbookCtx.engineForSheet(0)
	if eng == nil {
		t.Fatal("expected engine for first sheet")
	}

	eng.SetFormula("A1", `SHEET("Lookup Sheet")`)
	eng.SetFormula("A2", `SHEET('Lookup Sheet'!A1)`)
	eng.SetFormula("A3", `SHEETS()`)
	eng.SetFormula("A4", `SHEET(A1)`)

	for ref, want := range map[string]float64{"A1": 2, "A2": 2, "A3": 3, "A4": 1} {
		if got := eng.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestBuildWorkbookFormulaContextResolvesCachedExternalWorkbookRefs(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{Name: "Sheet1"}},
		ExternalLinks: []*resolve.WorkbookExternalLink{{
			Target:     "file:///C:/workbooks/source.xlsx",
			SheetNames: []string{"Prices"},
			CachedSheets: []*resolve.WorkbookExternalSheet{{
				ID:   0,
				Name: "Prices",
				Cells: []*resolve.WorkbookExternalCell{{
					Ref:   "A1",
					Type:  "n",
					Value: "7",
				}, {
					Ref:   "B1",
					Type:  "n",
					Value: "3",
				}, {
					Ref:   "A2",
					Type:  "str",
					Value: "North",
				}},
			}},
		}},
	}
	workbookCtx := buildWorkbookFormulaContext(wb)
	eng := workbookCtx.engineForSheet(0)
	if eng == nil {
		t.Fatal("expected engine for first sheet")
	}

	eng.SetFormula("A1", `SUM([1]Prices!A1:B1)`)
	if got := eng.GetCell("A1"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("SUM([1]Prices!A1:B1) = %#v, want 10", got)
	}
	if got := workbookCtx.getCellForSheet(0, `'[source.xlsx]Prices'!A2`); got.ToText() != "North" {
		t.Fatalf("quoted filename external ref = %#v, want North", got)
	}
	values := workbookCtx.getRangeForSheet(0, `[1]Prices!A1`, `B1`)
	if len(values) != 2 || !values[0].IsNumber() || values[0].Num != 7 || !values[1].IsNumber() || values[1].Num != 3 {
		t.Fatalf("external range values = %#v, want [7 3]", values)
	}
}

func TestRenderPivotTableEmitsHTMLBlock(t *testing.T) {
	pt := &resolve.PivotTable{
		Name:        "My Pivot",
		LocationRef: "A1:D5",
		SourceSheet: "Data",
		SourceRef:   "A1:D50",
		RowFields:   []string{"Region"},
		ColFields:   []string{"Quarter"},
		PageFields:  []string{"Year"},
		DataFields: []*resolve.PivotDataField{
			{Name: "Sum of Revenue", SourceFieldName: "Revenue", Subtotal: "sum"},
		},
	}
	var b strings.Builder
	renderPivotTable(&b, pt)
	out := b.String()

	for _, want := range []string{
		"pivot-table",
		"My Pivot",
		"Data!A1:D50",
		"Region",
		"Quarter",
		"Year",
		"Sum of Revenue",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("pivot table HTML missing %q; got:\n%s", want, out)
		}
	}
}

func TestRenderPivotTableNilSafe(t *testing.T) {
	var b strings.Builder
	renderPivotTable(&b, nil)
	if b.Len() != 0 {
		t.Errorf("expected empty output for nil pivot table, got %q", b.String())
	}
}


func TestRenderTableEnrichment(t *testing.T) {
	tbl := &resolve.Table{
		ID:             1,
		Name:           "TableName",
		DisplayName:    "TableName",
		Ref:            "G1:I4",
		TotalsRowShown: false,
		HeaderRowCount: 1,
		TotalsRowCount: 0,
		AutoFilterRef:  "G1:I4",
		TableStyle: &resolve.TableStyleInfo{
			Name:           "TableStyleLight1",
			ShowRowStripes: true,
		},
		Columns: []*resolve.TableColumn{
			{ID: 1, Name: "Name"},
			{ID: 2, Name: "Age"},
			{ID: 3, Name: "AgeGroup", CalculatedColumnFormula: "INT(H2/10)"},
		},
	}

	var b strings.Builder
	renderTable(&b, tbl)
	out := b.String()

	if !strings.Contains(out, "TableStyleLight1") {
		t.Errorf("expected style name in output, got: %s", out)
	}
	if !strings.Contains(out, "rowStripes") {
		t.Errorf("expected rowStripes flag in output, got: %s", out)
	}
	if !strings.Contains(out, "G1:I4") {
		t.Errorf("expected AutoFilter ref in output, got: %s", out)
	}
	if !strings.Contains(out, "INT(H2/10)") {
		t.Errorf("expected calculated column formula in output, got: %s", out)
	}
}

func TestRenderTableNilSafe(t *testing.T) {
	var b strings.Builder
	renderTable(&b, nil)
	if b.Len() != 0 {
		t.Errorf("expected empty output for nil table, got %q", b.String())
	}
}
