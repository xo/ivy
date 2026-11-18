// This file is not a port. It tests that the elements whose computed style
// is in the cache of the cascade share the styles of their pseudo-elements,
// and that the styles give the values of upstream without storing them.

package layout

import (
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"testing"
)

// TestSharedPseudoStyles tests that two elements with the same style share
// the styles of their pseudo-elements, and that an element with a style
// attribute keeps its own.
func TestSharedPseudoStyles(t *testing.T) {
	p := parseBase(t, `
          <style>p::before { content: "x"; color: red }</style>
          <p>a</p><p>b</p><p style="color: blue">c</p>
        `, testBaseURL, nil)
	var ps []*Element
	for el := range p.html.EtreeElement.IterTag("p") {
		ps = append(ps, el)
	}
	if len(ps) != 3 {
		t.Fatalf("got %d elements p, want 3", len(ps))
	}
	sf := p.styleFor
	first, second, own := sf.Get(ps[0], "before"), sf.Get(ps[1], "before"), sf.Get(ps[2], "before")
	if first == nil || second == nil || own == nil {
		t.Fatalf("styles of ::before: %p %p %p", first, second, own)
	}
	if first != second {
		t.Errorf("the second p has a style of its own for ::before")
	}
	if own == first {
		t.Errorf("the p with a style attribute shares the style of ::before")
	}
	if got := sf.Get(ps[1], "after"); got != nil {
		t.Errorf("::after has a style: %p", got)
	}
}

// TestSharedPseudoStylesAttr tests that the pseudo-elements that share a
// style take attr() from their own elements.
func TestSharedPseudoStylesAttr(t *testing.T) {
	result := parseAll(t, `
          <style>a::after { content: attr(href) }</style>
          <p><a href="one">x</a><a href="two">y</a></p>
        `)
	assertTree(t, result, `[
            ('p', 'Block', [
                ('p', 'Line', [
                    ('a', 'Inline', [
                        ('a', 'Text', 'x'),
                        ('a::after', 'Inline', [
                            ('a::after', 'Text', 'one')])]),
                    ('a', 'Inline', [
                        ('a', 'Text', 'y'),
                        ('a::after', 'Inline', [
                            ('a::after', 'Text', 'two')])])])])]`)
}

// TestSharedPseudoAnchors tests that an element that shares the styles of
// the pseudo-elements of an earlier element collects their anchors again,
// as upstream does.
func TestSharedPseudoAnchors(t *testing.T) {
	log, capture := captureLogs(slog.LevelInfo)
	parseBase(t, `
          <style>p::before { content: "x"; -weasy-anchor: "a" }</style>
          <p>a</p><p>b</p>
        `, testBaseURL, log)
	assertLog(t, capture, "Anchor defined twice")
}

// TestStyleValuesWithoutStore tests that the computed values of the styles
// without the fast paths of propertyInfo, which store each value as
// upstream does, are the values with them.
func TestStyleValuesWithoutStore(t *testing.T) {
	const source = `
          <style>
            html { direction: rtl; --gap: 3px }
            body { margin-inline-start: 2em; font: 12px serif; text-decoration: underline }
            p { border-top-style: solid; padding: var(--gap); page: wide; color: green }
            em { float: left; border-inline-start-width: 4px; border-left-style: dotted }
            span { position: absolute; letter-spacing: 1px; outline-style: solid }
            li::marker { color: red }
            p::before { content: "x"; border-left-width: 2px }
            td { column-rule-style: solid; text-decoration-color: blue }
          </style>
          <body><p>a <em>b</em> <span>c<i>d</i></span></p>
          <ul><li>e</li></ul>
          <table><tr><td>f</td></tr></table>
          <p style="--gap: 5px">g</p></body>
        `
	keys := append(slices.Sorted(maps.Keys(InitialValues)), "__gap", "__other")
	values := func() []string {
		p := parseBase(t, source, testBaseURL, nil)
		var out []string
		for el := range p.html.EtreeElement.Iter() {
			for _, pseudo := range []string{"", "before", "marker"} {
				style := p.styleFor.Get(el, pseudo)
				if style == nil {
					continue
				}
				for _, s := range []*Style{style, style.AnonymousStyle()} {
					for _, key := range keys {
						out = append(out, fmt.Sprintf("%s::%s %d %s %s", el.Tag, pseudo, s.kind, key, ValueString(s.Get(key))))
					}
					for _, key := range []string{"position", "float", "display"} {
						out = append(out, fmt.Sprintf("%s::%s %d specified %s %s", el.Tag, pseudo, s.kind, key, ValueString(s.Specified(key))))
					}
				}
			}
		}
		return out
	}
	got := values()

	saved := propertyInfos
	defer func() { propertyInfos = saved }()
	propertyInfos = map[string]*propertyInfo{}
	for key, info := range saved {
		propertyInfos[key] = &propertyInfo{id: info.id}
	}
	want := values()

	if len(got) != len(want) {
		t.Fatalf("got %d values, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got %s\nwant %s", got[i], want[i])
		}
	}
}

// TestPropertyInfos tests the classes of a few properties.
func TestPropertyInfos(t *testing.T) {
	for _, test := range []struct {
		key                                     string
		constant, fromParent, zeroWidth, isSpec bool
	}{
		{"margin_top", true, false, false, false},
		{"anchor", true, false, false, false},
		{"color", false, true, false, false},
		{"page", false, true, false, false},
		{"text_decoration_line", false, true, false, false},
		{"border_top_width", false, false, true, false},
		{"outline_width", false, false, true, false},
		{"column_rule_width", false, false, true, false},
		{"position", false, false, false, true},
		{"float", false, false, false, true},
		{"display", false, false, false, false},
		{"border_block_start_width", false, false, false, false},
	} {
		info := propertyInfos[test.key]
		if info == nil {
			t.Errorf("%s: no propertyInfo", test.key)
			continue
		}
		if info.constant != test.constant || info.fromParent != test.fromParent ||
			info.zeroWidth != test.zeroWidth || (info.specified != 0) != test.isSpec {
			t.Errorf("%s: got %+v", test.key, *info)
		}
	}
	// The style of the line of a width and its logical properties stop the
	// zero width.
	info, style := propertyInfos["border_left_width"], propertyInfos["border_inline_start_style"]
	if style == nil || info.cascades[style.id/64]&(1<<(style.id%64)) == 0 {
		t.Errorf("border_inline_start_style does not stop the zero border_left_width")
	}
}
