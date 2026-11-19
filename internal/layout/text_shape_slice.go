// This file is not a port. It lets a text take the glyphs of a longer text
// that the layout shaped before, so that the layout shapes most texts of a
// line once (okage D70).
//
// Port: Pango shapes the whole text of a layout. WeasyPrint lays out the
// start of a text to find its first line, then the first line again, then
// the first line with the next word, and the next line starts a new
// layout. So the port shaped each character about three times. The port
// keeps the last long text that it shaped for each font of a style, as a
// parent. A later text that is the start or the end of the parent takes
// the glyphs of the parent, when HarfBuzz marks each cut as safe to break
// and the segmenter gives the text the runs of the parent. The glyphs are
// the glyphs that the shaper gives for the text.

package layout

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// The bounds of the parents.
const (
	// minParentBytes is the length in bytes of the shortest text that
	// becomes a parent. A shorter text, such as the text of a strut, does
	// not take the place of a longer parent.
	minParentBytes = 32
	// maxParentRunes is the number of runes of the longest parent.
	maxParentRunes = maxScratch
	// maxParents bounds the parents of a configuration.
	maxParents = 256
)

// parentKey is the key of a parent: the font of a style, the value of
// font-kerning, which the font of a style does not hold, and the direction.
type parentKey struct {
	font    string
	kerning string
	rtl     bool
}

// shapedParent is a text of one paragraph that a configuration shaped,
// with the runs that the segmenter gave for it.
type shapedParent struct {
	text string
	s    *shapedText
	// runs are the runs of the segmenter, one for each run of s.
	runs []parentRun
}

// parentRun is a run that the segmenter gave for a parent: its runes, its
// face, its script and its direction.
type parentRun struct {
	start, end int32
	face       *font.Face
	script     language.Script
	dir        di.Direction
	// rev is true when the glyphs of the run go from the last cluster to
	// the first, as in a run from right to left.
	rev bool
}

// newShapedParent returns a shaped text of one paragraph as a parent, with
// the runs of the segmenter. It returns nil when the text cannot be a
// parent:
//
//  1. The text is too short or too long.
//  2. The glyphs of a run are not in the order of their clusters.
//  3. The text has more than one run, and one of them has a script that
//     joins its letters, such as Arabic. The shaper of such a script reads
//     the characters around its run, which a part of the text does not
//     have.
func newShapedParent(text string, s *shapedText, runs []parentRun) *shapedParent {
	if len(text) < minParentBytes || len(s.clusters) > maxParentRunes || len(runs) == 0 || len(runs) != len(s.runs) {
		return nil
	}
	runs = slices.Clone(runs)
	for i := range runs {
		r := &runs[i]
		if len(runs) > 1 && joins(r.script) {
			return nil
		}
		glyphs := s.runs[i].glyphs
		if len(glyphs) == 0 {
			return nil
		}
		up, down := true, true
		for j := 1; j < len(glyphs); j++ {
			up = up && glyphs[j-1].cluster <= glyphs[j].cluster
			down = down && glyphs[j-1].cluster >= glyphs[j].cluster
		}
		if !up && !down {
			return nil
		}
		r.rev = !up
	}
	return &shapedParent{text: text, s: s, runs: runs}
}

// joins reports whether the shaper joins the letters of a script. It is
// hasArabicJoining of the harfbuzz package of okage. Only the shaper of
// these scripts reads the characters before and after a run.
func joins(script language.Script) bool {
	switch script {
	case language.Adlam, language.Arabic, language.Chorasmian, language.Hanifi_Rohingya, language.Mandaic,
		language.Manichaean, language.Mongolian, language.Nko, language.Old_Uyghur, language.Phags_Pa,
		language.Psalter_Pahlavi, language.Sogdian, language.Syriac:
		return true
	}
	return false
}

// keepParent keeps p as the parent of key, when p is not nil.
func (fc *FontConfiguration) keepParent(key parentKey, p *shapedParent) {
	if p == nil {
		return
	}
	if _, ok := fc.parents[key]; !ok && len(fc.parents) >= maxParents {
		clear(fc.parents)
	}
	fc.parents[key] = p
}

// shapeSlice returns the shaped text of a text that is the start or the
// end of the parent of key, from the glyphs of the parent. It returns nil
// when the text must be shaped.
//
// A text takes the glyphs of the parent when these facts are true:
//
//  1. Each cut is the start or the end of the parent, the start of a run,
//     or the start of a cluster that HarfBuzz does not mark as unsafe to
//     break.
//  2. The segmenter gives the text the runs of the parent, cut at the
//     cuts, with their faces, scripts and directions.
//
// Then the shaper gives the text the glyphs of its clusters in the parent.
func (fc *FontConfiguration) shapeSlice(key parentKey, text string, style *Style) *shapedText {
	p := fc.parents[key]
	if p == nil || text == "" || len(text) >= len(p.text) {
		return nil
	}
	var start, end int
	switch {
	case strings.HasPrefix(p.text, text):
		start, end = 0, len(text)
	case strings.HasSuffix(p.text, text):
		start, end = len(p.text)-len(text), len(p.text)
	default:
		return nil
	}
	first, ok := p.cut(start)
	if !ok {
		return nil
	}
	last, ok := p.cut(end)
	if !ok || !fc.sameRuns(p, first, last, text, style, key.rtl) {
		return nil
	}
	return p.slice(first, last, start)
}

