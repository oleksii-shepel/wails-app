package docbinary

import (
	"encoding/binary"
	"testing"
)

func TestBTEPageNumbers(t *testing.T) {
	plc := make([]byte, 20)
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], 10)
	binary.LittleEndian.PutUint32(plc[8:12], 20)
	binary.LittleEndian.PutUint32(plc[12:16], 7)
	binary.LittleEndian.PutUint32(plc[16:20], 11)

	got, err := BTEPageNumbers(plc)
	if err != nil { t.Fatal(err) }
	if len(got) != 2 || got[0] != 7 || got[1] != 11 { t.Fatalf("pages = %#v, want [7 11]", got) }
}

func TestParseCHPXFKPExtractsRawGrpprl(t *testing.T) {
	page := make([]byte, FKPPageSize)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], 100)
	binary.LittleEndian.PutUint32(page[4:8], 110)
	page[8] = 20
	page[40] = 3
	copy(page[41:44], []byte{1, 2, 3})

	got, err := ParseCHPXFKP(page)
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].FC != 100 || len(got[0].Grpprl) != 3 { t.Fatalf("CHPX = %#v", got) }
	if got[0].Grpprl[0] != 1 || got[0].Grpprl[2] != 3 { t.Fatalf("grpprl = %#v", got[0].Grpprl) }
}

func TestParsePAPXFKPExtractsStandardRecord(t *testing.T) {
	page := make([]byte, FKPPageSize)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], 200)
	binary.LittleEndian.PutUint32(page[4:8], 220)
	page[8] = 30
	page[60] = 3
	binary.LittleEndian.PutUint16(page[61:63], 9)
	copy(page[63:67], []byte{4, 5, 6, 7})

	got, err := ParsePAPXFKP(page)
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].FC != 200 || !got[0].HasStyle || got[0].StyleIndex != 9 { t.Fatalf("PAPX = %#v", got) }
	if len(got[0].Grpprl) != 4 || got[0].Grpprl[0] != 4 || got[0].Grpprl[3] != 7 { t.Fatalf("grpprl = %#v", got[0].Grpprl) }
}

func TestParsePAPXFKPExtractsExtendedRecord(t *testing.T) {
	page := make([]byte, FKPPageSize)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], 300)
	binary.LittleEndian.PutUint32(page[4:8], 330)
	page[8] = 40
	page[80] = 0
	page[81] = 3
	binary.LittleEndian.PutUint16(page[82:84], 12)
	copy(page[84:88], []byte{8, 9, 10, 11})

	got, err := ParsePAPXFKP(page)
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || !got[0].HasStyle || got[0].StyleIndex != 12 { t.Fatalf("PAPX = %#v", got) }
	if len(got[0].Grpprl) != 4 || got[0].Grpprl[0] != 8 || got[0].Grpprl[3] != 11 { t.Fatalf("grpprl = %#v", got[0].Grpprl) }
}
