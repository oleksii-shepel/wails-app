package opc

import "encoding/xml"

// ContentTypes models [Content_Types].xml.
type ContentTypes struct {
	XMLName   xml.Name       `xml:"Types"`
	Defaults  []DefaultType  `xml:"Default"`
	Overrides []OverrideType `xml:"Override"`
}

// DefaultType maps a file extension to a content type.
type DefaultType struct {
	Extension   string `xml:"Extension,attr"`
	ContentType string `xml:"ContentType,attr"`
}

// OverrideType maps an explicit part name to a content type.
type OverrideType struct {
	PartName    string `xml:"PartName,attr"`
	ContentType string `xml:"ContentType,attr"`
}
