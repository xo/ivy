package fixture

import (
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	// ODF 1.1 encrypts with Blowfish, which the maintainers approved (D24).
	"golang.org/x/crypto/blowfish" //nolint:staticcheck // the format needs it
)

// ODFCipher is a cipher that encrypts the entries of an OpenDocument file.
type ODFCipher int

// The ciphers.
const (
	// ODFBlowfish is Blowfish in the mode CFB, with the start key SHA-1 and
	// the checksum SHA1/1K, as OpenOffice and ODF 1.1 write it.
	ODFBlowfish ODFCipher = iota
	// ODFAESCBC is AES-256 in the mode CBC, with the start key SHA-256 and
	// the checksum SHA-256 of 1K, as ODF 1.2 writes it.
	ODFAESCBC
	// ODFAESGCM is AES-256 in the mode GCM, with the start key SHA-256 and
	// no checksum, as ODF 1.4 writes it, but with PBKDF2.
	ODFAESGCM
)

// ODFEncryption sets how EncryptODF encrypts a file.
type ODFEncryption struct {
	Password string
	Cipher   ODFCipher
	// Iterations is the iteration count of PBKDF2.
	Iterations int
	// Whole encrypts the whole package as the one entry encrypted-package,
	// as LibreOffice does for ODF 1.4.
	Whole bool
}

// EncryptODF returns an OpenDocument file of the mime type mime that holds
// the entries, encrypted. The entry mimetype and the manifest stay in the
// clear.
func EncryptODF(enc ODFEncryption, mime string, entries ...Entry) []byte {
	if enc.Whole {
		inner := Zip(append([]Entry{
			{Name: "mimetype", Data: []byte(mime), Store: true},
			{Name: "META-INF/manifest.xml", Data: []byte(manifest(mime, nil))},
		}, entries...)...)
		entries = []Entry{{Name: "encrypted-package", Data: inner}}
	}
	out := make([]Entry, 0, len(entries)+2)
	out = append(out, Entry{Name: "mimetype", Data: []byte(mime), Store: true})
	xml := make([]string, 0, len(entries))
	for _, e := range entries {
		data, meta := encryptODFEntry(enc, e)
		out = append(out, Entry{Name: e.Name, Data: data, Store: true})
		xml = append(xml, meta)
	}
	out = append(out, Entry{Name: "META-INF/manifest.xml", Data: []byte(manifest(mime, xml))})
	return Zip(out...)
}

// manifest returns META-INF/manifest.xml with the entries of xml.
func manifest(mime string, xml []string) string {
	root := fmt.Sprintf(` <manifest:file-entry manifest:full-path="/" manifest:media-type="%s"/>`, mime)
	return `<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.2">
` + root + "\n" + strings.Join(xml, "\n") + "\n</manifest:manifest>\n"
}

// encryptODFEntry compresses and encrypts the entry e, and returns the data
// and the element file-entry of the manifest.
func encryptODFEntry(enc ODFEncryption, e Entry) ([]byte, string) {
	var z bytes.Buffer
	zw, err := flate.NewWriter(&z, flate.BestCompression)
	if err != nil {
		panic(err)
	}
	if _, err := zw.Write(e.Data); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	plain := z.Bytes()
	salt := random(16)
	b64 := base64.StdEncoding.EncodeToString
	var startKey []byte
	if enc.Cipher == ODFBlowfish {
		s := sha1.Sum([]byte(enc.Password))
		startKey = s[:]
	} else {
		s := sha256.Sum256([]byte(enc.Password))
		startKey = s[:]
	}
	keySize := 32
	if enc.Cipher == ODFBlowfish {
		keySize = 16
	}
	key, err := pbkdf2.Key(sha1.New, string(startKey), salt, enc.Iterations, keySize)
	if err != nil {
		panic(err)
	}
	head := plain[:min(len(plain), 1024)]
	var data []byte
	var meta string
	switch enc.Cipher {
	case ODFBlowfish:
		iv := random(8)
		b, err := blowfish.NewCipher(key)
		if err != nil {
			panic(err)
		}
		data = cfbEncrypt(b, iv, plain)
		sum := sha1.Sum(head)
		meta = fmt.Sprintf(`  <manifest:encryption-data manifest:checksum-type="SHA1/1K" manifest:checksum="%s">
   <manifest:algorithm manifest:algorithm-name="Blowfish CFB" manifest:initialisation-vector="%s"/>
   <manifest:key-derivation manifest:key-derivation-name="PBKDF2" manifest:iteration-count="%d" manifest:salt="%s"/>
  </manifest:encryption-data>`, b64(sum[:]), b64(iv), enc.Iterations, b64(salt))
	case ODFAESCBC:
		iv := random(16)
		b, err := aes.NewCipher(key)
		if err != nil {
			panic(err)
		}
		// The padding of XML Encryption: random bytes, and the length of
		// the padding in the last byte.
		pad := 16 - len(plain)%16
		padded := append(bytes.Clone(plain), random(pad)...)
		padded[len(padded)-1] = byte(pad)
		data = make([]byte, len(padded))
		cipher.NewCBCEncrypter(b, iv).CryptBlocks(data, padded)
		sum := sha256.Sum256(head)
		meta = fmt.Sprintf(`  <manifest:encryption-data manifest:checksum-type="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0#sha256-1k" manifest:checksum="%s">
   <manifest:algorithm manifest:algorithm-name="http://www.w3.org/2001/04/xmlenc#aes256-cbc" manifest:initialisation-vector="%s"/>
   <manifest:start-key-generation manifest:start-key-generation-name="http://www.w3.org/2000/09/xmldsig#sha256" manifest:key-size="32"/>
   <manifest:key-derivation manifest:key-derivation-name="PBKDF2" manifest:key-size="32" manifest:iteration-count="%d" manifest:salt="%s"/>
  </manifest:encryption-data>`, b64(sum[:]), b64(iv), enc.Iterations, b64(salt))
	case ODFAESGCM:
		iv := random(12)
		b, err := aes.NewCipher(key)
		if err != nil {
			panic(err)
		}
		gcm, err := cipher.NewGCM(b)
		if err != nil {
			panic(err)
		}
		data = gcm.Seal(nil, iv, plain, nil)
		meta = fmt.Sprintf(`  <manifest:encryption-data>
   <manifest:algorithm manifest:algorithm-name="http://www.w3.org/2009/xmlenc11#aes256-gcm" manifest:initialisation-vector="%s"/>
   <manifest:start-key-generation manifest:start-key-generation-name="http://www.w3.org/2001/04/xmlenc#sha256" manifest:key-size="32"/>
   <manifest:key-derivation manifest:key-derivation-name="PBKDF2" manifest:key-size="32" manifest:iteration-count="%d" manifest:salt="%s"/>
  </manifest:encryption-data>`, b64(iv), enc.Iterations, b64(salt))
	}
	entry := fmt.Sprintf(` <manifest:file-entry manifest:full-path="%s" manifest:media-type="" manifest:size="%d">
%s
 </manifest:file-entry>`, e.Name, len(e.Data), meta)
	return data, entry
}

// cfbEncrypt encrypts data with b in the mode CFB, with a feedback of one
// whole block.
func cfbEncrypt(b cipher.Block, iv, data []byte) []byte {
	n := b.BlockSize()
	reg := bytes.Clone(iv)
	stream := make([]byte, n)
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += n {
		b.Encrypt(stream, reg)
		m := min(n, len(data)-i)
		for j := range m {
			out[i+j] = data[i+j] ^ stream[j]
		}
		copy(reg, out[i:i+m])
	}
	return out
}

// random returns n random bytes.
func random(n int) []byte {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return buf
}
