package render

import (
	"strings"
	"testing"

	resolve "office/pkg/word/ir"
	"office/pkg/word/layout"
)

func TestParseTOCOptionsCustomStyles(t *testing.T) {
	options := parseTOCOptions(`TOC \\t "Custom Heading,1,Special Topic,2" \\n`)

	if !options.SuppressPageNumbers {
		t.Fatal("expected \\n to suppress page numbers")
	}
	if options.HeadingsOnly {
		t.Fatal("did not expect headings-only mode")
	}
	if got := options.CustomStyleLevels["CUSTOM HEADING"]; got != 1 {
		t.Fatalf("custom style level = %d, want 1", got)
	}
	if got := options.CustomStyleLevels["SPECIAL TOPIC"]; got != 2 {
		t.Fatalf("second custom style level = %d, want 2", got)
	}
}

func TestParseTOCOptionsCaptionAndSequenceSwitches(t *testing.T) {
	options := parseTOCOptions(`TOC \\a figures \\s chapter \\d ":"`)

	if !options.CaptionTextOnly {
		t.Fatal("expected \\a to enable caption-text-only mode")
	}
	if options.CaptionSequence != "figures" {
		t.Fatalf("caption sequence = %q, want %q", options.CaptionSequence, "figures")
	}
	if options.PageSequence != "chapter" {
		t.Fatalf("page sequence = %q, want %q", options.PageSequence, "chapter")
	}
	if options.Separator != ":" {
		t.Fatalf("sequence separator = %q, want %q", options.Separator, ":")
	}
}

func TestGenerateTOCUsesCustomStylesWithoutPageNumbers(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 4,
			Blocks: []layout.BlockBox{{
				Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					StyleName: "Custom Heading",
					Runs:      []resolve.Run{{Text: "Quarterly Review"}},
				}},
			}},
		}},
	}

	got := r.generateTOC(`TOC \\t "Custom Heading,1" \\n`)
	if got != "Quarterly Review" {
		t.Fatalf("generateTOC = %q, want %q", got, "Quarterly Review")
	}
}

func TestGenerateTOCHeadingsOnlySkipsTCEntries(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 2,
			Blocks: []layout.BlockBox{
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					StyleID: "Heading1",
					Runs:    []resolve.Run{{Text: "Heading One"}},
				}}},
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{FieldKind: "TC", FieldArg: `TC "Manual Entry" \\l 1`}},
				}}},
			},
		}},
	}

	got := r.generateTOC(`TOC \\o "1-1" \\h`)
	if got != "Heading One" {
		t.Fatalf("generateTOC with \\h = %q, want %q", got, "Heading One")
	}
}

func TestGenerateTOCFilterIncludesMatchingTCEntriesOnly(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 3,
			Blocks: []layout.BlockBox{{
				Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{
						{FieldKind: "TC", FieldArg: `TC "Kept Entry" \\l 1 \\f "keep"`},
						{FieldKind: "TC", FieldArg: `TC "Skipped Entry" \\l 1 \\f "skip"`},
					},
				}},
			}},
		}},
	}

	got := r.generateTOC(`TOC \\f "keep"`)
	if !strings.Contains(got, "Kept Entry") {
		t.Fatalf("generateTOC with \\f did not include matching entry: %q", got)
	}
	if strings.Contains(got, "Skipped Entry") {
		t.Fatalf("generateTOC with \\f included filtered-out entry: %q", got)
	}
}

func TestGenerateTOCBookmarkScopeFiltersHeadings(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 2,
			Blocks: []layout.BlockBox{
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index:     1,
					StyleID:   "Heading1",
					Bookmarks: []string{"scope"},
					Runs:      []resolve.Run{{Text: "First Heading"}},
				}}},
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index:     2,
					StyleID:   "Heading1",
					Bookmarks: []string{"scope"},
					Runs:      []resolve.Run{{Text: "Second Heading"}},
				}}},
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index:   3,
					StyleID: "Heading1",
					Runs:    []resolve.Run{{Text: "Outside Heading"}},
				}}},
			},
		}},
	}

	got := r.generateTOC(`TOC \\o "1-1" \\b "scope" \\n`)
	if got != "First Heading\nSecond Heading" {
		t.Fatalf("generateTOC with \\b scope = %q", got)
	}
}

