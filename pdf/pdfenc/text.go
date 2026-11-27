package pdfenc

import (
	"fmt"
	"io"
	"math"
	"slices"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/internal/font"
)

// glyphBytes is the number of bytes that the device counts for each glyph
// of a run that it writes as text.
const glyphBytes = 16

// maxGlyphID is the largest glyph ID of a CID font of the writer.
const maxGlyphID = 0xFFFF

// embedded is a font that the file embeds, and the glyphs that the pages
// draw with it.
type embedded struct {
	font   *pdf.Font
	key    font.SourceKey
	glyphs map[pdf.GlyphID]struct{}
}

// font returns the font of the writer for the face, or nil when its glyphs
// go into the file as paths: the face has no source, the license of the
// font forbids a subset (D58), the writer cannot read the font, or an
// earlier attempt did not make its subset. The encoder makes one font for
// each source, and keeps it for the whole file.
func (e *encoder) font(face *font.Face) *embedded {
	src, ok := face.Source()
	if !ok {
		return nil
	}
	key := src.Key()
	if emb, ok := e.fonts[key]; ok {
		return emb
	}
	if len(e.fonts) >= e.lim.MaxObjects {
		return nil
	}
	var emb *embedded
	switch {
	case e.blocked[key]:
	case !src.Embeddable():
		e.log.Debug("pdfenc: the license of the font forbids a subset, so its glyphs are paths")
	case !e.charge(int64(len(src.Data))):
	default:
		f, err := newFont(src)
		if err != nil {
			e.log.Debug("pdfenc: font drawn as paths", "err", err)
			break
		}
		emb = &embedded{font: f, key: key, glyphs: map[pdf.GlyphID]struct{}{}}
	}
	e.fonts[key] = emb
	return emb
}

// newFont returns the font of the writer for src.
func newFont(src font.Source) (f *pdf.Font, err error) {
	defer func() {
		// a font is input from a stranger (D5)
		if r := recover(); r != nil {
			f, err = nil, fmt.Errorf("reading font: %v: %w", r, ivy.ErrFormat)
		}
	}()
	var opts []pdf.FontOption
	if src.Key().HasVariations() {
		coords := make([]pdf.VariationCoord, len(src.Variations))
		for i, v := range src.Variations {
			coords[i] = pdf.VariationCoord{Tag: pdf.Tag(v.Tag), Value: v.Value}
		}
		opts = append(opts, pdf.WithVariations(coords...))
	}
	f, err = pdf.NewFont(src.Data, src.Index, opts...)
	if err != nil {
		return nil, fmt.Errorf("reading font: %w", err)
	}
	return f, nil
}

