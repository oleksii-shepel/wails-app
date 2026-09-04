package render

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// evaluateWordFormula evaluates a Word field formula expression.
// The instr includes the leading "=" (e.g. "= 2+2" or "=SUM(ABOVE)").
// It returns the computed value as a string, or empty string on failure.
func evaluateWordFormula(instr string) string {
	expr := strings.TrimSpace(instr)
	if strings.HasPrefix(expr, "=") {
		expr = strings.TrimSpace(expr[1:])
	}
	if expr == "" {
		return ""
	}
	p := &wordFormulaParser{s: expr, pos: 0}
	v, ok := p.parseExpression()
	if !ok || p.pos != len(p.s) {
		return ""
	}
	// Format integers without decimals, floats with reasonable precision.
	if v == math.Trunc(v) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// wordFormulaParser implements a simple recursive-descent parser for Word formulas.
type wordFormulaParser struct {
	s   string
	pos int
}

func (p *wordFormulaParser) peek() byte {
	if p.pos >= len(p.s) {
		return 0
	}
	return p.s[p.pos]
}

func (p *wordFormulaParser) consume() byte {
	ch := p.peek()
	p.pos++
	return ch
}

func (p *wordFormulaParser) skipSpaces() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

// parseExpression parses addition and subtraction.
func (p *wordFormulaParser) parseExpression() (float64, bool) {
	left, ok := p.parseTerm()
	if !ok {
		return 0, false
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.s) {
			return left, true
		}
		op := p.s[p.pos]
		if op != '+' && op != '-' {
			return left, true
		}
		p.pos++
		right, ok := p.parseTerm()
		if !ok {
			return 0, false
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
}

// parseTerm parses multiplication and division.
func (p *wordFormulaParser) parseTerm() (float64, bool) {
	left, ok := p.parseFactor()
	if !ok {
		return 0, false
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.s) {
			return left, true
		}
		op := p.s[p.pos]
		if op != '*' && op != '/' && op != '%' {
			return left, true
		}
		p.pos++
		right, ok := p.parseFactor()
		if !ok {
			return 0, false
		}
		switch op {
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, false
			}
			left /= right
		case '%':
			left = math.Mod(left, right)
		}
	}
}

// parseFactor parses exponentiation.
func (p *wordFormulaParser) parseFactor() (float64, bool) {
	left, ok := p.parseUnary()
	if !ok {
		return 0, false
	}
	for {
		p.skipSpaces()
		if p.pos >= len(p.s) || p.s[p.pos] != '^' {
			return left, true
		}
		p.pos++
		right, ok := p.parseUnary()
		if !ok {
			return 0, false
		}
		left = math.Pow(left, right)
	}
}

// parseUnary parses unary plus/minus.
func (p *wordFormulaParser) parseUnary() (float64, bool) {
	p.skipSpaces()
	if p.pos < len(p.s) {
		if p.s[p.pos] == '+' {
			p.pos++
			return p.parseUnary()
		}
		if p.s[p.pos] == '-' {
			p.pos++
			v, ok := p.parseUnary()
			return -v, ok
		}
	}
	return p.parsePrimary()
}

