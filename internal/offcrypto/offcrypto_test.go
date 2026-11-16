package offcrypto

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
)

// password is the password of every test file.
const password = "Password1234_"

// loPassword is the password of the files that LibreOffice wrote.
const loPassword = "secret"

func readFile(t testing.TB, name string) []byte {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return buf
}

func TestPackages(t *testing.T) {
	tests := []struct {
		base     string
		password string
		scheme   Scheme
		// sum is the SHA-256 of the decrypted package, or empty if the test
		// only makes sure that the package is a zip archive.
		sum string
	}{
		{"agile_sha512", password, Agile, "4dd9dd0ccbfc7fb8769f1f3307830d3cc4c5042e32d619f4b2835fada89d13c6"},
		{"agile_sha1", password, Agile, "4dd9dd0ccbfc7fb8769f1f3307830d3cc4c5042e32d619f4b2835fada89d13c6"},
		{"standard", password, Standard, "ca1c0ebb465553361b9034e696d4081df0a2d41918f820060325b3ca634eb69b"},
		{"libreoffice_standard", loPassword, Standard, ""},
	}
	for _, test := range tests {
		t.Run(test.base, func(t *testing.T) {
			info, err := ParseEncryptionInfo(readFile(t, test.base+".info"))
			if err != nil {
				t.Fatal(err)
			}
			if info.Scheme != test.scheme {
				t.Fatalf("expected %v, got: %v", test.scheme, info.Scheme)
			}
			if _, err := info.Key(t.Context(), "wrong"); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ivy.ErrPassword for a wrong password, got: %v", err)
			}
			key, err := info.Key(t.Context(), test.password)
			if err != nil {
				t.Fatal(err)
			}
			pkg := readFile(t, test.base+".package")
			plain, err := key.Decrypt(t.Context(), bytes.NewReader(pkg), int64(len(pkg)), ivy.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if test.sum != "" {
				if sum := sha256.Sum256(plain); hex.EncodeToString(sum[:]) != test.sum {
					t.Errorf("expected the sum %s, got: %x", test.sum, sum)
				}
			}
			if _, err := zip.NewReader(bytes.NewReader(plain), int64(len(plain))); err != nil {
				t.Errorf("expected a zip archive, got: %v", err)
			}
			switch err := key.CheckIntegrity(t.Context(), bytes.NewReader(pkg), int64(len(pkg))); {
			case test.scheme == Agile && err != nil:
				t.Errorf("expected the integrity to hold, got: %v", err)
			case test.scheme != Agile && !errors.Is(err, ivy.ErrUnsupported):
				t.Errorf("expected ivy.ErrUnsupported, got: %v", err)
			}
			if test.scheme == Agile {
				bad := bytes.Clone(pkg)
				bad[len(bad)-1] ^= 1
				if err := key.CheckIntegrity(t.Context(), bytes.NewReader(bad), int64(len(bad))); !errors.Is(err, ivy.ErrFormat) {
					t.Errorf("expected ivy.ErrFormat for a changed package, got: %v", err)
				}
			}
			if _, err := key.Decrypt(t.Context(), bytes.NewReader(pkg), int64(len(pkg)), ivy.Limits{MaxMemory: 1024}); !errors.Is(err, ivy.ErrLimit) {
				t.Errorf("expected ivy.ErrLimit, got: %v", err)
			}
			if _, err := key.Decrypt(t.Context(), bytes.NewReader(pkg), 100, ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
				t.Errorf("expected ivy.ErrFormat for a short stream, got: %v", err)
			}
		})
	}
}

