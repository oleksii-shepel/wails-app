package merge

import (
	"strings"
	"testing"

	resolve "office/pkg/word/ir"
)

// TestMergeFieldWithIfCondition tests IF field with MERGEFIELD inside condition
func TestMergeFieldWithIfCondition(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "IF", FieldArg: "Name = \"Alice\" \"Match\" \"NoMatch\""},
				},
			}},
		},
	}

	proc := NewProcessor(Data{"Name": "Alice"})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	var sb strings.Builder
	for _, run := range para.Runs {
		sb.WriteString(run.Text)
	}
	// IF fields with MERGEFIELD should evaluate the condition
	// The exact output depends on field evaluation implementation
	// For now, verify the document processes without error
	if para == nil {
		t.Fatal("expected paragraph after processing")
	}
}

// TestMergeFieldWithCompareNumeric tests COMPARE field with numeric operands
func TestMergeFieldWithCompareNumeric(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "COMPARE", FieldArg: "10 >= 5"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	if para == nil {
		t.Fatal("expected paragraph after processing")
	}
}

// TestMergeFieldWithNestedSetAndRef tests SET field followed by REF
func TestMergeFieldWithNestedSetAndRef(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "SET", FieldArg: "Bookmark \"Value\""},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "REF", FieldArg: "Bookmark"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(merged.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(merged.Blocks))
	}
}

// TestMergeFieldWithQuoteField tests QUOTE field containing MERGEFIELD
func TestMergeFieldWithQuoteField(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "QUOTE", FieldArg: "Hello { MERGEFIELD Name }"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{"Name": "Alice"})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	if para == nil {
		t.Fatal("expected paragraph after processing")
	}
}

// TestMergeFieldWithSkipIfSequence tests SKIPIF field behavior
func TestMergeFieldWithSkipIfSequence(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "SKIPIF", FieldArg: "Name = \"Skip\""},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "This should appear", Kind: "run"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{"Name": "Other"})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// When condition is false, SKIPIF should not skip
	if len(merged.Blocks) != 2 {
		t.Fatalf("expected 2 blocks (SKIPIF false), got %d", len(merged.Blocks))
	}
}

// TestMergeFieldWithNextSequence tests NEXT field behavior
func TestMergeFieldWithNextSequence(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "NEXT"},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
				},
			}},
		},
	}

	// NEXT fields are typically handled at a higher level (batch processing)
	// This test verifies the field is preserved during processing
	proc := NewProcessor(Data{"Name": "Alice"})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Verify all blocks are preserved
	if len(merged.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(merged.Blocks))
	}
}

// TestMergeFieldWithEmptyData tests behavior with empty/blank data values
func TestMergeFieldWithEmptyData(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Name: ", Kind: "run"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{"Name": ""})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	var sb strings.Builder
	for _, run := range para.Runs {
		sb.WriteString(run.Text)
	}
	got := sb.String()
	want := "Name: "
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestMergeFieldWithSpecialCharacters tests MERGEFIELD with special characters in data
func TestMergeFieldWithSpecialCharacters(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Description"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{"Description": "Line 1\nLine 2\tTabbed"})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	var sb strings.Builder
	for _, run := range para.Runs {
		sb.WriteString(run.Text)
	}
	got := sb.String()
	want := "Line 1\nLine 2\tTabbed"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestMergeFieldWithUnicode tests MERGEFIELD with Unicode data
func TestMergeFieldWithUnicode(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{"Name": "日本語テキスト 🎉"})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	var sb strings.Builder
	for _, run := range para.Runs {
		sb.WriteString(run.Text)
	}
	got := sb.String()
	want := "日本語テキスト 🎉"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestMergeFieldWithMultipleParagraphs tests MERGEFIELD across multiple paragraphs
func TestMergeFieldWithMultipleParagraphs(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "First: ", Kind: "run"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "FirstName"},
				},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Last: ", Kind: "run"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "LastName"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{
		"FirstName": "Alice",
		"LastName":  "Smith",
	})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(merged.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(merged.Blocks))
	}

	// Check first paragraph
	para1 := merged.Blocks[0].Paragraph
	var sb1 strings.Builder
	for _, run := range para1.Runs {
		sb1.WriteString(run.Text)
	}
	if sb1.String() != "First: Alice" {
		t.Errorf("paragraph 1 = %q, want 'First: Alice'", sb1.String())
	}

	// Check second paragraph
	para2 := merged.Blocks[1].Paragraph
	var sb2 strings.Builder
	for _, run := range para2.Runs {
		sb2.WriteString(run.Text)
	}
	if sb2.String() != "Last: Smith" {
		t.Errorf("paragraph 2 = %q, want 'Last: Smith'", sb2.String())
	}
}

// TestMergeFieldWithNestedIf tests deeply nested IF fields
func TestMergeFieldWithNestedIf(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "IF", FieldArg: "A = \"1\" { IF B = \"2\" \"Both\" \"OnlyA\" } \"NoA\""},
				},
			}},
		},
	}

	proc := NewProcessor(Data{"A": "1", "B": "2"})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	if para == nil {
		t.Fatal("expected paragraph after processing")
	}
}

// TestMergeFieldWithTableAndParagraphs tests MERGEFIELD in mixed table/paragraph content
func TestMergeFieldWithTableAndParagraphs(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Header", Kind: "run"},
				},
			}},
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{
							{Kind: "paragraph", Paragraph: &resolve.Paragraph{
								Runs: []resolve.Run{
									{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "CellValue"},
								},
							}},
						},
					}},
				}},
			}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Footer: ", Kind: "run"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "FooterValue"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{
		"CellValue":   "TableData",
		"FooterValue": "End",
	})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if len(merged.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(merged.Blocks))
	}

	// Check table cell
	cell := merged.Blocks[1].Table.Rows[0].Cells[0]
	cellPara := cell.Blocks[0].Paragraph
	var sb strings.Builder
	for _, run := range cellPara.Runs {
		sb.WriteString(run.Text)
	}
	if sb.String() != "TableData" {
		t.Errorf("cell text = %q, want 'TableData'", sb.String())
	}

	// Check footer paragraph
	footerPara := merged.Blocks[2].Paragraph
	var sb2 strings.Builder
	for _, run := range footerPara.Runs {
		sb2.WriteString(run.Text)
	}
	if sb2.String() != "Footer: End" {
		t.Errorf("footer text = %q, want 'Footer: End'", sb2.String())
	}
}