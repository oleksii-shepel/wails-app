package legacy

import word97 "office/pkg/word/parsers/doc/word97"

// applyResolvedWord97Drawings projects the already-decoded Word97 drawing map
// onto a materialized legacy run slice. Drawing ownership and OfficeArt
// semantics are resolved before this point; this helper only performs the
// canonical anchor-character stream transformation for the full-buffer
// compatibility path.
func applyResolvedWord97Drawings(runs []docRun, drawings map[int]word97.Drawing) []docRun {
	if len(runs) == 0 || len(drawings) == 0 {
		return runs
	}
	out := make([]docRun, 0, len(runs))
	for _, r := range runs {
		if r.Kind != "" || !r.Spec || !containsLegacyDrawingAnchorRune(r.Text) {
			out = append(out, r)
			continue
		}
		runes := []rune(r.Text)
		segmentStart := 0
		segmentStartCP := r.StartCP
		cp := r.StartCP
		resolvedAny := false
		for i, ch := range runes {
			if ch != legacyDrawingAnchorChar {
				cp++
				continue
			}
			drawing, ok := drawings[cp]
			if !ok || (drawing.Image == nil && drawing.Shape == nil) {
				cp++
				continue
			}
			resolvedAny = true
			if i > segmentStart {
				out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:i]), segmentStartCP))
			}
			projected := docRun{
				Format:     r.Format,
				ParaFormat: r.ParaFormat,
				StartCP:    cp,
				Revision:   r.Revision,
			}
			if drawing.Image != nil {
				projected.Kind = "image"
				projected.Image = drawing.Image
			} else {
				projected.Kind = "shape"
				projected.Shape = drawing.Shape
			}
			out = append(out, projected)
			cp++
			segmentStart = i + 1
			segmentStartCP = cp
		}
		if !resolvedAny {
			out = append(out, r)
			continue
		}
		if segmentStart < len(runes) {
			out = append(out, cloneDocRunWithText(r, string(runes[segmentStart:]), segmentStartCP))
		}
	}
	return out
}
