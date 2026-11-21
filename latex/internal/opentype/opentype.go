// Package opentype gives the engine of LaTeX the part of the API of
// github.com/go-opentype/opentype that it calls, on the fonts of okage
// (D62). The engine is a copy, and its file font.go imports this package in
// place of go-opentype, so that a new commit of the engine still applies.
//
// okage gives no helper for the kerning of a pair of characters or for the
// ligature of a sequence. This package shapes the pair or the sequence with
// the shaper of okage, with only the one feature on, and reads the result.
// Item 4 of "Work for okage" in docs/BACKLOG.md asks okage for the helpers.
package opentype

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/font"
	"github.com/xo/okage/typesetting/di"
	gofont "github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// GlyphIndex is the index of a glyph in a font.
type GlyphIndex = font.GID

// Font is a parsed font program. Its methods are safe for concurrent use:
// a lock guards the face of okage, which keeps caches.
type Font struct {
	mu     sync.Mutex
	face   *gofont.Face
	shaper shaping.HarfbuzzShaper
	upem   float64
	// draw is the face that the driver draws with. It is a second parse of
	// the same data, so that drawing does not wait for the lock of face.
	draw *font.Face
	// kerns holds the kerning of each pair of characters, in font units,
	// and ligatures the glyph of each sequence, or 0 when the font has none.
	kerns     map[[2]rune]float64
	ligatures map[string][]GlyphIndex
}

// Parse reads a TrueType or OpenType font. It returns an error that wraps
// ivy.ErrFormat if data is not such a font.
func Parse(data []byte) (f *Font, err error) {
	draw, err := font.Load(data, 0)
	if err != nil {
		return nil, err
	}
	// The parser of okage can panic on a malformed font (D5).
	defer func() {
		if r := recover(); r != nil {
			f, err = nil, fmt.Errorf("reading font: %v: %w", r, ivy.ErrFormat)
		}
	}()
	face, err := gofont.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading font: %w: %w", ivy.ErrFormat, err)
	}
	upem := float64(face.Upem())
	if upem <= 0 {
		upem = 1000
	}
	return &Font{
		face:      face,
		upem:      upem,
		draw:      draw,
		kerns:     make(map[[2]rune]float64),
		ligatures: make(map[string][]GlyphIndex),
	}, nil
}

// DrawFace returns the face that draws the glyphs of the font.
func (f *Font) DrawFace() *font.Face {
	return f.draw
}

// UnitsPerEm returns the units of the em of the font.
func (f *Font) UnitsPerEm() int {
	return int(f.upem)
}

// GlyphIndex returns the glyph of r in the cmap of the font.
func (f *Font) GlyphIndex(r rune) (gid GlyphIndex, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	defer func() {
		if recover() != nil {
			gid, ok = 0, false
		}
	}()
	g, ok := f.face.NominalGlyph(r)
	if !ok || g == 0 {
		return 0, false
	}
	return GlyphIndex(g), true
}

// NewFaceAt returns the font at the size pt, in points.
func (f *Font) NewFaceAt(pt float64) *Face {
	return &Face{f: f, size: pt}
}

// advance returns the advance of the glyph in font units. The caller
// holds the lock.
func (f *Font) advance(gid GlyphIndex) (adv float64) {
	defer func() {
		if recover() != nil {
			adv = 0
		}
	}()
	return float64(f.face.HorizontalAdvance(gofont.GID(gid)))
}

// shape shapes text with the features on, and all the default features
// that change the glyphs off, at a size of one unit for each font unit.
// The caller holds the lock.
func (f *Font) shape(text []rune, features ...string) (out shaping.Output, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = shaping.Output{}, false
		}
	}()
	tags := []string{"kern", "liga", "clig", "calt", "rlig", "dlig"}
	feats := make([]shaping.FontFeature, 0, len(tags))
	for _, tag := range tags {
		var on uint32
		for _, want := range features {
			if want == tag {
				on = 1
			}
		}
		feats = append(feats, shaping.FontFeature{Tag: ot.MustNewTag(tag), Value: on})
	}
	out = f.shaper.Shape(shaping.Input{
		Text:         text,
		RunEnd:       len(text),
		Direction:    di.DirectionLTR,
		Face:         f.face,
		FontFeatures: feats,
		Size:         fixed.I(int(f.upem)),
		Script:       language.Latin,
		Language:     language.NewLanguage("en"),
	})
	return out, true
}

// Face is a font at a size.
type Face struct {
	f    *Font
	size float64
}

// Scale returns the size of one font unit in points.
func (fc *Face) Scale() float64 {
	return fc.size / fc.f.upem
}

