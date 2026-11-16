package msole

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/xo/magic/cfb"
	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/gowemf"
	"github.com/xo/ivy/internal/metafile"
	"github.com/xo/ivy/internal/raster"
)

// The sizes of the image of a metafile, in pixels on its longest side. A
// metafile has no size in pixels, so the package draws it at 96 dots per
// inch, within these bounds.
const (
	minSide = 256
	maxSide = 1024
)

// The mapping modes of a METAFILEPICT whose extent is a size in units of
// 0.01 mm (MS-WMF 2.1.1.16).
const (
	mmIsotropic   = 7
	mmAnisotropic = 8
)

var (
	pngSignature  = []byte("\x89PNG\r\n\x1a\n")
	jpegSignature = []byte{0xff, 0xd8, 0xff}
	gifSignature  = []byte("GIF8")
)

// ThumbnailPreview returns the preview of the thumbnail cd of a summary
// (MS-OLEPS 2.11). It reads a metafile of CF_METAFILEPICT, an enhanced
// metafile of CF_ENHMETAFILE, a bitmap of CF_DIB, and a PNG, JPEG or GIF
// image in any format of the clipboard. It returns an error that wraps
// ivy.ErrUnsupported for another format.
func ThumbnailPreview(cd cfb.ClipboardData, cfg ivy.Config) (ivy.Preview, error) {
	data := cd.Data
	switch {
	case cd.Tag > 0:
		// the name of the format comes first
		data = data[min(int(cd.Tag), len(data)):]
	case cd.Tag == -3:
		data = data[min(16, len(data)):]
	}
	if isRaster(data) {
		return RasterPreview(data, cfg)
	}
	if cd.Tag == -1 {
		switch cd.Format {
		case cfb.CFMetafilePict:
			return metafilePict(data, cfg)
		case cfb.CFEnhMetafile:
			return MetafilePreview(data, cfg)
		case cfb.CFDIB:
			return DIBPreview(data, cfg)
		}
	}
	if isMetafile(data) {
		return MetafilePreview(data, cfg)
	}
	return ivy.Preview{}, fmt.Errorf("thumbnail with the tag %d and the format %d: %w", cd.Tag, cd.Format, ivy.ErrUnsupported)
}

// isRaster reports whether data starts with the signature of PNG, JPEG or
// GIF.
func isRaster(data []byte) bool {
	return bytes.HasPrefix(data, pngSignature) || bytes.HasPrefix(data, jpegSignature) || bytes.HasPrefix(data, gifSignature)
}

// isMetafile reports whether data starts with the header of an EMF file, or
// with the placeable header of a WMF file.
func isMetafile(data []byte) bool {
	switch {
	case len(data) >= 44 && binary.LittleEndian.Uint32(data) == 1 && string(data[40:44]) == " EMF":
		return true
	case len(data) >= 4 && binary.LittleEndian.Uint32(data) == 0x9ac6cdd7:
		return true
	}
	return false
}

// metafilePict returns the preview of a METAFILEPICT of 16 bits and the WMF
// file after it. The extent of the header gives the shape of the picture
// in the isotropic and anisotropic modes.
func metafilePict(data []byte, cfg ivy.Config) (ivy.Preview, error) {
	if len(data) < 8 {
		return ivy.Preview{}, fmt.Errorf("metafilepict of %d bytes: %w", len(data), ivy.ErrFormat)
	}
	mm := int16(binary.LittleEndian.Uint16(data))
	w := math.Abs(float64(int16(binary.LittleEndian.Uint16(data[2:]))))
	h := math.Abs(float64(int16(binary.LittleEndian.Uint16(data[4:]))))
	pic, err := metafile.Open(data[8:], metafile.Options{Config: cfg})
	if err != nil {
		return ivy.Preview{}, err
	}
	pw, ph := pic.Size()
	width, height := float64(pw), float64(ph)
	if (mm == mmIsotropic || mm == mmAnisotropic) && w > 0 && h > 0 {
		// 0.01 mm to points
		width, height = w*72/2540, h*72/2540
	}
	return picturePreview(pic, width, height, cfg)
}

// MetafilePreview returns the preview of the WMF, EMF or EMF+ file in data.
// Its image is the picture at 96 dots per inch, made larger or smaller to
// fit between 256 and 1024 pixels on its longest side, over white.
func MetafilePreview(data []byte, cfg ivy.Config) (ivy.Preview, error) {
	pic, err := metafile.Open(data, metafile.Options{Config: cfg})
	if err != nil {
		return ivy.Preview{}, err
	}
	w, h := pic.Size()
	return picturePreview(pic, float64(w), float64(h), cfg)
}

