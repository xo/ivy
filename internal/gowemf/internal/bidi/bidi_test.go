// Copied from github.com/mgilbir/gowemf, file internal/bidi/bidi_test.go, at commit
// 377668cb7b13 (D7, docs/UPSTREAM.md). Its MIT license is in
// internal/gowemf/LICENSE.
// ivy: the tests that read the downloaded files of Unicode, and the test that
// runs the Go command, are left out.

package bidi

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func classesOf(runes []rune) ([]Class, []Bracket) {
	c := make([]Class, len(runes))
	b := make([]Bracket, len(runes))
	for i, r := range runes {
		c[i], b[i] = Lookup(r), LookupBracket(r)
	}
	return c, b
}

func levelString(levels []int8) string {
	f := make([]string, len(levels))
	for i, l := range levels {
		if l == Removed {
			f[i] = "x"
		} else {
			f[i] = strconv.Itoa(int(l))
		}
	}
	return strings.Join(f, " ")
}

func orderString(order []int) string {
	f := make([]string, len(order))
	for i, o := range order {
		f[i] = strconv.Itoa(o)
	}
	return strings.Join(f, " ")
}

// TestExamples checks UAX #9's own examples, in its upper-case-is-R notation,
// so make check covers the algorithm without the downloaded files.
func TestExamples(t *testing.T) {
	// The examples use upper case for strong right-to-left letters.
	toRunes := func(s string) []rune {
		var out []rune
		for _, r := range s {
			if r >= 'A' && r <= 'Z' {
				r = 0x05d0 + (r - 'A') // Hebrew letters stand in for R
			}
			out = append(out, r)
		}
		return out
	}
	visual := func(s string, dir Direction) string {
		runes := toRunes(s)
		classes, brackets := classesOf(runes)
		levels, _ := Resolve(classes, brackets, dir)
		mirror := map[rune]rune{'(': ')', ')': '(', '[': ']', ']': '['}
		var b strings.Builder
		for _, i := range Reorder(levels) {
			r := runes[i]
			if r >= 0x05d0 && r < 0x05d0+26 {
				r = 'A' + (r - 0x05d0)
			}
			if m, ok := mirror[r]; ok && levels[i]%2 == 1 {
				r = m // L4, as the examples display it
			}
			b.WriteRune(r)
		}
		return b.String()
	}
	for _, c := range []struct {
		storage, display string
		dir              Direction
	}{
		{"car means CAR.", "car means RAC.", LeftToRight},
		{"he said \"THE VALUES ARE 123, 456, 789, OK\".", "he said \"KO ,789 ,456 ,123 ERA SEULAV EHT\".", LeftToRight},
		{"IT IS A bmw 500, OK.", ".KO ,bmw 500 A SI TI", RightToLeft},
		{"AB(CD[&ef]!)gh", "gh(![ef&]DC)BA", RightToLeft},
		{"book(s) ARABIC", "book(s) CIBARA", LeftToRight},
		{"ARABIC book(s)", "book(s) CIBARA", RightToLeft},
	} {
		if got := visual(c.storage, c.dir); got != c.display {
			t.Errorf("%q: %q, want %q", c.storage, got, c.display)
		}
	}
}

// TestLevels checks hand-resolved cases for rules the examples above do not
// reach.
func TestLevels(t *testing.T) {
	for _, c := range []struct {
		text   string
		dir    Direction
		levels string
	}{
		{"\u05d0 $12", RightToLeft, "1 1 2 2 2"},        // W5: a terminator before European numbers
		{"\u0661,\u0662", LeftToRight, "2 2 2"},         // W4: a common separator between Arabic numbers
		{"a\u202bb ", LeftToRight, "0 x 2 0"},           // L1: trailing whitespace inside an embedding
		{"\u0661\u202b\u0300", LeftToRight, "2 x 1"},    // sos and eos from the explicit levels
		{"\u05d0\u2066a\u2069b", Auto, "1 1 2 1 2"},     // P2 skips isolates; isolates resolve as neutrals
		{"a\u2067\u05d0", LeftToRight, "0 0 1"},         // an unmatched isolate initiator
		{"\u202ea\u05d0\u202c", LeftToRight, "x 1 1 x"}, // X4: an override
		// BD16: 63 openers fit the stack, so the last pairs with the closer
		// and takes the preceding L context; a 64th empties the pair list.
		{"a" + strings.Repeat("(", 63) + "b)", RightToLeft, strings.TrimSpace(strings.Repeat("2 ", 66))},
		{"a" + strings.Repeat("(", 64) + "b)", RightToLeft, strings.Repeat("2 ", 66) + "1"},
	} {
		classes, brackets := classesOf([]rune(c.text))
		levels, _ := Resolve(classes, brackets, c.dir)
		if got := levelString(levels); got != c.levels {
			t.Errorf("%+q: %q, want %q", c.text, got, c.levels)
		}
	}
}

func TestLookup(t *testing.T) {
	for r, want := range map[rune]Class{'a': L, 0x05d0: R, 0x0627: AL, '1': EN, '+': ES, '$': ET, 0x0661: AN, ',': CS, 0x0300: NSM, 0x200d: BN, '\n': B, '\t': S, ' ': WS, '!': ON, 0x202a: LRE, 0x202d: LRO, 0x202b: RLE, 0x202e: RLO, 0x202c: PDF, 0x2066: LRI, 0x2067: RLI, 0x2068: FSI, 0x2069: PDI, 0x05eb: R, 0x10ffff: BN, -1: L} {
		if got := Lookup(r); got != want {
			t.Errorf("U+%04X: %v, want %v", r, got, want)
		}
	}
	if b := LookupBracket('('); b != (Bracket{BracketOpen, '('}) {
		t.Fatal(b)
	}
	if b := LookupBracket(')'); b != (Bracket{BracketClose, '('}) {
		t.Fatal(b)
	}
	if LookupBracket(0x232a) != LookupBracket(0x3009) || LookupBracket(0x2329).Key != 0x3008 {
		t.Fatal("canonical angle brackets")
	}
	if b := LookupBracket('a'); b.Kind != BracketNone {
		t.Fatal(b)
	}
}

func ExampleReorder() {
	runes := []rune("abc \u05d0\u05d1\u05d2 123")
	classes, brackets := classesOf(runes)
	levels, _ := Resolve(classes, brackets, RightToLeft)
	fmt.Println(levelString(levels))
	fmt.Println(Reorder(levels))
	// Output:
	// 2 2 2 1 1 1 1 1 2 2 2
	// [8 9 10 7 6 5 4 3 0 1 2]
}
