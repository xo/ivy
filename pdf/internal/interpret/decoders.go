// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports no upstream file. It fills the hooks of JBIG2 and JPX of
// pdf/internal/syntax with the ports of hayro-jbig2 and hayro-jpeg2000, as
// the features of hayro-syntax wire those crates. The calls follow
// hayro-syntax src/filter/jbig2.rs and src/filter/jpx.rs.

package interpret

import (
	"encoding/binary"
	"fmt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/jbig2"
	"github.com/xo/ivy/internal/jpeg2000"
	"github.com/xo/ivy/pdf/internal/syntax"
)

// ImageDecoders returns the decoders of JBIG2 and JPX for
// syntax.ImageDecoders: internal/jbig2 and internal/jpeg2000, with the
// settings that hayro-syntax uses. The decoders stop when cancel returns
// true, and they take their limits from lim. A nil cancel never stops.
//
// The decoder of JPX does not apply the ICC profile of an image, as
// hayro-syntax does not. The color space of the PDF file or the number of
// channels of the profile selects the color space.
func ImageDecoders(cancel func() bool, lim ivy.Limits) syntax.ImageDecoders {
	lim = lim.WithDefaults()
	return syntax.ImageDecoders{
		JBIG2: func(data, globals []byte) (syntax.JBIG2Bitmap, error) {
			return decodeJBIG2(data, globals, cancel, lim)
		},
		JPX: func(data []byte, channels uint8, settings syntax.JPXSettings) (syntax.JPXImage, error) {
			return decodeJPX(data, channels, settings, cancel, lim)
		},
	}
}

// cancelFunc adapts a function to the cancel checks of the decoders.
type cancelFunc func() bool

// IsCancelled reports whether the decoding must stop.
func (f cancelFunc) IsCancelled() bool {
	return f != nil && f()
}

// decodeJBIG2 is the part of decode in hayro-syntax src/filter/jbig2.rs that
// calls hayro-jbig2. It returns the page as rows of packed bits, with 1 for
// black.
func decodeJBIG2(data, globalsData []byte, cancel func() bool, lim ivy.Limits) (syntax.JBIG2Bitmap, error) {
	settings := jbig2.DecodeSettings{MemoryLimit: int(min(lim.MaxMemory, int64(jbig2.DefaultMemoryLimit)))}
	if cancel != nil {
		settings.Cancel = cancelFunc(cancel)
	}
	var globals *jbig2.Globals
	if globalsData != nil {
		g, err := jbig2.NewGlobals(globalsData, settings)
		if err != nil {
			return syntax.JBIG2Bitmap{}, fmt.Errorf("reading jbig2 globals: %w", err)
		}
		globals = g
	}
	page, err := jbig2.NewEmbeddedPage(data, globals, settings)
	if err != nil {
		return syntax.JBIG2Bitmap{}, fmt.Errorf("reading jbig2 page: %w", err)
	}
	bitmap, err := page.Decode()
	if err != nil {
		return syntax.JBIG2Bitmap{}, fmt.Errorf("decoding jbig2 page: %w", err)
	}
	width, height := bitmap.Width(), bitmap.Height()
	rowBytes := (int(width) + 7) / 8
	out := make([]byte, 0, rowBytes*int(height))
	var word [4]byte
	for row := range bitmap.Rows() {
		start := len(out)
		for _, w := range row {
			binary.BigEndian.PutUint32(word[:], w)
			out = append(out, word[:]...)
			if len(out)-start >= rowBytes {
				break
			}
		}
		out = out[:min(len(out), start+rowBytes)]
		for len(out) < start+rowBytes {
			out = append(out, 0)
		}
	}
	return syntax.JBIG2Bitmap{Width: width, Height: height, Data: out}, nil
}

// decodeJPX is the part of decode in hayro-syntax src/filter/jpx.rs that
// calls hayro-jpeg2000.
func decodeJPX(data []byte, channels uint8, s syntax.JPXSettings, cancel func() bool, lim ivy.Limits) (syntax.JPXImage, error) {
	settings := jpeg2000.DecodeSettings{Limits: lim}
	if s.TargetResolution != [2]uint32{} {
		settings.TargetResolution = &jpeg2000.Resolution{Width: s.TargetResolution[0], Height: s.TargetResolution[1]}
	}
	if cancel != nil {
		settings.Cancel = jpeg2000.NewCancelHandle(cancelFunc(cancel))
	}
	var image *jpeg2000.Image
	var err error
	if channels != 0 {
		image, err = jpeg2000.FromCodestream(data, channels, &settings)
	} else {
		image, err = jpeg2000.New(data, &settings)
	}
	if err != nil {
		return syntax.JPXImage{}, fmt.Errorf("reading jpx image: %w", err)
	}
	decoded, err := image.Decode(nil)
	if err != nil {
		return syntax.JPXImage{}, fmt.Errorf("decoding jpx image: %w", err)
	}
	samples, err := decoded.DataU8()
	if err != nil {
		return syntax.JPXImage{}, fmt.Errorf("converting jpx image: %w", err)
	}
	out := syntax.JPXImage{
		Width:            image.Width(),
		Height:           image.Height(),
		OriginalBitDepth: image.OriginalBitDepth(),
		HasAlpha:         image.HasAlpha(),
		Data:             samples,
	}
	cs := image.ColorSpace()
	switch cs.Kind {
	case jpeg2000.ColorSpaceGray:
		out.ColorSpace = syntax.JPXGray
	case jpeg2000.ColorSpaceRGB:
		out.ColorSpace = syntax.JPXRGB
	case jpeg2000.ColorSpaceCMYK:
		out.ColorSpace = syntax.JPXCMYK
	case jpeg2000.ColorSpaceUnknown:
		out.ColorSpace, out.NumChannels = syntax.JPXUnknown, cs.Channels
	default:
		out.ColorSpace, out.NumChannels = syntax.JPXICC, cs.Channels
	}
	return out, nil
}
