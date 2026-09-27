package sax

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestPlaceholderInheritanceCarriesCompleteParagraphStyle(t *testing.T) {
	src := &ir.Paragraph{
		Align: "ctr", TextDirection: "rtl",
		MarginLeftEMU: 457200, HasMarginLeft: true,
		MarginRightEMU: 228600, HasMarginRight: true,
		TextIndentEMU: -114300, HasTextIndent: true,
		DefaultTabSizeEMU: 914400,
		TabStops:          []ir.TabStop{{Pos: 457200, Algn: "ctr"}},
		FontAlign:         "base",
		BulletChar:        "•", BulletColor: "112233", BulletFontFamily: "Oswald", BulletSizePct: 90000,
		SpaceBefore: 600, HasSpaceBefore: true,
		SpaceAfter: 800, HasSpaceAfter: true,
		LineSpacingPct: 115000, HasLineSpacing: true,
	}
	dst := &ir.Paragraph{}
	inheritParagraphFacts(dst, src)

	if dst.Align != "ctr" || dst.TextDirection != "rtl" || !dst.HasMarginLeft || !dst.HasMarginRight || !dst.HasTextIndent {
		t.Fatalf("basic paragraph inheritance = %#v", dst)
	}
	if dst.DefaultTabSizeEMU != 914400 || len(dst.TabStops) != 1 || dst.FontAlign != "base" {
		t.Fatalf("tab/font-align inheritance = %#v", dst)
	}
	if dst.BulletChar != "•" || dst.BulletColor != "112233" || dst.BulletFontFamily != "Oswald" || dst.BulletSizePct != 90000 {
		t.Fatalf("bullet inheritance = %#v", dst)
	}
	if !dst.HasSpaceBefore || dst.SpaceBefore != 600 || !dst.HasSpaceAfter || dst.SpaceAfter != 800 || !dst.HasLineSpacing || dst.LineSpacingPct != 115000 {
		t.Fatalf("spacing inheritance = %#v", dst)
	}
}

func TestPlaceholderParagraphStyleDoesNotOverrideAuthoredSpacing(t *testing.T) {
	src := &ir.Paragraph{
		SpaceBefore: 600, HasSpaceBefore: true,
		SpaceAfter: 800, HasSpaceAfter: true,
		LineSpacingPct: 115000, HasLineSpacing: true,
	}
	dst := &ir.Paragraph{
		SpaceBefore: 100, HasSpaceBefore: true,
		SpaceAfter: 1600, HasSpaceAfter: true,
		LineSpacingPt: 2200, HasLineSpacing: true,
	}
	inheritParagraphFacts(dst, src)
	if dst.SpaceBefore != 100 || dst.SpaceAfter != 1600 || dst.LineSpacingPt != 2200 || dst.LineSpacingPct != 0 {
		t.Fatalf("authored spacing was overwritten = %#v", dst)
	}
}

func TestSample3SubtitleStyleKeepsAuthoredAfterAndInheritsBodyLineSpacing(t *testing.T) {
	// sample-3.pptx: the slide subtitle authors spcAft=1600 while the master's
	// body placeholder supplies lnSpc=115000 and Playfair Display 18pt.
	master := &ir.Shape{Paragraphs: []*ir.Paragraph{{
		LineSpacingPct: 115000, HasLineSpacing: true,
		Runs: []*ir.Run{{FontFamily: "Playfair Display", FontSize: 1800, Color: "000000"}},
	}}}
	slide := &ir.Shape{Paragraphs: []*ir.Paragraph{{
		SpaceAfter: 1600, HasSpaceAfter: true,
		Runs: []*ir.Run{{Text: "Your Name • Sept 20XX"}},
	}}}
	inheritTextFacts(slide, master)
	p := slide.Paragraphs[0]
	r := p.Runs[0]
	if p.SpaceAfter != 1600 || !p.HasSpaceAfter || p.LineSpacingPct != 115000 || !p.HasLineSpacing {
		t.Fatalf("sample-3 subtitle paragraph = %#v", p)
	}
	if r.FontFamily != "Playfair Display" || r.FontSize != 1800 || r.Color != "000000" {
		t.Fatalf("sample-3 subtitle run = %#v", r)
	}
}

func Test37625PictureBulletBlocksInheritedBuNone(t *testing.T) {
	dst := &ir.Paragraph{
		BulletImageRelID: "rId1",
		BulletImageData:  []byte("PNG-BULLET"),
		BulletImageMIME:  "image/png",
		BulletNone:       false,
	}
	src := &ir.Paragraph{BulletNone: true}

	inheritParagraphFacts(dst, src)

	if dst.BulletNone {
		t.Fatal("inherited buNone suppressed authored picture bullet")
	}
	if dst.BulletImageRelID != "rId1" || string(dst.BulletImageData) != "PNG-BULLET" {
		t.Fatalf("picture bullet changed: rel=%q data=%q", dst.BulletImageRelID, string(dst.BulletImageData))
	}
}
