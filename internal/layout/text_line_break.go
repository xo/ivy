// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports weasyprint/text/line_break.py: where to break the lines of
// text.
//
// Port: upstream lays out the text with a PangoLayout. The port has no
// Pango. TextLayout takes the place of the class Layout and of the
// PangoLayout that it holds: it selects the fonts with the FontDatabase of
// okage, shapes the text with go-text, finds the opportunities to break the
// lines with the segmenter of go-text (UAX #14), and breaks the first line
// as Pango does with its default wrap mode, PANGO_WRAP_WORD. split_first_line
// and the functions around it follow the Python code line by line, with the
// calls to Pango replaced by the methods of TextLayout.
//
// The hyphenation of hyphens: auto is off, because Pyphen and its
// dictionaries are not ported (D15). A soft hyphen still breaks a word.

package layout

import (
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/okage/svg"
	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/segmenter"
	"github.com/xo/okage/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// pangoScale is PANGO_SCALE, the number of units of Pango in a pixel. The
// port compares the widths in these units, as Pango does.
const pangoScale = 1024

// maxShapedRunes bounds the text that one layout shapes.
//
// Port: upstream has no bound. A longer text is measured without its end.
const maxShapedRunes = 1 << 18

// maxHyphenTries bounds the soft hyphens that split_first_line tries.
const maxHyphenTries = 256

// maxShapedTexts and maxShapedBytes bound the cache of the shaped texts of a
// configuration: the number of texts and the bytes of their texts.
const (
	maxShapedTexts = 4096
	maxShapedBytes = 1 << 22
)

// shapeKey is the key of a shaped text: the font of a style, the direction
// and the text.
type shapeKey struct {
	font string
	rtl  bool
	text string
}

// shapedText is a text shaped with the fonts of a style, in units of em.
type shapedText struct {
	// runeToByte maps the index of each rune, and the end, to its byte.
	runeToByte []int
	// clusters are the clusters of the text, in logical order.
	clusters []textCluster
	// runs are the runs of one face, in logical order.
	runs []shapedRun
	// ascent and descent are the metrics of the first face, for a line
	// without text.
	ascent, descent float64
}

// shapedRun is a run of one face and one direction.
type shapedRun struct {
	id     svg.FontID
	hasID  bool
	glyphs []shapedGlyph
	// ascent and descent are the metrics of the face, in em.
	ascent, descent float64
}

// shapedGlyph is a glyph of a run, in the order of the run, which is the
// visual order of the glyphs of a run from right to left.
type shapedGlyph struct {
	id      uint32
	cluster int // the index of the cluster in shapedText.clusters
	advance float64
	xOffset float64
	yOffset float64
}

// textCluster is a cluster of the text: the characters that the shaper
// draws together.
type textCluster struct {
	start, end int // bytes
	runeStart  int
	r          rune // the first rune
	advance    float64
	inkTop     float64
	run        int
}

// LogAttr is a PangoLogAttr: the log attributes of Pango that the port
// uses, for each rune of a text and for its end.
type LogAttr struct {
	// IsLineBreak is true when a line can break before the rune.
	IsLineBreak bool
	// IsWordStart, IsWordEnd and IsWordBoundary mark the words.
	IsWordStart    bool
	IsWordEnd      bool
	IsWordBoundary bool
}

// isParagraphSeparator reports whether r ends a paragraph of Pango. The
// separator is not in the line that it ends.
func isParagraphSeparator(r rune) bool {
	return r == '\n' || r == '\r' || r == ' '
}

// isForcedBreak reports whether r ends a line inside a paragraph of Pango.
// The character is in the line that it ends.
func isForcedBreak(r rune) bool {
	return r == '\v' || r == '\f' || r == '\u0085' || r == ' '
}

// isWhite reports whether r is white space that can hang at the end of a
// line, as the attribute is_white of Pango says.
//
// Port: a no-break space (U+00A0, U+2007 and U+202F) does not hang. In
// upstream, Pango does not let "d\u00a0" go past the width at the end of
// a text, as test_line_breaking_nbsp shows.
func isWhite(r rune) bool {
	switch r {
	case '\u00a0', '\u2007', '\u202f':
		return false
	}
	return r == '\t' || unicode.Is(unicode.Zs, r)
}

// shape shapes a text with the fonts of a style, from the cache when it
// can.
func (fc *FontConfiguration) shape(text string, style *Style) *shapedText {
	rtl := isStr(style.Get("direction"), "rtl")
	key := shapeKey{fontStyleCacheKey(style, false), rtl, text}
	if s, ok := fc.shaped[key]; ok {
		return s
	}
	s := fc.shapeText(text, style, rtl)
	if len(fc.shaped) >= maxShapedTexts || fc.shapedBytes+len(text) > maxShapedBytes {
		clear(fc.shaped)
		fc.shapedBytes = 0
	}
	fc.shaped[key] = s
	fc.shapedBytes += len(text)
	return s
}

// shapeText shapes a text.
func (fc *FontConfiguration) shapeText(text string, style *Style, rtl bool) (s *shapedText) {
	runes := []rune(text)
	if len(runes) > maxShapedRunes {
		runes = runes[:maxShapedRunes]
	}
	s = &shapedText{runeToByte: make([]int, len(runes)+1)}
	offset := 0
	for i, r := range runes {
		s.runeToByte[i] = offset
		offset += utf8.RuneLen(r)
	}
	s.runeToByte[len(runes)] = offset

	m, faces := fc.styleFontmap(style)
	primary, primaryID := m.primary()
	if primary == nil {
		// No font: each character is half the font size, as the default
		// metrics say.
		s.ascent, s.descent = 0.9, 0.3
		s.runs = []shapedRun{{ascent: s.ascent, descent: s.descent}}
		for i, r := range runes {
			s.clusters = append(s.clusters, textCluster{
				start: s.runeToByte[i], end: s.runeToByte[i+1], runeStart: i, r: r,
				advance: zeroWidth(r, 0.5), inkTop: 0.5,
			})
		}
		return s
	}
	s.ascent, s.descent = faceExtents(primary)
	if len(runes) == 0 {
		return s
	}

	defer func() {
		// Port: a panic in the shaper is a text without fonts.
		if r := recover(); r != nil {
			fc.log.debug("Failed to shape text: %v", r)
			s.clusters = s.clusters[:0]
			s.runs = []shapedRun{{ascent: s.ascent, descent: s.descent}}
			for i, r := range runes {
				s.clusters = append(s.clusters, textCluster{
					start: s.runeToByte[i], end: s.runeToByte[i+1], runeStart: i, r: r,
					advance: zeroWidth(r, 0.5), inkTop: 0.5,
				})
			}
		}
	}()

	direction := di.DirectionLTR
	if rtl {
		direction = di.DirectionRTL
	}
	input := shaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: direction,
		Face:      primary,
		Size:      fixed.I(int(primary.Upem())),
		Language:  languageOf(style),
	}
	features := styleFeatures(style)
	// byRune holds the clusters by their first rune, until they are sorted.
	byRune := make([]*textCluster, len(runes))
	// The bidirectional algorithm of go-text reads one paragraph, so each
	// paragraph is split on its own.
	for start := 0; start < len(runes); {
		end := start
		for end < len(runes) && !isParagraphSeparator(runes[end]) {
			end++
		}
		if end < len(runes) {
			end++
			if runes[end-1] == '\r' && end < len(runes) && runes[end] == '\n' {
				end++
			}
		}
		input.RunStart, input.RunEnd = start, end
		start = end
		for _, item := range fc.segmenter.Split(input, faces) {
			if item.Face == nil {
				continue
			}
			id := m.byFace[item.Face]
			upem := float64(item.Face.Upem())
			item.Size = fixed.I(int(item.Face.Upem()))
			item.FontFeatures = features
			if rule, ok := fc.faceRules[id]; ok {
				item.FontFeatures = append(slices.Clone(rule.features), features...)
			}
			out := fc.shaper.Shape(item)
			ascent, descent := faceExtents(item.Face)
			run := shapedRun{id: id, hasID: true, ascent: ascent, descent: descent}
			runIndex := len(s.runs)
			for _, g := range out.Glyphs {
				ci := g.TextIndex()
				if ci < 0 || ci >= len(runes) {
					continue
				}
				c := byRune[ci]
				if c == nil {
					end := min(ci+max(g.RunesCount(), 1), len(runes))
					c = &textCluster{
						start: s.runeToByte[ci], end: s.runeToByte[end], runeStart: ci, r: runes[ci],
						run: runIndex,
					}
					byRune[ci] = c
				}
				advance := float64(g.Advance) / 64 / upem
				c.advance += advance
				c.inkTop = max(c.inkTop, float64(g.YBearing+g.YOffset)/64/upem)
				run.glyphs = append(run.glyphs, shapedGlyph{
					id:      uint32(g.GlyphID),
					cluster: ci, // the first rune until the clusters are sorted
					advance: advance,
					xOffset: float64(g.XOffset) / 64 / upem,
					yOffset: float64(g.YOffset) / 64 / upem,
				})
			}
			s.runs = append(s.runs, run)
		}
	}
	// Each character that the shaper left out, such as a separator of
	// paragraphs, is a cluster without width.
	for i, r := range runes {
		if byRune[i] != nil {
			continue
		}
		covered := false
		for j := i - 1; j >= 0 && j >= i-8; j-- {
			if c := byRune[j]; c != nil {
				covered = c.end > s.runeToByte[i]
				break
			}
		}
		if !covered {
			if len(s.runs) == 0 {
				s.runs = append(s.runs, shapedRun{id: primaryID, hasID: true, ascent: s.ascent, descent: s.descent})
			}
			byRune[i] = &textCluster{start: s.runeToByte[i], end: s.runeToByte[i+1], runeStart: i, r: r}
		}
	}
	// Sort the clusters in the order of the text.
	index := make([]int, len(runes))
	for i, c := range byRune {
		if c != nil {
			index[i] = len(s.clusters)
			c.advance = zeroWidth(c.r, c.advance)
			s.clusters = append(s.clusters, *c)
		}
	}
	for i := range s.runs {
		for j := range s.runs[i].glyphs {
			g := &s.runs[i].glyphs[j]
			g.cluster = index[g.cluster]
		}
	}
	return s
}

