package legacy

import docbinary "office/pkg/word/parsers/doc/binary"

const legacyDrawingAnchorChar = '\b'

// Compatibility name only. The structural record is canonical DOC metadata.
type legacyFloatingDrawingAnchor = docbinary.FloatingAnchor

func containsLegacyDrawingAnchorRune(text string) bool {
	for _, ch := range text {
		if ch == legacyDrawingAnchorChar {
			return true
		}
	}
	return false
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
