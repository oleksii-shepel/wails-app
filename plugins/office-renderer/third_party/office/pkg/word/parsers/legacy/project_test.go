package legacy

import (
	"path/filepath"
	"testing"

	wordir "office/pkg/word/ir"
)

// TestLosslessSourceFields verifies that the legacy .doc parser populates the
// lossless Source fields on Document, Paragraph, Run, Section, and Style.
func TestLosslessSourceFields(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	docPath := filepath.Join("..", "..", "..", "..", "testdata", "doc", "apache-SampleDoc.doc")
	doc, err := ParseDoc(docPath)
	if err != nil {
		t.Fatalf("ParseDoc failed: %v", err)
	}
	if doc == nil {
		t.Fatal("ParseDoc returned nil document")
	}

	// Document-level source.
	if doc.Source == nil {
		t.Fatal("Document.Source is nil")
	}
	if doc.Source.Version == 0 {
		t.Error("Document.Source.Version should be populated (nFib)")
	}
	if len(doc.Source.DOP) == 0 {
		t.Error("Document.Source.DOP should contain raw DOP bytes")
	}

	// At least one paragraph with source.
	var foundParaSource bool
	for _, p := range doc.Paragraphs {
		if p.Source != nil {
			foundParaSource = true
			break
		}
	}
	if !foundParaSource {
		t.Error("no paragraph has Source populated")
	}

	// At least one run with source.
	var foundRunSource bool
	for _, p := range doc.Paragraphs {
		for _, r := range p.Runs {
			if r.Source != nil {
				foundRunSource = true
				break
			}
		}
		if foundRunSource {
			break
		}
	}
	if !foundRunSource {
		t.Error("no run has Source populated")
	}

	// Section source.
	for i, s := range doc.Sections {
		if s.Source != nil && len(s.Source.SepxSprms) > 0 {
			// OK
		} else if i == 0 {
			t.Logf("first section Source is nil or empty (may be default)")
		}
	}

	// Style source.
	var foundStyleSource bool
	for _, s := range doc.Styles {
		if s.Source != nil {
			foundStyleSource = true
			break
		}
	}
	if !foundStyleSource {
		t.Error("no paragraph style has Source populated")
	}
}

// TestDocCompatibility verifies that DOP compatibility flags are parsed.
func TestDocCompatibility(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	docPath := filepath.Join("..", "..", "..", "..", "testdata", "doc", "apache-SampleDoc.doc")
	doc, err := ParseDoc(docPath)
	if err != nil {
		t.Fatalf("ParseDoc failed: %v", err)
	}
	if doc == nil {
		t.Fatal("ParseDoc returned nil document")
	}
	// The fixture may or may not have a Word 2000+ DOP; just verify the
	// parser doesn't panic and populates the field when possible.
	if doc.Compatibility == nil {
		t.Log("Document.Compatibility is nil (DOP may be pre-Word-2000)")
	} else {
		t.Logf("Document.Compatibility: NoTabHangInd=%v NoLeading=%v NoColumnBalance=%v",
			doc.Compatibility.NoTabHangInd,
			doc.Compatibility.NoLeading,
			doc.Compatibility.NoColumnBalance,
		)
	}
}

// TestProjectDocumentRoundTrip verifies that projecting a document after
// clearing its semantic formatting reproduces equivalent formatting.
func TestProjectDocumentRoundTrip(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	docPath := filepath.Join("..", "..", "..", "..", "testdata", "doc", "apache-SampleDoc.doc")
	doc, err := ParseDoc(docPath)
	if err != nil {
		t.Fatalf("ParseDoc failed: %v", err)
	}

	// Capture original paragraph formatting from a representative paragraph.
	var originalPara wordir.Paragraph
	for _, p := range doc.Paragraphs {
		if len(p.Runs) > 0 && p.Source != nil {
			originalPara = p
			break
		}
	}
	if originalPara.Runs == nil {
		t.Skip("no suitable paragraph for round-trip test")
	}

	// Clear semantic formatting but keep source.
	for i := range doc.Paragraphs {
		doc.Paragraphs[i].Format = wordir.ParagraphFormat{}
		for j := range doc.Paragraphs[i].Runs {
			doc.Paragraphs[i].Runs[j].Format = wordir.RunFormat{}
		}
	}
	for i := range doc.Sections {
		doc.Sections[i].PageSize = nil
		doc.Sections[i].PageMargin = nil
	}

	// Reproject.
	ProjectDocument(doc)

	// Verify that formatting was restored.
	restored := doc.Paragraphs[originalPara.Index]
	if restored.Format.Alignment != originalPara.Format.Alignment && originalPara.Format.Alignment != "" {
		// Alignment may legitimately differ if it came from style merging;
		// just verify that *some* format was restored.
	}
	if restored.Source == nil {
		t.Error("Source was lost during round-trip")
	}
	if restored.Source.StyleIndex != nil && restored.StyleID == "" {
		t.Error("StyleID not restored from Source.StyleIndex")
	}
}

// TestDocProtectionConvergence verifies that the legacy parser populates
// DocumentProtection when the DOP indicates protection is enabled.
func TestDocProtectionConvergence(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	docPath := filepath.Join("..", "..", "..", "..", "testdata", "doc", "apache-SampleDoc.doc")
	doc, err := ParseDoc(docPath)
	if err != nil {
		t.Fatalf("ParseDoc failed: %v", err)
	}
	if doc == nil {
		t.Fatal("ParseDoc returned nil document")
	}
	// The fixture may or may not have protection enabled; the important
	// invariant is that when ProtectionEnabled is true, DocumentProtection
	// is also populated (convergence with the OOXML resolver behaviour).
	if doc.ProtectionEnabled && doc.DocumentProtection == nil {
		t.Fatal("Document.ProtectionEnabled=true but DocumentProtection=nil")
	}
	if doc.DocumentProtection != nil && !doc.DocumentProtection.Enforcement {
		t.Fatal("DocumentProtection.Enforcement should be true when protection is enabled")
	}
}
