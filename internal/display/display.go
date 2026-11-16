// Package display is the display list of D10. A page description, such as
// XPS, or later PDF, emits commands to a Device. The device that draws with
// okage is the main one, and a List records the commands for the tests.
//
// The commands follow the operators of PDF: save and restore the state,
// concatenate a transform, fill and stroke a path, clip, draw an image, fill
// a run of glyphs, and begin and end a group with an opacity. A path, a clip
// and a shader are in the coordinates of the user space at the call: the
// device maps them with the current transform.
package display

import (
	"fmt"
	"image"
	"strings"

	"github.com/xo/ivy/internal/font"
	"github.com/xo/okage"
)

// Device receives the commands of a page.
type Device interface {
	// Save pushes the transform and the clip.
	Save()
	// Restore pops the transform and the clip that the last Save pushed.
	Restore()
	// Concat multiplies the current transform by t, so that t applies
	// first.
	Concat(t okage.Transform)
	// FillPath fills p with the shader.
	FillPath(p *okage.Path, rule okage.FillRule, shader okage.Shader)
	// StrokePath strokes p with the shader.
	StrokePath(p *okage.Path, stroke okage.Stroke, shader okage.Shader)
	// ClipPath intersects the clip with p.
	ClipPath(p *okage.Path, rule okage.FillRule)
	// DrawImage draws img into the rectangle dst, with the opacity.
	DrawImage(img image.Image, dst okage.Rect, opacity float32)
	// FillGlyphs fills the outlines of a run of glyphs with the shader.
	FillGlyphs(run GlyphRun, shader okage.Shader)
	// BeginGroup starts a group, which draws to a layer of its own.
	BeginGroup(opacity float32)
	// EndGroup draws the layer of the group with its opacity.
	EndGroup()
}

// GlyphRun is a run of glyphs of one font and one size.
type GlyphRun struct {
	Face *font.Face
	// Size is the size of the font, an em, in the units of the user space.
	Size float32
	// Glyphs are the glyphs, with the position of each origin.
	Glyphs []Glyph
	// Transform is applied to each glyph, at the scale of the size, before
	// its position, such as a skew for a slanted face.
	Transform okage.Transform
}

// Glyph is one glyph of a run.
type Glyph struct {
	ID   font.GID
	X, Y float32
}

// Path returns the outlines of the run as one path, or nil if no glyph has an
// outline.
func (run GlyphRun) Path() *okage.Path {
	pb := okage.NewPathBuilder()
	glyph := run.Transform.PreScale(run.Size, run.Size)
	for _, g := range run.Glyphs {
		outline := run.Face.Outline(g.ID)
		if outline == nil {
			continue
		}
		t := glyph.PostConcat(okage.TransformFromTranslate(g.X, g.Y))
		// Transform changes its path in place, and the outline is shared, so
		// the run transforms a copy
		if p, ok := Clone(outline).Transform(t); ok {
			pb.PushPath(p)
		}
	}
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// Clone returns a copy of p. A path of okage changes in place when it
// transforms, so a shared path is cloned first.
func Clone(p *okage.Path) *okage.Path {
	pb := okage.NewPathBuilder()
	pb.PushPath(p)
	c, ok := pb.Finish()
	if !ok {
		return p
	}
	return c
}

// List records the commands that it receives, and replays them onto another
// device. Its String lists the commands, one on each line, for the tests.
type List struct {
	cmds []func(Device)
	ops  []string
}

// The interfaces that a List implements.
var _ Device = (*List)(nil)

// record keeps one command.
func (l *List) record(op string, cmd func(Device)) {
	l.ops = append(l.ops, op)
	l.cmds = append(l.cmds, cmd)
}

// Replay sends the commands to d, in order.
func (l *List) Replay(d Device) {
	for _, cmd := range l.cmds {
		cmd(d)
	}
}

// String lists the commands, one on each line.
func (l *List) String() string {
	return strings.Join(l.ops, "\n")
}

// Save satisfies the Device interface.
func (l *List) Save() {
	l.record("save", func(d Device) { d.Save() })
}

// Restore satisfies the Device interface.
func (l *List) Restore() {
	l.record("restore", func(d Device) { d.Restore() })
}

// Concat satisfies the Device interface.
func (l *List) Concat(t okage.Transform) {
	l.record(fmt.Sprintf("concat %v", t), func(d Device) { d.Concat(t) })
}

// FillPath satisfies the Device interface.
func (l *List) FillPath(p *okage.Path, rule okage.FillRule, shader okage.Shader) {
	l.record(fmt.Sprintf("fill %v %v", bounds(p), rule), func(d Device) { d.FillPath(p, rule, shader) })
}

// StrokePath satisfies the Device interface.
func (l *List) StrokePath(p *okage.Path, stroke okage.Stroke, shader okage.Shader) {
	l.record(fmt.Sprintf("stroke %v %g", bounds(p), stroke.Width), func(d Device) { d.StrokePath(p, stroke, shader) })
}

// ClipPath satisfies the Device interface.
func (l *List) ClipPath(p *okage.Path, rule okage.FillRule) {
	l.record(fmt.Sprintf("clip %v %v", bounds(p), rule), func(d Device) { d.ClipPath(p, rule) })
}

// DrawImage satisfies the Device interface.
func (l *List) DrawImage(img image.Image, dst okage.Rect, opacity float32) {
	l.record(fmt.Sprintf("image %v %v %g", img.Bounds().Size(), dst, opacity),
		func(d Device) { d.DrawImage(img, dst, opacity) })
}

// FillGlyphs satisfies the Device interface.
func (l *List) FillGlyphs(run GlyphRun, shader okage.Shader) {
	l.record(fmt.Sprintf("glyphs %d %g", len(run.Glyphs), run.Size), func(d Device) { d.FillGlyphs(run, shader) })
}

// BeginGroup satisfies the Device interface.
func (l *List) BeginGroup(opacity float32) {
	l.record(fmt.Sprintf("group %g", opacity), func(d Device) { d.BeginGroup(opacity) })
}

// EndGroup satisfies the Device interface.
func (l *List) EndGroup() {
	l.record("end group", func(d Device) { d.EndGroup() })
}

// bounds returns the bounds of p, rounded, for the list of commands.
func bounds(p *okage.Path) string {
	if p == nil {
		return "empty"
	}
	b := p.Bounds()
	return fmt.Sprintf("[%.0f %.0f %.0f %.0f]", b.Left(), b.Top(), b.Right(), b.Bottom())
}
