package sax

import (
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestFindPlaceholderDoesNotCrossSemanticRoleOnIndexCollision(t *testing.T) {
	date := &ir.Shape{Placeholder: ir.PlaceholderDateTime, PlaceholderIdx: 2, TextAnchor: "b"}
	body := &ir.Shape{Placeholder: ir.PlaceholderBody, PlaceholderIdx: 1, TextAnchor: ""}
	target := &ir.Shape{Placeholder: ir.PlaceholderContent, PlaceholderIdx: 2}

	got := findPlaceholder([]*ir.Shape{date, body}, target)
	if got != body {
		t.Fatalf("placeholder match = %#v, want compatible body placeholder rather than same-index date placeholder", got)
	}
}

func TestContentPlaceholderDoesNotInheritBottomAnchorFromSameIndexDatePlaceholder(t *testing.T) {
	target := &ir.Shape{Placeholder: ir.PlaceholderContent, PlaceholderIdx: 2}
	layout := []*ir.Shape{{Placeholder: ir.PlaceholderContent, PlaceholderIdx: 2}}
	master := []*ir.Shape{
		{Placeholder: ir.PlaceholderDateTime, PlaceholderIdx: 2, TextAnchor: "b"},
		{Placeholder: ir.PlaceholderBody, PlaceholderIdx: 1, TextAnchor: ""},
	}

	inheritShapeTree(target, layout, master)
	if target.TextAnchor != "" {
		t.Fatalf("content placeholder anchor = %q, want top/default; same-index date placeholder must not leak anchor", target.TextAnchor)
	}
}

func TestSubtitlePlaceholderMayInheritSameIndexBodyStyleFacts(t *testing.T) {
	target := &ir.Shape{Placeholder: ir.PlaceholderSubtitle, PlaceholderIdx: 1}
	got := findPlaceholder([]*ir.Shape{
		{Placeholder: ir.PlaceholderDateTime, PlaceholderIdx: 1, TextAnchor: "b"},
		{Placeholder: ir.PlaceholderBody, PlaceholderIdx: 1, TextAnchor: "ctr"},
	}, target)
	if got == nil || got.Placeholder != ir.PlaceholderBody {
		t.Fatalf("subtitle placeholder match = %#v, want same-index body placeholder", got)
	}
}
