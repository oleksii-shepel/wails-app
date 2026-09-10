package opc

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"hash"
	"strings"

	"office/pkg/ooxml/cfb"
)

const (
	olePackageStreamName          = "Package"
	oleEncryptionInfoStreamName   = "EncryptionInfo"
	oleEncryptedPackageStreamName = "EncryptedPackage"
	defaultOOXMLPassword          = "VelvetSweatshop"

	standardEncryptionMode packageEncryptionMode = iota + 1
	agileEncryptionMode

	standardEncryptionSpinCount = 50000
	agileSegmentSize            = 4096

	ecmaCipherAES128 = 0x660E
	ecmaCipherAES192 = 0x660F
	ecmaCipherAES256 = 0x6610

	ecmaHashSHA1   = 0x8004
	ecmaHashSHA256 = 0x800C
	ecmaHashSHA384 = 0x800D
	ecmaHashSHA512 = 0x800E

	agileEncryptionNS       = "http://schemas.microsoft.com/office/2006/encryption"
	agilePasswordNS         = "http://schemas.microsoft.com/office/2006/keyEncryptor/password"
	agileEncryptionNSStrict = "http://schemas.openxmlformats.org/officeDocument/2006/encryption"
	agilePasswordNSStrict   = "http://schemas.openxmlformats.org/officeDocument/2006/keyEncryptor/password"
)

var (
	agileVerifierInputBlock = []byte{0xFE, 0xA7, 0xD2, 0x76, 0x3B, 0x4B, 0x9E, 0x79}
	agileVerifierHashBlock  = []byte{0xD7, 0xAA, 0x0F, 0x6D, 0x30, 0x61, 0x34, 0x4E}
	agileCryptoKeyBlock     = []byte{0x14, 0x6E, 0x0B, 0xE7, 0xAB, 0xAC, 0xD0, 0xD6}
)

type packageEncryptionMode int

type hashAlgorithm struct {
	name string
	size int
	new  func() hash.Hash
}

type standardEncryptionInfo struct {
	cipherID              uint32
	hashAlg               hashAlgorithm
	keyBits               int
	salt                  []byte
	encryptedVerifier     []byte
	encryptedVerifierHash []byte
}

type agileEncryptionInfo struct {
	keyData     agileKeyData
	passwordKey agilePasswordKey
}

type agileKeyData struct {
	cipherAlgorithm string
	cipherChaining  string
	hashAlgorithm   hashAlgorithm
	blockSize       int
	keyBits         int
	hashSize        int
	saltValue       []byte
}

type agilePasswordKey struct {
	cipherAlgorithm            string
	cipherChaining             string
	hashAlgorithm              hashAlgorithm
	saltSize                   int
	blockSize                  int
	keyBits                    int
	hashSize                   int
	spinCount                  int
	saltValue                  []byte
	encryptedVerifierHashInput []byte
	encryptedVerifierHashValue []byte
	encryptedKeyValue          []byte
}

type encryptionInfo struct {
	mode     packageEncryptionMode
	standard *standardEncryptionInfo
	agile    *agileEncryptionInfo
}

type agileDescriptorXML struct {
	XMLName       xml.Name              `xml:"http://schemas.microsoft.com/office/2006/encryption encryption"`
	KeyData       agileKeyDataXML       `xml:"http://schemas.microsoft.com/office/2006/encryption keyData"`
	KeyEncryptors agileKeyEncryptorsXML `xml:"http://schemas.microsoft.com/office/2006/encryption keyEncryptors"`
}

type agileKeyDataXML struct {
	SaltSize        int    `xml:"saltSize,attr"`
	BlockSize       int    `xml:"blockSize,attr"`
	KeyBits         int    `xml:"keyBits,attr"`
	HashSize        int    `xml:"hashSize,attr"`
	CipherAlgorithm string `xml:"cipherAlgorithm,attr"`
	CipherChaining  string `xml:"cipherChaining,attr"`
	HashAlgorithm   string `xml:"hashAlgorithm,attr"`
	SaltValue       string `xml:"saltValue,attr"`
}

type agileKeyEncryptorsXML struct {
	KeyEncryptors []agileKeyEncryptorXML `xml:"http://schemas.microsoft.com/office/2006/encryption keyEncryptor"`
}

type agileKeyEncryptorXML struct {
	Password *agilePasswordEncryptedKeyXML `xml:"http://schemas.microsoft.com/office/2006/keyEncryptor/password encryptedKey"`
}

type agilePasswordEncryptedKeyXML struct {
	SaltSize                   int    `xml:"saltSize,attr"`
	BlockSize                  int    `xml:"blockSize,attr"`
	KeyBits                    int    `xml:"keyBits,attr"`
	HashSize                   int    `xml:"hashSize,attr"`
	CipherAlgorithm            string `xml:"cipherAlgorithm,attr"`
	CipherChaining             string `xml:"cipherChaining,attr"`
	HashAlgorithm              string `xml:"hashAlgorithm,attr"`
	SaltValue                  string `xml:"saltValue,attr"`
	SpinCount                  int    `xml:"spinCount,attr"`
	EncryptedVerifierHashInput string `xml:"encryptedVerifierHashInput,attr"`
	EncryptedVerifierHashValue string `xml:"encryptedVerifierHashValue,attr"`
	EncryptedKeyValue          string `xml:"encryptedKeyValue,attr"`
}

