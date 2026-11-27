package rtf

import (
	"bytes"
	"encoding/hex"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/fixture"
)

// TestFloatFiles checks the floats of the files float-*.rtf of testdata
// (D60, D63). README.md of testdata says what each file holds. The places
// of the pictures come from the PDF of LibreOffice 26.8, in points from
// the top left corner of the page. The test checks the declarations of the
// other floats, and that the text goes on the sides of the pictures that
// LibreOffice gives.
func TestFloatFiles(t *testing.T) {
	for _, tt := range []struct {
		name string
		want []string
		// pic is the place of the picture, and left and right are set
		// when the text goes at its left and at its right
		pic         image.Rectangle
		left, right bool
	}{
		{"s-mid", []string{"float: left; margin-left: 162pt; margin-right: 9pt; margin-bottom: 0pt; -ivy-wrap-flow: both 9pt"}, image.Rect(234, 113, 378, 185), true, true},
		{"s-right", []string{"float: right; margin-right: 0pt; margin-left: 9pt; margin-bottom: 0pt; -ivy-wrap-flow: both 9pt"}, image.Rect(396, 113, 540, 185), true, false},
		{"s-largest", []string{"-ivy-wrap-flow: maximum 9pt"}, image.Rect(180, 113, 324, 185), false, true},
		{"x-page", []string{"float: right; margin-right: 108pt; margin-left: 9pt; margin-top: 0pt; -ivy-float-position: page 144pt"}, image.Rectangle{}, false, false},
		{"x-bottom", []string{"float: right; margin-right: 0pt; margin-left: 0pt; margin-top: 0pt; -ivy-float-position: page bottom"}, image.Rectangle{}, false, false},
		{"x-topbottom", []string{"display: block; margin-left: 72pt; margin-bottom: 0pt; width: 144pt; height: 72pt"}, image.Rectangle{}, false, false},
		{"f-late", []string{"width: 216pt; height: 144pt; float: left; margin: 9pt 9pt 9pt 0pt; -ivy-float-position: margin 72pt"}, image.Rectangle{}, false, false},
		{"f-bottom", []string{"-ivy-float-position: margin bottom"}, image.Rectangle{}, false, false},
		{"f-page", []string{"width: 144pt; height: 72pt; float: right; margin: 0pt -72pt 0pt 7.2pt; -ivy-float-position: page 144pt"}, image.Rectangle{}, false, false},
		{"f-para", []string{`width: 144pt; height: 72pt; float: right; margin: 18pt 0pt 7.2pt 7.2pt"`}, image.Rectangle{}, false, false},
		{"f-nowrap", []string{"float: left; margin: 0pt 162pt 0pt 162pt; -ivy-float-position: margin 144pt"}, image.Rectangle{}, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", "float-"+tt.name+".rtf"))
			if err != nil {
				t.Fatal(err)
			}
			html, _ := convertHTML(t, buf)
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Errorf("expected %q, got:\n%s", want, html)
				}
			}
			if tt.pic.Empty() {
				return
			}
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
			var pic image.Rectangle
			b := img.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					if r, g, _, _ := img.At(x, y).RGBA(); r>>8 > 150 && g>>8 < 100 {
						pic = pic.Union(image.Rect(x, y, x+1, y+1))
					}
				}
			}
			if d := pic.Min.Sub(tt.pic.Min); abs(d.X) > 2 || abs(d.Y) > 2 || pic.Dx() < tt.pic.Dx()-2 || pic.Dy() < tt.pic.Dy()-2 {
				t.Errorf("expected the picture at %v, got %v", tt.pic, pic)
			}
			rows := image.Rect(0, tt.pic.Min.Y+4, 0, tt.pic.Max.Y-4)
			left := inkBox(img, image.Rect(72, rows.Min.Y, tt.pic.Min.X-4, rows.Max.Y))
			right := inkBox(img, image.Rect(tt.pic.Max.X+4, rows.Min.Y, 540, rows.Max.Y))
			if !left.Empty() != tt.left || !right.Empty() != tt.right {
				t.Errorf("expected the text at the left %v and at the right %v, got %v and %v", tt.left, tt.right, left, right)
			}
		})
	}
}

