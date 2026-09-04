package legacy

import (
	"crypto/rc4"
	"encoding/binary"
	"fmt"

	"office/pkg/ooxml/crypt"
)

// XlsPassword holds the decryption password for legacy XLS files.
// Callers set this before invoking ParseXls, mirroring POI's
// Biff8EncryptionKey.setCurrentUserPassword thread-local pattern.
// An empty string means "no password provided".
var XlsPassword string

const (
	biff8RC4RekeyInterval = 1024

	filePassEncryptionXOR   = 0
	filePassEncryptionOther = 1

	biff8VersionBinaryRC4Major = 1
	biff8VersionBinaryRC4Minor = 1
	biff8VersionCryptoAPIMinor = 2

	cryptoAPICipherRC4        = 0x6801
	cryptoAPIHashMD5          = 0x8003
	cryptoAPIHashSHA1         = 0x8004
	cryptoAPIFlagAES   uint32 = 0x20
)

type biff8EncryptionMode int

const (
	biff8ModeBinaryRC4 biff8EncryptionMode = iota
	biff8ModeCryptoAPI
)

type biff8HashAlgorithm int

const (
	biff8HashMD5 biff8HashAlgorithm = iota
	biff8HashSHA1
)

type biff8EncryptionInfo struct {
	mode                  biff8EncryptionMode
	hashAlg               biff8HashAlgorithm
	keyBits               int
	filePassEnd           int
	salt                  []byte
	encryptedVerifier     []byte
	encryptedVerifierHash []byte
	verifierHashSize      int
}

// maybeDecryptBIFFStream scans the workbook stream for a FilePass record.
// When XOR encryption is detected and the password verifies, it decrypts
// the stream in place and returns nil.  Otherwise it returns the data
// unchanged or an error.
func maybeDecryptBIFFStream(data []byte) ([]byte, error) {
	for pos := 0; pos < len(data)-8; {
		sid := binary.LittleEndian.Uint16(data[pos:])
		size := binary.LittleEndian.Uint16(data[pos+2:])
		if sid == biffFilePass {
			if size < 4 {
				return nil, fmt.Errorf("legacy xls: FilePass record too short (%d bytes)", size)
			}
			encType := binary.LittleEndian.Uint16(data[pos+4:])
			switch encType {
			case filePassEncryptionXOR:
				if size < 6 {
					return nil, fmt.Errorf("legacy xls: XOR FilePass record too short (%d bytes)", size)
				}
				key := binary.LittleEndian.Uint16(data[pos+6:])
				verifier := binary.LittleEndian.Uint16(data[pos+8:])
				if !verifyXORPassword(key, verifier, XlsPassword) {
					return nil, fmt.Errorf("legacy xls: invalid password for XOR-encrypted workbook")
				}
				xorArray := crypt.CreateXorArray1(XlsPassword)
				decryptXORStream(data, pos+4+int(size), xorArray)
				return data, nil
			case filePassEncryptionOther:
				payloadEnd := pos + 4 + int(size)
				if payloadEnd > len(data) {
					return nil, fmt.Errorf("legacy xls: FilePass record exceeds stream length")
				}
				info, err := parseBiff8EncryptionInfo(data[pos+4:payloadEnd], payloadEnd)
				if err != nil {
					return nil, err
				}
				keyMaterial, err := verifyBiff8Password(info, XlsPassword)
				if err != nil {
					return nil, err
				}
				return decryptBiff8Stream(data, info, keyMaterial)
			default:
				return nil, fmt.Errorf("legacy xls: unknown encryption type %d", encType)
			}
		}
		if sid == biffEOF {
			break
		}
		pos += 4 + int(size)
	}
	return data, nil
}

