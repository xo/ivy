// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports part of weasyprint/pdf/stream.py: the class Stream that the
// drawing writes to, with the operators of pydyf that the drawing calls. The
// stream records the commands of the display list of D10 instead of the
// operators of PDF, and keeps the graphics state of PDF that the commands
// need: the transform, the colors, the alphas, the line and the soft masks.

package layout

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"
	"github.com/xo/okage/tinycss2"
	gofont "github.com/xo/okage/typesetting/font"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	ivyfont "github.com/xo/ivy/internal/font"
)

// The bounds of the drawing.
const (
	// maxGroupDepth is the depth of the groups that the drawing nests.
	// Each group draws to a layer of the size of the page.
	//
	// Port: upstream nests the groups of PDF without a bound. Deeper groups
	// draw onto their parent, without their opacity and their mask.
	maxGroupDepth = 32
	// maxCellPixels is the number of pixels of a cell of a tiling pattern
	// and of an SVG image that the drawing renders.
	//
	// Port: upstream writes the patterns and the SVG images as vectors.
	maxCellPixels = 1 << 24
	// maxDrawWork is the number of commands that the drawing of a page
	// records, as a multiple of Limits.MaxObjects.
	maxDrawWork = 64
)

// drawContext holds what the streams of a page share: the cancellation, the
// options of the drawing and the faces of the glyphs.
type drawContext struct {
	// ctx stops the drawing. It is the context of one call to DrawPage.
	ctx context.Context
	// pageRectangle is page_rectangle of upstream: the area of the page,
	// with its bleed, in the user space of the page.
	pageRectangle [4]float64
	// deviceArea is the box of the page rectangle on the device: (x, y,
	// width, height) in pixels.
	deviceArea [4]float64
	// antiAlias and interpolate are the options of the drawing.
	antiAlias, interpolate bool
	log                    *logger
	limits                 ivy.Limits
	// fonts are the fonts of the document, for the text of the SVG images.
	fonts *svg.FontDatabase
	// faces holds the fonts of the glyphs, by face of go-text.
	faces map[*gofont.Face]*ivyfont.Face
	// work counts the commands, and maxWork bounds them.
	work, maxWork int
	// err is the first error of the drawing. The drawing goes on after an
	// error that is not the error of ctx, and the page is drawn in part.
	err error
	// halted is true when ctx is done or the drawing made too many
	// commands.
	halted bool
	// depth is the depth of the groups at the replay, and opened tells for
	// each group of a soft mask whether it started a group of the device.
	depth  int
	opened []bool
}

// stopped reports whether the drawing must stop: ctx is done or the drawing
// made too many commands. It checks ctx once in 256 calls.
func (dc *drawContext) stopped() bool {
	if dc.halted {
		return true
	}
	dc.work++
	if dc.work > dc.maxWork {
		dc.fail(fmt.Errorf("drawing more than %d commands: %w", dc.maxWork, ivy.ErrLimit))
		dc.halted = true
		return true
	}
	if dc.work%256 == 0 {
		if err := dc.ctx.Err(); err != nil {
			dc.err = err
			dc.halted = true
			return true
		}
	}
	return false
}

// fail keeps the first error of the drawing.
func (dc *drawContext) fail(err error) {
	if dc.err == nil {
		dc.err = err
	}
}

// face returns the font of the glyphs of a face of go-text.
func (dc *drawContext) face(f *gofont.Face) *ivyfont.Face {
	if f == nil {
		return nil
	}
	if face, ok := dc.faces[f]; ok {
		return face
	}
	face := ivyfont.FromFace(f)
	dc.faces[f] = face
	return face
}

// paintColor is the color of the filling or of the stroking: a color of
// sRGB or a tiling pattern, and the alpha of the graphics state.
type paintColor struct {
	r, g, b float64
	alpha   float64
	pattern *stream
}

// graphicsState is the part of the graphics state of PDF that the drawing
// uses.
type graphicsState struct {
	// ctm is the transform from the user space to the user space of the
	// stream at its creation.
	ctm          Matrix
	fill, stroke paintColor
	lineWidth    float64
	lineCap      okage.LineCap
	dash         []float64
	dashOffset   float64
	// groups is the number of groups of the soft masks that the state
	// started, which its restore ends.
	groups int
}

// defaultState is the graphics state of PDF at the start of a stream.
func defaultState() graphicsState {
	return graphicsState{
		ctm:       identity,
		fill:      paintColor{alpha: 1},
		stroke:    paintColor{alpha: 1},
		lineWidth: 1,
		lineCap:   okage.LineCapButt,
	}
}

