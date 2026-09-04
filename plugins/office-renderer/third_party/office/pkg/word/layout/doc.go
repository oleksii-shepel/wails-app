// Package layout computes explicit geometry for the shared Word IR.
//
// It converts Word formatting properties such as paragraph spacing, line
// spacing, table geometry, pagination constraints, and floating-object anchors
// into point-unit measurements. Renderers can therefore consume layout results
// without depending on OOXML or legacy parser mechanics, or on browser flow as
// a proxy for Word layout.
package layout
