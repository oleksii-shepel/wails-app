package cascade

import (
	"testing"

	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestBuildThemeColorMapAppliesPresentationAliases(t *testing.T) {
	base := map[string]string{"lt1": "FFFFFF", "dk1": "000000", "accent1": "112233"}
	got := BuildThemeColorMap(base, &parts.ColorMap{Bg1: "dk1", Tx1: "lt1", Accent1: "accent1"})
	if got["bg1"] != "000000" || got["tx1"] != "FFFFFF" || got["accent1"] != "112233" {
		t.Fatalf("effective color map = %#v", got)
	}
}

func TestApplyColorMapOverrideRespectsMasterMapping(t *testing.T) {
	inherited := map[string]string{"bg1": "AAAAAA"}
	got := ApplyColorMapOverride(map[string]string{"lt1": "FFFFFF"}, inherited, &parts.ColorMapOverride{MasterClrMapping: &parts.MasterColorMapping{}})
	if got["bg1"] != "AAAAAA" {
		t.Fatalf("master color mapping changed inherited colors: %#v", got)
	}
}

func TestApplyThemeOverrideDoesNotMutateBase(t *testing.T) {
	base := &parts.Theme{Name: "base"}
	override := &parts.ThemeOverride{ClrScheme: &parts.ColorScheme{}}
	got := ApplyThemeOverride(base, override)
	if got == base {
		t.Fatal("theme override must create an effective theme")
	}
	if base.Name != "base" || got.Name != "base" {
		t.Fatal("theme metadata was not preserved")
	}
}
