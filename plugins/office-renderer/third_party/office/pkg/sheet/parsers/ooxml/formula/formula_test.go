package formula

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func numberPtr(v float64) *float64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func TestParseSimpleFormula(t *testing.T) {
	tests := []struct {
		name     string
		formula  string
		expected string
	}{
		{"number", "42", "42"},
		{"addition", "1+2", "(1 + 2)"},
		{"subtraction", "5-3", "(5 - 3)"},
		{"multiplication", "2*3", "(2 * 3)"},
		{"division", "10/2", "(10 / 2)"},
		{"power", "2^3", "(2 ^ 3)"},
		{"mixed", "1+2*3", "(1 + (2 * 3))"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.formula)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			if got := expr.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseExternalWorkbookReference(t *testing.T) {
	for _, formula := range []string{
		`[1]Sheet2!A1`,
		`SUM([1]Sheet2!A1:[1]Sheet2!A3)`,
		`'[Book.xlsx]Lookup Sheet'!$B$2`,
	} {
		if _, err := Parse(formula); err != nil {
			t.Fatalf("Parse(%q) error: %v", formula, err)
		}
	}
}

func TestEvaluateSimpleFormula(t *testing.T) {
	tests := []struct {
		name     string
		formula  string
		expected float64
	}{
		{"addition", "1+2", 3},
		{"subtraction", "5-3", 2},
		{"multiplication", "2*3", 6},
		{"division", "10/2", 5},
		{"power", "2^3", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if !result.IsNumber() {
				t.Fatalf("Expected number, got %v", result.Type)
			}
			if result.Num != tt.expected {
				t.Errorf("Num = %v, want %v", result.Num, tt.expected)
			}
		})
	}
}

func TestEvaluateFunctions(t *testing.T) {
	tests := []struct {
		name     string
		formula  string
		expected float64
	}{
		{"sum", "SUM(1,2,3)", 6},
		{"average", "AVERAGE(2,4,6)", 4},
		{"count", "COUNT(1,2,\"text\")", 2},
		{"max", "MAX(3,1,4,1,5)", 5},
		{"min", "MIN(3,1,4,1,5)", 1},
		{"abs", "ABS(-5)", 5},
		{"round", "ROUND(3.14159,2)", 3.14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("Got error: %s", result.Err.String())
			}
			if !result.IsNumber() {
				t.Fatalf("Expected number, got %v", result.Type)
			}
			// Allow small floating point differences
			if result.Num < tt.expected-0.001 || result.Num > tt.expected+0.001 {
				t.Errorf("Num = %v, want %v", result.Num, tt.expected)
			}
		})
	}
}

func TestEvaluateLogic(t *testing.T) {
	tests := []struct {
		name     string
		formula  string
		expected bool
	}{
		{"true", "TRUE()", true},
		{"false", "FALSE()", false},
		{"and", "AND(TRUE(),TRUE())", true},
		{"or", "OR(FALSE(),TRUE())", true},
		{"not", "NOT(FALSE())", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("Got error: %s", result.Err.String())
			}
			if !result.IsBool() {
				t.Fatalf("Expected bool, got %v", result.Type)
			}
			if result.Bool != tt.expected {
				t.Errorf("Bool = %v, want %v", result.Bool, tt.expected)
			}
		})
	}
}

func TestDateComponentFunctions(t *testing.T) {
	// DATE(2024, 3, 15) → serial for 2024-03-15
	// Excel epoch: Dec 30 1899. 2024-03-15 serial = 45366
	const serial2024_03_15 = 45366.0

	result, err := Evaluate("DATE(2024,3,15)")
	if err != nil {
		t.Fatalf("DATE error: %v", err)
	}
	if !result.IsNumber() || result.Num != serial2024_03_15 {
		t.Fatalf("DATE(2024,3,15) = %v, want %v", result.Num, serial2024_03_15)
	}

	for _, tt := range []struct {
		formula string
		want    float64
	}{
		{fmt.Sprintf("YEAR(%v)", serial2024_03_15), 2024},
		{fmt.Sprintf("MONTH(%v)", serial2024_03_15), 3},
		{fmt.Sprintf("DAY(%v)", serial2024_03_15), 15},
		// WEEKDAY: 2024-03-15 is a Friday → type 1 → 6
		{fmt.Sprintf("WEEKDAY(%v,1)", serial2024_03_15), 6},
		// WEEKDAY type 2 → Monday=1, so Friday=5
		{fmt.Sprintf("WEEKDAY(%v,2)", serial2024_03_15), 5},
		// DAYS: 5 days apart
		{"DAYS(DATE(2024,3,20),DATE(2024,3,15))", 5},
	} {
		t.Run(tt.formula, func(t *testing.T) {
			r, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if r.IsError() {
				t.Fatalf("got error: %s", r.Err.String())
			}
			if !r.IsNumber() || r.Num != tt.want {
				t.Errorf("result = %v, want %v", r.Num, tt.want)
			}
		})
	}

	// HOUR/MINUTE/SECOND: serial with fractional day = 0.5 → noon
	// 0.5 days = 12:00:00
	hourResult, err := Evaluate("HOUR(DATE(2024,3,15)+0.5)")
	if err != nil {
		t.Fatalf("HOUR error: %v", err)
	}
	if !hourResult.IsNumber() || hourResult.Num != 12 {
		t.Fatalf("HOUR result = %v, want 12", hourResult.Num)
	}
}

