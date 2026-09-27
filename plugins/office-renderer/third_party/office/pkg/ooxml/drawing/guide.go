package drawing

import (
	"fmt"
	"math"
	"strconv"
)

// Guide represents a DrawingML adjustment guide value.
// Guides can be fixed values or formulas.
type Guide struct {
	// Name is the guide identifier (e.g., "adj1", "adj2")
	Name string
	// Formula is the expression to evaluate (e.g., "val/2", "wd/2")
	Formula string
	// Value is the fixed value if Formula is empty
	Value float64
}

// GuideContext provides values for formula evaluation.
type GuideContext struct {
	// Shape dimensions
	Width, Height float64
	// Other guide values by name
	Guides map[string]float64
	// Constants
	Val float64 // The guide's own value (for formulas like "val/2")
}

// Evaluate computes the guide value in the given context.
func (g *Guide) Evaluate(ctx GuideContext) (float64, error) {
	if g.Formula == "" {
		return g.Value, nil
	}

	// Simple formula evaluation
	// Supported operations: +, -, *, /, parentheses, and references to wd, ht, adjN, val
	return evalFormula(g.Formula, ctx)
}

// EvaluateFormula evaluates a DrawingML guide formula with the provided context.
func EvaluateFormula(formula string, ctx GuideContext) (float64, error) {
	return evalFormula(formula, ctx)
}

// evalFormula evaluates a DrawingML guide formula using a proper parser.
func evalFormula(formula string, ctx GuideContext) (float64, error) {
	parser := newFormulaParser(formula, ctx)
	return parser.parseExpression()
}

// formulaParser implements a simple recursive descent parser for DrawingML formulas.
type formulaParser struct {
	tokens []token
	pos    int
	ctx    GuideContext
}

type token struct {
	typ tokenType
	val string
	num float64
}

type tokenType int

const (
	tokenNumber tokenType = iota
	tokenIdentifier
	tokenOperator
	tokenLParen
	tokenRParen
	tokenComma
	tokenEOF
)

// newFormulaParser creates a new parser for the given formula.
func newFormulaParser(formula string, ctx GuideContext) *formulaParser {
	tokens := tokenize(formula)
	return &formulaParser{tokens: tokens, ctx: ctx}
}

// tokenize converts a formula string into tokens.
func tokenize(formula string) []token {
	var tokens []token
	i := 0
	n := len(formula)

	for i < n {
		ch := formula[i]

		// Skip whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}

		// Numbers
		if (ch >= '0' && ch <= '9') || ch == '.' {
			start := i
			for i < n && ((formula[i] >= '0' && formula[i] <= '9') || formula[i] == '.') {
				i++
			}
			num, err := strconv.ParseFloat(formula[start:i], 64)
			if err != nil {
				// If parsing fails, treat as identifier
				tokens = append(tokens, token{typ: tokenIdentifier, val: formula[start:i]})
			} else {
				tokens = append(tokens, token{typ: tokenNumber, num: num})
			}
			continue
		}

		// Identifiers (letters and underscores)
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
			start := i
			for i < n && ((formula[i] >= 'a' && formula[i] <= 'z') ||
				(formula[i] >= 'A' && formula[i] <= 'Z') ||
				(formula[i] >= '0' && formula[i] <= '9') ||
				formula[i] == '_') {
				i++
			}
			tokens = append(tokens, token{typ: tokenIdentifier, val: formula[start:i]})
			continue
		}

		// Operators
		if ch == '+' || ch == '-' || ch == '*' || ch == '/' {
			tokens = append(tokens, token{typ: tokenOperator, val: string(ch)})
			i++
			continue
		}

		// Parentheses and comma
		if ch == '(' {
			tokens = append(tokens, token{typ: tokenLParen})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, token{typ: tokenRParen})
			i++
			continue
		}
		if ch == ',' {
			tokens = append(tokens, token{typ: tokenComma})
			i++
			continue
		}

		// Unknown character
		i++
	}

	tokens = append(tokens, token{typ: tokenEOF})
	return tokens
}

