package doc

import (
	"os"
	"strings"
	"testing"
)

func TestSourceProjectionDoesNotReopenWord97(t *testing.T) {
	data, err := os.ReadFile("materialize_source.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"OpenBytes(",
		"OpenFileBacked(",
		"StreamReaderAt(",
		"word97.OpenCoreReaderAt(",
		"docbinary.ReadWord97FIBReaderAt(",
		"docbinary.ParseWord97Layout(",
		"word97.LoadDocumentSettingsReaderAt(",
		"word97.LoadFontFamiliesReaderAt(",
		"word97.LoadStyleCatalogReaderAt(",
		"word97.LoadNumberingCatalogReaderAt(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("source projection must use already-open canonical state: found %q", forbidden)
		}
	}
	for _, want := range []string{
		"applySequentialDocumentSourceCore",
		"opened.Source",
		"opened.Settings.Apply(doc)",
		"opened.FontFamilies",
		"opened.Styles",
		"opened.Numbering",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("source projection must consume %q", want)
		}
	}
}

func TestNormalWord97MaterializationReusesOpenedCore(t *testing.T) {
	data, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		"func openWord97CFB(",
		"func materializeWord97CFB(",
		"openWord97CFB(f)",
		"applySequentialDocumentSourceCore(f, opened, doc)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("normal Word97 materialization must reuse canonical open: missing %q", want)
		}
	}
	if strings.Count(source, "word97.OpenReaderAt(") != 1 {
		t.Fatalf("doc.go must have one canonical Word97 open call, got %d", strings.Count(source, "word97.OpenReaderAt("))
	}
	for _, forbidden := range []string{
		"attachSequentialDocumentSource(",
		"attachSequentialDocumentSourcePath(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("normal parser must not reopen after materialization: found %q", forbidden)
		}
	}
}
