// This file of ivy holds the parts of mathpdf.go and svgimage.go of
// go-tex/engine at the commit 8234996d66f827438f248e8b0059031f52cb9964
// that do not draw with go-pdfkit (D62): the affine maps that
// pdfpicture.go uses, and the reading of the root of an SVG image. The
// driver of ivy draws an SVG image and the specials in ivy_svgdraw.go.

// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage/roxmltree"
)

// affine is a 2×3 matrix [a c e / b d f]; it maps (x,y) → (a·x+c·y+e, b·x+d·y+f).
type affine struct{ a, b, c, d, e, f float64 }

func identity() affine { return affine{1, 0, 0, 1, 0, 0} }

func (m affine) mul(n affine) affine { // m ∘ n (apply n first, then m)
	return affine{
		a: m.a*n.a + m.c*n.b,
		b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d,
		d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e,
		f: m.b*n.e + m.d*n.f + m.f,
	}
}

func (m affine) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// parseTransform handles SVG's transform list — translate(a[,b]) and scale(a[,b])
// (the forms go-tex/math emits) plus rotate(deg[,cx,cy]), matrix(a,b,c,d,e,f) and
// skewX/skewY(deg) (the forms a drawing package's driver emits for an arbitrary
// coordinate map) — composed left-to-right.
func parseTransform(s string) affine {
	m := identity()
	for len(s) > 0 {
		i := strings.IndexByte(s, '(')
		if i < 0 {
			break
		}
		op := strings.TrimSpace(s[:i])
		// ivy: the parenthesis that closes is after the one that opens. A
		// transform such as "a)b(c" made upstream slice out of range.
		j := strings.IndexByte(s[i:], ')')
		if j < 0 {
			break
		}
		j += i
		args := parseFloats(s[i+1 : j])
		s = s[j+1:]
		switch op {
		case "translate":
			tx, ty := arg(args, 0), arg(args, 1)
			m = m.mul(affine{1, 0, 0, 1, tx, ty})
		case "scale":
			sx := arg(args, 0)
			sy := sx
			if len(args) > 1 {
				sy = args[1]
			}
			m = m.mul(affine{sx, 0, 0, sy, 0, 0})
		case "rotate":
			rad := arg(args, 0) * math.Pi / 180
			cos, sin := math.Cos(rad), math.Sin(rad)
			r := affine{cos, sin, -sin, cos, 0, 0}
			if len(args) >= 3 { // rotate about (cx,cy)
				cx, cy := args[1], args[2]
				r = affine{1, 0, 0, 1, cx, cy}.mul(r).mul(affine{1, 0, 0, 1, -cx, -cy})
			}
			m = m.mul(r)
		case "matrix":
			if len(args) >= 6 {
				m = m.mul(affine{args[0], args[1], args[2], args[3], args[4], args[5]})
			}
		case "skewX":
			m = m.mul(affine{1, 0, math.Tan(arg(args, 0) * math.Pi / 180), 1, 0, 0})
		case "skewY":
			m = m.mul(affine{1, math.Tan(arg(args, 0) * math.Pi / 180), 0, 1, 0, 0})
		}
	}
	return m
}

