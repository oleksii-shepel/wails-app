package legacy

import (
	"encoding/binary"

	wordir "office/pkg/word/ir"
)

type legacyDocSettings struct {
	DefaultTabStop        int
	FootnoteNumberFormat  string
	FootnoteNumberStart   int
	FootnoteRestart       string
	EndnotePosition       string
	EndnoteNumberFormat   string
	EndnoteNumberStart    int
	EndnoteRestart        string
	ZoomPercent           int
	ViewMode              string
	ProtectionEnabled     bool
	DocumentProtection    *wordir.DocumentProtection
	CreatedAt             string
	ModifiedAt            string
	LastPrintedAt         string
	WordCount             int
	CharCount             int
	PageCount             int
	ParagraphCount        int
	MirrorMargins         bool
	TrackChangesEnabled   bool
}

// loadLegacyDocSettings reads the Word 97+ DOP record from FibRgFcLcb97[31].
// MS-DOC stores document defaults such as dxaTab, endnote restart/start
// settings, endnote placement, and numbering format in this block.
func loadLegacyDocSettings(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) legacyDocSettings {
	dop, ok := loadLegacyDopRecord(wordStream, tableStream, fibBase, cbRgFcLcb)
	if !ok {
		return legacyDocSettings{}
	}
	settings := legacyDocSettings{}
	if len(dop) >= 0x0C {
		if dxaTab := int(int16(binary.LittleEndian.Uint16(dop[0x0A:0x0C]))); dxaTab > 0 {
			settings.DefaultTabStop = dxaTab
		}
	}
	if len(dop) >= 0x04 {
		footnoteInfo := binary.LittleEndian.Uint16(dop[0x02:0x04])
		if restart := legacyDopNoteRestart(byte(footnoteInfo & 0x0003)); restart != "" {
			settings.FootnoteRestart = restart
		}
		if start := int((footnoteInfo & 0xFFFC) >> 2); start > 0 {
			settings.FootnoteNumberStart = start
		}
	}
	if len(dop) >= 0x38 {
		edn := binary.LittleEndian.Uint16(dop[0x34:0x36])
		edn1 := binary.LittleEndian.Uint16(dop[0x36:0x38])
		if restart := legacyDopNoteRestart(byte(edn & 0x0003)); restart != "" {
			settings.EndnoteRestart = restart
		}
		if pos := legacyDopEndnotePosition(byte(edn1 & 0x0003)); pos != "" {
			settings.EndnotePosition = pos
		}
		if start := int((edn & 0xFFFC) >> 2); start > 0 {
			settings.EndnoteNumberStart = start
		}
		if format := legacyDopEndnoteNumberFormat(dop, edn1); format != "" {
			settings.EndnoteNumberFormat = format
		}
	}
	if format := legacyDopFootnoteNumberFormat(dop); format != "" {
		settings.FootnoteNumberFormat = format
	}
	if len(dop) >= 0x40 {
		view := binary.LittleEndian.Uint16(dop[0x3E:0x40])
		settings.ZoomPercent = int((view & 0x0FF8) >> 3)
		settings.ViewMode = legacyDopViewMode(byte(view & 0x0007))
	}
	if len(dop) >= 0x0A {
		docinfo2 := dop[0x09]
		settings.ProtectionEnabled = docinfo2&0x02 != 0
		settings.MirrorMargins = docinfo2&0x20 != 0
		if settings.ProtectionEnabled {
			settings.DocumentProtection = &wordir.DocumentProtection{
				Enforcement: true,
			}
		}
	}
	if len(dop) >= 0x12 {
		settings.CreatedAt = formatLegacyDttm(binary.LittleEndian.Uint32(dop[0x0E:0x12]))
	}
	if len(dop) >= 0x16 {
		settings.ModifiedAt = formatLegacyDttm(binary.LittleEndian.Uint32(dop[0x12:0x16]))
	}
	if len(dop) >= 0x1A {
		settings.LastPrintedAt = formatLegacyDttm(binary.LittleEndian.Uint32(dop[0x16:0x1A]))
	}
	if len(dop) >= 0x1E {
		settings.WordCount = int(binary.LittleEndian.Uint32(dop[0x1A:0x1E]))
	}
	if len(dop) >= 0x22 {
		settings.CharCount = int(binary.LittleEndian.Uint32(dop[0x1E:0x22]))
	}
	if len(dop) >= 0x26 {
		settings.PageCount = int(binary.LittleEndian.Uint32(dop[0x22:0x26]))
	}
	if len(dop) >= 0x2A {
		settings.ParagraphCount = int(binary.LittleEndian.Uint32(dop[0x26:0x2A]))
	}
	if len(dop) >= 0x06 {
		docinfo1 := dop[0x06]
		settings.TrackChangesEnabled = docinfo1&0x80 != 0
	}
	return settings
}

