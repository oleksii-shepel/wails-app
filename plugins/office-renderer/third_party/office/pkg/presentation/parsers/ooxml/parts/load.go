package parts

import (
	"fmt"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

// LoadPackage parses the typed PresentationML parts from a generic OPC package.
// OPC discovery and top-level traversal are owned by packageAssembler so part
// decoders do not each reconstruct package semantics.
func LoadPackage(pkg *opc.Package) (*Package, error) {
	assembler, err := newPackageAssembler(pkg)
	if err != nil {
		return nil, err
	}
	return assembler.assemble()
}

func addPackageLoadDiagnostic(pkg *Package, severity diag.Severity, code, message, partName string, cause error) {
	if pkg == nil {
		return
	}
	pkg.Diagnostics.Add(diag.Diagnostic{
		Severity: severity,
		Code:     code,
		Message:  message,
		Location: diag.Location{Part: partName},
		Cause:    cause,
	})
}

func collectPackageDiagnostics(pkg *Package) diag.List {
	var d diag.List
	seenParts := map[string]struct{}{}
	for i, entry := range pkg.Slides {
		if entry.PartName == "" {
			d.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "pptx/missing-slide-part",
				Message:  fmt.Sprintf("slide %d (relID %q): relationship not resolved to a part", i+1, entry.RelID),
			})
		} else if entry.Slide == nil {
			d.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "pptx/empty-slide",
				Message:  fmt.Sprintf("slide %d: part loaded but parsed as empty", i+1),
			})
		} else {
			appendPresentationDiagnostics(&d, entry.PartName, collectCommonSlideDataDiagnostics(&entry.Slide.CSld), seenParts)
		}
		if entry.Layout != nil {
			appendPresentationDiagnostics(&d, entry.LayoutPartName, collectCommonSlideDataDiagnostics(&entry.Layout.CSld), seenParts)
		}
	}
	for _, master := range pkg.SlideMasters {
		if master == nil {
			continue
		}
		if master.Master != nil {
			appendPresentationDiagnostics(&d, master.PartName, collectCommonSlideDataDiagnostics(&master.Master.CSld), seenParts)
		}
	}
	if len(pkg.Themes) == 0 {
		d.Add(diag.Diagnostic{
			Severity: diag.SeverityInfo,
			Code:     "pptx/no-theme",
			Message:  "no theme part found; colors and fonts will not be resolved",
		})
	}
	return d
}

func appendPresentationDiagnostics(dst *diag.List, partName string, diagnostics diag.List, seenParts map[string]struct{}) {
	if dst == nil || len(diagnostics) == 0 {
		return
	}
	if partName != "" {
		if _, ok := seenParts[partName]; ok {
			return
		}
		seenParts[partName] = struct{}{}
	}
	for _, diagnostic := range diagnostics {
		if partName != "" && diagnostic.Location.Part == "" {
			diagnostic.Location.Part = partName
		}
		dst.Add(diagnostic)
	}
}

func collectCommonSlideDataDiagnostics(data *CommonSlideData) diag.List {
	if data == nil {
		return nil
	}
	return collectShapeTreeDiagnostics(&data.SpTree)
}

func collectShapeTreeDiagnostics(tree *ShapeTree) diag.List {
	if tree == nil {
		return nil
	}
	var diagnostics diag.List
	diagnostics = append(diagnostics, tree.Diagnostics...)
	for i := range tree.GroupShapes {
		appendGroupShapeDiagnostics(&diagnostics, &tree.GroupShapes[i])
	}
	return diagnostics
}

func appendGroupShapeDiagnostics(dst *diag.List, group *GroupShape) {
	if dst == nil || group == nil {
		return
	}
	*dst = append(*dst, group.Diagnostics...)
	for i := range group.GroupShapes {
		appendGroupShapeDiagnostics(dst, &group.GroupShapes[i])
	}
}

