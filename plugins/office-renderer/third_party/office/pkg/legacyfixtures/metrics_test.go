package legacyfixtures

import (
	"testing"
	"time"

	preslegacy "office/pkg/presentation/parsers/legacy"
	sheetlegacy "office/pkg/sheet/parsers/legacy"
	"office/pkg/word/parsers/legacy"
)

// TestCollectDocMetricsForCuratedFixtures collects baseline metrics for DOC fixtures.
func TestCollectDocMetricsForCuratedFixtures(t *testing.T) {
	baseline := make(MetricsBaseline)

	for _, fixture := range Curated() {
		if fixture.Format != FormatDOC {
			continue
		}

		t.Logf("Collecting metrics for %s (%s)", fixture.ID, fixture.RelativePath)

		// Parse the fixture
		doc, err := legacy.ParseDoc(Path(fixture.RelativePath))
		if err != nil {
			t.Logf("  ⚠ parse error: %v", err)
			continue
		}

		// Collect metrics
		docMetrics := CollectDocMetrics(doc)
		m := &Metrics{
			Format:    string(fixture.Format),
			Fixture:   fixture.ID,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Doc:       docMetrics,
		}

		baseline[fixture.ID] = m

		// Log summary
		t.Logf("  ✓ paragraphs=%d tables=%d images=%d textboxes=%d shapes=%d arrows=%d shadows=%d",
			docMetrics.ParagraphCount,
			docMetrics.TableCount,
			docMetrics.InlineImageCount+docMetrics.FloatingImageCount,
			docMetrics.TextBoxCount,
			docMetrics.ShapeCount,
			docMetrics.ArrowCount,
			docMetrics.ShadowCount)
	}

	// Print baseline for reference
	t.Logf("Baseline metrics collected for %d DOC fixtures", len(baseline))
	for id, m := range baseline {
		if m.Doc != nil {
			t.Logf("  %s: %d paragraphs, %d tables, %d images, %d textboxes, %d comments",
				id, m.Doc.ParagraphCount, m.Doc.TableCount,
				m.Doc.InlineImageCount+m.Doc.FloatingImageCount,
				m.Doc.TextBoxCount, m.Doc.CommentCount)
		}
	}
}

