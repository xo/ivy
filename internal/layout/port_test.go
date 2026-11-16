// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file holds the tests of the port that upstream does not have: the
// tree of the documents, the URLs of a Source, the helpers of html.py, the
// guards, and the fuzz test of the cascade, of the boxes and of the layout.

package layout

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
)

// TestTreeFromXML checks that an XHTML document gives the tree of the same
// HTML document: the tags of XHTML without namespace, the attributes in
// other namespaces in the Clark notation, and the text and the tails.
func TestTreeFromXML(t *testing.T) {
	src := Source{FS: fstest.MapFS{
		"a/doc.xhtml": {Data: []byte(`<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="fr">
<head><base href="../b/"/><title>T</title></head>
<body><p epub:type="note">a<!-- c -->b<em>c</em>d</p><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg></body>
</html>`)},
	}}
	html, err := LoadHTML(src, "a/doc.xhtml", HTMLOptions{})
	if err != nil {
		t.Fatal(err)
	}
	root := html.EtreeElement
	if root.Tag != "html" {
		t.Errorf("root tag: %q", root.Tag)
	}
	if lang, _ := root.Get("{" + nsXML + "}lang"); lang != "fr" {
		t.Errorf("xml:lang: %q", lang)
	}
	p := findElement(root, "p")
	if p == nil {
		t.Fatal("no p")
	}
	if v, _ := p.Get("{http://www.idpf.org/2007/ops}type"); v != "note" {
		t.Errorf("epub:type: %q", v)
	}
	if p.Text != "ab" || p.Children[0].Tag != "em" || p.Children[0].Text != "c" || p.Children[0].Tail != "d" {
		t.Errorf("text: %q %q %q", p.Text, p.Children[0].Text, p.Children[0].Tail)
	}
	if findElement(root, "{"+nsSVG+"}svg") == nil {
		t.Error("no svg")
	}
	if html.BaseURL != "file:///b/" {
		t.Errorf("base URL: %q", html.BaseURL)
	}
}

// TestTreeLimits checks the guards of the trees: the number and the depth of
// the elements.
func TestTreeLimits(t *testing.T) {
	deep := strings.Repeat("<div>", 1000) + "x"
	src := Source{FS: fstest.MapFS{"deep.html": {Data: []byte(deep)}, "many.html": {Data: []byte(strings.Repeat("<p>", 100))}}}
	html, err := LoadHTML(src, "deep.html", HTMLOptions{})
	if err != nil {
		t.Fatal(err)
	}
	depth := 0
	for el := html.EtreeElement; len(el.Children) > 0; el = el.Children[len(el.Children)-1] {
		depth++
	}
	if depth >= DefaultMaxTreeDepth {
		t.Errorf("depth %d not bounded", depth)
	}
	html, err = LoadHTML(src, "many.html", HTMLOptions{Limits: ivy.Limits{MaxObjects: 10}})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range html.EtreeElement.Iter() {
		count++
	}
	if count > 10 {
		t.Errorf("%d elements past the limit", count)
	}
}

