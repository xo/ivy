// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/layout/page.py: the layout for pages and CSS3
// margin boxes.

package layout

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/xo/ivy"
)

// orientedBox is OrientedBox in weasyprint/layout/page.py, with its
// subclasses VerticalBox and HorizontalBox: a box seen along one axis.
//
// Port: upstream has a class for each axis. The port has one type, and
// vertical selects the axis.
type orientedBox struct {
	lc       *layoutContext
	box      *Box
	vertical bool

	// Inner dimension: that of the content area, as opposed to the
	// outer dimension: that of the margin area.
	inner             float64
	marginA           float64
	marginB           float64
	paddingPlusBorder float64

	// The cached min-content and max-content sizes of a horizontal box.
	minContent, maxContent       float64
	hasMinContent, hasMaxContent bool
}

// newVerticalBox is VerticalBox.__init__ in weasyprint/layout/page.py.
func newVerticalBox(lc *layoutContext, box *Box) *orientedBox {
	return &orientedBox{
		lc:       lc,
		box:      box,
		vertical: true,
		inner:    box.Height,
		marginA:  box.MarginTop,
		marginB:  box.MarginBottom,
		paddingPlusBorder: box.PaddingTop + box.PaddingBottom +
			box.BorderTopWidth + box.BorderBottomWidth,
	}
}

// newHorizontalBox is HorizontalBox.__init__ in weasyprint/layout/page.py.
func newHorizontalBox(lc *layoutContext, box *Box) *orientedBox {
	return &orientedBox{
		lc:      lc,
		box:     box,
		inner:   box.Width,
		marginA: box.MarginLeft,
		marginB: box.MarginRight,
		paddingPlusBorder: box.PaddingLeft + box.PaddingRight +
			box.BorderLeftWidth + box.BorderRightWidth,
	}
}

// newOrientedBox returns a vertical or a horizontal box.
func newOrientedBox(lc *layoutContext, box *Box, vertical bool) *orientedBox {
	if vertical {
		return newVerticalBox(lc, box)
	}
	return newHorizontalBox(lc, box)
}

// sugar is the property sugar of OrientedBox.
func (o *orientedBox) sugar() float64 {
	return o.paddingPlusBorder + o.marginA + o.marginB
}

// outer is the property outer of OrientedBox.
func (o *orientedBox) outer() float64 {
	return o.sugar() + o.inner
}

// setOuter is the setter of the property outer of OrientedBox.
func (o *orientedBox) setOuter(newOuterWidth float64) {
	o.inner = min(max(o.minContentSize(), newOuterWidth-o.sugar()), o.maxContentSize())
}

// outerMinContentSize is the property outer_min_content_size.
func (o *orientedBox) outerMinContentSize() float64 {
	if o.inner == Auto {
		return o.sugar() + o.minContentSize()
	}
	return o.sugar() + o.inner
}

// outerMaxContentSize is the property outer_max_content_size.
func (o *orientedBox) outerMaxContentSize() float64 {
	if o.inner == Auto {
		return o.sugar() + o.maxContentSize()
	}
	return o.sugar() + o.inner
}

// restoreBoxAttributes is restore_box_attributes.
func (o *orientedBox) restoreBoxAttributes() {
	if o.vertical {
		o.box.Height = o.inner
		o.box.MarginTop = o.marginA
		o.box.MarginBottom = o.marginB
	} else {
		o.box.Width = o.inner
		o.box.MarginLeft = o.marginA
		o.box.MarginRight = o.marginB
	}
}

// minContentSize is the property min_content_size.
func (o *orientedBox) minContentSize() float64 {
	if o.vertical {
		// TODO: Define what are the min-content and max-content heights.
		return 0
	}
	if !o.hasMinContent {
		o.minContent = minContentWidth(o.lc, o.box, false)
		o.hasMinContent = true
	}
	return o.minContent
}

// maxContentSize is the property max_content_size.
func (o *orientedBox) maxContentSize() float64 {
	if o.vertical {
		return 1e6
	}
	if !o.hasMaxContent {
		o.maxContent = maxContentWidth(o.lc, o.box, false)
		o.hasMaxContent = true
	}
	return o.maxContent
}

// pageMaker is PageMaker in weasyprint/layout/page.py: it collects the
// states of the pagination that the page based counters need. The state of
// the item of index i is the state at the start of the page of index i.
type pageMaker struct {
	resumeAt      skipStack
	nextPage      nextPage
	rightPage     bool
	quoteDepth    int
	counterValues map[string][]int
	counterScopes []map[string]bool
	// pageGroups are the page groups of the page: the name, the index of
	// the page in the group, and where the group starts.
	pageGroups     []pageGroup
	contentChanged bool
	pagesWanted    bool
	anchors        []string
	counters       []*TargetCounter
}

// pageGroup is an item of the page groups of upstream, [name, index,
// resume_at].
type pageGroup struct {
	name     Value
	index    int
	resumeAt skipStack
}

// clonePageGroups returns a deep copy of page groups, copy.deepcopy of
// upstream.
func clonePageGroups(groups []pageGroup) []pageGroup {
	c := make([]pageGroup, len(groups))
	for i, group := range groups {
		c[i] = pageGroup{group.name, group.index, group.resumeAt.clone()}
	}
	return c
}

// newPageMakerFromRootBox is PageMaker.from_root_box.
func newPageMakerFromRootBox(root *Box) *pageMaker {
	// TODO: take care of text direction and writing mode.
	// https://www.w3.org/TR/css-page-3/#progression
	recto := isStr(root.Style.Get("direction"), "ltr")
	rightPage := recto
	switch str(root.Style.Get("break_before")) {
	case "right":
		rightPage = true
	case "left":
		rightPage = false
	case "recto":
		rightPage = recto
	case "verso":
		rightPage = !recto
	}
	start, _ := root.PageValues()
	return &pageMaker{
		nextPage:      nextPage{brk: "any", page: start},
		rightPage:     rightPage,
		counterValues: map[string][]int{"pages": {0}},
		counterScopes: []map[string]bool{{"pages": true}},
	}
}

