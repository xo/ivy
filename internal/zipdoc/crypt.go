package zipdoc

import (
	"archive/zip"
	"compress/flate"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha1" //nolint:gosec // the AES of WinZip authenticates with HMAC-SHA1
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"

	"github.com/xo/ivy"
)

// The flags and the methods of zip that the decryption reads.
const (
	flagEncrypted   = 0x1
	flagDescriptor  = 0x8
	flagStrong      = 0x40
	methodAES       = 99
	extraAES        = 0x9901
	aesVerifierLen  = 2
	aesAuthLen      = 10
	aesIterations   = 1000
	zipCryptoHeader = 12
)

// errWrongKey is the error of an entry that does not decrypt with a
// password: its check byte, its verifier or its CRC is wrong.
var errWrongKey = fmt.Errorf("wrong password: %w", ivy.ErrPassword)

// Encrypted reports whether the archive encrypts the entry f.
func Encrypted(f *zip.File) bool {
	return f.Flags&flagEncrypted != 0
}

// Unlock finds the password of the encrypted entry f, and keeps it for each
// encrypted entry of the archive. It tries the password of cfg, then, while
// the password is wrong, it asks the function PasswordFunc of cfg for
// another one, as D12 says. It reads the whole entry to check a password, so
// a caller gives the smallest encrypted entry. It returns an error that
// wraps ivy.ErrPassword if no password is right.
//
// An archive that a format does not unlock refuses each encrypted entry with
// ivy.ErrUnsupported.
func (a *Archive) Unlock(ctx context.Context, f *zip.File, cfg ivy.Config) error {
	a.keyed = true
	err := a.try(f, cfg.Password)
	for attempt := 1; errors.Is(err, ivy.ErrPassword) && cfg.PasswordFunc != nil; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("unlocking zip archive: %w", cerr)
		}
		password, perr := cfg.PasswordFunc(ctx, attempt)
		if perr != nil {
			return fmt.Errorf("asking for the password: %w: %w", ivy.ErrPassword, perr)
		}
		err = a.try(f, password)
	}
	if err != nil {
		return err
	}
	return nil
}

// UnlockSmallest unlocks the archive with Unlock and its smallest encrypted
// entry, for a format that does not choose the entry. An archive without
// encrypted entries needs no password, and UnlockSmallest does nothing.
func (a *Archive) UnlockSmallest(ctx context.Context, cfg ivy.Config) error {
	var smallest *zip.File
	for _, f := range a.zr.File {
		if Encrypted(f) && !f.FileInfo().IsDir() && (smallest == nil || f.CompressedSize64 < smallest.CompressedSize64) {
			smallest = f
		}
	}
	if smallest == nil {
		return nil
	}
	return a.Unlock(ctx, smallest, cfg)
}

// try reads the entry f with password, and keeps the password if the entry
// decrypts.
func (a *Archive) try(f *zip.File, password string) error {
	rc, err := a.decrypt(f, password)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, io.LimitReader(rc, a.lim.MaxMemory)); err != nil {
		if errors.Is(err, ivy.ErrPassword) || errors.Is(err, ivy.ErrFormat) {
			return fmt.Errorf("reading %s: %w", f.Name, err)
		}
		return fmt.Errorf("reading %s: %w: %w", f.Name, ivy.ErrFormat, err)
	}
	a.password = password
	return nil
}

// decrypt opens the encrypted entry f with password. The reader that it
// returns checks the CRC of the entry at its end, and returns an error that
// wraps ivy.ErrPassword if it is wrong. It checks the authentication code of
// AES too, and returns an error that wraps ivy.ErrFormat if it is wrong.
func (a *Archive) decrypt(f *zip.File, password string) (io.ReadCloser, error) {
	if f.Flags&flagStrong != 0 {
		return nil, fmt.Errorf("zip entry %s with strong encryption: %w", f.Name, ivy.ErrUnsupported)
	}
	raw, err := f.OpenRaw()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w: %w", f.Name, ivy.ErrFormat, err)
	}
	method, checkCRC := f.Method, true
	var plain io.Reader
	if f.Method == methodAES {
		var ae aesExtra
		if ae, err = readAESExtra(f.Extra); err != nil {
			return nil, fmt.Errorf("zip entry %s: %w", f.Name, err)
		}
		method = ae.method
		// AE-2 sets the CRC to 0, and only the authentication code checks
		// the data
		checkCRC = ae.version == 1
		if plain, err = aesReader(raw, f.CompressedSize64, ae.strength, password); err != nil {
			return nil, fmt.Errorf("zip entry %s: %w", f.Name, err)
		}
	} else {
		check := byte(f.CRC32 >> 24)
		if f.Flags&flagDescriptor != 0 {
			// the CRC follows the data, so the check byte is the high byte
			// of the time
			check = byte(f.ModifiedTime >> 8)
		}
		if plain, err = zipCryptoReader(raw, f.CompressedSize64, check, password); err != nil {
			return nil, fmt.Errorf("zip entry %s: %w", f.Name, err)
		}
	}
	var rc io.ReadCloser
	switch method {
	case zip.Store:
		rc = io.NopCloser(plain)
	case zip.Deflate:
		rc = flate.NewReader(plain)
	default:
		return nil, fmt.Errorf("zip entry %s of method %d: %w", f.Name, method, ivy.ErrUnsupported)
	}
	// the decompressor can stop before the end of the encrypted data, and
	// the authentication code of AES follows it
	rc = &tailReader{rc: rc, tail: plain}
	if !checkCRC {
		return rc, nil
	}
	return &crcReader{rc: rc, want: f.CRC32, h: crc32.NewIEEE()}, nil
}

