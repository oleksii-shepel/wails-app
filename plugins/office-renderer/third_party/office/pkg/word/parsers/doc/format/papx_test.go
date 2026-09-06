package docformat

import (
	"encoding/binary"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestParsePAPXFKPPreservesStyleAndSemanticMetadata(t *testing.T) {
	page := make([]byte, 512)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], 200)
	binary.LittleEndian.PutUint32(page[4:8], 240)
	page[8] = 30
	page[60] = 3
	binary.LittleEndian.PutUint16(page[61:63], 9)
	copy(page[63:67], []byte{1, 2, 3, 4})

	row := &TableRowProps{TableJc: "center"}
	depth := 2
	got := ParsePAPXFKP(page, func(grpprl []byte) (wordir.ParagraphFormat, PAPXMetadata) {
		if len(grpprl) != 4 { t.Fatalf("grpprl length = %d", len(grpprl)) }
		return wordir.ParagraphFormat{KeepNext: true}, PAPXMetadata{TableDepth: depth, TableRowProps: row}
	})
	if len(got) != 1 { t.Fatalf("entries = %d", len(got)) }
	if got[0].FC != 200 || got[0].Format.BinaryStyleIndex == nil || *got[0].Format.BinaryStyleIndex != 9 { t.Fatalf("entry = %#v", got[0]) }
	if !got[0].Format.KeepNext || got[0].Meta.TableDepth != 2 || got[0].Meta.TableRowProps == nil || got[0].Meta.TableRowProps.TableJc != "center" { t.Fatalf("semantic entry = %#v", got[0]) }
	if len(got[0].Sprms) != 4 || got[0].Sprms[0] != 1 || got[0].Sprms[3] != 4 { t.Fatalf("sprms = %#v", got[0].Sprms) }
}

func TestTableRowPropsValidBoundariesAndMaxCellIndex(t *testing.T) {
	p := &TableRowProps{CellBoundaries: []int{0, 100, 250}, HMerge: map[int]byte{3: 1}, CellFormats: map[int]wordir.TableCellFormat{4: {VertAlign: "center"}}}
	if got := p.ValidCellBoundaries(); len(got) != 3 { t.Fatalf("boundaries = %#v", got) }
	if got := p.MaxCellIndex(); got != 5 { t.Fatalf("max cell index = %d, want 5", got) }
	p.CellBoundaries = []int{0, 100, 90}
	if got := p.ValidCellBoundaries(); got != nil { t.Fatalf("invalid boundaries accepted: %#v", got) }
}
