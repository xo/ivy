package graphics

import (
	"image"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// Recorder records the commands of a page, and replays them onto another
// device. It counts its commands, and past the limit it drops them and
// keeps the error.
type Recorder struct {
	cmds []func(display.Device, float64)
	max  int
	// depth is the depth of Save that the commands reached.
	depth int
	err   error
}

// The interfaces that a Recorder implements.
var (
	_ display.Device         = (*Recorder)(nil)
	_ display.TextFiller     = (*Recorder)(nil)
	_ display.StrokeAdjuster = (*Recorder)(nil)
	_ display.Linker         = (*Recorder)(nil)
	_ display.Bookmarker     = (*Recorder)(nil)
)

// newRecorder returns an empty recorder with the limits of lim.
func newRecorder(lim ivy.Limits) *Recorder {
	return &Recorder{max: lim.MaxObjects}
}

// Err returns the error of a limit that the recorder reached, or nil.
func (r *Recorder) Err() error {
	return r.err
}

// Len returns the number of the commands.
func (r *Recorder) Len() int {
	return len(r.cmds)
}

// record keeps one command. It returns false past the limit.
func (r *Recorder) record(cmd func(display.Device)) bool {
	return r.recordScaled(func(d display.Device, _ float64) { cmd(d) })
}

// recordScaled keeps one command that takes the number of the pixels of
// the target for each unit of the device space.
func (r *Recorder) recordScaled(cmd func(display.Device, float64)) bool {
	if len(r.cmds) >= r.max {
		if r.err == nil {
			r.err = limitError("commands of a page", int64(r.max))
		}
		return false
	}
	r.cmds = append(r.cmds, cmd)
	return true
}

// Replay sends the commands to d, in order, and restores the states that
// they left saved. pixels is the number of the pixels of d for each unit of
// the device space, which sets the width of a thin line.
func (r *Recorder) Replay(d display.Device, pixels float64) {
	for _, cmd := range r.cmds {
		cmd(d, pixels)
	}
	for range max(r.depth, 0) {
		d.Restore()
	}
}

// Save satisfies the display.Device interface.
func (r *Recorder) Save() {
	if r.record(func(d display.Device) { d.Save() }) {
		r.depth++
	}
}

// Restore satisfies the display.Device interface.
func (r *Recorder) Restore() {
	if r.depth > 0 && r.record(func(d display.Device) { d.Restore() }) {
		r.depth--
	}
}

// Concat satisfies the display.Device interface.
func (r *Recorder) Concat(t okage.Transform) {
	r.record(func(d display.Device) { d.Concat(t) })
}

// FillPath satisfies the display.Device interface.
func (r *Recorder) FillPath(p *okage.Path, rule okage.FillRule, shader okage.Shader) {
	r.record(func(d display.Device) { d.FillPath(p, rule, shader) })
}

// StrokePath satisfies the display.Device interface.
func (r *Recorder) StrokePath(p *okage.Path, stroke okage.Stroke, shader okage.Shader) {
	r.record(func(d display.Device) { d.StrokePath(p, stroke, shader) })
}

// minLinePixels is the width in pixels of the target of the thinnest line
// that is not a hairline. With it, a thin line has about the weight that
// Ghostscript gives it with anti-aliasing: 0.7 gave the closest pages of
// tiger.eps, golfer.eps and escher.ps.
const minLinePixels = 0.7

// strokeMin records a stroke whose width is at least minLinePixels of the
// target. scale is the number of the units of the device space for each
// unit of the space of the stroke. When adjust is true, the stroke has the
// stroke adjustment of strokeadjust (PLRM 6.5.2).
func (r *Recorder) strokeMin(p *okage.Path, stroke okage.Stroke, adjust bool, shader okage.Shader, scale float64) {
	r.recordScaled(func(d display.Device, pixels float64) {
		st := stroke
		if px := float64(st.Width) * scale * pixels; st.Width > 0 && px < minLinePixels && scale*pixels > 0 {
			st.Width = float32(minLinePixels / (scale * pixels))
		}
		if adjust {
			display.StrokeAdjusted(d, p, st, shader)
			return
		}
		d.StrokePath(p, st, shader)
	})
}

// StrokeAdjusted satisfies the display.StrokeAdjuster interface.
func (r *Recorder) StrokeAdjusted(p *okage.Path, stroke okage.Stroke, shader okage.Shader) {
	r.record(func(d display.Device) { display.StrokeAdjusted(d, p, stroke, shader) })
}

// ClipPath satisfies the display.Device interface.
func (r *Recorder) ClipPath(p *okage.Path, rule okage.FillRule) {
	r.record(func(d display.Device) { d.ClipPath(p, rule) })
}

// DrawImage satisfies the display.Device interface.
func (r *Recorder) DrawImage(img image.Image, t okage.Transform, opts display.ImageOptions) {
	r.record(func(d display.Device) { d.DrawImage(img, t, opts) })
}

// FillMask satisfies the display.Device interface.
func (r *Recorder) FillMask(mask *image.Alpha, t okage.Transform, opts display.ImageOptions, shader okage.Shader) {
	r.record(func(d display.Device) { d.FillMask(mask, t, opts, shader) })
}

// FillGlyphs satisfies the display.Device interface.
func (r *Recorder) FillGlyphs(run display.GlyphRun, shader okage.Shader) {
	r.record(func(d display.Device) { d.FillGlyphs(run, shader) })
}

// FillText satisfies the display.TextFiller interface.
func (r *Recorder) FillText(p *okage.Path, rule okage.FillRule, shader okage.Shader) {
	r.record(func(d display.Device) { display.FillText(d, p, rule, shader) })
}

// BeginGroup satisfies the display.Device interface.
func (r *Recorder) BeginGroup(g display.Group) {
	r.record(func(d display.Device) { d.BeginGroup(g) })
}

// EndGroup satisfies the display.Device interface.
func (r *Recorder) EndGroup() {
	r.record(func(d display.Device) { d.EndGroup() })
}

// LinkURI satisfies the display.Linker interface.
func (r *Recorder) LinkURI(rect okage.Rect, uri string) {
	r.record(func(d display.Device) { display.LinkURI(d, rect, uri) })
}

// LinkAnchor satisfies the display.Linker interface.
func (r *Recorder) LinkAnchor(rect okage.Rect, name string) {
	r.record(func(d display.Device) { display.LinkAnchor(d, rect, name) })
}

// Anchor satisfies the display.Linker interface.
func (r *Recorder) Anchor(name string, x, y float32) {
	r.record(func(d display.Device) { display.Anchor(d, name, x, y) })
}

// Bookmark satisfies the display.Bookmarker interface.
func (r *Recorder) Bookmark(level int, label string, x, y float32, open bool) {
	r.record(func(d display.Device) { display.Bookmark(d, level, label, x, y, open) })
}
