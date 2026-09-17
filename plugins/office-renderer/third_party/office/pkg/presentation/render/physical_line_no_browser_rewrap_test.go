package render

import (
	"os"
	"strings"
	"testing"
)

func TestPhysicalLinesDoNotAskBrowserToRewrapMeasuredText(t *testing.T) {
	src, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if strings.Contains(body, `ppt-physical-line\" style=\"display:block;width:100%%;white-space:pre-wrap`) {
		t.Fatal("measured physical lines must not be browser-rewrapped after layout has chosen breaks")
	}
	if !strings.Contains(body, `display:block;width:100%%;white-space:pre;text-indent:`) {
		t.Fatal("measured physical line should serialize as a non-rewrapping block")
	}
}
