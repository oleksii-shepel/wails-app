package resolve

import (
	"math"
	"strings"
	"testing"

	"office/pkg/ooxml/chart"
	dml "office/pkg/ooxml/drawing"
	"office/pkg/ooxml/smartart"
	smartartrender "office/pkg/ooxml/smartart/render"
	"office/pkg/ooxml/xmltree"
	"office/pkg/word/parsers/ooxml/parts"
)

func TestNormalizeRunPreservesCharacterStyleID(t *testing.T) {
	italic := true
	ctx := &resolveCtx{charStyles: map[string]CharacterStyle{
		"Emphasis": {ID: "Emphasis", Format: RunFormat{Italic: &italic}},
	}}
	raw := &parts.Run{
		Properties: &parts.RunProperties{RunStyle: &parts.StringVal{Val: "Emphasis"}},
		Contents:   []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "styled"}}},
	}
	runs := normalizeRun(ctx, raw, "", "", RunFormat{})
	if len(runs) != 1 || runs[0].StyleID != "Emphasis" {
		t.Fatalf("runs=%+v, want preserved Emphasis style", runs)
	}
	if runs[0].Format.Italic == nil || !*runs[0].Format.Italic {
		t.Fatalf("run format=%+v, want character style formatting", runs[0].Format)
	}
}

func TestParseFieldStyleRef(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{`STYLEREF "Heading 1"`, "Heading 1"},
		{`STYLEREF Heading1`, "Heading1"},
		{`STYLEREF "Heading 1" \l`, "Heading 1"},
		{`STYLEREF`, ""},
		{`STYLEREF \l`, ""},
	}
	for _, tt := range tests {
		got := parseFieldStyleRef(tt.instr)
		if got != tt.want {
			t.Errorf("parseFieldStyleRef(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestParseDocPropertyName(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{`DOCPROPERTY "Title"`, "Title"},
		{`DOCPROPERTY Title`, "Title"},
		{`DOCPROPERTY`, ""},
		{`DOCPROPERTY \l`, ""},
		{`DOCVARIABLE "ProjectID"`, "ProjectID"},
		{`DOCVARIABLE ProjectID`, "ProjectID"},
		{`DOCVARIABLE`, ""},
	}
	for _, tt := range tests {
		got := parseDocPropertyName(tt.instr)
		if got != tt.want {
			t.Errorf("parseDocPropertyName(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestParseFieldTarget(t *testing.T) {
	tests := []struct {
		rest string
		want string
	}{
		{"BookmarkName \\l", "BookmarkName"},
		{"\\l", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := parseFieldTarget(tt.rest)
		if got != tt.want {
			t.Errorf("parseFieldTarget(%q) = %q, want %q", tt.rest, got, tt.want)
		}
	}
}

func TestParseFieldHyperlink(t *testing.T) {
	tests := []struct {
		instr      string
		wantURL    string
		wantAnchor string
	}{
		{`HYPERLINK "https://example.com"`, "https://example.com", ""},
		{`HYPERLINK "https://example.com" \l "section1"`, "https://example.com", "section1"},
		{`HYPERLINK ""`, "", ""},
	}
	for _, tt := range tests {
		url, anchor := parseFieldHyperlink(tt.instr)
		if url != tt.wantURL || anchor != tt.wantAnchor {
			t.Errorf("parseFieldHyperlink(%q) = (%q, %q), want (%q, %q)", tt.instr, url, anchor, tt.wantURL, tt.wantAnchor)
		}
	}
}

func TestParseFieldRef(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{`REF BookmarkName`, "BookmarkName"},
		{`REF "Bookmark Name"`, `"Bookmark`},
		{`REF`, ""},
	}
	for _, tt := range tests {
		got := parseFieldRef(tt.instr)
		if got != tt.want {
			t.Errorf("parseFieldRef(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestAggregateFieldDisplayText(t *testing.T) {
	runs := []Run{{Text: "Hello"}, {Text: " "}, {Text: "World"}}
	got := aggregateFieldDisplayText(runs)
	if got != "Hello World" {
		t.Errorf("aggregateFieldDisplayText = %q, want %q", got, "Hello World")
	}
	if aggregateFieldDisplayText(nil) != "" {
		t.Error("expected empty string for nil runs")
	}
}

func TestExtractFormFieldData(t *testing.T) {
	// text input
	ff := &parts.FormFieldData{Kind: "text", Name: "Field1", DefaultTxt: "hello", MaxLength: 10}
	run := &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{FormField: ff}}}}
	got := extractFormFieldData(run)
	if got == nil || got.Kind != "text" || got.Name != "Field1" || got.DefaultTxt != "hello" || got.MaxLength != 10 {
		t.Errorf("extractFormFieldData text = %+v", got)
	}
	// checkbox
	ff = &parts.FormFieldData{Kind: "checkbox", Name: "Check1", Checked: true}
	run = &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{FormField: ff}}}}
	got = extractFormFieldData(run)
	if got == nil || got.Kind != "checkbox" || !got.Checked {
		t.Errorf("extractFormFieldData checkbox = %+v", got)
	}
	// dropdown
	ff = &parts.FormFieldData{Kind: "dropdown", Name: "Drop1", DropItems: []string{"a", "b"}, DropDefIdx: 1}
	run = &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{FormField: ff}}}}
	got = extractFormFieldData(run)
	if got == nil || got.Kind != "dropdown" || len(got.DropItems) != 2 || got.DropDefIdx != 1 {
		t.Errorf("extractFormFieldData dropdown = %+v", got)
	}
	// nil
	if extractFormFieldData(nil) != nil {
		t.Error("expected nil for nil input")
	}
	// run without form field
	if extractFormFieldData(&parts.Run{}) != nil {
		t.Error("expected nil for run without form field")
	}
}

func TestFieldInstrKind(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{"PAGE", "PAGE"},
		{"= 2+2", "FORMULA"},
		{"=SUM(ABOVE)", "FORMULA"},
		{"MERGEFIELD Name", "MERGEFIELD"},
		{"EMBED Excel.Sheet", "EMBED"},
		{"LINK Excel.Sheet \"path\"", "LINK"},
		{"UNKNOWN", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := fieldInstrKind(tt.instr)
		if got != tt.want {
			t.Errorf("fieldInstrKind(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestFieldInstrKindExpanded(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{"  DATE  \\@ \"yyyy-MM-dd\"", "DATE"},
		{"TIME \\@ \"HH:mm\"", "TIME"},
		{"CREATEDATE", "CREATEDATE"},
		{"SAVEDATE", "SAVEDATE"},
		{"PRINTDATE", "PRINTDATE"},
		{"TITLE", "TITLE"},
		{"AUTHOR", "AUTHOR"},
		{"SUBJECT", "SUBJECT"},
		{"FILENAME \\p", "FILENAME"},
		{"HYPERLINK \"https://example.com\"", "HYPERLINK"},
		{"NUMCHARS", "NUMCHARS"},
		{"NUMWORDS", "NUMWORDS"},
		{"PAGEREF Bookmark1 \\h", "PAGEREF"},
		{"REF Bookmark1 \\h", "REF"},
		{"SEQ Figure", "SEQ"},
		{"ASK Bookmark \"Prompt\"", "ASK"},
		{"QUOTE \"quoted text\"", "QUOTE"},
		{"AUTOTEXT EntryName", "AUTOTEXT"},
		{"IF 1 = 1 \"yes\" \"no\"", "IF"},
		{"COMPARE A B", "COMPARE"},
		{"TOC \\o \"1-3\"", "TOC"},
		{"INDEX \\e \":\"", "INDEX"},
		{"XE \"term\"", "XE"},
		{"TC \"heading\"", "TC"},
		{"STYLEREF Heading1", "STYLEREF"},
		{"DOCPROPERTY Title", "DOCPROPERTY"},
		{"DOCVARIABLE ProjectID", "DOCVARIABLE"},
		{"FILLIN \"prompt\"", "FILLIN"},
		{"NOTEREF Bookmark", "NOTEREF"},
		{"SECTION", "SECTION"},
		{"SECTIONPAGES", "SECTIONPAGES"},
		{"EDITTIME", "EDITTIME"},
		{"USERADDRESS", "USERADDRESS"},
		{"USERINITIALS", "USERINITIALS"},
		{"MACROBUTTON NoMacro Click", "MACROBUTTON"},
		{"PRIVATE", "PRIVATE"},
		{"ADVANCE \\d 1", "ADVANCE"},
		{"LISTNUM", "LISTNUM"},
		{"INCLUDETEXT \"file.txt\"", "INCLUDETEXT"},
		{"INCLUDEPICTURE \"img.png\"", "INCLUDEPICTURE"},
		{"EQ \\f(,)", "EQ"},
		{"SYMBOL 65", "SYMBOL"},
		{"TA \\l \"1-2\"", "TA"},
		{"TOA \\h", "TOA"},
		{"BIDIOUTLINE", "BIDIOUTLINE"},
		{"   ", ""},
	}
	for _, tt := range tests {
		if got := fieldInstrKind(tt.instr); got != tt.want {
			t.Errorf("fieldInstrKind(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestResolveVMLShape_RectAndOval(t *testing.T) {
	tests := []struct {
		name    string
		content parts.RunContent
		check   func(t *testing.T, shape *Shape)
	}{
		{
			name: "rect",
			content: parts.RunContent{
				Kind:            parts.RunContentVMLShape,
				VMLShapeType:    "rect",
				VMLStyle:        "position:absolute;left:12pt;top:24pt;width:72pt;height:36pt",
				VMLFillColor:    "#ABCDEF",
				VMLStrokeColor:  "#112233",
				VMLStrokeWeight: "1.5pt",
			},
			check: func(t *testing.T, shape *Shape) {
				t.Helper()
				if shape.Kind != "rect" {
					t.Fatalf("kind = %q, want rect", shape.Kind)
				}
				if !shape.Floating || shape.X == 0 || shape.Y == 0 {
					t.Fatalf("expected floating positioned shape, got %+v", shape)
				}
				if shape.FillColor != "ABCDEF" {
					t.Fatalf("fill = %q, want ABCDEF", shape.FillColor)
				}
				if shape.StrokeColor != "112233" {
					t.Fatalf("stroke = %q, want 112233", shape.StrokeColor)
				}
				if shape.Width == 0 || shape.Height == 0 || shape.StrokeWidth == 0 {
					t.Fatalf("expected dimensions and stroke width, got %+v", shape)
				}
				if shape.Geometry != nil {
					t.Fatalf("plain rect should use fallback geometry, got %+v", shape.Geometry)
				}
			},
		},
		{
			name: "oval",
			content: parts.RunContent{
				Kind:           parts.RunContentVMLShape,
				VMLShapeType:   "oval",
				VMLStyle:       "width:30pt;height:20pt",
				VMLFillColor:   "#00FF00",
				VMLStrokeColor: "#0000FF",
			},
			check: func(t *testing.T, shape *Shape) {
				t.Helper()
				if shape.Geometry == nil || shape.Geometry.Name != "ellipse" {
					t.Fatalf("geometry = %+v, want ellipse", shape.Geometry)
				}
				if shape.LineStyle == nil || shape.LineStyle.Color != "0000FF" {
					t.Fatalf("lineStyle = %+v, want stroke color 0000FF", shape.LineStyle)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shape := resolveVMLShape(tt.content)
			if shape == nil {
				t.Fatal("resolveVMLShape returned nil")
			}
			tt.check(t, shape)
		})
	}
}

func TestResolveVMLShape_LineFromEndpoints(t *testing.T) {
	shape := resolveVMLShape(parts.RunContent{
		Kind:            parts.RunContentVMLShape,
		VMLShapeType:    "line",
		VMLFrom:         "0pt,0pt",
		VMLTo:           "0pt,48pt",
		VMLStrokeColor:  "#AA0000",
		VMLStrokeWeight: "2pt",
	})
	if shape == nil {
		t.Fatal("resolveVMLShape returned nil")
	}
	if shape.Kind != "line" {
		t.Fatalf("kind = %q, want line", shape.Kind)
	}
	if shape.Width != 0 || shape.Height == 0 {
		t.Fatalf("expected vertical line dimensions, got width=%d height=%d", shape.Width, shape.Height)
	}
	if shape.StrokeColor != "AA0000" || shape.StrokeWidth == 0 {
		t.Fatalf("unexpected stroke settings: %+v", shape)
	}
}

func TestResolveVMLShape_HiddenShapeReturnsNil(t *testing.T) {
	shape := resolveVMLShape(parts.RunContent{
		Kind:           parts.RunContentVMLShape,
		VMLShapeType:   "rect",
		VMLStyle:       "width:24pt;height:12pt",
		VMLFilledAttr:  "f",
		VMLStrokedAttr: "f",
	})
	if shape != nil {
		t.Fatalf("expected nil for invisible VML shape, got %+v", shape)
	}
}

func TestResolveVMLShape_PolylineFallback(t *testing.T) {
	shape := resolveVMLShape(parts.RunContent{
		Kind:            parts.RunContentVMLShape,
		VMLShapeType:    "polyline",
		VMLStyle:        "position:absolute;left:10pt;top:20pt;width:40pt;height:15pt",
		VMLStrokeColor:  "#445566",
		VMLStrokeWeight: "1pt",
		VMLFilledAttr:   "f",
	})
	if shape == nil {
		t.Fatal("resolveVMLShape returned nil")
	}
	if shape.Kind != "rect" {
		t.Fatalf("kind = %q, want rect fallback", shape.Kind)
	}
	if !shape.Floating || shape.Width == 0 || shape.Height == 0 {
		t.Fatalf("expected positioned polyline fallback, got %+v", shape)
	}
	if shape.StrokeColor != "445566" {
		t.Fatalf("stroke = %q, want 445566", shape.StrokeColor)
	}
}

func TestResolveVMLShape_PolylineGeometry(t *testing.T) {
	shape := resolveVMLShape(parts.RunContent{
		Kind:            parts.RunContentVMLShape,
		VMLShapeType:    "polyline",
		VMLStyle:        "width:50pt;height:30pt",
		VMLPoints:       "0,0,50,0,50,30",
		VMLStrokeColor:  "#445566",
		VMLStrokeWeight: "1pt",
		VMLFilledAttr:   "f",
	})
	if shape == nil {
		t.Fatal("resolveVMLShape returned nil")
	}
	if shape.GeometryData == nil || shape.GeometryData.Type != dml.GeometryTypeCustom {
		t.Fatalf("expected custom geometry, got %+v", shape.GeometryData)
	}
	if len(shape.GeometryData.Paths) != 1 || len(shape.GeometryData.Paths[0].Commands) != 3 {
		t.Fatalf("expected 1 path with 3 commands (M+L+L), got %+v", shape.GeometryData.Paths)
	}
}

func TestResolveVMLShape_CurveGeometry(t *testing.T) {
	shape := resolveVMLShape(parts.RunContent{
		Kind:            parts.RunContentVMLShape,
		VMLShapeType:    "curve",
		VMLStyle:        "width:50pt;height:30pt",
		VMLPoints:       "0,0,10,30,40,30,50,0",
		VMLStrokeColor:  "#445566",
		VMLStrokeWeight: "1pt",
		VMLFilledAttr:   "f",
	})
	if shape == nil {
		t.Fatal("resolveVMLShape returned nil")
	}
	if shape.GeometryData == nil || shape.GeometryData.Type != dml.GeometryTypeCustom {
		t.Fatalf("expected custom geometry, got %+v", shape.GeometryData)
	}
	if len(shape.GeometryData.Paths) != 1 || len(shape.GeometryData.Paths[0].Commands) != 2 {
		t.Fatalf("expected 1 path with 2 commands (M+C), got %+v", shape.GeometryData.Paths)
	}
}

func TestResolveVMLShape_TextPath(t *testing.T) {
	tests := []struct {
		name     string
		on       string
		str      string
		wantNil  bool
		wantText string
	}{
		{"on-true", "t", "Hello", false, "Hello"},
		{"on-1", "1", "World", false, "World"},
		{"on-false", "f", "Ignored", true, ""},
		{"empty-on", "", "Ignored", true, ""},
		{"empty-string", "t", "", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shape := resolveVMLShape(parts.RunContent{
				Kind:              parts.RunContentVMLShape,
				VMLShapeType:      "oval",
				VMLStyle:          "width:30pt;height:20pt",
				VMLFillColor:      "#00FF00",
				VMLTextPathOn:     tt.on,
				VMLTextPathString: tt.str,
			})
			if shape == nil {
				t.Fatal("resolveVMLShape returned nil")
			}
			if tt.wantNil {
				if shape.TextPath != nil {
					t.Fatalf("expected nil textpath, got %+v", shape.TextPath)
				}
				return
			}
			if shape.TextPath == nil {
				t.Fatal("expected textpath, got nil")
			}
			if shape.TextPath.String != tt.wantText {
				t.Fatalf("textpath.String = %q, want %q", shape.TextPath.String, tt.wantText)
			}
		})
	}
}

func TestEffectiveTableLookFlags(t *testing.T) {
	defaults := effectiveTableLookFlags(nil)
	if !defaults.FirstRow || defaults.LastRow || defaults.FirstCol || defaults.LastCol || defaults.NoHBand || !defaults.NoVBand {
		t.Fatalf("default flags = %+v", defaults)
	}

	flags := effectiveTableLookFlags(&parts.TableLook{
		Val:      "07E0",
		FirstRow: "0",
		LastRow:  "1",
		FirstCol: "1",
		LastCol:  "0",
		NoHBand:  "0",
		NoVBand:  "1",
	})
	if flags.FirstRow || !flags.LastRow || !flags.FirstCol || flags.LastCol || flags.NoHBand || !flags.NoVBand {
		t.Fatalf("resolved flags = %+v", flags)
	}
}

func TestResolveConditionalTableStylesAndOverlay(t *testing.T) {
	raw := []*parts.TableStyleProps{
		nil,
		{
			Type: "band1H",
			CellProps: &parts.TableCellProperties{
				Shading: &parts.Shading{Fill: "AAAAAA"},
			},
			RunProps: &parts.RunProperties{
				Italic: &parts.OnOff{},
			},
			ParaProps: &parts.ParagraphProperties{
				Justification: &parts.StringVal{Val: "center"},
			},
		},
		{
			Type: "firstRow",
			CellProps: &parts.TableCellProperties{
				Width:  &parts.TableMeasure{Width: 2400, Type: "dxa"},
				NoWrap: &parts.OnOff{},
			},
			RunProps: &parts.RunProperties{
				Bold:  &parts.OnOff{},
				Color: &parts.RunColor{Val: "112233"},
			},
			ParaProps: &parts.ParagraphProperties{
				ContextualSpacing: &parts.OnOff{},
			},
		},
	}

	if got := resolveConditionalTableStyles(nil, nil); got != nil {
		t.Fatalf("resolveConditionalTableStyles(nil) = %#v, want nil", got)
	}

	resolved := resolveConditionalTableStyles(raw, nil)
	if len(resolved) != 2 {
		t.Fatalf("resolved style count = %d, want 2", len(resolved))
	}
	if resolved[0].Type != "band1H" || resolved[1].Type != "firstRow" {
		t.Fatalf("resolved types = %#v", resolved)
	}

	cell, run, para := buildConditionalOverlay(resolved, []string{"band1H", "firstRow"})
	if cell.Width == nil || *cell.Width != 2400 || cell.WidthType != "dxa" {
		t.Fatalf("cell width overlay = %+v", cell)
	}
	if cell.Shading != "AAAAAA" || !cell.NoWrap {
		t.Fatalf("cell overlay = %+v", cell)
	}
	if run.Bold == nil || !*run.Bold || run.Italic == nil || !*run.Italic || run.Color != "112233" {
		t.Fatalf("run overlay = %+v", run)
	}
	if para.Alignment != "center" || !para.ContextualSpacing {
		t.Fatalf("para overlay = %+v", para)
	}
}

func TestNormalizeContentControl(t *testing.T) {
	if normalizeContentControl(nil) != nil {
		t.Error("expected nil for nil props")
	}
	props := &parts.SdtProperties{
		ID:                        1,
		Tag:                       "tag1",
		Alias:                     "Title",
		Type:                      "docPartObj",
		Checked:                   true,
		DateFormat:                "yyyy-MM-dd",
		DateFullDate:              "2024-01-15T00:00:00Z",
		DateLanguage:              "en-US",
		DateCalendar:              "gregorian",
		DocPartGallery:            "Table of Contents",
		DocPartCategory:           "General",
		DocPartUnique:             true,
		ShowingPlcHdr:             true,
		PlaceholderDocPart:        "DefaultPlaceholder",
		PlaceholderDocPartGallery: "Quick Parts",
		DataXPath:                 "/root/item",
		DataStoreID:               "ds1",
		ListItems:                 []parts.SdtListItem{{DisplayText: "a"}, {DisplayText: "b"}},
	}
	cc := normalizeContentControl(props)
	if cc == nil {
		t.Fatal("expected non-nil ContentControl")
	}
	if cc.ID != 1 || cc.Tag != "tag1" || cc.Title != "Title" || cc.Type != "docPartObj" || !cc.Checked {
		t.Errorf("ContentControl basic fields mismatch: %+v", cc)
	}
	if cc.DateFormat != "yyyy-MM-dd" || cc.DateFullDate != "2024-01-15T00:00:00Z" || cc.DateLanguage != "en-US" || cc.DateCalendar != "gregorian" || cc.DocPartGallery != "Table of Contents" || cc.DocPartCategory != "General" || !cc.DocPartUnique || !cc.ShowingPlaceholder || cc.DataXPath != "/root/item" || cc.DataStoreID != "ds1" {
		t.Errorf("ContentControl extended fields mismatch: %+v", cc)
	}
	if cc.PlaceholderDocPart != "DefaultPlaceholder" || cc.PlaceholderDocPartGallery != "Quick Parts" {
		t.Errorf("ContentControl placeholder fields mismatch: %+v", cc)
	}
	if len(cc.ListItems) != 2 || cc.ListItems[0].DisplayText != "a" {
		t.Errorf("ContentControl ListItems mismatch: %+v", cc.ListItems)
	}
}

func TestNormalizeContentControlCheckbox14(t *testing.T) {
	// Modern w14:checkbox should set Checked from Checked14.
	props := &parts.SdtProperties{Type: "checkBox", Checked14: true}
	cc := normalizeContentControl(props)
	if cc == nil {
		t.Fatal("expected non-nil ContentControl")
	}
	if !cc.Checked {
		t.Errorf("Checked = false, want true (from Checked14)")
	}
	// Legacy checkBox still works.
	props2 := &parts.SdtProperties{Type: "checkBox", Checked: true}
	cc2 := normalizeContentControl(props2)
	if cc2 == nil || !cc2.Checked {
		t.Errorf("legacy Checked = false, want true")
	}
}

func TestNormalizeSdtInlineRuns(t *testing.T) {
	ctx := &resolveCtx{}
	inlines := []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "hello"}}}}},
	}
	runs := normalizeSdtInlineRuns(ctx, inlines, RunFormat{})
	if len(runs) != 1 || runs[0].Text != "hello" {
		t.Errorf("expected 1 run with text 'hello', got %+v", runs)
	}
}

