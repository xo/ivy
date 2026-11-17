package msdoc

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/xo/ivy"
)

// This file writes the sections of a document: each section on pages of its
// own size, with its headers and its footers (MS-DOC 2.4.4 and 2.8.11).

// The stories of the headers and the footers of a section, in the order of
// the PlcfHdd.
const (
	storyEvenHeader = iota
	storyOddHeader
	storyEvenFooter
	storyOddFooter
	storyFirstHeader
	storyFirstFooter
	storyCount
)

// storySeparators is the number of the stories of the separators of the
// notes at the start of the PlcfHdd.
const storySeparators = 6

// docSection is a section of the document: where it starts, its page, and
// the running elements of its headers and footers, by story, or "".
type docSection struct {
	cp      uint32
	name    string
	page    sectionPage
	running [storyCount]string
	// heights are the estimates of the heights of the headers and the
	// footers, in points, by story.
	heights [storyCount]float64
}

// docSections reads the sections of the document, with their pages and
// their headers and footers, and returns them with the HTML of the running
// elements of each section, by section.
func (p *parser) docSections(ctx context.Context, paper ivy.Paper) ([]docSection, []string, error) {
	plc, n := p.sedPlc()
	if n == 0 {
		return []docSection{{name: "s1", page: p.sectionPage(0, paper)}}, []string{""}, nil
	}
	secs := make([]docSection, n)
	for i := range secs {
		secs[i] = docSection{cp: u32(plc, 4*i), page: p.sectionPage(i, paper), name: "s" + strconv.Itoa(i+1)}
	}
	stories := p.headerStories(n)
	running := make([]string, n)
	// a story converts once, and the sections that share it share its
	// running element
	names := map[[2]uint32]string{}
	heights := map[[2]uint32]float64{}
	for i := range secs {
		var b strings.Builder
		for k, st := range stories[i] {
			if st[1] <= st[0] {
				continue
			}
			if name, ok := names[st]; ok {
				secs[i].running[k], secs[i].heights[k] = name, heights[st]
				continue
			}
			paras, err := p.paragraphsIn(ctx, st[0], st[1])
			if err != nil {
				return nil, nil, err
			}
			// the marks at the end of a story end it, and show nothing
			for len(paras) > 1 && len(paras[len(paras)-1].runs) == 0 && !paras[len(paras)-1].pap.inTable {
				paras = paras[:len(paras)-1]
			}
			name := "h" + strconv.Itoa(len(names)+1)
			names[st] = name
			pg := secs[i].page
			heights[st] = p.storyHeight(paras, float64(pg.width-pg.left-pg.right)/20)
			secs[i].running[k], secs[i].heights[k] = name, heights[st]
			b.WriteString(`<div style="position: running(` + name + `)">`)
			b.WriteString(p.body(paras, nil))
			b.WriteString("</div>\n")
		}
		running[i] = b.String()
	}
	return secs, running, nil
}

// headerStories returns the ranges of the characters of the six stories of
// the headers and the footers of each of the n sections. A section without
// a story takes the story of the section before it.
func (p *parser) headerStories(n int) [][storyCount][2]uint32 {
	out := make([][storyCount][2]uint32, n)
	plc := p.s.part(fcPlcfHdd)
	if p.s.word6 || len(plc) < 8 || p.s.fib.ccpHdd == 0 {
		return out
	}
	cps := len(plc) / 4
	base := p.s.fib.ccpText + p.s.fib.ccpFtn
	for i := range n {
		for k := range storyCount {
			j := storySeparators + storyCount*i + k
			if j+1 < cps {
				a, b := u32(plc, 4*j), u32(plc, 4*j+4)
				if a < b && b <= p.s.fib.ccpHdd {
					out[i][k] = [2]uint32{base + a, base + b}
					continue
				}
			}
			if i > 0 {
				out[i][k] = out[i-1][k]
			}
		}
	}
	return out
}

