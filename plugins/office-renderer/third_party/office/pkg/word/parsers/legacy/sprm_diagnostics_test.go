package legacy

import (
	"testing"

	"office/pkg/diag"
)

func TestScanUnmappedSprmsEmitsDiagnostics(t *testing.T) {
	// grpprl with two unmapped CHP SPRMs:
	// 0x0806 sprmCFData (1 byte operand)
	// 0x484E sprmCHresi (2 byte operand)
	grpprl := []byte{
		0x06, 0x08, 0x01, // sprmCFData + 1-byte operand
		0x4E, 0x48, 0x00, 0x00, // sprmCHresi + 2-byte operand
	}
	list := scanUnmappedSprms(grpprl, "CHP", unmappedChpxSprms)
	if len(list) != 2 {
		t.Fatalf("len(diagnostics) = %d, want 2", len(list))
	}
	for _, d := range list {
		if d.Severity != diag.SeverityInfo {
			t.Fatalf("severity = %q, want info", d.Severity)
		}
		if d.Code != "legacy.sprm.unmapped" {
			t.Fatalf("code = %q, want legacy.sprm.unmapped", d.Code)
		}
	}
	if list[0].Message == "" {
		t.Fatal("first diagnostic message is empty")
	}
	if list[1].Message == "" {
		t.Fatal("second diagnostic message is empty")
	}
}

func TestScanUnmappedSprmsIgnoresMappedSprms(t *testing.T) {
	// grpprl with a mapped CHP SPRM (sprmCFBold = 0x0205) and no unmapped ones.
	grpprl := []byte{
		0x05, 0x02, 0x01, // sprmCFBold + 1-byte operand
	}
	list := scanUnmappedSprms(grpprl, "CHP", unmappedChpxSprms)
	if len(list) != 0 {
		t.Fatalf("len(diagnostics) = %d, want 0", len(list))
	}
}

func TestScanUnmappedSprmsEmptyGrpprl(t *testing.T) {
	list := scanUnmappedSprms(nil, "CHP", unmappedChpxSprms)
	if len(list) != 0 {
		t.Fatalf("len(diagnostics) = %d, want 0", len(list))
	}
}

func TestLegacyUnmappedSPRMPassesThroughDocument(t *testing.T) {
	// Build a minimal doc with a CHPX grpprl containing an unmapped SPRM.
	grpprl := []byte{0x06, 0x08, 0x01} // sprmCFData
	format, meta := parseChpxGrpprlWithMetadataAndFonts(grpprl, nil)
	if len(meta.Diagnostics) != 1 {
		t.Fatalf("meta.Diagnostics = %d, want 1", len(meta.Diagnostics))
	}
	entry := chpxEntry{FC: 0, Format: format, Diagnostics: meta.Diagnostics}
	_ = entry
}