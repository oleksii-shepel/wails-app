package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLSerializerDoesNotReenterPresentationTextLayout(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, forbidden := range []string{
		"ResolveParagraphVisualPlan(",
		"LayoutParagraphLines(",
		"CanLayoutParagraphLines(",
		"LayoutTextFrame(",
		"ResolveTextFrameMetrics(",
		"ResolveTextFlowMetrics(",
		"func renderParagraph(",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("HTML serializer retained text-layout escape hatch %q", forbidden)
		}
	}
}

func TestHTMLTableTextConsumesPhysicalParagraphPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "cell.ParagraphPlans") || !strings.Contains(body, "renderParagraphWithPlan") {
		t.Fatal("table text must consume physical paragraph plans")
	}
}