// pattern holds what a tiling pattern adds to a stream.
type pattern struct {
	// bbox is the box of the cell, and repeatWidth and repeatHeight are the
	// steps of the pattern.
	bbox                      [4]float64
	repeatWidth, repeatHeight float64
	// matrix maps the space of the pattern to the space of the stream that
	// made it.
	matrix Matrix
	// width and height are the size of the pixmap of a step.
	width, height int
	// cell is the pixmap of a step, drawn at the first fill.
	cell *okage.Pixmap
}

// stream is the class Stream of weasyprint/pdf/stream.py. It records the
// commands of a page, of a group, of a soft mask or of a pattern, and
// replays them onto a device.
type stream struct {
	dc *drawContext
	// ops are the recorded commands.
	ops []func(d display.Device)
	// base maps the user space of the stream at its creation to the pixels
	// of the device. A group and a mask get the transform of their parent
	// at their creation.
	base Matrix
	// bbox is the BBox of a group or of a soft mask, which clips its
	// content, or nil.
	bbox *[4]float64
	// pattern is set for the stream of a tiling pattern.
	pattern *pattern
	// area is the area of the device that the stream can draw to: (x, y,
	// width, height) in pixels. The stream of a page, of a group and of a
	// soft mask can draw to the page, and the stream of a pattern to its
	// cell.
	area [4]float64

	state graphicsState
	stack []graphicsState
	path  *okage.PathBuilder
	// clipRule is the rule of the clip that the next painting of the path
	// applies, or nil.
	clipRule *okage.FillRule
}

// newStream returns the stream of a page, whose user space maps to the
// pixels of the device by base.
func newStream(dc *drawContext, base Matrix) *stream {
	return &stream{dc: dc, base: base, state: defaultState(), path: okage.NewPathBuilder(), area: dc.deviceArea}
}

// clone is Stream.clone: a new stream that shares the context of s, for a
// group, a soft mask or a pattern. Its user space is the current user space
// of s.
func (s *stream) clone() *stream {
	return newStream(s.dc, s.state.ctm.Mul(s.base))
}

// record keeps a command.
func (s *stream) record(op func(d display.Device)) {
	if s.dc.stopped() {
		return
	}
	s.ops = append(s.ops, op)
}

// replay sends the commands of s to d. It ends the groups of the soft masks
// that the stream started and did not end, as the end of a content stream
// of PDF does.
func (s *stream) replay(d display.Device) {
	dc := s.dc
	opened := len(dc.opened)
	for i, op := range s.ops {
		if i%256 == 255 && dc.ctx.Err() != nil {
			dc.fail(dc.ctx.Err())
			break
		}
		op(d)
	}
	for len(dc.opened) > opened {
		n := len(dc.opened) - 1
		if dc.opened[n] {
			d.EndGroup()
			dc.depth--
		}
		dc.opened = dc.opened[:n]
	}
}

// ctm is the property ctm of Stream: the transform from the current user
// space to the user space of the stream.
func (s *stream) ctm() Matrix {
	return s.state.ctm
}

// pushState is Stream.push_state, the operator q.
func (s *stream) pushState() {
	s.record(func(d display.Device) { d.Save() })
	s.stack = append(s.stack, s.state)
	s.state.groups = 0
}

// popState is Stream.pop_state, the operator Q. It ends the groups of the
// soft masks of the state first.
func (s *stream) popState() {
	if len(s.stack) == 0 {
		return
	}
	dc := s.dc
	for range s.state.groups {
		s.record(func(d display.Device) {
			n := len(dc.opened) - 1
			if n < 0 {
				return
			}
			if dc.opened[n] {
				d.EndGroup()
				dc.depth--
			}
			dc.opened = dc.opened[:n]
		})
	}
	s.record(func(d display.Device) { d.Restore() })
	s.state = s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
}

// stacked is Stream.stacked: it runs f between pushState and popState.
func (s *stream) stacked(f func()) {
	s.pushState()
	defer s.popState()
	f()
}

// transform is Stream.transform, the operator cm.
func (s *stream) transform(a, b, c, d, e, f float64) {
	m := Matrix{A: a, B: b, C: c, D: d, E: e, F: f}
	t := m.transform()
	s.record(func(d display.Device) { d.Concat(t) })
	s.state.ctm = m.Mul(s.state.ctm)
}

