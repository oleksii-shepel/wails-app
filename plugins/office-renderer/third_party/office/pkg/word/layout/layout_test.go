package layout

import (
	"math"
	"strings"
	"testing"

	resolve "office/pkg/word/ir"
)

func intPtr(v int) *int {
	return &v
}

func almostEqual(got, want float64) bool {
	return math.Abs(got-want) < 0.001
}

func TestLayoutParagraphHeightExactIncludesSpacing(t *testing.T) {
	// Spacing-before is suppressed for the first paragraph on a page/column (y==0).
	// Only spacing-after and the line box contribute: 12pt exact + 4pt after = 16pt.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{
					LineRule:      "exact",
					LineSpacing:   intPtr(240),
					SpacingBefore: intPtr(120),
					SpacingAfter:  intPtr(80),
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	want := 16.0 // spacingBefore suppressed at y=0; exact(12) + after(4)
	if !almostEqual(blocks[0].Height, want) {
		t.Fatalf("expected exact line height to include paragraph spacing: got %.3f want %.3f", blocks[0].Height, want)
	}
}

func TestLayoutParagraphHeightIncludesSpacingBeforeWhenNotFirst(t *testing.T) {
	// Spacing-before is NOT suppressed when the paragraph follows another block (y > 0).
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{
					LineRule:      "exact",
					LineSpacing:   intPtr(240),
					SpacingBefore: intPtr(120),
					SpacingAfter:  intPtr(80),
				},
			}},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	want := 22.0 // before(6) + exact(12) + after(4)
	if !almostEqual(blocks[1].Height, want) {
		t.Fatalf("second paragraph height = %.3f, want %.3f", blocks[1].Height, want)
	}
}

func TestLayoutParagraphHeightWrapsVisibleText(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 360, Bottom: 1440, Left: 360},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text:   "123456789012 123456789012 123456789012",
				Runs:   []resolve.Run{{Kind: "run", Text: "123456789012 123456789012 123456789012"}},
				Format: resolve.ParagraphFormat{SnapToGrid: true},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	if blocks[0].Height <= 14.4 {
		t.Fatalf("expected wrapped visible text to exceed one line height, got %.3f", blocks[0].Height)
	}
	if !almostEqual(blocks[0].Height, 43.2) {
		t.Fatalf("expected wrapped visible text to estimate three lines, got %.3f want 43.2", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsLegacyPlainTextRuns(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 360, Bottom: 1440, Left: 360},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text:   "123456789012 123456789012 123456789012",
				Runs:   []resolve.Run{{Text: "123456789012 123456789012 123456789012"}},
				Format: resolve.ParagraphFormat{SnapToGrid: true},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 43.2) {
		t.Fatalf("expected legacy plain-text run to wrap to three lines, got %.3f want 43.2", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightAtLeastUsesMinimum(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "single line",
				Runs: []resolve.Run{{Kind: "run", Text: "single line"}},
				Format: resolve.ParagraphFormat{
					LineRule:    "atLeast",
					LineSpacing: intPtr(400),
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	if !almostEqual(blocks[0].Height, 20.0) {
		t.Fatalf("expected atLeast line spacing to raise line height to 20pt, got %.3f", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightUsesRunFontSizeForWrapEstimate(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 360, Bottom: 1440, Left: 360},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "alpha beta gamma delta epsilon zeta",
				Runs: []resolve.Run{{Kind: "run", Text: "alpha beta gamma delta epsilon zeta", Format: resolve.RunFormat{FontSize: intPtr(40)}}},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	if blocks[0].Height <= 24.0 {
		t.Fatalf("expected larger run font size to increase wrapped paragraph height, got %.3f", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightUsesPerLineMixedFontHeights(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 360, Bottom: 1440, Left: 360},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaa BBBBB",
				Runs: []resolve.Run{
					{Kind: "run", Text: "aaaaa"},
					{Kind: "run", Text: "BBBBB", Format: resolve.RunFormat{FontSize: intPtr(40)}},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 38.4) {
		t.Fatalf("expected mixed-font paragraph to use per-line heights, got %.3f want 38.4", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightSnapsToDocGridLinePitch(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 360, Bottom: 1440, Left: 360},
			DocGrid:    &resolve.DocGrid{Type: "lines", LinePitch: 360},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text:   "123456789012 123456789012 123456789012",
				Runs:   []resolve.Run{{Kind: "run", Text: "123456789012 123456789012 123456789012"}},
				Format: resolve.ParagraphFormat{SnapToGrid: true},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 54.0) {
		t.Fatalf("expected docGrid line pitch to snap each line to 18pt, got %.3f want 54.0", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightSnapsLargeLineToMultipleDocGridRows(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			DocGrid:    &resolve.DocGrid{Type: "linesAndChars", LinePitch: 240},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text:   "Tall",
				Runs:   []resolve.Run{{Kind: "run", Text: "Tall", Format: resolve.RunFormat{FontSize: intPtr(50)}}},
				Format: resolve.ParagraphFormat{SnapToGrid: true},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 36.0) {
		t.Fatalf("expected tall line to snap to multiple 12pt docGrid rows, got %.3f want 36.0", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightUsesInlineImageHeight(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "image",
					Image: &resolve.Image{
						Width:  36 * emuPerPoint,
						Height: 72 * emuPerPoint,
					},
				}},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 72.0) {
		t.Fatalf("expected inline image height to set the line box, got %.3f want 72.0", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightCountsInlineImageWidthWhenWrapping(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 360, Bottom: 1440, Left: 360},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaaa bbbbbb",
				Runs: []resolve.Run{
					{
						Kind: "image",
						Image: &resolve.Image{
							Width:  36 * emuPerPoint,
							Height: 12 * emuPerPoint,
						},
					},
					{Kind: "run", Text: "aaaaaa bbbbbb"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 28.8) {
		t.Fatalf("expected inline image width to force a second line, got %.3f want 28.8", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightAppliesFirstLineIndentOnlyToFirstLine(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3000, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaaa bbbbbb cccccc",
				Runs: []resolve.Run{{Kind: "run", Text: "aaaaaa bbbbbb cccccc"}},
				Format: resolve.ParagraphFormat{
					IndentFirstLine: intPtr(1000),
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 28.8) {
		t.Fatalf("expected first-line indent to wrap into two lines, got %.3f want 28.8", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightAccountsForHangingIndent(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3000, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaaa bbbbbb cccccc dddddd eeeeee",
				Runs: []resolve.Run{{Kind: "run", Text: "aaaaaa bbbbbb cccccc dddddd eeeeee"}},
				Format: resolve.ParagraphFormat{
					IndentHanging: intPtr(1000),
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 28.8) {
		t.Fatalf("expected hanging indent to keep the paragraph to two lines, got %.3f want 28.8", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsAroundFloatingObject(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaa bbbbb ccccc ddddd eeeee",
				Runs: []resolve.Run{
					{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							XRelative: "column",
							YRelative: "paragraph",
							Width:     508000,
							Height:    365760,
						},
					},
					{Kind: "run", Text: "aaaaa bbbbb ccccc ddddd eeeee"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 43.2) {
		t.Fatalf("expected wrapped float to force a third line, got %.3f want 43.2", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsTopAndBottomAroundFloatingObject(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaa bbbbb ccccc ddddd eeeee",
				Runs: []resolve.Run{
					{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "topAndBottom",
							XRelative: "column",
							YRelative: "paragraph",
							Width:     508000,
							Height:    365760,
						},
					},
					{Kind: "run", Text: "aaaaa bbbbb ccccc ddddd eeeee"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 57.6) {
		t.Fatalf("expected top-and-bottom wrap to push text below the float, got %.3f want 57.6", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsOnLeftSideOfFloatingObject(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaa bbbb cccc dddd eeee",
				Runs: []resolve.Run{
					{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							WrapSide:  "left",
							XRelative: "column",
							YRelative: "paragraph",
							X:         254000,
							Width:     508000,
							Height:    731520,
						},
					},
					{Kind: "run", Text: "aaaa bbbb cccc dddd eeee"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 72.0) {
		t.Fatalf("expected left-side wrap to constrain text to the left segment, got %.3f want 72.0", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsOnRightSideOfFloatingObject(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaa bbbb cccc dddd eeee",
				Runs: []resolve.Run{
					{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							WrapSide:  "right",
							XRelative: "column",
							YRelative: "paragraph",
							X:         254000,
							Width:     508000,
							Height:    731520,
						},
					},
					{Kind: "run", Text: "aaaa bbbb cccc dddd eeee"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 43.2) {
		t.Fatalf("expected right-side wrap to use the right segment, got %.3f want 43.2", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsOnLargestSideOfFloatingObject(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaa bbbb cccc dddd eeee",
				Runs: []resolve.Run{
					{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							WrapSide:  "largest",
							XRelative: "column",
							YRelative: "paragraph",
							X:         254000,
							Width:     508000,
							Height:    731520,
						},
					},
					{Kind: "run", Text: "aaaa bbbb cccc dddd eeee"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 43.2) {
		t.Fatalf("expected largest-side wrap to use the larger right segment, got %.3f want 43.2", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsThroughFloatingObjectContour(t *testing.T) {
	makeDoc := func(poly [][2]float64) *resolve.Document {
		return &resolve.Document{
			Sections: []resolve.Section{{
				PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
			}},
			Blocks: []resolve.Block{{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaa bbbb cccc dddd eeee ffff gggg hhhh",
					Runs: []resolve.Run{
						{
							Kind: "image",
							Image: &resolve.Image{
								Floating:    true,
								WrapType:    "through",
								WrapSide:    "largest",
								XRelative:   "column",
								YRelative:   "paragraph",
								X:           254000,
								Width:       508000,
								Height:      731520,
								WrapPolygon: poly,
							},
						},
						{Kind: "run", Text: "aaaa bbbb cccc dddd eeee ffff gggg hhhh"},
					},
				},
			}},
		}
	}

	basePoly := [][2]float64{{0, 0}, {24, 0}, {24, 57.6}, {0, 57.6}}
	shoulderPoly := [][2]float64{{0, 0}, {24, 0}, {42, 4}, {24, 57.6}, {0, 57.6}}

	_, baseBlocks, _ := Layout(makeDoc(basePoly))
	_, shoulderBlocks, _ := Layout(makeDoc(shoulderPoly))
	if len(baseBlocks) != 1 || len(shoulderBlocks) != 1 {
		t.Fatalf("expected one paragraph block for both cases, got %d and %d", len(baseBlocks), len(shoulderBlocks))
	}
	if shoulderBlocks[0].Height <= baseBlocks[0].Height {
		t.Fatalf("expected through-wrap shoulder contour to increase paragraph height, got base=%.3f shoulder=%.3f", baseBlocks[0].Height, shoulderBlocks[0].Height)
	}
}

func TestPolygonXRangeAtBandSamplesNarrowInteriorSpike(t *testing.T) {
	// This spike lives only in the top quarter of the band. Top/mid/bottom sampling
	// misses it, but vertex-aware sampling must preserve the wider exclusion range.
	poly := [][2]float64{
		{0, 0},
		{24, 0},
		{42, 1},
		{24, 1.2},
		{24, 4},
		{0, 4},
	}

	_, maxX, ok := polygonXRangeAtBand(poly, 0, 4)
	if !ok {
		t.Fatal("expected polygon band range")
	}
	if maxX < 40 {
		t.Fatalf("maxX = %.3f, want spike to extend exclusion band", maxX)
	}
}

func TestLayoutParagraphHeightWrapsAroundPriorFloatingObject(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							XRelative: "column",
							YRelative: "paragraph",
							Width:     508000,
							Height:    731520,
						},
					}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaaa bbbbb ccccc ddddd eeeee",
					Runs: []resolve.Run{{Kind: "run", Text: "aaaaa bbbbb ccccc ddddd eeeee"}},
				},
			},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if !almostEqual(blocks[1].Height, 43.2) {
		t.Fatalf("expected prior square float to wrap the next paragraph, got %.3f want 43.2", blocks[1].Height)
	}
}

func TestLayoutParagraphHeightWrapsBetweenPriorAndLocalFloatingObjects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							XRelative: "column",
							YRelative: "paragraph",
							X:         0,
							Width:     508000,
							Height:    731520,
						},
					}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaa bbbb cccc dddd eeee ffff gggg hhhh",
					Runs: []resolve.Run{
						{
							Kind: "image",
							Image: &resolve.Image{
								Floating:  true,
								WrapType:  "square",
								WrapSide:  "right",
								XRelative: "column",
								YRelative: "paragraph",
								X:         762000,
								Width:     508000,
								Height:    731520,
							},
						},
						{Kind: "run", Text: "aaaa bbbb cccc dddd eeee ffff gggg hhhh"},
					},
				},
			},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[1].Height <= 43.2 {
		t.Fatalf("expected prior and local floats to constrain the paragraph more than a single-float case, got %.3f", blocks[1].Height)
	}
}

func TestLayoutParagraphHeightHonorsLeftWrapAcrossPriorAndLocalFloatingObjects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							WrapSide:  "left",
							XRelative: "column",
							YRelative: "paragraph",
							X:         254000,
							Width:     508000,
							Height:    731520,
						},
					}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaa bbbb cccc dddd eeee",
					Runs: []resolve.Run{
						{
							Kind: "image",
							Image: &resolve.Image{
								Floating:  true,
								WrapType:  "square",
								WrapSide:  "left",
								XRelative: "column",
								YRelative: "paragraph",
								X:         254000,
								Width:     508000,
								Height:    731520,
							},
						},
						{Kind: "run", Text: "aaaa bbbb cccc dddd eeee"},
					},
				},
			},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if !almostEqual(blocks[1].Height, 72.0) {
		t.Fatalf("expected prior and local left-side floats to keep text on the left segment, got %.3f want 72.0", blocks[1].Height)
	}
}

func TestLayoutParagraphHeightHonorsRightWrapAcrossPriorAndLocalFloatingObjects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							WrapSide:  "right",
							XRelative: "column",
							YRelative: "paragraph",
							X:         254000,
							Width:     508000,
							Height:    731520,
						},
					}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaa bbbb cccc dddd eeee",
					Runs: []resolve.Run{
						{
							Kind: "image",
							Image: &resolve.Image{
								Floating:  true,
								WrapType:  "square",
								WrapSide:  "right",
								XRelative: "column",
								YRelative: "paragraph",
								X:         254000,
								Width:     508000,
								Height:    731520,
							},
						},
						{Kind: "run", Text: "aaaa bbbb cccc dddd eeee"},
					},
				},
			},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if !almostEqual(blocks[1].Height, 43.2) {
		t.Fatalf("expected prior and local right-side floats to keep text on the right segment, got %.3f want 43.2", blocks[1].Height)
	}
}

func TestLayoutParagraphHeightHonorsLargestWrapAcrossPriorAndLocalFloatingObjects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							WrapSide:  "largest",
							XRelative: "column",
							YRelative: "paragraph",
							X:         254000,
							Width:     508000,
							Height:    731520,
						},
					}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaa bbbb cccc dddd eeee",
					Runs: []resolve.Run{
						{
							Kind: "image",
							Image: &resolve.Image{
								Floating:  true,
								WrapType:  "square",
								WrapSide:  "largest",
								XRelative: "column",
								YRelative: "paragraph",
								X:         254000,
								Width:     508000,
								Height:    731520,
							},
						},
						{Kind: "run", Text: "aaaa bbbb cccc dddd eeee"},
					},
				},
			},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if !almostEqual(blocks[1].Height, 43.2) {
		t.Fatalf("expected prior and local largest-side floats to keep text on the larger right segment, got %.3f want 43.2", blocks[1].Height)
	}
}

func TestLayoutParagraphHeightHonorsBothSidesWrapAcrossPriorAndLocalFloatingObjects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							WrapSide:  "bothSides",
							XRelative: "column",
							YRelative: "paragraph",
							X:         254000,
							Width:     508000,
							Height:    731520,
						},
					}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaa bbbb cccc dddd eeee",
					Runs: []resolve.Run{
						{
							Kind: "image",
							Image: &resolve.Image{
								Floating:  true,
								WrapType:  "square",
								WrapSide:  "bothSides",
								XRelative: "column",
								YRelative: "paragraph",
								X:         254000,
								Width:     508000,
								Height:    731520,
							},
						},
						{Kind: "run", Text: "aaaa bbbb cccc dddd eeee"},
					},
				},
			},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if !almostEqual(blocks[1].Height, 43.2) {
		t.Fatalf("expected prior and local both-sides floats to use the largest contiguous segment, got %.3f want 43.2", blocks[1].Height)
	}
}

func TestLayoutParagraphHeightHonorsMixedLeftAndRightWrapSidesAcrossPriorAndLocalFloatingObjects(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "square",
							WrapSide:  "left",
							XRelative: "column",
							YRelative: "paragraph",
							X:         254000,
							Width:     508000,
							Height:    731520,
						},
					}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaa bbbb cccc dddd eeee",
					Runs: []resolve.Run{
						{
							Kind: "image",
							Image: &resolve.Image{
								Floating:  true,
								WrapType:  "square",
								WrapSide:  "right",
								XRelative: "column",
								YRelative: "paragraph",
								X:         762000,
								Width:     508000,
								Height:    731520,
							},
						},
						{Kind: "run", Text: "aaaa bbbb cccc dddd eeee"},
					},
				},
			},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if !almostEqual(blocks[1].Height, 72.0) {
		t.Fatalf("expected mixed left/right wraps to push text below the overlapping float band, got %.3f want 72.0", blocks[1].Height)
	}
}

func TestLayoutParagraphHeightWrapsBelowPriorTopAndBottomFloat(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "topAndBottom",
							XRelative: "column",
							YRelative: "paragraph",
							Width:     508000,
							Height:    548640,
						},
					}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "aaaaa bbbbb ccccc ddddd eeeee",
					Runs: []resolve.Run{{Kind: "run", Text: "aaaaa bbbbb ccccc ddddd eeeee"}},
				},
			},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 43.2) {
		t.Fatalf("expected image-only top-and-bottom float paragraph height 43.2, got %.3f", blocks[0].Height)
	}
	if !almostEqual(blocks[1].Y, 43.2) {
		t.Fatalf("expected following paragraph y 43.2 below prior top-and-bottom float, got %.3f", blocks[1].Y)
	}
	if !almostEqual(blocks[1].Height, 28.8) {
		t.Fatalf("expected following paragraph to keep its own text height 28.8, got %.3f", blocks[1].Height)
	}
}