// TestURLs checks the URLs of a Source and the fetcher.
func TestURLs(t *testing.T) {
	for _, c := range []struct{ base, ref, want string }{
		{"file:///OEBPS/text/ch1.xhtml", "../css/a.css", "file:///OEBPS/css/a.css"},
		{"file:///OEBPS/text/ch1.xhtml", "/x.png", "file:///x.png"},
		{"file:///a/b.html", "c%20d.png", "file:///a/c%20d.png"},
		{"file:///a/b.html", "é.png", "file:///a/%C3%A9.png"},
		{"file:///a/b.html", "http://example.com/x", "http://example.com/x"},
	} {
		got, ok := urlJoin(nil, c.base, c.ref, false, "")
		if !ok || got != c.want {
			t.Errorf("urlJoin(%q, %q): got %q, want %q", c.base, c.ref, got, c.want)
		}
	}
	if pathToURL("a b/c.html") != "file:///a%20b/c.html" {
		t.Errorf("pathToURL: %q", pathToURL("a b/c.html"))
	}

	fetcher := &URLFetcher{FS: fstest.MapFS{"a b/c.css": {Data: []byte("p{}")}}}
	r, err := fetcher.Fetch("file:///a%20b/c.css")
	if err != nil || string(r.Body) != "p{}" || r.ContentType != "text/css" {
		t.Errorf("fetch file: %v %v", r, err)
	}
	r, err = fetcher.Fetch("data:text/css;base64,cHt9")
	if err != nil || string(r.Body) != "p{}" || r.ContentType != "text/css" {
		t.Errorf("fetch data: %v %v", r, err)
	}
	r, err = fetcher.Fetch("data:,a%20b")
	if err != nil || string(r.Body) != "a b" || r.ContentType != "text/plain" {
		t.Errorf("fetch data: %v %v", r, err)
	}
	for _, u := range []string{"http://example.com/a.css", "file:///../x", "file://host/a", "relative.css", "file:///missing"} {
		if _, err := fetcher.Fetch(u); !errors.Is(err, ErrURLFetching) {
			t.Errorf("%s: expected ErrURLFetching, got %v", u, err)
		}
	}
	small := &URLFetcher{FS: fstest.MapFS{"a": {Data: []byte("0123456789")}}, MaxSize: 4}
	if _, err := small.Fetch("file:///a"); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got %v", err)
	}

	for _, c := range []struct {
		u, base string
		want    Value
	}{
		{"#a%20b", "", Tuple{String("internal"), String("a b")}},
		{"x.png", "file:///a/b.html", Tuple{String("external"), String("file:///a/x.png")}},
		{"x.png", "", nil},
	} {
		if got := getURLTuple(c.u, c.base); !Equal(got, c.want) {
			t.Errorf("getURLTuple(%q, %q): got %s", c.u, c.base, ValueString(got))
		}
	}
	link := GetLink(nil, "b.html#x", "file:///a/b.html")
	if !Equal(link, Tuple{String("url"), Tuple{String("internal"), String("x")}}) {
		t.Errorf("GetLink: %s", ValueString(link))
	}
}

// TestHTMLHelpers checks the parsers of the attributes of html.py.
func TestHTMLHelpers(t *testing.T) {
	for _, c := range []struct {
		s    string
		want int
		ok   bool
	}{
		{"12", 12, true}, {" -3x", -3, true}, {"+4", 4, true}, {"x", 0, false}, {"", 0, false},
	} {
		if got, ok := parseInteger(c.s); got != c.want || ok != c.ok {
			t.Errorf("parseInteger(%q): %d %v", c.s, got, ok)
		}
	}
	if _, ok := parseNonNegativeInteger("-1"); ok {
		t.Error("parseNonNegativeInteger(-1) is valid")
	}
	if v, ok := mapToDimensionProperty("50%"); !ok || v != "50.0%" {
		t.Errorf("mapToDimensionProperty: %q", v)
	}
	if v, ok := mapToDimensionProperty("12.5"); !ok || v != "12.5px" {
		t.Errorf("mapToDimensionProperty: %q", v)
	}
	if _, ok := mapToDimensionPropertyIgnoringZero("0"); ok {
		t.Error("mapToDimensionPropertyIgnoringZero(0) is valid")
	}
	if v, ok := mapToPixelLength("3"); !ok || v != "3px" {
		t.Errorf("mapToPixelLength: %q", v)
	}
	for _, c := range []struct{ s, want string }{
		{"red", "#ff0000"}, {" #00f ", "#0000ff"}, {"transparent", "None"}, {"nope", "None"},
	} {
		if got := legacyColorCSS(c.s); got != c.want {
			t.Errorf("legacyColorCSS(%q): %q", c.s, got)
		}
	}
	el := &Element{Tag: "link", Attrib: []Attr{{"rel", " StyleSheet\talternate "}}}
	if !elementHasLinkType(el, "stylesheet") || !elementHasLinkType(el, "alternate") || elementHasLinkType(el, "icon") {
		t.Error("elementHasLinkType")
	}
	if u, ok := parseURL(nil, " a.png ", "file:///d/x.html", false); !ok || u != "file:///d/a.png" {
		t.Errorf("parseURL: %q", u)
	}
	box := &Box{Kind: KindInline}
	a := &Element{Tag: "a", Attrib: []Attr{{"href", "#top"}}}
	handleA(&builder{}, a, box, "")
	if !Equal(box.Link, Tuple{String("url"), Tuple{String("internal"), String("top")}}) {
		t.Errorf("handle_a: %s", ValueString(box.Link))
	}
}

