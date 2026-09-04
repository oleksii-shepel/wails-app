package specmap

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const xsdNamespace = "http://www.w3.org/2001/XMLSchema"

// Inventory is a schema-derived map of OOXML element declarations/usages.
type Inventory struct {
	Bundles  []BundleSummary     `json:"bundles"`
	Schemas  []SchemaSummary     `json:"schemas"`
	Summary  InventorySummary    `json:"summary"`
	Elements []ElementOccurrence `json:"elements"`
}

// BundleSummary describes one parsed spec bundle.
type BundleSummary struct {
	Name         string `json:"name"`
	SchemaCount  int    `json:"schema_count"`
	ElementCount int    `json:"element_count"`
}

// SchemaSummary describes one parsed XSD file.
type SchemaSummary struct {
	Bundle             string `json:"bundle"`
	Schema             string `json:"schema"`
	Namespace          string `json:"namespace"`
	ElementCount       int    `json:"element_count"`
	GlobalElementCount int    `json:"global_element_count"`
}

// InventorySummary aggregates bundle/schema/namespace counts.
type InventorySummary struct {
	BundleCount  int                `json:"bundle_count"`
	SchemaCount  int                `json:"schema_count"`
	ElementCount int                `json:"element_count"`
	Namespaces   []NamespaceSummary `json:"namespaces"`
}

// NamespaceSummary aggregates counts per target namespace.
type NamespaceSummary struct {
	Namespace          string `json:"namespace"`
	SchemaCount        int    `json:"schema_count"`
	ElementCount       int    `json:"element_count"`
	GlobalElementCount int    `json:"global_element_count"`
}

// ElementOccurrence records one xs:element occurrence in the source XSDs.
type ElementOccurrence struct {
	Bundle            string   `json:"bundle"`
	Schema            string   `json:"schema"`
	Namespace         string   `json:"namespace"`
	XSDPath           string   `json:"xsd_path"`
	Context           []string `json:"context,omitempty"`
	Parent            string   `json:"parent,omitempty"`
	Name              string   `json:"name,omitempty"`
	Ref               string   `json:"ref,omitempty"`
	Type              string   `json:"type,omitempty"`
	MinOccurs         string   `json:"min_occurs,omitempty"`
	MaxOccurs         string   `json:"max_occurs,omitempty"`
	SubstitutionGroup string   `json:"substitution_group,omitempty"`
	Global            bool     `json:"global"`
}

type schemaFrame struct {
	Kind  string
	Name  string
	Depth int
}

// LoadDir parses every zip bundle under specsDir and inventories all .xsd entries.
func LoadDir(specsDir string) (*Inventory, error) {
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
	return LoadBundles(bundlePaths)
}