// computeFixedDimension is compute_fixed_dimension in
// weasyprint/layout/page.py. It computes and sets a margin box fixed
// dimension on box.
//
// box is the margin box to work on. outer is the target outer dimension.
// vertical is true to set height, margin-top and margin-bottom, and false
// for width, margin-left and margin-right. topOrLeft is true if the margin
// box in if the top or left half of the page. This determines which margin
// should be 'auto' if the values are over-constrained.
func computeFixedDimension(lc *layoutContext, marginBox *Box, outer float64, vertical, topOrLeft bool) {
	// See https://drafts.csswg.org/css-page-3/#margin-constraints.
	box := newOrientedBox(lc, marginBox, vertical)

	// Rule 2.
	total := box.paddingPlusBorder
	for _, size := range [...]float64{box.marginA, box.marginB, box.inner} {
		if size != Auto {
			total += size
		}
	}
	if total > outer {
		if box.marginA == Auto {
			box.marginA = 0
		}
		if box.marginB == Auto {
			box.marginB = 0
		}
		if box.inner == Auto {
			// This is not specified, but without it box.inner would end up
			// with a negative value. Instead, this will trigger rule 3
			// below.
			// https://lists.w3.org/Archives/Public/www-style/2012Jul/0006.html
			box.inner = 0
		}
	}
	// Rule 3.
	if box.marginA != Auto && box.marginB != Auto && box.inner != Auto {
		if topOrLeft {
			box.marginA = Auto
		} else {
			box.marginB = Auto
		}
	}
	// Rule 4.
	autos := 0
	for _, size := range [...]float64{box.marginA, box.marginB, box.inner} {
		if size == Auto {
			autos++
		}
	}
	if autos == 1 {
		switch {
		case box.inner == Auto:
			box.inner = outer - box.paddingPlusBorder - box.marginA - box.marginB
		case box.marginA == Auto:
			box.marginA = outer - box.paddingPlusBorder - box.marginB - box.inner
		case box.marginB == Auto:
			box.marginB = outer - box.paddingPlusBorder - box.marginA - box.inner
		}
	}
	// Rule 5.
	if box.inner == Auto {
		if box.marginA == Auto {
			box.marginA = 0
		}
		if box.marginB == Auto {
			box.marginB = 0
		}
		box.inner = outer - box.paddingPlusBorder - box.marginA - box.marginB
	}
	// Rule 6.
	if box.marginA == Auto && box.marginB == Auto {
		box.marginA = (outer - box.paddingPlusBorder - box.inner) / 2
		box.marginB = box.marginA
	}

	// Port: upstream asserts that no value is auto. The rules above
	// resolve each one.
	box.restoreBoxAttributes()
}

// computeVariableDimension is compute_variable_dimension in
// weasyprint/layout/page.py. It computes and sets a margin box fixed
// dimension on the boxes.
//
// Described in: https://drafts.csswg.org/css-page-3/#margin-dimension
//
// sideBoxes are three boxes on a same side (as opposed to a corner): a
// @*-left or @*-top margin box, a @*-center or @*-middle margin box, and a
// @*-right or @*-bottom margin box. vertical is true to set height,
// margin-top and margin-bottom, and false for width, margin-left and
// margin-right. availableSize is the distance between the page box’s left
// right border edges.
func computeVariableDimension(lc *layoutContext, sideBoxes [3]*Box, vertical bool, availableSize float64) {
	boxA := newOrientedBox(lc, sideBoxes[0], vertical)
	boxB := newOrientedBox(lc, sideBoxes[1], vertical)
	boxC := newOrientedBox(lc, sideBoxes[2], vertical)
	side := [3]*orientedBox{boxA, boxB, boxC}

	for _, box := range side {
		if box.marginA == Auto {
			box.marginA = 0
		}
		if box.marginB == Auto {
			box.marginB = 0
		}
	}

	if !boxB.box.isGenerated {
		// Non-generated boxes get zero for every box-model property.
		// Port: upstream asserts that the inner size of B is 0.
		switch {
		case boxA.inner == Auto && boxC.inner == Auto:
			// A and C both have 'width: auto'.
			maxExternal := boxA.outerMaxContentSize() + boxC.outerMaxContentSize()
			minExternal := boxA.outerMinContentSize() + boxC.outerMinContentSize()
			switch {
			case availableSize > maxExternal:
				// Sum of the outer max-content widths is less than the
				// available width.
				flexSpace := availableSize - maxExternal
				flexFactorA := boxA.outerMaxContentSize()
				flexFactorC := boxC.outerMaxContentSize()
				flexFactorSum := flexFactorA + flexFactorC
				if flexFactorSum == 0 {
					flexFactorSum = 1
				}
				boxA.setOuter(boxA.maxContentSize() + flexSpace*flexFactorA/flexFactorSum)
				boxC.setOuter(boxC.maxContentSize() + flexSpace*flexFactorC/flexFactorSum)
			case availableSize > minExternal:
				// Sum of the outer min-content widths is less than the
				// available width.
				flexSpace := availableSize - minExternal
				flexFactorA := boxA.maxContentSize() - boxA.minContentSize()
				flexFactorC := boxC.maxContentSize() - boxC.minContentSize()
				flexFactorSum := flexFactorA + flexFactorC
				if flexFactorSum == 0 {
					flexFactorSum = 1
				}
				boxA.setOuter(boxA.minContentSize() + flexSpace*flexFactorA/flexFactorSum)
				boxC.setOuter(boxC.minContentSize() + flexSpace*flexFactorC/flexFactorSum)
			default:
				flexSpace := availableSize - minExternal
				flexFactorA := boxA.minContentSize()
				flexFactorC := boxC.minContentSize()
				flexFactorSum := flexFactorA + flexFactorC
				if flexFactorSum == 0 {
					flexFactorSum = 1
				}
				boxA.setOuter(boxA.minContentSize() + flexSpace*flexFactorA/flexFactorSum)
				boxC.setOuter(boxC.minContentSize() + flexSpace*flexFactorC/flexFactorSum)
			}
		case boxA.inner == Auto:
			// Only one box has 'width: auto'.
			boxA.setOuter(availableSize - boxC.outer())
		case boxC.inner == Auto:
			boxC.setOuter(availableSize - boxA.outer())
		}
	} else {
		if boxB.inner == Auto {
			// Resolve any auto width of the middle box (B).
			acMaxContentSize := 2 * max(boxA.outerMaxContentSize(), boxC.outerMaxContentSize())
			maxSize := boxB.outerMaxContentSize() + acMaxContentSize
			if availableSize > maxSize {
				flexSpace := availableSize - maxSize
				flexFactorB := boxB.outerMaxContentSize()
				flexFactorAC := acMaxContentSize
				flexFactorSum := flexFactorB + flexFactorAC
				if flexFactorSum == 0 {
					flexFactorSum = 1
				}
				boxB.setOuter(boxB.maxContentSize() + flexSpace*flexFactorB/flexFactorSum)
			} else {
				acMinContentSize := 2 * max(boxA.outerMinContentSize(), boxC.outerMinContentSize())
				minSize := boxB.outerMinContentSize() + acMinContentSize
				flexSpace := availableSize - minSize
				if availableSize > minSize {
					flexFactorB := boxB.maxContentSize() - boxB.minContentSize()
					flexFactorAC := acMaxContentSize - acMinContentSize
					flexFactorSum := flexFactorB + flexFactorAC
					if flexFactorSum == 0 {
						flexFactorSum = 1
					}
					boxB.setOuter(boxB.minContentSize() + flexSpace*flexFactorB/flexFactorSum)
				} else {
					flexFactorB := boxB.minContentSize()
					flexFactorAC := acMinContentSize
					flexFactorSum := flexFactorB + flexFactorAC
					if flexFactorSum == 0 {
						flexFactorSum = 1
					}
					boxB.setOuter(boxB.minContentSize() + flexSpace*flexFactorB/flexFactorSum)
				}
			}
		}
		if boxA.inner == Auto {
			boxA.setOuter((availableSize - boxB.outer()) / 2)
		}
		if boxC.inner == Auto {
			boxC.setOuter((availableSize - boxB.outer()) / 2)
		}
	}

	for _, box := range side {
		// Port: upstream asserts that no inner size is auto. A box whose
		// inner size stays auto gets 0.
		if box.inner == Auto {
			box.inner = 0
		}
		box.restoreBoxAttributes()
	}
}

