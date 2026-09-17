// Package event defines the format-neutral presentation parser event boundary.
//
// OOXML SAX and legacy PPT record decoders are producers. Canonical IR builders,
// diagnostics and parity gates are consumers. The contract intentionally owns
// presentation semantics rather than XML or binary-record vocabulary.
package event

import "office/pkg/presentation/ir"

type Type string

const (
	StartPresentation Type = "startPresentation"
	EndPresentation   Type = "endPresentation"
	StartSlide        Type = "startSlide"
	EndSlide          Type = "endSlide"
	StartGroup        Type = "startGroup"
	EndGroup          Type = "endGroup"
	Shape             Type = "shape"
	Notes             Type = "notes"
	Comment           Type = "comment"
	Transition        Type = "transition"
)

type Event struct {
	Type         Type
	Format       string // pptx or ppt
	SlideIndex   int
	Depth        int
	Shape        *ir.Shape
	Slide        *ir.Slide
	Presentation *ir.Presentation
}

type Consumer interface{ HandlePresentationEvent(Event) error }
type ConsumerFunc func(Event) error

func (f ConsumerFunc) HandlePresentationEvent(e Event) error { return f(e) }

type Producer interface{ ProducePresentationEvents(Consumer) error }

// EmitPresentation projects canonical presentation semantics onto the shared
// event boundary. It is also the migration bridge for legacy PPT while its
// binary record decoder is progressively moved to emit during record traversal.
func EmitPresentation(format string, p *ir.Presentation, c Consumer) error {
	if p == nil {
		return nil
	}
	if err := c.HandlePresentationEvent(Event{Type: StartPresentation, Format: format, Presentation: p}); err != nil {
		return err
	}
	for i, s := range p.Slides {
		if s == nil {
			continue
		}
		if err := c.HandlePresentationEvent(Event{Type: StartSlide, Format: format, SlideIndex: i, Slide: s}); err != nil {
			return err
		}
		if s.Transition != nil {
			if err := c.HandlePresentationEvent(Event{Type: Transition, Format: format, SlideIndex: i, Slide: s}); err != nil {
				return err
			}
		}
		for _, sh := range s.Shapes {
			if err := emitShape(format, i, 0, sh, c); err != nil {
				return err
			}
		}
		if s.Notes != nil {
			if err := c.HandlePresentationEvent(Event{Type: Notes, Format: format, SlideIndex: i, Slide: s}); err != nil {
				return err
			}
		}
		for range s.Comments {
			if err := c.HandlePresentationEvent(Event{Type: Comment, Format: format, SlideIndex: i, Slide: s}); err != nil {
				return err
			}
		}
		if err := c.HandlePresentationEvent(Event{Type: EndSlide, Format: format, SlideIndex: i, Slide: s}); err != nil {
			return err
		}
	}
	return c.HandlePresentationEvent(Event{Type: EndPresentation, Format: format, Presentation: p})
}

func emitShape(format string, slide, depth int, sh *ir.Shape, c Consumer) error {
	if sh == nil {
		return nil
	}
	if sh.Kind == ir.ShapeGroup {
		if err := c.HandlePresentationEvent(Event{Type: StartGroup, Format: format, SlideIndex: slide, Depth: depth, Shape: sh}); err != nil {
			return err
		}
		for _, child := range sh.Children {
			if err := emitShape(format, slide, depth+1, child, c); err != nil {
				return err
			}
		}
		return c.HandlePresentationEvent(Event{Type: EndGroup, Format: format, SlideIndex: slide, Depth: depth, Shape: sh})
	}
	return c.HandlePresentationEvent(Event{Type: Shape, Format: format, SlideIndex: slide, Depth: depth, Shape: sh})
}
