// Package offcrypto decrypts the Office files that a password protects, as
// MS-OFFCRYPTO describes. It reads four schemes:
//
//   - Agile encryption of ECMA-376, version 4.4, for Office Open XML.
//   - Standard encryption of ECMA-376, versions 2.2, 3.2 and 4.2, for Office
//     Open XML.
//   - RC4 CryptoAPI encryption, for the binary formats .doc, .xls and .ppt.
//   - RC4 encryption, version 1.1, for .doc and .xls.
//
// The package works on bytes. A caller reads the streams from the Compound
// File, such as EncryptionInfo and EncryptedPackage, and gives them here.
// For a binary format, the caller finds the encryption header in the format,
// gives it to ParseEncryptionInfo, and decrypts each stream with the block
// rules of the format, through Key.DecryptAt or Key.Stream.
//
// The package is written from the specification. nolze/msoffcrypto-tool
// (MIT) was a reference, and its test files are in testdata.
package offcrypto

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"unicode/utf16"

	"github.com/xo/ivy"
)

// MaxSpinCount is the largest spin count of Agile encryption that the
// package accepts. It is the largest that MS-OFFCRYPTO allows. Office writes
// 100000. The context bounds the time of the work.
const MaxSpinCount = 10_000_000

// Scheme is a scheme of encryption of MS-OFFCRYPTO.
type Scheme int

// The schemes.
const (
	// Agile is the Agile encryption of ECMA-376, version 4.4.
	Agile Scheme = iota + 1
	// Standard is the Standard encryption of ECMA-376, with AES.
	Standard
	// RC4CryptoAPI is the RC4 CryptoAPI encryption of the binary formats.
	RC4CryptoAPI
	// RC4 is the RC4 encryption of the binary formats, version 1.1.
	RC4
)

// String satisfies the fmt.Stringer interface.
func (s Scheme) String() string {
	switch s {
	case Agile:
		return "agile"
	case Standard:
		return "standard"
	case RC4CryptoAPI:
		return "rc4 cryptoapi"
	case RC4:
		return "rc4"
	}
	return fmt.Sprintf("Scheme(%d)", int(s))
}

// Info is a parsed EncryptionInfo stream, or the encryption header of a
// binary format.
type Info struct {
	// Scheme is the scheme of the encryption.
	Scheme Scheme
	// Major and Minor are the version of the structure.
	Major, Minor uint16

	agile    *agileInfo
	standard *standardInfo
	rc4      *rc4Info
}

// ParseEncryptionInfo parses the EncryptionInfo stream of an Office Open XML
// file, or the encryption header of a binary format, which starts with the
// same version. It returns an error that wraps ivy.ErrFormat if buf is
// malformed, and one that wraps ivy.ErrUnsupported for a scheme or an
// algorithm that the package does not read, such as Extensible encryption,
// a certificate or DES.
func ParseEncryptionInfo(buf []byte) (*Info, error) {
	if len(buf) < 4 {
		return nil, fmt.Errorf("encryption info of %d bytes: %w", len(buf), ivy.ErrFormat)
	}
	major := binary.LittleEndian.Uint16(buf)
	minor := binary.LittleEndian.Uint16(buf[2:])
	in := &Info{Major: major, Minor: minor}
	var err error
	switch {
	case major == 4 && minor == 4:
		in.Scheme = Agile
		in.agile, err = parseAgile(buf[4:])
	case (major == 2 || major == 3 || major == 4) && minor == 2:
		in.standard, err = parseStandard(buf[4:])
		if err == nil {
			in.Scheme = Standard
			if in.standard.rc4 {
				in.Scheme = RC4CryptoAPI
			}
		}
	case major == 1 && minor == 1:
		in.Scheme = RC4
		in.rc4, err = parseRC4(buf[4:])
	case (major == 3 || major == 4) && minor == 3:
		return nil, fmt.Errorf("extensible encryption %d.%d: %w", major, minor, ivy.ErrUnsupported)
	default:
		return nil, fmt.Errorf("encryption version %d.%d: %w", major, minor, ivy.ErrFormat)
	}
	if err != nil {
		return nil, err
	}
	return in, nil
}

// Key derives the key of the password, and makes sure that the password is
// right. It returns an error that wraps ivy.ErrPassword if the password is
// wrong, and the error of ctx if ctx ends during the work.
func (in *Info) Key(ctx context.Context, password string) (*Key, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch in.Scheme {
	case Agile:
		return in.agile.key(ctx, password)
	case Standard, RC4CryptoAPI:
		return in.standard.key(password)
	case RC4:
		return in.rc4.key(password)
	}
	return nil, fmt.Errorf("scheme %v: %w", in.Scheme, ivy.ErrUnsupported)
}

// Key is the key of a password that the scheme of an Info accepts.
type Key struct {
	scheme Scheme
	// secret is the key of the package for Agile and Standard, the hash of
	// the salt and the password for RC4 CryptoAPI, and the truncated hash H1
	// for RC4.
	secret []byte
	// keyLen is the number of bytes of the RC4 key of a block.
	keyLen   int
	agile    *agileInfo
	standard *standardInfo
}

// Scheme returns the scheme of the key.
func (k *Key) Scheme() Scheme {
	return k.scheme
}

// passwordBytes returns the password in UTF-16LE, as every scheme hashes it.
func passwordBytes(password string) []byte {
	units := utf16.Encode([]rune(password))
	buf := make([]byte, 2*len(units))
	for i, u := range units {
		binary.LittleEndian.PutUint16(buf[2*i:], u)
	}
	return buf
}

// equal reports whether the hash a starts with the n bytes of b, in
// constant time.
func equal(a, b []byte, n int) bool {
	if n <= 0 || len(a) < n || len(b) < n {
		return false
	}
	return subtle.ConstantTimeCompare(a[:n], b[:n]) == 1
}

// fit returns buf cut or padded to n bytes. The padding is the byte 0x36,
// as MS-OFFCRYPTO pads a hash.
func fit(buf []byte, n int) []byte {
	if len(buf) >= n {
		return buf[:n]
	}
	return append(bytes.Clone(buf), bytes.Repeat([]byte{0x36}, n-len(buf))...)
}

// le32 returns n in 4 bytes, little endian.
func le32(n uint32) []byte {
	return binary.LittleEndian.AppendUint32(nil, n)
}
