// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/formatting_structure/boxes.py: the classes of
// all the types of boxes of the formatting structure of CSS, the box model.
// See https://www.w3.org/TR/CSS21/visuren.html.
//
// The names are the names of CSS 2.1, with the exception of TextBox. In
// WeasyPrint, any text is in a TextBox. What CSS calls anonymous inline
// boxes are text boxes, but not all text boxes are anonymous inline boxes.
// See https://www.w3.org/TR/CSS21/visuren.html#anonymous.
//
// Port: upstream has a class for each type of box, and the layout tells them
// apart with isinstance. The port has one struct, Box, with a Kind. Each
// abstract class of upstream is a method that reports whether the kind is
// one of its subclasses, such as IsBlockLevel for BlockLevelBox. Is reports
// isinstance for a concrete class. So the layout follows the Python code
// line by line, and a copy of a box is a copy of the struct.

package layout

import (
	"maps"
	"math"
	"strings"
	"unicode"
)

// Kind is the concrete class of a box of weasyprint/formatting_structure/
// boxes.py.
type Kind uint8

// The kinds of boxes. Each one is the class of upstream with the same name
// and the suffix Box.
const (
	// KindBlock is BlockBox: a block-level box that is also a block
	// container. A non-replaced element with a display value of block or
	// list-item generates a block box.
	KindBlock Kind = iota
	// KindLine is LineBox: a line in an inline formatting context. It only
	// holds inline-level boxes. While the box tree is built, one line box
	// holds all the consecutive inline boxes. The layout later splits it
	// into one line box for each line.
	KindLine
	// KindInline is InlineBox: an inline box with inline children. A
	// non-replaced element with a display value of inline generates one.
	KindInline
	// KindText is TextBox: a box that holds only text and no children. Any
	// text of the document ends up in a text box. What CSS calls anonymous
	// inline boxes are also text boxes.
	KindText
	// KindInlineBlock is InlineBlockBox: inline on the outside and a block
	// container on the inside.
	KindInlineBlock
	// KindBlockReplaced is BlockReplacedBox: a replaced box that is
	// block-level.
	KindBlockReplaced
	// KindInlineReplaced is InlineReplacedBox: a replaced box that is
	// inline-level.
	KindInlineReplaced
	// KindTable is TableBox: the box of display: table.
	KindTable
	// KindInlineTable is InlineTableBox: the box of display: inline-table.
	// It is a TableBox.
	KindInlineTable
	// KindTableRowGroup is TableRowGroupBox.
	KindTableRowGroup
	// KindTableRow is TableRowBox.
	KindTableRow
	// KindTableColumnGroup is TableColumnGroupBox.
	KindTableColumnGroup
	// KindTableColumn is TableColumnBox. It is not really a parent box, but
	// upstream treats it as one to remove some corner cases.
	KindTableColumn
	// KindTableCell is TableCellBox.
	KindTableCell
	// KindTableCaption is TableCaptionBox. It is a BlockBox.
	KindTableCaption
	// KindPage is PageBox: the box of a page. At first, the whole document
	// is in the box of the root element. The layout makes a new page box
	// after each page break.
	KindPage
	// KindMargin is MarginBox: a box in the margins of a page, as CSS 3
	// Paged Media defines it.
	KindMargin
	// KindFootnoteArea is FootnoteAreaBox: the box that shows the
	// footnotes, as GCPM defines it. It is a BlockBox.
	KindFootnoteArea
	// KindFlex is FlexBox: block-level on the outside and a flex container
	// on the inside.
	KindFlex
	// KindInlineFlex is InlineFlexBox: inline-level on the outside and a
	// flex container on the inside.
	KindInlineFlex
	// KindGrid is GridBox: block-level on the outside and a grid container
	// on the inside.
	KindGrid
	// KindInlineGrid is InlineGridBox: inline-level on the outside and a
	// grid container on the inside.
	KindInlineGrid

	// kindSize is not a class of upstream. It is the tuple (width, height)
	// that the layout passes in place of a box as a containing block. See
	// sizeBlock.
	kindSize
)

// kindNames are the names of the classes of upstream, without the suffix
// Box, as type(box).__name__[:-3] gives them.
var kindNames = [...]string{
	KindBlock:            "Block",
	KindLine:             "Line",
	KindInline:           "Inline",
	KindText:             "Text",
	KindInlineBlock:      "InlineBlock",
	KindBlockReplaced:    "BlockReplaced",
	KindInlineReplaced:   "InlineReplaced",
	KindTable:            "Table",
	KindInlineTable:      "InlineTable",
	KindTableRowGroup:    "TableRowGroup",
	KindTableRow:         "TableRow",
	KindTableColumnGroup: "TableColumnGroup",
	KindTableColumn:      "TableColumn",
	KindTableCell:        "TableCell",
	KindTableCaption:     "TableCaption",
	KindPage:             "Page",
	KindMargin:           "Margin",
	KindFootnoteArea:     "FootnoteArea",
	KindFlex:             "Flex",
	KindInlineFlex:       "InlineFlex",
	KindGrid:             "Grid",
	KindInlineGrid:       "InlineGrid",
}

// String returns the name of the class of upstream without the suffix
// Box, such as "InlineReplaced".
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "Unknown"
}

// StringSetEntry is an item of the attribute string_set of a box: the name
// of a named string and its value.
type StringSetEntry struct {
	Name  string
	Value string
}

