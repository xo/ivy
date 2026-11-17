package interp

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strconv"
)

// maxProcDepth bounds the nesting of the procedures that the scanner reads.
const maxProcDepth = 256

// The classes of the bytes of the scanner.
const (
	clsRegular = iota
	clsSpace
	clsDelim
)

// byteClass holds the class of each byte (PLRM 3.2.2).
var byteClass = func() (c [256]uint8) {
	for _, b := range []byte{0, '\t', '\n', '\f', '\r', ' '} {
		c[b] = clsSpace
	}
	for _, b := range []byte("()<>[]{}/%") {
		c[b] = clsDelim
	}
	return c
}()

// Token reads the next object from f, as the operator token does. It
// returns false at the end of the file.
func (in *Interp) Token(f *File) (Object, bool, error) {
	return in.token(f, 0)
}

// token reads the next object at a depth of procedures.
func (in *Interp) token(f *File, depth int) (Object, bool, error) {
	b, err := skipSpace(f)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Null, false, nil
		}
		return Null, false, ioError(err)
	}
	switch b {
	case '(':
		o, err := in.scanString(f)
		return o, err == nil, err
	case ')':
		return Null, false, errSyntax
	case '<':
		c, err := f.ReadByte()
		switch {
		case err != nil:
			return Null, false, errSyntax
		case c == '<':
			return ExecName("<<"), true, nil
		case c == '~':
			o, err := in.scanASCII85(f)
			return o, err == nil, err
		}
		_ = f.UnreadByte()
		o, err := in.scanHex(f)
		return o, err == nil, err
	case '>':
		c, err := f.ReadByte()
		if err != nil || c != '>' {
			return Null, false, errSyntax
		}
		return ExecName(">>"), true, nil
	case '[', ']':
		return ExecName(string(b)), true, nil
	case '{':
		o, err := in.scanProc(f, depth+1)
		return o, err == nil, err
	case '}':
		return Null, false, errSyntax
	case '/':
		c, err := f.ReadByte()
		if err == nil && c == '/' {
			name, err := scanName(f, 0, false)
			if err != nil {
				return Null, false, err
			}
			v, ok := in.Lookup(name)
			if !ok {
				in.errCommand = NameObj(name)
				return Null, false, errUndefined
			}
			return v, true, nil
		}
		if err == nil {
			_ = f.UnreadByte()
		}
		name, err := scanName(f, 0, false)
		if err != nil {
			return Null, false, err
		}
		return NameObj(name), true, nil
	}
	if b >= 128 && b <= 159 {
		return in.scanBinary(f, b)
	}
	text, err := scanName(f, b, true)
	if err != nil {
		return Null, false, err
	}
	if o, ok := parseNumber(text); ok {
		return o, true, nil
	}
	return ExecName(text), true, nil
}

// skipSpace skips white space and comments, and returns the next byte.
func skipSpace(f *File) (byte, error) {
	for {
		b, err := f.ReadByte()
		if err != nil {
			return 0, err
		}
		switch {
		case byteClass[b] == clsSpace:
		case b == '%':
			for {
				c, err := f.ReadByte()
				if err != nil {
					return 0, err
				}
				if c == '\n' || c == '\r' || c == '\f' {
					break
				}
			}
		default:
			return b, nil
		}
	}
}

// scanName reads the bytes of a name or a number, up to a delimiter or white
// space. first is the first byte, when has is true.
func scanName(f *File, first byte, has bool) (string, error) {
	var buf [64]byte
	text := buf[:0]
	if has {
		text = append(text, first)
	}
	for {
		b, ok := nextByte(f)
		if !ok {
			break
		}
		if byteClass[b] != clsRegular {
			// a white space after the token is consumed, a delimiter is not
			// (PLRM 3.2.2)
			if byteClass[b] == clsDelim {
				_ = f.UnreadByte()
			}
			break
		}
		if len(text) >= 1<<16 {
			return "", errLimitcheck
		}
		text = append(text, b)
	}
	return string(text), nil
}

