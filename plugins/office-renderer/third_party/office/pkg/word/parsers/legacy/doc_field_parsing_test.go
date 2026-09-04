package legacy

import (
	"strings"
	"testing"

	wordrender "office/pkg/word/render"
)

// TestParseDocFieldInstructionPrefixDisambiguation validates that longer
// prefixes (PAGEREF, NUMPAGES, STYLEREF, CREATEDATE, SAVEDATE, PRINTDATE)
// are not incorrectly matched by their shorter siblings (PAGE, NUM*, DATE, REF).
func TestParseDocFieldInstructionPrefixDisambiguation(t *testing.T) {
	tests := []struct {
		instr    string
		wantKind string
	}{
		{"PAGEREF Section1", "PAGEREF"},
		{"PAGE", "PAGE"},
		{"NUMPAGES", "NUMPAGES"},
		{"NUMCHARS", "NUMCHARS"},
		{"NUMWORDS", "NUMWORDS"},
		{"CREATEDATE", "CREATEDATE"},
		{"SAVEDATE", "SAVEDATE"},
		{"PRINTDATE", "PRINTDATE"},
		{"DATE", "DATE"},
		{"STYLEREF Heading1", "STYLEREF"},
		{"REF BookmarkName", "REF"},
		{"DOCPROPERTY Author", "DOCPROPERTY"},
		{"DOCVARIABLE ProjectID", "DOCVARIABLE"},
		{"TOC", "TOC"},
		{"INDEX", "INDEX"},
		{"XE Entry", "XE"},
		{"TC Entry", "TC"},
		{"IF 1 = 1 \"yes\" \"no\"", "IF"},
		{"TITLE", "TITLE"},
		{"SUBJECT", "SUBJECT"},
	}
	for _, tt := range tests {
		field := parseDocFieldInstruction(tt.instr)
		if field.Kind != tt.wantKind {
			t.Errorf("parse %q: got Kind=%q, want %q", tt.instr, field.Kind, tt.wantKind)
		}
	}
}

// TestParseDocFieldInstructionRef validates REF field parsing (bookmark references).
func TestParseDocFieldInstructionRef(t *testing.T) {
	tests := []struct {
		instr      string
		wantKind   string
		wantAnchor string
	}{
		{"REF BookmarkName", "REF", "BookmarkName"},
		{"REF \"Bookmark Name\"", "REF", "Bookmark Name"},
		{`REF "Cross-Reference"`, "REF", "Cross-Reference"},
		{"REF", "REF", ""},
	}
	for _, tt := range tests {
		field := parseDocFieldInstruction(tt.instr)
		if field.Kind != tt.wantKind {
			t.Errorf("REF parse: got Kind=%q, want %q", field.Kind, tt.wantKind)
		}
		if field.Anchor != tt.wantAnchor {
			t.Errorf("REF parse %q: got Anchor=%q, want %q", tt.instr, field.Anchor, tt.wantAnchor)
		}
	}
}

// TestParseDocFieldInstructionStyleref validates STYLEREF field parsing (style-name references).
func TestParseDocFieldInstructionStyleref(t *testing.T) {
	tests := []struct {
		instr    string
		wantKind string
		wantArg  string
	}{
		{"STYLEREF \"Heading 1\"", "STYLEREF", "Heading 1"},
		{"STYLEREF Heading2", "STYLEREF", "Heading2"},
		{`STYLEREF "Normal"`, "STYLEREF", "Normal"},
		{"STYLEREF", "STYLEREF", ""},
	}
	for _, tt := range tests {
		field := parseDocFieldInstruction(tt.instr)
		if field.Kind != tt.wantKind {
			t.Errorf("STYLEREF parse: got Kind=%q, want %q", field.Kind, tt.wantKind)
		}
		if field.Arg != tt.wantArg {
			t.Errorf("STYLEREF parse %q: got Arg=%q, want %q", tt.instr, field.Arg, tt.wantArg)
		}
	}
}