// LoadBundles parses all .xsd files found in the supplied zip bundles.
func LoadBundles(bundlePaths []string) (*Inventory, error) {
	inv := &Inventory{}

	bundleCounts := map[string]int{}
	bundleSchemaCounts := map[string]int{}
	schemaIndex := map[string]*SchemaSummary{}
	namespaceIndex := map[string]*NamespaceSummary{}

	for _, bundlePath := range bundlePaths {
		bundleName := filepath.Base(bundlePath)
		if err := loadBundle(inv, bundlePath, bundleName, bundleCounts, bundleSchemaCounts, schemaIndex, namespaceIndex); err != nil {
			return nil, err
		}
	}

	for bundleName, elementCount := range bundleCounts {
		inv.Bundles = append(inv.Bundles, BundleSummary{
			Name:         bundleName,
			SchemaCount:  bundleSchemaCounts[bundleName],
			ElementCount: elementCount,
		})
	}

	for _, summary := range schemaIndex {
		inv.Schemas = append(inv.Schemas, *summary)
	}
	for _, summary := range namespaceIndex {
		inv.Summary.Namespaces = append(inv.Summary.Namespaces, *summary)
	}

	sort.Slice(inv.Bundles, func(i, j int) bool {
		return inv.Bundles[i].Name < inv.Bundles[j].Name
	})
	sort.Slice(inv.Schemas, func(i, j int) bool {
		if inv.Schemas[i].Bundle != inv.Schemas[j].Bundle {
			return inv.Schemas[i].Bundle < inv.Schemas[j].Bundle
		}
		return inv.Schemas[i].Schema < inv.Schemas[j].Schema
	})
	sort.Slice(inv.Summary.Namespaces, func(i, j int) bool {
		return inv.Summary.Namespaces[i].Namespace < inv.Summary.Namespaces[j].Namespace
	})
	sort.Slice(inv.Elements, func(i, j int) bool {
		a, b := inv.Elements[i], inv.Elements[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Schema != b.Schema {
			return a.Schema < b.Schema
		}
		if a.XSDPath != b.XSDPath {
			return a.XSDPath < b.XSDPath
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Ref < b.Ref
	})

	inv.Summary.BundleCount = len(inv.Bundles)
	inv.Summary.SchemaCount = len(inv.Schemas)
	inv.Summary.ElementCount = len(inv.Elements)
	return inv, nil
}

func loadBundle(inv *Inventory, bundlePath, bundleName string, bundleCounts map[string]int, bundleSchemaCounts map[string]int, schemaIndex map[string]*SchemaSummary, namespaceIndex map[string]*NamespaceSummary) error {
	reader, err := zip.OpenReader(bundlePath)
	if err != nil {
		return fmt.Errorf("open bundle %s: %w", bundleName, err)
	}
	defer reader.Close()

	sort.Slice(reader.File, func(i, j int) bool {
		return reader.File[i].Name < reader.File[j].Name
	})

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(file.Name), ".xsd") {
			continue
		}

		data, err := readZipFile(file)
		if err != nil {
			return fmt.Errorf("read %s in %s: %w", file.Name, bundleName, err)
		}
		targetNamespace, occurrences, err := parseSchema(bundleName, file.Name, data)
		if err != nil {
			return fmt.Errorf("parse %s in %s: %w", file.Name, bundleName, err)
		}

		key := bundleName + "::" + file.Name
		schemaSummary := &SchemaSummary{
			Bundle:    bundleName,
			Schema:    file.Name,
			Namespace: targetNamespace,
		}
		for _, occurrence := range occurrences {
			schemaSummary.ElementCount++
			if occurrence.Global {
				schemaSummary.GlobalElementCount++
			}
		}
		schemaIndex[key] = schemaSummary
		bundleCounts[bundleName] += len(occurrences)
		bundleSchemaCounts[bundleName]++

		nsSummary := namespaceIndex[targetNamespace]
		if nsSummary == nil {
			nsSummary = &NamespaceSummary{Namespace: targetNamespace}
			namespaceIndex[targetNamespace] = nsSummary
		}
		nsSummary.SchemaCount++
		nsSummary.ElementCount += len(occurrences)
		nsSummary.GlobalElementCount += schemaSummary.GlobalElementCount

		inv.Elements = append(inv.Elements, occurrences...)
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

func parseSchema(bundleName, schemaName string, data []byte) (string, []ElementOccurrence, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))

	var (
		targetNamespace string
		path            []string
		frames          []schemaFrame
		occurrences     []ElementOccurrence
	)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}

		switch token := tok.(type) {
		case xml.StartElement:
			if token.Name.Space != xsdNamespace {
				continue
			}

			local := token.Name.Local
			path = append(path, local)
			if local == "schema" {
				targetNamespace = attr(token.Attr, "targetNamespace")
			}

			if local == "element" {
				occurrences = append(occurrences, ElementOccurrence{
					Bundle:            bundleName,
					Schema:            schemaName,
					Namespace:         targetNamespace,
					XSDPath:           strings.Join(path, "/"),
					Context:           contextNames(frames),
					Parent:            parentName(frames),
					Name:              attr(token.Attr, "name"),
					Ref:               attr(token.Attr, "ref"),
					Type:              attr(token.Attr, "type"),
					MinOccurs:         attr(token.Attr, "minOccurs"),
					MaxOccurs:         attr(token.Attr, "maxOccurs"),
					SubstitutionGroup: attr(token.Attr, "substitutionGroup"),
					Global:            len(path) == 2,
				})
			}

			if frame := namedFrame(local, len(path), token.Attr); frame != nil {
				frames = append(frames, *frame)
			}

		case xml.EndElement:
			if token.Name.Space != xsdNamespace {
				continue
			}
			if len(frames) > 0 && frames[len(frames)-1].Depth == len(path) {
				frames = frames[:len(frames)-1]
			}
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
		}
	}

	return targetNamespace, occurrences, nil
}

