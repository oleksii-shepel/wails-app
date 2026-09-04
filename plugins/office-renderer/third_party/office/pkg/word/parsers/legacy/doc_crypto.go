package legacy

import (
	"crypto/rc4"
	"encoding/binary"
	"fmt"
	"strings"

	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/crypt"
)

// DocPassword holds the decryption password for legacy DOC files.
// An empty string means "no password provided".
var DocPassword string

const (
	docFibBaseLen       = 68
	docRC4RekeyInterval = 512

	docFlagEncrypted   = 0x0100
	docFlagWhichTblStm = 0x0200
	docFlagObfuscated  = 0x8000
)

type docEncryptionMode int

const (
	docModeXOR docEncryptionMode = iota
	docModeBinaryRC4
	docModeCryptoAPI
)

type docEncryptionInfo struct {
	mode                  docEncryptionMode
	hashAlg               crypt.HashAlgorithm
	keyBits               int
	salt                  []byte
	encryptedVerifier     []byte
	encryptedVerifierHash []byte
	verifierHashSize      int
	key                   uint16 // XOR key
	verifier              uint16 // XOR verifier
}

// maybeDecryptDocStreams checks if the CFB container holds an encrypted .doc
// and, when a password is available, decrypts the relevant streams and returns
// a new CFB file with plaintext contents.
func maybeDecryptDocStreams(f *cfb.File) (*cfb.File, error) {
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		wordStream, ok = f.Stream("Ole")
	}
	if !ok || len(wordStream) < 18 {
		return f, nil
	}

	if binary.LittleEndian.Uint16(wordStream[0:2]) != legacyDocFibSignature {
		return f, nil
	}

	flags := binary.LittleEndian.Uint16(wordStream[10:12])
	if flags&docFlagEncrypted == 0 {
		return f, nil
	}

	if DocPassword == "" {
		return nil, fmt.Errorf("legacy doc: document is encrypted but no password was provided")
	}

	tableName := "0Table"
	if flags&docFlagWhichTblStm != 0 {
		tableName = "1Table"
	}
	tableStream, ok := f.Stream(tableName)
	if !ok {
		return nil, fmt.Errorf("legacy doc: encrypted document missing %s stream", tableName)
	}

	lKey := binary.LittleEndian.Uint32(wordStream[14:18])

	info, err := parseDocEncryptionInfo(wordStream, tableStream, flags, lKey)
	if err != nil {
		return nil, err
	}

	keyMaterial, err := verifyDocPassword(info)
	if err != nil {
		return nil, err
	}

	decryptedWordDoc := decryptDocStream(wordStream, docFibBaseLen, info, keyMaterial)
	decryptedTable := decryptDocStream(tableStream, int(lKey), info, keyMaterial)

	var decryptedData []byte
	if dataStream, ok := f.Stream("Data"); ok && len(dataStream) > 0 {
		decryptedData = decryptDocStream(dataStream, 0, info, keyMaterial)
	}

	return rebuildDocCFB(f, decryptedWordDoc, decryptedTable, decryptedData, tableName)
}

func parseDocEncryptionInfo(wordStream, tableStream []byte, flags uint16, lKey uint32) (*docEncryptionInfo, error) {
	if flags&docFlagObfuscated != 0 {
		// XOR mode: lKey contains the 16-bit verifier in the low word,
		// and the 16-bit key is read from the first 2 bytes of the table stream.
		if len(tableStream) < 4 {
			return nil, fmt.Errorf("legacy doc: XOR encryption info too short")
		}
		return &docEncryptionInfo{
			mode:     docModeXOR,
			key:      binary.LittleEndian.Uint16(tableStream[0:2]),
			verifier: uint16(lKey & 0xFFFF),
		}, nil
	}

	if int(lKey) > len(tableStream) {
		return nil, fmt.Errorf("legacy doc: encryption header size %d exceeds table stream length %d", lKey, len(tableStream))
	}
	header := tableStream[:lKey]
	if len(header) < 4 {
		return nil, fmt.Errorf("legacy doc: encryption header too short (%d bytes)", len(header))
	}

	versionMajor := binary.LittleEndian.Uint16(header[0:2])
	versionMinor := binary.LittleEndian.Uint16(header[2:4])

	switch {
	case versionMajor == 1 && versionMinor == 1:
		// Binary RC4
		if len(header) < 52 {
			return nil, fmt.Errorf("legacy doc: Binary RC4 encryption header too short (%d bytes)", len(header))
		}
		return &docEncryptionInfo{
			mode:                  docModeBinaryRC4,
			hashAlg:               crypt.HashMD5,
			keyBits:               40,
			salt:                  append([]byte(nil), header[4:20]...),
			encryptedVerifier:     append([]byte(nil), header[20:36]...),
			encryptedVerifierHash: append([]byte(nil), header[36:52]...),
			verifierHashSize:      16,
		}, nil

	case versionMajor >= 2 && versionMajor <= 4 && versionMinor == 2:
		// CryptoAPI RC4
		cryptoInfo, err := crypt.ParseCryptoAPIEncryptionInfo(header)
		if err != nil {
			return nil, fmt.Errorf("legacy doc: %w", err)
		}
		return &docEncryptionInfo{
			mode:                  docModeCryptoAPI,
			hashAlg:               cryptoInfo.HashAlg,
			keyBits:               cryptoInfo.KeyBits,
			salt:                  cryptoInfo.Salt,
			encryptedVerifier:     cryptoInfo.EncryptedVerifier,
			encryptedVerifierHash: cryptoInfo.EncryptedVerifierHash,
			verifierHashSize:      cryptoInfo.VerifierHashSize,
		}, nil

	default:
		return nil, fmt.Errorf("legacy doc: unsupported encryption version %d.%d", versionMajor, versionMinor)
	}
}