func parseBiff8EncryptionInfo(payload []byte, filePassEnd int) (*biff8EncryptionInfo, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("legacy xls: BIFF8 FilePass record too short (%d bytes)", len(payload))
	}
	versionMajor := binary.LittleEndian.Uint16(payload[2:4])
	versionMinor := binary.LittleEndian.Uint16(payload[4:6])
	switch {
	case versionMajor == biff8VersionBinaryRC4Major && versionMinor == biff8VersionBinaryRC4Minor:
		if len(payload) < 54 {
			return nil, fmt.Errorf("legacy xls: Binary RC4 FilePass record too short (%d bytes)", len(payload))
		}
		return &biff8EncryptionInfo{
			mode:                  biff8ModeBinaryRC4,
			hashAlg:               biff8HashMD5,
			keyBits:               40,
			filePassEnd:           filePassEnd,
			salt:                  append([]byte(nil), payload[6:22]...),
			encryptedVerifier:     append([]byte(nil), payload[22:38]...),
			encryptedVerifierHash: append([]byte(nil), payload[38:54]...),
			verifierHashSize:      16,
		}, nil
	case versionMajor >= 2 && versionMajor <= 4 && versionMinor == biff8VersionCryptoAPIMinor:
		if len(payload) < 14 {
			return nil, fmt.Errorf("legacy xls: CryptoAPI FilePass record too short (%d bytes)", len(payload))
		}
		flags := binary.LittleEndian.Uint32(payload[6:10])
		if flags&cryptoAPIFlagAES != 0 {
			return nil, fmt.Errorf("legacy xls: FilePass AES encryption is not supported")
		}
		headerSize := int(binary.LittleEndian.Uint32(payload[10:14]))
		headerStart := 14
		headerEnd := headerStart + headerSize
		if headerSize <= 0 || headerEnd > len(payload) {
			return nil, fmt.Errorf("legacy xls: CryptoAPI FilePass header exceeds record length")
		}
		header := payload[headerStart:headerEnd]
		if len(header) < 32 {
			return nil, fmt.Errorf("legacy xls: CryptoAPI FilePass header too short (%d bytes)", len(header))
		}
		cipherAlg := binary.LittleEndian.Uint32(header[8:12])
		if cipherAlg != cryptoAPICipherRC4 {
			return nil, fmt.Errorf("legacy xls: unsupported CryptoAPI cipher 0x%04X", cipherAlg)
		}
		hashAlg, err := parseCryptoAPIHashAlgorithm(binary.LittleEndian.Uint32(header[12:16]))
		if err != nil {
			return nil, err
		}
		keyBits := int(binary.LittleEndian.Uint32(header[16:20]))
		if keyBits == 0 {
			keyBits = 40
		}
		verifierStart := headerEnd
		if verifierStart+40 > len(payload) {
			return nil, fmt.Errorf("legacy xls: CryptoAPI verifier truncated")
		}
		saltSize := int(binary.LittleEndian.Uint32(payload[verifierStart : verifierStart+4]))
		if saltSize != 16 {
			return nil, fmt.Errorf("legacy xls: invalid CryptoAPI verifier salt size %d", saltSize)
		}
		encVerifierStart := verifierStart + 4 + saltSize
		encVerifierEnd := encVerifierStart + 16
		hashSizeOffset := encVerifierEnd
		verifierHashSize := int(binary.LittleEndian.Uint32(payload[hashSizeOffset : hashSizeOffset+4]))
		encVerifierHashStart := hashSizeOffset + 4
		encVerifierHashLen := 20
		encVerifierHashEnd := encVerifierHashStart + encVerifierHashLen
		if encVerifierHashEnd > len(payload) {
			return nil, fmt.Errorf("legacy xls: CryptoAPI verifier hash truncated")
		}
		return &biff8EncryptionInfo{
			mode:                  biff8ModeCryptoAPI,
			hashAlg:               hashAlg,
			keyBits:               keyBits,
			filePassEnd:           filePassEnd,
			salt:                  append([]byte(nil), payload[verifierStart+4:encVerifierStart]...),
			encryptedVerifier:     append([]byte(nil), payload[encVerifierStart:encVerifierEnd]...),
			encryptedVerifierHash: append([]byte(nil), payload[encVerifierHashStart:encVerifierHashEnd]...),
			verifierHashSize:      verifierHashSize,
		}, nil
	default:
		return nil, fmt.Errorf("legacy xls: unsupported FilePass version %d.%d", versionMajor, versionMinor)
	}
}

func parseCryptoAPIHashAlgorithm(id uint32) (biff8HashAlgorithm, error) {
	switch id {
	case cryptoAPIHashMD5:
		return biff8HashMD5, nil
	case cryptoAPIHashSHA1:
		return biff8HashSHA1, nil
	default:
		return 0, fmt.Errorf("legacy xls: unsupported CryptoAPI hash algorithm 0x%04X", id)
	}
}

func toCryptHashAlg(ha biff8HashAlgorithm) crypt.HashAlgorithm {
	if ha == biff8HashSHA1 {
		return crypt.HashSHA1
	}
	return crypt.HashMD5
}

func verifyBiff8Password(info *biff8EncryptionInfo, password string) ([]byte, error) {
	if info == nil {
		return nil, fmt.Errorf("legacy xls: missing BIFF8 encryption info")
	}
	switch info.mode {
	case biff8ModeBinaryRC4:
		keyMaterial, err := crypt.VerifyBinaryRC4Password(password, info.salt, info.encryptedVerifier, info.encryptedVerifierHash)
		if err != nil {
			return nil, fmt.Errorf("legacy xls: Binary RC4 verifier cipher: %w", err)
		}
		if keyMaterial == nil {
			return nil, fmt.Errorf("legacy xls: invalid password for Binary RC4-encrypted workbook")
		}
		return keyMaterial, nil
	case biff8ModeCryptoAPI:
		keyMaterial, err := crypt.VerifyCryptoAPIPassword(password, toCryptHashAlg(info.hashAlg), info.keyBits,
			info.salt, info.encryptedVerifier, info.encryptedVerifierHash, info.verifierHashSize)
		if err != nil {
			return nil, fmt.Errorf("legacy xls: CryptoAPI verifier cipher: %w", err)
		}
		if keyMaterial == nil {
			return nil, fmt.Errorf("legacy xls: invalid password for CryptoAPI-encrypted workbook")
		}
		return keyMaterial, nil
	default:
		return nil, fmt.Errorf("legacy xls: unsupported BIFF8 encryption mode")
	}
}