func loadLegacyDopRecord(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) ([]byte, bool) {
	const dopFibIndex = 31
	if dopFibIndex >= cbRgFcLcb || fibBase+(dopFibIndex+1)*8 > len(wordStream) {
		return nil, false
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+dopFibIndex*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+dopFibIndex*8+4:]))
	if lcb < 12 || fc < 0 || fc+lcb > len(tableStream) {
		return nil, false
	}
	return tableStream[fc : fc+lcb], true
}

func legacyDopEndnotePosition(epc byte) string {
	switch epc {
	case 0:
		return "sectEnd"
	case 3:
		return "docEnd"
	default:
		return ""
	}
}

func legacyDopNoteRestart(restart byte) string {
	switch restart {
	case 0:
		return "continuous"
	case 1:
		return "eachSect"
	case 2:
		return "eachPage"
	default:
		return ""
	}
}

func legacyDopFootnoteNumberFormat(dop []byte) string {
	if len(dop) >= 0x1EE {
		if format := legacyNfcToListFormat(byte(binary.LittleEndian.Uint16(dop[0x1EC:0x1EE]))); format != "" {
			return format
		}
	}
	if len(dop) >= 0x38 {
		edn1 := binary.LittleEndian.Uint16(dop[0x36:0x38])
		return legacyNfcToListFormat(byte((edn1 & 0x003C) >> 2))
	}
	return ""
}

func legacyDopEndnoteNumberFormat(dop []byte, edn1 uint16) string {
	if len(dop) >= 0x1F0 {
		if format := legacyNfcToListFormat(byte(binary.LittleEndian.Uint16(dop[0x1EE:0x1F0]))); format != "" {
			return format
		}
	}
	return legacyNfcToListFormat(byte((edn1 & 0x03C0) >> 6))
}

