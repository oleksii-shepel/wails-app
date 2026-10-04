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
	// Layout may provide a measured width/margin for centered or right-aligned
	// lines; otherwise a block's automatic width fills the available frame.
	// In both cases whitespace must remain preformatted so CSS cannot re-wrap it.
	if !strings.Contains(body, `display:block;%swhite-space:pre;text-indent:`) {
		t.Fatal("measured physical line should serialize as a non-rewrapping block")
	}
}
