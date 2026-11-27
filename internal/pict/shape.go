package pict

import (
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/raster"
)

// point is a point of QuickDraw: h across and v down.
type point struct {
	h, v int
}

// pattern is a pattern of QuickDraw: 8 by 8 bits, which draw with the
// colors of the foreground and the background, or a pattern of pixels with
// colors of its own, or one color.
type pattern struct {
	bits [8]byte
	// tile is the image of a pattern of pixels, or nil.
	tile *image.NRGBA
	// rgb is set for a pattern of one color, c.
	rgb bool
	c   color.NRGBA
}

// black is the pattern of all the bits, and white the pattern of none.
var (
	black = pattern{bits: [8]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}}
	white = pattern{}
)

// The transfer modes of QuickDraw that the shapes take.
const (
	patCopy = 8
	patXor  = 10
)

// maxTile bounds the size of a pattern of pixels, and maxTiles the number
// of the shaders of patterns that a picture keeps.
const (
	maxTile  = 64
	maxTiles = 256
)

// state is the state of the drawing of a picture: the pen, the patterns,
// the colors, the origin, the clip and the text.
type state struct {
	pen     point
	penSize point
	penMode int
	penPat  pattern
	fillPat pattern
	bkPat   pattern
	fg, bg  color.NRGBA
	// ovSize is the size of the corners of a rounded rectangle.
	ovSize point
	// origin is the origin of the coordinates of the opcodes in the
	// coordinates of the frame.
	origin point
	// clip is the clip region, or nil, and clipBox its bounds.
	clip    *okage.Path
	clipBox rect

	// last is the rectangle of the last rectangle, rounded rectangle, oval
	// or arc, which the opcodes of the same shape draw again. lastPoly and
	// lastRgn are the last polygon and the last region.
	last     rect
	lastPoly []point
	lastRgn  region

	txLoc   point
	txFont  int
	txFace  int
	txMode  int
	txSize  int
	spExtra float32
	// ratio scales the text across and down.
	ratio [2]float32
	// fonts holds the names of the fonts by their numbers.
	fonts map[int]string
}

// newState returns the state at the start of a picture.
func newState() state {
	return state{
		penSize: point{1, 1},
		penMode: patCopy,
		penPat:  black,
		fillPat: black,
		bkPat:   white,
		fg:      color.NRGBA{0, 0, 0, 255},
		bg:      color.NRGBA{255, 255, 255, 255},
		txMode:  1,
		ratio:   [2]float32{1, 1},
	}
}

// oldColor returns the color of a color of the first QuickDraw, such as
// blackColor (33), with the values of the palette of the Macintosh.
func oldColor(v int) color.NRGBA {
	switch v {
	case 30:
		return color.NRGBA{255, 255, 255, 255}
	case 69:
		return color.NRGBA{0xFC, 0xF3, 0x05, 255}
	case 137:
		return color.NRGBA{0xF2, 0x08, 0x84, 255}
	case 205:
		return color.NRGBA{0xDD, 0x08, 0x06, 255}
	case 273:
		return color.NRGBA{0x02, 0xAB, 0xEA, 255}
	case 341:
		return color.NRGBA{0x00, 0x80, 0x11, 255}
	case 409:
		return color.NRGBA{0x00, 0x00, 0xD4, 255}
	}
	return color.NRGBA{0, 0, 0, 255}
}

// ratio returns numer / denom of TxRatio, or 1 for a ratio out of the range
// from 1/16 to 16.
func ratio(numer, denom int) float32 {
	if numer <= 0 || denom <= 0 || numer > 16*denom || denom > 16*numer {
		return 1
	}
	return float32(numer) / float32(denom)
}

// okage returns the rectangle r of okage.
func (r rect) okage() (okage.Rect, bool) {
	return okage.RectFromLTRB(float32(r.left), float32(r.top), float32(r.right), float32(r.bottom))
}

// pattern reads the 8 bytes of a Pattern.
func (r *reader) pattern() pattern {
	var pat pattern
	copy(pat.bits[:], r.b[r.pos:r.pos+8])
	r.pos += 8
	return pat
}