// current returns the current token.
func (p *formulaParser) current() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokenEOF}
	}
	return p.tokens[p.pos]
}

// advance moves to the next token.
func (p *formulaParser) advance() {
	p.pos++
}

// parseExpression parses an expression with operator precedence.
func (p *formulaParser) parseExpression() (float64, error) {
	return p.parseAddSub()
}

// parseAddSub parses addition and subtraction.
func (p *formulaParser) parseAddSub() (float64, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return 0, err
	}

	for {
		tok := p.current()
		if tok.typ != tokenOperator || (tok.val != "+" && tok.val != "-") {
			break
		}
		p.advance()

		right, err := p.parseMulDiv()
		if err != nil {
			return 0, err
		}

		if tok.val == "+" {
			left += right
		} else {
			left -= right
		}
	}

	return left, nil
}

// parseMulDiv parses multiplication and division.
func (p *formulaParser) parseMulDiv() (float64, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return 0, err
	}

	for {
		tok := p.current()
		if tok.typ != tokenOperator || (tok.val != "*" && tok.val != "/") {
			break
		}
		p.advance()

		right, err := p.parsePrimary()
		if err != nil {
			return 0, err
		}

		if tok.val == "*" {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}

	return left, nil
}

// parsePrimary parses primary expressions (numbers, identifiers, parentheses, function calls).
func (p *formulaParser) parsePrimary() (float64, error) {
	tok := p.current()

	switch tok.typ {
	case tokenNumber:
		p.advance()
		return tok.num, nil

	case tokenIdentifier:
		p.advance()
		return p.evalIdentifier(tok.val)

	case tokenLParen:
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		if p.current().typ != tokenRParen {
			return 0, fmt.Errorf("expected ')'")
		}
		p.advance()
		return val, nil

	case tokenOperator:
		// Handle unary plus/minus
		if tok.val == "+" || tok.val == "-" {
			p.advance()
			val, err := p.parsePrimary()
			if err != nil {
				return 0, err
			}
			if tok.val == "-" {
				return -val, nil
			}
			return val, nil
		}
		fallthrough

	default:
		return 0, fmt.Errorf("unexpected token: %v", tok)
	}
}

// evalIdentifier evaluates an identifier (variable or function call).
func (p *formulaParser) evalIdentifier(name string) (float64, error) {
	// Check if it's a function call
	if p.current().typ == tokenLParen {
		return p.evalFunction(name)
	}

	// Check for special variables and DrawingML named constants.
	switch name {
	case "wd", "w":
		return p.ctx.Width, nil
	case "ht", "h":
		return p.ctx.Height, nil
	case "val":
		return p.ctx.Val, nil
	case "ssd":
		if p.ctx.Width < p.ctx.Height {
			return p.ctx.Width, nil
		}
		return p.ctx.Height, nil
	case "lsd":
		if p.ctx.Width > p.ctx.Height {
			return p.ctx.Width, nil
		}
		return p.ctx.Height, nil
	case "t":
		return 0, nil
	case "b":
		return p.ctx.Height, nil
	case "l":
		return 0, nil
	case "r":
		return p.ctx.Width, nil
	case "cd2":
		return math.Pi / 2, nil
	case "cd4":
		return math.Pi / 4, nil
	}

	// Check for guide references (any name registered in the guide map)
	if val, ok := p.ctx.Guides[name]; ok {
		return val, nil
	}

	return 0, fmt.Errorf("unknown identifier: %s", name)
}

