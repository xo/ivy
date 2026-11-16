package drawpage

import (
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"

	"github.com/xo/ivy/internal/drawingml"
)

// The limits of the text of a shape.
const (
	// maxParagraphs is the number of paragraphs of a shape.
	maxParagraphs = 4096
	// maxRuns is the number of runs of a paragraph.
	maxRuns = 4096
	// maxListDepth is the depth of the lists of a shape.
	maxListDepth = 10
	// maxSpaces is the number of spaces of one text:s.
	maxSpaces = 1024
)

// textBase is what the text of a shape inherits: the styles of the shape,
// and the list style and the outline styles of a presentation object.
type textBase struct {
	// chain is the chain of the graphic or the presentation style of the
	// shape, the root first.
	chain []roxmltree.Node
	// outline is the name of the presentation styles of the levels of an
	// outline, without the number of the level, such as "Focus-outline".
	// It is empty for a shape that is not an outline.
	outline string
	// list is the list style of the shape.
	list    roxmltree.Node
	hasList bool
	// body is the style of the text body of the shape.
	body drawingml.BodyStyle
}

// textWalker reads the paragraphs of a shape.
type textWalker struct {
	b     *builder
	base  *textBase
	paras []drawingml.Paragraph
	// para and runs are the paragraph that the walker reads.
	runs []drawingml.Run
	// size counts the bytes of text.
	size int
}

// textBody reads the text of the element n: its text:p, text:h and
// text:list children.
func (b *builder) textBody(n roxmltree.Node, base *textBase) *drawingml.TextBody {
	w := &textWalker{b: b, base: base}
	w.block(n, nil, 0, base.list, base.hasList)
	if len(w.paras) == 0 {
		return nil
	}
	tb := &drawingml.TextBody{Body: base.body, Paragraphs: w.paras}
	numberParagraphs(tb.Paragraphs)
	return tb
}

// numberParagraphs counts the numbers of the paragraphs with automatic
// bullets, as drawingml does.
func numberParagraphs(paras []drawingml.Paragraph) {
	var counts [maxListDepth]int
	for i := range paras {
		p := &paras[i]
		lvl := min(max(p.Level, 0), maxListDepth-1)
		for j := lvl + 1; j < len(counts); j++ {
			counts[j] = 0
		}
		if !p.Style.Bullet.OK || p.Style.Bullet.V.Kind != drawingml.BulletNumber {
			continue
		}
		counts[lvl]++
		p.Number = p.Style.Bullet.V.Start + counts[lvl] - 1
	}
}

// block reads the paragraphs and the lists among the children of n. level
// is the depth of the lists around them, from 0 outside a list. item is the
// level style of the list item around them, or nil.
func (w *textWalker) block(n roxmltree.Node, item *roxmltree.Node, level int, list roxmltree.Node, hasList bool) {
	for c := range n.Children() {
		if !c.IsElement() || len(w.paras) >= maxParagraphs {
			continue
		}
		uri, _ := c.TagName().Namespace()
		if uri != nsText {
			continue
		}
		switch c.TagName().Name() {
		case "p", "h":
			w.paragraph(c, item, level)
			item = nil
		case "list":
			if level >= maxListDepth {
				continue
			}
			l, ok := list, hasList
			if name := at(c, nsText, "style-name"); name != "" {
				if s, found := w.b.st.lists[name]; found {
					l, ok = s, true
				}
			}
			for li := range c.Children() {
				if !li.IsElement() {
					continue
				}
				switch li.TagName().Name() {
				case "list-item":
					var style *roxmltree.Node
					if ok {
						if s, found := levelStyle(l, level+1); found {
							style = &s
						}
					}
					w.block(li, style, level+1, l, ok)
				case "list-header":
					w.block(li, nil, level+1, l, ok)
				}
			}
		case "section", "soft-page-break":
			w.block(c, item, level, list, hasList)
		}
	}
}

