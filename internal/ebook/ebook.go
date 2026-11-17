// Package ebook holds the code that the formats of ebooks share: the OPF
// package of EPUB, HTMLZ and TXTZ, the style sheets that give the size of the
// page and the default fonts, the decode of text, the language of a plain
// text, the head and the first image of an HTML document, and the page that
// shows a cover image.
//
// The package reads HTML with the HTML parser of okage, as the layout does
// (D7, D21).
package ebook

import (
	"fmt"
	"strconv"

	"github.com/xo/ivy"
)

// BookStyle is the style sheet of an ebook: a page of 6 by 9 inches, the
// size of a trade paperback, and a serif font. BookStyleFor gives it on
// another paper. The style sheets of the book
// come after it, so they can change each rule that is not important.
//
// The rules match the pages that calibre makes of an ebook with the same
// page and font. They come from measurements of the PDF files of
// ebook-convert, not from the code of calibre, which is under the GPL (D7):
//
//   - calibre takes the size and the margins of the page from its options,
//     and leaves out those of the book. It also leaves out the margins of
//     the body, and its padding on the left and on the right. It keeps the
//     padding at the top and at the bottom: a padding of 2em at the top of
//     the body moves the first line down by 22.5pt. These rules are
//     important, so that the book cannot change them.
//   - The font of the text is 15px, which is 11.25pt. calibre takes a size
//     in pixels, so a font of 11pt is not possible.
//   - A line of Liberation Serif at 15px is 17px high, as Chromium draws
//     it, so the height of a line is 17/15 of the font.
//   - A word that is wider than the column breaks at any letter.
//   - The code of the generic monospace family is 16/15 of the text, as
//     Chromium draws it.
//   - An image is at most as high as the page area.
//   - A line break ends the line whatever its display, as in Chromium. A
//     book can give it display: block, which would add an empty line.
var BookStyle = BookStyleFor(ivy.Paper{})

// BookStyleFor returns the style sheet of an ebook on the paper p, with the
// margins of an ebook. The zero p is the default paper of an ebook, 6 by 9
// inches (D32).
func BookStyleFor(p ivy.Paper) string {
	_, h := BookArea(p)
	return `@page {
  size: ` + PageSize(p, "6in 9in") + ` !important;
  margin: ` + bookMarginY + ` ` + bookMarginX + ` !important;
}
html {
  font-family: serif;
  font-size: 15px;
  line-height: 1.1333;
  overflow-wrap: break-word;
}
` + BodyStyle + `pre, code, kbd, samp, tt {
  font-size: 1.0667em;
}
img, svg, video {
  max-width: 100%;
}
img {
  max-height: ` + points(h) + `;
}
br {
  display: inline !important;
}
`
}

// BodyStyle is the rule of the body that calibre applies to each book and
// each document that it converts: the body has no margin, and no padding on
// the left and on the right. BookStyle holds it. A format that calibre
// converts with the page of PaperStyle, such as CHM, Markdown or text, adds
// it after PaperStyle. Without it, the body of HTML has a margin of 8px,
// and the text starts 6pt to the right of the text of calibre.
const BodyStyle = `body {
  margin: 0 !important;
  padding-left: 0 !important;
  padding-right: 0 !important;
}
`

// The default paper of an ebook, and the margins of its pages in points
// and as CSS lengths.
var bookPaper = ivy.Paper{Width: 432, Height: 648}

const (
	bookMarginXPt = 36
	bookMarginYPt = 43.2
	bookMarginX   = "0.5in"
	bookMarginY   = "0.6in"
)

// bookPaperOf returns p, or the default paper of an ebook if p is zero.
func bookPaperOf(p ivy.Paper) ivy.Paper {
	if p.Width <= 0 || p.Height <= 0 {
		return bookPaper
	}
	return p
}

// BookArea returns the size in points of the page area of an ebook on the
// paper p: the page less its margins. The zero p is the default paper of an
// ebook, with a page area of 5 by 7.8 inches.
func BookArea(p ivy.Paper) (width, height float64) {
	p = bookPaperOf(p)
	return max(float64(p.Width)-2*bookMarginXPt, 1), max(float64(p.Height)-2*bookMarginYPt, 1)
}

// PageSize returns the value of the property size of @page for the paper
// p, such as "595.28pt 841.89pt", or def if p is zero.
func PageSize(p ivy.Paper, def string) string {
	if p.Width <= 0 || p.Height <= 0 {
		return def
	}
	return points(float64(p.Width)) + " " + points(float64(p.Height))
}

// points returns v as a CSS length in points.
func points(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 32) + "pt"
}

// PaperStyle is the style sheet of a document that is not an ebook, such as
// a web page: a page of US Letter, and a serif font. PaperStyleFor gives it
// on another paper.
var PaperStyle = PaperStyleFor(ivy.Paper{})

// PaperStyleFor returns the style sheet of a document that is not an ebook
// on the paper p, with margins of 0.75 inch. The zero p is US Letter (D32).
// The size of the page is not important, so a document that gives the size
// of its pages keeps it.
func PaperStyleFor(p ivy.Paper) string {
	return `@page {
  size: ` + PageSize(p, "letter") + `;
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
}

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
