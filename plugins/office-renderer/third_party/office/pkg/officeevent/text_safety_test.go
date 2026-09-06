package officeevent

import (
	"testing"

	presax "office/pkg/presentation/parsers/ooxml/sax"
	sheetsax "office/pkg/sheet/parsers/ooxml/sax"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestWordHandlerSanitizesEnvelopeTextButPreservesNativeEvent(t *testing.T) {
	native := wordsax.Event{Text: "a\x01b\tc"}
	var got Event
	err := (wordHandler{handler: HandlerFunc(func(ev Event) error { got = ev; return nil })}).HandleEvent(native)
	if err != nil { t.Fatal(err) }
	if got.Text != "ab\tc" { t.Fatalf("Text = %q, want %q", got.Text, "ab\tc") }
	if got.Word == nil || got.Word.Text != native.Text { t.Fatalf("native Word event mutated: %#v", got.Word) }
}

func TestSpreadsheetHandlerSanitizesEnvelopeTextButPreservesNativeEvent(t *testing.T) {
	native := sheetsax.Event{Text: "a\x02b\nc"}
	var got Event
	err := (spreadsheetHandler{handler: HandlerFunc(func(ev Event) error { got = ev; return nil })}).HandleEvent(native)
	if err != nil { t.Fatal(err) }
	if got.Text != "ab\nc" { t.Fatalf("Text = %q, want %q", got.Text, "ab\nc") }
	if got.Spreadsheet == nil || got.Spreadsheet.Text != native.Text { t.Fatalf("native Spreadsheet event mutated: %#v", got.Spreadsheet) }
}

func TestPresentationHandlerSanitizesEnvelopeTextButPreservesNativeEvent(t *testing.T) {
	native := presax.Event{Text: "a\x03b\rc"}
	var got Event
	err := (presentationHandler{handler: HandlerFunc(func(ev Event) error { got = ev; return nil })}).HandleEvent(native)
	if err != nil { t.Fatal(err) }
	if got.Text != "ab\rc" { t.Fatalf("Text = %q, want %q", got.Text, "ab\rc") }
	if got.Presentation == nil || got.Presentation.Text != native.Text { t.Fatalf("native Presentation event mutated: %#v", got.Presentation) }
}
