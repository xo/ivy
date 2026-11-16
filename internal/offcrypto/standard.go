package offcrypto

import (
	"bytes"
	"crypto/aes"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"fmt"

	"github.com/xo/ivy"
)

// The flags of the encryption header (MS-OFFCRYPTO 2.3.1).
const (
	flagCryptoAPI = 0x04
	flagExternal  = 0x10
	flagAES       = 0x20
)

// The identifiers of the algorithms of CryptoAPI.
const (
	algAES128 = 0x660E
	algAES192 = 0x660F
	algAES256 = 0x6610
	algRC4    = 0x6801
	algSHA1   = 0x8004
)

// standardSpinCount is the number of times that Standard encryption hashes
// the password (MS-OFFCRYPTO 2.3.4.7).
const standardSpinCount = 50000

// standardInfo holds the encryption header and the verifier of Standard
// encryption and of RC4 CryptoAPI encryption, which share their layout.
type standardInfo struct {
	// rc4 is true for RC4 CryptoAPI, and false for AES.
	rc4 bool
	// keyBits is the size of the key, in bits.
	keyBits int

	salt                  []byte
	encryptedVerifier     []byte
	encryptedVerifierHash []byte
}

// parseStandard parses the part of the stream after the version: the
// flags, the size of the header, the header and the verifier
// (MS-OFFCRYPTO 2.3.4.5 and 2.3.5.1).
func parseStandard(buf []byte) (*standardInfo, error) {
	if len(buf) < 8 {
		return nil, fmt.Errorf("standard encryption header of %d bytes: %w", len(buf), ivy.ErrFormat)
	}
	flags := binary.LittleEndian.Uint32(buf)
	headerSize := binary.LittleEndian.Uint32(buf[4:])
	buf = buf[8:]
	if headerSize < 32 || uint64(headerSize) > uint64(len(buf)) {
		return nil, fmt.Errorf("standard encryption header of %d bytes: %w", headerSize, ivy.ErrFormat)
	}
	header, verifier := buf[:headerSize], buf[headerSize:]
	if flags&flagExternal != 0 {
		return nil, fmt.Errorf("extensible encryption: %w", ivy.ErrUnsupported)
	}
	if flags&flagCryptoAPI == 0 {
		return nil, fmt.Errorf("encryption without cryptoapi: %w", ivy.ErrFormat)
	}
	alg := binary.LittleEndian.Uint32(header[8:])
	algHash := binary.LittleEndian.Uint32(header[12:])
	keyBits := int(binary.LittleEndian.Uint32(header[16:]))
	if alg == 0 {
		alg = algRC4
		if flags&flagAES != 0 {
			alg = algAES128
		}
	}
	if algHash != 0 && algHash != algSHA1 {
		return nil, fmt.Errorf("hash algorithm %#x: %w", algHash, ivy.ErrUnsupported)
	}
	si := &standardInfo{}
	switch alg {
	case algAES128, algAES192, algAES256:
		want := 128 + 64*int(alg-algAES128)
		if keyBits == 0 {
			keyBits = want
		}
		if keyBits != want {
			return nil, fmt.Errorf("aes key of %d bits: %w", keyBits, ivy.ErrFormat)
		}
	case algRC4:
		si.rc4 = true
		if keyBits == 0 {
			keyBits = 40
		}
		if keyBits < 40 || keyBits > 128 || keyBits%8 != 0 {
			return nil, fmt.Errorf("rc4 key of %d bits: %w", keyBits, ivy.ErrFormat)
		}
	default:
		return nil, fmt.Errorf("cipher algorithm %#x: %w", alg, ivy.ErrUnsupported)
	}
	si.keyBits = keyBits
	// The verifier: the size of the salt, the salt, the encrypted verifier,
	// the size of the hash, and the encrypted hash.
	hashLen := 32
	if si.rc4 {
		hashLen = sha1.Size
	}
	if len(verifier) < 4+16+16+4+hashLen {
		return nil, fmt.Errorf("encryption verifier of %d bytes: %w", len(verifier), ivy.ErrFormat)
	}
	if saltSize := binary.LittleEndian.Uint32(verifier); saltSize != 16 {
		return nil, fmt.Errorf("salt of %d bytes: %w", saltSize, ivy.ErrFormat)
	}
	si.salt = verifier[4:20]
	si.encryptedVerifier = verifier[20:36]
	si.encryptedVerifierHash = verifier[40 : 40+hashLen]
	return si, nil
}

// key derives the key of the password and makes sure that it decrypts the
// verifier (MS-OFFCRYPTO 2.3.4.7, 2.3.4.9, 2.3.5.2 and 2.3.5.6).
func (si *standardInfo) key(password string) (*Key, error) {
	h := sha1.New()
	h.Write(si.salt)
	h.Write(passwordBytes(password))
	h0 := h.Sum(nil)
	if si.rc4 {
		k := &Key{scheme: RC4CryptoAPI, secret: h0, keyLen: si.keyBits / 8, standard: si}
		c, err := k.Stream(0)
		if err != nil {
			return nil, err
		}
		verifier := make([]byte, len(si.encryptedVerifier))
		c.XORKeyStream(verifier, si.encryptedVerifier)
		hash := make([]byte, len(si.encryptedVerifierHash))
		c.XORKeyStream(hash, si.encryptedVerifierHash)
		sum := sha1.Sum(verifier)
		if !equal(sum[:], hash, sha1.Size) {
			return nil, fmt.Errorf("rc4 cryptoapi encryption: %w", ivy.ErrPassword)
		}
		return k, nil
	}
	hn := h0
	var it [4]byte
	for i := range uint32(standardSpinCount) {
		binary.LittleEndian.PutUint32(it[:], i)
		h.Reset()
		h.Write(it[:])
		h.Write(hn)
		hn = h.Sum(hn[:0])
	}
	h.Reset()
	h.Write(hn)
	h.Write(le32(0))
	hfinal := h.Sum(nil)
	// The key is the hash of each of two pads with the final hash in it.
	x := make([]byte, 0, 2*sha1.Size)
	for _, pad := range []byte{0x36, 0x5c} {
		buf := bytes.Repeat([]byte{pad}, 64)
		for i, b := range hfinal {
			buf[i] ^= b
		}
		sum := sha1.Sum(buf)
		x = append(x, sum[:]...)
	}
	secret := x[:si.keyBits/8]
	verifier, err := ecbDecrypt(secret, si.encryptedVerifier)
	if err != nil {
		return nil, err
	}
	hash, err := ecbDecrypt(secret, si.encryptedVerifierHash)
	if err != nil {
		return nil, err
	}
	sum := sha1.Sum(verifier)
	if !equal(sum[:], hash, sha1.Size) {
		return nil, fmt.Errorf("standard encryption: %w", ivy.ErrPassword)
	}
	return &Key{scheme: Standard, secret: secret, standard: si}, nil
}

// ecbDecrypt decrypts src with AES in the mode ECB.
func ecbDecrypt(key, src []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes key of %d bytes: %w", len(key), ivy.ErrFormat)
	}
	if len(src)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("aes data of %d bytes: %w", len(src), ivy.ErrFormat)
	}
	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += aes.BlockSize {
		b.Decrypt(dst[i:], src[i:])
	}
	return dst, nil
}

// rc4Cipher returns the RC4 cipher of key.
func rc4Cipher(key []byte) (*rc4.Cipher, error) {
	c, err := rc4.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("rc4 key of %d bytes: %w", len(key), ivy.ErrFormat)
	}
	return c, nil
}
