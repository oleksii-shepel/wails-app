package legacy

import "office/pkg/officeart/legacy/escher"

// PictureRef returns the OfficeArt pib/BStore index from a shape's properties.
func PictureRef(props []escher.Property) (uint32, bool) {
	// OfficeArt pib is property 0x0104.
	const propPib uint16 = 0x0104
	for _, prop := range props {
		if prop.ID == propPib && !prop.IsComplex {
			return prop.Value, prop.Value != 0
		}
	}
	return 0, false
}

// fillBlipRef returns the BLIP index an OfficeArt fill paints with. Texture and
// picture fills reference the picture store through fillBlip (0x0186), which is
// separate from the shape's own picture index (pib).
func fillBlipRef(props []escher.Property) uint32 {
	for _, prop := range props {
		if prop.ID == escher.PropFillBlip && !prop.IsComplex {
			return prop.Value
		}
	}
	return 0
}

// ResolveShapeMedia follows a shape's pib reference through a parsed BStore.
func ResolveShapeMedia(node *escher.ShapeNode, store BStore) (Media, bool) {
	if node == nil || len(store) == 0 {
		return Media{}, false
	}
	ref, ok := PictureRef(node.Properties)
	if !ok {
		return Media{}, false
	}
	return store.Media(ref)
}
