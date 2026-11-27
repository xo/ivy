package pict

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/xo/ivy"
)

// pictBuilder builds a picture of version 2.
type pictBuilder struct {
	bytes.Buffer
}

func (b *pictBuilder) u16(v ...int) {
	for _, x := range v {
		_ = binary.Write(b, binary.BigEndian, uint16(x))
	}
}

func (b *pictBuilder) u32(v ...int) {
	for _, x := range v {
		_ = binary.Write(b, binary.BigEndian, uint32(x))
	}
}

// rect writes a rectangle of w by h at the origin.
func (b *pictBuilder) rect(w, h int) {
	b.u16(0, 0, h, w)
}

// newPict2 starts a picture of version 2 with a frame of w by h, and its
// extended header.
func newPict2(w, h int) *pictBuilder {
	b := &pictBuilder{}
	b.u16(0)
	b.rect(w, h)
	b.u16(0x0011, 0x02FF, 0x0C00, 0xFFFE, 0)
	b.u32(0x00480000, 0x00480000)
	b.rect(w, h)
	b.u32(0)
	// DefHilite and a clip region of the frame
	b.u16(0x001E, 0x0001, 10)
	b.rect(w, h)
	return b
}

// pixMapHeader writes the fields of a PixMap after its bounds.
func (b *pictBuilder) pixMapHeader(packType, pixelType, pixelSize, cmpCount, cmpSize int) {
	b.u16(0, packType)
	b.u32(0, 0x00480000, 0x00480000)
	b.u16(pixelType, pixelSize, cmpCount, cmpSize)
	b.u32(0, 0, 0)
}

// literal writes a row as one run of literal bytes of PackBits, with the
// count of a row of rowBytes.
func (b *pictBuilder) literal(row []byte, rowBytes int) {
	var packed []byte
	for len(row) > 0 {
		n := min(len(row), 128)
		packed = append(packed, byte(n-1))
		packed = append(packed, row[:n]...)
		row = row[n:]
	}
	if rowBytes > 250 {
		b.u16(len(packed))
	} else {
		b.WriteByte(byte(len(packed)))
	}
	b.Write(packed)
}

// directPict returns a picture of w by h with one DirectBitsRect of 32
// bits in planes, whose pixel x, y has the color of fill.
func directPict(w, h int, fill func(x, y int) color.NRGBA) []byte {
	b := newPict2(w, h)
	b.u16(0x009A)
	b.u32(0xFF)
	b.u16(0x8000 | 4*w)
	b.rect(w, h)
	b.pixMapHeader(4, 16, 32, 3, 8)
	b.rect(w, h)
	b.rect(w, h)
	b.u16(0)
	for y := range h {
		row := make([]byte, 3*w)
		for x := range w {
			c := fill(x, y)
			row[x], row[w+x], row[2*w+x] = c.R, c.G, c.B
		}
		b.literal(row, 4*w)
	}
	b.u16(0x00FF)
	return b.Bytes()
}

// indexedPict returns a picture of w by h with one PackBitsRect of 8 bits
// and a color table of two colors: the left half has the color 0.
func indexedPict(w, h int) []byte {
	b := newPict2(w, h)
	rowBytes := (w + 1) &^ 1
	b.u16(0x0098, 0x8000|rowBytes)
	b.rect(w, h)
	b.pixMapHeader(0, 0, 8, 1, 8)
	// the color table: seed, flags, count - 1, and value, red, green and
	// blue of each color
	b.u32(0)
	b.u16(0, 1)
	b.u16(0, 0xFFFF, 0, 0)
	b.u16(1, 0, 0, 0xFFFF)
	b.rect(w, h)
	b.rect(w, h)
	b.u16(0)
	for range h {
		row := make([]byte, rowBytes)
		for x := w / 2; x < w; x++ {
			row[x] = 1
		}
		if rowBytes < 8 {
			b.Write(row)
		} else {
			b.literal(row, rowBytes)
		}
	}
	b.u16(0x00FF)
	return b.Bytes()
}

// bitMapPict returns a picture of version 1 of w by h with one BitsRect of
// a BitMap whose first column is black.
func bitMapPict(w, h int) []byte {
	b := &pictBuilder{}
	b.u16(0)
	b.rect(w, h)
	b.Write([]byte{0x11, 0x01, 0x90})
	rowBytes := 2
	b.u16(rowBytes)
	b.rect(w, h)
	b.rect(w, h)
	b.rect(w, h)
	b.u16(0)
	for range h {
		b.Write([]byte{0x80, 0})
	}
	b.WriteByte(0xFF)
	return b.Bytes()
}

