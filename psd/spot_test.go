package psd

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"testing"
)

// psdFile builds an RGB file of 8 bits with a merged image of w by h, without
// layers. planes holds a plane of w*h bytes for each channel, and res holds
// the image resources by their identifier.
func psdFile(w, h int, planes [][]byte, res map[uint16][]byte) []byte {
	var b bytes.Buffer
	put := func(v any) {
		_ = binary.Write(&b, binary.BigEndian, v)
	}
	b.WriteString("8BPS")
	put(uint16(1))
	b.Write(make([]byte, 6))
	put(uint16(len(planes)))
	put(uint32(h))
	put(uint32(w))
	put(uint16(8))
	put(uint16(3))
	put(uint32(0))
	var r bytes.Buffer
	for _, id := range []uint16{resDisplayInfoOld, resAlphaIDs, resAltSpotColors, resDisplayInfo} {
		data, ok := res[id]
		if !ok {
			continue
		}
		r.WriteString("8BIM")
		_ = binary.Write(&r, binary.BigEndian, id)
		r.Write([]byte{0, 0})
		_ = binary.Write(&r, binary.BigEndian, uint32(len(data)))
		r.Write(data)
		if len(data)%2 == 1 {
			r.WriteByte(0)
		}
	}
	put(uint32(r.Len()))
	b.Write(r.Bytes())
	put(uint32(0))
	put(uint16(0))
	for _, p := range planes {
		b.Write(p)
	}
	return b.Bytes()
}

// colorStruct returns the color structure of Photoshop for a space and its
// four values.
func colorStruct(space uint16, v ...uint16) []byte {
	b := binary.BigEndian.AppendUint16(nil, space)
	for i := range 4 {
		var c uint16
		if i < len(v) {
			c = v[i]
		}
		b = binary.BigEndian.AppendUint16(b, c)
	}
	return b
}

// displayInfoRes returns the resource of the display information of Photoshop
// CS3 for channels of a color, an opacity and a kind.
func displayInfoRes(entries ...[]byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, 1)
	for _, e := range entries {
		b = append(b, e...)
	}
	return b
}

// displayEntry returns an entry of the display information.
func displayEntry(c []byte, opacity uint16, kind byte) []byte {
	return append(binary.BigEndian.AppendUint16(append([]byte(nil), c...), opacity), kind)
}

// renderPSD decodes the image of the file in buf.
func renderPSD(t *testing.T, buf []byte) *image.NRGBA {
	t.Helper()
	img, err := Decode(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	n, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("expected an *image.NRGBA, got: %T", img)
	}
	return n
}

func TestSpotSolidity(t *testing.T) {
	// a red image of 2 by 1, with full blue ink on the first pixel only
	red := [][]byte{{255, 255}, {0, 0}, {0, 0}, {0, 255}}
	blue := colorStruct(spaceRGB, 0, 0, 0xffff)
	tests := []struct {
		solidity uint16
		exp      color.NRGBA
	}{
		{0, color.NRGBA{0, 0, 0, 255}},
		{50, color.NRGBA{0, 0, 128, 255}},
		{100, color.NRGBA{0, 0, 255, 255}},
	}
	for _, test := range tests {
		res := map[uint16][]byte{
			resAlphaIDs:    binary.BigEndian.AppendUint32(nil, 5),
			resDisplayInfo: displayInfoRes(displayEntry(blue, test.solidity, kindSpot)),
		}
		img := renderPSD(t, psdFile(2, 1, red, res))
		if c := img.NRGBAAt(0, 0); c != test.exp {
			t.Errorf("solidity %d: expected %v, got: %v", test.solidity, test.exp, c)
		}
		if c, exp := img.NRGBAAt(1, 0), (color.NRGBA{255, 0, 0, 255}); c != exp {
			t.Errorf("solidity %d: expected %v without ink, got: %v", test.solidity, exp, c)
		}
	}
}

func TestSpotAlternateColor(t *testing.T) {
	// a white image of 1 by 1 with a spot channel of full ink, whose color
	// structure is a book color, then an alpha channel that must not draw
	white := [][]byte{{255}, {255}, {255}, {0}, {0}}
	book := colorStruct(3, 1, 2, 3, 4)
	alt := binary.BigEndian.AppendUint16(nil, 1)
	alt = binary.BigEndian.AppendUint16(alt, 1)
	alt = binary.BigEndian.AppendUint32(alt, 7)
	// sRGB red is Lab 54.29, 80.80, 69.89 under the white of 5000 K
	alt = append(alt, colorStruct(spaceLab, 5429, uint16(8080), uint16(6989))...)
	res := map[uint16][]byte{
		resAlphaIDs: append(binary.BigEndian.AppendUint32(nil, 7), 0, 0, 0, 9),
		resDisplayInfo: displayInfoRes(
			displayEntry(book, 0, kindSpot),
			displayEntry(colorStruct(spaceRGB, 0xffff), 50, 1),
		),
		resAltSpotColors: alt,
	}
	img := renderPSD(t, psdFile(1, 1, white, res))
	c := img.NRGBAAt(0, 0)
	if c.R < 250 || c.G > 5 || c.B > 5 || c.A != 255 {
		t.Errorf("expected red, got: %v", c)
	}
	// without the alternate color, the book color does not convert, and the
	// page stays white
	delete(res, resAltSpotColors)
	img = renderPSD(t, psdFile(1, 1, white, res))
	if c, exp := img.NRGBAAt(0, 0), (color.NRGBA{255, 255, 255, 255}); c != exp {
		t.Errorf("expected %v, got: %v", exp, c)
	}
}

func TestSpotMismatch(t *testing.T) {
	// two extra channels and one entry of display information: the extra
	// channels are unknown and do not draw
	white := [][]byte{{255}, {255}, {255}, {0}, {0}}
	res := map[uint16][]byte{
		resAlphaIDs:    append(binary.BigEndian.AppendUint32(nil, 7), 0, 0, 0, 9),
		resDisplayInfo: displayInfoRes(displayEntry(colorStruct(spaceRGB), 100, kindSpot)),
	}
	img := renderPSD(t, psdFile(1, 1, white, res))
	if c, exp := img.NRGBAAt(0, 0), (color.NRGBA{255, 255, 255, 255}); c != exp {
		t.Errorf("expected %v, got: %v", exp, c)
	}
}

func TestLabToSRGB(t *testing.T) {
	tests := []struct {
		l, a, b float64
		exp     [3]float64
	}{
		{100, 0, 0, [3]float64{1, 1, 1}},
		{0, 0, 0, [3]float64{0, 0, 0}},
		{54.29, 80.80, 69.89, [3]float64{1, 0, 0}},
		{29.57, 68.29, -112.03, [3]float64{0, 0, 1}},
	}
	for _, test := range tests {
		c := labToSRGB(test.l, test.a, test.b)
		for i := range c {
			if math.Abs(c[i]-test.exp[i]) > 0.01 {
				t.Errorf("Lab %v %v %v: expected %v, got: %v", test.l, test.a, test.b, test.exp, c)
				break
			}
		}
	}
}

func TestHSBToRGB(t *testing.T) {
	if c, exp := hsbToRGB(240, 1, 1), ([3]float64{0, 0, 1}); c != exp {
		t.Errorf("expected %v, got: %v", exp, c)
	}
	if c, exp := hsbToRGB(0, 0, 0.5), ([3]float64{0.5, 0.5, 0.5}); c != exp {
		t.Errorf("expected %v, got: %v", exp, c)
	}
}
