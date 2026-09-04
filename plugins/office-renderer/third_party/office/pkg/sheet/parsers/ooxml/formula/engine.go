package formula

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Engine provides formula evaluation for a spreadsheet.
type Engine struct {
	// cells maps cell references to their values.
	cells map[string]Value
	// formulas maps cell references to their formula strings.
	formulas map[string]string
	// arrayFormulas maps array-member cell references to their anchor formula metadata.
	arrayFormulas map[string]arrayFormulaBinding
	// spillErrors tracks anchor cells whose inferred spill range is blocked.
	spillErrors map[string]bool
	// external resolvers handle sheet-qualified references.
	externalGetCell             func(string) Value
	externalGetRange            func(string, string) []Value
	externalResolveSheetOrdinal func(string) (int, bool)
	externalSheetCount          func() int
	resolvePivotData            func(string, string, Value, []PivotDataCriterion) (Value, bool)
	// evaluated tracks which cells have been evaluated.
	evaluated map[string]bool
	// evaluating tracks cells currently being evaluated (for circular detection).
	evaluating map[string]bool
}

// SetExternalResolver configures handlers for sheet-qualified references.
func (e *Engine) SetExternalResolver(getCell func(string) Value, getRange func(string, string) []Value) {
	e.externalGetCell = getCell
	e.externalGetRange = getRange
}

// SetSheetResolver configures handlers for resolving sheet ordinals and workbook sheet count.
func (e *Engine) SetSheetResolver(resolveSheetOrdinal func(string) (int, bool), sheetCount func() int) {
	e.externalResolveSheetOrdinal = resolveSheetOrdinal
	e.externalSheetCount = sheetCount
}

// SetPivotDataResolver configures workbook-backed GETPIVOTDATA lookup behavior.
func (e *Engine) SetPivotDataResolver(resolvePivotData func(string, string, Value, []PivotDataCriterion) (Value, bool)) {
	e.resolvePivotData = resolvePivotData
}

type arrayFormulaBinding struct {
	anchorRef string
	rangeRef  string
	formula   string
	implicit  bool
}

// ArrayFormulaGroup describes an explicit or inferred array formula range.
type ArrayFormulaGroup struct {
	AnchorRef string
	RangeRef  string
	Formula   string
	TopRow    int
	LeftCol   int
	BottomRow int
	RightCol  int
	Implicit  bool
}

// NewEngine creates a new formula evaluation engine.
func NewEngine() *Engine {
	return &Engine{
		cells:         make(map[string]Value),
		formulas:      make(map[string]string),
		arrayFormulas: make(map[string]arrayFormulaBinding),
		spillErrors:   make(map[string]bool),
		evaluated:     make(map[string]bool),
		evaluating:    make(map[string]bool),
	}
}

// SetCell sets a cell value.
func (e *Engine) SetCell(ref string, value Value) {
	ref = normalizeCellRef(ref)
	e.cells[ref] = value
}

// SetFormula sets a cell formula.
func (e *Engine) SetFormula(ref string, formula string) {
	ref = normalizeCellRef(ref)
	if binding, ok := e.arrayFormulas[ref]; ok && binding.implicit {
		e.clearImplicitArrayFormulas(binding.anchorRef)
	}
	e.clearImplicitArrayFormulas(ref)
	delete(e.spillErrors, ref)
	e.formulas[ref] = formula
	// Clear evaluated flag since formula changed
	delete(e.evaluated, ref)
	if ref == "" || strings.TrimSpace(formula) == "" || e.hasExplicitArrayAnchor(ref) {
		return
	}
	if rangeRef, ok := inferImplicitSpillRange(ref, formula); ok {
		_, blocked := e.setImplicitArrayFormula(ref, rangeRef, formula)
		if blocked {
			e.spillErrors[ref] = true
		}
	} else if rangeRef, ok := e.inferImplicitSpillRange(ref, formula); ok {
		_, blocked := e.setImplicitArrayFormula(ref, rangeRef, formula)
		if blocked {
			e.spillErrors[ref] = true
		}
	}
}

// Evaluate parses and evaluates an arbitrary formula expression against the engine.
// Cell formulas continue to use GetCell, which supplies the current-cell context used
// by spill and relative-reference functions.
func (e *Engine) Evaluate(expression string) (Value, error) {
	if e == nil {
		return Empty(), fmt.Errorf("formula: nil engine")
	}
	expr, err := Parse(expression)
	if err != nil {
		return Empty(), err
	}
	return expr.Eval(&engineContext{engine: e}), nil
}

// SetArrayFormula registers an array formula for every cell in the declared range.
func (e *Engine) SetArrayFormula(anchorRef, rangeRef, formula string) {
	anchorRef = normalizeCellRef(anchorRef)
	formula = strings.TrimSpace(formula)
	if anchorRef == "" || rangeRef == "" || formula == "" {
		return
	}
	e.clearImplicitArrayFormulas(anchorRef)
	delete(e.spillErrors, anchorRef)
	start, end, ok := parseRangeRef(rangeRef)
	if !ok {
		return
	}
	for row := start.row; row <= end.row; row++ {
		for col := start.col; col <= end.col; col++ {
			ref := fmt.Sprintf("%s%d", colToLetters(col), row)
			e.arrayFormulas[ref] = arrayFormulaBinding{anchorRef: anchorRef, rangeRef: rangeRef, formula: formula}
			delete(e.evaluated, ref)
		}
	}
}

func (e *Engine) setImplicitArrayFormula(anchorRef, rangeRef, formula string) (bool, bool) {
	start, end, ok := parseRangeRef(rangeRef)
	if !ok {
		return false, false
	}
	for row := start.row; row <= end.row; row++ {
		for col := start.col; col <= end.col; col++ {
			ref := fmt.Sprintf("%s%d", colToLetters(col), row)
			if ref != anchorRef {
				if _, ok := e.formulas[ref]; ok {
					return false, true
				}
				if _, ok := e.cells[ref]; ok {
					return false, true
				}
				if binding, ok := e.arrayFormulas[ref]; ok && (!binding.implicit || binding.anchorRef != anchorRef) {
					return false, true
				}
			}
		}
	}
	for row := start.row; row <= end.row; row++ {
		for col := start.col; col <= end.col; col++ {
			ref := fmt.Sprintf("%s%d", colToLetters(col), row)
			e.arrayFormulas[ref] = arrayFormulaBinding{anchorRef: anchorRef, rangeRef: rangeRef, formula: formula, implicit: true}
			delete(e.evaluated, ref)
		}
	}
	return true, false
}

func (e *Engine) clearImplicitArrayFormulas(anchorRef string) {
	for ref, binding := range e.arrayFormulas {
		if !binding.implicit || binding.anchorRef != anchorRef {
			continue
		}
		delete(e.arrayFormulas, ref)
		delete(e.evaluated, ref)
		if ref != anchorRef {
			delete(e.cells, ref)
		}
	}
}

func (e *Engine) hasExplicitArrayAnchor(anchorRef string) bool {
	binding, ok := e.arrayFormulas[anchorRef]
	return ok && binding.anchorRef == anchorRef && !binding.implicit
}

// ArrayFormulaGroups returns the explicit and inferred array ranges known to the engine.
func (e *Engine) ArrayFormulaGroups() []ArrayFormulaGroup {
	if len(e.arrayFormulas) == 0 {
		return nil
	}
	groups := make(map[string]ArrayFormulaGroup)
	for _, binding := range e.arrayFormulas {
		if binding.anchorRef == "" || binding.rangeRef == "" {
			continue
		}
		start, end, ok := parseRangeRef(binding.rangeRef)
		if !ok {
			continue
		}
		key := binding.anchorRef + "|" + binding.rangeRef + "|" + binding.formula
		groups[key] = ArrayFormulaGroup{
			AnchorRef: binding.anchorRef,
			RangeRef:  binding.rangeRef,
			Formula:   binding.formula,
			TopRow:    start.row,
			LeftCol:   start.col,
			BottomRow: end.row,
			RightCol:  end.col,
			Implicit:  binding.implicit,
		}
	}
	if len(groups) == 0 {
		return nil
	}
	result := make([]ArrayFormulaGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, group)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TopRow != result[j].TopRow {
			return result[i].TopRow < result[j].TopRow
		}
		if result[i].LeftCol != result[j].LeftCol {
			return result[i].LeftCol < result[j].LeftCol
		}
		return result[i].AnchorRef < result[j].AnchorRef
	})
	return result
}

