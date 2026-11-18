// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/__init__.py: it transforms a "before
// layout" box tree into an "after layout" tree. It breaks boxes across lines
// and pages, and determines the size and the dimension of each box fragment.
//
// Boxes in the new tree have used values in their fields PositionX,
// PositionY, Width and Height, among others.
//
// See https://www.w3.org/TR/CSS21/cascade.html#used-value
//
// The file also holds the types that the files of layout/ share: Auto, the
// skip stacks, the next page and the result of the layout of a block-level
// box.
//
// Port: layout_document is the type Document, which lays out the pages
// when a program asks for them. See Document.

package layout

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/xo/okage/tinycss2"

	"github.com/xo/ivy"
)

// Auto is the value auto of a used value of the box model, such as the
// width of a box before the layout resolves it. Upstream keeps the string
// 'auto' in the attribute. The port keeps Auto in the float64 field.
// Compare a field with Auto with ==.
//
// Port: Auto is negative infinity, which no used value of upstream takes.
// The layout resolves each auto value that it reads, so the boxes that it
// returns hold no Auto, except in the fields Top, Right, Bottom and Left.
var Auto = math.Inf(-1)

// usedValue converts the result of percentage, a Float, String("auto") or
// nil, into the value of a field: Auto for auto, and 0 for None.
func usedValue(v Value) float64 {
	switch v := v.(type) {
	case Float:
		return float64(v)
	case Int:
		return float64(v)
	case String:
		if v == "auto" {
			return Auto
		}
	}
	return 0
}

// skipStack is a skip stack, or a resume_at, of upstream: the dict that maps
// the index of a child to the skip stack of that child. A nil skipStack is
// None. The skip stack of a text box maps the number of UTF-8 bytes to skip
// to nil.
type skipStack map[int]skipStack

// item returns the item of a skip stack of one item, as
// `(index, child_stack), = skip_stack.items()` does.
//
// Port: upstream fails to unpack a skip stack of more items. The port takes
// the smallest index, so that the result does not depend on the order of
// the map.
func (s skipStack) item() (int, skipStack) {
	first := true
	var index int
	for i := range s {
		if first || i < index {
			index, first = i, false
		}
	}
	return index, s[index]
}

// clone returns a deep copy of the skip stack, copy.deepcopy of upstream.
func (s skipStack) clone() skipStack {
	if s == nil {
		return nil
	}
	c := make(skipStack, len(s))
	for i, child := range s {
		c[i] = child.clone()
	}
	return c
}

// skipStackEqual reports whether two skip stacks are equal, as the operator
// == of Python does for dicts and None.
func skipStackEqual(a, b skipStack) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for i, child := range a {
		other, ok := b[i]
		if !ok || !skipStackEqual(child, other) {
			return false
		}
	}
	return true
}

// nextPage is the dict next_page of upstream, {'break': 'any', 'page':
// None}: the type of the break before the next page, and the name of the
// next page.
type nextPage struct {
	// brk is the value of the key 'break': "any", "page", "left", "right",
	// "recto" or "verso".
	brk string
	// page is the value of the key 'page': the page name of the next page,
	// or nil for None.
	page Value
}

// anyNextPage returns {'break': 'any', 'page': None}.
func anyNextPage() nextPage {
	return nextPage{brk: "any"}
}

// noMaxLines is the max_lines None of upstream.
const noMaxLines = -1

// blockLayoutResult is the tuple that block_level_layout and the other
// layouts of the block-level boxes return: (new_box, resume_at, next_page,
// adjoining_margins, collapsing_through, max_lines).
type blockLayoutResult struct {
	// box is the laid out box, or nil for None.
	box *Box
	// resumeAt is where the next page resumes, or nil.
	resumeAt skipStack
	// nextPage is the break and the name of the next page.
	nextPage nextPage
	// adjoiningMargins are the margins that adjoin the bottom of the box.
	adjoiningMargins []float64
	// collapsingThrough is set when the margins of the box collapse
	// through it.
	collapsingThrough bool
	// maxLines is the number of lines that line-clamp still allows, or
	// noMaxLines for None. Only block_level_layout returns it. The other
	// layouts return noMaxLines.
	maxLines int
}

// sizeBlock returns the tuple (width, height) that upstream passes as a
// containing block in place of a box. Its Kind is kindSize, and only its
// Width and Height are set.
func sizeBlock(width, height float64) *Box {
	return &Box{Kind: kindSize, Width: width, Height: height}
}

// isBox reports whether a containing block is a box, and not a tuple of
// sizeBlock: isinstance(containing_block, boxes.Box) of upstream.
func (b *Box) isBox() bool {
	return b.Kind != kindSize
}

// ErrNoPage is the error of Document.Page for a page that the document does
// not have.
var ErrNoPage = errors.New("no such page")

// maxLoops is the max_loops of layout_document: the number of times that
// the document is laid out again for the counters of the pages.
const maxLoops = 8

// maxLayoutDepth bounds the nesting of the layouts of the boxes.
//
// Port: upstream recurses until RecursionError. The box tree is at most
// DefaultMaxTreeDepth elements deep, with the anonymous boxes around them.
const maxLayoutDepth = 2*DefaultMaxTreeDepth + 64

// layoutAbort is the value of the panic that stops a layout at a limit or
// at the cancellation of its context. The entry points recover it into its
// error.
type layoutAbort struct {
	err error
}

