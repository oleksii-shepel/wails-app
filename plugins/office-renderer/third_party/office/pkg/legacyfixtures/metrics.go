package legacyfixtures

import (
	"encoding/json"
	"fmt"
	"os"

	dml "office/pkg/ooxml/drawing"
	presentir "office/pkg/presentation/ir"
	sheetir "office/pkg/sheet/ir"
	wordir "office/pkg/word/ir"
)

// Metrics captures observed document statistics for regression detection.
type Metrics struct {
	Format    string      `json:"format"`  // "doc", "xls", "ppt"
	Fixture   string      `json:"fixture"` // fixture ID
	Timestamp string      `json:"timestamp"`
	Doc       *DocMetrics `json:"doc,omitempty"`
	Xls       *XlsMetrics `json:"xls,omitempty"`
	Ppt       *PptMetrics `json:"ppt,omitempty"`
}

// DocMetrics captures legacy DOC statistics.
type DocMetrics struct {
	ParagraphCount     int `json:"paragraph_count"`
	TableCount         int `json:"table_count"`
	InlineImageCount   int `json:"inline_image_count"`
	FloatingImageCount int `json:"floating_image_count"`
	TextBoxCount       int `json:"textbox_count"`
	FootnoteCount      int `json:"footnote_count"`
	EndnoteCount       int `json:"endnote_count"`
	CommentCount       int `json:"comment_count"`
	StyleCount         int `json:"style_count"`
	CharStyleCount     int `json:"char_style_count"`
	ListCount          int `json:"list_count"`
	SectionCount       int `json:"section_count"`
	BookmarkCount      int `json:"bookmark_count"`
	ShapeCount         int `json:"shape_count"`
	FloatingShapeCount int `json:"floating_shape_count"`
	GroupCount         int `json:"group_count"`
	TextPathCount      int `json:"text_path_count"`
	ShadowCount        int `json:"shadow_count"`
	ArrowCount         int `json:"arrow_count"`
}

// XlsMetrics captures legacy XLS statistics.
type XlsMetrics struct {
	SheetCount      int `json:"sheet_count"`
	RowCount        int `json:"row_count"`
	ColumnCount     int `json:"column_count"`
	FormulaCount    int `json:"formula_count"`
	MergedCellCount int `json:"merged_cell_count"`
	StringCellCount int `json:"string_cell_count"`
	NumberCellCount int `json:"number_cell_count"`
}

// PptMetrics captures legacy PPT statistics.
type PptMetrics struct {
	SlideCount       int `json:"slide_count"`
	ShapeCount       int `json:"shape_count"`
	TextBoxCount     int `json:"textbox_count"`
	PictureCount     int `json:"picture_count"`
	GroupCount       int `json:"group_count"`
	MasterSlideCount int `json:"master_slide_count"`
}

// MetricsBaseline is a collection of baseline metrics keyed by fixture ID.
type MetricsBaseline map[string]*Metrics

// LoadMetricsBaseline loads the baseline metrics from a JSON file.
func LoadMetricsBaseline(filePath string) (MetricsBaseline, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read baseline file: %w", err)
	}

	var m MetricsBaseline
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal baseline: %w", err)
	}

	return m, nil
}

// SaveMetricsBaseline saves metrics to a JSON file.
func SaveMetricsBaseline(filePath string, m MetricsBaseline) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write baseline file: %w", err)
	}

	return nil
}

// CollectDocMetrics extracts metrics from a parsed legacy DOC document.
func CollectDocMetrics(doc *wordir.Document) *DocMetrics {
	if doc == nil {
		return &DocMetrics{}
	}

	m := &DocMetrics{
		StyleCount:     len(doc.Styles),
		CharStyleCount: len(doc.CharacterStyles),
		SectionCount:   len(doc.Sections),
		FootnoteCount:  len(doc.Footnotes),
		EndnoteCount:   len(doc.Endnotes),
		CommentCount:   len(doc.Comments),
	}

	// Count paragraphs and media at top level.
	countBlocksAndMedia(doc.Blocks, m)

	return m
}

// countBlocksAndMedia recursively counts paragraph, table, image, and textbox metrics.
func countBlocksAndMedia(blocks []wordir.Block, m *DocMetrics) {
	for _, b := range blocks {
		if b.Kind == "paragraph" && b.Paragraph != nil {
			m.ParagraphCount++
			if b.Paragraph.List != nil {
				m.ListCount++
			}
			m.BookmarkCount += len(b.Paragraph.Bookmarks)
			countRunMedia(b.Paragraph.Runs, m)
		} else if b.Kind == "table" && b.Table != nil {
			m.TableCount++
			for _, row := range b.Table.Rows {
				for _, cell := range row.Cells {
					countBlocksAndMedia(cell.Blocks, m)
				}
			}
		}
	}
}

