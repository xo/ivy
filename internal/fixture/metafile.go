package fixture

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"unicode/utf16"
)

// EMF builds an Enhanced Metafile from its records (MS-EMF). The reference
// device has 100 pixels to the inch, and the picture is a frame of pixels
// of that device. The logical units are the pixels, the default of GDI.
type EMF struct {
	width, height int
	records       [][]byte
	handles       uint32
	program       string
	title         string
}

// NewEMF returns an EMF file of width by height pixels. A size that is a
// multiple of 5 gives an exact frame.
func NewEMF(width, height int) *EMF {
	return &EMF{width: width, height: height}
}

// le appends the values to b in little-endian order. An int is 32 bits, and
// an int16 or a uint16 is 16 bits.
func le(b []byte, values ...any) []byte {
	for _, v := range values {
		switch v := v.(type) {
		case int:
			b = binary.LittleEndian.AppendUint32(b, uint32(int32(v)))
		case uint32:
			b = binary.LittleEndian.AppendUint32(b, v)
		case int16:
			b = binary.LittleEndian.AppendUint16(b, uint16(v))
		case uint16:
			b = binary.LittleEndian.AppendUint16(b, v)
		case float32:
			b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
		case color.NRGBA:
			b = append(b, v.R, v.G, v.B, 0)
		case []byte:
			b = append(b, v...)
		default:
			panic("fixture: bad value")
		}
	}
	return b
}

// Record appends a record of the type typ with the body of the values, as le
// writes them.
func (e *EMF) Record(typ uint32, values ...any) *EMF {
	body := le(nil, values...)
	for len(body)%4 != 0 {
		body = append(body, 0)
	}
	e.records = append(e.records, le(nil, typ, uint32(8+len(body)), body))
	return e
}

// handle returns a new handle of an object.
func (e *EMF) handle() uint32 {
	e.handles++
	return e.handles
}

// Describe sets the description of the header: the program and the title.
func (e *EMF) Describe(program, title string) *EMF {
	e.program, e.title = program, title
	return e
}

// Stock returns the handle of the stock object n (MS-EMF 2.1.31), such as 5
// for NULL_BRUSH and 8 for NULL_PEN.
func Stock(n uint32) uint32 {
	return 0x80000000 | n
}

// Brush creates a solid brush, or a hatched brush when hatch is 0 or more,
// and returns its handle (EMR_CREATEBRUSHINDIRECT).
func (e *EMF) Brush(c color.NRGBA, hatch int) uint32 {
	h := e.handle()
	style := uint32(0)
	if hatch >= 0 {
		style = 2
	}
	e.Record(39, h, style, c, uint32(max(hatch, 0)))
	return h
}

// Pen creates a pen and returns its handle (EMR_CREATEPEN). A style of 0 is a
// solid pen, and 5 is a null pen.
func (e *EMF) Pen(style uint32, width int, c color.NRGBA) uint32 {
	h := e.handle()
	e.Record(38, h, style, width, 0, c)
	return h
}

// Font creates a font and returns its handle (EMR_EXTCREATEFONTINDIRECTW).
// A negative height is the size of the em.
func (e *EMF) Font(height, weight int, italic bool, face string) uint32 {
	h := e.handle()
	name := make([]byte, 64)
	for i, u := range utf16.Encode([]rune(face)) {
		if i < 31 {
			binary.LittleEndian.PutUint16(name[2*i:], u)
		}
	}
	var it byte
	if italic {
		it = 1
	}
	e.Record(82, h, height, 0, 0, 0, weight, []byte{it, 0, 0, 1, 0, 0, 0, 0}, name)
	return h
}

// Select selects the object h (EMR_SELECTOBJECT).
func (e *EMF) Select(h uint32) *EMF {
	return e.Record(37, h)
}

// Rectangle draws a rectangle with the pen and the brush (EMR_RECTANGLE).
func (e *EMF) Rectangle(left, top, right, bottom int) *EMF {
	return e.Record(43, left, top, right, bottom)
}

