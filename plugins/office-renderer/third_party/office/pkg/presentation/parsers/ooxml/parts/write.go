package parts

import (
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

// WritePackage serializes a Presentation parts.Package into an OPC ZIP archive.
func WritePackage(pkg *Package, w io.Writer) error {
	if pkg == nil {
		return fmt.Errorf("nil package")
	}

	opcPkg := opc.NewPackage()
	for _, part := range pkg.PreservedParts {
		if part.Name == "" || strings.EqualFold(part.Name, "[content_types].xml") {
			continue
		}
		opcPkg.AddPart(part.Name, part.ContentType, append([]byte(nil), part.Data...))
	}
	relMgr := newRelationshipManagerForPackage(pkg)

	// Root rel: presentation.
	if pkg.Presentation != nil {
		relMgr.addRootRel(relTypeOfficeDocument, "ppt/presentation.xml")

		// Rebuild presentation-level ID lists so relationship IDs stay consistent.
		pkg.Presentation.SlideMasterIDs = nil
		pkg.Presentation.SlideIDs = nil

		// Presentation relationships: slide masters and their layouts.
		// Layouts are written first so each slide can reference its layout rel.
		for masterIdx, entry := range pkg.SlideMasters {
			if entry == nil || entry.Master == nil || entry.PartName == "" {
				continue
			}
			masterRel := relMgr.addPresentationRel(relTypeSlideMaster, relPartName(entry.PartName))
			entry.RelID = masterRel.ID
			pkg.Presentation.SlideMasterIDs = append(pkg.Presentation.SlideMasterIDs, IDRef{
				ID:    2147483648 + masterIdx,
				RelID: masterRel.ID,
			})

			entry.Master.SlideLayouts = nil
			var masterRels []rels.Relationship
			for layoutIdx, layout := range entry.Layouts {
				if layout == nil || layout.PartName == "" {
					continue
				}
				layoutRel := relMgr.addPresentationRel(relTypeSlideLayout, relPartName(layout.PartName))
				layout.RelID = layoutRel.ID
				layoutData, err := xml.Marshal(layout)
				if err != nil {
					return fmt.Errorf("marshal slide layout %s: %w", layout.PartName, err)
				}
				opcPkg.AddPart(layout.PartName, "application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml", prependXMLHeader(addSlideLayoutNamespace(string(layoutData))))

				// Layout relationship back to its slide master.
				layoutRels := rels.Document{Relationships: []rels.Relationship{{
					ID:     "rIdMaster",
					Type:   relTypeSlideMaster,
					Target: makeRelTarget(layout.PartName, entry.PartName),
				}}}
				layoutRelsData, err := marshalMergedRels(pkg, rels.RelationshipsPartName(layout.PartName), layoutRels)
				if err != nil {
					return fmt.Errorf("marshal layout rels for %s: %w", layout.PartName, err)
				}
				opcPkg.AddPart(rels.RelationshipsPartName(layout.PartName), "application/vnd.openxmlformats-package.relationships+xml", layoutRelsData)

				entry.Master.SlideLayouts = append(entry.Master.SlideLayouts, IDRef{
					ID:    layoutIdx + 1,
					RelID: layoutRel.ID,
				})
				masterRels = append(masterRels, rels.Relationship{
					ID:     layoutRel.ID,
					Type:   relTypeSlideLayout,
					Target: makeRelTarget(entry.PartName, layout.PartName),
				})
			}

			// Master relationship to the first theme.
			for themePartName := range pkg.Themes {
				masterRels = append(masterRels, rels.Relationship{
					ID:     "rIdTheme",
					Type:   relTypeTheme,
					Target: makeRelTarget(entry.PartName, themePartName),
				})
				break
			}
			if len(masterRels) > 0 {
				masterRelsData, err := marshalMergedRels(pkg, rels.RelationshipsPartName(entry.PartName), rels.Document{Relationships: masterRels})
				if err != nil {
					return fmt.Errorf("marshal master rels for %s: %w", entry.PartName, err)
				}
				opcPkg.AddPart(rels.RelationshipsPartName(entry.PartName), "application/vnd.openxmlformats-package.relationships+xml", masterRelsData)
			}

			masterData, err := xml.Marshal(entry.Master)
			if err != nil {
				return fmt.Errorf("marshal slide master %s: %w", entry.PartName, err)
			}
			opcPkg.AddPart(entry.PartName, "application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml", prependXMLHeader(addSlideMasterNamespace(string(masterData))))
		}

		// Presentation relationships: slides.
		commentNum := 0
		notesNum := 0
		for i, entry := range pkg.Slides {
			slideRel := relMgr.addPresentationRel(relTypeSlide, relPartName(entry.PartName))
			entry.RelID = slideRel.ID
			pkg.Presentation.SlideIDs = append(pkg.Presentation.SlideIDs, IDRef{
				ID:    256 + i,
				RelID: slideRel.ID,
			})
			// Collect slide-level relationship IDs that are already reserved so
			// SmartArt relationship IDs do not collide with them.
			usedSlideRelIDs := preservedRelationshipIDs(pkg, rels.RelationshipsPartName(entry.PartName))
			if entry.Layout != nil && entry.Layout.RelID != "" {
				usedSlideRelIDs["rIdLayout"] = true
			}
			for relID := range entry.MediaParts {
				usedSlideRelIDs[relID] = true
			}
			for relID := range entry.ChartParts {
				usedSlideRelIDs[relID] = true
			}
			for relID := range entry.OleObjectParts {
				usedSlideRelIDs[relID] = true
			}
			for relID := range entry.OleObjectLinks {
				usedSlideRelIDs[relID] = true
			}
			for relID := range entry.HlinkURLs {
				usedSlideRelIDs[relID] = true
			}
			if len(entry.Comments) > 0 {
				usedSlideRelIDs["rIdComments"] = true
			}
			if entry.Notes != nil {
				usedSlideRelIDs["rIdNotes"] = true
			}

			// Write SmartArt diagram parts and update hosting graphicFrames.
			var smartArtSlideRels []rels.Relationship
			if len(entry.SmartArtParts) > 0 {
				smartArtSlideRels = writeSlideSmartArt(entry, relMgr, opcPkg, usedSlideRelIDs)
			}

			if entry.Slide != nil {
				slideData, err := xml.Marshal(entry.Slide)
				if err != nil {
					return fmt.Errorf("marshal slide %s: %w", entry.PartName, err)
				}
				opcPkg.AddPart(entry.PartName, "application/vnd.openxmlformats-officedocument.presentationml.slide+xml", prependXMLHeader(addSlideNamespace(string(slideData))))
			}

			// Write slide-level relationships.
			var slideRels []rels.Relationship
			slideRels = append(slideRels, smartArtSlideRels...)
			if entry.Layout != nil && entry.Layout.RelID != "" {
				slideRels = append(slideRels, rels.Relationship{
					ID:     "rIdLayout",
					Type:   relTypeSlideLayout,
					Target: makeRelTarget(entry.PartName, entry.Layout.PartName),
				})
			}

			// Write slide-level media and chart relationships.
			for relID, mp := range entry.MediaParts {
				target := makeRelTarget(entry.PartName, mp.Name)
				relType := relTypeImage
				switch {
				case strings.HasPrefix(mp.ContentType, "audio/"):
					relType = relTypeAudio
				case strings.HasPrefix(mp.ContentType, "video/"):
					relType = relTypeVideo
				}
				slideRels = append(slideRels, rels.Relationship{ID: relID, Type: relType, Target: target})
				opcPkg.AddPart(mp.Name, mp.ContentType, mp.Data)
			}
			chartNum := 0
			chartRelIDs := make([]string, 0, len(entry.ChartParts))
			for relID := range entry.ChartParts {
				chartRelIDs = append(chartRelIDs, relID)
			}
			sort.Strings(chartRelIDs)
			for _, relID := range chartRelIDs {
				chartNum++
				chartPartName := fmt.Sprintf("ppt/charts/chart%d.xml", chartNum)
				target := makeRelTarget(entry.PartName, chartPartName)
				slideRels = append(slideRels, rels.Relationship{ID: relID, Type: relTypeChart, Target: target})
				opcPkg.AddPart(chartPartName, "application/vnd.openxmlformats-officedocument.drawingml.chart+xml", prependXMLHeader(entry.ChartParts[relID]))
			}

			oleRelIDs := make([]string, 0, len(entry.OleObjectParts))
			for relID := range entry.OleObjectParts {
				oleRelIDs = append(oleRelIDs, relID)
			}
			sort.Strings(oleRelIDs)
			for _, relID := range oleRelIDs {
				part := entry.OleObjectParts[relID]
				if part.Name == "" {
					part.Name = fmt.Sprintf("ppt/embeddings/oleObject%s.bin", strings.TrimPrefix(relID, "rId"))
				}
				if part.ContentType == "" {
					part.ContentType = "application/vnd.openxmlformats-officedocument.oleObject"
				}
				relType := relTypePackage
				if strings.Contains(strings.ToLower(part.ContentType), "oleobject") {
					relType = relTypeOLEObject
				}
				target := makeRelTarget(entry.PartName, part.Name)
				slideRels = append(slideRels, rels.Relationship{ID: relID, Type: relType, Target: target})
				opcPkg.AddPart(part.Name, part.ContentType, part.Data)
			}

			// Write externally linked OLE/package relationships.
			if len(entry.OleObjectLinks) > 0 {
				linkRelIDs := make([]string, 0, len(entry.OleObjectLinks))
				for rid := range entry.OleObjectLinks {
					linkRelIDs = append(linkRelIDs, rid)
				}
				sort.Strings(linkRelIDs)
				for _, rid := range linkRelIDs {
					target := entry.OleObjectLinks[rid]
					if strings.TrimSpace(target) == "" {
						continue
					}
					slideRels = append(slideRels, rels.Relationship{ID: rid, Type: relTypeOLEObject, Target: target, TargetMode: "External"})
				}
			}

			// Write slide-level hyperlinks.
			if len(entry.HlinkURLs) > 0 {
				hlinkRelIDs := make([]string, 0, len(entry.HlinkURLs))
				for rid := range entry.HlinkURLs {
					hlinkRelIDs = append(hlinkRelIDs, rid)
				}
				sort.Strings(hlinkRelIDs)
				for _, rid := range hlinkRelIDs {
					slideRels = append(slideRels, rels.Relationship{
						ID:         rid,
						Type:       relTypeHyperlink,
						Target:     entry.HlinkURLs[rid],
						TargetMode: "External",
					})
				}
			}

			// Write slide-level comments.
			if len(entry.Comments) > 0 {
				commentNum++
				commentsPartName := fmt.Sprintf("ppt/comments/comment%d.xml", commentNum)
				commentsRelID := "rIdComments"
				target := makeRelTarget(entry.PartName, commentsPartName)
				slideRels = append(slideRels, rels.Relationship{ID: commentsRelID, Type: relTypeComments, Target: target})
				comments := CommentList{Comments: make([]xmlCm, 0, len(entry.Comments))}
				for _, cm := range entry.Comments {
					if cm == nil {
						continue
					}
					comments.Comments = append(comments.Comments, xmlCm{
						AuthorID: cm.AuthorID,
						Index:    cm.Index,
						DateTime: cm.DateTime,
						Pos:      &xmlCmPos{X: cm.X, Y: cm.Y},
						Text:     cm.Text,
					})
				}
				commentsData, err := xml.Marshal(comments)
				if err != nil {
					return fmt.Errorf("marshal comments for %s: %w", entry.PartName, err)
				}
				opcPkg.AddPart(commentsPartName, "application/vnd.openxmlformats-officedocument.presentationml.comments+xml", prependXMLHeader(addCommentsNamespace(string(commentsData))))
			}

			// Write notes slide for this slide.
			if entry.Notes != nil {
				notesNum++
				notesPartName := fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", notesNum)
				notesRelID := "rIdNotes"
				target := makeRelTarget(entry.PartName, notesPartName)
				slideRels = append(slideRels, rels.Relationship{ID: notesRelID, Type: relTypeNotesSlide, Target: target})
				notesData, err := xml.Marshal(entry.Notes)
				if err != nil {
					return fmt.Errorf("marshal notes slide for %s: %w", entry.PartName, err)
				}
				opcPkg.AddPart(notesPartName, "application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml", prependXMLHeader(addNotesSlideNamespace(string(notesData))))

				notesRels := rels.Document{Relationships: []rels.Relationship{{ID: "rIdNotesMaster", Type: relTypeNotesMaster, Target: "../notesMasters/notesMaster1.xml"}}}
				notesRelsData, err := marshalMergedRels(pkg, rels.RelationshipsPartName(notesPartName), notesRels)
				if err != nil {
					return fmt.Errorf("marshal notes slide rels for %s: %w", entry.PartName, err)
				}
				opcPkg.AddPart(rels.RelationshipsPartName(notesPartName), "application/vnd.openxmlformats-package.relationships+xml", notesRelsData)
			}

			relsPath := rels.RelationshipsPartName(entry.PartName)
			_, hasPreservedSlideRels := preservedPartData(pkg, relsPath)
			if len(slideRels) > 0 || hasPreservedSlideRels {
				relsDoc := rels.Document{Relationships: slideRels}
				relsData, err := marshalMergedRels(pkg, relsPath, relsDoc)
				if err != nil {
					return fmt.Errorf("marshal slide rels for %s: %w", entry.PartName, err)
				}
				opcPkg.AddPart(relsPath, "application/vnd.openxmlformats-package.relationships+xml", relsData)
			}
		}

		// Write notes master and notes slides if any slide has notes.
		hasNotes := false
		for _, entry := range pkg.Slides {
			if entry.Notes != nil {
				hasNotes = true
				break
			}
		}
		if hasNotes {
			if pkg.NotesMaster == nil {
				pkg.NotesMaster = DefaultNotesMaster()
			}
			notesMasterPartName := "ppt/notesMasters/notesMaster1.xml"
			notesMasterData, err := xml.Marshal(pkg.NotesMaster)
			if err != nil {
				return fmt.Errorf("marshal notes master: %w", err)
			}
			opcPkg.AddPart(notesMasterPartName, "application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml", prependXMLHeader(addNotesMasterNamespace(string(notesMasterData))))
			relMgr.addPresentationRel(relTypeNotesMaster, "notesMasters/notesMaster1.xml")

			// Notes master relationship to the first theme.
			var themeTarget string
			for partName := range pkg.Themes {
				themeTarget = makeRelTarget(notesMasterPartName, partName)
				break
			}
			if themeTarget != "" {
				notesMasterRels := rels.Document{Relationships: []rels.Relationship{{ID: "rIdTheme", Type: relTypeTheme, Target: themeTarget}}}
				notesMasterRelsData, err := marshalMergedRels(pkg, "ppt/notesMasters/_rels/notesMaster1.xml.rels", notesMasterRels)
				if err != nil {
					return fmt.Errorf("marshal notes master rels: %w", err)
				}
				opcPkg.AddPart("ppt/notesMasters/_rels/notesMaster1.xml.rels", "application/vnd.openxmlformats-package.relationships+xml", notesMasterRelsData)
			}
		}

		// Write comment authors if any comments were emitted.
		if len(pkg.CommentAuthors) > 0 {
			authors := CommentAuthorList{Authors: make([]xmlCmAuthor, 0, len(pkg.CommentAuthors))}
			for id, author := range pkg.CommentAuthors {
				authors.Authors = append(authors.Authors, xmlCmAuthor{
					ID:       id,
					Name:     author.Name,
					Initials: author.Initials,
				})
			}
			sort.Slice(authors.Authors, func(i, j int) bool {
				return authors.Authors[i].ID < authors.Authors[j].ID
			})
			authorsData, err := xml.Marshal(authors)
			if err != nil {
				return fmt.Errorf("marshal comment authors: %w", err)
			}
			opcPkg.AddPart("ppt/commentAuthors.xml", "application/vnd.openxmlformats-officedocument.presentationml.commentAuthors+xml", prependXMLHeader(addCommentAuthorsNamespace(string(authorsData))))
			relMgr.addPresentationRel(relTypeCommentAuthors, "commentAuthors.xml")
		}

		// Themes.
		for partName, theme := range pkg.Themes {
			rel := relMgr.addPresentationRel(relTypeTheme, relPartName(partName))
			themeData, err := xml.Marshal(theme)
			if err != nil {
				return fmt.Errorf("marshal theme %s: %w", partName, err)
			}
			opcPkg.AddPart(partName, "application/vnd.openxmlformats-officedocument.theme+xml", prependXMLHeader(addThemeNamespace(string(themeData))))
			_ = rel
		}

		// Table styles.
		if pkg.TableStyles != nil {
			rel := relMgr.addPresentationRel(relTypeTableStyles, "tableStyles.xml")
			tsData, err := xml.Marshal(pkg.TableStyles)
			if err != nil {
				return fmt.Errorf("marshal tableStyles: %w", err)
			}
			opcPkg.AddPart("ppt/tableStyles.xml", "application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml", prependXMLHeader(tsData))
			_ = rel
		}

		// Embedded fonts.
		if len(pkg.EmbeddedFonts) > 0 {
			pkg.Presentation.EmbeddedFontDecls = buildEmbeddedFontDecls(pkg, opcPkg, relMgr)
		}

		// Marshal presentation XML after all relationship IDs are assigned.
		presData, err := xml.Marshal(pkg.Presentation)
		if err != nil {
			return fmt.Errorf("marshal presentation: %w", err)
		}
		opcPkg.AddPart("ppt/presentation.xml", "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml", prependXMLHeader(addPresNamespace(string(presData))))

		// Write presentation rels.
		presRels := relMgr.presentationRels()
		presRelsData, err := marshalMergedRels(pkg, "ppt/_rels/presentation.xml.rels", presRels)
		if err != nil {
			return fmt.Errorf("marshal presentation rels: %w", err)
		}
		opcPkg.AddPart("ppt/_rels/presentation.xml.rels", "application/vnd.openxmlformats-package.relationships+xml", presRelsData)
	}

	// Root rels.
	rootRels := relMgr.rootRels()
	rootRelsData, err := marshalMergedRels(pkg, "_rels/.rels", rootRels)
	if err != nil {
		return fmt.Errorf("marshal root rels: %w", err)
	}
	opcPkg.AddPart("_rels/.rels", "application/vnd.openxmlformats-package.relationships+xml", rootRelsData)

	pruneUnreachableManagedPresentationParts(opcPkg)
	return opcPkg.Write(w)
}

func addPresNamespace(s string) []byte {
	const ns = ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	return injectNamespace(s, "presentation", ns)
}

func addNotesMasterNamespace(s string) []byte {
	const ns = ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	return injectNamespace(s, "notesMaster", ns)
}

func addNotesSlideNamespace(s string) []byte {
	const ns = ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	return injectNamespace(s, "notes", ns)
}

func addCommentsNamespace(s string) []byte {
	const ns = ` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	return injectNamespace(s, "cmLst", ns)
}

func addCommentAuthorsNamespace(s string) []byte {
	const ns = ` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	return injectNamespace(s, "cmAuthorLst", ns)
}

func addSlideNamespace(s string) []byte {
	const ns = ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	return []byte(cleanHlinkClickNamespaces(string(injectNamespace(s, "sld", ns))))
}

func addSlideMasterNamespace(s string) []byte {
	const ns = ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	return injectNamespace(s, "sldMaster", ns)
}

func addSlideLayoutNamespace(s string) []byte {
	const ns = ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
	return injectNamespace(s, "sldLayout", ns)
}

func addThemeNamespace(s string) []byte {
	const ns = ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
	return injectNamespace(s, "theme", ns)
}

// cleanHlinkClickNamespaces rewrites hlinkClick elements produced by Go's
// encoding/xml into the prefixed form expected by OOXML consumers. After the
// root slide element declares xmlns:a and xmlns:r, hlinkClick can use
// <a:hlinkClick r:id="..."> instead of redeclaring namespaces and inventing a
// relationships prefix.
func cleanHlinkClickNamespaces(s string) string {
	for {
		start := strings.Index(s, "<hlinkClick")
		if start < 0 {
			break
		}
		// Find the end of the start tag.
		startTagEnd := strings.Index(s[start:], ">")
		if startTagEnd < 0 {
			break
		}
		startTagEnd += start
		startTag := s[start : startTagEnd+1]

		var rid, action string
		for _, attr := range []string{"relationships:id=", "r:id="} {
			if idx := strings.Index(startTag, attr); idx >= 0 {
				q := startTag[idx+len(attr):]
				if len(q) > 0 && (q[0] == '\'' || q[0] == '"') {
					term := string(q[0])
					q = q[1:]
					if closeIdx := strings.Index(q, term); closeIdx >= 0 {
						rid = q[:closeIdx]
					}
				}
				break
			}
		}
		if idx := strings.Index(startTag, "action="); idx >= 0 {
			q := startTag[idx+7:]
			if len(q) > 0 && (q[0] == '\'' || q[0] == '"') {
				term := string(q[0])
				q = q[1:]
				if closeIdx := strings.Index(q, term); closeIdx >= 0 {
					action = q[:closeIdx]
				}
			}
		}

		replacement := "<a:hlinkClick"
		if rid != "" {
			replacement += fmt.Sprintf(` r:id=%q`, rid)
		}
		if action != "" {
			replacement += fmt.Sprintf(` action=%q`, action)
		}

		var elemEnd int
		if startTag[len(startTag)-2:] == " />" || startTag[len(startTag)-2:] == "/>" {
			// Self-closing tag.
			replacement += "/>"
			elemEnd = startTagEnd + 1
		} else {
			// Find matching end tag.
			endTagStart := strings.Index(s[startTagEnd:], "</hlinkClick>")
			if endTagStart < 0 {
				break
			}
			endTagStart += startTagEnd
			replacement += "></a:hlinkClick>"
			elemEnd = endTagStart + len("</hlinkClick>")
		}
		s = s[:start] + replacement + s[elemEnd:]
	}
	return s
}

// injectNamespace inserts namespace declarations into the start tag of the
// first element matching elemName. It handles both empty start tags
// (e.g. <sld>) and start tags that already declare a default namespace
// (e.g. <sld xmlns="...">).
func injectNamespace(s, elemName, ns string) []byte {
	prefix := "<" + elemName
	for i := 0; i <= len(s)-len(prefix); i++ {
		if s[i:i+len(prefix)] != prefix {
			continue
		}
		// Ensure this is really the start tag, not a longer element name.
		nextIdx := i + len(prefix)
		if nextIdx >= len(s) {
			continue
		}
		next := s[nextIdx]
		if next != ' ' && next != '>' && next != '/' {
			continue
		}
		// Find the end of the start tag.
		for j := nextIdx; j < len(s); j++ {
			if s[j] == '>' {
				return []byte(s[:j] + ns + s[j:])
			}
		}
	}
	return []byte(s)
}

func prependXMLHeader(data []byte) []byte {
	if len(data) > 0 && data[0] == '<' {
		return append([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`), data...)
	}
	return data
}

func preservedPartData(pkg *Package, name string) ([]byte, bool) {
	if pkg == nil {
		return nil, false
	}
	name = strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "/")
	for _, part := range pkg.PreservedParts {
		partName := strings.TrimPrefix(strings.ReplaceAll(part.Name, "\\", "/"), "/")
		if strings.EqualFold(partName, name) {
			return part.Data, true
		}
	}
	return nil, false
}

func preservedRelationshipIDs(pkg *Package, relsPartName string) map[string]bool {
	out := map[string]bool{}
	data, ok := preservedPartData(pkg, relsPartName)
	if !ok || len(data) == 0 {
		return out
	}
	var doc rels.Document
	if err := xml.Unmarshal(data, &doc); err != nil {
		return out
	}
	for _, rel := range doc.Relationships {
		if rel.ID != "" {
			out[rel.ID] = true
		}
	}
	return out
}

// marshalMergedRels preserves original relationships that are not replaced by
// the modeled writer. This keeps extension/vendor relationships and their
// opaque target parts alive across load/edit/save round trips.
func marshalMergedRels(pkg *Package, relsPartName string, generated rels.Document) ([]byte, error) {
	data, ok := preservedPartData(pkg, relsPartName)
	if !ok || len(data) == 0 {
		return marshalRels(generated)
	}
	var original rels.Document
	if err := xml.Unmarshal(data, &original); err != nil {
		// A malformed original .rels should not prevent saving a repaired package.
		return marshalRels(generated)
	}
	usedIDs := make(map[string]bool, len(generated.Relationships))
	seen := make(map[string]bool, len(generated.Relationships))
	for _, rel := range generated.Relationships {
		usedIDs[rel.ID] = true
		seen[rel.Type+"\x00"+rel.Target+"\x00"+rel.TargetMode] = true
	}
	for _, rel := range original.Relationships {
		if rel.ID == "" || usedIDs[rel.ID] {
			continue
		}
		// Relationships owned by the modeled presentation writer are regenerated
		// from the edited semantic package. If one is absent from generated, the
		// corresponding object was removed and the old relationship must not be
		// resurrected from the preserved source package.
		if isManagedPresentationRelationshipForPart(relsPartName, rel.Type) {
			continue
		}
		key := rel.Type + "\x00" + rel.Target + "\x00" + rel.TargetMode
		if seen[key] {
			continue
		}
		generated.Relationships = append(generated.Relationships, rel)
		usedIDs[rel.ID] = true
		seen[key] = true
	}
	sort.SliceStable(generated.Relationships, func(i, j int) bool {
		return relationshipIDNumber(generated.Relationships[i].ID) < relationshipIDNumber(generated.Relationships[j].ID)
	})
	return marshalRels(generated)
}

func relationshipIDNumber(id string) int {
	if strings.HasPrefix(id, "rId") {
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "rId")); err == nil {
			return n
		}
	}
	return int(^uint(0) >> 1)
}

func newRelationshipManagerForPackage(pkg *Package) *relationshipManager {
	rm := newRelationshipManager()
	maxID := 0
	if pkg != nil {
		for _, part := range pkg.PreservedParts {
			if !strings.HasSuffix(strings.ToLower(part.Name), ".rels") {
				continue
			}
			var doc rels.Document
			if err := xml.Unmarshal(part.Data, &doc); err != nil {
				continue
			}
			for _, rel := range doc.Relationships {
				if n := relationshipIDNumber(rel.ID); n > maxID && n < int(^uint(0)>>1) {
					maxID = n
				}
			}
		}
	}
	if maxID >= rm.nextID {
		rm.nextID = maxID + 1
	}
	return rm
}

func marshalRels(doc rels.Document) ([]byte, error) {
	data, err := xml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return prependXMLHeader(data), nil
}

func relPartName(name string) string {
	if len(name) > 4 && name[:4] == "ppt/" {
		return name[4:]
	}
	return name
}

func makeRelTarget(sourcePart, targetPart string) string {
	// Compute a relative path from sourcePart to targetPart.
	// e.g. source="ppt/slides/slide1.xml" target="ppt/media/image1.png" → "../media/image1.png"
	sourceDir := path.Dir(sourcePart)
	targetDir := path.Dir(targetPart)
	targetFile := path.Base(targetPart)
	if sourceDir == targetDir {
		return targetFile
	}
	// Count source parent levels.
	var ups []string
	for {
		if strings.HasPrefix(targetDir, sourceDir+"/") {
			rel := strings.TrimPrefix(targetDir, sourceDir+"/")
			if len(ups) > 0 {
				return strings.Join(ups, "/") + "/" + rel + "/" + targetFile
			}
			return rel + "/" + targetFile
		}
		if sourceDir == "" || sourceDir == "." {
			break
		}
		ups = append(ups, "..")
		sourceDir = path.Dir(sourceDir)
	}
	if len(ups) > 0 {
		return strings.Join(ups, "/") + "/" + targetDir + "/" + targetFile
	}
	return targetDir + "/" + targetFile
}

type relationshipManager struct {
	nextID       int
	root         []rels.Relationship
	presentation []rels.Relationship
}

func newRelationshipManager() *relationshipManager {
	return &relationshipManager{nextID: 1}
}

func (rm *relationshipManager) nextRelID() string {
	id := "rId" + strconv.Itoa(rm.nextID)
	rm.nextID++
	return id
}

func (rm *relationshipManager) addRootRel(relType, target string) rels.Relationship {
	r := rels.Relationship{ID: rm.nextRelID(), Type: relType, Target: target}
	rm.root = append(rm.root, r)
	return r
}

func (rm *relationshipManager) addPresentationRel(relType, target string) rels.Relationship {
	r := rels.Relationship{ID: rm.nextRelID(), Type: relType, Target: target}
	rm.presentation = append(rm.presentation, r)
	return r
}

func (rm *relationshipManager) rootRels() rels.Document {
	return rels.Document{Relationships: rm.root}
}

func (rm *relationshipManager) presentationRels() rels.Document {
	return rels.Document{Relationships: rm.presentation}
}

func buildEmbeddedFontDecls(pkg *Package, opcPkg *opc.Package, rm *relationshipManager) []EmbeddedFontDecl {
	fonts := make([]*EmbeddedFontData, len(pkg.EmbeddedFonts))
	copy(fonts, pkg.EmbeddedFonts)
	sort.Slice(fonts, func(i, j int) bool {
		return fonts[i].Typeface < fonts[j].Typeface
	})

	decls := make([]EmbeddedFontDecl, 0, len(fonts))
	usedNames := make(map[string]bool)
	counter := 0

	for _, font := range fonts {
		if font == nil || strings.TrimSpace(font.Typeface) == "" {
			continue
		}
		decl := EmbeddedFontDecl{
			Font: FontRef{Typeface: strings.TrimSpace(font.Typeface)},
		}

		add := func(part *FontPart, suffix string, ref **EmbeddedFontStyleRef) {
			if part == nil || len(part.Data) == 0 {
				return
			}
			partName := part.Name
			if partName == "" {
				base := safeFontFileName(font.Typeface) + "-" + suffix
				for usedNames[base] {
					counter++
					base = safeFontFileName(font.Typeface) + "-" + suffix + strconv.Itoa(counter)
				}
				partName = "ppt/fonts/" + base + ".fntdata"
			}
			if usedNames[partName] {
				// Fallback: append counter to an existing loaded name to avoid collisions.
				counter++
				partName = "ppt/fonts/" + safeFontFileName(font.Typeface) + "-" + suffix + strconv.Itoa(counter) + ".fntdata"
			}
			usedNames[partName] = true

			contentType := part.ContentType
			if contentType == "" {
				contentType = "application/x-fontdata"
			}
			rel := rm.addPresentationRel(relTypeFont, makeRelTarget("ppt/presentation.xml", partName))
			opcPkg.AddPart(partName, contentType, part.Data)
			*ref = &EmbeddedFontStyleRef{RelID: rel.ID}
		}

		add(font.Regular, "regular", &decl.Regular)
		add(font.Bold, "bold", &decl.Bold)
		add(font.Italic, "italic", &decl.Italic)
		add(font.BoldItalic, "boldItalic", &decl.BoldItalic)

		decls = append(decls, decl)
	}

	return decls
}

func safeFontFileName(family string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(family) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	name := strings.ToLower(b.String())
	if name == "" {
		return "font"
	}
	return name
}

func isManagedPresentationRelationshipForPart(relsPartName, relType string) bool {
	source, ok := relationshipSourcePart(relsPartName)
	if !ok {
		return false
	}
	typ := canonicalRelationshipType(relType)
	if source == "" {
		return typ == relTypeOfficeDocument
	}
	if source == "ppt/presentation.xml" {
		switch typ {
		case relTypeSlideMaster, relTypeSlide, relTypeNotesMaster, relTypeTheme,
			relTypeFont, relTypeCommentAuthors, relTypeTableStyles:
			return true
		}
		return false
	}
	if strings.HasPrefix(source, "ppt/slides/") {
		switch typ {
		case relTypeSlideLayout, relTypeImage, relTypeAudio, relTypeVideo,
			relTypeChart, relTypeOLEObject, relTypePackage, relTypeNotesSlide,
			relTypeComments, relTypeHyperlink, relTypeDiagramData,
			relTypeDiagramLayout, relTypeDiagramStyle, relTypeDiagramColors,
			relTypeDiagramQuickStyle:
			return true
		}
		return strings.TrimSpace(relType) == relTypeDiagramDrawing
	}
	if strings.HasPrefix(source, "ppt/slideMasters/") {
		return typ == relTypeSlideLayout || typ == relTypeTheme
	}
	if strings.HasPrefix(source, "ppt/slideLayouts/") {
		return typ == relTypeSlideMaster
	}
	if strings.HasPrefix(source, "ppt/notesSlides/") {
		return typ == relTypeNotesMaster
	}
	if strings.HasPrefix(source, "ppt/notesMasters/") {
		return typ == relTypeTheme
	}
	if strings.HasPrefix(source, "ppt/diagrams/") {
		switch typ {
		case relTypeDiagramData, relTypeDiagramLayout, relTypeDiagramStyle,
			relTypeDiagramColors, relTypeDiagramQuickStyle, relTypeImage:
			return true
		}
		return strings.TrimSpace(relType) == relTypeDiagramDrawing
	}
	return false
}

func pruneUnreachableManagedPresentationParts(pkg *opc.Package) {
	if pkg == nil {
		return
	}
	reachable := map[string]bool{}
	seenSources := map[string]bool{}
	queue := []string{""}
	for len(queue) > 0 {
		source := queue[0]
		queue = queue[1:]
		if seenSources[source] {
			continue
		}
		seenSources[source] = true
		relsName := rels.RelationshipsPartName(source)
		if _, ok := pkg.Part(relsName); ok {
			reachable[relsName] = true
		}
		doc, ok, err := pkg.Relationships(source)
		if err != nil || !ok || doc == nil {
			continue
		}
		for _, rel := range doc.Relationships {
			if rel.IsExternal() {
				continue
			}
			target := rels.ResolveTarget(source, rel.Target)
			if target == "" {
				continue
			}
			if !reachable[target] {
				reachable[target] = true
				queue = append(queue, target)
			}
		}
	}

	for _, part := range pkg.Parts() {
		if reachable[part.Name] || !isManagedPresentationPart(part.Name) {
			continue
		}
		pkg.RemovePart(part.Name)
	}
}

func isManagedPresentationPart(name string) bool {
	name = strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "/")
	if name == "" || strings.EqualFold(name, "[content_types].xml") || name == "_rels/.rels" {
		return false
	}
	if strings.HasSuffix(strings.ToLower(name), ".rels") {
		if source, ok := relationshipSourcePart(name); ok {
			return isManagedPresentationPart(source)
		}
		return false
	}
	if name == "ppt/presentation.xml" || name == "ppt/commentAuthors.xml" || name == "ppt/tableStyles.xml" {
		return true
	}
	for _, prefix := range []string{
		"ppt/slides/", "ppt/slideLayouts/", "ppt/slideMasters/",
		"ppt/notesSlides/", "ppt/notesMasters/", "ppt/comments/",
		"ppt/theme/", "ppt/charts/", "ppt/diagrams/", "ppt/media/",
		"ppt/embeddings/", "ppt/fonts/",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func relationshipSourcePart(name string) (string, bool) {
	name = strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "/")
	if name == "_rels/.rels" {
		return "", true
	}
	if !strings.HasSuffix(name, ".rels") {
		return "", false
	}
	dir := path.Dir(name)
	if path.Base(dir) != "_rels" {
		return "", false
	}
	base := strings.TrimSuffix(path.Base(name), ".rels")
	return path.Join(path.Dir(dir), base), true
}
