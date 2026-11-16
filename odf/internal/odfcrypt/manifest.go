// Package odfcrypt reads the manifest of an OpenDocument file, and decrypts
// the entries that a password protects, as part 3 of ODF 1.2 describes in
// section 3.4 and as OpenOffice wrote them before ODF 1.2:
//
//   - The ciphers Blowfish in the mode CFB, AES in the mode CBC, and AES in
//     the mode GCM.
//   - The start key, the SHA-1 or the SHA-256 of the password.
//   - The key derivation PBKDF2 with HMAC-SHA1, and Argon2id, which ODF 1.4
//     adds and LibreOffice writes by default (D25).
//   - The checksum of the first 1024 bytes, with SHA-1 or SHA-256.
package odfcrypt

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/ivy"
	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// PackageName is the entry that holds the whole document, encrypted, when
// the file encrypts the whole package. LibreOffice writes it for ODF 1.4.
const PackageName = "encrypted-package"

// MaxArgon2Passes is the largest number of passes of Argon2id that the
// package runs. LibreOffice writes 3. The memory of Argon2id is bounded by
// ivy.Limits.MaxMemory when the key is derived.
const MaxArgon2Passes = 64

// parseArgon2 reads the parameters of Argon2id from the element
// key-derivation: the passes, the memory in KiB and the lanes. ODF 1.4 puts
// them in the namespace of the manifest, and LibreOffice in its own
// namespace loext, with the same local names.
func parseArgon2(n roxmltree.Node, enc *Encryption) error {
	passes, err := strconv.Atoi(attr(n, "argon2-iterations"))
	if err != nil || passes < 1 {
		return fmt.Errorf("argon2 iterations %q: %w", attr(n, "argon2-iterations"), ivy.ErrFormat)
	}
	if passes > MaxArgon2Passes {
		return fmt.Errorf("argon2 iterations %d: %w", passes, ivy.ErrLimit)
	}
	memory, err := strconv.ParseUint(attr(n, "argon2-memory"), 10, 32)
	if err != nil || memory < 8 {
		return fmt.Errorf("argon2 memory %q: %w", attr(n, "argon2-memory"), ivy.ErrFormat)
	}
	lanes, err := strconv.ParseUint(attr(n, "argon2-lanes"), 10, 8)
	if err != nil || lanes < 1 {
		return fmt.Errorf("argon2 lanes %q: %w", attr(n, "argon2-lanes"), ivy.ErrFormat)
	}
	enc.Iterations, enc.Memory, enc.Lanes = passes, uint32(memory), uint8(lanes)
	return nil
}

// MaxIterations is the largest iteration count of PBKDF2 that the package
// accepts. LibreOffice writes 100000. The context bounds the time between
// two entries.
const MaxIterations = 10_000_000

// Manifest is the file META-INF/manifest.xml of an OpenDocument file.
type Manifest struct {
	// Mime is the media type of the entry "/", the mime type of the
	// document. It is empty if the manifest gives none.
	Mime string
	// Entries holds the entries of the manifest, by their full path.
	Entries map[string]*Entry
}

// Entry is an entry of the manifest.
type Entry struct {
	Path      string
	MediaType string
	// Size is the size of the entry before compression and encryption, or
	// -1 if the manifest does not give it.
	Size int64
	// Encryption is nil if the entry is not encrypted.
	Encryption *Encryption
}

// Encrypted reports whether the manifest encrypts an entry.
func (m *Manifest) Encrypted() bool {
	for _, e := range m.Entries {
		if e.Encryption != nil {
			return true
		}
	}
	return false
}

// Encryption returns the encryption of the entry name, or nil if the
// manifest does not encrypt it.
func (m *Manifest) Encryption(name string) *Encryption {
	if e, ok := m.Entries[name]; ok {
		return e.Encryption
	}
	return nil
}

// Algorithm is a cipher of an entry.
type Algorithm int

// The ciphers.
const (
	// Blowfish is Blowfish in the mode CFB with blocks of 8 bytes, of
	// OpenOffice and ODF 1.0 and 1.1.
	Blowfish Algorithm = iota + 1
	// AESCBC is AES in the mode CBC, of ODF 1.2.
	AESCBC
	// AESGCM is AES in the mode GCM, of ODF 1.4.
	AESGCM
)

// Derivation is a key derivation of an entry.
type Derivation int

