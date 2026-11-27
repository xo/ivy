package rtf

// This file places the frames: the paragraphs at a position on the page,
// which RTF 1.9.1 calls positioned objects and frames (\absw, \phmrg,
// \posx and the others). The paragraphs with the same frame share one
// element. It also places the tables that float (\tphmrg, \tposx and the
// others).

// maxFrameSize is the largest size or position of a frame, in twips.
const maxFrameSize = 31680

// frameProps are the properties of the frame of a paragraph. They are a
// comparable value, so that the paragraphs of the same frame share it.
type frameProps struct {
	// set is set by a word of the place or of the size of the frame. A
	// paragraph without it is not in a frame.
	set bool
	// width and height are \absw and \absh in twips: 0 is the size of the
	// text, and a negative height is an exact height.
	width, height int
	// h and v are the origins of the place: 'c' the column, 'm' the
	// margin, 'p' the page and 't' the paragraph. 0 is the column for h
	// and the margin for v, as in LibreOffice 26.8.
	h, v byte
	// x and y are the place from the origins in twips, when xAlign and
	// yAlign are 0. Else xAlign is 'l', 'c', 'r', 'i' or 'o', for the left,
	// the center, the right, the inside and the outside, and yAlign is
	// 't', 'c', 'b', 'i' or 'o', for the top, the center, the bottom, the
	// inside and the outside, or 'n' for the line of the paragraph.
	x, y           int
	xAlign, yAlign byte
	// dx and dy are the distances of the text at the left and the right,
	// and at the top and the bottom, in twips.
	dx, dy int
	// wrap is 0 for text around the frame, 'n' for text above and below
	// it, \nowrap, and 'o' for text through it, \overlay.
	wrap byte
	// dropCap is set for the frame of a drop cap, which stays a paragraph.
	dropCap bool
}

// word handles a control word of the frame. It returns false for another
// control word.
func (f *frameProps) word(t token) bool {
	v := min(max(t.param, -maxFrameSize), maxFrameSize)
	dist := min(max(t.param, 0), 14400)
	switch t.word {
	case "absw":
		f.width = max(v, 0)
	case "absh":
		f.height = v
	case "phcol":
		f.h = 'c'
	case "phmrg":
		f.h = 'm'
	case "phpg":
		f.h = 'p'
	case "pvmrg":
		f.v = 'm'
	case "pvpg":
		f.v = 'p'
	case "pvpara":
		f.v = 't'
	case "posx", "posnegx":
		f.x, f.xAlign = v, 0
	case "posxl", "posxc", "posxr", "posxi", "posxo":
		f.xAlign = t.word[4]
	case "posy", "posnegy":
		f.y, f.yAlign = v, 0
	case "posyt", "posyc", "posyb", "posyin", "posyout":
		f.yAlign = t.word[4]
	case "posyil":
		f.yAlign = 'n'
	case "dxfrtext":
		f.dx, f.dy = dist, dist
		return true
	case "dfrmtxtx":
		f.dx = dist
		return true
	case "dfrmtxty":
		f.dy = dist
		return true
	case "nowrap":
		f.wrap = 'n'
		return true
	case "overlay":
		f.wrap = 'o'
		return true
	case "wrapdefault", "wraparound", "wraptight", "wrapthrough":
		f.wrap = 0
		return true
	case "dropcapli", "dropcapt":
		f.dropCap = true
		return true
	default:
		return false
	}
	f.set = true
	return true
}

// paraFrame returns the frame of a paragraph of the target t with the
// properties p at the level lvl of the tables, or the zero value for a
// paragraph that is not in a frame here. Only a frame of the body, out of
// a table, in a section of one column, is a frame. A frame of a drop cap
// stays a paragraph, and so does a frame at its paragraph of the width of
// its text.
func (c *converter) paraFrame(t *target, p paraProps, lvl int) frameProps {
	f := p.frame
	if !f.set || f.dropCap || lvl > 0 || t != c.body || c.section.cols > 1 {
		return frameProps{}
	}
	if (f.v == 't' || f.yAlign == 'n') && f.width <= 0 && f.wrap != 'o' {
		return frameProps{}
	}
	return f
}

// openFrame opens the element of the frame f in the target t.
func (c *converter) openFrame(t *target, f frameProps) {
	if !f.set {
		return
	}
	t.out.WriteString(`<div style="` + attrText(c.frameStyle(f)) + `">`)
	t.frame = f
}

// closeFrame closes the element of the open frame of the target t.
func (c *converter) closeFrame(t *target) {
	if !t.frame.set {
		return
	}
	t.out.WriteString("</div>")
	t.frame = frameProps{}
}