// Box is a box of weasyprint/formatting_structure/boxes.py, of any class.
// Kind is its class. The fields are the attributes that upstream sets on
// the boxes. A field that only some classes use is described with the
// class.
//
// The layout of a later stage reads and sets the used values of the box
// model: the position, the size, the margins, the paddings and the borders.
//
// Port: the layout makes and copies millions of boxes for a large book. So
// Box holds only the attributes that most boxes use. The attributes of the
// boxes of tables are in tableFields, the attributes that few boxes set are
// in boxExtra, and the attributes of a flex item are in flexItem. A box
// allocates each of them when the layout first sets one of its fields. Until
// then, the box reads the default values of upstream.
type Box struct {
	// ElementTag is the tag of the element of the box, with the name of the
	// pseudo-element after "::" for a pseudo-element, such as "p::before".
	// It is "" for a box of a page, the None of upstream.
	ElementTag string
	// Element is the element of the box, or nil.
	Element *Element
	// Style is the computed style of the box.
	Style *Style
	// Children are the child boxes.
	Children []*Box

	// Text is the text of a text box.
	Text string
	// TextLayout is the layout of the text of a text box, set by the
	// layout of the lines. It is the pango_layout of upstream.
	TextLayout *TextLayout
	// JustificationSpacing is the spacing that justifies a text box.
	JustificationSpacing float64

	// Clearance is the clearance of a block-level box, set by the layout.
	// See HasClearance.
	Clearance float64
	// Baseline is the baseline of the box, set by the layout.
	Baseline float64

	// The used values of the box model, in pixels, set by the layout.
	PositionX, PositionY float64
	Width, Height        float64
	MarginTop            float64
	MarginRight          float64
	MarginBottom         float64
	MarginLeft           float64
	PaddingTop           float64
	PaddingRight         float64
	PaddingBottom        float64
	PaddingLeft          float64
	BorderTopWidth       float64
	BorderRightWidth     float64
	BorderBottomWidth    float64
	BorderLeftWidth      float64

	// The used values of min-width, max-width, min-height and max-height,
	// set by the layout. boxExtra holds the offsets of a positioned box.
	MinWidth, MaxWidth   float64
	MinHeight, MaxHeight float64

	// Background is the layout of the background of the box, or nil for
	// the None of upstream. layout/background.py sets it.
	Background *Background

	// textIndent is the used value of text-indent of a block container,
	// set by the layout of its lines.
	textIndent float64
	// index is the index of the box in the children of its parent, which
	// the layout sets on a laid out box.
	index int
	// resumeAt is the resume_at of a line box.
	resumeAt skipStack

	// radii are the radii (x, y) of the corners of the border, set by the
	// layout: top left, top right, bottom right and bottom left. nil is
	// four radii of 0.
	//
	// Port: upstream has the attributes border_top_left_radius and so on.
	// Most boxes have no radius, so the box only keeps a pointer. The
	// layout never changes the radii in place, so the copies of a box share
	// them.
	radii *[4][2]float64

	// removedSides is the set remove_decoration_sides of upstream: the
	// sides whose decoration the box removed. A copy of the box shares the
	// set, as in upstream. nil is an empty set that no other box shares.
	//
	// Port: upstream makes an empty set for each box. The port makes the
	// set when a box is copied or when it removes a side.
	removedSides *sideSet

	// ext holds the attributes that few boxes set, or nil. See extra.
	ext *boxExtra
	// tbl holds the attributes of the boxes of tables, or nil. See table.
	tbl *tableFields
	// flx holds the attributes that layout/flex.py sets on a flex item, or
	// nil. See flex.
	flx *flexItem

	// Kind is the class of the box.
	Kind Kind

	// LeadingCollapsibleSpace and TrailingCollapsibleSpace keep track of
	// the collapsing spaces that the processing of the white space removed,
	// for the opportunities to wrap.
	LeadingCollapsibleSpace  bool
	TrailingCollapsibleSpace bool

	// The flags that the building of the boxes sets on some boxes.
	IsTableWrapper   bool
	IsFlexItem       bool
	IsGridItem       bool
	IsForRootElement bool
	IsColumn         bool
	IsLeader         bool
	IsOutsideMarker  bool
	// ForceFragmentation forces the fragmentation of the box.
	ForceFragmentation bool

	// HasClearance is false for the None of upstream of Clearance.
	HasClearance bool

	// isTab marks the text box of one tab, which the layout places at the
	// tab stops of -ivy-tab-stops. boxExtra.tab is its stop. Upstream has
	// no tab stops.
	isTab bool

	// notFloated is set when the box is a flex or a grid item, whose method
	// is_floated upstream replaces by a function that returns False.
	notFloated bool

	// placeholder is the state of the placeholder of an absolute box. See
	// newAbsolutePlaceholder.
	placeholder placeholderState
	// bordersSet is set when the layout set the used widths of the
	// borders, for hasattr(box, 'border_left_width') of upstream.
	bordersSet bool
}

// boxExtra holds the attributes of a box that few boxes set. A box makes
// it when the layout first sets one of them. See Box.
type boxExtra struct {
	// FirstLetterStyle and FirstLineStyle are the styles of ::first-letter
	// and ::first-line of the element, or nil.
	FirstLetterStyle *Style
	FirstLineStyle   *Style

	// Replacement is the image of a replaced box.
	Replacement Image

	// Top, Right, Bottom and Left are the used values of the offsets of a
	// positioned box, set by the layout. An offset can be Auto.
	//
	// Port: upstream sets them as attributes of the box. Only the
	// positioned boxes have them, so the port keeps them here and the box
	// is smaller.
	Top, Right   float64
	Bottom, Left float64

	// TransformationMatrix is the transformation of the box, set by the
	// drawing, or nil.
	TransformationMatrix any
	// BookmarkLabel is the label of the bookmark of the box, and
	// HasBookmarkLabel is false for the None of upstream.
	BookmarkLabel    string
	HasBookmarkLabel bool
	// StringSet holds the named strings that the box sets, or nil.
	StringSet []StringSetEntry
	// Footnote is the footnote of a footnote call.
	Footnote *Box
	// Note is the note of a note call, NoteCall is the call of a note, and
	// NoteCallback is the callback of a note.
	Note         *Box
	NoteCall     *Box
	NoteCallback *Box
	// CachedCounterValues are the values of the counters when the box was
	// built, for the targets and the pages. nil is the None of upstream.
	CachedCounterValues map[string][]int
	// Link is the link of the box, as GetLink gives it, or nil.
	Link Value
	// MissingLink is the box whose counters are missing, set by
	// TargetCollector.CollectMissingCounters, or nil.
	MissingLink *Box

	// TextOverflow and BlockEllipsis are the overflow of the text of a line
	// box: "clip" and nil by default, the "none" of upstream.
	TextOverflow  string
	BlockEllipsis Value

	// tab is the stop of the text box of a tab. See Box.isTab.
	tab tabStop

	// pageOverflow is the descent of the last line of a page when that
	// line passes the bottom of the page, by -ivy-page-overflow. The
	// layout sets it on the line and on each block that ends with it, and
	// a parent subtracts it from the bottom of the block when it checks
	// the bottom of the page. See lineboxLayout.
	pageOverflow float64
	// lastCut is set on a line box that lost the extra space of 'last' of
	// -ivy-line-multiple. See lastLineSpace.
	lastCut bool
	// rowPart is set on a line box that is not the first part of its row
	// next to a float of -ivy-wrap-flow (D63). See iterLineBoxes.
	rowPart bool

	// Advancements is the advancements of a grid container. Upstream has a
	// TODO to move it into the context of the rendering.
	Advancements map[any]any
	// ViewportOverflow is the overflow of the viewport, set on the box of
	// the root element.
	ViewportOverflow string

	// PageType is the type of a page box.
	PageType PageType
	// AtKeyword is the at-keyword of a margin box, such as "@top-center".
	AtKeyword string
	// Page is the page of a footnote area box.
	Page *Box
	// CanvasBackground is the background of the canvas of a page box, or
	// nil. layout/background.py sets it.
	CanvasBackground *Background
	// fixedBoxes are the fixed boxes of a page box.
	fixedBoxes []*Box
	// isGenerated is set on a margin box that has content.
	isGenerated bool

	// BorderImage and MaskBorderImage are the images of border-image-source
	// and mask-border-source, or nil.
	BorderImage     Image
	MaskBorderImage Image
}

// defaultExtra holds the default values of boxExtra, which a box without
// one reads. Nothing writes it.
var defaultExtra = boxExtra{TextOverflow: "clip"}

// extra returns the attributes of the box that few boxes set, for reading
// only. A box that has none returns the default values.
func (b *Box) extra() *boxExtra {
	if b.ext != nil {
		return b.ext
	}
	return &defaultExtra
}

// ownExtra returns the attributes of the box that few boxes set, for
// writing. It makes them on the first call.
func (b *Box) ownExtra() *boxExtra {
	if b.ext == nil {
		e := defaultExtra
		b.ext = &e
	}
	return b.ext
}