func TestContentControlsEqual(t *testing.T) {
	cc1 := &ContentControl{
		ID:              1,
		Tag:             "t",
		DateFullDate:    "2024-01-15T00:00:00Z",
		DateLanguage:    "en-US",
		DateCalendar:    "gregorian",
		DocPartGallery:  "Table of Contents",
		DocPartCategory: "General",
		DocPartUnique:   true,
	}
	cc2 := &ContentControl{
		ID:              1,
		Tag:             "t",
		DateFullDate:    "2024-01-15T00:00:00Z",
		DateLanguage:    "en-US",
		DateCalendar:    "gregorian",
		DocPartGallery:  "Table of Contents",
		DocPartCategory: "General",
		DocPartUnique:   true,
	}
	cc3 := &ContentControl{ID: 2, Tag: "t"}
	cc4 := &ContentControl{ID: 1, Tag: "t", DocPartGallery: "Quick Parts"}
	cc5 := &ContentControl{ID: 1, Tag: "t", PlaceholderDocPart: "A"}
	cc6 := &ContentControl{ID: 1, Tag: "t", PlaceholderDocPartGallery: "B"}
	if !contentControlsEqual(cc1, cc2) {
		t.Error("expected equal content controls")
	}
	if contentControlsEqual(cc1, cc3) {
		t.Error("expected different content controls")
	}
	if contentControlsEqual(cc1, cc4) {
		t.Error("expected different content controls for docPart metadata")
	}
	if contentControlsEqual(cc1, cc5) {
		t.Error("expected different content controls for placeholder docPart")
	}
	if contentControlsEqual(cc1, cc6) {
		t.Error("expected different content controls for placeholder gallery")
	}
	if !contentControlsEqual(nil, nil) {
		t.Error("expected nil == nil")
	}
	if contentControlsEqual(cc1, nil) {
		t.Error("expected non-nil != nil")
	}
}

func TestNormalizeRuby(t *testing.T) {
	ctx := &resolveCtx{}
	// nil input
	if normalizeRuby(ctx, nil, RunFormat{}) != nil {
		t.Error("expected nil for nil ruby")
	}
	// with base and text runs
	ruby := &parts.Ruby{
		BaseRuns: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "base"}}}}},
		TextRuns: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}},
	}
	run := normalizeRuby(ctx, ruby, RunFormat{})
	if run == nil || run.Kind != "ruby" || run.Ruby == nil {
		t.Fatalf("expected ruby run, got %+v", run)
	}
	if run.Ruby.Base != "base" || run.Ruby.Text != "text" {
		t.Errorf("ruby content mismatch: base=%q text=%q", run.Ruby.Base, run.Ruby.Text)
	}
}

func TestResolveFormField(t *testing.T) {
	// text from ffData
	ff := &parts.FormFieldData{Kind: "text", Name: "f1", DefaultTxt: "hello", MaxLength: 10}
	r := resolveFormField("FORMTEXT", ff, nil, RunFormat{})
	if r.Kind != "formField" || r.FormField == nil || r.FormField.Type != "text" {
		t.Errorf("resolveFormField text = %+v", r)
	}
	// checkbox from keyword fallback
	r = resolveFormField("FORMCHECKBOX", nil, nil, RunFormat{})
	if r.FormField == nil || r.FormField.Type != "checkbox" {
		t.Errorf("resolveFormField checkbox = %+v", r)
	}
	// dropdown from keyword fallback
	r = resolveFormField("FORMDROPDOWN", nil, nil, RunFormat{})
	if r.FormField == nil || r.FormField.Type != "dropdown" {
		t.Errorf("resolveFormField dropdown = %+v", r)
	}
	// text with display runs fallback
	dr := []Run{{Text: "display"}}
	r = resolveFormField("FORMTEXT", nil, dr, RunFormat{})
	if r.FormField.DefaultText != "display" {
		t.Errorf("resolveFormField display fallback = %q, want display", r.FormField.DefaultText)
	}
}

func TestSymToRune(t *testing.T) {
	if got := symToRune("0041"); got != 'A' {
		t.Errorf("symToRune(0041) = %q, want A", got)
	}
	if got := symToRune("F041"); got != 'A' {
		t.Errorf("symToRune(F041) = %q, want A (private-use stripped)", got)
	}
	if got := symToRune(""); got != 0 {
		t.Errorf("symToRune(empty) = %q, want 0", got)
	}
	if got := symToRune("GGGG"); got != 0 {
		t.Errorf("symToRune(GGGG) = %q, want 0", got)
	}
}

func TestApplyWordColorModifier(t *testing.T) {
	// shade darkens
	if got := applyWordColorModifier("FFFFFF", "80", ""); got == "FFFFFF" {
		t.Error("expected shade to darken white")
	}
	// tint lightens
	if got := applyWordColorModifier("000000", "", "80"); got == "000000" {
		t.Error("expected tint to lighten black")
	}
	// invalid hex returns original
	if got := applyWordColorModifier("invalid", "80", ""); got != "invalid" {
		t.Errorf("applyWordColorModifier invalid = %q, want invalid", got)
	}
	// no modifier returns original
	if got := applyWordColorModifier("FF0000", "", ""); got != "FF0000" {
		t.Errorf("applyWordColorModifier no mod = %q, want FF0000", got)
	}
}

func TestResolveBorderSide(t *testing.T) {
	if resolveBorderSide(nil) != nil {
		t.Error("expected nil for nil border")
	}
	if resolveBorderSide(&parts.Border{Val: ""}) != nil {
		t.Error("expected nil for empty-valued border")
	}
	// nil and none now return real BorderSide values so explicit suppression is preserved.
	side := resolveBorderSide(&parts.Border{Val: "nil"})
	if side == nil || side.Val != "nil" || side.SizePt != 0 {
		t.Errorf("expected nil-valued border side, got %+v", side)
	}
	side = resolveBorderSide(&parts.Border{Val: "none"})
	if side == nil || side.Val != "none" || side.SizePt != 0 {
		t.Errorf("expected none border side, got %+v", side)
	}
	b := &parts.Border{Val: "single", Size: 8, Space: 4, Color: "FF0000"}
	side = resolveBorderSide(b)
	if side == nil || side.Val != "single" || side.SizePt != 1.0 || side.SpacePt != 4 || side.Color != "FF0000" {
		t.Errorf("resolveBorderSide = %+v", side)
	}
	// default size
	b = &parts.Border{Val: "single"}
	side = resolveBorderSide(b)
	if side.SizePt != 0.75 {
		t.Errorf("expected default size 0.75, got %f", side.SizePt)
	}
}

func TestResolveParagraphBorders(t *testing.T) {
	if resolveParagraphBorders(nil) != nil {
		t.Error("expected nil for nil input")
	}
	b := &parts.ParagraphBorders{Top: &parts.Border{Val: "single"}}
	out := resolveParagraphBorders(b)
	if out == nil || out.Top == nil || out.Left != nil {
		t.Errorf("resolveParagraphBorders = %+v", out)
	}
	// all nil sides → nil
	b = &parts.ParagraphBorders{}
	if resolveParagraphBorders(b) != nil {
		t.Error("expected nil when all sides are nil")
	}
	// between and bar
	b = &parts.ParagraphBorders{
		Top:     &parts.Border{Val: "single", Size: 4, Space: 1, Color: "000000"},
		Between: &parts.Border{Val: "dashed", Size: 6, Space: 2, Color: "FF0000"},
		Bar:     &parts.Border{Val: "double", Size: 8, Space: 3, Color: "0000FF"},
	}
	out = resolveParagraphBorders(b)
	if out == nil {
		t.Fatal("expected non-nil borders")
	}
	if out.Between == nil || out.Between.Val != "dashed" || out.Between.SizePt != 0.75 || out.Between.Color != "FF0000" {
		t.Errorf("between = %+v", out.Between)
	}
	if out.Bar == nil || out.Bar.Val != "double" || out.Bar.SizePt != 1.0 || out.Bar.Color != "0000FF" {
		t.Errorf("bar = %+v", out.Bar)
	}
	if out.Left != nil || out.Bottom != nil || out.Right != nil {
		t.Errorf("expected left/bottom/right nil, got left=%+v bottom=%+v right=%+v", out.Left, out.Bottom, out.Right)
	}
}

func TestResolveTableBorders(t *testing.T) {
	if resolveTableBorders(nil) != nil {
		t.Error("expected nil for nil input")
	}
	b := &parts.TableBorders{Top: &parts.Border{Val: "single"}}
	out := resolveTableBorders(b)
	if out == nil || out.Top == nil || out.InsideH != nil {
		t.Errorf("resolveTableBorders = %+v", out)
	}
	// all nil sides → nil
	b = &parts.TableBorders{}
	if resolveTableBorders(b) != nil {
		t.Error("expected nil when all sides are nil")
	}
}

func TestResolveCellBorders(t *testing.T) {
	if resolveCellBorders(nil) != nil {
		t.Error("expected nil for nil input")
	}
	b := &parts.TableCellBorders{Top: &parts.Border{Val: "single"}}
	out := resolveCellBorders(b)
	if out == nil || out.Top == nil || out.Left != nil {
		t.Errorf("resolveCellBorders = %+v", out)
	}
}

func TestResolveFrameProps(t *testing.T) {
	if resolveFrameProps(nil) != nil {
		t.Error("expected nil for nil input")
	}
	w, h, x, y, hs, vs, lines := 100, 200, 300, 400, 10, 20, 3
	f := &parts.FramePr{
		W: &w, H: &h, X: &x, Y: &y,
		XAlign: "left", YAlign: "top",
		HAnchor: "page", VAnchor: "page",
		Wrap:   "auto",
		HSpace: &hs, VSpace: &vs,
		DropCap: "drop",
		Lines:   &lines,
	}
	fp := resolveFrameProps(f)
	if fp == nil {
		t.Fatal("expected non-nil FrameProps")
	}
	if fp.W != 100 || fp.H != 200 || fp.X != 300 || fp.Y != 400 {
		t.Errorf("frame geometry mismatch: %+v", fp)
	}
	if fp.XAlign != "left" || fp.YAlign != "top" || fp.HAnchor != "page" || fp.VAnchor != "page" {
		t.Errorf("frame alignment mismatch: %+v", fp)
	}
	if fp.WrapType != "auto" || fp.HSpace != 10 || fp.VSpace != 20 || fp.DropCap != "drop" || fp.Lines != 3 {
		t.Errorf("frame misc mismatch: %+v", fp)
	}
}

func TestNormalizeBodyBlocks(t *testing.T) {
	ctx := &resolveCtx{}
	pkg := &parts.Package{}
	blocks := []parts.BodyBlock{
		{Kind: parts.BodyBlockParagraph, Paragraph: &parts.Paragraph{Runs: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "hello"}}}}}}},
	}
	out := normalizeBodyBlocks(ctx, pkg, blocks)
	if len(out) != 1 || out[0].Kind != "paragraph" || out[0].Paragraph == nil || out[0].Paragraph.Text != "hello" {
		t.Errorf("normalizeBodyBlocks = %+v", out)
	}
	if out[0].Index != 0 {
		t.Errorf("expected block index 0, got %d", out[0].Index)
	}
	// empty blocks
	if normalizeBodyBlocks(ctx, pkg, nil) != nil {
		t.Error("expected nil for empty blocks")
	}
}

func TestResolveThemeFontSlot(t *testing.T) {
	tests := []struct {
		slot  string
		fonts map[string]string
		want  string
	}{
		{"minorHAnsi", map[string]string{"minor": "Calibri", "major": "Cambria"}, "Calibri"},
		{"majorAscii", map[string]string{"minor": "Calibri", "major": "Cambria"}, "Cambria"},
		{"minorBidi", map[string]string{"minor": "Arial", "major": "Times"}, "Arial"},
		{"minorEastAsia", map[string]string{"minor": "Calibri", "minorEA": "MS Mincho"}, "MS Mincho"},
		{"majorBidi", map[string]string{"major": "Cambria", "majorCS": "Arial"}, "Arial"},
		{"other", map[string]string{"minor": "Calibri", "major": "Cambria"}, ""},
		{"minorHAnsi", nil, ""},
	}
	for _, tt := range tests {
		got := resolveThemeFontSlot(tt.fonts, tt.slot)
		if got != tt.want {
			t.Errorf("resolveThemeFontSlot(%q) = %q, want %q", tt.slot, got, tt.want)
		}
	}
}

func TestResolveThemeFontSlotWithLangUsesSupplementalScriptFallback(t *testing.T) {
	fonts := map[string]string{
		"major":   "Cambria",
		"majorEA": "Doc Default EA",
		"majorCS": "Doc Default CS",
		supplementalThemeFontKey("major", "Jpan"): "MS Gothic",
		supplementalThemeFontKey("major", "Arab"): "Traditional Arabic",
	}
	lang := &parts.RunLang{
		EastAsia: "ja-JP",
		Bidi:     "ar-SA",
	}
	if got := resolveThemeFontSlotWithLang(fonts, "majorEastAsia", lang); got != "MS Gothic" {
		t.Fatalf("resolveThemeFontSlotWithLang(majorEastAsia) = %q, want MS Gothic", got)
	}
	if got := resolveThemeFontSlotWithLang(fonts, "majorBidi", lang); got != "Traditional Arabic" {
		t.Fatalf("resolveThemeFontSlotWithLang(majorBidi) = %q, want Traditional Arabic", got)
	}
	if got := resolveThemeFontSlotWithLang(fonts, "majorAscii", lang); got != "Cambria" {
		t.Fatalf("resolveThemeFontSlotWithLang(majorAscii) = %q, want Cambria", got)
	}
}

func TestResolveThemeFontSlotWithLangFallsBackToDocumentDefaults(t *testing.T) {
	fonts := map[string]string{
		"minor":   "Calibri",
		"minorEA": "Doc Default EA",
		"minorCS": "Doc Default CS",
	}
	lang := &parts.RunLang{
		EastAsia: "ko-KR",
		Bidi:     "he-IL",
	}
	if got := resolveThemeFontSlotWithLang(fonts, "minorEastAsia", lang); got != "Doc Default EA" {
		t.Fatalf("resolveThemeFontSlotWithLang(minorEastAsia) = %q, want Doc Default EA", got)
	}
	if got := resolveThemeFontSlotWithLang(fonts, "minorBidi", lang); got != "Doc Default CS" {
		t.Fatalf("resolveThemeFontSlotWithLang(minorBidi) = %q, want Doc Default CS", got)
	}
}

