// Copied from github.com/yuin/goldmark, file extension/strikethrough_test.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

package extension

import (
	"testing"

	"github.com/xo/ivy/internal/goldmark/parser"
	"github.com/xo/ivy/internal/goldmark/renderer/html"
	"github.com/xo/ivy/internal/goldmark/testutil"
)

func TestStrikethrough(t *testing.T) {
	markdown := testutil.NewMarkdownToStringFunc(
		parser.New(
			parser.WithExtensions(NewStrikethroughParser()),
		),
		html.New(
			html.WithUnsafe(),
			html.WithExtensions(NewStrikethroughHTMLRenderer()),
		),
	)
	testutil.DoTestCaseFile(markdown, "testdata/strikethrough.txt", t, testutil.ParseCliCaseArg()...)
}
