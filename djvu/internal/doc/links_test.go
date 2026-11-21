// ivy: this file is not in dejapdf. It tests the parts of
// internal/convert/links.go that the copy keeps in links.go.

package doc

import "testing"

func TestMapAreaOf(t *testing.T) {
	for _, c := range []struct {
		in   string
		want MapArea
		ok   bool
	}{
		{`(maparea "http://a" "" (rect 1 2 3 4))`, MapArea{"http://a", [4]float64{1, 2, 4, 6}}, true},
		{`(maparea (url "#2" "_blank") "c" (oval 1 2 3 4) (border 1))`, MapArea{"#2", [4]float64{1, 2, 4, 6}}, true},
		{`(maparea "x" "" (text 1 2 3 4))`, MapArea{"x", [4]float64{1, 2, 4, 6}}, true},
		{`(maparea "x" "" (poly 1 2 5 9 3 0))`, MapArea{"x", [4]float64{1, 0, 5, 9}}, true},
		{`(maparea "x" "" (line 5 9 1 2))`, MapArea{"x", [4]float64{1, 2, 5, 9}}, true},
		{`(maparea "x" "" (line 1 2 5 2))`, MapArea{}, false},
		{`(maparea "x" "" (poly 1 2 5 9))`, MapArea{}, false},
		{`(maparea "x" "" (rect 1 2 0 4))`, MapArea{}, false},
		{`(maparea "x" "" (rect 1 2 NaN 4))`, MapArea{}, false},
		{`(maparea "x" "" (rect 1 2 3 Inf))`, MapArea{}, false},
		{`(maparea "x" "" (rect 1 2 (3) 4))`, MapArea{}, false},
		{`(maparea "x" "" rect)`, MapArea{}, false},
		{`(maparea "" "" (rect 1 2 3 4))`, MapArea{}, false},
		{`(maparea (url) "" (rect 1 2 3 4))`, MapArea{}, false},
		{`(maparea "x" "")`, MapArea{}, false},
		{`(metadata (title "x"))`, MapArea{}, false},
	} {
		e := ParseSexps([]byte(c.in))
		if len(e) != 1 {
			t.Fatalf("%s: expected one expression, got: %d", c.in, len(e))
		}
		got, ok := MapAreaOf(e[0])
		if ok != c.ok || got != c.want {
			t.Errorf("%s: expected %v %v, got: %v %v", c.in, c.want, c.ok, got, ok)
		}
	}
}

func TestTargets(t *testing.T) {
	dir := &Dir{Components: []Component{
		{ID: "dict.djbz", Name: "dict.djbz", Title: "dict.djbz", Type: Include},
		{ID: "p1.djvu", Name: "p1.djvu", Title: "1", Type: PageComp},
		{ID: "p2.djvu", Name: "p2.djvu", Title: "cover", Type: PageComp},
		{ID: "p3.djvu", Name: "p3.djvu", Title: "p3.djvu", Type: PageComp},
	}}
	tg := NewTargets(dir, 3)
	for _, c := range []struct {
		url  string
		cur  int
		page int
		uri  string
	}{
		{"#cover", 0, 1, ""},
		{"#p3.djvu", 0, 2, ""},
		{"#1", 2, 0, ""},
		{"#2", 0, 1, ""},
		{"#+1", 1, 2, ""},
		{"#-1", 1, 0, ""},
		{"#+2", 1, -1, ""},
		{"#4", 0, -1, ""},
		{"#0", 0, -1, ""},
		{"#dict.djbz", 0, -1, ""},
		{"#", 0, -1, ""},
		{"http://example.com/#1", 0, -1, "http://example.com/#1"},
	} {
		page, uri := tg.Resolve(c.url, c.cur)
		if page != c.page || uri != c.uri {
			t.Errorf("%s from %d: expected %d %q, got: %d %q", c.url, c.cur, c.page, c.uri, page, uri)
		}
	}
	// a page of the directory past the pages of the document is no target
	if page, _ := NewTargets(dir, 1).Resolve("#cover", 0); page != -1 {
		t.Errorf("expected no page, got: %d", page)
	}
}
