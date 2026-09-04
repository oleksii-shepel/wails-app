package speccoverage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"office/pkg/ooxml/coverage"
	"office/pkg/ooxml/specmap"
)

const (
	nsPackageContentTypes = "http://schemas.openxmlformats.org/package/2006/content-types"
	nsPackageRels         = "http://schemas.openxmlformats.org/package/2006/relationships"
	nsDrawingMain         = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsDrawingPicture      = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	nsWordDrawing         = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsWordMain            = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsMarkupCompat        = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	nsVML                 = "urn:schemas-microsoft-com:vml"
	nsWordProcessingShape = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
)

var docxPackages = map[string]struct{}{
	"ooxml/opc":                {},
	"ooxml/rels":               {},
	"ooxml/drawingxml":         {},
	"word/parsers/ooxml/parts": {},
}

var seededDocxNamespaces = []string{
	nsPackageContentTypes,
	nsPackageRels,
	nsWordMain,
	nsDrawingMain,
	nsDrawingPicture,
	nsWordDrawing,
	nsWordProcessingShape,
	nsMarkupCompat,
	nsVML,
}

// Report is a docx-focused OOXML spec coverage summary.
type Report struct {
	GeneratedAt string       `json:"generated_at"`
	Summary     Summary      `json:"summary"`
	Packages    []PkgStats   `json:"packages"`
	Namespaces  []NSStats    `json:"namespaces"`
	Structures  []Checkpoint `json:"structures"`
}

// Summary aggregates high-level metrics for the docx-focused report.
type Summary struct {
	SpecElementCount        int     `json:"spec_element_count"`
	BindingElementCount     int     `json:"binding_element_count"`
	MatchedElementCount     int     `json:"matched_element_count"`
	SpecCoveragePercent     float64 `json:"spec_coverage_percent"`
	BindingInSpecPercent    float64 `json:"binding_in_spec_percent"`
	ExactBindingCount       int     `json:"exact_binding_count"`
	HeuristicBindingCount   int     `json:"heuristic_binding_count"`
	ManualBindingCount      int     `json:"manual_binding_count"`
	SpecOnlyElementCount    int     `json:"spec_only_element_count"`
	BindingOnlyElementCount int     `json:"binding_only_element_count"`
	StructureFullCount      int     `json:"structure_full_count"`
	StructurePartialCount   int     `json:"structure_partial_count"`
	StructureMissingCount   int     `json:"structure_missing_count"`
}

// PkgStats summarizes one source package in the docx report.
type PkgStats struct {
	Package               string   `json:"package"`
	Files                 []string `json:"files"`
	Namespaces            []string `json:"namespaces"`
	BindingElementCount   int      `json:"binding_element_count"`
	MatchedElementCount   int      `json:"matched_element_count"`
	ExactBindingCount     int      `json:"exact_binding_count"`
	HeuristicBindingCount int      `json:"heuristic_binding_count"`
	ManualBindingCount    int      `json:"manual_binding_count"`
}

// NSStats summarizes one namespace in the docx report.
type NSStats struct {
	Namespace               string   `json:"namespace"`
	SpecElementCount        int      `json:"spec_element_count"`
	BindingElementCount     int      `json:"binding_element_count"`
	MatchedElementCount     int      `json:"matched_element_count"`
	SpecOnlyElementCount    int      `json:"spec_only_element_count"`
	BindingOnlyElementCount int      `json:"binding_only_element_count"`
	SpecCoveragePercent     float64  `json:"spec_coverage_percent"`
	BindingInSpecPercent    float64  `json:"binding_in_spec_percent"`
	ExactBindingCount       int      `json:"exact_binding_count"`
	HeuristicBindingCount   int      `json:"heuristic_binding_count"`
	ManualBindingCount      int      `json:"manual_binding_count"`
	ExamplesMissing         []string `json:"examples_missing,omitempty"`
	ExamplesBindingOnly     []string `json:"examples_binding_only,omitempty"`
}

