package edit

import (
	"sort"

	wordir "office/pkg/word/ir"
)

// LatentStyles provides access to the document's latent styles configuration.
type LatentStyles struct {
	doc *Document
}

// LatentStyles returns an editor for the document's latent styles.
func (d *Document) LatentStyles() *LatentStyles {
	return &LatentStyles{doc: d}
}

// Names returns all latent style exception names, sorted alphabetically.
func (ls *LatentStyles) Names() []string {
	if ls == nil || ls.doc == nil || ls.doc.IR == nil || ls.doc.IR.LatentStyles == nil {
		return nil
	}
	names := make([]string, 0, len(ls.doc.IR.LatentStyles.Exceptions))
	for _, exc := range ls.doc.IR.LatentStyles.Exceptions {
		names = append(names, exc.Name)
	}
	sort.Strings(names)
	return names
}

// Get returns the latent style exception for the given name.
// If not present, the zero value is returned and ok is false.
func (ls *LatentStyles) Get(name string) (wordir.LatentStyle, bool) {
	if ls == nil || ls.doc == nil || ls.doc.IR == nil || ls.doc.IR.LatentStyles == nil {
		return wordir.LatentStyle{}, false
	}
	for _, exc := range ls.doc.IR.LatentStyles.Exceptions {
		if exc.Name == name {
			return exc, true
		}
	}
	return wordir.LatentStyle{}, false
}

// Add adds or replaces a latent style exception.
func (ls *LatentStyles) Add(exc wordir.LatentStyle) {
	if ls == nil || ls.doc == nil || ls.doc.IR == nil || exc.Name == "" {
		return
	}
	if ls.doc.IR.LatentStyles == nil {
		ls.doc.IR.LatentStyles = &wordir.LatentStyles{}
	}
	for i := range ls.doc.IR.LatentStyles.Exceptions {
		if ls.doc.IR.LatentStyles.Exceptions[i].Name == exc.Name {
			ls.doc.IR.LatentStyles.Exceptions[i] = exc
			return
		}
	}
	ls.doc.IR.LatentStyles.Exceptions = append(ls.doc.IR.LatentStyles.Exceptions, exc)
}

// Remove deletes a latent style exception by name.
func (ls *LatentStyles) Remove(name string) {
	if ls == nil || ls.doc == nil || ls.doc.IR == nil || ls.doc.IR.LatentStyles == nil {
		return
	}
	filtered := ls.doc.IR.LatentStyles.Exceptions[:0]
	for _, exc := range ls.doc.IR.LatentStyles.Exceptions {
		if exc.Name != name {
			filtered = append(filtered, exc)
		}
	}
	ls.doc.IR.LatentStyles.Exceptions = filtered
}

// Count returns the number of latent style exceptions.
func (ls *LatentStyles) Count() int {
	if ls == nil || ls.doc == nil || ls.doc.IR == nil || ls.doc.IR.LatentStyles == nil {
		return 0
	}
	return len(ls.doc.IR.LatentStyles.Exceptions)
}

// Clear removes all latent style exceptions.
func (ls *LatentStyles) Clear() {
	if ls == nil || ls.doc == nil || ls.doc.IR == nil {
		return
	}
	ls.doc.IR.LatentStyles = nil
}