// picturePreview returns the preview of pic, drawn into a rectangle of
// width by height points.
func picturePreview(pic *metafile.Picture, width, height float64, cfg ivy.Config) (ivy.Preview, error) {
	lim := cfg.Limits.WithDefaults()
	size, ok := imageSize(width, height)
	if !ok {
		return ivy.Preview{}, fmt.Errorf("metafile of %gx%g points: %w", width, height, ivy.ErrFormat)
	}
	if err := raster.CheckPixels(size, lim); err != nil {
		return ivy.Preview{}, err
	}
	log := cfg.Log()
	decode := func(ctx context.Context) (image.Image, error) {
		pm, ok := okage.NewPixmap(uint32(size.X), uint32(size.Y))
		if !ok {
			return nil, fmt.Errorf("image of %v: %w", size, ivy.ErrLimit)
		}
		pm.Fill(okage.ColorFromRGBA8(255, 255, 255, 255))
		pw, ph := pic.Size()
		r, ok := okage.RectFromXYWH(0, 0, pw, ph)
		if !ok {
			return nil, fmt.Errorf("metafile of %gx%g points: %w", pw, ph, ivy.ErrFormat)
		}
		t := okage.TransformFromScale(float32(size.X)/pw, float32(size.Y)/ph)
		err := pic.Draw(ctx, display.NewOkage(pm, t), r)
		if _, partial := errors.AsType[*ivy.PartialError](err); partial {
			// a thumbnail drawn in part still shows the document
			log.Debug("msole: thumbnail drawn in part", "err", err)
			err = nil
		}
		if err != nil {
			return nil, err
		}
		return pm, nil
	}
	return ivy.Preview{Kind: ivy.Thumbnail, Size: size, Decode: decode}, nil
}

// imageSize returns the size in pixels of the image of a picture of width
// by height points.
func imageSize(width, height float64) (image.Point, bool) {
	long := max(width, height)
	if !(width > 0 && height > 0) || math.IsInf(long, 0) {
		return image.Point{}, false
	}
	scale := 96.0 / 72
	switch {
	case long*scale > maxSide:
		scale = maxSide / long
	case long*scale < minSide:
		scale = minSide / long
	}
	w := max(1, int(math.Ceil(width*scale-0.01)))
	h := max(1, int(math.Ceil(height*scale-0.01)))
	return image.Pt(w, h), true
}

// DIBPreview returns the preview of a device-independent bitmap: a
// BITMAPINFO and its pixels, with no BITMAPFILEHEADER (MS-WMF 2.2.2.9).
func DIBPreview(data []byte, cfg ivy.Config) (ivy.Preview, error) {
	lim := cfg.Limits.WithDefaults()
	limits := gowemf.ImageLimits{
		MaxBytes:        uint64(lim.MaxMemory),
		MaxPixels:       uint64(lim.MaxPixels),
		MaxDecodedBytes: uint64(lim.MaxMemory),
	}
	dib, err := gowemf.ParsePackedDIB(data, 0, nil, limits)
	if err != nil {
		return ivy.Preview{}, dibError(err)
	}
	size := image.Pt(dib.Width(), dib.Height())
	if err := raster.CheckPixels(size, lim); err != nil {
		return ivy.Preview{}, err
	}
	decode := func(ctx context.Context) (img image.Image, err error) {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("decoding the bitmap: %w", err)
		}
		defer func() {
			if r := recover(); r != nil {
				img, err = nil, fmt.Errorf("decoding the bitmap: panic: %v: %w", r, ivy.ErrFormat)
			}
		}()
		img, err = dib.Image()
		if err != nil {
			return nil, dibError(err)
		}
		return img, nil
	}
	return ivy.Preview{Kind: ivy.Thumbnail, Size: size, Decode: decode}, nil
}

// dibError wraps an error of gowemf in the error of ivy that matches it.
func dibError(err error) error {
	switch {
	case errors.Is(err, gowemf.ErrLimit):
		return fmt.Errorf("reading the bitmap: %w: %w", ivy.ErrLimit, err)
	case errors.Is(err, gowemf.ErrUnsupported):
		return fmt.Errorf("reading the bitmap: %w: %w", ivy.ErrUnsupported, err)
	}
	return fmt.Errorf("reading the bitmap: %w: %w", ivy.ErrFormat, err)
}

// RasterPreview returns the preview of the PNG, JPEG or GIF image in data.
func RasterPreview(data []byte, cfg ivy.Config) (ivy.Preview, error) {
	size, err := raster.Config(data)
	if err != nil {
		return ivy.Preview{}, err
	}
	lim := cfg.Limits.WithDefaults()
	return ivy.Preview{
		Kind: ivy.Thumbnail,
		Size: size,
		Decode: func(ctx context.Context) (image.Image, error) {
			return raster.Decode(ctx, data, lim)
		},
	}, nil
}
