// Copied from github.com/yuin/goldmark, file extension/footnote_test.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

package extension

import (
	"bytes"
	"testing"

	gast "github.com/xo/ivy/internal/goldmark/ast"
	"github.com/xo/ivy/internal/goldmark/parser"
	"github.com/xo/ivy/internal/goldmark/renderer/html"
	"github.com/xo/ivy/internal/goldmark/testutil"
	"github.com/xo/ivy/internal/goldmark/text"
	"github.com/xo/ivy/internal/goldmark/util"
)

func TestFootnote(t *testing.T) {
	markdown := testutil.NewMarkdownToStringFunc(
		parser.New(
			parser.WithExtensions(NewFootnoteParser()),
		),
		html.New(
			html.WithUnsafe(),
			html.WithExtensions(NewFootnoteHTMLRenderer()),
		),
	)
	testutil.DoTestCaseFile(markdown, "testdata/footnote.txt", t, testutil.ParseCliCaseArg()...)
}

type footnoteID struct {
}

func (a *footnoteID) Transform(node *gast.Document, _ text.Reader, _ parser.Context) {
	node.Metadata()["footnote-prefix"] = "article12-"
}

func TestFootnoteOptions(t *testing.T) {
	markdown := testutil.NewMarkdownToStringFunc(
		parser.New(
			parser.WithExtensions(NewFootnoteParser()),
		),
		html.New(
			html.WithUnsafe(),
			html.WithExtensions(NewFootnoteHTMLRenderer(
				WithIDPrefix("article12-"),
				WithLinkClass("link-class"),
				WithBacklinkClass("backlink-class"),
				WithLinkTitle("link-title-%%-^^"),
				WithBacklinkTitle("backlink-title"),
				WithBacklinkHTML("^"),
			)),
		),
	)

	testutil.DoTestCase(
		markdown,
		testutil.MarkdownTestCase{
			No:          1,
			Description: "Footnote with options",
			Markdown: `That's some text with a footnote.[^1]

Same footnote.[^1]

Another one.[^2]

[^1]: And that's the footnote.
[^2]: Another footnote.
`,
			Expected: `<p>That's some text with a footnote.<sup id="article12-fnref:1"><a href="#article12-fn:1" class="link-class" title="link-title-2-1" role="doc-noteref">1</a></sup></p>
<p>Same footnote.<sup id="article12-fnref1:1"><a href="#article12-fn:1" class="link-class" title="link-title-2-1" role="doc-noteref">1</a></sup></p>
<p>Another one.<sup id="article12-fnref:2"><a href="#article12-fn:2" class="link-class" title="link-title-1-2" role="doc-noteref">2</a></sup></p>
<div class="footnotes" role="doc-endnotes">
<hr>
<ol>
<li id="article12-fn:1">
<p>And that's the footnote.&#160;<a href="#article12-fnref:1" class="backlink-class" title="backlink-title" role="doc-backlink">^</a>&#160;<a href="#article12-fnref1:1" class="backlink-class" title="backlink-title" role="doc-backlink">^</a></p>
</li>
<li id="article12-fn:2">
<p>Another footnote.&#160;<a href="#article12-fnref:2" class="backlink-class" title="backlink-title" role="doc-backlink">^</a></p>
</li>
</ol>
</div>`,
		},
		t,
	)

	markdown = testutil.NewMarkdownToStringFunc(
		parser.New(
			parser.WithASTTransformers(
				util.Prioritized[parser.ASTTransformer](&footnoteID{}, 100),
			),
			parser.WithExtensions(NewFootnoteParser()),
		),
		html.New(
			html.WithUnsafe(),
			html.WithExtensions(NewFootnoteHTMLRenderer(
				WithIDPrefixFunction(func(n gast.Node) []byte {
					v, ok := n.OwnerDocument().Metadata()["footnote-prefix"]
					if ok {
						return util.StringToReadOnlyBytes(v.(string))
					}
					return nil
				}),
				WithLinkClass("link-class"),
				WithBacklinkClass("backlink-class"),
				WithLinkTitle("link-title-%%-^^"),
				WithBacklinkTitle("backlink-title"),
				WithBacklinkHTML("^"),
			)),
		),
	)

	testutil.DoTestCase(
		markdown,
		testutil.MarkdownTestCase{
			No:          2,
			Description: "Footnote with an id prefix function",
			Markdown: `That's some text with a footnote.[^1]

Same footnote.[^1]

Another one.[^2]

[^1]: And that's the footnote.
[^2]: Another footnote.
`,
			Expected: `<p>That's some text with a footnote.<sup id="article12-fnref:1"><a href="#article12-fn:1" class="link-class" title="link-title-2-1" role="doc-noteref">1</a></sup></p>
<p>Same footnote.<sup id="article12-fnref1:1"><a href="#article12-fn:1" class="link-class" title="link-title-2-1" role="doc-noteref">1</a></sup></p>
<p>Another one.<sup id="article12-fnref:2"><a href="#article12-fn:2" class="link-class" title="link-title-1-2" role="doc-noteref">2</a></sup></p>
<div class="footnotes" role="doc-endnotes">
<hr>
<ol>
<li id="article12-fn:1">
<p>And that's the footnote.&#160;<a href="#article12-fnref:1" class="backlink-class" title="backlink-title" role="doc-backlink">^</a>&#160;<a href="#article12-fnref1:1" class="backlink-class" title="backlink-title" role="doc-backlink">^</a></p>
</li>
<li id="article12-fn:2">
<p>Another footnote.&#160;<a href="#article12-fnref:2" class="backlink-class" title="backlink-title" role="doc-backlink">^</a></p>
</li>
</ol>
</div>`,
		},
		t,
	)
}

func TestFootnoteRenderSubtree(t *testing.T) {
	p := parser.New(parser.WithExtensions(NewFootnoteParser()))
	r := html.New(html.WithExtensions(NewFootnoteHTMLRenderer(
		WithLinkTitle("link-title-%%-^^"),
	)))
	source := []byte("A[^1] and B[^1].\n\nMore text.\n\n[^1]: Note.\n")
	doc := p.Parse(source)

	var buf bytes.Buffer
	if err := r.Render(&buf, source, doc.FirstChild()); err != nil {
		t.Fatal(err)
	}
	expected := `<p>A<sup id="fnref:1"><a href="#fn:1" class="footnote-ref" title="link-title-2-1" role="doc-noteref">1</a></sup> ` +
		`and B<sup id="fnref1:1"><a href="#fn:1" class="footnote-ref" title="link-title-2-1" role="doc-noteref">1</a></sup>.</p>` + "\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}
