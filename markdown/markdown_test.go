package markdown

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook/ebooktest"
	"github.com/xo/ivy/internal/fixture"
)

// sample is a Markdown file with the parts that the style sheet styles.
const sample = "Intro text.\n\n" +
	"# The *Title* of `ivy`\n" +
	"\n" +
	"A paragraph with **bold**, ~~struck~~ and a link to https://example.com.\n" +
	"\n" +
	"## Images\n" +
	"\n" +
	"![a logo](img/logo.png) ![missing](img/none.png) ![outside](../secret.png) ![remote](https://example.com/a.png)\n" +
	"![again](./img/logo.png)\n" +
	"\n" +
	"| Name | Size |\n" +
	"| :--- | ---: |\n" +
	"| a    | 1    |\n" +
	"\n" +
	"- [x] done\n" +
	"- [ ] open\n" +
	"\n" +
	"```go\n" +
	"fmt.Println(\"<hi>\")\n" +
	"```\n" +
	"\n" +
	"> A quote.\n" +
	"\n" +
	"Term\n" +
	": Definition\n" +
	"\n" +
	"A note.[^1]\n" +
	"\n" +
	"[^1]: The note.\n" +
	"\n" +
	"<div class=\"raw\">raw html</div>\n"

// dir is the folder of the sample.
func dir() fstest.MapFS {
	return fstest.MapFS{
		"img/logo.png": {Data: fixture.PNG(4, 4, color.White)},
		"secret.png":   {Data: []byte("secret")},
	}
}

func TestOpen(t *testing.T) {
	d, err := Format.Open(t.Context(), strings.NewReader(sample), ivy.Config{})
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.(ivy.Describer).Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "The Title of ivy" {
		t.Errorf("expected the first heading of level 1 as the title, got: %q", info.Title)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSource(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader(sample), withDir(dir()))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Documents) != 1 || src.Title != "The Title of ivy" || len(src.StyleSheets) != 2 {
		t.Fatalf("unexpected source: %+v", src)
	}
	buf, err := fs.ReadFile(src.FS, src.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	doc := string(buf)
	for _, want := range []string{
		"<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<title>The Title of ivy</title>",
		`<h1 id="the-title-of-ivy">The <em>Title</em> of <code>ivy</code></h1>`,
		`<strong>bold</strong>, <del>struck</del>`,
		`<a href="https://example.com">https://example.com</a>`,
		`<img src="images/0.png" alt="a logo">`,
		` missing outside remote` + "\n" + `<img src="images/0.png" alt="again">`,
		`<th style="text-align:left">Name</th>`,
		`<input checked="" disabled="" type="checkbox"`,
		`<pre><code class="language-go">fmt.Println(&quot;&lt;hi&gt;&quot;)`,
		"<blockquote>\n<p>A quote.</p>",
		"<dt>Term</dt>\n<dd>Definition</dd>",
		`<a href="#fn:1" class="footnote-ref" role="doc-noteref">1</a>`,
		`<div class="raw">raw html</div>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected the html to hold %q, got:\n%s", want, doc)
		}
	}
	img, err := fs.ReadFile(src.FS, "images/0.png")
	if err != nil || !bytes.Equal(img, fixture.PNG(4, 4, color.White)) {
		t.Errorf("expected the logo in the fs, got: %v", err)
	}
	if _, err := fs.Stat(src.FS, "images/1.png"); err == nil {
		t.Error("expected only one image in the fs")
	}
}

func TestNoFolder(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader("![a logo](logo.png) ![inline](data:image/png;base64,AAAA)"))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	buf, _ := fs.ReadFile(src.FS, src.Documents[0])
	if !strings.Contains(string(buf), "<p>a logo <img src=\"data:image/png;base64,AAAA\" alt=\"inline\"></p>") {
		t.Errorf("expected the alternate text and the data url, got:\n%s", buf)
	}
	info, _ := d.Describe(t.Context())
	if info.Title != "" {
		t.Errorf("expected no title, got: %q", info.Title)
	}
}

func TestEncodings(t *testing.T) {
	for name, buf := range map[string][]byte{
		"bom":     append([]byte{0xef, 0xbb, 0xbf}, "# Été"...),
		"utf-16":  {0xff, 0xfe, '#', 0, ' ', 0, 0xc9, 0, 't', 0, 0xe9, 0},
		"invalid": []byte("# \xc9t\xe9"),
	} {
		t.Run(name, func(t *testing.T) {
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			info, _ := d.Describe(t.Context())
			// text that is not valid UTF-8 is windows-1252, as ebook.Text reads it
			if info.Title != "Été" {
				t.Errorf("expected %q, got: %q", "Été", info.Title)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		cfg  ivy.Config
		want error
	}{
		"nul":       {text: "a\x00b", want: ivy.ErrFormat},
		"too large": {text: sample, cfg: ivy.Config{Limits: ivy.Limits{MaxMemory: 10}}, want: ivy.ErrLimit},
		"too deep":  {text: strings.Repeat(">", 100) + " deep", want: ivy.ErrLimit},
		"too many":  {text: sample, cfg: ivy.Config{Limits: ivy.Limits{MaxObjects: 10}}, want: ivy.ErrLimit},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Open(t.Context(), strings.NewReader(tc.text), WithConfig(tc.cfg))
			if !errors.Is(err, tc.want) {
				t.Errorf("expected %v, got: %v", tc.want, err)
			}
		})
	}
	d, err := Open(t.Context(), strings.NewReader(""))
	if err != nil {
		t.Fatalf("expected an empty file to open, got: %v", err)
	}
	if _, err := d.source(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// TestHostile parses input that is slow for a naive parser of Markdown. Each
// one must finish in a short time.
func TestHostile(t *testing.T) {
	const n = 50000
	for name, text := range map[string]string{
		"brackets":   strings.Repeat("[", n) + strings.Repeat("]", n),
		"links":      strings.Repeat("[a](", n),
		"emphasis":   strings.Repeat("*a _b ", n),
		"backticks":  strings.Repeat("`a``", n),
		"quotes":     strings.Repeat(">", n) + " deep",
		"lists":      strings.Repeat("- ", n) + "deep",
		"html":       strings.Repeat("<a ", n),
		"entities":   strings.Repeat("&#x", n),
		"table":      "|" + strings.Repeat("a|", n) + "\n|" + strings.Repeat("-|", n) + "\n" + strings.Repeat("|b\n", n),
		"references": strings.Repeat("[a]: <", n),
		"wide table": "|" + strings.Repeat("a|", n) + "\n|" + strings.Repeat("-|", n) + "\n|b|\n",
	} {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			d, err := Open(t.Context(), strings.NewReader(text))
			if err == nil {
				_, err = d.source(t.Context())
			}
			if err != nil && !errors.Is(err, ivy.ErrLimit) {
				t.Errorf("expected no error or ivy.ErrLimit, got: %v", err)
			}
			if elapsed := time.Since(start); elapsed > 10*time.Second {
				t.Errorf("took %v", elapsed)
			}
		})
	}
}

func TestCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, strings.NewReader(sample)); err == nil {
		t.Error("expected an error for a canceled context")
	}
	d, err := Open(t.Context(), strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.source(ctx); err == nil {
		t.Error("expected an error for a canceled context")
	}
	if _, err := d.source(t.Context()); err != nil {
		t.Errorf("expected a later call to convert the text, got: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add([]byte(sample))
	f.Add([]byte("> > - [ ] *a* **b** ~~c~~ [^1]\n\n[^1]: x\n\n|a|b|\n|-|-|\n|1|2|\n"))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 4 << 20, MaxPixels: 1 << 20, MaxObjects: 10000, MaxDepth: 16}}
	files := dir()
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg), withDir(files))
		if err != nil {
			return
		}
		_, _ = d.Describe(t.Context())
		_, _ = d.source(t.Context())
		ebooktest.RenderFirstPage(t, d)
	})
}

func TestPages(t *testing.T) {
	d, err := Open(t.Context(), strings.NewReader(sample), withDir(dir()))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected a page, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 612 || h != 792 {
		t.Errorf("expected a page of US Letter, got: %g by %g", w, h)
	}
	img := ebooktest.RenderPage(t, d, 0)
	// "Intro text." is in the top left corner, inside the margins of 0.75
	// inch, and the title follows it.
	if ink := ebooktest.Ink(img, image.Rect(72, 72, 300, 100)); ink < 20 {
		t.Errorf("expected the ink of the text, got %d pixels", ink)
	}
	if ink := ebooktest.Ink(img, image.Rect(72, 100, 500, 160)); ink < 100 {
		t.Errorf("expected the ink of the title, got %d pixels", ink)
	}
	if ink := ebooktest.Ink(img, image.Rect(0, 0, 816, 70)); ink != 0 {
		t.Errorf("expected no ink in the top margin, got %d pixels", ink)
	}
	if _, err := d.Page(t.Context(), n); !errors.Is(err, ivy.ErrPageRange) {
		t.Errorf("expected ivy.ErrPageRange, got: %v", err)
	}
}