// translate is transform(e=e, f=f).
func (s *stream) translate(e, f float64) {
	s.transform(1, 0, 0, 1, e, f)
}

// moveTo is the operator m.
func (s *stream) moveTo(x, y float64) {
	s.path.MoveTo(float32(x), float32(y))
}

// lineTo is the operator l.
func (s *stream) lineTo(x, y float64) {
	s.path.LineTo(float32(x), float32(y))
}

// curveTo is the operator c.
func (s *stream) curveTo(x1, y1, x2, y2, x3, y3 float64) {
	s.path.CubicTo(float32(x1), float32(y1), float32(x2), float32(y2), float32(x3), float32(y3))
}

// close is the operator h.
func (s *stream) close() {
	s.path.Close()
}

// rectangle is the operator re.
func (s *stream) rectangle(x, y, width, height float64) {
	s.moveTo(x, y)
	s.lineTo(x+width, y)
	s.lineTo(x+width, y+height)
	s.lineTo(x, y+height)
	s.close()
}

// takePath returns the current path, or nil for an empty path, and starts a
// new one.
func (s *stream) takePath() *okage.Path {
	p, ok := s.path.Finish()
	s.path = okage.NewPathBuilder()
	if !ok {
		return nil
	}
	return p
}

// paintPath ends the path: it calls paint with the path, then applies the
// clip that clip asked for.
func (s *stream) paintPath(paint func(p *okage.Path)) {
	p := s.takePath()
	if p != nil && paint != nil {
		paint(p)
	}
	if s.clipRule != nil {
		rule := *s.clipRule
		s.clipRule = nil
		s.record(func(d display.Device) { d.ClipPath(p, rule) })
	}
}

// fill is the operator f, or f* for evenOdd.
func (s *stream) fill(evenOdd bool) {
	rule := okage.FillRuleWinding
	if evenOdd {
		rule = okage.FillRuleEvenOdd
	}
	shader := s.shader(s.state.fill)
	s.paintPath(func(p *okage.Path) {
		if shader == nil {
			return
		}
		s.record(func(d display.Device) {
			if sh := shader(); sh != nil {
				d.FillPath(p, rule, sh)
			}
		})
	})
}

// stroke is the operator S.
func (s *stream) stroke() {
	shader := s.shader(s.state.stroke)
	st := okage.DefaultStroke()
	width := s.state.lineWidth
	if !s.dc.antiAlias {
		// Port: without antialiasing, a stroke thinner than a pixel of the
		// device covers no center of a pixel, and okage draws nothing.
		// Ghostscript, which draws the expected pixels of the tests, draws
		// such a stroke one pixel wide.
		if scale := math.Sqrt(math.Abs(s.absolute().Determinant())); scale > 0 && width*scale < 1 {
			width = 1 / scale
		}
	}
	st.Width = float32(width)
	st.LineCap = s.state.lineCap
	if dash := s.state.dash; len(dash) > 0 {
		if len(dash)%2 == 1 {
			// PDF repeats a dash array of an odd length, okage does not.
			dash = append(dash[:len(dash):len(dash)], dash...)
		}
		array := make([]float32, len(dash))
		for i, v := range dash {
			array[i] = float32(v)
		}
		if sd, ok := okage.NewStrokeDash(array, float32(s.state.dashOffset)); ok {
			st.Dash = sd
		}
	}
	s.paintPath(func(p *okage.Path) {
		if shader == nil {
			return
		}
		s.record(func(d display.Device) {
			if sh := shader(); sh != nil {
				d.StrokePath(p, st, sh)
			}
		})
	})
}

// clip is the operator W, or W* for evenOdd. The clip applies at the next
// painting of the path, such as end.
func (s *stream) clip(evenOdd bool) {
	rule := okage.FillRuleWinding
	if evenOdd {
		rule = okage.FillRuleEvenOdd
	}
	s.clipRule = &rule
}

// end is the operator n.
func (s *stream) end() {
	s.paintPath(nil)
}