// tableFields holds the attributes of the boxes of a table. A box makes it
// when the layout first sets one of them. See Box.
type tableFields struct {
	// Colspan and Rowspan are the spans of a table cell, 1 by default.
	Colspan int
	Rowspan int
	// GridX is the position of a cell, a column or a column group in the
	// grid of its table.
	GridX int
	// IsHeader and IsFooter mark the header and the footer row groups of a
	// table.
	IsHeader bool
	IsFooter bool
	// ColumnGroups are the column groups of a table box.
	ColumnGroups []*Box
	// ColumnPositions are the positions of the columns of a table box, set
	// by the layout.
	ColumnPositions []float64
	// CollapsedBorderGrid is the grid of the collapsed borders of a table
	// box, or nil.
	CollapsedBorderGrid *CollapsedBorderGrid
	// ColumnWidths are the widths of the columns of a table box, set by
	// the layout.
	ColumnWidths []float64
	// Empty is set on a table cell that has no in-flow child, for
	// empty-cells.
	Empty bool
	// SkipCellBorderTop and SkipCellBorderBottom are set on a table box
	// split across pages, whose cells of the first or the last row do not
	// draw their collapsed border on that side.
	SkipCellBorderTop    bool
	SkipCellBorderBottom bool
	// SkippedRows is the number of rows of a table box that the previous
	// pages hold.
	SkippedRows int
	// tableAttrs holds the attributes that layout/table.py sets on the
	// boxes of a table.
	tableAttrs
}

// defaultTable holds the default values of tableFields, which a box
// without one reads. Nothing writes it.
var defaultTable = tableFields{Colspan: 1, Rowspan: 1}

// table returns the attributes of the boxes of tables, for reading only. A
// box that has none returns the default values.
func (b *Box) table() *tableFields {
	if b.tbl != nil {
		return b.tbl
	}
	return &defaultTable
}

// ownTable returns the attributes of the boxes of tables, for writing. It
// makes them on the first call.
func (b *Box) ownTable() *tableFields {
	if b.tbl == nil {
		t := defaultTable
		b.tbl = &t
	}
	return b.tbl
}

// flex returns the attributes that layout/flex.py sets on a flex item. It
// makes them on the first call.
func (b *Box) flex() *flexItem {
	if b.flx == nil {
		b.flx = new(flexItem)
	}
	return b.flx
}

// sideSet is a set of the sides "top", "right", "bottom" and "left".
type sideSet uint8

// sideBit returns the bit of a side in a sideSet, or 0.
func sideBit(side string) sideSet {
	switch side {
	case "top":
		return 1
	case "right":
		return 2
	case "bottom":
		return 4
	case "left":
		return 8
	}
	return 0
}

// decorationRemoved reports whether the box removed the decoration of a
// side, as side in box.remove_decoration_sides of upstream.
func (b *Box) decorationRemoved(side string) bool {
	return b.removedSides != nil && *b.removedSides&sideBit(side) != 0
}

// shareRemovedSides makes the set of the removed sides of the box, so that
// a copy of the box shares it.
func (b *Box) shareRemovedSides() {
	if b.removedSides == nil {
		b.removedSides = new(sideSet)
	}
}

// newBox is Box.__init__ and the __init__ of the subclasses in
// weasyprint/formatting_structure/boxes.py. It makes a box of a kind.
// children are the children of a parent box, text is the text of a text
// box, and image is the replacement of a replaced box.
func newBox(kind Kind, elementTag string, style *Style, element *Element, children []*Box) *Box {
	return initBox(new(Box), kind, elementTag, style, element, children)
}

// initBox is newBox for the box b, which it makes new. It returns b.
func initBox(b *Box, kind Kind, elementTag string, style *Style, element *Element, children []*Box) *Box {
	*b = Box{
		Kind:       kind,
		ElementTag: elementTag,
		Element:    element,
		Style:      style,
		Children:   children,
	}
	switch kind {
	case KindTableCell:
		// HTML 4.01 gives special meaning to colspan=0
		// https://www.w3.org/TR/html401/struct/tables.html#adef-rowspan
		// but HTML 5 removed it
		// https://html.spec.whatwg.org/multipage/tables.html#attr-tdth-colspan
		// rowspan=0 is still there though.
		//
		// Port: the spans are clamped to the limits of HTML, 1000 columns
		// and 65534 rows. Upstream has no limit, and a hostile span makes a
		// grid larger than the memory.
		if element != nil {
			if colspan, ok := element.Get("colspan"); ok {
				if n, ok := parseInteger(colspan); ok {
					b.ownTable().Colspan = min(max(n, 1), 1000)
				}
			}
			if rowspan, ok := element.Get("rowspan"); ok {
				if n, ok := parseInteger(rowspan); ok {
					b.ownTable().Rowspan = min(max(n, 0), 65534)
				}
			}
		}
	case KindGrid, KindInlineGrid:
		// TODO: we shouldn’t store this in the box but in the rendering
		// context instead.
		b.ownExtra().Advancements = map[any]any{}
	}
	return b
}

// newTextBox is TextBox.__init__ in weasyprint/formatting_structure/
// boxes.py.
//
// Port: upstream asserts that the text is not empty. The callers of the
// port check it.
func newTextBox(elementTag string, style *Style, element *Element, text string) *Box {
	b := newBox(KindText, elementTag, style, element, nil)
	b.Text = text
	return b
}

// newReplacedBox is ReplacedBox.__init__ in weasyprint/formatting_structure/
// boxes.py, for a BlockReplacedBox or an InlineReplacedBox.
func newReplacedBox(kind Kind, elementTag string, style *Style, element *Element, replacement Image) *Box {
	b := newBox(kind, elementTag, style, element, nil)
	b.ownExtra().Replacement = replacement
	return b
}

// NewPageBox is PageBox.__init__ in weasyprint/formatting_structure/
// boxes.py. A page box is not linked to any element.
func NewPageBox(pageType PageType, style *Style) *Box {
	b := newBox(KindPage, "", style, nil, nil)
	b.ownExtra().PageType = pageType
	return b
}

// NewMarginBox is MarginBox.__init__ in weasyprint/formatting_structure/
// boxes.py. A margin box is not linked to any element.
func NewMarginBox(atKeyword string, style *Style) *Box {
	b := newBox(KindMargin, "", style, nil, nil)
	b.ownExtra().AtKeyword = atKeyword
	return b
}

// NewFootnoteAreaBox is FootnoteAreaBox.__init__ in
// weasyprint/formatting_structure/boxes.py. A footnote area box is not
// linked to any element.
func NewFootnoteAreaBox(page *Box, style *Style) *Box {
	b := newBox(KindFootnoteArea, "", style, nil, nil)
	b.ownExtra().Page = page
	return b
}

// anonymousFrom is the class method anonymous_from in
// weasyprint/formatting_structure/boxes.py. It returns an anonymous box of a
// kind that inherits from parent, with children.
func anonymousFrom(kind Kind, parent *Box, children []*Box) *Box {
	b := newBox(kind, parent.ElementTag, parent.Style.AnonymousStyle(), parent.Element, children)
	if kind == KindLine && !isStr(parent.Style.Get("overflow"), "visible") {
		// LineBox.anonymous_from.
		b.ownExtra().TextOverflow = str(parent.Style.Get("text_overflow"))
	}
	return b
}

