package word97

import (
	"encoding/binary"
	"fmt"
	"io"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

// DocumentSettings contains Word97+ DOP semantics that affect document-level
// rendering and authoring state. Binary ownership lives in the sequential
// parser; consumers only project this structure into word/ir.
type DocumentSettings struct {
	DefaultTabStop int
	FootnoteNumberFormat string
	FootnoteNumberStart int
	FootnoteRestart string
	EndnotePosition string
	EndnoteNumberFormat string
	EndnoteNumberStart int
	EndnoteRestart string
	ZoomPercent int
	ViewMode string
	ProtectionEnabled bool
	DocumentProtection *wordir.DocumentProtection
	CreatedAt string
	ModifiedAt string
	LastPrintedAt string
	WordCount int
	CharCount int
	PageCount int
	ParagraphCount int
	MirrorMargins bool
	TrackChangesEnabled bool
	Compatibility *wordir.DocCompatibility
}

// LoadDocumentSettingsReaderAt reads only the FIB-addressed DOP range.
func LoadDocumentSettingsReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) DocumentSettings {
	blob, ok := readTableRange(table, tableSize, fib, layout, fibDOP, 1)
	if !ok { return DocumentSettings{} }
	return DecodeDocumentSettings(blob)
}

// DecodeDocumentSettings decodes the layout/rendering-relevant DOP subset.
func DecodeDocumentSettings(dop []byte) DocumentSettings {
	var s DocumentSettings
	if len(dop) >= 0x0C {
		if v:=int(int16(binary.LittleEndian.Uint16(dop[0x0A:0x0C]))); v>0 { s.DefaultTabStop=v }
	}
	if len(dop) >= 0x04 {
		v:=binary.LittleEndian.Uint16(dop[0x02:0x04]); s.FootnoteRestart=noteRestart(byte(v&3)); if n:=int((v&0xFFFC)>>2);n>0{s.FootnoteNumberStart=n}
	}
	if len(dop) >= 0x38 {
		edn:=binary.LittleEndian.Uint16(dop[0x34:0x36]); edn1:=binary.LittleEndian.Uint16(dop[0x36:0x38])
		s.EndnoteRestart=noteRestart(byte(edn&3)); s.EndnotePosition=endnotePosition(byte(edn1&3)); if n:=int((edn&0xFFFC)>>2);n>0{s.EndnoteNumberStart=n}
		s.EndnoteNumberFormat=nfcFormat(byte((edn1&0x03C0)>>6))
	}
	if len(dop)>=0x1F0 { if v:=nfcFormat(byte(binary.LittleEndian.Uint16(dop[0x1EE:0x1F0])));v!=""{s.EndnoteNumberFormat=v} }
	if len(dop)>=0x1EE { s.FootnoteNumberFormat=nfcFormat(byte(binary.LittleEndian.Uint16(dop[0x1EC:0x1EE]))) } else if len(dop)>=0x38 { edn1:=binary.LittleEndian.Uint16(dop[0x36:0x38]);s.FootnoteNumberFormat=nfcFormat(byte((edn1&0x003C)>>2)) }
	if len(dop)>=0x40 { v:=binary.LittleEndian.Uint16(dop[0x3E:0x40]);s.ZoomPercent=int((v&0x0FF8)>>3);s.ViewMode=viewMode(byte(v&7)) }
	if len(dop)>=0x0A { b:=dop[0x09];s.ProtectionEnabled=b&0x02!=0;s.MirrorMargins=b&0x20!=0;if s.ProtectionEnabled{s.DocumentProtection=&wordir.DocumentProtection{Enforcement:true}} }
	if len(dop)>=0x06 { s.TrackChangesEnabled=dop[0x06]&0x80!=0 }
	if len(dop)>=0x12{s.CreatedAt=formatDTTM(binary.LittleEndian.Uint32(dop[0x0E:0x12]))}
	if len(dop)>=0x16{s.ModifiedAt=formatDTTM(binary.LittleEndian.Uint32(dop[0x12:0x16]))}
	if len(dop)>=0x1A{s.LastPrintedAt=formatDTTM(binary.LittleEndian.Uint32(dop[0x16:0x1A]))}
	if len(dop)>=0x1E{s.WordCount=int(binary.LittleEndian.Uint32(dop[0x1A:0x1E]))}
	if len(dop)>=0x22{s.CharCount=int(binary.LittleEndian.Uint32(dop[0x1E:0x22]))}
	if len(dop)>=0x26{s.PageCount=int(binary.LittleEndian.Uint32(dop[0x22:0x26]))}
	if len(dop)>=0x2A{s.ParagraphCount=int(binary.LittleEndian.Uint32(dop[0x26:0x2A]))}
	s.Compatibility=decodeCompatibility(dop)
	return s
}

