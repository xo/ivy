package display

import (
	"image"
	"sync"

	"github.com/xo/ivy/internal/raster"
	"github.com/xo/okage"
)

// The bounds of the caches of an OkageDevice.
const (
	// maxMasks is the number of soft masks that a device keeps. A mask
	// holds one byte for each pixel of the target.
	maxMasks = 16
	// maxLayers is the number of free layers that a device keeps for the
	// next group.
	maxLayers = 4
)

// OkageDevice draws the commands onto a pixmap with okage. It is not safe for
// concurrent use: a page draws with a device of its own.
type OkageDevice struct {
	// dst is the pixmap that the commands draw onto: the target, or the layer
	// of the innermost group.
	dst    *okage.Pixmap
	state  state
	stack  []state
	groups []group

	images sync.Map // image.Image -> *okage.Pixmap
	masks  map[maskKey]*okage.Mask
	free   []*okage.Pixmap
	// aliased draws the paths and the clips without antialiasing.
	aliased bool
}

// state is the part of the device that Save pushes.
type state struct {
	t    okage.Transform
	clip *okage.Mask
}

// group is a group that BeginGroup started.
type group struct {
	parent *okage.Pixmap
	g      Group
	mask   *okage.Mask
	depth  int
}

// maskKey identifies a soft mask that a device drew.
type maskKey struct {
	key any
	t   okage.Transform
}

// The interfaces that an OkageDevice implements.
var _ Device = (*OkageDevice)(nil)

// NewOkage returns a device that draws onto dst, with the transform t from
// the user space to the pixels of dst.
func NewOkage(dst *okage.Pixmap, t okage.Transform) *OkageDevice {
	return &OkageDevice{dst: dst, state: state{t: t}}
}

// Save satisfies the Device interface.
func (d *OkageDevice) Save() {
	d.stack = append(d.stack, d.state)
}

// Restore satisfies the Device interface. A Restore without a Save does
// nothing.
func (d *OkageDevice) Restore() {
	if len(d.stack) == 0 {
		return
	}
	d.state = d.stack[len(d.stack)-1]
	d.stack = d.stack[:len(d.stack)-1]
}

// Concat satisfies the Device interface.
func (d *OkageDevice) Concat(t okage.Transform) {
	d.state.t = d.state.t.PreConcat(t)
}

// SetAntiAlias turns the antialiasing of the paths, the glyphs and the clips
// on or off. It is on by default. Without it, a pixel is covered or not, as
// in Ghostscript with -dGraphicsAlphaBits=1, which the tests of the layout
// compare with.
func (d *OkageDevice) SetAntiAlias(on bool) {
	d.aliased = !on
}

// paint returns a paint of okage with the shader.
func (d *OkageDevice) paint(shader okage.Shader) okage.Paint {
	p := okage.DefaultPaint()
	p.Shader = shader
	p.AntiAlias = !d.aliased
	return p
}

// FillPath satisfies the Device interface.
func (d *OkageDevice) FillPath(p *okage.Path, rule okage.FillRule, shader okage.Shader) {
	if p == nil || shader == nil {
		return
	}
	pt := d.paint(shader)
	d.dst.FillPath(p, &pt, rule, d.state.t, d.state.clip)
}

// StrokePath satisfies the Device interface.
func (d *OkageDevice) StrokePath(p *okage.Path, stroke okage.Stroke, shader okage.Shader) {
	if p == nil || shader == nil {
		return
	}
	pt := d.paint(shader)
	d.dst.StrokePath(p, &pt, &stroke, d.state.t, d.state.clip)
}

// ClipPath satisfies the Device interface. An empty path clips everything.
func (d *OkageDevice) ClipPath(p *okage.Path, rule okage.FillRule) {
	clip, ok := okage.NewMask(d.dst.Width(), d.dst.Height())
	if !ok {
		return
	}
	if p != nil {
		if d.state.clip == nil {
			clip.FillPath(p, rule, !d.aliased, d.state.t)
		} else {
			clip = d.state.clip.Clone()
			clip.IntersectPath(p, rule, !d.aliased, d.state.t)
		}
	}
	d.state.clip = clip
}

