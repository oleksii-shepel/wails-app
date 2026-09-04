// Package word is the stable, high-level entry point for working with Word
// documents.
//
// The package intentionally keeps the common path small:
//
//	doc, err := word.Open("report.docx")
//	if err != nil {
//		// handle error
//	}
//	html, err := word.HTML(doc)
//
// Both OOXML Word documents (.docx, .docm, .dotx, .dotm) and legacy binary
// Word documents (.doc) resolve to the same [Document] intermediate
// representation. Use [Edit] or [New] when mutation is required.
//
// Advanced callers can use the lower-level subpackages directly:
//   - ir contains the stable format-neutral Word model;
//   - edit provides mutation and DOC/DOCX authoring;
//   - parsers/ooxml/parts exposes package-preserving OOXML access;
//   - parsers/ooxml/sax provides streaming WordprocessingML events;
//   - parsers/legacy exposes lossless binary DOC provenance;
//   - render, layout, and merge provide specialized processing layers.
//
// # Preservation boundary
//
// Open resolves a document into semantic IR. Saving that IR through edit
// regenerates a Word package from the semantic model. If an application must
// retain unknown/vendor OOXML package parts byte-for-byte, use the typed
// parsers/ooxml/parts load/write path instead; that layer preserves opaque OPC
// parts and unsupported XML extensions that are intentionally outside the IR.
package word