// TestHTMLMetadata checks get_html_metadata.
func TestHTMLMetadata(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	html := fakeHTML(t, `<html lang="en"><head>
      <title>The title</title><title>Not this one</title>
      <meta name="keywords" content="a, b,a">
      <meta name="author" content="A">
      <meta name="author" content="B">
      <meta name="description" content="D">
      <meta name="dcterms.created" content="bad">
      <meta name="dcterms.created" content="1997-07-16T19:20+01:00">
      <meta name="custom" content="C">
      <link rel="attachment" href="file.pdf" title="F">
    </head></html>`, testBaseURL, log)
	m := GetHTMLMetadata(html)
	if m.Title != "The title" || m.Lang != "en" || m.Description != "D" || m.Created != "1997-07-16T19:20+01:00" {
		t.Errorf("metadata: %+v", m)
	}
	if strings.Join(m.Keywords, "|") != "a|b" || strings.Join(m.Authors, "|") != "A|B" || m.Custom["custom"] != "C" {
		t.Errorf("metadata: %+v", m)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].URL != "file:///file.pdf" || m.Attachments[0].Description != "F" {
		t.Errorf("attachments: %+v", m.Attachments)
	}
	assertLog(t, capture, "Invalid date")
}

// TestImportGuards checks the guards of @import: a style sheet that imports
// itself stops at the depth limit, and the imports stop at maxImports.
func TestImportGuards(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	fsys := testFS(t, map[string]string{
		"self.css": `@import "self.css"; @import "self.css"; p { color: red }`,
	})
	doc := `<link rel=stylesheet href=self.css><p>a`
	html := fakeHTML(t, doc, testBaseURL, log)
	html.URLFetcher = &URLFetcher{FS: fsys}
	styleFor := computedStyles(t, html, ComputedStylesOptions{})
	assertRGBA(t, "color", styleFor.Get(findElement(html.EtreeElement, "p"), "").Get("color"), 1, 0, 0, 1)
	if len(capture.logs()) == 0 {
		t.Error("expected errors for the imports past the limits")
	}
}

// TestCascadeLimit checks that a cascade past the limit of work fails with
// ivy.ErrLimit.
func TestCascadeLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("<style>")
	for range 200 {
		b.WriteString("* { color: red; margin: 1px; padding: 1px }")
	}
	b.WriteString("</style>")
	b.WriteString(strings.Repeat("<p>a", 200))
	html := fakeHTML(t, b.String(), testBaseURL, nil)
	html.Limits = ivy.Limits{MaxObjects: 100}.WithDefaults()
	_, err := GetAllComputedStyles(html, ComputedStylesOptions{})
	if !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ErrLimit, got %v", err)
	}
}

