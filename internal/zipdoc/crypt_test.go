package zipdoc

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

func TestEncrypted(t *testing.T) {
	png := fixture.PNG(8, 4, color.White)
	ciphers := map[string]fixture.ZipCipher{
		"zipcrypto": fixture.ZipCrypto,
		"aes-1":     fixture.WinZipAES1,
		"aes-2":     fixture.WinZipAES2,
	}
	for name, c := range ciphers {
		t.Run(name, func(t *testing.T) {
			entries := []fixture.Entry{
				{Name: "a.png", Data: png},
				{Name: "b.txt", Data: bytes.Repeat([]byte("ivy "), 1000)},
				{Name: "c.txt", Data: []byte("stored"), Store: true},
			}
			buf := fixture.EncryptedZip(c, "secret", entries...)
			a, err := Open(bytes.NewReader(buf), ivy.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			f := a.Find("c.txt")
			if !Encrypted(f) {
				t.Fatal("expected an encrypted entry")
			}
			// a format that does not unlock the archive does not read it
			if _, err := a.Read(f); !errors.Is(err, ivy.ErrUnsupported) {
				t.Errorf("expected ErrUnsupported, got: %v", err)
			}
			if err := a.Unlock(t.Context(), f, ivy.Config{Password: "wrong"}); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ErrPassword, got: %v", err)
			}
			if _, err := a.Read(f); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ErrPassword, got: %v", err)
			}
			var asked []int
			cfg := ivy.Config{Password: "wrong", PasswordFunc: func(_ context.Context, attempt int) (string, error) {
				asked = append(asked, attempt)
				if attempt < 2 {
					return "still wrong", nil
				}
				return "secret", nil
			}}
			if err := a.Unlock(t.Context(), f, cfg); err != nil {
				t.Fatal(err)
			}
			if len(asked) != 2 {
				t.Errorf("expected 2 attempts, got: %v", asked)
			}
			for _, e := range entries {
				got, err := a.Read(a.Find(e.Name))
				if err != nil {
					t.Fatalf("%s: %v", e.Name, err)
				}
				if !bytes.Equal(got, e.Data) {
					t.Errorf("%s: expected the data of the entry", e.Name)
				}
			}
			if size, err := a.Config(a.Find("a.png")); err != nil || size.X != 8 || size.Y != 4 {
				t.Errorf("expected 8x4, got: %v %v", size, err)
			}
		})
	}
}

func TestEncryptedErrors(t *testing.T) {
	buf := fixture.EncryptedZip(fixture.WinZipAES2, "secret", fixture.Entry{Name: "a.txt", Data: []byte("data of the entry"), Store: true})
	// a change of the encrypted data fails the authentication code, and the
	// password is right
	i := bytes.Index(buf, []byte("a.txt")) + len("a.txt") + 11 + 16 + 2
	buf[i] ^= 0xff
	a, err := Open(bytes.NewReader(buf), ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Unlock(t.Context(), a.Find("a.txt"), ivy.Config{Password: "secret"}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat, got: %v", err)
	}
	// the function that asks for a password can stop
	stop := errors.New("stop")
	cfg := ivy.Config{PasswordFunc: func(context.Context, int) (string, error) { return "", stop }}
	if err := a.Unlock(t.Context(), a.Find("a.txt"), cfg); !errors.Is(err, stop) || !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ErrPassword and the error of the function, got: %v", err)
	}
	if _, err := readAESExtra([]byte{0x01, 0x99, 7, 0, 3, 0, 'A', 'E', 3, 8, 0}); !errors.Is(err, ivy.ErrUnsupported) {
		t.Errorf("expected ErrUnsupported for version 3, got: %v", err)
	}
	if _, err := readAESExtra(nil); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ErrFormat without the extra field, got: %v", err)
	}
}
