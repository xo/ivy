package msdoc

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"slices"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
	"github.com/xo/ivy/internal/odraw"
)

// The tests of this file come from the faults that the visual review found
// in the files of Word 97 and of Word 6, against the PDFs of Microsoft
// Word.

// u16s returns the values in 2 bytes each, little endian.
func u16s(vs ...uint16) []byte {
	var out []byte
	for _, v := range vs {
		out = binary.LittleEndian.AppendUint16(out, v)
	}
	return out
}

// testParser returns a parser of streams of a document of Word 97 with the
// table stream table, the stream Data data, and room for every pair of the
// FIB.
func testParser(table, data []byte) *parser {
	s := &streams{table: table, data: data, fib: fib{fcLcb: make([][2]uint32, 100)}}
	cfg := ivy.Config{Limits: ivy.Limits{}.WithDefaults()}
	return &parser{s: s, cfg: cfg, lim: cfg.Limits, pictures: map[uint32]*picture{}, styles: parseStyles(nil, 0)}
}

// TestTableRows checks that the rows of a table have the edges, the
// borders, the shading and the merged cells of their TAP
// (libreoffice/doc/landscape-letter-form.doc).
func TestTableRows(t *testing.T) {
	single := []byte{4, 1, 1, 0} // half a point, single, black
	tc := func(grf uint16) []byte {
		b := u16s(grf, 0)
		for range 4 {
			b = append(b, single...)
		}
		return b
	}
	defTable := func(cells ...uint16) sprm {
		// two cells of 2 and 3 inches from -108 twips
		b := append(u16s(0), 2)
		b = append(b, u16s(0xFF94, 2772, 7092)...)
		for _, grf := range cells {
			b = append(b, tc(grf)...)
		}
		binary.LittleEndian.PutUint16(b, uint16(len(b)-1))
		return sprm{op: sprmTDefTable, operand: b}
	}
	first, second := &tap{}, &tap{}
	// the first cell starts the cells that merge, and the cell below it
	// continues them
	applyTap(first, defTable(vertMergeRestart<<5, 0))
	applyTap(second, defTable(vertMergeContinue<<5, 0))
	// a solid black shading of the second cell of the first row
	applyTap(first, sprm{op: sprmTDefTableShd80, operand: append([]byte{4}, u16s(0, 1<<10|1)...)})
	applyTap(first, sprm{op: sprmTDxaGapHalf, operand: u16s(108)})
	var b strings.Builder
	writeTable(&b, []tableRow{{cells: []string{"A1", "B1"}, tap: first}, {cells: []string{"A2", "B2"}, tap: second}}, page{})
	html := b.String()
	for _, want := range []string{
		`margin-left: -5.4pt`, `<col style="width: 144pt"/>`, `<col style="width: 216pt"/>`,
		`rowspan="2"`, `border-top: 0.5pt solid #000000`, `background-color: #000000`,
		`padding: 0pt 5.4pt 0pt 5.4pt`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the table:\n%s", want, html)
		}
	}
	if strings.Contains(html, "A2") {
		t.Errorf("expected the merged cell A2 left out:\n%s", html)
	}
}

// TestShading checks the colors of the patterns of shading.
func TestShading(t *testing.T) {
	for _, test := range []struct {
		v   uint16
		exp shd
	}{
		{0, shd{}},
		// clear with a background of yellow
		{7 << 5, shd{color: 0xFFFF00, set: true}},
		// solid black
		{1<<10 | 1, shd{color: 0, set: true}},
		// 50% of an automatic black over an automatic white
		{8 << 10, shd{color: 0x808080, set: true}},
	} {
		if got := shd80(test.v); got != test.exp {
			t.Errorf("shd80(%#x): expected %+v, got %+v", test.v, test.exp, got)
		}
	}
}

