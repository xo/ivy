package test

import (
	"bytes"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/image/draw"

	"github.com/xo/ivy"
	"github.com/xo/ivy/odf"
	"github.com/xo/ivy/ps"
)

// TestEPSPreview compares the preview of an EPS file that ivy decodes with
// the page that Ghostscript renders from the PostScript of the same file
// (D13). The preview is a TIFF image or the hex lines of EPSI, which
// another program made from the same picture, so the two images must
// match. Ghostscript runs as an outside program, and no code of it is read.
//
// On 2026-11-17 the mean differences are 0.00 for tiff8-preview.eps and
// 0.00 for half.epsi.
func TestEPSPreview(t *testing.T) {
	for _, tt := range []struct {
		name string
		max  float64
	}{
		{"tiff8-preview.eps", 3},
		{"half.epsi", 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("..", "ps", "testdata", tt.name)
			buf, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := ps.Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			got, err := ivy.BestPreview(t.Context(), d, image.Point{})
			if err != nil {
				t.Fatal(err)
			}
			want, err := run(t.Context(), "gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-r72",
				"-dEPSCrop", "-sOutputFile=-", path)
			if err != nil {
				t.Fatal(err)
			}
			diff := meanDiff(got, want)
			t.Logf("mean difference: %.2f", diff)
			if diff < 0 || diff > tt.max {
				stem := filepath.Join(t.TempDir(), tt.name)
				writePNG(t, stem+"-ivy.png", got)
				writePNG(t, stem+"-gs.png", want)
				t.Errorf("expected the page of Ghostscript within a mean difference of %g, got: %.2f (ivy %v, gs %v)",
					tt.max, diff, got.Bounds().Size(), want.Bounds().Size())
			}
		})
	}
}

// TestODFPreview makes LibreOffice save an OpenDocument text, which then
// holds the thumbnail of its first page, and compares the preview that ivy
// reads from the file with the first page of the PDF file of LibreOffice,
// scaled to the size of the thumbnail (D13). The test compares the two
// images at a quarter of the size of the thumbnail. There, a white image
// differs from the page by about 12.
//
// On 2026-11-17 the mean difference is 3.16.
func TestODFPreview(t *testing.T) {
	const maxDiff = 6
	dir := t.TempDir()
	src := filepath.Join(dir, "in", "document.odt")
	if err := os.MkdirAll(filepath.Dir(src), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, buildODT(t), 0o600); err != nil {
		t.Fatal(err)
	}
	exe, err := exec.LookPath("soffice")
	if err != nil {
		t.Fatal("oracle soffice is not installed:", err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "--headless", "--norestore",
		"-env:UserInstallation=file://"+filepath.Join(dir, "profile"),
		"--convert-to", "odt", "--outdir", dir, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running soffice: %v: %s", err, out)
	}
	path := filepath.Join(dir, "document.odt")
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("soffice wrote no odt:", err)
	}
	d, err := odf.Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	got, err := ivy.BestPreview(t.Context(), d, image.Point{})
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := sofficePDF(t.Context(), dir, path)
	if err != nil {
		t.Fatal(err)
	}
	page, err := run(t.Context(), "pdftoppm", "-png", "-r", "96", "-f", "1", "-l", "1", "-singlefile", pdf)
	if err != nil {
		t.Fatal(err)
	}
	want := image.NewRGBA(got.Bounds())
	draw.CatmullRom.Scale(want, want.Bounds(), page, page.Bounds(), draw.Src, nil)
	// the two images draw the glyphs at other fractions of a pixel, and a
	// quarter of the size compares the lines of text instead of the edges
	// of the glyphs
	diff := meanDiff(quarter(got), quarter(want))
	t.Logf("preview %v, page %v, mean difference: %.2f", got.Bounds().Size(), page.Bounds().Size(), diff)
	if out := os.Getenv("IVY_ORACLE_OUT"); out != "" {
		writePNG(t, filepath.Join(out, "odt-preview-ivy.png"), got)
		writePNG(t, filepath.Join(out, "odt-preview-soffice.png"), want)
	}
	if diff < 0 || diff > maxDiff {
		t.Errorf("expected the first page of soffice within a mean difference of %d, got: %.2f", maxDiff, diff)
	}
}

// quarter returns img scaled to a quarter of its width and its height.
func quarter(img image.Image) image.Image {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, max(b.Dx()/4, 1), max(b.Dy()/4, 1)))
	draw.CatmullRom.Scale(out, out.Bounds(), img, b, draw.Src, nil)
	return out
}