// Ellipse draws an ellipse with the pen and the brush (EMR_ELLIPSE).
func (e *EMF) Ellipse(left, top, right, bottom int) *EMF {
	return e.Record(42, left, top, right, bottom)
}

// Polyline draws lines through the points with the pen (EMR_POLYLINE16).
// The points are pairs of coordinates.
func (e *EMF) Polyline(pts ...int) *EMF {
	values := make([]any, 0, 5+len(pts))
	values = append(values, 0, 0, 0, 0, uint32(len(pts)/2))
	for _, v := range pts {
		values = append(values, int16(v))
	}
	return e.Record(87, values...)
}

// IntersectClip intersects the clip with a rectangle
// (EMR_INTERSECTCLIPRECT).
func (e *EMF) IntersectClip(left, top, right, bottom int) *EMF {
	return e.Record(30, left, top, right, bottom)
}

// ExcludeClip takes a rectangle out of the clip (EMR_EXCLUDECLIPRECT).
func (e *EMF) ExcludeClip(left, top, right, bottom int) *EMF {
	return e.Record(29, left, top, right, bottom)
}

// TextColor sets the color of the text (EMR_SETTEXTCOLOR).
func (e *EMF) TextColor(c color.NRGBA) *EMF {
	return e.Record(24, c)
}

// Transparent sets the mode of the background to TRANSPARENT
// (EMR_SETBKMODE).
func (e *EMF) Transparent() *EMF {
	return e.Record(18, uint32(1))
}

// Text draws the string s with its reference point at (x, y), with the
// alignment of the text, which is the top left by default
// (EMR_EXTTEXTOUTW).
func (e *EMF) Text(x, y int, s string) *EMF {
	u := utf16.Encode([]rune(s))
	str := le(nil)
	for _, c := range u {
		str = le(str, c)
	}
	// the string follows the 76 bytes of the record up to offDx
	return e.Record(84, 0, 0, -1, -1, uint32(1), float32(1), float32(1),
		x, y, uint32(len(u)), uint32(76), uint32(0), 0, 0, -1, -1, uint32(0), str)
}

// Image draws img into the rectangle at (x, y) of w by h, as a DIB of 32
// bits (EMR_STRETCHDIBITS with SRCCOPY).
func (e *EMF) Image(x, y, w, h int, img *image.NRGBA) *EMF {
	b := img.Bounds()
	bmi := le(nil, uint32(40), b.Dx(), -b.Dy(), uint16(1), uint16(32), uint32(0), uint32(4*b.Dx()*b.Dy()), 0, 0, uint32(0), uint32(0))
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
		uint32(0), uint32(0x00cc0020), w, h, bmi, bits)
}

// Gradient fills the rectangle with a gradient from c0 to c1, across or, if
// vertical, down (EMR_GRADIENTFILL).
func (e *EMF) Gradient(left, top, right, bottom int, c0, c1 color.NRGBA, vertical bool) *EMF {
	mode := uint32(0)
	if vertical {
		mode = 1
	}
	vertex := func(x, y int, c color.NRGBA) []any {
		return []any{x, y, uint16(c.R) << 8, uint16(c.G) << 8, uint16(c.B) << 8, uint16(0)}
	}
	values := make([]any, 0, 21)
	values = append(values, left, top, right, bottom, uint32(2), uint32(1), mode)
	values = append(values, vertex(left, top, c0)...)
	values = append(values, vertex(right, bottom, c1)...)
	values = append(values, uint32(0), uint32(1))
	return e.Record(118, values...)
}

