package drawing

import (
	"math"
	"testing"
)

func TestEvaluateFormulaSpecialIdentifiers(t *testing.T) {
	ctx := GuideContext{Width: 200, Height: 100, Val: 42}

	// Dimensions
	if got, _ := EvaluateFormula("wd", ctx); got != 200 {
		t.Errorf("wd = %v, want 200", got)
	}
	if got, _ := EvaluateFormula("ht", ctx); got != 100 {
		t.Errorf("ht = %v, want 100", got)
	}
	if got, _ := EvaluateFormula("val", ctx); got != 42 {
		t.Errorf("val = %v, want 42", got)
	}

	// Shortest side
	if got, _ := EvaluateFormula("ssd", ctx); got != 100 {
		t.Errorf("ssd = %v, want 100", got)
	}

	// Longest side
	if got, _ := EvaluateFormula("lsd", ctx); got != 200 {
		t.Errorf("lsd = %v, want 200", got)
	}

	// Edges
	if got, _ := EvaluateFormula("t", ctx); got != 0 {
		t.Errorf("t = %v, want 0", got)
	}
	if got, _ := EvaluateFormula("b", ctx); got != 100 {
		t.Errorf("b = %v, want 100", got)
	}
	if got, _ := EvaluateFormula("l", ctx); got != 0 {
		t.Errorf("l = %v, want 0", got)
	}
	if got, _ := EvaluateFormula("r", ctx); got != 200 {
		t.Errorf("r = %v, want 200", got)
	}

	// Constants
	cd2, _ := EvaluateFormula("cd2", ctx)
	if math.Abs(cd2-math.Pi/2) > 1e-9 {
		t.Errorf("cd2 = %v, want pi/2", cd2)
	}
	cd4, _ := EvaluateFormula("cd4", ctx)
	if math.Abs(cd4-math.Pi/4) > 1e-9 {
		t.Errorf("cd4 = %v, want pi/4", cd4)
	}
}

func TestEvaluateFormulaGuideReference(t *testing.T) {
	ctx := GuideContext{Width: 100, Height: 100, Guides: map[string]float64{"adj1": 50}}
	if got, _ := EvaluateFormula("adj1", ctx); got != 50 {
		t.Errorf("guide ref = %v, want 50", got)
	}
}

func TestEvaluateFormulaNumberLiteral(t *testing.T) {
	ctx := GuideContext{}
	if got, _ := EvaluateFormula("3", ctx); got != 3 {
		t.Errorf("literal = %v, want 3", got)
	}
}

func TestEvaluateFormulaUnknownIdentifier(t *testing.T) {
	_, err := EvaluateFormula("unknown_var", GuideContext{})
	if err == nil {
		t.Error("expected error for unknown identifier")
	}
}

func TestEvaluateFormulaSinCosTan(t *testing.T) {
	// sin(30°) = 0.5
	got, _ := EvaluateFormula("sin(30)", GuideContext{})
	if math.Abs(got-0.5) > 0.001 {
		t.Errorf("sin(30) = %v, want 0.5", got)
	}
	// cos(0°) = 1
	got2, _ := EvaluateFormula("cos(0)", GuideContext{})
	if math.Abs(got2-1.0) > 0.001 {
		t.Errorf("cos(0) = %v, want 1.0", got2)
	}
	// tan(45°) = 1
	got3, _ := EvaluateFormula("tan(45)", GuideContext{})
	if math.Abs(got3-1.0) > 0.001 {
		t.Errorf("tan(45) = %v, want 1.0", got3)
	}
}

func TestEvaluateFormulaAbsSqrt(t *testing.T) {
	got, _ := EvaluateFormula("abs(-5)", GuideContext{})
	if got != 5 {
		t.Errorf("abs(-5) = %v, want 5", got)
	}
	got2, _ := EvaluateFormula("sqrt(16)", GuideContext{})
	if got2 != 4 {
		t.Errorf("sqrt(16) = %v, want 4", got2)
	}
}

func TestEvaluateFormulaSqrtNegative(t *testing.T) {
	_, err := EvaluateFormula("sqrt(-1)", GuideContext{})
	if err == nil {
		t.Error("expected error for sqrt of negative")
	}
}

func TestEvaluateFormulaMaxMin(t *testing.T) {
	got, _ := EvaluateFormula("max(1, 5, 3)", GuideContext{})
	if got != 5 {
		t.Errorf("max = %v, want 5", got)
	}
	got2, _ := EvaluateFormula("min(1, 5, 3)", GuideContext{})
	if got2 != 1 {
		t.Errorf("min = %v, want 1", got2)
	}
}

