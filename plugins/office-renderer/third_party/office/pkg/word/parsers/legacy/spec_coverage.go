package legacy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type DocSpecCoverageReport struct {
	GeneratedAt string                      `json:"generated_at"`
	Summary     DocSpecCoverageSummary      `json:"summary"`
	SprmGroups  []DocSpecCoverageGroup      `json:"sprm_groups"`
	Structures  []DocSpecCoverageCheckpoint `json:"structures"`
}

type DocSpecCoverageSummary struct {
	CanonicalSPRMs        int     `json:"canonical_sprms"`
	HandledSPRMs          int     `json:"handled_sprms"`
	DecodeOnlySPRMs       int     `json:"decode_only_sprms"`
	SemanticSPRMs         int     `json:"semantic_sprms"`
	ParserCoveragePct     float64 `json:"parser_coverage_pct"`
	SemanticCoveragePct   float64 `json:"semantic_coverage_pct"`
	StructureFullCount    int     `json:"structure_full_count"`
	StructurePartialCount int     `json:"structure_partial_count"`
	StructureMissingCount int     `json:"structure_missing_count"`
}

type DocSpecCoverageGroup struct {
	ID                  string  `json:"id"`
	Label               string  `json:"label"`
	OurSource           string  `json:"our_source"`
	CanonicalSource     string  `json:"canonical_source"`
	HandledCount        int     `json:"handled_count"`
	DecodeOnlyCount     int     `json:"decode_only_count"`
	SemanticCount       int     `json:"semantic_count"`
	CanonicalCount      int     `json:"canonical_count"`
	ParserCoveragePct   float64 `json:"parser_coverage_pct"`
	SemanticCoveragePct float64 `json:"semantic_coverage_pct"`
	Notes               string  `json:"notes,omitempty"`
}

