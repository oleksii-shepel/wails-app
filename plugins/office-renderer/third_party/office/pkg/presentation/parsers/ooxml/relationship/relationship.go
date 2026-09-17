// Package relationship owns PresentationML relationship type normalization.
// It is shared by typed OOXML loading and the streaming SAX reader so Strict
// and Transitional packages have one relationship interpretation path.
package relationship

import "strings"

const (
	StrictOfficeBase       = "http://purl.oclc.org/ooxml/officeDocument/relationships/"
	TransitionalOfficeBase = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"

	OfficeDocument = TransitionalOfficeBase + "officeDocument"
	Slide          = TransitionalOfficeBase + "slide"
	SlideMaster    = TransitionalOfficeBase + "slideMaster"
	SlideLayout    = TransitionalOfficeBase + "slideLayout"
	Theme          = TransitionalOfficeBase + "theme"
	ThemeOverride  = TransitionalOfficeBase + "themeOverride"
	Font           = TransitionalOfficeBase + "font"
	Image          = TransitionalOfficeBase + "image"
	Audio          = TransitionalOfficeBase + "audio"
	Video          = TransitionalOfficeBase + "video"
	NotesMaster    = TransitionalOfficeBase + "notesMaster"
	NotesSlide     = TransitionalOfficeBase + "notesSlide"
	Comments       = TransitionalOfficeBase + "comments"
	CommentAuthors = TransitionalOfficeBase + "commentAuthors"
	Hyperlink      = TransitionalOfficeBase + "hyperlink"
	TableStyles    = TransitionalOfficeBase + "tableStyles"
	OLEObject      = TransitionalOfficeBase + "oleObject"
	Package        = TransitionalOfficeBase + "package"
)

// CanonicalType maps Strict relationship URIs to their Transitional semantic
// identity. Unknown relationship types are preserved verbatim (apart from
// surrounding whitespace).
func CanonicalType(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, StrictOfficeBase) {
		return TransitionalOfficeBase + strings.TrimPrefix(value, StrictOfficeBase)
	}
	return value
}

func Is(value, expected string) bool { return CanonicalType(value) == expected }
