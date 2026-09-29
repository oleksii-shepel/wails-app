package render

import (
	"math"
	"strings"
	"testing"

	"office/pkg/diag"
	dml "office/pkg/ooxml/drawing"
	resolve "office/pkg/word/ir"
	"office/pkg/word/layout"
)

func TestCountDocWords(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "Hello world"}},
			}},
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{
							{Kind: "paragraph", Paragraph: &resolve.Paragraph{
								Runs: []resolve.Run{{Text: "foo bar baz"}},
							}},
						},
					}},
				}},
			}},
		},
	}
	r := &htmlRenderer{doc: doc}
	if got := r.countDocWords(); got != 5 {
		t.Errorf("countDocWords() = %d, want 5", got)
	}
}

func TestCountDocChars(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{{Text: "a b c"}},
			}},
		},
	}
	r := &htmlRenderer{doc: doc}
	if got := r.countDocChars(); got != 3 {
		t.Errorf("countDocChars() = %d, want 3", got)
	}
}

func TestEvaluateIfField(t *testing.T) {
	doc := &resolve.Document{Title: "Report", Author: "Alice", Subject: "Finance"}
	r := &htmlRenderer{doc: doc}
	tests := []struct {
		instr string
		want  string
	}{
		{`IF 1 = 1 "yes" "no"`, "yes"},
		{`IF 1 = 2 "yes" "no"`, "no"},
		{`IF "a" = "a" "yes" "no"`, "yes"},
		{`COMPARE 5 > 3 "greater" "less"`, "greater"},
		{`COMPARE 3 > 5 "greater" "less"`, "less"},
		{`IF TITLE = "Report" "yes" "no"`, "yes"},
		{`IF AUTHOR = "Bob" "yes" "no"`, "no"},
		{"", ""},
		{"UNKNOWN 1 2 3", ""},
	}
	for _, tt := range tests {
		got := r.evaluateIfField(tt.instr)
		if got != tt.want {
			t.Errorf("evaluateIfField(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestResolveIfOperand(t *testing.T) {
	doc := &resolve.Document{Title: "Report", Author: "Alice", Subject: "Finance"}
	r := &htmlRenderer{doc: doc}
	if got := r.resolveIfOperand("TITLE"); got != "Report" {
		t.Errorf("resolveIfOperand(TITLE) = %q, want Report", got)
	}
	if got := r.resolveIfOperand("AUTHOR"); got != "Alice" {
		t.Errorf("resolveIfOperand(AUTHOR) = %q, want Alice", got)
	}
	if got := r.resolveIfOperand("SUBJECT"); got != "Finance" {
		t.Errorf("resolveIfOperand(SUBJECT) = %q, want Finance", got)
	}
	if got := r.resolveIfOperand("42"); got != "42" {
		t.Errorf("resolveIfOperand(42) = %q, want 42", got)
	}
}

func TestCompareIfOperands(t *testing.T) {
	tests := []struct {
		left, right, op string
		want            bool
	}{
		{"1", "1", "=", true},
		{"1", "2", "=", false},
		{"2", "1", ">", true},
		{"1", "2", "<", true},
		{"1", "1", ">=", true},
		{"1", "1", "<=", true},
		{"1", "2", ">=", false},
		{"2", "1", "<=", false},
		{"1", "2", "<>", true},
		{"1", "1", "<>", false},
		{"a", "a", "=", true},
		{"a", "b", "=", false},
	}
	for _, tt := range tests {
		got := compareIfOperands(tt.left, tt.right, tt.op)
		if got != tt.want {
			t.Errorf("compareIfOperands(%q, %q, %q) = %v, want %v", tt.left, tt.right, tt.op, got, tt.want)
		}
	}
}

func TestResolveStyleRef(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleName: "Heading 1",
				Runs:      []resolve.Run{{Text: "Introduction"}},
			}},
			{Kind: "table", Table: &resolve.Table{
				Rows: []resolve.TableRow{{
					Cells: []resolve.TableCell{{
						Blocks: []resolve.Block{
							{Kind: "paragraph", Paragraph: &resolve.Paragraph{
								StyleName: "Heading 2",
								Runs:      []resolve.Run{{Text: "Background"}},
							}},
						},
					}},
				}},
			}},
		},
	}
	r := &htmlRenderer{doc: doc}
	if got := r.resolveStyleRef("Heading 1"); got != "Introduction" {
		t.Errorf("resolveStyleRef(Heading 1) = %q, want Introduction", got)
	}
	if got := r.resolveStyleRef("heading 1"); got != "Introduction" {
		t.Errorf("resolveStyleRef(heading 1) = %q, want Introduction", got)
	}
	if got := r.resolveStyleRef("Heading 2"); got != "Background" {
		t.Errorf("resolveStyleRef(Heading 2) = %q, want Background", got)
	}
	if got := r.resolveStyleRef("Missing"); got != "" {
		t.Errorf("resolveStyleRef(Missing) = %q, want empty", got)
	}
	if got := r.resolveStyleRef(""); got != "" {
		t.Errorf("resolveStyleRef(empty) = %q, want empty", got)
	}
}

