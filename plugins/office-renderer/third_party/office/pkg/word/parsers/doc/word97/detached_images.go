package word97

import (
	"crypto/sha1"

	wordir "office/pkg/word/ir"
)

// AppendDetachedDrawingImages preserves OfficeArt BLIPs discovered during
// compatibility recovery when they could not be correlated to a canonical
// floating anchor. Anchored and inline images already present in the document
// are suppressed by payload digest.
func AppendDetachedDrawingImages(doc *wordir.Document, images []*wordir.Image) {
	if doc == nil || len(images) == 0 {
		return
	}
	seen := documentImageDigests(doc)
	for _, img := range images {
		if img == nil || len(img.Data) == 0 {
			continue
		}
		digest := sha1.Sum(img.Data)
		if _, ok := seen[digest]; ok {
			continue
		}
		seen[digest] = struct{}{}

		image := *img
		image.Data = append([]byte(nil), img.Data...)
		image.Floating = true
		paragraph := wordir.Paragraph{
			Index: len(doc.Paragraphs),
			Runs:  []wordir.Run{{Kind: "image", Image: &image}},
		}
		doc.Paragraphs = append(doc.Paragraphs, paragraph)
		doc.Blocks = append(doc.Blocks, wordir.Block{
			Kind:      "paragraph",
			Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1],
		})
	}
}

func documentImageDigests(doc *wordir.Document) map[[20]byte]struct{} {
	seen := map[[20]byte]struct{}{}
	if doc == nil {
		return seen
	}
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Image == nil || len(run.Image.Data) == 0 {
				continue
			}
			seen[sha1.Sum(run.Image.Data)] = struct{}{}
		}
	}
	return seen
}
