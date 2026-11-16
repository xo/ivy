package odfcrypt

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strconv"
	"sync"

	// ODF 1.1 encrypts with Blowfish, which the maintainers approved (D24).
	"golang.org/x/crypto/blowfish" //nolint:staticcheck // the format needs it

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/raster"
)

// checksumLen is the number of bytes that the checksum covers.
const checksumLen = 1024

// Keyring decrypts the entries of one file with one password. It keeps the
// key of each salt, because PBKDF2 takes time, and each entry can have a
// salt of its own. It is safe for concurrent use.
type Keyring struct {
	password string

	mu   sync.Mutex
	keys map[string][]byte
}

// NewKeyring returns a keyring of password.
func NewKeyring(password string) *Keyring {
	return &Keyring{password: password, keys: make(map[string][]byte)}
}

// Supported returns an error that wraps ivy.ErrUnsupported if the package
// cannot decrypt the entry, such as for the key derivation Argon2id.
func (enc *Encryption) Supported() error {
	if enc.Derivation == Argon2id {
		return fmt.Errorf("key derivation argon2id: %w", ivy.ErrUnsupported)
	}
	return nil
}

// Decrypt decrypts the bytes data of an entry, makes sure that the
// checksum matches, and inflates the result. It returns an error that
// wraps ivy.ErrPassword if the password is empty or wrong, and one that
// wraps ivy.ErrLimit if the entry is larger than lim.MaxMemory.
func (kr *Keyring) Decrypt(ctx context.Context, enc *Encryption, data []byte, lim ivy.Limits) ([]byte, error) {
	if err := enc.Supported(); err != nil {
		return nil, err
	}
	lim = lim.WithDefaults()
	key, err := kr.key(ctx, enc)
	if err != nil {
		return nil, err
	}
	plain, err := decrypt(enc, key, data)
	if err != nil {
		return nil, err
	}
	if enc.Checksum != NoChecksum {
		head := plain[:min(len(plain), checksumLen)]
		var sum []byte
		if enc.Checksum == SHA1K {
			s := sha1.Sum(head)
			sum = s[:]
		} else {
			s := sha256.Sum256(head)
			sum = s[:]
		}
		if subtle.ConstantTimeCompare(sum, enc.Sum) != 1 {
			return nil, fmt.Errorf("odf checksum: %w", ivy.ErrPassword)
		}
	}
	out, err := raster.ReadAll(flate.NewReader(bytes.NewReader(plain)), lim.MaxMemory)
	switch {
	case errors.Is(err, ivy.ErrLimit):
		return nil, err
	case err != nil && enc.Checksum == NoChecksum && enc.Algorithm != AESGCM:
		// Without a checksum or an authenticated cipher, a wrong password
		// shows up only here.
		return nil, fmt.Errorf("odf entry: %w", ivy.ErrPassword)
	case err != nil:
		return nil, fmt.Errorf("inflating odf entry: %w", err)
	}
	return out, nil
}

// key derives the key of enc from the password, or returns the key that the
// keyring derived before for the same parameters.
func (kr *Keyring) key(ctx context.Context, enc *Encryption) ([]byte, error) {
	if kr.password == "" {
		return nil, fmt.Errorf("odf key: %w", ivy.ErrPassword)
	}
	id := strconv.FormatBool(enc.StartSHA256) + "\x00" + strconv.Itoa(enc.Iterations) + "\x00" +
		strconv.Itoa(enc.KeySize) + "\x00" + string(enc.Salt)
	kr.mu.Lock()
	key, ok := kr.keys[id]
	kr.mu.Unlock()
	if ok {
		return key, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var start []byte
	if enc.StartSHA256 {
		s := sha256.Sum256([]byte(kr.password))
		start = s[:]
	} else {
		s := sha1.Sum([]byte(kr.password))
		start = s[:]
	}
	if enc.KeySize < 1 || enc.KeySize > 64 {
		return nil, fmt.Errorf("odf key of %d bytes: %w", enc.KeySize, ivy.ErrFormat)
	}
	key, err := pbkdf2.Key(sha1.New, string(start), enc.Salt, enc.Iterations, enc.KeySize)
	if err != nil {
		return nil, fmt.Errorf("odf key: %w: %w", ivy.ErrFormat, err)
	}
	kr.mu.Lock()
	kr.keys[id] = key
	kr.mu.Unlock()
	return key, nil
}

// decrypt decrypts data with the cipher of enc. For AES in the mode CBC, it
// removes the padding of XML Encryption, whose last byte is the length of
// the padding.
func decrypt(enc *Encryption, key, data []byte) ([]byte, error) {
	switch enc.Algorithm {
	case Blowfish:
		b, err := blowfish.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("blowfish key of %d bytes: %w", len(key), ivy.ErrFormat)
		}
		if len(enc.IV) != blowfish.BlockSize {
			return nil, fmt.Errorf("blowfish iv of %d bytes: %w", len(enc.IV), ivy.ErrFormat)
		}
		return cfbDecrypt(b, enc.IV, data), nil
	case AESCBC:
		b, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("aes key of %d bytes: %w", len(key), ivy.ErrFormat)
		}
		if len(enc.IV) != aes.BlockSize {
			return nil, fmt.Errorf("aes iv of %d bytes: %w", len(enc.IV), ivy.ErrFormat)
		}
		if len(data) == 0 || len(data)%aes.BlockSize != 0 {
			return nil, fmt.Errorf("aes data of %d bytes: %w", len(data), ivy.ErrFormat)
		}
		plain := make([]byte, len(data))
		cipher.NewCBCDecrypter(b, enc.IV).CryptBlocks(plain, data)
		pad := int(plain[len(plain)-1])
		if pad < 1 || pad > aes.BlockSize {
			return nil, fmt.Errorf("aes padding: %w", ivy.ErrPassword)
		}
		return plain[:len(plain)-pad], nil
	case AESGCM:
		b, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("aes key of %d bytes: %w", len(key), ivy.ErrFormat)
		}
		if len(enc.IV) == 0 {
			return nil, fmt.Errorf("aes gcm without an iv: %w", ivy.ErrFormat)
		}
		gcm, err := cipher.NewGCMWithNonceSize(b, len(enc.IV))
		if err != nil {
			return nil, fmt.Errorf("aes gcm iv of %d bytes: %w", len(enc.IV), ivy.ErrFormat)
		}
		plain, err := gcm.Open(nil, enc.IV, data, nil)
		if err != nil {
			return nil, fmt.Errorf("aes gcm: %w", ivy.ErrPassword)
		}
		return plain, nil
	}
	return nil, fmt.Errorf("cipher %d: %w", enc.Algorithm, ivy.ErrUnsupported)
}

// cfbDecrypt decrypts data with b in the mode CFB, with a feedback of one
// whole block, as OpenOffice uses Blowfish.
func cfbDecrypt(b cipher.Block, iv, data []byte) []byte {
	n := b.BlockSize()
	reg := bytes.Clone(iv)
	stream := make([]byte, n)
	plain := make([]byte, len(data))
	for i := 0; i < len(data); i += n {
		b.Encrypt(stream, reg)
		m := min(n, len(data)-i)
		for j := range m {
			plain[i+j] = data[i+j] ^ stream[j]
		}
		copy(reg, data[i:i+m])
	}
	return plain
}