func openContainerBytes(data []byte, zipErr error, options OpenOptions) (*Package, error) {
	file, err := cfb.OpenBytes(data)
	if err != nil {
		return nil, zipErr
	}

	plainPackage, ok, err := file.StreamBestEffort(olePackageStreamName)
	if ok {
		return OpenBytesWithOptions(plainPackage, options)
	}
	if err != nil {
		return nil, err
	}

	encryptionInfoData, ok, err := file.StreamWithError(oleEncryptionInfoStreamName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, zipErr
	}
	encryptedPackageData, ok, err := file.StreamBestEffort(oleEncryptedPackageStreamName)
	if !ok {
		return nil, fmt.Errorf("opc: OLE package has %q but no %q stream", oleEncryptionInfoStreamName, oleEncryptedPackageStreamName)
	}
	if err != nil && len(encryptedPackageData) == 0 {
		return nil, err
	}

	decrypted, err := DecryptEncryptedPackageWithPassword(encryptionInfoData, encryptedPackageData, options.Password)
	if err != nil {
		return nil, err
	}
	return OpenBytesWithOptions(decrypted, options)
}

// DecryptEncryptedPackage decrypts an EncryptedPackage stream using the
// ECMA-376 default password policy.
func DecryptEncryptedPackage(infoData, encryptedPackage []byte) ([]byte, error) {
	return DecryptEncryptedPackageWithPassword(infoData, encryptedPackage, "")
}

// DecryptEncryptedPackageWithPassword decrypts an EncryptedPackage stream using
// a password scoped to this call. The ECMA-376 default password remains a
// fallback candidate for compatibility with protected-but-not-user-passworded
// Office packages.
func DecryptEncryptedPackageWithPassword(infoData, encryptedPackage []byte, password string) ([]byte, error) {
	info, err := parseEncryptionInfo(infoData)
	if err != nil {
		return nil, err
	}
	switch info.mode {
	case standardEncryptionMode:
		return decryptStandardPackage(info.standard, encryptedPackage, password)
	case agileEncryptionMode:
		return decryptAgilePackage(info.agile, encryptedPackage, password)
	default:
		return nil, fmt.Errorf("opc: unsupported OOXML encryption mode")
	}
}

func parseEncryptionInfo(data []byte) (*encryptionInfo, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("opc: EncryptionInfo stream too short (%d bytes)", len(data))
	}

	versionMajor := binary.LittleEndian.Uint16(data[0:2])
	versionMinor := binary.LittleEndian.Uint16(data[2:4])
	flags := binary.LittleEndian.Uint32(data[4:8])

	switch {
	case versionMajor >= 2 && versionMajor <= 4 && versionMinor == 2:
		if flags&0x20 == 0 {
			return nil, fmt.Errorf("opc: non-AES OOXML OLE encryption is not supported")
		}
		std, err := parseStandardEncryptionInfo(data[8:])
		if err != nil {
			return nil, err
		}
		return &encryptionInfo{mode: standardEncryptionMode, standard: std}, nil
	case versionMajor == 4 && versionMinor == 4:
		agile, err := parseAgileEncryptionInfo(data[8:])
		if err != nil {
			return nil, err
		}
		return &encryptionInfo{mode: agileEncryptionMode, agile: agile}, nil
	default:
		return nil, fmt.Errorf("opc: unsupported OOXML encryption version %d.%d", versionMajor, versionMinor)
	}
}

func parseStandardEncryptionInfo(data []byte) (*standardEncryptionInfo, error) {
	if len(data) < 4+32 {
		return nil, fmt.Errorf("opc: standard EncryptionInfo stream too short (%d bytes)", len(data))
	}

	pos := 0
	_ = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4

	if pos+32 > len(data) {
		return nil, fmt.Errorf("opc: standard encryption header truncated")
	}
	headerFlags := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	_ = headerFlags
	pos += 4 // sizeExtra
	cipherID := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	hashID := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	keyBits := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
	pos += 4
	if keyBits == 0 {
		return nil, fmt.Errorf("opc: standard encryption key size missing")
	}
	pos += 4 // providerType
	pos += 8 // reserved

	if pos+4 > len(data) {
		return nil, fmt.Errorf("opc: standard encryption verifier missing")
	}
	if binary.LittleEndian.Uint32(data[pos:pos+4]) != 16 {
		for {
			if pos+2 > len(data) {
				return nil, fmt.Errorf("opc: standard encryption CSP name truncated")
			}
			if data[pos] == 0 && data[pos+1] == 0 {
				pos += 2
				break
			}
			pos += 2
		}
	}

	if pos+4+16+16+4 > len(data) {
		return nil, fmt.Errorf("opc: standard encryption verifier truncated")
	}

	saltSize := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	if saltSize != 16 {
		return nil, fmt.Errorf("opc: unsupported standard encryption salt size %d", saltSize)
	}
	salt := append([]byte(nil), data[pos:pos+16]...)
	pos += 16
	encryptedVerifier := append([]byte(nil), data[pos:pos+16]...)
	pos += 16
	verifierHashSize := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	if verifierHashSize != 20 {
		return nil, fmt.Errorf("opc: unsupported standard verifier hash size %d", verifierHashSize)
	}

	keyBytes, err := aesKeyBytes(cipherID, keyBits)
	if err != nil {
		return nil, err
	}
	if pos+32 > len(data) {
		return nil, fmt.Errorf("opc: standard encrypted verifier hash truncated")
	}
	encryptedVerifierHash := append([]byte(nil), data[pos:pos+32]...)
	pos += 32
	_ = keyBytes

	hashAlg, err := hashAlgorithmFromECMA(hashID)
	if err != nil {
		return nil, err
	}
	if hashAlg.name != "SHA-1" {
		return nil, fmt.Errorf("opc: standard encryption hash %s is not supported", hashAlg.name)
	}

	return &standardEncryptionInfo{
		cipherID:              cipherID,
		hashAlg:               hashAlg,
		keyBits:               keyBits,
		salt:                  salt,
		encryptedVerifier:     encryptedVerifier,
		encryptedVerifierHash: encryptedVerifierHash,
	}, nil
}