// GetCell returns a cell's value, evaluating if necessary.
func (e *Engine) GetCell(ref string) Value {
	ref = normalizeCellRef(ref)
	if hasSheetQualifier(ref) && e.externalGetCell != nil {
		return e.externalGetCell(ref)
	}

	// Check if already evaluated
	if val, ok := e.cells[ref]; ok {
		if _, hasFormula := e.formulas[ref]; !hasFormula {
			return val
		}
		if e.evaluated[ref] {
			return val
		}
	}

	// Check for circular reference
	if e.evaluating[ref] {
		return Error(ErrRef)
	}

	// Evaluate the cell
	return e.evaluateCell(ref)
}

// evaluateCell evaluates a cell's formula.
func (e *Engine) evaluateCell(ref string) Value {
	formula, ok := e.formulas[ref]
	arrayFormula, hasArrayFormula := e.arrayFormulas[ref]
	if !ok && hasArrayFormula {
		formula = arrayFormula.formula
		ok = true
	}
	if ok && !hasArrayFormula && e.spillErrors[ref] {
		e.cells[ref] = Error(ErrSpill)
		e.evaluated[ref] = true
		return e.cells[ref]
	}
	if !ok {
		// No formula, return stored value or empty
		if val, ok := e.cells[ref]; ok {
			return val
		}
		return Empty()
	}

	// Mark as evaluating
	e.evaluating[ref] = true
	defer delete(e.evaluating, ref)

	// Parse and evaluate
	ctx := &engineContext{engine: e, currentRef: ref}
	if hasArrayFormula {
		ctx.arrayAnchorRef = arrayFormula.anchorRef
	}
	expr, err := Parse(formula)
	if err != nil {
		e.cells[ref] = Error(ErrValue)
		e.evaluated[ref] = true
		return e.cells[ref]
	}

	result := expr.Eval(ctx)
	e.cells[ref] = result
	e.evaluated[ref] = true
	return result
}

// evaluateAll evaluates all formulas in the engine.
func (e *Engine) evaluateAll() {
	for ref := range e.formulas {
		e.GetCell(ref)
	}
}

// GetRange returns all values in a cell range.
func (e *Engine) GetRange(start, end string) []Value {
	start = normalizeCellRef(start)
	end = normalizeCellRef(end)
	if (hasSheetQualifier(start) || hasSheetQualifier(end)) && e.externalGetRange != nil {
		return e.externalGetRange(start, end)
	}
	sheetPrefix, startCell := splitSheetRef(start)
	endPrefix, endCell := splitSheetRef(end)
	if endPrefix == "" {
		endPrefix = sheetPrefix
	}
	if sheetPrefix != endPrefix {
		return nil
	}
	startRow, startCol := parseCellRef(startCell)
	endRow, endCol := parseCellRef(endCell)

	if startRow == 0 || endRow == 0 || startCol == 0 || endCol == 0 {
		return nil
	}

	// Normalize range
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	if startCol > endCol {
		startCol, endCol = endCol, startCol
	}

	var values []Value
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			ref := sheetPrefix + fmt.Sprintf("%s%d", colToLetters(col), row)
			values = append(values, e.GetCell(ref))
		}
	}
	return values
}

// normalizeCellRef normalizes a cell reference.
func normalizeCellRef(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.ToUpper(ref)
	ref = strings.ReplaceAll(ref, "$", "")
	return ref
}

func hasSheetQualifier(ref string) bool {
	return strings.LastIndex(ref, "!") >= 0
}

func splitSheetRef(ref string) (string, string) {
	idx := strings.LastIndex(ref, "!")
	if idx < 0 {
		return "", ref
	}
	return ref[:idx+1], ref[idx+1:]
}

// colToLetters converts a column number to letters (1 -> A, 27 -> AA).
func colToLetters(col int) string {
	if col <= 0 {
		return ""
	}

	var result []byte
	for col > 0 {
		col--
		result = append(result, byte('A'+col%26))
		col /= 26
	}

	// Reverse
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// engineContext implements the Context interface for formula evaluation.
type engineContext struct {
	engine         *Engine
	currentRef     string
	arrayAnchorRef string
}

func (c *engineContext) GetCell(ref string) Value {
	return c.engine.GetCell(ref)
}

func (c *engineContext) GetRange(start, end string) []Value {
	return c.engine.GetRange(start, end)
}

func (c *engineContext) CurrentRef() string {
	return c.currentRef
}

func (c *engineContext) ArrayAnchorRef() string {
	return c.arrayAnchorRef
}

func (c *engineContext) GetFormula(ref string) (string, bool) {
	return c.engine.GetFormula(ref)
}

func (c *engineContext) ResolveSheetOrdinal(token string) (int, bool) {
	return c.engine.ResolveSheetOrdinal(token)
}

func (c *engineContext) SheetCount() int {
	return c.engine.SheetCount()
}

func (c *engineContext) GetSpillRange(ref string) (string, string, bool) {
	return c.engine.GetSpillRange(ref)
}

func (c *engineContext) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool) {
	return c.engine.ResolvePivotData(dataField, pivotTableRef, pivotTableValue, criteria)
}

// Evaluate evaluates a formula string and returns the result.
// This is a standalone evaluation without cell context.
func Evaluate(formula string) (Value, error) {
	expr, err := Parse(formula)
	if err != nil {
		return Value{}, err
	}

	// Create a minimal context
	ctx := &emptyContext{}
	return expr.Eval(ctx), nil
}

// emptyContext provides a minimal context for standalone evaluation.
type emptyContext struct{}

func (c *emptyContext) GetCell(ref string) Value {
	return Empty()
}

func (c *emptyContext) GetRange(start, end string) []Value {
	return nil
}

func (c *emptyContext) CurrentRef() string {
	return ""
}

func (c *emptyContext) ArrayAnchorRef() string {
	return ""
}

func (c *emptyContext) GetSpillRange(ref string) (string, string, bool) {
	return "", "", false
}

func (c *emptyContext) GetFormula(ref string) (string, bool) {
	return "", false
}

func (c *emptyContext) ResolveSheetOrdinal(token string) (int, bool) {
	token = strings.TrimSpace(token)
	if token == "" || isCellRef(normalizeCellRef(token)) {
		return 1, true
	}
	return 0, false
}

func (c *emptyContext) SheetCount() int {
	return 1
}

func (c *emptyContext) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool) {
	return Value{}, false
}

// EvaluateInContext evaluates a formula with access to cell values.
func EvaluateInContext(formula string, getCell func(string) Value, getRange func(string, string) []Value) (Value, error) {
	expr, err := Parse(formula)
	if err != nil {
		return Value{}, err
	}

	ctx := &callbackContext{
		getCell:  getCell,
		getRange: getRange,
	}
	return expr.Eval(ctx), nil
}

// callbackContext implements Context using callback functions.
type callbackContext struct {
	getCell  func(string) Value
	getRange func(string, string) []Value
}

func (c *callbackContext) GetCell(ref string) Value {
	if c.getCell != nil {
		return c.getCell(ref)
	}
	return Empty()
}

func (c *callbackContext) GetRange(start, end string) []Value {
	if c.getRange != nil {
		return c.getRange(start, end)
	}
	return nil
}

func (c *callbackContext) CurrentRef() string {
	return ""
}

func (c *callbackContext) ArrayAnchorRef() string {
	return ""
}

func (c *callbackContext) GetSpillRange(ref string) (string, string, bool) {
	return "", "", false
}

func (c *callbackContext) GetFormula(ref string) (string, bool) {
	return "", false
}

func (e *Engine) GetFormula(ref string) (string, bool) {
	ref = normalizeCellRef(ref)
	if formula, ok := e.formulas[ref]; ok {
		return formula, true
	}
	if binding, ok := e.arrayFormulas[ref]; ok {
		return binding.formula, true
	}
	return "", false
}

func (c *callbackContext) ResolveSheetOrdinal(token string) (int, bool) {
	token = strings.TrimSpace(token)
	if token == "" || isCellRef(normalizeCellRef(token)) {
		return 1, true
	}
	return 0, false
}

func (c *callbackContext) SheetCount() int {
	return 1
}

func (c *callbackContext) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool) {
	return Value{}, false
}

func (e *Engine) GetSpillRange(ref string) (string, string, bool) {
	ref = normalizeCellRef(ref)
	if binding, ok := e.arrayFormulas[ref]; ok {
		parts := strings.SplitN(binding.rangeRef, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], true
		}
		return parts[0], parts[0], true
	}
	return "", "", false
}

