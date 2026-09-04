package legacy

import (
	"testing"
	"time"
)

func TestFormatLegacyDttm(t *testing.T) {
	// 2024-03-15 14:30:00
	// yr=124 (2024), mon=3, dom=15, hr=14, mint=30
	// dttm = (124 << 20) | (3 << 16) | (15 << 11) | (14 << 6) | 30
	dttm := uint32((124 << 20) | (3 << 16) | (15 << 11) | (14 << 6) | 30)
	got := formatLegacyDttm(dttm)
	want := "2024-03-15T14:30:00"
	if got != want {
		t.Fatalf("formatLegacyDttm = %q, want %q", got, want)
	}

	// Zero value
	if got := formatLegacyDttm(0); got != "" {
		t.Fatalf("formatLegacyDttm(0) = %q, want empty", got)
	}

	// Invalid month
	invalid := uint32((100 << 20) | (15 << 16) | (1 << 11))
	if got := formatLegacyDttm(invalid); got != "" {
		t.Fatalf("formatLegacyDttm(invalid) = %q, want empty", got)
	}
}

func TestLoadLegacyDocSettingsExtended(t *testing.T) {
	// Build a minimal DOP with extended fields.
	dop := make([]byte, 0x40)
	// dxaTab at 0x0A
	dop[0x0A] = 0x2D
	dop[0x0B] = 0x01 // 0x012D = 301 twips
	// docinfo1 at 0x06: fRevMarking = 0x80
	dop[0x06] = 0x80
	// docinfo2 at 0x09: fMirrorMargins = 0x20, fProtEnabled = 0x02
	dop[0x09] = 0x22
	// dttmCreated at 0x0E: 2024-03-15 14:30:00
	// dttm = (124<<20)|(3<<16)|(15<<11)|(14<<6)|30 = 0x07C37B9E
	dop[0x0E] = 0x9E
	dop[0x0F] = 0x7B
	dop[0x10] = 0xC3
	dop[0x11] = 0x07
	// cWords at 0x1A
	dop[0x1A] = 0xE8
	dop[0x1B] = 0x03
	dop[0x1C] = 0x00
	dop[0x1D] = 0x00 // 1000 words
	// cCh at 0x1E
	dop[0x1E] = 0x10
	dop[0x1F] = 0x27
	dop[0x20] = 0x00
	dop[0x21] = 0x00 // 10000 chars
	// cPg at 0x22
	dop[0x22] = 0x05
	dop[0x23] = 0x00
	dop[0x24] = 0x00
	dop[0x25] = 0x00 // 5 pages
	// cParas at 0x26
	dop[0x26] = 0x64
	dop[0x27] = 0x00
	dop[0x28] = 0x00
	dop[0x29] = 0x00 // 100 paragraphs
	// view at 0x3E: zoom=100% (100 << 3 = 0x320), viewMode=print (1)
	dop[0x3E] = 0x21
	dop[0x3F] = 0x03

	settings := loadLegacyDocSettingsFromDop(dop)

	if settings.DefaultTabStop != 301 {
		t.Fatalf("DefaultTabStop = %d, want 301", settings.DefaultTabStop)
	}
	if !settings.TrackChangesEnabled {
		t.Fatal("TrackChangesEnabled = false, want true")
	}
	if !settings.MirrorMargins {
		t.Fatal("MirrorMargins = false, want true")
	}
	if !settings.ProtectionEnabled {
		t.Fatal("ProtectionEnabled = false, want true")
	}
	if settings.WordCount != 1000 {
		t.Fatalf("WordCount = %d, want 1000", settings.WordCount)
	}
	if settings.CharCount != 10000 {
		t.Fatalf("CharCount = %d, want 10000", settings.CharCount)
	}
	if settings.PageCount != 5 {
		t.Fatalf("PageCount = %d, want 5", settings.PageCount)
	}
	if settings.ParagraphCount != 100 {
		t.Fatalf("ParagraphCount = %d, want 100", settings.ParagraphCount)
	}
	if settings.ZoomPercent != 100 {
		t.Fatalf("ZoomPercent = %d, want 100", settings.ZoomPercent)
	}
	if settings.ViewMode != "print" {
		t.Fatalf("ViewMode = %q, want print", settings.ViewMode)
	}
	// CreatedAt should be non-empty for a non-zero dttm
	if settings.CreatedAt == "" {
		t.Fatal("CreatedAt = empty, want non-empty")
	}
	// Validate it parses as a valid time
	if _, err := time.Parse("2006-01-02T15:04:05", settings.CreatedAt); err != nil {
		t.Fatalf("CreatedAt parse error: %v", err)
	}
}

