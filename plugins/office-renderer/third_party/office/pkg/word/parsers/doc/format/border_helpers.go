package docformat

import wordir "office/pkg/word/ir"

// parseBRC80MayBeNil decodes a compact BRC80 only when it represents a visible
// border. Zero/none border records are structural defaults and must not
// materialize an IR border side.
func parseBRC80MayBeNil(op []byte) *wordir.BorderSide {
	if len(op) < 4 {
		return nil
	}
	// BRC80 byte 1 carries brcType. 0 means nil/none; 0xFF is also used by
	// legacy structures as an unset sentinel.
	if op[1] == 0 || op[1] == 0xFF {
		return nil
	}
	return parseBRC80(op)
}