// levelStyle returns the style of the level of a list style, from 1.
func levelStyle(list roxmltree.Node, level int) (roxmltree.Node, bool) {
	var last roxmltree.Node
	found := false
	for e := range list.Children() {
		if !e.IsElement() {
			continue
		}
		v, err := strconv.Atoi(at(e, nsText, "level"))
		if err != nil {
			continue
		}
		if v == level {
			return e, true
		}
		if v < level {
			last, found = e, true
		}
	}
	return last, found
}

// paragraph reads text:p or text:h. level is the depth of the list around
// it, and item is the style of the level of its list item, or nil.
func (w *textWalker) paragraph(n roxmltree.Node, item *roxmltree.Node, level int) {
	st := w.b.st
	chain := w.base.chain
	if w.base.outline != "" && level > 0 {
		if _, ok := st.style("presentation", w.base.outline+strconv.Itoa(min(level, 9))); ok {
			chain = st.chain("presentation", w.base.outline+strconv.Itoa(min(level, 9)))
		}
	}
	pchain := st.chain("paragraph", at(n, nsText, "style-name"))
	// the default style of the paragraphs does not apply over the style of
	// the shape
	if len(pchain) > 0 {
		if _, ok := st.defaults["paragraph"]; ok {
			pchain = pchain[1:]
		}
	}
	pp := st.props(pchain, "paragraph-properties", st.props(chain, "paragraph-properties", nil))
	tp := st.props(pchain, "text-properties", st.props(chain, "text-properties", nil))
	style := w.b.paraStyle(pp)
	style.Run = w.b.runStyle(tp)
	lvl := 0
	if level > 0 {
		lvl = level - 1
	}
	if item != nil {
		w.b.bullet(&style, *item)
	}
	w.runs = w.runs[:0:0]
	w.inline(n, tp, 0)
	if len(w.paras) < maxParagraphs {
		w.paras = append(w.paras, drawingml.Paragraph{Style: style, Level: lvl, Runs: w.runs, End: style.Run})
	}
	w.runs = nil
}

// inline reads the text and the spans of n, with the text properties tp.
func (w *textWalker) inline(n roxmltree.Node, tp props, depth int) {
	if depth > maxListDepth*2 {
		return
	}
	style := w.b.runStyle(tp)
	for c := range n.Children() {
		if len(w.runs) >= maxRuns || w.size > 1<<20 {
			return
		}
		if c.IsText() {
			t, _ := c.Text()
			w.text(collapse(t), style)
			continue
		}
		if !c.IsElement() {
			continue
		}
		uri, _ := c.TagName().Namespace()
		switch uri {
		case nsText:
		case nsPresentation:
			// the fields of a master, such as presentation:footer, take
			// the text of the page, which the package does not read
			continue
		default:
			continue
		}
		switch c.TagName().Name() {
		case "span", "a", "meta", "ruby-base":
			sub := tp
			if name := at(c, nsText, "style-name"); name != "" {
				chain := w.b.st.chain("text", name)
				if len(chain) > 0 {
					if _, ok := w.b.st.defaults["text"]; ok {
						chain = chain[1:]
					}
				}
				sub = w.b.st.props(chain, "text-properties", clone(tp))
			}
			w.inline(c, sub, depth+1)
		case "s":
			count := 1
			if v, err := strconv.Atoi(at(c, nsText, "c")); err == nil {
				count = min(max(v, 1), maxSpaces)
			}
			w.text(strings.Repeat(" ", count), style)
		case "tab":
			w.text("\t", style)
		case "line-break":
			w.runs = append(w.runs, drawingml.Run{Break: true, Style: style})
		case "page-number":
			if w.b.number > 0 {
				w.text(strconv.Itoa(w.b.number), style)
			} else {
				w.inline(c, tp, depth+1)
			}
		case "note", "annotation", "bookmark", "bookmark-start", "bookmark-end", "reference-mark":
		default:
			// a field, such as text:date, keeps the text that it holds
			w.inline(c, tp, depth+1)
		}
	}
}

// text adds a run of text.
func (w *textWalker) text(t string, style drawingml.RunStyle) {
	if t == "" {
		return
	}
	w.size += len(t)
	w.runs = append(w.runs, drawingml.Run{Text: t, Style: style})
}

