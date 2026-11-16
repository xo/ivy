// Copied from github.com/yuin/goldmark, file parser/auto_link.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

package parser

import (
	"github.com/xo/ivy/internal/goldmark/ast"
	"github.com/xo/ivy/internal/goldmark/text"
)

type autoLinkParser struct {
}

var defaultAutoLinkParser = &autoLinkParser{}

// NewAutoLinkParser returns a new InlineParser that parses autolinks
// surrounded by '<' and '>' .
func NewAutoLinkParser() InlineParser {
	return defaultAutoLinkParser
}

func (s *autoLinkParser) Trigger() []byte {
	return []byte{'<'}
}

func (s *autoLinkParser) Parse(_ ast.Node, block text.Reader, _ Context) ast.Node {
	line, segment := block.PeekLine()
	stop := ScanEmail(line[1:])
	isEmail := true
	if stop < 0 {
		stop = ScanURL(line[1:])
		isEmail = false
	}
	if stop < 0 {
		return nil
	}
	stop++
	if stop >= len(line) || line[stop] != '>' {
		return nil
	}
	textVal := text.NewSingleLineValueFromIndex(text.NewIndex(segment.Start, segment.Start+stop+1), text.IdentityDecoder)
	labelVal := text.NewSingleLineValueFromIndex(text.NewIndex(segment.Start+1, segment.Start+stop), text.IdentityDecoder)
	block.Advance(stop + 1)
	var dest text.SingleLineValue
	if isEmail {
		dest = text.NewSingleLineValueFromString("mailto:"+string(line[1:stop]), text.IdentityDecoder)
	} else {
		dest = text.NewSingleLineValueFromIndex(text.NewIndex(segment.Start+1, segment.Start+stop), text.IdentityDecoder)
	}
	n := ast.NewAutoLink(dest, labelVal, ast.WithAutoLinkText(textVal))
	return n
}