// pixmap returns the pixmap of img. The device keeps the pixmap of each
// image that needs a conversion, for an image that a page draws more than
// once. An *image.RGBA from the origin needs none.
func (d *OkageDevice) pixmap(img image.Image) *okage.Pixmap {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Rect.Min == (image.Point{}) && rgba.Stride == 4*rgba.Rect.Dx() {
		pm, err := raster.Pixmap(rgba)
		if err != nil {
			return nil
		}
		return pm
	}
	pix, ok := d.images.Load(img)
	if !ok {
		p, err := raster.Pixmap(img)
		if err != nil {
			return nil
		}
		pix, _ = d.images.LoadOrStore(img, p)
	}
	pm, _ := pix.(*okage.Pixmap)
	return pm
}

// quality returns the filter of okage for the options.
func quality(opts ImageOptions) okage.FilterQuality {
	if opts.Smooth {
		return okage.FilterQualityBilinear
	}
	return okage.FilterQualityNearest
}

// DrawImage satisfies the Device interface.
func (d *OkageDevice) DrawImage(img image.Image, t okage.Transform, opts ImageOptions) {
	if img == nil {
		return
	}
	pm := d.pixmap(img)
	if pm == nil {
		return
	}
	pp := okage.DefaultPixmapPaint()
	pp.Opacity = opts.Opacity
	pp.Quality = quality(opts)
	d.dst.DrawPixmap(0, 0, pm.AsRef(), &pp, d.state.t.PreConcat(t), d.state.clip)
}

// FillMask satisfies the Device interface. It colors the stencil directly
// for a solid color. For another shader, it draws the stencil into a mask of
// the size of the target, and fills the target through it.
func (d *OkageDevice) FillMask(mask *image.Alpha, t okage.Transform, opts ImageOptions, shader okage.Shader) {
	if mask == nil || shader == nil || mask.Rect.Empty() {
		return
	}
	pp := okage.DefaultPixmapPaint()
	pp.Opacity = opts.Opacity
	pp.Quality = quality(opts)
	tt := d.state.t.PreConcat(t)
	if c, ok := shader.(okage.SolidColor); ok {
		pm := stencil(mask, okage.Color(c))
		if pm == nil {
			return
		}
		d.dst.DrawPixmap(0, 0, pm.AsRef(), &pp, tt, d.state.clip)
		return
	}
	pm := stencil(mask, okage.ColorBlack)
	layer, ok := okage.NewPixmap(d.dst.Width(), d.dst.Height())
	if pm == nil || !ok {
		return
	}
	layer.DrawPixmap(0, 0, pm.AsRef(), &pp, tt, nil)
	cover := intersect(okage.MaskFromPixmap(layer.AsRef(), okage.MaskTypeAlpha), d.state.clip)
	r, ok := okage.RectFromXYWH(0, 0, float32(d.dst.Width()), float32(d.dst.Height()))
	if !ok {
		return
	}
	pt := d.paint(shader.Transform(d.state.t))
	d.dst.FillRect(r, &pt, okage.IdentityTransform(), cover)
}

// stencil returns a pixmap of the color c, with the alpha of each pixel of
// mask as its coverage.
func stencil(mask *image.Alpha, c okage.Color) *okage.Pixmap {
	b := mask.Rect
	pm, ok := okage.NewPixmap(uint32(b.Dx()), uint32(b.Dy()))
	if !ok {
		return nil
	}
	pc := c.Premultiply().ToColorU8()
	r, g, bl, a := uint32(pc.Red()), uint32(pc.Green()), uint32(pc.Blue()), uint32(pc.Alpha())
	data := pm.DataMut()
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := mask.Pix[mask.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			m := uint32(row[x])
			data[i] = uint8((r*m + 127) / 255)
			data[i+1] = uint8((g*m + 127) / 255)
			data[i+2] = uint8((bl*m + 127) / 255)
			data[i+3] = uint8((a*m + 127) / 255)
			i += 4
		}
	}
	return pm
}

