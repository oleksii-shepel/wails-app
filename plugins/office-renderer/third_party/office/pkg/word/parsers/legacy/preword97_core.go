package legacy

import (
	"encoding/binary"
	"sort"

	"golang.org/x/text/encoding/charmap"
)

type preWord97DocCore struct {
	Span              PreWord97BodySpan
	Fonts             []PreWord97FontEntry
	PreferredCharset  *charmap.Charmap
	ChpxEntries       []chpxEntry
	PapxEntries       []papxEntry
	ListTables        *legacyListTables
	ComplexFileTable  *legacyComplexFileTable
	SyntheticPiecePlc []byte
}

func loadPreWord97DocCore(wordStream []byte) (*preWord97DocCore, bool) {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		return nil, false
	}
	papxEntries := convertPreWord97PapxEntries(loadPreWord97PapxEntries(wordStream, span))
	core := &preWord97DocCore{
		Span:             span,
		Fonts:            LoadPreWord97FontTable(wordStream, span.FontTableOffset, span.FontTableSize),
		PreferredCharset: guessPreWord97Charset(wordStream, span),
		ChpxEntries:      loadPreWord97ChpxEntries(wordStream, span),
		PapxEntries:      papxEntries,
		ListTables:       preWord97SynthesizeListTables(papxEntries),
	}
	if span.FComplex && span.ComplexTableOffset > 0 && span.ComplexTableOffset < len(wordStream) {
		core.ComplexFileTable = loadLegacyComplexFileTable(wordStream[span.ComplexTableOffset:])
	}
	if !span.FComplex {
		core.SyntheticPiecePlc = buildPreWord97SinglePiecePlcPcd(span)
	}
	return core, true
}

func (core *preWord97DocCore) assembleMainStoryRuns(wordStream []byte) []docRun {
	var plc []byte
	if core.Span.FComplex {
		if core.ComplexFileTable == nil {
			return nil
		}
		plc = core.ComplexFileTable.PlcPcd
	} else {
		plc = core.SyntheticPiecePlc
	}
	if len(plc) == 0 {
		return nil
	}
	runs, ok := assembleDocRunsFromPlcPcdWithCharset(wordStream, plc, core.ChpxEntries, core.PapxEntries, core.PreferredCharset)
	if !ok {
		return nil
	}
	return runs
}

func buildPreWord97SinglePiecePlcPcd(span PreWord97BodySpan) []byte {
	charCount := span.BodyEnd - span.Start
	if span.Start <= 0 || charCount <= 0 {
		return nil
	}
	plc := make([]byte, 16)
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], uint32(charCount))
	fcRaw := uint32(0x40000000) | uint32(span.Start*2)
	binary.LittleEndian.PutUint32(plc[10:14], fcRaw)
	return plc
}

func convertPreWord97PapxEntries(entries []preWord97PapxEntry) []papxEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]papxEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.EndFC <= entry.StartFC {
			continue
		}
		out = append(out, papxEntry{
			FC:                uint32(entry.StartFC),
			Format:            entry.Format,
			Meta:              entry.Meta,
			LegacyInTableHint: entry.LegacyInTableHint,
			LegacyRowMarkHint: entry.LegacyRowMarkHint,
		})
	}
	return out
}

// preWord97SynthesizeListTables builds a lightweight binary-list table map from
// old PAPx entries by assigning synthetic ilfo IDs and carrying observed ANLD
// format/start metadata through existing list continuation plumbing.
func preWord97SynthesizeListTables(entries []papxEntry) *legacyListTables {
	if len(entries) == 0 {
		return nil
	}
	nextIlfo := 1
	ilfoByStyle := map[int]int{}
	formatByIlfoLevel := map[string]string{}
	startByIlfoLevel := map[string]int{}
	seen := false
	for i := range entries {
		pf := &entries[i].Format
		if pf.BinaryListLevel == nil || *pf.BinaryListLevel < 0 || *pf.BinaryListLevel > 8 {
			continue
		}
		styleKey := -1
		if pf.BinaryStyleIndex != nil {
			styleKey = *pf.BinaryStyleIndex
		}
		ilfo, ok := ilfoByStyle[styleKey]
		if !ok {
			ilfo = nextIlfo
			nextIlfo++
			ilfoByStyle[styleKey] = ilfo
		}
		ilfoVal := ilfo
		pf.BinaryListOverride = &ilfoVal
		level := *pf.BinaryListLevel
		nfc := 0
		if pf.BinaryListFormat != nil {
			nfc = *pf.BinaryListFormat
		}
		key := legacyListKey(ilfo, level)
		if _, ok := formatByIlfoLevel[key]; !ok {
			formatByIlfoLevel[key] = preWord97NfcToListFormat(nfc)
		}
		if entries[i].Meta.PreWord97ListStart != nil {
			if start := *entries[i].Meta.PreWord97ListStart; start > 0 {
				if _, ok := startByIlfoLevel[key]; !ok {
					startByIlfoLevel[key] = start
				}
			}
		}
		seen = true
	}
	if !seen || len(formatByIlfoLevel) == 0 {
		return nil
	}
	return &legacyListTables{
		formatByIlfoLevel: formatByIlfoLevel,
		startByIlfoLevel:  startByIlfoLevel,
	}
}

func loadPreWord97ChpxEntries(wordStream []byte, span PreWord97BodySpan) []chpxEntry {
	if span.ChpTableOffset <= 0 || span.ChpTableSize < 10 || span.ChpTableOffset+span.ChpTableSize > len(wordStream) {
		return nil
	}
	plc := wordStream[span.ChpTableOffset : span.ChpTableOffset+span.ChpTableSize]
	n := (len(plc) - 4) / 6
	if n <= 0 {
		return nil
	}
	cpBytes := (n + 1) * 4
	if cpBytes+n*2 > len(plc) {
		return nil
	}
	fontEntries := LoadPreWord97FontTable(wordStream, span.FontTableOffset, span.FontTableSize)
	fontNames := make([]string, len(fontEntries))
	for i, fe := range fontEntries {
		fontNames[i] = fe.Name
	}

	var entries []chpxEntry
	for index := 0; index < n; index++ {
		pageNum := int(binary.LittleEndian.Uint16(plc[cpBytes+index*2 : cpBytes+index*2+2]))
		pageOff := pageNum * 512
		if pageOff < 0 || pageOff+512 > len(wordStream) {
			continue
		}
		entries = append(entries, parsePreWord97ChpxFKPPage(wordStream[pageOff:pageOff+512], fontNames)...)
	}
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].FC < entries[j].FC
	})
	write := 0
	for _, entry := range entries {
		if write > 0 && entries[write-1].FC == entry.FC {
			continue
		}
		entries[write] = entry
		write++
	}
	return entries[:write]
}