// TestWordCryptoAPI decrypts the table stream of a Word file with RC4
// CryptoAPI, in blocks of 512 bytes, and compares it with the stream that
// msoffcrypto-tool decrypted. The first lKey bytes of the stream hold the
// encryption header, in the clear.
func TestWordCryptoAPI(t *testing.T) {
	const lKey = 198
	table := readFile(t, "rc4cryptoapi_doc.table")
	info, err := ParseEncryptionInfo(table[:lKey])
	if err != nil {
		t.Fatal(err)
	}
	if info.Scheme != RC4CryptoAPI {
		t.Fatalf("expected rc4 cryptoapi, got: %v", info.Scheme)
	}
	if _, err := info.Key(t.Context(), "wrong"); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword, got: %v", err)
	}
	key, err := info.Key(t.Context(), password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := key.Decrypt(t.Context(), bytes.NewReader(table), int64(len(table)), ivy.Limits{}); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ivy.ErrUnsupported for the package of rc4, got: %v", err)
	}
	// Decrypt in two calls that split a block, to test the offsets.
	plain := bytes.Clone(table[lKey:])
	if err := key.DecryptAt(plain[:1000], lKey, 512); err != nil {
		t.Fatal(err)
	}
	if err := key.DecryptAt(plain[1000:], lKey+1000, 512); err != nil {
		t.Fatal(err)
	}
	const want = "9e97f8fc3fe1239b42929c62971087426520294420119fd96fb31a155ccbad24"
	if sum := sha256.Sum256(plain); hex.EncodeToString(sum[:]) != want {
		t.Errorf("expected the sum %s, got: %x", want, sum)
	}
}

// TestWordRC4 makes sure that the password of a Word file that LibreOffice
// encrypted with RC4 opens it.
func TestWordRC4(t *testing.T) {
	table := readFile(t, "libreoffice_rc4_doc.table")
	info, err := ParseEncryptionInfo(table)
	if err != nil {
		t.Fatal(err)
	}
	if info.Scheme != RC4 {
		t.Fatalf("expected rc4, got: %v", info.Scheme)
	}
	if _, err := info.Key(t.Context(), "wrong"); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword, got: %v", err)
	}
	if _, err := info.Key(t.Context(), loPassword); err != nil {
		t.Fatal(err)
	}
}

// TestExcel decrypts the globals of two Excel files, in blocks of 1024
// bytes, and makes sure that the record CodePage holds 1200, the code page
// of BIFF8.
func TestExcel(t *testing.T) {
	tests := []struct {
		name     string
		password string
		scheme   Scheme
	}{
		{"rc4cryptoapi_xls.workbook", password, RC4CryptoAPI},
		{"libreoffice_rc4_xls.workbook", loPassword, RC4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wb := readFile(t, test.name)
			var key *Key
			codePage := -1
			for off := 0; off+4 <= len(wb); {
				typ := binary.LittleEndian.Uint16(wb[off:])
				n := int(binary.LittleEndian.Uint16(wb[off+2:]))
				if off+4+n > len(wb) {
					break
				}
				data := wb[off+4 : off+4+n]
				switch {
				case typ == 0x2f:
					info, err := ParseEncryptionInfo(data[2:])
					if err != nil {
						t.Fatal(err)
					}
					if info.Scheme != test.scheme {
						t.Fatalf("expected %v, got: %v", test.scheme, info.Scheme)
					}
					if key, err = info.Key(t.Context(), test.password); err != nil {
						t.Fatal(err)
					}
				case key != nil && typ == 0x42:
					buf := bytes.Clone(data)
					if err := key.DecryptAt(buf, int64(off+4), 1024); err != nil {
						t.Fatal(err)
					}
					codePage = int(binary.LittleEndian.Uint16(buf))
				}
				off += 4 + n
			}
			if codePage != 1200 {
				t.Errorf("expected the code page 1200, got: %d", codePage)
			}
		})
	}
}