func TestGenerateTOCPreserveTabsWithPageSeparator(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 7,
			Blocks: []layout.BlockBox{{
				Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index:   1,
					StyleID: "Heading1",
					Runs: []resolve.Run{
						{Text: "Chapter"},
						{Kind: "tab"},
						{Text: "Overview"},
					},
				}},
			}},
		}},
	}

	got := r.generateTOC(`TOC \\o "1-1" \\w \\p "-"`)
	if got != "Chapter\tOverview - 7" {
		t.Fatalf("generateTOC with \\w \\p = %q", got)
	}
}

func TestGenerateTOCCaptionTextOnly(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 12,
			Blocks: []layout.BlockBox{{
				Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index: 1,
					Runs: []resolve.Run{
						{Text: "Figure "},
						{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure`},
						{Text: ": Mercury"},
					},
				}},
			}},
		}},
	}

	got := r.generateTOC(`TOC \\a figures \\p "-"`)
	if got != "Mercury - 12" {
		t.Fatalf("generateTOC with \\a = %q", got)
	}
}

func TestGenerateTOCCaptionSequencePageNumbers(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 14,
			Blocks: []layout.BlockBox{
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index: 0,
					Runs: []resolve.Run{
						{Text: "Chapter "},
						{FieldKind: "SEQ", FieldArg: "chapter", FieldInstr: `SEQ chapter`},
					},
				}}},
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index: 1,
					Runs: []resolve.Run{
						{Text: "Figure "},
						{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure`},
						{Text: ": Mercury"},
					},
				}}},
			},
		}},
	}

	got := r.generateTOC(`TOC \\c "Figure" \\s chapter \\d ":" \\p "-"`)
	if got != "Figure 1: Mercury - 1:14" {
		t.Fatalf("generateTOC with \\c/\\s = %q", got)
	}
}

func TestGenerateIndexRangeBookmarkFiltersEntries(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 4,
			Blocks: []layout.BlockBox{
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index:     1,
					Bookmarks: []string{"scope"},
					Runs:      []resolve.Run{{FieldKind: "XE", FieldArg: `XE "Alpha"`}},
				}}},
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index:     2,
					Bookmarks: []string{"scope"},
					Runs:      []resolve.Run{{FieldKind: "XE", FieldArg: `XE "Beta"`}},
				}}},
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index: 3,
					Runs:  []resolve.Run{{FieldKind: "XE", FieldArg: `XE "Gamma"`}},
				}}},
			},
		}},
	}

	got := r.generateIndex(`INDEX \r "scope"`)
	if !strings.Contains(got, "Alpha") || !strings.Contains(got, "Beta") {
		t.Fatalf("generateIndex with \\r missing scoped entries: %q", got)
	}
	if strings.Contains(got, "Gamma") {
		t.Fatalf("generateIndex with \\r included out-of-scope entry: %q", got)
	}
}

func TestGenerateIndexSequenceDelimiter(t *testing.T) {
	r := &htmlRenderer{
		pages: []layout.Page{{
			Number: 6,
			Blocks: []layout.BlockBox{
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index: 0,
					Runs: []resolve.Run{
						{Text: "Chapter "},
						{FieldKind: "SEQ", FieldArg: "chapter", FieldInstr: `SEQ chapter \r 2`},
					},
				}}},
				{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
					Index: 1,
					Runs:  []resolve.Run{{FieldKind: "XE", FieldArg: `XE "Aristotle"`}},
				}}},
			},
		}},
	}

	got := r.generateIndex(`INDEX \s chapter \d "."`)
	if !strings.Contains(got, "Aristotle") {
		t.Fatalf("generateIndex with \\s missing entry: %q", got)
	}
	if !strings.Contains(got, "2.6") {
		t.Fatalf("generateIndex with \\s/\\d missing chapter-prefixed page number: %q", got)
	}
}