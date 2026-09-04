package edit

import (
	"strings"
	"testing"
	"time"
)

func TestRefreshFieldsDate(t *testing.T) {
	d := NewDocument()
	d.IR.CreatedAt = "2024-03-15T10:30:00Z"
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetField(`DATE \@ "dd/MM/yyyy"`)
	r.ir().FieldKind = "DATE"

	d.RefreshFields()
	if r.Text() == "" {
		t.Fatal("expected date text to be refreshed")
	}
	// Should contain today's date in dd/MM/yyyy format.
	parts := strings.Split(r.Text(), "/")
	if len(parts) != 3 {
		t.Fatalf("expected dd/MM/yyyy, got %q", r.Text())
	}
}

func TestRefreshFieldsDocProperty(t *testing.T) {
	d := NewDocument()
	d.SetTitle("My Title")
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetDocProperty("Title")
	r.ir().FieldKind = "DOCPROPERTY"

	d.RefreshFields()
	if r.Text() != "My Title" {
		t.Fatalf("title = %q, want My Title", r.Text())
	}
}

func TestRefreshFieldsDocVariable(t *testing.T) {
	d := NewDocument()
	d.SetVariable("version", "1.2.3")
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetField("DOCVARIABLE version")
	r.ir().FieldKind = "DOCVARIABLE"
	r.ir().FieldArg = "version"

	d.RefreshFields()
	if r.Text() != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", r.Text())
	}
}

func TestFormatFieldDateCommonPatterns(t *testing.T) {
	ref := time.Date(2024, time.March, 15, 14, 30, 0, 0, time.UTC)
	tests := []struct {
		pic  string
		want string
	}{
		{"dd/MM/yyyy", "15/03/2024"},
		{"yyyy-MM-dd", "2024-03-15"},
		{"dd mmmm yyyy", "15 March 2024"},
		{"dd mmm yyyy", "15 Mar 2024"},
		{"mmmm dd, yyyy", "March 15, 2024"},
		{"hh:mm:ss AM/PM", "02:30:00 PM"},
	}
	for _, tt := range tests {
		got := formatFieldDate(`DATE \@ "`+tt.pic+`"`, ref)
		if got != tt.want {
			t.Errorf("formatFieldDate(%q) = %q, want %q", tt.pic, got, tt.want)
		}
	}
}

func TestParseIRTime(t *testing.T) {
	if got := parseIRTime("2024-03-15T10:30:00Z"); got.IsZero() {
		t.Error("expected non-zero time for RFC3339")
	}
	if got := parseIRTime("2024-03-15T10:30:00"); got.IsZero() {
		t.Error("expected non-zero time for local RFC3339")
	}
	if got := parseIRTime("2024-03-15"); got.IsZero() {
		t.Error("expected non-zero time for date only")
	}
	if got := parseIRTime(""); !got.IsZero() {
		t.Error("expected zero time for empty string")
	}
}