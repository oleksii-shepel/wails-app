package edit

import (
	"fmt"
	"strings"

	wordir "office/pkg/word/ir"
)

// Validate checks the document for common issues and returns a slice of
// human-readable problem descriptions. An empty slice means the document
// passes basic validation.
func (d *Document) Validate() []string {
	var issues []string

	if d.IR == nil {
		return []string{"document has no IR"}
	}
	d.rebuildFlatSlices()

	// Numbering definitions.
	numDefined := make(map[int]bool)
	for _, nd := range d.IR.NumberingDefinitions {
		numDefined[nd.NumID] = true
	}
	for _, p := range d.IR.Paragraphs {
		if p.Numbering != nil && !numDefined[p.Numbering.NumID] {
			issues = append(issues, fmt.Sprintf(
				"paragraph %d references numbering ID %d with no definition",
				p.Index, p.Numbering.NumID))
		}
	}

	// Style references.
	for _, p := range d.IR.Paragraphs {
		if p.StyleID != "" {
			if _, ok := d.IR.Styles[p.StyleID]; !ok {
				issues = append(issues, fmt.Sprintf(
					"paragraph %d references missing style %q",
					p.Index, p.StyleID))
			}
		}
	}

	// Run-level checks.
	for _, p := range d.IR.Paragraphs {
		for i, r := range p.Runs {
			if r.StyleID != "" {
				if _, ok := d.IR.CharacterStyles[r.StyleID]; !ok {
					issues = append(issues, fmt.Sprintf(
						"paragraph %d run %d references missing character style %q",
						p.Index, i, r.StyleID))
				}
			}
			if r.Kind == "hyperlink" && r.Target == "" && r.Anchor == "" {
				issues = append(issues, fmt.Sprintf(
					"paragraph %d run %d is a hyperlink with no target or anchor",
					p.Index, i))
			}
			if r.Image != nil {
				if r.Image.Data == nil || len(r.Image.Data) == 0 {
					issues = append(issues, fmt.Sprintf(
						"paragraph %d run %d image has no data",
						p.Index, i))
				}
				if r.Image.MIMEType == "" {
					issues = append(issues, fmt.Sprintf(
						"paragraph %d run %d image has no MIME type",
						p.Index, i))
				}
				if strings.TrimSpace(r.Image.Alt) == "" && strings.TrimSpace(r.Image.Title) == "" {
					issues = append(issues, fmt.Sprintf(
						"paragraph %d run %d image has no alt text or title",
						p.Index, i))
				}
			}
		}
	}
	for _, tbl := range d.IR.Tables {
		validateTable(&issues, &tbl)
	}

	if len(d.IR.Sections) == 0 {
		issues = append(issues, "document has no sections")
	}

	return issues
}

func validateTable(issues *[]string, tbl *wordir.Table) {
	if tbl == nil {
		return
	}
	for rowIdx, row := range tbl.Rows {
		for cellIdx, cell := range row.Cells {
			if tableCellEmpty(cell) {
				*issues = append(*issues, fmt.Sprintf(
					"table %d row %d cell %d is empty",
					tbl.Index, rowIdx, cellIdx))
			}
			for nestedIdx := range cell.Tables {
				nested := cell.Tables[nestedIdx]
				validateTable(issues, &nested)
			}
		}
	}
}

func tableCellEmpty(cell wordir.TableCell) bool {
	if strings.TrimSpace(cell.Text) != "" {
		return false
	}
	for _, p := range cell.Paragraphs {
		if strings.TrimSpace(p.Text) != "" {
			return false
		}
	}
	return len(cell.Tables) == 0
}
