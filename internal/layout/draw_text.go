// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/draw/text.py: the text and its decorations.
//
// Port: upstream writes the glyphs of the runs of Pango as text operators of
// PDF, with the widths and the mapping to Unicode of each glyph. The port
// fills the outlines of the glyphs of TextLine.GlyphRuns, at the positions
// of the shaper, so the widths and the kerning of PDF are not needed. A
// glyph of the tables COLR and SVG, such as a color emoji, draws as the
// tree that okage gives for it (okage D57). A glyph without an outline that
// has a bitmap, such as an emoji of the tables CBDT and sbix, draws its
// bitmap.

package layout

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"
	"github.com/xo/okage/tinycss2"

	"github.com/xo/ivy"

	"github.com/xo/ivy/internal/display"
	ivyfont "github.com/xo/ivy/internal/font"
)

// maxEllipsisLoops bounds the words that block-ellipsis removes from a
// line.
//
// Port: upstream removes the last word until the text fits, without a
// bound. Each loop shapes the text again.
const maxEllipsisLoops = 1 << 10

// drawText is draw_text in weasyprint/draw/text.py: it draws a text box.
// blockEllipsis is nil for 'none'.
func drawText(s *stream, textbox *Box, offsetX float64, textOverflow string, blockEllipsis Value) {
	style := textbox.Style
	// Do not draw the invisible text boxes.
	if !isStr(style.Get("visibility"), "visible") || textbox.TextLayout == nil {
		return
	}
	layout := textbox.TextLayout

	// Draw the underline and the overline.
	textDecorationValues, _ := style.Get("text_decoration_line").(StringSet)
	textDecorationColor := getColor(style, "text_decoration_color")
	var thickness float64
	if textDecorationValues.Has("underline") || textDecorationValues.Has("overline") {
		if isStrIn(style.Get("text_decoration_thickness"), "auto", "from-font") {
			thickness = layout.UnderlineThickness
		} else {
			thickness, _ = percentageFloat(style.Get("text_decoration_thickness"), style, num(style.Get("font_size")))
		}
	}
	if textDecorationValues.Has("overline") {
		offsetY := textbox.Baseline - layout.Ascent + thickness/2
		drawTextDecoration(s, textbox, offsetX, offsetY, thickness, textDecorationColor)
	}
	if textDecorationValues.Has("underline") {
		var underlineOffset float64
		if isStr(style.Get("text_underline_offset"), "auto") {
			underlineOffset = -layout.UnderlinePosition
		} else {
			underlineOffset, _ = percentageFloat(style.Get("text_underline_offset"), style, num(style.Get("font_size")))
		}
		offsetY := textbox.Baseline + underlineOffset + thickness/2
		drawTextDecoration(s, textbox, offsetX, offsetY, thickness, textDecorationColor)
	}

	// Draw the text.
	x, y := textbox.PositionX, textbox.PositionY+textbox.Baseline
	s.setColor(getColor(style, "color"), false)
	layout.Reactivate(style)
	drawFirstLine(s, textbox, textOverflow, blockEllipsis, x, y)
	if textbox.isTab {
		// Port: upstream has no tab stops.
		drawTabLeader(s, textbox, y)
	}

	// Draw the line through.
	if textDecorationValues.Has("line-through") {
		thickness := layout.StrikethroughThickness
		offsetY := textbox.Baseline - layout.StrikethroughPosition
		drawTextDecoration(s, textbox, offsetX, offsetY, thickness, textDecorationColor)
	}
	layout.Deactivate()
}

// isNone reports whether a value of block-ellipsis is 'none'.
func isNone(v Value) bool {
	return v == nil || isStr(v, "none")
}