// styleFonts is the fontmap of a style, with the faces of the characters
// that it resolved. A style keeps it for one FontConfiguration, while the
// FontConfiguration reads no new @font-face rule.
type styleFonts struct {
	fc *FontConfiguration
	// rules is the number of @font-face rules of fc when the fontmap was
	// made.
	rules int
	m     *fontmap
	faces runeFaces
}

// styleFontmap returns the fontmap of a style, and the shaping.Fontmap that
// keeps the faces of its characters.
//
// Port: the segmenter of go-text asks for the face of each character of
// each line, and the fontmap looks for it again in each font of the chain.
// The style keeps the faces between the lines. A face of a character does
// not change for a style, until a new @font-face rule changes the faces of
// the families.
func (fc *FontConfiguration) styleFontmap(style *Style) (*fontmap, *runeFaces) {
	if sf := style.fonts; sf != nil && sf.fc == fc && sf.rules == len(fc.rules) {
		return sf.m, &sf.faces
	}
	sf := &styleFonts{fc: fc, rules: len(fc.rules), m: fc.newFontmap(style)}
	sf.faces.m = sf.m
	style.fonts = sf
	return sf.m, &sf.faces
}

// runeFaces is a shaping.Fontmap that keeps the face of each character
// that its fontmap resolved.
type runeFaces struct {
	m     *fontmap
	ascii [128]*font.Face
	known [128]bool
	other map[rune]*font.Face
}

// ResolveFace satisfies shaping.Fontmap.
func (rf *runeFaces) ResolveFace(r rune) *font.Face {
	if r >= 0 && r < 128 {
		if !rf.known[r] {
			rf.ascii[r], rf.known[r] = rf.m.ResolveFace(r), true
		}
		return rf.ascii[r]
	}
	if f, ok := rf.other[r]; ok {
		return f
	}
	f := rf.m.ResolveFace(r)
	if rf.other == nil {
		rf.other = map[rune]*font.Face{}
	}
	rf.other[r] = f
	return f
}

// zeroWidth returns 0 for the characters that Pango draws without width:
// the separators of paragraphs and the forced breaks. A tab gets its width
// from the tab stops.
func zeroWidth(r rune, advance float64) float64 {
	if isParagraphSeparator(r) || isForcedBreak(r) || r == '\t' {
		return 0
	}
	return advance
}

// faceExtents returns the ascent and the descent of a face, in em, as Pango
// reads them from the font.
func faceExtents(f *font.Face) (ascent, descent float64) {
	upem := float64(f.Upem())
	if upem == 0 {
		return 0.9, 0.3
	}
	extents, ok := f.FontHExtents()
	if !ok {
		return 0.9, 0.3
	}
	return float64(extents.Ascender) / upem, -float64(extents.Descender) / upem
}

