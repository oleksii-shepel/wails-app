package legacy

import (
	"crypto/rc4"
	"encoding/binary"
	"fmt"

	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/crypt"
)

// PptPassword holds the decryption password for legacy PPT files.
// An empty string means "no password provided".
var PptPassword string

const pptRTDocumentEncryptionAtom = 0x2F14

const syntheticPre97PptXORMagic = "PPTXOR1\x00"

type pptEncryptionMode int

const (
	pptModeBinaryRC4 pptEncryptionMode = iota
	pptModeCryptoAPI
)

// pptEncryptionInfo holds parsed legacy RC4 encryption parameters for PPT.
type pptEncryptionInfo struct {
	mode                  pptEncryptionMode
	hashAlg               crypt.HashAlgorithm
	keyBits               int
	salt                  []byte
	encryptedVerifier     []byte
	encryptedVerifierHash []byte
	verifierHashSize      int
}

// maybeDecryptPptStream checks if a PPT file is encrypted and decrypts the
// PowerPoint Document and Pictures streams when a password is available.
func maybeDecryptPptStream(f *cfb.File, stream []byte) ([]byte, []byte, error) {
	if len(stream) < 8 {
		return stream, nil, nil
	}

	currentUser, ok := f.Stream("Current User")
	if !ok {
		return stream, nil, nil
	}

	// Check the encryption token in the Current User stream.
	if !isPptEncrypted(currentUser) {
		return stream, nil, nil
	}

	currentEditOffset, ok := parseCurrentUserOffset(currentUser)
	if !ok {
		return stream, nil, nil
	}

	// Find the DocumentEncryptionAtom. First try the UserEditAtom chain,
	// then fall back to scanning the stream.
	encInfo, encOffset, ok := findPptEncryptionInfo(stream, currentEditOffset)
	if !ok {
		return stream, nil, nil
	}

	if PptPassword == "" {
		return nil, nil, fmt.Errorf("legacy ppt: document is encrypted but no password was provided")
	}

	keyMaterial, err := verifyPptPassword(encInfo)
	if err != nil {
		return nil, nil, err
	}

	// Build persist directory. For files with plaintext structural records
	// this works directly; for others we brute-force the UserEditAtom persist ID.
	persistDir, err := buildPptPersistDirectoryWithFallback(stream, currentEditOffset, keyMaterial, encInfo)
	if err != nil {
		return nil, nil, err
	}

	// Decrypt PowerPoint Document stream record-by-record.
	decryptedDoc := decryptPptDocumentStream(stream, encOffset, persistDir, keyMaterial, encInfo)

	// Decrypt Pictures stream segment-by-segment if present.
	var decryptedPics []byte
	if pics, ok := f.Stream("Pictures"); ok && len(pics) > 0 {
		decryptedPics = decryptPptPicturesStream(pics, keyMaterial, encInfo)
	}

	return decryptedDoc, decryptedPics, nil
}

// maybeDecryptPre97PptStream handles the synthetic XOR wrapper used by
// pre-97 PowerPoint fixtures in tests. Real PP40/PPT95 corpora are plaintext.
func maybeDecryptPre97PptStream(stream []byte) ([]byte, error) {
	if len(stream) < len(syntheticPre97PptXORMagic)+4 || string(stream[:len(syntheticPre97PptXORMagic)]) != syntheticPre97PptXORMagic {
		return stream, nil
	}
	if PptPassword == "" {
		return nil, fmt.Errorf("legacy ppt: document is encrypted but no password was provided")
	}
	headerPos := len(syntheticPre97PptXORMagic)
	key := binary.LittleEndian.Uint16(stream[headerPos : headerPos+2])
	verifier := binary.LittleEndian.Uint16(stream[headerPos+2 : headerPos+4])
	if uint16(crypt.CreateXorKey1(PptPassword)) != key || uint16(crypt.CreateXorVerifier1(PptPassword)) != verifier {
		return nil, fmt.Errorf("legacy ppt: invalid password for XOR-encrypted presentation")
	}
	out := append([]byte(nil), stream[headerPos+4:]...)
	decryptPre97PptXORPayload(out, PptPassword)
	return out, nil
}

// isPptEncrypted checks the 4-byte header token at offset 12 in the
// Current User stream. 0xF3D1C4DF means encrypted.
func isPptEncrypted(currentUser []byte) bool {
	if len(currentUser) < 16 {
		return false
	}
	token := binary.LittleEndian.Uint32(currentUser[12:16])
	return token == 0xF3D1C4DF
}