// Checkpoint is a higher-level parser surface summary similar to the legacy report.
type Checkpoint struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	IRTarget string `json:"ir_target,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type elementStats struct {
	Namespace string
	Name      string
	Exact     bool
	Heuristic bool
	Manual    bool
	InSpec    bool
}

// BuildDocxSpecCoverageReport builds a docx-focused report from the OOXML spec inventory and current bindings.
func BuildDocxSpecCoverageReport(repoRoot string) (*Report, error) {
	inv, err := specmap.LoadDir(filepath.Join(repoRoot, "specs"))
	if err != nil {
		return nil, err
	}
	inv = coverage.ExtendInventory(inv)
	base, err := coverage.Build(repoRoot, inv)
	if err != nil {
		return nil, err
	}

	filteredBindings := filterBindings(base.Bindings)
	docxNamespaces := collectDocxNamespaces(filteredBindings)
	specByNamespace := buildSpecIndex(inv, docxNamespaces)
	bindingStats := buildBindingIndex(filteredBindings)
	applyCrossNamespaceMatches(bindingStats, specByNamespace)

	report := &Report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Packages:    buildPackageStats(filteredBindings, bindingStats),
		Namespaces:  buildNamespaceStats(docxNamespaces, specByNamespace, bindingStats),
		Structures:  docxStructureCheckpoints(),
	}

	report.Summary = buildSummary(report.Namespaces, report.Structures)
	return report, nil
}

func collectDocxNamespaces(bindings []coverage.Binding) []string {
	set := make(map[string]struct{}, len(seededDocxNamespaces))
	for _, namespace := range seededDocxNamespaces {
		set[namespace] = struct{}{}
	}
	for _, binding := range bindings {
		if binding.Namespace == "" {
			continue
		}
		set[binding.Namespace] = struct{}{}
	}
	namespaces := make([]string, 0, len(set))
	for namespace := range set {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	return namespaces
}

func filterBindings(bindings []coverage.Binding) []coverage.Binding {
	filtered := make([]coverage.Binding, 0, len(bindings))
	for _, binding := range bindings {
		if _, ok := docxPackages[binding.Package]; ok {
			filtered = append(filtered, binding)
		}
	}
	return filtered
}

func buildSpecIndex(inv *specmap.Inventory, namespaces []string) map[string]map[string]struct{} {
	allowed := make(map[string]struct{}, len(namespaces))
	for _, namespace := range namespaces {
		allowed[namespace] = struct{}{}
	}
	specByNamespace := make(map[string]map[string]struct{}, len(namespaces))
	for _, el := range inv.Elements {
		if el.Name == "" {
			continue
		}
		if _, ok := allowed[el.Namespace]; !ok {
			continue
		}
		if specByNamespace[el.Namespace] == nil {
			specByNamespace[el.Namespace] = map[string]struct{}{}
		}
		specByNamespace[el.Namespace][el.Name] = struct{}{}
	}
	for _, namespace := range namespaces {
		if specByNamespace[namespace] == nil {
			specByNamespace[namespace] = map[string]struct{}{}
		}
	}
	return specByNamespace
}

func buildBindingIndex(bindings []coverage.Binding) map[string]elementStats {
	index := make(map[string]elementStats)
	for _, binding := range bindings {
		key := elementKey(binding.Namespace, binding.Name)
		stat := index[key]
		stat.Namespace = binding.Namespace
		stat.Name = binding.Name
		stat.Exact = stat.Exact || binding.ExplicitNamespace
		stat.Heuristic = stat.Heuristic || binding.HeuristicNamespace
		stat.Manual = stat.Manual || binding.Manual
		stat.InSpec = stat.InSpec || binding.InSpec
		index[key] = stat
	}
	return index
}

// applyCrossNamespaceMatches propagates bindings from extension namespaces to
// the canonical transitional namespaces they supersede.  For example, the
// Microsoft wps: extension moved many wp: elements into a separate schema;
// bindings tagged with the wps: URI should still satisfy the wp: spec inventory.
func applyCrossNamespaceMatches(bindingStats map[string]elementStats, specByNamespace map[string]map[string]struct{}) {
	// wps: (wordprocessingShape extension) → wp: (wordprocessingDrawing)
	wpSpec := specByNamespace[nsWordDrawing]
	for _, stat := range bindingStats {
		if stat.Namespace != nsWordProcessingShape {
			continue
		}
		if wpSpec != nil {
			if _, ok := wpSpec[stat.Name]; !ok {
				continue
			}
		}
		wpKey := elementKey(nsWordDrawing, stat.Name)
		wpStat := bindingStats[wpKey]
		wpStat.Namespace = nsWordDrawing
		wpStat.Name = stat.Name
		wpStat.Exact = wpStat.Exact || stat.Exact
		wpStat.Heuristic = wpStat.Heuristic || stat.Heuristic
		wpStat.Manual = wpStat.Manual || stat.Manual
		wpStat.InSpec = true
		bindingStats[wpKey] = wpStat
	}

	// wps: (wordprocessingShape extension) → a: (drawingml/2006/main)
	aSpec := specByNamespace[nsDrawingMain]
	for _, stat := range bindingStats {
		if stat.Namespace != nsWordProcessingShape {
			continue
		}
		if aSpec != nil {
			if _, ok := aSpec[stat.Name]; !ok {
				continue
			}
		}
		aKey := elementKey(nsDrawingMain, stat.Name)
		aStat := bindingStats[aKey]
		aStat.Namespace = nsDrawingMain
		aStat.Name = stat.Name
		aStat.Exact = aStat.Exact || stat.Exact
		aStat.Heuristic = aStat.Heuristic || stat.Heuristic
		aStat.Manual = aStat.Manual || stat.Manual
		aStat.InSpec = true
		bindingStats[aKey] = aStat
	}
}

func buildPackageStats(bindings []coverage.Binding, bindingStats map[string]elementStats) []PkgStats {
	packageKeys := make(map[string]map[string]struct{})
	packages := make(map[string]*PkgStats)
	for _, binding := range bindings {
		pkg := packages[binding.Package]
		if pkg == nil {
			pkg = &PkgStats{Package: binding.Package}
			packages[binding.Package] = pkg
		}
		pkg.Files = appendUnique(pkg.Files, binding.File)
		pkg.Namespaces = appendUnique(pkg.Namespaces, binding.Namespace)
		if packageKeys[binding.Package] == nil {
			packageKeys[binding.Package] = map[string]struct{}{}
		}
		packageKeys[binding.Package][elementKey(binding.Namespace, binding.Name)] = struct{}{}
	}
	var out []PkgStats
	for pkgName, pkg := range packages {
		keys := packageKeys[pkgName]
		for key := range keys {
			stat := bindingStats[key]
			pkg.BindingElementCount++
			if stat.InSpec {
				pkg.MatchedElementCount++
			}
			if stat.Exact {
				pkg.ExactBindingCount++
			}
			if stat.Heuristic {
				pkg.HeuristicBindingCount++
			}
			if stat.Manual {
				pkg.ManualBindingCount++
			}
		}
		sort.Strings(pkg.Files)
		sort.Strings(pkg.Namespaces)
		out = append(out, *pkg)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MatchedElementCount != out[j].MatchedElementCount {
			return out[i].MatchedElementCount > out[j].MatchedElementCount
		}
		return out[i].Package < out[j].Package
	})
	return out
}

func buildNamespaceStats(docxNamespaces []string, specByNamespace map[string]map[string]struct{}, bindingStats map[string]elementStats) []NSStats {
	var out []NSStats
	for _, namespace := range docxNamespaces {
		specNames := specByNamespace[namespace]
		boundNames := make(map[string]elementStats)
		for _, stat := range bindingStats {
			if stat.Namespace != namespace {
				continue
			}
			boundNames[stat.Name] = stat
		}

		matched := 0
		exact := 0
		heuristic := 0
		manual := 0
		var missing []string
		for name := range specNames {
			if _, ok := boundNames[name]; ok {
				matched++
				continue
			}
			missing = append(missing, name)
		}

		var bindingOnly []string
		for name, stat := range boundNames {
			if stat.Exact {
				exact++
			}
			if stat.Heuristic {
				heuristic++
			}
			if stat.Manual {
				manual++
			}
			if _, ok := specNames[name]; !ok {
				bindingOnly = append(bindingOnly, name)
			}
		}

		sort.Strings(missing)
		sort.Strings(bindingOnly)

		out = append(out, NSStats{
			Namespace:               namespace,
			SpecElementCount:        len(specNames),
			BindingElementCount:     len(boundNames),
			MatchedElementCount:     matched,
			SpecOnlyElementCount:    len(missing),
			BindingOnlyElementCount: len(bindingOnly),
			SpecCoveragePercent:     percent(matched, len(specNames)),
			BindingInSpecPercent:    percent(matched, len(boundNames)),
			ExactBindingCount:       exact,
			HeuristicBindingCount:   heuristic,
			ManualBindingCount:      manual,
			ExamplesMissing:         head(missing, 12),
			ExamplesBindingOnly:     head(bindingOnly, 12),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MatchedElementCount != out[j].MatchedElementCount {
			return out[i].MatchedElementCount > out[j].MatchedElementCount
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func buildSummary(namespaces []NSStats, checkpoints []Checkpoint) Summary {
	var summary Summary
	for _, ns := range namespaces {
		summary.SpecElementCount += ns.SpecElementCount
		summary.BindingElementCount += ns.BindingElementCount
		summary.MatchedElementCount += ns.MatchedElementCount
		summary.ExactBindingCount += ns.ExactBindingCount
		summary.HeuristicBindingCount += ns.HeuristicBindingCount
		summary.ManualBindingCount += ns.ManualBindingCount
		summary.SpecOnlyElementCount += ns.SpecOnlyElementCount
		summary.BindingOnlyElementCount += ns.BindingOnlyElementCount
	}
	summary.SpecCoveragePercent = percent(summary.MatchedElementCount, summary.SpecElementCount)
	summary.BindingInSpecPercent = percent(summary.MatchedElementCount, summary.BindingElementCount)
	for _, checkpoint := range checkpoints {
		switch checkpoint.Status {
		case "full":
			summary.StructureFullCount++
		case "partial":
			summary.StructurePartialCount++
		case "missing":
			summary.StructureMissingCount++
		}
	}
	return summary
}

func docxStructureCheckpoints() []Checkpoint {
	checkpoints := []Checkpoint{
		{ID: "opc-core", Label: "OPC package + relationships", Status: "full", IRTarget: "MainPartName / package graph / related part loading", Notes: "The OOXML substrate opens encrypted and plain OPC containers, resolves `.rels`, and dispatches the main Word parts."},
		{ID: "document-body", Label: "Document body + block/runs", Status: "full", IRTarget: "Document.Body / Paragraphs / Tables / Runs", Notes: "Main document parsing covers paragraphs, tables, hyperlinks, bookmarks, comments, math, and run-level content into Word IR."},
		{ID: "styles-numbering", Label: "Styles + numbering", Status: "full", IRTarget: "Document.Styles / Lists / ParagraphFormat / RunFormat", Notes: "Styles, doc defaults, numbering, and inheritance resolution are wired through `parts` and `resolve`."},
		{ID: "headers-footers", Label: "Headers and footers", Status: "full", IRTarget: "Headers / Footers / Section references", Notes: "Header/footer part loading, section references, and write-back regeneration are active."},
		{ID: "notes-comments", Label: "Footnotes, endnotes, comments", Status: "full", IRTarget: "Footnotes / Endnotes / Comments / references", Notes: "Story parts, references, and comment ranges are parsed and resolved into the document model."},
		{ID: "settings-properties", Label: "Settings, theme, and document properties", Status: "full", IRTarget: "Document settings / Theme colors-fonts / Core-extended-custom properties", Notes: "Settings parsing covers the full w:settings surface including drawing grid, book-fold printing, captions, read-mode ink lock-down, smart-tag types, attached schemas, and all scalar mail-merge fields. Nested ODSO field-map and recipient data are resolved into MailMergeSettings."},
		{ID: "content-controls", Label: "Structured document tags and form fields", Status: "partial", IRTarget: "ContentControl / Run.FormField", Notes: "SDT properties, bindings, checkbox/plain-text controls, and `ffData` parsing are present; the wider control matrix still needs fuller inventory coverage."},
		{ID: "fields", Label: "Field instructions and semantic fields", Status: "partial", IRTarget: "Run.FieldKind / FieldInstr / Anchor / Target / FormField", Notes: "Common field families resolve semantically, including the current HTML mail-merge scope; the remaining field gap is long-tail nested/composite combinations rather than basic merge-control support."},
		{ID: "embedded-fonts", Label: "Embedded fonts", Status: "full", IRTarget: "EmbeddedFonts / FontFamilies", Notes: "Font-table relationship resolution and embedded font payload lifting are implemented for OOXML Word packages."},
		{ID: "drawingml-images", Label: "DrawingML and VML images", Status: "full", IRTarget: "Run.Image / floating image geometry", Notes: "Inline and anchored image paths, media relationships, crop, wrap, and VML fallback image handling are active."},
		{ID: "drawingml-shapes", Label: "DrawingML shapes and text boxes", Status: "partial", IRTarget: "Run.Shape / Run.TextBox / Paragraph.Frame", Notes: "Preset geometry, fills, body properties, and text-box extraction are implemented, but complex custom geometry and effect combinations are still incomplete."},
		{ID: "charts-smartart", Label: "Charts and SmartArt", Status: "partial", IRTarget: "Run.Chart / Run.Shape SVG payloads", Notes: "Chart and SmartArt references are resolved and rendered through shared OOXML helpers, but the semantic/rendering surface is narrower than plain text and images."},
		{ID: "revisions", Label: "Tracked changes and property revisions", Status: "full", IRTarget: "Run.Revision* / Paragraph.PropertyRevision / TableCell changes / Section.PropertyRevision", Notes: "Run-level insertions, deletions, and moves render with <ins>/<del> wrappers and revision metadata. Paragraph, table, row, and cell property revisions are resolved into IR and rendered as data attributes. Table cell insertion, deletion, and merge tracking are rendered. Move-range markers are emitted at paragraph boundaries; inline span correlation requires paragraph-unmarshaler changes and is documented as a future enhancement."},
	}
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].ID < checkpoints[j].ID
	})
	return checkpoints
}

// WriteJSON writes the report as deterministic pretty JSON.
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

// WriteMarkdown writes the human-readable markdown companion.
func WriteMarkdown(report *Report, path, jsonName string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# DOCX Spec Coverage\n\n")
	b.WriteString("This report is generated from the current Word OOXML parser bindings plus the local OOXML schema inventory. It measures `.docx` schema/binding surface and overlays higher-level parser checkpoints; it does not measure rendering fidelity by itself.\n\n")
	fmt.Fprintf(&b, "Source JSON: `%s`\n\n", jsonName)

	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("| --- | ---: |\n")
	fmt.Fprintf(&b, "| Spec element names | %d |\n", report.Summary.SpecElementCount)
	fmt.Fprintf(&b, "| Bound element names | %d |\n", report.Summary.BindingElementCount)
	fmt.Fprintf(&b, "| Matched element names | %d |\n", report.Summary.MatchedElementCount)
	fmt.Fprintf(&b, "| Spec coverage | %.1f%% |\n", report.Summary.SpecCoveragePercent)
	fmt.Fprintf(&b, "| Bound names in spec | %.1f%% |\n", report.Summary.BindingInSpecPercent)
	fmt.Fprintf(&b, "| Exact namespace matches | %d |\n", report.Summary.ExactBindingCount)
	fmt.Fprintf(&b, "| Heuristic namespace matches | %d |\n", report.Summary.HeuristicBindingCount)
	fmt.Fprintf(&b, "| Manual-handler names | %d |\n", report.Summary.ManualBindingCount)
	fmt.Fprintf(&b, "| Spec-only names | %d |\n", report.Summary.SpecOnlyElementCount)
	fmt.Fprintf(&b, "| Binding-only names | %d |\n", report.Summary.BindingOnlyElementCount)
	fmt.Fprintf(&b, "| Structure checkpoints (full / partial / missing) | %d / %d / %d |\n", report.Summary.StructureFullCount, report.Summary.StructurePartialCount, report.Summary.StructureMissingCount)

	b.WriteString("\n## Parser Packages\n\n")
	b.WriteString("| Package | Files | Bound | Matched | Exact | Heuristic | Manual |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, pkg := range report.Packages {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d | %d | %d |\n",
			pkg.Package, len(pkg.Files), pkg.BindingElementCount, pkg.MatchedElementCount, pkg.ExactBindingCount, pkg.HeuristicBindingCount, pkg.ManualBindingCount)
	}

	b.WriteString("\n## Namespaces\n\n")
	b.WriteString("| Namespace | Spec | Bound | Matched | Spec coverage | Bound in spec |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, ns := range report.Namespaces {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %.1f%% | %.1f%% |\n",
			ns.Namespace, ns.SpecElementCount, ns.BindingElementCount, ns.MatchedElementCount, ns.SpecCoveragePercent, ns.BindingInSpecPercent)
	}

	b.WriteString("\n## Largest Namespace Gaps\n\n")
	b.WriteString("| Namespace | Missing | Example names |\n")
	b.WriteString("| --- | ---: | --- |\n")
	for _, ns := range report.Namespaces {
		if ns.SpecOnlyElementCount == 0 {
			continue
		}
		fmt.Fprintf(&b, "| `%s` | %d | `%s` |\n",
			ns.Namespace, ns.SpecOnlyElementCount, strings.Join(ns.ExamplesMissing, "`, `"))
	}

	b.WriteString("\n## Structure Checkpoints\n\n")
	b.WriteString("| Surface | Status | IR target | Notes |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, checkpoint := range report.Structures {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			checkpoint.Label,
			checkpoint.Status,
			escapeMarkdown(checkpoint.IRTarget),
			escapeMarkdown(checkpoint.Notes))
	}

	b.WriteString("\n## Sources\n\n")
	b.WriteString("- `pkg/word/parsers/ooxml/parts/model.go`, `pkg/ooxml/opc/contenttypes.go`, and `pkg/ooxml/rels/rels.go` for the active `.docx` binding surface.\n")
	b.WriteString("- `specs/*.zip` via `pkg/ooxml/specmap` for the OOXML schema inventory.\n")
	b.WriteString("- Shared extraction heuristics from `pkg/ooxml/coverage` for namespace attribution and manual handler detection.\n")

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func elementKey(namespace, name string) string {
	return namespace + "\x00" + name
}

func percent(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) * 100 / float64(den)
}

func head(values []string, n int) []string {
	if len(values) <= n {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:n]...)
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func escapeMarkdown(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "|", "\\|")
}