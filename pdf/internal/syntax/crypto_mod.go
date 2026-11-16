// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports hayro-syntax src/crypto/mod.rs, which upstream ported from
// pdf.js.
//
// Port: the modules aes, md5, rc4, sha256, sha384 and sha512 of upstream
// become crypto/aes, crypto/cipher, crypto/md5, crypto/rc4, crypto/sha256 and
// crypto/sha512 (D16). The derivation of the keys and the standard security
// handler stay here.

package syntax

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"

	"github.com/xo/ivy"
)

// passwordPadding is PASSWORD_PADDING in hayro-syntax src/crypto/mod.rs.
var passwordPadding = [32]byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

// DecryptionError is DecryptionError in hayro-syntax src/crypto/mod.rs. It
// is an error of the decryption.
//
// Port: each case wraps a sentinel error of ivy: ivy.ErrPassword for
// PasswordProtected, ivy.ErrUnsupported for UnsupportedAlgorithm, and
// ivy.ErrFormat for the others.
type DecryptionError uint8

// The cases of DecryptionError in hayro-syntax src/crypto/mod.rs.
const (
	// DecryptionMissingIDEntry is DecryptionError::MissingIDEntry: the
	// entry ID is missing.
	DecryptionMissingIDEntry DecryptionError = iota + 1
	// DecryptionPasswordProtected is DecryptionError::PasswordProtected: the
	// password is missing or wrong.
	DecryptionPasswordProtected
	// DecryptionInvalidEncryption is DecryptionError::InvalidEncryption: the
	// encryption is not valid.
	DecryptionInvalidEncryption
	// DecryptionUnsupportedAlgorithm is
	// DecryptionError::UnsupportedAlgorithm: the encryption uses an
	// algorithm that the package does not support.
	DecryptionUnsupportedAlgorithm
)

// Error returns the message of the error.
func (e DecryptionError) Error() string {
	switch e {
	case DecryptionMissingIDEntry:
		return "decrypting pdf: missing id entry: " + ivy.ErrFormat.Error()
	case DecryptionPasswordProtected:
		return "decrypting pdf: " + ivy.ErrPassword.Error()
	case DecryptionUnsupportedAlgorithm:
		return "decrypting pdf: unsupported algorithm: " + ivy.ErrUnsupported.Error()
	}
	return "decrypting pdf: invalid encryption: " + ivy.ErrFormat.Error()
}

// Unwrap returns the sentinel error of ivy for the case.
func (e DecryptionError) Unwrap() error {
	switch e {
	case DecryptionPasswordProtected:
		return ivy.ErrPassword
	case DecryptionUnsupportedAlgorithm:
		return ivy.ErrUnsupported
	}
	return ivy.ErrFormat
}

// decryptorTag is DecryptorTag in hayro-syntax src/crypto/mod.rs.
type decryptorTag uint8

const (
	tagNone decryptorTag = iota
	tagRc4
	tagAes128
	tagAes256
)

// decryptorTagFromName is DecryptorTag::from_name in hayro-syntax
// src/crypto/mod.rs.
func decryptorTagFromName(name Name) (decryptorTag, bool) {
	switch name.AsStr() {
	case "None", "Identity":
		return tagNone, true
	case "V2":
		return tagRc4, true
	case "AESV2":
		return tagAes128, true
	case "AESV3":
		return tagAes256, true
	}
	return 0, false
}

// decryptor is Decryptor in hayro-syntax src/crypto/mod.rs.
//
// Port: the Rust enum becomes a struct. kind selects the case, and dict is
// set for the cases Aes128 and Aes256.
type decryptor struct {
	kind decryptorTag
	key  []byte
	dict *decryptorData
}

// targetKind is the case of a decryptionTarget.
type targetKind uint8

const (
	targetString targetKind = iota
	targetStream
	targetNamed
)

// decryptionTarget is DecryptionTarget in hayro-syntax src/crypto/mod.rs.
type decryptionTarget struct {
	kind targetKind
	name Name
}

