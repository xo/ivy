// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports no file of hayro-font, and it does not port skrifa.
// hayro-font reads a Type 1 font program (FontFile, and a Type 1 font in a
// CID font) with skrifa in src/program/mod.rs. go-text v0.3.5 has no reader
// of Type 1 fonts: it has no parser of PFA or PFB files, no eexec
// decryption, and no interpreter of Type 1 charstrings. Its interpreter in
// font/cff/interpreter runs Type 2 charstrings only. The source of skrifa is
// not in the checkout, so this file is written from the format.
//
// The reader follows "Adobe Type 1 Font Format" of Adobe. It reads the
// binary form PFB and the plain form PFA, decrypts the private part, and
// runs the charstrings of a glyph to draw its outline. It supports flex, the
// replacement of hints, seac, and the blend of a multiple master font.

package pdffont

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/xo/okage/kurbo"

	"github.com/xo/ivy"
	"github.com/xo/ivy/pdf/internal/postscript"
)

// The limits of the reader of Type 1 fonts. A font is untrusted input.
const (
	// maxType1Glyphs bounds the glyphs of a font.
	maxType1Glyphs = 65535
	// maxType1Subrs bounds the index of a subroutine.
	maxType1Subrs = 65535
	// maxType1Stack bounds the operand stack of a charstring. The format
	// allows 24 operands. Some fonts use more.
	maxType1Stack = 64
	// maxType1PSStack bounds the stack that callothersubr and pop share.
	maxType1PSStack = 64
	// maxType1Depth bounds the nesting of callsubr.
	maxType1Depth = 16
	// maxType1Ops bounds the operators that one glyph runs, with its
	// subroutines and the glyphs of seac.
	maxType1Ops = 100000
	// maxType1Flex bounds the points of one flex.
	maxType1Flex = 16
)

// The keys of the encryption of Type 1 fonts.
const (
	eexecKey      = 55665
	charstringKey = 4330
)

var (
	errType1      = fmt.Errorf("malformed Type 1 font: %w", ivy.ErrFormat)
	errType1Limit = fmt.Errorf("type 1 charstring: %w", ivy.ErrLimit)
)

// type1Program is a parsed Type 1 font.
type type1Program struct {
	matrix kurbo.Affine
	// encoding holds the glyph name of each code of the built-in
	// encoding. hasEncoding is false if the font has none.
	encoding    [256]string
	hasEncoding bool

	glyphs  []type1ProgramGlyph
	names   map[string]GlyphID
	subrs   [][]byte
	lenIV   int
	weights []float64
}

// type1ProgramGlyph is a glyph: its name and its charstring, still encrypted.
type type1ProgramGlyph struct {
	name       string
	charstring []byte
}

// newType1Program reads a Type 1 font in the PFB or PFA form.
func newType1Program(data []byte) (*type1Program, error) {
	if len(data) >= 2 && data[0] == 0x80 && data[1] == 0x01 {
		var err error
		if data, err = decodePFB(data); err != nil {
			return nil, err
		}
	}
	f := &type1Program{
		matrix: kurbo.NewAffine([6]float64{0.001, 0, 0, 0.001, 0, 0}),
		lenIV:  4,
	}
	clearText, private := splitEexec(data)
	s := &type1Scanner{font: f, rd: map[string]bool{"RD": true, "-|": true}}
	s.scan(clearText)
	if private != nil {
		s.scan(private)
	}
	if len(f.glyphs) == 0 {
		return nil, fmt.Errorf("type 1 font without charstrings: %w", ivy.ErrFormat)
	}
	// Port: like FreeType, the reader moves .notdef to glyph 0, or adds an
	// empty .notdef there, so that glyph 0 is always .notdef.
	idx := -1
	for i, g := range f.glyphs {
		if g.name == ".notdef" {
			idx = i
			break
		}
	}
	switch {
	case idx > 0:
		f.glyphs[0], f.glyphs[idx] = f.glyphs[idx], f.glyphs[0]
	case idx < 0:
		f.glyphs = append([]type1ProgramGlyph{{name: ".notdef"}}, f.glyphs...)
		if len(f.glyphs) > maxType1Glyphs {
			f.glyphs = f.glyphs[:maxType1Glyphs]
		}
	}
	f.names = make(map[string]GlyphID, len(f.glyphs))
	for i, g := range f.glyphs {
		if _, ok := f.names[g.name]; !ok {
			f.names[g.name] = GlyphID(i)
		}
	}
	return f, nil
}

