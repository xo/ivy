package xps

import (
	"github.com/xo/ivy/internal/font"
	"github.com/xo/okage"
)

// sidewaysTurn turns a glyph of a sideways run 90 degrees counterclockwise
// on the page, so that the top of the glyph points against the direction of
// the run (ECMA-388, the IsSideways attribute of Glyphs).
var sidewaysTurn = okage.TransformFromRow(0, -1, 1, 0, 0, 0)

// fallbackOrigin is the height of the vertical origin above the baseline, in
// ems, for a font that cannot give its vertical metrics. It is a common
// ascender.
const fallbackOrigin = 0.88

// verticalMetrics returns the vertical advance of the glyph gid of f, for a
// sideways glyph run, and the vertical origin of the glyph: half of its
// horizontal advance, and the height above the baseline. Each value is in
// ems. A font that cannot give the vertical advance advances 1 em.
func verticalMetrics(f *font.Face, gid font.GID) (advance, ox, oy float32) {
	ox = f.Advance(gid) / 2
	if advance = f.VerticalAdvance(gid); advance <= 0 {
		advance = 1
	}
	_, oy, ok := f.VerticalOrigin(gid)
	if !ok {
		oy = fallbackOrigin
	}
	return advance, ox, oy
}