// decrypt is Decryptor::decrypt in hayro-syntax src/crypto/mod.rs.
func (d *decryptor) decrypt(id ObjectIdentifier, data []byte, target decryptionTarget) ([]byte, bool) {
	if target.kind == targetNamed && target.name == KeyIdentity {
		return bytes.Clone(data), true
	}
	switch d.kind {
	case tagNone:
		return bytes.Clone(data), true
	case tagRc4:
		if target.kind == targetNamed {
			return nil, false
		}
		return decryptRc4(d.key, data, id)
	}
	var crypt cryptDictionary
	switch target.kind {
	case targetString:
		crypt = d.dict.stringFilter
	case targetStream:
		crypt = d.dict.streamFilter
	default:
		c, ok := d.dict.cryptFilters[target.name]
		if !ok {
			return nil, false
		}
		crypt = c
	}
	switch crypt.cfm {
	case tagNone:
		return bytes.Clone(data), true
	case tagRc4:
		return decryptRc4(d.key, data, id)
	case tagAes128:
		return decryptAes128(d.key, data, id)
	}
	return decryptAes256(d.key, data)
}

// getDecryptorFromDict is get in hayro-syntax src/crypto/mod.rs. It builds
// the decryptor of the standard security handler from the encryption
// dictionary.
func getDecryptorFromDict(dict Dict, id, password []byte) (*decryptor, error) {
	filter, ok := dict.Get[Name](KeyFilter)
	if !ok {
		return nil, DecryptionInvalidEncryption
	}
	if filter != "Standard" {
		return nil, DecryptionUnsupportedAlgorithm
	}
	encryptionV, ok := dict.Get[uint8](KeyV)
	if !ok {
		return nil, DecryptionInvalidEncryption
	}
	encryptMetadata, ok := dict.Get[bool](KeyEncryptMetadata)
	if !ok {
		encryptMetadata = true
	}
	revision, ok := dict.Get[uint8](KeyR)
	if !ok {
		return nil, DecryptionInvalidEncryption
	}
	var length uint16
	switch encryptionV {
	case 1:
		length = 40
	case 2:
		if length, ok = dict.Get[uint16](KeyLength); !ok {
			length = 40
		}
	case 4:
		if length, ok = dict.Get[uint16](KeyLength); !ok {
			length = 128
		}
	case 5:
		length = 256
	default:
		return nil, DecryptionUnsupportedAlgorithm
	}
	var algorithm decryptorTag
	var data *decryptorData
	switch encryptionV {
	case 1, 2:
		algorithm = tagRc4
	case 4, 5, 6:
		algorithm = tagAes128
		if encryptionV != 4 {
			algorithm = tagAes256
		}
		data, ok = decryptorDataFromDict(dict, length)
		if !ok {
			return nil, DecryptionInvalidEncryption
		}
	}
	byteLength := length / 8
	if byteLength == 0 {
		return nil, DecryptionInvalidEncryption
	}
	ownerString, ok := dict.Get[String](KeyO)
	if !ok {
		return nil, DecryptionInvalidEncryption
	}
	userString, ok := dict.Get[String](KeyU)
	if !ok {
		return nil, DecryptionInvalidEncryption
	}
	raw, ok := dict.Get[int64](KeyP)
	if !ok {
		return nil, DecryptionInvalidEncryption
	}
	// Upstream takes the low 32 bits of a negative value as i32, and casts
	// a positive one. Both keep the low 32 bits.
	permissions := uint32(raw)
	var key []byte
	var err error
	if revision <= 4 {
		key, err = decryptionKeyRev1234(password, encryptMetadata, revision, byteLength, ownerString, permissions, id)
		if err != nil {
			return nil, err
		}
		if err := authenticateUserPasswordRev234(revision, key, id, userString); err != nil {
			return nil, err
		}
	} else {
		key, err = decryptionKeyRev56(dict, revision, password, ownerString, userString)
		if err != nil {
			return nil, err
		}
	}
	// See pdf.js issue 19484.
	if encryptionV == 4 && len(key) < 16 {
		key = append(key, make([]byte, 16-len(key))...)
	}
	return &decryptor{kind: algorithm, key: key, dict: data}, nil
}

