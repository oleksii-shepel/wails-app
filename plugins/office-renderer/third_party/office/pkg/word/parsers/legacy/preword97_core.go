package legacy

import (
	pre97 "office/pkg/word/parsers/doc/pre97"

	"golang.org/x/text/encoding/charmap"
)

type preWord97DocCore struct {
	Canonical         *pre97.Core
	Span              PreWord97BodySpan
	Fonts             []PreWord97FontEntry
	PreferredCharset  *charmap.Charmap
	ChpxEntries       []chpxEntry
	PapxEntries       []papxEntry
	ListTables        *legacyListTables
	ComplexFileTable  *legacyComplexFileTable
	SyntheticPiecePlc []byte
	PiecePlc          []byte
}

func loadPreWord97DocCore(wordStream []byte) (*preWord97DocCore, bool) {
	canonical, ok := pre97.OpenCore(wordStream)
	if !ok {
		return nil, false
	}
	span := legacyPreWord97Span(canonical.Span)
	fonts := convertCanonicalPreWord97Fonts(canonical.Fonts)
	papxEntries := convertCanonicalPreWord97PAPX(canonical.PAPX)
	core := &preWord97DocCore{
		Canonical:         canonical,
		Span:              span,
		Fonts:             fonts,
		PreferredCharset:  canonical.PreferredCharset,
		ChpxEntries:       convertCanonicalPreWord97CHPX(canonical.CHPX, canonical.FontNames),
		PapxEntries:       papxEntries,
		ListTables:        preWord97SynthesizeListTables(papxEntries),
		ComplexFileTable:  canonical.Complex,
		PiecePlc:          append([]byte(nil), canonical.PiecePLC...),
	}
	if !span.FComplex {
		core.SyntheticPiecePlc = append([]byte(nil), canonical.PiecePLC...)
	}
	return core, true
}

func convertCanonicalPreWord97Fonts(fonts []pre97.FontEntry) []PreWord97FontEntry {
	if len(fonts) == 0 {
		return nil
	}
	out := make([]PreWord97FontEntry, len(fonts))
	for i, font := range fonts {
		out[i] = PreWord97FontEntry{Charset: font.Charset, Name: font.Name, AltName: font.AltName}
	}
	return out
}

func legacyPreWord97Span(span pre97.Span) PreWord97BodySpan {
	return PreWord97BodySpan{
		Signature: span.Signature, NFib: span.NFib, Start: span.Start, FcMac: span.FcMac,
		CcpText: span.CcpText, BodyEnd: span.BodyEnd, SecondaryStart: span.SecondaryStart,
		SecondarySize: span.SecondarySize, FComplex: span.FComplex,
		CcpFtn: span.CcpFtn, CcpHdd: span.CcpHdd, CcpMcr: span.CcpMcr, CcpAtn: span.CcpAtn,
		CcpEdn: span.CcpEdn, CcpTxbx: span.CcpTxbx, CcpHdrTxbx: span.CcpHdrTxbx,
		SedTableOffset: span.SedTableOffset, SedTableSize: span.SedTableSize,
		FtnRefTableOffset: span.FtnRefTableOffset, FtnRefTableSize: span.FtnRefTableSize,
		FtnTxtTableOffset: span.FtnTxtTableOffset, FtnTxtTableSize: span.FtnTxtTableSize,
		AtnRefTableOffset: span.AtnRefTableOffset, AtnRefTableSize: span.AtnRefTableSize,
		AtnTxtTableOffset: span.AtnTxtTableOffset, AtnTxtTableSize: span.AtnTxtTableSize,
		HddTableOffset: span.HddTableOffset, HddTableSize: span.HddTableSize,
		ChpTableOffset: span.ChpTableOffset, ChpTableSize: span.ChpTableSize,
		PapTableOffset: span.PapTableOffset, PapTableSize: span.PapTableSize,
		FontTableOffset: span.FontTableOffset, FontTableSize: span.FontTableSize,
		BkmkNameTableOffset: span.BkmkNameTableOffset, BkmkNameTableSize: span.BkmkNameTableSize,
		BkmkStartTableOffset: span.BkmkStartTableOffset, BkmkStartTableSize: span.BkmkStartTableSize,
		BkmkEndTableOffset: span.BkmkEndTableOffset, BkmkEndTableSize: span.BkmkEndTableSize,
		EdnRefTableOffset: span.EdnRefTableOffset, EdnRefTableSize: span.EdnRefTableSize,
		EdnTxtTableOffset: span.EdnTxtTableOffset, EdnTxtTableSize: span.EdnTxtTableSize,
		ComplexTableOffset: span.ComplexTableOffset,
	}
}