// quickTimePict returns a picture of version 2 of w by h with an image of
// QuickTime in JPEG of the color c.
func quickTimePict(t testing.TB, w, h int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	var d pictBuilder
	d.u16(0)
	// the matrix: a scale of 1 and no translation
	d.u32(0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000)
	d.u32(0) // the size of the matte
	d.rect(0, 0)
	d.u16(0)
	d.rect(w, h)
	d.u32(0, 0) // the accuracy and the size of the mask
	// the description of the image
	d.u32(86)
	d.WriteString("jpeg")
	d.u32(0)
	d.u16(0, 0, 0, 0)
	d.u32(0, 0, 0)
	d.u16(w, h)
	d.u32(0x00480000, 0x00480000, enc.Len())
	d.u16(1)
	d.Write(make([]byte, 32))
	d.u16(24, 0xFFFF)
	d.Write(enc.Bytes())
	b := newPict2(w, h)
	b.u16(0x8200)
	b.u32(d.Len())
	b.Write(d.Bytes())
	if b.Len()%2 == 1 {
		b.WriteByte(0)
	}
	b.u16(0x00FF)
	return b.Bytes()
}

func TestDecode(t *testing.T) {
	red, blue := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255}
	gradient := func(x, y int) color.NRGBA { return color.NRGBA{uint8(x * 10), uint8(y * 10), 7, 255} }
	tests := []struct {
		name string
		data []byte
		size image.Point
		at   map[image.Point]color.NRGBA
		near bool
	}{
		{
			name: "direct bits in planes",
			data: directPict(20, 10, gradient),
			size: image.Pt(20, 10),
			at:   map[image.Point]color.NRGBA{{0, 0}: gradient(0, 0), {19, 9}: gradient(19, 9), {7, 3}: gradient(7, 3)},
		},
		{
			name: "packed indexed pixmap",
			data: indexedPict(30, 4),
			size: image.Pt(30, 4),
			at:   map[image.Point]color.NRGBA{{0, 0}: red, {29, 3}: blue},
		},
		{
			name: "unpacked indexed pixmap",
			data: indexedPict(6, 3),
			size: image.Pt(6, 3),
			at:   map[image.Point]color.NRGBA{{2, 2}: red, {3, 0}: blue},
		},
		{
			name: "bitmap of version 1",
			data: bitMapPict(16, 5),
			size: image.Pt(16, 5),
			at:   map[image.Point]color.NRGBA{{0, 4}: {0, 0, 0, 255}, {1, 0}: {255, 255, 255, 255}},
		},
		{
			name: "quicktime jpeg",
			data: quickTimePict(t, 16, 16, blue),
			size: image.Pt(16, 16),
			at:   map[image.Point]color.NRGBA{{8, 8}: blue},
			near: true,
		},
		{
			name: "file header",
			data: append(make([]byte, headerSize), indexedPict(30, 4)...),
			size: image.Pt(30, 4),
			at:   map[image.Point]color.NRGBA{{0, 0}: red},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := Decode(tt.data, ivy.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if got := img.Bounds().Size(); got != tt.size {
				t.Fatalf("size %v, want %v", got, tt.size)
			}
			for p, want := range tt.at {
				got := color.NRGBAModel.Convert(img.At(p.X, p.Y)).(color.NRGBA)
				if tt.near {
					if diff(got.R, want.R) > 8 || diff(got.G, want.G) > 8 || diff(got.B, want.B) > 8 || got.A != 255 {
						t.Errorf("pixel %v: %v, want about %v", p, got, want)
					}
				} else if got != want {
					t.Errorf("pixel %v: %v, want %v", p, got, want)
				}
			}
		})
	}
}

func diff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func TestDecodeErrors(t *testing.T) {
	big := directPict(64, 64, func(int, int) color.NRGBA { return color.NRGBA{A: 255} })
	tests := []struct {
		name string
		data []byte
		lim  ivy.Limits
		want error
	}{
		{"empty", nil, ivy.Limits{}, ivy.ErrFormat},
		{"no version", make([]byte, 40), ivy.Limits{}, ivy.ErrFormat},
		{"no bitmap", append(newPict2(10, 10).Bytes(), 0, 0xFF), ivy.Limits{}, ivy.ErrFormat},
		{"too many pixels", big, ivy.Limits{MaxPixels: 1000}, ivy.ErrLimit},
		{"too many opcodes", big, ivy.Limits{MaxObjects: 2}, ivy.ErrLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode(tt.data, tt.lim); !errors.Is(err, tt.want) {
				t.Errorf("error %v, want %v", err, tt.want)
			}
		})
	}
	// a picture cut in its bitmap keeps no bitmap
	if _, err := Decode(big[:len(big)/2], ivy.Limits{}); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("cut picture: error %v, want %v", err, ivy.ErrFormat)
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(directPict(5, 3, func(x, y int) color.NRGBA { return color.NRGBA{uint8(x), uint8(y), 0, 255} }))
	f.Add(indexedPict(12, 2))
	f.Add(indexedPict(3, 2))
	f.Add(bitMapPict(8, 2))
	f.Add(quickTimePict(f, 8, 8, color.NRGBA{0, 255, 0, 255}))
	f.Add(vectorPict())
	f.Fuzz(func(t *testing.T, data []byte) {
		img, err := Decode(data, ivy.Limits{MaxPixels: 1 << 20, MaxMemory: 16 << 20, MaxObjects: 1 << 12})
		if err == nil && img.Bounds().Empty() {
			t.Error("empty image without an error")
		}
	})
}
