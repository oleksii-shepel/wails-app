// Package render converts the shared Word IR into supported output forms.
//
// HTML is the only full-document rendering surface. MathML extraction remains
// available for document math. PNG rasterisation and standalone SVG export are
// intentionally out of scope for the Word package, although HTML output may
// embed inline SVG for vector content when needed.
package render
