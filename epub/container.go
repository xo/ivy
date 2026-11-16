package epub

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook"
	"github.com/xo/ivy/internal/zipdoc"
)

// The paths of the container.
const (
	containerPath  = "META-INF/container.xml"
	encryptionPath = "META-INF/encryption.xml"
)

// The algorithms of encryption.xml that obfuscate a font. They are not DRM,
// and ivy undoes them (D12).
const (
	// algIDPF is the obfuscation of fonts of the IDPF, in OCF.
	algIDPF = "http://www.idpf.org/2008/embedding"
	// algAdobe is the obfuscation of fonts of Adobe.
	algAdobe = "http://ns.adobe.com/pdf/enc#RC"
)

// The number of bytes at the start of a font that each obfuscation changes.
const (
	idpfLength  = 1040
	adobeLength = 1024
)

// rootfile returns the path of the package document, from the container. A
// file without a container, or with a broken one, falls back on the first
// file whose name ends in .opf.
func rootfile(archive *zipdoc.Archive, lim ivy.Limits) (string, error) {
	if f := archive.Find(containerPath); f != nil {
		buf, err := archive.Read(f)
		if err != nil {
			return "", fmt.Errorf("reading container: %w", err)
		}
		doc, err := ebook.ParseXML(buf, lim)
		if err == nil {
			var fallback string
			for n := range doc.RootElement().Descendants() {
				if !n.IsElement() || ebook.Local(n) != "rootfile" {
					continue
				}
				name, ok := ebook.Resolve("", ebook.Attr(n, "full-path"))
				if !ok {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(ebook.Attr(n, "media-type")), "application/oebps-package+xml") {
					return name, nil
				}
				if fallback == "" {
					fallback = name
				}
			}
			if fallback != "" {
				return fallback, nil
			}
		}
	}
	for _, f := range archive.Files() {
		if strings.EqualFold(path.Ext(f.Name), ".opf") {
			if name, ok := zipdoc.CleanName(f.Name); ok {
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("epub without a package document: %w", ivy.ErrFormat)
}

// readEncryption reads encryption.xml, and returns the algorithm of each
// path that it lists. A file without it returns an empty map.
func readEncryption(archive *zipdoc.Archive, lim ivy.Limits) (map[string]string, error) {
	algorithms := make(map[string]string)
	f := archive.Find(encryptionPath)
	if f == nil {
		return algorithms, nil
	}
	buf, err := archive.Read(f)
	if err != nil {
		return nil, fmt.Errorf("reading encryption: %w", err)
	}
	doc, err := ebook.ParseXML(buf, lim)
	if err != nil {
		return nil, fmt.Errorf("parsing encryption: %w", err)
	}
	for data := range doc.RootElement().Descendants() {
		if !data.IsElement() || ebook.Local(data) != "EncryptedData" {
			continue
		}
		var algorithm, uri string
		for n := range data.Descendants() {
			if !n.IsElement() {
				continue
			}
			switch ebook.Local(n) {
			case "EncryptionMethod":
				if algorithm == "" {
					algorithm = strings.TrimSpace(ebook.Attr(n, "Algorithm"))
				}
			case "CipherReference":
				if uri == "" {
					uri = ebook.Attr(n, "URI")
				}
			}
		}
		// the paths of encryption.xml are relative to the root of the
		// container
		if name, ok := ebook.Resolve("", uri); ok {
			algorithms[name] = algorithm
		}
	}
	return algorithms, nil
}

// isObfuscation reports whether algorithm obfuscates a font, which is not
// DRM.
func isObfuscation(algorithm string) bool {
	return algorithm == algIDPF || algorithm == algAdobe
}

// deobfuscator returns the function that undoes the obfuscation algorithm
// of a font, with the key from the identifiers of the package.
func deobfuscator(algorithm string, m ebook.Metadata) (func([]byte) ([]byte, error), error) {
	var key []byte
	var length int
	switch algorithm {
	case algIDPF:
		id := m.Identifier
		if id == "" && len(m.Identifiers) > 0 {
			id = m.Identifiers[0]
		}
		if id == "" {
			return nil, fmt.Errorf("obfuscated font without an identifier: %w", ivy.ErrFormat)
		}
		// the key is the SHA-1 of the identifier without its white space. SHA-1
		// is a key of obfuscation here, not a guard of security
		sum := sha1.Sum([]byte(strings.Map(func(r rune) rune {
			switch r {
			case ' ', '\t', '\r', '\n':
				return -1
			}
			return r
		}, id)))
		key, length = sum[:], idpfLength
	case algAdobe:
		for _, id := range m.Identifiers {
			if k, ok := uuidKey(id); ok {
				key = k
				break
			}
		}
		if key == nil {
			return nil, fmt.Errorf("obfuscated font without a uuid: %w", ivy.ErrFormat)
		}
		length = adobeLength
	default:
		return nil, fmt.Errorf("font obfuscation %s: %w", algorithm, ivy.ErrUnsupported)
	}
	return func(data []byte) ([]byte, error) {
		out := make([]byte, len(data))
		copy(out, data)
		for i := range min(length, len(out)) {
			out[i] ^= key[i%len(key)]
		}
		return out, nil
	}, nil
}

// uuidKey returns the 16 bytes of the UUID of an identifier such as
// urn:uuid:12345678-1234-1234-1234-123456789abc.
func uuidKey(id string) ([]byte, bool) {
	id = strings.TrimSpace(id)
	if len(id) >= 9 && strings.EqualFold(id[:9], "urn:uuid:") {
		id = id[9:]
	}
	id = strings.NewReplacer("-", "", ":", "", "{", "", "}", "").Replace(id)
	if len(id) != 32 {
		return nil, false
	}
	key, err := hex.DecodeString(id)
	if err != nil {
		return nil, false
	}
	return key, true
}