// decryptAes256 is decrypt_aes256 in hayro-syntax src/crypto/mod.rs.
func decryptAes256(key, data []byte) ([]byte, bool) {
	if len(data) < 16 || len(key) != 32 {
		return nil, false
	}
	return aesDecryptCBC(key, data[16:], data[:16], true)
}

// decryptAes128 is decrypt_aes128 in hayro-syntax src/crypto/mod.rs.
func decryptAes128(key, data []byte, id ObjectIdentifier) ([]byte, bool) {
	return decryptRcAes(key, id, true, func(key []byte) ([]byte, bool) {
		if len(key) != 16 || len(data) < 16 {
			return nil, false
		}
		return aesDecryptCBC(key, data[16:], data[:16], true)
	})
}

// decryptRc4 is decrypt_rc4 in hayro-syntax src/crypto/mod.rs.
func decryptRc4(key, data []byte, id ObjectIdentifier) ([]byte, bool) {
	return decryptRcAes(key, id, false, func(key []byte) ([]byte, bool) {
		return rc4Crypt(key, data)
	})
}

// decryptRcAes is decrypt_rc_aes in hayro-syntax src/crypto/mod.rs. It
// derives the key of one object.
func decryptRcAes(key []byte, id ObjectIdentifier, isAes bool, withKey func([]byte) ([]byte, bool)) ([]byte, bool) {
	n := len(key)
	k := bytes.Clone(key)
	var num [4]byte
	binary.LittleEndian.PutUint32(num[:], id.ObjNumber)
	k = append(k, num[:3]...)
	binary.LittleEndian.PutUint32(num[:], uint32(id.GenNumber))
	k = append(k, num[:2]...)
	if isAes {
		k = append(k, "sAlT"...)
	}
	hash := md5.Sum(k)
	return withKey(hash[:min(16, n+5)])
}

// decryptorData is DecryptorData in hayro-syntax src/crypto/mod.rs.
type decryptorData struct {
	streamFilter cryptDictionary
	stringFilter cryptDictionary
	cryptFilters map[Name]cryptDictionary
}

// decryptorDataFromDict is DecryptorData::from_dict in hayro-syntax
// src/crypto/mod.rs.
func decryptorDataFromDict(dict Dict, defaultLength uint16) (*decryptorData, bool) {
	mappings := map[Name]cryptDictionary{}
	if cf, ok := dict.Get[Dict](KeyCF); ok {
		for _, key := range cf.Keys() {
			d, ok := cf.Get[Dict](key)
			if !ok {
				continue
			}
			if c, ok := cryptDictionaryFromDict(d, defaultLength); ok {
				mappings[key] = c
			}
		}
	}
	stmF, ok := dict.Get[Name](KeyStmF)
	if !ok {
		return nil, false
	}
	stream, ok := mappings[stmF]
	if !ok {
		stream = cryptDictionaryIdentity(defaultLength)
	}
	strF, ok := dict.Get[Name](KeyStrF)
	if !ok {
		return nil, false
	}
	str, ok := mappings[strF]
	if !ok {
		str = cryptDictionaryIdentity(defaultLength)
	}
	return &decryptorData{streamFilter: stream, stringFilter: str, cryptFilters: mappings}, true
}

// cryptDictionary is CryptDictionary in hayro-syntax src/crypto/mod.rs.
type cryptDictionary struct {
	cfm    decryptorTag
	length uint16
}

// cryptDictionaryFromDict is CryptDictionary::from_dict in hayro-syntax
// src/crypto/mod.rs.
func cryptDictionaryFromDict(dict Dict, defaultLength uint16) (cryptDictionary, bool) {
	name, ok := dict.Get[Name](KeyCFM)
	if !ok {
		return cryptDictionary{}, false
	}
	cfm, ok := decryptorTagFromName(name)
	if !ok {
		return cryptDictionary{}, false
	}
	// Length is in bytes for the standard security handler.
	length, ok := dict.Get[uint16](KeyLength)
	if !ok {
		length = defaultLength / 8
	}
	switch cfm {
	case tagAes128:
		length = 16
	case tagAes256:
		length = 32
	}
	return cryptDictionary{cfm: cfm, length: length}, true
}