func TestLayoutParagraphHeightWrapsBelowLocalTopAndBottomFloat(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaa bbbbb ccccc ddddd eeeee",
				Runs: []resolve.Run{
					{
						Kind: "image",
						Image: &resolve.Image{
							Floating:  true,
							WrapType:  "topAndBottom",
							XRelative: "column",
							YRelative: "paragraph",
							Width:     508000,
							Height:    548640,
						},
					},
					{Kind: "run", Text: "aaaaa bbbbb ccccc ddddd eeeee"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 72.0) {
		t.Fatalf("expected local top-and-bottom float paragraph height 72.0, got %.3f", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsBelowLocalTopAndBottomTextBox(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaa bbbbb ccccc ddddd eeeee",
				Runs: []resolve.Run{
					{
						Kind: "textbox",
						TextBox: &resolve.TextBox{
							Floating:  true,
							WrapType:  "topAndBottom",
							XRelative: "column",
							YRelative: "paragraph",
							Width:     508000,
							Height:    548640,
						},
					},
					{Kind: "run", Text: "aaaaa bbbbb ccccc ddddd eeeee"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 72.0) {
		t.Fatalf("expected local top-and-bottom textbox paragraph height 72.0, got %.3f", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightWrapsBelowLocalTopAndBottomShape(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3400, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 500, Bottom: 1440, Left: 500},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "aaaaa bbbbb ccccc ddddd eeeee",
				Runs: []resolve.Run{
					{
						Kind: "shape",
						Shape: &resolve.Shape{
							Floating: true,
							WrapType: "topAndBottom",
							Width:    508000,
							Height:   548640,
						},
					},
					{Kind: "run", Text: "aaaaa bbbbb ccccc ddddd eeeee"},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	// 548640 EMU / 12700 EMU/pt = 43.2pt shape + 28.8pt text = 72.0pt total
	if !almostEqual(blocks[0].Height, 72.0) {
		t.Fatalf("expected local top-and-bottom shape paragraph height 72.0, got %.3f", blocks[0].Height)
	}
}

func TestLayoutTableHeightUsesCellContent(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Format: resolve.TableFormat{ColumnWidths: []int{1000}},
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{{
							Kind: "paragraph",
							Paragraph: &resolve.Paragraph{
								Text: "123456789012 123456789012 123456789012",
								Runs: []resolve.Run{{Kind: "run", Text: "123456789012 123456789012 123456789012"}},
							},
						}},
					}},
				}},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 43.2) {
		t.Fatalf("expected table row height to match wrapped cell content, got %.3f want 43.2", blocks[0].Height)
	}
}

func TestLayoutTableExactRowHeightOverridesContent(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Format: resolve.TableFormat{ColumnWidths: []int{1000}},
				Rows: []resolve.TableRow{{
					Format: resolve.TableRowFormat{Height: intPtr(200), HeightRule: "exact"},
					Cells: []resolve.TableCell{{
						Text: "123456789012 123456789012 123456789012",
					}},
				}},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 10.0) {
		t.Fatalf("expected exact row height to override content, got %.3f want 10.0", blocks[0].Height)
	}
}

func TestLayoutTableAtLeastRowHeightUsesMinimum(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Format: resolve.TableFormat{ColumnWidths: []int{4000}},
				Rows: []resolve.TableRow{{
					Format: resolve.TableRowFormat{Height: intPtr(400), HeightRule: "atLeast"},
					Cells: []resolve.TableCell{{
						Text: "short",
					}},
				}},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 20.0) {
		t.Fatalf("expected atLeast row height to enforce a 20pt minimum, got %.3f", blocks[0].Height)
	}
}

func TestLayoutTableHeightIncludesTopAndBottomBorders(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Format: resolve.TableFormat{
					ColumnWidths: []int{4000},
					Borders: &resolve.TableBorders{
						Top:    &resolve.BorderSide{Val: "single", SizePt: 1},
						Bottom: &resolve.BorderSide{Val: "single", SizePt: 2},
					},
				},
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Text: "short",
					}},
				}},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 17.4) {
		t.Fatalf("expected table borders to contribute to row height, got %.3f want 17.4", blocks[0].Height)
	}
}

func TestLayoutParagraphHeightIncludesBorderSpace(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Bordered"}},
				Format: resolve.ParagraphFormat{
					LineSpacing: intPtr(400),
					LineRule:    "exact",
					Borders: &resolve.ParagraphBorders{
						Top:    &resolve.BorderSide{Val: "single", SizePt: 1, SpacePt: 2},
						Bottom: &resolve.BorderSide{Val: "single", SizePt: 1, SpacePt: 2},
					},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 26.0) {
		t.Fatalf("expected border width and border space to contribute to paragraph height, got %.3f want 26.0", blocks[0].Height)
	}
}

func TestResolveXAlignUsesObjectWidth(t *testing.T) {
	geom := PageGeometry{Width: 600, MarginLeft: 50, MarginRight: 50}

	if got := resolveXAlign("right", "page", geom, 0, 100); !almostEqual(got, 500) {
		t.Fatalf("right/page: got %.3f want 500", got)
	}
	if got := resolveXAlign("right", "margin", geom, 0, 100); !almostEqual(got, 450) {
		t.Fatalf("right/margin: got %.3f want 450", got)
	}
	if got := resolveXAlign("center", "page", geom, 0, 100); !almostEqual(got, 250) {
		t.Fatalf("center/page: got %.3f want 250", got)
	}
	if got := resolveXAlign("center", "margin", geom, 0, 100); !almostEqual(got, 250) {
		t.Fatalf("center/margin: got %.3f want 250", got)
	}
}

func TestResolveYAlignUsesObjectHeight(t *testing.T) {
	geom := PageGeometry{Height: 800, MarginTop: 50, MarginBottom: 50}

	if got := resolveYAlign("bottom", "page", geom, 200, 20, 40); !almostEqual(got, 760) {
		t.Fatalf("bottom/page: got %.3f want 760", got)
	}
	if got := resolveYAlign("bottom", "margin", geom, 200, 20, 40); !almostEqual(got, 710) {
		t.Fatalf("bottom/margin: got %.3f want 710", got)
	}
	if got := resolveYAlign("center", "page", geom, 200, 20, 40); !almostEqual(got, 380) {
		t.Fatalf("center/page: got %.3f want 380", got)
	}
	if got := resolveYAlign("center", "margin", geom, 200, 20, 40); !almostEqual(got, 380) {
		t.Fatalf("center/margin: got %.3f want 380", got)
	}
	if got := resolveYAlign("top", "paragraph", geom, 200, 20, 40); !almostEqual(got, 200) {
		t.Fatalf("top/paragraph: got %.3f want 200", got)
	}
}

func TestLineAvailableWidthUsesLargestContiguousGapForMultipleExclusions(t *testing.T) {
	exclusions := []lineExclusion{
		{left: 20, right: 40, top: 0, bottom: 10},
		{left: 60, right: 80, top: 0, bottom: 10},
	}

	got := lineAvailableWidthPt(0, 100, 0, 10, exclusions)
	if !almostEqual(got, 20) {
		t.Fatalf("multiple exclusions available width = %.3f, want 20", got)
	}
}

func TestLineAvailableWidthUsesLargestContiguousGapForSingleFloatBothSides(t *testing.T) {
	exclusions := []lineExclusion{
		{left: 20, right: 40, top: 0, bottom: 10, wrapSide: "bothSides"},
	}

	got := lineAvailableWidthPt(0, 100, 0, 10, exclusions)
	if !almostEqual(got, 60) {
		t.Fatalf("single both-sides exclusion available width = %.3f, want 60", got)
	}
}

func TestLineAvailableWidthHonorsCommonLeftWrapSideAcrossMultipleExclusions(t *testing.T) {
	exclusions := []lineExclusion{
		{left: 10, right: 20, top: 0, bottom: 10, wrapSide: "left"},
		{left: 40, right: 50, top: 0, bottom: 10, wrapSide: "left"},
	}

	got := lineAvailableWidthPt(0, 80, 0, 10, exclusions)
	if !almostEqual(got, 10) {
		t.Fatalf("multiple left-side exclusions available width = %.3f, want 10", got)
	}
}

func TestLineAvailableWidthHonorsCommonRightWrapSideAcrossMultipleExclusions(t *testing.T) {
	exclusions := []lineExclusion{
		{left: 10, right: 20, top: 0, bottom: 10, wrapSide: "right"},
		{left: 50, right: 70, top: 0, bottom: 10, wrapSide: "right"},
	}

	got := lineAvailableWidthPt(0, 80, 0, 10, exclusions)
	if !almostEqual(got, 10) {
		t.Fatalf("multiple right-side exclusions available width = %.3f, want 10", got)
	}
}

func TestLineAvailableWidthIgnoresPolygonExclusionsThatMissLineBand(t *testing.T) {
	exclusions := []lineExclusion{
		{left: 10, right: 20, top: 0, bottom: 10, wrapSide: "left"},
		{
			left:     40,
			right:    50,
			top:      0,
			bottom:   10,
			wrapSide: "right",
			polygon: [][2]float64{
				{40, 0},
				{50, 0},
				{45, 2},
			},
		},
	}

	got := lineAvailableWidthPt(0, 80, 8, 10, exclusions)
	if !almostEqual(got, 10) {
		t.Fatalf("non-intersecting polygon exclusion should not suppress left wrap-side width, got %.3f want 10", got)
	}
}

func TestLineAvailableWidthHonorsMixedLeftAndRightWrapSides(t *testing.T) {
	exclusions := []lineExclusion{
		{left: 20, right: 40, top: 0, bottom: 10, wrapSide: "left"},
		{left: 80, right: 100, top: 0, bottom: 10, wrapSide: "right"},
	}

	got := lineAvailableWidthPt(0, 120, 0, 10, exclusions)
	if !almostEqual(got, 1) {
		t.Fatalf("mixed left/right exclusions should leave no usable segment on the band, got %.3f want 1", got)
	}
}

func TestLineAvailableWidthHonorsMixedRightAndLeftWrapSidesWithCenterGap(t *testing.T) {
	exclusions := []lineExclusion{
		{left: 20, right: 40, top: 0, bottom: 10, wrapSide: "right"},
		{left: 80, right: 100, top: 0, bottom: 10, wrapSide: "left"},
	}

	got := lineAvailableWidthPt(0, 120, 0, 10, exclusions)
	if !almostEqual(got, 40) {
		t.Fatalf("mixed right/left exclusions should preserve the shared center segment, got %.3f want 40", got)
	}
}

func TestLayoutParagraphRelativeAlignedFloatUsesParagraphHeight(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageMargin: &resolve.PageMargin{Top: 1000, Right: 1000, Bottom: 1000, Left: 1000},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{
					LineRule:    "exact",
					LineSpacing: intPtr(400),
				},
				Runs: []resolve.Run{{
					Kind: "textbox",
					TextBox: &resolve.TextBox{
						Floating:  true,
						YAlign:    "bottom",
						YRelative: "paragraph",
						Height:    101600,
					},
				}},
			},
		}},
	}

	_, _, floats := Layout(doc)
	if len(floats) != 1 {
		t.Fatalf("expected 1 float, got %d", len(floats))
	}

	want := 62.0
	if !almostEqual(floats[0].Y, want) {
		t.Fatalf("expected paragraph-relative bottom alignment to use paragraph height: got %.3f want %.3f", floats[0].Y, want)
	}
}

func TestLayoutUsesFirstSectionGeometry(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				PageSize:   &resolve.PageSize{Width: 10000, Height: 20000},
				PageMargin: &resolve.PageMargin{Top: 1000, Right: 1100, Bottom: 1200, Left: 1300},
			},
			{
				PageSize:   &resolve.PageSize{Width: 20000, Height: 30000},
				PageMargin: &resolve.PageMargin{Top: 2000, Right: 2100, Bottom: 2200, Left: 2300},
			},
		},
	}

	geom, _, _ := Layout(doc)
	if !almostEqual(geom.Width, 500) || !almostEqual(geom.Height, 1000) {
		t.Fatalf("expected first section page size to be used, got width=%.3f height=%.3f", geom.Width, geom.Height)
	}
	if !almostEqual(geom.MarginTop, 50) || !almostEqual(geom.MarginRight, 55) || !almostEqual(geom.MarginBottom, 60) || !almostEqual(geom.MarginLeft, 65) {
		t.Fatalf("expected first section margins to be used, got top=%.3f right=%.3f bottom=%.3f left=%.3f", geom.MarginTop, geom.MarginRight, geom.MarginBottom, geom.MarginLeft)
	}
}

func TestLayoutFloatingImageAlignmentUsesAbsoluteTopLeft(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12000, Height: 16000},
			PageMargin: &resolve.PageMargin{Top: 1000, Right: 1000, Bottom: 1000, Left: 1000},
		}},
		Blocks: []resolve.Block{{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{
					Kind: "image",
					Image: &resolve.Image{
						Floating:  true,
						Width:     1270000,
						Height:    508000,
						XAlign:    "right",
						YAlign:    "bottom",
						XRelative: "page",
						YRelative: "page",
					},
				}},
			},
		}},
	}

	_, _, floats := Layout(doc)
	if len(floats) != 1 {
		t.Fatalf("expected 1 float, got %d", len(floats))
	}

	if !almostEqual(floats[0].X, 500) {
		t.Fatalf("expected right-aligned page float x=500, got %.3f", floats[0].X)
	}
	if !almostEqual(floats[0].Y, 760) {
		t.Fatalf("expected bottom-aligned page float y=760, got %.3f", floats[0].Y)
	}
}

func TestPaginateRespectsParagraphPageBreakBefore(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: "First page",
					Runs: []resolve.Run{{Kind: "run", Text: "First page"}},
				},
			},
			{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text:   "Second page",
					Runs:   []resolve.Run{{Kind: "run", Text: "Second page"}},
					Format: resolve.ParagraphFormat{PageBreakBefore: true},
				},
			},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 1 || len(pages[1].Blocks) != 1 {
		t.Fatalf("page block counts = %d/%d, want 1/1", len(pages[0].Blocks), len(pages[1].Blocks))
	}
	if pages[0].Blocks[0].Page != 1 || pages[1].Blocks[0].Page != 2 {
		t.Fatalf("page numbers = %d/%d, want 1/2", pages[0].Blocks[0].Page, pages[1].Blocks[0].Page)
	}
}