// parseNumber parses an integer, a real or a radix number.
func parseNumber(s string) (Object, bool) {
	if s == "" {
		return Null, false
	}
	c := s[0]
	if (c < '0' || c > '9') && c != '+' && c != '-' && c != '.' {
		return Null, false
	}
	if i := indexByte(s, '#'); i > 0 {
		base, err := strconv.Atoi(s[:i])
		if err != nil || base < 2 || base > 36 || i+1 >= len(s) {
			return Null, false
		}
		v, err := strconv.ParseUint(s[i+1:], base, 64)
		if err != nil || hasSign(s[i+1:]) {
			return Null, false
		}
		return Int(int(int32(uint32(v)))), true
	}
	digits, dot, exp := 0, 0, 0
	for i := 0; i < len(s); i++ {
		switch b := s[i]; {
		case b >= '0' && b <= '9':
			if exp == 0 {
				digits++
			}
		case b == '+' || b == '-':
			if i != 0 && s[i-1] != 'e' && s[i-1] != 'E' {
				return Null, false
			}
		case b == '.':
			if dot > 0 || exp > 0 {
				return Null, false
			}
			dot++
		case b == 'e' || b == 'E':
			if exp > 0 || digits == 0 {
				return Null, false
			}
			exp++
		default:
			return Null, false
		}
	}
	if digits == 0 {
		return Null, false
	}
	if dot == 0 && exp == 0 {
		v, err := strconv.ParseInt(s, 10, 64)
		if err == nil && v >= math.MinInt32 && v <= math.MaxInt32 {
			return Int(int(v)), true
		}
	}
	text := s
	if text[len(text)-1] == 'e' || text[len(text)-1] == 'E' {
		return Null, false
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
			return Real(v), true
		}
		return Null, false
	}
	return Real(v), true
}

// hasSign reports whether s starts with a sign.
func hasSign(s string) bool {
	return s != "" && (s[0] == '+' || s[0] == '-')
}

// indexByte returns the index of the first c in s, or -1.
func indexByte(s string, c byte) int {
	for i := range len(s) {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// scanString reads a string after its open parenthesis.
func (in *Interp) scanString(f *File) (Object, error) {
	var text []byte
	depth := 1
	for {
		b, err := f.ReadByte()
		if err != nil {
			return Null, errSyntax
		}
		if len(text) >= in.maxString {
			return Null, errLimitcheck
		}
		switch b {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return in.NewString(text)
			}
		case '\r':
			if c, err := f.ReadByte(); err == nil && c != '\n' {
				_ = f.UnreadByte()
			}
			b = '\n'
		case '\\':
			c, err := f.ReadByte()
			if err != nil {
				return Null, errSyntax
			}
			switch c {
			case 'n':
				b = '\n'
			case 'r':
				b = '\r'
			case 't':
				b = '\t'
			case 'b':
				b = '\b'
			case 'f':
				b = '\f'
			case '\n':
				continue
			case '\r':
				if c, err := f.ReadByte(); err == nil && c != '\n' {
					_ = f.UnreadByte()
				}
				continue
			case '0', '1', '2', '3', '4', '5', '6', '7':
				v := int(c - '0')
				for range 2 {
					d, err := f.ReadByte()
					if err != nil {
						break
					}
					if d < '0' || d > '7' {
						_ = f.UnreadByte()
						break
					}
					v = v*8 + int(d-'0')
				}
				b = byte(v)
			default:
				b = c
			}
		}
		text = append(text, b)
	}
}

// hexValue returns the value of a hexadecimal digit, or -1.
func hexValue(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	}
	return -1
}

