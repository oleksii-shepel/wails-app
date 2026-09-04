// Package crypt provides shared cryptographic primitives for legacy Office
// binary format encryption (XOR obfuscation, Binary RC4, CryptoAPI RC4).
package crypt

import (
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"unicode/utf16"
)

// XOR constants from POI CryptoFunctions.
var (
	XorInitialCodeArray = []int{
		0xE1F0, 0x1D0F, 0xCC9C, 0x84C0, 0x110C, 0x0E10, 0xF1CE,
		0x313E, 0x1872, 0xE139, 0xD40F, 0x84F9, 0x280C, 0xA96A,
		0x4EC3,
	}
	XorPadArray = []byte{
		0xBB, 0xFF, 0xFF, 0xBA, 0xFF,
		0xFF, 0xB9, 0x80, 0x00, 0xBE,
		0x0F, 0x00, 0xBF, 0x0F, 0x00,
	}
	XorEncryptionMatrix = [][]int{
		{0xAEFC, 0x4DD9, 0x9BB2, 0x2745, 0x4E8A, 0x9D14, 0x2A09},
		{0x7B61, 0xF6C2, 0xFDA5, 0xEB6B, 0xC6F7, 0x9DCF, 0x2BBF},
		{0x4563, 0x8AC6, 0x05AD, 0x0B5A, 0x16B4, 0x2D68, 0x5AD0},
		{0x0375, 0x06EA, 0x0DD4, 0x1BA8, 0x3750, 0x6EA0, 0xDD40},
		{0xD849, 0xA0B3, 0x5147, 0xA28E, 0x553D, 0xAA7A, 0x44D5},
		{0x6F45, 0xDE8A, 0xAD35, 0x4A4B, 0x9496, 0x390D, 0x721A},
		{0xEB23, 0xC667, 0x9CEF, 0x29FF, 0x53FE, 0xA7FC, 0x5FD9},
		{0x47D3, 0x8FA6, 0x0F6D, 0x1EDA, 0x3DB4, 0x7B68, 0xF6D0},
		{0xB861, 0x60E3, 0xC1C6, 0x93AD, 0x377B, 0x6EF6, 0xDDEC},
		{0x45A0, 0x8B40, 0x06A1, 0x0D42, 0x1A84, 0x3508, 0x6A10},
		{0xAA51, 0x4483, 0x8906, 0x022D, 0x045A, 0x08B4, 0x1168},
		{0x76B4, 0xED68, 0xCAF1, 0x85C3, 0x1BA7, 0x374E, 0x6E9C},
		{0x3730, 0x6E60, 0xDCC0, 0xA9A1, 0x4363, 0x86C6, 0x1DAD},
		{0x3331, 0x6662, 0xCCC4, 0x89A9, 0x0373, 0x06E6, 0x0DCC},
		{0x1021, 0x2042, 0x4084, 0x8108, 0x1231, 0x2462, 0x48C4},
	}
)

// ToAnsiPassword converts a Unicode password to single-byte ANSI characters.
// For each rune, if the low byte is non-zero it is used; otherwise the high byte.
func ToAnsiPassword(password string) []byte {
	arr := make([]byte, len(password))
	for i, c := range password {
		low := byte(c & 0xFF)
		high := byte((c >> 8) & 0xFF)
		if low != 0 {
			arr[i] = low
		} else {
			arr[i] = high
		}
	}
	return arr
}

// RotateLeftBase15Bit rotates a 15-bit value left by 1 bit.
func RotateLeftBase15Bit(v int) int {
	intermediate1 := 0
	if (v & 0x4000) != 0 {
		intermediate1 = 1
	}
	intermediate2 := (v << 1) & 0x7FFF
	return intermediate1 | intermediate2
}

// RotateLeftByte rotates a byte left by the given number of bits.
func RotateLeftByte(bits byte, shift int) byte {
	return (bits << shift) | (bits >> (8 - shift))
}

