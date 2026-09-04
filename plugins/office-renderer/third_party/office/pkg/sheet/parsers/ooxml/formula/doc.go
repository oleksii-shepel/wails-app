// Package formula provides formula evaluation for XLSX spreadsheets.
//
// The package implements a calculation engine that can evaluate Excel formulas
// including cell references, basic functions, and arithmetic operations.
//
// Supported features:
//   - Cell references (A1, $A$1, Sheet1!A1)
//   - Basic arithmetic: +, -, *, /, ^, %
//   - Comparison operators: =, <>, <, >, <=, >=
//   - Basic functions: SUM, AVERAGE, COUNT, MAX, MIN, IF, etc.
//   - Error propagation: #DIV/0!, #VALUE!, #REF!, etc.
//
// Usage:
//
//	engine := formula.NewEngine(sheet)
//	result := engine.Evaluate("SUM(A1:A10)")
//
// The engine maintains a dependency graph for efficient recalculation
// and supports circular reference detection.
package formula
