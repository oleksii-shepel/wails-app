package schema

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DeclKind identifies the kind of XSD global declaration.
type DeclKind string

const (
	DeclKindElement        DeclKind = "element"
	DeclKindComplexType    DeclKind = "complexType"
	DeclKindSimpleType     DeclKind = "simpleType"
	DeclKindGroup          DeclKind = "group"
	DeclKindAttribute      DeclKind = "attribute"
	DeclKindAttributeGroup DeclKind = "attributeGroup"
)

// Decl describes one global XSD declaration.
type Decl struct {
	Bundle    string   `json:"bundle"`
	Schema    string   `json:"schema"`
	Namespace string   `json:"namespace"`
	Kind      DeclKind `json:"kind"`
	Name      string   `json:"name"`
	Type      string   `json:"type,omitempty"`
	Ref       string   `json:"ref,omitempty"`
	Base      string   `json:"base,omitempty"`
	Abstract  bool     `json:"abstract,omitempty"`
}

// SchemaSummary aggregates declarations for one XSD file.
type SchemaSummary struct {
	Bundle string           `json:"bundle"`
	Schema string           `json:"schema"`
	NS     string           `json:"namespace"`
	Counts map[DeclKind]int `json:"counts"`
}

// NamespaceSummary aggregates declarations for one namespace.
type NamespaceSummary struct {
	Namespace string           `json:"namespace"`
	Counts    map[DeclKind]int `json:"counts"`
}

// Registry is a schema-derived registry of OOXML global declarations.
type Registry struct {
	Declarations []Decl             `json:"declarations"`
	Schemas      []SchemaSummary    `json:"schemas"`
	Namespaces   []NamespaceSummary `json:"namespaces"`
}

type schemaDoc struct {
	TargetNamespace string `xml:"targetNamespace,attr"`
}

