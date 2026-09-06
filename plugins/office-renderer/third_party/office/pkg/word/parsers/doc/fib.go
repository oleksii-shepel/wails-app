package doc

import "encoding/binary"

const (
	fibSignature = 0xA5EC
	fibWord95    = 0x0065
	fibWord97    = 0x00C1
	flagEncrypted = 0x0100
)

type fibInfo struct {
	Version   uint16
	Encrypted bool
	Valid     bool
}

// classifyFIB reads only the fixed FIB base needed to choose the sequential
// binary DOC decoder. It deliberately does not materialize the WordDocument
// stream or any variable-length FIB payload.
func classifyFIB(header []byte) fibInfo {
	if len(header) < 12 || binary.LittleEndian.Uint16(header[0:2]) != fibSignature {
		return fibInfo{}
	}
	version := binary.LittleEndian.Uint16(header[2:4])
	flags := binary.LittleEndian.Uint16(header[10:12])
	return fibInfo{Version: version, Encrypted: flags&flagEncrypted != 0, Valid: true}
}

func (f fibInfo) word97OrNewer() bool { return f.Valid && f.Version >= fibWord97 }
func (f fibInfo) word95() bool        { return f.Valid && f.Version == fibWord95 }
