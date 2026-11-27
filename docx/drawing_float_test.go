package docx

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/ivy/internal/fixture"
)

// TestPageFloatFiles checks the position on the page of the floats of the
// files of D60: the tables of w:tblpPr, the frames of w:framePr and the
// picture of wp:anchor, from the margin and from the page (D60).
func TestPageFloatFiles(t *testing.T) {
	for _, tt := range []struct {
		name string
		want []string
	}{
		{"t-late", []string{"float: left; margin: 0pt 9pt 0pt -5.4pt; -ivy-float-position: margin 72pt"}},
		{"t-page2", []string{"-ivy-float-position: margin 72pt"}},
		{"f-late", []string{"width: 216pt; height: 144pt; float: left; margin: 0pt 9pt 0pt 0pt; -ivy-float-position: margin 72pt"}},
		{"s-late", []string{"float: left; margin-left: 0pt; margin-right: 9pt; margin-bottom: 0pt; margin-top: 0pt; -ivy-float-position: margin 72pt"}},
		{"f-bottom", []string{"-ivy-float-position: margin bottom"}},
		{"t-bottom", []string{"-ivy-float-position: margin bottom"}},
		{"t-two", []string{"-ivy-float-position: margin 72pt", "-ivy-float-position: margin 72pt"}},
		{"t-never", []string{"-ivy-float-position: margin 72pt", "-ivy-float-position: margin 72pt"}},
		{"t-loop3", []string{"-ivy-float-position: margin 0pt"}},
		{"t-loop2", []string{"-ivy-float-position: margin 0pt"}},
		{"t-nofit", []string{"-ivy-float-position: page 684pt"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := os.ReadFile(filepath.Join("testdata", "float-"+tt.name+".docx"))
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
				t.Errorf("got %d positions, want %q:\n%s", got, tt.want, b)
			}
			if n, err := d.PageCount(t.Context()); err != nil || n < 2 {
				t.Errorf("got %d pages, %v", n, err)
			}
		})
	}
}

// TestFloatPosition checks the value of -ivy-float-position for each
// position of wp:positionV, w:tblpPr and w:framePr (D60).
func TestFloatPosition(t *testing.T) {
	for _, tt := range []struct {
		relative, align string
		y               float64
		want            string
	}{
		{"margin", "", 72, "margin 72pt"},
		{"page", "", 36.5, "page 36.5pt"},
		{"margin", "top", 72, "margin top"},
		{"page", "center", 0, "page center"},
		{"margin", "bottom", 0, "margin bottom"},
		{"margin", "inside", 0, "margin top"},
		{"page", "outside", 0, "page bottom"},
		{"paragraph", "", 72, ""},
		{"line", "top", 0, ""},
		{"text", "", 72, ""},
		{"", "", 72, ""},
	} {
		if got := floatPosition(tt.relative, tt.align, tt.y); got != tt.want {
			t.Errorf("floatPosition(%q, %q, %g) = %q, want %q", tt.relative, tt.align, tt.y, got, tt.want)
		}
	}
}