func parseAgileEncryptionInfo(data []byte) (*agileEncryptionInfo, error) {
	// Support both Microsoft and ISO strict namespaces.
	data = bytes.ReplaceAll(data, []byte(agileEncryptionNSStrict), []byte(agileEncryptionNS))
	data = bytes.ReplaceAll(data, []byte(agilePasswordNSStrict), []byte(agilePasswordNS))

	var descriptor agileDescriptorXML
	if err := xml.Unmarshal(data, &descriptor); err != nil {
		return nil, fmt.Errorf("opc: parse agile EncryptionInfo: %w", err)
	}

	keyHash, err := hashAlgorithmFromString(descriptor.KeyData.HashAlgorithm)
	if err != nil {
		return nil, err
	}
	keySalt, err := decodeBase64Attr(descriptor.KeyData.SaltValue)
	if err != nil {
		return nil, fmt.Errorf("opc: decode agile keyData saltValue: %w", err)
	}
	keyData := agileKeyData{
		cipherAlgorithm: descriptor.KeyData.CipherAlgorithm,
		cipherChaining:  descriptor.KeyData.CipherChaining,
		hashAlgorithm:   keyHash,
		blockSize:       descriptor.KeyData.BlockSize,
		keyBits:         descriptor.KeyData.KeyBits,
		hashSize:        descriptor.KeyData.HashSize,
		saltValue:       keySalt,
	}

	var passwordKeyXML *agilePasswordEncryptedKeyXML
	for _, encryptor := range descriptor.KeyEncryptors.KeyEncryptors {
		if encryptor.Password != nil {
			passwordKeyXML = encryptor.Password
			break
		}
	}
	if passwordKeyXML == nil {
		return nil, fmt.Errorf("opc: agile EncryptionInfo has no password key encryptor")
	}

	passwordHash, err := hashAlgorithmFromString(passwordKeyXML.HashAlgorithm)
	if err != nil {
		return nil, err
	}
	passwordSalt, err := decodeBase64Attr(passwordKeyXML.SaltValue)
	if err != nil {
		return nil, fmt.Errorf("opc: decode agile password saltValue: %w", err)
	}
	encryptedVerifierHashInput, err := decodeBase64Attr(passwordKeyXML.EncryptedVerifierHashInput)
	if err != nil {
		return nil, fmt.Errorf("opc: decode agile encryptedVerifierHashInput: %w", err)
	}
	encryptedVerifierHashValue, err := decodeBase64Attr(passwordKeyXML.EncryptedVerifierHashValue)
	if err != nil {
		return nil, fmt.Errorf("opc: decode agile encryptedVerifierHashValue: %w", err)
	}
	encryptedKeyValue, err := decodeBase64Attr(passwordKeyXML.EncryptedKeyValue)
	if err != nil {
		return nil, fmt.Errorf("opc: decode agile encryptedKeyValue: %w", err)
	}

	passwordKey := agilePasswordKey{
		cipherAlgorithm:            passwordKeyXML.CipherAlgorithm,
		cipherChaining:             passwordKeyXML.CipherChaining,
		hashAlgorithm:              passwordHash,
		saltSize:                   passwordKeyXML.SaltSize,
		blockSize:                  passwordKeyXML.BlockSize,
		keyBits:                    passwordKeyXML.KeyBits,
		hashSize:                   passwordKeyXML.HashSize,
		spinCount:                  passwordKeyXML.SpinCount,
		saltValue:                  passwordSalt,
		encryptedVerifierHashInput: encryptedVerifierHashInput,
		encryptedVerifierHashValue: encryptedVerifierHashValue,
		encryptedKeyValue:          encryptedKeyValue,
	}

	if keyData.cipherAlgorithm != "AES" || passwordKey.cipherAlgorithm != "AES" {
		return nil, fmt.Errorf("opc: agile cipher %q is not supported", keyData.cipherAlgorithm)
	}
	if keyData.cipherChaining != "ChainingModeCBC" || passwordKey.cipherChaining != "ChainingModeCBC" {
		return nil, fmt.Errorf("opc: agile chaining mode %q is not supported", keyData.cipherChaining)
	}
	if keyData.hashSize != keyData.hashAlgorithm.size {
		return nil, fmt.Errorf("opc: agile keyData hash size %d does not match %s", keyData.hashSize, keyData.hashAlgorithm.name)
	}
	if passwordKey.hashSize != passwordKey.hashAlgorithm.size {
		return nil, fmt.Errorf("opc: agile password hash size %d does not match %s", passwordKey.hashSize, passwordKey.hashAlgorithm.name)
	}

	return &agileEncryptionInfo{
		keyData:     keyData,
		passwordKey: passwordKey,
	}, nil
}