func TestParseEmbedClassName(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{`EMBED Excel.Sheet.12 \s`, "Excel.Sheet.12"},
		{`EMBED Excel.Sheet.12`, "Excel.Sheet.12"},
		{`EMBED`, ""},
		{`EMBED \s`, ""},
	}
	for _, tt := range tests {
		got := parseEmbedClassName(tt.instr)
		if got != tt.want {
			t.Errorf("parseEmbedClassName(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestParseLinkURL(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{`LINK Excel.Sheet.12 "C:\data.xlsx" "Sheet1!R1C1"`, `C:\data.xlsx`},
		{`LINK Excel.Sheet.12 "C:\data.xlsx"`, `C:\data.xlsx`},
		{`LINK "C:\data.xlsx"`, ""},
		{`LINK`, ""},
	}
	for _, tt := range tests {
		got := parseLinkURL(tt.instr)
		if got != tt.want {
			t.Errorf("parseLinkURL(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestExtractFieldString(t *testing.T) {
	tests := []struct {
		s        string
		wantVal  string
		wantRest string
	}{
		{`"hello" world`, "hello", "world"},
		{`hello world`, "", "hello world"},
		{`"hello`, "hello", ""},
		{`"" rest`, "", "rest"},
	}
	for _, tt := range tests {
		val, rest := extractFieldString(tt.s)
		if val != tt.wantVal || rest != tt.wantRest {
			t.Errorf("extractFieldString(%q) = (%q, %q), want (%q, %q)", tt.s, val, rest, tt.wantVal, tt.wantRest)
		}
	}
}

func TestEvaluateWordFormulaFull(t *testing.T) {
	if got := evaluateWordFormulaFull("= 2+2", ""); got != "4" {
		t.Errorf("evaluateWordFormulaFull(= 2+2, empty) = %q, want 4", got)
	}
	if got := evaluateWordFormulaFull("= 2+2", "5"); got != "" {
		t.Errorf("evaluateWordFormulaFull(= 2+2, 5) = %q, want empty", got)
	}
}

func TestHTMLRendererCalculateFieldValue(t *testing.T) {
	doc := &resolve.Document{
		Title:            "Report",
		Author:           "Alice Smith",
		UserName:         "Alice Smith",
		Subject:          "Finance",
		Filename:         "doc.docx",
		Template:         "template.dotx",
		RevNum:           "3",
		TotalEditMinutes: 42,
		DocumentVariables: map[string]string{
			"ProjectID": "42",
		},
		CustomProperties: map[string]string{
			"UserAddress": "123 Main St",
		},
	}
	r := &htmlRenderer{doc: doc, pageIndex: 2, pages: make([]layout.Page, 5), seqCounters: make(map[string]int)}
	r.bookmarkPages = map[string]int{"bm": 3}
	r.bookmarkText = map[string]string{"AskBookmark": "Stored answer", "MarginBookmark": "Section 2.3"}
	r.bookmarkNoteRefs = map[string]noteFieldTarget{"noteMark": {kind: "footnote", noteID: 9}}
	r.footnoteNumbers = map[int]string{9: "iv"}
	r.options.MergeData = map[string]string{"City": "NYC"}

	tests := []struct {
		run  resolve.Run
		want string
	}{
		{resolve.Run{FieldKind: "PAGE"}, "3"},
		{resolve.Run{FieldKind: "NUMPAGES"}, "5"},
		{resolve.Run{FieldKind: "TITLE"}, "Report"},
		{resolve.Run{FieldKind: "AUTHOR", Text: ""}, "Alice Smith"},
		{resolve.Run{FieldKind: "SUBJECT"}, "Finance"},
		{resolve.Run{FieldKind: "FILENAME", Text: ""}, "doc.docx"},
		{resolve.Run{FieldKind: "NUMWORDS", Text: ""}, "0"},
		{resolve.Run{FieldKind: "NUMCHARS", Text: ""}, "0"},
		{resolve.Run{FieldKind: "PAGEREF", Anchor: "bm", Text: ""}, "3"},
		{resolve.Run{FieldKind: "NOTEREF", FieldInstr: `NOTEREF noteMark`, Text: ""}, "iv"},
		{resolve.Run{FieldKind: "ASK", FieldArg: "AskBookmark", FieldInstr: `ASK AskBookmark "Prompt"`, Text: ""}, "Stored answer"},
		{resolve.Run{FieldKind: "AUTOTEXT", FieldArg: "ClauseName", FieldInstr: `AUTOTEXT ClauseName`, Text: ""}, "[ClauseName]"},
		{resolve.Run{FieldKind: "DOCPROPERTY", FieldArg: "Title"}, "Report"},
		{resolve.Run{FieldKind: "DOCPROPERTY", FieldArg: "Author"}, "Alice Smith"},
		{resolve.Run{FieldKind: "DOCPROPERTY", FieldArg: "Keywords"}, ""},
		{resolve.Run{FieldKind: "DOCVARIABLE", FieldArg: "ProjectID"}, "42"},
		{resolve.Run{FieldKind: "DOCVARIABLE", FieldArg: "MissingVar"}, ""},
		{resolve.Run{FieldKind: "STYLEREF", FieldArg: "Heading 1"}, ""},
		{resolve.Run{FieldKind: "REF", Anchor: "MarginBookmark", FieldInstr: `REF MarginBookmark \v`}, "2.3"},
		{resolve.Run{FieldKind: "IF", FieldArg: `IF 1 = 1 "yes" "no"`}, "yes"},
		{resolve.Run{
			FieldKind: "IF",
			FieldArg:  `IF 1 > 2 "yes" "no"`,
			FieldInstrRuns: []resolve.Run{
				{Text: " IF "},
				{Text: "1", Fields: []resolve.Field{{
					Kind:     "PAGE",
					Instr:    "PAGE",
					Children: []resolve.Run{{Text: "1"}},
				}}},
				{Text: ` > 2 "yes" "no"`},
			},
		}, "yes"},
		{resolve.Run{FieldKind: "COMPARE", FieldArg: `COMPARE 1 = 2 "yes" "no"`}, "no"},
		{resolve.Run{FieldKind: "QUOTE", FieldArg: `QUOTE "Page " 3`}, "Page 3"},
		{resolve.Run{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \r 7`}, "7"},
		{resolve.Run{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \c`}, "7"},
		{resolve.Run{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \* ROMAN`}, "VIII"},
		{resolve.Run{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \h`}, ""},
		{resolve.Run{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \c \* ALPHABETIC`}, "I"},
		{resolve.Run{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \c \* CardText`}, "nine"},
		{resolve.Run{FieldKind: "SEQ", FieldArg: "Figure", FieldInstr: `SEQ Figure \* OrdText`}, "tenth"},
		{resolve.Run{FieldKind: "SEQ", FieldArg: "Figure"}, "11"},
		{resolve.Run{FieldKind: "EDITTIME", Text: ""}, "42"},
		{resolve.Run{FieldKind: "USERADDRESS", Text: ""}, "123 Main St"},
		{resolve.Run{FieldKind: "USERINITIALS", Text: ""}, "AS"},
		{resolve.Run{FieldKind: "USERNAME", Text: ""}, "Alice Smith"},
		{resolve.Run{FieldKind: "AUTONUM", Text: ""}, "1"},
		{resolve.Run{FieldKind: "AUTONUMLGL", Text: ""}, "2."},
		{resolve.Run{FieldKind: "AUTONUMOUT", Text: ""}, "III."},
		{resolve.Run{FieldKind: "REVNUM", Text: ""}, "3"},
		{resolve.Run{FieldKind: "TEMPLATE", Text: ""}, "template.dotx"},
		{resolve.Run{FieldKind: "MACROBUTTON", FieldInstr: `MACROBUTTON NoMacro Click here`, Text: ""}, "Click here"},
		{resolve.Run{FieldKind: "PRIVATE", Text: "cached private text"}, "cached private text"},
		{resolve.Run{FieldKind: "FORMULA", FieldArg: "= 2+2", Text: ""}, "4"},
		{resolve.Run{FieldKind: "FORMULA", FieldArg: "= 2+2", Text: "5"}, ""},
		{resolve.Run{FieldKind: "MERGEFIELD", FieldArg: "Name", Text: ""}, "«Name»"},
		{resolve.Run{FieldKind: "MERGEFIELD", FieldArg: "Name", Text: "Alice"}, ""},
		{resolve.Run{FieldKind: "MERGEFIELD", FieldArg: "City", FieldInstr: `MERGEFIELD City \b "[" \f "]"`, Text: ""}, "[NYC]"},
		{resolve.Run{FieldKind: "EMBED", FieldArg: `EMBED Excel.Sheet`, Text: ""}, "[Excel.Sheet]"},
		{resolve.Run{FieldKind: "EMBED", FieldArg: `EMBED Excel.Sheet`, Text: "obj"}, ""},
		{resolve.Run{FieldKind: "LINK", FieldArg: `LINK Excel.Sheet "C:\file.xlsx"`, Text: ""}, `C:\file.xlsx`},
		{resolve.Run{FieldKind: "LINK", FieldArg: `LINK Excel.Sheet "C:\file.xlsx"`, Text: "obj"}, ""},
		{resolve.Run{FieldKind: "LISTNUM", FieldArg: "List1", Text: ""}, "[List1]"},
		{resolve.Run{FieldKind: "INCLUDETEXT", FieldArg: `C:\doc.txt`, Text: ""}, "[C:\\doc.txt]"},
		{resolve.Run{FieldKind: "INCLUDEPICTURE", FieldArg: `C:\img.png`, Text: ""}, "[C:\\img.png]"},
		{resolve.Run{FieldKind: "EQ", FieldArg: `\f(,)`, Text: ""}, ""},
		{resolve.Run{FieldKind: "SYMBOL", FieldArg: "65", Text: ""}, "A"},
		{resolve.Run{FieldKind: "CITATION", FieldArg: "Smith2020", Text: ""}, "[Smith2020]"},
		{resolve.Run{FieldKind: "TOA", FieldInstr: `TOA \h`, Text: ""}, "[Table of Authorities]"},
		{resolve.Run{FieldKind: "GOTOBUTTON", FieldArg: `GOTOBUTTON bm "Click"`, Text: ""}, `<a href="#bm" class="word-field word-field-gotobutton">Click</a>`},
		{resolve.Run{FieldKind: "BARCODE", FieldArg: "12345", Text: ""}, "[Barcode: 12345]"},
		{resolve.Run{FieldKind: "RD", FieldInstr: `RD "chapter1.docx"`, Text: ""}, ""},
		{resolve.Run{FieldKind: "IMPORT", FieldArg: "C:\\file.txt", Text: ""}, "[Import: C:\\file.txt]"},
		{resolve.Run{FieldKind: "SAVEPICTURE", FieldArg: "image.png", Text: ""}, "[Save Picture: image.png]"},
		{resolve.Run{FieldKind: "INFO", FieldArg: "Title", Text: ""}, "Report"},
		{resolve.Run{FieldKind: "INFO", FieldArg: "Author", Text: ""}, "Alice Smith"},
		{resolve.Run{FieldKind: "UNKNOWN"}, ""},
	}
	for _, tt := range tests {
		got := r.calculateFieldValue(tt.run)
		if got != tt.want {
			t.Errorf("calculateFieldValue(%+v) = %q, want %q", tt.run, got, tt.want)
		}
	}
}

func TestParseSeqControlSectionRestart(t *testing.T) {
	ctrl := parseSeqControl(`SEQ Fig \s`)
	if !ctrl.restartSection {
		t.Errorf("expected restartSection for \\s without number")
	}
	if ctrl.hasRestartHeading {
		t.Errorf("expected no heading restart for \\s without number")
	}

	ctrl = parseSeqControl(`SEQ Fig \s 2`)
	if ctrl.restartSection {
		t.Errorf("expected no section restart for \\s 2")
	}
	if !ctrl.hasRestartHeading || ctrl.restartHeading != 2 {
		t.Errorf("expected heading restart level 2, got %+v", ctrl)
	}

	ctrl = parseSeqControl(`SEQ Fig \s \* ROMAN`)
	if !ctrl.restartSection {
		t.Errorf("expected restartSection for \\s \\* ROMAN")
	}
	if ctrl.format != "ROMAN" {
		t.Errorf("expected format ROMAN after \\s, got %q", ctrl.format)
	}
}

func TestHTMLRendererSeqSectionRestart(t *testing.T) {
	doc := &resolve.Document{}
	r := &htmlRenderer{doc: doc, seqCounters: make(map[string]int)}

	// Without section restart, sequence increments globally.
	run1 := resolve.Run{FieldKind: "SEQ", FieldArg: "Fig", FieldInstr: `SEQ Fig`}
	run2 := resolve.Run{FieldKind: "SEQ", FieldArg: "Fig", FieldInstr: `SEQ Fig`}
	if got := r.calculateFieldValue(run1); got != "1" {
		t.Errorf("first SEQ = %q, want 1", got)
	}
	if got := r.calculateFieldValue(run2); got != "2" {
		t.Errorf("second SEQ = %q, want 2", got)
	}

	// With \s (section restart), a new section resets the counter.
	r.sectionIndex = 1
	run3 := resolve.Run{FieldKind: "SEQ", FieldArg: "Fig", FieldInstr: `SEQ Fig \s`}
	if got := r.calculateFieldValue(run3); got != "1" {
		t.Errorf("third SEQ in new section = %q, want 1", got)
	}

	// Continuing in the same section increments the section-local counter.
	run4 := resolve.Run{FieldKind: "SEQ", FieldArg: "Fig", FieldInstr: `SEQ Fig \s`}
	if got := r.calculateFieldValue(run4); got != "2" {
		t.Errorf("fourth SEQ in same section = %q, want 2", got)
	}

	// Original section counter is unchanged.
	r.sectionIndex = 0
	run5 := resolve.Run{FieldKind: "SEQ", FieldArg: "Fig", FieldInstr: `SEQ Fig`}
	if got := r.calculateFieldValue(run5); got != "3" {
		t.Errorf("fifth SEQ in original section = %q, want 3", got)
	}
}

func TestHTMLRendererResolveDocProperty(t *testing.T) {
	doc := &resolve.Document{
		Title:          "T",
		Author:         "A",
		Subject:        "S",
		Description:    "D",
		Keywords:       "K",
		LastModifiedBy: "L",
		DocumentVariables: map[string]string{
			"ProjectID": "42",
		},
	}
	r := &htmlRenderer{doc: doc}
	if got := r.resolveDocProperty("Title"); got != "T" {
		t.Errorf("resolveDocProperty(Title) = %q, want T", got)
	}
	if got := r.resolveDocProperty("DESCRIPTION"); got != "D" {
		t.Errorf("resolveDocProperty(DESCRIPTION) = %q, want D", got)
	}
	if got := r.resolveDocProperty("COMMENTS"); got != "D" {
		t.Errorf("resolveDocProperty(COMMENTS) = %q, want D", got)
	}
	if got := r.resolveDocProperty("Unknown"); got != "" {
		t.Errorf("resolveDocProperty(Unknown) = %q, want empty", got)
	}
	if got := r.resolveDocVariable("ProjectID"); got != "42" {
		t.Errorf("resolveDocVariable(ProjectID) = %q, want 42", got)
	}
	if got := r.resolveDocVariable(" projectid "); got != "42" {
		t.Errorf("resolveDocVariable(projectid) = %q, want 42", got)
	}
	if got := r.resolveDocVariable("Unknown"); got != "" {
		t.Errorf("resolveDocVariable(Unknown) = %q, want empty", got)
	}
}

func TestRenderRuby(t *testing.T) {
	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderRuby(resolve.Run{Ruby: &resolve.RubyContent{Base: "base", Text: "text"}})
	if got := r.buf.String(); got != "<ruby>base<rt>text</rt></ruby>" {
		t.Errorf("renderRuby = %q, want %q", got, "<ruby>base<rt>text</rt></ruby>")
	}
	r.buf.Reset()
	r.renderRuby(resolve.Run{})
	if r.buf.String() != "" {
		t.Error("expected empty buffer for nil ruby")
	}
}

func TestRenderFormField(t *testing.T) {
	r := &htmlRenderer{}
	r.renderFormField(resolve.Run{FormField: &resolve.FormField{Type: "text", Name: "f1", DefaultText: "hello", MaxLength: 10}})
	got := r.buf.String()
	if !strings.Contains(got, `type="text"`) || !strings.Contains(got, `value="hello"`) || !strings.Contains(got, `maxlength="10"`) || !strings.Contains(got, `name="f1"`) {
		t.Errorf("renderFormField text = %q", got)
	}
	r.buf.Reset()
	r.renderFormField(resolve.Run{FormField: &resolve.FormField{Type: "checkbox", Name: "c1", Checked: true}})
	got = r.buf.String()
	if !strings.Contains(got, `type="checkbox"`) || !strings.Contains(got, `checked`) || !strings.Contains(got, `name="c1"`) {
		t.Errorf("renderFormField checkbox = %q", got)
	}
	r.buf.Reset()
	r.renderFormField(resolve.Run{FormField: &resolve.FormField{Type: "dropdown", Name: "d1", Items: []string{"a", "b"}, DefaultIdx: 1}})
	got = r.buf.String()
	if !strings.Contains(got, `<select`) || !strings.Contains(got, `<option selected>b</option>`) || !strings.Contains(got, `<option>a</option>`) {
		t.Errorf("renderFormField dropdown = %q", got)
	}
	r.buf.Reset()
	r.renderFormField(resolve.Run{})
	if r.buf.String() != "" {
		t.Error("expected empty buffer for nil form field")
	}
}

// TestRenderNestedFields verifies that nested fields inside run.Fields are
// evaluated dynamically (e.g. PAGE, IF) and that fields with no calculated
// value fall back to their cached children.
func TestRenderNestedFields(t *testing.T) {
	doc := &resolve.Document{Title: "Report"}
	r := &htmlRenderer{doc: doc, pageIndex: 2}

	// Nested PAGE field should render the current page number.
	r.buf.Reset()
	r.renderRun(resolve.Run{
		Text: "", FieldKind: "",
		Fields: []resolve.Field{{
			Kind:     "PAGE",
			Instr:    "PAGE",
			Children: []resolve.Run{{Text: "1", Format: resolve.RunFormat{Bold: &[]bool{true}[0]}}},
		}},
	})
	got := r.buf.String()
	if !strings.Contains(got, "word-field-page") {
		t.Errorf("nested PAGE: expected word-field-page class, got %q", got)
	}
	if !strings.Contains(got, ">3<") {
		t.Errorf("nested PAGE: expected page number 3, got %q", got)
	}

	// Nested IF field should evaluate the condition.
	r.buf.Reset()
	r.renderRun(resolve.Run{
		Text: "", FieldKind: "",
		Fields: []resolve.Field{{
			Kind:     "IF",
			Instr:    `IF 1 = 1 "yes" "no"`,
			Arg:      `IF 1 = 1 "yes" "no"`,
			Children: []resolve.Run{{Text: "cached"}},
		}},
	})
	got = r.buf.String()
	if !strings.Contains(got, "word-field-if") {
		t.Errorf("nested IF: expected word-field-if class, got %q", got)
	}
	if !strings.Contains(got, "yes") {
		t.Errorf("nested IF: expected evaluated result 'yes', got %q", got)
	}

	// Nested IF field with structured instruction runs should evaluate nested
	// operands dynamically rather than trusting stale flattened FieldArg text.
	r.buf.Reset()
	r.renderRun(resolve.Run{
		Text: "",
		Fields: []resolve.Field{{
			Kind:  "IF",
			Instr: `IF 1 > 2 "yes" "no"`,
			Arg:   `IF 1 > 2 "yes" "no"`,
			InstrRuns: []resolve.Run{
				{Text: " IF "},
				{Text: "1", Fields: []resolve.Field{{
					Kind:     "PAGE",
					Instr:    "PAGE",
					Children: []resolve.Run{{Text: "1"}},
				}}},
				{Text: ` > 2 "yes" "no"`},
			},
			Children: []resolve.Run{{Text: "cached"}},
		}},
	})
	got = r.buf.String()
	if !strings.Contains(got, "yes") {
		t.Errorf("nested IF instruction runs: expected evaluated result 'yes', got %q", got)
	}

	// Nested QUOTE field should use structured instruction runs to splice in
	// nested field values rather than relying on stale flattened text.
	r.buf.Reset()
	r.renderRun(resolve.Run{
		Text: "",
		Fields: []resolve.Field{{
			Kind:  "QUOTE",
			Instr: `QUOTE "Page " 1`,
			Arg:   `QUOTE "Page " 1`,
			InstrRuns: []resolve.Run{
				{Text: ` QUOTE "Page "`},
				{Text: "1", Fields: []resolve.Field{{
					Kind:     "PAGE",
					Instr:    "PAGE",
					Children: []resolve.Run{{Text: "1"}},
				}}},
			},
			Children: []resolve.Run{{Text: "cached"}},
		}},
	})
	got = r.buf.String()
	if !strings.Contains(got, "Page 3") {
		t.Errorf("nested QUOTE instruction runs: expected evaluated result 'Page 3', got %q", got)
	}

	// Nested REF field has no calculated value, so cached children are rendered.
	r.buf.Reset()
	r.renderRun(resolve.Run{
		Text: "", FieldKind: "",
		Fields: []resolve.Field{{
			Kind:     "REF",
			Instr:    "REF Bookmark",
			Anchor:   "Bookmark",
			Children: []resolve.Run{{Text: "see here", Kind: "hyperlink", Target: "#Bookmark"}},
		}},
	})
	got = r.buf.String()
	if strings.Contains(got, "word-field-ref") {
		// REF has no calculated value, so we render children directly.
		// The flat child run already carries hyperlink styling.
	}
	if !strings.Contains(got, "see here") {
		t.Errorf("nested REF: expected cached text 'see here', got %q", got)
	}
}

func TestHTMLDebug(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				StyleID: "Normal",
				Runs:    []resolve.Run{{Text: "hello"}},
			}},
		},
	}
	out, err := HTMLDebug(doc)
	if err != nil {
		t.Fatalf("HTMLDebug: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello' in debug output")
	}
}