// tileKey identifies the shader of the bits of a pattern in two colors.
type tileKey struct {
	bits    [8]byte
	on, off color.NRGBA
}

// shader returns the shader that paints the pattern pat in the transfer
// mode, and whether the paint inverts the colors under it, for the modes
// of Xor. It returns nil for a paint that changes nothing. The modes of
// arithmetic paint as patCopy, and transparent as patOr.
func (r *reader) shader(pat pattern, mode int) (okage.Shader, bool) {
	m := mode &^ 64
	switch {
	case m >= 8 && m < 16:
		m -= 8
	case m == 36:
		m = 1
	case m >= 16:
		m = 0
	}
	bits := pat.bits
	if m&4 != 0 {
		// the modes of not invert the pattern
		for i := range bits {
			bits[i] = ^bits[i]
		}
	}
	fg, bg := r.st.fg, r.st.bg
	none := color.NRGBA{}
	xor := m&3 == 2
	switch {
	case pat.tile != nil && m&3 != 3:
		if xor {
			return okage.SolidColor(okage.ColorFromRGBA8(255, 255, 255, 255)), true
		}
		return r.tileShader(pat.tile), false
	case pat.rgb:
		bits, fg = black.bits, pat.c
	}
	var on, off color.NRGBA
	switch m & 3 {
	case 0:
		on, off = fg, bg
	case 1:
		on, off = fg, none
	case 2:
		on, off = color.NRGBA{255, 255, 255, 255}, none
	default:
		on, off = bg, none
	}
	switch bits {
	case black.bits:
		return solid(on), xor
	case white.bits:
		return solid(off), xor
	}
	key := tileKey{bits, on, off}
	if sh, ok := r.tiles[key]; ok {
		return sh, xor
	}
	tile := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			c := off
			if bits[y]>>(7-x)&1 == 1 {
				c = on
			}
			tile.SetNRGBA(x, y, c)
		}
	}
	sh := r.tileShader(tile)
	if r.tiles == nil {
		r.tiles = make(map[tileKey]okage.Shader)
	}
	if len(r.tiles) < maxTiles {
		r.tiles[key] = sh
	}
	return sh, xor
}

// solid returns the shader of the color c, or nil for a transparent color.
func solid(c color.NRGBA) okage.Shader {
	if c.A == 0 {
		return nil
	}
	return okage.SolidColor(okage.ColorFromRGBA8(c.R, c.G, c.B, c.A))
}

// tileShader returns the shader that repeats tile, one pixel for each unit
// of the picture, from the origin of the frame.
func (r *reader) tileShader(tile image.Image) okage.Shader {
	pm, err := raster.Pixmap(tile)
	if err != nil {
		return nil
	}
	return okage.NewPattern(pm.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityNearest, 1, okage.IdentityTransform())
}

// cover adds the area of b in the frame and the clip to the area of the
// picture, and reports whether b covers a part of them.
func (r *reader) cover(b okage.Rect) bool {
	box := r.frame
	if r.st.clip != nil {
		c := r.st.clipBox
		box = rect{max(box.top, c.top), max(box.left, c.left), min(box.bottom, c.bottom), min(box.right, c.right)}
	}
	w := min(float64(b.Right()), float64(box.right)) - max(float64(b.Left()), float64(box.left))
	h := min(float64(b.Bottom()), float64(box.bottom)) - max(float64(b.Top()), float64(box.top))
	if !(w > 0 && h > 0) {
		return false
	}
	r.area += w * h
	return true
}

// begin starts a command in the clip region, and end ends it.
func (r *reader) begin() {
	if r.st.clip != nil {
		r.list.Save()
		r.list.ClipPath(r.st.clip, okage.FillRuleWinding)
	}
}

func (r *reader) end() {
	if r.st.clip != nil {
		r.list.Restore()
	}
}

// paint fills the path p with the pattern pat in the transfer mode.
func (r *reader) paint(p *okage.Path, rule okage.FillRule, pat pattern, mode int) {
	if p == nil {
		return
	}
	r.paintWith(p.Bounds(), pat, mode, func(sh okage.Shader) {
		r.list.FillPath(p, rule, sh)
	})
}

