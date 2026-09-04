package parts

import (
	"encoding/xml"
	"strings"
)

// MarshalXML implements custom XML serialization for Cell to enforce the
// mutual exclusion required by the SpreadsheetML schema:
//   - t="inlineStr" → only <is> is emitted
//   - formula present → <f> is emitted, <v> is optional (cached value)
//   - otherwise → only <v> is emitted
func (c Cell) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Attr = []xml.Attr{}
	if c.Ref != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "r"}, Value: c.Ref})
	}
	if c.StyleIndex != 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "s"}, Value: intString(c.StyleIndex)})
	}
	if c.Type != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "t"}, Value: c.Type})
	}

	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if c.Formula != nil {
		if err := e.EncodeElement(c.Formula, xml.StartElement{Name: xml.Name{Local: "f"}}); err != nil {
			return err
		}
	}

	if c.Type == "inlineStr" {
		if c.InlineStr != nil {
			if err := e.EncodeElement(c.InlineStr, xml.StartElement{Name: xml.Name{Local: "is"}}); err != nil {
				return err
			}
		}
	} else {
		// Emit <v> even if empty; Excel often expects it for non-string types.
		if err := e.EncodeElement(c.Value, xml.StartElement{Name: xml.Name{Local: "v"}}); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// MarshalXML implements custom XML serialization for RichText.
// When Runs is empty, it emits a plain <t> element.
// When Runs is non-empty, it emits <r> runs and skips the bare <t>.
func (rt RichText) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if len(rt.Runs) == 0 {
		if err := e.EncodeElement(rt.PlainText, xml.StartElement{Name: xml.Name{Local: "t"}}); err != nil {
			return err
		}
	} else {
		for _, run := range rt.Runs {
			if err := e.EncodeElement(run, xml.StartElement{Name: xml.Name{Local: "r"}}); err != nil {
				return err
			}
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// MarshalXML for RTRun ensures the <t> element inside a run does not get
// escaped in a way that breaks the shared-string format. Leading/trailing
// whitespace in run text must be preserved with xml:space="preserve".
func (r RTRun) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	if r.Properties != nil {
		if err := e.EncodeElement(r.Properties, xml.StartElement{Name: xml.Name{Local: "rPr"}}); err != nil {
			return err
		}
	}

	textStart := xml.StartElement{Name: xml.Name{Local: "t"}}
	if strings.TrimSpace(r.Text) != r.Text {
		textStart.Attr = []xml.Attr{
			{Name: xml.Name{Space: "http://www.w3.org/XML/1998/namespace", Local: "space"}, Value: "preserve"},
		}
	}
	if err := e.EncodeElement(r.Text, textStart); err != nil {
		return err
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// intString returns the decimal string of an int.
func intString(v int) string {
	// Fast path for common values.
	switch v {
	case 0:
		return "0"
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	case 5:
		return "5"
	}
	var buf [20]byte
	return string(itoa(buf[:], v))
}

func itoa(buf []byte, v int) []byte {
	if v < 0 {
		buf[0] = '-'
		return append(buf[:1], itoa(buf[1:], -v)...)
	}
	if v < 10 {
		buf[0] = byte('0' + v)
		return buf[:1]
	}
	return append(itoa(buf, v/10), byte('0'+v%10))
}
