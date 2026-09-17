package render

import (
	"os"
	"strings"
	"testing"
)

func TestRendererDoesNotRetainSupersededFormattingHelpers(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, name := range []string{
		"func imageEffectNames(",
		"func emuCSSPt(",
		"func emuIntCSSPt(",
		"func points100CSSPt(",
		"func textAnchorJustify(",
		"func emuToPt(",
	} {
		if strings.Contains(source, name) {
			t.Fatalf("renderer retained superseded helper %q", name)
		}
	}
}
