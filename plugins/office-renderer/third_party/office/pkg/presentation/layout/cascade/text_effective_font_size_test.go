package cascade

import (
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func TestParagraphDefRPrOverridesInheritedFontSizeForRuns(t *testing.T) {
	ap := &parts.DMLParagraph{Properties: &parts.DMLParaProps{DefRPr: &parts.DMLRunPr{FontSize: 1800}}}
	_, defaults := ParagraphFromLocal(ap, TextDefaults{FontSize: 3200}, nil, nil)
	run := RunFromLocal("text", nil, defaults, nil, nil)
	if run.FontSize != 1800 {
		t.Fatalf("font size = %d, want paragraph-local 1800", run.FontSize)
	}
}

func TestEndParaRPrDoesNotOverrideRunDefaultFontSize(t *testing.T) {
	ap := &parts.DMLParagraph{EndParaRPr: &parts.DMLRunPr{FontSize: 4400}}
	_, defaults := ParagraphFromLocal(ap, TextDefaults{FontSize: 2400}, nil, nil)
	run := RunFromLocal("text", nil, defaults, nil, nil)
	if run.FontSize != 2400 {
		t.Fatalf("font size = %d, want inherited 2400; endParaRPr only formats paragraph mark", run.FontSize)
	}
}