// TextLayout is the class Layout of weasyprint/text/line_break.py, with the
// PangoLayout that it holds: a text, its style, and the width where its
// lines break.
type TextLayout struct {
	// Text is the text of the layout. SetText keeps only the first line
	// and one more character.
	Text string
	// Style is the style of the text. Deactivate removes it.
	Style *Style
	// JustificationSpacing is the spacing that justifies the text.
	JustificationSpacing float64
	// MaxWidth is the width that create_layout gave, or nil for None.
	MaxWidth *float64
	// FirstLineDirection is the direction of the first line: 0 for left to
	// right and 1 for right to left, the values of PangoDirection.
	FirstLineDirection int

	// The metrics of the first font, in pixels, for the decoration of the
	// text. HasDecorationMetrics is false when text-decoration-line is
	// none, the None of upstream.
	HasDecorationMetrics   bool
	Ascent                 float64
	UnderlinePosition      float64
	StrikethroughPosition  float64
	UnderlineThickness     float64
	StrikethroughThickness float64

	fc       *FontConfiguration
	fontSize float64
	// width is the width of pango_layout_set_width, in units of Pango, or
	// -1 for no width.
	width    int64
	wrapChar bool
	justify  bool
	shaped   *shapedText
	attrs    []LogAttr
	tabWidth float64
	// autospace holds, for each cluster, whether the space of
	// -ivy-text-autospace comes after it. It is nil until autospaceAfter
	// computes it.
	autospace []bool
}

// newTextLayout is Layout.__init__ in weasyprint/text/line_break.py.
func newTextLayout(style *Style, justificationSpacing float64, maxWidth *float64) *TextLayout {
	l := &TextLayout{JustificationSpacing: justificationSpacing, width: -1}
	l.setup(style)
	l.MaxWidth = maxWidth
	return l
}

// setup is Layout.setup in weasyprint/text/line_break.py.
func (l *TextLayout) setup(style *Style) {
	l.Style = style
	l.FirstLineDirection = 0
	l.fc = fontConfigOf(style)
	l.fontSize = num(style.Get("font_size"))

	l.HasDecorationMetrics = false
	if !isStr(style.Get("text_decoration_line"), "none") {
		l.HasDecorationMetrics = true
		l.Ascent, l.UnderlinePosition, l.StrikethroughPosition = 0.9*l.fontSize, -0.1*l.fontSize, 0.3*l.fontSize
		l.UnderlineThickness, l.StrikethroughThickness = 0.05*l.fontSize, 0.05*l.fontSize
		if f, _ := l.fc.newFontmap(style).primary(); f != nil {
			if upem := float64(f.Upem()); upem > 0 {
				ascent, _ := faceExtents(f)
				l.Ascent = ascent * l.fontSize
				scale := l.fontSize / upem
				l.UnderlinePosition = float64(f.LineMetric(font.UnderlinePosition)) * scale
				l.UnderlineThickness = float64(f.LineMetric(font.UnderlineThickness)) * scale
				l.StrikethroughPosition = float64(f.LineMetric(font.StrikethroughPosition)) * scale
				l.StrikethroughThickness = float64(f.LineMetric(font.StrikethroughThickness)) * scale
			}
		}
	}
}

// setWidth is pango_layout_set_width: a negative width, the -1 of Pango, is
// no width.
func (l *TextLayout) setWidth(width float64) {
	switch {
	case width < 0 || math.IsNaN(width):
		l.width = -1
	case width*pangoScale >= math.MaxInt64:
		l.width = math.MaxInt64
	default:
		l.width = int64(width * pangoScale)
	}
}

// SetText is Layout.set_text in weasyprint/text/line_break.py. justify adds
// the justification spacing to the word spacing, which the drawing needs
// and the layout does not.
func (l *TextLayout) SetText(text string, justify bool) {
	if index := strings.IndexByte(text, '\n'); index != -1 {
		// Keep only the first line plus one character, we don't need more
		_, size := utf8.DecodeRuneInString(text[index+1:])
		text = text[:index+1+size]
	}
	l.Text = text
	l.justify = justify
	l.shaped = nil
	l.attrs = nil
	l.tabWidth = 0
	l.autospace = nil
	if strings.Contains(text, "\t") {
		l.setTabs()
	}
}

// setTabs is Layout.set_tabs in weasyprint/text/line_break.py.
func (l *TextLayout) setTabs() {
	var width float64
	if n, ok := l.Style.Get("tab_size").(Int); ok {
		layout := newTextLayout(l.Style, l.JustificationSpacing, nil)
		layout.SetText(strings.Repeat(" ", min(int(n), 1<<10)), false)
		line, _, _ := layout.FirstLine()
		width, _ = lineSize(line, l.Style)
		width = math.RoundToEven(width)
	} else if d, ok := l.Style.Get("tab_size").(Dimension); ok {
		width = d.Value
	} else {
		width = num(l.Style.Get("tab_size"))
	}
	// 0 is not handled correctly by Pango
	if width <= 0 {
		width = 1.0 / pangoScale
	}
	l.tabWidth = width
}

// Deactivate is Layout.deactivate in weasyprint/text/line_break.py. It
// frees the shaped text and the style.
func (l *TextLayout) Deactivate() {
	l.Style = nil
	l.shaped = nil
	l.attrs = nil
	l.autospace = nil
}

// Reactivate is Layout.reactivate in weasyprint/text/line_break.py. It sets
// the style again and the text with the justification, for the drawing.
func (l *TextLayout) Reactivate(style *Style) {
	l.setup(style)
	l.SetText(l.Text, true)
}

// shapedText returns the shaped text of the layout.
func (l *TextLayout) shapedText() *shapedText {
	if l.shaped == nil {
		l.shaped = l.fc.shape(l.Text, l.Style)
	}
	return l.shaped
}

// LogAttrs returns the log attributes of the text of the layout, one for
// each rune and one for the end, as pango_layout_get_log_attrs_readonly
// gives them.
func (l *TextLayout) LogAttrs() []LogAttr {
	if l.attrs == nil {
		l.attrs = logAttrs(l.Text)
	}
	return l.attrs
}

// TextLine is a PangoLayoutLine: the first line of a layout.
type TextLine struct {
	// Length is the length of the line in bytes. A separator of
	// paragraphs that ends the line is not in it.
	Length int
	// Width and Height are the logical extents of the line, in pixels.
	Width, Height float64
	// Ascent is the distance from the top of the line to its baseline,
	// pango_layout_get_baseline of upstream.
	Ascent float64
	// InkTop is the distance from the baseline to the top of the ink of
	// the glyphs, the opposite of the y of the ink extents of Pango.
	InkTop float64
	// ResolvedDirection is the direction of the line: 0 for left to right,
	// 1 for right to left.
	ResolvedDirection int

	layout *TextLayout
	// first and last are the clusters of the line.
	first, last int
	// wrapped is true when the line breaks at an opportunity to break that
	// is not a forced break.
	wrapped bool
}

