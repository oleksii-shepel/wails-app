package fontmetrics

import "strings"

// NamedFace binds a parsed SFNT face to the authored Office family/style identity.
type NamedFace struct {
	Family string
	Weight int
	Italic bool
	Face   *Face
}

// Catalog is a reusable family/style resolver for Office text measurement.
type Catalog struct {
	Faces []NamedFace
}

func (c *Catalog) Empty() bool { return c == nil || len(c.Faces) == 0 }

// Match returns the closest face within the requested family. It never crosses
// family boundaries: callers retain their existing metric fallback when no face
// from the authored family is available.
func (c *Catalog) Match(family string, bold, italic bool) *Face {
	if c == nil {
		return nil
	}
	family = normalizeFamily(family)
	if family == "" {
		return nil
	}
	wantWeight := 400
	if bold {
		wantWeight = 700
	}
	bestScore := int(^uint(0) >> 1)
	var best *Face
	for i := range c.Faces {
		f := &c.Faces[i]
		if f.Face == nil || normalizeFamily(f.Family) != family {
			continue
		}
		weight := f.Weight
		if weight <= 0 {
			weight = 400
		}
		score := absInt(weight - wantWeight)
		if f.Italic != italic {
			score += 1000
		}
		if score < bestScore {
			bestScore = score
			best = f.Face
		}
	}
	return best
}

// MeasureText returns the authored string advance using cmap, hmtx and pair
// positioning. The boolean is false when the requested family/style is not
// available or any non-empty rune cannot be mapped by the face.
func (c *Catalog) MeasureText(text, family string, bold, italic bool, size float64) (float64, bool) {
	return c.MeasureTextWithPairs(text, family, bold, italic, size, true)
}

// MeasureTextWithPairs is MeasureText with explicit pair-positioning control.
// Word uses this to honor w:kern thresholds while presentation/EMF+ can keep
// authored/default pair positioning enabled.
func (c *Catalog) MeasureTextWithPairs(text, family string, bold, italic bool, size float64, pairs bool) (float64, bool) {
	face := c.Match(family, bold, italic)
	if face == nil || face.UnitsPerEm == 0 || size <= 0 {
		return 0, false
	}
	var total float64
	var prev uint16
	havePrev := false
	for _, r := range text {
		gid, ok := face.RuneToGlyph[r]
		if !ok {
			return 0, false
		}
		if havePrev && pairs {
			total += face.PairAdjustment(prev, gid, size)
		}
		adv, ok := face.AdvanceForGlyph(gid, size)
		if !ok {
			return 0, false
		}
		total += adv
		prev, havePrev = gid, true
	}
	return total, true
}

func normalizeFamily(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
