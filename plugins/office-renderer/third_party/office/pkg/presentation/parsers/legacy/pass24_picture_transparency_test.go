package legacy

import (
	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/drawing"
	"testing"
)

func TestPass24LegacyPictureTransparentColorBecomesCanonicalAlphaColorKey(t *testing.T) {
	effects := parseLegacyPptPictureEffects([]officeartescher.Property{{ID: legacyOfficeArtPropPictureTransparent, Value: 0x000000FF}})
	if len(effects) != 1 {
		t.Fatalf("effects=%#v, want one transparent color-key effect", effects)
	}
	cc, ok := effects[0].(drawing.ClrChange)
	if !ok {
		t.Fatalf("effect=%T, want drawing.ClrChange", effects[0])
	}
	if cc.From != "FF0000" || cc.To != "FF0000" || !cc.UseAlpha || cc.ToAlpha != 0 {
		t.Fatalf("color key=%#v, want red -> red alpha 0", cc)
	}
}

func TestPass24LegacyPictureWithoutTransparentColorHasNoSyntheticEffect(t *testing.T) {
	effects := parseLegacyPptPictureEffects([]officeartescher.Property{{ID: legacyOfficeArtPropPib, Value: 3, IsBlip: true}})
	if len(effects) != 0 {
		t.Fatalf("effects=%#v, want none", effects)
	}
}
