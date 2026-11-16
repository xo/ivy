package offcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash"
	"testing"
)

// agileEncrypt encrypts plain with Agile encryption, as Office does, and
// returns the EncryptionInfo and EncryptedPackage streams. If spinCount is
// larger than 100000, the verifier is random, so that a test of the limits
// does not wait for the hash.
func agileEncrypt(t *testing.T, password, hashName string, keyBits, spinCount int, plain []byte) ([]byte, []byte) {
	t.Helper()
	var newHash func() hash.Hash
	switch hashName {
	case "SHA1":
		newHash = sha1.New
	case "SHA256":
		newHash = sha256.New
	case "SHA384":
		newHash = sha512.New384
	case "SHA512":
		newHash = sha512.New
	case "MD5":
		newHash = md5.New
	}
	hashSize := newHash().Size()
	dataSalt, pwSalt := randBytes(16), randBytes(16)
	secret := randBytes(keyBits / 8)
	encrypt := func(key, iv, src []byte) []byte {
		b, err := aes.NewCipher(key)
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, (len(src)+15)/16*16)
		copy(buf, src)
		cipher.NewCBCEncrypter(b, iv).CryptBlocks(buf, buf)
		return buf
	}
	// The key of the password.
	h := newHash()
	h.Write(pwSalt)
	h.Write(passwordBytes(password))
	hn := h.Sum(nil)
	if spinCount <= 100000 {
		for i := range spinCount {
			h.Reset()
			h.Write(binary.LittleEndian.AppendUint32(nil, uint32(i)))
			h.Write(hn)
			hn = h.Sum(nil)
		}
	}
	derive := func(block []byte) []byte {
		h := newHash()
		h.Write(hn)
		h.Write(block)
		return fit(h.Sum(nil), keyBits/8)
	}
	iv := fit(pwSalt, 16)
	verifier := randBytes(16)
	vh := newHash()
	vh.Write(verifier)
	encVerifierInput := encrypt(derive(blockVerifierInput), iv, verifier)
	encVerifierValue := encrypt(derive(blockVerifierValue), iv, vh.Sum(nil))
	encKeyValue := encrypt(derive(blockKeyValue), iv, secret)
	// The package.
	dataIV := func(block []byte) []byte {
		h := newHash()
		h.Write(dataSalt)
		h.Write(block)
		return fit(h.Sum(nil), 16)
	}
	pkg := binary.LittleEndian.AppendUint64(nil, uint64(len(plain)))
	for i := 0; i*segmentSize < len(plain); i++ {
		seg := plain[i*segmentSize : min(len(plain), (i+1)*segmentSize)]
		pkg = append(pkg, encrypt(secret, dataIV(le32(uint32(i))), seg)...)
	}
	hmacKey := randBytes(hashSize)
	mac := hmac.New(newHash, hmacKey)
	mac.Write(pkg)
	encHmacKey := encrypt(secret, dataIV(blockHmacKey), hmacKey)
	encHmacValue := encrypt(secret, dataIV(blockHmacValue), mac.Sum(nil))
	b64 := base64.StdEncoding.EncodeToString
	params := func(salt []byte) string {
		return fmt.Sprintf(`saltSize="16" blockSize="16" keyBits="%d" hashSize="%d" cipherAlgorithm="AES" cipherChaining="ChainingModeCBC" hashAlgorithm="%s" saltValue="%s"`,
			keyBits, hashSize, hashName, b64(salt))
	}
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<encryption xmlns="http://schemas.microsoft.com/office/2006/encryption" xmlns:p="http://schemas.microsoft.com/office/2006/keyEncryptor/password">
<keyData %s/>
<dataIntegrity encryptedHmacKey="%s" encryptedHmacValue="%s"/>
<keyEncryptors><keyEncryptor uri="%s"><p:encryptedKey spinCount="%d" %s encryptedVerifierHashInput="%s" encryptedVerifierHashValue="%s" encryptedKeyValue="%s"/></keyEncryptor></keyEncryptors>
</encryption>`,
		params(dataSalt), b64(encHmacKey), b64(encHmacValue), passwordURI, spinCount, params(pwSalt),
		b64(encVerifierInput), b64(encVerifierValue), b64(encKeyValue))
	info := append([]byte{4, 0, 4, 0, 0x40, 0, 0, 0}, xml...)
	return info, pkg
}

// randBytes returns n random bytes.
func randBytes(n int) []byte {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return buf
}