// parsePrimary parses numbers, functions, constants, and parenthesized expressions.
func (p *wordFormulaParser) parsePrimary() (float64, bool) {
	p.skipSpaces()
	if p.pos >= len(p.s) {
		return 0, false
	}

	// Parenthesized expression.
	if p.s[p.pos] == '(' {
		p.pos++
		v, ok := p.parseExpression()
		if !ok {
			return 0, false
		}
		p.skipSpaces()
		if p.pos >= len(p.s) || p.s[p.pos] != ')' {
			return 0, false
		}
		p.pos++
		return v, true
	}

	// Number.
	if isWordFormulaDigit(p.s[p.pos]) || (p.s[p.pos] == '.' && p.pos+1 < len(p.s) && isWordFormulaDigit(p.s[p.pos+1])) {
		start := p.pos
		for p.pos < len(p.s) && isWordFormulaDigit(p.s[p.pos]) {
			p.pos++
		}
		if p.pos < len(p.s) && p.s[p.pos] == '.' {
			p.pos++
			for p.pos < len(p.s) && isWordFormulaDigit(p.s[p.pos]) {
				p.pos++
			}
		}
		numStr := p.s[start:p.pos]
		v, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}

	// Identifier (function or constant).
	if isWordFormulaIdentStart(p.s[p.pos]) {
		start := p.pos
		for p.pos < len(p.s) && isWordFormulaIdentChar(p.s[p.pos]) {
			p.pos++
		}
		name := strings.ToUpper(p.s[start:p.pos])
		p.skipSpaces()
		if p.pos < len(p.s) && p.s[p.pos] == '(' {
			return p.parseFunction(name)
		}
		return p.parseConstant(name)
	}

	return 0, false
}

func (p *wordFormulaParser) parseFunction(name string) (float64, bool) {
	// Consume '('.
	p.pos++
	var args []float64
	p.skipSpaces()
	if p.pos < len(p.s) && p.s[p.pos] == ')' {
		p.pos++
	} else {
		for {
			v, ok := p.parseExpression()
			if !ok {
				return 0, false
			}
			args = append(args, v)
			p.skipSpaces()
			if p.pos >= len(p.s) {
				return 0, false
			}
			if p.s[p.pos] == ',' {
				p.pos++
				continue
			}
			if p.s[p.pos] == ')' {
				p.pos++
				break
			}
			return 0, false
		}
	}
	return evaluateWordFormulaFunction(name, args)
}

func (p *wordFormulaParser) parseConstant(name string) (float64, bool) {
	switch name {
	case "TRUE":
		return 1, true
	case "FALSE":
		return 0, true
	case "ABOVE", "BELOW", "LEFT", "RIGHT":
		// Without table-cell context, positional references return 0.
		return 0, true
	}
	return 0, false
}

func evaluateWordFormulaFunction(name string, args []float64) (float64, bool) {
	switch name {
	case "SUM", "PRODUCT":
		if len(args) == 0 {
			return 0, true
		}
		result := args[0]
		for i := 1; i < len(args); i++ {
			if name == "SUM" {
				result += args[i]
			} else {
				result *= args[i]
			}
		}
		return result, true
	case "AVERAGE":
		if len(args) == 0 {
			return 0, true
		}
		var sum float64
		for _, v := range args {
			sum += v
		}
		return sum / float64(len(args)), true
	case "MIN":
		if len(args) == 0 {
			return 0, true
		}
		result := args[0]
		for i := 1; i < len(args); i++ {
			if args[i] < result {
				result = args[i]
			}
		}
		return result, true
	case "MAX":
		if len(args) == 0 {
			return 0, true
		}
		result := args[0]
		for i := 1; i < len(args); i++ {
			if args[i] > result {
				result = args[i]
			}
		}
		return result, true
	case "COUNT":
		return float64(len(args)), true
	case "ABS":
		if len(args) < 1 {
			return 0, false
		}
		return math.Abs(args[0]), true
	case "INT":
		if len(args) < 1 {
			return 0, false
		}
		return math.Trunc(args[0]), true
	case "ROUND":
		if len(args) < 1 {
			return 0, false
		}
		decimals := 0.0
		if len(args) >= 2 {
			decimals = args[1]
		}
		factor := math.Pow(10, decimals)
		return math.Round(args[0]*factor) / factor, true
	}
	return 0, false
}

func isWordFormulaDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isWordFormulaIdentStart(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || ch == '_'
}

func isWordFormulaIdentChar(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_'
}

// evaluateWordFormulaFull is the top-level evaluator used by htmlRenderer.
// It preserves cached text when present and evaluates the formula otherwise.
func evaluateWordFormulaFull(instr, cachedText string) string {
	if strings.TrimSpace(cachedText) != "" {
		return ""
	}
	return evaluateWordFormula(instr)
}