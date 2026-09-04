// Package smartart provides semantic parsing of SmartArt diagrams.
//
// SmartArt (DrawingML Diagrams) consists of:
//   - Data Model (dgm:dataModel): Nodes and relationships
//   - Layout Definition (dgm:layoutDef): Positioning algorithms
//   - Style Definition (dgm:styleDef): Visual styling
//
// This package returns a semantic Diagram model — a tree of nodes with
// parent/child relationships, levels, and layout metadata. Consumers that
// need pre-positioned shapes for rendering should use the render subpackage:
//
//	diagram, err := smartart.ParseDiagram(dataModelXML)
//	shapes := smartartrender.LayoutDiagram(diagram)
//
// Supported diagram types:
//   - Hierarchy (organization charts)
//   - Process (flowcharts)
//   - Cycle (circular relationships)
//   - Matrix (grid layouts)
//   - List (bullet points as shapes)
package smartart
