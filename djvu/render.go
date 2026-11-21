package djvu

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
	"sync"
	"sync/atomic"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/djvu/internal/doc"
	"github.com/xo/ivy/djvu/internal/iw44"
	"github.com/xo/ivy/djvu/internal/jb2"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/jpeg2000"
	"github.com/xo/ivy/internal/raster"
)

// maxReduction is the largest number of pixels of a page, across and down,
// that one pixel of a composite holds. The coverage of a pixel of the
// composite counts to (maxReduction+1)*(maxReduction+1)*unit*unit in 32
// bits.
const maxReduction = 254

// Page is a page of a DjVu document. It decodes its layers on its first
// render, and keeps them and the last composite while the document keeps
// the page among its recent pages.
type Page struct {
	d    *Document
	p    *doc.Page
	n    int
	w, h float32

	// mu makes a render that decodes the layers or composites them wait
	// for another render of the same page.
	mu     sync.Mutex
	layers atomic.Pointer[layers]
	// cached is the size of the layers in bytes. Document.mu guards it.
	cached int64
}

// The interfaces that a Page implements.
var (
	_ ivy.Page       = (*Page)(nil)
	_ display.Drawer = (*Page)(nil)
)

// layers are the decoded layers of a page, and the last composite.
type layers struct {
	mask   *jb2.Image
	pal    *doc.Palette
	bg, fg source
	// err holds the errors of the layers that did not decode.
	err error

	comp *image.RGBA
	// compErr holds the errors of the layers that the composite left out.
	compErr error
}

// source is a decoded IW44 or JPEG layer, of w by h pixels.
type source struct {
	iw   *iw44.Decoder
	img  *image.RGBA
	w, h int
}

// bytes returns the memory that the layers take, about.
func (l *layers) bytes() int64 {
	var n int64
	if l.mask != nil {
		for _, s := range l.mask.Shapes {
			n += int64(len(s.Pix))
		}
		n += 24 * int64(len(l.mask.Blits))
	}
	for _, s := range []source{l.bg, l.fg} {
		switch {
		case s.iw != nil:
			n += 6 * int64(s.w) * int64(s.h)
		case s.img != nil:
			n += int64(len(s.img.Pix))
		}
	}
	if l.comp != nil {
		n += int64(len(l.comp.Pix))
	}
	return n
}

// pageSize returns the size of a page in points, after its rotation.
func pageSize(in doc.Info) (float32, float32) {
	w, h := float32(in.Width)*72/float32(in.DPI), float32(in.Height)*72/float32(in.DPI)
	if in.Rotate == 90 || in.Rotate == 270 {
		return h, w
	}
	return w, h
}

// Size satisfies the ivy.Page interface. It is the size of the page at the
// resolution of its INFO chunk, after its rotation.
func (p *Page) Size() (float32, float32) {
	return p.w, p.h
}

// Render satisfies the ivy.Page interface. It composites the layers of the
// page at the resolution that t gives, up to the resolution of the page,
// and draws the composite as one image. A layer that does not decode is
// left out, and Render returns an *ivy.PartialError.
func (p *Page) Render(ctx context.Context, dst *okage.Pixmap, t okage.Transform) error {
	return p.draw(ctx, display.NewOkage(dst, t), t, false)
}

// Draw satisfies the display.Drawer interface (D58). It draws the composite
// of the layers at the resolution of the page, whatever res is, as one
// image. When d is a display.Linker or a display.Bookmarker, it then gives
// d the outline and the links of the page.
func (p *Page) Draw(ctx context.Context, d display.Device, res float32) error {
	err := p.draw(ctx, d, okage.TransformFromScale(res, res), true)
	if _, ok := errors.AsType[*ivy.PartialError](err); err == nil || ok {
		if lerr := p.drawLinks(ctx, d); lerr != nil && err == nil {
			err = &ivy.PartialError{Err: lerr}
		}
	}
	return err
}

