// Package legacy implements lossless parsing and write-back for legacy binary
// Microsoft Word .doc documents.
//
// ParseDocLossless and ParseDocBytesLossless retain binary provenance in the
// shared word/ir model. WriteDocument emits Word 97-2003 binary documents from
// that IR. Most applications should prefer the high-level office/pkg/word
// facade unless they specifically need binary SPRM/FIB/OfficeArt provenance.
package legacy
