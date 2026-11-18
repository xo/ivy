package graphics

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"

	"github.com/xo/ivy"
	"github.com/xo/ivy/internal/display"
	"github.com/xo/ivy/ps/internal/interp"
)

// result is the result of a program: its pages and the machine.
type result struct {
	m     *Machine
	in    *interp.Interp
	pages []Page
	err   error
}

// runProgram runs a program on a page of 100 by 100 points.
func runProgram(t *testing.T, program string) result {
	t.Helper()
	return runSetup(t, program, Setup{Width: 100, Height: 100})
}

// runSetup runs a program with a setup of the page device.
func runSetup(t *testing.T, program string, s Setup) result {
	t.Helper()
	in := interp.New(interp.Config{})
	m := New(in, Config{Setup: s})
	in.Finish()
	err := in.Run(t.Context(), interp.NewFile("test", []byte(program)))
	m.Finish(err)
	return result{m: m, in: in, pages: m.Pages(), err: err}
}

// pixel is a pixel of a page that render drew.
type pixmap struct {
	pix *okage.Pixmap
}

// render draws a page at 1 pixel for each point.
func render(t *testing.T, p Page) pixmap {
	t.Helper()
	w, h := uint32(math.Ceil(p.Width)), uint32(math.Ceil(p.Height))
	pix, ok := okage.NewPixmap(w, h)
	if !ok {
		t.Fatalf("pixmap of %dx%d", w, h)
	}
	pix.Fill(okage.ColorFromRGBA8(255, 255, 255, 255))
	s := float32(1 / resolutionScale)
	dev := display.NewOkage(pix, okage.TransformFromScale(s, s))
	p.List.Replay(dev, 1/resolutionScale)
	return pixmap{pix: pix}
}

// at returns the color at the point (x, y) of the default user space, with
// y up.
func (p pixmap) at(x, y int) color.RGBA {
	h := int(p.pix.Height())
	c, _ := p.pix.Pixel(uint32(x), uint32(h-1-y))
	return color.RGBA{c.Red(), c.Green(), c.Blue(), c.Alpha()}
}