func TestBuildThemeFontIndexUsesSupplementalThemeFonts(t *testing.T) {
	theme := &parts.Theme{
		Elements: &parts.ThemeElements{
			FontScheme: &parts.FontScheme{
				MajorFont: &parts.ThemeFontSet{
					Latin: parts.ThemeLatin{Typeface: "Cambria"},
					Fonts: []*parts.ThemeSupplementalFont{
						{Script: "Jpan", Typeface: "MS Gothic"},
						{Script: "Arab", Typeface: "Traditional Arabic"},
					},
				},
				MinorFont: &parts.ThemeFontSet{
					Latin: parts.ThemeLatin{Typeface: "Calibri"},
					Fonts: []*parts.ThemeSupplementalFont{
						{Script: "Hans", Typeface: "DengXian"},
						{Script: "Hebr", Typeface: "David"},
					},
				},
			},
		},
	}
	fonts := buildThemeFontIndex(theme, &parts.ThemeFontLang{
		EastAsia: "ja-JP",
		Bidi:     "ar-SA",
	})
	if fonts["majorEA"] != "MS Gothic" {
		t.Fatalf("majorEA = %q, want MS Gothic", fonts["majorEA"])
	}
	if fonts["majorEastAsia"] != "MS Gothic" {
		t.Fatalf("majorEastAsia = %q, want MS Gothic", fonts["majorEastAsia"])
	}
	if fonts["majorCS"] != "Traditional Arabic" {
		t.Fatalf("majorCS = %q, want Traditional Arabic", fonts["majorCS"])
	}
	if fonts["majorComplexScript"] != "Traditional Arabic" {
		t.Fatalf("majorComplexScript = %q, want Traditional Arabic", fonts["majorComplexScript"])
	}

	fonts = buildThemeFontIndex(theme, &parts.ThemeFontLang{
		EastAsia: "zh-CN",
		Bidi:     "he-IL",
	})
	if fonts["minorEA"] != "DengXian" {
		t.Fatalf("minorEA = %q, want DengXian", fonts["minorEA"])
	}
	if fonts["minorCS"] != "David" {
		t.Fatalf("minorCS = %q, want David", fonts["minorCS"])
	}
}

func TestBuildThemeFontIndexPrefersExplicitThemeFaces(t *testing.T) {
	theme := &parts.Theme{
		Elements: &parts.ThemeElements{
			FontScheme: &parts.FontScheme{
				MajorFont: &parts.ThemeFontSet{
					Latin:         parts.ThemeLatin{Typeface: "Cambria"},
					EastAsia:      []*parts.ThemeTypeface{{Typeface: "Explicit EA"}},
					ComplexScript: []*parts.ThemeTypeface{{Typeface: "Explicit CS"}},
					Fonts: []*parts.ThemeSupplementalFont{
						{Script: "Jpan", Typeface: "MS Gothic"},
						{Script: "Arab", Typeface: "Traditional Arabic"},
					},
				},
			},
		},
	}
	fonts := buildThemeFontIndex(theme, &parts.ThemeFontLang{
		EastAsia: "ja-JP",
		Bidi:     "ar-SA",
	})
	if fonts["majorEA"] != "Explicit EA" {
		t.Fatalf("majorEA = %q, want Explicit EA", fonts["majorEA"])
	}
	if fonts["majorCS"] != "Explicit CS" {
		t.Fatalf("majorCS = %q, want Explicit CS", fonts["majorCS"])
	}
}

func TestResolveFontSlot(t *testing.T) {
	ctx := &resolveCtx{themeFonts: map[string]string{
		"minor":   "Calibri",
		"major":   "Cambria",
		"minorEA": "MS Mincho",
		"majorCS": "Arial",
	}}
	if got := resolveFontSlot(ctx, "minorHAnsi"); got != "Calibri" {
		t.Errorf("resolveFontSlot(minorHAnsi) = %q, want Calibri", got)
	}
	if got := resolveFontSlot(ctx, "majorAscii"); got != "Cambria" {
		t.Errorf("resolveFontSlot(majorAscii) = %q, want Cambria", got)
	}
	if got := resolveFontSlot(ctx, "minorEastAsia"); got != "MS Mincho" {
		t.Errorf("resolveFontSlot(minorEastAsia) = %q, want MS Mincho", got)
	}
	if got := resolveFontSlot(ctx, "majorBidi"); got != "Arial" {
		t.Errorf("resolveFontSlot(majorBidi) = %q, want Arial", got)
	}
	if got := resolveFontSlot(ctx, "other"); got != "" {
		t.Errorf("resolveFontSlot(other) = %q, want empty", got)
	}
	if got := resolveFontSlot(&resolveCtx{}, "minorHAnsi"); got != "" {
		t.Errorf("resolveFontSlot(nil themeFonts) = %q, want empty", got)
	}
}

func TestResolveDrawingMLFontUsesLanguageSpecificSupplementalFonts(t *testing.T) {
	fonts := map[string]string{
		"major":   "Cambria",
		"majorEA": "Doc Default EA",
		"majorCS": "Doc Default CS",
		supplementalThemeFontKey("major", "Jpan"): "MS Gothic",
		supplementalThemeFontKey("major", "Arab"): "Traditional Arabic",
	}
	if got := resolveDrawingMLFont(fonts, "+mj-ea", "ja-JP"); got != "MS Gothic" {
		t.Fatalf("resolveDrawingMLFont(+mj-ea) = %q, want MS Gothic", got)
	}
	if got := resolveDrawingMLFont(fonts, "+mj-cs", "ar-SA"); got != "Traditional Arabic" {
		t.Fatalf("resolveDrawingMLFont(+mj-cs) = %q, want Traditional Arabic", got)
	}
	if got := resolveDrawingMLFont(fonts, "+mj-lt", "ja-JP"); got != "Cambria" {
		t.Fatalf("resolveDrawingMLFont(+mj-lt) = %q, want Cambria", got)
	}
}

func TestFindChartRef(t *testing.T) {
	if got := findChartRef(nil); got != "" {
		t.Errorf("findChartRef(nil) = %q, want empty", got)
	}
	if got := findChartRef(&parts.Graphic{}); got != "" {
		t.Errorf("findChartRef(empty) = %q, want empty", got)
	}
	g := &parts.Graphic{GraphicData: &parts.GraphicData{Chart: &parts.ChartRef{ID: "rId5"}}}
	if got := findChartRef(g); got != "rId5" {
		t.Errorf("findChartRef(chart) = %q, want rId5", got)
	}
}

func TestRunFieldContent(t *testing.T) {
	// nil run
	ft, it := runFieldContent(nil)
	if ft != "" || it != "" {
		t.Errorf("runFieldContent(nil) = (%q, %q), want (\"\", \"\")", ft, it)
	}
	// normal text run
	ft, it = runFieldContent(&parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "hello"}}}})
	if ft != "" || it != "" {
		t.Errorf("runFieldContent(text) = (%q, %q), want (\"\", \"\")", ft, it)
	}
	// fldChar begin
	ft, it = runFieldContent(&parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}})
	if ft != "begin" || it != "" {
		t.Errorf("runFieldContent(begin) = (%q, %q), want (\"begin\", \"\")", ft, it)
	}
	// fldChar separate
	ft, it = runFieldContent(&parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}})
	if ft != "separate" || it != "" {
		t.Errorf("runFieldContent(separate) = (%q, %q), want (\"separate\", \"\")", ft, it)
	}
	// fldChar end
	ft, it = runFieldContent(&parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "end"}}}})
	if ft != "end" || it != "" {
		t.Errorf("runFieldContent(end) = (%q, %q), want (\"end\", \"\")", ft, it)
	}
	// instrText
	ft, it = runFieldContent(&parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " PAGE "}}})
	if ft != "instr" || it != " PAGE " {
		t.Errorf("runFieldContent(instr) = (%q, %q), want (\"instr\", \" PAGE \")", ft, it)
	}
	// mixed: instrText after fldChar (fldChar wins)
	ft, it = runFieldContent(&parts.Run{Contents: []parts.RunContent{
		{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}},
		{Kind: parts.RunContentInstrText, InstrText: " PAGE "},
	}})
	if ft != "begin" || it != "" {
		t.Errorf("runFieldContent(mixed) = (%q, %q), want (\"begin\", \"\")", ft, it)
	}
	// empty fldChar
	ft, it = runFieldContent(&parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: nil}}})
	if ft != "" || it != "" {
		t.Errorf("runFieldContent(empty fldChar) = (%q, %q), want (\"\", \"\")", ft, it)
	}
}

func TestParseFieldFillinDefault(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{`FILLIN "Prompt" "default"`, "default"},
		{`FILLIN "Prompt"`, ""},
		{`FILLIN "Prompt" \d "default"`, ""}, // switch is skipped, returns empty
		{`FILLIN`, ""},
		{`FILLIN Prompt default`, "default"},
		{`FILLIN Prompt`, ""},
	}
	for _, tt := range tests {
		got := parseFieldFillinDefault(tt.instr)
		if got != tt.want {
			t.Errorf("parseFieldFillinDefault(%q) = %q, want %q", tt.instr, got, tt.want)
		}
	}
}

func TestClassifyListKind(t *testing.T) {
	tests := []struct {
		format   string
		template string
		want     string
	}{
		{"bullet", "", "bullet"},
		{"decimal", "", "ordered"},
		{"upperRoman", "", "ordered"},
		{"lowerRoman", "", "ordered"},
		{"upperLetter", "", "ordered"},
		{"lowerLetter", "", "ordered"},
		{"ordinal", "", "ordered"},
		{"cardinalText", "", "ordered"},
		{"ordinalText", "", "ordered"},
		{"hex", "", "ordered"},
		{"", "%1.", "ordered"},
		{"", "", "bullet"},
		{"unknown", "", "bullet"},
	}
	for _, tt := range tests {
		got := classifyListKind(tt.format, tt.template)
		if got != tt.want {
			t.Errorf("classifyListKind(%q, %q) = %q, want %q", tt.format, tt.template, got, tt.want)
		}
	}
}

func TestFieldsEqual(t *testing.T) {
	if !fieldsEqual(nil, nil) {
		t.Error("fieldsEqual(nil, nil) = false, want true")
	}
	if !fieldsEqual([]Field{}, []Field{}) {
		t.Error("fieldsEqual(empty, empty) = false, want true")
	}
	a := []Field{{Kind: "PAGE", Instr: "PAGE", Arg: "", Anchor: "", Target: ""}}
	b := []Field{{Kind: "PAGE", Instr: "PAGE", Arg: "", Anchor: "", Target: ""}}
	if !fieldsEqual(a, b) {
		t.Error("fieldsEqual(equal) = false, want true")
	}
	c := []Field{{Kind: "REF", Instr: "PAGE"}}
	if fieldsEqual(a, c) {
		t.Error("fieldsEqual(different) = true, want false")
	}
	if fieldsEqual(a, []Field{}) {
		t.Error("fieldsEqual(different len) = true, want false")
	}
}

func TestDrawingAnchorWrap(t *testing.T) {
	if wt, ws := drawingAnchorWrap(nil); wt != "" || ws != "" {
		t.Errorf("drawingAnchorWrap(nil) = (%q, %q), want (\"\", \"\")", wt, ws)
	}
	wt, ws := drawingAnchorWrap(&parts.DrawingAnchor{WrapNone: &parts.DrawingWrapEmpty{}})
	if wt != "none" || ws != "" {
		t.Errorf("drawingAnchorWrap(none) = (%q, %q), want (\"none\", \"\")", wt, ws)
	}
	wt, ws = drawingAnchorWrap(&parts.DrawingAnchor{WrapSquare: &parts.DrawingWrapSided{WrapText: "bothSides"}})
	if wt != "square" || ws != "bothSides" {
		t.Errorf("drawingAnchorWrap(square) = (%q, %q), want (\"square\", \"bothSides\")", wt, ws)
	}
	wt, ws = drawingAnchorWrap(&parts.DrawingAnchor{WrapTight: &parts.DrawingWrapSided{WrapText: "left"}})
	if wt != "tight" || ws != "left" {
		t.Errorf("drawingAnchorWrap(tight) = (%q, %q), want (\"tight\", \"left\")", wt, ws)
	}
	wt, ws = drawingAnchorWrap(&parts.DrawingAnchor{WrapThrough: &parts.DrawingWrapSided{WrapText: "right"}})
	if wt != "through" || ws != "right" {
		t.Errorf("drawingAnchorWrap(through) = (%q, %q), want (\"through\", \"right\")", wt, ws)
	}
	wt, ws = drawingAnchorWrap(&parts.DrawingAnchor{WrapTopAndBottom: &parts.DrawingWrapEmpty{}})
	if wt != "topAndBottom" || ws != "" {
		t.Errorf("drawingAnchorWrap(topAndBottom) = (%q, %q), want (\"topAndBottom\", \"\")", wt, ws)
	}
}

func TestDrawingAnchorPolygon(t *testing.T) {
	if got := drawingAnchorPolygon(nil); got != nil {
		t.Errorf("drawingAnchorPolygon(nil) = %v, want nil", got)
	}
	if got := drawingAnchorPolygon(&parts.DrawingAnchor{}); got != nil {
		t.Errorf("drawingAnchorPolygon(empty) = %v, want nil", got)
	}
	anchor := &parts.DrawingAnchor{
		WrapTight: &parts.DrawingWrapSided{
			Polygon: &parts.WrapPolygon{
				Start:  parts.WrapPolygonPt{X: 0, Y: 0},
				LineTo: []parts.WrapPolygonPt{{X: 1000000, Y: 0}, {X: 1000000, Y: 1000000}},
			},
		},
	}
	got := drawingAnchorPolygon(anchor)
	if len(got) != 3 {
		t.Fatalf("expected 3 points, got %d", len(got))
	}
	if got[0][0] != 0 || got[0][1] != 0 {
		t.Errorf("first point = %v, want [0 0]", got[0])
	}
	// 1000000 EMU = 10 points (1 EMU = 1/914400 inch, 1000000/914400*72 ≈ 78.74...)
	// we just check it's non-zero
	if got[1][0] == 0 {
		t.Error("expected second point x > 0")
	}
	// through path
	anchor2 := &parts.DrawingAnchor{
		WrapThrough: &parts.DrawingWrapSided{
			Polygon: &parts.WrapPolygon{
				Start:  parts.WrapPolygonPt{X: 0, Y: 0},
				LineTo: []parts.WrapPolygonPt{{X: 500000, Y: 500000}},
			},
		},
	}
	got2 := drawingAnchorPolygon(anchor2)
	if len(got2) != 2 {
		t.Fatalf("expected 2 points, got %d", len(got2))
	}
}

func TestResolveDrawingFill(t *testing.T) {
	ctx := &resolveCtx{themeColors: map[string]string{"accent1": "4472C4"}}
	// nil fill
	if got := resolveDrawingFill(ctx, nil); got != "" {
		t.Errorf("resolveDrawingFill(nil) = %q, want empty", got)
	}
	// solid fill with color
	if got := resolveDrawingFill(ctx, dml.SolidFill{Color: "FF0000"}); got != "FF0000" {
		t.Errorf("resolveDrawingFill(solid color) = %q, want FF0000", got)
	}
	// solid fill with theme color
	if got := resolveDrawingFill(ctx, dml.SolidFill{ThemeColor: "accent1"}); got != "4472C4" {
		t.Errorf("resolveDrawingFill(theme) = %q, want 4472C4", got)
	}
	// no fill
	if got := resolveDrawingFill(ctx, dml.NoFill{}); got != "" {
		t.Errorf("resolveDrawingFill(noFill) = %q, want empty", got)
	}
	// gradient fill
	if got := resolveDrawingFill(ctx, dml.GradientFill{Stops: []dml.GradientStop{{Color: "00FF00"}}}); got != "00FF00" {
		t.Errorf("resolveDrawingFill(gradient) = %q, want 00FF00", got)
	}
	// gradient fill empty
	if got := resolveDrawingFill(ctx, dml.GradientFill{}); got != "" {
		t.Errorf("resolveDrawingFill(gradient empty) = %q, want empty", got)
	}
	// pattern fill (not supported, falls through to default)
	if got := resolveDrawingFill(ctx, dml.PatternFill{Foreground: "FF0000"}); got != "" {
		t.Errorf("resolveDrawingFill(pattern) = %q, want empty", got)
	}
	// pointer variants
	if got := resolveDrawingFill(ctx, &dml.SolidFill{Color: "FF0000"}); got != "FF0000" {
		t.Errorf("resolveDrawingFill(*solid) = %q, want FF0000", got)
	}
	if got := resolveDrawingFill(ctx, &dml.NoFill{}); got != "" {
		t.Errorf("resolveDrawingFill(*noFill) = %q, want empty", got)
	}
	if got := resolveDrawingFill(ctx, &dml.GradientFill{Stops: []dml.GradientStop{{Color: "00FF00"}}}); got != "00FF00" {
		t.Errorf("resolveDrawingFill(*gradient) = %q, want 00FF00", got)
	}
}

func TestResolveDrawingSolidFill(t *testing.T) {
	ctx := &resolveCtx{themeColors: map[string]string{"accent1": "4472C4"}}
	// direct color
	if got := resolveDrawingSolidFill(ctx, dml.SolidFill{Color: "FF0000"}); got != "FF0000" {
		t.Errorf("resolveDrawingSolidFill(color) = %q, want FF0000", got)
	}
	// theme color
	if got := resolveDrawingSolidFill(ctx, dml.SolidFill{ThemeColor: "accent1"}); got != "4472C4" {
		t.Errorf("resolveDrawingSolidFill(theme) = %q, want 4472C4", got)
	}
	// missing theme color
	if got := resolveDrawingSolidFill(ctx, dml.SolidFill{ThemeColor: "missing"}); got != "" {
		t.Errorf("resolveDrawingSolidFill(missing theme) = %q, want empty", got)
	}
	// theme with shade/tint
	got := resolveDrawingSolidFill(ctx, dml.SolidFill{ThemeColor: "accent1", ThemeShade: "80", ThemeTint: ""})
	if got == "" || got == "4472C4" {
		t.Errorf("resolveDrawingSolidFill(shade) = %q, expected modified color", got)
	}
	// empty
	if got := resolveDrawingSolidFill(ctx, dml.SolidFill{}); got != "" {
		t.Errorf("resolveDrawingSolidFill(empty) = %q, want empty", got)
	}
}