// srgbOf returns the coordinates of sRGB and the alpha of a color.
func srgbOf(c *tinycss2.Color) (r, g, b, alpha float64) {
	if c == nil {
		return 0, 0, 0, 0
	}
	alpha = c.Alpha
	if c.Space == "device-cmyk" && len(c.Coordinates) == 4 {
		// Port: upstream writes the color in DeviceCMYK, and the reader of
		// the PDF file converts it. The port converts it without a profile.
		cc, m, y, k := c.Coordinates[0], c.Coordinates[1], c.Coordinates[2], c.Coordinates[3]
		return (1 - cc) * (1 - k), (1 - m) * (1 - k), (1 - y) * (1 - k), alpha
	}
	if srgb, ok := c.To("srgb"); ok && len(srgb.Coordinates) >= 3 {
		return clamp01(srgb.Coordinates[0]), clamp01(srgb.Coordinates[1]), clamp01(srgb.Coordinates[2]), alpha
	}
	channels := append([]float64(nil), c.Coordinates...)
	for len(channels) < 3 {
		channels = append(channels, 0)
	}
	return clamp01(channels[0]), clamp01(channels[1]), clamp01(channels[2]), alpha
}

// clamp01 clamps v to [0, 1], with 0 for NaN.
func clamp01(v float64) float64 {
	if !(v > 0) {
		return 0
	}
	return min(v, 1)
}

// setColor is Stream.set_color: it sets the color and the alpha of the
// filling, or of the stroking.
//
// Port: upstream writes the color in its color space, sRGB, Lab or
// DeviceCMYK. The port converts each color to sRGB, and DeviceCMYK without a
// profile.
func (s *stream) setColor(color *tinycss2.Color, stroke bool) {
	r, g, b, alpha := srgbOf(color)
	s.setAlpha(alpha, stroke, !stroke)
	c := &s.state.fill
	if stroke {
		c = &s.state.stroke
	}
	c.r, c.g, c.b, c.pattern = r, g, b, nil
}

// setColorPattern is set_color_space('Pattern') then set_color_special with
// the id of a pattern: the filling paints the pattern.
func (s *stream) setColorPattern(p *stream) {
	s.state.fill.pattern = p
}

// setAlpha is Stream.set_alpha: the alpha of the stroking, of the filling,
// or both.
func (s *stream) setAlpha(alpha float64, stroke, fill bool) {
	if stroke {
		s.state.stroke.alpha = alpha
	}
	if fill {
		s.state.fill.alpha = alpha
	}
}

// setLineWidth is the operator w.
func (s *stream) setLineWidth(width float64) {
	s.state.lineWidth = width
}

// setLineCap is the operator J: 0 for butt, 1 for round, 2 for square.
func (s *stream) setLineCap(lineCap int) {
	switch lineCap {
	case 1:
		s.state.lineCap = okage.LineCapRound
	case 2:
		s.state.lineCap = okage.LineCapSquare
	default:
		s.state.lineCap = okage.LineCapButt
	}
}

// setDash is the operator d.
func (s *stream) setDash(dash []float64, offset float64) {
	s.state.dash = append([]float64(nil), dash...)
	s.state.dashOffset = offset
}

// shader returns a function that gives the shader of a color at the replay,
// or nil for a color that paints nothing.
func (s *stream) shader(c paintColor) func() okage.Shader {
	if c.alpha <= 0 {
		return nil
	}
	if c.pattern != nil {
		p := c.pattern
		toUser, ok := s.state.ctm.Invert()
		if !ok {
			return nil
		}
		alpha := c.alpha
		return func() okage.Shader { return p.patternShader(toUser, alpha) }
	}
	color, ok := okage.ColorFromRGBA(float32(c.r), float32(c.g), float32(c.b), float32(min(c.alpha, 1)))
	if !ok {
		return nil
	}
	shader := okage.SolidColor(color)
	return func() okage.Shader { return shader }
}

// group starts the content of a group or of a soft mask on d: it clips the
// content to the BBox.
func (s *stream) clipToBBox(d display.Device) {
	if s.bbox == nil {
		return
	}
	x, y, w, h := s.bbox[0], s.bbox[1], s.bbox[2], s.bbox[3]
	r, ok := okage.RectFromXYWH(float32(x), float32(y), float32(w), float32(h))
	if !ok {
		d.ClipPath(nil, okage.FillRuleWinding)
		return
	}
	d.ClipPath(okage.PathFromRect(r), okage.FillRuleWinding)
}

// addGroup is Stream.add_group: a new stream for a transparency group, with
// the BBox (x, y, width, height), that drawXObject draws.
func (s *stream) addGroup(x, y, width, height float64) *stream {
	group := s.clone()
	group.bbox = &[4]float64{x, y, width, height}
	return group
}