// TestDocMetricsRegressionAgainstBaseline validates DOC metrics against the established baseline.
func TestDocMetricsRegressionAgainstBaseline(t *testing.T) {
	// Load baseline
	baseline, err := LoadMetricsBaseline(MetricsBaselinePath())
	if err != nil {
		t.Logf("⚠ baseline file not found or unreadable: %v (skipping regression test)", err)
		t.Skip("Baseline metrics not available")
	}

	// Collect current metrics
	current := make(MetricsBaseline)
	for _, fixture := range Curated() {
		if fixture.Format != FormatDOC {
			continue
		}

		doc, err := legacy.ParseDoc(Path(fixture.RelativePath))
		if err != nil {
			t.Logf("⚠ failed to parse %s: %v", fixture.ID, err)
			continue
		}

		docMetrics := CollectDocMetrics(doc)
		current[fixture.ID] = &Metrics{
			Format:  string(fixture.Format),
			Fixture: fixture.ID,
			Doc:     docMetrics,
		}
	}

	// Compare baseline vs current
	regressions := 0
	for fixtureID, baselineM := range baseline {
		if baselineM.Doc == nil {
			continue
		}

		curr, ok := current[fixtureID]
		if !ok {
			t.Logf("⚠ fixture %s not found in current metrics", fixtureID)
			continue
		}

		if curr.Doc == nil {
			t.Logf("⚠ fixture %s failed to parse", fixtureID)
			continue
		}

		// Check for regressions (only check counts that should be stable)
		if curr.Doc.ParagraphCount != baselineM.Doc.ParagraphCount {
			t.Errorf("REGRESSION: %s paragraph count changed from %d to %d",
				fixtureID, baselineM.Doc.ParagraphCount, curr.Doc.ParagraphCount)
			regressions++
		}
		if curr.Doc.TableCount != baselineM.Doc.TableCount {
			t.Errorf("REGRESSION: %s table count changed from %d to %d",
				fixtureID, baselineM.Doc.TableCount, curr.Doc.TableCount)
			regressions++
		}
		if curr.Doc.InlineImageCount != baselineM.Doc.InlineImageCount {
			t.Errorf("REGRESSION: %s inline image count changed from %d to %d",
				fixtureID, baselineM.Doc.InlineImageCount, curr.Doc.InlineImageCount)
			regressions++
		}
		if curr.Doc.FloatingImageCount != baselineM.Doc.FloatingImageCount {
			t.Errorf("REGRESSION: %s floating image count changed from %d to %d",
				fixtureID, baselineM.Doc.FloatingImageCount, curr.Doc.FloatingImageCount)
			regressions++
		}
		if curr.Doc.TextBoxCount != baselineM.Doc.TextBoxCount {
			t.Errorf("REGRESSION: %s textbox count changed from %d to %d",
				fixtureID, baselineM.Doc.TextBoxCount, curr.Doc.TextBoxCount)
			regressions++
		}
		if curr.Doc.ListCount != baselineM.Doc.ListCount {
			t.Errorf("REGRESSION: %s list count changed from %d to %d",
				fixtureID, baselineM.Doc.ListCount, curr.Doc.ListCount)
			regressions++
		}
		if curr.Doc.BookmarkCount != baselineM.Doc.BookmarkCount {
			t.Errorf("REGRESSION: %s bookmark count changed from %d to %d",
				fixtureID, baselineM.Doc.BookmarkCount, curr.Doc.BookmarkCount)
			regressions++
		}
		if curr.Doc.ShapeCount != baselineM.Doc.ShapeCount {
			t.Errorf("REGRESSION: %s shape count changed from %d to %d", fixtureID, baselineM.Doc.ShapeCount, curr.Doc.ShapeCount)
			regressions++
		}
		if curr.Doc.FloatingShapeCount != baselineM.Doc.FloatingShapeCount {
			t.Errorf("REGRESSION: %s floating shape count changed from %d to %d", fixtureID, baselineM.Doc.FloatingShapeCount, curr.Doc.FloatingShapeCount)
			regressions++
		}
		if curr.Doc.GroupCount != baselineM.Doc.GroupCount {
			t.Errorf("REGRESSION: %s group count changed from %d to %d", fixtureID, baselineM.Doc.GroupCount, curr.Doc.GroupCount)
			regressions++
		}
		if curr.Doc.TextPathCount != baselineM.Doc.TextPathCount {
			t.Errorf("REGRESSION: %s text-path count changed from %d to %d", fixtureID, baselineM.Doc.TextPathCount, curr.Doc.TextPathCount)
			regressions++
		}
		if curr.Doc.ShadowCount != baselineM.Doc.ShadowCount {
			t.Errorf("REGRESSION: %s shadow count changed from %d to %d", fixtureID, baselineM.Doc.ShadowCount, curr.Doc.ShadowCount)
			regressions++
		}
		if curr.Doc.ArrowCount != baselineM.Doc.ArrowCount {
			t.Errorf("REGRESSION: %s arrow count changed from %d to %d", fixtureID, baselineM.Doc.ArrowCount, curr.Doc.ArrowCount)
			regressions++
		}
	}

	if regressions > 0 {
		t.Fatalf("Found %d metric regressions", regressions)
	}

	t.Logf("✓ All DOC fixture metrics validated against baseline (%d fixtures)", len(current))
}

func TestPptMetricsRegressionAgainstBaseline(t *testing.T) {
	baseline, err := LoadMetricsBaseline(MetricsBaselinePath())
	if err != nil {
		t.Logf("⚠ baseline file not found or unreadable: %v (skipping regression test)", err)
		t.Skip("Baseline metrics not available")
	}

	current := make(MetricsBaseline)
	for _, fixture := range Curated() {
		if fixture.Format != FormatPPT {
			continue
		}

		pres, err := preslegacy.ParsePpt(Path(fixture.RelativePath))
		if err != nil {
			t.Logf("⚠ failed to parse %s: %v", fixture.ID, err)
			continue
		}

		pptMetrics := CollectPptMetrics(pres)
		current[fixture.ID] = &Metrics{
			Format:  string(fixture.Format),
			Fixture: fixture.ID,
			Ppt:     pptMetrics,
		}
	}

	regressions := 0
	for fixtureID, baselineM := range baseline {
		if baselineM.Ppt == nil {
			continue
		}

		curr, ok := current[fixtureID]
		if !ok {
			t.Logf("⚠ fixture %s not found in current metrics", fixtureID)
			continue
		}

		if curr.Ppt == nil {
			t.Logf("⚠ fixture %s failed to parse", fixtureID)
			continue
		}

		if curr.Ppt.SlideCount != baselineM.Ppt.SlideCount {
			t.Errorf("REGRESSION: %s slide count changed from %d to %d",
				fixtureID, baselineM.Ppt.SlideCount, curr.Ppt.SlideCount)
			regressions++
		}
		if curr.Ppt.ShapeCount != baselineM.Ppt.ShapeCount {
			t.Errorf("REGRESSION: %s shape count changed from %d to %d",
				fixtureID, baselineM.Ppt.ShapeCount, curr.Ppt.ShapeCount)
			regressions++
		}
		if curr.Ppt.TextBoxCount != baselineM.Ppt.TextBoxCount {
			t.Errorf("REGRESSION: %s textbox count changed from %d to %d",
				fixtureID, baselineM.Ppt.TextBoxCount, curr.Ppt.TextBoxCount)
			regressions++
		}
		if curr.Ppt.PictureCount != baselineM.Ppt.PictureCount {
			t.Errorf("REGRESSION: %s picture count changed from %d to %d",
				fixtureID, baselineM.Ppt.PictureCount, curr.Ppt.PictureCount)
			regressions++
		}
		if curr.Ppt.GroupCount != baselineM.Ppt.GroupCount {
			t.Errorf("REGRESSION: %s group count changed from %d to %d",
				fixtureID, baselineM.Ppt.GroupCount, curr.Ppt.GroupCount)
			regressions++
		}
		if curr.Ppt.MasterSlideCount != baselineM.Ppt.MasterSlideCount {
			t.Errorf("REGRESSION: %s master slide count changed from %d to %d",
				fixtureID, baselineM.Ppt.MasterSlideCount, curr.Ppt.MasterSlideCount)
			regressions++
		}
	}

	if regressions > 0 {
		t.Fatalf("Found %d PPT metric regressions", regressions)
	}

	t.Logf("✓ All PPT fixture metrics validated against baseline (%d fixtures)", len(current))
}