// TestWord6Sprms checks that the modifiers of Word 6 give the properties
// of the characters, with the codes of Word 97 (poi/doc/word95.doc).
func TestWord6Sprms(t *testing.T) {
	// sprmCFBold, sprmCHps of 12 points, sprmCIco red, a modifier of
	// Word 6 that the package does not read, and sprmCIss superscript
	grpprl := []byte{85, 1, 99, 24, 0, 98, 6, 97, 9, 4, 104, 1}
	c := defaultChp()
	base := c
	for s := range word6Sprms(grpprl) {
		applyChp(&c, s, &base)
	}
	if !c.bold || c.hps != 24 || !c.hasColor || c.color != 0xFF0000 || c.iss != 1 {
		t.Errorf("expected bold red superscript text of 12 points, got %+v", c)
	}
	// a code of an unknown size stops the iteration
	n := 0
	for range word6Sprms([]byte{85, 1, 250, 1, 86, 1}) {
		n++
	}
	if n != 1 {
		t.Errorf("expected 1 modifier before the unknown code, got %d", n)
	}
}

// TestLists checks the numbers and the bullets of the lists
// (libreoffice/doc/line-numbers.doc).
func TestLists(t *testing.T) {
	level := func(nfc byte, text ...uint16) []byte {
		b := make([]byte, lvlfSize, lvlfSize+2+2*len(text))
		binary.LittleEndian.PutUint32(b, 1)
		b[4] = nfc
		b = append(b, u16s(uint16(len(text)))...)
		return append(b, u16s(text...)...)
	}
	lstf := func(lsid uint32, simple bool) []byte {
		b := make([]byte, lstfSize)
		binary.LittleEndian.PutUint32(b, lsid)
		if simple {
			b[26] = 1
		}
		return b
	}
	// a list of numbers "1." and "1.1." on two levels, and a simple list of
	// bullets of Symbol
	plf := append(u16s(2), lstf(7, false)...)
	plf = append(plf, lstf(9, true)...)
	lvls := append(level(0, 0, '.'), level(0, 0, '.', 1, '.')...)
	for range 7 {
		lvls = append(lvls, level(0, 0)...)
	}
	lvls = append(lvls, level(nfcBullet, 0xF0B7)...)
	lfo := func(lsid uint32) []byte {
		b := make([]byte, lfoSize)
		binary.LittleEndian.PutUint32(b, lsid)
		return b
	}
	plfLfo := append(binary.LittleEndian.AppendUint32(nil, 2), lfo(7)...)
	plfLfo = append(plfLfo, lfo(9)...)
	table := append(append([]byte{}, plf...), lvls...)
	lfoAt := len(table)
	table = append(table, plfLfo...)
	p := testParser(table, nil)
	p.s.fib.fcLcb[fcPlfLst] = [2]uint32{0, uint32(len(plf))}
	p.s.fib.fcLcb[fcPlfLfo] = [2]uint32{uint32(lfoAt), uint32(len(plfLfo))}
	p.lists = p.readLists()
	var got []string
	for _, para := range [][2]int{{1, 0}, {1, 1}, {1, 1}, {1, 0}, {1, 1}, {2, 0}} {
		text, _, ok := p.marker(para[0], para[1], defaultChp())
		if !ok {
			t.Fatalf("expected a marker for the list %d, level %d", para[0], para[1])
		}
		got = append(got, strings.TrimSuffix(text, "\t"))
	}
	if exp := "1. 1.1. 1.2. 2. 2.1. •"; strings.Join(got, " ") != exp {
		t.Errorf("expected the markers %q, got %q", exp, strings.Join(got, " "))
	}
	for _, test := range []struct {
		n   int
		nfc byte
		exp string
	}{{4, 1, "IV"}, {9, 2, "ix"}, {28, 3, "BB"}, {3, 4, "c"}, {7, 22, "07"}} {
		if got := formatNumber(test.n, test.nfc); got != test.exp {
			t.Errorf("formatNumber(%d, %d): expected %q, got %q", test.n, test.nfc, test.exp, got)
		}
	}
}

