package legacyfixtures

import (
	"testing"

	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
	sheetir "office/pkg/sheet/ir"
	wordir "office/pkg/word/ir"
)

func TestCollectDocMetricsCountsNestedListsBookmarksAndTextboxes(t *testing.T) {
	doc := &wordir.Document{
		Styles:          map[string]wordir.Style{"Normal": {ID: "Normal"}},
		CharacterStyles: map[string]wordir.CharacterStyle{"Emphasis": {ID: "Emphasis"}},
		Sections:        []wordir.Section{{}},
		Footnotes:       map[int]wordir.Story{1: {}},
		Endnotes:        map[int]wordir.Story{1: {}},
		Comments:        map[int]wordir.Story{1: {}},
		Blocks: []wordir.Block{
			{
				Kind: "paragraph",
				Paragraph: &wordir.Paragraph{
					List:      &wordir.ListMarker{Kind: "ordered", Level: 0},
					Bookmarks: []string{"alpha", "beta"},
					Runs: []wordir.Run{
						{Image: &wordir.Image{}},
						{Image: &wordir.Image{Floating: true}},
						{TextBox: &wordir.TextBox{Blocks: []wordir.Block{{
							Kind: "paragraph",
							Paragraph: &wordir.Paragraph{
								List:      &wordir.ListMarker{Kind: "bullet", Level: 1},
								Bookmarks: []string{"gamma"},
							},
						}}}},
					},
				},
			},
			{
				Kind: "table",
				Table: &wordir.Table{Rows: []wordir.TableRow{
					{Cells: []wordir.TableCell{
						{Blocks: []wordir.Block{{
							Kind:      "paragraph",
							Paragraph: &wordir.Paragraph{Bookmarks: []string{"delta"}},
						}}},
					}},
				}},
			},
		},
	}

	metrics := CollectDocMetrics(doc)
	if metrics.ParagraphCount != 3 {
		t.Fatalf("ParagraphCount = %d, want 3", metrics.ParagraphCount)
	}
	if metrics.TableCount != 1 {
		t.Fatalf("TableCount = %d, want 1", metrics.TableCount)
	}
	if metrics.InlineImageCount != 1 {
		t.Fatalf("InlineImageCount = %d, want 1", metrics.InlineImageCount)
	}
	if metrics.FloatingImageCount != 1 {
		t.Fatalf("FloatingImageCount = %d, want 1", metrics.FloatingImageCount)
	}
	if metrics.TextBoxCount != 1 {
		t.Fatalf("TextBoxCount = %d, want 1", metrics.TextBoxCount)
	}
	if metrics.ListCount != 2 {
		t.Fatalf("ListCount = %d, want 2", metrics.ListCount)
	}
	if metrics.BookmarkCount != 4 {
		t.Fatalf("BookmarkCount = %d, want 4", metrics.BookmarkCount)
	}
	if metrics.StyleCount != 1 || metrics.CharStyleCount != 1 || metrics.SectionCount != 1 {
		t.Fatalf("style/section counts = %+v, want 1/1/1", metrics)
	}
	if metrics.FootnoteCount != 1 || metrics.EndnoteCount != 1 || metrics.CommentCount != 1 {
		t.Fatalf("note/comment counts = %+v, want 1/1/1", metrics)
	}
}

func TestCollectPptMetricsCountsKindsRecursively(t *testing.T) {
	pres := &presentationir.Presentation{
		Slides: []*presentationir.Slide{
			{
				BgColor: "FFFFFF",
				Shapes: []*presentationir.Shape{
					{Kind: presentationir.ShapeText},
					{Kind: presentationir.ShapeImage},
					{Kind: presentationir.ShapeGroup, Children: []*presentationir.Shape{{Kind: presentationir.ShapeText}, {Kind: presentationir.ShapeImage}}},
				},
				Notes: &presentationir.NotesSlide{Shapes: []*presentationir.Shape{{Kind: presentationir.ShapeText}}},
			},
		},
	}

	metrics := CollectPptMetrics(pres)
	if metrics.SlideCount != 1 {
		t.Fatalf("SlideCount = %d, want 1", metrics.SlideCount)
	}
	if metrics.ShapeCount != 5 {
		t.Fatalf("ShapeCount = %d, want 5", metrics.ShapeCount)
	}
	if metrics.TextBoxCount != 2 {
		t.Fatalf("TextBoxCount = %d, want 2", metrics.TextBoxCount)
	}
	if metrics.PictureCount != 2 {
		t.Fatalf("PictureCount = %d, want 2", metrics.PictureCount)
	}
	if metrics.GroupCount != 1 {
		t.Fatalf("GroupCount = %d, want 1", metrics.GroupCount)
	}
	if metrics.MasterSlideCount != 0 {
		t.Fatalf("MasterSlideCount = %d, want 0", metrics.MasterSlideCount)
	}
}