// paintWith draws with fill the paint of the pattern pat in the transfer
// mode, over the bounds b. The modes of Xor draw in a group that inverts
// the colors under it.
func (r *reader) paintWith(b okage.Rect, pat pattern, mode int, fill func(okage.Shader)) {
	sh, xor := r.shader(pat, mode)
	if sh == nil || !r.cover(b) {
		return
	}
	r.begin()
	if xor {
		r.list.BeginGroup(display.Group{Opacity: 1, Blend: okage.BlendModeDifference})
	}
	fill(sh)
	if xor {
		r.list.EndGroup()
	}
	r.end()
	r.drew, r.vector = true, true
}

// verb paints the path of a shape with the verb of the opcode op, from
// its low 3 bits: paint, erase, invert or fill. The verb frame takes the
// path of the outline, which outline returns for the size of the pen.
func (r *reader) verb(op int, p *okage.Path, rule okage.FillRule, outline func(pw, ph float32) (*okage.Path, okage.FillRule)) {
	st := &r.st
	switch op & 7 {
	case 0:
		if st.penSize.h <= 0 || st.penSize.v <= 0 {
			return
		}
		q, rule := outline(float32(st.penSize.h), float32(st.penSize.v))
		r.paint(q, rule, st.penPat, st.penMode)
	case 1:
		r.paint(p, rule, st.penPat, st.penMode)
	case 2:
		r.paint(p, rule, st.bkPat, patCopy)
	case 3:
		r.paint(p, rule, black, patXor)
	case 4:
		r.paint(p, rule, st.fillPat, patCopy)
	}
}

// line draws a line from a to b with the pen, which moves to b. The pen
// is a rectangle below and to the right of the point, so the line covers
// the hull of the pen at a and at b.
func (r *reader) line(a, b point) {
	st := &r.st
	st.pen = b
	if st.penSize.h <= 0 || st.penSize.v <= 0 {
		return
	}
	pb := okage.NewPathBuilder()
	penLine(pb, a, b, float32(st.penSize.h), float32(st.penSize.v))
	if p, ok := pb.Finish(); ok {
		r.paint(p, okage.FillRuleWinding, st.penPat, st.penMode)
	}
}

// penLine adds to pb the hull of the pen of pw by ph at a and at b,
// counterclockwise, so that the hulls of a polygon join with the winding
// rule.
func penLine(pb *okage.PathBuilder, a, b point, pw, ph float32) {
	ax, ay, bx, by := float32(a.h), float32(a.v), float32(b.h), float32(b.v)
	pts := []okage.Point{
		{X: ax, Y: ay}, {X: ax + pw, Y: ay}, {X: ax + pw, Y: ay + ph}, {X: ax, Y: ay + ph},
		{X: bx, Y: by}, {X: bx + pw, Y: by}, {X: bx + pw, Y: by + ph}, {X: bx, Y: by + ph},
	}
	hull := convexHull(pts)
	for i, p := range hull {
		if i == 0 {
			pb.MoveTo(p.X, p.Y)
		} else {
			pb.LineTo(p.X, p.Y)
		}
	}
	pb.Close()
}

// convexHull returns the convex hull of pts, in one direction, with the
// monotone chain of Andrew.
func convexHull(pts []okage.Point) []okage.Point {
	slices.SortFunc(pts, func(a, b okage.Point) int {
		switch {
		case a.X != b.X:
			return cmpF(a.X, b.X)
		default:
			return cmpF(a.Y, b.Y)
		}
	})
	cross := func(o, a, b okage.Point) float32 {
		return (a.X-o.X)*(b.Y-o.Y) - (a.Y-o.Y)*(b.X-o.X)
	}
	hull := make([]okage.Point, 0, 2*len(pts))
	for _, p := range pts {
		for len(hull) >= 2 && cross(hull[len(hull)-2], hull[len(hull)-1], p) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p)
	}
	lower := len(hull) + 1
	for i := len(pts) - 2; i >= 0; i-- {
		p := pts[i]
		for len(hull) >= lower && cross(hull[len(hull)-2], hull[len(hull)-1], p) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p)
	}
	return hull[:len(hull)-1]
}