// TestHeaderStories checks that each section takes the stories of its
// headers and its footers from the PlcfHdd, and that a section without a
// story takes the story of the section before it.
func TestHeaderStories(t *testing.T) {
	// the 6 stories of the separators, then 6 stories for each of the two
	// sections: the second section has an odd footer of its own only
	cps := []uint32{0, 0, 0, 0, 0, 0, 0, 0, 5, 5, 10, 10, 10, 10, 10, 10, 15, 15, 15, 16}
	p := testParser(u32s(cps...), nil)
	p.s.fib.fcLcb[fcPlcfHdd] = [2]uint32{0, uint32(4 * len(cps))}
	p.s.fib.ccpText, p.s.fib.ccpFtn, p.s.fib.ccpHdd = 100, 20, 16
	got := p.headerStories(2)
	base := uint32(120)
	if got[0][storyOddHeader] != [2]uint32{base, base + 5} || got[0][storyOddFooter] != [2]uint32{base + 5, base + 10} {
		t.Errorf("expected the odd header and footer of the first section, got %v", got[0])
	}
	if got[1][storyOddHeader] != got[0][storyOddHeader] {
		t.Errorf("expected the second section to take the odd header of the first, got %v", got[1])
	}
	if got[1][storyOddFooter] != [2]uint32{base + 10, base + 15} {
		t.Errorf("expected the odd footer of the second section, got %v", got[1])
	}
}

// inlinePicture returns a PICFAndOfficeArtData of a picture of 2 by 1
// inches at 50% and 100%, whose PICF the records follow.
func inlinePicture(records ...[]byte) []byte {
	body := bytes.Join(records, nil)
	picf := make([]byte, picfSize, picfSize+len(body))
	binary.LittleEndian.PutUint32(picf, uint32(picfSize+len(body)))
	binary.LittleEndian.PutUint16(picf[4:], picfSize)
	binary.LittleEndian.PutUint16(picf[28:], 2880)
	binary.LittleEndian.PutUint16(picf[30:], 1440)
	binary.LittleEndian.PutUint16(picf[32:], 500)
	binary.LittleEndian.PutUint16(picf[34:], 1000)
	return append(picf, body...)
}

// TestInlinePicture checks that the character of a picture gives the
// picture of the stream Data, at its size (libreoffice/doc/line-numbers.doc),
// and that a picture with a crop draws as an object of internal/odraw.
func TestInlinePicture(t *testing.T) {
	png := fixture.PNG(4, 2, color.NRGBA{R: 255, A: 255})
	// an OfficeArtBlipPNG of one identifier and a tag
	blip := append(u16s(0x6E0<<4, odraw.RecBlipPNG), binary.LittleEndian.AppendUint32(nil, uint32(16+1+len(png)))...)
	blip = append(blip, make([]byte, 17)...)
	blip = append(blip, png...)
	// an OfficeArtFBSE with the blip in it
	fbse := make([]byte, 36, 36+len(blip))
	fbse = append(fbse, blip...)
	entry := rec(2, 0, odraw.RecFBSE, fbse)
	// the entry in a container, and the entry after the shape of the
	// picture, whose pib is the index of the store of the document and
	// which shows the lower half of the picture
	half := uint32(0x8000)
	shape := fsp(1025, odraw.SptPictureFrame, 0xA00, []uint32{odraw.PropPib, 5, odraw.PropCropTop, half})
	data := append([]byte{0, 0}, inlinePicture(rec(0xF, 0, odraw.RecSpContainer, entry))...)
	at := uint32(len(data))
	data = append(data, inlinePicture(shape, entry)...)
	p := testParser(nil, data)
	pic := p.picture(t.Context(), chp{special: true, picLocation: 2})
	if pic == nil {
		t.Fatal("expected a picture")
	}
	if pic.width != 72 || pic.height != 72 {
		t.Errorf("expected a picture of 72 by 72 points, got %g by %g", pic.width, pic.height)
	}
	if len(p.media) != 1 || !bytes.Equal(p.media[0].data, png) || !strings.HasSuffix(p.media[0].name, ".png") {
		t.Errorf("expected the PNG file in the media, got %d files", len(p.media))
	}
	pic = p.picture(t.Context(), chp{special: true, picLocation: at})
	if pic == nil || !strings.HasPrefix(pic.name, "ivy-object:") || pic.width != 72 || pic.pad != 2 {
		t.Fatalf("expected the object of the picture, got %+v", pic)
	}
	r := renderObject(t, p, pic.name, 1)
	if got := r.at(38, 38); got != [4]uint8{0xff, 0, 0, 0xff} {
		t.Errorf("expected the red picture, got %v", got)
	}
	var b strings.Builder
	p.writeRun(&b, run{pic: pic})
	if want := `style="width: 76.00pt; height: 76.00pt; margin: -2.00pt"`; !strings.Contains(b.String(), want) {
		t.Errorf("expected %q, got: %s", want, b.String())
	}
}

