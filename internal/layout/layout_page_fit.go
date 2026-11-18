// This file is not a port. It adds the property -ivy-margin-box-fit to the
// pages: the top and the bottom margins of a page grow to hold the content
// of its margin boxes, as a word processor pushes the text of a page below
// a header that is taller than the top margin.

package layout

import (
	"math"
	"slices"
	"strings"
)

// maxMarginGrowth is the part of the height of a page that a margin grows
// to at most.
const maxMarginGrowth = 0.4

// pageGrowth is what the top and the bottom margins of a page grow by, in
// pixels.
type pageGrowth struct {
	top, bottom float64
}

// apply makes the margins of a page box larger, and its height smaller.
func (g pageGrowth) apply(page *Box) {
	if g.top <= 0 && g.bottom <= 0 {
		return
	}
	page.MarginTop += g.top
	page.MarginBottom += g.bottom
	page.Height = max(page.Height-g.top-g.bottom, 0)
}

// pageState is the state of the layout context that make_page changes,
// which makePage restores to make a page again.
type pageState struct {
	brokenOutOfFlow   []brokenOutOfFlow
	footnotes         []*Box
	reportedFootnotes []*Box
	renderedNotes     []*Box
	running           map[string][]*Box
	// makers is the number of page makers, next the page maker after the
	// page, or nil, and data a copy of the page maker of the page.
	makers int
	next   *pageMaker
	data   pageMaker
}

// savePageState returns the state of the layout context before the page of
// index i.
func (lc *layoutContext) savePageState(i int) *pageState {
	s := &pageState{
		brokenOutOfFlow:   slices.Clone(lc.brokenOutOfFlow),
		footnotes:         slices.Clone(lc.footnotes),
		reportedFootnotes: slices.Clone(lc.reportedFootnotes),
		renderedNotes:     slices.Clone(lc.renderedNotes),
		running:           map[string][]*Box{},
		makers:            len(lc.pageMaker),
		data:              *lc.pageMaker[i],
	}
	if i+1 < len(lc.pageMaker) {
		s.next = lc.pageMaker[i+1]
	}
	for name, pages := range lc.runningElements {
		if boxes, ok := pages[i+1]; ok {
			s.running[name] = slices.Clone(boxes)
		}
	}
	s.data.anchors = slices.Clone(s.data.anchors)
	s.data.counters = slices.Clone(s.data.counters)
	return s
}

// restore puts the layout context back in the state before the page of
// index i.
func (s *pageState) restore(lc *layoutContext, i int) {
	lc.brokenOutOfFlow = slices.Clone(s.brokenOutOfFlow)
	lc.footnotes = slices.Clone(s.footnotes)
	lc.reportedFootnotes = slices.Clone(s.reportedFootnotes)
	lc.renderedNotes = slices.Clone(s.renderedNotes)
	for name, pages := range lc.runningElements {
		delete(pages, i+1)
		if boxes, ok := s.running[name]; ok {
			pages[i+1] = slices.Clone(boxes)
		}
	}
	lc.pageMaker = lc.pageMaker[:min(s.makers, len(lc.pageMaker))]
	if s.next != nil && i+1 < len(lc.pageMaker) {
		lc.pageMaker[i+1] = s.next
	}
	data := s.data
	data.anchors = slices.Clone(data.anchors)
	data.counters = slices.Clone(data.counters)
	*lc.pageMaker[i] = data
}

// makePage is make_page in weasyprint/layout/page.py, see makePageGrown.
//
// Port: when the page has -ivy-margin-box-fit: grow, and the content of its
// margin boxes at the top or at the bottom is taller than the margin, the
// page is made again with larger margins. A margin grows to at most 40% of
// the height of the page.
func makePage(index int, lc *layoutContext, rootBox *Box, html *HTML) (*Box, skipStack) {
	state := lc.savePageState(index)
	page, resumeAt := makePageGrown(index, lc, rootBox, html, pageGrowth{})
	if !isStr(page.Style.Get("margin_box_fit"), "grow") || index+1 >= len(lc.pageMaker) {
		return page, resumeAt
	}
	grow := marginBoxGrowth(lc, page, index)
	if grow.top <= 0 && grow.bottom <= 0 {
		return page, resumeAt
	}
	state.restore(lc, index)
	return makePageGrown(index, lc, rootBox, html, grow)
}

// marginBoxGrowth returns what the margins of a page made by
// makePageGrown must grow by to hold the content of its margin boxes at the
// top and at the bottom.
func marginBoxGrowth(lc *layoutContext, page *Box, index int) pageGrowth {
	pageData := lc.pageMaker[index+1]
	quoteDepth := pageData.quoteDepth
	boxes := makeMarginBoxes(lc, page.Copy(), &quoteDepth,
		cloneCounterValues(pageData.counterValues), cloneCounterScopes(pageData.counterScopes))
	var top, bottom float64
	for _, box := range boxes {
		isTop := strings.HasPrefix(box.extra().AtKeyword, "@top-") && !strings.HasSuffix(box.extra().AtKeyword, "-corner")
		isBottom := strings.HasPrefix(box.extra().AtKeyword, "@bottom-") && !strings.HasSuffix(box.extra().AtKeyword, "-corner")
		if !isTop && !isBottom {
			continue
		}
		minY, maxY := math.Inf(1), math.Inf(-1)
		var extent func(*Box)
		extent = func(b *Box) {
			for _, child := range b.Children {
				// The content of element() keeps the position running().
				if child.IsFloated() || child.IsAbsolutelyPositioned() {
					continue
				}
				minY = min(minY, child.PositionY)
				maxY = max(maxY, child.PositionY+child.MarginHeight())
				// The margins of the last child of a block go through its
				// bottom, as Word counts the space after the last
				// paragraph of a header.
				if child.IsBlockContainer() && child.Kind != KindLine {
					extent(child)
				}
			}
		}
		extent(box)
		if minY > maxY {
			continue
		}
		need := box.MarginHeight() - box.Height + maxY - minY
		if isTop {
			top = max(top, need)
		} else {
			bottom = max(bottom, need)
		}
	}
	height := page.MarginHeight()
	limit := func(need, margin float64) float64 {
		return max(min(need, max(margin, maxMarginGrowth*height))-margin, 0)
	}
	return pageGrowth{top: limit(top, page.MarginTop), bottom: limit(bottom, page.MarginBottom)}
}