// TestParseDocFieldInstructionFillin validates FILLIN field parsing (fill-in prompts).
func TestParseDocFieldInstructionFillin(t *testing.T) {
	tests := []struct {
		instr    string
		wantKind string
		wantArg  string // Default value
	}{
		{"FILLIN \"Enter your name\" \"John Doe\"", "FILLIN", "John Doe"},
		{"FILLIN \"Name\" \"\"", "FILLIN", ""},
		{"FILLIN \"Prompt only\"", "FILLIN", ""},
		{"FILLIN", "FILLIN", ""},
	}
	for _, tt := range tests {
		field := parseDocFieldInstruction(tt.instr)
		if field.Kind != tt.wantKind {
			t.Errorf("FILLIN parse: got Kind=%q, want %q", field.Kind, tt.wantKind)
		}
		if field.Arg != tt.wantArg {
			t.Errorf("FILLIN parse %q: got Arg=%q, want %q", tt.instr, field.Arg, tt.wantArg)
		}
	}
}

// TestApplyDocFieldsToRunsPreservesRefFieldKind validates that REF fields are tagged with FieldKind.
func TestApplyDocFieldsToRunsPreservesRefFieldKind(t *testing.T) {
	runs := []docRun{
		{Text: "\x13"},             // FLDBEGIN
		{Text: " REF MyBookmark "}, // Instruction
		{Text: "\x14"},             // FLDSEP
		{Text: "Referenced Text"},  // Cached result
		{Text: "\x15"},             // FLDEND
	}
	result := applyDocFieldsToRuns(runs)
	if len(result) == 0 {
		t.Fatalf("Expected non-empty result; got %d runs", len(result))
	}
	// The cached result run should have FieldKind set
	found := false
	for _, r := range result {
		if r.FieldKind == "REF" && r.Anchor == "MyBookmark" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected REF field with Anchor=MyBookmark; got %v", result)
	}
}

// TestApplyDocFieldsToRunsPreservesStylerefFieldKind validates that STYLEREF fields are tagged.
func TestApplyDocFieldsToRunsPreservesStylerefFieldKind(t *testing.T) {
	runs := []docRun{
		{Text: "\x13"},                     // FLDBEGIN
		{Text: " STYLEREF \"Heading 1\" "}, // Instruction
		{Text: "\x14"},                     // FLDSEP
		{Text: "First Heading"},            // Cached result
		{Text: "\x15"},                     // FLDEND
	}
	result := applyDocFieldsToRuns(runs)
	if len(result) == 0 {
		t.Fatalf("Expected non-empty result; got %d runs", len(result))
	}
	// The cached result run should have FieldKind and Arg set
	found := false
	for _, r := range result {
		if r.FieldKind == "STYLEREF" && r.FieldArg == "Heading 1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected STYLEREF field with FieldArg=Heading 1; got %v", result)
	}
}

func TestApplyDocFieldsToRunsPreservesRawFieldInstruction(t *testing.T) {
	runs := []docRun{
		{Text: "\x13"},
		{Text: " HYPERLINK \"https://example.com/report\" "},
		{Text: "\x14"},
		{Text: "Report"},
		{Text: "\x15"},
	}

	result := applyDocFieldsToRuns(runs)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if got := result[0].FieldInstr; got != `HYPERLINK "https://example.com/report"` {
		t.Fatalf("FieldInstr = %q, want raw instruction", got)
	}
	if got := result[0].Target; got != "https://example.com/report" {
		t.Fatalf("Target = %q, want hyperlink URL", got)
	}
}