// BuildDir loads every schema zip under specsDir into a global declaration registry.
func BuildDir(specsDir string) (*Registry, error) {
	var bundlePaths []string
	err := filepath.WalkDir(specsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".zip") {
			return nil
		}
		bundlePaths = append(bundlePaths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(bundlePaths)
	return BuildBundles(bundlePaths)
}

// BuildBundles loads all XSD files found in the supplied bundle paths.
func BuildBundles(bundlePaths []string) (*Registry, error) {
	reg := &Registry{}
	schemaIndex := map[string]*SchemaSummary{}
	namespaceIndex := map[string]*NamespaceSummary{}

	for _, bundlePath := range bundlePaths {
		if err := loadBundle(reg, bundlePath, schemaIndex, namespaceIndex); err != nil {
			return nil, err
		}
	}

	for _, schema := range schemaIndex {
		reg.Schemas = append(reg.Schemas, *schema)
	}
	for _, ns := range namespaceIndex {
		reg.Namespaces = append(reg.Namespaces, *ns)
	}

	sort.Slice(reg.Declarations, func(i, j int) bool {
		a, b := reg.Declarations[i], reg.Declarations[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Bundle != b.Bundle {
			return a.Bundle < b.Bundle
		}
		return a.Schema < b.Schema
	})
	sort.Slice(reg.Schemas, func(i, j int) bool {
		if reg.Schemas[i].Bundle != reg.Schemas[j].Bundle {
			return reg.Schemas[i].Bundle < reg.Schemas[j].Bundle
		}
		return reg.Schemas[i].Schema < reg.Schemas[j].Schema
	})
	sort.Slice(reg.Namespaces, func(i, j int) bool {
		return reg.Namespaces[i].Namespace < reg.Namespaces[j].Namespace
	})

	return reg, nil
}

func loadBundle(reg *Registry, bundlePath string, schemaIndex map[string]*SchemaSummary, namespaceIndex map[string]*NamespaceSummary) error {
	reader, err := zip.OpenReader(bundlePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	bundleName := filepath.Base(bundlePath)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(file.Name), ".xsd") {
			continue
		}
		data, err := readZipFile(file)
		if err != nil {
			return err
		}
		schemaDecls, targetNamespace, err := parseSchema(file.Name, data)
		if err != nil {
			return fmt.Errorf("%s:%s: %w", bundleName, file.Name, err)
		}
		schemaKey := bundleName + "::" + file.Name
		schemaSummary := schemaIndex[schemaKey]
		if schemaSummary == nil {
			schemaSummary = &SchemaSummary{
				Bundle: bundleName,
				Schema: file.Name,
				NS:     targetNamespace,
				Counts: make(map[DeclKind]int),
			}
			schemaIndex[schemaKey] = schemaSummary
		}
		namespaceSummary := namespaceIndex[targetNamespace]
		if namespaceSummary == nil {
			namespaceSummary = &NamespaceSummary{
				Namespace: targetNamespace,
				Counts:    make(map[DeclKind]int),
			}
			namespaceIndex[targetNamespace] = namespaceSummary
		}
		for _, decl := range schemaDecls {
			decl.Bundle = bundleName
			decl.Schema = file.Name
			decl.Namespace = targetNamespace
			reg.Declarations = append(reg.Declarations, decl)
			schemaSummary.Counts[decl.Kind]++
			namespaceSummary.Counts[decl.Kind]++
		}
	}

	return nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// WriteGo writes the generated registry into a Go source file.
// WriteJSON writes the registry as pretty-printed JSON.
func WriteJSON(reg *Registry, path string) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// WriteMarkdown writes a human-readable summary report.
func WriteMarkdown(reg *Registry, path string) error {
	return writeMarkdown(reg, path, "# OOXML Schema Registry", "This report is generated from the local OOXML XSD bundles in `specs/ooxml/`.\nIt inventories global schema declarations across OOXML packages, which is structural coverage rather than parser/render fidelity.")
}

func writeMarkdown(reg *Registry, path, title, intro string) error {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(intro)
	b.WriteString("\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Kind | Count |\n")
	b.WriteString("| --- | ---: |\n")
	for _, kind := range []DeclKind{DeclKindElement, DeclKindComplexType, DeclKindSimpleType, DeclKindGroup, DeclKindAttribute, DeclKindAttributeGroup} {
		fmt.Fprintf(&b, "| `%s` | %d |\n", kind, countKind(reg.Declarations, kind))
	}
	fmt.Fprintf(&b, "| Namespaces | %d |\n", len(reg.Namespaces))
	fmt.Fprintf(&b, "| Schemas | %d |\n", len(reg.Schemas))
	fmt.Fprintf(&b, "| Total declarations | %d |\n", len(reg.Declarations))

	b.WriteString("\n## Namespaces\n\n")
	b.WriteString("| Namespace | Elements | Complex Types | Simple Types | Groups | Attributes | Attribute Groups |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, ns := range reg.Namespaces {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d | %d | %d |\n",
			ns.Namespace,
			ns.Counts[DeclKindElement],
			ns.Counts[DeclKindComplexType],
			ns.Counts[DeclKindSimpleType],
			ns.Counts[DeclKindGroup],
			ns.Counts[DeclKindAttribute],
			ns.Counts[DeclKindAttributeGroup],
		)
	}

	b.WriteString("\n## Largest Schemas\n\n")
	b.WriteString("| Schema | Namespace | Total |\n")
	b.WriteString("| --- | --- | ---: |\n")
	schemas := append([]SchemaSummary(nil), reg.Schemas...)
	sort.Slice(schemas, func(i, j int) bool {
		ti := totalCounts(schemas[i].Counts)
		tj := totalCounts(schemas[j].Counts)
		if ti != tj {
			return ti > tj
		}
		if schemas[i].Bundle != schemas[j].Bundle {
			return schemas[i].Bundle < schemas[j].Bundle
		}
		return schemas[i].Schema < schemas[j].Schema
	})
	limit := 20
	if len(schemas) < limit {
		limit = len(schemas)
	}
	for _, schema := range schemas[:limit] {
		fmt.Fprintf(&b, "| `%s/%s` | `%s` | %d |\n", schema.Bundle, schema.Schema, schema.NS, totalCounts(schema.Counts))
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func countKind(decls []Decl, kind DeclKind) int {
	total := 0
	for _, decl := range decls {
		if decl.Kind == kind {
			total++
		}
	}
	return total
}

func totalCounts(counts map[DeclKind]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}