func decryptStandardPackage(info *standardEncryptionInfo, encryptedPackage []byte, password string) ([]byte, error) {
	key, err := verifyStandardPassword(info, password)
	if err != nil {
		return nil, err
	}

	plaintextLen, ciphertext, err := readEncryptedPackageStream(encryptedPackage)
	if err != nil {
		return nil, err
	}
	cipherLen := roundUp(int(plaintextLen), aes.BlockSize)
	if cipherLen > len(ciphertext) {
		return nil, fmt.Errorf("opc: standard EncryptedPackage truncated (%d > %d)", cipherLen, len(ciphertext))
	}
	ciphertext = ciphertext[:cipherLen]

	keyBytes, err := aesKeyBytes(info.cipherID, info.keyBits)
	if err != nil {
		return nil, err
	}
	if len(key) != keyBytes {
		return nil, fmt.Errorf("opc: derived standard key length %d does not match cipher key length %d", len(key), keyBytes)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := decryptECB(block, ciphertext)
	if err != nil {
		return nil, err
	}
	if plaintextLen > uint64(len(plaintext)) {
		return nil, fmt.Errorf("opc: decrypted standard package truncated (%d > %d)", plaintextLen, len(plaintext))
	}
	return append([]byte(nil), plaintext[:plaintextLen]...), nil
}

func verifyStandardPassword(info *standardEncryptionInfo, password string) ([]byte, error) {
	keyBytes, err := aesKeyBytes(info.cipherID, info.keyBits)
	if err != nil {
		return nil, err
	}

	for _, password := range candidatePasswords(password) {
		pwHash := hashPassword(password, info.hashAlg, info.salt, standardEncryptionSpinCount)
		blockKey := []byte{0, 0, 0, 0}
		finalHash := generateKey(pwHash, info.hashAlg, blockKey, info.hashAlg.size)
		x1 := fillAndXor(finalHash, 0x36)
		x2 := fillAndXor(finalHash, 0x5C)
		key := append(append([]byte(nil), x1...), x2...)
		key = key[:keyBytes]

		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		verifier, err := decryptECB(block, info.encryptedVerifier)
		if err != nil {
			return nil, err
		}
		verifierHash, err := decryptECB(block, info.encryptedVerifierHash)
		if err != nil {
			return nil, err
		}
		expectedHash := digest(info.hashAlg, verifier)
		if equalBytes(expectedHash, verifierHash[:len(expectedHash)]) {
			return key, nil
		}
	}

	return nil, ooxmlPasswordError(password)
}

func decryptAgilePackage(info *agileEncryptionInfo, encryptedPackage []byte, password string) ([]byte, error) {
	key, err := verifyAgilePassword(info, password)
	if err != nil {
		return nil, err
	}

	plaintextLen, ciphertext, err := readEncryptedPackageStream(encryptedPackage)
	if err != nil {
		return nil, err
	}

	blockSize := info.keyData.blockSize
	if blockSize <= 0 {
		return nil, fmt.Errorf("opc: invalid agile block size %d", blockSize)
	}

	out := make([]byte, 0, plaintextLen)
	offset := 0
	remaining := int(plaintextLen)
	for blockIndex := 0; remaining > 0; blockIndex++ {
		chunkLen := remaining
		if chunkLen > agileSegmentSize {
			chunkLen = agileSegmentSize
		}
		cipherLen := roundUp(chunkLen, blockSize)
		if offset+cipherLen > len(ciphertext) {
			return nil, fmt.Errorf("opc: agile EncryptedPackage truncated in block %d", blockIndex)
		}
		iv := generateIV(info.keyData.hashAlgorithm, info.keyData.saltValue, littleEndianUint32(uint32(blockIndex)), blockSize)
		chunk, err := decryptAESBlockMode(key, iv, ciphertext[offset:offset+cipherLen])
		if err != nil {
			return nil, err
		}
		out = append(out, chunk[:chunkLen]...)
		offset += cipherLen
		remaining -= chunkLen
	}

	return out, nil
}

func verifyAgilePassword(info *agileEncryptionInfo, password string) ([]byte, error) {
	passwordKey := info.passwordKey
	if passwordKey.blockSize <= 0 {
		return nil, fmt.Errorf("opc: invalid agile password block size %d", passwordKey.blockSize)
	}

	for _, password := range candidatePasswords(password) {
		pwHash := hashPassword(password, passwordKey.hashAlgorithm, passwordKey.saltValue, passwordKey.spinCount)

		verifierKey := generateKey(pwHash, passwordKey.hashAlgorithm, agileVerifierInputBlock, passwordKey.keyBits/8)
		verifierIV := generateIV(passwordKey.hashAlgorithm, passwordKey.saltValue, nil, passwordKey.blockSize)
		verifierInput, err := decryptAESBlockMode(verifierKey, verifierIV, padBlock0(passwordKey.encryptedVerifierHashInput, passwordKey.blockSize))
		if err != nil {
			return nil, err
		}

		hashKey := generateKey(pwHash, passwordKey.hashAlgorithm, agileVerifierHashBlock, passwordKey.keyBits/8)
		hashIV := generateIV(passwordKey.hashAlgorithm, passwordKey.saltValue, nil, passwordKey.blockSize)
		verifierHashValue, err := decryptAESBlockMode(hashKey, hashIV, padBlock0(passwordKey.encryptedVerifierHashValue, passwordKey.blockSize))
		if err != nil {
			return nil, err
		}
		verifierHashValue = verifierHashValue[:passwordKey.hashAlgorithm.size]

		expectedHash := digest(passwordKey.hashAlgorithm, verifierInput)
		if !equalBytes(expectedHash, verifierHashValue) {
			continue
		}

		packageKeyKey := generateKey(pwHash, passwordKey.hashAlgorithm, agileCryptoKeyBlock, passwordKey.keyBits/8)
		packageKeyIV := generateIV(passwordKey.hashAlgorithm, passwordKey.saltValue, nil, passwordKey.blockSize)
		packageKey, err := decryptAESBlockMode(packageKeyKey, packageKeyIV, padBlock0(passwordKey.encryptedKeyValue, passwordKey.blockSize))
		if err != nil {
			return nil, err
		}
		keyBytes := info.keyData.keyBits / 8
		if keyBytes <= 0 || keyBytes > len(packageKey) {
			return nil, fmt.Errorf("opc: invalid agile package key size %d", info.keyData.keyBits)
		}
		return append([]byte(nil), packageKey[:keyBytes]...), nil
	}

	return nil, ooxmlPasswordError(password)
}

func readEncryptedPackageStream(stream []byte) (uint64, []byte, error) {
	if len(stream) < 8 {
		return 0, nil, fmt.Errorf("opc: EncryptedPackage stream too short (%d bytes)", len(stream))
	}
	plaintextLen := binary.LittleEndian.Uint64(stream[:8])
	return plaintextLen, stream[8:], nil
}

func candidatePasswords(password string) []string {
	if password == "" {
		return []string{defaultOOXMLPassword}
	}
	if password == defaultOOXMLPassword {
		return []string{password}
	}
	return []string{password, defaultOOXMLPassword}
}

func ooxmlPasswordError(password string) error {
	if password == "" {
		return fmt.Errorf("opc: encrypted OOXML package requires a password")
	}
	return fmt.Errorf("opc: invalid password for encrypted OOXML package")
}

// IsEncryptedPackageError reports whether err indicates that an OOXML package
// is encrypted and cannot be opened without a password.
func IsEncryptedPackageError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "opc: encrypted OOXML package requires a password") ||
		strings.Contains(msg, "opc: invalid password for encrypted OOXML package")
}