// Apply projects canonical DOP semantics onto the stable IR.
func (s DocumentSettings) Apply(doc *wordir.Document) {
	if doc==nil{return}
	if s.DefaultTabStop>0{doc.DefaultTabStop=s.DefaultTabStop}; if s.FootnoteNumberFormat!=""{doc.FootnoteNumberFormat=s.FootnoteNumberFormat}; if s.FootnoteNumberStart>0{doc.FootnoteNumberStart=s.FootnoteNumberStart}; if s.FootnoteRestart!=""{doc.FootnoteRestart=s.FootnoteRestart}
	if s.EndnotePosition!=""{doc.EndnotePosition=s.EndnotePosition}; if s.EndnoteNumberFormat!=""{doc.EndnoteNumberFormat=s.EndnoteNumberFormat}; if s.EndnoteNumberStart>0{doc.EndnoteNumberStart=s.EndnoteNumberStart}; if s.EndnoteRestart!=""{doc.EndnoteRestart=s.EndnoteRestart}
	if s.ZoomPercent>0{doc.ZoomPercent=s.ZoomPercent}; if s.ViewMode!=""{doc.ViewMode=s.ViewMode}; doc.ProtectionEnabled=s.ProtectionEnabled; if s.DocumentProtection!=nil{v:=*s.DocumentProtection;doc.DocumentProtection=&v}
	doc.CreatedAt=s.CreatedAt;doc.ModifiedAt=s.ModifiedAt;doc.LastPrintedAt=s.LastPrintedAt;doc.WordCount=s.WordCount;doc.CharCount=s.CharCount;doc.PageCount=s.PageCount;doc.ParagraphCount=s.ParagraphCount;doc.MirrorMargins=s.MirrorMargins;doc.TrackChangesEnabled=s.TrackChangesEnabled
	if s.Compatibility!=nil{v:=*s.Compatibility;doc.Compatibility=&v}
}