func (e *Engine) ResolveSheetOrdinal(token string) (int, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 1, true
	}
	if e.externalResolveSheetOrdinal != nil {
		if index, ok := e.externalResolveSheetOrdinal(token); ok {
			return index, true
		}
	}
	if hasSheetQualifier(token) {
		return 0, false
	}
	if isCellRef(normalizeCellRef(token)) {
		return 1, true
	}
	return 0, false
}

func (e *Engine) SheetCount() int {
	if e.externalSheetCount != nil {
		if count := e.externalSheetCount(); count > 0 {
			return count
		}
	}
	return 1
}

// ResolvePivotData delegates GETPIVOTDATA resolution to the configured resolver.
func (e *Engine) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue Value, criteria []PivotDataCriterion) (Value, bool) {
	if e.resolvePivotData == nil {
		return Value{}, false
	}
	return e.resolvePivotData(dataField, pivotTableRef, pivotTableValue, criteria)
}

type cellCoords struct {
	row int
	col int
}

type arrayShape struct {
	rows int
	cols int
}

func inferImplicitSpillRange(anchorRef, formula string) (string, bool) {
	expr, err := Parse(formula)
	if err != nil {
		return "", false
	}
	shape, ok := inferExpressionArrayShape(expr)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return "", false
	}
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if anchorRow == 0 || anchorCol == 0 {
		return "", false
	}
	bottomRow := anchorRow + shape.rows - 1
	rightCol := anchorCol + shape.cols - 1
	return fmt.Sprintf("%s:%s", anchorRef, fmt.Sprintf("%s%d", colToLetters(rightCol), bottomRow)), true
}

func (e *Engine) inferImplicitSpillRange(anchorRef, formula string) (string, bool) {
	expr, err := Parse(formula)
	if err != nil {
		return "", false
	}
	ctx := &engineContext{engine: e, currentRef: anchorRef, arrayAnchorRef: anchorRef}
	shape, ok := inferExpressionArrayShapeWithContext(expr, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return "", false
	}
	anchorRow, anchorCol := parseCellRef(anchorRef)
	if anchorRow == 0 || anchorCol == 0 {
		return "", false
	}
	bottomRow := anchorRow + shape.rows - 1
	rightCol := anchorCol + shape.cols - 1
	return fmt.Sprintf("%s:%s", anchorRef, fmt.Sprintf("%s%d", colToLetters(rightCol), bottomRow)), true
}

func inferExpressionArrayShape(expr Expression) (arrayShape, bool) {
	switch expr := expr.(type) {
	case *RangeExpr:
		start, end, ok := parseRangeRef(expr.Start + ":" + expr.End)
		if !ok {
			return arrayShape{}, false
		}
		return arrayShape{rows: end.row - start.row + 1, cols: end.col - start.col + 1}, true
	case *UnaryExpr:
		return inferExpressionArrayShape(expr.Expr)
	case *BinaryExpr:
		leftShape, leftOk := inferExpressionArrayShape(expr.Left)
		rightShape, rightOk := inferExpressionArrayShape(expr.Right)
		return combineArrayShapes(leftShape, leftOk, rightShape, rightOk)
	case *ApplyExpr:
		return arrayShape{}, false
	case *FuncCallExpr:
		switch strings.ToUpper(expr.Name) {
		case "IF":
			return inferConditionalBroadcastArrayShape(expr.Args)
		case "IFS":
			return inferConditionalBroadcastArrayShape(expr.Args)
		case "SWITCH":
			if len(expr.Args) > 0 {
				if selectorShape, ok := inferExpressionArrayShape(expr.Args[0]); ok && (selectorShape.rows > 1 || selectorShape.cols > 1) {
					return selectorShape, true
				}
			}
			if len(expr.Args) >= 2 {
				return inferConditionalBroadcastArrayShape(expr.Args[1:])
			}
			return arrayShape{}, false
		case "IFERROR", "IFNA":
			if len(expr.Args) != 2 {
				return arrayShape{}, false
			}
			if shape, ok := inferExpressionArrayShape(expr.Args[0]); ok {
				return shape, true
			}
			return inferExpressionArrayShape(expr.Args[1])
		case "TRANSPOSE":
			if len(expr.Args) != 1 {
				return arrayShape{}, false
			}
			shape, ok := inferExpressionArrayShape(expr.Args[0])
			if !ok {
				return arrayShape{}, false
			}
			return arrayShape{rows: shape.cols, cols: shape.rows}, true
		case "CHOOSE":
			return inferChooseArrayShape(expr.Args)
		case "CHOOSECOLS":
			return inferChooseAxisArrayShape(expr.Args, false)
		case "CHOOSEROWS":
			return inferChooseAxisArrayShape(expr.Args, true)
		case "MMULT":
			return inferMMultArrayShape(expr.Args, inferExpressionArrayShape)
		case "MINVERSE":
			return inferMinverseArrayShape(expr.Args, inferExpressionArrayShape)
		case "MUNIT":
			return inferMunitArrayShape(expr.Args, inferStaticIndexSelector)
		case "UNIQUE":
			return arrayShape{}, false
		case "SORT":
			return inferSortArrayShape(expr.Args)
		case "SORTBY":
			return inferSortByArrayShape(expr.Args)
		case "SEQUENCE":
			return inferSequenceArrayShape(expr.Args)
		case "RANDARRAY":
			return inferRandArrayShape(expr.Args)
		case "EXPAND":
			return inferExpandArrayShape(expr.Args)
		case "MAKEARRAY":
			return inferMakeArrayShape(expr.Args)
		case "BYROW":
			return inferByAxisArrayShape(expr.Args, true)
		case "BYCOL":
			return inferByAxisArrayShape(expr.Args, false)
		case "MAP":
			return inferMapArrayShape(expr.Args)
		case "SCAN":
			return inferScanArrayShape(expr.Args)
		case "LET":
			return arrayShape{}, false
		case "FILTER":
			return arrayShape{}, false
		case "HSTACK":
			return inferStackArrayShape(expr.Args, true)
		case "VSTACK":
			return inferStackArrayShape(expr.Args, false)
		case "TAKE":
			return inferTakeDropArrayShape(expr.Args, true)
		case "DROP":
			return inferTakeDropArrayShape(expr.Args, false)
		case "TOROW":
			return inferToVectorArrayShape(expr.Args, false)
		case "TOCOL":
			return inferToVectorArrayShape(expr.Args, true)
		case "WRAPROWS":
			return inferWrapArrayShape(expr.Args, true)
		case "WRAPCOLS":
			return inferWrapArrayShape(expr.Args, false)
		case "TEXTSPLIT":
			return inferTextsplitArrayShape(expr.Args)
		case "INDEX":
			return inferIndexArrayShape(expr.Args)
		case "OFFSET":
			return inferOffsetArrayShape(expr.Args)
		case "INDIRECT":
			return inferIndirectArrayShape(expr.Args, stubContext{})
		case "MATCH":
			return inferMatchArrayShape(expr.Args, inferExpressionArrayShape)
		case "XMATCH":
			return inferXMatchArrayShape(expr.Args, inferExpressionArrayShape)
		case "LOOKUP":
			return inferLookupScalarArrayShape(expr.Args, inferExpressionArrayShape, 0)
		case "VLOOKUP", "HLOOKUP":
			return inferLookupIndexedArrayShape(expr.Args, inferExpressionArrayShape, 0, 2)
		case "XLOOKUP":
			return inferXLookupArrayShape(expr.Args, inferExpressionArrayShape)
		case "ROW", "COLUMN":
			if len(expr.Args) == 0 {
				return arrayShape{}, false
			}
			return inferExpressionArrayShape(expr.Args[0])
		default:
			if !isScalarSpillFunction(expr.Name) {
				return arrayShape{}, false
			}
			var shapes []arrayShape
			for _, arg := range expr.Args {
				shape, ok := inferExpressionArrayShape(arg)
				if ok {
					shapes = append(shapes, shape)
				}
			}
			if len(shapes) == 0 {
				return arrayShape{}, false
			}
			return broadcastArrayShapes(shapes...)
		}
	default:
		return arrayShape{}, false
	}
}

func inferConditionalBroadcastArrayShape(args []Expression) (arrayShape, bool) {
	var shapes []arrayShape
	var okAny bool
	for _, arg := range args {
		shape, ok := inferExpressionArrayShape(arg)
		shapes = append(shapes, shape)
		okAny = okAny || ok
	}
	if !okAny {
		return arrayShape{}, false
	}
	return broadcastArrayShapes(shapes...)
}

