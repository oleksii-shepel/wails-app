package legacy

import (
	"fmt"

	"office/pkg/diag"
)

// legacyUnmappedSPRM records a diagnostic for a recognized-but-not-semantically-mapped SPRM.
func legacyUnmappedSPRM(list *diag.List, code uint16, name string) {
	if list == nil {
		return
	}
	list.Add(diag.Diagnostic{
		Severity: diag.SeverityInfo,
		Code:     "legacy.sprm.unmapped",
		Message:  fmt.Sprintf("SPRM %s (0x%04X) is recognized but has no semantic mapping", name, code),
	})
}

// unmappedChpxSprms lists CHP SPRMs that are handled in parseChpxGrpprlWithMetadataAndFonts
// for decode coverage but do not produce semantic IR output.
var unmappedChpxSprms = map[uint16]string{
	0x0806: "sprmCFData",
	0x0808: "sprmCFBoldBi",
	0x080A: "sprmCFItalicBi",
	0x080B: "sprmCFUseFELayout",
	0x080D: "sprmCFNoProof",
	0x080F: "sprmCFitText",
	0x0810: "sprmCFelidsrMark",
	0x0812: "sprmCFBoldOther",
	0x0813: "sprmCFItalicOther",
	0x0814: "sprmCFUseHL2",
	0x0815: "sprmCFBiDi",
	0x0817: "sprmCFBoldOther",
	0x0819: "sprmCFItalicOther",
	0x081A: "sprmCFUseHL2",
	0x081B: "sprmCFBiDi",
	0x081C: "sprmCFDntUL",
	0x081D: "sprmCFDntBdr",
	0x081E: "sprmCFUser",
	0x081F: "sprmCFUser",
	0x0820: "sprmCFUser",
	0x0821: "sprmCFUser",
	0x0823: "sprmCFUser",
	0x0824: "sprmCFUser",
	0x0825: "sprmCFUser",
	0x0826: "sprmCFUser",
	0x0827: "sprmCFUser",
	0x0828: "sprmCFUser",
	0x0829: "sprmCFUser",
	0x082A: "sprmCFUser",
	0x082B: "sprmCFUser",
	0x082C: "sprmCFUser",
	0x082D: "sprmCFUser",
	0x082E: "sprmCFUser",
	0x082F: "sprmCFUser",
	0x085B: "sprmCFUser",
	0x086A: "sprmCFUser",
	0x484E: "sprmCHresi",
	0xCA62: "sprmCDispFldRMark",
	0x0867: "sprmCObsolete",
	0x0868: "sprmCFUsePgsuSettings",
	0x0869: "sprmCObsolete",
	0x086B: "sprmCObsolete",
	0x086C: "sprmCObsolete",
	0x286F: "sprmCIdctHint",
	0x0856: "sprmCFObj",
	0x2859: "sprmCSfxText",
	0x6A09: "sprmCSymbol",
	0x0818: "sprmCFSpecVanish",
	0x2A83: "sprmCWall",
	0x2A86: "sprmCNeedFontFixup",
	0x4807: "sprmCIdslRMark",
	0x4867: "sprmCIdslRMarkDel",
	0x6815: "sprmCRsidProp",
	0x6816: "sprmCRsidText",
	0x6817: "sprmCRsidRMDel",
	0xCA31: "sprmCIstdPermute",
	0xCA85: "sprmCCnf",
	0x2879: "sprmCLbcCRJ",
	0x2A90: "sprmCFSdtVanish",
	0x6887: "sprmCPbiIBullet",
	0x4888: "sprmCPbiGrf",
	0xC81A: "sprmCFMathPr",
}

// unmappedPapxSprms lists PAP/TAP SPRMs that are handled in parsePapxGrpprlWithMetadata
// for decode coverage but do not produce semantic IR output.
var unmappedPapxSprms = map[uint16]string{
	0x2404: "sprmPFSideBySide",
	0x6428: "sprmPBrcBetween80",
	0x6628: "sprmPBrcBetween80Old",
	0x6629: "sprmPBrcBar80",
	0x4455: "sprmPDxcRight",
	0x4456: "sprmPDxcLeft",
	0x4457: "sprmPDxcLeft1",
	0x6465: "sprmPIpgp",
	0x646B: "sprmPTableProps",
	0x6646: "sprmPHugePapx",
	0x4600: "sprmPIstd",
	0xC601: "sprmPIstdPermute",
	0x2602: "sprmPIncLvl",
	0x245A: "sprmPFOpenTch",
	0x2471: "sprmPTtwo",
	0x2664: "sprmPWall",
	0xC666: "sprmPCnf",
	0xC669: "sprmPIstdListPermute",
	0xC66C: "sprmPTIstdInfo",
	0x2444: "sprmPFOld",
	0x2447: "sprmPFUsePgsuSettings",
	0x2448: "sprmPFAdjustRight",
	0x2443: "sprmPFNumRMIns",
	0xC645: "sprmPNumRM",
	0x443A: "sprmPFrameTextFlow",
	0xD667: "sprmTPropRMark",
}

// unmappedSepxSprms lists SEP SPRMs that are handled in parseSepxGrpprlWithMetadata
// for decode coverage but do not produce semantic IR output.
var unmappedSepxSprms = map[uint16]string{
	0x322A: "sprmSFRTLGutter",
	0xF203: "sprmSDxaColWidth",
	0xF204: "sprmSDxaColSpacing",
	0x3005: "sprmSFEvenlySpaced",
	0x3006: "sprmSFProtected",
	0x5007: "sprmSDmBinFirst",
	0x5008: "sprmSDmBinOther",
	0x3000: "sprmScnsPgn",
	0x3001: "sprmSiHeadingPgn",
	0x5026: "sprmSDmPaperReq",
	0x3239: "sprmSWall",
	0x703A: "sprmSRsid",
}

// scanUnmappedSprms walks a grpprl and emits info-level diagnostics for every SPRM
// code that appears in the supplied unmapped set.  The grpprl is re-scanned so this
// can be called after the main switch has already consumed the bytes.
func scanUnmappedSprms(grpprl []byte, group string, unmapped map[uint16]string) diag.List {
	var list diag.List
	pos := 0
	for pos+2 <= len(grpprl) {
		code := uint16(grpprl[pos]) | uint16(grpprl[pos+1])<<8
		pos += 2
		size := chpxOperandSize(code, grpprl, pos)
		if size < 0 || pos+size > len(grpprl) {
			break
		}
		if name, ok := unmapped[code]; ok {
			legacyUnmappedSPRM(&list, code, name)
		}
		pos += size
	}
	return list
}