package diag_test

import (
	"strings"
	"testing"

	"office/pkg/diag"
)

func TestListAddAndHasErrors(t *testing.T) {
	var list diag.List
	list.Add(diag.Diagnostic{Severity: diag.SeverityWarning, Message: "warn"})
	if list.HasErrors() {
		t.Fatal("warnings should not count as errors")
	}
	list.Add(diag.Diagnostic{Severity: diag.SeverityError, Message: "boom"})
	if !list.HasErrors() {
		t.Fatal("expected error severity to be detected")
	}
}

func TestDiagnosticErrorIncludesCodePartAndCause(t *testing.T) {
	d := diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Code:     "word.unsupported_markup",
		Message:  "unsupported markup",
		Location: diag.Location{Part: "word/document.xml"},
		Cause:    errString("root cause"),
	}
	got := d.Error()
	for _, want := range []string{
		"warning: unsupported markup",
		"(word.unsupported_markup)",
		"[word/document.xml]",
		"root cause",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic error %q missing %q", got, want)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }
