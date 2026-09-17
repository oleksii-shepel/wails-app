package render

import (
	"os"
	"strings"
	"testing"
)

func TestComposedTextSerializesPhysicalRunPaint(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, "renderRunWithPlan(b, &run, fragment.Visual, ctx)") {
		t.Fatal("composed physical text fragments must serialize their layout-owned run visual plan")
	}
	if strings.Contains(body, "renderRun(b, &run, fontScale, ctx)") {
		t.Fatal("composed physical text fragments must not re-resolve run paint in the renderer")
	}
}