// The key derivations.
const (
	// PBKDF2 is PBKDF2 with HMAC-SHA1.
	PBKDF2 Derivation = iota + 1
	// Argon2id is Argon2id (RFC 9106), which ODF 1.4 adds and which
	// LibreOffice writes by default (D25).
	Argon2id
)

// Checksum is the type of the checksum of an entry.
type Checksum int

// The types of the checksum.
const (
	// NoChecksum means that the entry has no checksum.
	NoChecksum Checksum = iota
	// SHA1K is the SHA-1 of the first 1024 bytes.
	SHA1K
	// SHA256K is the SHA-256 of the first 1024 bytes.
	SHA256K
)

// Encryption is the element encryption-data of an entry.
type Encryption struct {
	Algorithm Algorithm
	// KeySize is the size of the key of the cipher, in bytes.
	KeySize int
	IV      []byte
	// StartSHA256 is true if the start key is the SHA-256 of the password,
	// and false for SHA-1.
	StartSHA256 bool
	Derivation  Derivation
	// Iterations is the iteration count of PBKDF2, or the number of passes
	// of Argon2id.
	Iterations int
	// Memory is the memory of Argon2id, in KiB, and Lanes its parallelism.
	Memory   uint32
	Lanes    uint8
	Salt     []byte
	Checksum Checksum
	Sum      []byte
}

// The names of the manifest (ODF 1.2 part 3, section 4).
var (
	checksums = map[string]Checksum{
		"SHA1/1K": SHA1K,
		"urn:oasis:names:tc:opendocument:xmlns:manifest:1.0#sha1-1k":   SHA1K,
		"urn:oasis:names:tc:opendocument:xmlns:manifest:1.0#sha256-1k": SHA256K,
	}
	algorithms = map[string]struct {
		alg     Algorithm
		keySize int
	}{
		"Blowfish CFB": {Blowfish, 16},
		"http://www.w3.org/2001/04/xmlenc#aes128-cbc": {AESCBC, 16},
		"http://www.w3.org/2001/04/xmlenc#aes192-cbc": {AESCBC, 24},
		"http://www.w3.org/2001/04/xmlenc#aes256-cbc": {AESCBC, 32},
		"http://www.w3.org/2009/xmlenc11#aes128-gcm":  {AESGCM, 16},
		"http://www.w3.org/2009/xmlenc11#aes192-gcm":  {AESGCM, 24},
		"http://www.w3.org/2009/xmlenc11#aes256-gcm":  {AESGCM, 32},
	}
	startKeys = map[string]bool{
		"SHA1":                                   false,
		"http://www.w3.org/2000/09/xmldsig#sha1": false,
		"http://www.w3.org/2000/09/xmldsig#sha256": true,
		"http://www.w3.org/2001/04/xmlenc#sha256":  true,
	}
	derivations = map[string]Derivation{
		"PBKDF2": PBKDF2,
		"urn:org:documentfoundation:names:experimental:office:manifest:argon2id": Argon2id,
		"urn:oasis:names:tc:opendocument:xmlns:manifest:1.0#argon2id":            Argon2id,
	}
)

// ParseManifest parses the file META-INF/manifest.xml. It reads at most
// lim.MaxObjects XML nodes. It returns an error that wraps ivy.ErrFormat if
// buf is malformed, and one that wraps ivy.ErrUnsupported for a cipher or a
// checksum that it does not know.
func ParseManifest(buf []byte, lim ivy.Limits) (*Manifest, error) {
	lim = lim.WithDefaults()
	text := strings.TrimPrefix(string(buf), "\ufeff")
	if !utf8.ValidString(text) {
		return nil, fmt.Errorf("manifest: invalid utf-8: %w", ivy.ErrFormat)
	}
	opt := roxmltree.DefaultParsingOptions()
	opt.NodesLimit = uint32(min(lim.MaxObjects, math.MaxUint32))
	doc, err := roxmltree.ParseWithOptions(text, opt)
	switch {
	case errors.Is(err, okage.ErrXMLNodesLimitReached):
		return nil, fmt.Errorf("manifest of more than %d nodes: %w", lim.MaxObjects, ivy.ErrLimit)
	case err != nil:
		return nil, fmt.Errorf("manifest: %w: %w", ivy.ErrFormat, err)
	}
	root := doc.RootElement()
	if root.TagName().Name() != "manifest" {
		return nil, fmt.Errorf("manifest: root %q: %w", root.TagName().Name(), ivy.ErrFormat)
	}
	m := &Manifest{Entries: make(map[string]*Entry)}
	for n := range root.Children() {
		if !n.IsElement() || n.TagName().Name() != "file-entry" {
			continue
		}
		e, err := parseEntry(n)
		if err != nil {
			return nil, err
		}
		if e.Path == "/" {
			m.Mime = e.MediaType
		}
		if _, ok := m.Entries[e.Path]; !ok {
			m.Entries[e.Path] = e
		}
	}
	return m, nil
}