// drawXObject is the operator Do for a group: it draws the group at the
// alpha of the filling.
//
// Port: a group deeper than maxGroupDepth draws onto its parent, without
// its opacity.
func (s *stream) drawXObject(group *stream) {
	alpha := float32(min(max(s.state.fill.alpha, 0), 1))
	dc := s.dc
	s.record(func(d display.Device) {
		if dc.depth >= maxGroupDepth {
			d.Save()
			group.clipToBBox(d)
			group.replay(d)
			d.Restore()
			return
		}
		dc.depth++
		d.BeginGroup(display.Group{Opacity: alpha, Isolated: true})
		group.clipToBBox(d)
		group.replay(d)
		d.EndGroup()
		dc.depth--
	})
}

// setAlphaState is Stream.set_alpha_state: it sets a soft mask of the mode
// "luminosity" or "alpha" in the graphics state, and returns the stream of
// its content, with the BBox (x, y, width, height). The mask applies to what
// the stream draws until the state is restored.
func (s *stream) setAlphaState(x, y, width, height float64, mode string) *stream {
	alphaStream := s.addGroup(x, y, width, height)
	maskType := okage.MaskTypeLuminance
	if mode == "alpha" {
		maskType = okage.MaskTypeAlpha
	}
	dc := s.dc
	mask := &display.Mask{
		Type: maskType,
		Content: func(d display.Device) {
			d.Save()
			alphaStream.clipToBBox(d)
			alphaStream.replay(d)
			d.Restore()
		},
	}
	s.record(func(d display.Device) {
		// Port: a soft mask deeper than maxGroupDepth is left out.
		if dc.depth >= maxGroupDepth {
			dc.opened = append(dc.opened, false)
			return
		}
		dc.opened = append(dc.opened, true)
		dc.depth++
		d.BeginGroup(display.Group{Opacity: 1, Isolated: true, Mask: mask})
	})
	// The group ends at the restore of the state.
	s.state.groups++
	s.state.fill.alpha = 1
	return alphaStream
}

// addPattern is Stream.add_pattern: a new stream for a tiling pattern whose
// cell is (x, y, width, height) and whose steps are repeatWidth and
// repeatHeight. matrix maps the space of the pattern to the space of s.
func (s *stream) addPattern(x, y, width, height, repeatWidth, repeatHeight float64, matrix Matrix) *stream {
	toDevice := matrix.Mul(s.base)
	sx := math.Hypot(toDevice.A, toDevice.B)
	sy := math.Hypot(toDevice.C, toDevice.D)
	cellWidth, cellHeight, _ := boundCell(repeatWidth*sx, repeatHeight*sy)
	w, h := float64(cellWidth), float64(cellHeight)
	p := &stream{
		dc:    s.dc,
		state: defaultState(),
		path:  okage.NewPathBuilder(),
		pattern: &pattern{
			bbox:        [4]float64{x, y, width, height},
			repeatWidth: repeatWidth, repeatHeight: repeatHeight,
			matrix: matrix,
			width:  int(w), height: int(h),
		},
	}
	if repeatWidth > 0 && repeatHeight > 0 {
		p.base = Matrix{A: w / repeatWidth, D: h / repeatHeight}
	} else {
		p.base = identity
	}
	p.area = [4]float64{0, 0, w, h}
	return p
}

// boundCell returns the size in pixels of an image of width by height
// pixels that the drawing renders, such as the cell of a pattern or an SVG
// image, and the scale of the image from the requested size. The size is at
// least 1 by 1.
//
// Port: the image has at most maxCellPixels pixels. A larger image is
// rendered at a lower resolution.
func boundCell(width, height float64) (w, h int, scale float64) {
	clamp := func(v float64) float64 {
		if !(v >= 1) {
			return 1
		}
		return math.Ceil(min(v, maxCellPixels))
	}
	width, height = clamp(width), clamp(height)
	scale = 1
	if width*height > maxCellPixels {
		scale = math.Sqrt(maxCellPixels / (width * height))
	}
	return max(1, int(math.Floor(width*scale))), max(1, int(math.Floor(height*scale))), scale
}

