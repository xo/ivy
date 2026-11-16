// Copied from github.com/yuin/goldmark, file extension/tasklist_test.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

package extension

import (
	"testing"

	"github.com/xo/ivy/internal/goldmark/parser"
	"github.com/xo/ivy/internal/goldmark/renderer/html"
	"github.com/xo/ivy/internal/goldmark/testutil"
)

func TestTaskList(t *testing.T) {
	markdown := testutil.NewMarkdownToStringFunc(
		parser.New(
			parser.WithExtensions(NewTaskListItemParser()),
		),
		html.New(
			html.WithUnsafe(),
			html.WithExtensions(NewTaskListItemHTMLRenderer()),
		),
	)
	testutil.DoTestCaseFile(markdown, "testdata/tasklist.txt", t, testutil.ParseCliCaseArg()...)
}
