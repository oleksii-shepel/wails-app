package parts

import (
	"bytes"
	"fmt"
	"path"

	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
)

// writeSlideSmartArt emits the diagram parts for a slide, creates slide-level
// relationships for them, and rewrites the hosting p:graphicFrame elements so
// their dgm:relIds attributes point at the new relationship IDs.
//
// usedIDs contains relationship IDs already reserved on this slide (layout,
// media, charts, hyperlinks, comments, notes). The returned relationships can
// be appended to the slide's .rels file.
func writeSlideSmartArt(entry *SlideEntry, rm *relationshipManager, opcPkg *opc.Package, usedIDs map[string]bool) []rels.Relationship {
	if entry == nil || entry.Slide == nil || len(entry.SmartArtParts) == 0 {
		return nil
	}

	partByDataRel := make(map[string]*SmartArtPart, len(entry.SmartArtParts))
	for _, sap := range entry.SmartArtParts {
		partByDataRel[normalizeRelID(sap.RelID)] = sap
	}

	// Assign fresh slide-level relationship IDs for each diagram part and write
	// the parts into the package. oldDataRel -> {dm,lo,qs,cs} new IDs.
	newIDsByDataRel := make(map[string]map[string]string, len(entry.SmartArtParts))
	drawingPartNameByDataRel := make(map[string]string)
	drawingRelIDByDataRel := make(map[string]string)
	for _, sap := range entry.SmartArtParts {
		result := writeSmartArtParts(sap, entry.PartName, rm, opcPkg, usedIDs)
		newIDsByDataRel[normalizeRelID(sap.RelID)] = result.IDs
		if result.DrawingPartName != "" {
			drawingPartNameByDataRel[normalizeRelID(sap.RelID)] = result.DrawingPartName
			drawingRelIDByDataRel[normalizeRelID(sap.RelID)] = result.DrawingRelID
		}
	}

	// Rewrite dgm:relIds attributes in every hosting graphicFrame.
	rewriteSmartArtFrameRelIDs(&entry.Slide.CSld.SpTree, partByDataRel, newIDsByDataRel)

	// Build slide-level relationships.
	var slideRels []rels.Relationship
	for _, sap := range entry.SmartArtParts {
		dataRel := normalizeRelID(sap.RelID)
		newIDs, ok := newIDsByDataRel[dataRel]
		if !ok {
			continue
		}
		for _, key := range []string{"dm", "lo", "qs", "cs"} {
			newID, ok := newIDs[key]
			if !ok {
				continue
			}
			relType := smartArtRelTypeForKey(key)
			targetPartName := smartArtPartNameForKey(sap, key)
			slideRels = append(slideRels, rels.Relationship{
				ID:     newID,
				Type:   relType,
				Target: makeRelTarget(entry.PartName, targetPartName),
			})
		}
		if drawingPartName, ok := drawingPartNameByDataRel[dataRel]; ok {
			slideRels = append(slideRels, rels.Relationship{
				ID:     drawingRelIDByDataRel[dataRel],
				Type:   relTypeDiagramDrawing,
				Target: makeRelTarget(entry.PartName, drawingPartName),
			})
		}
	}
	return slideRels
}

// smartArtWriteResult holds the relationship IDs and optional drawing part name
// produced when writing a SmartArt diagram's parts.
type smartArtWriteResult struct {
	IDs             map[string]string
	DrawingPartName string
	DrawingRelID    string
}