// DocumentOptions holds the arguments of NewDocument, besides the document,
// its boxes and its styles.
type DocumentOptions struct {
	// FontConfig is the configuration of the fonts that the styles use,
	// as ComputedStylesOptions.FontConfig.
	FontConfig *FontConfiguration
	// TargetCollector is the collector of the targets of the cascade and
	// of the boxes, or nil.
	TargetCollector *TargetCollector
	// CounterStyle holds the counter styles of the style sheets, as
	// ComputedStylesOptions.CounterStyle, or nil for those of the user
	// agent.
	CounterStyle CounterStyle
	// Footnotes are the footnotes that BuildFormattingStructure returns.
	Footnotes []*Box
	// PagesBefore is the number of pages before the document, such as the
	// pages of the documents before it in a Book. The counter page of its
	// first page is PagesBefore + 1. Upstream has no such option.
	PagesBefore int
}

// Document is layout_document in weasyprint/layout/__init__.py: the layout
// of the whole document, which includes the line breaks, the page breaks,
// and the absolute size and position of all the boxes. It yields the page
// boxes, with their margin boxes.
//
// Page lays out the pages up to the page that a program asks for, and not
// the rest of the document, as the generator make_pages of upstream does.
// Count and All lay out the whole document, with the loop of upstream that
// lays out the document again for the counters of the pages.
//
// When the styles use counter(pages), a target counter (target-counter(),
// target-counters() or target-text()) or position: fixed, Page lays out the
// whole document before it returns a page (D43). The cascade finds these
// features in the styles that NewDocument takes. Page also lays out the whole document when the first pass of the
// loop changes the content of the page or of a page before it. Else, Page
// returns a page of the first pass, and the first page comes fast.
//
// A Document is not safe for concurrent use.
//
// Port: upstream lays out the whole document and returns a list.
type Document struct {
	html *HTML
	root *Box
	lc   *layoutContext

	// maxPages bounds the pages of each pass.
	maxPages int

	// The state of the loop of layout_document. prevPages are the pages of
	// the previous pass, the argument pages of make_pages, and pages are
	// the pages of the current pass. iter is the generator of the current
	// pass, or nil when it is done.
	loop              int
	prevPages         []*Box
	pages             []*Box
	iter              *pageIter
	initialTotalPages int
	actualTotalPages  int
	originalFootnotes []*Box
	// complete is set when the loop is done: pages are the pages of the
	// whole layout.
	complete bool

	// finished holds the pages with their margin boxes, by index, for the
	// current state of the layout. collected is the number of pages whose
	// bookmarks and named strings are collected, and watch holds the
	// elements whose bookmarks are seen, for the ::before, the ::after and
	// the other boxes.
	finished  map[int]*Box
	collected int
	watch     [3][]*Element

	// err is the error that stopped the layout. The document gives it
	// again.
	err error

	// pagesBefore is DocumentOptions.PagesBefore. pagesAfter is the
	// number of pages after the document, such as the pages of the
	// documents after it in a Book. The counter pages is the sum of
	// pagesBefore, the pages of the document and pagesAfter.
	pagesBefore int
	pagesAfter  int

	// whole is set when the styles need the whole layout before a page.
	whole bool
	// usesPages is set when the styles use the counter pages.
	usesPages bool
}

// NewDocument returns the layout of a document: the HTML document, the box
// of its root element from BuildFormattingStructure, and its styles from
// GetAllComputedStyles. It lays out no page yet.
func NewDocument(html *HTML, root *Box, styleFor *StyleFor, opts DocumentOptions) *Document {
	limits := html.Limits.WithDefaults()
	b := newBuilder(html, styleFor, BuildOptions{
		TargetCollector: opts.TargetCollector,
		CounterStyle:    opts.CounterStyle,
	})
	lc := newLayoutContext(styleFor, b, opts.FontConfig, limits)
	lc.footnotes = slices.Clone(opts.Footnotes)
	d := &Document{
		html: html,
		root: root,
		lc:   lc,
		// Port: upstream lays out as many pages as the document makes. The
		// port lays out at most Limits.MaxObjects / 16 pages, 65,536 by
		// default.
		maxPages:    max(1, limits.MaxObjects/16),
		finished:    map[int]*Box{},
		pagesBefore: max(opts.PagesBefore, 0),
		whole:       styleFor.uses.any(),
		usesPages:   styleFor.uses.pages,
	}
	return d
}

// Page returns the page box of the page of index i, from 0, with its margin
// boxes. It lays out the pages up to i first, or the whole document when
// the styles need it. It returns an error that wraps ErrNoPage when the
// document has fewer pages, an error that wraps ivy.ErrLimit when the
// document is past a limit, the error of ctx when ctx is done, and an error
// that wraps ivy.ErrFormat for a panic. After an error other than
// ErrNoPage, the document gives the same error again, because its layout
// stopped in the middle of a page.
//
// Port: a panic becomes an error that wraps ivy.ErrFormat.
func (d *Document) Page(ctx context.Context, i int) (page *Box, err error) {
	if i < 0 {
		return nil, fmt.Errorf("%w: page %d", ErrNoPage, i)
	}
	err = d.run(ctx, func() {
		page = d.page(i)
	})
	if err == nil && page == nil {
		err = fmt.Errorf("%w: page %d of %d", ErrNoPage, i, len(d.pages))
	}
	return page, err
}

// Count lays out the whole document and returns the number of its pages.
// It returns the errors of Page.
func (d *Document) Count(ctx context.Context) (n int, err error) {
	err = d.run(ctx, func() {
		d.completeLayout()
		n = len(d.pages)
	})
	return n, err
}

// All lays out the whole document and returns its page boxes, with their
// margin boxes. It returns the errors of Page.
func (d *Document) All(ctx context.Context) (pages []*Box, err error) {
	err = d.run(ctx, func() {
		d.completeLayout()
		pages = make([]*Box, len(d.pages))
		for i := range pages {
			pages[i] = d.page(i)
		}
	})
	return pages, err
}