func parseFloats(s string) []float64 {
	var out []float64
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if v, err := strconv.ParseFloat(strings.TrimSpace(f), 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func parseFloat(s string) float64 { v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return v }

func arg(a []float64, i int) float64 {
	if i < len(a) {
		return a[i]
	}
	return 0
}

// svgDefaultSize is the fallback intrinsic size (points) for an SVG whose root has
// neither width/height nor a viewBox.
const svgDefaultSize = 72

// errSVG marks an SVG that could not be parsed (no <svg> root); doIncludegraphics
// turns it into a SourceError at the \includegraphics line.
var errSVG = &graphicsError{"malformed SVG image"}

// svgDataURI reports whether a data: URI carries an SVG payload (its media type
// mentions "svg"), so loadImage can route it to the SVG loader.
func svgDataURI(uri string) bool {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return false
	}
	return strings.Contains(strings.ToLower(uri[:comma]), "svg")
}

// looksLikeSVG sniffs decoded bytes for an SVG root, covering files with an XML
// prolog or DOCTYPE ahead of <svg …>. Binary PNG/JPEG never contains "<svg".
func looksLikeSVG(data []byte) bool {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	return bytes.Contains(head, []byte("<svg"))
}

// loadSVGImage validates the SVG root and returns the verbatim bytes, the "svg"
// format tag and the intrinsic size (points). A missing/broken root is errSVG.
func loadSVGImage(data []byte) (out []byte, format imgFormat, iw, ih int, err error) {
	attrs, ok := svgRootAttrs(data)
	if !ok {
		return nil, 0, 0, 0, errSVG
	}
	iw, ih = svgIntrinsic(attrs)
	return data, imgSVG, iw, ih, nil
}

// svgRootAttrs returns the attributes of the root <svg> element. It fails (ok=false)
// when the document does not parse or its first element is not <svg>.
//
// ivy: it reads the XML with the parser of okage, not with encoding/xml (D7).
func svgRootAttrs(data []byte) (map[string]string, bool) {
	doc, err := roxmltree.ParseRoot(string(data), roxmltree.ParsingOptions{AllowDTD: true, NodesLimit: 1 << 16})
	if err != nil {
		return nil, false
	}
	root := doc.RootElement()
	if root.TagName().Name() != "svg" {
		return nil, false
	}
	m := make(map[string]string)
	for a := range root.Attributes() {
		m[a.Name()] = a.Value()
	}
	return m, true
}

// svgIntrinsic derives the natural size (points, rounded) from the root attributes:
// width/height when both are present, else the viewBox extent, else a default. A
// single present dimension falls back to a square of that size.
func svgIntrinsic(attrs map[string]string) (iw, ih int) {
	w := parseSVGLen(attrs["width"])
	h := parseSVGLen(attrs["height"])
	if w > 0 && h > 0 {
		return roundf(w), roundf(h)
	}
	if vb := parseFloats(attrs["viewBox"]); len(vb) == 4 && vb[2] > 0 && vb[3] > 0 {
		return roundf(vb[2]), roundf(vb[3])
	}
	if w > 0 {
		return roundf(w), roundf(w)
	}
	if h > 0 {
		return roundf(h), roundf(h)
	}
	return svgDefaultSize, svgDefaultSize
}

// svgViewport returns the source coordinate system for PDF drawing: the viewBox
// (min-x, min-y, width, height) when present, else 0,0,width,height, else a default
// square. Paths are authored in this system and mapped into the target box.
func svgViewport(attrs map[string]string) (x, y, w, h float64) {
	if vb := parseFloats(attrs["viewBox"]); len(vb) == 4 {
		return vb[0], vb[1], vb[2], vb[3]
	}
	w = parseSVGLen(attrs["width"])
	h = parseSVGLen(attrs["height"])
	if w > 0 && h > 0 {
		return 0, 0, w, h
	}
	return 0, 0, svgDefaultSize, svgDefaultSize
}

// parseSVGLen reads a leading number from an SVG length ("40", "40px", "2.5pt"),
// ignoring the unit (treated as a user unit / point). Empty or non-numeric → 0.
func parseSVGLen(s string) float64 {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '-' || s[i] == '+' || s[i] == 'e' || s[i] == 'E' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	v, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	return v
}

// roundf rounds a non-negative float to the nearest int.
func roundf(v float64) int { return int(v + 0.5) }

// parseSVGFill parses a fill value. It returns ok=false when the value is empty or
// unrecognised (so the caller keeps the inherited fill); none=true for
// "none"/"transparent" (draw nothing). Recognised: #rgb, #rrggbb, rgb(r,g,b) and a
// table of common named colours.
func parseSVGFill(s string) (color uint32, none, ok bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return 0, false, false
	case "none", "transparent":
		return 0, true, true
	case "currentcolor", "inherit":
		return 0, false, false
	}
	if strings.HasPrefix(s, "#") {
		if c, ok := parseHexColor(s[1:]); ok {
			return c, false, true
		}
		return 0, false, false
	}
	if strings.HasPrefix(s, "rgb(") && strings.HasSuffix(s, ")") {
		nums := parseFloats(s[4 : len(s)-1])
		if len(nums) == 3 {
			return clamp255(nums[0])<<16 | clamp255(nums[1])<<8 | clamp255(nums[2]), false, true
		}
		return 0, false, false
	}
	if c, ok := svgNamedColors[s]; ok {
		return c, false, true
	}
	return 0, false, false
}

// parseHexColor parses a 3- or 6-digit hex colour (no leading '#').
func parseHexColor(s string) (uint32, bool) {
	switch len(s) {
	case 3:
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return 0, false
		}
		r := (v >> 8) & 0xf
		g := (v >> 4) & 0xf
		b := v & 0xf
		return uint32(r<<20 | r<<16 | g<<12 | g<<8 | b<<4 | b), true
	case 6:
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return 0, false
		}
		return uint32(v), true
	}
	return 0, false
}

// clamp255 clamps a float channel to the 0..255 byte range as a uint32.
func clamp255(v float64) uint32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint32(v + 0.5)
}

// svgNamedColors is the subset of SVG/CSS named colours the PDF driver resolves.
var svgNamedColors = map[string]uint32{
	"black": 0x000000, "white": 0xffffff, "red": 0xff0000, "lime": 0x00ff00,
	"green": 0x008000, "blue": 0x0000ff, "yellow": 0xffff00, "cyan": 0x00ffff,
	"aqua": 0x00ffff, "magenta": 0xff00ff, "fuchsia": 0xff00ff, "gray": 0x808080,
	"grey": 0x808080, "silver": 0xc0c0c0, "maroon": 0x800000, "olive": 0x808000,
	"navy": 0x000080, "teal": 0x008080, "purple": 0x800080, "orange": 0xffa500,
	"pink": 0xffc0cb, "brown": 0xa52a2a, "gold": 0xffd700,
}