func findMainPart(nav packageNavigator) (string, error) {
	rootRels, ok, err := nav.relationships("")
	if err != nil {
		return "", fmt.Errorf("presentation parts: root relationships: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("presentation parts: root relationships not found")
	}
	for _, r := range rootRels.Relationships {
		if relationshipTypeIs(r.Type, relTypeOfficeDocument) && !r.IsExternal() {
			target, ok := nav.target("", r)
			if ok {
				return target, nil
			}
		}
	}
	return "", fmt.Errorf("presentation parts: officeDocument relationship not found")
}

func loadSlides(pkg *opc.Package, mainPartName string, slideIDs []IDRef, byID map[string]rels.Relationship, out *Package) error {
	for _, ref := range slideIDs {
		relID := normalizeRelID(ref.RelID)
		entry := &SlideEntry{RelID: relID}

		r, ok := byID[relID]
		if !ok || !relationshipTypeIs(r.Type, relTypeSlide) || r.IsExternal() {
			out.Slides = append(out.Slides, entry)
			continue
		}

		partName := rels.ResolveTarget(mainPartName, r.Target)
		entry.PartName = partName

		var slide Slide
		found, err := newPackageNavigator(pkg).decodeXML(partName, &slide)
		if !found {
			out.Slides = append(out.Slides, entry)
			continue
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityWarning, "pptx/malformed-slide", fmt.Sprintf("slide part could not be parsed; skipping slide content: %v", err), partName, err)
			out.Slides = append(out.Slides, entry)
			continue
		}
		entry.Slide = &slide
		entry.Slide.ThemeOverride = loadThemeOverrideForPart(pkg, partName)

		// Load the slide's layout and media via its own relationships.
		if err := loadSlideLayout(pkg, partName, entry, out); err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityWarning, "pptx/slide-layout-load-error", fmt.Sprintf("slide layout could not be loaded: %v", err), partName, err)
		}
		loadSlideMedia(pkg, partName, entry)

		// Load notes slide for this slide
		loadNotesSlide(pkg, partName, entry, out)

		// Load SmartArt parts for this slide
		smartArtParts, err := collectSmartArtReferences(pkg, partName)
		if err != nil {
			// Log error but don't fail - SmartArt is optional
			if out.Diagnostics == nil {
				out.Diagnostics = diag.List{}
			}
			out.Diagnostics.Add(diag.Diagnostic{
				Severity: diag.SeverityWarning,
				Code:     "pptx/smartart-load-error",
				Message:  fmt.Sprintf("Failed to load SmartArt parts for slide: %v", err),
				Location: diag.Location{Part: partName},
			})
		}
		entry.SmartArtParts = smartArtParts

		// PowerPoint commonly stores the diagram layout/colors/quick-style
		// relationships at the slide level, referenced by the hosting
		// p:graphicFrame. Load those now that the frame rel IDs are known.
		loadSmartArtRelatedPartsFromSlideRels(pkg, partName, entry)

		// Load Chart parts for this slide and capture their hosting frame bounds.
		entry.ChartParts = loadSlideChartParts(pkg, partName)
		entry.ChartFrames = buildChartFrameMap(entry.Slide)

		// Load embedded OLE object payloads for this slide.
		loadSlideOleObjectParts(pkg, partName, entry)

		// Load comments for this slide
		loadSlideComments(pkg, partName, entry, out)
		loadSlideHyperlinkURLs(pkg, partName, entry)

		out.Slides = append(out.Slides, entry)
	}
	return nil
}

func loadNotesSlide(pkg *opc.Package, slidePartName string, entry *SlideEntry, out *Package) {
	slideRels, ok, err := newPackageNavigator(pkg).relationships(slidePartName)
	if err != nil || !ok {
		return
	}
	for _, r := range slideRels.Relationships {
		if !relationshipTypeIs(r.Type, relTypeNotesSlide) || r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(slidePartName, r.Target)
		var notes NotesSlide
		found, decodeErr := newPackageNavigator(pkg).decodeXML(partName, &notes)
		if !found {
			continue
		}
		if decodeErr != nil {
			err := decodeErr
			addPackageLoadDiagnostic(out, diag.SeverityInfo, "pptx/malformed-notes-slide", fmt.Sprintf("notes slide could not be parsed and was skipped: %v", err), partName, err)
			continue
		}
		notes.ThemeOverride = loadThemeOverrideForPart(pkg, partName)
		entry.Notes = &notes
		break
	}
}

