// Copyright (c) 2011-2021, Simon Sapin and contributors.
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// This file ports tests/css/test_layers.py and tests/css/test_nesting.py.
//
// Upstream reads the used widths of the boxes of a rendered page. The tests
// here read the computed widths of the elements, which are the used widths
// for a width in pixels.

package layout

import (
	"strings"
	"testing"
)

// TestLayers is test_layers.
func TestLayers(t *testing.T) {
	for _, style := range []string{
		`@layer { div { width: 100px } }`,
		`@layer a { div { width: 100px } }`,
		`
    div { width: 100px }
    @layer a { div { width: 200px } }
    `,
		`
    @layer { div { width: 200px } }
    @layer a { div { width: 100px } }
    `,
		`
    @layer a { div { width: 200px } }
    @layer { div { width: 100px } }
    `,
		`
    @layer a { div { width: 200px } }
    @layer a { div { width: 100px } }
    `,
		`
    @layer a { div { width: 100px } }
    @layer a.b { div { width: 200px } }
    `,
		`
    @layer a.b { div { width: 200px } }
    @layer a { div { width: 100px } }
    `,
		`
    @layer a { div { width: 200px } }
    @layer b { div { width: 100px } }
    `,
		`
    @layer b, a;
    @layer a { div { width: 100px } }
    @layer b { div { width: 200px } }
    `,
		`
    @layer b;
    @layer a { div { width: 100px } }
    @layer b { div { width: 200px } }
    `,
		`
    @import url(data:text/css,div{width:100px});
    @layer a { div { width: 200px } }
    `,
		`
    @import url(data:text/css,div{width:200px}) layer;
    @layer a { div { width: 100px } }
    `,
		`
    @import url(data:text/css,div{width:200px}) layer(b);
    @layer a { div { width: 100px } }
    `,
		`
    @import url(data:text/css,div{width:100px}) layer(a);
    @layer a.b { div { width: 200px } }
    `,
		`
    @import url(data:text/css,div{width:200px}) layer(a.b);
    @layer a { div { width: 100px } }
    `,
		`
    @layer a { div { width: 100px } }
    @import url(data:text/css,div{width:200px}) layer(a.b);
    `,
	} {
		document, styleFor, capture := cascade(t, `
      <style>
        `+style+`
      </style>
      <div>abc</div>
    `)
		div := styleOf(t, document, styleFor, "div")
		if got := div.Get("width"); !Equal(got, Dimension{100, "px"}) {
			t.Errorf("%s: got width %s", style, ValueString(got))
		}
		assertNoLogs(t, capture)
	}
}

// TestNestingBlock is test_nesting_block.
func TestNestingBlock(t *testing.T) {
	for _, style := range []string{
		"div { p { width: 10px } }",
		"p { div & { width: 10px } }",
		"p { width: 20px; div & { width: 10px } }",
		"p { div & { width: 10px } width: 20px }",
		"div { & { & { p { & { width: 10px } } } } }",
		"@media print { div { p { width: 10px } } }",
		"div { em, p { width: 10px } }",
		"p { a, div & { width: 10px } }",
	} {
		document, styleFor, capture := cascade(t, `
      <style>`+style+`</style>
      <div><p></p></div><p></p>
    `)
		ps := elementsByTag(document.EtreeElement, "p")
		if got := styleFor.Get(ps[0], "").Get("width"); !Equal(got, Dimension{10, "px"}) {
			t.Errorf("%s: div p: got width %s", style, ValueString(got))
		}
		if got := styleFor.Get(ps[1], "").Get("width"); Equal(got, Dimension{10, "px"}) {
			t.Errorf("%s: p: got width %s", style, ValueString(got))
		}
		assertNoLogs(t, capture)
	}
}

// TestNestingTooDeep checks the guard of the nesting of style rules: a
// nesting deeper than the limit is ignored with a warning, and does not
// recurse without end.
func TestNestingTooDeep(t *testing.T) {
	style := strings.Repeat("div { ", 200) + strings.Repeat("} ", 200)
	_, styleFor, capture := cascade(t, "<style>"+style+"</style><div></div>")
	if styleFor == nil {
		t.Fatal("expected styles")
	}
	if len(capture.logs()) == 0 {
		t.Error("expected a warning")
	}
}
