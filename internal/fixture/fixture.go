// Package fixture builds small files in memory for the tests of ivy: images,
// zip archives and tar archives. Only tests import it.
package fixture

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

// Image returns an image of the given size, filled with c.
func Image(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

// PNG returns a PNG image of the given size, filled with c.
func PNG(w, h int, c color.Color) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, Image(w, h, c)); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// JPEG returns a JPEG image of the given size, filled with c.
func JPEG(w, h int, c color.Color) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, Image(w, h, c), nil); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// Entry is one entry of an archive.
type Entry struct {
	Name string
	Data []byte
	// Store stores the entry of a zip archive without compression.
	Store bool
}

// Zip returns a zip archive that holds the entries, in order.
func Zip(entries ...Entry) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		method := zip.Deflate
		if e.Store {
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: e.Name, Method: method})
		if err != nil {
			panic(err)
		}
		if _, err := w.Write(e.Data); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// Tar returns a tar archive that holds the entries, in order.
func Tar(entries ...Entry) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.Name, Mode: 0o644, Size: int64(len(e.Data)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			panic(err)
		}
		if _, err := tw.Write(e.Data); err != nil {
			panic(err)
		}
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