// TestPageFloatCSS checks the side and the margins of a table or a frame
// that floats at its position (D60).
func TestPageFloatCSS(t *testing.T) {
	for _, tt := range []struct {
		name string
		x, w float64
		wrap bool
		want string
	}{
		{"left", 0, 216, true, "float: left; margin: 1pt 2pt 3pt 0pt; -ivy-float-position: margin 72pt"},
		{"right", 252, 216, true, "float: right; margin: 1pt 0pt 3pt 4pt; -ivy-float-position: margin 72pt"},
		{"wide", 0, 450, true, "float: left; margin: 1pt 18pt 3pt 0pt; -ivy-float-position: margin 72pt"},
		{"not beside", 36, 216, false, "float: left; margin: 1pt 216pt 3pt 36pt; -ivy-float-position: margin 72pt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var s css
			pageFloatCSS(&s, tt.x, tt.w, 468, [4]float64{1, 2, 3, 4}, tt.wrap, "margin 72pt")
			if got := s.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFloatOverlap checks that a table with w:tblOverlap "never", a
// drawing with allowOverlap off and a VML shape with o:allowoverlap off
// avoid the other page floats, and that the other values and a drawing at
// its paragraph do not (D60).
func TestFloatOverlap(t *testing.T) {
	table := func(overlap string) string {
		return `<w:tbl><w:tblPr><w:tblpPr w:vertAnchor="margin" w:horzAnchor="margin" w:tblpY="1440"/>` + overlap +
			`<w:tblW w:w="4320" w:type="dxa"/></w:tblPr><w:tblGrid><w:gridCol w:w="4320"/></w:tblGrid>` +
			`<w:tr><w:tc><w:p><w:r><w:t>T</w:t></w:r></w:p></w:tc></w:tr></w:tbl><w:p/>`
	}
	drawing := func(allow, relative string) string {
		return `<w:p><w:r><w:drawing><wp:anchor distT="0" distB="0" distL="0" distR="0" allowOverlap="` + allow + `">` +
			`<wp:positionH relativeFrom="margin"><wp:posOffset>0</wp:posOffset></wp:positionH>` +
			`<wp:positionV relativeFrom="` + relative + `"><wp:posOffset>914400</wp:posOffset></wp:positionV>` +
			`<wp:extent cx="2743200" cy="1828800"/><wp:wrapSquare wrapText="bothSides"/><wp:docPr id="1" name="Box"/>` +
			`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"><wps:wsp><wps:spPr/><wps:bodyPr/></wps:wsp></a:graphicData></a:graphic>` +
			`</wp:anchor></w:drawing></w:r></w:p>`
	}
	vml := func(allow, relative, wrap string) string {
		return `<w:p><w:r><w:pict><v:rect ` + allow + ` style="position:absolute;margin-left:0;margin-top:72pt;width:216pt;height:144pt;` +
			`mso-position-vertical-relative:` + relative + `" fillcolor="red"><w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="` + wrap + `"/></v:rect></w:pict></w:r></w:p>`
	}
	for _, tt := range []struct {
		name, body string
		want       int
	}{
		{"table never", table(`<w:tblOverlap w:val="never"/>`), 1},
		{"table overlap", table(`<w:tblOverlap w:val="overlap"/>`), 0},
		{"table", table(``), 0},
		{"drawing 0", drawing("0", "margin"), 1},
		{"drawing false", drawing("false", "page"), 1},
		{"drawing 1", drawing("1", "margin"), 0},
		{"drawing at its paragraph", drawing("0", "paragraph"), 0},
		{"vml f", vml(`o:allowoverlap="f"`, "margin", "square"), 1},
		{"vml false", vml(`o:allowoverlap="false"`, "page", "topAndBottom"), 1},
		{"vml t", vml(`o:allowoverlap="t"`, "margin", "square"), 0},
		{"vml", vml(``, "margin", "square"), 0},
		{"vml at its paragraph", vml(`o:allowoverlap="f"`, "text", "square"), 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := convertHTML(t, fixture.DOCX(tt.body))
			if got := strings.Count(html, "-ivy-float-overlap: avoid"); got != tt.want {
				t.Errorf("got %d floats that avoid the others, want %d:\n%s", got, tt.want, html)
			}
		})
	}
}

// TestColumnFloat checks that a drawing and a VML shape with text around
// them float in their column in a section with columns, without
// -ivy-float-position, and that a shape that crosses the right side of its
// column floats at the left with a margin that gives its place (D60).
// The section has 3 columns of 120pt, with 36pt between them.
func TestColumnFloat(t *testing.T) {
	const sect = `<w:sectPr><w:pgSz w:w="12240" w:h="15840"/>` +
		`<w:pgMar w:top="1440" w:right="1800" w:bottom="1440" w:left="1800" w:header="720" w:footer="720" w:gutter="0"/>` +
		`<w:cols w:num="3" w:space="720"/></w:sectPr>`
	vml := func(x, relative, wrap string) string {
		return `<w:p><w:r><w:pict><v:oval style="position:absolute;margin-left:` + x + `;margin-top:72pt;width:140pt;height:92pt;` +
			`mso-position-vertical-relative:` + relative + `" fillcolor="aqua"><w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="` + wrap + `"/></v:oval></w:pict></w:r>` +
			`<w:r><w:t>Text</w:t></w:r></w:p>` + sect
	}
	drawing := func(x string) string {
		return `<w:p><w:r><w:drawing><wp:anchor distT="0" distB="0" distL="114300" distR="114300">` +
			`<wp:positionH relativeFrom="column"><wp:posOffset>` + x + `</wp:posOffset></wp:positionH>` +
			`<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>` +
			`<wp:extent cx="635000" cy="635000"/><wp:wrapSquare wrapText="bothSides"/><wp:docPr id="1" name="Picture 1"/>` +
			`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"/></a:graphic>` +
			`</wp:anchor></w:drawing></w:r><w:r><w:t>Text</w:t></w:r></w:p>` + sect
	}
	for _, tt := range []struct {
		name, body, want, position string
	}{
		{"vml across", vml("270pt", "text", "tight"), "float: left; margin-left: 270pt; margin-right: 9pt; margin-top: 72pt; -ivy-wrap-flow: both 9pt", ""},
		{"vml in its column", vml("-20pt", "text", "square"), "float: left; margin-left: 0pt; margin-right: 9pt; margin-top: 72pt; -ivy-wrap-flow: both 9pt", ""},
		// A position from the margin or from the page is kept in a column.
		{"vml from the margin", vml("270pt", "margin", "square"), "float: left; margin-left: 270pt; margin-right: 9pt; -ivy-float-position: margin 72pt; -ivy-wrap-flow: both 9pt", "margin 72pt"},
		// A shape above and below the text floats from the top of the part
		// of its paragraph in the column, with no room beside it.
		{"vml above and below", vml("20pt", "text", "topAndBottom"), "float: left; margin-left: 20pt; margin-right: 0pt; -ivy-float-position: block 72pt", "block 72pt"},
		{"vml above and below from the line", vml("20pt", "line", "topAndBottom"), "-ivy-float-position: line 72pt", "line 72pt"},
		// 50pt wide at 100pt crosses the column, at 60pt it does not
		{"drawing across", drawing("1270000"), "float: left; margin-left: 100pt; margin-right: 9pt", ""},
		{"drawing at the right", drawing("762000"), "float: right; margin-right: 10pt; margin-left: 9pt", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := convertHTML(t, fixture.DOCX(tt.body))
			if !strings.Contains(html, tt.want) {
				t.Errorf("expected %q in the xhtml:\n%s", tt.want, html)
			}
			if got := strings.Contains(html, "-ivy-float-position"); got != (tt.position != "") || got && !strings.Contains(html, "-ivy-float-position: "+tt.position) {
				t.Errorf("expected -ivy-float-position %q in the xhtml:\n%s", tt.position, html)
			}
		})
	}
}

// TestFitText tests the VML text boxes with mso-wrap-style none, which
// are as wide as their text in Word.
func TestFitText(t *testing.T) {
	shape := func(style, wrap string) string {
		return `<w:p><w:r><w:pict><v:rect style="position:absolute;margin-left:10pt;margin-top:20pt;width:114pt;height:77pt;` + style +
			`" fillcolor="red"><v:textbox inset="0,0,0,0"><w:txbxContent><w:p><w:r><w:t>Shape</w:t></w:r></w:p></w:txbxContent></v:textbox>` +
			`<w10:wrap xmlns:w10="urn:schemas-microsoft-com:office:word" type="` + wrap + `"/></v:rect></w:pict></w:r><w:r><w:t>Text</w:t></w:r></w:p>`
	}
	for _, tt := range []struct {
		name, body string
		fit        bool
	}{
		{"square", shape("mso-wrap-style:none", "square"), true},
		{"tight", shape("mso-wrap-style:none;mso-position-horizontal:left", "tight"), true},
		{"no wrap", shape("mso-wrap-style:none", "none"), true},
		{"wrap style square", shape("mso-wrap-style:square", "square"), false},
		{"at the right", shape("mso-wrap-style:none;mso-position-horizontal:right", "square"), false},
		// A block keeps the width of its shape.
		{"above and below", shape("mso-wrap-style:none", "topAndBottom"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := convertHTML(t, fixture.DOCX(tt.body))
			fit := strings.Contains(html, `<span class="fit"`)
			if fit != tt.fit || fit == strings.Contains(html, "width: 114pt") {
				t.Errorf("fit = %t, want %t, in the xhtml:\n%s", fit, tt.fit, html)
			}
		})
	}
}

// TestCellDrawingAbove tests a drawing in a table cell with layoutInCell
// above its line or its paragraph, which floats there in Word, but not
// above the top of the cell.
func TestCellDrawingAbove(t *testing.T) {
	drawing := func(relative, offset, inCell string) string {
		return `<w:p><w:r><w:drawing><wp:anchor distT="0" distB="0" distL="0" distR="0" layoutInCell="` + inCell + `">` +
			`<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>` +
			`<wp:positionV relativeFrom="` + relative + `"><wp:posOffset>` + offset + `</wp:posOffset></wp:positionV>` +
			`<wp:extent cx="63500" cy="63500"/><wp:wrapSquare wrapText="bothSides"/><wp:docPr id="1" name="Picture 1"/>` +
			`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"/></a:graphic>` +
			`</wp:anchor></w:drawing></w:r></w:p>`
	}
	cell := func(p string) string {
		return `<w:tbl><w:tblGrid><w:gridCol w:w="4000"/></w:tblGrid><w:tr><w:tc><w:p><w:r><w:t>Head</w:t></w:r></w:p>` + p + `</w:tc></w:tr></w:tbl><w:p/>`
	}
	for _, tt := range []struct {
		name, body, position string
	}{
		{"line", cell(drawing("line", "-1270000", "1")), "line -100pt"},
		{"paragraph", cell(drawing("paragraph", "-1270000", "1")), "block -100pt"},
		{"below", cell(drawing("line", "1270000", "1")), ""},
		{"not in cell", cell(drawing("line", "-1270000", "0")), ""},
		{"body", drawing("line", "-1270000", "1"), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := convertHTML(t, fixture.DOCX(tt.body))
			if got := strings.Contains(html, "-ivy-float-position"); got != (tt.position != "") || got && !strings.Contains(html, "-ivy-float-position: "+tt.position) {
				t.Errorf("expected -ivy-float-position %q in the xhtml:\n%s", tt.position, html)
			}
		})
	}
}

