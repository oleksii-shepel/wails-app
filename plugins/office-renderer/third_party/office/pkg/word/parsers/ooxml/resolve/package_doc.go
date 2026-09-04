// Package resolve projects typed WordprocessingML parts into the stable
// format-neutral word/ir model.
//
// Its exported document/content types are compatibility aliases of word/ir.
// New callers should normally use office/pkg/word.Open for the common path or
// import word/ir directly when they need the semantic model itself.
package resolve
