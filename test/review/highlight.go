package review

import (
	"context"
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"strings"
	"sync"

	"github.com/xo/ivy/markdown"
	"github.com/xo/transit"
	"github.com/xo/transit/grammars/bash"
	"github.com/xo/transit/grammars/c"
	"github.com/xo/transit/grammars/cpp"
	golang "github.com/xo/transit/grammars/go"
	htmlgrammar "github.com/xo/transit/grammars/html"
	"github.com/xo/transit/grammars/java"
	"github.com/xo/transit/grammars/javascript"
	"github.com/xo/transit/grammars/json"
	"github.com/xo/transit/grammars/python"
	"github.com/xo/transit/grammars/ruby"
	"github.com/xo/transit/grammars/rust"
	"github.com/xo/transit/grammars/sql"
	"github.com/xo/transit/grammars/typescript/typescript"
	"github.com/xo/transit/styles"
)

// The review gives markdown a highlighter of code with transit and the
// style "github" of transit, as iv does (D73).
// markdown-it in Chrome, the reference of Markdown, colors the code with
// Shiki and the light theme of GitHub.

// codeStyle is the style of transit that colors the code.
const codeStyle = "github"

// codeLanguage is a grammar of transit that colors a fenced code block.
type codeLanguage struct {
	// names are the names of the language in an info string, in lower
	// case. The first is the name of the grammar.
	names    []string
	language func() *transit.Language
	queries  fs.FS
	// files are the files of the highlight query in queries, in the order of
	// the tree-sitter.json of the grammar. Of two patterns on one node, the
	// later one wins.
	files []string
	// query compiles the query once, and goroutines share it.
	query func() (*transit.Query, error)
}

// codeLanguages are the languages that the review colors: the grammars of
// iv. The SQL dialects of transit that are close to standard SQL use the
// grammar of SQL.
var codeLanguages = []*codeLanguage{
	{names: []string{"bash", "sh", "shell", "zsh"}, language: bash.Language, queries: bash.Queries},
	{names: []string{"c", "h"}, language: c.Language, queries: c.Queries},
	{
		names: []string{"cpp", "c++", "cc", "cxx", "hpp"}, language: cpp.Language, queries: cpp.Queries,
		files: []string{"queries/c/highlights.scm", "queries/highlights.scm"},
	},
	{names: []string{"go", "golang"}, language: golang.Language, queries: golang.Queries},
	{names: []string{"html", "htm"}, language: htmlgrammar.Language, queries: htmlgrammar.Queries},
	{names: []string{"java"}, language: java.Language, queries: java.Queries},
	{
		names: []string{"javascript", "js", "jsx", "mjs", "cjs"}, language: javascript.Language, queries: javascript.Queries,
		files: []string{"queries/highlights.scm", "queries/highlights-jsx.scm", "queries/highlights-params.scm"},
	},
	{names: []string{"json", "jsonc"}, language: json.Language, queries: json.Queries},
	{names: []string{"python", "py", "python3"}, language: python.Language, queries: python.Queries},
	{names: []string{"ruby", "rb"}, language: ruby.Language, queries: ruby.Queries},
	{names: []string{"rust", "rs"}, language: rust.Language, queries: rust.Queries},
	{
		names: []string{
			"sql", "mysql", "mariadb", "postgres", "postgresql", "pgsql", "psql", "plpgsql",
			"sqlite", "sqlite3", "mssql", "sqlserver", "tsql", "t-sql", "oracle", "plsql",
		},
		language: sql.Language, queries: sql.Queries,
	},
	{
		names: []string{"typescript", "ts", "mts", "cts"}, language: typescript.Language, queries: typescript.Queries,
		files: []string{"queries/highlights.scm", "queries/javascript/highlights.scm"},
	},
}

// codeNames maps each name of a language to its grammar.
var codeNames = func() map[string]*codeLanguage {
	m := make(map[string]*codeLanguage)
	for _, l := range codeLanguages {
		if l.files == nil {
			l.files = []string{"queries/highlights.scm"}
		}
		l.query = sync.OnceValues(l.compile)
		for _, name := range l.names {
			m[name] = l
		}
	}
	return m
}()