func noteRestart(v byte)string{switch v{case 0:return "continuous";case 1:return "eachSect";case 2:return "eachPage"};return ""}
func endnotePosition(v byte)string{switch v{case 0:return "sectEnd";case 3:return "docEnd"};return ""}
func viewMode(v byte)string{switch v{case 1:return "print";case 2:return "outline";case 3:return "masterDoc";case 4:return "normal";case 5:return "online";case 6:return "web"};return ""}
func nfcFormat(v byte)string{switch v{case 0:return "decimal";case 1:return "upperRoman";case 2:return "lowerRoman";case 3:return "upperLetter";case 4:return "lowerLetter";case 5:return "ordinal";case 6:return "cardinalText";case 7:return "ordinalText";case 22:return "decimalZero";case 23:return "bullet"};return ""}
func formatDTTM(v uint32)string{if v==0{return ""};m:=int(v&0x3F);h:=int((v>>6)&0x1F);d:=int((v>>11)&0x1F);mo:=int((v>>16)&0x0F);y:=int((v>>20)&0x1FF);if mo<1||mo>12||d<1||d>31||h>23||m>59{return ""};return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:00",1900+y,mo,d,h,m)}

func decodeCompatibility(dop []byte)*wordir.DocCompatibility{
	if len(dop)<0x58{return nil};c:=&wordir.DocCompatibility{}
	if len(dop)>=0x0A{v:=binary.LittleEndian.Uint16(dop[0x08:0x0A]);c.NoTabHangInd=v&1!=0;c.NoSpaceRaiseLower=v&2!=0;c.SuppressSpBfAfterPgBrk=v&4!=0;c.WrapTrailSpaces=v&8!=0;c.PrintColBlack=v&0x10!=0;c.NoColumnBalance=v&0x20!=0;c.ConvMailMergeEsc=v&0x40!=0;c.SuppressTopSpacing=v&0x80!=0;c.UseSingleBorderforContiguousCells=v&0x100!=0;c.ShowBreaksInFrames=v&0x400!=0;c.SwapBordersFacingPages=v&0x800!=0}
	v:=binary.LittleEndian.Uint32(dop[0x54:0x58]);c.TableWrapLikeWW8=v&0x100!=0;c.SpaceLikeWW8=v&0x8000!=0;c.SuppressSpacingAtTopOfPage=v&0x10000!=0;c.SpacingInWholePoints=v&0x20000!=0;c.PrintBodyTextBeforeHeader=v&0x40000!=0;c.NoLeading=v&0x80000!=0;c.MwSmallCaps=v&0x200000!=0;c.SuppressTopSpacingWP=v&0x400000!=0;c.TruncateFontHeightsLikeWP6=v&0x800000!=0;c.SubFontBySize=v&0x1000000!=0;c.LineWrapLikeWord6=v&0x2000000!=0;c.DoNotSuppressParagraphBorders=v&0x4000000!=0;c.NoExtraLineSpacing=v&0x8000000!=0;c.SuppressBottomSpacing=v&0x10000000!=0;c.WpSpaceWidth=v&0x20000000!=0;c.WpJustification=v&0x40000000!=0;c.UsePrinterMetrics=v&0x80000000!=0
	if len(dop)>=0x21C{x:=dop[0x1FC:0x21C];c.ShapeLayoutLikeWW8=x[0]&1!=0;c.FootnoteLayoutLikeWW8=x[0]&2!=0;c.DoNotUseHTMLParagraphAutoSpacing=x[0]&4!=0;c.AdjustLineHeightInTable=x[0]&8!=0;c.ForgetLastTabAlignment=x[0]&0x10!=0;c.AutoSpaceLikeWord95=x[0]&0x20!=0;c.AlignTablesRowByRow=x[0]&0x40!=0;c.LayoutRawTableWidth=x[0]&0x80!=0;c.LayoutTableRowsApart=x[1]&1!=0;c.UseWord97LineBreakRules=x[1]&2!=0;c.DoNotBreakWrappedTables=x[1]&4!=0;c.DoNotSnapToGridInCell=x[1]&8!=0;c.SelectFldWithFirstOrLastChar=x[1]&0x10!=0;c.ApplyBreakingRules=x[1]&0x20!=0;c.DoNotWrapTextWithPunct=x[1]&0x40!=0;c.DoNotUseEastAsianBreakRules=x[1]&0x80!=0;c.UseWord2002TableRules=x[2]&1!=0;c.GrowAutofit=x[2]&2!=0;c.UseNormalStyleForListNumber=x[2]&4!=0;c.DoNotUseIndentAsNumberingTabStop=x[2]&8!=0;c.UseAltKinsokuLineBreakRules=x[2]&0x10!=0;c.AllowSpaceOfSameStyleInTable=x[2]&0x20!=0;c.DoNotSuppressIndentation=x[2]&0x40!=0;c.DoNotAutofitConstrainedTables=x[2]&0x80!=0;c.AutofitToFirstFixedWidthCell=x[3]&1!=0;c.UnderlineTabInNumberingList=x[3]&2!=0;c.DisplayHangulFixedWidth=x[3]&4!=0;c.SplitPgBreakAndParaMark=x[3]&8!=0;c.DoNotVertAlignInTxbx=x[3]&0x10!=0;c.UseAnsiKerningPairs=x[3]&0x20!=0;c.CachedFieldBalance=x[3]&0x40!=0;c.UICompat97To2003=x[3]&0x80!=0;c.DoNotBreakConstrainedForcedTables=x[4]&1!=0;c.DoNotVertAlignCellWithSp=x[4]&2!=0}
	return c
}