func TestDrawingSolidFillFromParts(t *testing.T) {
	// nil
	if got := drawingSolidFillFromParts(nil, nil); got != nil {
		t.Errorf("drawingSolidFillFromParts(nil) = %v, want nil", got)
	}
	// sRGB
	got := drawingSolidFillFromParts(&parts.DrawingSolidFill{SRGB: &parts.ThemeSRGBColor{Val: "FF0000"}}, nil)
	if got == nil || got.Color != "FF0000" {
		t.Errorf("drawingSolidFillFromParts(sRGB) = %+v, want Color=FF0000", got)
	}
	got = drawingSolidFillFromParts(&parts.DrawingSolidFill{
		SRGB: &parts.ThemeSRGBColor{
			Val: "FF0000",
			ThemeColorTransforms: parts.ThemeColorTransforms{
				AlphaMod: &parts.DrawingColorTransform{Val: 50000},
				Shade:    &parts.DrawingColorTransform{Val: 25000},
				Tint:     &parts.DrawingColorTransform{Val: 12500},
				RedOff:   &parts.DrawingColorTransform{Val: 10000},
				GreenMod: &parts.DrawingColorTransform{Val: 50000},
				Blue:     &parts.DrawingColorTransform{Val: 25000},
				Hue:      &parts.DrawingColorTransform{Val: 7200000},
				Sat:      &parts.DrawingColorTransform{Val: 25000},
				Lum:      &parts.DrawingColorTransform{Val: 75000},
				Gray:     &struct{}{},
				Comp:     &struct{}{},
				Inv:      &struct{}{},
				Gamma:    &struct{}{},
				InvGamma: &struct{}{},
			},
		},
	}, nil)
	if got == nil || got.Color == "FF0000" || !got.Gray || got.AlphaMod != 0.5 || got.Shade != 0.25 || !got.HasShade || got.Tint != 0.125 || !got.HasTint || got.RedOff != 0.1 || !got.HasRedOff || got.GreenMod != 0.5 || !got.HasGreenMod || got.Blue != 0.25 || !got.HasBlue || got.Hue != 120 || !got.HasHue || got.Sat != 0.25 || !got.HasSat || got.Lum != 0.75 || !got.HasLum || !got.Comp || !got.Inv || !got.Gamma || !got.InvGamma {
		t.Errorf("drawingSolidFillFromParts(sRGB transforms) = %+v", got)
	}
	// scheme color
	got = drawingSolidFillFromParts(&parts.DrawingSolidFill{
		SchemeClr: &parts.DrawingSchemeClr{Val: "accent1", LumMod: &parts.DrawingColorTransform{Val: 50000}, LumOff: &parts.DrawingColorTransform{Val: 25000}},
	}, map[string]string{"accent1": "4472C4"})
	if got == nil || got.ThemeColor != "accent1" || got.LumMod != 0.5 || got.LumOff != 0.25 {
		t.Errorf("drawingSolidFillFromParts(scheme) = %+v", got)
	}
	got = drawingSolidFillFromParts(&parts.DrawingSolidFill{
		SchemeClr: &parts.DrawingSchemeClr{
			Val:      "accent1",
			AlphaOff: &parts.DrawingColorTransform{Val: -25000},
			Shade:    &parts.DrawingColorTransform{Val: 0},
			Tint:     &parts.DrawingColorTransform{Val: 40000},
			Red:      &parts.DrawingColorTransform{Val: 100000},
			GreenOff: &parts.DrawingColorTransform{Val: 20000},
			BlueMod:  &parts.DrawingColorTransform{Val: 50000},
			Hue:      &parts.DrawingColorTransform{Val: 1800000},
			Sat:      &parts.DrawingColorTransform{Val: 50000},
			Lum:      &parts.DrawingColorTransform{Val: 25000},
			SatMod:   &parts.DrawingColorTransform{Val: 50000},
			HueOff:   &parts.DrawingColorTransform{Val: 1800000},
			Gray:     &struct{}{},
			Comp:     &struct{}{},
			Inv:      &struct{}{},
			Gamma:    &struct{}{},
			InvGamma: &struct{}{},
		},
	}, map[string]string{"accent1": "4472C4"})
	if got == nil || got.ThemeColor != "accent1" || got.AlphaOff != -0.25 || got.Shade != 0 || !got.HasShade || got.Tint != 0.4 || !got.HasTint || got.Red != 1 || !got.HasRed || got.GreenOff != 0.2 || !got.HasGreenOff || got.BlueMod != 0.5 || !got.HasBlueMod || got.Hue != 30 || !got.HasHue || got.Sat != 0.5 || !got.HasSat || got.Lum != 0.25 || !got.HasLum || got.SatMod != 0.5 || got.HueOff != 30 || !got.Gray || !got.Comp || !got.Inv || !got.Gamma || !got.InvGamma || got.Color == "" {
		t.Errorf("drawingSolidFillFromParts(scheme transforms) = %+v", got)
	}
	// missing scheme val
	got = drawingSolidFillFromParts(&parts.DrawingSolidFill{SchemeClr: &parts.DrawingSchemeClr{}}, nil)
	if got != nil {
		t.Errorf("drawingSolidFillFromParts(empty scheme) = %v, want nil", got)
	}
	// no color info
	got = drawingSolidFillFromParts(&parts.DrawingSolidFill{}, nil)
	if got != nil {
		t.Errorf("drawingSolidFillFromParts(empty) = %v, want nil", got)
	}
}

func TestResolveEmbeddedFonts(t *testing.T) {
	// nil/empty slice
	if got := resolveEmbeddedFonts(nil); len(got) != 0 {
		t.Errorf("resolveEmbeddedFonts(nil) = %d items, want 0", len(got))
	}
	// nil item
	if got := resolveEmbeddedFonts([]*parts.EmbeddedFontData{nil}); len(got) != 0 {
		t.Errorf("resolveEmbeddedFonts([nil]) = %d items, want 0", len(got))
	}
	// empty typeface
	if got := resolveEmbeddedFonts([]*parts.EmbeddedFontData{{Typeface: "   "}}); len(got) != 0 {
		t.Errorf("resolveEmbeddedFonts(empty typeface) = %d items, want 0", len(got))
	}
	// font with no faces
	if got := resolveEmbeddedFonts([]*parts.EmbeddedFontData{{Typeface: "Arial"}}); len(got) != 0 {
		t.Errorf("resolveEmbeddedFonts(no faces) = %d items, want 0", len(got))
	}
	// font with regular face
	fonts := []*parts.EmbeddedFontData{{
		Typeface: "TestFont",
		Regular:  &parts.FontPart{ContentType: "font/ttf", Data: []byte("data")},
	}}
	got := resolveEmbeddedFonts(fonts)
	if len(got) != 1 || got[0].Family != "TestFont" || len(got[0].Faces) != 1 {
		t.Fatalf("expected 1 font with 1 face, got %+v", got)
	}
	if got[0].Faces[0].Weight != 400 || got[0].Faces[0].Style != "normal" {
		t.Errorf("face = %+v", got[0].Faces[0])
	}
	// font with all faces
	fonts = []*parts.EmbeddedFontData{{
		Typeface:   "TestFont",
		Regular:    &parts.FontPart{ContentType: "font/ttf", Data: []byte("r")},
		Bold:       &parts.FontPart{ContentType: "font/ttf", Data: []byte("b")},
		Italic:     &parts.FontPart{ContentType: "font/ttf", Data: []byte("i")},
		BoldItalic: &parts.FontPart{ContentType: "font/ttf", Data: []byte("bi")},
	}}
	got = resolveEmbeddedFonts(fonts)
	if len(got) != 1 || len(got[0].Faces) != 4 {
		t.Fatalf("expected 4 faces, got %d", len(got[0].Faces))
	}
	// empty data parts are skipped
	fonts = []*parts.EmbeddedFontData{{
		Typeface: "TestFont",
		Regular:  &parts.FontPart{ContentType: "font/ttf", Data: []byte{}},
		Bold:     &parts.FontPart{ContentType: "font/ttf", Data: []byte("b")},
	}}
	got = resolveEmbeddedFonts(fonts)
	if len(got) != 1 || len(got[0].Faces) != 1 {
		t.Fatalf("expected 1 face (regular skipped), got %d", len(got[0].Faces))
	}
}

func TestEmitField(t *testing.T) {
	ctx := &resolveCtx{}
	fmt := RunFormat{Bold: boolPtr(true)}

	// HYPERLINK
	out := Paragraph{}
	emitField(ctx, &out, `HYPERLINK "https://example.com"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "link"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].Kind != "hyperlink" || out.Runs[0].Target != "https://example.com" {
		t.Errorf("emitField HYPERLINK = %+v", out.Runs)
	}

	// HYPERLINK with anchor only
	out = Paragraph{}
	emitField(ctx, &out, `HYPERLINK \l "bookmark"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "link"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].Target != "#bookmark" {
		t.Errorf("emitField HYPERLINK anchor = %+v", out.Runs)
	}

	// REF
	out = Paragraph{}
	emitField(ctx, &out, `REF BookmarkName`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].Anchor != "BookmarkName" || out.Runs[0].Target != "#BookmarkName" {
		t.Errorf("emitField REF = %+v", out.Runs)
	}

	// PAGEREF
	out = Paragraph{}
	emitField(ctx, &out, `PAGEREF BookmarkName`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "PAGEREF" || out.Runs[0].Anchor != "BookmarkName" {
		t.Errorf("emitField PAGEREF = %+v", out.Runs)
	}

	// DOCPROPERTY
	out = Paragraph{}
	emitField(ctx, &out, `DOCPROPERTY "Title"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "DOCPROPERTY" || out.Runs[0].FieldArg != "Title" {
		t.Errorf("emitField DOCPROPERTY = %+v", out.Runs)
	}

	// DOCVARIABLE
	out = Paragraph{}
	emitField(ctx, &out, `DOCVARIABLE "ProjectID"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "DOCVARIABLE" || out.Runs[0].FieldArg != "ProjectID" {
		t.Errorf("emitField DOCVARIABLE = %+v", out.Runs)
	}

	// STYLEREF
	out = Paragraph{}
	emitField(ctx, &out, `STYLEREF "Heading 1"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "STYLEREF" || out.Runs[0].FieldArg != "Heading 1" {
		t.Errorf("emitField STYLEREF = %+v", out.Runs)
	}

	// IF
	out = Paragraph{}
	emitField(ctx, &out, `IF 1 = 1 "yes" "no"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "IF" {
		t.Errorf("emitField IF = %+v", out.Runs)
	}

	// FILLIN
	out = Paragraph{}
	emitField(ctx, &out, `FILLIN "Enter name:"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "FILLIN" {
		t.Errorf("emitField FILLIN = %+v", out.Runs)
	}

	// SEQ
	out = Paragraph{}
	emitField(ctx, &out, `SEQ Figure`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "SEQ" || out.Runs[0].FieldArg != "Figure" {
		t.Errorf("emitField SEQ = %+v", out.Runs)
	}

	// ASK
	out = Paragraph{}
	emitField(ctx, &out, `ASK Bookmark "Prompt"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "ASK" {
		t.Errorf("emitField ASK = %+v", out.Runs)
	}

	// MERGEFIELD
	out = Paragraph{}
	emitField(ctx, &out, `MERGEFIELD Name`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "MERGEFIELD" || out.Runs[0].FieldArg != "Name" {
		t.Errorf("emitField MERGEFIELD = %+v", out.Runs)
	}

	// NEXTIF without cached display runs must still survive as a semantic run.
	out = Paragraph{}
	emitField(ctx, &out, `NEXTIF City = "Chicago"`, nil, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "NEXTIF" || out.Runs[0].FieldInstr != `NEXTIF City = "Chicago"` {
		t.Errorf("emitField NEXTIF without display runs = %+v", out.Runs)
	}

	// AUTOTEXT
	out = Paragraph{}
	emitField(ctx, &out, `AUTOTEXT AutoTextName`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "AUTOTEXT" || out.Runs[0].FieldArg != "AutoTextName" {
		t.Errorf("emitField AUTOTEXT = %+v", out.Runs)
	}

	// TOC
	out = Paragraph{}
	emitField(ctx, &out, `TOC \h \z`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "entry1"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "TOC" || out.Runs[0].Text != "entry1" {
		t.Errorf("emitField TOC = %+v", out.Runs)
	}

	// INDEX
	out = Paragraph{}
	emitField(ctx, &out, `INDEX \c "2"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "entry"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "INDEX" {
		t.Errorf("emitField INDEX = %+v", out.Runs)
	}

	// XE
	out = Paragraph{}
	emitField(ctx, &out, `XE "term"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "entry"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "XE" {
		t.Errorf("emitField XE = %+v", out.Runs)
	}

	// TC
	out = Paragraph{}
	emitField(ctx, &out, `TC "term"`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "entry"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "TC" {
		t.Errorf("emitField TC = %+v", out.Runs)
	}

	// FORMTEXT
	out = Paragraph{}
	emitField(ctx, &out, `FORMTEXT`, nil, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].Kind != "formField" || out.Runs[0].FormField == nil || out.Runs[0].FormField.Type != "text" {
		t.Errorf("emitField FORMTEXT = %+v", out.Runs)
	}

	// FORMCHECKBOX
	out = Paragraph{}
	emitField(ctx, &out, `FORMCHECKBOX`, nil, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].Kind != "formField" || out.Runs[0].FormField == nil || out.Runs[0].FormField.Type != "checkbox" {
		t.Errorf("emitField FORMCHECKBOX = %+v", out.Runs)
	}

	// FORMDROPDOWN
	out = Paragraph{}
	emitField(ctx, &out, `FORMDROPDOWN`, nil, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].Kind != "formField" || out.Runs[0].FormField == nil || out.Runs[0].FormField.Type != "dropdown" {
		t.Errorf("emitField FORMDROPDOWN = %+v", out.Runs)
	}

	// Default fallback (no recognized keyword)
	out = Paragraph{}
	emitField(ctx, &out, `UNKNOWN`, []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}}, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].Text != "text" {
		t.Errorf("emitField default = %+v", out.Runs)
	}

	// Empty display runs with semantic field should still preserve the field.
	out = Paragraph{}
	emitField(ctx, &out, `PAGE`, nil, fmt, nil)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "PAGE" || out.Runs[0].FieldInstr != "PAGE" {
		t.Errorf("emitField PAGE empty runs = %+v, want semantic PAGE field", out.Runs)
	}
}

func TestNormalizeSdtInlineRunsExtended(t *testing.T) {
	ctx := &resolveCtx{}
	fmt := RunFormat{}

	// Run
	out := normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "hello"}}}}},
	}, fmt)
	if len(out) != 1 || out[0].Text != "hello" {
		t.Errorf("normalizeSdtInlineRuns run = %+v", out)
	}

	// Hyperlink
	out = normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineHyperlink, Hyperlink: &parts.Hyperlink{ID: "rId1", Runs: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "link"}}}}}}},
	}, fmt)
	if len(out) != 1 || out[0].Text != "link" {
		t.Errorf("normalizeSdtInlineRuns hyperlink = %+v", out)
	}

	// Insertion
	out = normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineIns, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "inserted"}}}}, RevisionAuthor: "Alice", RevisionDate: "2024-01-01"},
	}, fmt)
	if len(out) != 1 || out[0].Revision != "inserted" || out[0].RevisionAuthor != "Alice" || out[0].RevisionDate != "2024-01-01" {
		t.Errorf("normalizeSdtInlineRuns ins = %+v", out)
	}

	// Deletion
	out = normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineDel, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "deleted"}}}}, RevisionAuthor: "Bob", RevisionDate: "2024-02-01"},
	}, fmt)
	if len(out) != 1 || out[0].Revision != "deleted" || out[0].RevisionAuthor != "Bob" || out[0].RevisionDate != "2024-02-01" {
		t.Errorf("normalizeSdtInlineRuns del = %+v", out)
	}

	// Math
	out = normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineMath, Math: &parts.OMMath{RawXML: []byte(`<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath>`)}},
	}, fmt)
	if len(out) != 1 || out[0].Kind != "math" || out[0].MathML == "" {
		t.Errorf("normalizeSdtInlineRuns math = %+v", out)
	}

	// Field
	out = normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineField, Field: &parts.Field{Instr: "PAGE", Runs: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "1"}}}}}}},
	}, fmt)
	if len(out) != 1 || out[0].FieldKind != "PAGE" {
		t.Errorf("normalizeSdtInlineRuns field = %+v", out)
	}

	// Nested SDT
	out = normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineSDT, SdtProps: &parts.SdtProperties{ID: 42, Type: "text"}, SdtInlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "nested"}}}}},
		}},
	}, fmt)
	if len(out) != 1 || out[0].Text != "nested" || out[0].ContentControl == nil || out[0].ContentControl.ID != 42 {
		t.Errorf("normalizeSdtInlineRuns sdt = %+v", out)
	}
}

func TestNormalizeParagraphNil(t *testing.T) {
	ctx := &resolveCtx{}
	out := normalizeParagraph(ctx, nil)
	if out.Text != "" || len(out.Runs) != 0 {
		t.Errorf("normalizeParagraph(nil) = %+v", out)
	}
}

func TestNormalizeParagraphProperties(t *testing.T) {
	ctx := &resolveCtx{
		defaultStyle: "Normal",
		styles: map[string]Style{
			"Heading1": {Name: "Heading 1", Format: ParagraphFormat{Alignment: "center"}},
		},
	}
	para := &parts.Paragraph{
		Properties: &parts.ParagraphProperties{
			Style: &parts.StringVal{Val: "Heading1"},
			Numbering: &parts.NumberingReference{
				NumID: &parts.IntVal{Val: 1},
				Level: &parts.IntVal{Val: 0},
			},
		},
	}
	out := normalizeParagraph(ctx, para)
	if out.StyleID != "Heading1" || out.StyleName != "Heading 1" {
		t.Errorf("style mismatch: %+v", out)
	}
	if out.Numbering == nil || out.Numbering.NumID != 1 {
		t.Errorf("numbering mismatch: %+v", out.Numbering)
	}
	if out.List == nil {
		t.Error("expected list marker")
	}
}

func TestNormalizeParagraphFieldSequence(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " PAGE "}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "1"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "end"}}}}},
		},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "PAGE" || out.Runs[0].Text != "1" {
		t.Errorf("normalizeParagraph field sequence = %+v", out.Runs)
	}
}

func TestNormalizeParagraphNestedField(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " IF "}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " PAGE "}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "2"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "end"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: ` > 1 "yes" "no" `}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "yes"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "end"}}}}},
		},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "IF" || out.Runs[0].Text != "yes" {
		t.Errorf("normalizeParagraph nested field = %+v", out.Runs)
	}
	if len(out.Runs) == 1 {
		if len(out.Runs[0].FieldInstrRuns) != 3 {
			t.Fatalf("nested IF FieldInstrRuns len = %d, want 3", len(out.Runs[0].FieldInstrRuns))
		}
		if len(out.Runs[0].FieldInstrRuns[1].Fields) != 1 || out.Runs[0].FieldInstrRuns[1].Fields[0].Kind != "PAGE" {
			t.Fatalf("nested IF instruction field = %+v", out.Runs[0].FieldInstrRuns[1].Fields)
		}
	}
}