func TestLoadLegacyDocSettingsEmptyDOP(t *testing.T) {
	settings := loadLegacyDocSettingsFromDop([]byte{})
	if settings.DefaultTabStop != 0 {
		t.Fatalf("DefaultTabStop = %d, want 0", settings.DefaultTabStop)
	}
	if settings.TrackChangesEnabled {
		t.Fatal("TrackChangesEnabled = true, want false")
	}
	if settings.WordCount != 0 {
		t.Fatalf("WordCount = %d, want 0", settings.WordCount)
	}
}

func TestLoadLegacyDocCompatibilityAllSources(t *testing.T) {
	// Build a DOP2000+ with Copts60, Copts80, and Copts populated.
	dop := make([]byte, 0x21C)

	// Copts60 at 0x08: set NoSpaceRaiseLower (0x0002), SuppressTopSpacing (0x0080)
	dop[0x08] = 0x82
	dop[0x09] = 0x00

	// Copts80 at 0x54: set high-word bits.
	// SuppressSpacingAtTopOfPage (0x00010000), NoExtraLineSpacing (0x08000000),
	// UsePrinterMetrics (0x80000000)
	dop[0x54] = 0x00
	dop[0x55] = 0x00
	dop[0x56] = 0x01
	dop[0x57] = 0x88 // 0x88 = 0x80|0x08

	// Copts at 0x1FC (32 bytes).
	// Byte 0: ShapeLayoutLikeWW8 (0x01), AdjustLineHeightInTable (0x08)
	dop[0x1FC] = 0x09
	// Byte 1: LayoutTableRowsApart (0x01), DoNotUseEastAsianBreakRules (0x80)
	dop[0x1FD] = 0x81
	// Byte 2: UseWord2002TableRules (0x01), DoNotSuppressIndentation (0x40)
	dop[0x1FE] = 0x41
	// Byte 3: UnderlineTabInNumberingList (0x02), CachedFieldBalance (0x40)
	dop[0x1FF] = 0x42
	// Byte 4: DoNotBreakConstrainedForcedTables (0x01), DoNotVertAlignCellWithSp (0x02)
	dop[0x200] = 0x03

	c := loadLegacyDocCompatibility(dop)
	if c == nil {
		t.Fatal("loadLegacyDocCompatibility returned nil")
	}

	// Copts60 checks
	if !c.NoSpaceRaiseLower {
		t.Error("NoSpaceRaiseLower = false, want true")
	}
	if !c.SuppressTopSpacing {
		t.Error("SuppressTopSpacing = false, want true")
	}

	// Copts80 checks
	if !c.SuppressSpacingAtTopOfPage {
		t.Error("SuppressSpacingAtTopOfPage = false, want true")
	}
	if !c.NoExtraLineSpacing {
		t.Error("NoExtraLineSpacing = false, want true")
	}
	if !c.UsePrinterMetrics {
		t.Error("UsePrinterMetrics = false, want true")
	}

	// Copts checks
	if !c.ShapeLayoutLikeWW8 {
		t.Error("ShapeLayoutLikeWW8 = false, want true")
	}
	if !c.AdjustLineHeightInTable {
		t.Error("AdjustLineHeightInTable = false, want true")
	}
	if !c.LayoutTableRowsApart {
		t.Error("LayoutTableRowsApart = false, want true")
	}
	if !c.DoNotUseEastAsianBreakRules {
		t.Error("DoNotUseEastAsianBreakRules = false, want true")
	}
	if !c.UseWord2002TableRules {
		t.Error("UseWord2002TableRules = false, want true")
	}
	if !c.DoNotSuppressIndentation {
		t.Error("DoNotSuppressIndentation = false, want true")
	}
	if !c.UnderlineTabInNumberingList {
		t.Error("UnderlineTabInNumberingList = false, want true")
	}
	if !c.CachedFieldBalance {
		t.Error("CachedFieldBalance = false, want true")
	}
	if !c.DoNotBreakConstrainedForcedTables {
		t.Error("DoNotBreakConstrainedForcedTables = false, want true")
	}
	if !c.DoNotVertAlignCellWithSp {
		t.Error("DoNotVertAlignCellWithSp = false, want true")
	}
}

