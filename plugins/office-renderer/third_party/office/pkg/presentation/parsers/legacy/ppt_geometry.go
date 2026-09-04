package legacy

import officeartlegacy "office/pkg/officeart/legacy"

// legacyPptShapeTypeToGeometry remains temporarily for source compatibility
// while PPT migrates to the shared OfficeArt frontend.
func legacyPptShapeTypeToGeometry(shapeType int) string {
	if shapeType < 0 || shapeType > 0xFFFF {
		return ""
	}
	return officeartlegacy.ShapeTypeToPreset(uint16(shapeType))
}
