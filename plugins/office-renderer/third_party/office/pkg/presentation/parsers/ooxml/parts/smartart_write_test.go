package parts

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"office/pkg/ooxml/opc"
)

func TestWritePackageSmartArtRoundTrip(t *testing.T) {
	src := filepath.Join("..", "..", "..", "..", "..", "testdata", "pptx", "smartart-simple.pptx")
	opcPkg, err := opc.Open(src)
	if err != nil {
		t.Fatalf("open source pptx: %v", err)
	}

	pkg, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(pkg.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pkg.Slides))
	}
	entry := pkg.Slides[0]
	if len(entry.SmartArtParts) != 1 {
		t.Fatalf("smart art parts = %d, want 1", len(entry.SmartArtParts))
	}
	if len(entry.Slide.CSld.SpTree.SmartArtFrames) != 1 {
		t.Fatalf("smart art frames = %d, want 1", len(entry.Slide.CSld.SpTree.SmartArtFrames))
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	written, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(written)
	if err != nil {
		t.Fatalf("load written package: %v", err)
	}

	if len(loaded.Slides) != 1 {
		t.Fatalf("loaded slides = %d, want 1", len(loaded.Slides))
	}
	loadedEntry := loaded.Slides[0]
	if len(loadedEntry.SmartArtParts) != 1 {
		t.Fatalf("loaded smart art parts = %d, want 1", len(loadedEntry.SmartArtParts))
	}
	if len(loadedEntry.Slide.CSld.SpTree.SmartArtFrames) != 1 {
		t.Fatalf("loaded smart art frames = %d, want 1", len(loadedEntry.Slide.CSld.SpTree.SmartArtFrames))
	}

	part := loadedEntry.SmartArtParts[0]
	if len(part.DataModelXML) == 0 {
		t.Fatal("data model XML missing after round-trip")
	}
	if len(part.LayoutXML) == 0 {
		t.Fatal("layout XML missing after round-trip")
	}
	if len(part.ColorsXML) == 0 {
		t.Fatal("colors XML missing after round-trip")
	}
	if len(part.QuickStyleXML) == 0 {
		t.Fatal("quick style XML missing after round-trip")
	}
	if len(part.DrawingXML) == 0 {
		t.Fatal("drawing XML missing after round-trip")
	}

	// Verify slide-level diagram relationships exist.
	relsDoc, ok, err := written.Relationships("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatalf("slide rels: %v", err)
	}
	if !ok {
		t.Fatal("slide rels missing")
	}
	wantTypes := map[string]bool{
		relTypeDiagramData:       true,
		relTypeDiagramLayout:     true,
		relTypeDiagramColors:     true,
		relTypeDiagramQuickStyle: true,
		relTypeDiagramDrawing:    true,
	}
	foundTypes := map[string]bool{}
	idByType := map[string]string{}
	for _, r := range relsDoc.Relationships {
		if wantTypes[r.Type] {
			foundTypes[r.Type] = true
			idByType[r.Type] = r.ID
		}
	}
	for typ := range wantTypes {
		if !foundTypes[typ] {
			t.Errorf("missing slide rel for %s", typ)
		}
	}

	// The graphicFrame's dgm:relIds should reference the new slide-level IDs.
	frame := loadedEntry.Slide.CSld.SpTree.SmartArtFrames[0]
	if got, want := frame.RelIDs["dm"], idByType[relTypeDiagramData]; got != want {
		t.Errorf("frame dm rel id = %q, want %q", got, want)
	}
	if got, want := frame.RelIDs["lo"], idByType[relTypeDiagramLayout]; got != want {
		t.Errorf("frame lo rel id = %q, want %q", got, want)
	}
	if got, want := frame.RelIDs["cs"], idByType[relTypeDiagramColors]; got != want {
		t.Errorf("frame cs rel id = %q, want %q", got, want)
	}
	if got, want := frame.RelIDs["qs"], idByType[relTypeDiagramQuickStyle]; got != want {
		t.Errorf("frame qs rel id = %q, want %q", got, want)
	}

	// The data model should have a .rels file pointing at its related parts.
	dataRels, ok, err := written.Relationships("ppt/diagrams/data1.xml")
	if err != nil {
		t.Fatalf("data rels: %v", err)
	}
	if !ok {
		t.Fatal("data model rels missing")
	}
	dataRelTypes := map[string]bool{}
	for _, r := range dataRels.Relationships {
		dataRelTypes[r.Type] = true
	}
	if !dataRelTypes[relTypeDiagramLayout] {
		t.Error("data model rels missing layout")
	}
	if !dataRelTypes[relTypeDiagramColors] {
		t.Error("data model rels missing colors")
	}
	if !dataRelTypes[relTypeDiagramQuickStyle] {
		t.Error("data model rels missing quick style")
	}

	// The pre-rendered drawing part should survive the round-trip.
	if _, ok := written.Part("ppt/diagrams/drawing1.xml"); !ok {
		t.Error("missing part ppt/diagrams/drawing1.xml")
	}
}

func TestSmartArtFrameCaptureOnLoad(t *testing.T) {
	src := filepath.Join("..", "..", "..", "..", "..", "testdata", "pptx", "smartart-simple.pptx")
	opcPkg, err := opc.Open(src)
	if err != nil {
		t.Fatalf("open source pptx: %v", err)
	}
	pkg, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(pkg.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pkg.Slides))
	}
	frames := pkg.Slides[0].Slide.CSld.SpTree.SmartArtFrames
	if len(frames) != 1 {
		t.Fatalf("smart art frames = %d, want 1", len(frames))
	}
	frame := frames[0]
	if frame.ID != 4 {
		t.Errorf("frame id = %d, want 4", frame.ID)
	}
	if frame.Name != "Diagram 3" {
		t.Errorf("frame name = %q, want Diagram 3", frame.Name)
	}
	if len(frame.RawXML) == 0 {
		t.Fatal("frame raw XML missing")
	}
	if frame.RelIDs == nil || frame.RelIDs["dm"] == "" {
		t.Fatal("frame data model rel id missing")
	}
}

// TestSmartArtWriteBackFile is a smoke test that writes a real SmartArt
// presentation to a temporary file. It ensures the output can be opened by the
// OPC reader and that the diagram parts are present.
func TestSmartArtWriteBackFile(t *testing.T) {
	src := filepath.Join("..", "..", "..", "..", "..", "testdata", "pptx", "smartart-simple.pptx")
	opcPkg, err := opc.Open(src)
	if err != nil {
		t.Fatalf("open source pptx: %v", err)
	}
	pkg, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	dir := t.TempDir()
	dst := filepath.Join(dir, "out.pptx")
	f, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create output file: %v", err)
	}
	if err := WritePackage(pkg, f); err != nil {
		f.Close()
		t.Fatalf("write package: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close output file: %v", err)
	}

	written, err := opc.Open(dst)
	if err != nil {
		t.Fatalf("open written file: %v", err)
	}
	for _, name := range []string{"ppt/diagrams/data1.xml", "ppt/diagrams/layout1.xml", "ppt/diagrams/colors1.xml", "ppt/diagrams/quickStyle1.xml"} {
		if _, ok := written.Part(name); !ok {
			t.Errorf("missing part %s", name)
		}
	}
}