// intersect returns the product of the masks a and b. A nil mask covers
// everything. It changes a in place when b is not nil.
func intersect(a, b *okage.Mask) *okage.Mask {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	ad, bd := a.DataMut(), b.Data()
	for i := range min(len(ad), len(bd)) {
		ad[i] = uint8((uint32(ad[i])*uint32(bd[i]) + 127) / 255)
	}
	return a
}

// FillGlyphs satisfies the Device interface.
func (d *OkageDevice) FillGlyphs(run GlyphRun, shader okage.Shader) {
	d.FillPath(run.Path(), okage.FillRuleWinding, shader)
}

// layer returns a transparent pixmap of the size of the target, or nil.
func (d *OkageDevice) layer() *okage.Pixmap {
	if n := len(d.free); n > 0 {
		layer := d.free[n-1]
		d.free = d.free[:n-1]
		clear(layer.DataMut())
		return layer
	}
	layer, ok := okage.NewPixmap(d.dst.Width(), d.dst.Height())
	if !ok {
		return nil
	}
	return layer
}

// BeginGroup satisfies the Device interface. A group that is not isolated,
// with an opacity of 1 or more, the normal blend mode and no mask, draws
// straight onto its parent. Each other group draws to a layer of its own,
// which starts transparent: the device draws every group as isolated and
// as not knockout.
func (d *OkageDevice) BeginGroup(g Group) {
	gr := group{parent: d.dst, g: g, depth: len(d.stack)}
	if g.Mask != nil {
		gr.mask = d.softMask(g.Mask)
	}
	layer := d.dst
	if g.Isolated || g.Opacity < 1 || blendMode(g.Blend) != okage.BlendModeSourceOver || g.Mask != nil {
		if l := d.layer(); l != nil {
			layer = l
		}
	}
	d.groups = append(d.groups, gr)
	d.Save()
	d.dst = layer
}

// softMask draws the content of m into a mask of the size of the target,
// or returns the mask that it drew for the same key and transform.
func (d *OkageDevice) softMask(m *Mask) *okage.Mask {
	key := maskKey{key: m.Key, t: d.state.t}
	if m.Key != nil {
		if mask, ok := d.masks[key]; ok {
			return mask
		}
	}
	pix, ok := okage.NewPixmap(d.dst.Width(), d.dst.Height())
	if !ok {
		return nil
	}
	child := NewOkage(pix, d.state.t)
	child.aliased = d.aliased
	backdrop := m.Backdrop.Alpha() > 0 && m.Backdrop != okage.ColorBlack
	if backdrop {
		pix.Fill(m.Backdrop)
		child.BeginGroup(Group{Opacity: 1, Isolated: true})
	}
	if m.Content != nil {
		m.Content(child)
	}
	if backdrop {
		child.EndGroup()
	}
	mask := okage.MaskFromPixmap(pix.AsRef(), m.Type)
	if m.Transfer != nil {
		m.Transfer(mask.DataMut())
	}
	if m.Key != nil {
		if d.masks == nil || len(d.masks) >= maxMasks {
			d.masks = make(map[maskKey]*okage.Mask)
		}
		d.masks[key] = mask
	}
	return mask
}

// EndGroup satisfies the Device interface. It restores the state of its
// BeginGroup, and draws the layer onto the parent with the opacity, the
// blend mode and the soft mask of the group, inside the clip of the parent.
func (d *OkageDevice) EndGroup() {
	if len(d.groups) == 0 {
		return
	}
	g := d.groups[len(d.groups)-1]
	d.groups = d.groups[:len(d.groups)-1]
	for len(d.stack) > g.depth+1 {
		d.Restore()
	}
	d.Restore()
	layer := d.dst
	d.dst = g.parent
	if layer == g.parent {
		return
	}
	pp := okage.DefaultPixmapPaint()
	pp.Opacity = g.g.Opacity
	pp.BlendMode = blendMode(g.g.Blend)
	mask := d.state.clip
	if g.mask != nil {
		mask = intersect(g.mask.Clone(), d.state.clip)
	}
	d.dst.DrawPixmap(0, 0, layer.AsRef(), &pp, okage.IdentityTransform(), mask)
	if len(d.free) < maxLayers {
		d.free = append(d.free, layer)
	}
}
