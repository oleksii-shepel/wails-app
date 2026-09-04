package edit

import (
	"sort"

	wordir "office/pkg/word/ir"
)

// FontTable provides access to the document's font table (word/fontTable.xml).
type FontTable struct {
	doc *Document
}

// FontTable returns an editor for the document's font table.
func (d *Document) FontTable() *FontTable {
	return &FontTable{doc: d}
}

// Names returns all font family names in the font table, sorted alphabetically.
func (ft *FontTable) Names() []string {
	if ft == nil || ft.doc == nil || ft.doc.IR == nil || len(ft.doc.IR.FontFamilies) == 0 {
		return nil
	}
	names := make([]string, 0, len(ft.doc.IR.FontFamilies))
	for name := range ft.doc.IR.FontFamilies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Get returns the font family info for the given name (case-sensitive).
// If the font is not present, the zero value is returned and ok is false.
func (ft *FontTable) Get(name string) (wordir.FontFamilyInfo, bool) {
	if ft == nil || ft.doc == nil || ft.doc.IR == nil {
		return wordir.FontFamilyInfo{}, false
	}
	info, ok := ft.doc.IR.FontFamilies[name]
	return info, ok
}

// Add adds or replaces a font family entry in the font table.
func (ft *FontTable) Add(name string, info wordir.FontFamilyInfo) {
	if ft == nil || ft.doc == nil || ft.doc.IR == nil || name == "" {
		return
	}
	if ft.doc.IR.FontFamilies == nil {
		ft.doc.IR.FontFamilies = make(map[string]wordir.FontFamilyInfo)
	}
	ft.doc.IR.FontFamilies[name] = info
}

// Remove deletes a font family entry from the font table.
func (ft *FontTable) Remove(name string) {
	if ft == nil || ft.doc == nil || ft.doc.IR == nil || ft.doc.IR.FontFamilies == nil {
		return
	}
	delete(ft.doc.IR.FontFamilies, name)
}

// Count returns the number of font families in the table.
func (ft *FontTable) Count() int {
	if ft == nil || ft.doc == nil || ft.doc.IR == nil {
		return 0
	}
	return len(ft.doc.IR.FontFamilies)
}

// Clear removes all entries from the font table.
func (ft *FontTable) Clear() {
	if ft == nil || ft.doc == nil || ft.doc.IR == nil {
		return
	}
	ft.doc.IR.FontFamilies = nil
}