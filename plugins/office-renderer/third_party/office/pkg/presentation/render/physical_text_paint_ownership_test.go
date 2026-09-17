package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLTextPaintConsumesLayoutPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	start := strings.Index(s, "func renderRun(")
	end := strings.Index(s[start:], "\nfunc (ctx *htmlRenderContext) isEmbeddedFamily")
	if start < 0 || end < 0 {
		t.Fatal("renderRun boundary not found")
	}
	body := s[start : start+end]
	for _, forbidden := range []string{"drawingcss.EffectList(run.Effects)", "run.FontSize >", "run.FontFamily !=", "run.Color !=", "run.Highlight !=", "run.Bold {", "run.Italic {", "run.Underline {", "run.Strike {"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderRun retained canonical text-paint interpretation %q", forbidden)
		}
	}
	if !strings.Contains(body, "playout.ResolveRunVisualPlan(run, fontScale)") {
		t.Fatal("renderRun must consume layout run visual plan")
	}
}
