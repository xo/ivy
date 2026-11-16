package msppt

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

// styleSheet is the style sheet of each slide. The sizes of the fonts are
// those of the default master of PowerPoint, a little smaller, because the
// package does not read the sizes of the runs of the text.
const styleSheet = `html {
  font-family: "Calibri", "Arial", sans-serif;
  font-size: 18pt;
}
body {
  margin: 0;
}
.shape {
  position: absolute;
  overflow: visible;
  margin: 0;
  padding: 3.6pt 7.2pt;
  box-sizing: border-box;
}
p, ul {
  margin: 0;
}
ul {
  padding-left: 1.2em;
}
.title {
  font-size: 36pt;
}
.center-title {
  font-size: 40pt;
  text-align: center;
}
.body {
  font-size: 22pt;
}
.center-body {
  font-size: 24pt;
  text-align: center;
}
.small-body {
  font-size: 16pt;
}
.flow {
  padding: 36pt;
}
`

// pt returns a length of master units, 576 to the inch, in points.
func pt(v float64) string {
	return strconv.FormatFloat(v/8, 'f', 2, 64) + "pt"
}

// slideHTML returns the head and the body of the HTML document of a slide
// of width by height master units.
func slideHTML(sl slide, width, height float64) (string, string) {
	head := fmt.Sprintf("<style>\n@page {\n  size: %s %s;\n  margin: 0;\n}\n</style>\n", pt(width), pt(height))
	var b strings.Builder
	anchored := false
	for _, sh := range sl.shapes {
		if !sh.hasAnchor {
			continue
		}
		anchored = true
		a := sh.anchor
		left := clamp(a.left, -width, 2*width)
		top := clamp(a.top, -height, 2*height)
		w := clamp(a.right-a.left, 0, 2*width)
		fmt.Fprintf(&b, `<div class="shape %s" style="left: %s; top: %s; width: %s">`,
			className(sh.kind), pt(left), pt(top), pt(w))
		writeText(&b, sh.text)
		b.WriteString("</div>\n")
	}
	if anchored {
		return head, b.String()
	}
	// a slide whose shapes have no anchor: its texts in order
	texts := sl.texts
	if len(texts) == 0 {
		for _, sh := range sl.shapes {
			texts = append(texts, sh.text)
		}
	}
	b.WriteString(`<div class="flow">`)
	for _, t := range texts {
		if t.kind == txNotes {
			continue
		}
		fmt.Fprintf(&b, `<div class="%s">`, className(t.kind))
		writeText(&b, t)
		b.WriteString("</div>\n")
	}
	b.WriteString("</div>\n")
	return head, b.String()
}

// clamp returns v within [lo, hi].
func clamp(v, lo, hi float64) float64 {
	return max(lo, min(v, hi))
}

// className returns the class of the style of a text of the type kind.
func className(kind uint32) string {
	switch kind {
	case txTitle:
		return "title"
	case txCenterTitle:
		return "center-title"
	case txBody, txHalfBody:
		return "body"
	case txQuarterBody:
		return "small-body"
	case txCenterBody:
		return "center-body"
	}
	return "other"
}

// writeText writes the paragraphs of a text. The paragraphs of a body are
// the items of a list with bullets.
func writeText(b *strings.Builder, t text) {
	paras := strings.Split(strings.TrimRight(t.text, "\r"), "\r")
	list := t.kind == txBody || t.kind == txHalfBody || t.kind == txQuarterBody
	if list {
		b.WriteString("<ul>")
	}
	for _, p := range paras {
		p = strings.Map(func(r rune) rune {
			switch {
			case r == 0x0B:
				return '\n'
			case r == '\t':
				return ' '
			case r < 0x20 || r == 0xFFFE || r == 0xFFFF:
				return -1
			}
			return r
		}, p)
		text := strings.ReplaceAll(html.EscapeString(p), "\n", "<br>")
		if list {
			if strings.TrimSpace(p) == "" {
				continue
			}
			b.WriteString("<li>" + text + "</li>")
			continue
		}
		if text == "" {
			text = "&#160;"
		}
		b.WriteString("<p>" + text + "</p>")
	}
	if list {
		b.WriteString("</ul>")
	}
}
