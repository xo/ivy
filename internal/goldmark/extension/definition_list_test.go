// Copied from github.com/yuin/goldmark, file extension/definition_list_test.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

package extension

import (
	"testing"

	"github.com/xo/ivy/internal/goldmark/parser"
	"github.com/xo/ivy/internal/goldmark/renderer/html"
	"github.com/xo/ivy/internal/goldmark/testutil"
)

func TestDefinitionList(t *testing.T) {
	markdown := testutil.NewMarkdownToStringFunc(
		parser.New(
			parser.WithExtensions(NewDefinitionListParser()),
		),
		html.New(
			html.WithUnsafe(),
			html.WithExtensions(NewDefinitionListHTMLRenderer()),
		),
	)
	testutil.DoTestCaseFile(markdown, "testdata/definition_list.txt", t, testutil.ParseCliCaseArg()...)
}

func TestDefinitionListWithGFMRendererOrder(t *testing.T) {
	for _, exts := range [][]html.Extension{
		{NewGFMHTMLRenderer(), NewDefinitionListHTMLRenderer()},
		{NewDefinitionListHTMLRenderer(), NewGFMHTMLRenderer()},
	} {
		markdown := testutil.NewMarkdownToStringFunc(
			parser.New(
				parser.WithExtensions(NewDefinitionListParser(), NewGFMParser()),
			),
			html.New(html.WithExtensions(exts...)),
		)
		got, err := markdown("Go\n:   A language.\n")
		if err != nil {
			t.Fatal(err)
		}
		expected := "<dl>\n<dt>Go</dt>\n<dd>A language.</dd>\n</dl>\n"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	}
}
