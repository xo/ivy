package rtf

import (
	"bytes"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// sample returns an RTF file with each feature that the tests check: a page
// of 5 by 4 inches with margins of half an inch, a header with the number
// of the page, a table, a picture, a footnote, text in three code pages and
// in Unicode, and a page break.
func sample() []byte {
	png := hex.EncodeToString(fixture.PNG(40, 20, color.NRGBA{R: 255, A: 255}))
	return []byte(`{\rtf1\ansi\ansicpg1252\deff0
{\fonttbl{\f0\fswiss\fcharset0 Arial;}{\f1\froman\fcharset204 Times New Roman;}{\f2\fmodern\fcharset128 MS Gothic;}{\f3\ftech\fcharset2 Symbol;}}
{\colortbl;\red255\green0\blue0;\red0\green0\blue255;}
{\info{\title A title}{\author An author}{\creatim\yr2024\mo3\dy14\hr9\min30}}
\paperw7200\paperh5760\margl720\margr720\margt1080\margb720\headery360
{\header\pard\qr\fs20 Page \chpgn\par}
\pard\qc\b\fs32 Heading\par
\pard\plain\fs24 Plain {\b bold} {\i\cf1 red italic} caf\'e9 {\f1 \'c0\'c1} {\f2 \'82\'a0} \uc1\u8364?euro {\v hidden}{\f3 \'b7}.\par
{\pard\li720\fi-360{\listtext\'95\tab}Item\par}
\pard Note{\super\chftn}{\footnote\pard\plain\fs20{\super\chftn} The note.}\par
\trowd\trgaph108\cellx1440\cellx2880\pard\intbl A\cell B\cell\row
\trowd\trgaph108\clbrdrt\brdrs\cellx1440\cellx2880\pard\intbl C\cell D\cell\row
\pard {\field{\*\fldinst HYPERLINK "https://example.com/"}{\fldrslt link}} {\*\unknown skipped} {\pict\pngblip\picw40\pich20\picwgoal1440\pichgoal720 ` + png + `}\par
\page Second page\par
}`)
}

// convertHTML converts an RTF file and returns its XHTML and the rules of
// its pages.
func convertHTML(t *testing.T, buf []byte) (string, string) {
	t.Helper()
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(src.FS, documentName)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), src.StyleSheets[1]
}

