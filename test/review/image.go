package review

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// thumbWidth is the width in pixels of the images of the grid of the
// report. Each one links to the full image.
const thumbWidth = 480

// writePNG writes img to path as a PNG image.
func writePNG(path string, img image.Image) error {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing the image: %w", err)
	}
	return nil
}

// writeThumb writes img to path as a JPEG image, over white, scaled down to
// thumbWidth pixels when it is wider.
func writeThumb(path string, img image.Image) error {
	flat := flatten(img)
	if w := flat.Rect.Dx(); w > thumbWidth {
		h := max(1, flat.Rect.Dy()*thumbWidth/w)
		flat = scaleTo(flat, image.Pt(thumbWidth, h))
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing the thumbnail: %w", err)
	}
	return nil
}

// readImage decodes the PNG or PNM image at path.
func readImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening the image: %w", err)
	}
	defer func() { _ = f.Close() }()
	var img image.Image
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ppm", ".pgm", ".pnm":
		img, err = decodePNM(bufio.NewReader(f))
	default:
		img, err = png.Decode(bufio.NewReader(f))
	}
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	return img, nil
}

// decodePNM decodes a binary PGM (P5) or PPM (P6) image with 8 bits for
// each sample, as ddjvu writes them.
func decodePNM(r *bufio.Reader) (image.Image, error) {
	var magic string
	var w, h, maxval int
	if err := pnmHeader(r, &magic, &w, &h, &maxval); err != nil {
		return nil, err
	}
	if maxval != 255 {
		return nil, fmt.Errorf("expected samples of 8 bits, got a maximum of %d", maxval)
	}
	if w <= 0 || h <= 0 || int64(w)*int64(h) > 1<<28 {
		return nil, fmt.Errorf("bad size %dx%d", w, h)
	}
	switch magic {
	case "P5":
		img := image.NewGray(image.Rect(0, 0, w, h))
		if _, err := io.ReadFull(r, img.Pix); err != nil {
			return nil, fmt.Errorf("reading the pixels: %w", err)
		}
		return img, nil
	case "P6":
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		row := make([]byte, 3*w)
		for y := range h {
			if _, err := io.ReadFull(r, row); err != nil {
				return nil, fmt.Errorf("reading the pixels: %w", err)
			}
			dst := img.Pix[y*img.Stride:]
			for x := range w {
				copy(dst[4*x:4*x+3], row[3*x:3*x+3])
				dst[4*x+3] = 255
			}
		}
		return img, nil
	}
	return nil, fmt.Errorf("expected a P5 or P6 image, got: %q", magic)
}

// pnmHeader reads the magic number, the width, the height and the largest
// sample of a PNM image, and the white space after them.
func pnmHeader(r *bufio.Reader, magic *string, w, h, maxval *int) error {
	tokens := make([]string, 0, 4)
	var tok []byte
	for len(tokens) < 4 {
		c, err := r.ReadByte()
		if err != nil {
			return errors.New("short header")
		}
		switch {
		case c == '#' && len(tok) == 0:
			if _, err := r.ReadString('\n'); err != nil {
				return errors.New("short header")
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if len(tok) != 0 {
				tokens = append(tokens, string(tok))
				tok = tok[:0]
			}
		default:
			if len(tok) > 16 {
				return errors.New("bad header")
			}
			tok = append(tok, c)
		}
	}
	*magic = tokens[0]
	for i, p := range []*int{w, h, maxval} {
		if _, err := fmt.Sscan(tokens[i+1], p); err != nil {
			return fmt.Errorf("bad header field %q", tokens[i+1])
		}
	}
	return nil
}

// linkOrCopy puts the file src at dst, with a hard link when it can, else
// with a copy.
func linkOrCopy(src, dst string) error {
	_ = os.Remove(dst)
	if os.Link(src, dst) == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("making %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copying %s: %w", src, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", dst, err)
	}
	return nil
}
