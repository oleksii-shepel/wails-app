package legacyfixtures

import (
	"fmt"
	"testing"

	word "office/pkg/word"
	wordir "office/pkg/word/ir"
)

func TestExtractOKLegacyDocRejectsUnexpectedBlankDocument(t *testing.T) {
	if extractOKLegacyDoc(&wordir.Document{}, Expectation{}) {
		t.Fatal("empty document unexpectedly passed without AllowEmpty")
	}
	if got := docExtractError(&wordir.Document{}, Expectation{}); got != "expected non-empty document structure" {
		t.Fatalf("docExtractError = %q, want expected non-empty document structure", got)
	}
}

func TestExtractOKLegacyDocAllowsConfiguredBlankDocument(t *testing.T) {
	expectation := Expectation{AllowEmpty: true}
	if !extractOKLegacyDoc(&wordir.Document{}, expectation) {
		t.Fatal("empty document did not pass with AllowEmpty")
	}
	if got := docExtractError(&wordir.Document{}, expectation); got != "" {
		t.Fatalf("docExtractError = %q, want empty string", got)
	}
}

func TestReportObservedMetricConsistentAcrossOverlappingCuratedAndCorpusDocs(t *testing.T) {
	curatedByPath := map[string]Fixture{}
	for _, fixture := range Curated() {
		if fixture.Format == FormatDOC {
			curatedByPath[fixture.RelativePath] = fixture
		}
	}

	pairs := make([]Fixture, 0, 16)
	for _, fixture := range Corpus() {
		if fixture.Format != FormatDOC {
			continue
		}
		if curated, ok := curatedByPath[fixture.RelativePath]; ok {
			pairs = append(pairs, curated, fixture)
		}
	}
	if len(pairs) == 0 {
		t.Fatal("no overlapping curated/corpus DOC fixtures found")
	}

	report := BuildReportForFixtures(pairs)
	rows := map[string]FixtureRow{}
	for _, row := range report.Fixtures {
		rows[row.ID] = row
	}

	for i := 0; i+1 < len(pairs); i += 2 {
		curated := pairs[i]
		corpus := pairs[i+1]
		curatedRow, ok := rows[curated.ID]
		if !ok {
			t.Fatalf("missing curated row for %s", curated.ID)
		}
		corpusRow, ok := rows[corpus.ID]
		if !ok {
			t.Fatalf("missing corpus row for %s", corpus.ID)
		}

		// Both rows inspect the same physical DOC. Reporting expectations may
		// differ, but extraction outcome and any observed structural metric must
		// not depend on whether the file was reached through Curated or Corpus.
		if curatedRow.ObservedMetric != corpusRow.ObservedMetric {
			t.Fatalf("observed metric mismatch for %s: curated=%q corpus=%q", curated.RelativePath, curatedRow.ObservedMetric, corpusRow.ObservedMetric)
		}
		if curatedRow.ObservedMetric == "" {
			if curatedRow.ExtractError != corpusRow.ExtractError {
				t.Fatalf("extract error mismatch for %s: curated=%q corpus=%q", curated.RelativePath, curatedRow.ExtractError, corpusRow.ExtractError)
			}
			// A report is allowed to contain parser failures; those are represented
			// by ExtractError rather than a fabricated numeric observed metric.
			continue
		}

		var curatedParagraphs, curatedTables int
		if _, err := fmt.Sscanf(curatedRow.ObservedMetric, "paragraphs=%d tables=%d", &curatedParagraphs, &curatedTables); err != nil {
			t.Fatalf("parse curated observed metric %q: %v", curatedRow.ObservedMetric, err)
		}
		var corpusParagraphs, corpusTables int
		if _, err := fmt.Sscanf(corpusRow.ObservedMetric, "paragraphs=%d tables=%d", &corpusParagraphs, &corpusTables); err != nil {
			t.Fatalf("parse corpus observed metric %q: %v", corpusRow.ObservedMetric, err)
		}

		doc, err := word.Open(Path(curated.RelativePath))
		if err != nil {
			t.Fatalf("word.Open(%s): %v", curated.RelativePath, err)
		}
		metrics := CollectDocMetrics(doc)

		if curatedParagraphs != corpusParagraphs || curatedTables != corpusTables {
			t.Fatalf("curated vs corpus mismatch for %s: curated=(%d,%d) corpus=(%d,%d)", curated.RelativePath, curatedParagraphs, curatedTables, corpusParagraphs, corpusTables)
		}
		if curatedParagraphs != metrics.ParagraphCount || curatedTables != metrics.TableCount {
			t.Fatalf("observed metric mismatch for %s: observed=(%d,%d) metrics=(%d,%d)", curated.RelativePath, curatedParagraphs, curatedTables, metrics.ParagraphCount, metrics.TableCount)
		}
	}
}
