package crypt

import (
	"encoding/binary"
	"fmt"
)

// CryptoAPIEncryptionInfo holds the parsed fields from a CryptoAPI
// EncryptionHeader + EncryptionVerifier block.
type CryptoAPIEncryptionInfo struct {
	VersionMajor          uint16
	VersionMinor          uint16
	Flags                 uint32
	HashAlg               HashAlgorithm
	KeyBits               int
	Salt                  []byte
	EncryptedVerifier     []byte
	EncryptedVerifierHash []byte
	VerifierHashSize      int
}

// ParseCryptoAPIEncryptionInfo parses a CryptoAPI EncryptionInfo structure
// from a byte slice.  The slice must start with versionMajor/versionMinor.
// It returns the parsed info or an error.
func ParseCryptoAPIEncryptionInfo(data []byte) (*CryptoAPIEncryptionInfo, error) {
	if len(data) < 14 {
		return nil, fmt.Errorf("crypt: CryptoAPI encryption info too short (%d bytes)", len(data))
	}

	versionMajor := binary.LittleEndian.Uint16(data[0:2])
	versionMinor := binary.LittleEndian.Uint16(data[2:4])

	if versionMinor != 2 || versionMajor < 2 || versionMajor > 4 {
		return nil, fmt.Errorf("crypt: unsupported encryption version %d.%d", versionMajor, versionMinor)
	}

	encFlags := binary.LittleEndian.Uint32(data[4:8])
	if encFlags&0x20 != 0 {
		return nil, fmt.Errorf("crypt: CryptoAPI AES encryption is not supported")
	}

	headerSize := int(binary.LittleEndian.Uint32(data[8:12]))
	headerStart := 12
	headerEnd := headerStart + headerSize
	if headerSize <= 0 || headerEnd > len(data) {
		return nil, fmt.Errorf("crypt: CryptoAPI header exceeds encryption info length")
	}

	cryptoHeader := data[headerStart:headerEnd]
	if len(cryptoHeader) < 32 {
		return nil, fmt.Errorf("crypt: CryptoAPI header too short (%d bytes)", len(cryptoHeader))
	}

	cipherAlg := binary.LittleEndian.Uint32(cryptoHeader[8:12])
	if cipherAlg != 0x6801 {
		return nil, fmt.Errorf("crypt: unsupported CryptoAPI cipher 0x%04X", cipherAlg)
	}

	hashAlgID := binary.LittleEndian.Uint32(cryptoHeader[12:16])
	var hashAlg HashAlgorithm
	switch hashAlgID {
	case 0x8003:
		hashAlg = HashMD5
	case 0x8004:
		hashAlg = HashSHA1
	default:
		return nil, fmt.Errorf("crypt: unsupported CryptoAPI hash algorithm 0x%04X", hashAlgID)
	}

	keyBits := int(binary.LittleEndian.Uint32(cryptoHeader[16:20]))
	if keyBits == 0 {
		keyBits = 40
	}

	// Skip CSP name (null-terminated UTF-16LE) if present.
	// POI checks if the next 4 bytes == 16 to detect absent CSP name.
	verifierStart := headerEnd
	if verifierStart+4 <= len(data) {
		check := binary.LittleEndian.Uint32(data[verifierStart : verifierStart+4])
		if check != 16 {
			// CSP name present; skip past null terminator
			for verifierStart < len(data)-1 {
				if data[verifierStart] == 0 && data[verifierStart+1] == 0 {
					verifierStart += 2
					break
				}
				verifierStart += 2
			}
		}
	}

	if verifierStart+40 > len(data) {
		return nil, fmt.Errorf("crypt: CryptoAPI verifier truncated")
	}
	saltSize := int(binary.LittleEndian.Uint32(data[verifierStart : verifierStart+4]))
	if saltSize != 16 {
		return nil, fmt.Errorf("crypt: invalid CryptoAPI verifier salt size %d", saltSize)
	}
	encVerifierStart := verifierStart + 4 + saltSize
	encVerifierEnd := encVerifierStart + 16
	hashSizeOffset := encVerifierEnd
	verifierHashSize := int(binary.LittleEndian.Uint32(data[hashSizeOffset : hashSizeOffset+4]))
	encVerifierHashStart := hashSizeOffset + 4
	encVerifierHashEnd := encVerifierHashStart + 20
	if encVerifierHashEnd > len(data) {
		return nil, fmt.Errorf("crypt: CryptoAPI verifier hash truncated")
	}

	return &CryptoAPIEncryptionInfo{
		VersionMajor:          versionMajor,
		VersionMinor:          versionMinor,
		Flags:                 encFlags,
		HashAlg:               hashAlg,
		KeyBits:               keyBits,
		Salt:                  append([]byte(nil), data[verifierStart+4:encVerifierStart]...),
		EncryptedVerifier:     append([]byte(nil), data[encVerifierStart:encVerifierEnd]...),
		EncryptedVerifierHash: append([]byte(nil), data[encVerifierHashStart:encVerifierHashEnd]...),
		VerifierHashSize:      verifierHashSize,
	}, nil
}
