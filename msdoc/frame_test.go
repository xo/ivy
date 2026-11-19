package msdoc

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
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
		name string
		pp   pap
		want string
	}{
		// 2 inches wide at 1 inch from the column, below 10 points of
		// its paragraph: the text wraps at its right
		{"paragraph", frame(0, pcVertParagraph, 1440, 200, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 10pt 0pt 0pt 72pt">`},
		// at 1 inch from the page, which is at the left margin
		{"page", frame(pcHorzPage, pcVertParagraph, 1800, 0, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt">`},
		// at the right of the column
		{"right", frame(0, pcVertParagraph, dxaAbsRight, 0, 2880, 0),
			`<div style="width: 144pt; float: right; margin: 0pt 0pt 0pt 0pt">`},
		// 2 inches below the top of the margin: a page float (D60)
		{"margin", frame(0, 0, 0, 2880, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: margin 144pt">`},
		// 3 inches below the top of the page
		{"page top", frame(0, pcVertPage, 0, 4320, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: page 216pt">`},
		// the special values of the position
		{"margin top", frame(0, 0, 0, dyaAbsTop, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: margin top">`},
		{"page center", frame(0, pcVertPage, 0, dyaAbsCenter, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: page center">`},
		{"margin bottom", frame(0, 0, 0, dyaAbsBottom, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: margin bottom">`},
		{"inside", frame(0, 0, 0, dyaAbsInside, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: margin top">`},
		{"outside", frame(0, 0, 0, dyaAbsOutside, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: margin bottom">`},
		// above and below the text at a position from the margin: a page
		// float with a margin to the right side of the column of 6
		// inches
		{"margin top and bottom", frame(0, 0, 1440, 2880, 2880, wrTopBottom),
			`<div style="width: 144pt; float: left; margin: 0pt 216pt 0pt 72pt; -ivy-float-position: margin 144pt">`},
		// at the center of the column
		{"margin center", frame(0, 0, dxaAbsCenter, 2880, 2880, 0),
			`<div style="width: 144pt; float: left; margin: 0pt 144pt 0pt 144pt; -ivy-float-position: margin 144pt">`},
		// of the width of its text: the width of the column
		{"margin auto", frame(0, 0, 0, 2880, 0, 0),
			`<div style="width: 432pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: margin 144pt">`},
		// no text beside the frame: a block at its position
		{"top and bottom", frame(0, pcVertParagraph, 1440, 200, 2880, wrTopBottom),
			`<div style="height: 10pt"></div><div style="width: 144pt; margin-left: 72pt">`},
		// the text as if the frame were not there: on the page
		{"none", frame(0, pcVertPage, 1440, 4320, 2880, wrNone),
			`<div style="width: 144pt; position: absolute; left: 72pt; top: 144pt">`},
		// a frame of the width of its text is a block
		{"auto", frame(0, pcVertParagraph, 720, 0, 0, 0),
			`<div style="margin-left: 36pt">`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := testParser(nil, nil)
			p.page = letter
			var b strings.Builder
			p.writeFrame(&b, tt.pp, "<p>x</p>")
			if html := b.String(); !strings.HasPrefix(html, tt.want) {
				t.Errorf("expected %q:\n%s", tt.want, html)
			}
		})
	}
}

// TestFrameParagraphs checks that the paragraphs of the same frame share a
// block, and that the block is at its position from the margin.
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
	if want := "-ivy-float-position: margin 72pt"; !strings.Contains(html, want) {
		t.Errorf("expected %q:\n%s", want, html)
	}
}

// TestPageFloatFiles checks the position on the page of the floats of the
// files of D60, which LibreOffice wrote from the .docx files of the tests
// of docx: the tables from sprmTPc and sprmTDyaAbs, and the frames and the
// picture, which LibreOffice wrote as shapes, from their places from the
// margin (D60).
func TestPageFloatFiles(t *testing.T) {
	for _, tt := range []struct {
		name string
		want []string
	}{
		{"t-late", []string{"-ivy-float-position: margin 72pt"}},
		{"t-page2", []string{"-ivy-float-position: margin 72pt"}},
		{"f-late", []string{"-ivy-float-position: margin 70.25pt"}},
		{"s-late", []string{"-ivy-float-position: margin 72pt"}},
		{"f-bottom", []string{"-ivy-float-position: margin 502.55pt"}},
		{"t-bottom", []string{"-ivy-float-position: margin bottom"}},
		{"t-two", []string{"-ivy-float-position: margin 72pt", "-ivy-float-position: margin 72pt"}},
		{"t-loop3", []string{"-ivy-float-position: margin 0pt"}},
		{"t-loop2", []string{"-ivy-float-position: margin 0pt"}},
		{"t-nofit", []string{"-ivy-float-position: page 684pt"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", "float-"+tt.name+".doc"))
			if err != nil {
				t.Fatal(err)
			}
			d, err := Open(t.Context(), bytes.NewReader(buf))
			if err != nil {
				t.Fatal(err)
			}
			src, err := d.source(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			b, err := fs.ReadFile(src.FS, src.Documents[0])
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(string(b), "-ivy-float-position"); got != len(tt.want) || !strings.Contains(string(b), tt.want[0]) {
				t.Errorf("got %d positions, want %q", got, tt.want)
			}
			if n, err := d.PageCount(t.Context()); err != nil || n < 2 {
				t.Errorf("got %d pages, %v", n, err)
			}
		})
	}
}