// clusterSpacing returns the spacing after the cluster i: the letter
// spacing, the word spacing after a space, and the space of
// -ivy-text-autospace.
func (l *TextLayout) clusterSpacing(i int) float64 {
	c := &l.shapedText().clusters[i]
	var spacing float64
	letterSpacing := l.Style.Get("letter_spacing")
	if !isStr(letterSpacing, "normal") {
		spacing = num(letterSpacing)
	}
	if c.r == ' ' || c.r == ' ' {
		wordSpacing := num(l.Style.Get("word_spacing"))
		if l.justify {
			// Justification is needed when drawing text but is useless
			// during layout, when it can be ignored.
			wordSpacing += l.JustificationSpacing
		}
		spacing += wordSpacing
	}
	if l.autospaceAfter(i) {
		// Port: upstream has no text-autospace.
		spacing += l.fontSize * autospaceRatio
	}
	return spacing
}

// advance returns the advance of the cluster i at the position x of the
// line.
func (l *TextLayout) advance(i int, x float64) float64 {
	c := &l.shapedText().clusters[i]
	if c.r == '\t' && l.tabWidth > 0 {
		next := (math.Floor(x/l.tabWidth+1e-9) + 1) * l.tabWidth
		return next - x
	}
	return c.advance*l.fontSize + l.clusterSpacing(i)
}

// FirstLine is Layout.get_first_line in weasyprint/text/line_break.py. It
// returns the first line of the layout, and the index in bytes of the start
// of the second line, with false when there is no second line.
//
// Port: the line breaks as a line of Pango with the wrap mode
// PANGO_WRAP_WORD, or PANGO_WRAP_CHAR after setWrapChar. The white space at
// the end of the line can go past the width. When no opportunity to break
// fits, the line breaks at the first opportunity after the width.
func (l *TextLayout) FirstLine() (*TextLine, int, bool) {
	s := l.shapedText()
	attrs := l.LogAttrs()
	limit := int64(-1)
	if l.width >= 0 {
		limit = l.width
	}
	line := &TextLine{layout: l, first: 0, last: 0}
	if isStr(l.Style.Get("direction"), "rtl") {
		line.ResolvedDirection = 1
	}
	l.FirstLineDirection = line.ResolvedDirection

	x := 0.0
	end := len(s.clusters)    // the first cluster after the line
	next, hasNext := 0, false // the byte where the second line starts
	lastBreak := -1           // the last cluster before which the line can break
	overflow := false
loop:
	for i := range s.clusters {
		c := &s.clusters[i]
		if isParagraphSeparator(c.r) {
			end, next, hasNext = i, c.end, true
			if c.r == '\r' && i+1 < len(s.clusters) && s.clusters[i+1].r == '\n' {
				next = s.clusters[i+1].end
			}
			break
		}
		canBreak := i > 0 && (l.wrapChar || c.runeStart < len(attrs) && attrs[c.runeStart].IsLineBreak)
		if canBreak {
			if overflow {
				// The line is too long and cannot break before: it breaks
				// at the first opportunity after the width.
				end, next, hasNext = i, c.start, true
				line.wrapped = true
				break
			}
			lastBreak = i
		}
		x += l.advance(i, x)
		if isForcedBreak(c.r) {
			end, next, hasNext = i+1, c.end, true
			break
		}
		// Port: Pango sums advances in whole units of Pango, and truncates
		// the width. The port sums exact advances, and truncates the sum
		// too, so that a line as wide as the width fits.
		if limit >= 0 && !overflow && !isWhite(c.r) && int64(math.Floor(x*pangoScale)) > limit {
			switch {
			case l.wrapChar && i > 0:
				end, next, hasNext = i, c.start, true
				line.wrapped = true
				break loop
			case lastBreak > 0:
				end, next, hasNext = lastBreak, s.clusters[lastBreak].start, true
				line.wrapped = true
				break loop
			}
			overflow = true
		}
	}

	// Port: when the whole text fits but for a final space, the space goes
	// past the width, and Pango gives it no width, as for a wrapped line.
	if limit >= 0 && !line.wrapped && end == len(s.clusters) && end > 0 && s.clusters[end-1].r == ' ' &&
		int64(math.Floor(x*pangoScale)) > limit {
		line.wrapped = true
	}
	line.last = end
	if end < len(s.clusters) {
		line.Length = s.clusters[end].start
	} else {
		line.Length = len(l.Text)
	}
	l.measureLine(line)
	return line, next, hasNext
}

// measureLine sets the extents of a line.
func (l *TextLayout) measureLine(line *TextLine) {
	s := l.shapedText()
	x := 0.0
	ascent, descent := 0.0, 0.0
	// Port: Pango gives a width of 0 to the final space of a line that it
	// wraps, in zero_line_final_space of pango-layout.c. Without it, words
	// that fill the width exactly break one word too early.
	zeroLast := line.wrapped && line.last > line.first && s.clusters[line.last-1].r == ' '
	for i := line.first; i < line.last; i++ {
		c := &s.clusters[i]
		if !zeroLast || i != line.last-1 {
			x += l.advance(i, x)
		}
		run := s.runs[c.run]
		ascent = max(ascent, run.ascent)
		descent = max(descent, run.descent)
		line.InkTop = max(line.InkTop, c.inkTop*l.fontSize)
	}
	if line.last > line.first {
		// Port: Pango puts the letter spacing between the characters, and
		// not after the last one. lineSize adds it, as upstream does.
		letterSpacing := l.Style.Get("letter_spacing")
		if !isStr(letterSpacing, "normal") {
			x -= num(letterSpacing)
		}
	} else {
		ascent, descent = s.ascent, s.descent
	}
	line.Width = x
	line.Ascent = ascent * l.fontSize
	line.Height = (ascent + descent) * l.fontSize
}

// GlyphRun is a run of glyphs of one face of a line, for the drawing.
type GlyphRun struct {
	// Font is the face of the run in the FontDatabase of the
	// configuration, and HasFont is false for a text without fonts.
	Font    svg.FontID
	HasFont bool
	// Size is the font size, in pixels.
	Size float64
	// Glyphs are the glyphs, from left to right.
	Glyphs []Glyph
}

// Glyph is a glyph of a run.
type Glyph struct {
	// ID is the glyph in the face.
	ID uint32
	// X is the position of the origin of the glyph from the start of the
	// line, and Y its position from the baseline, down, in pixels.
	X, Y float64
	// Advance is the advance of the glyph, with the spacing, in pixels.
	Advance float64
	// Cluster is the byte of the text where the cluster of the glyph
	// starts.
	Cluster int
}