// TestExactRowHeight checks that a row of an exact height does not grow
// with the empty paragraph after the text of a cell, as in Word: the cell
// "Jan 2009 – Dec 2009" on page 2 of
// libreoffice/doc/landscape-letter-form.doc.
func TestExactRowHeight(t *testing.T) {
	exact, least := &tap{}, &tap{}
	applyTap(exact, sprm{op: sprmTDyaRowHeight, operand: u16s(0xFEAC)}) // -340
	applyTap(least, sprm{op: sprmTDyaRowHeight, operand: u16s(340)})
	var b strings.Builder
	writeTable(&b, []tableRow{{cells: []string{"<p>Jan 2009</p><p><br></p>"}, tap: exact}, {cells: []string{"B"}, tap: least}}, page{})
	html := b.String()
	if !strings.Contains(html, `<div style="display: flow-root; height: 17pt"><p>Jan 2009</p><p><br></p></div>`) {
		t.Errorf("expected the cell of the exact row in a block of its height:\n%s", html)
	}
	if strings.Count(html, "<div") != 1 {
		t.Errorf("expected the row of a least height to grow:\n%s", html)
	}
}

// TestEastAsianFont checks that a run gives its East Asian font after its
// font, and the language of its East Asian text, as the package docx does.
func TestEastAsianFont(t *testing.T) {
	p := testParser(nil, nil)
	p.fonts = []font{{name: "Times New Roman", generic: "serif"}, {name: "SimSun", generic: "serif"}}
	c := defaultChp()
	for _, s := range []sprm{
		{op: sprmCRgFtc0, operand: u16s(0)},
		{op: sprmCRgFtc1, operand: u16s(1)},
		{op: sprmCRgLid1, operand: u16s(0x0804)},
	} {
		applyChp(&c, s, &c)
	}
	var b strings.Builder
	p.writeRun(&b, run{text: "中文 text", chp: c})
	p.writeRun(&b, run{text: "Latin", chp: c})
	html := b.String()
	for _, want := range []string{
		`<span lang="zh-CN" style="font-size: 10pt; font-family: &#34;Times New Roman&#34;, &#34;SimSun&#34;, serif">中文 text</span>`,
		`<span style="font-size: 10pt; font-family: &#34;Times New Roman&#34;, &#34;SimSun&#34;, serif">Latin</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the html:\n%s", want, html)
		}
	}
}

// TestParagraphLines checks that a negative indent moves the text into
// the padding of a cell, so that "Division/Program" of the text box of
// libreoffice/doc/landscape-letter-form.doc stays on one line, and that a
// paragraph that holds only a picture that floats keeps the line of its
// mark (libreoffice/doc/landscape-a4-certificate.doc).
func TestParagraphLines(t *testing.T) {
	p := testParser(nil, nil)
	var pp pap
	for _, s := range []sprm{
		{op: sprmPDxaLeft80, operand: u16s(0xFF3A)}, // -198
		{op: sprmPDxaLeft180, operand: u16s(198)},
	} {
		applyPap(&pp, s)
	}
	var b strings.Builder
	p.writeParagraph(&b, paragraph{pap: pp, runs: []run{{text: "Division/Program"}}})
	if html := b.String(); !strings.Contains(html, "margin-left: -9.9pt; text-indent: 9.9pt") {
		t.Errorf("expected the negative indent at the left:\n%s", html)
	}
	b.Reset()
	f := &drawn{floating: floating{pic: &picture{name: "media/1.png", width: 10, height: 10}, wr: 3}}
	p.writeParagraph(&b, paragraph{runs: []run{{drawn: f}}})
	if html := b.String(); !strings.HasSuffix(html, "<br></p>\n") {
		t.Errorf("expected the line of the mark after the picture:\n%s", html)
	}
	if css := p.css(letter); !strings.Contains(css, "body {\n  margin: 0;") {
		t.Errorf("expected a body without the margin of HTML:\n%s", css)
	}
}

// TestTabStops checks that a paragraph has the tab stops of its modifiers,
// with the hanging indent as a stop, and that the text after the bullet of
// a list goes to the tab stop of its level, as in
// libreoffice/doc/landscape-letter-form.doc, where the paragraph has an
// indent of 9.35 points and its level a stop at 18 points.
func TestTabStops(t *testing.T) {
	p := testParser(nil, nil)
	var pp pap
	// stops at 3 inches, centered, and at 6 inches, at the right with
	// dots, then a stop at 1 inch, which the next modifier deletes
	add := slices.Concat([]byte{13, 0, 3}, u16s(4320, 8640, 1440), []byte{tabCenter, tabRight | 1<<3, tabLeft})
	applyPap(&pp, sprm{op: sprmPChgTabsPapx, operand: add})
	applyPap(&pp, sprm{op: sprmPChgTabs, operand: append([]byte{6, 1}, append(u16s(1430, 20), 0)...)})
	if got, exp := p.tabStops(pp, false), "216pt center, 432pt right '.'"; got != exp {
		t.Errorf("expected the stops %q, got %q", exp, got)
	}
	// a hanging indent of 9.35 points, and the stop of a list at 18
	// points
	pp = pap{left: 187, first: -187}
	if got, exp := p.tabStops(pp, true), "9.35pt left"; got != exp {
		t.Errorf("expected the stop of the hanging indent %q, got %q", exp, got)
	}
	applyPap(&pp, sprm{op: sprmPChgTabs, operand: append(append([]byte{5, 0, 1}, u16s(360)...), tabList)})
	if got, exp := p.tabStops(pp, true), "18pt left"; got != exp {
		t.Errorf("expected the stop of the list %q, got %q", exp, got)
	}
	if got, exp := p.tabStops(pp, false), "9.35pt left, 18pt left"; got != exp {
		t.Errorf("expected the stops of a paragraph without a number %q, got %q", exp, got)
	}
	if css := p.css(letter); !strings.Contains(css, "tab-size: 36pt;\n  -ivy-tab-stops: auto;") {
		t.Errorf("expected the default stops of half an inch:\n%s", css)
	}
}

// TestFloatingTable checks that a table that floats is at its position
// from the margin, and from the top of its paragraph
// (libreoffice/doc/legal-table.doc).
func TestFloatingTable(t *testing.T) {
	row := &tap{}
	b := append(u16s(0), 1)
	b = append(b, u16s(0xFF94, 2772)...)
	binary.LittleEndian.PutUint16(b, uint16(len(b)-1))
	for _, s := range []sprm{
		{op: sprmTDefTable, operand: b},
		{op: sprmTPc, operand: []byte{0x60}},
		{op: sprmTDxaAbs, operand: u16s(0xF9DF)}, // -1569
		{op: sprmTDyaAbs, operand: u16s(222)},
	} {
		applyTap(row, s)
	}
	// the table leaves less than half an inch beside it, and is a block
	var w strings.Builder
	writeTable(&w, []tableRow{{cells: []string{"A1"}, tap: row}}, page{width: 2835 + 3000 + 1134, left: 2835, right: 1134})
	html := w.String()
	for _, want := range []string{`<div style="height: 11.1pt"></div>`, `margin-left: -83.85pt`} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the table:\n%s", want, html)
		}
	}
	// in a wider column, the text wraps at the right of the table, at
	// its distance from the text
	applyTap(row, sprm{op: sprmTDxaFromTextRight, operand: u16s(141)})
	w.Reset()
	writeTable(&w, []tableRow{{cells: []string{"A1"}, tap: row}}, page{width: 11906, left: 2835, right: 1134})
	html = w.String()
	if want := `float: left; margin: 11.1pt 7.05pt 0pt -83.85pt`; !strings.Contains(html, want) || strings.Contains(html, "<div") {
		t.Errorf("expected %q in the table:\n%s", want, html)
	}
}

// TestPageFloatTable checks that a table at a position from the margin or
// from the page floats at that position, for each position of sprmTPc and
// sprmTDyaAbs, and that a table too wide for text beside it fills the
// column with its margin (D60).
func TestPageFloatTable(t *testing.T) {
	def := append(u16s(0), 1)
	def = append(def, u16s(0, 2880)...)
	binary.LittleEndian.PutUint16(def, uint16(len(def)-1))
	for _, tt := range []struct {
		name   string
		pc     byte
		dyaAbs uint16
		width  int
		want   string
	}{
		{"margin", 0x00, 1440, 11906, "float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: margin 72pt"},
		{"page", 0x10, 1440, 11906, "float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: page 72pt"},
		{"top", 0x00, 0xFFFC, 11906, "-ivy-float-position: margin top"},
		{"center", 0x10, 0xFFF8, 11906, "-ivy-float-position: page center"},
		{"bottom", 0x00, 0xFFF4, 11906, "-ivy-float-position: margin bottom"},
		{"inside", 0x00, 0xFFF0, 11906, "-ivy-float-position: margin top"},
		{"outside", 0x00, 0xFFEC, 11906, "-ivy-float-position: margin bottom"},
		// a column of 3000 twips leaves no half inch beside the table
		{"narrow", 0x00, 1440, 2835 + 3000 + 1134, "float: left; margin: 0pt 6pt 0pt 0pt; -ivy-float-position: margin 72pt"},
		// the paragraph keeps the position as a margin
		{"paragraph", 0x20, 200, 11906, "float: left; margin: 10pt 0pt 0pt 0pt\""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := &tap{}
			for _, s := range []sprm{
				{op: sprmTDefTable, operand: def},
				{op: sprmTPc, operand: []byte{tt.pc}},
				{op: sprmTDyaAbs, operand: u16s(tt.dyaAbs)},
			} {
				applyTap(row, s)
			}
			var w strings.Builder
			writeTable(&w, []tableRow{{cells: []string{"A1"}, tap: row}}, page{width: tt.width, left: 2835, right: 1134})
			if html := w.String(); !strings.Contains(html, tt.want) || strings.Contains(html, "<div") {
				t.Errorf("expected %q in the table:\n%s", tt.want, html)
			}
		})
	}
}

// TestFrame checks that a paragraph in a frame at the right of the margin
// puts its text at the right, as the number of the page of the footers of
// libreoffice/doc/line-numbers.doc.
func TestFrame(t *testing.T) {
	p := testParser(nil, nil)
	var pp pap
	applyPap(&pp, sprm{op: sprmPDxaAbs, operand: u16s(0xFFF8)})
	applyPap(&pp, sprm{op: sprmPPc, operand: []byte{0x60}})
	var b strings.Builder
	p.writeParagraph(&b, paragraph{pap: pp, runs: []run{{text: "2"}}})
	if html := b.String(); !strings.Contains(html, "text-align: right") {
		t.Errorf("expected the text of the frame at the right:\n%s", html)
	}
}

// TestHeaderFit checks that the pages of a section grow their margins to
// hold their headers and their footers.
func TestHeaderFit(t *testing.T) {
	p := testParser(nil, nil)
	secs := []docSection{{name: "s1", page: sectionPage{page: letter, header: 720, footer: 720}}}
	secs[0].running[storyOddHeader] = "h1"
	css := p.sectionCSS(secs)
	for _, want := range []string{"margin: 72pt 90pt 72pt 90pt; -ivy-margin-box-fit: grow;", "content: element(h1); padding-top: 36pt"} {
		if !strings.Contains(css, want) {
			t.Errorf("expected %q in the rules of the pages:\n%s", want, css)
		}
	}
}