// CreateXorVerifier1 implements POI CryptoFunctions.createXorVerifier1.
func CreateXorVerifier1(password string) int {
	arr := ToAnsiPassword(password)
	verifier := 0
	if len(password) > 0 {
		for i := len(arr) - 1; i >= 0; i-- {
			verifier = RotateLeftBase15Bit(verifier)
			verifier ^= int(arr[i])
		}
		verifier = RotateLeftBase15Bit(verifier)
		verifier ^= len(arr)
		verifier ^= 0xCE4B
	}
	return verifier & 0xFFFF
}

// CreateXorKey1 implements POI CryptoFunctions.createXorKey1.
func CreateXorKey1(password string) int {
	v := CreateXorVerifier2(password)
	return (v >> 16) & 0xFFFF
}

// CreateXorVerifier2 implements POI CryptoFunctions.createXorVerifier2.
func CreateXorVerifier2(password string) int {
	const maxPasswordLength = 15
	if len(password) > maxPasswordLength {
		password = password[:maxPasswordLength]
	}
	arr := ToAnsiPassword(password)
	generatedKey := make([]byte, 4)
	if len(arr) > 0 {
		highOrderWord := XorInitialCodeArray[len(arr)-1]
		line := maxPasswordLength - len(arr)
		for _, ch := range arr {
			b := int(ch)
			for _, xor := range XorEncryptionMatrix[line] {
				if (b & 1) == 1 {
					highOrderWord ^= xor
				}
				b >>= 1
			}
			line++
		}
		verifier := CreateXorVerifier1(password)
		generatedKey[0] = byte(verifier & 0xFF)
		generatedKey[1] = byte((verifier >> 8) & 0xFF)
		generatedKey[2] = byte(highOrderWord & 0xFF)
		generatedKey[3] = byte((highOrderWord >> 8) & 0xFF)
	}
	return int(uint32(generatedKey[0]) | uint32(generatedKey[1])<<8 |
		uint32(generatedKey[2])<<16 | uint32(generatedKey[3])<<24)
}

// CreateXorArray1 implements POI CryptoFunctions.createXorArray1.
func CreateXorArray1(password string) []byte {
	if len(password) > 15 {
		password = password[:15]
	}
	passBytes := []byte(password)
	obfuscationArray := make([]byte, 16)
	copy(obfuscationArray, passBytes)
	if len(passBytes) == 0 {
		copy(obfuscationArray[len(passBytes):], XorPadArray)
	} else {
		copy(obfuscationArray[len(passBytes):], XorPadArray[:len(XorPadArray)-len(passBytes)+1])
	}
	xorKey := CreateXorKey1(password)
	baseKeyLE := []byte{byte(xorKey & 0xFF), byte((xorKey >> 8) & 0xFF)}
	for i := 0; i < 16; i++ {
		obfuscationArray[i] ^= baseKeyLE[i&1]
		obfuscationArray[i] = RotateLeftByte(obfuscationArray[i], 2)
	}
	return obfuscationArray
}

// UTF16LEBytes encodes a string as UTF-16LE bytes.
func UTF16LEBytes(s string) []byte {
	runes := utf16.Encode([]rune(s))
	out := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(out[i*2:], r)
	}
	return out
}

// HashAlgorithm identifies the hash algorithm for CryptoAPI.
type HashAlgorithm int

const (
	HashMD5 HashAlgorithm = iota
	HashSHA1
)

// DigestBytes computes the hash of data using the specified algorithm.
func DigestBytes(hashAlg HashAlgorithm, data []byte) []byte {
	switch hashAlg {
	case HashMD5:
		sum := md5.Sum(data)
		return append([]byte(nil), sum[:]...)
	case HashSHA1:
		sum := sha1.Sum(data)
		return append([]byte(nil), sum[:]...)
	default:
		return nil
	}
}

// TruncateOrPadZero truncates or zero-pads a byte slice to the given size.
func TruncateOrPadZero(data []byte, size int) []byte {
	out := make([]byte, size)
	copy(out, data)
	return out
}