func namedFrame(local string, depth int, attrs []xml.Attr) *schemaFrame {
	if depth < 2 {
		return nil
	}
	switch local {
	case "element", "complexType", "group", "attributeGroup":
	default:
		return nil
	}

	name := attr(attrs, "name")
	if name == "" {
		name = attr(attrs, "ref")
	}
	if name == "" {
		return nil
	}
	return &schemaFrame{
		Kind:  local,
		Name:  name,
		Depth: depth,
	}
}

func contextNames(frames []schemaFrame) []string {
	if len(frames) == 0 {
		return nil
	}
	out := make([]string, 0, len(frames))
	for _, frame := range frames {
		out = append(out, frame.Kind+":"+frame.Name)
	}
	return out
}

func parentName(frames []schemaFrame) string {
	if len(frames) == 0 {
		return ""
	}
	parent := frames[len(frames)-1]
	return parent.Kind + ":" + parent.Name
}

func attr(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

// WriteJSON writes a deterministic pretty-printed JSON inventory file.
func WriteJSON(inv *Inventory, outPath string) error {
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(outPath, data, 0o644)
}

// WriteMarkdown writes a compact human-readable summary alongside the JSON map.
func WriteMarkdown(inv *Inventory, outPath, jsonFilename string) error {
	var b strings.Builder
	b.WriteString("# OOXML Element Map\n\n")
	b.WriteString("This inventory is generated from the local XSD bundles in `specs/`.\n\n")
	b.WriteString("It records raw `xs:element` occurrences from the ECMA/OPC schema files, including nested/contextual declarations and `ref` usages.\n\n")
	if jsonFilename != "" {
		fmt.Fprintf(&b, "Full machine-readable map: [%s](%s)\n\n", jsonFilename, jsonFilename)
	}

	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Count |\n")
	b.WriteString("|---|---:|\n")
	fmt.Fprintf(&b, "| Bundles | %d |\n", inv.Summary.BundleCount)
	fmt.Fprintf(&b, "| Schemas | %d |\n", inv.Summary.SchemaCount)
	fmt.Fprintf(&b, "| Element occurrences | %d |\n", inv.Summary.ElementCount)

	b.WriteString("\n## Bundles\n\n")
	b.WriteString("| Bundle | Schemas | Element occurrences |\n")
	b.WriteString("|---|---:|---:|\n")
	for _, bundle := range inv.Bundles {
		fmt.Fprintf(&b, "| `%s` | %d | %d |\n", bundle.Name, bundle.SchemaCount, bundle.ElementCount)
	}

	b.WriteString("\n## Namespaces\n\n")
	b.WriteString("| Namespace | Schemas | Element occurrences | Global elements |\n")
	b.WriteString("|---|---:|---:|---:|\n")
	for _, summary := range inv.Summary.Namespaces {
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d |\n",
			summary.Namespace, summary.SchemaCount, summary.ElementCount, summary.GlobalElementCount)
	}

	b.WriteString("\n## Schemas\n\n")
	b.WriteString("| Bundle | Schema | Namespace | Element occurrences | Global elements |\n")
	b.WriteString("|---|---|---|---:|---:|\n")
	for _, schema := range inv.Schemas {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %d | %d |\n",
			schema.Bundle, schema.Schema, schema.Namespace, schema.ElementCount, schema.GlobalElementCount)
	}

	b.WriteString("\n## Notes\n\n")
	b.WriteString("- This is a spec-derived inventory, not a parser-coverage report.\n")
	b.WriteString("- Repeated element names may appear many times because the XSD records contextual child declarations/usages.\n")
	b.WriteString("- Transitional/Strict bundles are listed separately; overlapping elements are expected.\n")

	return os.WriteFile(outPath, []byte(b.String()), 0o644)
}
