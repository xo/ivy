// This file is not a port. It adds the tab stops of the word processors to
// the layout, with the property -ivy-tab-stops. Upstream only has
// tab-size, which puts the tabs of a text at multiples of one distance
// from the start of the text box.

package layout

import (
	"cmp"
	"math"
	"slices"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// maxTabStops is the number of tab stops that a value of -ivy-tab-stops
// can hold. Word holds at most 64 custom tab stops in a paragraph.
const maxTabStops = 256

// maxTabsInText is the number of tabs of one text that become text boxes
// of their own. The tabs after it keep the width of tab-size.
const maxTabsInText = 1024

// tabStop is a tab stop of -ivy-tab-stops.
type tabStop struct {
	// pos is the position of the stop from the start edge of the margin
	// box of the block container of the line, in pixels.
	pos float64
	// align is "left", "right", "center" or "decimal".
	align string
	// leader is the text that fills the space of the tab, or "".
	leader string
}

// validateTabStops validates -ivy-tab-stops: 'none', 'auto', or a comma
// separated list of stops. A stop is a length, an optional alignment
// 'left', 'right', 'center' or 'decimal', and an optional string, the
// leader.
func validateTabStops(tokens []tinycss2.Node, _ string) (Value, error) {
	if keyword := getSingleKeyword(tokens); keyword == "none" || keyword == "auto" {
		return String(keyword), nil
	}
	parts := splitOnComma(tokens)
	if len(parts) > maxTabStops {
		return nil, nil
	}
	stops := make(Tuple, 0, len(parts))
	for _, part := range parts {
		var pos Value
		var align, leader Value
		for _, token := range removeWhitespace(part) {
			if d, ok := getLength(token, true, false).(Dimension); ok && pos == nil {
				pos = d
				continue
			}
			if keyword := getKeyword(token); in(keyword, "left", "right", "center", "decimal") && align == nil {
				align = String(keyword)
				continue
			}
			if s, ok := token.(*tinycss2.StringToken); ok && leader == nil && len(s.Value) <= 16 {
				leader = String(s.Value)
				continue
			}
			return nil, nil
		}
		if pos == nil {
			return nil, nil
		}
		if align == nil {
			align = String("left")
		}
		if leader == nil {
			leader = String("")
		}
		stops = append(stops, Tuple{pos, align, leader})
	}
	return stops, nil
}

// computeTabStops computes -ivy-tab-stops: the positions in pixels.
func computeTabStops(style *Style, name string, value Value) Value {
	t, ok := value.(Tuple)
	if !ok {
		return value
	}
	stops := make(Tuple, 0, len(t))
	for _, item := range t {
		stop, ok := item.(Tuple)
		if !ok || len(stop) != 3 {
			continue
		}
		d, ok := stop[0].(Dimension)
		if !ok {
			continue
		}
		stops = append(stops, Tuple{Float(toPixels(d, style, name, -1)), stop[1], stop[2]})
	}
	return stops
}

// hasTabStops reports whether the layout places the tabs of a style with
// the tab stops of -ivy-tab-stops.
func hasTabStops(style *Style) bool {
	return !isStr(style.Get("tab_stops"), "none") && !isStr(style.Get("direction"), "rtl")
}

// tabStopsOf returns the stops of -ivy-tab-stops of a style, in the order
// of their positions.
func tabStopsOf(style *Style) []tabStop {
	t, _ := style.Get("tab_stops").(Tuple)
	stops := make([]tabStop, 0, len(t))
	for _, item := range t {
		if stop, ok := item.(Tuple); ok && len(stop) == 3 {
			pos := num(stop[0])
			if math.IsNaN(pos) || math.IsInf(pos, 0) {
				continue
			}
			stops = append(stops, tabStop{pos: pos, align: str(stop[1]), leader: str(stop[2])})
		}
	}
	slices.SortStableFunc(stops, func(a, b tabStop) int { return cmp.Compare(a.pos, b.pos) })
	return stops
}

// nextTabStop returns the tab stop of a style after the position x from
// the start edge of the margin box of the block container. It is the
// first custom stop after x. After the last custom stop, it is the next
// multiple of tab-size, a default stop that aligns left.
func nextTabStop(style *Style, x float64) tabStop {
	const epsilon = 1e-6
	for _, stop := range tabStopsOf(style) {
		if stop.pos > x+epsilon {
			return stop
		}
	}
	layout := newTextLayout(style, 0, nil)
	layout.setTabs()
	width := max(layout.tabWidth, 1)
	return tabStop{pos: (math.Floor(x/width+epsilon) + 1) * width, align: "left"}
}

// splitTabs gives each tab of the text boxes of the descendants of box a
// text box of its own, when the style of the text has tab stops. The layout
// of the lines then places these boxes at the tab stops.
func splitTabs(box *Box) {
	if !box.IsParent() {
		return
	}
	var children []*Box
	for i, child := range box.Children {
		if child.Kind != KindText || !strings.Contains(child.Text, "\t") || !hasTabStops(child.Style) {
			splitTabs(child)
			if children != nil {
				children = append(children, child)
			}
			continue
		}
		if children == nil {
			children = append(make([]*Box, 0, len(box.Children)+2), box.Children[:i]...)
		}
		text := child.Text
		for n := 0; n < maxTabsInText; n++ {
			before, after, found := strings.Cut(text, "\t")
			if !found {
				break
			}
			if before != "" {
				children = append(children, child.CopyWithText(before))
			}
			tab := child.CopyWithText("\t")
			tab.isTab = true
			children = append(children, tab)
			text = after
		}
		if text != "" {
			children = append(children, child.CopyWithText(text))
		}
	}
	if children != nil {
		box.Children = children
	}
}

// placeTab sets the width of the text box of a tab that starts at the
// position x of the page, for the breaking of the line: the width to its
// stop for a stop that aligns left, and no width for another stop.
// alignTabs gives the tabs their final widths.
func (lc *layoutContext) placeTab(box *Box, x float64) {
	stop := nextTabStop(box.Style, x-lc.tabOrigin)
	box.ownExtra().tab = stop
	box.Width = 0
	if stop.align == "left" {
		box.Width = max(lc.tabOrigin+stop.pos-x, 0)
	}
}

// alignTabs gives each tab of a line its final width, from left to right,
// and moves the boxes after it by the change of its width. A tab goes to
// the next stop after its start. The text after the tab, up to the next
// tab or to the end of the line, starts at a stop that aligns left, ends
// at a stop that aligns right, has its center at a stop that centers, and
// has its first full stop at a decimal stop.
//
// The breaking of the line placed the tabs after a tab that does not align
// left too early, so alignTabs finds the stop of each tab again.
func alignTabs(line *Box, origin float64) {
	if line == nil {
		return
	}
	var leaves []*Box
	hasTab := false
	var collect func(*Box)
	collect = func(b *Box) {
		for _, child := range b.Children {
			if !child.IsInNormalFlow() {
				continue
			}
			if child.IsParent() && !child.IsAtomicInlineLevel() {
				collect(child)
			} else {
				leaves = append(leaves, child)
				hasTab = hasTab || child.isTab
			}
		}
	}
	collect(line)
	if !hasTab {
		return
	}
	for i, tab := range leaves {
		if !tab.isTab {
			continue
		}
		stop := nextTabStop(tab.Style, tab.PositionX-origin)
		tab.ownExtra().tab = stop
		width := origin + stop.pos - tab.PositionX
		if stop.align != "left" {
			start := tab.PositionX + tab.Width
			end, decimal := start, math.NaN()
			for _, leaf := range leaves[i+1:] {
				if leaf.isTab {
					break
				}
				end = max(end, leaf.PositionX+leaf.MarginWidth())
				if stop.align == "decimal" && math.IsNaN(decimal) && leaf.Kind == KindText {
					if k := strings.IndexByte(leaf.Text, '.'); k >= 0 {
						decimal = leaf.PositionX + textWidthTo(leaf, k)
					}
				}
			}
			switch {
			case stop.align == "center":
				width -= (end - start) / 2
			case stop.align == "decimal" && !math.IsNaN(decimal):
				width -= decimal - start
			default:
				width -= end - start
			}
		}
		width = max(width, 0)
		delta := width - tab.Width
		if delta == 0 {
			continue
		}
		tab.Width = width
		if shiftAfter(line, tab, delta) {
			line.Width += delta
		}
	}
}

// shiftAfter moves the boxes of parent that come after the box tab by dx,
// and makes the boxes that hold tab dx wider. It reports whether parent
// holds tab.
func shiftAfter(parent, tab *Box, dx float64) bool {
	found := false
	for _, child := range parent.Children {
		switch {
		case found:
			child.Translate(dx, 0, false)
		case child == tab:
			found = true
		case child.IsParent() && !child.IsAtomicInlineLevel() && shiftAfter(child, tab, dx):
			child.Width += dx
			found = true
		}
	}
	return found
}

// textWidthTo returns the width of the text of a text box up to the byte
// k of its text.
func textWidthTo(box *Box, k int) float64 {
	layout := newTextLayout(box.Style, box.JustificationSpacing, nil)
	layout.SetText(box.Text[:min(k, len(box.Text))], false)
	line, _, _ := layout.FirstLine()
	return line.Width
}

// drawTabLeader draws the leader of the text box of a tab: as many copies
// of the leader as the tab holds, which end at the end of the tab, on the
// baseline y.
func drawTabLeader(s *stream, textbox *Box, y float64) {
	leader := textbox.extra().tab.leader
	if leader == "" || textbox.Width <= 0 || textbox.TextLayout == nil {
		return
	}
	layout := newTextLayout(textbox.Style, 0, nil)
	layout.SetText(leader, false)
	line, _, _ := layout.FirstLine()
	if line.Width <= 0 {
		return
	}
	n := int(textbox.Width / line.Width)
	if n <= 0 {
		return
	}
	n = min(n, 4096)
	layout.SetText(strings.Repeat(leader, n), false)
	line, _, _ = layout.FirstLine()
	box := textbox.CopyWithText(layout.Text)
	box.TextLayout = layout
	box.isTab = false
	drawFirstLine(s, box, "clip", nil, textbox.PositionX+textbox.Width-line.Width, y)
}
