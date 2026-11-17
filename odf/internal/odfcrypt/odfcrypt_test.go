package odfcrypt

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// readZip returns the entries of the zip archive in the file name of the
// test data of odf.
func readZip(t testing.TB, buf []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		if err := rc.Close(); err != nil {
			t.Fatal(err)
		}
		entries[f.Name] = data
	}
	return entries
}

func readFile(t testing.TB, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

// TestLibreOffice decrypts the files that LibreOffice wrote with the
// password "secret".
func TestLibreOffice(t *testing.T) {
	tests := []struct {
		name string
		alg  Algorithm
	}{
		{"encrypted_blowfish.odt", Blowfish},
		{"encrypted_aes.odt", AESCBC},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entries := readZip(t, readFile(t, test.name))
			m, err := ParseManifest(entries["META-INF/manifest.xml"], ivy.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if m.Mime != "application/vnd.oasis.opendocument.text" {
				t.Errorf("expected the mime type of a text, got: %q", m.Mime)
			}
			if !m.Encrypted() {
				t.Fatal("expected an encrypted manifest")
			}
			enc := m.Encryption("content.xml")
			if enc == nil || enc.Algorithm != test.alg {
				t.Fatalf("expected content.xml encrypted with %d, got: %+v", test.alg, enc)
			}
			if _, err := NewKeyring("wrong").Decrypt(t.Context(), enc, entries["content.xml"], ivy.Limits{}); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ivy.ErrPassword, got: %v", err)
			}
			if _, err := NewKeyring("").Decrypt(t.Context(), enc, entries["content.xml"], ivy.Limits{}); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ivy.ErrPassword for an empty password, got: %v", err)
			}
			kr := NewKeyring("secret")
			for path, e := range m.Entries {
				if e.Encryption == nil {
					continue
				}
				plain, err := kr.Decrypt(t.Context(), e.Encryption, entries[path], ivy.Limits{})
				if err != nil {
					t.Fatalf("%s: %v", path, err)
				}
				if int64(len(plain)) != e.Size {
					t.Errorf("%s: expected %d bytes, got: %d", path, e.Size, len(plain))
				}
				if path == "content.xml" && !bytes.Contains(plain, []byte("An encrypted OpenDocument text")) {
					t.Errorf("expected the text in content.xml, got: %.200s", plain)
				}
			}
			if _, err := kr.Decrypt(t.Context(), enc, entries["content.xml"], ivy.Limits{MaxMemory: 100}); !errors.Is(err, ivy.ErrLimit) {
				t.Errorf("expected ivy.ErrLimit, got: %v", err)
			}
		})
	}
}

// TestArgon2id decrypts the package that LibreOffice encrypts by default
// for ODF 1.4, with AES-GCM and a key from Argon2id.
func TestArgon2id(t *testing.T) {
	entries := readZip(t, readFile(t, "encrypted_argon2id.odt"))
	m, err := ParseManifest(entries["META-INF/manifest.xml"], ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	enc := m.Encryption(PackageName)
	if enc == nil || enc.Algorithm != AESGCM || enc.Derivation != Argon2id {
		t.Fatalf("expected the package encrypted with aes gcm and argon2id, got: %+v", enc)
	}
	if enc.Iterations != 3 || enc.Memory != 65536 || enc.Lanes != 4 {
		t.Errorf("expected 3 passes, 65536 KiB and 4 lanes, got: %d, %d, %d", enc.Iterations, enc.Memory, enc.Lanes)
	}
	plain, err := NewKeyring("secret").Decrypt(t.Context(), enc, entries[PackageName], ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if inner := readZip(t, plain); inner["content.xml"] == nil {
		t.Error("expected content.xml in the package")
	}
	if _, err := NewKeyring("wrong").Decrypt(t.Context(), enc, entries[PackageName], ivy.Limits{}); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword, got: %v", err)
	}
	if _, err := NewKeyring("secret").Decrypt(t.Context(), enc, entries[PackageName], ivy.Limits{MaxMemory: 1 << 20}); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit for 64 MiB of memory, got: %v", err)
	}
}