func TestBuildLegacyDocFromRunsPreservesFieldInstructionInIR(t *testing.T) {
	doc := buildLegacyDocFromRuns("field.doc", applyDocFieldsToRuns([]docRun{
		{Text: "\x13"},
		{Text: " REF SectionBookmark \\\\h "},
		{Text: "\x14"},
		{Text: "Section 1"},
		{Text: "\x15"},
	}))

	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	if len(doc.Paragraphs[0].Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(doc.Paragraphs[0].Runs))
	}
	run := doc.Paragraphs[0].Runs[0]
	if got := run.FieldKind; got != "REF" {
		t.Fatalf("FieldKind = %q, want REF", got)
	}
	if got := run.FieldInstr; got != `REF SectionBookmark \\h` {
		t.Fatalf("FieldInstr = %q, want raw REF instruction", got)
	}
	if got := run.Anchor; got != "SectionBookmark" {
		t.Fatalf("Anchor = %q, want SectionBookmark", got)
	}
}

// TestApplyDocFieldsToRunsPreservesFillinFieldKind validates that FILLIN fields are tagged with default value.
func TestApplyDocFieldsToRunsPreservesFillinFieldKind(t *testing.T) {
	runs := []docRun{
		{Text: "\x13"}, // FLDBEGIN
		{Text: " FILLIN \"Name?\" \"Jane Smith\" "}, // Instruction
		{Text: "\x14"},       // FLDSEP
		{Text: "Jane Smith"}, // Cached result
		{Text: "\x15"},       // FLDEND
	}
	result := applyDocFieldsToRuns(runs)
	if len(result) == 0 {
		t.Fatalf("Expected non-empty result; got %d runs", len(result))
	}
	// The cached result run should have FieldKind and Arg set
	found := false
	for _, r := range result {
		if r.FieldKind == "FILLIN" && r.FieldArg == "Jane Smith" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected FILLIN field with FieldArg=Jane Smith; got %v", result)
	}
}

func TestApplyDocFieldsToRunsPromotesLegacyTextFormField(t *testing.T) {
	runs := []docRun{
		{Text: "\x13"},
		{Text: " FORMTEXT "},
		{Text: "\x14"},
		{Text: "Alice"},
		{Text: "\x15"},
	}

	result := applyDocFieldsToRuns(runs)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if got := result[0].Kind; got != "formField" {
		t.Fatalf("Kind = %q, want formField", got)
	}
	if result[0].FormField == nil {
		t.Fatal("FormField = nil, want populated form field")
	}
	if got := result[0].FormField.Type; got != "text" {
		t.Fatalf("FormField.Type = %q, want text", got)
	}
	if got := result[0].FormField.DefaultText; got != "Alice" {
		t.Fatalf("FormField.DefaultText = %q, want Alice", got)
	}
	if got := result[0].FieldInstr; got != "FORMTEXT" {
		t.Fatalf("FieldInstr = %q, want FORMTEXT", got)
	}
}

func TestBuildLegacyDocFromRunsRendersLegacyDropdownFormField(t *testing.T) {
	doc := buildLegacyDocFromRuns("field.doc", applyDocFieldsToRuns([]docRun{
		{Text: "\x13"},
		{Text: " FORMDROPDOWN "},
		{Text: "\x14"},
		{Text: "Approved"},
		{Text: "\x15"},
	}))

	if len(doc.Paragraphs) != 1 || len(doc.Paragraphs[0].Runs) != 1 {
		t.Fatalf("paragraph/runs = %#v, want single form field run", doc.Paragraphs)
	}
	run := doc.Paragraphs[0].Runs[0]
	if run.Kind != "formField" || run.FormField == nil {
		t.Fatalf("run = %#v, want formField", run)
	}
	if got := run.FormField.Type; got != "dropdown" {
		t.Fatalf("FormField.Type = %q, want dropdown", got)
	}
	if len(run.FormField.Items) != 1 || run.FormField.Items[0] != "Approved" {
		t.Fatalf("FormField.Items = %#v, want [Approved]", run.FormField.Items)
	}

	html, err := wordrender.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `<select class="word-form-field" disabled>`) {
		t.Fatalf("expected dropdown form field in HTML:\n%s", html)
	}
	if !strings.Contains(html, `<option selected>Approved</option>`) {
		t.Fatalf("expected selected dropdown option in HTML:\n%s", html)
	}
}

