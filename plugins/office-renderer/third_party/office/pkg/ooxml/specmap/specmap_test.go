package specmap_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"office/pkg/ooxml/specmap"
)

func TestLoadBundlesCapturesNestedElementContext(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "schema.zip")
	writeZip(t, zipPath, map[string]string{
		"test.xsd": `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="outer">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="inner" minOccurs="0" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
  <xs:complexType name="CTStandalone">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`,
	})

	inv, err := specmap.LoadBundles([]string{zipPath})
	if err != nil {
		t.Fatalf("LoadBundles: %v", err)
	}
	if got, want := len(inv.Elements), 3; got != want {
		t.Fatalf("element count = %d, want %d", got, want)
	}

	var foundOuter, foundRef, foundChild bool
	for _, el := range inv.Elements {
		switch {
		case el.Name == "outer":
			foundOuter = el.Global && el.Parent == ""
		case el.Ref == "inner":
			foundRef = el.Parent == "element:outer" && el.MinOccurs == "0" && el.MaxOccurs == "unbounded"
		case el.Name == "child":
			foundChild = el.Parent == "complexType:CTStandalone" && el.Type == "xs:string"
		}
	}
	if !foundOuter {
		t.Fatal("missing expected global outer element record")
	}
	if !foundRef {
		t.Fatal("missing expected nested ref record with parent context")
	}
	if !foundChild {
		t.Fatal("missing expected complexType child record")
	}
}

func TestLoadDirFindsKnownOOXMLRoots(t *testing.T) {
	inv, err := specmap.LoadDir(repoPath(t, "specs", "ooxml"))
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	assertHasGlobal := func(namespace, name string) {
		t.Helper()
		for _, el := range inv.Elements {
			if el.Namespace == namespace && el.Name == name && el.Global {
				return
			}
		}
		t.Fatalf("missing global element %s in namespace %s", name, namespace)
	}

	assertHasGlobal("http://schemas.openxmlformats.org/spreadsheetml/2006/main", "workbook")
	assertHasGlobal("http://schemas.openxmlformats.org/wordprocessingml/2006/main", "document")
	assertHasGlobal("http://schemas.openxmlformats.org/presentationml/2006/main", "presentation")
}

func writeZip(t *testing.T, outPath string, files map[string]string) {
	t.Helper()
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, contents := range files {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := bytes.NewBufferString(contents).WriteTo(entry); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}
