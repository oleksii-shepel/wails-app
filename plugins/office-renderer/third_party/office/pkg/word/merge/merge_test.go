package merge

import (
	"strings"
	"testing"

	resolve "office/pkg/word/ir"
)

func TestNewProcessor(t *testing.T) {
	data := Data{"Name": "Alice", "City": "NYC"}
	proc := NewProcessor(data)
	if proc == nil {
		t.Fatal("NewProcessor returned nil")
	}
	v, ok := proc.lookup("Name")
	if !ok || v != "Alice" {
		t.Errorf("lookup(Name) = %q, %v; want Alice, true", v, ok)
	}
}

func TestProcessMergeField(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Hello ", Kind: "run"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
					{Text: ", welcome to ", Kind: "run"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Company"},
					{Text: "!", Kind: "run"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{
		"Name":    "Alice Smith",
		"Company": "Acme Corp",
	})

	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	if para == nil {
		t.Fatal("expected paragraph")
	}

	var sb strings.Builder
	for _, run := range para.Runs {
		sb.WriteString(run.Text)
	}
	got := sb.String()
	want := "Hello Alice Smith, welcome to Acme Corp!"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProcessMergeFieldMissing(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Missing"},
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
	var sb strings.Builder
	for _, run := range para.Runs {
		sb.WriteString(run.Text)
	}
	got := sb.String()
	want := "«Missing»"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProcessMergeFieldWithOnMissing(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Missing"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{})
	proc.OnMissingField = func(name string) string {
		return "[N/A]"
	}

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
	want := "[N/A]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProcessMergeFieldPreserve(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{"Name": "Alice"})
	proc.PreserveFields = true

	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	if len(para.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(para.Runs))
	}
	if para.Runs[0].FieldKind != "MERGEFIELD" {
		t.Errorf("expected FieldKind MERGEFIELD, got %q", para.Runs[0].FieldKind)
	}
	if para.Runs[0].Text != "Alice" {
		t.Errorf("got Text %q, want Alice", para.Runs[0].Text)
	}
}

func TestProcessTextPlaceholder(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Hello {{Name}}, welcome to {{Company}}!", Kind: "run"},
				},
			}},
		},
	}

	proc := NewProcessor(Data{
		"Name":    "Alice",
		"Company": "Acme",
	})

	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	got := para.Runs[0].Text
	want := "Hello Alice, welcome to Acme!"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProcessMergeFieldInTable(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{
							{Kind: "paragraph", Paragraph: &resolve.Paragraph{
								Runs: []resolve.Run{
									{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
								},
							}},
						},
					}},
				}},
			}},
		},
	}

	proc := NewProcessor(Data{"Name": "Alice"})
	merged, err := proc.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	cell := merged.Blocks[0].Table.Rows[0].Cells[0]
	para := cell.Blocks[0].Paragraph
	var sb strings.Builder
	for _, run := range para.Runs {
		sb.WriteString(run.Text)
	}
	got := sb.String()
	want := "Alice"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractMergeFields(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Company"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"}, // duplicate
				},
			}},
		},
	}

	fields := ExtractMergeFields(doc)
	if len(fields) != 2 {
		t.Fatalf("expected 2 unique fields, got %d", len(fields))
	}
	if fields[0] != "Name" || fields[1] != "Company" {
		t.Errorf("got fields %v, want [Name, Company]", fields)
	}
}

func TestMergeDocument(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "Hello ", Kind: "run"},
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
				},
			}},
		},
	}

	merged, err := MergeDocument(doc, Data{"Name": "Alice"})
	if err != nil {
		t.Fatalf("MergeDocument failed: %v", err)
	}

	para := merged.Blocks[0].Paragraph
	var sb strings.Builder
	for _, run := range para.Runs {
		sb.WriteString(run.Text)
	}
	got := sb.String()
	want := "Hello Alice"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMergeDocumentWithRecords(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{Text: "", Kind: "run", FieldKind: "MERGEFIELD", FieldArg: "Name"},
				},
			}},
		},
	}

	records := []Record{
		{"Name": "Alice"},
		{"Name": "Bob"},
	}

	results, err := MergeDocumentWithRecords(doc, records)
	if err != nil {
		t.Fatalf("MergeDocumentWithRecords failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	names := []string{}
	for _, result := range results {
		para := result.Blocks[0].Paragraph
		names = append(names, para.Runs[0].Text)
	}
	if names[0] != "Alice" || names[1] != "Bob" {
		t.Errorf("got names %v, want [Alice, Bob]", names)
	}
}