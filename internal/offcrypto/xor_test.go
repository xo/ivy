package offcrypto

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/bits"
	"testing"

	"github.com/xo/ivy"
)

func TestXORMethod1Values(t *testing.T) {
	// The values of the test of the XOR obfuscation of Apache POI, and the
	// verifier of the default password of Excel from msoffcrypto-tool. POI
	// keeps each element of the array rotated left by 3 bits, so that it
	// decrypts with a rotation before the XOR.
	tests := []struct {
		password string
		key      uint16
		verifier uint16
		array    string
	}{
		{"abc", 20810, 52250, "accca4abd6bac3bad6a32b45d37929bb"},
		{"", 0, 0, "eeffffeaffffe60200fa3c00fe3c0000"},
		{"VelvetSweatshop", 0, 0x9A0A, ""},
	}
	for _, test := range tests {
		pw := xorPassword(test.password)
		if test.key != 0 || test.password == "" {
			if key := xorKey1(pw); key != test.key {
				t.Errorf("%q: expected the key %d, got: %d", test.password, test.key, key)
			}
		}
		if v := xorVerifier1(pw); v != test.verifier {
			t.Errorf("%q: expected the verifier %d, got: %d", test.password, test.verifier, v)
		}
		if test.array != "" {
			a := xorArray1(pw)
			for i, x := range a {
				a[i] = bits.RotateLeft8(x, 3)
			}
			if got := hex.EncodeToString(a[:]); got != test.array {
				t.Errorf("%q: expected the array %s, got: %s", test.password, test.array, got)
			}
		}
	}
}

func TestXORExcel(t *testing.T) {
	wb := readFile(t, "xor_xls.workbook")
	le := binary.LittleEndian
	// the first records: BOF, INTERFACEHDR, FILEPASS
	off := 0
	var filePass []byte
	for off+4 <= len(wb) && filePass == nil {
		typ, n := le.Uint16(wb[off:]), int(le.Uint16(wb[off+2:]))
		if typ == 0x002F {
			filePass = wb[off+4 : off+4+n]
		}
		off += 4 + n
	}
	if len(filePass) != 6 || le.Uint16(filePass) != 0 {
		t.Fatalf("expected a filepass of the xor obfuscation, got: %x", filePass)
	}
	info := XORMethod1(le.Uint16(filePass[2:]), le.Uint16(filePass[4:]))
	if _, err := info.Key(t.Context(), "wrong"); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword, got: %v", err)
	}
	key, err := info.Key(t.Context(), "123456789012345")
	if err != nil {
		t.Fatal(err)
	}
	if key.Scheme() != XOR {
		t.Errorf("expected %v, got: %v", XOR, key.Scheme())
	}
	// the records after FILEPASS: INTERFACEHDR, MMS, INTERFACEEND and
	// WRITEACCESS, whose name has spaces after it
	want := map[uint16]func([]byte) bool{
		0x00C1: func(b []byte) bool { return bytes.Equal(b, []byte{0, 0}) },
		0x005C: func(b []byte) bool { return len(b) == 112 && bytes.HasSuffix(b, bytes.Repeat([]byte(" "), 20)) },
		0x0042: func(b []byte) bool { return len(b) == 2 && le.Uint16(b) == 1200 },
	}
	found := 0
	for off+4 <= len(wb) {
		typ, n := le.Uint16(wb[off:]), int(le.Uint16(wb[off+2:]))
		if off+4+n > len(wb) {
			break
		}
		data := bytes.Clone(wb[off+4 : off+4+n])
		if typ != 0x00E1 {
			if err := key.DecryptAt(data, int64(off+4+n), 0); err != nil {
				t.Fatal(err)
			}
		}
		if check, ok := want[typ]; ok {
			found++
			if !check(data) {
				t.Errorf("record %#x: unexpected data %x", typ, data)
			}
		}
		off += 4 + n
	}
	if found != len(want) {
		t.Errorf("expected %d records, got: %d", len(want), found)
	}
}

func TestXORWord(t *testing.T) {
	const password = "Secret"
	pw := xorPassword(password)
	info := XORMethod2(XORVerifier2(password))
	for _, wrong := range []string{"secret", ""} {
		if _, err := info.Key(t.Context(), wrong); !errors.Is(err, ivy.ErrPassword) {
			t.Errorf("expected ivy.ErrPassword for %q, got: %v", wrong, err)
		}
	}
	if _, err := XORMethod2(0).Key(t.Context(), ""); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword for the empty password, got: %v", err)
	}
	key, err := info.Key(t.Context(), password)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("\x00\x00 a text of Word, with zeros \x00\x00 and more text after them")
	enc := bytes.Clone(plain)
	a := xorArray2(pw)
	for i, b := range enc {
		if x := a[(i+70)%16]; b != 0 && b != x {
			enc[i] = b ^ x
		}
	}
	if bytes.Equal(enc, plain) {
		t.Fatal("expected the obfuscation to change the text")
	}
	if err := key.DecryptAt(enc, 70, 512); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(enc, plain) {
		t.Errorf("expected %q, got: %q", plain, enc)
	}
	if err := key.DecryptAt(enc, -1, 512); err == nil {
		t.Error("expected an error for a negative offset")
	}
	if _, err := key.Stream(0); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ivy.ErrUnsupported, got: %v", err)
	}
}

func TestXORPassword(t *testing.T) {
	tests := []struct {
		password string
		want     []byte
	}{
		{"abc", []byte("abc")},
		{"0123456789abcdefgh", []byte("0123456789abcde")},
		{"Ā中", []byte{0x01, 0x2d}},
	}
	for _, test := range tests {
		if got := xorPassword(test.password); !bytes.Equal(got, test.want) {
			t.Errorf("%q: expected %x, got: %x", test.password, test.want, got)
		}
	}
}
