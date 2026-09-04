// Package drawing provides shared DrawingML primitives for OOXML formats.
//
// DrawingML (Drawing Markup Language) is the common vector graphics format
// used across Word (docx), Excel (xlsx), and PowerPoint (pptx). This package
// extracts the shared geometry, color, theme, and transformation logic that
// all three formats need for rendering shapes, lines, connectors, and text boxes.
//
// Shared concepts:
//
//   - Colors and theme-color references
//   - Fills, lines, transparency, and basic effects
//   - 2D transforms, flips, rotation, and grouped coordinate systems
//   - Preset geometry, custom geometry, guides, and path commands
//   - Text-body primitives (bodyPr, paragraph spacing, bullet/number markers)
//   - Theme font/color resolution helpers
//
// The package is designed to be format-agnostic: it knows about DrawingML
// constructs but not about Word sections, Excel worksheets, or PowerPoint slides.
package drawing