type DocSpecCoverageCheckpoint struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	IRTarget string `json:"ir_target,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

func BuildDocSpecCoverageReport(repoRoot string) (*DocSpecCoverageReport, error) {
	groups := []DocSpecCoverageGroup{
		{
			ID:              "chp-sprms",
			Label:           "CHP SPRMs",
			OurSource:       filepath.ToSlash(filepath.Join("pkg", "word", "parsers", "legacy", "chpx.go")),
			CanonicalSource: filepath.ToSlash(filepath.Join("refs", "apache-poi", "poi-scratchpad", "src", "main", "java", "org", "apache", "poi", "hwpf", "sprm", "CharacterSprmUncompressor.java")),
			Notes:           "Counts are taken from the main CHPX switch in parseChpxGrpprlWithMetadataAndFonts versus POI's unCompressCHPOperation switch.",
		},
		{
			ID:              "pap-tap-sprms",
			Label:           "PAP/TAP SPRMs",
			OurSource:       filepath.ToSlash(filepath.Join("pkg", "word", "parsers", "legacy", "papx.go")),
			CanonicalSource: filepath.ToSlash(filepath.Join("refs", "apache-poi", "poi-scratchpad", "src", "main", "java", "org", "apache", "poi", "hwpf", "sprm", "ParagraphSprmUncompressor.java")) + " + " + filepath.ToSlash(filepath.Join("refs", "apache-poi", "poi-scratchpad", "src", "main", "java", "org", "apache", "poi", "hwpf", "sprm", "TableSprmUncompressor.java")),
			Notes:           "Our PAPX switch covers paragraph and table row/cell SPRMs together, so the canonical denominator is POI paragraph plus table uncompressors.",
		},
		{
			ID:              "sep-sprms",
			Label:           "SEP SPRMs",
			OurSource:       filepath.ToSlash(filepath.Join("pkg", "word", "parsers", "legacy", "sepx.go")),
			CanonicalSource: filepath.ToSlash(filepath.Join("refs", "apache-poi", "poi-scratchpad", "src", "main", "java", "org", "apache", "poi", "hwpf", "sprm", "SectionSprmUncompressor.java")),
			Notes:           "Counts come from the Word 97+ section grpprl switch in parseSepxGrpprl versus POI's unCompressSEPOperation switch.",
		},
	}

	chpxBody, err := extractFunctionBody(filepath.Join(repoRoot, "pkg", "word", "parsers", "legacy", "chpx.go"), "func parseChpxGrpprlWithMetadataAndFonts(")
	if err != nil {
		return nil, err
	}
	papxBody, err := extractFunctionBody(filepath.Join(repoRoot, "pkg", "word", "parsers", "legacy", "papx.go"), "func parsePapxGrpprlWithMetadata(")
	if err != nil {
		return nil, err
	}
	sepxBody, err := extractFunctionBody(filepath.Join(repoRoot, "pkg", "word", "parsers", "legacy", "sepx.go"), "func parseSepxGrpprlWithMetadata(")
	if err != nil {
		return nil, err
	}
	poiCHP, err := extractFunctionBody(filepath.Join(repoRoot, "refs", "apache-poi", "poi-scratchpad", "src", "main", "java", "org", "apache", "poi", "hwpf", "sprm", "CharacterSprmUncompressor.java"), "static void unCompressCHPOperation")
	if err != nil {
		return nil, err
	}
	poiPAP, err := extractFunctionBody(filepath.Join(repoRoot, "refs", "apache-poi", "poi-scratchpad", "src", "main", "java", "org", "apache", "poi", "hwpf", "sprm", "ParagraphSprmUncompressor.java"), "static void unCompressPAPOperation")
	if err != nil {
		return nil, err
	}
	poiTAP, err := extractFunctionBody(filepath.Join(repoRoot, "refs", "apache-poi", "poi-scratchpad", "src", "main", "java", "org", "apache", "poi", "hwpf", "sprm", "TableSprmUncompressor.java"), "static void unCompressTAPOperation")
	if err != nil {
		return nil, err
	}
	poiSEP, err := extractFunctionBody(filepath.Join(repoRoot, "refs", "apache-poi", "poi-scratchpad", "src", "main", "java", "org", "apache", "poi", "hwpf", "sprm", "SectionSprmUncompressor.java"), "static void unCompressSEPOperation")
	if err != nil {
		return nil, err
	}

	groups[0].HandledCount, groups[0].DecodeOnlyCount = countCaseLabels(chpxBody, "recognized for decode coverage")
	groups[0].SemanticCount = groups[0].HandledCount - groups[0].DecodeOnlyCount
	groups[0].CanonicalCount, _ = countCaseLabels(poiCHP, "")
	groups[0].ParserCoveragePct = ratioPct(groups[0].HandledCount, groups[0].CanonicalCount)
	groups[0].SemanticCoveragePct = ratioPct(groups[0].SemanticCount, groups[0].CanonicalCount)

	groups[1].HandledCount, groups[1].DecodeOnlyCount = countCaseLabels(papxBody, "recognized for decode coverage")
	groups[1].SemanticCount = groups[1].HandledCount - groups[1].DecodeOnlyCount
	poiPAPCount, _ := countCaseLabels(poiPAP, "")
	poiTAPCount, _ := countCaseLabels(poiTAP, "")
	groups[1].CanonicalCount = poiPAPCount + poiTAPCount
	groups[1].ParserCoveragePct = ratioPct(groups[1].HandledCount, groups[1].CanonicalCount)
	groups[1].SemanticCoveragePct = ratioPct(groups[1].SemanticCount, groups[1].CanonicalCount)

	groups[2].HandledCount, groups[2].DecodeOnlyCount = countCaseLabels(sepxBody, "recognized for decode coverage")
	groups[2].SemanticCount = groups[2].HandledCount - groups[2].DecodeOnlyCount
	groups[2].CanonicalCount, _ = countCaseLabels(poiSEP, "")
	groups[2].ParserCoveragePct = ratioPct(groups[2].HandledCount, groups[2].CanonicalCount)
	groups[2].SemanticCoveragePct = ratioPct(groups[2].SemanticCount, groups[2].CanonicalCount)

	structures := []DocSpecCoverageCheckpoint{
		{ID: "fib-main-text", Label: "FIB + piece table text extraction", Status: "full", IRTarget: "Document.Paragraphs", Notes: "Word 97+ and pre-97 text paths are both active."},
		{ID: "stsh-styles", Label: "STSH paragraph + character styles", Status: "full", IRTarget: "Document.Styles / CharacterStyles", Notes: "Default style inheritance and effective formatting are wired."},
		{ID: "dop-settings", Label: "DOP document settings", Status: "partial", IRTarget: "DefaultTabStop / Footnote* / Endnote* / ZoomPercent / ViewMode / ProtectionEnabled / CreatedAt / ModifiedAt / LastPrintedAt / WordCount / CharCount / PageCount / ParagraphCount / MirrorMargins / TrackChangesEnabled / DocCompatibility", Notes: "Default tab stop, footnote/endnote numbering, zoom/view/protection, creation/modification/print timestamps, document statistics (words/chars/pages/paragraphs), mirror margins, track-changes flag, and 25+ compatibility flags are covered; remaining DOP fields (drawing grid, typography, mail-merge, version tracking) still need inventory-level expansion."},
		{ID: "summary-info", Label: "SummaryInformation property set", Status: "full", IRTarget: "Title / Subject / Author / Description / Keywords / LastModifiedBy", Notes: "PID string properties are parsed from \\x05SummaryInformation."},
		{ID: "sttbfffn", Label: "SttbfFfn / FFN font metadata", Status: "full", IRTarget: "FontFamilies", Notes: "Names plus family/pitch/charset/PANOSE hints are preserved."},
		{ID: "embedded-fonts", Label: "Embedded font payload streams", Status: "full", IRTarget: "EmbeddedFonts", Notes: "SttbTtmbd entries now lift embedded EOT font payloads from the WordDocument stream into family/style faces."},
		{ID: "headers-footers", Label: "Headers and footers", Status: "full", IRTarget: "Headers / Footers", Notes: "Word 97+ and pre-97 story extraction are both active."},
		{ID: "notes-comments", Label: "Footnotes, endnotes, comments", Status: "full", IRTarget: "Footnotes / Endnotes / Comments", Notes: "Story text, references, separator/continuation stories, and legacy comment author/date wiring are covered."},
		{ID: "bookmarks", Label: "Bookmark PLCs", Status: "full", IRTarget: "Bookmarks / BookmarkEnds", Notes: "Bookmark start/end PLCs feed run anchors and span resolution."},
		{ID: "fields", Label: "Legacy fields", Status: "partial", IRTarget: "Run.FieldKind / FieldInstr / FieldArg / Anchor / Target", Notes: "Common semantic fields resolve, but nested/structural fidelity is still lossy."},
		{ID: "form-fields", Label: "Legacy FORMTEXT / FORMCHECKBOX / FORMDROPDOWN", Status: "partial", IRTarget: "Run.FormField", Notes: "Binary FFData metadata (name, max length, checked state, dropdown items/default index) is now parsed from the Word Data stream via NilPICFAndBinData; keyword-driven fallback still applies when binary data is absent or malformed."},
		{ID: "revisions", Label: "Tracked changes", Status: "partial", IRTarget: "Run.Revision* / Run.PropertyRevision / Paragraph.PropertyRevision / TableRow.PropertyRevision / Table.PropertyRevision / Section.PropertyRevision", Notes: "Run-level insertion/deletion flags plus run/paragraph/section property-revision metadata are covered. Table rows lift row property revisions from both canonical sprmTPropRMark (TAPx) and the paragraph-level fallback (row-mark PAPX) on both structured binary and metadata-backed heuristic rescue paths; cells inherit the row revision when available. Table-level revisions are propagated heuristically when all rows in a binary-structured table share the same property revision. True cell-level and table-level SPRMs do not exist in the legacy spec, so this is the best attainable coverage."},
		{ID: "officeart-images", Label: "OfficeArt images", Status: "partial", IRTarget: "Run.Image", Notes: "Floating anchors preserve geometry/wrap/crop, but not every legacy image path is equally structured."},
		{ID: "officeart-shapes", Label: "OfficeArt non-image shapes", Status: "partial", IRTarget: "Run.Shape / Paragraph.Frame / Run.TextBox", Notes: "Non-image anchors now emit floating shapes with a small preset-geometry subset plus basic FSP/FOPT styling; custom geometry is still missing."},
	}

	report := &DocSpecCoverageReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		SprmGroups:  groups,
		Structures:  structures,
	}
	for _, group := range groups {
		report.Summary.CanonicalSPRMs += group.CanonicalCount
		report.Summary.HandledSPRMs += group.HandledCount
		report.Summary.DecodeOnlySPRMs += group.DecodeOnlyCount
		report.Summary.SemanticSPRMs += group.SemanticCount
	}
	report.Summary.ParserCoveragePct = ratioPct(report.Summary.HandledSPRMs, report.Summary.CanonicalSPRMs)
	report.Summary.SemanticCoveragePct = ratioPct(report.Summary.SemanticSPRMs, report.Summary.CanonicalSPRMs)
	for _, checkpoint := range structures {
		switch checkpoint.Status {
		case "full":
			report.Summary.StructureFullCount++
		case "partial":
			report.Summary.StructurePartialCount++
		case "missing":
			report.Summary.StructureMissingCount++
		}
	}
	sort.Slice(report.Structures, func(i, j int) bool {
		return report.Structures[i].ID < report.Structures[j].ID
	})
	return report, nil
}

func WriteDocSpecCoverageJSON(report *DocSpecCoverageReport, path string) error {
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

func WriteDocSpecCoverageMarkdown(report *DocSpecCoverageReport, path, jsonName string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# DOC Legacy Spec Coverage\n\n")
	b.WriteString("This report is generated from the current Word legacy parser source plus the checked-in Apache POI HWPF reference switches. It measures explicit handler surface, not user-visible rendering quality by itself.\n\n")
	fmt.Fprintf(&b, "Source JSON: `%s`\n\n", jsonName)
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Value |\n")
	b.WriteString("| --- | ---: |\n")
	fmt.Fprintf(&b, "| Canonical SPRMs | %d |\n", report.Summary.CanonicalSPRMs)
	fmt.Fprintf(&b, "| Explicitly handled SPRMs | %d |\n", report.Summary.HandledSPRMs)
	fmt.Fprintf(&b, "| Decode-only SPRMs | %d |\n", report.Summary.DecodeOnlySPRMs)
	fmt.Fprintf(&b, "| Semantic SPRMs | %d |\n", report.Summary.SemanticSPRMs)
	fmt.Fprintf(&b, "| Parser coverage | %.1f%% |\n", report.Summary.ParserCoveragePct)
	fmt.Fprintf(&b, "| Semantic coverage | %.1f%% |\n", report.Summary.SemanticCoveragePct)
	fmt.Fprintf(&b, "| Structure checkpoints (full / partial / missing) | %d / %d / %d |\n", report.Summary.StructureFullCount, report.Summary.StructurePartialCount, report.Summary.StructureMissingCount)
	b.WriteString("\n## SPRM Groups\n\n")
	b.WriteString("| Group | Our handlers | Decode-only | Semantic | POI/canonical | Parser % | Semantic % |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, group := range report.SprmGroups {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %.1f%% | %.1f%% |\n", group.Label, group.HandledCount, group.DecodeOnlyCount, group.SemanticCount, group.CanonicalCount, group.ParserCoveragePct, group.SemanticCoveragePct)
	}
	b.WriteString("\n## Structure Checkpoints\n\n")
	b.WriteString("| Surface | Status | IR target | Notes |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, checkpoint := range report.Structures {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", checkpoint.Label, checkpoint.Status, escapeDocSpecMarkdown(checkpoint.IRTarget), escapeDocSpecMarkdown(checkpoint.Notes))
	}
	b.WriteString("\n## Sources\n\n")
	for _, group := range report.SprmGroups {
		fmt.Fprintf(&b, "- `%s`: `%s` vs `%s`\n", group.Label, group.OurSource, group.CanonicalSource)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func extractFunctionBody(path, funcMarker string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	src := string(data)
	start := strings.Index(src, funcMarker)
	if start < 0 {
		return "", fmt.Errorf("marker %q not found in %s", funcMarker, path)
	}
	brace := strings.Index(src[start:], "{")
	if brace < 0 {
		return "", fmt.Errorf("opening brace not found for %q in %s", funcMarker, path)
	}
	brace += start
	depth := 0
	for i := brace; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[brace : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unclosed function body for %q in %s", funcMarker, path)
}

func countCaseLabels(body, decodeOnlyNeedle string) (handled int, decodeOnly int) {
	caseRE := regexp.MustCompile(`(?m)^\s*case\s+([^:\n]+):[^\n]*$`)
	matches := caseRE.FindAllStringSubmatch(body, -1)
	for _, match := range matches {
		line := match[0]
		labelText := match[1]
		labelCount := 0
		for _, label := range strings.Split(labelText, ",") {
			label = strings.TrimSpace(label)
			if label == "" {
				continue
			}
			labelCount++
		}
		handled += labelCount
		if decodeOnlyNeedle != "" && strings.Contains(line, decodeOnlyNeedle) {
			decodeOnly += labelCount
		}
	}
	return handled, decodeOnly
}

func ratioPct(num, den int) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) * 100 / float64(den)
}

func escapeDocSpecMarkdown(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}