// near reports whether two colors differ by at most 8 in each channel.
func near(a, b color.RGBA) bool {
	d := func(x, y uint8) bool { return math.Abs(float64(x)-float64(y)) <= 8 }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

var (
	white = color.RGBA{255, 255, 255, 255}
	black = color.RGBA{0, 0, 0, 255}
	red   = color.RGBA{255, 0, 0, 255}
	green = color.RGBA{0, 255, 0, 255}
	blue  = color.RGBA{0, 0, 255, 255}
)

// check runs a program and checks the colors of points of its first page.
func check(t *testing.T, program string, points map[[2]int]color.RGBA) {
	t.Helper()
	r := runProgram(t, program)
	if r.err != nil {
		t.Fatalf("%q: %v", program, r.err)
	}
	if len(r.pages) == 0 {
		t.Fatalf("%q: no page", program)
	}
	p := render(t, r.pages[0])
	for pt, want := range points {
		if got := p.at(pt[0], pt[1]); !near(got, want) {
			t.Errorf("%q: at %v got %v, want %v", program, pt, got, want)
		}
	}
}

// TestMeshCMYK fills a triangle of a mesh in DeviceCMYK, whose colors are
// not linear in RGB. The colors follow the conversion of the components at
// each point, and not a line between the colors of the corners.
func TestMeshCMYK(t *testing.T) {
	t.Parallel()
	program := "<< /ShadingType 4 /ColorSpace /DeviceCMYK /DataSource [0 0 0 0 0 0 0 0 100 0 1 1 0 0 0 0 100 0 0 0 0] >> shfill showpage"
	points := map[[2]int]color.RGBA{}
	for _, x := range []int{30, 50, 70} {
		c := (float64(x) + 0.5) / 100
		rgb := cmykToRGB(c, c, 0, 0)
		points[[2]int{x, 2}] = color.RGBA{uint8(math.Round(rgb[0] * 255)), uint8(math.Round(rgb[1] * 255)), uint8(math.Round(rgb[2] * 255)), 255}
	}
	check(t, program, points)
}

func TestPaint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		program string
		points  map[[2]int]color.RGBA
	}{
		{"fill", "1 0 0 setrgbcolor 10 10 moveto 90 10 lineto 90 90 lineto closepath fill",
			map[[2]int]color.RGBA{{80, 20}: red, {20, 80}: white}},
		{"eofill", "newpath 10 10 80 80 re 30 30 40 40 re eofill",
			map[[2]int]color.RGBA{{20, 20}: black, {50, 50}: white}},
		{"rectfill", "0 1 0 setrgbcolor 0 0 50 50 rectfill",
			map[[2]int]color.RGBA{{25, 25}: green, {75, 75}: white}},
		{"stroke", "10 setlinewidth 0 0 1 setrgbcolor 0 50 moveto 100 50 lineto stroke",
			map[[2]int]color.RGBA{{50, 50}: blue, {50, 53}: blue, {50, 60}: white}},
		{"cap", "20 setlinewidth 2 setlinecap 30 50 moveto 70 50 lineto stroke",
			map[[2]int]color.RGBA{{25, 50}: black, {75, 50}: black, {85, 50}: white}},
		{"dash", "10 setlinewidth [10 10] 0 setdash 0 50 moveto 100 50 lineto stroke",
			map[[2]int]color.RGBA{{5, 50}: black, {15, 50}: white, {25, 50}: black}},
		{"clip", "0 0 50 50 rectclip 0 0 100 100 rectfill",
			map[[2]int]color.RGBA{{25, 25}: black, {75, 75}: white, {25, 75}: white}},
		{"initclip", "0 0 50 50 rectclip initclip 0 0 100 100 rectfill",
			map[[2]int]color.RGBA{{75, 75}: black}},
		{"gsave", "1 0 0 setrgbcolor gsave 0 0 1 setrgbcolor grestore 0 0 50 50 rectfill",
			map[[2]int]color.RGBA{{25, 25}: red}},
		{"save", "/s save def 0 0 1 setrgbcolor 50 50 translate s restore 0 0 50 50 rectfill",
			map[[2]int]color.RGBA{{25, 25}: black, {75, 75}: white}},
		{"translate", "50 50 translate 0 0 10 10 rectfill",
			map[[2]int]color.RGBA{{55, 55}: black, {5, 5}: white}},
		{"rotate", "50 50 translate 90 rotate 0 0 40 10 rectfill",
			map[[2]int]color.RGBA{{45, 70}: black, {70, 45}: white}},
		{"arc", "50 50 40 0 360 arc fill",
			map[[2]int]color.RGBA{{50, 50}: black, {50, 85}: black, {5, 5}: white}},
		{"gray", "0.5 setgray 0 0 100 100 rectfill",
			map[[2]int]color.RGBA{{50, 50}: {128, 128, 128, 255}}},
		{"cmyk", "0 1 1 0 setcmykcolor 0 0 100 100 rectfill",
			map[[2]int]color.RGBA{{50, 50}: {237, 29, 36, 255}}},
		{"hsb", "0.6667 1 1 sethsbcolor 0 0 100 100 rectfill",
			map[[2]int]color.RGBA{{50, 50}: blue}},
		{"indexed", "[/Indexed /DeviceRGB 1 <ff000000ff00>] setcolorspace 1 setcolor 0 0 100 100 rectfill",
			map[[2]int]color.RGBA{{50, 50}: green}},
		{"separation", "[/Separation /Spot /DeviceCMYK {dup 0 exch 0}] setcolorspace 1 setcolor 0 0 100 100 rectfill",
			map[[2]int]color.RGBA{{50, 50}: {0, 166, 81, 255}}},
		{"cie", "[/CIEBasedABC << /WhitePoint [0.9505 1 1.089] /MatrixLMN [0.4124 0.2126 0.0193 0.3576 0.7152 0.1192 0.1805 0.0722 0.9505] >>] setcolorspace 1 1 1 setcolor 0 0 100 100 rectfill",
			map[[2]int]color.RGBA{{50, 50}: white}},
		{"image", "0 0 translate 100 100 scale 2 2 8 [2 0 0 2 0 0] <00ff ff00> image",
			map[[2]int]color.RGBA{{25, 25}: black, {75, 25}: white, {25, 75}: white, {75, 75}: black}},
		{"imagemask", "1 0 0 setrgbcolor 100 100 scale 2 1 true [2 0 0 1 0 0] <80> imagemask",
			map[[2]int]color.RGBA{{25, 50}: red, {75, 50}: white}},
		{"colorimage", "100 100 scale 2 1 8 [2 0 0 1 0 0] <ff000000ff00> false 3 colorimage",
			map[[2]int]color.RGBA{{25, 50}: red, {75, 50}: green}},
		{"image dict", "/DeviceRGB setcolorspace 100 100 scale << /ImageType 1 /Width 1 /Height 1 /BitsPerComponent 8 /ImageMatrix [1 0 0 1 0 0] /DataSource <0000ff> /Decode [0 1 0 1 0 1] >> image",
			map[[2]int]color.RGBA{{50, 50}: blue}},
		{"image procedure", "100 100 scale 1 1 8 [1 0 0 1 0 0] { <80> } image",
			map[[2]int]color.RGBA{{50, 50}: {128, 128, 128, 255}}},
		{"image currentfile", "100 100 scale 1 1 8 [1 0 0 1 0 0] { currentfile 1 string readhexstring pop } image\n00\n",
			map[[2]int]color.RGBA{{50, 50}: black}},
		{"axial", "<< /ShadingType 2 /ColorSpace /DeviceRGB /Coords [0 0 100 0] /Function << /FunctionType 2 /Domain [0 1] /C0 [1 0 0] /C1 [0 0 1] /N 1 >> /Extend [true true] >> shfill",
			map[[2]int]color.RGBA{{1, 50}: red, {99, 50}: blue}},
		{"radial", "<< /ShadingType 3 /ColorSpace /DeviceGray /Coords [50 50 0 50 50 40] /Function << /FunctionType 2 /Domain [0 1] /C0 [0] /C1 [1] /N 1 >> >> shfill",
			map[[2]int]color.RGBA{{50, 50}: black, {95, 95}: white}},
		{"mesh", "<< /ShadingType 4 /ColorSpace /DeviceRGB /DataSource [0 0 0 0 1 0 0 100 0 0 1 0 0 0 100 0 0 1] >> shfill",
			map[[2]int]color.RGBA{{2, 2}: green, {2, 90}: {0, 25, 230, 255}, {90, 90}: white}},
		{"axial no extend", "<< /ShadingType 2 /ColorSpace /DeviceRGB /Coords [25 0 75 0] /Function << /FunctionType 2 /Domain [0 1] /C0 [1 0 0] /C1 [0 0 1] /N 1 >> >> shfill",
			map[[2]int]color.RGBA{{10, 50}: white, {50, 50}: {125, 0, 130, 255}, {90, 50}: white}},
		{"mesh function", `<< /ShadingType 4 /ColorSpace /DeviceRGB /Function << /FunctionType 3 /Domain [0 1] /Bounds [0.5] /Encode [0 1 0 1]
			/Functions [<< /FunctionType 2 /Domain [0 1] /C0 [1 0 0] /C1 [1 0 0] /N 1 >> << /FunctionType 2 /Domain [0 1] /C0 [0 0 1] /C1 [0 0 1] /N 1 >>] >>
			/DataSource [0 0 0 0 0 100 0 1 0 0 100 0 1 100 100 1] >> shfill`,
			map[[2]int]color.RGBA{{10, 50}: red, {48, 20}: red, {51, 80}: blue, {90, 50}: blue}},
		{"mesh background", `<< /PatternType 2 /Shading << /ShadingType 4 /ColorSpace /DeviceRGB /Background [0 1 0]
			/DataSource [0 40 40 1 0 0 0 60 40 1 0 0 0 50 60 1 0 0] >> >> matrix makepattern setpattern 0 0 100 100 rectfill`,
			map[[2]int]color.RGBA{{5, 5}: green, {95, 95}: green, {50, 52}: red}},
		{"pattern", `<< /PatternType 1 /PaintType 1 /TilingType 1 /BBox [0 0 20 20] /XStep 20 /YStep 20
			/PaintProc { pop 1 0 0 setrgbcolor 0 0 10 20 rectfill } >> matrix makepattern setpattern 0 0 100 100 rectfill`,
			map[[2]int]color.RGBA{{5, 50}: red, {15, 50}: white, {45, 50}: red, {55, 50}: white}},
		{"uncolored pattern", `<< /PatternType 1 /PaintType 2 /TilingType 1 /BBox [0 0 20 20] /XStep 20 /YStep 20
			/PaintProc { pop 0 0 10 20 rectfill } >> matrix makepattern
			[/Pattern /DeviceRGB] setcolorspace 0 0 1 4 -1 roll setcolor 0 0 100 100 rectfill`,
			map[[2]int]color.RGBA{{5, 50}: blue, {15, 50}: white}},
		{"shading pattern", `<< /PatternType 2 /Shading << /ShadingType 2 /ColorSpace /DeviceRGB /Coords [0 0 100 0]
			/Function << /FunctionType 2 /Domain [0 1] /C0 [1 0 0] /C1 [0 1 0] /N 1 >> /Extend [true true] >> >>
			matrix makepattern setpattern 0 0 100 50 rectfill`,
			map[[2]int]color.RGBA{{1, 25}: red, {99, 25}: green, {50, 75}: white}},
		{"form", `<< /FormType 1 /BBox [0 0 50 50] /Matrix [1 0 0 1 0 0] /PaintProc { pop 0 0 100 100 rectfill } >> execform`,
			map[[2]int]color.RGBA{{25, 25}: black, {75, 75}: white}},
		{"strokepath", "10 setlinewidth 0 50 moveto 100 50 lineto strokepath fill",
			map[[2]int]color.RGBA{{50, 52}: black, {50, 60}: white}},
		{"clippath", "0 0 50 50 rectclip clippath fill",
			map[[2]int]color.RGBA{{25, 25}: black, {75, 75}: white}},
		{"erasepage", "0 0 100 100 rectfill erasepage 0 0 10 10 rectfill",
			map[[2]int]color.RGBA{{50, 50}: white, {5, 5}: black}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			program := "/re { 4 2 roll moveto 1 index 0 rlineto 0 exch rlineto neg 0 rlineto closepath } def\n" + tt.program + " showpage"
			check(t, program, tt.points)
		})
	}
}