func aesKeyBytes(cipherID uint32, keyBits int) (int, error) {
	switch cipherID {
	case ecmaCipherAES128:
		if keyBits != 128 {
			return 0, fmt.Errorf("opc: invalid AES-128 key size %d", keyBits)
		}
		return 16, nil
	case ecmaCipherAES192:
		if keyBits != 192 {
			return 0, fmt.Errorf("opc: invalid AES-192 key size %d", keyBits)
		}
		return 24, nil
	case ecmaCipherAES256:
		if keyBits != 256 {
			return 0, fmt.Errorf("opc: invalid AES-256 key size %d", keyBits)
		}
		return 32, nil
	default:
		return 0, fmt.Errorf("opc: unsupported AES cipher 0x%04X", cipherID)
	}
}

func hashAlgorithmFromECMA(id uint32) (hashAlgorithm, error) {
	switch id {
	case ecmaHashSHA1:
		return hashAlgorithm{name: "SHA-1", size: 20, new: sha1.New}, nil
	case ecmaHashSHA256:
		return hashAlgorithm{name: "SHA-256", size: 32, new: sha256.New}, nil
	case ecmaHashSHA384:
		return hashAlgorithm{name: "SHA-384", size: 48, new: sha512.New384}, nil
	case ecmaHashSHA512:
		return hashAlgorithm{name: "SHA-512", size: 64, new: sha512.New}, nil
	default:
		return hashAlgorithm{}, fmt.Errorf("opc: unsupported hash algorithm 0x%04X", id)
	}
}

func hashAlgorithmFromString(name string) (hashAlgorithm, error) {
	switch name {
	case "SHA1", "SHA-1":
		return hashAlgorithm{name: "SHA-1", size: 20, new: sha1.New}, nil
	case "SHA256", "SHA-256":
		return hashAlgorithm{name: "SHA-256", size: 32, new: sha256.New}, nil
	case "SHA384", "SHA-384":
		return hashAlgorithm{name: "SHA-384", size: 48, new: sha512.New384}, nil
	case "SHA512", "SHA-512":
		return hashAlgorithm{name: "SHA-512", size: 64, new: sha512.New}, nil
	default:
		return hashAlgorithm{}, fmt.Errorf("opc: unsupported hash algorithm %q", name)
	}
}

func hashPassword(password string, hashAlg hashAlgorithm, salt []byte, spinCount int) []byte {
	h := hashAlg.new()
	h.Write(salt)
	h.Write(utf16LEBytes(password))
	sum := h.Sum(nil)
	iterator := make([]byte, 4)
	for i := 0; i < spinCount; i++ {
		binary.LittleEndian.PutUint32(iterator, uint32(i))
		h = hashAlg.new()
		h.Write(iterator)
		h.Write(sum)
		sum = h.Sum(nil)
	}
	return sum
}

func generateKey(passwordHash []byte, hashAlg hashAlgorithm, blockKey []byte, size int) []byte {
	h := hashAlg.new()
	h.Write(passwordHash)
	h.Write(blockKey)
	return padBlockByte(h.Sum(nil), size, 0x36)
}

func generateIV(hashAlg hashAlgorithm, salt, blockKey []byte, blockSize int) []byte {
	iv := salt
	if blockKey != nil {
		h := hashAlg.new()
		h.Write(salt)
		h.Write(blockKey)
		iv = h.Sum(nil)
	}
	return padBlockByte(iv, blockSize, 0x36)
}

func fillAndXor(data []byte, fill byte) []byte {
	buf := make([]byte, 64)
	for i := range buf {
		buf[i] = fill
	}
	for i := range data {
		buf[i] ^= data[i]
	}
	sum := sha1.Sum(buf)
	return append([]byte(nil), sum[:]...)
}

func decryptECB(block cipher.Block, ciphertext []byte) ([]byte, error) {
	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("opc: ECB ciphertext length %d is not a multiple of block size %d", len(ciphertext), block.BlockSize())
	}
	plaintext := make([]byte, len(ciphertext))
	for offset := 0; offset < len(ciphertext); offset += block.BlockSize() {
		block.Decrypt(plaintext[offset:offset+block.BlockSize()], ciphertext[offset:offset+block.BlockSize()])
	}
	return plaintext, nil
}

