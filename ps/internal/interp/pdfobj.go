package interp

import (
	"bytes"
	"math"
	"strconv"
)

// maxPDFDepth bounds the nesting that ToPDF writes.
const maxPDFDepth = 32

// ToPDF writes an object in the syntax of PDF, so that the port of hayro can
// read a dictionary of PostScript, such as the parameters of a filter or a
// shading. A procedure becomes the braces of a function of type 4. An
// object that PDF lacks becomes null.
func ToPDF(o Object) []byte {
	var b bytes.Buffer
	writePDF(&b, o, 0)
	return b.Bytes()
}

// writePDF writes one object.
func writePDF(b *bytes.Buffer, o Object, depth int) {
	if depth > maxPDFDepth {
		b.WriteString("null")
		return
	}
	switch o.typ {
	case TInt:
		b.WriteString(strconv.FormatInt(o.i, 10))
	case TReal:
		r := o.r
		if math.IsNaN(r) || math.IsInf(r, 0) {
			r = 0
		}
		b.WriteString(strconv.FormatFloat(r, 'f', -1, 64))
	case TBool:
		if o.i != 0 {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case TName:
		if o.exec {
			// an operator of a function of type 4
			b.WriteString(o.NameVal())
			return
		}
		writePDFName(b, o.NameVal())
	case TOperator:
		b.WriteString(o.Op().Name)
	case TString:
		b.WriteByte('<')
		const digits = "0123456789abcdef"
		for _, c := range o.Bytes() {
			b.WriteByte(digits[c>>4])
			b.WriteByte(digits[c&15])
		}
		b.WriteByte('>')
	case TArray, TPacked:
		open, end := byte('['), byte(']')
		if o.exec {
			open, end = '{', '}'
		}
		b.WriteByte(open)
		for i, e := range o.Array() {
			if i > 0 {
				b.WriteByte(' ')
			}
			writePDF(b, e, depth+1)
		}
		b.WriteByte(end)
	case TDict:
		b.WriteString("<<")
		o.Dict().Each(func(k, v Object) bool {
			if k.typ != TName {
				return true
			}
			writePDFName(b, k.NameVal())
			b.WriteByte(' ')
			writePDF(b, v, depth+1)
			b.WriteByte(' ')
			return true
		})
		b.WriteString(">>")
	default:
		b.WriteString("null")
	}
}

// writePDFName writes a name, with the escapes of PDF.
func writePDFName(b *bytes.Buffer, s string) {
	b.WriteByte('/')
	for i := range len(s) {
		c := s[i]
		if c <= ' ' || c >= 127 || c == '#' || byteClass[c] != clsRegular {
			b.WriteByte('#')
			b.WriteString(strconv.FormatInt(int64(c)|0x100, 16)[1:])
			continue
		}
		b.WriteByte(c)
	}
}
