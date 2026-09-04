package render

import (
	"testing"
)

func TestEvaluateWordFormula(t *testing.T) {
	tests := []struct {
		instr string
		want  string
	}{
		{"= 2+2", "4"},
		{"=2+2", "4"},
		{"= 10-3", "7"},
		{"= 4*5", "20"},
		{"= 20/4", "5"},
		{"= 2+3*4", "14"},
		{"= (2+3)*4", "20"},
		{"= 10/3", "3.33"},
		{"= -5+3", "-2"},
		{"= +5", "5"},
		{"= 2^3", "8"},
		{"= 10%3", "1"},
		{"= SUM(1,2,3)", "6"},
		{"= AVERAGE(2,4,6)", "4"},
		{"= MIN(3,1,2)", "1"},
		{"= MAX(3,1,2)", "3"},
		{"= COUNT(1,2,3)", "3"},
		{"= PRODUCT(2,3,4)", "24"},
		{"= ABS(-5)", "5"},
		{"= INT(3.7)", "3"},
		{"= ROUND(3.14159,2)", "3.14"},
		{"= TRUE", "1"},
		{"= FALSE", "0"},
		{"= ABOVE", "0"},
		{"= BELOW", "0"},
		{"= LEFT", "0"},
		{"= RIGHT", "0"},
		{"= SUM(ABOVE)", "0"},
		{"= 3.5+2.5", "6"},
		{"= 0.1+0.2", "0.30"},
		{"= SUM(1)", "1"},
		{"= AVERAGE(5)", "5"},
		{"= ROUND(2.5)", "3"},
		{"= 5/2", "2.50"},
		{"= (1+2)*(3+4)", "21"},
		{"= 2^2^2", "16"},
		{"", ""},
		{"=", ""},
		{"= unknown", ""},
		{"= 5/0", ""},
	}

	for _, tt := range tests {
		t.Run(tt.instr, func(t *testing.T) {
			got := evaluateWordFormula(tt.instr)
			if got != tt.want {
				t.Errorf("evaluateWordFormula(%q) = %q, want %q", tt.instr, got, tt.want)
			}
		})
	}
}

func TestEvaluateWordFormulaFullPreservesCachedText(t *testing.T) {
	got := evaluateWordFormulaFull("= 2+2", "4")
	if got != "" {
		t.Errorf("expected empty string when cached text is present, got %q", got)
	}
	got = evaluateWordFormulaFull("= 2+2", "")
	if got != "4" {
		t.Errorf("expected computed value when cached text is empty, got %q", got)
	}
}

func TestEvaluateWordFormulaFunctionEdgeCases(t *testing.T) {
	// Empty arg list
	got := evaluateWordFormula("= SUM()")
	if got != "0" {
		t.Errorf("SUM() = %q, want 0", got)
	}
	got = evaluateWordFormula("= AVERAGE()")
	if got != "0" {
		t.Errorf("AVERAGE() = %q, want 0", got)
	}
	got = evaluateWordFormula("= MIN()")
	if got != "0" {
		t.Errorf("MIN() = %q, want 0", got)
	}
	got = evaluateWordFormula("= MAX()")
	if got != "0" {
		t.Errorf("MAX() = %q, want 0", got)
	}
	got = evaluateWordFormula("= COUNT()")
	if got != "0" {
		t.Errorf("COUNT() = %q, want 0", got)
	}
	// Missing required arg
	got = evaluateWordFormula("= ABS()")
	if got != "" {
		t.Errorf("ABS() without args should fail, got %q", got)
	}
	got = evaluateWordFormula("= INT()")
	if got != "" {
		t.Errorf("INT() without args should fail, got %q", got)
	}
	got = evaluateWordFormula("= ROUND()")
	if got != "" {
		t.Errorf("ROUND() without args should fail, got %q", got)
	}
}