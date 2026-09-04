package formula

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Token represents a lexical token in a formula.
type Token struct {
	Type  TokenType
	Value string
}

// TokenType identifies the kind of token.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenNumber
	TokenString
	TokenError
	TokenIdentifier
	TokenCellRef
	TokenOperator
	TokenLParen
	TokenRParen
	TokenComma
	TokenColon
	TokenSemicolon
	TokenLBrace
	TokenRBrace
	TokenAt
	TokenHash
)

// String returns a human-readable token type name.
func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenNumber:
		return "NUMBER"
	case TokenString:
		return "STRING"
	case TokenError:
		return "ERROR"
	case TokenIdentifier:
		return "IDENTIFIER"
	case TokenCellRef:
		return "CELLREF"
	case TokenOperator:
		return "OPERATOR"
	case TokenLParen:
		return "LPAREN"
	case TokenRParen:
		return "RPAREN"
	case TokenComma:
		return "COMMA"
	case TokenColon:
		return "COLON"
	case TokenSemicolon:
		return "SEMICOLON"
	case TokenLBrace:
		return "LBRACE"
	case TokenRBrace:
		return "RBRACE"
	case TokenAt:
		return "AT"
	case TokenHash:
		return "HASH"
	default:
		return "UNKNOWN"
	}
}

// Lexer tokenizes formula strings.
type Lexer struct {
	input string
	pos   int
	ch    byte
}

// NewLexer creates a new lexer for the given input.
func NewLexer(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.pos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.pos]
	}
	l.pos++
}

