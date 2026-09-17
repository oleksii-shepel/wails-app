package cascade

import (
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func phShape(typ string, idx int) parts.Shape {
	return parts.Shape{NvSpPr: parts.NonVisualShapeProps{NvPr: parts.PlaceholderPr{Ph: &parts.PlaceholderInfo{Type: typ, Idx: idx}}}}
}

func TestResolvePlaceholderSourcesUsesLayoutAndMasterChain(t *testing.T) {
	layout := &parts.SlideLayout{}
	master := &parts.SlideMaster{}
	layout.CSld.SpTree.Shapes = []parts.Shape{phShape("body", 7)}
	master.CSld.SpTree.Shapes = []parts.Shape{phShape("body", 7)}
	got := ResolvePlaceholderSources(layout, master, &parts.PlaceholderInfo{Type: "body", Idx: 7})
	if got.Layout == nil || got.Master == nil {
		t.Fatalf("sources = %+v, want layout and master", got)
	}
}

func TestPlaceholderSentinelDoesNotInherit(t *testing.T) {
	shapes := []parts.Shape{phShape("body", 1)}
	if got := FindPlaceholder(shapes, &parts.PlaceholderInfo{Type: "body", Idx: int(^uint32(0))}); got != nil {
		t.Fatal("sentinel placeholder inherited unexpectedly")
	}
}

func TestPlaceholderKindFallback(t *testing.T) {
	if got := PlaceholderKind(&parts.PlaceholderInfo{Idx: 0}); got != ir.PlaceholderTitle {
		t.Fatalf("idx 0 = %v", got)
	}
	if got := PlaceholderKind(&parts.PlaceholderInfo{Idx: 2}); got != ir.PlaceholderBody {
		t.Fatalf("idx 2 = %v", got)
	}
}

func TestResolvePlaceholderSourcesUsesLayoutPlaceholderAsMasterKey(t *testing.T) {
	layout := &parts.SlideLayout{}
	master := &parts.SlideMaster{}
	// The slide points at layout idx 7. The layout template remaps that
	// placeholder to the master title placeholder. Master lookup must therefore
	// follow the resolved layout placeholder rather than reusing the slide key.
	layoutShape := phShape("title", 7)
	layout.CSld.SpTree.Shapes = []parts.Shape{layoutShape}
	master.CSld.SpTree.Shapes = []parts.Shape{phShape("title", 7), phShape("body", 7)}
	got := ResolvePlaceholderSources(layout, master, &parts.PlaceholderInfo{Type: "body", Idx: 7})
	if got.Layout == nil || got.Master == nil {
		t.Fatalf("sources = %+v, want layout and master", got)
	}
	if got.Master.NvSpPr.NvPr.Ph == nil || got.Master.NvSpPr.NvPr.Ph.Type != "title" {
		t.Fatalf("master = %+v, want title selected through layout chain", got.Master.NvSpPr.NvPr.Ph)
	}
}