// GlyphRuns returns the glyphs of the line, run by run, from left to right.
//
// Port: the runs are in the logical order of the line, and in the reverse
// order for a line from right to left. upstream draws the runs of Pango,
// which reorders them with the levels of the bidirectional algorithm.
func (line *TextLine) GlyphRuns() []GlyphRun {
	l := line.layout
	s := l.shapedText()
	order := make([]int, len(s.runs))
	for i := range order {
		order[i] = i
	}
	if line.ResolvedDirection == 1 {
		slices.Reverse(order)
	}
	var runs []GlyphRun
	x := 0.0
	for _, i := range order {
		run := s.runs[i]
		gr := GlyphRun{Font: run.id, HasFont: run.hasID, Size: l.fontSize}
		lastCluster := -1
		for _, g := range run.glyphs {
			if g.cluster < line.first || g.cluster >= line.last {
				continue
			}
			c := &s.clusters[g.cluster]
			if g.cluster != lastCluster && lastCluster >= 0 {
				x += l.clusterSpacing(lastCluster)
			}
			advance := g.advance * l.fontSize
			if c.r == '\t' {
				advance = l.advance(g.cluster, x)
			}
			gr.Glyphs = append(gr.Glyphs, Glyph{
				ID: g.id, X: x + g.xOffset*l.fontSize, Y: -g.yOffset * l.fontSize,
				Advance: advance, Cluster: c.start,
			})
			x += advance
			lastCluster = g.cluster
		}
		if lastCluster >= 0 {
			x += l.clusterSpacing(lastCluster)
		}
		if len(gr.Glyphs) > 0 {
			runs = append(runs, gr)
		}
	}
	return runs
}

// setWrapChar is pango_layout_set_wrap with PANGO_WRAP_CHAR.
func (l *TextLayout) setWrapChar() {
	l.wrapChar = true
}

// logAttrs returns the log attributes of a text: the opportunities to
// break the lines of UAX #14 from the segmenter of go-text, and its words.
//
// Port: Pango tailors the rules for some languages. The port does not.
func logAttrs(text string) []LogAttr {
	runes := []rune(text)
	attrs := make([]LogAttr, len(runes)+1)
	if len(runes) == 0 {
		return attrs
	}
	var seg segmenter.Segmenter
	seg.Init(runes)
	lines := seg.LineIterator()
	for lines.Next() {
		line := lines.Line()
		if end := line.Offset + len(line.Text); end > 0 && end < len(runes) {
			attrs[end].IsLineBreak = true
		}
	}
	tailorSpaceBreaks(runes, attrs)
	words := seg.WordIterator()
	for words.Next() {
		word := words.Word()
		attrs[word.Offset].IsWordStart = true
		attrs[word.Offset].IsWordBoundary = true
		end := word.Offset + len(word.Text)
		attrs[end].IsWordEnd = true
		attrs[end].IsWordBoundary = true
	}
	return attrs
}

// tailorSpaceBreaks allows a break after a space and before a closing
// punctuation, an infix separator or a slash: the classes CL, CP, IS and SY
// of UAX #14.
//
// Port: Pango tailors rule LB13 of UAX #14 as its "Example 7" does, so that
// the rule forbids a break before those classes only when no space comes
// before them. go-text follows LB13 as it is. Rule LB14 still holds: no
// break comes after an opening punctuation and its spaces.
func tailorSpaceBreaks(runes []rune, attrs []LogAttr) {
	for i := 1; i < len(runes); i++ {
		if runes[i-1] != ' ' || !isCloseOrInfix(runes[i]) {
			continue
		}
		j := i - 1
		for j >= 0 && runes[j] == ' ' {
			j--
		}
		if j >= 0 && unicode.Is(unicode.Ps, runes[j]) {
			continue
		}
		attrs[i].IsLineBreak = true
	}
}

// isCloseOrInfix reports whether r is of the class CL, CP, IS or SY of
// UAX #14. go-text keeps the classes in an internal package, so the table
// here stands for them.
func isCloseOrInfix(r rune) bool {
	if unicode.Is(unicode.Pe, r) {
		return true
	}
	switch r {
	case ',', '.', ':', ';', '/', '\u037e', '\u0589', '\u060c', '\u060d', '\u07f8', '\u2044',
		'\ufe10', '\ufe13', '\ufe14', '\u3001', '\u3002', '\ufe11', '\ufe12', '\ufe50', '\ufe52',
		'\uff0c', '\uff0e', '\uff61', '\uff64':
		return true
	}
	return false
}

// lineSize is line_size in weasyprint/text/line_break.py. It returns the
// logical width and height of a line, with the letter spacing of the style.
func lineSize(line *TextLine, style *Style) (width, height float64) {
	width, height = line.Width, line.Height
	if letterSpacing := style.Get("letter_spacing"); !isStr(letterSpacing, "normal") {
		width += num(letterSpacing)
	}
	return width, height
}

// FirstLineMetrics is the result of split_first_line in
// weasyprint/text/line_break.py.
type FirstLineMetrics struct {
	// Layout is the layout of the first line.
	Layout *TextLayout
	// Length is the length in UTF-8 bytes of the first line.
	Length int
	// ResumeIndex is the number of UTF-8 bytes to skip for the next line.
	// It can be larger than Length, for a preserved new line. HasResume is
	// false when the whole text fits in one line, the None of upstream.
	ResumeIndex int
	HasResume   bool
	// Width and Height are the size of the first line, in pixels.
	Width, Height float64
	// Baseline is the baseline of the first line, from its top, in pixels.
	Baseline float64
}

// firstLineMetrics is first_line_metrics in weasyprint/text/line_break.py.
func firstLineMetrics(firstLine *TextLine, text string, layout *TextLayout, resumeAt int, hasResume, spaceCollapse bool, style *Style, hyphenated bool, hyphenationCharacter string) FirstLineMetrics {
	length := firstLine.Length
	switch {
	case hyphenated:
		length -= len(hyphenationCharacter)
	case hasResume:
		// Set an infinite width as we don't want to break lines when
		// drawing, the lines have already been split and the size may
		// differ. Rendering is also much faster when no width is set.
		layout.setWidth(-1)

		// Create layout with final text
		firstLineText := text[:min(length, len(text))]

		// Remove trailing spaces if spaces collapse
		if spaceCollapse {
			firstLineText = strings.TrimRight(firstLineText, " ")
		}

		wrapped := firstLine.wrapped
		layout.SetText(firstLineText, false)
		firstLine, _, _ = layout.FirstLine()
		length = firstLine.Length
		if wrapped && !spaceCollapse && !firstLine.wrapped {
			// Port: the final space of a wrapped line keeps its width of 0
			// when the spaces do not collapse, as in the first layout.
			firstLine.wrapped = true
			layout.measureLine(firstLine)
		}
	}

	width, height := lineSize(firstLine, style)
	baseline := firstLine.Ascent
	layout.Deactivate()
	return FirstLineMetrics{
		Layout: layout, Length: length, ResumeIndex: resumeAt, HasResume: hasResume,
		Width: width, Height: height, Baseline: baseline,
	}
}

