# goldmark

This folder is a copy of [github.com/yuin/goldmark](https://github.com/yuin/goldmark)
at the commit `d473f9516b6b`, under the MIT license in `LICENSE` (D7, D21).
The commit is version 2 of goldmark, with the module path
`github.com/yuin/goldmark/v2`. The rules for a copy are in
[`docs/UPSTREAM.md`](../../docs/UPSTREAM.md).

The package `markdown` of ivy uses the copy to read CommonMark and to write
HTML.

## The files

Each file has the same path as in upstream.

| Folder | What it holds |
| --- | --- |
| the root | `package.go`, and the tests of CommonMark and of the options |
| `ast` | the tree of a Markdown document |
| `parser` | the parser of CommonMark |
| `renderer`, `renderer/html` | the renderer, and the renderer of HTML |
| `text`, `util` | the reader of the text, and the shared code |
| `extension`, `extension/ast` | the extensions: tables, strikethrough, links without brackets, task lists, footnotes and definition lists |
| `testutil` | the code that the tests share |
| `testdata`, `extension/testdata` | the cases of the tests |

The copy leaves out these files of upstream:

1. The folders `_benchmark` and `_tools`. They import outside packages.
2. The folder `fuzz`. The fuzz test of the package `markdown` of ivy reads
   Markdown through the copy.
3. `doc_test.go`. It runs the Go command on the examples of the README of
   upstream.
4. The extension for typography: `extension/typographer.go`, its test and
   `extension/testdata/typographer.txt`. ivy does not use it.
5. The files of the project: `go.mod`, `go.sum`, `Makefile`, `.gitignore`,
   `.golangci.yml`, `README.md`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`,
   and the folders `.github`, `.agent-plugins` and `.claude-plugin`.

The copy imports only the standard library and its own packages.

## The changes

Each change is marked in the code with a comment that starts with `ivy:`.

1. Each import of `github.com/yuin/goldmark/v2` names the copy,
   `github.com/xo/ivy/internal/goldmark`.
2. gofmt formats `util/html5entities.gen.go` and
   `util/unicode_case_folding.gen.go`, which upstream leaves unformatted.
3. `extension/table.go` leaves a table as a paragraph when the table holds
   more than 65536 cells and more cells than bytes of text. A table pads each
   row to the number of columns of its header. Without the change, a text of
   200 KB asks for billions of empty cells, and the memory runs out.
