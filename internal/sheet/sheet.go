// Package sheet puts the sheets of a spreadsheet on pages. A format reads
// its file into a Workbook: the cells with the text that they show, their
// styles, the widths of the columns, the heights of the rows, the merged
// cells and the setup of the page. Source converts the Workbook into HTML
// and CSS for the layout of D10, as printing does (D24): each sheet in
// order, each one in a document of its own.
//
// A sheet that is wider than its page is cut into bands of columns. The
// pages show the rows of the first band, then the rows of the next band, as
// Excel prints "down, then over", or the first rows of each band, then the
// next rows, as Excel prints "over, then down". The rows of the print
// titles repeat at the top of each page, and the columns of the print
// titles at the left of each band. A band ends at its last row that shows
// something, as Excel prints no page without ink.
//
// The conditional formats of a sheet change the look of its cells: a
// style, a color scale, a data bar or an icon. The comments of the cells
// can follow the cells on pages of their own.
//
// xlsx, msxls, odf and csv share the package. The two formats of Excel,
// xlsx and msxls, also share the sizes of the papers, the widths of the
// columns, the heights of the rows and the codes of the headers and the
// footers of Excel: PaperSize, Digit, RowScale and ParseHeaderFooter.
package sheet

import (
	"image/color"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/layout"
)

// MaxRows and MaxCols are the size of a sheet of Excel. A reader drops a
// cell past them.
const (
	MaxRows = 1 << 20
	MaxCols = 1 << 14
)

// Workbook is a spreadsheet: its sheets and the styles that their cells
// share.
type Workbook struct {
	// Sheets are the sheets to print, in order. A reader leaves out the
	// hidden sheets.
	Sheets []*Sheet
	// Styles are the styles of the cells. Styles[0] is the default style,
	// whose font is the font of the whole workbook. A workbook without
	// styles takes DefaultStyle.
	Styles []Style
	// Title is the title of the workbook, for the header and the footer.
	Title string
	// Files are the files that the images of the sheets name, such as
	// "media/image1.png", with their content.
	Files map[string][]byte
	// Objects are the pictures that the format draws itself, such as the
	// charts, by name. An image whose Object is set shows one.
	Objects map[string]layout.Object
}

// Sheet is one sheet. The indexes of the rows and the columns count from 0.
type Sheet struct {
	// Name is the name of the sheet, as its tab shows it.
	Name string
	// DefaultColWidth and DefaultRowHeight are the sizes of a column and of
	// a row without a size of their own, in points.
	DefaultColWidth, DefaultRowHeight float64
	// Cols holds the columns with a size or a style of their own, sorted
	// by Min, without overlaps.
	Cols []Col
	// Rows holds the rows with a cell, a size or a style, sorted by Index,
	// each index once.
	Rows []Row
	// Merges are the merged cells.
	Merges []Range
	// Page is the setup of the page.
	Page Page
	// Gridlines draws the light lines between the cells.
	Gridlines bool
	// PrintAreas are the ranges to print. Without one, the page shows the
	// cells that hold data or a visible style.
	PrintAreas []Range
	// TitleRows and TitleCols are the print titles, or nil.
	TitleRows, TitleCols *Span
	// RowBreaks and ColBreaks are the manual page breaks: the index of a
	// row or a column that starts a new page.
	RowBreaks, ColBreaks []int
	// Header and Footer are the header and the footer of the pages.
	Header, Footer HeaderFooter
	// Images are the images and the charts of the sheet.
	Images []Image
	// Note is a line that the page shows after the cells, such as the
	// note that a reader stopped at a limit.
	Note string
	// FixedRows keeps the height of each row, as Excel prints it: a line of
	// text that is higher than its row crosses the edges of the row. Without
	// it, the row grows to hold the line.
	FixedRows bool
	// CondFormats are the conditional formats of the cells.
	CondFormats []CondFormat
	// Comments are the comments of the cells, which the pages show after
	// the cells when Page.CommentsAtEnd is set, as Excel prints them.
	Comments []Comment
}

// Comment is the comment of a cell: its author and its text.
type Comment struct {
	Row, Col     int
	Author, Text string
}

// Range is a rectangle of cells, from the cell (Row0, Col0) to the cell
// (Row1, Col1), inclusive.
type Range struct {
	Row0, Col0, Row1, Col1 int
}

// Span is a run of rows or columns, from First to Last, inclusive.
type Span struct {
	First, Last int
}

// Col is a run of columns with the same size and style.
type Col struct {
	// Min and Max are the first and the last index of the run.
	Min, Max int
	// Width is the width in points, or 0 for the default width.
	Width float64
	// Hidden hides the columns.
	Hidden bool
	// Style is the style of the cells of the columns that the sheet does
	// not hold, or 0.
	Style int
}

// Row is a row of a sheet.
type Row struct {
	Index int
	// Height is the height in points, or 0 for the default height. The
	// layout makes a row taller when its text needs it.
	Height float64
	// Hidden hides the row.
	Hidden bool
	// Style is the style of the cells of the row that the sheet does not
	// hold, or 0.
	Style int
	// Cells are the cells of the row, sorted by Col, each column once.
	Cells []Cell
}

