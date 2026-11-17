package msdoc

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// This file reads the shapes of Office Drawing that float on the page and
// are not pictures: the text boxes, whose text is in the part of the text
// boxes, and the shapes that the package draws as SVG images, such as
// rectangles, ellipses, lines, freeforms and groups of them (MS-DOC 2.9.94
// and MS-ODRAW). The pictures of the main document are in picture.go.

// The records of Office Drawing of the shapes and the groups.
const (
	rtDggContainer  = 0xF000
	rtSpgrContainer = 0xF003
	rtSpContainer   = 0xF004
	rtFSPGR         = 0xF009
	rtChildAnchor   = 0xF00F
)

// The flags of an FSP (MS-ODRAW 2.2.40).
const (
	fspGroup = 0x0001
	fspFlipH = 0x0040
	fspFlipV = 0x0080
)

// The properties of a shape that the package reads (MS-ODRAW 2.3).
const (
	propRotation      = 0x0004
	propLTxid         = 0x0080
	propTextLeft      = 0x0081
	propTextTop       = 0x0082
	propTextRight     = 0x0083
	propTextBottom    = 0x0084
	propGeoLeft       = 0x0140
	propGeoTop        = 0x0141
	propGeoRight      = 0x0142
	propGeoBottom     = 0x0143
	propVertices      = 0x0145
	propSegmentInfo   = 0x0146
	propAdjustValue   = 0x0147
	propFillColor     = 0x0181
	propFillOpacity   = 0x0182
	propFillBools     = 0x01BF
	propLineColor     = 0x01C0
	propLineOpacity   = 0x01C1
	propLineWidth     = 0x01CB
	propLineDashing   = 0x01CE
	propLineBools     = 0x01FF
	propPosH          = 0x038F
	propPosV          = 0x0391
	propComplexFlag   = 0x8000
	propIDMask        = 0x3FFF
	emusPerPoint      = 12700
	fixedOne          = 65536
	defaultLineWidth  = 9525
	defaultGeoSize    = 21600
	maxShapeDepth     = 8
	maxShapeVertices  = 4096
	shapeTypeNone     = 0
	shapeTypeRect     = 1
	shapeTypeRoundRec = 2
	shapeTypeEllipse  = 3
	shapeTypeDiamond  = 4
	shapeTypeTriangle = 5
	shapeTypeRightTri = 6
	shapeTypeParallel = 7
	shapeTypeHexagon  = 9
	shapeTypeLine     = 20
	shapeTypeConnect  = 32
	shapeTypeTextBox  = 202
)

// odShape is a shape of Office Drawing: its id, its type, the flags of its
// FSP and its properties. A group has the rectangle of the coordinates of
// its children, and its children. A child of a group has its rectangle in
// the coordinates of the group.
type odShape struct {
	spid    uint32
	typ     int
	flags   uint32
	props   map[uint16]uint32
	complex map[uint16][]byte

	anchor    [4]float64
	hasAnchor bool
	group     [4]float64
	children  []*odShape
}

// prop returns the property id of the shape, or def.
func (s *odShape) prop(id uint16, def uint32) uint32 {
	if v, ok := s.props[id]; ok {
		return v
	}
	return def
}

// flag returns the bit of a property of boolean values, whose bit 16 above
// it makes it valid, or def.
func (s *odShape) flag(id uint16, bit uint, def bool) bool {
	v, ok := s.props[id]
	if !ok || v&(1<<(bit+16)) == 0 {
		return def
	}
	return v&(1<<bit) != 0
}

// drawing holds the shapes of the drawings of a document, by their ids.
type drawing map[uint32]*odShape

// readDrawing reads the shapes of the OfficeArtContent of the document
// (MS-DOC 2.9.172): the drawing group, then each drawing, of the main
// document or of the headers, after a byte that tells which.
func (p *parser) readDrawing() drawing {
	out := drawing{}
	if p.s.word6 || len(p.s.fib.fcLcb) <= fcDggInfo {
		return out
	}
	b := p.s.part(fcDggInfo)
	for len(b) >= 8 {
		typ, size := le.Uint16(b[2:]), le.Uint32(b[4:])
		if typ != rtDggContainer && typ != rtDgContainer && len(b) >= 9 && le.Uint16(b[3:]) == rtDgContainer {
			b = b[1:]
			continue
		}
		if uint64(size) > uint64(len(b)-8) {
			break
		}
		if typ == rtDgContainer {
			p.walkDrawing(b[8:8+size], out)
		}
		b = b[8+size:]
	}
	return out
}