func decryptAESBlockMode(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("opc: CBC ciphertext length %d is not a multiple of block size %d", len(ciphertext), block.BlockSize())
	}
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("opc: invalid IV length %d for block size %d", len(iv), block.BlockSize())
	}
	plaintext := make([]byte, len(ciphertext))
	decrypter := cipher.NewCBCDecrypter(block, iv)
	decrypter.CryptBlocks(plaintext, ciphertext)
	return plaintext, nil
}

func digest(hashAlg hashAlgorithm, data []byte) []byte {
	h := hashAlg.new()
	h.Write(data)
	return h.Sum(nil)
}

func padBlock0(data []byte, blockSize int) []byte {
	return padBlockByte(data, roundUp(len(data), blockSize), 0)
}

func padBlockByte(data []byte, size int, fill byte) []byte {
	if len(data) == size {
		return append([]byte(nil), data...)
	}
	out := make([]byte, size)
	for i := len(data); i < len(out); i++ {
		out[i] = fill
	}
	copy(out, data)
	return out
}

func roundUp(size, blockSize int) int {
	if blockSize <= 0 {
		return size
	}
	if size%blockSize == 0 {
		return size
	}
	return ((size / blockSize) + 1) * blockSize
}

func littleEndianUint32(v uint32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, v)
	return out
}

func decodeBase64Attr(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(value)
}

func utf16LEBytes(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for _, r := range s {
		var buf [4]byte
		n := utf16EncodeRune(buf[:], r)
		out = append(out, buf[:n]...)
	}
	return out
}

func utf16EncodeRune(dst []byte, r rune) int {
	if r < 0x10000 {
		binary.LittleEndian.PutUint16(dst, uint16(r))
		return 2
	}
	r -= 0x10000
	high := uint16(0xD800 + ((r >> 10) & 0x3FF))
	low := uint16(0xDC00 + (r & 0x3FF))
	binary.LittleEndian.PutUint16(dst[0:2], high)
	binary.LittleEndian.PutUint16(dst[2:4], low)
	return 4
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}


// randBytes returns n cryptographically secure random bytes.
func randBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// encryptAESBlockMode encrypts plaintext with AES-CBC using the given key and IV.
func encryptAESBlockMode(key, iv, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(plaintext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("opc: CBC plaintext length %d is not a multiple of block size %d", len(plaintext), block.BlockSize())
	}
	if len(iv) != block.BlockSize() {
		return nil, fmt.Errorf("opc: invalid IV length %d for block size %d", len(iv), block.BlockSize())
	}
	ciphertext := make([]byte, len(plaintext))
	encrypter := cipher.NewCBCEncrypter(block, iv)
	encrypter.CryptBlocks(ciphertext, plaintext)
	return ciphertext, nil
}

// encryptECB encrypts plaintext with AES-ECB using the given block.
// The plaintext is padded with zeros to a multiple of the block size.
func encryptECB(block cipher.Block, plaintext []byte) ([]byte, error) {
	blockSize := block.BlockSize()
	paddedLen := roundUp(len(plaintext), blockSize)
	padded := padBlockByte(plaintext, paddedLen, 0)
	ciphertext := make([]byte, paddedLen)
	for offset := 0; offset < paddedLen; offset += blockSize {
		block.Encrypt(ciphertext[offset:offset+blockSize], padded[offset:offset+blockSize])
	}
	return ciphertext, nil
}

