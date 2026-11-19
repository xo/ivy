package test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/pdf"
	"github.com/xo/ivy/pdf/pdfenc"
	"github.com/xo/ivy/test/review"
)

// viewerDiff is the threshold of D58: the mean difference of a page of the
// written file through a viewer from the source page.
const viewerDiff = 8

// TestPDFEnc writes the first 3 pages and the last page of each document of
// the corpus of the review as a PDF file (D58). It runs qpdf --check on the
// file. It renders each page of the file with the pdf package of ivy, with
// mutool draw of MuPDF and with pdftoppm of Poppler at 72 dots for each
// inch, and compares each one with ivy.RenderImage of the source page. Each
// must match within a mean difference of 8. For a source that is a PDF file,
// a page of a viewer also passes when the viewer draws the source file
// within 8 of the written file.
//
// The files of the corpus are in the cache of the review, which
// test/cmd/review fetches. IVY_REVIEW_CACHE names the folder of the cache,
// by default review/cache. Without the cache, the test is skipped. When
// IVY_PDFENC_PNG names a folder, the test writes the pages that it compares
// there, and the written files.
//
// okage D68 lists faults of the viewers: Poppler draws a knockout group as
// a group without the flag K, and MuPDF ignores the stroke adjustment. A
// page of a file that holds a knockout group or the stroke adjustment logs
// its difference through that viewer and does not fail. pdfencDiffers
// lists the other documents that do not fail.
func TestPDFEnc(t *testing.T) {
	cache := os.Getenv("IVY_REVIEW_CACHE")
	if cache == "" {
		cache = filepath.Join("review", "cache")
	}
	if _, err := os.Stat(filepath.Join(cache, "files")); err != nil {
		t.Skipf("no cache of the corpus in %s: fetch it with test/cmd/review, or set IVY_REVIEW_CACHE", cache)
	}
	for _, name := range []string{"qpdf", "mutool", "pdftoppm"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("oracle %s is not installed: %v", name, err)
		}
	}
	entries, err := review.LoadManifest(filepath.Join("review", "corpus.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	cfg := ivy.Config{Fonts: os.DirFS(filepath.Join(repo, "fonts"))}
	for _, e := range entries {
		format, ok := review.FormatFor(e.Kind)
		if e.Kind == "asset" || !ok {
			continue
		}
		t.Run(e.Name, func(t *testing.T) {
			t.Parallel()
			cfg := cfg
			cfg.Password = e.Password()
			encodeCorpus(t, format, cfg, e.Path(repo, cache), e.Kind == "pdf" || e.Kind == "ai")
		})
	}
}

// encodeCorpus writes the pages of the document at path as a PDF file, and
// compares them.
func encodeCorpus(t *testing.T, format ivy.Format, cfg ivy.Config, path string, isPDF bool) {
	ctx := t.Context()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("file not in the cache: %v", err)
	}
	doc, err := review.OpenIvy(ctx, format, bytes.NewReader(data), cfg, filepath.Dir(path))
	if err != nil {
		t.Skipf("ivy does not open the file: %v", err)
	}
	defer func() { _ = doc.Close() }()
	pager, ok := doc.(ivy.Pager)
	if !ok {
		t.Skip("the document has no pages")
	}
	n, err := pager.PageCount(ctx)
	if err != nil || n == 0 {
		t.Skipf("the document has no pages: %v", err)
	}
	var numbers []int
	for i := range min(n, 3) {
		numbers = append(numbers, i)
	}
	if n > 3 {
		numbers = append(numbers, n-1)
	}
	var buf bytes.Buffer
	err = pdfenc.Encode(ctx, &buf, doc, pdfenc.WithConfig(cfg), pdfenc.WithPages(numbers...))
	if _, partial := errors.AsType[*ivy.PartialError](err); err != nil && !partial {
		t.Fatalf("encoding: %v", err)
	} else if partial {
		t.Logf("encoded in part: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.pdf")
	if err := os.WriteFile(out, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if msg, err := exec.CommandContext(ctx, "qpdf", "--check", out).CombinedOutput(); err != nil {
		// qpdf exits with 3 for a warning
		if exit, ok := errors.AsType[*exec.ExitError](err); !ok || exit.ExitCode() != 3 {
			t.Errorf("qpdf --check: %v: %s", err, lastLines(string(msg), 5))
		} else {
			t.Logf("qpdf --check warns: %s", lastLines(string(msg), 5))
		}
	}
	knockout := bytes.Contains(buf.Bytes(), []byte("/K true"))
	adjust := bytes.Contains(buf.Bytes(), []byte("/SA true"))
	written, err := pdf.Open(ctx, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("opening the written file: %v", err)
	}
	defer func() { _ = written.Close() }()
	for i, num := range numbers {
		src, err := pager.Page(ctx, num)
		if err != nil {
			t.Fatal(err)
		}
		_, drawer := src.(display.Drawer)
		want, err := ivy.RenderImage(ctx, src, 72, color.White)
		if want == nil {
			t.Fatalf("page %d: rendering the source: %v", num+1, err)
		}
		page, err := written.Page(ctx, i)
		if err != nil {
			t.Fatalf("page %d: %v", num+1, err)
		}
		got, err := ivy.RenderImage(ctx, page, 72, color.White)
		if got == nil {
			t.Fatalf("page %d: rendering the written page: %v", num+1, err)
		}
		ivyDiff := cropDiff(got, want)
		line := fmt.Sprintf("pdfenc: page %d drawer=%t ivy=%.2f", num+1, drawer, ivyDiff)
		why, differs := pdfencDiffers[strings.TrimPrefix(t.Name(), "TestPDFEnc/")]
		switch {
		case ivyDiff >= 0 && ivyDiff <= viewerDiff:
		case differs:
			t.Logf("page %d: not compared through ivy: %s", num+1, why)
		default:
			t.Errorf("page %d: expected the written page within a mean difference of %d through ivy, got %.2f", num+1, viewerDiff, ivyDiff)
		}
		images := map[string]image.Image{"source": want, "ivy": got}
		for _, v := range viewers {
			img, err := v.render(ctx, out, i+1, false)
			if err != nil {
				t.Errorf("page %d: %v", num+1, err)
				continue
			}
			images[v.name] = img
			diff := cropDiff(img, want)
			line += fmt.Sprintf(" %s=%.2f", v.name, diff)
			same := -1.0
			if isPDF {
				if orig, err := v.render(ctx, path, num+1, true); err == nil {
					same = cropDiff(img, orig)
					line += fmt.Sprintf(" %s-source=%.2f", v.name, same)
				}
			}
			switch {
			case diff >= 0 && diff <= viewerDiff:
			case same >= 0 && same <= viewerDiff:
				t.Logf("page %d: %s draws the source file as differently", num+1, v.name)
			case v.name == "mupdf" && adjust:
				t.Logf("page %d: not compared through mutool, which ignores the stroke adjustment", num+1)
			case v.name == "poppler" && knockout:
				t.Logf("page %d: not compared through pdftoppm, which draws no knockout group", num+1)
			case differs:
				t.Logf("page %d: not compared through %s: %s", num+1, v.name, why)
			default:
				t.Errorf("page %d: expected the page of %s within a mean difference of %d, got %.2f", num+1, v.name, viewerDiff, diff)
			}
		}
		t.Log(line)
		if dir := os.Getenv("IVY_PDFENC_PNG"); dir != "" {
			base := filepath.Join(dir, strings.NewReplacer("/", "_").Replace(t.Name())+"-"+strconv.Itoa(num+1))
			for name, img := range images {
				writePNG(t, base+"-"+name+".png", img)
			}
		}
	}
	if dir := os.Getenv("IVY_PDFENC_PNG"); dir != "" {
		name := strings.NewReplacer("/", "_").Replace(t.Name()) + ".pdf"
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// viewer renders page n, from 1, of the PDF file at path at 72 dots for
// each inch. source is true for a source file of the corpus, whose crop box
// is the page, as in ivy.
type viewer struct {
	name   string
	render func(ctx context.Context, path string, n int, source bool) (image.Image, error)
}

// viewers are MuPDF and Poppler.
var viewers = []viewer{
	{name: "mupdf", render: func(ctx context.Context, path string, n int, _ bool) (image.Image, error) {
		return mutoolPage(ctx, path, n)
	}},
	{name: "poppler", render: func(ctx context.Context, path string, n int, source bool) (image.Image, error) {
		args := []string{"-png", "-r", "72", "-f", strconv.Itoa(n), "-l", strconv.Itoa(n), "-singlefile"}
		if source {
			args = append(args, "-cropbox")
		}
		return run(ctx, "pdftoppm", append(args, path)...)
	}},
}

// pdfencDiffers lists the documents of the corpus whose written pages ivy or
// a viewer draws in another way than ivy draws the source, and why. On
// 2026-11-19, each one looked the same as the source to a person. The test
// logs their difference through ivy and the viewers, and does not compare
// it.
var pdfencDiffers = map[string]string{
	"archive-org/djvu/ivanovskaiakolo00istogoog.djvu": scanDiffers,
	"archive-org/djvu/maudinelriven00thomgoog.djvu":   scanDiffers,
	"archive-org/djvu/memoirupontopog00smygoog.djvu":  scanDiffers,
	"dejapdf/djvu/bundled.djvu":                       scanDiffers,
	"dejapdf/djvu/pdf2djvu.djvu":                      scanDiffers,
	"dejapdf/djvu/shared-dict.djvu":                   scanDiffers,
	"fbreaderj/fb2/testfb2book.fb2":                   pictureDiffers,
	"idpf/epub/accessible-epub-3.epub":                pictureDiffers,
	"libreoffice/odp/slide-tile-background.odp":       pictureDiffers,
	"ripgrep-all/fb2/wasteland.fb2":                   pictureDiffers,
	"gutenberg/txt/pg24264.txt":                       textDiffers,
	"pdfjs/pdf/text-cjk-nonembedded.pdf":              textDiffers,
	"pdfjs/pdf/text-office-nonembedded.pdf":           textDiffers,
	"poi/xls/landscape-a4-drawing.xls":                textDiffers,
	"poi/xls/landscape-a4-japanese.xls":               textDiffers,
	"poi/ppt/backgrounds.ppt":                         "Poppler draws the tiles of the background at another phase than ivy and MuPDF",
	"repo/pdf/font-type3-widths-with-matrix.pdf": "the glyphs are stencil masks, which the file holds as soft masks, and ivy and the viewers " +
		"scale a soft mask down in another way than a stencil mask. The pages are the same at 300 dots for each inch",
}

// The reasons of pdfencDiffers.
const (
	scanDiffers    = "Poppler scales the scan of the page down with another filter, and its text is darker"
	pictureDiffers = "a picture of fine detail, which the viewer scales down with another filter"
	textDiffers    = "small dense text: the glyphs are paths in phase 7a, and Poppler fills them with another " +
		"anti-aliasing than the 8 by 8 coverage of the text of ivy (D46)"
)

// mutoolPage renders page n, from 1, of the PDF file at path with mutool
// draw at 72 dots for each inch.
func mutoolPage(ctx context.Context, path string, n int) (image.Image, error) {
	out := path + "." + strconv.Itoa(n) + ".png"
	defer func() { _ = os.Remove(out) }()
	cmd := exec.CommandContext(ctx, "mutool", "draw", "-q", "-r", "72", "-F", "png", "-o", out, path, strconv.Itoa(n))
	if msg, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("running mutool: %w: %s", err, msg)
	}
	f, err := os.Open(out)
	if err != nil {
		return nil, fmt.Errorf("opening the png of mutool: %w", err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding the png of mutool: %w", err)
	}
	return img, nil
}

// cropDiff returns meanDiff of a and b. When their sizes differ by a pixel
// at most, as the rounding of the size of a page can make them, it compares
// the pixels that both hold, as sameSize does.
func cropDiff(a, b image.Image) float64 {
	a, b, ok := sameSize(a, b)
	if !ok {
		return -1
	}
	return meanDiff(a, b)
}

// lastLines returns the last n lines of s.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}