// TestAgileRoundTrip encrypts a package with each hash and each size of key
// that Agile encryption allows, and decrypts it.
func TestAgileRoundTrip(t *testing.T) {
	plain := bytes.Repeat([]byte("ivy reads agile encryption. "), 400)
	for _, hashName := range []string{"SHA1", "SHA256", "SHA384", "SHA512", "MD5"} {
		for _, keyBits := range []int{128, 192, 256} {
			info, pkg := agileEncrypt(t, "pässwörd", hashName, keyBits, 1000, plain)
			in, err := ParseEncryptionInfo(info)
			if err != nil {
				t.Fatalf("%s %d: %v", hashName, keyBits, err)
			}
			if _, err := in.Key(t.Context(), "password"); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("%s %d: expected ivy.ErrPassword, got: %v", hashName, keyBits, err)
			}
			key, err := in.Key(t.Context(), "pässwörd")
			if err != nil {
				t.Fatalf("%s %d: %v", hashName, keyBits, err)
			}
			got, err := key.Decrypt(t.Context(), bytes.NewReader(pkg), int64(len(pkg)), ivy.Limits{})
			if err != nil {
				t.Fatalf("%s %d: %v", hashName, keyBits, err)
			}
			if !bytes.Equal(got, plain) {
				t.Errorf("%s %d: the package differs", hashName, keyBits)
			}
			if err := key.CheckIntegrity(t.Context(), bytes.NewReader(pkg), int64(len(pkg))); err != nil {
				t.Errorf("%s %d: %v", hashName, keyBits, err)
			}
		}
	}
}

func TestSpinCount(t *testing.T) {
	info, _ := agileEncrypt(t, "x", "SHA512", 256, MaxSpinCount+1, nil)
	if _, err := ParseEncryptionInfo(info); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
	info, _ = agileEncrypt(t, "x", "SHA512", 256, 1<<20, nil)
	in, err := ParseEncryptionInfo(info)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := in.Key(ctx, "x"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	agile := func(xml string) []byte {
		return append([]byte{4, 0, 4, 0, 0x40, 0, 0, 0}, xml...)
	}
	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"empty", nil, ivy.ErrFormat},
		{"version", []byte{9, 0, 9, 0}, ivy.ErrFormat},
		{"extensible", []byte{3, 0, 3, 0}, ivy.ErrUnsupported},
		{"short standard", []byte{3, 0, 2, 0, 0x24, 0, 0, 0}, ivy.ErrFormat},
		{"short rc4", []byte{1, 0, 1, 0, 1, 2, 3}, ivy.ErrFormat},
		{"not xml", agile("<encryption"), ivy.ErrFormat},
		{"wrong root", agile("<x/>"), ivy.ErrFormat},
		{"no key data", agile("<encryption/>"), ivy.ErrFormat},
		{"certificate only", agile(`<encryption><keyData/></encryption>`), ivy.ErrUnsupported},
		{"des", agile(`<encryption><keyData cipherAlgorithm="DES"/><keyEncryptors><keyEncryptor uri="` +
			passwordURI + `"><encryptedKey/></keyEncryptor></keyEncryptors></encryption>`), ivy.ErrUnsupported},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseEncryptionInfo(test.buf); !errors.Is(err, test.want) {
				t.Errorf("expected %v, got: %v", test.want, err)
			}
		})
	}
}

func FuzzParseEncryptionInfo(f *testing.F) {
	for _, name := range []string{"agile_sha512.info", "agile_sha1.info", "standard.info", "libreoffice_standard.info"} {
		f.Add(readFile(f, name))
	}
	f.Add(readFile(f, "libreoffice_rc4_doc.table")[:52])
	f.Add(readFile(f, "rc4cryptoapi_doc.table")[:198])
	pkg := readFile(f, "standard.package")
	f.Fuzz(func(t *testing.T, buf []byte) {
		info, err := ParseEncryptionInfo(buf)
		if err != nil {
			return
		}
		if info.Scheme == Agile && info.agile.spinCount > 1000 {
			// The spin count bounds the time, and a fuzz test cannot wait
			// for a large one.
			return
		}
		key, err := info.Key(t.Context(), password)
		if err != nil {
			return
		}
		_, _ = key.Decrypt(t.Context(), bytes.NewReader(pkg), int64(len(pkg)), ivy.Limits{MaxMemory: 1 << 20})
		_ = key.CheckIntegrity(t.Context(), bytes.NewReader(pkg), int64(len(pkg)))
		_ = key.DecryptAt(bytes.Clone(pkg[:100]), 1000, 512)
	})
}
