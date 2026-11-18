// This file is not a port. It tests that the functions of the white space
// give the texts of the regular expressions of upstream.

package layout

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

// TestWhitespaceFunctions compares normalizeLineFeeds, trimLineFeedBlanks
// and collapseSpaces with LINE_FEED_RE, TAB_RE and SPACE_RE of
// weasyprint/formatting_structure/text.py, on fixed and random texts.
func TestWhitespaceFunctions(t *testing.T) {
	lineFeedRE := regexp.MustCompile("\r\n?")
	tabRE := regexp.MustCompile("[\t ]*\n[\t ]*")
	spaceRE := regexp.MustCompile("[\t ]+")
	const random = 2000
	fixed := []string{
		"", " ", "\t", "\n", "\r", "\r\n", "\r\r\n\n", "a", "a b", "a  b", "a\tb", " a ",
		"a \n b", "a \n \n b", "\n \t\n", "a\r\nb\rc", "日本 \t 語\n\t", "\xff\r\n\xe0 \n",
	}
	texts := make([]string, 0, len(fixed)+random)
	texts = append(texts, fixed...)
	r := rand.New(rand.NewPCG(1, 2))
	alphabet := []string{" ", "\t", "\n", "\r", "a", "é", "\xff"}
	for range random {
		text := ""
		for range r.IntN(12) {
			text += alphabet[r.IntN(len(alphabet))]
		}
		texts = append(texts, text)
	}
	for _, text := range texts {
		if got, want := normalizeLineFeeds(text), lineFeedRE.ReplaceAllString(text, "\n"); got != want {
			t.Errorf("normalizeLineFeeds(%q) = %q, want %q", text, got, want)
		}
		if got, want := trimLineFeedBlanks(text), tabRE.ReplaceAllString(text, "\n"); got != want {
			t.Errorf("trimLineFeedBlanks(%q) = %q, want %q", text, got, want)
		}
		if got, want := collapseSpaces(text, false), spaceRE.ReplaceAllString(text, " "); got != want {
			t.Errorf("collapseSpaces(%q, false) = %q, want %q", text, got, want)
		}
		want := spaceRE.ReplaceAllString(strings.ReplaceAll(tabRE.ReplaceAllString(text, "\n"), "\n", " "), " ")
		if got := collapseSpaces(text, true); got != want {
			t.Errorf("collapseSpaces(%q, true) = %q, want %q", text, got, want)
		}
	}
}