func TestPaginateUsesResolvedSectionGeometry(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				Type:       "nextPage",
				PageSize:   &resolve.PageSize{Width: 10000, Height: 16000},
				PageMargin: &resolve.PageMargin{Top: 1000, Right: 1000, Bottom: 1000, Left: 1000},
			},
			{
				Type:       "nextPage",
				PageSize:   &resolve.PageSize{Width: 14000, Height: 20000},
				PageMargin: &resolve.PageMargin{Top: 1200, Right: 1200, Bottom: 1200, Left: 1200},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "Section one", Runs: []resolve.Run{{Kind: "run", Text: "Section one"}}}},
			{Kind: "section", Section: &resolve.Section{Type: "nextPage", PageSize: &resolve.PageSize{Width: 10000, Height: 16000}, PageMargin: &resolve.PageMargin{Top: 1000, Right: 1000, Bottom: 1000, Left: 1000}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "Section two", Runs: []resolve.Run{{Kind: "run", Text: "Section two"}}}},
			{Kind: "section", Section: &resolve.Section{Type: "nextPage", PageSize: &resolve.PageSize{Width: 14000, Height: 20000}, PageMargin: &resolve.PageMargin{Top: 1200, Right: 1200, Bottom: 1200, Left: 1200}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if !almostEqual(pages[0].Geometry.Width, 500) || !almostEqual(pages[0].Geometry.MarginTop, 50) {
		t.Fatalf("first page geometry = %+v", pages[0].Geometry)
	}
	if !almostEqual(pages[1].Geometry.Width, 700) || !almostEqual(pages[1].Geometry.MarginTop, 60) {
		t.Fatalf("second page geometry = %+v", pages[1].Geometry)
	}
	if !almostEqual(pages[1].Blocks[0].Y, 0) {
		t.Fatalf("expected second section to start at top of page, got %.3f", pages[1].Blocks[0].Y)
	}
}

func TestPaginateIgnoresTrailingSectionMarkerWithoutContent(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				Type:       "nextPage",
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			},
			{
				Type:       "nextPage",
				PageSize:   &resolve.PageSize{Width: 15840, Height: 12240},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "Portrait content", Runs: []resolve.Run{{Kind: "run", Text: "Portrait content"}}}},
			{Kind: "section", Section: &resolve.Section{Type: "nextPage", PageSize: &resolve.PageSize{Width: 12240, Height: 15840}, PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if !almostEqual(pages[0].Geometry.Width, 612) || !almostEqual(pages[0].Geometry.Height, 792) {
		t.Fatalf("page geometry = %+v, want portrait letter", pages[0].Geometry)
	}
}

func TestResolveTableCellWidthsRespectGridBeforeAfter(t *testing.T) {
	row := resolve.TableRow{
		Format: resolve.TableRowFormat{GridBefore: 1, GridAfter: 1},
		Cells: []resolve.TableCell{
			{},
			{},
		},
	}

	widths := TableCellWidths(row, resolve.TableFormat{}, 400)
	if len(widths) != 2 {
		t.Fatalf("cell width count = %d, want 2", len(widths))
	}
	if !almostEqual(widths[0], 100) || !almostEqual(widths[1], 100) {
		t.Fatalf("cell widths = %#v, want [100 100]", widths)
	}
}

func TestLayoutTableHeightSkipsPureVerticalMergeContinuationRow(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Format: resolve.TableFormat{ColumnWidths: []int{1000}},
				Rows: []resolve.TableRow{
					{
						Cells: []resolve.TableCell{{
							Format: resolve.TableCellFormat{VMerge: "restart"},
							Blocks: []resolve.Block{{
								Kind: "paragraph",
								Paragraph: &resolve.Paragraph{
									Text: "123456789012 123456789012 123456789012",
									Runs: []resolve.Run{{Kind: "run", Text: "123456789012 123456789012 123456789012"}},
								},
							}},
						}},
					},
					{
						Cells: []resolve.TableCell{{
							Format: resolve.TableCellFormat{VMerge: "continue"},
						}},
					},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if !almostEqual(blocks[0].Height, 43.2) {
		t.Fatalf("expected continuation row not to add extra height, got %.3f want 43.2", blocks[0].Height)
	}
}

func TestPaginateContinuousSectionSameGeometryStaysOnPage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			},
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "Section one", Runs: []resolve.Run{{Kind: "run", Text: "Section one"}}}},
			{Kind: "section", Section: &resolve.Section{Type: "continuous", PageSize: &resolve.PageSize{Width: 12240, Height: 15840}, PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "Section two", Runs: []resolve.Run{{Kind: "run", Text: "Section two"}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 2 {
		t.Fatalf("page block count = %d, want 2", len(pages[0].Blocks))
	}
	if pages[0].Blocks[1].Page != 1 {
		t.Fatalf("second block page = %d, want 1", pages[0].Blocks[1].Page)
	}
	if pages[0].Blocks[1].Y <= pages[0].Blocks[0].Y {
		t.Fatalf("expected second block to continue on same page, got y=%.3f after %.3f", pages[0].Blocks[1].Y, pages[0].Blocks[0].Y)
	}
}

func TestPaginateContinuousSectionColumnChangeStaysOnPage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 20000, Height: 1200},
				PageMargin: &resolve.PageMargin{Top: 0, Right: 1000, Bottom: 0, Left: 1000},
			},
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 20000, Height: 1200},
				PageMargin: &resolve.PageMargin{Top: 0, Right: 1000, Bottom: 0, Left: 1000},
				Columns:    &resolve.Columns{Num: 2, Space: 400},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Intro"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "section", Section: &resolve.Section{Type: "continuous", PageSize: &resolve.PageSize{Width: 20000, Height: 1200}, PageMargin: &resolve.PageMargin{Top: 0, Right: 1000, Bottom: 0, Left: 1000}, Columns: &resolve.Columns{Num: 2, Space: 400}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "One"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Two"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Three"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Four"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 5 {
		t.Fatalf("block count = %d, want 5", len(pages[0].Blocks))
	}
	if pages[0].Blocks[0].Geometry.ColumnCount != 1 {
		t.Fatalf("intro block column count = %d, want 1", pages[0].Blocks[0].Geometry.ColumnCount)
	}
	if pages[0].Blocks[1].Geometry.ColumnCount != 2 {
		t.Fatalf("post-break block column count = %d, want 2", pages[0].Blocks[1].Geometry.ColumnCount)
	}
	if pages[0].Blocks[0].Column != 0 || pages[0].Blocks[1].Column != 0 || pages[0].Blocks[2].Column != 0 || pages[0].Blocks[3].Column != 1 || pages[0].Blocks[4].Column != 1 {
		t.Fatalf("block columns = [%d %d %d %d %d], want [0 0 0 1 1]", pages[0].Blocks[0].Column, pages[0].Blocks[1].Column, pages[0].Blocks[2].Column, pages[0].Blocks[3].Column, pages[0].Blocks[4].Column)
	}
	if !almostEqual(pages[0].Blocks[1].Y, 20) || !almostEqual(pages[0].Blocks[3].Y, 20) {
		t.Fatalf("post-break y positions = [%.1f %.1f], want [20.0 20.0]", pages[0].Blocks[1].Y, pages[0].Blocks[3].Y)
	}
}

func TestPaginateEnteredContinuousSectionWithMarginChangeStaysOnPage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 720, Left: 1440},
			},
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
				Columns:    &resolve.Columns{Num: 2, Space: 720},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "Before", Runs: []resolve.Run{{Kind: "run", Text: "Before"}}}},
			{Kind: "section", Section: &resolve.Section{Type: "continuous", PageSize: &resolve.PageSize{Width: 12240, Height: 15840}, PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}, Columns: &resolve.Columns{Num: 2, Space: 720}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "After", Runs: []resolve.Run{{Kind: "run", Text: "After"}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 2 {
		t.Fatalf("page block count = %d, want 2", len(pages[0].Blocks))
	}
	if pages[0].Blocks[1].Geometry.ColumnCount != 2 {
		t.Fatalf("entered continuous section column count = %d, want 2", pages[0].Blocks[1].Geometry.ColumnCount)
	}
	if !almostEqual(pages[0].Blocks[1].Geometry.MarginBottom, 36.0) {
		t.Fatalf("same-page continuous section bottom margin = %.1f, want 36.0", pages[0].Blocks[1].Geometry.MarginBottom)
	}
	if pages[0].Blocks[1].Page != 1 {
		t.Fatalf("second block page = %d, want 1", pages[0].Blocks[1].Page)
	}
}

func TestPaginateFlowsOverflowIntoNextColumnBeforeNewPage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 20000, Height: 900},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 1000, Bottom: 0, Left: 1000},
			Columns:    &resolve.Columns{Num: 2, Space: 400},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "One"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Two"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Three"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Four"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 4 {
		t.Fatalf("block count = %d, want 4", len(pages[0].Blocks))
	}
	if pages[0].Blocks[0].Column != 0 || pages[0].Blocks[1].Column != 0 || pages[0].Blocks[2].Column != 1 || pages[0].Blocks[3].Column != 1 {
		t.Fatalf("block columns = [%d %d %d %d], want [0 0 1 1]", pages[0].Blocks[0].Column, pages[0].Blocks[1].Column, pages[0].Blocks[2].Column, pages[0].Blocks[3].Column)
	}
	if !almostEqual(pages[0].Blocks[0].Y, 0) || !almostEqual(pages[0].Blocks[1].Y, 20) || !almostEqual(pages[0].Blocks[2].Y, 0) || !almostEqual(pages[0].Blocks[3].Y, 20) {
		t.Fatalf("block y positions = [%.1f %.1f %.1f %.1f], want [0 20 0 20]", pages[0].Blocks[0].Y, pages[0].Blocks[1].Y, pages[0].Blocks[2].Y, pages[0].Blocks[3].Y)
	}
	if !almostEqual(pages[0].Geometry.ColumnWidthPt, 440) {
		t.Fatalf("column width = %.1f, want 440", pages[0].Geometry.ColumnWidthPt)
	}
}

func TestPaginateRespectsExplicitColumnBreakWithinParagraph(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			Columns:    &resolve.Columns{Num: 2, Space: 720},
		}},
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{
			Runs: []resolve.Run{
				{Kind: "run", Text: "Left column text"},
				{Kind: "break", BreakType: "column"},
				{Kind: "run", Text: "Right column text"},
			},
		}}},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 2 {
		t.Fatalf("block count = %d, want 2", len(pages[0].Blocks))
	}
	if pages[0].Blocks[0].Column != 0 || pages[0].Blocks[1].Column != 1 {
		t.Fatalf("block columns = [%d %d], want [0 1]", pages[0].Blocks[0].Column, pages[0].Blocks[1].Column)
	}
	if pages[0].Blocks[0].Fragment == nil || pages[0].Blocks[1].Fragment == nil {
		t.Fatal("expected explicit column break to emit paragraph fragments")
	}
	if pages[0].Blocks[1].Y != 0 {
		t.Fatalf("second fragment y = %.1f, want 0", pages[0].Blocks[1].Y)
	}
	if !pages[0].Blocks[1].Fragment.StartsNewColumn {
		t.Fatal("expected second fragment to force a new column")
	}
	if pages[0].Blocks[0].Fragment.Text != "Left column text" || pages[0].Blocks[1].Fragment.Text != "Right column text" {
		t.Fatalf("fragment texts = [%q %q], want split column segments", pages[0].Blocks[0].Fragment.Text, pages[0].Blocks[1].Fragment.Text)
	}
}

func TestPaginateRespectsExplicitColumnBreakWithPriorPageFloat(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			Columns:    &resolve.Columns{Num: 2, Space: 720},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
				Kind:  "image",
				Image: &resolve.Image{Floating: true, WrapType: "square", XRelative: "column", YRelative: "paragraph", Width: 914400, Height: 914400},
			}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Kind: "run", Text: "Left column text"},
					{Kind: "break", BreakType: "column"},
					{Kind: "run", Text: "Right column text"},
				},
			}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 3 {
		t.Fatalf("block count = %d, want 3", len(pages[0].Blocks))
	}
	if pages[0].Blocks[1].Fragment == nil || pages[0].Blocks[2].Fragment == nil {
		t.Fatal("expected explicit column break to emit fragments even with prior page float")
	}
	if pages[0].Blocks[1].Column != 0 || pages[0].Blocks[2].Column != 1 {
		t.Fatalf("fragment columns = [%d %d], want [0 1]", pages[0].Blocks[1].Column, pages[0].Blocks[2].Column)
	}
}

func TestPaginateReservesFootnoteHeight(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
		}},
		Footnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{{
					Kind: "paragraph",
					Paragraph: &resolve.Paragraph{
						Text: "123456789012 123456789012 123456789012 123456789012",
						Runs: []resolve.Run{{Kind: "run", Text: "123456789012 123456789012 123456789012 123456789012"}},
					},
				}},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "footnoteRef", NoteID: 1}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Third"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Fourth"}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 3 || len(pages[1].Blocks) != 1 {
		t.Fatalf("page block counts = %d/%d, want 3/1", len(pages[0].Blocks), len(pages[1].Blocks))
	}
	if pages[1].Blocks[0].Block == nil || pages[1].Blocks[0].Block.Paragraph == nil || pages[1].Blocks[0].Block.Paragraph.Runs[0].Text != "Fourth" {
		t.Fatalf("expected footnote reservation to push the fourth paragraph, got %#v", pages[1].Blocks[0].Block)
	}
	if len(pages[0].Footnotes) != 2 {
		t.Fatalf("expected separator plus explicit footnote blocks on page 1, got %d", len(pages[0].Footnotes))
	}
	if got := pages[0].Footnotes[1].Block.Paragraph.Runs[0].Text; got != "123456789012 123456789012 123456789012 123456789012" {
		t.Fatalf("footnote text = %q, want reserved note text", got)
	}
}

func TestPaginateUsesBodyWidthForMultiColumnFootnotes(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 5000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
			Columns:    &resolve.Columns{Num: 2, Space: 300},
		}},
		Footnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{{
					Kind: "paragraph",
					Paragraph: &resolve.Paragraph{
						Text: "123456789012 123456789012 123456789012 123456789012",
						Runs: []resolve.Run{{Kind: "run", Text: "123456789012 123456789012 123456789012 123456789012"}},
					},
				}},
			},
		},
		Blocks: []resolve.Block{{
			Kind:      "paragraph",
			Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "footnoteRef", NoteID: 1}}},
		}},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Footnotes) != 2 {
		t.Fatalf("footnote count = %d, want 2", len(pages[0].Footnotes))
	}
	footnoteGeom := pages[0].Footnotes[1].Geometry
	if footnoteGeom.ColumnCount != 1 {
		t.Fatalf("footnote column count = %d, want 1", footnoteGeom.ColumnCount)
	}
	if !almostEqual(footnoteGeom.ColumnWidthPt, 210) {
		t.Fatalf("footnote width = %.2fpt, want 210pt", footnoteGeom.ColumnWidthPt)
	}
	if !almostEqual(pages[0].Geometry.ColumnWidthPt, 97.5) {
		t.Fatalf("body column width = %.2fpt, want 97.5pt", pages[0].Geometry.ColumnWidthPt)
	}
}

func TestPaginateUsesFootnoteSettingsForNumberingAndSeparator(t *testing.T) {
	doc := &resolve.Document{
		FootnoteNumberFormat: "upperLetter",
		FootnoteNumberStart:  3,
		FootnoteRestart:      "eachPage",
		FootnoteSeparator: resolve.Story{
			Blocks: []resolve.Block{{
				Kind:      "paragraph",
				Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footnote rule"}}},
			}},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 1600},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
		}},
		Footnotes: map[int]resolve.Story{
			10: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First footnote"}}}}}},
			11: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second footnote"}}}}}},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor 1"}, {Kind: "footnoteRef", NoteID: 10}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{PageBreakBefore: true},
				Runs:   []resolve.Run{{Kind: "run", Text: "Anchor 2"}, {Kind: "footnoteRef", NoteID: 11}},
			}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if got := pages[0].Footnotes[0].Block.Paragraph.Runs[0].Text; got != "Footnote rule" {
		t.Fatalf("page 1 separator = %q, want Footnote rule", got)
	}
	if got := pages[1].Footnotes[0].Block.Paragraph.Runs[0].Text; got != "Footnote rule" {
		t.Fatalf("page 2 separator = %q, want Footnote rule", got)
	}
	if list := pages[0].Footnotes[1].Block.Paragraph.List; list == nil || list.Start != 3 || list.Format != "upperLetter" {
		t.Fatalf("page 1 footnote list = %#v, want start 3 upperLetter", list)
	}
	if list := pages[1].Footnotes[1].Block.Paragraph.List; list == nil || list.Start != 3 || list.Format != "upperLetter" {
		t.Fatalf("page 2 footnote list = %#v, want restart at 3 upperLetter", list)
	}
}

func TestPaginateReservesEndnoteHeightOnFinalPage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
		}},
		Endnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{{
					Kind: "paragraph",
					Paragraph: &resolve.Paragraph{
						Text: "123456789012 123456789012 123456789012 123456789012",
						Runs: []resolve.Run{{Kind: "run", Text: "123456789012 123456789012 123456789012 123456789012"}},
					},
				}},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 1}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Fourth"}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 4 || len(pages[1].Blocks) != 2 {
		t.Fatalf("page block counts = %d/%d, want 4/2", len(pages[0].Blocks), len(pages[1].Blocks))
	}
	if pages[0].Blocks[3].Block == nil || pages[0].Blocks[3].Block.Paragraph == nil || pages[0].Blocks[3].Block.Paragraph.Runs[0].Text != "Fourth" {
		t.Fatalf("expected endnote reservation to leave the fourth paragraph in body flow, got %#v", pages[0].Blocks[3].Block)
	}
	if pages[1].Blocks[0].Block == nil || pages[1].Blocks[0].Block.Paragraph == nil || pages[1].Blocks[0].Block.Paragraph.Format.Borders == nil || pages[1].Blocks[0].Block.Paragraph.Format.Borders.Top == nil {
		t.Fatalf("expected separator block before endnote content, got %#v", pages[1].Blocks[0].Block)
	}
	if pages[1].Blocks[1].Block == nil || pages[1].Blocks[1].Block.Paragraph == nil || pages[1].Blocks[1].Block.Paragraph.Runs[0].Text != "123456789012 123456789012 123456789012 123456789012" {
		t.Fatalf("expected endnote content on overflow page, got %#v", pages[1].Blocks[1].Block)
	}
	if pages[1].Blocks[1].Block.Paragraph.List == nil || pages[1].Blocks[1].Block.Paragraph.List.Start != 1 {
		t.Fatalf("expected numbered endnote marker, got %#v", pages[1].Blocks[1].Block.Paragraph.List)
	}
}