// text fills the run with the shader as text of an embedded font, which a
// reader can search and copy (D58). It returns false when the run must
// fill as paths: it has no valid text, its face cannot be embedded, or the
// writer cannot hold its glyphs or its paint. A nil shader writes the run
// as invisible text, which a reader can search and copy, and which the
// caller draws in another way, such as the outlines of -ivy-text-outline.
//
// The glyphs of the writer take a start and an advance for each glyph. The
// device pushes the transform T of the run, and puts each glyph at the
// position p of its origin through the inverse of the linear part of T, so
// that T maps the glyph onto the same place as the outline of the run.
func (d *device) text(run display.GlyphRun, shader okage.Shader) bool {
	if len(run.Glyphs) == 0 || !run.TextRanges() {
		return false
	}
	size := run.Size
	if !(size > 0) || !(float64(size) < maxCoordinate) {
		return false
	}
	for _, g := range run.Glyphs {
		if g.ID > maxGlyphID {
			return false
		}
	}
	t := run.Transform
	lin := t
	lin.TX, lin.TY = 0, 0
	toGlyphs, ok := lin.Invert()
	if !ok || !t.IsFinite() || !finiteScale(t) || !finiteScale(toGlyphs) {
		return false
	}
	inv, ok := t.Invert()
	if !ok || !finiteScale(inv) {
		return false
	}
	// pts are the origins of the glyphs in the space of the glyphs
	pts := make([]okage.Point, len(run.Glyphs))
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for i, g := range run.Glyphs {
		p := okage.Point{X: g.X, Y: g.Y}
		toGlyphs.MapPoint(&p)
		if !finitePoint(p) {
			return false
		}
		pts[i] = p
		minX, maxX = min(minX, p.X), max(maxX, p.X)
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	// the box that the glyphs can cover, with a margin of 8 ems, as the
	// okage device counts it
	pad := 8 * size
	box, ok := okage.RectFromLTRB(minX-pad, minY-pad, maxX+pad, maxY+pad)
	if !ok {
		return false
	}
	if _, ok := pageBounds(box, d.s.CTM().PreConcat(t)); !ok {
		return false
	}
	emb := d.enc.font(run.Face)
	if emb == nil {
		return false
	}
	// The writer of okage has no text rendering mode 3, so invisible text
	// fills with an opacity of 0.
	fill := pdf.Fill{Paint: pdf.ColorLumaBlack(), Opacity: pdf.NormalizedF32Zero, Rule: pdf.FillRuleNonZero}
	if shader != nil {
		paint, opacity, ok := d.paint(shader.Transform(inv))
		if !ok {
			return false
		}
		fill.Paint, fill.Opacity = paint, opacity
	}
	if !d.enc.charge(int64(len(run.Glyphs))*glyphBytes + int64(len(run.Text)) + commandBytes) {
		return true
	}
	if shader != nil && fill.Opacity == 0 {
		return true
	}
	glyphs := make([]pdf.Glyph, len(run.Glyphs))
	for i, g := range run.Glyphs {
		pg := pdf.Glyph{
			GlyphID:   pdf.GlyphID(g.ID),
			TextRange: pdf.TextRange{Start: g.Start, End: g.End},
		}
		if i+1 < len(pts) {
			pg.XAdvance = (pts[i+1].X - pts[i].X) / size
			pg.YAdvance = (pts[i].Y - pts[i+1].Y) / size
		} else {
			pg.XAdvance = run.Face.Advance(g.ID)
		}
		glyphs[i] = pg
		emb.glyphs[pg.GlyphID] = struct{}{}
	}
	baseText(glyphs, run.Text)
	pushed := !t.IsIdentity()
	if pushed {
		d.s.PushTransform(t)
	}
	d.s.SetStroke(nil)
	d.s.SetFill(&fill)
	d.s.DrawGlyphs(path.PointFromXY(pts[0].X, pts[0].Y), glyphs, emb.font, run.Text, size, false)
	d.s.SetFill(nil)
	if pushed {
		d.s.Pop()
	}
	return true
}

// failedFonts returns the key of each font whose subset the writer cannot
// make, with the glyphs that the pages drew. It makes the subset of each
// font in a file of its own.
func (e *encoder) failedFonts() map[font.SourceKey]bool {
	failed := map[font.SourceKey]bool{}
	for key, emb := range e.fonts {
		if emb == nil || len(emb.glyphs) == 0 {
			continue
		}
		if err := trySubset(emb); err != nil {
			e.log.Debug("pdfenc: no subset of the font, so its glyphs are paths", "err", err)
			failed[key] = true
		}
	}
	return failed
}

// trySubset writes a file of one page that draws the glyphs of emb, and
// returns the error of the writer.
func trySubset(emb *embedded) (err error) {
	defer func() {
		// a font is input from a stranger (D5)
		if r := recover(); r != nil {
			err = fmt.Errorf("writing the subset of a font: %v: %w", r, ivy.ErrFormat)
		}
	}()
	ids := make([]pdf.GlyphID, 0, len(emb.glyphs))
	for id := range emb.glyphs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	glyphs := make([]pdf.Glyph, len(ids))
	for i, id := range ids {
		glyphs[i] = pdf.Glyph{GlyphID: id, XAdvance: 1}
	}
	doc := pdf.NewDocument()
	page := doc.StartPage(pdf.DefaultPageSettings())
	s := page.Surface()
	s.DrawGlyphs(path.PointFromXY(0, 0), glyphs, emb.font, "", 10, false)
	s.Finish()
	page.Finish()
	if err := doc.Finish(io.Discard); err != nil {
		return fmt.Errorf("writing the subset of a font: %w", err)
	}
	return nil
}

// baseText gives the text of each cluster of several glyphs of a run from
// right to left to the widest glyph of the cluster, its base, and an empty
// range at the start of the cluster to its other glyphs.
//
// The writer of PDF gives the text of a cluster to its first glyph. In a
// run from right to left, the first glyph of a cluster is often a mark with
// no advance, such as the dots that Noto Naskh Arabic draws apart from
// their letter. A reader that places the characters by their glyphs, such
// as pdftotext, then reads a gap in the word and breaks it in two. A run
// from left to right keeps the ranges of its glyphs: its first glyph is the
// base.
func baseText(glyphs []pdf.Glyph, text string) {
	if !rightToLeft(glyphs) {
		return
	}
	for i := 0; i < len(glyphs); {
		r := glyphs[i].TextRange
		j := i + 1
		for j < len(glyphs) && glyphs[j].TextRange == r {
			j++
		}
		if j-i > 1 && r.Start < r.End {
			base := i
			for k := i + 1; k < j; k++ {
				if abs32(glyphs[k].XAdvance) > abs32(glyphs[base].XAdvance) {
					base = k
				}
			}
			splitCluster(glyphs[i:j], base-i, text[r.Start:r.End], r.Start)
		}
		i = j
	}
}

// splitCluster gives the text of a cluster to its glyphs. base is the
// index of its widest glyph and start the byte of the text of the cluster.
// When the cluster has one glyph for each character, such as an alef and
// its tanween, the base takes the first character and the other glyphs
// take the others, from the last glyph of the run to the first, which is
// their order in the text. The glyph of a letter then has the same text
// in each cluster, and the writer needs no /ActualText for it. Else the
// base takes the text, and the other glyphs take an empty range.
func splitCluster(cluster []pdf.Glyph, base int, text string, start int) {
	if utf8.RuneCountInString(text) == len(cluster) {
		_, n := utf8.DecodeRuneInString(text)
		cluster[base].TextRange = pdf.TextRange{Start: start, End: start + n}
		at := start + n
		for k := len(cluster) - 1; k >= 0; k-- {
			if k == base {
				continue
			}
			_, m := utf8.DecodeRuneInString(text[at-start:])
			cluster[k].TextRange = pdf.TextRange{Start: at, End: at + m}
			at += m
		}
		return
	}
	for k := range cluster {
		if k != base {
			cluster[k].TextRange = pdf.TextRange{Start: start, End: start}
		}
	}
}

// rightToLeft reports whether the clusters of a run go from right to left:
// the start of the text of each cluster is below the start of the cluster
// before it, and at least one cluster follows another.
func rightToLeft(glyphs []pdf.Glyph) bool {
	down := false
	for i := 1; i < len(glyphs); i++ {
		a, b := glyphs[i-1].TextRange, glyphs[i].TextRange
		switch {
		case b.Start > a.Start:
			return false
		case b.Start < a.Start:
			down = true
		}
	}
	return down
}

// abs32 returns the absolute value of v.
func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