// patternShader returns the shader of a tiling pattern, for the user space
// that toUser maps the space of the stream to. It draws the cell once.
func (s *stream) patternShader(toUser Matrix, alpha float64) okage.Shader {
	p := s.pattern
	if p == nil || p.repeatWidth <= 0 || p.repeatHeight <= 0 {
		return nil
	}
	if p.cell == nil {
		cell, ok := okage.NewPixmap(uint32(p.width), uint32(p.height))
		if !ok {
			return nil
		}
		dev := display.NewOkage(cell, s.base.transform())
		dev.SetAntiAlias(s.dc.antiAlias)
		x, y, w, h := p.bbox[0], p.bbox[1], p.bbox[2], p.bbox[3]
		if r, ok := okage.RectFromXYWH(float32(x), float32(y), float32(w), float32(h)); ok {
			dev.ClipPath(okage.PathFromRect(r), okage.FillRuleWinding)
		} else {
			dev.ClipPath(nil, okage.FillRuleWinding)
		}
		s.replay(dev)
		p.cell = cell
	}
	t := p.matrix.Mul(toUser).transform().
		PreScale(float32(p.repeatWidth/float64(p.width)), float32(p.repeatHeight/float64(p.height)))
	return okage.NewPattern(p.cell.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityNearest, float32(min(alpha, 1)), t)
}

// pdfFunction is a function of PDF that the gradients use: an exponential
// interpolation function of type 2, or a stitching function of type 3.
type pdfFunction struct {
	functionType int
	domain       [2]float64
	// c0, c1 and n are the values of a function of type 2.
	c0, c1 []float64
	n      float64
	// encode, bounds and functions are the values of a function of type 3.
	encode    []float64
	bounds    []float64
	functions []pdfFunction
}

// createInterpolationFunction is Stream.create_interpolation_function.
func createInterpolationFunction(domain [2]float64, c0, c1 []float64, n float64) pdfFunction {
	return pdfFunction{functionType: 2, domain: domain, c0: c0, c1: c1, n: n}
}

// createStitchingFunction is Stream.create_stitching_function.
func createStitchingFunction(domain [2]float64, encode, bounds []float64, subFunctions []pdfFunction) pdfFunction {
	return pdfFunction{functionType: 3, domain: domain, encode: encode, bounds: bounds, functions: subFunctions}
}

// eval returns the values of the function at t, as section 7.10 of PDF 2.0
// gives them.
func (f pdfFunction) eval(t float64) []float64 {
	t = min(max(t, f.domain[0]), f.domain[1])
	if f.functionType == 2 {
		x := t
		if f.n != 1 {
			x = math.Pow(t, f.n)
		}
		out := make([]float64, len(f.c0))
		for i := range out {
			c1 := 0.0
			if i < len(f.c1) {
				c1 = f.c1[i]
			}
			out[i] = f.c0[i] + x*(c1-f.c0[i])
		}
		return out
	}
	if len(f.functions) == 0 {
		return nil
	}
	k := 0
	for k < len(f.bounds) && t >= f.bounds[k] {
		k++
	}
	k = min(k, len(f.functions)-1)
	low, high := f.domain[0], f.domain[1]
	if k > 0 && k-1 < len(f.bounds) {
		low = f.bounds[k-1]
	}
	if k < len(f.bounds) {
		high = f.bounds[k]
	}
	e0, e1 := 0.0, 1.0
	if 2*k+1 < len(f.encode) {
		e0, e1 = f.encode[2*k], f.encode[2*k+1]
	}
	x := e0
	if high != low {
		x = e0 + (t-low)*(e1-e0)/(high-low)
	}
	return f.functions[k].eval(x)
}

// sample is a value of a function at t.
type sample struct {
	t      float64
	values []float64
}

// samples returns the values of the function where the stops of a gradient
// of okage sample it: at the bounds, and at more points in the segments
// whose interpolation is not linear. Each segment of a stitching function
// gives its own values at its ends, so that two stops at the same bound
// keep a sharp change of color.
func (f pdfFunction) samples() []sample {
	t0, t1 := f.domain[0], f.domain[1]
	// Port: a gradient of more than maxSampledFunctions stops, such as a
	// long repeating gradient, interpolates its stops linearly, so that it
	// has at most 16 times maxSampledFunctions stops.
	const maxSampledFunctions = 1 << 12
	steps := func(g pdfFunction) int {
		if g.functionType == 2 && g.n != 1 && len(f.functions) <= maxSampledFunctions {
			return 16
		}
		return 1
	}
	if f.functionType != 3 || len(f.functions) == 0 {
		n := steps(f)
		out := make([]sample, 0, n+1)
		for j := range n + 1 {
			t := t0 + (t1-t0)*float64(j)/float64(n)
			out = append(out, sample{t, f.eval(t)})
		}
		return out
	}
	var out []sample
	for k, g := range f.functions {
		low, high := t0, t1
		if k > 0 && k-1 < len(f.bounds) {
			low = f.bounds[k-1]
		}
		if k < len(f.bounds) {
			high = f.bounds[k]
		}
		e0, e1 := 0.0, 1.0
		if 2*k+1 < len(f.encode) {
			e0, e1 = f.encode[2*k], f.encode[2*k+1]
		}
		n := steps(g)
		for j := range n + 1 {
			x := float64(j) / float64(n)
			out = append(out, sample{low + (high-low)*x, g.eval(e0 + (e1-e0)*x)})
		}
	}
	return out
}