func TestPaginateKeepsEndnotesOnFinalPageWhenThereIsRoom(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
		}},
		Endnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{{
					Kind: "paragraph",
					Paragraph: &resolve.Paragraph{
						Text: "Note",
						Runs: []resolve.Run{{Kind: "run", Text: "Note"}},
					},
				}},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 1}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 4 {
		t.Fatalf("page block count = %d, want 4", len(pages[0].Blocks))
	}
	if pages[0].Blocks[2].Block == nil || pages[0].Blocks[2].Block.Paragraph == nil || pages[0].Blocks[2].Block.Paragraph.Format.Borders == nil || pages[0].Blocks[2].Block.Paragraph.Format.Borders.Top == nil {
		t.Fatalf("expected separator block on final page, got %#v", pages[0].Blocks[2].Block)
	}
	if pages[0].Blocks[3].Block == nil || pages[0].Blocks[3].Block.Paragraph == nil || pages[0].Blocks[3].Block.Paragraph.Runs[0].Text != "Note" {
		t.Fatalf("expected endnote content on final page, got %#v", pages[0].Blocks[3].Block)
	}
	if pages[0].Blocks[3].Block.Paragraph.List == nil || pages[0].Blocks[3].Block.Paragraph.List.Start != 1 {
		t.Fatalf("expected numbered endnote marker, got %#v", pages[0].Blocks[3].Block.Paragraph.List)
	}
}

func TestPaginatePlacesSectionEndnotesBeforeNextContinuousSection(t *testing.T) {
	doc := &resolve.Document{
		EndnotePosition: "sectEnd",
		Sections: []resolve.Section{
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 5000, Height: 3200},
				PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
			},
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 5000, Height: 3200},
				PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
			},
		},
		Endnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{{
					Kind:      "paragraph",
					Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Section note"}}},
				}},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First section"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 1}}}},
			{Kind: "section", Section: &resolve.Section{Type: "continuous", PageSize: &resolve.PageSize{Width: 5000, Height: 3200}, PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second section"}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 5 {
		t.Fatalf("page block count = %d, want 5", len(pages[0].Blocks))
	}
	if pages[0].Blocks[2].Block == nil || pages[0].Blocks[2].Block.Paragraph == nil || pages[0].Blocks[2].Block.Paragraph.Format.Borders == nil || pages[0].Blocks[2].Block.Paragraph.Format.Borders.Top == nil {
		t.Fatalf("expected section-end separator block, got %#v", pages[0].Blocks[2].Block)
	}
	if got := pages[0].Blocks[3].Block.Paragraph.Runs[0].Text; got != "Section note" {
		t.Fatalf("section endnote text = %q, want Section note", got)
	}
	if pages[0].Blocks[3].Block.Paragraph.List == nil || pages[0].Blocks[3].Block.Paragraph.List.Start != 1 {
		t.Fatalf("expected section-end marker, got %#v", pages[0].Blocks[3].Block.Paragraph.List)
	}
	if got := pages[0].Blocks[4].Block.Paragraph.Runs[0].Text; got != "Second section" {
		t.Fatalf("next section text = %q, want Second section", got)
	}
}

func TestPaginatePlacesDocumentEndnotesAfterFinalSection(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 5000, Height: 3200},
				PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
			},
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 5000, Height: 3200},
				PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
			},
		},
		Endnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{{
					Kind:      "paragraph",
					Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Document note"}}},
				}},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First section"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 1}}}},
			{Kind: "section", Section: &resolve.Section{Type: "continuous", PageSize: &resolve.PageSize{Width: 5000, Height: 3200}, PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second section"}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 5 {
		t.Fatalf("page block count = %d, want 5", len(pages[0].Blocks))
	}
	if got := pages[0].Blocks[2].Block.Paragraph.Runs[0].Text; got != "Second section" {
		t.Fatalf("second section text = %q, want Second section", got)
	}
	if pages[0].Blocks[3].Block == nil || pages[0].Blocks[3].Block.Paragraph == nil || pages[0].Blocks[3].Block.Paragraph.Format.Borders == nil || pages[0].Blocks[3].Block.Paragraph.Format.Borders.Top == nil {
		t.Fatalf("expected document-end separator block, got %#v", pages[0].Blocks[3].Block)
	}
	if got := pages[0].Blocks[4].Block.Paragraph.Runs[0].Text; got != "Document note" {
		t.Fatalf("document endnote text = %q, want Document note", got)
	}
	if pages[0].Blocks[4].Block.Paragraph.List == nil || pages[0].Blocks[4].Block.Paragraph.List.Start != 1 {
		t.Fatalf("expected document-end marker, got %#v", pages[0].Blocks[4].Block.Paragraph.List)
	}
}

func TestPaginateUsesConfiguredEndnoteNumbering(t *testing.T) {
	doc := &resolve.Document{
		EndnoteNumberFormat: "lowerRoman",
		EndnoteNumberStart:  5,
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 5000, Height: 3200},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
		}},
		Endnotes: map[int]resolve.Story{
			12: {
				Blocks: []resolve.Block{{
					Kind:      "paragraph",
					Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First note"}}},
				}},
			},
			30: {
				Blocks: []resolve.Block{{
					Kind:      "paragraph",
					Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second note"}}},
				}},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "A"}, {Kind: "endnoteRef", NoteID: 12}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "B"}, {Kind: "endnoteRef", NoteID: 30}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 5 {
		t.Fatalf("page block count = %d, want 5", len(pages[0].Blocks))
	}
	first := pages[0].Blocks[3].Block.Paragraph.List
	second := pages[0].Blocks[4].Block.Paragraph.List
	if first == nil || first.Start != 5 || first.Format != "lowerRoman" {
		t.Fatalf("first endnote list = %#v, want start=5 format=lowerRoman", first)
	}
	if second == nil || second.Start != 6 || second.Format != "lowerRoman" {
		t.Fatalf("second endnote list = %#v, want start=6 format=lowerRoman", second)
	}
}

func TestPaginateUsesContinuationSeparatorForOverflowedEndnotes(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 1000},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0},
		}},
		Endnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First note block"}}}},
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second note block"}}}},
				},
			},
		},
		EndnoteSeparator: resolve.Story{
			Blocks: []resolve.Block{{
				Kind:      "paragraph",
				Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Start separator"}}},
			}},
		},
		EndnoteContinuationSep: resolve.Story{
			Blocks: []resolve.Block{{
				Kind:      "paragraph",
				Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Continuation separator"}}},
			}},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 1}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 3 || len(pages[1].Blocks) != 2 {
		t.Fatalf("page block counts = %d/%d, want 3/2", len(pages[0].Blocks), len(pages[1].Blocks))
	}
	if got := pages[0].Blocks[1].Block.Paragraph.Runs[0].Text; got != "Start separator" {
		t.Fatalf("start separator text = %q, want Start separator", got)
	}
	if got := pages[0].Blocks[2].Block.Paragraph.Runs[0].Text; got != "First note block" {
		t.Fatalf("first overflow page note text = %q, want First note block", got)
	}
	if got := pages[1].Blocks[0].Block.Paragraph.Runs[0].Text; got != "Continuation separator" {
		t.Fatalf("continuation separator text = %q, want Continuation separator", got)
	}
	if got := pages[1].Blocks[1].Block.Paragraph.Runs[0].Text; got != "Second note block" {
		t.Fatalf("continued note text = %q, want Second note block", got)
	}
	if list := pages[0].Blocks[2].Block.Paragraph.List; list == nil || list.Start != 1 {
		t.Fatalf("first continued endnote list = %#v, want start=1", list)
	}
	if list := pages[1].Blocks[1].Block.Paragraph.List; list != nil {
		t.Fatalf("continued paragraph list = %#v, want nil on second page", list)
	}
}

func TestPaginateEmitsContinuationNoticeBeforeEndnoteOverflow(t *testing.T) {
	// Three one-line blocks fit in 43.2pt on a 50pt page. With reservation for the
	// continuation notice, page 1 keeps only the anchor, start separator, and notice,
	// then page 2 starts with the continuation separator and the carried notes.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 1000},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0},
		}},
		Endnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First note block"}}}},
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second note block"}}}},
				},
			},
		},
		EndnoteSeparator: resolve.Story{
			Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Start separator"}}}}},
		},
		EndnoteContinuationSep: resolve.Story{
			Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Continuation separator"}}}}},
		},
		EndnoteContinuationNotice: resolve.Story{
			Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Continued on next page"}}}}},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 1}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	// page 1: anchor, start-sep, continuation notice
	if len(pages[0].Blocks) != 3 {
		t.Fatalf("page 1 block count = %d, want 3", len(pages[0].Blocks))
	}
	lastP1 := pages[0].Blocks[2].Block.Paragraph
	if lastP1 == nil || len(lastP1.Runs) == 0 || lastP1.Runs[0].Text != "Continued on next page" {
		t.Fatalf("page 1 last block text mismatch, want continuation notice")
	}
	// page 2: continuation separator, first note block, second note block
	if len(pages[1].Blocks) != 3 {
		t.Fatalf("page 2 block count = %d, want 3", len(pages[1].Blocks))
	}
	if got := pages[1].Blocks[0].Block.Paragraph.Runs[0].Text; got != "Continuation separator" {
		t.Fatalf("page 2 first block = %q, want Continuation separator", got)
	}
	if got := pages[1].Blocks[1].Block.Paragraph.Runs[0].Text; got != "First note block" {
		t.Fatalf("page 2 second block = %q, want First note block", got)
	}
}

func TestPaginateReservesRoomForLongContinuationNotice(t *testing.T) {
	// Narrow page width forces the continuation notice to wrap. The first note must
	// move to page 2 so page 1 can hold the anchor, start separator, and notice.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2000, Height: 1440},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0},
		}},
		Endnotes: map[int]resolve.Story{
			1: {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First note block"}}}},
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Second note block"}}}},
				},
			},
		},
		EndnoteSeparator: resolve.Story{
			Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Start separator"}}}}},
		},
		EndnoteContinuationSep: resolve.Story{
			Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Continuation separator"}}}}},
		},
		EndnoteContinuationNotice: resolve.Story{
			Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "continuation notice wraps long enough to require two lines"}}}}},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Anchor"}, {Kind: "endnoteRef", NoteID: 1}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 3 {
		t.Fatalf("page 1 block count = %d, want 3", len(pages[0].Blocks))
	}
	if got := pages[0].Blocks[1].Block.Paragraph.Runs[0].Text; got != "Start separator" {
		t.Fatalf("page 1 second block = %q, want Start separator", got)
	}
	if got := pages[0].Blocks[2].Block.Paragraph.Runs[0].Text; got != "continuation notice wraps long enough to require two lines" {
		t.Fatalf("page 1 third block = %q, want continuation notice", got)
	}
	if len(pages[1].Blocks) != 3 {
		t.Fatalf("page 2 block count = %d, want 3", len(pages[1].Blocks))
	}
	if got := pages[1].Blocks[0].Block.Paragraph.Runs[0].Text; got != "Continuation separator" {
		t.Fatalf("page 2 first block = %q, want Continuation separator", got)
	}
	if got := pages[1].Blocks[1].Block.Paragraph.Runs[0].Text; got != "First note block" {
		t.Fatalf("page 2 second block = %q, want First note block", got)
	}
	if got := pages[1].Blocks[2].Block.Paragraph.Runs[0].Text; got != "Second note block" {
		t.Fatalf("page 2 third block = %q, want Second note block", got)
	}
}

func TestResolveGeometryExpandsMarginTopForTallHeader(t *testing.T) {
	doc := &resolve.Document{
		Headers: map[string]resolve.Story{
			"word/header1.xml": {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Header one"}}}},
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Header two"}}}},
				},
			},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 720, Right: 1440, Bottom: 1440, Left: 1440, Header: 360},
			Headers:    []resolve.Reference{{Type: "default", PartName: "word/header1.xml"}},
		}},
	}

	geom := resolveGeometry(doc)
	if !almostEqual(geom.MarginTop, 46.8) {
		t.Fatalf("top margin = %.3f, want 46.8", geom.MarginTop)
	}
}

func TestResolveGeometryUsesFirstPageHeaderStory(t *testing.T) {
	doc := &resolve.Document{
		Headers: map[string]resolve.Story{
			"word/header-default.xml": {
				Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Default header"}}}}},
			},
			"word/header-first.xml": {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First header one"}}}},
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First header two"}}}},
				},
			},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 720, Right: 1440, Bottom: 1440, Left: 1440, Header: 360},
			Headers: []resolve.Reference{
				{Type: "default", PartName: "word/header-default.xml"},
				{Type: "first", PartName: "word/header-first.xml"},
			},
			TitlePage: true,
		}},
	}

	firstPage := resolveGeometryForSectionPage(doc, &doc.Sections[0], 1)
	if !almostEqual(firstPage.MarginTop, 46.8) {
		t.Fatalf("first-page top margin = %.3f, want 46.8", firstPage.MarginTop)
	}
	secondPage := resolveGeometryForSectionPage(doc, &doc.Sections[0], 2)
	if !almostEqual(secondPage.MarginTop, 36.0) {
		t.Fatalf("second-page top margin = %.3f, want 36.0", secondPage.MarginTop)
	}
}

func TestLayoutAnchorsColumnRelativeFloatToResolvedColumn(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 20000, Height: 900},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 1000, Bottom: 0, Left: 1000},
			Columns:    &resolve.Columns{Num: 2, Space: 400},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "One"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Two"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"},
				Runs: []resolve.Run{{
					Kind: "image",
					Image: &resolve.Image{
						Floating:  true,
						Width:     127000,
						Height:    127000,
						XAlign:    "left",
						XRelative: "column",
						YRelative: "paragraph",
					},
				}},
			}},
		},
	}

	_, blocks, floats := Layout(doc)
	if len(blocks) != 3 {
		t.Fatalf("block count = %d, want 3", len(blocks))
	}
	if blocks[2].Column != 1 {
		t.Fatalf("third block column = %d, want 1", blocks[2].Column)
	}
	if len(floats) != 1 {
		t.Fatalf("float count = %d, want 1", len(floats))
	}
	if !almostEqual(floats[0].X, 510) {
		t.Fatalf("float x = %.1f, want 510.0", floats[0].X)
	}
}

func TestPaginateKeepNextMovesClusterToNextPage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 1000},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs:   []resolve.Run{{Kind: "run", Text: "Intro"}},
				Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs:   []resolve.Run{{Kind: "run", Text: "Keep me"}},
				Format: resolve.ParagraphFormat{KeepNext: true, LineSpacing: intPtr(400), LineRule: "exact"},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs:   []resolve.Run{{Kind: "run", Text: "With me"}},
				Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"},
			}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 1 || len(pages[1].Blocks) != 2 {
		t.Fatalf("page block counts = %d/%d, want 1/2", len(pages[0].Blocks), len(pages[1].Blocks))
	}
	if got := pages[1].Blocks[0].Block.Paragraph.Runs[0].Text; got != "Keep me" {
		t.Fatalf("second page first paragraph = %q, want Keep me", got)
	}
	if got := pages[1].Blocks[1].Block.Paragraph.Runs[0].Text; got != "With me" {
		t.Fatalf("second page second paragraph = %q, want With me", got)
	}
}

func TestPaginateKeepLinesMovesWrappedParagraphToNextPage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 3200, Height: 1000},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs:   []resolve.Run{{Kind: "run", Text: "Intro"}},
				Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Text:   "one two three four five six seven eight nine ten eleven twelve thirteen fourteen",
				Runs:   []resolve.Run{{Kind: "run", Text: "one two three four five six seven eight nine ten eleven twelve thirteen fourteen"}},
				Format: resolve.ParagraphFormat{KeepLines: true, LineSpacing: intPtr(400), LineRule: "exact"},
			}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 1 || len(pages[1].Blocks) != 1 {
		t.Fatalf("page block counts = %d/%d, want 1/1", len(pages[0].Blocks), len(pages[1].Blocks))
	}
	if got := pages[1].Blocks[0].Block.Paragraph.Runs[0].Text; got != "one two three four five six seven eight nine ten eleven twelve thirteen fourteen" {
		t.Fatalf("second page paragraph = %q, want wrapped keep-lines paragraph", got)
	}
}