// evalFunction evaluates a function call.
func (p *formulaParser) evalFunction(name string) (float64, error) {
	p.advance() // consume '('

	// Parse arguments
	var args []float64
	for {
		if p.current().typ == tokenRParen {
			break
		}

		arg, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		args = append(args, arg)

		if p.current().typ == tokenComma {
			p.advance()
		} else if p.current().typ != tokenRParen {
			return 0, fmt.Errorf("expected ',' or ')' in function arguments")
		}
	}

	p.advance() // consume ')'

	// Evaluate function
	switch name {
	case "sin":
		if len(args) != 1 {
			return 0, fmt.Errorf("sin expects 1 argument")
		}
		return math.Sin(args[0] * math.Pi / 180), nil // Degrees to radians
	case "cos":
		if len(args) != 1 {
			return 0, fmt.Errorf("cos expects 1 argument")
		}
		return math.Cos(args[0] * math.Pi / 180), nil
	case "tan":
		if len(args) != 1 {
			return 0, fmt.Errorf("tan expects 1 argument")
		}
		return math.Tan(args[0] * math.Pi / 180), nil
	case "abs":
		if len(args) != 1 {
			return 0, fmt.Errorf("abs expects 1 argument")
		}
		return math.Abs(args[0]), nil
	case "sqrt":
		if len(args) != 1 {
			return 0, fmt.Errorf("sqrt expects 1 argument")
		}
		if args[0] < 0 {
			return 0, fmt.Errorf("sqrt of negative number")
		}
		return math.Sqrt(args[0]), nil
	case "max":
		if len(args) < 1 {
			return 0, fmt.Errorf("max expects at least 1 argument")
		}
		max := args[0]
		for _, arg := range args[1:] {
			if arg > max {
				max = arg
			}
		}
		return max, nil
	case "min":
		if len(args) < 1 {
			return 0, fmt.Errorf("min expects at least 1 argument")
		}
		min := args[0]
		for _, arg := range args[1:] {
			if arg < min {
				min = arg
			}
		}
		return min, nil
	case "pin":
		// pin(x, min, max) = clamp x between min and max
		if len(args) != 3 {
			return 0, fmt.Errorf("pin expects 3 arguments")
		}
		x, min, max := args[0], args[1], args[2]
		if x < min {
			return min, nil
		}
		if x > max {
			return max, nil
		}
		return x, nil
	case "mid":
		// mid(x, y, z) = median of three values
		if len(args) != 3 {
			return 0, fmt.Errorf("mid expects 3 arguments")
		}
		x, y, z := args[0], args[1], args[2]
		// Sort the three values and return the middle one
		if x > y {
			x, y = y, x
		}
		if y > z {
			y, z = z, y
		}
		if x > y {
			x, y = y, x
		}
		return y, nil
	case "at2":
		// ArcTan: at2(x, y) = atan2(y, x) in degrees
		if len(args) != 2 {
			return 0, fmt.Errorf("at2 expects 2 arguments")
		}
		return math.Atan2(args[1], args[0]) * 180 / math.Pi, nil
	case "cat2":
		// Cosine ArcTan: cat2(x, y, z) = x * cos(atan2(z, y))
		if len(args) != 3 {
			return 0, fmt.Errorf("cat2 expects 3 arguments")
		}
		return args[0] * math.Cos(math.Atan2(args[2], args[1])), nil
	case "sat2":
		// Sine ArcTan: sat2(x, y, z) = x * sin(atan2(z, y))
		if len(args) != 3 {
			return 0, fmt.Errorf("sat2 expects 3 arguments")
		}
		return args[0] * math.Sin(math.Atan2(args[2], args[1])), nil
	case "mod":
		// DrawingML "mod" is vector magnitude: sqrt(x² + y² + z²)
		if len(args) != 3 {
			return 0, fmt.Errorf("mod expects 3 arguments")
		}
		return math.Sqrt(args[0]*args[0] + args[1]*args[1] + args[2]*args[2]), nil
	case "if":
		// if(cond, t, f) = t if cond > 0, else f
		if len(args) != 3 {
			return 0, fmt.Errorf("if expects 3 arguments")
		}
		if args[0] > 0 {
			return args[1], nil
		}
		return args[2], nil
	case "prod":
		// prod(x, y, ...) = product of all arguments
		if len(args) < 1 {
			return 0, fmt.Errorf("prod expects at least 1 argument")
		}
		result := args[0]
		for _, arg := range args[1:] {
			result *= arg
		}
		return result, nil
	default:
		return 0, fmt.Errorf("unknown function: %s", name)
	}
}

