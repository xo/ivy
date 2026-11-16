// Copied from github.com/yuin/goldmark, file parser/paragraph.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

package parser

import (
	"github.com/xo/ivy/internal/goldmark/ast"
	"github.com/xo/ivy/internal/goldmark/text"
	"github.com/xo/ivy/internal/goldmark/util"
)

type paragraphParser struct {
}

var defaultParagraphParser = &paragraphParser{}

// NewParagraphParser returns a new BlockParser that
// parses paragraphs.
func NewParagraphParser() BlockParser {
	return defaultParagraphParser
}

func (b *paragraphParser) Trigger() []byte {
	return nil
}

func (b *paragraphParser) Open(_ ast.Node, reader text.Reader, _ Context) (ast.Node, State) {
	line, segment := reader.PeekLine()
	if util.IsBlank(line) {
		return nil, NoChildren
	}
	node := ast.NewParagraph()
	node.AppendSource(segment.TrimLeftSpace(reader.Source()))
	reader.AdvanceToEOL()
	return node, NoChildren
}

func (b *paragraphParser) Continue(node ast.Node, reader text.Reader, _ Context) State {
	line, segment := reader.PeekLine()
	if util.IsBlank(line) {
		return Close
	}
	node.(*ast.Paragraph).AppendSource(segment.TrimLeftSpace(reader.Source()))
	reader.AdvanceToEOL()
	return Continue | NoChildren
}

func (b *paragraphParser) Close(node ast.Node, reader text.Reader, _ Context) {
	para := node.(*ast.Paragraph)
	lines := para.Source()
	if len(lines) != 0 {
		// trim trailing spaces
		length := len(lines)
		para.Source()[length-1] = para.Source()[length-1].TrimRightSpace(reader.Source())
	}
	if len(lines) == 0 {
		node.Parent().RemoveChild(node)
		return
	}
}

func (b *paragraphParser) CanInterruptParagraph() bool {
	return false
}

func (b *paragraphParser) CanAcceptIndentedLine() bool {
	return false
}
