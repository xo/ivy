package docx

import (
	"math"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/ooxml"
)

// The height of a header or a footer.
//
// Word puts the text of a page below its header, and above its footer: the
// top of the text is the larger of the top margin and the distance of the
// header from the top of the paper plus the height of the header, and the
// same holds for the footer at the bottom. The layout places a header in
// the margin of the page, and does not move the text. So the conversion
// estimates the height of each header and each footer from its paragraphs,
// and makes the margin of the page larger when the header or the footer
// needs it. The estimate also gives the top of each paragraph of a header
// or a footer, where the drawings that the paragraph anchors start.

// avgCharWidth is the mean width of a character of text, as a multiple of
// the size of its font. It is a mean of the fonts of Office.
const avgCharWidth = 0.5

// partHeight returns the height in points of the blocks of a header or a
// footer whose root is root, on a column of width points, and the top of
// each of its paragraphs from the top of the part.
func (c *converter) partHeight(root roxmltree.Node, width float64) (float64, map[roxmltree.NodeID]float64) {
	tops := map[roxmltree.NodeID]float64{}
	h := c.blocksHeight(root, width, 0, tops, 0)
	return h, tops
}

// blocksHeight returns the height of the blocks of n, which start at the
// top y, and records the top of each paragraph in tops.
func (c *converter) blocksHeight(n roxmltree.Node, width, y float64, tops map[roxmltree.NodeID]float64, depth int) float64 {
	if depth > 8 {
		return 0
	}
	h := 0.0
	for e := range n.Children() {
		if !e.IsElement() {
			continue
		}
		switch e.TagName().Name() {
		case "p":
			tops[e.ID()] = y + h
			h += c.paragraphHeight(e, width)
		case "tbl":
			h += c.tableHeight(e, width, depth)
		case "sdt":
			if content, ok := child(e, "sdtContent"); ok {
				h += c.blocksHeight(content, width, y+h, tops, depth+1)
			}
		case "customXml", "ins", "moveTo":
			h += c.blocksHeight(e, width, y+h, tops, depth+1)
		}
	}
	return h
}

// tableHeight returns the height of a table of a header or a footer: the
// sum of its rows, each as high as its highest cell.
func (c *converter) tableHeight(tbl roxmltree.Node, width float64, depth int) float64 {
	h := 0.0
	for _, tr := range c.rowsOf(tbl, nil, 0) {
		cells := cellsOf(tr, nil, 0)
		row := 0.0
		if trPr, ok := child(tr, "trPr"); ok {
			if e, ok := child(trPr, "trHeight"); ok {
				row = number(ooxml.Attr(e, "val"), 0) / 20
			}
		}
		for _, tc := range cells {
			w := width / float64(len(cells))
			pad := 0.0
			if tcPr, ok := child(tc, "tcPr"); ok {
				if e, ok := child(tcPr, "tcW"); ok && ooxml.Attr(e, "type") == "dxa" {
					if v := number(ooxml.Attr(e, "w"), 0) / 20; v > 0 {
						w = v
					}
				}
				if mar, ok := child(tcPr, "tcMar"); ok {
					m := props{}
					margins(m, "", mar)
					pad = (m.num("top", 0) + m.num("bottom", 0)) / 20
				}
			}
			row = max(row, pad+c.cellHeight(tc, max(w-10.8, 10), depth))
		}
		h += row
	}
	return h
}

// cellHeight returns the height of the blocks of the cell tc. The empty
// paragraph that ends a cell after a table in it takes no height, as in
// Word.
func (c *converter) cellHeight(tc roxmltree.Node, width float64, depth int) float64 {
	h := c.blocksHeight(tc, width, 0, map[roxmltree.NodeID]float64{}, depth+1)
	var last roxmltree.Node
	prev, n := "", 0
	for e := range tc.Children() {
		if e.IsElement() {
			if n > 0 {
				prev = last.TagName().Name()
			}
			last = e
			n++
		}
	}
	if n >= 2 && prev == "tbl" && last.TagName().Name() == "p" && empty(last) {
		h -= c.paragraphHeight(last, width)
	}
	return h
}

// paragraphHeight returns the height of a paragraph on a column of width
// points: its lines, with the text wrapped at the mean width of a
// character, and its spacing before and after.
func (c *converter) paragraphHeight(p roxmltree.Node, width float64) float64 {
	direct := props{}
	if ppr, ok := child(p, "pPr"); ok {
		direct = parsePPr(ppr)
	}
	sppr, srpr := props{}, props{}
	if st := c.styles.get(direct["pStyle"], "paragraph"); st != nil {
		sppr, srpr = st.ppr, st.rpr
	}
	ppr := c.styles.pDefault.over(sppr).over(direct)
	rpr := c.lineFont(p, c.styles.rDefault.over(srpr))
	if empty(p) {
		if ppr, ok := child(p, "pPr"); ok {
			if r, ok := child(ppr, "rPr"); ok {
				rpr = rpr.over(parseRPr(r))
			}
		}
		if rpr.on("vanish") {
			return 0
		}
	}
	size := fontSize(rpr)
	line := c.lineRatio(rpr) * size
	spacing := ppr.num("spacing.line", 240)
	switch ppr["spacing.lineRule"] {
	case "exact":
		if spacing > 0 {
			line = spacing / 20
		}
	case "atLeast":
		line = max(line, spacing/20)
	default:
		if spacing > 0 {
			line *= min(spacing/240, 10)
		}
	}
	avail := width - (ppr.num("ind.left", 0)+ppr.num("ind.right", 0))/20
	avail = max(avail, size)
	lines, inline := 1, 0.0
	chars := 0
	for d := range p.Descendants() {
		if !d.IsElement() {
			continue
		}
		switch d.TagName().Name() {
		case "t":
			if !inDrawing(d, p) {
				s, _ := d.Text()
				chars += len([]rune(s))
			}
		case "tab":
			if !inDrawing(d, p) {
				chars += 4
			}
		case "br", "cr":
			if !inDrawing(d, p) && ooxml.Attr(d, "type") == "" {
				lines++
			}
		case "inline":
			// a picture in the line makes the line as high as itself
			if e, ok := child(d, "extent"); ok {
				inline = max(inline, number(ooxml.Attr(e, "cy"), 0)/emuPerPoint)
			}
		}
	}
	lines += max(int(math.Ceil(float64(chars)*avgCharWidth*size/avail))-1, 0)
	h := float64(lines) * line
	if inline > line {
		h += min(inline, maxDrawingSize) - line
	}
	h += (ppr.num("spacing.before", 0) + ppr.num("spacing.after", 0)) / 20
	return h
}

// hfHeight holds the estimate of the height of a header or a footer, and
// the top of each of its paragraphs.
type hfHeight struct {
	height float64
	tops   map[roxmltree.NodeID]float64
}

// bodyMargins returns the top and the bottom margins of a page of the
// section s whose header is the running element header and whose footer is
// the running element footer, either of which can be "": the margins of
// the section, or larger ones that hold the header and the footer, as in
// Word.
func (c *converter) bodyMargins(s *section, header, footer string) (float64, float64) {
	top, bottom := s.top, s.bottom
	if h, ok := c.hfHeights[header]; ok && header != "" {
		top = max(top, min(s.header, s.top)+h.height)
	}
	if h, ok := c.hfHeights[footer]; ok && footer != "" {
		bottom = max(bottom, min(s.footer, s.bottom)+h.height)
	}
	// The text keeps at least a fifth of the page.
	limit := s.height * 0.4
	return min(top, max(s.top, limit)), min(bottom, max(s.bottom, limit))
}