// drawFirstLine is draw_first_line in weasyprint/draw/text.py: it draws the
// first line of a text box, whose baseline starts at (x, y).
//
// Port: upstream returns the emojis of the line, which draw_emojis draws.
// The port draws the color glyphs and the bitmaps of the glyphs of the
// line, see the comment of the file.
func drawFirstLine(s *stream, textbox *Box, textOverflow string, blockEllipsis Value, x, y float64) {
	layout := textbox.TextLayout
	style := textbox.Style

	// Do not draw the lines with only invisible characters.
	if strings.TrimSpace(textbox.Text) == "" {
		return
	}

	// The minimum font size that Pango handles.
	if num(style.Get("font_size")) < 1.0/pangoScale {
		return
	}

	// Port: pango_layout_set_single_paragraph_mode has no equivalent. The
	// text of a box holds no paragraph separator before its end.
	var ellipsis string
	if textOverflow == "ellipsis" || !isNone(blockEllipsis) {
		// Port: upstream asserts that the layout has a maximum width.
		if layout.MaxWidth != nil {
			maxWidth := *layout.MaxWidth
			layout.setWidth(maxWidth)
			if textOverflow == "ellipsis" {
				ellipsizeEnd(layout, maxWidth)
			} else {
				if isStr(blockEllipsis, "auto") {
					ellipsis = "…"
				} else if t, ok := blockEllipsis.(Tuple); ok && len(t) == 2 {
					// Port: upstream asserts that the value is a string.
					ellipsis = str(t[1])
				}

				// Remove the last word if hyphenated.
				newText := layout.Text
				hyphenateCharacter := str(style.Get("hyphenate_character"))
				if hyphenateCharacter != "" && strings.HasSuffix(newText, hyphenateCharacter) {
					lastWordEnd, ok := GetLastWordEnd(strings.TrimSuffix(newText, hyphenateCharacter), str(style.Get("lang")))
					if ok && lastWordEnd != 0 {
						newText = string([]rune(newText)[:lastWordEnd])
					}
				}

				layout.SetText(newText+ellipsis, true)
			}
		}
	}

	firstLine, _, hasNext := layout.FirstLine()

	if !isNone(blockEllipsis) && layout.MaxWidth != nil {
		for range maxEllipsisLoops {
			if !hasNext {
				break
			}
			text := strings.TrimSuffix(layout.Text, ellipsis)
			lastWordEnd, ok := GetLastWordEnd(text, str(style.Get("lang")))
			if !ok {
				break
			}
			newText := string([]rune(layout.Text)[:lastWordEnd])
			layout.SetText(newText+ellipsis, true)
			firstLine, _, hasNext = layout.FirstLine()
		}
	}

	fc := layout.fc
	// The color of the text, for the palette entry of a color glyph that
	// takes it. The alpha of the color is the opacity of the glyph.
	fill := s.state.fill
	fg := color.NRGBA{R: colorByte(fill.r), G: colorByte(fill.g), B: colorByte(fill.b), A: 0xff}
	variations := fontVariations(style)
	for _, run := range firstLine.GlyphRuns() {
		if !run.HasFont || fc == nil {
			continue
		}
		face := s.dc.face(fc.face(run.Font, style), fc.db, run.Font, variations)
		if face == nil {
			continue
		}
		// lo and hi are the bytes of the text that the clusters of the
		// glyphs cover.
		lo, hi := len(layout.Text), 0
		glyphs := make([]display.Glyph, 0, len(run.Glyphs))
		var bitmaps []glyphBitmap
		var colors []glyphTree
		for _, g := range run.Glyphs {
			r, _ := utf8.DecodeRuneInString(layout.Text[min(g.Cluster, len(layout.Text)):])
			if unicode.IsControl(r) {
				// Port: Pango draws a control character, such as a tab, as
				// the empty glyph PANGO_GLYPH_EMPTY. The shaper gives the
				// glyph of the font, often .notdef.
				continue
			}
			if g.ID == 0 {
				// Display .notdef and log a warning for the missing glyphs.
				if !isInvisible(r) {
					s.dc.log.warning(".notdef glyph rendered for Unicode string unsupported by fonts: \"%c\" (U+%04X)",
						r, r)
				}
			}
			// A color glyph of COLR or SVG comes first, then a bitmap, then
			// the outline, as the renderer of okage draws a glyph.
			if tree := fc.colorGlyph(run.Font, g.ID, variations, fg); tree != nil {
				colors = append(colors, glyphTree{tree, x + g.X, y + g.Y})
				continue
			}
			if face.Outline(g.ID) == nil {
				// Port: upstream draws the bitmap glyphs, such as the
				// color emojis of CBDT and sbix, with Cairo.
				if b, ok := face.Bitmap(g.ID); ok {
					bitmaps = append(bitmaps, glyphBitmap{b, x + g.X, y + g.Y})
					continue
				}
			}
			start, end := min(g.Cluster, len(layout.Text)), min(g.ClusterEnd, len(layout.Text))
			if start < end {
				lo, hi = min(lo, start), max(hi, end)
			}
			glyphs = append(glyphs, display.Glyph{ID: g.ID, X: float32(x + g.X), Y: float32(y + g.Y), Start: start, End: end})
		}
		for _, c := range colors {
			if err := s.drawColorGlyph(c.tree, c.x, c.y, run.Size); err != nil {
				s.dc.log.warning("Failed to draw a color glyph: %v", err)
			}
		}
		for _, b := range bitmaps {
			s.drawGlyphBitmap(b.bitmap, b.x, b.y, run.Size)
		}
		glyphRun := display.GlyphRun{
			Face:      face,
			Size:      float32(run.Size),
			Glyphs:    glyphs,
			Transform: okage.IdentityTransform(),
		}
		// Port: upstream gives Cairo the text and the clusters of the run of
		// Pango, and Cairo writes them into the PDF file. The run takes the
		// text that its clusters cover, for a writer of PDF (D58).
		setRunText(&glyphRun, layout.Text, lo, hi)
		if fc.syntheticBold(run.Font, style) {
			// Port: upstream has no synthetic bold, see syntheticBold.
			// The stroke grows the outlines by a 48th of an em on each
			// side, as FreeType emboldens a glyph.
			s.fillBoldGlyphs(glyphRun, run.Size/24)
			continue
		}
		s.fillGlyphs(glyphRun)
	}
}