// run calls f with the guards of the layout: it stops at the cancellation
// of ctx, and turns a panic into an error. The error stays the error of the
// document.
func (d *Document) run(ctx context.Context, f func()) (err error) {
	if d.err != nil {
		return d.err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	d.lc.interrupt = ctx.Err
	defer func() {
		d.lc.interrupt = nil
		if r := recover(); r != nil {
			if abort, ok := r.(layoutAbort); ok {
				err = abort.err
			} else {
				err = fmt.Errorf("%w: panic in the layout: %v", ivy.ErrFormat, r)
			}
			// The layout stopped in the middle of a page, so the document
			// cannot go on.
			d.err = err
		}
	}()
	f()
	return nil
}

// page returns the finished page of index i, or nil when the document has
// fewer pages. It lays out the pages that it needs.
func (d *Document) page(i int) *Box {
	if !d.complete && !d.whole && d.loop == 0 {
		for d.iter == nil && d.loop == 0 && !d.complete {
			d.startLoop()
		}
		for !d.complete && d.loop == 0 && len(d.pages) <= i {
			d.step()
		}
	}
	if !d.complete && (d.whole || d.loop > 0 || d.needsWholeDocument(i)) {
		d.completeLayout()
	}
	if i >= len(d.pages) {
		return nil
	}
	return d.finish(i)
}

// completeLayout lays out the whole document, with the loop of
// layout_document.
func (d *Document) completeLayout() {
	for !d.complete {
		if d.iter == nil {
			d.startLoop()
		}
		d.step()
	}
}

// startLoop starts a pass of the loop of layout_document.
func (d *Document) startLoop() {
	lc := d.lc
	if d.loop == 0 {
		first := newPageMakerFromRootBox(d.root)
		if d.pagesBefore > 0 {
			// Port: upstream has no pages before a document. The counter
			// page starts from them, and the increment of the first page
			// gives PagesBefore + 1.
			first.counterValues["page"] = []int{d.pagesBefore}
			first.counterScopes[0]["page"] = true
		}
		lc.pageMaker = []*pageMaker{first}
	} else {
		lc.footnotes = slices.Clone(d.originalFootnotes)
	}
	d.initialTotalPages = d.actualTotalPages
	if d.loop == 0 {
		d.originalFootnotes = slices.Clone(lc.footnotes)
	}
	d.pages = nil
	d.iter = newPageIter(lc, d.root, d.html, d.prevPages)
}

// step makes the next page of the current pass. At the end of the pass, it
// checks whether another pass is needed, as the loop of layout_document
// does.
func (d *Document) step() {
	lc := d.lc
	lc.work = 0
	// Check the context between the pages: a page can take less work
	// than the interval of the checks of spend.
	if lc.interrupt != nil {
		if err := lc.interrupt(); err != nil {
			lc.abort(err)
		}
	}
	if page, ok := d.iter.next(); ok {
		d.pages = append(d.pages, page)
		// Port: upstream has no bound on the number of pages.
		if len(d.pages) > d.maxPages {
			lc.abort(fmt.Errorf("%w: the document makes more than %d pages", ivy.ErrLimit, d.maxPages))
		}
		return
	}
	d.iter = nil
	d.actualTotalPages = len(d.pages)

	// Check whether another round is required.
	reloop := false
	for _, pageData := range lc.pageMaker {
		// Update pages.
		// Port: the counter pages also counts the pages before and after
		// the document, such as the pages of the other documents of a
		// Book.
		pageData.counterValues["pages"] = []int{d.pagesBefore + d.actualTotalPages + d.pagesAfter}
		if pageData.contentChanged {
			reloop = true
		} else if pageData.pagesWanted {
			reloop = d.initialTotalPages != d.actualTotalPages
		}
	}
	d.prevPages = d.pages
	d.loop++

	// No need for another loop, stop here.
	if !reloop || d.loop >= maxLoops {
		d.complete = true
		// The pages of the first pass that Page finished are not the
		// pages of the whole layout.
		clear(d.finished)
		d.collected = 0
		d.watch = [3][]*Element{}
		lc.stringSet = map[string]map[int][]string{}
	}
}

// needsWholeDocument reports whether the page of index i of the first pass
// needs another pass: the page or one of the pages before it wants the
// counter pages, or its content changed with the counters of the pages.
//
// Port: upstream always lays out the whole document.
func (d *Document) needsWholeDocument(i int) bool {
	for _, pageData := range d.lc.pageMaker[:min(i+2, len(d.lc.pageMaker))] {
		if pageData.contentChanged || pageData.pagesWanted {
			return true
		}
	}
	return false
}

// wholeLayoutUses holds the features of the styles of a document that need
// the whole layout before a page (D43).
//
// Port: upstream always lays out the whole document.
type wholeLayoutUses struct {
	// pages is set when the styles use counter(pages) or counters(pages).
	pages bool
	// targets is set when the styles use target-counter(),
	// target-counters() or target-text().
	targets bool
	// fixed is set when the styles give an element position: fixed.
	fixed bool
}

// any reports whether the styles use a feature that needs the whole
// layout.
func (u wholeLayoutUses) any() bool {
	return u.pages || u.targets || u.fixed
}

// add adds the features of v to u.
func (u *wholeLayoutUses) add(v wholeLayoutUses) {
	u.pages = u.pages || v.pages
	u.targets = u.targets || v.targets
	u.fixed = u.fixed || v.fixed
}

// forProperty keeps the features that the property name can use: fixed for
// position, and the counters for the others.
func (u wholeLayoutUses) forProperty(name string) wholeLayoutUses {
	if name == "position" {
		return wholeLayoutUses{fixed: u.fixed}
	}
	return wholeLayoutUses{pages: u.pages, targets: u.targets}
}

// wholeLayoutProperties are the properties whose values can need the whole
// layout.
var wholeLayoutProperties = [...]string{"content", "string_set", "bookmark_label", "position"}

// findWholeLayoutUses finds the features that need the whole layout in the
// cascaded styles of the elements and the pseudo-elements, and in the
// declarations of the @page rules and their margin rules. It reads four
// properties of each cascaded style, so it costs little next to the
// cascade. A declaration of an element that the cascade overrides does not
// count, but each declaration of the @page rules counts.
//
// A value with var() waits for the computed values. Its features are the
// features of its tokens and of the tokens of all the custom properties of
// the document, so it can count a feature that its computed value does not
// use.
//
// Port: upstream always lays out the whole document.
func findWholeLayoutUses(cascaded *cascadedStyles, sheets []sheetOrigin) wholeLayoutUses {
	var uses wholeLayoutUses
	// pending holds the values with var(), and pendingNames the names of
	// their properties.
	var pending []*Pending
	var pendingNames []string
	look := func(name string, value Value) {
		if p, ok := value.(*Pending); ok {
			pending = append(pending, p)
			pendingNames = append(pendingNames, name)
			return
		}
		uses.add(valueUses(name, value))
	}
	for _, k := range cascaded.order {
		style, _ := cascaded.styles.get(k)
		for _, name := range wholeLayoutProperties {
			if cv, ok := style[name]; ok {
				look(name, cv.value)
			}
		}
	}
	for _, sheet := range sheets {
		for _, rule := range *sheet.sheet.PageRules {
			for _, d := range rule.Declarations {
				if slices.Contains(wholeLayoutProperties[:], d.Name) {
					look(d.Name, d.Value)
				}
			}
		}
	}
	if len(pending) == 0 {
		return uses
	}

	// The custom properties, whose names start with "__", give the tokens
	// of var().
	var custom wholeLayoutUses
	customUses := func(name string, value Value) {
		if !strings.HasPrefix(name, "__") {
			return
		}
		switch v := value.(type) {
		case Tokens:
			custom.add(tokenUses(v))
		case *Pending:
			custom.add(tokenUses(v.Tokens))
		}
	}
	for _, k := range cascaded.order {
		style, _ := cascaded.styles.get(k)
		for name, cv := range style {
			customUses(name, cv.value)
		}
	}
	for _, sheet := range sheets {
		for _, rule := range *sheet.sheet.PageRules {
			for _, d := range rule.Declarations {
				customUses(d.Name, d.Value)
			}
		}
	}
	for i, p := range pending {
		v := tokenUses(p.Tokens)
		if slices.ContainsFunc(p.Tokens, checkVar) {
			v.add(custom)
		}
		uses.add(v.forProperty(pendingNames[i]))
	}
	return uses
}

// valueUses returns the features that a validated value of the property
// name uses.
func valueUses(name string, value Value) wholeLayoutUses {
	var u wholeLayoutUses
	switch name {
	case "position":
		u.fixed = isStr(value, "fixed")
	case "content", "bookmark_label":
		u = contentListUses(value)
	case "string_set":
		items, _ := value.(Tuple)
		for _, item := range items {
			if pair, ok := item.(Tuple); ok && len(pair) == 2 {
				u.add(contentListUses(pair[1]))
			}
		}
	}
	return u
}

// contentListUses returns the features that a content list uses.
func contentListUses(value Value) wholeLayoutUses {
	var u wholeLayoutUses
	items, _ := value.(Tuple)
	for _, item := range items {
		pair, ok := item.(Tuple)
		if !ok || len(pair) != 2 {
			continue
		}
		switch str(pair[0]) {
		case "counter()", "counters()":
			if args, ok := pair[1].(Tuple); ok && len(args) > 0 && isStr(args[0], "pages") {
				u.pages = true
			}
		case "target-counter()", "target-counters()", "target-text()":
			u.targets = true
		}
	}
	return u
}

// tokenUses returns the features that a list of tokens can use: a function
// counter() or counters() of the counter pages, a function target-counter(),
// target-counters() or target-text(), and the keyword fixed.
func tokenUses(tokens []tinycss2.Node) wholeLayoutUses {
	var u wholeLayoutUses
	for _, token := range tokens {
		switch t := token.(type) {
		case *tinycss2.IdentToken:
			u.fixed = u.fixed || t.LowerValue == "fixed"
		case *tinycss2.FunctionBlock:
			switch t.LowerName {
			case "counter", "counters":
				for _, arg := range t.Arguments {
					if ident, ok := arg.(*tinycss2.IdentToken); ok {
						u.pages = u.pages || ident.Value == "pages"
						break
					}
				}
			case "target-counter", "target-counters", "target-text":
				u.targets = true
			}
			u.add(tokenUses(t.Arguments))
		case *tinycss2.ParenthesesBlock:
			u.add(tokenUses(t.Content))
		case *tinycss2.SquareBracketsBlock:
			u.add(tokenUses(t.Content))
		case *tinycss2.CurlyBracketsBlock:
			u.add(tokenUses(t.Content))
		}
	}
	return u
}

// finish returns the page of index i of the current pages with its margin
// boxes, its fixed boxes and its backgrounds, as the end of layout_document
// does. It finishes each page once for each state of the layout.
//
// Port: upstream changes the pages of the list in place. The port finishes
// copies of the page box and of the box of the root element, so that the
// pages of a pass stay as make_pages made them.
func (d *Document) finish(i int) *Box {
	if page, ok := d.finished[i]; ok {
		return page
	}
	lc := d.lc
	lc.work = 0
	pages := d.pages
	// Calculate string-sets and bookmark-labels containing page based
	// counters when pagination is finished. No need to do that (maybe
	// multiple times) in make_page because they dont create boxes, only
	// appear in MarginBoxes and in the final PDF.
	for ; d.collected <= i; d.collected++ {
		d.collect(d.collected)
	}

	// Add margin boxes.
	page := pages[i]
	var root, footnoteArea, noteArea *Box
	if len(page.Children) != 3 {
		// Port: make_page always gives the root box, the footnote area and
		// the note area. A page without them stays as it is.
		d.finished[i] = page
		return page
	}
	root, footnoteArea, noteArea = page.Children[0], page.Children[1], page.Children[2]
	before := layoutFixedBoxes(lc, pages[:i], page)
	after := layoutFixedBoxes(lc, pages[i+1:], page)
	rootChildren := make([]*Box, 0, len(before)+len(root.Children)+len(after))
	rootChildren = append(rootChildren, before...)
	rootChildren = append(rootChildren, root.Children...)
	rootChildren = append(rootChildren, after...)
	root = root.Copy()
	root.Children = rootChildren
	lc.currentPage = i + 1 // page_number starts at 1

	// page_maker's page_state is ready for the MarginBoxes
	pageData := lc.pageMaker[lc.currentPage]
	page = page.Copy()
	children := []*Box{root}
	if len(footnoteArea.Children) > 0 {
		children = append(children, footnoteArea)
	}
	if len(noteArea.Children) > 0 {
		children = append(children, noteArea)
	}
	// Port: upstream gives the quote depth, the counter values and the
	// counter scopes of the page maker, which the content of the margin
	// boxes changes. The port gives copies, so that a page finished again
	// gets the same margin boxes.
	quoteDepth := pageData.quoteDepth
	children = append(children, makeMarginBoxes(lc, page, &quoteDepth,
		cloneCounterValues(pageData.counterValues), cloneCounterScopes(pageData.counterScopes))...)
	page.Children = children
	layoutBackgrounds(page, lc.getImageFromURI)
	d.finished[i] = page
	return page
}

// collect collects the named strings of the page of index i in the
// LayoutContext, removes the bookmarks that repeat the bookmark of an
// element, and computes the contents that need the counters of the pages,
// as the loop over the pages of layout_document does.
func (d *Document) collect(i int) {
	lc := d.lc
	// We need the updated page_counter_values
	pageCounterValues := lc.pageMaker[i+1].counterValues

	d.pages[i].Descendants(func(child *Box) bool {
		// Only one bookmark per original box
		if child.BookmarkLabel != "" {
			checklist := &d.watch[2]
			if strings.HasSuffix(child.ElementTag, "::before") {
				checklist = &d.watch[0]
			} else if strings.HasSuffix(child.ElementTag, "::after") {
				checklist = &d.watch[1]
			}
			if slices.Contains(*checklist, child.Element) {
				child.BookmarkLabel = ""
			} else {
				*checklist = append(*checklist, child.Element)
			}
		}

		if child.MissingLink != nil {
			tc := lc.targetCollector
			for _, key := range tc.counterOrder {
				item := tc.Counters[key]
				if child.MissingLink == key.Box && key.CSSToken != "content" {
					if key.CSSToken == "bookmark-label" && child.BookmarkLabel == "" {
						// don't refill it!
						continue
					}
					item.ParseAgain(pageCounterValues)
					// string_set is a pointer, but the bookmark_label is
					// just a string: copy it
					if key.CSSToken == "bookmark-label" {
						child.BookmarkLabel = key.Box.BookmarkLabel
					}
				}
			}
		}
		// Collect the string_sets in the LayoutContext
		for _, entry := range child.StringSet {
			pages := lc.stringSet[entry.Name]
			if pages == nil {
				pages = map[int][]string{}
				lc.stringSet[entry.Name] = pages
			}
			pages[i+1] = append(pages[i+1], entry.Value)
		}
		return true
	})
}

// cloneCounterScopes returns a deep copy of a list of counter scopes.
func cloneCounterScopes(scopes []map[string]bool) []map[string]bool {
	c := make([]map[string]bool, len(scopes))
	for i, scope := range scopes {
		c[i] = maps.Clone(scope)
	}
	return c
}

// layoutFixedBoxes is layout_fixed_boxes in weasyprint/layout/__init__.py.
// It lays out and returns the fixed boxes of pages on containingPage.
func layoutFixedBoxes(lc *layoutContext, pages []*Box, containingPage *Box) []*Box {
	var result []*Box
	for _, page := range pages {
		for _, box := range page.fixedBoxes {
			// As replaced boxes are never copied during layout, ensure that
			// we have different boxes (with a possibly different layout)
			// for each pages.
			if box.IsReplaced() {
				box = box.Copy()
			}
			// Absolute boxes in fixed boxes are rendered as fixed boxes'
			// children, even when they are fixed themselves.
			var absoluteBoxes []*Box
			absoluteBox, _ := absoluteBoxLayout(lc, box, containingPage, &absoluteBoxes, math.Inf(-1), nil)
			result = append(result, absoluteBox)
			for len(absoluteBoxes) > 0 {
				lc.spend(1)
				var newAbsoluteBoxes []*Box
				for _, box := range absoluteBoxes {
					absoluteLayout(lc, box, containingPage, &newAbsoluteBoxes, math.Inf(-1), nil)
				}
				absoluteBoxes = newAbsoluteBoxes
			}
		}
	}
	return result
}

// shapeList is a list of excluded shapes of a formatting context. A fake
// list is the FakeList of upstream, a list in which you can’t append
// objects, for the flex and the grid formatting contexts.
type shapeList struct {
	boxes []*Box
	fake  bool
}

// brokenOutOfFlow is an item of the dict broken_out_of_flow of upstream:
// the out-of-flow box newBox, broken at the end of a page, with the
// original box, its containing block, the root box of its formatting
// context and where it resumes.
type brokenOutOfFlow struct {
	newBox          *Box
	box             *Box
	containingBlock *Box
	rootBox         *Box
	resumeAt        skipStack
}

// layoutContext is LayoutContext in weasyprint/layout/__init__.py: the
// state of the layout of a document, which the functions of the layout
// share. It is the PageContext of the content of the margin boxes.
type layoutContext struct {
	styleFor        *StyleFor
	fontConfig      *FontConfiguration
	counterStyle    CounterStyle
	targetCollector *TargetCollector
	// builder builds the boxes of the margin boxes and of the footnote
	// area. Its image loader is get_image_from_uri.
	builder *builder
	log     *logger
	limits  ivy.Limits

	excludedShapesRootBoxes []*Box
	excludedShapesLists     map[*Box]*shapeList
	footnotes               []*Box
	currentPageFootnotes    []*Box
	reportedFootnotes       []*Box
	renderedNotes           []*Box
	// currentFootnoteArea is nil until make_page sets it.
	currentFootnoteArea *Box
	pageBottom          float64
	// stringSet and runningElements map the name of a named string or of
	// a running element to its values on each page, by page number.
	stringSet       map[string]map[int][]string
	runningElements map[string]map[int][]*Box
	currentPage     int
	forcedBreak     bool
	brokenOutOfFlow []brokenOutOfFlow
	inColumn        bool

	// pageMaker holds the states of the pagination, the page_maker of
	// upstream. It is set by layout_document.
	pageMaker []*pageMaker

	// tables is the cache of the preferred widths of the tables.
	tables map[*Box]*tablePreferredWidths

	// The guards of the port. work counts the units of work of the current
	// page, depth the nesting of the layouts, and interrupt is the method
	// Err of the context of the current call, or nil.
	work      int
	maxWork   int
	depth     int
	interrupt func() error

	// tabOrigin is the start edge of the margin box of the block container
	// of the line that the layout places, from which the tab stops of
	// -ivy-tab-stops are measured. Upstream has no tab stops.
	tabOrigin float64
}

// newLayoutContext is LayoutContext.__init__ in
// weasyprint/layout/__init__.py.
func newLayoutContext(styleFor *StyleFor, b *builder, fontConfig *FontConfiguration, limits ivy.Limits) *layoutContext {
	return &layoutContext{
		styleFor:            styleFor,
		fontConfig:          fontConfig,
		counterStyle:        b.counterStyle,
		targetCollector:     b.targetCollector,
		builder:             b,
		log:                 b.log,
		limits:              limits,
		excludedShapesLists: map[*Box]*shapeList{},
		stringSet:           map[string]map[int][]string{},
		runningElements:     map[string]map[int][]*Box{},
		tables:              map[*Box]*tablePreferredWidths{},
		// Port: upstream has no bound on the work. A page costs at most 64
		// times Limits.MaxObjects units of work.
		maxWork: 64 * limits.MaxObjects,
	}
}

// abort stops the layout with an error. The entry points return the error.
func (lc *layoutContext) abort(err error) {
	panic(layoutAbort{err: err})
}

// spend counts n units of work: the layout of a box, of a line or of a
// track. It stops the layout past the limit of the work of a page, and
// when the context of the call is done.
//
// Port: upstream has no bound. A document of nested flex containers, or of
// nested tables, lays out the same boxes a number of times that is
// exponential in its depth.
func (lc *layoutContext) spend(n int) {
	before := lc.work
	lc.work += n
	if lc.work > lc.maxWork {
		lc.abort(fmt.Errorf("%w: the layout of a page takes too long", ivy.ErrLimit))
	}
	if lc.interrupt != nil && before>>8 != lc.work>>8 {
		if err := lc.interrupt(); err != nil {
			lc.abort(err)
		}
	}
}

// enter counts one more level of nested layouts, and stops the layout past
// maxLayoutDepth. Each call needs a call to leave.
func (lc *layoutContext) enter() {
	lc.depth++
	if lc.depth > maxLayoutDepth {
		lc.abort(fmt.Errorf("%w: the layout nests more than %d boxes", ivy.ErrLimit, maxLayoutDepth))
	}
	lc.spend(1)
}

// leave ends a level of nested layouts that enter counted.
func (lc *layoutContext) leave() {
	lc.depth--
}

// getImageFromURI is get_image_from_uri of the context: the image of a URL,
// or nil when it cannot be read.
func (lc *layoutContext) getImageFromURI(url string, orientation Value) Image {
	return lc.builder.images.get(url, "", orientation)
}

// overflowsPage is overflows_page.
func (lc *layoutContext) overflowsPage(bottomSpace, positionY float64) bool {
	return overflows(lc.pageBottom-bottomSpace, positionY)
}

// overflows is LayoutContext.overflows.
func overflows(bottom, positionY float64) bool {
	// Use a small fudge factor to avoid floating numbers errors.
	// The 1e-9 value comes from PEP 485.
	return positionY > bottom*(1+1e-9)
}

// excludedShapes is the property excluded_shapes: the excluded shapes of
// the current formatting context.
func (lc *layoutContext) excludedShapes() []*Box {
	if list := lc.currentShapes(); list != nil {
		return list.boxes
	}
	return nil
}

// currentShapes returns the list of the excluded shapes of the current
// formatting context, or nil.
func (lc *layoutContext) currentShapes() *shapeList {
	if len(lc.excludedShapesRootBoxes) == 0 {
		return nil
	}
	return lc.excludedShapesLists[lc.excludedShapesRootBoxes[len(lc.excludedShapesRootBoxes)-1]]
}

// appendExcludedShape is context.excluded_shapes.append(box). A FakeList
// keeps no box.
func (lc *layoutContext) appendExcludedShape(box *Box) {
	if list := lc.currentShapes(); list != nil && !list.fake {
		list.boxes = append(list.boxes, box)
	}
}

// truncateExcludedShapes removes the excluded shapes of the current
// formatting context after the first n, as the calls to
// context.excluded_shapes.pop() do.
func (lc *layoutContext) truncateExcludedShapes(n int) {
	if list := lc.currentShapes(); list != nil && len(list.boxes) > n {
		list.boxes = list.boxes[:n]
	}
}

// setExcludedShapesFor is context._excluded_shapes[root_box] = shapes.
func (lc *layoutContext) setExcludedShapesFor(root *Box, shapes []*Box) {
	lc.excludedShapesLists[root] = &shapeList{boxes: shapes}
}

// createFormattingContext is _create_formatting_context.
func (lc *layoutContext) createFormattingContext(root *Box, newList *shapeList) {
	// Port: upstream asserts that root is not already a root box of a
	// formatting context. The port goes on.
	lc.excludedShapesRootBoxes = append(lc.excludedShapesRootBoxes, root)
	if _, ok := lc.excludedShapesLists[root]; !ok {
		lc.excludedShapesLists[root] = newList
	}
}

// finishFormattingContext is _finish_formatting_context.
func (lc *layoutContext) finishFormattingContext() {
	n := len(lc.excludedShapesRootBoxes)
	if n == 0 {
		return
	}
	root := lc.excludedShapesRootBoxes[n-1]
	lc.excludedShapesRootBoxes = lc.excludedShapesRootBoxes[:n-1]
	delete(lc.excludedShapesLists, root)
}

// createBlockFormattingContext is create_block_formatting_context. root is
// nil for the formatting context of a page.
func (lc *layoutContext) createBlockFormattingContext(root *Box) {
	lc.createFormattingContext(root, &shapeList{})
}

// finishBlockFormattingContext is finish_block_formatting_context. root is
// nil for None.
func (lc *layoutContext) finishBlockFormattingContext(root *Box) {
	// See https://www.w3.org/TR/CSS2/visudet.html#root-height.
	if root != nil && isStr(root.Style.Get("height"), "auto") && len(lc.excludedShapes()) > 0 {
		boxBottom := root.ContentBoxY() + root.Height
		maxShapeBottom := boxBottom
		for _, shape := range lc.excludedShapes() {
			maxShapeBottom = max(maxShapeBottom, shape.PositionY+shape.MarginHeight())
		}
		root.Height += maxShapeBottom - boxBottom
	}
	lc.finishFormattingContext()
}

// createFlexFormattingContext is create_flex_formatting_context.
func (lc *layoutContext) createFlexFormattingContext(root *Box) {
	lc.createFormattingContext(root, &shapeList{fake: true})
}

// finishFlexFormattingContext is finish_flex_formatting_context.
func (lc *layoutContext) finishFlexFormattingContext() {
	lc.finishFormattingContext()
}

// createGridFormattingContext is create_grid_formatting_context.
func (lc *layoutContext) createGridFormattingContext(root *Box) {
	lc.createFormattingContext(root, &shapeList{fake: true})
}

// finishGridFormattingContext is finish_grid_formatting_context.
func (lc *layoutContext) finishGridFormattingContext() {
	lc.finishFormattingContext()
}

// addBrokenOutOfFlow is add_broken_out_of_flow.
func (lc *layoutContext) addBrokenOutOfFlow(newBox, box, containingBlock *Box, resumeAt skipStack) {
	var root *Box
	if n := len(lc.excludedShapesRootBoxes); n > 0 {
		root = lc.excludedShapesRootBoxes[n-1]
	}
	item := brokenOutOfFlow{newBox, box, containingBlock, root, resumeAt}
	for i, other := range lc.brokenOutOfFlow {
		if other.newBox == newBox {
			lc.brokenOutOfFlow[i] = item
			return
		}
	}
	lc.brokenOutOfFlow = append(lc.brokenOutOfFlow, item)
}

// GetStringSetFor is get_string_set_for: the value of the named string
// name on page, for a keyword such as "first". It joins the strings of the
// tuple of upstream.
func (lc *layoutContext) GetStringSetFor(page *Box, name, keyword string) string {
	return strings.Join(getStringOrElementFor(lc, lc.stringSet, page, name, keyword), "")
}

// GetRunningElementFor is get_running_element_for: the boxes of the
// running element name on page, for a keyword such as "first".
func (lc *layoutContext) GetRunningElementFor(page *Box, name, keyword string) []*Box {
	return getStringOrElementFor(lc, lc.runningElements, page, name, keyword)
}

// getStringOrElementFor is get_string_or_element_for: the value of a
// string or of an element function.
//
// We'll have something like this that represents all assignments on a
// given page:
//
//	{1: ['First Header'], 3: ['Second Header'],
//	 4: ['Third Header', '3.5th Header']}
//
// Value depends on current page.
// https://drafts.csswg.org/css-gcpm/#funcdef-string
//
// store is the map where the resolved value is stored, page is the current
// page, name is the name of the named string or running element, and
// keyword indicates which value of the named string or running element to
// use. Default is the first assignment on the current page else the most
// recent assignment. It returns the texts of a string set, or the boxes of
// a running element.
func getStringOrElementFor[T any](lc *layoutContext, store map[string]map[int][]T, page *Box, name, keyword string) []T {
	pages := store[name]
	if pages == nil {
		// The store is a defaultdict upstream: reading a name adds it.
		// make_page then sees that the running elements of the name
		// changed, and makes the page again with them, as the note area
		// needs.
		pages = map[int][]T{}
		store[name] = pages
	}
	if current, ok := pages[lc.currentPage]; ok && len(current) > 0 {
		// A value was assigned on this page.
		switch keyword {
		case "first":
			return current[:1]
		case "start":
			element := page
			for element != nil {
				if setValue, ok := element.Style.Get("string_set").(Tuple); ok {
					for _, item := range setValue {
						if pair, ok := item.(Tuple); ok && len(pair) > 0 && isStr(pair[0], name) {
							return current[:1]
						}
					}
				}
				if len(element.Children) > 0 {
					element = element.Children[0]
					continue
				}
				break
			}
		case "last":
			return current[len(current)-1:]
		case "first-except":
			return nil
		case "all-once":
			return slices.Clone(current)
		}
	}
	// Search backwards through previous pages.
	if keyword != "all-once" {
		for previousPage := lc.currentPage - 1; previousPage > 0; previousPage-- {
			if previous, ok := pages[previousPage]; ok && len(previous) > 0 {
				return previous[len(previous)-1:]
			}
		}
	}
	return nil
}

// addRunningElement is add_running_element: it adds a running element to
// the list of running elements.
func (lc *layoutContext) addRunningElement(box *Box) {
	runningName := runningElementName(box)
	if box.IsNote() && box.Style.Get("anchor") == nil {
		index := 0
		for _, names := range lc.runningElements {
			for _, boxes := range names {
				index += len(boxes)
			}
		}
		box.Style.Set("anchor", String(fmt.Sprintf("note-%d", index)))
		if box.NoteCall != nil {
			box.NoteCall.Style.Set("anchor", String(fmt.Sprintf("call-%d", index)))
			box.NoteCall.Link = GetLink(lc.log, fmt.Sprintf("#note-%d", index), box.Style.BaseURL)
		}
		if box.NoteCallback != nil {
			box.NoteCallback.Link = GetLink(lc.log, fmt.Sprintf("#call-%d", index), box.Style.BaseURL)
		}
	}
	if !slices.Contains(lc.renderedNotes, box) {
		pages := lc.runningElements[runningName]
		if pages == nil {
			pages = map[int][]*Box{}
			lc.runningElements[runningName] = pages
		}
		pages[lc.currentPage] = append(pages[lc.currentPage], box)
	}
}

// runningElementName returns style['position'][1], the name of a running
// element or of a note.
func runningElementName(box *Box) string {
	if t, ok := box.Style.Get("position").(Tuple); ok && len(t) > 1 {
		return str(t[1])
	}
	return ""
}

// removeRunningElement is remove_running_element: it removes a running
// element from the list of running elements.
func (lc *layoutContext) removeRunningElement(box *Box) {
	pages := lc.runningElements[runningElementName(box)]
	if pages == nil {
		return
	}
	boxes := pages[lc.currentPage]
	if i := slices.Index(boxes, box); i >= 0 {
		pages[lc.currentPage] = slices.Delete(boxes, i, i+1)
	}
}

// layoutFootnote is layout_footnote: it adds a footnote to the layout for
// this page, and reports whether the footnote area overflows.
func (lc *layoutContext) layoutFootnote(footnote *Box) bool {
	if i := slices.Index(lc.footnotes, footnote); i >= 0 {
		lc.footnotes = slices.Delete(lc.footnotes, i, i+1)
	}
	lc.currentPageFootnotes = append(lc.currentPageFootnotes, footnote)
	return lc.updateFootnoteArea()
}

// unlayoutFootnote is unlayout_footnote: it removes a footnote from the
// layout and returns it to the waitlist.
func (lc *layoutContext) unlayoutFootnote(footnote *Box) {
	// TODO: Handle unlayouting a footnote that hasn't been laid out yet or
	// has already been unlayouted
	if slices.Contains(lc.footnotes, footnote) {
		return
	}
	lc.footnotes = append(lc.footnotes, footnote)
	if i := slices.Index(lc.currentPageFootnotes, footnote); i >= 0 {
		lc.currentPageFootnotes = slices.Delete(lc.currentPageFootnotes, i, i+1)
	} else if i := slices.Index(lc.reportedFootnotes, footnote); i >= 0 {
		lc.reportedFootnotes = slices.Delete(lc.reportedFootnotes, i, i+1)
	}
	lc.updateFootnoteArea()
}

// reportFootnote is report_footnote: it marks a footnote as being moved to
// the next page.
func (lc *layoutContext) reportFootnote(footnote *Box) {
	if i := slices.Index(lc.currentPageFootnotes, footnote); i >= 0 {
		lc.currentPageFootnotes = slices.Delete(lc.currentPageFootnotes, i, i+1)
	}
	lc.reportedFootnotes = append(lc.reportedFootnotes, footnote)
	lc.updateFootnoteArea()
}

// updateFootnoteArea is _update_footnote_area: it updates the page bottom
// size and the height of the footnote area, and reports whether the
// footnote area overflows.
func (lc *layoutContext) updateFootnoteArea() bool {
	area := lc.currentFootnoteArea
	if area == nil {
		// Port: upstream fails on the None of the area before make_page
		// sets it.
		return false
	}
	if area.Height != Auto && !lc.inColumn {
		lc.pageBottom += area.MarginHeight()
	}
	area.Children = slices.Clone(lc.currentPageFootnotes)
	if len(area.Children) == 0 {
		area.Height = 0
		if !lc.inColumn {
			lc.pageBottom -= area.MarginHeight()
		}
		return false
	}
	footnoteArea := createAnonymousBoxes(lc.builder, area.DeepCopy())
	footnoteArea = blockLevelLayout(lc, footnoteArea, math.Inf(-1), nil, area.Page,
		true, nil, nil, nil, nil, nil, false, noMaxLines).box
	if footnoteArea == nil {
		// Port: the layout of the area always gives a box upstream.
		return false
	}
	area.Height = footnoteArea.Height
	if !lc.inColumn {
		lc.pageBottom -= footnoteArea.MarginHeight()
	}
	if len(footnoteArea.Children) == 0 {
		return false
	}
	lastChild := footnoteArea.Children[len(footnoteArea.Children)-1]
	lastChildBottom := lastChild.PositionY + lastChild.MarginHeight() - lastChild.MarginBottom
	footnoteAreaBottom := footnoteArea.PositionY + footnoteArea.MarginHeight() - footnoteArea.MarginBottom
	return lastChildBottom > footnoteAreaBottom
}
