package offcrypto

import (
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha1"
	"fmt"

	"github.com/xo/ivy"
)

// rc4Info holds the encryption header of RC4 encryption, version 1.1
// (MS-OFFCRYPTO 2.3.6.1).
type rc4Info struct {
	salt                  []byte
	encryptedVerifier     []byte
	encryptedVerifierHash []byte
}

// parseRC4 parses the part of the header after the version: the salt, the
// encrypted verifier and its encrypted hash, of 16 bytes each.
func parseRC4(buf []byte) (*rc4Info, error) {
	if len(buf) < 48 {
		return nil, fmt.Errorf("rc4 encryption header of %d bytes: %w", len(buf), ivy.ErrFormat)
	}
	return &rc4Info{
		salt:                  buf[:16],
		encryptedVerifier:     buf[16:32],
		encryptedVerifierHash: buf[32:48],
	}, nil
}

// key derives the key of the password and makes sure that it decrypts the
// verifier (MS-OFFCRYPTO 2.3.6.2 and 2.3.6.4).
func (ri *rc4Info) key(password string) (*Key, error) {
	h0 := md5.Sum(passwordBytes(password))
	buf := make([]byte, 0, 16*(5+len(ri.salt)))
	for range 16 {
		buf = append(buf, h0[:5]...)
		buf = append(buf, ri.salt...)
	}
	h1 := md5.Sum(buf)
	k := &Key{scheme: RC4, secret: h1[:5], keyLen: md5.Size}
	c, err := k.Stream(0)
	if err != nil {
		return nil, err
	}
	verifier := make([]byte, len(ri.encryptedVerifier))
	c.XORKeyStream(verifier, ri.encryptedVerifier)
	hash := make([]byte, len(ri.encryptedVerifierHash))
	c.XORKeyStream(hash, ri.encryptedVerifierHash)
	sum := md5.Sum(verifier)
	if !equal(sum[:], hash, md5.Size) {
		return nil, fmt.Errorf("rc4 encryption: %w", ivy.ErrPassword)
	}
	return k, nil
}

// Stream returns the RC4 cipher of the block number block, from the start
// of the block. Only the keys of RC4 and RC4 CryptoAPI have one.
//
// A binary format sets its own blocks. Word and Excel start a new block
// every 512 or 1024 bytes of a stream, and DecryptAt follows that rule.
// PowerPoint starts a new block for each persist object, with its number.
func (k *Key) Stream(block uint32) (*rc4.Cipher, error) {
	switch k.scheme {
	case RC4:
		h := md5.New()
		h.Write(k.secret)
		h.Write(le32(block))
		return rc4Cipher(h.Sum(nil)[:k.keyLen])
	case RC4CryptoAPI:
		h := sha1.New()
		h.Write(k.secret)
		h.Write(le32(block))
		key := h.Sum(nil)[:k.keyLen]
		if k.keyLen == 5 {
			// A key of 40 bits takes 11 bytes of zeros (MS-OFFCRYPTO
			// 2.3.5.2).
			key = append(key, make([]byte, 11)...)
		}
		return rc4Cipher(key)
	}
	return nil, fmt.Errorf("rc4 stream of %v encryption: %w", k.scheme, ivy.ErrUnsupported)
}

// DecryptAt decrypts buf in place. buf holds the bytes of a stream from
// the position offset, and the stream starts a new block every blockSize
// bytes, with the block numbers 0, 1, 2 and on. Word uses blocks of 512
// bytes, and Excel blocks of 1024 bytes.
//
// A format that leaves some bytes in the clear, such as the headers of the
// records of Excel, calls DecryptAt once for each part that it encrypts,
// with the position of the part in the stream.
func (k *Key) DecryptAt(buf []byte, offset int64, blockSize int) error {
	if blockSize <= 0 || offset < 0 {
		return fmt.Errorf("rc4 block of %d bytes at %d: %w", blockSize, offset, ivy.ErrFormat)
	}
	var skip [256]byte
	for len(buf) > 0 {
		block := offset / int64(blockSize)
		if block > 1<<32-1 {
			return fmt.Errorf("rc4 block %d: %w", block, ivy.ErrLimit)
		}
		c, err := k.Stream(uint32(block))
		if err != nil {
			return err
		}
		// Skip the bytes of the block before offset.
		for n := int(offset % int64(blockSize)); n > 0; {
			m := min(n, len(skip))
			c.XORKeyStream(skip[:m], skip[:m])
			n -= m
		}
		n := min(len(buf), blockSize-int(offset%int64(blockSize)))
		c.XORKeyStream(buf[:n], buf[:n])
		buf = buf[n:]
		offset += int64(n)
	}
	return nil
}