// setRunText gives run the bytes lo to hi of text, and makes the range of
// each glyph a range of them. A glyph with a range outside lo to hi gets
// the empty range at lo. The run gets no text when lo is not before hi.
func setRunText(run *display.GlyphRun, text string, lo, hi int) {
	if lo >= hi {
		for i := range run.Glyphs {
			run.Glyphs[i].Start, run.Glyphs[i].End = 0, 0
		}
		return
	}
	run.Text = text[lo:hi]
	for i := range run.Glyphs {
		g := &run.Glyphs[i]
		if g.Start < lo || g.End > hi || g.Start > g.End {
			g.Start, g.End = lo, lo
		}
		g.Start -= lo
		g.End -= lo
	}
}

// glyphBitmap is the bitmap of a glyph whose origin is at (x, y).
type glyphBitmap struct {
	bitmap *ivyfont.Bitmap
	x, y   float64
}

// glyphTree is the tree of a color glyph whose origin is at (x, y).
type glyphTree struct {
	tree *svg.Tree
	x, y float64
}

// colorByte returns the byte of a channel of a color from 0 to 1.
func colorByte(v float64) uint8 {
	return uint8(math.Round(clamp01(v) * 0xff))
}

// drawColorGlyph draws the tree of a color glyph, which is in em units, with
// its origin at (x, y) and at the font size size. okage renders the tree
// into pixels at the scale of the device, as for an SVG image, and the
// stream draws the pixels with the alpha of the filling.
//
// Port: upstream draws the color glyphs with Cairo.
func (s *stream) drawColorGlyph(tree *svg.Tree, x, y, size float64) (err error) {
	if s.state.fill.alpha <= 0 || !(size > 0) {
		return nil
	}
	defer func() {
		// Port: a panic in the renderer of okage is a glyph that cannot be
		// drawn.
		if r := recover(); r != nil {
			err = fmt.Errorf("rendering color glyph: panic: %v: %w", r, ivy.ErrFormat)
		}
	}()
	bbox, ok := tree.Root().AbsLayerBoundingBox()
	if !ok {
		return nil
	}
	toUser, ok := s.absolute().Invert()
	if !ok {
		return nil
	}
	toDevice := Matrix{A: size, D: size, E: x, F: y}.Mul(s.absolute())
	// The box of the glyph on the device, inside the area of the stream.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, corner := range [4][2]float32{
		{bbox.Left(), bbox.Top()}, {bbox.Right(), bbox.Top()},
		{bbox.Left(), bbox.Bottom()}, {bbox.Right(), bbox.Bottom()},
	} {
		px, py := toDevice.TransformPoint(float64(corner[0]), float64(corner[1]))
		minX, minY = min(minX, px), min(minY, py)
		maxX, maxY = max(maxX, px), max(maxY, py)
	}
	area := s.area
	minX, minY = math.Floor(max(minX, area[0])), math.Floor(max(minY, area[1]))
	maxX, maxY = math.Ceil(min(maxX, area[0]+area[2])), math.Ceil(min(maxY, area[1]+area[3]))
	if !(maxX > minX) || !(maxY > minY) {
		return nil
	}
	w, h, scale := boundCell(maxX-minX, maxY-minY)
	toPixels := translation(-minX, -minY).Mul(Matrix{A: scale, D: scale})
	fromPixels, ok := toPixels.Invert()
	if !ok {
		return nil
	}
	pixmap, ok := okage.NewPixmap(uint32(w), uint32(h))
	if !ok {
		return nil
	}
	pm := pixmap.AsMut()
	svg.Render(tree, toDevice.Mul(toPixels).transform(), &pm)
	img := &image.RGBA{Pix: pixmap.Take(), Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	// The pixels map to the device, then to the current user space.
	s.drawImageMatrix(img, fromPixels.Mul(toUser), true)
	return nil
}