// draw composites the layers of the page and draws the composite onto
// dev, whose user space is the space of the page in points, after its
// rotation. t maps that space onto the pixels of the target, and sets the
// size of the composite. With full, the composite has the pixels of the
// page.
func (p *Page) draw(ctx context.Context, dev display.Device, t okage.Transform, full bool) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("rendering djvu page %d: %w", p.n+1, err)
	}
	defer recoverTo(fmt.Sprintf("rendering djvu page %d", p.n+1), &err)
	if p.p.Err != nil {
		return &ivy.PartialError{Err: fmt.Errorf("reading djvu page %d: %w", p.n+1, format(p.p.Err))}
	}
	in := p.p.Info
	if in.Width <= 0 || in.Height <= 0 {
		return fmt.Errorf("djvu page %d of %dx%d pixels: %w", p.n+1, in.Width, in.Height, ivy.ErrFormat)
	}
	w, h := float32(in.Width)*72/float32(in.DPI), float32(in.Height)*72/float32(in.DPI)
	rot := rotation(in.Rotate, w, h)
	page := t.PreConcat(rot)
	want := target(page, in)
	if full {
		want = image.Pt(in.Width, in.Height)
	}
	size, err := p.fit(want)
	if err != nil {
		return err
	}
	img, partial, err := p.composite(ctx, size)
	if err != nil {
		return err
	}
	place := okage.TransformFromScale(w/float32(size.X), h/float32(size.Y))
	dev.Save()
	dev.Concat(rot)
	dev.DrawImage(img, place, display.ImageOptions{Opacity: 1, Smooth: !aligned(page.PreConcat(place), size)})
	dev.Restore()
	if partial != nil {
		p.d.log.Debug("djvu: page drawn in part", "page", p.n+1, "err", partial)
		return &ivy.PartialError{Err: partial}
	}
	return nil
}

// rotation returns the transform that turns a page of w by h points by rot
// degrees clockwise, so that the page stays at the origin.
func rotation(rot int, w, h float32) okage.Transform {
	switch rot {
	case 90:
		return okage.TransformFromRow(0, 1, -1, 0, h, 0)
	case 180:
		return okage.TransformFromRow(-1, 0, 0, -1, w, h)
	case 270:
		return okage.TransformFromRow(0, -1, 1, 0, 0, w)
	}
	return okage.IdentityTransform()
}

// target returns the size of the composite of a page for the transform t
// from the points of the page, before its rotation, to the target: the
// size of the page on the target, but no more than the pixels of the page,
// and no less than 1/maxReduction of them.
func target(t okage.Transform, in doc.Info) image.Point {
	sx := math.Hypot(float64(t.SX), float64(t.KY)) * 72 / float64(in.DPI)
	sy := math.Hypot(float64(t.KX), float64(t.SY)) * 72 / float64(in.DPI)
	side := func(n int, s float64) int {
		low := (n + maxReduction - 1) / maxReduction
		if math.IsNaN(s) || s >= 1 {
			return n
		}
		// the size of the target rounds up, as ivy.RenderImage does
		return min(n, max(low, int(math.Ceil(float64(n)*s-1e-3))))
	}
	return image.Pt(side(in.Width, sx), side(in.Height, sy))
}

// aligned reports whether t maps the pixels of an image of size onto the
// pixels of the target one to one, with a turn by a multiple of 90 degrees
// at most, and a drift of less than a pixel across the image. The device
// then draws the image without smoothing, which keeps it sharp.
func aligned(t okage.Transform, size image.Point) bool {
	near := func(v float32, want float64, n int) bool {
		return math.Abs(math.Abs(float64(v))-want)*float64(n) < 0.5
	}
	whole := func(v float32) bool {
		return math.Abs(float64(v)-math.Round(float64(v))) < 0.01
	}
	if !whole(t.TX) || !whole(t.TY) {
		return false
	}
	straight := near(t.SX, 1, size.X) && near(t.SY, 1, size.Y) && near(t.KX, 0, size.Y) && near(t.KY, 0, size.X)
	turned := near(t.KY, 1, size.X) && near(t.KX, 1, size.Y) && near(t.SX, 0, size.X) && near(t.SY, 0, size.Y)
	return straight || turned
}

// composite returns the composite of the layers of the page at size, from
// the cache or new. It also returns the errors of the layers that did not
// decode.
func (p *Page) composite(ctx context.Context, size image.Point) (*image.RGBA, error, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := p.layers.Load()
	if l == nil {
		var err error
		if l, err = p.decode(ctx); err != nil {
			return nil, nil, err
		}
	}
	if l.comp == nil || l.comp.Rect.Size() != size {
		img, partial, err := p.build(ctx, l, size)
		if err != nil {
			return nil, nil, err
		}
		l.comp, l.compErr = img, partial
	}
	p.layers.Store(l)
	p.d.touch(p, l.bytes())
	return l.comp, errors.Join(l.err, l.compErr), nil
}

