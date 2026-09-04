package edit

import (
	"strings"
	"unicode"

	wordir "office/pkg/word/ir"
)

// DocumentStats holds computed statistics for a document.
type DocumentStats struct {
	Paragraphs    int // body paragraphs only
	Tables        int // body tables only
	Runs          int // runs in body paragraphs only
	Words         int // word count across body text
	Chars         int // character count including spaces
	CharsNoSpaces int // character count excluding spaces
	Images        int // inline images in body only
	Fields        int // inline fields in body only
}

// Statistics computes fresh statistics from the document body content.
// It does not include headers, footers, footnotes, endnotes, or comments.
func (d *Document) Statistics() DocumentStats {
	if d == nil || d.IR == nil {
		return DocumentStats{}
	}
	var s DocumentStats
	for _, b := range d.IR.Blocks {
		switch b.Kind {
		case "paragraph":
			s.Paragraphs++
			if b.Paragraph == nil {
				continue
			}
			for _, r := range b.Paragraph.Runs {
				s.Runs++
				if r.Image != nil {
					s.Images++
				}
				if r.FieldKind != "" {
					s.Fields++
				}
				text := r.Text
				s.Chars += len(text)
				for _, ch := range text {
					if !unicode.IsSpace(ch) {
						s.CharsNoSpaces++
					}
				}
				// Word count: split on whitespace.
				fields := strings.Fields(text)
				s.Words += len(fields)
			}
		case "table":
			s.Tables++
		}
	}
	return s
}

// Signatures returns the digital signatures discovered when the document was
// loaded, or nil if none. The signatures are read-only.
func (d *Document) Signatures() []wordir.DigitalSignature {
	if d == nil || d.IR == nil {
		return nil
	}
	return d.IR.DigitalSignatures
}

// HasSignatures reports whether the source package contained any digital
// signature origin part.
func (d *Document) HasSignatures() bool {
	if d == nil || d.IR == nil {
		return false
	}
	return d.IR.HasSignatures
}