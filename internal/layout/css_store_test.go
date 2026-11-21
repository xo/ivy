package layout

import (
	"slices"
	"testing"
)

// TestStyleStore tests the values that a style holds by the ids of the
// properties, and the custom properties beside them.
func TestStyleStore(t *testing.T) {
	s := &Style{}
	s.store("width", Int(1))
	s.store("color", String("red"))
	s.store("margin_top", Int(2))
	s.store("__custom", String("x"))
	s.store("width", Int(3))
	for _, test := range []struct {
		key  string
		want Value
	}{
		{"width", Int(3)},
		{"color", String("red")},
		{"margin_top", Int(2)},
		{"__custom", String("x")},
	} {
		if got, ok := s.lookup(test.key, propertyInfos[test.key]); !ok || !Equal(got, test.want) {
			t.Errorf("%s: got %v %t, want %v", test.key, got, ok, test.want)
		}
	}
	if s.Has("height") {
		t.Errorf("height: the style holds a value")
	}
	keys := s.storedKeys()
	slices.Sort(keys)
	if want := []string{"__custom", "color", "margin_top", "width"}; !slices.Equal(keys, want) {
		t.Errorf("expected keys %v, got: %v", want, keys)
	}

	c := s.Copy()
	c.Delete("color")
	if c.Has("color") || !s.Has("color") {
		t.Errorf("Delete of a copy: copy %t, style %t", c.Has("color"), s.Has("color"))
	}
	if got, _ := c.lookup("margin_top", propertyInfos["margin_top"]); !Equal(got, Int(2)) {
		t.Errorf("margin_top after Delete: got %v", got)
	}

	d := &Style{}
	d.store("height", Int(4))
	d.merge(s)
	keys = d.storedKeys()
	slices.Sort(keys)
	if want := []string{"__custom", "color", "height", "margin_top", "width"}; !slices.Equal(keys, want) {
		t.Errorf("expected keys after merge %v, got: %v", want, keys)
	}
}

// TestPropertySetRank tests the ranks of the ids of a set in each word.
func TestPropertySetRank(t *testing.T) {
	var p propertySet
	ids := []int{0, 5, 63, 64, 130, 255}
	for _, id := range ids {
		p.add(id)
	}
	for i, id := range ids {
		if got := p.rank(id); got != i {
			t.Errorf("rank(%d): expected %d, got: %d", id, i, got)
		}
	}
	if got := slices.Collect(p.all()); !slices.Equal(got, ids) {
		t.Errorf("expected ids %v, got: %v", ids, got)
	}
	p.clear(64)
	if p.has(64) || p.rank(130) != 3 {
		t.Errorf("clear(64): has %t, rank(130) %d", p.has(64), p.rank(130))
	}
}

// TestSlab tests that the values of a slab are new and distinct.
func TestSlab(t *testing.T) {
	var s slab[Box]
	seen := map[*Box]bool{}
	for range 3 * maxSlab {
		b := s.next()
		if seen[b] || b.Kind != 0 || b.Text != "" {
			t.Fatalf("slab gave a box that is not new")
		}
		b.Text = "used"
		seen[b] = true
	}
}