// TestRoundTrip decrypts the files that the fixture encrypts with each
// cipher.
func TestRoundTrip(t *testing.T) {
	text := []byte(strings.Repeat("<text>ivy decrypts OpenDocument.</text>", 100))
	small := []byte("<a/>")
	for _, c := range []fixture.ODFCipher{fixture.ODFBlowfish, fixture.ODFAESCBC, fixture.ODFAESGCM} {
		buf := fixture.EncryptODF(fixture.ODFEncryption{Password: "pässwörd", Cipher: c, Iterations: 10},
			"application/vnd.oasis.opendocument.text",
			fixture.Entry{Name: "content.xml", Data: text},
			fixture.Entry{Name: "styles.xml", Data: small})
		entries := readZip(t, buf)
		m, err := ParseManifest(entries["META-INF/manifest.xml"], ivy.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		kr := NewKeyring("pässwörd")
		for name, want := range map[string][]byte{"content.xml": text, "styles.xml": small} {
			got, err := kr.Decrypt(t.Context(), m.Encryption(name), entries[name], ivy.Limits{})
			if err != nil {
				t.Fatalf("cipher %d, %s: %v", c, name, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("cipher %d, %s: the entry differs", c, name)
			}
			if _, err := NewKeyring("password").Decrypt(t.Context(), m.Encryption(name), entries[name], ivy.Limits{}); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("cipher %d, %s: expected ivy.ErrPassword, got: %v", c, name, err)
			}
		}
	}
}

func TestParseManifestErrors(t *testing.T) {
	entry := func(data string) []byte {
		return []byte(`<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0">
<manifest:file-entry manifest:full-path="a.xml">` + data + `</manifest:file-entry></manifest:manifest>`)
	}
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"not xml", []byte("<manifest"), ivy.ErrFormat},
		{"invalid utf-8", []byte("<manifest>\xff</manifest>"), ivy.ErrFormat},
		{"wrong root", []byte("<x/>"), ivy.ErrFormat},
		{"no cipher", entry(`<manifest:encryption-data/>`), ivy.ErrFormat},
		{"unknown cipher", entry(`<manifest:encryption-data><manifest:algorithm manifest:algorithm-name="rot13"/></manifest:encryption-data>`), ivy.ErrUnsupported},
		{"unknown checksum", entry(`<manifest:encryption-data manifest:checksum-type="crc"/>`), ivy.ErrUnsupported},
		{"bad base64", entry(`<manifest:encryption-data><manifest:algorithm manifest:algorithm-name="Blowfish CFB" manifest:initialisation-vector="!"/></manifest:encryption-data>`), ivy.ErrFormat},
		{"iterations", entry(`<manifest:encryption-data><manifest:key-derivation manifest:key-derivation-name="PBKDF2" manifest:iteration-count="100000000"/></manifest:encryption-data>`), ivy.ErrLimit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseManifest(test.buf, ivy.Limits{}); !errors.Is(err, test.want) {
				t.Errorf("expected %v, got: %v", test.want, err)
			}
		})
	}
	if _, err := ParseManifest(entry(""), ivy.Limits{MaxObjects: 2}); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
}

func FuzzParseManifest(f *testing.F) {
	for _, name := range []string{"encrypted_blowfish.odt", "encrypted_aes.odt", "encrypted_argon2id.odt", "sample.odt"} {
		f.Add(readZip(f, readFile(f, name))["META-INF/manifest.xml"])
	}
	data := bytes.Repeat([]byte{0x5a}, 64)
	f.Fuzz(func(t *testing.T, buf []byte) {
		m, err := ParseManifest(buf, ivy.Limits{MaxObjects: 1 << 12})
		if err != nil {
			return
		}
		kr := NewKeyring("secret")
		for _, e := range m.Entries {
			if e.Encryption == nil || e.Encryption.Iterations > 1000 {
				// The iteration count bounds the time, and a fuzz test
				// cannot wait for a large one.
				continue
			}
			_, _ = kr.Decrypt(t.Context(), e.Encryption, data, ivy.Limits{MaxMemory: 1 << 20})
		}
	})
}

func TestWindows1252(t *testing.T) {
	for _, test := range []struct {
		password string
		exp      string
		ok       bool
	}{
		{"café", "caf\xe9", true},
		{"€uro", "\x80uro", true},
		{"ascii", "", false},
		{"日本", "", false},
	} {
		got, ok := Windows1252(test.password)
		if got != test.exp || ok != test.ok {
			t.Errorf("%q: expected %q and %v, got: %q and %v", test.password, test.exp, test.ok, got, ok)
		}
	}
}
