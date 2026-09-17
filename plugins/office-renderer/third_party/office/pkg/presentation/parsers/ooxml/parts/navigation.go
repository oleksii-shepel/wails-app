package parts

import (
	"encoding/xml"
	"fmt"

	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

// packageNavigator is the single OPC navigation surface used by PresentationML
// semantic loaders. It owns relationship lookup, target resolution and typed
// XML decoding; semantic loaders only decide which relationship types matter.
type packageNavigator struct{ pkg *opc.Package }

func newPackageNavigator(pkg *opc.Package) packageNavigator { return packageNavigator{pkg: pkg} }

func (n packageNavigator) relationships(source string) (*rels.Document, bool, error) {
	if n.pkg == nil {
		return nil, false, fmt.Errorf("presentation parts: package cannot be nil")
	}
	return n.pkg.Relationships(source)
}

func (n packageNavigator) relationship(source, relType string) (rels.Relationship, bool, error) {
	doc, ok, err := n.relationships(source)
	if err != nil || !ok {
		return rels.Relationship{}, false, err
	}
	for _, r := range doc.Relationships {
		if relationshipTypeIs(r.Type, relType) && !r.IsExternal() {
			return r, true, nil
		}
	}
	return rels.Relationship{}, false, nil
}

func (n packageNavigator) target(source string, r rels.Relationship) (string, bool) {
	if r.IsExternal() {
		return "", false
	}
	return rels.ResolveTarget(source, r.Target), true
}

func (n packageNavigator) part(partName string) (opc.Part, bool) {
	if n.pkg == nil {
		return opc.Part{}, false
	}
	return n.pkg.Part(partName)
}

func (n packageNavigator) relatedTarget(source, relType string) (string, bool, error) {
	r, ok, err := n.relationship(source, relType)
	if err != nil || !ok {
		return "", false, err
	}
	target, ok := n.target(source, r)
	return target, ok, nil
}

func (n packageNavigator) binaryPart(source string, r rels.Relationship) (media.Part, bool) {
	partName, ok := n.target(source, r)
	if !ok {
		return media.Part{}, false
	}
	p, ok := n.part(partName)
	if !ok {
		return media.Part{}, false
	}
	return media.Part{Name: partName, ContentType: p.ContentType, Data: p.Data}, true
}

func (n packageNavigator) decodeXML(partName string, dst any) (bool, error) {
	if n.pkg == nil {
		return false, fmt.Errorf("presentation parts: package cannot be nil")
	}
	p, ok := n.part(partName)
	if !ok {
		return false, nil
	}
	if err := xml.Unmarshal(p.Data, dst); err != nil {
		return true, err
	}
	return true, nil
}

func (n packageNavigator) decodeRelatedXML(source, relType string, dst any) (string, bool, error) {
	r, ok, err := n.relationship(source, relType)
	if err != nil || !ok {
		return "", false, err
	}
	partName, ok := n.target(source, r)
	if !ok {
		return "", false, nil
	}
	found, err := n.decodeXML(partName, dst)
	return partName, found, err
}