// cmpF compares two numbers.
func cmpF(a, b float32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// shape draws the rectangle, the rounded rectangle, the oval or the arc of
// the opcode op in the last rectangle. start and arc are the angles of an
// arc, in degrees clockwise from the top.
func (r *reader) shape(op, start, arc int) {
	b := r.st.last
	if b.empty() {
		return
	}
	kind := op >> 4
	ow, oh := float32(r.st.ovSize.h), float32(r.st.ovSize.v)
	path := func(b rect, inset point) *okage.Path {
		l, t := float32(b.left+inset.h), float32(b.top+inset.v)
		rt, bt := float32(b.right-inset.h), float32(b.bottom-inset.v)
		if rt <= l || bt <= t {
			return nil
		}
		pb := okage.NewPathBuilder()
		switch kind {
		case 3:
			if rr, ok := okage.RectFromLTRB(l, t, rt, bt); ok {
				pb.PushRect(rr)
			}
		case 4:
			pushRRect(pb, l, t, rt, bt, (ow-2*float32(inset.h))/2, (oh-2*float32(inset.v))/2)
		case 5:
			if rr, ok := okage.RectFromLTRB(l, t, rt, bt); ok {
				pb.PushOval(rr)
			}
		}
		p, ok := pb.Finish()
		if !ok {
			return nil
		}
		return p
	}
	if kind == 6 {
		r.arc(op, b, start, arc)
		return
	}
	outline := func(pw, ph float32) (*okage.Path, okage.FillRule) {
		outer := path(b, point{})
		inner := path(b, point{int(pw), int(ph)})
		return ring(outer, inner), okage.FillRuleEvenOdd
	}
	r.verb(op, path(b, point{}), okage.FillRuleWinding, outline)
}

// ring returns the path of outer with the hole inner, for the even-odd
// rule, or outer when inner is nil.
func ring(outer, inner *okage.Path) *okage.Path {
	if outer == nil || inner == nil {
		return outer
	}
	pb := okage.NewPathBuilder()
	pb.PushPath(outer)
	pb.PushPath(inner)
	p, ok := pb.Finish()
	if !ok {
		return outer
	}
	return p
}

// kappa places the control points of a cubic curve that draws a quarter of
// an ellipse.
const kappa = 0.5522848

// pushRRect adds to pb the rectangle from l, t to r, b with corners of
// ellipses of the radii rx and ry.
func pushRRect(pb *okage.PathBuilder, l, t, r, b, rx, ry float32) {
	rx, ry = min(rx, (r-l)/2), min(ry, (b-t)/2)
	if rx <= 0 || ry <= 0 {
		if rr, ok := okage.RectFromLTRB(l, t, r, b); ok {
			pb.PushRect(rr)
		}
		return
	}
	kx, ky := kappa*rx, kappa*ry
	pb.MoveTo(l+rx, t)
	pb.LineTo(r-rx, t)
	pb.CubicTo(r-rx+kx, t, r, t+ry-ky, r, t+ry)
	pb.LineTo(r, b-ry)
	pb.CubicTo(r, b-ry+ky, r-rx+kx, b, r-rx, b)
	pb.LineTo(l+rx, b)
	pb.CubicTo(l+rx-kx, b, l, b-ry+ky, l, b-ry)
	pb.LineTo(l, t+ry)
	pb.CubicTo(l, t+ry-ky, l+rx-kx, t, l+rx, t)
	pb.Close()
}

// arc draws the arc of the oval in b with the verb of op. The angles are
// those of the rectangle: 45 degrees is the top right corner. A frame
// draws the curve, and the other verbs the wedge.
func (r *reader) arc(op int, b rect, start, arc int) {
	if arc == 0 {
		return
	}
	arc = max(-360, min(arc, 360))
	cx, cy := float32(b.left+b.right)/2, float32(b.top+b.bottom)/2
	rx, ry := float32(b.dx())/2, float32(b.dy())/2
	// one segment for each 2 degrees
	n := max(2, int(math.Abs(float64(arc)))/2)
	at := func(i int, rx, ry float32) (float32, float32) {
		a := (float64(start) + float64(arc)*float64(i)/float64(n)) * math.Pi / 180
		return cx + rx*float32(math.Sin(a)), cy - ry*float32(math.Cos(a))
	}
	wedge := func() *okage.Path {
		pb := okage.NewPathBuilder()
		pb.MoveTo(cx, cy)
		for i := 0; i <= n; i++ {
			pb.LineTo(at(i, rx, ry))
		}
		pb.Close()
		p, ok := pb.Finish()
		if !ok {
			return nil
		}
		return p
	}
	outline := func(pw, ph float32) (*okage.Path, okage.FillRule) {
		irx, iry := rx-pw, ry-ph
		if irx <= 0 || iry <= 0 {
			return wedge(), okage.FillRuleWinding
		}
		pb := okage.NewPathBuilder()
		pb.MoveTo(at(0, rx, ry))
		for i := 1; i <= n; i++ {
			pb.LineTo(at(i, rx, ry))
		}
		for i := n; i >= 0; i-- {
			pb.LineTo(at(i, irx, iry))
		}
		pb.Close()
		p, ok := pb.Finish()
		if !ok {
			return nil, okage.FillRuleWinding
		}
		return p, okage.FillRuleWinding
	}
	r.verb(op, wedge(), okage.FillRuleWinding, outline)
}

// polygon reads a Polygon: its size, its bounds and its points.
func (r *reader) polygon() ([]point, bool) {
	start := r.pos
	size := r.u16()
	if size < 10 || start+size > len(r.b) {
		return nil, false
	}
	r.pos += 8
	n := (size - 10) / 4
	pts := make([]point, n)
	for i := range pts {
		pts[i] = r.point()
	}
	r.pos = start + size
	return pts, true
}

// poly draws the polygon pts with the verb of op. A frame draws the lines
// from each point to the next with the pen, and the other verbs fill the
// polygon with the even-odd rule.
func (r *reader) poly(op int, pts []point) {
	if len(pts) < 2 {
		return
	}
	pb := okage.NewPathBuilder()
	for i, p := range pts {
		if i == 0 {
			pb.MoveTo(float32(p.h), float32(p.v))
		} else {
			pb.LineTo(float32(p.h), float32(p.v))
		}
	}
	pb.Close()
	path, ok := pb.Finish()
	if !ok {
		path = nil
	}
	outline := func(pw, ph float32) (*okage.Path, okage.FillRule) {
		pb := okage.NewPathBuilder()
		for i := 1; i < len(pts); i++ {
			penLine(pb, pts[i-1], pts[i], pw, ph)
		}
		p, ok := pb.Finish()
		if !ok {
			return nil, okage.FillRuleWinding
		}
		return p, okage.FillRuleWinding
	}
	r.verb(op, path, okage.FillRuleEvenOdd, outline)
	if op&7 == 0 {
		r.st.pen = pts[len(pts)-1]
	}
}

// region is a region of QuickDraw: its bounds, and the rectangles that
// make it, or nil for a region that is its bounds.
type region struct {
	box   rect
	rects []rect
}

// maxRects bounds the number of the rectangles of a region.
const maxRects = 1 << 14

// region reads a Region. The data of a region that is not a rectangle
// holds lines: the coordinate down of the line, the coordinates across
// where the region starts or stops against the line above, and 0x7FFF. A
// last 0x7FFF ends the lines.
func (r *reader) region() (region, bool) {
	start := r.pos
	size := r.u16()
	if size < 10 || start+size > len(r.b) {
		return region{}, false
	}
	g := region{box: r.prect()}
	end := start + size
	defer func() { r.pos = end }()
	if size == 10 {
		return g, true
	}
	o := r.st.origin
	var cur, xs []int
	prev := 0
	g.rects = []rect{}
	for r.pos+2 <= end && len(g.rects) < maxRects {
		y := r.u16()
		if y == 0x7FFF {
			break
		}
		v := int(int16(y)) - o.v
		for i := 0; i+1 < len(cur) && v > prev; i += 2 {
			g.rects = append(g.rects, rect{prev, cur[i], v, cur[i+1]})
		}
		xs = xs[:0]
		for r.pos+2 <= end {
			x := r.u16()
			if x == 0x7FFF {
				break
			}
			xs = append(xs, int(int16(x))-o.h)
		}
		slices.Sort(xs)
		cur = toggle(cur, xs)
		prev = v
	}
	return g, true
}

// toggle returns the points of a that b does not hold and those of b that
// a does not hold, in order. a and b are in order.
func toggle(a, b []int) []int {
	out := make([]int, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case j == len(b) || i < len(a) && a[i] < b[j]:
			out = append(out, a[i])
			i++
		case i == len(a) || b[j] < a[i]:
			out = append(out, b[j])
			j++
		default:
			i++
			j++
		}
	}
	return out
}