func TestParseCSSHexRGB(t *testing.T) {
	r, g, b, ok := parseCSSHexRGB("FF0000")
	if !ok || r != 255 || g != 0 || b != 0 {
		t.Errorf("parseCSSHexRGB(FF0000) = (%d, %d, %d, %v), want (255, 0, 0, true)", r, g, b, ok)
	}
	r, g, b, ok = parseCSSHexRGB("00FF00")
	if !ok || r != 0 || g != 255 || b != 0 {
		t.Errorf("parseCSSHexRGB(00FF00) = (%d, %d, %d, %v), want (0, 255, 0, true)", r, g, b, ok)
	}
	_, _, _, ok = parseCSSHexRGB("FF00")
	if ok {
		t.Error("expected false for short hex")
	}
	_, _, _, ok = parseCSSHexRGB("GG0000")
	if ok {
		t.Error("expected false for invalid hex")
	}
}

func TestCssRGBA(t *testing.T) {
	if got := cssRGBA("FF0000", 0.5); got != "rgba(255, 0, 0, 0.50)" {
		t.Errorf("cssRGBA(FF0000, 0.5) = %q, want rgba(255, 0, 0, 0.50)", got)
	}
	if got := cssRGBA("invalid", 0.5); got != "rgba(0, 0, 0, 0)" {
		t.Errorf("cssRGBA(invalid, 0.5) = %q, want rgba(0, 0, 0, 0)", got)
	}
}

func TestCssSolidFillColor(t *testing.T) {
	if got := cssSolidFillColor(dml.SolidFill{Color: "FF0000"}); got != "#FF0000" {
		t.Errorf("cssSolidFillColor = %q, want #FF0000", got)
	}
	if got := cssSolidFillColor(dml.SolidFill{Color: "FF0000", Alpha: 0.5}); got != "rgba(255, 0, 0, 0.50)" {
		t.Errorf("cssSolidFillColor with alpha = %q, want rgba(255, 0, 0, 0.50)", got)
	}
	if got := cssSolidFillColor(dml.SolidFill{}); got != "" {
		t.Errorf("cssSolidFillColor empty = %q, want empty", got)
	}
}

func TestCssEffectColor(t *testing.T) {
	if got := cssEffectColor("FF0000", 1.0); got != "#FF0000" {
		t.Errorf("cssEffectColor = %q, want #FF0000", got)
	}
	if got := cssEffectColor("FF0000", 0.5); got != "rgba(255, 0, 0, 0.50)" {
		t.Errorf("cssEffectColor with alpha = %q, want rgba(255, 0, 0, 0.50)", got)
	}
}

func TestParseSymbolField(t *testing.T) {
	tests := []struct {
		arg  string
		want string
	}{
		{"65", "A"},
		{"0x2660", "♠"},
		{"0X41", "A"},
		{"", ""},
		{"999999", string(rune(999999))},
	}
	for _, tt := range tests {
		got := parseSymbolField(tt.arg)
		if got != tt.want {
			t.Errorf("parseSymbolField(%q) = %q, want %q", tt.arg, got, tt.want)
		}
	}
}

