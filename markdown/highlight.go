package markdown

import (
	"fmt"
	"strings"

	"github.com/xo/ivy/internal/goldmark/ast"
	"github.com/xo/okage/html"
)

// Highlighter returns the code of a fenced block in the language lang
// as HTML: the text of the code, escaped, with a span element around
// each run of text that has a style. ok is false for a language that
// it does not color, and the block keeps one color.
type Highlighter func(lang, code string) (html string, ok bool)

// WithHighlighter colors the fenced code blocks of a document with h.
func WithHighlighter(h Highlighter) Option {
	return func(o *options) {
		o.highlight = h
	}
}

// maxHighlight is the number of bytes of the largest block of code that a
// Highlighter colors. A larger block keeps one color.
const maxHighlight = 256 << 10

// highlightProps are the properties of the style attribute of a span that
// the HTML of a Highlighter keeps (D73).
var highlightProps = map[string]bool{
	"color":            true,
	"background-color": true,
	"font-weight":      true,
	"font-style":       true,
	"text-decoration":  true,
}

// highlight colors the fenced code blocks of doc with d.highlight. It
// returns the HTML of the code of each block that it colors. A block keeps
// one color in these cases, as D73 says:
//
//   - The block has no language, or the Highlighter does not know it.
//   - The block has more than maxHighlight bytes, or more than the limit of
//     memory.
//   - The HTML of the Highlighter is not the text of the block, or it holds
//     more nodes or a deeper nesting than the limits.
//   - The HTML of all the blocks reaches half of the limit of memory.
//   - The Highlighter panics.
func (d *Document) highlight(doc ast.Node) map[*ast.CodeBlock]string {
	if d.highlighter == nil {
		return nil
	}
	colored := make(map[*ast.CodeBlock]string)
	budget := d.lim.MaxMemory / 2
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		b, ok := n.(*ast.CodeBlock)
		if !ok || !entering || b.CodeBlockKind != ast.CodeBlockKindFenced {
			return ast.WalkContinue, nil
		}
		lang := codeLanguage(b, d.text)
		code := b.Value.Str(d.text)
		if lang == "" || len(code) > maxHighlight || int64(len(code)) > budget {
			return ast.WalkContinue, nil
		}
		s, ok := d.callHighlighter(lang, code)
		if !ok || int64(len(s)) > budget {
			return ast.WalkContinue, nil
		}
		s, ok = d.cleanHighlight(s, code)
		if !ok {
			return ast.WalkContinue, nil
		}
		budget -= int64(len(s))
		colored[b] = s
		return ast.WalkContinue, nil
	})
	return colored
}

// codeLanguage returns the language of a fenced code block for a
// Highlighter: the first word of its info string, up to a comma or a brace,
// as in "rust,ignore" or "js{1,3}". It returns "" for a block without a
// language.
func codeLanguage(b *ast.CodeBlock, text []byte) string {
	lang, ok := b.Language(text)
	if !ok {
		return ""
	}
	lang, _, _ = strings.Cut(lang, ",")
	lang, _, _ = strings.Cut(lang, "{")
	return lang
}

// callHighlighter calls d.highlighter. It returns false if the Highlighter
// panics.
func (d *Document) callHighlighter(lang, code string) (s string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			d.log.Warn("markdown highlighter panicked", "lang", lang, "err", fmt.Sprint(r))
			s, ok = "", false
		}
	}()
	return d.highlighter(lang, code)
}

// cleanHighlight parses s, the HTML of a Highlighter, and returns it with
// only the span elements and the properties of highlightProps in their
// style attributes. It keeps the text of any other element and drops the
// element. It returns false if the text of s is not code, or if s holds
// more nodes or a deeper nesting than the limits. The parse stops at the
// limits, so that the text of s is then shorter than code.
func (d *Document) cleanHighlight(s, code string) (string, bool) {
	doc, err := html.ParseFragment(strings.NewReader(s), "code",
		html.WithCharset("utf-8"), html.WithMaxNodes(d.lim.MaxObjects), html.WithMaxDepth(d.lim.MaxDepth))
	if err != nil {
		return "", false
	}
	var b, text strings.Builder
	root := doc.Root()
	for n := root.FirstChild(); n != nil; {
		switch n.Type() {
		case html.TextNode:
			text.WriteString(n.Data())
			b.WriteString(escape(n.Data()))
		case html.ElementNode:
			if n.FirstChild() != nil {
				if n.Namespace() == html.NamespaceHTML && n.Name() == "span" {
					b.WriteString(openSpan(n))
				}
				n = n.FirstChild()
				continue
			}
		}
		// the next node is the next sibling of n or of an ancestor, and each
		// span that ends is closed
		for n != root && n.NextSibling() == nil {
			n = n.Parent()
			if n != root && n.Type() == html.ElementNode && n.Namespace() == html.NamespaceHTML && n.Name() == "span" {
				b.WriteString(closeSpan(n))
			}
		}
		if n == root {
			break
		}
		n = n.NextSibling()
	}
	// the parser of HTML reads a CR and a CR LF as a LF
	want := strings.ReplaceAll(strings.ReplaceAll(code, "\r\n", "\n"), "\r", "\n")
	if text.String() != want {
		return "", false
	}
	return b.String(), true
}

// openSpan returns the start tag of the span n, with the properties of
// highlightProps in its style, or "" if it keeps none.
func openSpan(n *html.Node) string {
	style := spanStyle(n)
	if style == "" {
		return ""
	}
	return `<span style="` + style + `">`
}

// closeSpan returns the end tag of the span n, or "" if openSpan gave it no
// start tag.
func closeSpan(n *html.Node) string {
	if spanStyle(n) == "" {
		return ""
	}
	return "</span>"
}

// spanStyle returns the declarations of the style attribute of n whose
// properties are in highlightProps, and whose values hold only letters,
// digits, spaces and the characters "#%.,()-". It drops any other
// declaration.
func spanStyle(n *html.Node) string {
	attr, _ := n.Attr("style")
	var decls []string
	for decl := range strings.SplitSeq(attr, ";") {
		prop, value, ok := strings.Cut(decl, ":")
		prop = strings.ToLower(strings.TrimSpace(prop))
		value = strings.TrimSpace(value)
		if !ok || !highlightProps[prop] || value == "" || !safeValue(value) {
			continue
		}
		decls = append(decls, prop+": "+value)
	}
	return strings.Join(decls, "; ")
}

// safeValue reports whether the value of a declaration holds only letters,
// digits, spaces and the characters "#%.,()-". A color, a weight, a style
// and a decoration need no other character.
func safeValue(v string) bool {
	for _, c := range v {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.ContainsRune(" #%.,()-", c):
		default:
			return false
		}
	}
	return true
}

// renderCode returns the HTML of the colored fenced code block b, whose
// code is the HTML s.
func renderCode(b *ast.CodeBlock, s string, text []byte) string {
	var sb strings.Builder
	sb.WriteString("<pre><code")
	if lang, ok := b.Language(text); ok {
		sb.WriteString(` class="language-` + escapeAttr(lang) + `"`)
	}
	sb.WriteString(">" + s + "</code></pre>\n")
	return sb.String()
}

// escapeAttr escapes the characters that HTML reserves in the value of an
// attribute.
func escapeAttr(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