// walkDrawing reads the groups of an OfficeArtDgContainer into out.
func (p *parser) walkDrawing(b []byte, out drawing) {
	for rec := range records(b) {
		if rec.typ == rtSpgrContainer {
			p.readGroup(rec.body, 0, out)
		}
	}
}

// record is a record of Office Drawing: its version and instance, its
// type, and its body.
type record struct {
	vi   uint16
	typ  uint16
	body []byte
}

// records iterates over the records in b, and stops at one that does not
// fit.
func records(b []byte) func(func(record) bool) {
	return func(yield func(record) bool) {
		for len(b) >= 8 {
			vi, typ, size := le.Uint16(b), le.Uint16(b[2:]), le.Uint32(b[4:])
			if uint64(size) > uint64(len(b)-8) {
				return
			}
			if !yield(record{vi: vi, typ: typ, body: b[8 : 8+size]}) {
				return
			}
			b = b[8+size:]
		}
	}
}

// readGroup reads an OfficeArtSpgrContainer: its first shape is the group,
// and the others are its children. It adds each shape to out, and returns
// the group.
func (p *parser) readGroup(b []byte, depth int, out drawing) *odShape {
	if depth > maxShapeDepth {
		return nil
	}
	var group *odShape
	for rec := range records(b) {
		if len(out) > p.lim.MaxObjects {
			break
		}
		var s *odShape
		switch rec.typ {
		case rtSpContainer:
			s = p.readShape(rec.body)
		case rtSpgrContainer:
			s = p.readGroup(rec.body, depth+1, out)
		}
		switch {
		case s == nil:
		case group == nil:
			group = s
		default:
			group.children = append(group.children, s)
		}
		if s != nil && rec.typ == rtSpContainer {
			out[s.spid] = s
		}
	}
	return group
}

// readShape reads an OfficeArtSpContainer.
func (p *parser) readShape(b []byte) *odShape {
	s := &odShape{props: map[uint16]uint32{}, complex: map[uint16][]byte{}, typ: -1}
	for rec := range records(b) {
		switch rec.typ {
		case rtFSP:
			if len(rec.body) >= 8 {
				s.typ, s.spid, s.flags = int(rec.vi>>4), le.Uint32(rec.body), le.Uint32(rec.body[4:])
			}
		case rtFSPGR, rtChildAnchor:
			if len(rec.body) < 16 {
				continue
			}
			var r [4]float64
			for k := range 4 {
				r[k] = float64(int32(le.Uint32(rec.body[4*k:])))
			}
			if rec.typ == rtFSPGR {
				s.group = r
			} else {
				s.anchor, s.hasAnchor = r, true
			}
		case rtFOPT, rtTertiaryFOPT:
			readProps(rec, s)
		}
	}
	if s.typ < 0 {
		return nil
	}
	return s
}

// readProps reads the properties of an OfficeArtFOPT record into s. The
// data of the complex properties follow the table of the properties, in
// its order.
func readProps(rec record, s *odShape) {
	count := int(rec.vi >> 4)
	if 6*count > len(rec.body) {
		count = len(rec.body) / 6
	}
	data := rec.body[6*count:]
	for k := range count {
		e := rec.body[6*k:]
		op, v := le.Uint16(e), le.Uint32(e[2:])
		id := op & propIDMask
		if op&propComplexFlag == 0 {
			s.props[id] = v
			continue
		}
		if uint64(v) > uint64(len(data)) {
			return
		}
		s.complex[id], data = data[:v], data[v:]
	}
}

// drawn is a shape of Office Drawing that the conversion writes: a text
// box, with the paragraphs of its text, or an SVG image of a shape. It
// floats on the page as a picture does.
type drawn struct {
	floating
	shape *odShape
	// paras holds the paragraphs of a text box, and text is set for one.
	paras []paragraph
	text  bool
	// posH and posV are the alignment of the shape: 1 at the left or the
	// top, 2 at the center and 3 at the right or the bottom, or 0 for
	// its place.
	posH, posV int
}