// Bytes returns the file: the header, the records and EMR_EOF.
func (e *EMF) Bytes() []byte {
	var desc []byte
	if e.program != "" || e.title != "" {
		for _, c := range utf16.Encode([]rune(e.program + "\x00" + e.title + "\x00\x00")) {
			desc = le(desc, c)
		}
	}
	// the header with the fields of both extensions, which are zero, and
	// the description after them
	size := 108 + len(desc)
	for size%4 != 0 {
		size++
	}
	header := le(nil, uint32(1), uint32(size),
		0, 0, e.width-1, e.height-1,
		0, 0, e.width*254/10, e.height*254/10,
		uint32(0x464d4520), uint32(0x10000), uint32(0), uint32(len(e.records)+2),
		uint16(e.handles+1), uint16(0), uint32(len(desc)/2), uint32(108), uint32(0),
		1000, 1000, 254, 254, 0, 0, 0, 0, 0)
	header = append(header, desc...)
	for len(header) < size {
		header = append(header, 0)
	}
	if len(desc) == 0 {
		binary.LittleEndian.PutUint32(header[64:], 0)
	}
	out := header
	for _, r := range e.records {
		out = append(out, r...)
	}
	out = le(out, uint32(14), uint32(20), uint32(0), uint32(16), uint32(20))
	binary.LittleEndian.PutUint32(out[48:], uint32(len(out)))
	return out
}

// WMF builds a Windows Metafile of 16 bits from its records (MS-WMF), with a
// placeable header, or without one and with a window of its size.
type WMF struct {
	width, height int
	placeable     bool
	records       [][]byte
	objects       uint16
}

// NewWMF returns a WMF file of width by height logical units. With a
// placeable header, the file has 100 units to the inch.
func NewWMF(width, height int, placeable bool) *WMF {
	w := &WMF{width: width, height: height, placeable: placeable}
	if !placeable {
		w.Record(0x020b, int16(0), int16(0))
		w.Record(0x020c, int16(height), int16(width))
	}
	return w
}

// Record appends a record of the function fn with the parameters, as le
// writes them.
func (w *WMF) Record(fn uint16, values ...any) *WMF {
	params := le(nil, values...)
	if len(params)%2 != 0 {
		params = append(params, 0)
	}
	w.records = append(w.records, le(nil, uint32(3+len(params)/2), fn, params))
	return w
}

// Brush creates a solid brush, and returns its index
// (META_CREATEBRUSHINDIRECT).
func (w *WMF) Brush(c color.NRGBA) uint16 {
	w.Record(0x02fc, uint16(0), c, uint16(0))
	w.objects++
	return w.objects - 1
}

// NullPen creates a null pen, and returns its index
// (META_CREATEPENINDIRECT).
func (w *WMF) NullPen() uint16 {
	w.Record(0x02fa, uint16(5), int16(0), int16(0), color.NRGBA{})
	w.objects++
	return w.objects - 1
}

// Select selects the object at index i (META_SELECTOBJECT).
func (w *WMF) Select(i uint16) *WMF {
	return w.Record(0x012d, i)
}

// Rectangle draws a rectangle with the pen and the brush (META_RECTANGLE).
func (w *WMF) Rectangle(left, top, right, bottom int) *WMF {
	return w.Record(0x041b, int16(bottom), int16(right), int16(top), int16(left))
}

// Bytes returns the file: the headers, the records and META_EOF.
func (w *WMF) Bytes() []byte {
	var body []byte
	maxRecord := uint32(3)
	for _, r := range w.records {
		body = append(body, r...)
		maxRecord = max(maxRecord, binary.LittleEndian.Uint32(r))
	}
	body = le(body, uint32(3), uint16(0))
	header := le(nil, uint16(1), uint16(9), uint16(0x300), uint32((18+len(body))/2), w.objects, maxRecord, uint16(0))
	header = append(header, body...)
	if !w.placeable {
		return header
	}
	p := le(nil, uint32(0x9ac6cdd7), uint16(0), int16(0), int16(0), int16(w.width), int16(w.height), uint16(100), uint32(0))
	var sum uint16
	for i := 0; i < 20; i += 2 {
		sum ^= binary.LittleEndian.Uint16(p[i:])
	}
	p = le(p, sum)
	return append(p, header...)
}
