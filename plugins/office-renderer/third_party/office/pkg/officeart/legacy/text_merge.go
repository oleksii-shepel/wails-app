package legacy

// MergeTextProperties overlays only properties explicitly present in incoming.
func MergeTextProperties(base, incoming TextProperties) TextProperties {
	if incoming.HasLeft {
		base.HasLeft = true
		base.Insets.Left = incoming.Insets.Left
	}
	if incoming.HasTop {
		base.HasTop = true
		base.Insets.Top = incoming.Insets.Top
	}
	if incoming.HasRight {
		base.HasRight = true
		base.Insets.Right = incoming.Insets.Right
	}
	if incoming.HasBottom {
		base.HasBottom = true
		base.Insets.Bottom = incoming.Insets.Bottom
	}
	if incoming.HasWrap {
		base.HasWrap = true
		base.Wrap = incoming.Wrap
	}
	if incoming.HasAnchor {
		base.HasAnchor = true
		base.Anchor = incoming.Anchor
		base.AnchorCenter = incoming.AnchorCenter
	}
	if incoming.HasVert {
		base.HasVert = true
		base.Vert = incoming.Vert
	}
	if incoming.HasAutoFit {
		base.HasAutoFit = true
		base.AutoFit = incoming.AutoFit
	}
	return base
}
