package render

import (
	"office/pkg/officeart/legacy"
	"office/pkg/officeart/legacy/escher"
)

// LegacyHTML composes a legacy Escher tree into the common IR and renders it
// through exactly the same OfficeArt HTML path used by normalized shapes.
func LegacyHTML(node *escher.ShapeNode, store legacy.BStore) (string, error) {
	shape := legacy.ComposeShape(node, store)
	if shape == nil {
		return "", nil
	}
	return HTMLShape(shape)
}
