package legacy

import (
	"encoding/binary"
	"testing"
)

func TestParseATRDPre10Records(t *testing.T) {
	// Build a minimal wordStream FIB with one FibRgFcLcb entry at index 4.
	// fibBase = 0, cbRgFcLcb = 10, so FIB is 2 + 10*8 = 82 bytes.
	const (
		fibBase     = 2
		cbRgFcLcb   = 10
		fibSize     = fibBase + cbRgFcLcb*8
		tableOffset = 0
	)

	// ATRDPre10 record: "JD" author, 2023-03-15 date.
	plcfAtnBkf := buildLegacyATRDPre10PLC([]int{4}, []string{"JD"}, []string{"2023-03-15"})

	wordStream := make([]byte, fibSize)
	binary.LittleEndian.PutUint16(wordStream[0:2], uint16(cbRgFcLcb))
	// Write fc=0, lcb=len(plcfAtnBkf) at FIB index 4.
	putFibFcLcb(wordStream, fibBase, 4, tableOffset, len(plcfAtnBkf))

	tableStream := make([]byte, len(plcfAtnBkf))
	copy(tableStream[tableOffset:], plcfAtnBkf)

	authorByID, dateByID := parseATRDPre10Records(tableStream, wordStream, fibBase, cbRgFcLcb, 1)
	if authorByID[1] != "JD" {
		t.Errorf("author[1] = %q, want %q", authorByID[1], "JD")
	}
	if dateByID[1] != "2023-03-15" {
		t.Errorf("date[1] = %q, want %q", dateByID[1], "2023-03-15")
	}
}