func inferMMultArrayShape(args []Expression, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	if len(args) != 2 {
		return arrayShape{}, false
	}
	left, ok := infer(args[0])
	if !ok || left.rows <= 0 || left.cols <= 0 {
		return arrayShape{}, false
	}
	right, ok := infer(args[1])
	if !ok || right.rows <= 0 || right.cols <= 0 {
		return arrayShape{}, false
	}
	if left.cols != right.rows {
		return arrayShape{}, false
	}
	return arrayShape{rows: left.rows, cols: right.cols}, true
}

func inferMinverseArrayShape(args []Expression, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	if len(args) != 1 {
		return arrayShape{}, false
	}
	shape, ok := infer(args[0])
	if !ok || shape.rows <= 0 || shape.cols <= 0 || shape.rows != shape.cols {
		return arrayShape{}, false
	}
	return shape, true
}

func inferMunitArrayShape(args []Expression, infer func(Expression) (int, bool)) (arrayShape, bool) {
	if len(args) != 1 {
		return arrayShape{}, false
	}
	n, ok := infer(args[0])
	if !ok || n < 1 {
		return arrayShape{}, false
	}
	return arrayShape{rows: n, cols: n}, true
}

func inferChooseArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) < 2 {
		return arrayShape{}, false
	}
	selector, ok := inferStaticIndexSelector(args[0])
	if !ok || selector < 1 || selector >= len(args) {
		return arrayShape{}, false
	}
	return inferExpressionArrayShape(args[selector])
}

func inferChooseAxisArrayShape(args []Expression, selectRows bool) (arrayShape, bool) {
	if len(args) < 2 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShape(args[0])
	if !ok {
		return arrayShape{}, false
	}
	indices, ok := inferStaticChooseAxisSelectors(args[1:])
	if !ok {
		return arrayShape{}, false
	}
	if selectRows {
		for _, index := range indices {
			if _, ok := resolveChooseAxisIndex(index, shape.rows); !ok {
				return arrayShape{}, false
			}
		}
		return arrayShape{rows: len(indices), cols: shape.cols}, len(indices) > 1 || shape.cols > 1
	}
	for _, index := range indices {
		if _, ok := resolveChooseAxisIndex(index, shape.cols); !ok {
			return arrayShape{}, false
		}
	}
	return arrayShape{rows: shape.rows, cols: len(indices)}, shape.rows > 1 || len(indices) > 1
}

func inferStaticChooseAxisSelectors(args []Expression) ([]int, bool) {
	indices := make([]int, 0, len(args))
	for _, arg := range args {
		idx, ok := inferStaticIndexSelector(arg)
		if !ok || idx == 0 {
			return nil, false
		}
		indices = append(indices, idx)
	}
	if len(indices) == 0 {
		return nil, false
	}
	return indices, true
}

func inferOffsetArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) < 3 || len(args) > 5 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShape(args[0])
	if !ok {
		shape = arrayShape{rows: 1, cols: 1}
	}
	if len(args) >= 4 {
		height, ok := inferStaticIndexSelector(args[3])
		if !ok || height <= 0 {
			return arrayShape{}, false
		}
		shape.rows = height
	}
	if len(args) >= 5 {
		width, ok := inferStaticIndexSelector(args[4])
		if !ok || width <= 0 {
			return arrayShape{}, false
		}
		shape.cols = width
	}
	if shape.rows <= 1 && shape.cols <= 1 {
		return arrayShape{}, false
	}
	return shape, true
}

func inferXLookupArrayShape(args []Expression, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	if len(args) < 3 || len(args) > 6 {
		return arrayShape{}, false
	}
	modeShape, hasModeShape := inferXLookupOptionalArrayShape(args, infer)
	if lookupValueShape, ok := infer(args[0]); ok {
		if lookupValueShape.rows > 0 && lookupValueShape.cols > 0 && (lookupValueShape.rows > 1 || lookupValueShape.cols > 1) {
			lookupArrayShape, ok := infer(args[1])
			if !ok {
				return arrayShape{}, false
			}
			if lookupArrayShape.rows <= 0 || lookupArrayShape.cols <= 0 {
				return arrayShape{}, false
			}
			if (lookupArrayShape.rows != 1 && lookupArrayShape.cols != 1) || (lookupArrayShape.rows == 1 && lookupArrayShape.cols == 1) {
				return arrayShape{}, false
			}
			returnShape, ok := infer(args[2])
			if !ok {
				return arrayShape{}, false
			}
			if lookupArrayShape.cols == 1 {
				if returnShape.rows != lookupArrayShape.rows {
					return arrayShape{}, false
				}
				resultShape := lookupValueShape
				if returnShape.cols > 1 {
					var combined bool
					resultShape, combined = broadcastArrayShapes(lookupValueShape, arrayShape{rows: 1, cols: returnShape.cols})
					if !combined {
						return arrayShape{}, false
					}
				}
				return combineLookupShape(resultShape, hasModeShape, modeShape)
			}
			if returnShape.cols != lookupArrayShape.cols {
				return arrayShape{}, false
			}
			resultShape := lookupValueShape
			if returnShape.rows > 1 {
				var combined bool
				resultShape, combined = broadcastArrayShapes(lookupValueShape, arrayShape{rows: returnShape.rows, cols: 1})
				if !combined {
					return arrayShape{}, false
				}
			}
			return combineLookupShape(resultShape, hasModeShape, modeShape)
		}
	}
	lookupShape, ok := infer(args[1])
	if !ok {
		return arrayShape{}, false
	}
	if lookupShape.rows <= 0 || lookupShape.cols <= 0 {
		return arrayShape{}, false
	}
	if lookupShape.rows != 1 && lookupShape.cols != 1 {
		return arrayShape{}, false
	}
	if lookupShape.rows == 1 && lookupShape.cols == 1 {
		return arrayShape{}, false
	}
	returnShape, ok := infer(args[2])
	if !ok {
		return arrayShape{}, false
	}
	if lookupShape.cols == 1 {
		if returnShape.rows != lookupShape.rows {
			return arrayShape{}, false
		}
		if returnShape.cols <= 1 && !hasModeShape {
			return arrayShape{}, false
		}
		return combineLookupShape(arrayShape{rows: 1, cols: returnShape.cols}, hasModeShape, modeShape)
	}
	if returnShape.cols != lookupShape.cols {
		return arrayShape{}, false
	}
	if returnShape.rows <= 1 && !hasModeShape {
		return arrayShape{}, false
	}
	return combineLookupShape(arrayShape{rows: returnShape.rows, cols: 1}, hasModeShape, modeShape)
}

func inferMatchArrayShape(args []Expression, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	if len(args) < 2 || len(args) > 3 {
		return arrayShape{}, false
	}
	baseShape, hasBaseShape := inferLookupBroadcastArrayShape(args, infer, 0)
	optionalShape, hasOptionalShape := inferLookupBroadcastArrayShape(args, infer, 2)
	if hasBaseShape {
		return combineLookupShape(baseShape, hasOptionalShape, optionalShape)
	}
	return optionalShape, hasOptionalShape
}

func inferXMatchArrayShape(args []Expression, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	if len(args) < 2 || len(args) > 4 {
		return arrayShape{}, false
	}
	modeShape, hasModeShape := inferXMatchModeArrayShape(args, infer)
	lookupValueShape, hasLookupValueShape := infer(args[0])
	if hasLookupValueShape && lookupValueShape.rows > 0 && lookupValueShape.cols > 0 && (lookupValueShape.rows > 1 || lookupValueShape.cols > 1) {
		return combineLookupShape(lookupValueShape, hasModeShape, modeShape)
	}
	if hasModeShape {
		return modeShape, true
	}
	return arrayShape{}, false
}

func inferXMatchModeArrayShape(args []Expression, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	matchShape, hasMatchShape := inferLookupBroadcastArrayShape(args, infer, 2)
	searchShape, hasSearchShape := inferLookupBroadcastArrayShape(args, infer, 3)
	if hasMatchShape {
		if hasSearchShape {
			if combined, ok := broadcastArrayShapes(matchShape, searchShape); ok {
				if combined.rows == matchShape.rows && combined.cols == matchShape.cols {
					return combined, true
				}
			}
		}
		return matchShape, true
	}
	return searchShape, hasSearchShape
}

