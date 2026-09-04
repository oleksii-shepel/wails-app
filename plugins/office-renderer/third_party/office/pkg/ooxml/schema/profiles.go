package schema

import "sort"

// Profile identifies an OOXML family-specific schema surface.
type Profile struct {
	Key        string
	Label      string
	Namespaces map[string]struct{}
}

var commonNamespaces = []string{
	"http://schemas.openxmlformats.org/package/2006/content-types",
	"http://schemas.openxmlformats.org/package/2006/relationships",
	"http://schemas.openxmlformats.org/package/2006/metadata/core-properties",
	"http://schemas.openxmlformats.org/package/2006/digital-signature",
	"http://schemas.openxmlformats.org/officeDocument/2006/bibliography",
	"http://schemas.openxmlformats.org/officeDocument/2006/characteristics",
	"http://schemas.openxmlformats.org/officeDocument/2006/custom-properties",
	"http://schemas.openxmlformats.org/officeDocument/2006/customXml",
	"http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes",
	"http://schemas.openxmlformats.org/officeDocument/2006/extended-properties",
	"http://schemas.openxmlformats.org/officeDocument/2006/math",
	"http://schemas.openxmlformats.org/officeDocument/2006/relationships",
	"http://schemas.openxmlformats.org/officeDocument/2006/sharedTypes",
	"http://schemas.openxmlformats.org/schemaLibrary/2006/main",
	"http://purl.oclc.org/ooxml/officeDocument/bibliography",
	"http://purl.oclc.org/ooxml/officeDocument/characteristics",
	"http://purl.oclc.org/ooxml/officeDocument/customProperties",
	"http://purl.oclc.org/ooxml/officeDocument/customXml",
	"http://purl.oclc.org/ooxml/officeDocument/docPropsVTypes",
	"http://purl.oclc.org/ooxml/officeDocument/extendedProperties",
	"http://purl.oclc.org/ooxml/officeDocument/math",
	"http://purl.oclc.org/ooxml/officeDocument/relationships",
	"http://purl.oclc.org/ooxml/officeDocument/sharedTypes",
	"http://purl.oclc.org/ooxml/schemaLibrary/main",
}

var sharedDrawingNamespaces = []string{
	"http://schemas.openxmlformats.org/drawingml/2006/chart",
	"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing",
	"http://schemas.openxmlformats.org/drawingml/2006/diagram",
	"http://schemas.openxmlformats.org/drawingml/2006/lockedCanvas",
	"http://schemas.openxmlformats.org/drawingml/2006/main",
	"http://schemas.openxmlformats.org/drawingml/2006/picture",
	"http://purl.oclc.org/ooxml/drawingml/chart",
	"http://purl.oclc.org/ooxml/drawingml/chartDrawing",
	"http://purl.oclc.org/ooxml/drawingml/diagram",
	"http://purl.oclc.org/ooxml/drawingml/lockedCanvas",
	"http://purl.oclc.org/ooxml/drawingml/main",
	"http://purl.oclc.org/ooxml/drawingml/picture",
}

// BuiltInProfiles returns the family-specific OOXML schema surfaces this repo targets.
func BuiltInProfiles() []Profile {
	return []Profile{
		{
			Key:   "docx",
			Label: "DOCX Schema Registry",
			Namespaces: namespaceSet(commonNamespaces, sharedDrawingNamespaces, []string{
				"http://schemas.openxmlformats.org/wordprocessingml/2006/main",
				"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing",
				"http://purl.oclc.org/ooxml/wordprocessingml/main",
				"http://purl.oclc.org/ooxml/drawingml/wordprocessingDrawing",
				"urn:schemas-microsoft-com:vml",
				"urn:schemas-microsoft-com:office:office",
				"urn:schemas-microsoft-com:office:word",
			}),
		},
		{
			Key:   "xlsx",
			Label: "XLSX Schema Registry",
			Namespaces: namespaceSet(commonNamespaces, sharedDrawingNamespaces, []string{
				"http://schemas.openxmlformats.org/spreadsheetml/2006/main",
				"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing",
				"http://purl.oclc.org/ooxml/spreadsheetml/main",
				"http://purl.oclc.org/ooxml/drawingml/spreadsheetDrawing",
				"urn:schemas-microsoft-com:vml",
				"urn:schemas-microsoft-com:office:office",
				"urn:schemas-microsoft-com:office:excel",
			}),
		},
		{
			Key:   "pptx",
			Label: "PPTX Schema Registry",
			Namespaces: namespaceSet(commonNamespaces, sharedDrawingNamespaces, []string{
				"http://schemas.openxmlformats.org/presentationml/2006/main",
				"http://purl.oclc.org/ooxml/presentationml/main",
				"urn:schemas-microsoft-com:vml",
				"urn:schemas-microsoft-com:office:office",
				"urn:schemas-microsoft-com:office:powerpoint",
			}),
		},
	}
}

// FilterProfile returns a registry limited to the namespaces relevant to a built-in profile.
func FilterProfile(reg *Registry, profile Profile) *Registry {
	if reg == nil {
		return nil
	}
	filtered := &Registry{}
	for _, decl := range reg.Declarations {
		if _, ok := profile.Namespaces[decl.Namespace]; ok {
			filtered.Declarations = append(filtered.Declarations, decl)
		}
	}
	for _, schema := range reg.Schemas {
		if _, ok := profile.Namespaces[schema.NS]; ok {
			filtered.Schemas = append(filtered.Schemas, schema)
		}
	}
	for _, ns := range reg.Namespaces {
		if _, ok := profile.Namespaces[ns.Namespace]; ok {
			filtered.Namespaces = append(filtered.Namespaces, ns)
		}
	}
	return filtered
}

// ProfileKeys returns the built-in profile keys in a stable order.
func ProfileKeys() []string {
	profiles := BuiltInProfiles()
	keys := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		keys = append(keys, profile.Key)
	}
	sort.Strings(keys)
	return keys
}

// WriteProfileMarkdown writes a family-specific report.
func WriteProfileMarkdown(reg *Registry, profile Profile, path string) error {
	return writeMarkdown(reg, path, "# "+profile.Label, "This report is generated from the local OOXML XSD bundles in `specs/ooxml/` and filtered to the namespaces relevant to `"+profile.Key+"`.\nIt inventories structural schema declarations that future parser/render work can target for full coverage.")
}

func namespaceSet(groups ...[]string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, group := range groups {
		for _, ns := range group {
			set[ns] = struct{}{}
		}
	}
	return set
}
