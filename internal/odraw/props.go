package odraw

// The ids of the properties of a shape that the package reads (MS-ODRAW
// 2.3).
const (
	PropRotation           = 0x0004
	PropTextLeft           = 0x0081
	PropTextTop            = 0x0082
	PropTextRight          = 0x0083
	PropTextBottom         = 0x0084
	PropWrapText           = 0x0085
	PropAnchorText         = 0x0087
	PropCropTop            = 0x0100
	PropCropBottom         = 0x0101
	PropCropLeft           = 0x0102
	PropCropRight          = 0x0103
	PropPib                = 0x0104
	PropPictureTransparent = 0x0107
	PropGeoLeft            = 0x0140
	PropGeoTop             = 0x0141
	PropGeoRight           = 0x0142
	PropGeoBottom          = 0x0143
	PropVertices           = 0x0145
	PropSegmentInfo        = 0x0146
	PropAdjust1            = 0x0147
	PropFillType           = 0x0180
	PropFillColor          = 0x0181
	PropFillOpacity        = 0x0182
	PropFillBackColor      = 0x0183
	PropFillBackOpacity    = 0x0184
	PropFillCrMod          = 0x0185
	PropFillBlip           = 0x0186
	PropFillAngle          = 0x018B
	PropFillFocus          = 0x018C
	PropFillToLeft         = 0x018D
	PropFillToTop          = 0x018E
	PropFillToRight        = 0x018F
	PropFillToBottom       = 0x0190
	PropFillShadeColors    = 0x0197
	PropFillShadeType      = 0x019C
	PropFillBools          = 0x01BF
	PropLineColor          = 0x01C0
	PropLineOpacity        = 0x01C1
	PropLineBackColor      = 0x01C2
	PropLineWidth          = 0x01CB
	PropLineDashing        = 0x01CE
	PropLineStartArrow     = 0x01D0
	PropLineEndArrow       = 0x01D1
	PropLineStartWidth     = 0x01D2
	PropLineStartLength    = 0x01D3
	PropLineEndWidth       = 0x01D4
	PropLineEndLength      = 0x01D5
	PropLineBools          = 0x01FF
	PropShadowColor        = 0x0201
	PropShadowOpacity      = 0x0204
	PropShadowOffsetX      = 0x0205
	PropShadowOffsetY      = 0x0206
	PropShadowBools        = 0x023F
	PropMaster             = 0x0301
	PropGroupBools         = 0x03BF
)

// The bits of the boolean properties.
const (
	FillFilled     = 0x00000010
	FillUseFilled  = 0x00100000
	LineLine       = 0x00000008
	LineUseLine    = 0x00080000
	ShadowShadow   = 0x00000002
	ShadowUseShdw  = 0x00020000
	GroupHidden    = 0x00000002
	GroupUseHidden = 0x00020000
)

// The kinds of fills of fillType (MS-ODRAW 2.4.13).
const (
	FillSolid       = 0
	FillPattern     = 1
	FillTexture     = 2
	FillPicture     = 3
	FillShade       = 4
	FillShadeCenter = 5
	FillShadeShape  = 6
	FillShadeScale  = 7
	FillShadeTitle  = 8
	FillBackground  = 9
)

// Props holds the properties of an OfficeArtFOPT and of an
// OfficeArtTertiaryFOPT, by id, with the data of the complex properties.
type Props struct {
	vals    map[uint16]uint32
	complex map[uint16][]byte
}

// Read adds the properties of the FOPT body b, with n properties, to p. A
// property that p holds stays.
func (p *Props) Read(b []byte, n int) {
	if p.vals == nil {
		p.vals = make(map[uint16]uint32)
	}
	n = min(n, len(b)/6)
	data := b[6*n:]
	for i := range n {
		opid := le.Uint16(b[6*i:])
		op := le.Uint32(b[6*i+2:])
		id := opid & 0x3FFF
		var cdata []byte
		if opid&0x8000 != 0 {
			// the data of a complex property follow the table, in order
			size := uint64(op)
			if arrayWithoutHeader(data, op) {
				size += 6
			}
			size = min(size, uint64(len(data)))
			cdata, data = data[:size], data[size:]
		}
		if _, ok := p.vals[id]; ok {
			continue
		}
		p.vals[id] = op
		if cdata != nil {
			if p.complex == nil {
				p.complex = make(map[uint16][]byte)
			}
			p.complex[id] = cdata
		}
	}
}

// Set sets the property id to v, for a format that gives a property in a
// record of its own.
func (p *Props) Set(id uint16, v uint32) {
	if p.vals == nil {
		p.vals = make(map[uint16]uint32)
	}
	p.vals[id] = v
}

// arrayWithoutHeader reports whether data starts with an IMsoArray whose
// size op leaves out its header of 6 bytes, as some writers do for the
// vertices and the segments of a path.
func arrayWithoutHeader(data []byte, op uint32) bool {
	if len(data) < 6 || uint64(op)+6 > uint64(len(data)) {
		return false
	}
	n, cb := uint64(le.Uint16(data)), uint64(le.Uint16(data[4:]))
	sizes := []uint64{cb}
	if cb == 0xFFF0 {
		sizes = []uint64{2, 4}
	}
	for _, size := range sizes {
		if n > 0 && n*size == uint64(op) {
			return true
		}
	}
	return false
}

// Get returns the value of the property id.
func (p *Props) Get(id uint16) (uint32, bool) {
	v, ok := p.vals[id]
	return v, ok
}

// Complex returns the data of the complex property id, or nil.
func (p *Props) Complex(id uint16) []byte {
	return p.complex[id]
}

// Fixed returns a FixedPoint value of 16.16 bits.
func Fixed(v uint32) float64 {
	return float64(int32(v)) / 65536
}

// Array returns the elements of an IMsoArray (MS-ODRAW 2.2.51). size is
// the size of an element when the array says 0xFFF0.
func Array(b []byte, size int) [][]byte {
	if len(b) < 6 {
		return nil
	}
	n := int(le.Uint16(b))
	cb := int(le.Uint16(b[4:]))
	if cb == 0xFFF0 {
		cb = size
	}
	if cb <= 0 || cb > 64 {
		return nil
	}
	n = min(n, (len(b)-6)/cb)
	out := make([][]byte, n)
	for i := range n {
		out[i] = b[6+cb*i : 6+cb*(i+1)]
	}
	return out
}

// Chain is the properties of a shape, then the properties that it
// inherits, such as those of a shape of a master. The first set that holds
// a property gives it.
type Chain []*Props

// Get returns the value of the property id.
func (c Chain) Get(id uint16) (uint32, bool) {
	for _, p := range c {
		if v, ok := p.Get(id); ok {
			return v, true
		}
	}
	return 0, false
}

// Value returns the value of the property id, or def.
func (c Chain) Value(id uint16, def uint32) uint32 {
	if v, ok := c.Get(id); ok {
		return v
	}
	return def
}

// Complex returns the data of the complex property id, from the set that
// gives the property.
func (c Chain) Complex(id uint16) []byte {
	for _, p := range c {
		if _, ok := p.Get(id); ok {
			return p.Complex(id)
		}
	}
	return nil
}

// Flag returns a bit of a boolean property: from the first set of
// properties that sets its bit of use, or def.
func (c Chain) Flag(id uint16, bit, use uint32, def bool) bool {
	for _, p := range c {
		if v, ok := p.Get(id); ok && v&use != 0 {
			return v&bit != 0
		}
	}
	return def
}