// anchors returns the shapes of the main document, or of the headers when
// header is set, by the character of their anchor in their part of the
// text: the FSPAs of the PlcSpaMom or of the PlcSpaHdr.
func (p *parser) anchors(header bool) map[uint32][]byte {
	i := fcPlcSpaMom
	if header {
		i = fcPlcSpaHdr
	}
	out := map[uint32][]byte{}
	plc := p.s.part(i)
	if p.s.word6 || len(plc) < 4+fspaSize {
		return out
	}
	n := (len(plc) - 4) / (4 + fspaSize)
	for k := range min(n, p.lim.MaxObjects) {
		out[u32(plc, 4*k)] = plc[4*(n+1)+fspaSize*k : 4*(n+1)+fspaSize*(k+1)]
	}
	return out
}

// textboxes returns the range of the text of each text box of the main
// document, or of the headers, by the id of its shape: the PlcftxbxTxt or
// the PlcfHdrtxbxTxt, with an FTXBXS for each text box (MS-DOC 2.8.38 and
// 2.9.92).
func (p *parser) textboxes(header bool) map[uint32][2]uint32 {
	i, part := fcPlcftxbxTxt, partTextboxes
	if header {
		i, part = fcPlcfHdrTxbx, partHeaderTextboxes
	}
	const ftxbxsSize = 22
	out := map[uint32][2]uint32{}
	plc := p.s.part(i)
	n := min((len(plc)-4)/(4+ftxbxsSize), p.lim.MaxObjects)
	start, end := p.s.fib.partRange(part)
	for k := range max(n, 0) {
		ftxbxs := plc[4*(n+1)+ftxbxsSize*k:]
		// a text box that Word can reuse holds no text
		if u16(ftxbxs, 8) != 0 {
			continue
		}
		a, b := uint64(start)+uint64(u32(plc, 4*k)), uint64(start)+uint64(u32(plc, 4*k+4))
		if a >= b || b > uint64(end) {
			continue
		}
		out[u32(ftxbxs, 14)] = [2]uint32{uint32(a), uint32(b)}
	}
	return out
}

// drawnAt returns the shape whose anchor is the character cp of the main
// document, or of the headers when header is set, or nil for a shape that
// the package does not draw. A text box reads its text.
func (p *parser) drawnAt(ctx context.Context, cp uint32, header bool) (*drawn, error) {
	if p.drawing == nil {
		p.drawing = p.readDrawing()
		p.spas = [2]map[uint32][]byte{p.anchors(false), p.anchors(true)}
		p.txbxs = [2]map[uint32][2]uint32{p.textboxes(false), p.textboxes(true)}
	}
	k := 0
	if header {
		start, _ := p.s.fib.partRange(partHeaders)
		cp -= start
		k = 1
	}
	fspa, ok := p.spas[k][cp]
	if !ok {
		return nil, nil
	}
	s := p.drawing[u32(fspa, 0)]
	if s == nil {
		return nil, nil
	}
	left, top := int(int32(u32(fspa, 4))), int(int32(u32(fspa, 8)))
	right, bottom := int(int32(u32(fspa, 12))), int(int32(u32(fspa, 16)))
	w, h := right-left, bottom-top
	if w < 0 || h < 0 || w > 31680 || h > 31680 || w+h == 0 {
		return nil, nil
	}
	flags := u16(fspa, 20)
	d := &drawn{
		floating: floating{
			left: left, top: top,
			bx: int(flags >> 1 & 3), by: int(flags >> 3 & 3), wr: int(flags >> 5 & 0xF),
			behind: flags&0x4000 != 0 || s.flag(propGroupBools, 5, false),
		},
		shape: s,
		posH:  int(s.prop(propPosH, 0)),
		posV:  int(s.prop(propPosV, 0)),
	}
	if v, ok := s.props[propPosRelH]; ok && v <= 3 {
		d.bx = [...]int{0, 1, 2, 2}[v]
	}
	if v, ok := s.props[propPosRelV]; ok && v <= 3 {
		d.by = [...]int{0, 1, 2, 2}[v]
	}
	if r, ok := p.txbxs[k][s.spid]; ok {
		paras, err := p.paragraphsIn(ctx, r[0], r[1])
		if err != nil {
			return nil, err
		}
		// the mark at the end of the text box shows nothing
		for len(paras) > 1 && len(paras[len(paras)-1].runs) == 0 && !paras[len(paras)-1].pap.inTable {
			paras = paras[:len(paras)-1]
		}
		d.paras, d.text = paras, true
		d.pic = &picture{width: float64(w) / 20, height: float64(h) / 20}
		return d, nil
	}
	if pib := int(s.prop(propPib, 0)); pib > 0 {
		// a picture of the store, which picture.go reads for the main
		// document only
		if p.store == nil {
			p.store = [][]byte{}
			p.walkShapes(p.s.part(fcDggInfo), 0, &p.store, map[uint32]*shapeProps{})
		}
		if pib > len(p.store) {
			return nil, nil
		}
		data, ext := blipFromEntry(p.store[pib-1], p.s.word, p.lim.MaxMemory)
		if data == nil {
			return nil, nil
		}
		name := "media/" + strconv.Itoa(len(p.media)+1) + ext
		p.media = append(p.media, media{name: name, data: data})
		d.pic = &picture{name: name, width: float64(w) / 20, height: float64(h) / 20}
		return d, nil
	}
	data, pad := shapeSVG(s, float64(w)/20, float64(h)/20)
	if data == nil {
		return nil, nil
	}
	name := "media/" + strconv.Itoa(len(p.media)+1) + ".svg"
	p.media = append(p.media, media{name: name, data: data})
	d.pic = &picture{name: name, width: float64(w)/20 + 2*pad, height: float64(h)/20 + 2*pad}
	d.left, d.top = d.left-int(pad*20), d.top-int(pad*20)
	return d, nil
}

