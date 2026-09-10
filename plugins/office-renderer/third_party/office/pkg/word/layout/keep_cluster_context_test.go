package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestKeepNextClusterMeasurementCarriesFlowYAcrossBlocks(t *testing.T) {
	before := 200
	after := 120
	blocks := []resolve.Block{
		{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "first",
				Format: resolve.ParagraphFormat{SpacingAfter: &after, KeepNext: true},
				Runs: []resolve.Run{{Kind: "run", Text: "first"}},
			},
		},
		{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "second",
				Format: resolve.ParagraphFormat{SpacingBefore: &before},
				Runs: []resolve.Run{{Kind: "run", Text: "second"}},
			},
		},
	}
	geom := PageGeometry{Width: 300, Height: 400, MarginLeft: 20, MarginRight: 20, MarginTop: 20, MarginBottom: 20, ColumnCount: 1, ColumnWidthPt: 260}
	doc := &resolve.Document{}

	got := measureBlockClusterHeightAt(doc, blocks, 0, 1, geom, 0, 50, nil)
	h0, _ := measureBlock(doc, blocks, 0, geom, 0, 50, nil)
	h1, _ := measureBlock(doc, blocks, 1, geom, 0, 50+h0, nil)
	want := h0 + h1
	if got != want {
		t.Fatalf("contextual keep cluster height = %.2fpt, want %.2fpt", got, want)
	}
}

func TestKeepNextClusterMeasurementUsesActiveFloatGeometry(t *testing.T) {
	text := "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen"
	blocks := []resolve.Block{
		{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: text,
				Format: resolve.ParagraphFormat{KeepNext: true},
				Runs: []resolve.Run{{Kind: "run", Text: text}},
			},
		},
		{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Text: "tail",
				Runs: []resolve.Run{{Kind: "run", Text: "tail"}},
			},
		},
	}
	geom := PageGeometry{Width: 220, Height: 400, MarginLeft: 20, MarginRight: 20, MarginTop: 20, MarginBottom: 20, ColumnCount: 1, ColumnWidthPt: 180}
	float := FloatBox{
		X: 20, Y: 20, Width: 100, Height: 100,
		FloatTable: &resolve.Table{
			Format: resolve.TableFormat{
				FloatProps: &resolve.TableFloatProps{},
			},
		},
	}
	doc := &resolve.Document{}

	clear := measureBlockClusterHeightAt(doc, blocks, 0, 1, geom, 0, 0, nil)
	constrained := measureBlockClusterHeightAt(doc, blocks, 0, 1, geom, 0, 0, []FloatBox{float})
	if constrained < clear {
		t.Fatalf("float-constrained keep cluster = %.2fpt, clear = %.2fpt; contextual preflight must not measure shorter", constrained, clear)
	}
}

func TestKeepNextClusterCanTerminateInTable(t *testing.T) {
	blocks := []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Format: resolve.ParagraphFormat{KeepNext: true}}},
		{Kind: "table", Table: &resolve.Table{}},
	}
	if got := keepNextClusterEnd(blocks, 0); got != 1 {
		t.Fatalf("paragraph->table keep cluster end = %d, want 1", got)
	}
}
