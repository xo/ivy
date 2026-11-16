// Copied from github.com/oov/psd, file compress.go, at commit a5d50ec0acac
// (D7, docs/UPSTREAM.md). Its MIT license is in LICENSE.

// ivy: the package is named oovpsd, after the folder of the copy.
package oovpsd

import (
	"compress/zlib"
	"errors"
	"image"
	"io"
)

// CompressionMethod represents compression method that is used in psd file.
type CompressionMethod int16

// These compression methods are defined in this document.
//
// http://www.adobe.com/devnet-apps/photoshop/fileformatashtml/#50577409_pgfId-1054855
const (
	CompressionMethodRaw                  = CompressionMethod(0)
	CompressionMethodRLE                  = CompressionMethod(1)
	CompressionMethodZIPWithoutPrediction = CompressionMethod(2)
	CompressionMethodZIPWithPrediction    = CompressionMethod(3)
)

var (
	errBrokenPackBits = errors.New("psd: compressed image data seems broken")
)

// Decode decodes the compressed image data from r.
//
// You can pass 0 to sizeHint if unknown, but in this case may read more data than necessary from r.
func (cm CompressionMethod) Decode(dest []byte, r io.Reader, sizeHint int64, rect image.Rectangle, depth int, channels int, large bool) (read int, err error) {
	switch cm {
	case CompressionMethodRaw:
		return io.ReadFull(r, dest)
	case CompressionMethodRLE:
		// ivy: width is the number of bytes of one line. Upstream counts a
		// line of 16 or 32 bits as rect.Dx() bytes, so the goroutines of
		// decodePackBits split the lines of such an image at the wrong
		// offsets, and the decode fails as broken data.
		width := (rect.Dx()*depth + 7) >> 3
		return decodePackBits(dest, r, width, rect.Dy()*channels, large)
	case CompressionMethodZIPWithoutPrediction:
		return decodeZLIB(dest, r, sizeHint)
	case CompressionMethodZIPWithPrediction:
		if read, err = decodeZLIB(dest, r, sizeHint); err != nil {
			return read, err
		}
		decodeDelta(dest, rect.Dx(), depth)
		return read, err
	}
	return 0, errors.New("psd: compression method " + itoa(int(cm)) + " is not implemented")
}

// decodeZLIB decodes compressed data by zlib.
// You can pass 0 to length if unknown,
// but in that case may read more data than necessary from r.
func decodeZLIB(dest []byte, r io.Reader, length int64) (read int, err error) {
	N := length
	if N == 0 {
		N = 0x7fffffffffffffff
	}
	lr := &io.LimitedReader{R: r, N: N}
	zr, err := zlib.NewReader(lr)
	if err != nil {
		return 0, err
	}
	if _, err = io.ReadFull(zr, dest); err != nil {
		zr.Close()
		return int(N - lr.N), err
	}
	if err = zr.Close(); err != nil {
		return int(N - lr.N), err
	}
	return int(N - lr.N), nil
}

func decodeDelta(buf []byte, width int, depth int) {
	switch depth {
	case 16:
		var d uint16
		for i := 0; i < len(buf); {
			d = 0
			for j := 0; j < width; j++ {
				d += readUint16(buf, i)
				writeUint16(buf, d, i)
				i += 2
			}
		}
	case 32:
		var d uint32
		for i := 0; i < len(buf); {
			d = 0
			for j := 0; j < width; j++ {
				d += readUint32(buf, i)
				writeUint32(buf, d, i)
				i += 4
			}
		}
	}
}