func TestConvert(t *testing.T) {
	html, sheet := convertHTML(t, sample())
	for _, want := range []string{
		`font-weight: bold">bold</span>`,
		`font-style: italic; color: #ff0000">red italic</span>`,
		`café`,
		`АБ`,
		`あ`,
		`€euro`,
		`•`,
		`text-align: center`,
		`<span style="display: inline-block; min-width: 18pt; text-indent: 0">`,
		`<span class="fn"><p `,
		`<span class="fnref"></span><span style="font-family: &quot;Arial&quot;, sans-serif; font-size: 10pt"> The note.</span>`,
		`<col style="width: 72pt"/>`,
		`<td style="padding-left: 5.4pt; padding-right: 5.4pt">`,
		`border-top: 0.5pt solid black`,
		`<a href="https://example.com/">`,
		`<img src="media/1.png" alt="" style="width: 72pt; height: 36pt"/>`,
		`break-before: page`,
		`position: running(hf1); display: flow-root`,
		`Page </span><span class="pg"></span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected the xhtml to hold %q", want)
		}
	}
	for _, unwanted := range []string{"hidden", "skipped", "?euro"} {
		if strings.Contains(html, unwanted) {
			t.Errorf("expected the xhtml without %q", unwanted)
		}
	}
	if t.Failed() {
		t.Log(html)
	}
	if want := `@page s1 { size: 360pt 288pt; margin: 54pt 36pt 36pt 36pt; -ivy-margin-box-fit: grow; @top-center { content: element(hf1); padding-top: 18pt }`; !strings.Contains(sheet, want) {
		t.Errorf("expected the style sheet to hold %q, got:\n%s", want, sheet)
	}
}

// inkBox returns the box of the pixels of img that are darker than mid
// grey, in the rectangle r.
func inkBox(img image.Image, r image.Rectangle) image.Rectangle {
	var box image.Rectangle
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if (cr+cg+cb)/3 < 0x8000 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

func TestPages(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(sample()))
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.PageCount(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 pages, got: %d", n)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := p.Size(); w != 360 || h != 288 {
		t.Errorf("expected a page of 360 by 288 points, got: %v by %v", w, h)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	header := inkBox(img, image.Rect(0, 0, 360, 54))
	if header.Empty() || header.Min.X < 250 || header.Min.Y < 17 {
		t.Errorf("expected the header at the right of the top margin, got: %v", header)
	}
	if body := inkBox(img, image.Rect(0, 54, 360, 252)); body.Empty() || body.Min.X < 35 {
		t.Errorf("expected the text inside the margins, got: %v", body)
	}
}

// TestTallHeader checks that a header that is taller than the top margin
// pushes the text of the page below it, as Word does.
func TestTallHeader(t *testing.T) {
	// a page of 5 by 4 inches, with a top margin of 0.5 inch and a header
	// of four lines at 0.25 inch from the top
	buf := []byte(`{\rtf1\ansi\deff0{\fonttbl{\f0 Times New Roman;}}\paperw7200\paperh5760\margl720\margr720\margt720\margb720\headery360` +
		`{\header\pard\fs24 one\par two\par three\par four\par}\pard\fs24 Body\par}`)
	d, err := Open(t.Context(), bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Page(t.Context(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := ivy.RenderImage(t.Context(), p, 72, color.White)
	if err != nil {
		t.Fatal(err)
	}
	// the header ends at about 18 + 4 * 14 = 74 points
	header := inkBox(img, image.Rect(0, 0, 360, 80))
	body := inkBox(img, image.Rect(0, 80, 360, 288))
	if header.Empty() || body.Empty() || body.Min.Y < header.Max.Y || body.Min.Y > 100 {
		t.Errorf("expected the text below the header, got the header %v and the text %v", header, body)
	}
}

// TestCellWords checks the cells of libreoffice/rtf/landscape-a4-table.rtf:
// \clpadt is the left padding, as Word reads it, \brdrw gives the width of
// a border, and a paragraph takes the font size of its mark, which gives
// the least height of its lines.
func TestCellWords(t *testing.T) {
	doc := `{\rtf1\ansi\deff0{\fonttbl{\f0 Times New Roman;}{\f1 Arial;}}` +
		`\trowd\clbrdrt\brdrs\brdrw30\clpadft3\clpadt108\clpadfr3\clpadr108\cellx1440` +
		`\pard\plain\intbl\f1\fs20{\fs16 small}\cell\row\pard\plain Body\par}`
	html, _ := convertHTML(t, []byte(doc))
	for _, want := range []string{
		"border-top: 1.5pt solid black",
		"padding-left: 5.4pt; padding-right: 5.4pt",
		`<p style="font-family: &quot;Arial&quot;, serif; font-size: 10pt; line-height: 1.15; -ivy-line-multiple: 1"><span style="font-family: &quot;Arial&quot;, serif; font-size: 8pt">small</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the xhtml:\n%s", want, html)
		}
	}
}