func decryptBiff8Stream(data []byte, info *biff8EncryptionInfo, keyMaterial []byte) ([]byte, error) {
	out := append([]byte(nil), data...)
	mask := buildEncryptedPayloadMask(data, info.filePassEnd)
	startBlock := info.filePassEnd / biff8RC4RekeyInterval
	for block := startBlock; block*biff8RC4RekeyInterval < len(data); block++ {
		chunkStart := block * biff8RC4RekeyInterval
		chunkEnd := chunkStart + biff8RC4RekeyInterval
		if chunkEnd > len(data) {
			chunkEnd = len(data)
		}
		var blockKey []byte
		switch info.mode {
		case biff8ModeBinaryRC4:
			blockKey = crypt.DeriveBinaryRC4BlockKey(keyMaterial, block)
		case biff8ModeCryptoAPI:
			blockKey = crypt.DeriveCryptoAPIBlockKey(keyMaterial, toCryptHashAlg(info.hashAlg), info.keyBits, block)
		default:
			return nil, fmt.Errorf("legacy xls: unsupported BIFF8 encryption mode")
		}
		cipher, err := rc4.NewCipher(blockKey)
		if err != nil {
			return nil, fmt.Errorf("legacy xls: RC4 chunk cipher: %w", err)
		}
		plain := make([]byte, chunkEnd-chunkStart)
		cipher.XORKeyStream(plain, data[chunkStart:chunkEnd])
		for i := chunkStart; i < chunkEnd; i++ {
			if mask[i] {
				out[i] = plain[i-chunkStart]
			}
		}
	}
	return out, nil
}

func buildEncryptedPayloadMask(data []byte, startPos int) []bool {
	mask := make([]bool, len(data))
	for pos := startPos; pos+4 <= len(data); {
		sid := binary.LittleEndian.Uint16(data[pos:])
		size := int(binary.LittleEndian.Uint16(data[pos+2:]))
		payloadStart := pos + 4
		payloadEnd := payloadStart + size
		if payloadStart > len(data) {
			break
		}
		if payloadEnd > len(data) {
			payloadEnd = len(data)
		}
		if !isNeverEncryptedRecord(sid) {
			start := payloadStart
			// BoundSheet8.lbPlyPos must remain plain even in encrypted streams.
			if sid == biffBoundSheet {
				start += 4
				if start > payloadEnd {
					start = payloadEnd
				}
			}
			for i := start; i < payloadEnd; i++ {
				mask[i] = true
			}
		}
		if payloadEnd >= len(data) {
			break
		}
		pos += 4 + size
	}
	return mask
}

// verifyXORPassword checks the supplied password against the key and verifier
// stored in the FilePass record.  An empty password is treated as valid only
// when the workbook was saved without a password (key == 0 && verifier == 0).
func verifyXORPassword(key, verifier uint16, password string) bool {
	if password == "" {
		return key == 0 && verifier == 0
	}
	return uint16(crypt.CreateXorKey1(password)) == key && uint16(crypt.CreateXorVerifier1(password)) == verifier
}

// decryptXORStream applies XOR obfuscation decryption in place to all record
// payloads after the FilePass record.  Record headers (SID + size) and the
// payloads of never-encrypted record types remain untouched.
//
// The XOR array index for each payload byte follows POI's implementation of
// MS-OFFCRYPTO section 2.3.7.3:
//
//	index = (streamOffsetOfByte + recordPayloadSize) mod 16
func decryptXORStream(data []byte, startPos int, xorArray []byte) {
	for pos := startPos; pos < len(data)-4; {
		sid := binary.LittleEndian.Uint16(data[pos:])
		size := int(binary.LittleEndian.Uint16(data[pos+2:]))
		payloadStart := pos + 4
		if !isNeverEncryptedRecord(sid) && payloadStart+size <= len(data) {
			for i := 0; i < size; i++ {
				streamOffset := payloadStart + i
				idx := (streamOffset + size) % 16
				b := data[streamOffset]
				b = (b << 3) | (b >> 5) // rotate left by 3
				b ^= xorArray[idx]
				data[streamOffset] = b
			}
		}
		pos += 4 + size
	}
}

// isNeverEncryptedRecord returns true for record SIDs whose payloads must not
// be decrypted.  Matches POI Biff8DecryptingStream.isNeverEncryptedRecord.
func isNeverEncryptedRecord(sid uint16) bool {
	switch sid {
	case biffBOF, biffBOFOld, biffBOF3, biffBOF4:
		return true
	case biffInterfaceHdr:
		return true
	case biffFilePass:
		return true
	}
	return false
}