// CreateLayout is create_layout in weasyprint/text/line_break.py. It
// returns a layout with the line breaks of Pango. maxWidth is nil for
// None.
func CreateLayout(text string, style *Style, maxWidth *float64, justificationSpacing float64) *TextLayout {
	layout := newTextLayout(style, justificationSpacing, maxWidth)

	// Make sure that max_width * Pango.SCALE == max_width * 1024 fits in a
	// signed integer. Treat bigger values same as None: unconstrained width.
	textWrap := isStrIn(style.Get("white_space"), "normal", "pre-wrap", "pre-line")
	if maxWidth != nil && textWrap && *maxWidth < 1<<21 {
		layout.setWidth(max(0, *maxWidth))
	}

	layout.SetText(text, false)
	return layout
}

// runeLen returns the number of runes of s, len() of a str of Python.
func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// runeSlice returns s[start:end] of Python, with indexes in runes. A
// negative end is no end.
func runeSlice(s string, start, end int) string {
	i := 0
	startByte, endByte := len(s), len(s)
	if start <= 0 {
		startByte = 0
	}
	for b := range s {
		if i == start {
			startByte = b
		}
		if end >= 0 && i == end {
			endByte = b
			break
		}
		i++
	}
	if startByte > endByte {
		return ""
	}
	return s[startByte:endByte]
}

