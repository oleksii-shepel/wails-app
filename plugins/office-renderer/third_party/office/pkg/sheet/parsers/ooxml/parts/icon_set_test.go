package parts_test

import (
	"encoding/xml"
	"testing"

	"office/pkg/sheet/parsers/ooxml/parts"
)

// TestIconSetBindsIconSetAttribute pins the ECMA-376 attribute name: the element
// and its attribute are both called "iconSet" (<iconSet iconSet="3Arrows">).
// Binding the field to "set" silently left every rule on the renderer's default,
// so xlsx icon sets always drew as traffic lights.
func TestIconSetBindsIconSetAttribute(t *testing.T) {
	type rule struct {
		Type    string         `xml:"type,attr"`
		IconSet *parts.IconSet `xml:"iconSet"`
	}
	var got rule
	src := `<cfRule type="iconSet"><iconSet iconSet="3Arrows" showValue="0"><cfvo type="percent" val="0"/></iconSet></cfRule>`
	if err := xml.Unmarshal([]byte(src), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != "iconSet" {
		t.Fatalf("rule type = %q, want iconSet", got.Type)
	}
	if got.IconSet == nil {
		t.Fatal("iconSet element was not parsed")
	}
	if got.IconSet.Set != "3Arrows" {
		t.Fatalf("IconSet.Set = %q, want 3Arrows", got.IconSet.Set)
	}
	if got.IconSet.ShowValue {
		t.Fatal("IconSet.ShowValue = true, want false from showValue=\"0\"")
	}
	if len(got.IconSet.Cfvo) != 1 || got.IconSet.Cfvo[0].Val != "0" {
		t.Fatalf("IconSet.Cfvo = %+v, want a single 0 threshold", got.IconSet.Cfvo)
	}
}
