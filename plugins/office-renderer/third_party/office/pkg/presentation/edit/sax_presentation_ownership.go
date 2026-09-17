package edit

import pasir "office/pkg/presentation/ir"

// transferSAXPresentationSemantics moves presentation/slide semantics that are
// fully decoded by the forward-only stream. ComposePackage is no longer the
// production authority for canvas size, transitions, or notes shape trees.
func transferSAXPresentationSemantics(src, dst *pasir.Presentation) {
	if src == nil || dst == nil {
		return
	}
	if src.SlideWidthEMU != 0 {
		dst.SlideWidthEMU = src.SlideWidthEMU
	}
	if src.SlideHeightEMU != 0 {
		dst.SlideHeightEMU = src.SlideHeightEMU
	}
	for i, slide := range src.Slides {
		if slide == nil || i >= len(dst.Slides) || dst.Slides[i] == nil {
			continue
		}
		if slide.Transition != nil {
			tr := *slide.Transition
			dst.Slides[i].Transition = &tr
		}
		if slide.Notes != nil {
			n := *slide.Notes
			n.Shapes = slide.Notes.Shapes
			dst.Slides[i].Notes = &n
		}
	}
}