func TestNormalizeParagraphDeeplyNestedField(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " IF "}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " IF "}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " PAGE "}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "1"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "end"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: ` > 0 "inner-yes" "inner-no" `}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "inner-yes"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "end"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: ` = "inner-yes" "outer-yes" "outer-no" `}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "outer-yes"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "end"}}}}},
		},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "IF" || out.Runs[0].Text != "outer-yes" {
		t.Errorf("normalizeParagraph deeply nested field = %+v", out.Runs)
	}
	if len(out.Runs) == 1 {
		if len(out.Runs[0].FieldInstrRuns) != 3 {
			t.Fatalf("deeply nested IF FieldInstrRuns len = %d, want 3", len(out.Runs[0].FieldInstrRuns))
		}
		if len(out.Runs[0].FieldInstrRuns[1].Fields) != 1 || out.Runs[0].FieldInstrRuns[1].Fields[0].Kind != "IF" {
			t.Fatalf("deeply nested IF instruction field = %+v", out.Runs[0].FieldInstrRuns[1].Fields)
		}
		inner := out.Runs[0].FieldInstrRuns[1].Fields[0]
		if len(inner.InstrRuns) != 3 || len(inner.InstrRuns[1].Fields) != 1 || inner.InstrRuns[1].Fields[0].Kind != "PAGE" {
			t.Fatalf("inner IF instruction field = %+v", inner.InstrRuns)
		}
	}
}

func TestNormalizeParagraphFieldWithoutDisplayRunsPreserved(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: ` NEXTIF City = "Chicago" `}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "end"}}}}},
		},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "NEXTIF" {
		t.Fatalf("normalizeParagraph zero-display field = %+v", out.Runs)
	}
	if got := out.Runs[0].FieldInstr; got != `NEXTIF City = "Chicago"` {
		t.Fatalf("FieldInstr = %q, want NEXTIF City = \"Chicago\"", got)
	}
}

func TestNormalizeParagraphUnclosedField(t *testing.T) {
	ctx := &resolveCtx{}
	// Unclosed field with result runs → flushed by story boundary helper
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " PAGE "}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "separate"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "3"}}}}},
		},
	}
	out := normalizeParagraph(ctx, para)
	flushRemainingParagraphFields(ctx, &out, RunFormat{})
	if len(out.Runs) != 1 || out.Runs[0].FieldKind != "PAGE" || out.Runs[0].Text != "3" {
		t.Errorf("normalizeParagraph unclosed with result = %+v", out.Runs)
	}

	// Unclosed field WITHOUT result runs → discarded by story boundary helper
	ctx.fieldStack = ctx.fieldStack[:0]
	para = &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentFldChar, FldChar: &parts.FldChar{Type: "begin"}}}}},
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentInstrText, InstrText: " PAGE "}}}},
		},
	}
	out = normalizeParagraph(ctx, para)
	flushRemainingParagraphFields(ctx, &out, RunFormat{})
	if len(out.Runs) != 0 {
		t.Errorf("normalizeParagraph unclosed no result = %+v, want empty", out.Runs)
	}
}

func TestNormalizeParagraphFallbackRuns(t *testing.T) {
	ctx := &resolveCtx{
		pkg: &parts.Package{
			MainPartName: "word/document.xml",
			ScopedHyperlinkURLs: map[parts.RelationshipRef]string{
				{SourcePart: "word/document.xml", ID: "rId1"}: "http://example.com",
			},
		},
		currentPart: "word/document.xml",
	}
	para := &parts.Paragraph{
		Runs:       []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "hello"}}}}},
		Hyperlinks: []*parts.Hyperlink{{ID: "rId1", Runs: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "link"}}}}}}},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Runs) != 2 || out.Runs[0].Text != "hello" || out.Runs[1].Text != "link" {
		t.Errorf("normalizeParagraph fallback = %+v", out.Runs)
	}
}

func TestNormalizeParagraphBookmarks(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Bookmarks: []string{"bm1", "bm2"},
		Runs:      []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Bookmarks) != 2 || out.Bookmarks[0] != "bm1" || out.Bookmarks[1] != "bm2" {
		t.Errorf("bookmarks = %v", out.Bookmarks)
	}
}

func TestNormalizeParagraphRuby(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRuby, Ruby: &parts.Ruby{
				BaseRuns: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "base"}}}}},
				TextRuns: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "ruby"}}}}},
			}},
		},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Runs) != 1 || out.Runs[0].Kind != "ruby" || out.Runs[0].Ruby == nil {
		t.Errorf("normalizeParagraph ruby = %+v", out.Runs)
	}
}

func TestNormalizeParagraphMath(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineMath, Math: &parts.OMMath{RawXML: []byte(`<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath>`)}},
		},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Runs) != 1 || out.Runs[0].Kind != "math" || out.Runs[0].MathML == "" {
		t.Errorf("normalizeParagraph math = %+v", out.Runs)
	}
}

func TestNormalizeBodyBlocksExtended(t *testing.T) {
	ctx := &resolveCtx{}
	pkg := &parts.Package{}

	// Table block
	blocks := []parts.BodyBlock{
		{Kind: parts.BodyBlockTable, Table: &parts.Table{Rows: []*parts.TableRow{{Cells: []*parts.TableCell{{Blocks: []parts.ContentBlock{
			{Kind: parts.ContentBlockParagraph, Paragraph: &parts.Paragraph{Runs: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "cell"}}}}}}},
		}}}}}}},
	}
	out := normalizeBodyBlocks(ctx, pkg, blocks)
	if len(out) != 1 || out[0].Kind != "table" || out[0].Table == nil || len(out[0].Table.Rows) != 1 {
		t.Errorf("normalizeBodyBlocks table = %+v", out)
	}

	// Section block
	blocks = []parts.BodyBlock{
		{Kind: parts.BodyBlockSection, Section: &parts.SectionProperties{PageSize: &parts.PageSize{Width: 100, Height: 200}}},
	}
	out = normalizeBodyBlocks(ctx, pkg, blocks)
	if len(out) != 1 || out[0].Kind != "section" || out[0].Section == nil || out[0].Section.PageSize.Width != 100 {
		t.Errorf("normalizeBodyBlocks section = %+v", out)
	}

	// SDT block wrapping paragraph
	blocks = []parts.BodyBlock{
		{Kind: parts.BodyBlockSDT, SdtProps: &parts.SdtProperties{ID: 99, Type: "text"}, SdtBlocks: []parts.BodyBlock{
			{Kind: parts.BodyBlockParagraph, Paragraph: &parts.Paragraph{Runs: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "sdt"}}}}}}},
		}},
	}
	out = normalizeBodyBlocks(ctx, pkg, blocks)
	if len(out) != 1 || out[0].ContentControl == nil || out[0].ContentControl.ID != 99 {
		t.Errorf("normalizeBodyBlocks sdt = %+v", out)
	}
}

func TestRunFormatFromPropertiesDoubleStrike(t *testing.T) {
	val := parts.BoolVal(true)
	props := &parts.RunProperties{DoubleStrike: &parts.OnOff{Val: &val}}
	got := runFormatFromPropertiesWithTheme(props, nil, nil)
	if got.DoubleStrike == nil || !*got.DoubleStrike {
		t.Errorf("expected DoubleStrike=true, got %+v", got.DoubleStrike)
	}
}

func TestNormalizeSdtInlineRunsRuby(t *testing.T) {
	ctx := &resolveCtx{}
	fmt := RunFormat{}
	out := normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineRuby, Ruby: &parts.Ruby{
			BaseRuns: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "base"}}}}},
			TextRuns: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "rt"}}}}},
		}},
	}, fmt)
	if len(out) != 1 || out[0].Kind != "ruby" || out[0].Ruby == nil || out[0].Ruby.Base != "base" {
		t.Errorf("normalizeSdtInlineRuns ruby = %+v", out)
	}
}

func TestRunFormatFromPropertiesExtended(t *testing.T) {
	tests := []struct {
		name  string
		props *parts.RunProperties
		check func(t *testing.T, got RunFormat)
	}{
		{
			name:  "CS (complex script)",
			props: &parts.RunProperties{CS: &parts.OnOff{Val: boolValPtr(true)}},
			check: func(t *testing.T, got RunFormat) {
				if got.ComplexScript == nil || !*got.ComplexScript {
					t.Errorf("expected ComplexScript=true")
				}
			},
		},
		{
			name:  "WebHidden",
			props: &parts.RunProperties{WebHidden: &parts.OnOff{Val: boolValPtr(true)}},
			check: func(t *testing.T, got RunFormat) {
				if got.WebHidden == nil || !*got.WebHidden {
					t.Errorf("expected WebHidden=true")
				}
			},
		},
		{
			name:  "Kerning",
			props: &parts.RunProperties{Kerning: &parts.IntVal{Val: 24}},
			check: func(t *testing.T, got RunFormat) {
				if got.Kerning == nil || *got.Kerning != 24 {
					t.Errorf("expected Kerning=24, got %v", got.Kerning)
				}
			},
		},
		{
			name:  "CharScale",
			props: &parts.RunProperties{CharScale: &parts.IntVal{Val: 150}},
			check: func(t *testing.T, got RunFormat) {
				if got.CharScale == nil || *got.CharScale != 150 {
					t.Errorf("expected CharScale=150, got %v", got.CharScale)
				}
			},
		},
		{
			name:  "FitText",
			props: &parts.RunProperties{FitText: &parts.IntVal{Val: 1440}},
			check: func(t *testing.T, got RunFormat) {
				if got.FitTextWidth == nil || *got.FitTextWidth != 1440 {
					t.Errorf("expected FitTextWidth=1440, got %v", got.FitTextWidth)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runFormatFromPropertiesWithTheme(tt.props, nil, nil)
			tt.check(t, got)
		})
	}
}

func boolValPtr(b bool) *parts.BoolVal {
	v := parts.BoolVal(b)
	return &v
}

func TestNormalizeParagraphMoveFromMoveTo(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineMoveFrom, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "old"}}}}, RevisionAuthor: "Alice", RevisionDate: "2024-01-01", MoveID: 1},
			{Kind: parts.ParagraphInlineMoveTo, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "new"}}}}, RevisionAuthor: "Alice", RevisionDate: "2024-01-01", MoveID: 1},
		},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(out.Runs))
	}
	if out.Runs[0].Revision != "movedFrom" || out.Runs[0].RevisionAuthor != "Alice" || out.Runs[0].MoveID != 1 {
		t.Errorf("run0 = %+v", out.Runs[0])
	}
	if out.Runs[1].Revision != "movedTo" || out.Runs[1].RevisionAuthor != "Alice" || out.Runs[1].MoveID != 1 {
		t.Errorf("run1 = %+v", out.Runs[1])
	}
}

func TestNormalizeParagraphMoveRanges(t *testing.T) {
	ctx := &resolveCtx{}
	para := &parts.Paragraph{
		Inlines: []parts.ParagraphInline{
			{Kind: parts.ParagraphInlineRun, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "text"}}}}},
		},
		MoveFromRangeStarts: []parts.MoveRangeStart{{ID: 1, Author: "Alice", Date: "2024-01-01", Name: "move1"}},
		MoveFromRangeEnds:   []parts.MoveRangeEnd{{ID: 1}},
		MoveToRangeStarts:   []parts.MoveRangeStart{{ID: 2, Author: "Bob", Date: "2024-02-01"}},
		MoveToRangeEnds:     []parts.MoveRangeEnd{{ID: 2}},
	}
	out := normalizeParagraph(ctx, para)
	if len(out.MoveFromRangeStarts) != 1 || out.MoveFromRangeStarts[0].ID != 1 || out.MoveFromRangeStarts[0].Author != "Alice" || out.MoveFromRangeStarts[0].Name != "move1" {
		t.Errorf("MoveFromRangeStarts = %+v", out.MoveFromRangeStarts)
	}
	if len(out.MoveFromRangeEnds) != 1 || out.MoveFromRangeEnds[0] != 1 {
		t.Errorf("MoveFromRangeEnds = %+v", out.MoveFromRangeEnds)
	}
	if len(out.MoveToRangeStarts) != 1 || out.MoveToRangeStarts[0].ID != 2 || out.MoveToRangeStarts[0].Author != "Bob" {
		t.Errorf("MoveToRangeStarts = %+v", out.MoveToRangeStarts)
	}
	if len(out.MoveToRangeEnds) != 1 || out.MoveToRangeEnds[0] != 2 {
		t.Errorf("MoveToRangeEnds = %+v", out.MoveToRangeEnds)
	}
}

func TestNormalizeSdtInlineRunsMoveFromMoveTo(t *testing.T) {
	ctx := &resolveCtx{}
	fmt := RunFormat{}
	out := normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{
		{Kind: parts.ParagraphInlineMoveFrom, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "a"}}}}, RevisionAuthor: "Bob"},
		{Kind: parts.ParagraphInlineMoveTo, Run: &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "b"}}}}, RevisionAuthor: "Bob"},
	}, fmt)
	if len(out) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(out))
	}
	if out[0].Revision != "movedFrom" || out[0].Text != "a" {
		t.Errorf("run0 = %+v", out[0])
	}
	if out[1].Revision != "movedTo" || out[1].Text != "b" {
		t.Errorf("run1 = %+v", out[1])
	}
}

func TestParagraphFormatFromPropertiesTabs(t *testing.T) {
	props := &parts.ParagraphProperties{
		Tabs: []parts.TabStop{
			{Val: "left", Pos: 720},
			{Val: "center", Pos: 4320},
			{Val: "right", Pos: 8640},
			{Val: "clear", Pos: 1440},
		},
	}
	got := paragraphFormatFromProperties(props, nil)
	if len(got.TabStops) != 3 {
		t.Fatalf("expected 3 tab stops (clear skipped), got %d", len(got.TabStops))
	}
	if got.TabStops[0].Pos != 720 || got.TabStops[0].Algn != "left" {
		t.Errorf("tab0 = %+v", got.TabStops[0])
	}
	if got.TabStops[1].Pos != 4320 || got.TabStops[1].Algn != "center" {
		t.Errorf("tab1 = %+v", got.TabStops[1])
	}
	if got.TabStops[2].Pos != 8640 || got.TabStops[2].Algn != "right" {
		t.Errorf("tab2 = %+v", got.TabStops[2])
	}
}

func TestRunFormatFromPropertiesBorders(t *testing.T) {
	props := &parts.RunProperties{
		Borders: &parts.Border{Val: "single", Size: 8, Space: 1, Color: "FF0000"},
	}
	got := runFormatFromPropertiesWithTheme(props, nil, nil)
	if got.Borders == nil {
		t.Fatal("expected Borders to be set")
	}
	if got.Borders.Top == nil || got.Borders.Top.Val != "single" || got.Borders.Top.Color != "FF0000" {
		t.Errorf("top border = %+v", got.Borders.Top)
	}
	if got.Borders.Bottom == nil || got.Borders.Bottom.Val != "single" || got.Borders.Bottom.Color != "FF0000" {
		t.Errorf("bottom border = %+v", got.Borders.Bottom)
	}
	if got.Borders.Left == nil || got.Borders.Left.Val != "single" || got.Borders.Left.Color != "FF0000" {
		t.Errorf("left border = %+v", got.Borders.Left)
	}
	if got.Borders.Right == nil || got.Borders.Right.Val != "single" || got.Borders.Right.Color != "FF0000" {
		t.Errorf("right border = %+v", got.Borders.Right)
	}
}

func TestRunFormatFromPropertiesFELayout(t *testing.T) {
	props := &parts.RunProperties{
		EastAsianLayout: &parts.EastAsianLayout{
			Combine:          &parts.OnOff{Val: boolValPtr(true)},
			CombineBrackets:  &parts.OnOff{Val: boolValPtr(true)},
			Vert:             &parts.OnOff{Val: boolValPtr(true)},
			VertCompress:     &parts.OnOff{Val: boolValPtr(true)},
			CombineCharCount: 3,
		},
	}
	got := runFormatFromPropertiesWithTheme(props, nil, nil)
	if got.FELayout == nil {
		t.Fatal("expected FELayout to be set")
	}
	if !got.FELayout.CombineCharacters {
		t.Error("expected CombineCharacters=true")
	}
	if !got.FELayout.CombineBrackets {
		t.Error("expected CombineBrackets=true")
	}
	if !got.FELayout.Vertical {
		t.Error("expected Vertical=true")
	}
	if !got.FELayout.VerticalCompressed {
		t.Error("expected VerticalCompressed=true")
	}
	if got.FELayout.CombineCharCount != 3 {
		t.Errorf("expected CombineCharCount=3, got %d", got.FELayout.CombineCharCount)
	}
}

func TestParagraphFormatFromPropertiesExtended(t *testing.T) {
	props := &parts.ParagraphProperties{
		MirrorIndents:       &parts.OnOff{Val: boolValPtr(true)},
		NoAllowOverlap:      &parts.OnOff{Val: boolValPtr(true)},
		SuppressLineNumbers: &parts.OnOff{Val: boolValPtr(true)},
		TextAlignment:       &parts.StringVal{Val: "center"},
		Kinsoku:             &parts.OnOff{Val: boolValPtr(true)},
		WordWrap:            &parts.OnOff{Val: boolValPtr(true)},
		OverflowPunct:       &parts.OnOff{Val: boolValPtr(true)},
		TopLinePunct:        &parts.OnOff{Val: boolValPtr(true)},
		AutoSpaceDE:         &parts.OnOff{Val: boolValPtr(true)},
		AutoSpaceDN:         &parts.OnOff{Val: boolValPtr(true)},
	}
	got := paragraphFormatFromProperties(props, nil)
	if !got.MirrorIndents {
		t.Error("expected MirrorIndents=true")
	}
	if !got.NoAllowOverlap {
		t.Error("expected NoAllowOverlap=true")
	}
	if !got.NoLineNumber {
		t.Error("expected NoLineNumber=true")
	}
	if got.FontAlign != "center" {
		t.Errorf("expected FontAlign=center, got %q", got.FontAlign)
	}
	if !got.Kinsoku {
		t.Error("expected Kinsoku=true")
	}
	if got.WordWrap == nil || !*got.WordWrap {
		t.Error("expected WordWrap=true")
	}
	if !got.OverflowPunct {
		t.Error("expected OverflowPunct=true")
	}
	if !got.TopLinePunct {
		t.Error("expected TopLinePunct=true")
	}
	if !got.AutoSpaceDE {
		t.Error("expected AutoSpaceDE=true")
	}
	if !got.AutoSpaceDN {
		t.Error("expected AutoSpaceDN=true")
	}
}

