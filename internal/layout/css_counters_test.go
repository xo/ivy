// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_counters.py.
//
// The tests of the counter styles call CounterStyle.RenderValue and
// CounterStyle.RenderMarker directly.

package layout

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// render is RENDER in tests/css/test_counters.py: render_value of the
// counter styles of the user agent.
func render(value int, name string) string {
	return userAgentStylesheets().counterStyle.Copy().RenderValue(value, String(name))
}

// counterBoxCases are the tests of upstream that check the counters of
// the box tree.
var counterBoxCases = []struct {
	name     string
	all      bool
	source   string
	expected string
}{
	{"test_counters_1", true, `
      <style>
        p { counter-increment: p 2 }
        p:before { content: counter(p); }
        p:nth-child(1) { counter-increment: none; }
        p:nth-child(2) { counter-increment: p; }
      </style>
      <p></p>
      <p></p>
      <p></p>
      <p style="counter-reset: p 117 p"></p>
      <p></p>
      <p></p>
      <p style="counter-reset: p -13"></p>
      <p></p>
      <p></p>
      <p style="counter-reset: p 42"></p>
      <p></p>
      <p></p>`, `[('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '0')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '1')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '3')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '2')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '4')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '6')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-11')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-9')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-7')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '44')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '46')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '48')])])])]`},
	{"test_counters_2", true, `
      <ol style="list-style-position: inside">
        <li></li>
        <li></li>
        <li></li>
        <li><ol>
          <li></li>
          <li style="counter-increment: none"></li>
          <li></li>
        </ol></li>
        <li></li>
      </ol>`, `[('ol', 'Block', [('li', 'Block', [('li', 'Line', [('li::marker', 'Inline', [('li::marker', 'Text', '1. ')])])]), ('li', 'Block', [('li', 'Line', [('li::marker', 'Inline', [('li::marker', 'Text', '2. ')])])]), ('li', 'Block', [('li', 'Line', [('li::marker', 'Inline', [('li::marker', 'Text', '3. ')])])]), ('li', 'Block', [('li', 'Block', [('li', 'Line', [('li::marker', 'Inline', [('li::marker', 'Text', '4. ')])])]), ('ol', 'Block', [('li', 'Block', [('li', 'Line', [('li::marker', 'Inline', [('li::marker', 'Text', '1. ')])])]), ('li', 'Block', [('li', 'Line', [('li::marker', 'Inline', [('li::marker', 'Text', '1. ')])])]), ('li', 'Block', [('li', 'Line', [('li::marker', 'Inline', [('li::marker', 'Text', '2. ')])])])])]), ('li', 'Block', [('li', 'Line', [('li::marker', 'Inline', [('li::marker', 'Text', '5. ')])])])])]`},
	{"test_counters_3", true, `
      <style>
        p { display: list-item; list-style: inside decimal }
      </style>
      <div>
        <p></p>
        <p></p>
        <p style="counter-reset: list-item 7 list-item -56"></p>
      </div>
      <p></p>`, `[('div', 'Block', [('p', 'Block', [('p', 'Line', [('p::marker', 'Inline', [('p::marker', 'Text', '1. ')])])]), ('p', 'Block', [('p', 'Line', [('p::marker', 'Inline', [('p::marker', 'Text', '2. ')])])]), ('p', 'Block', [('p', 'Line', [('p::marker', 'Inline', [('p::marker', 'Text', '-55. ')])])])]), ('p', 'Block', [('p', 'Line', [('p::marker', 'Inline', [('p::marker', 'Text', '1. ')])])])]`},
	{"test_counters_4", true, `
      <style>
        section:before { counter-reset: h; content: '' }
        h1:before { counter-increment: h; content: counters(h, '.') }
      </style>
      <body>
        <section><h1></h1>
          <h1></h1>
          <section><h1></h1>
            <h1></h1>
          </section>
          <h1></h1>
        </section>
      </body>`, `[('section', 'Block', [('section', 'Block', [('section', 'Line', [('section::before', 'Inline', [])])]), ('h1', 'Block', [('h1', 'Line', [('h1::before', 'Inline', [('h1::before', 'Text', '1')])])]), ('h1', 'Block', [('h1', 'Line', [('h1::before', 'Inline', [('h1::before', 'Text', '2')])])]), ('section', 'Block', [('section', 'Block', [('section', 'Line', [('section::before', 'Inline', [])])]), ('h1', 'Block', [('h1', 'Line', [('h1::before', 'Inline', [('h1::before', 'Text', '2.1')])])]), ('h1', 'Block', [('h1', 'Line', [('h1::before', 'Inline', [('h1::before', 'Text', '2.2')])])])]), ('h1', 'Block', [('h1', 'Line', [('h1::before', 'Inline', [('h1::before', 'Text', '3')])])])])]`},
	{"test_counters_5", true, `
      <style>
        p:before { content: counter(c) }
      </style>
      <div>
        <span style="counter-reset: c">
          Scope created now, deleted after the div
        </span>
      </div>
      <p></p>`, `[('div', 'Block', [('div', 'Line', [('span', 'Inline', [('span', 'Text', 'Scope created now, deleted after the div ')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '0')])])])]`},
	{"test_counters_6", true, `
      <p style="counter-increment: c;
                display: list-item; list-style: inside decimal">`, `[('p', 'Block', [('p', 'Line', [('p::marker', 'Inline', [('p::marker', 'Text', '0. ')])])])]`},
	{"test_counters_7", true, `
      <style>
        p { counter-increment: p 2 }
        p:before { content: counter(p) '.' counter(P); }
      </style>
      <p></p>
      <p style="counter-increment: P 3"></p>
      <p></p>`, `[('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '2.0')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '2.3')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '4.3')])])])]`},
	{"test_counters_8", true, `
      <style>
        p:before { content: 'a'; display: list-item }
      </style>
      <p></p>
      <p></p>`, `[('p', 'Block', [('p::before', 'Block', [('p::marker', 'Block', [('p::marker', 'Line', [('p::marker', 'Text', '• ')])]), ('p::before', 'Block', [('p::before', 'Line', [('p::before', 'Text', 'a')])])])]), ('p', 'Block', [('p::before', 'Block', [('p::marker', 'Block', [('p::marker', 'Line', [('p::marker', 'Text', '• ')])]), ('p::before', 'Block', [('p::before', 'Line', [('p::before', 'Text', 'a')])])])])]`},
	{"test_counter_styles_1", true, `
      <style>
        body { --var: 'Counter'; counter-reset: p -12 }
        p { counter-increment: p }
        p:nth-child(1):before { content: '-' counter(p, none) '-'; }
        p:nth-child(2):before { content: counter(p, disc); }
        p:nth-child(3):before { content: counter(p, circle); }
        p:nth-child(4):before { content: counter(p, square); }
        p:nth-child(5):before { content: counter(p); }
        p:nth-child(6):before { content: var(--var) ':' counter(p); }
        p:nth-child(7):before { content: counter(p) ':' var(--var); }
      </style>
      <p></p>
      <p></p>
      <p></p>
      <p></p>
      <p></p>
      <p></p>
      <p></p>
    `, `[('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '--')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '•')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '◦')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '▪')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-7')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', 'Counter:-6')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-5:Counter')])])])]`},
	{"test_counter_styles_2", true, `
      <style>
        p { counter-increment: p }
        p::before { content: counter(p, decimal-leading-zero); }
      </style>
      <p style="counter-reset: p -1987"></p>
      <p></p>
      <p style="counter-reset: p -12"></p>
      <p></p>
      <p></p>
      <p></p>
      <p style="counter-reset: p -2"></p>
      <p></p>
      <p></p>
      <p></p>
      <p style="counter-reset: p 8"></p>
      <p></p>
      <p></p>
      <p style="counter-reset: p 98"></p>
      <p></p>
      <p></p>
      <p style="counter-reset: p 4134"></p>
      <p></p>
    `, `[('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-1986')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-1985')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-11')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-10')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-9')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-8')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '-1')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '00')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '01')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '02')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '09')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '10')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '11')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '99')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '100')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '101')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '4135')])])]), ('p', 'Block', [('p', 'Line', [('p::before', 'Inline', [('p::before', 'Text', '4136')])])])]`},
}

