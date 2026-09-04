// Package drawingml normalizes the shared OOXML DrawingML model into the
// canonical OfficeArt IR. DOCX, XLSX and PPTX keep their package/relationship
// parsing in their respective frontends, while fill/line/effect/geometry/text
// semantics converge here before rendering.
package drawingml