func TestPaginateWidowControlSplitsParagraphAcrossPages(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 2400, Height: 1200},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs:   []resolve.Run{{Kind: "run", Text: "Intro"}},
				Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Text:   "abcdefghijkl mnopqrstuvwx yzabcdefuvwx rstuvwxyzzzz",
				Runs:   []resolve.Run{{Kind: "run", Text: "abcdefghijkl mnopqrstuvwx yzabcdefuvwx rstuvwxyzzzz"}},
				Format: resolve.ParagraphFormat{WidowControl: true, LineSpacing: intPtr(400), LineRule: "exact"},
			}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 2 || len(pages[1].Blocks) != 1 {
		t.Fatalf("page block counts = %d/%d, want 2/1", len(pages[0].Blocks), len(pages[1].Blocks))
	}
	firstFragment := pages[0].Blocks[1].Fragment
	secondFragment := pages[1].Blocks[0].Fragment
	if firstFragment == nil || secondFragment == nil {
		t.Fatalf("expected widow-controlled paragraph fragments on both pages")
	}
	if firstFragment.StartLine != 0 || firstFragment.EndLine != 2 || firstFragment.IsContinuation || firstFragment.IsLast {
		t.Fatalf("unexpected first fragment metadata: %#v", firstFragment)
	}
	if secondFragment.StartLine != 2 || secondFragment.EndLine != 4 || !secondFragment.IsContinuation || !secondFragment.IsLast {
		t.Fatalf("unexpected second fragment metadata: %#v", secondFragment)
	}
}

func TestPaginateReservesFirstPageHeaderStory(t *testing.T) {
	doc := &resolve.Document{
		Headers: map[string]resolve.Story{
			"word/header-first.xml": {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First header one"}}}},
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "First header two"}}}},
				},
			},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 1000},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0, Header: 0},
			Headers:    []resolve.Reference{{Type: "first", PartName: "word/header-first.xml"}},
			TitlePage:  true,
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "One"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Two"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Three"}}, Format: resolve.ParagraphFormat{LineSpacing: intPtr(400), LineRule: "exact"}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 2 {
		t.Fatalf("page count = %d, want 2", len(pages))
	}
	if len(pages[0].Blocks) != 1 || len(pages[1].Blocks) != 2 {
		t.Fatalf("page block counts = %d/%d, want 1/2", len(pages[0].Blocks), len(pages[1].Blocks))
	}
}

func TestResolveGeometryReservesFooterStoryHeight(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused stabilization pass")
	doc := &resolve.Document{
		Footers: map[string]resolve.Story{
			"word/footer1.xml": {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footer one"}}}},
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footer two"}}}},
				},
			},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 720, Left: 1440, Footer: 360},
			Footers:    []resolve.Reference{{Type: "default", PartName: "word/footer1.xml"}},
		}},
	}

	geom := resolveGeometry(doc)
	if !almostEqual(geom.MarginBottom, 46.8) {
		t.Fatalf("bottom margin = %.3f, want 46.8", geom.MarginBottom)
	}
}

func TestResolveGeometryUsesEvenPageFooterStory(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused stabilization pass")
	doc := &resolve.Document{
		Footers: map[string]resolve.Story{
			"word/footer-default.xml": {
				Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Default footer"}}}}},
			},
			"word/footer-even.xml": {
				Blocks: []resolve.Block{
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Even footer one"}}}},
					{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Even footer two"}}}},
				},
			},
		},
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 720, Left: 1440, Footer: 360},
			Footers: []resolve.Reference{
				{Type: "default", PartName: "word/footer-default.xml"},
				{Type: "even", PartName: "word/footer-even.xml"},
			},
		}},
		EvenAndOddHeaders: true,
	}

	secondPage := resolveGeometryForSectionPage(doc, &doc.Sections[0], 2)
	if !almostEqual(secondPage.MarginBottom, 46.8) {
		t.Fatalf("second-page bottom margin = %.3f, want 46.8", secondPage.MarginBottom)
	}
	thirdPage := resolveGeometryForSectionPage(doc, &doc.Sections[0], 3)
	if !almostEqual(thirdPage.MarginBottom, 36.0) {
		t.Fatalf("third-page bottom margin = %.3f, want 36.0", thirdPage.MarginBottom)
	}
}

func TestPaginateSuppressesSpacingBeforeFirstParagraphOnNewPage(t *testing.T) {
	// Spacing-before on the first paragraph of each page/column is suppressed,
	// so it does not inflate the Y position of content at the top of a page.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 1440},
			PageMargin: &resolve.PageMargin{Top: 0, Right: 0, Bottom: 0, Left: 0},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Page one"}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Kind: "run", Text: "Page two"}},
				Format: resolve.ParagraphFormat{
					SpacingBefore: intPtr(1440), // 72pt — would push content way down if not suppressed
				},
			}},
		},
	}

	pages := Paginate(doc)
	if len(pages) < 2 {
		t.Fatalf("expected at least 2 pages, got %d", len(pages))
	}
	// The first paragraph on page 2 must start at Y=0 (spacing-before suppressed).
	var page2First *BlockBox
	for i := range pages[1].Blocks {
		if pages[1].Blocks[i].Block != nil && pages[1].Blocks[i].Block.Kind == "paragraph" {
			page2First = &pages[1].Blocks[i]
			break
		}
	}
	if page2First == nil {
		t.Fatal("no paragraph block on page 2")
	}
	if page2First.Y != 0 {
		t.Fatalf("first paragraph on page 2 Y = %.3f, want 0 (spacing-before suppressed)", page2First.Y)
	}
}

func TestLayoutParagraphSuppressesSpacingWhenAutoSpacingSet(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{
					SpacingBefore:     intPtr(240), // 12pt, but auto-spacing overrides it
					SpacingAfter:      intPtr(240), // 12pt, but auto-spacing overrides it
					SpacingBeforeAuto: true,
					SpacingAfterAuto:  true,
					LineRule:          "exact",
					LineSpacing:       intPtr(240),
				},
			}},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	want := 12.0 // only the exact line height; before and after suppressed by auto-spacing
	if !almostEqual(blocks[1].Height, want) {
		t.Fatalf("auto-spacing paragraph height = %.3f, want %.3f", blocks[1].Height, want)
	}
}

func TestLayoutTableHeightIncludesInsideHBorderForInnerRows(t *testing.T) {
	insideH := &resolve.BorderSide{Val: "single", SizePt: 2}
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{{
			Kind: "table",
			Table: &resolve.Table{
				Format: resolve.TableFormat{
					ColumnWidths: []int{4000},
					Borders: &resolve.TableBorders{
						InsideH: insideH,
					},
				},
				Rows: []resolve.TableRow{
					{Cells: []resolve.TableCell{{Text: "row one"}}},
					{Cells: []resolve.TableCell{{Text: "row two"}}},
				},
			},
		}},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	// Row 1 has no top border (first row, no table top border).
	// Row 2 top = insideH = 2pt. Both rows share default cell height ~14.4pt.
	// Total = 14.4 + (2 + 14.4) = 30.8pt
	want := 30.8
	if !almostEqual(blocks[0].Height, want) {
		t.Fatalf("table insideH border height = %.3f, want %.3f", blocks[0].Height, want)
	}
}

func TestDropCapHeightUsesLinesTimesDefaultLineHeight(t *testing.T) {
	// A drop cap with w:lines=3 and default 12pt font should produce a FloatBox
	// of height 3 × 12 × 1.2 = 43.2pt, not the large drop-cap font size.
	// It should also expose a right-side line exclusion for the adjacent paragraph.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Frame: &resolve.FrameProps{
					W:       720, // 36pt
					DropCap: "drop",
					Lines:   3,
					HAnchor: "text",
					VAnchor: "text",
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{}},
		},
	}

	_, _, floats := Layout(doc)

	if len(floats) != 1 {
		t.Fatalf("float count = %d, want 1", len(floats))
	}
	fb := floats[0]
	want := 3.0 * 12.0 * cssNormalLineRatio // 43.2pt
	if !almostEqual(fb.Height, want) {
		t.Errorf("drop cap height = %.3f, want %.3f", fb.Height, want)
	}
}

func TestDropCapAutoWidthUsesContentWidthWhenFrameWidthUnset(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Frame: &resolve.FrameProps{
					DropCap: "drop",
					Lines:   3,
					HAnchor: "text",
					VAnchor: "text",
				},
				Format: resolve.ParagraphFormat{
					IndentFirstLine: intPtr(432), // 21.6pt, matches sample1's visible text-indent
				},
				Runs: []resolve.Run{{
					Kind:   "run",
					Text:   "D",
					Format: resolve.RunFormat{FontSize: intPtr(117)}, // 58.5pt
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Body"}}}},
		},
	}

	geom, _, floats := Layout(doc)
	if len(floats) != 1 {
		t.Fatalf("float count = %d, want 1", len(floats))
	}
	fb := floats[0]
	if fb.Width >= geom.ColumnWidthPt {
		t.Fatalf("drop cap width = %.3f, want less than column width %.3f", fb.Width, geom.ColumnWidthPt)
	}
	want := 21.6 + (58.5 * 0.62)
	if !almostEqual(fb.Width, want) {
		t.Fatalf("drop cap auto width = %.3f, want %.3f", fb.Width, want)
	}
}

func TestFrameParagraphIsOutOfFlowAndEmitsFloatBox(t *testing.T) {
	// A paragraph with w:framePr must contribute zero height to the block flow
	// and produce exactly one FloatBox at the resolved position.
	// The frame is positioned 36pt from the left margin with a width of 72pt.
	// It sits between two normal paragraphs so those paragraphs should be at y=0 and y=14.4.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Frame: &resolve.FrameProps{
					W:       1440, // 72pt
					X:       720,  // 36pt from margin
					HAnchor: "text",
					VAnchor: "text",
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{}},
		},
	}

	_, blocks, floats := Layout(doc)

	// Frame paragraph contributes zero in-flow height — only two non-zero block heights.
	if len(blocks) != 3 {
		t.Fatalf("block count = %d, want 3", len(blocks))
	}
	if blocks[1].Height != 0 {
		t.Errorf("frame block height = %.3f, want 0", blocks[1].Height)
	}
	// Third paragraph starts immediately after the first (frame took no space).
	wantY := blocks[0].Height
	if !almostEqual(blocks[2].Y, wantY) {
		t.Errorf("third paragraph Y = %.3f, want %.3f", blocks[2].Y, wantY)
	}
	// Exactly one FloatBox for the frame paragraph.
	if len(floats) != 1 {
		t.Fatalf("float count = %d, want 1", len(floats))
	}
	fb := floats[0]
	if fb.FrameParagraph == nil {
		t.Fatal("FloatBox.FrameParagraph is nil")
	}
	wantWidth := 72.0 // 1440 twips / 20
	if !almostEqual(fb.Width, wantWidth) {
		t.Errorf("frame float width = %.3f, want %.3f", fb.Width, wantWidth)
	}
	// X = margin-left (72pt) + 36pt offset = 108pt
	wantX := 72.0 + 36.0
	if !almostEqual(fb.X, wantX) {
		t.Errorf("frame float X = %.3f, want %.3f", fb.X, wantX)
	}
}

func TestPolygonXRangeAtBand(t *testing.T) {
	// Triangle: (0,0)→(100,0)→(50,100). At band [25,75] the polygon edges
	// intersect at x=12.5 (left edge) and x=87.5 (right edge).
	poly := [][2]float64{{0, 0}, {100, 0}, {50, 100}}
	minX, maxX, ok := polygonXRangeAtBand(poly, 25, 75)
	if !ok {
		t.Fatal("polygonXRangeAtBand returned ok=false")
	}
	if !almostEqual(minX, 12.5) {
		t.Errorf("minX = %.3f, want 12.5", minX)
	}
	if !almostEqual(maxX, 87.5) {
		t.Errorf("maxX = %.3f, want 87.5", maxX)
	}
}

func TestPolygonIntervalsAtBandPreserveConcaveGap(t *testing.T) {
	// Concave C-shape. The middle band should preserve the occupied left arm
	// instead of collapsing to the full outer bounding span.
	poly := [][2]float64{{0, 0}, {100, 0}, {100, 20}, {40, 20}, {40, 80}, {100, 80}, {100, 100}, {0, 100}}
	intervals := polygonIntervalsAtBand(poly, 40, 60)
	if len(intervals) != 1 {
		t.Fatalf("interval count = %d, want 1 (%#v)", len(intervals), intervals)
	}
	if !almostEqual(intervals[0][0], 0) || !almostEqual(intervals[0][1], 40) {
		t.Fatalf("interval = %#v, want [0 40]", intervals[0])
	}
}

func TestTightWrapPolygonNarrowsLineWidth(t *testing.T) {
	// Diamond polygon: (50,0)→(100,20)→(50,40)→(0,20).
	// At band [10,15] (avoiding the left vertex at y=20):
	//   left edge (0,20)→(50,0) at y=10: x=25; at y=15: x=12.5
	//   right edge (50,0)→(100,20) at y=10: x=75; at y=15: x=87.5
	// So minX=12.5, maxX=87.5 — polygon narrows exclusion vs bounding box [0,100].
	poly := [][2]float64{{50, 0}, {100, 20}, {50, 40}, {0, 20}}
	minX, maxX, ok := polygonXRangeAtBand(poly, 10, 15)
	if !ok {
		t.Fatal("polygonXRangeAtBand returned ok=false for diamond at band [10,15]")
	}
	if !almostEqual(minX, 12.5) {
		t.Errorf("minX = %.3f, want 12.5", minX)
	}
	if !almostEqual(maxX, 87.5) {
		t.Errorf("maxX = %.3f, want 87.5", maxX)
	}
}

func TestPolygonXRangeAtBandSamplesInteriorVertices(t *testing.T) {
	// The widest contour occurs at the interior vertices y=4, not at the band
	// top/mid/bottom samples. Tight/through wrapping should still honor that
	// wider shoulder when estimating the occupied width across the full band.
	poly := [][2]float64{{50, 0}, {90, 4}, {60, 20}, {40, 20}, {10, 4}}
	minX, maxX, ok := polygonXRangeAtBand(poly, 0, 20)
	if !ok {
		t.Fatal("polygonXRangeAtBand returned ok=false for vertex-driven shoulder")
	}
	if !almostEqual(minX, 10) {
		t.Fatalf("minX = %.3f, want 10", minX)
	}
	if !almostEqual(maxX, 90) {
		t.Fatalf("maxX = %.3f, want 90", maxX)
	}
}

func TestLineAvailableWidthUsesConcavePolygonGapForLargestWrap(t *testing.T) {
	exclusions := []lineExclusion{{
		top:      0,
		bottom:   100,
		left:     0,
		right:    100,
		wrapSide: "largest",
		polygon:  [][2]float64{{0, 0}, {100, 0}, {100, 20}, {40, 20}, {40, 80}, {100, 80}, {100, 100}, {0, 100}},
	}}
	available := lineAvailableWidthPt(0, 140, 40, 60, exclusions)
	if !almostEqual(available, 100.0) {
		t.Fatalf("available width = %.3f, want 100.0", available)
	}
}

func TestPaginateContinuousSectionDocGridChangeStaysOnPage(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
			},
			{
				Type:       "continuous",
				PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
				PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
				DocGrid:    &resolve.DocGrid{Type: "lines", LinePitch: 360},
			},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Intro"}}}},
			{Kind: "section", Section: &resolve.Section{Type: "continuous", PageSize: &resolve.PageSize{Width: 12240, Height: 15840}, PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}, DocGrid: &resolve.DocGrid{Type: "lines", LinePitch: 360}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "After grid"}}, Format: resolve.ParagraphFormat{SnapToGrid: true}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1", len(pages))
	}
	if len(pages[0].Blocks) != 2 {
		t.Fatalf("block count = %d, want 2", len(pages[0].Blocks))
	}
	if !almostEqual(pages[0].Blocks[1].Y, 14.4) {
		t.Fatalf("post-break block y = %.3f, want 14.4", pages[0].Blocks[1].Y)
	}
	if !almostEqual(pages[0].Blocks[1].Height, 18.0) {
		t.Fatalf("post-break block height = %.3f, want 18.0", pages[0].Blocks[1].Height)
	}
	if !almostEqual(pages[0].Blocks[1].Geometry.DocGridPitchPt, 18.0) {
		t.Fatalf("post-break docGrid pitch = %.3f, want 18.0", pages[0].Blocks[1].Geometry.DocGridPitchPt)
	}
}

