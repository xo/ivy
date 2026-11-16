// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/test_stacking.py: the stacking contexts.

package layout

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// zIndexSource is z_index_source in tests/test_stacking.py.
const zIndexSource = `
  <style>
    @page { size: 10px }
    div, div * { width: 10px; height: 10px; position: absolute }
    article { background: red; z-index: %v }
    section { background: blue; z-index: %v }
    nav { background: lime; z-index: %v }
  </style>
  <div>
    <article></article>
    <section></section>
    <nav></nav>
  </div>`

// flattenBlocksAndCells is flatten_blocks_and_cells in
// tests/test_stacking.py.
func flattenBlocksAndCells(blocksAndCells []BlockAndCells) []string {
	tags := make([]string, 0, len(blocksAndCells))
	for _, item := range blocksAndCells {
		tags = append(tags, item.Box.ElementTag)
		tags = append(tags, flattenBlocksAndCells(item.Children)...)
	}
	return tags
}

// serializeStacking is serialize_stacking in tests/test_stacking.py, as the
// repr of its tuple.
func serializeStacking(context *StackingContext) string {
	children := make([]string, 0, len(context.ZeroZContexts))
	for _, child := range context.ZeroZContexts {
		children = append(children, serializeStacking(child))
	}
	tags := flattenBlocksAndCells(context.BlocksAndCells)
	quoted := make([]string, len(tags))
	for i, tag := range tags {
		quoted[i] = "'" + tag + "'"
	}
	return fmt.Sprintf("('%s', [%s], [%s])", context.Box.ElementTag, strings.Join(quoted, ", "), strings.Join(children, ", "))
}

// TestStackingNested is test_nested in tests/test_stacking.py.
func TestStackingNested(t *testing.T) {
	for _, tc := range []struct{ source, contexts string }{
		{`
      <p id=lorem></p>
      <div style="position: relative">
        <p id=lipsum></p>
      </div>`, "('html', ['body', 'p'], [('div', ['p'], [])])"},
		{`
      <div style="position: relative">
        <p style="position: relative"></p>
      </div>`, "('html', ['body'], [('div', [], []), ('p', [], [])])"},
	} {
		pages := renderPages(t, tc.source)
		page := pages[0]
		html := only(t, page)
		got := serializeStacking(stackingContextFromBox(html, page, nil, map[*Box]*StackingContext{}))
		if got != tc.contexts {
			t.Errorf("got %s, want %s", got, tc.contexts)
		}
	}
}

// TestStackingImageContexts is test_image_contexts in tests/test_stacking.py.
func TestStackingImageContexts(t *testing.T) {
	pages := renderPages(t, `
      <body>Some text: <img style="position: relative" src=pattern.png>`)
	page := pages[0]
	html := only(t, page)
	context := stackingContextFromBox(html, page, nil, map[*Box]*StackingContext{})
	// The image is *not* in this context:
	want := parsePyTrees(t, `[
        ('html', 'Block', [
            ('body', 'Block', [
                ('body', 'Line', [
                    ('body', 'Text', 'Some text: ')])])])]`)
	if got := serialize([]*Box{context.Box}); !slices.EqualFunc(got, want, func(a, b pyTree) bool { return a.String() == b.String() }) {
		t.Errorf("got %s, want %s", pyTrees(got), pyTrees(want))
	}
	// ... but in a sub-context:
	boxes := make([]*Box, 0, len(context.ZeroZContexts))
	for _, c := range context.ZeroZContexts {
		boxes = append(boxes, c.Box)
	}
	want = parsePyTrees(t, `[('img', 'InlineReplaced', '<replaced>')]`)
	if got := serialize(boxes); !slices.EqualFunc(got, want, func(a, b pyTree) bool { return a.String() == b.String() }) {
		t.Errorf("got %s, want %s", pyTrees(got), pyTrees(want))
	}
}

// TestStackingZIndex is test_z_index in tests/test_stacking.py.
func TestStackingZIndex(t *testing.T) {
	for _, tc := range []struct {
		zIndexes [3]any
		color    string
	}{
		{[3]any{3, 2, 1}, "R"},
		{[3]any{1, 2, 3}, "G"},
		{[3]any{1, 2, -3}, "B"},
		{[3]any{1, 2, "auto"}, "B"},
		{[3]any{-1, "auto", -2}, "B"},
	} {
		t.Run(fmt.Sprint(tc.zIndexes), func(t *testing.T) {
			pixels := strings.Repeat(strings.Repeat(tc.color, 10)+"\n", 10)
			assertPixels(t, "z_index", pixels, fmt.Sprintf(zIndexSource, tc.zIndexes[0], tc.zIndexes[1], tc.zIndexes[2]))
		})
	}
}