// anonymousTextFrom is TextBox.anonymous_from in
// weasyprint/formatting_structure/boxes.py.
func anonymousTextFrom(parent *Box, text string) *Box {
	return newTextBox(parent.ElementTag, parent.Style.AnonymousStyle(), parent.Element, text)
}

// anonymousReplacedFrom is InlineReplacedBox.anonymous_from and
// BlockReplacedBox.anonymous_from in weasyprint/formatting_structure/
// boxes.py.
func anonymousReplacedFrom(kind Kind, parent *Box, image Image) *Box {
	return newReplacedBox(kind, parent.ElementTag, parent.Style.AnonymousStyle(), parent.Element, image)
}

// String is __repr__ in weasyprint/formatting_structure/boxes.py.
func (b *Box) String() string {
	switch b.Kind {
	case KindPage:
		return "<PageBox " + b.extra().PageType.key() + ">"
	case KindMargin:
		return "<MarginBox " + b.extra().AtKeyword + ">"
	case KindFootnoteArea:
		return "<FootnoteAreaBox @footnote>"
	}
	return "<" + b.Kind.String() + "Box " + b.ElementTag + ">"
}

// The abstract classes of upstream, and the classes that have subclasses.

// IsParent reports whether the box is a ParentBox: a box that has
// children.
func (b *Box) IsParent() bool {
	switch b.Kind {
	case KindText, KindBlockReplaced, KindInlineReplaced:
		return false
	}
	return true
}

// IsBlockLevel reports whether the box is a BlockLevelBox: a box that
// takes part in a block formatting context. An element with a display
// value of block, list-item or table generates a block-level box.
func (b *Box) IsBlockLevel() bool {
	switch b.Kind {
	case KindBlock, KindBlockReplaced, KindTable, KindInlineTable,
		KindTableCaption, KindFootnoteArea, KindFlex, KindGrid:
		return true
	}
	return false
}

// IsBlockContainer reports whether the box is a BlockContainerBox: a box
// that holds only block-level boxes, or that sets up an inline formatting
// context and so holds only line boxes. A non-replaced element with a
// display value of block, list-item, inline-block or table-cell generates a
// block container box.
func (b *Box) IsBlockContainer() bool {
	switch b.Kind {
	case KindBlock, KindInlineBlock, KindTableCell, KindTableCaption,
		KindMargin, KindFootnoteArea:
		return true
	}
	return false
}

// IsBlockBox reports whether the box is a BlockBox, or one of its
// subclasses TableCaptionBox and FootnoteAreaBox.
func (b *Box) IsBlockBox() bool {
	switch b.Kind {
	case KindBlock, KindTableCaption, KindFootnoteArea:
		return true
	}
	return false
}

// IsInlineLevel reports whether the box is an InlineLevelBox: a box that
// takes part in an inline formatting context. An inline-level box that is
// not an inline box is atomic: an inline block, a replaced element or an
// inline table. An element with a display value of inline, inline-table or
// inline-block generates an inline-level box.
func (b *Box) IsInlineLevel() bool {
	switch b.Kind {
	case KindInline, KindText, KindInlineBlock, KindInlineReplaced,
		KindInlineFlex, KindInlineGrid:
		return true
	}
	return false
}

// IsAtomicInlineLevel reports whether the box is an AtomicInlineLevelBox:
// an atomic box in an inline formatting context, which cannot be split
// for line breaks.
func (b *Box) IsAtomicInlineLevel() bool {
	return b.Kind == KindInlineBlock || b.Kind == KindInlineReplaced
}

// IsReplaced reports whether the box is a ReplacedBox: a box whose content
// is replaced. For example, <img> is replaced: its content is drawn
// outside of CSS and is opaque to it.
func (b *Box) IsReplaced() bool {
	return b.Kind == KindBlockReplaced || b.Kind == KindInlineReplaced
}

// IsTable reports whether the box is a TableBox, or an InlineTableBox.
func (b *Box) IsTable() bool {
	return b.Kind == KindTable || b.Kind == KindInlineTable
}

// IsFlexContainer reports whether the box is a FlexContainerBox: a box that
// holds only flex items.
func (b *Box) IsFlexContainer() bool {
	return b.Kind == KindFlex || b.Kind == KindInlineFlex
}

// IsGridContainer reports whether the box is a GridContainerBox: a box
// that holds only grid items.
func (b *Box) IsGridContainer() bool {
	return b.Kind == KindGrid || b.Kind == KindInlineGrid
}

// Is reports whether the box is an instance of the class of kind, as
// isinstance does: KindBlock holds the subclasses of BlockBox, and
// KindTable holds InlineTableBox.
func (b *Box) Is(kind Kind) bool {
	switch kind {
	case KindBlock:
		return b.IsBlockBox()
	case KindTable:
		return b.IsTable()
	}
	return b.Kind == kind
}

// The class attributes of upstream for the anonymous boxes of the tables.
// See https://www.w3.org/TR/CSS21/tables.html#anonymous-boxes.

// properTableChild is the attribute proper_table_child.
func (b *Box) properTableChild() bool {
	switch b.Kind {
	case KindTableRowGroup, KindTableRow, KindTableColumnGroup,
		KindTableColumn, KindTableCaption:
		return true
	}
	return false
}

// internalTableOrCaption is the attribute internal_table_or_caption.
func (b *Box) internalTableOrCaption() bool {
	switch b.Kind {
	case KindTableRowGroup, KindTableRow, KindTableColumnGroup,
		KindTableColumn, KindTableCell, KindTableCaption:
		return true
	}
	return false
}

// tabularContainer is the attribute tabular_container.
func (b *Box) tabularContainer() bool {
	switch b.Kind {
	case KindTable, KindInlineTable, KindTableRowGroup, KindTableRow:
		return true
	}
	return false
}

// hasProperParent reports whether parentKind is in the attribute
// proper_parents of a proper table child. upstream compares the type of the
// parent, so a subclass of a proper parent is not one.
func (b *Box) hasProperParent(parentKind Kind) bool {
	switch b.Kind {
	case KindTableRowGroup, KindTableColumnGroup, KindTableCaption:
		return parentKind == KindTable || parentKind == KindInlineTable
	case KindTableRow:
		return parentKind == KindTable || parentKind == KindInlineTable || parentKind == KindTableRowGroup
	case KindTableColumn:
		return parentKind == KindTable || parentKind == KindInlineTable || parentKind == KindTableColumnGroup
	}
	return false
}

// Copy is copy in weasyprint/formatting_structure/boxes.py. It returns a
// shallow copy of the box.
//
// Port: the copy shares the set of the removed sides, as in upstream, and
// has its own copy of the attributes of boxExtra, tableFields and flexItem.
func (b *Box) Copy() *Box {
	c := new(Box)
	c.copyFrom(b)
	return c
}

