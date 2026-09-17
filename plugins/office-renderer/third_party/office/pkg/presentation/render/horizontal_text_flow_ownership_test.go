package render

import (
	"os"
	"strings"
	"testing"
)

func TestMeasuredPhysicalLinesDoNotAskBrowserToWrapAgain(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, `white-space:pre;text-indent:`) {
		t.Fatal("measured text lines must materialize final physical rows without browser re-wrapping")
	}
	if strings.Contains(body, `white-space:pre-wrap`) {
		t.Fatal("measured text lines must not wrap a second time in the browser")
	}
}
