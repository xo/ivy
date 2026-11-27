package docx

import (
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// TestSymbolCodes checks that the text of a bullet of a symbol font takes
// the characters of the private area, where the font maps them.
func TestSymbolCodes(t *testing.T) {
	tests := []struct {
		s, exp string
		ok     bool
	}{
		{"", "", true},
		{"§", "", true},
		{"o", "", true},
		{"", "", false},
		{"•", "", false},
	}
	for _, test := range tests {
		got, ok := symbolCodes(test.s)
		if got != test.exp || ok != test.ok {
			t.Errorf("%q: expected %q %t, got %q %t", test.s, test.exp, test.ok, got, ok)
		}
	}
}

// TestSymbolNotdef checks that a character that the font Symbol does not
// have, in w:sym or in a bullet, draws as the empty box of the font, as
// F094 on page 1 of poi/docx/form-footnotes.docx.
func TestSymbolNotdef(t *testing.T) {
	tests := []struct {
		font string
		r    rune
		exp  rune
	}{
		{"Symbol", 0xf094, notdefRune},
		{"Symbol", 0x94, notdefRune},
		{"Symbol", 0xf0ff, notdefRune},
		{"Symbol", 0xf0b7, '•'},
		{"Times New Roman", 0xf094, 0x94},
		{"Times New Roman", notdefRune, 0},
	}
	for _, test := range tests {
		if got := symbolRune(test.font, test.r); got != test.exp {
			t.Errorf("%s %U: expected %U, got %U", test.font, test.r, test.exp, got)
		}
	}
	numbering := fixture.WordPart{ID: "rIdN", Type: "numbering", Name: "word/numbering.xml", Data: fixture.WordXML("w:numbering",
		`<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/><w:lvlText w:val="`+"\uf094"+`"/>`+
			`<w:pPr><w:ind w:left="360" w:hanging="360"/></w:pPr><w:rPr><w:rFonts w:ascii="Symbol" w:hAnsi="Symbol"/></w:rPr></w:lvl></w:abstractNum>`+
			`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)}
	body := `<w:p><w:r><w:t xml:space="preserve">OF </w:t></w:r><w:r><w:sym w:font="Symbol" w:char="F094"/></w:r></w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>Item</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body, numbering))
	if n := strings.Count(html, notdefBox); n != 2 {
		t.Errorf("expected 2 empty boxes, got %d:\n%s", n, html)
	}
}

// TestSymbolBulletLine checks that the bullet of the font Symbol gives its
// line the height of a line of Symbol, as Word does: the lines of the table
// INSTRUCTIONS of poi/docx/form-footnotes.docx are 11.03 points apart at 9
// points, not the 10.35 points of Times New Roman.
func TestSymbolBulletLine(t *testing.T) {
	numbering := fixture.WordPart{ID: "rIdN", Type: "numbering", Name: "word/numbering.xml", Data: fixture.WordXML("w:numbering",
		`<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/><w:lvlText w:val="`+"\uf0a8"+`"/>`+
			`<w:pPr><w:ind w:left="360" w:hanging="360"/></w:pPr><w:rPr><w:rFonts w:ascii="Symbol" w:hAnsi="Symbol"/></w:rPr></w:lvl></w:abstractNum>`+
			`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)}
	body := `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr><w:spacing w:line="360" w:lineRule="auto"/></w:pPr>` +
		`<w:r><w:rPr><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="18"/></w:rPr><w:t>Item</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body, numbering))
	if want := "min-width: 18pt; text-indent: 0; line-height: 1.84; font-size: 10pt"; !strings.Contains(html, want) {
		t.Errorf("expected %q:\n%s", want, html)
	}
}

// TestCourierBulletLine checks that a bullet of Courier New, whose line is
// lower than the line of its text, does not make the line higher, as in
// Word: the "o" lines of page 3 of aspose/docx/rendering.docx are 13.8
// points apart, the line of Times New Roman at 12 points.
func TestCourierBulletLine(t *testing.T) {
	numbering := fixture.WordPart{ID: "rIdN", Type: "numbering", Name: "word/numbering.xml", Data: fixture.WordXML("w:numbering",
		`<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/><w:lvlText w:val="o"/>`+
			`<w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr><w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New"/></w:rPr></w:lvl></w:abstractNum>`+
			`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)}
	body := `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr>` +
		`<w:r><w:rPr><w:rFonts w:ascii="Times New Roman" w:hAnsi="Times New Roman"/><w:sz w:val="24"/></w:rPr><w:t>Item</w:t></w:r></w:p>`
	html, _ := convertHTML(t, fixture.DOCX(body, numbering))
	if want := "text-indent: 0; line-height: 0;"; !strings.Contains(html, want) {
		t.Errorf("expected %q:\n%s", want, html)
	}
}
