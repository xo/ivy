package test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/comic"
)

// TestComicEncrypted opens comic book archives that zip of Info-ZIP encrypts
// with ZipCrypto, and that 7-Zip encrypts with the AES of WinZip, with keys
// of 128 and 256 bits (D12).
func TestComicEncrypted(t *testing.T) {
	dir := t.TempDir()
	names := []string{"page1.png", "page2.png"}
	for i, name := range names {
		img := image.NewNRGBA(image.Rect(0, 0, 10*(i+1), 10))
		for p := range img.Pix {
			img.Pix[p] = 0xff
		}
		img.Set(0, 0, color.NRGBA{R: 255, A: 255})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name string
		args []string
	}{
		{"zipcrypto.cbz", []string{"zip", "-q", "-P", "secret", "zipcrypto.cbz"}},
		{"aes128.cbz", []string{"7z", "a", "-tzip", "-psecret", "-mem=AES128", "aes128.cbz"}},
		{"aes256.cbz", []string{"7z", "a", "-tzip", "-psecret", "-mem=AES256", "aes256.cbz"}},
		{"aes256-store.cbz", []string{"7z", "a", "-tzip", "-psecret", "-mem=AES256", "-mx=0", "aes256-store.cbz"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, err := exec.LookPath(test.args[0])
			if err != nil {
				t.Fatalf("%s is not installed: %v", test.args[0], err)
			}
			cmd := exec.CommandContext(t.Context(), path, append(test.args[1:], names...)...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("running %s: %v: %s", test.args[0], err, out)
			}
			buf, err := os.ReadFile(filepath.Join(dir, test.name))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := comic.Open(t.Context(), bytes.NewReader(buf)); err == nil {
				t.Error("expected an error without the password")
			}
			d, err := comic.Open(t.Context(), bytes.NewReader(buf), comic.WithConfig(ivy.Config{Password: "secret"}))
			if err != nil {
				t.Fatal(err)
			}
			for i := range names {
				page, err := d.Page(t.Context(), i)
				if err != nil {
					t.Fatal(err)
				}
				img, err := ivy.RenderImage(t.Context(), page, 72, nil)
				if err != nil {
					t.Fatal(err)
				}
				if img.Bounds().Dx() != 10*(i+1) {
					t.Errorf("page %d: expected a width of %d, got: %d", i, 10*(i+1), img.Bounds().Dx())
				}
				if got := color.RGBAModel.Convert(img.At(0, 0)); got != (color.RGBA{255, 0, 0, 255}) {
					t.Errorf("page %d: expected red, got: %v", i, got)
				}
			}
		})
	}
}
