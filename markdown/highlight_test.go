package markdown

import (
	"html"
	"image/color"
	"io/fs"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/ebook/ebooktest"
)

// fakeHighlighter colors code in the languages go and python. It puts each
// word of the code in a span of the color red, and counts its calls.
type fakeHighlighter struct {
	calls []string
}

func (f *fakeHighlighter) highlight(lang, code string) (string, bool) {
	f.calls = append(f.calls, lang)
	if lang != "go" && lang != "python" {
		return "", false
	}
	var b strings.Builder
	for i, word := range strings.Split(code, " ") {
		if i > 0 {
			b.WriteByte(' ')
		}
		if word != "" {
			b.WriteString(`<span style="color: #ff0000">` + html.EscapeString(word) + `</span>`)
		}
	}
	return b.String(), true
}

// sourceHTML returns the HTML that the document of text gives the layout.
func sourceHTML(t *testing.T, text string, opts ...Option) string {
	t.Helper()
	d, err := Open(t.Context(), strings.NewReader(text), opts...)
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	buf, err := fs.ReadFile(src.FS, src.Documents[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(buf)
}

func TestHighlight(t *testing.T) {
	var f fakeHighlighter
	text := "```go\nfmt.Println(\"<hi>\")\n```\n\n" +
		"```python {.x}\nprint(1)\n```\n\n" +
		"```rust,ignore\nfn main() {}\n```\n\n" +
		"```\nno language\n```\n\n" +
		"    indented code\n"
	doc := sourceHTML(t, text, WithHighlighter(f.highlight))
	for _, want := range []string{
		`<pre><code class="language-go"><span style="color: #ff0000">fmt.Println("&lt;hi&gt;")` + "\n" + `</span></code></pre>`,
		`<pre><code class="language-python"><span style="color: #ff0000">print(1)` + "\n" + `</span></code></pre>`,
		// the highlighter does not know rust
		`<pre><code class="language-rust,ignore">fn main() {}` + "\n" + `</code></pre>`,
		"<pre><code>no language\n</code></pre>",
		"<pre><code>indented code\n</code></pre>",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected the html to hold %q, got:\n%s", want, doc)
		}
	}
	// a block without a language and an indented block do not call the
	// highlighter, and the language is cut at a comma or a brace
	if got := strings.Join(f.calls, " "); got != "go python rust" {
		t.Errorf("expected calls for go, python and rust, got: %q", got)
	}
}

func TestHighlightPage(t *testing.T) {
	var f fakeHighlighter
	d, err := Open(t.Context(), strings.NewReader("```go\nfunc main() {}\n```\n"), WithHighlighter(f.highlight))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := count(ebooktest.RenderPage(t, d, 0), color.NRGBA{R: 0xff, A: 0xff}); n < 20 {
		t.Errorf("expected red code on the page, got %d red pixels", n)
	}
}

func TestHighlightClean(t *testing.T) {
	const code = "a < b\n"
	for name, tc := range map[string]struct {
		html string
		want string
	}{
		"other element": {
			html: `<b>a</b> <span style="color: red">&lt;</span> <script>b</script>` + "\n",
			want: `a <span style="color: red">&lt;</span> b` + "\n",
		},
		"other property": {
			html: `<span style="color: #123456; font-size: 40px; FONT-WEIGHT: bold">a</span> &lt; b` + "\n",
			want: `<span style="color: #123456; font-weight: bold">a</span> &lt; b` + "\n",
		},
		"unsafe value": {
			html: `<span style="color: url(&quot;x&quot;)">a</span> &lt; <span style="background-color: #fff;">b</span>` + "\n",
			want: `a &lt; <span style="background-color: #fff">b</span>` + "\n",
		},
		"nested spans": {
			html: `<span style="font-style: italic"><span style="color: blue">a</span> &lt;</span> b` + "\n",
			want: `<span style="font-style: italic"><span style="color: blue">a</span> &lt;</span> b` + "\n",
		},
		"wrong text": {html: "a &lt; c\n"},
		"lost text":  {html: "a <b\n"},
	} {
		t.Run(name, func(t *testing.T) {
			h := func(string, string) (string, bool) { return tc.html, true }
			doc := sourceHTML(t, "```go\n"+code+"```\n", WithHighlighter(h))
			want := `<code class="language-go">` + tc.want + `</code>`
			if tc.want == "" {
				// the block keeps one color
				want = `<code class="language-go">a &lt; b` + "\n</code>"
			}
			if !strings.Contains(doc, want) {
				t.Errorf("expected the html to hold %q, got:\n%s", want, doc)
			}
		})
	}
}

func TestHighlightPanic(t *testing.T) {
	h := func(lang, code string) (string, bool) {
		if lang == "go" {
			panic("broken highlighter")
		}
		return `<span style="color: red">` + html.EscapeString(code) + `</span>`, true
	}
	doc := sourceHTML(t, "```go\nx\n```\n\n```c\ny\n```\n", WithHighlighter(h))
	for _, want := range []string{
		`<code class="language-go">x` + "\n</code>",
		`<code class="language-c"><span style="color: red">y` + "\n</span></code>",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected the html to hold %q, got:\n%s", want, doc)
		}
	}
}

func TestHighlightLimits(t *testing.T) {
	t.Run("size", func(t *testing.T) {
		var f fakeHighlighter
		long := strings.Repeat("word ", maxHighlight/5+1)
		doc := sourceHTML(t, "```go\n"+long+"\n```\n\n```go\nshort\n```\n", WithHighlighter(f.highlight))
		if len(f.calls) != 1 {
			t.Errorf("expected one call, for the short block, got: %q", f.calls)
		}
		if !strings.Contains(doc, `<code class="language-go">word word `) {
			t.Error("expected the long block in one color")
		}
		if !strings.Contains(doc, `<span style="color: #ff0000">short`) {
			t.Error("expected the short block in color")
		}
	})
	t.Run("objects", func(t *testing.T) {
		var f fakeHighlighter
		cfg := ivy.Config{Limits: ivy.Limits{MaxObjects: 100}}
		doc := sourceHTML(t, "```go\n"+strings.Repeat("a ", 200)+"\n```\n", WithConfig(cfg), WithHighlighter(f.highlight))
		if len(f.calls) != 1 || strings.Contains(doc, "<span") {
			t.Errorf("expected one call and the block in one color, got: %q", f.calls)
		}
	})
}

func TestAlerts(t *testing.T) {
	text := "> [!NOTE]\n> Useful text.\n\n" +
		"> [!warning]\n>\n> Second paragraph.\n\n" +
		"> [!TIP] text on the line\n\n" +
		"> [!OTHER]\n> Not an alert.\n\n" +
		"> [!CAUTION]\n\n" +
		"- > [!NOTE]\n  > In a list.\n"
	doc := sourceHTML(t, text)
	for _, want := range []string{
		`<blockquote class="markdown-alert markdown-alert-note">` +
			`<p class="markdown-alert-title"><img src="data:image/svg+xml;base64,`,
		`Note</p>` + "\n<p>Useful text.</p>",
		`<blockquote class="markdown-alert markdown-alert-warning">` +
			`<p class="markdown-alert-title"><img src="data:image/svg+xml;base64,`,
		`Warning</p>` + "\n<p>Second paragraph.</p>",
		"<blockquote>\n<p>[!TIP] text on the line</p>",
		"<blockquote>\n<p>[!OTHER]\nNot an alert.</p>",
		"<blockquote>\n<p>[!CAUTION]</p>",
		"<blockquote>\n<p>[!NOTE]\nIn a list.</p>",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("expected the html to hold %q, got:\n%s", want, doc)
		}
	}
}