// writeSmartArtParts writes a data model and its optional related parts into the
// OPC package. It returns a map from dgm:relIds key (dm/lo/qs/cs) to the new
// slide-level relationship ID.
func writeSmartArtParts(sap *SmartArtPart, slidePartName string, rm *relationshipManager, opcPkg *opc.Package, usedIDs map[string]bool) smartArtWriteResult {
	result := smartArtWriteResult{IDs: map[string]string{}}

	addRel := func(key, relType, targetPartName, contentType string, data []byte) string {
		if len(data) == 0 && key != "dm" {
			return ""
		}
		id := nextUniqueRelID(rm, usedIDs)
		opcPkg.AddPart(targetPartName, contentType, prependXMLHeader(data))
		result.IDs[key] = id
		return id
	}

	addRel("dm", relTypeDiagramData, sap.PartName, contentTypeDiagramData, sap.DataModelXML)

	if len(sap.LayoutXML) > 0 {
		name := relatedSmartArtPartName(sap.PartName, "layout")
		addRel("lo", relTypeDiagramLayout, name, contentTypeDiagramLayout, sap.LayoutXML)
	}
	if len(sap.StyleXML) > 0 {
		name := relatedSmartArtPartName(sap.PartName, "style")
		addRel("qs", relTypeDiagramStyle, name, contentTypeDiagramStyle, sap.StyleXML)
	}
	if len(sap.ColorsXML) > 0 {
		name := relatedSmartArtPartName(sap.PartName, "colors")
		addRel("cs", relTypeDiagramColors, name, contentTypeDiagramColors, sap.ColorsXML)
	}
	if len(sap.QuickStyleXML) > 0 {
		name := relatedSmartArtPartName(sap.PartName, "quickStyle")
		addRel("qs", relTypeDiagramQuickStyle, name, contentTypeDiagramQuickStyle, sap.QuickStyleXML)
	}

	// Write the optional pre-rendered drawing part.
	if len(sap.DrawingXML) > 0 {
		name := relatedSmartArtPartName(sap.PartName, "drawing")
		result.DrawingRelID = nextUniqueRelID(rm, usedIDs)
		result.DrawingPartName = name
		opcPkg.AddPart(name, contentTypeDiagramDrawing, prependXMLHeader(sap.DrawingXML))
	}

	// Write the data model's own relationships to its related parts.
	writeSmartArtDataRels(sap, result.IDs, opcPkg)

	return result
}

// writeSmartArtDataRels creates the .rels file for a diagram data part so the
// data model can locate its layout, style, colors, and quick-style parts.
func writeSmartArtDataRels(sap *SmartArtPart, newIDs map[string]string, opcPkg *opc.Package) {
	var dataRels []rels.Relationship
	add := func(key, relType, suffix string) {
		if _, ok := newIDs[key]; !ok {
			return
		}
		targetName := relatedSmartArtPartName(sap.PartName, suffix)
		dataRels = append(dataRels, rels.Relationship{
			ID:     fmt.Sprintf("rId%s", key),
			Type:   relType,
			Target: makeRelTarget(sap.PartName, targetName),
		})
	}
	add("lo", relTypeDiagramLayout, "layout")
	add("cs", relTypeDiagramColors, "colors")
	if len(sap.QuickStyleXML) > 0 {
		add("qs", relTypeDiagramQuickStyle, "quickStyle")
	} else if len(sap.StyleXML) > 0 {
		add("qs", relTypeDiagramStyle, "style")
	}

	if len(dataRels) > 0 {
		relsPath := rels.RelationshipsPartName(sap.PartName)
		relsData, err := marshalRels(rels.Document{Relationships: dataRels})
		if err == nil {
			opcPkg.AddPart(relsPath, "application/vnd.openxmlformats-package.relationships+xml", relsData)
		}
	}
}

// rewriteSmartArtFrameRelIDs updates the raw p:graphicFrame XML for every
// SmartArt frame so its dgm:relIds attributes reference the new slide-level
// relationship IDs.
func rewriteSmartArtFrameRelIDs(tree *ShapeTree, partByDataRel map[string]*SmartArtPart, newIDsByDataRel map[string]map[string]string) {
	if tree == nil {
		return
	}
	rewriteSmartArtFrameSliceRelIDs(tree.SmartArtFrames, partByDataRel, newIDsByDataRel)
	for i := range tree.GroupShapes {
		rewriteSmartArtGroupRelIDs(&tree.GroupShapes[i], partByDataRel, newIDsByDataRel)
	}
}

func rewriteSmartArtGroupRelIDs(g *GroupShape, partByDataRel map[string]*SmartArtPart, newIDsByDataRel map[string]map[string]string) {
	if g == nil {
		return
	}
	rewriteSmartArtFrameSliceRelIDs(g.SmartArtFrames, partByDataRel, newIDsByDataRel)
	for i := range g.GroupShapes {
		rewriteSmartArtGroupRelIDs(&g.GroupShapes[i], partByDataRel, newIDsByDataRel)
	}
}

