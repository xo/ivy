// This file is not a port. It tests the keys of the caches that the port
// makes with strconv: they must give the text that fmt gives.

package layout

import (
	"fmt"
	"strings"
	"testing"
)

// TestPageTypeKey tests that the key of a page type is the text of fmt.
func TestPageTypeKey(t *testing.T) {
	for _, p := range []PageType{
		{},
		{Side: "left", Blank: true, Name: "chapter", Index: 12},
		{Side: "right", Name: "a \"quoted\"\nname\x00\u00e9", Index: -3, Groups: []PageGroup{{"g", 1}, {"h\t", 22}}},
	} {
		var b strings.Builder
		fmt.Fprintf(&b, "%q %t %q %d", p.Side, p.Blank, p.Name, p.Index)
		for _, g := range p.Groups {
			fmt.Fprintf(&b, " %q %d", g.Name, g.Index)
		}
		if got, want := p.key(), b.String(); got != want {
			t.Errorf("key() = %q, want %q", got, want)
		}
	}
}

// TestValueStringQuotes tests that ValueString quotes as fmt does.
func TestValueStringQuotes(t *testing.T) {
	for _, s := range []string{"", "serif", "a \"b\"\n\x00\u00e9\u200b"} {
		if got, want := ValueString(String(s)), fmt.Sprintf("%q", s); got != want {
			t.Errorf("ValueString(%q) = %s, want %s", s, got, want)
		}
		d := Dimension{Value: 1.5, Unit: s}
		want := fmt.Sprintf("Dimension(1.5, %q)", s)
		if s == "" {
			want = "Dimension(1.5, None)"
		}
		if got := ValueString(d); got != want {
			t.Errorf("ValueString(%v) = %s, want %s", d, got, want)
		}
	}
	if got := ValueString(Int(-42)); got != "-42" {
		t.Errorf("ValueString(Int(-42)) = %s, want -42", got)
	}
}

// TestFontKeysCleared tests that Set clears the font keys that a style
// keeps.
func TestFontKeysCleared(t *testing.T) {
	style := newInitialStyle(&StyleContext{})
	before := fontStyleCacheKey(style, true)
	if got := fontStyleCacheKey(style, true); got != before {
		t.Fatalf("second key %q, want %q", got, before)
	}
	style.Set("font_size", Float(42))
	after := fontStyleCacheKey(style, true)
	if after == before {
		t.Fatalf("key %q did not change after Set", after)
	}
	if want := fontKey(style) + ValueString(style.Get("font_size")) + ValueString(style.Get("line_height")); after != want {
		t.Errorf("key %q, want %q", after, want)
	}
}
