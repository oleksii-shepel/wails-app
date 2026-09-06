package word97

import "testing"

func TestParsePreservationAndOLERuns(t *testing.T) {
	meta := ParsePreservation([]byte(`{
		"version":1,
		"ole":[{
			"progId":"Excel.Sheet.8",
			"data":"AQID",
			"mime":"application/vnd.ms-excel",
			"type":"Embed",
			"text":"Workbook"
		}]
	}`))
	if meta == nil {
		t.Fatal("ParsePreservation returned nil")
	}
	runs := meta.OLERuns()
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].Kind != "olePlaceholder" || runs[0].Text != "Workbook" {
		t.Fatalf("run = %#v", runs[0])
	}
	if runs[0].OLEProgID != "Excel.Sheet.8" || runs[0].OLEMIME != "application/vnd.ms-excel" || runs[0].OLEType != "Embed" {
		t.Fatalf("OLE metadata not preserved: %#v", runs[0])
	}
	if len(runs[0].OLEData) != 3 || runs[0].OLEData[0] != 1 || runs[0].OLEData[2] != 3 {
		t.Fatalf("OLEData = %v", runs[0].OLEData)
	}
	meta.OLE[0].Data[0] = 9
	if runs[0].OLEData[0] != 1 {
		t.Fatal("OLERuns aliased preservation payload")
	}
}

func TestParsePreservationRejectsUnknownVersion(t *testing.T) {
	if got := ParsePreservation([]byte(`{"version":2}`)); got != nil {
		t.Fatalf("ParsePreservation future version = %#v, want nil", got)
	}
	if got := ParsePreservation([]byte(`not-json`)); got != nil {
		t.Fatalf("ParsePreservation malformed payload = %#v, want nil", got)
	}
}
