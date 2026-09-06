package legacyfixtures

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	presentationir "office/pkg/presentation/ir"
	preslegacy "office/pkg/presentation/parsers/legacy"
	presrender "office/pkg/presentation/render"
	sheetir "office/pkg/sheet/ir"
	sheetlegacy "office/pkg/sheet/parsers/legacy"
	sheetrender "office/pkg/sheet/render"
	word "office/pkg/word"
	wordir "office/pkg/word/ir"
	wordrender "office/pkg/word/render"
)

type Report struct {
	GeneratedAt string         `json:"generated_at,omitempty"`
	Summary     ReportSummary  `json:"summary"`
	Formats     []FormatReport `json:"formats"`
	Fixtures    []FixtureRow   `json:"fixtures"`
}

type ReportSummary struct {
	FixtureCount     int `json:"fixture_count"`
	OpenPassCount    int `json:"open_pass_count"`
	ExtractPassCount int `json:"extract_pass_count"`
	RenderPassCount  int `json:"render_pass_count"`
	FormatCount      int `json:"format_count"`
}

type FormatReport struct {
	Format           Format `json:"format"`
	FixtureCount     int    `json:"fixture_count"`
	OpenPassCount    int    `json:"open_pass_count"`
	ExtractPassCount int    `json:"extract_pass_count"`
	RenderPassCount  int    `json:"render_pass_count"`
}

type FixtureRow struct {
	ID             string `json:"id"`
	Format         Format `json:"format"`
	Bucket         string `json:"bucket"`
	RelativePath   string `json:"relative_path"`
	OpenOK         bool   `json:"open_ok"`
	ExtractOK      bool   `json:"extract_ok"`
	RenderOK       bool   `json:"render_ok"`
	OpenError      string `json:"open_error,omitempty"`
	ExtractError   string `json:"extract_error,omitempty"`
	RenderError    string `json:"render_error,omitempty"`
	ObservedMetric string `json:"observed_metric,omitempty"`
}

func BuildReport() *Report {
	return BuildReportForFixtures(Curated())
}

func BuildCorpusReport() *Report {
	return BuildReportForFixtures(Corpus())
}

func BuildReportForFixtures(fixtures []Fixture) *Report {
	rows := make([]FixtureRow, 0, len(fixtures))
	formatStats := map[Format]*FormatReport{}
	for _, fixture := range fixtures {
		row := inspectFixture(fixture)
		rows = append(rows, row)
		stat := formatStats[fixture.Format]
		if stat == nil {
			stat = &FormatReport{Format: fixture.Format}
			formatStats[fixture.Format] = stat
		}
		stat.FixtureCount++
		if row.OpenOK {
			stat.OpenPassCount++
		}
		if row.ExtractOK {
			stat.ExtractPassCount++
		}
		if row.RenderOK {
			stat.RenderPassCount++
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Format != rows[j].Format {
			return rows[i].Format < rows[j].Format
		}
		if rows[i].Bucket != rows[j].Bucket {
			return rows[i].Bucket < rows[j].Bucket
		}
		return rows[i].ID < rows[j].ID
	})
	formats := make([]FormatReport, 0, len(formatStats))
	for _, format := range []Format{FormatDOC, FormatXLS, FormatPPT} {
		if stat := formatStats[format]; stat != nil {
			formats = append(formats, *stat)
		}
	}
	report := &Report{
		Summary:  ReportSummary{FixtureCount: len(rows), FormatCount: len(formats)},
		Formats:  formats,
		Fixtures: rows,
	}
	for _, row := range rows {
		if row.OpenOK {
			report.Summary.OpenPassCount++
		}
		if row.ExtractOK {
			report.Summary.ExtractPassCount++
		}
		if row.RenderOK {
			report.Summary.RenderPassCount++
		}
	}
	return report
}

