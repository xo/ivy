package rtf

import (
	"bytes"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// sample returns an RTF file with each feature that the tests check: a page
// of 5 by 4 inches with margins of half an inch, a header with the number
// of the page, a table, a picture, a footnote, text in three code pages and
// in Unicode, and a page break.
func sample() []byte {
	png := hex.EncodeToString(fixture.PNG(40, 20, color.NRGBA{R: 255, A: 255}))
	return []byte(`{\rtf1\ansi\ansicpg1252\deff0
{\fonttbl{\f0\fswiss\fcharset0 Arial;}{\f1\froman\fcharset204 Times New Roman;}{\f2\fmodern\fcharset128 MS Gothic;}{\f3\ftech\fcharset2 Symbol;}}
{\colortbl;\red255\green0\blue0;\red0\green0\blue255;}
{\info{\title A title}{\author An author}{\creatim\yr2024\mo3\dy14\hr9\min30}}
\paperw7200\paperh5760\margl720\margr720\margt1080\margb720\headery360
{\header\pard\qr\fs20 Page \chpgn\par}
\pard\qc\b\fs32 Heading\par
\pard\plain\fs24 Plain {\b bold} {\i\cf1 red italic} caf\'e9 {\f1 \'c0\'c1} {\f2 \'82\'a0} \uc1\u8364?euro {\v hidden}{\f3 \'b7}.\par
{\pard\li720\fi-360{\listtext\'95\tab}Item\par}
\pard Note{\super\chftn}{\footnote\pard\plain\fs20{\super\chftn} The note.}\par
\trowd\trgaph108\cellx1440\cellx2880\pard\intbl A\cell B\cell\row
\trowd\trgaph108\clbrdrt\brdrs\cellx1440\cellx2880\pard\intbl C\cell D\cell\row
\pard {\field{\*\fldinst HYPERLINK "https://example.com/"}{\fldrslt link}} {\*\unknown skipped} {\pict\pngblip\picw40\pich20\picwgoal1440\pichgoal720 ` + png + `}\par
\page Second page\par
}`)
}

// convertHTML converts an RTF file and returns its XHTML and the rules of
// its pages.
func convertHTML(t *testing.T, buf []byte) (string, string) {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(src.FS, documentName)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), src.StyleSheets[1]
}

func TestConvert(t *testing.T) {
	html, sheet := convertHTML(t, sample())
	for _, want := range []string{
		`font-weight: bold">bold</span>`,
		`font-style: italic; color: #ff0000">red italic</span>`,
		`café`,
		`АБ`,
		`あ`,
		`€euro`,
		`•`,
		`text-align: center`,
		`<span style="display: inline-block; min-width: 18pt; text-indent: 0">`,
		`<span class="fn"><p `,
		`<span class="fnref"></span><span style="font-size: 10pt"> The note.</span>`,
		`<td style="padding-left: 5.4pt; padding-right: 5.4pt; width: 72pt">`,
		`border-top: 0.5pt solid black`,
		`<a href="https://example.com/">`,
		`<img src="media/1.png" alt="" style="width: 72pt; height: 36pt"/>`,
		`break-before: page`,
		`position: running(hf1)`,
		`Page </span><span class="pg"></span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q", want)
		}
	}
	for _, unwanted := range []string{"hidden", "skipped", "?euro"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("expected the xhtml without %q", unwanted)
		}
	}
	if t.Failed() {
		t.Log(html)
	}
	if want := `@page s1 { size: 360pt 288pt; margin: 54pt 36pt 36pt 36pt; @top-center { content: element(hf1); padding-top: 18pt }`; !strings.Contains(sheet, want) {
		t.Errorf("expected the style sheet to hold %q, got:\n%s", want, sheet)
	}
}

// inkBox returns the box of the pixels of img that are darker than mid
// grey, in the rectangle r.
func inkBox(img image.Image, r image.Rectangle) image.Rectangle {
	var box image.Rectangle
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if (cr+cg+cb)/3 < 0x8000 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

func TestPages(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(sample()))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 360 || h != 288 {
		t.Errorf("expected a page of 360 by 288 points, got: %v by %v", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	header := inkBox(img, image.Rect(0, 0, 360, 54))
	if header.Empty() || header.Min.X < 250 || header.Min.Y < 17 {
		t.Errorf("expected the header at the right of the top margin, got: %v", header)
	}
	if body := inkBox(img, image.Rect(0, 54, 360, 252)); body.Empty() || body.Min.X < 35 {
		t.Errorf("expected the text inside the margins, got: %v", body)
	}
}

func TestDescribe(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(sample()))
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2024, 3, 14, 9, 30, 0, 0, time.UTC)
	if info.Title != "A title" || len(info.Authors) != 1 || info.Authors[0] != "An author" || !info.Created.Equal(want) {
		t.Errorf("expected the title, the author and the date of creation, got: %+v", info)
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader([]byte("not rtf"))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 16}}
	if _, err := Open(t.Context(), bytes.NewReader(sample()), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
}

func TestDeepGroups(t *testing.T) {
	const depth = 10000
	buf := []byte(`{\rtf1 before` + strings.Repeat("{", depth) + "deep" + strings.Repeat("}", depth) + ` after\par}`)
	html, _ := convertHTML(t, buf)
	if strings.Contains(html, "deep") || !strings.Contains(html, "before") || !strings.Contains(html, "after") {
		t.Errorf("expected the groups past the depth left out, and the rest kept, got:\n%s", html)
	}
}

func TestUnbalanced(t *testing.T) {
	for _, s := range []string{`{\rtf1 open {\b bold`, `{\rtf1 }}}} closed`, `{\rtf1\u`, `{\rtf1\'`, `{\rtf1{\pict\bin999999 ab}}`} {
		d, err := Open(t.Context(), bytes.NewReader([]byte(s)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.PageCount(t.Context()); err != nil {
			t.Errorf("expected pages for %q, got: %v", s, err)
		}
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(sample())
	f.Add([]byte(`{\rtf1\ansi{\fonttbl{\f0 Arial;}}\trowd\cellx100\intbl a\cell\row\sect\sectd\pgwsxn100 b\par}`))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1 << 14, MaxDepth: 32}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		_, _ = d.Describe(t.Context())
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		_, _ = ivy.RenderImage(t.Context(), p, 9, nil)
	})
}
