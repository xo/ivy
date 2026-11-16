package metafile

import (
	"fmt"
	"image"
	"image/color"

	"github.com/xo/okage"

	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/gowemf"
)

// The interfaces that a backend implements.
var _ gowemf.RasterBackend = (*backend)(nil)

// DrawRaster satisfies the gowemf.RasterBackend interface. It draws a
// ternary raster operation of GDI, which combines a pattern P, a source
// bitmap S and the destination D bit by bit.
//
// The device cannot read its pixels, so the backend draws the operation as
// blend modes. For each pair of bits of P and S, the operation gives 0, 1,
// D or the inverse of D. So the result is ((D xor A) and M) or O, where A,
// M and O are images of P and S alone. The backend draws A with the blend
// mode Difference, M with Multiply and O with Lighten, and leaves out each
// one that changes nothing. The result is exact where P, S and D are black
// or white in each channel, which is the case of the masks that programs
// draw with these operations, such as SRCAND and SRCPAINT. It is close for
// other colors.
func (b *backend) DrawRaster(r gowemf.RasterDraw, clip gowemf.Clip) error {
	if err := b.begin(); err != nil {
		return err
	}
	area := b.path(r.Area)
	if area == nil {
		return nil
	}
	var a, m, o gowemf.RasterOperation
	for i := uint(0); i < 8; i += 2 {
		r0, r1 := r.Operation>>i&1, r.Operation>>(i+1)&1
		// the bits i and i+1 of each derived operation, so that it does not
		// depend on D
		both := gowemf.RasterOperation(3) << i
		if r0 == 1 && r1 == 0 {
			a |= both
		}
		if r0 != r1 {
			m |= both
		}
		if r0 == 1 && r1 == 1 {
			o |= both
		}
	}
	type pass struct {
		op    gowemf.RasterOperation
		blend okage.BlendMode
	}
	// the result does not depend on D if m is 0
	passes := []pass{{o, okage.BlendModeSourceOver}}
	if m != 0 {
		passes = passes[:0]
		if a != 0 {
			passes = append(passes, pass{a, okage.BlendModeDifference})
		}
		if m != 0xff {
			passes = append(passes, pass{m, okage.BlendModeMultiply})
		}
		if o != 0 {
			passes = append(passes, pass{o, okage.BlendModeLighten})
		}
	}
	// make each operand before the first one is drawn, so that the
	// operation is drawn whole or not at all
	draws := make([]func(), len(passes))
	for i, p := range passes {
		var why string
		draws[i], why = b.rasterOperand(p.op, r, area)
		if why != "" {
			b.skip(fmt.Sprintf("raster operation 0x%02x: %s", uint8(r.Operation), why))
			return nil
		}
	}
	for i, p := range passes {
		draw := draws[i]
		if draw == nil {
			continue
		}
		if p.blend != okage.BlendModeSourceOver {
			b.d.BeginGroup(display.Group{Opacity: 1, Blend: p.blend, Isolated: true})
		}
		b.clipped(clip, func() {
			b.d.Save()
			b.d.ClipPath(area, okage.FillRuleWinding)
			draw()
			b.d.Restore()
		})
		if p.blend != okage.BlendModeSourceOver {
			b.d.EndGroup()
		}
	}
	return nil
}

// rasterOperand returns the function that draws the image of P and S that op
// gives, over area, the area of the transfer. It returns a reason instead when
// the backend cannot draw it.
func (b *backend) rasterOperand(op gowemf.RasterOperation, r gowemf.RasterDraw, area *okage.Path) (draw func(), why string) {
	fill := func(shader okage.Shader) func() {
		return func() { b.d.FillPath(area, okage.FillRuleWinding, shader) }
	}
	switch {
	case op == 0x00:
		return fill(okage.SolidColor(okage.ColorBlack)), ""
	case op == 0xff:
		return fill(okage.SolidColor(okage.ColorWhite)), ""
	case !op.UsesSource():
		// op is P or the inverse of P
		if r.Pattern == nil {
			return nil, "no brush"
		}
		paint := *r.Pattern
		if op == 0x0f {
			var ok bool
			if paint, ok = invertPaint(paint); !ok {
				return nil, "the inverse of a pattern brush"
			}
		}
		shader := b.shader(paint)
		if shader == nil {
			return nil, ""
		}
		return fill(shader), ""
	}
	if r.Source == nil {
		return nil, "no source bitmap"
	}
	var pc color.NRGBA
	if op.UsesPattern() {
		if r.Pattern == nil || r.Pattern.Kind != gowemf.PaintSolid {
			return nil, "a pattern brush with a source bitmap"
		}
		pc = r.Pattern.Color
	}
	src := r.Source.Source.Intersect(r.Source.Image.Bounds())
	if src.Empty() {
		return nil, ""
	}
	if int64(src.Dx())*int64(src.Dy()) > b.pic.lim.MaxPixels {
		return nil, "a source bitmap past the limit of pixels"
	}
	out := image.NewNRGBA(src)
	for y := src.Min.Y; y < src.Max.Y; y++ {
		for x := src.Min.X; x < src.Max.X; x++ {
			s := color.NRGBAModel.Convert(r.Source.Image.At(x, y)).(color.NRGBA)
			out.SetNRGBA(x, y, color.NRGBA{op.Apply(pc.R, s.R, 0), op.Apply(pc.G, s.G, 0), op.Apply(pc.B, s.B, 0), 255})
		}
	}
	im := *r.Source
	im.Image, im.Source, im.Opacity = out, src, 1
	img, t, ok := b.placeImage(im)
	if !ok {
		return nil, ""
	}
	return func() {
		b.d.DrawImage(img, t, display.ImageOptions{Opacity: 1, Smooth: im.Smooth})
	}, ""
}

// invertPaint returns the inverse of the colors of a solid color or of a
// hatch, and false for another paint.
func invertPaint(p gowemf.Paint) (gowemf.Paint, bool) {
	invert := func(c color.NRGBA) color.NRGBA { return color.NRGBA{^c.R, ^c.G, ^c.B, c.A} }
	switch p.Kind {
	case gowemf.PaintSolid, gowemf.PaintHatch:
		p.Color = invert(p.Color)
		if p.Background != nil {
			bg := invert(*p.Background)
			p.Background = &bg
		}
		return p, true
	}
	return p, false
}