func TestParseRefNumberFormat(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{`REF bm1 \# "0.00"`, "0.00"},
		{`REF bm1 \# 0`, "0"},
		{`REF bm1`, ""},
		{`REF bm1 \s`, ""},
	}
	for _, tt := range tests {
		got := parseRefNumberFormat(tt.instr)
		if got != tt.want {
			t.Errorf("parseRefNumberFormat(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestFormatRefNumber(t *testing.T) {
	tests := []struct {
		s       string
		picture string
		want    string
	}{
		{"1234.5", "0.00", "1234.50"},
		{"1234", "0", "1234"},
		{"abc", "0.00", "abc"},
		{"1234.5", "$0.00", "$1234.50"},
	}
	for _, tt := range tests {
		got := formatRefNumber(tt.s, tt.picture)
		if got != tt.want {
			t.Errorf("formatRefNumber(%q, %q) = %q, want %q", tt.s, tt.picture, got, tt.want)
		}
	}
}

func TestEvaluateNextIf(t *testing.T) {
	tests := []struct {
		instr string
		want  bool
	}{
		{`NEXTIF "a" = "a"`, true},
		{`NEXTIF "a" = "b"`, false},
		{`NEXTIF 5 > 3`, true},
		{`NEXTIF 3 > 5`, false},
		{`NEXTIF 5 <= 5`, true},
		{`NEXTIF 5 >= 6`, false},
		{`NEXTIF 5 <> 3`, true},
		{`NEXTIF 5 <> 5`, false},
		{"", false},
	}
	for _, tt := range tests {
		got := evaluateNextIf(tt.instr)
		if got != tt.want {
			t.Errorf("evaluateNextIf(%q) = %v, want %v", tt.instr, got, tt.want)
		}
	}
}

func TestEvaluateSkipIf(t *testing.T) {
	tests := []struct {
		instr string
		want  bool
	}{
		{`SKIPIF "a" = "a"`, true},
		{`SKIPIF "a" = "b"`, false},
		{`SKIPIF 5 > 3`, true},
		{"", false},
	}
	for _, tt := range tests {
		got := evaluateSkipIf(tt.instr)
		if got != tt.want {
			t.Errorf("evaluateSkipIf(%q) = %v, want %v", tt.instr, got, tt.want)
		}
	}
}

func TestEvaluateMergeConditionalUsesResolverForUnquotedOperands(t *testing.T) {
	resolver := func(name string) (string, bool) {
		if name == "City" {
			return "Chicago", true
		}
		return "", false
	}
	if !evaluateMergeConditional(`NEXTIF City = "Chicago"`, "NEXTIF", resolver) {
		t.Fatalf("expected resolver-backed NEXTIF comparison to match current merge data")
	}
	if evaluateMergeConditional(`SKIPIF City = "Boston"`, "SKIPIF", resolver) {
		t.Fatalf("expected resolver-backed SKIPIF comparison to fail when merge data differs")
	}
}

func TestEvaluateMergeConditionalCaseSensitive(t *testing.T) {
	// Without \c, comparison is case-insensitive.
	if !evaluateMergeConditional(`NEXTIF "ABC" = "abc"`, "NEXTIF", nil) {
		t.Errorf("expected case-insensitive match without \\c")
	}
	// With \c, comparison is case-sensitive.
	if evaluateMergeConditional(`NEXTIF \c "ABC" = "abc"`, "NEXTIF", nil) {
		t.Errorf("expected no match with case-sensitive \\c")
	}
	if !evaluateMergeConditional(`NEXTIF \c "abc" = "abc"`, "NEXTIF", nil) {
		t.Errorf("expected exact match with case-sensitive \\c")
	}
	// \c can appear after operands.
	if evaluateMergeConditional(`SKIPIF "ABC" = "abc" \c`, "SKIPIF", nil) {
		t.Errorf("expected no match when \\c appears after operands")
	}
	// <> with \c.
	if !evaluateMergeConditional(`NEXTIF \c "ABC" <> "abc"`, "NEXTIF", nil) {
		t.Errorf("expected case-sensitive inequality to be true")
	}
}

func TestParseMergeFieldSwitches(t *testing.T) {
	tests := []struct {
		instr      string
		wantBefore string
		wantAfter  string
	}{
		{`MERGEFIELD City`, "", ""},
		{`MERGEFIELD City \b "["`, "[", ""},
		{`MERGEFIELD City \f "]"`, "", "]"},
		{`MERGEFIELD City \b "[" \f "]"`, "[", "]"},
		{`MERGEFIELD City \f "]" \b "["`, "[", "]"},
		{`MERGEFIELD City \* MERGEFORMAT \b "(" \f ")"`, "(", ")"},
		{`MERGEFIELD "City Name" \b "before "`, "before ", ""},
	}
	for _, tt := range tests {
		gotBefore, gotAfter := parseMergeFieldSwitches(tt.instr)
		if gotBefore != tt.wantBefore || gotAfter != tt.wantAfter {
			t.Errorf("parseMergeFieldSwitches(%q) = (%q, %q), want (%q, %q)",
				tt.instr, gotBefore, gotAfter, tt.wantBefore, tt.wantAfter)
		}
	}
}

func TestResolveFieldInstructionRunsQuotesMultiWordValues(t *testing.T) {
	doc := &resolve.Document{Author: "Alice Smith", Title: "Annual Report"}
	r := &htmlRenderer{doc: doc}

	// Nested AUTHOR field that resolves to a multi-word name.
	instrRuns := []resolve.Run{
		{Text: "IF "},
		{Fields: []resolve.Field{{
			Kind:     "AUTHOR",
			Children: []resolve.Run{},
		}}},
		{Text: ` = "Alice Smith" "yes" "no"`},
	}
	got := r.resolveFieldInstructionRuns(instrRuns)
	want := `IF "Alice Smith" = "Alice Smith" "yes" "no"`
	if got != want {
		t.Errorf("resolveFieldInstructionRuns() = %q, want %q", got, want)
	}

	// Nested TITLE field that resolves to a multi-word title.
	instrRuns = []resolve.Run{
		{Text: "IF "},
		{Fields: []resolve.Field{{
			Kind:     "TITLE",
			Children: []resolve.Run{},
		}}},
		{Text: ` = "Annual Report" "match" "no"`},
	}
	got = r.resolveFieldInstructionRuns(instrRuns)
	want = `IF "Annual Report" = "Annual Report" "match" "no"`
	if got != want {
		t.Errorf("resolveFieldInstructionRuns() = %q, want %q", got, want)
	}

	// Single-word value should not be quoted.
	instrRuns = []resolve.Run{
		{Text: "IF "},
		{Fields: []resolve.Field{{
			Kind:     "AUTHOR",
			Children: []resolve.Run{},
		}}},
		{Text: ` = "Bob" "yes" "no"`},
	}
	// Override doc.Author for single-word test.
	r.doc = &resolve.Document{Author: "Bob"}
	got = r.resolveFieldInstructionRuns(instrRuns)
	want = `IF Bob = "Bob" "yes" "no"`
	if got != want {
		t.Errorf("resolveFieldInstructionRuns() = %q, want %q", got, want)
	}
}

func TestEvaluateIfFieldWithNestedMultiWordField(t *testing.T) {
	doc := &resolve.Document{Author: "Alice Smith"}
	r := &htmlRenderer{doc: doc}

	// Simulate { IF { AUTHOR } = "Alice Smith" "yes" "no" }
	run := resolve.Run{
		FieldKind:  "IF",
		FieldInstr: `IF { AUTHOR } = "Alice Smith" "yes" "no"`,
		FieldInstrRuns: []resolve.Run{
			{Text: "IF "},
			{Fields: []resolve.Field{{
				Kind:     "AUTHOR",
				Children: []resolve.Run{},
			}}},
			{Text: ` = "Alice Smith" "yes" "no"`},
		},
	}
	got, ok := r.evaluateIfFieldRun(run)
	if !ok {
		t.Fatalf("evaluateIfFieldRun() returned ok=false")
	}
	if got != "yes" {
		t.Errorf("evaluateIfFieldRun() = %q, want %q", got, "yes")
	}
}

func TestGenerateTOAWithEntries(t *testing.T) {
	doc := &resolve.Document{Author: "Alice"}
	r := &htmlRenderer{doc: doc, pages: []layout.Page{
		{Number: 1, Blocks: []layout.BlockBox{{
			Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{FieldKind: "TA", FieldArg: `TA \l "Doe v. Smith" \s "Cases" \c 1`},
				},
			}},
		}}},
		{Number: 3, Blocks: []layout.BlockBox{{
			Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{FieldKind: "TA", FieldArg: `TA \l "Roe v. Wade" \s "Cases" \c 1`},
				},
			}},
		}}},
	}}
	got := r.generateTOA(`TOA \h`)
	want := "Cases\nDoe v. Smith, 1\nRoe v. Wade, 3"
	if got != want {
		t.Errorf("generateTOA() = %q, want %q", got, want)
	}
}

func TestGenerateBibliographyWithCitations(t *testing.T) {
	doc := &resolve.Document{Author: "Alice"}
	r := &htmlRenderer{doc: doc, pages: []layout.Page{
		{Number: 1, Blocks: []layout.BlockBox{{
			Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{FieldKind: "CITATION", FieldArg: "Smith2020"},
					{FieldKind: "CITATION", FieldArg: "Jones2019"},
				},
			}},
		}}},
		{Number: 2, Blocks: []layout.BlockBox{{
			Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{
				Runs: []resolve.Run{
					{FieldKind: "CITATION", FieldArg: "Smith2020"},
				},
			}},
		}}},
	}}
	got := r.generateBibliography(``)
	want := "Jones2019\nSmith2020"
	if got != want {
		t.Errorf("generateBibliography() = %q, want %q", got, want)
	}
}

func TestParseGotoButton(t *testing.T) {
	tests := []struct {
		instr        string
		wantBookmark string
		wantDisplay  string
	}{
		{`GOTOBUTTON MyBookmark "Click here"`, "MyBookmark", "Click here"},
		{`GOTOBUTTON MyBookmark Click here`, "MyBookmark", "Click here"},
		{`GOTOBUTTON MyBookmark`, "MyBookmark", "Go to MyBookmark"},
		{"", "", ""},
	}
	for _, tt := range tests {
		bm, disp := parseGotoButton(tt.instr)
		if bm != tt.wantBookmark || disp != tt.wantDisplay {
			t.Errorf("parseGotoButton(%q) = (%q, %q), want (%q, %q)", tt.instr, bm, disp, tt.wantBookmark, tt.wantDisplay)
		}
	}
}

