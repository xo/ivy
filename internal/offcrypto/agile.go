package offcrypto

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// passwordURI names the key encryptor of a password.
const passwordURI = "http://schemas.microsoft.com/office/2006/keyEncryptor/password"

// maxNodes is the number of XML nodes that the descriptor of Agile
// encryption can hold. Office writes about ten.
const maxNodes = 4096

// The block keys of Agile encryption (MS-OFFCRYPTO 2.3.4.11 to 2.3.4.14).
var (
	blockVerifierInput = []byte{0xfe, 0xa7, 0xd2, 0x76, 0x3b, 0x4b, 0x9e, 0x79}
	blockVerifierValue = []byte{0xd7, 0xaa, 0x0f, 0x6d, 0x30, 0x61, 0x34, 0x4e}
	blockKeyValue      = []byte{0x14, 0x6e, 0x0b, 0xe7, 0xab, 0xac, 0xd0, 0xd6}
	blockHmacKey       = []byte{0x5f, 0xb2, 0xad, 0x01, 0x0c, 0xb9, 0xe1, 0xf6}
	blockHmacValue     = []byte{0xa0, 0x67, 0x7f, 0x02, 0xb2, 0x2c, 0x84, 0x33}
)

// agileParams are the attributes that the elements keyData and
// encryptedKey share.
type agileParams struct {
	saltSize  int
	blockSize int
	keyBits   int
	hashSize  int
	hash      func() hash.Hash
	salt      []byte
}

// agileInfo is the XML descriptor of Agile encryption.
type agileInfo struct {
	// data holds the parameters of the package, from keyData.
	data agileParams
	// pw holds the parameters of the password, from encryptedKey.
	pw        agileParams
	spinCount int

	encryptedVerifierInput []byte
	encryptedVerifierValue []byte
	encryptedKeyValue      []byte
	// encryptedHmacKey and encryptedHmacValue are empty if the file holds no
	// element dataIntegrity.
	encryptedHmacKey   []byte
	encryptedHmacValue []byte
}

// parseAgile parses the part of the stream after the version: a reserved
// field and the XML descriptor (MS-OFFCRYPTO 2.3.4.10).
func parseAgile(buf []byte) (*agileInfo, error) {
	if len(buf) < 4 {
		return nil, fmt.Errorf("agile encryption info of %d bytes: %w", len(buf), ivy.ErrFormat)
	}
	text := bytes.TrimRight(buf[4:], "\x00")
	text = bytes.TrimPrefix(text, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(text) {
		return nil, fmt.Errorf("agile encryption descriptor: invalid utf-8: %w", ivy.ErrFormat)
	}
	opt := roxmltree.DefaultParsingOptions()
	opt.NodesLimit = maxNodes
	doc, err := roxmltree.ParseWithOptions(string(text), opt)
	switch {
	case errors.Is(err, okage.ErrXMLNodesLimitReached):
		return nil, fmt.Errorf("agile encryption descriptor of more than %d nodes: %w", maxNodes, ivy.ErrLimit)
	case err != nil:
		return nil, fmt.Errorf("agile encryption descriptor: %w: %w", ivy.ErrFormat, err)
	}
	root := doc.RootElement()
	if root.TagName().Name() != "encryption" {
		return nil, fmt.Errorf("agile encryption descriptor: root %q: %w", root.TagName().Name(), ivy.ErrFormat)
	}
	ai := &agileInfo{}
	var keyData, encryptedKey, integrity roxmltree.Node
	var hasKeyData, hasKey, hasIntegrity bool
	for n := range root.Children() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "keyData":
			keyData, hasKeyData = n, true
		case "dataIntegrity":
			integrity, hasIntegrity = n, true
		case "keyEncryptors":
			encryptedKey, hasKey = passwordKey(n)
		}
	}
	if !hasKeyData {
		return nil, fmt.Errorf("agile encryption descriptor without keyData: %w", ivy.ErrFormat)
	}
	if !hasKey {
		return nil, fmt.Errorf("agile encryption without a password key: %w", ivy.ErrUnsupported)
	}
	if ai.data, err = parseParams(keyData); err != nil {
		return nil, err
	}
	if ai.pw, err = parseParams(encryptedKey); err != nil {
		return nil, err
	}
	spin, err := strconv.Atoi(attr(encryptedKey, "spinCount"))
	if err != nil || spin < 0 {
		return nil, fmt.Errorf("spin count %q: %w", attr(encryptedKey, "spinCount"), ivy.ErrFormat)
	}
	if spin > MaxSpinCount {
		return nil, fmt.Errorf("spin count %d: %w", spin, ivy.ErrLimit)
	}
	ai.spinCount = spin
	type field struct {
		dst  *[]byte
		n    roxmltree.Node
		name string
	}
	fields := []field{
		{&ai.encryptedVerifierInput, encryptedKey, "encryptedVerifierHashInput"},
		{&ai.encryptedVerifierValue, encryptedKey, "encryptedVerifierHashValue"},
		{&ai.encryptedKeyValue, encryptedKey, "encryptedKeyValue"},
	}
	if hasIntegrity {
		fields = append(fields,
			field{&ai.encryptedHmacKey, integrity, "encryptedHmacKey"},
			field{&ai.encryptedHmacValue, integrity, "encryptedHmacValue"})
	}
	for _, f := range fields {
		if *f.dst, err = base64Attr(f.n, f.name); err != nil {
			return nil, err
		}
	}
	return ai, nil
}