// compile joins the files of the highlight query in order, and compiles
// them as one query.
func (l *codeLanguage) compile() (*transit.Query, error) {
	var b strings.Builder
	for _, name := range l.files {
		buf, err := fs.ReadFile(l.queries, name)
		if err != nil {
			return nil, fmt.Errorf("reading highlight query %s of %s: %w", name, l.names[0], err)
		}
		b.Write(buf)
		b.WriteByte('\n')
	}
	q, err := transit.NewQuery(l.language(), b.String())
	if err != nil {
		return nil, fmt.Errorf("compiling highlight query of %s: %w", l.names[0], err)
	}
	return q, nil
}

// highlighter returns a markdown.Highlighter that colors code with transit
// and the style codeStyle. It keeps a parser and a query cursor, so one
// document uses it, and goroutines must not share it. ctx is the context of
// the parses, without its cancel.
func highlighter(ctx context.Context, log *slog.Logger) markdown.Highlighter {
	style, ok := styles.Get(codeStyle)
	if !ok {
		log.Warn("review highlighter: no style", "style", codeStyle)
		return nil
	}
	ctx = context.WithoutCancel(ctx)
	parser, cursor := transit.NewParser(), transit.NewQueryCursor()
	return func(lang, code string) (string, bool) {
		l, ok := codeNames[strings.ToLower(lang)]
		if !ok {
			return "", false
		}
		s, err := colorCode(ctx, parser, cursor, style, l, code)
		if err != nil {
			log.Warn("review highlighter", "lang", lang, "err", err)
			return "", false
		}
		return s, true
	}
}

// colorCode returns code as HTML, with a span for each run of bytes that
// have the same capture name, and a style in style.
func colorCode(ctx context.Context, parser *transit.Parser, cursor *transit.QueryCursor, style *styles.Style, l *codeLanguage, code string) (string, error) {
	q, err := l.query()
	if err != nil {
		return "", err
	}
	if err := parser.SetLanguage(l.language()); err != nil {
		return "", fmt.Errorf("setting language %s: %w", l.names[0], err)
	}
	src := []byte(code)
	tree, err := parser.Parse(ctx, src, nil)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", l.names[0], err)
	}
	// capture holds the capture name of each byte. The captures come in the
	// order of the nodes, so a later capture is inside an earlier one or
	// after it, or a later pattern on the same node: the last one wins.
	capture := make([]string, len(src))
	root, names := tree.RootNode(), q.CaptureNames()
	cursor.SetByteRange(root.StartByte(), root.EndByte())
	for m, i := range cursor.Captures(ctx, q, root, src) {
		c := m.Captures[i]
		name := names[c.Index]
		if strings.HasPrefix(name, "_") {
			// the name of a predicate, not of a style
			continue
		}
		end := min(c.Node.EndByte(), len(capture))
		for j := c.Node.StartByte(); j < end; j++ {
			capture[j] = name
		}
	}
	var b strings.Builder
	for start := 0; start < len(src); {
		end := start + 1
		for end < len(src) && capture[end] == capture[start] {
			end++
		}
		text := html.EscapeString(string(src[start:end]))
		if css := captureCSS(style, capture[start]); css != "" {
			b.WriteString(`<span style="` + css + `">` + text + `</span>`)
		} else {
			b.WriteString(text)
		}
		start = end
	}
	return b.String(), nil
}

// captureCSS returns the declarations of the style of a capture name, or ""
// for no capture name or a capture without a style.
func captureCSS(style *styles.Style, name string) string {
	if name == "" {
		return ""
	}
	e := style.Lookup(name)
	var decls []string
	if e.Fg.IsSet() {
		r, g, b := e.Fg.RGB()
		decls = append(decls, fmt.Sprintf("color: #%02x%02x%02x", r, g, b))
	}
	if e.Bold {
		decls = append(decls, "font-weight: bold")
	}
	if e.Italic {
		decls = append(decls, "font-style: italic")
	}
	if e.Underline {
		decls = append(decls, "text-decoration: underline")
	}
	return strings.Join(decls, "; ")
}