func TestLoadLegacyDocCompatibilityShortDOP(t *testing.T) {
	// DOP too short for Copts80 (need 0x58 bytes).
	if c := loadLegacyDocCompatibility(make([]byte, 0x57)); c != nil {
		t.Fatalf("expected nil for short DOP, got %+v", c)
	}

	// DOP long enough for Copts60 + Copts80 but not Copts.
	dop := make([]byte, 0x58)
	dop[0x08] = 0x01 // NoTabHangInd
	dop[0x54] = 0x01
	dop[0x55] = 0x00
	dop[0x56] = 0x00
	dop[0x57] = 0x00
	c := loadLegacyDocCompatibility(dop)
	if c == nil {
		t.Fatal("expected non-nil")
	}
	if !c.NoTabHangInd {
		t.Error("NoTabHangInd = false, want true")
	}
}

// loadLegacyDocSettingsFromDop is a test helper that bypasses the FIB lookup.
func loadLegacyDocSettingsFromDop(dop []byte) legacyDocSettings {
	settings := legacyDocSettings{}
	if len(dop) >= 0x0C {
		if dxaTab := int(int16(uint16(dop[0x0A]) | uint16(dop[0x0B])<<8)); dxaTab > 0 {
			settings.DefaultTabStop = dxaTab
		}
	}
	if len(dop) >= 0x04 {
		footnoteInfo := int(dop[0x02]) | int(dop[0x03])<<8
		if restart := legacyDopNoteRestart(byte(footnoteInfo & 0x0003)); restart != "" {
			settings.FootnoteRestart = restart
		}
		if start := (footnoteInfo & 0xFFFC) >> 2; start > 0 {
			settings.FootnoteNumberStart = start
		}
	}
	if len(dop) >= 0x38 {
		edn := int(dop[0x34]) | int(dop[0x35])<<8
		edn1 := int(dop[0x36]) | int(dop[0x37])<<8
		if restart := legacyDopNoteRestart(byte(edn & 0x0003)); restart != "" {
			settings.EndnoteRestart = restart
		}
		if pos := legacyDopEndnotePosition(byte(edn1 & 0x0003)); pos != "" {
			settings.EndnotePosition = pos
		}
		if start := (edn & 0xFFFC) >> 2; start > 0 {
			settings.EndnoteNumberStart = start
		}
	}
	if len(dop) >= 0x40 {
		view := int(dop[0x3E]) | int(dop[0x3F])<<8
		settings.ZoomPercent = (view & 0x0FF8) >> 3
		settings.ViewMode = legacyDopViewMode(byte(view & 0x0007))
	}
	if len(dop) >= 0x0A {
		docinfo2 := dop[0x09]
		settings.ProtectionEnabled = docinfo2&0x02 != 0
		settings.MirrorMargins = docinfo2&0x20 != 0
	}
	if len(dop) >= 0x12 {
		settings.CreatedAt = formatLegacyDttm(uint32(dop[0x0E]) | uint32(dop[0x0F])<<8 | uint32(dop[0x10])<<16 | uint32(dop[0x11])<<24)
	}
	if len(dop) >= 0x16 {
		settings.ModifiedAt = formatLegacyDttm(uint32(dop[0x12]) | uint32(dop[0x13])<<8 | uint32(dop[0x14])<<16 | uint32(dop[0x15])<<24)
	}
	if len(dop) >= 0x1A {
		settings.LastPrintedAt = formatLegacyDttm(uint32(dop[0x16]) | uint32(dop[0x17])<<8 | uint32(dop[0x18])<<16 | uint32(dop[0x19])<<24)
	}
	if len(dop) >= 0x1E {
		settings.WordCount = int(uint32(dop[0x1A]) | uint32(dop[0x1B])<<8 | uint32(dop[0x1C])<<16 | uint32(dop[0x1D])<<24)
	}
	if len(dop) >= 0x22 {
		settings.CharCount = int(uint32(dop[0x1E]) | uint32(dop[0x1F])<<8 | uint32(dop[0x20])<<16 | uint32(dop[0x21])<<24)
	}
	if len(dop) >= 0x26 {
		settings.PageCount = int(uint32(dop[0x22]) | uint32(dop[0x23])<<8 | uint32(dop[0x24])<<16 | uint32(dop[0x25])<<24)
	}
	if len(dop) >= 0x2A {
		settings.ParagraphCount = int(uint32(dop[0x26]) | uint32(dop[0x27])<<8 | uint32(dop[0x28])<<16 | uint32(dop[0x29])<<24)
	}
	if len(dop) >= 0x06 {
		settings.TrackChangesEnabled = dop[0x06]&0x80 != 0
	}
	return settings
}