func rewriteSmartArtFrameSliceRelIDs(frames []SmartArtFrame, partByDataRel map[string]*SmartArtPart, newIDsByDataRel map[string]map[string]string) {
	for i := range frames {
		frame := &frames[i]
		oldDataRel, ok := frame.RelIDs["dm"]
		if !ok {
			continue
		}
		if _, ok := partByDataRel[oldDataRel]; !ok {
			continue
		}
		newIDs, ok := newIDsByDataRel[oldDataRel]
		if !ok {
			continue
		}
		frame.RawXML = rewriteSmartArtRelIDAttrs(frame.RawXML, frame.RelIDs, newIDs)
		for k, v := range newIDs {
			frame.RelIDs[k] = v
		}
	}
}

// rewriteSmartArtRelIDAttrs substitutes dgm:relIds attributes in raw XML.
// PowerPoint emits the attributes with an r: prefix (e.g. r:dm="rId2").
func rewriteSmartArtRelIDAttrs(raw []byte, oldIDs, newIDs map[string]string) []byte {
	keys := []string{"dm", "lo", "qs", "cs"}
	for _, key := range keys {
		oldID, ok1 := oldIDs[key]
		newID, ok2 := newIDs[key]
		if !ok1 || !ok2 || oldID == newID {
			continue
		}
		for _, prefix := range []string{"", "r:"} {
			oldAttr := []byte(prefix + key + `="` + oldID + `"`)
			newAttr := []byte(prefix + key + `="` + newID + `"`)
			raw = bytes.ReplaceAll(raw, oldAttr, newAttr)
		}
	}
	return raw
}

// relatedSmartArtPartName derives a related part name from the data model part
// name using PowerPoint's conventional suffixes.
func relatedSmartArtPartName(dataPartName, kind string) string {
	dir := path.Dir(dataPartName)
	base := path.Base(dataPartName)
	ext := path.Ext(base)
	stem := base[:len(base)-len(ext)]
	// Extract a trailing number from the stem (e.g. "data1" -> "1").
	num := ""
	for i := len(stem) - 1; i >= 0; i-- {
		if stem[i] >= '0' && stem[i] <= '9' {
			num = string(stem[i]) + num
		} else {
			break
		}
	}
	if num == "" {
		num = "1"
	}
	switch kind {
	case "layout":
		return dir + "/layout" + num + ext
	case "style":
		return dir + "/style" + num + ext
	case "colors":
		return dir + "/colors" + num + ext
	case "quickStyle":
		return dir + "/quickStyle" + num + ext
	case "drawing":
		return dir + "/drawing" + num + ext
	}
	return dataPartName
}

// smartArtRelTypeForKey maps a dgm:relIds key to its relationship type.
func smartArtRelTypeForKey(key string) string {
	switch key {
	case "dm":
		return relTypeDiagramData
	case "lo":
		return relTypeDiagramLayout
	case "qs":
		return relTypeDiagramQuickStyle
	case "cs":
		return relTypeDiagramColors
	}
	return ""
}

// smartArtPartNameForKey returns the part name for a dgm:relIds key.
func smartArtPartNameForKey(sap *SmartArtPart, key string) string {
	switch key {
	case "dm":
		return sap.PartName
	case "lo":
		return relatedSmartArtPartName(sap.PartName, "layout")
	case "qs":
		if len(sap.QuickStyleXML) > 0 {
			return relatedSmartArtPartName(sap.PartName, "quickStyle")
		}
		return relatedSmartArtPartName(sap.PartName, "style")
	case "cs":
		return relatedSmartArtPartName(sap.PartName, "colors")
	}
	return sap.PartName
}

// nextUniqueRelID returns the next relationship ID from rm that is not already
// in use on this slide.
func nextUniqueRelID(rm *relationshipManager, used map[string]bool) string {
	for {
		id := rm.nextRelID()
		if !used[id] {
			used[id] = true
			return id
		}
	}
}