// writeDrawn writes a shape that floats on the page: a text box as a block
// with its text, or the SVG image of a shape.
func (p *parser) writeDrawn(b *strings.Builder, d *drawn) {
	f := d.floating
	pg := p.page
	if pg.width == 0 {
		pg = letter
	}
	w, h := f.pic.width, f.pic.height
	// an aligned shape takes its place from the box of its origin
	refW, refH := float64(pg.width-pg.left-pg.right)/20, float64(pg.height-pg.top-pg.bottom)/20
	if f.bx == 1 {
		refW = float64(pg.width) / 20
	}
	if f.by == 1 {
		refH = float64(pg.height) / 20
	}
	switch d.posH {
	case 1:
		f.left = 0
	case 2:
		f.left = int((refW - w) / 2 * 20)
	case 3:
		f.left = int((refW - w) * 20)
	}
	if f.by != 2 {
		switch d.posV {
		case 1:
			f.top = 0
		case 2:
			f.top = int((refH - h) / 2 * 20)
		case 3:
			f.top = int((refH - h) * 20)
		}
	}
	style := p.floatStyle(&f, w, h)
	if !d.text {
		b.WriteString(`<img src="` + attr(f.pic.name) + `" alt="" style="` + strings.Join(style, "; ") + `"/>`)
		return
	}
	s := d.shape
	inset := func(id uint16, def uint32) string {
		return num(float64(int32(s.prop(id, def))) / emusPerPoint)
	}
	style = append(style, "box-sizing: border-box", "overflow: visible",
		"padding: "+inset(propTextTop, 45720)+"pt "+inset(propTextRight, 91440)+"pt "+
			inset(propTextBottom, 45720)+"pt "+inset(propTextLeft, 91440)+"pt")
	if s.flag(propFillBools, 4, true) && float64(s.prop(propFillOpacity, fixedOne)) > 0 {
		if c, ok := odColor(s.prop(propFillColor, 0xFFFFFF)); ok {
			style = append(style, "background-color: "+c)
		}
	}
	if s.flag(propLineBools, 3, true) {
		if c, ok := odColor(s.prop(propLineColor, 0)); ok {
			lw := float64(s.prop(propLineWidth, defaultLineWidth)) / emusPerPoint
			style = append(style, "border: "+num(max(lw, 0.25))+"pt solid "+c)
		}
	}
	switch s.typ {
	case shapeTypeEllipse:
		style = append(style, "border-radius: 50%")
	case shapeTypeRoundRec:
		style = append(style, "border-radius: "+num(min(w, h)*float64(s.prop(propAdjustValue, 3600))/defaultGeoSize)+"pt")
	}
	b.WriteString(`<div style="` + attr(strings.Join(style, "; ")) + `">`)
	b.WriteString(p.body(d.paras, nil))
	b.WriteString("</div>")
}