func TestResolveGreetingLine(t *testing.T) {
	r := &htmlRenderer{options: HTMLOptions{MergeData: map[string]string{
		"FirstName": "John",
		"LastName":  "Doe",
		"Title":     "Mr.",
	}}}
	if got := r.resolveGreetingLine(""); got != "Dear Mr. John Doe:" {
		t.Errorf("resolveGreetingLine() = %q, want %q", got, "Dear Mr. John Doe:")
	}
	// No merge data
	r2 := &htmlRenderer{options: HTMLOptions{MergeData: map[string]string{}}}
	if got := r2.resolveGreetingLine(""); got != "Dear Sir or Madam:" {
		t.Errorf("resolveGreetingLine() = %q, want %q", got, "Dear Sir or Madam:")
	}
}

func TestResolveAddressBlock(t *testing.T) {
	r := &htmlRenderer{options: HTMLOptions{MergeData: map[string]string{
		"Company":  "Acme Corp",
		"Address1": "123 Main St",
		"City":     "Springfield",
		"State":    "IL",
	}}}
	want := "Acme Corp\n123 Main St\nSpringfield\nIL"
	if got := r.resolveAddressBlock(""); got != want {
		t.Errorf("resolveAddressBlock() = %q, want %q", got, want)
	}
	// No merge data
	r2 := &htmlRenderer{options: HTMLOptions{MergeData: map[string]string{}}}
	if got := r2.resolveAddressBlock(""); got != "[Address Block]" {
		t.Errorf("resolveAddressBlock() = %q, want %q", got, "[Address Block]")
	}
}

func TestParseGreetingLineOptions(t *testing.T) {
	tests := []struct {
		instr     string
		wantError string
	}{
		{`GREETINGLINE \e "To Whom It May Concern:"`, "To Whom It May Concern:"},
		{`GREETINGLINE`, ""},
		{"", ""},
	}
	for _, tt := range tests {
		opts := parseGreetingLineOptions(tt.instr)
		if opts.errorGreeting != tt.wantError {
			t.Errorf("parseGreetingLineOptions(%q).errorGreeting = %q, want %q", tt.instr, opts.errorGreeting, tt.wantError)
		}
	}
}

func TestResolveGreetingLineWithErrorSwitch(t *testing.T) {
	r := &htmlRenderer{options: HTMLOptions{MergeData: map[string]string{}}}
	// No merge data and no error switch → default fallback.
	if got := r.resolveGreetingLine(`GREETINGLINE`); got != "Dear Sir or Madam:" {
		t.Errorf("resolveGreetingLine() = %q, want %q", got, "Dear Sir or Madam:")
	}
	// No merge data with error switch → use switch.
	if got := r.resolveGreetingLine(`GREETINGLINE \e "To Whom It May Concern:"`); got != "To Whom It May Concern:" {
		t.Errorf("resolveGreetingLine() = %q, want %q", got, "To Whom It May Concern:")
	}
}

func TestParseAddressBlockOptions(t *testing.T) {
	tests := []struct {
		instr          string
		wantInclude    bool
		wantCountryFmt int
	}{
		{`ADDRESSBLOCK \c 2`, false, 2},
		{`ADDRESSBLOCK \c 1`, true, 1},
		{`ADDRESSBLOCK`, true, 0},
		{"", true, 0},
	}
	for _, tt := range tests {
		opts := parseAddressBlockOptions(tt.instr)
		if opts.includeCountry != tt.wantInclude {
			t.Errorf("parseAddressBlockOptions(%q).includeCountry = %v, want %v", tt.instr, opts.includeCountry, tt.wantInclude)
		}
		if opts.countryFormat != tt.wantCountryFmt {
			t.Errorf("parseAddressBlockOptions(%q).countryFormat = %d, want %d", tt.instr, opts.countryFormat, tt.wantCountryFmt)
		}
	}
}

func TestResolveAddressBlockExcludesCountry(t *testing.T) {
	r := &htmlRenderer{options: HTMLOptions{MergeData: map[string]string{
		"City":    "Springfield",
		"Country": "USA",
	}}}
	// With \c 2, country should be excluded.
	got := r.resolveAddressBlock(`ADDRESSBLOCK \c 2`)
	if strings.Contains(got, "USA") {
		t.Errorf("resolveAddressBlock with \\c 2 should exclude country, got: %q", got)
	}
	if !strings.Contains(got, "Springfield") {
		t.Errorf("resolveAddressBlock should include city, got: %q", got)
	}
}

func TestCssTextReflection(t *testing.T) {
	tests := []struct {
		refl *dml.Reflection
		want string
	}{
		{nil, ""},
		{&dml.Reflection{Alpha: 0, Distance: 2, Size: 0.5, Direction: 0}, "-webkit-box-reflect: below 2.00pt linear-gradient(below, rgba(0,0,0,1.00) 0%, transparent 50%)"},
		{&dml.Reflection{Alpha: 0.5, Distance: 1, Size: 1, Direction: 180}, "-webkit-box-reflect: above 1.00pt linear-gradient(above, rgba(0,0,0,0.50) 0%, transparent 100%)"},
		{&dml.Reflection{Alpha: 0, Distance: 0, Size: 0.3, Direction: 90}, "-webkit-box-reflect: right 0.00pt linear-gradient(right, rgba(0,0,0,1.00) 0%, transparent 30%)"},
	}
	for _, tt := range tests {
		got := cssTextReflection(tt.refl)
		if got != tt.want {
			t.Errorf("cssTextReflection(%+v) = %q, want %q", tt.refl, got, tt.want)
		}
	}
}
func TestHTMLMaterializedNotesHaveTargetsAndBacklinks(t *testing.T) {
	doc := &resolve.Document{
		Sections:  []resolve.Section{{PageSize: &resolve.PageSize{Width: 12240, Height: 15840}, PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440}}},
		Footnotes: map[int]resolve.Story{7: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Footnote seven"}}}}}}},
		Endnotes:  map[int]resolve.Story{9: {Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Endnote nine"}}}}}}},
		Blocks:    []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Body"}, {Kind: "footnoteRef", NoteID: 7}, {Kind: "endnoteRef", NoteID: 9}}}}},
	}
	out, err := HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		`<a href="#fn7" id="fnref7"><sup>1</sup></a>`,
		`<a href="#en9" id="enref9"><sup>1</sup></a>`,
		`class="note-target" id="fn7" role="doc-footnote"`,
		`href="#fnref7"`,
		`class="note-target" id="en9" role="doc-endnote"`,
		`href="#enref9"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output: %s", want, out)
		}
	}
}

func TestHTMLPreservesCommentRangeMetadata(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{
			CommentRangeStarts: []int{4}, CommentRangeEnds: []int{4},
			Runs: []resolve.Run{{Text: "Commented text"}, {Kind: "commentRef", CommentID: 4}},
		}}},
		Comments: map[int]resolve.Story{4: {Text: "Comment body", Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Comment body"}}}}}}},
	}
	out, err := HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		`class="word-comment-range-start" data-comment-id="4"`,
		`class="word-comment-range-end" data-comment-id="4"`,
		`aria-describedby="comment4"`,
		`id="comment4" role="note"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output: %s", want, out)
		}
	}
}

func TestHTMLContinuousSectionColumnSeparatorUsesWrapperRule(t *testing.T) {
	geom := layout.PageGeometry{ColumnCount: 2, ColumnGapPt: 36, ColumnSep: true}
	got := pageBodySectionStyleAttr(geom)
	want := ` style="column-count:2;column-gap:36.0pt;column-rule:1px solid #bbb"`
	if got != want {
		t.Fatalf("section style = %q, want %q", got, want)
	}
}

func TestEffectiveTableCellBordersUsesMergedGridEdge(t *testing.T) {
	borders := &resolve.TableBorders{
		Left:    &resolve.BorderSide{Val: "single", Color: "111111"},
		Right:   &resolve.BorderSide{Val: "single", Color: "222222"},
		InsideV: &resolve.BorderSide{Val: "dotted", Color: "333333"},
	}
	got := effectiveTableCellBorders(nil, borders, 0, 1, 0, 2, 2)
	if got == nil || got.Left == nil || got.Right == nil {
		t.Fatalf("missing merged outer borders: %+v", got)
	}
	if got.Left.Color != "111111" || got.Right.Color != "222222" {
		t.Fatalf("merged outer borders = left:%+v right:%+v", got.Left, got.Right)
	}
}

func TestEffectiveTableCellBordersUsesInsideVAfterMergedCell(t *testing.T) {
	borders := &resolve.TableBorders{
		Right:   &resolve.BorderSide{Val: "single", Color: "222222"},
		InsideV: &resolve.BorderSide{Val: "dotted", Color: "333333"},
	}
	got := effectiveTableCellBorders(nil, borders, 0, 1, 0, 3, 2)
	if got == nil || got.Right == nil || got.Right.Color != "333333" {
		t.Fatalf("merged interior right border = %+v, want insideV", got)
	}
}

func TestHTMLTableCellPartialPaddingKeepsTableDefaults(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{{Kind: "table", Table: &resolve.Table{
		Format: resolve.TableFormat{CellPadTop: 20, CellPadRight: 40, CellPadBottom: 60, CellPadLeft: 80},
		Rows: []resolve.TableRow{{Cells: []resolve.TableCell{{
			Format: resolve.TableCellFormat{CellPadTop: 100},
			Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}},
		}}}},
	}}}}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "padding:5.0pt 2.0pt 3.0pt 4.0pt") {
		t.Fatalf("partial cell padding lost table defaults: %s", out)
	}
}

func TestHTMLClampsCellSpanToDeclaredTableGrid(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{{Kind: "table", Table: &resolve.Table{
		Format: resolve.TableFormat{ColumnWidths: []int{1000, 1000}},
		Rows: []resolve.TableRow{{Cells: []resolve.TableCell{
			{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "A"}}}}}},
			{Format: resolve.TableCellFormat{GridSpan: 2}, Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "B"}}}}}},
		}}},
	}}}}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `colspan="2"><p>B`) {
		t.Fatalf("cell span exceeded two-column grid: %s", out)
	}
	if !strings.Contains(out, ">B<") {
		t.Fatalf("expected clamped cell content: %s", out)
	}
}

func TestHTMLMaterializedListNoteTargetIsInsideListItem(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{{
		Kind: "paragraph", NoteKind: "footnote", NoteID: 3,
		Paragraph: &resolve.Paragraph{List: &resolve.ListMarker{Kind: "ordered", Level: 0}, Runs: []resolve.Run{{Text: "note text"}}},
	}}}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	li := strings.Index(out, "<li")
	target := strings.Index(out, `id="fn3"`)
	closeLI := strings.Index(out, "</li>")
	if li < 0 || target < li || closeLI < target {
		t.Fatalf("note target must be nested inside its list item: %s", out)
	}
}