func TestNormalizeSectionBidi(t *testing.T) {
	pkg := &parts.Package{}
	props := &parts.SectionProperties{
		Bidi: &parts.OnOff{Val: boolValPtr(true)},
	}
	section := normalizeSection(pkg, props)
	if !section.Bidi {
		t.Errorf("expected Bidi=true, got %v", section.Bidi)
	}
	props.Bidi = &parts.OnOff{Val: boolValPtr(false)}
	section = normalizeSection(pkg, props)
	if section.Bidi {
		t.Errorf("expected Bidi=false, got %v", section.Bidi)
	}
}

func TestNormalizeSectionPageBorders(t *testing.T) {
	pkg := &parts.Package{}
	props := &parts.SectionProperties{
		PageBorders: &parts.PageBorders{
			Top:        &parts.Border{Val: "single", Size: 24, Space: 4, Color: "FF0000"},
			Left:       &parts.Border{Val: "double", Size: 12, Space: 2, Color: "00FF00"},
			Bottom:     &parts.Border{Val: "dashed", Size: 8, Space: 1, Color: "0000FF"},
			Right:      &parts.Border{Val: "none", Size: 4, Space: 0, Color: "auto"},
			Display:    "firstPage",
			OffsetFrom: "text",
			ZOrder:     "front",
		},
	}
	section := normalizeSection(pkg, props)
	if section.PageBorders == nil {
		t.Fatal("expected PageBorders to be set")
	}
	if section.PageBorders.Top == nil || section.PageBorders.Top.Val != "single" || section.PageBorders.Top.SizePt != 3.0 || section.PageBorders.Top.SpacePt != 4 || section.PageBorders.Top.Color != "FF0000" {
		t.Errorf("unexpected top border: %+v", section.PageBorders.Top)
	}
	if section.PageBorders.Left == nil || section.PageBorders.Left.Val != "double" || section.PageBorders.Left.SizePt != 1.5 || section.PageBorders.Left.SpacePt != 2 || section.PageBorders.Left.Color != "00FF00" {
		t.Errorf("unexpected left border: %+v", section.PageBorders.Left)
	}
	if section.PageBorders.Bottom == nil || section.PageBorders.Bottom.Val != "dashed" || section.PageBorders.Bottom.SizePt != 1.0 || section.PageBorders.Bottom.SpacePt != 1 || section.PageBorders.Bottom.Color != "0000FF" {
		t.Errorf("unexpected bottom border: %+v", section.PageBorders.Bottom)
	}
	if section.PageBorders.Right == nil || section.PageBorders.Right.Val != "none" {
		t.Errorf("expected right border 'none', got %+v", section.PageBorders.Right)
	}
	if section.PageBorders.Display != "firstPage" {
		t.Errorf("expected Display=firstPage, got %q", section.PageBorders.Display)
	}
	if section.PageBorders.OffsetFrom != "text" {
		t.Errorf("expected OffsetFrom=text, got %q", section.PageBorders.OffsetFrom)
	}
	if section.PageBorders.ZOrder != "front" {
		t.Errorf("expected ZOrder=front, got %q", section.PageBorders.ZOrder)
	}
}

func TestNormalizeSectionPageBordersNil(t *testing.T) {
	pkg := &parts.Package{}
	section := normalizeSection(pkg, &parts.SectionProperties{})
	if section.PageBorders != nil {
		t.Errorf("expected PageBorders nil, got %+v", section.PageBorders)
	}
}

func TestNormalizeSectionFootnoteProps(t *testing.T) {
	pkg := &parts.Package{}
	props := &parts.SectionProperties{
		FootnoteProps: &parts.NoteProperties{
			Position: &parts.StringVal{Val: "belowText"},
			Format:   &parts.StringVal{Val: "upperRoman"},
			Start:    &parts.IntVal{Val: 5},
			Restart:  &parts.StringVal{Val: "eachSect"},
		},
	}
	section := normalizeSection(pkg, props)
	if section.FootnotePos != "belowText" {
		t.Errorf("expected FootnotePos=belowText, got %q", section.FootnotePos)
	}
	if section.FootnoteNumFmt != "upperRoman" {
		t.Errorf("expected FootnoteNumFmt=upperRoman, got %q", section.FootnoteNumFmt)
	}
	if section.FootnoteNumStart != 5 {
		t.Errorf("expected FootnoteNumStart=5, got %d", section.FootnoteNumStart)
	}
	if section.FootnoteRestart != "eachSect" {
		t.Errorf("expected FootnoteRestart=eachSect, got %q", section.FootnoteRestart)
	}
}

func TestNormalizeSectionEndnoteProps(t *testing.T) {
	pkg := &parts.Package{}
	props := &parts.SectionProperties{
		EndnoteProps: &parts.NoteProperties{
			Format:  &parts.StringVal{Val: "lowerLetter"},
			Start:   &parts.IntVal{Val: 2},
			Restart: &parts.StringVal{Val: "continuous"},
		},
	}
	section := normalizeSection(pkg, props)
	if section.EndnoteNumFmt != "lowerLetter" {
		t.Errorf("expected EndnoteNumFmt=lowerLetter, got %q", section.EndnoteNumFmt)
	}
	if section.EndnoteNumStart != 2 {
		t.Errorf("expected EndnoteNumStart=2, got %d", section.EndnoteNumStart)
	}
	if section.EndnoteRestart != "continuous" {
		t.Errorf("expected EndnoteRestart=continuous, got %q", section.EndnoteRestart)
	}
}

func TestNormalizeSectionNoEndnote(t *testing.T) {
	pkg := &parts.Package{}
	// w:noEndnote present and true → EndnoteAtSectionEnd = false
	props := &parts.SectionProperties{
		NoEndnote: &parts.OnOff{Val: boolValPtr(true)},
	}
	section := normalizeSection(pkg, props)
	if section.EndnoteAtSectionEnd {
		t.Errorf("expected EndnoteAtSectionEnd=false when noEndnote=true, got true")
	}

	// w:noEndnote present and false → EndnoteAtSectionEnd = true
	props.NoEndnote = &parts.OnOff{Val: boolValPtr(false)}
	section = normalizeSection(pkg, props)
	if !section.EndnoteAtSectionEnd {
		t.Errorf("expected EndnoteAtSectionEnd=true when noEndnote=false, got false")
	}

	// w:noEndnote absent → EndnoteAtSectionEnd = true
	section = normalizeSection(pkg, &parts.SectionProperties{})
	if !section.EndnoteAtSectionEnd {
		t.Errorf("expected EndnoteAtSectionEnd=true when noEndnote absent, got false")
	}
}

func TestParagraphFormatFromPropertiesSuppressAutoHyphens(t *testing.T) {
	props := &parts.ParagraphProperties{
		SuppressAutoHyphens: &parts.OnOff{Val: boolValPtr(true)},
	}
	got := paragraphFormatFromProperties(props, nil)
	if !got.NoAutoHyphenation {
		t.Errorf("expected NoAutoHyphenation=true, got %v", got.NoAutoHyphenation)
	}
	props.SuppressAutoHyphens = &parts.OnOff{Val: boolValPtr(false)}
	got = paragraphFormatFromProperties(props, nil)
	if got.NoAutoHyphenation {
		t.Errorf("expected NoAutoHyphenation=false, got %v", got.NoAutoHyphenation)
	}
}

func TestParagraphFormatFromPropertiesAdjustRightInd(t *testing.T) {
	props := &parts.ParagraphProperties{
		AdjustRightInd: &parts.OnOff{Val: boolValPtr(true)},
	}
	got := paragraphFormatFromProperties(props, nil)
	if !got.AdjustRightInd {
		t.Errorf("expected AdjustRightInd=true, got %v", got.AdjustRightInd)
	}
	props.AdjustRightInd = &parts.OnOff{Val: boolValPtr(false)}
	got = paragraphFormatFromProperties(props, nil)
	if got.AdjustRightInd {
		t.Errorf("expected AdjustRightInd=false, got %v", got.AdjustRightInd)
	}
}

func TestTableCellFormatFromPropertiesExtended(t *testing.T) {
	// hMerge
	out := tableCellFormatFromProperties(&parts.TableCellProperties{
		HMerge: &parts.VMerge{Val: "restart"},
	}, nil)
	if !out.HMerge {
		t.Errorf("expected HMerge=true, got %v", out.HMerge)
	}

	// noWrap
	out = tableCellFormatFromProperties(&parts.TableCellProperties{
		NoWrap: &parts.OnOff{Val: boolValPtr(true)},
	}, nil)
	if !out.NoWrap {
		t.Errorf("expected NoWrap=true, got %v", out.NoWrap)
	}
	out = tableCellFormatFromProperties(&parts.TableCellProperties{
		NoWrap: &parts.OnOff{Val: boolValPtr(false)},
	}, nil)
	if out.NoWrap {
		t.Errorf("expected NoWrap=false, got %v", out.NoWrap)
	}

	// cell margins (tcMar)
	out = tableCellFormatFromProperties(&parts.TableCellProperties{
		Margins: &parts.TableCellMargins{
			Top:    &parts.TableMeasure{Type: "dxa", Width: 10},
			Bottom: &parts.TableMeasure{Type: "dxa", Width: 20},
			Left:   &parts.TableMeasure{Type: "dxa", Width: 30},
			Right:  &parts.TableMeasure{Type: "dxa", Width: 40},
		},
	}, nil)
	if out.CellPadTop != 10 {
		t.Errorf("expected CellPadTop=10, got %d", out.CellPadTop)
	}
	if out.CellPadBottom != 20 {
		t.Errorf("expected CellPadBottom=20, got %d", out.CellPadBottom)
	}
	if out.CellPadLeft != 30 {
		t.Errorf("expected CellPadLeft=30, got %d", out.CellPadLeft)
	}
	if out.CellPadRight != 40 {
		t.Errorf("expected CellPadRight=40, got %d", out.CellPadRight)
	}

	// cell margins with non-dxa type should be ignored
	out = tableCellFormatFromProperties(&parts.TableCellProperties{
		Margins: &parts.TableCellMargins{
			Top: &parts.TableMeasure{Type: "nil", Width: 10},
		},
	}, nil)
	if out.CellPadTop != 0 {
		t.Errorf("expected CellPadTop=0 for nil type, got %d", out.CellPadTop)
	}
}

func TestNormalizeTableRowCantSplit(t *testing.T) {
	ctx := &resolveCtx{}
	row := normalizeTableRow(ctx, &parts.TableRow{
		Properties: &parts.TableRowProperties{
			CantSplit: &parts.OnOff{Val: boolValPtr(true)},
		},
	}, 0, 1, 0, nil, nil)
	if !row.Format.CantSplit {
		t.Errorf("expected CantSplit=true, got %v", row.Format.CantSplit)
	}

	row = normalizeTableRow(ctx, &parts.TableRow{
		Properties: &parts.TableRowProperties{
			CantSplit: &parts.OnOff{Val: boolValPtr(false)},
		},
	}, 0, 1, 0, nil, nil)
	if row.Format.CantSplit {
		t.Errorf("expected CantSplit=false, got %v", row.Format.CantSplit)
	}
}

func TestNormalizeTableRowCellSpacing(t *testing.T) {
	ctx := &resolveCtx{}
	row := normalizeTableRow(ctx, &parts.TableRow{
		Properties: &parts.TableRowProperties{
			CellSpacing: &parts.TableMeasure{Type: "dxa", Width: 200},
		},
	}, 0, 1, 1, nil, nil)
	if row.Format.CellSpacing != 200 {
		t.Errorf("expected CellSpacing=200, got %d", row.Format.CellSpacing)
	}
}

func TestNormalizeTableRowTblPrEx(t *testing.T) {
	ctx := &resolveCtx{}
	row := normalizeTableRow(ctx, &parts.TableRow{
		Properties: &parts.TableRowProperties{
			CantSplit: &parts.OnOff{Val: boolValPtr(true)},
		},
		TblPrEx: &parts.TablePropertiesException{
			Shading: &parts.Shading{Fill: "FF0000"},
			Borders: &parts.TableBorders{
				Top: &parts.Border{Val: "single", Size: 8, Color: "000000"},
			},
			Alignment: &parts.StringVal{Val: "center"},
			Indent:    &parts.TableMeasure{Type: "dxa", Width: 144},
			CellMargin: &parts.TableCellMargin{
				Top:    &parts.TableMeasure{Type: "dxa", Width: 100},
				Bottom: &parts.TableMeasure{Type: "dxa", Width: 100},
				Left:   &parts.TableMeasure{Type: "dxa", Width: 100},
				Right:  &parts.TableMeasure{Type: "dxa", Width: 100},
			},
			BidiVisual: &parts.OnOff{Val: boolValPtr(true)},
		},
	}, 0, 1, 1, nil, nil)
	if row.Override == nil {
		t.Fatalf("expected Override to be set")
	}
	if row.Override.Shading != "FF0000" {
		t.Errorf("expected Shading=FF0000, got %s", row.Override.Shading)
	}
	if row.Override.Borders == nil || row.Override.Borders.Top == nil {
		t.Errorf("expected Borders.Top to be set")
	}
	if row.Override.Alignment != "center" {
		t.Errorf("expected Alignment=center, got %s", row.Override.Alignment)
	}
	if row.Override.Indent != 144 {
		t.Errorf("expected Indent=144, got %d", row.Override.Indent)
	}
	if row.Override.CellPadTop != 100 || row.Override.CellPadBottom != 100 || row.Override.CellPadLeft != 100 || row.Override.CellPadRight != 100 {
		t.Errorf("expected cell padding 100, got top=%d bottom=%d left=%d right=%d", row.Override.CellPadTop, row.Override.CellPadBottom, row.Override.CellPadLeft, row.Override.CellPadRight)
	}
	if !row.Override.BidiVisual {
		t.Errorf("expected BidiVisual=true")
	}
}

func TestMergeTableCellFormatPadding(t *testing.T) {
	base := TableCellFormat{CellPadTop: 1, CellPadBottom: 2, CellPadLeft: 3, CellPadRight: 4}
	overlay := TableCellFormat{CellPadTop: 10, CellPadBottom: 20}
	out := mergeTableCellFormat(base, overlay)
	if out.CellPadTop != 10 {
		t.Errorf("expected CellPadTop=10, got %d", out.CellPadTop)
	}
	if out.CellPadBottom != 20 {
		t.Errorf("expected CellPadBottom=20, got %d", out.CellPadBottom)
	}
	if out.CellPadLeft != 3 {
		t.Errorf("expected CellPadLeft=3, got %d", out.CellPadLeft)
	}
	if out.CellPadRight != 4 {
		t.Errorf("expected CellPadRight=4, got %d", out.CellPadRight)
	}
}

func TestNormalizeTableCellTrackedChanges(t *testing.T) {
	ctx := &resolveCtx{}
	cell := normalizeTableCell(ctx, &parts.TableCell{
		Properties: &parts.TableCellProperties{
			CellIns:   &parts.TrackedCellChange{Author: "Alice", Date: "2024-01-01", ID: 1},
			CellDel:   &parts.TrackedCellChange{Author: "Bob", Date: "2024-02-01", ID: 2},
			CellMerge: &parts.TrackedCellChange{Author: "Carol", Date: "2024-03-01", ID: 3},
		},
	}, 0, 1, 0, 1, nil, nil)
	if cell.CellIns == nil || cell.CellIns.Author != "Alice" || cell.CellIns.ID != 1 {
		t.Errorf("CellIns = %+v", cell.CellIns)
	}
	if cell.CellDel == nil || cell.CellDel.Author != "Bob" || cell.CellDel.ID != 2 {
		t.Errorf("CellDel = %+v", cell.CellDel)
	}
	if cell.CellMerge == nil || cell.CellMerge.Author != "Carol" || cell.CellMerge.ID != 3 {
		t.Errorf("CellMerge = %+v", cell.CellMerge)
	}

	// nil properties → no panics, nil marks
	cell = normalizeTableCell(ctx, &parts.TableCell{}, 0, 1, 0, 1, nil, nil)
	if cell.CellIns != nil || cell.CellDel != nil || cell.CellMerge != nil {
		t.Errorf("expected nil tracked changes for empty cell, got ins=%v del=%v merge=%v", cell.CellIns, cell.CellDel, cell.CellMerge)
	}
}

func TestResolveDocumentSettingsExtended(t *testing.T) {
	pkg := &parts.Package{
		Document: &parts.Document{Body: &parts.Body{}},
		Settings: &parts.Settings{
			MirrorMargins:              &parts.OnOff{Val: boolValPtr(true)},
			DoNotDisplayPageBoundaries: &parts.OnOff{Val: boolValPtr(true)},
			BordersDoNotSurroundFooter: &parts.OnOff{Val: boolValPtr(true)},
			BordersDoNotSurroundHeader: &parts.OnOff{Val: boolValPtr(true)},
			DoNotTrackFormatting:       &parts.OnOff{Val: boolValPtr(true)},
			DoNotTrackMoves:            &parts.OnOff{Val: boolValPtr(true)},
		},
	}
	doc, err := ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if !doc.MirrorMargins {
		t.Errorf("expected MirrorMargins=true")
	}
	if !doc.DoNotDisplayPageBoundaries {
		t.Errorf("expected DoNotDisplayPageBoundaries=true")
	}
	if !doc.BordersDoNotSurroundFooter {
		t.Errorf("expected BordersDoNotSurroundFooter=true")
	}
	if !doc.BordersDoNotSurroundHeader {
		t.Errorf("expected BordersDoNotSurroundHeader=true")
	}
	if !doc.DoNotTrackFormatting {
		t.Errorf("expected DoNotTrackFormatting=true")
	}
	if !doc.DoNotTrackMoves {
		t.Errorf("expected DoNotTrackMoves=true")
	}

	// false values should not set the flags
	pkg.Settings.MirrorMargins.Val = boolValPtr(false)
	pkg.Settings.DoNotDisplayPageBoundaries.Val = boolValPtr(false)
	doc, _ = ResolvePackage(pkg)
	if doc.MirrorMargins {
		t.Errorf("expected MirrorMargins=false")
	}
	if doc.DoNotDisplayPageBoundaries {
		t.Errorf("expected DoNotDisplayPageBoundaries=false")
	}
}

