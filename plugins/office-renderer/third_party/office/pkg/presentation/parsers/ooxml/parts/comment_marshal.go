package parts

import (
	"encoding/xml"
	"strconv"
)

// MarshalXML emits PptxComment as p:cm.
func (c PptxComment) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	pNS := "http://schemas.openxmlformats.org/presentationml/2006/main"
	start.Name = xml.Name{Space: pNS, Local: "cm"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "authorId"}, Value: strconv.Itoa(c.AuthorID)},
		{Name: xml.Name{Local: "idx"}, Value: strconv.Itoa(c.Index)},
	}
	if c.DateTime != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "dt"}, Value: c.DateTime})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if c.X != 0 || c.Y != 0 {
		pos := xmlCmPos{X: c.X, Y: c.Y}
		if err := e.EncodeElement(pos, xml.StartElement{Name: xml.Name{Space: pNS, Local: "pos"}}); err != nil {
			return err
		}
	}
	if c.Text != "" {
		if err := e.EncodeElement(c.Text, xml.StartElement{Name: xml.Name{Space: pNS, Local: "text"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}