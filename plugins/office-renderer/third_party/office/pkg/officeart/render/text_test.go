package render

import (
	"strings"
	"testing"

	"office/pkg/officeart/ir"
)

func TestTextStyleNoWrapVerticalAndAnchor(t *testing.T) {
	body := &ir.TextBody{
		Wrap:         ir.TextWrapNone,
		Vert:         ir.TextVerticalRL,
		Anchor:       ir.TextAnchorCenter,
		AnchorCenter: true,
		Insets:       ir.Insets{Left: 12700},
	}
	style := StyleString(TextStyle(body))
	for _, want := range []string{
		"white-space:nowrap;",
		"overflow:visible;",
		"writing-mode:vertical-rl;",
		"justify-content:center;",
		"text-align:center;",
		"padding-left:1.0000pt;",
	} {
		if !strings.Contains(style, want) {
			t.Fatalf("style %q missing %q", style, want)
		}
	}
}

func TestTextStyleWordArtVerticalUsesVerticalWritingMode(t *testing.T) {
	body := &ir.TextBody{Vert: ir.TextWordArtVertical}
	style := StyleString(TextStyle(body))
	for _, want := range []string{"writing-mode:vertical-rl;", "text-orientation:mixed;"} {
		if !strings.Contains(style, want) {
			t.Fatalf("style %q missing %q", style, want)
		}
	}
}
