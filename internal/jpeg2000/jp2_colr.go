// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jpeg2000

// This file ports hayro-jpeg2000 src/jp2/colr.rs.

import "bytes"

// colrParse is parse in hayro-jpeg2000 src/jp2/colr.rs. It reads the
// colour specification box. The first box wins.
func colrParse(boxes *imageBoxes, data []byte) error {
	if boxes.colorSpecification != nil {
		return nil
	}

	reader := newBitReader(data)
	b, ok := reader.readBytes(3)
	if !ok {
		return errInvalidBox
	}
	meth := b[0]

	var cs colrColorSpace
	switch meth {
	case 1:
		value, ok := reader.readU32()
		if !ok {
			return errInvalidBox
		}
		e, ok := enumeratedColorspaceFromRaw(value, &reader)
		if !ok {
			return errInvalidBox
		}
		cs = colrColorSpace{kind: colrEnumerated, enumerated: e}
	case 2:
		tail, ok := reader.tail()
		if !ok {
			return errInvalidBox
		}
		cs = colrColorSpace{kind: colrICC, icc: bytes.Clone(tail)}
	default:
		cs = colrColorSpace{kind: colrUnknown}
	}

	boxes.colorSpecification = &colorSpecificationBox{colorSpace: cs}
	return nil
}

// colorSpecificationBox is ColorSpecificationBox in hayro-jpeg2000
// src/jp2/colr.rs.
type colorSpecificationBox struct {
	colorSpace colrColorSpace
}

// colrColorSpace is ColorSpace in hayro-jpeg2000 src/jp2/colr.rs. It takes
// the name of its module, because the package has a ColorSpace of its own.
//
// Port: the Rust enum becomes a struct. kind names the case, enumerated
// holds the data of Enumerated, and icc holds the data of Icc.
type colrColorSpace struct {
	kind       colrColorSpaceKind
	enumerated enumeratedColorspace
	icc        []byte
}

// colrColorSpaceKind is the case of colrColorSpace.
type colrColorSpaceKind uint8

// The cases of colrColorSpace.
const (
	colrUnknown colrColorSpaceKind = iota
	colrEnumerated
	colrICC
)

// enumeratedColorspace is EnumeratedColorspace in hayro-jpeg2000
// src/jp2/colr.rs.
//
// Port: the Rust enum becomes a struct. kind names the case, and cieLab
// holds the data of CieLab.
type enumeratedColorspace struct {
	kind   enumeratedKind
	cieLab cieLab
}

// enumeratedKind is the case of enumeratedColorspace.
type enumeratedKind uint8

// The cases of EnumeratedColorspace. Each name takes the prefix enum,
// because the package has one namespace.
const (
	enumBiLevel1 enumeratedKind = iota
	enumYCbCr1
	enumYCbCr2
	enumYCbCr3
	enumPhotoYcc
	enumCmy
	enumCmyk
	enumYcck
	enumCieLab
	enumBiLevel2
	enumSrgb
	enumGreyscale
	enumSycc
	enumCieJab
	enumEsRgb
	enumRommRgb
	enumYPbPr112560
	enumYPbPr125050
	enumEsYcc
	enumScRgb
	enumScRgbGray
)

// The cases that j2cParse sets, without the prefix, for readability.
const (
	srgb      = enumSrgb
	greyscale = enumGreyscale
	sycc      = enumSycc
)

// cieLab is CieLab in hayro-jpeg2000 src/jp2/colr.rs. Each field holds the
// value and whether the box holds it.
type cieLab struct {
	rl, ol, ra, oa, rb, ob optU32
}

// optU32 is an Option<u32> of Rust.
type optU32 struct {
	v  uint32
	ok bool
}

// unwrapOr is Option::unwrap_or.
func (o optU32) unwrapOr(d uint32) uint32 {
	if o.ok {
		return o.v
	}
	return d
}

// enumeratedColorspaceFromRaw is EnumeratedColorspace::from_raw.
func enumeratedColorspaceFromRaw(value uint32, reader *bitReader) (enumeratedColorspace, bool) {
	var k enumeratedKind
	switch value {
	case 0:
		k = enumBiLevel1
	case 1:
		k = enumYCbCr1
	case 3:
		k = enumYCbCr2
	case 4:
		k = enumYCbCr3
	case 9:
		k = enumPhotoYcc
	case 11:
		k = enumCmy
	case 12:
		k = enumCmyk
	case 13:
		k = enumYcck
	case 14:
		read := func() optU32 {
			v, ok := reader.readU32()
			return optU32{v, ok}
		}
		var lab cieLab
		lab.rl = read()
		lab.ol = read()
		lab.ra = read()
		lab.oa = read()
		lab.rb = read()
		lab.ob = read()
		_ = read() // il
		return enumeratedColorspace{kind: enumCieLab, cieLab: lab}, true
	case 15:
		k = enumBiLevel2
	case 16:
		k = enumSrgb
	case 17:
		k = enumGreyscale
	case 18:
		k = enumSycc
	case 19:
		k = enumCieJab
	case 20:
		k = enumEsRgb
	case 21:
		k = enumRommRgb
	case 22:
		k = enumYPbPr112560
	case 23:
		k = enumYPbPr125050
	case 24:
		k = enumEsYcc
	case 25:
		k = enumScRgb
	case 26:
		k = enumScRgbGray
	default:
		return enumeratedColorspace{}, false
	}
	return enumeratedColorspace{kind: k}, true
}
