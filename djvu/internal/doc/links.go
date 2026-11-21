// Copyright (c) 2026 Serge Vakulenko
// SPDX-License-Identifier: MIT

// Copied from github.com/sergev/dejapdf, file internal/convert/links.go, at
// commit 2b6ca0c1123c (D7, D26, docs/UPSTREAM.md). Its MIT license is in
// djvu/internal/LICENSE.
//
// ivy: the copy keeps the parts of the file that read DjVu: newTargets,
// resolve, the reading of a maparea in links, and area. The parts that
// write PDF are left out, because pdfenc of ivy writes the links (D58).

package doc // ivy: renamed from convert

import (
	"math"
	"strconv"
	"strings"
)

// Targets resolves DjVu link URLs.
//
// ivy: exported, and it holds the number of the pages in place of the
// references of the written pages of upstream.
type Targets struct {
	byName map[string]int
	pages  int
}

// NewTargets returns the targets of the pages of a document: the pages of
// the directory dir, or a single page when dir is nil.
//
// ivy: it takes the directory and the number of the pages in place of a
// whole document, so that a bundled document needs only its directory.
func NewTargets(dir *Dir, pages int) *Targets {
	t := &Targets{byName: map[string]int{}, pages: pages}
	if dir != nil {
		k := 0
		for _, c := range dir.Components {
			if c.Type != PageComp {
				continue
			}
			for _, s := range []string{c.Title, c.Name, c.ID} {
				if _, ok := t.byName[s]; !ok && s != "" {
					t.byName[s] = k
				}
			}
			k++
		}
	}
	return t
}

// Resolve turns url, seen from page cur, into the index of a page, or -1
// and a URI. "#" links name a page id or title, a number from 1, or a
// relative +n or -n. A "#" link that names no page gives -1 and "".
//
// ivy: exported. Every page of the document is a target.
func (t *Targets) Resolve(url string, cur int) (int, string) {
	if !strings.HasPrefix(url, "#") {
		return -1, url
	}
	n := t.pages
	s := url[1:]
	if i, ok := t.byName[s]; ok {
		if i >= n { // ivy: a page of the directory that the document left out
			return -1, ""
		}
		return i, ""
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return -1, ""
	}
	if s[0] == '+' || s[0] == '-' {
		v += cur
	} else {
		v--
	}
	if v < 0 || v >= n {
		return -1, ""
	}
	return v, ""
}

// MapArea is a hyperlink of a page: the URL of a maparea annotation, and
// the bounding box of its shape, (x0, y0, x1, y1) in the pixels of the page
// with y up.
//
// ivy: added. Upstream writes the link of PDF in links.
type MapArea struct {
	URL  string
	Rect [4]float64
}

// MapAreaOf reads the maparea annotation a. It returns false for another
// annotation, for a maparea without a URL, and for a shape without an area.
//
// ivy: the part of links of upstream that reads a maparea.
func MapAreaOf(a *Sexp) (MapArea, bool) {
	if a.Head() != "maparea" || len(a.List) < 4 {
		return MapArea{}, false
	}
	url := a.List[1]
	if url.IsList && url.Head() == "url" && len(url.List) >= 2 {
		url = url.List[1]
	}
	if url.IsList || url.Atom == "" { // ivy: an empty URL is not a link
		return MapArea{}, false
	}
	if !a.List[3].IsList { // ivy: a shape that is not a list
		return MapArea{}, false
	}
	r, ok := area(a.List[3])
	if !ok {
		return MapArea{}, false
	}
	return MapArea{URL: url.Atom, Rect: r}, true
}

// area returns the bounding box of a rect, oval or poly shape.
//
// ivy: it also reads the shapes text, as a rect, and line, as the box of
// its two points when the box has an area.
func area(sh *Sexp) ([4]float64, bool) {
	var v []float64
	for _, a := range sh.List[min(1, len(sh.List)):] {
		f, err := strconv.ParseFloat(a.Atom, 64)
		if a.IsList || err != nil {
			return [4]float64{}, false
		}
		if math.IsNaN(f) || math.IsInf(f, 0) { // ivy: a number that is not finite
			return [4]float64{}, false
		}
		v = append(v, f)
	}
	switch sh.Head() {
	case "rect", "oval", "text": // ivy: text
		if len(v) == 4 && v[2] > 0 && v[3] > 0 {
			return [4]float64{v[0], v[1], v[0] + v[2], v[1] + v[3]}, true
		}
	case "poly", "line": // ivy: line
		if (sh.Head() == "line" && len(v) == 4) || (len(v) >= 6 && len(v)%2 == 0) {
			r := [4]float64{v[0], v[1], v[0], v[1]}
			for i := 2; i < len(v); i += 2 {
				r[0], r[1] = min(r[0], v[i]), min(r[1], v[i+1])
				r[2], r[3] = max(r[2], v[i]), max(r[3], v[i+1])
			}
			if r[2] > r[0] && r[3] > r[1] {
				return r, true
			}
		}
	}
	return [4]float64{}, false
}
