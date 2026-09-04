package edit

// Relationship describes a single OOXML package relationship.
type Relationship struct {
	SourcePart string // part that owns the relationship, e.g. "word/document.xml"
	ID         string // relationship ID, e.g. "rId1"
	Type       string // relationship type URI
	Target     string // target path or URL
	TargetMode string // "External" or empty for internal
}

// Relationships returns the relationships that would be written when the
// document is serialized to OOXML. It performs a full package conversion
// internally, so it may be expensive for large documents. Returns nil and an
// error if conversion fails.
func (d *Document) Relationships() ([]Relationship, error) {
	if d == nil || d.IR == nil {
		return nil, nil
	}
	pkg, err := d.ToOOXML()
	if err != nil {
		return nil, err
	}
	var out []Relationship
	for partName, rels := range pkg.PartRels {
		if rels == nil {
			continue
		}
		for _, r := range rels.Relationships {
			out = append(out, Relationship{
				SourcePart: partName,
				ID:         r.ID,
				Type:       r.Type,
				Target:     r.Target,
				TargetMode: r.TargetMode,
			})
		}
	}
	return out, nil
}

// RelationshipsByType returns relationships filtered by type URI.
// Convenience helper for inspecting common relationships such as images,
// hyperlinks, headers, or footers.
func (d *Document) RelationshipsByType(typ string) ([]Relationship, error) {
	all, err := d.Relationships()
	if err != nil {
		return nil, err
	}
	var out []Relationship
	for _, r := range all {
		if r.Type == typ {
			out = append(out, r)
		}
	}
	return out, nil
}

// AddRelationship registers an extra relationship to be merged into the
// generated OOXML package on the next save. If SourcePart is empty it defaults
// to the main document part. Duplicate IDs within the same source part are
// ignored.
func (d *Document) AddRelationship(r Relationship) {
	if d == nil {
		return
	}
	// Avoid adding duplicates.
	for _, existing := range d.extraRels {
		if existing.SourcePart == r.SourcePart && existing.ID == r.ID {
			return
		}
	}
	d.extraRels = append(d.extraRels, r)
}

// RemoveRelationship removes a user-added relationship from the extra set.
// It does not affect relationships that originated from the loaded document.
func (d *Document) RemoveRelationship(sourcePart, id string) {
	if d == nil {
		return
	}
	filtered := d.extraRels[:0]
	for _, r := range d.extraRels {
		if r.SourcePart == sourcePart && r.ID == id {
			continue
		}
		filtered = append(filtered, r)
	}
	d.extraRels = filtered
}