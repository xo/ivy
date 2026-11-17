package odraw

// The flags of an OfficeArtFSP (MS-ODRAW 2.2.40).
const (
	FSPGroup      = 0x0001
	FSPChild      = 0x0002
	FSPPatriarch  = 0x0004
	FSPDeleted    = 0x0008
	FSPFlipH      = 0x0040
	FSPFlipV      = 0x0080
	FSPBackground = 0x0400
)

// Rect is a rectangle in the units of its space.
type Rect struct {
	Left, Top, Right, Bottom float64
}

// Shape is a shape of a drawing: an OfficeArtSpContainer, or a group of
// them, an OfficeArtSpgrContainer (MS-ODRAW 2.2.14 and 2.2.16).
type Shape struct {
	// Type is the type of the shape, an MSOSPT.
	Type  int
	SPID  uint32
	Flags uint32
	Props Props
	// Anchor is the box of a child of a group, in the units of the group,
	// from its OfficeArtChildAnchor, when HasAnchor is set. The client
	// anchor of a shape that is not a child is in ClientAnchor.
	Anchor    Rect
	HasAnchor bool
	// ClientAnchor, ClientData and ClientTextbox are the bodies of the
	// records that the format of the file defines, or nil. They are slices
	// of the drawing that ReadDrawing read.
	ClientAnchor  []byte
	ClientData    []byte
	ClientTextbox []byte
	// Group is the box of the children of a group in their units, and
	// Children are its shapes.
	Group    Rect
	Children []*Shape
}

// IsGroup reports whether the shape is a group.
func (sh *Shape) IsGroup() bool {
	return sh.Flags&FSPGroup != 0 || len(sh.Children) > 0
}

// Drawing is the shapes of a drawing, such as a slide or a sheet: an
// OfficeArtDgContainer.
type Drawing struct {
	// Root is the group of the shapes, whose first shape is the patriarch.
	Root *Shape
	// Background is the shape of the background, or nil.
	Background *Shape
	// count is the number of shapes, to bound the work of a drawing.
	count int
}

// ReadDrawing reads the body dg of an OfficeArtDgContainer: maxObjects
// shapes at most, in groups maxDepth deep at most.
func ReadDrawing(dg []byte, maxObjects, maxDepth int) *Drawing {
	d := &Drawing{}
	for h, body := range Children(dg) {
		switch h.Type {
		case RecSpgrContainer:
			if d.Root == nil {
				d.Root = d.readGroup(body, maxObjects, maxDepth)
			}
		case RecSpContainer:
			if d.Background == nil {
				d.Background = d.readShape(body)
			}
		}
	}
	return d
}

// readGroup reads an OfficeArtSpgrContainer. Its first shape is the group
// itself, with the box of its children.
func (d *Drawing) readGroup(b []byte, maxObjects, maxDepth int) *Shape {
	var g *Shape
	for h, body := range Children(b) {
		if d.count >= maxObjects {
			break
		}
		var sh *Shape
		switch h.Type {
		case RecSpContainer:
			sh = d.readShape(body)
		case RecSpgrContainer:
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
			if fspgr := Child(body, RecFSPGR); h.Type == RecSpContainer && len(fspgr) >= 16 {
				g.Group = readRect(fspgr)
			}
			continue
		}
		g.Children = append(g.Children, sh)
	}
	return g
}

// readShape reads an OfficeArtSpContainer.
func (d *Drawing) readShape(b []byte) *Shape {
	d.count++
	sh := &Shape{}
	for h, body := range Children(b) {
		switch h.Type {
		case RecFSP:
			sh.Type = int(h.Inst)
			if len(body) >= 8 {
				sh.SPID = le.Uint32(body)
				sh.Flags = le.Uint32(body[4:])
			}
		case RecFOPT, RecTertiaryFOPT:
			sh.Props.Read(body, int(h.Inst))
		case RecChildAnchor:
			if len(body) >= 16 {
				sh.Anchor, sh.HasAnchor = readRect(body), true
			}
		case RecClientAnchor:
			sh.ClientAnchor = body
		case RecClientData:
			sh.ClientData = body
		case RecClientTextbox:
			sh.ClientTextbox = body
		}
	}
	return sh
}

// readRect reads a rectangle of four signed integers of 32 bits: the left,
// the top, the right and the bottom.
func readRect(b []byte) Rect {
	return Rect{
		Left:   float64(int32(le.Uint32(b))),
		Top:    float64(int32(le.Uint32(b[4:]))),
		Right:  float64(int32(le.Uint32(b[8:]))),
		Bottom: float64(int32(le.Uint32(b[12:]))),
	}
}

// Shapes calls f for each shape of the drawing that is not a group, in the
// order that they draw, with the transform of its group. anchor gives the
// box of a shape that is not a child, in the space of the page, from its
// client anchor.
func (d *Drawing) Shapes(anchor func(sh *Shape) (Rect, bool), f func(sh *Shape, t *Transform)) {
	if d.Root != nil {
		Walk(d.Root, nil, anchor, f)
	}
}

// Walk calls f for each shape of the group g that is not a group, with the
// transform t of the group onto the page. anchor gives the box of the group
// when t is nil.
func Walk(g *Shape, t *Transform, anchor func(sh *Shape) (Rect, bool), f func(sh *Shape, t *Transform)) {
	if g.Group.Right != g.Group.Left && g.Group.Bottom != g.Group.Top {
		box, ok := g.Anchor, g.HasAnchor
		if t == nil && anchor != nil {
			box, ok = anchor(g)
		}
		if ok {
			t = &Transform{From: g.Group, To: t.Apply(box)}
		}
	}
	for _, sh := range g.Children {
		if sh.Flags&FSPDeleted != 0 {
			continue
		}
		if sh.IsGroup() {
			Walk(sh, t, anchor, f)
			continue
		}
		f(sh, t)
	}
}

// Transform maps the coordinates of a group onto the coordinates of the
// page.
type Transform struct {
	From, To Rect
}

// Apply maps r through t. A nil t leaves r as it is.
func (t *Transform) Apply(r Rect) Rect {
	if t == nil {
		return r
	}
	sx := (t.To.Right - t.To.Left) / nonzero(t.From.Right-t.From.Left)
	sy := (t.To.Bottom - t.To.Top) / nonzero(t.From.Bottom-t.From.Top)
	return Rect{
		Left:   t.To.Left + (r.Left-t.From.Left)*sx,
		Top:    t.To.Top + (r.Top-t.From.Top)*sy,
		Right:  t.To.Left + (r.Right-t.From.Left)*sx,
		Bottom: t.To.Top + (r.Bottom-t.From.Top)*sy,
	}
}

func nonzero(v float64) float64 {
	if v == 0 {
		return 1
	}
	return v
}
