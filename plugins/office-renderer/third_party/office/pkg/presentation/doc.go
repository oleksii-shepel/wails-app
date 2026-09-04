// Package presentation provides the high-level API for opening, creating,
// editing, and rendering PowerPoint presentations.
//
// Open and OpenBytes accept both modern OPC PowerPoint containers (PPTX/PPTM)
// and legacy binary PPT containers and return the shared semantic IR. New and
// Edit expose the mutable editing API. Advanced callers can use the parser,
// resolver, edit, and render subpackages directly when they need lower-level
// control.
package presentation