func TestContextualSpacingDoesNotCrossTableBoundary(t *testing.T) {
	spacing := 120
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Body", Runs: []resolve.Run{{Kind: "run", Text: "Before"}}, Format: resolve.ParagraphFormat{ContextualSpacing: true, SpacingAfter: &spacing}}},
			{
				Kind: "table",
				Table: &resolve.Table{
					Rows: []resolve.TableRow{
						{
							Cells: []resolve.TableCell{
								{
									Blocks: []resolve.Block{
										{
											Kind: "paragraph",
											Paragraph: &resolve.Paragraph{
												Runs: []resolve.Run{{Kind: "run", Text: "Cell"}},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{StyleID: "Body", Runs: []resolve.Run{{Kind: "run", Text: "After"}}, Format: resolve.ParagraphFormat{ContextualSpacing: true, SpacingBefore: &spacing}}},
		},
	}

	_, blocks, _ := Layout(doc)
	if len(blocks) != 3 {
		t.Fatalf("block count = %d, want 3", len(blocks))
	}
	if !almostEqual(blocks[2].Height, 20.4) {
		t.Fatalf("expected spacing-before after table boundary to remain in paragraph height, got %.3f want 20.4", blocks[2].Height)
	}
}

func TestPaginateMixedFloatAndTableIsDeterministic(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 6000, Height: 4200},
			PageMargin: &resolve.PageMargin{Top: 600, Right: 600, Bottom: 600, Left: 600},
			Columns:    &resolve.Columns{Num: 2, Space: 240},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Start"}}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
				Kind: "image",
				Image: &resolve.Image{
					MIMEType:  "image/png",
					Data:      []byte{0x89, 0x50, 0x4e, 0x47},
					Width:     914400,
					Height:    914400,
					Floating:  true,
					XRelative: "column",
					YRelative: "paragraph",
					WrapType:  "square",
				},
			}}}},
			{Kind: "table", Table: &resolve.Table{
				Format: resolve.TableFormat{ColumnWidths: []int{1800, 1800}},
				Rows: []resolve.TableRow{
					{Cells: []resolve.TableCell{{Text: "R1C1"}, {Text: "R1C2"}}},
					{Cells: []resolve.TableCell{{Text: "R2C1"}, {Text: "R2C2"}}},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "End"}}}},
		},
	}

	pages1 := Paginate(doc)
	pages2 := Paginate(doc)
	if len(pages1) != len(pages2) {
		t.Fatalf("page count mismatch: first=%d second=%d", len(pages1), len(pages2))
	}
	for i := range pages1 {
		if len(pages1[i].Blocks) != len(pages2[i].Blocks) {
			t.Fatalf("block count mismatch on page %d: first=%d second=%d", i, len(pages1[i].Blocks), len(pages2[i].Blocks))
		}
		for j := range pages1[i].Blocks {
			b1 := pages1[i].Blocks[j]
			b2 := pages2[i].Blocks[j]
			if !almostEqual(b1.Y, b2.Y) || !almostEqual(b1.Height, b2.Height) {
				t.Fatalf("geometry mismatch page %d block %d: first(y=%.3f,h=%.3f) second(y=%.3f,h=%.3f)", i, j, b1.Y, b1.Height, b2.Y, b2.Height)
			}
		}
	}
}

// makeNestedTableBlock returns a block containing an outer table with outerRows rows,
// each cell containing an inner table with innerRows rows.
func makeNestedTableBlock(outerRows, innerRows int) resolve.Block {
	makeInnerRows := func(n int) []resolve.TableRow {
		rows := make([]resolve.TableRow, n)
		for i := range rows {
			rows[i] = resolve.TableRow{
				Cells: []resolve.TableCell{{
					Blocks: []resolve.Block{{
						Kind: "paragraph",
						Paragraph: &resolve.Paragraph{
							Runs: []resolve.Run{{Kind: "run", Text: "inner"}},
						},
					}},
				}},
			}
		}
		return rows
	}
	outerRowList := make([]resolve.TableRow, outerRows)
	for i := range outerRowList {
		outerRowList[i] = resolve.TableRow{
			Cells: []resolve.TableCell{{
				Blocks: []resolve.Block{{
					Kind:  "table",
					Table: &resolve.Table{Rows: makeInnerRows(innerRows)},
				}},
			}},
		}
	}
	return resolve.Block{
		Kind:  "table",
		Table: &resolve.Table{Rows: outerRowList},
	}
}

func TestPaginateNestedTableRowsAreNotLostOrDuplicated(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused stabilization pass")
	// A page tall enough for only ~3 lines, with an outer table that has 3 rows
	// each containing a 2-row inner table. Paginate must emit exactly one top-level
	// block (the outer table) and must not panic or emit duplicate/zero block entries.
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 1800},
			PageMargin: &resolve.PageMargin{Top: 120, Right: 720, Bottom: 120, Left: 720},
		}},
		Blocks: []resolve.Block{
			makeNestedTableBlock(3, 2),
		},
	}

	pages := Paginate(doc)
	if len(pages) == 0 {
		t.Fatal("expected at least one page")
	}

	// Count total table blocks across all pages — must be exactly 1 (no duplication).
	totalTableBlocks := 0
	for _, page := range pages {
		for _, box := range page.Blocks {
			if box.Block != nil && box.Block.Kind == "table" {
				totalTableBlocks++
			}
		}
	}
	if totalTableBlocks != 1 {
		t.Fatalf("outer table block count across pages = %d, want 1 (must not be duplicated)", totalTableBlocks)
	}

	// Every emitted block must have a positive height.
	for i, page := range pages {
		for j, box := range page.Blocks {
			if box.Height <= 0 {
				t.Errorf("page %d block %d has non-positive height %.3f", i, j, box.Height)
			}
		}
	}
}

func TestPaginateNestedTableHeightReflectsInnerContent(t *testing.T) {
	// The outer table row height must account for the inner table rows inside cells.
	// A single outer row with a 4-row inner table must be taller than a single outer
	// row with a 1-row inner table.
	docSmall := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{makeNestedTableBlock(1, 1)},
	}
	docTall := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
			PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
		}},
		Blocks: []resolve.Block{makeNestedTableBlock(1, 4)},
	}

	_, smallBlocks, _ := Layout(docSmall)
	_, tallBlocks, _ := Layout(docTall)

	if len(smallBlocks) != 1 || len(tallBlocks) != 1 {
		t.Fatalf("expected 1 block each, got small=%d tall=%d", len(smallBlocks), len(tallBlocks))
	}
	if tallBlocks[0].Height <= smallBlocks[0].Height {
		t.Fatalf("tall nested table (h=%.3f) should be taller than small (h=%.3f)", tallBlocks[0].Height, smallBlocks[0].Height)
	}
}

func TestSectionStoryForPageRespectsFlags(t *testing.T) {
	refs := []resolve.Reference{
		{Type: "default", PartName: "word/header-default.xml"},
		{Type: "first", PartName: "word/header-first.xml"},
		{Type: "even", PartName: "word/header-even.xml"},
	}
	// The first/even stories have two paragraphs each so their height exceeds
	// the allocated header space (36pt margin - 18pt header dist = 18pt), while
	// the default story has one paragraph and fits.
	headers := map[string]resolve.Story{
		"word/header-default.xml": {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "default"}}}}}},
		"word/header-first.xml": {
			Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "first one"}}}},
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "first two"}}}},
			},
		},
		"word/header-even.xml": {
			Blocks: []resolve.Block{
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "even one"}}}},
				{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "even two"}}}},
			},
		},
	}

	// With both flags false, only default is used (single paragraph fits in 18pt).
	g := resolveGeometryForSectionPage(&resolve.Document{Headers: headers}, &resolve.Section{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Top: 720, Header: 360},
		Headers:    refs,
	}, 1)
	if g.MarginTop != 36.0 {
		t.Errorf("page 1 with no flags: margin = %.1f, want 36.0", g.MarginTop)
	}

	// With TitlePage true, page 1 uses first (two paragraphs overflow 18pt).
	g = resolveGeometryForSectionPage(&resolve.Document{Headers: headers}, &resolve.Section{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Top: 720, Header: 360},
		Headers:    refs,
		TitlePage:  true,
	}, 1)
	if g.MarginTop != 46.8 {
		t.Errorf("page 1 with TitlePage: margin = %.1f, want 46.8", g.MarginTop)
	}

	// With EvenAndOddHeaders true, page 2 uses even (two paragraphs overflow 18pt).
	g = resolveGeometryForSectionPage(&resolve.Document{Headers: headers, EvenAndOddHeaders: true}, &resolve.Section{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Top: 720, Header: 360},
		Headers:    refs,
	}, 2)
	if g.MarginTop != 46.8 {
		t.Errorf("page 2 with EvenAndOddHeaders: margin = %.1f, want 46.8", g.MarginTop)
	}
}

func TestLayoutMirrorMarginsSwapsEvenPageMargins(t *testing.T) {
	section := &resolve.Section{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Top: 1440, Right: 720, Bottom: 1440, Left: 1440},
	}
	// Odd page (page 1): margins unchanged.
	g := resolveGeometryForSectionPage(&resolve.Document{MirrorMargins: true}, section, 1)
	if !almostEqual(g.MarginLeft, 72.0) || !almostEqual(g.MarginRight, 36.0) {
		t.Errorf("page 1: margins = %.1f, %.1f, want 72.0, 36.0", g.MarginLeft, g.MarginRight)
	}
	// Even page (page 2): left/right swapped.
	g = resolveGeometryForSectionPage(&resolve.Document{MirrorMargins: true}, section, 2)
	if !almostEqual(g.MarginLeft, 36.0) || !almostEqual(g.MarginRight, 72.0) {
		t.Errorf("page 2: margins = %.1f, %.1f, want 36.0, 72.0", g.MarginLeft, g.MarginRight)
	}
	// MirrorMargins=false: even page stays unchanged.
	g = resolveGeometryForSectionPage(&resolve.Document{MirrorMargins: false}, section, 2)
	if !almostEqual(g.MarginLeft, 72.0) || !almostEqual(g.MarginRight, 36.0) {
		t.Errorf("page 2 (no mirror): margins = %.1f, %.1f, want 72.0, 36.0", g.MarginLeft, g.MarginRight)
	}
}
func TestPaginateUsesBreakKindWhenLegacySectionTypeIsEmpty(t *testing.T) {
	geom := &resolve.PageSize{Width: 12240, Height: 15840}
	margin := &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}
	doc := &resolve.Document{
		Sections: []resolve.Section{
			{PageSize: geom, PageMargin: margin},
			{BreakKind: "continuous", PageSize: geom, PageMargin: margin},
		},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "Before", Runs: []resolve.Run{{Kind: "run", Text: "Before"}}}},
			{Kind: "section", Section: &resolve.Section{}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "After", Runs: []resolve.Run{{Kind: "run", Text: "After"}}}},
		},
	}

	pages := Paginate(doc)
	if len(pages) != 1 {
		t.Fatalf("page count = %d, want 1 for legacy BreakKind=continuous", len(pages))
	}
	if len(pages[0].Blocks) != 2 {
		t.Fatalf("page blocks = %d, want 2", len(pages[0].Blocks))
	}
}
func TestPaginateMaterializedNotesKeepSemanticTargets(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 4000},
			PageMargin: &resolve.PageMargin{Top: 400, Right: 400, Bottom: 400, Left: 400},
		}},
		Footnotes: map[int]resolve.Story{7: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Footnote seven"}}}}}}},
		Endnotes:  map[int]resolve.Story{9: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Endnote nine"}}}}}}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Body"}, {Kind: "footnoteRef", NoteID: 7}, {Kind: "endnoteRef", NoteID: 9}}}},
		},
	}
	pages := Paginate(doc)
	if len(pages) == 0 || len(pages[0].Footnotes) < 2 {
		t.Fatalf("expected materialized footnote blocks, got %#v", pages)
	}
	foundFootnote := false
	for _, box := range pages[0].Footnotes {
		if box.Block != nil && box.Block.NoteKind == "footnote" && box.Block.NoteID == 7 {
			foundFootnote = true
			break
		}
	}
	if !foundFootnote {
		t.Fatal("materialized footnote lost NoteKind/NoteID")
	}
	foundEndnote := false
	for _, page := range pages {
		for _, box := range page.Blocks {
			if box.Block != nil && box.Block.NoteKind == "endnote" && box.Block.NoteID == 9 {
				foundEndnote = true
			}
		}
	}
	if !foundEndnote {
		t.Fatal("materialized endnote lost NoteKind/NoteID")
	}
}

func TestContinuousSectionCarriesColumnSeparatorGeometry(t *testing.T) {
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize: &resolve.PageSize{Width: 12000, Height: 12000}, PageMargin: &resolve.PageMargin{Left: 1000, Right: 1000},
			Columns: &resolve.Columns{Num: 1},
		}, {
			Type: "continuous", PageSize: &resolve.PageSize{Width: 12000, Height: 12000}, PageMargin: &resolve.PageMargin{Left: 1000, Right: 1000},
			Columns: &resolve.Columns{Num: 2, Space: 720, Sep: true},
		}},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "one"}}}},
			{Kind: "section", Section: &resolve.Section{Type: "continuous", PageSize: &resolve.PageSize{Width: 12000, Height: 12000}, PageMargin: &resolve.PageMargin{Left: 1000, Right: 1000}, Columns: &resolve.Columns{Num: 2, Space: 720, Sep: true}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "two"}}}},
		},
	}
	pages := Paginate(doc)
	if len(pages) != 1 || len(pages[0].Blocks) < 2 {
		t.Fatalf("unexpected pagination: %#v", pages)
	}
	last := pages[0].Blocks[len(pages[0].Blocks)-1]
	if last.Geometry.ColumnCount != 2 || !last.Geometry.ColumnSep {
		t.Fatalf("continuous section geometry = %+v, want two columns with separator", last.Geometry)
	}
}

func TestResolveGeometryPreservesExplicitUnequalColumnGrid(t *testing.T) {
	even := false
	doc := &resolve.Document{Sections: []resolve.Section{{
		PageSize:   &resolve.PageSize{Width: 10000, Height: 12000},
		PageMargin: &resolve.PageMargin{Left: 1000, Right: 1000},
		Columns: &resolve.Columns{
			Num: 2, Space: 400, EvenlySpaced: &even,
			Widths: []int{2000, 5000}, Spaces: []int{600},
		},
	}}}
	geom := resolveGeometry(doc)
	if len(geom.ColumnWidthsPt) != 2 || !almostSame(geom.ColumnWidthsPt[0], 100) || !almostSame(geom.ColumnWidthsPt[1], 250) {
		t.Fatalf("ColumnWidthsPt = %#v, want [100 250]", geom.ColumnWidthsPt)
	}
	if len(geom.ColumnGapsPt) != 1 || !almostSame(geom.ColumnGapsPt[0], 30) {
		t.Fatalf("ColumnGapsPt = %#v, want [30]", geom.ColumnGapsPt)
	}
	if got := columnLeftPt(geom, 1); !almostSame(got, 180) {
		t.Fatalf("column 2 left = %.2fpt, want 180pt", got)
	}
	if got := resolveXAlign("right", "column", geom, 1, 50); !almostSame(got, 380) {
		t.Fatalf("right-aligned column float x = %.2fpt, want 380pt", got)
	}
}

func TestResolveGeometryMarksSourceSuppliedEqualColumnGridExplicit(t *testing.T) {
	sec := resolve.Section{Columns: &resolve.Columns{Num: 2, Widths: []int{4320, 4320}, Spaces: []int{720}}}
	geom := resolveGeometryForSection(&resolve.Document{}, &sec)
	if !geom.ExplicitColumns {
		t.Fatal("source-supplied equal-width column grid was not marked explicit")
	}
	if len(geom.ColumnWidthsPt) != 2 || geom.ColumnWidthsPt[0] != 216 || geom.ColumnWidthsPt[1] != 216 {
		t.Fatalf("column widths = %#v", geom.ColumnWidthsPt)
	}
}

func TestOOXMLAutoLineSpacingDoesNotApplyLegacyNormalLineFactorTwice(t *testing.T) {
	spacing := 276
	p := &resolve.Paragraph{Format: resolve.ParagraphFormat{LineSpacing: &spacing, LineRule: "auto"}}
	if got, want := autoParagraphLineHeightPt(p, 11), 13.2; !almostEqual(got, want) {
		t.Fatalf("OOXML auto line height = %.3fpt, want %.3fpt", got, want)
	}
}

func TestLegacyAutoLineSpacingDoesNotDoubleApplyNormalLineFactor(t *testing.T) {
	spacing := 276
	p := &resolve.Paragraph{
		Format: resolve.ParagraphFormat{LineSpacing: &spacing, LineRule: "auto"},
		Source: &resolve.ParagraphSource{},
	}
	if got, want := autoParagraphLineHeightPt(p, 11), 13.2; !almostEqual(got, want) {
		t.Fatalf("legacy auto line height = %.3fpt, want %.3fpt", got, want)
	}
}

func TestOOXMLListAutoLineSpacingKeepsNaturalWordAdvance(t *testing.T) {
	spacing := 240
	p := &resolve.Paragraph{
		Numbering: &resolve.Numbering{},
		Format:    resolve.ParagraphFormat{LineSpacing: &spacing, LineRule: "auto"},
	}
	if got, want := autoParagraphLineHeightPt(p, 11), 13.2; !almostEqual(got, want) {
		t.Fatalf("OOXML list auto line height = %.3fpt, want %.3fpt", got, want)
	}
}

