package msppt

// The records and the properties of the shapes of Office Drawing (MS-ODRAW)
// that the package reads.
const (
	rtOfficeArtDggContainer = 0xF000
	rtOfficeArtBStore       = 0xF001
	rtOfficeArtFBSE         = 0xF007
	rtOfficeArtFOPT         = 0xF00B
	rtOfficeArtTertiaryFOPT = 0xF122
	rtOfficeArtBlipFirst    = 0xF018
	rtOfficeArtBlipLast     = 0xF117
	rtOfficeArtBlipEMF      = 0xF01A
	rtOfficeArtBlipWMF      = 0xF01B
	rtOfficeArtBlipPICT     = 0xF01C
	rtOfficeArtBlipJPEG     = 0xF01D
	rtOfficeArtBlipPNG      = 0xF01E
	rtOfficeArtBlipDIB      = 0xF01F
	rtOfficeArtBlipTIFF     = 0xF029
	rtOfficeArtBlipJPEG2    = 0xF02A
)

// The flags of an OfficeArtFSP (MS-ODRAW 2.2.40).
const (
	fspGroup      = 0x0001
	fspChild      = 0x0002
	fspPatriarch  = 0x0004
	fspDeleted    = 0x0008
	fspFlipH      = 0x0040
	fspFlipV      = 0x0080
	fspBackground = 0x0400
)

// The ids of the properties of a shape (MS-ODRAW 2.3).
const (
	propRotation           = 0x0004
	propPib                = 0x0104
	propPictureTransparent = 0x0107
	propCropTop            = 0x0100
	propCropBottom         = 0x0101
	propCropLeft           = 0x0102
	propCropRight          = 0x0103
	propTextLeft           = 0x0081
	propTextTop            = 0x0082
	propTextRight          = 0x0083
	propTextBottom         = 0x0084
	propWrapText           = 0x0085
	propAnchorText         = 0x0087
	propGeoRight           = 0x0142
	propGeoBottom          = 0x0143
	propVertices           = 0x0145
	propSegmentInfo        = 0x0146
	propAdjust1            = 0x0147
	propFillType           = 0x0180
	propFillColor          = 0x0181
	propFillOpacity        = 0x0182
	propFillBackColor      = 0x0183
	propFillBackOpac       = 0x0184
	propFillBlip           = 0x0186
	propFillAngle          = 0x018B
	propFillFocus          = 0x018C
	propFillToLeft         = 0x018D
	propFillToTop          = 0x018E
	propFillToRight        = 0x018F
	propFillToBottom       = 0x0190
	propFillShadeCols      = 0x0197
	propFillBools          = 0x01BF
	propLineColor          = 0x01C0
	propLineOpacity        = 0x01C1
	propLineWidth          = 0x01CB
	propLineDashing        = 0x01CE
	propLineStartArrow     = 0x01D0
	propLineEndArrow       = 0x01D1
	propLineBools          = 0x01FF
	propShadowColor        = 0x0201
	propShadowOffsetX      = 0x0205
	propShadowOffsetY      = 0x0206
	propShadowBools        = 0x023F
	propGroupBools         = 0x03BF
)

// The bits of the boolean properties.
const (
	fillFilled     = 0x00000010
	fillUseFilled  = 0x00100000
	lineLine       = 0x00000008
	lineUseLine    = 0x00080000
	shadowShadow   = 0x00000002
	shadowUseShdw  = 0x00020000
	groupHidden    = 0x00000002
	groupUseHidden = 0x00020000
)

// The kinds of fills of fillType (MS-ODRAW 2.4.13).
const (
	fillSolid       = 0
	fillPattern     = 1
	fillTexture     = 2
	fillPicture     = 3
	fillShade       = 4
	fillShadeCenter = 5
	fillShadeShape  = 6
	fillShadeScale  = 7
	fillShadeTitle  = 8
	fillBackground  = 9
)

// The flags of an OfficeArtCOLORREF (MS-ODRAW 2.2.2).
const (
	colorSchemeIndex = 0x08000000
	colorSysIndex    = 0x10000000
)

// props holds the properties of an OfficeArtFOPT and of an
// OfficeArtTertiaryFOPT, by id. complex holds the data of a complex
// property.
type props struct {
	vals    map[uint16]uint32
	complex map[uint16][]byte
}

// readProps adds the properties of the FOPT body b, with n properties, to
// p. A property that p holds stays.
func (p *props) readProps(b []byte, n int) {
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
			if size > uint64(len(data)) {
				size = uint64(len(data))
			}
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

// get returns the value of the property id.
func (p *props) get(id uint16) (uint32, bool) {
	v, ok := p.vals[id]
	return v, ok
}

// fixed returns a FixedPoint value of 16.16 bits.
func fixed(v uint32) float64 {
	return float64(int32(v)) / 65536
}