// copyFrom makes the box a shallow copy of src, as Copy does.
func (b *Box) copyFrom(src *Box) {
	src.shareRemovedSides()
	*b = *src
	if src.ext != nil {
		e := *src.ext
		b.ext = &e
	}
	if src.tbl != nil {
		t := *src.tbl
		b.tbl = &t
	}
	if src.flx != nil {
		f := *src.flx
		b.flx = &f
	}
}

// DeepCopy is deepcopy in weasyprint/formatting_structure/boxes.py. It
// returns a copy of the box with recursive copies of its children.
func (b *Box) DeepCopy() *Box {
	c := b.Copy()
	if b.IsParent() {
		c.Children = make([]*Box, len(b.Children))
		for i, child := range b.Children {
			c.Children[i] = child.DeepCopy()
		}
	}
	return c
}

// AllChildren is all_children in weasyprint/formatting_structure/boxes.py:
// the children, and the column groups of a table.
func (b *Box) AllChildren() []*Box {
	if b.IsTable() {
		all := make([]*Box, 0, len(b.Children)+len(b.table().ColumnGroups))
		all = append(all, b.Children...)
		return append(all, b.table().ColumnGroups...)
	}
	return b.Children
}

// InFlowChildren is the property in_flow_children in
// weasyprint/formatting_structure/boxes.py.
func (b *Box) InFlowChildren() []*Box {
	var children []*Box
	for _, child := range b.Children {
		if child.IsInNormalFlow() {
			children = append(children, child)
		}
	}
	return children
}

// InFlowOrFloatedChildren is the property in_flow_or_floated_children in
// weasyprint/formatting_structure/boxes.py.
func (b *Box) InFlowOrFloatedChildren() []*Box {
	var children []*Box
	for _, child := range b.Children {
		if child.IsInNormalFlow() || child.IsFloated() {
			children = append(children, child)
		}
	}
	return children
}

// Descendants is descendants in weasyprint/formatting_structure/boxes.py:
// the box, its children and their descendants, depth first. A placeholder
// of an absolute box is a box among the children: Descendants yields it,
// and not its descendants, as upstream does by default.
func (b *Box) Descendants(yield func(*Box) bool) bool {
	return b.descendants(false, yield)
}

// descendants is descendants(placeholders) in
// weasyprint/formatting_structure/boxes.py. With placeholders, it also
// yields the descendants of the placeholders of the absolute boxes.
func (b *Box) descendants(placeholders bool, yield func(*Box) bool) bool {
	if !yield(b) {
		return false
	}
	for _, child := range b.Children {
		if placeholders || !child.IsPlaceholder() {
			if !child.descendants(placeholders, yield) {
				return false
			}
		} else if !yield(child) {
			return false
		}
	}
	return true
}

// descendantsList returns the descendants of the box in a slice.
func (b *Box) descendantsList() []*Box {
	var list []*Box
	b.Descendants(func(d *Box) bool {
		list = append(list, d)
		return true
	})
	return list
}

// Translate is translate in weasyprint/formatting_structure/boxes.py. It
// changes the position of the box and of its children.
//
// A placeholder whose box is not laid out yet only moves its own position,
// as AbsolutePlaceholder.translate in weasyprint/layout/absolute.py does:
// its descendants do not have a position yet.
func (b *Box) Translate(dx, dy float64, ignoreFloats bool) {
	if dx == 0 && dy == 0 {
		return
	}
	if b.placeholder == placeholderPending {
		b.PositionX += dx
		b.PositionY += dy
		return
	}
	if b.IsTable() {
		positions := b.table().ColumnPositions
		for i := range positions {
			positions[i] += dx
		}
	}
	b.PositionX += dx
	b.PositionY += dy
	for _, child := range b.AllChildren() {
		if !ignoreFloats || !child.IsFloated() {
			child.Translate(dx, dy, ignoreFloats)
		}
	}
}

// The heights and the widths.

// PaddingWidth is padding_width: the width of the padding box.
func (b *Box) PaddingWidth() float64 {
	return b.Width + b.PaddingLeft + b.PaddingRight
}

// PaddingHeight is padding_height: the height of the padding box.
func (b *Box) PaddingHeight() float64 {
	return b.Height + b.PaddingTop + b.PaddingBottom
}

// BorderWidth is border_width: the width of the border box.
func (b *Box) BorderWidth() float64 {
	return b.PaddingWidth() + b.BorderLeftWidth + b.BorderRightWidth
}

// BorderHeight is border_height: the height of the border box.
func (b *Box) BorderHeight() float64 {
	return b.PaddingHeight() + b.BorderTopWidth + b.BorderBottomWidth
}

// MarginWidth is margin_width: the width of the margin box, the outer box.
func (b *Box) MarginWidth() float64 {
	return b.BorderWidth() + b.MarginLeft + b.MarginRight
}

// MarginHeight is margin_height: the height of the margin box, the outer
// box.
func (b *Box) MarginHeight() float64 {
	return b.BorderHeight() + b.MarginTop + b.MarginBottom
}

// The positions of the corners.

// ContentBoxX is content_box_x: the absolute horizontal position of the
// content box.
func (b *Box) ContentBoxX() float64 {
	return b.PositionX + b.MarginLeft + b.PaddingLeft + b.BorderLeftWidth
}

// ContentBoxY is content_box_y: the absolute vertical position of the
// content box.
func (b *Box) ContentBoxY() float64 {
	return b.PositionY + b.MarginTop + b.PaddingTop + b.BorderTopWidth
}

// PaddingBoxX is padding_box_x: the absolute horizontal position of the
// padding box.
func (b *Box) PaddingBoxX() float64 {
	return b.PositionX + b.MarginLeft + b.BorderLeftWidth
}

// PaddingBoxY is padding_box_y: the absolute vertical position of the
// padding box.
func (b *Box) PaddingBoxY() float64 {
	return b.PositionY + b.MarginTop + b.BorderTopWidth
}

// BorderBoxX is border_box_x: the absolute horizontal position of the
// border box.
func (b *Box) BorderBoxX() float64 {
	return b.PositionX + b.MarginLeft
}

// BorderBoxY is border_box_y: the absolute vertical position of the border
// box.
func (b *Box) BorderBoxY() float64 {
	return b.PositionY + b.MarginTop
}

// HitArea is hit_area: the rectangle (x, y, w, h) where the box is
// clickable. An inline box uses its line height.
func (b *Box) HitArea() [4]float64 {
	if b.Kind == KindInline {
		// Use line-height (margin_height) rather than border_height
		return [4]float64{b.BorderBoxX(), b.PositionY, b.BorderWidth(), b.MarginHeight()}
	}
	// "Border area. That's the area that hit-testing is done on."
	// https://lists.w3.org/Archives/Public/www-style/2012Jun/0318.html
	// TODO: manage the border radii, use outer_border_radii instead
	return [4]float64{b.BorderBoxX(), b.BorderBoxY(), b.BorderWidth(), b.BorderHeight()}
}

// RoundedBox is the result of rounded_box: the position, the size and the
// radii of a box inside the outer border box.
type RoundedBox struct {
	X, Y, Width, Height float64
	// The radii (x, y) of the corners.
	TopLeft, TopRight, BottomRight, BottomLeft [2]float64
}