// SplitFirstLine is split_first_line in weasyprint/text/line_break.py. It
// fits as much as possible of a text in the available width for one line.
// maxWidth is nil for None, an unconstrained width.
//
// Port: upstream takes the context of the layout for the dictionaries of
// the hyphenation, which the port does not have.
func SplitFirstLine(text string, style *Style, maxWidth *float64, justificationSpacing float64, isLineStart, minimum bool) FirstLineMetrics {
	// See https://www.w3.org/TR/css-text-3/#white-space-property
	whiteSpace := style.Get("white_space")
	textWrap := isStrIn(whiteSpace, "normal", "pre-wrap", "pre-line")
	spaceCollapse := isStrIn(whiteSpace, "normal", "nowrap", "pre-line")

	originalMaxWidth := maxWidth
	if !textWrap {
		maxWidth = nil
	}

	// Step #1: Get a draft layout with the first line.
	var layout *TextLayout
	var firstLine *TextLine
	var resumeIndex int
	var hasResume bool
	const ratio = 4 // number that almost always respects char_height / char_width > ratio
	shortText := text
	fontSize := num(style.Get("font_size"))
	if maxWidth != nil && !math.IsInf(*maxWidth, 1) && fontSize != 0 {
		// Try to use a small amount of text to avoid the whole layout. We
		// need at least one line, and one possible line break point on the
		// second line.
		if fontSize*ratio > *maxWidth {
			// Trying to find minimum or very small size, let's naively
			// split on spaces and keep one word + one letter.
			if spaceIndex := strings.IndexByte(text, ' '); spaceIndex != -1 {
				// index + space + one letter
				_, size := utf8.DecodeRuneInString(text[spaceIndex+1:])
				shortText = text[:spaceIndex+1+size]
			}
		} else {
			// Use the magic ration and hope that we’ll get the right amount
			// of text.
			n := *maxWidth / fontSize * ratio
			if n < float64(len(text)) {
				shortText = runeSlice(text, 0, int(n))
			}
		}
		layout = CreateLayout(shortText, style, maxWidth, justificationSpacing)
		firstLine, resumeIndex, hasResume = layout.FirstLine()
		if !hasResume && shortText != text {
			// The small amount of text fits in one line, give up and use
			// the whole text.
			shortText = text
			layout.SetText(text, false)
			firstLine, resumeIndex, hasResume = layout.FirstLine()
		} else {
			// If the second line of the short text can break, we have the
			// next line break point required for step #3 in it, drop the
			// end of the text.
			firstLineText := shortText
			if hasResume {
				firstLineText = shortText[:min(resumeIndex, len(shortText))]
			}
			if firstLineText != shortText {
				start, end := runeLen(firstLineText)+1, runeLen(shortText)
				textEndLogAttrs := sliceAttrs(layout.LogAttrs(), start, end)
				if _, ok := getNextBreakPoint(textEndLogAttrs); ok {
					text = shortText
				}
			}
		}
	} else {
		layout = CreateLayout(text, style, originalMaxWidth, justificationSpacing)
		firstLine, resumeIndex, hasResume = layout.FirstLine()
	}

	// Step #2: Don't split lines when it's not needed.
	if maxWidth == nil {
		// The first line can take all the place needed.
		return firstLineMetrics(firstLine, text, layout, resumeIndex, hasResume, spaceCollapse, style, false, "")
	}
	firstLineWidth, _ := lineSize(firstLine, style)
	if !hasResume && firstLineWidth <= *maxWidth {
		// The first line fits in the available width.
		return firstLineMetrics(firstLine, text, layout, resumeIndex, hasResume, spaceCollapse, style, false, "")
	}

	// Step #3: Try to put the first word of the second line on the first
	// line https://mail.gnome.org/archives/gtk-i18n-list/2013-September/msg00006
	// is a good thread related to this problem.
	var firstLineText, secondLineText string
	if firstLineWidth <= *maxWidth {
		// The first line fits but may have been cut too early by Pango.
		cut := len(text)
		if hasResume {
			cut = min(resumeIndex, len(text))
		}
		firstLineText = text[:cut]
		secondLineText = text[cut:]
	} else {
		// The line can't be split earlier, try to hyphenate the first
		// word.
		firstLineText = ""
		secondLineText = text
	}
	breakPoint, hasBreakPoint := 0, false
	if firstLineText != shortText {
		// Find then second line’s first break point.
		start, end := runeLen(firstLineText)+1, runeLen(shortText)
		secondLineLogAttrs := sliceAttrs(layout.LogAttrs(), start, end)
		breakPoint, hasBreakPoint = getNextBreakPoint(secondLineLogAttrs)
	}
	nextWord := secondLineText
	if hasBreakPoint {
		nextWord = runeSlice(secondLineText, 0, breakPoint)
	}
	nextWord = strings.TrimRight(nextWord, " ")
	if nextWord != "" {
		index := breakPoint
		if !hasBreakPoint || breakPoint == 0 {
			index = runeLen(secondLineText) - 1
		}
		if spaceCollapse && runeSlice(secondLineText, index, index+1) == " " {
			// Next word might fit without a space afterwards only try when
			// space collapsing is allowed.
			newFirstLineText := firstLineText + nextWord
			layout.SetText(newFirstLineText, false)
			firstLine, resumeIndex, hasResume = layout.FirstLine()
			if !hasResume {
				if firstLineText != "" {
					// The next word fits in the first line, keep the
					// layout.
					resumeIndex = len(newFirstLineText) + 1
					return firstLineMetrics(firstLine, text, layout, resumeIndex, true, spaceCollapse, style, false, "")
				}
				// Second line is None.
				resumeIndex, hasResume = firstLine.Length+1, true
				if resumeIndex >= len(text) {
					hasResume = false
				}
			}
		}
	} else if firstLineText != "" {
		// We found something on the first line but we did not find a word
		// on the next line, no need to hyphenate, we can keep the current
		// layout.
		return firstLineMetrics(firstLine, text, layout, resumeIndex, hasResume, spaceCollapse, style, false, "")
	}

	// Step #4: Try to hyphenate
	hyphens := style.Get("hyphens")
	hyphenated := false
	const softHyphen = "\u00ad"
	hyphenateCharacter := str(style.Get("hyphenate_character"))

	// Port: the automatic hyphenation needs Pyphen, which is not ported
	// (D15). So lang is None, and only the soft hyphens hyphenate.
	manualHyphenation := false
	if !isStr(hyphens, "none") {
		manualHyphenation = strings.Contains(firstLineText+secondLineText, softHyphen)
	}

	// Automatic hyphenation opportunities within a word must be ignored if
	// the word contains a conditional hyphen, in favor of the conditional
	// hyphen(s).
	// See https://drafts.csswg.org/css-text-3/#valdef-hyphens-auto
	var dictionaryIterations []string
	if manualHyphenation {
		// Manual hyphenation: check that the line ends with a soft hyphen
		// and add the missing hyphen
		if strings.HasSuffix(firstLineText, softHyphen) {
			// The first line has been split on a soft hyphen
			firstLineText, secondLineText = "", firstLineText
		}
		// Port: upstream tries each soft hyphen of the text, and each try
		// lays out the text again. The port tries at most maxHyphenTries
		// of them.
		for i := len(secondLineText); i > 0 && len(dictionaryIterations) < maxHyphenTries; {
			j := strings.LastIndex(secondLineText[:i], softHyphen)
			if j < 0 {
				break
			}
			dictionaryIterations = append(dictionaryIterations, secondLineText[:j+len(softHyphen)])
			i = j
		}
	}

	var newFirstLineText, hyphenatedFirstLineText string
	if len(dictionaryIterations) > 0 {
		for _, firstWordPart := range dictionaryIterations {
			newFirstLineText = firstLineText + firstWordPart
			hyphenatedFirstLineText = newFirstLineText + hyphenateCharacter
			newLayout := CreateLayout(hyphenatedFirstLineText, style, maxWidth, justificationSpacing)
			newFirstLine, _, hasIndex := newLayout.FirstLine()
			newFirstLineWidth, _ := lineSize(newFirstLine, style)
			newSpace := *maxWidth - newFirstLineWidth
			hyphenated = !hasIndex && (newSpace >= 0 || firstWordPart == dictionaryIterations[len(dictionaryIterations)-1])
			if hyphenated {
				layout = newLayout
				firstLine = newFirstLine
				resumeIndex, hasResume = len(newFirstLineText), true
				break
			}
		}

		if !hyphenated && firstLineText == "" {
			// Recreate the layout with no max_width to be sure that we
			// don't break before or inside the hyphenate character
			hyphenated = true
			layout.SetText(hyphenatedFirstLineText, false)
			layout.setWidth(-1)
			firstLine, _, _ = layout.FirstLine()
			resumeIndex, hasResume = len(newFirstLineText), true
			if strings.HasPrefix(text, softHyphen) {
				resumeIndex += len(softHyphen)
			}
		}
	}

	if !hyphenated && strings.HasSuffix(firstLineText, softHyphen) {
		// Recreate the layout with no max_width to be sure that we don't
		// break inside the hyphenate-character string
		hyphenated = true
		hyphenatedFirstLineText = firstLineText + hyphenateCharacter
		layout.SetText(hyphenatedFirstLineText, false)
		layout.setWidth(-1)
		firstLine, _, _ = layout.FirstLine()
		resumeIndex, hasResume = len(firstLineText), true
	}

	// Step 5: Try to break word if it's too long for the line
	overflowWrap := style.Get("overflow_wrap")
	firstLineWidth, _ = lineSize(firstLine, style)
	space := *maxWidth - firstLineWidth
	// If we can break words and the first line is too long
	canBreak := isStr(style.Get("word_break"), "break-all") || (isLineStart &&
		(isStr(overflowWrap, "anywhere") || (isStr(overflowWrap, "break-word") && !minimum)))
	if space < 0 && canBreak {
		// Is it really OK to remove hyphenation for word-break ?
		hyphenated = false
		// TODO: Modify code to preserve W3C condition:
		// "Shaping characters are still shaped as if the word were not
		// broken"
		// The way new lines are processed in this function (one by one
		// with no memory of the last) prevents shaping characters (arabic,
		// for instance) from keeping their shape when wrapped on the next
		// line with pango layout. Maybe insert Unicode shaping characters
		// in text?
		layout.SetText(text, false)
		layout.setWidth(*maxWidth)
		layout.setWrapChar()
		var index int
		var hasIndex bool
		firstLine, index, hasIndex = layout.FirstLine()
		resumeIndex, hasResume = index, true
		if !hasIndex || index == 0 {
			resumeIndex = firstLine.Length
		}
		if resumeIndex >= len(text) {
			hasResume = false
		}
	}

	return firstLineMetrics(firstLine, text, layout, resumeIndex, hasResume, spaceCollapse, style, hyphenated, hyphenateCharacter)
}

// sliceAttrs returns attrs[start:end] of Python, with the bounds clamped.
func sliceAttrs(attrs []LogAttr, start, end int) []LogAttr {
	start = min(max(start, 0), len(attrs))
	end = min(max(end, start), len(attrs))
	return attrs[start:end]
}

