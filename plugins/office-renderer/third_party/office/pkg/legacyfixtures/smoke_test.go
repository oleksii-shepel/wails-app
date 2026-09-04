package legacyfixtures_test

import (
	"strings"
	"testing"

	legacyfixtures "office/pkg/legacyfixtures"
	preslegacy "office/pkg/presentation/parsers/legacy"
	presrender "office/pkg/presentation/render"
	sheetlegacy "office/pkg/sheet/parsers/legacy"
	sheetrender "office/pkg/sheet/render"
	wordlegacy "office/pkg/word/parsers/legacy"
	wordrender "office/pkg/word/render"
)

func TestLegacyCorpusSmoke(t *testing.T) {
	for _, fixture := range legacyfixtures.Corpus() {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			path := legacyfixtures.Path(fixture.RelativePath)
			switch fixture.Format {
			case legacyfixtures.FormatDOC:
				doc, err := wordlegacy.ParseDoc(path)
				if err != nil {
					if isEncryptedFixtureError(err) {
						t.Skipf("encrypted fixture skipped: %v", err)
					}
					t.Skipf("parse doc skipped: %v", err)
				}
				html, err := wordrender.HTML(doc)
				if err != nil {
					t.Fatalf("render html: %v", err)
				}
				if !strings.Contains(html, "<html") {
					t.Fatalf("expected html document output")
				}
			case legacyfixtures.FormatXLS:
				wb, err := sheetlegacy.ParseXls(path)
				if err != nil {
					if isEncryptedFixtureError(err) {
						t.Skipf("encrypted fixture skipped: %v", err)
					}
					t.Skipf("parse xls skipped: %v", err)
				}
				html, err := sheetrender.HTML(wb)
				if err != nil {
					t.Fatalf("render html: %v", err)
				}
				if !strings.Contains(html, "<html") {
					t.Fatalf("expected html document output")
				}
			case legacyfixtures.FormatPPT:
				pres, err := preslegacy.ParsePpt(path)
				if err != nil {
					if isEncryptedFixtureError(err) {
						t.Skipf("encrypted fixture skipped: %v", err)
					}
					t.Skipf("parse ppt skipped: %v", err)
				}
				html, err := presrender.HTML(pres)
				if err != nil {
					t.Fatalf("render html: %v", err)
				}
				if !strings.Contains(html, "<html") {
					t.Fatalf("expected html document output")
				}
			default:
				t.Fatalf("unsupported fixture format %q", fixture.Format)
			}
		})
	}
}

func TestLegacyCuratedSmoke(t *testing.T) {
	for _, fixture := range legacyfixtures.Curated() {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			path := legacyfixtures.Path(fixture.RelativePath)
			switch fixture.Format {
			case legacyfixtures.FormatDOC:
				doc, err := wordlegacy.ParseDoc(path)
				if err != nil {
					if isEncryptedFixtureError(err) {
						t.Skipf("encrypted fixture skipped: %v", err)
					}
					t.Fatalf("parse doc: %v", err)
				}
				if len(doc.Paragraphs) < fixture.Expectation.MinParagraphs {
					t.Fatalf("paragraph count = %d, want >= %d", len(doc.Paragraphs), fixture.Expectation.MinParagraphs)
				}
				if len(doc.Tables) < fixture.Expectation.MinTables {
					t.Fatalf("table count = %d, want >= %d", len(doc.Tables), fixture.Expectation.MinTables)
				}
				if fixture.Expectation.NeedsHTML {
					html, err := wordrender.HTML(doc)
					if err != nil {
						t.Fatalf("render html: %v", err)
					}
					if !strings.Contains(html, "<html") {
						t.Fatalf("expected html document output")
					}
				}
			case legacyfixtures.FormatXLS:
				wb, err := sheetlegacy.ParseXls(path)
				if err != nil {
					if isEncryptedFixtureError(err) {
						t.Skipf("encrypted fixture skipped: %v", err)
					}
					t.Fatalf("parse xls: %v", err)
				}
				if len(wb.Sheets) < fixture.Expectation.MinSheets {
					t.Fatalf("sheet count = %d, want >= %d", len(wb.Sheets), fixture.Expectation.MinSheets)
				}
				maxRows := 0
				for _, sheet := range wb.Sheets {
					if len(sheet.Rows) > maxRows {
						maxRows = len(sheet.Rows)
					}
				}
				if maxRows < fixture.Expectation.MinRows {
					t.Fatalf("max row count = %d, want >= %d", maxRows, fixture.Expectation.MinRows)
				}
				if fixture.Expectation.NeedsHTML {
					html, err := sheetrender.HTML(wb)
					if err != nil {
						t.Fatalf("render html: %v", err)
					}
					if !strings.Contains(html, "<html") {
						t.Fatalf("expected html document output")
					}
				}
			case legacyfixtures.FormatPPT:
				pres, err := preslegacy.ParsePpt(path)
				if err != nil {
					if isEncryptedFixtureError(err) {
						t.Skipf("encrypted fixture skipped: %v", err)
					}
					t.Fatalf("parse ppt: %v", err)
				}
				if len(pres.Slides) < fixture.Expectation.MinSlides {
					t.Fatalf("slide count = %d, want >= %d", len(pres.Slides), fixture.Expectation.MinSlides)
				}
				maxShapes := 0
				for _, slide := range pres.Slides {
					if len(slide.Shapes) > maxShapes {
						maxShapes = len(slide.Shapes)
					}
				}
				if maxShapes < fixture.Expectation.MinShapes {
					t.Fatalf("max shape count = %d, want >= %d", maxShapes, fixture.Expectation.MinShapes)
				}
				if fixture.Expectation.NeedsHTML {
					html, err := presrender.HTML(pres)
					if err != nil {
						t.Fatalf("render html: %v", err)
					}
					if !strings.Contains(html, "<html") {
						t.Fatalf("expected html document output")
					}
				}
			default:
				t.Fatalf("unsupported fixture format %q", fixture.Format)
			}
		})
	}
}

func isEncryptedFixtureError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "encrypted") {
		return true
	}
	if strings.Contains(msg, "filepass") {
		return true
	}
	if strings.Contains(msg, "password") &&
		(strings.Contains(msg, "legacy doc:") ||
			strings.Contains(msg, "legacy xls:") ||
			strings.Contains(msg, "legacy ppt:") ||
			strings.Contains(msg, "opc:")) {
		return true
	}
	if strings.Contains(msg, "encryption") &&
		(strings.Contains(msg, "legacy doc:") ||
			strings.Contains(msg, "legacy xls:") ||
			strings.Contains(msg, "legacy ppt:") ||
			strings.Contains(msg, "opc:")) {
		return true
	}
	return false
}