// RoundedBox is rounded_box in weasyprint/formatting_structure/boxes.py.
// bt, br, bb and bl are the distances from the outer border box that define
// the rectangle to round.
func (b *Box) RoundedBox(bt, br, bb, bl float64) RoundedBox {
	var radii [4][2]float64
	if b.radii != nil {
		radii = *b.radii
	}
	tlrx, tlry := radii[0][0], radii[0][1]
	trrx, trry := radii[1][0], radii[1][1]
	brrx, brry := radii[2][0], radii[2][1]
	blrx, blry := radii[3][0], radii[3][1]

	// TODO: clamp all computed values, see #2705.
	clamp := func(v float64) float64 { return min(max(0, v), math.MaxInt64) }
	tlrx = clamp(tlrx - bl)
	tlry = clamp(tlry - bt)
	trrx = clamp(trrx - br)
	trry = clamp(trry - bt)
	brrx = clamp(brrx - br)
	brry = clamp(brry - bb)
	blrx = clamp(blrx - bl)
	blry = clamp(blry - bb)

	x := b.BorderBoxX() + bl
	y := b.BorderBoxY() + bt
	width := b.BorderWidth() - bl - br
	height := b.BorderHeight() - bt - bb

	// Fix overlapping curves
	// See https://www.w3.org/TR/css-backgrounds-3/#corner-overlap
	ratio := 1.0
	for _, pair := range [][2]float64{
		{width, tlrx + trrx},
		{width, blrx + brrx},
		{height, tlry + blry},
		{height, trry + brry},
	} {
		if pair[1] > 0 {
			ratio = min(ratio, pair[0]/pair[1])
		}
	}
	return RoundedBox{
		X: x, Y: y, Width: width, Height: height,
		TopLeft:     [2]float64{tlrx * ratio, tlry * ratio},
		TopRight:    [2]float64{trrx * ratio, trry * ratio},
		BottomRight: [2]float64{brrx * ratio, brry * ratio},
		BottomLeft:  [2]float64{blrx * ratio, blry * ratio},
	}
}

// RoundedBoxRatio is rounded_box_ratio in weasyprint/formatting_structure/
// boxes.py.
func (b *Box) RoundedBoxRatio(ratio float64) RoundedBox {
	return b.RoundedBox(
		b.BorderTopWidth*ratio, b.BorderRightWidth*ratio,
		b.BorderBottomWidth*ratio, b.BorderLeftWidth*ratio)
}

// RoundedPaddingBox is rounded_padding_box: the position, the size and the
// radii of the rounded padding box.
func (b *Box) RoundedPaddingBox() RoundedBox {
	return b.RoundedBox(b.BorderTopWidth, b.BorderRightWidth, b.BorderBottomWidth, b.BorderLeftWidth)
}

// RoundedBorderBox is rounded_border_box: the position, the size and the
// radii of the rounded border box.
func (b *Box) RoundedBorderBox() RoundedBox {
	return b.RoundedBox(0, 0, 0, 0)
}

// RoundedContentBox is rounded_content_box: the position, the size and the
// radii of the rounded content box.
func (b *Box) RoundedContentBox() RoundedBox {
	return b.RoundedBox(
		b.BorderTopWidth+b.PaddingTop, b.BorderRightWidth+b.PaddingRight,
		b.BorderBottomWidth+b.PaddingBottom, b.BorderLeftWidth+b.PaddingLeft)
}

// The positioning schemes.

// positionFunction returns the name of the function of a value of position,
// such as "running()", or "" for a keyword. Upstream reads
// style['position'][0], which is the first character of a keyword and never
// the name of a function.
func positionFunction(position Value) string {
	if t, ok := position.(Tuple); ok && len(t) > 0 {
		return str(t[0])
	}
	return ""
}

// The flags of flowFlags.
const (
	flowKnown    uint8 = 1 << iota // the other flags are set
	flowFloated                    // float is left, right, inline-start or inline-end
	flowFootnote                   // float is footnote
	flowNote                       // position is note()
	flowAbsolute                   // position is absolute or fixed
	flowRunning                    // position is running()
)

// flowFlags returns the flags that float and position of a style give to
// the predicates of the boxes, such as IsFloated.
//
// Port: upstream reads float and position for each call. The style keeps
// the flags, which the layout reads many times for each box.
func flowFlags(s *Style) uint8 {
	if s.flow != 0 {
		return s.flow
	}
	flags := flowKnown
	float := s.Get("float")
	if isStrIn(float, "left", "right", "inline-start", "inline-end") {
		flags |= flowFloated
	}
	if isStr(float, "footnote") {
		flags |= flowFootnote
	}
	position := s.Get("position")
	switch positionFunction(position) {
	case "note()":
		flags |= flowNote
	case "running()":
		flags |= flowRunning
	}
	if isStrIn(position, "absolute", "fixed") {
		flags |= flowAbsolute
	}
	s.flow = flags
	return flags
}

// IsFloated is is_floated: it reports whether the box is floated.
func (b *Box) IsFloated() bool {
	if b.notFloated {
		return false
	}
	return flowFlags(b.Style)&flowFloated != 0
}

// IsFootnote is is_footnote: it reports whether the box is a footnote.
func (b *Box) IsFootnote() bool {
	return flowFlags(b.Style)&flowFootnote != 0
}

// IsNote is is_note: it reports whether the box is a note.
func (b *Box) IsNote() bool {
	return flowFlags(b.Style)&flowNote != 0
}

// IsAbsolutelyPositioned is is_absolutely_positioned: it reports whether
// the box is in the absolute positioning scheme.
func (b *Box) IsAbsolutelyPositioned() bool {
	return flowFlags(b.Style)&flowAbsolute != 0
}

// IsRunning is is_running: it reports whether the box is a running element.
func (b *Box) IsRunning() bool {
	return flowFlags(b.Style)&flowRunning != 0
}

// IsInNormalFlow is is_in_normal_flow: it reports whether the box is in the
// normal flow.
func (b *Box) IsInNormalFlow() bool {
	return !b.IsFloated() && !b.IsAbsolutelyPositioned() &&
		!b.IsRunning() && !b.IsFootnote() && !b.IsNote()
}

// IsMonolithic is is_monolithic: it reports whether the box is monolithic.
// See https://www.w3.org/TR/css-break-3/#monolithic.
func (b *Box) IsMonolithic() bool {
	overflow := b.Style.Get("overflow")
	return b.IsAtomicInlineLevel() || b.IsReplaced() ||
		isStrIn(overflow, "auto", "scroll") ||
		(isStr(overflow, "hidden") && !isStr(b.Style.Get("height"), "auto"))
}

// EstablishesFormattingContext is establishes_formatting_context: it
// reports whether the box sets up a block formatting context. See
// https://www.w3.org/TR/CSS2/visuren.html#block-formatting.
func (b *Box) EstablishesFormattingContext() bool {
	return b.IsFloated() ||
		b.IsAbsolutelyPositioned() ||
		b.IsColumn ||
		(b.IsBlockContainer() && !b.IsBlockBox()) ||
		(b.IsBlockBox() && !isStr(b.Style.Get("overflow"), "visible")) ||
		tupleHas(b.Style.Get("display"), "flow-root")
}