// tailReader reads tail to its end after the end of rc, so that a check at
// the end of tail runs.
type tailReader struct {
	rc   io.ReadCloser
	tail io.Reader
}

// Read satisfies the io.Reader interface.
func (r *tailReader) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	if errors.Is(err, io.EOF) {
		if _, terr := io.Copy(io.Discard, r.tail); terr != nil {
			return n, fmt.Errorf("reading the end of the entry: %w", terr)
		}
	}
	// io.EOF stays as it is, so that the caller sees the end
	return n, err //nolint:wrapcheck
}

// Close satisfies the io.Closer interface.
func (r *tailReader) Close() error {
	return r.rc.Close() //nolint:wrapcheck // the error of the decompressor
}

// crcReader checks the CRC of the data that it reads at the end.
type crcReader struct {
	rc   io.ReadCloser
	want uint32
	h    hash.Hash32
}

// Read satisfies the io.Reader interface.
func (r *crcReader) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	r.h.Write(p[:n])
	if errors.Is(err, io.EOF) && r.h.Sum32() != r.want {
		// the check byte of ZipCrypto lets 1 wrong password in 256 pass
		return n, errWrongKey
	}
	// io.EOF stays as it is, so that the caller sees the end
	return n, err //nolint:wrapcheck
}

// Close satisfies the io.Closer interface.
func (r *crcReader) Close() error {
	return r.rc.Close() //nolint:wrapcheck // the error of the decompressor
}

// zipKeys are the three keys of ZipCrypto, the traditional encryption of
// PKWARE (APPNOTE.TXT, 6.1).
type zipKeys [3]uint32

// newZipKeys returns the keys of password.
func newZipKeys(password string) *zipKeys {
	k := &zipKeys{0x12345678, 0x23456789, 0x34567890}
	for i := range len(password) {
		k.update(password[i])
	}
	return k
}

// crc updates the CRC-32 c with the byte b.
func crc(c uint32, b byte) uint32 {
	return crc32.IEEETable[byte(c)^b] ^ c>>8
}

// update updates the keys with a byte of plain text.
func (k *zipKeys) update(b byte) {
	k[0] = crc(k[0], b)
	k[1] = (k[1]+k[0]&0xff)*134775813 + 1
	k[2] = crc(k[2], byte(k[1]>>24))
}

// decrypt decrypts buf in place.
func (k *zipKeys) decrypt(buf []byte) {
	for i, c := range buf {
		t := k[2] | 2
		p := c ^ byte((t*(t^1))>>8)
		k.update(p)
		buf[i] = p
	}
}

// zipCryptoReader returns the reader of the plain text of an entry of size
// bytes that ZipCrypto encrypts. The last byte of the header of 12 bytes
// must be check.
func zipCryptoReader(raw io.Reader, size uint64, check byte, password string) (io.Reader, error) {
	if size < zipCryptoHeader {
		return nil, fmt.Errorf("short encryption header: %w", ivy.ErrFormat)
	}
	k := newZipKeys(password)
	var head [zipCryptoHeader]byte
	if _, err := io.ReadFull(raw, head[:]); err != nil {
		return nil, fmt.Errorf("reading encryption header: %w: %w", ivy.ErrFormat, err)
	}
	k.decrypt(head[:])
	if head[zipCryptoHeader-1] != check {
		return nil, errWrongKey
	}
	return &zipCryptoStream{r: io.LimitReader(raw, int64(size-zipCryptoHeader)), k: k}, nil
}

// zipCryptoStream decrypts the data of an entry that ZipCrypto encrypts.
type zipCryptoStream struct {
	r io.Reader
	k *zipKeys
}

// Read satisfies the io.Reader interface.
func (s *zipCryptoStream) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	s.k.decrypt(p[:n])
	// io.EOF stays as it is, so that the caller sees the end
	return n, err //nolint:wrapcheck
}

// aesExtra is the extra field 0x9901 of the AES of WinZip.
type aesExtra struct {
	version  uint16
	strength byte
	method   uint16
}