// Kind is the kind of the value of a cell. The alignment General aligns a
// number to the right, a boolean and an error to the center, and text to
// the left.
type Kind uint8

// The kinds of the values.
const (
	KindText Kind = iota
	KindNumber
	KindBool
	KindError
)

// Cell is a cell of a row.
type Cell struct {
	Col int
	// Text is the text that the cell shows, such as a number formatted
	// with its format code.
	Text string
	// Runs is the rich text of the cell. If it is set, the cell shows it
	// in the place of Text.
	Runs []Run
	// Style is the index of the style of the cell in Workbook.Styles.
	Style int
	Kind  Kind
	// Value is the number of a cell of KindNumber, which the conditional
	// formats compare.
	Value float64
	// AsText aligns a number of the alignment General to the left, as
	// text. Excel does so for a number whose format is text only, "@".
	AsText bool
	// Color is the color of the section of the format code, such as
	// [Red], or transparent.
	Color color.NRGBA
}

// Run is a run of rich text.
type Run struct {
	Text string
	// Font is the font of the run, or nil for the font of the cell.
	Font *Font
}

// Style is the style of a cell.
type Style struct {
	Font Font
	// Fill is the color of the background, or transparent.
	Fill color.NRGBA
	// Borders are the borders: top, right, bottom and left.
	Borders [4]Border
	// HAlign is the horizontal alignment: "" for General, "left",
	// "center", "right", "justify", "fill", "distributed" or
	// "centerContinuous".
	HAlign string
	// VAlign is the vertical alignment: "" or "bottom", "top", "center",
	// "justify" or "distributed".
	VAlign string
	// Wrap breaks the text into lines.
	Wrap bool
	// Indent is the indent of the text in points.
	Indent float64
	// Rotation is the angle of the text in degrees, counterclockwise, from
	// -90 to 90, or 255 for letters stacked from top to bottom.
	Rotation int
	// Shrink makes the text smaller to fit the cell.
	Shrink bool
}

// Font is a font of a cell.
type Font struct {
	// Family is the name of the family, such as "Calibri".
	Family string
	// Generic is the generic family that follows Family: "serif",
	// "sans-serif" or "monospace". It is "sans-serif" when empty.
	Generic string
	// Size is the size in points, or 0 for the size of the default style.
	Size                 float64
	Bold, Italic, Strike bool
	// Underline is 0 for none, 1 for a single line and 2 for a double
	// line.
	Underline int
	// Color is the color of the text, or transparent for black.
	Color color.NRGBA
	// VertAlign is 0, or 1 for superscript and 2 for subscript.
	VertAlign int
}

// Border is a side of the border of a cell.
type Border struct {
	// Width is the width in points, or 0 for none.
	Width float64
	// Line is "solid", "dashed", "dotted" or "double".
	Line string
	// Color is the color, or transparent for black.
	Color color.NRGBA
}

// Page is the setup of the page of a sheet. The sizes are in points.
type Page struct {
	// Width and Height are the size of the paper in portrait.
	Width, Height float64
	// Landscape turns the paper.
	Landscape bool
	// Top, Right, Bottom and Left are the margins.
	Top, Right, Bottom, Left float64
	// Header and Footer are the distances of the header from the top of
	// the paper and of the footer from its bottom.
	Header, Footer float64
	// Scale is the scale of the cells, such as 0.5, or 0 for 1. Excel
	// takes a scale from 10% to 400%.
	Scale float64
	// Fit sets the scale so that the cells fit on FitWidth pages across
	// and FitHeight pages down. A count of 0 sets no bound.
	Fit                 bool
	FitWidth, FitHeight int
	// CenterH centers the cells on the page across, and CenterV centers
	// them down the page.
	CenterH, CenterV bool
	// ScaleHeaderFooter scales the text of the header and the footer with
	// the cells, as Excel does unless a sheet asks otherwise.
	ScaleHeaderFooter bool
	// HeaderArea and FooterArea are the heights of the areas of the header
	// and the footer between the margins and the cells, as OpenDocument
	// has them. With ScaleHeaderFooter, the scale of the cells scales
	// them too, as LibreOffice does.
	HeaderArea, FooterArea float64
	// OverThenDown prints the pages in the order over, then down: the
	// first rows of each band of columns, then the next rows. Without it,
	// the pages go down, then over: all the rows of the first band, then
	// of the next band.
	OverThenDown bool
	// CommentsAtEnd prints the comments of the cells on pages after the
	// cells.
	CommentsAtEnd bool
}

// HeaderFooter is a header or a footer: the text of its three parts.
type HeaderFooter struct {
	Left, Center, Right []Field
	// Font is the font of the text, or nil for the font of the default
	// style.
	Font *Font
}

// Empty reports whether the header or the footer shows nothing.
func (hf HeaderFooter) Empty() bool {
	return len(hf.Left) == 0 && len(hf.Center) == 0 && len(hf.Right) == 0
}

// FieldKind is the kind of a field of a header or a footer.
type FieldKind uint8