// EncryptStandardPackage encrypts plaintext using Standard ECMA-376 encryption
// (AES-128 + SHA-1, ECB mode) and returns the EncryptionInfo and EncryptedPackage streams.
func EncryptStandardPackage(password string, plaintext []byte) (encryptionInfo, encryptedPackage []byte, err error) {
	salt, err := randBytes(16)
	if err != nil {
		return nil, nil, err
	}
	verifier, err := randBytes(16)
	if err != nil {
		return nil, nil, err
	}

	hashAlg := hashAlgorithm{name: "SHA-1", size: 20, new: sha1.New}
	cipherID := ecmaCipherAES128
	keyBits := 128
	keyBytes := keyBits / 8
	spinCount := standardEncryptionSpinCount

	pwHash := hashPassword(password, hashAlg, salt, spinCount)
	blockKey := []byte{0, 0, 0, 0}
	finalHash := generateKey(pwHash, hashAlg, blockKey, hashAlg.size)
	x1 := fillAndXor(finalHash, 0x36)
	x2 := fillAndXor(finalHash, 0x5C)
	keyData := append(append([]byte(nil), x1...), x2...)
	keyData = keyData[:keyBytes]

	block, err := aes.NewCipher(keyData)
	if err != nil {
		return nil, nil, err
	}

	encryptedVerifier, err := encryptECB(block, verifier)
	if err != nil {
		return nil, nil, err
	}

	verifierHash := digest(hashAlg, verifier)
	encryptedVerifierHash, err := encryptECB(block, padBlockByte(verifierHash, 32, 0))
	if err != nil {
		return nil, nil, err
	}

	// Build EncryptionInfo stream
	//
	// The standard EncryptionInfo structure (ECMA-376 §2.3.5.1) consists of:
	//   versionMajor (2) + versionMinor (2) + flags (4)
	//   headerSize (4) + headerFlags (4) + sizeExtra (4) + cipherID (4) +
	//   hashID (4) + keyBits (4) + providerType (4) + reserved1 (4) + reserved2 (4)
	//   [CSP name as wide string, only if saltSize != 16 at this offset]
	//   saltSize (4) + salt (16) + encryptedVerifier (16) + verifierHashSize (4) +
	//   encryptedVerifierHash (32 for SHA-1)
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint16(4))  // versionMajor
	binary.Write(&buf, binary.LittleEndian, uint16(2))  // versionMinor
	binary.Write(&buf, binary.LittleEndian, uint32(0x24)) // flags (AES)

	// Header – 32 bytes after headerSize. When CSP name is empty we omit it
	// and place saltSize (= 16) immediately after the header so the reader
	// detects checkForSalt == 16 and skips CSP-name parsing.
	binary.Write(&buf, binary.LittleEndian, uint32(32)) // headerSize
	binary.Write(&buf, binary.LittleEndian, uint32(0x00000024)) // headerFlags
	binary.Write(&buf, binary.LittleEndian, uint32(0))          // sizeExtra
	binary.Write(&buf, binary.LittleEndian, uint32(cipherID))   // cipherID
	binary.Write(&buf, binary.LittleEndian, uint32(ecmaHashSHA1)) // hashID
	binary.Write(&buf, binary.LittleEndian, uint32(keyBits))    // keyBits
	binary.Write(&buf, binary.LittleEndian, uint32(0x18))       // providerType (AES)
	binary.Write(&buf, binary.LittleEndian, uint32(0))          // reserved1
	binary.Write(&buf, binary.LittleEndian, uint32(0))          // reserved2

	// Verifier – no CSP name when saltSize is 16
	binary.Write(&buf, binary.LittleEndian, uint32(16)) // saltSize
	buf.Write(salt)
	buf.Write(encryptedVerifier)
	binary.Write(&buf, binary.LittleEndian, uint32(20)) // verifierHashSize
	buf.Write(encryptedVerifierHash)

	encryptionInfo = buf.Bytes()

	// Encrypt package
	cipherLen := roundUp(len(plaintext), aes.BlockSize)
	paddedPlaintext := padBlockByte(plaintext, cipherLen, 0)
	encryptedData, err := encryptECB(block, paddedPlaintext)
	if err != nil {
		return nil, nil, err
	}

	encryptedPackage = make([]byte, 8+len(encryptedData))
	binary.LittleEndian.PutUint64(encryptedPackage, uint64(len(plaintext)))
	copy(encryptedPackage[8:], encryptedData)

	return encryptionInfo, encryptedPackage, nil
}

// EncryptAgilePackage encrypts plaintext using Agile ECMA-376 encryption
// (AES-256 + SHA-512, CBC mode, 4096-byte segments) and returns the
// EncryptionInfo and EncryptedPackage streams.
func EncryptAgilePackage(password string, plaintext []byte) (encryptionInfo, encryptedPackage []byte, err error) {
	blockSize := 16
	keyBits := 256
	keyBytes := keyBits / 8
	spinCount := 100000

	hashAlg := hashAlgorithm{name: "SHA-512", size: 64, new: sha512.New}
	cipherAlg := "AES"
	chaining := "ChainingModeCBC"

	keySalt, err := randBytes(blockSize)
	if err != nil {
		return nil, nil, err
	}
	verifierSalt, err := randBytes(blockSize)
	if err != nil {
		return nil, nil, err
	}
	verifierInput, err := randBytes(blockSize)
	if err != nil {
		return nil, nil, err
	}
	keySpec, err := randBytes(keyBytes)
	if err != nil {
		return nil, nil, err
	}
	integritySalt, err := randBytes(hashAlg.size)
	if err != nil {
		return nil, nil, err
	}

	pwHash := hashPassword(password, hashAlg, verifierSalt, spinCount)

	// encryptedVerifierHashInput
	verifierKey := generateKey(pwHash, hashAlg, agileVerifierInputBlock, keyBytes)
	verifierIV := generateIV(hashAlg, verifierSalt, nil, blockSize)
	encryptedVerifierHashInput, err := encryptAESBlockMode(verifierKey, verifierIV, padBlock0(verifierInput, blockSize))
	if err != nil {
		return nil, nil, err
	}

	// encryptedVerifierHashValue
	hashedVerifier := digest(hashAlg, verifierInput)
	hashKey := generateKey(pwHash, hashAlg, agileVerifierHashBlock, keyBytes)
	hashIV := generateIV(hashAlg, verifierSalt, nil, blockSize)
	encryptedVerifierHashValue, err := encryptAESBlockMode(hashKey, hashIV, padBlock0(hashedVerifier, blockSize))
	if err != nil {
		return nil, nil, err
	}

	// encryptedKeyValue
	packageKeyKey := generateKey(pwHash, hashAlg, agileCryptoKeyBlock, keyBytes)
	packageKeyIV := generateIV(hashAlg, verifierSalt, nil, blockSize)
	encryptedKeyValue, err := encryptAESBlockMode(packageKeyKey, packageKeyIV, padBlock0(keySpec, blockSize))
	if err != nil {
		return nil, nil, err
	}

	// DataIntegrity: encryptedHmacKey
	integrityKeyIV := generateIV(hashAlg, keySalt, agileIntegrityKeyBlock, blockSize)
	encryptedHmacKey, err := encryptAESBlockMode(keySpec, integrityKeyIV, padBlock0(integritySalt, blockSize))
	if err != nil {
		return nil, nil, err
	}

	// Encrypt package in 4096-byte segments
	encryptedData := make([]byte, 0, roundUp(len(plaintext), blockSize))
	remaining := len(plaintext)
	for blockIndex := 0; remaining > 0; blockIndex++ {
		chunkLen := remaining
		if chunkLen > agileSegmentSize {
			chunkLen = agileSegmentSize
		}
		paddedChunk := padBlock0(plaintext[len(plaintext)-remaining:len(plaintext)-remaining+chunkLen], blockSize)
		iv := generateIV(hashAlg, keySalt, littleEndianUint32(uint32(blockIndex)), blockSize)
		encryptedChunk, err := encryptAESBlockMode(keySpec, iv, paddedChunk)
		if err != nil {
			return nil, nil, err
		}
		encryptedData = append(encryptedData, encryptedChunk...)
		remaining -= chunkLen
	}

	encryptedPackage = make([]byte, 8+len(encryptedData))
	binary.LittleEndian.PutUint64(encryptedPackage, uint64(len(plaintext)))
	copy(encryptedPackage[8:], encryptedData)

	// Compute integrity HMAC
	integrityValueIV := generateIV(hashAlg, keySalt, agileIntegrityValueBlock, blockSize)
	hmacValue := computeHMAC(hashAlg, integritySalt, encryptedPackage)
	encryptedHmacValue, err := encryptAESBlockMode(keySpec, integrityValueIV, padBlock0(hmacValue, blockSize))
	if err != nil {
		return nil, nil, err
	}

	// Build EncryptionInfo XML manually to ensure correct namespaces.
	xmlStr := fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
		`<encryption xmlns="%s" xmlns:p="%s">`+
		`<keyData saltSize="%d" blockSize="%d" keyBits="%d" hashSize="%d" cipherAlgorithm="%s" cipherChaining="%s" hashAlgorithm="%s" saltValue="%s"/>`+
		`<dataIntegrity encryptedHmacKey="%s" encryptedHmacValue="%s"/>`+
		`<keyEncryptors>`+
		`<keyEncryptor uri="%s">`+
		`<p:encryptedKey spinCount="%d" saltSize="%d" blockSize="%d" keyBits="%d" hashSize="%d" cipherAlgorithm="%s" cipherChaining="%s" hashAlgorithm="%s" saltValue="%s" encryptedVerifierHashInput="%s" encryptedVerifierHashValue="%s" encryptedKeyValue="%s"/>`+
		`</keyEncryptor>`+
		`</keyEncryptors>`+
		`</encryption>`,
		agileEncryptionNS, agilePasswordNS,
		blockSize, blockSize, keyBits, hashAlg.size, cipherAlg, chaining, hashAlg.name, base64.StdEncoding.EncodeToString(keySalt),
		base64.StdEncoding.EncodeToString(encryptedHmacKey),
		base64.StdEncoding.EncodeToString(encryptedHmacValue),
		agilePasswordNS,
		spinCount, blockSize, blockSize, keyBits, hashAlg.size, cipherAlg, chaining, hashAlg.name, base64.StdEncoding.EncodeToString(verifierSalt),
		base64.StdEncoding.EncodeToString(encryptedVerifierHashInput),
		base64.StdEncoding.EncodeToString(encryptedVerifierHashValue),
		base64.StdEncoding.EncodeToString(encryptedKeyValue),
	)

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint16(4))  // versionMajor
	binary.Write(&buf, binary.LittleEndian, uint16(4))  // versionMinor
	binary.Write(&buf, binary.LittleEndian, uint32(0x40)) // flags
	buf.WriteString(xmlStr)

	encryptionInfo = buf.Bytes()
	return encryptionInfo, encryptedPackage, nil
}