func inspectFixture(fixture Fixture) FixtureRow {
	row := FixtureRow{
		ID:           fixture.ID,
		Format:       fixture.Format,
		Bucket:       fixture.Bucket,
		RelativePath: fixture.RelativePath,
	}
	path := Path(fixture.RelativePath)
	if _, err := os.Stat(path); err != nil {
		row.OpenError = err.Error()
		return row
	}
	row.OpenOK = true
	switch fixture.Format {
	case FormatDOC:
		doc, err := word.Open(path)
		if err != nil {
			row.ExtractError = err.Error()
			return row
		}
		docMetrics := CollectDocMetrics(doc)
		row.ObservedMetric = fmt.Sprintf("paragraphs=%d tables=%d", docMetrics.ParagraphCount, docMetrics.TableCount)
		row.ExtractOK = extractOKLegacyDoc(doc, fixture.Expectation)
		if !row.ExtractOK {
			row.ExtractError = docExtractError(doc, fixture.Expectation)
		}
		if fixture.Expectation.NeedsHTML {
			html, err := wordrender.HTML(doc)
			if err != nil {
				row.RenderError = err.Error()
				return row
			}
			row.RenderOK = strings.Contains(html, "<html")
			if !row.RenderOK {
				row.RenderError = "missing html root"
			}
		}
	case FormatXLS:
		wb, err := sheetlegacy.ParseXls(path)
		if err != nil {
			row.ExtractError = err.Error()
			return row
		}
		maxRows := 0
		for _, sheet := range wb.Sheets {
			if len(sheet.Rows) > maxRows {
				maxRows = len(sheet.Rows)
			}
		}
		row.ObservedMetric = fmt.Sprintf("sheets=%d maxRows=%d", len(wb.Sheets), maxRows)
		row.ExtractOK = extractOKLegacyXLS(wb, maxRows, fixture.Expectation)
		if !row.ExtractOK {
			row.ExtractError = xlsExtractError(wb, maxRows, fixture.Expectation)
		}
		if fixture.Expectation.NeedsHTML {
			html, err := sheetrender.HTML(wb)
			if err != nil {
				row.RenderError = err.Error()
				return row
			}
			row.RenderOK = strings.Contains(html, "<html")
			if !row.RenderOK {
				row.RenderError = "missing html root"
			}
		}
	case FormatPPT:
		pres, err := preslegacy.ParsePpt(path)
		if err != nil {
			row.ExtractError = err.Error()
			return row
		}
		maxShapes := 0
		for _, slide := range pres.Slides {
			if len(slide.Shapes) > maxShapes {
				maxShapes = len(slide.Shapes)
			}
		}
		row.ObservedMetric = fmt.Sprintf("slides=%d maxShapes=%d", len(pres.Slides), maxShapes)
		row.ExtractOK = extractOKLegacyPPT(pres, maxShapes, fixture.Expectation)
		if !row.ExtractOK {
			row.ExtractError = pptExtractError(pres, maxShapes, fixture.Expectation)
		}
		if fixture.Expectation.NeedsHTML {
			html, err := presrender.HTML(pres)
			if err != nil {
				row.RenderError = err.Error()
				return row
			}
			row.RenderOK = strings.Contains(html, "<html")
			if !row.RenderOK {
				row.RenderError = "missing html root"
			}
		}
	}
	return row
}

