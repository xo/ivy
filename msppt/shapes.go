package msppt

// The records of a shape of MS-PPT that the package reads.
const (
	rtOEPlaceholderAtom = rtPlaceholderAtom
)

// shape is a shape of a drawing: an OfficeArtSpContainer, or a group of
// them (MS-ODRAW 2.2.14 and 2.2.16).
type shape struct {
	// typ is the type of the shape, an MSOSPT.
	typ   int
	spid  uint32
	flags uint32
	props props
	// anchor is the box of the shape: in master units for a shape of the
	// slide, or in the units of its group for a child.
	anchor    rect
	hasAnchor bool
	// group is the box of the children of a group in their units, and
	// children are its shapes.
	group    rect
	children []*shape
	// placeholder is the type of the placeholder, or 0.
	placeholder int
	// text is the text of the client text box, or nil, and textRef the
	// index of a text of the slide list, or -1.
	text    *text
	textRef int
}

// drawing is the shapes of a slide or of a master.
type drawing struct {
	root       *shape
	background *shape
	// count is the number of shapes, to bound the work of a slide.
	count int
}

// readDrawing reads the OfficeArtDgContainer of the PPDrawing b.
func readDrawing(b []byte, maxObjects, maxDepth int) *drawing {
	dg := child(child(b, rtPPDrawing), rtOfficeArtDg)
	d := &drawing{}
	for h, body := range children(dg) {
		switch h.typ {
		case rtOfficeArtSpgr:
			if d.root == nil {
				d.root = d.readGroup(body, maxObjects, maxDepth)
			}
		case rtOfficeArtSp:
			if d.background == nil {
				d.background = d.readShape(body)
			}
		}
	}
	return d
}

// readGroup reads an OfficeArtSpgrContainer. Its first shape is the group
// itself, with the box of its children.
func (d *drawing) readGroup(b []byte, maxObjects, maxDepth int) *shape {
	var g *shape
	for h, body := range children(b) {
		if d.count >= maxObjects {
			break
		}
		var sh *shape
		switch h.typ {
		case rtOfficeArtSp:
			sh = d.readShape(body)
		case rtOfficeArtSpgr:
			if maxDepth > 1 {
				sh = d.readGroup(body, maxObjects, maxDepth-1)
			}
		}
		if sh == nil {
			continue
		}
		if g == nil {
			// the group shape itself
			g = sh
			if fspgr := child(body, rtOfficeArtFSPGR); h.typ == rtOfficeArtSp && len(fspgr) >= 16 {
				g.group = rect{
					left:   float64(int32(le.Uint32(fspgr))),
					top:    float64(int32(le.Uint32(fspgr[4:]))),
					right:  float64(int32(le.Uint32(fspgr[8:]))),
					bottom: float64(int32(le.Uint32(fspgr[12:]))),
				}
			}
			continue
		}
		g.children = append(g.children, sh)
	}
	return g
}

// readShape reads an OfficeArtSpContainer.
func (d *drawing) readShape(b []byte) *shape {
	d.count++
	sh := &shape{textRef: -1}
	for h, body := range children(b) {
		switch h.typ {
		case rtOfficeArtFSP:
			sh.typ = int(h.inst)
			if len(body) >= 8 {
				sh.spid = le.Uint32(body)
				sh.flags = le.Uint32(body[4:])
			}
		case rtOfficeArtFOPT, rtOfficeArtTertiaryFOPT:
			sh.props.readProps(body, int(h.inst))
		case rtOfficeArtClientAnch:
			sh.anchor, sh.hasAnchor = clientAnchor(body)
		case rtOfficeArtChild:
			if len(body) >= 16 {
				sh.anchor = rect{
					left:   float64(int32(le.Uint32(body))),
					top:    float64(int32(le.Uint32(body[4:]))),
					right:  float64(int32(le.Uint32(body[8:]))),
					bottom: float64(int32(le.Uint32(body[12:]))),
				}
				sh.hasAnchor = true
			}
		case rtOfficeArtClientData:
			if ph := child(body, rtOEPlaceholderAtom); len(ph) >= 5 {
				sh.placeholder = int(ph[4])
			}
		case rtOfficeArtClientText:
			var texts []text
			for th, tb := range children(body) {
				switch th.typ {
				case rtOutlineTextRefAtom:
					if len(tb) >= 4 {
						sh.textRef = int(int32(le.Uint32(tb)))
					}
				default:
					texts = addText(texts, th, tb)
				}
			}
			if len(texts) > 0 {
				sh.text = &texts[0]
			}
		}
	}
	return sh
}

// clientAnchor reads the anchor of a shape of a slide: a RectStruct or a
// SmallRectStruct, each with the top, the left, the right and the bottom.
func clientAnchor(a []byte) (rect, bool) {
	switch {
	case len(a) >= 16:
		return rect{
			top:    float64(int32(le.Uint32(a))),
			left:   float64(int32(le.Uint32(a[4:]))),
			right:  float64(int32(le.Uint32(a[8:]))),
			bottom: float64(int32(le.Uint32(a[12:]))),
		}, true
	case len(a) >= 8:
		return rect{
			top:    float64(int16(le.Uint16(a))),
			left:   float64(int16(le.Uint16(a[2:]))),
			right:  float64(int16(le.Uint16(a[4:]))),
			bottom: float64(int16(le.Uint16(a[6:]))),
		}, true
	}
	return rect{}, false
}

// shapes calls f for each shape of the drawing that is not a group, in the
// order that they draw, with the transform of its group.
func (d *drawing) shapes(f func(sh *shape, t *transform)) {
	if d.root != nil {
		walk(d.root, nil, f)
	}
}

// walk calls f for each shape of the group g, with the transform t of the
// group onto the slide.
func walk(g *shape, t *transform, f func(sh *shape, t *transform)) {
	if g.hasAnchor && g.group.right != g.group.left && g.group.bottom != g.group.top {
		t = &transform{from: g.group, to: t.apply(g.anchor)}
	}
	for _, sh := range g.children {
		if sh.flags&fspDeleted != 0 {
			continue
		}
		if sh.flags&fspGroup != 0 || len(sh.children) > 0 {
			walk(sh, t, f)
			continue
		}
		f(sh, t)
	}
}

// transform maps the coordinates of a group onto the coordinates of the
// slide.
type transform struct {
	from, to rect
}

// apply maps r through t.
func (t *transform) apply(r rect) rect {
	if t == nil {
		return r
	}
	sx := (t.to.right - t.to.left) / nonzero(t.from.right-t.from.left)
	sy := (t.to.bottom - t.to.top) / nonzero(t.from.bottom-t.from.top)
	return rect{
		left:   t.to.left + (r.left-t.from.left)*sx,
		top:    t.to.top + (r.top-t.from.top)*sy,
		right:  t.to.left + (r.right-t.from.left)*sx,
		bottom: t.to.top + (r.bottom-t.from.top)*sy,
	}
}

func nonzero(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}
