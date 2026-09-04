package edit

import wordir "office/pkg/word/ir"

// SetEmbeddedOLEObject turns this run into an embedded OLE/package object.
// preview may be nil; when supplied it is used as the visual representation.
func (r *Run) SetEmbeddedOLEObject(data []byte, mimeType, progID string, preview *wordir.Image) {
	if ir := r.ir(); ir != nil {
		ir.Kind = "olePlaceholder"
		ir.Text = progID
		ir.OLEProgID = progID
		ir.OLEData = append([]byte(nil), data...)
		ir.OLEMIME = mimeType
		ir.OLELinkURL = ""
		ir.OLEType = "Embed"
		ir.OLEPreview = cloneOLEPreview(preview)
	}
}

// SetLinkedOLEObject turns this run into an externally linked OLE object.
func (r *Run) SetLinkedOLEObject(targetURL, progID string, preview *wordir.Image) {
	if ir := r.ir(); ir != nil {
		ir.Kind = "olePlaceholder"
		ir.Text = progID
		ir.OLEProgID = progID
		ir.OLEData = nil
		ir.OLEMIME = ""
		ir.OLELinkURL = targetURL
		ir.OLEType = "Link"
		ir.OLEPreview = cloneOLEPreview(preview)
	}
}

// OLEObject reports whether the run carries an OLE object and returns its metadata.
func (r *Run) OLEObject() (progID string, embeddedData []byte, mimeType, linkURL string, ok bool) {
	ir := r.ir()
	if ir == nil || ir.Kind != "olePlaceholder" {
		return "", nil, "", "", false
	}
	return ir.OLEProgID, append([]byte(nil), ir.OLEData...), ir.OLEMIME, ir.OLELinkURL, true
}

// SetOLEPreview replaces the object's preview image without changing the object payload/link.
func (r *Run) SetOLEPreview(preview *wordir.Image) {
	if ir := r.ir(); ir != nil {
		ir.OLEPreview = cloneOLEPreview(preview)
	}
}

func cloneOLEPreview(src *wordir.Image) *wordir.Image {
	if src == nil {
		return nil
	}
	out := *src
	out.Data = append([]byte(nil), src.Data...)
	return &out
}