// cryptDictionaryIdentity is CryptDictionary::identity in hayro-syntax
// src/crypto/mod.rs.
func cryptDictionaryIdentity(defaultLength uint16) cryptDictionary {
	return cryptDictionary{cfm: tagNone, length: defaultLength}
}

// computeHashRev56 is compute_hash_rev56 in hayro-syntax src/crypto/mod.rs.
// It is the hash of the revisions 5 and 6.
func computeHashRev56(password, validationSalt, userString []byte, revision uint8) ([32]byte, error) {
	input := make([]byte, 0, len(password)+len(validationSalt)+len(userString))
	input = append(input, password...)
	input = append(input, validationSalt...)
	input = append(input, userString...)
	hash := sha256.Sum256(input)
	// The revision 5 uses only the first hash.
	if revision == 5 {
		return hash, nil
	}
	k := hash[:]
	var round uint16
	for {
		single := make([]byte, 0, len(password)+len(k)+len(userString))
		single = append(single, password...)
		single = append(single, k...)
		single = append(single, userString...)
		k1 := bytes.Repeat(single, 64)
		block, err := aes.NewCipher(k[:16])
		if err != nil {
			return [32]byte{}, DecryptionInvalidEncryption
		}
		// The length of k1 is a multiple of 64, so the padding of upstream
		// falls away when it truncates the result.
		e := make([]byte, len(k1))
		cipher.NewCBCEncrypter(block, k[16:32]).CryptBlocks(e, k1)
		// The first 16 bytes as a big endian number modulo 3. Since 256 is
		// 1 modulo 3, it is the sum of the bytes modulo 3.
		var sum uint
		for _, b := range e[:16] {
			sum += uint(b)
		}
		switch sum % 3 {
		case 0:
			h := sha256.Sum256(e)
			k = h[:]
		case 1:
			h := sha512.Sum384(e)
			k = h[:]
		default:
			h := sha512.Sum512(e)
			k = h[:]
		}
		round++
		if round > 63 && uint16(e[len(e)-1]) <= round-32 {
			break
		}
	}
	var result [32]byte
	copy(result[:], k[:32])
	return result, nil
}

// decryptionKeyRev1234 is decryption_key_rev1234 in hayro-syntax
// src/crypto/mod.rs.
func decryptionKeyRev1234(password []byte, encryptMetadata bool, revision uint8, byteLength uint16, ownerString String, permissions uint32, id []byte) ([]byte, error) {
	// Upstream notes that the password still needs a conversion to
	// PDFDocEncoding.
	var padded [32]byte
	n := copy(padded[:], password)
	copy(padded[n:], passwordPadding[:32-n])
	input := make([]byte, 0, 32+len(ownerString)+4+len(id)+4)
	input = append(input, padded[:]...)
	input = append(input, ownerString...)
	input = binary.LittleEndian.AppendUint32(input, permissions)
	input = append(input, id...)
	if !encryptMetadata && revision >= 4 {
		input = append(input, 0xff, 0xff, 0xff, 0xff)
	}
	hash := md5.Sum(input)
	if int(byteLength) > len(hash) {
		return nil, DecryptionInvalidEncryption
	}
	if revision >= 3 {
		for range 50 {
			hash = md5.Sum(hash[:byteLength])
		}
	}
	return bytes.Clone(hash[:byteLength]), nil
}

// authenticateUserPasswordRev234 is authenticate_user_password_rev234 in
// hayro-syntax src/crypto/mod.rs.
func authenticateUserPasswordRev234(revision uint8, key, id []byte, userString String) error {
	switch revision {
	case 2:
		result, ok := rc4Crypt(key, passwordPadding[:])
		if !ok || !bytes.Equal(result, userString) {
			return DecryptionPasswordProtected
		}
	case 3, 4:
		result, ok := userPasswordRev34(key, id)
		if !ok || len(userString) < 16 || !bytes.Equal(result[:16], userString[:16]) {
			return DecryptionPasswordProtected
		}
	default:
		return DecryptionInvalidEncryption
	}
	return nil
}

