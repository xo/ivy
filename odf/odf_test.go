package odf

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

func TestOpenLibreOffice(t *testing.T) {
	for _, name := range []string{"sample.odt", "sheet.ods", "sample.odp"} {
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := d.Close(); err != nil {
					t.Error(err)
				}
			})
			if d.Mime() == "" {
				t.Error("expected a mime type")
			}
			img, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil {
				t.Fatal(err)
			}
			if size := img.Bounds().Size(); size.X == 0 || size.X > 512 || size.Y > 512 {
				t.Errorf("expected a thumbnail of at most 512 pixels, got: %v", size)
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
	}{
		{"not a zip", []byte("plain text")},
		{"wrong mime", fixture.Zip(fixture.Entry{Name: "mimetype", Data: []byte("image/openraster")})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Open(t.Context(), bytes.NewReader(test.buf)); !errors.Is(err, ivy.ErrFormat) {
				t.Errorf("expected ErrFormat, got: %v", err)
			}
		})
	}
}

func TestNoThumbnail(t *testing.T) {
	buf := fixture.Zip(fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "text")})
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
		t.Errorf("expected ErrNoPreview, got: %v", err)
	}
}

func TestFormat(t *testing.T) {
	buf := fixture.Zip(
		fixture.Entry{Name: "mimetype", Data: []byte(mimePrefix + "graphics")},
		fixture.Entry{Name: "Thumbnails/thumbnail.png", Data: fixture.PNG(10, 20, color.White)},
	)
	d, err := Format.Open(t.Context(), bytes.NewReader(buf), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.BestPreview(t.Context(), d, image.Pt(10, 10))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Size() != image.Pt(10, 20) {
		t.Errorf("expected 10x20, got: %v", img.Bounds().Size())
	}
}

func FuzzOpen(f *testing.F) {
	for _, name := range []string{"sample.odt", "sheet.ods", "sample.odp", "encrypted_blowfish.odt", "encrypted_aes.odt", "encrypted_argon2id.odt"} {
		buf, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf)
	}
	f.Add(encrypted(fixture.ODFAESCBC, false))
	// Without a password, the fuzz test reads the manifests but derives no
	// key. The fuzz test of odfcrypt covers the decryption.
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		for p, err := range d.Previews(t.Context()) {
			if err != nil {
				break
			}
			_, _ = p.Decode(t.Context())
		}
		// the first page of each kind of file
		if _, err := d.PageCount(t.Context()); err != nil {
			return
		}
		if p, err := d.Page(t.Context(), 0); err == nil {
			_, _ = ivy.RenderImage(t.Context(), p, 8, nil)
		}
	})
}

// encrypted returns an OpenDocument text that the fixture encrypts with c
// and the password "secret", with a thumbnail of 10 by 20 pixels.
func encrypted(c fixture.ODFCipher, whole bool) []byte {
	return fixture.EncryptODF(
		fixture.ODFEncryption{Password: "secret", Cipher: c, Iterations: 100, Whole: whole},
		mimePrefix+"text",
		fixture.Entry{Name: "content.xml", Data: []byte("<office:document-content/>")},
		fixture.Entry{Name: thumbnailName, Data: fixture.PNG(10, 20, color.White)},
	)
}

// TestEncryptedLibreOffice opens the files that LibreOffice encrypted with
// the password "secret". LibreOffice writes no thumbnail into an encrypted
// file.
func TestEncryptedLibreOffice(t *testing.T) {
	for _, name := range []string{"encrypted_blowfish.odt", "encrypted_aes.odt"} {
		t.Run(name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ivy.ErrPassword without a password, got: %v", err)
			}
			if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "wrong"})); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ivy.ErrPassword for a wrong password, got: %v", err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "secret"}))
			if err != nil {
				t.Fatal(err)
			}
			if d.Mime() != mimePrefix+"text" {
				t.Errorf("expected the mime type of a text, got: %q", d.Mime())
			}
			if _, err := ivy.BestPreview(t.Context(), d, image.Point{}); !errors.Is(err, ivy.ErrNoPreview) {
				t.Errorf("expected ErrNoPreview, got: %v", err)
			}
		})
	}
}

// TestEncryptedArgon2id opens the file that LibreOffice encrypts by
// default for ODF 1.4, with AES-GCM and a key from Argon2id (D25).
func TestEncryptedArgon2id(t *testing.T) {
	buf, err := os.ReadFile(filepath.Join("testdata", "encrypted_argon2id.odt"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "wrong"})); !errors.Is(err, ivy.ErrPassword) {
		t.Errorf("expected ivy.ErrPassword, got: %v", err)
	}
	attempts := 0
	cfg := ivy.Config{PasswordFunc: func(_ context.Context, attempt int) (string, error) {
		attempts = attempt
		return "secret", nil
	}}
	d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || d.Mime() != "application/vnd.oasis.opendocument.text" {
		t.Errorf("expected a text after one attempt, got: %d, %q", attempts, d.Mime())
	}
	if n, err := d.PageCount(t.Context()); err != nil || n != 1 {
		t.Errorf("expected 1 page, got: %d, %v", n, err)
	}
}

// TestEncryptedThumbnail decrypts the thumbnail of a file that the fixture
// encrypts with each cipher, and with the whole package encrypted.
func TestEncryptedThumbnail(t *testing.T) {
	tests := []struct {
		name   string
		cipher fixture.ODFCipher
		whole  bool
	}{
		{"blowfish", fixture.ODFBlowfish, false},
		{"aes cbc", fixture.ODFAESCBC, false},
		{"aes gcm", fixture.ODFAESGCM, false},
		{"whole package", fixture.ODFAESGCM, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := encrypted(test.cipher, test.whole)
			if _, err := Open(t.Context(), bytes.NewReader(buf)); !errors.Is(err, ivy.ErrPassword) {
				t.Errorf("expected ivy.ErrPassword without a password, got: %v", err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(ivy.Config{Password: "secret"}))
			if err != nil {
				t.Fatal(err)
			}
			if d.Mime() != mimePrefix+"text" {
				t.Errorf("expected the mime type of a text, got: %q", d.Mime())
			}
			for p, err := range d.Previews(t.Context()) {
				if err != nil {
					t.Fatal(err)
				}
				if p.Size != image.Pt(10, 20) {
					t.Errorf("expected the size 10x20 in the list, got: %v", p.Size)
				}
			}
			img, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Size() != image.Pt(10, 20) {
				t.Errorf("expected 10x20, got: %v", img.Bounds().Size())
			}
		})
	}
}

func TestPasswordFunc(t *testing.T) {
	buf := encrypted(fixture.ODFBlowfish, false)
	var attempts []int
	cfg := ivy.Config{Password: "first", PasswordFunc: func(_ context.Context, attempt int) (string, error) {
		attempts = append(attempts, attempt)
		if attempt < 3 {
			return "wrong", nil
		}
		return "secret", nil
	}}
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg)); err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 3 || attempts[0] != 1 || attempts[2] != 3 {
		t.Errorf("expected the attempts 1, 2 and 3, got: %v", attempts)
	}
	errStop := errors.New("stop")
	cfg.PasswordFunc = func(context.Context, int) (string, error) { return "", errStop }
	if _, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg)); !errors.Is(err, ivy.ErrPassword) || !errors.Is(err, errStop) {
		t.Errorf("expected ivy.ErrPassword and the error of the function, got: %v", err)
	}
}
