package cascade

import (
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func TestParagraphFromLocalPreservesAbsoluteLineSpacing(t *testing.T) {
	p, _ := ParagraphFromLocal(&parts.DMLParagraph{Properties: &parts.DMLParaProps{LnSpc: &parts.DMLSpacing{SpcPts: &parts.SpacingPts{Val: 1800}}}}, TextDefaults{}, nil, nil)
	if p == nil || !p.HasLineSpacing || p.LineSpacingPt != 1800 || p.LineSpacingPct != 0 {
		t.Fatalf("absolute line spacing lost: %#v", p)
	}
}