func verifyDocPassword(info *docEncryptionInfo) ([]byte, error) {
	switch info.mode {
	case docModeXOR:
		if uint16(crypt.CreateXorKey1(DocPassword)) != info.key || uint16(crypt.CreateXorVerifier1(DocPassword)) != info.verifier {
			return nil, fmt.Errorf("legacy doc: invalid password for XOR-encrypted document")
		}
		return nil, nil // XOR doesn't use key material
	case docModeBinaryRC4:
		keyMaterial, err := crypt.VerifyBinaryRC4Password(DocPassword, info.salt, info.encryptedVerifier, info.encryptedVerifierHash)
		if err != nil {
			return nil, fmt.Errorf("legacy doc: Binary RC4 verifier error: %w", err)
		}
		if keyMaterial == nil {
			return nil, fmt.Errorf("legacy doc: invalid password for Binary RC4-encrypted document")
		}
		return keyMaterial, nil
	case docModeCryptoAPI:
		keyMaterial, err := crypt.VerifyCryptoAPIPassword(DocPassword, info.hashAlg, info.keyBits,
			info.salt, info.encryptedVerifier, info.encryptedVerifierHash, info.verifierHashSize)
		if err != nil {
			return nil, fmt.Errorf("legacy doc: CryptoAPI verifier error: %w", err)
		}
		if keyMaterial == nil {
			return nil, fmt.Errorf("legacy doc: invalid password for CryptoAPI-encrypted document")
		}
		return keyMaterial, nil
	default:
		return nil, fmt.Errorf("legacy doc: unsupported encryption mode")
	}
}

func decryptDocStream(stream []byte, offset int, info *docEncryptionInfo, keyMaterial []byte) []byte {
	if offset >= len(stream) {
		return append([]byte(nil), stream...)
	}
	out := append([]byte(nil), stream...)
	copy(out, stream[:offset])

	switch info.mode {
	case docModeXOR:
		xorArray := crypt.CreateXorArray1(DocPassword)
		for i := offset; i < len(stream); i++ {
			idx := i % 16
			b := stream[i]
			b = (b << 3) | (b >> 5) // rotate left by 3
			b ^= xorArray[idx]
			out[i] = b
		}
	case docModeBinaryRC4, docModeCryptoAPI:
		// Block counter is based on stream position / chunkSize (512), not on
		// the start of the encrypted portion. Chunks are aligned to 512-byte
		// boundaries from the beginning of the stream.
		for block := 0; block*docRC4RekeyInterval < len(stream); block++ {
			chunkStart := block * docRC4RekeyInterval
			chunkEnd := chunkStart + docRC4RekeyInterval
			if chunkEnd > len(stream) {
				chunkEnd = len(stream)
			}
			if chunkStart < offset {
				chunkStart = offset
			}
			if chunkStart >= chunkEnd {
				continue
			}
			var blockKey []byte
			if info.mode == docModeBinaryRC4 {
				blockKey = crypt.DeriveBinaryRC4BlockKey(keyMaterial, block)
			} else {
				blockKey = crypt.DeriveCryptoAPIBlockKey(keyMaterial, info.hashAlg, info.keyBits, block)
			}
			cipher, _ := rc4.NewCipher(blockKey)
			cipher.XORKeyStream(out[chunkStart:chunkEnd], stream[chunkStart:chunkEnd])
		}
	}
	return out
}

func rebuildDocCFB(orig *cfb.File, wordDoc, table, data []byte, tableName string) (*cfb.File, error) {
	var w *cfb.Writer
	if orig.SectorSize() == 4096 {
		w = cfb.NewWriter4K()
	} else {
		w = cfb.NewWriter()
	}

	// Copy all original streams, substituting decrypted ones.
	for _, name := range orig.StreamNames() {
		var streamData []byte
		switch strings.ToLower(name) {
		case "worddocument", "ole":
			if wordDoc != nil {
				streamData = wordDoc
			}
		case "0table", "1table":
			if strings.EqualFold(name, tableName) && table != nil {
				streamData = table
			}
		case "data":
			if data != nil {
				streamData = data
			}
		}
		if streamData == nil {
			var ok bool
			streamData, ok = orig.Stream(name)
			if !ok {
				continue
			}
		}
		if err := w.AddStream(name, streamData); err != nil {
			return nil, err
		}
	}

	// Preserve empty storages that don't conflict with streams.
	streamSet := make(map[string]bool)
	for _, name := range orig.StreamNames() {
		streamSet[strings.ToLower(name)] = true
	}
	for _, name := range orig.StorageNames() {
		if !streamSet[strings.ToLower(name)] {
			if err := w.AddStorage(name); err != nil {
				return nil, err
			}
		}
	}

	newBytes, err := w.Bytes()
	if err != nil {
		return nil, err
	}
	return cfb.OpenBytes(newBytes)
}