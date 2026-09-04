package legacyfixtures

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Format string

const (
	FormatDOC Format = "doc"
	FormatXLS Format = "xls"
	FormatPPT Format = "ppt"
)

type Expectation struct {
	MinParagraphs int
	MinTables     int
	MinSheets     int
	MinRows       int
	MinSlides     int
	MinShapes     int
	AllowEmpty    bool
	NeedsHTML     bool
}

type Fixture struct {
	ID           string
	Format       Format
	Bucket       string
	RelativePath string
	Expectation  Expectation
}

func RepoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func Path(rel string) string {
	return filepath.Join(RepoRoot(), filepath.FromSlash(rel))
}

func Curated() []Fixture {
	return []Fixture{
		// Curated DOC fixtures are intentional renderer/parser contracts.
		{ID: "doc-small-lists", Format: FormatDOC, Bucket: "small", RelativePath: "testdata/doc/legacy-lists.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-medium-sections", Format: FormatDOC, Bucket: "medium", RelativePath: "testdata/doc/legacy-layout-sections.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-medium-tables", Format: FormatDOC, Bucket: "medium", RelativePath: "testdata/doc/legacy-layout-tables.doc", Expectation: Expectation{MinParagraphs: 1, MinTables: 1, NeedsHTML: true}},
		{ID: "doc-edge-floating-images", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-images-floating.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-edge-header-footer-fields", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-header-footer-fields.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-edge-mixed-page", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-mixed-page.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		// Small OfficeArt fixtures are deliberately single-purpose so parser/render regressions are attributable.
		{ID: "doc-officeart-line-solid", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-officeart-line-solid.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-officeart-arrows", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-officeart-arrows.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-officeart-shadow", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-officeart-shadow.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-officeart-textbox-padding", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-officeart-textbox-padding.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-officeart-textbox-vertical", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-officeart-textbox-vertical.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-officeart-rotation", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-officeart-rotation.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{ID: "doc-officeart-group", Format: FormatDOC, Bucket: "edge", RelativePath: "testdata/doc/legacy-officeart-group.doc", Expectation: Expectation{MinParagraphs: 1, NeedsHTML: true}},
		{
			ID:           "xls-small-simple",
			Format:       FormatXLS,
			Bucket:       "small",
			RelativePath: "testdata/xls/apache-SampleSS.xls",
			Expectation:  Expectation{MinSheets: 3, MinRows: 3, NeedsHTML: true},
		},
		{
			ID:           "xls-medium-formula",
			Format:       FormatXLS,
			Bucket:       "medium",
			RelativePath: "testdata/xls/apache-SimpleWithFormula.xls",
			Expectation:  Expectation{MinSheets: 3, MinRows: 3, NeedsHTML: true},
		},
		{
			ID:           "xls-edge-comments-format",
			Format:       FormatXLS,
			Bucket:       "edge",
			RelativePath: "testdata/xls/apache-SimpleWithComments.xls",
			Expectation:  Expectation{MinSheets: 3, MinRows: 3, NeedsHTML: true},
		},
		{
			ID:           "ppt-small-basic",
			Format:       FormatPPT,
			Bucket:       "small",
			RelativePath: "testdata/ppt/apache-basic_test_ppt_file.ppt",
			Expectation:  Expectation{MinSlides: 2, NeedsHTML: true},
		},
		{
			ID:           "ppt-medium-bullets",
			Format:       FormatPPT,
			Bucket:       "medium",
			RelativePath: "testdata/ppt/apache-bullets.ppt",
			Expectation:  Expectation{MinSlides: 1, MinShapes: 2, NeedsHTML: true},
		},
		{
			ID:           "ppt-edge-sparse-show",
			Format:       FormatPPT,
			Bucket:       "edge",
			RelativePath: "testdata/ppt/apache-SampleShow.ppt",
			Expectation:  Expectation{MinSlides: 1, MinShapes: 2, NeedsHTML: true},
		},
		{
			ID:           "xls-edge-conditional-formatting",
			Format:       FormatXLS,
			Bucket:       "edge",
			RelativePath: "testdata/xls/apache-ConditionalFormattingSamples.xls",
			Expectation:  Expectation{MinSheets: 1, MinRows: 1, NeedsHTML: true},
		},
		{
			ID:           "xls-medium-data-format",
			Format:       FormatXLS,
			Bucket:       "medium",
			RelativePath: "testdata/xls/apache-SimpleWithDataFormat.xls",
			Expectation:  Expectation{MinSheets: 1, MinRows: 1, NeedsHTML: true},
		},
		{
			ID:           "ppt-medium-headers-footers",
			Format:       FormatPPT,
			Bucket:       "medium",
			RelativePath: "testdata/ppt/apache-headers_footers.ppt",
			Expectation:  Expectation{MinSlides: 1, NeedsHTML: true},
		},
		{
			ID:           "ppt-edge-master-theme",
			Format:       FormatPPT,
			Bucket:       "edge",
			RelativePath: "testdata/ppt/apache-WithMaster.ppt",
			Expectation:  Expectation{MinSlides: 2, MinShapes: 1, NeedsHTML: true},
		},
		{
			ID:           "ppt-edge-hyperlink-records",
			Format:       FormatPPT,
			Bucket:       "edge",
			RelativePath: "testdata/ppt/libre-novell6655408.ppt",
			Expectation:  Expectation{MinSlides: 1, NeedsHTML: true},
		},
		// PPT text-shape geometry (ESCHER anchored text shapes / auto-shapes).
		{
			ID:           "ppt-edge-text-shapes",
			Format:       FormatPPT,
			Bucket:       "edge",
			RelativePath: "testdata/ppt/apache-text_shapes.ppt",
			Expectation:  Expectation{MinSlides: 1, MinShapes: 3, NeedsHTML: true},
		},
		// M8: Rich media — PPT image shapes.
		{
			ID:           "ppt-edge-pictures",
			Format:       FormatPPT,
			Bucket:       "edge",
			RelativePath: "testdata/ppt/apache-pictures.ppt",
			Expectation:  Expectation{MinSlides: 1, MinShapes: 1, NeedsHTML: true},
		},
		// M8: Rich media — XLS embedded chart.
		{
			ID:           "xls-edge-charts",
			Format:       FormatXLS,
			Bucket:       "edge",
			RelativePath: "testdata/xls/apache-WithChart.xls",
			Expectation:  Expectation{MinSheets: 1, MinRows: 1, NeedsHTML: true},
		},
		// M9: Advanced XLS — array formulas.
		{
			ID:           "xls-edge-array-formulas",
			Format:       FormatXLS,
			Bucket:       "edge",
			RelativePath: "testdata/xls/libre-matrix.xls",
			Expectation:  Expectation{MinSheets: 1, MinRows: 1, NeedsHTML: true},
		},
		// M9: Advanced XLS — data validation.
		{
			ID:           "xls-edge-data-validation",
			Format:       FormatXLS,
			Bucket:       "edge",
			RelativePath: "testdata/xls/libre-validation.xls",
			Expectation:  Expectation{MinSheets: 1, MinRows: 1, NeedsHTML: true},
		},
		// M9: Advanced XLS — named ranges (global).
		{
			ID:           "xls-edge-named-ranges",
			Format:       FormatXLS,
			Bucket:       "edge",
			RelativePath: "testdata/xls/libre-named-ranges-global.xls",
			Expectation:  Expectation{MinSheets: 1, MinRows: 1, NeedsHTML: true},
		},
		// M9: Advanced XLS — database functions.
		{
			ID:           "xls-edge-database-functions",
			Format:       FormatXLS,
			Bucket:       "edge",
			RelativePath: "testdata/xls/libre-database.xls",
			Expectation:  Expectation{MinSheets: 1, MinRows: 1, NeedsHTML: true},
		},
	}
}

func Corpus() []Fixture {
	fixtures := make([]Fixture, 0, 64)
	fixtures = append(fixtures, discoverFixturesForFormat(FormatDOC, "testdata/doc", ".doc")...)
	fixtures = append(fixtures, discoverFixturesForFormat(FormatXLS, "testdata/xls", ".xls")...)
	fixtures = append(fixtures, discoverFixturesForFormat(FormatPPT, "testdata/ppt", ".ppt")...)
	return fixtures
}

var corpusExpectationOverrides = map[string]Expectation{}

func discoverFixturesForFormat(format Format, relDir, ext string) []Fixture {
	entries, err := os.ReadDir(Path(relDir))
	if err != nil {
		return nil
	}
	fixtures := make([]Fixture, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.EqualFold(filepath.Ext(name), ext) {
			continue
		}
		relPath := filepath.ToSlash(filepath.Join(relDir, name))
		expectation := Expectation{NeedsHTML: true}
		if override, ok := corpusExpectationOverrides[relPath]; ok {
			expectation = override
		}
		fixtures = append(fixtures, Fixture{
			ID:           corpusFixtureID(format, name),
			Format:       format,
			Bucket:       "corpus",
			RelativePath: relPath,
			Expectation:  expectation,
		})
	}
	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].RelativePath < fixtures[j].RelativePath
	})
	return fixtures
}

func corpusFixtureID(format Format, name string) string {
	base := strings.TrimSuffix(strings.ToLower(name), strings.ToLower(filepath.Ext(name)))
	return string(format) + "-corpus-" + sanitizeLegacyFixtureID(base)
}

func sanitizeLegacyFixtureID(text string) string {
	text = strings.ToLower(text)
	var b strings.Builder
	prevDash := false
	for _, r := range text {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "fixture"
	}
	return result
}