// decodePFB joins the segments of a PFB file.
func decodePFB(data []byte) ([]byte, error) {
	var out []byte
	for len(data) >= 2 && data[0] == 0x80 {
		kind := data[1]
		if kind == 3 {
			break
		}
		if len(data) < 6 {
			return nil, errType1
		}
		n := binary.LittleEndian.Uint32(data[2:])
		data = data[6:]
		if uint64(n) > uint64(len(data)) {
			n = uint32(len(data))
		}
		out = append(out, data[:n]...)
		data = data[n:]
	}
	if len(out) == 0 {
		return nil, errType1
	}
	return out, nil
}

// splitEexec splits a font into its clear text and its decrypted private
// part. The private part is nil if the font has no eexec.
func splitEexec(data []byte) (clearText, private []byte) {
	i := bytes.Index(data, []byte("eexec"))
	if i < 0 {
		return data, nil
	}
	clearText = data[:i]
	rest := data[i+5:]
	for len(rest) > 0 && isPSSpace(rest[0]) {
		rest = rest[1:]
	}
	if len(rest) >= 4 && isHex(rest[0]) && isHex(rest[1]) && isHex(rest[2]) && isHex(rest[3]) {
		rest = decodeHexLoose(rest)
	}
	private = decrypt(rest, eexecKey, 4)
	return clearText, private
}

// isPSSpace reports whether b is white space of PostScript.
func isPSSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\f', 0:
		return true
	}
	return false
}

// isHex reports whether b is a hexadecimal digit.
func isHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

// hexValue returns the value of a hexadecimal digit.
func hexValue(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	}
	return b - 'A' + 10
}

// decodeHexLoose decodes hexadecimal digits up to the first byte that is
// neither a digit nor white space.
func decodeHexLoose(data []byte) []byte {
	out := make([]byte, 0, len(data)/2)
	var hi byte
	half := false
	for _, b := range data {
		switch {
		case isHex(b):
			if half {
				out = append(out, hi<<4|hexValue(b))
			} else {
				hi = hexValue(b)
			}
			half = !half
		case isPSSpace(b):
		default:
			return out
		}
	}
	return out
}

// decrypt decrypts data with the key r, and drops the first skip bytes. A
// negative skip leaves the data as it is, as lenIV -1 does.
func decrypt(data []byte, r uint16, skip int) []byte {
	if skip < 0 {
		return data
	}
	out := make([]byte, len(data))
	for i, c := range data {
		out[i] = c ^ byte(r>>8)
		r = (uint16(c)+r)*52845 + 22719
	}
	if skip > len(out) {
		return nil
	}
	return out[skip:]
}

// type1Scanner reads the dictionaries of a Type 1 font with the scanner of
// PostScript. It keeps the last three tokens, which tell what a block of
// binary data after RD is.
type type1Scanner struct {
	font *type1Program
	rd   map[string]bool
	sc   *postscript.Scanner
	data []byte
	last [3]postscript.Token

	seenMatrix, seenEncoding, seenLenIV, seenWeights bool
}

// push records a token.
func (s *type1Scanner) push(t postscript.Token) {
	s.last[0], s.last[1], s.last[2] = s.last[1], s.last[2], t
}

// next reads the next token. A byte that the scanner cannot read is
// skipped. It returns false at the end of the data.
func (s *type1Scanner) next() (postscript.Token, bool) {
	for !s.sc.AtEnd() {
		off := s.sc.Offset()
		t, err := s.sc.ParseToken()
		if err == nil {
			return t, true
		}
		if s.sc.Offset() <= off {
			s.sc.SetOffset(off + 1)
		}
	}
	return nil, false
}

// scan reads one part of the font.
func (s *type1Scanner) scan(data []byte) {
	s.data = data
	s.sc = postscript.NewScanner(data)
	s.last = [3]postscript.Token{}
	for {
		t, ok := s.next()
		if !ok {
			return
		}
		if n, ok := t.(postscript.Name); ok {
			str := string(n.AsBytes())
			switch {
			case n.IsLiteral():
				s.literal(str)
			case s.rd[str]:
				s.binary()
				continue
			}
		}
		s.push(t)
	}
}