// standardizePageBasedCounters is _standardize_page_based_counters in
// weasyprint/layout/page.py. It drops the 'pages' counter from the style in
// @page and @margin context, and ensures `counter-increment: page` for
// @page context if not otherwise manipulated by the style. pseudoType is ""
// for None.
func standardizePageBasedCounters(style *Style, pseudoType string) {
	pageCounterTouched := false
	for _, propname := range [...]string{"counter_set", "counter_reset", "counter_increment"} {
		value := style.Get(propname)
		if isStr(value, "auto") {
			style.Set(propname, Tuple{})
			continue
		}
		justifiedValues := Tuple{}
		values, _ := value.(Tuple)
		for _, item := range values {
			pair, ok := item.(Tuple)
			if !ok || len(pair) != 2 {
				// Port: upstream fails to unpack a value that is not a
				// pair.
				continue
			}
			name := str(pair[0])
			if name == "page" {
				pageCounterTouched = true
			}
			if name != "pages" {
				justifiedValues = append(justifiedValues, pair)
			}
		}
		style.Set(propname, justifiedValues)
	}

	if pseudoType == "" && !pageCounterTouched {
		increments, _ := style.Get("counter_increment").(Tuple)
		style.Set("counter_increment",
			append(Tuple{Tuple{String("page"), Int(1)}}, increments...))
	}
}

// updatePageCounters is _update_page_counters in
// weasyprint/layout/page.py.
func updatePageCounters(style *Style, lc *layoutContext, page *Box, counterValues map[string][]int, counterScopes []map[string]bool, index int) {
	standardizePageBasedCounters(style, "")
	updateCounters(counterValues, counterScopes, style)
	targetCollector := lc.targetCollector
	pageMakers := lc.pageMaker
	pageData := pageMakers[index]

	// Evaluate and cache page values only once.
	var cachedAnchors []string
	var cachedCounters []*TargetCounter
	for _, previousPageData := range pageMakers[:index] {
		cachedAnchors = append(cachedAnchors, previousPageData.anchors...)
		cachedCounters = append(cachedCounters, previousPageData.counters...)
	}

	page.descendants(true, func(child *Box) bool {
		lc.spend(1)
		// Cache target's page counters.
		if anchor := child.Style.Get("anchor"); truthy(anchor) {
			anchorName := str(anchor)
			if !slices.Contains(cachedAnchors, anchorName) {
				pageData.anchors = append(pageData.anchors, anchorName)
				cachedAnchors = append(cachedAnchors, anchorName)
				targetCollector.CacheTargetPageCounters(anchorName, counterValues, index, len(pageMakers))
			}
		}

		// string-set and bookmark-labels don't create boxes, only
		// `content` requires another call to make_page. There is maximum
		// one 'content' item per box.
		var counter *TargetCounter
		if child.MissingLink != nil {
			// A Counter exists for the css-token 'content'.
			counter = targetCollector.Counters[CounterKey{child.MissingLink, "content"}]
		}

		// Resolve missing page based counters.
		if counter == nil {
			return true
		}
		callParseAgain := false

		// Prevent endless loops
		refreshMissingCounters := !slices.Contains(cachedCounters, counter)
		if refreshMissingCounters {
			pageData.counters = append(pageData.counters, counter)
			cachedCounters = append(cachedCounters, counter)
			counter.PageMakerIndex = index
		}

		// Step 1: page based back-references
		// Marked as pending by target_collector.cache_target_page_counters
		if counter.Pending {
			if !counterValuesEqual(counterValues, counter.CachedPageCounterValues) {
				counter.CachedPageCounterValues = copyCounterValues(counterValues)
			}
			counter.Pending = false
			callParseAgain = true
		}

		// Step 2: local counters
		// If the box mixed-in page counters changed, update the content
		// and cache the new values.
		missingCounters := counter.MissingCounters
		if len(missingCounters) > 0 {
			if slices.Contains(missingCounters, "pages") {
				pageData.pagesWanted = true
			}
			refresh := refreshMissingCounters &&
				!counterValuesEqual(counterValues, counter.CachedPageCounterValues)
			if refresh {
				counter.CachedPageCounterValues = copyCounterValues(counterValues)
				for _, counterName := range missingCounters {
					if _, ok := counterValues[counterName]; ok {
						callParseAgain = true
						// no need to loop them all
						break
					}
				}
			}
		}

		// Step 3: targeted counters
		for anchorName, missedCounters := range counter.MissingTargetCounters {
			if !slices.Contains(missedCounters, "pages") {
				continue
			}
			// Adjust 'pages_wanted'
			item := targetCollector.Targets[anchorName]
			if item == nil {
				// Port: upstream reads the attribute of None.
				continue
			}
			pageMakerIndex := item.PageMakerIndex
			if pageMakerIndex >= 0 && pageMakerIndex < len(pageMakers) && slices.Contains(cachedAnchors, anchorName) {
				pageMakers[pageMakerIndex].pagesWanted = true
			}
			// 'content_changed' is triggered in
			// targets.cache_target_page_counters()
		}

		if callParseAgain {
			pageData.contentChanged = true
			if counter.ParseAgain != nil {
				counter.ParseAgain(counterValues)
			}
		}
		return true
	})
}

