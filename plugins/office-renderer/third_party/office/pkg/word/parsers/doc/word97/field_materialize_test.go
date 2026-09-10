package word97

import "testing"

func TestMaterializeFieldsNestedResult(t *testing.T) {
	runs := []Run{{Text: "\x13 REF outer \x14A \x13 REF inner \x14B\x15 C\x15", StartCP: 0}}
	got := MaterializeFields(runs, nil)
	if len(got) == 0 {
		t.Fatal("no runs")
	}
	found := false
	for _, r := range got {
		if len(r.Fields) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("nested field tree missing: %#v", got)
	}
}
func TestMaterializeFieldsFormFallback(t *testing.T) {
	got := MaterializeFields([]Run{{Text: "\x13 FORMTEXT \x14value\x15"}}, nil)
	if len(got) != 1 || got[0].Kind != "formField" || got[0].FormField == nil || got[0].FormField.Type != "text" || got[0].FormField.DefaultText != "value" {
		t.Fatalf("form field = %#v", got)
	}
}
func TestMaterializeFieldsSequenceCompatibilityArg(t *testing.T) {
	got := MaterializeFields([]Run{{Text: "\x13 SEQ Figure \\* ARABIC \x141\x15"}}, nil)
	if len(got) != 1 || got[0].FieldKind != "SEQ" || got[0].FieldArg != "Figure" {
		t.Fatalf("SEQ = %#v", got)
	}
}