// storyHeight returns an estimate of the height of the paragraphs of a
// header or a footer, in points, on a column of width points: the lines of
// each paragraph, with its text wrapped at half the size of its font for
// each character, and its spacing. The rows of a table take the height of
// their highest cell.
func (p *parser) storyHeight(paras []paragraph, width float64) float64 {
	h, cell, row := 0.0, 0.0, 0.0
	for _, para := range paras {
		size := 10.0
		if para.mark.hps > 0 {
			size = float64(para.mark.hps) / 2
		}
		chars := 0
		for _, r := range para.runs {
			if r.chp.hps > 0 {
				size = max(size, float64(r.chp.hps)/2)
			}
			chars += len([]rune(r.text))
			if r.pic != nil {
				size = max(size, r.pic.height/1.15)
			}
		}
		line := size * 1.15
		switch {
		case para.pap.lineExact > 0:
			line = float64(para.pap.lineExact) / 20
		case para.pap.lineMult > 0:
			line *= para.pap.lineMult
		}
		lines := max(1, int(math.Ceil(float64(chars)*0.5*size/max(width, size))))
		ph := float64(lines)*line + float64(para.pap.before+para.pap.after)/20
		switch {
		case !para.pap.inTable:
			h += ph
		case para.pap.ttp && para.end == 0x07:
			h += max(row, cell)
			row, cell = 0, 0
		default:
			cell += ph
			if para.end == 0x07 {
				row, cell = max(row, cell), 0
			}
		}
	}
	return h + max(row, cell)
}

// sectionCSS returns the rules of the pages of the sections.
func (p *parser) sectionCSS(secs []docSection) string {
	var b strings.Builder
	for i, s := range secs {
		if i > 0 && s.name == secs[i-1].name {
			continue
		}
		pg := s.page
		box := func(k int) string {
			if s.running[k] == "" {
				return "none"
			}
			return "element(" + s.running[k] + ")"
		}
		rules := func(header, footer int) string {
			top := max(float64(pg.top)/20, float64(min(s.page.header, pg.top))/20+s.heights[header])
			bottom := max(float64(pg.bottom)/20, float64(min(s.page.footer, pg.bottom))/20+s.heights[footer])
			// the text keeps at least a fifth of the page
			limit := float64(pg.height) / 20 * 0.4
			top, bottom = min(top, max(float64(pg.top)/20, limit)), min(bottom, max(float64(pg.bottom)/20, limit))
			return fmt.Sprintf("margin-top: %spt; margin-bottom: %spt; "+
				"@top-center { content: %s; padding-top: %s; vertical-align: top; width: 100%%; text-align: left } "+
				"@bottom-center { content: %s; padding-bottom: %s; vertical-align: bottom; width: 100%%; text-align: left }",
				num(top), num(bottom), box(header), pt(min(s.page.header, pg.top)), box(footer), pt(min(s.page.footer, pg.bottom)))
		}
		fmt.Fprintf(&b, "@page %s { size: %s; margin: %s %s %s %s; %s }\n", s.name, pg.size(),
			pt(pg.top), pt(pg.right), pt(pg.bottom), pt(pg.left), rules(storyOddHeader, storyOddFooter))
		if p.facingPages() {
			fmt.Fprintf(&b, "@page %s:left { %s }\n", s.name, rules(storyEvenHeader, storyEvenFooter))
		}
		if s.page.titlePage {
			fmt.Fprintf(&b, "@page %s:nth(1 of %s) { %s }\n", s.name, s.name, rules(storyFirstHeader, storyFirstFooter))
		}
	}
	return b.String()
}

// nameSections gives a continuous section of the same page as the section
// before the name of that page, so that it stays on the page.
func nameSections(secs []docSection, sl sectionList) {
	for i := 1; i < len(secs); i++ {
		prev, s := secs[i-1], secs[i]
		if sl.continuous(s.cp) && prev.page.page == s.page.page && prev.running == s.running {
			secs[i].name = prev.name
		}
	}
}

// sectionOf returns the index of the section that holds the character cp.
func sectionOf(secs []docSection, cp uint32) int {
	i := sort.Search(len(secs), func(i int) bool { return secs[i].cp > cp })
	return max(i-1, 0)
}

// facingPages reports whether the even pages have headers and footers of
// their own: the flag fFacingPages of the Dop (MS-DOC 2.7.2).
func (p *parser) facingPages() bool {
	dop := p.s.part(fcDop)
	return !p.s.word6 && len(dop) > 0 && dop[0]&1 != 0
}
