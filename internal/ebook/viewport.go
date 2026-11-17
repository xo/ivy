package ebook

import (
	"strconv"
	"strings"

	"github.com/xo/ivy"
)

// maxViewport is the largest width or height of a viewport, in CSS pixels.
// A larger one is not a page, and neither is one below 1 pixel.
const maxViewport = 1 << 15

// ParseViewport returns the width and the height in CSS pixels of a
// viewport of fixed layout, such as "width=1200, height=1600". It returns
// false if s does not give both as numbers from 1 to 32768, as with
// "width=device-width".
func ParseViewport(s string) (width, height float64, ok bool) {
	for _, field := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		key, value, found := strings.Cut(field, "=")
		if !found {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "px"), 64)
		if err != nil || v < 1 || v > maxViewport {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "width":
			width = v
		case "height":
			height = v
		}
	}
	return width, height, width > 0 && height > 0
}

// DocumentViewport returns the content of the meta element viewport in the
// head of the document doc, such as "width=1200, height=1600".
func DocumentViewport(doc string, lim ivy.Limits) (string, bool) {
	tree := ParseHTML(doc, lim)
	if tree == nil {
		return "", false
	}
	for el := range Elements(tree.Root()) {
		switch el.Name() {
		case "meta":
			if name, _ := el.Attr("name"); strings.EqualFold(strings.TrimSpace(name), "viewport") {
				content, ok := el.Attr("content")
				return content, ok
			}
		case "body":
			return "", false
		}
	}
	return "", false
}