func (core *preWord97DocCore) assembleMainStoryRuns(wordStream []byte) []docRun {
	if core == nil || core.Canonical == nil {
		return nil
	}
	cursor, err := pre97.NewPieceCursor(wordStream, core.Canonical)
	if err != nil {
		return nil
	}
	var runs []docRun
	for {
		run, ok, err := cursor.Next()
		if err != nil || !ok {
			if err != nil {
				return nil
			}
			break
		}
		runs = append(runs, docRun{
			Text:                run.Text,
			Kind:                run.Kind,
			Format:              run.Format,
			ParaFormat:          run.ParagraphFormat,
			TableDepth:          run.TableDepth,
			LegacyInTableHint:   run.LegacyInTableHint,
			LegacyRowMarkHint:   run.LegacyRowMarkHint,
			TableProps:          legacyTableRowPropsFromCanonical(run.TableProps),
			StartCP:             run.StartCP,
			FieldKind:           run.FieldKind,
			FieldInstr:          run.FieldInstr,
			FieldArg:            run.FieldArg,
			Target:              run.Target,
			Anchor:              run.Anchor,
			NoteID:              run.NoteID,
			CommentID:           run.CommentID,
			Image:               run.Image,
			Shape:               run.Shape,
			Spec:                run.Spec,
			Data:                run.Data,
			Obj:                 run.Obj,
			ChpxSprms:           append([]byte(nil), run.ChpxSprms...),
			PapxSprms:           append([]byte(nil), run.PapxSprms...),
		})
	}
	return runs
}

func buildPreWord97SinglePiecePlcPcd(span PreWord97BodySpan) []byte {
	return pre97.BuildSinglePiecePLC(pre97.Span{Start: span.Start, BodyEnd: span.BodyEnd})
}

func convertCanonicalPreWord97PAPX(entries []pre97.PAPXEntry) []papxEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]papxEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.EndFC <= entry.StartFC {
			continue
		}
		format, semantic := pre97.DecodePAPXGrpprl(entry.Sprms)
		if entry.StyleIndex != nil {
			styleIndex := *entry.StyleIndex
			format.BinaryStyleIndex = &styleIndex
		}
		meta := papxMetadata{PreWord97ListStart: semantic.ListStart}
		out = append(out, papxEntry{
			FC:                uint32(entry.StartFC),
			Format:            format,
			Meta:              meta,
			LegacyInTableHint: semantic.InTable,
			LegacyRowMarkHint: semantic.RowMark,
			Sprms:             append([]byte(nil), entry.Sprms...),
		})
	}
	return out
}

func convertCanonicalPreWord97CHPX(entries []pre97.CHPXEntry, fontNames []string) []chpxEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]chpxEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, chpxEntry{
			FC:     entry.FC,
			Format: pre97.DecodeCHPXGrpprl(entry.Sprms, fontNames),
			Sprms:  append([]byte(nil), entry.Sprms...),
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

// These wrappers remain for legacy tests/callers during migration. Production
// pre-97 core construction above consumes canonical Core state directly.
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
			Sprms:             append([]byte(nil), entry.Sprms...),
		})
	}
	return out
}

func loadPreWord97ChpxEntries(wordStream []byte, span PreWord97BodySpan) []chpxEntry {
	canonical := pre97.Span{
		Start: span.Start, BodyEnd: span.BodyEnd,
		ChpTableOffset: span.ChpTableOffset, ChpTableSize: span.ChpTableSize,
		FontTableOffset: span.FontTableOffset, FontTableSize: span.FontTableSize,
	}
	fonts := pre97.LoadFontTable(wordStream, span.FontTableOffset, span.FontTableSize)
	return convertCanonicalPreWord97CHPX(pre97.LoadCHPXEntries(wordStream, canonical), pre97.FontNames(fonts))
}