// shading is a shading of PDF that add_shading returns: an axial shading
// of type 2 or a radial shading of type 3.
type shading struct {
	shadingType int
	domain      [2]float64
	coords      []float64
	extend      bool
	function    pdfFunction
}

// addShading is Stream.add_shading.
func (s *stream) addShading(shadingType int, domain [2]float64, coords []float64, extend bool, function pdfFunction) *shading {
	return &shading{shadingType: shadingType, domain: domain, coords: coords, extend: extend, function: function}
}

// gradientShader returns the shader of okage of a shading, at an alpha.
//
// Port: okage interpolates linearly between the stops of a gradient, so the
// stops sample the functions whose exponent is not 1. A shading without
// extend uses okage.SpreadModeDecal, which paints nothing past the ends of
// the gradient (okage D55). A radial shading without extend whose circles
// do not nest paints its first and its last colors past its domain, because
// okage does not paint a point with the largest circle that holds it, as
// PDF does.
func (sh *shading) gradientShader(alpha float64) okage.Shader {
	t0, t1 := sh.domain[0], sh.domain[1]
	color := func(v []float64) okage.Color {
		var r, g, b float64
		switch len(v) {
		case 0:
		case 1, 2:
			r, g, b = v[0], v[0], v[0]
		default:
			r, g, b = v[0], v[1], v[2]
		}
		c, _ := okage.ColorFromRGBA(float32(clamp01(r)), float32(clamp01(g)), float32(clamp01(b)), float32(clamp01(alpha)))
		return c
	}
	if !(t1 > t0) {
		return okage.SolidColor(color(sh.function.eval(t1)))
	}
	samples := sh.function.samples()
	stops := make([]okage.GradientStop, 0, len(samples))
	for _, sample := range samples {
		stops = append(stops, okage.NewGradientStop(float32((sample.t-t0)/(t1-t0)), color(sample.values)))
	}
	var shader okage.Shader
	var ok bool
	c := sh.coords
	spread := okage.SpreadModePad
	switch {
	case sh.shadingType == 2 && len(c) == 4:
		if !sh.extend {
			spread = okage.SpreadModeDecal
		}
		shader, ok = okage.NewLinearGradient(okage.Point{X: float32(c[0]), Y: float32(c[1])},
			okage.Point{X: float32(c[2]), Y: float32(c[3])}, stops, spread, okage.IdentityTransform())
	case sh.shadingType == 3 && len(c) == 6:
		r0, r1 := max(c[2], 0), max(c[5], 0)
		if !sh.extend && math.Abs(r1-r0) > math.Hypot(c[3]-c[0], c[4]-c[1]) {
			spread = okage.SpreadModeDecal
		}
		shader, ok = okage.NewRadialGradient(okage.Point{X: float32(c[0]), Y: float32(c[1])}, float32(r0),
			okage.Point{X: float32(c[3]), Y: float32(c[4])}, float32(r1), stops, spread, okage.IdentityTransform())
	}
	if !ok {
		return okage.SolidColor(color(sh.function.eval(t1)))
	}
	return shader
}

// paintShading is the operator sh: it paints the shading in the whole clip.
func (s *stream) paintShading(sh *shading) {
	alpha := s.state.fill.alpha
	if alpha <= 0 {
		return
	}
	p := s.paintArea()
	if p == nil {
		return
	}
	s.record(func(d display.Device) {
		d.FillPath(p, okage.FillRuleWinding, sh.gradientShader(alpha))
	})
}

// paintArea returns a rectangle of the current user space that covers the
// area that the stream can paint, or nil.
func (s *stream) paintArea() *okage.Path {
	area := s.dc.pageRectangle
	if s.pattern != nil {
		area = s.pattern.bbox
	}
	x, y, w, h := area[0]-area[2], area[1]-area[3], 3*area[2], 3*area[3]
	inverse, ok := s.state.ctm.Invert()
	if !ok {
		return nil
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]float64{{x, y}, {x + w, y}, {x, y + h}, {x + w, y + h}} {
		px, py := inverse.TransformPoint(corner[0], corner[1])
		minX, minY = min(minX, px), min(minY, py)
		maxX, maxY = max(maxX, px), max(maxY, py)
	}
	r, ok := okage.RectFromLTRB(float32(minX), float32(minY), float32(maxX), float32(maxY))
	if !ok {
		return nil
	}
	return okage.PathFromRect(r)
}

