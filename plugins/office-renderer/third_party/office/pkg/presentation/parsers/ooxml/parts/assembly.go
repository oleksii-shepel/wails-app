package parts

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

// packageAssembler owns OPC navigation and the assembly order for a typed
// PresentationML package. Individual part loaders should not rediscover the
// presentation root or decide top-level traversal order.
type packageAssembler struct {
	opc      *opc.Package
	nav      packageNavigator
	mainName string
	main     *Presentation
	mainRels *rels.Document
	byID     map[string]rels.Relationship
	byType   map[string][]rels.Relationship
	out      *Package
}

func newPackageAssembler(pkg *opc.Package) (*packageAssembler, error) {
	if pkg == nil {
		return nil, fmt.Errorf("presentation parts: package cannot be nil")
	}
	if pkg.PackageType() != opc.PackageTypePresentation {
		return nil, fmt.Errorf("presentation parts: expected %q package, got %q", opc.PackageTypePresentation, pkg.PackageType())
	}

	nav := newPackageNavigator(pkg)
	mainName, err := findMainPart(nav)
	if err != nil {
		return nil, err
	}
	var main Presentation
	found, err := nav.decodeXML(mainName, &main)
	if !found {
		return nil, fmt.Errorf("presentation parts: main part %q not found", mainName)
	}
	if err != nil {
		return nil, fmt.Errorf("presentation parts: parse %s: %w", mainName, err)
	}

	a := &packageAssembler{
		opc:      pkg,
		nav:      nav,
		mainName: mainName,
		main:     &main,
		out: &Package{
			MainPartName: mainName,
			Presentation: &main,
			Themes:       make(map[string]*Theme),
			MediaParts:   make(map[string]media.Part),
		},
	}
	a.preserveParts()

	doc, ok, err := nav.relationships(mainName)
	if err != nil {
		return nil, fmt.Errorf("presentation parts: relationships for %s: %w", mainName, err)
	}
	if ok {
		a.mainRels = doc
		a.byID = indexRelsByID(doc)
		a.byType = indexRelsByType(doc)
	} else {
		a.byID = map[string]rels.Relationship{}
		a.byType = map[string][]rels.Relationship{}
	}
	return a, nil
}

func (a *packageAssembler) preserveParts() {
	for _, part := range a.opc.Parts() {
		if strings.EqualFold(part.Name, "[content_types].xml") {
			continue
		}
		a.out.PreservedParts = append(a.out.PreservedParts, PreservedPart{
			Name: part.Name, ContentType: part.ContentType, Data: append([]byte(nil), part.Data...),
		})
	}
}

func (a *packageAssembler) assemble() (*Package, error) {
	// A missing presentation relationship part is valid enough for a partial
	// typed package: retain the parsed main part and preserved OPC payloads.
	if a.mainRels == nil {
		return a.out, nil
	}

	loadEmbeddedFonts(a.opc, a.mainName, a.main.EmbeddedFontDecls, a.byID, a.out)
	if err := loadSlides(a.opc, a.mainName, a.main.SlideIDs, a.byID, a.out); err != nil {
		return nil, err
	}
	if err := loadSlideMasters(a.opc, a.mainName, a.main.SlideMasterIDs, a.byID, a.out); err != nil {
		return nil, err
	}
	loadNotesMaster(a.opc, a.mainName, a.byType, a.out)
	loadThemes(a.opc, a.mainName, a.byType, a.out)
	loadTableStyles(a.opc, a.mainName, a.byType, a.out)
	loadMediaParts(a.opc, a.mainName, a.byType, a.out)
	loadCommentAuthors(a.opc, a.mainName, a.byType, a.out)

	a.out.Diagnostics = append(a.out.Diagnostics, collectPackageDiagnostics(a.out)...)
	return a.out, nil
}
