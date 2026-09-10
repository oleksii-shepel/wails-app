package word97

import (
	"os"
	"strings"
	"testing"
)

func TestWord97FieldGrammarStaysShared(t *testing.T) {
	data, err := os.ReadFile("field_parse.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "docfield.ParseInstruction(instr)") {
		t.Fatal("Word97 field parser must consume the shared DOC field grammar")
	}
	for _, forbidden := range []string{
		`strings.HasPrefix(upper, "HYPERLINK")`,
		`strings.HasPrefix(upper, "REF")`,
		`strings.HasPrefix(upper, "FORMTEXT")`,
		`strings.HasPrefix(upper, "EMBED")`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("Word97 field parser duplicated shared grammar %q", forbidden)
		}
	}
}
