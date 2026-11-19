// This file is not a port. It tests the page floats of
// -ivy-float-position (D60).

package layout

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// pageFloatStyle is the style of the tests of D60. A page of 300px has
// margins of 50px, so its page area is 200px square and holds 10 lines of
// 20px. A float is 100px wide and 60px high.
const pageFloatStyle = `
      <style>
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        @page { size: 300px 300px; margin: 50px }
        body { margin: 0; font: 20px/20px weasyprint }
        p { margin: 0; orphans: 1; widows: 1 }
        .f { float: left; width: 100px; height: 60px }
      </style>`

// paragraphs returns n paragraphs of one line each, with the texts Pfrom
// to P(from+n-1).
func paragraphs(from, n int) string {
	var b strings.Builder
	for i := from; i < from+n; i++ {
		fmt.Fprintf(&b, "<p>P%d</p>", i)
	}
	return b.String()
}

// renderPageFloats lays out a document and returns its pages and the
// number of layouts of pages.
func renderPageFloats(t *testing.T, source string) ([]*Box, int) {
	t.Helper()
	p := parseBase(t, source, testBaseURL, nil)
	root, err := p.builder.buildFormattingStructure(p.html.EtreeElement)
	if err != nil {
		t.Fatal(err)
	}
	doc := NewDocument(p.html, root, p.styleFor, DocumentOptions{
		FontConfig:      p.fc,
		TargetCollector: p.builder.targetCollector,
		CounterStyle:    p.builder.counterStyle,
		Footnotes:       p.builder.footnotes,
	})
	pages, err := doc.All(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return pages, doc.lc.pageFloats.layouts
}

// describePages describes the pages: for each page, the floats of the
// class f as F<id>@x,y of their border boxes, and the text boxes that
// start with P as <text>@x,y, in the order of the tree.
func describePages(pages []*Box) string {
	out := make([]string, 0, len(pages))
	for _, page := range pages {
		var items []string
		page.Descendants(func(d *Box) bool {
			switch {
			case d.IsFloated() && d.Element != nil && d.Element.GetDefault("class", "") == "f" && !d.IsTable():
				items = append(items, fmt.Sprintf("F%s@%g,%g", d.Element.GetDefault("id", ""), d.BorderBoxX(), d.BorderBoxY()))
			case d.Kind == KindText && strings.HasPrefix(d.Text, "P"):
				items = append(items, fmt.Sprintf("%s@%g,%g", d.Text, d.PositionX, d.PositionY))
			}
			return true
		})
		out = append(out, strings.Join(items, " "))
	}
	return strings.Join(out, " | ")
}

// pageFloat returns a float of the class f with the id id and the style
// -ivy-float-position: value, and more style.
func pageFloatDiv(id, value, more string) string {
	return `<div class=f id=` + id + ` style="-ivy-float-position: ` + value + `; ` + more + `"></div>`
}

func TestFloatPositionValidation(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  Value
	}{
		{"auto", String("auto")},
		{"margin 1in", Tuple{String("margin"), Float(96)}},
		{"10px page", Tuple{String("page"), Float(10)}},
		{"page -10px", Tuple{String("page"), Float(-10)}},
		{"margin 0", Tuple{String("margin"), Float(0)}},
		{"bottom margin", Tuple{String("margin"), String("bottom")}},
		{"page center", Tuple{String("page"), String("center")}},
		{"margin top", Tuple{String("margin"), String("top")}},
		// The values that are not valid keep the initial value.
		{"margin", String("auto")},
		{"10px", String("auto")},
		{"margin page", String("auto")},
		{"margin 10%", String("auto")},
		{"margin top 10px", String("auto")},
		{"column 10px", String("auto")},
		{"top bottom", String("auto")},
	} {
		t.Run(tt.value, func(t *testing.T) {
			page := renderPages(t, `<style>@page { size: 100px } div { -ivy-float-position: `+tt.value+` }</style><div></div>`)[0]
			var got Value
			page.Descendants(func(d *Box) bool {
				if d.ElementTag == "div" {
					got = d.Style.Get("float_position")
				}
				return got == nil
			})
			if !Equal(got, tt.want) {
				t.Errorf("-ivy-float-position: %s = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
	// The property is not inherited.
	page := renderPages(t, `<div style="-ivy-float-position: margin 1px"><p>a</p></div>`)[0]
	page.Descendants(func(d *Box) bool {
		if d.ElementTag == "p" && !isStr(d.Style.Get("float_position"), "auto") {
			t.Errorf("p has -ivy-float-position: %v, want auto", d.Style.Get("float_position"))
		}
		return true
	})
}

func TestPageFloatPosition(t *testing.T) {
	// The anchor of the float is after P1, at y = 70. The page area is
	// from y = 50 to y = 250, and the page from 0 to 300.
	for _, tt := range []struct {
		value, more string
		y           float64
		layouts     int
	}{
		{"margin 20px", "", 70, 1},
		{"margin 0", "", 50, 2},
		{"margin -10px", "", 40, 2},
		{"margin 100px", "", 150, 1},
		{"page 20px", "", 20, 2},
		{"page 120px", "", 120, 1},
		{"margin top", "", 50, 2},
		{"margin center", "", 120, 1},
		{"margin bottom", "", 190, 1},
		{"page top", "", 0, 2},
		{"page center", "", 120, 1},
		{"page bottom", "", 240, 1},
		// A float that passes the bottom of its area moves up until it
		// fits.
		{"margin 180px", "", 190, 1},
		{"page 280px", "", 240, 1},
		// A float that is taller than its area moves up to the top of the
		// area, and not above it. So does a float at the bottom or at the
		// center of its area.
		{"margin 100px", "height: 300px", 50, 2},
		{"page bottom", "height: 400px", 0, 2},
		{"margin center", "height: 300px", 50, 2},
		// The margins of the float stay its distances from the text, and
		// the position is the one of its border box.
		{"margin 100px", "margin: 10px", 150, 1},
	} {
		t.Run(tt.value+" "+tt.more, func(t *testing.T) {
			pages, layouts := renderPageFloats(t, pageFloatStyle+paragraphs(1, 1)+
				pageFloatDiv("a", tt.value, tt.more)+paragraphs(2, 3))
			want := fmt.Sprintf("Fa@50,%g", tt.y)
			if strings.Contains(tt.more, "margin") {
				want = fmt.Sprintf("Fa@60,%g", tt.y)
			}
			if got := describePages(pages); !strings.HasPrefix(got, want+" ") && !strings.Contains(got, " "+want+" ") {
				t.Errorf("pages = %s, want %s", got, want)
			}
			if layouts != tt.layouts {
				t.Errorf("%d layouts, want %d", layouts, tt.layouts)
			}
		})
	}
}

func TestPageFloat(t *testing.T) {
	for _, tt := range []struct {
		name    string
		body    string
		want    string
		layouts int
	}{{
		// The float is above its anchor. The page is made again with the
		// float placed before the flow, so the text before the anchor
		// wraps around it, and the flow skips the float at its anchor.
		"above the anchor",
		paragraphs(1, 6) + pageFloatDiv("a", "margin 20px", "") + paragraphs(7, 2),
		"Fa@50,70 P1@50,50 P2@150,70 P3@150,90 P4@150,110 P5@50,130 P6@50,150 P7@50,170 P8@50,190",
		2,
	}, {
		// The float is below its anchor. It covers no line, and the page
		// has one layout.
		"below the anchor",
		paragraphs(1, 1) + pageFloatDiv("a", "margin bottom", "") + paragraphs(2, 9),
		"P1@50,50 Fa@50,190 P2@50,70 P3@50,90 P4@50,110 P5@50,130 P6@50,150 P7@50,170 P8@150,190 P9@150,210 P10@150,230",
		1,
	}, {
		// The float is on the page of its anchor.
		"next page",
		paragraphs(1, 15) + pageFloatDiv("a", "margin 20px", "") + paragraphs(16, 2),
		"P1@50,50 P2@50,70 P3@50,90 P4@50,110 P5@50,130 P6@50,150 P7@50,170 P8@50,190 P9@50,210 P10@50,230 | " +
			"Fa@50,70 P11@50,50 P12@150,70 P13@150,90 P14@150,110 P15@50,130 P16@50,150 P17@50,170",
		3,
	}, {
		// A float at the right keeps its side.
		"right",
		paragraphs(1, 4) + pageFloatDiv("a", "margin 0", "float: right") + paragraphs(5, 1),
		"Fa@150,50 P1@50,50 P2@50,70 P3@50,90 P4@50,110 P5@50,130",
		2,
	}, {
		// Two floats at the same place cover each other. The layout does
		// not move a page float for the other floats.
		"two floats",
		paragraphs(1, 4) + pageFloatDiv("a", "margin 20px", "") + paragraphs(5, 1) +
			pageFloatDiv("b", "margin 20px", "") + paragraphs(6, 1),
		"Fa@50,70 Fb@50,70 P1@50,50 P2@150,70 P3@150,90 P4@150,110 P5@50,130 P6@50,150",
		2,
	}, {
		// An ordinary float after a page float that is lower on the page
		// does not move down to it.
		"ordinary float",
		paragraphs(1, 1) + pageFloatDiv("a", "margin bottom", "") +
			`<div class=f id=o style="width: 50px; height: 20px"></div>` + paragraphs(2, 1),
		"P1@50,50 Fa@50,190 Fo@50,70 P2@100,70",
		1,
	}, {
		// The float pushes its anchor to the next page. The float stays on
		// its page, and the anchor goes on. The next page does not place
		// the float again.
		"anchor on the next page",
		paragraphs(1, 9) + pageFloatDiv("a", "margin 0", "width: 200px") + paragraphs(10, 1),
		"Fa@50,50 P1@50,110 P2@50,130 P3@50,150 P4@50,170 P5@50,190 P6@50,210 P7@50,230 | P8@50,50 P9@50,70 P10@50,90",
		3,
	}, {
		// A float in a line is placed after the line, so it is above the
		// flow when it covers a line before.
		"in a line",
		paragraphs(1, 5) + `<p>P6 <span class=f id=a style="-ivy-float-position: margin 20px"></span></p>` + paragraphs(7, 1),
		"Fa@50,70 P1@50,50 P2@150,70 P3@150,90 P4@150,110 P5@50,130 P6@50,150 P7@50,170",
		2,
	}, {
		// A table floats as its wrapper, which takes the property.
		"table",
		paragraphs(1, 3) + `<table class=f id=a style="-ivy-float-position: margin 0; height: 60px"><tr><td></td></tr></table>` +
			paragraphs(4, 1),
		"Fa@50,50 P1@150,50 P2@150,70 P3@150,90 P4@50,110",
		2,
	}, {
		// In a table cell, the property does nothing.
		"table cell",
		paragraphs(1, 1) + `<table style="width: 200px; border-spacing: 0"><tr><td style="padding: 0">` +
			pageFloatDiv("a", "margin 100px", "") + `P2</td></tr></table>`,
		"P1@50,50 Fa@50,70 P2@150,70",
		1,
	}, {
		// In a column, the property does nothing. The columns balance, so
		// the float and P2 are at the top of the second column.
		"column",
		`<div style="columns: 2; column-gap: 0">` + paragraphs(1, 1) + pageFloatDiv("a", "margin 100px", "width: 50px") +
			paragraphs(2, 1) + `</div>`,
		"P1@50,50 Fa@150,50 P2@200,50",
		1,
	}, {
		// In a float, the property does nothing.
		"float",
		paragraphs(1, 1) + `<div style="float: left; width: 150px">` + pageFloatDiv("a", "margin 100px", "") + `P2</div>`,
		"P1@50,50 Fa@50,70 P2@150,70",
		1,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			pages, layouts := renderPageFloats(t, pageFloatStyle+tt.body)
			if got := describePages(pages); got != tt.want {
				t.Errorf("pages =\n%s\nwant\n%s", got, tt.want)
			}
			if layouts != tt.layouts {
				t.Errorf("%d layouts, want %d", layouts, tt.layouts)
			}
		})
	}
}

// dumpTexts returns the text boxes of each page, with their places, in the
// order of their places.
func dumpTexts(pages []*Box) string {
	out := make([]string, 0, len(pages))
	for _, page := range pages {
		var items []string
		page.descendants(true, func(d *Box) bool {
			if d.Kind == KindText && strings.TrimSpace(d.Text) != "" {
				items = append(items, fmt.Sprintf("%08.2f %08.2f %s", d.PositionY, d.PositionX, d.Text))
			}
			return true
		})
		slices.Sort(items)
		out = append(out, strings.Join(items, "\n"))
	}
	return strings.Join(out, "\n|\n")
}

func TestPageFloatState(t *testing.T) {
	// The second page has a footnote, a running element with a named
	// string, the page counter, line numbers, a float that the first page
	// cut, an empty line that passes the bottom of the page by its descent,
	// and a space before at the top of the page. The page float of the
	// anchor in P14 is above the flow, so the page is made again. The page
	// float of the anchor in P9, the first line of the page, is at the
	// bottom of the line, so the page has one layout. The pages must be the
	// same.
	style := pageFloatStyle + `
      <style>
        body { -ivy-page-overflow: descent; -ivy-page-top-space: discard; -ivy-line-numbers: 10px 1 1 continuous }
        p { margin-top: 5px }
        .n { float: footnote; font-size: 10px; line-height: 10px }
        h2 { position: running(head); string-set: title content(); font: inherit; margin: 0 }
        .big { float: right; width: 40px }
        @page { @top-center { content: element(head) } @bottom-center { content: counter(page) " " string(title) } }
      </style>`
	anchor := `<span class=f id=a style="-ivy-float-position: margin 25px"></span>`
	body := func(at int) string {
		var b strings.Builder
		b.WriteString(`<div class=big>` + strings.Repeat("x<br>", 14) + `</div>`)
		for i := 1; i <= 18; i++ {
			switch i {
			case 12:
				b.WriteString(`<h2>Head</h2><p>P12 <span class=n>Note</span></p>`)
				continue
			case 16:
				b.WriteString(`<p> </p>`)
			}
			fmt.Fprintf(&b, "<p>P%d", i)
			if i == at {
				b.WriteString(" " + anchor)
			}
			b.WriteString("</p>")
		}
		return b.String()
	}
	early, earlyLayouts := renderPageFloats(t, style+body(9))
	late, lateLayouts := renderPageFloats(t, style+body(14))
	want, got := dumpTexts(early), dumpTexts(late)
	if got != want {
		t.Errorf("pages with the anchor in P14 =\n%s\nwant\n%s", got, want)
	}
	if strings.Count(got, "Note") != 1 || strings.Count(got, "Head") != 2 || strings.Count(got, " x\n") != 14 {
		t.Errorf("pages do not have one footnote, the running element on pages 2 and 3, and the 14 lines of the cut float:\n%s", got)
	}
	if lateLayouts != earlyLayouts+1 {
		t.Errorf("%d layouts with the anchor in P14, want %d", lateLayouts, earlyLayouts+1)
	}
	if want := "Fa@50,75 P9@50,55 P10@150,80"; !strings.Contains(describePages(late), want) {
		t.Errorf("pages =\n%s\nwant %s", describePages(late), want)
	}
}

func TestPageFloatGalley(t *testing.T) {
	// The pages with page floats above and below their anchors are the same
	// with the galley and without it. The parts of the galley place the
	// page floats as ordinary floats, and the pages take the lines below
	// the floats from the galley.
	style := `.f { float: left; width: 80px; height: 50px } .r { float: right; width: 60px; height: 90px }`
	positions := []string{"margin 20px", "page 10px", "margin bottom", "margin center", "page bottom"}
	body := galleyParagraphs(60, func(i int) string {
		switch {
		case i%7 == 3:
			return `<span class="f" style="-ivy-float-position: ` + positions[i%len(positions)] + `"></span>`
		case i%11 == 5:
			return `</p><div class="r" style="-ivy-float-position: ` + positions[(i+2)%len(positions)] + `"></div><p>`
		}
		return ""
	})
	checkGalley(t, galleyDocument(style, body), true)
}
