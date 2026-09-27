package legacy

// DashName normalizes the OfficeArt msoLineDashStyle enumeration into the
// DrawingML/CSS-compatible semantic names used by all three legacy adapters.
func DashName(raw uint32) string {
	switch raw {
	case 1:
		return "sysDash"
	case 2:
		return "sysDot"
	case 3:
		return "sysDashDot"
	case 4:
		return "sysDashDotDot"
	case 5:
		return "dot"
	case 6:
		return "dash"
	case 7:
		return "lgDash"
	case 8:
		return "dashDot"
	case 9:
		return "lgDashDot"
	case 10:
		return "lgDashDotDot"
	default:
		return "solid"
	}
}

// LineJoinFromOfficeArt maps MSOLINEJOIN to canonical line-join names.
func LineJoinFromOfficeArt(raw uint32) string {
	switch raw {
	case 0:
		return "bevel"
	case 1:
		return "miter"
	case 2:
		return "round"
	default:
		return "round"
	}
}

// LineCapFromOfficeArt maps MSOLINECAP to canonical cap names.
func LineCapFromOfficeArt(raw uint32) string {
	switch raw {
	case 0:
		return "round"
	case 1:
		return "square"
	case 2:
		return "flat"
	default:
		return "flat"
	}
}