// readAESExtra reads the extra field of the AES of WinZip from the extra
// fields of an entry.
func readAESExtra(extra []byte) (aesExtra, error) {
	for len(extra) >= 4 {
		id, n := binary.LittleEndian.Uint16(extra), int(binary.LittleEndian.Uint16(extra[2:]))
		if n > len(extra)-4 {
			break
		}
		body := extra[4 : 4+n]
		extra = extra[4+n:]
		if id != extraAES || n < 7 || string(body[2:4]) != "AE" {
			continue
		}
		ae := aesExtra{
			version:  binary.LittleEndian.Uint16(body),
			strength: body[4],
			method:   binary.LittleEndian.Uint16(body[5:]),
		}
		if ae.version != 1 && ae.version != 2 || ae.strength < 1 || ae.strength > 3 {
			return aesExtra{}, fmt.Errorf("aes extra field: %w", ivy.ErrUnsupported)
		}
		return ae, nil
	}
	return aesExtra{}, fmt.Errorf("entry of method 99 without an aes extra field: %w", ivy.ErrFormat)
}

// aesReader returns the reader of the plain text of an entry of size bytes
// that the AES of WinZip encrypts with a key of the given strength: 1, 2 or
// 3 for 128, 192 or 256 bits. The entry is the salt, the password
// verifier, the encrypted data, and the authentication code.
func aesReader(raw io.Reader, size uint64, strength byte, password string) (io.Reader, error) {
	keyLen := 8 + 8*int(strength)
	saltLen := keyLen / 2
	overhead := uint64(saltLen + aesVerifierLen + aesAuthLen)
	if size < overhead {
		return nil, fmt.Errorf("short aes entry: %w", ivy.ErrFormat)
	}
	head := make([]byte, saltLen+aesVerifierLen)
	if _, err := io.ReadFull(raw, head); err != nil {
		return nil, fmt.Errorf("reading aes header: %w: %w", ivy.ErrFormat, err)
	}
	keys, err := pbkdf2.Key(sha1.New, password, head[:saltLen], aesIterations, 2*keyLen+aesVerifierLen)
	if err != nil {
		return nil, fmt.Errorf("deriving the aes key: %w", err)
	}
	if subtle.ConstantTimeCompare(keys[2*keyLen:], head[saltLen:]) != 1 {
		return nil, errWrongKey
	}
	block, err := aes.NewCipher(keys[:keyLen])
	if err != nil {
		return nil, fmt.Errorf("aes key: %w", err)
	}
	return &aesStream{
		r:     raw,
		left:  int64(size - overhead),
		block: block,
		mac:   hmac.New(sha1.New, keys[keyLen:2*keyLen]),
		// the counter of WinZip is little-endian, and starts at 1
		counter: [aes.BlockSize]byte{1},
		used:    aes.BlockSize,
	}, nil
}

// aesStream decrypts the data of an entry that the AES of WinZip encrypts,
// in the mode CTR with a counter that is little-endian, and checks its
// authentication code at the end.
type aesStream struct {
	r       io.Reader
	left    int64
	block   cipher.Block
	mac     hash.Hash
	counter [aes.BlockSize]byte
	stream  [aes.BlockSize]byte
	// used is the number of bytes of stream that the stream used.
	used int
	done bool
}

// Read satisfies the io.Reader interface.
func (s *aesStream) Read(p []byte) (int, error) {
	if s.left <= 0 {
		if err := s.finish(); err != nil {
			return 0, err
		}
		return 0, io.EOF
	}
	if int64(len(p)) > s.left {
		p = p[:s.left]
	}
	n, err := s.r.Read(p)
	s.left -= int64(n)
	s.mac.Write(p[:n])
	for i := range p[:n] {
		if s.used == aes.BlockSize {
			s.block.Encrypt(s.stream[:], s.counter[:])
			for j := range s.counter {
				s.counter[j]++
				if s.counter[j] != 0 {
					break
				}
			}
			s.used = 0
		}
		p[i] ^= s.stream[s.used]
		s.used++
	}
	switch {
	case errors.Is(err, io.EOF) && s.left > 0:
		return n, fmt.Errorf("short aes entry: %w: %w", ivy.ErrFormat, io.ErrUnexpectedEOF)
	case err != nil && !errors.Is(err, io.EOF):
		return n, fmt.Errorf("reading aes entry: %w", err)
	}
	return n, nil
}

// finish reads the authentication code once, and checks it.
func (s *aesStream) finish() error {
	if s.done {
		return nil
	}
	s.done = true
	var code [aesAuthLen]byte
	if _, err := io.ReadFull(s.r, code[:]); err != nil {
		return fmt.Errorf("reading aes authentication code: %w: %w", ivy.ErrFormat, err)
	}
	if !hmac.Equal(s.mac.Sum(nil)[:aesAuthLen], code[:]) {
		// the verifier lets 1 wrong password in 65536 pass, so a wrong
		// code is more likely a change of the data
		return fmt.Errorf("aes authentication code: %w", ivy.ErrFormat)
	}
	return nil
}
