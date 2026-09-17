package render

import (
	"os"
	"strings"
	"testing"
)

func TestSmartArtSerializationConsumesPhysicalParagraphPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderSmartArtShape(")
	end := strings.Index(src[start:], "func renderSmartArtVectorShape(")
	if start < 0 || end < 0 {
		t.Fatal("SmartArt renderer not found")
	}
	body := src[start : start+end]
	if strings.Contains(body, "renderParagraph(b, p,") {
		t.Fatal("SmartArt renderer must not resolve paragraph layout")
	}
	if !strings.Contains(body, "renderParagraphWithPlan(b, p, textPlan.Paragraphs[i]") {
		t.Fatal("SmartArt renderer must consume physical paragraph plans")
	}
}
