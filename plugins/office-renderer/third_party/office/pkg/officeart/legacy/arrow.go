package legacy

type ArrowKind string
type ArrowSize string

const (
	ArrowNone     ArrowKind = ""
	ArrowTriangle ArrowKind = "triangle"
	ArrowStealth  ArrowKind = "stealth"
	ArrowDiamond  ArrowKind = "diamond"
	ArrowOval     ArrowKind = "oval"
	ArrowOpen     ArrowKind = "arrow"

	ArrowSmall  ArrowSize = "sm"
	ArrowMedium ArrowSize = "med"
	ArrowLarge  ArrowSize = "lg"
)

// ArrowKindFromOfficeArt maps OfficeArt line endpoint enumeration values used
// by legacy binary formats to the shared semantic names.
func ArrowKindFromOfficeArt(raw uint32) ArrowKind {
	switch raw {
	case 1:
		return ArrowTriangle
	case 2:
		return ArrowStealth
	case 3:
		return ArrowDiamond
	case 4:
		return ArrowOval
	case 5:
		return ArrowOpen
	default:
		return ArrowNone
	}
}

// PPTArrowKind handles PowerPoint's historic one-based offset used by its
// existing parser path. This compatibility helper can disappear after PPT is
// fully routed through the shared Escher frontend.
func PPTArrowKind(raw uint32) ArrowKind {
	switch raw {
	case 2:
		return ArrowTriangle
	case 3:
		return ArrowOpen
	case 4:
		return ArrowStealth
	case 5:
		return ArrowDiamond
	case 6:
		return ArrowOval
	default:
		return ArrowNone
	}
}

func ArrowSizeFromOfficeArt(raw uint32) ArrowSize {
	switch raw {
	case 0:
		return ArrowSmall
	case 2:
		return ArrowLarge
	default:
		return ArrowMedium
	}
}

func PPTArrowSize(raw uint32) ArrowSize {
	switch raw {
	case 1:
		return ArrowSmall
	case 2:
		return ArrowMedium
	case 3:
		return ArrowLarge
	default:
		return ""
	}
}