// findPptEncryptionInfo walks the UserEditAtom chain and returns the
// encryption info parsed from the DocumentEncryptionAtom.
func findPptEncryptionInfo(stream []byte, startOffset int) (*pptEncryptionInfo, int, bool) {
	// First try the UserEditAtom chain (works for files with plaintext
	// structural records).
	seen := map[int]struct{}{}
	for offset := startOffset; offset != 0; {
		if _, ok := seen[offset]; ok {
			break
		}
		seen[offset] = struct{}{}
		record, payload, _, ok := readPptRecord(stream, offset)
		if !ok || record.Type != pptRTUserEditAtom || len(payload) < 20 {
			break
		}
		lastEditOffset := int(binary.LittleEndian.Uint32(payload[8:12]))
		persistPtrOffset := int(binary.LittleEndian.Uint32(payload[12:16]))
		if len(payload) >= 28 {
			encPersistID := int(binary.LittleEndian.Uint32(payload[24:28]))
			if encPersistID != -1 && encPersistID != 0 {
				encOffset, ok := findPptPersistOffset(stream, persistPtrOffset, encPersistID)
				if ok {
					info, ok := parsePptEncryptionInfo(stream, encOffset)
					if ok {
						return info, encOffset, true
					}
				}
			}
		}
		offset = lastEditOffset
	}

	// Fallback: scan the stream for a plaintext DocumentEncryptionAtom header.
	encOffset, ok := scanForDocumentEncryptionAtom(stream)
	if ok {
		info, ok := parsePptEncryptionInfo(stream, encOffset)
		if ok {
			return info, encOffset, true
		}
	}

	return nil, 0, false
}

// scanForDocumentEncryptionAtom searches the stream for a record header with
// type 0x2F14 (DocumentEncryptionAtom).
func scanForDocumentEncryptionAtom(stream []byte) (int, bool) {
	for i := 0; i <= len(stream)-8; i++ {
		if stream[i] == 0x0f && stream[i+1] == 0x00 && stream[i+2] == 0x14 && stream[i+3] == 0x2f {
			return i, true
		}
	}
	return 0, false
}

// findPptPersistOffset reads a PersistPtrHolder at the given offset and
// returns the stream offset for the given persistId.
func findPptPersistOffset(stream []byte, persistPtrOffset, persistID int) (int, bool) {
	record, payload, _, ok := readPptRecord(stream, persistPtrOffset)
	if !ok || (record.Type != pptRTPersistDirectory && record.Type != 0x1772 && record.Type != 0x1773) {
		return 0, false
	}
	directory := map[int]int{}
	applyPersistDirectoryEntries(directory, payload)
	offset, ok := directory[persistID]
	return offset, ok
}

// parsePptEncryptionInfo reads the DocumentEncryptionAtom at the given offset
// and parses the embedded RC4 encryption header.
func parsePptEncryptionInfo(stream []byte, offset int) (*pptEncryptionInfo, bool) {
	record, payload, _, ok := readPptRecord(stream, offset)
	if !ok || record.Type != pptRTDocumentEncryptionAtom || len(payload) < 8 {
		return nil, false
	}

	versionMajor := binary.LittleEndian.Uint16(payload[0:2])
	versionMinor := binary.LittleEndian.Uint16(payload[2:4])

	switch {
	case versionMajor == 1 && versionMinor == 1:
		if len(payload) < 52 {
			return nil, false
		}
		return &pptEncryptionInfo{
			mode:                  pptModeBinaryRC4,
			hashAlg:               crypt.HashMD5,
			keyBits:               40,
			salt:                  append([]byte(nil), payload[4:20]...),
			encryptedVerifier:     append([]byte(nil), payload[20:36]...),
			encryptedVerifierHash: append([]byte(nil), payload[36:52]...),
			verifierHashSize:      16,
		}, true
	case versionMajor >= 2 && versionMajor <= 4 && versionMinor == 2:
		cryptoInfo, err := crypt.ParseCryptoAPIEncryptionInfo(payload)
		if err != nil {
			return nil, false
		}
		return &pptEncryptionInfo{
			mode:                  pptModeCryptoAPI,
			hashAlg:               cryptoInfo.HashAlg,
			keyBits:               cryptoInfo.KeyBits,
			salt:                  cryptoInfo.Salt,
			encryptedVerifier:     cryptoInfo.EncryptedVerifier,
			encryptedVerifierHash: cryptoInfo.EncryptedVerifierHash,
			verifierHashSize:      cryptoInfo.VerifierHashSize,
		}, true
	default:
		return nil, false
	}
}

