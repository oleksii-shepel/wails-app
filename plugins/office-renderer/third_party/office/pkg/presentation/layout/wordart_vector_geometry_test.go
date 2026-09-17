package layout

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	"office/pkg/presentation/ir"
)

func TestWordArtPhysicalGeometryMaterializesPresetPaths(t *testing.T) {
	cases := []struct {
		warp string
		want string
	}{
		{"textArchUpCurve", "Q 50 5"},
		{"textArchDownCurve", "Q 50 95"},
		{"textCircleCurve", "A 45 45"},
		{"textWave1", "C 18 18"},
		{"textTriangle", "L 50 20"},
	}
	for _, tc := range cases {
		t.Run(tc.warp, func(t *testing.T) {
			shape := &ir.Shape{OfficeArt: &officeartir.Shape{GeometryText: &officeartir.GeometryText{Text: "Warped", Warp: tc.warp}}}
			plan := ResolveTextGeometryVisualPlan(shape)
			if !plan.Vector || plan.PathData == "" || !contains(plan.PathData, tc.want) {
				t.Fatalf("warp %q physical path = %q, want fragment %q", tc.warp, plan.PathData, tc.want)
			}
		})
	}
}

func TestWordArtVerticalOwnsPhysicalBaseline(t *testing.T) {
	shape := &ir.Shape{OfficeArt: &officeartir.Shape{GeometryText: &officeartir.GeometryText{Text: "Vertical", Warp: "textWave1", Vertical: true}}}
	plan := ResolveTextGeometryVisualPlan(shape)
	if plan.PathData != "M 50 5 L 50 95" {
		t.Fatalf("vertical WordArt path = %q", plan.PathData)
	}
}

func contains(s, part string) bool {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return true
		}
	}
	return false
}
