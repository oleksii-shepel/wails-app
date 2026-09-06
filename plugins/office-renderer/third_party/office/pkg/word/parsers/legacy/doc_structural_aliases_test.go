package legacy

import docbinary "office/pkg/word/parsers/doc/binary"

// Historical FIB names are retained only for legacy compatibility fixtures.
// Production structural decoding uses doc/binary directly.
const (
	legacyFibPlcSpaMom = docbinary.FibPlcSpaMom
	legacyFibPlcSpaHdr = docbinary.FibPlcSpaHdr
)
