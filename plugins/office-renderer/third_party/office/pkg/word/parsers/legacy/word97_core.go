package legacy

import (
	"encoding/binary"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

type legacyWord97DocCore struct {
	TableStreamName string
	TableStream     []byte
	// DataStream carries OfficeArt blips and legacy object payloads. Word 97+
	// documents commonly split drawing metadata between the table stream and
	// this stream, so drawing resolution must not assume WordDocument owns it.
	DataStream       []byte
	FibBase          int
	CbRgFcLcb        int
	StoryCounts      legacyStoryCounts
	CcpText          int
	ClxData          []byte
	ComplexFileTable *legacyComplexFileTable
	ChpxEntries      []chpxEntry
	PapxEntries      []papxEntry
	ListTables       *legacyListTables
	StyleSheet       *legacyStyleSheet
	DocSettings      legacyDocSettings
	FontFamilies     map[string]wordir.FontFamilyInfo
	EmbeddedFonts    []*wordir.EmbeddedFont
	// Version is the nFib value from the FIB.
	Version uint16
}

func loadLegacyWord97DocCore(wordStream []byte, f *cfb.File) (*legacyWord97DocCore, bool) {
	if len(wordStream) < 34 {
		return nil, false
	}
	nFib, ok := legacyDocFibVersion(wordStream)
	if !ok || nFib < legacyDocFibWord97 {
		return nil, false
	}
	flags := binary.LittleEndian.Uint16(wordStream[10:12])
	tableStreamName := "0Table"
	if flags&(1<<9) != 0 {
		tableStreamName = "1Table"
	}
	tableStream, ok := f.Stream(tableStreamName)
	if !ok || len(tableStream) == 0 {
		return nil, false
	}
	var dataStream []byte
	if ds, ok := f.Stream("Data"); ok && len(ds) > 0 {
		dataStream = ds
	} else if ds, ok := f.Stream("DataStream"); ok && len(ds) > 0 {
		dataStream = ds
	}

	pos := 32
	if pos+2 > len(wordStream) {
		return nil, false
	}
	csw := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	lw97Base := pos + 2 + csw*2 + 2
	if lw97Base+16 > len(wordStream) {
		return nil, false
	}
	pos += 2 + csw*2
	if pos+2 > len(wordStream) {
		return nil, false
	}
	cslw := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	counts := readLegacyStoryCounts(wordStream, lw97Base, cslw)
	pos += 2 + cslw*4
	if pos+2 > len(wordStream) {
		return nil, false
	}
	cbRgFcLcb := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	pos += 2
	fibBase := pos

	const clxIndex = 33
	if clxIndex >= cbRgFcLcb || fibBase+(clxIndex+1)*8 > len(wordStream) {
		return nil, false
	}
	fcClx := int(binary.LittleEndian.Uint32(wordStream[fibBase+clxIndex*8:]))
	lcbClx := int(binary.LittleEndian.Uint32(wordStream[fibBase+clxIndex*8+4:]))
	if lcbClx <= 0 || fcClx < 0 || fcClx+lcbClx > len(tableStream) {
		return nil, false
	}
	clxData := tableStream[fcClx : fcClx+lcbClx]
	cft := loadLegacyComplexFileTable(clxData)
	if cft == nil {
		return nil, false
	}
	papxEntries := loadPapxEntries(wordStream, tableStream, fibBase, cbRgFcLcb)
	return &legacyWord97DocCore{
		TableStreamName:  tableStreamName,
		TableStream:      tableStream,
		DataStream:       dataStream,
		FibBase:          fibBase,
		CbRgFcLcb:        cbRgFcLcb,
		StoryCounts:      counts,
		CcpText:          counts.Text,
		ClxData:          clxData,
		ComplexFileTable: cft,
		ChpxEntries:      loadChpxEntries(wordStream, tableStream, fibBase, cbRgFcLcb),
		PapxEntries:      papxEntries,
		ListTables:       loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb, papxEntries),
		StyleSheet:       loadLegacyStyleSheet(wordStream, tableStream, fibBase, cbRgFcLcb),
		DocSettings:      loadLegacyDocSettings(wordStream, tableStream, fibBase, cbRgFcLcb),
		FontFamilies:     loadLegacyDocFontFamilies(wordStream, tableStream, fibBase, cbRgFcLcb),
		EmbeddedFonts:    loadLegacyDocEmbeddedFonts(wordStream, tableStream, fibBase, cbRgFcLcb),
		Version:          nFib,
	}, true
}