// loadSlideHyperlinkURLs collects external hyperlink relationships from a slide part.
func loadSlideHyperlinkURLs(pkg *opc.Package, slidePartName string, entry *SlideEntry) {
	slideRels, ok, err := newPackageNavigator(pkg).relationships(slidePartName)
	if err != nil || !ok {
		return
	}
	for _, r := range slideRels.Relationships {
		if !relationshipTypeIs(r.Type, relTypeHyperlink) {
			continue
		}
		if entry.HlinkURLs == nil {
			entry.HlinkURLs = make(map[string]string)
		}
		entry.HlinkURLs[normalizeRelID(r.ID)] = r.Target
	}
}

func loadSlideOleObjectParts(pkg *opc.Package, slidePartName string, entry *SlideEntry) {
	relIDs := collectOleObjectRelIDs(entry)
	if len(relIDs) == 0 {
		return
	}
	slideRels, ok, err := newPackageNavigator(pkg).relationships(slidePartName)
	if err != nil || !ok {
		return
	}
	entry.OleObjectParts = make(map[string]media.Part, len(relIDs))
	needed := make(map[string]struct{}, len(relIDs))
	for _, id := range relIDs {
		needed[id] = struct{}{}
	}
	for _, r := range slideRels.Relationships {
		if !relationshipTypeIs(r.Type, relTypeOLEObject) && !relationshipTypeIs(r.Type, relTypePackage) {
			continue
		}
		id := normalizeRelID(r.ID)
		if _, ok := needed[id]; !ok {
			continue
		}
		if r.IsExternal() {
			if entry.OleObjectLinks == nil {
				entry.OleObjectLinks = make(map[string]string)
			}
			entry.OleObjectLinks[id] = r.Target
			continue
		}
		p, ok := newPackageNavigator(pkg).binaryPart(slidePartName, r)
		if !ok {
			continue
		}
		entry.OleObjectParts[id] = p
	}
}

func collectOleObjectRelIDs(entry *SlideEntry) []string {
	if entry == nil || entry.Slide == nil {
		return nil
	}
	var ids []string
	for _, ole := range entry.Slide.CSld.SpTree.OleObjectFrames {
		if ole.RelID != "" {
			ids = append(ids, normalizeRelID(ole.RelID))
		}
	}
	var walkGroups func(*GroupShape)
	walkGroups = func(g *GroupShape) {
		if g == nil {
			return
		}
		for _, ole := range g.OleObjectFrames {
			if ole.RelID != "" {
				ids = append(ids, normalizeRelID(ole.RelID))
			}
		}
		for i := range g.GroupShapes {
			walkGroups(&g.GroupShapes[i])
		}
	}
	for i := range entry.Slide.CSld.SpTree.GroupShapes {
		walkGroups(&entry.Slide.CSld.SpTree.GroupShapes[i])
	}
	return ids
}

func loadSlideComments(pkg *opc.Package, slidePartName string, entry *SlideEntry, out *Package) {
	slideRels, ok, err := newPackageNavigator(pkg).relationships(slidePartName)
	if err != nil || !ok {
		return
	}
	for _, r := range slideRels.Relationships {
		if !relationshipTypeIs(r.Type, relTypeComments) || r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(slidePartName, r.Target)
		var cl CommentList
		found, err := newPackageNavigator(pkg).decodeXML(partName, &cl)
		if !found {
			continue
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityInfo, "pptx/malformed-comments", fmt.Sprintf("comments part could not be parsed and was skipped: %v", err), partName, err)
			continue
		}
		for i := range cl.Comments {
			cm := &cl.Comments[i]
			pc := &PptxComment{
				AuthorID: cm.AuthorID,
				Index:    cm.Index,
				DateTime: cm.DateTime,
				Text:     cm.Text,
			}
			if cm.Pos != nil {
				pc.X = cm.Pos.X
				pc.Y = cm.Pos.Y
			}
			entry.Comments = append(entry.Comments, pc)
		}
		break
	}
}

