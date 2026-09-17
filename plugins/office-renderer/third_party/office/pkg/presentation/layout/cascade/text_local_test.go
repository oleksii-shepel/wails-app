package cascade

import (
	"testing"

	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestRunFromLocalOverridesInheritedCharacterDefaults(t *testing.T) {
	inheritedBold := true
	localBold := false
	r := RunFromLocal("text", &parts.DMLRunPr{Bold: &localBold, FontSize: 2400}, TextDefaults{Bold: &inheritedBold, FontSize: 1800, FontFamily: "Inherited"}, nil, nil)
	if r.Bold {
		t.Fatal("local bold=false must override inherited bold=true")
	}
	if r.FontSize != 2400 {
		t.Fatalf("font size = %d, want 2400", r.FontSize)
	}
	if r.FontFamily != "Inherited" {
		t.Fatalf("font family = %q, want inherited family", r.FontFamily)
	}
}

func TestParagraphFromLocalKeepsInheritedDefaultsWithoutLocalProperties(t *testing.T) {
	margin := 12345
	p, effective := ParagraphFromLocal(&parts.DMLParagraph{}, TextDefaults{Align: "ctr", MarginLeftEMU: &margin, BulletColor: "336699"}, nil, nil)
	if p.Align != "ctr" {
		t.Fatalf("align = %q, want ctr", p.Align)
	}
	if !p.HasMarginLeft || p.MarginLeftEMU != margin {
		t.Fatalf("margin = (%v,%d), want inherited %d", p.HasMarginLeft, p.MarginLeftEMU, margin)
	}
	if p.BulletColor != "336699" {
		t.Fatalf("bullet color = %q", p.BulletColor)
	}
	if effective.Align != "ctr" {
		t.Fatalf("effective defaults align = %q", effective.Align)
	}
}

func TestParagraphLocalAutoNumberReplacesInheritedBulletGlyph(t *testing.T) {
	ap := &parts.DMLParagraph{Properties: &parts.DMLParaProps{
		BuAutoNum: &parts.BuAutoNum{Type: "arabicPeriod", StartAt: 5},
	}}
	p, _ := ParagraphFromLocal(ap, TextDefaults{BulletChar: "➢", BulletFontFamily: "Wingdings 3"}, nil, nil)
	if p.BulletChar != "" {
		t.Fatalf("bullet = %q, want cleared by local buAutoNum", p.BulletChar)
	}
	if !p.AutoNum || p.AutoNumType != "arabicPeriod" || p.AutoNumStart != 5 {
		t.Fatalf("autonum = %v %q %d, want arabicPeriod start 5", p.AutoNum, p.AutoNumType, p.AutoNumStart)
	}
}

func TestInheritedAutoNumberSurvivesParagraphWithoutLocalMarker(t *testing.T) {
	ap := &parts.DMLParagraph{}
	p, _ := ParagraphFromLocal(ap, TextDefaults{BulletSpecified: true, AutoNum: true, AutoNumType: "arabicPeriod", AutoNumStart: 1}, nil, nil)
	if !p.AutoNum || p.AutoNumType != "arabicPeriod" || p.BulletChar != "" || p.BulletNone {
		t.Fatalf("inherited autonum lost: %#v", p)
	}
}

func TestMoreSpecificBulletMarkerBlocksLessSpecificBuNone(t *testing.T) {
	dst := TextDefaults{BulletSpecified: true, BulletChar: "•"}
	src := TextDefaults{BulletSpecified: true, BulletNone: true}
	got := MergeTextDefaults(dst, src)
	if got.BulletNone || got.BulletChar != "•" {
		t.Fatalf("less-specific buNone replaced specific marker: %#v", got)
	}
}

func TestMoreSpecificAutoNumberBlocksLessSpecificBulletGlyph(t *testing.T) {
	dst := TextDefaults{BulletSpecified: true, AutoNum: true, AutoNumType: "arabicPeriod", AutoNumStart: 1}
	src := TextDefaults{BulletSpecified: true, BulletChar: "➢", BulletFontFamily: "Wingdings 3"}
	got := MergeTextDefaults(dst, src)
	if !got.AutoNum || got.AutoNumType != "arabicPeriod" || got.BulletChar != "" {
		t.Fatalf("less-specific bullet replaced autonum: %#v", got)
	}
}


func TestMergeTextDefaultsTreatsBulletSizeFormAsSingleCascadeProperty(t *testing.T) {
	got := MergeTextDefaults(
		TextDefaults{BulletSize: 1800},
		TextDefaults{BulletSizePct: 125000},
	)
	if got.BulletSize != 1800 || got.BulletSizePct != 0 {
		t.Fatalf("local absolute marker size cross-inherited percentage: %#v", got)
	}

	got = MergeTextDefaults(
		TextDefaults{BulletSizePct: 80000},
		TextDefaults{BulletSize: 2400},
	)
	if got.BulletSizePct != 80000 || got.BulletSize != 0 {
		t.Fatalf("local percentage marker size cross-inherited absolute: %#v", got)
	}
}