// literal handles a key of a dictionary.
func (s *type1Scanner) literal(key string) {
	switch key {
	case "FontMatrix":
		if !s.seenMatrix {
			if v := s.numbers(); len(v) == 6 {
				s.font.matrix = kurbo.NewAffine([6]float64(v))
				s.seenMatrix = true
			}
		}
	case "WeightVector":
		if !s.seenWeights {
			s.font.weights = s.numbers()
			s.seenWeights = true
		}
	case "lenIV":
		if s.seenLenIV {
			return
		}
		off := s.sc.Offset()
		if t, ok := s.next(); ok {
			if n, ok := t.(postscript.Number); ok && !n.IsReal && n.Integer >= -1 && n.Integer < 1<<16 {
				s.font.lenIV = int(n.Integer)
				s.seenLenIV = true
				return
			}
		}
		s.sc.SetOffset(off)
	case "Encoding":
		if !s.seenEncoding {
			s.encoding()
		}
	default:
		// A procedure that reads a string defines another name for RD.
		off := s.sc.Offset()
		if t, ok := s.next(); ok {
			if p, ok := t.(postscript.Procedure); ok && procedureReadsString(p) {
				s.rd[key] = true
			}
		}
		s.sc.SetOffset(off)
	}
}

// procedureReadsString reports whether a procedure calls readstring.
func procedureReadsString(p postscript.Procedure) bool {
	sc := p.Tokens()
	for i := 0; i < 64 && !sc.AtEnd(); i++ {
		t, err := sc.ParseToken()
		if err != nil {
			return false
		}
		if n, ok := t.(postscript.Name); ok {
			if str := string(n.AsBytes()); str == "readstring" {
				return true
			}
		}
	}
	return false
}

// numbers reads an array or a procedure of numbers. It returns nil if the
// next token starts neither.
func (s *type1Scanner) numbers() []float64 {
	off := s.sc.Offset()
	t, ok := s.next()
	if !ok {
		return nil
	}
	var sc *postscript.Scanner
	switch t := t.(type) {
	case postscript.Procedure:
		sc = t.Tokens()
	case postscript.Name:
		if string(t.AsBytes()) != "[" {
			s.sc.SetOffset(off)
			return nil
		}
		sc = s.sc
	default:
		s.sc.SetOffset(off)
		return nil
	}
	var out []float64
	for i := 0; i < 64 && !sc.AtEnd(); i++ {
		t, err := sc.ParseToken()
		if err != nil {
			break
		}
		n, ok := t.(postscript.Number)
		if !ok {
			break
		}
		out = append(out, n.Float64())
	}
	return out
}

// encoding reads the value of /Encoding: StandardEncoding, or an array
// that the font fills with "dup code /name put".
func (s *type1Scanner) encoding() {
	off := s.sc.Offset()
	t, ok := s.next()
	if !ok {
		return
	}
	if n, ok := t.(postscript.Name); ok {
		if str := string(n.AsBytes()); str == "StandardEncoding" && !n.IsLiteral() {
			for c := range 256 {
				s.font.encoding[c], _ = standardGet(uint8(c))
			}
			s.font.hasEncoding = true
			s.seenEncoding = true
			return
		}
	}
	if _, ok := t.(postscript.Number); !ok {
		s.sc.SetOffset(off)
		return
	}
	s.font.hasEncoding = true
	s.seenEncoding = true
	var prev [2]postscript.Token
	for {
		t, ok := s.next()
		if !ok {
			return
		}
		if n, ok := t.(postscript.Name); ok && !n.IsLiteral() {
			str := string(n.AsBytes())
			switch str {
			case "def", "readonly":
				return
			case "put":
				code, ok1 := prev[0].(postscript.Number)
				name, ok2 := prev[1].(postscript.Name)
				if ok1 && ok2 && name.IsLiteral() && !code.IsReal && code.Integer >= 0 && code.Integer < 256 {
					s.font.encoding[code.Integer] = string(name.AsBytes())
				}
			}
		}
		prev[0], prev[1] = prev[1], t
	}
}

// binary reads the block of binary data after RD. The block is a
// subroutine after "dup index length", or a charstring after "/name
// length".
func (s *type1Scanner) binary() {
	n, ok := s.last[2].(postscript.Number)
	if !ok || n.IsReal || n.Integer < 0 {
		return
	}
	start := s.sc.Offset() + 1
	end := start + int(n.Integer)
	if start > len(s.data) || end > len(s.data) || end < start {
		return
	}
	block := s.data[start:end]
	s.sc.SetOffset(end)
	if idx, ok := s.last[1].(postscript.Number); ok {
		if dup, ok := s.last[0].(postscript.Name); ok {
			if string(dup.AsBytes()) == "dup" && !idx.IsReal && idx.Integer >= 0 && idx.Integer < maxType1Subrs {
				for len(s.font.subrs) <= int(idx.Integer) {
					s.font.subrs = append(s.font.subrs, nil)
				}
				s.font.subrs[idx.Integer] = block
			}
		}
	} else if name, ok := s.last[1].(postscript.Name); ok && name.IsLiteral() && len(s.font.glyphs) < maxType1Glyphs {
		s.font.glyphs = append(s.font.glyphs, type1ProgramGlyph{name: string(name.AsBytes()), charstring: block})
	}
	s.last = [3]postscript.Token{}
}