// parseEntry parses the element file-entry.
func parseEntry(n roxmltree.Node) (*Entry, error) {
	e := &Entry{
		Path:      attr(n, "full-path"),
		MediaType: attr(n, "media-type"),
		Size:      -1,
	}
	if s := attr(n, "size"); s != "" {
		size, err := strconv.ParseInt(s, 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("manifest: %s: size %q: %w", e.Path, s, ivy.ErrFormat)
		}
		e.Size = size
	}
	for c := range n.Children() {
		if c.IsElement() && c.TagName().Name() == "encryption-data" {
			enc, err := parseEncryption(c)
			if err != nil {
				return nil, fmt.Errorf("manifest: %s: %w", e.Path, err)
			}
			e.Encryption = enc
			break
		}
	}
	return e, nil
}

// parseEncryption parses the element encryption-data.
func parseEncryption(n roxmltree.Node) (*Encryption, error) {
	enc := &Encryption{}
	if name := attr(n, "checksum-type"); name != "" {
		c, ok := checksums[name]
		if !ok {
			return nil, fmt.Errorf("checksum %q: %w", name, ivy.ErrUnsupported)
		}
		enc.Checksum = c
		var err error
		if enc.Sum, err = base64Attr(n, "checksum"); err != nil {
			return nil, err
		}
	}
	var hasAlg, hasDerivation bool
	for c := range n.Children() {
		if !c.IsElement() {
			continue
		}
		var err error
		switch c.TagName().Name() {
		case "algorithm":
			name := attr(c, "algorithm-name")
			a, ok := algorithms[name]
			if !ok {
				return nil, fmt.Errorf("cipher %q: %w", name, ivy.ErrUnsupported)
			}
			hasAlg = true
			enc.Algorithm = a.alg
			if enc.KeySize == 0 {
				enc.KeySize = a.keySize
			}
			if enc.IV, err = base64Attr(c, "initialisation-vector"); err != nil {
				return nil, err
			}
		case "start-key-generation":
			name := attr(c, "start-key-generation-name")
			sha256, ok := startKeys[name]
			if !ok {
				return nil, fmt.Errorf("start key %q: %w", name, ivy.ErrUnsupported)
			}
			enc.StartSHA256 = sha256
		case "key-derivation":
			name := attr(c, "key-derivation-name")
			d, ok := derivations[name]
			if !ok {
				return nil, fmt.Errorf("key derivation %q: %w", name, ivy.ErrUnsupported)
			}
			hasDerivation = true
			enc.Derivation = d
			if s := attr(c, "key-size"); s != "" {
				if enc.KeySize, err = strconv.Atoi(s); err != nil {
					return nil, fmt.Errorf("key size %q: %w", s, ivy.ErrFormat)
				}
			}
			if d == PBKDF2 {
				s := attr(c, "iteration-count")
				if enc.Iterations, err = strconv.Atoi(s); err != nil || enc.Iterations < 1 {
					return nil, fmt.Errorf("iteration count %q: %w", s, ivy.ErrFormat)
				}
				if enc.Iterations > MaxIterations {
					return nil, fmt.Errorf("iteration count %d: %w", enc.Iterations, ivy.ErrLimit)
				}
			}
			if d == Argon2id {
				if err := parseArgon2(c, enc); err != nil {
					return nil, err
				}
			}
			if enc.Salt, err = base64Attr(c, "salt"); err != nil {
				return nil, err
			}
		}
	}
	if !hasAlg || !hasDerivation {
		return nil, fmt.Errorf("encryption without a cipher or a key derivation: %w", ivy.ErrFormat)
	}
	return enc, nil
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
		return nil, fmt.Errorf("%s: %w: %w", name, ivy.ErrFormat, err)
	}
	return buf, nil
}