// GuideEvaluator manages a collection of guides and evaluates them.
type GuideEvaluator struct {
	guides map[string]*Guide
}

// NewGuideEvaluator creates a new GuideEvaluator.
func NewGuideEvaluator() *GuideEvaluator {
	return &GuideEvaluator{
		guides: make(map[string]*Guide),
	}
}

// AddGuide adds a guide to the evaluator.
func (ge *GuideEvaluator) AddGuide(name string, g *Guide) {
	ge.guides[name] = g
}

// EvaluateAll evaluates all guides with the given shape dimensions.
func (ge *GuideEvaluator) EvaluateAll(width, height float64) (map[string]float64, error) {
	results := make(map[string]float64)

	// Build dependency graph and separate fixed values from formulas
	fixedGuides := make(map[string]*Guide)
	formulaGuides := make(map[string]*Guide)
	dependencies := make(map[string][]string) // guide name -> list of guides it depends on

	for name, guide := range ge.guides {
		if guide.Formula == "" {
			fixedGuides[name] = guide
		} else {
			formulaGuides[name] = guide
			// Parse formula to find dependencies
			deps := findDependencies(guide.Formula)
			dependencies[name] = deps
		}
	}

	// Evaluate fixed guides first
	for name, guide := range fixedGuides {
		val, _ := guide.Evaluate(GuideContext{Width: width, Height: height})
		results[name] = val
	}

	// Topologically sort formula guides based on dependencies
	sorted, err := topologicalSort(formulaGuides, dependencies, results)
	if err != nil {
		return nil, fmt.Errorf("error sorting guide dependencies: %v", err)
	}

	// Evaluate formulas in dependency order
	for _, name := range sorted {
		guide := formulaGuides[name]
		ctx := GuideContext{
			Width:  width,
			Height: height,
			Guides: results,
			Val:    guide.Value, // Use the guide's value for "val" references
		}
		val, err := guide.Evaluate(ctx)
		if err != nil {
			return nil, fmt.Errorf("error evaluating guide %q: %v", name, err)
		}
		results[name] = val
	}

	return results, nil
}

// GeometryType represents the type of geometry.
type GeometryType string

const (
	GeometryTypePreset GeometryType = "preset"
	GeometryTypeCustom GeometryType = "custom"
	GeometryTypeLine   GeometryType = "line"
)

// Geometry represents a DrawingML geometry with guides.
type Geometry struct {
	Type     GeometryType
	Name     string  // Preset shape name or custom geometry ID
	Guides   []Guide // Adjustment guides
	Handles  []AdjustHandle
	Paths    []Path // For custom geometry
	FillRule string // "nonzero" or "evenodd"
}

// AdjustHandle preserves a DrawingML adjustment handle from a:ahLst.
// Values are stored as raw guide/formula expressions so parser layers can keep
// authored semantics even when renderers do not yet expose interactive handles.
type AdjustHandle struct {
	Kind     string
	PosX     string
	PosY     string
	GDRefX   string
	MinX     string
	MaxX     string
	GDRefY   string
	MinY     string
	MaxY     string
	GDRefR   string
	MinR     string
	MaxR     string
	GDRefAng string
	MinAng   string
	MaxAng   string
}

// EvaluateGuides evaluates all guides in the geometry.
func (g *Geometry) EvaluateGuides(width, height float64) (map[string]float64, error) {
	evaluator := NewGuideEvaluator()
	for i, guide := range g.Guides {
		name := guide.Name
		if name == "" {
			name = fmt.Sprintf("adj%d", i+1)
		}
		evaluator.AddGuide(name, &guide)
	}
	return evaluator.EvaluateAll(width, height)
}