func loadCommentAuthors(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, r := range byType[relTypeCommentAuthors] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		var cal CommentAuthorList
		found, err := newPackageNavigator(pkg).decodeXML(partName, &cal)
		if !found {
			continue
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityInfo, "pptx/malformed-comment-authors", fmt.Sprintf("comment authors part could not be parsed and was skipped: %v", err), partName, err)
			continue
		}
		if len(cal.Authors) > 0 {
			out.CommentAuthors = make(map[int]CommentAuthor, len(cal.Authors))
			for _, a := range cal.Authors {
				out.CommentAuthors[a.ID] = CommentAuthor{
					ID:       a.ID,
					Name:     a.Name,
					Initials: a.Initials,
				}
			}
		}
		break
	}
}

func loadSlideMedia(pkg *opc.Package, slidePartName string, entry *SlideEntry) {
	entry.MediaParts = make(map[string]media.Part)
	slideRels, ok, err := newPackageNavigator(pkg).relationships(slidePartName)
	if err != nil || !ok {
		return
	}
	for _, r := range slideRels.Relationships {
		loadRelationshipMediaPart(pkg, slidePartName, r, entry.MediaParts)
	}
}

func loadLayoutMedia(pkg *opc.Package, layoutPartName string, entry *SlideEntry) {
	entry.LayoutMediaParts = make(map[string]media.Part)
	layoutRels, ok, err := newPackageNavigator(pkg).relationships(layoutPartName)
	if err != nil || !ok {
		return
	}
	for _, r := range layoutRels.Relationships {
		loadRelationshipMediaPart(pkg, layoutPartName, r, entry.LayoutMediaParts)
	}
}

func loadMasterMedia(pkg *opc.Package, masterPartName string, entry *SlideMasterEntry) {
	entry.MediaParts = make(map[string]media.Part)
	masterRels, ok, err := newPackageNavigator(pkg).relationships(masterPartName)
	if err != nil || !ok {
		return
	}
	for _, r := range masterRels.Relationships {
		loadRelationshipMediaPart(pkg, masterPartName, r, entry.MediaParts)
	}
}

func loadSlideLayout(pkg *opc.Package, slidePartName string, entry *SlideEntry, out *Package) error {
	slideRels, ok, err := newPackageNavigator(pkg).relationships(slidePartName)
	if err != nil {
		addPackageLoadDiagnostic(out, diag.SeverityWarning, "pptx/malformed-slide-relationships", fmt.Sprintf("slide relationships could not be parsed; dependent parts may be unavailable: %v", err), slidePartName, err)
		return nil
	}
	if !ok {
		return nil
	}
	for _, r := range slideRels.Relationships {
		if !relationshipTypeIs(r.Type, relTypeSlideLayout) || r.IsExternal() {
			continue
		}
		layoutPartName := rels.ResolveTarget(slidePartName, r.Target)
		entry.LayoutPartName = layoutPartName
		var layout SlideLayout
		found, err := newPackageNavigator(pkg).decodeXML(layoutPartName, &layout)
		if !found {
			break
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityWarning, "pptx/malformed-slide-layout", fmt.Sprintf("slide layout could not be parsed; continuing without it: %v", err), layoutPartName, err)
			break
		}
		layout.PartName = layoutPartName
		layout.ThemeOverride = loadThemeOverrideForPart(pkg, layoutPartName)
		entry.Layout = &layout
		loadLayoutMedia(pkg, layoutPartName, entry)

		// Follow the layout's relationship to its slide master (POI XSLF parity).
		layoutRels, ok, err := newPackageNavigator(pkg).relationships(layoutPartName)
		if ok && err == nil {
			for _, lr := range layoutRels.Relationships {
				if relationshipTypeIs(lr.Type, relTypeSlideMaster) && !lr.IsExternal() {
					entry.MasterPartName = rels.ResolveTarget(layoutPartName, lr.Target)
					break
				}
			}
		}
		break
	}
	return nil
}

