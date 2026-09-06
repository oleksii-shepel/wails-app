package legacy

import "testing"

func TestLegacyBookmarkEndCPByNameInvertsCanonicalEnds(t *testing.T) {
	got := legacyBookmarkEndCPByName(map[int][]string{
		7:  {"alpha", "beta"},
		12: {"gamma"},
	})
	want := map[string]int{"alpha": 7, "beta": 7, "gamma": 12}
	if len(got) != len(want) {
		t.Fatalf("bookmark ends = %#v, want %#v", got, want)
	}
	for name, cp := range want {
		if got[name] != cp {
			t.Fatalf("bookmark %q end = %d, want %d", name, got[name], cp)
		}
	}
}

func TestMaterializeLegacyWord97ParseDataRejectsNilCore(t *testing.T) {
	if got := materializeLegacyWord97ParseData(nil, nil); got != nil {
		t.Fatalf("parse data = %#v, want nil", got)
	}
}