// fit returns size, made smaller until the composite is within the
// limits. A pixel of the composite takes 4 bytes, and its coverage and its
// color 6 more while the page builds.
func (p *Page) fit(size image.Point) (image.Point, error) {
	lim := p.d.lim
	in := p.p.Info
	for range 64 {
		cells := int64(size.X) * int64(size.Y)
		if cells <= lim.MaxPixels && 10*cells <= lim.MaxMemory {
			return size, nil
		}
		size = image.Pt(max(1, size.X*7/8), max(1, size.Y*7/8))
		if size.X*maxReduction < in.Width || size.Y*maxReduction < in.Height {
			break
		}
	}
	return image.Point{}, fmt.Errorf("djvu page %d of %dx%d pixels: %w", p.n+1, in.Width, in.Height, ivy.ErrLimit)
}

// decode decodes the layers of the page. It returns an error only for the
// context. The errors of the layers go in the field err of the result.
func (p *Page) decode(ctx context.Context) (*layers, error) {
	l := &layers{}
	var errs []error
	fail := func(what string, err error) {
		errs = append(errs, fmt.Errorf("decoding the %s of djvu page %d: %w", what, p.n+1, format(err)))
	}
	pd, pp, err := p.load()
	if err != nil {
		fail("page", err)
		l.err = errors.Join(errs...)
		return l, nil
	}
	mask, err := pd.Mask(pp)
	if err != nil {
		fail("mask", err)
	}
	l.mask = mask
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("rendering djvu page %d: %w", p.n+1, err)
	}
	if l.pal, err = pp.Palette(); err != nil {
		fail("palette", err)
		l.pal = nil
	}
	if l.bg, err = p.source(ctx, pp, "BG44", "BGjp", "BG2k"); err != nil {
		fail("background", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("rendering djvu page %d: %w", p.n+1, err)
	}
	if l.fg, err = p.source(ctx, pp, "FG44", "FGjp", "FG2k"); err != nil {
		fail("foreground", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("rendering djvu page %d: %w", p.n+1, err)
	}
	l.err = errors.Join(errs...)
	return l, nil
}

// load returns the page and its document. A page of a bundled document
// reads its component, and the components that it includes, from the
// source.
func (p *Page) load() (*doc.Document, *doc.Page, error) {
	if p.d.bundle == nil {
		return p.d.doc, p.p, nil
	}
	c, ok := p.d.bundle.pageComponent(p.n)
	if !ok {
		return nil, nil, fmt.Errorf("djvu page %d not in the directory: %w", p.n+1, ivy.ErrFormat)
	}
	return p.d.bundle.page(c)
}

// source decodes the IW44 layer of the chunks iw, the JPEG layer of the
// chunk jp, or the JPEG 2000 layer of the chunk j2k. A layer that decodes
// in part comes with its error.
func (p *Page) source(ctx context.Context, pp *doc.Page, iw, jp, j2k string) (source, error) {
	if cs := pp.Form.All(iw); len(cs) > 0 {
		dec, err := pp.Layer(iw)
		if dec == nil {
			return source{}, err
		}
		w, h, _ := iw44.Size(cs[0].Data)
		return source{iw: dec, w: w, h: h}, err
	}
	var img image.Image
	var err error
	if c := pp.Form.Find(jp); c != nil {
		img, err = raster.Decode(ctx, c.Data, p.d.lim)
	} else if c := pp.Form.Find(j2k); c != nil {
		img, err = decodeJPEG2000(ctx, c.Data, p.d.lim)
	} else {
		return source{}, nil
	}
	if err != nil {
		return source{}, err
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Rect, img, b.Min, draw.Src)
	return source{img: rgba, w: b.Dx(), h: b.Dy()}, nil
}

// plane is a layer at the scale of a composite: the pixels, and where the
// center of each falls on the page.
type plane struct {
	w, h   int
	n      int // the bytes of a pixel: 1 for gray, 3 for RGB, 4 for RGBA
	pix    []byte
	ox, oy float64 // the center of pixel (0, 0), in pixels of the page
	sx, sy float64 // the distance between two pixels, in pixels of the page
}

// plane returns the layer for a composite whose pixel holds r by r pixels
// of a page of width by height pixels, or nil if there is no layer. It
// reconstructs an IW44 layer only down to the scale that r needs.
func (s source) plane(width, height int, r float64) (*plane, error) {
	if s.w <= 0 || s.h <= 0 || (s.iw == nil && s.img == nil) {
		return nil, nil
	}
	rx, ry := ratio(width, height, s.w, s.h)
	if s.img != nil {
		return &plane{w: s.w, h: s.h, n: 4, pix: s.img.Pix, ox: rx / 2, oy: ry / 2, sx: rx, sy: ry}, nil
	}
	scale := 1
	for scale < 16 && float64(2*scale)*min(rx, ry) <= r {
		scale *= 2
	}
	im, err := s.iw.ImageScaled(scale)
	if err != nil {
		return nil, err
	}
	n := 3
	if im.Gray {
		n = 1
	}
	if len(im.Pix) < n*im.W*im.H {
		return nil, fmt.Errorf("iw44 image of %dx%d with %d bytes: %w", im.W, im.H, len(im.Pix), ivy.ErrFormat)
	}
	// the rows of a reduced IW44 image start at the bottom of the layer
	off := s.h - 1 - (im.H-1)*scale
	return &plane{
		w: im.W, h: im.H, n: n, pix: im.Pix,
		ox: rx / 2, oy: (float64(off) + 0.5) * ry,
		sx: float64(scale) * rx, sy: float64(scale) * ry,
	}, nil
}

// ratio returns the pixels of the page, across and down, for each pixel of
// a layer of w by h. A layer of DjVu is the page reduced by a whole number,
// rounded up. A layer of another size stretches over the page.
func ratio(width, height, w, h int) (float64, float64) {
	for red := 1; red <= 12; red++ {
		if (width+red-1)/red == w && (height+red-1)/red == h {
			return float64(red), float64(red)
		}
	}
	return float64(width) / float64(w), float64(height) / float64(h)
}

// axis holds, for each pixel of a composite along one axis, the two pixels
// of a plane around its center and the weight of the second.
type axis struct {
	i0, i1 []int32
	f      []float32
}

// newAxis returns the axis of n pixels of a composite, each of size pixels
// of the page, over pixels of a plane from o, at a distance of s.
func newAxis(n int, size, o, s float64, pn int) axis {
	a := axis{i0: make([]int32, n), i1: make([]int32, n), f: make([]float32, n)}
	for k := range n {
		a.i0[k], a.i1[k], a.f[k] = weight((float64(k)+0.5)*size, o, s, pn)
	}
	return a
}

// weight returns the two pixels of a plane of n pixels from o, at a
// distance of s, around pos, and the weight of the second.
func weight(pos, o, s float64, n int) (int32, int32, float32) {
	g := (pos - o) / s
	switch {
	case !(g > 0):
		return 0, 0, 0
	case g >= float64(n-1):
		return int32(n - 1), int32(n - 1), 0
	}
	i := int32(g)
	return i, i + 1, float32(g - float64(i))
}

// sample returns the color of pl between the rows y0 and y1, with the
// weight fy of y1, and the columns of the pixel u of the axis ax.
func (pl *plane) sample(ax *axis, u int, y0, y1 int32, fy float32) (float32, float32, float32) {
	x0, x1, fx := ax.i0[u], ax.i1[u], ax.f[u]
	row0, row1 := int(y0)*pl.w, int(y1)*pl.w
	n := pl.n
	p00, p01 := pl.pix[n*(row0+int(x0)):], pl.pix[n*(row0+int(x1)):]
	p10, p11 := pl.pix[n*(row1+int(x0)):], pl.pix[n*(row1+int(x1)):]
	mix := func(c int) float32 {
		top := float32(p00[c]) + fx*(float32(p01[c])-float32(p00[c]))
		bottom := float32(p10[c]) + fx*(float32(p11[c])-float32(p10[c]))
		return top + fy*(bottom-top)
	}
	if n == 1 {
		v := mix(0)
		return v, v, v
	}
	return mix(0), mix(1), mix(2)
}

// unit is the weight of a whole pixel of the page in a grid.
const unit = 64

// grid maps the pixels of a page onto the pixels of a composite, along one
// axis. A pixel of the page falls in one pixel of the composite, or across
// the edge of two. cell holds the first of them for each pixel of the page,
// and weight the share of the pixel in it, in units of 1/unit. The rest of
// the pixel falls in the next one. total holds the sum of the shares in each
// pixel of the composite.
type grid struct {
	cell   []int32
	weight []int32
	total  []int32
}

// newGrid returns the grid of n pixels of a page onto m pixels of a
// composite, for m from 1 to n.
func newGrid(n, m int) grid {
	g := grid{cell: make([]int32, n), weight: make([]int32, n), total: make([]int32, m)}
	s := float64(m) / float64(n)
	for i := range n {
		start, end := float64(i)*s, float64(i+1)*s
		c := min(int(start), m-1)
		w := int32(unit)
		if edge := float64(c + 1); end > edge && c+1 < m {
			w = int32(math.Round(unit * (edge - start) / s))
		}
		g.cell[i], g.weight[i] = int32(c), w
		g.total[c] += w
		if w < unit {
			g.total[c+1] += unit - w
		}
	}
	return g
}

// build composites the layers at size: the background, or white, and over
// it the mask in the colors of the foreground, or black. The coverage of a
// pixel of the composite is the share of the pixels of the page under it
// that the mask covers. It also returns the errors of the layers that it
// leaves out.
func (p *Page) build(ctx context.Context, l *layers, size image.Point) (*image.RGBA, error, error) {
	in := p.p.Info
	width, height := in.Width, in.Height
	cw, ch := size.X, size.Y
	out := image.NewRGBA(image.Rect(0, 0, cw, ch))
	cellW, cellH := float64(width)/float64(cw), float64(height)/float64(ch)
	var partial []error

	bg, err := l.bg.plane(width, height, min(cellW, cellH))
	if err != nil {
		partial = append(partial, fmt.Errorf("decoding the background of djvu page %d: %w", p.n+1, format(err)))
		bg = nil
	}
	if bg == nil {
		for i := range out.Pix {
			out.Pix[i] = 0xff
		}
	} else {
		ax := newAxis(cw, cellW, bg.ox, bg.sx, bg.w)
		for v := range ch {
			if v%64 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, nil, fmt.Errorf("rendering djvu page %d: %w", p.n+1, err)
				}
			}
			y0, y1, fy := weight((float64(v)+0.5)*cellH, bg.oy, bg.sy, bg.h)
			row := out.Pix[v*out.Stride:]
			for u := range cw {
				cr, cg, cb := bg.sample(&ax, u, y0, y1, fy)
				row[4*u], row[4*u+1], row[4*u+2], row[4*u+3] = uint8(cr+0.5), uint8(cg+0.5), uint8(cb+0.5), 0xff
			}
		}
	}

	if l.mask != nil {
		fg, err := l.fg.plane(width, height, min(cellW, cellH))
		if err != nil {
			partial = append(partial, fmt.Errorf("decoding the foreground of djvu page %d: %w", p.n+1, format(err)))
			fg = nil
		}
		if err := p.paintMask(ctx, l, fg, out); err != nil {
			return nil, nil, err
		}
	}
	if in.Gamma != 22 {
		correct(out.Pix, float64(in.Gamma)/10)
	}
	return out, errors.Join(partial...), nil
}