// countRunMedia counts inline images, textboxes, and OfficeArt shapes within runs.
func countRunMedia(runs []wordir.Run, m *DocMetrics) {
	for _, r := range runs {
		if r.Image != nil {
			if r.Image.Floating {
				m.FloatingImageCount++
			} else {
				m.InlineImageCount++
			}
		}
		if r.TextBox != nil {
			m.TextBoxCount++
			countBlocksAndMedia(r.TextBox.Blocks, m)
		}
		if r.Shape != nil {
			countWordShape(r.Shape, m)
		}
	}
}

func countWordShape(shape *wordir.Shape, m *DocMetrics) {
	if shape == nil {
		return
	}
	m.ShapeCount++
	if shape.Floating {
		m.FloatingShapeCount++
	}
	if shape.Kind == "group" || len(shape.Children) > 0 {
		m.GroupCount++
	}
	if shape.TextPath != nil && shape.TextPath.String != "" {
		m.TextPathCount++
	}
	if shape.LineStyle != nil {
		if shape.LineStyle.HeadEnd != nil && shape.LineStyle.HeadEnd.Type != dml.ArrowheadNone {
			m.ArrowCount++
		}
		if shape.LineStyle.TailEnd != nil && shape.LineStyle.TailEnd.Type != dml.ArrowheadNone {
			m.ArrowCount++
		}
	}
	for _, effect := range shape.Effects.Effects {
		if effect != nil && effect.Type() == dml.EffectTypeShadow {
			m.ShadowCount++
		}
	}
	if shape.TextBox != nil {
		m.TextBoxCount++
		countBlocksAndMedia(shape.TextBox.Blocks, m)
	}
	for i := range shape.Children {
		countWordShape(&shape.Children[i], m)
	}
}

// CollectXlsMetrics extracts metrics from a parsed legacy XLS workbook.
func CollectXlsMetrics(workbook *sheetir.Workbook) *XlsMetrics {
	if workbook == nil {
		return &XlsMetrics{}
	}

	m := &XlsMetrics{
		SheetCount: len(workbook.Sheets),
	}

	for _, sheet := range workbook.Sheets {
		m.RowCount += len(sheet.Rows)
		m.MergedCellCount += len(sheet.MergedCells)
		for _, row := range sheet.Rows {
			m.ColumnCount += len(row.Cells)
			for _, cell := range row.Cells {
				if cell == nil {
					continue
				}
				if cell.Formula != "" {
					m.FormulaCount++
				}
				switch cell.Kind {
				case sheetir.CellString:
					m.StringCellCount++
				case sheetir.CellNumber, sheetir.CellDate:
					m.NumberCellCount++
				}
			}
		}
	}

	return m
}

// CollectPptMetrics extracts metrics from a parsed legacy PPT presentation.
func CollectPptMetrics(pres *presentir.Presentation) *PptMetrics {
	if pres == nil {
		return &PptMetrics{}
	}

	m := &PptMetrics{
		SlideCount: len(pres.Slides),
	}

	for _, slide := range pres.Slides {
		countPresentationShapes(slide.Shapes, m)
		if slide.Header != nil {
			countPresentationShapes([]*presentir.Shape{slide.Header}, m)
		}
		if slide.Footer != nil {
			countPresentationShapes([]*presentir.Shape{slide.Footer}, m)
		}
		if slide.DateTime != nil {
			countPresentationShapes([]*presentir.Shape{slide.DateTime}, m)
		}
	}

	return m
}

func countPresentationShapes(shapes []*presentir.Shape, m *PptMetrics) {
	for _, shape := range shapes {
		if shape == nil {
			continue
		}
		m.ShapeCount++
		switch shape.Kind {
		case presentir.ShapeText:
			m.TextBoxCount++
		case presentir.ShapeImage:
			m.PictureCount++
		case presentir.ShapeGroup:
			m.GroupCount++
		}
		countPresentationShapes(shape.Children, m)
	}
}

// MetricsBaselinePath returns the path to the fixture metrics baseline JSON file.
func MetricsBaselinePath() string {
	return Path("docs/legacy-fixture-metrics-baseline.json")
}