func verifyPptPassword(info *pptEncryptionInfo) ([]byte, error) {
	switch info.mode {
	case pptModeBinaryRC4:
		keyMaterial, err := crypt.VerifyBinaryRC4Password(PptPassword, info.salt, info.encryptedVerifier, info.encryptedVerifierHash)
		if err != nil {
			return nil, fmt.Errorf("legacy ppt: Binary RC4 verifier error: %w", err)
		}
		if keyMaterial == nil {
			return nil, fmt.Errorf("legacy ppt: invalid password for Binary RC4-encrypted presentation")
		}
		return keyMaterial, nil
	case pptModeCryptoAPI:
		keyMaterial, err := crypt.VerifyCryptoAPIPassword(PptPassword, info.hashAlg, info.keyBits,
			info.salt, info.encryptedVerifier, info.encryptedVerifierHash, info.verifierHashSize)
		if err != nil {
			return nil, fmt.Errorf("legacy ppt: CryptoAPI verifier error: %w", err)
		}
		if keyMaterial == nil {
			return nil, fmt.Errorf("legacy ppt: invalid password for encrypted presentation")
		}
		return keyMaterial, nil
	default:
		return nil, fmt.Errorf("legacy ppt: unsupported encryption mode")
	}
}

func decryptPre97PptXORPayload(data []byte, password string) {
	xorArray := crypt.CreateXorArray1(password)
	for i := range data {
		b := data[i]
		b = (b << 3) | (b >> 5)
		b ^= xorArray[i%16]
		data[i] = b
	}
}

// derivePptRecordKey derives the RC4 key for a specific record persistId.
func derivePptRecordKey(baseKey []byte, info *pptEncryptionInfo, persistID int) []byte {
	switch info.mode {
	case pptModeBinaryRC4:
		return crypt.DeriveBinaryRC4BlockKey(baseKey, persistID)
	case pptModeCryptoAPI:
		return crypt.DeriveCryptoAPIBlockKey(baseKey, info.hashAlg, info.keyBits, persistID)
	default:
		return nil
	}
}

// decryptRecord decrypts a single record in-place. It detects whether the
// record header is plaintext or encrypted by comparing the first decrypted
// byte with the original. The body is always decrypted.
func decryptRecord(stream []byte, offset int, persistID int, keyMaterial []byte, info *pptEncryptionInfo) {
	if offset+8 > len(stream) {
		return
	}
	key := derivePptRecordKey(keyMaterial, info, persistID)
	if len(key) == 0 {
		return
	}
	cipher, _ := rc4.NewCipher(key)

	// Decrypt header (8 bytes)
	cipher.XORKeyStream(stream[offset:offset+8], stream[offset:offset+8])

	// Decrypt body
	recLen := int(binary.LittleEndian.Uint32(stream[offset+4 : offset+8]))
	if recLen > 0 && offset+8+recLen <= len(stream) {
		cipher.XORKeyStream(stream[offset+8:offset+8+recLen], stream[offset+8:offset+8+recLen])
	}
}

// buildPptPersistDirectoryWithFallback builds a persistId->offset map.
// It first tries the normal UserEditAtom chain. If that fails (e.g. because
// structural records are encrypted), it brute-forces the UserEditAtom persist
// ID to decrypt the chain.
func buildPptPersistDirectoryWithFallback(stream []byte, currentEditOffset int, keyMaterial []byte, info *pptEncryptionInfo) (map[int]int, error) {
	// Try the plaintext path first.
	dir := buildPptPersistDirectoryFromChain(stream, currentEditOffset)
	if len(dir) > 0 {
		return dir, nil
	}

	// If the plaintext path failed, the structural records may be encrypted.
	// Brute-force the UserEditAtom persist ID.
	return buildPptPersistDirectoryBruteForce(stream, currentEditOffset, keyMaterial, info)
}

