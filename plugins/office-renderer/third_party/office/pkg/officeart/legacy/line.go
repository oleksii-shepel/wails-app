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