// EqualBytes returns true if two byte slices are equal.
func EqualBytes(a, b []byte) bool {
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

// DeriveBinaryRC4PasswordKey derives the base key for Binary RC4.
func DeriveBinaryRC4PasswordKey(password string, salt []byte) []byte {
	if len(password) > 255 {
		password = password[:255]
	}
	pwHash := md5.Sum(UTF16LEBytes(password))
	buf := make([]byte, 0, 16*(5+len(salt)))
	for i := 0; i < 16; i++ {
		buf = append(buf, pwHash[:5]...)
		buf = append(buf, salt...)
	}
	sum := md5.Sum(buf)
	return append([]byte(nil), sum[:5]...)
}

// DeriveBinaryRC4BlockKey derives the per-block key for Binary RC4.
func DeriveBinaryRC4BlockKey(keyMaterial []byte, block int) []byte {
	blockLE := make([]byte, 4)
	binary.LittleEndian.PutUint32(blockLE, uint32(block))
	sum := md5.Sum(append(append([]byte(nil), keyMaterial...), blockLE...))
	return append([]byte(nil), sum[:]...)
}

// DeriveCryptoAPIPasswordKey derives the base key for CryptoAPI RC4.
func DeriveCryptoAPIPasswordKey(password string, hashAlg HashAlgorithm, salt []byte) []byte {
	return DigestBytes(hashAlg, append(append([]byte(nil), salt...), UTF16LEBytes(password)...))
}

// DeriveCryptoAPIBlockKey derives the per-block key for CryptoAPI RC4.
func DeriveCryptoAPIBlockKey(keyMaterial []byte, hashAlg HashAlgorithm, keyBits int, block int) []byte {
	blockLE := make([]byte, 4)
	binary.LittleEndian.PutUint32(blockLE, uint32(block))
	hash := DigestBytes(hashAlg, append(append([]byte(nil), keyMaterial...), blockLE...))
	keyBytes := keyBits / 8
	if keyBytes <= 0 {
		keyBytes = 5
	}
	key := TruncateOrPadZero(hash, keyBytes)
	if keyBits == 40 {
		key = TruncateOrPadZero(key, 16)
	}
	return key
}

// VerifyBinaryRC4Password verifies a Binary RC4 password and returns the key material.
func VerifyBinaryRC4Password(password string, salt, encryptedVerifier, encryptedVerifierHash []byte) ([]byte, error) {
	keyMaterial := DeriveBinaryRC4PasswordKey(password, salt)
	blockKey := DeriveBinaryRC4BlockKey(keyMaterial, 0)
	cipher, err := rc4.NewCipher(blockKey)
	if err != nil {
		return nil, err
	}
	verifier := make([]byte, len(encryptedVerifier))
	cipher.XORKeyStream(verifier, encryptedVerifier)
	verifierHash := make([]byte, len(encryptedVerifierHash))
	cipher.XORKeyStream(verifierHash, encryptedVerifierHash)
	sum := md5.Sum(verifier)
	if !EqualBytes(sum[:], verifierHash) {
		return nil, nil
	}
	return keyMaterial, nil
}

// VerifyCryptoAPIPassword verifies a CryptoAPI RC4 password and returns the key material.
func VerifyCryptoAPIPassword(password string, hashAlg HashAlgorithm, keyBits int, salt, encryptedVerifier, encryptedVerifierHash []byte, verifierHashSize int) ([]byte, error) {
	keyMaterial := DeriveCryptoAPIPasswordKey(password, hashAlg, salt)
	blockKey := DeriveCryptoAPIBlockKey(keyMaterial, hashAlg, keyBits, 0)
	cipher, err := rc4.NewCipher(blockKey)
	if err != nil {
		return nil, err
	}
	verifier := make([]byte, len(encryptedVerifier))
	cipher.XORKeyStream(verifier, encryptedVerifier)
	verifierHash := make([]byte, len(encryptedVerifierHash))
	cipher.XORKeyStream(verifierHash, encryptedVerifierHash)
	sum := DigestBytes(hashAlg, verifier)
	n := verifierHashSize
	if n <= 0 || n > len(sum) {
		n = len(sum)
	}
	if n > len(verifierHash) {
		n = len(verifierHash)
	}
	if !EqualBytes(sum[:n], verifierHash[:n]) {
		return nil, nil
	}
	return keyMaterial, nil
}