func (l *Lexer) peekChar() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	switch l.ch {
	case 0:
		return Token{Type: TokenEOF}
	case '(':
		l.readChar()
		return Token{Type: TokenLParen, Value: "("}
	case ')':
		l.readChar()
		return Token{Type: TokenRParen, Value: ")"}
	case ',':
		l.readChar()
		return Token{Type: TokenComma, Value: ","}
	case ';':
		l.readChar()
		return Token{Type: TokenSemicolon, Value: ";"}
	case ':':
		l.readChar()
		return Token{Type: TokenColon, Value: ":"}
	case '"':
		return l.readString()
	case '{':
		l.readChar()
		return Token{Type: TokenLBrace, Value: "{"}
	case '}':
		l.readChar()
		return Token{Type: TokenRBrace, Value: "}"}
	case '@':
		l.readChar()
		return Token{Type: TokenAt, Value: "@"}
	case '#':
		return l.readErrorOrHash()
	case '[':
		if tok, ok := l.readBracketedWorkbookQualifiedRef(); ok {
			return tok
		}
		l.readChar()
		return l.NextToken()
	case '\'':
		if tok, ok := l.readQuotedSheetQualifiedRef(); ok {
			return tok
		}
		l.readChar()
		return l.NextToken()
	case '+', '-', '*', '/', '^', '%', '=', '<', '>', '&':
		return l.readOperator()
	default:
		if isDigit(l.ch) || (l.ch == '.' && isDigit(l.peekChar())) {
			return l.readNumber()
		}
		if isLetter(l.ch) || l.ch == '$' || l.ch == '_' || l.ch == '\\' {
			return l.readIdentifierOrCellRef()
		}
		// Skip unknown character
		l.readChar()
		return l.NextToken()
	}
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readString() Token {
	var sb strings.Builder
	l.readChar() // skip opening quote
	for l.ch != '"' && l.ch != 0 {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	l.readChar() // skip closing quote
	return Token{Type: TokenString, Value: sb.String()}
}

func (l *Lexer) readErrorOrHash() Token {
	// Save state for potential backtrack.
	savedPos := l.pos
	savedCh := l.ch

	var sb strings.Builder
	sb.WriteByte(l.ch)
	l.readChar()
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '/' || l.ch == '!' || l.ch == '?' || l.ch == '_' || l.ch == '.' {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	val := sb.String()
	if ParseError(val) != ErrNone {
		return Token{Type: TokenError, Value: val}
	}
	// Not a recognised error literal; backtrack to just after '#'.
	l.pos = savedPos
	l.ch = savedCh
	l.readChar()
	return Token{Type: TokenHash, Value: "#"}
}

func (l *Lexer) readNumber() Token {
	var sb strings.Builder
	hasDot := false

	for isDigit(l.ch) || (l.ch == '.' && !hasDot) {
		if l.ch == '.' {
			hasDot = true
		}
		sb.WriteByte(l.ch)
		l.readChar()
	}

	// Handle scientific notation
	if l.ch == 'e' || l.ch == 'E' {
		sb.WriteByte(l.ch)
		l.readChar()
		if l.ch == '+' || l.ch == '-' {
			sb.WriteByte(l.ch)
			l.readChar()
		}
		for isDigit(l.ch) {
			sb.WriteByte(l.ch)
			l.readChar()
		}
	}

	return Token{Type: TokenNumber, Value: sb.String()}
}

func (l *Lexer) readOperator() Token {
	var sb strings.Builder

	// Multi-character operators
	if l.ch == '<' {
		sb.WriteByte(l.ch)
		l.readChar()
		if l.ch == '=' || l.ch == '>' {
			sb.WriteByte(l.ch)
			l.readChar()
		}
		return Token{Type: TokenOperator, Value: sb.String()}
	}
	if l.ch == '>' {
		sb.WriteByte(l.ch)
		l.readChar()
		if l.ch == '=' {
			sb.WriteByte(l.ch)
			l.readChar()
		}
		return Token{Type: TokenOperator, Value: sb.String()}
	}

	// Single character operators
	sb.WriteByte(l.ch)
	l.readChar()
	return Token{Type: TokenOperator, Value: sb.String()}
}

func (l *Lexer) readIdentifierOrCellRef() Token {
	var sb strings.Builder
	hasDollar := false

	// Check for absolute reference
	if l.ch == '$' {
		hasDollar = true
		sb.WriteByte(l.ch)
		l.readChar()
	}

	// Read letters (column part)
	startedWithLetter := false
	for isLetter(l.ch) {
		startedWithLetter = true
		sb.WriteByte(l.ch)
		l.readChar()
	}

	// If we have $ but no letters, this is just an identifier starting with $
	if hasDollar && !startedWithLetter {
		// Continue reading as identifier
		for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
			sb.WriteByte(l.ch)
			l.readChar()
		}
		return Token{Type: TokenIdentifier, Value: sb.String()}
	}

	// Check for row number (indicates cell reference)
	if startedWithLetter && (isDigit(l.ch) || l.ch == '$') {
		// Allow absolute row reference: A$1
		if l.ch == '$' {
			sb.WriteByte(l.ch)
			l.readChar()
		}
		if !isDigit(l.ch) {
			// $ not followed by digit — fall through to identifier
			for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '.' {
				sb.WriteByte(l.ch)
				l.readChar()
			}
			return Token{Type: TokenIdentifier, Value: sb.String()}
		}
		// Read the digit sequence
		for isDigit(l.ch) {
			sb.WriteByte(l.ch)
			l.readChar()
		}
		// If more letters follow (e.g. SUMX2MY2, SUMX2PY2), this is an identifier, not a cell ref.
		if isLetter(l.ch) {
			for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '.' {
				sb.WriteByte(l.ch)
				l.readChar()
			}
			return Token{Type: TokenIdentifier, Value: sb.String()}
		}
		if l.ch == '!' {
			if tok, ok := l.readSheetQualifiedRef(sb.String()); ok {
				return tok
			}
		}
		return Token{Type: TokenCellRef, Value: sb.String()}
	}

	if l.ch == '!' {
		if tok, ok := l.readSheetQualifiedRef(sb.String()); ok {
			return tok
		}
	}

	// Otherwise, it's an identifier (function name, named range, etc.)
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '.' {
		sb.WriteByte(l.ch)
		l.readChar()
	}

	return Token{Type: TokenIdentifier, Value: sb.String()}
}

func (l *Lexer) readSheetQualifiedRef(prefix string) (Token, bool) {
	if l.ch != '!' {
		return Token{}, false
	}
	l.readChar()
	ref, ok := l.readCellRefLiteral()
	if !ok {
		return Token{}, false
	}
	return Token{Type: TokenCellRef, Value: prefix + "!" + ref}, true
}

func (l *Lexer) readQuotedSheetQualifiedRef() (Token, bool) {
	if l.ch != '\'' {
		return Token{}, false
	}
	var sb strings.Builder
	sb.WriteByte(l.ch)
	l.readChar()
	for l.ch != 0 {
		sb.WriteByte(l.ch)
		if l.ch == '\'' {
			l.readChar()
			if l.ch == '\'' {
				sb.WriteByte(l.ch)
				l.readChar()
				continue
			}
			break
		}
		l.readChar()
	}
	if l.ch != '!' {
		return Token{}, false
	}
	prefix := sb.String()
	l.readChar()
	ref, ok := l.readCellRefLiteral()
	if !ok {
		return Token{}, false
	}
	return Token{Type: TokenCellRef, Value: prefix + "!" + ref}, true
}

func (l *Lexer) readBracketedWorkbookQualifiedRef() (Token, bool) {
	if l.ch != '[' {
		return Token{}, false
	}
	var sb strings.Builder
	sb.WriteByte(l.ch)
	l.readChar()
	for l.ch != 0 && l.ch != ']' {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	if l.ch != ']' {
		return Token{}, false
	}
	sb.WriteByte(l.ch)
	l.readChar()
	sheetStart := sb.Len()
	for isExternalSheetNameChar(l.ch) {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	if sb.Len() == sheetStart || l.ch != '!' {
		return Token{}, false
	}
	prefix := sb.String()
	l.readChar()
	ref, ok := l.readCellRefLiteral()
	if !ok {
		return Token{}, false
	}
	return Token{Type: TokenCellRef, Value: prefix + "!" + ref}, true
}

func isExternalSheetNameChar(ch byte) bool {
	return isLetter(ch) || isDigit(ch) || ch == '_' || ch == '.' || ch == '\\' || ch == '-'
}

func (l *Lexer) readCellRefLiteral() (string, bool) {
	var sb strings.Builder
	if l.ch == '$' {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	colStart := sb.Len()
	for isLetter(l.ch) {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	if sb.Len() == colStart {
		return "", false
	}
	if l.ch == '$' {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	rowStart := sb.Len()
	for isDigit(l.ch) {
		sb.WriteByte(l.ch)
		l.readChar()
	}
	if sb.Len() == rowStart {
		return "", false
	}
	return sb.String(), true
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

// Parser converts tokens into an AST.
type Parser struct {
	lexer     *Lexer
	curToken  Token
	peekToken Token
}

// NewParser creates a new parser for the given input.
func NewParser(input string) *Parser {
	l := NewLexer(input)
	p := &Parser{lexer: l}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

// Parse parses the formula and returns an expression.
func (p *Parser) Parse() (Expression, error) {
	return p.parseExpression(0)
}

// Expression represents a parsed formula expression.
type Expression interface {
	Eval(ctx Context) Value
	String() string
}

// Context provides the evaluation context for expressions.
type Context interface {
	// GetCell returns the value of a cell.
	GetCell(ref string) Value
	// GetRange returns all values in a cell range.
	GetRange(start, end string) []Value
	// CurrentRef returns the cell currently being evaluated, if any.
	CurrentRef() string
	// ArrayAnchorRef returns the anchor cell for the current array formula, if any.
	ArrayAnchorRef() string
	// GetFormula returns the stored formula text for a referenced cell, if any.
	GetFormula(ref string) (string, bool)
	// ResolveSheetOrdinal resolves a sheet name or qualified reference to a sheet index.
	ResolveSheetOrdinal(token string) (int, bool)
	// SheetCount returns the number of sheets visible to the current context.
	SheetCount() int
	// GetSpillRange returns the start and end refs of the spilled array formula
	// at the given anchor cell, if one exists.
	GetSpillRange(ref string) (start, end string, ok bool)
}

type bindingResolver interface {
	ResolveScalarBinding(ref string) (Value, bool)
	ResolveArrayBinding(ref string) ([]Value, arrayShape, bool)
}

// BinaryExpr represents a binary operation.
type BinaryExpr struct {
	Left  Expression
	Op    string
	Right Expression
}

func (e *BinaryExpr) Eval(ctx Context) Value {
	leftVal := e.Left.Eval(ctx)
	if leftVal.IsError() {
		return leftVal
	}

	rightVal := e.Right.Eval(ctx)
	if rightVal.IsError() {
		return rightVal
	}

	return evalBinaryOp(leftVal, e.Op, rightVal)
}

func (e *BinaryExpr) String() string {
	return fmt.Sprintf("(%s %s %s)", e.Left.String(), e.Op, e.Right.String())
}

// UnaryExpr represents a unary operation.
type UnaryExpr struct {
	Op   string
	Expr Expression
}

func (e *UnaryExpr) Eval(ctx Context) Value {
	val := e.Expr.Eval(ctx)
	if val.IsError() {
		return val
	}

	switch e.Op {
	case "+":
		num, err := val.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		return Number(num)
	case "-":
		num, err := val.ToNumber()
		if err != nil {
			return Error(ErrValue)
		}
		return Number(-num)
	default:
		return Error(ErrValue)
	}
}

func (e *UnaryExpr) String() string {
	return fmt.Sprintf("%s%s", e.Op, e.Expr.String())
}

// NumberExpr represents a numeric literal.
type NumberExpr struct {
	Value float64
}

func (e *NumberExpr) Eval(ctx Context) Value {
	return Number(e.Value)
}

func (e *NumberExpr) String() string {
	return fmt.Sprintf("%g", e.Value)
}

// StringExpr represents a string literal.
type StringExpr struct {
	Value string
}

func (e *StringExpr) Eval(ctx Context) Value {
	return Text(e.Value)
}

func (e *StringExpr) String() string {
	return fmt.Sprintf("\"%s\"", e.Value)
}

// ErrorExpr represents an Excel error literal.
type ErrorExpr struct {
	Err ErrorType
}

func (e *ErrorExpr) Eval(ctx Context) Value {
	return Error(e.Err)
}

func (e *ErrorExpr) String() string {
	return e.Err.String()
}

// CellRefExpr represents a cell reference.
type CellRefExpr struct {
	Ref string
}

func (e *CellRefExpr) Eval(ctx Context) Value {
	if resolver, ok := ctx.(bindingResolver); ok {
		if value, ok := resolver.ResolveScalarBinding(e.Ref); ok {
			return value
		}
	}
	if strings.EqualFold(e.Ref, "TRUE") {
		return Bool(true)
	}
	if strings.EqualFold(e.Ref, "FALSE") {
		return Bool(false)
	}
	return ctx.GetCell(e.Ref)
}

func (e *CellRefExpr) String() string {
	return e.Ref
}

// FuncCallExpr represents a function call.
type FuncCallExpr struct {
	Name string
	Args []Expression
}

func (e *FuncCallExpr) Eval(ctx Context) Value {
	return evalFunction(e.Name, e.Args, ctx)
}

func (e *FuncCallExpr) String() string {
	args := make([]string, len(e.Args))
	for i, arg := range e.Args {
		args[i] = arg.String()
	}
	return fmt.Sprintf("%s(%s)", e.Name, strings.Join(args, ", "))
}

// ApplyExpr represents a postfix call on an arbitrary expression.
type ApplyExpr struct {
	Callee Expression
	Args   []Expression
}

func (e *ApplyExpr) Eval(ctx Context) Value {
	return evalApplyExpression(e.Callee, e.Args, ctx)
}

func (e *ApplyExpr) String() string {
	args := make([]string, len(e.Args))
	for i, arg := range e.Args {
		args[i] = arg.String()
	}
	return fmt.Sprintf("%s(%s)", e.Callee.String(), strings.Join(args, ", "))
}

// RangeExpr represents a cell range (e.g., A1:B10).
type RangeExpr struct {
	Start string
	End   string
}

// ArrayLiteralExpr represents an array constant literal (e.g., {1,2;3,4}).
type ArrayLiteralExpr struct {
	Rows [][]Expression
}

// ImplicitIntersectionExpr forces scalar implicit intersection on a range.
type ImplicitIntersectionExpr struct {
	Expr Expression
}

// SpillRangeExpr represents a spilled range reference (e.g., A1#).
// Evaluation requires engine support for looking up the inferred spill shape.
type SpillRangeExpr struct {
	AnchorRef string
}

func (e *RangeExpr) Eval(ctx Context) Value {
	if ctx.ArrayAnchorRef() == "" {
		return Error(ErrValue)
	}
	targetRef, ok := arrayRangeTargetRef(e.Start, e.End, ctx)
	if !ok || targetRef == "" {
		return Error(ErrNA)
	}
	return ctx.GetCell(targetRef)
}

func (e *RangeExpr) String() string {
	return fmt.Sprintf("%s:%s", e.Start, e.End)
}

func (e *ArrayLiteralExpr) Eval(ctx Context) Value {
	if ctx.ArrayAnchorRef() == "" {
		return Error(ErrValue)
	}
	rowOff, colOff, ok := currentArrayOffsets(ctx)
	if !ok {
		return Error(ErrValue)
	}
	if rowOff < 0 || rowOff >= len(e.Rows) {
		return Error(ErrValue)
	}
	row := e.Rows[rowOff]
	if colOff < 0 || colOff >= len(row) {
		return Error(ErrValue)
	}
	return row[colOff].Eval(ctx)
}

func (e *ArrayLiteralExpr) String() string {
	var sb strings.Builder
	sb.WriteString("{")
	for i, row := range e.Rows {
		if i > 0 {
			sb.WriteString(";")
		}
		for j, expr := range row {
			if j > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(expr.String())
		}
	}
	sb.WriteString("}")
	return sb.String()
}

func (e *ImplicitIntersectionExpr) Eval(ctx Context) Value {
	switch expr := e.Expr.(type) {
	case *RangeExpr:
		currentRow, currentCol := parseCellRef(ctx.CurrentRef())
		if currentRow == 0 {
			return Error(ErrValue)
		}
		startRow, startCol := parseCellRef(expr.Start)
		endRow, endCol := parseCellRef(expr.End)
		if startRow == 0 || endRow == 0 {
			return Error(ErrValue)
		}
		// Single column range: intersect by row.
		if startCol == endCol {
			if currentRow >= startRow && currentRow <= endRow {
				ref := fmt.Sprintf("%s%d", colToLetters(startCol), currentRow)
				return ctx.GetCell(ref)
			}
			return Error(ErrValue)
		}
		// Single row range: intersect by column.
		if startRow == endRow {
			if currentCol >= startCol && currentCol <= endCol {
				ref := fmt.Sprintf("%s%d", colToLetters(currentCol), startRow)
				return ctx.GetCell(ref)
			}
			return Error(ErrValue)
		}
		// 2D range: intersect by row and column.
		if currentRow >= startRow && currentRow <= endRow && currentCol >= startCol && currentCol <= endCol {
			ref := fmt.Sprintf("%s%d", colToLetters(currentCol), currentRow)
			return ctx.GetCell(ref)
		}
		return Error(ErrValue)
	case *CellRefExpr:
		return expr.Eval(ctx)
	default:
		return e.Expr.Eval(ctx)
	}
}

func (e *ImplicitIntersectionExpr) String() string {
	return "@" + e.Expr.String()
}

func (e *SpillRangeExpr) Eval(ctx Context) Value {
	start, end, ok := ctx.GetSpillRange(e.AnchorRef)
	if !ok {
		return Error(ErrValue)
	}
	if ctx.ArrayAnchorRef() == "" {
		return Error(ErrValue)
	}
	targetRef, ok := arrayRangeTargetRef(start, end, ctx)
	if !ok || targetRef == "" {
		return Error(ErrNA)
	}
	return ctx.GetCell(targetRef)
}

func (e *SpillRangeExpr) String() string {
	return e.AnchorRef + "#"
}

// precedence returns the precedence of an operator.
func precedence(op string) int {
	switch op {
	case "^":
		return 4
	case "*", "/":
		return 3
	case "+", "-", "&":
		return 2
	case "=", "<>", "<", ">", "<=", ">=":
		return 1
	default:
		return 0
	}
}

// isRightAssociative returns true if the operator is right-associative.
func isRightAssociative(op string) bool {
	return op == "^"
}

func (p *Parser) parseExpression(minPrec int) (Expression, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		if p.curToken.Type != TokenOperator {
			break
		}

		op := p.curToken.Value
		prec := precedence(op)
		if prec < minPrec {
			break
		}

		p.nextToken()

		nextMinPrec := prec
		if !isRightAssociative(op) {
			nextMinPrec++
		}

		right, err := p.parseExpression(nextMinPrec)
		if err != nil {
			return nil, err
		}

		left = &BinaryExpr{Left: left, Op: op, Right: right}
	}

	return left, nil
}

func (p *Parser) parsePrimary() (Expression, error) {
	expr, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	for {
		if p.curToken.Type == TokenLParen {
			args, err := p.parseCallArguments()
			if err != nil {
				return nil, err
			}
			expr = &ApplyExpr{Callee: expr, Args: args}
		} else if p.curToken.Type == TokenOperator && p.curToken.Value == "%" {
			// Postfix percentage: expr% == expr/100
			p.nextToken()
			expr = &BinaryExpr{Left: expr, Op: "/", Right: &NumberExpr{Value: 100}}
		} else {
			break
		}
	}
	return expr, nil
}

func (p *Parser) parseArrayLiteral() (Expression, error) {
	p.nextToken() // consume '{'
	var rows [][]Expression
	var currentRow []Expression
	for {
		if p.curToken.Type == TokenRBrace {
			p.nextToken()
			if len(currentRow) > 0 {
				rows = append(rows, currentRow)
			}
			return &ArrayLiteralExpr{Rows: rows}, nil
		}
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		currentRow = append(currentRow, expr)
		switch p.curToken.Type {
		case TokenComma:
			p.nextToken()
			continue
		case TokenSemicolon:
			p.nextToken()
			rows = append(rows, currentRow)
			currentRow = nil
			continue
		case TokenRBrace:
			p.nextToken()
			rows = append(rows, currentRow)
			return &ArrayLiteralExpr{Rows: rows}, nil
		default:
			return nil, fmt.Errorf("expected , ; or } in array literal, got %s", p.curToken.Value)
		}
	}
}

func (p *Parser) parseAtom() (Expression, error) {
	switch p.curToken.Type {
	case TokenNumber:
		val, err := strconv.ParseFloat(p.curToken.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number: %s", p.curToken.Value)
		}
		expr := &NumberExpr{Value: val}
		p.nextToken()
		return expr, nil

	case TokenString:
		expr := &StringExpr{Value: p.curToken.Value}
		p.nextToken()
		return expr, nil

	case TokenError:
		errType := ParseError(p.curToken.Value)
		if errType == ErrNone {
			return nil, fmt.Errorf("invalid error literal: %s", p.curToken.Value)
		}
		expr := &ErrorExpr{Err: errType}
		p.nextToken()
		return expr, nil

	case TokenCellRef:
		ref := p.curToken.Value
		p.nextToken()
		if p.curToken.Type == TokenHash {
			p.nextToken()
			return &SpillRangeExpr{AnchorRef: ref}, nil
		}
		if p.curToken.Type == TokenLParen && isAmbiguousCellLikeFunction(ref) {
			return p.parseFuncCall(ref)
		}
		if p.curToken.Type == TokenColon {
			p.nextToken()
			switch p.curToken.Type {
			case TokenCellRef, TokenIdentifier:
				end := p.curToken.Value
				p.nextToken()
				return &RangeExpr{Start: ref, End: end}, nil
			default:
				return nil, fmt.Errorf("expected range end")
			}
		}
		return &CellRefExpr{Ref: ref}, nil

	case TokenIdentifier:
		name := p.curToken.Value
		p.nextToken()
		if p.curToken.Type == TokenLParen {
			return p.parseFuncCall(name)
		}
		if p.curToken.Type == TokenColon {
			p.nextToken()
			switch p.curToken.Type {
			case TokenCellRef, TokenIdentifier:
				end := p.curToken.Value
				p.nextToken()
				if start, ok := expandThreeDSheetSpanRef(name, end); ok {
					return &RangeExpr{Start: start, End: end}, nil
				}
				return &RangeExpr{Start: name, End: end}, nil
			default:
				return nil, fmt.Errorf("expected range end")
			}
		}
		// Named reference or function without parens
		return &CellRefExpr{Ref: name}, nil

	case TokenOperator:
		if p.curToken.Value == "+" || p.curToken.Value == "-" {
			op := p.curToken.Value
			p.nextToken()
			expr, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			return &UnaryExpr{Op: op, Expr: expr}, nil
		}

	case TokenLParen:
		p.nextToken()
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		if p.curToken.Type != TokenRParen {
			return nil, fmt.Errorf("expected )")
		}
		p.nextToken()
		return expr, nil
	case TokenLBrace:
		return p.parseArrayLiteral()
	case TokenAt:
		p.nextToken()
		expr, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		return &ImplicitIntersectionExpr{Expr: expr}, nil
	}

	return nil, fmt.Errorf("unexpected token: %s", p.curToken.Value)
}

func expandThreeDSheetSpanRef(startSheet, end string) (string, bool) {
	if startSheet == "" || strings.LastIndex(startSheet, "!") >= 0 {
		return "", false
	}
	idx := strings.LastIndex(end, "!")
	if idx < 0 || idx == len(end)-1 {
		return "", false
	}
	cellRef := end[idx+1:]
	if !isCellRef(cellRef) {
		return "", false
	}
	return startSheet + "!" + cellRef, true
}

func isAmbiguousCellLikeFunction(name string) bool {
	switch strings.ToUpper(name) {
	case "LOG10", "ATAN2", "SUMXMY2":
		return true
	default:
		return false
	}
}

func (p *Parser) parseFuncCall(name string) (Expression, error) {
	args, err := p.parseCallArguments()
	if err != nil {
		return nil, err
	}
	return &FuncCallExpr{Name: strings.ToUpper(name), Args: args}, nil
}

func (p *Parser) parseCallArguments() ([]Expression, error) {
	p.nextToken() // consume (

	var args []Expression
	if p.curToken.Type != TokenRParen {
		for {
			arg, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)

			if p.curToken.Type == TokenComma || p.curToken.Type == TokenSemicolon {
				p.nextToken()
				continue
			}

			break
		}
	}

	if p.curToken.Type != TokenRParen {
		return nil, fmt.Errorf("expected )")
	}
	p.nextToken()

	return args, nil
}

func evalBinaryOp(left Value, op string, right Value) Value {
	switch op {
	case "&":
		return concatValues(left, right)
	case "+":
		return addValues(left, right)
	case "-":
		return subtractValues(left, right)
	case "*":
		return multiplyValues(left, right)
	case "/":
		return divideValues(left, right)
	case "^":
		return powerValues(left, right)
	case "=":
		return compareValues(left, "=", right)
	case "<>":
		return compareValues(left, "<>", right)
	case "<":
		return compareValues(left, "<", right)
	case ">":
		return compareValues(left, ">", right)
	case "<=":
		return compareValues(left, "<=", right)
	case ">=":
		return compareValues(left, ">=", right)
	default:
		return Error(ErrValue)
	}
}

func concatValues(left, right Value) Value {
	if left.IsError() {
		return left
	}
	if right.IsError() {
		return right
	}
	return Text(left.ToText() + right.ToText())
}

func addValues(left, right Value) Value {
	leftNum, err := left.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	rightNum, err := right.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(leftNum + rightNum)
}

func subtractValues(left, right Value) Value {
	leftNum, err := left.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	rightNum, err := right.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(leftNum - rightNum)
}

func multiplyValues(left, right Value) Value {
	leftNum, err := left.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	rightNum, err := right.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	return Number(leftNum * rightNum)
}

func divideValues(left, right Value) Value {
	leftNum, err := left.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	rightNum, err := right.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if rightNum == 0 {
		return Error(ErrDiv0)
	}
	return Number(leftNum / rightNum)
}

func powerValues(left, right Value) Value {
	leftNum, err := left.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	rightNum, err := right.ToNumber()
	if err != nil {
		return Error(ErrValue)
	}
	if leftNum == 0 && rightNum < 0 {
		return Error(ErrDiv0)
	}
	return Number(math.Pow(leftNum, rightNum))
}

func compareValues(left Value, op string, right Value) Value {
	switch op {
	case "=":
		return Bool(valuesEqual(left, right))
	case "<>":
		return Bool(!valuesEqual(left, right))
	case "<":
		if result, ok := valueLessThanOrdered(left, right); ok {
			return Bool(result)
		}
		return Error(ErrValue)
	case ">":
		if result, ok := valueLessThanOrdered(right, left); ok {
			return Bool(result)
		}
		return Error(ErrValue)
	case "<=":
		eq := valuesEqual(left, right)
		if lt, ok := valueLessThanOrdered(left, right); ok {
			return Bool(eq || lt)
		}
		return Error(ErrValue)
	case ">=":
		eq := valuesEqual(left, right)
		if lt, ok := valueLessThanOrdered(right, left); ok {
			return Bool(eq || lt)
		}
		return Error(ErrValue)
	default:
		return Error(ErrValue)
	}
}

func valuesEqual(left, right Value) bool {
	if left.Type != right.Type {
		// Try numeric comparison
		leftNum, leftErr := left.ToNumber()
		rightNum, rightErr := right.ToNumber()
		if leftErr == nil && rightErr == nil {
			return leftNum == rightNum
		}
		return false
	}

	switch left.Type {
	case TypeNumber:
		return left.Num == right.Num
	case TypeText:
		return left.Text == right.Text
	case TypeBool:
		return left.Bool == right.Bool
	case TypeError:
		return left.Err == right.Err
	case TypeEmpty:
		return true
	default:
		return false
	}
}

func valueLessThan(left, right Value) bool {
	// Try numeric comparison first
	leftNum, leftErr := left.ToNumber()
	rightNum, rightErr := right.ToNumber()
	if leftErr == nil && rightErr == nil {
		return leftNum < rightNum
	}

	// String comparison
	if left.Type == TypeText && right.Type == TypeText {
		return left.Text < right.Text
	}

	return false
}

func valueLessThanOrdered(left, right Value) (bool, bool) {
	// Try numeric comparison first
	leftNum, leftErr := left.ToNumber()
	rightNum, rightErr := right.ToNumber()
	if leftErr == nil && rightErr == nil {
		return leftNum < rightNum, true
	}

	// String comparison
	if left.Type == TypeText && right.Type == TypeText {
		return left.Text < right.Text, true
	}

	return false, false
}

// Parse parses a formula string and returns the AST.
func Parse(formula string) (Expression, error) {
	// Remove leading = if present
	formula = strings.TrimSpace(formula)
	formula = strings.TrimPrefix(formula, "=")

	parser := NewParser(formula)
	return parser.Parse()
}

// isCellRef checks if a string is a valid cell reference.
func isCellRef(s string) bool {
	if len(s) == 0 {
		return false
	}

	i := 0
	if s[i] == '$' {
		i++
	}

	// Must have at least one letter
	if i >= len(s) || !unicode.IsLetter(rune(s[i])) {
		return false
	}

	// Read letters
	for i < len(s) && unicode.IsLetter(rune(s[i])) {
		i++
	}

	// Must have at least one digit
	if i >= len(s) {
		return false
	}
	if s[i] == '$' {
		i++
	}
	if i >= len(s) || !unicode.IsDigit(rune(s[i])) {
		return false
	}

	// Read digits
	for i < len(s) && unicode.IsDigit(rune(s[i])) {
		i++
	}

	return i == len(s)
}