// TestCountersBoxes runs counterBoxCases.
func TestCountersBoxes(t *testing.T) {
	for _, c := range counterBoxCases {
		t.Run(c.name, func(t *testing.T) {
			log, capture := captureLogs(slog.LevelInfo)
			assertTree(t, parseAllLog(t, c.source, testBaseURL, log), c.expected)
			assertNoLogs(t, capture)
		})
	}
}

// markerText returns the text of the first line of a box: the text of the
// first child of the first child of its line, such as the text of a
// ::before.
func markerText(t *testing.T, box *Box) string {
	t.Helper()
	for range 3 {
		if len(box.Children) == 0 {
			t.Fatalf("%v has no children", box)
		}
		box = box.Children[0]
	}
	return box.Text
}

// TestCounters9 is test_counters_9. Upstream reads the boxes of the pages
// of render_pages. The port reads the same boxes of parse_all.
func TestCounters9(t *testing.T) {
	t.Skip("upstream marks the test as an expected failure: the list-item counter of a list without counter-reset: list-item")
	html := parseAll(t, `
      <ol>
        <li></li>
        <li>
          <ol style="counter-reset: a">
            <li></li>
            <li></li>
          </ol>
        </li>
        <li></li>
      </ol>
    `)
	body := html.Children[0]
	ol1 := body.Children[0]
	oli1, oli2, oli3 := ol1.Children[0], ol1.Children[1], ol1.Children[2]
	ol2 := oli2.Children[1]
	oli21, oli22 := ol2.Children[0], ol2.Children[1]
	for _, c := range []struct {
		box  *Box
		want string
	}{
		{oli1, "1. "}, {oli2, "2. "}, {oli21, "1. "}, {oli22, "2. "}, {oli3, "3. "},
	} {
		if got := markerText(t, c.box); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

// TestCounterSet is test_counter_set. Upstream reads the boxes of the pages
// of render_pages. The port reads the same boxes of parse_all.
func TestCounterSet(t *testing.T) {
	html := parseAll(t, `
      <style>
        body { counter-reset: h2 0 h3 4; font-size: 1px }
        article { counter-reset: h2 2 }
        h1 { counter-increment: h1 }
        h1::before { content: counter(h1) }
        h2 { counter-increment: h2; counter-set: h3 3 }
        h2::before { content: counter(h2) }
        h3 { counter-increment: h3 }
        h3::before { content: counter(h3) }
      </style>
      <article>
        <h1></h1>
      </article>
      <article>
        <h2></h2>
        <h3></h3>
      </article>
      <article>
        <h3></h3>
      </article>
      <article>
        <h2></h2>
      </article>
      <article>
        <h3></h3>
        <h3></h3>
      </article>
      <article>
        <h1></h1>
        <h2></h2>
        <h3></h3>
      </article>
    `)
	body := html.Children[0]
	got := make([]string, 0, len(body.Children))
	for _, article := range body.Children {
		var texts []string
		for _, h := range article.Children {
			texts = append(texts, markerText(t, h))
		}
		got = append(got, strings.Join(texts, " "))
	}
	if want := "1|3 4|5|3|4 5|1 3 4"; strings.Join(got, "|") != want {
		t.Errorf("got %q, want %q", strings.Join(got, "|"), want)
	}
}

// TestCounterStyles ports test_counter_styles_3 to test_counter_styles_11.
func TestCounterStyles(t *testing.T) {
	for _, c := range []struct {
		style  string
		values []int
		want   string
	}{
		// test_counter_styles_3
		{"decimal-leading-zero", []int{
			-1986, -1985,
			-11, -10, -9, -8,
			-1, 0, 1, 2,
			9, 10, 11,
			99, 100, 101,
			4135, 4136,
		}, `
			-1986 -1985  -11 -10 -9 -8  -1 00 01 02  09 10 11
			99 100 101  4135 4136`},
		// test_counter_styles_4
		{"lower-roman", []int{
			-1986, -1985,
			-1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
			49, 50,
			389, 390,
			3489, 3490, 3491,
			4999, 5000, 5001,
		}, `
			-1986 -1985  -1 0 i ii iii iv v vi vii viii ix x xi xii
			xlix l  ccclxxxix cccxc  mmmcdlxxxix mmmcdxc mmmcdxci
			4999 5000 5001`},
		// test_counter_styles_5
		{"upper-roman", []int{
			-1986, -1985,
			-1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
			49, 50,
			389, 390,
			3489, 3490, 3491,
			4999, 5000, 5001,
		}, `
			-1986 -1985  -1 0 I II III IV V VI VII VIII IX X XI XII
			XLIX L  CCCLXXXIX CCCXC  MMMCDLXXXIX MMMCDXC MMMCDXCI
			4999 5000 5001`},
		// test_counter_styles_6
		{"lower-alpha", []int{
			-1986, -1985,
			-1, 0, 1, 2, 3, 4,
			25, 26, 27, 28, 29,
			2002, 2003,
		}, `-1986 -1985  -1 0 a b c d  y z aa ab ac bxz bya`},
		// test_counter_styles_7
		{"upper-alpha", []int{
			-1986, -1985,
			-1, 0, 1, 2, 3, 4,
			25, 26, 27, 28, 29,
			2002, 2003,
		}, `-1986 -1985  -1 0 A B C D  Y Z AA AB AC BXZ BYA`},
		// test_counter_styles_8
		{"lower-latin", []int{
			-1986, -1985,
			-1, 0, 1, 2, 3, 4,
			25, 26, 27, 28, 29,
			2002, 2003,
		}, `-1986 -1985  -1 0 a b c d  y z aa ab ac bxz bya`},
		// test_counter_styles_9
		{"upper-latin", []int{
			-1986, -1985,
			-1, 0, 1, 2, 3, 4,
			25, 26, 27, 28, 29,
			2002, 2003,
		}, `-1986 -1985  -1 0 A B C D  Y Z AA AB AC BXZ BYA`},
		// test_counter_styles_10
		{"georgian", []int{
			-1986, -1985,
			-1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
			20, 30, 40, 50, 60, 70, 80, 90, 100,
			200, 300, 400, 500, 600, 700, 800, 900, 1000,
			2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000,
			19999, 20000, 20001,
		}, `
			-1986 -1985  -1 0 ა
			ბ გ დ ე ვ ზ ჱ თ ი ია იბ
			კ ლ მ ნ ჲ ო პ ჟ რ
			ს ტ ჳ ფ ქ ღ ყ შ ჩ
			ც ძ წ ჭ ხ ჴ ჯ ჰ ჵ
			ჵჰშჟთ 20000 20001`},
		// test_counter_styles_11
		{"armenian", []int{
			-1986, -1985,
			-1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
			20, 30, 40, 50, 60, 70, 80, 90, 100,
			200, 300, 400, 500, 600, 700, 800, 900, 1000,
			2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000,
			9999, 10000, 10001,
		}, `
			-1986 -1985  -1 0 Ա
			Բ Գ Դ Ե Զ Է Ը Թ Ժ ԺԱ ԺԲ
			Ի Լ Խ Ծ Կ Հ Ձ Ղ Ճ
			Մ Յ Ն Շ Ո Չ Պ Ջ Ռ
			Ս Վ Տ Ր Ց Ւ Փ Ք
			ՔՋՂԹ 10000 10001`},
	} {
		t.Run(c.style, func(t *testing.T) {
			want := strings.Fields(c.want)
			if len(want) != len(c.values) {
				t.Fatalf("the test has %d values and %d results", len(c.values), len(want))
			}
			for i, value := range c.values {
				if got := render(value, c.style); got != want[i] {
					t.Errorf("%d: got %q, want %q", value, got, want[i])
				}
			}
		})
	}
}

// markers returns the markers of the li elements of a document, as the
// building of the boxes renders them: with the counter style of the
// document, the computed list-style-type of each li, and the counter
// values 1, 2, 3 and so on.
func markers(t *testing.T, source string) []string {
	t.Helper()
	log, capture := captureLogs(0)
	document := fakeHTML(t, source, testBaseURL, log)
	counterStyle := userAgentStylesheets().counterStyle.Copy()
	sf := computedStyles(t, document, ComputedStylesOptions{CounterStyle: counterStyle})
	lis := elementsByTag(document.EtreeElement, "li")
	result := make([]string, 0, len(lis))
	for i, li := range lis {
		listStyleType := sf.Get(li, "").Get("list_style_type")
		result = append(result, counterStyle.RenderMarker(listStyleType, i+1))
	}
	assertNoLogs(t, capture)
	return result
}

// TestCounterSymbols is test_counter_symbols. Upstream reads the text of
// the markers of the boxes. The port renders the markers of the computed
// list-style-type.
func TestCounterSymbols(t *testing.T) {
	for _, c := range []struct {
		arguments string
		values    []string
	}{
		{`cyclic "a" "b" "c"`, []string{"a ", "b ", "c ", "a "}},
		{`symbolic "a" "b"`, []string{"a ", "b ", "aa ", "bb "}},
		{`"a" "b"`, []string{"a ", "b ", "aa ", "bb "}},
		{`alphabetic "a" "b"`, []string{"a ", "b ", "aa ", "ab "}},
		{`fixed "a" "b"`, []string{"a ", "b ", "3 ", "4 "}},
		{`numeric "0" "1" "2"`, []string{"1 ", "2 ", "10 ", "11 "}},
	} {
		t.Run(c.arguments, func(t *testing.T) {
			got := markers(t, fmt.Sprintf(`
              <style>ol { list-style-type: symbols(%s) }</style>
              <ol><li>abc</li><li>abc</li><li>abc</li><li>abc</li></ol>`, c.arguments))
			if strings.Join(got, "|") != strings.Join(c.values, "|") {
				t.Errorf("got %q, want %q", got, c.values)
			}
		})
	}
}

// TestListStyleTypes is test_list_style_types. Upstream reads the text of
// the markers of the boxes. The port renders the markers of the computed
// list-style-type.
func TestListStyleTypes(t *testing.T) {
	for _, c := range []struct {
		styleType string
		values    []string
	}{
		{"decimal", []string{"1. ", "2. ", "3. ", "4. "}},
		{`"/"`, []string{"/", "/", "/", "/"}},
	} {
		t.Run(c.styleType, func(t *testing.T) {
			got := markers(t, fmt.Sprintf(`
              <style>ol { list-style-type: %s }</style>
              <ol><li>abc</li><li>abc</li><li>abc</li><li>abc</li></ol>`, c.styleType))
			if strings.Join(got, "|") != strings.Join(c.values, "|") {
				t.Errorf("got %q, want %q", got, c.values)
			}
		})
	}
}

// TestListStyleTypeEmptyString is test_list_style_type_empty_string, a
// regression test for #1883. The port renders the marker of the computed
// list-style-type instead of the page.
func TestListStyleTypeEmptyString(t *testing.T) {
	got := markers(t, `<ul><li style="list-style-type: ''">`)
	if len(got) != 1 || got[0] != "" {
		t.Errorf("got %q, want an empty marker", got)
	}
}

// TestCounterMultipleExtends is test_counter_multiple_extends, inspired by
// the W3C failing test system-extends-invalid. Upstream reads the text of
// the markers of the boxes. The port renders the markers of the computed
// list-style-type.
func TestCounterMultipleExtends(t *testing.T) {
	got := markers(t, `
      <style>
        @counter-style a {
          system: extends b;
          prefix: a;
        }
        @counter-style b {
          system: extends c;
          suffix: b;
        }
        @counter-style c {
          system: extends b;
          pad: 2 c;
        }
        @counter-style d {
          system: extends d;
          prefix: d;
        }
        @counter-style e {
          system: extends unknown;
          prefix: e;
        }
        @counter-style f {
          system: extends decimal;
          symbols: a;
        }
        @counter-style g {
          system: extends decimal;
          additive-symbols: 1 a;
        }
      </style>
      <ol>
        <li style="list-style-type: a"></li>
        <li style="list-style-type: b"></li>
        <li style="list-style-type: c"></li>
        <li style="list-style-type: d"></li>
        <li style="list-style-type: e"></li>
        <li style="list-style-type: f"></li>
        <li style="list-style-type: g"></li>
        <li style="list-style-type: h"></li>
      </ol>`)
	want := []string{"a1b", "2b", "c3. ", "d4. ", "e5. ", "6. ", "7. ", "8. "}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestCounterRepresentationGuard checks the guards of the port on the
// counters: a symbolic counter or a pad that would make a huge text falls
// back to decimal or is cut, and a long chain of fallbacks ends.
func TestCounterRepresentationGuard(t *testing.T) {
	cs := userAgentStylesheets().counterStyle.Copy()
	symbolic := Tuple{String("symbols()"), Tuple{String("symbolic"), String("a")}}
	if got := cs.RenderValue(1_000_000_000, symbolic); got != "1000000000" {
		t.Errorf("symbolic: got %d bytes, want the decimal fallback", len(got))
	}
	cs["big-pad"] = CounterStyleRule{
		"system": Tuple{String("extends"), String("decimal"), nil},
		"pad":    Tuple{Int(1_000_000_000), Tuple{String("string"), String("0")}},
	}
	if got := cs.RenderValue(1, String("big-pad")); len(got) > maxCounterRepresentation+1 {
		t.Errorf("pad: got %d bytes", len(got))
	}
	for i := range 100 {
		cs[fmt.Sprintf("f%d", i)] = CounterStyleRule{
			"system":   Tuple{nil, String("fixed"), Int(1)},
			"symbols":  Tuple{Tuple{String("string"), String("x")}},
			"fallback": String(fmt.Sprintf("f%d", i+1)),
		}
	}
	cs.RenderValue(5, String("f0"))
}