// passwordKey returns the element encryptedKey of the key encryptor of a
// password in the element keyEncryptors.
func passwordKey(n roxmltree.Node) (roxmltree.Node, bool) {
	for ke := range n.Children() {
		if !ke.IsElement() || ke.TagName().Name() != "keyEncryptor" || attr(ke, "uri") != passwordURI {
			continue
		}
		for c := range ke.Children() {
			if c.IsElement() && c.TagName().Name() == "encryptedKey" {
				return c, true
			}
		}
	}
	return roxmltree.Node{}, false
}

// parseParams parses the attributes that keyData and encryptedKey share.
// It accepts AES in the mode CBC only, the cipher that Office writes.
func parseParams(n roxmltree.Node) (agileParams, error) {
	var p agileParams
	name := n.TagName().Name()
	if alg := attr(n, "cipherAlgorithm"); alg != "AES" {
		return p, fmt.Errorf("%s: cipher algorithm %q: %w", name, alg, ivy.ErrUnsupported)
	}
	if chaining := attr(n, "cipherChaining"); chaining != "ChainingModeCBC" {
		return p, fmt.Errorf("%s: chaining %q: %w", name, chaining, ivy.ErrUnsupported)
	}
	switch alg := attr(n, "hashAlgorithm"); alg {
	case "SHA1", "SHA-1":
		p.hash = sha1.New
	case "SHA256":
		p.hash = sha256.New
	case "SHA384":
		p.hash = sha512.New384
	case "SHA512":
		p.hash = sha512.New
	case "MD5":
		p.hash = md5.New
	default:
		return p, fmt.Errorf("%s: hash algorithm %q: %w", name, alg, ivy.ErrUnsupported)
	}
	for _, f := range []struct {
		dst  *int
		name string
	}{
		{&p.saltSize, "saltSize"},
		{&p.blockSize, "blockSize"},
		{&p.keyBits, "keyBits"},
		{&p.hashSize, "hashSize"},
	} {
		v, err := strconv.Atoi(attr(n, f.name))
		if err != nil {
			return p, fmt.Errorf("%s: %s %q: %w", name, f.name, attr(n, f.name), ivy.ErrFormat)
		}
		*f.dst = v
	}
	if p.blockSize != aes.BlockSize {
		return p, fmt.Errorf("%s: block of %d bytes: %w", name, p.blockSize, ivy.ErrFormat)
	}
	if p.keyBits != 128 && p.keyBits != 192 && p.keyBits != 256 {
		return p, fmt.Errorf("%s: aes key of %d bits: %w", name, p.keyBits, ivy.ErrFormat)
	}
	if p.hashSize != p.hash().Size() {
		return p, fmt.Errorf("%s: hash of %d bytes: %w", name, p.hashSize, ivy.ErrFormat)
	}
	if p.saltSize < 1 || p.saltSize > 65536 {
		return p, fmt.Errorf("%s: salt of %d bytes: %w", name, p.saltSize, ivy.ErrFormat)
	}
	var err error
	if p.salt, err = base64Attr(n, "saltValue"); err != nil {
		return p, err
	}
	if len(p.salt) == 0 {
		return p, fmt.Errorf("%s: empty salt: %w", name, ivy.ErrFormat)
	}
	return p, nil
}

