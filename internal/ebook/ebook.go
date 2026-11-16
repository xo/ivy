// Package ebook holds the code that the formats of ebooks share: the OPF
// package of EPUB, HTMLZ and TXTZ, the style sheets that give the size of the
// page and the default fonts, the decode of text, and a light scan of the
// tags of HTML.
//
// The scan of tags reads the title and the metadata of a document, and the
// first image of the page of a cover. It is not a parser of HTML. The layout
// reads each document with the parser of HTML of okage (D21).
package ebook

import (
	"fmt"

	"github.com/xo/ivy"
)

// BookStyle is the style sheet of an ebook: a page of 6 by 9 inches, the
// size of a trade paperback, and a serif font. The style sheets of the book
// come after it, so they can change each rule.
const BookStyle = `@page {
  size: 6in 9in;
  margin: 0.6in 0.5in;
}
html {
  font-family: serif;
  font-size: 11pt;
  line-height: 1.3;
}
body {
  margin: 0;
}
img, svg, video {
  max-width: 100%;
}
`

// PaperStyle is the style sheet of a document that is not an ebook, such as
// a web page: a page of US Letter, and a serif font.
const PaperStyle = `@page {
  size: letter;
  margin: 0.75in;
}
html {
  font-family: serif;
  font-size: 12pt;
}
img, svg, video {
  max-width: 100%;
}
`

// Recover turns a panic into an error that wraps ivy.ErrFormat, and sets
// *err to it. A function defers it at a public entry point (D5, D18):
//
//	defer ebook.Recover("opening epub", &err)
//
// A fuzz test still sees the panic in the text of the error.
func Recover(what string, err *error) {
	if p := recover(); p != nil {
		*err = fmt.Errorf("%s: panic: %v: %w", what, p, ivy.ErrFormat)
	}
}