func TestMathInfoGapFunctions(t *testing.T) {
	tests := []struct {
		formula string
		want    float64
	}{
		{"TRUNC(3.7)", 3},
		{"TRUNC(-3.7)", -3},
		{"TRUNC(3.755,2)", 3.75},
		{"SIGN(5)", 1},
		{"SIGN(-5)", -1},
		{"SIGN(0)", 0},
		{"LOG(100)", 2},
		{"LOG(8,2)", 3},
		{"LOG10(1000)", 3},
		{"LN(1)", 0},
		{"EXP(0)", 1},
	}

	for _, tt := range tests {
		t.Run(tt.formula, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("got error: %s", result.Err.String())
			}
			if !result.IsNumber() {
				t.Fatalf("not a number: %v", result)
			}
			if math.Abs(result.Num-tt.want) > 1e-9 {
				t.Errorf("result = %v, want %v", result.Num, tt.want)
			}
		})
	}

	// RANDBETWEEN: result must be within [3,7]
	rb, err := Evaluate("RANDBETWEEN(3,7)")
	if err != nil {
		t.Fatalf("RANDBETWEEN error: %v", err)
	}
	if !rb.IsNumber() || rb.Num < 3 || rb.Num > 7 {
		t.Fatalf("RANDBETWEEN result %v out of [3,7]", rb.Num)
	}

	// XOR
	xorTrue, _ := Evaluate("XOR(TRUE,FALSE)")
	if !xorTrue.IsBool() || !xorTrue.Bool {
		t.Fatalf("XOR(T,F) = %v, want TRUE", xorTrue)
	}
	xorFalse, _ := Evaluate("XOR(TRUE,TRUE)")
	if !xorFalse.IsBool() || xorFalse.Bool {
		t.Fatalf("XOR(T,T) = %v, want FALSE", xorFalse)
	}

	// ROWS / COLUMNS
	engine := NewEngine()
	engine.SetFormula("A1", "ROWS(B1:B5)")
	engine.SetFormula("A2", "COLUMNS(B1:E1)")
	engine.SetFormula("A3", "ROWS(INDEX(B1:D3,0,2))")
	engine.SetFormula("A4", "COLUMNS(INDEX(B1:D3,2,0))")
	engine.SetFormula("A5", "ROWS(TRANSPOSE(B1:C3))")
	engine.SetFormula("A6", "COLUMNS(OFFSET(B1,0,0,2,3))")
	engine.SetFormula("A7", "COLUMNS(CHOOSECOLS(B1:D3,1,3))")
	engine.SetFormula("A8", "ROWS(CHOOSEROWS(B1:D4,1,3,4))")
	engine.SetFormula("A9", "ROWS(TAKE(B1:D4,2,2))")
	engine.SetFormula("A10", "COLUMNS(DROP(B1:D4,1,1))")
	if got := engine.GetCell("A1"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("ROWS(B1:B5) = %v, want 5", got.Num)
	}
	if got := engine.GetCell("A2"); !got.IsNumber() || got.Num != 4 {
		t.Fatalf("COLUMNS(B1:E1) = %v, want 4", got.Num)
	}
	for ref, want := range map[string]float64{"A3": 3, "A4": 3, "A5": 2, "A6": 3, "A7": 2, "A8": 3, "A9": 2, "A10": 2} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestTextGapFunctions(t *testing.T) {
	tests := []struct {
		name    string
		formula string
		want    string
	}{
		{name: "proper", formula: `PROPER("hello world")`, want: "Hello World"},
		{name: "proper mixed", formula: `PROPER("jOHN doe")`, want: "John Doe"},
		{name: "rept", formula: `REPT("ab",3)`, want: "ababab"},
		{name: "rept zero", formula: `REPT("x",0)`, want: ""},
		{name: "exact true", formula: `EXACT("ABC","ABC")`, want: "TRUE"},
		{name: "exact false case", formula: `EXACT("abc","ABC")`, want: "FALSE"},
		{name: "char", formula: `CHAR(65)`, want: "A"},
		{name: "code", formula: `CODE("A")`, want: "65"},
		{name: "find", formula: `FIND("lo","hello")`, want: "4"},
		{name: "find start", formula: `FIND("l","hello",4)`, want: "4"},
		{name: "search ci", formula: `SEARCH("LO","hello")`, want: "4"},
		{name: "substitute all", formula: `SUBSTITUTE("aXbXc","X","-")`, want: "a-b-c"},
		{name: "substitute inst", formula: `SUBSTITUTE("aXbXc","X","-",2)`, want: "aXb-c"},
		{name: "replace", formula: `REPLACE("abcde",2,3,"ZZ")`, want: "aZZe"},
		{name: "textjoin", formula: `TEXTJOIN(",",TRUE,"a","","b")`, want: "a,b"},
		{name: "textjoin no skip", formula: `TEXTJOIN(",",FALSE,"a","","b")`, want: "a,,b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("Got error: %s", result.Err.String())
			}
			var got string
			if result.IsBool() {
				if result.Bool {
					got = "TRUE"
				} else {
					got = "FALSE"
				}
			} else {
				got = result.ToText()
			}
			if got != tt.want {
				t.Errorf("result = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEngineTreatsChooseAsReferenceForInfoFunctions(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("C1", Number(10))
	engine.SetCell("C2", Number(20))
	engine.SetCell("E1", Number(2))
	engine.SetFormula("G1", `ISREF(CHOOSE(1,A1:A2,C1:C2))`)
	engine.SetFormula("G2", `AREAS(CHOOSE(1,A1:A2,C1:C2))`)
	engine.SetFormula("G3", `ISREF(CHOOSE(E1,A1:A2,C1:C2))`)
	engine.SetFormula("G4", `ISREF(CHOOSE(1,10,20))`)
	engine.SetFormula("G5", `ISREF(CHOOSE(SEQUENCE(2,1),A1,A2))`)
	engine.SetFormula("G6", `ISREF(CHOOSE(SEQUENCE(2,1),A1,10))`)

	if got := engine.GetCell("G1"); !got.IsBool() || !got.Bool {
		t.Fatalf("G1 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("G2"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("G2 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("G3"); !got.IsBool() || !got.Bool {
		t.Fatalf("G3 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("G4"); !got.IsBool() || got.Bool {
		t.Fatalf("G4 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("G5"); !got.IsBool() || !got.Bool {
		t.Fatalf("G5 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("G6"); !got.IsBool() || got.Bool {
		t.Fatalf("G6 = %#v, want FALSE", got)
	}
}

func TestEngineIsRefValidatesReferenceFunctionArguments(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetFormula("H1", `ISREF(OFFSET(A1,0,0))`)
	engine.SetFormula("H2", `ISREF(OFFSET(10,0,0))`)
	engine.SetFormula("H3", `ISREF(CHOOSECOLS(A1:A2,1))`)
	engine.SetFormula("H4", `ISREF(CHOOSECOLS(10,1))`)
	engine.SetFormula("H5", `ISREF(IF(TRUE,A1,10))`)
	engine.SetFormula("H6", `ISREF(IF(FALSE,A1,10))`)
	engine.SetFormula("H7", `ISREF(IF(SEQUENCE(2,1),A1,A2))`)
	engine.SetFormula("H8", `ISREF(IF(SEQUENCE(2,1)-1,A1,10))`)
	engine.SetFormula("H9", `AREAS(IF(TRUE,A1:A2,C1:C2))`)
	engine.SetFormula("H10", `ISREF(IFS(TRUE,A1,FALSE,10))`)
	engine.SetFormula("H11", `ISREF(IFS(FALSE,A1,TRUE,10))`)
	engine.SetFormula("H12", `ISREF(SWITCH(1,1,A1,2,10))`)
	engine.SetFormula("H13", `ISREF(SWITCH(2,1,A1,2,10))`)
	engine.SetFormula("H14", `AREAS(SWITCH(1,1,A1:A2,2,C1:C2))`)
	engine.SetFormula("H15", `ISREF(IFERROR(A1,10))`)
	engine.SetFormula("H16", `ISREF(IFERROR(UNKNOWNFUNC(),A1))`)
	engine.SetFormula("H17", `ISREF(IFNA(#N/A,A1))`)
	engine.SetFormula("H18", `ISREF(IFNA(A1,10))`)
	engine.SetFormula("H19", `ISREF(IFS(SEQUENCE(2,1)-1,A1,TRUE,A2))`)
	engine.SetFormula("H20", `ISREF(IFS(SEQUENCE(2,1)-1,A1,TRUE,10))`)
	engine.SetFormula("H21", `ISREF(SWITCH(SEQUENCE(2,1),1,A1,2,A2))`)
	engine.SetFormula("H22", `ISREF(SWITCH(SEQUENCE(2,1),1,A1,2,10))`)
	engine.SetFormula("H23", `AREAS(IFS(SEQUENCE(2,1)-1,A1:A2,TRUE,C1:C2))`)
	engine.SetFormula("H24", `AREAS(SWITCH(SEQUENCE(2,1),1,A1:A2,2,C1:C2))`)
	engine.SetFormula("H25", `AREAS(IFERROR(A1:A2,C1:C2))`)
	engine.SetFormula("H26", `AREAS(IFNA(A1:A2,C1:C2))`)
	engine.SetFormula("H27", `AREAS(IFERROR(IF(SEQUENCE(2,1)-1,A1:A2,UNKNOWNFUNC()),C1:C2))`)
	engine.SetFormula("H28", `AREAS(IFNA(IF(SEQUENCE(2,1)-1,A1:A2,#N/A),C1:C2))`)
	engine.SetFormula("H29", `ISREF(IFS(SEQUENCE(1,2)-1,A1,TRUE,A2))`)
	engine.SetFormula("H30", `ISREF(IFS(SEQUENCE(1,2)-1,A1,TRUE,10))`)
	engine.SetFormula("H31", `ISREF(SWITCH(SEQUENCE(1,2),1,A1,2,A2))`)
	engine.SetFormula("H32", `ISREF(SWITCH(SEQUENCE(1,2),1,A1,2,10))`)
	engine.SetFormula("H33", `AREAS(IFERROR(IF(SEQUENCE(1,2)-1,A1:B1,UNKNOWNFUNC()),C1:D1))`)
	engine.SetFormula("H34", `AREAS(IFNA(IF(SEQUENCE(1,2)-1,A1:B1,#N/A),C1:D1))`)
	engine.SetFormula("H35", `ISREF(IFERROR(IF(SEQUENCE(1,2)-1,A1,UNKNOWNFUNC()),10))`)
	engine.SetFormula("H36", `ISREF(IFNA(IF(SEQUENCE(1,2)-1,A1,#N/A),10))`)
	engine.SetFormula("H37", `ISREF(IFERROR(IF(SEQUENCE(1,2)-1,A1,UNKNOWNFUNC()),A2))`)
	engine.SetFormula("H38", `ISREF(IFNA(IF(SEQUENCE(1,2)-1,A1,#N/A),A2))`)
	engine.SetFormula("H39", `ISREF(IF(SEQUENCE(1,2)-1,INDEX(A1:B2,SEQUENCE(1,2)-1,1),A1))`)
	engine.SetFormula("H40", `ISREF(CHOOSE(SEQUENCE(1,2),A1,INDEX(A1:B2,SEQUENCE(1,2)-1,1)))`)
	engine.SetFormula("H41", `AREAS(CHOOSE(SEQUENCE(1,2),A1,IF(SEQUENCE(1,2)-1,A2,10)))`)
	engine.SetFormula("H42", `AREAS(CHOOSE(SEQUENCE(2,1),A1,IF(SEQUENCE(2,1)-1,A2,10)))`)
	engine.SetFormula("H43", `ISREF(CHOOSE(MOD(SEQUENCE(2,2)-1,2)+1,A1,IF(SEQUENCE(2,2)-1,A2,10)))`)
	engine.SetFormula("H44", `AREAS(CHOOSE(MOD(SEQUENCE(2,2)-1,2)+1,A1,IF(SEQUENCE(2,2)-1,A2,10)))`)
	engine.SetFormula("H45", `ISREF(IFS(SEQUENCE(2,2)=1,A1,TRUE,IF(SEQUENCE(2,2)-1,A2,10)))`)
	engine.SetFormula("H46", `AREAS(IFS(SEQUENCE(2,2)=1,A1,TRUE,IF(SEQUENCE(2,2)-1,A2,10)))`)
	engine.SetFormula("H47", `ISREF(SWITCH(IF(SEQUENCE(2,2)=1,1,2),1,A1,2,IF(SEQUENCE(2,2)-1,A2,10)))`)
	engine.SetFormula("H48", `AREAS(SWITCH(IF(SEQUENCE(2,2)=1,1,2),1,A1,2,IF(SEQUENCE(2,2)-1,A2,10)))`)
	engine.SetFormula("H49", `ISREF(IFERROR(IF(SEQUENCE(2,2)-1,A1,UNKNOWNFUNC()),IF(SEQUENCE(2,2),A2,10)))`)
	engine.SetFormula("H50", `AREAS(IFERROR(IF(SEQUENCE(2,2)-1,A1:B2,UNKNOWNFUNC()),IF(SEQUENCE(2,2),C1:D2,10)))`)
	engine.SetFormula("H51", `ISREF(IFNA(IF(SEQUENCE(2,2)-1,A1,#N/A),IF(SEQUENCE(2,2),A2,10)))`)
	engine.SetFormula("H52", `AREAS(IFNA(IF(SEQUENCE(2,2)-1,A1:B2,#N/A),IF(SEQUENCE(2,2),C1:D2,10)))`)

	if got := engine.GetCell("H1"); !got.IsBool() || !got.Bool {
		t.Fatalf("H1 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H2"); !got.IsBool() || got.Bool {
		t.Fatalf("H2 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H3"); !got.IsBool() || !got.Bool {
		t.Fatalf("H3 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H4"); !got.IsBool() || got.Bool {
		t.Fatalf("H4 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H5"); !got.IsBool() || !got.Bool {
		t.Fatalf("H5 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H6"); !got.IsBool() || got.Bool {
		t.Fatalf("H6 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H7"); !got.IsBool() || !got.Bool {
		t.Fatalf("H7 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H8"); !got.IsBool() || got.Bool {
		t.Fatalf("H8 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H9"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H9 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H10"); !got.IsBool() || !got.Bool {
		t.Fatalf("H10 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H11"); !got.IsBool() || got.Bool {
		t.Fatalf("H11 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H12"); !got.IsBool() || !got.Bool {
		t.Fatalf("H12 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H13"); !got.IsBool() || got.Bool {
		t.Fatalf("H13 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H14"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H14 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H15"); !got.IsBool() || !got.Bool {
		t.Fatalf("H15 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H16"); !got.IsBool() || !got.Bool {
		t.Fatalf("H16 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H17"); !got.IsBool() || !got.Bool {
		t.Fatalf("H17 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H18"); !got.IsBool() || !got.Bool {
		t.Fatalf("H18 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H19"); !got.IsBool() || !got.Bool {
		t.Fatalf("H19 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H20"); !got.IsBool() || got.Bool {
		t.Fatalf("H20 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H21"); !got.IsBool() || !got.Bool {
		t.Fatalf("H21 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H22"); !got.IsBool() || got.Bool {
		t.Fatalf("H22 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H23"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H23 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H24"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H24 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H25"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H25 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H26"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H26 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H27"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H27 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H28"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H28 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H29"); !got.IsBool() || !got.Bool {
		t.Fatalf("H29 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H30"); !got.IsBool() || got.Bool {
		t.Fatalf("H30 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H31"); !got.IsBool() || !got.Bool {
		t.Fatalf("H31 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H32"); !got.IsBool() || got.Bool {
		t.Fatalf("H32 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H33"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H33 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H34"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H34 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H35"); !got.IsBool() || got.Bool {
		t.Fatalf("H35 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H36"); !got.IsBool() || got.Bool {
		t.Fatalf("H36 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("H37"); !got.IsBool() || !got.Bool {
		t.Fatalf("H37 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H38"); !got.IsBool() || !got.Bool {
		t.Fatalf("H38 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H39"); !got.IsBool() || !got.Bool {
		t.Fatalf("H39 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H40"); !got.IsBool() || !got.Bool {
		t.Fatalf("H40 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H41"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H41 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H42"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H42 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H43"); !got.IsBool() || !got.Bool {
		t.Fatalf("H43 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H44"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H44 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H45"); !got.IsBool() || !got.Bool {
		t.Fatalf("H45 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H46"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H46 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H47"); !got.IsBool() || !got.Bool {
		t.Fatalf("H47 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H48"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H48 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H49"); !got.IsBool() || !got.Bool {
		t.Fatalf("H49 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H50"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H50 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("H51"); !got.IsBool() || !got.Bool {
		t.Fatalf("H51 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("H52"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("H52 = %#v, want numeric 1", got)
	}
}

func TestEngineEvaluatesArrayDrivenIfsAndSwitch2D(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(2))
	engine.SetCell("A2", Number(3))
	engine.SetCell("B2", Number(4))

	engine.SetFormula("D1", `SWITCH(A1:B2,1,"one",2,"two","other")`)
	engine.SetFormula("G1", `IFS(A1:B2<=2,10,A1:B2=3,20,TRUE,30)`)

	for ref, want := range map[string]string{"D1": "one", "E1": "two", "D2": "other", "E2": "other"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %q", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 10, "G2": 20, "H2": 30} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestLookupCriteriaAndStatGapFunctions(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("apple"))
	engine.SetCell("A2", Text("banana"))
	engine.SetCell("A3", Text("carrot"))
	engine.SetCell("B1", Number(5))
	engine.SetCell("B2", Number(7))
	engine.SetCell("B3", Number(9))
	engine.SetCell("D1", Text("Q1"))
	engine.SetCell("E1", Text("Q2"))
	engine.SetCell("F1", Text("Q3"))
	engine.SetCell("D2", Number(10))
	engine.SetCell("E2", Number(20))
	engine.SetCell("F2", Number(30))
	engine.SetCell("H1", Number(10))
	engine.SetCell("H2", Number(20))
	engine.SetCell("H3", Number(30))
	engine.SetCell("H4", Number(5))
	engine.SetCell("I1", Text("A"))
	engine.SetCell("I2", Text("B"))
	engine.SetCell("I3", Text("A"))
	engine.SetCell("I4", Text("B"))

	engine.SetFormula("K1", `VLOOKUP("banana",A1:B3,2,FALSE)`)
	engine.SetFormula("K2", `VLOOKUP("b*",A1:B3,2,FALSE)`)
	engine.SetFormula("K3", `HLOOKUP("Q2",D1:F2,2,FALSE)`)
	engine.SetFormula("K4", `MINIFS(H1:H4,I1:I4,"A")`)
	engine.SetFormula("K5", `MAXIFS(H1:H4,I1:I4,"B")`)
	engine.SetFormula("K6", `MAXA(TRUE(),FALSE(),-2)`)
	engine.SetFormula("K7", `MINA(TRUE(),FALSE(),-2)`)

	for ref, want := range map[string]float64{
		"K1": 7,
		"K2": 7,
		"K3": 20,
		"K4": 10,
		"K5": 20,
		"K6": 1,
		"K7": -2,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}

	for _, tt := range []struct {
		formula string
		want    float64
	}{
		{`STDEV.S(1,2,3,4)`, math.Sqrt(5.0 / 3.0)},
		{`STDEV.P(1,2,3,4)`, math.Sqrt(1.25)},
		{`VAR.S(1,2,3,4)`, 5.0 / 3.0},
		{`VAR.P(1,2,3,4)`, 1.25},
		{`GCD(24,18)`, 6},
		{`LCM(6,8)`, 24},
		{`FACT(5)`, 120},
		{`COMBIN(5,2)`, 10},
		{`PERMUT(5,2)`, 20},
		{`MROUND(10,3)`, 9},
		{`QUOTIENT(7,3)`, 2},
	} {
		t.Run(tt.formula, func(t *testing.T) {
			got, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if got.IsError() {
				t.Fatalf("got error: %s", got.Err.String())
			}
			if !got.IsNumber() || math.Abs(got.Num-tt.want) > 1e-9 {
				t.Fatalf("%s = %#v, want numeric %v", tt.formula, got, tt.want)
			}
		})
	}
}

func TestTrigDateTimeAndTextFormattingGapFunctions(t *testing.T) {
	for _, tt := range []struct {
		formula string
		want    float64
	}{
		{`SIN(RADIANS(30))`, 0.5},
		{`DEGREES(ATAN(1))`, 45},
		{`ATAN2(1,1)`, math.Pi / 4},
		{`DAY(EDATE(DATE(2024,1,31),1))`, 29},
		{`MONTH(EDATE(DATE(2024,1,31),1))`, 2},
		{`DAY(EOMONTH(DATE(2024,1,15),1))`, 29},
		{`MONTH(EOMONTH(DATE(2024,1,15),1))`, 2},
		{`TIME(1,30,0)`, 1.5 / 24},
		{`DATEVALUE("2024-03-15")`, 45366},
		{`HOUR(TIMEVALUE("15:04:05"))`, 15},
		{`MINUTE(TIMEVALUE("15:04:05"))`, 4},
		{`SECOND(TIMEVALUE("15:04:05"))`, 5},
		{`DATEDIF(DATE(2024,1,15),DATE(2024,3,20),"M")`, 2},
		{`WEEKNUM(DATE(2024,1,1),1)`, 1},
		{`ISOWEEKNUM(DATE(2024,1,4))`, 1},
		{`DAY(WORKDAY(DATE(2024,3,1),1))`, 4},
		{`NETWORKDAYS(DATE(2024,3,1),DATE(2024,3,5))`, 3},
		{`NUMBERVALUE("1,234.5",".",",")`, 1234.5},
	} {
		t.Run(tt.formula, func(t *testing.T) {
			got, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if got.IsError() {
				t.Fatalf("got error: %s", got.Err.String())
			}
			if !got.IsNumber() || math.Abs(got.Num-tt.want) > 1e-9 {
				t.Fatalf("%s = %#v, want numeric %v", tt.formula, got, tt.want)
			}
		})
	}

	for _, tt := range []struct {
		formula string
		want    string
	}{
		{`CLEAN(CHAR(10))`, ""},
		{`FIXED(1234.567,1,FALSE)`, "1,234.6"},
		{`DOLLAR(-1234.5,1)`, "($1,234.5)"},
		{`TEXTBEFORE("alpha-beta-gamma","-",2)`, "alpha-beta"},
		{`TEXTAFTER("alpha-beta-gamma","-",2)`, "gamma"},
	} {
		t.Run(tt.formula, func(t *testing.T) {
			got, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if got.IsError() {
				t.Fatalf("got error: %s", got.Err.String())
			}
			if !got.IsText() || got.Text != tt.want {
				t.Fatalf("%s = %#v, want text %q", tt.formula, got, tt.want)
			}
		})
	}
}

func TestFinancialGapFunctions(t *testing.T) {
	for _, tt := range []struct {
		formula string
		want    float64
	}{
		{`PMT(0.1/12,12,1000)`, -87.91588723000992},
		{`PV(0.1/12,12,-87.91588723000992)`, 1000},
		{`FV(0.1/12,12,-87.91588723000992,1000)`, 0},
		{`NPER(0.1/12,-87.91588723000992,1000)`, 12},
		{`RATE(12,PMT(0.1/12,12,1000),1000)`, 0.1 / 12},
		{`IPMT(0.1/12,1,12,1000)`, -0.1 / 12 * 1000},
		{`PPMT(0.1/12,1,12,1000)`, -87.91588723000992 + (0.1 / 12 * 1000)},
		{`IPMT(0.1/12,1,12,1000,0,1)`, 0},
		// CUMIPMT: sum of interest payments periods 1–12 (outflow, negative)
		{`CUMIPMT(0.1/12,12,1000,1,12,0)`, -(87.91588723000992*12 - 1000)},
		// CUMPRINC: sum of principal payments periods 1–12 equals −PV
		{`CUMPRINC(0.1/12,12,1000,1,12,0)`, -1000},
		// SLN: (1000 − 100) / 10 = 90
		{`SLN(1000,100,10)`, 90},
		// SYD period 1: (1000−100)*(10−1+1)*2/(10*11) = 900*20/110
		{`SYD(1000,100,10,1)`, 900.0 * 20 / 110},
		// DDB period 1 with factor 2: min(1000−0, 1000*2/5) = 400
		{`DDB(1000,100,5,1)`, 400},
		// DDB period 2: book=600, dep=min(600−100,600*2/5)=240
		{`DDB(1000,100,5,2)`, 240},
		// DB: rate=round(1−(100/1000)^(1/6),3)=0.319; dep=1000*0.319*7/12
		{`DB(1000,100,6,1,7)`, 1000 * 0.319 * 7.0 / 12},
		{`NPV(0.1,100,100)`, 173.55371900826447},
		{`EFFECT(0.12,12)`, 0.12682503013196977},
		{`NOMINAL(EFFECT(0.12,12),12)`, 0.12},
	} {
		t.Run(tt.formula, func(t *testing.T) {
			got, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if got.IsError() {
				t.Fatalf("got error: %s", got.Err.String())
			}
			if !got.IsNumber() || math.Abs(got.Num-tt.want) > 1e-9 {
				t.Fatalf("%s = %#v, want numeric %v", tt.formula, got, tt.want)
			}
		})
	}
}

func TestIRRAndMIRR(t *testing.T) {
	// Set up cash flows in A1:A4: initial outflow + three inflows.
	engine := NewEngine()
	engine.SetCell("A1", Number(-1000))
	engine.SetCell("A2", Number(300))
	engine.SetCell("A3", Number(400))
	engine.SetCell("A4", Number(400))
	engine.SetFormula("B1", "IRR(A1:A4)")
	engine.SetFormula("B2", "MIRR(A1:A4,0.1,0.12)")

	if got := engine.GetCell("B1"); !got.IsNumber() || math.Abs(got.Num-0.04696986163211051) > 1e-9 {
		t.Fatalf("IRR = %#v, want ~0.04696986163211051", got)
	}
	if got := engine.GetCell("B2"); !got.IsNumber() || math.Abs(got.Num-0.069789459841953) > 1e-9 {
		t.Fatalf("MIRR = %#v, want ~0.069789459841953", got)
	}
}

func TestEngineEvaluatesTextSplitAndScalarSpillGapFunctions(t *testing.T) {
	engine := NewEngine()
	engine.SetFormula("A1", `TEXTSPLIT("a,b;c,d",",",";")`)
	engine.SetFormula("D1", `SIN(SEQUENCE(2,2))`)

	for ref, want := range map[string]string{
		"A1": "a",
		"B1": "b",
		"A2": "c",
		"B2": "d",
	} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %q", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{
		"D1": math.Sin(1),
		"E1": math.Sin(2),
		"D2": math.Sin(3),
		"E2": math.Sin(4),
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || math.Abs(got.Num-want) > 1e-9 {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestInfoCoercionFunctions(t *testing.T) {
	boolTests := []struct {
		name    string
		formula string
		want    bool
	}{
		{name: "iserr true for value error", formula: "ISERR(#VALUE!)", want: true},
		{name: "iserr false for na", formula: "ISERR(#N/A)", want: false},
		{name: "iserr false for number", formula: "ISERR(1)", want: false},
	}

	for _, tt := range boolTests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("got error: %s", result.Err.String())
			}
			if !result.IsBool() || result.Bool != tt.want {
				t.Fatalf("result = %#v, want bool %v", result, tt.want)
			}
		})
	}

	numberTests := []struct {
		name    string
		formula string
		want    float64
	}{
		{name: "error.type value", formula: "ERROR.TYPE(#VALUE!)", want: 3},
		{name: "error.type na", formula: "ERROR.TYPE(#N/A)", want: 7},
		{name: "type number", formula: "TYPE(1)", want: 1},
		{name: "type text", formula: `TYPE("x")`, want: 2},
		{name: "type logical", formula: "TYPE(TRUE)", want: 4},
		{name: "type error", formula: "TYPE(#REF!)", want: 16},
		{name: "n number", formula: "N(42)", want: 42},
		{name: "n true", formula: "N(TRUE)", want: 1},
		{name: "n false", formula: "N(FALSE)", want: 0},
		{name: "n text", formula: `N("42")`, want: 0},
	}

	for _, tt := range numberTests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("got error: %s", result.Err.String())
			}
			if !result.IsNumber() || result.Num != tt.want {
				t.Fatalf("result = %#v, want number %v", result, tt.want)
			}
		})
	}

	textTests := []struct {
		name    string
		formula string
		want    string
	}{
		{name: "t text", formula: `T("hello")`, want: "hello"},
		{name: "t number", formula: "T(99)", want: ""},
		{name: "t logical", formula: "T(TRUE)", want: ""},
	}

	for _, tt := range textTests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("got error: %s", result.Err.String())
			}
			if !result.IsText() || result.Text != tt.want {
				t.Fatalf("result = %#v, want text %q", result, tt.want)
			}
		})
	}

	if result, err := Evaluate("TYPE(A1:B2)"); err != nil {
		t.Fatalf("TYPE range error: %v", err)
	} else if result.IsError() || !result.IsNumber() || result.Num != 64 {
		t.Fatalf("TYPE(A1:B2) = %#v, want array type 64", result)
	}

	for _, tt := range []struct {
		formula string
		label   string
	}{
		{formula: "TYPE(INDEX(B1:D3,0,2))", label: "TYPE index column reference"},
		{formula: "TYPE(OFFSET(B1,0,0,2,3))", label: "TYPE offset area reference"},
		{formula: "TYPE(CHOOSECOLS(B1:D3,1,3))", label: "TYPE choosecols reference"},
		{formula: `TYPE(CHOOSE(SEQUENCE(3,1),"one","two","three"))`, label: "TYPE choose scalar array"},
		{formula: `TYPE(TEXTSPLIT("a,b,c",","))`, label: "TYPE textsplit array"},
		{formula: `TYPE(LET(x,SEQUENCE(2,2),x))`, label: "TYPE let array"},
		{formula: `TYPE(LAMBDA(x,x*10)(SEQUENCE(2,2)))`, label: "TYPE lambda apply array"},
	} {
		if result, err := Evaluate(tt.formula); err != nil {
			t.Fatalf("%s eval error: %v", tt.label, err)
		} else if result.IsError() || !result.IsNumber() || result.Num != 64 {
			t.Fatalf("%s = %#v, want array type 64", tt.label, result)
		}
	}

	if result, err := Evaluate("ERROR.TYPE(1)"); err != nil {
		t.Fatalf("ERROR.TYPE non-error eval error: %v", err)
	} else if !result.IsError() || result.Err != ErrNA {
		t.Fatalf("ERROR.TYPE(1) = %#v, want #N/A", result)
	}

	if result, err := Evaluate("N(#DIV/0!)"); err != nil {
		t.Fatalf("N error eval error: %v", err)
	} else if !result.IsError() || result.Err != ErrDiv0 {
		t.Fatalf("N(#DIV/0!) = %#v, want #DIV/0!", result)
	}

	if result, err := Evaluate("T(#DIV/0!)"); err != nil {
		t.Fatalf("T error eval error: %v", err)
	} else if !result.IsError() || result.Err != ErrDiv0 {
		t.Fatalf("T(#DIV/0!) = %#v, want #DIV/0!", result)
	}
}

func TestReferenceAndParityFunctions(t *testing.T) {
	boolTests := []struct {
		name    string
		formula string
		want    bool
	}{
		{name: "isref cell ref", formula: "ISREF(A1)", want: true},
		{name: "isref range ref", formula: "ISREF(A1:B2)", want: true},
		{name: "isref index ref", formula: "ISREF(INDEX(A1:C3,2,3))", want: true},
		{name: "isref offset ref", formula: "ISREF(OFFSET(A1,1,0,2,2))", want: true},
		{name: "isref transpose ref", formula: "ISREF(TRANSPOSE(A1:B2))", want: true},
		{name: "isref choosecols ref", formula: "ISREF(CHOOSECOLS(A1:C2,2))", want: true},
		{name: "isref chooserows ref", formula: "ISREF(CHOOSEROWS(A1:C2,1))", want: true},
		{name: "isref take ref", formula: "ISREF(TAKE(A1:C3,2,2))", want: true},
		{name: "isref drop ref", formula: "ISREF(DROP(A1:C3,1,1))", want: true},
		{name: "isref scalar", formula: "ISREF(1)", want: false},
		{name: "isref text", formula: `ISREF("A1")`, want: false},
		{name: "iseven even", formula: "ISEVEN(4)", want: true},
		{name: "iseven odd", formula: "ISEVEN(5)", want: false},
		{name: "iseven negative", formula: "ISEVEN(-2)", want: true},
		{name: "isodd odd", formula: "ISODD(5)", want: true},
		{name: "isodd even", formula: "ISODD(4)", want: false},
		{name: "isodd negative", formula: "ISODD(-3)", want: true},
	}

	for _, tt := range boolTests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("got error: %s", result.Err.String())
			}
			if !result.IsBool() || result.Bool != tt.want {
				t.Fatalf("result = %#v, want bool %v", result, tt.want)
			}
		})
	}

	for _, tt := range []struct {
		name    string
		formula string
		want    float64
	}{
		{name: "areas cell ref", formula: "AREAS(A1)", want: 1},
		{name: "areas range ref", formula: "AREAS(A1:B2)", want: 1},
		{name: "areas offset ref", formula: "AREAS(OFFSET(A1,1,0,2,2))", want: 1},
		{name: "areas drop ref", formula: "AREAS(DROP(A1:C3,1,1))", want: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() || !result.IsNumber() || result.Num != tt.want {
				t.Fatalf("result = %#v, want number %v", result, tt.want)
			}
		})
	}

	if result, err := Evaluate("AREAS(1)"); err != nil {
		t.Fatalf("AREAS scalar eval error: %v", err)
	} else if !result.IsError() || result.Err != ErrValue {
		t.Fatalf("AREAS(1) = %#v, want #VALUE!", result)
	}
}

func TestSheetFunctions(t *testing.T) {
	engine := NewEngine()
	engine.SetFormula("A1", "SHEET()")
	engine.SetFormula("A2", "SHEET(B3)")
	engine.SetFormula("A3", "SHEETS()")
	engine.SetFormula("A4", "SHEETS(B3)")

	for ref, want := range map[string]float64{"A1": 1, "A2": 1, "A3": 1, "A4": 1} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}

	if got, err := Evaluate(`SHEET("MissingSheet")`); err != nil {
		t.Fatalf("SHEET missing error: %v", err)
	} else if !got.IsError() || got.Err != ErrNA {
		t.Fatalf("SHEET(\"MissingSheet\") = %#v, want #N/A", got)
	}

	if got, err := Evaluate(`SHEETS(1)`); err != nil {
		t.Fatalf("SHEETS invalid error: %v", err)
	} else if !got.IsError() || got.Err != ErrRef {
		t.Fatalf("SHEETS(1) = %#v, want #REF!", got)
	}

	multi := NewEngine()
	multi.SetSheetResolver(
		func(token string) (int, bool) {
			normalized := strings.ToUpper(strings.TrimSpace(token))
			if strings.HasPrefix(normalized, "SHEET2!") || normalized == "SHEET2" {
				return 2, true
			}
			if strings.HasPrefix(normalized, "SHEET3!") || normalized == "SHEET3" {
				return 3, true
			}
			if normalized == "" || strings.HasPrefix(normalized, "A") || strings.HasPrefix(normalized, "B") {
				return 1, true
			}
			return 0, false
		},
		func() int { return 3 },
	)
	multi.SetFormula("B1", `SHEET("Sheet2")`)
	multi.SetFormula("B2", `SHEET(Sheet2!A1)`)
	multi.SetFormula("B3", `SHEETS()`)
	multi.SetFormula("B4", `SHEETS(Sheet2!A1)`)
	multi.SetFormula("B5", `SHEET(OFFSET(Sheet2!A1,0,0,2,2))`)
	multi.SetFormula("B6", `SHEET(DROP(Sheet3!A1:C3,1,1))`)

	for ref, want := range map[string]float64{"B1": 2, "B2": 2, "B3": 3, "B4": 1, "B5": 2, "B6": 3} {
		if got := multi.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestAddressFunction(t *testing.T) {
	tests := []struct {
		name    string
		formula string
		want    string
	}{
		{name: "absolute a1", formula: "ADDRESS(2,3)", want: "$C$2"},
		{name: "relative a1", formula: "ADDRESS(2,3,4)", want: "C2"},
		{name: "mixed a1 row abs", formula: "ADDRESS(2,3,2)", want: "C$2"},
		{name: "mixed a1 col abs", formula: "ADDRESS(2,3,3)", want: "$C2"},
		{name: "absolute r1c1", formula: "ADDRESS(2,3,1,FALSE)", want: "R2C3"},
		{name: "relative r1c1", formula: "ADDRESS(2,3,4,FALSE)", want: "R[2]C[3]"},
		{name: "quoted sheet", formula: `ADDRESS(2,3,1,TRUE,"Lookup Sheet")`, want: `'Lookup Sheet'!$C$2`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("got error: %s", result.Err.String())
			}
			if !result.IsText() || result.Text != tt.want {
				t.Fatalf("result = %#v, want text %q", result, tt.want)
			}
		})
	}
}

func TestIndirectFunction(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("A2", Number(20))
	engine.SetCell("A3", Number(30))
	engine.SetFormula("C1", `INDIRECT("A1")`)
	engine.SetFormula("C2", `SUM(INDIRECT("A1:A3"))`)
	engine.SetFormula("C3", `ROWS(INDIRECT("A1:B3"))`)
	engine.SetFormula("C4", `COLUMNS(INDIRECT("A1:B3"))`)
	engine.SetFormula("C5", `ISREF(INDIRECT("A1:B3"))`)
	engine.SetFormula("C6", `AREAS(INDIRECT("A1:B3"))`)

	for ref, want := range map[string]float64{"C1": 10, "C2": 60, "C3": 3, "C4": 2, "C6": 1} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("C5"); !got.IsBool() || !got.Bool {
		t.Fatalf("C5 = %#v, want TRUE", got)
	}

	multi := NewEngine()
	multi.SetSheetResolver(
		func(token string) (int, bool) {
			normalized := strings.ToUpper(strings.TrimSpace(token))
			if strings.HasPrefix(normalized, "SHEET2!") || normalized == "SHEET2" {
				return 2, true
			}
			if normalized == "" || strings.HasPrefix(normalized, "A") || strings.HasPrefix(normalized, "B") || strings.HasPrefix(normalized, "C") || strings.HasPrefix(normalized, "D") {
				return 1, true
			}
			return 0, false
		},
		func() int { return 2 },
	)
	multi.SetFormula("D1", `SHEET(INDIRECT("Sheet2!A1"))`)
	if got := multi.GetCell("D1"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("D1 = %#v, want numeric 2", got)
	}

	if got, err := Evaluate(`INDIRECT("missing")`); err != nil {
		t.Fatalf("INDIRECT invalid eval error: %v", err)
	} else if !got.IsError() || got.Err != ErrRef {
		t.Fatalf("INDIRECT(\"missing\") = %#v, want #REF!", got)
	}
	if got, err := Evaluate(`INDIRECT("R1C1",FALSE)`); err != nil {
		t.Fatalf("INDIRECT r1c1 eval error: %v", err)
	} else if !got.IsError() || got.Err != ErrRef {
		t.Fatalf("INDIRECT(\"R1C1\",FALSE) = %#v, want #REF!", got)
	}
}

func TestIndirectFunctionInfersDynamicSpillRange(t *testing.T) {
	values := map[string]Value{
		"INCOME!D4": Number(120),
		"INCOME!E4": Number(150),
		"INCOME!F4": Number(180),
	}
	getCell := func(ref string) Value {
		if value, ok := values[normalizeCellRef(ref)]; ok {
			return value
		}
		return Empty()
	}

	engine := NewEngine()
	engine.SetExternalResolver(getCell, nil)
	engine.SetCell("B31", Number(4))
	engine.SetFormula("D31", `IF(NOT(ISBLANK(B31)),INDIRECT("Income!D"&B31&":F"&B31),"")`)

	start, end, ok := engine.GetSpillRange("D31")
	if !ok || start != "D31" || end != "F31" {
		t.Fatalf("spill range = %q:%q, ok=%v, want D31:F31", start, end, ok)
	}

	for ref, want := range map[string]float64{"D31": 120, "E31": 150, "F31": 180} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestFormulaMetadataFunctions(t *testing.T) {
	engine := NewEngine()
	engine.SetFormula("A1", "1+1")
	engine.SetCell("A2", Number(7))
	engine.SetArrayFormula("C1", "C1:C2", "ROW(A1:A2)")
	engine.SetFormula("B1", `FORMULATEXT(A1)`)
	engine.SetFormula("B2", `ISFORMULA(A1)`)
	engine.SetFormula("B3", `ISFORMULA(A2)`)
	engine.SetFormula("B4", `FORMULATEXT(INDIRECT("A1"))`)
	engine.SetFormula("B5", `FORMULATEXT(C2)`)
	engine.SetFormula("B6", `ISFORMULA(OFFSET(C1,1,0,1,1))`)
	engine.SetFormula("B7", `FORMULATEXT(A2)`)

	if got := engine.GetCell("B1"); !got.IsText() || got.Text != "=1+1" {
		t.Fatalf("B1 = %#v, want text \"=1+1\"", got)
	}
	if got := engine.GetCell("B2"); !got.IsBool() || !got.Bool {
		t.Fatalf("B2 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("B3"); !got.IsBool() || got.Bool {
		t.Fatalf("B3 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("B4"); !got.IsText() || got.Text != "=1+1" {
		t.Fatalf("B4 = %#v, want text \"=1+1\"", got)
	}
	if got := engine.GetCell("B5"); !got.IsText() || got.Text != "=ROW(A1:A2)" {
		t.Fatalf("B5 = %#v, want array formula text", got)
	}
	if got := engine.GetCell("B6"); !got.IsBool() || !got.Bool {
		t.Fatalf("B6 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("B7"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("B7 = %#v, want #N/A", got)
	}
	if got, err := Evaluate(`FORMULATEXT(1)`); err != nil {
		t.Fatalf("FORMULATEXT scalar eval error: %v", err)
	} else if !got.IsError() || got.Err != ErrValue {
		t.Fatalf("FORMULATEXT(1) = %#v, want #VALUE!", got)
	}
}

func TestIsTypeFunctionsAndNA(t *testing.T) {
	tests := []struct {
		name     string
		formula  string
		wantBool *bool
		wantErr  ErrorType
		wantIsNA bool
	}{
		{name: "isnumber true", formula: "ISNUMBER(7)", wantBool: boolPtr(true)},
		{name: "isnumber false on text", formula: `ISNUMBER("x")`, wantBool: boolPtr(false)},
		{name: "istext true", formula: `ISTEXT("hi")`, wantBool: boolPtr(true)},
		{name: "istext false on number", formula: "ISTEXT(1)", wantBool: boolPtr(false)},
		{name: "isblank true on empty", formula: "ISBLANK(A1)", wantBool: boolPtr(true)},
		{name: "iserror true", formula: "ISERROR(#N/A)", wantBool: boolPtr(true)},
		{name: "iserror false on number", formula: "ISERROR(1)", wantBool: boolPtr(false)},
		{name: "isna true", formula: "ISNA(#N/A)", wantBool: boolPtr(true)},
		{name: "isna false on value error", formula: "ISNA(#VALUE!)", wantBool: boolPtr(false)},
		{name: "islogical true on bool", formula: "ISLOGICAL(TRUE)", wantBool: boolPtr(true)},
		{name: "islogical false on number", formula: "ISLOGICAL(1)", wantBool: boolPtr(false)},
		{name: "isnontext true on number", formula: "ISNONTEXT(1)", wantBool: boolPtr(true)},
		{name: "isnontext false on text", formula: `ISNONTEXT("a")`, wantBool: boolPtr(false)},
		{name: "na returns na error", formula: "NA()", wantIsNA: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if tt.wantIsNA {
				if !result.IsError() || result.Err != ErrNA {
					t.Fatalf("result = %#v, want #N/A", result)
				}
				return
			}
			if !result.IsBool() || result.Bool != *tt.wantBool {
				t.Fatalf("result = %#v, want bool %v", result, *tt.wantBool)
			}
		})
	}
}

func TestAndOrNotWithRanges(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Bool(true))
	engine.SetCell("A2", Bool(true))
	engine.SetCell("B1", Bool(false))
	engine.SetCell("B2", Bool(true))

	engine.SetFormula("C1", "AND(A1:A2)")
	engine.SetFormula("C2", "OR(B1:B2)")
	engine.SetFormula("C3", "AND(A1:A2,B1:B2)")

	if got := engine.GetCell("C1"); !got.IsBool() || !got.Bool {
		t.Fatalf("C1 = %#v, want true", got)
	}
	if got := engine.GetCell("C2"); !got.IsBool() || !got.Bool {
		t.Fatalf("C2 = %#v, want true", got)
	}
	if got := engine.GetCell("C3"); !got.IsBool() || got.Bool {
		t.Fatalf("C3 = %#v, want false", got)
	}
}

func TestEvaluateErrorLiterals(t *testing.T) {
	tests := []struct {
		name     string
		formula  string
		expected ErrorType
	}{
		{"na", "#N/A", ErrNA},
		{"value", "#VALUE!", ErrValue},
		{"name", "#NAME?", ErrName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if !result.IsError() || result.Err != tt.expected {
				t.Fatalf("result = %#v, want error %s", result, tt.expected.String())
			}
		})
	}
}

func TestEvaluateText(t *testing.T) {
	tests := []struct {
		name     string
		formula  string
		expected string
	}{
		{"concat", `CONCATENATE("Hello"," ","World")`, "Hello World"},
		{"upper", `UPPER("hello")`, "HELLO"},
		{"lower", `LOWER("HELLO")`, "hello"},
		{"len", `LEN("hello")`, "5"},
		{"left", `LEFT("hello",2)`, "he"},
		{"right", `RIGHT("hello",2)`, "lo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("Got error: %s", result.Err.String())
			}
			got := result.ToText()
			if got != tt.expected {
				t.Errorf("Text = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIfFunction(t *testing.T) {
	tests := []struct {
		name     string
		formula  string
		expected float64
	}{
		{"if true", "IF(1=1,10,20)", 10},
		{"if false", "IF(1=2,10,20)", 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("Got error: %s", result.Err.String())
			}
			if !result.IsNumber() {
				t.Fatalf("Expected number, got %v", result.Type)
			}
			if result.Num != tt.expected {
				t.Errorf("Num = %v, want %v", result.Num, tt.expected)
			}
		})
	}
}

func TestIfErrorAndIfNaFunctions(t *testing.T) {
	tests := []struct {
		name        string
		formula     string
		wantNumber  *float64
		wantError   ErrorType
		wantIsError bool
	}{
		{name: "iferror catches name error", formula: "IFERROR(UNKNOWNFUNC(),7)", wantNumber: numberPtr(7)},
		{name: "iferror preserves non error without evaluating fallback", formula: "IFERROR(1,UNKNOWNFUNC())", wantNumber: numberPtr(1)},
		{name: "iferror single arg preserves non error", formula: "IFERROR(AVERAGE(1,2,3))", wantNumber: numberPtr(2)},
		{name: "ifna catches na", formula: "IFNA(#N/A,7)", wantNumber: numberPtr(7)},
		{name: "ifna preserves other errors without evaluating fallback", formula: "IFNA(#VALUE!,UNKNOWNFUNC())", wantError: ErrValue, wantIsError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if tt.wantIsError {
				if !result.IsError() || result.Err != tt.wantError {
					t.Fatalf("result = %#v, want error %s", result, tt.wantError.String())
				}
				return
			}
			if !result.IsNumber() || result.Num != *tt.wantNumber {
				t.Fatalf("result = %#v, want numeric %v", result, *tt.wantNumber)
			}
		})
	}
}

func TestIfErrorSingleArgReturnsBlankOnError(t *testing.T) {
	result, err := Evaluate("IFERROR(AVERAGE())")
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}
	if !result.IsText() || result.Text != "" {
		t.Fatalf("result = %#v, want blank text", result)
	}
}

func TestIfsAndSwitchFunctions(t *testing.T) {
	tests := []struct {
		name        string
		formula     string
		wantNumber  *float64
		wantError   ErrorType
		wantIsError bool
	}{
		{name: "ifs returns first true branch lazily", formula: "IFS(FALSE,UNKNOWNFUNC(),TRUE,7)", wantNumber: numberPtr(7)},
		{name: "ifs returns na when no condition matches", formula: "IFS(FALSE,1,FALSE,2)", wantError: ErrNA, wantIsError: true},
		{name: "switch returns matching branch lazily", formula: "SWITCH(2,1,UNKNOWNFUNC(),2,7,UNKNOWNFUNC())", wantNumber: numberPtr(7)},
		{name: "switch uses default when no branch matches", formula: "SWITCH(3,1,10,2,20,30)", wantNumber: numberPtr(30)},
		{name: "switch returns na without default", formula: "SWITCH(3,1,10,2,20)", wantError: ErrNA, wantIsError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if tt.wantIsError {
				if !result.IsError() || result.Err != tt.wantError {
					t.Fatalf("result = %#v, want error %s", result, tt.wantError.String())
				}
				return
			}
			if !result.IsNumber() || result.Num != *tt.wantNumber {
				t.Fatalf("result = %#v, want numeric %v", result, *tt.wantNumber)
			}
		})
	}
}

func TestEngine(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("A2", Number(20))
	engine.SetFormula("A3", "SUM(A1,A2)")

	result := engine.GetCell("A3")
	if !result.IsNumber() {
		t.Fatalf("Expected number, got %v", result.Type)
	}
	if result.Num != 30 {
		t.Errorf("Num = %v, want 30", result.Num)
	}
}

func TestEngineWithRanges(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("B1", Number(1))
	engine.SetCell("B2", Number(2))
	engine.SetCell("B3", Number(3))

	// Test that range expansion works
	values := engine.GetRange("B1", "B3")
	if len(values) != 3 {
		t.Fatalf("Expected 3 values, got %d", len(values))
	}

	// Calculate sum manually
	var sum float64
	for _, v := range values {
		if v.IsNumber() {
			num, _ := v.ToNumber()
			sum += num
		}
	}
	if sum != 6 {
		t.Errorf("Sum = %v, want 6", sum)
	}
}

func TestEngineEvaluatesRangeFunctions(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	engine.SetFormula("A4", "SUM(A1:A3)")

	result := engine.GetCell("A4")
	if !result.IsNumber() {
		t.Fatalf("Expected number, got %v", result.Type)
	}
	if result.Num != 6 {
		t.Errorf("Num = %v, want 6", result.Num)
	}
}

func TestEngineEvaluatesArrayFormulaMembers(t *testing.T) {
	engine := NewEngine()
	engine.SetArrayFormula("A1", "A1:B1", "COLUMN(A1:B1)")

	left := engine.GetCell("A1")
	if !left.IsNumber() || left.Num != 1 {
		t.Fatalf("A1 = %#v, want numeric 1", left)
	}
	right := engine.GetCell("B1")
	if !right.IsNumber() || right.Num != 2 {
		t.Fatalf("B1 = %#v, want numeric 2", right)
	}
}

func TestEngineEvaluatesArrayFormulaRangeArithmetic(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("A2", Number(20))
	engine.SetCell("B1", Number(1))
	engine.SetCell("B2", Number(2))
	engine.SetArrayFormula("C1", "C1:C2", "A1:A2+B1:B2")

	first := engine.GetCell("C1")
	if !first.IsNumber() || first.Num != 11 {
		t.Fatalf("C1 = %#v, want numeric 11", first)
	}
	second := engine.GetCell("C2")
	if !second.IsNumber() || second.Num != 22 {
		t.Fatalf("C2 = %#v, want numeric 22", second)
	}
}

func TestEngineBroadcastsSingletonArrayDimensions(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("B1", Number(20))
	engine.SetCell("A2", Number(30))
	engine.SetArrayFormula("C1", "C1:D2", "A1:B1+A1:A2")

	if got := engine.GetCell("C1"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("C1 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 30 {
		t.Fatalf("D1 = %#v, want numeric 30", got)
	}
	if got := engine.GetCell("C2"); !got.IsNumber() || got.Num != 40 {
		t.Fatalf("C2 = %#v, want numeric 40", got)
	}
	if got := engine.GetCell("D2"); !got.IsNumber() || got.Num != 50 {
		t.Fatalf("D2 = %#v, want numeric 50", got)
	}
}

func TestEngineEvaluatesSumProductRanges(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B1", Number(3))
	engine.SetCell("B2", Number(4))
	engine.SetCell("C1", Number(10))
	engine.SetCell("C2", Number(20))
	engine.SetCell("D1", Number(30))
	engine.SetCell("D2", Number(40))
	engine.SetFormula("E1", "SUMPRODUCT(A1:B2,C1:D2)")

	got := engine.GetCell("E1")
	if !got.IsNumber() || got.Num != 300 {
		t.Fatalf("E1 = %#v, want numeric 300", got)
	}
}

func TestEngineEvaluatesRangeBasedLogicalAndTextFunctions(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Bool(true))
	engine.SetCell("A2", Bool(false))
	engine.SetCell("B1", Text("Hello"))
	engine.SetCell("B2", Text("World"))
	engine.SetFormula("C1", "AND(A1:A2)")
	engine.SetFormula("C2", "OR(A1:A2)")
	engine.SetFormula("C3", "CONCATENATE(B1:B2)")

	if got := engine.GetCell("C1"); !got.IsBool() || got.Bool {
		t.Fatalf("C1 = %#v, want FALSE", got)
	}
	if got := engine.GetCell("C2"); !got.IsBool() || !got.Bool {
		t.Fatalf("C2 = %#v, want TRUE", got)
	}
	if got := engine.GetCell("C3"); !got.IsText() || got.Text != "HelloWorld" {
		t.Fatalf("C3 = %#v, want HelloWorld", got)
	}
}

func TestEngineEvaluatesSumIfRanges(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))
	engine.SetCell("B3", Number(30))
	engine.SetFormula("C1", `SUMIF(A1:A3,">1",B1:B3)`)

	got := engine.GetCell("C1")
	if !got.IsNumber() || got.Num != 50 {
		t.Fatalf("C1 = %#v, want numeric 50", got)
	}
}

func TestEngineEvaluatesArrayConstant(t *testing.T) {
	engine := NewEngine()
	engine.SetArrayFormula("A1", "A1:B2", "{1,2;3,4}")

	if got := engine.GetCell("A1"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("A1 = %#v, want 1", got)
	}
	if got := engine.GetCell("B1"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("B1 = %#v, want 2", got)
	}
	if got := engine.GetCell("A2"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("A2 = %#v, want 3", got)
	}
	if got := engine.GetCell("B2"); !got.IsNumber() || got.Num != 4 {
		t.Fatalf("B2 = %#v, want 4", got)
	}
}

func TestEngineArrayConstantOutsideContextReturnsValueError(t *testing.T) {
	engine := NewEngine()
	engine.SetFormula("A1", "{1,2,3}")
	got := engine.GetCell("A1")
	if !got.IsError() || got.Err != ErrValue {
		t.Fatalf("A1 = %#v, want #VALUE!", got)
	}
}

func TestEngineImplicitIntersectionVerticalRange(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("A2", Number(20))
	engine.SetCell("A3", Number(30))
	engine.SetFormula("C2", "@A1:A3")

	if got := engine.GetCell("C2"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("C2 = %#v, want 20", got)
	}
}

func TestEngineImplicitIntersectionHorizontalRange(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("B1", Number(20))
	engine.SetCell("C1", Number(30))
	engine.SetFormula("B2", "@A1:C1")

	if got := engine.GetCell("B2"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("B2 = %#v, want 20", got)
	}
}

func TestEngineSpillRangeOperator(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("A2", Number(20))
	engine.SetCell("A3", Number(30))
	engine.SetArrayFormula("B1", "B1:B3", "A1:A3*2")
	engine.SetArrayFormula("C1", "C1:C3", "B1#")

	if got := engine.GetCell("C1"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("C1 = %#v, want 20", got)
	}
	if got := engine.GetCell("C2"); !got.IsNumber() || got.Num != 40 {
		t.Fatalf("C2 = %#v, want 40", got)
	}
	if got := engine.GetCell("C3"); !got.IsNumber() || got.Num != 60 {
		t.Fatalf("C3 = %#v, want 60", got)
	}
}

func TestParseErrorCalc(t *testing.T) {
	if ParseError("#CALC!") != ErrCalc {
		t.Fatal("expected ErrCalc for #CALC!")
	}
	if ParseError("#CALC") != ErrCalc {
		t.Fatal("expected ErrCalc for #CALC")
	}
	if ErrCalc.String() != "#CALC!" {
		t.Fatalf("ErrCalc.String() = %q, want #CALC!", ErrCalc.String())
	}
}

func TestEngineEvaluatesCriteriaFunctionFamilies(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C1", Text("x"))
	engine.SetCell("C2", Text("y"))
	engine.SetCell("C3", Text("x"))
	engine.SetFormula("D1", `COUNTIFS(A1:A3,">1",C1:C3,"x")`)
	engine.SetFormula("D2", `SUMIFS(B1:B3,A1:A3,">1",C1:C3,"x")`)
	engine.SetFormula("D3", `AVERAGEIF(A1:A3,">1",B1:B3)`)
	engine.SetFormula("D4", `AVERAGEIFS(B1:B3,A1:A3,">1",C1:C3,"x")`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("D1 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("D2"); !got.IsNumber() || got.Num != 30 {
		t.Fatalf("D2 = %#v, want numeric 30", got)
	}
	if got := engine.GetCell("D3"); !got.IsNumber() || got.Num != 25 {
		t.Fatalf("D3 = %#v, want numeric 25", got)
	}
	if got := engine.GetCell("D4"); !got.IsNumber() || got.Num != 30 {
		t.Fatalf("D4 = %#v, want numeric 30", got)
	}
}

func TestEngineEvaluatesDatabaseFunctionFamily(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("Type"))
	engine.SetCell("B1", Text("Qty"))
	engine.SetCell("C1", Text("Label"))
	engine.SetCell("A2", Text("A"))
	engine.SetCell("B2", Number(10))
	engine.SetCell("C2", Text("red"))
	engine.SetCell("A3", Text("B"))
	engine.SetCell("B3", Number(20))
	engine.SetCell("C3", Text(""))
	engine.SetCell("A4", Text("A"))
	engine.SetCell("B4", Number(30))
	engine.SetCell("C4", Text("blue"))
	engine.SetCell("A5", Text("A"))
	engine.SetCell("B5", Text("n/a"))
	engine.SetCell("C5", Text("green"))
	engine.SetCell("E1", Text("Type"))
	engine.SetCell("E2", Text("A"))
	engine.SetCell("I1", Text("Type"))
	engine.SetCell("J1", Text("Label"))
	engine.SetCell("I2", Text("A"))
	engine.SetCell("J2", Text("red"))
	engine.SetCell("E1", Text("Type"))
	engine.SetCell("E2", Text("A"))

	engine.SetFormula("G1", `DSUM(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G2", `DCOUNT(A1:C5,2,E1:E2)`)
	engine.SetFormula("G3", `DCOUNTA(A1:C5,C1,E1:E2)`)
	engine.SetFormula("G4", `DAVERAGE(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G5", `DMAX(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G6", `DMIN(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G7", `DPRODUCT(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G8", `DSTDEV(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G9", `DSTDEVP(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G10", `DVAR(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G11", `DVARP(A1:C5,"Qty",E1:E2)`)
	engine.SetFormula("G12", `DGET(A1:C5,"Qty",I1:J2)`)
	engine.SetFormula("G13", `DGET(A1:C5,"Qty",E1:E2)`)

	if got := engine.GetCell("G1"); !got.IsNumber() || got.Num != 40 {
		t.Fatalf("G1 = %#v, want numeric 40", got)
	}
	if got := engine.GetCell("G2"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("G2 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("G3"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("G3 = %#v, want numeric 3", got)
	}
	if got := engine.GetCell("G4"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("G4 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("G5"); !got.IsNumber() || got.Num != 30 {
		t.Fatalf("G5 = %#v, want numeric 30", got)
	}
	if got := engine.GetCell("G6"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("G6 = %#v, want numeric 10", got)
	}
	if got := engine.GetCell("G7"); !got.IsNumber() || got.Num != 300 {
		t.Fatalf("G7 = %#v, want numeric 300", got)
	}
	if got := engine.GetCell("G8"); !got.IsNumber() || math.Abs(got.Num-math.Sqrt(200)) > 1e-9 {
		t.Fatalf("G8 = %#v, want numeric sqrt(200)", got)
	}
	if got := engine.GetCell("G9"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("G9 = %#v, want numeric 10", got)
	}
	if got := engine.GetCell("G10"); !got.IsNumber() || got.Num != 200 {
		t.Fatalf("G10 = %#v, want numeric 200", got)
	}
	if got := engine.GetCell("G11"); !got.IsNumber() || got.Num != 100 {
		t.Fatalf("G11 = %#v, want numeric 100", got)
	}
	if got := engine.GetCell("G12"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("G12 = %#v, want numeric 10", got)
	}
	if got := engine.GetCell("G13"); !got.IsError() || got.Err != ErrNum {
		t.Fatalf("G13 = %#v, want #NUM!", got)
	}
}

func TestEngineEvaluatesInlineArrayExpressionsInAggregates(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C1", Text("x"))
	engine.SetCell("C2", Text("y"))
	engine.SetCell("C3", Text("x"))
	engine.SetFormula("D1", `SUM(A1:A3*B1:B3)`)
	engine.SetFormula("D2", `MAX(A1:A3+B1:B3)`)
	engine.SetFormula("D3", `COUNTIF(A1:A3+B1:B3,">15")`)
	engine.SetFormula("D4", `SUM(IF(C1:C3="x",B1:B3,0))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 140 {
		t.Fatalf("D1 = %#v, want numeric 140", got)
	}
	if got := engine.GetCell("D2"); !got.IsNumber() || got.Num != 33 {
		t.Fatalf("D2 = %#v, want numeric 33", got)
	}
	if got := engine.GetCell("D3"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("D3 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("D4"); !got.IsNumber() || got.Num != 40 {
		t.Fatalf("D4 = %#v, want numeric 40", got)
	}
}

func TestEngineEvaluatesAggregateFunctionFamily(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(2))
	engine.SetCell("A2", Number(3))
	engine.SetCell("A3", Number(4))
	engine.SetCell("B1", Number(1))
	engine.SetCell("B2", Number(5))
	engine.SetCell("B3", Number(9))
	engine.SetFormula("C1", `PRODUCT(A1:A3)`)
	engine.SetFormula("C2", `SUMSQ(A1:A3)`)
	engine.SetFormula("C3", `LARGE(B1:B3,2)`)
	engine.SetFormula("C4", `SMALL(B1:B3,2)`)

	if got := engine.GetCell("C1"); !got.IsNumber() || got.Num != 24 {
		t.Fatalf("C1 = %#v, want numeric 24", got)
	}
	if got := engine.GetCell("C2"); !got.IsNumber() || got.Num != 29 {
		t.Fatalf("C2 = %#v, want numeric 29", got)
	}
	if got := engine.GetCell("C3"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("C3 = %#v, want numeric 5", got)
	}
	if got := engine.GetCell("C4"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("C4 = %#v, want numeric 5", got)
	}
}

func TestEngineEvaluatesAggregateWrapperFunction(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	engine.SetCell("A4", Number(4))
	engine.SetCell("A5", Number(5))
	engine.SetFormula("B1", `AGGREGATE(9,0,A1:A5)`)
	engine.SetFormula("B2", `AGGREGATE(1,6,A1:A5)`)
	engine.SetFormula("B3", `AGGREGATE(12,4,A1:A5)`)
	engine.SetFormula("B4", `AGGREGATE(14,0,A1:A5,2)`)
	engine.SetFormula("B5", `AGGREGATE(15,0,A1:A5,2)`)
	engine.SetFormula("B6", `AGGREGATE(16,0,A1:A5,0.75)`)
	engine.SetFormula("B7", `AGGREGATE(99,0,A1:A5)`)

	if got := engine.GetCell("B1"); !got.IsNumber() || got.Num != 15 {
		t.Fatalf("B1 = %#v, want numeric 15", got)
	}
	if got := engine.GetCell("B2"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("B2 = %#v, want numeric 3", got)
	}
	if got := engine.GetCell("B3"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("B3 = %#v, want numeric 3", got)
	}
	if got := engine.GetCell("B4"); !got.IsNumber() || got.Num != 4 {
		t.Fatalf("B4 = %#v, want numeric 4", got)
	}
	if got := engine.GetCell("B5"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("B5 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("B6"); !got.IsNumber() || got.Num != 4 {
		t.Fatalf("B6 = %#v, want numeric 4", got)
	}
	if got := engine.GetCell("B7"); !got.IsError() || got.Err != ErrValue {
		t.Fatalf("B7 = %#v, want #VALUE!", got)
	}
}

func TestEngineEvaluatesHyperlinkFormulaDisplayText(t *testing.T) {
	engine := NewEngine()
	engine.SetFormula("A1", `HYPERLINK("https://example.com","Example")`)
	engine.SetFormula("A2", `HYPERLINK("#Sheet2!A1")`)
	engine.SetFormula("A3", `HYPERLINK(123,456)`)

	if got := engine.GetCell("A1").ToText(); got != "Example" {
		t.Fatalf("A1 = %q, want %q", got, "Example")
	}
	if got := engine.GetCell("A2").ToText(); got != "#Sheet2!A1" {
		t.Fatalf("A2 = %q, want %q", got, "#Sheet2!A1")
	}
	if got := engine.GetCell("A3").ToText(); got != "456" {
		t.Fatalf("A3 = %q, want %q", got, "456")
	}
}

func TestEngineGetPivotDataReturnsExplicitRefError(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("PivotTable1"))
	engine.SetFormula("B1", `GETPIVOTDATA("Sales",A1)`)
	engine.SetFormula("B2", `GETPIVOTDATA("Sales")`)

	if got := engine.GetCell("B1"); !got.IsError() || got.Err != ErrRef {
		t.Fatalf("B1 = %#v, want #REF!", got)
	}
	if got := engine.GetCell("B2"); !got.IsError() || got.Err != ErrValue {
		t.Fatalf("B2 = %#v, want #VALUE!", got)
	}
}

func TestEngineGetPivotDataUsesConfiguredResolver(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("PivotTable1"))
	engine.SetPivotDataResolver(func(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool) {
		if dataField != "Sum of Sales" {
			t.Fatalf("dataField = %q, want Sum of Sales", dataField)
		}
		if pivotTableRef != "A1" {
			t.Fatalf("pivotTableRef = %q, want A1", pivotTableRef)
		}
		if pivotTableValue.ToText() != "PivotTable1" {
			t.Fatalf("pivotTableValue = %#v, want PivotTable1", pivotTableValue)
		}
		if len(criteria) != 1 || criteria[0].Field != "Region" || criteria[0].Item.ToText() != "North" {
			t.Fatalf("criteria = %#v, want Region=North", criteria)
		}
		return Number(42), true
	})
	engine.SetFormula("B1", `GETPIVOTDATA("Sum of Sales",A1,"Region","North")`)

	if got := engine.GetCell("B1"); !got.IsNumber() || got.Num != 42 {
		t.Fatalf("B1 = %#v, want 42", got)
	}
}

func TestEngineEvaluatesStatFunctionFamily(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(2))
	engine.SetCell("A4", Number(3))
	engine.SetCell("A5", Number(4))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C1", Text(""))
	engine.SetCell("C3", Number(1))
	engine.SetFormula("D1", `MEDIAN(A1:A5)`)
	engine.SetFormula("D2", `RANK(B2,B1:B3)`)
	engine.SetFormula("D3", `RANK.EQ(B2,B1:B3)`)
	engine.SetFormula("D4", `COUNTBLANK(C1:C3)`)
	engine.SetFormula("D5", `MODE(A1:A5)`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("D1 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("D2"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("D2 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("D3"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("D3 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("D4"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("D4 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("D5"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("D5 = %#v, want numeric 2", got)
	}
}

func TestEngineEvaluatesRoundingFunctionFamily(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(2.3))
	engine.SetCell("A2", Number(-2.3))
	engine.SetFormula("B1", `CEILING(A1,1)`)
	engine.SetFormula("B2", `FLOOR(A1,1)`)
	engine.SetFormula("B3", `ROUNDUP(A1,0)`)
	engine.SetFormula("B4", `ROUNDDOWN(A1,0)`)
	engine.SetFormula("B5", `ROUNDUP(A2,0)`)
	engine.SetFormula("B6", `ROUNDDOWN(A2,0)`)

	if got := engine.GetCell("B1"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("B1 = %#v, want numeric 3", got)
	}
	if got := engine.GetCell("B2"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("B2 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("B3"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("B3 = %#v, want numeric 3", got)
	}
	if got := engine.GetCell("B4"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("B4 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("B5"); !got.IsNumber() || got.Num != -3 {
		t.Fatalf("B5 = %#v, want numeric -3", got)
	}
	if got := engine.GetCell("B6"); !got.IsNumber() || got.Num != -2 {
		t.Fatalf("B6 = %#v, want numeric -2", got)
	}
}

func TestEngineInfersImplicitSpillRanges(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C1", Number(2))
	engine.SetCell("EA1", Text("Q1"))
	engine.SetCell("EB1", Text("Q2"))
	engine.SetCell("EC1", Text("Q3"))
	engine.SetCell("EA2", Number(100))
	engine.SetCell("EB2", Number(200))
	engine.SetCell("EC2", Number(300))
	engine.SetCell("EG1", Number(1))
	engine.SetCell("EH1", Number(10))
	engine.SetCell("EI1", Number(100))
	engine.SetCell("EG2", Number(2))
	engine.SetCell("EH2", Number(20))
	engine.SetCell("EI2", Number(200))
	engine.SetCell("EG3", Number(3))
	engine.SetCell("EH3", Number(30))
	engine.SetCell("EI3", Number(300))
	engine.SetCell("EJ1", Number(2))
	engine.SetCell("EK1", Number(3))
	engine.SetCell("EM1", Text("Q1"))
	engine.SetCell("EN1", Text("Q2"))
	engine.SetCell("EO1", Text("Q3"))
	engine.SetCell("EM2", Number(100))
	engine.SetCell("EN2", Number(200))
	engine.SetCell("EO2", Number(300))
	engine.SetCell("EM3", Number(1000))
	engine.SetCell("EN3", Number(2000))
	engine.SetCell("EO3", Number(3000))
	engine.SetCell("EP1", Number(2))
	engine.SetCell("EP2", Number(3))
	engine.SetCell("FA1", Text("beta"))
	engine.SetCell("FA2", Text("beta"))
	engine.SetCell("FA3", Text("gamma"))
	engine.SetCell("FB1", Number(1))
	engine.SetCell("FB2", Number(-1))
	engine.SetCell("FC1", Number(10))
	engine.SetCell("FC2", Number(20))
	engine.SetCell("FC3", Number(30))
	engine.SetCell("FD1", Number(4))
	engine.SetCell("FD2", Number(4))
	engine.SetCell("FD3", Number(40))
	engine.SetCell("FE1", Number(1))
	engine.SetCell("FE2", Number(3))
	engine.SetCell("FE3", Number(5))
	engine.SetCell("FE4", Number(7))
	engine.SetCell("FF1", Number(1))
	engine.SetCell("FF2", Number(-1))
	engine.SetCell("FG1", Number(4))
	engine.SetCell("FG2", Number(4))
	engine.SetCell("FH1", Number(1))
	engine.SetCell("FH2", Number(3))
	engine.SetCell("FH3", Number(5))
	engine.SetCell("FI1", Number(10))
	engine.SetCell("FI2", Number(30))
	engine.SetCell("FI3", Number(50))
	engine.SetCell("FJ1", Bool(true))
	engine.SetCell("FJ2", Bool(false))
	engine.SetCell("IA1", Bool(true))
	engine.SetCell("IB1", Bool(false))
	engine.SetCell("FK1", Number(1))
	engine.SetCell("FL1", Number(-1))
	engine.SetCell("OU1", Number(4))
	engine.SetCell("OV1", Number(5))
	engine.SetCell("PA1", Number(2))
	engine.SetCell("PB1", Number(10))
	engine.SetCell("PG1", Number(150))
	engine.SetCell("PG2", Number(250))
	engine.SetCell("PH1", Number(2))
	engine.SetCell("PH2", Number(4))
	engine.SetCell("PI1", Number(10))
	engine.SetCell("PJ1", Number(150))
	engine.SetCell("PS1", Number(15))
	engine.SetCell("PT1", Number(25))
	engine.SetCell("RX1", Number(2))
	engine.SetCell("RY1", Text("c"))
	engine.SetCell("RU1", Number(1))
	engine.SetCell("RV1", Text("b"))
	engine.SetCell("RW1", Number(3))
	engine.SetCell("RU2", Number(10))
	engine.SetCell("RV2", Number(20))
	engine.SetCell("RW2", Number(30))
	engine.SetCell("TD1", Number(2))
	engine.SetCell("TD2", Text("c"))
	engine.SetCell("TB1", Number(1))
	engine.SetCell("TB2", Text("b"))
	engine.SetCell("TB3", Number(3))
	engine.SetCell("TC1", Number(100))
	engine.SetCell("TC2", Number(200))
	engine.SetCell("TC3", Number(300))
	engine.SetCell("UC1", Number(6))
	engine.SetCell("UC2", Number(0))
	engine.SetCell("UA1", Number(5))
	engine.SetCell("UA2", Number(3))
	engine.SetCell("UA3", Number(1))
	engine.SetCell("UB1", Number(50))
	engine.SetCell("UB2", Number(30))
	engine.SetCell("UB3", Number(10))
	engine.SetCell("VH1", Number(6))
	engine.SetCell("VI1", Number(0))
	engine.SetCell("VE1", Number(5))
	engine.SetCell("VF1", Number(3))
	engine.SetCell("VG1", Number(1))
	engine.SetCell("VE2", Number(50))
	engine.SetCell("VF2", Number(30))
	engine.SetCell("VG2", Number(10))
	engine.SetCell("XU1", Text("b*"))
	engine.SetCell("XV1", Text("g*"))
	engine.SetCell("XW1", Text("alpha"))
	engine.SetCell("XW2", Text("beta"))
	engine.SetCell("XW3", Text("gamma"))
	engine.SetCell("XX1", Number(10))
	engine.SetCell("XX2", Number(20))
	engine.SetCell("XX3", Number(30))
	engine.SetCell("XY1", Text("Q1"))
	engine.SetCell("XZ1", Text("Q2"))
	engine.SetCell("YA1", Text("Q3"))
	engine.SetCell("XY2", Number(100))
	engine.SetCell("XZ2", Number(200))
	engine.SetCell("YA2", Number(300))
	engine.SetCell("XY3", Number(1000))
	engine.SetCell("XZ3", Number(2000))
	engine.SetCell("YA3", Number(3000))
	engine.SetCell("FP1", Text("beta"))
	engine.SetCell("FP2", Text("delta"))
	engine.SetCell("FR1", Number(1))
	engine.SetCell("FR2", Number(3))
	engine.SetCell("LO1", Text("q1"))
	engine.SetCell("LP1", Text("q2"))
	engine.SetCell("LQ1", Text("q3"))
	engine.SetCell("LO2", Number(11))
	engine.SetCell("LP2", Number(22))
	engine.SetCell("LQ2", Number(33))
	engine.SetCell("LO3", Number(111))
	engine.SetCell("LP3", Number(222))
	engine.SetCell("LQ3", Number(333))
	engine.SetCell("OD1", Number(1))
	engine.SetCell("OE1", Number(10))
	engine.SetCell("OF1", Number(100))
	engine.SetCell("OD2", Number(2))
	engine.SetCell("OE2", Number(20))
	engine.SetCell("OF2", Number(200))
	engine.SetCell("OD3", Number(3))
	engine.SetCell("OE3", Number(30))
	engine.SetCell("OF3", Number(300))
	engine.SetCell("OA1", Number(10))
	engine.SetCell("OB1", Number(50))
	engine.SetCell("OC1", Number(7))
	engine.SetCell("OC2", Number(8))
	engine.SetCell("OC3", Number(9))
	engine.SetCell("LA1", Number(2))
	engine.SetCell("LB1", Number(6))
	engine.SetCell("MO1", Text("q1"))
	engine.SetCell("MO2", Text("q2"))
	engine.SetCell("LM1", Number(100))
	engine.SetCell("LN1", Number(1000))
	engine.SetCell("LM2", Number(200))
	engine.SetCell("LN2", Number(2000))
	engine.SetCell("LM3", Number(300))
	engine.SetCell("LN3", Number(3000))
	engine.SetCell("HC1", Number(2))
	engine.SetCell("HD1", Number(3))
	engine.SetCell("II1", Text("q1"))
	engine.SetCell("IJ1", Text("q2"))
	engine.SetCell("IK1", Number(100))
	engine.SetCell("IL1", Number(200))
	engine.SetCell("HI1", Number(0))
	engine.SetCell("HI2", Number(0))
	engine.SetCell("HJ1", Number(3))
	engine.SetCell("HK1", Number(5))
	engine.SetCell("P1", Number(1))
	engine.SetCell("Q1", Number(2))
	engine.SetCell("P2", Number(3))
	engine.SetCell("Q2", Number(4))
	engine.SetCell("AC1", Number(2))
	engine.SetCell("AC2", Number(3))
	engine.SetFormula("D1", `A1:A3+B1:B3`)
	engine.SetFormula("E1", `IF(A1:A3>1,B1:B3,0)`)
	engine.SetFormula("F1", `A1:A3`)
	engine.SetFormula("G1", `IFS(TRUE,A1:A3,FALSE,B1:B3)`)
	engine.SetFormula("H1", `SWITCH(C1,1,A1:A3,2,B1:B3,0)`)
	engine.SetFormula("I1", `IFERROR(UNKNOWNFUNC(),A1:A3)`)
	engine.SetFormula("J1", `IFNA(#N/A,B1:B3)`)
	engine.SetFormula("K1", `SWITCH(A1:A3,1,100,2,200,300)`)
	engine.SetFormula("L1", `IFS(A1:A3=1,10,A1:A3=2,20,TRUE,30)`)
	engine.SetFormula("M1", `IFERROR(IF(A1:A3=1,UNKNOWNFUNC(),A1:A3),0)`)
	engine.SetFormula("N1", `IFNA(IF(A1:A3=1,#N/A,A1:A3),0)`)
	engine.SetFormula("R1", `IFERROR(IF(P1:Q2=1,UNKNOWNFUNC(),P1:Q2),0)`)
	engine.SetFormula("T1", `IFNA(IF(P1:Q2=1,#N/A,P1:Q2),0)`)
	engine.SetFormula("V1", `HSTACK(A1:A3,B1:B3)`)
	engine.SetFormula("X1", `VSTACK(A1:B1,A2:B2,A3:B3)`)
	engine.SetFormula("Z1", `EXPAND(A1:B2,3,3,0)`)
	engine.SetFormula("AD1", `MAKEARRAY(2,3,LAMBDA(r,c,r*10+c))`)
	engine.SetFormula("AH1", `BYROW(A1:B2,LAMBDA(row,SUM(row)))`)
	engine.SetFormula("AJ1", `BYCOL(A1:B2,LAMBDA(col,SUM(col)))`)
	engine.SetFormula("AL1", `MAP(A1:A3,B1:B3,LAMBDA(a,b,a+b))`)
	engine.SetFormula("AN1", `SCAN(0,A1:A3,LAMBDA(acc,v,acc+v))`)
	// remaining context-aware inference paths
	engine.SetFormula("AP1", `CHOOSE(2,A1:A3,B1:B3)`)
	engine.SetFormula("AR1", `CHOOSECOLS(A1:B3,2)`)
	engine.SetFormula("AT1", `CHOOSEROWS(A1:B3,1,3)`)
	engine.SetFormula("AV1", `SEQUENCE(3,2)`)
	engine.SetFormula("AY1", `SORT(B1:B3)`)
	engine.SetFormula("BB1", `SORTBY(B1:B3,A1:A3,-1)`)
	engine.SetFormula("BE1", `TAKE(A1:B3,2)`)
	engine.SetFormula("BH1", `DROP(A1:B3,1)`)
	engine.SetFormula("BK1", `TOROW(A1:A3)`)
	engine.SetFormula("BN1", `TOCOL(A1:B1)`)
	engine.SetFormula("BP1", `WRAPROWS(A1:A3,2,0)`)
	engine.SetFormula("BS1", `WRAPCOLS(A1:A3,2,0)`)
	engine.SetFormula("BV1", `UNIQUE(A1:A3)`)
	engine.SetFormula("BX1", `FILTER(A1:A3,A1:A3>1)`)
	engine.SetFormula("BZ1", `TRANSPOSE(A1:B1)`)
	engine.SetFormula("CB1", `OFFSET(A1,0,0,3,2)`)
	engine.SetFormula("CF1", `INDEX(A1:B3,0,1)`)
	engine.SetFormula("CH1", `XLOOKUP(C1,A1:A3,A1:B3)`)
	engine.SetFormula("CK1", `RANDARRAY(2,3)`)
	engine.SetFormula("CN1", `TEXTSPLIT("a,b,c",",")`)
	engine.SetFormula("CQ1", `LET(x,A1:A3,x)`)
	engine.SetFormula("CT1", `XLOOKUP(A1:A3,A1:A3,B1:B3)`)
	engine.SetFormula("KT1", `XLOOKUP(A1:A2,A1:A3,A1:B3)`)
	engine.SetFormula("MA1", `XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3)`)
	engine.SetFormula("MD1", `XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3,II1:IJ1,0)`)
	engine.SetFormula("CV1", `MATCH(A1:A3,A1:A3,0)`)
	engine.SetFormula("HE1", `MATCH(HC1:HD1,FE1:FE4,FB1:FB2)`)
	engine.SetFormula("CY1", `XMATCH(A1:A3,A1:A3)`)
	engine.SetFormula("DB1", `LOOKUP(A1:A3,A1:A3,B1:B3)`)
	engine.SetFormula("ID1", `LOOKUP(HC1:HD1,FE1:FE4,FE1:FE4)`)
	engine.SetFormula("KY1", `LOOKUP(II1:IJ1,LO1:LQ3)`)
	engine.SetFormula("KY3", `LOOKUP(IK1:IL1,LM1:LN3)`)
	engine.SetFormula("MC1", `LOOKUP(MO1:MO2,LO1:LQ3)`)
	engine.SetFormula("MF1", `VLOOKUP(A1:A2,EG1:EI3,EJ1:EK1,FALSE)`)
	engine.SetFormula("MH1", `HLOOKUP(EM1:EN1,EM1:EO3,EP1:EP2,FALSE)`)
	engine.SetFormula("DE1", `VLOOKUP(A1:A3,A1:B3,2,FALSE)`)
	engine.SetFormula("DG1", `HLOOKUP(EA1:EC1,EA1:EC2,2,FALSE)`)
	engine.SetFormula("DK1", `VLOOKUP(C1,EG1:EI3,EJ1:EK1,FALSE)`)
	engine.SetFormula("FX1", `VLOOKUP(C1,EG1:EI3,EJ1:EK1,FJ1:FJ2)`)
	engine.SetFormula("DM1", `HLOOKUP("Q2",EM1:EO3,EP1:EP2,FALSE)`)
	engine.SetFormula("GB1", `HLOOKUP("Q2",EM1:EO3,EP1:EP2,FK1:FL1)`)
	engine.SetFormula("DO1", `XMATCH(FA1:FA2,FA1:FA3,0,FB1:FB2)`)
	engine.SetFormula("HL1", `XMATCH(HJ1:HK1,FE1:FE4,HI1:HI2)`)
	engine.SetFormula("KI1", `XMATCH(HJ1:HK1,FE1:FE4,FK1:FL1,FB1:FB2)`)
	engine.SetFormula("DQ1", `XLOOKUP(FA1:FA2,FA1:FA3,FC1:FC3,"",0,FB1:FB2)`)
	engine.SetFormula("HO1", `XLOOKUP(HJ1:HK1,FE1:FE4,FE1:FE4,"",HI1:HI2)`)
	engine.SetFormula("KK1", `XLOOKUP(HJ1:HK1,FE1:FE4,FE1:FE4,"",FK1:FL1,FB1:FB2)`)
	engine.SetFormula("DU1", `VLOOKUP(FG1:FG2,FH1:FI3,2,FJ1:FJ2)`)
	engine.SetFormula("DW1", `XMATCH("beta",FA1:FA3,0,FB1:FB2)`)
	engine.SetFormula("HU1", `XMATCH("beta",FA1:FA3,0,FK1:FL1)`)
	engine.SetFormula("HY1", `XMATCH(4,FE1:FE4,FK1:FL1,FB1:FB2)`)
	engine.SetFormula("FO1", `XMATCH(4,FE1:FE4,FB1:FB2,FK1:FL1)`)
	engine.SetFormula("HQ1", `MATCH(4,FE1:FE4,FB1:FB2)`)
	engine.SetFormula("HR1", `MATCH(4,FE1:FE4,FK1:FL1)`)
	engine.SetFormula("DY1", `XLOOKUP("beta",FA1:FA3,FC1:FC3,"",0,FB1:FB2)`)
	engine.SetFormula("EB4", `XLOOKUP("beta",FA1:FA3,FC1:FC3,"",FB1:FB2,FK1:FL1)`)
	engine.SetFormula("HW1", `XLOOKUP(4,FE1:FE4,FE1:FE4,"",FK1:FL1,1)`)
	engine.SetFormula("JC6", `XLOOKUP(4,FE1:FE4,FE1:FE4,"",FK1:FL1,FB1:FB2)`)
	engine.SetFormula("FM1", `XLOOKUP("beta",FA1:FA3,FC1:FC3,FB1:FB2,0,FK1:FL1)`)
	engine.SetFormula("FC4", `XLOOKUP("beta",FA1:FA3,FC1:FC3,FC1:FC3,0,FB1:FB2)`)
	engine.SetFormula("FB4", `XLOOKUP(FA1:FA2,FA1:FA3,FC1:FC3,FC1:FD1,0)`)
	engine.SetFormula("EA4", `XLOOKUP("delta",FA1:FA3,FC1:FC3,FC1:FC2,0)`)
	engine.SetFormula("FQ1", `XLOOKUP(FP1:FP2,FA1:FA3,FC1:FC3,FC1:FC2,0)`)
	engine.SetFormula("GE1", `XLOOKUP(FP1:FP2,FA1:FA3,FC1:FD3,FD1:FE1,0)`)
	engine.SetFormula("GK1", `XMATCH(LA1:LB1,FE1:FE4,FK1:FL1)`)
	engine.SetFormula("OA4", `XLOOKUP(OA1:OB1,OD1:OF1,OD1:OF3,OC1:OC3,0)`)
	engine.SetFormula("FS1", `MATCH(FR1:FR2,FE1:FE4,FB1:FB2)`)
	engine.SetFormula("FT1", `MATCH(FR1:FR2,FE1:FE4,FK1:FL1)`)
	engine.SetFormula("OW1", `VLOOKUP(OU1:OV1,FH1:FI3,2,IA1:IB1)`)
	engine.SetFormula("PC1", `HLOOKUP(PA1:PB1,W1:Y3,2,IA1:IB1)`)
	engine.SetFormula("PE1", `VLOOKUP(PH1:PH2,EG1:EI3,EJ1:EK1,FJ1:FJ2)`)
	engine.SetFormula("PK1", `HLOOKUP(PI1:PJ1,EG1:EI3,EP1:EP2,FJ1:FJ2)`)
	engine.SetFormula("PM1", `LOOKUP(PS1:PT1,LO2:LQ3)`)
	engine.SetFormula("PR1", `LOOKUP(PG1:PG2,LM1:LN3)`)
	engine.SetFormula("RZ1", `LOOKUP(RX1:RY1,RU1:RW2)`)
	engine.SetFormula("TE1", `LOOKUP(TD1:TD2,TB1:TC3)`)
	engine.SetFormula("UD1", `VLOOKUP(UC1:UC2,UA1:UB3,2,TRUE)`)
	engine.SetFormula("VJ1", `HLOOKUP(VH1:VI1,VE1:VG2,2,TRUE)`)
	engine.SetFormula("XB1", `VLOOKUP(XU1:XV1,XW1:XX3,2,TRUE)`)
	engine.SetFormula("XD1", `HLOOKUP("Q2*",XY1:YA3,AC1:AC2,TRUE)`)
	engine.SetFormula("EZ1", `VLOOKUP(4,FH1:FI3,2,FJ1:FJ2)`)
	engine.SetFormula("IZ1", `VLOOKUP(4,FH1:FI3,2,IA1:IB1)`)
	engine.SetFormula("JB1", `HLOOKUP(2,W1:Y3,2,IA1:IB1)`)

	for ref, want := range map[string]float64{
		"D1": 11,
		"D2": 22,
		"D3": 33,
		"E1": 0,
		"E2": 20,
		"E3": 30,
		"F1": 1,
		"F2": 2,
		"F3": 3,
		"G1": 1,
		"G2": 2,
		"G3": 3,
		"H1": 10,
		"H2": 20,
		"H3": 30,
		"I1": 1,
		"I2": 2,
		"I3": 3,
		"J1": 10,
		"J2": 20,
		"J3": 30,
		"K1": 100,
		"K2": 200,
		"K3": 300,
		"L1": 10,
		"L2": 20,
		"L3": 30,
		"M1": 0,
		"M2": 2,
		"M3": 3,
		"N1": 0,
		"N2": 2,
		"N3": 3,
		"R1": 0,
		"S1": 2,
		"R2": 3,
		"S2": 4,
		"T1": 0,
		"U1": 2,
		"T2": 3,
		"U2": 4,
		// HSTACK(A1:A3, B1:B3) → 3 rows × 2 cols anchored at V1
		"V1": 1, "W1": 10,
		"V2": 2, "W2": 20,
		"V3": 3, "W3": 30,
		// VSTACK(A1:B1, A2:B2, A3:B3) → 3 rows × 2 cols anchored at X1
		"X1": 1, "Y1": 10,
		"X2": 2, "Y2": 20,
		"X3": 3, "Y3": 30,
		// EXPAND(A1:B2,3,3,0) → 3×3, pads with 0
		"Z1": 1, "AA1": 10, "AB1": 0,
		"Z2": 2, "AA2": 20, "AB2": 0,
		"Z3": 0, "AA3": 0, "AB3": 0,
		// MAKEARRAY(2,3,LAMBDA(r,c,r*10+c))
		"AD1": 11, "AE1": 12, "AF1": 13,
		"AD2": 21, "AE2": 22, "AF2": 23,
		// BYROW(A1:B2, SUM) → column of row sums
		"AH1": 11, "AH2": 22,
		// BYCOL(A1:B2, SUM) → row of col sums
		"AJ1": 3, "AK1": 30,
		// MAP(A1:A3, B1:B3, a+b)
		"AL1": 11, "AL2": 22, "AL3": 33,
		// SCAN(0, A1:A3, acc+v)
		"AN1": 1, "AN2": 3, "AN3": 6,
		// CHOOSE(2,A1:A3,B1:B3) → B1:B3
		"AP1": 10, "AP2": 20, "AP3": 30,
		// CHOOSECOLS(A1:B3,2) → col B
		"AR1": 10, "AR2": 20, "AR3": 30,
		// CHOOSEROWS(A1:B3,1,3) → rows 1,3 → 2×2
		"AT1": 1, "AU1": 10, "AT2": 3, "AU2": 30,
		// SEQUENCE(3,2) → [[1,2],[3,4],[5,6]]
		"AV1": 1, "AW1": 2, "AV2": 3, "AW2": 4, "AV3": 5, "AW3": 6,
		// SORT(B1:B3) ascending → [10,20,30]
		"AY1": 10, "AY2": 20, "AY3": 30,
		// SORTBY(B1:B3,A1:A3,-1) → sort B by A descending: B3,B2,B1
		"BB1": 30, "BB2": 20, "BB3": 10,
		// TAKE(A1:B3,2) → first 2 rows
		"BE1": 1, "BF1": 10, "BE2": 2, "BF2": 20,
		// DROP(A1:B3,1) → drop row 1
		"BH1": 2, "BI1": 20, "BH2": 3, "BI2": 30,
		// TOROW(A1:A3) → [1,2,3] as row
		"BK1": 1, "BL1": 2, "BM1": 3,
		// TOCOL(A1:B1) → [1,10] as column
		"BN1": 1, "BN2": 10,
		// WRAPROWS(A1:A3,2,0) → [[1,2],[3,0]]
		"BP1": 1, "BQ1": 2, "BP2": 3, "BQ2": 0,
		// WRAPCOLS(A1:A3,2,0) → [[1,3],[2,0]]
		"BS1": 1, "BT1": 3, "BS2": 2, "BT2": 0,
		// UNIQUE(A1:A3) → [1,2,3] (all unique)
		"BV1": 1, "BV2": 2, "BV3": 3,
		// FILTER(A1:A3,A1:A3>1) → [2,3]
		"BX1": 2, "BX2": 3,
		// TRANSPOSE(A1:B1) → [[1],[10]] as 2×1
		"BZ1": 1, "BZ2": 10,
		// OFFSET(A1,0,0,3,2) → A1:B3
		"CB1": 1, "CC1": 10, "CB2": 2, "CC2": 20, "CB3": 3, "CC3": 30,
		// INDEX(A1:B3,0,1) → entire col 1 = [1,2,3]
		"CF1": 1, "CF2": 2, "CF3": 3,
		// XLOOKUP(C1,A1:A3,A1:B3) → C1=2 → row 2 of A1:B3 = [2,20] → 1×2
		"CH1": 2, "CI1": 20,
		// XLOOKUP(A1:A3,A1:A3,B1:B3) → [10,20,30]
		"CT1": 10, "CT2": 20, "CT3": 30,
		// XLOOKUP(A1:A2,A1:A3,A1:B3) → [[1,10],[2,20]]
		"KT1": 1, "KU1": 10, "KT2": 2, "KU2": 20,
		// XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3) → [[q1,q2],[11,22],[111,222]]
		"MA2": 11, "MB2": 22, "MA3": 111, "MB3": 222,
		// XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3,II1:IJ1,0) found-case ignores incompatible if_not_found shape
		"MD2": 11, "ME2": 22, "MD3": 111, "ME3": 222,
		// MATCH(A1:A3,A1:A3,0) → [1,2,3]
		"CV1": 1, "CV2": 2, "CV3": 3,
		// MATCH(HC1:HD1,FE1:FE4,FB1:FB2) → [1,2], lookup_value shape dominates incompatible match_type orientation
		"HE1": 1, "HF1": 2,
		// XMATCH(A1:A3,A1:A3) → [1,2,3]
		"CY1": 1, "CY2": 2, "CY3": 3,
		// LOOKUP(A1:A3,A1:A3,B1:B3) → [10,20,30]
		"DB1": 10, "DB2": 20, "DB3": 30,
		// LOOKUP(HC1:HD1,FE1:FE4,FE1:FE4) → [1,3] horizontal spill from horizontal lookup_value
		"ID1": 1, "IE1": 3,
		// LOOKUP(II1:IJ1,LO1:LQ3) → [111,222] horizontal spill from row-form 2D source
		"KY1": 111, "KZ1": 222,
		// LOOKUP(IK1:IL1,LM1:LN3) → [1000,2000] horizontal spill from column-form 2D source
		"KY3": 1000, "KZ3": 2000,
		// LOOKUP(MO1:MO2,LO1:LQ3) → [111,222] vertical spill from row-form 2D source
		"MC1": 111, "MC2": 222,
		// VLOOKUP(A1:A2,EG1:EI3,EJ1:EK1,FALSE) → [[10,100],[20,200]]
		"MF1": 10, "MG1": 100, "MF2": 20, "MG2": 200,
		// HLOOKUP(EM1:EN1,EM1:EO3,EP1:EP2,FALSE) → [[100,200],[1000,2000]]
		"MH1": 100, "MI1": 200, "MH2": 1000, "MI2": 2000,
		// VLOOKUP(A1:A3,A1:B3,2,FALSE) → [10,20,30]
		"DE1": 10, "DE2": 20, "DE3": 30,
		// HLOOKUP(EA1:EC1,EA1:EC2,2,FALSE) → [100,200,300]
		"DG1": 100, "DH1": 200, "DI1": 300,
		// VLOOKUP(C1,EG1:EI3,EJ1:EK1,FALSE) → [20,200]
		"DK1": 20, "DL1": 200,
		// VLOOKUP(C1,EG1:EI3,EJ1:EK1,FJ1:FJ2) → [20,200], selector shape dominates incompatible range_lookup
		"FX1": 20, "FY1": 200,
		// HLOOKUP("Q2",EM1:EO3,EP1:EP2,FALSE) → [200,2000]
		"DM1": 200, "DM2": 2000,
		// HLOOKUP("Q2",EM1:EO3,EP1:EP2,FK1:FL1) → [200,2000], selector shape dominates incompatible range_lookup
		"GB1": 200, "GB2": 2000,
		// XMATCH with per-element search_mode: [1,-1] over duplicate lookup values
		"DO1": 1, "DO2": 2,
		// XMATCH vector lookup with incompatible match_mode orientation keeps lookup_value spill shape
		"HL1": 2, "HM1": 3,
		// XMATCH vector lookup with horizontal match_mode and vertical search_mode keeps horizontal spill
		"KI1": 2, "KJ1": 3,
		// XLOOKUP with per-element search_mode: [1,-1] over duplicate lookup values
		"DQ1": 10, "DQ2": 20,
		// XLOOKUP vector lookup with incompatible match_mode orientation keeps lookup_value spill shape
		"HO1": 3, "HP1": 5,
		// XLOOKUP vector lookup with horizontal match_mode and vertical search_mode keeps horizontal spill
		"KK1": 3, "KL1": 5,
		// VLOOKUP with per-element range_lookup: TRUE returns floor match
		"DU1": 30,
		// XMATCH with scalar lookup and per-element search_mode
		"DW1": 1, "DW2": 2,
		// XMATCH scalar lookup with horizontal search_mode spills horizontally
		"HU1": 1, "HV1": 2,
		// XMATCH scalar lookup with horizontal match_mode and vertical search_mode keeps horizontal spill
		"HY1": 3, "HZ1": 2,
		// XMATCH with incompatible match/search mode array shapes keeps dominant match_mode spill shape
		"FO1": 3, "FO2": 2,
		// MATCH scalar lookup with per-element match_type spills by match_type orientation
		"HQ1": 2, "HR1": 2,
		// XLOOKUP with scalar lookup and per-element search_mode
		"DY1": 10, "DY2": 20,
		// XLOOKUP scalar lookup with horizontal match_mode spills horizontally
		"HW1": 5, "HX1": 3,
		// XLOOKUP scalar lookup with horizontal match_mode and vertical search_mode keeps horizontal spill
		"JC6": 5, "JD6": 3,
		// XLOOKUP with incompatible match/search mode array shapes keeps dominant mode spill shape
		"EB4": 10, "EB5": 10,
		// XLOOKUP with horizontal search_mode and incompatible if_not_found shape spills horizontally by search_mode
		"FM1": 10, "FN1": 20,
		// XLOOKUP optional-array precedence: incompatible if_not_found shape does not suppress compatible mode-array spill
		"FC4": 10, "FC5": 20,
		// XLOOKUP found-case ignores incompatible array if_not_found shape; both duplicate lookups resolve to first exact match
		"FB4": 10, "FB5": 10,
		// XLOOKUP scalar lookup miss with array if_not_found
		"EA4": 10, "EA5": 20,
		// XLOOKUP vector lookup partial miss: found element returns match, missed element returns if_not_found at projected offset
		"FQ1": 10, "FQ2": 20,
		// XLOOKUP 2D return + partial miss: found row gets per-match return row; missed row gets if_not_found projected across cols
		"GE1": 10, "GF1": 4, "GE2": 4, "GF2": 1,
		// XMATCH horizontal vector lookup with compatible horizontal match_mode: per-element mode drives next-larger/next-smaller
		"GK1": 2, "GL1": 3,
		// XLOOKUP horizontal lookup partial miss: found column returns 3x1 column; missed column uses projected vertical fallback
		"OA4": 10, "OB4": 7, "OA5": 20, "OB5": 8, "OA6": 30, "OB6": 9,
		// MATCH vector lookup with per-element match_type same axis: each cell gets its own match_type value
		"FS1": 1, "FS2": 2,
		// MATCH vector lookup with incompatible match_type axis: match_type clamped to col=0 for all rows
		"FT1": 1, "FT2": 2,
		// VLOOKUP vector lookup with horizontal range_lookup: approximate and exact modes apply per element
		"OW1": 30, "OX1": 50,
		// HLOOKUP vector lookup with horizontal range_lookup: approximate and exact modes apply per element
		"PC1": 2, "PD1": 20,
		// VLOOKUP vector lookup with vertical range_lookup and horizontal selectors: exact/approximate modes project per row
		"PE1": 20, "PF1": 200,
		// HLOOKUP vector lookup with vertical range_lookup and vertical selectors: exact/approximate modes project per row
		"PK1": 20, "PL1": 200, "PK2": 30,
		// LOOKUP horizontal vector with approximate matching over row-form 2D source
		"PM1": 111, "PN1": 222,
		// LOOKUP vertical vector with approximate matching over column-form 2D source
		"PR1": 1000, "PR2": 2000,
		// LOOKUP horizontal mixed-type vector over row-form 2D source
		"RZ1": 10, "SA1": 20,
		// LOOKUP vertical mixed-type vector over column-form 2D source
		"TE1": 100, "TE2": 200,
		// VLOOKUP approximate over descending first column: out-of-range high falls back, low stays #N/A
		"UD1": 10,
		// HLOOKUP approximate over descending first row: out-of-range high falls back, low stays #N/A
		"VJ1": 10,
		// VLOOKUP approximate wildcard literals with horizontal vector lookup values spill horizontally
		"XB1": 10, "XC1": 20,
		// HLOOKUP approximate wildcard literal with vertical selector array spills vertically
		"XD1": 200, "XD2": 2000,
		// VLOOKUP with scalar lookup and per-element range_lookup
		"EZ1": 30,
		// VLOOKUP scalar lookup with horizontal range_lookup spills horizontally
		"IZ1": 30,
		// HLOOKUP scalar lookup with horizontal range_lookup spills horizontally
		"JB1": 2,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	// RANDARRAY(2,3) → 2×3; values are random so just verify shape and numeric
	for _, ref := range []string{"CK1", "CL1", "CM1", "CK2", "CL2", "CM2"} {
		if got := engine.GetCell(ref); !got.IsNumber() {
			t.Fatalf("%s = %#v, want a number (RANDARRAY member)", ref, got)
		}
	}
	// TEXTSPLIT("a,b,c",",") → 1×3 text spill
	for ref, want := range map[string]string{"CN1": "a", "CO1": "b", "CP1": "c"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %q", ref, got, want)
		}
	}
	// LET(x,A1:A3,x) → 3×1 numeric spill
	for ref, want := range map[string]float64{"CQ1": 1, "CQ2": 2, "CQ3": 3} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("DU2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("DU2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("PE2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("PE2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("PF2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("PF2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("PL2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("PL2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("UD2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("UD2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("VK1"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("VK1 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("JA1"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("JA1 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("JC1"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("JC1 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("HQ2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("HQ2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("HS1"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("HS1 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("EZ2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("EZ2 = %#v, want #N/A", got)
	}
}

func TestInferExpressionArrayShapeForBranchConditionals(t *testing.T) {
	for _, tt := range []struct {
		formula string
		rows    int
		cols    int
	}{
		{formula: `IF(A1:A3,B1:B3,0)`, rows: 3, cols: 1},
		{formula: `IF(A1:B2,C1:D2,0)`, rows: 2, cols: 2},
		{formula: `IFS(TRUE,A1:A3,FALSE,B1:B3)`, rows: 3, cols: 1},
		{formula: `IFS(A1:B2=1,10,TRUE,20)`, rows: 2, cols: 2},
		{formula: `SWITCH(A1:A3,1,100,2,200,300)`, rows: 3, cols: 1},
		{formula: `SWITCH(A1:B2,1,100,2,200,300)`, rows: 2, cols: 2},
		{formula: `IFERROR(A1:A3,0)`, rows: 3, cols: 1},
		{formula: `IFERROR(A1:B2,0)`, rows: 2, cols: 2},
		{formula: `IFNA(A1:A3,0)`, rows: 3, cols: 1},
		{formula: `IFNA(A1:B2,0)`, rows: 2, cols: 2},
		{formula: `HSTACK(A1:A3,B1:B3)`, rows: 3, cols: 2},
		{formula: `VSTACK(A1:B1,A2:B2)`, rows: 2, cols: 2},
		{formula: `HSTACK(A1:B2,D1:E2)`, rows: 2, cols: 4},
		{formula: `VSTACK(A1:B2,A3:B4)`, rows: 4, cols: 2},
		// TRANSPOSE: flips rows and cols
		{formula: `TRANSPOSE(A1:B3)`, rows: 2, cols: 3},
		// CHOOSE: static selector picks the branch
		{formula: `CHOOSE(2,A1,B1:B3)`, rows: 3, cols: 1},
		// CHOOSECOLS: select two columns from a 3×3 range → 3×2
		{formula: `CHOOSECOLS(A1:C3,1,3)`, rows: 3, cols: 2},
		// CHOOSEROWS: select two rows from a 3×3 range → 2×3
		{formula: `CHOOSEROWS(A1:C3,1,3)`, rows: 2, cols: 3},
		// SORT: preserves source shape
		{formula: `SORT(A1:B3)`, rows: 3, cols: 2},
		// SORTBY: preserves first-arg shape
		{formula: `SORTBY(A1:B3,A1:A3)`, rows: 3, cols: 2},
		// SEQUENCE: literal dimensions
		{formula: `SEQUENCE(4,3)`, rows: 4, cols: 3},
		// RANDARRAY: literal dimensions
		{formula: `RANDARRAY(2,4)`, rows: 2, cols: 4},
		// EXPAND: expand A1:B2 (2×2) to 4 rows × 3 cols
		{formula: `EXPAND(A1:B2,4,3)`, rows: 4, cols: 3},
		// MAKEARRAY: literal rows × cols (lambda arg ignored for shape)
		{formula: `MAKEARRAY(3,4,LAMBDA(r,c,r+c))`, rows: 3, cols: 4},
		// BYROW: reduces each row → (rows,1)
		{formula: `BYROW(A1:B3,LAMBDA(r,SUM(r)))`, rows: 3, cols: 1},
		// BYCOL: reduces each column → (1,cols)
		{formula: `BYCOL(A1:B3,LAMBDA(c,SUM(c)))`, rows: 1, cols: 2},
		// MAP: broadcasts all input arrays, last arg is lambda
		{formula: `MAP(A1:B3,A1:B3,LAMBDA(a,b,a+b))`, rows: 3, cols: 2},
		// SCAN: shape of the accumulation array (args[1])
		{formula: `SCAN(0,A1:B3,LAMBDA(acc,v,acc+v))`, rows: 3, cols: 2},
		// TAKE: first 2 rows, first 2 cols of A1:C4
		{formula: `TAKE(A1:C4,2,2)`, rows: 2, cols: 2},
		// DROP: drop first row and first col of A1:C4 (4×3) → 3×2
		{formula: `DROP(A1:C4,1,1)`, rows: 3, cols: 2},
		// TOROW: flatten 3×1 range to 1×3 row
		{formula: `TOROW(A1:A3)`, rows: 1, cols: 3},
		// TOCOL: flatten 1×3 range to 3×1 column
		{formula: `TOCOL(A1:C1)`, rows: 3, cols: 1},
		// WRAPROWS: 6-element vector wrapped at width 3 → 2×3
		{formula: `WRAPROWS(A1:A6,3)`, rows: 2, cols: 3},
		// WRAPCOLS: 6-element vector wrapped at height 3 → 3×2
		{formula: `WRAPCOLS(A1:A6,3)`, rows: 3, cols: 2},
		// TEXTSPLIT: literal string "a,b,c" split by "," → 1×3
		{formula: `TEXTSPLIT("a,b,c",",")`, rows: 1, cols: 3},
		// INDEX whole-column selector: INDEX(3×3,0,2) → column 2 → 3×1
		{formula: `INDEX(A1:C3,0,2)`, rows: 3, cols: 1},
		// OFFSET with explicit height/width: OFFSET(A1,0,0,3,2) → 3×2
		{formula: `OFFSET(A1,0,0,3,2)`, rows: 3, cols: 2},
		// XLOOKUP: scalar lookup, multi-col return → 1×2 horizontal spill
		{formula: `XLOOKUP(C1,A1:A3,A1:B3)`, rows: 1, cols: 2},
		// XLOOKUP: vector lookup_value with single-col return → same 3×1 spill shape
		{formula: `XLOOKUP(A1:A3,A1:A3,B1:B3)`, rows: 3, cols: 1},
		// XLOOKUP: vector lookup_value with 2D row return produces a 2×2 spill
		{formula: `XLOOKUP(A1:A2,A1:A3,A1:B3)`, rows: 2, cols: 2},
		// XLOOKUP: horizontal lookup_value with 2D column return produces a 3×2 spill
		{formula: `XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3)`, rows: 3, cols: 2},
		// XLOOKUP: found-case non-scalar return remains dominant over incompatible if_not_found shape
		{formula: `XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3,II1:IJ1,0)`, rows: 3, cols: 2},
		// MATCH: vector lookup_value drives a 3×1 spill
		{formula: `MATCH(A1:A3,A1:A3,0)`, rows: 3, cols: 1},
		// MATCH: lookup_value array remains dominant with incompatible match_type orientation
		{formula: `MATCH(HC1:HD1,FE1:FE4,FB1:FB2)`, rows: 1, cols: 2},
		// XMATCH: vector lookup_value drives a 3×1 spill
		{formula: `XMATCH(A1:A3,A1:A3)`, rows: 3, cols: 1},
		// LOOKUP: vector lookup_value drives a 3×1 spill
		{formula: `LOOKUP(A1:A3,A1:A3,B1:B3)`, rows: 3, cols: 1},
		// LOOKUP: horizontal lookup_value drives a 1×2 spill
		{formula: `LOOKUP(HC1:HD1,FE1:FE4,FE1:FE4)`, rows: 1, cols: 2},
		// LOOKUP: horizontal lookup_value over row-form 2D source spills horizontally
		{formula: `LOOKUP(II1:IJ1,LO1:LQ3)`, rows: 1, cols: 2},
		// LOOKUP: horizontal lookup_value over column-form 2D source spills horizontally
		{formula: `LOOKUP(IK1:IL1,LM1:LN3)`, rows: 1, cols: 2},
		// LOOKUP: vertical lookup_value over row-form 2D source still spills only by lookup_value shape
		{formula: `LOOKUP(MO1:MO2,LO1:LQ3)`, rows: 2, cols: 1},
		// VLOOKUP: vector lookup_value and horizontal selector combine into a 2×2 spill
		{formula: `VLOOKUP(A1:A2,EG1:EI3,EJ1:EK1,FALSE)`, rows: 2, cols: 2},
		// HLOOKUP: horizontal lookup_value and vertical selector combine into a 2×2 spill
		{formula: `HLOOKUP(EM1:EN1,EM1:EO3,EP1:EP2,FALSE)`, rows: 2, cols: 2},
		// VLOOKUP: vector lookup_value drives a 3×1 spill
		{formula: `VLOOKUP(A1:A3,A1:B3,2,FALSE)`, rows: 3, cols: 1},
		// HLOOKUP: vector lookup_value drives a 1×3 spill
		{formula: `HLOOKUP(EA1:EC1,EA1:EC2,2,FALSE)`, rows: 1, cols: 3},
		// VLOOKUP: selector array drives a 1×2 spill
		{formula: `VLOOKUP(C1,EG1:EI3,EJ1:EK1,FALSE)`, rows: 1, cols: 2},
		// VLOOKUP: selector array remains dominant with incompatible range_lookup orientation
		{formula: `VLOOKUP(C1,EG1:EI3,EJ1:EK1,FJ1:FJ2)`, rows: 1, cols: 2},
		// HLOOKUP: selector array drives a 2×1 spill
		{formula: `HLOOKUP("Q2",EM1:EO3,EP1:EP2,FALSE)`, rows: 2, cols: 1},
		// HLOOKUP: selector array remains dominant with incompatible range_lookup orientation
		{formula: `HLOOKUP("Q2",EM1:EO3,EP1:EP2,FK1:FL1)`, rows: 2, cols: 1},
		// XMATCH: vector lookup_value with vector search_mode
		{formula: `XMATCH(FA1:FA2,FA1:FA3,0,FB1:FB2)`, rows: 2, cols: 1},
		// XMATCH: vector lookup_value remains dominant with incompatible match_mode orientation
		{formula: `XMATCH(HJ1:HK1,FE1:FE4,HI1:HI2)`, rows: 1, cols: 2},
		// XMATCH: vector lookup_value with horizontal match_mode and vertical search_mode
		{formula: `XMATCH(HJ1:HK1,FE1:FE4,FK1:FL1,FB1:FB2)`, rows: 1, cols: 2},
		// XLOOKUP: vector lookup_value with vector search_mode
		{formula: `XLOOKUP(FA1:FA2,FA1:FA3,FC1:FC3,"",0,FB1:FB2)`, rows: 2, cols: 1},
		// XLOOKUP: vector lookup_value remains dominant with incompatible match_mode orientation
		{formula: `XLOOKUP(HJ1:HK1,FE1:FE4,FE1:FE4,"",HI1:HI2)`, rows: 1, cols: 2},
		// XLOOKUP: vector lookup_value with horizontal match_mode and vertical search_mode
		{formula: `XLOOKUP(HJ1:HK1,FE1:FE4,FE1:FE4,"",FK1:FL1,FB1:FB2)`, rows: 1, cols: 2},
		// VLOOKUP: vector lookup_value with vector range_lookup
		{formula: `VLOOKUP(FG1:FG2,FH1:FI3,2,FJ1:FJ2)`, rows: 2, cols: 1},
		// XMATCH: scalar lookup with vector search_mode
		{formula: `XMATCH("beta",FA1:FA3,0,FB1:FB2)`, rows: 2, cols: 1},
		// XMATCH: scalar lookup with horizontal search_mode
		{formula: `XMATCH("beta",FA1:FA3,0,FK1:FL1)`, rows: 1, cols: 2},
		// XMATCH: scalar lookup with horizontal match_mode and vertical search_mode
		{formula: `XMATCH(4,FE1:FE4,FK1:FL1,FB1:FB2)`, rows: 1, cols: 2},
		// XMATCH: incompatible match/search mode arrays retain dominant match_mode spill shape
		{formula: `XMATCH(4,FE1:FE4,FB1:FB2,FK1:FL1)`, rows: 2, cols: 1},
		// MATCH: scalar lookup with vertical match_type array
		{formula: `MATCH(4,FE1:FE4,FB1:FB2)`, rows: 2, cols: 1},
		// MATCH: scalar lookup with horizontal match_type array
		{formula: `MATCH(4,FE1:FE4,FK1:FL1)`, rows: 1, cols: 2},
		// XLOOKUP: scalar lookup with vector search_mode
		{formula: `XLOOKUP("beta",FA1:FA3,FC1:FC3,"",0,FB1:FB2)`, rows: 2, cols: 1},
		// XLOOKUP: scalar lookup with horizontal match_mode array
		{formula: `XLOOKUP(4,FE1:FE4,FE1:FE4,"",FK1:FL1,1)`, rows: 1, cols: 2},
		// XLOOKUP: scalar lookup with horizontal match_mode and vertical search_mode
		{formula: `XLOOKUP(4,FE1:FE4,FE1:FE4,"",FK1:FL1,FB1:FB2)`, rows: 1, cols: 2},
		// XLOOKUP: incompatible match/search mode arrays retain dominant mode shape
		{formula: `XLOOKUP("beta",FA1:FA3,FC1:FC3,"",FB1:FB2,FK1:FL1)`, rows: 2, cols: 1},
		// XLOOKUP: horizontal search_mode with incompatible if_not_found shape retains horizontal mode spill
		{formula: `XLOOKUP("beta",FA1:FA3,FC1:FC3,FB1:FB2,0,FK1:FL1)`, rows: 1, cols: 2},
		// XLOOKUP: scalar lookup with compatible mode-array and incompatible if_not_found array still spills by mode-array
		{formula: `XLOOKUP("beta",FA1:FA3,FC1:FC3,FC1:FC3,0,FB1:FB2)`, rows: 2, cols: 1},
		// XLOOKUP: vector lookup with incompatible if_not_found shape still spills from lookup_value
		{formula: `XLOOKUP(FA1:FA2,FA1:FA3,FC1:FC3,FC1:FD1,0)`, rows: 2, cols: 1},
		// XLOOKUP: scalar lookup miss with array if_not_found
		{formula: `XLOOKUP("delta",FA1:FA3,FC1:FC3,FC1:FC2,0)`, rows: 2, cols: 1},
		// XLOOKUP: vector lookup with compatible array if_not_found drives 2×1 spill
		{formula: `XLOOKUP(FP1:FP2,FA1:FA3,FC1:FC3,FC1:FC2,0)`, rows: 2, cols: 1},
		// XLOOKUP: 2D per-match return + partial miss + compatible horizontal if_not_found → 2×2 spill
		{formula: `XLOOKUP(FP1:FP2,FA1:FA3,FC1:FD3,FD1:FE1,0)`, rows: 2, cols: 2},
		// XMATCH: horizontal vector lookup with compatible horizontal match_mode → 1×2 spill
		{formula: `XMATCH(LA1:LB1,FE1:FE4,FK1:FL1)`, rows: 1, cols: 2},
		// XLOOKUP: horizontal vector lookup with compatible vertical if_not_found drives 3×2 spill
		{formula: `XLOOKUP(OA1:OB1,OD1:OF1,OD1:OF3,OC1:OC3,0)`, rows: 3, cols: 2},
		// MATCH: vector lookup with compatible per-element match_type drives 2×1 spill
		{formula: `MATCH(FR1:FR2,FE1:FE4,FB1:FB2)`, rows: 2, cols: 1},
		// MATCH: vector lookup dominates incompatible horizontal match_type, clamps to single column
		{formula: `MATCH(FR1:FR2,FE1:FE4,FK1:FL1)`, rows: 2, cols: 1},
		// VLOOKUP: vector lookup with compatible horizontal range_lookup spills horizontally
		{formula: `VLOOKUP(OU1:OV1,FH1:FI3,2,IA1:IB1)`, rows: 1, cols: 2},
		// HLOOKUP: vector lookup with compatible horizontal range_lookup spills horizontally
		{formula: `HLOOKUP(PA1:PB1,W1:Y3,2,IA1:IB1)`, rows: 1, cols: 2},
		// VLOOKUP: vector lookup with vertical range_lookup and horizontal selectors spills 2×2
		{formula: `VLOOKUP(PH1:PH2,EG1:EI3,EJ1:EK1,FJ1:FJ2)`, rows: 2, cols: 2},
		// HLOOKUP: horizontal lookup with vertical selector and vertical range_lookup spills 2×2
		{formula: `HLOOKUP(PI1:PJ1,EG1:EI3,EP1:EP2,FJ1:FJ2)`, rows: 2, cols: 2},
		// LOOKUP: horizontal vector approximate match over row-form 2D source spills horizontally
		{formula: `LOOKUP(PS1:PT1,LO2:LQ3)`, rows: 1, cols: 2},
		// LOOKUP: vertical vector approximate match over column-form 2D source spills vertically
		{formula: `LOOKUP(PG1:PG2,LM1:LN3)`, rows: 2, cols: 1},
		// LOOKUP: horizontal mixed-type vector over row-form 2D source spills horizontally
		{formula: `LOOKUP(RX1:RY1,RU1:RW2)`, rows: 1, cols: 2},
		// LOOKUP: vertical mixed-type vector over column-form 2D source spills vertically
		{formula: `LOOKUP(TD1:TD2,TB1:TC3)`, rows: 2, cols: 1},
		// VLOOKUP: vector approximate lookup over descending first-column table spills vertically
		{formula: `VLOOKUP(UC1:UC2,UA1:UB3,2,TRUE)`, rows: 2, cols: 1},
		// HLOOKUP: vector approximate lookup over descending first-row table spills horizontally
		{formula: `HLOOKUP(VH1:VI1,VE1:VG2,2,TRUE)`, rows: 1, cols: 2},
		// VLOOKUP: approximate wildcard-literal vector lookup over text-key table spills horizontally
		{formula: `VLOOKUP(XU1:XV1,XW1:XX3,2,TRUE)`, rows: 1, cols: 2},
		// HLOOKUP: approximate wildcard-literal lookup with vertical selector array spills vertically
		{formula: `HLOOKUP("Q2*",XY1:YA3,AC1:AC2,TRUE)`, rows: 2, cols: 1},
		// VLOOKUP: scalar lookup with vector range_lookup
		{formula: `VLOOKUP(4,FH1:FI3,2,FJ1:FJ2)`, rows: 2, cols: 1},
		// VLOOKUP: scalar lookup with horizontal range_lookup
		{formula: `VLOOKUP(4,FH1:FI3,2,IA1:IB1)`, rows: 1, cols: 2},
		// HLOOKUP: scalar lookup with horizontal range_lookup
		{formula: `HLOOKUP(2,W1:Y3,2,IA1:IB1)`, rows: 1, cols: 2},
	} {
		expr, err := Parse(tt.formula)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", tt.formula, err)
		}
		shape, ok := inferExpressionArrayShape(expr)
		if !ok {
			t.Fatalf("inferExpressionArrayShape(%q) returned not-ok", tt.formula)
		}
		if shape.rows != tt.rows || shape.cols != tt.cols {
			t.Fatalf("inferExpressionArrayShape(%q) = (%d,%d), want (%d,%d)", tt.formula, shape.rows, shape.cols, tt.rows, tt.cols)
		}
	}
}

func TestEngineInfersTextsplitSpills(t *testing.T) {
	// 1×3: cell reference whose text is split by comma
	t.Run("cell_ref_1x3", func(t *testing.T) {
		engine := NewEngine()
		engine.SetCell("A1", Text("one,two,three"))
		engine.SetFormula("B1", `TEXTSPLIT(A1,",")`)
		for ref, want := range map[string]string{
			"B1": "one", "C1": "two", "D1": "three",
		} {
			if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
				t.Fatalf("%s = %#v, want text %q", ref, got, want)
			}
		}
	})
	// 2×2: literal string split by both col-delim and row-delim
	t.Run("literal_2x2", func(t *testing.T) {
		engine := NewEngine()
		engine.SetFormula("A1", `TEXTSPLIT("a,b|c,d",",","|")`)
		for ref, want := range map[string]string{
			"A1": "a", "B1": "b",
			"A2": "c", "B2": "d",
		} {
			if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
				t.Fatalf("%s = %#v, want text %q", ref, got, want)
			}
		}
	})
	// 1×4: literal string, multiple column delimiters
	t.Run("literal_1x4", func(t *testing.T) {
		engine := NewEngine()
		engine.SetFormula("A1", `TEXTSPLIT("w-x-y-z","-")`)
		for ref, want := range map[string]string{
			"A1": "w", "B1": "x", "C1": "y", "D1": "z",
		} {
			if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
				t.Fatalf("%s = %#v, want text %q", ref, got, want)
			}
		}
	})
}

func TestEngineInfersLetSpills(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	// LET assigns a name to A1:A3 and returns it — spills 3×1
	engine.SetFormula("B1", `LET(x,A1:A3,x)`)
	// LET with computation: multiply each element by 10 — spills 3×1
	engine.SetFormula("C1", `LET(r,A1:A3,r*10)`)

	for ref, want := range map[string]float64{
		"B1": 1, "B2": 2, "B3": 3,
		"C1": 10, "C2": 20, "C3": 30,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineInfersScalarFunctionSpills(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(-1.2))
	engine.SetCell("A2", Number(2.4))
	engine.SetCell("A3", Number(-3.6))
	engine.SetFormula("B1", `ABS(A1:A3)`)
	engine.SetFormula("C1", `ROUND(A1:A3,0)`)

	for ref, want := range map[string]float64{
		"B1": 1.2,
		"B2": 2.4,
		"B3": 3.6,
		"C1": -1,
		"C2": 2,
		"C3": -4,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineInfersSheetQualifiedSpillRanges(t *testing.T) {
	values := map[string]Value{
		"SHEET2!A1":    Number(10),
		"SHEET2!A2":    Number(20),
		"SHEET2!A3":    Number(30),
		"[1]SHEET2!A1": Number(100),
		"[1]SHEET2!A2": Number(200),
		"[1]SHEET2!A3": Number(300),
	}
	getCell := func(ref string) Value {
		if v, ok := values[normalizeCellRef(ref)]; ok {
			return v
		}
		return Empty()
	}
	engine := NewEngine()
	engine.SetExternalResolver(
		getCell,
		func(start, end string) []Value {
			resolver := NewEngine()
			resolver.SetExternalResolver(getCell, nil)
			return resolver.GetRange(start, end)
		},
	)
	engine.SetFormula("D1", `Sheet2!A1:A3`)

	for ref, want := range map[string]float64{"D1": 10, "D2": 20, "D3": 30} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}

	engine.SetFormula("F1", `[1]Sheet2!A1:A3`)
	for ref, want := range map[string]float64{"F1": 100, "F2": 200, "F3": 300} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineInfersTransposeSpills(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(10))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B2", Number(20))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B3", Number(30))
	engine.SetFormula("D1", `TRANSPOSE(A1:B3)`)

	for ref, want := range map[string]float64{
		"D1": 1,
		"E1": 2,
		"F1": 3,
		"D2": 10,
		"E2": 20,
		"F2": 30,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesIndexSelectorsAndBroadcasts2D(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(10))
	engine.SetCell("C1", Number(100))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B2", Number(20))
	engine.SetCell("C2", Number(200))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C3", Number(300))
	engine.SetCell("D1", Number(1000))
	engine.SetCell("D2", Number(2000))
	engine.SetCell("D3", Number(3000))
	engine.SetFormula("E1", `INDEX(A1:C3,2,3)`)
	engine.SetFormula("F1", `INDEX(A1:C3,0,2)`)
	engine.SetFormula("H1", `INDEX(A1:C3,2,0)+D1:D3`)

	if got := engine.GetCell("E1"); !got.IsNumber() || got.Num != 200 {
		t.Fatalf("E1 = %#v, want numeric 200", got)
	}
	for ref, want := range map[string]float64{"F1": 10, "F2": 20, "F3": 30} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{
		"H1": 1002,
		"I1": 1020,
		"J1": 1200,
		"H2": 2002,
		"I2": 2020,
		"J2": 2200,
		"H3": 3002,
		"I3": 3020,
		"J3": 3200,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineComposesArraySelectorsInsideAggregatesAndOffsetSpills(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(10))
	engine.SetCell("C1", Number(100))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B2", Number(20))
	engine.SetCell("C2", Number(200))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C3", Number(300))
	engine.SetFormula("K1", `SUM(INDEX(A1:C3,2,0))`)
	engine.SetFormula("K2", `SUM(TRANSPOSE(A1:B3))`)
	engine.SetFormula("L1", `OFFSET(A1,1,0,2,2)`)
	engine.SetFormula("N1", `SUM(OFFSET(A1,1,0,2,2))`)

	if got := engine.GetCell("K1"); !got.IsNumber() || got.Num != 222 {
		t.Fatalf("K1 = %#v, want numeric 222", got)
	}
	if got := engine.GetCell("K2"); !got.IsNumber() || got.Num != 66 {
		t.Fatalf("K2 = %#v, want numeric 66", got)
	}
	for ref, want := range map[string]float64{"L1": 2, "M1": 20, "L2": 3, "M2": 30} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("N1"); !got.IsNumber() || got.Num != 55 {
		t.Fatalf("N1 = %#v, want numeric 55", got)
	}
}

func TestEngineEvaluatesChooseSelectorsAndRangeAwareComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C1", Text("Hello"))
	engine.SetCell("C2", Text("World"))
	engine.SetCell("D1", Text("One"))
	engine.SetCell("D2", Text("Two"))
	engine.SetCell("E1", Bool(true))
	engine.SetCell("E2", Bool(true))
	engine.SetCell("F1", Bool(false))
	engine.SetCell("F2", Bool(false))
	engine.SetCell("I1", Number(1))
	engine.SetCell("I2", Number(2))
	engine.SetCell("I3", Number(3))
	engine.SetFormula("G1", `CHOOSE(2,A1:A3,B1:B3)`)
	engine.SetFormula("H1", `COUNTIF(CHOOSE(1,A1:A3,B1:B3),">1")`)
	engine.SetFormula("H2", `CONCATENATE(CHOOSE(2,C1:C2,D1:D2))`)
	engine.SetFormula("H3", `AND(CHOOSE(1,E1:E2,F1:F2))`)
	engine.SetFormula("J1", `CHOOSE(I1:I3,"one","two","three")`)
	engine.SetFormula("K1", `UPPER(CHOOSE(I1:I3,"aa","bb","cc"))`)

	for ref, want := range map[string]float64{"G1": 10, "G2": 20, "G3": 30} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("H1"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("H1 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("H2"); !got.IsText() || got.Text != "OneTwo" {
		t.Fatalf("H2 = %#v, want text OneTwo", got)
	}
	if got := engine.GetCell("H3"); !got.IsBool() || !got.Bool {
		t.Fatalf("H3 = %#v, want TRUE", got)
	}
	for ref, want := range map[string]string{"J1": "one", "J2": "two", "J3": "three", "K1": "AA", "K2": "BB", "K3": "CC"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
}

func TestEngineEvaluatesChooseAxisSelectorsAndScalarComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(10))
	engine.SetCell("C1", Number(100))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B2", Number(20))
	engine.SetCell("C2", Number(200))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C3", Number(300))
	engine.SetCell("D1", Text("one"))
	engine.SetCell("E1", Text("two"))
	engine.SetCell("D2", Text("three"))
	engine.SetCell("E2", Text("four"))
	engine.SetCell("L1", Number(3))
	engine.SetCell("L2", Number(1))
	engine.SetCell("M1", Number(2))
	engine.SetCell("M2", Number(1))
	engine.SetFormula("F1", `CHOOSECOLS(A1:C3,3,1)`)
	engine.SetFormula("I1", `CHOOSEROWS(D1:E2,2)`)
	engine.SetFormula("I3", `UPPER(CHOOSEROWS(D1:E2,2))`)
	engine.SetFormula("K1", `LEN(CHOOSECOLS(D1:E2,2))`)
	engine.SetFormula("L4", `CHOOSECOLS(A1:C3,L1:L2)`)
	engine.SetFormula("N4", `CHOOSEROWS(A1:C3,M1:M2)`)

	for ref, want := range map[string]float64{
		"F1": 100,
		"G1": 1,
		"F2": 200,
		"G2": 2,
		"F3": 300,
		"G3": 3,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]string{"I1": "three", "J1": "four", "I3": "THREE", "J3": "FOUR"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"K1": 3, "K2": 4} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{
		"L4": 100,
		"M4": 1,
		"L5": 200,
		"M5": 2,
		"L6": 300,
		"M6": 3,
		"N4": 2,
		"O4": 20,
		"P4": 200,
		"N5": 1,
		"O5": 10,
		"P5": 100,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesTakeDropSelectorsAndNumericTextComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1.2))
	engine.SetCell("B1", Number(10.4))
	engine.SetCell("C1", Number(100.6))
	engine.SetCell("A2", Number(2.3))
	engine.SetCell("B2", Number(20.5))
	engine.SetCell("C2", Number(200.7))
	engine.SetCell("A3", Number(3.4))
	engine.SetCell("B3", Number(30.6))
	engine.SetCell("C3", Number(300.8))
	engine.SetFormula("D1", `TAKE(A1:C3,2,2)`)
	engine.SetFormula("G1", `DROP(A1:C3,1,1)`)
	engine.SetFormula("J1", `ROUND(TAKE(A1:C3,2,2),0)`)
	engine.SetFormula("L1", `MOD(DROP(A1:C3,1,1),10)`)
	engine.SetFormula("N1", `TEXT(TAKE(A1:C3,2,1),"0.0")`)

	for ref, want := range map[string]float64{"D1": 1.2, "E1": 10.4, "D2": 2.3, "E2": 20.5} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"G1": 20.5, "H1": 200.7, "G2": 30.6, "H2": 300.8} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"J1": 1, "K1": 10, "J2": 2, "K2": 21} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"L1": 0.5, "M1": 0.6999999999999886, "L2": 0.6000000000000014, "M2": 0.8000000000000114} {
		if got := engine.GetCell(ref); !got.IsNumber() || math.Abs(got.Num-want) > 1e-9 {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]string{"N1": "1.2", "N2": "2.3"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
}

func TestEngineEvaluatesToRowToColAndComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(10))
	engine.SetCell("C1", Number(100))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B2", Number(20))
	engine.SetCell("C2", Number(200))
	engine.SetCell("A4", Text("one"))
	engine.SetCell("B4", Text("two"))
	engine.SetCell("A5", Text("three"))
	engine.SetCell("B5", Text("four"))
	engine.SetFormula("D1", `TOROW(A1:C2)`)
	engine.SetFormula("D3", `TOCOL(A1:C2,0,1)`)
	engine.SetFormula("F3", `SUM(TOROW(A1:C2))`)
	engine.SetFormula("G4", `UPPER(TOROW(A4:B5))`)
	engine.SetFormula("G6", `LEN(TOCOL(A4:B5,0,1))`)

	for ref, want := range map[string]float64{"D1": 1, "E1": 10, "F1": 100, "G1": 2, "H1": 20, "I1": 200} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"D3": 1, "D4": 2, "D5": 10, "D6": 20, "D7": 100, "D8": 200} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("F3"); !got.IsNumber() || got.Num != 333 {
		t.Fatalf("F3 = %#v, want numeric 333", got)
	}
	for ref, want := range map[string]string{"G4": "ONE", "H4": "TWO", "I4": "THREE", "J4": "FOUR"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"G6": 3, "G7": 5, "G8": 3, "G9": 4} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesToVectorIgnoreModesAndWrapFunctions(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("C1", Number(10))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B2", Error(ErrDiv0))
	engine.SetCell("C2", Number(20))
	engine.SetFormula("D1", `TOROW(A1:C2,3)`)
	engine.SetFormula("D3", `TOCOL(A1:C2,3,1)`)
	engine.SetFormula("F3", `SUM(WRAPROWS(TOROW(A1:C2,3),3,0))`)
	engine.SetFormula("F5", `WRAPROWS(TOROW(A1:C2,3),3,"pad")`)
	engine.SetFormula("J1", `WRAPCOLS(TOCOL(A1:C2,3,1),3,0)`)

	for ref, want := range map[string]float64{"D1": 1, "E1": 10, "F1": 2, "G1": 20} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"D3": 1, "D4": 2, "D5": 10, "D6": 20} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("F3"); !got.IsNumber() || got.Num != 33 {
		t.Fatalf("F3 = %#v, want numeric 33", got)
	}
	for ref, want := range map[string]string{"F5": "1", "G5": "10", "H5": "2", "F6": "20", "G6": "pad", "H6": "pad"} {
		if got := engine.GetCell(ref).ToText(); got != want {
			t.Fatalf("%s = %q, want %q", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"J1": 1, "J2": 2, "J3": 10, "K1": 20, "K2": 0, "K3": 0} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesFilterRowsColsAndIfEmpty(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(10))
	engine.SetCell("C1", Number(100))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B2", Number(20))
	engine.SetCell("C2", Number(200))
	engine.SetCell("A3", Number(3))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C3", Number(300))
	engine.SetCell("D1", Bool(false))
	engine.SetCell("D2", Bool(true))
	engine.SetCell("D3", Bool(true))
	engine.SetCell("A5", Bool(true))
	engine.SetCell("B5", Bool(false))
	engine.SetCell("C5", Bool(true))
	engine.SetCell("N1", Text("one"))
	engine.SetCell("N2", Text("two"))
	engine.SetCell("N3", Text("three"))
	engine.SetFormula("F1", `FILTER(A1:C3,D1:D3)`)
	engine.SetFormula("J1", `FILTER(A1:C3,A5:C5)`)
	engine.SetFormula("M1", `SUM(FILTER(A1:C3,D1:D3))`)
	engine.SetFormula("O1", `UPPER(FILTER(N1:N3,D1:D3,"none"))`)
	engine.SetFormula("Q1", `FILTER(A1:C3,D1:D3*0,"none")`)
	engine.SetFormula("Q2", `FILTER(A1:C3,D1:D3*0)`)

	for ref, want := range map[string]float64{"F1": 2, "G1": 20, "H1": 200, "F2": 3, "G2": 30, "H2": 300} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"J1": 1, "K1": 100, "J2": 2, "K2": 200, "J3": 3, "K3": 300} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("M1"); !got.IsNumber() || got.Num != 555 {
		t.Fatalf("M1 = %#v, want numeric 555", got)
	}
	for ref, want := range map[string]string{"O1": "TWO", "O2": "THREE"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
	if got := engine.GetCell("Q1"); !got.IsText() || got.Text != "none" {
		t.Fatalf("Q1 = %#v, want text none", got)
	}
	if got := engine.GetCell("Q2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("Q2 = %#v, want #N/A", got)
	}
}

func TestEngineEvaluatesUniqueRowsColsAndExactlyOnce(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Text("alpha"))
	engine.SetCell("A2", Number(1))
	engine.SetCell("B2", Text("alpha"))
	engine.SetCell("A3", Number(2))
	engine.SetCell("B3", Text("beta"))
	engine.SetCell("A4", Number(3))
	engine.SetCell("B4", Text("beta"))
	engine.SetCell("A5", Number(3))
	engine.SetCell("B5", Text("beta"))
	engine.SetCell("D1", Number(1))
	engine.SetCell("E1", Number(1))
	engine.SetCell("F1", Number(3))
	engine.SetCell("D2", Number(2))
	engine.SetCell("E2", Number(2))
	engine.SetCell("F2", Number(4))
	engine.SetCell("H1", Text("one"))
	engine.SetCell("H2", Text("one"))
	engine.SetCell("H3", Text("two"))
	engine.SetCell("H4", Text("three"))
	engine.SetCell("H5", Text("three"))
	engine.SetCell("H6", Text("four"))
	engine.SetCell("J1", Number(1))
	engine.SetCell("J2", Number(1))
	engine.SetCell("J3", Number(2))
	engine.SetCell("J4", Number(3))
	engine.SetCell("J5", Number(3))
	engine.SetFormula("L1", `UNIQUE(A1:B5)`)
	engine.SetFormula("O1", `UNIQUE(D1:F2,1)`)
	engine.SetFormula("R1", `UPPER(UNIQUE(H1:H6,0,1))`)
	engine.SetFormula("S1", `SUM(UNIQUE(J1:J5,0,1))`)

	for ref, want := range map[string]Value{
		"L1": Number(1), "M1": Text("alpha"),
		"L2": Number(2), "M2": Text("beta"),
		"L3": Number(3), "M3": Text("beta"),
	} {
		got := engine.GetCell(ref)
		if !valuesEqual(got, want) {
			t.Fatalf("%s = %#v, want %#v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"O1": 1, "P1": 3, "O2": 2, "P2": 4} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]string{"R1": "TWO", "R2": "FOUR"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
	if got := engine.GetCell("S1"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("S1 = %#v, want numeric 2", got)
	}
}

func TestEngineEvaluatesSortRowsColsVectorsAndComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(3))
	engine.SetCell("B1", Text("c"))
	engine.SetCell("A2", Number(1))
	engine.SetCell("B2", Text("a"))
	engine.SetCell("A3", Number(2))
	engine.SetCell("B3", Text("b"))
	engine.SetCell("A4", Number(1))
	engine.SetCell("B4", Text("d"))
	engine.SetCell("L1", Number(4))
	engine.SetCell("M1", Number(1))
	engine.SetCell("N1", Number(3))
	engine.SetCell("O1", Number(2))
	engine.SetCell("L3", Number(1))
	engine.SetCell("M3", Number(3))
	engine.SetCell("N3", Number(2))
	engine.SetCell("L4", Number(2))
	engine.SetCell("M4", Number(1))
	engine.SetCell("N4", Number(4))
	engine.SetCell("T1", Text("one"))
	engine.SetCell("T2", Text("three"))
	engine.SetCell("T3", Text("two"))
	engine.SetCell("T4", Text("four"))
	engine.SetCell("V1", Number(3))
	engine.SetCell("V2", Number(1))
	engine.SetCell("V3", Number(4))
	engine.SetCell("V4", Number(2))
	engine.SetFormula("D1", `SORT(A1:B4)`)
	engine.SetFormula("G1", `SORT(A1:B4,2,-1)`)
	engine.SetFormula("P1", `SORT(L1:O1)`)
	engine.SetFormula("P3", `SORT(L3:N4,2,-1,1)`)
	engine.SetFormula("U1", `UPPER(SORT(T1:T4,1,-1))`)
	engine.SetFormula("W1", `SUM(SORT(V1:V4))`)

	for ref, want := range map[string]Value{
		"D1": Number(1), "E1": Text("a"),
		"D2": Number(1), "E2": Text("d"),
		"D3": Number(2), "E3": Text("b"),
		"D4": Number(3), "E4": Text("c"),
		"G1": Number(1), "H1": Text("d"),
		"G2": Number(3), "H2": Text("c"),
		"G3": Number(2), "H3": Text("b"),
		"G4": Number(1), "H4": Text("a"),
	} {
		got := engine.GetCell(ref)
		if !valuesEqual(got, want) {
			t.Fatalf("%s = %#v, want %#v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"P1": 1, "Q1": 2, "R1": 3, "S1": 4, "P3": 2, "Q3": 1, "R3": 3, "P4": 4, "Q4": 2, "R4": 1} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]string{"U1": "TWO", "U2": "THREE", "U3": "ONE", "U4": "FOUR"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
	if got := engine.GetCell("W1"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("W1 = %#v, want numeric 10", got)
	}
}

func TestEngineEvaluatesSortByRowsColsAndComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("B1", Text("b"))
	engine.SetCell("C1", Number(2))
	engine.SetCell("D1", Number(2))
	engine.SetCell("A2", Number(30))
	engine.SetCell("B2", Text("a"))
	engine.SetCell("C2", Number(1))
	engine.SetCell("D2", Number(2))
	engine.SetCell("A3", Number(20))
	engine.SetCell("B3", Text("d"))
	engine.SetCell("C3", Number(2))
	engine.SetCell("D3", Number(1))
	engine.SetCell("A4", Number(40))
	engine.SetCell("B4", Text("c"))
	engine.SetCell("C4", Number(1))
	engine.SetCell("D4", Number(1))
	engine.SetCell("F1", Number(40))
	engine.SetCell("G1", Number(10))
	engine.SetCell("H1", Number(30))
	engine.SetCell("I1", Number(20))
	engine.SetCell("F2", Text("d"))
	engine.SetCell("G2", Text("a"))
	engine.SetCell("H2", Text("c"))
	engine.SetCell("I2", Text("b"))
	engine.SetCell("F3", Number(2))
	engine.SetCell("G3", Number(1))
	engine.SetCell("H3", Number(2))
	engine.SetCell("I3", Number(1))
	engine.SetCell("F4", Number(1))
	engine.SetCell("G4", Number(2))
	engine.SetCell("H4", Number(2))
	engine.SetCell("I4", Number(1))
	engine.SetCell("K1", Text("one"))
	engine.SetCell("K2", Text("two"))
	engine.SetCell("K3", Text("three"))
	engine.SetCell("K4", Text("four"))
	engine.SetCell("L1", Number(2))
	engine.SetCell("L2", Number(4))
	engine.SetCell("L3", Number(3))
	engine.SetCell("L4", Number(1))
	engine.SetCell("N1", Number(3))
	engine.SetCell("N2", Number(1))
	engine.SetCell("N3", Number(4))
	engine.SetCell("N4", Number(2))
	engine.SetFormula("P1", `SORTBY(A1:B4,C1:C4,1,D1:D4,-1)`)
	engine.SetFormula("S1", `SORTBY(F1:I2,F3:I3,1,F4:I4,-1)`)
	engine.SetFormula("W1", `UPPER(SORTBY(K1:K4,L1:L4,-1))`)
	engine.SetFormula("X1", `SUM(SORTBY(N1:N4,L1:L4,-1))`)

	for ref, want := range map[string]Value{
		"P1": Number(30), "Q1": Text("a"),
		"P2": Number(40), "Q2": Text("c"),
		"P3": Number(10), "Q3": Text("b"),
		"P4": Number(20), "Q4": Text("d"),
		"S1": Number(10), "T1": Number(20), "U1": Number(30), "V1": Number(40),
		"S2": Text("a"), "T2": Text("b"), "U2": Text("c"), "V2": Text("d"),
	} {
		got := engine.GetCell(ref)
		if !valuesEqual(got, want) {
			t.Fatalf("%s = %#v, want %#v", ref, got, want)
		}
	}
	for ref, want := range map[string]string{"W1": "TWO", "W2": "THREE", "W3": "ONE", "W4": "FOUR"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
	if got := engine.GetCell("X1"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("X1 = %#v, want numeric 10", got)
	}
}

func TestEngineEvaluatesSequenceAndComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(2))
	engine.SetCell("B1", Number(3))
	engine.SetCell("A2", Number(3))
	engine.SetFormula("D1", `SEQUENCE(A1,B1)`)
	engine.SetFormula("H1", `SEQUENCE(1,4,10,-2)`)
	engine.SetFormula("M1", `SUM(SEQUENCE(3,1,1,2))`)
	engine.SetFormula("O1", `TEXT(SEQUENCE(A2,1,1,0.5),"0.0")`)

	for ref, want := range map[string]float64{"D1": 1, "E1": 2, "F1": 3, "D2": 4, "E2": 5, "F2": 6} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"H1": 10, "I1": 8, "J1": 6, "K1": 4} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("M1"); !got.IsNumber() || got.Num != 9 {
		t.Fatalf("M1 = %#v, want numeric 9", got)
	}
	for ref, want := range map[string]string{"O1": "1", "O2": "1.5", "O3": "2"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %s", ref, got, want)
		}
	}
}

func TestEngineEvaluatesRandArrayAndComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(2))
	engine.SetCell("B1", Number(3))
	engine.SetFormula("D1", `RANDARRAY(A1,B1)`)
	engine.SetFormula("H1", `RANDARRAY(1,4,10,12,1)`)
	engine.SetFormula("M1", `COUNT(RANDARRAY(3,1,1,2,1))`)
	engine.SetFormula("O1", `ROUND(RANDARRAY(A1,1,1,9),0)`)

	for _, ref := range []string{"D1", "E1", "F1", "D2", "E2", "F2"} {
		got := engine.GetCell(ref)
		if !got.IsNumber() || got.Num < 0 || got.Num > 1 {
			t.Fatalf("%s = %#v, want numeric value in [0,1]", ref, got)
		}
	}
	for _, ref := range []string{"H1", "I1", "J1", "K1"} {
		got := engine.GetCell(ref)
		if !got.IsNumber() || got.Num < 10 || got.Num > 12 || got.Num != math.Trunc(got.Num) {
			t.Fatalf("%s = %#v, want whole number in [10,12]", ref, got)
		}
	}
	if got := engine.GetCell("M1"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("M1 = %#v, want numeric 3", got)
	}
	for _, ref := range []string{"O1", "O2"} {
		got := engine.GetCell(ref)
		if !got.IsNumber() || got.Num < 1 || got.Num > 9 || got.Num != math.Trunc(got.Num) {
			t.Fatalf("%s = %#v, want rounded numeric value in [1,9]", ref, got)
		}
	}
}

func TestEngineEvaluatesXMatchModesAndComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("alpha"))
	engine.SetCell("A2", Text("beta"))
	engine.SetCell("A3", Text("gamma"))
	engine.SetCell("B1", Number(1))
	engine.SetCell("B2", Number(3))
	engine.SetCell("B3", Number(5))
	engine.SetFormula("D1", `XMATCH("beta",A1:A3)`)
	engine.SetFormula("D2", `XMATCH("beta",A1:A3,0,-1)`)
	engine.SetFormula("E1", `XMATCH(4,B1:B3,-1)`)
	engine.SetFormula("E2", `XMATCH(4,B1:B3,1)`)
	engine.SetFormula("F1", `ABS(XMATCH(3,B1:B3)-1)`)
	engine.SetFormula("F2", `XMATCH("delta",A1:A3)`)
	engine.SetFormula("G1", `XMATCH(A1:A3,A1:A3)`)

	for ref, want := range map[string]float64{"D1": 2, "D2": 2, "E1": 2, "E2": 3, "F1": 1, "G1": 1, "G2": 2, "G3": 3} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("F2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("F2 = %#v, want #N/A", got)
	}
}

func TestEngineEvaluatesXLookupModesAndComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("alpha"))
	engine.SetCell("A2", Text("beta"))
	engine.SetCell("A3", Text("gamma"))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))
	engine.SetCell("B3", Number(30))
	engine.SetCell("C1", Text("x"))
	engine.SetCell("D1", Text("y"))
	engine.SetCell("E1", Text("z"))
	engine.SetCell("C2", Number(3))
	engine.SetCell("D2", Number(5))
	engine.SetCell("E2", Number(7))
	engine.SetCell("J1", Number(100))
	engine.SetCell("K1", Number(1000))
	engine.SetCell("J2", Number(200))
	engine.SetCell("K2", Number(2000))
	engine.SetCell("J3", Number(300))
	engine.SetCell("K3", Number(3000))
	engine.SetCell("L1", Text("q1"))
	engine.SetCell("M1", Text("q2"))
	engine.SetCell("N1", Text("q3"))
	engine.SetCell("L2", Number(11))
	engine.SetCell("M2", Number(22))
	engine.SetCell("N2", Number(33))
	engine.SetCell("L3", Number(111))
	engine.SetCell("M3", Number(222))
	engine.SetCell("N3", Number(333))
	engine.SetCell("LO1", Text("q1"))
	engine.SetCell("LP1", Text("q2"))
	engine.SetCell("LQ1", Text("q3"))
	engine.SetCell("LO2", Number(11))
	engine.SetCell("LP2", Number(22))
	engine.SetCell("LQ2", Number(33))
	engine.SetCell("LO3", Number(111))
	engine.SetCell("LP3", Number(222))
	engine.SetCell("LQ3", Number(333))
	engine.SetCell("MO1", Text("q1"))
	engine.SetCell("MO2", Text("q2"))
	engine.SetCell("ME1", Number(1))
	engine.SetCell("MF1", Number(10))
	engine.SetCell("MG1", Number(100))
	engine.SetCell("ME2", Number(2))
	engine.SetCell("MF2", Number(20))
	engine.SetCell("MG2", Number(200))
	engine.SetCell("ME3", Number(3))
	engine.SetCell("MF3", Number(30))
	engine.SetCell("MG3", Number(300))
	engine.SetCell("MH1", Text("q1"))
	engine.SetCell("MI1", Text("q2"))
	engine.SetCell("MJ1", Text("q3"))
	engine.SetCell("MH2", Number(11))
	engine.SetCell("MI2", Number(22))
	engine.SetCell("MJ2", Number(33))
	engine.SetCell("MH3", Number(111))
	engine.SetCell("MI3", Number(222))
	engine.SetCell("MJ3", Number(333))
	engine.SetCell("OA1", Number(10))
	engine.SetCell("OB1", Number(50))
	engine.SetCell("OC1", Number(7))
	engine.SetCell("OC2", Number(8))
	engine.SetCell("OC3", Number(9))
	engine.SetCell("MK1", Number(2))
	engine.SetCell("MK2", Number(3))
	engine.SetCell("ML1", Number(2))
	engine.SetCell("MM1", Number(3))
	engine.SetCell("MN1", Number(1))
	engine.SetCell("MN2", Number(2))
	engine.SetCell("II1", Text("q1"))
	engine.SetCell("IJ1", Text("q2"))
	engine.SetCell("IK1", Number(100))
	engine.SetCell("IL1", Number(200))
	engine.SetFormula("G1", `XLOOKUP("beta",A1:A3,B1:B3)`)
	engine.SetFormula("G2", `XLOOKUP("delta",A1:A3,B1:B3,"missing")`)
	engine.SetFormula("H1", `XLOOKUP(4,C2:E2,C1:E1,"",-1)`)
	engine.SetFormula("H2", `XLOOKUP(4,C2:E2,C1:E1,"",1)`)
	engine.SetFormula("I1", `LEN(XLOOKUP("gamma",A1:A3,A1:A3))`)
	engine.SetFormula("I2", `XLOOKUP("beta",A1:A3,B1:B3,0,0,-1)`)
	engine.SetFormula("P1", `XLOOKUP("beta",A1:A3,J1:K3)`)
	engine.SetFormula("R1", `XLOOKUP("q2",L1:N1,L2:N3)`)
	engine.SetFormula("T1", `XLOOKUP(A1:A3,A1:A3,B1:B3)`)
	engine.SetFormula("U1", `XLOOKUP(L1:N1,L1:N1,L2:N2)`)
	engine.SetFormula("X1", `XLOOKUP(A1:A2,A1:A3,A1:B3)`)
	engine.SetFormula("AC1", `XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3)`)
	engine.SetFormula("AF1", `XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3,II1:IJ1,0)`)
	engine.SetFormula("AO1", `XLOOKUP(OA1:OB1,ME1:MG1,ME1:MG3,OC1:OC3,0)`)
	engine.SetFormula("Z1", `LOOKUP(II1:IJ1,L1:N3)`)
	engine.SetFormula("Z3", `LOOKUP(IK1:IL1,J1:K3)`)
	engine.SetFormula("AE1", `LOOKUP(MO1:MO2,LO1:LQ3)`)
	engine.SetFormula("AK1", `VLOOKUP(MN1:MN2,ME1:MG3,ML1:MM1,FALSE)`)
	engine.SetFormula("AM1", `HLOOKUP(LO1:LP1,MH1:MJ3,MK1:MK2,FALSE)`)

	if got := engine.GetCell("G1"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("G1 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("G2"); !got.IsText() || got.Text != "missing" {
		t.Fatalf("G2 = %#v, want text missing", got)
	}
	if got := engine.GetCell("H1"); !got.IsText() || got.Text != "x" {
		t.Fatalf("H1 = %#v, want text x", got)
	}
	if got := engine.GetCell("H2"); !got.IsText() || got.Text != "y" {
		t.Fatalf("H2 = %#v, want text y", got)
	}
	if got := engine.GetCell("I1"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("I1 = %#v, want numeric 5", got)
	}
	if got := engine.GetCell("I2"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("I2 = %#v, want numeric 20", got)
	}
	for ref, want := range map[string]float64{
		"P1": 200, "Q1": 2000,
		"R1": 22, "R2": 222,
		// XLOOKUP(A1:A3,A1:A3,B1:B3) → [10,20,30]
		"T1": 10, "T2": 20, "T3": 30,
		// XLOOKUP(L1:N1,L1:N1,L2:N2) → [11,22,33]
		"U1": 11, "V1": 22, "W1": 33,
		// LOOKUP(II1:IJ1,L1:N3) → [111,222]
		"Z1": 111, "AA1": 222,
		// LOOKUP(IK1:IL1,J1:K3) → [1000,2000]
		"Z3": 1000, "AA3": 2000,
		// LOOKUP(MO1:MO2,LO1:LQ3) → [111,222]
		"AE1": 111, "AE2": 222,
		// XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3) → [[q1,q2],[11,22],[111,222]]
		"AC2": 11, "AD2": 22, "AC3": 111, "AD3": 222,
		// XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3,II1:IJ1,0) found-case ignores incompatible if_not_found shape
		"AF2": 11, "AG2": 22, "AF3": 111, "AG3": 222,
		// XLOOKUP(OA1:OB1,ME1:MG1,ME1:MG3,OC1:OC3,0) → [[10,7],[20,8],[30,9]]
		"AO1": 10, "AP1": 7, "AO2": 20, "AP2": 8, "AO3": 30, "AP3": 9,
		// VLOOKUP(MN1:MN2,ME1:MG3,ML1:MM1,FALSE) → [[10,100],[20,200]]
		"AK1": 10, "AL1": 100, "AK2": 20, "AL2": 200,
		// HLOOKUP(LO1:LP1,MH1:MJ3,MK1:MK2,FALSE) → [[11,22],[111,222]]
		"AM1": 11, "AN1": 22, "AM2": 111, "AN2": 222,
	} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]string{
		// XLOOKUP(A1:A2,A1:A3,A1:B3) → [["alpha",10],["beta",20]]
		"X1": "alpha",
		"X2": "beta",
		// XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3) first return row remains text
		"AC1": "q1",
		"AD1": "q2",
		// XLOOKUP(LO1:LP1,LO1:LQ1,LO1:LQ3,II1:IJ1,0) first return row remains text
		"AF1": "q1",
		"AG1": "q2",
	} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %q", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"Y1": 10, "Y2": 20} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesMatchLookupWildcardAndBinaryModes(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("alpha"))
	engine.SetCell("A2", Text("beta"))
	engine.SetCell("A3", Text("beta-two"))
	engine.SetCell("A4", Text("gamma"))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))
	engine.SetCell("B3", Number(30))
	engine.SetCell("B4", Number(40))
	engine.SetCell("D1", Number(1))
	engine.SetCell("D2", Number(3))
	engine.SetCell("D3", Number(5))
	engine.SetCell("D4", Number(7))
	engine.SetCell("E1", Text("a"))
	engine.SetCell("E2", Text("b"))
	engine.SetCell("E3", Text("c"))
	engine.SetCell("E4", Text("d"))
	engine.SetCell("EA1", Text("Q1"))
	engine.SetCell("EB1", Text("Q2"))
	engine.SetCell("EC1", Text("Q3"))
	engine.SetCell("EA2", Number(100))
	engine.SetCell("EB2", Number(200))
	engine.SetCell("EC2", Number(300))
	engine.SetCell("U1", Number(2))
	engine.SetCell("V1", Number(3))
	engine.SetCell("W1", Number(1))
	engine.SetCell("X1", Number(10))
	engine.SetCell("Y1", Number(100))
	engine.SetCell("W2", Number(2))
	engine.SetCell("X2", Number(20))
	engine.SetCell("Y2", Number(200))
	engine.SetCell("W3", Number(3))
	engine.SetCell("X3", Number(30))
	engine.SetCell("Y3", Number(300))
	engine.SetCell("AC1", Number(2))
	engine.SetCell("AC2", Number(3))
	engine.SetCell("AD1", Text("Q1"))
	engine.SetCell("AE1", Text("Q2"))
	engine.SetCell("AF1", Text("Q3"))
	engine.SetCell("AD2", Number(100))
	engine.SetCell("AE2", Number(200))
	engine.SetCell("AF2", Number(300))
	engine.SetCell("AD3", Number(1000))
	engine.SetCell("AE3", Number(2000))
	engine.SetCell("AF3", Number(3000))
	engine.SetCell("BA1", Text("beta"))
	engine.SetCell("BA2", Text("beta"))
	engine.SetCell("BA3", Text("gamma"))
	engine.SetCell("BB1", Number(10))
	engine.SetCell("BB2", Number(20))
	engine.SetCell("BB3", Number(30))
	engine.SetCell("BC1", Number(1))
	engine.SetCell("BC2", Number(-1))
	engine.SetCell("BE1", Number(1))
	engine.SetCell("BE2", Number(3))
	engine.SetCell("BE3", Number(5))
	engine.SetCell("BE4", Number(7))
	engine.SetCell("BF1", Number(4))
	engine.SetCell("BF2", Number(4))
	engine.SetCell("BG1", Number(1))
	engine.SetCell("BG2", Number(-1))
	engine.SetCell("BH1", Number(1))
	engine.SetCell("BH2", Number(3))
	engine.SetCell("BH3", Number(5))
	engine.SetCell("BI1", Number(10))
	engine.SetCell("BI2", Number(30))
	engine.SetCell("BI3", Number(50))
	engine.SetCell("BJ1", Number(4))
	engine.SetCell("BJ2", Number(4))
	engine.SetCell("BK1", Bool(true))
	engine.SetCell("BK2", Bool(false))
	engine.SetCell("IA1", Bool(true))
	engine.SetCell("IB1", Bool(false))
	engine.SetCell("IC1", Number(2))
	engine.SetCell("ID1", Number(0))
	engine.SetCell("IE1", Number(1))
	engine.SetCell("IF1", Number(-1))
	engine.SetCell("IG1", Text("b*"))
	engine.SetCell("IH1", Text("b*"))
	engine.SetCell("CA1", Number(1))
	engine.SetCell("CB1", Number(-1))
	engine.SetCell("HA1", Number(2))
	engine.SetCell("HB1", Number(3))
	engine.SetCell("HH1", Number(0))
	engine.SetCell("HH2", Number(0))
	engine.SetCell("HJ1", Number(3))
	engine.SetCell("HK1", Number(5))
	engine.SetCell("M1", Number(7))
	engine.SetCell("M2", Number(5))
	engine.SetCell("M3", Number(3))
	engine.SetCell("M4", Number(1))
	engine.SetCell("N1", Text("d"))
	engine.SetCell("N2", Text("c"))
	engine.SetCell("N3", Text("b"))
	engine.SetCell("N4", Text("a"))
	engine.SetCell("HD1", Text("b*"))
	engine.SetCell("HE1", Text("g*"))

	engine.SetFormula("G1", `MATCH("beta",A1:A4,0)`)
	engine.SetFormula("G2", `MATCH(4,D1:D4,1)`)
	engine.SetFormula("G3", `MATCH("b*",A1:A4,0)`)
	engine.SetFormula("GE1", `MATCH(HA1:HB1,D1:D4,BC1:BC2)`)
	engine.SetFormula("H1", `LOOKUP(4,D1:D4,E1:E4)`)
	engine.SetFormula("H2", `LOOKUP("beta",A1:A4,B1:B4)`)
	engine.SetFormula("H3", `LOOKUP("b*",A1:A4,B1:B4)`)
	engine.SetFormula("H8", `LOOKUP(HA1:HB1,D1:D4,D1:D4)`)
	engine.SetFormula("H9", `LOOKUP(4,M1:M4,N1:N4)`)
	engine.SetFormula("H10", `LOOKUP(HD1:HE1,A1:A4,B1:B4)`)
	engine.SetFormula("H11", `VLOOKUP("b*",A1:B4,2,TRUE)`)
	engine.SetFormula("H12", `VLOOKUP(HD1:HE1,A1:B4,2,TRUE)`)
	engine.SetFormula("H13", `HLOOKUP("Q2*",AD1:AF3,2,TRUE)`)
	engine.SetFormula("H14", `HLOOKUP("Q2*",AD1:AF3,AC1:AC2,TRUE)`)
	engine.SetFormula("H16", `VLOOKUP("b*",A1:B4,2,IA1:IB1)`)
	engine.SetFormula("H17", `HLOOKUP("Q2*",AD1:AF3,2,IA1:IB1)`)
	// XLOOKUP scalar wildcard with per-element match_mode {2,0}: first uses wildcard scan, second uses exact match
	engine.SetFormula("H18", `XLOOKUP("b*",A1:A4,B1:B4,"",IC1:ID1)`)
	// XMATCH scalar wildcard with per-element match_mode {2,0}
	engine.SetFormula("H19", `XMATCH("b*",A1:A4,IC1:ID1)`)
	// XLOOKUP vector lookup_value with per-element match_mode: HD1="b*" (wildcard hit 20), HE1="g*" (exact miss → "")
	engine.SetFormula("H20", `XLOOKUP(HD1:HE1,A1:A4,B1:B4,"",IC1:ID1)`)
	// XMATCH vector lookup_value with per-element match_mode: "b*" wildcard → pos 2; "g*" exact → #N/A
	engine.SetFormula("H21", `XMATCH(HD1:HE1,A1:A4,IC1:ID1)`)
	// XLOOKUP scalar wildcard with per-element search_mode {1,-1}: forward → first "b*" (val 20), reverse → last "b*" (val 30)
	engine.SetFormula("H22", `XLOOKUP("b*",A1:A4,B1:B4,"",2,IE1:IF1)`)
	// XMATCH scalar wildcard with per-element search_mode {1,-1}: forward → pos 2, reverse → pos 3
	engine.SetFormula("H23", `XMATCH("b*",A1:A4,2,IE1:IF1)`)
	// XLOOKUP vector lookup_value + per-element search_mode: IG1="b*"+forward→20, IH1="b*"+reverse→30
	engine.SetFormula("H24", `XLOOKUP(IG1:IH1,A1:A4,B1:B4,"",2,IE1:IF1)`)
	// XMATCH vector lookup_value + per-element search_mode: "b*"+forward→pos2, "b*"+reverse→pos3
	engine.SetFormula("H25", `XMATCH(IG1:IH1,A1:A4,2,IE1:IF1)`)
	engine.SetFormula("I1", `XMATCH("b*",A1:A4,2)`)
	engine.SetFormula("I2", `XMATCH(4,D1:D4,-1,2)`)
	engine.SetFormula("I3", `XMATCH(4,M1:M4,1,-2)`)
	engine.SetFormula("H4", `LOOKUP(D1:D4,D1:D4,E1:E4)`)
	engine.SetFormula("K1", `MATCH(A1:A4,A1:A4,0)`)
	engine.SetFormula("L1", `VLOOKUP(A1:A4,A1:B4,2,FALSE)`)
	engine.SetFormula("S1", `VLOOKUP(2,W1:Y3,U1:V1,FALSE)`)
	engine.SetFormula("FZ1", `VLOOKUP(2,W1:Y3,U1:V1,BK1:BK2)`)
	engine.SetFormula("J1", `XLOOKUP("b*",A1:A4,B1:B4,"missing",2)`)
	engine.SetFormula("J2", `XLOOKUP(4,D1:D4,E1:E4,"",-1,2)`)
	engine.SetFormula("J3", `XLOOKUP(4,M1:M4,N1:N4,"",1,-2)`)
	engine.SetFormula("P1", `HLOOKUP(EA1:EC1,EA1:EC2,2,FALSE)`)
	engine.SetFormula("AB1", `HLOOKUP("Q2",AD1:AF3,AC1:AC2,FALSE)`)
	engine.SetFormula("GC1", `HLOOKUP("Q2",AD1:AF3,AC1:AC2,CA1:CB1)`)
	engine.SetFormula("AM1", `XMATCH(BA1:BA2,BA1:BA3,0,BC1:BC2)`)
	engine.SetFormula("GO1", `XMATCH(HJ1:HK1,D1:D4,HH1:HH2)`)
	engine.SetFormula("KM1", `XMATCH(HJ1:HK1,D1:D4,CA1:CB1,BC1:BC2)`)
	engine.SetFormula("GT1", `MATCH(4,BE1:BE4,BC1:BC2)`)
	engine.SetFormula("GU1", `MATCH(4,BE1:BE4,CA1:CB1)`)
	engine.SetFormula("AN1", `XLOOKUP(BA1:BA2,BA1:BA3,BB1:BB3,"",0,BC1:BC2)`)
	engine.SetFormula("GQ1", `XLOOKUP(HJ1:HK1,D1:D4,D1:D4,"",HH1:HH2)`)
	engine.SetFormula("KO1", `XLOOKUP(HJ1:HK1,D1:D4,D1:D4,"",CA1:CB1,BC1:BC2)`)
	engine.SetFormula("AP1", `VLOOKUP(BJ1:BJ2,BH1:BI3,2,BK1:BK2)`)
	engine.SetFormula("AR1", `XMATCH("beta",BA1:BA3,0,BC1:BC2)`)
	engine.SetFormula("GW1", `XMATCH("beta",BA1:BA3,0,CA1:CB1)`)
	engine.SetFormula("JA6", `XMATCH(4,BE1:BE4,CA1:CB1,BC1:BC2)`)
	engine.SetFormula("BS1", `XMATCH(4,BE1:BE4,BC1:BC2,CA1:CB1)`)
	engine.SetFormula("AT1", `XLOOKUP("beta",BA1:BA3,BB1:BB3,"",0,BC1:BC2)`)
	engine.SetFormula("GY1", `XLOOKUP(4,D1:D4,D1:D4,"",CA1:CB1,1)`)
	engine.SetFormula("KF1", `XLOOKUP(4,D1:D4,D1:D4,"",CA1:CB1,BC1:BC2)`)
	engine.SetFormula("BQ1", `XLOOKUP("beta",BA1:BA3,BB1:BB3,BC1:BC2,0,CA1:CB1)`)
	engine.SetFormula("BP1", `XLOOKUP("beta",BA1:BA3,BB1:BB3,"",BC1:BC2,CA1:CB1)`)
	engine.SetFormula("BN1", `XLOOKUP("beta",BA1:BA3,BB1:BB3,BB1:BB3,0,BC1:BC2)`)
	engine.SetFormula("BM1", `XLOOKUP(BA1:BA2,BA1:BA3,BB1:BB3,BB1:BC1,0)`)
	engine.SetFormula("AX1", `XLOOKUP("delta",BA1:BA3,BB1:BB3,BB1:BB2,0)`)
	engine.SetFormula("AV1", `VLOOKUP(4,BH1:BI3,2,BK1:BK2)`)
	engine.SetFormula("IY1", `VLOOKUP(4,BH1:BI3,2,IA1:IB1)`)
	engine.SetFormula("JA4", `HLOOKUP(2,W1:Y3,2,IA1:IB1)`)

	for ref, want := range map[string]float64{"G1": 2, "G2": 2, "G3": 2, "GE1": 1, "GF1": 2, "I1": 2, "I2": 2, "I3": 2, "K1": 1, "K2": 2, "K3": 3, "K4": 4} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("H1"); !got.IsText() || got.Text != "b" {
		t.Fatalf("H1 = %#v, want text b", got)
	}
	if got := engine.GetCell("H2"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("H2 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("H3"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("H3 = %#v, want numeric 10", got)
	}
	for ref, want := range map[string]string{"H4": "a", "H5": "b", "H6": "c", "H7": "d"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %q", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"H8": 1, "I8": 3} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"H10": 10, "I10": 30} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"H11": 10, "H12": 10, "I12": 30, "H13": 200, "H14": 200, "H15": 2000, "H16": 10, "I16": 20, "H17": 200, "I17": 200} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	// XLOOKUP wildcard per-element match_mode: match_mode=2 → wildcard hit 20, match_mode=0 → if_not_found ""
	if got := engine.GetCell("H18"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("H18 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("I18"); !got.IsText() || got.Text != "" {
		t.Fatalf("I18 = %#v, want empty text (if_not_found)", got)
	}
	// XMATCH wildcard per-element match_mode: match_mode=2 → position 2, match_mode=0 → #N/A
	if got := engine.GetCell("H19"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("H19 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("I19"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("I19 = %#v, want #N/A", got)
	}
	// XLOOKUP vector lookup_value + per-element match_mode: "b*" wildcard → 20, "g*" exact → if_not_found ""
	if got := engine.GetCell("H20"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("H20 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("I20"); !got.IsText() || got.Text != "" {
		t.Fatalf("I20 = %#v, want empty text (if_not_found)", got)
	}
	// XMATCH vector lookup_value + per-element match_mode: "b*" wildcard → position 2, "g*" exact → #N/A
	if got := engine.GetCell("H21"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("H21 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("I21"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("I21 = %#v, want #N/A", got)
	}
	// XLOOKUP scalar wildcard with per-element search_mode: forward→first match (20), reverse→last match (30)
	if got := engine.GetCell("H22"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("H22 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("I22"); !got.IsNumber() || got.Num != 30 {
		t.Fatalf("I22 = %#v, want numeric 30", got)
	}
	// XMATCH scalar wildcard with per-element search_mode: forward→pos 2, reverse→pos 3
	if got := engine.GetCell("H23"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("H23 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("I23"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("I23 = %#v, want numeric 3", got)
	}
	// XLOOKUP vector lookup_value + per-element search_mode: "b*"+forward→20, "b*"+reverse→30
	if got := engine.GetCell("H24"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("H24 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("I24"); !got.IsNumber() || got.Num != 30 {
		t.Fatalf("I24 = %#v, want numeric 30", got)
	}
	// XMATCH vector lookup_value + per-element search_mode: "b*"+forward→pos 2, "b*"+reverse→pos 3
	if got := engine.GetCell("H25"); !got.IsNumber() || got.Num != 2 {
		t.Fatalf("H25 = %#v, want numeric 2", got)
	}
	if got := engine.GetCell("I25"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("I25 = %#v, want numeric 3", got)
	}
	if got := engine.GetCell("H9"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("H9 = %#v, want #N/A", got)
	}
	for ref, want := range map[string]float64{"L1": 10, "L2": 20, "L3": 30, "L4": 40, "P1": 100, "Q1": 200, "R1": 300} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"S1": 20, "T1": 200, "AB1": 200, "AB2": 2000, "AM1": 1, "AM2": 2, "AN1": 10, "AN2": 20, "AP1": 30, "AR1": 1, "AR2": 2, "AT1": 10, "AT2": 20, "AV1": 30, "AX1": 10, "AX2": 20, "BM1": 10, "BM2": 10, "BN1": 10, "BN2": 20, "BP1": 10, "BP2": 10, "BQ1": 10, "BR1": 20, "BS1": 3, "BS2": 2, "FZ1": 20, "GA1": 200, "GC1": 200, "GC2": 2000, "GO1": 2, "GP1": 3, "GQ1": 3, "GR1": 5, "GT1": 2, "GU1": 2, "GW1": 1, "GX1": 2, "GY1": 5, "GZ1": 3, "IY1": 30, "JA4": 2, "JA6": 3, "JB6": 2, "KF1": 5, "KG1": 3, "KM1": 2, "KN1": 3, "KO1": 3, "KP1": 5} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("IZ1"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("IZ1 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("JB4"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("JB4 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("GT2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("GT2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("GV1"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("GV1 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("AP2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("AP2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("AV2"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("AV2 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("J1"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("J1 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("J2"); !got.IsText() || got.Text != "b" {
		t.Fatalf("J2 = %#v, want text b", got)
	}
	if got := engine.GetCell("J3"); !got.IsText() || got.Text != "c" {
		t.Fatalf("J3 = %#v, want text c", got)
	}
}

func TestEngineEvaluatesHStackAndVStack(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(2))
	engine.SetCell("A2", Number(3))
	engine.SetCell("B2", Number(4))
	engine.SetCell("D1", Number(10))
	engine.SetCell("D2", Number(20))
	engine.SetCell("D3", Number(30))
	engine.SetCell("F1", Text("x"))
	engine.SetCell("F2", Text("z"))

	engine.SetFormula("J1", `HSTACK(A1:B2,D1:D3)`)
	engine.SetFormula("J5", `VSTACK(A1:B2,F1:F2)`)

	for ref, want := range map[string]float64{"J1": 1, "K1": 2, "L1": 10, "J2": 3, "K2": 4, "L2": 20, "L3": 30} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("J3"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("J3 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("K3"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("K3 = %#v, want #N/A", got)
	}
	for ref, want := range map[string]float64{"J5": 1, "K5": 2, "J6": 3, "K6": 4} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]string{"J7": "x", "J8": "z"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %q", ref, got, want)
		}
	}
	if got := engine.GetCell("K7"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("K7 = %#v, want #N/A", got)
	}
	if got := engine.GetCell("K8"); !got.IsError() || got.Err != ErrNA {
		t.Fatalf("K8 = %#v, want #N/A", got)
	}
}

func TestEngineEvaluatesExpandAndMakeArray(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(2))
	engine.SetCell("A2", Number(3))
	engine.SetCell("B2", Number(4))

	engine.SetFormula("D1", `EXPAND(A1:B2,3,4,0)`)
	engine.SetFormula("J1", `MAKEARRAY(2,3,LAMBDA(r,c,r*10+c))`)
	engine.SetFormula("N1", `EXPAND(A1:B2,1,2)`)

	for ref, want := range map[string]float64{"D1": 1, "E1": 2, "D2": 3, "E2": 4, "F1": 0, "G1": 0, "F3": 0, "G3": 0} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	for ref, want := range map[string]float64{"J1": 11, "K1": 12, "L1": 13, "J2": 21, "K2": 22, "L2": 23} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
	if got := engine.GetCell("N1"); !got.IsError() || got.Err != ErrValue {
		t.Fatalf("N1 = %#v, want #VALUE!", got)
	}
}

func TestEngineRejectsArrayValuedMakeArrayResults(t *testing.T) {
	engine := NewEngine()
	engine.SetFormula("D1", `MAKEARRAY(2,2,LAMBDA(r,c,SEQUENCE(1,2,r+c,1)))`)

	if got := engine.GetCell("D1"); !got.IsError() || got.Err != ErrValue {
		t.Fatalf("D1 = %#v, want #VALUE! for array-valued lambda result", got)
	}
}

func TestEngineEvaluatesByRowByColMapReduceAndScan(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(2))
	engine.SetCell("C1", Number(3))
	engine.SetCell("A2", Number(4))
	engine.SetCell("B2", Number(5))
	engine.SetCell("C2", Number(6))
	engine.SetCell("E1", Number(10))
	engine.SetCell("E2", Number(20))
	engine.SetCell("E3", Number(30))
	engine.SetCell("F1", Number(1))
	engine.SetCell("F2", Number(2))
	engine.SetCell("F3", Number(3))

	engine.SetFormula("H1", `BYROW(A1:C2,LAMBDA(r,SUM(r)))`)
	engine.SetFormula("J1", `BYCOL(A1:C2,LAMBDA(c,MAX(c)))`)
	engine.SetFormula("N1", `MAP(E1:E3,F1:F3,LAMBDA(x,y,x+y))`)
	engine.SetFormula("P1", `REDUCE(0,E1:E3,LAMBDA(a,v,a+v))`)
	engine.SetFormula("R1", `SCAN(0,F1:F3,LAMBDA(a,v,a+v))`)

	for ref, want := range map[string]float64{"H1": 6, "H2": 15, "J1": 4, "K1": 5, "L1": 6, "N1": 11, "N2": 22, "N3": 33, "P1": 60, "R1": 1, "R2": 3, "R3": 6} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineRejectsArrayValuedByRowByColResults(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(2))
	engine.SetCell("A2", Number(3))
	engine.SetCell("B2", Number(4))

	engine.SetFormula("D1", `BYROW(A1:B2,LAMBDA(r,r))`)
	engine.SetFormula("F1", `BYCOL(A1:B2,LAMBDA(c,c))`)

	for ref := range map[string]struct{}{"D1": {}, "F1": {}} {
		if got := engine.GetCell(ref); !got.IsError() || got.Err != ErrValue {
			t.Fatalf("%s = %#v, want #VALUE! for array-valued lambda result", ref, got)
		}
	}
}

func TestEngineRejectsArrayValuedMapScanReduceResults(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))

	engine.SetFormula("D1", `MAP(A1:A2,B1:B2,LAMBDA(x,y,SEQUENCE(1,2,x,y)))`)
	engine.SetFormula("F1", `SCAN(0,A1:A2,LAMBDA(acc,v,SEQUENCE(1,2,acc+v,1)))`)
	engine.SetFormula("H1", `REDUCE(0,A1:A2,LAMBDA(acc,v,SEQUENCE(1,2,acc+v,1)))`)

	for ref := range map[string]struct{}{"D1": {}, "F1": {}, "H1": {}} {
		if got := engine.GetCell(ref); !got.IsError() || got.Err != ErrValue {
			t.Fatalf("%s = %#v, want #VALUE! for array-valued lambda result", ref, got)
		}
	}
}

func TestEngineRejectsNestedArrayValuedHigherOrderResults(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(2))
	engine.SetCell("A2", Number(3))
	engine.SetCell("B2", Number(4))

	engine.SetFormula("D1", `BYROW(A1:B2,LAMBDA(r,LAMBDA(x,x)(r)))`)
	engine.SetFormula("F1", `MAP(A1:A2,LAMBDA(x,LAMBDA(y,y)(SEQUENCE(1,2,x,1))))`)

	for ref := range map[string]struct{}{"D1": {}, "F1": {}} {
		if got := engine.GetCell(ref); !got.IsError() || got.Err != ErrValue {
			t.Fatalf("%s = %#v, want #VALUE! for nested array-valued lambda result", ref, got)
		}
	}
}

func TestEngineLookupWildcardEscapes(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("a*"))
	engine.SetCell("A2", Text("ab"))
	engine.SetCell("B1", Number(10))
	engine.SetCell("B2", Number(20))

	engine.SetFormula("D1", `XMATCH("a~*",A1:A2,2)`)
	engine.SetFormula("D2", `XLOOKUP("a~*",A1:A2,B1:B2,"missing",2)`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 1 {
		t.Fatalf("D1 = %#v, want numeric 1", got)
	}
	if got := engine.GetCell("D2"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("D2 = %#v, want numeric 10", got)
	}
}

func TestEngineEvaluatesLetBindingsAndArrayComposition(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("B1", Number(2))
	engine.SetCell("A2", Number(3))
	engine.SetCell("B2", Number(4))

	engine.SetFormula("D1", `LET(x,10,y,x*2,y+5)`)
	engine.SetFormula("E1", `LET(r,A1:B2,SUM(r))`)
	engine.SetFormula("G1", `LET(seq,SEQUENCE(2,2,1,1),seq*10)`)
	engine.SetFormula("J1", `BYROW(A1:B2,LAMBDA(r,LET(total,SUM(r),total*2)))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 25 {
		t.Fatalf("D1 = %#v, want numeric 25", got)
	}
	if got := engine.GetCell("E1"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("E1 = %#v, want numeric 10", got)
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 20, "G2": 30, "H2": 40, "J1": 6, "J2": 14} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesDirectLambdaInvocation(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(2))
	engine.SetCell("A2", Number(4))

	engine.SetFormula("D1", `LAMBDA(x,x+1)(2)`)
	engine.SetFormula("E1", `LAMBDA(x,y,x*y)(A1,A2)`)
	engine.SetFormula("G1", `LAMBDA(x,x*10)(SEQUENCE(2,2))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("D1 = %#v, want numeric 3", got)
	}
	if got := engine.GetCell("E1"); !got.IsNumber() || got.Num != 8 {
		t.Fatalf("E1 = %#v, want numeric 8", got)
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 20, "G2": 30, "H2": 40} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesLetBoundLambdaInvocation(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(5))

	engine.SetFormula("D1", `LET(f,LAMBDA(x,x+1),f(2))`)
	engine.SetFormula("E1", `LET(f,LAMBDA(x,y,x*y),f(A1,3))`)
	engine.SetFormula("G1", `LET(f,LAMBDA(x,x*10),f(SEQUENCE(2,2)))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 3 {
		t.Fatalf("D1 = %#v, want numeric 3", got)
	}
	if got := engine.GetCell("E1"); !got.IsNumber() || got.Num != 15 {
		t.Fatalf("E1 = %#v, want numeric 15", got)
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 20, "G2": 30, "H2": 40} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesLambdaReturnedFromLet(t *testing.T) {
	engine := NewEngine()

	engine.SetFormula("D1", `(LET(a,2,f,LAMBDA(x,x+a),f))(3)`)
	engine.SetFormula("G1", `(LET(scale,10,f,LAMBDA(x,x*scale),f))(SEQUENCE(2,2))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("D1 = %#v, want numeric 5", got)
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 20, "G2": 30, "H2": 40} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesLambdaReturnedFromLambda(t *testing.T) {
	engine := NewEngine()

	engine.SetFormula("D1", `LAMBDA(a,LAMBDA(x,x+a))(2)(3)`)
	engine.SetFormula("G1", `LAMBDA(scale,LAMBDA(x,x*scale))(10)(SEQUENCE(2,2))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("D1 = %#v, want numeric 5", got)
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 20, "G2": 30, "H2": 40} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesLambdaReturnedFromIfAndChoose(t *testing.T) {
	engine := NewEngine()

	engine.SetFormula("D1", `IF(1=1,LAMBDA(x,x+2),LAMBDA(x,x+99))(3)`)
	engine.SetFormula("E1", `CHOOSE(2,LAMBDA(x,x+1),LAMBDA(x,x*2))(4)`)
	engine.SetFormula("G1", `CHOOSE(1,LAMBDA(x,x*10),LAMBDA(x,x*100))(SEQUENCE(2,2))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("D1 = %#v, want numeric 5", got)
	}
	if got := engine.GetCell("E1"); !got.IsNumber() || got.Num != 8 {
		t.Fatalf("E1 = %#v, want numeric 8", got)
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 20, "G2": 30, "H2": 40} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesLambdaReturnedFromIfsAndSwitch(t *testing.T) {
	engine := NewEngine()

	engine.SetFormula("D1", `IFS(FALSE,LAMBDA(x,x+99),TRUE,LAMBDA(x,x+2))(3)`)
	engine.SetFormula("E1", `SWITCH(2,1,LAMBDA(x,x+1),2,LAMBDA(x,x*2))(4)`)
	engine.SetFormula("G1", `SWITCH(1,1,LAMBDA(x,x*10),2,LAMBDA(x,x*100))(SEQUENCE(2,2))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("D1 = %#v, want numeric 5", got)
	}
	if got := engine.GetCell("E1"); !got.IsNumber() || got.Num != 8 {
		t.Fatalf("E1 = %#v, want numeric 8", got)
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 20, "G2": 30, "H2": 40} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesLambdaReturnedFromIfErrorAndIfNa(t *testing.T) {
	engine := NewEngine()

	engine.SetFormula("D1", `IFERROR(UNKNOWNFUNC(),LAMBDA(x,x+2))(3)`)
	engine.SetFormula("E1", `IFNA(#N/A,LAMBDA(x,x*2))(4)`)
	engine.SetFormula("G1", `IFNA(#N/A,LAMBDA(x,x*10))(SEQUENCE(2,2))`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 5 {
		t.Fatalf("D1 = %#v, want numeric 5", got)
	}
	if got := engine.GetCell("E1"); !got.IsNumber() || got.Num != 8 {
		t.Fatalf("E1 = %#v, want numeric 8", got)
	}
	for ref, want := range map[string]float64{"G1": 10, "H1": 20, "G2": 30, "H2": 40} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestEngineEvaluatesBareBooleanConstants(t *testing.T) {
	engine := NewEngine()

	engine.SetFormula("D1", `IF(TRUE,10,20)`)
	engine.SetFormula("E1", `IF(FALSE,10,20)`)
	engine.SetFormula("F1", `TRUE=FALSE`)

	if got := engine.GetCell("D1"); !got.IsNumber() || got.Num != 10 {
		t.Fatalf("D1 = %#v, want numeric 10", got)
	}
	if got := engine.GetCell("E1"); !got.IsNumber() || got.Num != 20 {
		t.Fatalf("E1 = %#v, want numeric 20", got)
	}
	if got := engine.GetCell("F1"); !got.IsBool() || got.Bool {
		t.Fatalf("F1 = %#v, want boolean false", got)
	}
}

func TestEngineBlockedImplicitSpillsReturnSpillError(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(1))
	engine.SetCell("A2", Number(2))
	engine.SetCell("A3", Number(3))
	engine.SetCell("D2", Number(999))
	engine.SetFormula("D1", `A1:A3`)

	if got := engine.GetCell("D1"); !got.IsError() || got.Err != ErrSpill {
		t.Fatalf("D1 = %#v, want #SPILL!", got)
	}
	if got := engine.GetCell("D2"); !got.IsNumber() || got.Num != 999 {
		t.Fatalf("D2 = %#v, want numeric 999", got)
	}
	if got := engine.GetCell("D3"); !got.IsEmpty() {
		t.Fatalf("D3 = %#v, want empty", got)
	}
}

func TestEvaluateInContextSupportsSheetQualifiedRefs(t *testing.T) {
	values := map[string]Value{
		"JAN!A1":       Number(1),
		"FEB!A1":       Number(2),
		"MAR!A1":       Number(3),
		"SHEET2!A1":    Number(10),
		"SHEET2!A2":    Number(20),
		"SHEET2!A3":    Number(30),
		"'SHEET 2'!A1": Number(42),
		"[1]SHEET2!A1": Number(11),
		"[1]SHEET2!A2": Number(22),
		"[1]SHEET2!A3": Number(33),
	}
	getCell := func(ref string) Value {
		if v, ok := values[normalizeCellRef(ref)]; ok {
			return v
		}
		return Empty()
	}
	getRange := func(start, end string) []Value {
		eng := NewEngine()
		eng.SetExternalResolver(getCell, nil)
		startSheet, startCell := splitSheetRef(normalizeCellRef(start))
		endSheet, endCell := splitSheetRef(normalizeCellRef(end))
		if startSheet != "" && endSheet != "" && startSheet != endSheet {
			sheetOrder := []string{"JAN!", "FEB!", "MAR!"}
			startIndex, endIndex := -1, -1
			for i, sheet := range sheetOrder {
				if sheet == startSheet {
					startIndex = i
				}
				if sheet == endSheet {
					endIndex = i
				}
			}
			if startIndex >= 0 && endIndex >= 0 {
				if startIndex > endIndex {
					startIndex, endIndex = endIndex, startIndex
				}
				var values []Value
				for i := startIndex; i <= endIndex; i++ {
					values = append(values, eng.GetRange(sheetOrder[i]+startCell, sheetOrder[i]+endCell)...)
				}
				return values
			}
		}
		return eng.GetRange(start, end)
	}
	if got, err := EvaluateInContext(`Sheet2!A1`, getCell, getRange); err != nil || !got.IsNumber() || got.Num != 10 {
		t.Fatalf("Sheet2!A1 = %#v, err=%v, want numeric 10", got, err)
	}
	if got, err := EvaluateInContext(`SUM(Sheet2!A1:Sheet2!A3)`, getCell, getRange); err != nil || !got.IsNumber() || got.Num != 60 {
		t.Fatalf("SUM(Sheet2!A1:Sheet2!A3) = %#v, err=%v, want numeric 60", got, err)
	}
	if got, err := EvaluateInContext(`'Sheet 2'!A1`, getCell, getRange); err != nil || !got.IsNumber() || got.Num != 42 {
		t.Fatalf("'Sheet 2'!A1 = %#v, err=%v, want numeric 42", got, err)
	}
	if got, err := EvaluateInContext(`[1]Sheet2!A1`, getCell, getRange); err != nil || !got.IsNumber() || got.Num != 11 {
		t.Fatalf("[1]Sheet2!A1 = %#v, err=%v, want numeric 11", got, err)
	}
	if got, err := EvaluateInContext(`SUM([1]Sheet2!A1:[1]Sheet2!A3)`, getCell, getRange); err != nil || !got.IsNumber() || got.Num != 66 {
		t.Fatalf("SUM([1]Sheet2!A1:[1]Sheet2!A3) = %#v, err=%v, want numeric 66", got, err)
	}
	if got, err := EvaluateInContext(`SUM(Jan:Mar!A1)`, getCell, getRange); err != nil || !got.IsNumber() || got.Num != 6 {
		t.Fatalf("SUM(Jan:Mar!A1) = %#v, err=%v, want numeric 6", got, err)
	}
}

func TestEngineArrayFormulaRangeMismatchReturnsNA(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(10))
	engine.SetCell("A2", Number(20))
	engine.SetArrayFormula("C1", "C1:C3", "A1:A2")

	third := engine.GetCell("C3")
	if !third.IsError() || third.Err != ErrNA {
		t.Fatalf("C3 = %#v, want #N/A", third)
	}
}

func TestEngineRowAndColumnUseCurrentCellContext(t *testing.T) {
	engine := NewEngine()
	engine.SetFormula("C3", "ROW()+COLUMN()")

	result := engine.GetCell("C3")
	if !result.IsNumber() {
		t.Fatalf("Expected number, got %v", result.Type)
	}
	if result.Num != 6 {
		t.Fatalf("Num = %v, want 6", result.Num)
	}
}

func TestRowAndColumnFollowReferenceExpressions(t *testing.T) {
	expr, err := Parse("DROP(C3:F6,1,1)")
	if err != nil {
		t.Fatalf("Parse(DROP(...)): %v", err)
	}
	if targetRef, errValue := referenceTargetRef(expr, &engineContext{currentRef: "A7"}); targetRef != "D4" || errValue.Type != TypeEmpty {
		t.Fatalf("referenceTargetRef(DROP(...)) = (%q, %#v), want (\"D4\", empty)", targetRef, errValue)
	}
	rowExpr, err := Parse("ROW(DROP(C3:F6,1,1))")
	if err != nil {
		t.Fatalf("Parse(ROW(DROP(...))): %v", err)
	}
	if got := rowExpr.Eval(&engineContext{currentRef: "A7"}); !got.IsNumber() || got.Num != 4 {
		t.Fatalf("Eval(ROW(DROP(...))) = %#v, want numeric 4", got)
	}

	engine := NewEngine()
	engine.SetFormula("A1", "ROW(INDEX(B2:D4,0,2))")
	engine.SetFormula("A2", "COLUMN(INDEX(B2:D4,2,0))")
	engine.SetFormula("A3", "ROW(OFFSET(C5,0,0,2,3))")
	engine.SetFormula("A4", "COLUMN(CHOOSECOLS(B2:E4,1,3))")
	engine.SetFormula("A5", "ROW(CHOOSEROWS(B2:D5,1,3,4))")
	engine.SetFormula("A6", "COLUMN(TAKE(C3:F6,2,2))")
	engine.SetFormula("A7", "ROW(DROP(C3:F6,1,1))")

	for ref, want := range map[string]float64{"A1": 2, "A2": 2, "A3": 5, "A4": 2, "A5": 2, "A6": 3, "A7": 4} {
		if got := engine.GetCell(ref); !got.IsNumber() || got.Num != want {
			t.Fatalf("%s = %#v, want numeric %v", ref, got, want)
		}
	}
}

func TestSetFormulaClearsOverlappingImplicitSpillBinding(t *testing.T) {
	engine := NewEngine()
	engine.SetFormula("A1", "ROW(B2:D4)")
	engine.SetFormula("A2", "99")

	if got := engine.GetCell("A2"); !got.IsNumber() || got.Num != 99 {
		t.Fatalf("A2 = %#v, want numeric 99", got)
	}
}

func TestCountIf(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Text("x"))
	engine.SetCell("A2", Text("y"))
	engine.SetCell("A3", Text("x"))
	engine.SetFormula("A4", `COUNTIF(A1:A3,"x")`)

	result := engine.GetCell("A4")
	if !result.IsNumber() {
		t.Fatalf("Expected number, got %v", result.Type)
	}
	if result.Num != 2 {
		t.Errorf("Num = %v, want 2", result.Num)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name    string
		formula string
		errType ErrorType
	}{
		{"div by zero", "1/0", ErrDiv0},
		{"sqrt negative", "SQRT(-1)", ErrNum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if !result.IsError() {
				t.Fatalf("Expected error, got %v", result.Type)
			}
			if result.Err != tt.errType {
				t.Errorf("Err = %v, want %v", result.Err, tt.errType)
			}
		})
	}
}
func TestStatisticalGapFunctions(t *testing.T) {
	const eps = 1e-4
	engine := NewEngine()
	// A1:A5 = {2, 4, 4, 4, 5}, B1:B5 = {9, 10, 11, 12, 13}
	for i, v := range []float64{2, 4, 4, 4, 5} {
		engine.SetCell(fmt.Sprintf("A%d", i+1), Number(v))
	}
	for i, v := range []float64{9, 10, 11, 12, 13} {
		engine.SetCell(fmt.Sprintf("B%d", i+1), Number(v))
	}
	// C1:C5 = {1,2,3,4,5}
	for i, v := range []float64{1, 2, 3, 4, 5} {
		engine.SetCell(fmt.Sprintf("C%d", i+1), Number(v))
	}

	// Use a separate cell per test to avoid cache issues
	type tc struct {
		cell    string
		formula string
		want    float64
	}
	tests := []tc{
		// CORREL: {2,4,4,4,5} vs {9,10,11,12,13} -> cov=1.2, stdx=sqrt(4.8/5)=0.98, stdy=sqrt(10/5)=1.414 -> 6/sqrt(4.8*10)=6/6.928=0.8660
		{"T1", "CORREL(A1:A5,B1:B5)", 0.8660},
		// COVAR population = 6/5 = 1.2
		{"T2", "COVAR(A1:A5,B1:B5)", 1.2},
		{"T3", "COVARIANCE.P(A1:A5,B1:B5)", 1.2},
		// COVARIANCE.S = 6/4 = 1.5
		{"T4", "COVARIANCE.S(A1:A5,B1:B5)", 1.5},
		// SLOPE(y=A,x=B) = 6/10 = 0.6
		{"T5", "SLOPE(A1:A5,B1:B5)", 0.6},
		// INTERCEPT = 3.8 - 0.6*11 = 3.8 - 6.6 = -2.8
		{"T6", "INTERCEPT(A1:A5,B1:B5)", -2.8},
		// FORECAST(11, y=A, x=B) = 0.6*11 + (-2.8) = 3.8
		{"T7", "FORECAST(11,A1:A5,B1:B5)", 3.8},
		// PERCENTILE C1:C5 k=0.5 -> 3
		{"T8", "PERCENTILE(C1:C5,0.5)", 3.0},
		// PERCENTILE.EXC C1:C5 k=0.5 -> idx=0.5*6-1=2 -> sorted[2]=3
		{"T9", "PERCENTILE.EXC(C1:C5,0.5)", 3.0},
		// QUARTILE Q2 = median = 3
		{"T10", "QUARTILE(C1:C5,2)", 3.0},
		// QUARTILE.EXC Q2
		{"T11", "QUARTILE.EXC(C1:C5,2)", 3.0},
		// PERCENTRANK: value=3 in {1,2,3,4,5}, rank=2/(5-1)=0.5
		{"T12", "PERCENTRANK(C1:C5,3)", 0.5},
		// NORMDIST cumulative
		{"T13", "NORMDIST(0,0,1,1)", 0.5},
		// NORMDIST PDF ≈ 0.3989
		{"T14", "NORMDIST(0,0,1,0)", 0.3989},
		// NORMSDIST
		{"T15", "NORMSDIST(0)", 0.5},
		// NORMINV(0.5,0,1) = 0
		{"T16", "NORMINV(0.5,0,1)", 0.0},
		// NORMSINV(0.5) = 0
		{"T17", "NORMSINV(0.5)", 0.0},
		// CONFIDENCE.NORM alpha=0.05, sigma=1, n=100 -> z(0.975)*1/10 ≈ 0.196
		{"T18", "CONFIDENCE(0.05,1,100)", 0.1960},
		// FACTDOUBLE
		{"T19", "FACTDOUBLE(7)", 105.0},
		{"T20", "FACTDOUBLE(6)", 48.0},
		// COMBINA: C(2+3-1,3) = C(4,3) = 4
		{"T21", "COMBINA(2,3)", 4.0},
		// MULTINOMIAL: 6!/(2!2!2!)=90
		{"T22", "MULTINOMIAL(2,2,2)", 90.0},
		// SUMX2MY2: sum(x^2-y^2) = (4-81)+(16-100)+(16-121)+(16-144)+(25-169) = -538
		{"T23", "SUMX2MY2(A1:A5,B1:B5)", -538.0},
		// SUMX2PY2: sum(x^2+y^2) = (4+81)+(16+100)+(16+121)+(16+144)+(25+169) = 692
		{"T24", "SUMX2PY2(A1:A5,B1:B5)", 692.0},
		// SUMXMY2: sum((x-y)^2) = (2-9)^2+(4-10)^2+(4-11)^2+(4-12)^2+(5-13)^2 = 49+36+49+64+64 = 262
		{"T25", "SUMXMY2(A1:A5,B1:B5)", 262.0},
		// SUBTOTAL(9,...) = SUM
		{"T26", "SUBTOTAL(9,C1:C5)", 15.0},
		// PDURATION(0.1,100,200) = log(200/100)/log(1.1) = log(2)/log(1.1) ≈ 7.2725
		{"T27", "PDURATION(0.1,100,200)", 7.2725},
		// RRI(10,100,200) = (200/100)^(1/10)-1 = 2^0.1-1 ≈ 0.07177
		{"T28", "RRI(10,100,200)", 0.07177},
		// ISPMT(0.1, 1, 4, 1000) = 1000*0.1*(1/4-1) = 100*(-0.75) = -75
		{"T29", "ISPMT(0.1,1,4,1000)", -75.0},
		// DOLLARDE: 1.02 with fraction 16 -> 1 + 2/16 = 1.125
		{"T31", "DOLLARDE(1.02,16)", 1.125},
		// DOLLARFR: 1.125 with fraction 16 -> 1 + 0.125*16/100 = 1.02
		{"T32", "DOLLARFR(1.125,16)", 1.02},
	}

	for _, tt := range tests {
		t.Run(tt.formula, func(t *testing.T) {
			engine.SetFormula(tt.cell, tt.formula)
			got := engine.GetCell(tt.cell)
			if got.IsError() {
				t.Fatalf("got error %v", got.Err)
			}
			if math.Abs(got.Num-tt.want) > eps {
				t.Errorf("got %v, want %v", got.Num, tt.want)
			}
		})
	}
}

func TestMatrixFunctions(t *testing.T) {
	engine := NewEngine()
	// Set up 2x2 matrix in M1:N2 = [[1,2],[3,4]]
	engine.SetCell("M1", Number(1))
	engine.SetCell("N1", Number(2))
	engine.SetCell("M2", Number(3))
	engine.SetCell("N2", Number(4))

	// MDETERM([[1,2],[3,4]]) = 1*4-2*3 = -2
	engine.SetFormula("P1", "MDETERM(M1:N2)")
	got := engine.GetCell("P1")
	if got.IsError() {
		t.Fatalf("MDETERM: got error %v", got.Err)
	}
	if math.Abs(got.Num-(-2.0)) > 1e-9 {
		t.Errorf("MDETERM = %v, want -2", got.Num)
	}

	// MMULT: [[1,2],[3,4]] * [[1,2],[3,4]] = [[7,10],[15,22]]
	engine.SetFormula("P2", "MMULT(M1:N2,M1:N2)")
	// MMULT returns spill; P2 = [7, 10, 15, 22] for anchor P2, col/row offset
	// just check it doesn't error for anchor cell
	r := engine.GetCell("P2")
	if r.IsError() {
		t.Fatalf("MMULT anchor: got error %v", r.Err)
	}
	if math.Abs(r.Num-7.0) > 1e-9 {
		t.Errorf("MMULT[0,0] = %v, want 7", r.Num)
	}

	// MUNIT(2): 2x2 identity, anchor = [1, 0, 0, 1]
	engine.SetFormula("S1", "MUNIT(2)")
	r2 := engine.GetCell("S1")
	if r2.IsError() {
		t.Fatalf("MUNIT anchor: got error %v", r2.Err)
	}
	if math.Abs(r2.Num-1.0) > 1e-9 {
		t.Errorf("MUNIT[0,0] = %v, want 1", r2.Num)
	}
}

func TestMatrixFunctionsEdgeCases(t *testing.T) {
	engine := NewEngine()
	for ref, value := range map[string]Value{
		"A1": Number(1),
		"B1": Number(2),
		"A2": Number(3),
		"B2": Number(4),
		"D1": Number(2),
		"E1": Number(0),
		"D2": Number(0),
		"E2": Number(2),
		"G1": Number(1),
		"H1": Number(2),
		"I1": Number(3),
		"G2": Number(4),
		"H2": Number(5),
		"I2": Number(6),
		"K1": Number(1),
		"L1": Number(2),
		"K2": Number(2),
		"L2": Number(4),
	} {
		engine.SetCell(ref, value)
	}
	engine.SetFormula("N1", "MMULT(A1:B2,D1:E2)")
	engine.SetFormula("Q1", "MINVERSE(A1:B2)")
	engine.SetFormula("T1", "MUNIT(3)")
	engine.SetFormula("W1", "MMULT(G1:I2,A1:B2)")
	engine.SetFormula("W2", "MDETERM(G1:I2)")
	engine.SetFormula("W3", "MINVERSE(K1:L2)")
	engine.SetFormula("W4", "MUNIT(0)")

	for ref, want := range map[string]float64{
		"N1": 2,
		"O1": 4,
		"N2": 6,
		"O2": 8,
		"Q1": -2,
		"R1": 1,
		"Q2": 1.5,
		"R2": -0.5,
		"T1": 1,
		"U1": 0,
		"V1": 0,
		"T2": 0,
		"U2": 1,
		"V2": 0,
		"T3": 0,
		"U3": 0,
		"V3": 1,
	} {
		got := engine.GetCell(ref)
		if got.IsError() {
			t.Fatalf("%s: got error %v", ref, got.Err)
		}
		if math.Abs(got.Num-want) > 1e-9 {
			t.Fatalf("%s = %v, want %v", ref, got.Num, want)
		}
	}

	for ref, wantErr := range map[string]ErrorType{
		"W1": ErrValue,
		"W2": ErrValue,
		"W3": ErrNum,
		"W4": ErrValue,
	} {
		got := engine.GetCell(ref)
		if !got.IsError() || got.Err != wantErr {
			t.Fatalf("%s = %#v, want error %v", ref, got, wantErr)
		}
	}
}

func TestXNPVAndXIRR(t *testing.T) {
	const eps = 1e-4
	engine := NewEngine()

	// Cash flows: -1000, 400, 400, 400 at day 0, 365, 730, 1095
	// Store in D1:D4 and E1:E4
	for i, v := range []float64{-1000, 400, 400, 400} {
		engine.SetCell(fmt.Sprintf("D%d", i+1), Number(v))
	}
	for i, v := range []float64{0, 365, 730, 1095} {
		engine.SetCell(fmt.Sprintf("E%d", i+1), Number(v))
	}

	// XNPV(0.1, cashflows, dates): sum cf/(1+r)^(days/365)
	// = -1000 + 400/1.1 + 400/1.1^2 + 400/1.1^3
	// = -1000 + 363.636 + 330.579 + 300.526 = -5.259
	engine.SetFormula("F1", "XNPV(0.1,D1:D4,E1:E4)")
	got := engine.GetCell("F1")
	if got.IsError() {
		t.Fatalf("XNPV: got error %v", got.Err)
	}
	want := -1000.0 + 400/1.1 + 400/1.21 + 400/1.331
	if math.Abs(got.Num-want) > eps {
		t.Errorf("XNPV = %v, want %v", got.Num, want)
	}

	// XIRR: find rate r such that XNPV(r, cashflows, dates) = 0
	// For -1000,400,400,400 at years 0,1,2,3 the IRR ≈ 0.09701 (just above 9.7%)
	engine.SetFormula("F2", "XIRR(D1:D4,E1:E4)")
	r := engine.GetCell("F2")
	if r.IsError() {
		t.Fatalf("XIRR: got error %v", r.Err)
	}
	// Verify: XNPV at this rate should be near 0
	rate := r.Num
	xnpvCheck := -1000.0 + 400/math.Pow(1+rate, 1) + 400/math.Pow(1+rate, 2) + 400/math.Pow(1+rate, 3)
	if math.Abs(xnpvCheck) > 0.01 {
		t.Errorf("XIRR=%v but XNPV(%v) = %v (want ~0)", rate, rate, xnpvCheck)
	}
}

func TestAmpersandConcatOperator(t *testing.T) {
	tests := []struct {
		formula string
		want    string
	}{
		{`"hello" & " world"`, "hello world"},
		{`"a" & "b" & "c"`, "abc"},
		{`1 & " items"`, "1 items"},
		{`TRUE & " flag"`, "TRUE flag"},
		{`"val=" & 3.14`, "val=3.14"},
		{`"" & "x"`, "x"},
		{`"x" & ""`, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.formula, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("got error: %s", result.Err.String())
			}
			if !result.IsText() || result.Text != tt.want {
				t.Errorf("got %v, want text %q", result, tt.want)
			}
		})
	}

	// Error propagation: error in either operand propagates
	if got, err := Evaluate(`#VALUE! & "x"`); err != nil {
		t.Fatalf("error concat eval error: %v", err)
	} else if !got.IsError() || got.Err != ErrValue {
		t.Fatalf(`#VALUE! & "x" = %v, want #VALUE!`, got)
	}
	if got, err := Evaluate(`"x" & #REF!`); err != nil {
		t.Fatalf("error concat eval error: %v", err)
	} else if !got.IsError() || got.Err != ErrRef {
		t.Fatalf(`"x" & #REF! = %v, want #REF!`, got)
	}

	// Array context: & broadcasts across arrays
	engine := NewEngine()
	engine.SetCell("A1", Text("alpha"))
	engine.SetCell("A2", Text("beta"))
	engine.SetCell("A3", Text("gamma"))
	engine.SetArrayFormula("B1", "B1:B3", `A1:A3 & "!"`)
	for ref, want := range map[string]string{"B1": "alpha!", "B2": "beta!", "B3": "gamma!"} {
		if got := engine.GetCell(ref); !got.IsText() || got.Text != want {
			t.Fatalf("%s = %#v, want text %q", ref, got, want)
		}
	}
}

// TestCubeAndMacroGapFunctions verifies that Batch 7 (Cube/OLAP) and Batch 8
// (Macro/External-Call) functions return appropriate error values.
func TestCubeAndMacroGapFunctions(t *testing.T) {
	tests := []struct {
		formula string
		wantErr ErrorType
	}{
		// Batch 7: Cube/OLAP — return #REF! until OLAP data sources are supported
		{`CUBEMEMBER("Sales","[Product].[All]")`, ErrRef},
		{`CUBEVALUE("Sales","[Measures].[Amount]")`, ErrRef},
		{`CUBERANKEDMEMBER("Sales",1,"[Product].[All]")`, ErrRef},
		{`CUBEKPIMEMBER("Sales","Revenue","KPI")`, ErrRef},
		{`CUBEMEMBERPROPERTY("Sales","[Product].[A]","[Color]")`, ErrRef},
		// Batch 8: Macro/External-Call — return #VALUE! until macro support is available
		{`CALL("DLL","Func","JJE",1,2)`, ErrValue},
		{`REGISTER("DLL","Func","JJE",1,2)`, ErrValue},
		{`REGISTER.ID("DLL","Func","JJE",1,2)`, ErrValue},
	}
	for _, tt := range tests {
		t.Run(tt.formula, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if !result.IsError() {
				t.Errorf("%s = %v (type=%v), want error %v", tt.formula, result, result.Type, tt.wantErr)
				return
			}
			if result.Err != tt.wantErr {
				t.Errorf("%s = error %v, want error %v", tt.formula, result.Err, tt.wantErr)
			}
		})
	}
}

func TestPercentPostfixOperator(t *testing.T) {
	tests := []struct {
		formula string
		want    float64
	}{
		{"50%", 0.5},
		{"100%", 1.0},
		{"200%", 2.0},
		{"0%", 0.0},
		{"1%", 0.01},
		{"50% + 0.5", 1.0},
		{"10% * 200", 20.0},
	}
	for _, tt := range tests {
		t.Run(tt.formula, func(t *testing.T) {
			result, err := Evaluate(tt.formula)
			if err != nil {
				t.Fatalf("Evaluate error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("got error: %s", result.Err.String())
			}
			if !result.IsNumber() || math.Abs(result.Num-tt.want) > 1e-9 {
				t.Errorf("got %v, want numeric %v", result.Num, tt.want)
			}
		})
	}
}

func TestEngineEvaluateArbitraryExpression(t *testing.T) {
	engine := NewEngine()
	engine.SetCell("A1", Number(5))
	got, err := engine.Evaluate("A1*2")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !got.IsNumber() || got.Num != 10 {
		t.Fatalf("Evaluate = %#v, want 10", got)
	}
}