func TestCollectXlsMetricsCountsCellsFormulasAndMerges(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{
			{
				MergedCells: []*sheetir.MergedCell{{TopRow: 1, LeftCol: 1, BottomRow: 1, RightCol: 2}},
				Rows: []*sheetir.Row{
					{Cells: []*sheetir.Cell{
						{Kind: sheetir.CellString, Ref: "A1"},
						{Kind: sheetir.CellNumber, Ref: "B1", Formula: "SUM(A2:A3)"},
					}},
					{Cells: []*sheetir.Cell{
						{Kind: sheetir.CellDate, Ref: "A2"},
						{Kind: sheetir.CellFormula, Ref: "B2", Formula: "T(\"x\")"},
					}},
				},
			},
		},
	}

	metrics := CollectXlsMetrics(wb)
	if metrics.SheetCount != 1 {
		t.Fatalf("SheetCount = %d, want 1", metrics.SheetCount)
	}
	if metrics.RowCount != 2 {
		t.Fatalf("RowCount = %d, want 2", metrics.RowCount)
	}
	if metrics.ColumnCount != 4 {
		t.Fatalf("ColumnCount = %d, want 4", metrics.ColumnCount)
	}
	if metrics.FormulaCount != 2 {
		t.Fatalf("FormulaCount = %d, want 2", metrics.FormulaCount)
	}
	if metrics.MergedCellCount != 1 {
		t.Fatalf("MergedCellCount = %d, want 1", metrics.MergedCellCount)
	}
	if metrics.StringCellCount != 1 {
		t.Fatalf("StringCellCount = %d, want 1", metrics.StringCellCount)
	}
	if metrics.NumberCellCount != 2 {
		t.Fatalf("NumberCellCount = %d, want 2", metrics.NumberCellCount)
	}
}
func TestCollectDocMetricsCountsOfficeArtSemantics(t *testing.T) {
	shape := &wordir.Shape{
		Kind:     "group",
		Floating: true,
		LineStyle: &drawing.LineStyle{
			HeadEnd: &drawing.Arrowhead{Type: drawing.ArrowheadTriangle},
			TailEnd: &drawing.Arrowhead{Type: drawing.ArrowheadArrow},
		},
		Effects: drawing.EffectList{Effects: []drawing.Effect{
			drawing.Shadow{Color: "666666", Alpha: 0.75},
		}},
		TextPath: &wordir.TextPath{On: "t", String: "WordArt"},
		TextBox: &wordir.TextBox{Blocks: []wordir.Block{{
			Kind:      "paragraph",
			Paragraph: &wordir.Paragraph{},
		}}},
		Children: []wordir.Shape{{Kind: "rect"}, {Kind: "ellipse"}},
	}
	doc := &wordir.Document{Blocks: []wordir.Block{{
		Kind:      "paragraph",
		Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Kind: "shape", Shape: shape}}},
	}}}

	metrics := CollectDocMetrics(doc)
	if metrics.ShapeCount != 3 || metrics.FloatingShapeCount != 1 || metrics.GroupCount != 1 {
		t.Fatalf("shape metrics = %+v, want 3 shapes / 1 floating / 1 group", metrics)
	}
	if metrics.TextPathCount != 1 || metrics.ShadowCount != 1 || metrics.ArrowCount != 2 {
		t.Fatalf("OfficeArt semantic metrics = %+v, want textPath=1 shadow=1 arrows=2", metrics)
	}
	if metrics.TextBoxCount != 1 || metrics.ParagraphCount != 2 {
		t.Fatalf("textbox metrics = %+v, want textbox=1 paragraphs=2", metrics)
	}
}
