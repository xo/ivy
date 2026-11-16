// Copied from github.com/yuin/goldmark, file options_test.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

//go:build !goldmark_v1_attribute

package goldmark_test

import (
	"testing"

	"github.com/xo/ivy/internal/goldmark/parser"
	"github.com/xo/ivy/internal/goldmark/renderer/html"
	"github.com/xo/ivy/internal/goldmark/testutil"
)

func TestAttributeAndAutoHeadingID(t *testing.T) {
	markdown := testutil.NewMarkdownToStringFunc(
		parser.New(parser.WithAttribute(), parser.WithAutoHeadingID()),
		html.New(),
	)
	testutil.DoTestCaseFile(markdown, "testdata/options-auto-heading-id.txt", t, testutil.ParseCliCaseArg()...)
}

func TestParseDelimiterSimple(t *testing.T) {
	markdown := testutil.NewMarkdownToStringFunc(
		parser.New(parser.WithParseDelimiterFunc(parser.ParseDelimiterSimple)),
		html.New(html.WithUnsafe()),
	)
	testutil.DoTestCaseFile(markdown, "testdata/options-parse-delimiter-simple.txt", t, testutil.ParseCliCaseArg()...)
}
