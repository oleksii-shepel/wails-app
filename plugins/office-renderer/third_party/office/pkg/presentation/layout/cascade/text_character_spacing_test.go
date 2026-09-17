package cascade

import (
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func TestRunFromLocalPreservesCharacterSpacingAndKerning(t *testing.T) {
	spc, kern := 125, 1200
	r := RunFromLocal("Text", &parts.DMLRunPr{FontSize: 2400, Spacing: &spc, Kern: &kern}, TextDefaults{}, nil, nil)
	if r.CharSpacing != 125 || r.Kern != 1200 {
		t.Fatalf("spacing/kern = %d/%d", r.CharSpacing, r.Kern)
	}
}

func TestParagraphFromLocalInheritsMasterBulletAndEndParagraphDefaults(t *testing.T) {
	fontSize := 2400
	defaults := TextDefaults{BulletChar: "\uf07d", BulletFontFamily: "Wingdings 3", FontSize: 1800}
	p, effective := ParagraphFromLocal(&parts.DMLParagraph{EndParaRPr: &parts.DMLRunPr{FontSize: fontSize}}, defaults, nil, nil)
	if p == nil || p.BulletChar != "\uf07d" || p.BulletFontFamily != "Wingdings 3" {
		t.Fatalf("bullet = %q/%q", p.BulletChar, p.BulletFontFamily)
	}
	if effective.FontSize != 1800 {
		t.Fatalf("effective font size = %d, want inherited 1800; endParaRPr only formats paragraph mark", effective.FontSize)
	}
}
