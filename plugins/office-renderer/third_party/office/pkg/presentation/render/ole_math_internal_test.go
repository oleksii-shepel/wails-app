package render

import (
	"math"
	playout "office/pkg/presentation/layout"
	"testing"
)

func TestFirstOLEMathSVGPaintAcceptsNamedColorAndSingleQuotes(t *testing.T) {
	svg := `<svg><path fill='yellow' d='M0 0L1 1'/></svg>`
	if got := firstOLEMathSVGPaint(svg, "path"); got != "yellow" {
		t.Fatalf("paint = %q, want yellow", got)
	}
}

func TestFirstOLEMathSVGPaintReadsNamedColorFromStyle(t *testing.T) {
	svg := `<svg><path style='fill:white;stroke:none' d='M0 0L1 1'/></svg>`
	if got := firstOLEMathSVGPaint(svg, "path"); got != "white" {
		t.Fatalf("paint = %q, want white", got)
	}
}

func TestValidOLEMathColorRejectsPaintServersAndCurrentColor(t *testing.T) {
	for _, value := range []string{"none", "transparent", "currentColor", "url(#gradient)"} {
		if validOLEMathColor(value) {
			t.Fatalf("%q should not be used as an inherited equation color", value)
		}
	}
}

func TestDominantOLEMathSVGPaintIgnoresLeadingBookkeepingPath(t *testing.T) {
	svg := `<svg><path fill="black" d="M0 0"/><path fill="white" d="M1 1"/><path fill="white" d="M2 2"/><path fill="white" d="M3 3"/></svg>`
	if got := dominantOLEMathSVGPaint(svg, []string{"path"}); got != "white" {
		t.Fatalf("dominant paint = %q, want white", got)
	}
}

func TestOLEMathForegroundColorPrefersRetainedTextOverOutlinedPaths(t *testing.T) {
	ole := &playout.PhysicalOLEPlan{IconMIME: "image/svg+xml", IconData: []byte(`<svg><path fill="yellow"/><path fill="yellow"/><text fill="white">x</text></svg>`)}
	if got := oleMathForegroundColor(ole); got != "white" {
		t.Fatalf("foreground = %q, want white", got)
	}
}

func TestOLEMathForegroundColorPrefersAuthoredLegacyRecolor(t *testing.T) {
	ole := &playout.PhysicalOLEPlan{
		MathColor: "FFFF00",
		IconMIME:  "image/svg+xml",
		IconData:  []byte(`<svg><path fill="black"/></svg>`),
	}
	if got := oleMathForegroundColor(ole); got != "#FFFF00" {
		t.Fatalf("foreground = %q, want #FFFF00", got)
	}
}

func TestOLEMathPreviewPlacementIncludesLeftInkOrigin(t *testing.T) {
	ole := &playout.PhysicalOLEPlan{IconMIME: "image/svg+xml", IconData: []byte(`<svg viewBox="0 0 100 100"><text x="20" y="60" font-size="20">x</text></svg>`)}
	left, baseline, center, ok := oleMathPreviewPlacement(ole)
	if !ok || left != .2 || baseline != .6 || center != .54 {
		t.Fatalf("placement = left %.3f, baseline %.3f, centre %.3f, ok %t; want .2, .6, .54, true", left, baseline, center, ok)
	}
}

func TestOLEMathPreviewPlacementRestoresCroppedWMFFrameOrigin(t *testing.T) {
	ole := &playout.PhysicalOLEPlan{IconMIME: "image/svg+xml", IconData: []byte(`<svg viewBox="87 0 1674 450" width="100%"><text x="88" y="320" font-size="384">x</text></svg>`)}
	left, _, _, ok := oleMathPreviewPlacement(ole)
	if !ok || math.Abs(left-88.0/1761.0) > 1e-9 {
		t.Fatalf("left origin = %.6f, want %.6f (ok=%t)", left, 88.0/1761.0, ok)
	}
}
