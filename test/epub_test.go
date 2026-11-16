package test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/xo/ivy"
	"github.com/xo/ivy/epub"
)

// The page of the EPUB of TestEPUB: 4 by 6 inches, with margins of half an
// inch. At 96 dots per inch, the page is 384 by 576 pixels, and the margins
// are 48 pixels.
const (
	oraclePageWidth  = 384
	oraclePageHeight = 576
	oracleMargin     = 48
)

// oracleStyle is the style sheet of the EPUB of TestEPUB. Both programs read
// the font from the file go.ttf of the book, so both draw the same glyphs.
const oracleStyle = `@font-face { font-family: "Oracle"; src: url(go.ttf) }
@page { size: 4in 6in; margin: 0.5in }
html, body { margin: 0; padding: 0 }
body { font-family: "Oracle"; font-size: 16px; line-height: 24px }
h1 { font-size: 24px; line-height: 32px; margin: 0 0 16px; font-weight: normal }
p { margin: 0 0 12px; text-align: left; hyphens: manual }
`

// oracleText is the text of the chapter of the EPUB of TestEPUB.
var oracleText = []string{
	"The quick brown fox jumps over the lazy dog.",
	"Pack my box with five dozen liquor jugs. How vexingly quick daft zebras jump.",
	"Sphinx of black quartz, judge my vow. The five boxing wizards jump quickly.",
	"Jackdaws love my big sphinx of quartz. Amazingly few discotheques provide jukeboxes.",
}