func inferXLookupOptionalArrayShape(args []Expression, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	modeShape, hasModeShape := inferXLookupModeArrayShape(args, infer)
	fallbackShape, hasFallbackShape := inferLookupBroadcastArrayShape(args, infer, 3)
	if hasModeShape {
		if hasFallbackShape {
			if combined, ok := broadcastArrayShapes(modeShape, fallbackShape); ok {
				if combined.rows == modeShape.rows && combined.cols == modeShape.cols {
					return combined, true
				}
			}
		}
		return modeShape, true
	}
	return fallbackShape, hasFallbackShape
}

func inferXLookupModeArrayShape(args []Expression, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	matchShape, hasMatchShape := inferLookupBroadcastArrayShape(args, infer, 4)
	searchShape, hasSearchShape := inferLookupBroadcastArrayShape(args, infer, 5)
	if hasMatchShape {
		if hasSearchShape {
			if combined, ok := broadcastArrayShapes(matchShape, searchShape); ok {
				if combined.rows == matchShape.rows && combined.cols == matchShape.cols {
					return combined, true
				}
			}
		}
		return matchShape, true
	}
	return searchShape, hasSearchShape
}

func inferLookupScalarArrayShape(args []Expression, infer func(Expression) (arrayShape, bool), argIndexes ...int) (arrayShape, bool) {
	if len(args) < 2 {
		return arrayShape{}, false
	}
	if len(argIndexes) == 0 {
		argIndexes = []int{0}
	}
	return inferLookupBroadcastArrayShape(args, infer, argIndexes...)
}

func inferLookupIndexedArrayShape(args []Expression, infer func(Expression) (arrayShape, bool), argIndexes ...int) (arrayShape, bool) {
	if len(args) < 3 {
		return arrayShape{}, false
	}
	baseIndexes := argIndexes
	if len(baseIndexes) == 0 {
		baseIndexes = []int{0, 2}
	}
	baseShape, hasBaseShape := inferLookupBroadcastArrayShape(args, infer, baseIndexes...)
	optionalShape, hasOptionalShape := inferLookupBroadcastArrayShape(args, infer, 3)
	if hasBaseShape {
		return combineLookupShape(baseShape, hasOptionalShape, optionalShape)
	}
	return optionalShape, hasOptionalShape
}

func inferLookupBroadcastArrayShape(args []Expression, infer func(Expression) (arrayShape, bool), argIndexes ...int) (arrayShape, bool) {
	var shapes []arrayShape
	for _, argIndex := range argIndexes {
		if argIndex < 0 || argIndex >= len(args) {
			continue
		}
		shape, ok := infer(args[argIndex])
		if !ok || (shape.rows <= 1 && shape.cols <= 1) {
			continue
		}
		shapes = append(shapes, shape)
	}
	if len(shapes) == 0 {
		return arrayShape{}, false
	}
	return broadcastArrayShapes(shapes...)
}

func combineLookupShape(base arrayShape, hasExtra bool, extra arrayShape) (arrayShape, bool) {
	if !hasExtra {
		return base, true
	}
	shape, ok := broadcastArrayShapes(base, extra)
	if !ok {
		return base, true
	}
	if (base.rows > 1 || base.cols > 1) && (shape.rows != base.rows || shape.cols != base.cols) {
		return base, true
	}
	return shape, true
}

func inferTakeDropArrayShape(args []Expression, take bool) (arrayShape, bool) {
	if len(args) < 2 || len(args) > 3 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShape(args[0])
	if !ok {
		return arrayShape{}, false
	}
	rowCount, ok := inferStaticIndexSelector(args[1])
	if !ok || rowCount == 0 || absInt(rowCount) > shape.rows {
		return arrayShape{}, false
	}
	result := shape
	if take {
		result.rows = absInt(rowCount)
	} else {
		result.rows = shape.rows - absInt(rowCount)
	}
	if len(args) == 3 {
		colCount, ok := inferStaticIndexSelector(args[2])
		if !ok || colCount == 0 || absInt(colCount) > shape.cols {
			return arrayShape{}, false
		}
		if take {
			result.cols = absInt(colCount)
		} else {
			result.cols = shape.cols - absInt(colCount)
		}
	}
	if result.rows <= 0 || result.cols <= 0 {
		return arrayShape{}, false
	}
	return result, result.rows > 1 || result.cols > 1
}