// shapeFile returns an RTF file of Letter paper with margins of 1 inch,
// whose paragraph holds a picture of w by 1440 twips at left and top, or of
// 2 by 1 inches when w is 0, in a shape with the control words words and
// the properties props, after its first word. head holds the control
// words of the document.
func shapeFile(head string, left, top, w int, words, props string) []byte {
	if w == 0 {
		w = 2880
	}
	png := hex.EncodeToString(fixture.PNG(4, 2, color.NRGBA{R: 200, G: 60, B: 60, A: 255}))
	return []byte(`{\rtf1\ansi\paperw12240\paperh15840\margl1440\margr1440\margt1440\margb1440` + head + `
\pard First {\shp{\*\shpinst\shpleft` + strconv.Itoa(left) + `\shptop` + strconv.Itoa(top) +
		`\shpright` + strconv.Itoa(left+w) + `\shpbottom` + strconv.Itoa(top+1440) + words + props +
		`{\sp{\sn pib}{\sv {\pict\pngblip\picw4\pich2 ` + png + `}}}}}text\par
}`)
}

// TestShapeFloat checks the place of a picture of a shape for the control
// words and the properties of its shape (D60, D63).
func TestShapeFloat(t *testing.T) {
	sp := func(name, value string) string {
		return `{\sp{\sn ` + name + `}{\sv ` + value + `}}`
	}
	for _, tt := range []struct {
		name, head   string
		left, top, w int
		words, props string
		want         []string
		not          string
	}{
		{"column", "", 1440, 0, 0, `\shpwr2`, "",
			[]string{`<p style="[^"]*"><img`, "float: left; margin-left: 72pt; margin-right: 9pt; margin-bottom: 0pt; -ivy-wrap-flow: both 9pt; width: 144pt; height: 72pt"}, "-ivy-float-position"},
		{"margin", "", 0, 2880, 0, `\shpbxmargin\shpbymargin\shpwr2`, "",
			[]string{"margin-top: 0pt; -ivy-float-position: margin 144pt"}, ""},
		{"page", "", 0, 2880, 0, `\shpbxcolumn\shpbxignore\shpbypara\shpbyignore\shpwr2`, sp("posrelh", "0") + sp("posrelv", "1"),
			[]string{"-ivy-float-position: page 144pt"}, ""},
		{"ignore", "", 0, 2880, 0, `\shpbymargin\shpbyignore\shpwr2`, "", []string{"margin-top: 144pt"}, "-ivy-float-position"},
		{"center", "", 0, 0, 0, `\shpbymargin\shpwr2`, sp("posv", "2"),
			[]string{"-ivy-float-position: margin center"}, ""},
		{"page center", "", 0, 0, 0, `\shpwr2`, sp("posh", "2") + sp("posrelh", "1"),
			[]string{"float: left; margin-left: 162pt"}, ""},
		{"distances", "", 0, 0, 0, `\shpwr4\shpwrk2`, sp("dxWrapDistLeft", "0") + sp("dxWrapDistRight", "254000") + sp("dyWrapDistBottom", "127000"),
			[]string{"margin-right: 20pt; margin-bottom: 10pt; -ivy-wrap-flow: end 20pt"}, ""},
		{"start", "", 4320, 0, 0, `\shpwr2\shpwrk1`, "", []string{"float: right; margin-right: 108pt", "-ivy-wrap-flow: start 9pt"}, ""},
		{"above and below", "", 0, 720, 0, `\shpwr1`, "",
			[]string{"display: block; margin-left: 0pt; padding-top: 36pt"}, "float"},
		{"above and below margin", "", 0, 1440, 0, `\shpbymargin\shpwr1`, "",
			[]string{"float: left; margin: 0pt 324pt 0pt 0pt; -ivy-float-position: margin 72pt"}, "-ivy-wrap-flow"},
		// a shape that leaves less than half an inch beside it has no text
		// beside it
		{"narrow", "", 360, 0, 8400, `\shpwr2`, "", []string{"display: block; margin-left: 18pt"}, "float"},
		{"none", "", 0, 720, 0, `\shpbypage\shpwr3\shpfblwtxt1`, "",
			[]string{"position: absolute; left: 0pt; top: -36pt; z-index: -1"}, "float"},
		{"none bottom", "", 0, 0, 0, `\shpbymargin\shpwr3`, sp("posv", "3") + sp("fBehindDocument", "1"),
			[]string{"position: absolute; left: 0pt; top: 576pt; z-index: -1"}, "float"},
		{"columns", `\cols2`, 0, 0, 0, `\shpbymargin\shpwr2`, "", []string{"float: left"}, "-ivy-float-position"},
		{"inline", "", 0, 0, 0, `\shpwr2`, sp("fPseudoInline", "1"), []string{`First </span><img src="media/1.png" alt="" style="width: 144pt; height: 72pt"/>`}, "float"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := convertHTML(t, shapeFile(tt.head, tt.left, tt.top, tt.w, tt.words, tt.props))
			for _, want := range tt.want {
				if !strings.Contains(html, want) && !matches(want, html) {
					t.Errorf("expected %q, got:\n%s", want, html)
				}
			}
			if tt.not != "" && strings.Contains(html, tt.not) {
				t.Errorf("expected no %q, got:\n%s", tt.not, html)
			}
		})
	}
}

