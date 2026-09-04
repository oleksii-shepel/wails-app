// Package officeevent provides a shared Office-wide event envelope and
// dispatcher over the native Word, Spreadsheet, and Presentation streaming walkers.
//
// It does not replace format-specific event vocabularies. Instead, it wraps the
// native `pkg/word/parsers/ooxml/sax`, `pkg/sheet/parsers/ooxml/sax`, and
// `pkg/presentation/parsers/ooxml/sax` streams in a common envelope so generic
// tooling can observe events from multiple Office package families through one
// handler surface.
package officeevent
