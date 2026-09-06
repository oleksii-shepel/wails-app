package legacy

import (
	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

// mergeLegacyStreamPreservationMeta is now only a compatibility adapter. The
// OfficeMeta stream is decoded canonically by parsers/doc/word97; this bridge
// projects the decoded values into the temporary legacy context carrier.
func mergeLegacyStreamPreservationMeta(f *cfb.File, core *legacyDocStreamCore) {
	if f == nil || core == nil {
		return
	}
	meta := word97.LoadPreservation(f)
	if meta == nil {
		return
	}

	if meta.FootnoteSeparator != nil {
		core.FootnoteSeparator = *meta.FootnoteSeparator
	}
	if meta.FootnoteContinuationSep != nil {
		core.FootnoteContinuationSep = *meta.FootnoteContinuationSep
	}
	if meta.FootnoteContinuationNotice != nil {
		core.FootnoteContinuationNotice = *meta.FootnoteContinuationNotice
	}
	if meta.EndnoteSeparator != nil {
		core.EndnoteSeparator = *meta.EndnoteSeparator
	}
	if meta.EndnoteContinuationSep != nil {
		core.EndnoteContinuationSep = *meta.EndnoteContinuationSep
	}
	if meta.EndnoteContinuationNotice != nil {
		core.EndnoteContinuationNotice = *meta.EndnoteContinuationNotice
	}
	mergeLegacyMetaStringStories(&core.Headers, meta.Headers)
	mergeLegacyMetaStringStories(&core.Footers, meta.Footers)
	mergeLegacyMetaIntStories(&core.Footnotes, meta.Footnotes)
	mergeLegacyMetaIntStories(&core.Endnotes, meta.Endnotes)
	mergeLegacyMetaIntStories(&core.Comments, meta.Comments)

	n := len(meta.SectionHeaders)
	if len(meta.SectionFooters) > n {
		n = len(meta.SectionFooters)
	}
	if n > len(core.Sections) {
		core.Sections = append(core.Sections, make([]wordir.Section, n-len(core.Sections))...)
	}
	for i := 0; i < n; i++ {
		if i < len(meta.SectionHeaders) && meta.SectionHeaders[i] != nil {
			core.Sections[i].Headers = append([]wordir.Reference(nil), meta.SectionHeaders[i]...)
		}
		if i < len(meta.SectionFooters) && meta.SectionFooters[i] != nil {
			core.Sections[i].Footers = append([]wordir.Reference(nil), meta.SectionFooters[i]...)
		}
	}
}
