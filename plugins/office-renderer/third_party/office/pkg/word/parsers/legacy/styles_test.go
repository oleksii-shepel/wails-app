package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestLegacyMergeFramePropsTextDirection(t *testing.T) {
	base := &wordir.FrameProps{W: 100, TextDirection: "tbRl"}
	overlay := &wordir.FrameProps{H: 200, TextDirection: "btLr"}
	result := legacyMergeFrameProps(base, overlay)
	if result.TextDirection != "btLr" {
		t.Errorf("expected TextDirection btLr, got %q", result.TextDirection)
	}
	if result.W != 100 {
		t.Errorf("expected W 100, got %d", result.W)
	}
	if result.H != 200 {
		t.Errorf("expected H 200, got %d", result.H)
	}
}

func TestLegacyMergeFramePropsAnchorLock(t *testing.T) {
	base := &wordir.FrameProps{W: 100}
	overlay := &wordir.FrameProps{AnchorLock: true}
	result := legacyMergeFrameProps(base, overlay)
	if !result.AnchorLock {
		t.Errorf("expected AnchorLock true")
	}
}

func TestLegacyMergeFramePropsNilOverlay(t *testing.T) {
	base := &wordir.FrameProps{W: 100, TextDirection: "tbRl", AnchorLock: true}
	result := legacyMergeFrameProps(base, nil)
	if result.TextDirection != "tbRl" {
		t.Errorf("expected TextDirection tbRl, got %q", result.TextDirection)
	}
	if !result.AnchorLock {
		t.Errorf("expected AnchorLock true")
	}
}

func TestLegacyMergeFramePropsNilBase(t *testing.T) {
	overlay := &wordir.FrameProps{W: 100, TextDirection: "tbRl", AnchorLock: true}
	result := legacyMergeFrameProps(nil, overlay)
	if result.TextDirection != "tbRl" {
		t.Errorf("expected TextDirection tbRl, got %q", result.TextDirection)
	}
	if !result.AnchorLock {
		t.Errorf("expected AnchorLock true")
	}
}