// glyphByName returns the glyph with the name.
func (f *type1Program) glyphByName(name string) (GlyphID, bool) {
	g, ok := f.names[name]
	return g, ok
}

// encodingName returns the name of a code in the built-in encoding.
func (f *type1Program) encodingName(code uint8) (string, bool) {
	if !f.hasEncoding || f.encoding[code] == "" {
		return "", false
	}
	return f.encoding[code], true
}

// encodingGlyph returns the glyph of a code in the built-in encoding.
func (f *type1Program) encodingGlyph(code uint8) (GlyphID, bool) {
	name, ok := f.encodingName(code)
	if !ok {
		return 0, false
	}
	return f.glyphByName(name)
}

// outline draws a glyph into the pen, scaled by the font matrix to
// UnitsPerEm. A charstring with an error draws the part before the error.
func (f *type1Program) outline(gid GlyphID, pen *outlinePath) {
	if int(gid) >= len(f.glyphs) {
		return
	}
	d := &type1Decoder{
		font:   f,
		pen:    pen,
		matrix: kurbo.AffineScale(float64(UnitsPerEm)).Mul(f.matrix),
	}
	_ = d.glyph(f.glyphs[gid].charstring, 0, 0)
	if d.open {
		pen.closePath()
	}
}

// type1Decoder runs the charstrings of one glyph.
type type1Decoder struct {
	font   *type1Program
	pen    *outlinePath
	matrix kurbo.Affine

	stack   []float64
	ps      []float64
	x, y    float64
	ox, oy  float64
	open    bool
	flexing bool
	flex    []kurbo.Point
	ops     int
	seac    bool
	done    bool
}

// errType1Endchar ends the run of a charstring.
var errType1Endchar = errors.New("endchar")

// glyph runs the charstring of a glyph with its origin at ox, oy.
func (d *type1Decoder) glyph(cs []byte, ox, oy float64) error {
	d.ox, d.oy = ox, oy
	d.x, d.y = ox, oy
	d.stack = d.stack[:0]
	d.ps = d.ps[:0]
	d.flexing = false
	d.flex = d.flex[:0]
	d.done = false
	err := d.run(decrypt(cs, charstringKey, d.font.lenIV), 0)
	if errors.Is(err, errType1Endchar) {
		err = nil
	}
	return err
}

// point maps a point of the charstring to the pen.
func (d *type1Decoder) point(x, y float64) (float32, float32) {
	p := d.matrix.MulPoint(kurbo.NewPoint(x, y))
	return float32(p.X), float32(p.Y)
}

// args returns the top n operands of the stack. It returns nil if the
// stack holds fewer.
func (d *type1Decoder) args(n int) []float64 {
	if len(d.stack) < n {
		return nil
	}
	return d.stack[len(d.stack)-n:]
}

// moveTo starts a contour at the current point plus dx, dy.
func (d *type1Decoder) moveTo(dx, dy float64) {
	d.x += dx
	d.y += dy
	if d.flexing {
		if len(d.flex) < maxType1Flex {
			d.flex = append(d.flex, kurbo.NewPoint(d.x, d.y))
		}
		return
	}
	if d.open {
		d.pen.closePath()
	}
	d.pen.moveTo(d.point(d.x, d.y))
	d.open = true
}

// lineTo draws a line to the current point plus dx, dy.
func (d *type1Decoder) lineTo(dx, dy float64) {
	d.x += dx
	d.y += dy
	d.pen.lineTo(d.point(d.x, d.y))
}

// curveTo draws a curve with relative control points.
func (d *type1Decoder) curveTo(dx1, dy1, dx2, dy2, dx3, dy3 float64) {
	x1, y1 := d.x+dx1, d.y+dy1
	x2, y2 := x1+dx2, y1+dy2
	d.x, d.y = x2+dx3, y2+dy3
	ax, ay := d.point(x1, y1)
	bx, by := d.point(x2, y2)
	cx, cy := d.point(d.x, d.y)
	d.pen.curveTo(ax, ay, bx, by, cx, cy)
}

