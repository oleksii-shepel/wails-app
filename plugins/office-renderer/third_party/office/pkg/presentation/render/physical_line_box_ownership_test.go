package render

import (
	"os"
	"strings"
	"testing"
)

func TestMeasuredPhysicalLinesSuppressParagraphLineHeightStrut(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, `if plan.LinesMeasured`) || !strings.Contains(body, `pStyles = append(pStyles, "line-height:0")`) {
		t.Fatal("measured physical paragraphs must suppress the browser paragraph line-height strut")
	}
	if !strings.Contains(body, `line-height:%.4fpt;min-height:%.4fpt;overflow:visible;vertical-align:baseline`) {
		t.Fatal("physical lines must preserve layout-owned advance without clipping glyph ink")
	}
}
