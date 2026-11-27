package pict

import (
	"image"
	"image/color"
	"testing"

	"github.com/xo/ivy"
)

// box writes a rectangle: top, left, bottom and right.
func (b *pictBuilder) box(top, left, bottom, right int) {
	b.u16(top, left, bottom, right)
}

// end ends a picture of version 2 with OpEndPic, at an even byte.
func (b *pictBuilder) end() []byte {
	if b.Len()%2 == 1 {
		b.WriteByte(0)
	}
	b.u16(0x00FF)
	return b.Bytes()
}

// rgbFg writes RGBFgCol with the color c.
func (b *pictBuilder) rgbFg(c color.NRGBA) {
	b.u16(0x001A, int(c.R)*0x101, int(c.G)*0x101, int(c.B)*0x101)
}

// at returns the color of the point x, y of the frame in the image img of
// a frame of 100 by 100 units at the origin.
func at(img image.Image, x, y float64) color.NRGBA {
	s := float64(img.Bounds().Dx()) / 100
	return color.NRGBAModel.Convert(img.At(int(x*s+s/2), int(y*s+s/2))).(color.NRGBA)
}

// decode100 decodes a picture of 100 by 100 units, which draws at
// vectorScale.
func decode100(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := Decode(data, ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := img.Bounds().Size(), image.Pt(100*vectorScale, 100*vectorScale); got != want {
		t.Fatalf("size %v, want %v", got, want)
	}
	return img
}

var (
	opaqueBlack = color.NRGBA{0, 0, 0, 255}
	opaqueWhite = color.NRGBA{255, 255, 255, 255}
	clearColor  = color.NRGBA{}
	red         = color.NRGBA{255, 0, 0, 255}
	green       = color.NRGBA{0, 255, 0, 255}
	blue        = color.NRGBA{0, 0, 255, 255}
)

// checkColors fails t for each point of want whose color is not the color
// of the image img of decode100.
func checkColors(t *testing.T, img image.Image, want map[[2]float64]color.NRGBA) {
	t.Helper()
	for p, c := range want {
		if got := at(img, p[0], p[1]); got != c {
			t.Errorf("point %v: %v, want %v", p, got, c)
		}
	}
}

func TestShapes(t *testing.T) {
	b := newPict2(100, 100)
	// a red rectangle, painted
	b.rgbFg(red)
	b.u16(0x0031)
	b.box(10, 10, 30, 30)
	// a blue oval, framed with a pen of 2
	b.rgbFg(blue)
	b.u16(0x0007, 2, 2)
	b.u16(0x0050)
	b.box(10, 40, 50, 80)
	// a green line with a pen of 1
	b.rgbFg(green)
	b.u16(0x0007, 1, 1)
	b.u16(0x0020, 80, 10, 80, 50)
	// a rectangle that the gray pattern fills in black and white
	b.u16(0x001A, 0, 0, 0)
	b.u16(0x000A, 0xAA55, 0xAA55, 0xAA55, 0xAA55)
	b.u16(0x0034)
	b.box(60, 60, 90, 90)
	img := decode100(t, b.end())
	checkColors(t, img, map[[2]float64]color.NRGBA{
		{20, 20}: red,
		{5, 5}:   clearColor,
		{40, 30}: blue,
		{41, 30}: blue,
		{43, 30}: clearColor,
		{60, 30}: clearColor,
		{60, 10}: blue,
		{30, 80}: green,
		{30, 81}: clearColor,
		{60, 60}: opaqueBlack,
		{61, 60}: opaqueWhite,
		{60, 61}: opaqueWhite,
		{61, 61}: opaqueBlack,
	})
}

func TestSameShapesAndModes(t *testing.T) {
	b := newPict2(100, 100)
	// a black rectangle, and the left half of it inverted
	b.u16(0x0031)
	b.box(0, 0, 50, 50)
	b.u16(0x0033)
	b.box(0, 0, 50, 25)
	// a framed rectangle, then the same rectangle erased with the white
	// background pattern
	b.u16(0x0030)
	b.box(60, 60, 90, 90)
	b.u16(0x003A)
	// a pen of the mode patOr and a pattern of a bit in each byte paints
	// the bit and keeps the rest
	b.u16(0x0008, 9)
	b.u16(0x0009, 0x8000, 0, 0, 0)
	b.u16(0x0031)
	b.box(0, 64, 8, 72)
	img := decode100(t, b.end())
	checkColors(t, img, map[[2]float64]color.NRGBA{
		{10, 10}: opaqueWhite,
		{40, 10}: opaqueBlack,
		{60, 75}: opaqueWhite,
		{75, 75}: opaqueWhite,
		{64, 0}:  opaqueBlack,
		{65, 0}:  clearColor,
		{64, 1}:  clearColor,
	})
}

func TestArcsPolygonsRegions(t *testing.T) {
	b := newPict2(100, 100)
	// the wedge of the arc from 0 to 90 degrees: the top right quarter
	b.u16(0x0061)
	b.box(0, 0, 40, 40)
	b.u16(0, 90)
	// a triangle, painted
	b.rgbFg(red)
	b.u16(0x0071, 10+3*4)
	b.box(50, 50, 90, 90)
	b.u16(50, 50, 90, 50, 90, 90)
	// a region of two rectangles: rows 60 to 75 of the columns 10 to 20,
	// and rows 75 to 90 of the columns 10 to 30
	b.rgbFg(blue)
	b.u16(0x0081, 36)
	b.box(60, 10, 90, 30)
	b.u16(60, 10, 20, 0x7FFF, 75, 20, 30, 0x7FFF, 90, 10, 30, 0x7FFF, 0x7FFF)
	// the frame of the same region with a pen of 1, in green
	b.rgbFg(green)
	b.u16(0x0088)
	img := decode100(t, b.end())
	checkColors(t, img, map[[2]float64]color.NRGBA{
		{30, 10}: opaqueBlack,
		{10, 10}: clearColor,
		{10, 30}: clearColor,
		{55, 80}: red,
		{80, 55}: clearColor,
		{15, 70}: blue,
		{25, 70}: clearColor,
		{25, 80}: blue,
		{10, 70}: green,
		{29, 80}: green,
		{20, 89}: green,
	})
}

func TestClipAndOrigin(t *testing.T) {
	b := newPict2(100, 100)
	// a clip region of the top left quarter
	b.u16(0x0001, 10)
	b.box(0, 0, 50, 50)
	b.u16(0x0031)
	b.box(0, 0, 100, 100)
	// the clip of the frame, and an origin that moves the coordinates
	b.u16(0x0001, 10)
	b.box(0, 0, 100, 100)
	b.u16(0x000C, 10, 20)
	b.rgbFg(red)
	b.u16(0x0031)
	b.box(80, 70, 90, 80)
	img := decode100(t, b.end())
	checkColors(t, img, map[[2]float64]color.NRGBA{
		{25, 25}: opaqueBlack,
		{75, 25}: clearColor,
		{25, 75}: clearColor,
		{65, 65}: red,
		{75, 75}: clearColor,
	})
}

func TestPixPat(t *testing.T) {
	b := newPict2(100, 100)
	// a dithered pattern of one color
	b.u16(0x0014, 2)
	b.u16(0, 0, 0, 0)
	b.u16(0, 0xFFFF, 0)
	b.u16(0x0034)
	b.box(0, 0, 10, 10)
	// a pattern of pixels of 8 bits: a tile of 2 by 2 pixels, red on the
	// left and blue on the right
	b.u16(0x0014, 1)
	b.u16(0, 0, 0, 0)
	b.u16(0x8000 | 2)
	b.box(0, 0, 2, 2)
	b.pixMapHeader(0, 0, 8, 1, 8)
	b.u32(0)
	b.u16(0, 1)
	b.u16(0, 0xFFFF, 0, 0)
	b.u16(1, 0, 0, 0xFFFF)
	b.Write([]byte{0, 1, 0, 1})
	b.u16(0x0034)
	b.box(20, 20, 40, 40)
	img := decode100(t, b.end())
	checkColors(t, img, map[[2]float64]color.NRGBA{
		{5, 5}:   green,
		{20, 20}: red,
		{21, 20}: blue,
		{22, 21}: red,
	})
}

func TestText(t *testing.T) {
	dark := func(data []byte) int {
		img := decode100(t, data)
		n := 0
		bb := img.Bounds()
		for y := bb.Min.Y; y < bb.Max.Y; y++ {
			for x := bb.Min.X; x < bb.Max.X; x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a > 0x8000 {
					n++
				}
			}
		}
		return n
	}
	text := func(face int) []byte {
		b := newPict2(100, 100)
		// the font Helvetica, named by fontName, in 24 points
		b.u16(0x002C, 2+1+9, 300)
		b.WriteByte(9)
		b.WriteString("Helvetica")
		if b.Len()%2 == 1 {
			b.WriteByte(0)
		}
		b.u16(0x0003, 300, 0x000D, 24)
		b.u16(0x0004)
		b.WriteByte(byte(face))
		if b.Len()%2 == 1 {
			b.WriteByte(0)
		}
		b.u16(0x0028, 50, 5)
		b.WriteByte(2)
		b.WriteString("MM")
		// DHText moves the text across
		if b.Len()%2 == 1 {
			b.WriteByte(0)
		}
		b.u16(0x0029)
		b.Write([]byte{45, 2})
		b.WriteString("MM")
		return b.end()
	}
	plain, bold := dark(text(0)), dark(text(faceBold))
	if plain < 2000 {
		t.Errorf("plain text has %d dark pixels, want 2000 or more", plain)
	}
	if bold <= plain {
		t.Errorf("bold text has %d dark pixels, want more than the %d of plain text", bold, plain)
	}
}

