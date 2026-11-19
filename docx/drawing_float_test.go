package docx

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
