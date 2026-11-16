// Copyright (c) The Hayro Authors.
//
// Use of this source code is governed by the Apache-2.0 or MIT license that
// can be found in the LICENSE-APACHE and LICENSE-MIT files.

// This file ports no upstream file. It gives a renderer the image of a
// stream that no page draws, such as the thumbnail of a page, which upstream
// does not read.

package interpret

import "github.com/xo/ivy/pdf/internal/syntax"

// NewRasterImage returns the image of a stream of an image that no content
// stream draws, such as the entry Thumb of a page. It decodes the image with
// the decoders and the limits of settings, and returns false when the stream
// is not an image with colors.
func NewRasterImage(stream syntax.Stream, settings InterpreterSettings) (*RasterImage, bool) {
	x, ok := newImageXObject(stream, nil, &settings, newCache(), nil)
	if !ok || x.kind != imageKindImage {
		return nil, false
	}
	return &RasterImage{imageXObject: x}, true
}