// PageValues is page_values in weasyprint/formatting_structure/boxes.py:
// the start and the end page values of the box, for the named pages.
func (b *Box) PageValues() (start, end Value) {
	start, end = b.Style.Get("page"), b.Style.Get("page")
	if !b.IsParent() || b.IsTable() {
		return start, end
	}
	// ParentBox.page_values.
	// See https://www.w3.org/TR/css-break-4/#break-propagation.
	// The specification only includes in-flow boxes, but as page breaks
	// can appear between floats, it makes sense to include them here too.
	//
	// Port: upstream makes the list of these children. The port finds the
	// first and the last one.
	var first, last *Box
	for _, child := range b.Children {
		if child.IsInNormalFlow() || child.IsFloated() {
			if first == nil {
				first = child
			}
			last = child
		}
	}
	switch first {
	case nil:
	case last:
		childStart, childEnd := first.PageValues()
		start = pyOr(childStart, start)
		end = pyOr(childEnd, end)
	default:
		childStart, _ := first.PageValues()
		_, childEnd := last.PageValues()
		start = pyOr(childStart, start)
		end = pyOr(childEnd, end)
	}
	return start, end
}

// pyOr is the operator or of Python for two values.
func pyOr(a, b Value) Value {
	if truthy(a) {
		return a
	}
	return b
}

// IsAttachment is is_attachment: it reports whether the link of the box is
// stored as an attachment of the PDF.
func (b *Box) IsAttachment() bool {
	if b.Element != nil && b.Element.Tag == "a" {
		return elementHasLinkType(b.Element, "attachment")
	}
	return false
}

// IsInput is is_input: it reports whether the box is an input of a form.
// See https://html.spec.whatwg.org/multipage/forms.html#category-submit.
func (b *Box) IsInput() bool {
	if isStr(b.Style.Get("appearance"), "auto") && b.Element != nil {
		switch b.Element.Tag {
		case "button", "input", "select", "textarea":
			return b.Kind != KindLine && b.Kind != KindText
		}
	}
	return false
}

// IsForm is is_form: it reports whether the box is a form element.
func (b *Box) IsForm() bool {
	return b.Element != nil && b.Element.Tag == "form"
}

// resetSpacing is _reset_spacing: it sets the margin, the padding and the
// border of a side to 0.
func (b *Box) resetSpacing(side string) {
	if b.removedSides == nil {
		b.removedSides = new(sideSet)
	}
	*b.removedSides |= sideBit(side)
	switch side {
	case "top":
		b.MarginTop, b.PaddingTop, b.BorderTopWidth = 0, 0, 0
	case "right":
		b.MarginRight, b.PaddingRight, b.BorderRightWidth = 0, 0, 0
	case "bottom":
		b.MarginBottom, b.PaddingBottom, b.BorderBottomWidth = 0, 0, 0
	case "left":
		b.MarginLeft, b.PaddingLeft, b.BorderLeftWidth = 0, 0, 0
	}
}

// RemoveDecoration is remove_decoration of ParentBox and InlineLevelBox in
// weasyprint/formatting_structure/boxes.py. It removes the decoration of
// the start and of the end of a box that is split.
func (b *Box) RemoveDecoration(start, end bool) {
	if isStr(b.Style.Get("box_decoration_break"), "clone") {
		return
	}
	if b.IsInlineLevel() {
		ltr := isStr(b.Style.Get("direction"), "ltr")
		if start {
			if ltr {
				b.resetSpacing("left")
			} else {
				b.resetSpacing("right")
			}
		}
		if end {
			if ltr {
				b.resetSpacing("right")
			} else {
				b.resetSpacing("left")
			}
		}
		return
	}
	if start {
		b.resetSpacing("top")
	}
	if end {
		b.resetSpacing("bottom")
	}
}

// CopyWithChildren is copy_with_children in weasyprint/formatting_structure/
// boxes.py. It returns an equivalent box with new children.
func (b *Box) CopyWithChildren(newChildren []*Box) *Box {
	c := b.Copy()
	c.Children = newChildren
	// Clear and reset removed decorations as we don't want to keep the
	// previous data, for example when a box is split between two pages.
	b.removedSides = nil
	return c
}

// CopyWithText is copy_with_text in weasyprint/formatting_structure/
// boxes.py. It returns a text box like this one, with another text.
func (b *Box) CopyWithText(text string) *Box {
	c := b.Copy()
	c.Text = text
	return c
}

// GetWrappedTable is get_wrapped_table in weasyprint/formatting_structure/
// boxes.py. It returns the table that a table wrapper wraps, or nil.
func (b *Box) GetWrappedTable() *Box {
	for _, child := range b.Children {
		if child.IsTable() {
			return child
		}
	}
	return nil
}

// TopMarginCollapses is top_margin_collapses in
// weasyprint/formatting_structure/boxes.py.
func (b *Box) TopMarginCollapses() bool {
	return !(b.BorderTopWidth != 0 || b.PaddingTop != 0 ||
		b.IsFlexItem || b.IsGridItem ||
		b.EstablishesFormattingContext() ||
		b.IsTableWrapper ||
		b.IsForRootElement)
}

// BottomMarginCollapses is bottom_margin_collapses in
// weasyprint/formatting_structure/boxes.py.
func (b *Box) BottomMarginCollapses() bool {
	return !(b.BorderBottomWidth != 0 || b.PaddingBottom != 0 ||
		b.IsFlexItem || b.IsGridItem ||
		b.EstablishesFormattingContext() ||
		b.IsTableWrapper ||
		b.IsForRootElement)
}

// Span is the property span of TableColumnGroupBox and TableColumnBox in
// weasyprint/formatting_structure/boxes.py: the number of columns of a
// column or a column group, at least 1.
func (b *Box) Span() int {
	if b.Kind == KindTableColumnGroup && len(b.Children) > 0 {
		return len(b.Children)
	}
	if b.Element == nil {
		return 1
	}
	if span, ok := parseInteger(b.Element.GetDefault("span", "")); ok {
		// Port: Python has integers of any size. The span is bounded, so
		// that the columns of a hostile document stay few.
		return min(max(span, 1), maxSpan)
	}
	return 1
}

// maxSpan bounds the span of a column and of a column group, as HTML bounds
// it.
//
// Port: upstream has no bound. A span of a billion columns makes a billion
// boxes.
const maxSpan = 1000

// GetCells is get_cells of TableColumnGroupBox in
// weasyprint/formatting_structure/boxes.py: the cells that start in the
// columns of the group. The cells of a column are set by the layout.
func (b *Box) GetCells(cellsOf func(column *Box) []*Box) []*Box {
	cells := make([]*Box, 0, len(b.Children))
	for _, column := range b.Children {
		cells = append(cells, cellsOf(column)...)
	}
	return cells
}

// Bleed is the property bleed of PageBox in weasyprint/formatting_structure/
// boxes.py: the bleed of each side of a page, by side.
func (b *Box) Bleed() map[string]float64 {
	bleed := map[string]float64{}
	for _, side := range [...]string{"top", "right", "bottom", "left"} {
		if d, ok := b.Style.Get("bleed_" + side).(Dimension); ok {
			bleed[side] = d.Value
		}
	}
	return bleed
}

