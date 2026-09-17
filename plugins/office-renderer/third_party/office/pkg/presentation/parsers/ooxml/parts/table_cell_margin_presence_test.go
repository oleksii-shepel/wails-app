package parts

import (
	"encoding/xml"
	"testing"
)

func TestTableCellPropsPreserveAuthoredZeroMarginPresence(t *testing.T) {
	var props PresentationTableCellProps
	if err := xml.Unmarshal([]byte(`<a:tcPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" marL="0" marR="91440"/>`), &props); err != nil {
		t.Fatal(err)
	}
	if !props.MarLSet || props.MarL != 0 {
		t.Fatalf("marL = (%v,%d), want authored zero", props.MarLSet, props.MarL)
	}
	if !props.MarRSet || props.MarR != 91440 {
		t.Fatalf("marR = (%v,%d), want 91440", props.MarRSet, props.MarR)
	}
	if props.MarTSet || props.MarBSet {
		t.Fatalf("omitted vertical margins marked authored: %+v", props)
	}
}
