package msdoc

import (
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
)

// TestCharacterBorder checks that a run with a border of its characters
// has the border, a line higher by the border above and below the text,
// and one border with the next run of the same border
// (poi/doc/floating-pictures.doc).
func TestCharacterBorder(t *testing.T) {
	c := defaultChp()
	// a Brc of 8 bytes, after its size: an automatic color, half a point,
	// a single line, and a distance of 1 point from the text
	applyChp(&c, sprm{op: sprmCBrc, operand: []byte{8, 0, 0, 0, 0xFF, 4, 1, 1, 0}}, &c)
	if want := (brc{width: 0.5, typ: 1, space: 1, set: true}); c.border != want {
		t.Fatalf("got the border %+v, want %+v", c.border, want)
	}
	c80 := defaultChp()
	applyChp(&c80, sprm{op: sprmCBrc80, operand: []byte{4, 1, 0, 0}}, &c80)
	if want := (brc{width: 0.5, typ: 1, set: true}); c80.border != want {
		t.Fatalf("got the border %+v, want %+v", c80.border, want)
	}
	p := testParser(nil, nil)
	var b strings.Builder
	p.writeParagraph(&b, paragraph{runs: []run{
		{text: "a ", chp: defaultChp()}, {text: "bor", chp: c}, {text: "der", chp: c}, {text: " b", chp: defaultChp()},
	}})
	html := b.String()
	// a font that lineRatios does not hold has single lines of 1.15 times
	// its size: 11.5pt, with 1.5pt more above and below the text
	for _, want := range []string{
		`<span style="font-size: 10pt; line-height: 14.5pt; border: 0.5pt solid #000000; padding: 1.75pt 1pt; border-right: none; padding-right: 0">bor</span>`,
		`<span style="font-size: 10pt; line-height: 14.5pt; border: 0.5pt solid #000000; padding: 1.75pt 1pt; border-left: none; padding-left: 0">der</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in:\n%s", want, html)
		}
	}
}

// TestLineNumberFont checks that the numbers of the lines have the font of
// the style Line Number over the style Normal.
func TestLineNumberFont(t *testing.T) {
	p := testParser(nil, nil)
	normal := style{stk: stkParagraph, chp: chp{hps: 24, ftc: -1, ftcEA: -1}, done: true}
	p.styles = &styleSheet{styles: []style{normal}, ftc: -1, ftcEA: -1}
	if got, want := p.lineNumberFont(), "; font-size: 12pt"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	lineNumber := style{sti: stiLineNumber, stk: stkCharacter, chpx: append(u16s(sprmCHps), 16, 0)}
	p.styles = &styleSheet{styles: []style{normal, lineNumber}, ftc: -1, ftcEA: -1}
	if got, want := p.lineNumberFont(), "; font-size: 8pt"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSectionColumns checks that a section gives its columns and the space
// between them, and that a column break starts the next column of a
// section with columns, and a page in a section of one column.
func TestSectionColumns(t *testing.T) {
	grpprl := slices.Concat(u16s(sprmSCcolumns, 2), u16s(sprmSDxaColumns, 360))
	plc := slices.Concat(u32s(0, 10), u16s(0), u32s(100), u16s(0), u32s(0))
	p := testParser(plc, nil)
	p.s.fib.fcLcb[fcPlcfSed] = [2]uint32{0, uint32(len(plc))}
	p.s.word = slices.Concat(make([]byte, 100), u16s(uint16(len(grpprl))), grpprl)
	if sp := p.sectionPage(0, ivy.Paper{}); sp.columns != 3 || sp.columnGap != 360 {
		t.Errorf("got %d columns %d twips apart, want 3 columns 360 twips apart", sp.columns, sp.columnGap)
	}
	for _, tt := range []struct {
		columns int
		want    string
	}{{3, `<p class="column">`}, {1, `<p class="page">`}} {
		p.columns = tt.columns
		var b strings.Builder
		p.writeParagraph(&b, paragraph{columnBreak: true, runs: []run{{text: "x", chp: defaultChp()}}})
		if html := b.String(); !strings.HasPrefix(html, tt.want) {
			t.Errorf("with %d columns, expected %q:\n%s", tt.columns, tt.want, html)
		}
	}
}

// TestFramePosition checks that a frame is at its position from the
// column, the margin or the page, and that the text wraps around it.
func TestFramePosition(t *testing.T) {
	// a frame keeps a position that is not negative plus one
	var pp pap
	applyPap(&pp, sprm{op: sprmPDyaAbs, operand: u16s(1)})
	applyPap(&pp, sprm{op: sprmPDxaAbs, operand: u16s(0xFFF8)})
	if pp.dyaAbs != 0 || pp.dxaAbs != dxaAbsRight || !pp.framed {
		t.Errorf("got dyaAbs %d and dxaAbs %d, want 0 and %d", pp.dyaAbs, pp.dxaAbs, dxaAbsRight)
	}
	frame := func(pcHorz, pcVert, dxaAbs, dyaAbs, width int, wr byte) pap {
		return pap{framed: true, pcHorz: pcHorz, pcVert: pcVert, dxaAbs: dxaAbs, dyaAbs: dyaAbs, dxaWidth: width, wr: wr}
	}
	for _, tt := range []struct {
		name    string
		pp      pap
		pageTop bool
		want    string
	}{
		// 2 inches wide at 1 inch from the column, below 10 points of
		// its paragraph: the text wraps at its right
		{"paragraph", frame(0, pcVertParagraph, 1440, 200, 2880, 0), false,
			`<div style="width: 144pt; float: left; margin: 10pt 0pt 0pt 72pt">`},
		// at 1 inch from the page, which is at the left margin
		{"page", frame(pcHorzPage, pcVertParagraph, 1800, 0, 2880, 0), false,
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt">`},
		// at the right of the column
		{"right", frame(0, pcVertParagraph, dxaAbsRight, 0, 2880, 0), false,
			`<div style="width: 144pt; float: right; margin: 0pt 0pt 0pt 0pt">`},
		// 2 inches below the top of the margin, at the top of a page
		{"margin top", frame(0, 0, 0, 2880, 2880, 0), true,
			`<div style="width: 144pt; float: left; margin: 144pt 0pt 0pt 0pt">`},
		// another frame stays at its paragraph
		{"margin", frame(0, 0, 0, 2880, 2880, 0), false,
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt">`},
		// 3 inches below the top of the page, which is 2 inches below the
		// top margin of 1 inch
		{"page top", frame(0, pcVertPage, 0, 4320, 2880, 0), true,
			`<div style="width: 144pt; float: left; margin: 144pt 0pt 0pt 0pt">`},
		// no text beside the frame: a block at its position
		{"top and bottom", frame(0, pcVertParagraph, 1440, 200, 2880, wrTopBottom), false,
			`<div style="height: 10pt"></div><div style="width: 144pt; margin-left: 72pt">`},
		// the text as if the frame were not there: on the page
		{"none", frame(0, pcVertPage, 1440, 4320, 2880, wrNone), false,
			`<div style="width: 144pt; position: absolute; left: 72pt; top: 144pt">`},
		// a frame of the width of its text is a block
		{"auto", frame(0, pcVertParagraph, 720, 0, 0, 0), false,
			`<div style="margin-left: 36pt">`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := testParser(nil, nil)
			p.page = letter
			var b strings.Builder
			p.writeFrame(&b, tt.pp, "<p>x</p>", tt.pageTop)
			if html := b.String(); !strings.HasPrefix(html, tt.want) {
				t.Errorf("expected %q:\n%s", tt.want, html)
			}
		})
	}
}

// TestFrameParagraphs checks that the paragraphs of the same frame share a
// block, and that a frame at the top of a page is at its position from the
// margin.
func TestFrameParagraphs(t *testing.T) {
	p := testParser(nil, nil)
	p.page = letter
	pp := pap{framed: true, dyaAbs: 1440, dxaWidth: 2880}
	text := []run{{text: "x", chp: defaultChp()}}
	html := p.body([]paragraph{
		{pap: pp, runs: text, end: 0x0D}, {pap: pp, runs: text, end: 0x0D}, {runs: text, end: 0x0D},
	}, func(*strings.Builder, paragraph, func()) {})
	if n := strings.Count(html, "<div"); n != 1 {
		t.Errorf("got %d blocks, want 1:\n%s", n, html)
	}
	if want := "margin: 72pt 0pt 0pt 0pt"; !strings.Contains(html, want) {
		t.Errorf("expected %q:\n%s", want, html)
	}
}