// cut returns the index of the cluster of the parent that starts at the
// byte b, or the number of clusters for the end of the parent. It reports
// false when no cluster starts at b, or when the text cannot break there.
func (p *shapedParent) cut(b int) (int, bool) {
	clusters := p.s.clusters
	if b == len(p.text) {
		return len(clusters), true
	}
	i, found := slices.BinarySearchFunc(clusters, int32(b), func(c textCluster, b int32) int {
		return cmp.Compare(c.start, b)
	})
	if !found {
		return 0, false
	}
	// The shaper can mark the first cluster of a run, which is still safe.
	return i, i == 0 || !clusters[i].unsafe || clusters[i-1].run != clusters[i].run
}

// runes returns the runes of the clusters first to last of the parent.
func (p *shapedParent) runes(first, last int) (start, end int32) {
	clusters := p.s.clusters
	end = p.runs[len(p.runs)-1].end
	if last < len(clusters) {
		end = clusters[last].runeStart
	}
	return clusters[first].runeStart, end
}

// sameRuns reports whether the segmenter gives a text the runs of the
// clusters first to last of the parent: the same runes, faces, scripts
// and directions. For example, a text of only spaces and punctuation can
// get another script than the parent, and then other glyphs, and the
// bidirectional algorithm can give its last characters another direction.
func (fc *FontConfiguration) sameRuns(p *shapedParent, first, last int, text string, style *Style, rtl bool) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	runes := textRunes(&fc.shapeRunes, text, maxShapedRunes)
	m, faces := fc.styleFontmap(style)
	primary, _ := m.primary()
	if primary == nil {
		return false
	}
	direction := di.DirectionLTR
	if rtl {
		direction = di.DirectionRTL
	}
	input := shaping.Input{
		Text:      runes,
		RunEnd:    len(runes),
		Direction: direction,
		Face:      primary,
		Size:      fixed.I(int(primary.Upem())),
		Language:  languageOf(style),
	}
	items := fc.segmenter.Split(input, faces)
	start, end := p.runes(first, last)
	r0, r1 := p.s.clusters[first].run, p.s.clusters[last-1].run+1
	if len(items) != int(r1-r0) {
		return false
	}
	for i, item := range items {
		r := &p.runs[int(r0)+i]
		if item.RunStart != int(max(r.start, start)-start) || item.RunEnd != int(min(r.end, end)-start) ||
			item.Face != r.face || item.Script != r.script || item.Direction != r.dir {
			return false
		}
	}
	return true
}

// slice returns the shaped text of the clusters first to last of the
// parent, which start at the byte start. A text that starts at the start
// of the parent shares the clusters and the glyphs of the parent.
func (p *shapedParent) slice(first, last, start int) *shapedText {
	ps := p.s
	r0, r1 := ps.clusters[first].run, ps.clusters[last-1].run+1
	s := &shapedText{ascent: ps.ascent, descent: ps.descent, runs: make([]shapedRun, 0, r1-r0)}
	if first == 0 {
		s.clusters = ps.clusters[:last:last]
	} else {
		s.clusters = make([]textCluster, last-first)
		runeStart := ps.clusters[first].runeStart
		for i, c := range ps.clusters[first:last] {
			c.start -= int32(start)
			c.end -= int32(start)
			c.runeStart -= runeStart
			c.run -= r0
			s.clusters[i] = c
		}
	}
	for i := r0; i < r1; i++ {
		run := ps.runs[i]
		glyphs := run.glyphs
		// The glyphs of a run are in the order of their clusters, or in
		// the reverse order.
		var g0, g1 int
		if p.runs[i].rev {
			g0 = sort.Search(len(glyphs), func(j int) bool { return int(glyphs[j].cluster) < last })
			g1 = sort.Search(len(glyphs), func(j int) bool { return int(glyphs[j].cluster) < first })
		} else {
			g0 = sort.Search(len(glyphs), func(j int) bool { return int(glyphs[j].cluster) >= first })
			g1 = sort.Search(len(glyphs), func(j int) bool { return int(glyphs[j].cluster) >= last })
		}
		if first == 0 {
			run.glyphs = glyphs[g0:g1:g1]
		} else {
			run.glyphs = make([]shapedGlyph, g1-g0)
			for j, g := range glyphs[g0:g1] {
				g.cluster -= int32(first)
				run.glyphs[j] = g
			}
		}
		s.runs = append(s.runs, run)
	}
	return s
}