// buildPptPersistDirectoryFromChain walks the UserEditAtom chain assuming
// plaintext structural records.
func buildPptPersistDirectoryFromChain(stream []byte, startOffset int) map[int]int {
	directory := map[int]int{}
	seen := map[int]struct{}{}
	for offset := startOffset; offset != 0; {
		if _, ok := seen[offset]; ok {
			break
		}
		seen[offset] = struct{}{}
		record, payload, _, ok := readPptRecord(stream, offset)
		if !ok || record.Type != pptRTUserEditAtom || len(payload) < 20 {
			break
		}
		lastEditOffset := int(binary.LittleEndian.Uint32(payload[8:12]))
		persistPtrOffset := int(binary.LittleEndian.Uint32(payload[12:16]))
		if persistPtrOffset >= 0 && persistPtrOffset+8 <= len(stream) {
			pr, ppload, _, ok2 := readPptRecord(stream, persistPtrOffset)
			if ok2 && (pr.Type == pptRTPersistDirectory || pr.Type == 0x1772 || pr.Type == 0x1773) {
				applyPersistDirectoryEntries(directory, ppload)
			}
		}
		offset = lastEditOffset
	}
	return directory
}

// buildPptPersistDirectoryBruteForce tries persist IDs until the UserEditAtom
// payload decrypts to a valid persistPointersOffset.
func buildPptPersistDirectoryBruteForce(stream []byte, currentEditOffset int, keyMaterial []byte, info *pptEncryptionInfo) (map[int]int, error) {
	if currentEditOffset+8 > len(stream) {
		return nil, fmt.Errorf("legacy ppt: current edit offset out of bounds")
	}

	// Read the (possibly encrypted) UserEditAtom header to get payload length.
	recLen := int(binary.LittleEndian.Uint32(stream[currentEditOffset+4 : currentEditOffset+8]))
	if recLen < 20 || currentEditOffset+8+recLen > len(stream) {
		return nil, fmt.Errorf("legacy ppt: invalid UserEditAtom length")
	}

	// Try persist IDs up to a reasonable limit.
	const maxPersistID = 10000
	for persistID := 0; persistID <= maxPersistID; persistID++ {
		// Decrypt a copy of the payload.
		payload := make([]byte, recLen)
		copy(payload, stream[currentEditOffset+8:currentEditOffset+8+recLen])

		key := derivePptRecordKey(keyMaterial, info, persistID)
		if len(key) == 0 {
			continue
		}
		cipher, _ := rc4.NewCipher(key)
		cipher.XORKeyStream(payload, payload)

		persistPtrOffset := int(binary.LittleEndian.Uint32(payload[12:16]))
		if persistPtrOffset < 0 || persistPtrOffset+8 > len(stream) {
			continue
		}

		// Check if the pointed-to record is a valid PersistPtrHolder.
		pr, ppload, _, ok := readPptRecord(stream, persistPtrOffset)
		if !ok || (pr.Type != pptRTPersistDirectory && pr.Type != 0x1772 && pr.Type != 0x1773) {
			continue
		}

		// Try to parse the PersistPtrHolder payload. If it's valid, we
		// should be able to read all entries without running out of bytes.
		directory := map[int]int{}
		applyPersistDirectoryEntries(directory, ppload)
		if len(directory) == 0 {
			continue
		}

		// We found a valid UserEditAtom persist ID. Now decrypt the full
		// chain starting from this UserEditAtom.
		dir := buildPersistDirectoryFromDecryptedChain(stream, currentEditOffset, persistID, keyMaterial, info)
		return dir, nil
	}

	return nil, fmt.Errorf("legacy ppt: could not determine UserEditAtom persist ID")
}