// attr returns s escaped for an attribute.
func attr(s string) string {
	return strings.NewReplacer("&", "&amp;", `"`, "&#34;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// odColor returns an OfficeArtCOLORREF (MS-ODRAW 2.2.2) as a color of CSS,
// or false for a color of a scheme, of a palette or of the system, which
// the package does not know.
func odColor(v uint32) (string, bool) {
	if v>>24&0x19 != 0 {
		return "", false
	}
	return fmt.Sprintf("#%02x%02x%02x", v&0xFF, v>>8&0xFF, v>>16&0xFF), true
}

// shapeSVG returns the SVG image of the shape s, of w by h points, and the
// space around the shape in the image, in points, which holds the half of
// its line that is outside the shape. It returns nil for a shape that
// draws nothing, or whose geometry the package does not know.
func shapeSVG(s *odShape, w, h float64) ([]byte, float64) {
	var body strings.Builder
	lw := 0.0
	if !drawShape(&body, s, [4]float64{0, 0, w, h}, 0, &lw) {
		return nil, 0
	}
	pad := lw/2 + 0.5
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" width="%spt" height="%spt" viewBox="%s %s %s %s">%s</svg>`,
		num(w+2*pad), num(h+2*pad), num(-pad), num(-pad), num(w+2*pad), num(h+2*pad), body.String()), pad
}

// drawShape writes the SVG elements of the shape s in the box r: left,
// top, right and bottom. A group writes its children, from the
// coordinates of the group into r. lw takes the widest line. It reports
// whether it wrote an element.
func drawShape(b *strings.Builder, s *odShape, r [4]float64, depth int, lw *float64) bool {
	if depth > maxShapeDepth {
		return false
	}
	if s.flags&fspGroup != 0 {
		g := s.group
		gw, gh := g[2]-g[0], g[3]-g[1]
		if gw == 0 || gh == 0 {
			return false
		}
		sx, sy := (r[2]-r[0])/gw, (r[3]-r[1])/gh
		wrote := false
		for _, c := range s.children {
			if !c.hasAnchor {
				continue
			}
			a := c.anchor
			cr := [4]float64{r[0] + (a[0]-g[0])*sx, r[1] + (a[1]-g[1])*sy, r[0] + (a[2]-g[0])*sx, r[1] + (a[3]-g[1])*sy}
			if drawShape(b, c, cr, depth+1, lw) {
				wrote = true
			}
		}
		return wrote
	}
	x, y, w, h := r[0], r[1], r[2]-r[0], r[3]-r[1]
	// a shape that turns between 45 and 135 degrees, or between 225 and
	// 315, has the box of its anchor turned by 90 degrees
	rot := float64(int32(s.prop(propRotation, 0))) / fixedOne
	rot = math.Mod(math.Mod(rot, 360)+360, 360)
	if rot >= 45 && rot < 135 || rot >= 225 && rot < 315 {
		x, y, w, h = x+(w-h)/2, y+(h-w)/2, h, w
	}
	geom := shapeGeometry(s, w, h)
	if geom == "" {
		return false
	}
	fill := "none"
	if s.typ != shapeTypeLine && s.typ != shapeTypeConnect && s.flag(propFillBools, 4, true) {
		if c, ok := odColor(s.prop(propFillColor, 0xFFFFFF)); ok {
			fill = c
			if o := float64(s.prop(propFillOpacity, fixedOne)) / fixedOne; o < 1 {
				fill += `" fill-opacity="` + num(max(o, 0))
			}
		}
	}
	stroke := "none"
	if s.flag(propLineBools, 3, true) {
		if c, ok := odColor(s.prop(propLineColor, 0)); ok {
			width := min(max(float64(s.prop(propLineWidth, defaultLineWidth))/emusPerPoint, 0.25), 72)
			*lw = max(*lw, width)
			dash := dashes(s.prop(propLineDashing, 0), width)
			stroke = c + `" stroke-width="` + num(width)
			if dash != "" {
				stroke += `" stroke-dasharray="` + dash
			}
			if (s.typ == shapeTypeLine || s.typ == shapeTypeConnect) && dash == "" && (w != 0 || h != 0) {
				// a solid straight line is the area of its stroke: okage
				// draws no stroke of less than a pixel whose path has no
				// width or no height
				n := width / 2 / math.Hypot(w, h)
				nx, ny := -h*n, w*n
				geom = fmt.Sprintf("M%s %s L%s %s L%s %s L%s %s Z", num(nx), num(ny), num(w+nx), num(h+ny),
					num(w-nx), num(h-ny), num(-nx), num(-ny))
				fill, stroke = c, "none"
			}
		}
	}
	if fill == "none" && stroke == "none" {
		return false
	}
	var tr []string
	tr = append(tr, "translate("+num(x)+" "+num(y)+")")
	if rot != 0 {
		tr = append(tr, "rotate("+num(rot)+" "+num(w/2)+" "+num(h/2)+")")
	}
	if s.flags&(fspFlipH|fspFlipV) != 0 {
		fx, fy := 1.0, 1.0
		if s.flags&fspFlipH != 0 {
			fx = -1
		}
		if s.flags&fspFlipV != 0 {
			fy = -1
		}
		tr = append(tr, "translate("+num(w/2)+" "+num(h/2)+") scale("+num(fx)+" "+num(fy)+") translate("+num(-w/2)+" "+num(-h/2)+")")
	}
	fmt.Fprintf(b, `<path transform="%s" d="%s" fill="%s" stroke="%s"/>`, strings.Join(tr, " "), geom, fill, stroke)
	return true
}

// dashes returns the dashes of a line of the width w for the lineDashing
// of MS-ODRAW 2.3.8.17, or "" for a solid line.
func dashes(v uint32, w float64) string {
	unit := func(vs ...float64) string {
		parts := make([]string, len(vs))
		for i, d := range vs {
			parts[i] = num(d * w)
		}
		return strings.Join(parts, " ")
	}
	switch v {
	case 1, 6:
		return unit(4, 3)
	case 2, 5:
		return unit(1, 1)
	case 3, 7:
		return unit(4, 3, 1, 3)
	case 4, 9:
		return unit(4, 3, 1, 3, 1, 3)
	case 8:
		return unit(8, 3)
	case 10:
		return unit(8, 3, 1, 3)
	}
	return ""
}

// shapeGeometry returns the path of SVG of the shape s in a box of w by h,
// or "" for a shape whose geometry the package does not know.
func shapeGeometry(s *odShape, w, h float64) string {
	adj := float64(s.prop(propAdjustValue, 0))
	poly := func(pts ...float64) string {
		var b strings.Builder
		for i := 0; i+1 < len(pts); i += 2 {
			if i == 0 {
				b.WriteString("M")
			} else {
				b.WriteString(" L")
			}
			b.WriteString(num(pts[i]) + " " + num(pts[i+1]))
		}
		b.WriteString(" Z")
		return b.String()
	}
	share := func(def float64) float64 {
		if _, ok := s.props[propAdjustValue]; !ok {
			adj = def
		}
		return min(max(adj/defaultGeoSize, 0), 1)
	}
	switch s.typ {
	case shapeTypeRect, shapeTypeTextBox:
		return poly(0, 0, w, 0, w, h, 0, h)
	case shapeTypeRoundRec:
		rx := min(w, h) * share(3600)
		return fmt.Sprintf("M%s 0 H%s A%s %s 0 0 1 %s %s V%s A%s %s 0 0 1 %s %s H%s A%s %s 0 0 1 0 %s V%s A%s %s 0 0 1 %s 0 Z",
			num(rx), num(w-rx), num(rx), num(rx), num(w), num(rx), num(h-rx), num(rx), num(rx), num(w-rx), num(h),
			num(rx), num(rx), num(rx), num(h-rx), num(rx), num(rx), num(rx), num(rx))
	case shapeTypeEllipse:
		return fmt.Sprintf("M0 %s A%s %s 0 1 0 %s %s A%s %s 0 1 0 0 %s Z",
			num(h/2), num(w/2), num(h/2), num(w), num(h/2), num(w/2), num(h/2), num(h/2))
	case shapeTypeDiamond:
		return poly(w/2, 0, w, h/2, w/2, h, 0, h/2)
	case shapeTypeTriangle:
		return poly(w*share(10800), 0, w, h, 0, h)
	case shapeTypeRightTri:
		return poly(0, 0, w, h, 0, h)
	case shapeTypeParallel:
		o := w * share(5400)
		return poly(o, 0, w, 0, w-o, h, 0, h)
	case shapeTypeHexagon:
		o := w * share(5400)
		return poly(o, 0, w-o, 0, w, h/2, w-o, h, o, h, 0, h/2)
	case shapeTypeLine, shapeTypeConnect:
		return "M0 0 L" + num(w) + " " + num(h)
	case shapeTypeNone:
		return freeform(s, w, h)
	}
	return ""
}

// freeform returns the path of a shape of its own geometry: its vertices
// and its segments, in the box of its geometry, scaled to w by h (MS-ODRAW
// 2.3.6).
func freeform(s *odShape, w, h float64) string {
	pts := vertices(s.complex[propVertices])
	if len(pts) == 0 {
		return ""
	}
	gl, gt := float64(int32(s.prop(propGeoLeft, 0))), float64(int32(s.prop(propGeoTop, 0)))
	gr, gb := float64(int32(s.prop(propGeoRight, defaultGeoSize))), float64(int32(s.prop(propGeoBottom, defaultGeoSize)))
	if gr == gl || gb == gt {
		return ""
	}
	pt := func(i int) string {
		return num((pts[i][0]-gl)*w/(gr-gl)) + " " + num((pts[i][1]-gt)*h/(gb-gt))
	}
	var b strings.Builder
	segs := s.complex[propSegmentInfo]
	if len(segs) < 6 {
		// a polyline through the vertices
		for i := range pts {
			if i == 0 {
				b.WriteString("M")
			} else {
				b.WriteString(" L")
			}
			b.WriteString(pt(i))
		}
		return b.String()
	}
	cb := int(le.Uint16(segs[4:]))
	if cb != 2 && cb != 0xFFF0 {
		return ""
	}
	at := 0
	for i := 6; i+2 <= len(segs) && at <= len(pts); i += 2 {
		v := le.Uint16(segs[i:])
		typ, count := int(v>>13), int(v&0x1FFF)
		switch typ {
		case 0, 1:
			if b.Len() == 0 && at > 0 {
				// a path starts at the point before its first segment
				b.WriteString("M" + pt(at-1))
			}
			n := max(count, 1)
			if typ == 1 {
				n *= 3
			}
			if at+n > len(pts) {
				return strings.TrimSpace(b.String())
			}
			for k := range n {
				switch {
				case b.Len() == 0:
					b.WriteString("M")
				case typ == 0:
					b.WriteString(" L")
				case k%3 == 0:
					b.WriteString(" C")
				default:
					b.WriteString(" ")
				}
				b.WriteString(pt(at + k))
			}
			at += n
		case 2:
			if at >= len(pts) {
				return strings.TrimSpace(b.String())
			}
			b.WriteString(" M" + pt(at))
			at++
		case 3:
			b.WriteString(" Z")
		case 4:
			return strings.TrimSpace(b.String())
		case 5, 6:
			// an escape uses count vertices
			at += int(v & 0xFF)
		}
	}
	return strings.TrimSpace(b.String())
}

// vertices reads the points of an IMsoArray of the property pVertices: a
// count, a count of the room, the size of each point, then the points.
func vertices(b []byte) [][2]float64 {
	if len(b) < 6 {
		return nil
	}
	n, cb := int(le.Uint16(b)), int(le.Uint16(b[4:]))
	b = b[6:]
	size := cb
	if cb == 0xFFF0 {
		size = 4
	}
	if size != 4 && size != 8 {
		return nil
	}
	n = min(n, len(b)/size, maxShapeVertices)
	out := make([][2]float64, n)
	for i := range n {
		e := b[size*i:]
		if size == 4 {
			out[i] = [2]float64{float64(int16(le.Uint16(e))), float64(int16(le.Uint16(e[2:])))}
		} else {
			out[i] = [2]float64{float64(int32(le.Uint32(e))), float64(int32(le.Uint32(e[4:])))}
		}
	}
	return out
}
