// Copied from github.com/yuin/goldmark, file text/reader_test.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

package text_test

import (
	"regexp"
	"testing"

	"github.com/xo/ivy/internal/goldmark/text"
)

func TestFindSubMatchReader(t *testing.T) {
	s := "微笑"
	r := text.NewReader([]byte(":"+s+":"), text.NewDecoder())
	reg := regexp.MustCompile(`:(\p{L}+):`)
	match := r.FindSubMatch(reg)
	if len(match) != 2 || string(match[1]) != s {
		t.Fatal("no match cjk")
	}
}
