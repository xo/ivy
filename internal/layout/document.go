// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports part of weasyprint/document.py: Page.paint, which draws a
// page box. The rest of the module writes PDF, which ivy does not write.

package layout

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/xo/okage"
	gofont "github.com/xo/okage/typesetting/font"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	ivyfont "github.com/xo/ivy/internal/font"
)

// DrawOptions are the options of DrawPage. The zero DrawOptions draws with
// antialiasing and smooth images.
type DrawOptions struct {
	// Aliased draws without antialiasing: a pixel is covered or not.
	Aliased bool
	// Sharp draws the images without interpolation, whatever their
	// image-rendering.
	Sharp bool
	// FontConfig gives the fonts of the document to its SVG images. It can
	// be nil.
	FontConfig *FontConfiguration
	// Limits bound the work of the drawing. A zero field takes its default.
	Limits ivy.Limits
	// Logger receives the log of the drawing, or nil.
	Logger *slog.Logger
}

// DrawPage is Page.paint in weasyprint/document.py, with the transforms of
// gather_anchors: it draws a page box of Document.Page onto dev. toDevice
// maps the CSS pixels of the page, from the top left corner of its margin
// box, onto the user space of dev.
//
// The page box is not safe for concurrent use: the drawing sets the
// transforms of its boxes, and reads their styles. DrawPage returns the
// error of ctx when ctx stops the drawing, and an *ivy.PartialError when it
// drew the page in part.
func DrawPage(ctx context.Context, dev display.Device, page *Box, toDevice okage.Transform, opts DrawOptions) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("drawing page: %w", err)
	}
	limits := opts.Limits.WithDefaults()
	dc := &drawContext{
		ctx:           ctx,
		pageRectangle: page.BleedArea(),
		antiAlias:     !opts.Aliased,
		interpolate:   !opts.Sharp,
		log:           newLogger(opts.Logger),
		limits:        limits,
		faces:         map[*gofont.Face]*ivyfont.Face{},
		maxWork:       maxDrawWork * limits.MaxObjects,
	}
	base := Matrix{
		A: float64(toDevice.SX), B: float64(toDevice.KY),
		C: float64(toDevice.KX), D: float64(toDevice.SY),
		E: float64(toDevice.TX), F: float64(toDevice.TY),
	}
	dc.deviceArea = deviceBox(base, dc.pageRectangle)
	if opts.FontConfig != nil {
		dc.fonts = opts.FontConfig.FontDatabase()
	}
	defer func() {
		// Port: a panic of the drawing is a page drawn in part.
		if r := recover(); r != nil {
			err = &ivy.PartialError{Err: fmt.Errorf("drawing page: panic: %v: %w", r, ivy.ErrFormat)}
		}
	}()

	gatherAnchors(page, 0)
	s := newStream(dc, base)
	s.stacked(func() {
		drawPage(s, page)
	})
	dev.Save()
	dev.Concat(toDevice)
	s.replay(dev)
	dev.Restore()

	switch {
	case dc.err == nil:
		return nil
	case ctx.Err() != nil:
		return fmt.Errorf("drawing page: %w", ctx.Err())
	}
	return &ivy.PartialError{Err: dc.err}
}

// deviceBox returns the box on the device of the rectangle r, (x, y, width,
// height), that m maps to the device, rounded out to whole pixels.
func deviceBox(m Matrix, r [4]float64) [4]float64 {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]float64{{r[0], r[1]}, {r[0] + r[2], r[1]}, {r[0], r[1] + r[3]}, {r[0] + r[2], r[1] + r[3]}} {
		x, y := m.TransformPoint(corner[0], corner[1])
		minX, minY = min(minX, x), min(minY, y)
		maxX, maxY = max(maxX, x), max(maxY, y)
	}
	minX, minY = math.Floor(minX), math.Floor(minY)
	return [4]float64{minX, minY, math.Ceil(maxX) - minX, math.Ceil(maxY) - minY}
}