// makeMarginBoxes is make_margin_boxes in weasyprint/layout/page.py: it
// returns the laid-out margin boxes for this page.
func makeMarginBoxes(lc *layoutContext, page *Box, quoteDepth *int, counterValues map[string][]int, counterScopes []map[string]bool) []*Box {
	// makeBox returns a margin box with resolved percentages.
	//
	// The margin box may still have 'auto' values.
	//
	// Port: upstream returns None if this margin box should not be
	// generated. No caller of upstream gets None: the box is not
	// generated, and isGenerated is false.
	//
	// atKeyword is which margin box to return, e.g. '@top-left', and
	// containingBlock is as expected by resolvePercentages.
	makeBox := func(atKeyword string, containingBlock *Box) *Box {
		style := lc.styleFor.Get(page.PageType, atKeyword)
		if style == nil {
			// doesn't affect counters
			style = page.Style.AnonymousStyle()
		}
		standardizePageBasedCounters(style, atKeyword)
		box := NewMarginBox(atKeyword, style)
		// Empty boxes should not be generated, but they may be needed for
		// the layout of their neighbors.
		// TODO: should be the computed value.
		box.isGenerated = !isStrIn(style.Get("content"), "normal", "inhibit", "none")
		// TODO: get actual counter values at the time of the last page
		// break
		if box.isGenerated {
			// @margins mustn't manipulate page-context counters
			marginQuoteDepth := *quoteDepth
			marginCounterValues := copyCounterValues(counterValues)
			marginCounterScopes := cloneCounterScopes(counterScopes)
			// TODO: check this, probably useless
			marginCounterScopes = append(marginCounterScopes, map[string]bool{})
			updateCounters(marginCounterValues, marginCounterScopes, box.Style)
			box.Children = lc.builder.contentToBoxes(
				box.Style, box, &marginQuoteDepth, marginCounterValues, lc, page)
			lc.checkBuilder()
			box.ProcessWhitespace(false)
			box.ProcessTextTransform()
			box = createAnonymousBoxes(lc.builder, box)
			lc.checkBuilder()
		}
		resolvePercentages(box, containingBlock)
		if !box.isGenerated {
			box.Width, box.Height = 0, 0
			for _, side := range [...]string{"top", "right", "bottom", "left"} {
				box.resetSpacing(side)
			}
		}
		return box
	}

	var result []*Box
	marginTop := page.MarginTop
	marginBottom := page.MarginBottom
	marginLeft := page.MarginLeft
	marginRight := page.MarginRight
	maxBoxWidth := page.BorderWidth()
	maxBoxHeight := page.BorderHeight()

	// Generate side boxes.
	pageEndX := marginLeft + maxBoxWidth
	pageEndY := marginTop + maxBoxHeight
	sides := [...]struct {
		prefix               string
		vertical             bool
		cbWidth, cbHeight    float64
		positionX, positionY float64
	}{
		{"top", false, maxBoxWidth, marginTop, marginLeft, 0},
		{"bottom", false, maxBoxWidth, marginBottom, marginLeft, pageEndY},
		{"left", true, marginLeft, maxBoxHeight, 0, marginTop},
		{"right", true, marginRight, maxBoxHeight, pageEndX, marginTop},
	}
	for _, s := range sides {
		var suffixes [3]string
		var fixedOuter, variableOuter float64
		if s.vertical {
			suffixes = [3]string{"top", "middle", "bottom"}
			fixedOuter, variableOuter = s.cbWidth, s.cbHeight
		} else {
			suffixes = [3]string{"left", "center", "right"}
			variableOuter, fixedOuter = s.cbWidth, s.cbHeight
		}
		var sideBoxes [3]*Box
		anyGenerated := false
		for i, suffix := range suffixes {
			sideBoxes[i] = makeBox("@"+s.prefix+"-"+suffix, sizeBlock(s.cbWidth, s.cbHeight))
			anyGenerated = anyGenerated || sideBoxes[i].isGenerated
		}
		if !anyGenerated {
			continue
		}
		computeVariableDimension(lc, sideBoxes, s.vertical, variableOuter)
		for i, offset := range [...]float64{0, 0.5, 1} {
			box := sideBoxes[i]
			if !box.isGenerated {
				continue
			}
			box.PositionX = s.positionX
			box.PositionY = s.positionY
			if s.vertical {
				box.PositionY += offset * (variableOuter - box.MarginHeight())
			} else {
				box.PositionX += offset * (variableOuter - box.MarginWidth())
			}
			computeFixedDimension(lc, box, fixedOuter, !s.vertical, s.prefix == "top" || s.prefix == "left")
			result = append(result, marginBoxContentLayout(lc, box))
		}
	}

	// Generate corner boxes.
	corners := [...]struct {
		atKeyword            string
		cbWidth, cbHeight    float64
		positionX, positionY float64
	}{
		{"@top-left-corner", marginLeft, marginTop, 0, 0},
		{"@top-right-corner", marginRight, marginTop, pageEndX, 0},
		{"@bottom-left-corner", marginLeft, marginBottom, 0, pageEndY},
		{"@bottom-right-corner", marginRight, marginBottom, pageEndX, pageEndY},
	}
	for _, c := range corners {
		box := makeBox(c.atKeyword, sizeBlock(c.cbWidth, c.cbHeight))
		if !box.isGenerated {
			continue
		}
		box.PositionX = c.positionX
		box.PositionY = c.positionY
		computeFixedDimension(lc, box, c.cbHeight, true, strings.Contains(c.atKeyword, "top"))
		computeFixedDimension(lc, box, c.cbWidth, false, strings.Contains(c.atKeyword, "left"))
		result = append(result, marginBoxContentLayout(lc, box))
	}
	return result
}

// checkBuilder stops the layout when the builder of the boxes is past its
// limit.
func (lc *layoutContext) checkBuilder() {
	if lc.builder.err != nil {
		lc.abort(lc.builder.err)
	}
}