// The kinds of the fields. The page and the date of the printing are not
// known, so a date and a time are left out.
const (
	FieldText FieldKind = iota
	FieldPage
	FieldPages
	FieldSheet
	FieldTitle
)

// Field is a piece of a header or a footer: text, or a field that the page
// fills.
type Field struct {
	Kind FieldKind
	Text string
}

// Image is an image or a chart on a sheet, anchored at a cell.
type Image struct {
	// Row and Col are the cell of the top left corner.
	Row, Col int
	// DX and DY are the offset of the corner in the cell, in points.
	DX, DY float64
	// Width and Height are the size, in points.
	Width, Height float64
	// Path is the path of the image in Workbook.Files, or "".
	Path string
	// Object is the name of the picture in Workbook.Objects, or "".
	Object string
	// Label is the text of a frame that stands for a picture that the
	// package cannot show, such as a chart, when Path and Object are
	// empty.
	Label string
}

// DefaultStyle is the default style of a workbook without styles: Calibri
// of 11 points, the default font of Excel.
var DefaultStyle = Style{Font: Font{Family: "Calibri", Size: 11}}

// The sizes of a US Letter and an A4 page, in points.
const (
	LetterWidth  = 612
	LetterHeight = 792
	A4Width      = 595.28
	A4Height     = 841.89
)

// DefaultPage is the page of Excel: US Letter, with the margins Normal.
var DefaultPage = Page{
	Width: LetterWidth, Height: LetterHeight,
	Top: 54, Bottom: 54, Left: 50.4, Right: 50.4,
	Header: 21.6, Footer: 21.6,
}

// SetPaper sets the paper of the page to paper, the paper of a program
// for a sheet whose file gives none, in the orientation of paper. The
// margins stay. A zero paper keeps the page (D32). A side of the paper
// stays from 1 inch to 200 inches.
func (p *Page) SetPaper(paper ivy.Paper) {
	if !(paper.Width > 0) || !(paper.Height > 0) {
		return
	}
	portrait := paper.Portrait()
	p.Width = min(max(float64(portrait.Width), 72), 14400)
	p.Height = min(max(float64(portrait.Height), 72), 14400)
	p.Landscape = paper.Width > paper.Height
}

// visible reports whether the style draws something in an empty cell: a
// fill or a border.
func (s *Style) visible() bool {
	if s.Fill.A != 0 {
		return true
	}
	for _, b := range s.Borders {
		if b.Width > 0 {
			return true
		}
	}
	return false
}

// row returns the row of the index i, or nil.
func (sh *Sheet) row(i int) *Row {
	lo, hi := 0, len(sh.Rows)
	for lo < hi {
		m := (lo + hi) / 2
		if sh.Rows[m].Index < i {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(sh.Rows) && sh.Rows[lo].Index == i {
		return &sh.Rows[lo]
	}
	return nil
}

// col returns the run of columns that holds the index i, or nil.
func (sh *Sheet) col(i int) *Col {
	lo, hi := 0, len(sh.Cols)
	for lo < hi {
		m := (lo + hi) / 2
		if sh.Cols[m].Max < i {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(sh.Cols) && sh.Cols[lo].Min <= i && i <= sh.Cols[lo].Max {
		return &sh.Cols[lo]
	}
	return nil
}

// cell returns the cell of the column c in the row r, or nil.
func (r *Row) cell(c int) *Cell {
	if r == nil {
		return nil
	}
	lo, hi := 0, len(r.Cells)
	for lo < hi {
		m := (lo + hi) / 2
		if r.Cells[m].Col < c {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(r.Cells) && r.Cells[lo].Col == c {
		return &r.Cells[lo]
	}
	return nil
}

// colWidth returns the width of the column i in points, or 0 if it is
// hidden.
func (sh *Sheet) colWidth(i int) float64 {
	if c := sh.col(i); c != nil {
		if c.Hidden {
			return 0
		}
		if c.Width > 0 {
			return c.Width
		}
		if c.Width < 0 {
			return 0
		}
	}
	return sh.DefaultColWidth
}

// rowHeight returns the height of the row r in points, or 0 if it is
// hidden.
func (sh *Sheet) rowHeight(r *Row) float64 {
	if r == nil {
		return sh.DefaultRowHeight
	}
	if r.Hidden {
		return 0
	}
	if r.Height > 0 {
		return r.Height
	}
	if r.Height < 0 {
		return 0
	}
	return sh.DefaultRowHeight
}

// ColWidth returns the width of the column i in points, or 0 if it is
// hidden. The columns must be sorted, as Sort sorts them.
func (sh *Sheet) ColWidth(i int) float64 {
	return sh.colWidth(i)
}

// RowHeight returns the height of the row i in points, or 0 if it is
// hidden. The rows must be sorted, as Sort sorts them.
func (sh *Sheet) RowHeight(i int) float64 {
	return sh.rowHeight(sh.row(i))
}

// Sort sorts the rows of the sheet, the cells of each row and the
// columns, and merges the rows and the cells with the same index, the
// later one first. A reader that reads them out of order calls it.
func (sh *Sheet) Sort() {
	sortRows(sh)
}