func TestHTMLPrintCSSHidesInteractiveNoteChrome(t *testing.T) {
	out, err := HTML(&resolve.Document{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `@media print { .diagnostics, .note-backlink, .comment-ref, aside.notes .comments { display:none !important; } }`) {
		t.Fatalf("missing print-only suppression for interactive note/comment chrome: %s", out)
	}
}

func TestHTMLNestedListsRemainInsideParentListItems(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 1, Runs: []resolve.Run{{Text: "Parent"}}, List: &resolve.ListMarker{Kind: "bullet", Level: 0}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 2, Runs: []resolve.Run{{Text: "Child"}}, List: &resolve.ListMarker{Kind: "bullet", Level: 1}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 3, Runs: []resolve.Run{{Text: "Parent sibling"}}, List: &resolve.ListMarker{Kind: "bullet", Level: 0}}},
		},
	}
	out, err := HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	parent := strings.Index(out, ">Parent")
	childList := strings.Index(out[parent:], "<ul")
	parentClose := strings.Index(out[parent:], "</li>")
	if parent < 0 || childList < 0 || parentClose < 0 || childList > parentClose {
		t.Fatalf("nested list must be emitted before the parent </li>: %s", out)
	}
	if strings.Contains(out, "</li>\n<ul>\n<li") {
		t.Fatalf("nested list escaped its owning list item: %s", out)
	}
}

func TestHTMLOrderedListsRenderResolvedWordNumberingTemplates(t *testing.T) {
	doc := &resolve.Document{
		Blocks: []resolve.Block{
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 11, Runs: []resolve.Run{{Text: "One"}}, List: &resolve.ListMarker{Kind: "ordered", Level: 0, Format: "decimal", Template: "%1."}, Numbering: &resolve.Numbering{NumID: 7, Level: 0, Start: 1, Format: "decimal", Text: "%1."}}},
			{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 12, Runs: []resolve.Run{{Text: "One one"}}, List: &resolve.ListMarker{Kind: "ordered", Level: 1, Format: "decimal", Template: "%1.%2."}, Numbering: &resolve.Numbering{NumID: 7, Level: 1, Start: 1, Format: "decimal", Text: "%1.%2."}}},
		},
	}
	out, err := HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `>1.</span>`) {
		t.Fatalf("missing resolved level-0 marker: %s", out)
	}
	if !strings.Contains(out, `>1.1.</span>`) {
		t.Fatalf("missing resolved multi-level marker: %s", out)
	}
	if !strings.Contains(out, `list-style-type:none`) {
		t.Fatalf("ordered list items should disable browser-generated numbering: %s", out)
	}
}

func TestHTMLOrderedListsPreferSemanticTemplateOverStaleResolvedText(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 1, Runs: []resolve.Run{{Text: "Parent"}}, List: &resolve.ListMarker{Kind: "ordered", Level: 0, Format: "decimal", Template: "%1."}, Numbering: &resolve.Numbering{NumID: 9, Level: 0, Start: 1, Format: "decimal", Text: "2."}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 2, Runs: []resolve.Run{{Text: "Child"}}, List: &resolve.ListMarker{Kind: "ordered", Level: 1, Format: "decimal", Template: "%1.%2."}, Numbering: &resolve.Numbering{NumID: 9, Level: 1, Start: 1, Format: "decimal", Text: "2.1."}}},
	}}
	html, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `>1.1.</span>`) {
		t.Fatalf("template-resolved nested marker missing:\n%s", html)
	}
}

func TestHTMLCommentReferencesDoNotDisturbDocumentFlow(t *testing.T) {
	doc := &resolve.Document{}
	htmlText, err := HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(htmlText, ".comment-ref { cursor: help; position:absolute;") {
		t.Fatalf("comment references should remain semantic without consuming inline layout space")
	}
}

func TestHTMLBulletListsUseExplicitWordMarkerAndDisableNativeMarker(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{
			Index:     1,
			Runs:      []resolve.Run{{Text: "Bullet text"}},
			List:      &resolve.ListMarker{Kind: "bullet", Level: 0, Template: "•"},
			Numbering: &resolve.Numbering{NumID: 5, Level: 0, Format: "bullet", Text: "•"},
		}},
	}}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `list-style-type:none`) {
		t.Fatalf("bullet list item should disable native browser marker: %s", out)
	}
	if !strings.Contains(out, `class="word-list-marker word-list-bullet" aria-hidden="true">•</span>`) {
		t.Fatalf("explicit Word bullet marker missing: %s", out)
	}
}

func TestHTMLBulletMarkerPreservesNumberingRunFormat(t *testing.T) {
	font := "Wingdings"
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{
			Index:     1,
			Runs:      []resolve.Run{{Text: "Tick"}},
			List:      &resolve.ListMarker{Kind: "bullet", Level: 0, Template: "ü"},
			Numbering: &resolve.Numbering{NumID: 8, Level: 0, Format: "bullet", Text: "ü", RunFormat: &resolve.RunFormat{FontASCII: font}},
		}},
	}}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `word-list-bullet`) || !strings.Contains(out, `font-family:Wingdings`) || !strings.Contains(out, `>ü</span>`) {
		t.Fatalf("symbol-font bullet formatting was not preserved: %s", out)
	}
}

func TestBulletMarkerTextNormalizesLegacyOfficePrivateUseGlyphs(t *testing.T) {
	tests := []struct {
		legacy string
		want   string
	}{
		{"\uF0B7", "•"},
		{"\uF0A7", "▪"},
		{"\uF0A8", "□"},
		{"\uF0D8", "➢"},
		{"\uF0FC", "✓"},
		{"\uF076", "◆"},
	}
	for _, tc := range tests {
		list := &resolve.ListMarker{Kind: "bullet", Template: tc.legacy}
		if got := bulletMarkerText(list, nil); got != tc.want {
			t.Fatalf("bulletMarkerText(%U) = %q, want %q", []rune(tc.legacy)[0], got, tc.want)
		}
	}
}

func TestHTMLLegacyPrivateUseBulletDoesNotLeakIntoOutput(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{
		Index: 1, Runs: []resolve.Run{{Text: "Bullet text"}},
		List:      &resolve.ListMarker{Kind: "bullet", Level: 0, Template: "\uF0B7"},
		Numbering: &resolve.Numbering{NumID: 5, Level: 0, Format: "bullet", Text: "\uF0B7"},
	}}}}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\uF0B7") {
		t.Fatalf("legacy Symbol private-use bullet leaked into HTML: %s", out)
	}
	if !strings.Contains(out, `word-list-bullet`+`" aria-hidden="true">•</span>`) {
		t.Fatalf("normalized Unicode bullet missing: %s", out)
	}
}

func TestSamePageBodySectionGeometryDistinguishesExplicitColumnWidths(t *testing.T) {
	left := layout.PageGeometry{ColumnCount: 2, ColumnGapPt: 18, ColumnWidthsPt: []float64{180, 300}, ColumnGapsPt: []float64{18}}
	right := left
	right.ColumnWidthsPt = []float64{220, 260}
	if samePageBodySectionGeometry(left, right) {
		t.Fatal("different physical column widths were treated as identical")
	}
}

func TestSamePageBodySectionGeometryAcceptsEquivalentExplicitColumns(t *testing.T) {
	left := layout.PageGeometry{ColumnCount: 2, ColumnGapPt: 18, ColumnSep: true, ColumnWidthsPt: []float64{240, 240}, ColumnGapsPt: []float64{18}}
	right := left
	if !samePageBodySectionGeometry(left, right) {
		t.Fatal("equivalent explicit column geometry should match")
	}
}

func TestHTMLStylesIncludeFootnoteSeparator(t *testing.T) {
	doc := &resolve.Document{}
	html, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, ".page-footnotes::before") || !strings.Contains(html, "border-top: 0.75pt solid #000") {
		t.Fatalf("footnote separator CSS missing")
	}
}

func TestHTMLUnequalPhysicalColumnsUseExplicitGridTracks(t *testing.T) {
	geom := layout.PageGeometry{
		ColumnCount:    3,
		ColumnGapPt:    18,
		ColumnWidthsPt: []float64{120, 180, 210},
		ColumnGapsPt:   []float64{12, 24},
	}
	got := pageBodySectionStyleAttr(geom)
	for _, want := range []string{
		"display:grid",
		"grid-template-columns:120.0pt 12.0pt 180.0pt 24.0pt 210.0pt",
		"align-items:start",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("section style %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "column-count") {
		t.Fatalf("unequal physical columns fell back to browser multicolumn flow: %q", got)
	}
}

func TestHTMLExplicitPhysicalColumnChildUsesPhysicalGridTrack(t *testing.T) {
	geom := layout.PageGeometry{
		ColumnCount:    3,
		ColumnGapPt:    18,
		ColumnWidthsPt: []float64{120, 180, 210},
		ColumnGapsPt:   []float64{12, 24},
	}
	if got := pageBodyColumnStyleAttr(geom, 0); !strings.Contains(got, "grid-column:1") {
		t.Fatalf("column 0 style = %q", got)
	}
	if got := pageBodyColumnStyleAttr(geom, 1); !strings.Contains(got, "grid-column:3") {
		t.Fatalf("column 1 style = %q", got)
	}
	if got := pageBodyColumnStyleAttr(geom, 2); !strings.Contains(got, "grid-column:5") {
		t.Fatalf("column 2 style = %q", got)
	}
}

func TestHTMLPageBodyDoesNotApplyBrowserColumnsOverExplicitPhysicalGrid(t *testing.T) {
	r := &htmlRenderer{
		pageHeightPt:       792,
		basePageHeightPt:   792,
		marginTopPt:        72,
		baseMarginTopPt:    72,
		marginRightPt:      72,
		baseMarginRightPt:  72,
		marginBottomPt:     72,
		baseMarginBottomPt: 72,
		marginLeftPt:       72,
		baseMarginLeftPt:   72,
		columnCount:        2,
		baseColumnCount:    2,
		columnGapPt:        18,
		baseColumnGapPt:    18,
		columnWidthsPt:     []float64{180, 270},
		baseColumnWidthsPt: []float64{180, 270},
		columnGapsPt:       []float64{18},
		baseColumnGapsPt:   []float64{18},
	}
	if got := r.documentBodyStyleAttr(); strings.Contains(got, "column-count") {
		t.Fatalf("page body double-applied browser columns over explicit grid: %q", got)
	}
}

func TestHTMLExplicitEqualPhysicalColumnsStillUseWordGrid(t *testing.T) {
	geom := layout.PageGeometry{
		ColumnCount: 2, ColumnWidthPt: 216, ColumnGapPt: 36,
		ColumnWidthsPt: []float64{216, 216}, ColumnGapsPt: []float64{36},
		ExplicitColumns: true,
	}
	if !hasExplicitPhysicalColumns(geom) {
		t.Fatal("source-supplied equal physical columns must remain explicit")
	}
	if got := explicitColumnGridTemplate(geom); got != "216.0pt 36.0pt 216.0pt" {
		t.Fatalf("grid = %q", got)
	}
}

func TestPhysicalFragmentRunsPreservesWrappedWordSpacing(t *testing.T) {
	fragment := &layout.ParagraphFragment{Lines: []layout.ParagraphLineLayout{
		{Slots: []layout.ParagraphSlotLayout{{Runs: []resolve.Run{{Text: "Multi-"}}}}},
		{Slots: []layout.ParagraphSlotLayout{{Runs: []resolve.Run{{Text: "column"}, {Text: " "}, {Text: "flow"}}}}},
		{Slots: []layout.ParagraphSlotLayout{{Runs: []resolve.Run{{Text: "text."}}}}},
	}}
	var got strings.Builder
	for _, run := range physicalFragmentRuns(fragment) {
		got.WriteString(run.Text)
	}
	if got.String() != "Multi-column flow text." {
		t.Fatalf("fragment text = %q", got.String())
	}
}

func TestHTMLPrintHidesDiagnostics(t *testing.T) {
	doc := &resolve.Document{Diagnostics: diag.List{{Severity: diag.SeverityInfo, Code: "legacy.test", Message: "test diagnostic"}}}
	html, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "@media print { .diagnostics,") {
		t.Fatalf("print CSS does not hide diagnostics: %s", html)
	}
}