// run runs a charstring or a subroutine at a depth of callsubr.
func (d *type1Decoder) run(cs []byte, depth int) error {
	if depth > maxType1Depth {
		return errType1Limit
	}
	for i := 0; i < len(cs); {
		d.ops++
		if d.ops > maxType1Ops {
			return errType1Limit
		}
		b := int(cs[i])
		i++
		switch {
		case b >= 32:
			var v float64
			switch {
			case b <= 246:
				v = float64(b - 139)
			case b <= 250:
				if i >= len(cs) {
					return errType1
				}
				v = float64((b-247)*256 + int(cs[i]) + 108)
				i++
			case b <= 254:
				if i >= len(cs) {
					return errType1
				}
				v = float64(-(b-251)*256 - int(cs[i]) - 108)
				i++
			default:
				if i+4 > len(cs) {
					return errType1
				}
				v = float64(int32(binary.BigEndian.Uint32(cs[i:])))
				i += 4
			}
			if len(d.stack) >= maxType1Stack {
				return errType1Limit
			}
			d.stack = append(d.stack, v)
			continue
		case b == 12:
			if i >= len(cs) {
				return errType1
			}
			b = 1200 + int(cs[i])
			i++
		}
		if b == 10 {
			// callsubr
			a := d.args(1)
			if a == nil {
				return errType1
			}
			idx := int(a[0])
			d.stack = d.stack[:len(d.stack)-1]
			if idx < 0 || idx >= len(d.font.subrs) || d.font.subrs[idx] == nil {
				return errType1
			}
			if err := d.run(decrypt(d.font.subrs[idx], charstringKey, d.font.lenIV), depth+1); err != nil {
				return err
			}
			if d.done {
				return errType1Endchar
			}
			continue
		}
		if b == 11 {
			// return
			return nil
		}
		if err := d.operator(b); err != nil {
			return err
		}
	}
	return nil
}

// operator runs one operator other than a number, callsubr and return.
func (d *type1Decoder) operator(op int) error {
	clearStack := true
	defer func() {
		if clearStack {
			d.stack = d.stack[:0]
		}
	}()
	switch op {
	case 1, 3, 1200 + 0, 1200 + 1, 1200 + 2:
		// hstem, vstem, dotsection, vstem3, hstem3: hints
	case 4:
		// vmoveto
		if a := d.args(1); a != nil {
			d.moveTo(0, a[0])
		}
	case 5:
		// rlineto
		if a := d.args(2); a != nil {
			d.lineTo(a[0], a[1])
		}
	case 6:
		// hlineto
		if a := d.args(1); a != nil {
			d.lineTo(a[0], 0)
		}
	case 7:
		// vlineto
		if a := d.args(1); a != nil {
			d.lineTo(0, a[0])
		}
	case 8:
		// rrcurveto
		if a := d.args(6); a != nil {
			d.curveTo(a[0], a[1], a[2], a[3], a[4], a[5])
		}
	case 9:
		// closepath
		if d.open {
			d.pen.closePath()
			d.open = false
		}
	case 13:
		// hsbw
		if a := d.args(2); a != nil {
			d.x, d.y = d.ox+a[0], d.oy
		}
	case 14:
		// endchar
		d.done = true
		return errType1Endchar
	case 21:
		// rmoveto
		if a := d.args(2); a != nil {
			d.moveTo(a[0], a[1])
		}
	case 22:
		// hmoveto
		if a := d.args(1); a != nil {
			d.moveTo(a[0], 0)
		}
	case 30:
		// vhcurveto
		if a := d.args(4); a != nil {
			d.curveTo(0, a[0], a[1], a[2], a[3], 0)
		}
	case 31:
		// hvcurveto
		if a := d.args(4); a != nil {
			d.curveTo(a[0], 0, a[1], a[2], 0, a[3])
		}
	case 1200 + 6:
		// seac
		a := d.args(5)
		if a == nil {
			return errType1
		}
		return d.doSeac(a[0], a[1], a[2], a[3], a[4])
	case 1200 + 7:
		// sbw
		if a := d.args(4); a != nil {
			d.x, d.y = d.ox+a[0], d.oy+a[1]
		}
	case 1200 + 12:
		// div
		a := d.args(2)
		if a == nil {
			return errType1
		}
		v := 0.0
		if a[1] != 0 {
			v = a[0] / a[1]
		}
		d.stack = append(d.stack[:len(d.stack)-2], v)
		clearStack = false
	case 1200 + 16:
		clearStack = false
		return d.callOtherSubr()
	case 1200 + 17:
		// pop
		clearStack = false
		if len(d.ps) > 0 && len(d.stack) < maxType1Stack {
			d.stack = append(d.stack, d.ps[len(d.ps)-1])
			d.ps = d.ps[:len(d.ps)-1]
		}
	case 1200 + 33:
		// setcurrentpoint
		if a := d.args(2); a != nil {
			d.x, d.y = a[0], a[1]
		}
	}
	return nil
}

