// Package layout is the layout of D10: a port of the layout of WeasyPrint
// (D15, D21). It lays out documents of HTML and CSS onto pages.
//
// The port is done in stages. On 2026-11-16 the package holds Source, the
// input that each format gives it, Book, and stages A, B, C and D. Stage A
// is the document tree, the parts of html.py and urls.py that read a
// document, and the cascade of css/, which computes the style of each
// element, pseudo-element and page. Stage B is the box tree of
// formatting_structure/, the fonts and the line breaks of text/, and the
// images of images.py. Stage C is the layout of layout/: Document lays out
// the boxes on pages. Stage D is the drawing of draw/ and stacking.py:
// DrawPage draws a page onto the display list of D10. Book puts the
// documents of a whole Source on pages, for the formats. README.md lists
// the files and the differences from upstream.
package layout

import "io/fs"

// Source is a flow of text that a format converts into HTML and CSS, for the
// layout to put on pages. Each format makes a Source from its own structure,
// such as the spine of an EPUB file or the body of a FictionBook file.
type Source struct {
	// FS holds the documents and their resources: the style sheets, the
	// images and the fonts that the documents link to. A link in a document
	// is a path in FS, relative to that document.
	FS fs.FS

	// Documents are the paths in FS of the documents, in the order of
	// reading. A document whose path ends in .xhtml, .xht or .xml is read as
	// XHTML. Any other document is read as HTML.
	Documents []string

	// StyleSheets are style sheets of the format, as CSS text. The layout
	// applies them after its own default style sheet and before the style
	// sheets of the documents. A format sets the default fonts and the size
	// of the page here, with @page.
	StyleSheets []string

	// Title is the title of the whole work, if the format knows it.
	Title string
}