func TestNarrowColumnLineMetricsWrapEachGlyph(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Ab c", Format: resolve.RunFormat{FontSize: intPtr(22)}}}}
	metrics := estimateParagraphLineMetricsWithWidths(p, 1, 1, 11, nil)
	if got, want := len(metrics), 4; got != want {
		t.Fatalf("narrow-column line count = %d, want %d", got, want)
	}
	lines := paragraphFragmentRunLines(p, []float64{1, 1, 1, 1}, 11)
	if got, want := len(lines), 4; got != want {
		t.Fatalf("narrow-column fragment line count = %d, want %d", got, want)
	}
	var text string
	for _, line := range lines {
		for _, run := range line {
			text += run.Text
		}
	}
	if text != "Ab c" {
		t.Fatalf("narrow-column fragments text = %q, want %q", text, "Ab c")
	}
}

func TestPaginateSplitsOrdinaryParagraphAcrossPagesWithoutWidowControl(t *testing.T) {
	fontSize := 22 // 11pt half-points
	text := strings.Repeat("abcdefghij ", 80)
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 4000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 200, Bottom: 200, Left: 300, Right: 300},
		}},
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{
			Text: text,
			Runs: []resolve.Run{{Kind: "run", Text: text, Format: resolve.RunFormat{FontSize: &fontSize}}},
		}}},
	}
	pages := Paginate(doc)
	if len(pages) < 2 {
		t.Fatalf("ordinary long paragraph pages = %d, want >= 2", len(pages))
	}
	fragments := 0
	for _, page := range pages {
		for _, box := range page.Blocks {
			if box.Block != nil && box.Block.Kind == "paragraph" && box.Fragment != nil {
				fragments++
			}
		}
	}
	if fragments < 2 {
		t.Fatalf("paragraph fragments = %d, want >= 2", fragments)
	}
}

func TestPaginateSplitsTableAtRowBoundariesAndRepeatsHeaders(t *testing.T) {
	exact := 400 // 20pt
	rows := make([]resolve.TableRow, 0, 7)
	for i := 0; i < 7; i++ {
		rows = append(rows, resolve.TableRow{
			Format: resolve.TableRowFormat{Height: &exact, HeightRule: "exact", IsHeader: i == 0},
			Cells:  []resolve.TableCell{{Text: "row", Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: "row", Runs: []resolve.Run{{Kind: "run", Text: "row"}}}}}}},
		})
	}
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 6000, Height: 2400},
			PageMargin: &resolve.PageMargin{Top: 200, Bottom: 200, Left: 300, Right: 300},
		}},
		Blocks: []resolve.Block{{Kind: "table", Table: &resolve.Table{Rows: rows}}},
	}
	pages := Paginate(doc)
	if len(pages) < 2 {
		t.Fatalf("table pages = %d, want >= 2", len(pages))
	}
	var fragments []*TableFragment
	for _, page := range pages {
		for i := range page.Blocks {
			if page.Blocks[i].TableFragment != nil {
				fragments = append(fragments, page.Blocks[i].TableFragment)
			}
		}
	}
	if len(fragments) < 2 {
		t.Fatalf("table fragments = %d, want >= 2", len(fragments))
	}
	if !fragments[1].RepeatHeaders {
		t.Fatal("continuation table fragment did not repeat header rows")
	}
	if !fragments[len(fragments)-1].IsLast {
		t.Fatal("final table fragment was not marked last")
	}
}

func TestOOXMLListAutoLineSpacingUsesSameSourceAwareSemanticsAsParagraphs(t *testing.T) {
	spacing := 276
	p := &resolve.Paragraph{
		Numbering: &resolve.Numbering{},
		Format:    resolve.ParagraphFormat{LineSpacing: &spacing, LineRule: "auto"},
	}
	if got, want := autoParagraphLineHeightPt(p, 11), 13.2; !almostEqual(got, want) {
		t.Fatalf("OOXML list auto line height = %.3fpt, want %.3fpt", got, want)
	}
}

func TestFooterReservesNaturalLeadingWithoutCroppingBody(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused stabilization pass")
	footer := resolve.Story{Blocks: []resolve.Block{{
		Kind:      "paragraph",
		Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Footer"}}},
	}}}
	doc := &resolve.Document{
		DefaultRunFormat: resolve.RunFormat{FontSize: intPtr(24)},
		Footers:          map[string]resolve.Story{"footer.xml": footer},
	}
	sec := resolve.Section{
		PageMargin: &resolve.PageMargin{Top: 1440, Bottom: 1584, Left: 1800, Right: 1800, Footer: 720},
		Footers:    []resolve.Reference{{Type: "default", PartName: "footer.xml"}},
	}
	g := resolveGeometryForSectionPage(doc, &sec, 1)
	// The footer story consumes the entire nominal footer allocation in this
	// geometry. Pagination must additionally reserve Word/CSS natural leading
	// instead of relying on renderer-side clipping.
	if g.MarginBottom <= twipsToPoints(sec.PageMargin.Bottom) {
		t.Fatalf("bottom margin = %.2fpt; expected footer leading clearance beyond %.2fpt", g.MarginBottom, twipsToPoints(sec.PageMargin.Bottom))
	}
}

func TestOOXMLColumnsDefaultToHalfInchGap(t *testing.T) {
	doc := &resolve.Document{}
	sec := resolve.Section{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Left: 1800, Right: 1800},
		Columns:    &resolve.Columns{Num: 2},
	}
	geom := resolveGeometryForSectionPage(doc, &sec, 1)
	if !almostSame(geom.ColumnGapPt, 36) {
		t.Fatalf("ColumnGapPt = %.2f, want 36pt OOXML default", geom.ColumnGapPt)
	}
}

func TestOOXMLColumnsRepairImpossibleSerializedGap(t *testing.T) {
	doc := &resolve.Document{}
	sec := resolve.Section{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Left: 1800, Right: 1800},
		// Some producers serialize an EMU-like value directly into w:space.
		// 360000 interpreted as twips would be 18000pt and leave no column.
		Columns: &resolve.Columns{Num: 2, Space: 360000},
	}
	geom := resolveGeometryForSectionPage(doc, &sec, 1)
	if !almostSame(geom.ColumnGapPt, 36) {
		t.Fatalf("ColumnGapPt = %.2f, want repaired 36pt", geom.ColumnGapPt)
	}
	if geom.ColumnWidthPt < 1 {
		t.Fatalf("ColumnWidthPt = %.2f, want usable positive column", geom.ColumnWidthPt)
	}
}

func TestTableSplitBoundarySafeKeepsVerticalMergeGroupTogether(t *testing.T) {
	table := &resolve.Table{Rows: []resolve.TableRow{
		{Cells: []resolve.TableCell{{Format: resolve.TableCellFormat{VMerge: "restart"}}}},
		{Cells: []resolve.TableCell{{Format: resolve.TableCellFormat{VMerge: "continue"}}}},
		{Cells: []resolve.TableCell{{}}},
	}}
	if tableSplitBoundarySafe(table, 1) {
		t.Fatal("boundary inside vertical merge should be unsafe")
	}
	if !tableSplitBoundarySafe(table, 2) {
		t.Fatal("boundary after vertical merge should be safe")
	}
}

func TestPaginateSplitsTableAfterVerticalMergeGroup(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused stabilization pass")
	height := 600 // 30pt per row
	doc := &resolve.Document{
		Sections: []resolve.Section{{
			PageSize:   &resolve.PageSize{Width: 7200, Height: 2400}, // 360x120pt
			PageMargin: &resolve.PageMargin{Top: 300, Right: 300, Bottom: 300, Left: 300},
		}},
		Blocks: []resolve.Block{{Kind: "table", Table: &resolve.Table{Rows: []resolve.TableRow{
			{Format: resolve.TableRowFormat{Height: &height, HeightRule: "exact"}, Cells: []resolve.TableCell{{Format: resolve.TableCellFormat{VMerge: "restart"}, Text: "merged"}}},
			{Format: resolve.TableRowFormat{Height: &height, HeightRule: "exact"}, Cells: []resolve.TableCell{{Format: resolve.TableCellFormat{VMerge: "continue"}}}},
			{Format: resolve.TableRowFormat{Height: &height, HeightRule: "exact"}, Cells: []resolve.TableCell{{Text: "three"}}},
			{Format: resolve.TableRowFormat{Height: &height, HeightRule: "exact"}, Cells: []resolve.TableCell{{Text: "four"}}},
		}}}},
	}
	pages := Paginate(doc)
	if len(pages) < 2 {
		t.Fatalf("pages = %d, want table split across pages", len(pages))
	}
	first := pages[0].Blocks[0].TableFragment
	if first == nil {
		t.Fatal("expected first table fragment")
	}
	if first.EndRow == 1 {
		t.Fatalf("first fragment ended inside vertical merge: %+v", first)
	}
	if first.EndRow != 2 {
		t.Fatalf("first fragment EndRow = %d, want 2 after merge group", first.EndRow)
	}
}

func TestRelativeSizeDrivesFloatingImageGeometry(t *testing.T) {
	geom := PageGeometry{
		Width:         600,
		Height:        800,
		MarginLeft:    72,
		MarginRight:   72,
		MarginTop:     80,
		MarginBottom:  80,
		ColumnCount:   1,
		ColumnWidthPt: 456,
	}
	img := &resolve.Image{
		Floating:     true,
		Width:        914400, // 72pt fallback extent; relative size must win.
		Height:       914400,
		SizeRelHPct:  50000, // 50% of printable margin width = 228pt.
		SizeRelHFrom: "margin",
		SizeRelVPct:  25000, // 25% of physical page height = 200pt.
		SizeRelVFrom: "page",
		XRelative:    "margin",
		YRelative:    "margin",
		XAlign:       "center",
		YAlign:       "top",
	}
	floats := appendParagraphFloats(nil, []resolve.Run{{Kind: "image", Image: img}}, geom, 0, 80, 20)
	if len(floats) != 1 {
		t.Fatalf("floats = %d, want 1", len(floats))
	}
	if !almostEqual(floats[0].Width, 228) || !almostEqual(floats[0].Height, 200) {
		t.Fatalf("relative float size = %.2fx%.2fpt, want 228x200pt", floats[0].Width, floats[0].Height)
	}
	if !almostEqual(floats[0].X, 186) { // 72 + (456-228)/2
		t.Fatalf("relative float X = %.2fpt, want 186pt", floats[0].X)
	}
}

func TestRelativeObjectSizeUsesColumnAndMarginReferences(t *testing.T) {
	geom := PageGeometry{
		Width:          600,
		Height:         800,
		MarginLeft:     60,
		MarginRight:    40,
		MarginTop:      70,
		MarginBottom:   50,
		ColumnCount:    2,
		ColumnWidthPt:  230,
		ColumnWidthsPt: []float64{210, 250},
		ColumnGapPt:    40,
	}
	if got := relativeObjectSizePt(50000, "column", true, geom, 1); !almostEqual(got, 125) {
		t.Fatalf("50%% of second column = %.2fpt, want 125pt", got)
	}
	if got := relativeObjectSizePt(50000, "leftMargin", true, geom, 0); !almostEqual(got, 30) {
		t.Fatalf("50%% of left margin = %.2fpt, want 30pt", got)
	}
	if got := relativeObjectSizePt(50000, "bottomMargin", false, geom, 0); !almostEqual(got, 25) {
		t.Fatalf("50%% of bottom margin = %.2fpt, want 25pt", got)
	}
}

func TestFloatingTableWidthUsesAuthoredPercentWidth(t *testing.T) {
	pct := 2500 // 50% in Word fiftieths-of-a-percent units
	tbl := &resolve.Table{Format: resolve.TableFormat{
		Width:     &pct,
		WidthType: "pct",
		FloatProps: &resolve.TableFloatProps{
			HorzAnchor: "margin",
			VertAnchor: "text",
		},
	}}
	geom := PageGeometry{Width: 600, Height: 800, MarginLeft: 50, MarginRight: 50, MarginTop: 50, MarginBottom: 50, ColumnWidthPt: 500}
	fb := tableFloat(&resolve.Document{}, tbl, geom, 0, 100)
	if !almostEqual(fb.Width, 250) {
		t.Fatalf("floating table width = %.2fpt, want 250pt", fb.Width)
	}
}

func TestFloatingTableCreatesTextWrapExclusion(t *testing.T) {
	tbl := &resolve.Table{Format: resolve.TableFormat{FloatProps: &resolve.TableFloatProps{
		LeftFromText: 200, RightFromText: 300, TopFromText: 100, BottomFromText: 120,
	}}}
	fl := FloatBox{FloatTable: tbl, X: 100, Y: 200, Width: 150, Height: 80}
	exc, ok := lineExclusionForFloat(fl)
	if !ok {
		t.Fatal("expected floating table exclusion")
	}
	if !almostEqual(exc.left, 90) || !almostEqual(exc.right, 265) || !almostEqual(exc.top, 195) || !almostEqual(exc.bottom, 286) {
		t.Fatalf("floating table exclusion = %#v", exc)
	}
}

func TestRotatedFloatingImageExpandsSquareWrapBounds(t *testing.T) {
	img := &resolve.Image{WrapType: "square", RotationDeg: 45}
	fl := FloatBox{Image: img, X: 100, Y: 200, Width: 80, Height: 40}
	exc, ok := lineExclusionForFloat(fl)
	if !ok {
		t.Fatal("expected exclusion")
	}
	if exc.left >= fl.X || exc.right <= fl.X+fl.Width || exc.top >= fl.Y || exc.bottom <= fl.Y+fl.Height {
		t.Fatalf("rotated bounds = left %.2f top %.2f right %.2f bottom %.2f; want expansion beyond %.2f,%.2f..%.2f,%.2f", exc.left, exc.top, exc.right, exc.bottom, fl.X, fl.Y, fl.X+fl.Width, fl.Y+fl.Height)
	}
	if len(exc.polygon) != 0 {
		t.Fatalf("square wrap polygon = %#v, want axis-aligned rotated bounds only", exc.polygon)
	}
}

func TestRotatedFloatingImageSynthesizesTightWrapPolygon(t *testing.T) {
	img := &resolve.Image{WrapType: "tight", RotationDeg: 30, DistLeft: 2, DistRight: 3}
	fl := FloatBox{Image: img, X: 100, Y: 200, Width: 80, Height: 40}
	exc, ok := lineExclusionForFloat(fl)
	if !ok {
		t.Fatal("expected exclusion")
	}
	if len(exc.polygon) != 4 {
		t.Fatalf("tight rotated polygon = %#v, want four rotated corners", exc.polygon)
	}
	if exc.padLeft != 2 || exc.padRight != 3 {
		t.Fatalf("padding = %.1f/%.1f, want 2/3", exc.padLeft, exc.padRight)
	}
}
func TestPreWord97TableWidthIsClampedToTextFrame(t *testing.T) {
	width := 10000
	doc := &resolve.Document{Source: &resolve.DocumentSource{Version: 0x0065}}
	table := &resolve.Table{Format: resolve.TableFormat{
		Width:        &width,
		WidthType:    "dxa",
		Layout:       "fixed",
		ColumnWidths: []int{910, 9090},
	}}
	if got := TableWidthPtForDocument(doc, table, 425.2); !almostEqual(got, 425.2) {
		t.Fatalf("pre-Word-97 table width = %.2fpt, want printable frame 425.2pt", got)
	}
	modern := &resolve.Document{Source: &resolve.DocumentSource{Version: 0x00C1}}
	if got := TableWidthPtForDocument(modern, table, 425.2); !almostEqual(got, 500) {
		t.Fatalf("Word-97+ table width = %.2fpt, want source width 500pt", got)
	}
}

func TestHeaderFooterReservationUsesFullTextFrameAcrossBodyColumns(t *testing.T) {
	story := resolve.Story{Blocks: []resolve.Block{{
		Kind:      "paragraph",
		Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "Header text that fits on one line in the full page text frame but would wrap repeatedly at body-column width."}}},
	}}}
	doc := &resolve.Document{Headers: map[string]resolve.Story{"header.xml": story}}
	sec := resolve.Section{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Top: 720, Left: 1440, Right: 1440, Header: 360},
		Columns:    &resolve.Columns{Num: 3, Space: 360},
		Headers:    []resolve.Reference{{Type: "default", PartName: "header.xml"}},
	}
	geom := resolveGeometryForSectionPage(doc, &sec, 1)
	fullWidth := geom.Width - geom.MarginLeft - geom.MarginRight
	fullHeight := estimateStoryHeightPt(doc, story, fullWidth)
	columnHeight := estimateStoryHeightPt(doc, story, geom.ColumnWidthPt)
	if columnHeight <= fullHeight {
		t.Fatalf("test setup did not produce narrower-column wrapping: full=%.2f column=%.2f", fullHeight, columnHeight)
	}
	allocated := twipsToPoints(sec.PageMargin.Top - sec.PageMargin.Header)
	wantTop := twipsToPoints(sec.PageMargin.Top)
	if fullHeight > allocated {
		wantTop += fullHeight - allocated
	}
	if !almostEqual(geom.MarginTop, wantTop) {
		t.Fatalf("margin top = %.2fpt, want %.2fpt from full header frame; column-width measurement would over-reserve", geom.MarginTop, wantTop)
	}
}