// computeAll computes each property of each element and pseudo-element of a
// document, for the fuzz test.
func computeAll(styleFor *StyleFor, root *Element) {
	names := make([]string, 0, len(InitialValues))
	for name := range InitialValues {
		names = append(names, name)
	}
	for el := range root.Iter() {
		for _, pseudo := range []string{"", "before", "after", "marker", "footnote-call", "footnote-marker"} {
			style := styleFor.Get(el, pseudo)
			if style == nil {
				continue
			}
			for _, name := range names {
				style.Get(name)
			}
			style.AnonymousStyle().Get("display")
		}
	}
	for _, pageType := range []PageType{{Side: "right"}, {Side: "left", Index: 1, Name: "a", Groups: []PageGroup{{"a", 0}}}} {
		styleFor.AddPageDeclarations(pageType)
		if err := styleFor.SetComputedStyles(pageType, root, root, "", "", nil); err != nil {
			continue
		}
		for _, key := range styleFor.CascadedKeys() {
			if key.Pseudo != "" && key.Owner.ownerKey() == pageType.ownerKey() {
				_ = styleFor.SetComputedStyles(key.Owner, key.Owner, root, key.Pseudo, "", nil)
			}
		}
		if style := styleFor.Get(pageType, ""); style != nil {
			for _, name := range names {
				style.Get(name)
			}
		}
	}
}