// marginBoxContentLayout is margin_box_content_layout in
// weasyprint/layout/page.py: it lays out a margin box’s content once the
// box has dimensions. Upstream also takes the page, which it does not use.
func marginBoxContentLayout(lc *layoutContext, box *Box) *Box {
	var positionedBoxes []*Box
	result := blockContainerLayout(lc, box, math.Inf(-1), nil, true,
		&positionedBoxes, &positionedBoxes, nil, nil, nil, false, noMaxLines)
	// Port: upstream asserts that resume_at is None, and that the box is
	// not None. The port keeps the box without its content when the
	// layout gives none.
	if result.box == nil {
		box = box.CopyWithChildren(nil)
	} else {
		box = result.box
	}
	for i := 0; i < len(positionedBoxes); i++ {
		lc.spend(1)
		absoluteLayout(lc, positionedBoxes[i], box, &positionedBoxes, 0, nil)
	}

	verticalAlign := box.Style.Get("vertical_align")
	// Every other value is read as 'top', ie. no change.
	if isStrIn(verticalAlign, "middle", "bottom") && len(box.Children) > 0 {
		firstChild := box.Children[0]
		lastChild := box.Children[len(box.Children)-1]
		top := firstChild.PositionY
		bottom := lastChild.PositionY + lastChild.MarginHeight()
		contentHeight := bottom - top
		offset := box.Height - contentHeight
		if isStr(verticalAlign, "middle") {
			offset /= 2
		}
		for _, child := range box.Children {
			child.Translate(0, offset, false)
		}
	}
	return box
}

// pageWidthOrHeight is page_width_or_height in weasyprint/layout/page.py.
// It takes an oriented box and sets either width, margin-left and
// margin-right, or height, margin-top and margin-bottom.
//
// "The width and horizontal margins of the page box are then calculated
// exactly as for a non-replaced block element in normal flow. The height
// and vertical margins of the page box are calculated analogously (instead
// of using the block height formulas). In both cases if the values are
// over-constrained, instead of ignoring any margins, the containing block
// is resized to coincide with the margin edges of the page box."
//
// https://drafts.csswg.org/css-page-3/#page-box-page-rule
// https://www.w3.org/TR/CSS21/visudet.html#blockwidth
func pageWidthOrHeight(box *orientedBox, containingBlockSize float64) {
	remaining := containingBlockSize - box.paddingPlusBorder
	switch {
	case box.inner == Auto:
		if box.marginA == Auto {
			box.marginA = 0
		}
		if box.marginB == Auto {
			box.marginB = 0
		}
		box.inner = remaining - box.marginA - box.marginB
	case box.marginA == Auto && box.marginB == Auto:
		box.marginA = (remaining - box.inner) / 2
		box.marginB = box.marginA
	case box.marginA == Auto:
		box.marginA = remaining - box.inner - box.marginB
	case box.marginB == Auto:
		box.marginB = remaining - box.inner - box.marginA
	}
	box.restoreBoxAttributes()
}

// pageWidth is page_width in weasyprint/layout/page.py, with the min and
// max widths.
func pageWidth(box *Box, lc *layoutContext, containingBlockWidth float64) {
	handleMinMaxWidth(box, func() struct{} {
		pageWidthOrHeight(newHorizontalBox(lc, box), containingBlockWidth)
		return struct{}{}
	})
}

// pageHeight is page_height in weasyprint/layout/page.py, with the min and
// max heights.
func pageHeight(box *Box, lc *layoutContext, containingBlockHeight float64) {
	handleMinMaxHeight(box, func() struct{} {
		pageWidthOrHeight(newVerticalBox(lc, box), containingBlockHeight)
		return struct{}{}
	})
}

// setPageTypeStyles is set_page_type_computed_styles in
// weasyprint/layout/page.py. It sets the style for page types and
// pseudo-types matching pageType.
//
// Port: the name differs from upstream, because the tests of stage A have
// a helper of that name.
func setPageTypeStyles(lc *layoutContext, pageType PageType, html *HTML) {
	styleFor := lc.styleFor
	styleFor.AddPageDeclarations(pageType)

	// Apply style for page
	// @page inherits from the root element:
	// https://lists.w3.org/Archives/Public/www-style/2012Jan/1164.html
	root := html.EtreeElement
	if err := styleFor.SetComputedStyles(pageType, root, root, "", html.BaseURL, nil); err != nil {
		lc.abort(err)
	}

	// Apply style for page pseudo-elements (margin boxes)
	key := pageType.ownerKey()
	for _, cascaded := range styleFor.CascadedKeys() {
		if cascaded.Pseudo == "" || cascaded.Owner.ownerKey() != key {
			continue
		}
		// The pseudo-element inherits from the element.
		if err := styleFor.SetComputedStyles(cascaded.Owner, cascaded.Owner, root, cascaded.Pseudo, html.BaseURL, nil); err != nil {
			lc.abort(err)
		}
	}
}

// includesResumeAt is _includes_resume_at in weasyprint/layout/page.py.
func includesResumeAt(resumeAt, pageGroupResumeAt skipStack) bool {
	for {
		if len(pageGroupResumeAt) == 0 {
			return true
		}
		pageChildIndex, pageChildResumeAt := pageGroupResumeAt.item()
		childResumeAt, ok := resumeAt[pageChildIndex]
		if resumeAt == nil || !ok {
			return false
		}
		if pageChildResumeAt == nil {
			return true
		}
		resumeAt, pageGroupResumeAt = childResumeAt, pageChildResumeAt
	}
}

