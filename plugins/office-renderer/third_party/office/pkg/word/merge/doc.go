// Package merge provides mail-merge field processing for Word documents.
//
// It finds MERGEFIELD fields (and text-based placeholders like {{Name}})
// in a resolved document and replaces them with values from a data source,
// producing a new merged document.
//
// # Basic Usage
//
// Load and resolve a template document, then create a processor with data
// and process the document:
//
//	doc, err := word.Open("template.docx")
//	if err != nil { ... }
//
//	proc := merge.NewProcessor(merge.Data{
//	    "Name":    "Alice Smith",
//	    "Company": "Acme Corp",
//	})
//
//	mergedDoc, err := proc.Process(doc)
//	if err != nil { ... }
//
//	// Render to HTML with merged values
//	html, _, err := render.HTMLWithOptions(mergedDoc, render.HTMLOptions{})
//
// # Multi-Record Mail Merge
//
// For producing multiple documents from a single template:
//
//	records := []merge.Record{
//	    {"Name": "Alice", "Company": "Acme"},
//	    {"Name": "Bob", "Company": "Globex"},
//	}
//	docs, err := merge.MergeDocumentWithRecords(doc, records)
//
// # Field Types Supported
//
// The processor handles two kinds of merge fields:
//
//  1. Native Word MERGEFIELD fields (w:fldChar/w:instrText with
//     MERGEFIELD instruction). These are created by Word's Insert >
//     Quick Parts > Field > MergeField feature.
//
//  2. Text-based placeholders like {{Name}} or «Name» in regular text
//     runs. These are useful for simple templates or when working with
//     documents that don't use Word's native field mechanism.
//
// # Missing Field Handling
//
// By default, when a merge field has no matching data, the processor
// preserves the field's cached text (if any) or falls back to a
// placeholder like «FieldName». You can customize this behavior by
// setting OnMissingField:
//
//	proc.OnMissingField = func(fieldName string) string {
//	    return "[N/A]"
//	}
//
// # Preserving Field Structure
//
// By default, MERGEFIELD runs are replaced with plain text runs. To
// keep the field structure (updating only the cached text), set:
//
//	proc.PreserveFields = true
//
// This is useful when you need to round-trip the document back to
// Word and want the fields to remain editable.
package merge