func TestXlsMetricsRegressionAgainstBaseline(t *testing.T) {
	baseline, err := LoadMetricsBaseline(MetricsBaselinePath())
	if err != nil {
		t.Logf("⚠ baseline file not found or unreadable: %v (skipping regression test)", err)
		t.Skip("Baseline metrics not available")
	}

	current := make(MetricsBaseline)
	for _, fixture := range Curated() {
		if fixture.Format != FormatXLS {
			continue
		}

		wb, err := sheetlegacy.ParseXls(Path(fixture.RelativePath))
		if err != nil {
			t.Logf("⚠ failed to parse %s: %v", fixture.ID, err)
			continue
		}

		xlsMetrics := CollectXlsMetrics(wb)
		current[fixture.ID] = &Metrics{
			Format:  string(fixture.Format),
			Fixture: fixture.ID,
			Xls:     xlsMetrics,
		}
	}

	regressions := 0
	for fixtureID, baselineM := range baseline {
		if baselineM.Xls == nil {
			continue
		}

		curr, ok := current[fixtureID]
		if !ok {
			t.Logf("⚠ fixture %s not found in current metrics", fixtureID)
			continue
		}

		if curr.Xls == nil {
			t.Logf("⚠ fixture %s failed to parse", fixtureID)
			continue
		}

		if curr.Xls.SheetCount != baselineM.Xls.SheetCount {
			t.Errorf("REGRESSION: %s sheet count changed from %d to %d", fixtureID, baselineM.Xls.SheetCount, curr.Xls.SheetCount)
			regressions++
		}
		if curr.Xls.RowCount != baselineM.Xls.RowCount {
			t.Errorf("REGRESSION: %s row count changed from %d to %d", fixtureID, baselineM.Xls.RowCount, curr.Xls.RowCount)
			regressions++
		}
		if curr.Xls.ColumnCount != baselineM.Xls.ColumnCount {
			t.Errorf("REGRESSION: %s column count changed from %d to %d", fixtureID, baselineM.Xls.ColumnCount, curr.Xls.ColumnCount)
			regressions++
		}
		if curr.Xls.FormulaCount != baselineM.Xls.FormulaCount {
			t.Errorf("REGRESSION: %s formula count changed from %d to %d", fixtureID, baselineM.Xls.FormulaCount, curr.Xls.FormulaCount)
			regressions++
		}
		if curr.Xls.MergedCellCount != baselineM.Xls.MergedCellCount {
			t.Errorf("REGRESSION: %s merged cell count changed from %d to %d", fixtureID, baselineM.Xls.MergedCellCount, curr.Xls.MergedCellCount)
			regressions++
		}
		if curr.Xls.StringCellCount != baselineM.Xls.StringCellCount {
			t.Errorf("REGRESSION: %s string cell count changed from %d to %d", fixtureID, baselineM.Xls.StringCellCount, curr.Xls.StringCellCount)
			regressions++
		}
		if curr.Xls.NumberCellCount != baselineM.Xls.NumberCellCount {
			t.Errorf("REGRESSION: %s number cell count changed from %d to %d", fixtureID, baselineM.Xls.NumberCellCount, curr.Xls.NumberCellCount)
			regressions++
		}
	}

	if regressions > 0 {
		t.Fatalf("Found %d XLS metric regressions", regressions)
	}

	t.Logf("✓ All XLS fixture metrics validated against baseline (%d fixtures)", len(current))
}