func loadSlideMasters(pkg *opc.Package, mainPartName string, masterIDs []IDRef, byID map[string]rels.Relationship, out *Package) error {
	for _, ref := range masterIDs {
		relID := normalizeRelID(ref.RelID)
		entry := &SlideMasterEntry{RelID: relID}

		r, ok := byID[relID]
		if !ok || !relationshipTypeIs(r.Type, relTypeSlideMaster) || r.IsExternal() {
			out.SlideMasters = append(out.SlideMasters, entry)
			continue
		}

		partName := rels.ResolveTarget(mainPartName, r.Target)
		entry.PartName = partName

		var master SlideMaster
		found, err := newPackageNavigator(pkg).decodeXML(partName, &master)
		if !found {
			out.SlideMasters = append(out.SlideMasters, entry)
			continue
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityWarning, "pptx/malformed-slide-master", fmt.Sprintf("slide master could not be parsed; continuing without it: %v", err), partName, err)
			out.SlideMasters = append(out.SlideMasters, entry)
			continue
		}
		entry.Master = &master

		if err := loadMasterLayouts(pkg, partName, &master, entry, out); err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityWarning, "pptx/master-layout-load-error", fmt.Sprintf("slide master layouts could not be loaded completely: %v", err), partName, err)
		}
		loadMasterMedia(pkg, partName, entry)

		out.SlideMasters = append(out.SlideMasters, entry)
	}
	return nil
}

func loadMasterLayouts(pkg *opc.Package, masterPartName string, master *SlideMaster, entry *SlideMasterEntry, out *Package) error {
	masterRels, ok, err := newPackageNavigator(pkg).relationships(masterPartName)
	if err != nil {
		addPackageLoadDiagnostic(out, diag.SeverityWarning, "pptx/malformed-master-relationships", fmt.Sprintf("slide master relationships could not be parsed; layouts may be unavailable: %v", err), masterPartName, err)
		return nil
	}
	if !ok {
		return nil
	}
	byID := indexRelsByID(masterRels)

	for _, layoutRef := range master.SlideLayouts {
		relID := normalizeRelID(layoutRef.RelID)
		r, ok := byID[relID]
		if !ok || !relationshipTypeIs(r.Type, relTypeSlideLayout) || r.IsExternal() {
			continue
		}
		layoutPartName := rels.ResolveTarget(masterPartName, r.Target)
		var layout SlideLayout
		found, err := newPackageNavigator(pkg).decodeXML(layoutPartName, &layout)
		if !found {
			continue
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityWarning, "pptx/malformed-master-layout", fmt.Sprintf("master layout could not be parsed and was skipped: %v", err), layoutPartName, err)
			continue
		}
		layout.PartName = layoutPartName
		layout.ThemeOverride = loadThemeOverrideForPart(pkg, layoutPartName)
		entry.Layouts = append(entry.Layouts, &layout)
	}
	return nil
}

func loadThemeOverrideForPart(pkg *opc.Package, sourcePartName string) *ThemeOverride {
	partName, ok := themeOverridePartName(pkg, sourcePartName)
	if !ok {
		return nil
	}
	var override ThemeOverride
	found, err := newPackageNavigator(pkg).decodeXML(partName, &override)
	if !found || err != nil {
		return nil
	}
	return &override
}

func themeOverridePartName(pkg *opc.Package, sourcePartName string) (string, bool) {
	partRels, ok, err := newPackageNavigator(pkg).relationships(sourcePartName)
	if err != nil || !ok {
		return "", false
	}
	for _, r := range partRels.Relationships {
		if !relationshipTypeIs(r.Type, relTypeThemeOverride) || r.IsExternal() {
			continue
		}
		return rels.ResolveTarget(sourcePartName, r.Target), true
	}
	return "", false
}

func loadThemes(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, r := range byType[relTypeTheme] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		var theme Theme
		found, err := newPackageNavigator(pkg).decodeXML(partName, &theme)
		if !found {
			continue
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityInfo, "pptx/malformed-theme", fmt.Sprintf("theme part could not be parsed and was skipped: %v", err), partName, err)
			continue
		}
		out.Themes[partName] = &theme
	}
}

