package cascade

import (
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// PlaceholderSources is the inherited placeholder chain for one slide shape.
// Matching belongs to composition: the OOXML resolver should only project the
// selected sources into canonical IR.
type PlaceholderSources struct {
	Layout *parts.Shape
	Master *parts.Shape
}

// ResolvePlaceholderSources selects layout and master ancestors for a slide
// placeholder using PresentationML type/index inheritance rules.
func ResolvePlaceholderSources(layout *parts.SlideLayout, master *parts.SlideMaster, ph *parts.PlaceholderInfo) PlaceholderSources {
	if ph == nil || PlaceholderIndexIsSentinel(ph) {
		return PlaceholderSources{}
	}
	var out PlaceholderSources
	if layout != nil {
		out.Layout = FindPlaceholder(layout.CSld.SpTree.Shapes, ph)
	}
	if master != nil {
		// Placeholder inheritance is a chain, not two independent lookups.
		// A slide placeholder first resolves to its layout placeholder; the
		// layout placeholder then identifies the corresponding master
		// placeholder. Layouts are allowed to remap placeholder type/index, so
		// matching the master directly against the slide placeholder can drop
		// master text styles (font, size, color, paragraph spacing, bullets).
		masterKey := ph
		if out.Layout != nil && out.Layout.NvSpPr.NvPr.Ph != nil {
			masterKey = out.Layout.NvSpPr.NvPr.Ph
		}
		out.Master = FindPlaceholder(master.CSld.SpTree.Shapes, masterKey)
	}
	return out
}

func FindPlaceholder(shapes []parts.Shape, ph *parts.PlaceholderInfo) *parts.Shape {
	if ph == nil || PlaceholderIndexIsSentinel(ph) {
		return nil
	}
	targetKind := PlaceholderKind(ph)
	// For indexed placeholders, idx is the inheritance identity. Layouts may
	// legitimately change the placeholder type while retaining the same idx.
	// Resolve that identity first; kind/type are fallbacks for unindexed/default
	// placeholders.
	if ph.Idx != 0 {
		for i := range shapes {
			candidate := &shapes[i]
			cph := candidate.NvSpPr.NvPr.Ph
			if cph != nil && cph.Idx == ph.Idx {
				return candidate
			}
		}
	}
	for i := range shapes {
		candidate := &shapes[i]
		cph := candidate.NvSpPr.NvPr.Ph
		if cph == nil {
			continue
		}
		if ph.Type != "" && cph.Type == ph.Type && ph.Idx == cph.Idx {
			return candidate
		}
		if PlaceholderKind(cph) == targetKind && (ph.Idx == 0 || cph.Idx == ph.Idx) {
			return candidate
		}
	}
	return nil
}

func FindPlaceholderByKind(shapes []parts.Shape, kind ir.PlaceholderKind) *parts.Shape {
	for i := range shapes {
		candidate := &shapes[i]
		if PlaceholderKind(candidate.NvSpPr.NvPr.Ph) == kind {
			return candidate
		}
	}
	return nil
}

func PlaceholderIndexIsSentinel(ph *parts.PlaceholderInfo) bool {
	if ph == nil || ph.Idx < 0 {
		return false
	}
	return int64(ph.Idx) == int64(^uint32(0))
}

func PlaceholderKind(ph *parts.PlaceholderInfo) ir.PlaceholderKind {
	if ph == nil {
		return ir.PlaceholderNone
	}
	switch ph.Type {
	case "title", "ctrTitle":
		return ir.PlaceholderTitle
	case "body":
		return ir.PlaceholderBody
	case "subTitle":
		return ir.PlaceholderSubtitle
	case "dt":
		return ir.PlaceholderDateTime
	case "sldNum":
		return ir.PlaceholderSlideNumber
	case "ftr":
		return ir.PlaceholderFooter
	case "hdr":
		return ir.PlaceholderHeader
	case "obj":
		return ir.PlaceholderContent
	case "chart":
		return ir.PlaceholderChart
	case "tbl":
		return ir.PlaceholderTable
	case "pic":
		return ir.PlaceholderPicture
	case "media":
		return ir.PlaceholderMedia
	case "clipArt":
		return ir.PlaceholderClipArt
	case "dgm":
		return ir.PlaceholderDiagram
	case "sldImg":
		return ir.PlaceholderSlideImage
	case "":
		if ph.Idx == 0 {
			return ir.PlaceholderTitle
		}
		return ir.PlaceholderBody
	default:
		return ir.PlaceholderOther
	}
}

// HeaderFooterPlaceholderEnabled applies master-level visibility flags before
// inherited footer/date/slide-number placeholders enter the composed scene.
func HeaderFooterPlaceholderEnabled(master *parts.SlideMaster, kind ir.PlaceholderKind) bool {
	if master == nil || master.HF == nil {
		return true
	}
	switch kind {
	case ir.PlaceholderDateTime:
		return master.HF.DateTime == nil || *master.HF.DateTime
	case ir.PlaceholderSlideNumber:
		return master.HF.SlideNumber == nil || *master.HF.SlideNumber
	case ir.PlaceholderFooter:
		return master.HF.Footer == nil || *master.HF.Footer
	default:
		return true
	}
}