// TestFloatsOnly tests a paragraph that holds only a picture that floats.
// Its mark is on a line, as in Word, and the box of the picture is the
// picture.
func TestFloatsOnly(t *testing.T) {
	anchor := `<w:r><w:drawing><wp:anchor distT="0" distB="0" distL="0" distR="0">` +
		`<wp:positionH relativeFrom="column"><wp:posOffset>0</wp:posOffset></wp:positionH>` +
		`<wp:positionV relativeFrom="paragraph"><wp:posOffset>0</wp:posOffset></wp:positionV>` +
		`<wp:extent cx="63500" cy="63500"/><wp:wrapSquare wrapText="bothSides"/><wp:docPr id="1" name="Picture 1"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:blipFill><a:blip r:embed="rIdImg"/></pic:blipFill></pic:pic></a:graphicData></a:graphic>` +
		`</wp:anchor></w:drawing></w:r>`
	for _, tt := range []struct {
		name, body string
		br         bool
	}{
		{"float", `<w:p>` + anchor + `</w:p>`, true},
		{"float and text", `<w:p>` + anchor + `<w:r><w:t>Text</w:t></w:r></w:p>`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			html, _ := convertHTML(t, fixture.DOCX(tt.body))
			if got := strings.Contains(html, "</span><br/></p>"); got != tt.br {
				t.Errorf("break = %t, want %t, in the xhtml:\n%s", got, tt.br, html)
			}
			if !strings.Contains(html, "width: 5pt; line-height: 0") {
				t.Errorf("expected a picture without a line in the xhtml:\n%s", html)
			}
		})
	}
}