var (
	agileIntegrityKeyBlock   = []byte{0x5F, 0xB2, 0xAD, 0x01, 0x0C, 0xB9, 0xE1, 0xF6}
	agileIntegrityValueBlock = []byte{0xA0, 0x67, 0x7F, 0x02, 0xB2, 0x2C, 0x84, 0x33}
)

func computeHMAC(hashAlg hashAlgorithm, key, data []byte) []byte {
	h := hmac.New(hashAlg.new, key)
	h.Write(data)
	return h.Sum(nil)
}

// WriteEncryptedAgilePackage encrypts the OOXML zip plaintext with Agile
// ECMA-376 encryption (AES-256 + SHA-512) and returns a complete CFB-wrapped
// container ready to be written to disk as a .docx/.xlsx/.pptx file.
//
// The container contains the EncryptionInfo and EncryptedPackage streams
// expected by Office 2010+ and other ECMA-376-compliant readers.
func WriteEncryptedAgilePackage(password string, plaintext []byte) ([]byte, error) {
	info, encrypted, err := EncryptAgilePackage(password, plaintext)
	if err != nil {
		return nil, err
	}
	return wrapEncryptedStreams(info, encrypted)
}

// WriteEncryptedStandardPackage encrypts the OOXML zip plaintext with Standard
// ECMA-376 encryption (AES-128 + SHA-1, ECB) and returns a complete CFB-wrapped
// container ready to be written to disk. Prefer WriteEncryptedAgilePackage for
// new code; the standard mode exists for compatibility with older readers.
func WriteEncryptedStandardPackage(password string, plaintext []byte) ([]byte, error) {
	info, encrypted, err := EncryptStandardPackage(password, plaintext)
	if err != nil {
		return nil, err
	}
	return wrapEncryptedStreams(info, encrypted)
}

func wrapEncryptedStreams(info, encrypted []byte) ([]byte, error) {
	w := cfb.NewWriter()
	if err := w.AddStream(oleEncryptionInfoStreamName, info); err != nil {
		return nil, fmt.Errorf("opc: wrap EncryptionInfo: %w", err)
	}
	if err := w.AddStream(oleEncryptedPackageStreamName, encrypted); err != nil {
		return nil, fmt.Errorf("opc: wrap EncryptedPackage: %w", err)
	}
	return w.Bytes()
}