func TestEvaluateFormulaPin(t *testing.T) {
	got, _ := EvaluateFormula("pin(10, 0, 5)", GuideContext{})
	if got != 5 {
		t.Errorf("pin(10,0,5) = %v, want 5", got)
	}
	got2, _ := EvaluateFormula("pin(-1, 0, 5)", GuideContext{})
	if got2 != 0 {
		t.Errorf("pin(-1,0,5) = %v, want 0", got2)
	}
	got3, _ := EvaluateFormula("pin(3, 0, 5)", GuideContext{})
	if got3 != 3 {
		t.Errorf("pin(3,0,5) = %v, want 3", got3)
	}
}

func TestEvaluateFormulaMid(t *testing.T) {
	got, _ := EvaluateFormula("mid(1, 5, 3)", GuideContext{})
	if got != 3 {
		t.Errorf("mid(1,5,3) = %v, want 3", got)
	}
}

func TestEvaluateFormulaUnknownFunction(t *testing.T) {
	_, err := EvaluateFormula("foo(1)", GuideContext{})
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

func TestEvaluateFormulaFunctionArgError(t *testing.T) {
	_, err := EvaluateFormula("sin()", GuideContext{})
	if err == nil {
		t.Error("expected error for sin with no args")
	}
	_, err2 := EvaluateFormula("sin(1, 2)", GuideContext{})
	if err2 == nil {
		t.Error("expected error for sin with too many args")
	}
}

func TestEvaluateFormulaIfProd(t *testing.T) {
	got, _ := EvaluateFormula("if(1, 10, 20)", GuideContext{})
	if got != 10 {
		t.Errorf("if(1,10,20) = %v, want 10", got)
	}
	got2, _ := EvaluateFormula("prod(2, 3, 4)", GuideContext{})
	if got2 != 24 {
		t.Errorf("prod(2,3,4) = %v, want 24", got2)
	}
}

func TestEvaluateFormulaMod(t *testing.T) {
	got, _ := EvaluateFormula("mod(3, 4, 0)", GuideContext{})
	if got != 5 {
		t.Errorf("mod(3,4,0) = %v, want 5", got)
	}
}

func TestEvaluateFormulaAt2(t *testing.T) {
	// at2(x, y) = atan2(y, x) in degrees
	got, _ := EvaluateFormula("at2(1, 0)", GuideContext{})
	if math.Abs(got-0) > 0.001 {
		t.Errorf("at2(1,0) = %v, want 0", got)
	}
	got2, _ := EvaluateFormula("at2(0, 1)", GuideContext{})
	if math.Abs(got2-90) > 0.001 {
		t.Errorf("at2(0,1) = %v, want 90", got2)
	}
}

func TestEvaluateFormulaCat2Sat2(t *testing.T) {
	got, _ := EvaluateFormula("cat2(10, 1, 0)", GuideContext{})
	if math.Abs(got-10) > 0.001 {
		t.Errorf("cat2(10,1,0) = %v, want 10", got)
	}
	got2, _ := EvaluateFormula("sat2(10, 1, 0)", GuideContext{})
	if math.Abs(got2-0) > 0.001 {
		t.Errorf("sat2(10,1,0) = %v, want 0", got2)
	}
}

func TestEvaluateFormulaDivisionByZero(t *testing.T) {
	_, err := EvaluateFormula("1/0", GuideContext{})
	if err == nil {
		t.Error("expected error for division by zero")
	}
}

func TestEvaluateFormulaUnaryMinus(t *testing.T) {
	got, _ := EvaluateFormula("-5", GuideContext{})
	if got != -5 {
		t.Errorf("-5 = %v, want -5", got)
	}
	got2, _ := EvaluateFormula("--5", GuideContext{})
	if got2 != 5 {
		t.Errorf("--5 = %v, want 5", got2)
	}
}

func TestEvaluateFormulaMissingParen(t *testing.T) {
	_, err := EvaluateFormula("(1+2", GuideContext{})
	if err == nil {
		t.Error("expected error for missing paren")
	}
}

func TestEvaluateFormulaUnexpectedToken(t *testing.T) {
	_, err := EvaluateFormula(",", GuideContext{})
	if err == nil {
		t.Error("expected error for unexpected token")
	}
}

func TestEvaluateFormulaMissingComma(t *testing.T) {
	_, err := EvaluateFormula("sin(1 2)", GuideContext{})
	if err == nil {
		t.Error("expected error for missing comma")
	}
}

func TestGuideEvaluateNoFormula(t *testing.T) {
	g := Guide{Value: 42}
	got, err := g.Evaluate(GuideContext{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != 42 {
		t.Errorf("value = %v, want 42", got)
	}
}

func TestGuideEvaluateWithFormula(t *testing.T) {
	g := Guide{Formula: "wd/2", Value: 10}
	got, err := g.Evaluate(GuideContext{Width: 100})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != 50 {
		t.Errorf("value = %v, want 50", got)
	}
}

func TestSSDAndLSDBranches(t *testing.T) {
	// ssd when width > height
	got, _ := EvaluateFormula("ssd", GuideContext{Width: 200, Height: 100})
	if got != 100 {
		t.Errorf("ssd(w>h) = %v, want 100", got)
	}
	// ssd when width < height
	got2, _ := EvaluateFormula("ssd", GuideContext{Width: 50, Height: 100})
	if got2 != 50 {
		t.Errorf("ssd(w<h) = %v, want 50", got2)
	}

	// lsd when width > height
	got3, _ := EvaluateFormula("lsd", GuideContext{Width: 200, Height: 100})
	if got3 != 200 {
		t.Errorf("lsd(w>h) = %v, want 200", got3)
	}
	// lsd when width < height
	got4, _ := EvaluateFormula("lsd", GuideContext{Width: 50, Height: 100})
	if got4 != 100 {
		t.Errorf("lsd(w<h) = %v, want 100", got4)
	}
}

func TestEvaluateAllWithCycle(t *testing.T) {
	ge := NewGuideEvaluator()
	ge.AddGuide("a", &Guide{Name: "a", Formula: "b+1"})
	ge.AddGuide("b", &Guide{Name: "b", Formula: "a+1"})
	_, err := ge.EvaluateAll(100, 100)
	if err == nil {
		t.Error("expected error for circular dependency")
	}
}

func TestEvaluateAllFixedAndFormula(t *testing.T) {
	ge := NewGuideEvaluator()
	ge.AddGuide("fixed", &Guide{Name: "fixed", Value: 10})
	ge.AddGuide("derived", &Guide{Name: "derived", Formula: "fixed*2"})
	results, err := ge.EvaluateAll(100, 100)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if results["fixed"] != 10 {
		t.Errorf("fixed = %v, want 10", results["fixed"])
	}
	if results["derived"] != 20 {
		t.Errorf("derived = %v, want 20", results["derived"])
	}
}

func TestNormalizePathArc(t *testing.T) {
	path := Path{
		Commands: []PathCommand{{
			Type:   "A",
			Args:   []float64{0.5, 0.3, 0, 1, 0},
			Points: []Point{{X: 0.5, Y: 0.5}},
		}},
	}
	normalized := NormalizePath(path, 200, 100)
	if len(normalized.Commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(normalized.Commands))
	}
	cmd := normalized.Commands[0]
	if cmd.Args[0] != 100 { // 0.5 * 200
		t.Errorf("rx = %v, want 100", cmd.Args[0])
	}
	if cmd.Args[1] != 30 { // 0.3 * 100
		t.Errorf("ry = %v, want 30", cmd.Args[1])
	}
	if cmd.Points[0].X != 100 || cmd.Points[0].Y != 50 {
		t.Errorf("point = (%v,%v), want (100,50)", cmd.Points[0].X, cmd.Points[0].Y)
	}
}

func TestFindDependencies(t *testing.T) {
	deps := findDependencies("adj1 + adj2 * wd")
	if len(deps) != 2 {
		t.Fatalf("deps = %d, want 2", len(deps))
	}
	// wd should be skipped
	for _, d := range deps {
		if d == "wd" || d == "ht" || d == "val" {
			t.Errorf("should skip %s", d)
		}
	}
}

func TestFindDependenciesWithFunction(t *testing.T) {
	deps := findDependencies("sin(adj1) + cos(adj2)")
	if len(deps) != 2 {
		t.Fatalf("deps = %d, want 2", len(deps))
	}
}

func TestEvaluateFormulaWhitespace(t *testing.T) {
	got, _ := EvaluateFormula(" 3 + 4 ", GuideContext{})
	if got != 7 {
		t.Errorf("whitespace = %v, want 7", got)
	}
}

func TestEvaluateFormulaParenExpression(t *testing.T) {
	got, _ := EvaluateFormula("(3 + 4) * 2", GuideContext{})
	if got != 14 {
		t.Errorf("paren = %v, want 14", got)
	}
}

func TestEvaluateFormulaIfZero(t *testing.T) {
	got, _ := EvaluateFormula("if(0, 10, 20)", GuideContext{})
	if got != 20 {
		t.Errorf("if(0) = %v, want 20", got)
	}
	got2, _ := EvaluateFormula("if(-1, 10, 20)", GuideContext{})
	if got2 != 20 {
		t.Errorf("if(-1) = %v, want 20", got2)
	}
}

func TestGeometryEvaluateGuides(t *testing.T) {
	g := Geometry{
		Guides: []Guide{
			{Name: "adj1", Value: 10},
			{Name: "adj2", Formula: "adj1*2"},
		},
	}
	results, err := g.EvaluateGuides(100, 100)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if results["adj1"] != 10 {
		t.Errorf("adj1 = %v, want 10", results["adj1"])
	}
	if results["adj2"] != 20 {
		t.Errorf("adj2 = %v, want 20", results["adj2"])
	}
}

func TestGeometryEvaluateGuidesAutoName(t *testing.T) {
	g := Geometry{
		Guides: []Guide{
			{Value: 5},
			{Value: 10},
		},
	}
	results, err := g.EvaluateGuides(100, 100)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if results["adj1"] != 5 {
		t.Errorf("adj1 = %v, want 5", results["adj1"])
	}
	if results["adj2"] != 10 {
		t.Errorf("adj2 = %v, want 10", results["adj2"])
	}
}

func TestEvaluateFormulaComplex(t *testing.T) {
	ctx := GuideContext{Width: 100, Height: 50, Guides: map[string]float64{"adj1": 10}}
	got, _ := EvaluateFormula("wd/2 + adj1*3 - ht", ctx)
	// 100/2 + 10*3 - 50 = 50 + 30 - 50 = 30
	if got != 30 {
		t.Errorf("complex = %v, want 30", got)
	}
}

func TestEvaluateFormulaMaxMinSingleArg(t *testing.T) {
	got, _ := EvaluateFormula("max(5)", GuideContext{})
	if got != 5 {
		t.Errorf("max(5) = %v, want 5", got)
	}
	got2, _ := EvaluateFormula("min(5)", GuideContext{})
	if got2 != 5 {
		t.Errorf("min(5) = %v, want 5", got2)
	}
}

func TestEvaluateFormulaProdSingleArg(t *testing.T) {
	got, _ := EvaluateFormula("prod(5)", GuideContext{})
	if got != 5 {
		t.Errorf("prod(5) = %v, want 5", got)
	}
}

func TestEvaluateFormulaPinMiddle(t *testing.T) {
	got, _ := EvaluateFormula("pin(5, 0, 10)", GuideContext{})
	if got != 5 {
		t.Errorf("pin(5,0,10) = %v, want 5", got)
	}
}

func TestEvaluateFormulaMidAllOrders(t *testing.T) {
	// Test different orderings to cover all branches of mid's sorting
	got1, _ := EvaluateFormula("mid(3, 1, 2)", GuideContext{})
	if got1 != 2 {
		t.Errorf("mid(3,1,2) = %v, want 2", got1)
	}
	got2, _ := EvaluateFormula("mid(1, 3, 2)", GuideContext{})
	if got2 != 2 {
		t.Errorf("mid(1,3,2) = %v, want 2", got2)
	}
	got3, _ := EvaluateFormula("mid(2, 1, 3)", GuideContext{})
	if got3 != 2 {
		t.Errorf("mid(2,1,3) = %v, want 2", got3)
	}
}

func TestEvaluateFormulaWAndHShortcuts(t *testing.T) {
	ctx := GuideContext{Width: 200, Height: 100}
	got, _ := EvaluateFormula("w", ctx)
	if got != 200 {
		t.Errorf("w = %v, want 200", got)
	}
	got2, _ := EvaluateFormula("h", ctx)
	if got2 != 100 {
		t.Errorf("h = %v, want 100", got2)
	}
}

func TestEvaluateFormulaUnknownIdentifierNumericFallback(t *testing.T) {
	// "123" is treated as a number literal
	got, _ := EvaluateFormula("123", GuideContext{})
	if got != 123 {
		t.Errorf("numeric literal = %v, want 123", got)
	}
}

func TestEvaluateFormulaUnaryPlus(t *testing.T) {
	got, _ := EvaluateFormula("+5", GuideContext{})
	if got != 5 {
		t.Errorf("+5 = %v, want 5", got)
	}
}

func TestEvaluateFormulaEmpty(t *testing.T) {
	_, err := EvaluateFormula("", GuideContext{})
	if err == nil {
		t.Error("expected error for empty formula")
	}
}

func TestEvaluateFormulaMaxMinEmpty(t *testing.T) {
	_, err := EvaluateFormula("max()", GuideContext{})
	if err == nil {
		t.Error("expected error for max() with no args")
	}
	_, err2 := EvaluateFormula("min()", GuideContext{})
	if err2 == nil {
		t.Error("expected error for min() with no args")
	}
}

func TestEvaluateFormulaPinWrongArgs(t *testing.T) {
	_, err := EvaluateFormula("pin(1, 2)", GuideContext{})
	if err == nil {
		t.Error("expected error for pin with 2 args")
	}
}

func TestEvaluateFormulaPinBelowMin(t *testing.T) {
	got, _ := EvaluateFormula("pin(1, 2, 3)", GuideContext{})
	if got != 2 {
		t.Errorf("pin(1,2,3) = %v, want 2", got)
	}
}

func TestEvaluateFormulaMidXGreaterThanY(t *testing.T) {
	// Test where first arg > second arg to hit the first swap branch
	got, _ := EvaluateFormula("mid(3, 1, 2)", GuideContext{})
	if got != 2 {
		t.Errorf("mid(3,1,2) = %v, want 2", got)
	}
}

func TestEvaluateFormulaMidSecondSwap(t *testing.T) {
	// mid(3,1,0): after first swap (1,3,0), after y>z (1,0,3), second x>y is false
	// mid(2,0,1): after first swap (0,2,1), after y>z (0,1,2), second x>y is false
	// Need a case where second x>y is true: mid(3,1,0) -> after y>z: (1,0,3) -> x=1,y=0 -> x>y true
	got, _ := EvaluateFormula("mid(3, 1, 0)", GuideContext{})
	if got != 1 {
		t.Errorf("mid(3,1,0) = %v, want 1", got)
	}
}

func TestEvaluateFormulaMinDecreasing(t *testing.T) {
	// min(3, 1, 2) should hit arg < min branch
	got, _ := EvaluateFormula("min(3, 1, 2)", GuideContext{})
	if got != 1 {
		t.Errorf("min(3,1,2) = %v, want 1", got)
	}
}

func TestEvaluateFormulaCosTanAbsWrongArgs(t *testing.T) {
	for _, fn := range []string{"cos", "tan", "abs"} {
		_, err := EvaluateFormula(fn+"()", GuideContext{})
		if err == nil {
			t.Errorf("expected error for %s with no args", fn)
		}
		_, err2 := EvaluateFormula(fn+"(1, 2)", GuideContext{})
		if err2 == nil {
			t.Errorf("expected error for %s with too many args", fn)
		}
	}
}

func TestEvaluateFormulaSqrtWrongArgs(t *testing.T) {
	_, err := EvaluateFormula("sqrt()", GuideContext{})
	if err == nil {
		t.Error("expected error for sqrt with no args")
	}
	_, err2 := EvaluateFormula("sqrt(1, 2)", GuideContext{})
	if err2 == nil {
		t.Error("expected error for sqrt with too many args")
	}
}

func TestEvaluateFormulaSinWithBrokenArg(t *testing.T) {
	_, err := EvaluateFormula("sin(1+,)", GuideContext{})
	if err == nil {
		t.Error("expected error for broken argument expression")
	}
}

func TestEvaluateFormulaAddSubWithError(t *testing.T) {
	_, err := EvaluateFormula("1 + ,", GuideContext{})
	if err == nil {
		t.Error("expected error for add with broken right side")
	}
}

func TestEvaluateFormulaMulDivWithError(t *testing.T) {
	_, err := EvaluateFormula("1 * ,", GuideContext{})
	if err == nil {
		t.Error("expected error for multiply with broken right side")
	}
}

func TestEvaluateFormulaUnaryWithBrokenArg(t *testing.T) {
	_, err := EvaluateFormula("-(1+,)", GuideContext{})
	if err == nil {
		t.Error("expected error for unary minus with broken arg")
	}
}

func TestEvaluateFormulaNumericLiteralIdentifier(t *testing.T) {
	// "3" as an identifier should parse as a number
	got, _ := EvaluateFormula("3*2", GuideContext{})
	if got != 6 {
		t.Errorf("3*2 = %v, want 6", got)
	}
}

func TestFindDependenciesNumericSkip(t *testing.T) {
	deps := findDependencies("adj1 + 3 * wd")
	for _, d := range deps {
		if d == "3" {
			t.Error("should skip numeric literal 3")
		}
	}
}

func TestEvaluateAllFormulaError(t *testing.T) {
	ge := NewGuideEvaluator()
	ge.AddGuide("a", &Guide{Name: "a", Formula: "unknown_var + 1"})
	_, err := ge.EvaluateAll(100, 100)
	if err == nil {
		t.Error("expected error for invalid formula")
	}
}

func TestEvaluateAllFormulaEvalError(t *testing.T) {
	ge := NewGuideEvaluator()
	ge.AddGuide("a", &Guide{Name: "a", Formula: "sqrt(-1)"})
	_, err := ge.EvaluateAll(100, 100)
	if err == nil {
		t.Error("expected error for formula evaluation error")
	}
}

func TestTopologicalSortPartialCycle(t *testing.T) {
	ge := NewGuideEvaluator()
	ge.AddGuide("a", &Guide{Name: "a", Formula: "b+1"})
	ge.AddGuide("b", &Guide{Name: "b", Formula: "a+1"})
	ge.AddGuide("c", &Guide{Name: "c", Formula: "wd*2"})
	_, err := ge.EvaluateAll(100, 100)
	if err == nil {
		t.Error("expected error for partial cycle")
	}
}

func TestEvaluateFormulaMidWrongArgs(t *testing.T) {
	_, err := EvaluateFormula("mid(1, 2)", GuideContext{})
	if err == nil {
		t.Error("expected error for mid with 2 args")
	}
}

func TestEvaluateFormulaAt2WrongArgs(t *testing.T) {
	_, err := EvaluateFormula("at2(1)", GuideContext{})
	if err == nil {
		t.Error("expected error for at2 with 1 arg")
	}
}

func TestEvaluateFormulaCat2WrongArgs(t *testing.T) {
	_, err := EvaluateFormula("cat2(1, 2)", GuideContext{})
	if err == nil {
		t.Error("expected error for cat2 with 2 args")
	}
}

func TestEvaluateFormulaSat2WrongArgs(t *testing.T) {
	_, err := EvaluateFormula("sat2(1, 2)", GuideContext{})
	if err == nil {
		t.Error("expected error for sat2 with 2 args")
	}
}

func TestEvaluateFormulaModWrongArgs(t *testing.T) {
	_, err := EvaluateFormula("mod(1, 2)", GuideContext{})
	if err == nil {
		t.Error("expected error for mod with 2 args")
	}
}

func TestEvaluateFormulaIfWrongArgs(t *testing.T) {
	_, err := EvaluateFormula("if(1, 2)", GuideContext{})
	if err == nil {
		t.Error("expected error for if with 2 args")
	}
}

func TestEvaluateFormulaProdEmpty(t *testing.T) {
	_, err := EvaluateFormula("prod()", GuideContext{})
	if err == nil {
		t.Error("expected error for prod with no args")
	}
}

func TestEvaluateFormulaMissingCloseParen(t *testing.T) {
	_, err := EvaluateFormula("sin(1", GuideContext{})
	if err == nil {
		t.Error("expected error for missing close paren")
	}
}

func TestEvaluateFormulaMissingCommaInArgs(t *testing.T) {
	_, err := EvaluateFormula("max(1 2)", GuideContext{})
	if err == nil {
		t.Error("expected error for missing comma")
	}
}

func TestFormulaParserCurrentPastEOF(t *testing.T) {
	p := newFormulaParser("1", GuideContext{})
	p.advance() // move past the number
	p.advance() // move past EOF
	_ = p.current()
	// Should not panic; returns tokenEOF
}

func TestEvaluateFormulaUnexpectedOperator(t *testing.T) {
	_, err := EvaluateFormula("*", GuideContext{})
	if err == nil {
		t.Error("expected error for unexpected operator token")
	}
}

func TestEvaluateFormulaTokenizeUnknownChar(t *testing.T) {
	// Unknown characters like ! should be skipped by tokenizer
	got, _ := EvaluateFormula("3!+4", GuideContext{})
	if got != 7 {
		t.Errorf("3!+4 = %v, want 7", got)
	}
}

func TestEvaluateFormulaTokenizeInvalidNumber(t *testing.T) {
	// ".." looks like a number but parseFloat fails; tokenizer treats as identifier
	_, err := EvaluateFormula("..", GuideContext{})
	if err == nil {
		t.Error("expected error for invalid number token")
	}
}