// TestApplyDocFieldsToRunsNestedFieldInInstruction validates that a nested
// field inside a parent field's instruction is substituted into the parent's
// instruction text and does not get overwritten by the outer field.
func TestApplyDocFieldsToRunsNestedFieldInInstruction(t *testing.T) {
	runs := []docRun{
		{Text: "\x13"},                 // FLDBEGIN outer
		{Text: " IF "},                 // outer instruction part 1
		{Text: "\x13"},                 // FLDBEGIN inner
		{Text: " PAGE "},               // inner instruction
		{Text: "\x14"},                 // FLDSEP inner
		{Text: "1"},                    // inner result
		{Text: "\x15"},                 // FLDEND inner
		{Text: " > 1 \"yes\" \"no\" "}, // outer instruction part 2
		{Text: "\x14"},                 // FLDSEP outer
		{Text: "yes"},                  // outer result
		{Text: "\x15"},                 // FLDEND outer
	}
	result := applyDocFieldsToRuns(runs)
	if len(result) == 0 {
		t.Fatalf("Expected non-empty result; got %d runs", len(result))
	}
	// The outer result run should carry the outer field metadata.
	if result[0].FieldKind != "IF" {
		t.Errorf("Expected outer FieldKind=IF; got %q", result[0].FieldKind)
	}
	// Build the IR and verify the instruction was substituted correctly.
	doc := buildLegacyDocFromRuns("nested.doc", result)
	if len(doc.Paragraphs) != 1 || len(doc.Paragraphs[0].Runs) != 1 {
		t.Fatalf("Expected 1 paragraph with 1 run; got %d paragraphs", len(doc.Paragraphs))
	}
	irRun := doc.Paragraphs[0].Runs[0]
	if irRun.FieldKind != "IF" {
		t.Errorf("Expected IR run FieldKind=IF; got %q", irRun.FieldKind)
	}
	// The inner PAGE result ("1") should have been substituted into the instruction.
	if !strings.Contains(irRun.FieldInstr, "1") {
		t.Errorf("Expected FieldInstr to contain substituted '1'; got %q", irRun.FieldInstr)
	}
}

// TestApplyDocFieldsToRunsNestedFieldInResult validates that a nested field
// inside a parent field's result is captured with correct flat metadata on
// each run and does not get overwritten by the outer field.
func TestApplyDocFieldsToRunsNestedFieldInResult(t *testing.T) {
	runs := []docRun{
		{Text: "\x13"},    // FLDBEGIN outer
		{Text: " REF B "}, // outer instruction
		{Text: "\x14"},    // FLDSEP outer
		{Text: "prefix "}, // outer result part 1
		{Text: "\x13"},    // FLDBEGIN inner
		{Text: " PAGE "},  // inner instruction
		{Text: "\x14"},    // FLDSEP inner
		{Text: "2"},       // inner result
		{Text: "\x15"},    // FLDEND inner
		{Text: " suffix"}, // outer result part 2
		{Text: "\x15"},    // FLDEND outer
	}
	result := applyDocFieldsToRuns(runs)
	if len(result) != 3 {
		t.Fatalf("Expected 3 result runs (prefix, PAGE, suffix); got %d", len(result))
	}
	if result[0].FieldKind != "REF" {
		t.Errorf("Expected first run FieldKind=REF; got %q", result[0].FieldKind)
	}
	if result[1].FieldKind != "PAGE" {
		t.Errorf("Expected middle run FieldKind=PAGE; got %q", result[1].FieldKind)
	}
	if result[1].Text != "2" {
		t.Errorf("Expected middle run Text=2; got %q", result[1].Text)
	}
	// In the flat model only the first result run of a field gets the
	// parent's metadata, matching the legacy behaviour.
	if result[2].FieldKind != "" {
		t.Errorf("Expected last run FieldKind=\"\" (flat model); got %q", result[2].FieldKind)
	}
}