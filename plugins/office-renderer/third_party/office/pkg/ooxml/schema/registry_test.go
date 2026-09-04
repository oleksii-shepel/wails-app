package schema

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildBundlesInventoriesGlobalDeclarations(t *testing.T) {
	bundle := writeSchemaZip(t, map[string]string{
		"word.xsd": `<?xml version="1.0" encoding="UTF-8"?>
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test:word">
  <xsd:element name="document" type="w:CT_Document"/>
  <xsd:complexType name="CT_Document">
    <xsd:sequence>
      <xsd:element name="body" type="xsd:string"/>
    </xsd:sequence>
  </xsd:complexType>
  <xsd:simpleType name="ST_OnOff">
    <xsd:restriction base="xsd:string"/>
  </xsd:simpleType>
  <xsd:group name="EG_RunLevelElts"/>
  <xsd:attribute name="val" type="xsd:string"/>
  <xsd:attributeGroup name="AG_Attrs"/>
</xsd:schema>`,
	})

	reg, err := BuildBundles([]string{bundle})
	if err != nil {
		t.Fatalf("BuildBundles: %v", err)
	}
	if len(reg.Declarations) != 6 {
		t.Fatalf("declarations = %d, want 6", len(reg.Declarations))
	}
	if got := countKind(reg.Declarations, DeclKindElement); got != 1 {
		t.Fatalf("element count = %d, want 1", got)
	}
	if got := countKind(reg.Declarations, DeclKindComplexType); got != 1 {
		t.Fatalf("complexType count = %d, want 1", got)
	}
	if got := countKind(reg.Declarations, DeclKindSimpleType); got != 1 {
		t.Fatalf("simpleType count = %d, want 1", got)
	}
	if got := reg.Namespaces[0].Counts[DeclKindAttributeGroup]; got != 1 {
		t.Fatalf("attributeGroup count = %d, want 1", got)
	}
}

func TestWriteOutputs(t *testing.T) {
	reg := &Registry{
		Declarations: []Decl{{
			Bundle:    "bundle.zip",
			Schema:    "word.xsd",
			Namespace: "urn:test:word",
			Kind:      DeclKindElement,
			Name:      "document",
			Type:      "w:CT_Document",
		}},
		Schemas: []SchemaSummary{{
			Bundle: "bundle.zip",
			Schema: "word.xsd",
			NS:     "urn:test:word",
			Counts: map[DeclKind]int{DeclKindElement: 1},
		}},
		Namespaces: []NamespaceSummary{{
			Namespace: "urn:test:word",
			Counts:    map[DeclKind]int{DeclKindElement: 1},
		}},
	}

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "registry.md")
	if err := WriteMarkdown(reg, mdPath); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("ReadFile(md): %v", err)
	}
	if !strings.Contains(string(mdData), "OOXML Schema Registry") {
		t.Fatalf("generated markdown missing title: %s", string(mdData))
	}
}

func TestFilterProfile(t *testing.T) {
	reg := &Registry{
		Declarations: []Decl{
			{Namespace: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Kind: DeclKindElement, Name: "document"},
			{Namespace: "http://schemas.openxmlformats.org/spreadsheetml/2006/main", Kind: DeclKindElement, Name: "workbook"},
			{Namespace: "http://schemas.openxmlformats.org/presentationml/2006/main", Kind: DeclKindElement, Name: "presentation"},
		},
		Schemas: []SchemaSummary{
			{NS: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Counts: map[DeclKind]int{DeclKindElement: 1}},
			{NS: "http://schemas.openxmlformats.org/spreadsheetml/2006/main", Counts: map[DeclKind]int{DeclKindElement: 1}},
			{NS: "http://schemas.openxmlformats.org/presentationml/2006/main", Counts: map[DeclKind]int{DeclKindElement: 1}},
		},
		Namespaces: []NamespaceSummary{
			{Namespace: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Counts: map[DeclKind]int{DeclKindElement: 1}},
			{Namespace: "http://schemas.openxmlformats.org/spreadsheetml/2006/main", Counts: map[DeclKind]int{DeclKindElement: 1}},
			{Namespace: "http://schemas.openxmlformats.org/presentationml/2006/main", Counts: map[DeclKind]int{DeclKindElement: 1}},
		},
	}

	profiles := BuiltInProfiles()
	if len(profiles) != 3 {
		t.Fatalf("profiles = %d, want 3", len(profiles))
	}
	docx := FilterProfile(reg, profiles[0])
	xlsx := FilterProfile(reg, profiles[1])
	pptx := FilterProfile(reg, profiles[2])
	if len(docx.Declarations) != 1 || docx.Declarations[0].Name != "document" {
		t.Fatalf("docx declarations = %#v", docx.Declarations)
	}
	if len(xlsx.Declarations) != 1 || xlsx.Declarations[0].Name != "workbook" {
		t.Fatalf("xlsx declarations = %#v", xlsx.Declarations)
	}
	if len(pptx.Declarations) != 1 || pptx.Declarations[0].Name != "presentation" {
		t.Fatalf("pptx declarations = %#v", pptx.Declarations)
	}
}

func writeSchemaZip(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Create(%s): %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%s): %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip: %v", err)
	}
	return path
}