// FuzzCascade parses a document and a style sheet, computes all the
// styles, builds the boxes, splits the lines of each text box, lays out the
// first three pages and draws them. It must never panic or hang (D5).
func FuzzCascade(f *testing.F) {
	f.Add(`<p style="color: var(--a)">a<span>b</span></p>`, `p { --a: red; margin: calc(1em + 2px) } @page :first { size: a4 landscape; @top-left { content: counter(page) } }`)
	f.Add(`<ol start=3><li value=2>a</li></ol><table cellpadding=2 border=1><tr><td width=50%>x`, `li::marker { content: counters(list-item, ".", upper-roman) } @counter-style x { system: additive; additive-symbols: 10 X, 1 I }`)
	f.Add(`<div><p>a</p></div>`, `@layer a, b; @layer b { div { & p { width: min(10px, 5vw) } } } @media print { @import "x.css"; }`)
	f.Add(`<svg width="10" height="5%"></svg>`, `@font-face { src: url(a.woff) format("woff"); font-family: x } @import url(self.css) layer(a);`)
	f.Add(`<p>`, `p { --b: var(--a) var(--a); --c: var(--b) var(--b); width: var(--c); background: linear-gradient(red, 10% blue); grid-template: "a b" 1fr / auto }`)
	f.Add(`<table><col span=3><tr><td rowspan=0 colspan=2>a<td>b<tr><td>c</table><span style="display: table-cell">d</span>`, `table { border-collapse: collapse; border: 2px solid; direction: rtl }`)
	f.Add(`<p id=a>Lorem <q>ipsum</q><a href="#a">dolor</a><img src=p.png alt=x><object data=p.svg>y</object> sit&shy;amet</p><pre>a&#9;b&#10;c</pre>`, `@font-face { src: url(wp.otf); font-family: wp } p { font-family: wp; letter-spacing: 1ex; word-spacing: 1ch; text-transform: capitalize; hyphens: manual } a::after { content: target-counter(attr(href), list-item) target-text(attr(href)) leader(dotted) } q::before { content: open-quote } p { string-set: s content(); bookmark-level: 1 }`)
	f.Add(`<div style="display: flex">a<span>b</span><p>c</p></div><div style="display: grid">d</div><p style="float: footnote">e</p><li>f`, `li { list-style: inside url(p.png) } div::before { content: "x"; display: list-item } p { overflow-wrap: anywhere; width: 1px; white-space: pre-wrap }`)
	f.Add(`<p style="float: left; width: 50%">a b c</p><div style="position: absolute; top: 10%; right: 3px">x</div><div style="position: fixed; bottom: 0">y</div><p style="break-before: page; columns: 3; column-rule: solid">a b c d e f g h</p>`, `@page { size: 200px 100px; margin: 10px; @bottom-center { content: counter(page) "/" counter(pages) } } p { line-height: 30px }`)
	f.Add(`<table style="table-layout: fixed; width: 100px"><caption>c</caption><thead><tr><th colspan=2>h</th></tr></thead><tr><td rowspan=3>a</td><td style="width: 20%">b</td></tr><tr><td>c</td></tr></table><div style="display: flex; flex-wrap: wrap; flex-direction: column"><p style="flex: 1 0 30px; order: 2">a</p><p style="align-self: stretch">b</p></div>`, `@page { size: 150px } table { border-spacing: 3px; border-collapse: collapse } p { margin: auto }`)
	f.Add(`<div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(20px, 1fr)); grid-template-areas: 'a b' 'c c'"><p style="grid-area: c">a</p><p style="grid-row: span 3 / -1; grid-column: 5">b</p><p>c</p></div><p>a<span class="f">note</span>b<a href="#x" id=x>c</a></p><h1>t</h1>`, `.f { float: footnote } a::after { content: target-counter(attr(href), page) } h1 { string-set: h content(); break-after: right } @page :left { @top-left { content: string(h) } }`)
	f.Add(`<div style="display: inline-block; width: 30px">a b</div><img src=p.png style="width: 50%; object-fit: cover"><ul><li style="list-style: outside">a</li></ul><div style="line-clamp: 2">a b c d e f g h i j</div><p style="text-align: justify; text-indent: 2em">a b c d e f g h i j k</p>`, `p { width: 40px; orphans: 2; widows: 2 } div { overflow: hidden; height: 10px } @page { size: 100px 60px }`)
	f.Add(`<div style="opacity: .5; transform: rotate(30deg) scale(2)">a</div><p style="box-shadow: 2px 3px 4px 1px red, inset 1px 1px blue; border-radius: 5px">b</p><span style="text-decoration: underline wavy">c</span>`, `div { background: repeating-radial-gradient(ellipse, red, rgba(0,0,255,.5) 3px); outline: 2px dashed } p { border: 3px double green; background: url(p.png) space, url(p.svg) round 1px 2px / 5px }`)
	f.Add(`<div>a</div><p>b</p>`, `div { border-image: url(p.svg) 30% round; border: 4px solid; mask-border: url(p.png) 1 fill } p { border: 2px dotted red; border-radius: 3px 10px; clip: rect(1px, 2px, 3px, 4px); position: absolute; text-overflow: ellipsis; overflow: hidden; white-space: nowrap; width: 5px } @page { bleed: 3px; marks: crop cross }`)
	wp, err := os.ReadFile("testdata/weasyprint.otf")
	if err != nil {
		f.Fatal(err)
	}
	pattern, err := os.ReadFile("testdata/pattern.png")
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, doc, css string) {
		if len(doc) > 1<<12 || len(css) > 1<<12 {
			return
		}
		fsys := fstest.MapFS{
			"self.css": {Data: []byte(css + `@import "self.css";`)},
			"x.css":    {Data: []byte(css)},
			"wp.otf":   {Data: wp},
			"p.png":    {Data: pattern},
			"p.svg":    {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="4" height="4">` + doc + `</svg>`)},
		}
		html := fakeHTML(t, doc, "file:///doc.html", nil)
		html.URLFetcher = &URLFetcher{FS: fsys}
		html.Limits = ivy.Limits{MaxObjects: 1 << 12, MaxDepth: 8, MaxPixels: 1 << 16}.WithDefaults()
		fc := testFontConfig(nil)
		targetCollector := NewTargetCollector(nil)
		styleFor, err := GetAllComputedStyles(html, ComputedStylesOptions{
			UserStylesheets:     []CSSSource{{Text: css, BaseURL: "file:///user.css"}},
			PresentationalHints: true,
			FontConfig:          fc,
			TargetCollector:     targetCollector,
		})
		if err != nil {
			if !errors.Is(err, ivy.ErrLimit) {
				t.Fatal(err)
			}
			return
		}
		computeAll(styleFor, html.EtreeElement)
		cs := userAgentStylesheets().counterStyle.Copy()
		for _, name := range []string{"decimal", "x", "upper-roman", "disc", "armenian"} {
			cs.RenderValue(-12, String(name))
			cs.RenderMarker(String(name), 123456)
		}

		root, footnotes, err := BuildFormattingStructure(html, styleFor, BuildOptions{TargetCollector: targetCollector})
		if err != nil {
			if !errors.Is(err, ivy.ErrLimit) {
				t.Fatal(err)
			}
			return
		}
		splitAllLines(t, root)

		document := NewDocument(html, root, styleFor, DocumentOptions{
			FontConfig:      fc,
			TargetCollector: targetCollector,
			Footnotes:       footnotes,
		})
		for i := range 3 {
			page, err := document.Page(context.Background(), i)
			if err != nil {
				if !errors.Is(err, ErrNoPage) && !errors.Is(err, ivy.ErrLimit) {
					t.Fatal(err)
				}
				break
			}
			fuzzDraw(t, page, fc, html.Limits)
		}
	})
}

// fuzzDraw draws a page onto a pixmap of 64 by 64 pixels. The drawing must
// succeed, or draw the page in part for a limit or a malformed resource.
func fuzzDraw(t *testing.T, page *Box, fc *FontConfiguration, limits ivy.Limits) {
	t.Helper()
	dst, ok := okage.NewPixmap(64, 64)
	if !ok {
		t.Fatal("no pixmap")
	}
	area := page.BleedArea()
	scale := 64 / max(area[2], area[3], 1)
	toDevice := okage.TransformFromScale(float32(scale), float32(scale)).
		PreTranslate(float32(-area[0]), float32(-area[1]))
	err := DrawPage(context.Background(), display.NewOkage(dst, okage.IdentityTransform()), page, toDevice,
		DrawOptions{FontConfig: fc, Limits: limits})
	if err == nil {
		return
	}
	// DrawPage turns a panic into an error. The fuzz test still fails on it.
	if _, ok := errors.AsType[*ivy.PartialError](err); !ok || strings.Contains(err.Error(), "panic") ||
		(!errors.Is(err, ivy.ErrLimit) && !errors.Is(err, ivy.ErrFormat)) {
		t.Fatal(err)
	}
}

// splitAllLines splits each text box of a box tree into lines, as the
// layout of the lines does: with a width of 3em, and from the resume
// index of each line. Each line must use some text.
func splitAllLines(t *testing.T, root *Box) {
	t.Helper()
	root.Descendants(func(box *Box) bool {
		if box.Kind != KindText {
			return true
		}
		fontSize := num(box.Style.Get("font_size"))
		width := 3 * fontSize
		text := box.Text
		for skip := 0; skip < len(text); {
			m := SplitFirstLine(text[skip:], box.Style, &width, 0, skip == 0, false)
			if !m.HasResume {
				break
			}
			if m.ResumeIndex <= 0 {
				t.Fatalf("no progress in %q at %d", text, skip)
			}
			skip += m.ResumeIndex
		}
		return true
	})
}

// TestLoadHTMLCharset checks that a document of HTML in an encoding that
// the parser of okage does not decode itself, such as the code page of a
// CHM file, is decoded with internal/charset.
func TestLoadHTMLCharset(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		exp  string
	}{
		{"windows-1251", []byte("<meta charset=\"windows-1251\"><p>\xcf\xf0\xe8\xe2\xe5\xf2"), "Привет"},
		{"koi8-r", []byte("<meta http-equiv=\"Content-Type\" content=\"text/html; charset=koi8-r\"><p>\xf0\xd2\xc9\xd7\xc5\xd4"), "Привет"},
		{"shift_jis", []byte("<meta charset=\"shift_jis\"><p>\x82\xa0"), "あ"},
		{"windows-1252", []byte("<p>caf\xe9"), "café"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			src := Source{FS: fstest.MapFS{"a.html": {Data: test.data}}}
			html, err := LoadHTML(src, "a.html", HTMLOptions{})
			if err != nil {
				t.Fatal(err)
			}
			p := findElement(html.EtreeElement, "p")
			if p == nil {
				t.Fatal("expected a p element")
			}
			if p.Text != test.exp {
				t.Errorf("expected %q, got: %q", test.exp, p.Text)
			}
		})
	}
}