// drawImage is the operator Do for an image: it draws img in the unit
// square of the user space, with its first row at y = 1, as PDF draws an
// image. interpolate smooths the image.
func (s *stream) drawImage(img image.Image, interpolate bool) {
	if img == nil {
		return
	}
	alpha := s.state.fill.alpha
	if alpha <= 0 {
		return
	}
	size := img.Bounds().Size()
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	t := okage.TransformFromRow(1/float32(size.X), 0, 0, -1/float32(size.Y), 0, 1)
	opts := display.ImageOptions{Opacity: float32(min(alpha, 1)), Smooth: interpolate && s.dc.interpolate}
	s.record(func(d display.Device) { d.DrawImage(img, t, opts) })
}

// drawImageMatrix draws img with the transform m from its pixels to the
// current user space. interpolate smooths the image.
func (s *stream) drawImageMatrix(img image.Image, m Matrix, interpolate bool) {
	alpha := s.state.fill.alpha
	if img == nil || alpha <= 0 {
		return
	}
	b := img.Bounds()
	t := translation(-float64(b.Min.X), -float64(b.Min.Y)).Mul(m).transform()
	opts := display.ImageOptions{Opacity: float32(min(alpha, 1)), Smooth: interpolate && s.dc.interpolate}
	s.record(func(d display.Device) { d.DrawImage(img, t, opts) })
}

// fillGlyphs fills a run of glyphs with the color of the filling. It stands
// for the text operators of PDF.
func (s *stream) fillGlyphs(run display.GlyphRun) {
	shader := s.shader(s.state.fill)
	if shader == nil || run.Face == nil || len(run.Glyphs) == 0 {
		return
	}
	s.record(func(d display.Device) {
		if sh := shader(); sh != nil {
			d.FillGlyphs(run, sh)
		}
	})
}

// fillBoldGlyphs fills a run of glyphs, and strokes their outlines with
// the fill and a line of width points, for a synthetic bold face.
//
// Port: upstream has no synthetic bold. Pango asks Cairo to embolden a face
// that has no bold.
func (s *stream) fillBoldGlyphs(run display.GlyphRun, width float64) {
	shader := s.shader(s.state.fill)
	if shader == nil || run.Face == nil || len(run.Glyphs) == 0 {
		return
	}
	p := run.Path()
	s.record(func(d display.Device) {
		if sh := shader(); sh != nil {
			d.FillGlyphs(run, sh)
			if p != nil {
				st := okage.DefaultStroke()
				st.Width = float32(width)
				st.LineJoin = okage.LineJoinRound
				d.StrokePath(p, st, sh)
			}
		}
	})
}

// drawGlyphBitmap draws the bitmap of a glyph whose origin is at (x, y), at
// the font size size: a color bitmap as an image, and a mask with the color
// of the filling.
//
// Port: upstream draws the bitmap glyphs with Cairo.
func (s *stream) drawGlyphBitmap(b *ivyfont.Bitmap, x, y, size float64) {
	alpha := s.state.fill.alpha
	px := b.Image.Bounds()
	if alpha <= 0 || px.Dx() <= 0 || px.Dy() <= 0 {
		return
	}
	sx, sy := float64(b.W)*size/float64(px.Dx()), float64(b.H)*size/float64(px.Dy())
	t := okage.TransformFromRow(float32(sx), 0, 0, float32(sy),
		float32(x+float64(b.X)*size-float64(px.Min.X)*sx), float32(y+float64(b.Y)*size-float64(px.Min.Y)*sy))
	opts := display.ImageOptions{Opacity: float32(min(alpha, 1)), Smooth: s.dc.interpolate}
	if mask, ok := b.Image.(*image.Alpha); ok && b.Mask {
		shader := s.shader(s.state.fill)
		if shader == nil {
			return
		}
		opts.Opacity = 1
		s.record(func(d display.Device) {
			if sh := shader(); sh != nil {
				d.FillMask(mask, t, opts, sh)
			}
		})
		return
	}
	img := b.Image
	s.record(func(d display.Device) { d.DrawImage(img, t, opts) })
}

// absolute returns the transform from the current user space to the pixels
// of the device.
func (s *stream) absolute() Matrix {
	return s.state.ctm.Mul(s.base)
}