// scanHex reads a hexadecimal string after its open angle bracket.
func (in *Interp) scanHex(f *File) (Object, error) {
	var text []byte
	half, hi := false, 0
	for {
		b, err := f.ReadByte()
		if err != nil {
			return Null, errSyntax
		}
		if b == '>' {
			if half {
				text = append(text, byte(hi<<4))
			}
			return in.NewString(text)
		}
		if byteClass[b] == clsSpace {
			continue
		}
		v := hexValue(b)
		if v < 0 {
			return Null, errSyntax
		}
		if len(text) >= in.maxString {
			return Null, errLimitcheck
		}
		if half {
			text = append(text, byte(hi<<4|v))
		} else {
			hi = v
		}
		half = !half
	}
}

// scanASCII85 reads an ASCII base-85 string after its <~.
func (in *Interp) scanASCII85(f *File) (Object, error) {
	var text []byte
	d := a85Decoder{}
	for {
		b, err := f.ReadByte()
		if err != nil {
			return Null, errSyntax
		}
		if b == '~' {
			c, err := f.ReadByte()
			if err != nil || c != '>' {
				return Null, errSyntax
			}
			out, ok := d.finish(text)
			if !ok {
				return Null, errSyntax
			}
			return in.NewString(out)
		}
		if len(text) >= in.maxString {
			return Null, errLimitcheck
		}
		var ok bool
		if text, ok = d.add(text, b); !ok {
			return Null, errSyntax
		}
	}
}

// a85Decoder decodes ASCII base-85, one byte at a time.
type a85Decoder struct {
	group [5]byte
	n     int
}

// add adds one byte of text, and appends the bytes that it completes to
// out. It returns false for a byte that is not of the encoding.
func (d *a85Decoder) add(out []byte, b byte) ([]byte, bool) {
	switch {
	case byteClass[b] == clsSpace:
		return out, true
	case b == 'z' && d.n == 0:
		return append(out, 0, 0, 0, 0), true
	case b < '!' || b > 'u':
		return out, false
	}
	d.group[d.n] = b - '!'
	d.n++
	if d.n == 5 {
		var v uint64
		for _, c := range d.group {
			v = v*85 + uint64(c)
		}
		if v > math.MaxUint32 {
			return out, false
		}
		out = binary.BigEndian.AppendUint32(out, uint32(v))
		d.n = 0
	}
	return out, true
}

// finish appends the bytes of the last partial group.
func (d *a85Decoder) finish(out []byte) ([]byte, bool) {
	if d.n == 0 {
		return out, true
	}
	if d.n == 1 {
		return out, false
	}
	var v uint64
	for i := range 5 {
		c := byte(84)
		if i < d.n {
			c = d.group[i]
		}
		v = v*85 + uint64(c)
	}
	if v > math.MaxUint32 {
		return out, false
	}
	var four [4]byte
	binary.BigEndian.PutUint32(four[:], uint32(v))
	d.n, out = 0, append(out, four[:d.n-1]...)
	return out, true
}

// scanProc reads a procedure after its open brace.
func (in *Interp) scanProc(f *File, depth int) (Object, error) {
	if depth > maxProcDepth {
		return Null, errLimitcheck
	}
	var elems []Object
	for {
		b, err := skipSpace(f)
		if err != nil {
			return Null, errSyntax
		}
		if b == '}' {
			o, err := in.NewArray(elems)
			if err != nil {
				return Null, err
			}
			o.exec = true
			return o, nil
		}
		_ = f.UnreadByte()
		o, ok, err := in.token(f, depth)
		if err != nil {
			return Null, err
		}
		if !ok {
			return Null, errSyntax
		}
		if len(elems) >= in.maxArray {
			return Null, errLimitcheck
		}
		elems = append(elems, o)
	}
}

// readN reads n bytes.
func readN(f *File, n int) ([]byte, error) {
	b := make([]byte, n)
	for i := range b {
		c, err := f.ReadByte()
		if err != nil {
			return nil, errSyntax
		}
		b[i] = c
	}
	return b, nil
}

