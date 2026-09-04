package drawing

import (
	"testing"
)

func TestColorSchemeIndex(t *testing.T) {
	cs := &ColorScheme{
		Dk1:     "111111",
		Lt1:     "FFFFFF",
		Dk2:     "222222",
		Lt2:     "EEEEEE",
		Accent1: "4472C4",
		Accent2: "ED7D31",
		Hlink:   "0563C1",
		FolHlink: "954F72",
	}
	idx := cs.Index()

	// Primary names
	if idx["dk1"] != "111111" {
		t.Errorf("dk1 = %q, want 111111", idx["dk1"])
	}
	if idx["lt1"] != "FFFFFF" {
		t.Errorf("lt1 = %q, want FFFFFF", idx["lt1"])
	}
	if idx["accent1"] != "4472C4" {
		t.Errorf("accent1 = %q, want 4472C4", idx["accent1"])
	}
	if idx["hlink"] != "0563C1" {
		t.Errorf("hlink = %q, want 0563C1", idx["hlink"])
	}
	if idx["folHlink"] != "954F72" {
		t.Errorf("folHlink = %q, want 954F72", idx["folHlink"])
	}

	// Aliases
	if idx["tx1"] != "111111" {
		t.Errorf("tx1 = %q, want 111111", idx["tx1"])
	}
	if idx["text1"] != "111111" {
		t.Errorf("text1 = %q, want 111111", idx["text1"])
	}
	if idx["bg1"] != "FFFFFF" {
		t.Errorf("bg1 = %q, want FFFFFF", idx["bg1"])
	}
	if idx["background1"] != "FFFFFF" {
		t.Errorf("background1 = %q, want FFFFFF", idx["background1"])
	}
	if idx["tx2"] != "222222" {
		t.Errorf("tx2 = %q, want 222222", idx["tx2"])
	}
	if idx["bg2"] != "EEEEEE" {
		t.Errorf("bg2 = %q, want EEEEEE", idx["bg2"])
	}
}

func TestColorSchemeIndexNil(t *testing.T) {
	var cs *ColorScheme
	idx := cs.Index()
	if len(idx) != 0 {
		t.Fatalf("expected empty index for nil, got %d entries", len(idx))
	}
}

func TestColorSchemeIndexEmptySlots(t *testing.T) {
	cs := &ColorScheme{Dk1: "111111"}
	idx := cs.Index()
	if _, ok := idx["accent1"]; ok {
		t.Error("expected accent1 to be absent when empty")
	}
}

func TestResolveThemeColor(t *testing.T) {
	cs := &ColorScheme{Dk1: "111111", Lt1: "FFFFFF"}

	// Primary name
	if got := cs.ResolveThemeColor("dk1"); got != "111111" {
		t.Errorf("ResolveThemeColor(dk1) = %q, want 111111", got)
	}

	// Alias
	if got := cs.ResolveThemeColor("text1"); got != "111111" {
		t.Errorf("ResolveThemeColor(text1) = %q, want 111111", got)
	}

	// Unknown
	if got := cs.ResolveThemeColor("unknown"); got != "" {
		t.Errorf("ResolveThemeColor(unknown) = %q, want empty", got)
	}

	// Case insensitive
	if got := cs.ResolveThemeColor("DK1"); got != "111111" {
		t.Errorf("ResolveThemeColor(DK1) = %q, want 111111", got)
	}

	// Nil scheme
	var nilCS *ColorScheme
	if got := nilCS.ResolveThemeColor("dk1"); got != "" {
		t.Errorf("nil.ResolveThemeColor = %q, want empty", got)
	}
}

func TestApplyColorModifier(t *testing.T) {
	// Empty base
	if got := ApplyColorModifier("", "FF", "FF"); got != "" {
		t.Errorf("empty base = %q, want empty", got)
	}

	// Invalid hex returns unchanged
	if got := ApplyColorModifier("badhex", "FF", "FF"); got != "badhex" {
		t.Errorf("invalid base = %q, want badhex", got)
	}

	// Shade (darken)
	got := ApplyColorModifier("FFFFFF", "80", "")
	if got != "808080" {
		t.Errorf("shade white by 0.5 = %q, want 808080", got)
	}

	// Tint (lighten)
	got2 := ApplyColorModifier("000000", "", "80")
	if got2 != "7F7F7F" {
		t.Errorf("tint black by 0.5 = %q, want 7F7F7F", got2)
	}

	// Both
	got3 := ApplyColorModifier("808080", "FF", "FF")
	if got3 != "808080" {
		t.Errorf("shade+tint full = %q, want 808080", got3)
	}

	// Invalid shade/tint ignored
	got4 := ApplyColorModifier("FFFFFF", "ZZ", "")
	if got4 != "FFFFFF" {
		t.Errorf("invalid shade = %q, want FFFFFF", got4)
	}
}

