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

	c.reserveAuthoredRelationshipIDs(doc)
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
	// authoredRelIDs holds relationship IDs the IR already owns (hyperlinks and
	// linked objects). Generated parts must not reuse one of them.
	authoredRelIDs map[string]bool
}

func (c *converter) nextRID() string {
	for {
		c.nextRelID++
		rid := fmt.Sprintf("rId%d", c.nextRelID)
		if !c.authoredRelIDs[rid] {
			return rid
		}
	}
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
	if rid == "" {
		return
	}
	if c.authoredRelIDs == nil {
		c.authoredRelIDs = make(map[string]bool)
	}
	c.authoredRelIDs[rid] = true
	var n int
	if _, err := fmt.Sscanf(rid, "rId%d", &n); err == nil {
		if n >= c.nextRelID {
			c.nextRelID = n + 1
		}
	}
}

// reserveAuthoredRelationshipIDs claims every relationship ID the IR carries
// before anything is converted. nextRID hands out the IDs for generated parts
// (images, OLE previews, …), and the writer resolves a media part's target
// through its relationship, so a picture that happened to receive a hyperlink's
// ID would be published under the hyperlink's URL.
func (c *converter) reserveAuthoredRelationshipIDs(doc *wordir.Document) {
	c.reserveStoryRelationshipIDs(doc.Blocks)
	stories := make([]wordir.Story, 0,
		len(doc.Headers)+len(doc.Footers)+len(doc.Footnotes)+len(doc.Endnotes)+len(doc.Comments))
	for _, story := range doc.Headers {
		stories = append(stories, story)
	}
	for _, story := range doc.Footers {
		stories = append(stories, story)
	}
	for _, story := range doc.Footnotes {
		stories = append(stories, story)
	}
	for _, story := range doc.Endnotes {
		stories = append(stories, story)
	}
	for _, story := range doc.Comments {
		stories = append(stories, story)
	}
	for _, story := range stories {
		c.reserveStoryRelationshipIDs(story.Blocks)
	}
}

func (c *converter) reserveStoryRelationshipIDs(blocks []wordir.Block) {
	for i := range blocks {
		block := &blocks[i]
		if block.Paragraph != nil {
			c.reserveParagraphRelationshipIDs(block.Paragraph)
		}
		if block.Table != nil {
			c.reserveTableRelationshipIDs(block.Table)
		}
	}
}

func (c *converter) reserveTableRelationshipIDs(table *wordir.Table) {
	if table == nil {
		return
	}
	for ri := range table.Rows {
		for ci := range table.Rows[ri].Cells {
			cell := &table.Rows[ri].Cells[ci]
			for pi := range cell.Paragraphs {
				c.reserveParagraphRelationshipIDs(&cell.Paragraphs[pi])
			}
			for ti := range cell.Tables {
				c.reserveTableRelationshipIDs(&cell.Tables[ti])
			}
			c.reserveStoryRelationshipIDs(cell.Blocks)
		}
	}
}

func (c *converter) reserveParagraphRelationshipIDs(paragraph *wordir.Paragraph) {
	if paragraph == nil {
		return
	}
	for i := range paragraph.Runs {
		run := &paragraph.Runs[i]
		c.reserveRelID(run.RelID)
		c.reserveRelID(run.OLERelID)
		if run.TextBox != nil {
			c.reserveStoryRelationshipIDs(run.TextBox.Blocks)
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
