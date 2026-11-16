package metafile

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

var (
	white = color.NRGBA{255, 255, 255, 255}
	cyan  = color.NRGBA{0, 255, 255, 255}
)

// stretchDIBits appends an EMR_STRETCHDIBITS record that draws img into the
// rectangle at (x, y) of w by h with the raster operation rop.
func stretchDIBits(e *fixture.EMF, x, y, w, h int, img *image.NRGBA, rop uint32) *fixture.EMF {
	b := img.Bounds()
	le := binary.LittleEndian
	bmi := le.AppendUint32(nil, 40)
	bmi = le.AppendUint32(bmi, uint32(b.Dx()))
	bmi = le.AppendUint32(bmi, uint32(int32(-b.Dy())))
	bmi = le.AppendUint16(bmi, 1)
	bmi = le.AppendUint16(bmi, 32)
	bmi = append(bmi, make([]byte, 24)...)
	var bits []byte
	for py := b.Min.Y; py < b.Max.Y; py++ {
		for px := b.Min.X; px < b.Max.X; px++ {
			c := img.NRGBAAt(px, py)
			bits = append(bits, c.B, c.G, c.R, 0)
		}
	}
	const head = 80
	return e.Record(81, 0, 0, -1, -1, x, y, 0, 0, b.Dx(), b.Dy(),
		uint32(head), uint32(len(bmi)), uint32(head+len(bmi)), uint32(len(bits)),
		uint32(0), rop, w, h, bmi, bits)
}

// patBlt appends an EMR_BITBLT record without a source bitmap.
func patBlt(e *fixture.EMF, x, y, w, h int, rop uint32) *fixture.EMF {
	return e.Record(76, 0, 0, -1, -1, x, y, w, h, rop, 0, 0,
		float32(1), float32(0), float32(0), float32(1), float32(0), float32(0),
		uint32(0), uint32(0), uint32(0), uint32(0), uint32(0), uint32(0))
}

// halves returns an image of 2 by 1 pixels: black on the left and white on
// the right.
func halves() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, black)
	img.SetNRGBA(1, 0, white)
	return img
}

func TestRasterOperations(t *testing.T) {
	for _, tt := range []struct {
		name        string
		draw        func(e *fixture.EMF)
		left, right color.NRGBA
	}{
		// SRCAND keeps the page where the source is white
		{"SRCAND", func(e *fixture.EMF) { stretchDIBits(e, 0, 0, 100, 100, halves(), 0x008800c6) }, black, red},
		// SRCPAINT keeps the page where the source is black
		{"SRCPAINT", func(e *fixture.EMF) { stretchDIBits(e, 0, 0, 100, 100, halves(), 0x00ee0086) }, red, white},
		// SRCINVERT inverts the page where the source is white
		{"SRCINVERT", func(e *fixture.EMF) { stretchDIBits(e, 0, 0, 100, 100, halves(), 0x00660046) }, red, cyan},
		// SRCERASE is the source and the inverse of the page
		{"SRCERASE", func(e *fixture.EMF) { stretchDIBits(e, 0, 0, 100, 100, halves(), 0x00440328) }, black, cyan},
		// MERGECOPY is the source and the brush, without the page
		{"MERGECOPY", func(e *fixture.EMF) {
			e.Select(e.Brush(green, -1))
			stretchDIBits(e, 0, 0, 100, 100, halves(), 0x00c000ca)
		}, black, green},
		// DSPDxax draws the brush where the source is white
		{"DSPDxax", func(e *fixture.EMF) {
			e.Select(e.Brush(blue, -1))
			stretchDIBits(e, 0, 0, 100, 100, halves(), 0x00e20746)
		}, red, blue},
		// DSTINVERT inverts the page in its rectangle
		{"DSTINVERT", func(e *fixture.EMF) { patBlt(e, 50, 0, 50, 100, 0x00550009) }, red, cyan},
		// PATINVERT inverts the page where the brush is white
		{"PATINVERT", func(e *fixture.EMF) {
			e.Select(e.Brush(white, -1))
			patBlt(e, 50, 0, 50, 100, 0x005a0049)
		}, red, cyan},
		// R2_MASKPEN fills with the brush and the page
		{"R2_MASKPEN", func(e *fixture.EMF) {
			e.Select(e.Brush(color.NRGBA{255, 255, 0, 255}, -1))
			e.Record(20, uint32(9)).Rectangle(-1, -1, 50, 101)
			e.Select(e.Brush(color.NRGBA{0, 255, 255, 255}, -1))
			e.Rectangle(50, -1, 101, 101)
		}, red, black},
		// R2_XORPEN inverts the page where the brush is white
		{"R2_XORPEN", func(e *fixture.EMF) {
			e.Select(e.Brush(white, -1))
			e.Record(20, uint32(7)).Rectangle(50, -1, 101, 101)
		}, red, cyan},
		// EMR_INVERTRGN inverts the page in the region
		{"InvertRgn", func(e *fixture.EMF) {
			region := []any{uint32(32), uint32(1), uint32(1), uint32(16), 50, 0, 100, 100, 50, 0, 100, 100}
			e.Record(73, append([]any{50, 0, 100, 100, uint32(48)}, region...)...)
		}, red, cyan},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := filled(red).Rectangle(-1, -1, 101, 101)
			tt.draw(e)
			pm, err := render(t, e.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			expect(t, pm, 25, 50, tt.left)
			expect(t, pm, 75, 50, tt.right)
		})
	}
}