// fontStyleCacheKey is _font_style_cache_key in
// weasyprint/text/line_break.py.
//
// Port: the style keeps the key, which upstream makes again for each line.
func fontStyleCacheKey(style *Style, includeSize bool) string {
	i, bit := 0, uint8(1)
	if includeSize {
		i, bit = 1, 2
	}
	if style.fontKeysSet&bit != 0 {
		return style.fontKeys[i]
	}
	key := fontKey(style)
	if includeSize {
		key += ValueString(style.Get("font_size")) + ValueString(style.Get("line_height"))
	}
	style.fontKeys[i] = key
	style.fontKeysSet |= bit
	return key
}

// Strut is strut in weasyprint/text/line_break.py. It returns the used
// value of line-height and the baseline, from the top edge of the line
// height. It satisfies TextMetrics.
func (fc *FontConfiguration) Strut(style *Style) (lineHeight, baseline float64) {
	fontSize := num(style.Get("font_size"))
	if fontSize == 0 {
		return 0, 0
	}

	key := fontStyleCacheKey(style, true)
	if result, ok := fc.strutLayouts[key]; ok {
		return result[0], result[1]
	}

	// Port: the spacing does not change the height of the line, and a
	// spacing in lh needs the strut. Upstream recurses until RecursionError
	// for letter-spacing: 1lh.
	measured := style.Copy()
	measured.Set("letter_spacing", String("normal"))
	measured.Set("word_spacing", Int(0))
	layout := newTextLayout(measured, 0, nil)
	layout.fc = fc
	layout.SetText(" ", false)
	line, _, _ := layout.FirstLine()
	metrics := firstLineMetrics(line, "", layout, 0, false, false, measured, false, "")
	textHeight, baseline := metrics.Height, metrics.Baseline
	value := style.Get("line_height")
	if isStr(value, "normal") {
		fc.strutLayouts[key] = [2]float64{textHeight, baseline}
		return textHeight, baseline
	}
	switch {
	case checkMath(value):
		lineHeight, _ = percentageFloat(value, style, fontSize)
	case isNumber(value):
		lineHeight = num(value) * fontSize
	default:
		if d, ok := value.(Dimension); ok {
			lineHeight = d.Value
		}
	}
	result := [2]float64{lineHeight, baseline + (lineHeight-textHeight)/2}
	fc.strutLayouts[key] = result
	return result[0], result[1]
}

// characterRatioKey is the key of the cache of character_ratio.
type characterRatioKey struct {
	unit string
	font string
}

// CharacterRatio is character_ratio in weasyprint/text/line_break.py. It
// returns the ratio of the font size that a unit uses. It satisfies
// TextMetrics.
func (fc *FontConfiguration) CharacterRatio(style *Style, unit string) float64 {
	var character string
	switch unit {
	case "ex":
		character = "x"
	case "cap":
		character = "O"
	case "ic":
		character = "水"
	case "ch":
		character = "0"
	default:
		// Port: upstream asserts.
		return 0.5
	}

	cacheKey := characterRatioKey{unit, fontStyleCacheKey(style, false)}
	var cache map[any]any
	if ctx := style.Context(); ctx != nil {
		if ctx.Cache == nil {
			ctx.Cache = map[any]any{}
		}
		cache = ctx.Cache
	}
	if ratio, ok := cache[cacheKey].(float64); ok {
		return ratio
	}

	// Avoid recursion for letter-spacing and word-spacing properties
	style = style.Copy()
	style.Set("letter_spacing", String("normal"))
	style.Set("word_spacing", Int(0))
	// Random big value
	style.Set("font_size", Int(1000))

	layout := newTextLayout(style, 0, nil)
	layout.fc = fc
	layout.SetText(character, false)
	line, _, _ := layout.FirstLine()

	var measure float64
	if unit == "ex" {
		measure = line.InkTop
	} else {
		// Port: upstream compares the character, and not the unit, with
		// 'cap', so cap also measures the logical width.
		measure = line.Width
	}

	// Zero means some kind of failure, fallback is 0.5.
	// We round to try keeping exact values that were altered by Pango.
	ratio := math.Round(measure/1000*1e5) / 1e5
	if ratio == 0 {
		ratio = 0.5
	}
	if cache != nil {
		cache[cacheKey] = ratio
	}
	return ratio
}

// getLogAttrs is get_log_attrs in weasyprint/text/line_break.py.
//
// Port: Pango takes the language. The rules of go-text do not depend on it.
func getLogAttrs(text, _ string) []LogAttr {
	// TODO: this should be removed when bidi is supported
	for _, char := range []string{"\u202a", "\u202b", "\u202c", "\u202d", "\u202e"} {
		text = strings.ReplaceAll(text, char, "\u200b")
	}
	return logAttrs(text)
}

// getNextBreakPoint is get_next_break_point in
// weasyprint/text/line_break.py. It returns the index of the first
// attribute that can break a line, and false for None.
func getNextBreakPoint(logAttrs []LogAttr) (int, bool) {
	for i, attr := range logAttrs {
		if attr.IsLineBreak {
			return i, true
		}
	}
	return 0, false
}

// getNextBreakPointFromText is get_next_break_point_from_text in
// weasyprint/text/line_break.py.
func getNextBreakPointFromText(text, lang string) (int, bool) {
	length := runeLen(text)
	if text == "" || length < 2 {
		return 0, false
	}
	logAttrs := getLogAttrs(text, lang)
	return getNextBreakPoint(sliceAttrs(logAttrs, 1, length))
}

// CanBreakText is can_break_text in weasyprint/text/line_break.py. It
// reports whether a line can break inside a text.
func CanBreakText(text, lang string) bool {
	_, ok := getNextBreakPointFromText(text, lang)
	return ok
}

// GetNextWordBoundaries is get_next_word_boundaries in
// weasyprint/text/line_break.py. It returns the start and the end, in
// runes, of the first word of a text, and false for None.
func GetNextWordBoundaries(text, lang string) (start, end int, ok bool) {
	if text == "" || runeLen(text) < 2 {
		return 0, 0, false
	}
	logAttrs := getLogAttrs(text, lang)
	wordStart := 0
	for i, attr := range logAttrs {
		if attr.IsWordEnd {
			return wordStart, i, true
		}
		if attr.IsWordBoundary {
			wordStart = i
		}
	}
	return 0, 0, false
}

// GetLastWordEnd is get_last_word_end in weasyprint/text/line_break.py. It
// returns the end, in runes, of the last word of a text, and false for
// None.
func GetLastWordEnd(text, lang string) (int, bool) {
	length := runeLen(text)
	if text == "" || length < 2 {
		return 0, false
	}
	logAttrs := getLogAttrs(text, lang)
	for i := len(logAttrs) - 1; i >= 0; i-- {
		if j := len(logAttrs) - 1 - i; j > 0 && logAttrs[i].IsWordEnd {
			return length - j, true
		}
	}
	return 0, false
}
