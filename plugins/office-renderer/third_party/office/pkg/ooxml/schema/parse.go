package schema

import "encoding/xml"

const xsdNamespace = "http://www.w3.org/2001/XMLSchema"

type xsdNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Nodes   []xsdNode  `xml:",any"`
}

func parseSchema(schemaName string, data []byte) ([]Decl, string, error) {
	var doc xsdNode
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, "", err
	}
	targetNamespace := attrValue(doc.Attrs, "targetNamespace")
	decls := make([]Decl, 0)
	for _, node := range doc.Nodes {
		if node.XMLName.Space != xsdNamespace {
			continue
		}
		kind, ok := localDeclKind(node.XMLName.Local)
		if !ok {
			continue
		}
		decl := Decl{
			Kind:     kind,
			Name:     attrValue(node.Attrs, "name"),
			Type:     attrValue(node.Attrs, "type"),
			Ref:      attrValue(node.Attrs, "ref"),
			Abstract: attrValue(node.Attrs, "abstract") == "true",
		}
		decl.Base = findBase(node)
		if decl.Name == "" && decl.Ref == "" {
			continue
		}
		decls = append(decls, decl)
	}
	return decls, targetNamespace, nil
}

func localDeclKind(local string) (DeclKind, bool) {
	switch local {
	case "element":
		return DeclKindElement, true
	case "complexType":
		return DeclKindComplexType, true
	case "simpleType":
		return DeclKindSimpleType, true
	case "group":
		return DeclKindGroup, true
	case "attribute":
		return DeclKindAttribute, true
	case "attributeGroup":
		return DeclKindAttributeGroup, true
	default:
		return "", false
	}
}

func attrValue(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func findBase(node xsdNode) string {
	queue := append([]xsdNode(nil), node.Nodes...)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.XMLName.Space == xsdNamespace {
			switch cur.XMLName.Local {
			case "extension", "restriction":
				if base := attrValue(cur.Attrs, "base"); base != "" {
					return base
				}
			}
		}
		queue = append(queue, cur.Nodes...)
	}
	return ""
}