// buildEPUB returns an EPUB 3 file of one chapter with a heading and
// paragraphs, which uses the Go font from the book.
func buildEPUB(t *testing.T, paragraphs int) []byte {
	t.Helper()
	var body strings.Builder
	body.WriteString("<h1>Chapter One</h1>\n")
	for i := range paragraphs {
		fmt.Fprintf(&body, "<p>%s</p>\n", oracleText[i%len(oracleText)])
	}
	files := []struct {
		name, data string
	}{
		{"mimetype", "application/epub+zip"},
		{"META-INF/container.xml", `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`},
		{"OEBPS/content.opf", `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="id">urn:uuid:7b0a9e52-4f2c-4b8e-9b77-5a3d1c0f0001</dc:identifier>
    <dc:title>Oracle</dc:title>
    <dc:language>en</dc:language>
    <meta property="dcterms:modified">2026-11-16T00:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/>
    <item id="style" href="style.css" media-type="text/css"/>
    <item id="font" href="go.ttf" media-type="font/ttf"/>
  </manifest>
  <spine><itemref idref="chapter"/></spine>
</package>`},
		{"OEBPS/nav.xhtml", `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Contents</title></head>
<body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Chapter One</a></li></ol></nav></body>
</html>`},
		{"OEBPS/chapter.xhtml", `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>Chapter One</title><link rel="stylesheet" type="text/css" href="style.css"/></head>
<body>
` + body.String() + `</body>
</html>`},
		{"OEBPS/style.css", oracleStyle},
		{"OEBPS/go.ttf", string(goregular.TTF)},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, f := range files {
		method := zip.Deflate
		if i == 0 {
			// The mimetype comes first, and is stored.
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(f.data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// calibrePDF converts an EPUB file to PDF with ebook-convert of calibre. The
// options give calibre the page and the margins of the style sheet, and stop
// it from changing the sizes of the fonts and the heights of the lines.
func calibrePDF(ctx context.Context, dir, epubPath string) (string, error) {
	path, err := exec.LookPath("ebook-convert")
	if err != nil {
		return "", fmt.Errorf("oracle ebook-convert is not installed: %w", err)
	}
	pdfPath := filepath.Join(dir, "book.pdf")
	cmd := exec.CommandContext(ctx, path, epubPath, pdfPath,
		"--custom-size", "4x6", "--unit", "inch",
		"--pdf-page-margin-left", "36", "--pdf-page-margin-right", "36",
		"--pdf-page-margin-top", "36", "--pdf-page-margin-bottom", "36",
		"--margin-left", "0", "--margin-right", "0", "--margin-top", "0", "--margin-bottom", "0",
		"--disable-font-rescaling", "--minimum-line-height", "0",
		"--chapter-mark", "none", "--page-breaks-before", "/",
		"--pdf-default-font-size", "16", "--embed-all-fonts")
	// calibre keeps its configuration in the folder of the test, and draws
	// without a display.
	cmd.Env = append(os.Environ(),
		"CALIBRE_CONFIG_DIRECTORY="+filepath.Join(dir, "calibre"),
		"QT_QPA_PLATFORM=offscreen",
		"QTWEBENGINE_CHROMIUM_FLAGS=--no-sandbox")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("running ebook-convert: %w: %s", err, out.String())
	}
	return pdfPath, nil
}

// pdfPageCount returns the number of pages of a PDF file, from pdfinfo of
// poppler.
func pdfPageCount(ctx context.Context, path string) (int, error) {
	out, err := exec.CommandContext(ctx, "pdfinfo", path).Output()
	if err != nil {
		return 0, fmt.Errorf("running pdfinfo: %w", err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "Pages:"); ok {
			var n int
			if _, err := fmt.Sscan(rest, &n); err != nil {
				return 0, fmt.Errorf("reading the pages of pdfinfo: %w", err)
			}
			return n, nil
		}
	}
	return 0, fmt.Errorf("pdfinfo gives no page count for %s", path)
}

// inkBox returns the box of the pixels of img that are darker than mid
// grey, over white, and the number of these pixels.
func inkBox(img image.Image) (image.Rectangle, int) {
	b := img.Bounds()
	box := image.Rectangle{}
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := overWhite(img.At(x, y))
			if (c[0]+c[1]+c[2])/3 >= 128 {
				continue
			}
			n++
			box = box.Union(image.Rect(x-b.Min.X, y-b.Min.Y, x-b.Min.X+1, y-b.Min.Y+1))
		}
	}
	return box, n
}

// inkRows returns, for each row of pixels of img, whether the row holds a
// pixel darker than mid grey: the lines of the text.
func inkRows(img image.Image) []bool {
	b := img.Bounds()
	rows := make([]bool, b.Dy())
	for y := range b.Dy() {
		for x := range b.Dx() {
			c := overWhite(img.At(b.Min.X+x, b.Min.Y+y))
			if (c[0]+c[1]+c[2])/3 < 128 {
				rows[y] = true
				break
			}
		}
	}
	return rows
}

// lineBands returns the bands of consecutive rows with ink, as [top, bottom)
// pairs.
func lineBands(rows []bool) [][2]int {
	var bands [][2]int
	for y := 0; y < len(rows); y++ {
		if !rows[y] {
			continue
		}
		top := y
		for y < len(rows) && rows[y] {
			y++
		}
		bands = append(bands, [2]int{top, y})
	}
	return bands
}

// TestEPUB compares the first page of a small EPUB file that ivy draws with
// the page that calibre makes: ebook-convert converts the book to PDF, and
// pdftoppm of poppler draws page 1 of the PDF file.
//
// The book fixes the size of the page, the margins, the font, its size and
// the height of the lines, so the two programs break the lines at the same
// words and put them at the same heights. The pixels still differ: calibre
// lays out with Qt WebEngine (Chromium), whose glyph positions and
// antialiasing differ from okage by a fraction of a pixel, and a line of
// text that moves by one pixel changes every pixel of its glyphs. A
// comparison of the pixels with a tight bound would compare the
// rasterizers, not the layouts. So the test compares what the layout
// decides: the number of pages, the box of the ink of page 1, and the bands
// of rows that hold the lines of text, each within 3 pixels. It also checks
// the mean difference of the pixels, before and after a blur that hides the
// antialiasing, against the bounds of oracleMeanDiff and oracleBlurredDiff.
func TestEPUB(t *testing.T) {
	const paragraphs = 24
	dir := t.TempDir()
	data := buildEPUB(t, paragraphs)
	epubPath := filepath.Join(dir, "book.epub")
	if err := os.WriteFile(epubPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	pdfPath, err := calibrePDF(t.Context(), dir, epubPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := run(t.Context(), "pdftoppm", "-png", "-r", "96", "-f", "1", "-l", "1", "-singlefile", pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	wantPages, err := pdfPageCount(t.Context(), pdfPath)
	if err != nil {
		t.Fatal(err)
	}

	d, err := epub.Open(t.Context(), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
	}()
	pager, ok := any(d).(ivy.Pager)
	if !ok {
		t.Fatal("expected an epub document to be an ivy.Pager")
	}
	gotPages, err := pager.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	page, err := pager.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ivy.RenderImage(t.Context(), page, 96, color.White)
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("IVY_ORACLE_OUT"); out != "" {
		writePNG(t, filepath.Join(out, "epub-ivy.png"), got)
		writePNG(t, filepath.Join(out, "epub-calibre.png"), want)
	}

	if size := got.Bounds().Size(); size != image.Pt(oraclePageWidth, oraclePageHeight) {
		t.Errorf("expected a page of %dx%d pixels from ivy, got: %v", oraclePageWidth, oraclePageHeight, size)
	}
	if size := want.Bounds().Size(); size != image.Pt(oraclePageWidth, oraclePageHeight) {
		t.Errorf("expected a page of %dx%d pixels from calibre, got: %v", oraclePageWidth, oraclePageHeight, size)
	}
	t.Logf("pages: ivy %d, calibre %d", gotPages, wantPages)
	if gotPages != wantPages {
		t.Errorf("expected %d pages, as calibre, got: %d", wantPages, gotPages)
	}

	gotBox, gotInk := inkBox(got)
	wantBox, wantInk := inkBox(want)
	t.Logf("ink box: ivy %v (%d pixels), calibre %v (%d pixels)", gotBox, gotInk, wantBox, wantInk)
	near := func(a, b int) bool { return a-b <= 3 && b-a <= 3 }
	if !near(gotBox.Min.X, wantBox.Min.X) || !near(gotBox.Min.Y, wantBox.Min.Y) ||
		!near(gotBox.Max.X, wantBox.Max.X) || !near(gotBox.Max.Y, wantBox.Max.Y) {
		t.Errorf("expected the ink box of calibre %v within 3 pixels, got: %v", wantBox, gotBox)
	}
	if gotBox.Min.X < oracleMargin-2 || gotBox.Min.Y < oracleMargin-2 ||
		gotBox.Max.X > oraclePageWidth-oracleMargin+2 || gotBox.Max.Y > oraclePageHeight-oracleMargin+2 {
		t.Errorf("expected the ink inside the margins, got: %v", gotBox)
	}

	gotBands, wantBands := lineBands(inkRows(got)), lineBands(inkRows(want))
	t.Logf("lines: ivy %d, calibre %d", len(gotBands), len(wantBands))
	if len(gotBands) != len(wantBands) {
		t.Errorf("expected %d lines, as calibre, got: %d\nivy: %v\ncalibre: %v", len(wantBands), len(gotBands), gotBands, wantBands)
	} else {
		for i := range gotBands {
			if !near(gotBands[i][0], wantBands[i][0]) || !near(gotBands[i][1], wantBands[i][1]) {
				t.Errorf("expected line %d at the rows %v of calibre within 3 pixels, got: %v", i+1, wantBands[i], gotBands[i])
			}
		}
	}

	maxShift := 0
	if len(gotBands) == len(wantBands) {
		for i := range gotBands {
			for k := range 2 {
				maxShift = max(maxShift, gotBands[i][k]-wantBands[i][k], wantBands[i][k]-gotBands[i][k])
			}
		}
	}
	t.Logf("largest shift of a line: %d pixels", maxShift)

	diff := meanDiff(got, want)
	blurred := meanDiff(blur(got, 2), blur(want, 2))
	t.Logf("mean difference: %.2f, after a blur of 5 by 5 pixels: %.2f", diff, blurred)
	if diff < 0 || diff > oracleMeanDiff {
		t.Errorf("expected a mean difference of at most %v, got: %.2f", oracleMeanDiff, diff)
	}
	if blurred < 0 || blurred > oracleBlurredDiff {
		t.Errorf("expected a mean difference after the blur of at most %v, got: %.2f", oracleBlurredDiff, blurred)
	}
}

// The bounds of the mean differences of TestEPUB. On 2026-11-16, the mean
// difference of the pixels was 8.04, and 3.15 after the blur, with the lines
// at most 1 pixel apart. The bounds leave room for a change of calibre or of
// poppler. The ink box and the lines catch a page that is blank or drawn at
// another place. The means catch a page drawn with other glyphs or colors:
// with the margins of calibre 5 points wider, the mean difference was 21.78.
const (
	oracleMeanDiff    = 12
	oracleBlurredDiff = 6
)

// blur returns img over white, blurred with a box of 2r+1 by 2r+1 pixels, as
// a grey image. The blur hides the differences of the antialiasing and of the
// positions of the glyphs within a pixel.
func blur(img image.Image, r int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	grey := make([]float64, w*h)
	for y := range h {
		for x := range w {
			c := overWhite(img.At(b.Min.X+x, b.Min.Y+y))
			grey[y*w+x] = (c[0] + c[1] + c[2]) / 3
		}
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			var sum, n float64
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					if xx, yy := x+dx, y+dy; xx >= 0 && xx < w && yy >= 0 && yy < h {
						sum += grey[yy*w+xx]
						n++
					}
				}
			}
			out.Pix[y*out.Stride+x] = uint8(sum/n + 0.5)
		}
	}
	return out
}
