package msppt

import (
	"github.com/xo/ivy/internal/odraw"
)

// shape is a shape of Office Drawing with what a slide gives it: its
// anchor, its placeholder and its text.
type shape struct {
	*odraw.Shape
	// anchor is the box of the shape: in master units for a shape of the
	// slide, or in the units of its group for a child.
	anchor    odraw.Rect
	hasAnchor bool
	// placeholder is the type of the placeholder, or 0.
	placeholder int
	// text is the text of the client text box, or nil, and textRef the
	// index of a text of the slide list, or -1.
	text    *text
	textRef int
	// props9 are the picture bullets and the automatic numbers of the
	// paragraphs of the text, from the extension of PowerPoint 2000.
	props9 []pf9
}

// drawing is the shapes of a slide or of a master.
type drawing struct {
	od         *odraw.Drawing
	background *shape
	// byShape maps each shape of Office Drawing to the shape of the slide.
	byShape map[*odraw.Shape]*shape
}

// readDrawing reads the OfficeArtDgContainer of the PPDrawing in the body b
// of a slide or a master.
func readDrawing(b []byte, maxObjects, maxDepth int) *drawing {
	dg := child(child(b, rtPPDrawing), odraw.RecDgContainer)
	d := &drawing{od: odraw.ReadDrawing(dg, maxObjects, maxDepth), byShape: make(map[*odraw.Shape]*shape)}
	var add func(sh *odraw.Shape, depth int)
	add = func(sh *odraw.Shape, depth int) {
		d.byShape[sh] = newShape(sh, maxObjects)
		if depth > 0 {
			for _, c := range sh.Children {
				add(c, depth-1)
			}
		}
	}
	if d.od.Root != nil {
		add(d.od.Root, maxDepth)
	}
	if d.od.Background != nil {
		d.background = newShape(d.od.Background, maxObjects)
	}
	return d
}

// newShape reads what a slide gives the shape sh: its anchor, its
// placeholder, its text, and the picture bullets and the automatic numbers
// of its text.
func newShape(sh *odraw.Shape, maxObjects int) *shape {
	s := &shape{Shape: sh, textRef: -1}
	if a, ok := clientAnchor(sh.ClientAnchor); ok {
		s.anchor, s.hasAnchor = a, true
	} else if sh.HasAnchor {
		s.anchor, s.hasAnchor = sh.Anchor, true
	}
	if ph := child(sh.ClientData, rtPlaceholderAtom); len(ph) >= 5 {
		s.placeholder = int(ph[4])
	}
	s.props9 = shapeProps9(sh.ClientData, maxObjects)
	if sh.ClientTextbox != nil {
		var texts []text
		for th, tb := range children(sh.ClientTextbox) {
			switch th.Type {
			case rtOutlineTextRefAtom:
				if len(tb) >= 4 {
					s.textRef = int(int32(le.Uint32(tb)))
				}
			default:
				texts = addText(texts, th, tb)
			}
		}
		if len(texts) > 0 {
			s.text = &texts[0]
		}
	}
	return s
}

// clientAnchor reads the anchor of a shape of a slide: a RectStruct or a
// SmallRectStruct, each with the top, the left, the right and the bottom.
func clientAnchor(a []byte) (odraw.Rect, bool) {
	switch {
	case len(a) >= 16:
		return odraw.Rect{
			Top:    float64(int32(le.Uint32(a))),
			Left:   float64(int32(le.Uint32(a[4:]))),
			Right:  float64(int32(le.Uint32(a[8:]))),
			Bottom: float64(int32(le.Uint32(a[12:]))),
		}, true
	case len(a) >= 8:
		return odraw.Rect{
			Top:    float64(int16(le.Uint16(a))),
			Left:   float64(int16(le.Uint16(a[2:]))),
			Right:  float64(int16(le.Uint16(a[4:]))),
			Bottom: float64(int16(le.Uint16(a[6:]))),
		}, true
	}
	return odraw.Rect{}, false
}

// shapes calls f for each shape of the drawing that is not a group, in the
// order that they draw, with the transform of its group.
func (d *drawing) shapes(f func(sh *shape, t *odraw.Transform)) {
	anchor := func(sh *odraw.Shape) (odraw.Rect, bool) {
		if s := d.byShape[sh]; s != nil {
			return s.anchor, s.hasAnchor
		}
		return odraw.Rect{}, false
	}
	d.od.Shapes(anchor, func(sh *odraw.Shape, t *odraw.Transform) {
		if s := d.byShape[sh]; s != nil {
			f(s, t)
		}
	})
}