// TestCellBorderPadding checks that a cell keeps its whole width for its
// padding and its text, as in Word: the padding gives back the half of
// each side border that the layout takes from the cell, so that the narrow
// cells of libreoffice/rtf/landscape-a4-table.rtf do not wrap "x 2".
func TestCellBorderPadding(t *testing.T) {
	doc := `{\rtf1\ansi\deff0{\fonttbl{\f0 Times New Roman;}}` +
		`\trowd\trgaph108\clbrdrl\brdrs\brdrw40\clbrdrr\brdrs\cellx1440\cellx2880` +
		`\pard\plain\intbl A\cell B\cell\row\pard\plain Body\par}`
	html, _ := convertHTML(t, []byte(doc))
	for _, want := range []string{
		// 108 twips less 20 at the left, and less 5 at the right
		"border-right: 0.5pt solid black; border-left: 2pt solid black; padding-left: 4.4pt; padding-right: 5.15pt",
		`<td style="padding-left: 5.4pt; padding-right: 5.4pt">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the xhtml:\n%s", want, html)
		}
	}
}

func TestDescribe(t *testing.T) {
	d, err := Open(t.Context(), bytes.NewReader(sample()))
	if err != nil {
		t.Fatal(err)
	}
	info, err := d.Describe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2024, 3, 14, 9, 30, 0, 0, time.UTC)
	if info.Title != "A title" || len(info.Authors) != 1 || info.Authors[0] != "An author" || !info.Created.Equal(want) {
		t.Errorf("expected the title, the author and the date of creation, got: %+v", info)
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(t.Context(), bytes.NewReader([]byte("not rtf"))); !errors.Is(err, ivy.ErrFormat) {
		t.Errorf("expected ivy.ErrFormat, got: %v", err)
	}
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 16}}
	if _, err := Open(t.Context(), bytes.NewReader(sample()), WithConfig(cfg)); !errors.Is(err, ivy.ErrLimit) {
		t.Errorf("expected ivy.ErrLimit, got: %v", err)
	}
}

func TestDeepGroups(t *testing.T) {
	const depth = 10000
	buf := []byte(`{\rtf1 before` + strings.Repeat("{", depth) + "deep" + strings.Repeat("}", depth) + ` after\par}`)
	html, _ := convertHTML(t, buf)
	if strings.Contains(html, "deep") || !strings.Contains(html, "before") || !strings.Contains(html, "after") {
		t.Errorf("expected the groups past the depth left out, and the rest kept, got:\n%s", html)
	}
}

func TestUnbalanced(t *testing.T) {
	for _, s := range []string{`{\rtf1 open {\b bold`, `{\rtf1 }}}} closed`, `{\rtf1\u`, `{\rtf1\'`, `{\rtf1{\pict\bin999999 ab}}`} {
		d, err := Open(t.Context(), bytes.NewReader([]byte(s)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.PageCount(t.Context()); err != nil {
			t.Errorf("expected pages for %q, got: %v", s, err)
		}
	}
}

func TestPaper(t *testing.T) {
	landscape := ivy.A4.Landscape()
	for _, test := range []struct {
		name  string
		file  string
		paper ivy.Paper
		exp   ivy.Paper
	}{
		{"default", `{\rtf1\ansi Text\par}`, ivy.Paper{}, ivy.Letter},
		{"no paper", `{\rtf1\ansi Text\par}`, landscape, landscape},
		{"landscape word", `{\rtf1\ansi\landscape Text\par}`, ivy.A4, landscape},
		{"own paper", `{\rtf1\ansi\paperw7200\paperh5760 Text\par}`, landscape, ivy.Paper{Width: 360, Height: 288}},
		{"own section", `{\rtf1\ansi\sectd\pgwsxn5760\pghsxn7200 Text\par}`, landscape, ivy.Paper{Width: 288, Height: 360}},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, err := Open(t.Context(), strings.NewReader(test.file), WithConfig(ivy.Config{Paper: test.paper}))
			if err != nil {
				t.Fatal(err)
			}
			fixture.CheckPaper(t, d, test.exp)
		})
	}
	// the margins of RTF stay on the paper of the program
	d, err := Open(t.Context(), strings.NewReader(`{\rtf1\ansi Text\par}`), WithConfig(ivy.Config{Paper: landscape}))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if exp := "size: 841.89pt 595.28pt; margin: 72pt 90pt 72pt 90pt;"; !strings.Contains(src.StyleSheets[1], exp) {
		t.Errorf("expected %q in the rules of the pages, got:\n%s", exp, src.StyleSheets[1])
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(sample())
	f.Add([]byte(`{\rtf1\ansi{\fonttbl{\f0 Arial;}}\trowd\cellx100\intbl a\cell\row\sect\sectd\pgwsxn100 b\par}`))
	cfg := ivy.Config{Limits: ivy.Limits{MaxMemory: 1 << 20, MaxPixels: 1 << 16, MaxObjects: 1 << 14, MaxDepth: 32}}
	f.Fuzz(func(t *testing.T, buf []byte) {
		d, err := Open(t.Context(), bytes.NewReader(buf), WithConfig(cfg))
		if err != nil {
			return
		}
		_, _ = d.Describe(t.Context())
		p, err := d.Page(t.Context(), 0)
		if err != nil {
			return
		}
		_, _ = ivy.RenderImage(t.Context(), p, 9, nil)
	})
}

// TestTableGrid checks that the rows of a table with cells of other edges
// share the columns of all their edges, that a border word with no line
// draws no border, and that the header rows repeat on each page
// (libreoffice/rtf/landscape-a4-table.rtf and table-repeat.rtf).
func TestTableGrid(t *testing.T) {
	doc := `{\rtf1\ansi` +
		`\trowd\trhdr\clbrdrt\brdrtbl\clbrdrb\brdrs\cellx1440\cellx4320\pard\intbl Head\cell More\cell\row` +
		`\trowd\cellx1440\cellx2880\cellx4320\pard\intbl A\cell B\cell C\cell\row` +
		`\pard After\par}`
	html, _ := convertHTML(t, []byte(doc))
	for _, want := range []string{
		`<col style="width: 72pt"/><col style="width: 72pt"/><col style="width: 72pt"/>`,
		`<td colspan="2"`, `<thead>`, `border-bottom: 0.5pt solid black`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the xhtml:\n%s", want, html)
		}
	}
	if strings.Contains(html, "border-top") {
		t.Errorf("expected no top border:\n%s", html)
	}
}

// TestEastAsianFont checks that a run gives the font of \dbch\af after its
// font, when that font has the \fcharset of an East Asian font, and the
// language of \langfe or of the \fcharset on its East Asian text
// (libreoffice/rtf/chart-outline.rtf).
func TestEastAsianFont(t *testing.T) {
	doc := `{\rtf1\ansi\deff0{\fonttbl{\f0\froman\fcharset0 Times New Roman;}{\f1\fswiss\fcharset0 Arial;}` +
		`{\f14\froman\fcharset136 PMingLiU;}{\f15\fmodern\fcharset128 MS Mincho;}}` +
		`\pard\plain\f1\langfe1028\loch\af1\hich\af1\dbch\af14 \u27161?\u38988? text\par` +
		`\pard\plain\f1\dbch\af0 Latin only\par` +
		`\pard\plain\f1\dbch\af15 \u12354?\par}`
	html, _ := convertHTML(t, []byte(doc))
	for _, want := range []string{
		`<span lang="zh-TW" style="font-family: &quot;Arial&quot;, &quot;PMingLiU&quot;, sans-serif; font-size: 12pt">標題 text</span>`,
		`<span style="font-family: &quot;Arial&quot;, sans-serif; font-size: 12pt">Latin only</span>`,
		`<span lang="ja" style="font-family: &quot;Arial&quot;, &quot;MS Mincho&quot;, sans-serif; font-size: 12pt">あ</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the xhtml:\n%s", want, html)
		}
	}
}

// TestVerticalMerge checks that the cells of \clvmgf and \clvmrg merge
// into one cell of rows (libreoffice/rtf/landscape-a4-table.rtf).
func TestVerticalMerge(t *testing.T) {
	doc := `{\rtf1\ansi` +
		`\trowd\clvmgf\cellx1440\cellx2880\pard\intbl Merged\cell B1\cell\row` +
		`\trowd\clvmrg\cellx1440\cellx2880\pard\intbl\cell B2\cell\row` +
		`\trowd\cellx1440\cellx2880\pard\intbl A3\cell B3\cell\row}`
	html, _ := convertHTML(t, []byte(doc))
	if !strings.Contains(html, `<td rowspan="2" style="padding-left: 5.4pt; padding-right: 5.4pt"><p `) {
		t.Errorf("expected a cell of two rows:\n%s", html)
	}
	if n := strings.Count(html, "<td"); n != 5 {
		t.Errorf("expected 5 cells, got %d:\n%s", n, html)
	}
}

// TestNestedTable checks that a table of \itap2 goes into the cell of the
// table around it, with the row of \nesttableprops, and that the text of
// \nonesttables is left out.
func TestNestedTable(t *testing.T) {
	doc := `{\rtf1\ansi` +
		`\trowd\cellx2880\cellx5760` +
		`\pard\intbl\itap2 N1\nestcell N2\nestcell{\*\nesttableprops\trowd\cellx1000\cellx2000\nestrow}{\nonesttables\par}` +
		`\pard\intbl Outer\cell Right\cell` +
		`\trowd\cellx2880\cellx5760\row\pard After\par}`
	html, _ := convertHTML(t, []byte(doc))
	if n := strings.Count(html, "<table"); n != 2 {
		t.Fatalf("expected 2 tables, got %d:\n%s", n, html)
	}
	inner := strings.LastIndex(html, "<table")
	outerCell := strings.Index(html, "<td")
	if inner < outerCell || !strings.Contains(html[inner:], "N2") || strings.Index(html, "Outer") < inner {
		t.Errorf("expected the nested table in the first cell before its text:\n%s", html)
	}
	if !strings.Contains(html, `<col style="width: 50pt"/>`) || !strings.Contains(html, `<col style="width: 144pt"/>`) {
		t.Errorf("expected the columns of both tables:\n%s", html)
	}
	if !strings.Contains(html, "After") {
		t.Errorf("expected the text after the table:\n%s", html)
	}
}

// TestStyleSheet checks that a paragraph takes the formatting of its style
// and of the style that it is based on, and a run the formatting of its
// character style, before the control words after them.
func TestStyleSheet(t *testing.T) {
	doc := `{\rtf1\ansi\deff0{\fonttbl{\f0\froman Times New Roman;}}` +
		`{\stylesheet{\ql\fs24 Normal;}{\s1\sbasedon2\b\fs32 Heading;}{\s2\qc\i Base;}{\*\cs10\additive\ul Emphasis;}}` +
		`\pard\plain\s1 Title\par` +
		`\pard\plain\s1\qr\b0 Right\par` +
		`\pard\plain Plain {\cs10 under}\par}`
	html, _ := convertHTML(t, []byte(doc))
	for _, want := range []string{
		`text-align: center"><span style="font-family: &quot;Times New Roman&quot;, serif; font-size: 16pt; font-weight: bold; font-style: italic">Title</span>`,
		`text-align: right"><span style="font-family: &quot;Times New Roman&quot;, serif; font-size: 16pt; font-style: italic">Right</span>`,
		`font-size: 12pt; text-decoration-line: underline">under</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q in the xhtml:\n%s", want, html)
		}
	}
	if strings.Contains(html, "Heading") || strings.Contains(html, "Normal") {
		t.Errorf("expected the names of the styles left out:\n%s", html)
	}
}

// TestWatermark checks that the WordArt of a header, such as the watermark
// of libreoffice/rtf/watermark.rtf, shows once on each page, turned and
// centered on the margins, with its text, its fill and its opacity.
func TestWatermark(t *testing.T) {
	shape := `{\shp{\*\shpinst\shpleft0\shptop0\shpright8484\shpbottom4242\shpwr3` +
		`{\sp{\sn shapeType}{\sv 136}}{\sp{\sn rotation}{\sv 20643840}}{\sp{\sn gtextUNICODE}{\sv ASAP}}` +
		`{\sp{\sn gtextFont}{\sv DejaVu Sans}}{\sp{\sn fillColor}{\sv 6108695}}{\sp{\sn fillOpacity}{\sv 32768}}` +
		`{\sp{\sn posh}{\sv 2}}{\sp{\sn posrelh}{\sv 0}}{\sp{\sn posv}{\sv 2}}{\sp{\sn posrelv}{\sv 0}}{\sp{\sn fBehindDocument}{\sv 1}}}` +
		`{\shprslt\par}}`
	doc := `{\rtf1\ansi\paperw12240\paperh15840\margl1440\margr1440\margt1440\margb1440` +
		`{\header\pard ` + shape + `\par}{\headerf\pard ` + shape + `\par}\titlepg\pard Body\par}`
	d, err := Open(t.Context(), bytes.NewReader([]byte(doc)))
	if err != nil {
		t.Fatal(err)
	}
	src, err := d.source(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(src.FS, documentName)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	if n := strings.Count(html, "<img"); n != 1 {
		t.Fatalf("expected one image of the watermark, got %d:\n%s", n, html)
	}
	// a box of 424.2 by 212.1 points turned by 315 degrees, centered on
	// the margins of 468 by 648 points
	if !strings.Contains(html, `style="position: fixed; left: 9.03pt; top: 99.03pt; width: 449.93pt; height: 449.93pt; z-index: -1"`) {
		t.Errorf("expected the image fixed at the center of the margins:\n%s", html)
	}
	svg, err := fs.ReadFile(src.FS, "media/1.svg")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`rotate(315)`, `textLength="424.2"`, `fill="#17365d" fill-opacity="0.5"`, `font-family="'DejaVu Sans', sans-serif"`, `>ASAP</text>`} {
		if !strings.Contains(string(svg), want) {
			t.Errorf("expected %q in the svg:\n%s", want, svg)
		}
	}
}
