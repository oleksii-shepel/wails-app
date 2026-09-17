// Package layout implements PPTX scene composition and inheritance resolution.
//
// It sits between the parts parser and the renderer, providing:
//   - A clear separation between parse, resolve, and render phases
//   - Exact inheritance of fills, lines, text defaults, and placeholders
//   - Preservation of tree order and z-order across master, layout, and slide content
//   - Auditable defaults for debugging render mismatches
//
// The layout package transforms raw parts.Package into a composed Scene
// where each slide's visual tree is fully resolved with inherited properties.
package layout