// userPasswordRev34 is user_password_rev34 in hayro-syntax
// src/crypto/mod.rs.
func userPasswordRev34(key, id []byte) ([]byte, bool) {
	input := make([]byte, 0, 32+len(id))
	input = append(input, passwordPadding[:]...)
	input = append(input, id...)
	hash := md5.Sum(input)
	encrypted, ok := rc4Crypt(key, hash[:])
	if !ok {
		return nil, false
	}
	for i := byte(1); i <= 19; i++ {
		k := bytes.Clone(key)
		for j := range k {
			k[j] ^= i
		}
		if encrypted, ok = rc4Crypt(k, encrypted); !ok {
			return nil, false
		}
	}
	out := make([]byte, 32)
	copy(out, encrypted)
	return out, true
}

// decryptionKeyRev56 is decryption_key_rev56 in hayro-syntax
// src/crypto/mod.rs.
func decryptionKeyRev56(dict Dict, revision uint8, password []byte, ownerString, userString String) ([]byte, error) {
	// Upstream notes that the password still needs SASLprep.
	password = password[:min(len(password), 127)]
	stringLen := 48
	if revision <= 4 {
		stringLen = 32
	}
	if len(ownerString) < stringLen || len(userString) < stringLen || stringLen < 40 {
		return nil, DecryptionInvalidEncryption
	}
	trimmedOs := ownerString[:stringLen]
	ownerHash, ownerValidationSalt, ownerKeySalt := trimmedOs[:32], trimmedOs[32:40], trimmedOs[40:]
	trimmedUs := userString[:stringLen]
	userHash, userValidationSalt, userKeySalt := trimmedUs[:32], trimmedUs[32:40], trimmedUs[40:]
	h, err := computeHashRev56(password, ownerValidationSalt, trimmedUs, revision)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(h[:], ownerHash) {
		intermediate, err := computeHashRev56(password, ownerKeySalt, trimmedUs, revision)
		if err != nil {
			return nil, err
		}
		oe, ok := dict.Get[String](KeyOE)
		if !ok || len(oe) != 32 {
			return nil, DecryptionInvalidEncryption
		}
		out, _ := aesDecryptCBC(intermediate[:], oe, make([]byte, 16), false)
		return out, nil
	}
	h, err = computeHashRev56(password, userValidationSalt, nil, revision)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(h[:], userHash) {
		intermediate, err := computeHashRev56(password, userKeySalt, nil, revision)
		if err != nil {
			return nil, err
		}
		ue, ok := dict.Get[String](KeyUE)
		if !ok || len(ue) != 32 {
			return nil, DecryptionInvalidEncryption
		}
		out, _ := aesDecryptCBC(intermediate[:], ue, make([]byte, 16), false)
		return out, nil
	}
	// Upstream notes that the entry Perms is not checked.
	return nil, DecryptionPasswordProtected
}

// aesDecryptCBC is AESCipher::decrypt_cbc in hayro-syntax
// src/crypto/aes.rs, on crypto/aes. It decrypts the whole blocks of data,
// and it drops a valid padding when unpad is true. The size of the key
// selects AES-128 or AES-256.
func aesDecryptCBC(key, data, iv []byte, unpad bool) ([]byte, bool) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, false
	}
	n := len(data) - len(data)%aes.BlockSize
	result := make([]byte, n)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(result, data[:n])
	if unpad && len(result) > 0 {
		padLen := int(result[len(result)-1])
		if padLen > 0 && padLen <= 16 && len(result) >= padLen {
			start := len(result) - padLen
			if bytes.Count(result[start:], []byte{byte(padLen)}) == padLen {
				result = result[:start]
			}
		}
	}
	return result, true
}

// rc4Crypt is Rc4::decrypt in hayro-syntax src/crypto/rc4.rs, on
// crypto/rc4. It fails for a key that crypto/rc4 refuses, where upstream
// panics for an empty key.
func rc4Crypt(key, data []byte) ([]byte, bool) {
	c, err := rc4.NewCipher(key)
	if err != nil {
		return nil, false
	}
	out := make([]byte, len(data))
	c.XORKeyStream(out, data)
	return out, true
}
