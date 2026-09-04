// Package parts implements the typed WordprocessingML package layer.
//
// It sits directly above OPC and is the correct API when package-preserving
// OOXML round-trip fidelity matters. Unknown OPC parts, relationship chains,
// unsupported extension XML, and other opaque producer data are retained here
// even when they are intentionally absent from the semantic word/ir model.
package parts
