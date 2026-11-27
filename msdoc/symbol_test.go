package msdoc

import (
	"bytes"
	"strings"
	"testing"
)

// TestSymbolNotdef checks that a code that the font Symbol does not have,
// from 0x7F to 0xA0, 0xF0 and 0xFF, draws as the empty box of the font, as
// in Word and in the package docx: in a run of Symbol, in a bullet, and in
// a character of sprmCSymbol. The other codes take their characters.
func TestSymbolNotdef(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(readFile(t, "symbol-notdef.doc")))
	if err != nil {
		t.Fatal(err)
	}
	html := htmlOf(t, d)
	if n := strings.Count(html, notdefBox); n != 2 {
		t.Errorf("expected 2 empty boxes, got %d:\n%s", n, html)
	}
	if !strings.Contains(html, "α") {
		t.Errorf("expected the character α of the code F061:\n%s", html)
	}
	// Word writes a symbol of Insert Symbol as a special character with
	// sprmCSymbol, which gives the font and the code
	p := testParser(nil, nil)
	p.fonts = []font{{name: "Times New Roman"}, {name: "Symbol"}}
	text := []uint16{'O', 'F', ' ', '(', ' ', '(', 0x0D}
	p.s.word = u16s(text...)
	p.s.fib.ccpText = uint32(len(text))
	p.pieces = []piece{{cpEnd: uint32(len(text))}}
	symbol := func(code uint16) []byte {
		b := append(u16s(sprmCFSpec), 1)
		return append(b, append(u16s(sprmCSymbol), u16s(1, code)...)...)
	}
	p.chpx = []fkpRun{
		{fcStart: 0, fcEnd: 6}, {fcStart: 6, fcEnd: 8, grpprl: symbol(0xF094)},
		{fcStart: 8, fcEnd: 10}, {fcStart: 10, fcEnd: 12, grpprl: symbol(0x61)},
	}
	paras, err := p.paragraphs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	var b strings.Builder
	p.writeParagraph(&b, paras[0])
	got := b.String()
	if strings.Count(got, notdefBox) != 1 || !strings.Contains(got, "α") || strings.Contains(got, "(") {
		t.Errorf("expected the empty box and α in place of the characters (, got:\n%s", got)
	}
	if !strings.Contains(got, "line-height: 1.2250") {
		t.Errorf("expected the line of Symbol, got:\n%s", got)
	}
}