// matches reports whether the regular expression of the test want matches
// html, for a want that holds one.
func matches(want, html string) bool {
	if !strings.Contains(want, "[^") {
		return false
	}
	return regexp.MustCompile(want).MatchString(html)
}

// TestShapeInLine checks that the shapes of a header, of a group of shapes
// and of a text box stay in their lines, and that a text box floats with
// its text and keeps the properties of its shape around a shape in it.
func TestShapeInLine(t *testing.T) {
	png := hex.EncodeToString(fixture.PNG(4, 2, color.NRGBA{R: 200, A: 255}))
	pict := `{\sp{\sn pib}{\sv {\pict\pngblip\picw4\pich2 ` + png + `}}}`
	shape := `{\shp{\*\shpinst\shpright2880\shpbottom1440\shpwr2` + pict + `}}`
	buf := []byte(`{\rtf1\ansi
{\header \pard H ` + shape + `\par}
\pard A {\shpgrp{\*\shpinst\shpright2880\shpbottom1440}` + shape + `}\par
\pard C {\shpgrp{\*\shpinst\shpright2880\shpbottom1440\shpwr2` + shape + shape + `}}\par
\pard B {\shp{\*\shpinst\shpleft4320\shpright7200\shpbottom1440\shpwr2\shpwrk2{\sp{\sn fillColor}{\sv 255}}{\sp{\sn lineWidth}{\sv 25400}}{\shptxt \pard In the box ` + shape + `\par}}}\par
\pard\par
}`)
	html, _ := convertHTML(t, buf)
	if n := strings.Count(html, "float"); n != 1 {
		t.Errorf("expected one float, got %d:\n%s", n, html)
	}
	for _, want := range []string{
		`H </span><img src="media/1.png" alt="" style="width: 144pt; height: 72pt"/>`,
		`A </span><img src="media/2.png" alt="" style="width: 144pt; height: 72pt"/>`,
		`<span style="float: right; margin-right: 72pt; margin-left: 9pt; margin-bottom: 0pt; -ivy-wrap-flow: end 9pt; width: 144pt; height: 72pt; box-sizing: border-box; ` +
			`padding: 3.6pt 7.2pt 3.6pt 7.2pt; background-color: #ff0000; border: 2pt solid #000000">`,
		`C </span><img src="media/3.png" alt="" style="width: 144pt; height: 72pt"/><img src="media/4.png"`,
		`In the box </span><img src="media/5.png"`,
		`</p></span><span style="font-size: 12pt">B </span></p>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected %q, got:\n%s", want, html)
		}
	}
	// a paragraph that holds only a float has the line of its mark
	buf = shapeFile("", 0, 0, 0, `\shpwr2`, "")
	buf = bytes.Replace(buf, []byte("First "), nil, 1)
	buf = bytes.Replace(buf, []byte("}}}}}text"), []byte("}}}}}"), 1)
	if html, _ := convertHTML(t, buf); !strings.Contains(html, `"/><br/></p>`) {
		t.Errorf("expected the line of the mark, got:\n%s", html)
	}
}