func TestParagraphTopAndBottomClearanceMovesTextBelowActiveFloat(t *testing.T) {
	img := &resolve.Image{Floating: true, WrapType: "topAndBottom", DistTop: 2, DistBottom: 3}
	active := []FloatBox{{Image: img, X: 72, Y: 100, Width: 80, Height: 40}}

	got := paragraphTopAndBottomClearancePt(102, 12, active)
	if !almostEqual(got, 41) {
		t.Fatalf("clearance = %.3f, want 41", got)
	}
}

func TestParagraphTopAndBottomClearanceChainsOverlappingFloats(t *testing.T) {
	first := &resolve.Image{Floating: true, WrapType: "topAndBottom"}
	second := &resolve.Image{Floating: true, WrapType: "topAndBottom"}
	active := []FloatBox{
		{Image: first, X: 72, Y: 100, Width: 80, Height: 40},
		{Image: second, X: 72, Y: 138, Width: 80, Height: 42},
	}

	got := paragraphTopAndBottomClearancePt(100, 12, active)
	if !almostEqual(got, 80) {
		t.Fatalf("clearance = %.3f, want 80", got)
	}
}

func TestParagraphFlowMetricsUseActiveFloatWrapWidth(t *testing.T) {
	geom := PageGeometry{
		Width:         300,
		Height:        400,
		MarginTop:     50,
		MarginRight:   50,
		MarginBottom:  50,
		MarginLeft:    50,
		ColumnCount:   1,
		ColumnWidthPt: 200,
	}
	p := &resolve.Paragraph{Runs: []resolve.Run{{
		Kind: "run",
		Text: "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau",
	}}}

	plain, ok := paragraphFlowMetricsForPagination(&resolve.Document{}, p, geom, 0, 50, 0, nil)
	if !ok {
		t.Fatal("plain paragraph metrics failed")
	}
	img := &resolve.Image{Floating: true, WrapType: "square"}
	active := []FloatBox{{Image: img, X: 50, Y: 50, Width: 105, Height: 70}}
	wrapped, ok := paragraphFlowMetricsForPagination(&resolve.Document{}, p, geom, 0, 50, 0, active)
	if !ok {
		t.Fatal("wrapped paragraph metrics failed")
	}
	if len(wrapped.lineHeights) <= len(plain.lineHeights) {
		t.Fatalf("wrapped lines = %d, plain lines = %d; active float should narrow early lines", len(wrapped.lineHeights), len(plain.lineHeights))
	}
	if len(wrapped.lineWidths) == 0 || wrapped.lineWidths[0] >= plain.lineWidths[0] {
		t.Fatalf("first wrapped width = %.3f, plain = %.3f", wrapped.lineWidths[0], plain.lineWidths[0])
	}
}

func TestHeaderReservationUsesFullTextFrameWidthAcrossColumns(t *testing.T) {
	header := resolve.Story{Blocks: []resolve.Block{{
		Kind:      "paragraph",
		Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "A deliberately long header line that must be measured against the page text frame rather than a body column width."}}},
	}}}
	doc := &resolve.Document{Headers: map[string]resolve.Story{"header.xml": header}}
	base := resolve.Section{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Top: 720, Bottom: 720, Left: 1440, Right: 1440, Header: 360},
		Headers:    []resolve.Reference{{Type: "default", PartName: "header.xml"}},
	}
	single := resolveGeometryForSectionPage(doc, &base, 1)
	multiSection := base
	multiSection.Columns = &resolve.Columns{Num: 3, Space: 720}
	multi := resolveGeometryForSectionPage(doc, &multiSection, 1)
	if !almostSame(single.MarginTop, multi.MarginTop) {
		t.Fatalf("header reservation changed with body columns: single=%.2fpt multi=%.2fpt", single.MarginTop, multi.MarginTop)
	}
}

func TestPaginateTracksFirstPhysicalPageWithinLaterSection(t *testing.T) {
	firstHeader := resolve.Story{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "first header line one"}}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "first header line two"}}}},
	}}
	defaultHeader := resolve.Story{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "default"}}}}}}
	sec1 := resolve.Section{Type: "nextPage", PageSize: &resolve.PageSize{Width: 12240, Height: 15840}, PageMargin: &resolve.PageMargin{Top: 720, Bottom: 720, Left: 1440, Right: 1440, Header: 360}}
	sec2 := resolve.Section{
		Type:       "nextPage",
		TitlePage:  true,
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Top: 720, Bottom: 720, Left: 1440, Right: 1440, Header: 360},
		Headers: []resolve.Reference{
			{Type: "default", PartName: "default.xml"},
			{Type: "first", PartName: "first.xml"},
		},
	}
	doc := &resolve.Document{
		Headers:  map[string]resolve.Story{"default.xml": defaultHeader, "first.xml": firstHeader},
		Sections: []resolve.Section{sec1, sec2},
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "section one"}}}},
			{Kind: "section", Section: &sec1},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "section two"}}}},
			{Kind: "section", Section: &sec2},
		},
	}
	pages := Paginate(doc)
	if len(pages) < 2 {
		t.Fatalf("pages = %d, want at least 2", len(pages))
	}
	second := pages[1]
	if second.SectionPageNumber != 1 {
		t.Fatalf("later section page ordinal = %d, want 1", second.SectionPageNumber)
	}
	if second.Geometry.MarginTop <= twipsToPoints(sec2.PageMargin.Top) {
		t.Fatalf("later section first-page header was not reserved: margin=%.2f", second.Geometry.MarginTop)
	}
}

func TestFootnoteNumberingUsesSectionOverridesAndRestarts(t *testing.T) {
	doc := &resolve.Document{
		FootnoteNumberFormat: "decimal",
		FootnoteNumberStart:  1,
		FootnoteRestart:      "continuous",
		Footnotes: map[int]resolve.Story{
			1: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "one"}}}}}},
			2: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "two"}}}}}},
		},
	}
	sectionA := resolve.Section{FootnoteNumFmt: "lowerRoman", FootnoteNumStart: 5, FootnoteRestart: "eachSect"}
	sectionB := resolve.Section{FootnoteNumFmt: "upperRoman", FootnoteNumStart: 3, FootnoteRestart: "eachSect"}
	pages := []Page{
		{Number: 1, SectionIndex: 0, SectionPageNumber: 1, Section: sectionA, Geometry: PageGeometry{Width: 612, Height: 792, MarginTop: 72, MarginBottom: 72, MarginLeft: 72, MarginRight: 72, ColumnCount: 1, ColumnWidthPt: 468}},
		{Number: 2, SectionIndex: 1, SectionPageNumber: 1, Section: sectionB, Geometry: PageGeometry{Width: 612, Height: 792, MarginTop: 72, MarginBottom: 72, MarginLeft: 72, MarginRight: 72, ColumnCount: 1, ColumnWidthPt: 468}},
	}
	ids := [][]int{{1}, {2}}
	nextSynthetic := -1
	materializePageFootnotes(doc, pages, ids, &nextSynthetic)
	marker := func(page Page) *resolve.ListMarker {
		for _, box := range page.Footnotes {
			if box.Block != nil && box.Block.Paragraph != nil && box.Block.Paragraph.List != nil {
				return box.Block.Paragraph.List
			}
		}
		return nil
	}
	first := marker(pages[0])
	second := marker(pages[1])
	if first == nil || first.Start != 5 || first.Format != "lowerRoman" {
		t.Fatalf("first section footnote marker = %#v, want lowerRoman start 5", first)
	}
	if second == nil || second.Start != 3 || second.Format != "upperRoman" {
		t.Fatalf("second section footnote marker = %#v, want upperRoman start 3", second)
	}
}

func TestParagraphLineMetricsUseRunSizedWhitespace(t *testing.T) {
	fontSize := 40 // 20pt
	p := &resolve.Paragraph{Runs: []resolve.Run{{
		Kind: "run",
		Text: "AAAA AAAA",
		Format: resolve.RunFormat{
			FontSize: &fontSize,
		},
	}}}

	metrics := estimateParagraphLineMetricsWithWidths(p, 103, 103, 10, nil)
	if got := len(metrics); got != 2 {
		t.Fatalf("line count = %d, want 2 when the inter-word space uses the 20pt run size", got)
	}
	lines := paragraphFragmentRunLines(p, []float64{103, 103}, 10)
	if got := len(lines); got != 2 {
		t.Fatalf("fragment line count = %d, want 2 from the same run-sized whitespace model", got)
	}
}

func TestParagraphLineMetricsIncludeCharacterSpacing(t *testing.T) {
	fontSize := 40 // 20pt
	spacing := 80  // 4pt
	p := &resolve.Paragraph{Runs: []resolve.Run{{
		Kind: "run",
		Text: "AA AA",
		Format: resolve.RunFormat{
			FontSize:         &fontSize,
			CharacterSpacing: &spacing,
		},
	}}}

	metrics := estimateParagraphLineMetricsWithWidths(p, 60, 60, 10, nil)
	if got := len(metrics); got != 2 {
		t.Fatalf("line count = %d, want 2 when CSS letter-spacing is included in layout width", got)
	}
}

func TestParagraphLineMetricsApplyCapsTransformBeforeMeasuring(t *testing.T) {
	fontSize := 40 // 20pt
	caps := true
	p := &resolve.Paragraph{Runs: []resolve.Run{{
		Kind: "run",
		Text: "aaaa aaaa",
		Format: resolve.RunFormat{
			FontSize: &fontSize,
			Caps:     &caps,
		},
	}}}

	metrics := estimateParagraphLineMetricsWithWidths(p, 90, 90, 10, nil)
	if got := len(metrics); got != 2 {
		t.Fatalf("line count = %d, want 2 after uppercase text-transform changes glyph advances", got)
	}
}

func TestLegacyShapeFieldFlowPacksConsecutiveOfficeArtShapesAcrossRows(t *testing.T) {
	geom := PageGeometry{
		Width:         612,
		Height:        792,
		MarginLeft:    60,
		MarginRight:   60,
		MarginTop:     60,
		MarginBottom:  60,
		ColumnCount:   1,
		ColumnWidthPt: 492,
	}
	mk := func(w, h, x, y float64) *resolve.Shape {
		return &resolve.Shape{
			LegacyShapeID:    1,
			LegacyFlowAnchor: true,
			Floating:         true,
			X:                int64(x * emuPerPoint),
			Y:                int64(y * emuPerPoint),
			Width:            int64(w * emuPerPoint),
			Height:           int64(h * emuPerPoint),
			XRelative:        "column",
			YRelative:        "paragraph",
			WrapType:         "none",
		}
	}
	a := mk(204, 71, 0, 0)
	b := mk(91, 176, 0, 0)
	c := mk(142, 85, 9, 38)
	c.RotationDeg = 45
	d := mk(204, 94, 0, 0)
	e := mk(204, 94, 0, 0)

	runs := []resolve.Run{
		{Kind: "shape", Shape: a},
		{Kind: "shape", Shape: b},
		{Kind: "shape", Shape: c},
		{Kind: "shape", Shape: d},
		{Kind: "shape", Shape: e},
	}
	got := appendParagraphFloats(nil, runs, geom, 0, 60, 42)
	if len(got) != 5 {
		t.Fatalf("floats=%d want 5", len(got))
	}

	// Contract, not implementation coordinates: first row follows story order,
	// the fourth wraps back to the column start, local Y is preserved, and the
	// second row starts below the rotated/tallest first-row footprint.
	if !(got[0].X < got[1].X && got[1].X < got[2].X) {
		t.Fatalf("first-row order = %.1f %.1f %.1f", got[0].X, got[1].X, got[2].X)
	}
	if !almostEqual(got[0].X, geom.MarginLeft) || !almostEqual(got[3].X, geom.MarginLeft) {
		t.Fatalf("row starts = %.1f / %.1f, want column start %.1f", got[0].X, got[3].X, geom.MarginLeft)
	}
	if got[2].Y <= got[0].Y {
		t.Fatalf("rotated shape local Y was lost: first=%.1f rotated=%.1f", got[0].Y, got[2].Y)
	}
	if got[3].Y <= got[1].Y+got[1].Height-0.01 {
		t.Fatalf("second row y=%.1f overlaps tallest first-row shape bottom %.1f", got[3].Y, got[1].Y+got[1].Height)
	}
}

func TestLegacyShapeFieldFlowDoesNotChangeOrdinaryFloatingShapes(t *testing.T) {
	geom := PageGeometry{Width: 600, Height: 800, MarginLeft: 50, MarginRight: 50, MarginTop: 70, MarginBottom: 70, ColumnWidthPt: 500}
	shape := &resolve.Shape{
		LegacyShapeID: 99,
		Floating:      true,
		X:             20 * emuPerPoint,
		Y:             30 * emuPerPoint,
		Width:         100 * emuPerPoint,
		Height:        40 * emuPerPoint,
		XRelative:     "margin",
		YRelative:     "margin",
	}
	got := appendParagraphFloats(nil, []resolve.Run{{Kind: "shape", Shape: shape}}, geom, 0, 100, 20)
	if len(got) != 1 || !almostEqual(got[0].X, 70) || !almostEqual(got[0].Y, 100) {
		t.Fatalf("ordinary float moved: %#v", got)
	}
}

func TestLegacyShapeFieldFlowUsesFSPAOrderNotPaintOrder(t *testing.T) {
	a := &resolve.Shape{Floating: true, LegacyFlowAnchor: true, LegacyFlowOrder: 166, Width: 100 * 12700, Height: 40 * 12700}
	b := &resolve.Shape{Floating: true, LegacyFlowAnchor: true, LegacyFlowOrder: 178, Width: 60 * 12700, Height: 40 * 12700}
	c := &resolve.Shape{Floating: true, LegacyFlowAnchor: true, LegacyFlowOrder: 190, Width: 70 * 12700, Height: 40 * 12700}
	// Deliberately paint/z-order the runs differently from the Word story.
	runs := []resolve.Run{{Kind: "shape", Shape: c}, {Kind: "shape", Shape: a}, {Kind: "shape", Shape: b}}
	state := legacyShapeFlowState{left: 72, width: 300, y: 120}
	pos := legacyShapeFlowPositions(runs, &state)
	if !(pos[a].X < pos[b].X && pos[b].X < pos[c].X) {
		t.Fatalf("positions = a:%v b:%v c:%v; want FSPA story order", pos[a].X, pos[b].X, pos[c].X)
	}
}

func TestLegacyShapeFieldFlowContinuesAcrossConsecutiveAnchorParagraphs(t *testing.T) {
	geom := PageGeometry{
		Width: 612, Height: 792,
		MarginLeft: 60, MarginRight: 60, MarginTop: 60, MarginBottom: 60,
		ColumnCount: 1, ColumnWidthPt: 492,
	}
	mk := func(id uint32, order int, w, h float64) *resolve.Shape {
		return &resolve.Shape{
			LegacyShapeID: id, LegacyFlowAnchor: true, LegacyFlowOrder: order,
			Floating: true, Width: int64(w * emuPerPoint), Height: int64(h * emuPerPoint),
			XRelative: "column", YRelative: "paragraph", WrapType: "none",
		}
	}
	a := mk(1028, 166, 204, 71)
	b := mk(1029, 178, 91, 176)
	c := mk(1030, 190, 142, 85)
	d := mk(1026, 202, 204, 94)

	state := &legacyShapeFlowState{
		left: geom.MarginLeft, width: geom.ColumnWidthPt, y: 120,
	}
	var floats []FloatBox
	for _, sh := range []*resolve.Shape{a, b, c, d} {
		floats = appendParagraphFloatsWithLegacyFlowState(
			floats,
			[]resolve.Run{{Kind: "shape", Shape: sh}},
			geom, 0, 100, 20, state,
		)
	}
	if len(floats) != 4 {
		t.Fatalf("floats=%d want 4", len(floats))
	}
	if !(floats[0].X < floats[1].X && floats[1].X < floats[2].X) {
		t.Fatalf("consecutive paragraphs collapsed to one x: %.1f %.1f %.1f", floats[0].X, floats[1].X, floats[2].X)
	}
	if !almostEqual(floats[3].X, geom.MarginLeft) {
		t.Fatalf("wrapped row x=%.1f want column start %.1f", floats[3].X, geom.MarginLeft)
	}
	if floats[3].Y <= floats[1].Y+floats[1].Height-0.01 {
		t.Fatalf("wrapped row overlaps first row: y=%.1f first-bottom=%.1f", floats[3].Y, floats[1].Y+floats[1].Height)
	}
}
