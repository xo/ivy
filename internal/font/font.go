// Package font is the font layer of D10. It loads the fonts that a document
// embeds, maps characters to glyphs, and gives the outline and the advance of
// a glyph.
//
// On 2026-11-16 it reads TrueType, OpenType and CFF fonts with go-text (D7).
// Type 1 and Type 3, and the substitute of a font that a document names and
// does not embed, come with PDF in phase 3.
package font

import (
	"bytes"
	"fmt"
	"sync"

	gofont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"

	"github.com/xo/ivy"
	"github.com/xo/okage"
)

// GID is the index of a glyph in a font.
type GID = uint32

// Face is a font. Its methods are safe for concurrent use.
type Face struct {
	mu       sync.Mutex
	face     *gofont.Face
	upem     float32
	outlines map[GID]*okage.Path
}

// Load reads the font in data, which is TrueType, OpenType or CFF in an
// OpenType wrapper. For a collection, index selects the font. It returns an
// error that wraps ivy.ErrFormat if data is not such a font.
func Load(data []byte, index int) (*Face, error) {
	faces, err := gofont.ParseTTC(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading font: %w: %w", ivy.ErrFormat, err)
	}
	if index < 0 || index >= len(faces) {
		return nil, fmt.Errorf("font %d of a collection of %d: %w", index, len(faces), ivy.ErrFormat)
	}
	face := faces[index]
	upem := float32(face.Upem())
	if upem <= 0 {
		upem = 1000
	}
	return &Face{face: face, upem: upem, outlines: make(map[GID]*okage.Path)}, nil
}

// Glyph returns the glyph of the character r, and false if the font has none.
func (f *Face) Glyph(r rune) (GID, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	gid, ok := f.face.NominalGlyph(r)
	return GID(gid), ok
}

// Advance returns the horizontal advance of the glyph, in ems.
func (f *Face) Advance(gid GID) float32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.face.HorizontalAdvance(opentype.GID(gid)) / f.upem
}

// Outline returns the outline of the glyph, in ems, with y down, from the
// origin of the glyph on the baseline. It returns nil for a glyph without an
// outline, such as a space. The outlines are kept, and a caller must not
// change one.
func (f *Face) Outline(gid GID) *okage.Path {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.outlines[gid]; ok {
		return p
	}
	p := f.outline(gid)
	f.outlines[gid] = p
	return p
}

// outline builds the outline of the glyph.
func (f *Face) outline(gid GID) *okage.Path {
	outline, ok := f.face.GlyphData(opentype.GID(gid)).(gofont.GlyphOutline)
	if !ok || len(outline.Segments) == 0 {
		return nil
	}
	scale := 1 / f.upem
	pt := func(p opentype.SegmentPoint) (float32, float32) {
		return p.X * scale, -p.Y * scale
	}
	pb := okage.NewPathBuilder()
	for _, s := range outline.Segments {
		switch s.Op {
		case opentype.SegmentOpMoveTo:
			pb.MoveTo(pt(s.Args[0]))
		case opentype.SegmentOpLineTo:
			pb.LineTo(pt(s.Args[0]))
		case opentype.SegmentOpQuadTo:
			x1, y1 := pt(s.Args[0])
			x, y := pt(s.Args[1])
			pb.QuadTo(x1, y1, x, y)
		case opentype.SegmentOpCubeTo:
			x1, y1 := pt(s.Args[0])
			x2, y2 := pt(s.Args[1])
			x, y := pt(s.Args[2])
			pb.CubicTo(x1, y1, x2, y2, x, y)
		}
	}
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}