func inferStackArrayShape(args []Expression, horizontal bool) (arrayShape, bool) {
	if len(args) == 0 {
		return arrayShape{}, false
	}
	shape, ok := combineStackShapes(args, horizontal, func(expr Expression) (arrayShape, bool) {
		if inferred, ok := inferExpressionArrayShape(expr); ok {
			return inferred, true
		}
		return arrayShape{rows: 1, cols: 1}, true
	})
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferToVectorArrayShape(args []Expression, asColumn bool) (arrayShape, bool) {
	if len(args) < 1 || len(args) > 3 {
		return arrayShape{}, false
	}
	shape, ok := inferLinearizeSourceShape(args[0])
	if !ok {
		return arrayShape{}, false
	}
	ignoreMode := 0
	if len(args) >= 2 {
		ignoreValue, ok := inferStaticIndexSelector(args[1])
		if !ok || ignoreValue < 0 || ignoreValue > 3 {
			return arrayShape{}, false
		}
		ignoreMode = ignoreValue
	}
	if ignoreMode != 0 {
		return arrayShape{}, false
	}
	if len(args) == 3 {
		if _, ok := inferStaticIndexSelector(args[2]); !ok {
			return arrayShape{}, false
		}
	}
	total := shape.rows * shape.cols
	if total <= 1 {
		return arrayShape{}, false
	}
	if asColumn {
		return arrayShape{rows: total, cols: 1}, true
	}
	return arrayShape{rows: 1, cols: total}, true
}

func inferWrapArrayShape(args []Expression, wrapRows bool) (arrayShape, bool) {
	if len(args) < 2 || len(args) > 3 {
		return arrayShape{}, false
	}
	shape, ok := inferLinearizeSourceShape(args[0])
	if !ok {
		return arrayShape{}, false
	}
	wrapCount, ok := inferStaticIndexSelector(args[1])
	if !ok || wrapCount <= 0 {
		return arrayShape{}, false
	}
	total := shape.rows * shape.cols
	if total <= 0 {
		return arrayShape{}, false
	}
	if wrapRows {
		rows := (total + wrapCount - 1) / wrapCount
		return arrayShape{rows: rows, cols: wrapCount}, rows > 1 || wrapCount > 1
	}
	cols := (total + wrapCount - 1) / wrapCount
	return arrayShape{rows: wrapCount, cols: cols}, wrapCount > 1 || cols > 1
}

func inferExpressionArrayShapeWithContext(expr Expression, ctx Context) (arrayShape, bool) {
	if shape, ok := inferExpressionArrayShape(expr); ok {
		return shape, true
	}
	switch expr := expr.(type) {
	case *CellRefExpr:
		if resolver, ok := ctx.(bindingResolver); ok {
			if _, shape, ok := resolver.ResolveArrayBinding(expr.Ref); ok {
				return shape, shape.rows > 1 || shape.cols > 1
			}
		}
		return arrayShape{}, false
	case *UnaryExpr:
		return inferExpressionArrayShapeWithContext(expr.Expr, ctx)
	case *BinaryExpr:
		leftShape, leftOk := inferExpressionArrayShapeWithContext(expr.Left, ctx)
		rightShape, rightOk := inferExpressionArrayShapeWithContext(expr.Right, ctx)
		return combineArrayShapes(leftShape, leftOk, rightShape, rightOk)
	case *ApplyExpr:
		return inferApplyArrayShapeWithContext(expr.Callee, expr.Args, ctx)
	case *FuncCallExpr:
		switch strings.ToUpper(expr.Name) {
		case "IF":
			var shapes []arrayShape
			var okAny bool
			for _, arg := range expr.Args {
				shape, ok := inferExpressionArrayShapeWithContext(arg, ctx)
				shapes = append(shapes, shape)
				okAny = okAny || ok
			}
			if !okAny {
				return arrayShape{}, false
			}
			return broadcastArrayShapes(shapes...)
		case "IFS":
			return inferIfsArrayShapeWithContext(expr.Args, ctx)
		case "SWITCH":
			return inferSwitchArrayShapeWithContext(expr.Args, ctx)
		case "IFERROR":
			return inferIfErrorArrayShapeWithContext(expr.Args, ctx)
		case "IFNA":
			return inferIfNaArrayShapeWithContext(expr.Args, ctx)
		case "TOROW":
			return inferToVectorArrayShapeWithContext(expr.Args, false, ctx)
		case "TOCOL":
			return inferToVectorArrayShapeWithContext(expr.Args, true, ctx)
		case "CHOOSE":
			return inferChooseArrayShapeWithContext(expr.Args, ctx)
		case "CHOOSECOLS":
			return inferChooseAxisArrayShapeWithContext(expr.Args, false, ctx)
		case "CHOOSEROWS":
			return inferChooseAxisArrayShapeWithContext(expr.Args, true, ctx)
		case "MMULT":
			return inferMMultArrayShape(expr.Args, func(inner Expression) (arrayShape, bool) {
				return inferExpressionArrayShapeWithContext(inner, ctx)
			})
		case "MINVERSE":
			return inferMinverseArrayShape(expr.Args, func(inner Expression) (arrayShape, bool) {
				return inferExpressionArrayShapeWithContext(inner, ctx)
			})
		case "MUNIT":
			sourceCtx := arraySourceContext(ctx)
			return inferMunitArrayShape(expr.Args, func(arg Expression) (int, bool) {
				value := arg.Eval(sourceCtx)
				if value.IsError() {
					return 0, false
				}
				number, err := value.ToNumber()
				if err != nil {
					return 0, false
				}
				return int(math.Trunc(number)), true
			})
		case "UNIQUE":
			return inferUniqueArrayShapeWithContext(expr.Args, ctx)
		case "SORT":
			return inferSortArrayShapeWithContext(expr.Args, ctx)
		case "SORTBY":
			return inferSortByArrayShapeWithContext(expr.Args, ctx)
		case "SEQUENCE":
			return inferSequenceArrayShapeWithContext(expr.Args, ctx)
		case "RANDARRAY":
			return inferRandArrayShapeWithContext(expr.Args, ctx)
		case "EXPAND":
			return inferExpandArrayShapeWithContext(expr.Args, ctx)
		case "MAKEARRAY":
			return inferMakeArrayShapeWithContext(expr.Args, ctx)
		case "BYROW":
			return inferByAxisArrayShapeWithContext(expr.Args, true, ctx)
		case "BYCOL":
			return inferByAxisArrayShapeWithContext(expr.Args, false, ctx)
		case "MAP":
			return inferMapArrayShapeWithContext(expr.Args, ctx)
		case "SCAN":
			return inferScanArrayShapeWithContext(expr.Args, ctx)
		case "LET":
			return inferLetArrayShapeWithContext(expr.Args, ctx)
		case "FILTER":
			return inferFilterArrayShapeWithContext(expr.Args, ctx)
		case "HSTACK":
			return inferStackArrayShapeWithContext(expr.Args, true, ctx)
		case "VSTACK":
			return inferStackArrayShapeWithContext(expr.Args, false, ctx)
		case "WRAPROWS":
			return inferWrapArrayShapeWithContext(expr.Args, true, ctx)
		case "WRAPCOLS":
			return inferWrapArrayShapeWithContext(expr.Args, false, ctx)
		case "TEXTSPLIT":
			return inferTextsplitArrayShapeWithContext(expr.Args, ctx)
		case "INDIRECT":
			return inferIndirectArrayShape(expr.Args, ctx)
		case "MATCH":
			return inferMatchArrayShape(expr.Args, func(inner Expression) (arrayShape, bool) {
				return inferExpressionArrayShapeWithContext(inner, ctx)
			})
		case "XMATCH":
			return inferXMatchArrayShape(expr.Args, func(inner Expression) (arrayShape, bool) {
				return inferExpressionArrayShapeWithContext(inner, ctx)
			})
		case "LOOKUP":
			return inferLookupScalarArrayShape(expr.Args, func(inner Expression) (arrayShape, bool) {
				return inferExpressionArrayShapeWithContext(inner, ctx)
			}, 0)
		case "VLOOKUP", "HLOOKUP":
			return inferLookupIndexedArrayShape(expr.Args, func(inner Expression) (arrayShape, bool) {
				return inferExpressionArrayShapeWithContext(inner, ctx)
			}, 0, 2)
		case "XLOOKUP":
			return inferXLookupArrayShape(expr.Args, func(inner Expression) (arrayShape, bool) {
				return inferExpressionArrayShapeWithContext(inner, ctx)
			})
		default:
			if resolver, ok := ctx.(lambdaBindingResolver); ok {
				if lambda, ok := resolver.ResolveLambdaBinding(expr.Name); ok {
					return inferLambdaInvocationArrayShapeWithContext(lambda, expr.Args, ctx)
				}
			}
			if !isScalarSpillFunction(expr.Name) {
				return arrayShape{}, false
			}
			var shapes []arrayShape
			for _, arg := range expr.Args {
				shape, ok := inferExpressionArrayShapeWithContext(arg, ctx)
				if ok {
					shapes = append(shapes, shape)
				}
			}
			if len(shapes) == 0 {
				return arrayShape{}, false
			}
			return broadcastArrayShapes(shapes...)
		}
	default:
		return arrayShape{}, false
	}
}

func inferIndirectArrayShape(args []Expression, ctx Context) (arrayShape, bool) {
	start, end, ok, _ := indirectTargetRange(args, ctx)
	if !ok {
		return arrayShape{}, false
	}
	startCoords, endCoords, ok := parseRangeRef(start + ":" + end)
	if !ok {
		return arrayShape{}, false
	}
	return arrayShape{
		rows: endCoords.row - startCoords.row + 1,
		cols: endCoords.col - startCoords.col + 1,
	}, true
}

func inferToVectorArrayShapeWithContext(args []Expression, asColumn bool, ctx Context) (arrayShape, bool) {
	if len(args) < 1 || len(args) > 3 {
		return arrayShape{}, false
	}
	ignoreMode, scanByColumn, ok := linearizeArrayOptions(args[1:], ctx)
	if !ok {
		return arrayShape{}, false
	}
	values, ok, _ := linearizeSourceValues(args[0], ignoreMode, scanByColumn, ctx)
	if !ok {
		return arrayShape{}, false
	}
	if len(values) <= 1 {
		return arrayShape{}, false
	}
	if asColumn {
		return arrayShape{rows: len(values), cols: 1}, true
	}
	return arrayShape{rows: 1, cols: len(values)}, true
}

func inferChooseArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	if len(args) < 2 {
		return arrayShape{}, false
	}
	if shape, ok := inferChooseArrayShape(args); ok {
		return shape, true
	}
	sourceCtx := arraySourceContext(ctx)
	values := normalizeArgumentValues(expandArgumentValues(args[0], sourceCtx))
	if len(values) <= 1 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShapeWithContext(args[0], sourceCtx)
	if !ok || shape.rows <= 0 || shape.cols <= 0 || shape.rows*shape.cols != len(values) {
		shape = arrayShape{rows: 1, cols: len(values)}
	}
	for _, value := range values {
		if value.IsError() {
			return arrayShape{}, false
		}
		num, err := value.ToNumber()
		if err != nil {
			return arrayShape{}, false
		}
		selector := int(num)
		if selector < 1 || selector >= len(args) {
			return arrayShape{}, false
		}
	}
	return shape, true
}

func inferIfsArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	var shapes []arrayShape
	var okAny bool
	for _, arg := range args {
		shape, ok := inferExpressionArrayShapeWithContext(arg, ctx)
		shapes = append(shapes, shape)
		okAny = okAny || ok
	}
	if okAny {
		return broadcastArrayShapes(shapes...)
	}
	selected, _, ok := selectIfsBranch(args, ctx)
	if !ok || selected == nil {
		return arrayShape{}, false
	}
	return inferExpressionArrayShapeWithContext(selected, ctx)
}

func inferSwitchArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	if len(args) >= 1 {
		if selectorShape, ok := inferExpressionArrayShapeWithContext(args[0], ctx); ok && (selectorShape.rows > 1 || selectorShape.cols > 1) {
			return selectorShape, true
		}
	}
	selected, _, ok := selectSwitchBranch(args, ctx)
	if !ok || selected == nil {
		return arrayShape{}, false
	}
	return inferExpressionArrayShapeWithContext(selected, ctx)
}

func inferIfErrorArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	if len(args) != 2 {
		return arrayShape{}, false
	}
	if shape, ok := inferExpressionArrayShapeWithContext(args[0], ctx); ok {
		return shape, true
	}
	value := args[0].Eval(ctx)
	if !value.IsError() {
		return arrayShape{}, false
	}
	return inferExpressionArrayShapeWithContext(args[1], ctx)
}

func inferIfNaArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	if len(args) != 2 {
		return arrayShape{}, false
	}
	if shape, ok := inferExpressionArrayShapeWithContext(args[0], ctx); ok {
		return shape, true
	}
	value := args[0].Eval(ctx)
	if !value.IsError() || value.Err != ErrNA {
		return arrayShape{}, false
	}
	return inferExpressionArrayShapeWithContext(args[1], ctx)
}

func inferChooseAxisArrayShapeWithContext(args []Expression, selectRows bool, ctx Context) (arrayShape, bool) {
	rows, cols, ok := chooseAxisReferenceShape(args, ctx, selectRows)
	if !ok || (rows <= 1 && cols <= 1) {
		return arrayShape{}, false
	}
	return arrayShape{rows: rows, cols: cols}, true
}

func inferWrapArrayShapeWithContext(args []Expression, wrapRows bool, ctx Context) (arrayShape, bool) {
	if len(args) < 2 || len(args) > 3 {
		return arrayShape{}, false
	}
	wrapCount, ok := indexSelectorNumber(args[1], ctx)
	if !ok || wrapCount <= 0 {
		return arrayShape{}, false
	}
	values, ok, _ := linearizeSourceValues(args[0], 0, false, ctx)
	if !ok || len(values) == 0 {
		return arrayShape{}, false
	}
	if wrapRows {
		rows := (len(values) + wrapCount - 1) / wrapCount
		return arrayShape{rows: rows, cols: wrapCount}, rows > 1 || wrapCount > 1
	}
	cols := (len(values) + wrapCount - 1) / wrapCount
	return arrayShape{rows: wrapCount, cols: cols}, wrapCount > 1 || cols > 1
}

func inferTextsplitArrayShape(args []Expression) (arrayShape, bool) {
	return inferTextsplitArrayShapeWithContext(args, &emptyContext{})
}

func inferTextsplitArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	if len(args) < 2 || len(args) > 6 {
		return arrayShape{}, false
	}
	textVal := args[0].Eval(ctx)
	if textVal.IsError() {
		return arrayShape{}, false
	}
	colDelims, ok, _ := textsplitDelimiters(args[1], ctx)
	if !ok {
		return arrayShape{}, false
	}
	var rowDelims []string
	if len(args) >= 3 {
		rowVal := args[2].Eval(ctx)
		if !rowVal.IsError() && !rowVal.IsEmpty() {
			rowDelims, ok, _ = textsplitDelimiters(args[2], ctx)
			if !ok {
				return arrayShape{}, false
			}
		}
	}
	ignoreEmpty := false
	if len(args) >= 4 {
		ignoreVal := args[3].Eval(ctx)
		if !ignoreVal.IsError() && !ignoreVal.IsEmpty() {
			flag, err := ignoreVal.ToBool()
			if err == nil {
				ignoreEmpty = flag
			}
		}
	}
	caseInsensitive := false
	if len(args) >= 5 {
		modeVal := args[4].Eval(ctx)
		if !modeVal.IsError() && !modeVal.IsEmpty() {
			mode, err := modeVal.ToNumber()
			if err == nil {
				caseInsensitive = mode != 0
			}
		}
	}
	rows := []string{textVal.ToText()}
	if len(rowDelims) > 0 {
		rows = textsplitSplit(textVal.ToText(), rowDelims, ignoreEmpty, caseInsensitive)
	}
	if len(rows) == 0 {
		return arrayShape{}, false
	}
	maxCols := 0
	for _, row := range rows {
		cols := textsplitSplit(row, colDelims, ignoreEmpty, caseInsensitive)
		if len(cols) > maxCols {
			maxCols = len(cols)
		}
	}
	if maxCols == 0 {
		return arrayShape{}, false
	}
	return arrayShape{rows: len(rows), cols: maxCols}, len(rows) > 1 || maxCols > 1
}

func inferStackArrayShapeWithContext(args []Expression, horizontal bool, ctx Context) (arrayShape, bool) {
	if len(args) == 0 {
		return arrayShape{}, false
	}
	shape, ok := combineStackShapes(args, horizontal, func(expr Expression) (arrayShape, bool) {
		if inferred, ok := inferExpressionArrayShapeWithContext(expr, ctx); ok {
			return inferred, true
		}
		return arrayShape{rows: 1, cols: 1}, true
	})
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func combineStackShapes(args []Expression, horizontal bool, infer func(Expression) (arrayShape, bool)) (arrayShape, bool) {
	result := arrayShape{}
	for _, arg := range args {
		shape, ok := infer(arg)
		if !ok || shape.rows <= 0 || shape.cols <= 0 {
			return arrayShape{}, false
		}
		if horizontal {
			if shape.rows > result.rows {
				result.rows = shape.rows
			}
			result.cols += shape.cols
		} else {
			result.rows += shape.rows
			if shape.cols > result.cols {
				result.cols = shape.cols
			}
		}
	}
	return result, result.rows > 0 && result.cols > 0
}

func inferFilterArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := filterArrayResult(args, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferUniqueArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := uniqueArrayResult(args, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferSortArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) < 1 || len(args) > 4 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShape(args[0])
	if !ok {
		return arrayShape{}, false
	}
	byColumn := false
	if len(args) == 4 {
		value, ok := inferStaticIndexSelector(args[3])
		if !ok || (value != 0 && value != 1) {
			return arrayShape{}, false
		}
		byColumn = value != 0
	}
	axis := sortSelectionAxis(shape, byColumn)
	keyLimit := shape.cols
	if axis == filterAxisCols {
		keyLimit = shape.rows
	}
	if len(args) >= 2 {
		sortIndex, ok := inferStaticIndexSelector(args[1])
		if !ok || sortIndex < 1 || sortIndex > keyLimit {
			return arrayShape{}, false
		}
	}
	if len(args) >= 3 {
		sortOrder, ok := inferStaticIndexSelector(args[2])
		if !ok || (sortOrder != 1 && sortOrder != -1) {
			return arrayShape{}, false
		}
	}
	return shape, shape.rows > 1 || shape.cols > 1
}

func inferSortArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := sortArrayResult(args, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferSortByArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) < 2 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShape(args[0])
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferSequenceArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) < 1 || len(args) > 4 {
		return arrayShape{}, false
	}
	rows, ok := inferStaticIndexSelector(args[0])
	if !ok || rows <= 0 {
		return arrayShape{}, false
	}
	cols := 1
	if len(args) >= 2 {
		cols, ok = inferStaticIndexSelector(args[1])
		if !ok || cols <= 0 {
			return arrayShape{}, false
		}
	}
	return arrayShape{rows: rows, cols: cols}, rows > 1 || cols > 1
}

func inferSortByArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := sortByArrayResult(args, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferSequenceArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	rows, cols, _, _, ok, _ := sequenceArgs(args, ctx)
	if !ok || (rows <= 1 && cols <= 1) {
		return arrayShape{}, false
	}
	return arrayShape{rows: rows, cols: cols}, true
}

func inferRandArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) > 5 {
		return arrayShape{}, false
	}
	rows := 1
	cols := 1
	var ok bool
	if len(args) >= 1 {
		rows, ok = inferStaticIndexSelector(args[0])
		if !ok || rows <= 0 {
			return arrayShape{}, false
		}
	}
	if len(args) >= 2 {
		cols, ok = inferStaticIndexSelector(args[1])
		if !ok || cols <= 0 {
			return arrayShape{}, false
		}
	}
	return arrayShape{rows: rows, cols: cols}, rows > 1 || cols > 1
}

func inferRandArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	rows, cols, _, _, _, ok, _ := randArrayArgs(args, ctx)
	if !ok || (rows <= 1 && cols <= 1) {
		return arrayShape{}, false
	}
	return arrayShape{rows: rows, cols: cols}, true
}

func inferExpandArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) < 2 || len(args) > 4 {
		return arrayShape{}, false
	}
	sourceShape, ok := inferExpressionArrayShape(args[0])
	if !ok {
		sourceShape = arrayShape{rows: 1, cols: 1}
	}
	rows, ok := inferStaticIndexSelector(args[1])
	if !ok || rows <= 0 {
		return arrayShape{}, false
	}
	cols := sourceShape.cols
	if len(args) >= 3 {
		cols, ok = inferStaticIndexSelector(args[2])
		if !ok || cols <= 0 {
			return arrayShape{}, false
		}
	}
	if rows < sourceShape.rows || cols < sourceShape.cols {
		return arrayShape{}, false
	}
	return arrayShape{rows: rows, cols: cols}, rows > 1 || cols > 1
}

func inferExpandArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := expandArrayResult(args, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferMakeArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) != 3 {
		return arrayShape{}, false
	}
	rows, ok := inferStaticIndexSelector(args[0])
	if !ok || rows <= 0 {
		return arrayShape{}, false
	}
	cols, ok := inferStaticIndexSelector(args[1])
	if !ok || cols <= 0 {
		return arrayShape{}, false
	}
	return arrayShape{rows: rows, cols: cols}, rows > 1 || cols > 1
}

func inferMakeArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := makeArrayResult(args, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferByAxisArrayShape(args []Expression, byRow bool) (arrayShape, bool) {
	if len(args) != 2 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShape(args[0])
	if !ok {
		shape = arrayShape{rows: 1, cols: 1}
	}
	if byRow {
		return arrayShape{rows: shape.rows, cols: 1}, shape.rows > 1
	}
	return arrayShape{rows: 1, cols: shape.cols}, shape.cols > 1
}

func inferByAxisArrayShapeWithContext(args []Expression, byRow bool, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := byAxisLambdaResult(args, byRow, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferMapArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) < 2 {
		return arrayShape{}, false
	}
	shapes := make([]arrayShape, 0, len(args)-1)
	for _, arg := range args[:len(args)-1] {
		shape, ok := inferExpressionArrayShape(arg)
		if !ok {
			shape = arrayShape{rows: 1, cols: 1}
		}
		shapes = append(shapes, shape)
	}
	shape, ok := broadcastArrayShapes(shapes...)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferMapArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := mapLambdaResult(args, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferScanArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) != 3 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShape(args[1])
	if !ok {
		shape = arrayShape{rows: 1, cols: 1}
	}
	if shape.rows <= 1 && shape.cols <= 1 {
		return arrayShape{}, false
	}
	return shape, true
}

func inferScanArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	_, shape, ok, _ := scanLambdaResult(args, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferLetArrayShapeWithContext(args []Expression, ctx Context) (arrayShape, bool) {
	letCtx, finalExpr, ok, _ := letContextForArgs(args, ctx)
	if !ok {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShapeWithContext(finalExpr, letCtx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferApplyArrayShapeWithContext(callee Expression, args []Expression, ctx Context) (arrayShape, bool) {
	lambdaCtx, body, ok, _ := lambdaInvocationContext(callee, args, ctx)
	if !ok {
		return arrayShape{}, false
	}
	return inferLambdaBodyArrayShape(body, lambdaCtx)
}

func inferLambdaInvocationArrayShapeWithContext(lambda lambdaClosure, args []Expression, ctx Context) (arrayShape, bool) {
	lambdaCtx, body, ok, _ := lambdaInvocationContextFromDefinition(lambda, args, ctx)
	if !ok {
		return arrayShape{}, false
	}
	return inferLambdaBodyArrayShape(body, lambdaCtx)
}

func inferLambdaBodyArrayShape(body Expression, ctx Context) (arrayShape, bool) {
	shape, ok := inferExpressionArrayShapeWithContext(body, ctx)
	if !ok || (shape.rows <= 1 && shape.cols <= 1) {
		return arrayShape{}, false
	}
	return shape, true
}

func inferIndexArrayShape(args []Expression) (arrayShape, bool) {
	if len(args) < 2 || len(args) > 4 {
		return arrayShape{}, false
	}
	shape, ok := inferExpressionArrayShape(args[0])
	if !ok {
		return arrayShape{}, false
	}
	rowNum, ok := inferStaticIndexSelector(args[1])
	if !ok {
		return arrayShape{}, false
	}
	colNum := 1
	if len(args) >= 3 {
		colValue, ok := inferStaticIndexSelector(args[2])
		if !ok {
			return arrayShape{}, false
		}
		colNum = colValue
	}
	if rowNum < 0 || colNum < 0 {
		return arrayShape{}, false
	}
	switch {
	case rowNum == 0 && colNum == 0:
		return shape, shape.rows > 1 || shape.cols > 1
	case rowNum == 0:
		if colNum < 1 || colNum > shape.cols {
			return arrayShape{}, false
		}
		return arrayShape{rows: shape.rows, cols: 1}, shape.rows > 1
	case colNum == 0:
		if rowNum < 1 || rowNum > shape.rows {
			return arrayShape{}, false
		}
		return arrayShape{rows: 1, cols: shape.cols}, shape.cols > 1
	default:
		return arrayShape{}, false
	}
}

func inferStaticIndexSelector(expr Expression) (int, bool) {
	switch expr := expr.(type) {
	case *NumberExpr:
		return int(expr.Value), true
	case *UnaryExpr:
		value, ok := inferStaticIndexSelector(expr.Expr)
		if !ok {
			return 0, false
		}
		if expr.Op == "-" {
			return -value, true
		}
		if expr.Op == "+" {
			return value, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func isScalarSpillFunction(name string) bool {
	switch strings.ToUpper(name) {
	case "ABS", "ROUND", "ROUNDUP", "ROUNDDOWN", "INT", "MOD", "POWER", "SQRT", "LEN", "LEFT", "RIGHT", "MID", "TRIM", "UPPER", "LOWER", "NOT", "ISNUMBER", "ISTEXT", "ISBLANK", "ISERROR", "VALUE", "TEXT", "CEILING", "FLOOR",
		"SIN", "COS", "TAN", "ASIN", "ACOS", "ATAN", "SINH", "COSH", "TANH", "ASINH", "ACOSH", "ATANH",
		"DEGREES", "RADIANS", "CLEAN", "NUMBERVALUE", "MROUND", "QUOTIENT",
		"PMT", "PV", "FV", "NPER", "RATE", "IPMT", "PPMT", "EFFECT", "NOMINAL",
		"SLN", "SYD", "DDB", "DB",
		"NORMDIST", "NORM.DIST", "NORMSDIST", "NORM.S.DIST",
		"NORMINV", "NORM.INV", "NORMSINV", "NORM.S.INV",
		"FACTDOUBLE", "COMBINA",
		"PDURATION", "RRI", "ISPMT", "DOLLARDE", "DOLLARFR":
		return true
	default:
		return false
	}
}

func combineArrayShapes(left arrayShape, leftOk bool, right arrayShape, rightOk bool) (arrayShape, bool) {
	switch {
	case leftOk && rightOk:
		return broadcastArrayShapes(left, right)
	case leftOk:
		return left, true
	case rightOk:
		return right, true
	default:
		return arrayShape{}, false
	}
}

func broadcastArrayShapes(shapes ...arrayShape) (arrayShape, bool) {
	target := arrayShape{rows: 1, cols: 1}
	for _, shape := range shapes {
		if shape.rows > target.rows {
			target.rows = shape.rows
		}
		if shape.cols > target.cols {
			target.cols = shape.cols
		}
	}
	for _, shape := range shapes {
		rows := shape.rows
		cols := shape.cols
		if rows == 0 {
			rows = 1
		}
		if cols == 0 {
			cols = 1
		}
		if rows != 1 && rows != target.rows {
			return arrayShape{}, false
		}
		if cols != 1 && cols != target.cols {
			return arrayShape{}, false
		}
	}
	return target, true
}

func parseRangeRef(ref string) (cellCoords, cellCoords, bool) {
	parts := strings.SplitN(strings.TrimSpace(ref), ":", 2)
	_, startRef := splitSheetRef(normalizeCellRef(parts[0]))
	startRow, startCol := parseCellRef(startRef)
	if startRow == 0 || startCol == 0 {
		return cellCoords{}, cellCoords{}, false
	}
	endRow, endCol := startRow, startCol
	if len(parts) == 2 {
		_, endRef := splitSheetRef(normalizeCellRef(parts[1]))
		endRow, endCol = parseCellRef(endRef)
		if endRow == 0 || endCol == 0 {
			return cellCoords{}, cellCoords{}, false
		}
	}
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	if startCol > endCol {
		startCol, endCol = endCol, startCol
	}
	return cellCoords{row: startRow, col: startCol}, cellCoords{row: endRow, col: endCol}, true
}
