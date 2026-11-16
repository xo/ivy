// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/draw/text.py: the text and its decorations.
//
// Port: upstream writes the glyphs of the runs of Pango as text operators of
// PDF, with the widths and the mapping to Unicode of each glyph. The port
// fills the outlines of the glyphs of TextLine.GlyphRuns, at the positions
// of the shaper, so the widths and the kerning of PDF are not needed. The
// emojis of the SVG, PNG and COLR tables are not drawn: internal/font gives
// the outlines of the glyphs only, so such a glyph draws nothing.

package layout

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/tinycss2"

	"github.com/xo/ivy/internal/display"
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
// The port draws no emoji, see the comment of the file.
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
	for _, run := range firstLine.GlyphRuns() {
		if !run.HasFont || fc == nil {
			continue
		}
		face := s.dc.face(fc.face(run.Font, style))
		if face == nil {
			continue
		}
		glyphs := make([]display.Glyph, 0, len(run.Glyphs))
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
			glyphs = append(glyphs, display.Glyph{ID: g.ID, X: float32(x + g.X), Y: float32(y + g.Y)})
		}
		s.fillGlyphs(display.GlyphRun{
			Face:      face,
			Size:      float32(run.Size),
			Glyphs:    glyphs,
			Transform: okage.IdentityTransform(),
		})
	}
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