func TestOOXMLParagraphAutoLineSpacingDoesNotDoubleApplyNormalLineFactor(t *testing.T) {
	spacing := 276
	p := &resolve.Paragraph{Format: resolve.ParagraphFormat{LineSpacing: &spacing, LineRule: "auto"}}
	got := paragraphStyleAttrForParagraph(p, false, false, false)
	if !strings.Contains(got, "line-height:1.200") {
		t.Fatalf("OOXML paragraph style = %q, want line-height:1.200", got)
	}
	if strings.Contains(got, "line-height:1.380") {
		t.Fatalf("OOXML paragraph style still double-applies normal line factor: %q", got)
	}
}

func TestLegacyParagraphAutoLineSpacingRetainsCalibratedFactor(t *testing.T) {
	spacing := 276
	p := &resolve.Paragraph{
		Format: resolve.ParagraphFormat{LineSpacing: &spacing, LineRule: "auto"},
		Source: &resolve.ParagraphSource{},
	}
	got := paragraphStyleAttrForParagraph(p, false, false, false)
	if !strings.Contains(got, "line-height:1.200") {
		t.Fatalf("legacy paragraph style = %q, want line-height:1.200", got)
	}
}

func TestHTMLTableFragmentRendersOnlyRequestedRowsAndRepeatsHeader(t *testing.T) {
	table := &resolve.Table{
		Format: resolve.TableFormat{Caption: "Whole table"},
		Rows: []resolve.TableRow{
			{Format: resolve.TableRowFormat{IsHeader: true}, Cells: []resolve.TableCell{{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Header"}}}}}}}},
			{Cells: []resolve.TableCell{{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "One"}}}}}}}},
			{Cells: []resolve.TableCell{{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Two"}}}}}}}},
			{Cells: []resolve.TableCell{{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Text: "Three"}}}}}}}},
		},
	}
	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderTableFragment(table, &layout.TableFragment{StartRow: 2, EndRow: 4, RepeatHeaders: true, IsContinuation: true, IsLast: true})
	got := r.buf.String()
	if !strings.Contains(got, "Header") || !strings.Contains(got, "Two") || !strings.Contains(got, "Three") {
		t.Fatalf("fragment output missing expected rows: %s", got)
	}
	if strings.Contains(got, ">One<") {
		t.Fatalf("fragment output leaked row outside range: %s", got)
	}
	if strings.Contains(got, "<caption>") {
		t.Fatalf("continuation fragment repeated table caption: %s", got)
	}
}
func TestRotatedProxyGeometryExpandsAndBuildsPolygon(t *testing.T) {
	x, y, w, h, poly := rotatedProxyGeometry(100, 200, 80, 40, 30)
	if w <= 80 || h <= 40 {
		t.Fatalf("proxy size = %.2fx%.2f, want expansion beyond 80x40", w, h)
	}
	if x >= 100 || y >= 200 {
		t.Fatalf("proxy origin = %.2f,%.2f, want shifted outward from 100,200", x, y)
	}
	if !strings.HasPrefix(poly, "polygon(") || strings.Count(poly, "%") != 8 {
		t.Fatalf("polygon = %q, want four percentage coordinates", poly)
	}
}

func TestBareLegacyOLEAnchorDoesNotLeakPlaceholderText(t *testing.T) {
	r := &htmlRenderer{}
	r.renderRun(resolve.Run{Kind: "olePlaceholder", Text: "Embedded Object"})
	if got := r.buf.String(); got != "" {
		t.Fatalf("bare OLE anchor rendered %q, want empty output", got)
	}
}
func TestWrapPolygonProxyCSSScalesAndRotatesInsideProxy(t *testing.T) {
	const emuPerPoint = int64(12700)
	proxyX, proxyY, proxyW, proxyH, _ := rotatedProxyGeometry(20, 30, 200, 100, 90)
	got := wrapPolygonProxyCSS(
		[][2]float64{{0, 0}, {100, 0}, {100, 50}, {0, 50}},
		100*emuPerPoint, 50*emuPerPoint,
		200, 100,
		proxyX, proxyY, proxyW, proxyH,
		20, 30, 90,
	)
	want := "polygon(100.00% 0.00%, 100.00% 100.00%, 0.00% 100.00%, 0.00% 0.00%)"
	if got != want {
		t.Fatalf("wrap polygon css = %q, want %q", got, want)
	}
}

func TestOrdinaryPhysicalParagraphFragmentOwnsRenderedLineBreaks(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "browser must not reflow this source text"}}}
	box := layout.BlockBox{
		Block:    &resolve.Block{Kind: "paragraph", Paragraph: p},
		Geometry: layout.PageGeometry{MarginLeft: 72, MarginTop: 72, ColumnCount: 1},
		Y:        20,
		Height:   24,
		Fragment: &layout.ParagraphFragment{
			// Ordinary full-width lines deliberately do not require constrained
			// spatial rendering. Their physical topology is still authoritative.
			SpatialRendering: false,
			Lines: []layout.ParagraphLineLayout{
				{Slots: []layout.ParagraphSlotLayout{{
					Left: 72, Right: 272, Top: 96, Bottom: 108,
					Runs: []resolve.Run{{Kind: "run", Text: "layout line one"}},
				}}},
				{Slots: []layout.ParagraphSlotLayout{{
					Left: 72, Right: 272, Top: 108, Bottom: 120,
					Runs: []resolve.Run{{Kind: "run", Text: "layout line two"}},
				}}},
			},
			IsLast: true,
		},
	}

	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderPageParagraphBox([]layout.BlockBox{box}, 0)
	got := r.buf.String()

	if strings.Contains(got, "browser must not reflow this source text") {
		t.Fatalf("ordinary physical fragment fell back to semantic/browser reflow: %s", got)
	}
	for _, want := range []string{
		`class="word-physical-slot" data-line="0" data-slot="0"`,
		`class="word-physical-slot" data-line="1" data-slot="0"`,
		">layout line one<",
		">layout line two<",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ordinary physical fragment missing %q: %s", want, got)
		}
	}
}

func TestPhysicalParagraphFragmentUsesSlotGeometryInsteadOfFlattenedRuns(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{{Kind: "run", Text: "source"}}}
	box := layout.BlockBox{
		Block:    &resolve.Block{Kind: "paragraph", Paragraph: p},
		Geometry: layout.PageGeometry{MarginLeft: 72, MarginTop: 72, ColumnCount: 1},
		Y:        20,
		Height:   24,
		Fragment: &layout.ParagraphFragment{
			SpatialRendering: true,
			Lines: []layout.ParagraphLineLayout{{Slots: []layout.ParagraphSlotLayout{
				{Left: 82, Right: 122, Top: 96, Bottom: 108, Runs: []resolve.Run{{Kind: "run", Text: "left"}}},
				{Left: 172, Right: 222, Top: 96, Bottom: 108, Runs: []resolve.Run{{Kind: "run", Text: "right"}}},
			}}},
			IsLast: true,
		},
	}

	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderPageParagraphBox([]layout.BlockBox{box}, 0)
	got := r.buf.String()
	// There is intentionally no flattened ParagraphFragment.Runs shadow anymore;
	// the slot text below is the only renderable fragment content.
	for _, want := range []string{
		`class="word-physical-slot" data-line="0" data-slot="0"`,
		`left:10.00pt;top:4.00pt;width:40.00pt;height:12.00pt;line-height:12.00pt`,
		`class="word-physical-slot" data-line="0" data-slot="1"`,
		`left:100.00pt;top:4.00pt;width:50.00pt;height:12.00pt;line-height:12.00pt`,
		">left<", ">right<",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("physical fragment missing %q: %s", want, got)
		}
	}
}