// frameStyle returns the style of the element of the frame f, in the
// current section. A frame at a position from the margin or from the page
// floats there (D60), and a frame at its paragraph floats at the top of
// the next paragraph, both as pageFloatCSS says. A frame of the width of
// its text then takes the width of the column. A frame that the text runs
// through is placed on the page, as a shape is.
func (c *converter) frameStyle(f frameProps) string {
	sec := c.section
	col := sec.width - sec.left - sec.right
	w := f.width
	if w <= 0 {
		w = col
	}
	h := abs(f.height)
	// the place from the left of the column, and from the top of the area
	// of the page inside its margins
	originX, areaW := 0, col
	if f.h == 'p' {
		originX, areaW = -sec.left, sec.width
	}
	x := originX + f.x
	switch f.xAlign {
	case 'l', 'i':
		x = originX
	case 'c':
		x = originX + (areaW-w)/2
	case 'r', 'o':
		x = originX + areaW - w
	}
	page := f.v == 'p'
	para := f.v == 't' || f.yAlign == 'n'
	var s css
	s.set("width", pt(w))
	switch {
	case f.height < 0:
		s.set("height", pt(h))
	case f.height > 0:
		s.set("min-height", pt(h))
	}
	switch {
	case f.wrap == 'o':
		s.set("position", "absolute")
		s.set("left", pt(x))
		if para {
			if f.y > 0 {
				s.set("margin-top", pt(f.y))
			}
			break
		}
		originY, areaH := 0, sec.height-sec.top-sec.bottom
		if page {
			originY, areaH = -sec.top, sec.height
		}
		y := originY + f.y
		switch f.yAlign {
		case 't', 'i':
			y = originY
		case 'c':
			y = originY + (areaH-h)/2
		case 'b', 'o':
			y = originY + areaH - h
		}
		s.set("top", pt(y))
	case para:
		// the frame is at the top of the next paragraph, and a position
		// above it or of an alignment stays there
		top := 0
		if f.yAlign == 0 && f.y > 0 {
			top = f.y
		}
		pageFloatCSS(&s, x, w, col, [4]int{top, f.dx, f.dy, f.dx}, f.wrap != 'n', "")
	default:
		area := "margin "
		if page {
			area = "page "
		}
		position := area + pt(f.y)
		switch f.yAlign {
		case 't', 'i':
			position = area + "top"
		case 'c':
			position = area + "center"
		case 'b', 'o':
			position = area + "bottom"
		}
		pageFloatCSS(&s, x, w, col, [4]int{f.dy, f.dx, f.dy, f.dx}, f.wrap != 'n', position)
	}
	return s.String()
}

// pageFloatCSS sets the declarations of a frame of the width w at x in a
// column of the width col, all in twips, which floats at position, with
// the distances dist from the text at the top, the right, the bottom and
// the left, as the package docx does for its frames (D60). It floats to
// the side of the column where it is. When it leaves less than half an
// inch of the column beside it, or when wrap is false, it floats at the
// left with a margin to the right side of the column, so that no line fits
// beside it. An empty position leaves the float at its place in the flow.
func pageFloatCSS(s *css, x, w, col int, dist [4]int, wrap bool, position string) {
	x = min(max(x, -maxFrameSize), maxFrameSize)
	rest := col - x - w
	switch {
	case !wrap || max(x, rest) < 720 || col-w < 720:
		s.set("float", "left")
		s.set("margin", pt(dist[0])+" "+pt(max(rest, 0))+" "+pt(dist[2])+" "+pt(x))
	case 2*x+w > col:
		s.set("float", "right")
		s.set("margin", pt(dist[0])+" "+pt(rest)+" "+pt(dist[2])+" "+pt(dist[3]))
	default:
		s.set("float", "left")
		s.set("margin", pt(dist[0])+" "+pt(dist[1])+" "+pt(dist[2])+" "+pt(x))
	}
	s.set("-ivy-float-position", position)
}

// tablePos is the position of a table that floats, from the words of the
// rows of RTF 1.9.1, such as \tphmrg, \tposx and \tpvpg.
type tablePos struct {
	// set is set by a word of the position. A table without it is a block
	// in the flow.
	set bool
	// h and v are the origins of the position, as in frameProps: 'c' the
	// column, 'm' the margin, 'p' the page and 't' the paragraph. 0 is
	// the column for h and the margin for v, as in LibreOffice 26.8.
	h, v byte
	// x and y are the position from the origins in twips, when xAlign and
	// yAlign are 0. Else xAlign and yAlign are the alignments of
	// frameProps. \tposyil, the line of the paragraph, is the position y.
	x, y           int
	xAlign, yAlign byte
	// dist holds the distances of the text in twips, at the top, the
	// right, the bottom and the left.
	dist [4]int
	// noOverlap is set by \tabsnoovrlp, for a table that does not cover
	// another table that floats.
	noOverlap bool
}

