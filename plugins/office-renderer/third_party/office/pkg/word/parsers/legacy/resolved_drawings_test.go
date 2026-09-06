package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

func TestApplyResolvedWord97DrawingsProjectsCanonicalShape(t *testing.T) {
	shape := &wordir.Shape{Kind: "rect"}
	runs := []docRun{{Text: "a\x08b", Spec: true, StartCP: 10}}
	got := applyResolvedWord97Drawings(runs, map[int]word97.Drawing{11: {Shape: shape}})
	if len(got) != 3 {
		t.Fatalf("runs = %#v, want text/shape/text", got)
	}
	if got[0].Text != "a" || got[0].StartCP != 10 {
		t.Fatalf("prefix = %#v", got[0])
	}
	if got[1].Kind != "shape" || got[1].Shape != shape || got[1].StartCP != 11 {
		t.Fatalf("drawing = %#v", got[1])
	}
	if got[2].Text != "b" || got[2].StartCP != 12 {
		t.Fatalf("suffix = %#v", got[2])
	}
}

func TestApplyResolvedWord97DrawingsLeavesUnresolvedAnchorInText(t *testing.T) {
	runs := []docRun{{Text: "a\x08b", Spec: true, StartCP: 10}}
	got := applyResolvedWord97Drawings(runs, map[int]word97.Drawing{99: {Shape: &wordir.Shape{Kind: "rect"}}})
	if len(got) != 1 || got[0].Text != runs[0].Text {
		t.Fatalf("runs = %#v, want original unresolved anchor run", got)
	}
}

func TestApplyResolvedWord97DrawingsProjectsCanonicalImage(t *testing.T) {
	image := &wordir.Image{MIMEType: "image/png"}
	runs := []docRun{{Text: "\x08", Spec: true, StartCP: 4}}
	got := applyResolvedWord97Drawings(runs, map[int]word97.Drawing{4: {Image: image}})
	if len(got) != 1 || got[0].Kind != "image" || got[0].Image != image || got[0].StartCP != 4 {
		t.Fatalf("runs = %#v", got)
	}
}
