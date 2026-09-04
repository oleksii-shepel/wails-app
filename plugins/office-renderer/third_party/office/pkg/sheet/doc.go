// Package sheet provides the high-level Spreadsheet API for XLSX-family and
// legacy XLS workbooks.
//
// Common workflows should start here: Open/OpenBytes for reading, HTML for
// browser rendering, Edit/New for in-memory mutation, NewStreamWriter for
// bounded-memory XLSX generation, and Walk* for event-driven XLSX processing.
// Advanced callers can use the edit, parsers, render, stream, and ir
// subpackages directly when they need lower-level control.
//
// PNG and standalone SVG export are intentionally out of scope. Spreadsheet
// vector graphics are rendered inline as part of HTML output.
package sheet
