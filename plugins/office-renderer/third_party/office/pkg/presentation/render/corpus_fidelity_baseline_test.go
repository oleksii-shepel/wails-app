package render_test

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"office/pkg/presentation/edit"
	playout "office/pkg/presentation/layout"
	"office/pkg/presentation/render"
)

// TestPresentationFixtureCorpusClosesSharedPhysicalBoundary turns the reference
// presentation corpus into a shared layout/render contract. Parser-specific
// tests may assert semantic extraction; this test deliberately starts at the
// canonical IR and verifies the common physical/rendering path for every slide.
func TestPresentationFixtureCorpusClosesSharedPhysicalBoundary(t *testing.T) {
	cases := []struct {
		name   string
		slides int
	}{
		{"sample-1", 23},
		{"sample-2", 10},
		{"sample-3", 10},
		{"sample-4", 10},
		{"sample-5", 7},
		{"sample-8", 1},
		{"sample-9", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(repoPath(t, "testdata", "pptx", tc.name+".pptx"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			edited, err := edit.LoadFromPPTXBytes(data)
			if err != nil {
				t.Fatalf("load fixture: %v", err)
			}
			if got := len(edited.IR.Slides); got != tc.slides {
				t.Fatalf("slides = %d, want reference corpus count %d", got, tc.slides)
			}

			for i, slide := range edited.IR.Slides {
				plan := playout.MaterializePhysicalSlide(slide)
				if plan.Index != slide.Index {
					t.Fatalf("slide %d physical index = %d, canonical index = %d", i+1, plan.Index, slide.Index)
				}
				assertFinitePhysicalVisuals(t, tc.name, i+1, plan.Content)
				assertFinitePhysicalVisuals(t, tc.name, i+1, plan.Decorations)
			}

			html, _, err := render.HTMLWithOptions(edited.IR, render.HTMLOptions{})
			if err != nil {
				t.Fatalf("render fixture: %v", err)
			}
			if got := strings.Count(html, `class="canvas"`); got != tc.slides {
				t.Fatalf("rendered canvases = %d, want %d", got, tc.slides)
			}
			for _, forbidden := range []string{"NaNpt", "Infpt", "-Infpt", "NaNdeg", "Infdeg"} {
				if strings.Contains(html, forbidden) {
					t.Fatalf("renderer emitted non-physical CSS token %q", forbidden)
				}
			}
		})
	}
}

func assertFinitePhysicalVisuals(t *testing.T, fixture string, slide int, visuals []playout.PhysicalVisual) {
	t.Helper()
	for i, visual := range visuals {
		if visual.Drawing == nil {
			continue
		}
		b := visual.Drawing.Paint.Bounds
		values := []struct {
			name string
			v    float64
		}{{"left", b.LeftPt}, {"top", b.TopPt}, {"width", b.WidthPt}, {"height", b.HeightPt}}
		for _, value := range values {
			if math.IsNaN(value.v) || math.IsInf(value.v, 0) {
				t.Fatalf("%s slide %d visual %d has non-finite paint %s: %s", fixture, slide, i, value.name, fmt.Sprint(value.v))
			}
		}
	}
}
