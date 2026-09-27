package sax

import (
	"bytes"
	"os"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPass34OOXMLEmbeddedMSGraphOLEPromotesToSemanticChart(t *testing.T) {
	payload, err := os.ReadFile("testdata/msgraph-chart8.bin")
	if err != nil {
		t.Skipf("msgraph-chart8.bin fixture is not installed: %v", err)
	}
	s := &ir.Shape{Kind: ir.ShapeOleObject, OLEObjectProgID: "MSGraph.Chart.8", OLEObjectData: payload}
	promoteOOXMLMSGraphOLE(s)
	if s.Kind != ir.ShapeChart {
		t.Fatal("MSGraph OLE object was not promoted to ShapeChart")
	}
	if s.Kind != ir.ShapeChart {
		t.Fatalf("shape kind = %v, want ShapeChart", s.Kind)
	}
	if len(s.ChartData) == 0 || !bytes.Contains(s.ChartData, []byte("<c:chartSpace")) {
		t.Fatalf("semantic chart payload missing or not canonical chart XML: %d bytes", len(s.ChartData))
	}
}

func TestPass34OrdinaryOLEIsNotPromoted(t *testing.T) {
	s := &ir.Shape{Kind: ir.ShapeOleObject, OLEObjectProgID: "Excel.Sheet.8", OLEObjectData: []byte("not-msgraph")}
	promoteOOXMLMSGraphOLE(s)
	if s.Kind != ir.ShapeOleObject || len(s.ChartData) != 0 {
		t.Fatal("ordinary OLE object was promoted as MSGraph")
	}
	if s.Kind != ir.ShapeOleObject || len(s.ChartData) != 0 {
		t.Fatalf("ordinary OLE object mutated: kind=%v chart=%d", s.Kind, len(s.ChartData))
	}
}
