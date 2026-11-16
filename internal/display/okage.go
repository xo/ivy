package display

import (
	"image"
	"sync"

	"github.com/xo/ivy/internal/raster"
	"github.com/xo/okage"
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
}

// state is the part of the device that Save pushes.
type state struct {
	t    okage.Transform
	clip *okage.Mask
}

// group is a group that BeginGroup started.
type group struct {
	parent  *okage.Pixmap
	opacity float32
	depth   int
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

// paint returns a paint of okage with the shader.
func paint(shader okage.Shader) okage.Paint {
	p := okage.DefaultPaint()
	p.Shader = shader
	p.AntiAlias = true
	return p
}

// FillPath satisfies the Device interface.
func (d *OkageDevice) FillPath(p *okage.Path, rule okage.FillRule, shader okage.Shader) {
	if p == nil || shader == nil {
		return
	}
	pt := paint(shader)
	d.dst.FillPath(p, &pt, rule, d.state.t, d.state.clip)
}

// StrokePath satisfies the Device interface.
func (d *OkageDevice) StrokePath(p *okage.Path, stroke okage.Stroke, shader okage.Shader) {
	if p == nil || shader == nil {
		return
	}
	pt := paint(shader)
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
			clip.FillPath(p, rule, true, d.state.t)
		} else {
			clip = d.state.clip.Clone()
			clip.IntersectPath(p, rule, true, d.state.t)
		}
	}
	d.state.clip = clip
}

// DrawImage satisfies the Device interface. The device keeps the pixmap of
// each image that it draws, for an image that a page draws more than once.
func (d *OkageDevice) DrawImage(img image.Image, dst okage.Rect, opacity float32) {
	pix, ok := d.images.Load(img)
	if !ok {
		p, err := raster.Pixmap(img)
		if err != nil {
			return
		}
		pix, _ = d.images.LoadOrStore(img, p)
	}
	pm, _ := pix.(*okage.Pixmap)
	b := img.Bounds()
	t := d.state.t.
		PreConcat(okage.TransformFromTranslate(dst.X(), dst.Y())).
		PreScale(dst.Width()/float32(b.Dx()), dst.Height()/float32(b.Dy()))
	pp := okage.DefaultPixmapPaint()
	pp.Opacity = opacity
	pp.Quality = okage.FilterQualityBilinear
	d.dst.DrawPixmap(0, 0, pm.AsRef(), &pp, t, d.state.clip)
}

// FillGlyphs satisfies the Device interface.
func (d *OkageDevice) FillGlyphs(run GlyphRun, shader okage.Shader) {
	d.FillPath(run.Path(), okage.FillRuleWinding, shader)
}

// BeginGroup satisfies the Device interface. A group with an opacity of 1 or
// more draws straight onto the target.
func (d *OkageDevice) BeginGroup(opacity float32) {
	layer, ok := okage.NewPixmap(d.dst.Width(), d.dst.Height())
	if !ok {
		layer = d.dst
	}
	d.groups = append(d.groups, group{parent: d.dst, opacity: opacity, depth: len(d.stack)})
	d.Save()
	d.dst = layer
}

// EndGroup satisfies the Device interface. It restores the state of its
// BeginGroup, and draws the layer onto the parent with the opacity of the
// group, inside the clip of the parent.
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
	pp.Opacity = g.opacity
	d.dst.DrawPixmap(0, 0, layer.AsRef(), &pp, okage.IdentityTransform(), d.state.clip)
}
