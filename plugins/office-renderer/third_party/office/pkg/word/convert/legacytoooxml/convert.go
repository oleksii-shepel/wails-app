package legacytoooxml

import (
	"fmt"
	"strings"

	wordir "office/pkg/word/ir"
	wordparts "office/pkg/word/parsers/ooxml/parts"
)

// Convert transforms a legacy Word IR document into an OOXML parts.Package
// that can be serialized to a .docx file via wordparts.WritePackage.
func Convert(doc *wordir.Document) (*wordparts.Package, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil document")
	}

	c := &converter{
		pkg: &wordparts.Package{
			MainPartName: "word/document.xml",
			Document: &wordparts.Document{
				Body: &wordparts.Body{},
			},
			Headers: make(map[string]*wordparts.Header),
			Footers: make(map[string]*wordparts.Footer),
		},
	}
	if len(doc.CustomXMLParts) > 0 {
		c.pkg.CustomXMLParts = make(map[string][]byte, len(doc.CustomXMLParts))
		for key, data := range doc.CustomXMLParts {
			key = strings.ToUpper(strings.TrimSpace(key))
			if key == "" {
				continue
			}
			c.pkg.CustomXMLParts[key] = append([]byte(nil), data...)
		}
	}

	c.mapCoreProperties(doc)
	c.mapStyles(doc)
	c.mapNumbering(doc)

	c.convertBody(doc)

	c.mapFootnotes(doc)
	c.mapEndnotes(doc)
	c.mapComments(doc)
	c.mapSettings(doc)
	c.mapFontTable(doc)
	c.mapTheme(doc)

	return c.pkg, nil
}

type converter struct {
	pkg            *wordparts.Package
	nextRelID      int
	nextBookmarkID int
}

func (c *converter) nextRID() string {
	c.nextRelID++
	return fmt.Sprintf("rId%d", c.nextRelID)
}

func (c *converter) bookmarkID() int {
	c.nextBookmarkID++
	return c.nextBookmarkID
}

func (c *converter) hyperlinkRelID(url string) string {
	for rid, u := range c.pkg.HyperlinkTargetsFor(c.pkg.MainPartName) {
		if u == url {
			return rid
		}
	}
	rid := c.nextRID()
	c.pkg.SetHyperlinkTarget(c.pkg.MainPartName, rid, url)
	return rid
}

func (c *converter) reserveRelID(rid string) {
	var n int
	if _, err := fmt.Sscanf(rid, "rId%d", &n); err == nil {
		if n >= c.nextRelID {
			c.nextRelID = n + 1
		}
	}
}

func stringVal(s string) *wordparts.StringVal {
	if s == "" {
		return nil
	}
	return &wordparts.StringVal{Val: s}
}

func intVal(v int) *wordparts.IntVal {
	return &wordparts.IntVal{Val: v}
}

func onOffPtr(b *bool) *wordparts.OnOff {
	if b == nil || !*b {
		return nil
	}
	bv := wordparts.BoolVal(true)
	return &wordparts.OnOff{Val: &bv}
}

func onOffBool(b bool) *wordparts.OnOff {
	if !b {
		return nil
	}
	bv := wordparts.BoolVal(true)
	return &wordparts.OnOff{Val: &bv}
}

func ptrInt(v int) *int {
	return &v
}