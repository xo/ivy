// This file is not a port. It tests the floats of -ivy-wrap-flow (D63).

package layout

import (
	"fmt"
	"strings"
	"testing"
)

// wrapFlowStyle is the style of the tests of D63. A page of 300px has
// margins of 50px, so its page area is from x = 50 to x = 250 and holds 10
// lines of 20px. A letter is 20px wide. A float of the class f is 40px
// square.
const wrapFlowStyle = `
      <style>
        @font-face { src: url(weasyprint.otf); font-family: weasyprint }
        @page { size: 300px 300px; margin: 50px }
        body { margin: 0; font: 20px/20px weasyprint }
        p { margin: 0; orphans: 1; widows: 1 }
        .f { float: left; width: 40px; height: 40px }
      </style>`

// describeWords describes the text boxes of the pages that do not start
// with a digit as <text>@x,y, in the order of the tree, and the pages
// apart with " | ". The text of a line is in one text box.
func describeWords(pages []*Box) string {
	out := make([]string, 0, len(pages))
	for _, page := range pages {
		var items []string
		page.Descendants(func(d *Box) bool {
			if d.Kind == KindText && d.Text != "" && (d.Text[0] < '0' || d.Text[0] > '9') {
				items = append(items, fmt.Sprintf("%s@%g,%g", d.Text, d.PositionX, d.PositionY))
			}
			return true
		})
		out = append(out, strings.Join(items, " "))
	}
	return strings.Join(out, " | ")
}