// NormalizePoint normalizes a point from shape coordinates (0-1) to absolute coordinates.
func NormalizePoint(p Point, width, height float64) Point {
	return Point{
		X: p.X * width,
		Y: p.Y * height,
	}
}

// NormalizePath normalizes all points in a path from shape coordinates to absolute coordinates.
func NormalizePath(path Path, width, height float64) Path {
	normalized := Path{
		Commands:      make([]PathCommand, len(path.Commands)),
		FillRule:      path.FillRule,
		PaintFlagsSet: path.PaintFlagsSet,
		FillEnabled:   path.FillEnabled,
		StrokeEnabled: path.StrokeEnabled,
	}

	for i, cmd := range path.Commands {
		normalizedCmd := PathCommand{
			Type: cmd.Type,
			Args: append([]float64(nil), cmd.Args...),
		}

		normalizedCmd.Points = make([]Point, len(cmd.Points))
		for j, pt := range cmd.Points {
			normalizedCmd.Points[j] = NormalizePoint(pt, width, height)
		}
		if cmd.Type == "A" && len(normalizedCmd.Args) >= 2 {
			normalizedCmd.Args[0] *= width
			normalizedCmd.Args[1] *= height
		}

		normalized.Commands[i] = normalizedCmd
	}

	return normalized
}

// findDependencies parses a formula and returns the list of guide names it references.
func findDependencies(formula string) []string {
	// Tokenize to find all identifier references. Exclude known constants
	// (wd, ht, val) and function names.
	var deps []string
	seen := make(map[string]bool)
	i := 0
	n := len(formula)

	for i < n {
		ch := formula[i]

		// Skip whitespace
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}

		// Check for identifier
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
			start := i
			i++
			for i < n && ((formula[i] >= 'a' && formula[i] <= 'z') ||
				(formula[i] >= 'A' && formula[i] <= 'Z') ||
				(formula[i] >= '0' && formula[i] <= '9') ||
				formula[i] == '_') {
				i++
			}
			ident := formula[start:i]

			// Skip known constants and numeric literals
			if ident == "wd" || ident == "ht" || ident == "val" {
				continue
			}
			// Skip if followed by '(' (function call)
			if i < n && formula[i] == '(' {
				continue
			}
			if !seen[ident] {
				seen[ident] = true
				deps = append(deps, ident)
			}
			continue
		}

		// Skip other characters
		i++
	}

	return deps
}

// topologicalSort performs a topological sort of formula guides based on dependencies.
func topologicalSort(formulaGuides map[string]*Guide, dependencies map[string][]string, results map[string]float64) ([]string, error) {
	// Kahn's algorithm for topological sorting
	inDegree := make(map[string]int)
	for name := range formulaGuides {
		inDegree[name] = 0
	}

	// Build adjacency list and calculate in-degrees
	adj := make(map[string][]string)
	for name, deps := range dependencies {
		for _, dep := range deps {
			// Only consider dependencies that are formula guides (not fixed values or wd/ht/val)
			if _, isFormula := formulaGuides[dep]; isFormula {
				adj[dep] = append(adj[dep], name)
				inDegree[name]++
			}
		}
	}

	// Initialize queue with nodes having zero in-degree
	queue := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	// Process queue
	sorted := make([]string, 0, len(formulaGuides))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		sorted = append(sorted, node)

		for _, neighbor := range adj[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// Check for cycles
	if len(sorted) != len(formulaGuides) {
		// Find nodes that weren't sorted (part of a cycle)
		unsorted := make([]string, 0)
		for name := range formulaGuides {
			found := false
			for _, sortedName := range sorted {
				if sortedName == name {
					found = true
					break
				}
			}
			if !found {
				unsorted = append(unsorted, name)
			}
		}
		return nil, fmt.Errorf("circular dependency detected among guides: %v", unsorted)
	}

	return sorted, nil
}