func TestResolveCompatSettingsNewFlags(t *testing.T) {
	cs := &parts.CompatSettings{
		Settings: []parts.CompatSetting{
			{Name: "adjustLineHeightInTable", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
			{Name: "alignTablesRowByRow", URI: "http://schemas.microsoft.com/office/word", Value: "true"},
			{Name: "allowPNG", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
		},
	}
	got := resolveCompatSettings(cs)
	if got == nil {
		t.Fatal("expected non-nil DocCompatibility")
	}
	if !got.AdjustLineHeightInTable {
		t.Errorf("expected AdjustLineHeightInTable=true")
	}
	if !got.AlignTablesRowByRow {
		t.Errorf("expected AlignTablesRowByRow=true")
	}
	if !got.AllowPNG {
		t.Errorf("expected AllowPNG=true")
	}
}

func TestRunFormatFromPropertiesPositionAndEffect(t *testing.T) {
	props := &parts.RunProperties{
		Position: &parts.IntVal{Val: -8},
		Effect:   &parts.StringVal{Val: "blinkBackground"},
	}
	got := runFormatFromPropertiesWithTheme(props, nil, nil)
	if got.Position == nil || *got.Position != -8 {
		t.Errorf("expected Position=-8, got %v", got.Position)
	}
	if got.TextAnimation != "blinkBackground" {
		t.Errorf("expected TextAnimation=blinkBackground, got %q", got.TextAnimation)
	}
}

func TestResolveCellBordersDiagonal(t *testing.T) {
	b := &parts.TableCellBorders{
		Top:   &parts.Border{Val: "single", Size: 4, Color: "000000"},
		TL2BR: &parts.Border{Val: "dashed", Size: 6, Color: "FF0000"},
		TR2BL: &parts.Border{Val: "double", Size: 8, Color: "00FF00"},
	}
	out := resolveCellBorders(b)
	if out == nil {
		t.Fatal("expected non-nil CellBorders")
	}
	if out.Top == nil || out.Top.Val != "single" {
		t.Errorf("expected Top=single, got %+v", out.Top)
	}
	if out.TL2BR == nil || out.TL2BR.Val != "dashed" || out.TL2BR.SizePt != 0.75 {
		t.Errorf("expected TL2BR=dashed 0.75pt, got %+v", out.TL2BR)
	}
	if out.TR2BL == nil || out.TR2BL.Val != "double" || out.TR2BL.SizePt != 1.0 {
		t.Errorf("expected TR2BL=double 1.0pt, got %+v", out.TR2BL)
	}
}

func TestResolveActiveWritingStyles(t *testing.T) {
	pkg := &parts.Package{
		Document: &parts.Document{Body: &parts.Body{}},
		Settings: &parts.Settings{
			ActiveWritingStyles: []parts.ActiveWritingStyle{
				{Lang: "en-US", VendorID: "Microsoft", DLLVersion: 1, NLCheck: true, CheckStyle: true},
				{Lang: "fr-FR", VendorID: "Microsoft", DLLVersion: 2},
			},
		},
	}
	doc, err := ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage failed: %v", err)
	}
	if len(doc.ActiveWritingStyles) != 2 {
		t.Fatalf("expected 2 active writing styles, got %d", len(doc.ActiveWritingStyles))
	}
	if doc.ActiveWritingStyles[0].Lang != "en-US" || !doc.ActiveWritingStyles[0].NLCheck {
		t.Errorf("first style mismatch: %+v", doc.ActiveWritingStyles[0])
	}
	if doc.ActiveWritingStyles[1].Lang != "fr-FR" || doc.ActiveWritingStyles[1].DLLVersion != 2 {
		t.Errorf("second style mismatch: %+v", doc.ActiveWritingStyles[1])
	}
}

func TestResolveScopedChartInHeader(t *testing.T) {
	docChartXML, err := chart.MarshalChart(&chart.Chart{
		ChartType:  chart.ChartColumn,
		Title:      "DocChart",
		Categories: []string{"A"},
		Series:     []chart.Series{{Name: "DocSeries", Values: []float64{1}}},
	})
	if err != nil {
		t.Fatalf("marshal doc chart: %v", err)
	}
	hdrChartXML, err := chart.MarshalChart(&chart.Chart{
		ChartType:  chart.ChartBar,
		Title:      "HdrChart",
		Categories: []string{"B"},
		Series:     []chart.Series{{Name: "HdrSeries", Values: []float64{2}}},
	})
	if err != nil {
		t.Fatalf("marshal hdr chart: %v", err)
	}

	pkg := &parts.Package{
		MainPartName: "word/document.xml",
		Document: &parts.Document{
			Body: &parts.Body{
				Paragraphs: []*parts.Paragraph{{
					Runs: []*parts.Run{{
						Contents: []parts.RunContent{{
							Kind: parts.RunContentDrawing,
							Drawing: &parts.Drawing{
								Inline: &parts.DrawingInline{
									Graphic: &parts.Graphic{
										GraphicData: &parts.GraphicData{
											URI:   "http://schemas.openxmlformats.org/drawingml/2006/chart",
											Chart: &parts.ChartRef{ID: "rId1"},
										},
									},
								},
							},
						}},
					}},
				}},
				Section: &parts.SectionProperties{
					HeaderRefs: []parts.HeaderFooterReference{{Type: "default", ID: "rIdHdr"}},
				},
			},
		},
		PartRels: map[string]*parts.Relationships{
			"word/document.xml": {
				Relationships: []*parts.Relationship{
					{ID: "rId1", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart", Target: "charts/docChart.xml"},
					{ID: "rIdHdr", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header", Target: "header1.xml"},
				},
			},
			"word/header1.xml": {
				Relationships: []*parts.Relationship{
					{ID: "rId1", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart", Target: "charts/hdrChart.xml"},
				},
			},
		},
		ScopedChartParts: map[parts.RelationshipRef][]byte{
			{SourcePart: "word/document.xml", ID: "rId1"}: docChartXML,
			{SourcePart: "word/header1.xml", ID: "rId1"}:  hdrChartXML,
		},
		Headers: map[string]*parts.Header{
			"word/header1.xml": {
				Paragraphs: []*parts.Paragraph{{
					Runs: []*parts.Run{{
						Contents: []parts.RunContent{{
							Kind: parts.RunContentDrawing,
							Drawing: &parts.Drawing{
								Inline: &parts.DrawingInline{
									Graphic: &parts.Graphic{
										GraphicData: &parts.GraphicData{
											URI:   "http://schemas.openxmlformats.org/drawingml/2006/chart",
											Chart: &parts.ChartRef{ID: "rId1"},
										},
									},
								},
							},
						}},
					}},
				}},
			},
		},
	}

	doc, err := ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}

	// Main document run should get the document chart (rendered as SVG in Image).
	if len(doc.Paragraphs) != 1 || len(doc.Paragraphs[0].Runs) != 1 {
		t.Fatalf("expected 1 paragraph with 1 run, got %+v", doc.Paragraphs)
	}
	docRun := doc.Paragraphs[0].Runs[0]
	if docRun.Kind != "chart" || docRun.Image == nil {
		t.Fatalf("expected document chart run, got kind=%q image=%v", docRun.Kind, docRun.Image)
	}
	if !strings.Contains(string(docRun.Image.Data), "DocChart") {
		t.Errorf("document chart SVG missing title: %s", string(docRun.Image.Data))
	}

	// Header run should get the header chart.
	hdrStory, ok := doc.Headers["word/header1.xml"]
	if !ok || len(hdrStory.Paragraphs) != 1 || len(hdrStory.Paragraphs[0].Runs) != 1 {
		t.Fatalf("expected header story with 1 paragraph and 1 run")
	}
	hdrRun := hdrStory.Paragraphs[0].Runs[0]
	if hdrRun.Kind != "chart" || hdrRun.Image == nil {
		t.Fatalf("expected header chart run, got kind=%q image=%v", hdrRun.Kind, hdrRun.Image)
	}
	if !strings.Contains(string(hdrRun.Image.Data), "HdrChart") {
		t.Errorf("header chart SVG missing title: %s", string(hdrRun.Image.Data))
	}
}

func TestResolveChartAppliesThemeColorTransforms(t *testing.T) {
	chartXML := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart>
    <c:plotArea>
      <c:barChart>
        <c:barDir val="col"/>
        <c:ser>
          <c:idx val="0"/>
          <c:order val="0"/>
          <c:spPr>
            <a:solidFill>
              <a:schemeClr val="accent1">
                <a:lumMod val="50000"/>
                <a:lumOff val="25000"/>
              </a:schemeClr>
            </a:solidFill>
          </c:spPr>
          <c:tx><c:strRef><c:strCache><c:pt idx="0"><c:v>Series 1</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:cat><c:strRef><c:strCache><c:pt idx="0"><c:v>A</c:v></c:pt></c:strCache></c:strRef></c:cat>
          <c:val><c:numRef><c:numCache><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
      </c:barChart>
      <c:catAx><c:axId val="1"/></c:catAx>
      <c:valAx><c:axId val="2"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`)

	pkg := &parts.Package{
		MainPartName: "word/document.xml",
		Theme: &parts.Theme{
			Elements: &parts.ThemeElements{
				ColorScheme: &parts.ColorScheme{
					Accent1: &parts.ThemeColorEntry{SRGB: &parts.ThemeSRGBColor{Val: "FF0000"}},
				},
			},
		},
		Document: &parts.Document{
			Body: &parts.Body{
				Paragraphs: []*parts.Paragraph{{
					Runs: []*parts.Run{{
						Contents: []parts.RunContent{{
							Kind: parts.RunContentDrawing,
							Drawing: &parts.Drawing{
								Inline: &parts.DrawingInline{
									Graphic: &parts.Graphic{
										GraphicData: &parts.GraphicData{
											URI:   "http://schemas.openxmlformats.org/drawingml/2006/chart",
											Chart: &parts.ChartRef{ID: "rId1"},
										},
									},
								},
							},
						}},
					}},
				}},
			},
		},
		PartRels: map[string]*parts.Relationships{
			"word/document.xml": {
				Relationships: []*parts.Relationship{
					{ID: "rId1", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart", Target: "charts/chart1.xml"},
				},
			},
		},
		ScopedChartParts: map[parts.RelationshipRef][]byte{
			{SourcePart: "word/document.xml", ID: "rId1"}: chartXML,
		},
	}

	doc, err := ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if len(doc.Paragraphs) != 1 || len(doc.Paragraphs[0].Runs) != 1 {
		t.Fatalf("expected 1 paragraph with 1 run, got %+v", doc.Paragraphs)
	}
	run := doc.Paragraphs[0].Runs[0]
	if run.Kind != "chart" || run.Image == nil {
		t.Fatalf("expected chart run, got kind=%q image=%v", run.Kind, run.Image)
	}
	if !strings.Contains(string(run.Image.Data), "#BF4040") {
		t.Fatalf("chart SVG missing transformed theme color: %s", string(run.Image.Data))
	}
}

func TestEvaluateSimpleXPath(t *testing.T) {
	data := []byte(`<root><customer id="42"><name>Alice</name></customer><customer><name>Bob</name></customer><items><item>First</item><item>Second</item></items></root>`)
	root, err := xmltree.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tests := []struct {
		xpath    string
		wantText string
		wantAttr string
	}{
		{"/root/customer/name", "Alice", ""},
		{"/root/customer[1]/name", "Alice", ""},
		{"/root/customer[2]/name", "Bob", ""},
		{"/root/customer/@id", "", "42"},
		{"//customer/name", "Alice", ""},
		{"/root/items/item[2]", "Second", ""},
		{"/root//item[1]", "First", ""},
		{"/root/nonexistent", "", ""},
		{"/root/customer/@missing", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.xpath, func(t *testing.T) {
			node, attrVal := evaluateSimpleXPath(root, tt.xpath)
			if attrVal != "" {
				if attrVal != tt.wantAttr {
					t.Errorf("attr = %q, want %q", attrVal, tt.wantAttr)
				}
				return
			}
			var got string
			if node != nil {
				got = node.AllText()
			}
			if got != tt.wantText {
				t.Errorf("text = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestResolveCompatSettingsDirectChildren(t *testing.T) {
	cs := &parts.CompatSettings{
		Settings: []parts.CompatSetting{
			{Name: "noTabHangInd", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
			{Name: "alignTablesRowByRow", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
			{Name: "allowPNG", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
			{Name: "autoSpaceLikeWord95", URI: "http://schemas.microsoft.com/office/word", Value: "0"},
		},
	}
	got := resolveCompatSettings(cs)
	if got == nil {
		t.Fatal("expected non-nil DocCompatibility")
	}
	if !got.NoTabHangInd {
		t.Errorf("expected NoTabHangInd=true")
	}
	if !got.AlignTablesRowByRow {
		t.Errorf("expected AlignTablesRowByRow=true")
	}
	if !got.AllowPNG {
		t.Errorf("expected AllowPNG=true")
	}
	if got.AutoSpaceLikeWord95 {
		t.Errorf("expected AutoSpaceLikeWord95=false")
	}
}

func TestResolveCompatSettingsNameVariants(t *testing.T) {
	cs := &parts.CompatSettings{
		Settings: []parts.CompatSetting{
			{Name: "doNotBreakConstrainedForcedTable", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
			{Name: "underlineTabInNumList", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
			{Name: "useNormalStyleForList", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
			{Name: "useWord2002TableStyleRules", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
			{Name: "cachedColBalance", URI: "http://schemas.microsoft.com/office/word", Value: "1"},
		},
	}
	got := resolveCompatSettings(cs)
	if got == nil {
		t.Fatal("expected non-nil DocCompatibility")
	}
	if !got.DoNotBreakConstrainedForcedTables {
		t.Errorf("expected DoNotBreakConstrainedForcedTables=true")
	}
	if !got.UnderlineTabInNumberingList {
		t.Errorf("expected UnderlineTabInNumberingList=true")
	}
	if !got.UseNormalStyleForListNumber {
		t.Errorf("expected UseNormalStyleForListNumber=true")
	}
	if !got.UseWord2002TableRules {
		t.Errorf("expected UseWord2002TableRules=true")
	}
	if !got.CachedFieldBalance {
		t.Errorf("expected CachedFieldBalance=true")
	}
}

func TestResolveCompatSettingsRemainingFlags(t *testing.T) {
	cs := &parts.CompatSettings{
		Settings: []parts.CompatSetting{
			{Name: "convMailMergeEsc", Value: "1"},
			{Name: "doNotSuppressParagraphBorders", Value: "1"},
			{Name: "doNotUseEastAsianBreakRules", Value: "1"},
			{Name: "doNotWrapTextWithPunct", Value: "1"},
			{Name: "forgetLastTabAlignment", Value: "1"},
			{Name: "layoutRawTableWidth", Value: "1"},
			{Name: "layoutTableRowsApart", Value: "1"},
			{Name: "mwSmallCaps", Value: "1"},
			{Name: "noExtraLineSpacing", Value: "1"},
			{Name: "noSpaceRaiseLower", Value: "1"},
			{Name: "printBodyTextBeforeHeader", Value: "1"},
			{Name: "printColBlack", Value: "1"},
			{Name: "selectFldWithFirstOrLastChar", Value: "1"},
			{Name: "showBreaksInFrames", Value: "1"},
			{Name: "spacingInWholePoints", Value: "1"},
			{Name: "subFontBySize", Value: "1"},
			{Name: "suppressBottomSpacing", Value: "1"},
			{Name: "suppressSpBfAfterPgBrk", Value: "1"},
			{Name: "suppressSpacingAtTopOfPage", Value: "1"},
			{Name: "suppressTopSpacing", Value: "1"},
			{Name: "suppressTopSpacingWP", Value: "1"},
			{Name: "swapBordersFacingPages", Value: "1"},
			{Name: "truncateFontHeightsLikeWP6", Value: "1"},
			{Name: "useFELayout", Value: "1"},
			{Name: "usePrinterMetrics", Value: "1"},
			{Name: "useSingleBorderforContiguousCells", Value: "1"},
			{Name: "useWord97LineBreakRules", Value: "1"},
			{Name: "wpJustification", Value: "1"},
			{Name: "wpSpaceWidth", Value: "1"},
			{Name: "wrapTrailSpaces", Value: "1"},
		},
	}
	got := resolveCompatSettings(cs)
	if got == nil {
		t.Fatal("expected non-nil DocCompatibility")
	}
	if !got.ConvMailMergeEsc {
		t.Errorf("expected ConvMailMergeEsc=true")
	}
	if !got.DoNotSuppressParagraphBorders {
		t.Errorf("expected DoNotSuppressParagraphBorders=true")
	}
	if !got.DoNotUseEastAsianBreakRules {
		t.Errorf("expected DoNotUseEastAsianBreakRules=true")
	}
	if !got.DoNotWrapTextWithPunct {
		t.Errorf("expected DoNotWrapTextWithPunct=true")
	}
	if !got.ForgetLastTabAlignment {
		t.Errorf("expected ForgetLastTabAlignment=true")
	}
	if !got.LayoutRawTableWidth {
		t.Errorf("expected LayoutRawTableWidth=true")
	}
	if !got.LayoutTableRowsApart {
		t.Errorf("expected LayoutTableRowsApart=true")
	}
	if !got.MwSmallCaps {
		t.Errorf("expected MwSmallCaps=true")
	}
	if !got.NoExtraLineSpacing {
		t.Errorf("expected NoExtraLineSpacing=true")
	}
	if !got.NoSpaceRaiseLower {
		t.Errorf("expected NoSpaceRaiseLower=true")
	}
	if !got.PrintBodyTextBeforeHeader {
		t.Errorf("expected PrintBodyTextBeforeHeader=true")
	}
	if !got.PrintColBlack {
		t.Errorf("expected PrintColBlack=true")
	}
	if !got.SelectFldWithFirstOrLastChar {
		t.Errorf("expected SelectFldWithFirstOrLastChar=true")
	}
	if !got.ShowBreaksInFrames {
		t.Errorf("expected ShowBreaksInFrames=true")
	}
	if !got.SpacingInWholePoints {
		t.Errorf("expected SpacingInWholePoints=true")
	}
	if !got.SubFontBySize {
		t.Errorf("expected SubFontBySize=true")
	}
	if !got.SuppressBottomSpacing {
		t.Errorf("expected SuppressBottomSpacing=true")
	}
	if !got.SuppressSpBfAfterPgBrk {
		t.Errorf("expected SuppressSpBfAfterPgBrk=true")
	}
	if !got.SuppressSpacingAtTopOfPage {
		t.Errorf("expected SuppressSpacingAtTopOfPage=true")
	}
	if !got.SuppressTopSpacing {
		t.Errorf("expected SuppressTopSpacing=true")
	}
	if !got.SuppressTopSpacingWP {
		t.Errorf("expected SuppressTopSpacingWP=true")
	}
	if !got.SwapBordersFacingPages {
		t.Errorf("expected SwapBordersFacingPages=true")
	}
	if !got.TruncateFontHeightsLikeWP6 {
		t.Errorf("expected TruncateFontHeightsLikeWP6=true")
	}
	if !got.UseFELayout {
		t.Errorf("expected UseFELayout=true")
	}
	if !got.UsePrinterMetrics {
		t.Errorf("expected UsePrinterMetrics=true")
	}
	if !got.UseSingleBorderforContiguousCells {
		t.Errorf("expected UseSingleBorderforContiguousCells=true")
	}
	if !got.UseWord97LineBreakRules {
		t.Errorf("expected UseWord97LineBreakRules=true")
	}
	if !got.WpJustification {
		t.Errorf("expected WpJustification=true")
	}
	if !got.WpSpaceWidth {
		t.Errorf("expected WpSpaceWidth=true")
	}
	if !got.WrapTrailSpaces {
		t.Errorf("expected WrapTrailSpaces=true")
	}
}

func TestResolveTableBordersStartEnd(t *testing.T) {
	b := &parts.TableBorders{
		Start: &parts.Border{Val: "single", Color: "FF0000"},
		End:   &parts.Border{Val: "double", Color: "00FF00"},
	}
	out := resolveTableBorders(b)
	if out == nil {
		t.Fatal("expected non-nil")
	}
	if out.Start == nil || out.Start.Val != "single" {
		t.Errorf("Start = %+v, want single", out.Start)
	}
	if out.End == nil || out.End.Val != "double" {
		t.Errorf("End = %+v, want double", out.End)
	}
}

func TestResolveCellBordersStartEnd(t *testing.T) {
	b := &parts.TableCellBorders{
		Start: &parts.Border{Val: "single", Color: "FF0000"},
		End:   &parts.Border{Val: "double", Color: "00FF00"},
	}
	out := resolveCellBorders(b)
	if out == nil {
		t.Fatal("expected non-nil")
	}
	if out.Start == nil || out.Start.Val != "single" {
		t.Errorf("Start = %+v, want single", out.Start)
	}
	if out.End == nil || out.End.Val != "double" {
		t.Errorf("End = %+v, want double", out.End)
	}
}

func TestVmlArcGeometry(t *testing.T) {
	tests := []struct {
		name        string
		startAngle  string
		endAngle    string
		wantPathCmd int // expected number of path commands
		wantArc     bool
	}{
		{"quarter circle top-right", "0", "90", 4, true},
		{"quarter circle right-bottom", "90", "180", 4, true},
		{"half circle", "0", "180", 4, true},
		{"three-quarter circle", "0", "270", 4, true},
		{"full circle", "0", "360", 4, true},
		{"negative angles", "-90", "90", 4, true},
		{"empty start", "", "90", 0, false},
		{"empty end", "0", "", 0, false},
		{"invalid start", "abc", "90", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			geom := vmlArcGeometry(tt.startAngle, tt.endAngle)
			if !tt.wantArc {
				if geom != nil {
					t.Fatalf("expected nil geometry for invalid input")
				}
				return
			}
			if geom == nil {
				t.Fatalf("expected non-nil geometry")
			}
			cmds := geom.Paths[0].Commands
			if len(cmds) != tt.wantPathCmd {
				t.Fatalf("len(commands) = %d, want %d", len(cmds), tt.wantPathCmd)
			}
			if cmds[0].Type != "M" {
				t.Fatalf("first command = %s, want M", cmds[0].Type)
			}
			if cmds[1].Type != "L" {
				t.Fatalf("second command = %s, want L", cmds[1].Type)
			}
			if cmds[2].Type != "A" {
				t.Fatalf("third command = %s, want A", cmds[2].Type)
			}
			if cmds[3].Type != "Z" {
				t.Fatalf("fourth command = %s, want Z", cmds[3].Type)
			}
		})
	}
}

func TestVmlArcGeometryAngles(t *testing.T) {
	// 0° at 12 o'clock, 90° at 3 o'clock.
	geom := vmlArcGeometry("0", "90")
	if geom == nil {
		t.Fatal("expected non-nil geometry")
	}
	cmds := geom.Paths[0].Commands

	// Start point at 0° should be top center: (0.5, 0)
	startPt := cmds[1].Points[0]
	if math.Abs(startPt.X-0.5) > 1e-9 || math.Abs(startPt.Y-0.0) > 1e-9 {
		t.Fatalf("start point = (%.4f, %.4f), want (0.5, 0.0)", startPt.X, startPt.Y)
	}

	// End point at 90° should be right center: (1, 0.5)
	endPt := cmds[2].Points[0]
	if math.Abs(endPt.X-1.0) > 1e-9 || math.Abs(endPt.Y-0.5) > 1e-9 {
		t.Fatalf("end point = (%.4f, %.4f), want (1.0, 0.5)", endPt.X, endPt.Y)
	}

	// Arc args: rx=0.5, ry=0.5, rotation=0, large-arc=0, sweep=1
	args := cmds[2].Args
	if len(args) < 5 || args[3] != 0 || args[4] != 1 {
		t.Fatalf("arc args = %v, want large-arc=0, sweep=1", args)
	}
}

func TestVmlPathGeometry(t *testing.T) {
	tests := []struct {
		name       string
		pathData   string
		coordSize  string
		wantCmds   []string
		wantPoints []dml.Point
	}{
		{
			name:     "simple rectangle",
			pathData: "m 0,0 l 21600,0 21600,21600 0,21600 x e",
			wantCmds: []string{"M", "L", "Z"},
			wantPoints: []dml.Point{
				{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1},
			},
		},
		{
			name:     "explicit l command",
			pathData: "m 1000,1000 l 2000,1000 2000,2000 1000,2000 x",
			wantCmds: []string{"M", "L", "Z"},
			wantPoints: []dml.Point{
				{X: 1000.0 / 21600, Y: 1000.0 / 21600},
				{X: 2000.0 / 21600, Y: 1000.0 / 21600},
				{X: 2000.0 / 21600, Y: 2000.0 / 21600},
				{X: 1000.0 / 21600, Y: 2000.0 / 21600},
			},
		},
		{
			name:     "cubic bezier",
			pathData: "m 0,0 c 1000,0 2000,1000 2000,2000 e",
			wantCmds: []string{"M", "C"},
			wantPoints: []dml.Point{
				{X: 0, Y: 0},
				{X: 1000.0 / 21600, Y: 0},
				{X: 2000.0 / 21600, Y: 1000.0 / 21600},
				{X: 2000.0 / 21600, Y: 2000.0 / 21600},
			},
		},
		{
			name:      "custom coordSize",
			pathData:  "m 0,0 l 100,0 100,100 0,100 x",
			coordSize: "100,100",
			wantCmds:  []string{"M", "L", "Z"},
			wantPoints: []dml.Point{
				{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1},
			},
		},
		{
			name:     "empty path",
			pathData: "",
			wantCmds: nil,
		},
		{
			name:     "only end command",
			pathData: "e",
			wantCmds: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			geom := vmlPathGeometry(tt.pathData, tt.coordSize, "")
			if tt.wantCmds == nil {
				if geom != nil {
					t.Fatalf("expected nil geometry for empty/invalid path")
				}
				return
			}
			if geom == nil {
				t.Fatalf("expected non-nil geometry")
			}
			cmds := geom.Paths[0].Commands
			var gotCmds []string
			var gotPoints []dml.Point
			for _, c := range cmds {
				gotCmds = append(gotCmds, c.Type)
				gotPoints = append(gotPoints, c.Points...)
			}
			if len(gotCmds) != len(tt.wantCmds) {
				t.Fatalf("commands = %v, want %v", gotCmds, tt.wantCmds)
			}
			for i := range gotCmds {
				if gotCmds[i] != tt.wantCmds[i] {
					t.Fatalf("command[%d] = %s, want %s", i, gotCmds[i], tt.wantCmds[i])
				}
			}
			if len(gotPoints) != len(tt.wantPoints) {
				t.Fatalf("point count = %d, want %d", len(gotPoints), len(tt.wantPoints))
			}
			for i := range gotPoints {
				if math.Abs(gotPoints[i].X-tt.wantPoints[i].X) > 1e-9 || math.Abs(gotPoints[i].Y-tt.wantPoints[i].Y) > 1e-9 {
					t.Fatalf("point[%d] = (%.6f, %.6f), want (%.6f, %.6f)", i, gotPoints[i].X, gotPoints[i].Y, tt.wantPoints[i].X, tt.wantPoints[i].Y)
				}
			}
		})
	}
}

func TestRenderSmartArtSVGRichGeometryFillTextAndConnectorStyle(t *testing.T) {
	shapes := []*smartartrender.SmartArtShape{
		{
			ID: "root", Type: "diamond", Width: 160, Height: 90, RotationDeg: 15,
			Fill:        dml.GradientFill{GradientType: dml.GradientTypeLinear, Angle: 45, Stops: []dml.GradientStop{{Position: 0, Color: "FF0000", Alpha: 1}, {Position: 1, Color: "0000FF", Alpha: 1}}},
			Line:        &dml.LineStyle{Color: "112233", Width: 2},
			Paragraphs:  []*smartart.TextParagraph{{Align: "ctr", Runs: []smartart.TextRun{{Text: "Chief", Bold: true, FontSize: 1800, Color: "FFFFFF"}}}},
			Connections: []smartartrender.Connection{{ToID: "child"}}, DrawingOrder: 1,
		},
		{ID: "child", Type: "ellipse", X: 220, Y: 120, Width: 140, Height: 80, Fill: dml.PictureFill{ImageData: []byte("PNG"), ContentType: "image/png"}, Text: "Child", DrawingOrder: 2},
	}
	connector := &dml.LineStyle{Color: "00AA00", Width: 2.5, Dash: dml.DashPatternDash}
	out := renderSmartArtSVGWithStyle(shapes, 640, 360, connector)
	for _, want := range []string{`<linearGradient id="sa-fill-0"`, `rotate(15.0000`, `font-weight="700"`, `stroke="#00AA00"`, `stroke-dasharray="3, 3"`, `<pattern id="sa-fill-1"`, `data:image/png;base64,`} {
		if !strings.Contains(out, want) {
			t.Fatalf("SmartArt SVG missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, `<path d=`) {
		t.Fatalf("expected shared preset geometry path, got:\n%s", out)
	}
}

func TestRenderSmartArtSVGUsesExactDrawingConnectorGeometry(t *testing.T) {
	shapes := []*smartartrender.SmartArtShape{
		{ID: "a", Type: "rect", Width: 100, Height: 60, Fill: dml.SolidFill{Color: "CCCCCC", Alpha: 1}},
		{ID: "b", Type: "rect", X: 220, Y: 120, Width: 100, Height: 60, Fill: dml.SolidFill{Color: "DDDDDD", Alpha: 1}},
	}
	line := &dml.LineStyle{Color: "445566", Width: 2, Dash: dml.DashPatternDashDot, HeadEnd: &dml.Arrowhead{Type: dml.ArrowheadTriangle, Size: dml.ArrowheadSizeMedium, Length: dml.ArrowheadSizeMedium}}
	connectors := []smartArtDrawingConnector{{Type: "bentConnector3", X: 100, Y: 30, W: 120, H: 120, Adjustments: []int{40000}, Line: line}}
	out := renderSmartArtSVGFull(shapes, 640, 360, nil, connectors)
	if !strings.Contains(out, `<polyline`) {
		t.Fatalf("expected authored bent connector polyline:\n%s", out)
	}
	if !strings.Contains(out, `stroke-dasharray="3, 3, 1, 3"`) {
		t.Fatalf("expected authored connector dash style:\n%s", out)
	}
	if !strings.Contains(out, `<polygon`) {
		t.Fatalf("expected authored connector arrowhead:\n%s", out)
	}
}

func TestRenderSmartArtTextHonorsBodyPropsBulletsAndAutofit(t *testing.T) {
	shape := &smartartrender.SmartArtShape{
		TextColor: "FFFFFF",
		TextBodyProps: dml.TextBodyProperties{
			Anchor: "b", Vert: "vert270", Rot: 15000,
			AutofitType: "normAutofit", AutofitFontScale: 80000,
			AutofitLineSpaceReduction: 10000,
			Columns:                   2, ColumnSpacingEMU: 127000,
			InsetL: 12700, InsetR: 12700, InsetT: 12700, InsetB: 12700,
		},
		Paragraphs: []*smartart.TextParagraph{
			{Level: 1, Align: "l", BulletChar: "•", BulletColor: "FF0000", BulletSizePct: 75000, Runs: []smartart.TextRun{{Text: "First", FontSize: 2000, FontFamily: "Aptos", Color: "00FF00", Bold: true}}},
			{Align: "l", AutoNum: true, AutoNumType: "alphaLcParenR", AutoNumStart: 2, Runs: []smartart.TextRun{{Text: "Second", FontSize: 1800}}},
		},
	}
	var b strings.Builder
	renderSmartArtText(&b, shape, 240, 120, 1)
	out := b.String()
	for _, want := range []string{`rotate(-75.0000`, `>• </tspan>`, `font-size="16.00"`, `font-weight="700"`, `>b) </tspan>`, `fill="#00FF00"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("SmartArt text output missing %q:\n%s", want, out)
		}
	}
}

func TestSmartArtPictureFillPreservesCropAndTileGeometry(t *testing.T) {
	cropFill := dml.PictureFill{ImageURL: "data:image/png;base64,AA==", Stretch: true, Crop: &dml.Rect{X: .1, Y: .2, Width: .5, Height: .4}}
	var cropDefs strings.Builder
	attr := smartArtPictureFill(cropFill, "crop", &cropDefs)
	if attr != `fill="url(#crop)"` {
		t.Fatalf("crop fill attr = %q", attr)
	}
	for _, want := range []string{`x="-0.200000"`, `y="-0.500000"`, `width="2.000000"`, `height="2.500000"`} {
		if !strings.Contains(cropDefs.String(), want) {
			t.Fatalf("crop defs missing %q: %s", want, cropDefs.String())
		}
	}

	tileFill := dml.PictureFill{ImageURL: "data:image/png;base64,AA==", Tile: true, TileScaleX: 50000, TileScaleY: 25000, TileOffsetX: 91440, TileOffsetY: 182880, TileFlip: "xy", TileAlign: "br", Alpha: .6}
	var tileDefs strings.Builder
	smartArtPictureFill(tileFill, "tile", &tileDefs)
	for _, want := range []string{`x="0.60000"`, `y="0.95000"`, `width="0.50000"`, `height="0.25000"`, `transform="translate(1 1) scale(-1 -1)"`, `opacity="0.6000"`} {
		if !strings.Contains(tileDefs.String(), want) {
			t.Fatalf("tile defs missing %q: %s", want, tileDefs.String())
		}
	}
}

func TestSmartArtSVGEffectSupportsAdvancedEffectStack(t *testing.T) {
	effects := dml.EffectList{Effects: []dml.Effect{
		dml.InnerShadow{Color: "112233", Alpha: .4, Blur: 4, Distance: 3, Angle: 45},
		dml.Blur{Radius: 2},
		dml.SoftEdge{Radius: 3},
		dml.Reflection{Alpha: .3, Blur: 2, Distance: 5, Direction: 90},
	}}
	var defs strings.Builder
	id := smartArtSVGEffect(effects, 7, &defs)
	if id != "sa-effect-7" {
		t.Fatalf("effect id = %q", id)
	}
	out := defs.String()
	for _, want := range []string{`is-shadow`, `softAlpha`, `reflFade`, `feGaussianBlur`} {
		if !strings.Contains(out, want) {
			t.Fatalf("effect defs missing %q:\n%s", want, out)
		}
	}
}
func TestNormalizeColumnsPreservesLargeAuthoredSpacing(t *testing.T) {
	got := normalizeColumns(&parts.Columns{Num: 2, Space: 360000})
	if got == nil {
		t.Fatal("normalizeColumns returned nil")
	}
	if got.Space != 360000 {
		t.Fatalf("column space = %d, want authored 360000 twips", got.Space)
	}
}

func TestBuildNumberingIndexPreservesLevelIndentGeometry(t *testing.T) {
	numbering := &parts.Numbering{
		AbstractNums: []*parts.AbstractNum{{
			ID: 4,
			Levels: []*parts.Lvl{{
				Level: 2,
				ParaProps: &parts.LvlPPr{Ind: &parts.LvlInd{
					Left: 2160, Hanging: 360, FirstLine: 120,
				}},
			}},
		}},
		Nums: []*parts.Num{{ID: 9, AbstractNumID: &parts.IntVal{Val: 4}}},
	}
	index := buildNumberingIndex(numbering, nil, nil)
	got, ok := resolveNumbering(index, 9, 2)
	if !ok {
		t.Fatal("numbering level not resolved")
	}
	if got.IndentLeft != 2160 || got.IndentHanging != 360 || got.IndentFirstLine != 120 {
		t.Fatalf("numbering indent = left:%d hanging:%d first:%d", got.IndentLeft, got.IndentHanging, got.IndentFirstLine)
	}
}

func TestNormalizeContentBlocksIncludesBlockSdtContentInFlatCollections(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused test pass")
	ctx := &resolveCtx{}
	paragraph := &parts.Paragraph{Runs: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "inside"}}}}}}
	cellParagraph := &parts.Paragraph{Runs: []*parts.Run{{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "cell"}}}}}}
	table := &parts.Table{Rows: []*parts.TableRow{{Cells: []*parts.TableCell{{Paragraphs: []*parts.Paragraph{cellParagraph}}}}}}
	blocks := []parts.ContentBlock{{
		Kind:     parts.ContentBlockSDT,
		SdtProps: &parts.SdtProperties{Type: "richText", Tag: "outer"},
		SdtBlocks: []parts.ContentBlock{
			{Kind: parts.ContentBlockParagraph, Paragraph: paragraph},
			{Kind: parts.ContentBlockTable, Table: table},
		},
	}}
	paragraphs, tables, resolvedBlocks, _ := normalizeContentBlocks(ctx, blocks, nil, nil)
	if len(paragraphs) != 1 || len(tables) != 1 || len(resolvedBlocks) != 2 {
		t.Fatalf("flat collections = paragraphs:%d tables:%d blocks:%d", len(paragraphs), len(tables), len(resolvedBlocks))
	}
	for _, block := range resolvedBlocks {
		if block.ContentControl == nil || block.ContentControl.Tag != "outer" {
			t.Fatalf("missing SDT metadata: %+v", block)
		}
	}
}

func TestNestedInlineSdtKeepsInnermostControlIdentity(t *testing.T) {
	t.Skip("not OfficeArt: excluded from the OfficeArt-focused test pass")
	ctx := &resolveCtx{}
	textRun := &parts.Run{Contents: []parts.RunContent{{Kind: parts.RunContentText, Text: &parts.Text{Value: "value"}}}}
	inner := parts.ParagraphInline{
		Kind:       parts.ParagraphInlineSDT,
		SdtProps:   &parts.SdtProperties{Type: "plainText", Tag: "inner"},
		SdtInlines: []parts.ParagraphInline{{Kind: parts.ParagraphInlineRun, Run: textRun}},
	}
	outer := parts.ParagraphInline{
		Kind:       parts.ParagraphInlineSDT,
		SdtProps:   &parts.SdtProperties{Type: "richText", Tag: "outer"},
		SdtInlines: []parts.ParagraphInline{inner},
	}
	runs := normalizeSdtInlineRuns(ctx, []parts.ParagraphInline{outer}, RunFormat{})
	if len(runs) != 1 || runs[0].ContentControl == nil || runs[0].ContentControl.Tag != "inner" {
		t.Fatalf("nested SDT identity = %+v", runs)
	}
}
