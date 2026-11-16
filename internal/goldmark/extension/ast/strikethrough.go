// Copied from github.com/yuin/goldmark, file extension/ast/strikethrough.go, at commit d473f9516b6b
// (D7, docs/UPSTREAM.md). Its MIT license is in internal/goldmark/LICENSE.
// ivy: the imports name the copy, github.com/xo/ivy/internal/goldmark.

// Package ast defines AST nodes that represents extension's elements
package ast

import (
	gast "github.com/xo/ivy/internal/goldmark/ast"
)

// A Strikethrough struct represents a strikethrough of GFM text.
type Strikethrough struct {
	gast.BaseInline
}

// Dump implements Node.Dump.
func (n *Strikethrough) Dump(_ []byte) *gast.NodeDump {
	return gast.NewNodeDump(n, nil)
}

// KindStrikethrough is a NodeKind of the Strikethrough node.
var KindStrikethrough = gast.NewNodeKind("Strikethrough")

// Kind implements Node.Kind.
func (n *Strikethrough) Kind() gast.NodeKind {
	return KindStrikethrough
}

// NewStrikethrough returns a new Strikethrough node.
func NewStrikethrough() *Strikethrough {
	n := &Strikethrough{}
	n.Init(n)
	return n
}