// paintMask paints the mask of l onto the composite out, in the colors of
// the palette, of the foreground plane fg, or in black.
func (p *Page) paintMask(ctx context.Context, l *layers, fg *plane, out *image.RGBA) error {
	in := p.p.Info
	width, height := in.Width, in.Height
	cw, ch := out.Rect.Dx(), out.Rect.Dy()
	cols, rows := newGrid(width, cw), newGrid(height, ch)
	m := l.mask
	cov := make([]uint32, cw*ch)
	var colors [][3]byte
	var idx []uint16
	if l.pal != nil && len(l.pal.Colors) > 0 {
		colors = l.pal.Colors
		if len(l.pal.Index) > 0 {
			idx = make([]uint16, cw*ch)
		}
	}
	for i, b := range m.Blits {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("rendering djvu page %d: %w", p.n+1, err)
			}
		}
		s := m.Shape(b.Shape)
		if s.W == 0 || s.H == 0 {
			continue
		}
		var color uint16
		if idx != nil && i < len(l.pal.Index) {
			color = uint16(l.pal.Index[i] + 1)
		}
		// the blits count rows from the bottom of the mask
		top := m.H - b.Y - s.H
		x0, x1 := max(0, -b.X), min(s.W, width-b.X)
		for row := range s.H {
			py := top + row
			if py < 0 || py >= height {
				continue
			}
			src := s.Pix[row*s.W : (row+1)*s.W]
			cy, wy := int(rows.cell[py]), rows.weight[py]
			for c := x0; c < x1; c++ {
				if src[c] == 0 {
					continue
				}
				px := b.X + c
				cx, wx := int(cols.cell[px]), cols.weight[px]
				add := func(k int, w int32) {
					cov[k] += uint32(w)
					if color != 0 {
						idx[k] = color
					}
				}
				k := cy*cw + cx
				add(k, wx*wy)
				if wx < unit {
					add(k+1, (unit-wx)*wy)
				}
				if wy < unit {
					add(k+cw, wx*(unit-wy))
					if wx < unit {
						add(k+cw+1, (unit-wx)*(unit-wy))
					}
				}
			}
		}
	}

	var ax axis
	cellW, cellH := float64(width)/float64(cw), float64(height)/float64(ch)
	if fg != nil && colors == nil {
		ax = newAxis(cw, cellW, fg.ox, fg.sx, fg.w)
	}
	for v := range ch {
		var y0, y1 int32
		var fy float32
		if fg != nil && colors == nil {
			y0, y1, fy = weight((float64(v)+0.5)*cellH, fg.oy, fg.sy, fg.h)
		}
		pix := out.Pix[v*out.Stride:]
		for u := range cw {
			n := int(cov[v*cw+u])
			if n == 0 {
				continue
			}
			area := int(cols.total[u]) * int(rows.total[v])
			if area <= 0 {
				continue
			}
			a := float32(min(n, area)) / float32(area)
			var fr, fgc, fb float32
			switch {
			case colors != nil:
				c := colors[0]
				if idx != nil {
					if k := int(idx[v*cw+u]); k > 0 && k <= len(colors) {
						c = colors[k-1]
					}
				}
				fr, fgc, fb = float32(c[0]), float32(c[1]), float32(c[2])
			case fg != nil:
				fr, fgc, fb = fg.sample(&ax, u, y0, y1, fy)
			}
			q := pix[4*u : 4*u+3 : 4*u+3]
			q[0] = uint8(float32(q[0]) + a*(fr-float32(q[0])) + 0.5)
			q[1] = uint8(float32(q[1]) + a*(fgc-float32(q[1])) + 0.5)
			q[2] = uint8(float32(q[2]) + a*(fb-float32(q[2])) + 0.5)
		}
	}
	return nil
}

