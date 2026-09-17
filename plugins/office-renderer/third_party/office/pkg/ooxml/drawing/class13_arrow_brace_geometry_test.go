package drawing

import (
	"fmt"
	"testing"
)

func TestClass13ArrowFamiliesOwnDistinctGeometry(t *testing.T) {
	names := []string{"bentUpArrow", "bentArrow", "uturnArrow", "stripedRightArrow", "notchedRightArrow"}
	seen := map[string]string{}
	for _, name := range names {
		p, err := (&PresetGeometry{Name: name, Adjustments: []float64{.22, .31, .38, .05}}).GetPath(216, 108)
		if err != nil || len(p.Commands) == 0 {
			t.Fatalf("%s: empty/error: %v", name, err)
		}
		sig := pathCommandSignature(p)
		if prev, ok := seen[sig]; ok {
			t.Fatalf("%s reuses %s geometry", name, prev)
		}
		seen[sig] = name
	}
}

func TestClass13BracketBracePresetsAreClosedFillGeometry(t *testing.T) {
	for _, bounds := range [][2]float64{{216, 108}, {108, 216}} {
		for _, name := range []string{"leftBracket", "rightBracket", "bracketPair", "leftBrace", "rightBrace", "bracePair"} {
			p, err := (&PresetGeometry{Name: name, Adjustments: []float64{.24}}).GetPath(bounds[0], bounds[1])
			if err != nil || len(p.Commands) == 0 {
				t.Fatalf("%s: %v", name, err)
			}
			closed := false
			for _, c := range p.Commands {
				if c.Type == "Z" {
					closed = true
					break
				}
			}
			if !closed {
				t.Fatalf("%s must be closed fill geometry", name)
			}
		}
	}
}

func TestClass13PresetAdjustmentsChangeGeometry(t *testing.T) {
	for _, name := range []string{"bentUpArrow", "bentArrow", "uturnArrow", "stripedRightArrow", "notchedRightArrow", "leftBracket", "leftBrace"} {
		a, _ := (&PresetGeometry{Name: name, Adjustments: []float64{.12, .18, .22, .03}}).GetPath(216, 108)
		b, _ := (&PresetGeometry{Name: name, Adjustments: []float64{.38, .42, .55, .08}}).GetPath(216, 108)
		if pathCommandSignature(a) == pathCommandSignature(b) {
			t.Fatalf("%s ignores adjustments", name)
		}
	}
}

func pathCommandSignature(p Path) string {
	b := make([]byte, 0, 256)
	for _, c := range p.Commands {
		b = append(b, c.Type...)
		for _, pt := range c.Points {
			b = append(b, []byte(fmtFloat(pt.X))...)
			b = append(b, ',')
			b = append(b, []byte(fmtFloat(pt.Y))...)
		}
		for _, a := range c.Args {
			b = append(b, []byte(fmtFloat(a))...)
		}
	}
	return string(b)
}

func fmtFloat(v float64) string { return fmt.Sprintf("%.4f", v) }