// doSeac draws an accented glyph from two glyphs of StandardEncoding.
func (d *type1Decoder) doSeac(asb, adx, ady, bchar, achar float64) error {
	if d.seac || bchar < 0 || bchar > 255 || achar < 0 || achar > 255 {
		return errType1
	}
	d.seac = true
	base, ok1 := standardGet(uint8(bchar))
	accent, ok2 := standardGet(uint8(achar))
	if !ok1 || !ok2 {
		return errType1
	}
	bg, ok1 := d.font.glyphByName(base)
	ag, ok2 := d.font.glyphByName(accent)
	if !ok1 || !ok2 {
		return errType1
	}
	if err := d.glyph(d.font.glyphs[bg].charstring, 0, 0); err != nil {
		return err
	}
	if d.open {
		d.pen.closePath()
		d.open = false
	}
	if err := d.glyph(d.font.glyphs[ag].charstring, adx-asb, ady); err != nil {
		return err
	}
	d.done = true
	return errType1Endchar
}

// callOtherSubr runs callothersubr: flex, the replacement of hints, and the
// blend of a multiple master font. The results of any other subroutine are
// its arguments, as the subroutines of a printer give them.
func (d *type1Decoder) callOtherSubr() error {
	a := d.args(2)
	if a == nil {
		return errType1
	}
	num, n := int(a[1]), int(a[0])
	d.stack = d.stack[:len(d.stack)-2]
	if n < 0 || n > len(d.stack) {
		return errType1
	}
	args := append([]float64(nil), d.stack[len(d.stack)-n:]...)
	d.stack = d.stack[:len(d.stack)-n]
	var results []float64
	switch num {
	case 0:
		// The end of a flex. The seven points are the reference point and
		// the points of two curves.
		results = []float64{d.x, d.y}
		if len(d.flex) >= 7 {
			p := d.flex
			for k := 1; k+2 < 7; k += 3 {
				ax, ay := d.point(p[k].X, p[k].Y)
				bx, by := d.point(p[k+1].X, p[k+1].Y)
				cx, cy := d.point(p[k+2].X, p[k+2].Y)
				d.pen.curveTo(ax, ay, bx, by, cx, cy)
			}
			results = []float64{p[6].X, p[6].Y}
		} else if len(d.flex) > 0 {
			last := d.flex[len(d.flex)-1]
			d.pen.lineTo(d.point(last.X, last.Y))
			results = []float64{last.X, last.Y}
		}
		d.flexing = false
		d.flex = d.flex[:0]
	case 1:
		// The start of a flex.
		d.flexing = true
		d.flex = d.flex[:0]
	case 2:
		// A point of a flex, which the last rmoveto added.
	case 14, 15, 16, 17, 18:
		results = d.blend(num, args)
	default:
		results = args
	}
	// pop returns the results in their order.
	for i := len(results) - 1; i >= 0; i-- {
		if len(d.ps) >= maxType1PSStack {
			return errType1Limit
		}
		d.ps = append(d.ps, results[i])
	}
	return nil
}

// blend runs the subroutines 14 to 18 of a multiple master font. Each
// result is the value of the first master plus the weighted deltas of the
// others.
func (d *type1Decoder) blend(num int, args []float64) []float64 {
	n := [...]int{1, 2, 3, 4, 6}[num-14]
	k := len(d.font.weights)
	if k == 0 || len(args) != n*k {
		return args
	}
	out := make([]float64, n)
	for i := range n {
		v := args[i]
		for j := 1; j < k; j++ {
			v += d.font.weights[j] * args[n+i*(k-1)+j-1]
		}
		out[i] = v
	}
	return out
}

// type1Names returns the names of the glyphs, for the tests.
func (f *type1Program) type1Names() string {
	names := make([]string, len(f.glyphs))
	for i, g := range f.glyphs {
		names[i] = g.name
	}
	return strings.Join(names, " ")
}
