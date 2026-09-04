package edit

import (
	"fmt"

	wordir "office/pkg/word/ir"
)

// SignatureLineOptions configures a new signature line to be inserted.
type SignatureLineOptions struct {
	SuggestedSigner        string
	SuggestedSigner2       string
	SuggestedSignerEmail   string
	SigningInstructions    string
	AllowComments          bool
	ShowSignDate           bool
	ProviderID             string
	SigningInstructionsSet bool
	ProviderURL            string
}

// AddSignatureLine inserts a signature line into the given paragraph.
// It appends a run containing a VML-based signature line shape.
func (d *Document) AddSignatureLine(p *Paragraph, opts SignatureLineOptions) error {
	if d == nil || d.IR == nil || p == nil || p.ir == nil {
		return fmt.Errorf("nil document or paragraph")
	}

	sl := &wordir.SignatureLine{
		SuggestedSigner:        opts.SuggestedSigner,
		SuggestedSigner2:       opts.SuggestedSigner2,
		SuggestedSignerEmail:   opts.SuggestedSignerEmail,
		SigningInstructions:    opts.SigningInstructions,
		AllowComments:          opts.AllowComments,
		ShowSignDate:           opts.ShowSignDate,
		IsSignatureLine:        true,
		ProviderID:             opts.ProviderID,
		SigningInstructionsSet: opts.SigningInstructionsSet,
		ProviderURL:            opts.ProviderURL,
	}

	run := wordir.Run{
		Kind:          "shape",
		Text:          "",
		SignatureLine: sl,
		Shape: &wordir.Shape{
			Kind:      "rect",
			Width:     192 * 12700, // ~192pt default width
			Height:    96 * 12700,  // ~96pt default height
			FillColor: "#FFFFFF",
			StrokeColor: "#000000",
		},
	}

	p.ir.Runs = append(p.ir.Runs, run)
	return nil
}

// SignatureLines returns all signature lines found in the document body and
// stories (headers, footers, footnotes, endnotes, comments).
func (d *Document) SignatureLines() []wordir.SignatureLine {
	if d == nil || d.IR == nil {
		return nil
	}
	var out []wordir.SignatureLine

	add := func(runs []wordir.Run) {
		for _, r := range runs {
			if r.SignatureLine != nil {
				out = append(out, *r.SignatureLine)
			}
		}
	}

	for _, b := range d.IR.Blocks {
		if b.Paragraph != nil {
			add(b.Paragraph.Runs)
		}
	}
	for _, s := range d.headerPtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs)
		}
	}
	for _, s := range d.footerPtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs)
		}
	}
	for _, s := range d.footnotePtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs)
		}
	}
	for _, s := range d.endnotePtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs)
		}
	}
	for _, s := range d.commentPtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs)
		}
	}

	return out
}

// HasSignatureLines reports whether the document contains any signature lines.
func (d *Document) HasSignatureLines() bool {
	return len(d.SignatureLines()) > 0
}