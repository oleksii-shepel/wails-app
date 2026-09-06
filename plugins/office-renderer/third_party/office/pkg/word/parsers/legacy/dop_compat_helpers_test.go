package legacy

func legacyDopEndnotePosition(epc byte) string {
	switch epc {
	case 0:
		return "sectEnd"
	case 3:
		return "docEnd"
	default:
		return ""
	}
}

func legacyDopNoteRestart(restart byte) string {
	switch restart {
	case 0:
		return "continuous"
	case 1:
		return "eachSect"
	case 2:
		return "eachPage"
	default:
		return ""
	}
}