// path returns the path of the region, or nil for an empty region.
func (g region) path() *okage.Path {
	rects := g.rects
	if rects == nil {
		rects = []rect{g.box}
	}
	pb := okage.NewPathBuilder()
	for _, b := range rects {
		if rr, ok := b.okage(); ok {
			pb.PushRect(rr)
		}
	}
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// setClip sets the clip region to g. A rectangle that holds the frame
// clips nothing.
func (r *reader) setClip(g region) {
	f := r.frame
	if g.rects == nil && g.box.top <= f.top && g.box.left <= f.left && g.box.bottom >= f.bottom && g.box.right >= f.right {
		r.st.clip = nil
		return
	}
	p := g.path()
	if p == nil {
		// an empty clip region draws nothing
		r.st.clip, r.st.clipBox = okage.PathFromRect(okage.Rect{}), rect{}
		return
	}
	r.st.clip, r.st.clipBox = p, g.box
}

// maxMask bounds the number of the pixels of the mask of the frame of a
// region, and maxPen the size of its pen.
const (
	maxMask = 1 << 20
	maxPen  = 64
)

// regionOp draws the region g with the verb of op. The frame of a region
// is the part of the region that the region inset by the size of the pen
// does not hold. Past maxMask or maxPen, a frame paints the region.
func (r *reader) regionOp(op int, g region) {
	p := g.path()
	if p == nil {
		return
	}
	st := &r.st
	w, h := g.box.dx(), g.box.dy()
	pw, ph := st.penSize.h, st.penSize.v
	if op&7 != 0 || int64(w)*int64(h) > maxMask || pw > maxPen || ph > maxPen {
		r.verb(op, p, okage.FillRuleWinding, func(float32, float32) (*okage.Path, okage.FillRule) {
			return p, okage.FillRuleWinding
		})
		return
	}
	if pw <= 0 || ph <= 0 {
		return
	}
	inside := make([]bool, w*h)
	rects := g.rects
	if rects == nil {
		rects = []rect{g.box}
	}
	for _, b := range rects {
		for y := max(b.top, g.box.top); y < min(b.bottom, g.box.bottom); y++ {
			for x := max(b.left, g.box.left); x < min(b.right, g.box.right); x++ {
				inside[(y-g.box.top)*w+x-g.box.left] = true
			}
		}
	}
	inner := erode(inside, w, h, pw, 1)
	inner = erode(inner, w, h, ph, w)
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	for i, in := range inside {
		if in && !inner[i] {
			mask.Pix[i] = 255
		}
	}
	b, ok := g.box.okage()
	if !ok {
		return
	}
	t := okage.TransformFromTranslate(float32(g.box.left), float32(g.box.top))
	r.paintWith(b, st.penPat, st.penMode, func(sh okage.Shader) {
		r.list.FillMask(mask, t, display.ImageOptions{Opacity: 1}, sh)
	})
}

// erode returns the pixels of in that have n pixels of in on each side,
// along the lines of w by h pixels whose pixels are step apart: 1 across
// and w down.
func erode(in []bool, w, h, n, step int) []bool {
	out := make([]bool, len(in))
	lines, length := h, w
	stride := w
	if step != 1 {
		lines, length, stride = w, h, 1
	}
	for l := range lines {
		base := l * stride
		// run counts the pixels of in that end at i
		run := 0
		for i := range length {
			if in[base+i*step] {
				run++
			} else {
				run = 0
			}
			// the pixel i-n has n pixels of in on each side once the run
			// ending at i holds 2n+1 pixels
			if run >= 2*n+1 {
				out[base+(i-n)*step] = true
			}
		}
	}
	return out
}
