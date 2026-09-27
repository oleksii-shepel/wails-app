package sax

import (
	"bytes"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPass6MasterTextStyleMaterializesPictureBulletPayloadBeforeInheritance(t *testing.T) {
	b := newIRBuilder()
	const part = "ppt/slideMasters/slideMaster1.xml"
	b.partPayloads[part] = map[string]relationshipPayload{
		"rId3": {Data: []byte("PNG-BULLET"), ContentType: "image/png"},
	}

	if err := b.HandleEvent(Event{
		Type:           EventTextStyle,
		Scope:          "master",
		PartName:       part,
		TextStyleKind:  "bodyStyle",
		TextStyleLevel: 0,
		Paragraph: ParagraphProperties{
			HasBullet:        true,
			BulletImageRelID: "rId3",
			BulletSizePct:    113071,
		},
	}); err != nil {
		t.Fatal(err)
	}

	style := b.masterTextStyles["bodyStyle"][0].Paragraph
	if !bytes.Equal(style.BulletImageData, []byte("PNG-BULLET")) || style.BulletImageMIME != "image/png" {
		t.Fatalf("master style picture bullet payload = %q/%q", string(style.BulletImageData), style.BulletImageMIME)
	}

	p := &ir.Paragraph{}
	b.inheritParagraphStyle(p, style)
	if p.BulletNone || p.BulletImageRelID != "rId3" || !bytes.Equal(p.BulletImageData, []byte("PNG-BULLET")) || p.BulletImageMIME != "image/png" {
		t.Fatalf("inherited picture bullet = none:%v rel:%q data:%q mime:%q", p.BulletNone, p.BulletImageRelID, string(p.BulletImageData), p.BulletImageMIME)
	}
}
