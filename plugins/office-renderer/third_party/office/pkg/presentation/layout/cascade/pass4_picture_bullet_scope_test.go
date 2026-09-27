package cascade

import (
	"testing"

	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func Test37625EffectiveTextDefaultsPreservesPictureBulletOwnerScope(t *testing.T) {
	shape := &parts.TextBody{}
	layout := &parts.TextBody{ListStyle: &parts.DMLTextListStyle{Lvl1PPr: &parts.DMLTextStyleLevel{
		BuBlip: &parts.BuBlip{Blip: parts.Blip{Embed: "rId3"}},
	}}}
	got := EffectiveTextDefaults(shape, layout, nil, nil, nil, ir.PlaceholderBody, 0, nil, nil, nil, &ir.Theme{})
	if got.BulletImageRelID != "rId3" || got.BulletImageScope != "layout" {
		t.Fatalf("picture bullet = rel %q scope %q, want rId3/layout", got.BulletImageRelID, got.BulletImageScope)
	}
}
