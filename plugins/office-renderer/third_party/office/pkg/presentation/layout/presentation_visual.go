package layout

import (
	"strings"

	"office/pkg/presentation/ir"
)

// PresentationVisualPlan is the presentation-wide physical style contract.
// Theme interpretation belongs here so renderers consume effective values.
type PresentationVisualPlan struct{ DefaultTextColor string }

func ResolvePresentationVisualPlan(pres *ir.Presentation) PresentationVisualPlan {
	out := PresentationVisualPlan{DefaultTextColor: "000000"}
	if pres == nil || pres.Theme == nil {
		return out
	}
	if color, ok := pres.Theme.ColorScheme["dk1"]; ok {
		color = strings.TrimPrefix(strings.TrimSpace(color), "#")
		if color != "" {
			out.DefaultTextColor = color
		}
	}
	return out
}

// ResolvePhysicalPlaceholderClass projects resolved placeholder identity into
// the physical contract so serializers do not interpret placeholder kinds.
func ResolvePhysicalPlaceholderClass(shape *ir.Shape) string {
	if shape == nil {
		return ""
	}
	switch shape.Placeholder {
	case ir.PlaceholderTitle:
		return "ph-title"
	case ir.PlaceholderBody:
		return "ph-body"
	case ir.PlaceholderSubtitle:
		return "ph-subtitle"
	default:
		return ""
	}
}
