package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLDoesNotOwnFreeConnectorGeometry(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{"trimSmartArtConnectorTip", "smartArtCurveControlPoints", "playout.ResolveFreeConnectorVisualPlan(shape)"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer retained connector geometry operation %q", forbidden)
		}
	}
	if !strings.Contains(src, "item.FreeConnector") {
		t.Fatal("renderer must consume the precomputed physical free-connector plan")
	}
}
