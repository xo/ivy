package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu"
)

// The tolerances of TestDJVU, from 0 to 255. On 2026-11-17 the largest mean
// difference is 1.55 and the largest blurred one is 0.74, on the pages with
// IW44 foreground colors (see the log). ddjvu reduces a mask with a filter of
// its own, so the edges of the glyphs differ by a few levels.
const (
	djvuMeanDiff    = 4
	djvuBlurredDiff = 2
)

// TestDJVU compares the pages of the test files of djvu with the pages that
// ddjvu of DjVuLibre renders, which is the oracle of iv for DjVu. It checks
// the number of pages against djvused, and renders each page at 50 and 100
// dots per inch. It also renders a page that the test turns, and one with
// another gamma, by a change of the INFO chunk.
//
// IVY_DJVU_DIR keeps the images, for a person who looks at them.
func TestDJVU(t *testing.T) {
	dir := os.Getenv("IVY_DJVU_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	names, err := filepath.Glob(filepath.Join("..", "djvu", "testdata", "*.djvu"))
	if err != nil || len(names) == 0 {
		t.Fatalf("expected the test files of djvu, got: %v", err)
	}
	type file struct {
		name string
		data []byte
	}
	files := make([]file, 0, len(names)+4)
	for _, path := range names {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file{filepath.Base(path), data})
	}
	compound, err := os.ReadFile(filepath.Join("..", "djvu", "testdata", "compound.djvu"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rot := range []struct {
		name string
		flag byte
	}{{"90", 5}, {"180", 2}, {"270", 6}} {
		files = append(files, file{"compound-rot" + rot.name + ".djvu", patchInfo(t, compound, 9, rot.flag)})
	}
	files = append(files, file{"compound-gamma14.djvu", patchInfo(t, compound, 8, 14)})

	for _, f := range files {
		t.Run(f.name, func(t *testing.T) {
			path := filepath.Join(dir, f.name)
			if err := os.WriteFile(path, f.data, 0o600); err != nil {
				t.Fatal(err)
			}
			d, err := djvu.Open(t.Context(), bytes.NewReader(f.data))
			if err != nil {
				t.Fatal(err)
			}
			n, _ := d.PageCount(t.Context())
			want, err := djvusedPages(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if n != want {
				t.Fatalf("expected %d pages, got: %d", want, n)
			}
			for i := range n {
				page, err := d.Page(t.Context(), i)
				if err != nil {
					t.Fatal(err)
				}
				for _, dpi := range []int{50, 100} {
					got, err := ivy.RenderImage(t.Context(), page, float32(dpi), color.White)
					if err != nil {
						t.Fatalf("page %d at %d dpi: %v", i+1, dpi, err)
					}
					want, err := ddjvuPage(t.Context(), path, i+1, dpi)
					if err != nil {
						t.Fatal(err)
					}
					stem := strings.TrimSuffix(path, ".djvu") + fmt.Sprintf("-%d-%d", i+1, dpi)
					writePNG(t, stem+"-ivy.png", got)
					writePNG(t, stem+"-ddjvu.png", want)
					got, want, ok := sameSize(got, want)
					if !ok {
						t.Fatalf("page %d at %d dpi: expected a size of %v, got: %v", i+1, dpi, want.Bounds().Size(), got.Bounds().Size())
					}
					diff := meanDiff(got, want)
					blurred := meanDiff(blur(got, 2), blur(want, 2))
					t.Logf("page %d at %d dpi: mean difference %.2f, after a blur of 5 by 5 pixels %.2f", i+1, dpi, diff, blurred)
					if diff > djvuMeanDiff || blurred > djvuBlurredDiff {
						t.Errorf("page %d at %d dpi: expected the page of ddjvu within %d, and %d blurred, got: %.2f and %.2f",
							i+1, dpi, djvuMeanDiff, djvuBlurredDiff, diff, blurred)
					}
				}
			}
		})
	}
}

// TestDJVUIndirect compares the pages of an indirect document with the
// pages that ddjvu renders from it. djvmcvt of DjVuLibre converts the
// bundled test files of djvu into indirect documents, and djvu reads their
// files from the folder of WithDir (D38).
func TestDJVUIndirect(t *testing.T) {
	bin, err := exec.LookPath("djvmcvt")
	if err != nil {
		t.Fatalf("oracle djvmcvt is not installed: %v", err)
	}
	for _, name := range []string{"bundled.djvu", "shared-dict.djvu", "thumbnails.djvu"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			src, err := filepath.Abs(filepath.Join("..", "djvu", "testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), bin, "-i", src, dir, "index.djvu")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("running djvmcvt: %v: %s", err, out)
			}
			path := filepath.Join(dir, "index.djvu")
			index, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			d, err := djvu.Open(t.Context(), bytes.NewReader(index), djvu.WithDir(os.DirFS(dir)))
			if err != nil {
				t.Fatal(err)
			}
			n, _ := d.PageCount(t.Context())
			want, err := djvusedPages(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if n != want {
				t.Fatalf("expected %d pages, got: %d", want, n)
			}
			for i := range n {
				page, err := d.Page(t.Context(), i)
				if err != nil {
					t.Fatal(err)
				}
				got, err := ivy.RenderImage(t.Context(), page, 50, color.White)
				if err != nil {
					t.Fatalf("page %d: %v", i+1, err)
				}
				want, err := ddjvuPage(t.Context(), path, i+1, 50)
				if err != nil {
					t.Fatal(err)
				}
				got, want, ok := sameSize(got, want)
				if !ok {
					t.Fatalf("page %d: expected a size of %v, got: %v", i+1, want.Bounds().Size(), got.Bounds().Size())
				}
				diff := meanDiff(got, want)
				blurred := meanDiff(blur(got, 2), blur(want, 2))
				t.Logf("page %d: mean difference %.2f, after a blur of 5 by 5 pixels %.2f", i+1, diff, blurred)
				if diff > djvuMeanDiff || blurred > djvuBlurredDiff {
					t.Errorf("page %d: expected the page of ddjvu within %d, and %d blurred, got: %.2f and %.2f",
						i+1, djvuMeanDiff, djvuBlurredDiff, diff, blurred)
				}
			}
		})
	}
}

// patchInfo returns a copy of the single page DjVu file data, with the byte
// at off in its INFO chunk set to v.
func patchInfo(t *testing.T, data []byte, off int, v byte) []byte {
	t.Helper()
	i := bytes.Index(data, []byte("INFO"))
	if i < 0 || i+8+off >= len(data) {
		t.Fatal("expected an INFO chunk")
	}
	out := bytes.Clone(data)
	out[i+8+off] = v
	return out
}

// sameSize crops a and b to the size that they share, if they differ by a
// pixel at most, as the rounding of the size of a page can.
func sameSize(a, b image.Image) (image.Image, image.Image, bool) {
	sa, sb := a.Bounds().Size(), b.Bounds().Size()
	if abs(sa.X-sb.X) > 1 || abs(sa.Y-sb.Y) > 1 {
		return a, b, false
	}
	r := image.Rect(0, 0, min(sa.X, sb.X), min(sa.Y, sb.Y))
	return crop(a, r), crop(b, r), true
}

// crop returns the part r of img, from its corner.
func crop(img image.Image, r image.Rectangle) image.Image {
	out := image.NewRGBA(r)
	b := img.Bounds()
	for y := range r.Dy() {
		for x := range r.Dx() {
			out.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// djvusedPages returns the number of pages of the DjVu file at path, as
// djvused counts them.
func djvusedPages(ctx context.Context, path string) (int, error) {
	bin, err := exec.LookPath("djvused")
	if err != nil {
		return 0, fmt.Errorf("oracle djvused is not installed: %w", err)
	}
	out, err := exec.CommandContext(ctx, bin, "-e", "n", path).Output()
	if err != nil {
		return 0, fmt.Errorf("running djvused: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("reading the pages of djvused: %w", err)
	}
	return n, nil
}

// ddjvuPage renders page n of the DjVu file at path at dpi dots per inch
// with ddjvu.
func ddjvuPage(ctx context.Context, path string, n, dpi int) (image.Image, error) {
	bin, err := exec.LookPath("ddjvu")
	if err != nil {
		return nil, fmt.Errorf("oracle ddjvu is not installed: %w", err)
	}
	cmd := exec.CommandContext(ctx, bin, "-format=ppm", "-page="+strconv.Itoa(n), "-scale="+strconv.Itoa(dpi), path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running ddjvu: %w: %s", err, stderr.String())
	}
	return decodePPM(bufio.NewReader(&stdout))
}

// decodePPM decodes a binary PPM image with 8 bits for each sample, as
// ddjvu writes it.
func decodePPM(r *bufio.Reader) (image.Image, error) {
	var magic string
	var w, h, maxval int
	if _, err := fmt.Fscan(r, &magic, &w, &h, &maxval); err != nil {
		return nil, fmt.Errorf("reading the header of the ppm of ddjvu: %w", err)
	}
	if magic != "P6" || maxval != 255 || w <= 0 || h <= 0 || w*h > 1<<26 {
		return nil, fmt.Errorf("unexpected ppm of ddjvu: %s %dx%d %d", magic, w, h, maxval)
	}
	if _, err := r.ReadByte(); err != nil {
		return nil, fmt.Errorf("reading the ppm of ddjvu: %w", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	row := make([]byte, 3*w)
	for y := range h {
		if _, err := io.ReadFull(r, row); err != nil {
			return nil, fmt.Errorf("reading the pixels of the ppm of ddjvu: %w", err)
		}
		for x := range w {
			copy(img.Pix[y*img.Stride+4*x:], row[3*x:3*x+3])
			img.Pix[y*img.Stride+4*x+3] = 0xff
		}
	}
	return img, nil
}

// TestDJVUIW44 compares a standalone IW44 image, FORM:PM44 in color and
// FORM:BM44 in gray, with the image that ddjvu renders at its size in
// pixels. The test makes each image from the BG44 chunks of a test file of
// djvu.
func TestDJVUIW44(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct{ name, typ string }{{"photo.djvu", "PM44"}, {"gray.djvu", "BM44"}} {
		t.Run(c.typ, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "djvu", "testdata", c.name))
			if err != nil {
				t.Fatal(err)
			}
			iw := iw44File(t, data, c.typ)
			path := filepath.Join(dir, c.typ+".djvu")
			if err := os.WriteFile(path, iw, 0o600); err != nil {
				t.Fatal(err)
			}
			d, err := djvu.Open(t.Context(), bytes.NewReader(iw))
			if err != nil {
				t.Fatal(err)
			}
			page, err := d.Page(t.Context(), 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ivy.RenderImage(t.Context(), page, 72, color.White)
			if err != nil {
				t.Fatal(err)
			}
			size := got.Bounds().Size()
			bin, err := exec.LookPath("ddjvu")
			if err != nil {
				t.Fatalf("oracle ddjvu is not installed: %v", err)
			}
			cmd := exec.CommandContext(t.Context(), bin, "-format=ppm", fmt.Sprintf("-size=%dx%d", size.X, size.Y), path)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("running ddjvu: %v: %s", err, stderr.String())
			}
			want, err := decodePPM(bufio.NewReader(&stdout))
			if err != nil {
				t.Fatal(err)
			}
			if want.Bounds().Size() != size {
				t.Fatalf("expected %v, got: %v", want.Bounds().Size(), size)
			}
			diff := meanDiff(got, want)
			t.Logf("mean difference: %.2f", diff)
			if diff < 0 || diff > djvuMeanDiff {
				t.Errorf("expected the image of ddjvu within %d, got: %.2f", djvuMeanDiff, diff)
			}
		})
	}
}

// iw44File returns a standalone IW44 file of the type typ with the BG44
// chunks of the single page DjVu file data.
func iw44File(t *testing.T, data []byte, typ string) []byte {
	t.Helper()
	body := []byte(typ)
	// the chunks of FORM:DJVU follow AT&T, its header and its type
	for off := 16; off+8 <= len(data); {
		n := int(binary.BigEndian.Uint32(data[off+4:]))
		if off+8+n > len(data) {
			break
		}
		if string(data[off:off+4]) == "BG44" {
			body = append(body, typ...)
			body = binary.BigEndian.AppendUint32(body, uint32(n))
			body = append(body, data[off+8:off+8+n]...)
			if n%2 == 1 {
				body = append(body, 0)
			}
		}
		off += 8 + n + n%2
	}
	if len(body) == len(typ) {
		t.Fatal("expected BG44 chunks")
	}
	out := []byte("AT&TFORM")
	out = binary.BigEndian.AppendUint32(out, uint32(len(body)))
	return append(out, body...)
}