// vectorPict returns a picture of version 2 with each kind of the opcodes
// of the lines, the shapes, the regions and the text, for the fuzz test.
func vectorPict() []byte {
	b := newPict2(40, 40)
	b.u16(0x0007, 2, 2, 0x000B, 6, 6)
	b.u16(0x0020, 1, 1, 30, 30, 0x0021, 5, 35, 0x0023, 0x0505)
	b.u16(0x0030)
	b.box(2, 2, 10, 10)
	b.u16(0x0041)
	b.box(12, 2, 20, 10)
	b.u16(0x0052)
	b.box(2, 12, 10, 20)
	b.u16(0x0063)
	b.box(12, 12, 20, 20)
	b.u16(45, 270, 0x0068, 0, 90)
	b.u16(0x0070, 10+3*4)
	b.box(20, 20, 30, 30)
	b.u16(20, 20, 30, 20, 30, 30, 0x0078)
	b.u16(0x0080, 36)
	b.box(20, 2, 35, 15)
	b.u16(20, 2, 8, 0x7FFF, 27, 8, 15, 0x7FFF, 35, 2, 15, 0x7FFF, 0x7FFF)
	b.u16(0x0088, 0x000A, 0xAA55, 0xAA55, 0xAA55, 0xAA55, 0x003C)
	b.u16(0x000D, 9, 0x0028, 38, 2)
	b.WriteByte(3)
	b.WriteString("PIC")
	return b.end()
}

func TestVectorPict(t *testing.T) {
	img, err := Decode(vectorPict(), ivy.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := img.Bounds().Size(), image.Pt(40*vectorScale, 40*vectorScale); got != want {
		t.Errorf("size %v, want %v", got, want)
	}
}