func loadTableStyles(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, r := range byType[relTypeTableStyles] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		var styles TableStyleList
		found, err := newPackageNavigator(pkg).decodeXML(partName, &styles)
		if !found {
			continue
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityInfo, "pptx/malformed-table-styles", fmt.Sprintf("table styles part could not be parsed and was skipped: %v", err), partName, err)
			continue
		}
		out.TableStyles = &styles
		return
	}
}

func loadNotesMaster(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, r := range byType[relTypeNotesMaster] {
		if r.IsExternal() {
			continue
		}
		partName := rels.ResolveTarget(mainPartName, r.Target)
		var notesMaster NotesMaster
		found, err := newPackageNavigator(pkg).decodeXML(partName, &notesMaster)
		if !found {
			continue
		}
		if err != nil {
			addPackageLoadDiagnostic(out, diag.SeverityInfo, "pptx/malformed-notes-master", fmt.Sprintf("notes master could not be parsed and was skipped: %v", err), partName, err)
			continue
		}
		out.NotesMaster = &notesMaster
		return
	}
}

func loadEmbeddedFonts(pkg *opc.Package, mainPartName string, decls []EmbeddedFontDecl, byID map[string]rels.Relationship, out *Package) {
	for _, decl := range decls {
		embedded := fontdata.LoadEmbeddedFont(pkg, mainPartName, decl.Font.Typeface, fontdata.StyleRefs{
			Regular:    refID(decl.Regular),
			Bold:       refID(decl.Bold),
			Italic:     refID(decl.Italic),
			BoldItalic: refID(decl.BoldItalic),
		}, fontdata.StyleKeys{
			Regular:    keyID(decl.Regular),
			Bold:       keyID(decl.Bold),
			Italic:     keyID(decl.Italic),
			BoldItalic: keyID(decl.BoldItalic),
		}, byID)
		if embedded != nil {
			out.EmbeddedFonts = append(out.EmbeddedFonts, embedded)
		}
	}
}

func keyID(ref *EmbeddedFontStyleRef) string {
	if ref == nil {
		return ""
	}
	return ref.FontKey
}

func refID(ref *EmbeddedFontStyleRef) string {
	if ref == nil {
		return ""
	}
	return normalizeRelID(ref.RelID)
}

func loadMediaParts(pkg *opc.Package, mainPartName string, byType map[string][]rels.Relationship, out *Package) {
	for _, relsByType := range byType {
		for _, r := range relsByType {
			loadRelationshipMediaPart(pkg, mainPartName, r, out.MediaParts)
		}
	}
}

func loadRelationshipMediaPart(pkg *opc.Package, sourcePartName string, rel rels.Relationship, dst map[string]media.Part) {
	if dst == nil || rel.IsExternal() {
		return
	}
	p, ok := newPackageNavigator(pkg).binaryPart(sourcePartName, rel)
	if !ok || !isPresentationMediaPart(p.ContentType, p.Name) {
		return
	}
	dst[normalizeRelID(rel.ID)] = p
}

func isPresentationMediaPart(contentType, partName string) bool {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if strings.HasPrefix(baseType, "image/") || strings.HasPrefix(baseType, "audio/") || strings.HasPrefix(baseType, "video/") {
		return true
	}
	partName = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(partName), "\\", "/"))
	return strings.Contains(partName, "/media/")
}

func indexRelsByID(r *rels.Document) map[string]rels.Relationship {
	m := make(map[string]rels.Relationship, len(r.Relationships))
	for _, rel := range r.Relationships {
		m[normalizeRelID(rel.ID)] = rel
	}
	return m
}

func indexRelsByType(r *rels.Document) map[string][]rels.Relationship {
	m := make(map[string][]rels.Relationship)
	for _, rel := range r.Relationships {
		relType := canonicalRelationshipType(rel.Type)
		m[relType] = append(m[relType], rel)
	}
	return m
}

func normalizeRelID(id string) string {
	return id
}