// updatePageGroups is _update_page_groups in weasyprint/layout/page.py. It
// updates the page groups, and returns the name of the page.
func updatePageGroups(pageGroups *[]pageGroup, resumeAt skipStack, next nextPage, root *Box, blank bool) Value {
	// https://www.w3.org/TR/css-gcpm-3/#document-sequence-selectors

	// Remove or increment page groups.
	kept := (*pageGroups)[:0]
	for _, group := range *pageGroups {
		if includesResumeAt(resumeAt, group.resumeAt) {
			group.index++
			kept = append(kept, group)
		}
	}
	*pageGroups = kept

	// Add page groups.
	if !blank {
		if (len(resumeAt) > 0 && next.brk == "any") || !truthy(next.page) {
			// We don’t have a forced page break or a named page.
			return next.page
		}
		if n := len(*pageGroups); n > 0 && Equal((*pageGroups)[n-1].name, next.page) {
			// We’re already in an element whose page name is the next page
			// name.
			return next.page
		}
	}

	// Find the box that has the named page. It is a first in-flow child of
	// the element corresponding to resume_at.

	// Find element corresponding to resume_at.
	pageGroupResumeAt := resumeAt.clone()
	if len(pageGroupResumeAt) == 0 {
		pageGroupResumeAt = skipStack{0: nil}
	}
	currentResumeAt := pageGroupResumeAt
	currentElement := root
	var parentElement *Box
	var childIndex int
	for {
		var childResumeAt skipStack
		childIndex, childResumeAt = currentResumeAt.item()
		parentElement = currentElement
		if childIndex < 0 || childIndex >= len(currentElement.Children) {
			// Port: upstream raises IndexError for a resume_at that does
			// not lead to a child, such as the position in a text box.
			return next.page
		}
		currentElement = currentElement.Children[childIndex]
		if childResumeAt == nil {
			break
		}
		currentResumeAt = childResumeAt
	}

	if blank {
		// Page is blank, don’t create a new page group and return parent’s
		// page name.
		return parentElement.Style.Get("page")
	}

	// Find the descendant with named page.
	for {
		if Equal(currentElement.Style.Get("page"), next.page) {
			*pageGroups = append(*pageGroups, pageGroup{next.page, 0, pageGroupResumeAt})
			return next.page
		}
		if !currentElement.IsParent() {
			// Shouldn’t happen.
			return next.page
		}
		found := false
		for i, child := range currentElement.Children {
			if !child.IsInNormalFlow() {
				continue
			}
			currentResumeAt[childIndex] = skipStack{i: nil}
			currentResumeAt = currentResumeAt[childIndex]
			childIndex = i
			currentElement = child
			found = true
			break
		}
		if !found {
			// Shouldn’t happen.
			return next.page
		}
	}
}

