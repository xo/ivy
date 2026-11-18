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
// documents of a whole Source on pages, for the formats. The images also
// read EMF and WMF, and a Source can give objects that a format draws
// itself, such as charts. README.md lists the files and the differences
// from upstream.
package layout

import (
	"context"
	"io/fs"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
)

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

	// Objects are the pictures that the format draws itself, such as the
	// charts of an Office document, by name. An <img> element whose src is
	// ObjectURL(name), or an <object> element whose data is ObjectURL(name),
	// shows the object. A name holds only ASCII letters, digits, "-", "_"
	// and ".". The layout reads no file for an object.
	Objects map[string]Object

	// Splitter cuts the long documents into parts that a Book lays out on
	// their own, or nil (D55).
	Splitter Splitter
}

// Splitter cuts a long document of a Source, such as a long sheet, into
// parts. A Book lays out each part as a document of its own, so that the
// first page of the document does not wait for the whole document, and
// PageCount lays out the parts at the same time (D55). A Splitter must be
// safe for concurrent use.
type Splitter interface {
	// Split returns the parts of the document path, in order, or nil when
	// the book lays out the document whole.
	Split(path string) []Part
	// Recut returns the part that takes the place of next, the part after
	// part, when the probe row of part does not start a page. row is the
	// first row of the page that holds the probe row of part. The new part
	// starts at that row and ends where next ends.
	Recut(part, next Part, row int) Part
}

// Part is a part of a long document (D55). A row of a part is an element
// tr of its document. The rows are counted in tree order, from 0.
//
// The document of a part holds more rows than the pages that the book
// keeps. The rows before the cut row keep the borders and the merged cells
// above the cut row as the whole document has them. The probe row and the
// rows after it keep the borders and the merged cells below the last row
// that the part keeps.
type Part struct {
	// Path is the path in FS of the HTML document of the part.
	Path string
	// Cut is the cut row, or -1 for none. The book drops the pages before
	// the page that starts at the cut row.
	Cut int
	// Probe is the probe row, or -1 for none. The book drops the page that
	// holds the probe row and the pages after it. When the probe row does
	// not start that page, the book asks Splitter.Recut for the next part.
	Probe int
	// Pages is the number of pages that the format expects the book to
	// keep of the part.
	Pages int
}

// ObjectScheme is the scheme of the URL of an object of Source.Objects. No
// file of the FS of a Source has such a URL, because the URL of a file has
// the scheme file.
const ObjectScheme = "ivy-object"

// ObjectURL returns the URL of the object of Source.Objects with the name
// name, such as "ivy-object:chart1".
func ObjectURL(name string) string {
	return ObjectScheme + ":" + name
}

// Object is a picture that a format draws itself, such as a chart. The
// layout places it as a replaced element, as it places an image, and draws
// it with Draw. An Object must be safe for concurrent use, because the pages
// of a book can be drawn at the same time.
type Object interface {
	// Size returns the intrinsic size of the object in points, 1/72 of an
	// inch. If the width or the height is zero or less, the object has no
	// intrinsic size, and the layout gives it the size of the style of its
	// element, or 300 by 150 CSS pixels.
	Size() (width, height float64)
	// Draw draws the object onto dev, into the rectangle rect of the user
	// space of dev. The y axis of the user space points down. Draw must
	// leave the state of dev as it found it, so it saves and restores the
	// state of dev around a change. It returns an *ivy.PartialError when it
	// draws the object in part, and the error of ctx when ctx ends. The
	// layout logs any other error, and draws the page in part.
	Draw(ctx context.Context, dev display.Device, rect okage.Rect) error
}