func WriteJSON(report *Report, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func WriteMarkdown(report *Report, path, jsonName string) error {
	return WriteMarkdownReport(report, path, jsonName, "Legacy Fixture Coverage", "This report is generated from the curated legacy smoke fixtures.")
}

func WriteMarkdownReport(report *Report, path, jsonName, title, intro string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	b.WriteString(intro)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Source JSON: `%s`\n\n", jsonName)
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("| --- | ---: |\n")
	fmt.Fprintf(&b, "| Fixtures | %d |\n", report.Summary.FixtureCount)
	fmt.Fprintf(&b, "| Open pass | %d |\n", report.Summary.OpenPassCount)
	fmt.Fprintf(&b, "| Extract pass | %d |\n", report.Summary.ExtractPassCount)
	fmt.Fprintf(&b, "| Render pass | %d |\n", report.Summary.RenderPassCount)
	b.WriteString("\n## By Format\n\n")
	b.WriteString("| Format | Fixtures | Open | Extract | Render |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: |\n")
	for _, format := range report.Formats {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n", format.Format, format.FixtureCount, format.OpenPassCount, format.ExtractPassCount, format.RenderPassCount)
	}
	b.WriteString("\n## Fixture Matrix\n\n")
	b.WriteString("| ID | Format | Bucket | Path | Open | Extract | Render | Observed |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, row := range report.Fixtures {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			row.ID,
			row.Format,
			row.Bucket,
			row.RelativePath,
			passFail(row.OpenOK),
			passFail(row.ExtractOK),
			passFail(row.RenderOK),
			escapeMD(row.ObservedMetric),
		)
	}
	b.WriteString("\n## Failures\n\n")
	hasFailures := false
	for _, row := range report.Fixtures {
		if row.OpenOK && row.ExtractOK && row.RenderOK {
			continue
		}
		hasFailures = true
		fmt.Fprintf(&b, "- `%s`: open=%s extract=%s render=%s", row.ID, passFail(row.OpenOK), passFail(row.ExtractOK), passFail(row.RenderOK))
		var details []string
		if row.OpenError != "" {
			details = append(details, "open: "+row.OpenError)
		}
		if row.ExtractError != "" {
			details = append(details, "extract: "+row.ExtractError)
		}
		if row.RenderError != "" {
			details = append(details, "render: "+row.RenderError)
		}
		if len(details) > 0 {
			fmt.Fprintf(&b, " (%s)", escapeMD(strings.Join(details, "; ")))
		}
		b.WriteString("\n")
	}
	if !hasFailures {
		b.WriteString("- none\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func extractOKLegacyDoc(doc *wordir.Document, expectation Expectation) bool {
	if expectation.MinParagraphs == 0 && expectation.MinTables == 0 {
		if expectation.AllowEmpty {
			return true
		}
		return len(doc.Paragraphs) > 0 || len(doc.Tables) > 0
	}
	return len(doc.Paragraphs) >= expectation.MinParagraphs && len(doc.Tables) >= expectation.MinTables
}

func docExtractError(doc *wordir.Document, expectation Expectation) string {
	_ = doc
	if expectation.MinParagraphs == 0 && expectation.MinTables == 0 {
		if expectation.AllowEmpty {
			return ""
		}
		return "expected non-empty document structure"
	}
	return fmt.Sprintf("expected paragraphs>=%d tables>=%d", expectation.MinParagraphs, expectation.MinTables)
}

func extractOKLegacyXLS(wb *sheetir.Workbook, maxRows int, expectation Expectation) bool {
	if expectation.MinSheets == 0 && expectation.MinRows == 0 {
		return len(wb.Sheets) > 0
	}
	return len(wb.Sheets) >= expectation.MinSheets && maxRows >= expectation.MinRows
}

func xlsExtractError(wb *sheetir.Workbook, maxRows int, expectation Expectation) string {
	_ = wb
	_ = maxRows
	if expectation.MinSheets == 0 && expectation.MinRows == 0 {
		return "expected non-empty workbook content"
	}
	return fmt.Sprintf("expected sheets>=%d maxRows>=%d", expectation.MinSheets, expectation.MinRows)
}

func extractOKLegacyPPT(pres *presentationir.Presentation, maxShapes int, expectation Expectation) bool {
	_ = maxShapes
	if expectation.MinSlides == 0 && expectation.MinShapes == 0 {
		return len(pres.Slides) > 0
	}
	return len(pres.Slides) >= expectation.MinSlides && maxShapes >= expectation.MinShapes
}

func pptExtractError(pres *presentationir.Presentation, maxShapes int, expectation Expectation) string {
	_ = pres
	_ = maxShapes
	if expectation.MinSlides == 0 && expectation.MinShapes == 0 {
		return "expected non-empty presentation content"
	}
	return fmt.Sprintf("expected slides>=%d maxShapes>=%d", expectation.MinSlides, expectation.MinShapes)
}

func passFail(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func escapeMD(text string) string {
	text = strings.ReplaceAll(text, "|", "\\|")
	text = strings.ReplaceAll(text, "\n", " ")
	return text
}
