package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDocumentFormFieldsEmpty(t *testing.T) {
	d := NewDocument()
	d.AddParagraph().AddRun().SetText("hello")
	if got := d.FormFields(); len(got) != 0 {
		t.Fatalf("form fields = %d, want 0", len(got))
	}
}

func TestDocumentFormFieldsEnumeratesBodyAndStories(t *testing.T) {
	d := NewDocument()

	bodyRun := d.AddParagraph().AddRun()
	bodyRun.SetText("body")
	bodyRun.ir().Kind = "formField"
	bodyRun.ir().FormField = &wordir.FormField{Type: "text", Name: "BodyField", DefaultText: "Alpha"}

	header := d.AddHeader("default")
	headerRun := header.AddParagraph().AddRun()
	headerRun.SetText("header")
	headerRun.ir().Kind = "formField"
	headerRun.ir().FormField = &wordir.FormField{Type: "checkbox", Name: "HeaderCheck", Checked: true}

	fields := d.FormFields()
	if len(fields) != 2 {
		t.Fatalf("form fields = %d, want 2", len(fields))
	}
	if fields[0].Location != "body" || fields[0].Field == nil || fields[0].Field.Name() != "BodyField" {
		t.Fatalf("body field = %+v", fields[0])
	}
	if fields[1].Location != "header" || fields[1].Field == nil || fields[1].Field.Name() != "HeaderCheck" {
		t.Fatalf("header field = %+v", fields[1])
	}
}

func TestRunFormFieldWrapperAndSetters(t *testing.T) {
	d := NewDocument()
	run := d.AddParagraph().AddRun()
	run.ir().Kind = "formField"
	run.ir().FormField = &wordir.FormField{
		Type:             "dropdown",
		Name:             "Choices",
		Enabled:          true,
		CalcOnExit:       true,
		DefaultText:      "unused",
		MaxLength:        12,
		TextType:         "regular",
		TextFormat:       "fmt",
		Checked:          true,
		CheckboxSize:     20,
		CheckboxSizeAuto: true,
		Items:            []string{"A", "B"},
		DefaultIdx:       1,
	}

	ff := run.FormField()
	if ff == nil {
		t.Fatal("expected run form field")
	}
	if ff.Type() != "dropdown" || ff.Name() != "Choices" {
		t.Fatalf("type/name = %q / %q", ff.Type(), ff.Name())
	}
	ff.SetName("Updated")
	ff.SetEnabled(false)
	ff.SetCalcOnExit(false)
	ff.SetDefaultText("hello")
	ff.SetMaxLength(20)
	ff.SetTextType("number")
	ff.SetTextFormat("0.00")
	ff.SetChecked(false)
	ff.SetCheckboxSize(24)
	ff.SetCheckboxSizeAuto(false)
	ff.SetItems([]string{"X", "Y", "Z"})
	ff.SetDefaultIndex(2)

	if ff.Name() != "Updated" || ff.Enabled() || ff.CalcOnExit() {
		t.Fatalf("updated flags = %+v", ff)
	}
	if ff.DefaultText() != "hello" || ff.MaxLength() != 20 || ff.TextType() != "number" || ff.TextFormat() != "0.00" {
		t.Fatalf("updated text metadata = %#v", ff.ir)
	}
	if ff.Checked() || ff.CheckboxSize() != 24 || ff.CheckboxSizeAuto() {
		t.Fatalf("updated checkbox metadata = %#v", ff.ir)
	}
	if got := ff.Items(); len(got) != 3 || got[2] != "Z" {
		t.Fatalf("items = %#v", got)
	}
	items := ff.Items()
	items[0] = "mutated"
	if ff.Items()[0] != "X" {
		t.Fatal("Items should return a copy")
	}
	if ff.DefaultIndex() != 2 {
		t.Fatalf("default index = %d", ff.DefaultIndex())
	}
}

func TestDocumentFormFieldByName(t *testing.T) {
	d := NewDocument()
	run := d.AddParagraph().AddRun()
	run.ir().Kind = "formField"
	run.ir().FormField = &wordir.FormField{Name: "CustomerID", Type: "text"}

	ff := d.FormFieldByName("CustomerID")
	if ff == nil {
		t.Fatal("expected form field by name")
	}
	if ff.Type() != "text" {
		t.Fatalf("type = %q", ff.Type())
	}
	if d.FormFieldByName("Missing") != nil {
		t.Fatal("expected nil for missing form field")
	}
}

func TestFormFieldNilSafety(t *testing.T) {
	var ff *FormField
	if ff.Type() != "" || ff.Name() != "" || ff.Enabled() || ff.CalcOnExit() || ff.DefaultText() != "" {
		t.Fatal("nil form field should return zero values")
	}
	if ff.MaxLength() != 0 || ff.TextType() != "" || ff.TextFormat() != "" {
		t.Fatal("nil text metadata should return zero values")
	}
	if ff.Checked() || ff.CheckboxSize() != 0 || ff.CheckboxSizeAuto() {
		t.Fatal("nil checkbox metadata should return zero values")
	}
	if ff.Items() != nil || ff.DefaultIndex() != 0 {
		t.Fatal("nil dropdown metadata should return zero values")
	}
	ff.SetName("x")
	ff.SetEnabled(true)
	ff.SetCalcOnExit(true)
	ff.SetDefaultText("x")
	ff.SetMaxLength(1)
	ff.SetTextType("regular")
	ff.SetTextFormat("fmt")
	ff.SetChecked(true)
	ff.SetCheckboxSize(10)
	ff.SetCheckboxSizeAuto(true)
	ff.SetItems([]string{"A"})
	ff.SetDefaultIndex(1)
}