// TestFrames checks the frames of the paragraphs (D60): the paragraphs of
// the same frame share it, and a frame of a drop cap, of a table, of a
// section of columns and at its paragraph without a width stays a
// paragraph.
func TestFrames(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		want       []string
		frames     int
	}{
		{"shared", `\pard\posx720\posy1440\absw2880 One\par Two\par\pard Out\par`,
			[]string{`width: 144pt; float: left; margin: 0pt 0pt 0pt 36pt; -ivy-float-position: margin 72pt">` +
				`<p style="[^"]*"><span[^>]*>One</span></p><p style="[^"]*"><span[^>]*>Two</span></p></div><p style="[^"]*"><span[^>]*>Out</span></p>`}, 1},
		{"two", `\pard\posy1440\absw2880 One\par\pard\posy4320\absw2880 Two\par`, []string{"margin 72pt", "margin 216pt"}, 2},
		{"page", `\pard\phpg\posxc\pvpg\posyc\absw2880\absh1440\dfrmtxtx288\dfrmtxty144 One\par`,
			[]string{"width: 144pt; min-height: 72pt; float: left; margin: 7.2pt 14.4pt 7.2pt 162pt; -ivy-float-position: page center"}, 1},
		{"width of the text", `\pard\phmrg\posxr\pvmrg\posyt One\par`,
			[]string{"width: 468pt; float: left; margin: 0pt 0pt 0pt 0pt; -ivy-float-position: margin top"}, 1},
		{"overlay", `\pard\pvpg\posy2880\absw2880\absh-1440\overlay One\par`,
			[]string{"width: 144pt; height: 72pt; position: absolute; left: 0pt; top: 72pt"}, 1},
		{"table", `\trowd\cellx2880\pard\intbl\posy1440\absw2880 One\cell\row\pard\posy1440\absw2880 Two\par`,
			[]string{"</table><div", "Two</span></p></div>"}, 1},
		{"drop cap", `\pard\dropcapli3\dropcapt1\posy1440\absw2880 One\par`, nil, 0},
		{"paragraph", `\pard\pvpara\posy1440 One\par`, nil, 0},
		{"columns", `\cols2\pard\posy1440\absw2880 One\par`, nil, 0},
		{"wrap only", `\pard\nowrap\dxfrtext180 One\par`, nil, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := convertHTML(t, []byte(`{\rtf1\ansi\paperw12240\paperh15840\margl1440\margr1440\margt1440\margb1440`+"\n"+tt.body+"}"))
			for _, want := range tt.want {
				if !strings.Contains(html, want) && !matches(want, html) {
					t.Errorf("expected %q, got:\n%s", want, html)
				}
			}
			if n := strings.Count(html, "<div style=\"width"); n != tt.frames {
				t.Errorf("expected %d frames, got %d:\n%s", tt.frames, n, html)
			}
			if strings.Count(html, "<div") != strings.Count(html, "</div>") {
				t.Errorf("expected balanced elements, got:\n%s", html)
			}
		})
	}
}