// makePageGrown is make_page in weasyprint/layout/page.py: it returns one
// laid out page without margin boxes, and where the next page resumes.
//
// It starts with the initial values from lc.pageMaker[index]. The
// resulting values, used as initial values for the next page, are stored
// in lc.pageMaker.
//
// Port: grow makes the top and the bottom margins of the page larger, for
// -ivy-margin-box-fit. See makePage.
func makePageGrown(index int, lc *layoutContext, rootBox *Box, html *HTML, grow pageGrowth) (*Box, skipStack) {
	pageNumber := index + 1
	pageMakers := lc.pageMaker
	pageIsEmpty := true
	pageData := pageMakers[index]
	var adjoiningMargins []float64
	var positionedBoxes []*Box // Mixed absolute and fixed

	// Apply and get style for current page.
	var nextPageSide string
	switch pageData.nextPage.brk {
	case "left", "right":
		nextPageSide = pageData.nextPage.brk
	case "recto", "verso":
		directionLTR := isStr(rootBox.Style.Get("direction"), "ltr")
		breakVerso := pageData.nextPage.brk == "verso"
		if directionLTR != breakVerso {
			nextPageSide = "right"
		} else {
			nextPageSide = "left"
		}
	}
	blank := (nextPageSide == "left" && pageData.rightPage) ||
		(nextPageSide == "right" && !pageData.rightPage) ||
		(len(lc.reportedFootnotes) > 0 && pageData.resumeAt == nil)
	side := "left"
	if pageData.rightPage {
		side = "right"
	}
	pageGroups := clonePageGroups(pageData.pageGroups)
	name := updatePageGroups(&pageGroups, pageData.resumeAt, pageData.nextPage, rootBox, blank)
	groups := make([]PageGroup, len(pageGroups))
	for i, group := range pageGroups {
		groups[i] = PageGroup{Name: str(group.name), Index: group.index}
	}
	pageType := PageType{Side: side, Blank: blank, Name: str(name), Index: index, Groups: groups}
	setPageTypeStyles(lc, pageType, html)
	style := lc.styleFor.Get(pageType, "")
	if style == nil {
		// Port: set_page_type_computed_styles always gives a style
		// upstream.
		lc.abort(fmt.Errorf("%w: page without a style", ivy.ErrFormat))
	}

	// Propagate overflow from the root or body.
	style.Set("overflow", String(rootBox.ViewportOverflow))

	// Create and setup page box.
	page := NewPageBox(pageType, style)
	deviceSize, _ := page.Style.Get("size").(Tuple)
	var cbWidth, cbHeight float64
	if len(deviceSize) == 2 {
		cbWidth, cbHeight = num(deviceSize[0]), num(deviceSize[1])
	} else {
		// Port: upstream fails to unpack a size that is not a pair. The
		// port takes the initial size.
		initial, _ := InitialValues["size"].(Tuple)
		if len(initial) == 2 {
			cbWidth, cbHeight = num(initial[0]), num(initial[1])
		}
	}
	resolvePercentages(page, sizeBlock(cbWidth, cbHeight))
	page.PositionX = 0
	page.PositionY = 0
	pageWidth(page, lc, cbWidth)
	pageHeight(page, lc, cbHeight)
	grow.apply(page)
	initialContainingBlock := page
	var previousResumeAt skipStack
	if blank {
		previousResumeAt = pageData.resumeAt
		rootBox = rootBox.CopyWithChildren([]*Box{})
	}

	// Setup context and create root formatting context.
	// https://www.w3.org/TR/css-display-4/#root
	// Port: upstream asserts that the root box is a block-level box.
	lc.createBlockFormattingContext(nil)
	lc.currentPage = pageNumber
	lc.currentPageFootnotes = nil
	lc.forcedBreak = pageData.nextPage.brk != "any" || truthy(pageData.nextPage.page)
	if pageNumber == 1 {
		sizes := &lc.styleFor.Context.InitialPageSizes
		sizes.Box = &[2]float64{cbWidth, cbHeight}
		sizes.Area = &[2]float64{page.Width, page.Height}
	}
	lc.pageBottom = page.ContentBoxY() + page.Height
	previousPageOutOfFlow := lc.brokenOutOfFlow
	lc.brokenOutOfFlow = nil

	// Create footnote area.
	footnoteAreaStyle := lc.styleFor.Get(pageType, "@footnote")
	if footnoteAreaStyle == nil {
		footnoteAreaStyle = style.AnonymousStyle()
	}
	footnoteArea := NewFootnoteAreaBox(page, footnoteAreaStyle)
	lc.currentFootnoteArea = footnoteArea
	resolvePercentages(footnoteArea, page)
	footnoteArea.PositionX = page.ContentBoxX()
	footnoteArea.PositionY = page.ContentBoxY() + page.Height
	reportedFootnotes := lc.reportedFootnotes
	lc.reportedFootnotes = nil
	for i, reportedFootnote := range reportedFootnotes {
		lc.footnotes = append(lc.footnotes, reportedFootnote)
		overflow := lc.layoutFootnote(reportedFootnote)
		if overflow && i != 0 {
			lc.reportFootnote(reportedFootnote)
			lc.reportedFootnotes = slices.Clone(reportedFootnotes[i:])
			break
		}
	}

	// Create note area.
	noteAreaStyle := lc.styleFor.Get(pageType, "@note-area")
	if noteAreaStyle == nil {
		noteAreaStyle = style.AnonymousStyle()
	}
	noteArea := blockify(makeBox("@note-area", noteAreaStyle, nil, nil))
	noteArea.PositionX = page.ContentBoxX()
	noteArea.PositionY = page.ContentBoxY()
	quoteDepth := pageData.quoteDepth
	counterValues := copyCounterValues(pageData.counterValues)
	if !isStr(noteArea.Style.Get("content"), "normal") {
		noteArea.Children = lc.builder.contentToBoxes(
			noteArea.Style, noteArea, &quoteDepth, counterValues, lc, page)
		lc.checkBuilder()
		noteArea = createAnonymousBoxes(lc.builder, noteArea)
		lc.checkBuilder()
	}

	// Do the root box layout.
	rootBox.PositionX = page.ContentBoxX()
	rootBox.PositionY = page.ContentBoxY()
	switch {
	case noteArea.IsInNormalFlow():
		result := blockLevelLayout(lc, noteArea, 0, nil, initialContainingBlock,
			pageIsEmpty, &positionedBoxes, &positionedBoxes, adjoiningMargins,
			nil, nil, false, noMaxLines)
		// Port: the layout of the note area always gives a box upstream.
		if result.box != nil {
			noteArea = result.box
		}
		if len(noteArea.Children) > 0 {
			rootBox.PositionY = noteArea.PositionY + noteArea.MarginHeight()
		}
	case noteArea.IsFloated():
		if box, _ := floatLayout(lc, noteArea, initialContainingBlock,
			&positionedBoxes, &positionedBoxes, 0, nil); box != nil {
			noteArea = box
		}
		if len(noteArea.Children) > 0 {
			// Port: upstream stores the note area itself, not a list of
			// shapes, and the layout of the root box then fails. The port
			// stores a list that holds the note area.
			lc.setExcludedShapesFor(rootBox, []*Box{noteArea})
		}
	default:
		if box, _ := absoluteBoxLayout(lc, noteArea, page, &positionedBoxes, 0, nil); box != nil {
			noteArea = box
		}
	}

	// Display out-of-flow boxes broken on the previous page.
	// TODO: we shouldn’t separate broken in-flow and out-of-flow layout.
	adjoiningMargins = nil
	positionedBoxes = nil // Mixed absolute and fixed
	var outOfFlowBoxes []*Box
	excludedShapes := map[*Box][]*Box{}
	var excludedShapesOrder []*Box
	for _, item := range previousPageOutOfFlow {
		lc.spend(1)
		box, containingBlock, contextBox, skip := item.box, item.containingBlock, item.rootBox, item.resumeAt
		if contextBox != nil {
			lc.createBlockFormattingContext(contextBox)
		}
		box.PositionY = rootBox.ContentBoxY()
		var outOfFlowBox *Box
		var outOfFlowResumeAt skipStack
		if box.IsFloated() {
			outOfFlowBox, outOfFlowResumeAt = floatLayout(lc, box, containingBlock,
				&positionedBoxes, &positionedBoxes, 0, skip)
			if _, ok := excludedShapes[contextBox]; !ok {
				excludedShapesOrder = append(excludedShapesOrder, contextBox)
			}
			excludedShapes[contextBox] = append(excludedShapes[contextBox], outOfFlowBox)
		} else {
			// Port: upstream asserts that the box is absolutely
			// positioned.
			outOfFlowBox, outOfFlowResumeAt = absoluteBoxLayout(lc, box, containingBlock,
				&positionedBoxes, 0, skip)
		}
		if outOfFlowBox != nil {
			outOfFlowBoxes = append(outOfFlowBoxes, outOfFlowBox)
		}
		pageIsEmpty = false
		if len(outOfFlowResumeAt) > 0 {
			lc.addBrokenOutOfFlow(outOfFlowBox, box, containingBlock, outOfFlowResumeAt)
		}
		if contextBox != nil {
			lc.finishBlockFormattingContext(nil)
		}
	}
	for _, contextBox := range excludedShapesOrder {
		lc.setExcludedShapesFor(contextBox, excludedShapes[contextBox])
	}

	// Display in-flow content.
	initialRootBox := rootBox
	initialResumeAt := pageData.resumeAt
	previousRunningElements := runningElementsOfPage(lc, pageNumber)
	for _, runningElements := range lc.runningElements {
		delete(runningElements, pageNumber)
	}
	for _, name := range sortedKeys(previousRunningElements) {
		for _, element := range previousRunningElements[name].boxes {
			if element.IsNote() {
				lc.renderedNotes = append(lc.renderedNotes, element)
			}
		}
	}
	result := blockLevelLayout(lc, rootBox, 0, pageData.resumeAt, initialContainingBlock,
		pageIsEmpty, &positionedBoxes, &positionedBoxes, adjoiningMargins,
		nil, nil, false, noMaxLines)
	rootBox, resumeAt, next := result.box, result.resumeAt, result.nextPage
	if rootBox == nil {
		// In-flow page rendering didn’t progress, only out-of-flow did.
		// Render empty box at skip_stack and force fragmentation to make
		// the root box and its descendants cover the whole page height.
		//
		// Port: upstream asserts that the page is not empty. A page that
		// is empty and makes no progress ends the document, see below.
		box := initialRootBox.DeepCopy()
		initialRootBox = box
		parent := box
		skip := initialResumeAt
		for len(skip) == 1 {
			var index int
			index, skip = skip.item()
			if index < 0 || index >= len(box.Children) {
				// Port: upstream raises IndexError.
				break
			}
			box, parent = box.Children[index], box
		}
		parent.Children = nil
		parent.ForceFragmentation = true
		result = blockLevelLayout(lc, initialRootBox, 0, initialResumeAt, initialContainingBlock,
			true, &positionedBoxes, &positionedBoxes, adjoiningMargins,
			nil, nil, false, noMaxLines)
		rootBox = result.box
		resumeAt = initialResumeAt
		if rootBox == nil || pageIsEmpty {
			// Port: the layout makes no progress at all. The port ends the
			// document with this page, instead of making the same page
			// again.
			if rootBox == nil {
				rootBox = initialRootBox.CopyWithChildren(nil)
			}
			resumeAt = nil
		}
	}
	rootBox.Children = append(outOfFlowBoxes, rootBox.Children...)
	runningElements := runningElementsOfPage(lc, pageNumber)
	if !runningElementsEqual(previousRunningElements, runningElements) {
		for name, previousRunnings := range previousRunningElements {
			if runnings := runningElements[name]; len(runnings.boxes) > 0 {
				if !previousRunnings.ok || len(previousRunnings.boxes) > len(runnings.boxes) {
					lc.pageMaker[index].contentChanged = true
				}
			}
		}
	}

	// Do the layout of the footnote area.
	footnoteArea = createAnonymousBoxes(lc.builder, footnoteArea.DeepCopy())
	lc.checkBuilder()
	if box := blockLevelLayout(lc, footnoteArea, math.Inf(-1), nil, footnoteArea.Page,
		true, &positionedBoxes, &positionedBoxes, nil, nil, nil, false, noMaxLines).box; box != nil {
		footnoteArea = box
	}
	footnoteArea.Translate(0, -footnoteArea.MarginHeight(), false)

	// Do the layout of fixed boxes.
	var fixedBoxes []*Box
	for _, placeholder := range positionedBoxes {
		if isStr(placeholder.Style.Get("position"), "fixed") {
			fixedBoxes = append(fixedBoxes, placeholder.placeholderBox())
		}
	}
	page.fixedBoxes = fixedBoxes
	for i := 0; i < len(positionedBoxes); i++ {
		lc.spend(1)
		absoluteLayout(lc, positionedBoxes[i], page, &positionedBoxes, 0, nil)
	}

	lc.finishBlockFormattingContext(nil)

	page.Children = []*Box{rootBox, footnoteArea, noteArea}

	// Update page counter values.
	counterScopes := cloneCounterScopes(pageData.counterScopes)
	if len(counterScopes) == 0 {
		// Port: the page makers always have a scope upstream.
		counterScopes = []map[string]bool{{}}
	}
	updatePageCounters(style, lc, page, counterValues, counterScopes, index)

	if blank {
		resumeAt = previousResumeAt
		next = pageMakers[index].nextPage
	}

	// Check whether we need to append or update the next page_maker item.
	newPage := len(lc.pageMaker) <= index+1
	contentChanged := !newPage && !skipStackEqual(lc.pageMaker[index+1].resumeAt, resumeAt)
	if newPage || contentChanged {
		// Reset remake state and force page re-creation for old pages.
		data := &pageMaker{
			resumeAt:       resumeAt,
			nextPage:       next,
			rightPage:      !pageData.rightPage,
			quoteDepth:     quoteDepth,
			counterValues:  counterValues,
			counterScopes:  counterScopes,
			pageGroups:     pageGroups,
			contentChanged: contentChanged,
		}
		if newPage {
			lc.pageMaker = append(lc.pageMaker, data)
		} else {
			lc.pageMaker[index+1] = data
		}
	}

	return page, resumeAt
}

