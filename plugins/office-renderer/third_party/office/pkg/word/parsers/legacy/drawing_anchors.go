package legacy

import (
	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	docofficeart "office/pkg/word/parsers/doc/officeart"
	word97 "office/pkg/word/parsers/doc/word97"
)

const legacyDrawingAnchorChar = '\b'

// Compatibility name only. The structural record is canonical DOC metadata.
type legacyFloatingDrawingAnchor = docbinary.FloatingAnchor

func processLegacyDrawingAnchors(runs []docRun, wordStream, tableStream []byte, fibBase, cbRgFcLcb, mainTextLen int, images []*wordir.Image, additionalOfficeArtStreams ...[]byte) []docRun {
	if len(runs) == 0 || len(tableStream) == 0 {
		return runs
	}
	observed := legacyObservedDrawingAnchorCPs(runs)
	if len(observed) == 0 {
		return runs
	}
	anchors := loadLegacyFloatingDrawingAnchors(wordStream, tableStream, fibBase, cbRgFcLcb, mainTextLen, observed)
	if len(anchors) == 0 {
		return runs
	}
	officeArtStreams := make([][]byte, 0, 2+len(additionalOfficeArtStreams))
	officeArtStreams = append(officeArtStreams, wordStream, tableStream)
	officeArtStreams = append(officeArtStreams, additionalOfficeArtStreams...)
	catalog := docofficeart.DecodeStreams(officeArtStreams...)
	resolved := make(map[int]word97.Drawing, len(anchors))
	imageIndex := 0
	reuseSingle := len(images) == 1
	for _, cp := range observed {
		anchor, ok := anchors[cp]
		if !ok {
			continue
		}
		template := legacySelectDrawingImage(images, imageIndex, reuseSingle)
		if template != nil {
			image := *template
			if docbinary.ApplyAnchorToImage(&image, anchor) {
				catalog.ApplyImageVisual(anchor.Shape, &image)
				resolved[cp] = word97.Drawing{Image: &image}
				if !reuseSingle {
					imageIndex++
				}
			}
			continue
		}
		if shape := catalog.ProjectAnchoredShape(anchor); shape != nil {
			resolved[cp] = word97.Drawing{Shape: shape}
		}
	}
	return applyResolvedWord97Drawings(runs, resolved)
}

func containsLegacyDrawingAnchorRune(text string) bool {
	for _, ch := range text {
		if ch == legacyDrawingAnchorChar {
			return true
		}
	}
	return false
}

func legacyObservedDrawingAnchorCPs(runs []docRun) []int {
	var cps []int
	for _, r := range runs {
		if r.Kind != "" || !r.Spec || r.Text == "" {
			continue
		}
		cp := r.StartCP
		for _, ch := range r.Text {
			if ch == legacyDrawingAnchorChar {
				cps = append(cps, cp)
			}
			cp++
		}
	}
	return cps
}

func loadLegacyFloatingDrawingAnchors(wordStream, tableStream []byte, fibBase, cbRgFcLcb, mainTextLen int, observedCPs []int) map[int]legacyFloatingDrawingAnchor {
	anchors := docbinary.FloatingAnchorsAt(wordStream, tableStream, fibBase, cbRgFcLcb, docbinary.FibPlcSpaMom, 0, mainTextLen, false)
	if len(anchors) == 0 {
		anchors = docbinary.RecoverFloatingAnchors(wordStream, tableStream, fibBase, cbRgFcLcb, mainTextLen, docbinary.FibPlcSpaMom, observedCPs)
	}
	return docbinary.AnchorMapByCP(anchors)
}

// Compatibility wrapper retained for lossless tests and malformed-file probes.
// Decoding itself is canonical.
func parseLegacyPlcfSpa(blob []byte, mainTextLen int) ([]legacyFloatingDrawingAnchor, bool) {
	return docbinary.ParsePlcfSpa(blob, mainTextLen)
}

// Compatibility wrappers keep old lossless callers source-compatible without
// preserving a second implementation of FSPA placement semantics.
type legacyAnchorPlacement = docbinary.AnchorPlacement

func legacyPlacementFromAnchor(anchor legacyFloatingDrawingAnchor) (legacyAnchorPlacement, bool) {
	return docbinary.PlacementFromAnchor(anchor)
}

func legacyAnchorRelative(anchor legacyFloatingDrawingAnchor, vertical bool) string {
	return docbinary.AnchorRelative(anchor, vertical)
}

func legacySelectDrawingImage(images []*wordir.Image, index int, reuseSingle bool) *wordir.Image {
	if len(images) == 0 {
		return nil
	}
	if reuseSingle {
		return images[0]
	}
	if index < 0 || index >= len(images) {
		return nil
	}
	return images[index]
}

func legacyWrapTypeFromSpaFlags(flags uint16) string {
	return docbinary.WrapTypeFromSpaFlags(flags)
}

func legacyWrapSideFromSpaFlags(flags uint16) string {
	return docbinary.WrapSideFromSpaFlags(flags)
}

func legacyAnchorRelativeFromSpaFlags(flags uint16, vertical bool) string {
	anchor := legacyFloatingDrawingAnchor{
		Flags:     flags,
		BX:        uint8((flags >> 1) & 0x03),
		BY:        uint8((flags >> 3) & 0x03),
		RcaSimple: flags&0x2000 != 0,
	}
	return docbinary.AnchorRelative(anchor, vertical)
}

func legacyShapeUsesAnchorLineFlow(anchor legacyFloatingDrawingAnchor) bool {
	return docbinary.UsesAnchorLineFlow(anchor)
}

func loadLegacyFloatingDrawingAnchorsAtIndex(
	wordStream, tableStream []byte,
	fibBase, cbRgFcLcb, index, cpBase, storyTextLen int,
	_ []int, header bool,
) map[int]legacyFloatingDrawingAnchor {
	return docbinary.AnchorMapByCP(docbinary.FloatingAnchorsAt(
		wordStream,
		tableStream,
		fibBase,
		cbRgFcLcb,
		index,
		cpBase,
		storyTextLen,
		header,
	))
}