// AdvanceIndexUnits returns the advance of the glyph, in font units.
func (fc *Face) AdvanceIndexUnits(gid GlyphIndex) float64 {
	fc.f.mu.Lock()
	defer fc.f.mu.Unlock()
	return fc.f.advance(gid)
}

// AdvanceUnits returns the advance of the glyph of r, in points, without a
// rounding. It returns 0 when the font has no glyph for r.
func (fc *Face) AdvanceUnits(r rune) float64 {
	gid, ok := fc.f.GlyphIndex(r)
	if !ok {
		return 0
	}
	return fc.AdvanceIndexUnits(gid) * fc.Scale()
}

// KernUnits returns the kerning of the pair of characters, in font units.
// It shapes the pair with the feature kern only, and returns the change of
// the advance of the pair.
func (fc *Face) KernUnits(prev, cur rune) float64 {
	f := fc.f
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [2]rune{prev, cur}
	if k, ok := f.kerns[key]; ok {
		return k
	}
	var k float64
	out, ok := f.shape([]rune{prev, cur}, "kern")
	if ok && len(out.Glyphs) == 2 {
		var nominal, shaped float64
		for _, g := range out.Glyphs {
			nominal += f.advance(GlyphIndex(g.GlyphID))
			shaped += float64(g.Advance) / 64
		}
		shaped += float64(out.Glyphs[1].XOffset) / 64
		k = shaped - nominal
	}
	f.kerns[key] = k
	return k
}

// KerningIsContextual reports whether the kerning of the font holds rules
// that a pair cannot evaluate. The pairs here are shaped, so it reports
// false.
func (fc *Face) KerningIsContextual() bool {
	return false
}

// Shape returns the glyphs of s with the feature on, such as "liga".
func (fc *Face) Shape(s, feature string) []GlyphIndex {
	f := fc.f
	f.mu.Lock()
	defer f.mu.Unlock()
	key := feature + "\x00" + s
	if gids, ok := f.ligatures[key]; ok {
		return gids
	}
	var gids []GlyphIndex
	if out, ok := f.shape([]rune(s), feature); ok {
		gids = make([]GlyphIndex, len(out.Glyphs))
		for i, g := range out.Glyphs {
			gids[i] = GlyphIndex(g.GlyphID)
		}
	}
	f.ligatures[key] = gids
	return gids
}

// Point is a point of an outline, in font units, with y up.
type Point struct {
	X, Y float64
}

// Segment is a segment of an outline. Op is 'M', 'L', 'Q' or 'C', and P
// holds its points.
type Segment struct {
	Op byte
	P  []Point
}

// GlyphOutline returns the outline of the glyph, in font units, with y up.
func (fc *Face) GlyphOutline(gid GlyphIndex) (segs []Segment, ok bool) {
	f := fc.f
	f.mu.Lock()
	defer f.mu.Unlock()
	defer func() {
		if recover() != nil {
			segs, ok = nil, false
		}
	}()
	outline, ok := f.face.GlyphDataOutline(gofont.GID(gid))
	if !ok {
		return nil, false
	}
	segs = make([]Segment, 0, len(outline.Segments))
	for _, s := range outline.Segments {
		var seg Segment
		n := 1
		switch s.Op {
		case ot.SegmentOpMoveTo:
			seg.Op = 'M'
		case ot.SegmentOpLineTo:
			seg.Op = 'L'
		case ot.SegmentOpQuadTo:
			seg.Op, n = 'Q', 2
		case ot.SegmentOpCubeTo:
			seg.Op, n = 'C', 3
		default:
			continue
		}
		for _, p := range s.Args[:n] {
			seg.P = append(seg.P, Point{X: float64(p.X), Y: float64(p.Y)})
		}
		segs = append(segs, seg)
	}
	return segs, true
}

// GlyphSVGPath returns the outline of the glyph as the data of an SVG path,
// at the size of the face, with y down from the baseline.
func (fc *Face) GlyphSVGPath(gid GlyphIndex) (string, bool) {
	segs, ok := fc.GlyphOutline(gid)
	if !ok || len(segs) == 0 {
		return "", false
	}
	scale := fc.Scale()
	var b strings.Builder
	num := func(v float64) {
		b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	}
	for _, s := range segs {
		if s.Op == 'M' && b.Len() > 0 {
			b.WriteByte('Z')
		}
		b.WriteByte(s.Op)
		for i, p := range s.P {
			if i > 0 {
				b.WriteByte(' ')
			}
			num(p.X * scale)
			b.WriteByte(' ')
			num(-p.Y * scale)
		}
	}
	b.WriteByte('Z')
	return b.String(), true
}
