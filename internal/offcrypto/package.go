package offcrypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/xo/ivy"
)

// segmentSize is the size of a segment of the package in Agile encryption.
// Standard encryption has no segments, and the package reads it in pieces
// of the same size.
const segmentSize = 4096

// Decrypt decrypts the EncryptedPackage stream of an Office Open XML file,
// which r holds in its first size bytes. It returns the package, a zip
// archive. Only the keys of Agile and Standard encryption decrypt a
// package.
//
// It returns an error that wraps ivy.ErrLimit if the package is larger than
// lim.MaxMemory, and one that wraps ivy.ErrFormat if the stream is shorter
// than its header says.
func (k *Key) Decrypt(ctx context.Context, r io.ReaderAt, size int64, lim ivy.Limits) ([]byte, error) {
	if k.scheme != Agile && k.scheme != Standard {
		return nil, fmt.Errorf("package of %v encryption: %w", k.scheme, ivy.ErrUnsupported)
	}
	lim = lim.WithDefaults()
	var header [8]byte
	if size < 8 {
		return nil, fmt.Errorf("encrypted package of %d bytes: %w", size, ivy.ErrFormat)
	}
	if err := readAt(r, header[:], 0); err != nil {
		return nil, fmt.Errorf("reading encrypted package: %w: %w", ivy.ErrFormat, err)
	}
	n := binary.LittleEndian.Uint64(header[:])
	if n > uint64(lim.MaxMemory) {
		return nil, fmt.Errorf("package of %d bytes: %w", n, ivy.ErrLimit)
	}
	if n > uint64(size-8) {
		return nil, fmt.Errorf("package of %d bytes in a stream of %d bytes: %w", n, size, ivy.ErrFormat)
	}
	b, err := aes.NewCipher(k.secret)
	if err != nil {
		return nil, fmt.Errorf("aes key of %d bytes: %w", len(k.secret), ivy.ErrFormat)
	}
	out := make([]byte, n)
	buf := make([]byte, segmentSize)
	for i, off := 0, 0; off < len(out); i, off = i+1, off+segmentSize {
		if i%256 == 255 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		// The last segment is padded to the size of a block.
		m := min(segmentSize, len(out)-off)
		m += (aes.BlockSize - m%aes.BlockSize) % aes.BlockSize
		if int64(off)+int64(m) > size-8 {
			return nil, fmt.Errorf("segment %d of the encrypted package is short: %w", i, ivy.ErrFormat)
		}
		if err := readAt(r, buf[:m], 8+int64(off)); err != nil {
			return nil, fmt.Errorf("reading segment %d of the encrypted package: %w: %w", i, ivy.ErrFormat, err)
		}
		if k.scheme == Agile {
			cipher.NewCBCDecrypter(b, k.agile.iv(le32(uint32(i)))).CryptBlocks(buf[:m], buf[:m])
		} else {
			for j := 0; j < m; j += aes.BlockSize {
				b.Decrypt(buf[j:], buf[j:])
			}
		}
		copy(out[off:], buf[:m])
	}
	return out, nil
}

// CheckIntegrity makes sure that the HMAC of the EncryptedPackage stream
// matches the HMAC that the file holds, in the element dataIntegrity of
// Agile encryption (MS-OFFCRYPTO 2.3.4.14). r holds the stream in its first
// size bytes.
//
// The check is optional, because the password check of Key already makes
// sure that the password is right. It returns an error that wraps
// ivy.ErrFormat if the HMAC differs, and one that wraps ivy.ErrUnsupported
// if the file holds no HMAC.
func (k *Key) CheckIntegrity(ctx context.Context, r io.ReaderAt, size int64) error {
	if k.scheme != Agile || len(k.agile.encryptedHmacKey) == 0 {
		return fmt.Errorf("integrity of %v encryption: %w", k.scheme, ivy.ErrUnsupported)
	}
	ai := k.agile
	hmacKey, err := cbcDecrypt(k.secret, ai.iv(blockHmacKey), ai.encryptedHmacKey)
	if err != nil {
		return err
	}
	value, err := cbcDecrypt(k.secret, ai.iv(blockHmacValue), ai.encryptedHmacValue)
	if err != nil {
		return err
	}
	if len(hmacKey) < ai.data.hashSize {
		return fmt.Errorf("hmac key of %d bytes: %w", len(hmacKey), ivy.ErrFormat)
	}
	mac := hmac.New(ai.data.hash, hmacKey[:ai.data.hashSize])
	buf := make([]byte, 64<<10)
	for off := int64(0); off < size; off += int64(len(buf)) {
		if err := ctx.Err(); err != nil {
			return err
		}
		m := int(min(int64(len(buf)), size-off))
		if err := readAt(r, buf[:m], off); err != nil {
			return fmt.Errorf("reading encrypted package: %w: %w", ivy.ErrFormat, err)
		}
		mac.Write(buf[:m])
	}
	if !equal(mac.Sum(nil), value, ai.data.hashSize) {
		return fmt.Errorf("hmac of the encrypted package: %w", ivy.ErrFormat)
	}
	return nil
}

// readAt reads len(buf) bytes of r at off. A reader can return io.EOF with
// the last bytes, and readAt accepts it.
func readAt(r io.ReaderAt, buf []byte, off int64) error {
	n, err := r.ReadAt(buf, off)
	switch {
	case n == len(buf):
		return nil
	case err == nil:
		err = io.ErrUnexpectedEOF
	}
	return fmt.Errorf("reading %d bytes at %d: %w", len(buf), off, err)
}
