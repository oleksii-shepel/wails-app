package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestParsePptFileUsesSingleEventDrivenVisualPass(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parsePptFile(")
	end := strings.Index(source[start:], "func buildLegacyPptTextOnlySlides(")
	if start < 0 || end < 0 {
		t.Fatal("could not isolate parsePptFile production path")
	}
	body := source[start : start+end]
	if !strings.Contains(body, "parseLegacyPptSlideVisualsEventDriven(") {
		t.Fatal("production legacy parser must construct slide visuals through the event-driven slide pass")
	}
	for _, forbidden := range []string{
		"parseLegacyPptSlideTextShapes(",
		"parseLegacyPptSlideTableShapes(",
		"parseLegacyPptSlideImageShapes(",
		"parseLegacyPptSlideConnectorShapes(",
		"parseLegacyPptSlideGroupShapes(",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("production legacy parser still reopens slide records through %s", forbidden)
		}
	}
}