// word handles a control word of the position of a table. It returns
// false for another control word.
func (p *tablePos) word(t token) bool {
	v := min(max(t.param, -maxFrameSize), maxFrameSize)
	switch t.word {
	case "tphcol":
		p.h = 'c'
	case "tphmrg":
		p.h = 'm'
	case "tphpg":
		p.h = 'p'
	case "tpvmrg":
		p.v = 'm'
	case "tpvpg":
		p.v = 'p'
	case "tpvpara":
		p.v = 't'
	case "tposx", "tposnegx":
		p.x, p.xAlign = v, 0
	case "tposxl", "tposxc", "tposxr", "tposxi", "tposxo":
		p.xAlign = t.word[5]
	case "tposy", "tposnegy":
		p.y, p.yAlign = v, 0
	case "tposyil":
		p.yAlign = 0
	case "tposyt", "tposyc", "tposyb", "tposyin", "tposyout":
		p.yAlign = t.word[5]
	case "tdfrmtxtTop", "tdfrmtxtRight", "tdfrmtxtBottom", "tdfrmtxtLeft":
		k := map[string]int{"tdfrmtxtTop": 0, "tdfrmtxtRight": 1, "tdfrmtxtBottom": 2, "tdfrmtxtLeft": 3}[t.word]
		p.dist[k] = min(max(t.param, 0), 14400)
		return true
	case "tabsnoovrlp":
		p.noOverlap = t.param != 0
		return true
	default:
		return false
	}
	p.set = true
	return true
}

// tableFloatCSS sets the declarations of a table of the width w that
// floats at the position p, whose first cell has the padding pad at the
// left, all in twips, in the current section (D60). It follows the PDF
// files of LibreOffice 26.8. A position or the left alignment puts the
// text of the first cell there, so the edge of the table is the padding
// to the left of it, but not to the left of the page. The right alignment
// keeps the distance of the text from the right side of the area. A table
// at a position from the margin or from the page floats there. A table at
// its paragraph floats at the top of the next paragraph, with its
// position as a margin above it. The text goes on both sides of a table
// that leaves half an inch beside it on each side (D63). A table that
// leaves less than half an inch beside it on both sides has no text beside
// it.
func (c *converter) tableFloatCSS(s *css, p tablePos, w, pad int) {
	sec := c.section
	col := sec.width - sec.left - sec.right
	originX, areaW := 0, col
	if p.h == 'p' {
		originX, areaW = -sec.left, sec.width
	}
	var x int
	switch p.xAlign {
	case 'l', 'i':
		x = max(originX-pad, -sec.left)
	case 'c':
		x = originX + (areaW-w)/2
	case 'r', 'o':
		x = originX + areaW - w - p.dist[1]
	default:
		x = max(originX+p.x-pad, -sec.left)
	}
	x = min(max(x, -maxFrameSize), maxFrameSize)
	position, top := "", p.dist[0]
	if p.v == 't' {
		top = max(p.y, 0)
	} else {
		area := "margin "
		if p.v == 'p' {
			area = "page "
		}
		position = area + pt(p.y)
		switch p.yAlign {
		case 't', 'i':
			position = area + "top"
		case 'c':
			position = area + "center"
		case 'b', 'o':
			position = area + "bottom"
		}
	}
	d := p.dist
	rest := col - x - w
	left, right := x-d[3] >= 720, rest-d[1] >= 720
	switch {
	case !left && !right:
		s.set("float", "left")
		s.set("margin", pt(top)+" "+pt(max(rest, 0))+" "+pt(d[2])+" "+pt(x))
	case left && right:
		s.set("float", "left")
		s.set("margin", pt(top)+" "+pt(d[1])+" "+pt(d[2])+" "+pt(x))
		s.set("-ivy-wrap-flow", "both "+pt(max(d[1], d[3])))
	case left:
		s.set("float", "right")
		s.set("margin", pt(top)+" "+pt(rest)+" "+pt(d[2])+" "+pt(d[3]))
	default:
		s.set("float", "left")
		s.set("margin", pt(top)+" "+pt(d[1])+" "+pt(d[2])+" "+pt(x))
	}
	s.set("-ivy-float-position", position)
	if p.noOverlap && position != "" {
		s.set("-ivy-float-overlap", "avoid")
	}
}
