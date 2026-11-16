// Copyright (c) The Hayro Authors
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

package jbig2

// This file ports hayro-jbig2 src/integration.rs, the integration with an
// image library.
//
// Port: upstream implements the trait ImageDecoder of the Rust crate image,
// and it registers a hook for the decoding of JBIG2 files with that crate.
// Go uses the package image of the standard library: a page decodes into an
// *image.Gray, and RegisterDecodingHook registers the format with
// image.RegisterFormat.

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"sync"
)

// Dimensions is ImageDecoder::dimensions for Page. It returns the width and
// the height of the page.
func (p *Page) Dimensions() (uint32, uint32) {
	return p.Width(), p.Height()
}

// ReadImage is ImageDecoder::read_image for Page. It decodes the page into
// buf as 8-bit gray samples. The buffer must hold width times height bytes.
func (p *Page) ReadImage(buf []byte) error {
	return decodeIntoBuf(p, buf)
}

// Image decodes the page into a gray image, with black as 0 and white as 255.
//
// Port: the trait ImageDecoder of upstream reports the color type L8. This
// method gives the image of that color type.
func (p *Page) Image() (*image.Gray, error) {
	pb, err := p.Decode()
	if err != nil {
		return nil, err
	}
	pix, err := pb.DataU8()
	if err != nil {
		return nil, err
	}
	w, h := int(pb.Width()), int(pb.Height())
	return &image.Gray{Pix: pix, Stride: w, Rect: image.Rect(0, 0, w, h)}, nil
}

// decodeIntoBuf is decode_into_buf in hayro-jbig2 src/integration.rs.
func decodeIntoBuf(p *Page, buf []byte) error {
	pb, err := p.Decode()
	if err != nil {
		return err
	}
	return pb.StoreU8Into(buf)
}

// Decoder is Jbig2Decoder in hayro-jbig2 src/integration.rs. It decodes the
// first page of a JBIG2 file from a stream.
type Decoder struct {
	input  []byte
	width  uint32
	height uint32
}

// NewDecoder is Jbig2Decoder::new. It reads all of r, and it parses the
// file to find the size of its first page.
func NewDecoder(r io.Reader) (*Decoder, error) {
	input, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("jbig2: reading the input: %w", err)
	}

	file, err := NewFile(input, DecodeSettings{})
	if err != nil {
		return nil, err
	}
	page, err := firstPage(file)
	if err != nil {
		return nil, err
	}

	return &Decoder{input: input, width: page.Width(), height: page.Height()}, nil
}

// Dimensions is ImageDecoder::dimensions for Jbig2Decoder.
func (d *Decoder) Dimensions() (uint32, uint32) {
	return d.width, d.height
}

// ReadImage is ImageDecoder::read_image for Jbig2Decoder. It decodes the
// first page into buf as 8-bit gray samples.
func (d *Decoder) ReadImage(buf []byte) error {
	file, err := NewFile(d.input, DecodeSettings{})
	if err != nil {
		return err
	}
	page, err := firstPage(file)
	if err != nil {
		return err
	}
	return decodeIntoBuf(page, buf)
}

// firstPage is first_page in hayro-jbig2 src/integration.rs.
func firstPage(file *File) (*Page, error) {
	page := file.Page(0)
	if page == nil {
		return nil, ErrMissingPageInfo
	}
	return page, nil
}

// jbig2Magic is JBIG2_MAGIC, the ID string of a JBIG2 file.
const jbig2Magic = "\x97JB2\r\n\x1a\n"

// registerOnce guards RegisterDecodingHook.
var registerOnce sync.Once

// RegisterDecodingHook is register_decoding_hook in hayro-jbig2
// src/integration.rs. It registers the format "jbig2" with the package
// image, so that image.Decode decodes the first page of a JBIG2 file. It
// returns false if the format is already registered.
func RegisterDecodingHook() bool {
	registered := false
	registerOnce.Do(func() {
		image.RegisterFormat("jbig2", jbig2Magic, decodeImage, decodeImageConfig)
		registered = true
	})
	return registered
}

// decodeImage is the decoder that RegisterDecodingHook registers.
func decodeImage(r io.Reader) (image.Image, error) {
	input, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("jbig2: reading the input: %w", err)
	}
	file, err := NewFile(input, DecodeSettings{})
	if err != nil {
		return nil, err
	}
	page, err := firstPage(file)
	if err != nil {
		return nil, err
	}
	return page.Image()
}

// decodeImageConfig is the configuration decoder that RegisterDecodingHook
// registers.
func decodeImageConfig(r io.Reader) (image.Config, error) {
	d, err := NewDecoder(r)
	if err != nil {
		return image.Config{}, err
	}
	return image.Config{ColorModel: color.GrayModel, Width: int(d.width), Height: int(d.height)}, nil
}