// BleedArea is the property bleed_area of PageBox in
// weasyprint/formatting_structure/boxes.py.
func (b *Box) BleedArea() [4]float64 {
	bleed := b.Bleed()
	return [4]float64{
		-bleed["left"], -bleed["top"],
		b.MarginWidth() + bleed["left"] + bleed["right"],
		b.MarginHeight() + bleed["top"] + bleed["bottom"],
	}
}

// The text.

// ContentText is the property content_text in
// weasyprint/formatting_structure/boxes.py. It is the text of the box with
// its white space processed and stripped, without the text of ::before,
// ::after and ::marker.
func (b *Box) ContentText() string {
	switch {
	case b.Kind == KindText:
		// Stripping may not be the "right" way, but it seems to be what
		// users usually want in this case. The specification asks for the
		// "text content", probably as defined in DOM.
		box := b.DeepCopy()
		box.ProcessWhitespace(false)
		return strings.TrimFunc(box.Text, pyIsSpace)
	case b.IsParent():
		box := b.DeepCopy()
		box.ProcessWhitespace(false)
		var text strings.Builder
		box.Descendants(func(child *Box) bool {
			if !strings.HasSuffix(child.ElementTag, "::before") &&
				!strings.HasSuffix(child.ElementTag, "::after") &&
				!strings.HasSuffix(child.ElementTag, "::marker") &&
				child.Kind == KindText {
				text.WriteString(child.Text)
			}
			return true
		})
		return strings.TrimFunc(text.String(), pyIsSpace)
	}
	return ""
}

// ProcessWhitespace is process_whitespace in
// weasyprint/formatting_structure/boxes.py: the first part of the
// processing model of white-space. It returns whether a collapsible space
// follows. See https://www.w3.org/TR/CSS21/text.html#white-space-model and
// https://drafts.csswg.org/css-text-3/#white-space-rules.
func (b *Box) ProcessWhitespace(followingCollapsibleSpace bool) bool {
	if b.Kind == KindText {
		return b.processTextWhitespace(followingCollapsibleSpace)
	}
	for _, child := range b.Children {
		childCollapsibleSpace := child.ProcessWhitespace(followingCollapsibleSpace)
		if child.Kind == KindText || child.Kind == KindInline {
			followingCollapsibleSpace = childCollapsibleSpace
		} else if child.IsInNormalFlow() {
			followingCollapsibleSpace = false
		}
	}
	return followingCollapsibleSpace
}

// processTextWhitespace is TextBox.process_whitespace.
func (b *Box) processTextWhitespace(followingCollapsibleSpace bool) bool {
	text := b.Text
	if text == "" {
		return followingCollapsibleSpace
	}

	// Normalize line feeds
	text = normalizeLineFeeds(text)

	whiteSpace := b.Style.Get("white_space")
	newLineCollapse := isStrIn(whiteSpace, "normal", "nowrap")
	spaceCollapse := isStrIn(whiteSpace, "normal", "nowrap", "pre-line")

	switch {
	case newLineCollapse:
		// \r characters were removed/converted earlier.
		//
		// TODO: this should be language-specific.
		// Could also replace with a zero width space character (U+200B),
		// or no character.
		// CSS3: https://www.w3.org/TR/css-text-3/#overflow-wrap
		//
		// Port: upstream removes the spaces around the line feeds, replaces
		// the line feeds with spaces and collapses the spaces, each in a
		// pass of its own. collapseSpaces does the three in one pass.
		//
		// Port: a line feed between two characters of Chinese or
		// Japanese goes, as CSS Text 3 says. With white-space normal, a
		// line feed that ideographic spaces follow stays, so that a
		// paragraph of Chinese or Japanese text that starts with them
		// starts a new line (D68).
		if isStr(whiteSpace, "normal") {
			text = collapseKeepingIdeographicBreaks(removeWideSegmentBreaks(text, true))
		} else {
			text = collapseSpaces(removeWideSegmentBreaks(text, false), true)
		}
	case spaceCollapse:
		// \r characters were removed/converted earlier.
		text = collapseSpaces(trimLineFeedBlanks(text), false)
	}

	if spaceCollapse {
		previousText := text
		if followingCollapsibleSpace && strings.HasPrefix(text, " ") {
			text = text[1:]
			b.LeadingCollapsibleSpace = true
		}
		followingCollapsibleSpace = strings.HasSuffix(previousText, " ")
	} else {
		followingCollapsibleSpace = false
	}

	b.Text = text
	return followingCollapsibleSpace
}

// ProcessTextTransform is process_text_transform in
// weasyprint/formatting_structure/boxes.py.
func (b *Box) ProcessTextTransform() {
	if b.Kind == KindText {
		textTransform := b.Style.Get("text_transform")
		lang, _ := b.Style.Get("lang").(String)
		langCode, _, _ := strings.Cut(string(lang), "-")
		langCode = strings.ToLower(langCode)
		if !isStr(textTransform, "none") {
			b.Text = transform(str(textTransform), b.Text, langCode)
		}
		if isStr(b.Style.Get("hyphens"), "none") {
			b.Text = strings.ReplaceAll(b.Text, "\u00ad", "") // U+00AD is soft hyphen
		}
		return
	}
	if !b.IsRunning() && !b.IsFootnote() && !b.IsNote() {
		for _, child := range b.Children {
			child.ProcessTextTransform()
		}
	}
}

// IsWhitespace is is_whitespace: it reports whether the box is a text box
// with only white space.
func (b *Box) IsWhitespace() bool {
	if b.Kind != KindText {
		return false
	}
	return strings.IndexFunc(b.Text, func(r rune) bool { return !pyIsSpace(r) }) < 0
}

// ExtractText is extract_text in weasyprint/formatting_structure/boxes.py:
// the part of the text of the box that target-text() and content() name:
// "text", "content", "before", "after" or "first-letter".
func (b *Box) ExtractText(textPart string) string {
	switch textPart {
	case "text", "content":
		return b.ContentText()
	case "before", "after":
		var text strings.Builder
		b.Descendants(func(child *Box) bool {
			if strings.HasSuffix(child.ElementTag, "::"+textPart) && !child.IsParent() {
				text.WriteString(child.ContentText())
			}
			return true
		})
		return text.String()
	case "first-letter":
		return getFirstLetter(b.ContentText())
	}
	return ""
}

// pyIsSpace reports whether r is white space, as str.isspace of Python
// says.
func pyIsSpace(r rune) bool {
	switch {
	case r >= 0x1c && r <= 0x1f:
		return true
	case unicode.IsSpace(r):
		return true
	}
	return r == 0x2028 || r == 0x2029
}

// copyCounterValues returns a copy of counter values, as {key: value.copy()
// for key, value in counter_values.items()} does.
func copyCounterValues(counterValues map[string][]int) map[string][]int {
	c := make(map[string][]int, len(counterValues))
	for key, value := range counterValues {
		c[key] = append([]int(nil), value...)
	}
	return c
}

// cloneCounterValues returns a shallow copy of counter values, as
// counters.copy() does: the lists are shared.
func cloneCounterValues(counterValues map[string][]int) map[string][]int {
	return maps.Clone(counterValues)
}
