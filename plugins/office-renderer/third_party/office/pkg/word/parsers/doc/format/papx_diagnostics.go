package docformat

import (
	"fmt"

	"office/pkg/diag"
)

var unmappedPAPXSprms = map[uint16]string{
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

func scanUnmappedPAPX(grpprl []byte) diag.List {
	var list diag.List
	for pos := 0; pos+2 <= len(grpprl); {
		code := uint16(grpprl[pos]) | uint16(grpprl[pos+1])<<8
		pos += 2
		size := OperandSize(code, grpprl, pos)
		if code == 0x8426 || code == 0x6412 || code == 0x6649 || code == 0x664A { size = 4 }
		if size < 0 || pos+size > len(grpprl) { break }
		if name, ok := unmappedPAPXSprms[code]; ok {
			list.Add(diag.Diagnostic{Severity: diag.SeverityInfo, Code: "legacy.sprm.unmapped", Message: fmt.Sprintf("SPRM %s (0x%04X) is recognized but has no semantic mapping", name, code)})
		}
		pos += size
	}
	return list
}