// collapse collapses the white space of the text of OpenDocument: a run of
// spaces, tabs and line feeds is one space (ODF 1.3, 6.1.2).
func collapse(s string) string {
	if !strings.ContainsAny(s, "\t\n\r") && !strings.Contains(s, "  ") {
		return s
	}
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// clone returns a copy of p.
func clone(p props) props {
	q := make(props, len(p))
	for k, v := range p {
		q[k] = v
	}
	return q
}

// runStyle returns the style of a run from the text properties tp.
func (b *builder) runStyle(tp props) drawingml.RunStyle {
	var s drawingml.RunStyle
	if v, ok := length(tp.get("fo:font-size")); ok && v > 0 {
		s.Size.Set(min(v, 4000))
	}
	if name := tp.get("style:font-name"); name != "" {
		if f, ok := b.st.fonts[name]; ok && f != "" {
			s.Latin.Set(f)
		} else {
			s.Latin.Set(name)
		}
	} else if f := tp.get("fo:font-family"); f != "" {
		s.Latin.Set(unquote(strings.Split(f, ",")[0]))
	}
	switch w := tp.get("fo:font-weight"); w {
	case "":
	case "bold", "bolder", "600", "700", "800", "900":
		s.Bold.Set(true)
	default:
		s.Bold.Set(false)
	}
	switch tp.get("fo:font-style") {
	case "italic", "oblique":
		s.Italic.Set(true)
	case "normal":
		s.Italic.Set(false)
	}
	if u := tp.get("style:text-underline-style"); u != "" {
		s.Underline.Set(u != "none")
	}
	if l := tp.get("style:text-line-through-style"); l != "" {
		s.Strike.Set(l != "none")
	}
	switch {
	case tp.get("fo:color") != "":
		if c, ok := drawingml.ParseHex(tp.get("fo:color")); ok {
			col := drawingml.RGBAFrom(c)
			if o, ok := percent(tp.get("loext:opacity")); ok && o > 0 {
				col = col.WithAlpha(o)
			}
			s.Paint.Set(drawingml.Solid(col))
		}
	case tp.get("style:use-window-font-color") == "true":
		s.Paint.Set(drawingml.Solid(drawingml.Black))
	}
	switch {
	case tp.get("fo:text-transform") == "uppercase":
		s.Caps.Set("all")
	case tp.get("fo:font-variant") == "small-caps":
		s.Caps.Set("small")
	}
	if pos := strings.Fields(tp.get("style:text-position")); len(pos) > 0 {
		switch pos[0] {
		case "super":
			s.Baseline.Set(0.33)
		case "sub":
			s.Baseline.Set(-0.33)
		default:
			if v, ok := percent(pos[0]); ok {
				s.Baseline.Set(v)
			}
		}
	}
	if v, ok := length(tp.get("fo:letter-spacing")); ok {
		s.Spacing.Set(min(max(v, -100), 100))
	}
	if bg := tp.get("fo:background-color"); bg != "" && bg != "transparent" {
		if c, ok := drawingml.ParseHex(bg); ok {
			s.Highlight.Set(drawingml.RGBAFrom(c))
		}
	}
	return s
}

// paraStyle returns the style of a paragraph from the paragraph properties
// pp.
func (b *builder) paraStyle(pp props) drawingml.ParaStyle {
	var s drawingml.ParaStyle
	switch pp.get("fo:text-align") {
	case "start", "left":
		s.Align.Set("l")
	case "center":
		s.Align.Set("ctr")
	case "end", "right":
		s.Align.Set("r")
	case "justify":
		s.Align.Set("just")
	}
	if v, ok := length(pp.get("fo:margin-left")); ok {
		s.MarL.Set(v)
	}
	if v, ok := length(pp.get("fo:margin-right")); ok {
		s.MarR.Set(v)
	}
	if v, ok := length(pp.get("fo:text-indent")); ok {
		s.Indent.Set(v)
	}
	if v, ok := length(pp.get("fo:margin-top")); ok {
		s.SpaceBefore.Set(drawingml.Spacing{Points: true, Pt: max(v, 0)})
	}
	if v, ok := length(pp.get("fo:margin-bottom")); ok {
		s.SpaceAfter.Set(drawingml.Spacing{Points: true, Pt: max(v, 0)})
	}
	lh := pp.get("fo:line-height")
	if v, ok := percent(lh); ok {
		s.LineSpacing.Set(drawingml.Spacing{Pct: min(max(v, 0.1), 10)})
	} else if v, ok := length(lh); ok {
		s.LineSpacing.Set(drawingml.Spacing{Points: true, Pt: max(v, 0)})
	}
	if strings.HasPrefix(pp.get("style:writing-mode"), "rl") {
		s.RTL.Set(true)
	}
	return s
}

// bullet sets the bullet and the indents of a paragraph in a list from the
// style of its level, such as text:list-level-style-bullet.
func (b *builder) bullet(s *drawingml.ParaStyle, level roxmltree.Node) {
	var bu drawingml.Bullet
	switch level.TagName().Name() {
	case "list-level-style-bullet":
		bu = drawingml.Bullet{Kind: drawingml.BulletChar, Char: at(level, nsText, "bullet-char")}
		if bu.Char == "" {
			bu.Char = "•"
		}
	case "list-level-style-number":
		bu = drawingml.Bullet{Kind: drawingml.BulletNumber, Scheme: numberScheme(level), Start: 1}
		if v, err := strconv.Atoi(at(level, nsText, "start-value")); err == nil {
			bu.Start = min(max(v, 1), 32767)
		}
		if bu.Scheme == "" {
			bu.Kind = drawingml.BulletNone
		}
	case "list-level-style-image":
		bu = drawingml.Bullet{Kind: drawingml.BulletPicture}
	default:
		return
	}
	s.Bullet.Set(bu)
	if p, ok := firstChild(level, nsStyle, "list-level-properties"); ok {
		if at(p, nsText, "list-level-position-and-space-mode") == "label-alignment" {
			if a, ok := firstChild(p, nsStyle, "list-level-label-alignment"); ok {
				if v, ok := length(at(a, nsFO, "margin-left")); ok {
					s.MarL.Set(v)
				}
				if v, ok := length(at(a, nsFO, "text-indent")); ok {
					s.Indent.Set(v)
				}
			}
		} else {
			before, _ := length(at(p, nsText, "space-before"))
			width, _ := length(at(p, nsText, "min-label-width"))
			s.MarL.Set(max(before+width, 0))
			s.Indent.Set(-max(width, 0))
		}
	}
	if t, ok := firstChild(level, nsStyle, "text-properties"); ok {
		if v, ok := percent(at(t, nsFO, "font-size")); ok && v > 0 {
			s.BulletSize.Set(&drawingml.BulletSize{Pct: min(max(v, 0.25), 4)})
		} else if v, ok := length(at(t, nsFO, "font-size")); ok && v > 0 {
			s.BulletSize.Set(&drawingml.BulletSize{Points: true, Pt: v})
		}
		if c, ok := drawingml.ParseHex(at(t, nsFO, "color")); ok && at(t, nsStyle, "use-window-font-color") != "true" {
			col := drawingml.RGBAFrom(c)
			s.BulletColor.Set(&col)
		}
	}
}

// numberScheme returns the scheme of DrawingML of a level style of numbers,
// such as "arabicPeriod", or "" for a level without numbers.
func numberScheme(level roxmltree.Node) string {
	var base string
	switch at(level, nsStyle, "num-format") {
	case "1":
		base = "arabic"
	case "a":
		base = "alphaLc"
	case "A":
		base = "alphaUc"
	case "i":
		base = "romanLc"
	case "I":
		base = "romanUc"
	default:
		return ""
	}
	prefix, suffix := at(level, nsStyle, "num-prefix"), at(level, nsStyle, "num-suffix")
	switch {
	case strings.HasSuffix(suffix, ")") && strings.HasPrefix(prefix, "("):
		return base + "ParenBoth"
	case strings.HasSuffix(suffix, ")"):
		return base + "ParenR"
	case suffix == "":
		return base + "Plain"
	}
	return base + "Period"
}
