package chart

import (
	"image/color"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

// This file draws the treemap and the sunburst of Office 2016. Both show a
// tree of categories: each point of the series is a leaf, and the levels of
// its categories are the parents of the leaf. A treemap divides its plot
// into rectangles, and a sunburst divides rings into slices. Each top level
// of the tree takes a color of its own, and its children take the same
// color, as in Excel.

// maxTreeNodes is the number of the nodes of a tree.
const maxTreeNodes = maxPoints

// The sizes of the parts of a tree, in points.
const (
	// treeEdge is the width of the white line around each rectangle or
	// slice.
	treeEdge = 1.5
	// treePad is the gap between the side of a rectangle or of a banner and
	// its label.
	treePad = 3.6
	// bannerPad is the height of a banner of a treemap more than the height
	// of its text.
	bannerPad = 7.5
)

// bannerGray is the color of the banners of a treemap.
var bannerGray = color.NRGBA{0xa6, 0xa6, 0xa6, 255}

// treeNode is a node of the tree of a treemap or a sunburst.
type treeNode struct {
	name  string
	value float64
	// point is the number of the point of a leaf, or -1.
	point int
	// top is the number of the node of the top level that holds the node,
	// which gives its color.
	top      int
	children []*treeNode
	// index finds a child by its name.
	index map[string]*treeNode
}

// buildTree returns the tree of the points of a treemap or a sunburst. The
// levels of the categories go from the innermost, and the path of a point
// goes from its outermost level to its innermost level that is not empty.
// A point without a positive value is left out, as Excel does.
func buildTree(levels [][]string, vals []float64) *treeNode {
	root := &treeNode{point: -1, top: -1}
	nodes := 0
	for i, v := range vals {
		if !(v > 0) || math.IsInf(v, 0) || nodes >= maxTreeNodes {
			continue
		}
		path := make([]string, 0, len(levels))
		for l := len(levels) - 1; l >= 0; l-- {
			name := ""
			if i < len(levels[l]) {
				name = levels[l][i]
			}
			path = append(path, name)
		}
		for len(path) > 1 && strings.TrimSpace(path[len(path)-1]) == "" {
			path = path[:len(path)-1]
		}
		if len(path) == 0 {
			path = []string{""}
		}
		n := root
		for depth, name := range path {
			next := n.index[name]
			if next == nil {
				next = &treeNode{name: name, point: -1, top: n.top}
				if n == root {
					next.top = len(root.children)
				}
				if n.index == nil {
					n.index = map[string]*treeNode{}
				}
				n.index[name] = next
				n.children = append(n.children, next)
				nodes++
			}
			next.value += v
			if depth == len(path)-1 && next.point < 0 {
				next.point = i
			}
			n = next
		}
		root.value += v
	}
	return root
}

// sorted returns the children of a node from the largest value to the
// smallest, as Excel lays them out.
func (n *treeNode) sorted() []*treeNode {
	out := slices.Clone(n.children)
	slices.SortStableFunc(out, func(a, b *treeNode) int {
		switch {
		case a.value > b.value:
			return -1
		case a.value < b.value:
			return 1
		}
		return 0
	})
	return out
}

// depth returns the number of the levels of a node below it.
func (n *treeNode) depth() int {
	d := 0
	for _, c := range n.children {
		d = max(d, c.depth()+1)
	}
	return d
}

// treeColor returns the color of a node: the color of its point, or the
// color of its node of the top level.
func (cv *canvas) treeColor(s *series, n *treeNode) color.NRGBA {
	if col, ok := s.points[n.point]; ok && n.point >= 0 {
		return col
	}
	return cv.c.palette(n.top)
}

// contrast returns the color of a label on a fill: white on a dark fill,
// else the color of the labels of the chart.
func contrast(fill color.NRGBA) color.NRGBA {
	l := 0.299*float64(fill.R) + 0.587*float64(fill.G) + 0.114*float64(fill.B)
	if l < 170 {
		return color.NRGBA{255, 255, 255, 255}
	}
	return labelGray
}

// treeLabel returns the label of a node of a tree, or "".
func treeLabel(l labels, s *series, n *treeNode) string {
	if !l.show() {
		return ""
	}
	return labelText(l, seriesName(s), n.name, n.value, math.NaN(), s.format)
}

// fitLines breaks a text into lines no wider than maxW: at its spaces, and
// a word that is wider than maxW between its characters. It returns false
// when the lines are higher than maxH.
func (cv *canvas) fitLines(s string, st textStyle, maxW, maxH float64) ([]string, bool) {
	if s == "" || !(maxW > 0) || !(maxH > 0) || !(st.size > 0) {
		return nil, false
	}
	rows := math.Floor(maxH / (st.size * lineHeight))
	if rows < 1 || cv.t.width(s, st) > maxW*rows*1.5+st.size {
		// the text cannot fit in the lines that maxH holds
		return nil, false
	}
	var lines []string
	for _, line := range cv.t.wrap(s, st, maxW) {
		for cv.t.width(line, st) > maxW && utf8.RuneCountInString(line) > 1 {
			// the longest start of the word that fits, of one character
			// at least
			starts := []int{}
			for i := range line {
				if i > 0 {
					starts = append(starts, i)
				}
			}
			k, _ := slices.BinarySearchFunc(starts, maxW, func(i int, w float64) int {
				if cv.t.width(line[:i], st) > w {
					return 1
				}
				return -1
			})
			cut := starts[0]
			if k > 0 {
				cut = starts[k-1]
			}
			lines = append(lines, line[:cut])
			line = line[cut:]
			if float64(len(lines)) > rows {
				return nil, false
			}
		}
		if cv.t.width(line, st) > maxW {
			return nil, false
		}
		lines = append(lines, line)
	}
	if float64(len(lines)) > rows {
		return nil, false
	}
	return lines, true
}

// treeText returns the style of the labels of a tree on a fill.
func (cv *canvas) treeText(p *plot, s *series, fill color.NRGBA) textStyle {
	st := s.labels.text
	if !p.labelColor {
		st.color, st.hasColor = contrast(fill), true
	}
	return st
}

// tile is a rectangle of a treemap, for a leaf or for a banner.
type tile struct {
	b      box
	n      *treeNode
	banner bool
}

// treemap draws a treemap into the box b, with the rectangles of
// layoutTreemap, white lines between them, and the labels of the leaves
// and of the top level.
func (cv *canvas) treemap(b box) {
	p := cv.c.plots[0]
	s := p.series[0]
	root := p.tree
	if root == nil || len(root.children) == 0 || !(root.value > 0) {
		cv.frame(b)
		return
	}
	l := s.labels
	bh := 0.0
	if p.parents == "banner" && l.show() {
		bh = l.text.size*lineHeight + bannerPad
	}
	tiles := layoutTreemap(root, b, bh)
	white := color.NRGBA{255, 255, 255, 255}
	for _, t := range tiles {
		fill := cv.treeColor(s, t.n)
		if t.banner {
			fill = bannerGray
		}
		cv.rect(t.b, fill, white, treeEdge)
	}
	if !l.show() {
		return
	}
	for _, t := range tiles {
		fill := cv.treeColor(s, t.n)
		if t.banner {
			st := cv.treeText(p, s, bannerGray)
			if lines, ok := cv.fitLines(t.n.name, st, t.b.w()-2*treePad, t.b.h()); ok {
				cv.t.draw(cv.dev, block{lines: lines[:1], st: st}, t.b.x0+treePad, (t.b.y0+t.b.y1)/2, alignStart, alignCenter, 0)
			}
			continue
		}
		st := cv.treeText(p, s, fill)
		lines, ok := cv.fitLines(treeLabel(l, s, t.n), st, t.b.w()-2*treePad, t.b.h()-2*treePad)
		if !ok {
			continue
		}
		bl := block{lines: lines, st: st}
		w, _ := cv.t.size(bl)
		cv.t.draw(cv.dev, bl, t.b.x0+treePad+w/2, t.b.y1-treePad, alignCenter, alignEnd, 0)
	}
	if p.parents != "overlapping" {
		return
	}
	// The labels of the top level go over the top left corner of their
	// rectangles.
	for _, n := range root.children {
		if len(n.children) == 0 {
			continue
		}
		var r box
		first := true
		for _, t := range tiles {
			if t.n.top != n.top {
				continue
			}
			if first {
				r, first = t.b, false
				continue
			}
			r = box{min(r.x0, t.b.x0), min(r.y0, t.b.y0), max(r.x1, t.b.x1), max(r.y1, t.b.y1)}
		}
		if first {
			continue
		}
		st := cv.treeText(p, s, cv.treeColor(s, n))
		if lines, ok := cv.fitLines(n.name, st, r.w()-2*treePad, r.h()/2); ok {
			cv.t.draw(cv.dev, block{lines: lines, st: st}, r.x0+treePad, r.y0+treePad, alignStart, alignStart, 0)
		}
	}
}

// layoutTreemap lays out the leaves of a tree in the box b, as Excel lays
// out a treemap: the squarified layout of Bruls, Huizing and van Wijk, with
// the children of each node from the largest to the smallest. Excel lays
// out the children that are parents apart from the children that are
// leaves, each group in a rectangle of its own. A node of the top level
// with children has a banner bh high above them.
func layoutTreemap(root *treeNode, b box, bh float64) []tile {
	var tiles []tile
	var place func(n *treeNode, r box, depth int)
	place = func(n *treeNode, r box, depth int) {
		if !r.ok() || len(tiles) >= maxTreeNodes {
			return
		}
		if len(n.children) == 0 {
			tiles = append(tiles, tile{b: r, n: n})
			return
		}
		if depth == 0 && bh > 0 {
			banner := box{r.x0, r.y0, r.x1, min(r.y1, r.y0+bh)}
			tiles = append(tiles, tile{b: banner, n: n, banner: true})
			r.y0 = banner.y1
		}
		var inner, leaves []*treeNode
		for _, c := range n.children {
			if len(c.children) > 0 {
				inner = append(inner, c)
			} else {
				leaves = append(leaves, c)
			}
		}
		if len(inner) == 0 || len(leaves) == 0 {
			squarify(n.sorted(), r, func(c *treeNode, cb box) { place(c, cb, depth+1) })
			return
		}
		blocks := []*treeNode{{children: inner}, {children: leaves}}
		for _, blk := range blocks {
			for _, c := range blk.children {
				blk.value += c.value
			}
		}
		squarify((&treeNode{children: blocks}).sorted(), r, func(blk *treeNode, cb box) {
			squarify(blk.sorted(), cb, func(c *treeNode, cc box) { place(c, cc, depth+1) })
		})
	}
	squarify(root.sorted(), b, func(c *treeNode, cb box) { place(c, cb, 0) })
	return tiles
}

// squarify lays out nodes, from the largest to the smallest, in the box b,
// and calls place for each node with its rectangle. Each row of nodes runs
// along the shorter side of the box that is left, and takes the next node
// while the worst ratio of the sides of its rectangles does not grow.
func squarify(nodes []*treeNode, b box, place func(n *treeNode, b box)) {
	total := 0.0
	for _, n := range nodes {
		total += n.value
	}
	for i := 0; i < len(nodes) && total > 0 && b.ok(); {
		across := b.w() <= b.h()
		side, other := b.h(), b.w()
		if across {
			side, other = b.w(), b.h()
		}
		worst := func(sum, lo, hi float64) float64 {
			t := other * sum / total
			a := t * sum / (side * lo)
			z := side * hi / (t * sum)
			return max(a, 1/a, z, 1/z)
		}
		j := i + 1
		sum, lo, hi := nodes[i].value, nodes[i].value, nodes[i].value
		ratio := worst(sum, lo, hi)
		for j < len(nodes) {
			v := nodes[j].value
			next := worst(sum+v, min(lo, v), max(hi, v))
			if next > ratio {
				break
			}
			sum, lo, hi, ratio = sum+v, min(lo, v), max(hi, v), next
			j++
		}
		t := other * sum / total
		if j == len(nodes) {
			t = other
		}
		at := 0.0
		for k := i; k < j; k++ {
			l := side * nodes[k].value / sum
			if k == j-1 {
				l = side - at
			}
			if across {
				place(nodes[k], box{b.x0 + at, b.y0, b.x0 + at + l, b.y0 + t})
			} else {
				place(nodes[k], box{b.x0, b.y0 + at, b.x0 + t, b.y0 + at + l})
			}
			at += l
		}
		if across {
			b.y0 += t
		} else {
			b.x0 += t
		}
		total -= sum
		i = j
	}
}

// sunburst draws a sunburst into the box b: a ring for each level of the
// tree around a hole as wide as a ring, from the top clockwise. The
// children of each node go from the largest to the smallest, and a label
// turns along its radius.
func (cv *canvas) sunburst(b box) {
	p := cv.c.plots[0]
	s := p.series[0]
	root := p.tree
	if root == nil || len(root.children) == 0 || !(root.value > 0) {
		cv.frame(b)
		return
	}
	cx, cy := (b.x0+b.x1)/2, (b.y0+b.y1)/2
	r := min(b.w(), b.h()) / 2
	if !(r > 0) {
		return
	}
	ring := r / float64(root.depth()+1)
	white := color.NRGBA{255, 255, 255, 255}
	type slice struct {
		n      *treeNode
		depth  int
		a0, a1 float64
	}
	var segs []slice
	var walk func(n *treeNode, depth int, a0, a1 float64)
	walk = func(n *treeNode, depth int, a0, a1 float64) {
		at := a0
		for _, c := range n.sorted() {
			span := (a1 - a0) * c.value / n.value
			segs = append(segs, slice{n: c, depth: depth, a0: at, a1: at + span})
			if len(c.children) > 0 {
				walk(c, depth+1, at, at+span)
			}
			at += span
		}
	}
	walk(root, 0, 0, 2*math.Pi)
	for _, sl := range segs {
		inner := ring * float64(sl.depth+1)
		cv.fill(slicePath(cx, cy, inner+ring, inner, sl.a0, sl.a1), cv.treeColor(s, sl.n))
	}
	for _, sl := range segs {
		inner := ring * float64(sl.depth+1)
		if path := slicePath(cx, cy, inner+ring, inner, sl.a0, sl.a1); path != nil {
			cv.stroke(path, white, treeEdge, "", false)
		}
	}
	l := s.labels
	if !l.show() {
		return
	}
	for _, sl := range segs {
		inner := ring * float64(sl.depth+1)
		span := sl.a1 - sl.a0
		chord := 2 * inner
		if span < math.Pi {
			chord = 2 * inner * math.Sin(span/2)
		}
		st := cv.treeText(p, s, cv.treeColor(s, sl.n))
		lines, ok := cv.fitLines(treeLabel(l, s, sl.n), st, ring-2*treePad, chord-treePad)
		if !ok {
			continue
		}
		mid := (sl.a0 + sl.a1) / 2
		x, y := polar(cx, cy, inner+ring/2, mid)
		// The text runs along the radius, outward on the right half and
		// inward on the left half, so that it does not stand on its head.
		deg := mid * 180 / math.Pi
		turn := deg - 90
		if deg > 180 {
			turn = deg - 270
		}
		cv.t.draw(cv.dev, block{lines: lines, st: st}, x, y, alignCenter, alignCenter, -turn)
	}
}