func TestWrapFlowValidation(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  Value
	}{
		{"auto", String("auto")},
		{"both", Tuple{String("both"), Float(0)}},
		{"both 10px", Tuple{String("both"), Float(10)}},
		{"0.25in start", Tuple{String("start"), Float(24)}},
		{"end 0", Tuple{String("end"), Float(0)}},
		{"maximum 9pt", Tuple{String("maximum"), Float(12)}},
		// The values that are not valid keep the initial value.
		{"10px", String("auto")},
		{"both -1px", String("auto")},
		{"both 10%", String("auto")},
		{"both start", String("auto")},
		{"both 1px 2px", String("auto")},
		{"left", String("auto")},
	} {
		t.Run(tt.value, func(t *testing.T) {
			page := renderPages(t, `<style>@page { size: 100px } div { -ivy-wrap-flow: `+tt.value+` }</style><div></div>`)[0]
			var got Value
			page.Descendants(func(d *Box) bool {
				if d.ElementTag == "div" {
					got = d.Style.Get("wrap_flow")
				}
				return got == nil
			})
			if !Equal(got, tt.want) {
				t.Errorf("-ivy-wrap-flow: %s = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
	// The property is not inherited.
	page := renderPages(t, `<div style="-ivy-wrap-flow: both"><p>a</p></div>`)[0]
	page.Descendants(func(d *Box) bool {
		if d.ElementTag == "p" && !isStr(d.Style.Get("wrap_flow"), "auto") {
			t.Errorf("p has -ivy-wrap-flow: %v, want auto", d.Style.Get("wrap_flow"))
		}
		return true
	})
	// The property of a table goes to its wrapper, which floats.
	page = renderPages(t, `<table style="float: left; -ivy-wrap-flow: both 1px"><tr><td>a</td></tr></table>`)[0]
	page.Descendants(func(d *Box) bool {
		if d.IsTableWrapper && !Equal(d.Style.Get("wrap_flow"), Tuple{String("both"), Float(1)}) {
			t.Errorf("the wrapper has -ivy-wrap-flow: %v, want both 1px", d.Style.Get("wrap_flow"))
		}
		return true
	})
}

func TestWrapShapeValidation(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  Value
	}{
		{"none", String("none")},
		{"polygon(0 0, 10px 0, 10px 20px)", Tuple{Float(0), Float(0), Float(10), Float(0), Float(10), Float(20)}},
		{"polygon(-3pt 0, 1in 0, 0 1in)", Tuple{Float(-4), Float(0), Float(96), Float(0), Float(0), Float(96)}},
		// The values that are not valid keep the initial value.
		{"polygon(0 0, 10px 0)", String("none")},
		{"polygon(0 0, 10px 0, 10px)", String("none")},
		{"polygon(0 0, 10px 0, 10px 20px 1px)", String("none")},
		{"polygon(0 0, 10% 0, 10px 20px)", String("none")},
		{"circle(10px)", String("none")},
		{"polygon(0 0, 1px 0, 1px 1px) polygon(0 0, 1px 0, 1px 1px)", String("none")},
	} {
		t.Run(tt.value, func(t *testing.T) {
			page := renderPages(t, `<style>@page { size: 100px } div { -ivy-wrap-shape: `+tt.value+` }</style><div></div>`)[0]
			var got Value
			page.Descendants(func(d *Box) bool {
				if d.ElementTag == "div" {
					got = d.Style.Get("wrap_shape")
				}
				return got == nil
			})
			if !Equal(got, tt.want) {
				t.Errorf("-ivy-wrap-shape: %s = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestWrapFlow(t *testing.T) {
	// float returns a float of the class f with the style -ivy-wrap-flow:
	// value, and more style.
	float := func(value, more string) string {
		return `<div class=f style="-ivy-wrap-flow: ` + value + `; ` + more + `"></div>`
	}
	for _, tt := range []struct {
		name string
		body string
		want string
	}{{
		// The float is in the middle of the column. Its border box and its
		// distance cut the lines from x = 120 to x = 180, so each row of
		// the float has a room of 70px on each side. The text fills the
		// room on the left, then the room on the right at the same height.
		"both",
		float("both 10px", "margin-left: 80px") + `<p>aaa bbb ccc ddd eee fff ggg</p>`,
		"aaa@50,50 bbb@180,50 ccc@50,70 ddd@180,70 eee fff@50,90 ggg@50,110",
	}, {
		// auto keeps the rule of CSS: the text is on the right of the
		// margin box of the float.
		"auto",
		float("auto", "margin-left: 80px") + `<p>aaa bbb ccc</p>`,
		"aaa@170,50 bbb@170,70 ccc@50,90",
	}, {
		// start and end keep the text on one side of the float.
		"start",
		float("start 10px", "margin-left: 80px") + `<p>aaa bbb ccc ddd</p>`,
		"aaa@50,50 bbb@50,70 ccc ddd@50,90",
	}, {
		"end",
		float("end 10px", "margin-left: 80px") + `<p>aaa bbb ccc ddd</p>`,
		"aaa@180,50 bbb@180,70 ccc ddd@50,90",
	}, {
		// maximum keeps the text on the side with the larger room: from
		// x = 50 to x = 140 on the left, and from x = 200 to x = 250 on
		// the right.
		"maximum",
		float("maximum 10px", "margin-left: 100px") + `<p>aaa bbb ccc ddd</p>`,
		"aaa@50,50 bbb@50,70 ccc ddd@50,90",
	}, {
		// The polygon of -ivy-wrap-shape is a diamond in the float of
		// 40px by 80px, from x = 130 to x = 170 and from y = 50 to y =
		// 130. The part of the diamond next to each line cuts it: from x
		// = 140 to x = 160 for the first and the fourth lines, and from x
		// = 130 to x = 170 for the two lines in the middle. The line
		// below the diamond takes the whole width.
		"polygon",
		float("both", "margin-left: 80px; height: 80px; -ivy-wrap-shape: polygon(20px 0, 40px 40px, 20px 80px, 0 40px)") +
			`<p>aaaa bbbb cccc dddd eeee ffff gggg hhhh iiii</p>`,
		"aaaa@50,50 bbbb@160,50 cccc@50,70 dddd@170,70 eeee@50,90 ffff@170,90 gggg@50,110 hhhh@160,110 iiii@50,130",
	}, {
		// A room narrower than 0.25 inch holds no text: here the room of
		// 20px on the left of the float.
		"narrow room",
		float("both", "margin-left: 20px") + `<p>aaa bbb ccc</p>`,
		"aaa bbb@110,50 ccc@110,70",
	}, {
		// Two floats on the same row make three rooms.
		"two floats",
		float("both", "margin-left: 40px; width: 20px") + float("both", "margin-left: 40px; width: 20px") +
			`<p>aa bb cc dd ee</p>`,
		"aa@50,50 bb@110,50 cc@170,50 dd@50,70 ee@110,70",
	}, {
		// The margins of the float only give its place. The first line is
		// next to the top margin of the float, and not next to its border
		// box, so it takes the whole width.
		"margin",
		float("both 10px", "margin-left: 80px; margin-top: 30px") + `<p>aaa bbb ccc ddd eee</p>`,
		"aaa bbb@50,50 ccc@50,70 ddd@180,70 eee@50,90",
	}, {
		// The border box of the float starts inside the first line, which
		// is then in two parts.
		"inside a line",
		float("both 10px", "margin-left: 80px; margin-top: 10px") + `<p>aaa bbb ccc ddd eee</p>`,
		"aaa@50,50 bbb@180,50 ccc@50,70 ddd@180,70 eee@50,90",
	}, {
		// The float is in the middle of the first line. The text before it
		// stays in the first room, and the text after it goes on in the
		// next room.
		"in the text",
		`<p>aa <span class=f style="-ivy-wrap-flow: both 10px; margin-left: 80px"></span>bbb ccc ddd eee</p>`,
		"aa @50,50 bbb@180,50 ccc@50,70 ddd@180,70 eee@50,90",
	}, {
		// The float is in the text of the second line, which it cuts in
		// two parts. The first line does not see it.
		"in a later line",
		`<p>aaa bbb ccc <span class=f style="-ivy-wrap-flow: both 10px; margin-left: 80px"></span>ddd eee fff</p>`,
		"aaa bbb@50,50 ccc @50,70 ddd@180,70 eee@50,90 fff@180,90",
	}, {
		// A paragraph that ends in a room ends at the bottom of its row,
		// and the next paragraph starts below it.
		"paragraphs",
		float("both 10px", "margin-left: 80px") + `<p>aaa</p><p>bbb ccc</p>`,
		"aaa@50,50 bbb@50,70 ccc@180,70",
	}, {
		// A page float of D60 with -ivy-wrap-flow: the text before its
		// anchor is on both sides of it too.
		"page float",
		`<p>aaa</p>` + float("both 10px", "margin-left: 80px; -ivy-float-position: margin 0") + `<p>bbb ccc ddd</p>`,
		"aaa@50,50 bbb@50,70 ccc@180,70 ddd@50,90",
	}, {
		// In a text from right to left, the first room is on the right,
		// and the text is at the right of each room.
		"rtl",
		`<div style="direction: rtl">` + float("both 10px", "float: right; margin-right: 80px") + `<p>aaa bbb ccc</p></div>`,
		"aaa@190,50 bbb@60,50 ccc@190,70",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			pages := renderPages(t, wrapFlowStyle+tt.body)
			if got := describeWords(pages); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestWrapFlowLineNumbers(t *testing.T) {
	// The line numbers of D56 number the first part of a row only.
	pages := renderPages(t, wrapFlowStyle+`<div style="-ivy-line-numbers: 10px 1 1 page">`+
		`<div class=f style="-ivy-wrap-flow: both 10px; margin-left: 80px"></div>`+
		`<p>aaa bbb ccc ddd eee</p></div>`)
	if got, want := describeWords(pages), "aaa@50,50 bbb@180,50 ccc@50,70 ddd@180,70 eee@50,90"; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	if got, want := numberTexts(pageLineNumbers(t, pages)), "1 2 3"; got != want {
		t.Errorf("got the numbers %q, want %q", got, want)
	}
}

func TestWrapFlowJustify(t *testing.T) {
	// The alignment applies to each part of a row: each part but the last
	// line of the paragraph fills its room of 70px.
	pages := renderPages(t, wrapFlowStyle+`<div class=f style="-ivy-wrap-flow: both 10px; margin-left: 80px"></div>`+
		`<p style="text-align: justify">a b c d e f</p>`)
	var got []string
	pages[0].Descendants(func(d *Box) bool {
		if d.Kind == KindLine && d.ElementTag == "p" {
			got = append(got, fmt.Sprintf("%g+%g", d.PositionX, d.Width))
		}
		return true
	})
	if want := "50+70 180+70 50+60"; strings.Join(got, " ") != want {
		t.Errorf("got the lines %v, want %s", got, want)
	}
}

func TestWrapFlowPageBreak(t *testing.T) {
	// The rows of a float at the bottom of a page are on that page, and
	// the rest of the text goes to the next page.
	pages := renderPages(t, wrapFlowStyle+`<p>`+strings.Repeat("x<br>", 7)+`x</p>`+
		`<div class=f style="-ivy-wrap-flow: both 10px; margin-left: 80px"></div>`+
		`<p>aaa bbb ccc ddd eee</p>`)
	got := describeWords(pages)
	if want := "aaa@50,210 bbb@180,210 ccc@50,230 ddd@180,230 | eee@50,50"; !strings.HasSuffix(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestWrapFlowColumns(t *testing.T) {
	// Two columns of 90px, from x = 50 to x = 140 and from x = 160 to
	// x = 250, take 10 lines each. The first column is filled before the
	// second one, unless the style gives more.
	for _, tt := range []struct {
		name  string
		style string
		body  string
		want  string
	}{{
		// The float of the first column crosses into the second one. Its
		// border box and its distance cut the lines from x = 110 to
		// x = 170. The lines of both columns avoid it.
		"across", "",
		`<div class=f style="-ivy-wrap-flow: both 10px; margin-left: 70px"></div>` +
			`<p>aaa bbb ccc ddd eee fff ggg hhh iii jjj kkk lll mmm</p>`,
		"aaa@50,50 bbb@50,70 ccc@50,90 ddd@50,110 eee@50,130 fff@50,150 ggg@50,170 hhh@50,190 iii@50,210 jjj@50,230 " +
			"kkk@170,50 lll@170,70 mmm@160,90",
	}, {
		// A float without -ivy-wrap-flow does not move the lines of the
		// second column.
		"auto", "",
		`<div class=f style="margin-left: 70px; margin-right: -110px"></div>` +
			`<p>aaa bbb ccc ddd eee fff ggg hhh iii jjj kkk lll mmm</p>`,
		"aaa@50,50 bbb@50,70 ccc@50,90 ddd@50,110 eee@50,130 fff@50,150 ggg@50,170 hhh@50,190 iii@50,210 jjj@50,230 " +
			"kkk@160,50 lll@160,70 mmm@160,90",
	}, {
		// The float of the second column, from x = 110 to x = 170 with its
		// distance, does not move the lines of the first one, which are
		// 80px wide.
		"later column", "",
		`<p>aaaa bbbb cccc dddd eeee ffff gggg hhhh iiii jjjj</p>` +
			`<div class=f style="-ivy-wrap-flow: both 10px; margin-left: -40px"></div><p>kkk</p>`,
		"aaaa@50,50 bbbb@50,70 cccc@50,90 dddd@50,110 eeee@50,130 ffff@50,150 gggg@50,170 hhhh@50,190 iiii@50,210 jjjj@50,230 " +
			"kkk@170,50",
	}, {
		// The columns that the layout balances avoid the float too.
		"balance", "column-fill: balance",
		`<div class=f style="-ivy-wrap-flow: both 10px; margin-left: 70px; height: 80px"></div>` +
			`<p>aaa bbb ccc ddd eee fff ggg hhh</p>`,
		"aaa@50,50 bbb@50,70 ccc@50,90 ddd@50,110 eee@170,50 fff@170,70 ggg@170,90 hhh@170,110",
	}, {
		// From right to left, the first column is on the right. The float
		// cuts the lines from x = 130 to x = 190, and the text of the
		// second column is at the right of its room.
		"rtl", "column-fill: balance; direction: rtl",
		`<div class=f style="float: right; -ivy-wrap-flow: both 10px; margin-right: 70px; height: 80px"></div>` +
			`<p>aaa bbb ccc ddd eee fff ggg hhh</p>`,
		"aaa@190,50 bbb@190,70 ccc@190,90 ddd@190,110 eee@70,50 fff@70,70 ggg@70,90 hhh@70,110",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			pages := renderPages(t, wrapFlowStyle+`<div style="columns: 2; column-gap: 20px; column-fill: auto; `+
				tt.style+`">`+tt.body+`</div>`)
			if got := describeWords(pages); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}