// scanBinary reads a binary token (PLRM 3.14.1). It reads the numbers, the
// booleans, the strings and the arrays of numbers. A binary object sequence
// and an encoded name are a syntax error.
func (in *Interp) scanBinary(f *File, b byte) (Object, bool, error) {
	switch b {
	case 132, 133:
		p, err := readN(f, 4)
		if err != nil {
			return Null, false, err
		}
		if b == 132 {
			return Int(int(int32(binary.BigEndian.Uint32(p)))), true, nil
		}
		return Int(int(int32(binary.LittleEndian.Uint32(p)))), true, nil
	case 134, 135:
		p, err := readN(f, 2)
		if err != nil {
			return Null, false, err
		}
		if b == 134 {
			return Int(int(int16(binary.BigEndian.Uint16(p)))), true, nil
		}
		return Int(int(int16(binary.LittleEndian.Uint16(p)))), true, nil
	case 136:
		p, err := readN(f, 1)
		if err != nil {
			return Null, false, err
		}
		return Int(int(int8(p[0]))), true, nil
	case 137:
		p, err := readN(f, 1)
		if err != nil {
			return Null, false, err
		}
		o, err := readFixed(f, p[0])
		return o, err == nil, err
	case 138, 139, 140:
		p, err := readN(f, 4)
		if err != nil {
			return Null, false, err
		}
		v := binary.LittleEndian.Uint32(p)
		if b == 138 {
			v = binary.BigEndian.Uint32(p)
		}
		return Real(float64(math.Float32frombits(v))), true, nil
	case 141:
		p, err := readN(f, 1)
		if err != nil {
			return Null, false, err
		}
		return Bool(p[0] != 0), true, nil
	case 142, 143, 144:
		var n int
		switch b {
		case 142:
			p, err := readN(f, 1)
			if err != nil {
				return Null, false, err
			}
			n = int(p[0])
		default:
			p, err := readN(f, 2)
			if err != nil {
				return Null, false, err
			}
			n = int(binary.BigEndian.Uint16(p))
			if b == 144 {
				n = int(binary.LittleEndian.Uint16(p))
			}
		}
		p, err := readN(f, n)
		if err != nil {
			return Null, false, err
		}
		o, err := in.NewString(p)
		return o, err == nil, err
	case 149:
		p, err := readN(f, 3)
		if err != nil {
			return Null, false, err
		}
		rep := p[0]
		n := int(binary.BigEndian.Uint16(p[1:]))
		if rep >= 128 {
			n = int(binary.LittleEndian.Uint16(p[1:]))
		}
		elems := make([]Object, 0, min(n, 1024))
		for range n {
			o, err := readFixed(f, rep)
			if err != nil {
				return Null, false, err
			}
			elems = append(elems, o)
		}
		o, err := in.NewArray(elems)
		return o, err == nil, err
	}
	return Null, false, errSyntax
}

// readFixed reads one number of the representation rep of a binary token.
func readFixed(f *File, rep byte) (Object, error) {
	le := rep >= 128
	r := rep & 127
	switch {
	case r <= 31:
		p, err := readN(f, 4)
		if err != nil {
			return Null, err
		}
		v := binary.BigEndian.Uint32(p)
		if le {
			v = binary.LittleEndian.Uint32(p)
		}
		if r == 0 {
			return Int(int(int32(v))), nil
		}
		return Real(float64(int32(v)) / float64(uint32(1)<<r)), nil
	case r <= 47:
		p, err := readN(f, 2)
		if err != nil {
			return Null, err
		}
		v := binary.BigEndian.Uint16(p)
		if le {
			v = binary.LittleEndian.Uint16(p)
		}
		if r == 32 {
			return Int(int(int16(v))), nil
		}
		return Real(float64(int16(v)) / float64(uint32(1)<<(r-32))), nil
	case r == 48 || r == 49:
		p, err := readN(f, 4)
		if err != nil {
			return Null, err
		}
		v := binary.BigEndian.Uint32(p)
		if le {
			v = binary.LittleEndian.Uint32(p)
		}
		return Real(float64(math.Float32frombits(v))), nil
	}
	return Null, errSyntax
}