func TestOperands(t *testing.T) {
	t.Parallel()
	tests := []struct {
		program, want string
	}{
		{"10 20 moveto currentpoint", "10.0 20.0"},
		{"newpath 10 20 moveto 30 40 lineto pathbbox", "10.0 20.0 30.0 40.0"},
		{"2 3 scale 1 1 transform itransform", "1.0 1.0"},
		{"matrix currentmatrix 0 get", "4.16667"},
		{"5 setlinewidth currentlinewidth", "5.0"},
		{"[1 2] 3 setdash currentdash", "[1 2] 3"},
		{"0.25 0.5 0.75 setrgbcolor currentrgbcolor", "0.25 0.5 0.75"},
		{"0.5 setgray currentgray", "0.5"},
		{"0 0 1 setrgbcolor currenthsbcolor pop pop", "0.666667"},
		{"/DeviceCMYK setcolorspace currentcolor", "0.0 0.0 0.0 1.0"},
		{"/Helvetica findfont /FontName get", "/Helvetica"},
		{"/Helvetica findfont 10 scalefont setfont (Hello) stringwidth pop", "22.78"},
		{"/Helvetica findfont 10 scalefont setfont 0 0 moveto (Hello) show currentpoint pop", "22.78"},
		{"/Courier findfont 10 scalefont setfont (ab) stringwidth pop", "12.0"},
		{"/Times-Roman 10 selectfont 0 0 moveto (A) false charpath pathbbox pop pop pop 1 lt", "true"},
		{"/Helvetica findfont dup length dict copy dup /Encoding ISOLatin1Encoding put /H2 exch definefont /FontName get", "/Helvetica"},
		{"StandardEncoding 65 get ISOLatin1Encoding 233 get", "/A /eacute"},
		{"/StandardEncoding findencoding 97 get", "/a"},
		{"currentpagedevice /PageSize get aload pop", "100 100"},
		{"<< /PageSize [200 300] >> setpagedevice currentpagedevice /PageSize get aload pop", "200 300"},
		{"gstate currentgstate type", "/gstatetype"},
		{"0 0 moveto 10 0 lineto {(m)} {(l)} {(c)} {(p)} pathforall", "0.0 0.0 (m) 10.0 0.0 (l)"},
		{"0 0 moveto 10 10 lineto reversepath currentpoint", "0.0 0.0"},
		{"newpath 0 0 moveto 0 10 10 10 10 0 curveto flattenpath [ {pop pop 1} {pop pop 1} {} {} pathforall ] length 4 gt", "true"},
		{"{ 0 0 lineto } stopped $error /errorname get", "0 0 true /nocurrentpoint"},
	}
	for _, tt := range tests {
		t.Run(tt.program, func(t *testing.T) {
			t.Parallel()
			r := runProgram(t, tt.program)
			if r.err != nil {
				t.Fatal(r.err)
			}
			got := stack(t, r.in)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// stack returns the operand stack, from the bottom, as == prints it.
func stack(t *testing.T, in *interp.Interp) string {
	t.Helper()
	var parts []string
	for i := in.Count() - 1; i >= 0; i-- {
		o, _ := in.Peek(i)
		parts = append(parts, repr(o))
	}
	return strings.Join(parts, " ")
}

// repr returns the text of an object as == gives it, for the tests.
func repr(o interp.Object) string {
	switch o.Type() {
	case interp.TInt:
		return fmt.Sprint(o.IntVal())
	case interp.TReal:
		s := fmt.Sprintf("%.6g", o.Float())
		if !strings.ContainsAny(s, ".e") {
			s += ".0"
		}
		if v := o.Float(); math.Abs(v*100-math.Round(v*100)) < 1e-6 && math.Abs(v) < 1e6 && !strings.Contains(s, "e") {
			s = fmt.Sprintf("%g", math.Round(v*100)/100)
			if !strings.Contains(s, ".") {
				s += ".0"
			}
		}
		return s
	case interp.TBool:
		return fmt.Sprint(o.BoolVal())
	case interp.TName:
		return "/" + o.NameVal()
	case interp.TString:
		return "(" + o.Text() + ")"
	case interp.TArray, interp.TPacked:
		var parts []string
		for _, e := range o.Array() {
			parts = append(parts, repr(e))
		}
		return "[" + strings.Join(parts, " ") + "]"
	}
	return "-" + o.Type().String() + "-"
}

func TestPages(t *testing.T) {
	t.Parallel()
	r := runProgram(t, "0 0 10 10 rectfill showpage 0 0 20 20 rectfill copypage showpage << /PageSize [50 60] >> setpagedevice 0 0 5 5 rectfill")
	if r.err != nil {
		t.Fatal(r.err)
	}
	if len(r.pages) != 4 {
		t.Fatalf("got %d pages, want 4", len(r.pages))
	}
	if p := r.pages[3]; p.Width != 50 || p.Height != 60 {
		t.Errorf("page 4 is %gx%g, want 50x60", p.Width, p.Height)
	}
	// the page after copypage keeps the marks of copypage
	if got := render(t, r.pages[2]).at(15, 15); !near(got, black) {
		t.Errorf("page 3 at (15, 15) got %v, want black", got)
	}
	if got := render(t, r.pages[0]).at(15, 15); !near(got, white) {
		t.Errorf("page 1 at (15, 15) got %v, want white", got)
	}
}

func TestEPS(t *testing.T) {
	t.Parallel()
	// an EPS file draws its bounding box from (100, 200), and gives the
	// first page only
	r := runSetup(t, "100 200 50 50 rectfill showpage 0 0 1000 1000 rectfill showpage",
		Setup{Width: 100, Height: 100, X: 100, Y: 200, FirstPageOnly: true})
	if r.err != nil {
		t.Fatal(r.err)
	}
	if len(r.pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(r.pages))
	}
	p := render(t, r.pages[0])
	if got := p.at(25, 25); !near(got, black) {
		t.Errorf("at (25, 25) got %v, want black", got)
	}
	if got := p.at(75, 75); !near(got, white) {
		t.Errorf("at (75, 75) got %v, want white", got)
	}
}

func TestText(t *testing.T) {
	t.Parallel()
	check(t, "/Helvetica findfont 100 scalefont setfont 10 10 moveto (I) show showpage",
		map[[2]int]color.RGBA{{20, 40}: black, {60, 40}: white})
	// the bar of an L is at the bottom
	check(t, "/Helvetica findfont 100 scalefont setfont 10 10 moveto (L) show showpage",
		map[[2]int]color.RGBA{{40, 14}: black, {40, 60}: white})
	// a Type 3 font fills a square for each glyph
	check(t, `8 dict begin /FontType 3 def /FontMatrix [0.01 0 0 0.01 0 0] def /FontBBox [0 0 100 100] def
		/Encoding 256 array def 0 1 255 { Encoding exch /sq put } for
		/BuildGlyph { pop pop 100 0 0 0 100 100 setcachedevice 0 0 50 100 rectfill } def
		/BuildChar { 1 index /Encoding get exch get 1 index /BuildGlyph get exec } def
		currentdict end /Square exch definefont pop
		1 0 0 setrgbcolor /Square 40 selectfont 10 10 moveto (ab) show showpage`,
		map[[2]int]color.RGBA{{15, 30}: red, {40, 30}: white, {55, 30}: red, {80, 30}: white})
}

// encrypt encrypts data with the cipher of Type 1 fonts, after n bytes of
// zeros.
func encrypt(data []byte, r uint16, n int) []byte {
	out := make([]byte, 0, len(data)+n)
	for _, p := range append(make([]byte, n), data...) {
		c := p ^ byte(r>>8)
		r = (uint16(c)+r)*52845 + 22719
		out = append(out, c)
	}
	return out
}

// charstring encodes the numbers and the operators of a Type 1
// charstring: an int is a number, and a string is an operator.
func charstring(items ...any) []byte {
	ops := map[string]byte{"hsbw": 13, "rmoveto": 21, "rlineto": 5, "closepath": 9, "endchar": 14}
	var b []byte
	for _, it := range items {
		switch v := it.(type) {
		case int:
			switch {
			case v >= -107 && v <= 107:
				b = append(b, byte(v+139))
			case v >= 108 && v <= 1131:
				v -= 108
				b = append(b, byte(v/256+247), byte(v%256))
			default:
				v = -v - 108
				b = append(b, byte(v/256+251), byte(v%256))
			}
		case string:
			b = append(b, ops[v])
		}
	}
	return encrypt(b, 4330, 4)
}

// type1Font returns the program of a Type 1 font whose glyph A is a square
// of 500 units, and whose width is 600. hex encrypts it in hexadecimal.
func type1Font(hex bool) string {
	var priv bytes.Buffer
	priv.WriteString("dup /Private 8 dict dup begin\n/RD {string currentfile exch readstring pop} executeonly def\n")
	priv.WriteString("/ND {noaccess def} executeonly def\n/NP {noaccess put} executeonly def\n/lenIV 4 def\n")
	priv.WriteString("/Subrs 0 array ND\n2 index /CharStrings 2 dict dup begin\n")
	for _, g := range []struct {
		name string
		cs   []byte
	}{
		{".notdef", charstring(0, 0, "hsbw", "endchar")},
		{"A", charstring(0, 600, "hsbw", 0, 0, "rmoveto", 500, 0, "rlineto", 0, 500, "rlineto", -500, 0, "rlineto", "closepath", "endchar")},
	} {
		fmt.Fprintf(&priv, "/%s %d RD ", g.name, len(g.cs))
		priv.Write(g.cs)
		priv.WriteString(" ND\n")
	}
	priv.WriteString("end\nend\nreadonly put\nnoaccess put\ndup /FontName get exch definefont pop\nmark currentfile closefile\n")
	enc := encrypt(priv.Bytes(), 55665, 4)
	var b strings.Builder
	b.WriteString("%!PS-AdobeFont-1.0: Square\n10 dict begin\n/FontType 1 def\n/FontName /Square def\n")
	b.WriteString("/FontMatrix [0.001 0 0 0.001 0 0] readonly def\n/FontBBox {0 0 500 500} readonly def\n/PaintType 0 def\n")
	b.WriteString("/Encoding 256 array 0 1 255 {1 index exch /.notdef put} for dup 65 /A put readonly def\n")
	b.WriteString("currentdict end\ncurrentfile eexec\n")
	if hex {
		for i, c := range enc {
			fmt.Fprintf(&b, "%02x", c)
			if i%32 == 31 {
				b.WriteByte('\n')
			}
		}
	} else {
		b.Write(enc)
	}
	b.WriteString("\n")
	for range 8 {
		b.WriteString(strings.Repeat("0", 64) + "\n")
	}
	b.WriteString("cleartomark\n")
	return b.String()
}

func TestType1(t *testing.T) {
	t.Parallel()
	for _, hex := range []bool{true, false} {
		t.Run(fmt.Sprint("hex=", hex), func(t *testing.T) {
			t.Parallel()
			program := type1Font(hex) + "/Square 100 selectfont 10 10 moveto (A) show currentpoint pop 70 sub abs 0.01 lt { 0 1 0 setrgbcolor 90 90 5 5 rectfill } if showpage"
			check(t, program, map[[2]int]color.RGBA{{30, 30}: black, {50, 70}: white, {92, 92}: green})
		})
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	lim := ivy.Limits{MaxObjects: 1 << 12, MaxMemory: 1 << 20}
	for _, program := range []string{
		"{ 0 0 1 1 rectfill } loop",
		"{ 1 pop } loop",
		"/f { gsave f } def f",
		"{ 100000 string pop } loop",
		"1000000 1000000 8 [1 0 0 1 0 0] {<00>} image",
		"{ showpage } loop",
	} {
		t.Run(program, func(t *testing.T) {
			t.Parallel()
			in := interp.New(interp.Config{Limits: lim})
			m := New(in, Config{Setup: Setup{Width: 10, Height: 10}})
			in.Finish()
			err := in.Run(t.Context(), interp.NewFile("test", []byte(program)))
			m.Finish(err)
			var je *interp.JobError
			if !errors.Is(err, ivy.ErrLimit) && !errors.As(err, &je) && len(m.Pages()) < maxPages {
				t.Errorf("got %v, want a limit", err)
			}
		})
	}
}

func TestCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	in := interp.New(interp.Config{})
	New(in, Config{})
	in.Finish()
	err := in.Run(ctx, interp.NewFile("test", []byte("{ 0 0 1 1 rectfill } loop")))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
}

// sfnts returns the TrueType font goregular as the sfnts array of a Type 42
// font, and the glyph of the letter I.
func sfnts(t *testing.T) (string, int) {
	t.Helper()
	f, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	gid, err := f.GlyphIndex(&buf, 'I')
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("[")
	data := goregular.TTF
	for len(data) > 0 {
		n := min(len(data), 30000)
		b.WriteString("<" + hex.EncodeToString(data[:n]) + ">\n")
		data = data[n:]
	}
	b.WriteString("]")
	return b.String(), int(gid)
}

func TestType42(t *testing.T) {
	t.Parallel()
	arr, gid := sfnts(t)
	program := fmt.Sprintf(`10 dict begin /FontType 42 def /FontName /Go def /FontMatrix [1 0 0 1 0 0] def
		/FontBBox [0 0 1 1] def /PaintType 0 def /Encoding 256 array def 0 1 255 { Encoding exch /.notdef put } for
		Encoding 73 /I put /CharStrings << /.notdef 0 /I %d >> def /sfnts %s def
		currentdict end /Go exch definefont pop /Go 100 selectfont 10 10 moveto (I) show showpage`, gid, arr)
	check(t, program, map[[2]int]color.RGBA{{30, 40}: black, {60, 40}: white})
}

func TestComposite(t *testing.T) {
	t.Parallel()
	// a Type 0 font of FMapType 2 over Helvetica: the first byte selects
	// the font
	check(t, `<< /FontType 0 /FMapType 2 /FontMatrix [1 0 0 1 0 0] /Encoding [0]
		/FDepVector [/Helvetica findfont] >> /Comp exch definefont pop
		/Comp 100 selectfont 10 10 moveto <0049> show showpage`,
		map[[2]int]color.RGBA{{20, 40}: black, {60, 40}: white})
	// a CIDFont of type 2 with Identity-H, where a CID is a glyph
	arr, gid := sfnts(t)
	check(t, fmt.Sprintf(`20 dict begin /CIDFontType 2 def /CIDFontName /GoCID def /FontMatrix [1 0 0 1 0 0] def
		/FontBBox [0 0 1 1] def /CIDCount 1000 def /GDBytes 2 def /CIDMap 0 def /sfnts %s def
		/CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> def
		CIDFontName currentdict end /CIDFont defineresource pop
		/GoComp /Identity-H [/GoCID] composefont pop /GoComp 100 selectfont 10 10 moveto <%04x> show showpage`, arr, gid),
		map[[2]int]color.RGBA{{30, 40}: black, {60, 40}: white})
}

func TestStrokeAdjust(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		adjust string
		want   bool
	}{{"true", true}, {"false", false}} {
		r := runProgram(t, tt.adjust+" setstrokeadjust 10 50 moveto 90 50 lineto stroke showpage")
		if r.err != nil || len(r.pages) == 0 {
			t.Fatalf("%s: expected a page, got %v", tt.adjust, r.err)
		}
		var l display.List
		r.pages[0].List.Replay(&l, 1)
		if got := strings.Contains(l.String(), "stroke adjusted"); got != tt.want {
			t.Errorf("%s: expected an adjusted stroke %t, got:\n%s", tt.adjust, tt.want, l.String())
		}
	}
}