// pageRunning is the value of the running elements of a name on a page,
// running_elements.get(page_number) of upstream: the boxes, and ok false
// for None.
type pageRunning struct {
	boxes []*Box
	ok    bool
}

// runningElementsOfPage returns the running elements of a page, by name,
// for each name of the running elements.
func runningElementsOfPage(lc *layoutContext, pageNumber int) map[string]pageRunning {
	result := make(map[string]pageRunning, len(lc.runningElements))
	for name, pages := range lc.runningElements {
		boxes, ok := pages[pageNumber]
		result[name] = pageRunning{slices.Clone(boxes), ok}
	}
	return result
}

// runningElementsEqual compares two results of runningElementsOfPage, as
// the operator == of Python does for the dicts.
func runningElementsEqual(a, b map[string]pageRunning) bool {
	if len(a) != len(b) {
		return false
	}
	for name, runningA := range a {
		runningB, ok := b[name]
		if !ok || runningA.ok != runningB.ok || !slices.Equal(runningA.boxes, runningB.boxes) {
			return false
		}
	}
	return true
}

// sortedKeys returns the keys of a map in sorted order.
//
// Port: upstream iterates on the dicts in the order of insertion. The port
// sorts the names, so that the order does not depend on the map.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// pageIter is the generator make_pages in weasyprint/layout/page.py: it
// yields the laid out pages without margin boxes, and makes again the pages
// that need it.
type pageIter struct {
	lc    *layoutContext
	root  *Box
	html  *HTML
	pages []*Box

	i                 int
	resumeAt          skipStack
	reportedFootnotes bool
	done              bool
}

// newPageIter starts make_pages. pages are the pages of the previous pass,
// or nil for the first pass.
func newPageIter(lc *layoutContext, root *Box, html *HTML, pages []*Box) *pageIter {
	lc.renderedNotes = nil
	return &pageIter{lc: lc, root: root, html: html, pages: pages}
}

// next returns the next page, and false at the end of the pages.
func (it *pageIter) next() (*Box, bool) {
	if it.done {
		return nil, false
	}
	lc := it.lc
	if it.i != 0 && it.resumeAt == nil && !it.reportedFootnotes {
		// Throw away obsolete pages and content.
		lc.pageMaker = lc.pageMaker[:min(it.i+1, len(lc.pageMaker))]
		lc.brokenOutOfFlow = nil
		lc.reportedFootnotes = nil
		it.done = true
		return nil, false
	}
	pageData := lc.pageMaker[it.i]
	var page *Box
	// Port: upstream reads a page of the previous pass that does not
	// exist when the new pass makes more pages. The port makes it.
	if len(it.pages) == 0 || pageData.contentChanged || pageData.pagesWanted || it.i >= len(it.pages) || it.i+1 >= len(lc.pageMaker) {
		pageData.contentChanged, pageData.pagesWanted = false, false
		pageData.anchors, pageData.counters = nil, nil
		page, it.resumeAt = makePage(it.i, lc, it.root, it.html)
		it.reportedFootnotes = len(lc.reportedFootnotes) > 0
	} else {
		it.resumeAt = lc.pageMaker[it.i+1].resumeAt
		it.reportedFootnotes = false
		page = it.pages[it.i]
	}
	it.i++
	return page, true
}