// correct changes the colors of pix, made for a display of the given gamma,
// for a display of gamma 2.2, as the gamma of the INFO chunk asks.
func correct(pix []byte, gamma float64) {
	var lut [256]byte
	for i := range lut {
		lut[i] = uint8(math.Round(255 * math.Pow(float64(i)/255, gamma/2.2)))
	}
	for i := 0; i+2 < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2] = lut[pix[i]], lut[pix[i+1]], lut[pix[i+2]]
	}
}

// decodeJPEG2000 decodes a layer in JPEG 2000, a raw codestream or a JP2
// file, with the port of hayro-jpeg2000 (D17). The colors of an ICC profile
// become sRGB.
func decodeJPEG2000(ctx context.Context, data []byte, lim ivy.Limits) (image.Image, error) {
	settings := jpeg2000.DefaultDecodeSettings()
	settings.Limits = lim
	settings.Cancel = jpeg2000.NewCancelHandle(jpeg2000.CancelFunc(func() bool { return ctx.Err() != nil }))
	img, err := jpeg2000.DecodeImage(data, &settings, jpeg2000.MoxcmsTransform)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, fmt.Errorf("decoding jpeg 2000: %w", cerr)
		}
		return nil, fmt.Errorf("decoding jpeg 2000: %w", err)
	}
	return img, nil
}