// isInvisible reports whether r draws nothing, such as a space or a control
// character, for which a missing glyph is not worth a warning.
func isInvisible(r rune) bool {
	return r <= ' ' || r == 0xA0 || (r >= 0x2000 && r <= 0x200F) || r == 0xFEFF
}

// ellipsizeEnd stands for pango_layout_set_ellipsize with
// PANGO_ELLIPSIZE_END: when the text is wider than maxWidth, it keeps the
// longest start of the text that fits with "…" after it.
//
// Port: Pango removes the clusters of the end of the line, and draws the
// ellipsis with the font of the text. The port cuts the text at a character,
// with a binary search on the width of the line.
func ellipsizeEnd(layout *TextLayout, maxWidth float64) {
	text := layout.Text
	layout.setWidth(-1)
	line, _, _ := layout.FirstLine()
	if line.Width <= maxWidth {
		layout.setWidth(maxWidth)
		return
	}
	runes := []rune(text)
	fits := func(n int) bool {
		layout.SetText(string(runes[:n])+"…", true)
		line, _, _ := layout.FirstLine()
		return line.Width <= maxWidth
	}
	low, high := 0, len(runes)
	for low < high {
		mid := (low + high + 1) / 2
		if fits(mid) {
			low = mid
		} else {
			high = mid - 1
		}
	}
	layout.SetText(string(runes[:low])+"…", true)
}

// drawTextDecoration is draw_text_decoration in weasyprint/draw/text.py: it
// draws a text decoration of a text box.
func drawTextDecoration(s *stream, textbox *Box, offsetX, offsetY, thickness float64, color *tinycss2.Color) {
	c := []*tinycss2.Color{color}
	drawLine(s, textbox.PositionX, textbox.PositionY+offsetY,
		textbox.PositionX+textbox.Width, textbox.PositionY+offsetY,
		thickness, str(textbox.Style.Get("text_decoration_style")), c, offsetX)
}