func TestPhysicalParagraphContinuationRebasesVerticalGeometry(t *testing.T) {
	fragment := &layout.ParagraphFragment{
		SpatialRendering: true,
		IsContinuation:   true,
		IsLast:           true,
		Lines: []layout.ParagraphLineLayout{
			{Slots: []layout.ParagraphSlotLayout{{Left: 72, Right: 172, Top: 700, Bottom: 712, Runs: []resolve.Run{{Kind: "run", Text: "one"}}}}},
			{Slots: []layout.ParagraphSlotLayout{{Left: 72, Right: 172, Top: 712, Bottom: 724, Runs: []resolve.Run{{Kind: "run", Text: "two"}}}}},
		},
	}
	box := layout.BlockBox{
		Block:    &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{}},
		Geometry: layout.PageGeometry{MarginLeft: 72, MarginTop: 72, ColumnCount: 1},
		Y:        0,
		Height:   24,
		Fragment: fragment,
	}

	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderPhysicalParagraphFragment(box)
	got := r.buf.String()
	if !strings.Contains(got, `left:0.00pt;top:0.00pt;width:100.00pt;height:12.00pt;line-height:12.00pt`) ||
		!strings.Contains(got, `left:0.00pt;top:12.00pt;width:100.00pt;height:12.00pt`) {
		t.Fatalf("continuation slot geometry was not rebased to the fragment origin: %s", got)
	}
}

func TestRenderRunsUsesDefaultTabStopsWithoutCustomStops(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{{Text: "Column1"}, {Kind: "tab"}, {Text: "Column2"}}}
	r := &htmlRenderer{doc: &resolve.Document{DefaultTabStop: 720}, currentParagraph: p}
	r.resetTabState(p)
	r.renderRuns(p.Runs)
	got := r.buf.String()
	if !strings.Contains(got, "display:inline-block;width:") {
		t.Fatalf("ordinary-flow tab was not rendered with a default tab stop: %s", got)
	}
	if !strings.Contains(got, "Column1") || !strings.Contains(got, "Column2") {
		t.Fatalf("tab rendering lost surrounding text: %s", got)
	}
}

func TestPhysicalParagraphContinuationDoesNotRepeatFloatingAnchor(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{
		{Kind: "image", Image: &resolve.Image{Floating: true, WrapType: "square", Width: 12700, Height: 12700}},
		{Kind: "run", Text: "continued"},
	}}
	box := layout.BlockBox{
		Block:    &resolve.Block{Kind: "paragraph", Paragraph: p},
		Geometry: layout.PageGeometry{MarginLeft: 72, MarginTop: 72, ColumnCount: 1},
		Height:   12,
		Fragment: &layout.ParagraphFragment{
			SpatialRendering: true,
			IsContinuation:   true,
			IsLast:           true,
			Lines: []layout.ParagraphLineLayout{{Slots: []layout.ParagraphSlotLayout{{
				Left: 72, Right: 172, Top: 72, Bottom: 84,
				Runs: []resolve.Run{{Kind: "run", Text: "continued"}},
			}}}},
		},
	}
	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderPhysicalParagraphFragment(box)
	got := r.buf.String()
	if strings.Contains(got, "<img") || strings.Contains(got, "floating-image") {
		t.Fatalf("continuation fragment repeated floating anchor: %s", got)
	}
	if !strings.Contains(got, "continued") {
		t.Fatalf("continuation text missing: %s", got)
	}
}

func TestPeekAheadTabFieldMetricsUsesMeasuredDecimalPrefix(t *testing.T) {
	r := &htmlRenderer{doc: &resolve.Document{}}
	runs := []resolve.Run{{Text: "12.34"}}
	field, prefix := r.peekAheadTabFieldMetrics(runs, 0)
	wantField := r.estimateTextWidthForRun("12.34", resolve.RunFormat{})
	wantPrefix := r.estimateTextWidthForRun("12", resolve.RunFormat{})
	if math.Abs(field-wantField) > 0.001 {
		t.Fatalf("field width = %.3f, want %.3f", field, wantField)
	}
	if math.Abs(prefix-wantPrefix) > 0.001 {
		t.Fatalf("decimal prefix = %.3f, want %.3f", prefix, wantPrefix)
	}
	if math.Abs(prefix-field/2) < 0.001 {
		t.Fatalf("decimal prefix unexpectedly fell back to half-width approximation: field=%.3f prefix=%.3f", field, prefix)
	}
}

func TestPhysicalPositionedRunsConsumeAdvanceGeometryWithoutReapplyingField(t *testing.T) {
	advance := resolve.Run{Kind: "run", FieldKind: "ADVANCE", FieldInstr: `ADVANCE \r 12 \u 3`}
	fragment := &layout.ParagraphFragment{
		Lines: []layout.ParagraphLineLayout{{Slots: []layout.ParagraphSlotLayout{{
			Left: 72, Right: 200, Top: 72, Bottom: 86,
			Runs: []resolve.Run{{Kind: "run", Text: "A"}, advance, {Kind: "run", Text: "B"}},
			PositionedRuns: []layout.ParagraphRunLayout{
				{Run: resolve.Run{Kind: "run", Text: "A"}, Left: 0, Top: 0, Width: 6},
				{Run: advance, Left: 6, Top: 0},
				{Run: resolve.Run{Kind: "run", Text: "B"}, Left: 18, Top: -3, Width: 6},
			},
		}}}},
	}
	box := layout.BlockBox{
		Block:    &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{}},
		Geometry: layout.PageGeometry{MarginLeft: 72, MarginTop: 72, ColumnCount: 1},
		Height:   14, Fragment: fragment,
	}
	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderPhysicalParagraphFragment(box)
	got := r.buf.String()
	for _, want := range []string{
		`data-layout-owned="true"`,
		`class="word-physical-run" style="position:absolute;left:18.00pt;top:-3.00pt`,
		`>B<`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("layout-owned ADVANCE rendering missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "margin-left:12.00pt") || strings.Contains(got, "translateY(3.00pt)") {
		t.Fatalf("renderer re-applied ADVANCE instead of consuming physical geometry: %s", got)
	}
}

func TestPhysicalPositionedRunsConsumeLayoutOwnedSuperscriptAndPosition(t *testing.T) {
	size := 20
	pos := 6
	fragment := &layout.ParagraphFragment{Lines: []layout.ParagraphLineLayout{{Slots: []layout.ParagraphSlotLayout{{
		Left: 72, Right: 200, Top: 72, Bottom: 86,
		PositionedRuns: []layout.ParagraphRunLayout{
			{Run: resolve.Run{Kind: "run", Text: "2", Format: resolve.RunFormat{FontSize: &size, VertAlignVal: "superscript"}}, Left: 0, Top: -3.3, Width: 4, Height: 6.5},
			{Run: resolve.Run{Kind: "run", Text: "B", Format: resolve.RunFormat{FontSize: &size, Position: &pos}}, Left: 5, Top: -3, Width: 5, Height: 10},
		},
	}}}}}
	box := layout.BlockBox{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{}}, Geometry: layout.PageGeometry{MarginLeft: 72, MarginTop: 72, ColumnCount: 1}, Height: 14, Fragment: fragment}
	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderPhysicalParagraphFragment(box)
	got := r.buf.String()
	if strings.Contains(got, "<sup") || strings.Contains(got, "vertical-align:3.0pt") {
		t.Fatalf("renderer re-applied vertical semantics already owned by physical layout: %s", got)
	}
	for _, want := range []string{`top:-3.30pt`, `font-size:6.5pt`, `top:-3.00pt`} {
		if !strings.Contains(got, want) {
			t.Fatalf("layout-owned vertical rendering missing %q: %s", want, got)
		}
	}
}

func TestPhysicalInlineImageConsumesLayoutOwnedVerticalBox(t *testing.T) {
	const emuPerPoint = int64(12700)
	img := &resolve.Image{Width: 20 * emuPerPoint, Height: 8 * emuPerPoint, MIMEType: "image/png", Data: []byte{1}}
	fragment := &layout.ParagraphFragment{Lines: []layout.ParagraphLineLayout{{Slots: []layout.ParagraphSlotLayout{{
		Left: 72, Right: 200, Top: 72, Bottom: 86,
		PositionedRuns: []layout.ParagraphRunLayout{{Run: resolve.Run{Kind: "image", Image: img}, Left: 10, Top: 6, Width: 20, Height: 8}},
	}}}}}
	box := layout.BlockBox{Block: &resolve.Block{Kind: "paragraph", Paragraph: &resolve.Paragraph{}}, Geometry: layout.PageGeometry{MarginLeft: 72, MarginTop: 72, ColumnCount: 1}, Height: 14, Fragment: fragment}
	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderPhysicalParagraphFragment(box)
	got := r.buf.String()
	for _, want := range []string{`left:10.00pt`, `top:6.00pt`, `width:20.00pt;height:8.00pt`} {
		if !strings.Contains(got, want) {
			t.Fatalf("layout-owned inline object rendering missing %q: %s", want, got)
		}
	}
}

func TestHTMLOrderedListsDoNotFreezeOnResolvedLiteralMarker(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 1, Runs: []resolve.Run{{Text: "First"}}, List: &resolve.ListMarker{Kind: "ordered", Level: 0, Format: "decimal"}, Numbering: &resolve.Numbering{NumID: 3, Level: 0, Start: 1, Format: "decimal", Text: "3."}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 2, Runs: []resolve.Run{{Text: "Second"}}, List: &resolve.ListMarker{Kind: "ordered", Level: 0, Format: "decimal"}, Numbering: &resolve.Numbering{NumID: 3, Level: 0, Start: 1, Format: "decimal", Text: "3."}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 3, Runs: []resolve.Run{{Text: "Third"}}, List: &resolve.ListMarker{Kind: "ordered", Level: 0, Format: "decimal"}, Numbering: &resolve.Numbering{NumID: 3, Level: 0, Start: 1, Format: "decimal", Text: "3."}}},
	}}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{">1.</span>", ">2.</span>", ">3.</span>"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("missing sequential marker %q:\n%s", marker, out)
		}
	}
}

func TestHTMLPhysicalNestedListReservesParentRowBeforeChildren(t *testing.T) {
	doc := &resolve.Document{Blocks: []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 1, Runs: []resolve.Run{{Text: "Parent"}}, List: &resolve.ListMarker{Kind: "bullet", Level: 0}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 2, Runs: []resolve.Run{{Text: "Child"}}, List: &resolve.ListMarker{Kind: "bullet", Level: 1}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Index: 3, Runs: []resolve.Run{{Text: "Sibling"}}, List: &resolve.ListMarker{Kind: "bullet", Level: 0}}},
	}}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `--word-list-item-flow-height:`) {
		t.Fatalf("physical list item must expose its measured row height:\n%s", out)
	}
	if !strings.Contains(out, `li > ul, li > ol { padding-top: var(--word-list-item-flow-height, 0pt); }`) {
		t.Fatalf("nested list must start below its owning physical row")
	}
}
