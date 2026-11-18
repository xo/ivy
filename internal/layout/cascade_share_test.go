// This file is not a port. It tests that the elements whose computed style
// is in the cache of the cascade share the styles of their pseudo-elements.

package layout

import "testing"

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