// loadLegacyDocCompatibility parses document compatibility flags from the DOP
// record. MS-DOC defines three compatibility fields:
//   - Copts60 at offset 0x08 (Word 6.0 flags, 2 bytes)
//   - Copts80 at offset 0x54 (Word 8.0 flags, 4 bytes)
//   - Copts   at offset 0x1FC (Word 2000+ flags, 32 bytes)
// Returns nil when the DOP is too short for Copts80.
func loadLegacyDocCompatibility(dop []byte) *wordir.DocCompatibility {
	if len(dop) < 0x58 {
		return nil
	}
	c := &wordir.DocCompatibility{}

	// Copts60 at offset 0x08 (DopBase, 2 bytes).
	if len(dop) >= 0x0A {
		copts60 := binary.LittleEndian.Uint16(dop[0x08:0x0A])
		c.NoTabHangInd = copts60&0x0001 != 0
		c.NoSpaceRaiseLower = copts60&0x0002 != 0
		c.SuppressSpBfAfterPgBrk = copts60&0x0004 != 0
		c.WrapTrailSpaces = copts60&0x0008 != 0
		c.PrintColBlack = copts60&0x0010 != 0
		c.NoColumnBalance = copts60&0x0020 != 0
		c.ConvMailMergeEsc = copts60&0x0040 != 0
		c.SuppressTopSpacing = copts60&0x0080 != 0
		c.UseSingleBorderforContiguousCells = copts60&0x0100 != 0
		c.ShowBreaksInFrames = copts60&0x0400 != 0
		c.SwapBordersFacingPages = copts60&0x0800 != 0
	}

	// Copts80 at offset 0x54 (Dop95+, 4 bytes).
	copts80 := binary.LittleEndian.Uint32(dop[0x54:0x58])
	// Low-word bits (mirrors/extends Copts60).
	c.TableWrapLikeWW8 = copts80&0x00000100 != 0
	c.SpaceLikeWW8 = copts80&0x00008000 != 0
	// High-word bits (Word 8.0 additions).
	c.SuppressSpacingAtTopOfPage = copts80&0x00010000 != 0
	c.SpacingInWholePoints = copts80&0x00020000 != 0
	c.PrintBodyTextBeforeHeader = copts80&0x00040000 != 0
	c.NoLeading = copts80&0x00080000 != 0
	c.MwSmallCaps = copts80&0x00200000 != 0
	c.SuppressTopSpacingWP = copts80&0x00400000 != 0
	c.TruncateFontHeightsLikeWP6 = copts80&0x00800000 != 0
	c.SubFontBySize = copts80&0x01000000 != 0
	c.LineWrapLikeWord6 = copts80&0x02000000 != 0
	c.DoNotSuppressParagraphBorders = copts80&0x04000000 != 0
	c.NoExtraLineSpacing = copts80&0x08000000 != 0
	c.SuppressBottomSpacing = copts80&0x10000000 != 0
	c.WpSpaceWidth = copts80&0x20000000 != 0
	c.WpJustification = copts80&0x40000000 != 0
	c.UsePrinterMetrics = copts80&0x80000000 != 0

	// Copts at offset 0x1FC (Dop2000+, 32 bytes).
	if len(dop) >= 0x21C {
		copts := dop[0x1FC:0x21C]
		c.ShapeLayoutLikeWW8 = copts[0]&0x01 != 0
		c.FootnoteLayoutLikeWW8 = copts[0]&0x02 != 0
		c.DoNotUseHTMLParagraphAutoSpacing = copts[0]&0x04 != 0
		c.AdjustLineHeightInTable = copts[0]&0x08 != 0
		c.ForgetLastTabAlignment = copts[0]&0x10 != 0
		c.AutoSpaceLikeWord95 = copts[0]&0x20 != 0
		c.AlignTablesRowByRow = copts[0]&0x40 != 0
		c.LayoutRawTableWidth = copts[0]&0x80 != 0
		c.LayoutTableRowsApart = copts[1]&0x01 != 0
		c.UseWord97LineBreakRules = copts[1]&0x02 != 0
		c.DoNotBreakWrappedTables = copts[1]&0x04 != 0
		c.DoNotSnapToGridInCell = copts[1]&0x08 != 0
		c.SelectFldWithFirstOrLastChar = copts[1]&0x10 != 0
		c.ApplyBreakingRules = copts[1]&0x20 != 0
		c.DoNotWrapTextWithPunct = copts[1]&0x40 != 0
		c.DoNotUseEastAsianBreakRules = copts[1]&0x80 != 0
		c.UseWord2002TableRules = copts[2]&0x01 != 0
		c.GrowAutofit = copts[2]&0x02 != 0
		c.UseNormalStyleForListNumber = copts[2]&0x04 != 0
		c.DoNotUseIndentAsNumberingTabStop = copts[2]&0x08 != 0
		c.UseAltKinsokuLineBreakRules = copts[2]&0x10 != 0
		c.AllowSpaceOfSameStyleInTable = copts[2]&0x20 != 0
		c.DoNotSuppressIndentation = copts[2]&0x40 != 0
		c.DoNotAutofitConstrainedTables = copts[2]&0x80 != 0
		c.AutofitToFirstFixedWidthCell = copts[3]&0x01 != 0
		c.UnderlineTabInNumberingList = copts[3]&0x02 != 0
		c.DisplayHangulFixedWidth = copts[3]&0x04 != 0
		c.SplitPgBreakAndParaMark = copts[3]&0x08 != 0
		c.DoNotVertAlignInTxbx = copts[3]&0x10 != 0
		c.UseAnsiKerningPairs = copts[3]&0x20 != 0
		c.CachedFieldBalance = copts[3]&0x40 != 0
		c.UICompat97To2003 = copts[3]&0x80 != 0
		c.DoNotBreakConstrainedForcedTables = copts[4]&0x01 != 0
		c.DoNotVertAlignCellWithSp = copts[4]&0x02 != 0
	}

	return c
}

// legacyDopViewMode maps a DOP wvkSaved view-mode value to a string.
func legacyDopViewMode(v byte) string {
	switch v {
	case 1:
		return "print"
	case 2:
		return "outline"
	case 3:
		return "masterDoc"
	case 4:
		return "normal"
	case 5:
		return "online"
	case 6:
		return "web"
	default:
		return ""
	}
}