// attr returns the value of the attribute of n with the local name name,
// or the empty string.
func attr(n roxmltree.Node, name string) string {
	for a := range n.Attributes() {
		if a.Name() == name {
			return a.Value()
		}
	}
	return ""
}

// base64Attr decodes the attribute of n with the local name name, in
// base64.
func base64Attr(n roxmltree.Node, name string) ([]byte, error) {
	s := strings.Join(strings.Fields(attr(n, name)), "")
	buf, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w: %w", n.TagName().Name(), name, ivy.ErrFormat, err)
	}
	return buf, nil
}

// key derives the key of the password, makes sure that it decrypts the
// verifier, and decrypts the key of the package (MS-OFFCRYPTO 2.3.4.11 and
// 2.3.4.13).
func (ai *agileInfo) key(ctx context.Context, password string) (*Key, error) {
	p := ai.pw
	h := p.hash()
	h.Write(p.salt)
	h.Write(passwordBytes(password))
	hn := h.Sum(nil)
	var it [4]byte
	for i := range ai.spinCount {
		if i&0xffff == 0xffff {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		binary.LittleEndian.PutUint32(it[:], uint32(i))
		h.Reset()
		h.Write(it[:])
		h.Write(hn)
		hn = h.Sum(hn[:0])
	}
	derive := func(block []byte) []byte {
		h.Reset()
		h.Write(hn)
		h.Write(block)
		return fit(h.Sum(nil), p.keyBits/8)
	}
	iv := fit(p.salt, p.blockSize)
	input, err := cbcDecrypt(derive(blockVerifierInput), iv, ai.encryptedVerifierInput)
	if err != nil {
		return nil, err
	}
	value, err := cbcDecrypt(derive(blockVerifierValue), iv, ai.encryptedVerifierValue)
	if err != nil {
		return nil, err
	}
	h.Reset()
	h.Write(input[:min(p.saltSize, len(input))])
	if !equal(h.Sum(nil), value, p.hashSize) {
		return nil, fmt.Errorf("agile encryption: %w", ivy.ErrPassword)
	}
	secret, err := cbcDecrypt(derive(blockKeyValue), iv, ai.encryptedKeyValue)
	if err != nil {
		return nil, err
	}
	if len(secret) < ai.data.keyBits/8 {
		return nil, fmt.Errorf("agile key of %d bytes: %w", len(secret), ivy.ErrFormat)
	}
	return &Key{scheme: Agile, secret: secret[:ai.data.keyBits/8], agile: ai}, nil
}

// iv returns the initialization vector of the package for the block key
// block (MS-OFFCRYPTO 2.3.4.12).
func (ai *agileInfo) iv(block []byte) []byte {
	h := ai.data.hash()
	h.Write(ai.data.salt)
	h.Write(block)
	return fit(h.Sum(nil), ai.data.blockSize)
}

// cbcDecrypt decrypts src with AES in the mode CBC.
func cbcDecrypt(key, iv, src []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes key of %d bytes: %w", len(key), ivy.ErrFormat)
	}
	if len(src) == 0 || len(src)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("aes data of %d bytes: %w", len(src), ivy.ErrFormat)
	}
	dst := make([]byte, len(src))
	cipher.NewCBCDecrypter(b, iv).CryptBlocks(dst, src)
	return dst, nil
}
