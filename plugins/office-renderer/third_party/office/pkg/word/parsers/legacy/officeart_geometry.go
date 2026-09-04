package legacy

import officeartlegacy "office/pkg/officeart/legacy"

// legacyOfficeArtShapeTypeToGeometry remains temporarily for source
// compatibility while DOC migrates to the shared OfficeArt frontend.
func legacyOfficeArtShapeTypeToGeometry(shapeType uint16) string {
	return officeartlegacy.ShapeTypeToPreset(shapeType)
}