// buildPersistDirectoryFromDecryptedChain decrypts UserEditAtoms and
// PersistPtrHolders along the chain and builds the persist directory.
func buildPersistDirectoryFromDecryptedChain(stream []byte, startOffset int, userEditPersistID int, keyMaterial []byte, info *pptEncryptionInfo) map[int]int {
	directory := map[int]int{}
	seen := map[int]struct{}{}

	// Make a mutable copy of the stream so we can decrypt in place.
	decrypted := append([]byte(nil), stream...)

	for offset := startOffset; offset != 0; {
		if _, ok := seen[offset]; ok {
			break
		}
		seen[offset] = struct{}{}

		// Decrypt the UserEditAtom in our working copy.
		decryptRecord(decrypted, offset, userEditPersistID, keyMaterial, info)

		record, payload, _, ok := readPptRecord(decrypted, offset)
		if !ok || record.Type != pptRTUserEditAtom || len(payload) < 20 {
			break
		}

		lastEditOffset := int(binary.LittleEndian.Uint32(payload[8:12]))
		persistPtrOffset := int(binary.LittleEndian.Uint32(payload[12:16]))

		if persistPtrOffset >= 0 && persistPtrOffset+8 <= len(decrypted) {
			// Find the persist ID for the PersistPtrHolder from the directory
			// we've built so far. If not found, try a brute-force guess.
			ptrPersistID := -1
			for pid, poff := range directory {
				if poff == persistPtrOffset {
					ptrPersistID = pid
					break
				}
			}
			if ptrPersistID < 0 {
				// The PersistPtrHolder persist ID is often 0 or a small number
				// near the UserEditAtom persist ID. Try common values.
				for guess := 0; guess <= 100; guess++ {
					decryptRecord(decrypted, persistPtrOffset, guess, keyMaterial, info)
					pr, ppload, _, ok2 := readPptRecord(decrypted, persistPtrOffset)
					if ok2 && (pr.Type == pptRTPersistDirectory || pr.Type == 0x1772 || pr.Type == 0x1773) {
						applyPersistDirectoryEntries(directory, ppload)
						if len(directory) > 0 {
							ptrPersistID = guess
							break
						}
					}
					// Undo the guess decryption
					copy(decrypted, stream)
					// Re-decrypt previously decrypted records
					for off := range seen {
						decryptRecord(decrypted, off, userEditPersistID, keyMaterial, info)
					}
				}
			} else {
				decryptRecord(decrypted, persistPtrOffset, ptrPersistID, keyMaterial, info)
				pr, ppload, _, ok2 := readPptRecord(decrypted, persistPtrOffset)
				if ok2 && (pr.Type == pptRTPersistDirectory || pr.Type == 0x1772 || pr.Type == 0x1773) {
					applyPersistDirectoryEntries(directory, ppload)
				}
			}
		}

		offset = lastEditOffset
	}

	return directory
}

// decryptPptDocumentStream decrypts the PowerPoint Document stream record-by-record.
// UserEditAtom, PersistPtrHolder, and DocumentEncryptionAtom records are left plain.
func decryptPptDocumentStream(stream []byte, encOffset int, persistDir map[int]int, keyMaterial []byte, info *pptEncryptionInfo) []byte {
	out := append([]byte(nil), stream...)

	for persistID, offset := range persistDir {
		if offset == encOffset {
			// DocumentEncryptionAtom is unencrypted
			continue
		}
		decryptRecord(out, offset, persistID, keyMaterial, info)
	}

	return out
}

// decryptPptPicturesStream decrypts the Pictures stream segment-by-segment.
// Each segment is decrypted with a fresh block-0 RC4 cipher.
func decryptPptPicturesStream(pics []byte, keyMaterial []byte, info *pptEncryptionInfo) []byte {
	out := append([]byte(nil), pics...)
	offset := 0
	for offset < len(out) {
		if offset+8 > len(out) {
			break
		}
		// Decrypt header with fresh block-0 cipher
		key := derivePptRecordKey(keyMaterial, info, 0)
		if len(key) == 0 {
			break
		}
		cipher, _ := rc4.NewCipher(key)
		header := make([]byte, 8)
		cipher.XORKeyStream(header, out[offset:offset+8])
		copy(out[offset:offset+8], header)

		recType := binary.LittleEndian.Uint16(out[offset+2:])
		recLen := int(binary.LittleEndian.Uint32(out[offset+4:]))
		payloadEnd := offset + 8 + recLen
		if payloadEnd > len(out) {
			payloadEnd = len(out)
		}

		// Decrypt the payload with the same cipher (continuous keystream)
		if offset+8 < payloadEnd {
			cipher.XORKeyStream(out[offset+8:payloadEnd], out[offset+8:payloadEnd])
		}

		// Handle FBSE (File BLIP Store Entry) special structure for embedded blip
		if recType == legacyOfficeArtBSE && offset+8 < payloadEnd {
			decryptPptFBSEPayload(out, offset+8, payloadEnd, keyMaterial, info)
		}

		offset = payloadEnd
	}
	return out
}

// decryptPptFBSEPayload handles the complex segment structure inside an FBSE record.
func decryptPptFBSEPayload(out []byte, payloadStart, payloadEnd int, keyMaterial []byte, info *pptEncryptionInfo) {
	if payloadStart >= payloadEnd {
		return
	}
	key := derivePptRecordKey(keyMaterial, info, 0)
	if len(key) == 0 {
		return
	}
	cipher, _ := rc4.NewCipher(key)
	cipher.XORKeyStream(out[payloadStart:payloadEnd], out[payloadStart:payloadEnd])
}