func TestParseHexPercent(t *testing.T) {
	// 00 -> 0
	got, ok := parseHexPercent("00")
	if !ok || got != 0 {
		t.Errorf("parseHexPercent(00) = %v, %v, want 0, true", got, ok)
	}

	// FF -> 1
	got, ok = parseHexPercent("FF")
	if !ok || got != 1.0 {
		t.Errorf("parseHexPercent(FF) = %v, %v, want 1.0, true", got, ok)
	}

	// 80 -> ~0.5
	got, ok = parseHexPercent("80")
	if !ok || got != 128.0/255.0 {
		t.Errorf("parseHexPercent(80) = %v, %v, want ~0.502, true", got, ok)
	}

	// Invalid
	got, ok = parseHexPercent("ZZ")
	if ok {
		t.Errorf("parseHexPercent(ZZ) = %v, %v, want false", got, ok)
	}
}

func TestParseHexRGBInvalidByte(t *testing.T) {
	_, _, _, ok := parseHexRGB("GGGGGG")
	if ok {
		t.Error("expected false for invalid hex")
	}
	_, _, _, ok2 := parseHexRGB("1G0000")
	if ok2 {
		t.Error("expected false for invalid second byte")
	}
	_, _, _, ok3 := parseHexRGB("001G00")
	if ok3 {
		t.Error("expected false for invalid third byte")
	}
	_, _, _, ok4 := parseHexRGB("00001G")
	if ok4 {
		t.Error("expected false for invalid fourth byte")
	}
}

func TestParseHexRGBWithHash(t *testing.T) {
	r, g, b, ok := parseHexRGB("#FF0000")
	if !ok {
		t.Fatal("expected ok")
	}
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("parseHexRGB(#FF0000) = (%d,%d,%d)", r, g, b)
	}
}

func TestColorSchemeIndexEmptyAccentSlots(t *testing.T) {
	cs := &ColorScheme{Dk1: "1", Lt1: "2", Dk2: "3", Lt2: "4"}
	idx := cs.Index()
	if _, ok := idx["accent5"]; ok {
		t.Error("expected accent5 absent")
	}
	if _, ok := idx["hlink"]; ok {
		t.Error("expected hlink absent")
	}
}

func TestColorSchemeIndexAllSlots(t *testing.T) {
	cs := &ColorScheme{
		Dk1: "11", Lt1: "22", Dk2: "33", Lt2: "44",
		Accent1: "a1", Accent2: "a2", Accent3: "a3",
		Accent4: "a4", Accent5: "a5", Accent6: "a6",
		Hlink: "hh", FolHlink: "fh",
	}
	idx := cs.Index()
	for _, key := range []string{"dk1", "lt1", "dk2", "lt2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hlink", "folHlink", "tx1", "bg1", "tx2", "bg2"} {
		if _, ok := idx[key]; !ok {
			t.Errorf("expected %s in index", key)
		}
	}
}

func TestResolveThemeColorEmptyAlias(t *testing.T) {
	cs := &ColorScheme{Dk1: "1", Lt1: "2"}
	// tx2/bg2 aliases when Dk2/Lt2 are empty
	if got := cs.ResolveThemeColor("tx2"); got != "" {
		t.Errorf("tx2 empty = %q, want empty", got)
	}
	if got := cs.ResolveThemeColor("bg2"); got != "" {
		t.Errorf("bg2 empty = %q, want empty", got)
	}
}

func TestResolveThemeColorNilReceiver(t *testing.T) {
	var cs *ColorScheme
	if got := cs.ResolveThemeColor("accent1"); got != "" {
		t.Errorf("nil receiver = %q, want empty", got)
	}
}

func TestColorSchemeIndexNilReceiver(t *testing.T) {
	var cs *ColorScheme
	idx := cs.Index()
	if idx == nil {
		t.Error("expected non-nil map from nil receiver")
	}
	if len(idx) != 0 {
		t.Errorf("expected empty map, got %v", idx)
	}
}

func TestResolveThemeColorAliases(t *testing.T) {
	cs := &ColorScheme{
		Dk1: "111111",
		Lt1: "EEEEEE",
		Dk2: "222222",
		Lt2: "DDDDDD",
	}
	tests := []struct {
		slot, want string
	}{
		{"tx1", "111111"},
		{"text1", "111111"},
		{"bg1", "EEEEEE"},
		{"background1", "EEEEEE"},
		{"tx2", "222222"},
		{"text2", "222222"},
		{"bg2", "DDDDDD"},
		{"background2", "DDDDDD"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		if got := cs.ResolveThemeColor(tt.slot); got != tt.want {
			t.Errorf("ResolveThemeColor(%q) = %q, want %q", tt.slot, got, tt.want)
		}
	}
}

func TestResolveThemeColorSwitchAliases(t *testing.T) {
	// When Dk1/Lt1 are empty, the index won't include tx1/bg1 aliases,
	// so ResolveThemeColor falls through to the switch cases.
	cs := &ColorScheme{
		Dk2: "222222",
		Lt2: "DDDDDD",
	}
	// Dk1 and Lt1 are empty, so tx1/text1/bg1/background1 won't be in index
	if got := cs.ResolveThemeColor("tx1"); got != "" {
		t.Errorf("tx1 = %q, want empty", got)
	}
	if got := cs.ResolveThemeColor("text1"); got != "" {
		t.Errorf("text1 = %q, want empty", got)
	}
	if got := cs.ResolveThemeColor("bg1"); got != "" {
		t.Errorf("bg1 = %q, want empty", got)
	}
	if got := cs.ResolveThemeColor("background1"); got != "" {
		t.Errorf("background1 = %q, want empty", got)
	}
}
